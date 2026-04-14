---
phase: 21-llm-behavioral-judge
plan: 02
subsystem: test/oracle/judge, test/oracle/llm
tags: [llm-judge, rubrics, scoring, interpretation, behavioral-testing]
dependency_graph:
  requires: [test/oracle/llm/client.go, test/oracle/llm/prompt.go, test/oracle/llm/transcript.go, test/harness]
  provides: [test/oracle/llm/interpretation_test.go, test/oracle/judge/rubric.go, test/oracle/judge/scorer.go, test/oracle/judge/aggregate.go, test/oracle/judge/judge_test.go]
  affects: [test/oracle/llm/prompt.go]
tech_stack:
  added: []
  patterns: [llmjudge-build-tag, structured-rubric-scoring, json-score-parsing, offline-judge-flow]
key_files:
  created:
    - test/oracle/llm/interpretation_test.go
    - test/oracle/judge/rubric.go
    - test/oracle/judge/scorer.go
    - test/oracle/judge/aggregate.go
    - test/oracle/judge/judge_test.go
  modified:
    - test/oracle/llm/prompt.go
decisions:
  - "GoldenCase type and LoadGoldenFiles/LoadErrorGoldenFiles added to prompt.go for golden file walking"
  - "Interpretation tests sample every Nth case when >30 success goldens to cap API calls at ~40"
  - "Score validation uses retry-once pattern with explicit error message appended to prompt"
  - "Judge test uses t.Log not t.Error for score results — informational only, never blocks merge"
  - "Aggregate worst dimensions threshold set at 0.7 average"
metrics:
  duration: ~3 min
  completed: 2026-04-12
  tasks: 2/2
  files_created: 5
  files_modified: 1
---

# Phase 21 Plan 02: Output Interpretation & Judge Scoring Infrastructure Summary

5-dimension LLM judge with structured rubric anchors, offline transcript scoring, and aggregate reporting, plus interpretation behavioral tests that verify Claude correctly classifies tool output success/failure from golden files.

## Task Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Interpretation prompts and tests | 2cf81d4d | test/oracle/llm/prompt.go, test/oracle/llm/interpretation_test.go |
| 2 | Judge scoring infrastructure | bee40c08 | test/oracle/judge/rubric.go, scorer.go, aggregate.go, judge_test.go |

## Implementation Details

### Task 1: Interpretation Prompts and Tests
- Added `InterpretationSystemPrompt()` with structured STATUS/SUMMARY/LIMITATIONS/NEXT_ACTION format
- Added `InterpretationUserPrompt()` to format tool name + golden content
- Added `GoldenCase` type and `LoadGoldenFiles()`/`LoadErrorGoldenFiles()` to walk golden test data
- `TestInterpretation` feeds golden file content to Claude, asserts SUCCESS for success goldens and FAILURE/UNCLEAR for error goldens
- Deterministic sampling (every Nth, sorted by name) when >30 success cases
- Transcripts written for downstream judge consumption

### Task 2: Judge Scoring Infrastructure
- `Score` struct with 5 dimensions (tool_choice, description_use, output_interpretation, uncertainty_handling, polyglot_reasoning) plus total, verdict, failures, self_judged
- `ComputeVerdict()` implements D-06 thresholds: pass (no zeros, total >= 4.0), soft_fail (1 zero or total 2.5-4.0), fail (2+ zeros or total < 2.5)
- `ValidateScoreValues()` enforces {0.0, 0.5, 1.0} only (T-21-05 mitigation)
- `RubricPrompt()` returns full anchor definitions for all 5 dimensions per D-08
- `ScoreTranscript()` calls judge model, parses JSON response, validates, retries once on invalid values
- `ParseScoreJSON()` extracts JSON object from free-form LLM response
- `WriteScore()` persists per-scenario scores to testdata/scores/
- `AggregateReport` with dimension averages, pass/fail/soft_fail counts, worst performers/dimensions
- `TestJudge` reads transcripts from Plan 01/02, scores them sequentially, writes scores and aggregate
- `TestJudgeInline` placeholder for inline mode (D-16)
- Build tag `llmjudge` controls compilation per D-17

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- `go vet -tags="llmjudge" ./test/oracle/judge/...` passes
- `go vet -tags="llm" ./test/oracle/llm/...` passes
- `go vet ./...` (no tags) excludes judge package as expected
