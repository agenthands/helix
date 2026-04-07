---
phase: 02-code-intelligence-kernel
plan: 04
subsystem: symbols
tags: [symbol-retrieval, lsp, mcp-tools, hierarchy, blast-radius]
dependency_graph:
  requires: [lspool, kernel, mcp-registry, gen-types]
  provides: [symbol-retrieval, symbol-hierarchy, symbol-overview, symbol-search, blast-radius, symbol-mcp-tools]
  affects: [symbol-editing-plan-05]
tech_stack:
  added: []
  patterns: [lease-based-lsp-dispatch, recursive-hierarchy-resolution, blast-radius-deduplication]
key_files:
  created:
    - internal/kernel/symbols/retrieval.go
    - internal/kernel/symbols/hierarchy.go
    - internal/kernel/symbols/overview.go
    - internal/kernel/symbols/search.go
    - internal/kernel/symbols/blast.go
    - internal/kernel/symbols/tools.go
    - internal/kernel/symbols/retrieval_test.go
  modified: []
decisions:
  - "Hierarchy recursion capped at depth 3 to prevent unbounded LSP request chains"
  - "Blast radius treats refs/callers/implementations failures as non-fatal for partial results"
  - "0-indexed line/col in MCP tool args matching LSP protocol, 1-indexed in display output"
metrics:
  duration: 5min
  completed: "2026-04-07T20:35:00Z"
---

# Phase 02 Plan 04: Symbol Retrieval Tools Summary

All 9 symbol retrieval operations implemented as LSP-backed functions with MCP tool registration, hierarchy recursion with depth limits, and blast radius deduplication.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Symbol retrieval, hierarchy, overview, search, blast radius | c7ebed80 | retrieval.go, hierarchy.go, overview.go, search.go, blast.go, retrieval_test.go |
| 2 | MCP tool registration for symbol operations | edea5321 | tools.go |

## Implementation Details

### Task 1: Core Symbol Operations

Created `internal/kernel/symbols/` package with 5 implementation files:

- **retrieval.go**: `GoToDefinition`, `FindReferences`, `GetHover`, `FindImplementations` -- each delegates through WorkerLease.Request to the LSP, returning domain types (SymbolLocation, HoverResult). Includes `SymbolKindName` helper and `extractHoverContent` handling the LSP Hover union type.

- **hierarchy.go**: `GetCallHierarchy`, `GetTypeHierarchy` -- three-step LSP protocol (prepare, resolve incoming/outgoing or subtypes/supertypes, recurse). Depth-limited to 3 levels via `maxHierarchyDepth` constant.

- **overview.go**: `GetSymbolOverview` -- sends textDocument/documentSymbol and recursively maps DocumentSymbol[] to SymbolOutline[], preserving class-method-field hierarchy.

- **search.go**: `SearchSymbols` -- sends workspace/symbol, maps SymbolInformation to SymbolLocation with name and kind.

- **blast.go**: `AnalyzeBlastRadius` -- combines FindReferences + GetCallHierarchy(incoming) + FindImplementations, deduplicates by URI:line:col key, groups by file, computes total impact count. Each sub-operation is non-fatal (partial results on server capability gaps).

### Task 2: MCP Tool Registration

Created `tools.go` registering 9 tools via `RegisterTools(server, kernel, wsKeyFn)`:

1. `go_to_definition` (SYM-01)
2. `find_references` (SYM-02)
3. `get_symbol_overview` (SYM-03)
4. `search_symbols` (SYM-04)
5. `get_hover_info` (SYM-05)
6. `find_implementations` (SYM-06)
7. `get_call_hierarchy` (SYM-07)
8. `get_type_hierarchy` (SYM-08)
9. `analyze_blast_radius` (SYM-09)

Each handler: resolves workspace runtime from kernel, acquires clean read lease, converts path to file:// URI, calls underlying symbol function, formats result as text for MCP response.

### Tests

11 unit tests covering:
- Location type conversion
- Hover content extraction (MarkupContent, string, nil, map variants)
- SymbolKind name mapping
- DocumentSymbol hierarchy preservation
- Blast radius deduplication logic
- Position parameter construction
- Hierarchy depth limit constant

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None -- all functions are fully wired to WorkerLease LSP calls.

## Verification

- `go build ./...` -- passes
- `go test ./internal/kernel/symbols/... -v -count=1` -- 11/11 pass
- `go vet ./internal/kernel/symbols/...` -- clean
