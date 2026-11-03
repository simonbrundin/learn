package store

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/simon/repo-template/internal/model"
)

// postgresTestURL ger adressen till testdatabasen.
// Utan LEARN_TEST_DATABASE_URL hoppas testerna över.
func postgresTestURL(t *testing.T) string {
	dsn := strings.TrimSpace(os.Getenv("LEARN_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("LEARN_TEST_DATABASE_URL är inte satt, hoppar över PostgreSQL-testerna")
	}
	return dsn
}

// openTestPostgres skapar en egen databas för testet.
func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	base := postgresTestURL(t)
	name := fmt.Sprintf("learn_test_%d", time.Now().UnixNano())

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("kunde inte ansluta: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("kunde inte skapa testdatabas: %v", err)
	}

	dsn := replaceDatabase(base, name)
	repo, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("kunde inte öppna testdatabasen: %v", err)
	}
	t.Cleanup(func() {
		repo.Close()
		cleanup, err := sql.Open("pgx", base)
		if err == nil {
			cleanup.Exec("DROP DATABASE IF EXISTS " + name)
			cleanup.Close()
		}
	})
	return repo
}

// replaceDatabase byter ut databasnamnet i en DSN.
func replaceDatabase(dsn, name string) string {
	if index := strings.LastIndex(dsn, "/"); index > 0 {
		rest := ""
		if query := strings.Index(dsn[index:], "?"); query >= 0 {
			rest = dsn[index+query:]
		}
		return dsn[:index+1] + name + rest
	}
	return dsn
}

// TestPostgresRoundtrip går igenom hela gränssnittet mot PostgreSQL.
func TestPostgresRoundtrip(t *testing.T) {
	repo := openTestPostgres(t)

	book := &model.Book{
		Title:      "Testbok",
		Author:     "Testare",
		SourcePath: "/tmp/testbok.epub",
		Language:   "sv",
		ImportedAt: time.Now(),
	}
	if err := repo.AddBook(book); err != nil {
		t.Fatalf("kunde inte lägga till boken: %v", err)
	}
	if book.ID == 0 {
		t.Fatal("boken fick inget id")
	}

	chapter := &model.Chapter{BookID: book.ID, Index: 0, Title: "Kapitel 1", Text: "Text"}
	if err := repo.AddChapter(chapter); err != nil {
		t.Fatalf("kunde inte lägga till kapitlet: %v", err)
	}

	concepts := []model.Concept{
		{BookID: book.ID, ChapterID: chapter.ID, Name: "Kunskap", Definition: "En definition.",
			Summary: "En sammanfattning.", Kind: "concept", Keywords: []string{"nyckel", "ord"},
			Quotes: []string{"ett citat"}, Importance: 0.8, Mentions: 3},
		{BookID: book.ID, ChapterID: chapter.ID, Name: "Kunskapsgraf", Definition: "En annan definition.",
			Keywords: []string{"nyckel"}, Importance: 0.4, Mentions: 1},
	}
	if err := repo.AddConcepts(concepts); err != nil {
		t.Fatalf("kunde inte lägga till koncepten: %v", err)
	}
	if concepts[0].ID == 0 {
		t.Fatal("koncepten fick inget id")
	}

	if err := repo.AddEdge(model.Edge{
		SourceID: concepts[0].ID, TargetID: concepts[1].ID, Relation: "related", Weight: 0.5, BookID: book.ID,
	}); err != nil {
		t.Fatalf("kunde inte lägga till relationen: %v", err)
	}
	// Samma relation igen ska ignoreras.
	if err := repo.AddEdge(model.Edge{
		SourceID: concepts[0].ID, TargetID: concepts[1].ID, Relation: "related", Weight: 0.5, BookID: book.ID,
	}); err != nil {
		t.Fatalf("dubblerad relation orsakade fel: %v", err)
	}

	questions := []model.Question{{
		BookID: book.ID, ConceptID: concepts[0].ID, Kind: "multiple_choice",
		Prompt: "Vad är kunskap?", Options: []string{"Ett svar", "Ett annat"}, Answer: "Ett svar",
		Explanation: "Förklaring.", Difficulty: "easy",
	}}
	if err := repo.AddQuestions(questions); err != nil {
		t.Fatalf("kunde inte lägga till frågan: %v", err)
	}

	attempt := &model.Attempt{
		QuestionID: questions[0].ID, ConceptID: concepts[0].ID, Correct: true,
		AnsweredAt: time.Now(), EaseFactor: 2.6, IntervalDays: 3, DueAt: time.Now().Add(72 * time.Hour),
	}
	if err := repo.AddAttempt(attempt); err != nil {
		t.Fatalf("kunde inte spara svaret: %v", err)
	}

	if err := repo.AddIdea(model.Idea{BookID: book.ID, Kind: "combination",
		Prompt: "Vad händer om?", Basis: []string{"Kunskap", "Kunskapsgraf"}}); err != nil {
		t.Fatalf("kunde inte spara idén: %v", err)
	}

	// Läsning
	books, err := repo.ListBooks()
	if err != nil || len(books) != 1 {
		t.Fatalf("fick %d böcker, fel %v", len(books), err)
	}
	chapters, err := repo.Chapters(book.ID)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("fick %d kapitel, fel %v", len(chapters), err)
	}
	stored, err := repo.Concepts(book.ID)
	if err != nil || len(stored) != 2 {
		t.Fatalf("fick %d koncept, fel %v", len(stored), err)
	}
	if len(stored[0].Keywords) != 2 || stored[0].Keywords[0] != "nyckel" {
		t.Fatalf("nyckelorden kom inte tillbaka: %v", stored[0].Keywords)
	}
	if len(stored[0].Quotes) != 1 {
		t.Fatalf("citaten kom inte tillbaka: %v", stored[0].Quotes)
	}
	all, err := repo.AllConcepts()
	if err != nil || len(all) != 2 {
		t.Fatalf("AllConcepts gav %d koncept, fel %v", len(all), err)
	}
	edges, err := repo.Edges(book.ID)
	if err != nil || len(edges) != 1 {
		t.Fatalf("fick %d relationer, fel %v", len(edges), err)
	}
	allEdges, err := repo.AllEdges()
	if err != nil || len(allEdges) != 1 {
		t.Fatalf("AllEdges gav %d relationer, fel %v", len(allEdges), err)
	}
	readQuestions, err := repo.Questions(book.ID)
	if err != nil || len(readQuestions) != 1 || len(readQuestions[0].Options) != 2 {
		t.Fatalf("frågorna kom inte tillbaka: %v, fel %v", readQuestions, err)
	}
	latest, ok := repo.LatestAttempt(questions[0].ID)
	if !ok || !latest.Correct || latest.Repetitions != 0 {
		t.Fatalf("svaret kom inte tillbaka: %+v", latest)
	}
	if found, ok := repo.FindConcept("Kunskap", book.ID); !ok || found.Name != "Kunskap" {
		t.Fatalf("FindConcept hittade inte konceptet: %+v", found)
	}
	if found, err := repo.FindBook("Testbok"); err != nil || found == nil {
		t.Fatalf("FindBook hittade inte boken: %+v, fel %v", found, err)
	}

	// Misslyckande importer
	failure := model.ImportFailure{SourcePath: "/tmp/fel.epub", Title: "Fel",
		Reason: "ingen text", FailedAt: time.Now()}
	if err := repo.RecordFailure(failure); err != nil {
		t.Fatalf("kunde inte spara misslyckandet: %v", err)
	}
	// Samma fel igen ska uppdatera, inte duplicera.
	if err := repo.RecordFailure(failure); err != nil {
		t.Fatalf("uppdatering av misslyckande misslyckades: %v", err)
	}
	failures, err := repo.Failures()
	if err != nil || len(failures) != 1 {
		t.Fatalf("fick %d misslyckanden, fel %v", len(failures), err)
	}
	if err := repo.ClearFailure(failure.SourcePath); err != nil {
		t.Fatalf("kunde inte rensa misslyckandet: %v", err)
	}
	failures, _ = repo.Failures()
	if len(failures) != 0 {
		t.Fatalf("misslyckandet rensades inte: %d", len(failures))
	}

	// Borttagning
	if err := repo.DeleteBook(book.ID); err != nil {
		t.Fatalf("kunde inte ta bort boken: %v", err)
	}
	books, _ = repo.ListBooks()
	if len(books) != 0 {
		t.Fatalf("boken togs inte bort: %d", len(books))
	}
	conceptsAfter, _ := repo.AllConcepts()
	if len(conceptsAfter) != 0 {
		t.Fatalf("koncepten togs inte bort: %d", len(conceptsAfter))
	}
}

// TestPostgresTimeRoundtrip kontrollerar att tidsstämplar överlever.
func TestPostgresTimeRoundtrip(t *testing.T) {
	repo := openTestPostgres(t)
	book := &model.Book{Title: "Tider", ImportedAt: time.Now().Truncate(time.Millisecond)}
	if err := repo.AddBook(book); err != nil {
		t.Fatalf("kunde inte lägga till boken: %v", err)
	}
	books, err := repo.ListBooks()
	if err != nil || len(books) != 1 {
		t.Fatalf("fick %d böcker, fel %v", len(books), err)
	}
	if diff := books[0].ImportedAt.Sub(book.ImportedAt); diff > time.Second || diff < -time.Second {
		t.Fatalf("tidsstämpeln ändrades: %v mot %v", books[0].ImportedAt, book.ImportedAt)
	}
}
