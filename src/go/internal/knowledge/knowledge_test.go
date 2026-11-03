package knowledge

import (
	"strings"
	"testing"

	"github.com/simon/repo-template/internal/epub"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/testutil"
)

func extract(t *testing.T) ([]model.Concept, map[string][]string) {
	t.Helper()
	path, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatalf("kunde inte bygga test-epub: %v", err)
	}
	book, err := epub.Parse(path)
	if err != nil {
		t.Fatalf("kunde inte parsa test-epub: %v", err)
	}
	extractor := NewConceptExtractor(20)
	concepts := extractor.Extract(book.Episodes, 1)
	for index := range concepts {
		concepts[index].ID = int64(index + 1)
	}
	return concepts, extractor.Contexts()
}

func findConcept(concepts []model.Concept, prefix string) (model.Concept, bool) {
	for _, concept := range concepts {
		if strings.HasPrefix(strings.ToLower(concept.Name), prefix) {
			return concept, true
		}
	}
	return model.Concept{}, false
}

func TestExtractFindsKnownConcepts(t *testing.T) {
	concepts, _ := extract(t)
	if _, ok := findConcept(concepts, "system 1"); !ok {
		t.Fatalf("hittade inte System 1 bland %v", names(concepts))
	}
	if _, ok := findConcept(concepts, "system 2"); !ok {
		t.Fatalf("hittade inte System 2 bland %v", names(concepts))
	}
}

func TestExtractAttachesDefinitions(t *testing.T) {
	concepts, _ := extract(t)
	system1, ok := findConcept(concepts, "system 1")
	if !ok {
		t.Fatal("hittade inte System 1")
	}
	if !strings.Contains(system1.Definition, "fast") {
		t.Fatalf("oväntad definition: %q", system1.Definition)
	}
	if system1.Kind != "concept" {
		t.Fatalf("oväntad typ: %q", system1.Kind)
	}
	if system1.Importance <= 0 {
		t.Fatalf("konceptet saknar vikt: %v", system1.Importance)
	}
}

func TestExtractCollectsContexts(t *testing.T) {
	_, contexts := extract(t)
	found := false
	for key, sentences := range contexts {
		if !strings.Contains(key, "system 1") {
			continue
		}
		for _, sentence := range sentences {
			if strings.Contains(sentence, "System 1") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("kontexten för System 1 saknar meningar")
	}
}

func TestExtractSkipsGenericTerms(t *testing.T) {
	episodes := []model.Episode{{
		Title: "Notes",
		Text: "Pro Tip: Install the package from Git. Key Terms: the build is ready. " +
			"Gherkin is a language used to describe expected behaviors of an application.",
	}}
	extractor := NewConceptExtractor(10)
	concepts := extractor.Extract(episodes, 1)
	for _, concept := range concepts {
		switch strings.ToLower(concept.Name) {
		case "pro tip", "key terms", "build", "notes":
			t.Fatalf("generiskt uttryck blev koncept: %q", concept.Name)
		}
	}
	if _, ok := findConcept(concepts, "gherkin"); !ok {
		t.Fatalf("hittade inte Gherkin bland %v", names(concepts))
	}
}

func TestExtractStripsFootnoteNumbers(t *testing.T) {
	episodes := []model.Episode{{
		Title: "Libraries",
		Text: "Ruby Capybara 17 is a popular acceptance testing tool in Ruby. " +
			"Ruby ActiveRecord1 is a popular ORM in Ruby.",
	}}
	extractor := NewConceptExtractor(10)
	concepts := extractor.Extract(episodes, 1)
	for _, concept := range concepts {
		if strings.Contains(concept.Name, "17") || strings.Contains(concept.Name, "ActiveRecord1") {
			t.Fatalf("fotnotsmarkören finns kvar i %q", concept.Name)
		}
	}
}

func TestGraphLinksRelatedConcepts(t *testing.T) {
	concepts, contexts := extract(t)
	graph := NewGraph()
	graph.BuildFromContexts(concepts, contexts, 1)
	conceptsCount, edges := graph.Stats()
	if conceptsCount == 0 || edges == 0 {
		t.Fatalf("grafen är tom: %d koncept, %d kanter", conceptsCount, edges)
	}
	system1, _ := findConcept(concepts, "system 1")
	neighbours := graph.Neighbours(system1.ID)
	found := false
	for _, neighbour := range neighbours {
		if strings.Contains(strings.ToLower(neighbour.Concept.Name), "system 2") {
			found = true
		}
	}
	if !found {
		t.Fatalf("System 1 saknar grannen System 2: %v", neighbours)
	}
}

func TestGraphDetectsContrastRelation(t *testing.T) {
	concepts, contexts := extract(t)
	graph := NewGraph()
	graph.BuildFromContexts(concepts, contexts, 1)
	relations := map[string]bool{}
	for _, edge := range graph.Edges() {
		relations[edge.Relation] = true
	}
	if !relations[RelationContrast] {
		t.Fatalf("hittade ingen motsatsrelation: %v", relations)
	}
}

func TestGraphFindsPath(t *testing.T) {
	concepts, contexts := extract(t)
	graph := NewGraph()
	graph.BuildFromContexts(concepts, contexts, 1)
	system1, _ := findConcept(concepts, "system 1")
	system2, _ := findConcept(concepts, "system 2")
	path := graph.FindPath(system1.ID, system2.ID)
	if len(path) == 0 {
		t.Fatal("fann ingen kopplingsväg")
	}
	if path[0].ID != system1.ID || path[len(path)-1].ID != system2.ID {
		t.Fatalf("oväntad väg: %v", names(path))
	}
}

func names(concepts []model.Concept) []string {
	result := make([]string, 0, len(concepts))
	for _, concept := range concepts {
		result = append(result, concept.Name)
	}
	return result
}
