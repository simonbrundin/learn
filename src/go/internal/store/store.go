// Package store lagrar böcker, koncept, frågor och repetitionsplan i SQLite.
//
// Databasen ligger i ~/.learn/learn.db om ingen annan sökväg anges.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/simon/repo-template/internal/model"
)

const schema = `
CREATE TABLE IF NOT EXISTS books (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL,
    author       TEXT NOT NULL DEFAULT '',
    source_path  TEXT NOT NULL DEFAULT '',
    language     TEXT NOT NULL DEFAULT 'en',
    description  TEXT NOT NULL DEFAULT '',
    imported_at  REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS chapters (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id  INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    idx      INTEGER NOT NULL,
    title    TEXT NOT NULL DEFAULT '',
    text     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS concepts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id    INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_id INTEGER NOT NULL DEFAULT 0,
    name       TEXT NOT NULL,
    definition TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL DEFAULT '',
    kind       TEXT NOT NULL DEFAULT 'concept',
    keywords   TEXT NOT NULL DEFAULT '[]',
    quotes     TEXT NOT NULL DEFAULT '[]',
    importance REAL NOT NULL DEFAULT 0,
    mentions   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS edges (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id  INTEGER NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    target_id  INTEGER NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    relation   TEXT NOT NULL DEFAULT 'related',
    weight     REAL NOT NULL DEFAULT 1,
    book_id    INTEGER NOT NULL DEFAULT 0,
    UNIQUE (source_id, target_id, relation)
);

CREATE TABLE IF NOT EXISTS questions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id     INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    concept_id  INTEGER NOT NULL DEFAULT 0,
    kind        TEXT NOT NULL DEFAULT 'multiple_choice',
    prompt      TEXT NOT NULL,
    options     TEXT NOT NULL DEFAULT '[]',
    answer      TEXT NOT NULL DEFAULT '',
    explanation TEXT NOT NULL DEFAULT '',
    difficulty  TEXT NOT NULL DEFAULT 'medium'
);

CREATE TABLE IF NOT EXISTS attempts (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id   INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    concept_id    INTEGER NOT NULL DEFAULT 0,
    correct       INTEGER NOT NULL DEFAULT 0,
    answered_at   REAL NOT NULL,
    ease_factor   REAL NOT NULL DEFAULT 2.5,
    interval_days REAL NOT NULL DEFAULT 0,
    due_at        REAL NOT NULL,
    repetitions   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ideas (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INTEGER NOT NULL DEFAULT 0,
    kind    TEXT NOT NULL DEFAULT 'combination',
    prompt  TEXT NOT NULL,
    basis   TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_concepts_book ON concepts(book_id);
CREATE INDEX IF NOT EXISTS idx_concepts_name ON concepts(name);
CREATE INDEX IF NOT EXISTS idx_chapters_book ON chapters(book_id);
CREATE INDEX IF NOT EXISTS idx_questions_book ON questions(book_id);
CREATE INDEX IF NOT EXISTS idx_attempts_question ON attempts(question_id);
`

// Store är en tunn wrapper runt SQLite.
type Store struct {
	db   *sql.DB
	path string
}

// DefaultPath ger standardplatsen för databasen.
func DefaultPath() string {
	if override := os.Getenv("LEARN_DB"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "learn.db"
	}
	return filepath.Join(home, ".learn", "learn.db")
}

// Open öppnar (och skapar vid behov) databasen.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("kunde inte skapa databaskatalog: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("kunde inte öppna databasen: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("kunde inte initiera databasen: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

// Close stänger databasen.
func (s *Store) Close() error { return s.db.Close() }

// Path ger sökvägen till databasen.
func (s *Store) Path() string { return s.path }

// ---------------------------------------------------------------------------
// Böcker
// ---------------------------------------------------------------------------

// AddBook sparar en ny bok och sätter dess id.
func (s *Store) AddBook(book *model.Book) error {
	result, err := s.db.Exec(
		"INSERT INTO books (title, author, source_path, language, description, imported_at) VALUES (?, ?, ?, ?, ?, ?)",
		book.Title, book.Author, book.SourcePath, book.Language, book.Description,
		float64(book.ImportedAt.UnixNano())/1e9,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	book.ID = id
	return nil
}

// ListBooks returnerar alla böcker, senast importerade först.
func (s *Store) ListBooks() ([]model.Book, error) {
	rows, err := s.db.Query("SELECT * FROM books ORDER BY imported_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var books []model.Book
	for rows.Next() {
		var book model.Book
		var imported float64
		if err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.SourcePath,
			&book.Language, &book.Description, &imported); err != nil {
			return nil, err
		}
		book.ImportedAt = time.Unix(0, int64(imported*1e9))
		books = append(books, book)
	}
	return books, rows.Err()
}

// FindBook söker en bok på titel, författare eller id.
func (s *Store) FindBook(query string) (*model.Book, error) {
	books, err := s.ListBooks()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	for i := range books {
		if strings.Contains(strings.ToLower(books[i].Title), needle) ||
			strings.Contains(strings.ToLower(books[i].Author), needle) {
			return &books[i], nil
		}
	}
	for i := range books {
		if fmt.Sprintf("%d", books[i].ID) == strings.TrimSpace(query) {
			return &books[i], nil
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Kapitel
// ---------------------------------------------------------------------------

// AddChapter sparar ett kapitel och sätter dess id.
func (s *Store) AddChapter(chapter *model.Chapter) error {
	result, err := s.db.Exec(
		"INSERT INTO chapters (book_id, idx, title, text) VALUES (?, ?, ?, ?)",
		chapter.BookID, chapter.Index, chapter.Title, chapter.Text,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	chapter.ID = id
	return nil
}

// Chapters returnerar kapitlen för en bok i ordning.
func (s *Store) Chapters(bookID int64) ([]model.Chapter, error) {
	rows, err := s.db.Query("SELECT id, book_id, idx, title, text FROM chapters WHERE book_id = ? ORDER BY idx", bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chapters []model.Chapter
	for rows.Next() {
		var chapter model.Chapter
		if err := rows.Scan(&chapter.ID, &chapter.BookID, &chapter.Index, &chapter.Title, &chapter.Text); err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}
	return chapters, rows.Err()
}

// ---------------------------------------------------------------------------
// Koncept och relationer
// ---------------------------------------------------------------------------

// AddConcepts sparar koncept och sätter deras id.
func (s *Store) AddConcepts(concepts []model.Concept) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statement, err := tx.Prepare(
		"INSERT INTO concepts (book_id, chapter_id, name, definition, summary, kind, keywords, quotes, importance, mentions) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer statement.Close()
	for i := range concepts {
		keywords, _ := json.Marshal(concepts[i].Keywords)
		quotes, _ := json.Marshal(concepts[i].Quotes)
		result, err := statement.Exec(concepts[i].BookID, concepts[i].ChapterID, concepts[i].Name,
			concepts[i].Definition, concepts[i].Summary, concepts[i].Kind, string(keywords),
			string(quotes), concepts[i].Importance, concepts[i].Mentions)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		concepts[i].ID = id
	}
	return tx.Commit()
}

// Concepts returnerar koncepten, filtrerade på bok om bookID anges.
func (s *Store) Concepts(bookID int64) ([]model.Concept, error) {
	query := "SELECT id, book_id, chapter_id, name, definition, summary, kind, keywords, quotes, importance, mentions FROM concepts"
	args := []any{}
	if bookID > 0 {
		query += " WHERE book_id = ?"
		args = append(args, bookID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var concepts []model.Concept
	for rows.Next() {
		var concept model.Concept
		var keywords, quotes string
		if err := rows.Scan(&concept.ID, &concept.BookID, &concept.ChapterID, &concept.Name,
			&concept.Definition, &concept.Summary, &concept.Kind, &keywords, &quotes,
			&concept.Importance, &concept.Mentions); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(keywords), &concept.Keywords)
		_ = json.Unmarshal([]byte(quotes), &concept.Quotes)
		concepts = append(concepts, concept)
	}
	return concepts, rows.Err()
}

// FindConcept söker ett koncept på namn.
func (s *Store) FindConcept(name string, bookID int64) (model.Concept, bool) {
	concepts, err := s.Concepts(bookID)
	if err != nil {
		return model.Concept{}, false
	}
	needle := strings.ToLower(strings.TrimSpace(name))
	for _, concept := range concepts {
		if strings.ToLower(concept.Name) == needle {
			return concept, true
		}
	}
	for _, concept := range concepts {
		if strings.Contains(strings.ToLower(concept.Name), needle) {
			return concept, true
		}
	}
	for _, concept := range concepts {
		if strings.Contains(strings.ToLower(concept.Definition+" "+concept.Summary), needle) {
			return concept, true
		}
	}
	return model.Concept{}, false
}

// AddEdge sparar en relation mellan koncept.
func (s *Store) AddEdge(edge model.Edge) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO edges (source_id, target_id, relation, weight, book_id) VALUES (?, ?, ?, ?, ?)",
		edge.SourceID, edge.TargetID, edge.Relation, edge.Weight, edge.BookID,
	)
	return err
}

// Edges returnerar relationerna, filtrerade på bok om bookID anges.
func (s *Store) Edges(bookID int64) ([]model.Edge, error) {
	query := "SELECT id, source_id, target_id, relation, weight, book_id FROM edges"
	args := []any{}
	if bookID > 0 {
		query += " WHERE book_id = ?"
		args = append(args, bookID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var edges []model.Edge
	for rows.Next() {
		var edge model.Edge
		if err := rows.Scan(&edge.ID, &edge.SourceID, &edge.TargetID, &edge.Relation,
			&edge.Weight, &edge.BookID); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

// ---------------------------------------------------------------------------
// Frågor och repetitioner
// ---------------------------------------------------------------------------

// AddQuestions sparar frågor och sätter deras id.
func (s *Store) AddQuestions(questions []model.Question) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statement, err := tx.Prepare(
		"INSERT INTO questions (book_id, concept_id, kind, prompt, options, answer, explanation, difficulty) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer statement.Close()
	for i := range questions {
		options, _ := json.Marshal(questions[i].Options)
		result, err := statement.Exec(questions[i].BookID, questions[i].ConceptID, questions[i].Kind,
			questions[i].Prompt, string(options), questions[i].Answer, questions[i].Explanation,
			questions[i].Difficulty)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		questions[i].ID = id
	}
	return tx.Commit()
}

// Questions returnerar frågorna, filtrerade på bok om bookID anges.
func (s *Store) Questions(bookID int64) ([]model.Question, error) {
	query := "SELECT id, book_id, concept_id, kind, prompt, options, answer, explanation, difficulty FROM questions"
	args := []any{}
	if bookID > 0 {
		query += " WHERE book_id = ?"
		args = append(args, bookID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var questions []model.Question
	for rows.Next() {
		var question model.Question
		var options string
		if err := rows.Scan(&question.ID, &question.BookID, &question.ConceptID, &question.Kind,
			&question.Prompt, &options, &question.Answer, &question.Explanation,
			&question.Difficulty); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(options), &question.Options)
		questions = append(questions, question)
	}
	return questions, rows.Err()
}

// LatestAttempt returnerar det senaste svaret på en fråga.
func (s *Store) LatestAttempt(questionID int64) (*model.Attempt, bool) {
	row := s.db.QueryRow(
		"SELECT id, question_id, concept_id, correct, answered_at, ease_factor, interval_days, due_at, repetitions"+
			" FROM attempts WHERE question_id = ? ORDER BY answered_at DESC LIMIT 1", questionID)
	var attempt model.Attempt
	var answeredAt, dueAt float64
	var correct int
	if err := row.Scan(&attempt.ID, &attempt.QuestionID, &attempt.ConceptID, &correct,
		&answeredAt, &attempt.EaseFactor, &attempt.IntervalDays, &dueAt,
		&attempt.Repetitions); err != nil {
		return nil, false
	}
	attempt.Correct = correct != 0
	attempt.AnsweredAt = time.Unix(0, int64(answeredAt*1e9))
	attempt.DueAt = time.Unix(0, int64(dueAt*1e9))
	return &attempt, true
}

// AddAttempt sparar ett svar och sätter dess id.
func (s *Store) AddAttempt(attempt *model.Attempt) error {
	correct := 0
	if attempt.Correct {
		correct = 1
	}
	result, err := s.db.Exec(
		"INSERT INTO attempts (question_id, concept_id, correct, answered_at, ease_factor, interval_days, due_at, repetitions)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		attempt.QuestionID, attempt.ConceptID, correct,
		float64(attempt.AnsweredAt.UnixNano())/1e9, attempt.EaseFactor,
		attempt.IntervalDays, float64(attempt.DueAt.UnixNano())/1e9, attempt.Repetitions,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	attempt.ID = id
	return nil
}

// ---------------------------------------------------------------------------
// Idéer
// ---------------------------------------------------------------------------

// AddIdea sparar en idé.
func (s *Store) AddIdea(idea model.Idea) error {
	basis, _ := json.Marshal(idea.Basis)
	_, err := s.db.Exec("INSERT INTO ideas (book_id, kind, prompt, basis) VALUES (?, ?, ?, ?)",
		idea.BookID, idea.Kind, idea.Prompt, string(basis))
	return err
}
