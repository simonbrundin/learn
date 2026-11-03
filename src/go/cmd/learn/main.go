// Command learn är kommandoradsgränssnittet för kunskapssystemet.
//
// Exempel:
//
//	learn import min-bok.epub
//	learn ask "Vad är System 1?"
//	learn quiz --count 10
//	learn interactive
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/simon/repo-template/internal/analysis"
	"github.com/simon/repo-template/internal/calibre"
	"github.com/simon/repo-template/internal/ideas"
	"github.com/simon/repo-template/internal/learn"
	"github.com/simon/repo-template/internal/learning"
	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
)

func main() {
	dbPath, args := parseGlobalFlags(os.Args[1:])
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	command := args[0]
	rest := args[1:]

	// DATABASE_URL väljer PostgreSQL. Utan den används SQLite, med --db
	// eller LEARN_DB om det anges.
	var repo store.Repository
	var err error
	if dbPath == "" {
		repo, err = store.OpenFromEnv()
	} else {
		repo, err = store.Open(dbPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Fel:", err)
		os.Exit(1)
	}
	defer repo.Close()
	st := repo

	var code int
	switch command {
	case "import":
		code = cmdImport(rest, st)
	case "sync":
		code = cmdSync(rest, st)
	case "compare":
		code = cmdCompare(rest, st)
	case "list":
		code = cmdList(rest, st)
	case "show":
		code = cmdShow(rest, st)
	case "ask":
		code = cmdAsk(rest, st)
	case "explain":
		code = cmdExplain(rest, st)
	case "connect":
		code = cmdConnect(rest, st)
	case "brainstorm":
		code = cmdBrainstorm(rest, st)
	case "quiz":
		code = cmdQuiz(rest, st)
	case "interactive":
		code = cmdInteractive(rest, st)
	case "help", "-h", "--help":
		usage()
		code = 0
	default:
		fmt.Printf("Okänt kommando: %s\n\n", command)
		usage()
		code = 1
	}
	os.Exit(code)
}

func usage() {
	fmt.Println(`Learn – intelligent kunskapssystem för e-pub-böcker

Kommandon:
  import <fil.epub>        Importera en e-pub-fil
  sync                    Synka böcker från Calibre (--watch kör om automatiskt)
  compare [bok]           Jämför en bok med resten av kunskapsbasen
  list                     Lista importerade böcker
  show [bok]               Visa en bok i detalj
  ask "<fråga>"            Fråga kunskapsbasen
  explain <koncept>        Förklar ett koncept (--level novice|standard|advanced)
  connect <A> <B>          Visa koppling mellan koncept
  brainstorm [fråga]       Generera idéer
  quiz [bok]               Kör ett kunskapstest (--count N)
  interactive              Starta en interaktiv session

Globala flaggor:
  --db <sökväg>            Databasen (default: ~/.learn/learn.db)`)
}

// parseFlags tolkar flaggor oavsett var de står i argumentlistan och
// returnerar de positionella argumenten.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			return positional, fs.Parse(flagArgs)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if strings.Contains(name, "=") {
				continue
			}
			lookup := fs.Lookup(name)
			if lookup == nil {
				continue
			}
			if boolFlag, ok := lookup.Value.(interface{ IsBoolFlag() bool }); !ok || !boolFlag.IsBoolFlag() {
				if i+1 < len(args) {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		default:
			positional = append(positional, arg)
		}
	}
	return positional, fs.Parse(flagArgs)
}

func parseGlobalFlags(args []string) (string, []string) {
	dbPath := ""
	var remaining []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--db" && i+1 < len(args):
			dbPath = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--db="):
			dbPath = strings.TrimPrefix(args[i], "--db=")
		default:
			remaining = append(remaining, args[i])
		}
	}
	return dbPath, remaining
}

// ---------------------------------------------------------------------------
// Kommandon
// ---------------------------------------------------------------------------

func cmdImport(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	maxConcepts := fs.Int("max-concepts", 120, "max antal koncept per bok")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	if len(positional) < 1 {
		fmt.Println("Använd: learn import <fil.epub>")
		return 1
	}
	path := positional[0]
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("Filen finns inte: %s\n", path)
		return 1
	}
	fmt.Printf("Importerar... %s\n", filepath.Base(path))
	result, err := learn.ImportBook(path, st, *maxConcepts)
	if err != nil {
		fmt.Printf("Fel: %v\n", err)
		return 1
	}
	fmt.Printf("Inläst: %s", result.Book.Title)
	if result.Book.Author != "" {
		fmt.Printf(" (%s)", result.Book.Author)
	}
	fmt.Println()
	fmt.Printf("Hittade %d koncept\n", len(result.Concepts))
	printContribution(st, result.Book)
	fmt.Printf("Sparat i %s\n", st.Path())
	return 0
}

// printContribution visar kort vad boken tillför den samlade kunskapsbasen.
func printContribution(st store.Repository, book model.Book) {
	insight, err := analysis.Compare(book.ID, st, analysis.DefaultOptions())
	if err != nil || insight == nil {
		return
	}
	if insight.Totals.OtherBooks == 0 {
		return
	}
	fmt.Printf("Tillför: %d nya koncept (%.0f%%), %d delade, %d avvikelser varav %d motsägelser\n",
		insight.Totals.Unique, insight.Totals.Coverage*100,
		insight.Totals.Shared, insight.Totals.Unconventional, insight.Totals.Contradicting)
}

// cmdSync importerar böcker från Calibre-biblioteket. Med --watch körs
// synken om med jämna mellanrum så att nya böcker tas in automatiskt.
func cmdSync(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	library := fs.String("library", "", "Calibre-biblioteket (default: $LEARN_CALIBRE_LIBRARY eller ~/calibre)")
	watch := fs.Bool("watch", false, "kör synken om med jämna mellanrum")
	interval := fs.Duration("interval", 10*time.Minute, "intervall mellan synkar vid --watch")
	maxConcepts := fs.Int("max-concepts", 120, "max antal koncept per bok")
	limit := fs.Int("limit", 0, "max antal böcker per körning (0 = alla)")
	maxSize := fs.Int("max-file-size", 150, "max filstorlek i MB (0 = ingen gräns)")
	refresh := fs.Bool("refresh", false, "importera om redan importerade böcker")
	dryRun := fs.Bool("dry-run", false, "visa vad som skulle importeras utan att skriva")
	if _, err := parseFlags(fs, args); err != nil {
		return 1
	}

	opts := learn.SyncOptions{
		MaxConcepts: *maxConcepts,
		Refresh:     *refresh,
		Limit:       *limit,
		MaxFileSize: int64(*maxSize) * 1024 * 1024,
		DryRun:      *dryRun,
	}

	run := func() error {
		lib, err := calibre.Open(*library)
		if err != nil {
			return err
		}
		defer lib.Close()

		result, err := learn.SyncCalibre(lib, st, opts)
		if err != nil {
			return err
		}
		for _, imported := range result.Imported {
			fmt.Printf("+ %s", imported.Book.Title)
			if imported.Book.Author != "" {
				fmt.Printf(" – %s", imported.Book.Author)
			}
			fmt.Printf(" (%d koncept)\n", len(imported.Concepts))
		}
		for _, updated := range result.Updated {
			fmt.Printf("~ %s uppdaterad (%d koncept)\n", updated.Book.Title, len(updated.Concepts))
		}
		for _, failure := range result.Failed {
			fmt.Fprintf(os.Stderr, "Fel: %v\n", failure)
		}
		fmt.Printf("%s\n", result.Summary())
		return nil
	}

	if !*watch {
		if err := run(); err != nil {
			fmt.Fprintln(os.Stderr, "Fel:", err)
			return 1
		}
		return 0
	}

	fmt.Printf("Synkar Calibre var %s. Avsluta med Ctrl+C.\n", *interval)
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Fel:", err)
	}
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := run(); err != nil {
			fmt.Fprintln(os.Stderr, "Fel:", err)
		}
	}
	return 0
}

// cmdCompare visar vad en bok tillför kunskapsbasen och var den avviker
// från övrig kunskap.
func cmdCompare(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	limit := fs.Int("limit", 10, "antal koncept per kategori")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	book, ok := resolveBook(st, positional[0])
	if !ok {
		return 1
	}

	insight, err := analysis.Compare(book.ID, st, analysis.Options{
		MaxNovel:  *limit,
		MaxShared: *limit,
		MaxLinks:  *limit,
	})
	if err != nil {
		fmt.Println("Fel:", err)
		return 1
	}
	if insight == nil {
		fmt.Println("Kunde inte analysera boken.")
		return 1
	}

	fmt.Println(insight.Book.Title)
	if insight.Book.Author != "" {
		fmt.Printf("– %s\n", insight.Book.Author)
	}
	fmt.Printf("%d koncept: %d nya (%.0f%%), %d som också finns i andra böcker\n",
		insight.Totals.Concepts, insight.Totals.Unique,
		insight.Totals.Coverage*100, insight.Totals.Shared)
	if insight.Totals.Unconventional > 0 {
		fmt.Printf("%d avvikelser, varav %d motsägelser\n",
			insight.Totals.Unconventional, insight.Totals.Contradicting)
	}
	if insight.Totals.NovelKeywords > 0 {
		fmt.Printf("%d nyckelord som bara förekommer i boken\n", insight.Totals.NovelKeywords)
	}

	if len(insight.Novel) > 0 {
		fmt.Println()
		fmt.Println("Ny kunskap:")
		for _, novel := range insight.Novel {
			fmt.Printf("  %-32s vikt %.2f  %s\n", novel.Name, novel.Importance, snippet(novel.Definition))
		}
	}

	if len(insight.Unconventional) > 0 {
		fmt.Println()
		fmt.Println("Okonventionellt jämfört med övrig kunskap:")
		for _, item := range insight.Unconventional {
			fmt.Printf("  %s – %s\n", item.Name, item.Note)
			fmt.Printf("    här: %s\n", snippet(item.Here))
			if len(item.Elsewhere) > 0 {
				fmt.Printf("    i %s: %s\n", item.Elsewhere[0].BookTitle, snippet(item.Elsewhere[0].Definition))
			}
		}
	}

	if len(insight.RelationConflicts) > 0 {
		fmt.Println()
		fmt.Println("Relationer som betyder olika i olika böcker:")
		for _, conflict := range insight.RelationConflicts {
			fmt.Printf("  %s → %s: %s här, %s i %s\n", conflict.Source, conflict.Target,
				conflict.HereRelation, conflict.OtherRelation, conflict.BookTitle)
		}
	}

	if len(insight.CrossLinks) > 0 {
		fmt.Println()
		fmt.Println("Kopplingar till andra böcker:")
		for _, link := range insight.CrossLinks {
			fmt.Printf("  %-22s → %-22s i %s (%s)\n", link.Concept, link.OtherConcept,
				link.BookTitle, strings.Join(link.Keywords, ", "))
		}
	}
	return 0
}

// snippet kortar en text till en rad.
func snippet(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= 72 {
		return value
	}
	return string(runes[:72]) + "…"
}

func cmdList(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	if _, err := parseFlags(fs, args); err != nil {
		return 1
	}
	books, err := st.ListBooks()
	if err != nil {
		fmt.Println("Fel:", err)
		return 1
	}
	if len(books) == 0 {
		fmt.Println("Inga böcker importerade ännu. Kör: learn import <fil.epub>")
		return 0
	}
	for _, book := range books {
		concepts, _ := st.Concepts(book.ID)
		fmt.Printf("[%d] %s", book.ID, book.Title)
		if book.Author != "" {
			fmt.Printf(" – %s", book.Author)
		}
		fmt.Println()
		fmt.Printf("    %d koncept, språk: %s\n", len(concepts), book.Language)
	}
	return 0
}

func cmdShow(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	limit := fs.Int("limit", 15, "antal koncept att visa")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	book, ok := resolveBook(st, positional[0])
	if !ok {
		return 1
	}
	fmt.Println(book.Title)
	if book.Author != "" {
		fmt.Printf("– %s\n", book.Author)
	}
	if book.Description != "" {
		fmt.Println(book.Description)
	}
	fmt.Println()

	chapters, _ := st.Chapters(book.ID)
	fmt.Printf("Kapitel (%d):\n", len(chapters))
	for _, chapter := range chapters {
		words := len(strings.Fields(chapter.Text))
		fmt.Printf("  %3d. %s (%d ord)\n", chapter.Index+1, chapter.Title, words)
	}

	concepts, _ := st.Concepts(book.ID)
	sort.SliceStable(concepts, func(i, j int) bool {
		return concepts[i].Importance > concepts[j].Importance
	})
	fmt.Println()
	if len(concepts) > *limit {
		concepts = concepts[:*limit]
	}
	fmt.Printf("Toppkoncept (%d):\n", len(concepts))
	for _, concept := range concepts {
		detail := concept.Definition
		if detail == "" {
			detail = concept.Summary
		}
		if detail == "" {
			detail = "(ingen definition hittad)"
		}
		fmt.Printf("  - %s: %s\n", concept.Name, truncate(detail, 160))
	}
	return 0
}

func cmdAsk(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("ask", flag.ExitOnError)
	bookFlag := fs.String("book", "", "bok-id eller titel")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	question := strings.Join(positional, " ")
	if question == "" {
		fmt.Println("Använd: learn ask \"<fråga>\"")
		return 1
	}
	book, ok := resolveBook(st, *bookFlag)
	if !ok {
		return 1
	}
	concepts, _ := st.Concepts(book.ID)
	if len(concepts) == 0 {
		fmt.Println("Inget att söka i ännu. Importera en bok först.")
		return 1
	}
	graph, _ := learn.BuildGraph(st, book.ID)
	answer := learning.Answer(question, concepts, graph, nil)
	fmt.Println(answer.Text)
	return 0
}

func cmdExplain(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	level := fs.String("level", learning.LevelStandard, "novice|standard|advanced")
	bookFlag := fs.String("book", "", "bok-id eller titel")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	if len(positional) < 1 {
		fmt.Println("Använd: learn explain <koncept>")
		return 1
	}
	book, ok := resolveBook(st, *bookFlag)
	if !ok {
		return 1
	}
	concepts, _ := st.Concepts(book.ID)
	concept, found := findConcept(concepts, strings.Join(positional, " "))
	if !found {
		fmt.Printf("Kunde inte hitta konceptet: %s\n", positional[0])
		return 1
	}
	graph, _ := learn.BuildGraph(st, book.ID)
	explanation := learning.Explain(concept, *level, graph, 4)
	fmt.Println(explanation.Text())
	return 0
}

func cmdConnect(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	bookFlag := fs.String("book", "", "bok-id eller titel")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	if len(positional) < 2 {
		fmt.Println("Använd: learn connect <A> <B>")
		return 1
	}
	book, ok := resolveBook(st, *bookFlag)
	if !ok {
		return 1
	}
	concepts, _ := st.Concepts(book.ID)
	left, right, leftFound, rightFound := findConceptPair(concepts, positional)
	if !leftFound || !rightFound {
		fmt.Println("Kunde inte hitta båda koncepten.")
		return 1
	}
	graph, _ := learn.BuildGraph(st, book.ID)
	path := graph.FindPath(left.ID, right.ID)
	if len(path) == 0 {
		fmt.Printf("Ingen koppling hittad mellan %s och %s.\n", left.Name, right.Name)
		return 0
	}
	names := make([]string, 0, len(path))
	for _, concept := range path {
		names = append(names, concept.Name)
	}
	fmt.Println(strings.Join(names, " → "))
	return 0
}

func cmdBrainstorm(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("brainstorm", flag.ExitOnError)
	bookFlag := fs.String("book", "", "bok-id eller titel")
	limit := fs.Int("limit", 6, "antal idéer")
	seedValue := fs.Int64("seed-value", 0, "seed för slumpat urval")
	positional, err := parseFlags(fs, args)
	if err != nil {
		return 1
	}
	book, ok := resolveBook(st, *bookFlag)
	if !ok {
		return 1
	}
	concepts, _ := st.Concepts(book.ID)
	if len(concepts) == 0 {
		fmt.Println("Inget att brainstorma om ännu. Importera en bok först.")
		return 1
	}
	graph, _ := learn.BuildGraph(st, book.ID)
	seed := ""
	if len(positional) > 0 {
		seed = strings.Join(positional, " ")
	}
	result := ideas.Brainstorm(seed, concepts, graph, *limit, *seedValue)
	if len(result) == 0 {
		fmt.Println("Kunde inte hitta några idémönster.")
		return 0
	}
	fmt.Printf("Idéer (%d):\n", len(result))
	for index, idea := range result {
		fmt.Printf("  %d. %s\n", index+1, idea.Prompt)
		if len(idea.Basis) > 0 {
			fmt.Printf("     (utgår från: %s)\n", strings.Join(idea.Basis, ", "))
		}
	}
	return 0
}

func cmdQuiz(args []string, st store.Repository) int {
	fs := flag.NewFlagSet("quiz", flag.ExitOnError)
	count := fs.Int("count", 10, "antal frågor")
	regenerate := fs.Bool("regenerate", false, "generera nya frågor")
	seedValue := fs.Int64("seed-value", 0, "seed för slumpat urval")
	bookFlag := fs.String("book", "", "bok-id eller titel")
	if _, err := parseFlags(fs, args); err != nil {
		return 1
	}
	book, ok := resolveBook(st, *bookFlag)
	if !ok {
		return 1
	}
	concepts, _ := st.Concepts(book.ID)
	if len(concepts) == 0 {
		fmt.Println("Boken innehåller inga extraherade koncept.")
		return 1
	}

	questions, _ := st.Questions(book.ID)
	if len(questions) == 0 || *regenerate {
		engine := learning.NewQuizEngine(*seedValue)
		questions = engine.Generate(concepts, max(*count, 10), learning.StandardKinds)
		if err := st.AddQuestions(questions); err != nil {
			fmt.Println("Fel:", err)
			return 1
		}
	}

	attempts := map[int64]model.Attempt{}
	for _, question := range questions {
		if attempt, ok := st.LatestAttempt(question.ID); ok && attempt != nil {
			attempts[question.ID] = *attempt
		}
	}
	candidates := learning.DueQuestions(questions, attempts, nowFunc())
	if len(candidates) == 0 {
		fmt.Println("Alla frågor är färska – inget att repetera just nu.")
		return 0
	}
	if len(candidates) > *count {
		candidates = candidates[:*count]
	}

	fmt.Printf("Kunskapstest: %s\n\n", book.Title)
	correctCount, answeredCount := 0, 0
	for index, question := range candidates {
		response, ok := askQuestion(question, index+1)
		if !ok {
			fmt.Println("Avbruten.")
			break
		}
		correct := grade(question, response)
		answeredCount++
		if correct {
			correctCount++
		}
		recordAttempt(st, question, correct)
		fmt.Println()
	}
	if answeredCount > 0 {
		percent := 100 * correctCount / answeredCount
		fmt.Printf("Resultat: %d/%d rätt (%d%%)\n", correctCount, answeredCount, percent)
	} else {
		fmt.Println("Inga frågor besvarades.")
	}
	return 0
}

func cmdInteractive(args []string, st store.Repository) int {
	fmt.Println("Interaktiv session. Skriv 'hjälp' för kommandon, 'quit' för att avsluta.")
	reader := bufio.NewReader(os.Stdin)
	var context *model.Concept

	for {
		fmt.Print("\nlearn> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("")
			return 0
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		command := fields[0]
		rest := strings.TrimSpace(strings.TrimPrefix(line, command))

		switch command {
		case "quit", "exit", "q":
			return 0
		case "hjälp", "help", "?":
			fmt.Println(helpText)
		case "list":
			cmdList(nil, st)
		case "ask":
			concepts, _ := st.Concepts(0)
			graph, _ := learn.BuildGraph(st, 0)
			answer := learning.Answer(rest, concepts, graph, context)
			fmt.Println(answer.Text)
			if len(answer.Concepts) > 0 {
				matched := answer.Concepts[0]
				context = &matched
			}
		case "explain":
			concepts, _ := st.Concepts(0)
			concept, found := findConcept(concepts, rest)
			if !found {
				fmt.Println("Kunde inte hitta konceptet.")
				continue
			}
			context = &concept
			graph, _ := learn.BuildGraph(st, 0)
			fmt.Println(learning.Explain(concept, learning.LevelStandard, graph, 4).Text())
		case "connect":
			parts := splitArgs(rest)
			if len(parts) != 2 {
				fmt.Println(`Använd: connect "koncept A" "koncept B"`)
				continue
			}
			concepts, _ := st.Concepts(0)
			left, leftFound := findConcept(concepts, parts[0])
			right, rightFound := findConcept(concepts, parts[1])
			if !leftFound || !rightFound {
				fmt.Println("Kunde inte hitta koncepten.")
				continue
			}
			graph, _ := learn.BuildGraph(st, 0)
			path := graph.FindPath(left.ID, right.ID)
			if len(path) == 0 {
				fmt.Println("Ingen koppling hittad.")
				continue
			}
			names := make([]string, 0, len(path))
			for _, concept := range path {
				names = append(names, concept.Name)
			}
			fmt.Println(strings.Join(names, " → "))
		case "brainstorm":
			concepts, _ := st.Concepts(0)
			graph, _ := learn.BuildGraph(st, 0)
			for index, idea := range ideas.Brainstorm(rest, concepts, graph, 5, 0) {
				fmt.Printf("  %d. %s\n", index+1, idea.Prompt)
			}
		case "quiz":
			cmdQuiz([]string{"--count", "5"}, st)
		default:
			fmt.Printf("Okänt kommando: %s. Skriv 'hjälp'.\n", command)
		}
	}
}

const helpText = `Kommandon:
  ask <fråga>              Fråga kunskapsbasen
  explain <koncept>        Förklara ett koncept
  connect <A> <B>          Visa koppling mellan koncept
  brainstorm [fråga]        Generera idéer
  quiz                     Kör ett kunskapstest
  list                     Lista böcker
  quit                     Avsluta`

// ---------------------------------------------------------------------------
// Hjälpfunktioner
// ---------------------------------------------------------------------------

func resolveBook(st store.Repository, query string) (model.Book, bool) {
	books, err := st.ListBooks()
	if err != nil {
		fmt.Println("Fel:", err)
		return model.Book{}, false
	}
	if len(books) == 0 {
		fmt.Println("Inga böcker importerade ännu. Kör: learn import <fil.epub>")
		return model.Book{}, false
	}
	if strings.TrimSpace(query) == "" {
		return books[0], true
	}
	book, err := st.FindBook(query)
	if err != nil || book == nil {
		fmt.Printf("Kunde inte hitta boken: %s\n", query)
		return model.Book{}, false
	}
	return *book, true
}

// findConceptPair tolkar argumenten som två konceptnamn, även när de
// består av flera ord ("System 1 System 2").
func findConceptPair(concepts []model.Concept, parts []string) (model.Concept, model.Concept, bool, bool) {
	for split := 1; split < len(parts); split++ {
		left, leftFound := findConcept(concepts, strings.Join(parts[:split], " "))
		right, rightFound := findConcept(concepts, strings.Join(parts[split:], " "))
		if leftFound && rightFound {
			return left, right, true, true
		}
	}
	return model.Concept{}, model.Concept{}, false, false
}

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
		body := strings.ToLower(concept.Definition + " " + concept.Summary)
		if strings.Contains(body, needle) {
			return concept, true
		}
	}
	return model.Concept{}, false
}

func askQuestion(question model.Question, index int) (string, bool) {
	fmt.Printf("%d. %s\n", index, question.Prompt)
	reader := bufio.NewReader(os.Stdin)
	if len(question.Options) > 0 {
		for optionIndex, option := range question.Options {
			fmt.Printf("   %d) %s\n", optionIndex+1, option)
		}
		fmt.Printf("   Ditt svar (1-%d, eller skriv 'skip'): ", len(question.Options))
		raw, _ := reader.ReadString('\n')
		raw = strings.TrimSpace(raw)
		if strings.EqualFold(raw, "skip") {
			return "", false
		}
		if number, err := strconv.Atoi(raw); err == nil && number >= 1 && number <= len(question.Options) {
			return question.Options[number-1], true
		}
		return raw, true
	}
	fmt.Print("   Ditt svar: ")
	raw, _ := reader.ReadString('\n')
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "skip") {
		return "", false
	}
	return raw, true
}

func grade(question model.Question, response string) bool {
	switch question.Kind {
	case learning.KindEssay:
		fmt.Println("   (Essay – bedöms manuellt, räknas som godkänt)")
		return true
	case learning.KindShortAnswer:
		expected := map[string]bool{}
		for _, word := range strings.Fields(strings.ToLower(question.Answer)) {
			if len(word) > 3 {
				expected[word] = true
			}
		}
		given := map[string]bool{}
		for _, word := range strings.Fields(strings.ToLower(response)) {
			if len(word) > 3 {
				given[word] = true
			}
		}
		overlap := 0
		for word := range expected {
			if given[word] {
				overlap++
			}
		}
		correct := len(expected) > 0 && float64(overlap)/float64(len(expected)) >= 0.4
		if correct {
			fmt.Println("   Rätt!")
		} else {
			fmt.Printf("   Svar: %s\n", question.Answer)
		}
		return correct
	default:
		correct := strings.EqualFold(strings.TrimSpace(response), strings.TrimSpace(question.Answer))
		if correct {
			fmt.Println("   Rätt!")
		} else {
			fmt.Printf("   Rätt svar: %s\n", question.Answer)
			if question.Explanation != "" {
				fmt.Printf("   %s\n", truncate(question.Explanation, 300))
			}
		}
		return correct
	}
}

func recordAttempt(st store.Repository, question model.Question, correct bool) {
	var previous *model.Attempt
	if attempt, ok := st.LatestAttempt(question.ID); ok && attempt != nil {
		previous = attempt
	}
	attempt := learning.Scheduler{}.Update(previous, correct)
	attempt.QuestionID = question.ID
	attempt.ConceptID = question.ConceptID
	_ = st.AddAttempt(&attempt)
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func splitArgs(value string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	for _, r := range value {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case r == ' ' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// nowFunc gör att tiden kan stubbas i tester.
var nowFunc = func() time.Time { return time.Now() }
