# Env Setup Wizard

Env Setup Wizard is a Go CLI that turns an annotated `.env.example` into an interactive setup wizard and writes a validated `.env` safely.

The project is currently at **Phase 0**: its v1 contracts and repository foundations are in place, while the wizard implementation has not started yet.

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

The command currently prints `env-wizard dev`; user-facing generation will be implemented in the subsequent phases.
