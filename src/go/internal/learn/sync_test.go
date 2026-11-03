package learn

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/simon/repo-template/internal/calibre"
	"github.com/simon/repo-template/internal/store"
	"github.com/simon/repo-template/internal/testutil"
)

// buildCalibre skapar ett Calibre-bibliotek med en riktig e-pub i.
func buildCalibre(t *testing.T) (*calibre.Library, string) {
	t.Helper()
	dir := t.TempDir()

	epubPath, err := testutil.BuildEpub(filepath.Join(dir, "Bok (1)"))
	if err != nil {
		t.Fatalf("kunde inte bygga e-pub: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "metadata.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna testdatabas: %v", err)
	}
	defer db.Close()
	statements := []string{
		`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, path TEXT, timestamp REAL)`,
		`CREATE TABLE data (id INTEGER PRIMARY KEY, book INTEGER, format TEXT, uncompressed_size INTEGER, name TEXT)`,
		`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE books_authors_link (id INTEGER PRIMARY KEY, book INTEGER, author INTEGER)`,
		`INSERT INTO books (id, title, path, timestamp) VALUES (1, 'Thinking About Thinking', 'Bok (1)', 1700000000)`,
		`INSERT INTO data (id, book, format, uncompressed_size, name) VALUES (1, 1, 'EPUB', 100, 'thinking')`,
		`INSERT INTO authors (id, name) VALUES (1, 'Test Author')`,
		`INSERT INTO books_authors_link (id, book, author) VALUES (1, 1, 1)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("kunde inte fylla testdatabas: %v", err)
		}
	}

	lib, err := calibre.Open(dir)
	if err != nil {
		t.Fatalf("kunde inte öppna bibliotek: %v", err)
	}
	t.Cleanup(func() { lib.Close() })
	return lib, epubPath
}

func TestSyncImportsMissingBook(t *testing.T) {
	lib, _ := buildCalibre(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "learn.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna butiken: %v", err)
	}
	defer st.Close()

	result, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10})
	if err != nil {
		t.Fatalf("synk misslyckades: %v", err)
	}
	if len(result.Imported) != 1 {
		t.Fatalf("väntade 1 importerad bok, fick %d", len(result.Imported))
	}
	if result.Skipped != 0 {
		t.Errorf("väntade 0 hoppade, fick %d", result.Skipped)
	}
	books, err := st.ListBooks()
	if err != nil {
		t.Fatalf("kunde inte lista böcker: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("väntade 1 bok i databasen, fick %d", len(books))
	}
	if books[0].Title != "Thinking About Thinking" {
		t.Errorf("fel titel: %q", books[0].Title)
	}
}

func TestSyncSkipsAlreadyImportedBook(t *testing.T) {
	lib, _ := buildCalibre(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "learn.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna butiken: %v", err)
	}
	defer st.Close()

	if _, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10}); err != nil {
		t.Fatalf("första synken misslyckades: %v", err)
	}
	result, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10})
	if err != nil {
		t.Fatalf("andra synken misslyckades: %v", err)
	}
	if len(result.Imported) != 0 {
		t.Errorf("väntade inga nya böcker, fick %d", len(result.Imported))
	}
	if result.Skipped != 1 {
		t.Errorf("väntade 1 hoppad bok, fick %d", result.Skipped)
	}
	books, _ := st.ListBooks()
	if len(books) != 1 {
		t.Fatalf("väntade fortfarande 1 bok, fick %d", len(books))
	}
}

func TestSyncReimportsChangedBook(t *testing.T) {
	lib, epubPath := buildCalibre(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "learn.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna butiken: %v", err)
	}
	defer st.Close()

	if _, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10}); err != nil {
		t.Fatalf("första synken misslyckades: %v", err)
	}

	// Gör filen nyare än importen.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(epubPath, future, future); err != nil {
		t.Fatalf("kunde inte ändra tidsstämpel: %v", err)
	}

	result, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10})
	if err != nil {
		t.Fatalf("andra synken misslyckades: %v", err)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("väntade 1 uppdaterad bok, fick %d", len(result.Updated))
	}
	books, _ := st.ListBooks()
	if len(books) != 1 {
		t.Fatalf("väntade fortfarande 1 bok, fick %d", len(books))
	}
}

func TestSyncRemembersFailedBook(t *testing.T) {
	library := t.TempDir()
	bookDir := filepath.Join(library, "Trasig (1)")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatalf("kunde inte skapa bokmapp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "trasig.epub"), []byte("inte en e-pub"), 0o644); err != nil {
		t.Fatalf("kunde inte skapa trasig bok: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(library, "metadata.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, path TEXT, timestamp REAL)`,
		`CREATE TABLE data (id INTEGER PRIMARY KEY, book INTEGER, format TEXT, uncompressed_size INTEGER, name TEXT)`,
		`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE books_authors_link (id INTEGER PRIMARY KEY, book INTEGER, author INTEGER)`,
		`INSERT INTO books (id, title, path, timestamp) VALUES (1, 'Trasig', 'Trasig (1)', 1700000000)`,
		`INSERT INTO data (id, book, format, uncompressed_size, name) VALUES (1, 1, 'EPUB', 10, 'trasig')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	lib, err := calibre.Open(library)
	if err != nil {
		t.Fatalf("kunde inte öppna bibliotek: %v", err)
	}
	defer lib.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "learn.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna butiken: %v", err)
	}
	defer st.Close()

	result, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10})
	if err != nil {
		t.Fatalf("synk misslyckades: %v", err)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("väntade 1 misslyckad bok, fick %d", len(result.Failed))
	}

	// Andra körningen ska hoppa över boken i stället för att pröva igen.
	result, err = SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10})
	if err != nil {
		t.Fatalf("andra synken misslyckades: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("väntade inga nya misslyckanden, fick %d", len(result.Failed))
	}
	if result.Skipped != 1 {
		t.Fatalf("väntade 1 hoppad bok, fick %d", result.Skipped)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	lib, _ := buildCalibre(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "learn.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna butiken: %v", err)
	}
	defer st.Close()

	result, err := SyncCalibre(lib, st, SyncOptions{MaxConcepts: 10, DryRun: true})
	if err != nil {
		t.Fatalf("synk misslyckades: %v", err)
	}
	if len(result.Imported) != 1 {
		t.Fatalf("väntade 1 bok att rapportera, fick %d", len(result.Imported))
	}
	books, _ := st.ListBooks()
	if len(books) != 0 {
		t.Fatalf("dry-run skrev %d böcker", len(books))
	}
}
