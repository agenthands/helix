# Architecture Patterns

**Domain:** Context Intelligence & Resilient Editing for Go MCP Code Intelligence Platform
**Researched:** 2026-04-15

## Recommended Architecture

Two new subsystems integrate into the existing 4-layer architecture as **kernel-level components** (Layer 1), not skills (Layer 2). Both operate on the same data (source files, tree-sitter ASTs) and share the same lifecycle as existing kernel packages.

```
Daemon Bootstrap
  |
  +-- Kernel
  |     +-- lspool/         (existing - worker pool)
  |     +-- symbols/        (existing - 9 retrieval tools)
  |     +-- edit/           (existing - 6 edit tools, MODIFIED for fuzzy fallback)
  |     +-- fileops/        (existing - 6 file tools, MODIFIED for fuzzy replace)
  |     +-- diag/           (existing - 3 diagnostic tools)
  |     +-- repomap/        (NEW - tag extraction, PageRank graph, map rendering)
  |     +-- fuzzy/          (NEW - whitespace-normalized matching, DMP patching)
  |     +-- tagcache/       (NEW - SQLite tag cache with mtime invalidation)
  |     +-- tagger/         (NEW - tree-sitter tag queries, embedded .scm files)
  |
  +-- Skills (unchanged - memory, workflow, profile adapters)
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `internal/kernel/tagger/` | Tree-sitter tag extraction (def/ref) using embedded .scm queries. Language-agnostic query runner. | tagcache (writes tags), repomap (provides tags) |
| `internal/kernel/tagcache/` | SQLite cache for per-file tags with mtime-based invalidation. Separate DB from memory index. | tagger (stores results), repomap (reads cached tags) |
| `internal/kernel/repomap/` | PageRank graph construction, ranking, token-budgeted map rendering. Two MCP tools. | tagger, tagcache, lspool (optional LSP enrichment) |
| `internal/kernel/fuzzy/` | Whitespace-normalized matching, diff-match-patch fuzzy application. Pure functions, no state. | edit (called as fallback), fileops (called as fallback) |

### Data Flow

#### RepoMap Data Flow

```
1. Tool invocation (get_repo_map / get_context)
2. repomap.Builder collects file list from workspace root
3. For each file:
   a. tagcache.Get(path) -- check mtime, return cached if fresh
   b. On cache miss: tagger.Extract(path, lang) -- tree-sitter parse + query
   c. tagcache.Put(path, mtime, tags) -- persist
4. Build MultiDiGraph: files as nodes, def->ref edges with weights
5. Run PageRank with personalization (chat files, mentioned idents)
6. Render ranked tags into token-budgeted tree output
7. Return as MCP tool result
```

#### Fuzzy Edit Data Flow

```
1. Existing tool invoked (replace_symbol_body, replace_content)
2. Exact match attempted first (current behavior)
3. On exact match failure:
   a. fuzzy.NormalizeWhitespace(search, original)
   b. Attempt normalized exact match
   c. On failure: fuzzy.DiffMatchPatch(search, replace, original)
   d. Return result with strategy annotation ("exact" | "normalized" | "fuzzy")
4. Standalone fuzzy_edit tool: always runs full fuzzy pipeline
```

## Integration Decisions

### Q1: Where does the tag/symbol cache live?

**Decision: New separate SQLite database, NOT the existing memory DB.**

Rationale:
- The memory DB (`internal/memory/`) stores user-authored markdown with FTS5 search. Its schema, lifecycle, and watcher are designed for human-written content.
- The tag cache stores machine-generated data (tree-sitter tag extractions) that is fully rebuildable from source. Different schema: `(file_path, mtime, language, tags_blob)` vs memory's `(name, scope, topic, content, ...)`.
- Separate DBs means the tag cache can be blown away without affecting user memories.
- The memory DB uses `modernc.org/sqlite` (CGO-free) -- reuse the same driver, different file.
- Cache location: `{workspace_root}/.serena/tags.db` (project-scoped, gitignored).

Schema:
```sql
CREATE TABLE tags (
    file_path TEXT PRIMARY KEY,
    mtime     REAL NOT NULL,
    language  TEXT NOT NULL,
    tags      BLOB NOT NULL  -- gob-encoded []Tag
);
CREATE INDEX idx_tags_mtime ON tags(mtime);
```

Follow the same patterns as `internal/memory/index.go`: WAL mode, busy_timeout, mutex-protected access.

### Q2: How does the PageRank graph interact with the LSP worker pool?

**Decision: Tree-sitter first, LSP enrichment optional and lazy.**

The PageRank graph is built entirely from tree-sitter tags (definitions and references), NOT from LSP. This is critical because:

1. **LSP workers are expensive.** The pool has adaptive TTL, circuit breaking, and pressure eviction. Scanning hundreds of files through LSP would flood the pool.
2. **Tree-sitter is fast and stateless.** Parsing a file takes microseconds, no server startup, no initialization handshake.
3. **Aider's repomap.py does exactly this.** It uses tree-sitter queries for all tag extraction, with pygments as a fallback for languages where tree-sitter only provides defs (not refs). No LSP involvement.

The LSP pool interaction is limited to:
- **Optional hover enrichment:** When rendering the map, if an LSP worker is already warm (clean lease available without spin-up), we can enrich symbol entries with type signatures from `textDocument/hover`. This is a quality-of-life improvement, not a requirement.
- **The existing `symbols/overview.go` tool** provides LSP-based symbol listing. RepoMap complements it with cross-file importance ranking, not replaces it.

Implementation: `repomap.Builder` takes `*lspool.Pool` as an optional dependency. If nil or if lease acquisition fails/times out (100ms deadline), skip enrichment silently.

### Q3: Should tree-sitter queries (.scm files) be embedded or external?

**Decision: Embedded via `//go:embed`, with runtime override path.**

Rationale:
- Aider ships ~58 `.scm` query files across two directories (31 in tree-sitter-language-pack, 27 in tree-sitter-languages). These are the authoritative tag queries for each language.
- Embedding via `//go:embed` is the Go-native approach (used for the language registry YAML).
- Keeps single-binary distribution constraint satisfied.
- Runtime override: if `{workspace_root}/.serena/queries/{lang}-tags.scm` exists, use it instead. Allows users to customize tag extraction without rebuilding.

The existing `internal/kernel/edit/queries/` directory has 4 `.scm` files for body extraction (different purpose: `@name` + `@body` captures). The tag queries use different capture names (`@name.definition.function`, `@name.reference.call`, etc.). These are separate query sets serving different purposes:

| Query Set | Location | Captures | Purpose |
|-----------|----------|----------|---------|
| Body extraction | `internal/kernel/edit/queries/` | `@name`, `@body` | Precise byte-range for body surgery |
| Tag extraction | `internal/kernel/tagger/queries/` | `@name.definition.*`, `@name.reference.*` | Def/ref identification for graph |

Port the aider `.scm` files from `borrow/aider/aider/queries/tree-sitter-language-pack/` into `internal/kernel/tagger/queries/`. Start with the 4 languages that have tree-sitter grammars compiled in (Go, Python, TypeScript, Rust), expand later.

### Q4: Where does the fuzzy edit logic sit?

**Decision: New `internal/kernel/fuzzy/` package, consumed by both `edit/` and `fileops/`.**

Rationale:
- The fuzzy matching logic is **pure functions** operating on strings. No state, no LSP, no file I/O.
- Both `edit/replace.go` (symbol body replacement) and `fileops/replace.go` (content replacement) need fuzzy fallback.
- Putting it in `edit/` would force `fileops/` to import `edit/` (wrong dependency direction).
- Putting it in `fileops/` would force `edit/` to import `fileops/` (wrong dependency direction).
- A shared `fuzzy/` package at the kernel level is the clean solution.

The `fuzzy/` package provides:
```go
package fuzzy

// MatchResult describes how a match was found.
type MatchResult struct {
    Strategy   string  // "exact", "normalized", "fuzzy"
    NewText    string  // the result after applying replacement
    Confidence float64 // 0.0-1.0, from DMP match quality
}

// NormalizeAndMatch attempts whitespace-normalized exact match.
func NormalizeAndMatch(search, original string) (start, end int, ok bool)

// FuzzyReplace applies search->replace transformation to original using DMP.
func FuzzyReplace(search, replace, original string) (*MatchResult, error)

// FlexibleReplace tries exact, then normalized, then DMP fuzzy matching.
// This is the main entry point for both edit/ and fileops/.
func FlexibleReplace(search, replace, original string) (*MatchResult, error)
```

Integration points:
- `edit/replace.go` `ReplaceBodyWithPlan()`: After tree-sitter body extraction, if the new body doesn't compile, try fuzzy matching the old body against what tree-sitter found.
- `fileops/replace.go` `ReplaceInFile()`: When exact match returns 0 hits, fall back to `fuzzy.FlexibleReplace()`.
- New standalone MCP tool `fuzzy_edit` registered in `edit/tools.go` (or its own file in `edit/`).

### Q5: How to handle cache invalidation?

**Decision: Mtime-based invalidation (like aider), NOT file watcher.**

Rationale:
- Aider's `repomap.py` uses `os.path.getmtime()` -- check mtime on cache read, re-extract on mismatch. Simple, correct, no daemon overhead.
- The memory system uses fsnotify watcher because memories are edited infrequently and the index must be immediately consistent for search. Tags are different: they are queried in batch (hundreds of files per repomap call), and staleness of a few seconds is acceptable.
- File watchers for the entire source tree would be expensive (inotify/kqueue limits, especially on large repos).
- The mtime approach is lazy: only re-extract files that are actually queried AND have changed.
- Matches the existing pattern in `edit/treesitter.go` where tree-sitter parses are done on-demand per file, not cached.

Implementation in `tagcache/`:
```go
func (c *Cache) Get(filePath string) ([]Tag, bool) {
    mtime := getMtime(filePath)
    cached := c.lookup(filePath)
    if cached != nil && cached.Mtime == mtime {
        return cached.Tags, true  // cache hit
    }
    return nil, false  // cache miss, caller should re-extract
}
```

The SQLite cache persists across daemon restarts. On cold start, the first repomap call re-validates mtimes but avoids re-parsing unchanged files. This is the same warm-cache benefit the LSP worker pool provides.

## Patterns to Follow

### Pattern 1: Kernel Tool Registration (existing pattern)

New repomap tools follow the exact same pattern as `symbols/tools.go` and `edit/tools.go`:

```go
// internal/kernel/repomap/tools.go
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, builder *Builder, wsKeyFn func() workspace.WorkspaceKey) {
    tracer := k.Tracer()
    registerGetRepoMap(server, builder, wsKeyFn, tracer)
    registerGetContext(server, builder, wsKeyFn, tracer)
}
```

Registered in daemon bootstrap alongside existing kernel tools:
```go
// daemon.go step 10
repomap.RegisterTools(mcpServer, k, repoBuilder, wsKeyFn)
```

### Pattern 2: Gob-Encoded Cache Values

Tags are serialized as gob-encoded blobs in SQLite, not as individual rows. This keeps the schema simple and avoids N*M row explosion (N files * M tags per file). The cache is an opaque key-value store, not a queryable index.

### Pattern 3: Tiered Matching Strategy

Both aider's `search_replace.py` and our `fuzzy/` package use a strategy cascade:
1. Exact string match (fastest, highest confidence)
2. Whitespace-normalized match (handles indentation drift)
3. DMP fuzzy match (handles LLM output drift)

Each strategy is tried in order; first success wins. The result reports which strategy succeeded for transparency.

Aider's approach in `flexible_search_and_replace` iterates strategy/preprocessing combinations. We simplify: no git cherry-pick (too heavy, requires git), no relative indent preprocessing initially. Just exact -> normalized -> DMP.

### Pattern 4: Token Budget Binary Search

Aider's `get_ranked_tags_map_uncached` uses binary search to fit the map within `max_map_tokens`. Start with an estimate (`max_map_tokens // 25` tags), render, count tokens, adjust bounds. Replicate this approach.

For token counting without a model dependency, use the 4-chars-per-token heuristic (`len(text) / 4`). This is sufficient for budget fitting.

### Pattern 5: Skill Adapter for Profile Filtering

RepoMap tools need to participate in profile/mode filtering. Follow the existing kernel-tool-as-skill-adapter pattern (like `edit/skill.go`, `symbols/skill.go`):

```go
// internal/kernel/repomap/skill.go
func init() {
    skill.Register(&repomapSkill{})
}

type repomapSkill struct{}

func (s *repomapSkill) Name() string        { return "repomap" }
func (s *repomapSkill) Description() string  { return "Repository map and context selection" }
func (s *repomapSkill) Init(deps skill.SkillDeps) error { return nil }
func (s *repomapSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        {Name: "get_repo_map", Description: "..."},
        {Name: "get_context", Description: "..."},
    }
}
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: LSP-First Tag Extraction

**What:** Using the LSP worker pool to extract tags for every file in the repo.
**Why bad:** LSP workers are heavyweight (server process, initialization, memory). Scanning 500 files would require 500 `textDocument/documentSymbol` calls, potentially spinning up and killing workers. Aider explicitly avoids this.
**Instead:** Tree-sitter-first for tags. LSP only for enrichment on already-warm workers.

### Anti-Pattern 2: Shared Database with Memory System

**What:** Adding tag tables to the existing `internal/memory/` SQLite DB.
**Why bad:** Different lifecycles (user content vs machine cache), different invalidation strategies (watcher vs mtime), different schemas. Coupling them means tag cache corruption could lose user memories.
**Instead:** Separate SQLite file per workspace, rebuildable from source.

### Anti-Pattern 3: File Watcher for Tag Cache

**What:** Using fsnotify to watch the entire source tree and invalidate tags on change.
**Why bad:** inotify/kqueue limits (default 8192 on Linux), high overhead on large repos, race conditions with rapid saves, and the daemon already watches memory dirs. Adding another watcher for potentially thousands of source files is expensive.
**Instead:** Mtime-based lazy invalidation on cache read.

### Anti-Pattern 4: Fuzzy Logic in Edit Tool Handlers

**What:** Inlining fuzzy matching logic directly in `edit/tools.go` handlers.
**Why bad:** Creates code duplication when `fileops/replace.go` needs the same logic. Makes the fuzzy logic untestable in isolation. Mixes concerns.
**Instead:** `fuzzy/` package with pure functions, consumed by both `edit/` and `fileops/`.

### Anti-Pattern 5: Full Graph Library Dependency

**What:** Using a full graph library (Go equivalent of networkx) for PageRank.
**Why bad:** The RepoMap graph is a simple weighted MultiDiGraph with one algorithm (PageRank). A full graph library adds dependency weight for no benefit. Aider uses networkx because it is Python-standard; in Go there is no equivalent standard.
**Instead:** Use `github.com/alixaxel/pagerank` (weighted PageRank, ~200 LOC, zero deps) or implement PageRank directly (~50 LOC). The graph construction is specific to our tag data structures anyway.

### Anti-Pattern 6: Git-Based Fuzzy Editing

**What:** Porting aider's `git_cherry_pick_osr_onto_o` strategy that creates temporary git repos.
**Why bad:** Requires git binary, creates temp directories, extremely slow per operation (~100ms+). Aider uses it as a last resort. For an MCP tool called in tight loops, this is unacceptable.
**Instead:** DMP-only fuzzy matching. If DMP fails, report failure and let the agent retry with better input.

## New Dependencies

| Package | Version | Purpose | Why This One |
|---------|---------|---------|-------------|
| `github.com/alixaxel/pagerank` | latest | Weighted PageRank computation | Minimal, zero-dep, weighted edges, ~200 LOC |
| `github.com/sergi/go-diff` | v1.3+ | diff-match-patch for fuzzy editing | Go port of Google's DMP, MIT licensed, mature |
| `modernc.org/sqlite` | (existing) | Tag cache DB | Already in go.mod for memory FTS5 |
| `github.com/tree-sitter/go-tree-sitter` | (existing) | Tag extraction parser | Already in go.mod for body extraction |

No new tree-sitter grammar bindings needed initially -- Go, Python, TypeScript, Rust are already compiled in. Additional grammars can be added incrementally.

## New Files and Modified Files

### New Packages

| Package | Files | Purpose |
|---------|-------|---------|
| `internal/kernel/tagger/` | `tagger.go`, `queries.go`, `tagger_test.go` | Tag extraction with embedded .scm queries |
| `internal/kernel/tagger/queries/` | `go-tags.scm`, `python-tags.scm`, `typescript-tags.scm`, `rust-tags.scm` | Embedded tree-sitter tag queries (ported from aider) |
| `internal/kernel/tagcache/` | `cache.go`, `schema.go`, `cache_test.go` | SQLite tag cache with mtime invalidation |
| `internal/kernel/repomap/` | `builder.go`, `graph.go`, `render.go`, `tools.go`, `skill.go`, `repomap_test.go` | PageRank graph, map rendering, MCP tools |
| `internal/kernel/fuzzy/` | `match.go`, `dmp.go`, `normalize.go`, `fuzzy_test.go` | Fuzzy matching strategies, DMP wrapper |

### Modified Files

| File | Change |
|------|--------|
| `internal/kernel/edit/replace.go` | Add fuzzy fallback in `ReplaceBodyWithPlan()` when exact match fails |
| `internal/kernel/edit/tools.go` | Add `fuzzy_edit` standalone tool registration |
| `internal/kernel/fileops/replace.go` | Add fuzzy fallback in `ReplaceInFile()` when exact match returns 0 |
| `internal/daemon/daemon.go` | Wire tagcache, tagger, repomap builder; register repomap tools (after step 10) |
| `internal/daemon/imports.go` | Add blank import for repomap skill adapter |
| `go.mod` / `go.sum` | Add `alixaxel/pagerank`, `sergi/go-diff` |

## Build Order (Dependency-Driven)

```
Phase 1: Foundation (no deps on each other)
  1a. internal/kernel/fuzzy/       -- pure functions, testable in isolation
  1b. internal/kernel/tagger/      -- tree-sitter queries, needs only go-tree-sitter (existing)

Phase 2: Cache (depends on tagger types)
  2.  internal/kernel/tagcache/    -- SQLite cache, depends on tagger.Tag type

Phase 3: RepoMap (depends on tagger, tagcache)
  3.  internal/kernel/repomap/     -- graph + ranking + rendering + MCP tools

Phase 4: Integration (depends on fuzzy, repomap)
  4a. Modify edit/replace.go       -- fuzzy fallback
  4b. Modify fileops/replace.go    -- fuzzy fallback
  4c. Add fuzzy_edit MCP tool      -- standalone tool
  4d. Wire daemon bootstrap        -- tagcache, repomap builder
```

## Scalability Considerations

| Concern | At 100 files | At 10K files | At 100K files |
|---------|-------------|-------------|--------------|
| Tag extraction | <100ms, all in memory | 1-5s first scan, cached after | 10-30s first scan, SQLite cache critical |
| PageRank | <10ms, trivial graph | 100-500ms, acceptable | 1-5s, may need graph pruning |
| Tag cache DB size | <1MB | 10-50MB | 100-500MB, consider VACUUM schedule |
| Token budget rendering | Instant | Binary search 5-10 iterations | Same, binary search is O(log n) |
| Fuzzy DMP matching | <1ms per match | N/A (per-file, not per-repo) | N/A |

For repos >50K files, consider:
- Gitignore-aware file filtering (exclude `vendor/`, `node_modules/`, etc.)
- Incremental graph updates (re-extract only changed files, rebuild graph)
- Tag cache compaction on daemon startup

## Sources

- [alixaxel/pagerank - Weighted PageRank in Go](https://github.com/alixaxel/pagerank) -- HIGH confidence (direct library)
- [sergi/go-diff - Go port of diff-match-patch](https://github.com/sergi/go-diff) -- HIGH confidence (direct library)
- Aider `repomap.py` (borrow/aider/aider/repomap.py) -- HIGH confidence (direct code read)
- Aider `search_replace.py` (borrow/aider/aider/coders/search_replace.py) -- HIGH confidence (direct code read)
- Existing codebase: `internal/kernel/edit/`, `internal/memory/`, `internal/skill/`, `internal/daemon/daemon.go` -- HIGH confidence (direct code read)
