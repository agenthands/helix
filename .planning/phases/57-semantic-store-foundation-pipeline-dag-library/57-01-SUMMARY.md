---
phase: 57
plan: 01
subsystem: phasegraph
tags: [phasegraph, dag, stdlib, tdd]
requirements_addressed: [DAG-01, DAG-02, DAG-03, DAG-04]
dependency_graph:
  requires: []
  provides:
    - "internal/phasegraph"
    - "internal/phasegraph/pipelines"
    - "PhaseSpec / PhaseID / PhaseDeps / PhaseOutput / PhaseGraph / PhaseGraphResult"
    - "ValidatePhaseGraph / ValidatePhaseGraphWithOptions / ValidateOptions"
    - "RunPhaseGraph / ShutdownCompleted / WriteDOT"
    - "SemanticIndexPhases (12) / LiveUpdatePhases (9) / EvalPhases (10)"
  affects:
    - "internal/phasegraph (new package)"
tech_stack:
  added: []
  patterns:
    - "stdlib-only library (context, errors, fmt, io, sort, strings, time)"
    - "iterative three-color DFS for cycle detection"
    - "Kahn's algorithm with sorted-by-PhaseID determinism"
    - "errors.Join aggregation in ShutdownCompleted"
    - "options struct as backward-compatible API extension"
    - "shape-only []PhaseSpec literals with noopRun placeholder bodies"
key_files:
  created:
    - internal/phasegraph/phase.go
    - internal/phasegraph/dag.go
    - internal/phasegraph/validate.go
    - internal/phasegraph/run.go
    - internal/phasegraph/shutdown.go
    - internal/phasegraph/dot.go
    - internal/phasegraph/pipelines/semantic.go
    - internal/phasegraph/pipelines/live.go
    - internal/phasegraph/pipelines/eval.go
    - internal/phasegraph/phasegraph_test.go
    - internal/phasegraph/pipelines/pipelines_test.go
  modified: []
decisions:
  - "Two ValidatePhaseGraph entry points: ValidatePhaseGraph(phases) preserves the SPEC §39.3 zero-options contract; ValidatePhaseGraphWithOptions(phases, opts) takes a ValidateOptions struct. The first delegates to the second so behavior cannot drift."
  - "DOT writer errors are best-effort suppressed in the failure path (debug output must not mask the real PhaseGraphError)."
  - "Cycle detection is iterative DFS with sorted child traversal — deterministic cycle reports across runs (important for downstream golden-file tests in P02-P04)."
  - "PhaseDeps is filtered to declared Requires only — a phase that forgot to declare a dep cannot accidentally read it."
  - "noopRun placeholder Run bodies in pipelines/* are explicitly documented as DAG-02 contract — Phase 59-62 (semantic), Phase 60 (live), Phase 67 (eval) replace them with real implementations."
metrics:
  duration_minutes: 6
  commits: 3
  files_created: 11
  files_modified: 0
  tests_passing: 14
  loc_added: 1134
  completed: 2026-05-03
---

# Phase 57 Plan 01: Phasegraph DAG library Summary

stdlib-only `internal/phasegraph/` library validates phase DAGs (duplicate / missing-dep / cycle), executes via Kahn topo-sort, shuts down in reverse-topo order, and ships three shape-only pipeline declarations (semantic-index, live-update, eval) for P59-62/P60/P67 to fill in.

## Objective

Lock the DAG-orchestrator contract from SPEC §39.2-§39.9 before downstream phases depend on it. Independent of P02-P04 — runs in wave 1.

## What shipped

### Library (`internal/phasegraph/`)

- **phase.go** — Verbatim SPEC §39.2 types: `PhaseID`, `PhaseSpec`, `PhaseDeps`, `PhaseOutput`, `PhaseRunFunc`, `PhaseValidateFunc`, `PhaseShutdownFunc`, `PhaseGraph`, `PhaseGraphResult` (with `Record`).
- **dag.go** — Internal `adjList` + `buildPhaseGraph`, `findDuplicateIDs`, `findMissingDependencies`, `findCycle` (iterative three-color DFS with deterministic sorted iteration), `kahnSort` (deterministic by sorted PhaseID), `reverse`.
- **validate.go** — `ValidatePhaseGraph(phases)` (SPEC §39.3 entrypoint) plus `ValidatePhaseGraphWithOptions(phases, opts ValidateOptions)` overload supporting `DOTSink io.Writer`. `PhaseGraphError` typed error with `Kind` / `Phase` / `Missing` / `Cycle` fields.
- **run.go** — `RunPhaseGraph(ctx, *PhaseGraph) (*PhaseGraphResult, error)` per SPEC §39.8: per-phase `Validate` hook, on Run/Validate failure invokes `ShutdownCompleted` for already-recorded outputs in reverse-topo order. `buildPhaseDeps` filters outputs to declared `Requires` only.
- **shutdown.go** — Public `ShutdownCompleted(ctx, *PhaseGraph, outputs)` walks `ShutdownOrder`, skips phases that never completed, aggregates errors via `errors.Join`. Never panics.
- **dot.go** — `WriteDOT(io.Writer, []PhaseSpec)` emits a deterministic Graphviz `digraph` (sorted nodes + edges).

### Pipeline shapes (`internal/phasegraph/pipelines/`)

- **semantic.go** — 12 typed `PhaseID` consts + `SemanticIndexPhases` (SPEC §39.5: discover_files → … → initialize_graph_cache).
- **live.go** — 9 typed consts + `LiveUpdatePhases` (SPEC §39.6: collect_events → … → enqueue_lsp_revalidation).
- **eval.go** — 10 typed consts + `EvalPhases` (SPEC §39.7: prepare_workspace → … → aggregate_report).
- All Run bodies are the shared `noopRun` placeholder; doc comments call out which downstream phases (Phase 59-62 / Phase 60 / Phase 67) replace them — DAG-02 contract.

### Tests

- `internal/phasegraph/phasegraph_test.go` — 10 tests: happy path, duplicate, missing-dep, single-node cycle, multi-node cycle, run happy path, run-error-triggers-shutdown, WriteDOT, ValidatePhaseGraphWithOptions DOT-on-cycle, no-DOT-on-success.
- `internal/phasegraph/pipelines/pipelines_test.go` — 4 tests: semantic / live / eval shape validation, `TestPipelineCount_MatchesSpec` locks 12/9/10 cardinality vs SPEC §39.5/§39.6/§39.7.
- All 14 tests pass under `go test -race -count=1`. `go vet` clean. Zero non-stdlib imports under `internal/phasegraph/`.

## Commit timeline (TDD gates)

| Gate     | Commit     | Type     | Description |
|----------|------------|----------|-------------|
| RED      | `a7990db3` | test     | 11 RED tests; package does not exist → build fails |
| GREEN    | `f4f916e1` | feat     | 9 production files; 11 GREEN tests; -race clean |
| REFACTOR | `ec7022c3` | refactor | `ValidatePhaseGraphWithOptions` + 3 new tests; doc-comment markers |

## Success criteria status

- [x] All tests in `internal/phasegraph/...` pass with `-race -count=1` (14/14)
- [x] `go vet ./internal/phasegraph/...` clean
- [x] Stdlib-only invariant: zero third-party imports under `internal/phasegraph/`
- [x] Three pipeline shape declarations validate cleanly via `ValidatePhaseGraph` and contain SPEC §39.5/§39.6/§39.7 phase ID set verbatim
- [x] DAG-04 setup: `Phase 59-62`, `Phase 60`, `Phase 67` doc-comment landing zones present in pipelines/*.go for downstream consumers

## Deviations from Plan

None — plan executed exactly as written. Two minor adjustments:

1. **gofmt on `pipelines/eval.go`**: Initial Write used misaligned column widths inside the `const ( ... )` block. `gofmt -w` realigned them; alignment-only change.
2. **`TestValidate_NoDOTOnSuccess` added (extra)**: Plan called for `TestValidate_WritesDOTOnCycleWhenWriterProvided` only. I added a sibling test asserting the sink is NEVER written on success. This is a strengthening of the contract, not a divergence from it; counts as a Rule 1-style auto-fix (correctness — without the negative test, a future regression that always wrote DOT would still pass the positive test).

## Authentication gates

None.

## Known stubs

The `noopRun` placeholders in `pipelines/{semantic,live,eval}.go` are intentional stubs documented as DAG-02 contract. They are NOT regression-stubs — they are the v1.10 shape contract that downstream phases (P59-62 semantic indexing, P60 live updates, P67 eval) will fill in. Doc comments on each `*Phases` slice declaration call this out explicitly.

## Threat surface scan

No new surface introduced beyond what the plan's `<threat_model>` enumerated:

- **T-57-01-01 (Tampering — validate-before-run):** Mitigated by API shape — `RunPhaseGraph` accepts `*PhaseGraph` only, which is only obtainable from `ValidatePhaseGraph`/`ValidatePhaseGraphWithOptions`. There is no public `RunPhases([]PhaseSpec)` overload.
- **T-57-01-02 (DoS — runaway phase):** Documented in package doc — per-phase timeouts are caller responsibility (`context.WithTimeout`).
- **T-57-01-03 (Info disclosure — DOT output):** DOT output contains only declared phase IDs and dependency edges. Both are static developer-chosen constants. No file paths, env vars, or runtime data leak. Documented in `dot.go` and the package doc.
- **T-57-01-04 (EoP):** N/A — library has no privileged operations.

## Self-Check

**Files created (11):**

- `[FOUND]` internal/phasegraph/phase.go
- `[FOUND]` internal/phasegraph/dag.go
- `[FOUND]` internal/phasegraph/validate.go
- `[FOUND]` internal/phasegraph/run.go
- `[FOUND]` internal/phasegraph/shutdown.go
- `[FOUND]` internal/phasegraph/dot.go
- `[FOUND]` internal/phasegraph/pipelines/semantic.go
- `[FOUND]` internal/phasegraph/pipelines/live.go
- `[FOUND]` internal/phasegraph/pipelines/eval.go
- `[FOUND]` internal/phasegraph/phasegraph_test.go
- `[FOUND]` internal/phasegraph/pipelines/pipelines_test.go

**Commits (3):**

- `[FOUND]` a7990db3 — test(57-01): add failing tests
- `[FOUND]` f4f916e1 — feat(57-01): implement phasegraph library
- `[FOUND]` ec7022c3 — refactor(57-01): add ValidatePhaseGraphWithOptions

## Self-Check: PASSED
