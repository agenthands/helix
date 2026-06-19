---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
plan: 03
subsystem: bench-runtime
tags: [bench, ablation, fairness, baseline_rag, ablation_status, tdd]
requires:
  - "Plan 80-01: 5 ablation MODE.md (incl. baseline_rag stub, your_agent_no_semantic) so resolution + mode names exist"
  - "Plan 80-02: ResultInput.AblationStatus + resultDoc.AblationStatus (json ablation_status,omitempty)"
provides:
  - "RunCell D-04 fairness startup gate (DefaultContract.Validate() fatal-on-non-nil after profile resolution)"
  - "RunCell D-02 baseline_rag fail-close (Deferred/DeferredReason, no result.v2.json, no daemon)"
  - "RunCell D-03 ablation_status wiring via ablationStatusFor(mode) into BuildResult"
  - "CellOutcome.Deferred distinct third matrix outcome (neither Success nor infra Err)"
affects:
  - "Plan 80-04 (CI contract test — separate always-on fairness guarantee)"
  - "Plan 80-05 (multi-mode smoke asserts registered+deferred baseline_rag, no_semantic marker)"
  - "Phase 82 aggregator (reads ablation_status to tell a partial no_semantic row from a clean one)"
tech-stack:
  added: []
  patterns:
    - "Pure compile-time fairness predicate gated at runner startup (no IO, CI-cheap)"
    - "Fail-close by mode-name comparison (keeps the two-key MODE.md resolver change-free)"
    - "Distinct third outcome flag (Deferred) so a stub is neither Success nor Err"
key-files:
  created: []
  modified:
    - "bench/runtime/cell.go"
    - "bench/runtime/cell_test.go"
    - "bench/runtime/matrix.go"
decisions:
  - "Fairness gate called UNCONDITIONALLY right after profile resolution (Open Q1 scope A) — pure + CI-cheap; committed DefaultContract has no overrides so it never fatals in CI"
  - "baseline_rag detected by mode-name comparison in RunCell (not a frontmatter marker) so the Plan 01 two-key resolver stays change-free"
  - "Deferred is a distinct CellOutcome flag (not overloaded onto Success/Err): a deferred stub has Success==false (ResultValid false) AND Err==nil (registered stub, not infra failure)"
  - "ablationStatusFor is a tiny pure helper (mode→marker) rather than inline, so the no_semantic-only rule is unit-testable in isolation"
metrics:
  duration: ~9min
  tasks: 2
  files_modified: 3
  completed: 2026-06-19
---

# Phase 80 Plan 03: Cell-Level Ablation Behaviors (Fairness Gate + baseline_rag Fail-Close + ablation_status) Summary

Wired the three Phase 80 cell-level behaviors into `RunCell` — the uniform per-cell spine all six ablation modes flow through — as pure growth on the existing path (no fork, D-01): a D-04 fairness startup gate (`runners.DefaultContract.Validate()` fatal-on-non-nil right after profile resolution), a D-02 `baseline_rag` fail-close (short-circuit before any sandbox/daemon, no result row, distinct `Deferred` matrix outcome), and a D-03 `ablation_status` marker set only on the `your_agent_no_semantic` row via a small `ablationStatusFor(mode)` helper.

## What Was Built

### Task 1 — Fairness startup gate + ablation_status wiring (TDD RED→GREEN)
- **Fairness gate (D-04):** `RunCell` now calls `runners.DefaultContract.Validate()` unconditionally immediately after profile resolution and BEFORE `benchsandbox.New`; a non-nil return is fatal (`fmt.Errorf("bench/runtime: fairness contract invalid: %w", verr)`) so an unfair benchmark (an override deviating from the shared budget without a `WaiverReason`) never spawns a daemon. The committed contract has no overrides, so the gate never fatals in CI under the current contract.
- **ablation_status (D-03):** added `ablationStatusFor(mode string) string` returning `"guarantee_pending_phase_81"` iff `mode == "your_agent_no_semantic"`, else `""`. Wired `AblationStatus: ablationStatusFor(cfg.Mode)` into the existing `BuildResult(ResultInput{...})` call. Honest modes leave it empty → `omitempty` omits the key; the no_semantic arm carries the marker.
- **Tests:** `TestFairnessGate` (committed contract validates; an empty-WaiverReason override fails the predicate, mirroring `runners.TestEmptyWaiverReasonFatal`), `TestAblationStatus` (helper returns the marker only for no_semantic + BuildResult round-trip proves key presence/omission).

### Task 2 — baseline_rag fail-close + distinct matrix outcome (TDD RED→GREEN)
- **CellResult:** added `Deferred bool` + `DeferredReason string`.
- **RunCell fail-close (D-02):** after the fairness gate and BEFORE `benchsandbox.New`, `if cfg.Mode == "baseline_rag"` sets `res.Deferred = true`, `res.DeferredReason = "baseline_rag: real RAG arm deferred to Phase 83 (ABLATE-04)"`, and `return res, nil` — no sandbox, no daemon, no BuildResult, no writeDurable. Path-segment validation and `ResultPath` layout still run first. Detected by mode name (not a frontmatter marker) to keep the two-key resolver change-free.
- **CellOutcome / matrix (D-02):** added `CellOutcome.Deferred`; `runOneCell` sets `oc.Deferred = res.Deferred` while keeping `oc.Success = err == nil && res.VerifyExitCode == 0 && res.ResultValid`. A deferred stub has `ResultValid==false` → not Success, `Err==nil` → not an infra error, `Deferred==true` → the distinct third outcome.
- **Tests:** `TestBaselineRagFailClose` (nil error, `Deferred==true`, reason names Phase 83, `os.Stat(ResultPath)` is `os.ErrNotExist`), `TestDeferredOutcomeClassification` (dispatch with an injected deferred run → `Success==false` AND `Deferred==true`, `Succeeded==0`).

## Deviations from Plan

None - plan executed exactly as written.

## TDD Gate Compliance

Each task followed RED→GREEN with separate commits:
- Task 1: `test(80-03)` 2054e47d (RED, compile-fail on undefined `ablationStatusFor`) → `feat(80-03)` 9358280d (GREEN).
- Task 2: `test(80-03)` 4a7d385a (RED, compile-fail on undefined `Deferred`/`DeferredReason`) → `feat(80-03)` 6109bf8d (GREEN).
No REFACTOR commits — the GREEN implementations were already minimal and clean.

## Verification

- `go build ./cmd/helix` — OK
- `go vet ./...` — OK
- `go test ./bench/...` — all packages OK (incl. `bench/runtime`, `bench/runners`)
- Targeted: `go test ./bench/runtime/ -run 'TestFairnessGate|TestAblationStatus|TestBaselineRag|TestDeferred|TestDispatch' -count=1` — exits 0
- `go test ./bench/runtime/ -count=1` — green (no regression in existing cell/matrix tests)

## Known Stubs

`baseline_rag` is an intentional, documented fail-closed stub: it emits no result row this phase by design (the real RAG arm — chromem-go embedding index + `cmd/helix-bench-rag` — is deferred to **Phase 83 / ABLATE-04**, recorded in `bench/runners/baseline_rag/MODE.md` and the `DeferredReason`). The `Deferred` outcome makes this explicit and machine-checkable rather than a silent gap. This is not a stub that blocks the plan's goal — the plan's goal IS to make baseline_rag a registered-but-deferred stub.

The `your_agent_no_semantic` row is marked `ablation_status: guarantee_pending_phase_81` — also intentional and documented: the kernel `disable_semantic_subsystem` guarantee (ABLATE-06) lands in Phase 81, so this phase's no_semantic arm is tool-filter-only and its row is honestly flagged partial.

## Self-Check: PASSED
