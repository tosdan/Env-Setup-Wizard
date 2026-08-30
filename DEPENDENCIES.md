# Dependency policy

Env Setup Wizard keeps runtime dependencies few and places them behind internal adapters. The selection order is:

1. Go standard library when it is sufficient.
2. A mature project with a permissive license when substantial behavior would otherwise be reimplemented.
3. Small, focused custom code covered through the module interface.

## Direct runtime dependencies

| Module | Current version | Role | License | Isolation |
| --- | --- | --- | --- | --- |
| `charm.land/huh/v2` | `v2.0.3` | Interactive terminal wizard | MIT | `internal/wizard` adapter |
| `github.com/compose-spec/compose-go/v2` | `v2.14.0` | Compose-compatible dotenv semantics | Apache-2.0 | `internal/dotenv` adapter |
| `github.com/charmbracelet/x/term` | `v0.2.2` | Cross-platform TTY detection | MIT | `internal/app` runtime setup |
| `golang.org/x/sys` | `v0.42.0` | Windows `MoveFileExW` replacement | BSD-3-Clause | Build-tagged `internal/filesystem` adapter |

`golang.org/x/sys` became a direct dependency in Phase 8 when the Windows
replacement adapter began calling `MoveFileExW`. Non-Windows builds exclude that
adapter through build tags.

## Admission rules

A new dependency must be reviewed for maintenance activity, versioning policy, known vulnerabilities, transitive dependency count, standard-library alternatives, and whether it can be isolated behind an adapter.

Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause, and ISC are accepted automatically. Any other license requires an explicit review before introduction. CI and releases must run vulnerability and license checks and keep third-party notices current.

## Phase 0 decision record

- Huh and compose-go provide substantial behavior at narrow seams and are accepted.
- `x/term` was promoted from Huh's transitive dependencies when the workflow began enforcing its explicit cross-platform TTY preflight; it avoids duplicating platform-specific terminal detection.
- `x/sys/windows` was promoted when the safe writer required the native Windows replacement primitive; the dependency remains isolated behind `replaceFile`.
- No generic validation framework, CLI framework, or logging dependency is accepted for v1.
- No exception to the license policy is currently required.

## License inventory

`THIRD_PARTY_NOTICES` contains the license and notice files for the union of Go
modules linked into the five CGO-disabled v1 release targets. Regenerate and
verify it with:

```text
go run ./internal/tools/licenses -write
go run ./internal/tools/licenses -check
```

The verifier derives the inventory from `go list` for every target and rejects
missing, unrecognized, or unapproved top-level license files. A dependency
change therefore requires an explicit review of the regenerated notice.

The disposable Phase 0 experiments and their implications are recorded in [`docs/phase-0-spikes.md`](docs/phase-0-spikes.md).
