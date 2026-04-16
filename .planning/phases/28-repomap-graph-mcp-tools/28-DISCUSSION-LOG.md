# Phase 28: RepoMap Graph & MCP Tools - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-16
**Phase:** 28-repomap-graph-mcp-tools
**Areas discussed:** Graph model & PageRank, MCP tool design, LSP enrichment strategy, Token budgeting algorithm

---

## Graph Model & PageRank

### Graph Nodes

| Option | Description | Selected |
|--------|-------------|----------|
| Files (Recommended) | Nodes = files, edges = cross-file ref-to-def links. Simpler graph, faster PageRank, matches aider's approach. | ✓ |
| Symbols | Nodes = individual symbols. Finer-grained but much larger graph. | |
| Hybrid | File-level for ranking, symbol-level edges internally. | |

**User's choice:** Files
**Notes:** None

### Graph Persistence

| Option | Description | Selected |
|--------|-------------|----------|
| In-memory rebuild (Recommended) | Build from cached tags on first call. No schema migration burden. | ✓ |
| SQLite persistence | Store edges in tags.db. Instant availability but complex schema. | |
| You decide | Claude picks. | |

**User's choice:** In-memory rebuild
**Notes:** None

### PageRank Personalization

| Option | Description | Selected |
|--------|-------------|----------|
| Seed files (Recommended) | Personalization vector seeded from agent-specified files. | ✓ |
| Uniform + filter | Standard PageRank, then filter by relevance. | |
| You decide | Claude picks. | |

**User's choice:** Seed files
**Notes:** None

---

## MCP Tool Design

### Tool Registration Type

| Option | Description | Selected |
|--------|-------------|----------|
| Skill tools (Recommended) | ToolProvider skill in internal/skill/repomap/. Follows memory skill pattern. | ✓ |
| Kernel tools | Register in internal/kernel/repomap/. Direct kernel access. | |
| You decide | Claude picks. | |

**User's choice:** Skill tools
**Notes:** None

### Output Format

| Option | Description | Selected |
|--------|-------------|----------|
| Elided source tree (Recommended) | File paths as tree with elided symbols nested under each file. | ✓ |
| Flat symbol list | Ranked list of symbols with paths. | |
| Markdown sections | One markdown section per file with code blocks. | |

**User's choice:** Elided source tree
**Notes:** None

### Context Input

| Option | Description | Selected |
|--------|-------------|----------|
| Files + optional task text (Recommended) | File paths primary, optional task_description for future semantic matching. | ✓ |
| Task description only | NL description, system extracts files. Complex. | |
| Files only | Only file paths. Simple and deterministic. | |

**User's choice:** Files + optional task text
**Notes:** None

---

## LSP Enrichment Strategy

### Enrichment Trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Opportunistic (Recommended) | Piggyback on warm LSP sessions. Don't start new sessions for enrichment. | ✓ |
| On-demand per query | Actively request LSP refs for seed files on each query. | |
| Background sweep | Periodically sweep cached files through warm LS sessions. | |

**User's choice:** Opportunistic
**Notes:** None

### Merge Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Additive edges (Recommended) | LSP refs add new cross-file edges. Never replace tree-sitter tags. | ✓ |
| LSP overrides tree-sitter | Replace tree-sitter refs when LSP available. | |
| You decide | Claude picks. | |

**User's choice:** Additive edges
**Notes:** None

---

## Token Budgeting Algorithm

### Token Counting

| Option | Description | Selected |
|--------|-------------|----------|
| Character-based estimate (Recommended) | Approximate tokens as chars/4. Fast, no dependency. | ✓ |
| Tiktoken-compatible | Proper tokenizer library. More precise but adds dependency. | |
| You decide | Claude picks. | |

**User's choice:** Character-based estimate
**Notes:** None

### Pruning Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Lowest-ranked files first (Recommended) | Binary search on file count by PageRank score. | ✓ |
| Elision depth reduction | First reduce elision depth, then cut files. | |
| You decide | Claude picks. | |

**User's choice:** Lowest-ranked files first
**Notes:** None

---

## Claude's Discretion

- Graph rebuild caching strategy
- PageRank convergence parameters
- Tool parameter validation and error response format
- LSP enrichment batching and rate limiting
- File tree rendering format details

## Deferred Ideas

None
