---
phase: 31-multi-language-grammar-expansion
plan: 03
subsystem: code-intelligence-kernel
tags: [tree-sitter, grammar, tag-extraction, body-extraction, multi-language]
dependency_graph:
  requires: [grammar-wave2a, tag-queries-wave2a, body-queries-wave2a]
  provides: [grammar-wave2b, tag-queries-wave2b, body-queries-wave2b]
  affects: [internal/treesitter, internal/repomap, internal/kernel/edit]
tech_stack:
  added:
    - tree-sitter-lua v0.5.0
    - tree-sitter-zig v1.1.2
    - tree-sitter-hcl v1.2.0
  patterns: [grammar-registration, tag-query-adaptation, body-config, error-tolerant-query-compilation]
key_files:
  created:
    - internal/repomap/queries/lua_tags.scm
    - internal/repomap/queries/zig_tags.scm
    - internal/repomap/queries/hcl_tags.scm
    - internal/kernel/edit/queries/lua.scm
    - internal/kernel/edit/queries/zig.scm
    - internal/kernel/edit/queries/hcl.scm
  modified:
    - internal/treesitter/registry.go
    - internal/treesitter/registry_test.go
    - internal/repomap/extractor.go
    - internal/repomap/extractor_test.go
    - internal/repomap/render.go
    - internal/kernel/edit/treesitter.go
    - internal/kernel/edit/treesitter_test.go
    - go.mod
    - go.sum
decisions:
  - Swift skipped due to broken Go binding (parser.c missing from upstream release)
  - R skipped due to broken Go binding (scanner.c not included in binding.go)
  - Zig grammar uses lowercase node types (function_declaration, not FnProto as in aider reference)
  - Error-tolerant query compilation in NewTagExtractor (log+skip instead of fail-all)
  - HCL body extraction omitted (block-based language, no function bodies)
metrics:
  duration_seconds: 1212
  completed: "2026-04-19T09:20:26Z"
  tasks: 3
  files_created: 6
  files_modified: 9
---

# Phase 31 Plan 03: Wave 2b Grammar Expansion Summary

Tree-sitter grammar support expanded from 18 to 21 languages with Lua, Zig, HCL tag and body extraction. Swift and R skipped due to broken upstream Go bindings (missing C source files in published modules). Error-tolerant query compilation added to prevent single broken query from failing all languages.

## Task Completion

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Grammar dependencies, registry, LangFromExt | fe2e60ba | registry.go, render.go, go.mod |
| 2a | Tag queries, extractor wiring, tag tests | e0ac6470 | 3 *_tags.scm, extractor.go, extractor_test.go |
| 2b | Body queries, langConfig, body tests | 71863440 | 3 *.scm, treesitter.go, treesitter_test.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Swift Go binding missing parser.c**
- **Found during:** Task 1
- **Issue:** `github.com/alex-pinkus/tree-sitter-swift` binding's `binding.go` includes `../../src/parser.c` via CGO, but the published module does not contain the generated `parser.c` file (only `scanner.c` and `grammar.json` in `src/`). Additionally, the `bindings/go` submodule has a module path mismatch (declares as `github.com/tree-sitter/tree-sitter-swift` but is hosted at `github.com/alex-pinkus/tree-sitter-swift`).
- **Fix:** Skipped Swift entirely. No compatible Go binding exists for Swift tree-sitter grammar.
- **Impact:** 21 total languages instead of planned 23. Swift falls back to LSP-based operations.

**2. [Rule 3 - Blocking] R Go binding missing scanner.c include**
- **Found during:** Task 1
- **Issue:** `github.com/r-lib/tree-sitter-r` binding's `binding.go` only includes `parser.c` but R grammar has an external scanner (`scanner.c`). The binding template comment says "NOTE: if your language has an external scanner, add it here" but the include was never added, causing linker errors for undefined `tree_sitter_r_external_scanner_*` symbols.
- **Fix:** Skipped R entirely. Upstream binding is broken.
- **Impact:** R falls back to LSP-based operations.

**3. [Rule 1 - Bug] Zig tag query PascalCase node types incorrect**
- **Found during:** Task 2a
- **Issue:** Aider's Zig query uses `FnProto`, `VarDecl`, `IDENTIFIER` (PascalCase) but `tree-sitter-grammars/tree-sitter-zig` v1.1.2 uses lowercase node types: `function_declaration`, `variable_declaration`, `identifier`.
- **Fix:** Rewrote Zig tag query with correct lowercase node types.
- **Files modified:** zig_tags.scm
- **Commit:** e0ac6470

## Decisions Made

1. **Swift binding skip:** No usable Go binding exists -- parser.c missing from release + module path mismatch
2. **R binding skip:** Upstream binding broken -- external scanner not included in CGO includes
3. **Error-tolerant query compilation:** NewTagExtractor now logs warning and skips failed queries instead of failing all language extraction
4. **Zig query rewrite:** Adapted from PascalCase (aider reference) to lowercase (actual grammar) node types
5. **HCL body omission:** No langConfig entry -- HCL is block-based with no function bodies, falls back to LSP editing

## Verification

- `go build ./cmd/serena` -- PASS
- `go test ./... -count=1` -- PASS (full suite green, all packages)
- `go vet ./...` -- PASS
- GrammarRegistry reports 21 supported languages: bash, c, c_sharp, cpp, go, haskell, hcl, java, javascript, julia, kotlin, lua, ocaml, php, python, ruby, rust, scala, tsx, typescript, zig

## Known Stubs

None -- all queries are functional and tested.

## Self-Check: PASSED
