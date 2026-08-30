# Dependency policy

Env Setup Wizard keeps runtime dependencies few and places them behind internal adapters. The selection order is:

1. Go standard library when it is sufficient.
2. A mature project with a permissive license when substantial behavior would otherwise be reimplemented.
3. Small, focused custom code covered through the module interface.

## Direct runtime dependencies

| Module | Version at Phase 0 | Role | License | Isolation |
| --- | --- | --- | --- | --- |
| `charm.land/huh/v2` | `v2.0.3` | Interactive terminal wizard | MIT | `internal/wizard` adapter |
| `github.com/compose-spec/compose-go/v2` | `v2.14.0` | Compose-compatible dotenv semantics | Apache-2.0 | `internal/dotenv` adapter |

`golang.org/x/sys` is currently present only as a transitive dependency of Huh. It will not become a direct dependency until the Windows replacement adapter actually needs `golang.org/x/sys/windows`; appearing in the design is not sufficient reason to promote it.

## Admission rules

A new dependency must be reviewed for maintenance activity, versioning policy, known vulnerabilities, transitive dependency count, standard-library alternatives, and whether it can be isolated behind an adapter.

Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause, and ISC are accepted automatically. Any other license requires an explicit review before introduction. CI and releases must run vulnerability and license checks and keep third-party notices current.

## Phase 0 decision record

- Huh and compose-go provide substantial behavior at narrow seams and are accepted.
- No generic validation framework, CLI framework, or logging dependency is accepted for v1.
- No exception to the license policy is currently required.

The disposable Phase 0 experiments and their implications are recorded in [`docs/phase-0-spikes.md`](docs/phase-0-spikes.md).
