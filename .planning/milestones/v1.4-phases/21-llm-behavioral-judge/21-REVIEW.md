---
phase: 21
status: findings
finding_count: 8
critical: 0
high: 1
medium: 3
low: 2
info: 2
reviewed_files: 10
files_reviewed_list:
  - test/oracle/judge/aggregate.go
  - test/oracle/judge/rubric.go
  - test/oracle/judge/scorer.go
  - test/oracle/llm/client.go
  - test/oracle/llm/prompt.go
  - test/oracle/llm/transcript.go
  - test/oracle/llm/selection_test.go
  - test/oracle/llm/disambiguation_test.go
  - test/oracle/llm/interpretation_test.go
  - test/oracle/judge/judge_test.go
---

# Phase 21: Code Review Report

**Reviewed:** 2026-04-12T12:00:00Z
**Depth:** standard
**Files Reviewed:** 10 (6 source, 4 test)
**Status:** findings

## Summary

Phase 21 introduces LLM behavioral test infrastructure and a judge scoring system. The code is well-structured, follows Go idioms, and correctly implements build tag separation (`llm` vs `llmjudge`). API key handling is sound -- keys are never logged and tests skip cleanly when keys are absent.

The main concerns are: (1) a verdict threshold gap between spec and implementation that could produce unexpected classifications, (2) a `selfJudged` flag computed from env vars at scoring time rather than carried through the call chain, creating a subtle correctness risk, and (3) a fragile pattern of appending to outer slices from within `t.Run` subtests that becomes a data race if parallelism is added.

No security vulnerabilities found. API keys are read from environment, never hardcoded, never logged.

## High

### HI-01: Verdict threshold gap between spec and implementation

**File:** `test/oracle/judge/rubric.go:89`
**Issue:** The spec (D-06) defines soft_fail as "one zero OR total 2.5-3.5" but the implementation uses `s.Total >= 2.5 && s.Total < 4.0`. This means scores with total in (3.5, 4.0) -- e.g., 3.75 with zero zeros -- are classified as `soft_fail` per the code, but fall in an undefined gap in the spec. With the 3-level scale (0.0, 0.5, 1.0), valid totals are {0.0, 0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5.0}, so 3.75 cannot actually occur with valid scores. However, `ComputeVerdict` does not validate score values before computing -- it trusts the caller. If called with unvalidated scores (e.g., an LLM returns 0.75), the gap produces incorrect verdicts.
**Fix:** Either validate scores inside `ComputeVerdict` or align the threshold to match the spec exactly:
```go
// Option A: Validate first (defensive)
func ComputeVerdict(s *Score) error {
    if err := ValidateScoreValues(s); err != nil {
        return err
    }
    // ... rest of logic
}

// Option B: Use spec-exact threshold (3.5 boundary)
case zeros == 1 || (s.Total >= 2.5 && s.Total <= 3.5):
    s.Verdict = "soft_fail"
```

## Medium

### MD-01: selfJudged flag computed from env vars at scoring time, not passed through

**File:** `test/oracle/judge/scorer.go:38`
**Issue:** `ScoreTranscript` calls `llm.JudgeModel()` to determine the `selfJudged` flag. This reads `SERENA_JUDGE_MODEL` and `SERENA_TEST_MODEL` env vars at scoring time. If the env vars differ between transcript generation and judge scoring (different sessions, CI stages), the flag may not reflect reality. Additionally, the `model` parameter passed to `ScoreTranscript` may differ from what `JudgeModel()` returns, creating an inconsistency.
**Fix:** Pass `selfJudged` as an explicit parameter to `ScoreTranscript` instead of re-deriving it from env vars:
```go
func ScoreTranscript(ctx context.Context, client anthropic.Client, model string, selfJudged bool, tr *llm.Transcript) (*Score, error) {
    // ...
    score.SelfJudged = tr.SelfJudged || selfJudged
```

### MD-02: Outer slice append from within t.Run subtests is race-prone

**File:** `test/oracle/judge/judge_test.go:78`
**Issue:** `scores = append(scores, score)` mutates a slice declared outside the `t.Run` closure. This works only because subtests run sequentially (no `t.Parallel()`). If any future maintainer adds `t.Parallel()` for speed, this becomes a data race. The same pattern appears in the aggregate computation at line 82 which reads `scores` after all subtests complete.
**Fix:** Add a comment guard or restructure to collect scores outside `t.Run`:
```go
// Collect scores outside t.Run to avoid future race risk.
for i, tr := range transcripts {
    // ... delay logic ...
    score, err := ScoreTranscript(ctx, client, model, tr)
    if err != nil {
        t.Logf("ERROR scoring %s: %v", tr.ScenarioID, err)
        continue
    }
    if selfJudged {
        score.SelfJudged = true
    }
    WriteScore(t, score)
    t.Logf("Score for %s: verdict=%s total=%.1f", tr.ScenarioID, score.Verdict, score.Total)
    scores = append(scores, score)
}
```

### MD-03: ParseScoreJSON uses naive brace matching

**File:** `test/oracle/judge/scorer.go:79-86`
**Issue:** `ParseScoreJSON` finds the first `{` and last `}` in the response. If the LLM wraps its response in markdown fences (` ```json ... ``` `) or includes explanatory text with braces, the extracted substring may include non-JSON content. The retry mechanism partially mitigates this, but the parser itself is fragile.
**Fix:** Strip markdown fences before brace matching:
```go
func ParseScoreJSON(text string) (*Score, error) {
    // Strip common markdown fences.
    text = strings.TrimSpace(text)
    if strings.HasPrefix(text, "```") {
        if idx := strings.Index(text, "\n"); idx != -1 {
            text = text[idx+1:]
        }
        if idx := strings.LastIndex(text, "```"); idx != -1 {
            text = text[:idx]
        }
    }
    // ... existing brace matching logic
```

## Low

### LO-01: selection_test.go captures loop variable unnecessarily (Go 1.22+)

**File:** `test/oracle/llm/selection_test.go:41`
**Issue:** `tool := tool // capture loop variable` is unnecessary in Go 1.22+ where loop variables are per-iteration. Same pattern in `disambiguation_test.go:75` and `interpretation_test.go:54,96`. This is not a bug but adds noise. Confirm the project's minimum Go version before removing.
**Fix:** Remove the re-declaration if Go 1.22+ is the minimum:
```go
// Remove: tool := tool // capture loop variable
```

### LO-02: interpretation_test.go callCount increment position

**File:** `test/oracle/llm/interpretation_test.go:86,129`
**Issue:** `callCount++` is placed after `t.Run()` outside the subtest closure. The increment happens regardless of whether the subtest succeeded, failed, or was skipped. The `InterCallDelay()` at the top of each iteration depends on this counter. While correct for sequential execution, the counter does not accurately reflect "API calls actually made" -- it reflects "subtests attempted." This could cause misleading log output at line 132.
**Fix:** Move `callCount++` inside the subtest after the successful API call, or rename to `subtestCount` for clarity.

## Info

### IN-01: Unused stopReason in interpretation tests

**File:** `test/oracle/llm/interpretation_test.go:61,102`
**Issue:** `_ = stopReason` explicitly discards the stop reason. While this is intentional (it is written to the transcript), the blank assignment is noise. The value is already captured in the `Transcript` struct. Simply assigning directly to the struct field would be cleaner.
**Fix:** Already written to transcript; remove the `_ = stopReason` lines.

### IN-02: TestJudgeInline is a no-op placeholder

**File:** `test/oracle/judge/judge_test.go:95-102`
**Issue:** `TestJudgeInline` does nothing except log a message. This is documented as a placeholder (D-16) but contributes to test count inflation. Consider using `t.Skip` with a message indicating it is not yet implemented rather than logging.
**Fix:** Already using `t.Skip` on the non-inline path. The inline path just logs. This is fine as-is; noting for awareness.

---

_Reviewed: 2026-04-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
