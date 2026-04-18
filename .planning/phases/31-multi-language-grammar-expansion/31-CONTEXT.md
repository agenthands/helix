# Phase 31: Multi-language grammar expansion - Context

**Gathered:** 2026-04-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Expand tree-sitter grammar support from the current 4 languages (Go, Python, TypeScript/TSX, Rust) to full aider parity (~25 languages). Add grammar bindings, tag extraction queries (def/ref), and body extraction queries for each language. Shipped in tiered waves — high-value languages first, then the long tail.

</domain>

<decisions>
## Implementation Decisions

### Language Scope
- **D-01:** Full aider parity — target all ~25 languages that have `.scm` queries in the aider reference collections (`borrow/aider/aider/queries/`).
- **D-02:** Tiered waves — Wave 1: Java, C, C++, C#, Ruby, PHP, Kotlin (highest demand). Wave 2: remaining ~18 languages (Dart, Elixir, Elm, Haskell, Julia, Lua, OCaml, Scala, Swift, Zig, R, Clojure, Fortran, etc.).

### Grammar Sourcing
- **D-03:** Official tree-sitter org Go bindings preferred. Community-maintained forks acceptable for languages without official bindings. No CGO required (modernc-style pure Go or pre-built bindings).
- **D-04:** Accept binary size growth — single binary is the core value prop. All grammars compiled in, always available. No build-tag gating.

### Query Coverage
- **D-05:** Both query types for each new language — repomap tag queries (def/ref extraction in `internal/repomap/queries/`) AND edit body queries (symbol body surgery in `internal/kernel/edit/queries/`).
- **D-06:** Merge best of both aider reference collections — compare `tree-sitter-languages` and `tree-sitter-language-pack` per language, use the more complete/accurate queries. Some languages only exist in one collection.

### Testing Strategy
- **D-07:** Both unit and integration tests — fixture file per language with golden expected tags (tree-sitter unit tests), plus MCP integration tests for pipeline validation.
- **D-08:** Wave 1 languages (Java, C, C++, C#, Ruby, PHP, Kotlin) also get LS fixture integration tests. Wave 2 languages get tree-sitter-only test coverage.

### Claude's Discretion
- Exact language list per wave (based on grammar binding availability)
- Query adaptation from aider `.scm` format to Serena's tag/body extraction patterns
- Per-language grammar binding package selection (when multiple community options exist)
- Test fixture content (representative code samples per language)
- Wave 2 ordering within the wave

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Shared Grammar Registry
- `internal/treesitter/registry.go` — Current 4-language grammar registry. New languages are registered here.

### Existing Tag Queries
- `internal/repomap/queries/go_tags.scm` — Go tag query pattern (reference for new languages)
- `internal/repomap/queries/python_tags.scm` — Python tag query pattern
- `internal/repomap/queries/typescript_tags.scm` — TypeScript tag query pattern
- `internal/repomap/queries/rust_tags.scm` — Rust tag query pattern

### Existing Body Queries
- `internal/kernel/edit/queries/go.scm` — Go body extraction pattern (reference for new languages)
- `internal/kernel/edit/queries/python.scm` — Python body extraction pattern
- `internal/kernel/edit/queries/typescript.scm` — TypeScript body extraction pattern
- `internal/kernel/edit/queries/rust.scm` — Rust body extraction pattern

### Aider Reference Queries
- `borrow/aider/aider/queries/tree-sitter-languages/` — Established collection (~27 languages)
- `borrow/aider/aider/queries/tree-sitter-language-pack/` — Newer collection (~31 languages)

### Tag Extractor
- `internal/repomap/` — TagExtractor that consumes tag queries

### Body Extractor
- `internal/kernel/edit/treesitter.go` — BodyExtractor that consumes body queries

### Dependencies
- `go.mod` — Current tree-sitter bindings (go-tree-sitter v0.25.0, per-language grammar packages)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `GrammarRegistry` (`internal/treesitter/registry.go`) — Thread-safe registry with `GetLanguage`, `SupportsLanguage`, `SupportedLanguages`. New languages just need `r.languages["lang"] = tree_sitter.NewLanguage(...)` entries.
- Existing `.scm` query patterns — 4 working examples each for tag and body extraction. New languages follow the same structure.
- Aider reference queries — ~30 languages with pre-written tree-sitter queries for tag extraction.

### Established Patterns
- Grammar initialization via `tree_sitter.NewLanguage()` with per-language binding package imports
- `go:embed` for `.scm` query files
- Tag queries produce `@name` and `@definition.X` / `@reference.X` captures
- Body queries produce captures for function/method/class body boundaries

### Integration Points
- `GrammarRegistry.NewGrammarRegistry()` — add new language initializations
- `internal/repomap/queries/` — add new `{lang}_tags.scm` files
- `internal/kernel/edit/queries/` — add new `{lang}.scm` files
- `go.mod` — add new tree-sitter grammar dependencies

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following the established 4-language pattern and using aider queries as reference.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 31-multi-language-grammar-expansion*
*Context gathered: 2026-04-18*
