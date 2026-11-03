// Package api exponerar kunskapsbasen över HTTP.
//
// Handlers arbetar mot store.Repository, vilket gör att samma logik kan
// användas från CLI:t och från REST-API:t.
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/simon/repo-template/internal/ideas"
	"github.com/simon/repo-template/internal/knowledge"
	"github.com/simon/repo-template/internal/learn"
	"github.com/simon/repo-template/internal/learning"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
)

// Handlers innehåller API:t:s handlare.
type Handlers struct {
	repo store.Repository
	now  func() time.Time
}

// New skapar handlare för ett repository.
func New(repo store.Repository) *Handlers {
	return &Handlers{repo: repo, now: time.Now}
}

// Register lägger upp kunskapsbasens routes på given router.
func (h *Handlers) Register(r gin.IRouter) {
	r.POST("/books/import", h.ImportBook)
	r.GET("/books", h.ListBooks)
	r.GET("/books/:bookId", h.GetBook)
	r.GET("/books/:bookId/concepts", h.ListConcepts)
	r.GET("/books/:bookId/graph", h.GetGraph)
	r.GET("/books/:bookId/quiz", h.GetQuiz)
	r.GET("/books/:bookId/ideas", h.GetIdeas)
	r.GET("/concepts/:conceptId/explain", h.ExplainConcept)
	r.GET("/concepts/:conceptId/related", h.RelatedConcepts)
	r.GET("/ask", h.Ask)
	r.GET("/connect", h.Connect)
	r.POST("/questions/:questionId/attempt", h.PostAttempt)
}

// ---------------------------------------------------------------------------
// Svarstyper
// ---------------------------------------------------------------------------

// BookListResponse svarar på listan av böcker.
type BookListResponse struct {
	Books []model.Book `json:"books"`
}

// ChapterSummary är en kort beskrivning av ett kapitel.
type ChapterSummary struct {
	ID    int64  `json:"id"`
	Index int    `json:"index"`
	Title string `json:"title"`
	Words int    `json:"words"`
}

// BookDetailResponse svarar på en enskild bok.
type BookDetailResponse struct {
	Book          model.Book       `json:"book"`
	Chapters      []ChapterSummary `json:"chapters"`
	ConceptCount  int              `json:"concept_count"`
	EdgeCount     int              `json:"edge_count"`
	QuestionCount int              `json:"question_count"`
}

// ConceptListResponse svarar på en lista koncept.
type ConceptListResponse struct {
	Concepts []model.Concept `json:"concepts"`
}

// GraphNode är en nod i grafen.
type GraphNode struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Importance float64 `json:"importance"`
	Definition string  `json:"definition"`
}

// GraphEdge är en kant i grafen.
type GraphEdge struct {
	Source   int64   `json:"source"`
	Target   int64   `json:"target"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}

// GraphResponse svarar med hela grafen.
type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// QuizResponse svarar med frågor.
type QuizResponse struct {
	Questions []model.Question `json:"questions"`
}

// AttemptRequest är svaret på en fråga.
type AttemptRequest struct {
	Correct bool   `json:"correct"`
	Answer  string `json:"answer"`
}

// AttemptResponse svarar med den uppdaterade repetitionsplanen.
type AttemptResponse struct {
	Attempt model.Attempt `json:"attempt"`
	NextDue string        `json:"next_due"`
}

// IdeasResponse svarar med brainstorm-idéer.
type IdeasResponse struct {
	Ideas []model.Idea `json:"ideas"`
}

// AnswerResponse svarar på en fråga mot kunskapsbasen.
type AnswerResponse struct {
	Answer   string          `json:"answer"`
	Concepts []model.Concept `json:"concepts"`
}

// PathNode är ett steg i en kopplingsväg.
type PathNode struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PathResponse svarar med en kopplingsväg.
type PathResponse struct {
	Path []PathNode `json:"path"`
}

// ErrorResponse är standardfelet.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ---------------------------------------------------------------------------
// Böcker
// ---------------------------------------------------------------------------

// ImportBook godoc
// @Summary		Importera en e-pub
// @Description	Läser en e-pub från serverns filsystem och bygger kunskapsbasen
// @Tags			books
// @Accept			json
// @Produce		json
// @Param			path	body		ImportRequest	true	"Sökväg till e-pub"
// @Success		201		{object}	BookDetailResponse
// @Failure		400		{object}	ErrorResponse
// @Failure		500		{object}	ErrorResponse
// @Router			/books/import [post]
func (h *Handlers) ImportBook(c *gin.Context) {
	var request ImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ogiltig begäran"})
		return
	}
	if strings.TrimSpace(request.Path) == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "path krävs"})
		return
	}
	result, err := learn.ImportBook(request.Path, h.repo, request.MaxConcepts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	detail, err := h.bookDetail(result.Book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, detail)
}

// ImportRequest beskriver en bokimport.
type ImportRequest struct {
	Path        string `json:"path"`
	MaxConcepts int    `json:"max_concepts"`
}

// ListBooks godoc
// @Summary		Lista böcker
// @Description	Returnerar alla importerade böcker, senast importerade först
// @Tags			books
// @Produce		json
// @Success		200	{object}	BookListResponse
// @Failure		500	{object}	ErrorResponse
// @Router			/books [get]
func (h *Handlers) ListBooks(c *gin.Context) {
	books, err := h.repo.ListBooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if books == nil {
		books = []model.Book{}
	}
	c.JSON(http.StatusOK, BookListResponse{Books: books})
}

// GetBook godoc
// @Summary		Visa en bok
// @Description	Returnerar boken med kapitel, antal koncept, relationer och frågor
// @Tags			books
// @Produce		json
// @Param			bookId	path		int	true	"Bok-ID"
// @Success		200		{object}	BookDetailResponse
// @Failure		404		{object}	ErrorResponse
// @Router			/books/{bookId} [get]
func (h *Handlers) GetBook(c *gin.Context) {
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	detail, err := h.bookDetail(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// ListConcepts godoc
// @Summary		Lista koncept
// @Description	Returnerar koncepten i en bok, valfritt filtrerade på sökord
// @Tags			concepts
// @Produce		json
// @Param			bookId	path		int		true	"Bok-ID"
// @Param			q		query	string	false	"Sökfilter på namn eller definition"
// @Param			limit	query	int		false	"Max antal koncept"	default(50)
// @Success		200		{object}	ConceptListResponse
// @Failure		404		{object}	ErrorResponse
// @Router			/books/{bookId}/concepts [get]
func (h *Handlers) ListConcepts(c *gin.Context) {
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	concepts, err := h.repo.Concepts(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	query := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if query != "" {
		filtered := make([]model.Concept, 0, len(concepts))
		for _, concept := range concepts {
			haystack := strings.ToLower(concept.Name + " " + concept.Definition + " " + concept.Summary)
			if strings.Contains(haystack, query) {
				filtered = append(filtered, concept)
			}
		}
		concepts = filtered
	}
	limit := intQuery(c, "limit", 50)
	if limit > 0 && len(concepts) > limit {
		concepts = concepts[:limit]
	}
	if concepts == nil {
		concepts = []model.Concept{}
	}
	c.JSON(http.StatusOK, ConceptListResponse{Concepts: concepts})
}

// GetGraph godoc
// @Summary		Hämta kunskapsgrafen
// @Description	Returnerar noder och relationer för visualisering
// @Tags			knowledge
// @Produce		json
// @Param			bookId	path	int	true	"Bok-ID"
// @Success		200	{object}	GraphResponse
// @Failure		404	{object}	ErrorResponse
// @Router			/books/{bookId}/graph [get]
func (h *Handlers) GetGraph(c *gin.Context) {
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	concepts, err := h.repo.Concepts(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	edges, err := h.repo.Edges(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	nodes := make([]GraphNode, 0, len(concepts))
	for _, concept := range concepts {
		nodes = append(nodes, GraphNode{
			ID:         concept.ID,
			Name:       concept.Name,
			Importance: concept.Importance,
			Definition: concept.Definition,
		})
	}
	graphEdges := make([]GraphEdge, 0, len(edges))
	for _, edge := range edges {
		graphEdges = append(graphEdges, GraphEdge{
			Source:   edge.SourceID,
			Target:   edge.TargetID,
			Relation: edge.Relation,
			Weight:   edge.Weight,
		})
	}
	c.JSON(http.StatusOK, GraphResponse{Nodes: nodes, Edges: graphEdges})
}

// GetQuiz godoc
// @Summary		Hämta kunskapstest
// @Description	Returnerar frågor för boken, och genererar nya om inga finns
// @Tags			quiz
// @Produce		json
// @Param			bookId		path	int		true	"Bok-ID"
// @Param			count		query	int		false	"Antal frågor"	default(10)
// @Param			kinds		query	string	false	"Komma-separerade frågetyper"
// @Param			regenerate	query	bool	false	"Generera nya frågor"
// @Success		200			{object}	QuizResponse
// @Failure		404			{object}	ErrorResponse
// @Router			/books/{bookId}/quiz [get]
func (h *Handlers) GetQuiz(c *gin.Context) {
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	concepts, err := h.repo.Concepts(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if len(concepts) == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "boken innehåller inga koncept"})
		return
	}
	questions, err := h.repo.Questions(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	count := intQuery(c, "count", 10)
	if len(questions) == 0 || c.Query("regenerate") == "true" {
		kinds := parseKinds(c.Query("kinds"))
		engine := learning.NewQuizEngine(int64Query(c, "seed", 0))
		questions = engine.Generate(concepts, count, kinds)
		if err := h.repo.AddQuestions(questions); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return
		}
	}
	if count > 0 && len(questions) > count {
		questions = questions[:count]
	}
	if questions == nil {
		questions = []model.Question{}
	}
	c.JSON(http.StatusOK, QuizResponse{Questions: questions})
}

// GetIdeas godoc
// @Summary		Generera idéer
// @Description	Returnerar brainstorm-idéer baserade på bokens kunskapsgraf
// @Tags			ideas
// @Produce		json
// @Param			bookId	path		int		true	"Bok-ID"
// @Param			seed	query	string	false	"Ett eget tema att utgå från"
// @Param			limit	query	int		false	"Antal idéer"	default(6)
// @Success		200		{object}	IdeasResponse
// @Failure		404		{object}	ErrorResponse
// @Router			/books/{bookId}/ideas [get]
func (h *Handlers) GetIdeas(c *gin.Context) {
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	concepts, err := h.repo.Concepts(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	if len(concepts) == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "boken innehåller inga koncept"})
		return
	}
	graph, err := learn.BuildGraph(h.repo, book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	limit := intQuery(c, "limit", 6)
	result := ideas.Brainstorm(strings.TrimSpace(c.Query("seed")), concepts, graph, limit, int64Query(c, "seed_value", 0))
	if result == nil {
		result = []model.Idea{}
	}
	c.JSON(http.StatusOK, IdeasResponse{Ideas: result})
}

// ---------------------------------------------------------------------------
// Koncept
// ---------------------------------------------------------------------------

// ExplainConcept godoc
// @Summary		Förklara ett koncept
// @Description	Returnerar en nivåanpassad förklaring med nyckelpunkter, citat och följdfrågor
// @Tags			learning
// @Produce		json
// @Param			conceptId	path		int		true	"Koncept-ID"
// @Param			level		query	string	false	"novice|standard|advanced"	default(standard)
// @Param			bookId		query	int		false	"Bok-ID för grafen"
// @Success		200			{object}	learning.Explanation
// @Failure		404			{object}	ErrorResponse
// @Router			/concepts/{conceptId}/explain [get]
func (h *Handlers) ExplainConcept(c *gin.Context) {
	concept, ok := h.resolveConcept(c)
	if !ok {
		return
	}
	var graph = h.graphFor(c, concept)
	level := c.DefaultQuery("level", learning.LevelStandard)
	explanation := learning.Explain(concept, level, graph, 5)
	c.JSON(http.StatusOK, explanation)
}

// RelatedConcepts godoc
// @Summary		Relaterade koncept
// @Description	Returnerar konceptet grannar i grafen, rankade på styrka
// @Tags			knowledge
// @Produce		json
// @Param			conceptId	path		int		true	"Koncept-ID"
// @Param			bookId		query	int		false	"Bok-ID"
// @Param			limit		query	int		false	"Max antal grannar"	default(5)
// @Success		200			{object}	RelatedResponse
// @Failure		404			{object}	ErrorResponse
// @Router			/concepts/{conceptId}/related [get]
func (h *Handlers) RelatedConcepts(c *gin.Context) {
	concept, ok := h.resolveConcept(c)
	if !ok {
		return
	}
	graph := h.graphFor(c, concept)
	if graph == nil {
		c.JSON(http.StatusOK, RelatedResponse{Related: []RelatedConcept{}})
		return
	}
	limit := intQuery(c, "limit", 5)
	neighbours := graph.Related(concept, limit)
	related := make([]RelatedConcept, 0, len(neighbours))
	for _, neighbour := range neighbours {
		related = append(related, RelatedConcept{
			ID:       neighbour.Concept.ID,
			Name:     neighbour.Concept.Name,
			Relation: neighbour.Relation,
			Weight:   neighbour.Weight,
		})
	}
	c.JSON(http.StatusOK, RelatedResponse{Related: related})
}

// RelatedConcept är en granne i grafen.
type RelatedConcept struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Relation string  `json:"relation"`
	Weight   float64 `json:"weight"`
}

// RelatedResponse svarar med grannar.
type RelatedResponse struct {
	Related []RelatedConcept `json:"related"`
}

// Ask godoc
// @Summary		Ställ en fråga
// @Description	Rangerar koncepten mot frågan och svarar med definitioner och kopplingar
// @Tags			learning
// @Produce		json
// @Param			q		query	string	true	"Frågan"
// @Param			bookId	query	int		false	"Bok-ID"
// @Success		200		{object}	AnswerResponse
// @Failure		400		{object}	ErrorResponse
// @Router			/ask [get]
func (h *Handlers) Ask(c *gin.Context) {
	question := strings.TrimSpace(c.Query("q"))
	if question == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "q krävs"})
		return
	}
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	concepts, err := h.repo.Concepts(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	graph, err := learn.BuildGraph(h.repo, book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	result := learning.Answer(question, concepts, graph, nil)
	if result.Concepts == nil {
		result.Concepts = []model.Concept{}
	}
	c.JSON(http.StatusOK, AnswerResponse{Answer: result.Text, Concepts: result.Concepts})
}

// Connect godoc
// @Summary		Visa koppling mellan koncept
// @Description	Söker en väg i kunskapsgrafen mellan två koncept
// @Tags			knowledge
// @Produce		json
// @Param			from	query	string	true	"Utgångskoncept"
// @Param			to		query	string	true	"Slutkoncept"
// @Param			bookId	query	int		false	"Bok-ID"
// @Success		200		{object}	PathResponse
// @Failure		400		{object}	ErrorResponse
// @Failure		404		{object}	ErrorResponse
// @Router			/connect [get]
func (h *Handlers) Connect(c *gin.Context) {
	from := strings.TrimSpace(c.Query("from"))
	to := strings.TrimSpace(c.Query("to"))
	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "from och to krävs"})
		return
	}
	book, ok := h.resolveBook(c)
	if !ok {
		return
	}
	concepts, err := h.repo.Concepts(book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	left, leftFound := findConcept(concepts, from)
	right, rightFound := findConcept(concepts, to)
	if !leftFound || !rightFound {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "kunde inte hitta koncepten"})
		return
	}
	graph, err := learn.BuildGraph(h.repo, book.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	path := graph.FindPath(left.ID, right.ID)
	nodes := make([]PathNode, 0, len(path))
	for _, concept := range path {
		nodes = append(nodes, PathNode{ID: concept.ID, Name: concept.Name})
	}
	c.JSON(http.StatusOK, PathResponse{Path: nodes})
}

// PostAttempt godoc
// @Summary		Registrera ett svar
// @Description	Sparar svaret och uppdaterar repetitionsplanen (SM-2)
// @Tags			quiz
// @Accept			json
// @Produce		json
// @Param			questionId	path		int				true	"Fråge-ID"
// @Param			body		body		AttemptRequest	true	"Svaret"
// @Success		200			{object}	AttemptResponse
// @Failure		404			{object}	ErrorResponse
// @Router			/questions/{questionId}/attempt [post]
func (h *Handlers) PostAttempt(c *gin.Context) {
	questionID, err := strconv.ParseInt(c.Param("questionId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ogiltigt fråge-ID"})
		return
	}
	var request AttemptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ogiltig begäran"})
		return
	}
	questions, err := h.repo.Questions(0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	var question model.Question
	found := false
	for _, candidate := range questions {
		if candidate.ID == questionID {
			question = candidate
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "frågan hittades inte"})
		return
	}
	var previous *model.Attempt
	if attempt, ok := h.repo.LatestAttempt(questionID); ok && attempt != nil {
		previous = attempt
	}
	attempt := learning.Scheduler{}.Update(previous, request.Correct)
	attempt.QuestionID = questionID
	attempt.ConceptID = question.ConceptID
	if err := h.repo.AddAttempt(&attempt); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, AttemptResponse{
		Attempt: attempt,
		NextDue: attempt.DueAt.UTC().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Hjälpfunktioner
// ---------------------------------------------------------------------------

// resolveBook hittar boken från sökvägen eller query-parametern bookId.
func (h *Handlers) resolveBook(c *gin.Context) (model.Book, bool) {
	query := strings.TrimSpace(c.Param("bookId"))
	if query == "" {
		query = strings.TrimSpace(c.Query("bookId"))
	}
	if query == "" {
		query = strings.TrimSpace(c.Query("book"))
	}
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "bookId krävs"})
		return model.Book{}, false
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil {
		books, err := h.repo.ListBooks()
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			return model.Book{}, false
		}
		for _, book := range books {
			if book.ID == id {
				return book, true
			}
		}
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "boken hittades inte"})
		return model.Book{}, false
	}
	book, err := h.repo.FindBook(query)
	if err != nil || book == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "boken hittades inte"})
		return model.Book{}, false
	}
	return *book, true
}

// resolveConcept hittar konceptet från sökvägen.
func (h *Handlers) resolveConcept(c *gin.Context) (model.Concept, bool) {
	conceptID, err := strconv.ParseInt(c.Param("conceptId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ogiltigt koncept-ID"})
		return model.Concept{}, false
	}
	concepts, err := h.repo.Concepts(0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return model.Concept{}, false
	}
	for _, concept := range concepts {
		if concept.ID == conceptID {
			return concept, true
		}
	}
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "konceptet hittades inte"})
	return model.Concept{}, false
}

// graphFor bygger grafen för boken som konceptet tillhör.
func (h *Handlers) graphFor(c *gin.Context, concept model.Concept) *knowledge.Graph {
	bookID := concept.BookID
	if raw := strings.TrimSpace(c.Query("bookId")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			bookID = id
		}
	}
	graph, err := learn.BuildGraph(h.repo, bookID)
	if err != nil {
		return nil
	}
	return graph
}

// bookDetail bygger svaret för en enskild bok.
func (h *Handlers) bookDetail(bookID int64) (BookDetailResponse, error) {
	books, err := h.repo.ListBooks()
	if err != nil {
		return BookDetailResponse{}, err
	}
	var book model.Book
	for _, candidate := range books {
		if candidate.ID == bookID {
			book = candidate
			break
		}
	}
	chapters, err := h.repo.Chapters(bookID)
	if err != nil {
		return BookDetailResponse{}, err
	}
	summaries := make([]ChapterSummary, 0, len(chapters))
	for _, chapter := range chapters {
		summaries = append(summaries, ChapterSummary{
			ID:    chapter.ID,
			Index: chapter.Index,
			Title: chapter.Title,
			Words: len(strings.Fields(chapter.Text)),
		})
	}
	concepts, err := h.repo.Concepts(bookID)
	if err != nil {
		return BookDetailResponse{}, err
	}
	edges, err := h.repo.Edges(bookID)
	if err != nil {
		return BookDetailResponse{}, err
	}
	questions, err := h.repo.Questions(bookID)
	if err != nil {
		return BookDetailResponse{}, err
	}
	return BookDetailResponse{
		Book:          book,
		Chapters:      summaries,
		ConceptCount:  len(concepts),
		EdgeCount:     len(edges),
		QuestionCount: len(questions),
	}, nil
}

// findConcept söker ett koncept på namn, definition eller sammanfattning.
func findConcept(concepts []model.Concept, query string) (model.Concept, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return model.Concept{}, false
	}
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

// parseKinds tolkar en kommaseparerad lista med frågetyper.
func parseKinds(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return learning.StandardKinds
	}
	var kinds []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			kinds = append(kinds, part)
		}
	}
	if len(kinds) == 0 {
		return learning.StandardKinds
	}
	return kinds
}

// intQuery läser ett heltal ur query-parametern med standardvärde.
func intQuery(c *gin.Context, name string, fallback int) int {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

// int64Query läser ett 64-bitars heltal ur query-parametern med standardvärde.
func int64Query(c *gin.Context, name string, fallback int64) int64 {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}
