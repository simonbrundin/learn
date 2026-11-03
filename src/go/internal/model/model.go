// Package model innehåller domänmodellerna för kunskapssystemet.
//
// Modellerna är speglar av SQLite-raderna och används av alla lager:
// parsning, extraktion, lagring och presentation.
package model

import "time"

// Book är en importerad bok.
type Book struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	SourcePath  string    `json:"source_path"`
	Language    string    `json:"language"`
	Description string    `json:"description"`
	ImportedAt  time.Time `json:"imported_at"`
}

// Slug ger en filnamnsvänlig identifierare, t.ex. "thinking-fast-and-slow".
func (b Book) Slug() string {
	base := []rune(b.Title)
	cleaned := make([]rune, 0, len(base))
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			cleaned = append(cleaned, r)
		case r >= 'A' && r <= 'Z':
			cleaned = append(cleaned, r+('a'-'A'))
		case r == ' ':
			cleaned = append(cleaned, '-')
		default:
			cleaned = append(cleaned, ' ')
		}
	}
	return string(cleaned)
}

// Chapter är ett kapitel eller en sektion med sin fulltext.
type Chapter struct {
	ID     int64  `json:"id"`
	BookID int64  `json:"book_id"`
	Index  int    `json:"index"`
	Title  string `json:"title"`
	Text   string `json:"text"`
}

// Concept är ett nyckelkoncept som extraherats ur en bok.
type Concept struct {
	ID         int64    `json:"id"`
	BookID     int64    `json:"book_id"`
	ChapterID  int64    `json:"chapter_id"`
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Summary    string   `json:"summary"`
	Kind       string   `json:"kind"`
	Keywords   []string `json:"keywords"`
	Quotes     []string `json:"quotes"`
	Importance float64  `json:"importance"`
	Mentions   int      `json:"mentions"`
}

// Edge är en relation mellan två koncept.
type Edge struct {
	ID       int64   `json:"id"`
	SourceID int64   `json:"source_id"`
	TargetID int64   `json:"target_id"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
	BookID   int64   `json:"book_id"`
}

// Question är en genererad kunskapsfråga.
type Question struct {
	ID          int64    `json:"id"`
	BookID      int64    `json:"book_id"`
	ConceptID   int64    `json:"concept_id"`
	Kind        string   `json:"kind"`
	Prompt      string   `json:"prompt"`
	Options     []string `json:"options"`
	Answer      string   `json:"answer"`
	Explanation string   `json:"explanation"`
	Difficulty  string   `json:"difficulty"`
}

// Attempt är ett svar på en fråga med uppdaterad repetitionsplan.
type Attempt struct {
	ID           int64     `json:"id"`
	QuestionID   int64     `json:"question_id"`
	ConceptID    int64     `json:"concept_id"`
	Correct      bool      `json:"correct"`
	AnsweredAt   time.Time `json:"answered_at"`
	EaseFactor   float64   `json:"ease_factor"`
	IntervalDays float64   `json:"interval_days"`
	DueAt        time.Time `json:"due_at"`
	Repetitions  int       `json:"repetitions"`
}

// Idea är en brainstorm-idé baserad på inlärd kunskap.
type Idea struct {
	ID     int64    `json:"id"`
	BookID int64    `json:"book_id"`
	Kind   string   `json:"kind"`
	Prompt string   `json:"prompt"`
	Basis  []string `json:"basis"`
}

// Episode är ett färdigt avsnitt med metadata.
type Episode struct {
	Index  int      `json:"index"`
	Title  string   `json:"title"`
	Text   string   `json:"text"`
	Blocks []string `json:"blocks"`
}
