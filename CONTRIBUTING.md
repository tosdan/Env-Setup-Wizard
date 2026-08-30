# Contributing to Env Setup Wizard

Thank you for helping improve Env Setup Wizard. The project is still preparing
its first public release, so keeping the v1 contract small and predictable is
more important than expanding the feature set.

## Before starting

Read the documents that define the part you intend to change:

- [`SPEC.md`](SPEC.md) is the normative v1 behavior.
- [`PLAN.md`](PLAN.md) records the architecture, invariants, and test plan.
- [`FUTURE.md`](FUTURE.md) contains ideas deliberately deferred beyond v1.
- [`DEPENDENCIES.md`](DEPENDENCIES.md) defines dependency and license policy.
- [`SECURITY.md`](SECURITY.md) explains how to report a vulnerability privately.

For a substantial behavioral or architectural change, discuss the proposal in
an issue before investing in an implementation. A deferred item is not already
approved merely because it appears in `FUTURE.md`.

Do not use a public issue or pull request for an undisclosed vulnerability.
Follow `SECURITY.md` instead.

## Development setup

You need Git and Go 1.26 or newer. From a checkout of the repository:

```text
go mod download
go test ./...
go run ./cmd/env-wizard --version
```

The version command should print `env-wizard dev` for an ordinary local build.
Interactive manual tests require a real terminal. Use disposable template and
output paths so that a test cannot overwrite a working project's `.env`.

## Working on a change

1. Start from an up-to-date `main` branch and create a focused branch.
2. Add or update tests with the behavior change.
3. Keep the patch scoped; unrelated cleanup belongs in a separate change.
4. Update `SPEC.md`, `PLAN.md`, or user documentation when their contract changes.
5. Run the local checks below before opening a pull request.

Use `gofmt` on changed Go files. Prefer plain Go and the standard library. New
runtime dependencies require the review described in `DEPENDENCIES.md` and
should sit behind a narrow adapter when practical.

## Architecture and safety invariants

Preserve these boundaries:

- `internal/domain` contains ordered documents and questions without terminal,
  dotenv-library, or filesystem dependencies.
- `internal/dotenv` owns template parsing, Compose-compatible value semantics,
  merge behavior, and canonical rendering.
- `internal/wizard` adapts domain questions to the interactive UI and must never
  expose secret values in summaries or diagnostics.
- `internal/filesystem` owns path checks, backups, staging, and atomic replacement.
- `internal/app` orchestrates the workflow through interfaces rather than
  reimplementing adapter behavior.

In particular:

- never print the content or length of an `@secret` value;
- never let a failed or cancelled run modify the output;
- preserve the existing output until its byte-identical backup is durable;
- keep the template as the source of structure and variable ordering;
- delegate dotenv value semantics to `compose-go` rather than creating a second parser;
- isolate operating-system behavior in build-tagged filesystem or terminal adapters.

Error messages should identify the operation and affected path or field while
remaining safe for secret values. Wrap underlying Go errors when callers need
to inspect them.

## Tests and local checks

Run the following from the repository root:

```text
gofmt -w <changed-go-files>
go test -count=1 ./...
go vet ./...
go mod tidy -diff
go run ./internal/tools/licenses -check
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

Tests must be deterministic and non-interactive. Terminal behavior should be
tested through the wizard adapter; filesystem behavior should use temporary
directories. Add regression coverage for both LF and CRLF when a change affects
document structure or byte rendering.

A cross-compilation is useful but does not replace native tests for terminal or
filesystem behavior. Windows amd64, Linux amd64, and Linux arm64 are stable v1
targets. macOS amd64 and arm64 remain preview until the documented manual smoke
test is completed.

## Dependency changes

After adding, removing, or updating a module:

```text
go mod tidy
go run ./internal/tools/licenses -write
go run ./internal/tools/licenses -check
```

Review `go.mod`, `go.sum`, and `THIRD_PARTY_NOTICES`; do not commit a regenerated
notice without checking the new module, version, license, and attribution. Only
the licenses accepted by `DEPENDENCIES.md` pass automatically.

## Pull requests

A pull request should explain:

- the problem and why it belongs in the current scope;
- the behavioral or architectural decision made;
- the tests run and the platforms exercised;
- any user-facing, compatibility, dependency, or security impact.

Keep commits coherent and messages outcome-oriented. Generated release archives
and local binaries do not belong in a pull request. `THIRD_PARTY_NOTICES` is the
exception when it was intentionally regenerated after a dependency change.

By contributing, you agree that your contribution is licensed under the
repository's [Apache License 2.0](LICENSE).
