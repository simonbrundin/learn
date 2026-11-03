package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/simon/repo-template/internal/learn"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
	"github.com/simon/repo-template/internal/testutil"
)

func setup(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })

	epubPath, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := learn.ImportBook(epubPath, repo, 0); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	New(repo).Register(router)
	return router, repo
}

func request(t *testing.T, router *gin.Engine, method, target string, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	parsed := map[string]any{}
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("svaret var inte giltig JSON (%s): %s", err, recorder.Body.String())
		}
	}
	return recorder, parsed
}

func TestListBooks(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	books, ok := body["books"].([]any)
	if !ok || len(books) != 1 {
		t.Fatalf("förväntade en bok, fick: %v", body["books"])
	}
	first := books[0].(map[string]any)
	if first["title"] != "Thinking About Thinking" {
		t.Fatalf("oväntad titel: %v", first["title"])
	}
}

func TestGetBookDetail(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books/1", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	if body["concept_count"].(float64) < 3 {
		t.Fatalf("för få koncept: %v", body["concept_count"])
	}
	chapters, ok := body["chapters"].([]any)
	if !ok || len(chapters) != 2 {
		t.Fatalf("förväntade två kapitel, fick: %v", body["chapters"])
	}
}

func TestGetBookNotFound(t *testing.T) {
	router, _ := setup(t)
	recorder, _ := request(t, router, http.MethodGet, "/books/999", "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
}

func TestListConceptsWithFilter(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books/1/concepts?q=attention", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	concepts := body["concepts"].([]any)
	if len(concepts) == 0 {
		t.Fatal("filtret gav inga koncept")
	}
	for _, item := range concepts {
		concept := item.(map[string]any)
		haystack := strings.ToLower(concept["name"].(string) + " " + concept["definition"].(string) + " " + concept["summary"].(string))
		if !strings.Contains(haystack, "attention") {
			t.Fatalf("konceptet %v matchade inte filtret", concept["name"])
		}
	}
}

func TestGetGraph(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books/1/graph", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	nodes := body["nodes"].([]any)
	edges := body["edges"].([]any)
	if len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("grafen saknar noder eller kanter: %d/%d", len(nodes), len(edges))
	}
	edge := edges[0].(map[string]any)
	if edge["source"] == nil || edge["relation"] == "" {
		t.Fatalf("ofullständig kant: %v", edge)
	}
}

func TestQuizGeneratesAndStoresQuestions(t *testing.T) {
	router, repo := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books/1/quiz?count=3", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	questions := body["questions"].([]any)
	if len(questions) == 0 {
		t.Fatal("inga frågor genererades")
	}
	stored, err := repo.Questions(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) == 0 {
		t.Fatal("frågorna sparades inte i databasen")
	}
}

func TestPostAttemptUpdatesSchedule(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books/1/quiz?count=1", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	question := body["questions"].([]any)[0].(map[string]any)
	id := int64(question["id"].(float64))

	recorder, attemptBody := request(t, router, http.MethodPost, "/questions/"+itoa(id)+"/attempt", `{"correct":true,"answer":"2"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d (%s)", recorder.Code, recorder.Body.String())
	}
	attempt := attemptBody["attempt"].(map[string]any)
	if attempt["interval_days"].(float64) != 1 {
		t.Fatalf("förväntade intervall 1, fick: %v", attempt["interval_days"])
	}
	if attemptBody["next_due"] == "" {
		t.Fatal("saknar next_due")
	}
}

func TestExplainConcept(t *testing.T) {
	router, repo := setup(t)
	concepts, err := repo.Concepts(1)
	if err != nil || len(concepts) == 0 {
		t.Fatal("inga koncept i databasen")
	}
	target := concepts[0]
	recorder, body := request(t, router, http.MethodGet, "/concepts/"+itoa(target.ID)+"/explain?level=novice", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	if body["level"] != "novice" {
		t.Fatalf("oväntad nivå: %v", body["level"])
	}
	if body["title"] != target.Name {
		t.Fatalf("oväntad titel: %v", body["title"])
	}
}

func TestAskQuestion(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/ask?q=Vad%20%C3%A4r%20System%201%3F&bookId=1", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	answer := body["answer"].(string)
	if !strings.Contains(answer, "System 1") {
		t.Fatalf("svaret nämner inte System 1: %q", answer)
	}
}

func TestAskRequiresQuestion(t *testing.T) {
	router, _ := setup(t)
	recorder, _ := request(t, router, http.MethodGet, "/ask", "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
}

func TestConnectFindsPath(t *testing.T) {
	router, repo := setup(t)
	concepts, _ := repo.Concepts(1)
	var system1, system2 int64
	for _, concept := range concepts {
		switch concept.Name {
		case "System 1":
			system1 = concept.ID
		case "System 2":
			system2 = concept.ID
		}
	}
	if system1 == 0 || system2 == 0 {
		t.Fatal("koncepten System 1 och System 2 saknas")
	}
	recorder, body := request(t, router, http.MethodGet, "/connect?from=System%201&to=System%202&bookId=1", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	path := body["path"].([]any)
	if len(path) < 2 {
		t.Fatalf("för kort väg: %v", path)
	}
}

func TestIdeasEndpoint(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/books/1/ideas?limit=3", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	ideas := body["ideas"].([]any)
	if len(ideas) == 0 {
		t.Fatal("inga idéer genererades")
	}
}

func TestBookInsightEndpoint(t *testing.T) {
	router, repo := setup(t)

	// En andra bok med ett koncept som också finns i den första.
	second := model.Book{Title: "Second Book"}
	if err := repo.AddBook(&second); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddConcepts([]model.Concept{
		{BookID: second.ID, Name: "System 1", Definition: "System 1 is a different description entirely."},
	}); err != nil {
		t.Fatal(err)
	}

	recorder, body := request(t, router, http.MethodGet, "/books/1/insight", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	totals, ok := body["totals"].(map[string]any)
	if !ok {
		t.Fatalf("svaret saknar totals: %v", body)
	}
	if totals["concepts"].(float64) == 0 {
		t.Error("totals saknar antalet koncept")
	}
	if _, ok := body["novel"]; !ok {
		t.Error("svaret saknar nya koncept")
	}
	if _, ok := body["unconventional"]; !ok {
		t.Error("svaret saknar avvikelser")
	}
}

func TestLibraryInsightEndpoint(t *testing.T) {
	router, _ := setup(t)
	recorder, body := request(t, router, http.MethodGet, "/insight", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	if body["books"].(float64) != 1 {
		t.Errorf("väntade 1 bok, fick %v", body["books"])
	}
	if body["distinct_names"].(float64) == 0 {
		t.Error("saknar distinkta konceptnamn")
	}
}

func TestImportBookEndpoint(t *testing.T) {
	router, _ := setup(t)
	epubPath, err := testutil.BuildEpub(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	recorder, body := request(t, router, http.MethodPost, "/books/import", `{"path":"`+epubPath+`"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("oväntad status: %d (%s)", recorder.Code, recorder.Body.String())
	}
	if body["concept_count"].(float64) == 0 {
		t.Fatal("ingen koncept skapades")
	}
}

func TestImportBookRequiresPath(t *testing.T) {
	router, _ := setup(t)
	recorder, _ := request(t, router, http.MethodPost, "/books/import", `{}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
}

func TestRelatedConcepts(t *testing.T) {
	router, repo := setup(t)
	concepts, _ := repo.Concepts(1)
	target := concepts[0]
	recorder, body := request(t, router, http.MethodGet, "/concepts/"+itoa(target.ID)+"/related", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("oväntad status: %d", recorder.Code)
	}
	related := body["related"].([]any)
	if len(related) == 0 {
		t.Fatal("inga relaterade koncept")
	}
	first := related[0].(map[string]any)
	if first["name"] == "" || first["relation"] == "" {
		t.Fatalf("ofullständig granne: %v", first)
	}
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
