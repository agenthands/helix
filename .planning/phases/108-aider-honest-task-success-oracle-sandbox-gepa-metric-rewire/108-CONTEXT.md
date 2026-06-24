# Phase 108: Aider Honest Task-Success Oracle + Sandbox + GEPA Metric Rewire + Sequestered Split - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss); executed inline with uv per user.

<domain>
## Phase Boundary

Replace the gameable `choice_rate` reward with honest Aider-polyglot task-success — graded in a per-task sandbox with anti-tamper test restore — wired as the GEPA metric over a sequestered held-out TEST split gated by `val_size > 50`.

Requirements: ORACLE-01 (Aider hidden-test grading in a per-task sandbox; 0-tests-ran = hard ERROR; gold tests restored from an agent-unwritable path), TUNE-02 (GEPA metric rewired choice_rate→task-success; choice_rate at most a non-optimized diagnostic), TUNE-03 (sequestered held-out TEST split never passed to compile(); `val_size > 50` hard adoption-precondition gate).
</domain>

<decisions>
## Implementation Decisions / Locked constraints

- **Native test commands (parity-pinned to Go `nativeTestCommand`, loader.go:299-323):** python→`pytest`; rust→`cargo test -- --include-ignored` (the `--include-ignored` is LOAD-BEARING — Exercism marks acceptance tests `#[ignore]`, so a do-nothing stub that merely compiles passes plain `cargo test` → vacuous; this is the "do-nothing agent fails Rust" anti-vacuity hook); go→`go test ./...`; java→`./gradlew test`; javascript→`./npm-test.sh`; cpp→`./cpp-test.sh`. Unknown language fail-closes. Mirror in Python, pin via a shared `golden/aider_native_cmd.json` asserted by BOTH a Python test and a Go `*_test.go` under `bench/datasets/aider-polyglot/`.
- **Honest oracle:** 0 tests executed = hard ERROR (no vacuous pass — the Phase 81 false-green class). Gold test files restored from an agent-unwritable source copy before grading (agent test-tampering cannot force green).
- **Metric rewire:** `taskmetric.py` runs the Phase-107 agent in a sandbox, grades, returns `dspy.Prediction(score, feedback)`. `optimize.py` swaps `score_choice_rate` → `task_success`. `choice_rate`/`scorer.py` retained only as a non-optimized diagnostic pre-screen.
- **Corpus reality:** the current choice_rate corpus is 8 train + 3 test = 11. `val_size > 50` is therefore a STRICT gate that NO-SHIPS at the current size; anti-vacuity pins `val_size=50` → no-ship and `51` → adoptable. Growing the task-success corpus to >50 is future work (TUNE-FUT-01); this phase ships the GATE, not the corpus.
- **Boundary (ADOPT-04):** zero new Go deps; grader + metric stay Python under `tools/dspy-tune/`; `make vet` (`vet-tools-quarantine`) green; `git diff go.mod` empty. The only Go addition is a stdlib `*_test.go` parity assertion under `bench/datasets/aider-polyglot/`.
- **uv everywhere** for Python (per CLAUDE.md). Hermetic tests use fakes (no real cargo/pytest/network).
- **Anti-vacuity:** every gate ships a break-the-invariant → assert-RED test; fold review before verify.
</decisions>

<code_context>
## Existing Code Insights

`bench/datasets/aider-polyglot/loader.go` (`nativeTestCommand`, `NativeTestCommand`), `tools/dspy-tune/{optimize.py (choice_rate metric), scorer.py, test_split.py (disjoint-split guard), data/{train,test}.jsonl, golden/parity_cases.json}`, and the Phase-107 `tools/dspy-tune/agent/` package (ReActAgent the metric drives).
</code_context>

<specifics>
## Specific Ideas
Per the v2.3 research SUMMARY Phase-108 section and the locked decisions above.
</specifics>

<deferred>
## Deferred Ideas
Growing the task-success corpus to `val_size > 50` (TUNE-FUT-01); the live GEPA run; SWE-bench (Phase 109).
</deferred>
