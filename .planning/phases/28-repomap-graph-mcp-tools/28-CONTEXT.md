# Phase 28: RepoMap Graph & MCP Tools - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a cross-file reference graph from Phase 27's extracted tags, rank symbols/files via Personalized PageRank, and expose two token-budgeted MCP tools (`get_repo_map`, `get_context`). LSP sessions opportunistically enrich the graph with precise cross-file references. Phase 27 provides the tag extraction, cache, and elision layers that this phase consumes.

</domain>

<decisions>
## Implementation Decisions

### Graph Model
- **D-01:** File-level graph — nodes represent files, edges represent cross-file ref-to-def links between files. Simpler graph, faster PageRank, matches aider's approach. Symbol detail comes from elision at render time.
- **D-02:** In-memory rebuild — build graph from SQLite-cached tags on first `get_repo_map` call. No persistence of graph edges. Fast enough with cached tags. Matches Phase 27's D-12 lazy warming pattern.
- **D-03:** Edge weighting by reference count — files with more cross-references to a target file produce a higher-weight edge. This feeds into PageRank for importance scoring.

### PageRank & Ranking
- **D-04:** Personalized PageRank with seed files — `get_context` personalizes the PageRank teleportation vector by seeding from the files/symbols the agent specifies. Files mentioned in the task get higher teleportation probability.
- **D-05:** Standard (uniform) PageRank for `get_repo_map` — no personalization when showing the full repo overview. Files ranked purely by structural importance.

### MCP Tool Design
- **D-06:** Skill tools — register as a `ToolProvider` skill in `internal/skill/repomap/`. Follows the memory skill pattern. Clean separation — repomap is an optional capability, not core kernel.
- **D-07:** `get_repo_map` returns an elided source tree — file paths as a tree structure with elided symbols nested under each file. Matches aider's repo-map format. Agents can scan structure at a glance.
- **D-08:** `get_context` accepts file paths (primary input for PageRank seeding) plus an optional `task_description` string. Phase 28 uses only file-based personalization; task description is accepted but reserved for future semantic matching.
- **D-09:** Token budget parameter on both tools — integer controlling maximum output size. Required parameter with a reasonable default (e.g., 4096 tokens).

### LSP Enrichment
- **D-10:** Opportunistic enrichment — when an LSP worker lease is already active for a file (warm session), piggyback `textDocument/references` calls. Don't start new LS sessions just for enrichment. This keeps enrichment cost-free in the common case.
- **D-11:** Additive edges — LSP references add new cross-file edges to the graph that tree-sitter couldn't detect (e.g., interface implementations, cross-package calls). Never replace tree-sitter tags. Graph gets richer over time as more LS sessions warm up.

### Token Budgeting
- **D-12:** Character-based token estimation — approximate tokens as chars/4. Fast, no external dependency. Good enough for budget control since exact token counts aren't needed when the goal is "fit within N tokens."
- **D-13:** Prune lowest-ranked files first — binary search on the file count: include top-N files by PageRank score until output fits within budget. Simple, deterministic, respects the ranking.

### Claude's Discretion
- Graph rebuild caching strategy (e.g., dirty-flag to avoid rebuilding when tags haven't changed)
- PageRank convergence parameters (damping factor, iterations, epsilon)
- Tool parameter validation and error response format
- LSP enrichment batching and rate limiting
- File tree rendering format details (indentation, path compression)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 27 Output (consumed by this phase)
- `internal/repomap/tags.go` — Tag struct, TagKind enum (TagDef, TagRef)
- `internal/repomap/extractor.go` — TagExtractor with Extract(source, filePath, lang) method
- `internal/repomap/fallback.go` — FallbackExtractor with SymbolRequester interface for LSP fallback
- `internal/repomap/cache.go` — TagCache with GetOrExtract(filePath, extractFn) and SQLite persistence
- `internal/repomap/elide.go` — ElisionRenderer with RenderFile(source, lang, tags) for scope-aware output
- `internal/treesitter/registry.go` — Shared GrammarRegistry (Go/Python/TS/TSX/Rust)

### Skill System (tool registration pattern)
- `internal/skill/skill.go` — Skill, ToolProvider, WorkflowProvider interfaces
- `internal/skill/memory/skill.go` — Memory skill as reference implementation for a ToolProvider with store + SQLite
- `internal/daemon/daemon.go` — Daemon bootstrap, skill initialization, tool registration

### LSP Infrastructure (for enrichment)
- `internal/kernel/lspool/adapter.go` — WorkerLease with Request method (satisfies SymbolRequester)
- `internal/kernel/symbols/overview.go` — Existing documentSymbol integration pattern

### Dependencies
- `go.mod` — modernc.org/sqlite (for tag cache), tree-sitter bindings

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TagCache.GetOrExtract()` — bulk tag retrieval for graph building; iterate all cached files
- `ElisionRenderer.RenderFile()` — render elided output for each file in the repo map
- `TagExtractor` + `FallbackExtractor` — extract tags for uncached files during graph build
- `SymbolRequester` interface — used by FallbackExtractor, same pattern for LSP enrichment requests
- `skill.Register()` + `skill.InitAll()` — Caddy-style init registration for new repomap skill

### Established Patterns
- Skill tools return `[]*mcp.ToolDef` via `Tools()` method
- Skills receive `SkillDeps` with ProjectDir, GlobalDir, Logger
- Daemon blank-imports skill packages in `imports.go` for init() registration
- SQLite via modernc.org/sqlite — CGO-free, WAL mode, busy timeout

### Integration Points
- New `internal/skill/repomap/` package registered via init() in skill system
- Daemon bootstrap passes shared GrammarRegistry + TagCache to repomap skill
- LSP enrichment hooks into existing lspool WorkerLease lifecycle
- Profile filtering middleware controls tool visibility per agent profile

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following the aider-style repo map pattern.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 28-repomap-graph-mcp-tools*
*Context gathered: 2026-04-16*
