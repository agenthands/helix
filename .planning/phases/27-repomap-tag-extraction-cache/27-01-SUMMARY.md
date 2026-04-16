---
phase: 27-repomap-tag-extraction-cache
plan: 01
subsystem: treesitter, repomap, kernel/edit
tags: [tree-sitter, tag-extraction, grammar-registry, repomap]
dependency_graph:
  requires: []
  provides: [treesitter.GrammarRegistry, repomap.TagExtractor, repomap.Tag]
  affects: [internal/kernel/edit, internal/daemon]
tech_stack:
  added: []
  patterns: [go:embed .scm queries, shared grammar registry, qualified method names]
key_files:
  created:
    - internal/treesitter/registry.go
    - internal/treesitter/registry_test.go
    - internal/repomap/tags.go
    - internal/repomap/extractor.go
    - internal/repomap/extractor_test.go
    - internal/repomap/queries/go_tags.scm
    - internal/repomap/queries/python_tags.scm
    - internal/repomap/queries/typescript_tags.scm
    - internal/repomap/queries/rust_tags.scm
  modified:
    - internal/kernel/edit/treesitter.go
    - internal/kernel/edit/treesitter_test.go
    - internal/daemon/daemon.go
decisions:
  - Shared GrammarRegistry in internal/treesitter/ used by both edit and repomap packages
  - TSX shares TypeScript .scm query (same syntax constructs)
  - Qualified names via parent-chain walking per language (Go receiver, Python class, TS class, Rust impl)
metrics:
  duration: 4m16s
  completed: 2026-04-16
  tasks_completed: 3
  tasks_total: 3
  files_created: 9
  files_modified: 3
---

# Phase 27 Plan 01: Shared Grammar Registry & Tag Extraction Summary

Tree-sitter shared grammar registry (Go/Python/TS/TSX/Rust) with embedded .scm query tag extraction producing qualified def/ref tags, BodyExtractor refactored to consume shared registry.

## Task Results

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Shared grammar registry and Tag types | f19f8a70 | internal/treesitter/registry.go, internal/repomap/tags.go |
| 2 | Tree-sitter .scm queries and TagExtractor | cef73937 | internal/repomap/extractor.go, queries/*.scm |
| 3 | Refactor BodyExtractor to use shared GrammarRegistry | 078488fa | internal/kernel/edit/treesitter.go, internal/daemon/daemon.go |

## What Was Built

### Shared Grammar Registry (internal/treesitter/)
- `GrammarRegistry` with 5 language grammars: Go, Python, TypeScript, TSX, Rust
- Thread-safe with `sync.RWMutex` for concurrent reads
- `GetLanguage`, `SupportsLanguage`, `SupportedLanguages` methods
- 3 unit tests covering all registered languages

### Tag Types (internal/repomap/tags.go)
- `Tag` struct with Name, Kind, File, Line, Column, StartByte, EndByte per D-01/D-02/D-04
- `TagKind` enum: `TagDef` ("def") and `TagRef` ("ref")

### Tag Extractor (internal/repomap/extractor.go)
- `TagExtractor` compiles .scm queries once per language, reuses across files
- `Extract(source, filePath, lang)` returns `[]Tag` with def/ref tags
- Qualified method names per D-03: `Server.Run` (Go), `Foo.hello` (Python), `App.start` (TS), `Point.new` (Rust)
- `Close()` method releases compiled queries
- 12 unit tests covering all 4 languages + unsupported language error

### Embedded .scm Queries (internal/repomap/queries/)
- `go_tags.scm`: function, method, type defs + call/type refs
- `python_tags.scm`: function, class defs + call refs
- `typescript_tags.scm`: function, class, method, interface defs + call refs
- `rust_tags.scm`: function, struct, enum, trait, impl defs + call refs

### BodyExtractor Refactoring
- Removed duplicate grammar imports from `internal/kernel/edit/treesitter.go`
- `NewBodyExtractor` now takes `*treesitter.GrammarRegistry` parameter
- Daemon bootstrap creates shared registry, passes to BodyExtractor
- All 8 existing edit tests pass with zero regression
- Binary builds successfully

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

```
ok  github.com/postfix/serena/internal/treesitter   0.363s
ok  github.com/postfix/serena/internal/repomap       0.544s
ok  github.com/postfix/serena/internal/kernel/edit    0.292s
go vet: clean
go build ./cmd/serena: clean
```

## Known Stubs

None - all functionality is fully wired.

## Self-Check: PASSED
