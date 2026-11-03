package epub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simon/repo-template/internal/testutil"
)

func TestParseReadsMetadata(t *testing.T) {
	path := buildEpub(t)
	book, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse misslyckades: %v", err)
	}
	if book.Title() != "Thinking About Thinking" {
		t.Fatalf("oväntad titel: %q", book.Title())
	}
	if book.Author() != "Test Author" {
		t.Fatalf("oväntad författare: %q", book.Author())
	}
	if book.Metadata.Language != "en" {
		t.Fatalf("oväntat språk: %q", book.Metadata.Language)
	}
	if !strings.Contains(book.Metadata.Description, "testbok") {
		t.Fatalf("oväntad beskrivning: %q", book.Metadata.Description)
	}
}

func TestParseKeepsSpineOrder(t *testing.T) {
	path := buildEpub(t)
	book, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse misslyckades: %v", err)
	}
	if len(book.Episodes) != 2 {
		t.Fatalf("förväntade 2 kapitel, fick %d", len(book.Episodes))
	}
	if book.Episodes[0].Title != "Fast Thinking" || book.Episodes[1].Title != "Slow Thinking" {
		t.Fatalf("oväntade kapiteltitlar: %q, %q", book.Episodes[0].Title, book.Episodes[1].Title)
	}
}

func TestParseExtractsTextWithoutTags(t *testing.T) {
	path := buildEpub(t)
	book, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse misslyckades: %v", err)
	}
	text := book.Episodes[0].Text
	if !strings.Contains(text, "System 1 is the fast") {
		t.Fatalf("texten saknar stycket: %q", text)
	}
	if strings.Contains(text, "<p>") {
		t.Fatalf("taggar finns kvar i texten: %q", text)
	}
}

func TestParseKeepsHeadingsOutOfBodyText(t *testing.T) {
	path := buildEpub(t)
	book, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse misslyckades: %v", err)
	}
	if strings.HasPrefix(book.Episodes[0].Text, "Fast Thinking") {
		t.Fatalf("rubriken hamnade i brödtexten: %q", book.Episodes[0].Text)
	}
}

func TestParseFailsForMissingFile(t *testing.T) {
	if _, err := Parse(filepath.Join(t.TempDir(), "saknas.epub")); err == nil {
		t.Fatal("väntade fel för saknad fil")
	}
}

func TestParseFailsForNonZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.epub")
	if err := os.WriteFile(path, []byte("detta är inte en epub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(path); err == nil {
		t.Fatal("väntade fel för icke-epub")
	}
}

func buildEpub(t *testing.T) string {
	t.Helper()
	path, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatalf("kunde inte bygga test-epub: %v", err)
	}
	return path
}
