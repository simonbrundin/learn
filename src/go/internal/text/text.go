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
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,2}?)[:\-—] +(.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:är|är en|är ett|är en typ av) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:beskrivs som) (.+)$`),
	regexp.MustCompile(`^([\p{L}\p{N}'’-]+(?: [\p{L}\p{N}'’-]+){0,4}?) (?i:kan definieras som) (.+)$`),
}

// LooksLikeProse svarar på om en text ser ut som förklaring och inte
// som kod eller en utdata.
func LooksLikeProse(value string) bool {
	codeMarks := []string{"@", "{", "}", "::", "=>", "();", "$", "<%", "/>", "()", "[]"}
	for _, mark := range codeMarks {
		if strings.Contains(value, mark) {
			return false
		}
	}
	words := strings.Fields(value)
	if len(words) < 3 {
		return false
	}
	odd := 0
	for _, word := range words {
		letters := 0
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters++
			}
		}
		if letters*2 < len([]rune(word)) {
			odd++
		}
	}
	return odd*3 <= len(words)
}

// GenericWords är vanliga ord som sällan bär någon kunskap ensamma och
// därför inte är värda att vara koncept.
var GenericWords = map[string]bool{
	"unfortunately": true,
	"fortunately":   true,
	"sadly":         true,
	"happily":       true,
	"interestingly": true,
	"importantly":   true,
	"notably":       true,
	"generally":     true,
	"typically":     true,
	"frequently":    true,
	"particularly":  true,
	"especially":    true,
	"indeed":        true,
	"certainly":     true,
	"obviously":     true,
	"naturally":     true,
	"presumably":    true,
	"arguably":      true,
	"effectively":   true,
	"essentially":   true,
	"basically":     true,
	"ultimately":    true,
	"primarily":     true,
	"mainly":        true,
	"mostly":        true,
	"largely":       true,
	"partly":        true,
	"somewhat":      true,
	"slightly":      true,
	"barely":        true,
	"hardly":        true,
	"nearly":        true,
	"almost":        true,
	"roughly":       true,
	"approximately": true,
	"meanwhile":     true,
	"afterwards":    true,
	"elsewhere":     true,
	"everywhere":    true,
	"nowhere":       true,
	"somewhere":     true,
	"anywhere":      true,
	"therefore":     true,
	"hence":         true,
	"otherwise":     true,
	"instead":       true,
	"besides":       true,
	"likewise":      true,
	"similarly":     true,
	"consequently":  true,
	"accordingly":   true,
	"henceforth":    true,
	"thereafter":    true,
	"alternatively": true,
	"however":       true,
	"moreover":      true,
	"furthermore":   true,
	"nevertheless":  true,
	"nonetheless":   true,
	"anyway":        true,
	"actually":      true,
	"simply":        true,
	"just":          true,
	"really":        true,
	"quite":         true,
	"perhaps":       true,
	"maybe":         true,
	"probably":      true,
	"definitely":    true,
	"thus":          true,
	"beforehand":    true,
	"finally":       true,
	"initially":     true,
	"originally":    true,
	"previously":    true,
	"recently":      true,
	"currently":     true,
	"today":         true,
	"yesterday":     true,
	"tomorrow":      true,
	"later":         true,
	"sooner":        true,
	"again":         true,
	"twice":         true,
	"ever":          true,
	"sometimes":     true,
	"often":         true,
	"usually":       true,
	"rarely":        true,
	"seldom":        true,
	"everything":    true,
	"everyone":      true,
	"someone":       true,
	"anyone":        true,
	"nobody":        true,
	"somebody":      true,
	"here":          true,
	"there":         true,
	"then":          true,
	"than":          true,
	"given":         true,
	"when":          true,
	"once":          true,
	"string":        true,
	"system":        true,
	"build":         true,
	"test":          true,
	"tests":         true,
	"step":          true,
	"steps":         true,
	"class":         true,
	"module":        true,
	"method":        true,
	"function":      true,
	"value":         true,
	"values":        true,
	"name":          true,
	"names":         true,
	"type":          true,
	"types":         true,
	"list":          true,
	"lists":         true,
	"table":         true,
	"tables":        true,
	"row":           true,
	"rows":          true,
	"column":        true,
	"columns":       true,
	"file":          true,
	"files":         true,
	"folder":        true,
	"path":          true,
	"key":           true,
	"keys":          true,
	"text":          true,
	"code":          true,
	"data":          true,
	"result":        true,
	"results":       true,
	"output":        true,
	"input":         true,
	"error":         true,
	"errors":        true,
	"message":       true,
	"log":           true,
	"logs":          true,
	"user":          true,
	"users":         true,
	"time":          true,
	"date":          true,
	"number":        true,
	"count":         true,
	"size":          true,
	"order":         true,
	"group":         true,
	"case":          true,
	"cases":         true,
	"object":        true,
	"objects":       true,
	"item":          true,
	"items":         true,
	"page":          true,
	"pages":         true,
	"site":          true,
	"server":        true,
	"client":        true,
	"api":           true,
	"sql":           true,
	"tag":           true,
	"tags":          true,
	"script":        true,
	"scripts":       true,
	"command":       true,
	"option":        true,
	"options":       true,
	"version":       true,
	"release":       true,
	"feature":       true,
	"features":      true,
	"product":       true,
	"project":       true,
	"example":       true,
	"examples":      true,
	"question":      true,
	"answer":        true,
	"point":         true,
	"points":        true,
	"part":          true,
	"case study":    true,
	"something":     true,
	"anything":      true,
	"nothing":       true,
	"other":         true,
	"others":        true,
	"many":          true,
	"much":          true,
	"more":          true,
	"most":          true,
	"some":          true,
	"all":           true,
	"own":           true,
	"same":          true,
	"first":         true,
	"second":        true,
	"third":         true,
	"next":          true,
	"last":          true,
	"new":           true,
	"old":           true,
	"good":          true,
	"bad":           true,
	"best":          true,
	"great":         true,
	"small":         true,
	"large":         true,
	"long":          true,
	"short":         true,
	"little":        true,
	"big":           true,
	"different":     true,
	"important":     true,
	"possible":      true,
	"necessary":     true,
	"similar":       true,
	"common":        true,
	"simple":        true,
	"complex":       true,
	"easy":          true,
	"hard":          true,
	"difficult":     true,
	"getting":       true,
	"using":         true,
	"adding":        true,
	"creating":      true,
	"making":        true,
	"writing":       true,
	"reading":       true,
	"running":       true,
	"looking":       true,
	"trying":        true,
	"knowing":       true,
	"thinking":      true,
	"working":       true,
	"playing":       true,
	"starting":      true,
	"stopping":      true,
	"following":     true,
	"based":         true,
	"called":        true,
	"named":         true,
	"know":          true,
	"knows":         true,
	"see":           true,
	"sees":          true,
	"make":          true,
	"makes":         true,
	"take":          true,
	"takes":         true,
	"come":          true,
	"comes":         true,
	"go":            true,
	"goes":          true,
	"going":         true,
	"want":          true,
	"wants":         true,
	"use":           true,
	"uses":          true,
	"used":          true,
	"need":          true,
	"needs":         true,
	"try":           true,
	"tries":         true,
	"find":          true,
	"finds":         true,
	"ask":           true,
	"asks":          true,
	"work":          true,
	"works":         true,
	"call":          true,
	"calls":         true,
	"start":         true,
	"starts":        true,
	"show":          true,
	"shows":         true,
	"hear":          true,
	"hears":         true,
	"play":          true,
	"plays":         true,
	"run":           true,
	"runs":          true,
	"move":          true,
	"moves":         true,
	"live":          true,
	"believe":       true,
	"believe in":    true,
	"hold":          true,
	"holds":         true,
	"bring":         true,
	"happens":       true,
	"write":         true,
	"writes":        true,
	"provide":       true,
	"provides":      true,
	"sit":           true,
	"stands":        true,
	"lose":          true,
	"pay":           true,
	"meet":          true,
	"include":       true,
	"includes":      true,
	"continue":      true,
	"set":           true,
	"sets":          true,
	"learn":         true,
	"learns":        true,
	"change":        true,
	"changes":       true,
	"lead":          true,
	"leads":         true,
	"understand":    true,
	"understands":   true,
	"watch":         true,
	"watches":       true,
	"follow":        true,
	"follows":       true,
	"stop":          true,
	"stops":         true,
	"create":        true,
	"creates":       true,
	"builds":        true,
	"solve":         true,
	"solves":        true,
	"check":         true,
	"checks":        true,
	"download":      true,
	"downloads":     true,
	"upload":        true,
	"uploads":       true,
	"print":         true,
	"prints":        true,
	"search":        true,
	"searches":      true,
	"click":         true,
	"clicks":        true,
	"press":         true,
	"presses":       true,
	"return":        true,
	"returns":       true,
	"define":        true,
	"defines":       true,
	"delete":        true,
	"deletes":       true,
	"update":        true,
	"updates":       true,
	"insert":        true,
	"inserts":       true,
	"select":        true,
	"selects":       true,
	"install":       true,
	"installs":      true,
	"configure":     true,
	"configures":    true,
	"implement":     true,
	"implements":    true,
	"generate":      true,
	"generates":     true,
	"execute":       true,
	"executes":      true,
	"enable":        true,
	"enables":       true,
	"disable":       true,
	"disables":      true,
	"connect":       true,
	"connects":      true,
	"receive":       true,
	"receives":      true,
	"accept":        true,
	"accepts":       true,
	"reject":        true,
	"rejects":       true,
	"allow":         true,
	"allows":        true,
	"support":       true,
	"supports":      true,
	"require":       true,
	"requires":      true,
	"contains":      true,
	"represents":    true,
	"means":         true,
	"helps":         true,
	"gives":         true,
}

// LooksLikeIndex svarar på om en text ser ut som ett register över
// sidnumreferenser. I ett register står siffror tätt, i vanlig brödtext
// förekommer de sparsamt.
func LooksLikeIndex(s string) bool {
	if len(s) < 10_000 {
		return false
	}
	words := Tokens(s)
	if len(words) == 0 {
		return false
	}
	withDigit := 0
	for _, word := range words {
		for _, r := range word {
			if r >= '0' && r <= '9' {
				withDigit++
				break
			}
		}
	}
	return float64(withDigit)/float64(len(words)) > 0.25
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
			if !LooksLikeProse(definition) {
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
