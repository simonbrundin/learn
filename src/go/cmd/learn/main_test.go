package main

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/simon/repo-template/internal/testutil"
)

// buildCLI bygger learn-binären en gång och returnerar sökvägen till den.
func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "learn")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Env = os.Environ()
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("kunde inte bygga CLI: %v\n%s", err, output)
	}
	return binary
}

func runCLI(t *testing.T, binary string, stdin string, args ...string) string {
	t.Helper()
	command := exec.Command(binary, args...)
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("kommandot %v misslyckades: %v\n%s", args, err, output)
	}
	return string(output)
}

func TestCLIImportListAndAsk(t *testing.T) {
	binary := buildCLI(t)
	epubPath, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "cli.db")

	output := runCLI(t, binary, "", "--db", db, "import", epubPath)
	if !strings.Contains(output, "Thinking About Thinking") {
		t.Fatalf("importutskriften saknar titeln: %q", output)
	}

	output = runCLI(t, binary, "", "--db", db, "list")
	if !strings.Contains(output, "Thinking About Thinking") {
		t.Fatalf("listutskriften saknar titeln: %q", output)
	}

	output = runCLI(t, binary, "", "--db", db, "ask", "Vad är System 1?")
	if !strings.Contains(output, "System 1") {
		t.Fatalf("svaret saknar konceptet: %q", output)
	}
}

func TestCLIExplainAndBrainstorm(t *testing.T) {
	binary := buildCLI(t)
	epubPath, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "cli.db")
	runCLI(t, binary, "", "--db", db, "import", epubPath)

	output := runCLI(t, binary, "", "--db", db, "explain", "System 1")
	if !strings.Contains(output, "Relaterade koncept") {
		t.Fatalf("förklaringen saknar relaterade koncept: %q", output)
	}

	output = runCLI(t, binary, "", "--db", db, "brainstorm", "tänkande")
	if !strings.Contains(output, "Idéer") {
		t.Fatalf("brainstorm saknar idéer: %q", output)
	}
}

func TestCLIShowAndConnect(t *testing.T) {
	binary := buildCLI(t)
	epubPath, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "cli.db")
	runCLI(t, binary, "", "--db", db, "import", epubPath)

	output := runCLI(t, binary, "", "--db", db, "show", "1")
	if !strings.Contains(output, "Toppkoncept") {
		t.Fatalf("show saknar toppkoncept: %q", output)
	}

	output = runCLI(t, binary, "", "--db", db, "connect", "System 1", "System 2")
	if !strings.Contains(output, "System 1") {
		t.Fatalf("connect saknar kopplingsvägen: %q", output)
	}
}

func TestCLIQuizAcceptsAnswers(t *testing.T) {
	binary := buildCLI(t)
	epubPath, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "cli.db")
	runCLI(t, binary, "", "--db", db, "import", epubPath)

	output := runCLI(t, binary, "1\nskip\n", "--db", db, "quiz", "--count", "2")
	if !strings.Contains(output, "Kunskapstest") {
		t.Fatalf("quiz saknar rubriken: %q", output)
	}
	if !strings.Contains(output, "Resultat:") {
		t.Fatalf("quiz saknar resultatet: %q", output)
	}
}

func TestCLISyncImportsCalibreLibrary(t *testing.T) {
	binary := buildCLI(t)
	library := t.TempDir()

	// Ett minimalt Calibre-bibliotek med en riktig e-pub i.
	if _, err := testutil.BuildEpub(filepath.Join(library, "Bok (1)")); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(library, "metadata.db"))
	if err != nil {
		t.Fatal(err)
	}
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
			t.Fatal(err)
		}
	}
	db.Close()

	learnDB := filepath.Join(t.TempDir(), "cli.db")
	output := runCLI(t, binary, "", "--db", learnDB, "sync", "--library", library)
	if !strings.Contains(output, "Thinking About Thinking") {
		t.Fatalf("synkutskriften saknar titeln: %q", output)
	}
	if !strings.Contains(output, "1 nya") {
		t.Fatalf("synkutskriften saknar sammanfattningen: %q", output)
	}

	output = runCLI(t, binary, "", "--db", learnDB, "list")
	if !strings.Contains(output, "Thinking About Thinking") {
		t.Fatalf("listutskriften saknar titeln: %q", output)
	}

	// En andra körning ska hoppa över boken.
	output = runCLI(t, binary, "", "--db", learnDB, "sync", "--library", library)
	if !strings.Contains(output, "1 redan synkade") {
		t.Fatalf("andra körningen synkade inte om boken: %q", output)
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	binary := buildCLI(t)
	command := exec.Command(binary, "finns-inte")
	// Testet ska inte röra den delade standarddatabasen.
	command.Env = append(os.Environ(), "LEARN_DB="+filepath.Join(t.TempDir(), "unknown.db"))
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("väntade fel för okänt kommando")
	}
	if !strings.Contains(string(output), "Okänt kommando") {
		t.Fatalf("oväntad utskrift: %q", output)
	}
}
