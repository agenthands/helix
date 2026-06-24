---
phase: 108
slug: aider-honest-task-success-oracle-sandbox-gepa-metric-rewire
status: passed
verified: 2026-06-24
method: inline (executed + verified by orchestrator with uv; gsd-verifier unavailable mid-milestone)
---

# Phase 108 — Verification Report

**Verdict: PASSED** — goal-backward verification against code reality.

| # | Truth | Verdict | Evidence |
|---|-------|---------|----------|
| 1 | Aider task-success graded by native hidden tests in a per-task sandbox; parity-pinned to loader.go; **0 tests = hard ERROR**; gold tests restored from an agent-unwritable path (ORACLE-01) | ✅ | `grade_aider.py` + `sandbox.py`; `test_grade.py` (10 tests) incl. `test_zero_tests_is_hard_error_*`, `test_gold_test_restore_defeats_tampering`; Go `native_cmd_parity_test.go` green |
| 2 | GEPA metric rewired choice_rate → task-success; choice_rate at most a non-optimized diagnostic (TUNE-02) | ✅ | `taskmetric.py` is the reward; `optimize.py` demotes choice_rate to a printed diagnostic, never `compile()`'s metric |
| 3 | Sequestered held-out TEST never passed to compile(); `val_size>50` hard adoption gate (TUNE-03) | ✅ | `test_split.py` disjoint + `adoption_allowed`; `optimize.py` enforces the gate before compiling |
| 4 | Anti-vacuity break-the-invariant → RED | ✅ | mutation-confirmed: `>`→`>=` flips the val_size test RED; do-nothing Rust fails; 0-tests→GradeError; tamper-restore |
| 5 | Boundary (ADOPT-04): zero new Go deps; grader+metric Python under tools/; make vet green | ✅ | `go.mod` empty diff; no `.go` under `tools/`; `make vet` (incl. `vet-tools-quarantine`) green; no `shell=True` |

## Gates
- `uv run pytest` → **25 passed**; `uv run python test_grade.py`/`test_split.py` → green.
- `go test ./bench/datasets/aider-polyglot/` parity → green.
- `make vet` green; `git diff go.mod` empty.

## Requirements coverage
ORACLE-01 ✅, TUNE-02 ✅, TUNE-03 ✅ (3/3).

## Human verification
None for the hermetic surface. The live task-success GEPA run (needs an Aider corpus sized val_size>50 + an LM key) is deferred to a dev-time run / TUNE-FUT-01; the harness correctly no-ships today.
