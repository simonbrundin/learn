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
│   │   │   ├── api/     # REST API (Gin)
│   │   │   ├── cli/     # Enkel counter-CLI/TUI (demo)
│   │   │   └── learn/   # Kommandoradsgränssnitt för kunskapssystemet
│   │   └── internal/
│   │       ├── epub/    # E-pub-parsning (zip/OPF/NCX)
│   │       ├── text/    # Textanalys: meningar, nyckelord, definitioner
│   │       ├── knowledge/ # Konceptextraktion + kunskapsgraf
│   │       ├── learning/  # Frågegenerator (SM-2) + förklaringar
│   │       ├── ideas/     # Idégenerering
│   │       ├── store/     # SQLite-lager (modernc.org/sqlite, CGO-fritt)
│   │       └── learn/     # Importpipeline
│   └── nuxt/            # Nuxt SPA – se AGENTS.md
└── environments/        # Kubernetes (kustomize) per miljö
```

## Teknisk Stack

- **Go 1.27+** — Kärnlogik och CLI
- **Standardbiblioteket + x/net/html** — E-pub-parsning och textextraktion
- **modernc.org/sqlite** — Lokal databas (`~/.learn/learn.db`), ren Go utan CGO
- **heuristisk NLP** — koncept, definitioner och citat upptäcks utan tunga modeller

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
- [ ] Kunskapsgraf med visualisering
- [ ] Web-GUI med Gradio
- [x] Spaced repetition (SM-2 algoritm)
- [ ] AI-förklaringar med lokal modell
- [ ] Moln sync (valfritt)
- [ ] Mobilt GUI

## Varfor?

Vi läser alla böcker, men hur mycket kommer vi ihåg? 

Detta system löser det fundamentala problemet: **kunskap som inte testas, glöms.** 

Genom att automatiskt generera tester och förklaringar, och genom att koppla ihop koncept över böcker, bygger du en verklig förståelse som stannar.

---

*"Den som läser utan att reflektera, är som den som äter utan att smaka."*
