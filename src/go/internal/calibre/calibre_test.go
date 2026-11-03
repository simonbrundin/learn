package calibre

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// buildLibrary skapar ett minimalt Calibre-bibliotek med metadata.db.
func buildLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	db, err := sql.Open("sqlite", filepath.Join(dir, "metadata.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna testdatabas: %v", err)
	}
	defer db.Close()

	statements := []string{
		`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, path TEXT, timestamp REAL, sort TEXT,
			author_sort TEXT, isbn TEXT, lccn TEXT, flags INTEGER, uuid TEXT, has_cover INTEGER, last_modified TEXT)`,
		`CREATE TABLE data (id INTEGER PRIMARY KEY, book INTEGER, format TEXT, uncompressed_size INTEGER, name TEXT)`,
		`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT, sort TEXT, link TEXT)`,
		`CREATE TABLE books_authors_link (id INTEGER PRIMARY KEY, book INTEGER, author INTEGER)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("kunde inte skapa tabell: %v", err)
		}
	}

	inserts := []string{
		`INSERT INTO books (id, title, path, timestamp) VALUES (1, 'Tänkande', 'Författare/Tänkande (1)', 1700000000)`,
		`INSERT INTO books (id, title, path, timestamp) VALUES (2, 'Bara pdf', 'Författare/Bara pdf (2)', 1700000001)`,
		`INSERT INTO books (id, title, path, timestamp) VALUES (3, 'Saknad fil', 'Författare/Saknad fil (3)', 1700000002)`,
		`INSERT INTO data (id, book, format, uncompressed_size, name) VALUES (1, 1, 'EPUB', 100, 'Tänkande - Författare')`,
		`INSERT INTO data (id, book, format, uncompressed_size, name) VALUES (2, 1, 'ORIGINAL_EPUB', 200, 'Tänkande original')`,
		`INSERT INTO data (id, book, format, uncompressed_size, name) VALUES (3, 2, 'PDF', 300, 'Bara pdf')`,
		`INSERT INTO data (id, book, format, uncompressed_size, name) VALUES (4, 3, 'EPUB', 400, 'Saknad fil')`,
		`INSERT INTO authors (id, name) VALUES (1, 'Författare')`,
		`INSERT INTO books_authors_link (id, book, author) VALUES (1, 1, 1)`,
	}
	for _, statement := range inserts {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("kunde inte fylla testdatabas: %v", err)
		}
	}

	// Endast bok 1 finns som fil på disk.
	bookDir := filepath.Join(dir, "Författare", "Tänkande (1)")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatalf("kunde inte skapa bokmapp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "Tänkande - Författare.epub"), []byte("epub"), 0o644); err != nil {
		t.Fatalf("kunde inte skapa bokfil: %v", err)
	}
	return dir
}

func TestBooksPrefersEpubOverOriginal(t *testing.T) {
	lib, err := Open(buildLibrary(t))
	if err != nil {
		t.Fatalf("kunde inte öppna bibliotek: %v", err)
	}
	defer lib.Close()

	books, err := lib.Books()
	if err != nil {
		t.Fatalf("kunde inte lista böcker: %v", err)
	}
	if len(books) != 3 {
		t.Fatalf("väntade 3 böcker, fick %d", len(books))
	}
	first := books[0]
	if first.Format != FormatEPUB {
		t.Errorf("väntade EPUB, fick %q", first.Format)
	}
	if first.Author() != "Författare" {
		t.Errorf("väntade författaren Författare, fick %q", first.Author())
	}
	expected := filepath.Join(lib.Dir(), "Författare", "Tänkande (1)", "Tänkande - Författare.epub")
	if first.Path != expected {
		t.Errorf("fel sökväg:\n  vill ha %q\n  fick   %q", expected, first.Path)
	}
}

func TestEPUBsSkipsBooksWithoutFile(t *testing.T) {
	lib, err := Open(buildLibrary(t))
	if err != nil {
		t.Fatalf("kunde inte öppna bibliotek: %v", err)
	}
	defer lib.Close()

	epubs, err := lib.EPUBs()
	if err != nil {
		t.Fatalf("kunde inte lista e-puber: %v", err)
	}
	if len(epubs) != 1 {
		t.Fatalf("väntade 1 e-pub, fick %d", len(epubs))
	}
	if epubs[0].Title != "Tänkande" {
		t.Errorf("fel bok: %q", epubs[0].Title)
	}
}

func TestOpenFailsWithoutMetadataDB(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("väntade ett fel när metadata.db saknas")
	}
}

func TestDefaultPathUsesEnvironment(t *testing.T) {
	t.Setenv("LEARN_CALIBRE_LIBRARY", "/tmp/synt-bibliotek")
	if got := DefaultPath(); got != "/tmp/synt-bibliotek" {
		t.Errorf("väntade /tmp/synt-bibliotek, fick %q", got)
	}
}
