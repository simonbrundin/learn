package model

import "time"

// InsightTotals sammanfattar hur en bok förhåller sig till resten av
// kunskapsbasen.
type InsightTotals struct {
	// Concepts är antalet koncept i boken.
	Concepts int `json:"concepts"`
	// Unique är koncept som bara finns i denna bok.
	Unique int `json:"unique"`
	// Shared är koncept som också förekommer i andra böcker.
	Shared int `json:"shared"`
	// Unconventional är koncept vars beskrivning avviker från andra böcker.
	Unconventional int `json:"unconventional"`
	// Contradicting är koncept som motsäger andra böckers beskrivning.
	Contradicting int `json:"contradicting"`
	// NovelKeywords är nyckelord som bara förekommer i boken.
	NovelKeywords int `json:"novel_keywords"`
	// Coverage är andelen koncept som är unika, 0–1.
	Coverage float64 `json:"coverage"`
	// OtherBooks är antalet böcker boken jämförs med.
	OtherBooks int `json:"other_books"`
}

// NovelConcept är ett koncept som bara denna bok har.
type NovelConcept struct {
	Name          string   `json:"name"`
	Definition    string   `json:"definition"`
	Summary       string   `json:"summary"`
	Importance    float64  `json:"importance"`
	Mentions      int      `json:"mentions"`
	Keywords      []string `json:"keywords"`
	NovelKeywords []string `json:"novel_keywords"`
}

// SharedVersion är hur samma koncept beskrivs i en annan bok.
type SharedVersion struct {
	BookID     int64   `json:"book_id"`
	BookTitle  string  `json:"book_title"`
	Definition string  `json:"definition"`
	Similarity float64 `json:"similarity"`
	// Verdict är "consistent", "nuance" eller "unconventional".
	Verdict string `json:"verdict"`
	// Negated är sant när den andra boken säger motsatsen.
	Negated bool `json:"negated"`
}

// SharedConcept är ett koncept som förekommer i flera böcker.
type SharedConcept struct {
	Name          string          `json:"name"`
	Importance    float64         `json:"importance"`
	Here          string          `json:"here"`
	Elsewhere     []SharedVersion `json:"elsewhere"`
	Verdict       string          `json:"verdict"`
	NovelKeywords []string        `json:"novel_keywords"`
}

// UnconventionalConcept är ett koncept som avviker från hur andra böcker
// beskriver samma sak.
type UnconventionalConcept struct {
	SharedConcept
	// Note förklarar vad avvikelsen består av.
	Note string `json:"note"`
}

// RelationConflict är en relation som har olika betydelse i olika böcker.
type RelationConflict struct {
	Source        string `json:"source"`
	Target        string `json:"target"`
	HereRelation  string `json:"here_relation"`
	OtherRelation string `json:"other_relation"`
	BookID        int64  `json:"book_id"`
	BookTitle     string `json:"book_title"`
}

// CrossLink kopplar ett koncept i boken till ett koncept i en annan bok.
type CrossLink struct {
	Concept      string   `json:"concept"`
	OtherConcept string   `json:"other_concept"`
	BookID       int64    `json:"book_id"`
	BookTitle    string   `json:"book_title"`
	Keywords     []string `json:"keywords"`
	Strength     float64  `json:"strength"`
}

// BookInsight beskriver vad en bok tillför kunskapsbasen och var den
// avviker från övrig kunskap.
type BookInsight struct {
	Book              Book                    `json:"book"`
	Totals            InsightTotals           `json:"totals"`
	Novel             []NovelConcept          `json:"novel"`
	Shared            []SharedConcept         `json:"shared"`
	Unconventional    []UnconventionalConcept `json:"unconventional"`
	RelationConflicts []RelationConflict      `json:"relation_conflicts"`
	CrossLinks        []CrossLink             `json:"cross_links"`
}

// LibraryInsight sammanfattar kunskapsbasen som helhet.
type LibraryInsight struct {
	Books              int                     `json:"books"`
	Concepts           int                     `json:"concepts"`
	DistinctNames      int                     `json:"distinct_names"`
	SharedNames        int                     `json:"shared_names"`
	CorpusCoverage     float64                 `json:"corpus_coverage"`
	MostShared         []SharedConcept         `json:"most_shared"`
	MostUnconventional []UnconventionalConcept `json:"most_unconventional"`
	GeneratedAt        time.Time               `json:"generated_at"`
}
