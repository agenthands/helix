# Phase 27: RepoMap Tag Extraction & Cache - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a tree-sitter-based tag extraction system that produces def/ref tags from source files, with LSP documentSymbol fallback for unsupported languages. Tags are cached in SQLite with mtime-based invalidation. Output supports scope-aware elision (signatures without bodies). Phase 28 consumes this to build reference graphs and MCP tools.

</domain>

<decisions>
## Implementation Decisions

### Tag Data Model
- **D-01:** Two tag kinds only: `def` (definition) and `ref` (reference). No finer taxonomy.
- **D-02:** Flat tags — name + kind + file + line + column. No scope nesting or parent chains.
- **D-03:** Qualified names when available — methods include receiver/owner (e.g., `Server.Run` in Go), free functions use bare name. Tree-sitter extracts this per language.
- **D-04:** Byte offsets only (StartByte/EndByte) — no raw text stored in tags. Elided text derived at render time by reading the source file.

### Tree-sitter Query Strategy
- **D-05:** Embedded `.scm` query files per language via `go:embed`. Aider-style pattern. Easy to read, test, extend per language.
- **D-06:** Shared grammar registry — extract grammar/language mapping from `BodyExtractor` into a shared package (e.g., `internal/treesitter/`). Both `edit` and `repomap` import it.
- **D-07:** LSP fallback maps all `documentSymbol` results as `def` tags. References come only from tree-sitter queries or Phase 28's LSP enrichment.
- **D-08:** Both defs and refs extracted in Phase 27 via tree-sitter `.scm` queries. Phase 28 enriches with precise cross-file references from warm LSP sessions.

### Cache & Invalidation
- **D-09:** Separate SQLite database at `.serena/tags.db`. Independent lifecycle from memory store. Can be deleted/rebuilt without affecting memories.
- **D-10:** File-level mtime invalidation. Track file path + mtime. If mtime changed, re-extract all tags for that file. Matches RMAP-03 requirement.
- **D-11:** Cache file lives in project `.serena/` directory alongside project config. Per-project, survives daemon restarts, user can gitignore.
- **D-12:** Lazy cache warming — extract tags on first access, not on project activation. Cache builds up organically. Phase 28's `get_repo_map` tool triggers bulk extraction.

### Elision Format
- **D-13:** Signature + ellipsis format — full signature line with `⋯` replacing the body. E.g., `func (s *Server) Run(ctx context.Context) error { ⋯ }`.
- **D-14:** Struct/class fields shown, method bodies elided — fields are part of the type's "signature". Agents need type shapes for context.
- **D-15:** Elision performed at render time, not extraction time. Cache stores byte ranges only. Allows different elision levels per Phase 28 query. Keeps cache schema simple.

### Claude's Discretion
- Tree-sitter `.scm` query specifics per language (which node types to match for defs/refs)
- SQLite schema column details (indexes, constraints, WAL mode)
- Shared grammar registry package structure and API
- Error handling for corrupt cache, missing grammars, parse failures

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Tree-sitter Code
- `internal/kernel/edit/treesitter.go` — Existing BodyExtractor with Go/Python/TS/Rust grammar setup. Source for shared grammar extraction (D-06).

### Existing SQLite Usage
- `internal/memory/index.go` — FTS5 SQLite usage pattern with modernc.org/sqlite (CGO-free). Reference for cache DB setup.

### Language Registry
- `internal/langregistry/registry.go` — 52-language registry. Used to determine which languages have tree-sitter grammars vs need LSP fallback.
- `internal/langregistry/languages.go` — Language entries with metadata.

### LSP DocumentSymbol
- `internal/kernel/symbols/overview.go` — Existing documentSymbol integration. Reference for LSP fallback path (D-07).
- `internal/kernel/lspool/adapter.go` — LSP adapter with documentSymbol support.

### Dependencies
- `go.mod` — Tree-sitter bindings: go-tree-sitter v0.25.0, tree-sitter-go v0.25.0, tree-sitter-python v0.25.0, tree-sitter-rust v0.24.2, tree-sitter-typescript v0.23.2

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `BodyExtractor` (`internal/kernel/edit/treesitter.go`) — Grammar initialization for Go, Python, TS, Rust. Will be extracted into shared registry per D-06.
- `memory/index.go` — SQLite database setup pattern with modernc.org/sqlite. Reusable for tags.db.
- `langregistry` — Language detection and metadata. Determines tree-sitter vs LSP fallback per language.

### Established Patterns
- Tree-sitter grammars loaded via `tree_sitter.NewLanguage()` with per-language binding packages
- SQLite via modernc.org/sqlite (CGO-free) — no external C dependencies
- Language detection via file extension through `langregistry`

### Integration Points
- Shared grammar registry will be imported by both `internal/kernel/edit/` and new `internal/repomap/` package
- Tag cache will be initialized by daemon bootstrap alongside memory store
- LSP fallback uses existing `lspool` worker pool for documentSymbol requests

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following the aider-style tag extraction pattern.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 27-repomap-tag-extraction-cache*
*Context gathered: 2026-04-16*
