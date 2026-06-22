---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 05
subsystem: bench/aggregator
tags: [STATS-01, aggregator, n-gate, fail-closed, result.v2, tdd]
requires:
  - bench/runtime.Validate (schema validate-on-read)
  - bench/runtime.BuildResult / ResultInput (test fixtures)
  - bench/evaluators.Metrics (nullable pointer contract mirrored by rowMetrics)
provides:
  - aggregator.Load(runDir, expectedN) (*Loaded, error)
  - aggregator.Loaded / Row / rowMetrics (grouped valid rows by task/mode)
  - fail-closed N-gate (D-05) consumed by Plan 06 orchestrator
affects:
  - bench/aggregator (new load.go + load_test.go)
tech-stack:
  added: []
  patterns:
    - "group-by-(task,mode) + per-row decode preserving full doc + nullable metrics (deltas.go analog)"
    - "fail-closed gate: deficiency list -> hard error, nil result, write nothing (validateCostTable shape)"
    - "valid-row = exists + decode + runtime.Validate==nil (NOT file-present)"
    - "expectedN from caller arg, never len(glob) on disk (Pitfall 3)"
    - "nil metric stays nil, never fabricated 0 (Pitfall 4)"
key-files:
  created:
    - bench/aggregator/load.go
    - bench/aggregator/load_test.go
  modified: []
decisions:
  - "rowMetrics mirrors evaluators.Metrics pointer types (*bool/*int/*float64) per plan, broader than deltas.go's *float64-only subset, so the nil-preservation test asserts TokensInput==nil"
  - "discovered-but-all-invalid cells are still recorded (ensureCell) so the gate flags them deficient rather than silently absent"
  - "glob rooted at runDir via filepath.Join('*','*','*'); non-numeric/negative run_index segments skipped (T-82-05-02 path-traversal mitigation)"
metrics:
  duration: ~3m
  completed: 2026-06-20
---

# Phase 82 Plan 05: Aggregator result.v2 Loader + Fail-Closed N-Gate Summary

Implements the STATS-01 consumer half: `aggregator.Load(runDir, expectedN)` globs the durable
`<runDir>/<task>/<mode>/<run_index>/result.v2.json` tree, groups VALID rows by (task,mode) preserving
the full document plus a nullable-pointer metric subset, and FAILS CLOSED — any cell with fewer than
`expectedN` valid rows yields a hard error naming every deficient cell and returns a nil result so the
aggregator writes nothing.

## What Was Built

- **`Load(runDir string, expectedN int) (*Loaded, error)`** — globs the durable layout, validates each
  row, groups by (task,mode), enforces the N-gate. Returns `(*Loaded, nil)` only when every discovered
  cell has >= expectedN valid rows; otherwise `(nil, error)`.
- **`Loaded`** — grouped valid rows with deterministic ordering (`Rows(task, mode) []Row`, `Tasks()`).
- **`Row`** — `{RunIndex, Doc map[string]json.RawMessage, Metrics rowMetrics}`; full doc preserved
  verbatim for write-back, metrics decoded into nullable pointers.
- **`rowMetrics`** — mirrors `evaluators.Metrics` pointer types (`TaskSuccess *bool`, `TokensInput *int`,
  `EditLocality *float64`, …). Absent/null stays nil; never decoded to a fabricated 0.
- **Helpers** — `globRows` (rooted enumeration), `validRow` (read + decode + `runtime.Validate`),
  `deficientCells` (sorted `<task>/<mode>: got X want N`), `ensureCell` (discovery of all-invalid cells).

## Correctness Gate Compliance

- **Fail-closed (D-05):** deficient cell -> `fmt.Errorf("aggregate: insufficient runs: %v", deficient)`,
  result is nil. Caller writes nothing. Multiple deficient cells are all named (sorted).
- **expectedN from arg not disk (Pitfall 3):** the gate compares `len(validRows)` against the function
  argument, never `len(glob)`. The invalid-row test proves it: 3 files on disk, one fails
  `runtime.Validate` -> 2 valid -> `got 2 want 3` fail-closed.
- **Invalid row counts as deficient:** a row is valid iff it exists AND JSON-decodes AND
  `runtime.Validate(b)==nil`. A garbage `{not valid}` file is excluded, not silently skipped to pass.
- **Null discipline (Pitfall 4):** `TokensInput` left nil in the source Metrics decodes to a nil pointer;
  `TaskSuccess` present stays set. Asserted by `TestLoadNilMetricPreserved`.

## TDD Gate Compliance

- RED commit `626a25ec` (`test(82-05): ...`) preceded GREEN commit `cdd1e194` (`feat(82-05): ...`).
- RED evidence: `go test ./bench/aggregator/ -run TestNGate` => `undefined: Load` / `[build failed]` (RED-OK).
- GREEN evidence: `TestNGate` 4 subtests + `TestLoadNilMetricPreserved` all PASS.
- No REFACTOR commit needed — helpers (`globRows`/`validRow`/`deficientCells`) were extracted in the
  same GREEN pass and tests stayed green.

## Verification Commands

| Command | Result |
|---|---|
| `go test ./bench/aggregator/ -run 'TestNGate\|TestLoad' -v` | PASS (TestNGate/4 subtests + TestLoadNilMetricPreserved) |
| `go build ./...` | clean |
| `go vet ./bench/aggregator/...` | clean |
| `make vet` (incl. duckdb/kernel/semantic/ablation vettools) | clean |
| `go test ./...` | exit 0, no failures |
| `gofmt -w bench/aggregator/load.go bench/aggregator/load_test.go` | applied |

No HELIX_BIN required — pure fixtures via `runtime.BuildResult` + `runtime.Validate` (D-01).

## Deviations from Plan

None - plan executed exactly as written. The plan's recommended public surface
(`Load(runDir, expectedN) (*Loaded, error)` with grouped per-(task,mode) rows carrying full doc +
nullable rowMetrics) was implemented as specified.

## Threat Mitigations Applied

- **T-82-05-01** (disk-count partial matrix): expectedN is the arg; valid-row re-validates via
  `runtime.Validate`; invalid-file-counts-as-deficient test locks it.
- **T-82-05-02** (path traversal): glob is rooted at runDir via `filepath.Join`; only discovered entries
  under runDir are joined; non-numeric/negative run_index segments are skipped.
- **T-82-05-03** (fabricated 0): `rowMetrics` uses pointer types; nil stays nil; preservation test asserts.

## Self-Check: PASSED

- FOUND: bench/aggregator/load.go
- FOUND: bench/aggregator/load_test.go
- FOUND commit: 626a25ec (RED)
- FOUND commit: cdd1e194 (GREEN)
