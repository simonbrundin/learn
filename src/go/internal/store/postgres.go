package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/simon/repo-template/internal/model"
)

// postgresSchema beskriver samma struktur som SQLite-schemat, men med
// Postgres-typer. JSON-kolumner används för listor.
const postgresSchema = `
CREATE TABLE IF NOT EXISTS books (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT NOT NULL,
    author       TEXT NOT NULL DEFAULT '',
    source_path  TEXT NOT NULL DEFAULT '',
    language     TEXT NOT NULL DEFAULT 'en',
    description  TEXT NOT NULL DEFAULT '',
    imported_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chapters (
    id       BIGSERIAL PRIMARY KEY,
    book_id  BIGINT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    idx      INTEGER NOT NULL,
    title    TEXT NOT NULL DEFAULT '',
    text     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS concepts (
    id         BIGSERIAL PRIMARY KEY,
    book_id    BIGINT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_id BIGINT NOT NULL DEFAULT 0,
    name       TEXT NOT NULL,
    definition TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL DEFAULT '',
    kind       TEXT NOT NULL DEFAULT 'concept',
    keywords   JSONB NOT NULL DEFAULT '[]'::jsonb,
    quotes     JSONB NOT NULL DEFAULT '[]'::jsonb,
    importance DOUBLE PRECISION NOT NULL DEFAULT 0,
    mentions   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS edges (
    id         BIGSERIAL PRIMARY KEY,
    source_id  BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    target_id  BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    relation   TEXT NOT NULL DEFAULT 'related',
    weight     DOUBLE PRECISION NOT NULL DEFAULT 1,
    book_id    BIGINT NOT NULL DEFAULT 0,
    UNIQUE (source_id, target_id, relation)
);

CREATE TABLE IF NOT EXISTS questions (
    id          BIGSERIAL PRIMARY KEY,
    book_id     BIGINT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    concept_id  BIGINT NOT NULL DEFAULT 0,
    kind        TEXT NOT NULL DEFAULT 'multiple_choice',
    prompt      TEXT NOT NULL,
    options     JSONB NOT NULL DEFAULT '[]'::jsonb,
    answer      TEXT NOT NULL DEFAULT '',
    explanation TEXT NOT NULL DEFAULT '',
    difficulty  TEXT NOT NULL DEFAULT 'medium'
);

CREATE TABLE IF NOT EXISTS attempts (
    id            BIGSERIAL PRIMARY KEY,
    question_id   BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    concept_id    BIGINT NOT NULL DEFAULT 0,
    correct       BOOLEAN NOT NULL DEFAULT FALSE,
    answered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ease_factor   DOUBLE PRECISION NOT NULL DEFAULT 2.5,
    interval_days DOUBLE PRECISION NOT NULL DEFAULT 0,
    due_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    repetitions   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS ideas (
    id      BIGSERIAL PRIMARY KEY,
    book_id BIGINT NOT NULL DEFAULT 0,
    kind    TEXT NOT NULL DEFAULT 'combination',
    prompt  TEXT NOT NULL,
    basis   JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS import_failures (
    source_path TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT '',
    reason      TEXT NOT NULL DEFAULT '',
    failed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_concepts_book ON concepts(book_id);
CREATE INDEX IF NOT EXISTS idx_concepts_name ON concepts(name);
CREATE INDEX IF NOT EXISTS idx_chapters_book ON chapters(book_id);
CREATE INDEX IF NOT EXISTS idx_questions_book ON questions(book_id);
CREATE INDEX IF NOT EXISTS idx_attempts_question ON attempts(question_id);
`

// Postgres lagrar kunskapsbasen i PostgreSQL.
// Det är implementationen som används i klusret, där SQLite inte passar.
type Postgres struct {
	db   *sql.DB
	dsn  string
	path string
}

// Postgres uppfyller Repository.
var _ Repository = (*Postgres)(nil)

// OpenPostgres öppnar en PostgreSQL-databas och skapar schemat om det
// saknas. DSN-formatet är det vanliga postgres://user:pass@host:port/db.
func OpenPostgres(dsn string) (*Postgres, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("tom databasadress")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("kunde inte öppna databasen: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("kunde inte ansluta till databasen: %w", err)
	}
	if _, err := db.Exec(postgresSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("kunde inte initiera databasen: %w", err)
	}
	return &Postgres{db: db, dsn: dsn, path: dsn}, nil
}

// Close stänger anslutningen.
func (p *Postgres) Close() error { return p.db.Close() }

// Path ger identifieraren för databasen.
func (p *Postgres) Path() string { return p.path }

// ---------------------------------------------------------------------------
// Böcker
// ---------------------------------------------------------------------------

// AddBook sparar en ny bok och sätter dess id.
func (p *Postgres) AddBook(book *model.Book) error {
	if book.ImportedAt.IsZero() {
		book.ImportedAt = time.Now()
	}
	return p.db.QueryRow(
		`INSERT INTO books (title, author, source_path, language, description, imported_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		book.Title, book.Author, book.SourcePath, book.Language, book.Description,
		book.ImportedAt).Scan(&book.ID)
}

// ListBooks returnerar alla böcker, senast importerade först.
func (p *Postgres) ListBooks() ([]model.Book, error) {
	rows, err := p.db.Query(`SELECT id, title, author, source_path, language, description, imported_at
		FROM books ORDER BY imported_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var books []model.Book
	for rows.Next() {
		var book model.Book
		if err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.SourcePath,
			&book.Language, &book.Description, &book.ImportedAt); err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

// DeleteBook tar bort en bok med tillhörande kapitel, koncept, relationer,
// frågor och idéer.
func (p *Postgres) DeleteBook(bookID int64) error {
	statements := []string{
		"DELETE FROM attempts WHERE question_id IN (SELECT id FROM questions WHERE book_id = $1)",
		"DELETE FROM questions WHERE book_id = $1",
		"DELETE FROM edges WHERE book_id = $1",
		"DELETE FROM edges WHERE source_id IN (SELECT id FROM concepts WHERE book_id = $1) OR target_id IN (SELECT id FROM concepts WHERE book_id = $1)",
		"DELETE FROM concepts WHERE book_id = $1",
		"DELETE FROM chapters WHERE book_id = $1",
		"DELETE FROM ideas WHERE book_id = $1",
		"DELETE FROM books WHERE id = $1",
	}
	for _, statement := range statements {
		if _, err := p.db.Exec(statement, bookID); err != nil {
			return fmt.Errorf("kunde inte ta bort bok %d: %w", bookID, err)
		}
	}
	return nil
}

// RecordFailure sparar att en bok inte kunde importeras.
func (p *Postgres) RecordFailure(failure model.ImportFailure) error {
	if failure.FailedAt.IsZero() {
		failure.FailedAt = time.Now()
	}
	_, err := p.db.Exec(`
		INSERT INTO import_failures (source_path, title, reason, failed_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_path) DO UPDATE SET
			title = excluded.title, reason = excluded.reason, failed_at = excluded.failed_at`,
		failure.SourcePath, failure.Title, failure.Reason, failure.FailedAt)
	return err
}

// Failures listar de böcker som inte kunde importeras.
func (p *Postgres) Failures() ([]model.ImportFailure, error) {
	rows, err := p.db.Query(`SELECT source_path, title, reason, failed_at
		FROM import_failures ORDER BY failed_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var failures []model.ImportFailure
	for rows.Next() {
		var failure model.ImportFailure
		if err := rows.Scan(&failure.SourcePath, &failure.Title, &failure.Reason,
			&failure.FailedAt); err != nil {
			return nil, err
		}
		failures = append(failures, failure)
	}
	return failures, rows.Err()
}

// ClearFailure tar bort en registrerad misslyckande.
func (p *Postgres) ClearFailure(sourcePath string) error {
	_, err := p.db.Exec("DELETE FROM import_failures WHERE source_path = $1", sourcePath)
	return err
}

// FindBook söker en bok på titel, författare eller id.
func (p *Postgres) FindBook(query string) (*model.Book, error) {
	books, err := p.ListBooks()
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
func (p *Postgres) AddChapter(chapter *model.Chapter) error {
	return p.db.QueryRow(
		`INSERT INTO chapters (book_id, idx, title, text) VALUES ($1, $2, $3, $4) RETURNING id`,
		chapter.BookID, chapter.Index, chapter.Title, chapter.Text).Scan(&chapter.ID)
}

// Chapters returnerar kapitlen för en bok i ordning.
func (p *Postgres) Chapters(bookID int64) ([]model.Chapter, error) {
	rows, err := p.db.Query(
		`SELECT id, book_id, idx, title, text FROM chapters WHERE book_id = $1 ORDER BY idx`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chapters []model.Chapter
	for rows.Next() {
		var chapter model.Chapter
		if err := rows.Scan(&chapter.ID, &chapter.BookID, &chapter.Index, &chapter.Title,
			&chapter.Text); err != nil {
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
func (p *Postgres) AddConcepts(concepts []model.Concept) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statement, err := tx.Prepare(
		`INSERT INTO concepts (book_id, chapter_id, name, definition, summary, kind, keywords, quotes, importance, mentions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for i := range concepts {
		keywords, _ := json.Marshal(concepts[i].Keywords)
		quotes, _ := json.Marshal(concepts[i].Quotes)
		if err := statement.QueryRow(concepts[i].BookID, concepts[i].ChapterID, concepts[i].Name,
			concepts[i].Definition, concepts[i].Summary, concepts[i].Kind, keywords,
			quotes, concepts[i].Importance, concepts[i].Mentions).Scan(&concepts[i].ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Concepts returnerar koncepten, filtrerade på bok om bookID anges.
func (p *Postgres) Concepts(bookID int64) ([]model.Concept, error) {
	query := `SELECT id, book_id, chapter_id, name, definition, summary, kind, keywords, quotes, importance, mentions
		FROM concepts`
	args := []any{}
	if bookID > 0 {
		query += " WHERE book_id = $1"
		args = append(args, bookID)
	}
	rows, err := p.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var concepts []model.Concept
	for rows.Next() {
		var concept model.Concept
		var keywords, quotes []byte
		if err := rows.Scan(&concept.ID, &concept.BookID, &concept.ChapterID, &concept.Name,
			&concept.Definition, &concept.Summary, &concept.Kind, &keywords, &quotes,
			&concept.Importance, &concept.Mentions); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(keywords, &concept.Keywords)
		_ = json.Unmarshal(quotes, &concept.Quotes)
		concepts = append(concepts, concept)
	}
	return concepts, rows.Err()
}

// AllConcepts returnerar koncepten i hela databasen.
func (p *Postgres) AllConcepts() ([]model.Concept, error) {
	return p.Concepts(0)
}

// AllEdges returnerar alla relationer i databasen.
func (p *Postgres) AllEdges() ([]model.Edge, error) {
	return p.Edges(0)
}

// FindConcept söker ett koncept på namn.
func (p *Postgres) FindConcept(name string, bookID int64) (model.Concept, bool) {
	concepts, err := p.Concepts(bookID)
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
func (p *Postgres) AddEdge(edge model.Edge) error {
	_, err := p.db.Exec(
		`INSERT INTO edges (source_id, target_id, relation, weight, book_id) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (source_id, target_id, relation) DO NOTHING`,
		edge.SourceID, edge.TargetID, edge.Relation, edge.Weight, edge.BookID)
	return err
}

// Edges returnerar relationerna, filtrerade på bok om bookID anges.
func (p *Postgres) Edges(bookID int64) ([]model.Edge, error) {
	query := `SELECT id, source_id, target_id, relation, weight, book_id FROM edges`
	args := []any{}
	if bookID > 0 {
		query += " WHERE book_id = $1"
		args = append(args, bookID)
	}
	rows, err := p.db.Query(query, args...)
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
func (p *Postgres) AddQuestions(questions []model.Question) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statement, err := tx.Prepare(
		`INSERT INTO questions (book_id, concept_id, kind, prompt, options, answer, explanation, difficulty)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for i := range questions {
		options, _ := json.Marshal(questions[i].Options)
		if err := statement.QueryRow(questions[i].BookID, questions[i].ConceptID, questions[i].Kind,
			questions[i].Prompt, options, questions[i].Answer, questions[i].Explanation,
			questions[i].Difficulty).Scan(&questions[i].ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Questions returnerar frågorna, filtrerade på bok om bookID anges.
func (p *Postgres) Questions(bookID int64) ([]model.Question, error) {
	query := `SELECT id, book_id, concept_id, kind, prompt, options, answer, explanation, difficulty FROM questions`
	args := []any{}
	if bookID > 0 {
		query += " WHERE book_id = $1"
		args = append(args, bookID)
	}
	rows, err := p.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var questions []model.Question
	for rows.Next() {
		var question model.Question
		var options []byte
		if err := rows.Scan(&question.ID, &question.BookID, &question.ConceptID, &question.Kind,
			&question.Prompt, &options, &question.Answer, &question.Explanation,
			&question.Difficulty); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(options, &question.Options)
		questions = append(questions, question)
	}
	return questions, rows.Err()
}

// LatestAttempt returnerar det senaste svaret på en fråga.
func (p *Postgres) LatestAttempt(questionID int64) (*model.Attempt, bool) {
	row := p.db.QueryRow(
		`SELECT id, question_id, concept_id, correct, answered_at, ease_factor, interval_days, due_at, repetitions
		 FROM attempts WHERE question_id = $1 ORDER BY answered_at DESC LIMIT 1`, questionID)
	var attempt model.Attempt
	if err := row.Scan(&attempt.ID, &attempt.QuestionID, &attempt.ConceptID, &attempt.Correct,
		&attempt.AnsweredAt, &attempt.EaseFactor, &attempt.IntervalDays, &attempt.DueAt,
		&attempt.Repetitions); err != nil {
		return nil, false
	}
	return &attempt, true
}

// AddAttempt sparar ett svar och sätter dess id.
func (p *Postgres) AddAttempt(attempt *model.Attempt) error {
	if attempt.AnsweredAt.IsZero() {
		attempt.AnsweredAt = time.Now()
	}
	if attempt.DueAt.IsZero() {
		attempt.DueAt = attempt.AnsweredAt
	}
	return p.db.QueryRow(
		`INSERT INTO attempts (question_id, concept_id, correct, answered_at, ease_factor, interval_days, due_at, repetitions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		attempt.QuestionID, attempt.ConceptID, attempt.Correct, attempt.AnsweredAt,
		attempt.EaseFactor, attempt.IntervalDays, attempt.DueAt, attempt.Repetitions).Scan(&attempt.ID)
}

// ---------------------------------------------------------------------------
// Idéer
// ---------------------------------------------------------------------------

// AddIdea sparar en idé.
func (p *Postgres) AddIdea(idea model.Idea) error {
	basis, _ := json.Marshal(idea.Basis)
	_, err := p.db.Exec(
		"INSERT INTO ideas (book_id, kind, prompt, basis) VALUES ($1, $2, $3, $4)",
		idea.BookID, idea.Kind, idea.Prompt, basis)
	return err
}
