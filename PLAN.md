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
| `@section valore`                    | Apre o riapre un gruppo/pagina del wizard. Le occorrenze omonime vengono unite; senza section si usa `Configuration`. |

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
- una variabile con `@options` deve avere nel template un valore non vuoto appartenente alle option; un valore vuoto non rappresenta una selezione valida nella v1;
- una stessa annotation non può comparire due volte nello stesso blocco di campo;
- le annotation di campo devono essere seguite immediatamente da una variabile, senza righe vuote o commenti normali intermedi;
- `@section` aggiorna il contesto del documento e può restare senza una variabile successiva;
- i nomi delle section sono case-sensitive e le occorrenze con lo stesso nome confluiscono nello stesso gruppo logico;
- l'ordine dei gruppi è determinato dalla prima occorrenza del nome, mentre l'ordine delle domande dentro ogni gruppo segue l'ordine delle relative variabili nel documento;
- una section che non contiene alcuna variabile configurabile non genera una pagina vuota;
- il gruppo implicito `Configuration` può essere riaperto esplicitamente con `@section Configuration`.

Le annotation sono metadati del wizard, ma restano commenti dotenv utili anche
per chi modifica manualmente il file: annotation e commenti normali vengono
quindi conservati nel `.env` generato nelle posizioni originali.

Un template valido deve definire almeno una variabile. Un file vuoto o composto soltanto da commenti, annotation di section e righe vuote è un errore di configurazione prima del wizard. Un template che contiene variabili ma le marca tutte `@fixed` è invece valido: non genera domande o pagine vuote, salta l'esecuzione del form e prosegue con riepilogo, rendering, rilevamento no-op, conferma e scrittura. Anche in questo caso la v1 continua a richiedere un terminale e `--force` non abilita l'uso non interattivo.

### 2.2 Regole di default

Senza annotation, ogni variabile configurabile deve:

- diventare una domanda;
- usare il nome della chiave come prompt;
- usare il valore del template come default;
- considerare il valore vuoto come assenza di default;
- permettere all'utente di confermare il default semplicemente proseguendo.

Le annotation modificano questo comportamento solo quando esplicitamente presenti. Non devono essere introdotte annotation ridondanti come `@default`, `@optional`, `@string` o `@editable`.

Per distinguere un valore realmente vuoto da un valore non ancora impostato, il modello non deve affidarsi alla sola stringa `Value`. Ogni valore/question deve esporre almeno uno stato equivalente a `HasValue` oppure `ValueSource`. Le sorgenti previste sono `template`, `existing`, `user` e `fixed`.

Per `@type bool` la v1 accetta esclusivamente `true` e `false`, senza distinzione tra maiuscole e minuscole, e il writer serializza sempre `true` oppure `false`. Il template deve contenere un default booleano non vuoto. Un booleano vuoto o invalido nel template è un errore prima della creazione della domanda; lo stesso valore proveniente da un `.env` esistente viene invece trattato come valore corrente incompatibile e segue il flusso di recupero descritto nel §5.3.

Per `@type int` la v1 accetta esclusivamente interi decimali con segno opzionale nell'intervallo `int64`; per `@type port` accetta esclusivamente cifre decimali e impone l'intervallo `1..65535`. Whitespace, frazioni, notazione scientifica, prefissi esadecimali e altri separatori non sono validi. Gli zeri iniziali sono ammessi e il valore testuale valido viene conservato senza normalizzazione. Per entrambi i tipi il valore vuoto resta valido salvo la presenza separata di `@required`; la validazione usa limiti espliciti e non dipende dalla dimensione di `int` della piattaforma.

Per `@type url` la v1 valida URI assoluti generici, non soltanto URL HTTP. Un valore non vuoto deve avere uno schema sintatticamente valido e contenuto significativo dopo lo schema, espresso come host, path oppure parte opaca. Sono quindi ammessi schemi standard e personalizzati, inclusi casi come `https://example.com`, `postgres://user:pass@db:5432/app`, `unix:///var/run/app.sock` e `mailto:user@example.com`; vengono invece rifiutati riferimenti relativi come `example.com/api`, testo privo di schema, whitespace o caratteri di controllo e forme sintatticamente malformate. Userinfo, porta, query e fragment sono consentiti. La validazione non normalizza il valore e non esegue DNS lookup, controlli di raggiungibilità o connessioni. Il valore vuoto resta valido salvo la presenza separata di `@required`.

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
- valori semantici contenenti byte NUL, `CR` o `LF`, anche se rappresentati tramite escape in una riga quoted;
- commenti inline non quotati sulla stessa riga della variabile; un `#` dentro un valore quoted è parte del valore;
- variabili duplicate.

Le chiavi duplicate sono un errore anche nel `.env` esistente, perché il merge deve avere una sorgente univoca. Il file esistente viene quindi sottoposto a un controllo strutturale minimo delle chiavi, oltre al parsing semantico con `compose-go`.

Il subset ristretto sopra si applica al template, non al `.env` esistente. Quest'ultimo è soltanto una sorgente di valori e può usare tutta la sintassi accettata da `compose-go`, con lookup esterno controllato e vuoto; non è necessario preservarne struttura o quoting. Deve comunque essere un file UTF-8 semanticamente valido e privo di chiavi duplicate. Un errore che impedisce di interpretare il file termina l'esecuzione senza scritture; un singolo valore interpretato ma non rappresentabile o non valido secondo il nuovo template, compreso un valore multilinea, segue invece il recupero nel wizard descritto nel §5.3. Le chiavi obsolete restano ammesse in lettura ma vengono ignorate dal merge.

Il subset v1 usa la grammatica di chiave `[A-Za-z_][A-Za-z0-9_.-]*`. Le chiavi sono case-sensitive; `FOO` e `foo` sono quindi variabili distinte. Non sono supportate chiavi che iniziano con un numero, parentesi quadre o altre forme estese accettate da Compose.

La v1 non supporta l'interpolazione come funzionalità del template. Sequenze come `$VAR` e `${VAR}` sono ammesse nei default soltanto quando la sintassi sorgente le rende letterali, per esempio tramite single quote; le stesse sequenze in valori unquoted o double-quoted vengono rifiutate. I valori prodotti dal writer trattano sempre ogni `$` come testo letterale: `$VAR`, `${VAR}` e `$$` devono essere riletti rispettivamente come le stesse sequenze di caratteri, senza dipendere dall'ambiente del processo.

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

Lo scanner non deve interpretare escape o il significato semantico dei valori dopo il primo `=`. Può tuttavia mantenere lo stato lessicale minimo delle quote necessario a rifiutare le forme escluse dal subset v1, come commenti inline non quotati, interpolazione attiva in valori unquoted o double-quoted, delimitatore `:` e valori multilinea. La successiva validazione semantica rifiuta inoltre valori già interpretati che contengono NUL, `CR` o `LF`.

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

Le annotation consecutive si applicano alla prima variabile immediatamente successiva. Una riga vuota o un commento normale interrompono il blocco di annotation. Una nuova `@section` aggiorna il contesto documentale e non viene associata alla variabile successiva come metadato di campo. Section non contigue con lo stesso nome vengono unite soltanto nel modello delle domande: il documento e il writer mantengono l'ordine originale dei nodi e delle variabili. Annotation duplicate, sconosciute o prive del valore richiesto sono errori di template. Una annotation di campo senza variabile immediatamente successiva è un errore.

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

Il modulo dichiara `go 1.26.0` come versione minima e non usa funzionalità che richiedono Go 1.27 nella v1. La CI verifica le ultime patch disponibili delle linee 1.26 e 1.27; gli artifact ufficiali vengono compilati con l'ultima patch disponibile della linea 1.27. In questo modo l'interfaccia del modulo resta compatibile con entrambe le linee Go supportate al momento della progettazione.

Ogni nuova dipendenza deve essere valutata per maturità, attività del progetto, politica di versionamento, vulnerabilità note, numero di dipendenze transitive, possibilità di usare la standard library e possibilità di isolamento dietro un adapter. Le dipendenze runtime dirette devono restare limitate a quelle che forniscono capacità sostanziali non disponibili nella standard library.

La policy delle licenze ammette automaticamente dipendenze sotto Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause o ISC. Qualunque altra licenza richiede una revisione esplicita prima dell'introduzione. CI e processo di release devono verificare vulnerabilità e licenze e includere negli artifact `LICENSE` e le attribuzioni necessarie in `THIRD_PARTY_NOTICES`.

### 3.4 Writer canonico

Il writer non deve cercare di preservare lo stile originale di quoting quando un valore viene modificato. Deve usare un encoding canonico Compose-compatible e verificarlo tramite round-trip con `compose-go`:

```text
value
  ↓ EncodeValue
KEY=<encoded>
  ↓ dotenv.Parse
value originale
```

L'interfaccia dell'encoder accetta valori UTF-8 validi su una sola riga e rifiuta NUL, `CR` e `LF`. Per ogni valore accettato deve risultare `original == parsed`. Particolare attenzione va riservata a stringhe vuote, whitespace, `#`, `$`, sequenze simili a interpolazioni, quote, apostrofi, backslash, Unicode e `=`. I valori booleani vengono serializzati sempre come `true` o `false`.

### 3.5 Scrittura sicura

Il file finale non deve essere scritto direttamente. La pipeline deve essere:

```text
render completo
    ↓
file temporaneo nella stessa directory
    ↓
Write + Sync + Close
    ↓
se il target esiste: copia di backup + Sync + Close
    ↓
replace del target
```

Quando il target esiste, prima del replace il modulo filesystem deve copiarne byte per byte il contenuto in un nuovo file nella stessa directory, sincronizzarlo e chiuderlo. Il nome è `<output>.backup-<UTC>`, con timestamp UTC nel formato `YYYYMMDDTHHMMSSZ`, per esempio `.env.backup-20260830T143015Z`; in caso di collisione aggiunge `-1`, `-2` e così via usando creazione esclusiva, senza sovrascrivere backup precedenti. La v1 crea sempre questo backup durante un overwrite effettivo, anche con `--force`, non lo crea per un output nuovo e non elimina automaticamente i backup storici.

Se la creazione o la sincronizzazione del backup fallisce, l'operazione termina senza toccare il target. Il vecchio file deve rimanere intatto anche se un'operazione fallisce prima del replace. L'intero flusso deve restare dietro una singola interfaccia del modulo filesystem; `replaceFile` è un dettaglio interno con adapter specifici per Unix e Windows. La v1 privilegia l'integrità del contenuto e usa un temporaneo nella stessa directory; su Windows usa `MoveFileExW` tramite `golang.org/x/sys/windows`, mentre la preservazione completa di ACL e metadata è rinviata a `FUT-005`. L'atomicità e la durabilità dipendono comunque dal filesystem e non sono garantite in modo assoluto su share di rete o filesystem particolari.

Su Unix, un nuovo output e ogni backup vengono creati con permessi `0600`. Durante un overwrite, il nuovo output conserva invece esattamente i bit di permesso Unix del target precedente, applicandoli al temporaneo prima del replace, così un accesso di gruppo configurato intenzionalmente non viene interrotto. La v1 non garantisce la conservazione di proprietario, gruppo, ACL, timestamp o altri metadata. Su Windows, il nuovo file usa gli ACL derivati dalla directory e non è garantita la conservazione esatta degli ACL del target precedente.

Il backup contiene gli stessi eventuali secret del `.env`: il percorso creato deve essere comunicato all'utente e la documentazione deve raccomandare di ignorare `.env*` nel versionamento. Opzioni per disabilitazione e retention sono rinviate a `FUT-004`.

### 3.6 Piattaforme supportate

La v1 offre supporto stabile per Windows amd64, Linux amd64 e Linux arm64. Gli artifact macOS amd64 e arm64 vengono pubblicati come **preview**: devono compilare ed eseguire l'intera suite su runner macOS nativi Intel e Apple Silicon, ma non vengono dichiarati stabili finché non superano anche uno smoke test manuale in un vero terminale macOS. Windows arm64 non fa parte della v1.

Una piattaforma non viene dichiarata supportata sulla sola base di una cross-compilazione. Gli adapter specifici del sistema operativo e i test di filesystem devono essere eseguiti nativamente; le limitazioni della verifica manuale macOS devono essere dichiarate nel README e nelle note di release.

### 3.7 Distribuzione della v1

La v1 viene distribuita come singolo eseguibile standalone tramite GitHub Releases, senza installer o script remoti da eseguire via shell. Gli artifact Windows usano `.zip`; Linux e macOS usano `.tar.gz`. Ogni archivio contiene il binario, `LICENSE`, `THIRD_PARTY_NOTICES` e il README, ed è coperto da un file release `SHA256SUMS`.

I nomi seguono lo schema `env-wizard_<version>_<os>_<arch>.<ext>`, per esempio `env-wizard_1.0.0_windows_amd64.zip` e `env-wizard_1.0.0_linux_arm64.tar.gz`. Per chi possiede una toolchain Go supportata, il README documenta anche `go install github.com/tosdan/env-setup-wizard/cmd/env-wizard@latest` e la variante con versione esplicita. Launcher `npx` e package manager sono adapter di distribuzione futuri e non devono introdurre dipendenze npm nel motore Go (`FUT-006`); firme e attestazioni oltre SHA-256 sono rinviate a `FUT-007`.

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

Il modulo Go usa il path canonico lowercase `github.com/tosdan/env-setup-wizard`; il comando pubblico e gli artifact eseguibili usano il nome `env-wizard` (`env-wizard.exe` su Windows).

Le release seguono Semantic Versioning con tag Git `vMAJOR.MINOR.PATCH`. Le release candidate della prima versione usano `v1.0.0-rc.N` e la prima release completa è `v1.0.0`; in seguito una correzione compatibile incrementa PATCH e una nuova funzionalità compatibile incrementa MINOR. Per una release, `--version` stampa `env-wizard v1.0.0 (commit abc1234)` usando la versione e il commit breve iniettati tramite linker. Una build locale non versionata stampa `env-wizard dev`. La data di compilazione non viene incorporata, per non compromettere la riproducibilità degli artifact.

Default:

```text
template = .env.example
output   = .env
```

Eseguendo il comando senza argomenti, entrambi i path vengono risolti rispetto alla cartella corrente del processo: il template è `<cwd>/.env.example` e l'output è `<cwd>/.env`. I flag `--template` e `--output` sostituiscono singolarmente questi default.

Flag v1:

```text
--template PATH
--output PATH
--force
--version
```

Il wizard mostra sempre il riepilogo finale. Il risultato viene poi renderizzato in memoria e, se il target esistente è byte-identico, il comando stampa `No changes detected.`, restituisce `0` e termina senza conferma, backup o scrittura. Soltanto quando il contenuto è nuovo o differente, senza `--force` chiede una sola conferma: se il target non esiste usa `Create .env? [Y/n]`, con risposta predefinita positiva; se il target esiste usa `Overwrite existing .env? [y/N]`, con risposta predefinita negativa.

`--force` salta questa conferma finale sia in creazione sia in sovrascrittura, ma non deve bypassare il wizard, il riepilogo, il rilevamento no-op, la validazione del template o le validazioni dei valori.

Prima del wizard, i path del template e dell'output vengono convertiti in path assoluti e normalizzati. Il template deve esistere e risolversi a un file regolare leggibile; può essere esso stesso un symlink. La directory padre dell'output deve già esistere ed essere una directory: la v1 non crea automaticamente directory mancanti.

Se l'output esiste, deve essere un file regolare e non può essere un symlink. Template e output non possono identificare lo stesso file fisico, neppure tramite symlink, hardlink o differenze di maiuscole/minuscole su filesystem case-insensitive. Quando entrambi esistono, il confronto deve usare anche l'identità fornita dal filesystem, non soltanto le stringhe normalizzate.

Gli stessi controlli vengono ripetuti immediatamente prima della scrittura atomica. Se nel frattempo l'output è stato sostituito, è diventato un symlink o coincide con il template, il comando termina senza creare o modificare file.

La v1 è interattiva e richiede un terminale. Se stdin o il terminale necessario non sono disponibili, il comando termina prima di creare o modificare l'output con un messaggio chiaro su stderr. `--force` non abilita una modalità non interattiva.

Exit code v1:

```text
0      file creato o aggiornato; nessuna modifica rilevata; --help; --version; rifiuto della conferma finale
1      template o valori invalidi; errore di filesystem o altro errore operativo
2      argomenti o flag CLI invalidi
130    Ctrl+C o annullamento del wizard
```

Il rifiuto della conferma finale è una conclusione normale: non modifica l'output, stampa `No changes made.` e restituisce `0`. Un'interruzione durante il wizard è invece distinta dagli errori e restituisce `130`. L'application layer non deve usare `log.Fatal`, così codici e messaggi restano testabili attraverso la stessa interfaccia usata dal comando.

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
load existing .env with compose-go, if present
      ↓
merge defaults
      ↓
create questions; require at least one template variable
      ↓
run wizard if at least one configurable question exists
      ↓
show summary
      ↓
render candidate output in memory
      ↓
if byte-identical to existing output: report no changes and stop
      ↓
confirm create or overwrite, unless --force
      ↓
safe write, including backup before an effective overwrite
```

La validazione completa del template deve avvenire prima di mostrare il wizard. Un template errato è un errore dello sviluppatore e deve terminare l'esecuzione con causa leggibile e numero di riga quando l'errore è associabile a una riga; errori globali come l'assenza di variabili identificano invece il file senza inventare una riga.

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

Un valore proveniente dal `.env` esistente che non rispetta più il template è un problema recuperabile, non un errore fatale. Il valore incompatibile non viene usato come valore iniziale della domanda: il campo riparte dal default valido del template e riceve una diagnostica associata che informa l'utente della necessità di confermare o fornire un valore valido.

Per i campi non secret, la diagnostica può mostrare il valore incompatibile. Per un campo `@secret` deve usare soltanto un messaggio generico e non deve rivelare il valore né la sua lunghezza. Questa policy si applica a tutte le validazioni, compresi `@required`, `@type` e `@options`.

Il wizard deve quindi permettere di correggere la configurazione esistente. Il valore finale deve superare la validazione del campo prima di proseguire. Se l'utente annulla, il `.env` originale resta invariato; se completa il wizard, il nuovo valore viene scritto soltanto dopo il riepilogo e la conferma finale. Prima di un overwrite effettivo, la versione precedente viene conservata nel backup datato descritto nel §3.5.

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
      ExistingValueIssue *ExistingValueIssue
}
```

`ExistingValueIssue` rappresenta una diagnostica già resa sicura dal modulo di configurazione. L'adapter Huh deve soltanto visualizzarla e non deve conoscere o ripetere la logica con cui un valore esistente viene dichiarato incompatibile. Quando la diagnostica riguarda un secret, non contiene il valore originale.

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
| `@type bool` con valore vuoto nel template                      | Errore prima del wizard.                                                             |
| `@type bool` con valore vuoto o invalido nel file esistente     | Diagnostica recuperabile e fallback al default valido del template.                  |
| option contenente virgole                                       | Non supportata: la virgola è il solo separatore della v1.                            |

La validazione di `@required` viene applicata al valore finale dopo il merge e dopo la risposta dell'utente. `@fixed` usa sempre il valore del template. `@secret` può essere combinata con `@fixed`, ma non con `@options`.

### 5.5 Summary e secret handling

Il riepilogo deve essere raggruppato usando le stesse section unite del wizard e
deve mostrare i valori normali. Quando esiste `@prompt`, l'etichetta usa
`Prompt descrittivo (VARIABLE_KEY)`; senza `@prompt` usa soltanto la chiave. Per
i secret deve mostrare esclusivamente il relativo stato, per esempio:

```text
DB_PASSWORD    [set]
```

Un secret vuoto viene mostrato come `[not set]`. Non deve mai essere rivelato il contenuto né la lunghezza del valore. La stessa policy vale per log, errori, dump di debug e messaggi di cancellazione.

`@secret` protegge soltanto l'esperienza del wizard: il valore viene comunque scritto nel file `.env`. Il progetto deve documentare che `.env` non è un secret store e raccomandare Docker Secrets o un sistema equivalente per credenziali particolarmente sensibili.

Una cancellazione tramite Ctrl+C o un rifiuto alla conferma finale non deve creare o modificare il file di output.

Il rifiuto della conferma finale stampa `No changes made.` e termina con exit code `0`; Ctrl+C o l'annullamento del wizard terminano con `130`.

La conferma finale è unica: propone la creazione con default positivo quando il target non esiste e la sovrascrittura con default negativo quando il target esiste e differisce dal risultato renderizzato. Con `--force` il riepilogo resta visibile, ma la conferma non viene mostrata. Il confronto no-op avviene comunque: un risultato byte-identico stampa `No changes detected.` e non produce né backup né scritture.

### 5.6 Writer e annotation

Il writer deve:

- aggiornare soltanto le variabili del documento;
- mantenere ordine, commenti normali, annotation, righe vuote e line ending;
- non copiare variabili obsolete del vecchio `.env`;
- usare l'encoder canonico per i valori modificati;
- produrre un file che `compose-go/v2/dotenv` riesca a rileggere con gli stessi valori.

Sono distinti tre contratti di rendering:

1. `parse → render` senza trasformazioni deve essere byte-identico alla rappresentazione normalizzata del template valido; l'eventuale BOM iniziale è l'unica differenza intenzionale;
2. `generate .env` preserva le annotation nelle posizioni originali e può usare l'encoding canonico per le variabili aggiornate;
3. il risultato finale deve essere semanticamente rileggibile con `compose-go`, anche quando non è byte-identico al template.

---

## 6. Piano per fasi

### Fase 0 — Specifica e fondazioni

**Obiettivo:** congelare i contratti prima della UI.

Attività:

1. Creare il modulo Go `github.com/tosdan/env-setup-wizard` e il comando `cmd/env-wizard`, che produce il binario `env-wizard`.
2. Creare `SPEC.md` e documentarvi formato template, annotation, precedenza, output e policy secret.
3. Adottare la licenza Apache-2.0 per il codice del progetto e includere il relativo file `LICENSE`.
4. Applicare alle dipendenze dirette la policy del §3.3 e documentare le eventuali eccezioni.
5. Preparare la struttura `internal/` e le fixture iniziali.
6. Eseguire uno spike usa-e-getta di Huh v2 con `Input`, `EchoModePassword`, `Select`, `Confirm`, `Group`, `Placeholder`, `Validate`, `huh.ErrUserAborted` e un caso non-TTY. Lo spike deve compilare sulle piattaforme del §3.6 ed essere eseguito sui runner nativi previsti; non entra nel dominio del progetto.
7. Eseguire uno spike dell'encoder con `compose-go` e Docker Compose reale per verificare il round-trip letterale di `$`, quote, apostrofi, backslash, `#`, whitespace e `=` e il rifiuto di NUL, `CR` e `LF`.

**Criteri di completamento:**

- `go.mod` presente con module path `github.com/tosdan/env-setup-wizard`;
- architettura e subset del formato documentati;
- nessuna dipendenza UI nel dominio;
- comando che compila anche se il wizard non è ancora implementato.

### Fase 1 — CLI skeleton e pipeline minima

**Obiettivo:** caricare e renderizzare il template senza Huh.

Attività:

1. Implementare `--template`, `--output`, `--force`, `--version` con `flag`.
2. Risolvere i path assoluti con `filepath`, mai tramite concatenazione manuale.
3. Verificare che il template si risolva a un file regolare, che la directory padre dell'output esista e che un output esistente non sia un symlink o un file speciale.
4. Rifiutare template e output che identificano lo stesso file fisico, usando anche `os.SameFile` quando entrambi esistono.
5. Caricare il template preservando contenuto e line ending.
6. Implementare la pipeline provvisoria `load → structural scan → render` senza promettere ancora il parsing completo delle annotation.
7. Normalizzare BOM ed EOL secondo il contratto v1.
8. Rilevare il non-TTY prima di avviare l'interazione.
9. Attivare una CI minima sulle ultime patch Go 1.26 e 1.27 con `go test ./...`, `go vet ./...` e build sulle piattaforme del §3.6; i test specifici del sistema operativo vengono eseguiti su runner nativi.
10. Restituire errori con contesto (`parse template: %w`) senza leak di secret.

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
6. Normalizzare il BOM prima dello scanner e rifiutare il subset non supportato, compresi interpolazione attiva in valori unquoted o double-quoted, commenti inline non quotati e valori multilinea.
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
4. Gestire `@section` come contesto corrente del documento, unendo nel modello logico le occorrenze omonime senza riordinare il documento.
5. Rifiutare annotation sconosciute, valori mancanti e duplicati incompatibili.
6. Validare `@type` (`string`, `int`, `bool`, `port`, `url`).
7. Verificare che il default di `@options` sia non vuoto e appartenga alle option.
8. Verificare la matrice delle combinazioni incompatibili prima dell'avvio del wizard.
9. Validare i valori bool e imporre un valore risolto per `@type bool`.
10. Includere il numero di riga negli errori associabili a una riga e identificare chiaramente il file negli errori globali.
11. Rifiutare i template senza variabili, consentendo invece quelli con sole variabili `@fixed`.

**Criteri di completamento:**

- `ValidateDocument` completamente testabile senza terminale;
- nessun errore di configurazione arriva alla UI;
- `@fixed` e `@section` rispettano la semantica descritta.

### Fase 4 — Encoder e writer con round-trip

**Obiettivo:** generare valori Compose-compatible e preservare il documento.

Attività:

1. Definire `EncodeValue` con una rappresentazione canonica.
2. Coprire caratteri speciali, `$`, quote, apostrofi, backslash, Unicode e stringhe vuote, trattando sempre il contenuto come letterale.
3. Rifiutare valori contenenti NUL, `CR` o `LF`.
4. Validare ogni encoding riparsando il risultato con `compose-go` e coprire nello spike anche Docker Compose reale.
5. Implementare aggiornamento delle variabili nel modello.
6. Preservare annotation e commenti normali nelle posizioni originali.
7. Preservare ordine, righe vuote, section header e line ending.
8. Implementare golden test e table-driven test del writer.

**Milestone:**

```text
parse → update values → preserve document structure → render
```

produce un `.env` valido e rileggibile con i valori attesi.

### Fase 5 — Validation package e Question model

**Obiettivo:** separare le regole di validazione dal terminale e costruire il modello UI.

Attività:

1. Implementare validator puri: `Required`, `Integer`, `Boolean`, `Port`, `URL`.
2. Usare `strconv`, `net/url` e controlli di range della standard library: `Integer` usa base 10 e limiti `int64`, `Port` richiede sole cifre e l'intervallo `1..65535`; per `URL`, richiedere un URI assoluto con schema e host, path o parte opaca non vuoti, senza normalizzazione o accessi di rete.
3. Accettare per bool solo `true` e `false` ignorando il case, rifiutare il valore vuoto e serializzare sempre in lowercase.
4. Definire `Question` e `QuestionKind` nel dominio, includendo `HasValue` e `ValueSource`.
5. Convertire `Variable` in `Question` risolvendo prompt, descrizione, default, placeholder, tipo, required, secret e section.
6. Raggruppare le domande per section: ordine dei gruppi dalla prima occorrenza, ordine interno delle domande dal documento, nessun gruppo vuoto.
7. Escludere `@fixed`.
8. Rappresentare correttamente il caso valido senza `Question` quando tutte le variabili sono `@fixed`.

**Criteri di completamento:**

- validator riutilizzabili da UI, test e futura modalità non interattiva;
- conversione `Variable → Question` priva di dipendenze Huh;
- default e precedenza già risolti prima dell'adapter UI.
- zero `Question` è un risultato valido soltanto quando il template contiene almeno una variabile e tutte le variabili sono `@fixed`.

### Fase 6 — Integrazione Huh

**Obiettivo:** aggiungere il wizard interattivo mantenendo Huh confinato all'adapter.

Attività:

1. Implementare l'adapter `Question → huh.Field`.
2. Collegare `Input`, input password, `Select` e `Confirm`.
3. Collegare i validator ai campi Huh.
4. Usare un solo `Group` per ogni nome di section, comprese le occorrenze non contigue già unite nel modello.
5. Supportare `@placeholder` come suggerimento visivo (`@placeholder` non è un default); non implementare suggestions o `MultiSelect` nella v1.
6. Gestire Ctrl+C e cancellazione senza scrittura.
7. Verificare il comportamento su terminali Windows e Linux e tramite test controllati sui runner macOS nativi; l'esperienza manuale macOS resta preview.
8. Non costruire form o gruppi vuoti quando non esistono domande configurabili; proseguire direttamente con il riepilogo.

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
4. Rilevare i valori esistenti incompatibili, sostituirli come valore iniziale con il default valido del template e associare alla domanda una diagnostica recuperabile.
5. Non mostrare secret esistenti durante il wizard o nel summary; precompilare il valore reale nel campo con `EchoModePassword` e fare in modo che Invio senza modifica lo conservi quando il valore è valido.
6. Per un secret esistente incompatibile, mostrare una diagnostica generica e non usare il valore invalido come valore iniziale.
7. Gestire una cancellazione esplicita come valore vuoto e validarla con `@required`.
8. Gestire variabili nuove e obsolete secondo la source of truth del template.
9. Mostrare `[set]` o `[not set]` per i secret nel riepilogo.
10. Dopo il riepilogo, renderizzare il risultato in memoria e confrontarlo byte per byte con l'output esistente; se è identico, stampare `No changes detected.` e terminare con `0` senza conferma, backup o scrittura.
11. Soltanto in presenza di una modifica, chiedere un'unica conferma di creazione o sovrascrittura con i default definiti nel §5.1; con `--force`, mostrare comunque il riepilogo e saltare la conferma.

**Criteri di completamento:**

- un rerun riparte dai valori correnti;
- un valore corrente incompatibile viene segnalato e può essere corretto nel wizard senza modificare anticipatamente il file;
- i valori `@fixed` restano quelli del template;
- il cancel non modifica il file esistente;
- nessun secret viene stampato.
- un rerun byte-identico è un no-op osservabile e non crea backup.

### Fase 8 — Filesystem e sostituzione atomica

**Obiettivo:** rendere affidabile la scrittura su Windows e Unix.

Attività:

1. Renderizzare il contenuto completamente in memoria.
2. Ripetere i controlli di identità, tipo e symlink definiti nel §5.1 immediatamente prima della scrittura.
3. Creare il temporaneo nella stessa directory del target.
4. Scrivere, sincronizzare e chiudere il temporaneo.
5. Se il target esiste, creare esclusivamente e sincronizzare il backup UTC univoco prima di modificarlo; se il backup fallisce, interrompere lasciando intatto il target.
6. Sostituire il target tramite `replaceFile`.
7. Aggiungere implementazioni build-tagged Unix/Windows usando un temporaneo nella stessa directory e `MoveFileExW` su Windows nella v1.
8. Su Unix, usare `0600` per nuovi output e backup e conservare i bit di permesso del target durante l'overwrite; non promettere la conservazione di proprietario, gruppo, ACL o altri metadata nella v1.
9. Garantire cleanup del temporaneo in caso di errore senza eliminare backup storici.
10. Testare che un errore pre-replace lasci integro il file precedente e documentare i limiti di atomicità/durabilità del filesystem.

**Criteri di completamento:**

- nessun file parziale dopo un errore;
- overwrite verificato con backup datato della versione precedente;
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
8. Costruire con l'ultima patch disponibile di Go 1.27 gli artifact Windows amd64, Linux amd64, Linux arm64, macOS amd64 e macOS arm64; etichettare esplicitamente i due artifact macOS come preview fino allo smoke test manuale.
9. Iniettare versione Semantic Versioning e commit breve tramite linker per `--version`, usando `dev` nelle build locali e senza incorporare la data di compilazione.
10. Preparare `.zip` per Windows e `.tar.gz` per Linux/macOS con binario, README, licenza e notice, usando i nomi canonici del §3.7.
11. Generare e verificare `SHA256SUMS` per tutti gli archivi della release.

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
- section omonime non contigue unite in un solo gruppo;
- ordine dei gruppi determinato dalla prima occorrenza;
- ordine delle domande conservato all'interno della section unita;
- riapertura del gruppo implicito tramite `@section Configuration`;
- section senza variabili configurabili che non genera un gruppo vuoto;
- output che conserva l'ordine originale anche quando il wizard unisce section non contigue.
- template privo di variabili rifiutato prima del wizard;
- template con sole variabili `@fixed` valido e privo di gruppi o pagine vuote.

### 7.3 Test validator

Casi minimi:

- required: stringa non vuota e solo whitespace;
- int: segno opzionale, zeri iniziali e limiti `int64` validi; overflow, whitespace, frazioni, notazione scientifica, esadecimale e testo non numerico invalidi;
- int: valore vuoto valido senza `@required` e non valido con `@required`, conservazione letterale senza normalizzazione;
- bool: valori supportati e non supportati;
- bool: valore vuoto nel template come errore di configurazione;
- bool: valore vuoto o invalido nel file esistente come diagnostica recuperabile;
- port: `1`, `5432`, `65535` e zeri iniziali validi; `0`, `65536`, segno, whitespace, frazioni e testo non numerico invalidi;
- port: valore vuoto valido senza `@required` e non valido con `@required`, conservazione letterale senza normalizzazione;
- URL: URI assoluti HTTP, database, socket, opachi e con schema personalizzato;
- URL: riferimenti relativi, testo senza schema, contenuto vuoto dopo lo schema, whitespace, caratteri di controllo ed escape malformati non validi;
- URL: valore vuoto valido senza `@required` e non valido con `@required`;
- URL: valore letterale invariato dopo validazione, senza normalizzazione né accessi di rete;
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
- `$VAR`, `${VAR}` e `$$` conservati come sequenze letterali identiche;
- Unicode;
- URL;
- password;
- `=` nel valore.

Verificare inoltre che NUL, `CR` e `LF` vengano rifiutati prima del rendering.

### 7.5 Test workflow

- template valido e invalido;
- esecuzione senza argomenti con `.env.example` e `.env` nella cartella corrente;
- override indipendente dei default tramite `--template` e `--output`;
- template e output che identificano lo stesso file;
- template e output che identificano lo stesso file tramite symlink, hardlink o differenze di case;
- template symlink verso un file regolare;
- output symlink rifiutato;
- output esistente che è una directory o un file speciale rifiutato;
- directory padre dell'output mancante senza creazione automatica;
- sostituzione dell'output con un symlink durante il wizard rilevata prima della scrittura;
- uso senza TTY;
- `.env` assente;
- template con sole variabili `@fixed`: nessun form, riepilogo, conferma e scrittura normali;
- template con sole variabili `@fixed` che resta soggetto al requisito TTY anche con `--force`;
- `.env` esistente;
- `.env` esistente con sintassi Compose più ampia del subset del template;
- `.env` esistente semanticamente invalido o con chiavi duplicate rifiutato senza scritture;
- valore multilinea importato dal `.env` esistente trattato come incompatibile e recuperabile nel wizard;
- valori esistenti usati come default;
- valore esistente incompatibile con `int`, `port` o `url`: diagnostica, fallback al template e correzione nel wizard;
- valore esistente incompatibile con `bool` o `@options`: diagnostica, fallback al template e scelta di un valore ammesso;
- valore secret esistente incompatibile: diagnostica senza esposizione del valore;
- annullamento durante il recupero di un valore incompatibile senza modifica del `.env` originale;
- variabili nuove e obsolete;
- `@fixed` prevalente sul `.env` esistente;
- summary secret con `[set]`;
- summary secret con `[not set]`;
- secret esistente confermato senza modifiche;
- conferma di creazione con default positivo;
- conferma di sovrascrittura con default negativo;
- risposta negativa o Ctrl+C senza creazione o modifica dell'output;
- risposta negativa con messaggio `No changes made.` ed exit code `0`;
- Ctrl+C o annullamento del wizard con exit code `130`;
- argomenti invalidi con exit code `2` ed errori operativi con exit code `1`;
- `--version` per build di release e build locale `dev`;
- `--force` che mantiene wizard, validazioni e riepilogo ma salta la conferma finale;
- output esistente byte-identico con messaggio `No changes detected.`, exit code `0` e nessuna conferma, backup o scrittura, anche con `--force`;
- output semanticamente equivalente ma byte-diverso che segue il normale flusso di overwrite e backup;
- cancel e Ctrl+C senza scrittura;
- errore di scrittura senza perdita del file precedente;
- creazione senza backup quando l'output non esiste;
- overwrite con backup byte-identico nominato `<output>.backup-<UTC>`;
- collisione del nome di backup risolta con suffisso numerico senza sovrascritture;
- errore di creazione o sincronizzazione del backup che lascia intatto l'output;
- backup Unix creato con permessi restrittivi e percorso comunicato all'utente;
- nuovo output Unix con `0600` e overwrite che conserva i bit di permesso del target;
- comportamento Windows documentato senza garanzia di conservazione esatta degli ACL.

### 7.6 Test cross-platform e CI

Le fixture devono includere LF e CRLF, BOM, file senza newline finale, EOL misti e UTF-8 invalido. La CI minima deve partire dalla Fase 1: verifica le ultime patch Go 1.26 e 1.27 e usa runner nativi Windows amd64, Linux amd64, Linux arm64, macOS amd64 e macOS arm64 per i test dipendenti dal sistema operativo. La CI di release usa l'ultima patch Go 1.27 e aggiunge `govulncheck`, controllo licenze e artifact. Le prove che dipendono dal terminale devono essere isolate o coperte con test dell'adapter e test di integrazione controllati; non devono richiedere un terminale interattivo nella CI ordinaria. Il superamento della CI macOS consente la pubblicazione preview, non sostituisce lo smoke test manuale necessario per dichiarare stabile il supporto.

La pipeline di release deve inoltre verificare nomi e contenuto degli archivi, eseguire il binario estratto con `--version` sulla relativa piattaforma nativa e validare ogni riga di `SHA256SUMS` prima della pubblicazione.

---

## 8. Backlog v2

Non implementare nella prima release:

- `@options` opzionale o senza default (`FUT-001`);
- `@suggest` come autocomplete libero;
- `@multiple` e `MultiSelect`;
- `@pattern`;
- `@when` e dipendenze condizionali;
- `@generate` e generazione automatica con `crypto/rand`;
- valori multilinea (`FUT-002`);
- output attraverso symlink (`FUT-003`);
- modalità non interattiva;
- configurazione, disattivazione o retention automatica dei backup (`FUT-004`);
- conservazione completa di ownership, gruppo, ACL e metadata durante l'overwrite (`FUT-005`);
- subcommand (`init`, `validate`, `update`, `doctor`);
- supporto a un set più ampio di tipi (`email`, `hostname`, `duration`, `path`, ecc.);
- Windows arm64 e altre piattaforme non elencate nel §3.6;
- launcher `npx`, Homebrew, Scoop, WinGet e altri canali di distribuzione (`FUT-006`);
- firma o attestazione degli artifact oltre ai checksum SHA-256 (`FUT-007`);
- Cobra, framework di validazione o logger esterni senza una necessità concreta.

Prima di ogni ampliamento verificare che il nuovo requisito non trasformi le annotation in un DSL complesso e che il modello intermedio resti sufficiente.

Il contesto delle possibilità rinviate e le domande da riesaminare sono conservati in [`FUTURE.md`](FUTURE.md). Il registro deve essere revisionato obbligatoriamente al completamento della v1; i suoi elementi non entrano automaticamente nella release successiva.

---

## 9. Definition of Done della v1

La v1 è pronta quando:

- `env-wizard` compila e mostra `--version`;
- tag, prerelease e output di `--version` rispettano il contratto Semantic Versioning; gli artifact non incorporano la data di compilazione;
- `go.mod` dichiara `github.com/tosdan/env-setup-wizard` e gli artifact usano il nome `env-wizard`;
- `go.mod` dichiara `go 1.26.0`, CI verifica Go 1.26 e 1.27 e gli artifact sono compilati con l'ultima patch Go 1.27 disponibile;
- il comando senza argomenti usa `.env.example` e `.env` nella cartella corrente e permette di sovrascrivere indipendentemente entrambi i path tramite flag;
- il comando usa un lookup dotenv controllato e non dipende dall'ambiente esterno del processo;
- template validi con commenti, righe vuote, quote e annotation v1 vengono interpretati correttamente;
- template invalidi vengono rifiutati prima del wizard con causa e linea quando applicabile; gli errori globali identificano il file;
- un template senza variabili viene rifiutato, mentre un template con sole variabili `@fixed` salta il form ma conserva riepilogo e flusso di scrittura interattivo;
- template con BOM vengono normalizzati, mentre UTF-16, UTF-8 non valido, EOL misti, CR isolati e sintassi fuori dal subset vengono rifiutati;
- le chiavi rispettano `[A-Za-z_][A-Za-z0-9_.-]*` e i duplicati sono rifiutati;
- il writer conserva `$VAR`, `${VAR}` e `$$` come testo letterale, non dipende dall'ambiente del processo e rifiuta NUL, `CR` e `LF`;
- il parser semantico dotenv è delegato a `compose-go` con lookup controllato e deterministico;
- il dominio non dipende da Huh o filesystem;
- il wizard costruisce gruppi/field coerenti con il modello `Question` e usa `EchoModePassword` per `@secret`;
- `@required`, `@secret`, `@options`, `@type`, `@placeholder` e `@fixed` funzionano secondo specifica, mentre `@section` unisce le occorrenze omonime senza riordinare l'output;
- il rerun usa i valori del `.env` esistente con le precedenze definite;
- il `.env` esistente può usare la sintassi supportata da `compose-go`, mentre errori globali o chiavi duplicate interrompono l'esecuzione senza scritture;
- i valori esistenti incompatibili vengono segnalati nel wizard, sostituiti inizialmente dal default valido del template e corretti prima della scrittura;
- i secret esistenti vengono conservati quando l'utente conferma senza modificarli;
- i booleani vuoti sono rifiutati e quelli validi vengono scritti come `true` o `false`;
- `@type int` usa interi decimali `int64` e `@type port` usa cifre nell'intervallo `1..65535`, con comportamento indipendente dalla piattaforma e senza normalizzazione testuale;
- `@type url` accetta URI assoluti generici, rifiuta riferimenti relativi e non normalizza né verifica in rete i valori;
- i secret non compaiono in summary, log o errori e `.env` viene documentato come non-secret-store;
- il writer conserva annotation, commenti e struttura originale del documento;
- l'output supera i test di round-trip con `compose-go`;
- la scrittura usa temporaneo, `Sync`, `Close` e replace e conserva ogni output precedente in un backup UTC univoco prima dell'overwrite;
- su Unix i nuovi output e i backup usano `0600`, mentre un overwrite conserva i bit di permesso del target; ownership, gruppo, ACL e altri metadata non sono garantiti nella v1;
- cancel, Ctrl+C ed errori non lasciano `.env` parziali;
- il riepilogo viene sempre mostrato, la conferma finale distingue creazione e sovrascrittura e `--force` salta soltanto tale conferma;
- un risultato byte-identico all'output esistente termina con `No changes detected.` senza conferma, backup o scrittura, anche con `--force`;
- il comando rispetta gli exit code documentati, incluso `0` per il rifiuto finale e `130` per l'annullamento del wizard;
- il comando rifiuta l'uso non-TTY e impedisce `--output` uguale a `--template`;
- i controlli sui path impediscono di sovrascrivere fisicamente il template, rifiutano output symlink o non regolari e vengono ripetuti prima della scrittura;
- `go test ./...`, `go vet ./...` e `govulncheck ./...` sono verdi;
- sono disponibili artifact stabili Windows amd64, Linux amd64 e Linux arm64 e artifact preview macOS amd64 e arm64, tutti verificati su runner nativi; macOS resta dichiarato preview fino a uno smoke test manuale;
- gli artifact hanno nomi canonici, contengono binario, README, licenza e notice, superano lo smoke test `--version` e sono verificabili tramite `SHA256SUMS`;
- il README documenta sia il download del singolo artifact sia `go install github.com/tosdan/env-setup-wizard/cmd/env-wizard@latest`;
- README, `SPEC.md`, `CONTRIBUTING.md`, `SECURITY.md`, licenza Apache-2.0 e third-party notices sono presenti e coerenti.
- tutte le dipendenze rispettano la policy del §3.3, e CI verifica vulnerabilità e licenze prima della release.
- tutti gli elementi aperti in `FUTURE.md` sono stati riesaminati e classificati come pianificati, rifiutati oppure ancora rinviati con motivazione aggiornata.

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
