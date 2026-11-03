// Package text innehåller heuristisk textbehandling: segmentering,
// nyckelord, definitioner och citat.
//
// Funktionerna är avsiktligt beroendefria så att de kan användas utan
// externa NLP-bibliotek.
package text

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// wordRe matchar ord med bokstäver och siffror.
var wordRe = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'’-]*`)

// definitionPatterns hittar definitioner av formen "X är ...".
var definitionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:is|are) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:refers to) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:means) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?)[:\-—] +(.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:är|är en|är ett|är en typ av) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:beskrivs som) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:kan definieras som) (.+)$`),
}

// Definition är en term tillsammans med sin definition.
type Definition struct {
	Term       string
	Definition string
}

// Stopwords är ord som inte bär någon betydelse i nyckelordssammanställningar.
var Stopwords = map[string]bool{}

// LeadingConnectives är ord som inleder en sats och ska hoppas över
// när termer extraheras.
var LeadingConnectives = map[string]bool{
	"unlike": true, "whereas": true, "because": true, "although": true,
	"though": true, "while": true, "when": true, "whenever": true,
	"since": true, "after": true, "before": true, "however": true,
	"but": true, "so": true, "thus": true, "therefore": true,
	"moreover": true, "furthermore": true, "and": true, "or": true, "if": true,
}

// quoteMarkers markerar stycken som innehåller tankeväckande formuleringar.
var quoteMarkers = []string{
	"key insight", "important", "crucial", "essential", "fundamental",
	"in fact", "the point is", "remember that", "the main idea", "central to",
	"critical", "viktig", "nyckel", "central", "avgörande", "kärnan", "kom ihåg",
}

// abbreviations är förkortningar som inte avslutar en mening.
var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
	"etc": true, "e.g": true, "i.e": true, "vs": true, "cf": true, "ca": true,
}

func init() {
	for _, word := range []string{
		"the", "and", "for", "that", "have", "with", "this", "from", "they",
		"not", "are", "but", "was", "were", "will", "would", "there", "their",
		"what", "about", "which", "when", "make", "can", "like", "just", "know",
		"take", "into", "your", "some", "them", "then", "than", "these", "other",
		"been", "more", "also", "because", "does", "doing", "each", "very",
		"much", "such", "only", "own", "same", "too", "how", "why", "who",
		"whom", "here", "all", "any", "both", "he", "she", "it", "we", "you",
		"i", "is", "in", "on", "at", "to", "of", "as", "by", "or", "an", "be",
		"if", "so", "up", "out", "no", "yes", "one", "two", "may", "might",
		"must", "shall", "am", "being", "having",
		"och", "att", "det", "som", "en", "ett", "för", "med", "den", "de",
		"inte", "har", "hade", "är", "var", "men", "eller", "om", "på", "av",
		"till", "från", "kan", "ska", "skall", "vill", "blev", "blir", "vad",
		"vem", "hur", "när", "där", "här", "sig", "sin", "sina", "sitt",
		"mycket", "alla", "andra", "över", "under", "upp", "ner", "in", "ut",
		"efter", "före", "mellan", "utan",
	} {
		Stopwords[word] = true
	}
}

// Normalize normaliserar whitespace och unicode.
func Normalize(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u00ad", "")
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// Tokens delar texten i lowercase-ord.
func Tokens(s string) []string {
	matches := wordRe.FindAllString(s, -1)
	tokens := make([]string, 0, len(matches))
	for _, match := range matches {
		tokens = append(tokens, strings.ToLower(match))
	}
	return tokens
}

// ContentWords tokeniserar och filtrerar bort stoppord och korta ord.
func ContentWords(s string) []string {
	var words []string
	for _, token := range Tokens(s) {
		if Stopwords[token] || len([]rune(token)) <= 2 {
			continue
		}
		words = append(words, token)
	}
	return words
}

// Sentences delar text i meningar med enkla heuristiker.
func Sentences(s string) []string {
	s = Normalize(s)
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var parts []string
	start := 0
	for i := 0; i < len(runes); i++ {
		if runes[i] != '.' && runes[i] != '!' && runes[i] != '?' {
			continue
		}
		if i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) {
			continue
		}
		part := strings.TrimSpace(string(runes[start : i+1]))
		if part != "" {
			parts = append(parts, part)
		}
		start = i + 1
	}
	if rest := strings.TrimSpace(string(runes[start:])); rest != "" {
		parts = append(parts, rest)
	}

	// Slå ihop meningar som avslutas med en förkortning.
	var merged []string
	for _, part := range parts {
		if len(merged) > 0 {
			last := merged[len(merged)-1]
			tail := strings.ToLower(strings.TrimRight(strings.TrimSpace(last), ".!?"))
			if idx := strings.LastIndex(tail, " "); idx >= 0 {
				tail = tail[idx+1:]
			}
			if abbreviations[tail] {
				merged[len(merged)-1] = last + " " + part
				continue
			}
		}
		merged = append(merged, part)
	}
	return merged
}

// Keywords returnerar frekvensbaserade nyckelord.
func Keywords(s string, limit int) []string {
	counts := TermFrequencies(s)
	type kv struct {
		word  string
		count int
	}
	pairs := make([]kv, 0, len(counts))
	for word, count := range counts {
		pairs = append(pairs, kv{word, count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count != pairs[j].count {
			return pairs[i].count > pairs[j].count
		}
		return pairs[i].word < pairs[j].word
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	words := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		words = append(words, pair.word)
	}
	return words
}

// TermFrequencies räknar förekomster av innehållsord.
func TermFrequencies(s string) map[string]int {
	counts := map[string]int{}
	for _, word := range ContentWords(s) {
		counts[word]++
	}
	return counts
}

// Summarize bygger en sammanfattning baserad på nyckelordsfrekvens och position.
func Summarize(s string, maxSentences int) string {
	sents := Sentences(s)
	if len(sents) <= maxSentences {
		return strings.Join(sents, " ")
	}
	counts := TermFrequencies(s)
	if len(counts) == 0 {
		return strings.Join(sents[:maxSentences], " ")
	}
	maxFreq := 0
	for _, count := range counts {
		if count > maxFreq {
			maxFreq = count
		}
	}
	total := len(sents)
	type scored struct {
		score float64
		index int
	}
	var ranked []scored
	for index, sentence := range sents {
		words := ContentWords(sentence)
		if len(words) == 0 {
			continue
		}
		unique := map[string]bool{}
		sum := 0.0
		for _, word := range words {
			if unique[word] {
				continue
			}
			unique[word] = true
			sum += float64(counts[word]) / float64(maxFreq)
		}
		score := sum / math.Sqrt(float64(len(unique)))
		score *= 1.0 + 0.25*(1-float64(index)/float64(total))
		ranked = append(ranked, scored{score, index})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > maxSentences {
		ranked = ranked[:maxSentences]
	}
	indexes := make([]int, 0, len(ranked))
	for _, item := range ranked {
		indexes = append(indexes, item.index)
	}
	sort.Ints(indexes)
	chosen := make([]string, 0, len(indexes))
	for _, index := range indexes {
		chosen = append(chosen, sents[index])
	}
	return strings.Join(chosen, " ")
}

// FindDefinitions hittar definitioner av formen "X är ...".
func FindDefinitions(s string, limit int) []Definition {
	var found []Definition
	seen := map[string]bool{}
	for _, sentence := range Sentences(s) {
		for _, pattern := range definitionPatterns {
			match := pattern.FindStringSubmatch(sentence)
			if match == nil {
				continue
			}
			term := strings.Trim(strings.TrimSpace(match[1]), " .,")
			definition := strings.Trim(strings.TrimSpace(match[2]), " .,")
			if term == "" || definition == "" {
				continue
			}
			words := strings.Fields(term)
			if len(words) == 0 {
				continue
			}
			first := strings.ToLower(words[0])
			if Stopwords[first] || LeadingConnectives[first] {
				continue
			}
			if len([]rune(definition)) < 8 || len([]rune(definition)) > 400 {
				continue
			}
			// En definition som upprepar termen gång på gång är sannolikt
			// flera rubriker som sammanslagnits, inte en förklaring.
			if strings.Count(strings.ToLower(definition), strings.ToLower(term)) > 2 {
				continue
			}
			key := strings.ToLower(term)
			if seen[key] {
				continue
			}
			seen[key] = true
			found = append(found, Definition{Term: term, Definition: definition})
			break
		}
		if len(found) >= limit {
			break
		}
	}
	return found
}

// FindQuotes hittar tankeväckande stycken med kända markörer.
func FindQuotes(s string, limit int) []string {
	var quotes []string
	for _, sentence := range Sentences(s) {
		lowered := strings.ToLower(sentence)
		matched := false
		for _, marker := range quoteMarkers {
			if strings.Contains(lowered, marker) {
				matched = true
				break
			}
		}
		if !matched && strings.Count(sentence, `"`) >= 2 && len([]rune(sentence)) < 400 {
			matched = true
		}
		if matched {
			quotes = append(quotes, sentence)
		}
		if len(quotes) >= limit {
			break
		}
	}
	return quotes
}

// Similarity mäter överlapp mellan två texters nyckelord.
func Similarity(left, right string) float64 {
	a := map[string]bool{}
	b := map[string]bool{}
	for _, word := range ContentWords(left) {
		a[word] = true
	}
	for _, word := range ContentWords(right) {
		b[word] = true
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	overlap := 0
	for word := range a {
		if b[word] {
			overlap++
		}
	}
	return float64(overlap) / float64(len(a)+len(b)-overlap)
}
