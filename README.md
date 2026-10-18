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
├── importer/           # E-pub parsing och extraktion
│   ├── epub_parser.py
│   └── text_processor.py
├── knowledge/          # Kunskapsrepresentation
│   ├── concepts.py     # Konceptmodellering
│   └── graph.py        # Kunskapsgraf
├── learning/          # Inlärningsmotorer
│   ├── quiz_engine.py  # Frågegenerering
│   └── explanation.py  # Forklaringssystem
├── ideas/             # Brainstorming
│   └── brainstorm.py  # Idégenerering
├── cli/               # Kommandoradsgranssnitt
└── web/               # Webbgranssnitt (framtid)
```

## Teknisk Stack

- **Python 3.11+** — Kärnlogik
- **python-epub** — E-pub parsing
- **Spacy** — NLP och textanalys
- **NetworkX** — Kunskapsgraf
- **SQLite** — Lokal databas
- **Gradio/Streamlit** — Webb-GUI (framtid)

## Kom igång

```bash
# Klona repo
git clone https://github.com/ditt-namn/learn.git
cd learn

# Installera beroenden
pip install -r requirements.txt

# Importera en bok
python -m learn.cli import min-bok.epub

# Starta interaktiv session
python -m learn.cli interactive

# Generera frågor
python -m learn.cli quiz "min-bok"

# Brainstorma
python -m learn.cli brainstorm "Vad kan jag lara mig om X"
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
$ python -m learn.cli import "Thinking Fast and Slow.epub"
Importing... Thinking Fast and Slow (Daniel Kahneman)
Found 500+ concepts extracted
Saved to ~/.learn/books/kahneman-fast-slow/
```

### Ställ frågor
```bash
$ python -m learn.cli ask "Vad handlar System 1 och System 2 om?"
System 1 and System 2 are Kahneman's framework for two modes of thinking:

  SYSTEM 1 (Fast thinking)
  - Automatic, intuitive, emotional
  - Operates effortlessly
  - Examples: Detecting hostility, completing "bread and ..."

  SYSTEM 2 (Slow thinking)
  - Deliberate, logical, effortful
  - Requires attention
  - Examples: Counting, learning new skills

Kopplade koncept: Availability heuristic, Anchoring, Loss aversion
```

### Testa kunskap
```bash
$ python -m learn.cli quiz --count 10
Kunskapstest: Thinking Fast and Slow

1. Vilket system aktiveras nar du fattar snabba beslut?
   a) System 1   b) System 2   c) Bada   d) Inget

2. Vad ar en "mental short-cut" kallad i boken?
   ...

Resultat: 8/10 ratt (80%) - Bra jobbat!
```

## Framtidsplan

- [ ] E-pub import med felhantering
- [ ] Konceptextraktion med NLP
- [ ] Frågegenerator
- [ ] Kunskapsgraf med visalisering
- [ ] Web-GUI med Gradio
- [ ] Spaced repetition (SM-2 algoritm)
- [ ] AI-förklaringar med lokal modell
- [ ] Moln sync (valfritt)
- [ ] Mobilt GUI

## Varfor?

Vi läser alla böcker, men hur mycket kommer vi ihåg? 

Detta system löser det fundamentala problemet: **kunskap som inte testas, glöms.** 

Genom att automatiskt generera tester och förklaringar, och genom att koppla ihop koncept över böcker, bygger du en verklig förståelse som stannar.

---

*"Den som läser utan att reflektera, är som den som äter utan att smaka."*
