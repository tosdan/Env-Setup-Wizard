# Env Setup Wizard

Env Setup Wizard is a Go CLI that turns an annotated `.env.example` into an interactive setup wizard and writes a validated `.env` safely.

The project has completed **Phase 8**: the end-to-end interactive workflow now
merges an existing `.env`, validates and summarizes the result, detects no-op
runs, and writes through a synced temporary file with atomic replacement. Every
effective overwrite first creates a byte-identical, timestamped backup.

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

`--version` and the v1 command-line options are available. A declined confirmation
or byte-identical result performs no write. `--force` skips only the final
confirmation; validation, summary, no-op detection, and overwrite backups remain
active.

Backups may contain the same secrets as `.env`. Keep `.env*` out of version
control while explicitly retaining `.env.example`, as this repository's
`.gitignore` does. Atomicity and durability ultimately depend on the underlying
filesystem and may be weaker on network shares or unusual filesystems. Unix
overwrites preserve permission bits; exact ownership, ACL, and metadata
preservation is not guaranteed, and Windows replacements derive ACLs from the
directory.
