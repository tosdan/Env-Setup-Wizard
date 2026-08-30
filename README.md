# Env Setup Wizard

Env Setup Wizard is a Go CLI that turns an annotated `.env.example` into an interactive setup wizard and writes a validated `.env` safely.

The project has completed **Phase 7**: template parsing, annotations, validation,
existing `.env` merge and recovery, the interactive wizard, the grouped summary,
no-op detection, and create/overwrite confirmation are in place. Safe atomic
writes and backups are the next implementation stage.

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

`--version` and the v1 command-line options are available. The workflow handles a
declined confirmation or a byte-identical existing file without writing. When a
write is accepted (or `--force` is used), it currently stops at the Phase 8
safe-write boundary and leaves the filesystem unchanged.
