---
phase: 31-multi-language-grammar-expansion
plan: 02
subsystem: code-intelligence-kernel
tags: [tree-sitter, grammar, tag-extraction, body-extraction, multi-language]
dependency_graph:
  requires: [grammar-wave1, tag-queries-wave1, body-queries-wave1]
  provides: [grammar-wave2a, tag-queries-wave2a, body-queries-wave2a]
  affects: [internal/treesitter, internal/repomap, internal/kernel/edit]
tech_stack:
  added:
    - tree-sitter-scala v0.26.0
    - tree-sitter-bash v0.25.1
    - tree-sitter-haskell v0.23.1
    - tree-sitter-julia v0.25.0
    - tree-sitter-ocaml v0.24.2
  patterns: [grammar-registration, tag-query-adaptation, body-config, qualified-name-resolution, bodyNodeKind-fallback]
key_files:
  created:
    - internal/repomap/queries/scala_tags.scm
    - internal/repomap/queries/bash_tags.scm
    - internal/repomap/queries/haskell_tags.scm
    - internal/repomap/queries/julia_tags.scm
    - internal/repomap/queries/ocaml_tags.scm
    - internal/kernel/edit/queries/scala.scm
    - internal/kernel/edit/queries/bash.scm
    - internal/kernel/edit/queries/haskell.scm
    - internal/kernel/edit/queries/julia.scm
    - internal/kernel/edit/queries/ocaml.scm
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
  - OCaml binding exports LanguageOCaml() not Language()
  - Julia grammar v0.25.0 uses signature->call_expression->identifier for function names (not name field)
  - Julia body extraction uses bodyNodeKind fallback (block is unnamed child)
  - Haskell and OCaml omit langConfig (equation-based definitions have no standard body field)
metrics:
  duration_seconds: 430
  completed: "2026-04-19T09:00:00Z"
  tasks: 3
  files_created: 10
  files_modified: 9
---

# Phase 31 Plan 02: Wave 2a Grammar Expansion Summary

Tree-sitter grammar support expanded from 13 to 18 languages with tag extraction for Scala, Bash, Haskell, Julia, OCaml and body extraction for Scala, Bash, Julia -- adapted from aider reference queries with Julia grammar node-type fixes and OCaml predicate stripping.

## Task Completion

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Grammar dependencies, registry, LangFromExt | 6084bdda | registry.go, render.go, go.mod |
| 2a | Tag queries, qualified names, and tag tests | b73d25eb | 5 *_tags.scm, extractor.go, extractor_test.go |
| 2b | Body queries, langConfig, and body tests | 7d8da57b | 5 *.scm, treesitter.go, treesitter_test.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Julia tag query node types incompatible with v0.25.0**
- **Found during:** Task 2a
- **Issue:** Plan's Julia tag query used `(struct_definition name: (identifier))`, `(function_definition name: (identifier))`, and `(macro_definition name: (identifier))` but Julia grammar v0.25.0 has no `name` field on these nodes. `struct_definition` nests name in `type_head -> identifier`. `function_definition` and `macro_definition` nest name in `signature -> call_expression -> identifier`.
- **Fix:** Rewrote Julia tag query to match actual v0.25.0 grammar structure.
- **Files modified:** julia_tags.scm
- **Commit:** b73d25eb

**2. [Rule 2 - Missing functionality] Julia body extraction name resolution**
- **Found during:** Task 2b
- **Issue:** `extractNodeName` could not find Julia function names because `function_definition` has no `name` field -- name is nested at `signature -> call_expression -> identifier`.
- **Fix:** Extended `extractNodeName` with Julia signature traversal fallback.
- **Files modified:** treesitter.go
- **Commit:** 7d8da57b

## Decisions Made

1. **OCaml binding function:** Uses `LanguageOCaml()` (not `Language()`) per binding convention
2. **Julia query rewrite:** Adapted for v0.25.0 grammar with `type_head` and `signature` nesting
3. **Julia bodyNodeKind fallback:** Body is unnamed `block` child, uses existing `bodyNodeKind` pattern from Kotlin
4. **Haskell/OCaml body omission:** No langConfig entries -- equation-based definitions lack standard body field, falls back to LSP-based editing

## Verification

- `go build ./cmd/serena` -- PASS
- `go test ./internal/treesitter/... ./internal/repomap/... ./internal/kernel/edit/... -count=1` -- PASS (all 3 packages)
- `go vet ./...` -- PASS
- GrammarRegistry reports 18 supported languages: bash, c, c_sharp, cpp, go, haskell, java, javascript, julia, kotlin, ocaml, php, python, ruby, rust, scala, tsx, typescript

## Known Stubs

None -- all queries are functional and tested.

## Self-Check: PASSED

All 10 created files exist. All 3 commit hashes verified in git log.
