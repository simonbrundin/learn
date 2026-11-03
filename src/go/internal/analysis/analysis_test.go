package analysis

import (
	"path/filepath"
	"testing"

	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
)

// seed skapar en butik med två böcker. System 1 beskrivs olika i varje
// bok, vilket är kärnan i analysen.
func seed(t *testing.T, deepWorkDefinition string) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "analysis.db"))
	if err != nil {
		t.Fatalf("kunde inte öppna butiken: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	first := model.Book{Title: "Thinking Fast and Slow", Author: "Kahneman"}
	if err := st.AddBook(&first); err != nil {
		t.Fatal(err)
	}
	second := model.Book{Title: "Deep Work", Author: "Newport"}
	if err := st.AddBook(&second); err != nil {
		t.Fatal(err)
	}

	if err := st.AddConcepts([]model.Concept{
		{
			BookID: first.ID, Name: "System 1", Importance: 0.9,
			Definition: "System 1 is fast, automatic and effortless thinking.",
			Keywords:   []string{"system 1", "automatic", "habit"},
		},
		{
			BookID: first.ID, Name: "Loss Aversion", Importance: 0.8,
			Definition: "Loss aversion is the tendency to prefer avoiding losses.",
			Keywords:   []string{"loss", "risk"},
		},
		{
			BookID: second.ID, Name: "System 1", Importance: 0.7,
			Definition: deepWorkDefinition,
			Keywords:   []string{"system 1", "automatic"},
		},
		{
			BookID: second.ID, Name: "Attention", Importance: 0.6,
			Definition: "Attention is a finite resource that gets exhausted.",
			Keywords:   []string{"attention", "focus", "system 1"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return st
}

// bookID hittar en boks id på titel.
func bookID(t *testing.T, st *store.Store, title string) int64 {
	t.Helper()
	books, err := st.ListBooks()
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range books {
		if book.Title == title {
			return book.ID
		}
	}
	t.Fatalf("boken %q hittades inte", title)
	return 0
}

func TestCompareFindsNovelAndSharedConcepts(t *testing.T) {
	st := seed(t, "System 1 is the fast and automatic thinking that needs no effort.")
	id := bookID(t, st, "Thinking Fast and Slow")

	insight, err := Compare(id, st, DefaultOptions())
	if err != nil {
		t.Fatalf("analysen misslyckades: %v", err)
	}
	if insight == nil {
		t.Fatal("analysen gav inget resultat")
	}
	if insight.Totals.Concepts != 2 {
		t.Errorf("väntade 2 koncept, fick %d", insight.Totals.Concepts)
	}
	if insight.Totals.Unique != 1 || insight.Totals.Shared != 1 {
		t.Errorf("fel fördelning: %+v", insight.Totals)
	}
	if len(insight.Novel) != 1 || insight.Novel[0].Name != "Loss Aversion" {
		t.Errorf("fel nya koncept: %+v", insight.Novel)
	}
	if len(insight.Shared) != 1 || insight.Shared[0].Name != "System 1" {
		t.Errorf("fel delade koncept: %+v", insight.Shared)
	}
	if insight.Totals.Coverage != 0.5 {
		t.Errorf("väntade täckning 0.5, fick %v", insight.Totals.Coverage)
	}
	if insight.Totals.OtherBooks != 1 {
		t.Errorf("väntade 1 jämförelsebok, fick %d", insight.Totals.OtherBooks)
	}
}

func TestCompareDetectsUnconventionalDefinition(t *testing.T) {
	st := seed(t, "System 1 is the deliberate and slow thinking that requires planning.")
	id := bookID(t, st, "Deep Work")

	insight, err := Compare(id, st, DefaultOptions())
	if err != nil {
		t.Fatalf("analysen misslyckades: %v", err)
	}
	found := false
	for _, item := range insight.Unconventional {
		if item.Name == "System 1" {
			found = true
			if item.Verdict != "unconventional" {
				t.Errorf("väntade avvikande bedömning, fick %q", item.Verdict)
			}
			if item.Note == "" {
				t.Error("avvikelsen saknar förklaring")
			}
		}
	}
	if !found {
		t.Fatalf("hittade ingen avvikelse: %+v", insight.Unconventional)
	}
}

func TestCompareFindsContradiction(t *testing.T) {
	st := seed(t, "System 1 is fast and automatic thinking that never needs any effort.")
	id := bookID(t, st, "Deep Work")

	insight, err := Compare(id, st, DefaultOptions())
	if err != nil {
		t.Fatalf("analysen misslyckades: %v", err)
	}
	if insight.Totals.Contradicting == 0 {
		t.Fatalf("väntade en motsägelse: %+v", insight.Unconventional)
	}
}

func TestCompareFindsCrossLinks(t *testing.T) {
	st := seed(t, "System 1 is the fast and automatic thinking that needs no effort.")
	id := bookID(t, st, "Thinking Fast and Slow")

	insight, err := Compare(id, st, DefaultOptions())
	if err != nil {
		t.Fatalf("analysen misslyckades: %v", err)
	}
	if len(insight.CrossLinks) == 0 {
		t.Fatal("hittade inga korskopplingar")
	}
	link := insight.CrossLinks[0]
	if link.BookTitle != "Deep Work" {
		t.Errorf("korskopplingen pekar på %q", link.BookTitle)
	}
	if len(link.Keywords) == 0 {
		t.Error("korskopplingen saknar nyckelord")
	}
}

func TestLibrarySummarizesCorpus(t *testing.T) {
	st := seed(t, "System 1 is the fast and automatic thinking that needs no effort.")
	insight, err := Library(st)
	if err != nil {
		t.Fatalf("sammanfattningen misslyckades: %v", err)
	}
	if insight.Books != 2 {
		t.Errorf("väntade 2 böcker, fick %d", insight.Books)
	}
	if insight.Concepts != 4 {
		t.Errorf("väntade 4 koncept, fick %d", insight.Concepts)
	}
	if insight.SharedNames != 1 {
		t.Errorf("väntade 1 delat namn, fick %d", insight.SharedNames)
	}
}

func TestCompareCountsNovelKeywordsAcrossBooks(t *testing.T) {
	st := seed(t, "System 1 is the fast and automatic thinking that needs no effort.")
	// Ett nyckelord som förekommer i tre böcker räknas inte som nytt,
	// men indexet över nyckelord får inte skadas av beräkningen.
	first := bookID(t, st, "Thinking Fast and Slow")
	second := bookID(t, st, "Deep Work")
	concepts, _ := st.Concepts(first)
	for i := range concepts {
		if concepts[i].Name == "System 1" {
			concepts[i].Keywords = []string{"system 1", "shared", "habit"}
		}
	}
	if err := st.AddConcepts(concepts); err != nil {
		t.Fatal(err)
	}
	more, _ := st.Concepts(second)
	for i := range more {
		if more[i].Name == "System 1" {
			more[i].Keywords = []string{"system 1", "shared"}
		}
	}
	if err := st.AddConcepts(more); err != nil {
		t.Fatal(err)
	}

	insight, err := Compare(first, st, DefaultOptions())
	if err != nil {
		t.Fatalf("analysen misslyckades: %v", err)
	}
	// "shared" finns i båda böckerna och får inte räknas som nytt.
	for _, novel := range insight.Novel {
		for _, keyword := range novel.NovelKeywords {
			if keyword == "shared" {
				t.Error("nyckelord som finns i två böcker räknades som nytt")
			}
		}
	}
	for _, shared := range insight.Shared {
		for _, keyword := range shared.NovelKeywords {
			if keyword == "shared" {
				t.Error("nyckelord som finns i två böcker räknades som nytt")
			}
		}
	}
	// "habit" finns bara i boken och ska räknas som nytt.
	found := false
	for _, novel := range insight.Novel {
		for _, keyword := range novel.NovelKeywords {
			if keyword == "habit" {
				found = true
			}
		}
	}
	for _, shared := range insight.Shared {
		for _, keyword := range shared.NovelKeywords {
			if keyword == "habit" {
				found = true
			}
		}
	}
	if !found {
		t.Error("nyckelord som bara finns i boken räknades inte som nytt")
	}
}

func TestCompareUnknownBookReturnsNil(t *testing.T) {
	st := seed(t, "System 1 is the fast and automatic thinking that needs no effort.")
	insight, err := Compare(999, st, DefaultOptions())
	if err != nil {
		t.Fatalf("analysen misslyckades: %v", err)
	}
	if insight != nil {
		t.Error("väntade inget resultat för okänd bok")
	}
}
