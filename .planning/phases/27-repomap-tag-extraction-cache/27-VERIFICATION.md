---
phase: 27-repomap-tag-extraction-cache
verified: 2026-04-20T15:30:00Z
status: passed
score: 4/4
overrides_applied: 0
human_verification:
  - test: "Daemon restart preserves tag cache"
    status: PASS
    verified_date: "2026-04-20"
    evidence: "Manual curl test confirmed identical get_repo_map output after daemon restart. Automated tests added: TestScenario_CachePersistence_DaemonRestart (full daemon stack), TestCachePersistence_DaemonRestart (unit level)"
---

# Phase 27: RepoMap Tag Extraction & Cache Verification Report

**Phase Goal:** The system can extract, cache, and elide structural tags (definitions and references) from source files across multiple languages
**Verified:** 2026-04-16T18:30:00Z
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Tree-sitter .scm queries extract def/ref tags from Go, Python, TypeScript, and Rust source files with correct symbol names and locations | VERIFIED | 4 embedded .scm query files, TagExtractor with 12 unit tests across all 4 languages, qualified names confirmed (e.g. `Server.Run` in Go) |
| 2 | Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction, producing compatible tag data | VERIFIED | FallbackExtractor (64 lines) with SymbolRequester interface, 5 unit tests including D-07 constraint (def-only), nested qualified names |
| 3 | Extracted tags persist in SQLite with mtime-based invalidation, surviving daemon restarts and client reconnects without re-extraction of unchanged files | VERIFIED | TagCache (184 lines) with WAL mode, mtime-based invalidation, 7 tests including persistence test (close+reopen verifies tags survive) |
| 4 | Tag output uses scope-aware elision showing signatures without bodies (via tree-sitter), keeping output compact for token-budgeted consumption | VERIFIED | ElisionRenderer (370 lines) with tree-sitter body detection, struct field preservation (D-14), Python colon-style elision, 12 tests across all languages |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/treesitter/registry.go` | Shared grammar registry (D-06) | VERIFIED | 66 lines, 5 grammars (Go/Python/TS/TSX/Rust), thread-safe RWMutex |
| `internal/treesitter/registry_test.go` | Registry tests | VERIFIED | 57 lines, 3 tests |
| `internal/repomap/tags.go` | Tag data model (D-01, D-02, D-04) | VERIFIED | 26 lines, TagDef/TagRef kinds, flat tags with byte offsets |
| `internal/repomap/extractor.go` | Tag extractor with .scm queries | VERIFIED | 273 lines, go:embed queries, qualified names (D-03) |
| `internal/repomap/extractor_test.go` | Extractor tests | VERIFIED | 233 lines, 12 tests covering 4 languages |
| `internal/repomap/queries/go_tags.scm` | Go tree-sitter query | VERIFIED | Exists, embedded via go:embed |
| `internal/repomap/queries/python_tags.scm` | Python tree-sitter query | VERIFIED | Exists, embedded via go:embed |
| `internal/repomap/queries/typescript_tags.scm` | TypeScript tree-sitter query | VERIFIED | Exists, embedded via go:embed |
| `internal/repomap/queries/rust_tags.scm` | Rust tree-sitter query | VERIFIED | Exists, embedded via go:embed |
| `internal/repomap/fallback.go` | LSP documentSymbol fallback (D-07) | VERIFIED | 64 lines, SymbolRequester interface, def-only tags |
| `internal/repomap/fallback_test.go` | Fallback tests | VERIFIED | 176 lines, 5 tests |
| `internal/repomap/schema.go` | SQLite schema (D-09) | VERIFIED | 19 lines, file_tags table with kind CHECK constraint |
| `internal/repomap/cache.go` | SQLite tag cache (D-09, D-10, D-11, D-12) | VERIFIED | 184 lines, WAL mode, mtime invalidation, lazy warming |
| `internal/repomap/cache_test.go` | Cache tests | VERIFIED | 244 lines, 7 tests including persistence |
| `internal/repomap/elide.go` | Scope-aware elision (D-13, D-14, D-15) | VERIFIED | 370 lines, render-time elision, struct fields shown |
| `internal/repomap/elide_test.go` | Elision tests | VERIFIED | 196 lines, 12 tests across all languages |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/kernel/edit/treesitter.go` | `internal/treesitter/` | `treesitter.GrammarRegistry` field | WIRED | BodyExtractor takes `*treesitter.GrammarRegistry`, no longer imports grammar bindings directly |
| `internal/repomap/extractor.go` | `internal/treesitter/` | `treesitter.GrammarRegistry` constructor param | WIRED | `NewTagExtractor(registry)` |
| `internal/repomap/elide.go` | `internal/treesitter/` | `treesitter.GrammarRegistry` constructor param | WIRED | `NewElisionRenderer(registry)` |
| `internal/daemon/daemon.go` | `internal/treesitter/` | `treesitter.NewGrammarRegistry()` | WIRED | Line 190: daemon creates shared registry, passes to BodyExtractor |
| `internal/repomap/extractor.go` | `internal/repomap/queries/*.scm` | `go:embed` directives | WIRED | 4 embed directives for Go/Python/TS/Rust queries |
| `internal/repomap/cache.go` | `internal/repomap/schema.go` | Schema creation SQL | WIRED | NewTagCache executes schema DDL on open |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All repomap tests pass | `go test ./internal/repomap/... -count=1` | 32 tests PASS, 0.577s | PASS |
| All treesitter tests pass | `go test ./internal/treesitter/... -count=1` | PASS, 0.286s | PASS |
| All edit tests pass (no regression) | `go test ./internal/kernel/edit/... -count=1` | PASS, 0.295s | PASS |
| go vet clean | `go vet ./internal/treesitter/... ./internal/repomap/... ./internal/kernel/edit/...` | No output (clean) | PASS |
| Binary builds | `go build ./cmd/serena` | Exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-----------|-------------|--------|----------|
| RMAP-01 | 27-01 | Agent can extract def/ref tags from source files via tree-sitter .scm queries (Go, Python, TypeScript, Rust) | SATISFIED | TagExtractor with 4 embedded .scm queries, 12 unit tests, qualified names |
| RMAP-02 | 27-02 | Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction | SATISFIED | FallbackExtractor with SymbolRequester interface, 5 unit tests, D-07 enforced |
| RMAP-03 | 27-03 | Tag cache persists in SQLite with mtime-based invalidation, surviving client reconnects via daemon lifecycle | SATISFIED | TagCache with WAL, mtime checks, persistence test (close+reopen), parameterized queries |
| RMAP-09 | 27-04 | Output uses scope-aware elision (signatures without bodies via tree-sitter) | SATISFIED | ElisionRenderer with body detection, struct fields preserved, 12 tests |

### Decision Verification

| Decision | Description | Status | Evidence |
|----------|-------------|--------|----------|
| D-01 | Two tag kinds: def and ref | VERIFIED | `TagDef = "def"`, `TagRef = "ref"` in tags.go |
| D-02 | Flat tags, no scope nesting | VERIFIED | Tag struct has no parent/scope fields |
| D-03 | Qualified names with receiver/owner | VERIFIED | Tests confirm `Server.Run`, `Foo.hello`, `App.start`, `Point.new` |
| D-04 | Byte offsets only, no raw text | VERIFIED | Tag has StartByte/EndByte, no text field |
| D-05 | Embedded .scm query files | VERIFIED | 4 go:embed directives in extractor.go |
| D-06 | Shared grammar registry | VERIFIED | `internal/treesitter/` package, used by edit and repomap |
| D-07 | LSP fallback maps as def only | VERIFIED | FallbackExtractor + test explicitly validates no TagRef produced |
| D-08 | Both defs and refs from tree-sitter | VERIFIED | .scm queries contain both def and ref captures |
| D-09 | Separate SQLite at tags.db | VERIFIED | NewTagCache(dbPath) independent from memory store |
| D-10 | File-level mtime invalidation | VERIFIED | GetOrExtract checks mtime, re-extracts on mismatch |
| D-11 | Cache in project .serena/ directory | VERIFIED | dbPath parameter, daemon controls placement |
| D-12 | Lazy cache warming | VERIFIED | GetOrExtract on first access, no eager scan |
| D-13 | Signature + ellipsis format | VERIFIED | ElisionRenderer replaces body with ellipsis marker |
| D-14 | Struct fields shown, method bodies elided | VERIFIED | TestRenderFile_GoStruct confirms fields shown |
| D-15 | Elision at render time, not extraction | VERIFIED | Cache stores byte ranges, ElisionRenderer reads source at render time |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | No TODOs, FIXMEs, placeholders, or stubs found | - | - |

### Human Verification — RESOLVED

### 1. Daemon Restart Preserves Tag Cache — PASS

**Verified:** 2026-04-20 (manual + automated)
**Manual test:** Started daemon, called activate_project + get_repo_map via curl, stopped daemon, restarted, repeated — identical output from SQLite cache.
**Automated tests added:**
- `TestScenario_CachePersistence_DaemonRestart` (test/oracle/scenario/cache_persistence_test.go) — full daemon stack via harness.StartRunner + MCP protocol
- `TestCachePersistence_DaemonRestart` (internal/skill/repomap/cache_persistence_test.go) — unit level, verifies zero re-extractions on second session
- `TestCachePersistence_ModifiedFileReextracts` — verifies only modified files re-extract after restart

### Gaps Summary

No gaps found. All 4 success criteria verified. All 15 implementation decisions implemented. All 4 requirements (RMAP-01, RMAP-02, RMAP-03, RMAP-09) satisfied. The daemon restart cache persistence item is now fully automated.

---

_Verified: 2026-04-16T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
