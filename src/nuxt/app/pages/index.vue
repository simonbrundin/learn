<template>
  <div class="page">
    <header class="header">
      <div>
        <h1>Learn</h1>
        <p class="subtitle">Kunskapsbas från dina e-puber – koncept, grafer och quiz.</p>
      </div>
      <div class="status" :class="{ error: error }" role="status">
        {{ error ? 'Fel: ' + error : loading ? 'Laddar…' : 'Ansluten' }}
      </div>
    </header>

    <div class="layout" :key="selectedBookId ?? 'none'">
      <aside class="column">
        <section v-if="library" class="card" :style="{ '--i': 0 }">
          <h2>Kunskapsbasen</h2>
          <div class="stats compact">
            <div class="stat">
              <span class="stat-value">{{ library.books }}</span>
              <span class="stat-label">böcker</span>
            </div>
            <div class="stat">
              <span class="stat-value">{{ library.concepts }}</span>
              <span class="stat-label">koncept</span>
            </div>
            <div class="stat">
              <span class="stat-value">{{ library.distinct_names }}</span>
              <span class="stat-label">unika begrepp</span>
            </div>
          </div>
          <p class="hint">
            {{ library.shared_names }} begrepp återfinns i flera böcker
            ({{ Math.round(library.corpus_coverage * 100) }} % av alla).
          </p>
          <ul v-if="library.most_unconventional.length" class="insight-list">
            <li v-for="item in library.most_unconventional.slice(0, 3)" :key="item.name">
              <span class="insight-name">
                {{ item.name }}
                <em class="verdict" :class="item.verdict">{{ verdictLabel(item.verdict) }}</em>
              </span>
            </li>
          </ul>
        </section>

        <section class="card" :style="{ '--i': 0 }">
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
        </section>

        <section v-if="selectedBook" class="card" :style="{ '--i': 1 }">
          <h2>Koncept</h2>
          <p class="hint">{{ concepts.length }} koncept i boken</p>
          <ul class="concepts">
            <li v-for="concept in concepts" :key="concept.id">
              <button
                class="concept"
                :class="{ active: selectedConcept?.id === concept.id }"
                @click="explain(concept)"
              >
                <span class="concept-name">{{ concept.name }}</span>
                <span class="concept-meta">{{ concept.importance.toFixed(2) }}</span>
                <span class="bar">
                  <i :style="{ width: Math.round(concept.importance * 100) + '%' }" />
                </span>
              </button>
            </li>
          </ul>
        </section>
      </aside>

      <main class="column">
        <template v-if="selectedBook">
          <section class="card" :style="{ '--i': 1 }">
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
                  :style="{ '--i': index }"
                >
                  <title>{{ edge.relation }} ({{ edge.weight }})</title>
                </line>
              </g>
              <g v-for="node in graph.nodes" :key="node.id">
                <circle
                  v-if="node.id === strongestNodeId"
                  :cx="positionOf(node.id).x"
                  :cy="positionOf(node.id).y"
                  :r="nodeRadius(node) + 10"
                  class="halo"
                />
                <circle
                  :cx="positionOf(node.id).x"
                  :cy="positionOf(node.id).y"
                  :r="nodeRadius(node)"
                  :class="[
                    'node',
                    node.id === strongestNodeId ? 'strongest' : '',
                    selectedConcept?.id === node.id ? 'selected' : '',
                  ]"
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

          <section v-if="insightLoading && !insight" class="card" :style="{ '--i': 2 }">
            <h2>Analys mot kunskapsbasen</h2>
            <p class="hint">Jämför boken med resten av biblioteket…</p>
          </section>

          <section v-else-if="insight" class="card" :style="{ '--i': 2 }">
            <h2>Analys mot kunskapsbasen</h2>
            <div class="stats">
              <div class="stat">
                <span class="stat-value">{{ insight.totals.unique }}</span>
                <span class="stat-label">nya koncept</span>
              </div>
              <div class="stat">
                <span class="stat-value">{{ Math.round(insight.totals.coverage * 100) }}%</span>
                <span class="stat-label">av boken är nytt</span>
              </div>
              <div class="stat" :class="{ warn: insight.totals.unconventional > 0 }">
                <span class="stat-value">{{ insight.totals.unconventional }}</span>
                <span class="stat-label">avvikelser</span>
              </div>
              <div class="stat" :class="{ warn: insight.totals.contradicting > 0 }">
                <span class="stat-value">{{ insight.totals.contradicting }}</span>
                <span class="stat-label">motsägelser</span>
              </div>
              <div class="stat">
                <span class="stat-value">{{ insight.totals.novel_keywords }}</span>
                <span class="stat-label">nya nyckelord</span>
              </div>
            </div>
            <p class="hint">
              Jämfört med {{ insight.totals.other_books }} andra böcker i biblioteket.
            </p>

            <nav class="tabs" role="tablist">
              <button
                v-for="tab in insightTabs"
                :key="tab.id"
                role="tab"
                :aria-selected="insightTab === tab.id"
                :class="{ active: insightTab === tab.id }"
                @click="insightTab = tab.id"
              >
                {{ tab.label }}
                <em>{{ tab.count }}</em>
              </button>
            </nav>

            <ul v-if="insightTab === 'novel'" class="insight-list">
              <li v-for="item in insight.novel" :key="item.name">
                <span class="insight-name">{{ item.name }}</span>
                <span v-if="item.definition" class="insight-text">{{ item.definition }}</span>
                <span v-else-if="item.summary" class="insight-text">{{ item.summary }}</span>
                <span v-if="item.novel_keywords.length" class="insight-tags">
                  <i v-for="keyword in item.novel_keywords" :key="keyword">{{ keyword }}</i>
                </span>
              </li>
              <li v-if="!insight.novel.length" class="hint">
                Boken tillför inga helt nya koncept.
              </li>
            </ul>

            <ul v-else-if="insightTab === 'unconventional'" class="insight-list">
              <li v-for="item in insight.unconventional" :key="item.name">
                <span class="insight-name">
                  {{ item.name }}
                  <em class="verdict" :class="item.verdict">{{ verdictLabel(item.verdict) }}</em>
                </span>
                <span v-if="item.note" class="insight-text">{{ item.note }}</span>
                <span v-if="item.here" class="insight-here">Här: {{ item.here }}</span>
                <span v-if="item.elsewhere?.length" class="insight-there">
                  I {{ item.elsewhere[0].book_title }}: {{ item.elsewhere[0].definition }}
                </span>
              </li>
              <li v-if="!insight.unconventional.length" class="hint">
                Boken beskriver inget på ett sätt som avviker från övriga böcker.
              </li>
            </ul>

            <ul v-else class="insight-list">
              <li v-for="link in insight.cross_links" :key="link.concept + link.book_title">
                <span class="insight-name">
                  {{ link.concept }}
                  <span v-if="link.other_concept" class="arrow">→</span>
                  <span v-if="link.other_concept">{{ link.other_concept }}</span>
                </span>
                <span class="insight-text">i {{ link.book_title }}</span>
                <span class="insight-tags">
                  <i v-for="keyword in link.keywords" :key="keyword">{{ keyword }}</i>
                </span>
              </li>
              <li v-if="!insight.cross_links.length" class="hint">
                Inga korskopplingar hittades.
              </li>
            </ul>
          </section>

          <section v-if="explanation" :key="explanation.title" class="card" :style="{ '--i': 2 }">
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

          <section class="card" :style="{ '--i': 3 }">
            <h2>Ställ en fråga</h2>
            <form class="ask" @submit.prevent="ask">
              <input
                v-model="question"
                type="text"
                placeholder="Vad är System 1?"
                aria-label="Fråga om bokens innehåll"
              />
              <button type="submit" :disabled="asking">Fråga</button>
            </form>
            <pre v-if="answer" class="answer">{{ answer }}</pre>
          </section>

          <section class="card" :style="{ '--i': 4 }">
            <h2>Kunskapstest</h2>
            <button class="primary" @click="loadQuiz" :disabled="quizLoading">
              {{ quiz.length ? 'Nya frågor' : 'Starta quiz' }}
            </button>
            <div v-if="quiz.length" class="quiz">
              <article v-for="(item, index) in quiz" :key="item.id" class="question">
                <p class="question-text">{{ index + 1 }}. {{ item.prompt }}</p>
                <ol v-if="item.options?.length" class="options">
                  <li v-for="(option, optionIndex) in item.options" :key="optionIndex">
                    <button class="option" @click="answerQuestion(item, option)">
                      {{ option }}
                    </button>
                  </li>
                </ol>
                <p v-if="results[item.id]" class="result" :class="{ correct: results[item.id].correct }">
                  {{ results[item.id].correct ? 'Rätt!' : 'Fel.' }}
                  {{ results[item.id].explanation }}
                </p>
              </article>
            </div>
          </section>
        </template>

        <section v-else class="card" :style="{ '--i': 0 }">
          <h2>Kunskapsbas</h2>
          <p class="hint">Välj en bok för att se dess kunskapsgraf.</p>
        </section>
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

interface InsightVersion {
  book_id: number
  book_title: string
  definition: string
  similarity: number
  verdict: string
  negated: boolean
}

interface InsightNovel {
  name: string
  definition: string
  summary: string
  importance: number
  mentions: number
  keywords: string[]
  novel_keywords: string[]
}

interface InsightShared {
  name: string
  importance: number
  here: string
  elsewhere: InsightVersion[]
  verdict: string
  novel_keywords: string[]
  note?: string
}

interface InsightLink {
  concept: string
  other_concept: string
  book_id: number
  book_title: string
  keywords: string[]
  strength: number
}

interface InsightConflict {
  source: string
  target: string
  here_relation: string
  other_relation: string
  book_id: number
  book_title: string
}

interface BookInsight {
  book: Book
  totals: {
    concepts: number
    unique: number
    shared: number
    unconventional: number
    contradicting: number
    novel_keywords: number
    coverage: number
    other_books: number
  }
  novel: InsightNovel[]
  shared: InsightShared[]
  unconventional: InsightShared[]
  relation_conflicts: InsightConflict[]
  cross_links: InsightLink[]
}

interface LibraryInsight {
  books: number
  concepts: number
  distinct_names: number
  shared_names: number
  corpus_coverage: number
  most_shared: InsightShared[]
  most_unconventional: InsightShared[]
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
const insight = ref<BookInsight | null>(null)
const insightLoading = ref(false)
const insightTab = ref<'novel' | 'unconventional' | 'links'>('novel')
const library = ref<LibraryInsight | null>(null)

// Flikarna visar ny kunskap, avvikelser och korskopplingar.
const insightTabs = computed(() => {
  const data = insight.value
  return [
    { id: 'novel' as const, label: 'Ny kunskap', count: data?.novel.length ?? 0 },
    { id: 'unconventional' as const, label: 'Okonventionellt', count: data?.unconventional.length ?? 0 },
    { id: 'links' as const, label: 'Kopplingar', count: data?.cross_links.length ?? 0 },
  ]
})

// verdictLabel översätter bedömningen till svenska.
function verdictLabel(verdict: string) {
  switch (verdict) {
    case 'contradiction':
      return 'motsägelse'
    case 'unconventional':
      return 'avvikande'
    case 'nuance':
      return 'nyans'
    default:
      return 'i enlighet'
  }
}

// Noden med högst vikt får grafen enda pulserande markering.
const strongestNodeId = computed(() => {
  let best: GraphNode | null = null
  for (const node of graph.value.nodes) {
    if (!best || node.importance > best.importance) {
      best = node
    }
  }
  return best?.id ?? null
})

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
    const [response, libraryResponse] = await Promise.all([
      api<{ books: Book[] }>('/books'),
      api<LibraryInsight>('/insight').catch(() => null),
    ])
    books.value = response.books
    library.value = libraryResponse
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
  insight.value = null
  insightTab.value = 'novel'
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
  await loadInsight(id)
}

// Analysen jämför boken med resten av kunskapsbasen.
async function loadInsight(id: number) {
  insightLoading.value = true
  try {
    insight.value = await api<BookInsight>(`/books/${id}/insight`)
  } catch (e) {
    insight.value = null
  } finally {
    insightLoading.value = false
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
/* ============================================================
   A3 · Folio – djup oliv mark, terrakotta, serif genomgående.
   Rörelse: 300ms enter · 40ms stagger · 120ms hover · 3s puls.
   ============================================================ */

.page {
  --bg: #121513;
  --panel: #1a1e1b;
  --panel-2: #212722;
  --text: #f0efe9;
  --text-dim: #99a096;
  --accent: #c97f5a;
  --accent-soft: #e2a582;
  --line: #2a312b;
  --line-strong: #3a423b;
  --sage: #8fae86;
  --radius: 4px;
  --ease: cubic-bezier(0.2, 0, 0, 1);
  --serif: Georgia, 'Iowan Old Style', 'Times New Roman', serif;
  --sans: system-ui, -apple-system, 'Segoe UI', sans-serif;
  --mono: ui-monospace, 'SF Mono', Menlo, monospace;

  min-height: 100vh;
  background: var(--bg);
  color: var(--text);
  font-family: var(--sans);
  font-size: 15px;
  line-height: 1.55;
  padding: 2.4rem 1.6rem 5rem;
}

:global(body) {
  margin: 0;
  background: #121513;
  color-scheme: dark;
}

.header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  max-width: 1240px;
  margin: 0 auto 2rem;
  padding-bottom: 1.2rem;
  border-bottom: 1px solid var(--line);
}

h1 {
  margin: 0;
  font-family: var(--serif);
  font-size: 2.4rem;
  font-weight: 400;
  letter-spacing: -0.015em;
}

h2 {
  font-family: var(--mono);
  font-size: 0.66rem;
  font-weight: 500;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--text-dim);
  margin: 0 0 0.9rem;
}

.subtitle {
  margin: 0.3rem 0 0;
  color: var(--text-dim);
  font-size: 0.9rem;
}

.status {
  font-family: var(--mono);
  font-size: 0.68rem;
  letter-spacing: 0.1em;
  color: var(--sage);
  border: 1px solid rgba(143, 174, 134, 0.35);
  background: rgba(143, 174, 134, 0.08);
  padding: 0.3rem 0.75rem;
  border-radius: 999px;
  white-space: nowrap;
}

.status.error {
  color: var(--accent-soft);
  background: rgba(201, 127, 90, 0.1);
  border-color: rgba(201, 127, 90, 0.35);
}

.layout {
  display: grid;
  grid-template-columns: 272px 1fr;
  gap: 1.4rem;
  align-items: start;
  max-width: 1240px;
  margin: 0 auto;
}

.column {
  display: grid;
  gap: 1.4rem;
}

@media (max-width: 900px) {
  .layout {
    grid-template-columns: 1fr;
  }

  .page {
    padding: 1.6rem 1rem 4rem;
  }
}

/* ---------- Kort och inträde ---------- */
.card {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 1.1rem 1.2rem;
  opacity: 0;
  transform: translateY(8px);
  animation: rise 300ms var(--ease) both;
  animation-delay: calc(var(--i, 0) * 40ms);
}

@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ---------- Böcker ---------- */
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
  border-left: 2px solid transparent;
  color: inherit;
  padding: 0.5rem 0.6rem;
  border-radius: var(--radius);
  cursor: pointer;
  font: inherit;
  transition: background-color 120ms var(--ease), border-color 120ms var(--ease),
    transform 120ms var(--ease);
}

.books button:hover,
.concept:hover {
  background: var(--panel-2);
  border-left-color: var(--line-strong);
  transform: translateY(-1px);
}

.book.active {
  border-left-color: var(--accent);
  background: var(--panel-2);
}

.book-title {
  display: block;
  font-family: var(--serif);
  font-size: 1.02rem;
}

.book-meta {
  display: block;
  font-family: var(--mono);
  font-size: 0.7rem;
  color: var(--text-dim);
  margin-top: 0.1rem;
}

.book.active .book-title {
  color: var(--accent);
}

/* ---------- Koncept ---------- */
.hint {
  color: var(--text-dim);
  font-size: 0.86rem;
  margin: 0 0 0.7rem;
}

.hint code {
  font-family: var(--mono);
  font-size: 0.8rem;
  background: var(--panel-2);
  border: 1px solid var(--line);
  padding: 0.05rem 0.3rem;
  border-radius: 3px;
}

.concepts {
  max-height: 340px;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--line-strong) transparent;
}

.concept {
  position: relative;
  padding-right: 3rem;
}

.concept.active {
  border-left-color: var(--accent);
}

.concept.active .concept-name {
  color: var(--accent);
}

.concept-name {
  display: block;
  font-family: var(--serif);
  font-size: 0.96rem;
}

.concept-meta {
  position: absolute;
  top: 0.55rem;
  right: 0.6rem;
  font-family: var(--mono);
  font-size: 0.68rem;
  color: var(--text-dim);
}

.bar {
  display: block;
  height: 2px;
  margin-top: 5px;
  background: var(--line);
  overflow: hidden;
}

.bar > i {
  display: block;
  height: 100%;
  background: var(--accent);
  transform-origin: left;
  transform: scaleX(0);
  animation: grow 420ms var(--ease) both;
  animation-delay: calc(var(--i, 0) * 40ms + 120ms);
}

@keyframes grow {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}

/* ---------- Graf ---------- */
.graph {
  width: 100%;
  height: auto;
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: var(--radius);
}

.edge {
  stroke: var(--line-strong);
  stroke-width: 1.4;
  stroke-dasharray: 800;
  stroke-dashoffset: 800;
  animation: draw 520ms var(--ease) both;
  animation-delay: calc(var(--i, 0) * 25ms + 80ms);
}

@keyframes draw {
  to {
    stroke-dashoffset: 0;
  }
}

.edge.contrast {
  stroke: var(--accent);
}

.edge.cause,
.edge.effect {
  stroke: var(--sage);
}

.halo {
  fill: none;
  stroke: var(--accent);
  stroke-width: 1.5;
  transform-box: fill-box;
  transform-origin: center;
  animation: halo 3s ease-in-out infinite;
  pointer-events: none;
}

@keyframes halo {
  0% {
    transform: scale(0.7);
    opacity: 0.75;
  }
  70% {
    transform: scale(2);
    opacity: 0;
  }
  100% {
    transform: scale(2);
    opacity: 0;
  }
}

.node {
  fill: var(--sage);
  stroke: var(--line-strong);
  stroke-width: 1.5;
  cursor: pointer;
  transform-box: fill-box;
  transform-origin: center;
  transition: transform 120ms var(--ease), fill 120ms var(--ease);
}

.node:hover {
  transform: scale(1.12);
  fill: var(--accent-soft);
}

/* Starkaste noden markeras även utan rörelse, så stillbilden säger samma sak. */
.node.strongest {
  fill: var(--accent);
  stroke: var(--accent-soft);
}

.node.selected {
  fill: var(--accent-soft);
  stroke: var(--accent);
}

.label {
  fill: var(--text-dim);
  font-size: 10px;
  pointer-events: none;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.9rem;
  margin-top: 0.7rem;
  font-family: var(--mono);
  font-size: 0.7rem;
  color: var(--text-dim);
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.dot {
  width: 14px;
  height: 2px;
  background: var(--line-strong);
  display: inline-block;
}

.dot.contrast {
  background: var(--accent);
}

.dot.cause,
.dot.effect {
  background: var(--sage);
}

/* ---------- Analys mot kunskapsbasen ---------- */
.stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(96px, 1fr));
  gap: 1px;
  background: var(--line);
  border: 1px solid var(--line);
  margin-bottom: 0.8rem;
}

.stat {
  background: var(--panel);
  padding: 0.7rem 0.8rem;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.stat-value {
  font-family: var(--serif);
  font-size: 1.5rem;
  line-height: 1.1;
  color: var(--text);
}

.stat.warn .stat-value {
  color: var(--accent-soft);
}

.stat-label {
  font-family: var(--mono);
  font-size: 0.62rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--text-dim);
}

.stats.compact {
  grid-template-columns: repeat(3, 1fr);
}

.stats.compact .stat-value {
  font-size: 1.2rem;
}

.tabs {
  display: flex;
  gap: 0.3rem;
  border-bottom: 1px solid var(--line);
  margin-bottom: 0.9rem;
  flex-wrap: wrap;
}

.tabs button {
  background: none;
  border: 0;
  border-bottom: 1px solid transparent;
  color: var(--text-dim);
  font-family: var(--mono);
  font-size: 0.68rem;
  letter-spacing: 0.08em;
  padding: 0.45rem 0.7rem;
  margin-bottom: -1px;
  cursor: pointer;
  transition: color 120ms var(--ease), border-color 120ms var(--ease);
}

.tabs button:hover {
  color: var(--text);
}

.tabs button.active {
  color: var(--accent-soft);
  border-bottom-color: var(--accent);
}

.tabs button em {
  font-style: normal;
  color: var(--text-dim);
  margin-left: 0.35rem;
}

.insight-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.7rem;
}

.insight-list li {
  display: grid;
  gap: 0.2rem;
  padding-bottom: 0.7rem;
  border-bottom: 1px solid var(--line);
}

.insight-list li:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}

.insight-name {
  font-family: var(--serif);
  font-size: 1rem;
  color: var(--text);
}

.insight-name .verdict {
  font-family: var(--mono);
  font-size: 0.6rem;
  font-style: normal;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-dim);
  border: 1px solid var(--line-strong);
  padding: 0.1rem 0.35rem;
  margin-left: 0.4rem;
  vertical-align: middle;
}

.insight-name .verdict.contradiction {
  color: var(--accent-soft);
  border-color: rgba(201, 127, 90, 0.45);
}

.insight-name .verdict.unconventional {
  color: var(--sage);
  border-color: rgba(143, 174, 134, 0.4);
}

.insight-name .arrow {
  color: var(--accent-soft);
  margin: 0 0.3rem;
}

.insight-text {
  color: var(--text-dim);
  font-size: 0.86rem;
  overflow-wrap: anywhere;
}

.insight-here,
.insight-there {
  font-size: 0.82rem;
  color: var(--text-dim);
}

.insight-here::before,
.insight-there::before {
  font-family: var(--mono);
  font-size: 0.6rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--line-strong);
  margin-right: 0.4rem;
}

.insight-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.insight-tags i {
  font-family: var(--mono);
  font-style: normal;
  font-size: 0.62rem;
  letter-spacing: 0.06em;
  color: var(--sage);
  border: 1px solid rgba(143, 174, 134, 0.3);
  background: rgba(143, 174, 134, 0.07);
  padding: 0.1rem 0.4rem;
  border-radius: var(--radius);
}

/* ---------- Förklaring ---------- */
.definition {
  font-family: var(--serif);
  font-size: 1.08rem;
  line-height: 1.55;
  margin: 0 0 0.7rem;
}

.points {
  margin: 0;
  padding-left: 1.1rem;
  color: var(--text-dim);
  font-size: 0.92rem;
  line-height: 1.6;
}

.quote {
  border-left: 2px solid var(--accent);
  padding-left: 0.8rem;
  color: var(--text-dim);
  font-family: var(--serif);
  font-style: italic;
  margin: 0.9rem 0;
}

.related {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-top: 0.7rem;
}

.chip {
  font-size: 0.78rem;
  background: var(--panel-2);
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 0.2rem 0.7rem;
}

.chip em {
  color: var(--accent);
  font-style: normal;
  font-family: var(--mono);
  font-size: 0.72rem;
}

/* ---------- Fråga ---------- */
.ask {
  display: flex;
  gap: 0.5rem;
}

.ask input {
  flex: 1;
  min-width: 0;
  background: var(--panel-2);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  color: var(--text);
  padding: 0.55rem 0.7rem;
  font: inherit;
  transition: border-color 120ms var(--ease);
}

.ask input:focus {
  border-color: var(--accent);
}

button {
  font: inherit;
  cursor: pointer;
}

.ask button,
.primary {
  background: var(--accent);
  color: #16100c;
  border: none;
  border-radius: var(--radius);
  padding: 0.55rem 1.1rem;
  font-weight: 600;
  transition: transform 120ms var(--ease), background-color 120ms var(--ease);
}

.ask button:hover:not(:disabled),
.primary:hover:not(:disabled) {
  transform: translateY(-1px);
  background: var(--accent-soft);
}

.ask button:disabled,
.primary:disabled {
  opacity: 0.45;
  cursor: default;
}

.primary {
  font-size: 0.88rem;
}

.answer {
  white-space: pre-wrap;
  background: var(--bg);
  border: 1px solid var(--line);
  border-left: 2px solid var(--accent);
  border-radius: var(--radius);
  padding: 0.85rem 0.95rem;
  font-family: var(--serif);
  font-size: 0.98rem;
  line-height: 1.55;
  margin: 0.8rem 0 0;
}

/* ---------- Quiz ---------- */
.quiz {
  margin-top: 0.9rem;
  display: grid;
  gap: 0.9rem;
}

.question {
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 0.9rem 1rem;
}

.question-text {
  margin: 0 0 0.7rem;
  font-family: var(--serif);
  font-size: 1rem;
}

.options {
  margin: 0;
  padding-left: 1.2rem;
  display: grid;
  gap: 0.35rem;
}

.option {
  background: var(--panel-2);
  border: 1px solid var(--line);
  color: var(--text);
  border-radius: var(--radius);
  padding: 0.45rem 0.7rem;
  text-align: left;
  font: inherit;
  transition: transform 120ms var(--ease), border-color 120ms var(--ease),
    background-color 120ms var(--ease);
}

.option:hover {
  border-color: var(--accent);
  transform: translateY(-1px);
}

.result {
  margin: 0.7rem 0 0;
  font-size: 0.88rem;
  color: var(--accent-soft);
}

.result.correct {
  color: var(--sage);
}

/* ---------- Fokus ---------- */
:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

/* ---------- Reducerad rörelse ---------- */
@media (prefers-reduced-motion: reduce) {
  .card,
  .bar > i,
  .edge {
    animation-duration: 0.01ms !important;
    animation-delay: 0ms !important;
  }

  .halo {
    animation: none;
    opacity: 0.3;
  }

  .books button,
  .concept,
  .node,
  .option,
  .ask button,
  .primary {
    transition: none;
  }
}
</style>
