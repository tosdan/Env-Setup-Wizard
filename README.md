# Env Setup Wizard

Env Setup Wizard is a Go CLI that turns an annotated `.env.example` into an
interactive setup wizard and writes a validated `.env` safely. It is designed
for projects that want a friendly first-run configuration flow without adding a
runtime, framework, or application-specific setup script.

The v1 scope, release hardening, documentation, and cross-platform packaging
are complete.

## Features

- Run with no arguments: `.env.example` becomes `.env` in the current directory.
- When a named `*.env.example` Template is selected, choose between its matching
  `*.env` name and the project `.env` destination.
- Add prompts, descriptions, types, closed selections, secrets, fixed values,
  and grouped pages directly to the template.
- Reuse compatible values from an existing `.env` and recover interactively
  when a value is no longer compatible with the template.
- Show descriptive prompts and variable keys in a grouped summary while
  rendering secrets only as `[set]` or `[not set]`.
- Preserve normal comments and wizard annotations in their original positions.
- Detect byte-identical reruns without confirming, writing, or creating a backup.
- Write through a synced temporary file and atomically replace the destination.
- Create a byte-identical timestamped backup before every effective overwrite.

## Installation

### Prebuilt archive

Open [GitHub Releases](https://github.com/tosdan/Env-Setup-Wizard/releases) and
download `SHA256SUMS` plus the one archive matching the machine:

| Platform | Archive |
| --- | --- |
| Windows amd64 | `env-wizard_<version>_windows_amd64.zip` |
| Linux amd64 | `env-wizard_<version>_linux_amd64.tar.gz` |
| Linux arm64 | `env-wizard_<version>_linux_arm64.tar.gz` |
| macOS Intel (preview) | `env-wizard_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon (preview) | `env-wizard_<version>_darwin_arm64.tar.gz` |

Before extracting, calculate the archive's SHA-256 digest and compare it
case-insensitively with the matching row in `SHA256SUMS`. For example, on
Windows PowerShell:

```powershell
Get-FileHash -Algorithm SHA256 .\env-wizard_1.0.0_windows_amd64.zip
Select-String -SimpleMatch "env-wizard_1.0.0_windows_amd64.zip" .\SHA256SUMS
```

On Linux, use `sha256sum`; on macOS, use the preinstalled `shasum`:

```text
sha256sum env-wizard_1.0.0_linux_amd64.tar.gz
grep env-wizard_1.0.0_linux_amd64.tar.gz SHA256SUMS

shasum -a 256 env-wizard_1.0.0_darwin_arm64.tar.gz
grep env-wizard_1.0.0_darwin_arm64.tar.gz SHA256SUMS
```

Extract the archive and run the version check from the destination directory:

```powershell
Expand-Archive .\env-wizard_1.0.0_windows_amd64.zip -DestinationPath .\env-wizard
.\env-wizard\env-wizard.exe --version
```

```text
tar -xzf env-wizard_1.0.0_linux_amd64.tar.gz
./env-wizard --version
```

The executable is standalone: it can remain in that directory or be moved to a
directory already present in `PATH`. Each archive also contains the README,
Apache-2.0 `LICENSE`, and `THIRD_PARTY_NOTICES`. The macOS preview artifacts are
not Developer ID signed or notarized; if macOS blocks one, prefer the source
installation below instead of disabling Gatekeeper globally.

### Install from source

Go 1.26 or newer is required. Install the latest tagged version with:

```text
go install github.com/tosdan/env-setup-wizard/cmd/env-wizard@latest
```

Use an explicit version when reproducibility matters:

```text
go install github.com/tosdan/env-setup-wizard/cmd/env-wizard@v1.0.0
```

The Go binary installation directory (`GOBIN`, or `GOPATH/bin` by default) must
be present in `PATH`.

### Build a checkout

From a local checkout:

```text
go build -o env-wizard ./cmd/env-wizard
```

On Windows, use `-o env-wizard.exe`. The resulting executable is standalone;
end users do not need Go installed.

## Quick start

Create an annotated `.env.example`, open an interactive terminal in the project
directory, and run:

```text
env-wizard
```

With no arguments, both default paths use the current directory. A named
`*.env.example` passed through `--template` offers its matching `*.env` path as
the first choice unless `--output` is also provided:

```text
env-wizard --template config/app.env.example --output config/app.env
env-wizard --force
env-wizard --version
```

If the default `.env.example` is missing, the command reports the current
directory and shows how to select a different template with `--template`.

`--force` skips only the final create/overwrite confirmation. It does not skip
the output destination choice, TTY requirement, wizard, validation, summary,
no-op detection, or backups.

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

Runnable templates covering every v1 annotation are available in
[`examples/`](examples/README.md), with commands for keeping manual-test output
inside the ignored `.tmp` directory.

## Annotation reference

Annotations are comment lines in the form `# @name value`.

| Annotation | Effect |
| --- | --- |
| `@prompt value` | Shows `Prompt text (VARIABLE_KEY)` in questions and summary; without a distinct prompt, shows only the key. |
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
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
go run ./cmd/env-wizard --version
```

Local builds report `env-wizard dev`. Release builds inject the semantic version
and short commit without embedding a build timestamp. The release artifact
workflow builds and smoke-tests each supported target natively, checks archive
names and contents, and emits a verified `SHA256SUMS`. Manual runs stop there as
dry runs. A pushed Semantic Versioning tag publishes those verified files as a
GitHub Release; prerelease versions are marked accordingly and macOS remains
identified as preview. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the release
contract and local verification commands.

## Project documents

- [`SPEC.md`](SPEC.md) is the normative v1 behavior.
- [`PLAN.md`](PLAN.md) defines architecture and implementation order.
- [`FUTURE.md`](FUTURE.md) preserves intentionally deferred ideas.
- [`DEPENDENCIES.md`](DEPENDENCIES.md) records the dependency policy.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) explains the contribution workflow.
- [`SECURITY.md`](SECURITY.md) explains private vulnerability reporting and support.
- [`LICENSE`](LICENSE) contains the Apache-2.0 project license.
- [`THIRD_PARTY_NOTICES`](THIRD_PARTY_NOTICES) contains dependency licenses and notices.
