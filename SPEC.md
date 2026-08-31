# Env Setup Wizard v1 specification

This document is the normative functional contract for v1. `PLAN.md` describes implementation order and rationale; `FUTURE.md` records ideas intentionally deferred beyond v1.

## Command

The executable is `env-wizard` (`env-wizard.exe` on Windows). The Go module path is `github.com/tosdan/env-setup-wizard` and the minimum Go version is 1.26.0.

```text
env-wizard [--template PATH] [--output PATH] [--force]
env-wizard --version
```

Defaults are resolved independently against the current working directory:

```text
--template .env.example
--output   .env
```

When the implicit `.env.example` does not exist, the command reports that it
looked in the current directory and suggests either creating the file there or
using `--template PATH`. A missing explicitly selected template retains its
detailed path error.

`--force` skips only the final create/overwrite confirmation. It never skips the wizard, summary, validation, no-op detection, backup, or TTY requirement.

Exit codes:

| Code | Meaning |
| ---: | --- |
| `0` | File written, no change detected, final confirmation declined, `--help`, or `--version` |
| `1` | Invalid template/value, filesystem failure, or other operational error |
| `2` | Invalid command-line arguments |
| `130` | Ctrl+C or wizard cancellation |

A declined final confirmation prints `No changes made.`. A byte-identical result prints `No changes detected.`.

## Template format

The template is UTF-8, optionally with an initial BOM, and uses consistently either LF or CRLF. The writer emits UTF-8 without BOM and preserves the valid input EOL style and final-newline state.

Supported variable forms are:

```dotenv
KEY=
KEY=value
KEY="value"
KEY='value'
```

Keys match `[A-Za-z_][A-Za-z0-9_.-]*` and are case-sensitive. Comments and blank lines are supported. A valid template contains at least one variable.

The template rejects before the wizard:

- duplicate keys;
- `:` assignments and `export` prefixes;
- unquoted inline comments;
- multiline values or semantic values containing NUL, CR, or LF;
- mixed EOLs, isolated CR, invalid UTF-8, and UTF-16;
- interpolation-active `$VAR` or `${VAR}` in unquoted or double-quoted values;
- syntax outside the v1 subset.

Literal `$VAR`, `${VAR}`, and `$$` are valid in source forms that make them literal, such as single quotes. Value semantics are delegated to `github.com/compose-spec/compose-go/v2/dotenv` using an initially empty, controlled lookup; process environment variables never affect parsing.

## Annotations

Annotations use comment lines of the form `# @name value`. Field annotations bind to the immediately following variable. A blank line or normal comment interrupts the block. `@section` changes document context and does not bind as field metadata.

| Annotation | Contract |
| --- | --- |
| `@prompt value` | Question label; fallback is the variable key |
| `@description value` | Additional field help |
| `@required` | Reject empty or whitespace-only final values |
| `@secret` | Mask input and redact summary, logs, diagnostics, and errors |
| `@type string\|int\|bool\|port\|url` | Select validation and field kind; default is `string` |
| `@options v1,v2,...` | Closed, case-sensitive selection |
| `@placeholder value` | Visual hint for textual input, never a default |
| `@fixed` | Always use the template value and do not ask a question |
| `@section value` | Open or reopen a wizard group |

Flag annotations (`@required`, `@secret`, `@fixed`) take no value. Other annotations require a nonempty value. Unknown annotations, invalid duplicates, missing values, orphaned field annotations, and incompatible combinations are template errors.

`@options` trims comma-separated entries and rejects empty or duplicate entries. Commas inside options are unsupported. The template default must be nonempty and belong to the list. `@options` is incompatible with `@type bool|int|port|url`, `@secret`, and `@placeholder`.

`@fixed` is incompatible with `@prompt` and `@placeholder`; it may combine with `@required` and `@secret`. Its template value still passes all applicable validation.

Section names are case-sensitive. Equal names merge into one wizard group: group order follows first occurrence and question order follows document order within the merged group. The output always retains global document order. The implicit section is `Configuration`, which can be reopened explicitly. Empty sections do not produce pages.

## Type validation

- `string`: any valid single-line UTF-8 value.
- `int`: optional sign followed by decimal digits, within signed 64-bit range. Leading zeroes are accepted.
- `bool`: only `true` or `false`, case-insensitive on input and lowercase on output. The template default is mandatory and valid.
- `port`: decimal digits only, range `1..65535`. Leading zeroes are accepted.
- `url`: absolute generic URI with a valid scheme and a nonempty host, path, or opaque part.

Whitespace, fractions, scientific notation, and hexadecimal notation are invalid for numeric types. Valid `int`, `port`, and `url` text is preserved literally. Empty `int`, `port`, and `url` values are accepted unless `@required` is present. URL validation allows userinfo, port, query, fragment, and custom schemes; it performs no normalization, DNS lookup, reachability check, or connection.

## Existing output and merge

An existing output is parsed as a value source using the full syntax accepted by compose-go, not the restricted template syntax. It must be valid UTF-8 and have unique keys. Its formatting is not preserved. Obsolete keys are ignored.

Value precedence is:

```text
wizard answer
existing output
template default
```

`@fixed` always wins over existing and user values.

A compatible existing value initializes the field. Enter without modification preserves it, including the real value of a secret. Explicit deletion produces an empty value and may fail `@required`.

An existing value incompatible with the new template is recoverable: it is not selected as the initial value, the valid template default is used, and the wizard shows a diagnostic requiring confirmation or correction. Non-secret diagnostics may include the invalid value. Secret diagnostics never include its content or length. A parsed multiline value follows this same recovery flow because v1 cannot write it.

Variables present only in the existing output are not copied. Variables present only in the template are included.

## Wizard and summary

The wizard requires a TTY. A template with variables that are all `@fixed` is valid and skips form construction, but still requires a TTY and continues through summary and the normal final flow.

Field mapping:

```text
string/default      -> Input
string + secret     -> Input with EchoModePassword
int/port/url        -> Input with validator
bool                -> Confirm
options             -> Select
```

Invalid input keeps the user on the current field. The summary is always shown and is grouped like the wizard. Normal values are visible; secret values appear only as `[set]` or `[not set]`.

After rendering the candidate in memory, a byte-identical existing output terminates successfully without confirmation, backup, or write, including with `--force`. Otherwise creation asks `Create .env? [Y/n]` and overwrite asks `Overwrite existing .env? [y/N]` unless `--force` is present.

## Rendering

Annotations are omitted from generated output. Normal comments, blank lines, variable order, EOL style, and final-newline state come from the template. Obsolete existing variables are not copied.

Modified values use one canonical Compose-compatible encoding. For every accepted value, parsing `KEY=<encoded>` through compose-go with the controlled lookup must reproduce the original value exactly. `$VAR`, `${VAR}`, and `$$` remain identical literal sequences. NUL, CR, and LF are rejected before rendering.

## Filesystem safety and backups

Template and output paths are absolute and normalized before the wizard. The template must resolve to a readable regular file and may itself be a symlink. The output parent must already exist. An existing output must be a regular file and must not be a symlink.

Template and output must not identify the same physical file through spelling, case-insensitive comparison, symlink, or hardlink. Type, identity, and symlink checks run again immediately before writing.

The safe write sequence is:

1. Render the complete candidate in memory.
2. Create a temporary file in the output directory.
3. Write, sync, and close it.
4. If overwriting, create and sync a byte-identical backup of the old output.
5. Atomically replace the target through the OS adapter.

Backup names use `<output>.backup-YYYYMMDDTHHMMSSZ`, with `-1`, `-2`, and so on for collisions. Backup creation is exclusive and never overwrites older backups. A failed backup leaves the original untouched. Backups are mandatory for effective overwrites, including `--force`, and are never removed automatically in v1.

On Unix, new outputs and backups use `0600`; an overwrite preserves the previous output permission bits. Ownership, group, ACLs, timestamps, and other metadata are not guaranteed. On Windows, the replacement uses ACLs derived from the directory and exact preservation of previous ACLs is not guaranteed.

The backup path is reported after success. Backups may contain secrets; documentation recommends ignoring `.env*` while explicitly retaining `.env.example`.

## Security

`@secret` protects terminal presentation only. Secret values are still stored in plaintext in `.env` and its backups. They never appear in summaries, logs, diagnostics, errors, or debug dumps. `.env` is not a secret store; highly sensitive deployments should use Docker Secrets or an equivalent system.

Cancellation, declined confirmation, parse failures, validation failures, and pre-replace filesystem failures never create or modify the output. Temporary files are cleaned up on failure.

## Distribution

Stable v1 artifacts target Windows amd64, Linux amd64, and Linux arm64. macOS amd64 and arm64 artifacts are preview until a manual terminal smoke test complements native CI. Windows arm64 is not part of v1.

Releases use Semantic Versioning, beginning with `v1.0.0-rc.N` and then `v1.0.0`. Release `--version` output is `env-wizard v1.0.0 (commit abc1234)`; local builds use `env-wizard dev`. Build timestamps are excluded for reproducibility.

GitHub Releases contain `.zip` on Windows and `.tar.gz` on Linux/macOS, named `env-wizard_<version>_<os>_<arch>.<ext>`. Every archive includes the binary, README, Apache-2.0 `LICENSE`, and `THIRD_PARTY_NOTICES`; `SHA256SUMS` covers all archives.
