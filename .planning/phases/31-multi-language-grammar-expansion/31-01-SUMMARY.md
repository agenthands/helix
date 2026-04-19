---
phase: 31-multi-language-grammar-expansion
plan: 01
subsystem: code-intelligence-kernel
tags: [tree-sitter, grammar, tag-extraction, body-extraction, multi-language]
dependency_graph:
  requires: []
  provides: [grammar-wave1, tag-queries-wave1, body-queries-wave1]
  affects: [internal/treesitter, internal/repomap, internal/kernel/edit]
tech_stack:
  added:
    - tree-sitter-java v0.23.5
    - tree-sitter-c v0.24.1
    - tree-sitter-cpp v0.23.4
    - tree-sitter-c-sharp v0.23.5
    - tree-sitter-ruby v0.23.1
    - tree-sitter-php v0.24.2
    - tree-sitter-javascript v0.25.0
    - tree-sitter-kotlin v1.1.0 (tree-sitter-grammars)
  patterns: [grammar-registration, tag-query-adaptation, body-config, qualified-name-resolution]
key_files:
  created:
    - internal/repomap/queries/java_tags.scm
    - internal/repomap/queries/c_tags.scm
    - internal/repomap/queries/cpp_tags.scm
    - internal/repomap/queries/csharp_tags.scm
    - internal/repomap/queries/ruby_tags.scm
    - internal/repomap/queries/php_tags.scm
    - internal/repomap/queries/javascript_tags.scm
    - internal/repomap/queries/kotlin_tags.scm
    - internal/kernel/edit/queries/java.scm
    - internal/kernel/edit/queries/c.scm
    - internal/kernel/edit/queries/cpp.scm
    - internal/kernel/edit/queries/csharp.scm
    - internal/kernel/edit/queries/ruby.scm
    - internal/kernel/edit/queries/php.scm
    - internal/kernel/edit/queries/javascript.scm
    - internal/kernel/edit/queries/kotlin.scm
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
  - Kotlin binding sourced from tree-sitter-grammars org (fwcd fork has Go module path mismatch)
  - JavaScript query adapted for tree-sitter-javascript v0.25.0 (function_expression not function)
  - Kotlin grammar uses identifier instead of type_identifier/simple_identifier
  - bodyNodeKind fallback added for Kotlin (function_body is unnamed child)
  - extractNodeName extended for C/C++ declarator nesting pattern
metrics:
  duration_seconds: 665
  completed: "2026-04-19T08:46:59Z"
  tasks: 3
  files_created: 16
  files_modified: 9
---

# Phase 31 Plan 01: Wave 1 Grammar Expansion Summary

Tree-sitter grammar support expanded from 5 to 13 languages with tag and body extraction for Java, C, C++, C#, Ruby, PHP, JavaScript, Kotlin -- all adapted from aider reference queries to Serena's capture convention with qualified name resolution for OOP languages.

## Task Completion

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Grammar dependencies, registry, and LangFromExt | 508fa637 | registry.go, render.go, go.mod |
| 2a | Tag queries, qualified names, and tag tests | 5d2046be | 8 *_tags.scm, extractor.go, extractor_test.go |
| 2b | Body queries, langConfig, and body tests | 8ce58ef8 | 8 *.scm, treesitter.go, treesitter_test.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Kotlin binding package path mismatch**
- **Found during:** Task 1
- **Issue:** `github.com/fwcd/tree-sitter-kotlin/bindings/go@v0.3.2` does not exist as a Go module. The fwcd fork declares module path as `github.com/tree-sitter/tree-sitter-kotlin` but was required as `github.com/fwcd/tree-sitter-kotlin/bindings/go`.
- **Fix:** Used `github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go@latest` (v1.1.0) which has correct Go module structure.
- **Files modified:** go.mod, registry.go
- **Commit:** 508fa637

**2. [Rule 1 - Bug] JavaScript query node types incompatible with v0.25.0**
- **Found during:** Task 2a
- **Issue:** Aider's JavaScript query uses `(function ...)` and `(generator_function ...)` node types which don't exist in tree-sitter-javascript v0.25.0. The correct types are `function_expression` and `generator_function` (already exists but without name field for anonymous ones).
- **Fix:** Rewrote JavaScript tag query using verified AST node types: `function_declaration`, `function_expression`, `generator_function_declaration`, `generator_function`, `class_declaration`, `method_definition`.
- **Files modified:** javascript_tags.scm
- **Commit:** 5d2046be

**3. [Rule 1 - Bug] Kotlin grammar uses different node type names**
- **Found during:** Task 2a
- **Issue:** The tree-sitter-grammars Kotlin grammar uses `identifier` instead of `type_identifier`/`simple_identifier`, and does not have `navigation_suffix` node type. The aider query was written for the fwcd grammar version.
- **Fix:** Rewrote Kotlin tag query and qualified name function to use `identifier` instead of `type_identifier`.
- **Files modified:** kotlin_tags.scm, extractor.go
- **Commit:** 5d2046be

**4. [Rule 2 - Missing functionality] C/C++ body extraction name resolution**
- **Found during:** Task 2b
- **Issue:** `extractNodeName` only looked for `ChildByFieldName("name")`, but C/C++ `function_definition` nodes use `declarator -> function_declarator -> declarator (identifier)` nesting.
- **Fix:** Extended `extractNodeName` with C/C++ declarator traversal fallback.
- **Files modified:** treesitter.go
- **Commit:** 8ce58ef8

**5. [Rule 2 - Missing functionality] Kotlin body field not named**
- **Found during:** Task 2b
- **Issue:** Kotlin's `function_declaration` has `function_body` as an unnamed child (not a named field), so `ChildByFieldName("body")` returns nil.
- **Fix:** Added `bodyNodeKind` field to `langConfig` and fallback logic in `ExtractBody` to find body by node kind.
- **Files modified:** treesitter.go
- **Commit:** 8ce58ef8

## Decisions Made

1. **Kotlin binding source:** tree-sitter-grammars org at v1.1.0 (not fwcd fork) due to Go module path mismatch
2. **JavaScript query rewrite:** Adapted for v0.25.0 grammar with `function_expression` instead of `function`
3. **bodyNodeKind fallback:** New langConfig field for grammars where body is unnamed child (Kotlin)
4. **extractNodeName extension:** Declarator traversal for C/C++ compatible with existing languages

## Verification

- `go build ./cmd/serena` -- PASS
- `go test ./internal/treesitter/... ./internal/repomap/... ./internal/kernel/edit/... -count=1` -- PASS (all 3 packages)
- `go vet ./...` -- PASS
- GrammarRegistry reports 13 supported languages: c, c_sharp, cpp, go, java, javascript, kotlin, php, python, ruby, rust, tsx, typescript

## Known Stubs

None -- all queries are functional and tested.

## Self-Check: PASSED

All 16 created files exist. All 3 commit hashes verified in git log.
