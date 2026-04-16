---
phase: 26-fuzzy-edit-integration
plan: 02
subsystem: kernel/edit
tags: [fuzzy-matching, symbol-editing, replace-body, resilient-editing]
dependency_graph:
  requires: [internal/fuzzy (fuzzy.Match engine from 25-01)]
  provides: [SearchBody parameter on replace_symbol_body, FuzzyMatchInfo response metadata]
  affects: [internal/kernel/edit/replace.go, internal/kernel/edit/tools.go]
tech_stack:
  added: []
  patterns: [fuzzy fallback within tree-sitter body region, body-relative to absolute offset translation]
key_files:
  created:
    - internal/kernel/edit/replace_fuzzy_test.go
  modified:
    - internal/kernel/edit/replace.go
    - internal/kernel/edit/tools.go
decisions:
  - Nil guard on lease.Notify enables unit testing without full LSP worker lease
  - AllowEllipsis set to false for replace_symbol_body (symbol-oriented, not line-oriented)
metrics:
  duration_seconds: 199
  completed: "2026-04-16T15:24:26Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 2
---

# Phase 26 Plan 02: replace_symbol_body fuzzy fallback Summary

SearchBody fuzzy parameter on replace_symbol_body enabling sub-region matching within tree-sitter-extracted body via 4-strategy cascade

## What Was Done

### Task 1: Add SearchBody to ReplaceBodyArgs and wire fuzzy fallback
- Added `SearchBody string` field to `ReplaceBodyArgs` with `json:"search_body,omitempty"` tag
- Added `FuzzyMatchInfo` struct carrying `fuzzy.Strategy` and `Score` back to handler
- Changed `ReplaceBodyWithPlan` signature: `(ctx, lease, extractor, plan, lang, searchBody) -> (*FuzzyMatchInfo, error)`
- When `searchBody != ""`: fuzzy-matches within `source[startByte:endByte]` body region, translates body-relative offsets to absolute file positions
- When `searchBody == ""`: existing full-body-replace behavior is unchanged
- Handler formats response with `match_strategy`/`similarity_score` only when fuzzy was used (D-07/D-08)
- Added nil guard on `lease` for `notifyDidChange` calls to enable nil-lease testing
- Updated `ReplaceBody` convenience function to pass `""` as searchBody
- **Commit:** a8391f4f

### Task 2: Unit tests for replace_symbol_body fuzzy fallback
- `TestReplaceBodyFuzzy_ExactMatchWithinBody`: exact sub-region match preserves other lines in body
- `TestReplaceBodyFuzzy_WhitespaceNormalized`: trailing whitespace triggers whitespace_normalized strategy (score 0.95)
- `TestReplaceBodyFuzzy_NoSearchBody_FullReplace`: empty searchBody returns nil FuzzyMatchInfo (D-08 compliance)
- `TestReplaceBodyFuzzy_NoMatchInBody`: non-matching searchBody returns "no fuzzy match found" error
- `TestReplaceBodyFuzzy_OffsetTranslation`: body-relative to absolute offset translation correctness with multi-line body
- **Commit:** 8fc62110

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Functionality] Added nil guard on lease for notifyDidChange**
- **Found during:** Task 1
- **Issue:** Tests need to call ReplaceBodyWithPlan with nil lease (no LSP worker), but notifyDidChange would panic on nil lease
- **Fix:** Added `if lease != nil` guard on both fuzzy and non-fuzzy write paths
- **Files modified:** internal/kernel/edit/replace.go

**2. [Rule 2 - Missing Test] Added offset translation test**
- **Found during:** Task 2
- **Issue:** Plan listed 4 tests but offset translation (Pitfall 2 from plan) deserved its own test case
- **Fix:** Added TestReplaceBodyFuzzy_OffsetTranslation verifying alpha/beta/gamma line preservation
- **Files modified:** internal/kernel/edit/replace_fuzzy_test.go

## Verification Results

```
go build ./internal/kernel/edit/...   -- PASS
go vet ./internal/kernel/edit/...     -- PASS
go test ./internal/kernel/edit/...    -- PASS (all 25 tests)
go build ./cmd/serena                 -- PASS
```

## Self-Check: PASSED
