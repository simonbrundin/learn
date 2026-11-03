package store

import "github.com/simon/repo-template/internal/model"

// Repository beskriver de databasåtgärder som API:t och CLI:t behöver.
// Store (SQLite) är implementationen som används i dag.
type Repository interface {
	// Skrivning
	AddBook(book *model.Book) error
	AddChapter(chapter *model.Chapter) error
	AddConcepts(concepts []model.Concept) error
	AddEdge(edge model.Edge) error
	AddQuestions(questions []model.Question) error
	AddAttempt(attempt *model.Attempt) error
	AddIdea(idea model.Idea) error

	// Läsning
	ListBooks() ([]model.Book, error)
	FindBook(query string) (*model.Book, error)
	Chapters(bookID int64) ([]model.Chapter, error)
	Concepts(bookID int64) ([]model.Concept, error)
	AllConcepts() ([]model.Concept, error)
	FindConcept(name string, bookID int64) (model.Concept, bool)
	Edges(bookID int64) ([]model.Edge, error)
	AllEdges() ([]model.Edge, error)
	Questions(bookID int64) ([]model.Question, error)
	LatestAttempt(questionID int64) (*model.Attempt, bool)

	// Borttagning
	DeleteBook(bookID int64) error

	// Misslyckade importer
	RecordFailure(failure model.ImportFailure) error
	Failures() ([]model.ImportFailure, error)
	ClearFailure(sourcePath string) error

	// Path ger en beskrivning av var lagringen ligger.
	Path() string

	Close() error
}

// Store uppfyller Repository.
var _ Repository = (*Store)(nil)
