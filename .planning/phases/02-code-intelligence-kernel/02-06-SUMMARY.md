---
phase: 02-code-intelligence-kernel
plan: 06
subsystem: edit
tags: [tree-sitter, lsp, symbol-editing, cgo, ast]

requires:
  - phase: 02-04
    provides: "Symbol retrieval (FindReferences, GetSymbolOverview) for edit planning and safe delete"
  - phase: 02-05
    provides: "DiagnosticStore with WaitForDiagnostics for post-edit verification"
provides:
  - "Tree-sitter body extraction for Go, Python, TypeScript, Rust"
  - "6 symbol editing operations: replace body, insert before/after, rename, safe delete, verify"
  - "6 MCP tools for symbol editing with auto-verify"
affects: [03-multi-language, 04-integration]

tech-stack:
  added: [go-tree-sitter v0.25.0, tree-sitter-go, tree-sitter-python, tree-sitter-typescript, tree-sitter-rust]
  patterns: [tree-sitter-first body surgery with LSP fallback, auto-verify after mutation, dirty lease acquisition]

key-files:
  created:
    - internal/kernel/edit/treesitter.go
    - internal/kernel/edit/planner.go
    - internal/kernel/edit/replace.go
    - internal/kernel/edit/insert.go
    - internal/kernel/edit/rename.go
    - internal/kernel/edit/delete.go
    - internal/kernel/edit/verify.go
    - internal/kernel/edit/tools.go
    - internal/kernel/edit/treesitter_test.go
    - internal/kernel/edit/edit_test.go
    - internal/kernel/edit/queries/go.scm
    - internal/kernel/edit/queries/python.scm
    - internal/kernel/edit/queries/typescript.scm
    - internal/kernel/edit/queries/rust.scm
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Tree-sitter grammars imported via top-level module path (e.g. tree-sitter-typescript not bindings/go sub-path) due to Go module path declaration"
  - "CGO_ENABLED=1 required for tree-sitter C bindings; Apple Clang sufficient on macOS"
  - "Per-language .scm query files document declaration types and body field names (configuration, not runtime queries)"
  - "Mutation MCP tools automatically run VerifyEdit after each operation for immediate error feedback"

patterns-established:
  - "Tree-sitter-first body extraction (D-14) with LSP range fallback (D-15) for unsupported languages"
  - "Dirty lease pattern: mutation tools acquire leases with dirty=true for write serialization"
  - "Auto-verify pattern: mutation handlers append diagnostic results to tool response"

requirements-completed: [EDT-01, EDT-02, EDT-03, EDT-04, EDT-05, EDT-06]

duration: 7min
completed: 2026-04-07
---

# Phase 02 Plan 06: Symbol Editing Tools Summary

**Tree-sitter body extraction for 4 languages with 6 symbol editing MCP tools and automatic post-edit diagnostic verification**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-07T20:38:44Z
- **Completed:** 2026-04-07T20:45:59Z
- **Tasks:** 2
- **Files modified:** 16

## Accomplishments
- Tree-sitter body extraction precise for Go, Python, TypeScript, Rust with per-language declaration type configuration
- All 6 edit operations implemented: replace body (tree-sitter-first), insert before/after (LSP-first), rename (LSP forwarding), safe delete (reference-checked), verify edit (diagnostics)
- 6 MCP tools registered with mutation tools auto-verifying via DiagnosticStore
- 18 tests passing covering all 4 languages and edit operation types

## Task Commits

Each task was committed atomically:

1. **Task 1: Tree-sitter integration and edit operations** - `738c7e51` (feat)
2. **Task 2: MCP tool registration for edit operations** - `bfdef847` (feat)

## Files Created/Modified
- `internal/kernel/edit/treesitter.go` - BodyExtractor with Go/Python/TypeScript/Rust grammars
- `internal/kernel/edit/planner.go` - EditPlan coordinator resolving symbols via LSP
- `internal/kernel/edit/replace.go` - ReplaceBody with tree-sitter-first, LSP fallback
- `internal/kernel/edit/insert.go` - InsertBefore/InsertAfter using LSP DocumentSymbol ranges
- `internal/kernel/edit/rename.go` - RenameSymbol via LSP textDocument/rename
- `internal/kernel/edit/delete.go` - SafeDelete with reference checking
- `internal/kernel/edit/verify.go` - VerifyEdit post-edit diagnostic verification
- `internal/kernel/edit/tools.go` - 6 MCP tools with auto-verify pattern
- `internal/kernel/edit/treesitter_test.go` - Tree-sitter body extraction tests for 4 languages
- `internal/kernel/edit/edit_test.go` - Edit operation unit tests
- `internal/kernel/edit/queries/*.scm` - Per-language declaration type configuration

## Decisions Made
- Tree-sitter grammar packages use top-level module paths (tree-sitter/tree-sitter-typescript) not bindings/go sub-paths due to Go module declaration mismatch
- CGO_ENABLED=1 required; Apple Clang on macOS is sufficient
- .scm query files serve as documentation/configuration for declaration types and body field names
- Mutation tools auto-verify: every replace/insert/rename/delete appends diagnostic results to response

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Tree-sitter grammar Go bindings declare their module path at the repository root (e.g. `github.com/tree-sitter/tree-sitter-python`) not at `bindings/go` sub-path. Resolved by using the correct top-level import path.

## User Setup Required
None - no external service configuration required. CGO_ENABLED=1 is the only build-time requirement (standard on macOS with Xcode CLT).

## Next Phase Readiness
- All Phase 2 code intelligence requirements complete (SYM-01 through SYM-09, EDT-01 through EDT-06, FIL-01 through FIL-06, DGN-01 through DGN-03)
- Phase 2 ready for verification
- Tree-sitter grammars extensible for Phase 3 multi-language expansion

## Self-Check: PASSED

All 14 created files verified present. Both task commits (738c7e51, bfdef847) verified in git log.

---
*Phase: 02-code-intelligence-kernel*
*Completed: 2026-04-07*
