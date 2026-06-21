---
phase: 78-internal-toolbench-go-first-languagerunner-interface
plan: 02
subsystem: bench/runtime matrix + cell orchestrator + helix-bench CLI
tags: [language-axis, runner-dispatch, store-opt-in, seed-migration, cutover, tdd]
requires:
  - bench/languages.RunnerFor (Plan 01 registry; nil = verify.sh fallback)
  - bench/languages/go.GoRunner registered for (internal-toolbench, go) (Plan 01 init)
  - internal/eval/sandbox.WithWorkingDir + subprocess.StartDaemon variadic opts (Plan 01 D-03)
  - bench/runtime.RunCell spine (Phase 77 D-04/D-06/D-08)
provides:
  - bench/runtime.Cell.Language axis + 4-way ExpandMatrix(benchmarks, languages, modes, tasks)
  - bench/runtime.cellSeedDir join <root>/<benchmark>/<lang>/<task> (D-07)
  - bench/runtime.CellConfig.Language + StoreOptIn; RunCell RunnerFor dispatch (D-10)
  - writeCellConfig(storeOptIn) semantic_index.enabled toggle + WithWorkingDir wiring (D-01/D-02/D-03)
  - helix-bench run --languages flag; --benchmarks default flipped to internal-toolbench
  - migrated seed bench/datasets/internal-toolbench/go/IT-go-patch-apply-1 (capability=patch_apply)
affects:
  - Plans 03/04 (corpus fixtures land under the live internal-toolbench/<lang> root)
  - Plan 04 (store-on incremental_update cell uses this StoreOptIn->WithWorkingDir wiring)
  - Phase 85 (drops 7 languages with no matrix rewrite once <lang> is an axis)
tech-stack:
  added: []
  patterns:
    - "4-way cartesian product with per-axis V5 validation before any path join (T-78-03)"
    - "Derive-from-capability store opt-in (incremental_update) with task.json semantic_index override"
    - "git mv seed relocation preserving rename continuity (R100/R080/R073 in git diff)"
key-files:
  created:
    - .planning/phases/78-internal-toolbench-go-first-languagerunner-interface/deferred-items.md
  modified:
    - bench/runtime/matrix.go
    - bench/runtime/matrix_test.go
    - bench/runtime/cell.go
    - bench/runtime/cell_test.go
    - bench/runtime/cross_cell_test.go
    - bench/runtime/daemon_tap_integration_test.go
    - bench/runtime/result_test.go
    - bench/runtime/cctap_test.go
    - cmd/helix-bench/main.go
    - cmd/helix-bench/run_cmd_test.go
    - Makefile
    - bench/BENCH.md
    - bench/LICENSES.md
  renamed:
    - bench/datasets/toolbench-go/sum-doubler/* -> bench/datasets/internal-toolbench/go/IT-go-patch-apply-1/* (6 files, git mv)
decisions:
  - "StoreOptIn derived from seed task.json capability==incremental_update; explicit semantic_index:bool overrides (D-02 escape hatch)"
  - "D-10 dispatch maps RunTests.Passed -> VerifyExitCode (0/1); a non-nil RunTests error routes through preserve like a verify infra error (WR-01 semantics retained)"
  - "Go runner registered via blank import in cell.go (co-located with the RunnerFor consumer) so the registry is populated wherever RunCell is linked"
  - "discoverTasks scans <benchmark>/<lang>/ as a de-duplicated UNION across --languages; a missing language dir is non-fatal, wholly-empty discovery is a hard error"
  - "seed capability=patch_apply (D-08 discretion): the replace_in_file edit -> go test maps to patch apply"
metrics:
  duration: ~30min
  completed: 2026-06-17
---

# Phase 78 Plan 02: Matrix Language Axis + Runner Dispatch + Seed Cutover Summary

The spine now speaks `<benchmark>/<lang>/<task>`: `Cell` gained a `Language`
axis, `ExpandMatrix` became a 4-way product, `RunCell` grades outcomes through
`languages.RunnerFor` (Go runner for `internal-toolbench/go`, `verify.sh`
fallback otherwise), the per-cell semantic store is parameterized by a
capability-derived `StoreOptIn` (wiring `WithWorkingDir` for store-on cells), and
the Phase 77 seed was `git mv`-relocated to
`internal-toolbench/go/IT-go-patch-apply-1` with full history — keeping
`make bench-quick` green (1/1 cell, ~6s).

## What Was Built

### Task 1 — Language axis + RunnerFor dispatch + store opt-in (TDD RED -> GREEN)
- `matrix.go`: `Cell.Language` field; `ExpandMatrix(benchmarks, languages,
  modes, tasks)` is the 4-way cartesian product with `validateMatrixID(lang,
  "language")` in the validation loop (T-78-03 path-traversal). New `cellSeedDir`
  joins `<DatasetsRoot>/<benchmark>/<language>/<task>` (D-07). `runOneCell`
  threads `Language` into `CellConfig` and derives `StoreOptIn` from the seed
  `task.json` via the new `deriveStoreOptIn` helper (capability ==
  `incremental_update`, with an explicit `semantic_index` bool override as the
  D-02 escape hatch).
- `cell.go`: `CellConfig.Language` + `CellConfig.StoreOptIn`;
  `validateCellKey(cfg.Language, "language")` in the defensive block; the outcome
  step now dispatches `languages.RunnerFor(cfg.Benchmark, cfg.Language)` —
  `RunTests` when non-nil (`Passed` -> `VerifyExitCode` 0/1, a non-nil error
  routed through `preserve`), else the unchanged `runVerify(verify.sh)` fallback
  (D-10). `writeCellConfig` gained a `storeOptIn bool` and emits
  `semantic_index.enabled: %v`; a `StoreOptIn` cell passes
  `WithWorkingDir(repoDir)` into `StartDaemon` (D-01/D-02/D-03). The Go runner is
  blank-imported here for registration. The stale "ABSOLUTE per-cell store path"
  comments (Pitfall 5) were reconciled to describe the real enabled-toggle +
  per-cell-cwd behavior.
- Tests: `ExpandMatrix` single-cell + 4-way count + language path-traversal
  rejection; `cellSeedDir` lang join; `writeCellConfig` store-off/store-on;
  `RunnerFor` registered/nil dispatch; a Pitfall-5 stale-comment guard.

RED commit `755b3589` (referenced not-yet-existing symbols / new 4-way signature)
-> GREEN commit `34c928ca`.

### Task 2 — Seed git mv + --languages flag + single clean cutover (D-08/D-09)
- `git mv bench/datasets/toolbench-go/sum-doubler
  bench/datasets/internal-toolbench/go/IT-go-patch-apply-1` (old dir removed); all
  6 files renamed with history preserved (`git log --follow` reaches the Phase 77
  `feat(77-01)` seed commit). Relocated `task.json`:
  `id=IT-go-patch-apply-1`, `benchmark=internal-toolbench`,
  `capability=patch_apply`. The fixture's `sum.go` package comment updated.
- `cmd/helix-bench/main.go`: `--languages` `StringArrayVar` (default `[go]`);
  `--benchmarks` default flipped to `internal-toolbench`; `Languages []string` in
  `runBenchOpts`; the 4-way `ExpandMatrix` call; `discoverTasks` now scans
  `<benchmark>/<lang>/` as a de-duplicated union across languages.
- Cutover: `Makefile` (`SUITE=internal-toolbench`, `bench-quick` `--languages=go`
  `--tasks=IT-go-patch-apply-1`, comments), `bench/BENCH.md`,
  `bench/LICENSES.md`, and all bench/runtime + cmd test literals.

Commit `4f4df69a`. `make bench-quick` GREEN: 1/1 cells succeeded in ~6s (the cell
graded its outcome through the Go runner's `go test ./... -json`, exercising the
D-10 dispatch end-to-end).

## Verification Results

- `go test ./bench/... ./cmd/helix-bench/... -count=1` — PASS.
- `go vet ./...` — clean.
- `gofmt -l bench/runtime/` — empty.
- `make bench-quick` — exit 0, 1/1 cell, ~6s (≤90s budget; D-09 gate).
- Source assertions: `grep -q 'Language' bench/runtime/matrix.go`,
  `grep -q 'RunnerFor' bench/runtime/cell.go`,
  `grep -q 'WithWorkingDir' bench/runtime/cell.go` all pass; the stale
  "absolute per-cell store path" phrasing is gone (Pitfall 5).
- FS: `internal-toolbench/go/IT-go-patch-apply-1/task.json` present;
  `bench/datasets/toolbench-go` absent; `capability=patch_apply` present.
- `git log --follow` on the moved `sum.go` reaches `feat(77-01)` (D-08 history).
- Repo-wide: no `toolbench-go`/`sum-doubler` literals remain in `.go`/`Makefile`.

## Success Criteria Met

- **D-07**: `Cell.Language` axis + `--languages` flag + V5-validated `<lang>`
  segment + `<lang>` seed join.
- **D-08**: seed migrated via `git mv` with history; `IT-go-patch-apply-1` id +
  `capability=patch_apply`.
- **D-09**: single clean cutover; `make bench-quick` green; no stale
  `toolbench-go`.
- **D-10**: RunCell dispatches via `RunnerFor` (Go runner's RunTests for the
  internal-toolbench/go cell) with the verify.sh fallback intact.
- **D-01/D-02/D-03 wiring**: `writeCellConfig` parameterizes
  `semantic_index.enabled` from `StoreOptIn`; store-on cells pass
  `WithWorkingDir(repoDir)`.

## Deviations from Plan

None — plan executed exactly as written. (The seed's `sum.go` package-comment
literal and the `cmd` empty-dir/unknown-agent test literals were swept as part of
the D-09 "no stale literals in active code" acceptance, which the plan calls for.)

## Threat Surface

- **T-78-03 (path traversal, `<lang>` segment)** — mitigated:
  `validateMatrixID(lang, "language")` in `ExpandMatrix` AND
  `validateCellKey(cfg.Language, "language")` in `RunCell`; unit tests reject
  `..`/slash/leading-dot languages in both layers.
- **T-78-04 (store-on cwd deadlock)** — substrate landed: `StoreOptIn` cells pass
  `WithWorkingDir(repoDir)` so `.helix/semantic.duckdb` resolves per-cell;
  store-off cells (the default, including the migrated seed) keep
  `enabled:false`. The full `--parallel` isolation assertion lands in Plan 04.
- **T-78-05 (stale cutover residue)** — accepted/verified: the old dir is gone and
  no stale literals remain in active code/Makefile.

No new external-input paths beyond the validated `--languages` CLI segment.

## Deferred Issues

Pre-existing, out-of-scope failures (logged in `deferred-items.md`, NOT fixed —
unrelated to the bench harness, confirmed present with this plan's changes
stashed):
- `test/bench` `TestBenchToolsManifestMatchesRegistry`: live MCP registry reports
  53 tools vs the manifest's expected 47 (tool-registry growth in other phases).
- `test/bench` `TestToolDescriptionsGoldenFile`: golden file drift.

## TDD Gate Compliance

RED gate (`test(78-02): ...` commit `755b3589`) preceded GREEN gate
(`feat(78-02): ...` commit `34c928ca`). No unexpected RED-phase pass. The Task 2
seed git mv + cutover is a migration/config task (TDD-exempt per the sequential
instructions); its behavior is guarded by the pre-existing bench/runtime +
helix-bench tests and the `make bench-quick` gate.

## Self-Check: PASSED

All modified files present on disk; the migrated seed exists at
`internal-toolbench/go/IT-go-patch-apply-1`; commits `755b3589`, `34c928ca`,
`4f4df69a` present in git log.
