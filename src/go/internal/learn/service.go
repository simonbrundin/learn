// Package learn innehåller importpipeline och återuppbyggning av kunskapsgrafen.
package learn

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/simon/repo-template/internal/epub"
	"github.com/simon/repo-template/internal/knowledge"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
)

// ImportResult beskriver resultatet av en bokimport.
type ImportResult struct {
	Book     model.Book
	Chapters []model.Chapter
	Concepts []model.Concept
	Edges    int
}

// Summary ger en kort sammanfattning av importen.
func (r *ImportResult) Summary() string {
	return fmt.Sprintf("%s – %d kapitel, %d koncept, %d relationer",
		r.Book.Title, len(r.Chapters), len(r.Concepts), r.Edges)
}

// ImportBook parsar en e-pub, extraherar kunskap och sparar allt i databasen.
func ImportBook(path string, st *store.Store, maxConcepts int) (*ImportResult, error) {
	parsed, err := epub.Parse(path)
	if err != nil {
		return nil, err
	}

	absolute, absErr := filepath.Abs(path)
	if absErr != nil {
		absolute = path
	}

	book := model.Book{
		Title:       parsed.Title(),
		Author:      parsed.Author(),
		SourcePath:  absolute,
		Language:    parsed.Metadata.Language,
		Description: parsed.Metadata.Description,
		ImportedAt:  time.Now(),
	}
	if book.Title == "" {
		book.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if err := st.AddBook(&book); err != nil {
		return nil, err
	}

	var chapters []model.Chapter
	for index, episode := range parsed.Episodes {
		chapter := model.Chapter{
			BookID: book.ID,
			Index:  index,
			Title:  episode.Title,
			Text:   episode.Text,
		}
		if err := st.AddChapter(&chapter); err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}

	extractor := knowledge.NewConceptExtractor(maxConcepts)
	concepts := extractor.Extract(parsed.Episodes, book.ID)
	for i := range concepts {
		concepts[i].ChapterID = chapterFor(concepts[i], chapters).ID
	}
	if err := st.AddConcepts(concepts); err != nil {
		return nil, err
	}

	graph := knowledge.NewGraph()
	graph.BuildFromContexts(concepts, extractor.Contexts(), book.ID)
	for _, edge := range graph.Edges() {
		if err := st.AddEdge(edge); err != nil {
			return nil, err
		}
	}

	return &ImportResult{
		Book:     book,
		Chapters: chapters,
		Concepts: concepts,
		Edges:    len(graph.Edges()),
	}, nil
}

// chapterFor väljer det kapitel där konceptet nämns oftast, med fallback
// till ett proportionellt kapitel.
func chapterFor(concept model.Concept, chapters []model.Chapter) model.Chapter {
	if len(chapters) == 0 {
		return model.Chapter{}
	}
	for _, chapter := range chapters {
		if strings.Contains(strings.ToLower(chapter.Title), strings.ToLower(concept.Name)) {
			return chapter
		}
	}
	index := len(chapters) / 2
	if index >= len(chapters) {
		index = len(chapters) - 1
	}
	return chapters[index]
}

// BuildGraph bygger en kunskapsgraf från det som ligger i databasen.
func BuildGraph(st *store.Store, bookID int64) (*knowledge.Graph, error) {
	concepts, err := st.Concepts(bookID)
	if err != nil {
		return nil, err
	}
	edges, err := st.Edges(bookID)
	if err != nil {
		return nil, err
	}
	graph := knowledge.NewGraph()
	for _, concept := range concepts {
		graph.AddNode(concept)
	}
	for _, edge := range edges {
		graph.AddEdge(edge.SourceID, edge.TargetID, edge.Relation, edge.Weight, edge.BookID)
	}
	return graph, nil
}
