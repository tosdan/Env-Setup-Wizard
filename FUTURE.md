# Registro delle evoluzioni post-v1

Questo documento raccoglie le funzionalità escluse intenzionalmente dalla v1 ma meritevoli di una nuova valutazione. Non è una promessa di implementazione e non sostituisce il piano operativo: serve a conservare il contesto delle decisioni senza complicare la prima release.

## Regole del registro

- Aggiungere un elemento quando una possibilità concreta viene rinviata oltre la v1.
- Registrare la decisione presa per la v1, il motivo del rinvio e le domande da riesaminare.
- Non progettare in anticipo l'implementazione futura oltre quanto serve per non perdere il contesto.
- Al completamento della v1, riesaminare tutti gli elementi aperti e assegnare a ciascuno uno degli esiti: `pianificato`, `rifiutato` oppure `ancora rinviato` con una motivazione aggiornata.

## Revisione obbligatoria alla fine della v1

- [ ] Riesaminare tutti gli elementi con stato `da rivalutare`.
- [ ] Considerare il feedback raccolto durante l'uso della v1.
- [ ] Registrare l'esito di ogni elemento senza trasformare automaticamente il registro nel backlog della release successiva.
- [ ] Aggiornare `PLAN.md` e la roadmap soltanto per gli elementi effettivamente pianificati.

## Elementi da rivalutare

### FUT-001 — `@options` opzionale o senza default

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** una variabile annotata con `@options` deve avere nel `.env.example` un valore non vuoto appartenente alle opzioni dichiarate.

Esempio valido nella v1:

```dotenv
# @options development,staging,production
APP_ENV=development
```

Esempio intenzionalmente non supportato nella v1:

```dotenv
# @options development,staging,production
APP_ENV=
```

**Perché è stato rinviato:** un `Select` privo di default richiede di definire se il valore vuoto sia selezionabile, se la prima opzione sia implicita e come interagisca con `@required`. Una scelta implicita rischierebbe di produrre configurazioni non intenzionali.

**Domande da riesaminare:**

- Serve realmente una selezione opzionale nei template usati con la v1?
- Il valore vuoto deve apparire come un'opzione esplicita, per esempio `(nessuna)`, oppure richiede una nuova annotation?
- Quale deve essere l'interazione con `@required`?
- Come deve comportarsi il rerun quando il `.env` esistente contiene un valore vuoto?
- Come viene serializzata e mostrata nel riepilogo una selezione vuota?

### FUT-002 — Valori multilinea

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** template e risposte del wizard devono produrre valori UTF-8 letterali su una sola riga. Un valore multilinea letto da un `.env` esistente è accettato in importazione soltanto per essere segnalato come incompatibile e sostituito attraverso il flusso di recupero. Il valore finale rifiuta NUL, `CR` e `LF`; il writer supporta gli altri caratteri speciali senza interpolazione.

**Perché è stato rinviato:** i valori multilinea richiedono di definire l'esperienza di inserimento nel terminale, il formato canonico di serializzazione, il comportamento del rerun e la preservazione della struttura del documento. Il widget `Input` previsto per la v1 è intenzionalmente a riga singola.

**Domande da riesaminare:**

- Esistono casi d'uso reali emersi dalla v1, come certificati PEM o chiavi private?
- Il valore deve essere inserito direttamente, letto da un file o generato tramite un'altra annotation?
- Il writer deve usare escape in una singola riga oppure la sintassi multilinea supportata da Compose?
- Come devono funzionare riepilogo, modifica e cancellazione durante il rerun?
- Servono limiti di dimensione o regole aggiuntive per i secret multilinea?

### FUT-003 — Output attraverso symlink

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** il template può essere un symlink verso un file regolare, mentre un path di output che sia un symlink viene rifiutato. La directory padre può attraversare symlink normalmente, purché il target finale superi tutti i controlli di sicurezza.

**Perché è stato rinviato:** scrivere attraverso un symlink rende ambiguo se il programma debba sostituire il link oppure modificare il file a cui punta e può produrre comportamenti differenti tra Windows e Unix.

**Domande da riesaminare:**

- Esistono casi d'uso reali emersi dalla v1 che richiedono un `.env` symlink?
- L'operazione deve preservare il symlink e aggiornare il target oppure sostituire il link?
- Come si mantiene la garanzia di scrittura atomica sui sistemi operativi supportati?
- Quali controlli sono necessari per evitare cambi di destinazione tra validazione e scrittura?

### FUT-004 — Configurazione e retention dei backup

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** ogni overwrite effettivo conserva prima una copia byte-identica dell'output esistente nella stessa directory, con nome `<output>.backup-<UTC>` e suffisso numerico in caso di collisione. Il backup è obbligatorio anche con `--force`; non viene creato per un output nuovo né quando il risultato è byte-identico all'output esistente, e non viene eliminato automaticamente.

**Perché è stato rinviato:** disabilitare i backup o rimuoverli automaticamente introduce nuove scelte di sicurezza e operazioni distruttive. Una policy di retention sensata richiede dati reali su frequenza d'uso, dimensione dei file e aspettative degli utenti.

**Domande da riesaminare:**

- Serve un flag per disabilitare esplicitamente il backup?
- La retention deve essere basata sul numero di copie, sull'età o su entrambi?
- La pulizia deve essere automatica durante l'overwrite oppure affidata a un comando esplicito?
- Come devono essere riportati gli errori di pulizia senza compromettere la scrittura principale?
- È utile avvisare quando il pattern dei backup non risulta coperto da `.gitignore`?

### FUT-005 — Conservazione completa dei metadata del file

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** su Unix un nuovo `.env` e i backup usano `0600`, mentre l'overwrite conserva i bit di permesso del target precedente. Non vengono garantiti proprietario, gruppo, ACL, timestamp o altri metadata; su Windows il file sostitutivo usa gli ACL derivati dalla directory e può non conservare esattamente quelli del target precedente.

**Perché è stato rinviato:** la conservazione completa richiede primitive e privilegi differenti tra sistemi operativi e può entrare in conflitto con il modello di sicurezza del file temporaneo. Per il caso d'uso principale della v1 è più importante evitare file parziali e non modificare inaspettatamente i permessi Unix già configurati.

**Domande da riesaminare:**

- Quali metadata risultano realmente necessari dopo l'uso della v1: ownership, gruppo, ACL, attributi estesi o timestamp?
- Su Windows conviene adottare `ReplaceFileW` o un'altra primitiva dedicata?
- Un errore nella copia dei metadata deve impedire la sostituzione oppure produrre un warning?
- Come verificare la preservazione in CI senza richiedere privilegi elevati?

### FUT-006 — Launcher `npx` e package manager

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** distribuire eseguibili Go standalone tramite GitHub Releases e supportare facoltativamente `go install`; non pubblicare ancora wrapper npm, installer remoti o manifest per package manager.

**Perché è stato rinviato:** un launcher `npx` può ridurre la percezione di installare software e selezionare automaticamente il binario corretto, ma aggiunge una seconda supply chain, caching, mapping delle versioni e gestione degli errori di download. Homebrew, Scoop e WinGet richiedono inoltre workflow e manifest specifici che conviene costruire su release Go già stabili.

**Domande da riesaminare:**

- Gli utenti della v1 chiedono realmente un'esecuzione tramite `npx` o preferiscono package manager nativi?
- Il wrapper npm deve contenere binari, scaricarli al primo avvio oppure delegare a un package separato per piattaforma?
- Come vengono allineate versione npm e versione Semantic Versioning del binario Go?
- Dove viene mantenuta la cache e come si verificano checksum o firme prima dell'esecuzione?
- Quali canali hanno priorità tra npm, Homebrew, Scoop e WinGet?

### FUT-007 — Firma e attestazione degli artifact

**Stato:** da rivalutare dopo la v1.

**Decisione per la v1:** pubblicare `SHA256SUMS` insieme agli archivi, senza introdurre ancora firme crittografiche o attestazioni di provenienza.

**Perché è stato rinviato:** firme e attestazioni richiedono la scelta di un modello di identità, gestione delle chiavi o firma keyless e procedure di verifica documentate. I checksum risolvono l'integrità accidentale ma non autenticano da soli l'origine.

**Domande da riesaminare:**

- Serve una firma keyless, una chiave di progetto oppure un meccanismo offerto dalla piattaforma di hosting?
- Quali strumenti di verifica sono realistici per gli utenti Windows, Linux e macOS?
- Launcher e package manager futuri devono rifiutare artifact privi di firma valida?
- È opportuno produrre anche SBOM e attestazioni della build riproducibile?
