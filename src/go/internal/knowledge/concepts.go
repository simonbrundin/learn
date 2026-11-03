// Package knowledge extraherar koncept ur böcker och bygger kunskapsgrafer.
package knowledge

import (
	"regexp"
	"sort"
	"strings"

	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/text"
)

// capitalizedRe matchar versaler, frasade uttryck och numrerade termer.
var capitalizedRe = regexp.MustCompile(`\b([A-Z][\p{L}\p{N}'’-]+(?:[ ]+(?:of|the|and|for|in|on|to|or|as|de|och|av)[ ]+[A-Z][\p{L}\p{N}'’-]+|[ ]+[A-Z0-9][\p{L}\p{N}'’-]*)*)`)

// genericTerms är rubriker som inte är koncept.
var genericTerms = map[string]bool{
	"chapter": true, "part": true, "introduction": true, "conclusion": true,
	"summary": true, "figure": true, "table": true, "example": true,
	"note": true, "appendix": true, "preface": true, "foreword": true,
	"acknowledgements": true, "contents": true, "index": true,
}

// conceptCandidate är en potentiell term under uppbyggnad.
type conceptCandidate struct {
	mentions  int
	sentences []string
	name      string
}

// ConceptExtractor hittar de viktigaste koncepten i en bok.
type ConceptExtractor struct {
	// MaxConcepts begränsar hur många koncept som behålls.
	MaxConcepts int

	contexts map[string][]string
}

// NewConceptExtractor skapar en extraktor med given maxgräns.
func NewConceptExtractor(maxConcepts int) *ConceptExtractor {
	if maxConcepts <= 0 {
		maxConcepts = 60
	}
	return &ConceptExtractor{MaxConcepts: maxConcepts, contexts: map[string][]string{}}
}

// Contexts returnerar meningarna där varje koncept nämns.
func (e *ConceptExtractor) Contexts() map[string][]string { return e.contexts }

// Extract går igenom kapitlen och returnerar koncepten som hittades.
func (e *ConceptExtractor) Extract(episodes []model.Episode, bookID int64) []model.Concept {
	e.contexts = map[string][]string{}
	if len(episodes) == 0 {
		return nil
	}

	globalCounts := map[string]int{}
	perEpisode := make([][]string, 0, len(episodes))
	for _, episode := range episodes {
		sents := text.Sentences(episode.Text)
		perEpisode = append(perEpisode, sents)
		for _, word := range text.ContentWords(episode.Text) {
			globalCounts[word]++
		}
	}

	definitions := e.collectDefinitions(episodes)
	headings := map[string]bool{}
	for _, episode := range episodes {
		if episode.Title != "" {
			headings[strings.ToLower(episode.Title)] = true
		}
	}

	candidates := map[string]*conceptCandidate{}

	for index, episode := range episodes {
		for _, sentence := range perEpisode[index] {
			for _, term := range candidateTerms(sentence) {
				key := strings.ToLower(term)
				entry, ok := candidates[key]
				if !ok {
					entry = &conceptCandidate{}
					candidates[key] = entry
				}
				entry.mentions++
				if len(entry.sentences) < 12 {
					entry.sentences = append(entry.sentences, sentence)
				}
			}
		}
		for _, term := range candidateTerms(episode.Title) {
			key := strings.ToLower(term)
			entry, ok := candidates[key]
			if !ok {
				entry = &conceptCandidate{}
				candidates[key] = entry
			}
			entry.mentions += 2
		}
	}

	for key, entry := range candidates {
		entry.name = e.canonicalName(key, entry.sentences, definitions)
	}

	type scored struct {
		key   string
		score float64
	}
	var ranked []scored
	maxMentions := 1
	for _, entry := range candidates {
		if entry.mentions > maxMentions {
			maxMentions = entry.mentions
		}
	}
	for key, entry := range candidates {
		ranked = append(ranked, scored{key: key, score: e.score(key, entry, definitions, headings, globalCounts, maxMentions)})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].key < ranked[j].key
	})
	if len(ranked) > e.MaxConcepts {
		ranked = ranked[:e.MaxConcepts]
	}

	concepts := make([]model.Concept, 0, len(ranked))
	for _, item := range ranked {
		entry := candidates[item.key]
		definition := ""
		if def, ok := definitions[item.key]; ok {
			definition = def.Definition
		}
		sample := strings.Join(entry.sentences, " ")
		summary := text.Summarize(sample, 3)
		if summary == "" {
			summary = definition
		}

		var keywords []string
		for _, keyword := range text.Keywords(sample, 15) {
			if keyword == item.key || strings.Contains(keyword, item.key) {
				continue
			}
			keywords = append(keywords, keyword)
			if len(keywords) == 10 {
				break
			}
		}

		var quotes []string
		for _, quote := range text.FindQuotes(strings.Join(entry.sentences, " "), 4) {
			if strings.Contains(strings.ToLower(quote), item.key) {
				quotes = append(quotes, quote)
			}
		}

		kind := "term"
		if definition != "" {
			kind = "concept"
		}

		e.contexts[strings.ToLower(entry.name)] = entry.sentences

		concepts = append(concepts, model.Concept{
			BookID:     bookID,
			Name:       entry.name,
			Definition: definition,
			Summary:    summary,
			Kind:       kind,
			Keywords:   keywords,
			Quotes:     quotes,
			Importance: round(item.score, 4),
			Mentions:   entry.mentions,
		})
	}
	return concepts
}

func (e *ConceptExtractor) collectDefinitions(episodes []model.Episode) map[string]text.Definition {
	definitions := map[string]text.Definition{}
	for _, episode := range episodes {
		for _, def := range text.FindDefinitions(episode.Text, 40) {
			key := strings.ToLower(def.Term)
			existing, ok := definitions[key]
			if !ok || len(def.Definition) > len(existing.Definition) {
				definitions[key] = def
			}
		}
	}
	return definitions
}

func (e *ConceptExtractor) canonicalName(key string, sentences []string, definitions map[string]text.Definition) string {
	if def, ok := definitions[key]; ok {
		return def.Term
	}
	counts := map[string]int{}
	for _, sentence := range sentences {
		for _, term := range candidateTerms(sentence) {
			if strings.ToLower(term) == key {
				counts[term]++
			}
		}
	}
	best, bestCount := "", 0
	for term, count := range counts {
		if count > bestCount || (count == bestCount && term < best) {
			best, bestCount = term, count
		}
	}
	if best != "" {
		return best
	}
	return key
}

func (e *ConceptExtractor) score(
	key string,
	entry *conceptCandidate,
	definitions map[string]text.Definition,
	headings map[string]bool,
	globalCounts map[string]int,
	maxMentions int,
) float64 {
	mentionsScore := float64(entry.mentions) / float64(maxMentions)
	definitionScore := 0.0
	if _, ok := definitions[key]; ok {
		definitionScore = 1
	}
	headingScore := 0.0
	name := strings.ToLower(entry.name)
	for heading := range headings {
		if strings.Contains(heading, name) || strings.Contains(name, heading) {
			headingScore = 1
			break
		}
	}
	frequency := float64(globalCounts[key]) / 20.0
	if frequency > 1 {
		frequency = 1
	}
	return 0.4*mentionsScore + 0.3*definitionScore + 0.15*headingScore + 0.15*frequency
}

// candidateTerms hittar möjliga konceptnamn i en text.
func candidateTerms(source string) []string {
	matches := capitalizedRe.FindAllStringSubmatch(source, -1)
	var found []string
	for _, match := range matches {
		term := cleanTerm(strings.TrimSpace(match[1]))
		if term != "" {
			found = append(found, term)
		}
	}
	return found
}

func cleanTerm(term string) string {
	if term == "" {
		return ""
	}
	words := strings.Fields(term)
	for len(words) > 0 {
		lower := strings.ToLower(words[0])
		if text.Stopwords[lower] || text.LeadingConnectives[lower] {
			words = words[1:]
			continue
		}
		break
	}
	if len(words) == 0 || len(words) > 5 {
		return ""
	}
	if genericTerms[strings.ToLower(words[0])] {
		return ""
	}
	allStop := true
	for _, word := range words {
		if !text.Stopwords[strings.ToLower(word)] {
			allStop = false
			break
		}
	}
	if allStop {
		return ""
	}
	if len(words) == 1 && len([]rune(words[0])) <= 3 {
		return ""
	}
	return strings.Join(words, " ")
}

func round(value float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int64(value*factor+0.5)) / factor
}
