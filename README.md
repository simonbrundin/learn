# Learn — Intelligent Kunskapssystem

> Ladda in en bok, gräv upp kunskapen, och spåna idéer som förändrar ditt tänkande.

## Vision

Ett inlärningssystem som tar emot e-pub-filer och extraherar den verkligt värdefulla kunskapen. Inte bara text, utan *begripliga koncept*, *testbara frågor*, och *idéverktyg* som hjälper dig förstå och tillämpa det du läst.

**Tänk:** Din personliga AI-studiekompis som har läst alla dina böcker, förstår sammanhangen, och kan hjälpa dig att:

- Förklara abstrakta koncept på din nivå
- Testa din förståelse med intelligenta frågor
- Koppla ihop kunskap från olika böcker
- Brainstorma nya idéer baserat på det du läst

## Kärnfunktioner

### 1. E-pub Import & Parsing
- Ladda upp valfri e-pub-fil
- Extrahera metadata, kapitel, och fulltext
- Behandla struktur (delar, kapitel, sektioner)
- Synka automatiskt från Calibre-bibliotek (se Calibre-synk)

### 2. Kunskapsextraktion
- Identifiera kärnkoncept och definitioner
- Hitta viktiga citat och tankeväckande stycken
- Extrahera argument och logiska samband
- Identifiera nyckeltermer och deras relationer

### 3. Interaktiv Förklaring
- Få koncept förklarade steg-för-steg
- Anpassa förklaringsnivå efter din kunskap
- Få analogier och exempel från verkligheten
- Ställ följdfrågor och få djupare svar

### 4. Kunskapstestning
- Auto-genererade frågor baserat på innehållet
- Flerval, kortfrågor, och essay-prompter
- Spaced repetition för långsiktigt minne
- Svårighetsgrad anpassad till din nivå

### 5. Idébank & Brainstorming
- Sök bland all inlärd kunskap
- Koppla ihop koncept från olika böcker
- Få idéprompts baserat på det du läst
- Bygg en personlig kunskapsgraf

## Arkitektur

```
learn/
├── src/
│   ├── go/              # Go – API, CLI/TUI och kunskapssystemet
│   │   ├── cmd/
│   │   │   ├── api/     # REST API (Gin) + OpenAPI/Scalar
│   │   │   ├── cli/     # Enkel counter-CLI/TUI (demo)
│   │   │   └── learn/   # Kommandoradsgränssnitt för kunskapssystemet
│   │   └── internal/
│   │       ├── api/     # HTTP-handlare för kunskapsbasen
│   │       ├── epub/    # E-pub-parsning (zip/OPF/NCX)
│   │       ├── text/    # Textanalys: meningar, nyckelord, definitioner
│   │       ├── knowledge/ # Konceptextraktion + kunskapsgraf
│   │       ├── learning/  # Frågegenerator (SM-2) + förklaringar
│   │       ├── ideas/     # Idégenerering
│   │       ├── store/     # SQLite-lager + repository-gränssnitt
│   │       └── learn/     # Importpipeline
│   └── nuxt/            # Nuxt SPA – se AGENTS.md
└── environments/        # Kubernetes (kustomize) per miljö
```

## Teknisk Stack

- **Go 1.27+** — Kärnlogik, CLI och REST-API
- **Standardbiblioteket + x/net/html** — E-pub-parsning och textextraktion
- **modernc.org/sqlite** — Lokal databas (`~/.learn/learn.db`), ren Go utan CGO
- **heuristisk NLP** — koncept, definitioner och citat upptäcks utan tunga modeller
- **Nuxt 4 + Vue 3** — SPA som visar grafen, koncept och quiz

## Kom igång

```bash
# Klona repo
git clone https://github.com/ditt-namn/learn.git
cd learn/src/go

# Bygg CLI
go build -o learn ./cmd/learn

# Importera en bok
./learn import min-bok.epub

# Starta interaktiv session
./learn interactive

# Generera frågor
./learn quiz --count 10

# Brainstorma
./learn brainstorm "Vad kan jag lära mig om X"
```

## Calibre-synk

Learn läser Calibres bibliotek direkt. `metadata.db` öppnas skrivskyddat,
så Calibre kan köra samtidigt. Endast e-puber importeras (böcker i andra
format hoppas över).

```bash
# Synka hela biblioteket (default: ~/calibre)
./learn sync

# Se vad som skulle hända, utan att skriva
./learn sync --dry-run

# Synka ett annat bibliotek
./learn sync --library ~/Böcker

# Kör om synken automatiskt var tionde minut
./learn sync --watch --interval 10m
```

| Flagga | Standard | Betydelse |
| ------ | -------- | --------- |
| `--library` | `$LEARN_CALIBRE_LIBRARY` eller `~/calibre` | Sökväg till biblioteket |
| `--watch` | av | Kör synken om med jämna mellanrum |
| `--interval` | `10m` | Intervall vid `--watch` |
| `--max-concepts` | 60 | Max antal koncept per bok |
| `--max-file-size` | 150 | Max filstorlek i MB (0 = ingen gräns) |
| `--limit` | 0 | Max antal böcker per körning (0 = alla) |
| `--refresh` | av | Importera om redan importerade böcker |
| `--dry-run` | av | Visa vad som skulle importeras |

Böcker som redan importerats hoppas över. En bok importeras om när filen
ändras, och en bok som inte gick att importera prövas inte igen förrän
filen ändras.

### Automatisering med systemd

Enheter ligger i `src/go/systemd/`:

```bash
# Bygg och installera CLI:t
cd src/go && go build -o ~/.local/bin/learn ./cmd/learn

# Installera enheterna
cp src/go/systemd/learn-sync.{service,timer} ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now learn-sync.timer

# Status och logg
systemctl --user list-timers learn-sync.timer
journalctl --user -u learn-sync.service -f
```

Timern kör `learn sync` var tionde minut och tar igen missade körningar
efter att maskinen startat.

## REST-API

Samma kunskapsbas kan läsas via HTTP. API:t och CLI:t delar SQLite-filen
(`~/.learn/learn.db`, override med `LEARN_DB`).

```bash
# Starta API:t
cd src/go && go run ./cmd/api

# Böcker och grafer
curl localhost:8080/books
curl localhost:8080/books/1/graph

# Fråga och förklara
curl "localhost:8080/ask?q=Vad%20är%20System%201?&bookId=1"
curl "localhost:8080/concepts/1/explain?level=novice"

# Quiz och spaced repetition
curl "localhost:8080/books/1/quiz?count=5"
curl -X POST localhost:8080/questions/1/attempt \
  -H 'Content-Type: application/json' -d '{"correct":true,"answer":"2"}'
```

Interaktiv OpenAPI-dokumentation finns på `http://localhost:8080/docs/`.

| Endpoint | Beskrivning |
| --- | --- |
| `POST /books/import` | Importera en e-pub från serverns filsystem |
| `GET /books` | Lista böcker |
| `GET /books/{id}` | Bok med kapitel och antal koncept/relationer/frågor |
| `GET /books/{id}/concepts?q=&limit=` | Koncept, filtrerbart |
| `GET /books/{id}/graph` | Noder och relationer för visualisering |
| `GET /books/{id}/quiz?count=&kinds=&regenerate=` | Frågor (genereras vid behov) |
| `GET /books/{id}/ideas?seed=&limit=` | Idégenerering |
| `GET /concepts/{id}/explain?level=` | Nivåanpassad förklaring |
| `GET /concepts/{id}/related?limit=` | Grannar i grafen |
| `GET /ask?q=&bookId=` | Svar på en fråga |
| `GET /connect?from=&to=&bookId=` | Kopplingsväg mellan koncept |
| `POST /questions/{id}/attempt` | Registrera svar och uppdatera SM-2-planen |

## Webbgränssnitt

Nuxt-SPA:t i `src/nuxt` visar böcker, koncept, kunskapsgrafen (SVG), frågor
och quiz. Det pratar med API:t via `/api/` – i produktion via nginx-proxy, i
utveckling via Nuxts dev-proxy. Gränssnittet följer designsystemet "A3 Folio"
(djup mark, terrakotta, serif) med CSS-native rörelse – se AGENTS.md.

```bash
# Terminal 1: API:t
cd src/go && go run ./cmd/api

# Terminal 2: frontend
cd src/nuxt && bun run dev
```

## Arbetsflöde

```
[E-pub] --> [Parse] --> [Extrahera] --> [Analysera]
                                    |
                                    v
[Lagra] <-- [Koncept] <-- [Grafer]
   |
   v
[Anvand]
   |
   +---> Quiz ---------> Testa forstaelse
   |
   +---> Explain ------> Fa forkaringar
   |
   +---> Connect ------> Lankningar mellan koncept
   |
   +---> Brainstorm ---> Generera nya ideer
```

## Exempel

### Importera och analysera
```bash
$ ./learn import "Thinking Fast and Slow.epub"
Importerar... Thinking Fast and Slow.epub
Inläst: Thinking Fast and Slow (Daniel Kahneman)
Hittade 500+ koncept
Sparat i /home/user/.learn/learn.db
```

### Ställ frågor
```bash
$ ./learn ask "Vad handlar System 1 och System 2 om?"
System 1 — the fast, automatic and emotional thinking that runs without effort
  Kopplar till: System 2, Availability heuristic
System 2 — the slow, deliberate and analytical thinking that requires attention
  Kopplar till: System 1, Loss aversion
```

### Testa kunskap
```bash
$ ./learn quiz --count 10
Kunskapstest: Thinking Fast and Slow

1. Vad betyder System 1?
   1) the slow, deliberate and analytical thinking that requires attention
   2) the fast, automatic and emotional thinking that runs without effort
   3) the scarce resource that System 2 needs
   Ditt svar (1-3, eller skriv 'skip'): 2
   Rätt!

2. Stämmer detta på Loss aversion? "..." 
   ...

Resultat: 8/10 rätt (80%)
```

## Framtidsplan

- [x] E-pub import med felhantering
- [x] Konceptextraktion (heuristisk, utan NLP-beroende)
- [x] Frågegenerator
- [x] Kunskapsgraf med kopplingar och sökvägar
- [x] Kunskapsgraf med visualisering (SVG i Nuxt-SPA:t)
- [x] REST-API med OpenAPI/Scalar-dokumentation
- [x] Web-GUI (Nuxt SPA)
- [x] Spaced repetition (SM-2 algoritm)
- [ ] Kunskapsbas i PostgreSQL (K8s) i stället för lokal SQLite
- [ ] AI-förklaringar med lokal modell
- [ ] Moln sync (valfritt)
- [ ] Mobilt GUI

## Varfor?

Vi läser alla böcker, men hur mycket kommer vi ihåg? 

Detta system löser det fundamentala problemet: **kunskap som inte testas, glöms.** 

Genom att automatiskt generera tester och förklaringar, och genom att koppla ihop koncept över böcker, bygger du en verklig förståelse som stannar.

---

*"Den som läser utan att reflektera, är som den som äter utan att smaka."*
