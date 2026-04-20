---
phase: 33-fallback-extractor-wiring
plan: 02
subsystem: repomap
tags: [fallback-extractor, testing, compile-time-assertion, cache-persistence]
dependency_graph:
  requires: [33-01-fallback-wiring]
  provides: [fallback-path-tests, interface-assertion]
  affects: [internal/skill/repomap/skill_test.go]
tech_stack:
  added: []
  patterns: [compile-time-interface-assertion, mock-symbol-requester]
key_files:
  created: []
  modified:
    - internal/skill/repomap/skill_test.go
decisions:
  - "Use extractor=nil to trigger fallback path in tests (all LangFromExt languages have tree-sitter grammars; nil extractor naturally routes to fallback)"
metrics:
  duration_seconds: 271
  completed: "2026-04-20T13:45:41Z"
---

# Phase 33 Plan 02: Fallback Integration Tests & Interface Assertion Summary

Compile-time assertion proves WorkerLease satisfies SymbolRequester; integration tests verify fallback extraction fires for non-tree-sitter path and silently skips when no LS is available.

## What Was Done

### Task 1: Add compile-time interface assertion and TestWalkAndExtract_FallbackPath integration test
- Added `var _ repomap.SymbolRequester = (*lspool.WorkerLease)(nil)` compile-time assertion (D-33-04)
- Added `mockFallbackRequester` type using JSON unmarshal to simulate LSP documentSymbol responses
- Added `TestWalkAndExtract_FallbackPath`: creates a Java file with extractor=nil to force fallback, verifies AcquireFn is called, tags are cached via FallbackExtractor
- Added `TestWalkAndExtract_FallbackSkipsWhenNoLS`: AcquireFn returns error, verifies walkAndExtract does not error (D-33-02 silent skip) and no tags are cached
- **Commit:** `a8e1b049`

### Task 2: Verify existing cache persistence test covers D-33-03 and run full suite
- Confirmed `TestTagCache_Persistence` in `internal/repomap/cache_test.go` covers all D-33-03 criteria: create cache, insert tags, close, reopen at same path, verify tags present without re-extraction
- No code changes needed -- test already exists and passes
- Full test suite (`go test ./... -count=1`) green, `go vet ./...` clean

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] LangFromExt does not map .pl to "perl"**
- **Found during:** Task 1
- **Issue:** Plan assumed `.pl` extension maps to `"perl"` via `LangFromExt`, but `LangFromExt` has no entry for `.pl`. Files with unmapped extensions are silently skipped (return `""` from LangFromExt), so the fallback path was never reached.
- **Fix:** Changed test approach to use `.java` files with `extractor=nil` (no tree-sitter extractor on the skill). Since the tree-sitter path condition requires `s.extractor != nil`, a nil extractor naturally routes to the fallback path. This tests the same integration wiring without requiring a language absent from both LangFromExt and the grammar registry.
- **Files modified:** `internal/skill/repomap/skill_test.go`
- **Commit:** `a8e1b049`

## Verification Results

- `go test ./internal/skill/repomap/... -run TestWalkAndExtract_FallbackPath -count=1` -- PASS
- `go test ./internal/skill/repomap/... -run TestWalkAndExtract_FallbackSkipsWhenNoLS -count=1` -- PASS
- `go test ./internal/repomap/... -run TestTagCache_Persistence -count=1` -- PASS
- `go test ./... -count=1` -- PASS (full suite green)
- `go vet ./...` -- clean
- Compile-time assertion compiles successfully

## Self-Check: PASSED
