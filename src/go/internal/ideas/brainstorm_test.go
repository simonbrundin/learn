package ideas

import (
	"strings"
	"testing"

	"github.com/simon/repo-template/internal/knowledge"
	"github.com/simon/repo-template/internal/model"
)

func testConcepts() []model.Concept {
	return []model.Concept{
		{ID: 1, BookID: 1, Name: "System 1", Definition: "Snabbt tanke", Importance: 0.9},
		{ID: 2, BookID: 1, Name: "System 2", Definition: "Långsamt tanke", Importance: 0.8},
	}
}

func TestBrainstormProducesIdeas(t *testing.T) {
	concepts := testConcepts()
	graph := knowledge.NewGraph()
	graph.AddNode(concepts[0])
	graph.AddNode(concepts[1])
	graph.AddEdge(1, 2, knowledge.RelationContrast, 3.0, 1)

	ideas := Brainstorm("", concepts, graph, 6, 1)
	if len(ideas) == 0 {
		t.Fatal("inga idéer genererades")
	}
	found := false
	for _, idea := range ideas {
		if idea.Kind == KindContradiction {
			found = true
		}
		if !strings.Contains(idea.Prompt, "System") {
			t.Fatalf("idén saknar koppling till koncepten: %q", idea.Prompt)
		}
	}
	if !found {
		t.Fatalf("hittade ingen motsatsidé: %v", ideas)
	}
}

func TestBrainstormRespectsLimit(t *testing.T) {
	ideas := Brainstorm("tänkande", testConcepts(), knowledge.NewGraph(), 3, 1)
	if len(ideas) > 3 {
		t.Fatalf("för många idéer: %d", len(ideas))
	}
}

func TestBrainstormWithoutConcepts(t *testing.T) {
	if ideas := Brainstorm("x", nil, nil, 5, 1); len(ideas) != 0 {
		t.Fatalf("förväntade inga idéer, fick %d", len(ideas))
	}
}
