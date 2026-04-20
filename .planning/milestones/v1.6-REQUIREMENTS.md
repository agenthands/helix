# Requirements: Serena

**Defined:** 2026-04-15
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.6 Requirements

Requirements for Context Intelligence & Resilient Editing milestone. Each maps to roadmap phases.

### Fuzzy Editing

- [x] **FUZZ-01**: Agent can fuzzy-match a search block against file content using a 4-strategy cascade (exact → whitespace-normalized → indentation-flexible → fail with diff)
- [x] **FUZZ-02**: Agent receives strategy reporting in tool response (match_strategy used, similarity_score)
- [x] **FUZZ-03**: Replacement text preserves original file's indentation when fuzzy match succeeds
- [x] **FUZZ-04**: Agent can use standalone `fuzzy_edit` MCP tool for raw text fuzzy matching independent of symbol boundaries
- [x] **FUZZ-05**: `replace_symbol_body` falls back to fuzzy matching within tree-sitter-located body when exact match fails
- [x] **FUZZ-06**: `replace_content` falls back to fuzzy matching when exact/regex match fails
- [x] **FUZZ-07**: Fuzzy edit refuses ambiguous edits when search text matches multiple locations
- [x] **FUZZ-08**: Agent can use ellipsis/placeholder (`...`) in search blocks to indicate unchanged code sections

### RepoMap Context Intelligence

- [x] **RMAP-01**: Agent can extract def/ref tags from source files via tree-sitter .scm queries (Go, Python, TypeScript, Rust)
- [x] **RMAP-02**: Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction
- [x] **RMAP-03**: Tag cache persists in SQLite with mtime-based invalidation, surviving client reconnects via daemon lifecycle
- [x] **RMAP-04**: Cross-file reference graph built from extracted tags (nodes = files, edges = ref→def)
- [x] **RMAP-05**: Personalized PageRank ranks symbol importance with configurable personalization weights
- [x] **RMAP-06**: Agent can call `get_repo_map` to get a token-budgeted structural overview of the repo with ranked symbol importance
- [x] **RMAP-07**: Agent can call `get_context` to get task-focused context (most relevant symbols for given files/task description)
- [x] **RMAP-08**: Warm LSP sessions enrich the reference graph with precise cross-file references when available
- [x] **RMAP-09**: Output uses scope-aware elision (signatures without bodies via tree-sitter)
- [x] **RMAP-10**: Token budget parameter controls output size, with binary search to maximize coverage within budget

### Multi-Language Grammar Expansion

- [x] **D-01**: Full aider parity for languages with Go bindings — target all languages with .scm queries in aider reference collections
- [x] **D-02**: Tiered waves — Wave 1 (highest demand), Wave 2a, Wave 2b, Wave 3 (gap closure)
- [x] **D-03**: Official tree-sitter org Go bindings preferred, community forks acceptable, no CGO required
- [x] **D-04**: Accept binary size growth — all grammars compiled in, single binary, no build-tag gating
- [x] **D-05**: Both query types per language — repomap tag queries AND edit body queries
- [x] **D-06**: Merge best of aider reference collections (tree-sitter-languages and tree-sitter-language-pack)
- [x] **D-07**: Both unit and integration tests — fixture file per language with golden expected tags
- [x] **D-08**: Wave 1 languages get LS fixture integration tests, Wave 2+ gets tree-sitter-only coverage

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
| FUZZ-01 | Phase 29 | Complete |
| FUZZ-02 | Phase 29 | Complete |
| FUZZ-03 | Phase 29 | Complete |
| FUZZ-04 | Phase 26 | Complete |
| FUZZ-05 | Phase 26 | Complete |
| FUZZ-06 | Phase 26 | Complete |
| FUZZ-07 | Phase 29 | Complete |
| FUZZ-08 | Phase 29 | Complete |
| RMAP-01 | Phase 27 | Complete |
| RMAP-02 | Phase 27 | Complete |
| RMAP-03 | Phase 27 | Complete |
| RMAP-04 | Phase 30 | Complete |
| RMAP-05 | Phase 30 | Complete |
| RMAP-06 | Phase 30 | Complete |
| RMAP-07 | Phase 30 | Complete |
| RMAP-08 | Phase 30 | Complete |
| RMAP-09 | Phase 27 | Complete |
| RMAP-10 | Phase 30 | Complete |
| D-01 | Phase 31 | Complete |
| D-02 | Phase 31 | Complete |
| D-03 | Phase 31 | Complete |
| D-04 | Phase 31 | Complete |
| D-05 | Phase 31 | Complete |
| D-06 | Phase 31 | Complete |
| D-07 | Phase 31 | Complete |
| D-08 | Phase 31 | Complete |

**Coverage:**
- v1.6 requirements: 26 total (18 FUZZ/RMAP + 8 D)
- Mapped to phases: 26
- Unmapped: 0
- Complete: 26

---
*Requirements defined: 2026-04-15*
*Last updated: 2026-04-15 after roadmap creation*
