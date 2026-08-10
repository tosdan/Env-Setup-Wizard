# Piano di implementazione — Env Setup Wizard

## 1. Obiettivo

Realizzare `env-wizard`, una CLI Go cross-platform che generi e aggiorni un file `.env` a partire da `.env.example`, guidando l'utente tramite un wizard interattivo da terminale.

Il progetto deve:

- usare `.env.example` come source of truth;
- mantenere il template un normale file dotenv leggibile anche senza conoscere il wizard;
- supportare annotation opzionali nei commenti per descrivere l'esperienza del wizard;
- funzionare su Windows e Linux senza script separati;
- permettere il rerun usando i valori già presenti in `.env`;
- validare il template prima di avviare l'interfaccia;
- non esporre valori marcati come secret nei log, negli errori o nel riepilogo;
- scrivere il file di output in modo sicuro, senza lasciare un file parziale.

La v1 deve rimanere volutamente piccola: niente subcommand e niente Cobra. L'interfaccia CLI iniziale sarà basata sulla standard library `flag`.

---

## 2. Ambito della v1

### 2.1 Annotation supportate

Il vocabolario della prima release è congelato alle seguenti nove annotation:

| Annotation                           | Scopo                                                                                           |
| ------------------------------------ | ----------------------------------------------------------------------------------------------- |
| `@prompt valore`                     | Personalizza il testo della domanda. Fallback: nome della variabile.                            |
| `@description valore`                | Aggiunge una descrizione al campo.                                                              |
| `@required`                          | Rifiuta valori vuoti o composti solo da whitespace.                                             |
| `@secret`                            | Usa input mascherato (`EchoModePassword`) e nasconde il valore nei summary e nella diagnostica. |
| `@type string\|int\|bool\|port\|url` | Seleziona semantica, widget e validazione. `string` è il default.                               |
| `@options v1,v2,...`                 | Trasforma il campo in una selezione chiusa. Il default deve appartenere alle option.            |
| `@placeholder valore`                | Mostra un suggerimento senza trasformarlo nel valore di default.                                |
| `@fixed`                             | Scrive il valore del template senza mostrarlo nel wizard e senza permetterne la modifica.       |
| `@section valore`                    | Apre o cambia il gruppo/pagina del wizard. Senza section si usa `Configuration`.                |

Le annotation v2 (`@suggest`, `@multiple`, `@pattern`, `@when`, `@generate`) non fanno parte della v1. Il modello interno deve però evitare decisioni che ne impediscano una futura estensione.

La sintassi lessicale delle annotation v1 è:

```dotenv
# @annotation valore
```

Regole:

- sono ammessi spazi orizzontali prima di `#` e uno o più spazi orizzontali tra `#` e `@`;
- il nome dell'annotation è lowercase e case-sensitive;
- il valore è il testo restante dopo il nome, con whitespace iniziale e finale rimosso;
- `#@required` è un commento normale, non un'annotation;
- `# @unknown ...` è un errore di template, non un commento normale;
- `@required`, `@secret` e `@fixed` non accettano un valore;
- `@prompt`, `@description`, `@placeholder`, `@options` e `@section` richiedono un valore non vuoto;
- `@options` divide il valore sulla virgola, rifila gli elementi, rifiuta elementi vuoti e duplicati e confronta le option in modo case-sensitive;
- una stessa annotation non può comparire due volte nello stesso blocco di campo;
- le annotation di campo devono essere seguite immediatamente da una variabile, senza righe vuote o commenti normali intermedi;
- `@section` aggiorna il contesto del documento e può restare senza una variabile successiva.

Le annotation sono metadati del wizard e vengono rimosse dal file `.env` generato. I commenti che non seguono la sintassi `# @...` vengono invece conservati.

### 2.2 Regole di default

Senza annotation, ogni variabile configurabile deve:

- diventare una domanda;
- usare il nome della chiave come prompt;
- usare il valore del template come default;
- considerare il valore vuoto come assenza di default;
- permettere all'utente di confermare il default semplicemente proseguendo.

Le annotation modificano questo comportamento solo quando esplicitamente presenti. Non devono essere introdotte annotation ridondanti come `@default`, `@optional`, `@string` o `@editable`.

Per distinguere un valore realmente vuoto da un valore non ancora impostato, il modello non deve affidarsi alla sola stringa `Value`. Ogni valore/question deve esporre almeno uno stato equivalente a `HasValue` oppure `ValueSource`. Le sorgenti previste sono `template`, `existing`, `user` e `fixed`.

Per `@type bool` la v1 accetta esclusivamente `true` e `false`, senza distinzione tra maiuscole e minuscole, e il writer serializza sempre `true` oppure `false`. Il template deve contenere un default booleano non vuoto; un `.env` esistente può sostituirlo solo con un altro valore booleano valido. Un booleano vuoto nel template o nel file esistente è un errore prima della creazione della domanda.

### 2.3 Sintassi del template

Il template v1 è un subset intenzionale del formato accettato da Docker Compose:

```dotenv
KEY=
KEY=value
KEY="value"
KEY='value'
```

Sono supportati anche commenti, annotation e righe vuote. Nella v1 il template deve rifiutare esplicitamente, con un errore chiaro:

- delimitatore `:` al posto di `=`;
- prefisso `export`;
- valori multilinea;
- commenti inline non quotati sulla stessa riga della variabile; un `#` dentro un valore quoted è parte del valore;
- variabili duplicate.

Le chiavi duplicate sono un errore anche nel `.env` esistente, perché il merge deve avere una sorgente univoca. Il file esistente viene quindi sottoposto a un controllo strutturale minimo delle chiavi, oltre al parsing semantico con `compose-go`.

Il subset v1 usa la grammatica di chiave `[A-Za-z_][A-Za-z0-9_.-]*`. Le chiavi sono case-sensitive; `FOO` e `foo` sono quindi variabili distinte. Non sono supportate chiavi che iniziano con un numero, parentesi quadre o altre forme estese accettate da Compose.

Il template v1 non supporta riferimenti di interpolazione non escapati come `$VAR` e `${VAR}`. In questo modo i default del wizard sono deterministici e non dipendono dall'ambiente del processo. `$$` è ammesso esclusivamente come rappresentazione esplicita di un `$` letterale e deve essere coperto dai test dell'encoder; non è consentita l'interpolazione implicita nel valore generato.

Il controllo delle forme vietate è lessicale e non costituisce una seconda implementazione della semantica dotenv: lo scanner può identificare delimitatori, prefissi, commenti inline non quotati e riferimenti `$` vietati, ma non interpreta la semantica di quoting, escaping o valori dopo il primo `=`. Per distinguere un commento finale da un `#` contenuto in un valore quoted può eseguire soltanto la scansione minima necessaria.

Un BOM UTF-8 iniziale viene normalizzato prima del parsing strutturale e semantico. Il writer v1 produce UTF-8 senza BOM. UTF-16, UTF-8 non valido e caratteri di newline `CR` isolati non fanno parte del formato supportato.

Sono supportati file LF e CRLF, anche senza newline finale. Gli EOL misti vengono rifiutati nella v1 per mantenere il comportamento deterministico; il separatore e l'eventuale newline finale del documento valido vengono preservati nel rendering.

La semantica dei valori dotenv non deve essere reimplementata: sarà delegata a `github.com/compose-spec/compose-go/v2/dotenv`.

---

## 3. Decisioni architetturali

### 3.1 Separare semantica dotenv e struttura del documento

Il progetto userà due passaggi distinti:

```text
.env.example
  ├── compose-go/v2/dotenv → valori interpretati
  └── structural scanner    → ordine, commenti, annotation, variabili
                                ↓
                            Document
```

`compose-go/v2/dotenv` deve essere usato sia per il template sia per un eventuale `.env` già esistente. Il wrapper interno deve passare un lookup controllato e deterministico, non `os.LookupEnv` implicitamente. Per entrambi i file il lookup esterno iniziale è vuoto: il parser può usare soltanto le variabili già definite nello stesso file secondo la propria semantica, e il risultato non cambia in base all'ambiente del processo.

Il nostro scanner deve riconoscere soltanto:

- commenti;
- righe vuote;
- righe di variabile;
- sintassi del subset non supportato.

Lo scanner non deve interpretare quote, escape o il significato dei valori dopo il primo `=`. Può tuttavia rifiutare lessicalmente le forme escluse dal subset v1, come commenti inline non quotati, interpolazione `$` non escapata, delimitatore `:` e valori multilinea. Per distinguere un commento finale da un `#` contenuto in un valore quoted può eseguire soltanto la scansione minima necessaria.

### 3.2 Modello a nodi ordinati

Non usare `map[string]string` come rappresentazione del documento. Il modello deve preservare l'ordine e la struttura originale:

```go
type Document struct {
  Nodes []Node
  LineEnding string
  HasFinalNewline bool
}

type Variable struct {
  Key         string
  Value       string
  HasValue    bool
  ValueSource ValueSource
  RawLine     string
  Line        int
  Annotations Annotations
}

type ValueSource string

const (
      ValueFromTemplate ValueSource = "template"
      ValueFromExisting ValueSource = "existing"
      ValueFromUser     ValueSource = "user"
      ValueFromFixed    ValueSource = "fixed"
)
```

Il modello deve includere nodi per commenti, righe vuote, annotation e variabili. `LineEnding` e `HasFinalNewline` descrivono il formato globale del documento valido; gli EOL misti vengono rifiutati. `HasValue` indica che la variabile possiede un'assegnazione, anche quando il valore è la stringa vuota; non equivale quindi a "valore non vuoto". I valori semantici delle variabili sono associati per chiave ai risultati di `compose-go`.

Le annotation consecutive si applicano alla prima variabile immediatamente successiva. Una riga vuota o un commento normale interrompono il blocco di annotation. Una nuova `@section` aggiorna il contesto documentale e non viene associata alla variabile successiva come metadato di campo. Annotation duplicate, sconosciute o prive del valore richiesto sono errori di template. Una annotation di campo senza variabile immediatamente successiva è un errore.

### 3.3 Dipendenze ai bordi

Il dominio non deve importare Huh, compose-go o primitive OS-specifiche.

```text
                DOMAIN
                  │
        ┌─────────┼──────────┐
        ▼         ▼          ▼
 compose-go      Huh      filesystem
  adapter       adapter      adapter
```

Dipendenze runtime previste:

- `charm.land/huh/v2` per il wizard;
- `github.com/compose-spec/compose-go/v2/dotenv` per la semantica dotenv;
- `golang.org/x/sys/windows` soltanto se necessarie primitive Windows dedicate;
- standard library Go per CLI, validazione, filesystem, errori, logging e crittografia futura.

Ogni nuova dipendenza deve essere valutata per maturità, licenza, dipendenze transitive, possibilità di usare la standard library e possibilità di isolamento dietro un adapter.

### 3.4 Writer canonico

Il writer non deve cercare di preservare lo stile originale di quoting quando un valore viene modificato. Deve usare un encoding canonico Compose-compatible e verificarlo tramite round-trip con `compose-go`:

```text
value
  ↓ EncodeValue
KEY=<encoded>
  ↓ dotenv.Parse
value originale
```

Per ogni valore testato deve risultare `original == parsed`. Particolare attenzione va riservata a stringhe vuote, whitespace, `#`, `$`, quote, apostrofi, backslash, newline, Unicode e `=`. I valori booleani vengono serializzati sempre come `true` o `false`.

### 3.5 Scrittura sicura

Il file finale non deve essere scritto direttamente. La pipeline deve essere:

```text
render completo
    ↓
file temporaneo nella stessa directory
    ↓
Write + Sync + Close
    ↓
replace del target
```

Il file precedente deve rimanere intatto se un'operazione fallisce prima del replace. La sostituzione va isolata dietro `replaceFile`, con implementazioni file-specifiche per Unix e Windows. La v1 privilegia l'integrità del contenuto e userà un temporaneo nella stessa directory; su Windows si può usare `MoveFileExW` tramite `golang.org/x/sys/windows`, mentre `ReplaceFileW` sarà necessario soltanto se il requisito futuro includerà la preservazione di ACL e metadata del target esistente. L'atomicità e la durabilità dipendono comunque dal filesystem e non sono garantite in modo assoluto su share di rete o filesystem particolari.

---

## 4. Struttura del repository prevista

```text
env-setup-wizard/
├── cmd/
│   └── env-wizard/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go
│   ├── domain/
│   │   ├── document.go
│   │   ├── annotations.go
│   │   └── question.go
│   ├── dotenv/
│   │   ├── semantic.go
│   │   ├── structure.go
│   │   ├── annotations.go
│   │   ├── encoder.go
│   │   └── merge.go
│   ├── validation/
│   │   └── validation.go
│   ├── wizard/
│   │   ├── wizard.go
│   │   ├── huh_adapter.go
│   │   ├── field_factory.go
│   │   └── summary.go
│   └── filesystem/
│       ├── writer.go
│       ├── replace_unix.go
│       └── replace_windows.go
├── testdata/
│   ├── basic.env.example
│   ├── comments.env.example
│   ├── annotations.env.example
│   ├── quoting.env.example
│   ├── crlf.env.example
│   └── invalid/
├── .github/
│   └── workflows/
│       └── ci.yml
├── go.mod
├── go.sum
├── LICENSE
├── THIRD_PARTY_NOTICES
├── SPEC.md
├── CONTRIBUTING.md
├── SECURITY.md
├── README.md
└── PLAN.md
```

I package devono avere responsabilità nette:

- `domain`: modello senza dipendenze UI, dotenv o filesystem;
- `dotenv`: scanner strutturale, adapter semantico, annotation, merge ed encoder;
- `validation`: validator puri e riutilizzabili;
- `wizard`: conversione `Question → huh.Field`, gruppi, esecuzione e summary;
- `filesystem`: rendering e sostituzione sicura dei file;
- `app`: orchestration del workflow;
- `cmd`: parsing minimo degli argomenti e avvio dell'application layer.

---

## 5. Contratti funzionali

### 5.1 CLI

Comando base:

```text
env-wizard
```

Default:

```text
template = .env.example
output   = .env
```

Flag v1:

```text
--template PATH
--output PATH
--force
--version
```

`--force` evita soltanto la conferma di overwrite; non deve bypassare la validazione del template o le validazioni dei valori.

Il path del template e quello dell'output vengono normalizzati e confrontati prima del wizard. Se identificano lo stesso file, il comando termina con errore e non modifica il template. Il confronto deve tenere conto delle regole del filesystem corrente, inclusa la case-insensitivity su Windows quando applicabile.

La v1 è interattiva e richiede un terminale. Se stdin o il terminale necessario non sono disponibili, il comando termina prima di creare o modificare l'output con un messaggio chiaro su stderr. `--force` non abilita una modalità non interattiva.

Exit code minimi:

```text
0    successo
non-zero    argomenti invalidi, template invalido, annullamento o errore operativo
```

La distinzione tra annullamento e gli altri errori può essere resa più precisa in una release successiva; l'application layer non deve usare `log.Fatal`, così i codici e i messaggi restano testabili.

### 5.2 Workflow applicativo

L'application layer deve orchestrare esattamente questi passaggi:

```text
parse arguments
      ↓
load template
      ↓
parse semantic dotenv values
      ↓
scan structural document
      ↓
bind annotations and values
      ↓
validate complete template
      ↓
load existing .env, if present
      ↓
merge defaults
      ↓
create questions
      ↓
run wizard
      ↓
show summary
      ↓
confirm overwrite
      ↓
render output
      ↓
atomic write
```

La validazione completa del template deve avvenire prima di mostrare il wizard. Un template errato è un errore dello sviluppatore e deve terminare l'esecuzione con numero di riga e causa leggibile.

### 5.3 Precedenza dei valori

Per ogni variabile configurabile vale:

```text
risposta del wizard
        ↓
valore da .env esistente
        ↓
valore da .env.example
```

`@fixed` è un'eccezione: il valore deve provenire sempre dal template e non deve essere sovrascritto dal `.env` esistente o dall'utente.

Il merge deve conservare anche la presenza del valore, non solo la stringa. Per un secret già presente nel file esistente, il valore reale viene caricato internamente ma non viene mai stampato. Invio senza modifica mantiene il valore esistente; una cancellazione esplicita produce una stringa vuota e può fallire con `@required`. Nel summary un secret valorizzato appare come `[set]`, mentre un secret vuoto appare come `[not set]`.

Se una variabile con `@options` riceve dal `.env` esistente un valore non presente nell'elenco, il comando termina con errore prima del wizard: il valore corrente non può essere selezionato dal widget vincolato.

Le variabili presenti solo nel vecchio `.env` non devono essere copiate nel nuovo output, perché `.env.example` rimane la source of truth. Le variabili nuove presenti solo nel template devono essere incluse e richieste secondo le annotation.

### 5.4 Domande e mapping Huh

Il modello intermedio deve essere indipendente da Huh:

```go
type Question struct {
      Key         string
      Prompt      string
      Description string
      Value       string
      HasValue    bool
      ValueSource ValueSource
      Kind        QuestionKind
      Required    bool
      Secret      bool
      Options     []string
      Placeholder string
      Section     string
}
```

Quando un secret proveniente da `.env` esistente viene assegnato a una `Question`, il valore reale resta nel modello per il merge e per la scrittura, ma l'adapter Huh deve presentarlo mascherato. Il contratto "Invio senza modifica conserva il valore" deve essere implementato confrontando lo stato iniziale e quello restituito dal campo, non interpretando il carattere di mascheramento come contenuto.

Mapping previsto:

```text
tipo string/default      → Input
tipo string + secret      → Input con `EchoModePassword`
tipo int/port/url         → Input + validator
tipo bool                 → Confirm
options                   → Select
```

La v1 non supporta `@multiple`; il mapping a `MultiSelect` resta backlog. `@fixed` viene esclusa dalla lista delle domande ma rimane nell'output.

Combinazioni v1:

| Combinazione                                                    | Regola                                                                               |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `@fixed` + `@prompt` o `@placeholder`                           | Errore: il campo non è mostrato.                                                     |
| `@fixed` + `@required`                                          | Valida: il valore del template deve essere non vuoto.                                |
| `@fixed` + `@secret`                                            | Valida: il valore resta sempre secret.                                               |
| `@options` senza `@type`                                        | Valida: tipo implicito `string`.                                                     |
| `@options` + `@type bool`                                       | Errore nella v1: `@options` implica `Select`, mentre `@type bool` implica `Confirm`. |
| `@options` + `@type int\|port\|url`                             | Errore nella v1 per evitare semantica ambigua.                                       |
| `@options` + `@secret`                                          | Errore nella v1: la selezione chiusa non è appropriata per un secret.                |
| `@placeholder` + `@options` o `@type bool`                      | Errore nella v1: il widget non è un input testuale.                                  |
| `@placeholder` con valore già presente                          | Valida per un input testuale: il placeholder resta solo un suggerimento visivo.      |
| `@type bool` con valore vuoto nel template o nel file esistente | Errore prima del wizard.                                                             |
| option contenente virgole                                       | Non supportata: la virgola è il solo separatore della v1.                            |

La validazione di `@required` viene applicata al valore finale dopo il merge e dopo la risposta dell'utente. `@fixed` usa sempre il valore del template. `@secret` può essere combinata con `@fixed`, ma non con `@options`.

### 5.5 Summary e secret handling

Il riepilogo deve essere raggruppato per section e deve mostrare i valori normali. Per i secret deve mostrare esclusivamente:

```text
DB_PASSWORD    [set]
```

Un secret vuoto viene mostrato come `[not set]`. Non deve mai essere rivelato il contenuto né la lunghezza del valore. La stessa policy vale per log, errori, dump di debug e messaggi di cancellazione.

`@secret` protegge soltanto l'esperienza del wizard: il valore viene comunque scritto nel file `.env`. Il progetto deve documentare che `.env` non è un secret store e raccomandare Docker Secrets o un sistema equivalente per credenziali particolarmente sensibili.

Una cancellazione tramite Ctrl+C o un rifiuto alla conferma finale non deve creare o modificare il file di output.

### 5.6 Writer e annotation

Il writer deve:

- aggiornare soltanto le variabili del documento;
- mantenere ordine, commenti normali, righe vuote e line ending;
- rimuovere dall'output le annotation del wizard;
- non copiare variabili obsolete del vecchio `.env`;
- usare l'encoder canonico per i valori modificati;
- produrre un file che `compose-go/v2/dotenv` riesca a rileggere con gli stessi valori.

Sono distinti tre contratti di rendering:

1. `parse → render` senza trasformazioni può essere byte-identico al template valido;
2. `generate .env` rimuove le annotation e può usare l'encoding canonico per le variabili aggiornate;
3. il risultato finale deve essere semanticamente rileggibile con `compose-go`, anche quando non è byte-identico al template.

---

## 6. Piano per fasi

### Fase 0 — Specifica e fondazioni

**Obiettivo:** congelare i contratti prima della UI.

Attività:

1. Creare il modulo Go e il comando `cmd/env-wizard`.
2. Creare `SPEC.md` e documentarvi formato template, annotation, precedenza, output e policy secret.
3. Decidere la licenza del progetto, preferibilmente Apache-2.0, oppure MIT previa scelta esplicita.
4. Definire le dipendenze dirette e la dependency policy.
5. Preparare la struttura `internal/` e le fixture iniziali.
6. Eseguire uno spike usa-e-getta di Huh v2 con `Input`, `EchoModePassword`, `Select`, `Confirm`, `Group`, `Placeholder`, `Validate`, `huh.ErrUserAborted` e un caso non-TTY. Lo spike deve compilare su Windows e Linux e non entra nel dominio del progetto.

**Criteri di completamento:**

- `go.mod` presente;
- architettura e subset del formato documentati;
- nessuna dipendenza UI nel dominio;
- comando che compila anche se il wizard non è ancora implementato.

### Fase 1 — CLI skeleton e pipeline minima

**Obiettivo:** caricare e renderizzare il template senza Huh.

Attività:

1. Implementare `--template`, `--output`, `--force`, `--version` con `flag`.
2. Risolvere i path con `filepath`, mai tramite concatenazione manuale.
3. Caricare il template preservando contenuto e line ending.
4. Implementare la pipeline provvisoria `load → structural scan → render` senza promettere ancora il parsing completo delle annotation.
5. Normalizzare BOM ed EOL secondo il contratto v1.
6. Rifiutare `--output` uguale a `--template` prima del wizard.
7. Rilevare il non-TTY prima di avviare l'interazione.
8. Attivare una CI minima con `go test ./...`, `go vet ./...` e build su Linux e Windows.
9. Restituire errori con contesto (`parse template: %w`) senza leak di secret.

**Milestone:**

```text
.env.example → parse → render
```

restituisce lo stesso documento quando non sono stati modificati valori.

### Fase 2 — Modello documento e structural scanner

**Obiettivo:** rappresentare il template senza reimplementare la semantica dotenv.

Attività:

1. Definire `Document`, `Node`, `Comment`, `BlankLine`, `AnnotationLine`, `Variable`, `ValueSource`.
2. Riconoscere righe LF e CRLF, rifiutare EOL misti e preservare il line ending del documento valido.
3. Riconoscere `KEY=`, `KEY=value`, `KEY="value"`, `KEY='value'` senza interpretare il valore.
4. Gestire `=` presenti nel valore lasciando il contenuto al parser Compose.
5. Validare le chiavi con `[A-Za-z_][A-Za-z0-9_.-]*` e rifiutare variabili duplicate indicando linea corrente e precedente.
6. Normalizzare il BOM prima dello scanner e rifiutare il subset non supportato, compresi interpolazione `$` non escapata, commenti inline non quotati e valori multilinea.
7. Associare i valori semantici di `compose-go` alle variabili tramite chiave usando un lookup controllato.

**Criteri di completamento:**

- ordine e struttura preservati;
- duplicate key rifiutate;
- scanner privo di logica di unescaping/interpolazione;
- round-trip strutturale coperto da test.

### Fase 3 — Annotation parser e validazione del template

**Obiettivo:** trasformare i commenti `@...` in metadati validati.

Attività:

1. Parsare le nove annotation v1.
2. Applicare le annotation consecutive alla prima variabile immediatamente successiva.
3. Interrompere il blocco su riga vuota o commento normale; una annotation di campo deve essere seguita immediatamente da una variabile.
4. Gestire `@section` come contesto corrente del documento.
5. Rifiutare annotation sconosciute, valori mancanti e duplicati incompatibili.
6. Validare `@type` (`string`, `int`, `bool`, `port`, `url`).
7. Verificare che il default di `@options` appartenga alle option.
8. Verificare la matrice delle combinazioni incompatibili prima dell'avvio del wizard.
9. Validare i valori bool e imporre un valore risolto per `@type bool`.
10. Includere sempre il numero di riga negli errori del template.

**Criteri di completamento:**

- `ValidateDocument` completamente testabile senza terminale;
- nessun errore di configurazione arriva alla UI;
- `@fixed` e `@section` rispettano la semantica descritta.

### Fase 4 — Encoder e writer con round-trip

**Obiettivo:** generare valori Compose-compatible e preservare il documento.

Attività:

1. Definire `EncodeValue` con una rappresentazione canonica.
2. Coprire caratteri speciali, `$`, quote, backslash, newline, Unicode e stringhe vuote; rifiutare interpolazioni non consentite.
3. Validare ogni encoding riparsando il risultato con `compose-go`.
4. Implementare aggiornamento delle variabili nel modello.
5. Rimuovere le annotation dall'output mantenendo commenti normali.
6. Preservare ordine, righe vuote, section header e line ending.
7. Implementare golden test e table-driven test del writer.

**Milestone:**

```text
parse → update values → remove annotations → render
```

produce un `.env` valido e rileggibile con i valori attesi.

### Fase 5 — Validation package e Question model

**Obiettivo:** separare le regole di validazione dal terminale e costruire il modello UI.

Attività:

1. Implementare validator puri: `Required`, `Integer`, `Boolean`, `Port`, `URL`.
2. Usare `strconv`, `net/url` e controlli di range della standard library.
3. Accettare per bool solo `true` e `false` ignorando il case, rifiutare il valore vuoto e serializzare sempre in lowercase.
4. Definire `Question` e `QuestionKind` nel dominio, includendo `HasValue` e `ValueSource`.
5. Convertire `Variable` in `Question` risolvendo prompt, descrizione, default, placeholder, tipo, required, secret e section.
6. Raggruppare le domande per section.
7. Escludere `@fixed`.

**Criteri di completamento:**

- validator riutilizzabili da UI, test e futura modalità non interattiva;
- conversione `Variable → Question` priva di dipendenze Huh;
- default e precedenza già risolti prima dell'adapter UI.

### Fase 6 — Integrazione Huh

**Obiettivo:** aggiungere il wizard interattivo mantenendo Huh confinato all'adapter.

Attività:

1. Implementare l'adapter `Question → huh.Field`.
2. Collegare `Input`, input password, `Select` e `Confirm`.
3. Collegare i validator ai campi Huh.
4. Usare `Group` per ogni section.
5. Supportare `@placeholder` come suggerimento visivo (`@placeholder` non è un default); non implementare suggestions o `MultiSelect` nella v1.
6. Gestire Ctrl+C e cancellazione senza scrittura.
7. Verificare il comportamento su terminali Windows e Linux.

**Criteri di completamento:**

- il dominio non importa Huh;
- un valore non valido mantiene l'utente nel campo corrente;
- il wizard non parte se il template è invalido.

### Fase 7 — Rerun, summary e conferma

**Obiettivo:** rendere il tool utile anche per riconfigurare un'installazione.

Attività:

1. Rilevare il file `.env` esistente nel path di output effettivo; se il target non esiste, il rerun parte dai valori del template.
2. Parsarlo con `compose-go/v2/dotenv` usando un lookup controllato.
3. Applicare la precedenza `.env esistente → template`, con eccezione `@fixed`, conservando `HasValue` e `ValueSource`.
4. Non mostrare secret esistenti durante il wizard o nel summary; precompilare il valore reale nel campo con `EchoModePassword` e fare in modo che Invio senza modifica lo conservi.
5. Gestire una cancellazione esplicita come valore vuoto e validarla con `@required`.
6. Gestire variabili nuove e obsolete secondo la source of truth del template.
7. Mostrare `[set]` o `[not set]` per i secret nel riepilogo.
8. Chiedere conferma di overwrite quando necessario; `--force` evita solo questa conferma.

**Criteri di completamento:**

- un rerun riparte dai valori correnti;
- i valori `@fixed` restano quelli del template;
- il cancel non modifica il file esistente;
- nessun secret viene stampato.

### Fase 8 — Filesystem e sostituzione atomica

**Obiettivo:** rendere affidabile la scrittura su Windows e Unix.

Attività:

1. Renderizzare il contenuto completamente in memoria.
2. Creare il temporaneo nella stessa directory del target.
3. Scrivere, sincronizzare e chiudere il temporaneo.
4. Sostituire il target tramite `replaceFile`.
5. Aggiungere implementazioni build-tagged Unix/Windows usando un temporaneo nella stessa directory e `MoveFileExW` su Windows nella v1.
6. Usare permessi restrittivi (`0600`) quando il file viene creato su sistemi Unix.
7. Garantire cleanup del temporaneo in caso di errore.
8. Testare che un errore pre-replace lasci integro il file precedente e documentare i limiti di atomicità/durabilità del filesystem.

**Criteri di completamento:**

- nessun file parziale dopo un errore;
- overwrite verificato;
- path portabili tramite `os`, `io` e `filepath`;
- differenze OS isolate al package filesystem.

### Fase 9 — Hardening, documentazione e release

**Obiettivo:** rendere il progetto distribuibile e manutenibile.

Attività:

1. Completare README con installazione, uso, formato annotation ed esempio.
2. Aggiungere `LICENSE` e `THIRD_PARTY_NOTICES`.
3. Documentare la dependency policy.
4. Aggiungere `CONTRIBUTING.md` e `SECURITY.md`.
5. Estendere la CI minima con vulnerability scan, controllo licenze e build/artifact di release.
6. Eseguire `go test ./...`, `go vet ./...` e `govulncheck ./...`.
7. Verificare le licenze con `go-licenses` o tooling equivalente.
8. Costruire almeno Linux amd64 e Windows amd64; aggiungere Linux arm64 se richiesto.
9. Definire version injection per `--version`.
10. Preparare gli artifact con licenze e notice inclusi.

---

## 7. Test plan

### 7.1 Test del modello e parser strutturale

- commenti normali;
- righe vuote;
- variabili vuote e valorizzate;
- valori con `=`;
- valori quoted;
- ordine dei nodi;
- line ending LF e CRLF;
- BOM UTF-8 iniziale normalizzato;
- EOL misti, `CR` isolato, file vuoto e assenza di newline finale;
- numero di riga negli errori;
- duplicate key;
- sintassi non supportata;
- grammatica delle chiavi e case sensitivity;
- round-trip senza modifiche.

### 7.2 Test annotation

Per ogni annotation v1 verificare parsing, binding e rendering. Coprire anche:

- annotation consecutive;
- riga vuota che interrompe il binding;
- annotation sconosciuta;
- valore mancante;
- duplicati incompatibili;
- `@options` con default non consentito;
- option vuote, duplicate o contenenti virgole;
- `@options` + `@type bool`, `@placeholder` su widget non testuali e `@secret` + `@options`;
- combinazioni incompatibili della matrice v1;
- commento normale che interrompe il binding;
- annotation senza variabile successiva;
- `@fixed` esclusa dalle domande;
- `@section` applicata alle variabili successive;
- assenza di section con fallback `Configuration`.

### 7.3 Test validator

Casi minimi:

- required: stringa non vuota e solo whitespace;
- int: positivi, negativi, overflow e testo non numerico;
- bool: valori supportati e non supportati;
- bool: valore vuoto nel template o nel file esistente;
- port: `1`, `5432`, `65535` validi; `0`, `65536`, `foo` non validi;
- URL valida e invalida;
- errori privi del valore quando il campo è secret.

### 7.4 Test writer e round-trip semantico

Per ogni valore:

```text
EncodeValue(value)
→ compose-go/dotenv.Parse
→ confronto con value originale
```

Coprire almeno:

- stringhe vuote;
- spazi iniziali/finali;
- `#`, `$`, `"`, `'`, `\\`;
- `$VAR` e `${VAR}` rifiutati, `$$` interpretato come `$` letterale;
- newline;
- Unicode;
- URL;
- password;
- `=` nel valore.

### 7.5 Test workflow

- template valido e invalido;
- template e output che identificano lo stesso file;
- uso senza TTY;
- `.env` assente;
- `.env` esistente;
- valori esistenti usati come default;
- variabili nuove e obsolete;
- `@fixed` prevalente sul `.env` esistente;
- summary secret con `[set]`;
- summary secret con `[not set]`;
- secret esistente confermato senza modifiche;
- conferma overwrite;
- `--force`;
- cancel e Ctrl+C senza scrittura;
- errore di scrittura senza perdita del file precedente.

### 7.6 Test cross-platform e CI

Le fixture devono includere LF e CRLF, BOM, file senza newline finale, EOL misti e UTF-8 invalido. La CI minima deve partire dalla Fase 1 ed eseguire almeno `go test ./...`, `go vet ./...` e build su Linux e Windows. La CI di release aggiunge `govulncheck`, controllo licenze e artifact. Le prove che dipendono dal terminale devono essere isolate o coperte con test dell'adapter e test di integrazione controllati; non devono richiedere un terminale interattivo nella CI ordinaria.

---

## 8. Backlog v2

Non implementare nella prima release:

- `@suggest` come autocomplete libero;
- `@multiple` e `MultiSelect`;
- `@pattern`;
- `@when` e dipendenze condizionali;
- `@generate` e generazione automatica con `crypto/rand`;
- valori multilinea;
- modalità non interattiva;
- subcommand (`init`, `validate`, `update`, `doctor`);
- supporto a un set più ampio di tipi (`email`, `hostname`, `duration`, `path`, ecc.);
- Cobra, framework di validazione o logger esterni senza una necessità concreta.

Prima di ogni ampliamento verificare che il nuovo requisito non trasformi le annotation in un DSL complesso e che il modello intermedio resti sufficiente.

---

## 9. Definition of Done della v1

La v1 è pronta quando:

- `env-wizard` compila e mostra `--version`;
- il comando usa `.env.example` e `.env` come default corretti;
- il comando usa un lookup dotenv controllato e non dipende dall'ambiente esterno del processo;
- template validi con commenti, righe vuote, quote e annotation v1 vengono interpretati correttamente;
- template invalidi vengono rifiutati prima del wizard con linea e causa;
- template con BOM vengono normalizzati, mentre UTF-16, UTF-8 non valido, EOL misti, CR isolati e sintassi fuori dal subset vengono rifiutati;
- le chiavi rispettano `[A-Za-z_][A-Za-z0-9_.-]*` e i duplicati sono rifiutati;
- l'interpolazione `$VAR`/`${VAR}` viene rifiutata e `$$` è gestito solo come `$` letterale;
- il parser semantico dotenv è delegato a `compose-go` con lookup controllato e deterministico;
- il dominio non dipende da Huh o filesystem;
- il wizard costruisce gruppi/field coerenti con il modello `Question` e usa `EchoModePassword` per `@secret`;
- `@required`, `@secret`, `@options`, `@type`, `@placeholder`, `@fixed` e `@section` funzionano secondo specifica;
- il rerun usa i valori del `.env` esistente con le precedenze definite;
- i secret esistenti vengono conservati quando l'utente conferma senza modificarli;
- i booleani vuoti sono rifiutati e quelli validi vengono scritti come `true` o `false`;
- i secret non compaiono in summary, log o errori e `.env` viene documentato come non-secret-store;
- il writer rimuove le annotation e conserva la struttura utile del documento;
- l'output supera i test di round-trip con `compose-go`;
- la scrittura usa temporaneo, `Sync`, `Close` e replace;
- cancel, Ctrl+C ed errori non lasciano `.env` parziali;
- il comando rifiuta l'uso non-TTY e impedisce `--output` uguale a `--template`;
- `go test ./...`, `go vet ./...` e `govulncheck ./...` sono verdi;
- sono disponibili build Linux e Windows;
- README, `SPEC.md`, `CONTRIBUTING.md`, `SECURITY.md`, licenza e third-party notices sono presenti e coerenti.

---

## 10. Ordine esecutivo raccomandato

L'ordine da seguire è:

```text
1. Specifica e skeleton
2. Modello Document
3. Structural scanner + adapter compose-go
4. Annotation parser + validazione del template
5. Writer + round-trip test
6. Validation + Question model
7. Huh spike e adapter
8. Existing `.env` e merge
9. Summary e conferma
10. Scrittura sicura cross-platform
11. CI minima già dalla Fase 1
12. Hardening, documentazione e release
```

Il valore principale del progetto è il motore:

```text
.env.example
      ↓
struttura + annotation + valori Compose
      ↓
configuration model
      ↓
.env
```

Huh deve rimanere un adapter interattivo sopra quel motore, non il luogo in cui vengono mescolati parsing, validazione e filesystem.

L'ordine sopra è l'ordine di implementazione. L'ordine obbligatorio a runtime resta quello del workflow applicativo (§5.2): parsing semantico e strutturale, binding e validazione del template, caricamento del `.env` esistente, merge dei valori, creazione delle `Question`, esecuzione del wizard, summary, conferma e scrittura.
