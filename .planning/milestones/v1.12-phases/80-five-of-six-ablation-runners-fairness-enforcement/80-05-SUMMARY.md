---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
plan: 05
subsystem: testing
tags: [bench, ablation, deltas, matrix, fairness, result.v2, scripted-agent]

# Dependency graph
requires:
  - phase: 80-02
    provides: ablation_status additive open provenance field + additive-property pattern
  - phase: 80-03
    provides: RunCell fairness gate, baseline_rag fail-close (CellResult.Deferred / CellOutcome.Deferred), ablation_status wiring, durable <task>/<mode>/<run_index>/ row layout
  - phase: 79
    provides: evaluators.Metrics nullable record + result.v2 metrics object
provides:
  - "bench/runtime.ComputeAndWriteDeltas: single-run post-RunMatrix 3-delta pass (D-05)"
  - "ablation_deltas open top-level property surfaced in each of the 4 real-mode result.v2 rows"
  - "runBench post-matrix delta-pass invocation + deferred-cell operator logging"
  - "five-of-six scripted smoke proving 4 real + 1 partial + 0 baseline_rag rows end-to-end"
affects: [phase-82-aggregator, phase-83-baseline-rag, phase-89-reports]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Matrix-tier post-RunMatrix pass: cross-mode computation runs after wg.Wait() because the cell tier sees only one mode (Pitfall 3)"
    - "Verbatim row write-back: decode row as map[string]json.RawMessage, add one open property, re-Validate, atomic writeDurable (preserves all existing fields, no struct drift)"

key-files:
  created:
    - bench/runtime/deltas.go
    - bench/runtime/deltas_test.go
    - bench/runtime/five_of_six_test.go
  modified:
    - cmd/helix-bench/main.go
    - cmd/helix-bench/run_cmd_test.go

key-decisions:
  - "ablation_deltas is an open top-level property: comparison_name -> metric -> float64 delta; all 4 real-mode rows carry the SAME deltas object (criterion #4 'surface in the per-mode rows' read literally)"
  - "Comparable metric set kept small + fixed: tokens_input, tokens_output, tool_calls, files_modified, edit_locality; a metric null in either operand is skipped (no fabricated 0)"
  - "Delta operands use the RESOLVABLE mode names no_lsp / no_structured_edit (MODE.md + resolver), NOT the prior-wave prose aliases your_agent_no_lsp / your_agent_no_structured_edit"
  - "Comparison keys: full_minus_baseline_plain / full_minus_no_lsp / full_minus_no_structured_edit (stable greppable snake_case for the Phase 82 aggregator)"
  - "Write-back preserves the full row via map[string]json.RawMessage decode, not the typed resultDoc, so additive provenance keys survive round-trip"

patterns-established:
  - "Pattern 1: matrix-tier delta pass strictly after RunMatrix returns (Pitfall 3) — runBench invokes runtime.ComputeAndWriteDeltas over summary.Outcomes"
  - "Pattern 2: incomplete-task skip — a task missing any of the 4 real modes is reported in DeltaReport.Skipped, never computed against a nil baseline"

requirements-completed: [ABLATE-01]

# Metrics
duration: ~8min
completed: 2026-06-19
---

# Phase 80 Plan 05: Five-of-Six Ablation Deltas + Smoke Summary

**Single-run 3-delta pass (full vs baseline_plain/no_lsp/no_structured_edit) surfaced in each per-mode result.v2 row under an open ablation_deltas property, wired post-RunMatrix into runBench, and proven by a six-mode scripted smoke asserting 4 real + 1 partial + 0 baseline_rag rows.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-06-19T09:57:34Z
- **Completed:** 2026-06-19T10:04:26Z
- **Tasks:** 2 (TDD RED→GREEN each)
- **Files modified:** 5 (3 created, 2 modified)

## Accomplishments
- `ComputeAndWriteDeltas` (bench/runtime/deltas.go): groups matrix outcomes by task, computes exactly 3 fixed deltas over a small comparable-metric set, surfaces them in each of the 4 real-mode rows under `ablation_deltas`, re-validates, writes atomically; skips + reports tasks missing any real mode.
- `runBench` now invokes the delta pass strictly after the matrix barrier and logs deferred cells (baseline_rag) distinctly without counting them as failures.
- Five-of-six scripted smoke (`TestFiveOfSixSmoke`) drives all 6 modes via RunMatrix end-to-end and asserts the full contract: 4 real rows (schema-valid + 3 deltas), 1 partial no_semantic row (`ablation_status: guarantee_pending_phase_81`, no deltas), baseline_rag Deferred with zero rows.
- Scope held to single-run / 3-fixed-deltas — no BCa/pass@k/variance/leaderboard (Phase 82).

## Task Commits

1. **Task 1 RED: failing delta tests** - `3544157e` (test)
2. **Task 1 GREEN: 3-delta compute + write-back helper** - `86e484ba` (feat)
3. **Task 1 fix: resolvable mode names** - `55cc38ee` (fix — see Deviations)
4. **Task 2 RED: five-of-six smoke + runBench wiring test** - `05db8792` (test)
5. **Task 2 GREEN: wire delta pass into runBench** - `9c0c327d` (feat)

## Files Created/Modified
- `bench/runtime/deltas.go` - The single-run 3-delta compute + per-mode-row write-back helper (D-05).
- `bench/runtime/deltas_test.go` - Arithmetic, schema-valid write-back, and incomplete-task-skip tests.
- `bench/runtime/five_of_six_test.go` - Hermetic six-mode RunMatrix + delta-pass smoke (skips without a helix binary).
- `cmd/helix-bench/main.go` - Post-matrix `ComputeAndWriteDeltas` invocation + deferred-cell logging in `runBench`.
- `cmd/helix-bench/run_cmd_test.go` - `TestRunSubcommandWiresDeltaPass` asserting the 4 real rows carry `ablation_deltas` after a multi-mode run.

## Decisions Made
- The 3 deltas surface in EACH of the 4 real-mode rows (same deltas object), satisfying criterion #4 ("surface in the per-mode result rows") literally rather than only in a sibling file.
- Write-back decodes the row as `map[string]json.RawMessage` (not the typed `resultDoc`) so every existing additive provenance key survives the round-trip; only `ablation_deltas` is added.
- Comparable metric set is fixed at 5 run-cost/behavior metrics; null-in-either-operand metrics are dropped for that comparison.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Delta operands used non-resolvable mode-name aliases**
- **Found during:** Task 2 (wiring + smoke)
- **Issue:** Task 1's helper + tests used `your_agent_no_lsp` / `your_agent_no_structured_edit` (taken from the plan's `prior_wave_context` prose). The actual resolvable mode names — defined in `bench/runners/<mode>/MODE.md` and asserted in `mode_resolver_test.go` — are `no_lsp` / `no_structured_edit`. The plan body itself (tasks, lines 47-48/122) uses the short names. With the alias names the delta pass would never match any real row, silently skipping every task.
- **Fix:** Renamed the `modeNoLSP` / `modeNoStructuredEdit` constants and the test fixtures to the resolvable `no_lsp` / `no_structured_edit`. Comparison keys (`full_minus_no_lsp`, `full_minus_no_structured_edit`) are unchanged.
- **Files modified:** bench/runtime/deltas.go, bench/runtime/deltas_test.go
- **Verification:** `TestAblationDeltas*` green; the five-of-six smoke computes deltas for the one real task (report.Computed == [task], report.Skipped empty).
- **Committed in:** `55cc38ee`

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** The fix was required for the delta pass to function at all (operands must match resolvable mode names). No scope creep — single-run/3-delta scope held; no aggregator logic added.

## Issues Encountered
- The cmd-layer delta-wiring RED test and the runtime smoke both require a real helix binary; both `t.Skip` cleanly when `HELIX_BIN` is unset and no `helix` is on PATH. The full gate was run with `HELIX_BIN=$PWD/helix` so the daemon-spawning paths actually executed (not skipped).

## Threat Flags

None — the delta pass reads/writes only harness-produced durable rows under the run-scoped OutDir (T-80-02 mitigated: only the 4 real modes are operands; the no_semantic partial and the no-row baseline_rag are excluded by construction; each row is re-`Validate`d after write-back and written atomically via `writeDurable`). No new network/auth/file surface.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 80 is complete: 5 ablation MODE.md arms resolve, the fairness gate is wired + CI-tested (Plan 04), the five-of-six matrix is proven end-to-end, and the 3-delta substrate is surfaced in the rows for the Phase 82 aggregator to consume.
- Phase 81 owns the kernel `disable_semantic_subsystem` guarantee that flips the no_semantic arm from partial (`guarantee_pending_phase_81`) to a clean measurement.
- Phase 82 (multi-run aggregator: BCa/pass@k/variance/leaderboard) reads `ablation_deltas` by its stable comparison keys.

## Self-Check: PASSED

---
*Phase: 80-five-of-six-ablation-runners-fairness-enforcement*
*Completed: 2026-06-19*
