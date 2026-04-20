---
phase: 33-fallback-extractor-wiring
created: 2026-04-20
---

# Phase 33: FallbackExtractor Wiring & Cache Persistence — Context

## Domain Boundary

Wire the existing FallbackExtractor into the production pipeline so languages without tree-sitter grammars get LSP-based tag extraction via documentSymbol, and verify that the SQLite TagCache survives daemon restarts.

## Decisions

### D-33-01: Fallback trigger logic — Registry-based check

`walkAndExtract` checks whether a language has a tree-sitter grammar registered in the GrammarRegistry. If it does NOT, and an LSP server is available for that language, the FallbackExtractor is invoked. This is a proactive/deterministic approach (language registry lookup) rather than reactive (waiting for tree-sitter to error).

**Rationale:** Matches the existing `LangFromExt` pattern. All 23 current languages have grammars, so fallback only fires for future languages added without tree-sitter support.

**Claude's Discretion:** Implementation details of registry lookup (method name, caching of the check result).

### D-33-02: LSP availability handling — Log warning + silent skip

When FallbackExtractor needs an LSP connection but no language server is available/installed for that language:
- Log at debug level (same pattern as tree-sitter skip at `skill.go:318`)
- Return empty tags (file produces no tags for the graph)
- Do NOT surface errors to the agent or user

**Rationale:** Consistent with existing error-tolerant walk behavior. A missing LS for an obscure language should not break the entire walk. Observability is available via debug logs.

### D-33-03: Cache persistence test — Unit-level SQLite reopen test

Verify cache persistence via a unit test that:
1. Creates a TagCache at a temp path
2. Inserts tags via GetOrExtract
3. Closes the TagCache
4. Reopens a new TagCache at the same path
5. Verifies tags are present without calling the extract function

**Rationale:** Fast, deterministic, runs in CI. No daemon lifecycle required. The existing `cache_test.go` pattern supports this. The TagCache is already SQLite-backed with WAL mode — persistence is a property of SQLite, not the daemon.

### D-33-04: SymbolRequester production implementor — WorkerLease adapter

The `SymbolRequester` interface is satisfied by wrapping a `WorkerLease` from `lspool`. The adapter's `Request` method delegates to the lease's existing `Request` method. FallbackExtractor receives the adapter during `walkAndExtract` when a lease is available for the file's language.

**Claude's Discretion:** Whether the adapter is a named type or a closure, and how lease acquisition/release is managed within the walk loop.

## Canonical Refs

- `internal/repomap/fallback.go` — FallbackExtractor implementation
- `internal/repomap/fallback_test.go` — Existing tests (5 cases)
- `internal/repomap/cache.go` — TagCache SQLite-backed implementation
- `internal/skill/repomap/skill.go:288-331` — walkAndExtract (integration point)
- `internal/kernel/lspool/` — Worker pool providing LSP connections
- `.planning/v1.6-MILESTONE-AUDIT.md` — Gap identification source

## Prior Decisions Applied

- Phase 28 D-07: FallbackExtractor produces def-only tags (no references)
- Phase 27 D-10: File-level mtime invalidation
- Phase 27 D-12: Lazy extraction (no eager warming)
- Phase 31: Error-tolerant tag query compilation (log+skip pattern)

## Deferred Ideas

None.
