package learning

import (
	"strings"
	"testing"
	"time"

	"github.com/simon/repo-template/internal/knowledge"
	"github.com/simon/repo-template/internal/model"
)

func testConcepts() []model.Concept {
	return []model.Concept{
		{
			ID: 1, BookID: 1, Name: "System 1",
			Definition: "Det snabba, automatiska tänket.",
			Summary:    "Automatisk och snabb bearbetning.",
			Keywords:   []string{"snabb", "automatisk"}, Importance: 0.9,
		},
		{
			ID: 2, BookID: 1, Name: "System 2",
			Definition: "Det långsamma, ansträngande tänket.",
			Summary:    "Medveten och långsam bearbetning.",
			Keywords:   []string{"långsam", "medveten"}, Importance: 0.7,
		},
		{
			ID: 3, BookID: 1, Name: "Uppmärksamhet",
			Definition: "Den begränsade resurs som System 2 behöver.",
			Summary:    "En begränsad resurs som måste prioriteras.",
			Keywords:   []string{"resurs", "begränsad"}, Importance: 0.5,
		},
		{
			ID: 4, BookID: 1, Name: "Fördom",
			Definition: "En systematisk skevhet i tänkandet.",
			Summary:    "Systematisk skevhet som påverkar beslut.",
			Keywords:   []string{"skevhet", "beslut"}, Importance: 0.4,
		},
	}
}

func TestGenerateMultipleChoice(t *testing.T) {
	engine := NewQuizEngine(7)
	questions := engine.Generate(testConcepts(), 6, []string{KindMultipleChoice})
	if len(questions) != 6 {
		t.Fatalf("förväntade 6 frågor, fick %d", len(questions))
	}
	for _, question := range questions {
		if len(question.Options) < 3 {
			t.Fatalf("för få alternativ: %v", question.Options)
		}
		found := false
		for _, option := range question.Options {
			if option == question.Answer {
				found = true
			}
		}
		if !found {
			t.Fatalf("svaret finns inte bland alternativen: %v", question.Options)
		}
	}
}

func TestGenerateWithoutConceptsReturnsNothing(t *testing.T) {
	if questions := NewQuizEngine(1).Generate(nil, 5, StandardKinds); len(questions) != 0 {
		t.Fatalf("förväntade inga frågor, fick %d", len(questions))
	}
}

func TestGenerateCoversAllKinds(t *testing.T) {
	questions := NewQuizEngine(1).Generate(testConcepts(), 4, []string{KindShortAnswer, KindEssay})
	kinds := map[string]bool{}
	for _, question := range questions {
		kinds[question.Kind] = true
	}
	if !kinds[KindShortAnswer] || !kinds[KindEssay] {
		t.Fatalf("förväntade både kortfråga och essay, fick %v", kinds)
	}
}

func TestSchedulerGrowsIntervals(t *testing.T) {
	scheduler := Scheduler{}
	first := scheduler.Update(nil, true)
	if first.IntervalDays != 1 {
		t.Fatalf("förväntade 1 dag, fick %v", first.IntervalDays)
	}
	second := scheduler.Update(&first, true)
	if second.IntervalDays != 3 {
		t.Fatalf("förväntade 3 dagar, fick %v", second.IntervalDays)
	}
	third := scheduler.Update(&second, true)
	if third.IntervalDays <= second.IntervalDays {
		t.Fatalf("intervallet växte inte: %v -> %v", second.IntervalDays, third.IntervalDays)
	}
}

func TestSchedulerResetsOnWrongAnswer(t *testing.T) {
	scheduler := Scheduler{}
	correct := scheduler.Update(nil, true)
	wrong := scheduler.Update(&correct, false)
	if wrong.Repetitions != 0 {
		t.Fatalf("förväntade noll repetitioner, fick %d", wrong.Repetitions)
	}
	if wrong.DueAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("frågan borde vara förfallen direkt: %v", wrong.DueAt)
	}
}

func TestDueQuestionsPrefersUnanswered(t *testing.T) {
	questions := NewQuizEngine(3).Generate(testConcepts(), 2, []string{KindMultipleChoice})
	for index := range questions {
		questions[index].ID = int64(index + 1)
	}
	attempts := map[int64]model.Attempt{
		1: {QuestionID: 1, DueAt: time.Now().Add(10000 * time.Hour)},
	}
	due := DueQuestions(questions, attempts, time.Now())
	if len(due) != 1 || due[0].ID != 2 {
		t.Fatalf("förväntade endast fråga 2, fick %v", due)
	}
}

func TestExplainAdaptsToLevel(t *testing.T) {
	concepts := testConcepts()
	novice := Explain(concepts[0], LevelNovice, nil, 4)
	advanced := Explain(concepts[0], LevelAdvanced, nil, 4)
	if novice.Level != LevelNovice {
		t.Fatalf("oväntad nivå: %q", novice.Level)
	}
	if len(advanced.KeyPoints) < len(novice.KeyPoints) {
		t.Fatalf("advanced-nivån borde ha fler punkter: %d < %d",
			len(advanced.KeyPoints), len(novice.KeyPoints))
	}
}

func TestExplainIncludesRelatedConcepts(t *testing.T) {
	concepts := testConcepts()
	graph := knowledge.NewGraph()
	graph.AddNode(concepts[0])
	graph.AddNode(concepts[1])
	graph.AddEdge(1, 2, knowledge.RelationContrast, 2.0, 1)
	explanation := Explain(concepts[0], LevelStandard, graph, 4)
	found := false
	for _, related := range explanation.Related {
		if strings.Contains(related.Name, "System 2") {
			found = true
		}
	}
	if !found {
		t.Fatalf("relaterade koncept saknas: %v", explanation.Related)
	}
}

func TestRankConceptsPrefersNameMatch(t *testing.T) {
	ranked := RankConcepts("Vad är System 2?", testConcepts(), nil, 3)
	if len(ranked) == 0 || ranked[0].Name != "System 2" {
		t.Fatalf("oväntad rangordning: %v", ranked)
	}
}

func TestAnswerFallsBackWhenNothingMatches(t *testing.T) {
	result := Answer("xyzzy ofte", testConcepts(), nil, nil)
	if len(result.Concepts) != 0 {
		t.Fatalf("förväntade inga träffar, fick %d", len(result.Concepts))
	}
	if !strings.Contains(result.Text, "hittade inget") {
		t.Fatalf("oväntat svar: %q", result.Text)
	}
}

func TestAnswerUsesGraphRelations(t *testing.T) {
	concepts := testConcepts()
	graph := knowledge.NewGraph()
	for _, concept := range concepts {
		graph.AddNode(concept)
	}
	graph.AddEdge(1, 2, knowledge.RelationContrast, 3.0, 1)
	result := Answer("Vad är System 1?", concepts, graph, nil)
	if !strings.Contains(result.Text, "Kopplar till") {
		t.Fatalf("kopplingar saknas i svaret: %q", result.Text)
	}
}
