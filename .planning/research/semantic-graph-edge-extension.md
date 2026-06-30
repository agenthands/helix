# Research: Semantic Graph Edge Type Extension

**Date:** 2026-06-26
**Context:** v2.5 Agent Harness Rebuild is in progress. This is research for a future milestone.

## Executive Summary

Helix's current semantic graph has 3 edge types (`RESOLVES_TO`, `CALLS`, `USES_TYPE`) with a 7-tier confidence ladder. This research compares against codebase-memory-mcp which has 23+ edge types covering cross-service, channel, similarity, and data-flow relationships.

**Recommendation:** Extend Helix's edge model in phases, prioritizing:
1. **Structure edges** (DEFINES, IMPORTS, IMPLEMENTS, INHERITS) — highest ROI for semantic analysis
2. **Data flow edges** (DATA_FLOWS with arg-to-param mapping) — enables taint analysis
3. **Cross-service edges** (HTTP_CALLS, ASYNC_CALLS) — enables architecture analysis
4. **Similarity edges** (SIMILAR_TO, SEMANTICALLY_RELATED) — enables near-clone detection

## Current State: Helix Semantic Graph

### Languages Implemented
- Go (provider.go: 580 lines)
- TypeScript (provider.go: 408 lines)
- Python (provider.go: 386 lines)

### Edge Types (3 total)
| Edge Kind | Confidence Ladder | Source |
|-----------|-------------------|--------|
| `RESOLVES_TO` | 0.20–1.00 | Tree-sitter + LSP enrichment |
| `CALLS` | 0.20–1.00 | Tree-sitter + LSP enrichment |
| `USES_TYPE` | 0.20–1.00 | Tree-sitter + LSP enrichment |

### Confidence Model
7-tier ladder: `1.00 / 0.90 / 0.80 / 0.70 / 0.60 / 0.45 / 0.20`
- Comment-derived edges cap at 0.60
- LSP-validated edges reach 1.00
- Unresolved chains emit at 0.20

### Storage
- SQLite overlay (live workspace state)
- DuckDB (persistent snapshots)
- Bleve corpus (text search over symbols)

### Architecture
```
internal/semantic/
├── extract/           # Tree-sitter extraction (Go, TS, Python)
├── types/             # Type resolution layer (7-tier confidence)
├── store/             # SQLite overlay + DuckDB persistence
├── live/              # Live overlay watcher
├── lspenrich/         # LSP enrichment layer
└── retrieval/         # Bleve corpus
```

## Comparison: codebase-memory-mcp Edge Model

### Edge Types (23+)
```
DEFINES, DEFINES_METHOD, IMPORTS               # Structure
CALLS, HTTP_CALLS, ASYNC_CALLS                # Calls (sync/async/cross-service)
IMPLEMENTS, HANDLES, INHERITS                  # Inheritance
USAGE, CONFIGURES, WRITES                      # Resource usage
MEMBER_OF, TESTS, USES_TYPE                    # Membership
FILE_CHANGES_WITH                              # Co-change
SIMILAR_TO, SEMANTICALLY_RELATED               # Near-clone detection
DATA_FLOWS                                     # Field-level data flow
EMITS, LISTENS_ON                              # Channels (Socket.IO, EventEmitter)
CROSS_*                                        # Cross-repo links
```

### Node Types (13)
```
Project, Package, Folder, File, Module,
Class, Function, Method, Interface, Enum, Type,
Route, Resource
```

### Unique Capabilities
1. **Cypher queries** — openCypher read subset
2. **Vector semantic search** — Nomic embeddings (no API key)
3. **Cross-service linking** — HTTP routes, gRPC, GraphQL
4. **Near-clone detection** — MinHash + LSH (SIMILAR_TO)
5. **3D visualization** — Built-in UI
6. **Team artifact** — Compressed SQLite artifact for repo sharing

## Gap Analysis

### What Helix Already Has
- ✅ Call graph (`CALLS`)
- ✅ Type resolution (`RESOLVES_TO`, `USES_TYPE`)
- ✅ Confidence-weighted edges (7-tier ladder)
- ✅ LSP enrichment merge
- ✅ Live overlay with epoch versioning
- ✅ SQLite persistence

### What Helix Lacks

| Category | Edge Types | Value | Effort |
|----------|------------|-------|--------|
| **Structure** | DEFINES, IMPORTS, IMPLEMENTS, INHERITS | High — enables layer checking, blast radius | Medium — tree-sitter queries exist |
| **Data flow** | DATA_FLOWS with arg-to-param | High — enables taint analysis | High — requires interprocedural analysis |
| **Cross-service** | HTTP_CALLS, ASYNC_CALLS | Medium — enables architecture analysis | Medium — requires HTTP client/server detection |
| **Channels** | EMITS, LISTENS_ON | Medium — enables async flow analysis | Medium — requires framework-specific patterns |
| **Similarity** | SIMILAR_TO, SEMANTICALLY_RELATED | Low — requires embeddings | High — needs vector index |
| **Infrastructure** | Route, Resource nodes | Low — niche use case | Medium — requires IaC parsing |

## Proposed Implementation Phases

### Phase 1: Structure Edges (Highest ROI)

**Edges:** `DEFINES`, `IMPORTS`, `IMPLEMENTED_BY`, `EXTENDS`

**Why first:** These edges are nearly free — tree-sitter already parses them. They enable:
- Layer checking (no circular imports)
- Blast radius analysis (what breaks when I change X)
- Interface implementation tracking

**Implementation:**
1. Extend `internal/semantic/extract/golang/queries.scm` with import/extends/implements captures
2. Extend `GraphEdge.EdgeKind` enum in `internal/semantic/graph/repair.go`
3. Add `UpsertEdgesWithMerge` support for new edge kinds
4. Update storage schema (migration)

**Estimated effort:** 2-3 phases

### Phase 2: Data Flow Edges

**Edges:** `DATA_FLOWS` with arg-to-param mapping + field access chains

**Why second:** Enables taint analysis integration with SMTC security tools.

**Implementation:**
1. Add parameter binding analysis to type resolution
2. Track field access chains (e.g., `this.field.subfield`)
3. Store arg-to-param mappings in edge metadata

**Estimated effort:** 3-4 phases

### Phase 3: Cross-Service Edges

**Edges:** `HTTP_CALLS`, `ASYNC_CALLS`

**Why third:** Enables architecture analysis for microservices.

**Implementation:**
1. Detect HTTP client calls (fetch, axios, http.Get, etc.)
2. Detect HTTP server routes (annotations, decorators)
3. Extract route patterns and link client → server

**Estimated effort:** 2-3 phases

### Phase 4: Channel Edges

**Edges:** `EMITS`, `LISTENS_ON`

**Why fourth:** Enables async flow analysis.

**Implementation:**
1. Detect EventEmitter/Socket.IO patterns
2. Link emit calls to listener registrations

**Estimated effort:** 2 phases

### Phase 5 (Optional): Similarity Edges

**Edges:** `SIMILAR_TO`, `SEMANTICALLY_RELATED`

**Why last:** Requires embeddings infrastructure not present in Helix.

**Implementation:**
1. Integrate MinHash + LSH for structural similarity
2. Optionally: add embedding model for semantic similarity

**Estimated effort:** 3-4 phases (significant new dependency)

## Dependencies

### No New Go Dependencies Required
All phases can be implemented with current Go stdlib + tree-sitter.

### Storage Migration Required
- New `edge_kind` enum values
- New `edge_metadata` column for arg-to-param mappings
- Migration from current schema version

### Integration Points
- SMTC security tools (taint analysis) — consumer of DATA_FLOWS
- `get_architecture` tool — consumer of cross-service edges
- `blast_radius` tool — consumer of structure edges

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Performance regression (more edges) | Medium | Medium | Lazy edge materialization |
| Storage bloat | Medium | Low | Compression + pruning |
| Language parity (Go ≠ TS ≠ Python) | High | Medium | Per-language edge coverage docs |
| Cross-service detection accuracy | Medium | Low | Confidence scoring on detection |

## Open Questions

1. **Should we add a `Route` node type?** codebase-memory-mcp has this for REST endpoints. Helix could use it for HTTP route → handler linking.

2. **How do confidence scores apply to new edge types?** LSP-validated imports might get 1.00; tree-sitter-only imports start at 0.70.

3. **Should DATA_FLOWS edges be materialized or computed on-demand?** Materialized enables faster taint queries but increases storage.

4. **Do we need Cypher query support?** codebase-memory-mcp has it. Helix could add SQL-over-edges for complex traversals.

## References

- `internal/semantic/extract/golang/queries.scm` — Go tree-sitter queries
- `internal/semantic/types/doc.go` — 7-tier confidence ladder
- `internal/semantic/graph/repair.go` — GraphEdge value types
- `internal/semantic/store/effective_graph.go` — Edge storage queries
- `codebase-memory-mcp` repo — Reference implementation for edge types