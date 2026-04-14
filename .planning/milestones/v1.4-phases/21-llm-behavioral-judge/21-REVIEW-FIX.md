---
phase: 21
fixed_at: 2026-04-12T13:00:00Z
review_path: .planning/phases/21-llm-behavioral-judge/21-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Phase 21: Code Review Fix Report

**Fixed at:** 2026-04-12T13:00:00Z
**Source review:** .planning/phases/21-llm-behavioral-judge/21-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 8
- Fixed: 8
- Skipped: 0

## Fixes Applied

### HI-01: Verdict threshold gap between spec and implementation

- **Files:** `test/oracle/judge/rubric.go`, `test/oracle/judge/scorer.go`
- **Change:** Changed `ComputeVerdict` signature to return `error`. Added `ValidateScoreValues` call at function entry, returning error for invalid scores. Updated the single caller in `ScoreTranscript` to handle the error return.
- **Commit:** 95f6b2ec

### MD-01: selfJudged flag computed from env vars at scoring time

- **Files:** `test/oracle/judge/scorer.go`, `test/oracle/judge/judge_test.go`
- **Change:** Added explicit `selfJudged bool` parameter to `ScoreTranscript` instead of re-deriving from `llm.JudgeModel()` inside the function. Removed the `llm.JudgeModel()` call from scorer. Updated call site in `judge_test.go`.
- **Commit:** 7004bf3e

### MD-02: Outer slice append from within t.Run subtests is race-prone

- **Files:** `test/oracle/judge/judge_test.go`
- **Change:** Moved scoring logic out of `t.Run` subtests into the outer loop. Scores are now collected directly in the for-loop body, eliminating the race risk if `t.Parallel()` is ever added. Removed the unnecessary `tr := tr` capture (Go 1.25).
- **Commit:** 7004bf3e (combined with MD-01)

### MD-03: ParseScoreJSON uses naive brace matching

- **Files:** `test/oracle/judge/scorer.go`
- **Change:** Added markdown fence stripping before brace matching. If the response starts with triple backticks, the opening fence line and closing fence are removed before extracting JSON.
- **Commit:** 231d48f0

### LO-01: Unnecessary loop variable captures (Go 1.22+)

- **Files:** `test/oracle/llm/selection_test.go`, `test/oracle/llm/disambiguation_test.go`, `test/oracle/llm/interpretation_test.go`
- **Change:** Removed `tool := tool`, `target := target`, and `gc := gc` loop variable captures. Unnecessary since Go 1.22+ (project uses Go 1.25.1).
- **Commit:** e90c6d89

### LO-02: callCount increment position misleading

- **Files:** `test/oracle/llm/interpretation_test.go`
- **Change:** Renamed `callCount` to `subtestCount` throughout the function and updated the log message from "API calls" to "subtests" to accurately reflect what is being counted.
- **Commit:** e90c6d89 (combined with LO-01)

### IN-01: Unused stopReason blank assignments

- **Files:** `test/oracle/llm/interpretation_test.go`
- **Change:** Removed both `_ = stopReason` lines. The value is already captured in the `Transcript` struct's `StopReason` field, making the blank assignment pure noise.
- **Commit:** e90c6d89 (combined with LO-01)

### IN-02: TestJudgeInline is a no-op placeholder

- **Files:** `test/oracle/judge/judge_test.go`
- **Change:** Changed `t.Log(...)` to `t.Skip(...)` for the inline path message, preventing test count inflation for unimplemented functionality.
- **Commit:** e90c6d89 (combined with LO-01)

## Verification

- go vet (`-tags="llm" ./test/oracle/llm/...`): pass
- go vet (`-tags="llmjudge" ./test/oracle/judge/...`): pass
- Compilation: pass

---

_Fixed: 2026-04-12T13:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
