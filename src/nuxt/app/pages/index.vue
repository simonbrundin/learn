<template>
  <div class="page">
    <header class="header">
      <div>
        <h1>Learn</h1>
        <p class="subtitle">Kunskapsbas från dina e-puber – koncept, grafer och quiz.</p>
      </div>
      <div class="status" :class="{ error: error }">
        {{ error ? 'Fel: ' + error : loading ? 'Laddar…' : 'Ansluten' }}
      </div>
    </header>

    <div class="layout">
      <aside class="panel">
        <h2>Böcker</h2>
        <ul v-if="books.length" class="books">
          <li v-for="book in books" :key="book.id">
            <button
              class="book"
              :class="{ active: book.id === selectedBookId }"
              @click="selectBook(book.id)"
            >
              <span class="book-title">{{ book.title }}</span>
              <span class="book-meta">{{ book.author || 'okänd författare' }}</span>
            </button>
          </li>
        </ul>
        <p v-else class="hint">
          Inga böcker importerade ännu. Kör <code>learn import bok.epub</code> för att komma igång.
        </p>

        <template v-if="selectedBook">
          <h2>Koncept</h2>
          <p class="hint">{{ concepts.length }} koncept i boken</p>
          <ul class="concepts">
            <li v-for="concept in concepts" :key="concept.id">
              <button class="concept" @click="explain(concept)">
                <span class="concept-name">{{ concept.name }}</span>
                <span class="bar" :style="{ width: Math.round(concept.importance * 100) + '%' }" />
              </button>
            </li>
          </ul>
        </template>
      </aside>

      <main class="panel">
        <template v-if="selectedBook">
          <section>
            <h2>Kunskapsgraf</h2>
            <svg v-if="graph.nodes.length" class="graph" :viewBox="`0 0 ${graphWidth} ${graphHeight}`">
              <g>
                <line
                  v-for="(edge, index) in graph.edges"
                  :key="'e' + index"
                  :x1="positionOf(edge.source).x"
                  :y1="positionOf(edge.source).y"
                  :x2="positionOf(edge.target).x"
                  :y2="positionOf(edge.target).y"
                  :class="'edge ' + edge.relation"
                >
                  <title>{{ edge.relation }} ({{ edge.weight }})</title>
                </line>
              </g>
              <g v-for="node in graph.nodes" :key="node.id">
                <circle
                  :cx="positionOf(node.id).x"
                  :cy="positionOf(node.id).y"
                  :r="nodeRadius(node)"
                  :class="'node ' + (selectedConcept?.id === node.id ? 'selected' : '')"
                  @click="selectNode(node)"
                >
                  <title>{{ node.name }}</title>
                </circle>
                <text
                  :x="positionOf(node.id).x"
                  :y="positionOf(node.id).y + 4"
                  class="label"
                  text-anchor="middle"
                >
                  {{ truncate(node.name, 18) }}
                </text>
              </g>
            </svg>
            <p v-else class="hint">Ingen graf att visa ännu.</p>
            <div class="legend">
              <span v-for="relation in relations" :key="relation" class="legend-item">
                <i class="dot" :class="relation" /> {{ relation }}
              </span>
            </div>
          </section>

          <section v-if="explanation">
            <h2>{{ explanation.title }}</h2>
            <p class="definition">{{ explanation.definition }}</p>
            <ul v-if="explanation.key_points?.length" class="points">
              <li v-for="point in explanation.key_points" :key="point">{{ point }}</li>
            </ul>
            <p v-if="explanation.quotes?.length" class="quote">“{{ explanation.quotes[0] }}”</p>
            <div v-if="explanation.related?.length" class="related">
              <span v-for="item in explanation.related" :key="item.name" class="chip">
                {{ item.name }} <em>{{ item.relation }}</em>
              </span>
            </div>
          </section>

          <section>
            <h2>Ställ en fråga</h2>
            <form class="ask" @submit.prevent="ask">
              <input v-model="question" type="text" placeholder="Vad är System 1?" />
              <button type="submit" :disabled="asking">Fråga</button>
            </form>
            <pre v-if="answer" class="answer">{{ answer }}</pre>
          </section>

          <section>
            <h2>Kunskapstest</h2>
            <button class="primary" @click="loadQuiz" :disabled="quizLoading">
              {{ quiz.length ? 'Nya frågor' : 'Starta quiz' }}
            </button>
            <div v-if="quiz.length" class="quiz">
              <article v-for="(question, index) in quiz" :key="question.id" class="question">
                <p class="question-text">{{ index + 1 }}. {{ question.prompt }}</p>
                <ol v-if="question.options?.length" class="options">
                  <li v-for="(option, optionIndex) in question.options" :key="optionIndex">
                    <button class="option" @click="answerQuestion(question, option)">
                      {{ option }}
                    </button>
                  </li>
                </ol>
                <p v-if="results[question.id]" class="result" :class="{ correct: results[question.id].correct }">
                  {{ results[question.id].correct ? 'Rätt!' : 'Fel.' }}
                  {{ results[question.id].explanation }}
                </p>
              </article>
            </div>
          </section>
        </template>

        <p v-else class="hint">Välj en bok för att se dess kunskapsgraf.</p>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Book {
  id: number
  title: string
  author: string
  description: string
}

interface Concept {
  id: number
  name: string
  definition: string
  summary: string
  importance: number
  keywords: string[]
}

interface GraphNode {
  id: number
  name: string
  importance: number
  definition: string
}

interface GraphEdge {
  source: number
  target: number
  relation: string
  weight: number
}

interface Question {
  id: number
  kind: string
  prompt: string
  options: string[]
  answer: string
  explanation: string
  difficulty: string
}

interface Explanation {
  title: string
  level: string
  definition: string
  key_points: string[]
  quotes: string[]
  related: { name: string; relation: string }[]
}

const graphWidth = 640
const graphHeight = 420
const relations = ['related', 'contrast', 'cause', 'effect']

const loading = ref(true)
const error = ref<string | null>(null)
const books = ref<Book[]>([])
const selectedBookId = ref<number | null>(null)
const selectedBook = computed(
  () => books.value.find((book) => book.id === selectedBookId.value) ?? null,
)
const concepts = ref<Concept[]>([])
const graph = ref<{ nodes: GraphNode[]; edges: GraphEdge[] }>({ nodes: [], edges: [] })
const selectedConcept = ref<Concept | null>(null)
const explanation = ref<Explanation | null>(null)
const question = ref('')
const asking = ref(false)
const answer = ref('')
const quiz = ref<Question[]>([])
const quizLoading = ref(false)
const results = ref<Record<number, { correct: boolean; explanation: string }>>({})

async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  return (await response.json()) as T
}

async function loadBooks() {
  loading.value = true
  error.value = null
  try {
    const response = await api<{ books: Book[] }>('/books')
    books.value = response.books
    if (!selectedBookId.value && books.value.length) {
      await selectBook(books.value[0].id)
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Okänt fel'
  } finally {
    loading.value = false
  }
}

async function selectBook(id: number) {
  selectedBookId.value = id
  selectedConcept.value = null
  explanation.value = null
  answer.value = ''
  quiz.value = []
  results.value = {}
  try {
    const [conceptResponse, graphResponse] = await Promise.all([
      api<{ concepts: Concept[] }>(`/books/${id}/concepts?limit=40`),
      api<{ nodes: GraphNode[]; edges: GraphEdge[] }>(`/books/${id}/graph`),
    ])
    concepts.value = conceptResponse.concepts
    graph.value = graphResponse
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Okänt fel'
  }
}

async function explain(concept: Concept) {
  selectedConcept.value = concept
  try {
    explanation.value = await api<Explanation>(`/concepts/${concept.id}/explain?level=standard`)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Okänt fel'
  }
}

function selectNode(node: GraphNode) {
  const concept = concepts.value.find((item) => item.id === node.id)
  if (concept) {
    explain(concept)
  }
}

async function ask() {
  if (!question.value.trim() || !selectedBookId.value) {
    return
  }
  asking.value = true
  try {
    const response = await api<{ answer: string }>(
      `/ask?q=${encodeURIComponent(question.value)}&bookId=${selectedBookId.value}`,
    )
    answer.value = response.answer
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Okänt fel'
  } finally {
    asking.value = false
  }
}

async function loadQuiz() {
  if (!selectedBookId.value) {
    return
  }
  quizLoading.value = true
  results.value = {}
  try {
    const response = await api<{ questions: Question[] }>(
      `/books/${selectedBookId.value}/quiz?count=5&regenerate=true`,
    )
    quiz.value = response.questions
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Okänt fel'
  } finally {
    quizLoading.value = false
  }
}

async function answerQuestion(item: Question, option: string) {
  const correct = option === item.answer
  results.value = { ...results.value, [item.id]: { correct, explanation: item.explanation } }
  try {
    await api(`/questions/${item.id}/attempt`, {
      method: 'POST',
      body: JSON.stringify({ correct, answer: option }),
    })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Okänt fel'
  }
}

// Enkel radial layout: de viktigaste koncepten i mitten, resten i en ring.
const positions = computed(() => {
  const result = new Map<number, { x: number; y: number }>()
  const nodes = [...graph.value.nodes].sort((a, b) => b.importance - a.importance)
  const centerX = graphWidth / 2
  const centerY = graphHeight / 2
  const core = nodes.slice(0, 3)
  const ring = nodes.slice(3)
  const coreRadius = 46
  core.forEach((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(core.length, 1) - Math.PI / 2
    result.set(node.id, {
      x: centerX + Math.cos(angle) * coreRadius,
      y: centerY + Math.sin(angle) * coreRadius,
    })
  })
  const radius = Math.min(graphWidth, graphHeight) / 2 - 70
  ring.forEach((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(ring.length, 1) - Math.PI / 2
    result.set(node.id, {
      x: centerX + Math.cos(angle) * radius,
      y: centerY + Math.sin(angle) * radius,
    })
  })
  return result
})

function positionOf(id: number) {
  return positions.value.get(id) ?? { x: graphWidth / 2, y: graphHeight / 2 }
}

function nodeRadius(node: GraphNode) {
  return 10 + Math.round(node.importance * 22)
}

function truncate(value: string, length: number) {
  return value.length > length ? value.slice(0, length - 1) + '…' : value
}

onMounted(loadBooks)
</script>

<style scoped>
.page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem 1.5rem 4rem;
  color: #e5e7eb;
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
}

.header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

h1 {
  margin: 0;
  font-size: 1.8rem;
  letter-spacing: -0.02em;
}

h2 {
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: #94a3b8;
  margin: 0 0 0.6rem;
}

.subtitle {
  margin: 0.35rem 0 0;
  color: #94a3b8;
}

.status {
  font-size: 0.85rem;
  color: #34d399;
  background: rgba(52, 211, 153, 0.1);
  border: 1px solid rgba(52, 211, 153, 0.25);
  padding: 0.25rem 0.7rem;
  border-radius: 999px;
  white-space: nowrap;
}

.status.error {
  color: #f87171;
  background: rgba(248, 113, 113, 0.1);
  border-color: rgba(248, 113, 113, 0.3);
}

.layout {
  display: grid;
  grid-template-columns: 260px 1fr;
  gap: 1.5rem;
  align-items: start;
}

@media (max-width: 860px) {
  .layout {
    grid-template-columns: 1fr;
  }
}

.panel {
  background: #111827;
  border: 1px solid #1f2937;
  border-radius: 14px;
  padding: 1.1rem;
}

aside.panel section + section,
main.panel section + section {
  margin-top: 1.8rem;
}

ul {
  list-style: none;
  margin: 0;
  padding: 0;
}

.books button,
.concept {
  display: block;
  width: 100%;
  text-align: left;
  background: transparent;
  border: 1px solid transparent;
  color: inherit;
  padding: 0.5rem 0.6rem;
  border-radius: 8px;
  cursor: pointer;
  font: inherit;
}

.books button:hover,
.concept:hover {
  background: #1f2937;
}

.book.active {
  background: #1d4ed8;
  color: #fff;
}

.book-title {
  display: block;
  font-weight: 600;
}

.book-meta {
  display: block;
  font-size: 0.8rem;
  color: #94a3b8;
}

.book.active .book-meta {
  color: #dbeafe;
}

.concepts {
  max-height: 320px;
  overflow-y: auto;
}

.concept-name {
  display: block;
  font-size: 0.9rem;
}

.bar {
  display: block;
  height: 3px;
  margin-top: 4px;
  background: #3b82f6;
  border-radius: 2px;
  min-width: 4px;
}

.hint {
  color: #94a3b8;
  font-size: 0.88rem;
}

.graph {
  width: 100%;
  height: auto;
  background: #0b1220;
  border: 1px solid #1f2937;
  border-radius: 12px;
}

.edge {
  stroke: #334155;
  stroke-width: 1.5;
}

.edge.contrast {
  stroke: #f59e0b;
}

.edge.cause,
.edge.effect {
  stroke: #34d399;
}

.node {
  fill: #1d4ed8;
  stroke: #93c5fd;
  stroke-width: 1.5;
  cursor: pointer;
}

.node.selected {
  fill: #f59e0b;
  stroke: #fde68a;
}

.label {
  fill: #cbd5f5;
  font-size: 10px;
  pointer-events: none;
}

.legend {
  display: flex;
  gap: 1rem;
  margin-top: 0.6rem;
  font-size: 0.78rem;
  color: #94a3b8;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.dot {
  width: 10px;
  height: 2px;
  background: #334155;
  display: inline-block;
}

.dot.contrast {
  background: #f59e0b;
}

.dot.cause,
.dot.effect {
  background: #34d399;
}

.definition {
  margin: 0 0 0.6rem;
  line-height: 1.5;
}

.points {
  margin: 0;
  padding-left: 1.1rem;
  color: #cbd5f5;
  font-size: 0.9rem;
  line-height: 1.5;
}

.quote {
  border-left: 3px solid #3b82f6;
  padding-left: 0.8rem;
  color: #cbd5f5;
  font-style: italic;
  margin: 0.8rem 0;
}

.related {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-top: 0.6rem;
}

.chip {
  font-size: 0.78rem;
  background: #1f2937;
  border-radius: 999px;
  padding: 0.2rem 0.65rem;
}

.chip em {
  color: #94a3b8;
  font-style: normal;
}

.ask {
  display: flex;
  gap: 0.5rem;
}

.ask input {
  flex: 1;
  background: #0b1220;
  border: 1px solid #1f2937;
  border-radius: 8px;
  color: inherit;
  padding: 0.55rem 0.7rem;
  font: inherit;
}

button {
  font: inherit;
  cursor: pointer;
}

.ask button,
.primary {
  background: #2563eb;
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 0.55rem 1rem;
}

.ask button:disabled,
.primary:disabled {
  opacity: 0.5;
  cursor: default;
}

.answer {
  white-space: pre-wrap;
  background: #0b1220;
  border: 1px solid #1f2937;
  border-radius: 10px;
  padding: 0.8rem;
  font-size: 0.85rem;
  line-height: 1.5;
}

.quiz {
  margin-top: 0.8rem;
  display: grid;
  gap: 0.8rem;
}

.question {
  background: #0b1220;
  border: 1px solid #1f2937;
  border-radius: 10px;
  padding: 0.8rem;
}

.question-text {
  margin: 0 0 0.6rem;
  font-weight: 600;
}

.options {
  margin: 0;
  padding-left: 1.2rem;
  display: grid;
  gap: 0.3rem;
}

.option {
  background: transparent;
  border: 1px solid #1f2937;
  color: #cbd5f5;
  border-radius: 8px;
  padding: 0.4rem 0.6rem;
  text-align: left;
  font: inherit;
}

.option:hover {
  border-color: #3b82f6;
}

.result {
  margin: 0.6rem 0 0;
  font-size: 0.85rem;
  color: #f87171;
}

.result.correct {
  color: #34d399;
}
</style>
