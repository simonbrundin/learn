package text

import (
	"strings"
	"testing"
)

const sample = "System 1 is the fast thinking. It runs without effort. " +
	"Unlike System 2, System 1 is automatic. " +
	"The key insight is that feeling certain is not the same as being right."

func TestSentencesSplitsOnPunctuation(t *testing.T) {
	got := Sentences(sample)
	if len(got) != 4 {
		t.Fatalf("förväntade 4 meningar, fick %d: %v", len(got), got)
	}
}

func TestKeywordsExcludeStopwords(t *testing.T) {
	words := Keywords(sample, 10)
	if !contains(words, "system") {
		t.Fatalf("förväntade nyckelordet 'system', fick %v", words)
	}
	if contains(words, "the") || contains(words, "is") {
		t.Fatalf("stoppord filtrerades inte bort: %v", words)
	}
}

func TestFindDefinitions(t *testing.T) {
	definitions := FindDefinitions(sample, 5)
	found := map[string]string{}
	for _, definition := range definitions {
		found[definition.Term] = definition.Definition
	}
	if _, ok := found["System 1"]; !ok {
		t.Fatalf("hittade inte definitionen för System 1: %v", found)
	}
	if !strings.Contains(found["System 1"], "fast thinking") {
		t.Fatalf("oväntad definition: %q", found["System 1"])
	}
}

func TestFindQuotesUsesMarkers(t *testing.T) {
	quotes := FindQuotes(sample, 5)
	found := false
	for _, quote := range quotes {
		if strings.Contains(strings.ToLower(quote), "key insight") {
			found = true
		}
	}
	if !found {
		t.Fatalf("hittade inget citat med markören 'key insight': %v", quotes)
	}
}

func TestSummarizeKeepsShortTextIntact(t *testing.T) {
	short := "Only one sentence here."
	if got := Summarize(short, 5); got != short {
		t.Fatalf("förväntade oförändrad text, fick %q", got)
	}
}

func TestSummarizeLimitsSentenceCount(t *testing.T) {
	got := Summarize(sample, 2)
	if len(Sentences(got)) != 2 {
		t.Fatalf("förväntade 2 meningar, fick %d", len(Sentences(got)))
	}
}

func TestNormalizeCollapsesWhitespace(t *testing.T) {
	if got := Normalize("  a   b \n c "); got != "a b c" {
		t.Fatalf("oväntad normalisering: %q", got)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
