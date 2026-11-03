package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/simon/repo-template/internal/model"
)

func openStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna databasen: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAddAndListBooks(t *testing.T) {
	st := openStore(t)
	book := model.Book{Title: "Thinking Fast and Slow", Author: "Kahneman", Language: "en"}
	if err := st.AddBook(&book); err != nil {
		t.Fatalf("kunde inte spara boken: %v", err)
	}
	if book.ID == 0 {
		t.Fatal("boken fick inget id")
	}
	books, err := st.ListBooks()
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Title != "Thinking Fast and Slow" {
		t.Fatalf("oväntade böcker: %v", books)
	}
}

func TestFindBookByTitleAndID(t *testing.T) {
	st := openStore(t)
	book := model.Book{Title: "Thinking Fast and Slow", Author: "Kahneman"}
	if err := st.AddBook(&book); err != nil {
		t.Fatal(err)
	}
	found, err := st.FindBook("kahneman")
	if err != nil || found == nil {
		t.Fatalf("hittade inte boken på författare: %v", err)
	}
	found, err = st.FindBook("Thinking")
	if err != nil || found == nil {
		t.Fatalf("hittade inte boken på titel: %v", err)
	}
	missing, err := st.FindBook("obefintlig")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("fann en bok som inte finns: %v", missing)
	}
}

func TestChaptersAndConceptsRoundTrip(t *testing.T) {
	st := openStore(t)
	book := model.Book{Title: "Bok"}
	if err := st.AddBook(&book); err != nil {
		t.Fatal(err)
	}
	chapter := model.Chapter{BookID: book.ID, Index: 0, Title: "Kapitel 1", Text: "Text"}
	if err := st.AddChapter(&chapter); err != nil {
		t.Fatal(err)
	}
	concepts := []model.Concept{
		{BookID: book.ID, ChapterID: chapter.ID, Name: "System 1", Definition: "Snabbt",
			Keywords: []string{"snabb"}, Quotes: []string{"citat"}, Importance: 0.8, Mentions: 3},
	}
	if err := st.AddConcepts(concepts); err != nil {
		t.Fatal(err)
	}
	if concepts[0].ID == 0 {
		t.Fatal("konceptet fick inget id")
	}
	loaded, err := st.Concepts(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Name != "System 1" {
		t.Fatalf("oväntade koncept: %v", loaded)
	}
	if len(loaded[0].Keywords) != 1 || loaded[0].Keywords[0] != "snabb" {
		t.Fatalf("nyckelord förlorades: %v", loaded[0].Keywords)
	}
	chapters, err := st.Chapters(book.ID)
	if err != nil || len(chapters) != 1 {
		t.Fatalf("oväntade kapitel: %v %v", chapters, err)
	}
}

func TestEdgesAreStoredOnce(t *testing.T) {
	st := openStore(t)
	book := model.Book{Title: "Bok"}
	if err := st.AddBook(&book); err != nil {
		t.Fatal(err)
	}
	concepts := []model.Concept{
		{BookID: book.ID, Name: "A"}, {BookID: book.ID, Name: "B"},
	}
	if err := st.AddConcepts(concepts); err != nil {
		t.Fatal(err)
	}
	edge := model.Edge{SourceID: concepts[0].ID, TargetID: concepts[1].ID, Relation: "related", Weight: 1, BookID: book.ID}
	if err := st.AddEdge(edge); err != nil {
		t.Fatal(err)
	}
	if err := st.AddEdge(edge); err != nil {
		t.Fatal(err)
	}
	edges, err := st.Edges(book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("förväntade en relation, fick %d", len(edges))
	}
}

func TestQuestionsAndAttempts(t *testing.T) {
	st := openStore(t)
	book := model.Book{Title: "Bok"}
	if err := st.AddBook(&book); err != nil {
		t.Fatal(err)
	}
	questions := []model.Question{
		{BookID: book.ID, ConceptID: 1, Kind: "multiple_choice", Prompt: "Vad betyder X?",
			Options: []string{"a", "b"}, Answer: "a", Explanation: "förklaring", Difficulty: "easy"},
	}
	if err := st.AddQuestions(questions); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.Questions(book.ID)
	if err != nil || len(loaded) != 1 {
		t.Fatalf("oväntade frågor: %v %v", loaded, err)
	}
	if len(loaded[0].Options) != 2 {
		t.Fatalf("alternativen förlorades: %v", loaded[0].Options)
	}

	attempt := model.Attempt{QuestionID: loaded[0].ID, ConceptID: 1, Correct: true,
		AnsweredAt: nowUTC(), EaseFactor: 2.5, IntervalDays: 1, DueAt: nowUTC().AddDate(0, 0, 1)}
	if err := st.AddAttempt(&attempt); err != nil {
		t.Fatal(err)
	}
	stored, ok := st.LatestAttempt(loaded[0].ID)
	if !ok {
		t.Fatal("inget svar sparat")
	}
	if !stored.Correct {
		t.Fatal("svaret sparades som felaktigt")
	}
}

func nowUTC() time.Time { return time.Now().UTC() }
