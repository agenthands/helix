---
phase: 89-reports-ci-policy-contamination-canary
plan: 02
subsystem: bench/aggregator + bench/runtime
tags: [bench, aggregator, canary, contamination, exclusion, runtime]
requires:
  - bench/canary/canary.go Sentinel / InjectPrompt / IsContaminated / DocKeyCompletion (Phase 86 probe)
  - bench/aggregator/aggregate.go rowCanary (per-row contamination flag) / reduceCanaryRate / reduceLeaderRow / reduceCostRow / successCount
  - bench/aggregator/report.go renderLeaderboard / renderFooter / Report struct
  - bench/runtime/matrix.go runOneCell claude prompt-assembly branch (meta.Prompt)
provides:
  - cleanRows(rows) clean/contaminated split — the INFRA-05 headline-exclusion primitive
  - reduceLeaderRow / reduceCostRow filter each cell's rows through cleanRows before counting
  - Report.Contaminated []ContaminatedCell (sorted mode,task) + contaminatedCells collector
  - appendContaminationFootnote (renders excluded (task,mode) cells before the footer)
  - bench/runtime/prompt_inject.go InjectCanaryIfSelected + canarySelected (every-Kth K=7 FNV-1a)
  - production wiring of the canary injector into runOneCell (keyed on stable c.Task)
affects:
  - 89-03 (byte-reproducibility of all 4 reports incl. the footnote; lockstep-guard intent update for TestAggregateCanaryAbsentIsEmDash)
tech-stack:
  added: []
  patterns: [fail-safe exclusion, single-source-sentinel, pure deterministic selector, sort-before-emit, em-dash null discipline]
key-files:
  created:
    - bench/runtime/prompt_inject.go
    - bench/runtime/prompt_inject_test.go
  modified:
    - bench/aggregator/aggregate.go
    - bench/aggregator/report.go
    - bench/aggregator/aggregate_canary_test.go
    - bench/runtime/matrix.go
decisions:
  - "Headline exclusion is enforced at the per-cell row level inside reduceLeaderRow/reduceCostRow via cleanRows, not by post-filtering the reduced vector — a contaminated row never reaches successCount or any metric/cost vector (fail-safe)."
  - "reduceCanaryRate is deliberately NOT routed through cleanRows: it MEASURES contamination over all rows; only the headline EXCLUDES. cleanRows and reduceCanaryRate are the two halves of the same flag (rowCanary)."
  - "The contamination footnote is inserted BEFORE the provenance footer divider (\\n---\\n) so the audit trail rides inside the leaderboard body; an empty contaminated set renders NO footnote (em-dash discipline)."
  - "The production injector keys selection on the stable task id (c.Task) via an FNV-1a bucket mod K=7 — pure, no clock/RNG/env — so the matrix stays reproducible across machines; the sentinel is embedded ONLY via canary.InjectPrompt (never re-derived)."
  - "leaderboard.golden.md is byte-UNCHANGED: the canonical golden fixture (writeCostedRow) carries no completion keys, so nothing is contaminated and no footnote renders. The plan's Pitfall-1 golden regen was a no-op here precisely because the em-dash discipline holds."
metrics:
  duration: ~25m
  completed: 2026-06-21
---

# Phase 89 Plan 02: Contamination Canary — Aggregate-Time Exclusion + Footnote + Production Injector Summary

INFRA-05 closed: a model echoing the canary sentinel is now flagged at score time, EXCLUDED from the leaderboard headline (pass@1 / verified_correctness / cost_per_solved) via a `cleanRows` split, listed in a deterministic `leaderboard.md` contamination footnote, and — for the first time — actually has a sentinel to echo, because a pure deterministic production injector (`InjectCanaryIfSelected`, every Kth task) now embeds `canary.Sentinel` into select prompts in `runOneCell`.

## What Was Built

### Task 1 — Aggregate-time exclusion (cleanRows) + leaderboard footnote
- `cleanRows(rows) (clean, contaminated []Row)` in `aggregate.go`: partitions a cell's rows by `rowCanary`'s verdict. A row that is BOTH present AND contaminated goes to `contaminated`; every other row (clean, OR no-completion pre-canary artifact) stays clean (Pitfall 4 null discipline — a missing completion is never treated as contaminated).
- `reduceLeaderRow` and `reduceCostRow` now filter each task's rows through `cleanRows` BEFORE any counting, so `successCount` and every metric/cost vector see CLEAN rows only. A fully-contaminated cell contributes nothing to the headline (fail-safe — T-89-02-01).
- `reduceCanaryRate` is UNCHANGED: it still iterates ALL rows (the MEASUREMENT keeps counting contamination; only the headline EXCLUDES).
- `Report.Contaminated []ContaminatedCell` + `contaminatedCells(...)`: collects the excluded `(task,mode)` cells, sorted `(mode, task)` for determinism (T-89-02-02 audit trail).
- `appendContaminationFootnote` in `report.go`: renders the excluded cells as a `## Contamination canary exclusions (INFRA-05)` section inserted before the provenance footer. Empty set → no footnote.

### Task 2 — Deterministic production InjectPrompt caller
- `bench/runtime/prompt_inject.go`: `canarySelected(taskKey)` = FNV-1a(taskKey) mod `canaryInjectEveryK` (K=7) == 0 — a pure, machine-stable selector. `InjectCanaryIfSelected(taskKey, prompt)` returns `canary.InjectPrompt(prompt)` for selected tasks (sentinel appended, original preserved verbatim) and the prompt UNCHANGED otherwise.
- Sentinel referenced ONLY via `canary.InjectPrompt` — never re-derived (T-89-02-03 single-source).
- Wired into `runOneCell`'s claude branch, keyed on the stable `c.Task`, so a memorising model echoes the sentinel into its completion where `rowCanary`/`cleanRows` already flag and exclude it.

## Synthetic-contamination test outcome
`TestCanaryExclusionFromHeadline` (hermetic): a fixture with one clean task (2/2 solved+verified) and one fully-contaminated task (2/2 solved+verified but echoing the sentinel) proves the contaminated task's pass/verified contribution DISAPPEARS from the headline, `CanaryPassRate` still reports 0.5 (2 clean / 4 with-completion), `Report.Contaminated` lists exactly `(task-dirty, full)`, and the rendered footnote names it. `TestCleanRowsSplit` unit-tests the partition primitive; `TestInjectCanary*` prove the injector is selective, verbatim-on-non-select, and deterministic.

## leaderboard.golden.md diff
NONE. The canonical golden fixture (`writeCostedRow`/`goldenFixture`) carries no `completion` doc keys, so no row is contaminated, no footnote renders, and the golden is byte-identical. Confirmed via `go test -run TestSwebenchColumnsGoldenStable -update` producing zero `git diff` in `testdata/`. This is the em-dash discipline working as designed, not a missed regen.

## Note for 89-03
`TestAggregateCanaryAbsentIsEmDash` remains green (clean fixture → no footnote). 89-03 owns the FINAL byte-reproducibility assertion of all 4 reports (including the footnote when contamination IS present) and the lockstep-guard intent update — this plan kept all existing goldens/tests green and added the exclusion + injector substrate they build on.

## Deviations from Plan
None of substance. The plan anticipated regenerating `leaderboard.golden.md` (Pitfall 1); the regen was a verified no-op because the canonical golden fixture is uncontaminated. The footnote-insertion point was placed before the footer divider (inside the body) rather than after the overlap block textually, which is the same structural position and keeps the footer last.

## Threat Register Reconciliation (INFRA-05)
- T-89-02-01 (contaminated rows inflating the headline) — MITIGATED: `cleanRows` filters every headline reduce; a fully-contaminated cell contributes nothing (fail-safe).
- T-89-02-02 (contaminated cells disappearing without a trace) — MITIGATED: excluded cells collected on `Report.Contaminated` and rendered in the deterministic footnote; `CanaryPassRate` still measures them.
- T-89-02-03 (injector/detector Sentinel divergence) — MITIGATED: the production caller embeds via `canary.InjectPrompt` and the detector reads via `canary.IsContaminated`; the literal is never re-derived.
- T-89-02-SC (npm/pip/cargo installs) — ACCEPTED/N-A: zero new deps; no go.mod change (FNV is stdlib).

## Verification
- `go build ./...` — clean
- `go vet ./bench/...` — clean
- `make vet` (all custom vettools) — clean
- `go test ./bench/aggregator/... ./bench/runtime/... ./bench/canary/...` — pass
- `HELIX_BIN=$(pwd)/helix go test ./bench/runtime/...` — pass (34s, real-binary, no false-green)

## Self-Check: PASSED
- Created files exist: bench/runtime/prompt_inject.go, bench/runtime/prompt_inject_test.go, 89-02-SUMMARY.md
- Commits exist: 1a428b8e (RED), b3c4b025 (GREEN T1), 816771cc (RED T2), bc329e76 (GREEN T2)
