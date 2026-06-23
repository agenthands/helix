---
phase: 100-polyglot-edit-benchmark-committed-baseline
plan: 02
subsystem: testing
tags: [aider-polyglot, editbench, baseline-01, byte-reproducible, fail-closed, cell-branch, tdd]

# Dependency graph
requires:
  - phase: 100-01
    provides: "newDeterministicEditAgent / newEditAgentWithApply / newNativeTestFn / aiderEditMode const, EditFormatApplied *bool result.v2 key, exported LoadExercise/NativeTestCommand, aider_edit MODE.md"
provides:
  - "runAiderEditCell — the aider_edit mode-name cell branch driving RunExercise VERBATIM against the warm daemon"
  - "assembleAiderEditResult — pure deterministic-metrics-only result.v2 assembly (NULLs repo-derived patch_validator metrics)"
  - "aggregator.RenderAiderEditBaseline — pure sort-before-emit BENCH-RESULTS.md renderer keyed on result.v2.json"
  - "committed byte-reproducible baseline at bench/reports/aider-edit-baseline/{result.v2.json,BENCH-RESULTS.md}"
  - "bench-aider-edit Makefile target + aider_edit_baseline_regen.go local regenerate path"
affects: [aider-edit-cell, committed-baseline-report, editbench-04-future-expansion]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Mode-name cell branch (cfg.Mode == aiderEditMode) mirroring baselineRagMode — owns its own fixture-path resolution from cfg.SeedDir"
    - "Deterministic-metrics-only baseline: NULL repo-derived patch_validator metrics (files_modified/edit_locality/edit_distance_patch) + sorted metric_errors so the committed bytes are byte-reproducible"
    - "Fail-CLOSED committed-baseline read: present signal on os.ReadFile + present check on edit_format_applied — a missing file/key is a HARD failure, never a pass"

key-files:
  created:
    - bench/runtime/aider_edit_cell.go
    - bench/runtime/aider_edit_cell_test.go
    - bench/runtime/aider_edit_baseline_regen.go
    - bench/aggregator/aider_edit_baseline.go
    - bench/aggregator/aider_edit_baseline_test.go
    - bench/reports/aider-edit-baseline/result.v2.json
    - bench/reports/aider-edit-baseline/BENCH-RESULTS.md
  modified:
    - bench/runtime/cell.go
    - .gitignore
    - Makefile

key-decisions:
  - "NULL the repo-derived patch_validator metrics in the committed baseline: files_modified/edit_locality/edit_distance_patch are computed from a LIVE git working tree (RepoDir) and vary by machine/scratch-path, so they are non-reproducible (Pitfall 3) and excluded — recorded as sorted metric_errors, not fabricated values"
  - "runAiderEditCell drives the deterministic arm via assembleAiderEditResult (RepoDir-independent) so the live row and the committed baseline agree byte-for-byte"
  - "make bench-aider-edit regenerates via a //go:build ignore script (aider_edit_baseline_regen.go) — cmd/helix-bench run does NOT route the aider fixtures through the matrix (different on-disk layout), so the smallest entrypoint that drives runAiderEditCell is used; no new schema/matrix subsystem added"

patterns-established:
  - "Pattern: deterministic-metrics-only committed baseline — strip every repo/clock/path-derived field, sort all lists before emit, prove byte-reproducibility with a double-render diff-empty + golden test"
  - "Pattern: hermetic sibling (in-process apply seam) as sole authoritative proof + HELIX_BIN fail-closed live sentinel"

requirements-completed: [EDITBENCH-01, BASELINE-01]

# Metrics
duration: ~40min
completed: 2026-06-23
status: complete
---

# Phase 100 Plan 02: Live Cell + Committed Byte-Reproducible Baseline Summary

**`runAiderEditCell` (an `aider_edit` mode-name branch off `RunCell`) drives the Plan 01 deterministic EDIT-verb AgentFn + live native TestFn through `RunExercise` VERBATIM against the warm daemon, stamps `edit_format_applied`, and produces a committed, byte-reproducible polyglot-edit baseline (deterministic metrics only) — proven hermetically with no HELIX_BIN/network and fail-closed under HELIX_BIN.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 3
- **Files modified:** 10 (7 created, 3 modified)

## Accomplishments
- `runAiderEditCell` — the `aider_edit` cell branch (sibling of `runRAGCell`) that spawns the warm daemon, loads the vendored `go/wordy` fixture, drives `aiderpolyglot.RunExercise` VERBATIM (WR-01 intact) with `newDeterministicEditAgent` + `newNativeTestFn`, maps the attempt grade to a verify exit, reaps the daemon (graceful Stop + Kill), and writes a schema-valid `result.v2.json` carrying `edit_format_applied`.
- `cell.go` dispatch: `if cfg.Mode == aiderEditMode { return runAiderEditCell(...) }`, placed immediately after the `baselineRagMode` branch; detection is BY MODE NAME (the resolver is strict two-key; the MODE.md still validates as a side effect).
- `assembleAiderEditResult` — the pure, deterministic-metrics-only result.v2 assembly shared by the live cell and the hermetic sibling. It NULLs the repo-derived patch_validator metrics and sorts the metric_errors so the committed bytes are byte-reproducible.
- Committed baseline at the FIXED dir `bench/reports/aider-edit-baseline/` (`result.v2.json` + `BENCH-RESULTS.md`), force-tracked via a `.gitignore` negated allowlist (`!/bench/reports/aider-edit-baseline/**`), produced by the REAL pipeline and regenerable byte-identically via `make bench-aider-edit`.
- `aggregator.RenderAiderEditBaseline` — a pure, sort-before-emit BENCH-RESULTS.md renderer that fail-closes on a missing `edit_format_applied`.
- Full test cluster: hermetic sole-authoritative sibling, anti-vacuity (wrong reference → non-success), HELIX_BIN fail-closed live sentinel, committed-result fail-closed assertion, double-render byte-reproducibility golden, and a stripped/flipped anti-vacuity case.

## Task Commits

1. **Task 1: runAiderEditCell + RunCell mode-name branch (EDITBENCH-01 live half)** — `730f5b34` (feat, TDD test+impl)
2. **Task 2: capture + commit byte-reproducible baseline + .gitignore exception (BASELINE-01)** — `e8a21e28` (feat)
3. **Task 3: byte-reproducibility golden + committed-baseline assertion (BASELINE-01 anti-vacuity)** — `fff4dbf8` (test, TDD)

_TDD tasks (1, 3) followed RED → GREEN: Task 1's RED was a compile failure (undefined `assembleAiderEditResult`/`aiderEditResultInput`) verified before implementing; Task 3's tests grade the committed artifact produced in Task 2._

## Files Created/Modified
- `bench/runtime/aider_edit_cell.go` — `runAiderEditCell` (mode-name cell branch) + `assembleAiderEditResult` (deterministic result.v2 assembly).
- `bench/runtime/aider_edit_cell_test.go` — `TestAiderEditCellHermetic` (sole authoritative proof), `TestAiderEditCellAntiTamper` (revert-and-fail), `TestAiderEditCellLiveRan` (HELIX_BIN fail-closed sentinel).
- `bench/runtime/cell.go` — added the `aiderEditMode` dispatch branch after `baselineRagMode`.
- `bench/runtime/aider_edit_baseline_regen.go` — `//go:build ignore` local-only regenerator driving the real `runAiderEditCell` + renderer.
- `bench/aggregator/aider_edit_baseline.go` — `RenderAiderEditBaseline` (pure, sort-before-emit, fail-closed).
- `bench/aggregator/aider_edit_baseline_test.go` — committed-result + byte-reproducibility + anti-vacuity tests.
- `bench/reports/aider-edit-baseline/{result.v2.json,BENCH-RESULTS.md}` — the committed deterministic baseline.
- `.gitignore` — negated allowlist force-tracking the baseline dir.
- `Makefile` — `bench-aider-edit` target + `.PHONY` entry.

## Decisions Made
- **NULL the repo-derived patch_validator metrics in the committed baseline.** During capture, `edit_distance_patch`/`edit_locality`/`files_modified` came back varying (e.g. 18 vs 22; `edit_locality` as a float `0.999...` vs `1`) because `patch_validator` reads a LIVE git working tree (`RepoDir`). These are non-reproducible (Pitfall 3), so `assembleAiderEditResult` NULLs them explicitly and records the exclusion in sorted `metric_errors`. Only the truly deterministic quality metrics (`task_success`, `verified_correctness`, `edit_format_applied`, `outcome`) survive into the committed bytes. This was the load-bearing determinism fix.
- **Live cell and committed baseline share one assembly path.** `runAiderEditCell` builds its result through `assembleAiderEditResult` (RepoDir-independent), so the live `result.v2.json` and the committed baseline are byte-identical — the live sentinel and the golden test cannot drift apart.
- **`make bench-aider-edit` regenerates via a `//go:build ignore` script.** `cmd/helix-bench run` does not route the aider-polyglot fixtures through the matrix (the fixtures use a different on-disk layout than `cellSeedDir`), so per the plan the target drives the smallest entrypoint that exercises `runAiderEditCell` directly, with no new schema/matrix subsystem.

## Deviations from Plan

**1. [Rule 1 - Bug] Committed baseline carried non-reproducible repo-derived metrics**
- **Found during:** Task 2 (first regen run failed; values differed across captures).
- **Issue:** `coordinator.Grade` populated `files_modified`/`edit_locality`/`edit_distance_patch` from the live git working tree (via `RepoDir`), so the committed metrics were machine-/cwd-dependent and NOT byte-reproducible — directly violating BASELINE-01 + Pitfall 3.
- **Fix:** `assembleAiderEditResult` now NULLs those three metrics and appends sorted `metric_errors` annotations; the renderer restates only the reproducible quality metrics. Byte-identity confirmed across two `make bench-aider-edit` runs (zero git diff).
- **Files modified:** `bench/runtime/aider_edit_cell.go`, `bench/aggregator/aider_edit_baseline.go`.
- **Commit:** `e8a21e28`.

This is the only deviation; the plan explicitly anticipated this class (Pitfall 3 / "commit deterministic metrics ONLY"), so the fix realizes the plan's intent rather than departing from it.

## Issues Encountered
- The plan's `git check-ignore -v ... | grep '!'` acceptance grep returns empty ONCE the baseline files are tracked (git short-circuits ignore evaluation for tracked paths). The rule-level negation is proven with `git check-ignore -v --no-index`, which prints `.gitignore:276:!/bench/reports/aider-edit-baseline/**`, and `git ls-files` confirms both files are tracked — the underlying goal (files commit) is satisfied.

## User Setup Required
None — no external service configuration. `git diff go.mod` is empty (zero new deps). The committed baseline is hermetic (vendored go/wordy + native `go test`); `make bench-aider-edit` only needs the Go toolchain.

## Verification Evidence
- Hermetic suite (HELIX_BIN UNSET): `go test ./bench/runtime/... ./bench/aggregator/...` green.
- Live leg (HELIX_BIN SET): `TestAiderEditCellLiveRan` RAN (not skipped) and produced a schema-valid `result.v2.json` carrying `edit_format_applied: true` + `outcome: success`.
- `go vet ./...` clean; `git diff go.mod` empty.
- `make bench-aider-edit` regenerates the committed baseline byte-identically (zero git diff).
- `.gitignore` exception: both baseline files are `git ls-files`-tracked; `git check-ignore -v --no-index` prints the negated allowlist rule.

## Next Phase Readiness
- The full path (load → daemon → EDIT verb → native test → result.v2 → committed baseline) is now driven end-to-end and proven byte-reproducible.
- EDITBENCH-04 (full 6-track/225-task expansion) can reuse `runAiderEditCell` + `assembleAiderEditResult` unchanged; only the matrix routing of the aider fixtures into `RunCell` (or a multi-exercise regenerator) remains for the expansion.
- WR-01 stays intact: `RunExercise`/`restorePristineTests` were reused VERBATIM; the loader leaf is unchanged this plan (`git diff` on `loader.go` is empty).

## Self-Check: PASSED

- Created files verified present: `bench/runtime/aider_edit_cell.go`, `bench/runtime/aider_edit_cell_test.go`, `bench/runtime/aider_edit_baseline_regen.go`, `bench/aggregator/aider_edit_baseline.go`, `bench/aggregator/aider_edit_baseline_test.go`, `bench/reports/aider-edit-baseline/result.v2.json`, `bench/reports/aider-edit-baseline/BENCH-RESULTS.md`.
- Commits verified in git log: `730f5b34`, `e8a21e28`, `fff4dbf8`.
- Hermetic tests green with no HELIX_BIN/network: `TestAiderEditCellHermetic`, `TestAiderEditCellAntiTamper`, `TestAiderEditBaselineResultValid`, `TestAiderEditBaselineByteReproducible`, `TestAiderEditBaselineAntiVacuity`. Live sentinel `TestAiderEditCellLiveRan` RAN under HELIX_BIN. `go vet ./...` clean; `git diff go.mod` empty.

---
*Phase: 100-polyglot-edit-benchmark-committed-baseline*
*Completed: 2026-06-23*
