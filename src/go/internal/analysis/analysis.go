// Package analysis jämför en bok med resten av kunskapsbasen.
//
// Analysen svarar på två frågor: vad tillför boken som inte fanns
// tidigare, och var beskriver boken något på ett sätt som skiljer sig
// från hur samma sak beskrivs i andra böcker.
package analysis

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/simon/repo-template/internal/model"
	"github.com/simon/repo-template/internal/store"
	"github.com/simon/repo-template/internal/text"
)

// Gränser för bedömningen av likhet mellan definitioner.
const (
	// ConsistentSimilarity är över vilken likhet två beskrivningar
	// räknas som samma uppfattning.
	ConsistentSimilarity = 0.7
	// NuanceSimilarity är under vilken likhet beskrivningarna räknas
	// som avvikande.
	NuanceSimilarity = 0.35
)

// Options styr hur mycket analysen sammanfattar.
type Options struct {
	// MaxNovel begränsar antalet nya koncept som listas.
	MaxNovel int
	// MaxShared begränsar antalet delade koncept som listas.
	MaxShared int
	// MaxLinks begränsar antalet korskopplingar.
	MaxLinks int
}

// DefaultOptions ger balanserade gränser för ett bibliotek med många böcker.
func DefaultOptions() Options {
	return Options{MaxNovel: 25, MaxShared: 25, MaxLinks: 30}
}

// Compare jämför boken med resten av kunskapsbasen.
func Compare(bookID int64, repo store.Repository, opts Options) (*model.BookInsight, error) {
	books, err := repo.ListBooks()
	if err != nil {
		return nil, err
	}
	concepts, err := repo.AllConcepts()
	if err != nil {
		return nil, err
	}
	edges, err := repo.AllEdges()
	if err != nil {
		return nil, err
	}

	var target model.Book
	found := false
	titles := map[int64]string{}
	for _, book := range books {
		titles[book.ID] = book.Title
		if book.ID == bookID {
			target = book
			found = true
		}
	}
	if !found {
		return nil, nil
	}

	// Index över konceptnamn, nyckelord och böcker.
	byName := map[string][]model.Concept{}
	byKeyword := map[string][]model.Concept{}
	keywordBooks := map[string]map[int64]bool{}
	var own []model.Concept
	for _, concept := range concepts {
		if isGenericConcept(concept) {
			// Ord som "Tyvärr" är aldrig kunskap.
			continue
		}
		key := normalizeName(concept.Name)
		byName[key] = append(byName[key], concept)
		for _, keyword := range concept.Keywords {
			normalized := strings.ToLower(strings.TrimSpace(keyword))
			if normalized == "" {
				continue
			}
			byKeyword[normalized] = append(byKeyword[normalized], concept)
			if keywordBooks[normalized] == nil {
				keywordBooks[normalized] = map[int64]bool{}
			}
			keywordBooks[normalized][concept.BookID] = true
		}
		if concept.BookID == bookID {
			own = append(own, concept)
		}
	}

	insight := &model.BookInsight{Book: target}
	insight.Novel = []model.NovelConcept{}
	insight.Shared = []model.SharedConcept{}
	insight.Unconventional = []model.UnconventionalConcept{}
	insight.RelationConflicts = []model.RelationConflict{}
	insight.CrossLinks = []model.CrossLink{}
	insight.Totals.Concepts = len(own)
	insight.Totals.OtherBooks = len(books) - 1

	novelKeywords := map[string]bool{}
	for _, concept := range own {
		for _, keyword := range concept.Keywords {
			normalized := strings.ToLower(strings.TrimSpace(keyword))
			if normalized == "" {
				continue
			}
			// Räkna böcker som har nyckelordet, bortsett från boken
			// själv. Kartan kopieras så att indexet inte skadas.
			others := 0
			for otherBook := range keywordBooks[normalized] {
				if otherBook != bookID {
					others++
				}
			}
			if others == 0 {
				novelKeywords[normalized] = true
			}
		}
	}
	insight.Totals.NovelKeywords = len(novelKeywords)

	for _, concept := range own {
		matches := byName[normalizeName(concept.Name)]
		versions := make([]model.SharedVersion, 0, len(matches))
		for _, match := range matches {
			if match.BookID == bookID {
				continue
			}
			similarity := text.Similarity(description(concept), description(match))
			version := model.SharedVersion{
				BookID:     match.BookID,
				BookTitle:  titles[match.BookID],
				Definition: description(match),
				Similarity: round(similarity, 3),
				Verdict:    verdict(similarity),
				Negated:    contradicts(concept, match),
			}
			if version.Negated {
				version.Verdict = "contradiction"
			}
			versions = append(versions, version)
		}

		if len(versions) == 0 {
			insight.Novel = append(insight.Novel, model.NovelConcept{
				Name:          concept.Name,
				Definition:    concept.Definition,
				Summary:       concept.Summary,
				Importance:    concept.Importance,
				Mentions:      concept.Mentions,
				Keywords:      stringsOf(concept.Keywords),
				NovelKeywords: novelFor(concept, novelKeywords),
			})
			continue
		}

		sort.Slice(versions, func(i, j int) bool {
			return versions[i].Similarity < versions[j].Similarity
		})
		shared := model.SharedConcept{
			Name:          concept.Name,
			Importance:    concept.Importance,
			Here:          description(concept),
			Elsewhere:     versions,
			Verdict:       versions[0].Verdict,
			NovelKeywords: novelFor(concept, novelKeywords),
		}
		insight.Shared = append(insight.Shared, shared)

		if shared.Verdict == "unconventional" || shared.Verdict == "contradiction" {
			insight.Unconventional = append(insight.Unconventional, model.UnconventionalConcept{
				SharedConcept: shared,
				Note:          note(shared),
			})
		}
	}

	insight.Totals.Unique = len(insight.Novel)
	insight.Totals.Shared = len(insight.Shared)
	insight.Totals.Unconventional = len(insight.Unconventional)
	for _, item := range insight.Unconventional {
		if item.Verdict == "contradiction" {
			insight.Totals.Contradicting++
		}
	}
	if insight.Totals.Concepts > 0 {
		insight.Totals.Coverage = round(float64(insight.Totals.Unique)/float64(insight.Totals.Concepts), 3)
	}

	insight.RelationConflicts = relationConflicts(edges, own, titles, bookID)
	insight.CrossLinks = crossLinks(own, byKeyword, keywordBooks, titles, bookID, opts.MaxLinks)

	sortNovel(insight.Novel)
	sortShared(insight.Shared)
	sortUnconventional(insight.Unconventional)

	if opts.MaxNovel > 0 && len(insight.Novel) > opts.MaxNovel {
		insight.Novel = insight.Novel[:opts.MaxNovel]
	}
	if opts.MaxShared > 0 && len(insight.Shared) > opts.MaxShared {
		insight.Shared = insight.Shared[:opts.MaxShared]
	}
	if opts.MaxLinks > 0 && len(insight.CrossLinks) > opts.MaxLinks {
		insight.CrossLinks = insight.CrossLinks[:opts.MaxLinks]
	}
	return insight, nil
}

// Library sammanfattar kunskapsbasen som helhet.
func Library(repo store.Repository) (*model.LibraryInsight, error) {
	books, err := repo.ListBooks()
	if err != nil {
		return nil, err
	}
	concepts, err := repo.AllConcepts()
	if err != nil {
		return nil, err
	}

	byName := map[string][]model.Concept{}
	for _, concept := range concepts {
		if isGenericConcept(concept) {
			continue
		}
		key := normalizeName(concept.Name)
		byName[key] = append(byName[key], concept)
	}

	titles := map[int64]string{}
	for _, book := range books {
		titles[book.ID] = book.Title
	}

	insight := &model.LibraryInsight{
		Books:              len(books),
		Concepts:           len(concepts),
		DistinctNames:      len(byName),
		MostShared:         []model.SharedConcept{},
		MostUnconventional: []model.UnconventionalConcept{},
	}
	for _, group := range byName {
		if len(group) < 2 {
			continue
		}
		insight.SharedNames++
		shared := model.SharedConcept{Name: group[0].Name}
		worst := 1.0
		for i := range group {
			for j := i + 1; j < len(group); j++ {
				if group[i].BookID == group[j].BookID {
					continue
				}
				similarity := text.Similarity(description(group[i]), description(group[j]))
				version := model.SharedVersion{
					BookID:     group[j].BookID,
					BookTitle:  titles[group[j].BookID],
					Definition: description(group[j]),
					Similarity: round(similarity, 3),
					Verdict:    verdict(similarity),
					Negated:    contradicts(group[i], group[j]),
				}
				if version.Negated {
					version.Verdict = "contradiction"
				}
				shared.Elsewhere = append(shared.Elsewhere, version)
				if similarity < worst {
					worst = similarity
				}
			}
		}
		shared.Verdict = verdict(worst)
		insight.MostShared = append(insight.MostShared, shared)
		if shared.Verdict == "unconventional" || shared.Verdict == "contradiction" {
			insight.MostUnconventional = append(insight.MostUnconventional, model.UnconventionalConcept{
				SharedConcept: shared,
				Note:          note(shared),
			})
		}
	}
	if insight.DistinctNames > 0 {
		insight.CorpusCoverage = round(float64(insight.SharedNames)/float64(insight.DistinctNames), 3)
	}
	sort.Slice(insight.MostShared, func(i, j int) bool {
		return len(insight.MostShared[i].Elsewhere) > len(insight.MostShared[j].Elsewhere)
	})
	sort.Slice(insight.MostUnconventional, func(i, j int) bool {
		return insight.MostUnconventional[i].Verdict == "contradiction"
	})
	if len(insight.MostShared) > 20 {
		insight.MostShared = insight.MostShared[:20]
	}
	if len(insight.MostUnconventional) > 20 {
		insight.MostUnconventional = insight.MostUnconventional[:20]
	}
	return insight, nil
}

// relationConflicts hittar par av koncept som har olika relation i
// olika böcker, exempelvis "orsak" här och "motsats" där.
func relationConflicts(edges []model.Edge, own []model.Concept, titles map[int64]string, bookID int64) []model.RelationConflict {
	ownIDs := map[int64]model.Concept{}
	for _, concept := range own {
		ownIDs[concept.ID] = concept
	}
	// Relationen för varje konceptpar, per bok.
	pairs := map[string]map[int64]string{}
	for _, edge := range edges {
		source, ok := ownIDs[edge.SourceID]
		if !ok {
			continue
		}
		target, ok := ownIDs[edge.TargetID]
		if !ok {
			continue
		}
		key := pairKey(source.Name, target.Name)
		if pairs[key] == nil {
			pairs[key] = map[int64]string{}
		}
		pairs[key][edge.BookID] = edge.Relation
	}

	var conflicts []model.RelationConflict
	for key, perBook := range pairs {
		here, ok := perBook[bookID]
		if !ok {
			continue
		}
		for otherBook, relation := range perBook {
			if otherBook == bookID || relation == here {
				continue
			}
			parts := strings.SplitN(key, "|", 2)
			conflicts = append(conflicts, model.RelationConflict{
				Source:        parts[0],
				Target:        parts[1],
				HereRelation:  here,
				OtherRelation: relation,
				BookID:        otherBook,
				BookTitle:     titles[otherBook],
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Source+conflicts[i].Target < conflicts[j].Source+conflicts[j].Target
	})
	return conflicts
}

// maxSharedKeywordBooks är hur många böcker ett nyckelord får förekomma i
// och ändå räknas som specifikt för en korskoppling.
const maxSharedKeywordBooks = 3

// crossLinks hittar koncept i andra böcker som handlar om samma sak,
// utifrån delade nyckelord.
func crossLinks(
	own []model.Concept,
	byKeyword map[string][]model.Concept,
	keywordBooks map[string]map[int64]bool,
	titles map[int64]string,
	bookID int64,
	limit int,
) []model.CrossLink {
	best := map[string]model.CrossLink{}
	for _, concept := range own {
		counts := map[int64]int{}
		keywords := map[int64]map[string]bool{}
		// Antal delade nyckelord per kandidatkoncept.
		shared := map[int64]int{}
		// Bästa matchande koncept i varje annan bok.
		matches := map[int64]model.Concept{}
		matchesCount := map[int64]int{}
		for _, keyword := range concept.Keywords {
			normalized := strings.ToLower(strings.TrimSpace(keyword))
			if normalized == "" || text.GenericWords[normalized] {
				// Generiska ord som "file" eller "part" binder inget ihop.
				continue
			}
			if books, ok := keywordBooks[normalized]; ok && len(books) > maxSharedKeywordBooks {
				// Nyckelord som står i många böcker är allmänna, inte specifika.
				continue
			}
			for _, candidate := range byKeyword[normalized] {
				if candidate.BookID == bookID {
					continue
				}
				counts[candidate.BookID]++
				if keywords[candidate.BookID] == nil {
					keywords[candidate.BookID] = map[string]bool{}
				}
				keywords[candidate.BookID][normalized] = true
				// Räkna delade nyckelord per kandidat för att hitta bästa match.
				shared[candidate.ID]++
				if shared[candidate.ID] > matchesCount[candidate.BookID] {
					matchesCount[candidate.BookID] = shared[candidate.ID]
					matches[candidate.BookID] = candidate
				}
			}
		}
		for otherBook, count := range counts {
			if count < 2 || len(keywords[otherBook]) < 2 {
				// Minst två delade nyckelord krävs för en korskoppling.
				continue
			}
			key := strconv.FormatInt(concept.ID, 10) + ":" + strconv.FormatInt(otherBook, 10)
			strength := float64(count) / float64(len(concept.Keywords)+1)
			if strength > 1 {
				strength = 1
			}
			otherName := ""
			if match, ok := matches[otherBook]; ok {
				otherName = match.Name
			}
			link := model.CrossLink{
				Concept:      concept.Name,
				OtherConcept: otherName,
				BookID:       otherBook,
				BookTitle:    titles[otherBook],
				Keywords:     sortedKeys(keywords[otherBook]),
				Strength:     round(strength, 3),
			}
			existing, ok := best[key]
			if !ok || link.Strength > existing.Strength {
				best[key] = link
			}
		}
	}

	links := make([]model.CrossLink, 0, len(best))
	for _, link := range best {
		links = append(links, link)
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].Strength != links[j].Strength {
			return links[i].Strength > links[j].Strength
		}
		return links[i].Concept < links[j].Concept
	})
	if limit > 0 && len(links) > limit {
		links = links[:limit]
	}
	return links
}

// isGenericConcept svarar på om ett koncept bara är ett diskursord.
func isGenericConcept(concept model.Concept) bool {
	name := strings.ToLower(strings.TrimSpace(concept.Name))
	if name == "" {
		return true
	}
	return text.GenericWords[name]
}

// description ger det bästa beskrivande texten för ett koncept.
// Texten kortas ned så att långa utdrag inte jämförs, och kodutdrag
// undviks när det finns en förklaring i klartext.
func description(concept model.Concept) string {
	definition := strings.TrimSpace(concept.Definition)
	summary := strings.TrimSpace(concept.Summary)
	if definition != "" && text.LooksLikeProse(definition) {
		return shorten(definition, 400)
	}
	if summary != "" && text.LooksLikeProse(summary) {
		return shorten(summary, 400)
	}
	if definition != "" {
		return shorten(definition, 400)
	}
	return shorten(summary, 400)
}

// shorten kortar en text till given längd.
func shorten(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

// verdict klassar en likhet mellan två beskrivningar.
func verdict(similarity float64) string {
	switch {
	case similarity >= ConsistentSimilarity:
		return "consistent"
	case similarity >= NuanceSimilarity:
		return "nuance"
	default:
		return "unconventional"
	}
}

// negationMarkers är ord som visar att en beskrivning säger motsatsen.
var negationMarkers = []string{
	" is not ", " are not ", " isn't ", " aren't ", " never ",
	" cannot ", " can't ", " won't ", " doesn't ", " don't ",
	" unlike ", " contrary to ",
	" är inte ", " är ej ", " inte ", " aldrig ", " kan inte ",
}

// contradicts svarar på om den ena beskrivningen säger motsatsen mot
// den andra.
func contradicts(left, right model.Concept) bool {
	return hasNegation(description(left)) != hasNegation(description(right))
}

// hasNegation svarar på om texten innehåller ett negationstecken.
func hasNegation(value string) bool {
	lowered := " " + strings.ToLower(value) + " "
	for _, marker := range negationMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// note formulerar en kort förklaring av en avvikelse.
func note(shared model.SharedConcept) string {
	if len(shared.Elsewhere) == 0 {
		return ""
	}
	other := shared.Elsewhere[0]
	if shared.Verdict == "contradiction" {
		return "Boken säger motsatsen mot " + other.BookTitle
	}
	return "Beskrivningen skiljer sig från " + other.BookTitle +
		" (" + fmt.Sprintf("%.0f%%", other.Similarity*100) + " likhet)"
}

// novelFor ger de nyckelord som bara finns i boken.
func novelFor(concept model.Concept, novel map[string]bool) []string {
	result := []string{}
	for _, keyword := range concept.Keywords {
		if novel[strings.ToLower(strings.TrimSpace(keyword))] {
			result = append(result, keyword)
		}
	}
	return result
}

// stringsOf ger en lista som aldrig är nil, så att JSON innehåller [] och
// inte null.
func stringsOf(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// normalizeName gör konceptnamn jämförbara.
func normalizeName(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	replacer := strings.NewReplacer("’", "'", "–", "-", "—", "-")
	lowered = replacer.Replace(lowered)
	var builder strings.Builder
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '\'' {
			builder.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			builder.WriteRune(r + 32)
		} else {
			builder.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// pairKey ger en nyckel för ett konceptpar, oberoende av ordning.
func pairKey(left, right string) string {
	a := normalizeName(left)
	b := normalizeName(right)
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// sortedKeys ger nycklar i sorterad ordning.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortNovel sorterar nya koncept efter vikt.
func sortNovel(items []model.NovelConcept) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Importance != items[j].Importance {
			return items[i].Importance > items[j].Importance
		}
		return items[i].Name < items[j].Name
	})
}

// sortShared sorterar delade koncept efter hur avgörande avvikelsen är.
func sortShared(items []model.SharedConcept) {
	sort.Slice(items, func(i, j int) bool {
		if severity(items[i].Verdict) != severity(items[j].Verdict) {
			return severity(items[i].Verdict) > severity(items[j].Verdict)
		}
		return items[i].Name < items[j].Name
	})
}

// sortUnconventional sorterar avvikelser efter hur tydliga de är.
func sortUnconventional(items []model.UnconventionalConcept) {
	sort.Slice(items, func(i, j int) bool {
		if severity(items[i].Verdict) != severity(items[j].Verdict) {
			return severity(items[i].Verdict) > severity(items[j].Verdict)
		}
		return items[i].Name < items[j].Name
	})
}

// severity rangordnar en bedömning.
func severity(value string) int {
	switch value {
	case "contradiction":
		return 3
	case "unconventional":
		return 2
	case "nuance":
		return 1
	default:
		return 0
	}
}

// round avrundar till given decimal.
func round(value float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int64(value*factor+0.5)) / factor
}
