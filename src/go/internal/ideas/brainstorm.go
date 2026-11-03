// Package ideas genererar idéprompts utifrån inlärd kunskap.
package ideas

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/simon/repo-template/internal/knowledge"
	"github.com/simon/repo-template/internal/model"
)

// Idékategorier.
const (
	KindCombination   = "combination"
	KindContradiction = "contradiction"
	KindCausal        = "causal"
	KindCrossBook     = "cross_book"
	KindApplication   = "application"
	KindInversion     = "inversion"
	KindSeed          = "seed"
)

var applicationTemplates = []string{
	"Tillämpa %s på ett problem du arbetar med just nu: vad skulle du ändra?",
	"Skriv ett exempel där %s förutsäger något som faktiskt händer.",
	"Förklara för någon oväntig hur %s påverkar deras vardag.",
}

const inversionTemplate = "Vad händer om %s visar sig vara fel? Vilka antaganden faller då?"

const combinationTemplate = "Vad händer om du kombinerar %s och %s – finns det en gemensam mekanism?"

const contrastTemplate = "%s och %s verkar motsäga varandra. Vem har rätt, och beroende på vad?"

const causeTemplate = "Om %s leder till %s, vilka andra konsekvenser kan följa av samma samband?"

const crossBookTemplate = "Hur skiljer sig %s i %s från samma idé i %s?"

// Brainstorm genererar idéprompts utifrån kunskapsbasen.
func Brainstorm(seed string, concepts []model.Concept, graph *knowledge.Graph, limit int, seedValue int64) []model.Idea {
	if limit <= 0 {
		limit = 6
	}
	rng := rand.New(rand.NewSource(seedValue))
	var ideas []model.Idea

	usable := make([]model.Concept, 0, len(concepts))
	for _, concept := range concepts {
		if concept.Name != "" {
			usable = append(usable, concept)
		}
	}
	if len(usable) == 0 {
		return nil
	}

	strong := make([]model.Concept, len(usable))
	copy(strong, usable)
	sortByImportance(strong)
	if len(strong) > 6 {
		strong = strong[:6]
	}

	for i := 0; i+1 < len(strong) && i < 2; i++ {
		left, right := strong[i], strong[i+1]
		ideas = append(ideas, model.Idea{
			BookID: left.BookID,
			Kind:   KindCombination,
			Prompt: fmt.Sprintf(combinationTemplate, left.Name, right.Name),
			Basis:  []string{left.Name, right.Name},
		})
	}

	if graph != nil {
		for _, edge := range graph.Edges() {
			source, okSource := graph.Node(edge.SourceID)
			target, okTarget := graph.Node(edge.TargetID)
			if !okSource || !okTarget {
				continue
			}
			switch edge.Relation {
			case knowledge.RelationContrast:
				ideas = append(ideas, model.Idea{
					BookID: source.BookID,
					Kind:   KindContradiction,
					Prompt: fmt.Sprintf(contrastTemplate, source.Name, target.Name),
					Basis:  []string{source.Name, target.Name},
				})
			case knowledge.RelationCause, knowledge.RelationEffect:
				ideas = append(ideas, model.Idea{
					BookID: source.BookID,
					Kind:   KindCausal,
					Prompt: fmt.Sprintf(causeTemplate, source.Name, target.Name),
					Basis:  []string{source.Name, target.Name},
				})
			}
			if len(ideas) >= limit {
				break
			}
		}
	}

	if graph != nil {
		for _, link := range graph.CrossBookLinks(usable) {
			ideas = append(ideas, model.Idea{
				BookID: link[0].BookID,
				Kind:   KindCrossBook,
				Prompt: fmt.Sprintf(crossBookTemplate, link[0].Name, bookLabel(link[0]), bookLabel(link[1])),
				Basis:  []string{link[0].Name, link[1].Name},
			})
			if len(ideas) >= limit {
				break
			}
		}
	}

	for _, concept := range strong {
		if len(concept.Keywords) == 0 && concept.Definition == "" {
			continue
		}
		template := applicationTemplates[rng.Intn(len(applicationTemplates))]
		prompt := fmt.Sprintf(template, concept.Name)
		if seed != "" {
			prompt += " Utgå från: " + seed
		}
		ideas = append(ideas, model.Idea{
			BookID: concept.BookID,
			Kind:   KindApplication,
			Prompt: prompt,
			Basis:  []string{concept.Name},
		})
		ideas = append(ideas, model.Idea{
			BookID: concept.BookID,
			Kind:   KindInversion,
			Prompt: fmt.Sprintf(inversionTemplate, concept.Name),
			Basis:  []string{concept.Name},
		})
	}

	if seed != "" {
		lowered := strings.ToLower(seed)
		for _, concept := range usable {
			if !strings.Contains(strings.ToLower(concept.Name), lowered) {
				continue
			}
			ideas = append(ideas, model.Idea{
				BookID: concept.BookID,
				Kind:   KindSeed,
				Prompt: fmt.Sprintf("Du nämner \u201c%s\u201d. Kan du använda %s för att svara på det du egentligen vill veta?",
					seed, concept.Name),
				Basis: []string{concept.Name},
			})
		}
	}

	var unique []model.Idea
	seen := map[string]bool{}
	for _, idea := range ideas {
		if seen[idea.Prompt] {
			continue
		}
		seen[idea.Prompt] = true
		unique = append(unique, idea)
	}
	if len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}

func bookLabel(concept model.Concept) string {
	return fmt.Sprintf("bok %d", concept.BookID)
}

func sortByImportance(concepts []model.Concept) {
	for i := 1; i < len(concepts); i++ {
		for j := i; j > 0 && concepts[j].Importance > concepts[j-1].Importance; j-- {
			concepts[j], concepts[j-1] = concepts[j-1], concepts[j]
		}
	}
}
