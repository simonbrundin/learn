package learn

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/simon/repo-template/internal/store"
	"github.com/simon/repo-template/internal/testutil"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "learn.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna databasen: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestImportBookPopulatesStore(t *testing.T) {
	st := openStore(t)
	path, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := ImportBook(path, st, 60)
	if err != nil {
		t.Fatalf("importen misslyckades: %v", err)
	}
	if result.Book.Title != "Thinking About Thinking" {
		t.Fatalf("oväntad titel: %q", result.Book.Title)
	}
	if len(result.Chapters) != 2 {
		t.Fatalf("förväntade 2 kapitel, fick %d", len(result.Chapters))
	}
	if len(result.Concepts) == 0 {
		t.Fatal("inga koncept extraherades")
	}
	books, _ := st.ListBooks()
	if len(books) != 1 {
		t.Fatalf("förväntade en bok i databasen, fick %d", len(books))
	}
	concepts, _ := st.Concepts(result.Book.ID)
	if len(concepts) != len(result.Concepts) {
		t.Fatalf("koncepten sparades inte: %d != %d", len(concepts), len(result.Concepts))
	}
	if result.Edges == 0 {
		t.Fatal("inga relationer skapades")
	}
}

func TestImportedConceptsAreSearchable(t *testing.T) {
	st := openStore(t)
	path, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportBook(path, st, 60); err != nil {
		t.Fatal(err)
	}
	concept, found := st.FindConcept("System 2", 0)
	if !found {
		t.Fatal("hittade inte System 2")
	}
	if !strings.Contains(strings.ToLower(concept.Definition), "slow") {
		t.Fatalf("oväntad definition: %q", concept.Definition)
	}
}

func TestBuildGraphRestoresRelations(t *testing.T) {
	st := openStore(t)
	path, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := ImportBook(path, st, 60)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildGraph(st, result.Book.ID)
	if err != nil {
		t.Fatal(err)
	}
	concepts, edges := graph.Stats()
	if concepts == 0 || edges == 0 {
		t.Fatalf("grafen kunde inte återbyggas: %d koncept, %d kanter", concepts, edges)
	}
}

func TestImportFailsForMissingFile(t *testing.T) {
	st := openStore(t)
	if _, err := ImportBook(filepath.Join(t.TempDir(), "saknas.epub"), st, 60); err == nil {
		t.Fatal("väntade fel för saknad fil")
	}
}
