# Phase 0 spike results

Date: 2026-08-30

These experiments were disposable. Their source and compiled artifacts are not part of the product; this document retains only evidence that affects implementation.

## Huh v2

Tested version: `charm.land/huh/v2 v2.0.3`.

The experiment compiled a form containing:

- `Input` with title, description, placeholder, bound value, and validator;
- password `Input` using `EchoModePassword`;
- generic `Select[string]` with options and a bound value;
- `Confirm` with a bound boolean;
- a titled `Group` and `Form`;
- `WithInput`, `WithOutput`, `RunWithContext`, and `ErrUserAborted`.

The same source cross-compiled successfully for Windows amd64, Linux amd64, Linux arm64, macOS amd64, and macOS arm64.

### Finding: non-TTY input is not rejected by Huh

Running the form on Windows with an empty non-terminal reader and discarded output returned `nil` and retained the defaults. Huh therefore cannot be the source of the v1 TTY guarantee.

Implementation consequence: the application workflow must perform an explicit TTY preflight before constructing or running the Huh adapter. `--force` must not bypass it. Tests must cover the preflight independently from Huh.

### Testability

Huh accepts injected readers and writers and provides an accessible execution mode. Adapter tests can therefore use controlled I/O, while cancellation is mapped by checking `errors.Is(err, huh.ErrUserAborted)`.

Native execution was performed on Windows amd64. Native Linux and macOS execution remains a CI responsibility; cross-compilation alone does not promote platform support.

## Compose-compatible canonical encoding

Tested versions:

- `github.com/compose-spec/compose-go/v2 v2.14.0`;
- Docker Compose `v5.4.0`.

The candidate encoder always emits a single-quoted value and escapes only an embedded apostrophe as `\'`. It rejects NUL, CR, and LF before rendering.

The following 14 semantic values round-tripped through both compose-go and the actual `docker compose config` parser:

- empty string;
- plain text;
- leading and trailing spaces;
- a tab;
- `#`;
- `$VAR`;
- `${VAR}`;
- `$$`;
- double quote;
- apostrophe;
- backslash;
- `=`;
- Unicode.

NUL, CR, and LF were rejected before rendering as required.

### Finding: canonical Compose output re-escapes dollars

`docker compose config --format json` represents a literal dollar from an env file as `$$`. This is output serialization, not a change to the runtime value. The experiment accounted for this canonical escaping when comparing Docker Compose output; the encoder itself must not double the original semantic value before single-quoting it.

Implementation consequence: the v1 encoder can be small and deterministic—single-quote every accepted value, escape apostrophes, reject forbidden line characters, and verify semantic equality through compose-go. Docker Compose remains an integration oracle, with its canonical dollar escaping handled only in the test adapter.

The encoder experiment cross-compiled successfully for all five v1 artifact targets. Native Docker Compose execution was performed on Windows amd64; other native operating systems remain CI responsibilities.
