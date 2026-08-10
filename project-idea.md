## Cos'è questo progetto

Questo progetto fornisce una piccola CLI cross-platform pensata per guidare l'utente nella creazione del file `.env` necessario alla configurazione e all'avvio di un'applicazione tramite Docker.

Il tool utilizza `.env.example` come sorgente di verità e permette di arricchirlo con annotation opzionali definite tramite commenti. Queste annotation possono descrivere prompt più leggibili, valori obbligatori, secret, opzioni selezionabili, regole di validazione e sezioni del wizard. In assenza di annotation, il tool applica comunque un comportamento predefinito sensato, così da poter essere utilizzato anche con file `.env.example` già esistenti o minimamente modificati.

L'obiettivo è rendere il setup dell'applicazione semplice e coerente sia su Windows che su Linux, evitando di dover mantenere script separati per i diversi sistemi operativi e mantenendo `.env.example` leggibile e utilizzabile anche indipendentemente dal wizard.

---

Imposterei il progetto in modo che `.env.example` rimanga **un normale file dotenv valido e leggibile anche senza conoscere il wizard**. Le annotation sono solo metadati opzionali nei commenti; in loro assenza il comportamento deve essere sensato.

Con Huh v2 oggi userei l'import `charm.land/huh/v2`; Huh organizza i form in `Group` assimilabili a pagine e mette già a disposizione `Input`, `Select`, `MultiSelect`, `Confirm`, password masking, placeholder, suggestions e validation. ([GitHub][1])

## 1. Filosofia delle annotation

Partirei da queste regole:

```dotenv
DB_HOST=
DB_PORT=5432
```

deve già funzionare senza annotation.

Il comportamento di default sarebbe:

* ogni `KEY=VALUE` è configurabile;
* `KEY` diventa il titolo del prompt;
* il valore presente in `.env.example` è il default;
* valore vuoto significa nessun default;
* l'utente può confermare il default semplicemente andando avanti;
* le annotation modificano questo comportamento solo quando necessario.

Questo evita di trasformare:

```dotenv
.env.example
```

in:

```text
un DSL custom che per caso assomiglia a .env
```

### Sintassi proposta

Userei esclusivamente commenti nel formato:

```dotenv
# @annotation valore
VARIABLE=value
```

Per esempio:

```dotenv
# @prompt Database host
# @description Hostname del server PostgreSQL
# @required
DB_HOST=localhost
```

Le annotation consecutive si applicano alla **prima variabile immediatamente successiva**.

Una riga vuota interrompe il blocco:

```dotenv
# @required

DB_HOST=
```

In questo caso `@required` non dovrebbe applicarsi a `DB_HOST`.

Questo rende il parsing prevedibile.

---

# 2. Annotation che metterei nella v1

## `@prompt`

Personalizza il testo mostrato all'utente.

```dotenv
# @prompt Host del database
DB_HOST=localhost
```

Senza annotation:

```text
DB_HOST
> localhost
```

Con annotation:

```text
Host del database
> localhost
```

**Fallback:** nome della variabile.

---

## `@description`

Aggiunge una spiegazione al prompt.

```dotenv
# @prompt Host del database
# @description Hostname raggiungibile dai container Docker
DB_HOST=postgres
```

Idealmente Huh mostrerà qualcosa tipo:

```text
Host del database
Hostname raggiungibile dai container Docker

> postgres
```

Utile soprattutto per variabili poco intuitive.

---

## `@required`

Impedisce di proseguire con un valore vuoto.

```dotenv
# @required
DB_NAME=
```

Equivale concettualmente a:

```go
strings.TrimSpace(value) != ""
```

Importante: `@required` **non significa che il template debba essere vuoto**.

Questo è valido:

```dotenv
# @required
DB_PORT=5432
```

L'utente può accettare `5432`, ma non cancellarlo lasciando il campo vuoto.

---

## `@secret`

Indica che il valore è sensibile.

```dotenv
# @prompt Password PostgreSQL
# @required
# @secret
DB_PASSWORD=
```

Comportamento:

* input possibilmente in chiaro, leggibile dall'utente mentre compila il valore;
* valore non mostrato nel riepilogo;
* valore mai scritto nei log;
* errore di validazione mai contenente il valore;
* se viene caricato da un `.env` esistente, non deve essere stampato.

Huh v2 supporta esplicitamente modalità di echo per password/input nascosti. ([GitHub][2])

Una precisazione importante: `@secret` protegge solamente **l'UX del wizard**. Alla fine il valore finisce comunque nel `.env`, quindi non è un secret store. Docker stesso raccomanda Docker Secrets anziché environment variables per credenziali particolarmente sensibili. ([Docker Documentation][3])

---

## `@options`

Trasforma il campo in una scelta vincolata.

```dotenv
# @prompt Ambiente
# @options development,staging,production
APP_ENV=development
```

Wizard:

```text
Ambiente

> development
  staging
  production
```

Regola proposta:

```text
@options value1,value2,value3
```

Nella v1 eviterei sintassi più sofisticate.

Il valore presente nel template:

```dotenv
APP_ENV=development
```

deve esistere tra le option; altrimenti il parser produce un errore di configurazione.

---

## `@type`

Definisce il tipo semantico e quindi widget e validazione.

```dotenv
# @type int
WORKERS=4
```

Partirei con pochissimi tipi:

| Tipo     | Widget  | Validazione    |
| -------- | ------- | -------------- |
| `string` | Input   | nessuna        |
| `int`    | Input   | intero         |
| `bool`   | Confirm | boolean        |
| `port`   | Input   | intero 1–65535 |
| `url`    | Input   | URL valida     |

`string` sarebbe implicitamente il default.

Esempio:

```dotenv
# @prompt Porta PostgreSQL
# @type port
DB_PORT=5432
```

Non metterei subito `email`, `ip`, `hostname`, `duration`, `path`, ecc. Si possono aggiungere quando servono davvero.

---

## `@placeholder`

Fornisce un esempio **che non viene utilizzato come valore**.

```dotenv
# @prompt Host SMTP
# @placeholder smtp.example.com
SMTP_HOST=
```

La distinzione è importante:

```dotenv
SMTP_HOST=smtp.example.com
```

significa:

> il default è `smtp.example.com`

mentre:

```dotenv
# @placeholder smtp.example.com
SMTP_HOST=
```

significa:

> il valore è vuoto, `smtp.example.com` è solamente un suggerimento visivo.

---

## `@fixed`

Include la variabile nell'output ma non la mostra nel wizard.

```dotenv
# @fixed
COMPOSE_PROJECT_NAME=myapp
```

Output:

```dotenv
COMPOSE_PROJECT_NAME=myapp
```

ma nessuna domanda all'utente.

È molto utile per valori che vuoi mantenere nel `.env.example` perché fanno parte della configurazione, ma che l'utente non dovrebbe normalmente modificare.

Preferisco il nome `@fixed` a `@skip`: "`skip`" potrebbe essere interpretato come "non scrivere questa variabile", mentre `fixed` comunica bene che viene scritta ma non modificata.

---

## `@section`

Questa sarebbe l'unica annotation che non appartiene a una singola variabile.

```dotenv
# @section Database

DB_HOST=postgres
DB_PORT=5432
DB_NAME=

# @section Redis

REDIS_HOST=redis
REDIS_PORT=6379
```

Serve a creare le pagine/gruppi del wizard.

Huh usa proprio `Group` per organizzare i campi del form, quindi la corrispondenza è naturale. ([GitHub][1])

Potresti ottenere:

```text
Database
────────────
DB_HOST
DB_PORT
DB_NAME

       Continue →

Redis
────────────
REDIS_HOST
REDIS_PORT
```

Se non c'è nessuna `@section`, tutto finisce semplicemente nel gruppo `Configuration`.

---

# 3. Annotation che terrei per una v2

Non le implementerei immediatamente, ma progetterei il modello dati in modo da poterle aggiungere.

### `@pattern`

Validazione tramite regexp.

```dotenv
# @pattern ^[a-z][a-z0-9_-]+$
COMPOSE_PROJECT_NAME=myapp
```

Molto potente, ma meno user-friendly di validator semantici come `@type port`.

---

### `@suggest`

Diversa da `@options`.

```dotenv
# @suggest localhost,postgres,database
DB_HOST=
```

L'utente può scegliere un suggerimento **oppure scrivere qualsiasi altro valore**.

Huh supporta suggestions direttamente sugli input. ([GitHub][2])

Quindi:

```text
@options
```

= enum chiusa.

```text
@suggest
```

= autocomplete.

---

### `@multiple`

Da usare insieme a `@options`.

```dotenv
# @options api,worker,scheduler,admin
# @multiple
ENABLED_SERVICES=api,worker
```

Genererebbe un `MultiSelect`.

Serializzazione:

```dotenv
ENABLED_SERVICES=api,worker,scheduler
```

---

### `@when`

Prompt condizionale.

```dotenv
# @prompt Abilitare SMTP
# @type bool
SMTP_ENABLED=false

# @when SMTP_ENABLED=true
# @required
SMTP_HOST=

# @when SMTP_ENABLED=true
# @type port
SMTP_PORT=587
```

È molto interessante, ma introduce:

* dipendenze tra variabili;
* ordine di valutazione;
* possibili riferimenti a variabili future;
* eventuali cicli.

Per questo la terrei fuori dalla prima release.

---

### `@generate`

Generazione automatica di valori.

```dotenv
# @secret
# @generate random:32
JWT_SECRET=
```

Potrebbe generare tramite `crypto/rand` un secret casuale.

Molto utile, per esempio, per:

```text
JWT_SECRET
APP_KEY
SESSION_SECRET
```

ma anche qui definirei bene la semantica prima di implementarla.

---

# 4. Annotation che NON introdurrei

In particolare eviterei:

```dotenv
# @default localhost
DB_HOST=
```

perché hai già:

```dotenv
DB_HOST=localhost
```

Duplicare l'informazione crea il rischio di avere:

```dotenv
# @default localhost
DB_HOST=postgres
```

e dover decidere quale dei due sia quello vero.

Il `.env.example` deve rimanere la source of truth.

Eviterei anche:

```text
@optional
@string
@editable
```

perché rappresentano già il comportamento di default.

---

# 5. Esempio realistico di `.env.example`

Una prima versione potrebbe quindi essere molto leggibile:

```dotenv
# ========================================
# APPLICATION
# ========================================

# @section Application

# @prompt Environment
# @description Ambiente nel quale verrà eseguita l'applicazione
# @options development,staging,production
APP_ENV=development

# @prompt Debug mode
# @type bool
APP_DEBUG=false

# @fixed
COMPOSE_PROJECT_NAME=myapp


# ========================================
# DATABASE
# ========================================

# @section Database

# @prompt Database host
# @description Host raggiungibile dal container applicativo
# @required
DB_HOST=postgres

# @prompt Database port
# @type port
# @required
DB_PORT=5432

# @prompt Database name
# @required
DB_NAME=myapp

# @prompt Database username
# @required
DB_USERNAME=myapp

# @prompt Database password
# @secret
# @required
DB_PASSWORD=


# ========================================
# REDIS
# ========================================

# @section Redis

REDIS_HOST=redis

# @type port
REDIS_PORT=6379


# ========================================
# MAIL
# ========================================

# @section Mail

# @placeholder smtp.example.com
SMTP_HOST=

# @type port
SMTP_PORT=587

# @secret
SMTP_PASSWORD=
```

E questa è secondo me una caratteristica importante: **anche aprendolo senza il wizard, il file rimane perfettamente comprensibile**.

Docker Compose accetta commenti, righe vuote e coppie chiave-valore nei file `.env`, oltre a valori quoted/unquoted; quindi la convenzione basata su commenti non interferisce con il normale formato. ([Docker Documentation][4])

---

# 6. Modello interno: non userei `map[string]string`

Questo per me è uno dei punti architetturali più importanti.

Non farei:

```go
map[string]string
```

come rappresentazione del documento.

Farei qualcosa simile a:

```go
type Document struct {
    Nodes []Node
}

type Node interface {
    node()
}

type Variable struct {
    Key         string
    Value       string
    Annotations Annotations
}

type Comment struct {
    Raw string
}

type BlankLine struct{}

type AnnotationLine struct {
    Name  string
    Value string
}
```

E poi:

```go
type Annotations struct {
    Prompt      string
    Description string
    Required    bool
    Secret      bool
    Fixed       bool
    Type        ValueType
    Placeholder string
    Options     []string
    Section     string
}
```

Questo permette di preservare:

* ordine;
* commenti;
* righe vuote;
* sezioni;
* quote;
* line ending;
* posizione delle variabili.

Il writer può inoltre decidere di **non copiare le annotation nel `.env` finale**, mantenendo invece i normali commenti.

Per esempio il template:

```dotenv
# Database configuration

# @prompt Database host
# @required
DB_HOST=
```

potrebbe generare:

```dotenv
# Database configuration

DB_HOST=postgres
```

Questa sarebbe la mia scelta di default.

---

# 7. Architettura del progetto Go

Partirei con qualcosa del genere:

```text
env-wizard/
│
├── cmd/
│   └── env-wizard/
│       └── main.go
│
├── internal/
│   ├── document/
│   │   ├── model.go
│   │   ├── parser.go
│   │   ├── annotations.go
│   │   └── writer.go
│   │
│   ├── wizard/
│   │   ├── wizard.go
│   │   ├── field_factory.go
│   │   └── summary.go
│   │
│   ├── validation/
│   │   ├── validation.go
│   │   └── types.go
│   │
│   └── app/
│       └── app.go
│
├── testdata/
│   ├── basic.env.example
│   ├── annotated.env.example
│   └── complex.env.example
│
├── go.mod
├── go.sum
├── README.md
└── Makefile
```

Non introdurrei Cobra.

Per ora il package standard `flag` è più che sufficiente.

---

# 8. Separazione delle responsabilità

## `document`

Non conosce Huh.

Sa solamente:

```text
.env.example
       ↓
Document
       ↓
.env
```

Responsabilità:

* parsing;
* annotation;
* preservazione struttura;
* modifica valori;
* rendering.

Questo package dovrebbe essere testabile al 100% senza terminale.

---

## `validation`

Non conosce il file system e non conosce Huh.

Per esempio:

```go
func Required(value string) error
func Integer(value string) error
func Port(value string) error
func URL(value string) error
```

Huh permette di collegare funzioni di validazione agli input, quindi questi validator possono essere usati direttamente dalla UI. ([GitHub][2])

---

## `wizard`

Converte:

```go
Variable
```

in:

```go
huh.Field
```

Concettualmente:

```text
@options          → Select
@options+multiple → MultiSelect
@type bool        → Confirm
@secret           → Input + password echo
default           → Input
```

Questa logica dovrebbe stare quasi tutta in:

```text
field_factory.go
```

---

## `app`

Orchestra il workflow:

```text
parse arguments
      ↓
load template
      ↓
parse document
      ↓
validate annotations
      ↓
load existing .env (optional)
      ↓
run wizard
      ↓
show summary
      ↓
confirm
      ↓
write .env
```

`main.go` dovrebbe essere quasi vuoto.

---

# 9. Supporterei subito il rerun

C'è una funzionalità che aggiungerei già alla prima versione: se `.env` esiste, lo userei come **fonte dei valori correnti**.

Immagina:

```dotenv
.env.example

DB_HOST=postgres
DB_PORT=5432
DB_NAME=myapp
DB_PASSWORD=
```

e:

```dotenv
.env

DB_HOST=db.internal
DB_PORT=5432
DB_NAME=production
DB_PASSWORD=supersecret
```

Eseguendo nuovamente:

```bash
env-wizard
```

il wizard dovrebbe partire da:

```text
DB_HOST     db.internal
DB_PORT     5432
DB_NAME     production
DB_PASSWORD ********
```

non dai valori del template.

Definirei quindi la precedenza:

```text
risposta wizard
      ↓
.env esistente
      ↓
.env.example
```

Con eccezione per `@fixed`, che prenderei sempre da `.env.example`.

Questo rende il tool utile non solo durante il primo setup ma anche per **riconfigurare** un'installazione.

---

# 10. CLI iniziale

La manterrei volutamente piccola:

```bash
env-wizard
```

Default:

```text
template = .env.example
output   = .env
```

Poi:

```bash
env-wizard --template config/.env.example
```

```bash
env-wizard --output .env.local
```

e magari:

```bash
env-wizard --force
```

per evitare il confirm di overwrite.

Aggiungerei anche:

```bash
env-wizard --version
```

Non aggiungerei ancora subcommand.

---

# 11. Gestione degli errori

Distinguerei chiaramente due tipi di errore.

### Template errato

Per esempio:

```dotenv
# @type banana
FOO=
```

Output:

```text
Invalid .env.example

line 14: FOO: unknown type "banana"
```

oppure:

```dotenv
# @options dev,test,production
APP_ENV=local
```

```text
line 22: APP_ENV: default value "local" is not one of the allowed options
```

Questi sono errori dello sviluppatore e dovrebbero interrompere il programma **prima di mostrare il wizard**.

### Input utente errato

```text
Database port
> foo

Port must be an integer between 1 and 65535.
```

Qui rimani semplicemente nel field corrente.

---

# 12. Attenzione particolare al writer `.env`

Non farei banalmente:

```go
fmt.Sprintf("%s=%s", key, value)
```

Docker Compose ha regole precise per commenti, quoting e interpolazione dei valori `.env`; in particolare valori unquoted e double-quoted possono essere soggetti a interpolazione. ([Docker Documentation][4])

Separerei quindi esplicitamente:

```go
func EncodeValue(value string, style QuoteStyle) string
```

da tutto il resto.

Il parser potrebbe ricordare:

```go
type QuoteStyle int

const (
    Unquoted QuoteStyle = iota
    SingleQuoted
    DoubleQuoted
)
```

in modo da poter preservare, quando sensato, lo stile originale del template.

---

# 13. Plan di implementazione

## Fase 0 — Specifica

Prima di scrivere il wizard fisserei formalmente:

```text
annotation syntax
parser behaviour
default behaviour
overwrite behaviour
validation behaviour
output behaviour
```

È probabilmente la fase che ti farà risparmiare più tempo in seguito.

---

## Fase 1 — Skeleton CLI

Obiettivo:

```bash
env-wizard --template .env.example --output .env
```

senza ancora Huh.

Implementare:

```text
CLI args
↓
open template
↓
parse
↓
render identico
```

Il primo milestone dovrebbe essere:

```text
input .env.example
→ parse
→ render
→ stesso documento
```

---

## Fase 2 — Parser e AST

Implementare:

```go
Document
Variable
Comment
BlankLine
AnnotationLine
```

e parsing di:

```dotenv
KEY=
KEY=value
KEY="value"
KEY='value'
```

oltre alla preservazione di:

```text
LF
CRLF
commenti
righe vuote
ordine
```

Questo è il cuore del progetto.

---

## Fase 3 — Annotation parser

Implementare inizialmente:

```text
@prompt
@description
@required
@secret
@type
@options
@placeholder
@fixed
@section
```

Poi una fase separata:

```go
ValidateDocument(doc)
```

che controlli le combinazioni impossibili.

---

## Fase 4 — Wizard model indipendente da Huh

Prima di generare direttamente componenti Huh, creerei un modello intermedio:

```go
type Question struct {
    Key         string
    Prompt      string
    Description string
    Value       string
    Kind        QuestionKind
    Required    bool
    Secret      bool
    Options     []string
}
```

Quindi:

```text
Document
   ↓
[]Question
   ↓
Huh
```

Questo disaccoppia completamente il formato `.env` dalla libreria UI.

Se un domani vuoi sostituire Huh, non devi toccare il parser.

---

# 14. Field factory Huh

A quel punto:

```go
func NewField(question *Question) huh.Field
```

o equivalente.

La factory decide:

```text
string                 → Input
secret                 → Input password
options                → Select
bool                   → Confirm
multiple + options     → MultiSelect
```

Questo file probabilmente sarà sorprendentemente piccolo.

---

# 15. Summary

Prima di scrivere:

```text
Configuration
────────────────────────────

APP_ENV       development
APP_DEBUG     false

Database
────────────────────────────

DB_HOST       postgres
DB_PORT       5432
DB_NAME       myapp
DB_PASSWORD   ********

Redis
────────────────────────────

REDIS_HOST    redis
REDIS_PORT    6379

Create .env? [Y/n]
```

Per un secret eviterei perfino di indicarne la lunghezza.

Meglio:

```text
DB_PASSWORD    [set]
```

che:

```text
DB_PASSWORD    ************
```

perché la seconda può far intuire quanti caratteri contiene.

---

# 16. Writer

Pipeline finale:

```text
Document originale
       +
risposte
       ↓
UpdateVariables()
       ↓
RemoveAnnotationLines()
       ↓
Render()
       ↓
.env
```

Scriverei il file solo **dopo** che tutti i valori sono validi.

Su sistemi Unix, se il `.env` può contenere credenziali, userei permessi restrittivi (`0600`) quando viene creato; su Windows la gestione effettiva dei permessi è differente, ma il codice rimane comunque portabile.

---

# 17. Testing strategy

Questa parte la farei abbastanza bene anche se è un tool interno, perché parser + writer sono esattamente il genere di codice dove piccoli edge case fanno perdere tempo.

### Unit test parser

Input:

```dotenv
# Comment
FOO=bar

BAR="hello world"
EMPTY=
```

Verificare AST.

### Round-trip tests

```text
parse(template)
→ render(document)
→ template originale
```

finché non vengono modificati valori.

### Annotation tests

Per ciascuna annotation:

```text
input
↓
AST previsto
```

### Validation tests

Per esempio:

```text
port:
0       fail
1       pass
5432    pass
65535   pass
65536   fail
foo     fail
```

### Cross-platform tests

Userei fixture sia:

```text
LF
```

sia:

```text
CRLF
```

per evitare sorprese tra Linux e Windows.

---

# 18. Checklist implementativa

Ti lascerei il backlog del progetto più o meno così.

### Project setup

* [ ] Inizializzare modulo Go
* [ ] Aggiungere Huh v2
* [ ] Creare `cmd/env-wizard`
* [ ] Implementare `--template`
* [ ] Implementare `--output`
* [ ] Implementare `--force`
* [ ] Implementare `--version`

### Document model

* [ ] Creare `Document`
* [ ] Creare `Variable`
* [ ] Creare `Comment`
* [ ] Creare `BlankLine`
* [ ] Creare `AnnotationLine`
* [ ] Conservare ordine dei node
* [ ] Conservare LF/CRLF
* [ ] Conservare quote style

### `.env` parser

* [ ] Parse variabile vuota
* [ ] Parse valore semplice
* [ ] Parse valore single-quoted
* [ ] Parse valore double-quoted
* [ ] Parse commenti
* [ ] Parse righe vuote
* [ ] Gestire `=` all'interno del valore
* [ ] Gestire whitespace
* [ ] Restituire numero di linea negli errori
* [ ] Round-trip test parser → writer

### Annotation parser

* [ ] `@prompt`
* [ ] `@description`
* [ ] `@required`
* [ ] `@secret`
* [ ] `@type`
* [ ] `@options`
* [ ] `@placeholder`
* [ ] `@fixed`
* [ ] `@section`
* [ ] Errore su annotation sconosciuta
* [ ] Errore su annotation duplicata incompatibile
* [ ] Errore con numero di linea

### Validation

* [ ] `string`
* [ ] `int`
* [ ] `bool`
* [ ] `port`
* [ ] `url`
* [ ] required validator
* [ ] combinazione `@options` + default
* [ ] validazione completa template prima del wizard

### Question model

* [ ] Creare `Question`
* [ ] Convertire `Variable → Question`
* [ ] Risolvere prompt di default da KEY
* [ ] Applicare valore template come default
* [ ] Raggruppare domande per section
* [ ] Escludere `@fixed`

### Huh integration

* [ ] Input string
* [ ] Input required
* [ ] Input secret
* [ ] Input placeholder
* [ ] Select
* [ ] Confirm boolean
* [ ] Collegare validator
* [ ] Creare Group per section
* [ ] Gestire Ctrl+C senza creare `.env`

### Existing `.env`

* [ ] Rilevare `.env` esistente
* [ ] Leggere valori esistenti
* [ ] Usarli come default del wizard
* [ ] Non visualizzare secret esistenti
* [ ] Gestire variabili nuove presenti solo nel template
* [ ] Gestire variabili obsolete presenti solo nel vecchio `.env`
* [ ] Definire comportamento `@fixed`
* [ ] Confirm overwrite

### Summary

* [ ] Raggruppamento per section
* [ ] Mostrare valori normali
* [ ] Mostrare `[set]` per secret
* [ ] Confirm finale
* [ ] Cancel senza scrittura

### Writer

* [ ] Aggiornare valori nel Document
* [ ] Rimuovere annotation dal file generato
* [ ] Preservare commenti normali
* [ ] Preservare ordine
* [ ] Preservare blank line
* [ ] Encoding valori
* [ ] Gestire quoting
* [ ] Scrittura `.env`
* [ ] Permessi restrittivi dove applicabili
* [ ] Non lasciare file parziali in caso di errore

### Tests

* [ ] Unit test parser
* [ ] Unit test annotation parser
* [ ] Unit test validator
* [ ] Unit test writer
* [ ] Round-trip tests
* [ ] LF fixtures
* [ ] CRLF fixtures
* [ ] Test secret
* [ ] Test `.env` esistente
* [ ] Test overwrite
* [ ] Test malformed template
* [ ] Test Ctrl+C

### Release

* [ ] Build Linux amd64
* [ ] Build Linux arm64, se necessario
* [ ] Build Windows amd64
* [ ] Definire version injection
* [ ] CI per test
* [ ] CI per build
* [ ] Pubblicare artifact
* [ ] README con esempio `.env.example`

Go si presta bene alla produzione dei diversi binari tramite target `GOOS`/`GOARCH`; tenendo il progetto pure-Go ed evitando cgo, il cross-compiling rimane particolarmente semplice. ([Go][5])

---

## 19. Ordine che userei concretamente

Se dovessi iniziare domani, **non partirei da Huh**.

Farei:

```text
1. Document model
        ↓
2. Parser
        ↓
3. Writer + round-trip test
        ↓
4. Annotation parser
        ↓
5. Validation
        ↓
6. Question model
        ↓
7. Huh integration
        ↓
8. Existing .env
        ↓
9. Summary + write
        ↓
10. CI builds Windows/Linux
```

Il vero asset del progetto non è la UI: è il piccolo motore

```text
.env.example
      ↓
parse + annotations
      ↓
configuration model
      ↓
.env
```

Huh dovrebbe essere semplicemente l'adapter interattivo sopra quel motore. Così eviti che tra sei mesi tutto il progetto diventi un groviglio di `huh.NewInput()` mescolati a parsing, validazione e filesystem.

Per una **v1** congelerei il vocabolario a nove annotation:

```text
@prompt
@description
@required
@secret
@type
@options
@placeholder
@fixed
@section
```

e terrei `@suggest`, `@multiple`, `@pattern`, `@when` e `@generate` come backlog. È abbastanza potente per fare già un wizard interno molto buono, senza progettare un mini-framework prima di averne bisogno.

[1]: https://github.com/charmbracelet/huh/blob/main/README.md "huh/README.md at main · charmbracelet/huh · GitHub"
[2]: https://github.com/charmbracelet/huh/blob/main/field_input.go "huh/field_input.go at main · charmbracelet/huh · GitHub"
[3]: https://docs.docker.com/compose/how-tos/environment-variables/set-environment-variables/?utm_source=chatgpt.com "Set environment variables within your container's environment"
[4]: https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/?utm_source=chatgpt.com "Set, use, and manage variables in a Compose file with interpolation"
[5]: https://go.dev/wiki/WindowsCrossCompiling?utm_source=chatgpt.com "Go Wiki: Building Windows Go programs on Linux"



Sì, e secondo me puoi evitare quasi completamente il rischio di scrivere “il parser dotenv”. Per questo progetto userei il parser del progetto **Compose Specification**:

```go
github.com/compose-spec/compose-go/v2/dotenv
```

È parte di `compose-go`, la libreria Go di riferimento per la Compose Specification, ed espone direttamente `dotenv.Parse(io.Reader)`, che restituisce le variabili già interpretate. Essendo il vostro `.env` destinato a Docker Compose, lo preferirei a una libreria dotenv generica. ([Go Packages][1])

La cosa importante però è questa: **non userei quel parser come parser del documento**. `Parse` restituisce una `map[string]string`, quindi perde ordine, commenti, righe vuote e soprattutto le nostre annotation. ([Go Packages][2])

Io adotterei quindi un approccio ibrido.

### 1. Compose parser per la semantica

Lasciamo che `compose-go` si occupi delle parti difficili:

```go
values, err := dotenv.Parse(bytes.NewReader(content))
```

Quindi:

```dotenv
PASSWORD="hello world"
URL='https://foo.bar?a=1&b=2'
MESSAGE="hello\nworld"
```

vengono interpretati da una libreria già esistente.

Questo è importante perché il formato supportato da Compose ha più edge case di quanto sembri: quoting, escaping, inline comments, interpolazione `$VAR`, delimitatore `:` oltre a `=`, valori multilinea single-quoted, ecc. ([Docker Documentation][3])

### 2. Un nostro scanner molto semplice per la struttura

Il nostro codice deve invece capire soltanto:

```text
commento
annotation
riga vuota
variabile
```

Non deve sapere come interpretare il valore.

Per esempio:

```dotenv
# Database configuration

# @prompt Database host
# @required
DB_HOST="postgres"

# @secret
DB_PASSWORD=
```

diventa concettualmente:

```go
Document{
    Nodes: []Node{
        Comment{"# Database configuration"},
        BlankLine{},

        Annotation{"prompt", "Database host"},
        Annotation{"required", ""},
        Variable{
            Key: "DB_HOST",
        },

        BlankLine{},

        Annotation{"secret", ""},
        Variable{
            Key: "DB_PASSWORD",
        },
    },
}
```

Poi il valore effettivo di `DB_HOST` non lo prendiamo dal nostro parser. Lo prendiamo da:

```go
values["DB_HOST"]
```

che arriva da `compose-go/dotenv`.

Questa separazione mi piace molto perché significa che **noi non stiamo implementando un parser dotenv**. Stiamo implementando un parser del *nostro formato di documento annotato*, che è enormemente più semplice.

---

## Metterei anche dei vincoli al `.env.example`

Visto che è un file controllato da noi, non vedo motivo di supportare ogni possibile variante sintattica ammessa da Compose.

Per la **v1** definirei ufficialmente il nostro template come un subset:

```dotenv
KEY=value
KEY="value"
KEY='value'
```

più:

```dotenv
# commenti

# @annotation ...
```

E basta.

In particolare non supporterei inizialmente nel template:

```dotenv
KEY: value
```

anche se Compose lo accetta. ([Docker Documentation][3])

E soprattutto eviterei nella v1 i valori multilinea:

```dotenv
CERTIFICATE='-----BEGIN-----
blah
blah
-----END-----'
```

Docker Compose li supporta, ma complicano parecchio il nostro scanner strutturale. ([Docker Documentation][3])

Se un giorno serviranno, possiamo aggiungerli.

Questo è un vantaggio importante di avere un tool interno: **non dobbiamo scrivere un parser universale di `.env`**, dobbiamo solo definire chiaramente cosa accettiamo come `.env.example`.

---

## Il flusso che userei

Quindi:

```text
.env.example
      │
      ├──────────────► compose-go/dotenv
      │                       │
      │                       ▼
      │                map[string]string
      │                valori interpretati
      │
      ▼
nostro structural scanner
      │
      ▼
Document
(commenti, annotation,
 ordine, variabili)
      │
      └──────────┐
                 ▼
          merge tramite KEY
                 │
                 ▼
          Configuration
                 │
                 ▼
               Huh
```

Il principio è:

```text
compose-go → "che valore significa questa riga?"

nostro parser → "dove si trova questa variabile e
                 quali annotation ha?"
```

Sono responsabilità completamente diverse.

---

### Un esempio concreto

Template:

```dotenv
# @section Database

# @prompt Password del database
# @description Password utilizzata da PostgreSQL
# @secret
# @required
DB_PASSWORD="foo#bar"
```

Il nostro scanner vede semplicemente:

```go
Variable{
    Key: "DB_PASSWORD",
    Annotations: ...
}
```

Non deve chiedersi:

> `#bar` è un commento?

È compito del parser dotenv.

Il parser Compose interpreta correttamente il valore quoted secondo la sintassi `.env`. ([Docker Documentation][3])

Otteniamo:

```go
values["DB_PASSWORD"] == "foo#bar"
```

e associamo quel valore al nostro `Variable`.

---

## Come riconoscerei una variabile

La nostra parte può essere deliberatamente noiosa.

Qualcosa concettualmente equivalente a:

```go
func parseVariableLine(line string) (key string, ok bool) {
    i := strings.IndexByte(line, '=')
    if i < 0 {
        return "", false
    }

    key = strings.TrimSpace(line[:i])

    if !validKey(key) {
        return "", false
    }

    return key, true
}
```

Notare che **non tocchiamo minimamente quello che c'è dopo `=`**.

Non facciamo:

```go
value := strings.Trim(...)
```

non interpretiamo quote;

non interpretiamo escape;

non cerchiamo commenti;

non espandiamo `$FOO`.

Quella sarebbe esattamente la parte delicata che vogliamo delegare.

---

## Anche `joho/godotenv` è valido

L'altra alternativa seria è:

```go
github.com/joho/godotenv
```

È una libreria storica nell'ecosistema Go, ha `Parse(io.Reader)` e anch'essa restituisce una `map[string]string`; il repository documenta test/CI anche su Windows e Linux. ([GitHub][4])

Per un'app generica Go probabilmente sceglierei tranquillamente quella.

Nel **nostro caso specifico**, però:

```text
output → .env
consumer → Docker Compose
```

quindi trovo più coerente dipendere da:

```text
compose-spec/compose-go/v2/dotenv
```

che fa parte proprio della libreria di riferimento Compose. ([Go Packages][1])

---

## Una modifica alla nostra architettura precedente

Alla luce di questo, semplificherei anche il `Document` che avevamo immaginato.

Prima avevamo ipotizzato di gestire:

```go
QuoteStyle
SingleQuoted
DoubleQuoted
Unquoted
```

Io **non lo farei più nella prima versione**.

Terrei invece qualcosa come:

```go
type Variable struct {
    Key         string
    Value       string
    RawLine     string
    Annotations Annotations
    Line        int
}
```

Dove:

* `Key`: trovato dal nostro scanner;
* `Value`: ottenuto da `compose-go`;
* `RawLine`: originale, utile per preservare struttura;
* `Annotations`: nostre;
* `Line`: per errori decenti.

È molto meno codice.

---

## E sul writer farei una scelta analoga

Anche lì eviterei di tentare di preservare esattamente:

```dotenv
FOO='bar'
```

contro:

```dotenv
FOO="bar"
```

contro:

```dotenv
FOO=bar
```

quando l'utente modifica un valore.

Definirei **un encoding canonico per i nuovi valori**.

Per esempio potremmo decidere che il writer utilizza valori single-quoted quando serve un quoting letterale. Questo è particolarmente interessante con Compose perché i valori single-quoted sono trattati letteralmente, mentre quelli unquoted e double-quoted sono soggetti a interpolazione. ([Docker Documentation][3])

Così parser e writer hanno entrambi regole molto limitate e testabili.

---

Quindi la mia scelta oggi sarebbe:

```text
compose-spec/compose-go/v2/dotenv
             │
             │ semantic parsing
             ▼
       valori dotenv
             +
             │
             │
nostro scanner line-oriented
             │
             │ annotations + structure
             ▼
         Document
```

Secondo me **questa elimina la parte più rischiosa del progetto**. Il nostro parser custom diventerebbe probabilmente nell'ordine di 100–200 righe, non un parser dotenv completo, e soprattutto potremmo concentrarne i test su annotation e associazione commento→variabile invece che sugli infiniti edge case di quoting ed escaping.

[1]: https://pkg.go.dev/github.com/compose-spec/compose-go/v2 "compose package - github.com/compose-spec/compose-go/v2 - Go Packages"
[2]: https://pkg.go.dev/github.com/compose-spec/compose-go/v2%40v2.14.0/dotenv "dotenv package - github.com/compose-spec/compose-go/v2/dotenv - Go Packages"
[3]: https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/ "Set, use, and manage variables in a Compose file with interpolation | Docker Docs"
[4]: https://github.com/joho/godotenv/blob/main/README.md?plain=1&utm_source=chatgpt.com "godotenv/README.md at main · joho/godotenv · GitHub"

# Integrazione al piano — Strategia “non reinventare la ruota”

## Principio architetturale

Uno degli obiettivi del progetto deve essere mantenere il codice proprietario il più possibile concentrato sulle funzionalità che costituiscono realmente il valore del tool: il sistema di annotation, la costruzione del modello di configurazione, il mapping delle variabili verso le domande del wizard e il workflow di generazione del file `.env`.

Per tutte le funzionalità infrastrutturali o particolarmente soggette a edge case — parsing dotenv, interazione con il terminale, validazione di formati standard, generazione di valori casuali, gestione dei path e primitive specifiche dei sistemi operativi — si dovranno preferire la standard library di Go o librerie open source consolidate, con API stabili e licenze permissive.

L'obiettivo non è eliminare completamente il codice custom, ma evitare di implementare internamente componenti per i quali esistono già implementazioni robuste e largamente testate.

Le dipendenze esterne devono inoltre essere mantenute in numero limitato e isolate dietro componenti interni, in modo che un'eventuale sostituzione futura non richieda modifiche sostanziali al dominio applicativo.

---

# Dipendenze principali previste

La prima versione dovrebbe avere solamente poche dipendenze runtime dirette.

| Funzione                                   | Soluzione scelta                               | Licenza             |
| ------------------------------------------ | ---------------------------------------------- | ------------------- |
| Wizard terminale                           | `charm.land/huh/v2`                            | MIT                 |
| Parsing semantico `.env`                   | `github.com/compose-spec/compose-go/v2/dotenv` | Apache-2.0          |
| Primitive Windows eventualmente necessarie | `golang.org/x/sys/windows`                     | BSD-3-Clause        |
| CLI arguments                              | Go standard library `flag`                     | Go standard library |
| Validazione                                | Go standard library                            | Go standard library |
| Random/secret generation                   | Go standard library `crypto/rand`              | Go standard library |
| Path/filesystem                            | Go standard library `os`, `path/filepath`      | Go standard library |

Huh è distribuito con licenza MIT e la linea API corrente è `charm.land/huh/v2`; il progetto Charm gestisce direttamente rendering, input, select, password field e interazione con il terminale.

`compose-go` è invece la reference library Go della Compose Specification, è utilizzata anche dall'ecosistema Docker Compose ed è distribuita con licenza Apache-2.0. Il package `dotenv` espone direttamente funzioni come `Parse`, `ParseWithLookup` e `ReadFile`.

`golang.org/x/sys` fa parte dei repository supplementari mantenuti dal progetto Go e fornisce le primitive specifiche dei sistemi operativi; il modulo è distribuito con licenza BSD-style.

---

# 1. Parsing `.env`: parser semantico + scanner strutturale

Il progetto **non deve implementare un parser dotenv completo**.

La semantica del formato verrà delegata a:

```go
github.com/compose-spec/compose-go/v2/dotenv
```

Il parser Compose si occuperà quindi di interpretare correttamente valori quoted/unquoted, escaping, interpolazione e sintassi dotenv. La funzione `Parse` restituisce una `map[string]string` contenente i valori già interpretati.

Il nostro codice implementerà solamente un **structural scanner** del template.

Lo scanner deve riconoscere:

```text
commento
annotation
riga vuota
variabile
```

senza interpretare il valore della variabile.

Esempio:

```dotenv
# Database configuration

# @prompt Database host
# @required
DB_HOST="postgres"

# @secret
DB_PASSWORD=
```

Il nostro scanner produce:

```text
Comment
BlankLine
Annotation
Annotation
Variable(DB_HOST)
BlankLine
Annotation
Variable(DB_PASSWORD)
```

mentre:

```go
dotenv.Parse(...)
```

fornisce:

```go
map[string]string{
    "DB_HOST":     "postgres",
    "DB_PASSWORD": "",
}
```

I due risultati vengono poi associati tramite il nome della variabile.

Questa separazione è fondamentale:

```text
compose-go
    ↓
semantica dotenv

nostro scanner
    ↓
struttura + annotation

        ↓

Document
```

Il parser Compose stesso gestisce dettagli quali `export`, delimitatori, quoted values, commenti inline e interpolazione; questi aspetti non devono quindi essere duplicati nel nostro scanner.

---

# 2. Definire intenzionalmente un subset per `.env.example`

Anche se Compose supporta una sintassi piuttosto ampia, il nostro template non deve necessariamente accettarla tutta.

Per la prima versione è preferibile definire un formato intenzionalmente ristretto:

```dotenv
KEY=
KEY=value
KEY="value"
KEY='value'
```

con:

```dotenv
# commenti

# @annotation ...
```

Il template v1 dovrebbe invece evitare:

```dotenv
KEY: value
```

```dotenv
export KEY=value
```

valori multilinea e commenti normali sulla stessa riga della variabile.

Compose supporta alcune di queste forme, compresi `:` come delimitatore e valori single-quoted multilinea, ma supportarle nel nostro structural scanner aumenterebbe notevolmente la complessità senza fornire un beneficio reale per il caso d'uso iniziale.

Questa limitazione riguarda **il formato dei template accettati dal wizard**, non il formato `.env` in generale.

Il programma deve produrre un errore chiaro quando incontra una costruzione valida per Compose ma non supportata dal formato template del progetto.

---

# 3. Preservazione del documento

Il file `.env` non deve essere rigenerato partendo dalla `map` restituita dal parser.

La sorgente strutturale rimane sempre `.env.example`.

Il modello interno conserverà almeno:

```go
type Variable struct {
    Key         string
    Value       string
    RawLine     string
    Line        int
    Annotations Annotations
}
```

insieme a nodi dedicati a:

```text
Comment
BlankLine
Annotation
Variable
```

Questo permette di preservare:

- ordine delle variabili;
- commenti normali;
- righe vuote;
- suddivisione visiva del template;
- line ending originali.

Durante la generazione del `.env` vengono modificate solamente le righe contenenti variabili.

Le annotation vengono invece eliminate dal file finale.

---

# 4. Writer dotenv: encoder piccolo ma verificato dal parser Compose

`compose-go/dotenv` fornisce un parser ma non un'API pubblica equivalente a un `Marshal` per ricreare il documento.

Il writer sarà quindi una delle poche parti deliberate che implementeremo direttamente, ma dovrà avere uno scope estremamente limitato:

```go
func EncodeValue(value string) string
```

Il writer dovrà produrre una rappresentazione canonica Compose-compatible del valore, senza tentare di riprodurre esattamente lo stile di quoting originale.

Questo evita di dover mantenere logiche complesse come:

```text
preserva single quote
preserva double quote
preserva whitespace originale
preserva escaping originale
```

che non portano valore funzionale.

Particolare attenzione deve essere data al carattere `$`: Compose applica l'interpolazione ai valori unquoted e double-quoted, mentre i valori single-quoted sono trattati letteralmente.

La sicurezza principale dell'encoder sarà però un'altra: **round-trip validation**.

Per ogni valore generato dai test:

```text
valore originale
      ↓
EncodeValue
      ↓
KEY=<encoded>
      ↓
compose-go/dotenv.Parse
      ↓
valore risultante
```

deve valere:

```text
original == parsed
```

In questo modo utilizziamo lo stesso parser che userà la nostra applicazione come oracle per verificare il writer.

Dovranno esistere test specifici per:

```text
stringhe vuote
spazi
#
$
"
'
\
newline
URL
password
Unicode
= nel valore
```

---

# 5. Existing `.env`: stesso parser, nessuna seconda implementazione

Anche il `.env` già esistente deve essere letto tramite:

```go
compose-go/v2/dotenv
```

Non deve esistere un secondo parser dedicato.

La precedenza rimane:

```text
risposta wizard
    ↓
.env esistente
    ↓
.env.example
```

con l'eccezione:

```text
@fixed
→ sempre valore del template
```

In questo modo il tool usa esattamente la stessa semantica dotenv sia per il template sia per il file esistente.

---

# 6. Duplicati e configurazioni ambigue

Poiché la struttura del wizard richiede una relazione univoca tra variabile e domanda, il template deve rifiutare variabili duplicate.

Questo:

```dotenv
DB_HOST=localhost

DB_HOST=postgres
```

deve essere considerato un errore del template anche se un parser dotenv potrebbe tecnicamente produrre un risultato.

Output previsto:

```text
Invalid template

line 14: duplicate variable DB_HOST
previous declaration: line 7
```

L'obiettivo è evitare comportamenti impliciti e mantenere deterministica la relazione:

```text
Variable → Question
```

---

# 7. Validazione: standard library prima di librerie esterne

Non verrà introdotto un framework generico di validazione nella prima versione.

I tipi previsti dalle annotation devono utilizzare direttamente le primitive della standard library.

Esempi:

```text
int
→ strconv.ParseInt

bool
→ strconv.ParseBool

url
→ net/url

ip
→ net/netip

duration
→ time.ParseDuration

regexp
→ regexp

port
→ strconv + controllo range
```

Il progetto aggiungerà solamente un sottile layer:

```go
func ValidatePort(string) error
func ValidateURL(string) error
func ValidateInt(string) error
```

Gli stessi validator saranno utilizzabili:

- dal wizard Huh;
- dai test;
- da una futura modalità `validate`;
- da una futura modalità non interattiva.

Non deve quindi esistere logica di validazione direttamente nei componenti Huh.

---

# 8. Secret: primitive crittografiche standard

Se in futuro verrà implementata:

```text
@generate
```

la generazione di password, token o secret deve utilizzare esclusivamente:

```go
crypto/rand
```

e primitive standard come:

```go
encoding/hex
encoding/base64
```

Non devono essere implementati generatori pseudo-random custom.

Inoltre i secret devono seguire una policy trasversale:

```text
mai nei log
mai nei messaggi di errore
mai nel summary
mai nei dump di debug
```

Nel riepilogo:

```text
DB_PASSWORD    [set]
```

e non:

```text
DB_PASSWORD    mypassword
```

né una quantità di `*` che riveli la lunghezza.

Va inoltre documentato che il file `.env` non è un secret store: Docker raccomanda di non usare variabili d'ambiente per informazioni particolarmente sensibili quando è possibile utilizzare Docker Secrets.

---

# 9. Filesystem e path: solo standard library

La gestione dei path deve usare esclusivamente:

```go
path/filepath
os
io
```

Non devono essere costruiti path tramite concatenazione manuale.

Quindi:

```go
filepath.Join(...)
filepath.Abs(...)
filepath.Clean(...)
os.UserHomeDir()
os.CreateTemp(...)
```

invece di logica basata su:

```text
/
\
C:\
```

Questo mantiene il codice comune tra Windows e Linux.

---

# 10. Scrittura sicura del file

Il `.env` non deve essere scritto direttamente.

La pipeline prevista deve essere:

```text
render completo
      ↓
temporary file nella stessa directory
      ↓
write
      ↓
Sync
      ↓
Close
      ↓
replace file finale
```

Se qualcosa fallisce prima della sostituzione finale, il `.env` esistente rimane invariato.

La parte comune utilizzerà la standard library.

La differenza Windows/Unix verrà isolata dietro:

```go
func replaceFile(temp, target string) error
```

con file specifici:

```text
replace_unix.go
replace_windows.go
```

Su Windows, se avremo bisogno della primitiva nativa di sostituzione, utilizzeremo `golang.org/x/sys/windows` anziché implementare syscall manualmente. Il package espone `MoveFileEx` e i flag `MOVEFILE_REPLACE_EXISTING` e `MOVEFILE_WRITE_THROUGH`.

Questo mantiene la parte OS-specific ridotta a poche righe e basata sull'implementazione mantenuta dal progetto Go.

La semantica esatta dell'atomicità dovrà comunque essere testata e documentata come dipendente dalle garanzie fornite dal filesystem sottostante.

---

# 11. Terminale: Huh deve essere un adapter, non il dominio

Huh non deve comparire nei package che rappresentano la configurazione.

Il dominio definirà:

```go
type Question struct {
    Key         string
    Prompt      string
    Description string
    Value       string
    Kind        QuestionKind
    Required    bool
    Secret      bool
    Options     []string
}
```

e solamente:

```text
internal/wizard
```

conoscerà Huh.

La pipeline sarà:

```text
Variable
   ↓
Question
   ↓
Huh Field
```

Questo è particolarmente importante perché la linea v2 di Huh è l'API corrente ma è stata introdotta come nuova major release; mantenendo un adapter stretto, eventuali cambiamenti futuri nella libreria UI rimangono confinati a un singolo package.

---

# 12. CLI: niente framework finché non serve

Nella v1 verrà utilizzato:

```go
flag
```

della standard library.

Sono sufficienti:

```text
--template
--output
--force
--version
```

Non verrà introdotto Cobra solamente per gestire quattro flag.

Se in futuro la CLI evolve verso:

```text
env-wizard init
env-wizard validate
env-wizard update
env-wizard doctor
```

allora dovrà essere rivalutato l'uso di Cobra anziché costruire internamente un sistema di command routing.

Cobra è un progetto open source ampiamente utilizzato e distribuito con licenza Apache-2.0.

Quindi:

```text
v1 → standard flag

CLI complessa futura → Cobra
```

e non un framework CLI proprietario.

---

# 13. Logging ed error handling

Per logging e diagnostica verranno utilizzati:

```go
errors
fmt
log/slog
```

della standard library.

Non è prevista una dipendenza dedicata al logging.

Gli errori interni dovranno essere wrapped:

```go
fmt.Errorf("parse template: %w", err)
```

mentre l'output destinato all'utente verrà trasformato in messaggi leggibili dall'application layer.

Particolare attenzione deve essere prestata a non inserire valori delle variabili negli errori quando il campo è marcato `@secret`.

---

# 14. Testing: standard library come default

Il progetto utilizzerà inizialmente:

```go
testing
```

senza introdurre framework di test.

La parte più importante sarà rappresentata da table-driven tests e golden files.

Esempio:

```text
testdata/
├── basic.env.example
├── comments.env.example
├── annotations.env.example
├── quoting.env.example
├── crlf.env.example
└── invalid/
```

I test principali saranno:

```text
structural parsing
semantic parsing
annotation binding
validation
writer round-trip
merge
existing .env
LF / CRLF
secret handling
atomic write
Windows/Linux
```

Una dipendenza come `google/go-cmp` potrà essere introdotta solo se la comparazione dei modelli diventerà sufficientemente complessa da giustificarla; il progetto è BSD licensed, ma non è necessaria per partire.

---

# 15. Controllo delle dipendenze

Ogni nuova dipendenza runtime deve essere considerata una decisione architetturale.

Prima di aggiungerla bisogna verificare:

```text
problema che risolve
maturità
attività del progetto
versioning
licenza
numero di dipendenze transitive
possibilità di usare la standard library
possibilità di isolarla dietro un adapter
```

La regola generale sarà:

```text
standard library
    ↓ se insufficiente
progetto consolidato e permissivo
    ↓ se insufficiente
codice custom piccolo e testato
```

e non:

```text
cerco una libreria per ogni funzione
```

---

# 16. Vulnerability scanning

La CI dovrà eseguire:

```bash
govulncheck ./...
```

`govulncheck` è il tool ufficiale dell'ecosistema Go per verificare se il codice utilizza funzioni affette da vulnerabilità note e si basa sul Go Vulnerability Database.

Checklist:

- [ ] eseguire `go test ./...`
- [ ] eseguire `go vet ./...`
- [ ] eseguire `govulncheck ./...`
- [ ] eseguire build Windows
- [ ] eseguire build Linux

---

# 17. Controllo delle licenze delle dipendenze

Dato l'obiettivo di pubblicare il progetto come open source, la CI dovrebbe anche controllare le licenze introdotte dalle dipendenze.

Come tooling può essere utilizzato:

```text
google/go-licenses
```

che genera report sulle licenze dei package Go e delle relative dipendenze. La versione v2 è attualmente la linea stabile del progetto ed è essa stessa distribuita sotto Apache-2.0.

Il repository dovrebbe mantenere una policy esplicita, ad esempio accettando automaticamente licenze permissive note come:

```text
MIT
BSD-2-Clause
BSD-3-Clause
Apache-2.0
ISC
```

mentre una nuova dipendenza con licenza differente richiede una revisione esplicita.

Questo controllo non sostituisce una verifica legale quando necessaria, ma impedisce che una dipendenza incompatibile venga aggiunta accidentalmente.

---

# 18. Licenza del progetto

Per questo progetto utilizzerei come prima scelta:

```text
Apache-2.0
```

È una licenza permissiva, permette uso, modifica e distribuzione del software e contiene anche una concessione esplicita dei diritti brevettuali da parte dei contributor.

Una valida alternativa, ancora più semplice, è:

```text
MIT
```

che permette uso, modifica, distribuzione e sublicensing richiedendo essenzialmente il mantenimento dell'avviso di copyright e della licenza.

Per un progetto pubblico destinato potenzialmente ad avere contributor esterni, la preferenza architetturale sarebbe quindi:

```text
Apache-2.0
```

mentre MIT rimane una scelta perfettamente ragionevole se si desidera la licenza più semplice possibile.

Questa è una scelta progettuale e non costituisce consulenza legale.

---

# 19. Third-party notices

La licenza scelta per il nostro codice non elimina gli obblighi relativi alle dipendenze.

In particolare `compose-go` utilizza Apache-2.0 e il repository contiene anche un file `NOTICE`.

Come policy del progetto distribuiremo quindi insieme ai release artifact:

```text
LICENSE
THIRD_PARTY_NOTICES
```

oppure una directory:

```text
licenses/
```

contenente le attribuzioni necessarie.

Apache-2.0 specifica requisiti di conservazione della licenza, delle attribuzioni pertinenti e degli eventuali NOTICE applicabili alla redistribuzione.

La generazione di questo materiale potrà essere automatizzata durante la release tramite `go-licenses`.

---

# 20. Nuova struttura dei package

Alla luce di queste decisioni, la struttura proposta diventa:

```text
env-wizard/
│
├── cmd/
│   └── env-wizard/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   └── app.go
│   │
│   ├── domain/
│   │   ├── document.go
│   │   ├── annotations.go
│   │   └── question.go
│   │
│   ├── dotenv/
│   │   ├── semantic.go
│   │   ├── structure.go
│   │   ├── encoder.go
│   │   └── merge.go
│   │
│   ├── wizard/
│   │   ├── wizard.go
│   │   ├── huh_adapter.go
│   │   └── summary.go
│   │
│   ├── validation/
│   │   └── validation.go
│   │
│   └── filesystem/
│       ├── writer.go
│       ├── replace_unix.go
│       └── replace_windows.go
│
├── testdata/
│
├── LICENSE
├── THIRD_PARTY_NOTICES
├── go.mod
├── go.sum
└── README.md
```

Le dipendenze devono restare ai bordi:

```text
                DOMAIN
                  │
        ┌─────────┼──────────┐
        │         │          │
        ▼         ▼          ▼
   compose-go     Huh     filesystem
     adapter     adapter      adapter
```

Il dominio non importa direttamente nessuna di queste librerie.

---

# 21. Checklist aggiuntiva — “non reinventare la ruota”

## Dependency policy

- [ ] Documentare la dependency policy
- [ ] Preferire la standard library
- [ ] Limitare le dipendenze runtime dirette
- [ ] Verificare licenza di ogni nuova dipendenza
- [ ] Isolare le dipendenze dietro package interni
- [ ] Mantenere `go.mod` e `go.sum`
- [ ] Evitare dipendenze non necessarie

## Dotenv

- [ ] Integrare `compose-go/v2/dotenv`
- [ ] Usarlo per il parsing semantico del template
- [ ] Usarlo per il parsing del `.env` esistente
- [ ] Implementare solamente lo structural scanner
- [ ] Definire formalmente il subset template supportato
- [ ] Rifiutare duplicate key
- [ ] Rifiutare sintassi template non supportata
- [ ] Preservare commenti normali
- [ ] Preservare blank lines
- [ ] Preservare LF/CRLF
- [ ] Rimuovere annotation dall'output

## Writer

- [ ] Definire encoding canonico
- [ ] Non tentare di preservare quoting originale
- [ ] Testare `$` e interpolazione
- [ ] Testare quote e backslash
- [ ] Implementare round-trip con `compose-go`
- [ ] Aggiungere property/table tests sull'encoder

## Filesystem

- [ ] Scrivere prima su temporary file
- [ ] Eseguire Sync prima del replace
- [ ] Implementare adapter `replaceFile`
- [ ] Implementazione Unix
- [ ] Implementazione Windows
- [ ] Usare `x/sys/windows` se necessarie primitive Windows
- [ ] Testare overwrite
- [ ] Testare failure prima del replace
- [ ] Verificare che il file precedente rimanga integro

## Security

- [ ] Centralizzare secret handling
- [ ] Nessun secret nei log
- [ ] Nessun secret negli errori
- [ ] Nessun secret nel summary
- [ ] Usare `crypto/rand` per future generazioni
- [ ] Aggiungere test specifici per secret

## Validation

- [ ] `strconv` per numeri
- [ ] `strconv` per bool
- [ ] `net/url` per URL
- [ ] `net/netip` per IP
- [ ] `time.ParseDuration` per duration
- [ ] `regexp` per pattern
- [ ] Nessuna libreria validation generica iniziale

## CLI

- [ ] Usare `flag` nella v1
- [ ] Non introdurre Cobra prematuramente
- [ ] Rivalutare Cobra solo con subcommand reali

## Supply chain

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `govulncheck ./...`
- [ ] controllo licenze con `go-licenses`
- [ ] build CI Windows
- [ ] build CI Linux
- [ ] mantenere third-party notices
- [ ] verificare dipendenze prima delle release

## Open source readiness

- [ ] scegliere Apache-2.0 o MIT
- [ ] aggiungere `LICENSE`
- [ ] aggiungere `THIRD_PARTY_NOTICES`
- [ ] aggiungere `CONTRIBUTING.md`
- [ ] aggiungere `SECURITY.md`
- [ ] documentare dependency policy
- [ ] documentare formato delle annotation
- [ ] documentare subset `.env.example` supportato
```

