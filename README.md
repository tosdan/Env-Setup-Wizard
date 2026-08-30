# Env Setup Wizard

Env Setup Wizard is a Go CLI that turns an annotated `.env.example` into an interactive setup wizard and writes a validated `.env` safely.

The project has completed **Phase 6**: template parsing, annotations, validation,
configuration rendering, the question model, and the interactive wizard adapter
are in place. Existing `.env` merge, summary and confirmation, and safe writes
are the next implementation stages.

## Project documents

- [`SPEC.md`](SPEC.md) is the normative v1 behavior.
- [`PLAN.md`](PLAN.md) defines architecture and implementation order.
- [`FUTURE.md`](FUTURE.md) preserves intentionally deferred ideas.
- [`DEPENDENCIES.md`](DEPENDENCIES.md) records the dependency policy.

## Development check

Go 1.26 or newer is required.

```text
go test ./...
go run ./cmd/env-wizard --version
```

`--version` and the v1 command-line options are available. The workflow currently
stops after collecting and rendering the answers in memory; it does not write the
output until the remaining confirmation and safe-write stages are implemented.
