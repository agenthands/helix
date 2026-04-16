---
phase: 27-repomap-tag-extraction-cache
plan: 02
subsystem: repomap
tags: [lsp-fallback, tag-extraction, documentSymbol]
dependency_graph:
  requires: [lspool, protocol/gen]
  provides: [FallbackExtractor, SymbolRequester]
  affects: [repomap]
tech_stack:
  added: []
  patterns: [interface-based-mocking, recursive-symbol-flattening]
key_files:
  created:
    - internal/repomap/fallback.go
    - internal/repomap/fallback_test.go
  modified: []
decisions:
  - "SymbolRequester interface for testability instead of direct WorkerLease dependency"
  - "Qualified names use dot notation (ClassName.methodName) per D-03"
  - "StartByte/EndByte set to 0 since LSP does not provide byte offsets"
metrics:
  duration: 84s
  completed: 2026-04-16T17:54:32Z
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 0
---

# Phase 27 Plan 02: LSP DocumentSymbol Fallback Extractor Summary

LSP documentSymbol fallback with SymbolRequester interface producing def-only tags with qualified names for nested symbols.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | LSP fallback extractor | 72f164c3 | internal/repomap/fallback.go |
| 2 | Fallback extractor tests with mock LSP | f3dd1a8d | internal/repomap/fallback_test.go |

## Implementation Details

### FallbackExtractor (fallback.go)
- `SymbolRequester` interface abstracts `WorkerLease.Request` for testability
- `FallbackExtractor.Extract()` calls `textDocument/documentSymbol` and flattens hierarchical response
- All symbols mapped as `TagDef` only (D-07 constraint enforced)
- Nested symbols produce qualified names via `parentName.childName` format (D-03)
- `StartByte`/`EndByte` set to 0 since LSP lacks byte offset information

### Test Coverage (fallback_test.go)
- 5 test cases covering: flat symbols, nested/qualified names, no-ref constraint (D-07), error propagation, empty response
- Uses `mockRequester` implementing `SymbolRequester` interface
- D-07 explicitly validated: no tag has `TagRef` kind

## Decisions Made

1. **SymbolRequester interface**: Decouples from `lspool.WorkerLease` concrete type, enabling unit testing with mocks while `WorkerLease` still satisfies the interface at runtime.
2. **Qualified name format**: Uses dot notation (`ClassName.methodName`) matching common convention for nested symbol representation.
3. **Zero byte offsets**: LSP documentSymbol provides line/character positions but not byte offsets; elision will use line-based fallback when byte offsets are 0.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.
