# Feature Landscape

**Domain:** Codebase Context Intelligence & Resilient Editing for MCP Code Intelligence Platform
**Researched:** 2026-04-15
**Scope:** Only features needed for the v1.6 milestone. Existing v1.5 infrastructure (38+ MCP tools, 9 symbol retrieval, 6 symbol editing with tree-sitter body surgery, 6 file ops, 3 diagnostics, memory system, profiles/modes, typed error taxonomy) is the baseline. This milestone adds RepoMap context intelligence and fuzzy edit resilience.

## Table Stakes

Features agents expect from a context intelligence / edit platform. Missing = agents fall back to brute-force file reading and fragile exact-match edits.

### RepoMap / Codebase Context

| Feature | Why Expected | Complexity | Dependencies (existing) | Notes |
|---------|--------------|------------|------------------------|-------|
| Structural overview (file -> symbols with signatures) | Aider proved this is the baseline expectation. Every coding agent needs a condensed view of what exists in a repo without reading every file. | Medium | `GetSymbolOverview` (LSP documentSymbol), tree-sitter grammars (Go/Python/TS/Rust) | Serena has per-file symbol overview. Gap is repo-wide aggregation with cross-file awareness |
| Token budget control | Agents operate under strict context limits. Map must fit in N tokens. Aider defaults to 1K tokens, scales to 8K when no files selected. | Low | Token budget reporting already exists (`get_token_budget` tool) | Must be a tool parameter, not hardcoded. Agents need per-call budget control |
| Ranked symbol importance | Without ranking, agents get alphabetical file dumps wasting context on leaf helpers. Aider's PageRank finds transitively-important symbols. Cursor uses embedding similarity. | High | Cross-file reference graph (new), graph ranking algorithm (new) | Core differentiator vs. `tree .` output. PageRank on def/ref graph is the proven approach (no embedding model needed) |
| Incremental caching with mtime invalidation | Re-parsing entire repo on every call is too slow. Aider uses diskcache with mtime. Continue uses SQLite with tag_catalog. | Medium | modernc.org/sqlite (already a dependency), daemon lifecycle for cache persistence | Daemon architecture gives natural cross-session cache lifetime. Aider's cache is per-session |
| Multi-language support | Serena targets 52 languages. RepoMap must work across all, not just 4 tree-sitter languages. | Medium | 52-language registry, LSP documentSymbol as fallback | Tree-sitter for fast path (languages with grammars + tags.scm queries), LSP documentSymbol for the rest |

### Fuzzy / Resilient Editing

| Feature | Why Expected | Complexity | Dependencies (existing) | Notes |
|---------|--------------|------------|------------------------|-------|
| Whitespace-normalized matching | LLMs routinely produce search blocks with wrong indentation. Every mature tool handles this (Aider, RooCode, Claude Code). | Low | None beyond string processing | Strip/normalize leading whitespace and re-compare. Most common LLM failure mode |
| Multi-strategy fallback cascade | Exact -> whitespace-normalized -> fuzzy. Single-strategy systems fail 10-30% of the time on real LLM output. | Medium | Similarity algorithm (new) | Aider has 4 strategies (exact, whitespace, dotdotdots, fuzzy-disabled). RooCode has 9 strategies with Levenshtein. Start with 3-4 core strategies |
| Indentation preservation on replacement | When fuzzy match succeeds, replacement must adopt the original file's indentation, not the LLM's. | Medium | Whitespace analysis of matched region | Critical for Python/YAML where indentation is semantic. RooCode captures original indent and re-applies relative structure |
| Actionable error messages on match failure | When all strategies fail, return the closest match with similarity score and surrounding context. | Low | Similarity scoring from fuzzy matcher | Aider shows "did you mean this?" with context. Without this, agents retry blindly with the same broken search block |
| Match uniqueness validation | If search text matches multiple locations, refuse the edit (ambiguous). Agent must provide more context. | Low | None | Prevents silent wrong-location edits. Every mature tool enforces this |

## Differentiators

Features that set Serena apart. Not expected by agents, but high-value when present.

### RepoMap / Context Intelligence

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| Hybrid tree-sitter + LSP data source | Tree-sitter for fast structural extraction, LSP for semantic enrichment (type info, cross-file references) when the worker pool has warm sessions. No other tool combines both. | High | Existing LSP worker pool, existing tree-sitter BodyExtractor infrastructure | Aider is tree-sitter only. Cursor is embeddings only. Cody is search-API only. Serena can be both structural + semantic |
| Task-focused context selection tool | Given a task description + file set, return the most relevant symbols ranked by relevance to that task. Personalized PageRank weighted toward the task's file set. | High | RepoMap graph + personalization weights | Aider does this via chat_fnames personalization. Expose as explicit MCP tool parameter. Separate tool from overview |
| LSP-enriched reference graph | Use textDocument/references from warm LSP sessions to build more accurate cross-file edges than tree-sitter identifier matching alone. | High | Warm LSP pool sessions, existing reference resolution | Tree-sitter refs are approximate (name matching). LSP refs are precise (semantic). Use LSP when available, tree-sitter as baseline |
| Daemon-persistent cache | RepoMap cache lives as long as the daemon, surviving client reconnects. First call builds, subsequent calls get instant results. | Low | Existing daemon lifecycle | Aider's cache is per-session (diskcache). Cursor requires re-indexing. Serena's daemon gives free cross-session persistence |
| Scope-aware elided output | Show file structure with class/function signatures but elide bodies, using tree-sitter to determine exact scope boundaries. | Medium | Tree-sitter AST navigation | grep-ast style: show the "shape" of code without the bulk. Continue's repo-map provider does similar AST-based truncation |

### Fuzzy Editing

| Feature | Value Proposition | Complexity | Dependencies | Notes |
|---------|-------------------|------------|--------------|-------|
| Strategy reporting in tool response | Tell the agent which matching strategy succeeded (exact, whitespace-normalized, fuzzy@0.92). Builds agent trust and helps it calibrate future edits. | Low | Fuzzy matcher cascade | No other MCP tool reports this. Agents can learn to provide better search blocks |
| Symbol-aware fuzzy matching | For replace_symbol_body: use tree-sitter to locate the symbol first, then fuzzy-match within its body. Bounded search = fewer false positives. | Medium | Existing BodyExtractor, fuzzy matcher | Combines Serena's existing tree-sitter body extraction with fuzzy matching. Unique to Serena |
| Configurable similarity threshold | Let agents control the fuzzy threshold (default 0.8, range 0.6-1.0). Conservative agents use 0.95, aggressive agents use 0.7. | Low | Fuzzy matcher | RooCode does this (default 1.0, configurable down). Most tools hardcode the threshold |
| Ellipsis/placeholder support | LLMs use `...` to indicate "unchanged code here". Parse and handle this in search blocks. | Medium | Block splitter, per-chunk matching | Aider implements try_dotdotdots(). Reduces token waste when agents only show changed portions |
| Standalone fuzzy_edit MCP tool | Separate from existing symbol-aware tools. Raw text matching for when agents don't know or care about symbol boundaries. | Medium | Fuzzy matcher (shared with existing tools) | Complements replace_symbol_body (symbol-aware) with a text-level fallback. Lower barrier to use |

## Anti-Features

Features to explicitly NOT build. Each has a reason tied to Serena's architecture or project constraints.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Embedding/vector-based semantic search | Requires embedding model, vector DB, GPU or API dependency. Out of scope per PROJECT.md ("Augment Context Engine does this better"). Cursor's approach needs cloud infrastructure. | Structural graph ranking (PageRank on def/ref graph). Works offline, no model dependency, deterministic |
| Knowledge graph / code graph | Explicit out-of-scope per PROJECT.md ("CodeGraphContext/GitNexus own this space"). | Symbol reference graph for ranking only. Not a queryable knowledge graph |
| Git-aware context (blame, history, commit messages) | Out of scope per PROJECT.md ("GitHub MCP Server handles git comprehensively"). | RepoMap works on current file state only. Agents compose with git tools |
| Natural language query over codebase | Requires embeddings or LLM-in-the-loop for retrieval. Serena is a tool provider, not a retrieval agent. | Structured tools: overview, search, references. Agent composes these |
| Line-number-based edit addressing | Fragile. Lines shift between when agent reads file and when it edits. | Content-based matching (search text) and symbol-based matching (symbol name + body). Both are stable across reads |
| apply_patch / unified diff format | Model-specific. GPT-5 codex models are trained on apply_patch; Claude is trained on old_string/new_string. Serena serves all agents via MCP. | search/replace with fuzzy fallback. Model-agnostic format that works for any LLM |
| Full AST-aware diffing | Massive complexity for marginal gain over text-level fuzzy matching. Academic interest, not practical for MCP tools. | Tree-sitter for symbol location + body extraction. Text-level matching within located regions |
| Multi-repository RepoMap | Cross-repo analysis adds massive complexity. Sourcegraph/Cody handles this at enterprise scale. | Single workspace scope. Agents can call RepoMap per-workspace if needed |

## Feature Dependencies

```
Tree-sitter tag extraction (new: def/ref tags)
  |-- Uses: existing tree-sitter infrastructure (BodyExtractor, 4 language grammars)
  |-- Needs: tags.scm query files per language (new)
  |-- Fallback: LSP documentSymbol (existing GetSymbolOverview)
  v
SQLite tag cache (new)
  |-- Uses: existing modernc.org/sqlite dependency
  |-- Uses: daemon lifecycle for persistence
  v
Cross-file reference graph (new)
  |-- Built from: tag extraction (def/ref pairs)
  |-- Enriched by: LSP textDocument/references (existing, optional)
  v
PageRank ranking algorithm (new)
  |-- Input: reference graph
  |-- Personalization: task files, mentioned symbols
  v
Token-budgeted output formatter (new)
  |-- Input: ranked symbols
  |-- Uses: tree-sitter for scope-aware elision
  v
RepoMap overview MCP tool (new)
  |-- Aggregates: all above
  |-- Parameters: workspace, token_budget
  v
RepoMap context selection MCP tool (new)
  |-- Extends: overview with personalization weights
  |-- Parameters: workspace, token_budget, task_files, task_description

---

Fuzzy matcher library (new, independent of RepoMap)
  |-- Strategies: exact, whitespace-normalized, indentation-flexible, Levenshtein
  |-- Includes: similarity scoring, indentation preserver, strategy reporter
  v
Integration into replace_symbol_body (augment existing)
  |-- Uses: existing BodyExtractor for symbol location
  |-- Adds: fuzzy matching within located body
  v
Integration into replace_content (augment existing)
  |-- Uses: existing ReplaceInFile
  |-- Adds: fuzzy fallback when exact/regex match fails
  v
Standalone fuzzy_edit MCP tool (new)
  |-- Uses: fuzzy matcher library
  |-- Registered: via existing skill/tool pattern
```

## MVP Recommendation

### Phase 1: Fuzzy Editing Foundation
Build the fuzzy matching library first. Lower complexity than RepoMap, immediately useful in existing tools, and unblocks resilient editing across the board.

Prioritize:
1. **Fuzzy matcher with 4-strategy cascade** - exact, whitespace-normalized, indentation-flexible, Levenshtein fuzzy (threshold 0.8). Well-understood algorithms, pure Go, no external dependencies
2. **Indentation preservation** - capture original indent, compute relative indent from search/replace blocks, re-apply. Critical for Python/YAML correctness
3. **Integration into replace_symbol_body and replace_content** - augment existing tools with fuzzy fallback. Clean integration points already exist in `edit/replace.go` and `fileops/replace.go`
4. **Strategy reporting** - include `match_strategy` and `similarity_score` in tool response JSON
5. **Standalone fuzzy_edit MCP tool** - expose raw fuzzy matching as its own tool. Follows existing `skill.Register` + `ToolProvider` pattern

Defer: Ellipsis/placeholder support (medium complexity, add after core cascade is solid)

### Phase 2: RepoMap Core
Build the structural map with ranking.

Prioritize:
1. **Tree-sitter tag extraction** (def/ref) - extend existing tree-sitter infrastructure. Need tags.scm query files for Go/Python/TS/Rust (4 languages with existing grammars)
2. **LSP documentSymbol fallback** - for the other 48 languages. Existing `GetSymbolOverview` provides the data, just need aggregation
3. **SQLite tag cache with mtime invalidation** - leverage existing sqlite dependency. Store (file_path, mtime, tags_json)
4. **Cross-file reference graph** - build directed graph from extracted tags. Nodes = files, edges = ref-file -> def-file
5. **PageRank ranking** - implement personalized PageRank in pure Go (well-documented algorithm, ~100-200 lines)
6. **Token-budgeted overview output** - render ranked symbols within budget. Iterate ranked files, accumulate tokens, stop at budget
7. **RepoMap overview MCP tool** - expose as tool with `workspace`, `token_budget` parameters

Defer: LSP reference enrichment (depends on warm pool state, add as optimization in Phase 3)

### Phase 3: Context Selection & Enrichment
Build task-focused context selection on top of the map.

Prioritize:
1. **Task-focused context selection tool** - personalized PageRank weighted by task files/symbols. Separate MCP tool from overview
2. **LSP reference enrichment** - use warm LSP sessions to improve graph accuracy when available
3. **Scope-aware elided output** - show signatures without bodies using tree-sitter scope navigation
4. **Ellipsis support in fuzzy edits** - add dotdotdots handling to the fuzzy matcher

## Complexity Summary

| Feature | Complexity | Effort (days) | Risk | Confidence |
|---------|------------|---------------|------|------------|
| Fuzzy matcher cascade (4 strategies) | Medium | 2-3 | Low - well-understood algorithms, pure Go | HIGH |
| Indentation preservation | Medium | 1-2 | Medium - edge cases in mixed tabs/spaces, Python semantics | HIGH |
| Fuzzy integration into existing tools | Low | 1 | Low - clean integration points exist (`replace.go`, `fileops/replace.go`) | HIGH |
| Standalone fuzzy_edit tool | Low | 1 | Low - follows existing `skill.Register` + `ToolProvider` pattern | HIGH |
| Strategy reporting | Low | 0.5 | Low - mechanical addition to tool response | HIGH |
| Tree-sitter tag extraction (def/ref) | High | 3-4 | Medium - need tags.scm queries per language, different from body queries | MEDIUM |
| LSP documentSymbol fallback for RepoMap | Low | 1 | Low - existing `GetSymbolOverview` provides the data | HIGH |
| SQLite tag cache | Medium | 1-2 | Low - existing sqlite infrastructure, well-understood pattern | HIGH |
| Cross-file reference graph | Medium | 2-3 | Medium - graph construction from tag pairs, handling identifier ambiguity | MEDIUM |
| PageRank implementation (pure Go) | Medium | 2 | Low - well-documented algorithm (~150 LOC), no networkx needed | MEDIUM |
| Token budget + output formatting | Medium | 2 | Low - mechanical, token counting is approximate (byte heuristic ok) | HIGH |
| Task-focused context selection | Medium | 2 | Low - builds on existing graph + personalization weights | HIGH |
| LSP reference enrichment | High | 2-3 | High - depends on warm pool state, partial availability, async enrichment | MEDIUM |
| Scope-aware elided output | Medium | 2 | Medium - tree-sitter scope navigation for body elision | MEDIUM |
| Ellipsis/placeholder support | Medium | 1-2 | Medium - block splitting, per-chunk matching, edge cases | MEDIUM |

## Edge Cases and Known Difficulties

### RepoMap Edge Cases
- **Large monorepos (>10K files):** PageRank convergence time. Mitigation: limit iterations, use approximation
- **Languages without tree-sitter grammars:** Fall back to LSP documentSymbol (slower, requires warm LS). 48 of 52 languages lack tree-sitter grammars currently
- **Identifier collision across files:** `init()` in Go, `main()` everywhere. PageRank handles this naturally (common names get diluted importance)
- **Generated code / vendored dependencies:** Must respect .gitignore and .serenaignore. Don't index node_modules, vendor/, generated .pb.go
- **Cache invalidation on branch switch:** mtime changes for all files. Full re-index is acceptable (tree-sitter parsing is fast)

### Fuzzy Editing Edge Cases
- **Multiple equally-good matches:** Both exact and fuzzy can find >1 match. Must refuse ambiguous edits
- **Very short search blocks (1-2 lines):** High false positive rate with fuzzy matching. Lower threshold or require exact match for short blocks
- **Mixed tabs and spaces:** Whitespace normalization must handle tabs-to-spaces equivalence without destroying tab-indented files
- **Unicode in identifiers:** SequenceMatcher/Levenshtein must work on rune-level, not byte-level
- **Empty search block:** Insertion semantics, not replacement. Must be handled as special case
- **LLM adds/removes trailing newlines:** Common failure. Normalize trailing whitespace before matching
- **Search block from wrong file version:** Agent read file, file changed, agent sends stale search block. Fuzzy matching helps here naturally
- **Python indentation as logic:** Fuzzy matching that changes indentation in Python can change program semantics. Extra caution needed

## Sources

- [Aider RepoMap: Building a better repository map with tree-sitter](https://aider.chat/2023/10/22/repomap.html) - Original design article
- [Aider Repository Mapping System - DeepWiki](https://deepwiki.com/Aider-AI/aider/4.1-repository-mapping) - Technical deep-dive: RepoMap class, PageRank, caching, token budgets
- [Aider Repository Map Documentation](https://aider.chat/docs/repomap.html) - Official docs: 130+ languages, configurable token budgets
- [Aider Search and Replace Logic - DeepWiki](https://deepwiki.com/Aider-AI/aider/3.2-prompt-engineering-and-templates) - Strategy cascade: exact, whitespace, dotdotdots, fuzzy
- [Code Surgery: How AI Assistants Make Precise Edits - Fabian Hertwig](https://fabianhertwig.com/blog/coding-assistants-file-edits/) - Cross-tool comparison: Aider, Codex, RooCode, Cursor edit strategies
- [RooCode Search and Replace Strategy - DeepWiki](https://deepwiki.com/qpd-v/Roo-Code/6.2-search-and-replace-strategy) - 9 strategies, Levenshtein, middle-out search, indentation preservation
- [How Cursor Actually Indexes Your Codebase - Towards Data Science](https://towardsdatascience.com/how-cursor-actually-indexes-your-codebase/) - Embedding-based chunking, Merkle tree, Turbopuffer vector DB
- [Cursor Codebase Indexing Documentation](https://docs.cursor.com/context/codebase-indexing) - Official: semantic chunking, path obfuscation, local retrieval
- [Continue.dev Codebase Indexing - DeepWiki](https://deepwiki.com/continuedev/continue/3.4-context-providers) - LanceDB, tree-sitter chunking, SQLite FTS5, code_snippets index
- [Continue.dev Context Providers Documentation](https://docs.continue.dev/customize/context/codebase) - @codebase provider, repo-map provider, embeddings + keyword search
- [Sourcegraph Cody Agentic Context Fetching](https://sourcegraph.com/docs/cody/capabilities/agentic-context-fetching) - Mini-agent using search + tools for context retrieval
- [How Cody Understands Your Codebase - Sourcegraph](https://sourcegraph.com/blog/how-cody-understands-your-codebase) - RAG architecture, Search API, multi-repo support
- [RepoMapper MCP Server - GitHub](https://github.com/pdavis68/RepoMapper) - Go-based MCP server implementing Aider's RepoMap pattern
- [Context Engineering for Coding Agents - Martin Fowler](https://martinfowler.com/articles/exploring-gen-ai/context-engineering-coding-agents.html) - Selection, compression, ordering, isolation, format optimization
- [Context Engineering: Infrastructure for AI Agents](https://arxiv.org/html/2602.20478v1) - Three-tier architecture: hot/domain/cold memory
- [Claude Code Text Editor Tool](https://platform.claude.com/docs/en/agents-and-tools/tool-use/text-editor-tool) - old_string/new_string format, no fuzzy matching built-in
- [Claude Code Hash-Based Line Addressing Discussion](https://github.com/anthropics/claude-code/issues/25775) - Community discussion on improving edit reliability
