package learning

import (
	"regexp"
	"sort"
	"strings"

	"github.com/simon/repo-template/internal/knowledge"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/text"
)

// Förklarningsnivåer.
const (
	LevelNovice   = "novice"
	LevelStandard = "standard"
	LevelAdvanced = "advanced"
)

// Levels listar giltiga nivåer.
var Levels = []string{LevelNovice, LevelStandard, LevelAdvanced}

// Related är en relaterat koncept med relation.
type Related struct {
	Name     string `json:"name"`
	Relation string `json:"relation"`
}

// Explanation är en strukturerad förklaring av ett koncept.
type Explanation struct {
	Title      string    `json:"title"`
	Level      string    `json:"level"`
	Definition string    `json:"definition"`
	KeyPoints  []string  `json:"key_points"`
	Example    string    `json:"example"`
	Related    []Related `json:"related"`
	Quotes     []string  `json:"quotes"`
	FollowUps  []string  `json:"follow_ups"`
}

// Text renderar förklaringen för terminalen.
func (e Explanation) Text() string {
	var lines []string
	lines = append(lines, e.Title, "")
	if e.Definition != "" {
		lines = append(lines, "Vad det är: "+e.Definition, "")
	}
	if len(e.KeyPoints) > 0 {
		lines = append(lines, "Nyckelpunkter:")
		for _, point := range e.KeyPoints {
			lines = append(lines, "  - "+point)
		}
		lines = append(lines, "")
	}
	if e.Example != "" {
		lines = append(lines, "Exempel: "+e.Example, "")
	}
	if len(e.Quotes) > 0 {
		lines = append(lines, "Cit från boken:")
		for _, quote := range e.Quotes {
			lines = append(lines, "  \u201c"+quote+"\u201d")
		}
		lines = append(lines, "")
	}
	if len(e.Related) > 0 {
		lines = append(lines, "Relaterade koncept:")
		for _, related := range e.Related {
			lines = append(lines, "  - "+related.Name+" ("+related.Relation+")")
		}
		lines = append(lines, "")
	}
	if len(e.FollowUps) > 0 {
		lines = append(lines, "Följdfrågor:")
		for _, question := range e.FollowUps {
			lines = append(lines, "  ? "+question)
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// KeyPoints bryter ner en sammanfattning till punkter.
func KeyPoints(concept model.Concept, limit int) []string {
	source := concept.Summary
	if source == "" {
		source = concept.Definition
	}
	var points []string
	for _, sentence := range text.Sentences(source) {
		if len(strings.Fields(sentence)) > 3 {
			points = append(points, sentence)
		}
	}
	if len(points) < limit {
		for i, keyword := range concept.Keywords {
			if i >= len(concept.Quotes) {
				break
			}
			points = append(points, keyword+": "+concept.Quotes[i])
		}
	}
	if len(points) > limit {
		points = points[:limit]
	}
	return points
}

// Explain bygger en förklaring av ett koncept på vald nivå.
func Explain(concept model.Concept, level string, graph *knowledge.Graph, relatedLimit int) Explanation {
	valid := false
	for _, candidate := range Levels {
		if candidate == level {
			valid = true
		}
	}
	if !valid {
		level = LevelStandard
	}

	definition := concept.Definition
	if definition == "" {
		definition = concept.Summary
	}
	if level == LevelNovice && definition != "" {
		definition = Simplify(definition)
	}

	limit := 3
	if level == LevelAdvanced {
		limit = 5
	}
	points := KeyPoints(concept, limit)
	if level == LevelNovice && len(points) > 2 {
		points = points[:2]
	}

	var related []Related
	if graph != nil {
		for _, neighbour := range graph.Related(concept, relatedLimit) {
			related = append(related, Related{Name: neighbour.Concept.Name, Relation: neighbour.Relation})
		}
	}

	quotes := concept.Quotes
	if level != LevelAdvanced && len(quotes) > 1 {
		quotes = quotes[:1]
	}
	example := ""
	if len(concept.Quotes) > 0 {
		example = concept.Quotes[0]
	}

	var followUps []string
	for i, item := range related {
		if i >= 2 {
			break
		}
		followUps = append(followUps, "Hur hänger "+concept.Name+" ihop med "+item.Name+"?")
	}
	if len(concept.Keywords) > 0 {
		followUps = append(followUps, "Vilken roll spelar "+concept.Keywords[0]+" i sammanhanget?")
	}

	return Explanation{
		Title:      concept.Name,
		Level:      level,
		Definition: definition,
		KeyPoints:  points,
		Example:    example,
		Related:    related,
		Quotes:     quotes,
		FollowUps:  followUps,
	}
}

var simplifications = map[string]string{
	"utilize":        "använda",
	"demonstrate":    "visa",
	"aforementioned": "nämnda",
	"individuals":    "människor",
	"approximately":  "ungefär",
	"subsequently":   "sedan",
	"nevertheless":   "ändå",
	"furthermore":    "dessutom",
	"consequently":   "därför",
}

// Simplify kortar ner en text till dess första mening och enklare ord.
func Simplify(source string) string {
	sentences := text.Sentences(source)
	simplified := source
	if len(sentences) > 0 {
		simplified = sentences[0]
	}
	for word, replacement := range simplifications {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
		simplified = pattern.ReplaceAllString(simplified, replacement)
	}
	return simplified
}

// AnswerResult är svaret på en fråga mot kunskapsbasen.
type AnswerResult struct {
	Concepts []model.Concept
	Text     string
}

// RankConcepts rangerar koncept utifrån hur väl de matchar en fråga.
func RankConcepts(question string, concepts []model.Concept, context *model.Concept, limit int) []model.Concept {
	questionWords := tokens(question)
	if len(questionWords) == 0 {
		return nil
	}
	questionLower := strings.ToLower(question)
	type scored struct {
		concept model.Concept
		score   float64
	}
	var results []scored
	for _, concept := range concepts {
		nameLower := strings.ToLower(concept.Name)
		nameWords := tokens(concept.Name)
		bodyWords := tokens(concept.Definition + " " + concept.Summary)
		keywordWords := map[string]bool{}
		for _, keyword := range concept.Keywords {
			keywordWords[strings.ToLower(keyword)] = true
		}

		score := float64(overlap(questionWords, nameWords)) * 3.0
		if nameLower != "" && strings.Contains(questionLower, nameLower) {
			score += 5.0
		}
		score += float64(overlap(questionWords, bodyWords)) * 0.5
		for word := range questionWords {
			if keywordWords[word] {
				score += 1.5
			}
		}
		if context != nil && concept.ID == context.ID {
			score += 2.0
		}
		if score > 0 {
			results = append(results, scored{concept: concept, score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].concept.Importance > results[j].concept.Importance
	})
	if len(results) > limit {
		results = results[:limit]
	}
	matches := make([]model.Concept, 0, len(results))
	for _, item := range results {
		matches = append(matches, item.concept)
	}
	return matches
}

// Answer besvarar en fråga med utdrag ur kunskapsbasen.
func Answer(question string, concepts []model.Concept, graph *knowledge.Graph, context *model.Concept) AnswerResult {
	matches := RankConcepts(question, concepts, context, 3)
	if len(matches) == 0 {
		return AnswerResult{
			Concepts: nil,
			Text: "Jag hittade inget matchande koncept. Prova att använda en term " +
				"från boken, t.ex. \u201cVad är X?\u201d",
		}
	}
	var parts []string
	for _, concept := range matches {
		detail := concept.Definition
		if detail == "" {
			detail = concept.Summary
		}
		parts = append(parts, concept.Name+" \u2014 "+detail)
		if graph != nil {
			neighbours := graph.Related(concept, 2)
			if len(neighbours) > 0 {
				names := make([]string, 0, len(neighbours))
				for _, neighbour := range neighbours {
					names = append(names, neighbour.Concept.Name)
				}
				parts = append(parts, "  Kopplar till: "+strings.Join(names, ", "))
			}
		}
	}
	return AnswerResult{Concepts: matches, Text: strings.Join(parts, "\n")}
}

// tokens extraherar ord, även siffror som "2" i "System 2".
func tokens(source string) map[string]bool {
	result := map[string]bool{}
	for _, word := range text.ContentWords(source) {
		result[word] = true
	}
	for _, field := range strings.Fields(strings.ToLower(source)) {
		trimmed := strings.Trim(field, ".,;:!?()\"'")
		if trimmed != "" && isDigits(trimmed) {
			result[trimmed] = true
		}
	}
	return result
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(value) > 0
}

func overlap(a, b map[string]bool) int {
	count := 0
	for key := range a {
		if b[key] {
			count++
		}
	}
	return count
}
