# Requirements: Serena

**Defined:** 2026-04-15
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.6 Requirements

Requirements for Context Intelligence & Resilient Editing milestone. Each maps to roadmap phases.

### Fuzzy Editing

- [ ] **FUZZ-01**: Agent can fuzzy-match a search block against file content using a 4-strategy cascade (exact → whitespace-normalized → indentation-flexible → fail with diff)
- [ ] **FUZZ-02**: Agent receives strategy reporting in tool response (match_strategy used, similarity_score)
- [ ] **FUZZ-03**: Replacement text preserves original file's indentation when fuzzy match succeeds
- [ ] **FUZZ-04**: Agent can use standalone `fuzzy_edit` MCP tool for raw text fuzzy matching independent of symbol boundaries
- [ ] **FUZZ-05**: `replace_symbol_body` falls back to fuzzy matching within tree-sitter-located body when exact match fails
- [ ] **FUZZ-06**: `replace_content` falls back to fuzzy matching when exact/regex match fails
- [ ] **FUZZ-07**: Fuzzy edit refuses ambiguous edits when search text matches multiple locations
- [ ] **FUZZ-08**: Agent can use ellipsis/placeholder (`...`) in search blocks to indicate unchanged code sections

### RepoMap Context Intelligence

- [ ] **RMAP-01**: Agent can extract def/ref tags from source files via tree-sitter .scm queries (Go, Python, TypeScript, Rust)
- [ ] **RMAP-02**: Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction
- [ ] **RMAP-03**: Tag cache persists in SQLite with mtime-based invalidation, surviving client reconnects via daemon lifecycle
- [ ] **RMAP-04**: Cross-file reference graph built from extracted tags (nodes = files, edges = ref→def)
- [ ] **RMAP-05**: Personalized PageRank ranks symbol importance with configurable personalization weights
- [ ] **RMAP-06**: Agent can call `get_repo_map` to get a token-budgeted structural overview of the repo with ranked symbol importance
- [ ] **RMAP-07**: Agent can call `get_context` to get task-focused context (most relevant symbols for given files/task description)
- [ ] **RMAP-08**: Warm LSP sessions enrich the reference graph with precise cross-file references when available
- [ ] **RMAP-09**: Output uses scope-aware elision (signatures without bodies via tree-sitter)
- [ ] **RMAP-10**: Token budget parameter controls output size, with binary search to maximize coverage within budget

## Future Requirements

### Fuzzy Editing

- **FUZZ-09**: Configurable similarity threshold per tool call (default 0.8, range 0.6-1.0)

### RepoMap

- **RMAP-11**: Multi-workspace RepoMap aggregation

## Out of Scope

| Feature | Reason |
|---------|--------|
| Embedding/vector search | Augment Context Engine owns this (PROJECT.md) |
| Knowledge graphs | CodeGraphContext/GitNexus own this (PROJECT.md) |
| Git-aware context | GitHub MCP Server handles git (PROJECT.md) |
| apply_patch / unified diff format | Model-specific, Serena serves all agents |
| Full AST-aware diffing | Massive complexity for marginal gain |
| Multi-repository RepoMap | Cross-repo adds massive complexity; Sourcegraph handles this |
| Natural language codebase query | Requires embeddings or LLM-in-the-loop retrieval |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FUZZ-01 | Phase 25 | Pending |
| FUZZ-02 | Phase 25 | Pending |
| FUZZ-03 | Phase 25 | Pending |
| FUZZ-04 | Phase 26 | Pending |
| FUZZ-05 | Phase 26 | Pending |
| FUZZ-06 | Phase 26 | Pending |
| FUZZ-07 | Phase 25 | Pending |
| FUZZ-08 | Phase 25 | Pending |
| RMAP-01 | Phase 27 | Pending |
| RMAP-02 | Phase 27 | Pending |
| RMAP-03 | Phase 27 | Pending |
| RMAP-04 | Phase 28 | Pending |
| RMAP-05 | Phase 28 | Pending |
| RMAP-06 | Phase 28 | Pending |
| RMAP-07 | Phase 28 | Pending |
| RMAP-08 | Phase 28 | Pending |
| RMAP-09 | Phase 27 | Pending |
| RMAP-10 | Phase 28 | Pending |

**Coverage:**
- v1.6 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0

---
*Requirements defined: 2026-04-15*
*Last updated: 2026-04-15 after roadmap creation*
