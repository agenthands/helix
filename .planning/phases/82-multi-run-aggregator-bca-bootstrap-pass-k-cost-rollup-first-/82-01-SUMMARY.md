---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 01
subsystem: bench
tags: [cost-table, matrix, runs-axis, stats-01, foundation, wave-1]
requires:
  - "cmd/helix-bench cost-table types (CostRow/CostTable/validateCostTable)"
  - "bench/runtime ExpandMatrix + Cell.RunIndex path machinery"
provides:
  - "bench/cost importable package: CostRow, CostTable, LoadCostTable, ValidateCostTable, PriceFor, DateLayout, StalenessWindowDays"
  - "ExpandMatrix runs axis: N cells per (b,l,m,t) with RunIndex 0..N-1"
  - "helix-bench run --runs N (default 3) flag"
affects:
  - "cmd/helix-bench/validate_cost_table.go (now a thin CLI shim)"
  - "Plan 82-04 (bench/aggregator/cost.go) imports bench/cost"
tech-stack:
  added: []
  patterns:
    - "Move-not-duplicate: lift package-main types to an importable package (Open Q1)"
    - "Injected-clock fail-closed freshness gate (D-13)"
    - "Deterministic cartesian expansion with innermost runs axis (D-04)"
key-files:
  created:
    - bench/cost/cost_table.go
    - bench/cost/cost_table_test.go
  modified:
    - cmd/helix-bench/validate_cost_table.go
    - cmd/helix-bench/validate_test.go
    - bench/runtime/matrix.go
    - bench/runtime/matrix_test.go
    - bench/runtime/five_of_six_test.go
    - cmd/helix-bench/main.go
decisions:
  - "Open Q1: MOVE cost-table types to bench/cost (exactly one type CostTable in the tree), not duplicate"
  - "Keep package-main dateLayout/stalenessWindowDays as local copies for the sibling verify-tos validator (no need to import bench/cost there)"
  - "PriceFor treats an unknown model_id as a HARD error (cannot price), matching the D-13 fail-closed discipline"
  - "ExpandMatrix runs<1 clamps to 1 (never an empty matrix)"
metrics:
  duration: "~12m"
  completed: 2026-06-21
---

# Phase 82 Plan 01: bench/cost package + ExpandMatrix runs axis Summary

Moved the cost-table contract + freshness gate into an importable `bench/cost` package (one parser, one fail-closed gate shared by the validate-cost-table CLI and the Plan 04 aggregator) and added a `runs` axis to `ExpandMatrix` so the matrix runner emits N RunIndex cells per (task, mode) — the STATS-01 producer half — wired to a `helix-bench run --runs N` (default 3) flag.

## What Was Built

### Task 1 — `bench/cost` package (Open Q1: MOVE)
- New `bench/cost/cost_table.go` (`package cost`) exporting `CostRow`, `CostTable`, `LoadCostTable(path)`, `ValidateCostTable(today, path)`, `PriceFor(ct, modelID, today)`, and `DateLayout` / `StalenessWindowDays` constants. The validator body is the verbatim move of the former private `validateCostTable` (strict `yaml.NewDecoder` + `KnownFields(true)`, same per-row checks, same fail-closed messages, injected-clock discipline per D-13).
- `PriceFor` is the new per-lookup gate the aggregator (Plan 04) reuses: matches `r.ModelID == modelID`, fails closed on past `valid_until` or `last_verified` >90d stale, and returns a HARD error `cost: no row for model_id %q` for an unknown model (an unknown model cannot be priced).
- `LoadCostTable` does the strict decode once so a caller can load-once / price-many.
- `cmd/helix-bench/validate_cost_table.go` gutted to a thin CLI shim that imports `bench/cost` and calls `cost.ValidateCostTable(time.Now().UTC(), path)`; CLI wording unchanged. The shared `dateLayout`/`stalenessWindowDays` constants remain in package main as local copies for the sibling `verify_tos.go` validator (package-main only).
- `validate_test.go` updated to call `cost.ValidateCostTable` / `cost.CostTable`; all existing cost-table + TOS tests stay green.

### Task 2 — `ExpandMatrix` runs axis (D-04, STATS-01 producer)
- `ExpandMatrix(benchmarks, languages, modes, tasks []string, runs int)`: innermost loop now emits `runs` cells per (b,l,m,t) with distinct `RunIndex 0..runs-1`; `runs < 1` clamps to 1; capacity hint multiplied by `runs`; ordering stays deterministic (runs innermost).
- No path-machinery changes — `cellDurablePaths`, `validateRunIndexSegment`, `runOneCell` are already RunIndex-aware, so distinct RunIndex already lands distinct durable dirs (T-82-01-01 reuse).
- `helix-bench run --runs N` (default 3) wired through `runBenchOpts.Runs` into the `runtime.ExpandMatrix(...)` call.

### Task 3 — N-cell test scaffold (D-04)
- `TestExpandMatrixRuns`: `ExpandMatrix(..., 3)` over 2 tasks → 6 cells; each task has exactly RunIndex {0,1,2} (set, not order-dependent); `ExpandMatrix(..., 0)` clamps to one cell per task with RunIndex 0. This is a PURE test — runs WITHOUT `HELIX_BIN`.
- Updated all pre-existing `ExpandMatrix(...)` callers (`matrix_test.go` x5, `five_of_six_test.go` x1) to the new signature, passing `1` to preserve single-rep assertions.

## Deviations from Plan

None — plan executed exactly as written. The shared-constant handling (re-declaring `dateLayout`/`stalenessWindowDays` locally in package main for `verify_tos.go`) is the planned "re-declare a local copy or import them" branch from Task 1.

## Verification Evidence

All commands run from the repo root.

| Command | Result |
|---|---|
| `go build -o helix ./cmd/helix` | OK |
| `go build ./cmd/helix-bench` | OK |
| `go build ./...` | OK |
| `go vet ./...` | clean (VET DONE, exit 0) |
| `make vet` (5 custom analyzers: vet-noduckdb, vet-nokernel2semantic, vet-nosemantic2kernel, vet-compact-uses-store, vet-ablation-leakage) | exit 0 |
| `go test ./cmd/helix-bench/... ./bench/cost/...` | ok (both packages) |
| `go test ./bench/runtime/ -run TestExpandMatrix -v` | PASS — `TestExpandMatrixRuns` **RUN** (not SKIP) + PASS, all pre-existing matrix tests PASS |
| `go test ./...` | exit 0, zero failures |
| `HELIX_BIN="$(git rev-parse --show-toplevel)/helix" go test ./bench/runtime/ -count=1` | ok 33.877s — daemon-gated integration tests **RAN** (not skipped), no regression |
| `grep -rn "type CostTable" --include="*.go" .` | exactly one definition (`bench/cost/cost_table.go`) |

Confirmed `TestExpandMatrixRuns` RUNS (not skip): the HELIX_BIN-gated false-green trap does not apply — `ExpandMatrix` needs no daemon, so the pure test executed under plain `go test`. The daemon-gated integration tests were separately re-run WITH `HELIX_BIN` (33.8s wall = real daemon spawns) to prove no regression.

## Post-test Cleanup

- Removed the regenerable `bench/runtime/.helix/` daemon workspace dir after the HELIX_BIN run.
- Removed stray `helix` (gitignored) and `helix-bench` build artifacts from the repo root; neither was committed.

## Known Stubs

None. Both deliverables (the `bench/cost` package and the N-aware `ExpandMatrix`) are fully wired and exercised by tests.

## Threat Flags

None. No new network endpoints, auth paths, or schema changes. The two trust boundaries in the plan's threat model (RunIndex path segment; cost-table freshness) are handled by reusing the existing `validateRunIndexSegment` unchanged (only fed 0..N-1 non-negative ints) and by carrying the D-13 fail-closed freshness gate verbatim into `bench/cost`.

> Note (deferred, out of scope): the `helix-bench` build artifact at the repo root is NOT in `.gitignore` (the `helix` binary is). A follow-up could add `helix-bench` to `.gitignore`; this plan touches no build config.

## Self-Check: PASSED

- FOUND: bench/cost/cost_table.go
- FOUND: bench/cost/cost_table_test.go
- FOUND: commit dccf8352 (Task 1)
- FOUND: commit abcae78c (Task 2)
- FOUND: commit 88f62367 (Task 3)
