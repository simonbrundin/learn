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
	"pro tip": true, "caution": true, "warning": true, "tip": true,
	"pitfall": true, "anti-pattern": true, "antipattern": true,
	"key terms": true, "key term": true, "glossary": true, "faq": true,
	"best practices": true, "getting started": true, "overview": true,
	"further reading": true, "references": true, "troubleshooting": true,
	"requirements": true, "prerequisites": true, "benefits": true,
	"difficulty": true, "languages": true, "edition": true, "version": true,
	"copyright": true, "introduction to": true, "background": true,
	"step": true, "steps": true, "list": true, "review": true,
}

// scored är en term med sitt viktvärde.
type scored struct {
	key   string
	score float64
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
	// MinStrongConcepts är antalet tydliga begrepp som krävs innan
	// svagare termer tas med.
	MinStrongConcepts int

	contexts map[string][]string
}

// NewConceptExtractor skapar en extraktor med given maxgräns.
func NewConceptExtractor(maxConcepts int) *ConceptExtractor {
	if maxConcepts <= 0 {
		maxConcepts = 60
	}
	return &ConceptExtractor{MaxConcepts: maxConcepts, MinStrongConcepts: 8, contexts: map[string][]string{}}
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

	profiles := map[string]caseProfile{}
	for _, episode := range episodes {
		for key, profile := range caseProfiles(episode.Text) {
			merged := profiles[key]
			merged.midCapital += profile.midCapital
			merged.lower += profile.lower
			profiles[key] = merged
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
			if key == strings.ToLower(strings.TrimSpace(episode.Title)) {
				// Rubriken i sig är ett avsnitt, inte ett begrepp.
				continue
			}
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

	var ranked []scored
	maxMentions := 1
	for _, entry := range candidates {
		if entry.mentions > maxMentions {
			maxMentions = entry.mentions
		}
	}
	for key, entry := range candidates {
		ranked = append(ranked, scored{key: key, score: e.score(key, entry, definitions, headings, globalCounts, maxMentions, profiles[key])})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].key < ranked[j].key
	})
	ranked = e.preferStrongConcepts(ranked, definitions, profiles)
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
	profile caseProfile,
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
	// Begrepp som skrivs med versal mitt i texten är sannolikt riktiga
	// termer, medan vanliga ord oftast bara har versal när de inleder
	// en mening.
	termScore := 0.0
	if profile.midCapital > 0 {
		termScore = 1
	}
	return 0.3*mentionsScore + 0.25*definitionScore + 0.15*headingScore +
		0.15*frequency + 0.15*termScore
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
	if genericTerms[strings.ToLower(strings.Join(words, " "))] || genericTerms[strings.ToLower(words[0])] {
		return ""
	}
	if len(words) == 1 && text.GenericWords[strings.ToLower(words[0])] {
		return ""
	}
	// Släpp fotnotsmarkörer i slutet, till exempel "Ruby Capybara 17".
	for len(words) > 1 {
		last := words[len(words)-1]
		digits := 0
		for _, r := range last {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits == len([]rune(last)) && digits > 1 {
			words = words[:len(words)-1]
			continue
		}
		break
	}
	// Siffror direkt på ordet är fotnotsmarkörer, till exempel "ActiveRecord1".
	if len(words) > 0 {
		last := words[len(words)-1]
		trimmed := strings.TrimRight(last, "0123456789")
		if trimmed != last && trimmed != "" {
			words[len(words)-1] = trimmed
		}
	}
	// Böjningsformer med genitiv-s är sällan begrepp.
	joined := strings.ToLower(strings.Join(words, " "))
	if strings.HasSuffix(joined, "'s") || strings.HasSuffix(joined, "’s") {
		return ""
	}
	// Kontraktioner som "I've" är aldrig begrepp.
	if strings.ContainsAny(joined, "'’") {
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

// caseProfile beskriver hur en term skrivs i texten.
type caseProfile struct {
	// midCapital är antalet gånger termen förekommer med versal mitt i
	// en mening, vilket tyder på att den är ett riktigt begrepp.
	midCapital int
	// lower är antalet gånger termen förekommer med gemener.
	lower int
}

// caseProfiles räknar hur varje ord skrivs i texten.
func caseProfiles(source string) map[string]caseProfile {
	profiles := map[string]caseProfile{}
	for _, sentence := range text.Sentences(source) {
		words := strings.Fields(sentence)
		for index, word := range words {
			core := strings.Trim(word, ".,;:!?()[]\"'“”–—")
			if core == "" {
				continue
			}
			key := strings.ToLower(core)
			profile := profiles[key]
			switch {
			case index == 0:
				// Meningsstart ger ingen information om hur termen skrivs.
			case isCapitalized(core):
				profile.midCapital++
			case isLowercase(core):
				profile.lower++
			}
			profiles[key] = profile
		}
	}
	return profiles
}

// isCapitalized svarar på om ordet börjar med versal.
func isCapitalized(word string) bool {
	runes := []rune(word)
	if len(runes) == 0 {
		return false
	}
	return runes[0] >= 'A' && runes[0] <= 'Z'
}

// isLowercase svarar på om ordet börjar med gemena.
func isLowercase(word string) bool {
	runes := []rune(word)
	if len(runes) == 0 {
		return false
	}
	return runes[0] >= 'a' && runes[0] <= 'z'
}

// preferStrongConcepts sorterar bort termer som bara är vanliga ord i
// början av en mening, så länge boken har tillräckligt många tydliga
// begrepp.
func (e *ConceptExtractor) preferStrongConcepts(
	ranked []scored,
	definitions map[string]text.Definition,
	profiles map[string]caseProfile,
) []scored {
	var strong, weak []scored
	for _, item := range ranked {
		if isStrongConcept(item.key, definitions, profiles) {
			strong = append(strong, item)
			continue
		}
		weak = append(weak, item)
	}
	if len(strong) < e.MinStrongConcepts {
		return ranked
	}
	return strong
}

// isStrongConcept svarar på om en term är ett begrepp: den har en
// definition eller skrivs alltid med versal, vilket tyder på ett
// eget begrepp snarare än ett vanligt ord.
func isStrongConcept(key string, definitions map[string]text.Definition, profiles map[string]caseProfile) bool {
	if _, ok := definitions[key]; ok {
		return true
	}
	for _, word := range strings.Fields(key) {
		profile, ok := profiles[strings.ToLower(word)]
		if ok && profile.midCapital > 0 && profile.lower == 0 {
			return true
		}
	}
	return false
}

// round avrundar till given decimal.
func round(value float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int64(value*factor+0.5)) / factor
}
