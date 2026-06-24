---
phase: 108-aider-honest-task-success-oracle-sandbox-gepa-metric-rewire
plan: 01
subsystem: tooling
tags: [dspy-tune, aider-polyglot, task-success, gepa, oracle, val-size-gate, uv]
requires:
  - phase: 107
    provides: tools/dspy-tune/agent/ ReActAgent the task-success metric drives
provides:
  - Honest Aider-polyglot task-success grader (grade_aider.py) with 0-tests=hard-ERROR + parity-pinned native cmds
  - Per-task sandbox + gold-test restore (sandbox.py) defeating in-sandbox test tampering
  - task-success GEPA metric (taskmetric.py) replacing choice_rate as the reward
  - val_size>50 strict adoption gate in optimize.py (the v2.2 no-ship root-cause fix)
  - Cross-language parity guard (golden/aider_native_cmd.json + Go native_cmd_parity_test.go)
affects: [Phase 109 (SWE-bench grader plugs into taskmetric), Phase 110 (adoption gate)]
tech-stack:
  added: []
  patterns: [parity-pinned Go<->Python golden, honest fail-not-skip oracle, strict adoption gate]
key-files:
  created:
    - tools/dspy-tune/grade_aider.py
    - tools/dspy-tune/sandbox.py
    - tools/dspy-tune/taskmetric.py
    - tools/dspy-tune/test_grade.py
    - tools/dspy-tune/golden/aider_native_cmd.json
    - bench/datasets/aider-polyglot/native_cmd_parity_test.go
  modified:
    - tools/dspy-tune/optimize.py
    - tools/dspy-tune/test_split.py
key-decisions:
  - "choice_rate demoted to a non-optimized diagnostic pre-screen; task-success is the GEPA reward (TUNE-02)"
  - "val_size>50 is a STRICT gate — no-ships at the current 11-task corpus; growing the corpus is TUNE-FUT-01"
  - "Native test cmds mirror loader.go via a shared golden, pinned by tests on BOTH sides"
patterns-established:
  - "Honest oracle: 0 tests executed => hard ERROR, never a vacuous pass (Phase-81 class)"
  - "Gold-test restore from an agent-unwritable source before grading"
requirements-completed: [ORACLE-01, TUNE-02, TUNE-03]
duration: ~30min
completed: 2026-06-24
status: complete
---

# Phase 108 — Summary

**Replaced the gameable `choice_rate` reward with an honest Aider-polyglot task-success oracle (parity-pinned native test commands, 0-tests=hard-ERROR, anti-tamper gold-test restore) wired as the GEPA metric behind a strict `val_size>50` adoption gate — the direct fix for the v2.2 no-ship root cause.**

## What was built
- **`grade_aider.py`** — honest grader; per-language native argv is a parity mirror of `loader.go nativeTestCommand` (shared `golden/aider_native_cmd.json`). **0 tests executed → `GradeError`** (no vacuous pass); Rust runs `cargo test -- --include-ignored` so a compile-only stub fails.
- **`sandbox.py`** — per-task sandbox; restores gold tests from an agent-unwritable source so in-sandbox tampering can't force green.
- **`taskmetric.py`** — `score_task` (run agent → grade → score); `make_gepa_metric` wraps it into `dspy.Prediction`. Replaces choice_rate as the reward.
- **`optimize.py`** — choice_rate demoted to a diagnostic pre-screen; task-success is the reward; **`val_size>50` strict gate** no-ships the current tiny corpus.
- **`native_cmd_parity_test.go`** — Go-side assertion that `loader.go` (authority) == the dspy-tune golden.

## Verification
- **25 Python tests green** via `uv run pytest` + script paths (`grade OK`, `split-disjoint OK`).
- **Go parity test green**; `make vet` (incl. `vet-tools-quarantine`) green; `go.mod` unchanged; no `.go` under `tools/`; no `shell=True`.
- **Anti-vacuity mutation-confirmed:** `val_size>` → `>=` flips the gate test RED; do-nothing Rust stub scores fail; 0-tests → `GradeError`; tamper-restore; TEST/TRAIN disjoint.
- `optimize.py` with the current corpus prints a no-ship (AIDER_TASKS_DIR unset / val_size ≤ 50) — the correct, honest outcome.

## Deviations
- Executed inline with `uv` (the gsd-executor/verifier agents are unavailable mid-milestone; per user choice).
- The live task-success GEPA run requires an Aider corpus at `AIDER_TASKS_DIR` sized `val_size>50` (TUNE-FUT-01) + an LM key — deferred; the GATE, grader, metric, and parity guard ship now and correctly no-ship today.
