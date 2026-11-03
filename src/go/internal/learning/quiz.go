// Package learning innehåller frågegenerering, spaced repetition och förklaringar.
package learning

import (
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/simon/repo-template/internal/model"
)

// Frågetyper.
const (
	KindMultipleChoice = "multiple_choice"
	KindTrueFalse      = "true_false"
	KindShortAnswer    = "short_answer"
	KindEssay          = "essay"
)

// StandardKinds är de frågetyper som används om inget annat anges.
var StandardKinds = []string{KindMultipleChoice, KindTrueFalse, KindShortAnswer, KindEssay}

// QuizEngine genererar kunskapsfrågor ur koncept.
type QuizEngine struct {
	rng *rand.Rand
}

// NewQuizEngine skapar en motor med given seed för reproducerbara frågor.
func NewQuizEngine(seed int64) *QuizEngine {
	return &QuizEngine{rng: rand.New(rand.NewSource(seed))}
}

// Generate skapar count frågor fördelade över de angivna typerna.
func (e *QuizEngine) Generate(concepts []model.Concept, count int, kinds []string) []model.Question {
	usable := make([]model.Concept, 0, len(concepts))
	for _, concept := range concepts {
		if concept.Definition != "" || concept.Summary != "" {
			usable = append(usable, concept)
		}
	}
	if len(usable) == 0 {
		return nil
	}
	if len(kinds) == 0 {
		kinds = StandardKinds
	}
	sort.SliceStable(usable, func(i, j int) bool { return usable[i].Importance > usable[j].Importance })

	poolSize := count * 3
	if poolSize < count {
		poolSize = count
	}
	pool := usable
	if len(pool) > poolSize {
		pool = pool[:poolSize]
	}
	e.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	var questions []model.Question
	index := 0
	for len(questions) < count && len(pool) > 0 {
		concept := pool[index%len(pool)]
		kind := kinds[len(questions)%len(kinds)]
		if question, ok := e.makeQuestion(concept, pool, kind); ok {
			questions = append(questions, question)
		}
		index++
		if index > count*5 {
			break
		}
	}
	return questions
}

func (e *QuizEngine) makeQuestion(concept model.Concept, pool []model.Concept, kind string) (model.Question, bool) {
	switch kind {
	case KindMultipleChoice:
		return e.multipleChoice(concept, pool)
	case KindTrueFalse:
		return e.trueFalse(concept, pool)
	case KindShortAnswer:
		return e.shortAnswer(concept)
	case KindEssay:
		return e.essay(concept, pool)
	}
	return model.Question{}, false
}

func (e *QuizEngine) multipleChoice(concept model.Concept, pool []model.Concept) (model.Question, bool) {
	correct := concept.Definition
	if correct == "" {
		correct = concept.Summary
	}
	if correct == "" {
		return model.Question{}, false
	}
	var distractors []string
	for _, other := range pool {
		if other.ID == concept.ID {
			continue
		}
		detail := other.Definition
		if detail == "" {
			detail = other.Summary
		}
		if detail != "" {
			distractors = append(distractors, detail)
		}
	}
	if len(distractors) < 2 {
		return model.Question{}, false
	}
	count := 3
	if len(distractors) < count {
		count = len(distractors)
	}
	e.rng.Shuffle(len(distractors), func(i, j int) { distractors[i], distractors[j] = distractors[j], distractors[i] })
	options := append(append([]string{}, distractors[:count]...), correct)
	e.rng.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })

	explanation := concept.Summary
	if explanation == "" {
		explanation = correct
	}
	return model.Question{
		BookID:      concept.BookID,
		ConceptID:   concept.ID,
		Kind:        KindMultipleChoice,
		Prompt:      "Vad betyder " + concept.Name + "?",
		Options:     options,
		Answer:      correct,
		Explanation: explanation,
		Difficulty:  difficulty(concept),
	}, true
}

func (e *QuizEngine) trueFalse(concept model.Concept, pool []model.Concept) (model.Question, bool) {
	statement := concept.Definition
	if statement == "" {
		statement = concept.Summary
	}
	makeTrue := statement != ""
	var others []model.Concept
	for _, other := range pool {
		if other.ID != concept.ID && (other.Definition != "" || other.Summary != "") {
			others = append(others, other)
		}
	}
	if makeTrue && len(others) == 0 {
		return model.Question{}, false
	}
	answer := "Rätt"
	if !makeTrue {
		other := others[e.rng.Intn(len(others))]
		statement = other.Definition
		if statement == "" {
			statement = other.Summary
		}
		answer = "Falskt"
	}
	explanation := concept.Definition
	if explanation == "" {
		explanation = concept.Summary
	}
	return model.Question{
		BookID:      concept.BookID,
		ConceptID:   concept.ID,
		Kind:        KindTrueFalse,
		Prompt:      "Stämmer detta på " + concept.Name + "? \u201c" + statement + "\u201d",
		Options:     []string{"Rätt", "Falskt"},
		Answer:      answer,
		Explanation: explanation,
		Difficulty:  difficulty(concept),
	}, true
}

func (e *QuizEngine) shortAnswer(concept model.Concept) (model.Question, bool) {
	source := concept.Definition
	if source == "" {
		source = concept.Summary
	}
	if source == "" {
		return model.Question{}, false
	}
	words := strings.Fields(source)
	if len(words) > 6 {
		words = words[:6]
	}
	return model.Question{
		BookID:      concept.BookID,
		ConceptID:   concept.ID,
		Kind:        KindShortAnswer,
		Prompt:      "Beskriv " + concept.Name + " kortfattat.",
		Options:     nil,
		Answer:      strings.Join(words, " "),
		Explanation: source,
		Difficulty:  difficulty(concept),
	}, true
}

func (e *QuizEngine) essay(concept model.Concept, pool []model.Concept) (model.Question, bool) {
	other := concept
	var others []model.Concept
	for _, candidate := range pool {
		if candidate.ID != concept.ID {
			others = append(others, candidate)
		}
	}
	if len(others) > 0 {
		other = others[e.rng.Intn(len(others))]
	}
	explanation := concept.Definition
	if explanation == "" {
		explanation = concept.Summary
	}
	return model.Question{
		BookID:    concept.BookID,
		ConceptID: concept.ID,
		Kind:      KindEssay,
		Prompt: "Förklara hur " + concept.Name + " hänger ihop med " + other.Name +
			" och ge ett eget exempel.",
		Options:     nil,
		Answer:      "",
		Explanation: concept.Name + ": " + explanation,
		Difficulty:  "hard",
	}, true
}

func difficulty(concept model.Concept) string {
	switch {
	case concept.Importance >= 0.6:
		return "hard"
	case concept.Importance >= 0.3:
		return "medium"
	default:
		return "easy"
	}
}

// ---------------------------------------------------------------------------
// Spaced repetition (SM-2)
// ---------------------------------------------------------------------------

// Scheduler beräknar repetitionsplan efter ett svar.
type Scheduler struct{}

// Update returnerar den nya planen för ett svar.
func (s Scheduler) Update(previous *model.Attempt, correct bool) model.Attempt {
	now := time.Now()
	attempt := model.Attempt{
		Correct:    correct,
		AnsweredAt: now,
		EaseFactor: 2.5,
	}
	if previous != nil {
		attempt.QuestionID = previous.QuestionID
		attempt.ConceptID = previous.ConceptID
		attempt.EaseFactor = previous.EaseFactor
		attempt.IntervalDays = previous.IntervalDays
		attempt.DueAt = previous.DueAt
		attempt.Repetitions = previous.Repetitions
	}

	quality := 4.0
	if !correct {
		quality = 1
	}
	if quality < 3 {
		attempt.Repetitions = 0
		attempt.IntervalDays = 0
		attempt.DueAt = now
	} else {
		attempt.Repetitions++
		switch attempt.Repetitions {
		case 1:
			attempt.IntervalDays = 1
		case 2:
			attempt.IntervalDays = 3
		default:
			interval := attempt.IntervalDays
			if interval < 1 {
				interval = 1
			}
			attempt.IntervalDays = round(interval*attempt.EaseFactor, 2)
		}
		attempt.DueAt = now.Add(time.Duration(attempt.IntervalDays*24) * time.Hour)
	}
	ease := attempt.EaseFactor + (0.1 - (5-quality)*(0.08+(5-quality)*0.02))
	if ease < 1.3 {
		ease = 1.3
	}
	attempt.EaseFactor = round(ease, 3)
	return attempt
}

// DueQuestions prioriterar frågor som är förfaller, ovanstått nya.
func DueQuestions(questions []model.Question, attempts map[int64]model.Attempt, now time.Time) []model.Question {
	type entry struct {
		rank     int
		order    int
		question model.Question
	}
	var due []entry
	for index, question := range questions {
		attempt, answered := attempts[question.ID]
		switch {
		case !answered:
			due = append(due, entry{rank: 0, order: index, question: question})
		case !attempt.DueAt.After(now):
			due = append(due, entry{rank: 1, order: index, question: question})
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		if due[i].rank != due[j].rank {
			return due[i].rank < due[j].rank
		}
		return due[i].order < due[j].order
	})
	result := make([]model.Question, 0, len(due))
	for _, item := range due {
		result = append(result, item.question)
	}
	return result
}

func round(value float64, decimals int) float64 {
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	return float64(int64(value*factor+0.5)) / factor
}
