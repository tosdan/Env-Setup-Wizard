# Env Setup Wizard

Env Setup Wizard is a Go CLI that turns an annotated `.env.example` into an
interactive setup wizard and writes a validated `.env` safely. It is designed
for projects that want a friendly first-run configuration flow without adding a
runtime, framework, or application-specific setup script.

The core workflow is complete through **Phase 8**. Release hardening and
packaging are in progress in Phase 9.

## Features

- Run with no arguments: `.env.example` becomes `.env` in the current directory.
- Add prompts, descriptions, types, closed selections, secrets, fixed values,
  and grouped pages directly to the template.
- Reuse compatible values from an existing `.env` and recover interactively
  when a value is no longer compatible with the template.
- Show a grouped summary while rendering secrets only as `[set]` or `[not set]`.
- Detect byte-identical reruns without confirming, writing, or creating a backup.
- Write through a synced temporary file and atomically replace the destination.
- Create a byte-identical timestamped backup before every effective overwrite.

## Installation

Go 1.26 or newer is required when installing or building from source.

```text
go install github.com/tosdan/env-setup-wizard/cmd/env-wizard@latest
```

From a local checkout:

```text
go build -o env-wizard ./cmd/env-wizard
```

On Windows, use `-o env-wizard.exe`. The resulting executable is standalone;
end users do not need Go installed. Prebuilt release archives will become the
recommended installation method once the Phase 9 release pipeline is complete.

## Quick start

Create an annotated `.env.example`, open an interactive terminal in the project
directory, and run:

```text
env-wizard
```

The default paths are independent and can be overridden:

```text
env-wizard --template config/app.env.example --output config/app.env
env-wizard --force
env-wizard --version
```

`--force` skips only the final create/overwrite confirmation. It does not skip
the TTY requirement, wizard, validation, summary, no-op detection, or backups.

## Template example

```dotenv
# Application configuration
# @section Application
# @prompt Application name
# @description Used in logs and local development URLs
# @required
APP_NAME=my-app

# @options development,staging,production
ENVIRONMENT=development

# @type bool
DEBUG=false

# @section Network
# @prompt HTTP port
# @type port
PORT=8080

# @prompt Public URL
# @type url
PUBLIC_URL=http://localhost:8080

# @section Secrets
# @prompt API token
# @description Stored in plaintext in .env and its backups
# @secret
# @required
API_TOKEN=

# @section Internal
# @fixed
COMPOSE_PROJECT_NAME=my-app
```

Annotations are removed from the generated `.env`. Normal comments, blank
lines, variable order, consistent LF/CRLF style, and final-newline state come
from the template.

## Annotation reference

Annotations are comment lines in the form `# @name value`.

| Annotation | Effect |
| --- | --- |
| `@prompt value` | Sets the question label; the variable key is the fallback. |
| `@description value` | Adds help text to the field. |
| `@required` | Rejects an empty or whitespace-only final value. |
| `@secret` | Masks input and redacts the value from summaries and diagnostics. |
| `@type string\|int\|bool\|port\|url` | Selects validation and field behavior; the default is `string`. |
| `@options v1,v2,...` | Creates a closed, case-sensitive selection. |
| `@placeholder value` | Adds a visual hint to a textual input; it is not a default. |
| `@fixed` | Always uses the template value and skips the question. |
| `@section value` | Opens or reopens a wizard group. |

Field annotations must be consecutive and immediately precede their variable.
A blank line or normal comment before the variable leaves the annotation block
orphaned and makes the template invalid. `@section` changes the current group
instead of binding to one field. Sections with the same case-sensitive name are
merged in the wizard without changing output order.

`@options` requires a nonempty default from its list and cannot be combined with
typed numeric, boolean, or URL fields, `@secret`, or `@placeholder`. `@fixed`
cannot be combined with `@prompt` or `@placeholder`. See [`SPEC.md`](SPEC.md) for
the complete compatibility and validation contract.

## Reruns and backups

On a rerun, values from the existing output take precedence over template
defaults, except for `@fixed`. Obsolete variables are dropped because the
template remains the source of truth. An incompatible existing value is shown
as a recoverable field error and the wizard starts from the valid template
default. Secret diagnostics reveal neither content nor length.

Creation defaults to `yes`; overwrite defaults to `no`. Declining prints
`No changes made.` and exits successfully. A byte-identical candidate prints
`No changes detected.` and skips confirmation, backup, and write.

Every effective overwrite creates a byte-identical backup named
`<output>.backup-YYYYMMDDTHHMMSSZ`. Name collisions use `-1`, `-2`, and so on
without overwriting older backups. The CLI reports the created backup path.

## Security and filesystem guarantees

`@secret` protects terminal presentation only. `.env` and its backups store
values in plaintext and are not secret stores. Keep `.env*` out of version
control while explicitly retaining `.env.example`, as this repository's
`.gitignore` does.

The template must resolve to a readable regular file. An existing output must
be a distinct regular file and cannot be a symlink; the output directory must
already exist. Unix creates new outputs and backups with `0600` and preserves
existing output permission bits on overwrite. Exact ownership, ACL, timestamps,
and other metadata are not guaranteed. Windows replacements derive ACLs from
the directory. Atomicity and durability ultimately depend on the underlying
filesystem and may be weaker on network shares or unusual filesystems.

## Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | File written, no-op, declined confirmation, help, or version output. |
| `1` | Invalid template/value, filesystem failure, or another operational error. |
| `2` | Invalid command-line arguments. |
| `130` | Ctrl+C or wizard cancellation. |

## Platform status

The v1 stable targets are Windows amd64, Linux amd64, and Linux arm64. macOS
amd64 and arm64 remain preview until native CI is complemented by a manual
terminal smoke test. Windows arm64 is outside the v1 scope.

## Development

```text
go test ./...
go vet ./...
go run ./internal/tools/licenses -check
go run ./cmd/env-wizard --version
```

Local builds report `env-wizard dev`. Release builds inject the semantic version
and short commit without embedding a build timestamp.

## Project documents

- [`SPEC.md`](SPEC.md) is the normative v1 behavior.
- [`PLAN.md`](PLAN.md) defines architecture and implementation order.
- [`FUTURE.md`](FUTURE.md) preserves intentionally deferred ideas.
- [`DEPENDENCIES.md`](DEPENDENCIES.md) records the dependency policy.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) explains the contribution workflow.
- [`SECURITY.md`](SECURITY.md) explains private vulnerability reporting and support.
- [`LICENSE`](LICENSE) contains the Apache-2.0 project license.
- [`THIRD_PARTY_NOTICES`](THIRD_PARTY_NOTICES) contains dependency licenses and notices.
