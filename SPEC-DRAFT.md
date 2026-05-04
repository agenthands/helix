# SPEC.md — Helix Live Semantic Index and Graph Intelligence

## 1. Product Description

Helix is the IDE for coding agents. Today, Helix exposes online semantic tools backed by language servers, tree-sitter, RepoMap, diagnostics, file operations, memory, and profile/mode control. This specification adds a production-grade **Live Semantic Index** that turns those online capabilities into a durable, queryable, continuously updated semantic knowledge layer.

The Live Semantic Index stores files, symbols, references, imports, calls, types, diagnostics, graph edges, graph scores, clusters, retrieval metadata, and live overlay updates in a local embedded fact store. It enriches structural facts from tree-sitter with semantic evidence from LSPs, ranks code using task-specific graph projections, builds semantic clusters, and exposes MCP tools for deep code understanding, architecture mapping, impact analysis, and future security/dataflow analysis.

The system must support both reproducibility and freshness:

```text
committed snapshot
  immutable, durable, reproducible, testable

live overlay
  mutable, current, fast, structurally updated from source changes

LSP validation layer
  asynchronous semantic confirmation of overlay facts
```

The goal is not to replace existing Helix tools. The goal is to make their results persistent, composable, evidence-backed, ranked, live, and reusable.

Current model:

```text
Agent asks tool
  → Helix queries LSP/tree-sitter now
  → result returned
```

New model:

```text
Agent asks tool
  → Helix reads effective semantic graph
  → effective graph = committed snapshot + live overlay - tombstones
  → result uses graph/ranking/retrieval
  → critical facts are optionally revalidated with live LSP/tree-sitter
  → result returned with evidence, confidence, freshness, and graph version
```

## 2. Goals

### 2.1 Primary Goals

1. Add a durable semantic fact store for code intelligence.
2. Add live graph updates during source changes.
3. Maintain immutable committed snapshots for reproducibility.
4. Maintain a mutable live overlay for current worktree state.
5. Extract symbols, references, imports, calls, types, containment, diagnostics, and file relationships.
6. Merge tree-sitter structural facts with LSP semantic facts.
7. Persist evidence, confidence, freshness, validation state, and source for every derived relationship.
8. Add graph ranking using multiple task-specific PageRank projections.
9. Add semantic clustering for architecture understanding and context retrieval.
10. Add new MCP tools that expose deep semantic context and graph status.
11. Keep Helix deployable as a single Go binary.
12. Preserve lazy initialization, existing tool behavior, profile/mode gating, and graceful degradation.
13. Add an evaluation harness that proves agent + Helix improves success rate, cost, latency, and safety versus baseline agents.
14. Add explicit agent safety guardrails and Definition of Done rules for high-risk operations.
15. Add tiered type-resolution and access-chain resolution for best-effort semantic precision in dynamic and weakly typed languages.
16. Add a typed pipeline DAG with phase validation for daemon bootstrap, semantic indexing, live updates, and evaluation workflows.

### 2.2 Non-Goals for Initial Release

1. Full compiler-grade semantic analysis for all 52 languages.
2. Perfect dynamic dispatch resolution.
3. Full taint analysis engine.
4. Mandatory external graph database dependency.
5. Mandatory vector database dependency.
6. Replacing existing `get_repo_map` and `get_context` in a breaking way.
7. Requiring Python, Docker, or external services.
8. Recomputing full graph scores after every source edit.
9. Making evaluation dependent on a single proprietary benchmark.
10. Hard-blocking all agent actions with guardrails in the first release; guardrails start as warn/record and become enforceable by profile/mode.
11. Perfect type inference for dynamic languages.
12. Replacing all bootstrap code with a phase DAG in one migration step.

## 3. Critical Review of Previous Draft

The previous draft had a strong foundation but several production issues:

1. **Live update model was appended, not integrated.** The draft ended with a final architecture and then appended live graph updates afterward. This made the design look batch-first even though the product requirement is live-first.
2. **Snapshot and overlay semantics were not unified.** It defined snapshot tables first and overlay tables later, but did not define effective-read semantics, overlay tombstones, or compaction as part of the core store contract.
3. **Node identity was underspecified.** Edges referenced generic `src_id` and `dst_id`, but the schema did not define a unified node table or explain how file IDs, symbol IDs, diagnostic IDs, memory IDs, and cluster IDs coexist.
4. **Stable symbol IDs included file path unconditionally.** That breaks identity across renames. File path should be part of fallback identity, but content lineage, package/module path, owner path, qualified name, signature hash, and LSP identity should be used to preserve identity where possible.
5. **LSP enrichment pseudocode used `go_to_definition` on definitions.** For a symbol definition, go-to-definition often resolves to itself. Definition resolution should be used mainly for references/call sites; symbols should use hover, document symbols, call hierarchy, type hierarchy, implementations, and references.
6. **`DEFINES_OR_RESOLVES_TO` was not in the edge vocabulary.** The draft introduced an edge kind that was not declared elsewhere.
7. **Graph score freshness was not modeled in schema.** The live section introduced `exact`, `approximate`, `stale`, and `missing`, but the score table lacked `status`, `graph_version`, and `projection_version`.
8. **Cluster freshness was not modeled.** Incremental clustering was described, but cluster tables lacked status/version fields.
9. **Overlay tables used JSON blobs only.** JSON overlays are flexible but make hot queries slower and harder to index. Production design should use typed overlay tables or typed columns plus optional JSON metadata.
10. **Concurrent access and graph versions were underspecified.** The system needs atomic graph versioning, read snapshots, and lock boundaries for MCP queries versus update workers.
11. **Partial/failure states were too coarse.** The system needs distinct states for structurally fresh, semantically pending, partially validated, approximate scores, stale clusters, and failed components.
12. **Compaction lacked concurrency rules.** Compaction must not race active edit transactions, watcher events, or LSP revalidation.
13. **MCP tools did not consistently return freshness metadata.** All semantic tools must report snapshot ID, overlay status, graph version, score status, and pending validation when relevant.
14. **No schema migration/versioning contract for live overlays.** Store versioning must cover snapshots, overlays, and graph cache compatibility.
15. **No clear first-release language scope.** The draft mentioned 52 languages but should explicitly scope production correctness to Go, TypeScript/JavaScript, and Python first, with other languages best-effort.

This rewritten spec fixes those issues by making the live effective graph the central design.

## 4. Architectural Decisions

### ADR-001: Use DuckDB as the embedded persistent fact store

Use DuckDB as the default local semantic fact store.

Rationale:

```text
DuckDB = durable local analytical facts
Go graph layer = hot algorithm engine
MCP tools = agent interface
LSP/tree-sitter = evidence providers
```

Semantic code understanding needs a fact store first, not a graph database first. Graph databases are optional later accelerators, not the source of truth.

### ADR-002: Use committed snapshots plus live overlay

Maintain:

```text
committed_snapshot
  immutable
  durable
  reproducible
  suitable for tests and historical comparison

live_overlay
  mutable
  current worktree state
  contains upserts, tombstones, pending validation, approximate scores
```

Every effective semantic query reads:

```text
effective_facts = latest_committed_snapshot + live_overlay - tombstones
```

When the repo becomes idle, the overlay is compacted into a new committed snapshot.

### ADR-003: Tree-sitter updates first, LSP validates later

Tree-sitter must update the graph quickly after a file change. LSP confirmation runs asynchronously.

```text
Phase 1: structural update
  source: file watcher / Helix edit event + tree-sitter
  latency: sub-second to a few seconds
  confidence: medium
  status: structurally_fresh_semantically_pending

Phase 2: semantic validation
  source: LSP enrichment
  latency: bounded background job
  confidence: high
  status: fresh or partially_validated
```

This avoids blocking live graph freshness on slow/crashy language servers.

### ADR-004: PageRank is a ranking signal, not the semantic model

PageRank can rank central files and symbols, but it cannot resolve imports, overloads, receiver dispatch, scopes, or types.

Correct model:

```text
facts → graph projections → PageRank scores → ranked retrieval
```

Incorrect model:

```text
PageRank score → semantic truth
```

### ADR-005: Use custom Go graph algorithms first

Implement hot graph operations directly in Go:

```text
bounded BFS
reverse reachability
weighted PageRank
personalized PageRank
weak components
SCC
local neighborhood expansion
cluster repair
shortest semantic path
```

Gonum may be used for validation or non-critical algorithms, but not as the core storage or graph model.

### ADR-006: All semantic results must carry confidence, evidence, and freshness

Every tool response that uses the semantic index must report:

```text
snapshot_id
graph_version
overlay_active
freshness
score_status
cluster_status
pending_lsp_files
confidence/evidence for relationships
```

### ADR-007: Add an evaluation harness as a first-class product subsystem

Helix must be able to prove that agents using Helix perform better than agents without Helix. The evaluation harness must compare baseline, native, semantic, and guarded modes and measure success, cost, latency, tool behavior, safety compliance, and context quality.

Rationale:

```text
No eval harness → no objective way to justify new Helix features.
Eval harness → every feature can be shipped with evidence.
```

### ADR-008: Add guardrails and Definition of Done as policy plus documentation

Guardrails must exist both as human-readable docs and as machine-checkable policy receipts for risky operations.

Rationale:

```text
Docs teach the agent/client what good behavior means.
Policy receipts let Helix verify whether required pre-checks happened.
```

Initial guardrails are soft by default: warn and record. Profiles may later enforce them strictly.

### ADR-009: Add tiered type-resolution and access-chain resolution

Best-effort languages need explicit type-resolution patterns. The resolver must support confidence tiers, evidence tracking, fixpoint resolution of access chains, and comment-based fallbacks such as JSDoc, PHPDoc, YARD, and Python docstrings/type comments.

Rationale:

```text
a.b.c.d() chains are where tree-sitter-only extraction loses semantic precision.
Tiered confidence makes imperfect resolution useful without pretending it is exact.
```

### ADR-010: Add a typed pipeline DAG with phase validation

Daemon bootstrap, semantic indexing, live updates, and evaluation workflows must be describable as typed phase graphs with dependency validation, cycle detection, typed outputs, and deterministic shutdown order.

Rationale:

```text
Implicit step ordering causes fragile regressions.
A phase DAG makes ordering explicit and testable.
```

## 5. High-Level Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                         MCP Runtime                          │
│ stdio / HTTP / profile middleware / mode gating / telemetry  │
└──────────────────────────────┬───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                    Code Intelligence Kernel                   │
│ LSP worker pool / symbol ops / file ops / diagnostics         │
└──────────────────────────────┬───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                    Live Semantic Service                      │
│ index planner / extractors / LSP enrichment / resolvers       │
│ live update queue / overlay store / compaction worker         │
│ graph cache / PageRank / clustering / retrieval               │
└──────────────────────────────┬───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                      Semantic Fact Store                      │
│ DuckDB: snapshots + overlays + typed facts + graph metadata   │
└──────────────────────────────┬───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                    Agent Semantic Tools                       │
│ context / graph status / related symbols / clusters / impact │
└──────────────────────────────────────────────────────────────┘
```

Live update flow:

```text
source change
  → file watcher / Helix edit event
  → debounce + coalesce
  → parse changed file with tree-sitter
  → diff old effective facts vs new facts
  → write live overlay upserts/tombstones
  → repair graph cache
  → mark affected scores/clusters stale or approximate
  → enqueue LSP revalidation
  → compact overlay after idle
```

## 6. Package Layout

```text
internal/semantic/
  service.go
  config.go
  types.go

internal/semantic/store/
  duckdb.go
  migrations.go
  snapshot.go
  overlay.go
  effective.go

internal/semantic/indexer/
  planner.go
  full.go
  incremental.go
  compaction.go

internal/semantic/live/
  watcher.go
  queue.go
  coalesce.go
  update.go
  revalidate.go

internal/semantic/extract/
  registry.go
  provider.go
  treesitter.go
  captures.go

internal/semantic/resolve/
  symbols.go
  references.go
  imports.go
  merge.go

internal/semantic/lspenrich/
  enrich.go
  budget.go
  priority.go

internal/semantic/graph/
  graph.go
  cache.go
  repair.go
  traversal.go

internal/semantic/rank/
  pagerank.go
  personalized.go
  fusion.go

internal/semantic/cluster/
  components.go
  labelprop.go
  labels.go

internal/semantic/retrieve/
  seeds.go
  context.go
  token_budget.go

internal/semantic/tools/
  mcp_tools.go
  schemas.go

internal/semantic/testutil/
  fixtures.go
  golden.go

internal/semantic/typeresolve/
  chain.go
  confidence.go
  comments.go
  fixpoint.go
  language_rules.go

internal/guardrails/
  policy.go
  receipts.go
  dod.go
  docs.go

internal/eval/
  suite.go
  runner.go
  modes.go
  metrics.go
  report.go
  cost.go
  traces.go

internal/phasegraph/
  phase.go
  dag.go
  validate.go
  run.go
  shutdown.go
```

## 7. Core Types and Interfaces

```go
type SnapshotID uint64
type GraphVersion uint64
type FileID uint64
type SymbolID uint64
type ReferenceID uint64
type EdgeID uint64
type DiagnosticID uint64
type ClusterID uint64

type NodeKind string

const (
    NodeFile       NodeKind = "file"
    NodeSymbol     NodeKind = "symbol"
    NodeReference  NodeKind = "reference"
    NodeDiagnostic NodeKind = "diagnostic"
    NodeMemory     NodeKind = "memory"
    NodeCluster    NodeKind = "cluster"
)

type Freshness string

const (
    FreshnessFresh                           Freshness = "fresh"
    FreshnessStructurallyFreshPendingLSP     Freshness = "structurally_fresh_semantically_pending"
    FreshnessPartiallyValidated              Freshness = "partially_validated"
    FreshnessStale                           Freshness = "stale"
    FreshnessUpdating                        Freshness = "updating"
    FreshnessFailed                          Freshness = "failed"
)

type ScoreStatus string

const (
    ScoreExact       ScoreStatus = "exact"
    ScoreApproximate ScoreStatus = "approximate"
    ScoreStale       ScoreStatus = "stale"
    ScoreMissing     ScoreStatus = "missing"
)

type ValidationState string

const (
    ValidationPending     ValidationState = "pending"
    ValidationValidated   ValidationState = "validated"
    ValidationInvalidated ValidationState = "invalidated"
    ValidationSkipped     ValidationState = "skipped"
)
```

```go
type SemanticService interface {
    EnsureIndex(ctx context.Context, req EnsureIndexRequest) (*EnsureIndexResult, error)
    RefreshLiveGraph(ctx context.Context, req RefreshLiveGraphRequest) (*RefreshLiveGraphResult, error)
    GetStatus(ctx context.Context, repo RootRef) (*IndexStatus, error)

    QueryContext(ctx context.Context, req ContextRequest) (*ContextResult, error)
    ExplainSymbol(ctx context.Context, req ExplainSymbolRequest) (*SymbolExplanation, error)
    RelatedSymbols(ctx context.Context, req RelatedSymbolsRequest) (*RelatedSymbolsResult, error)
    ClusterMap(ctx context.Context, req ClusterMapRequest) (*ClusterMapResult, error)
    ExplainCluster(ctx context.Context, req ExplainClusterRequest) (*ClusterExplanation, error)
    ChangeImpact(ctx context.Context, req ChangeImpactRequest) (*ChangeImpactResult, error)
    ValidateEdge(ctx context.Context, req ValidateEdgeRequest) (*EdgeValidationResult, error)
}
```

## 8. Store Contract

The store must support committed snapshots, live overlays, effective reads, and compaction.

```go
type FactStore interface {
    BeginSnapshot(ctx context.Context, meta SnapshotMeta) (SnapshotID, error)
    CommitSnapshot(ctx context.Context, id SnapshotID, summary SnapshotSummary) error
    AbortSnapshot(ctx context.Context, id SnapshotID, reason string) error
    GetLatestSnapshot(ctx context.Context, repoID string) (*SnapshotMeta, error)

    UpsertSnapshotFiles(ctx context.Context, snapshot SnapshotID, files []FileFact) error
    UpsertSnapshotSymbols(ctx context.Context, snapshot SnapshotID, symbols []SymbolFact) error
    UpsertSnapshotReferences(ctx context.Context, snapshot SnapshotID, refs []ReferenceFact) error
    UpsertSnapshotEdges(ctx context.Context, snapshot SnapshotID, edges []EdgeFact) error
    UpsertSnapshotDiagnostics(ctx context.Context, snapshot SnapshotID, diagnostics []DiagnosticFact) error

    BeginOverlayTx(ctx context.Context, repoID string) (OverlayTx, error)
    LoadOverlaySummary(ctx context.Context, repoID string) (*OverlaySummary, error)
    ClearOverlay(ctx context.Context, repoID string) error

    LoadEffectiveFileFacts(ctx context.Context, repoID string, path string) (*EffectiveFileFacts, error)
    QueryEffectiveSymbols(ctx context.Context, req SymbolQuery) ([]SymbolFact, error)
    QueryEffectiveReferences(ctx context.Context, req ReferenceQuery) ([]ReferenceFact, error)
    QueryEffectiveEdges(ctx context.Context, req EdgeQuery) ([]EdgeFact, error)
    LoadEffectiveGraph(ctx context.Context, req LoadGraphRequest) (*GraphData, error)

    UpsertScores(ctx context.Context, scores []GraphScoreFact) error
    UpsertClusters(ctx context.Context, clusters []ClusterFact, members []ClusterMemberFact) error
}
```

```go
type OverlayTx interface {
    UpsertFile(FileFact) error
    MarkFileDeleted(path string) error

    UpsertSymbols([]SymbolFact) error
    MarkSymbolsDeleted([]SymbolID) error

    UpsertReferences([]ReferenceFact) error
    MarkReferencesDeleted([]ReferenceID) error

    UpsertEdges([]EdgeFact) error
    MarkEdgesDeleted([]EdgeID) error

    UpsertDiagnostics([]DiagnosticFact) error
    MarkDiagnosticsDeleted([]DiagnosticID) error

    WriteInvalidations(GraphInvalidation) error
    Commit() error
    Rollback() error
}
```

## 9. Data Model

### 9.1 Schema Version Table

```sql
CREATE TABLE semantic_schema_version (
  version INTEGER PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL
);
```

### 9.2 Snapshot Table

```sql
CREATE TABLE semantic_snapshots (
  snapshot_id UBIGINT PRIMARY KEY,
  repo_id TEXT NOT NULL,
  repo_root TEXT NOT NULL,
  base_snapshot_id UBIGINT,
  kind TEXT NOT NULL, -- full, incremental, live_compaction
  vcs_kind TEXT,
  branch TEXT,
  commit_hash TEXT,
  worktree_hash TEXT NOT NULL,
  schema_version INTEGER NOT NULL,
  indexer_version TEXT NOT NULL,
  tree_sitter_version TEXT,
  lsp_fingerprint TEXT,
  status TEXT NOT NULL, -- building, committed, aborted
  partial BOOLEAN NOT NULL DEFAULT false,
  partial_reason TEXT,
  created_at TIMESTAMP NOT NULL,
  committed_at TIMESTAMP,
  aborted_at TIMESTAMP,
  abort_reason TEXT
);

CREATE INDEX idx_semantic_snapshots_repo_status
ON semantic_snapshots(repo_id, status, committed_at);
```

### 9.3 Unified Node Table

Edges must have typed endpoints. Use a unified node table for all graph-addressable entities.

```sql
CREATE TABLE semantic_nodes (
  snapshot_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  node_kind TEXT NOT NULL,
  stable_key TEXT NOT NULL,
  display_name TEXT,
  file_id UBIGINT,
  symbol_id UBIGINT,
  diagnostic_id UBIGINT,
  memory_id TEXT,
  created_at TIMESTAMP NOT NULL,
  PRIMARY KEY (snapshot_id, node_id)
);

CREATE INDEX idx_semantic_nodes_kind
ON semantic_nodes(snapshot_id, node_kind);

CREATE INDEX idx_semantic_nodes_stable_key
ON semantic_nodes(snapshot_id, stable_key);
```

### 9.4 Files

```sql
CREATE TABLE semantic_files (
  snapshot_id UBIGINT NOT NULL,
  file_id UBIGINT NOT NULL,
  repo_id TEXT NOT NULL,
  path TEXT NOT NULL,
  language TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  size_bytes UBIGINT NOT NULL,
  line_count INTEGER NOT NULL,
  generated BOOLEAN NOT NULL DEFAULT false,
  ignored BOOLEAN NOT NULL DEFAULT false,
  ignore_reason TEXT,
  indexed_at TIMESTAMP NOT NULL,
  PRIMARY KEY (snapshot_id, file_id)
);

CREATE INDEX idx_semantic_files_path
ON semantic_files(snapshot_id, path);
```

### 9.5 Symbols

```sql
CREATE TABLE semantic_symbols (
  snapshot_id UBIGINT NOT NULL,
  symbol_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  file_id UBIGINT NOT NULL,
  language TEXT NOT NULL,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  qualified_name TEXT NOT NULL,
  stable_key TEXT NOT NULL,
  owner_symbol_id UBIGINT,
  parent_scope_id UBIGINT,
  package_path TEXT,
  start_byte INTEGER NOT NULL,
  end_byte INTEGER NOT NULL,
  start_line INTEGER NOT NULL,
  start_col INTEGER NOT NULL,
  end_line INTEGER NOT NULL,
  end_col INTEGER NOT NULL,
  signature TEXT,
  signature_hash TEXT,
  exported BOOLEAN,
  visibility TEXT,
  extraction_source TEXT NOT NULL,
  confidence DOUBLE NOT NULL,
  content_hash TEXT,
  lsp_identity TEXT,
  PRIMARY KEY (snapshot_id, symbol_id)
);

CREATE INDEX idx_semantic_symbols_name
ON semantic_symbols(snapshot_id, name);

CREATE INDEX idx_semantic_symbols_qname
ON semantic_symbols(snapshot_id, qualified_name);

CREATE INDEX idx_semantic_symbols_stable_key
ON semantic_symbols(snapshot_id, stable_key);
```

### 9.6 References

```sql
CREATE TABLE semantic_references (
  snapshot_id UBIGINT NOT NULL,
  ref_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  file_id UBIGINT NOT NULL,
  scope_symbol_id UBIGINT,
  name TEXT NOT NULL,
  ref_kind TEXT NOT NULL,
  receiver_text TEXT,
  start_byte INTEGER NOT NULL,
  end_byte INTEGER NOT NULL,
  start_line INTEGER NOT NULL,
  start_col INTEGER NOT NULL,
  end_line INTEGER NOT NULL,
  end_col INTEGER NOT NULL,
  resolved_symbol_id UBIGINT,
  resolution_source TEXT,
  validation_state TEXT NOT NULL,
  confidence DOUBLE NOT NULL,
  reason TEXT,
  PRIMARY KEY (snapshot_id, ref_id)
);

CREATE INDEX idx_semantic_refs_name
ON semantic_references(snapshot_id, name);

CREATE INDEX idx_semantic_refs_resolved
ON semantic_references(snapshot_id, resolved_symbol_id);
```

### 9.7 Edges

```sql
CREATE TABLE semantic_edges (
  snapshot_id UBIGINT NOT NULL,
  edge_id UBIGINT NOT NULL,
  src_node_id UBIGINT NOT NULL,
  dst_node_id UBIGINT NOT NULL,
  src_kind TEXT NOT NULL,
  dst_kind TEXT NOT NULL,
  edge_kind TEXT NOT NULL,
  weight DOUBLE NOT NULL,
  confidence DOUBLE NOT NULL,
  validation_state TEXT NOT NULL,
  evidence_ref_id UBIGINT,
  evidence_file_id UBIGINT,
  source TEXT NOT NULL,
  reason TEXT,
  created_at TIMESTAMP NOT NULL,
  PRIMARY KEY (snapshot_id, edge_id)
);

CREATE INDEX idx_semantic_edges_src
ON semantic_edges(snapshot_id, src_node_id, edge_kind);

CREATE INDEX idx_semantic_edges_dst
ON semantic_edges(snapshot_id, dst_node_id, edge_kind);

CREATE INDEX idx_semantic_edges_kind
ON semantic_edges(snapshot_id, edge_kind);
```

### 9.8 Diagnostics

```sql
CREATE TABLE semantic_diagnostics (
  snapshot_id UBIGINT NOT NULL,
  diagnostic_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  file_id UBIGINT NOT NULL,
  language TEXT NOT NULL,
  severity TEXT NOT NULL,
  code TEXT,
  message TEXT NOT NULL,
  start_line INTEGER NOT NULL,
  start_col INTEGER NOT NULL,
  end_line INTEGER NOT NULL,
  end_col INTEGER NOT NULL,
  source TEXT,
  related_symbol_id UBIGINT,
  PRIMARY KEY (snapshot_id, diagnostic_id)
);
```

### 9.9 Graph Scores

```sql
CREATE TABLE semantic_graph_scores (
  repo_id TEXT NOT NULL,
  snapshot_id UBIGINT NOT NULL,
  graph_version UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  score_name TEXT NOT NULL,
  score DOUBLE NOT NULL,
  rank INTEGER,
  status TEXT NOT NULL, -- exact, approximate, stale, missing
  computed_at TIMESTAMP NOT NULL,
  algorithm_version TEXT NOT NULL,
  PRIMARY KEY (repo_id, graph_version, node_id, score_name)
);

CREATE INDEX idx_semantic_scores_snapshot
ON semantic_graph_scores(snapshot_id, score_name, status);
```

### 9.10 Clusters

```sql
CREATE TABLE semantic_clusters (
  repo_id TEXT NOT NULL,
  snapshot_id UBIGINT NOT NULL,
  graph_version UBIGINT NOT NULL,
  cluster_id UBIGINT NOT NULL,
  algorithm TEXT NOT NULL,
  label TEXT,
  summary TEXT,
  score DOUBLE,
  status TEXT NOT NULL, -- exact, approximate, stale, missing
  computed_at TIMESTAMP NOT NULL,
  PRIMARY KEY (repo_id, graph_version, cluster_id)
);

CREATE TABLE semantic_cluster_members (
  repo_id TEXT NOT NULL,
  graph_version UBIGINT NOT NULL,
  cluster_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  weight DOUBLE NOT NULL,
  role TEXT,
  PRIMARY KEY (repo_id, graph_version, cluster_id, node_id)
);
```

### 9.11 Live Overlay Tables

Overlay tables must be typed enough for hot queries. Optional JSON metadata is allowed but not used as the primary query path.

```sql
CREATE TABLE semantic_live_overlay_meta (
  repo_id TEXT PRIMARY KEY,
  base_snapshot_id UBIGINT NOT NULL,
  graph_version UBIGINT NOT NULL,
  freshness TEXT NOT NULL,
  overlay_file_count INTEGER NOT NULL,
  pending_lsp_count INTEGER NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
```

```sql
CREATE TABLE semantic_live_overlay_files (
  repo_id TEXT NOT NULL,
  path TEXT NOT NULL,
  file_id UBIGINT NOT NULL,
  content_hash TEXT,
  language TEXT,
  status TEXT NOT NULL, -- upserted, deleted
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (repo_id, path)
);
```

```sql
CREATE TABLE semantic_live_overlay_symbols (
  repo_id TEXT NOT NULL,
  symbol_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  file_id UBIGINT NOT NULL,
  stable_key TEXT NOT NULL,
  name TEXT NOT NULL,
  qualified_name TEXT NOT NULL,
  kind TEXT NOT NULL,
  status TEXT NOT NULL, -- upserted, deleted
  validation_state TEXT NOT NULL,
  confidence DOUBLE NOT NULL,
  fact_json JSON,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (repo_id, symbol_id)
);
```

```sql
CREATE TABLE semantic_live_overlay_references (
  repo_id TEXT NOT NULL,
  ref_id UBIGINT NOT NULL,
  node_id UBIGINT NOT NULL,
  file_id UBIGINT NOT NULL,
  name TEXT NOT NULL,
  ref_kind TEXT NOT NULL,
  resolved_symbol_id UBIGINT,
  status TEXT NOT NULL,
  validation_state TEXT NOT NULL,
  confidence DOUBLE NOT NULL,
  fact_json JSON,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (repo_id, ref_id)
);
```

```sql
CREATE TABLE semantic_live_overlay_edges (
  repo_id TEXT NOT NULL,
  edge_id UBIGINT NOT NULL,
  src_node_id UBIGINT NOT NULL,
  dst_node_id UBIGINT NOT NULL,
  edge_kind TEXT NOT NULL,
  status TEXT NOT NULL, -- upserted, deleted
  validation_state TEXT NOT NULL,
  confidence DOUBLE NOT NULL,
  weight DOUBLE NOT NULL,
  source TEXT NOT NULL,
  fact_json JSON,
  updated_at TIMESTAMP NOT NULL,
  PRIMARY KEY (repo_id, edge_id)
);

CREATE INDEX idx_overlay_edges_src
ON semantic_live_overlay_edges(repo_id, src_node_id, edge_kind);

CREATE INDEX idx_overlay_edges_dst
ON semantic_live_overlay_edges(repo_id, dst_node_id, edge_kind);
```

## 10. Effective Read Semantics

Every semantic query must read effective facts.

Effective files:

```sql
SELECT * FROM semantic_files WHERE snapshot_id = ?
EXCEPT deleted overlay rows
UNION ALL upserted overlay rows
```

Effective edges pseudocode:

```go
func QueryEffectiveEdges(ctx context.Context, req EdgeQuery) ([]EdgeFact, error) {
    base := QuerySnapshotEdges(req.SnapshotID, req.Filters)
    overlay := QueryOverlayEdges(req.RepoID, req.Filters)

    deleted := map[EdgeID]bool{}
    upserted := map[EdgeID]EdgeFact{}

    for _, e := range overlay {
        if e.Status == "deleted" {
            deleted[e.EdgeID] = true
            continue
        }
        upserted[e.EdgeID] = e.ToFact()
    }

    out := []EdgeFact{}
    for _, e := range base {
        if deleted[e.EdgeID] {
            continue
        }
        if replacement, ok := upserted[e.EdgeID]; ok {
            out = append(out, replacement)
            delete(upserted, e.EdgeID)
            continue
        }
        out = append(out, e)
    }

    for _, e := range upserted {
        out = append(out, e)
    }

    return out, nil
}
```

All MCP tool responses must include the effective graph version used.

## 11. Symbol Identity

### 11.1 Stable Symbol Key

Symbol identity must survive normal edits and simple renames where possible.

```go
type StableSymbolKey struct {
    RepoID          string
    Language        string
    PackagePath     string
    OwnerPath       string
    QualifiedName    string
    Kind            string
    SignatureHash    string
    LSPIdentity      string
    FilePathFallback string
}
```

Canonicalization rules:

```text
1. Prefer LSP identity if stable and available.
2. Else use package/module path + owner path + qualified name + kind + signature hash.
3. Use file path only as fallback disambiguator.
4. For same-content file rename, preserve file lineage and update FilePathFallback.
5. For moved exported symbols, preserve identity if package path and qualified name are unchanged.
```

Hash:

```go
func StableSymbolID(key StableSymbolKey) SymbolID {
    return SymbolID(xxhash64(CanonicalizeStableSymbolKey(key)))
}
```

### 11.2 Merge Rules

Tree-sitter and LSP facts may overlap.

Merge priority:

```text
1. Same LSP identity.
2. Same stable key.
3. Exact range + same name + compatible kind.
4. Same qualified name + same file + overlapping range.
5. Language-specific fallback resolver.
```

Confidence:

```text
1.00 LSP-confirmed symbol
0.95 tree-sitter + LSP merged
0.80 tree-sitter symbol validated by local resolver
0.70 tree-sitter only
0.45 heuristic only
```

Pseudocode:

```go
func MergeSymbols(ts []SymbolFact, lsp []SymbolFact) []SymbolFact {
    index := NewSymbolMergeIndex()

    for _, s := range ts {
        s.ExtractionSource = "tree_sitter"
        s.Confidence = max(s.Confidence, 0.70)
        index.Insert(s)
    }

    for _, ls := range lsp {
        candidate := index.FindMergeCandidate(ls)
        if candidate == nil {
            ls.ExtractionSource = "lsp"
            ls.Confidence = max(ls.Confidence, 1.00)
            index.Insert(ls)
            continue
        }

        merged := MergeSymbolFacts(*candidate, ls)
        merged.ExtractionSource = "merged"
        merged.Confidence = max(candidate.Confidence, 0.95)
        index.Replace(candidate.SymbolID, merged)
    }

    return index.AllDeterministic()
}
```

## 12. Edge Vocabulary

### 12.1 Core Edge Kinds

```text
CONTAINS
DEFINES
IMPORTS
EXPORTS
REFERENCES
RESOLVES_TO
CALLS
IMPLEMENTS
EXTENDS
OVERRIDES
USES_TYPE
HAS_PARAMETER
RETURNS_TYPE
READS
WRITES
ALLOCATES
INSTANTIATES
HAS_DIAGNOSTIC
MENTIONED_IN_MEMORY
TESTS
CONFIGURES
DEPENDS_ON
```

### 12.2 Security-Reserved Edge Kinds

Reserved but not required for initial release:

```text
SOURCE_OF
SINK_OF
VALIDATES
SANITIZES
AUTHORIZES
DATA_FLOWS_TO
TAINT_FLOWS_TO
REACHABLE_FROM_ENDPOINT
CONFIGURES_SECURITY_BOUNDARY
```

### 12.3 Edge Confidence

```text
1.00 confirmed by LSP
0.90 confirmed by multiple independent sources
0.75 tree-sitter structural edge
0.60 language-specific heuristic
0.40 unresolved but plausible edge
0.20 weak text/memory association
```

### 12.4 Edge Weight

```go
func EdgeWeight(e EdgeFact) float32 {
    base := BaseWeightByKind(e.EdgeKind)
    src := SourceWeight(e.Source)
    val := ValidationWeight(e.ValidationState)
    return float32(base * src * val * e.Confidence)
}
```

Base weights:

```text
CALLS                  1.00
IMPLEMENTS             0.95
OVERRIDES              0.95
EXTENDS                0.90
RESOLVES_TO            0.90
USES_TYPE              0.75
REFERENCES             0.60
IMPORTS                0.50
DEPENDS_ON             0.50
CONTAINS               0.30
MENTIONED_IN_MEMORY    0.20
```

Source weights:

```text
lsp.call_hierarchy     1.00
lsp.type_hierarchy     1.00
lsp.definition         1.00
lsp.references         0.95
tree_sitter            0.75
local_resolver         0.70
heuristic              0.50
memory                 0.30
text_search            0.20
```

Validation weights:

```text
validated              1.00
pending                0.80
skipped                0.70
invalidated            0.00
```

## 13. Extraction Layer

### 13.1 Initial Language Scope

Production-grade first-release providers:

```text
Go
TypeScript / JavaScript
Python
```

Other languages can be indexed with generic tree-sitter/LSP facts where available, but must be reported as best-effort coverage.

### 13.2 File Discovery

Ignore by default:

```text
.git/
node_modules/
vendor/ unless configured
dist/
build/
target/
coverage/
generated files above configured size
binary files
files above max_file_size
```

### 13.3 Language Provider

```go
type LanguageProvider interface {
    Language() string
    Extensions() []string
    TreeSitterLanguage() *sitter.Language
    Queries() QueryBundle
    ImportResolver() ImportResolver
    ScopeBuilder() ScopeBuilder
    SymbolNormalizer() SymbolNormalizer
    ReferenceClassifier() ReferenceClassifier
    SupportsLSPEnrichment() bool
}
```

### 13.4 Generic Capture Names

```text
@definition.function
@definition.method
@definition.class
@definition.struct
@definition.interface
@definition.enum
@definition.field
@definition.variable
@definition.constant
@reference.identifier
@reference.call
@reference.field
@import.source
@import.alias
@export.name
@type.annotation
@heritage.extends
@heritage.implements
```

### 13.5 Extraction Pseudocode

```go
func ExtractFile(ctx context.Context, provider LanguageProvider, file SourceFile) (*ExtractedFile, error) {
    tree, err := ParseTreeSitter(provider.TreeSitterLanguage(), file.Content)
    if err != nil {
        return PartialExtract(file, err), nil
    }

    captures := RunQueries(tree, provider.Queries())
    scopes := provider.ScopeBuilder().Build(tree, captures)

    symbols := ExtractSymbols(file, captures, scopes)
    refs := ExtractReferences(file, captures, scopes)
    imports := ExtractImports(file, captures)

    symbols = provider.SymbolNormalizer().Normalize(symbols)
    refs = provider.ReferenceClassifier().Classify(refs, scopes)

    syntaxEdges := BuildSyntaxEdges(symbols, refs, imports)

    return &ExtractedFile{
        File:        BuildFileFact(file),
        Symbols:     symbols,
        References:  refs,
        Imports:     imports,
        SyntaxEdges: syntaxEdges,
    }, nil
}
```

## 14. LSP Enrichment Layer

### 14.1 LSP Use Cases

Use LSP for:

```text
document symbols
hover/type info
reference resolution via go-to-definition on references
find references
find implementations
call hierarchy
type hierarchy
diagnostics
```

Avoid using go-to-definition on a definition as the primary enrichment path. Use it on references and call sites.

### 14.2 LSP Budget

```yaml
semantic_index:
  lsp_enrichment:
    enabled: true
    timeout_per_file: "5s"
    timeout_total: "120s"
    max_symbols_per_file: 200
    max_references_per_symbol: 1000
    max_references_per_file: 5000
    max_call_hierarchy_depth: 2
    max_type_hierarchy_depth: 2
    enrich_public_symbols_first: true
    enrich_pagerank_candidates_first: true
```

### 14.3 Enrichment Priority

```text
1. File currently requested by an agent.
2. File changed by Helix edit tool.
3. Exported/public symbols.
4. Symbols with many tree-sitter references.
5. Entry-point-like symbols.
6. Symbols in high-ranked files.
7. Symbols with diagnostics.
8. Remaining symbols until budget exhausted.
```

### 14.4 Enrichment Pseudocode

```go
func EnrichWithLSP(ctx context.Context, file FileFact, symbols []SymbolFact, refs []ReferenceFact, budget LSPBudget) (*LSPEnrichResult, error) {
    result := &LSPEnrichResult{}

    docSymbols := TryDocumentSymbols(ctx, file)
    if docSymbols.OK {
        result.Symbols = append(result.Symbols, ConvertDocumentSymbols(docSymbols)...)
    }

    diagnostics := TryDiagnostics(ctx, file)
    result.Diagnostics = diagnostics

    orderedSymbols := PrioritizeSymbols(symbols, refs, budget)
    for _, sym := range orderedSymbols {
        if budget.Exhausted() || ctx.Err() != nil {
            result.Partial = true
            result.PartialReason = "budget exhausted"
            break
        }

        hover := TryHover(ctx, sym.Location())
        if hover.OK {
            result.Symbols = append(result.Symbols, EnrichSymbolType(sym, hover))
        }

        calls := TryCallHierarchy(ctx, sym.Location(), budget.CallDepth)
        result.Edges = append(result.Edges, BuildCallEdges(sym, calls)...)

        types := TryTypeHierarchy(ctx, sym.Location(), budget.TypeDepth)
        result.Edges = append(result.Edges, BuildTypeEdges(sym, types)...)

        impls := TryImplementations(ctx, sym.Location())
        result.Edges = append(result.Edges, BuildImplementationEdges(sym, impls)...)
    }

    orderedRefs := PrioritizeReferences(refs, budget)
    for _, ref := range orderedRefs {
        if budget.Exhausted() || ctx.Err() != nil {
            result.Partial = true
            result.PartialReason = "budget exhausted"
            break
        }

        def := TryGoToDefinition(ctx, ref.Location())
        if def.OK {
            target := ResolveLocationToSymbol(def.Location)
            result.References = append(result.References, MarkResolved(ref, target, "lsp.definition", 1.0))
            result.Edges = append(result.Edges, EdgeFact{
                SrcNodeID:       ref.NodeID,
                DstNodeID:       target.NodeID,
                EdgeKind:        "RESOLVES_TO",
                Source:          "lsp.definition",
                ValidationState: "validated",
                Confidence:      1.0,
                Weight:          1.0,
                EvidenceRefID:   ref.RefID,
            })
        }
    }

    return result, nil
}
```

## 15. Indexing Pipeline

### 15.1 Full/Incremental Snapshot Pipeline

```text
activate workspace
  → detect files
  → compute worktree hash
  → compare latest committed snapshot
  → plan full or incremental snapshot
  → parse files with tree-sitter
  → extract structural facts
  → enrich selected files with LSP
  → merge facts
  → build edges
  → compute graph scores
  → compute clusters
  → commit snapshot
  → initialize or refresh graph cache
```

### 15.2 Plan Algorithm

```go
func BuildIndexPlan(ctx context.Context, repo RepoState, latest *SnapshotMeta) (*IndexPlan, error) {
    files := DiscoverIndexableFiles(repo.Root)
    current := ComputeWorktreeManifest(files)

    if latest == nil {
        return FullPlan(files, "no previous snapshot"), nil
    }

    if latest.SchemaVersion != CurrentSchemaVersion {
        return FullPlan(files, "schema version changed"), nil
    }

    if RequiresFullReindex(latest.IndexerVersion, CurrentIndexerVersion) {
        return FullPlan(files, "indexer version requires full reindex"), nil
    }

    delta := DiffManifest(latest.Manifest, current)

    if delta.ChangeRatio() > Config.FullReindexChangeRatio {
        return FullPlan(files, "change ratio above threshold"), nil
    }

    return IncrementalPlan(latest.SnapshotID, delta), nil
}
```

### 15.3 Snapshot Commit Rules

```text
1. A snapshot is invisible to normal queries until committed.
2. Commit must be atomic from the perspective of semantic tools.
3. Failed commit must preserve the previous committed snapshot.
4. Partial LSP enrichment may still commit if structural facts are complete.
5. Snapshot summary records partial reason and coverage.
```

## 16. Live Update Pipeline

### 16.1 Change Event Types

```go
type SourceChangeKind string

const (
    ChangeFileCreated  SourceChangeKind = "file_created"
    ChangeFileModified SourceChangeKind = "file_modified"
    ChangeFileDeleted  SourceChangeKind = "file_deleted"
    ChangeFileRenamed  SourceChangeKind = "file_renamed"
    ChangeGitCheckout  SourceChangeKind = "git_checkout"
    ChangeBulkUpdate   SourceChangeKind = "bulk_update"
    ChangeHelixEdit    SourceChangeKind = "helix_edit"
    ChangeDiagnostics  SourceChangeKind = "diagnostics_changed"
)

type SourceChangeEvent struct {
    RepoID      string
    Kind        SourceChangeKind
    Path        string
    OldPath     string
    ContentHash string
    Source      string
    ObservedAt  time.Time
}
```

### 16.2 Debounce and Coalescing

Rules:

```text
modified + modified          → modified
created + modified           → created
created + deleted            → no-op
modified + deleted           → deleted
deleted + created same path  → modified
rename old→new               → renamed
many changes above threshold → bulk_update
```

Pseudocode:

```go
func CoalesceEvents(events []SourceChangeEvent) []SourceChangeEvent {
    byPath := map[string]SourceChangeEvent{}

    for _, ev := range events {
        key := ev.Path
        if ev.Kind == ChangeFileRenamed {
            key = ev.OldPath + "→" + ev.Path
        }

        prev, ok := byPath[key]
        if !ok {
            byPath[key] = ev
            continue
        }

        merged, keep := MergeChange(prev, ev)
        if !keep {
            delete(byPath, key)
            continue
        }
        byPath[key] = merged
    }

    result := Values(byPath)
    if len(result) > Config.LiveUpdates.BulkChangeThreshold {
        return []SourceChangeEvent{{
            RepoID: result[0].RepoID,
            Kind:   ChangeBulkUpdate,
            Source: "coalescer",
        }}
    }

    return SortEventsDeterministic(result)
}
```

### 16.3 Changed File Update

```go
func UpdateChangedFile(ctx context.Context, ev SourceChangeEvent) (*LiveUpdateResult, error) {
    oldFacts := Store.LoadEffectiveFileFacts(ctx, ev.RepoID, ev.Path)

    content, err := ReadWorkspaceFileChecked(ev.RepoID, ev.Path)
    if err != nil {
        return nil, err
    }

    provider := LanguageProviderForPath(ev.Path)
    extracted, err := ExtractFile(ctx, provider, SourceFile{
        RepoID:      ev.RepoID,
        Path:        ev.Path,
        Content:     content,
        ContentHash: Hash(content),
    })
    if err != nil {
        return nil, err
    }

    diff := DiffFileFacts(oldFacts, extracted)
    repair := ComputeGraphRepair(diff)

    tx, err := Store.BeginOverlayTx(ctx, ev.RepoID)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    if err := ApplyFileDiffToOverlay(tx, extracted, diff); err != nil {
        return nil, err
    }

    if err := tx.WriteInvalidations(repair.Invalidations); err != nil {
        return nil, err
    }

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    GraphCache.ApplyRepair(repair)
    MarkAffectedScoresAndClusters(repair)

    LSPQueue.Enqueue(RevalidateFileJob{
        RepoID: ev.RepoID,
        Path:   ev.Path,
        Reason: string(ev.Kind),
    })

    return BuildLiveUpdateResult(diff, repair), nil
}
```

### 16.4 File Delete

```go
func HandleFileDeleted(ctx context.Context, ev SourceChangeEvent) error {
    facts := Store.LoadEffectiveFileFacts(ctx, ev.RepoID, ev.Path)

    tx, err := Store.BeginOverlayTx(ctx, ev.RepoID)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    tx.MarkFileDeleted(ev.Path)
    tx.MarkSymbolsDeleted(facts.SymbolIDs())
    tx.MarkReferencesDeleted(facts.ReferenceIDs())
    tx.MarkEdgesDeleted(facts.EdgeIDs())
    tx.MarkDiagnosticsDeleted(facts.DiagnosticIDs())

    affected := ComputeDeletedFileInvalidation(facts)
    tx.WriteInvalidations(affected)

    if err := tx.Commit(); err != nil {
        return err
    }

    GraphCache.RemoveFileFacts(facts)
    MarkAffectedScoresAndClusters(affected)
    return nil
}
```

### 16.5 File Rename

```go
func HandleFileRenamed(ctx context.Context, ev SourceChangeEvent) error {
    oldFacts := Store.LoadEffectiveFileFacts(ctx, ev.RepoID, ev.OldPath)

    if FileExists(ev.Path) && HashFile(ev.Path) == oldFacts.File.ContentHash {
        tx, err := Store.BeginOverlayTx(ctx, ev.RepoID)
        if err != nil {
            return err
        }
        defer tx.Rollback()

        moved := RebaseFileFactsForRename(oldFacts, ev.OldPath, ev.Path)
        tx.MarkFileDeleted(ev.OldPath)
        tx.UpsertFile(moved.File)
        tx.UpsertSymbols(moved.Symbols)
        tx.UpsertReferences(moved.References)
        tx.UpsertEdges(moved.Edges)

        if err := tx.Commit(); err != nil {
            return err
        }

        GraphCache.ApplyRepair(ComputeRenameRepair(oldFacts, moved))
        return nil
    }

    if err := HandleFileDeleted(ctx, SourceChangeEvent{RepoID: ev.RepoID, Kind: ChangeFileDeleted, Path: ev.OldPath}); err != nil {
        return err
    }

    return UpdateChangedFile(ctx, SourceChangeEvent{RepoID: ev.RepoID, Kind: ChangeFileCreated, Path: ev.Path})
}
```

## 17. Graph Cache and Repair

### 17.1 Graph Cache

```go
type LiveGraphCache struct {
    RepoID        string
    BaseSnapshot  SnapshotID
    GraphVersion  GraphVersion

    Nodes map[NodeID]NodeState
    Out   map[NodeID][]GraphEdge
    In    map[NodeID][]GraphEdge

    DirtyNodes      Bitmap
    DirtyComponents Bitmap
    DirtyScores     map[ScoreName]bool
    DirtyClusters   map[ClusterID]bool

    mu sync.RWMutex
}
```

### 17.2 Repair Rules

A changed symbol invalidates:

```text
outgoing edges from symbol
incoming edges to symbol if stable key/signature/exported status changed
references inside symbol body
call edges from symbol body
type-use edges in symbol signature
diagnostics associated with symbol
cluster membership for affected component
PageRank scores for affected projections
```

```go
func ComputeGraphRepair(diff FileFactDiff) GraphRepair {
    repair := GraphRepair{}

    for _, sym := range diff.RemovedSymbols {
        repair.RemoveNode(sym.NodeID)
        repair.InvalidateIncoming(sym.NodeID)
        repair.InvalidateOutgoing(sym.NodeID)
    }

    for _, sym := range diff.ChangedSymbols {
        if sym.SignatureChanged || sym.ExportedChanged || sym.KindChanged || sym.StableKeyChanged {
            repair.InvalidateIncoming(sym.NodeID)
        }
        repair.InvalidateOutgoing(sym.NodeID)
        repair.MarkNodeDirty(sym.NodeID)
    }

    for _, ref := range diff.ChangedReferences {
        repair.InvalidateEdgesByReference(ref.RefID)
        if ref.ScopeSymbolID != 0 {
            repair.MarkSymbolDirty(ref.ScopeSymbolID)
        }
    }

    for _, edge := range diff.AddedEdges {
        repair.AddOrUpdateEdge(edge)
        repair.MarkNodeDirty(edge.SrcNodeID)
        repair.MarkNodeDirty(edge.DstNodeID)
    }

    return repair
}
```

### 17.3 Applying Repair

```go
func (g *LiveGraphCache) ApplyRepair(repair GraphRepair) GraphVersion {
    g.mu.Lock()
    defer g.mu.Unlock()

    for _, n := range repair.RemovedNodes {
        g.RemoveNode(n)
    }

    for _, e := range repair.RemovedEdges {
        g.RemoveEdge(e)
    }

    for _, e := range repair.UpsertedEdges {
        g.UpsertEdge(e)
    }

    for _, n := range repair.DirtyNodes {
        g.DirtyNodes.Add(n)
    }

    g.GraphVersion++
    return g.GraphVersion
}
```

## 18. PageRank and Ranking

### 18.1 Projections

Compute separate projections:

```text
CALL_GRAPH_PAGERANK
REFERENCE_PAGERANK
FILE_DEPENDENCY_PAGERANK
TYPE_HIERARCHY_PAGERANK
CHANGE_IMPACT_PAGERANK
SECURITY_SURFACE_PAGERANK
RETRIEVAL_CONTEXT_PAGERANK
```

### 18.2 Weighted PageRank

```text
PR(v) = (1 - d) / N + d * Σ(PR(u) * w(u,v) / out_weight(u))
```

```go
func WeightedPageRank(g *Graph, cfg PageRankConfig) []float64 {
    n := len(g.Nodes)
    if n == 0 {
        return nil
    }

    pr := UniformVector(n)
    next := make([]float64, n)
    outWeight := ComputeOutWeights(g)
    teleport := (1.0 - cfg.Damping) / float64(n)

    for iter := 0; iter < cfg.MaxIterations; iter++ {
        Fill(next, teleport)
        danglingMass := 0.0

        for u, edges := range g.Out {
            if outWeight[u] == 0 {
                danglingMass += pr[u]
                continue
            }
            for _, e := range edges {
                v := g.Index[e.To]
                next[v] += cfg.Damping * pr[u] * float64(e.Weight) / outWeight[u]
            }
        }

        DistributeUniform(next, cfg.Damping*danglingMass/float64(n))

        delta := L1Distance(pr, next)
        copy(pr, next)
        if delta < cfg.Epsilon {
            break
        }
    }

    return pr
}
```

### 18.3 Personalized PageRank

```go
func PersonalizedPageRank(g *Graph, seeds []PersonalizedSeed, cfg PageRankConfig) []float64 {
    n := len(g.Nodes)
    if n == 0 {
        return nil
    }

    teleport := BuildTeleportVector(g, seeds)
    pr := UniformVector(n)
    next := make([]float64, n)

    for iter := 0; iter < cfg.MaxIterations; iter++ {
        for i := range next {
            next[i] = (1.0 - cfg.Damping) * teleport[i]
        }

        for u, edges := range g.Out {
            if len(edges) == 0 {
                DistributeDangling(next, pr[u], teleport, cfg.Damping)
                continue
            }
            sum := SumWeights(edges)
            for _, e := range edges {
                v := g.Index[e.To]
                next[v] += cfg.Damping * pr[u] * float64(e.Weight) / sum
            }
        }

        delta := L1Distance(pr, next)
        copy(pr, next)
        if delta < cfg.Epsilon {
            break
        }
    }

    return pr
}
```

### 18.4 Incremental PageRank Repair

Use three levels:

```text
Level 1: immediately mark affected scores stale.
Level 2: local approximate repair over bounded k-hop neighborhood.
Level 3: full exact recompute after idle or explicit indexing.
```

```go
func RepairPageRankAfterChange(ctx context.Context, g *LiveGraphCache, dirty []NodeID) {
    affected := ExpandNeighborhood(g, dirty, 2, Config.MaxLocalPageRankNodes)

    if len(affected) > Config.MaxLocalPageRankNodes {
        Store.MarkProjectionStale(ctx, g.RepoID, "CALL_GRAPH_PAGERANK", g.GraphVersion)
        Scheduler.ScheduleFullPageRankAfterIdle(g.RepoID)
        return
    }

    subgraph := g.Subgraph(affected)
    seeds := BuildSeeds(dirty)
    scores := PersonalizedPageRank(subgraph, seeds, PageRankConfig{
        Damping:       0.85,
        Epsilon:       0.00001,
        MaxIterations: 30,
    })

    Store.UpsertApproxScores(ctx, g.RepoID, g.GraphVersion, scores)
}
```

### 18.5 Score Fusion

```text
score(node, task) =
    0.30 * personalized_pagerank
  + 0.20 * direct_match_score
  + 0.15 * symbol_importance_score
  + 0.15 * dependency_proximity_score
  + 0.10 * diagnostic_relevance_score
  + 0.10 * memory_relevance_score
```

```go
func FuseScores(features NodeFeatures, weights ScoreWeights) float64 {
    return weights.PersonalizedPageRank * features.PersonalizedPageRank +
        weights.DirectMatch * features.DirectMatch +
        weights.SymbolImportance * features.SymbolImportance +
        weights.DependencyProximity * features.DependencyProximity +
        weights.DiagnosticRelevance * features.DiagnosticRelevance +
        weights.MemoryRelevance * features.MemoryRelevance
}
```

## 19. Clustering

### 19.1 Strategy

```text
1. Build graph projection.
2. Remove low-confidence edges.
3. Compute weakly connected components.
4. Split huge components with deterministic label propagation.
5. Rank nodes inside cluster using PageRank.
6. Label cluster using top files/symbols/package prefixes.
7. Mark clusters exact, approximate, stale, or missing.
```

### 19.2 Weak Components

```go
func WeakComponents(g *Graph) [][]NodeID {
    visited := make([]bool, len(g.Nodes))
    comps := [][]NodeID{}

    for i := range g.Nodes {
        if visited[i] {
            continue
        }
        comp := []NodeID{}
        queue := []int{i}
        visited[i] = true

        for len(queue) > 0 {
            u := queue[0]
            queue = queue[1:]
            comp = append(comp, g.Nodes[u])

            for _, e := range g.Out[u] {
                v := g.Index[e.To]
                if !visited[v] {
                    visited[v] = true
                    queue = append(queue, v)
                }
            }
            for _, e := range g.In[u] {
                v := g.Index[e.To]
                if !visited[v] {
                    visited[v] = true
                    queue = append(queue, v)
                }
            }
        }

        comps = append(comps, comp)
    }

    return comps
}
```

### 19.3 Label Propagation

```go
func LabelPropagation(g *Graph, nodes []NodeID, cfg ClusterConfig) map[NodeID]int {
    labels := map[NodeID]int{}
    for i, n := range nodes {
        labels[n] = i
    }

    order := DeterministicShuffle(nodes, cfg.Seed)

    for iter := 0; iter < cfg.MaxIterations; iter++ {
        changed := false
        for _, node := range order {
            idx := g.Index[node]
            scores := map[int]float64{}

            for _, e := range g.Out[idx] {
                scores[labels[e.To]] += float64(e.Weight)
            }
            for _, e := range g.In[idx] {
                scores[labels[e.To]] += float64(e.Weight)
            }

            best := ArgMaxLabelDeterministic(scores)
            if best != labels[node] {
                labels[node] = best
                changed = true
            }
        }
        if !changed {
            break
        }
    }

    return labels
}
```

### 19.4 Incremental Cluster Repair

```go
func RepairClustersAfterChange(ctx context.Context, g *LiveGraphCache, diff FileFactDiff) {
    affectedClusters := Store.FindClustersByNodes(diff.AffectedNodeIDs())
    affectedSize := Store.ClusterMemberCount(affectedClusters)

    if affectedSize > Config.MaxIncrementalClusterRepairNodes {
        Store.MarkClustersStale(affectedClusters)
        Scheduler.ScheduleClusterRecomputeAfterIdle(g.RepoID)
        return
    }

    for _, clusterID := range affectedClusters {
        nodes := Store.LoadClusterNeighborhood(clusterID)
        labels := LabelPropagation(g.AsGraph(), nodes, ClusterConfig{
            MaxIterations: 5,
            Seed: StableClusterSeed(clusterID),
        })
        Store.UpdateClusterMembers(clusterID, labels, ScoreApproximate)
        Store.RelabelCluster(clusterID)
    }
}
```

## 20. Retrieval Engine

### 20.1 Context Retrieval Modes

```text
global_repo_overview
task_context
symbol_explanation
change_impact
cluster_explanation
security_surface
diagnostic_context
```

### 20.2 Retrieval Pipeline

```text
query/task
  → determine freshness mode
  → collect seeds
  → expand graph neighborhood
  → personalized PageRank
  → fuse graph/text/memory/diagnostic signals
  → select symbols under token budget
  → return evidence-backed context with freshness metadata
```

### 20.3 Freshness Modes

```go
type FreshnessMode string

const (
    FreshnessAllowStale     FreshnessMode = "allow_stale"
    FreshnessRequireCurrent FreshnessMode = "require_current"
    FreshnessValidateLive   FreshnessMode = "validate_live"
)
```

Tool defaults:

```text
get_semantic_context      allow structurally fresh + report pending LSP
explain_symbol_deep       validate live for requested symbol
get_change_impact_graph   validate live for target + direct neighbors
get_cluster_map           allow approximate scores + report status
validate_graph_edge       always revalidate live
```

### 20.4 Seed Collection

```go
func CollectSeeds(ctx context.Context, req ContextRequest) ([]PersonalizedSeed, error) {
    seeds := []PersonalizedSeed{}

    for _, path := range req.Files {
        file := Store.LookupEffectiveFile(path)
        seeds = append(seeds, PersonalizedSeed{NodeID: file.NodeID, Weight: 1.0})
    }

    for _, name := range ExtractSymbolNames(req.Task) {
        syms := Store.QueryEffectiveSymbols(SymbolQuery{Name: name})
        for _, s := range syms {
            seeds = append(seeds, PersonalizedSeed{NodeID: s.NodeID, Weight: 0.8})
        }
    }

    memoryHits := Memory.Search(req.Task)
    for _, hit := range memoryHits {
        linked := Store.NodesMentionedInMemory(hit.MemoryID)
        for _, n := range linked {
            seeds = append(seeds, PersonalizedSeed{NodeID: n, Weight: 0.4})
        }
    }

    diagnostics := Store.RelevantDiagnostics(req.Task)
    for _, d := range diagnostics {
        if d.RelatedSymbolNodeID != 0 {
            seeds = append(seeds, PersonalizedSeed{NodeID: d.RelatedSymbolNodeID, Weight: 0.6})
        }
    }

    return NormalizeSeeds(seeds), nil
}
```

### 20.5 Token-Budgeted Selection

```go
func SelectContext(candidates []ContextCandidate, budget TokenBudget) []ContextCandidate {
    sort.SliceStable(candidates, func(i, j int) bool {
        if candidates[i].Score == candidates[j].Score {
            return candidates[i].StableKey < candidates[j].StableKey
        }
        return candidates[i].Score > candidates[j].Score
    })

    selected := []ContextCandidate{}
    used := 0

    for _, c := range candidates {
        cost := EstimateTokens(c.RenderedPreview)
        if used+cost > budget.MaxTokens {
            if c.CanElide {
                elided := c.Elide(budget.MaxTokens - used)
                if elided.TokenCost > 0 {
                    selected = append(selected, elided)
                    used += elided.TokenCost
                }
            }
            continue
        }
        selected = append(selected, c)
        used += cost
    }

    return selected
}
```

## 21. LSP Revalidation Queue

```go
type RevalidateFileJob struct {
    RepoID     string
    Path       string
    Priority   int
    Reason     string
    EnqueuedAt time.Time
}

type LSPRevalidationQueue interface {
    Enqueue(job RevalidateFileJob)
    BoostFiles(paths []string)
    Run(ctx context.Context)
}
```

Priority:

```text
1. File edited by Helix tool.
2. File currently requested by agent.
3. File with diagnostics.
4. Exported/public API file.
5. Normal file watcher update.
```

```go
func RevalidateFile(ctx context.Context, job RevalidateFileJob) error {
    facts := Store.LoadEffectiveFileFacts(ctx, job.RepoID, job.Path)

    result, err := LSPEnricher.EnrichFile(ctx, LSPEnrichRequest{
        RepoID:     job.RepoID,
        File:       facts.File,
        Symbols:    facts.Symbols,
        References: facts.References,
        Budget:     Config.LiveLSPBudget,
    })
    if err != nil {
        Store.MarkFileSemanticPending(job.RepoID, job.Path, err.Error())
        return nil
    }

    diff := DiffLSPFacts(facts, result)
    repair := ComputeGraphRepairFromLSPDiff(diff)

    tx, err := Store.BeginOverlayTx(ctx, job.RepoID)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    tx.UpsertSymbols(MarkValidatedSymbols(diff.Symbols))
    tx.UpsertReferences(MarkValidatedReferences(diff.References))
    tx.UpsertEdges(MarkValidatedEdges(diff.Edges))
    tx.UpsertDiagnostics(result.Diagnostics)
    tx.WriteInvalidations(repair.Invalidations)

    if err := tx.Commit(); err != nil {
        return err
    }

    GraphCache.ApplyRepair(repair)
    Store.MarkFileSemanticallyFresh(job.RepoID, job.Path)
    return nil
}
```

## 22. Overlay Compaction

### 22.1 Compaction Triggers

Compact overlay when:

```text
overlay has changes
no file changes for compact_after_idle_ms
no active edit transaction
no active overlay transaction
LSP queue is empty or lsp_compaction_max_wait elapsed
graph cache version is stable
```

### 22.2 Compaction Pseudocode

```go
func CompactOverlay(ctx context.Context, repoID string) error {
    if !CanCompact(repoID) {
        return nil
    }

    lock := CompactionLocks.Acquire(repoID)
    defer lock.Release()

    base := Store.GetLatestSnapshot(ctx, repoID)
    overlay := Store.LoadOverlay(ctx, repoID)

    newSnapshot, err := Store.BeginSnapshot(ctx, SnapshotMeta{
        RepoID:         repoID,
        BaseSnapshotID: base.SnapshotID,
        Kind:           "live_compaction",
    })
    if err != nil {
        return err
    }

    merged := MergeBaseAndOverlay(base, overlay)

    if err := Store.WriteSnapshotFacts(ctx, newSnapshot, merged); err != nil {
        Store.AbortSnapshot(ctx, newSnapshot, err.Error())
        return err
    }

    if overlay.HasApproxScores || overlay.HasStaleScores {
        RecomputeExactScores(ctx, newSnapshot)
    }

    if overlay.HasDirtyClusters {
        RecomputeClusters(ctx, newSnapshot)
    }

    if err := Store.CommitSnapshot(ctx, newSnapshot, BuildSnapshotSummary(merged)); err != nil {
        Store.AbortSnapshot(ctx, newSnapshot, err.Error())
        return err
    }

    Store.ClearOverlay(ctx, repoID)
    GraphCache.SetBaseSnapshot(repoID, newSnapshot)
    return nil
}
```

## 23. MCP Tools

### 23.1 `index_semantic_graph`

Build or update committed semantic snapshot.

Mode: `review+`, `admin`.

Input:

```json
{
  "mode": "auto | full | incremental | refresh",
  "include_lsp": true,
  "include_graph_scores": true,
  "include_clusters": true,
  "max_duration_ms": 120000
}
```

Output:

```json
{
  "snapshot_id": "12345",
  "graph_version": 17,
  "mode": "incremental",
  "files_indexed": 42,
  "files_reused": 1200,
  "symbols": 18420,
  "references": 88210,
  "edges": 104502,
  "partial": false,
  "freshness": "fresh",
  "duration_ms": 18500
}
```

### 23.2 `refresh_semantic_graph`

Apply pending live source changes without forcing full reindex.

Mode: `read+`.

Input:

```json
{
  "wait_for_lsp": false,
  "max_wait_ms": 3000,
  "paths": ["internal/auth/middleware.go"]
}
```

Output:

```json
{
  "graph_version": 185,
  "files_updated": 1,
  "symbols_added": 2,
  "symbols_removed": 1,
  "edges_added": 5,
  "edges_removed": 3,
  "pending_lsp": true,
  "freshness": "structurally_fresh_semantically_pending"
}
```

### 23.3 `get_semantic_graph_status`

Mode: `read+`.

Output:

```json
{
  "enabled": true,
  "latest_snapshot_id": "12345",
  "graph_version": 184,
  "overlay_active": true,
  "overlay_files": 7,
  "pending_lsp_revalidations": 3,
  "freshness": "structurally_fresh_semantically_pending",
  "score_status": {
    "CALL_GRAPH_PAGERANK": "approximate",
    "REFERENCE_PAGERANK": "stale",
    "FILE_DEPENDENCY_PAGERANK": "exact"
  },
  "cluster_status": "partially_stale",
  "last_live_update_ms": 42
}
```

### 23.4 `get_semantic_context`

Mode: `read+`.

Input:

```json
{
  "task": "Find code relevant to authentication middleware",
  "files": ["internal/auth/middleware.go"],
  "symbols": ["AuthMiddleware"],
  "mode": "task_context",
  "freshness_mode": "allow_stale | require_current | validate_live",
  "max_tokens": 8000,
  "include_evidence": true
}
```

Every response must include:

```json
{
  "snapshot_id": "12345",
  "graph_version": 184,
  "overlay_active": true,
  "freshness": "structurally_fresh_semantically_pending",
  "approximate_scores": true,
  "pending_lsp_files": 3,
  "context": []
}
```

### 23.5 `explain_symbol_deep`

Mode: `read+`.

Defaults to `freshness_mode=validate_live` for the requested symbol.

### 23.6 `find_related_symbols`

Mode: `read+`.

Supports relationship filters, direction, max depth, freshness mode, and limit.

### 23.7 `get_cluster_map`

Mode: `read+`.

Allows approximate clusters but must report cluster status.

### 23.8 `explain_cluster`

Mode: `read+`.

Returns cluster label, summary, top symbols, top files, incoming/outgoing dependencies, score status, and freshness.

### 23.9 `get_change_impact_graph`

Mode: `review+`.

Must validate target symbol and direct neighbors when possible.

### 23.10 `validate_graph_edge`

Mode: `review+`.

Always performs live validation when LSP supports it.

## 24. Integration With Existing Tools

### 24.1 `get_repo_map`

If semantic index exists:

```text
use persisted graph scores, clusters, and effective overlay state
```

If missing or disabled:

```text
fall back to current tree-sitter tags + PageRank behavior
```

### 24.2 `get_context`

If semantic index exists:

```text
delegate internally to semantic retrieval engine
```

If missing:

```text
use existing dependency graph analysis
```

### 24.3 `analyze_blast_radius`

Enhance with:

```text
persistent graph expansion
live overlay state
LSP validation of critical edges
confidence and evidence in output
```

### 24.4 Edit Tools

After successful Helix edit tools:

```text
write file
  → emit ChangeHelixEdit event
  → update live overlay immediately
  → repair graph cache
  → run diagnostics/verify_edit if tool requires it
  → schedule high-priority LSP revalidation
```

```go
func AfterSuccessfulEdit(ctx context.Context, edit EditResult) {
    for _, path := range edit.ChangedFiles {
        LiveUpdateQueue.Enqueue(SourceChangeEvent{
            RepoID: edit.RepoID,
            Kind:   ChangeHelixEdit,
            Path:   path,
            Source: "helix_edit",
        })
    }
    LSPQueue.BoostFiles(edit.ChangedFiles)
}
```

### 24.5 `get_health`

Add semantic section:

```json
{
  "semantic_index": {
    "enabled": true,
    "store": "duckdb",
    "latest_snapshot_status": "ready",
    "graph_version": 184,
    "overlay_active": true,
    "pending_lsp_revalidations": 3,
    "last_live_update_ms": 42,
    "last_error": null
  }
}
```

## 25. Configuration

```yaml
semantic_index:
  enabled: true

  store:
    kind: duckdb
    path: ".helix/semantic.duckdb"
    memory_limit: "1GiB"
    threads: 4

  indexing:
    mode: lazy
    auto_index_on_activate: false
    max_file_size: "2MiB"
    max_files: 200000
    full_reindex_change_ratio: 0.25
    snapshot_retention: 5
    include_generated: false
    required_for_readyz: false

  extraction:
    extraction_ready_timeout: "30s"
    extraction_file_timeout: "3s"
    max_parallel_files: 4
    allow_partial_results: true

  live_updates:
    enabled: true
    debounce_ms: 250
    max_batch_delay_ms: 1500
    bulk_change_threshold: 200
    compact_after_idle_ms: 5000
    lsp_revalidate_after_idle_ms: 750
    lsp_compaction_max_wait_ms: 3000
    max_overlay_files: 1000
    max_overlay_age: "30m"

  lsp_enrichment:
    enabled: true
    timeout_per_file: "5s"
    timeout_total: "120s"
    max_symbols_per_file: 200
    max_references_per_symbol: 1000
    max_references_per_file: 5000
    max_call_hierarchy_depth: 2
    max_type_hierarchy_depth: 2

  graph:
    min_edge_confidence: 0.50
    max_loaded_nodes: 1000000
    max_loaded_edges: 5000000
    max_local_pagerank_nodes: 5000
    max_incremental_cluster_repair_nodes: 10000

  pagerank:
    damping: 0.85
    epsilon: 0.000001
    max_iterations: 100

  clustering:
    enabled: true
    max_component_size_before_split: 5000
    label_propagation_iterations: 20

  retrieval:
    default_max_tokens: 8000
    include_evidence_by_default: true

  guardrails:
    enabled: true
    enforcement: "warn" # off, warn, require_force, enforce
    require_impact_for_public_api_edit: true
    require_references_before_rename: true
    require_references_before_delete: true
    require_verify_after_edit: true
    stale_graph_policy: "warn" # allow, warn, block_for_review

  eval:
    enabled: true
    default_modes: ["baseline", "native", "semantic", "semantic_guarded"]
    output_dir: ".helix/eval"
    track_costs: true
    track_tool_behavior: true
    redact_source_in_reports: false

  type_resolution:
    enabled: true
    max_chain_depth: 8
    max_fixpoint_iterations: 8
    min_confidence_for_edge: 0.45
    comment_fallbacks: true
    emit_unresolved_edges: true

  phase_graph:
    enabled: true
    validate_on_startup: true
    fail_on_cycle: true
    dump_dot_on_error: true
```

> **Note on `extraction.*` vs `indexing.*` (Phase 59 P02 user decision).**
> `max_file_size` and `auto_index_on_activate` (the file-size cap and the
> initial-extraction-on-activation toggle) remain under `indexing.*` —
> Phase 59 reuses Phase 57's existing keys rather than duplicating them
> under `extraction.*`. The new `extraction.*` sub-tree carries only the
> four genuinely-new keys above (`extraction_ready_timeout`,
> `extraction_file_timeout`, `max_parallel_files`, `allow_partial_results`).

## 26. Operational Behavior

### 26.1 Lazy Initialization

Do not fully index on daemon start by default.

Create semantic index only when:

```text
user calls index_semantic_graph
user calls a semantic tool requiring it
config enables auto_index_on_activate
```

Live watcher may start on workspace activation if `live_updates.enabled=true`, but it must not force full indexing unless configured.

### 26.2 Stale and Pending States

If worktree changed after latest committed snapshot but overlay has not caught up:

```text
freshness = stale
```

If overlay has structurally processed the changes but LSP has not validated:

```text
freshness = structurally_fresh_semantically_pending
```

If LSP validates some but not all changed files:

```text
freshness = partially_validated
```

### 26.3 Large Repositories

For huge repos:

```text
limit graph projections
load subgraphs for task context
skip exact full PageRank if graph exceeds limit
mark scores stale/missing with reason
never crash due to graph load size
```

## 27. Git and External Changes

### 27.1 Git Checkout / Branch Switch

Detection:

```text
.git/HEAD changed
.git/index changed
many file events in short window
worktree hash mismatch
```

Behavior:

```go
func HandleGitChange(ctx context.Context, repoID string) {
    delta := ComputeWorktreeDelta(repoID)

    if delta.FileChangeCount > Config.LiveUpdates.BulkChangeThreshold {
        Store.ClearOverlay(ctx, repoID)
        Scheduler.ScheduleIndex(IndexRequest{
            RepoID: repoID,
            Mode:   IndexIncremental,
            Reason: "git bulk change",
        })
        return
    }

    EnqueueDeltaEvents(delta)
}
```

### 27.2 Watcher Misses

Mitigation:

```text
periodic manifest check
content hash check on semantic query
manual refresh_semantic_graph
```

## 28. Observability

### 28.1 Metrics

```text
helix_semantic_index_runs_total{mode,outcome}
helix_semantic_index_duration_seconds{mode}
helix_semantic_live_updates_total{kind,outcome}
helix_semantic_live_update_duration_seconds{kind}
helix_semantic_overlay_files{repo_state}
helix_semantic_pending_lsp_revalidations{language}
helix_semantic_files_indexed_total{language}
helix_semantic_symbols_total{language,kind}
helix_semantic_references_total{language}
helix_semantic_edges_total{edge_kind,source}
helix_semantic_lsp_enrichment_duration_seconds{language}
helix_semantic_lsp_enrichment_errors_total{language,outcome}
helix_semantic_graph_pagerank_duration_seconds{projection,status}
helix_semantic_cluster_duration_seconds{algorithm,status}
helix_semantic_store_query_duration_seconds{query_kind}
helix_semantic_compaction_duration_seconds{outcome}
helix_guardrail_checks_total{operation,outcome,enforcement}
helix_guardrail_violations_total{rule,operation,mode}
helix_eval_runs_total{suite,mode,outcome}
helix_eval_task_duration_seconds{suite,mode,outcome}
helix_eval_tool_calls_total{suite,mode,tool_name}
helix_eval_tokens_total{suite,mode,direction}
helix_type_resolution_attempts_total{language,outcome,confidence_tier}
helix_type_resolution_chain_depth{language}
helix_phasegraph_phase_duration_seconds{phase,outcome}
helix_phasegraph_validation_errors_total{phase,error_kind}
```

Bounded labels:

```text
tool_name
profile
mode
language
outcome
projection
algorithm
edge_kind
status
kind
```

### 28.2 Tracing

```text
semantic.ensure_index
semantic.plan
semantic.extract_file
semantic.lsp_enrich_file
semantic.merge_facts
semantic.write_facts
semantic.live.coalesce
semantic.live.update_file
semantic.live.graph_repair
semantic.live.revalidate_file
semantic.live.compact_overlay
semantic.load_graph
semantic.compute_pagerank
semantic.compute_clusters
semantic.retrieve_context
```

### 28.3 Privacy

```text
No remote export by default.
Metrics contain counts and durations only.
Traces must not include source code content unless debug content tracing is explicitly enabled.
Semantic DB remains local to workspace by default.
```

## 29. Failure Modes and Recovery

### 29.1 DuckDB Store Corruption

```text
1. Detect open/query failure.
2. Move database to timestamped .corrupt backup.
3. Report degraded semantic index status.
4. Allow rebuild.
```

### 29.2 LSP Timeout or Crash

```text
1. Continue with tree-sitter facts.
2. Mark affected files semantically pending or partially validated.
3. Lower confidence for unresolved edges.
4. Retry enrichment with backoff.
```

### 29.3 Memory Pressure

```text
1. Refuse full graph load above configured limit.
2. Use projection filters and subgraphs.
3. Stream writes to DuckDB.
4. Skip clustering if graph too large unless forced.
```

### 29.4 Overlay Too Large

```text
1. Force compaction if safe.
2. If not safe, schedule full/incremental snapshot rebuild.
3. Mark status partial if compaction repeatedly fails.
```

### 29.5 Rapid Edit Storm

```text
1. Coalesce events.
2. Pause PageRank local repair.
3. Mark scores stale.
4. Recompute after idle.
```

### 29.6 Snapshot Commit Failure

```text
1. Abort new snapshot.
2. Keep previous committed snapshot.
3. Keep live overlay if it remains valid.
4. Return recoverable=true.
```

## 30. Security and Safety

### 30.1 Filesystem Safety

```text
Never index outside active workspace root.
Respect ignore rules.
Do not follow symlinks outside workspace unless configured.
Do not read binary files.
Limit file size.
Treat generated files according to config.
```

### 30.2 MCP Mode Gating

```text
get_semantic_graph_status     read+
refresh_semantic_graph        read+
get_semantic_context          read+
explain_symbol_deep           read+
find_related_symbols          read+
get_cluster_map               read+
explain_cluster               read+
get_change_impact_graph       review+
validate_graph_edge           review+
index_semantic_graph          review+/admin
```

### 30.3 Content Safety for Telemetry

```text
No source code in metrics.
No source code in traces by default.
No source code in panic logs unless explicitly configured.
```

## 31. Testing Strategy

### 31.1 Unit Tests

Required tests:

```text
stable symbol ID generation
rename identity preservation
symbol merge rules
reference resolution confidence
edge weight calculation
effective read overlay merge
overlay tombstone handling
coalesce create/modify/delete events
file delete tombstones all facts
file rename preserves identity when content hash unchanged
edge invalidation on changed symbol
weighted PageRank convergence
personalized PageRank seed behavior
local PageRank repair status
weak components
label propagation determinism
cluster stale/approx/exact transitions
snapshot commit/abort
compaction concurrency guard
incremental invalidation
token-budgeted selection
guardrail policy receipt creation
guardrail missing-precheck detection
eval metric aggregation
eval mode comparison
access-chain type resolution fixpoint convergence
comment-based type fallback parsing
phase DAG cycle detection
phase DAG missing dependency detection
phase shutdown reverse topological order
```

### 31.2 Golden Fixtures

```text
testdata/semantic/go_basic
testdata/semantic/go_interfaces
testdata/semantic/typescript_react
testdata/semantic/python_dynamic
testdata/semantic/polyglot_collision
testdata/semantic/large_component
testdata/semantic/live_edit_call_change
testdata/semantic/file_rename_identity
testdata/semantic/git_checkout_bulk_change
testdata/guardrails/rename_without_refs
testdata/guardrails/public_api_without_blast_radius
testdata/eval/swebench_tiny
testdata/eval/helix_editing_tasks
testdata/typeresolve/jsdoc_chain
testdata/typeresolve/python_chain
testdata/phasegraph/bootstrap_cycle
```

### 31.3 Live Update Golden Example

```json
{
  "initial": {
    "symbols": ["A", "B"],
    "edges": ["A CALLS B"]
  },
  "edit": {
    "file": "main.go",
    "replace": "B()",
    "with": "C()"
  },
  "expected_live_overlay": {
    "removed_edges": ["A CALLS B"],
    "added_edges": ["A CALLS C"],
    "validation_state": "pending"
  },
  "expected_after_lsp": {
    "added_edges": ["A CALLS C"],
    "validation_state": "validated",
    "confidence_min": 0.95
  }
}
```

### 31.4 Integration Tests

Build tags:

```text
semantic_integration
semantic_lsp
semantic_live
semantic_large
```

Test cases:

```text
Go call hierarchy enrichment with gopls
TypeScript references enrichment
Python partial enrichment fallback
DuckDB snapshot rebuild
MCP tool response schemas
stale snapshot after external edit
live overlay update after Helix edit
file delete removes effective edges
file rename preserves stable identity
LSP timeout leaves graph structurally fresh
compaction creates new committed snapshot
git checkout bulk change schedules rebuild
guardrails warn on unsafe rename
semantic_guarded eval mode records required pre-checks
type resolver emits confidence-tiered CALLS/USES_TYPE edges
phase graph catches artificial bootstrap cycle
```

### 31.5 Performance Targets

```text
single file tree-sitter live update:
  p50 < 100 ms
  p95 < 500 ms

overlay write:
  p50 < 50 ms
  p95 < 250 ms

graph cache repair:
  p50 < 50 ms
  p95 < 300 ms

context retrieval from effective graph:
  p95 < 1s

100k edges PageRank:
  < 2s on developer laptop

1M edges PageRank:
  < 20s or graceful skip with warning

incremental single-file snapshot update:
  < 5s excluding LSP cold start

guardrail policy check:
  p95 < 10 ms

eval report aggregation for 100 tasks:
  < 5s excluding task execution

type-resolution chain of depth 8:
  p95 < 50 ms per file after extraction

phase graph validation for 100 phases:
  < 20 ms
```

## 32. Migration Plan

### Phase 0 — Schema and Store

Deliver:

```text
DuckDB store
schema migrations
snapshot tables
unified node table
file/symbol/reference/edge tables
overlay tables
effective read API
```

Acceptance:

```text
Can create/commit/abort snapshot.
Can insert/query snapshot facts.
Can apply overlay upserts/tombstones.
Can query effective facts.
```

### Phase 1 — Tree-sitter Extraction

Deliver:

```text
Go provider
TypeScript/JavaScript provider
Python provider
symbols/references/imports/syntax edges
stable symbol IDs
```

Acceptance:

```text
Golden fixtures pass.
Stable IDs deterministic.
Rename fixture preserves identity where expected.
```

### Phase 2 — Live Overlay Updates

Deliver:

```text
watcher
coalescer
live update queue
changed file update
file delete handling
file rename handling
graph cache repair
refresh_semantic_graph tool
```

Acceptance:

```text
Single-file edit updates effective graph without full reindex.
Deleted file facts disappear from effective query.
Graph version increments on live update.
```

### Phase 3 — LSP Enrichment and Revalidation

Deliver:

```text
hover enrichment
definition enrichment for references
find references
call hierarchy
type hierarchy
implementations
diagnostics snapshot
LSP revalidation queue
```

Acceptance:

```text
LSP-confirmed edges get confidence 1.0.
Timeouts produce structurally fresh pending-LSP state, not failure.
```

### Phase 4 — Graph Scores

Deliver:

```text
graph loading
weighted PageRank
personalized PageRank
multiple projections
score persistence with exact/approx/stale/missing status
local PageRank repair
```

Acceptance:

```text
RepoMap can optionally consume persisted scores.
PageRank deterministic within tolerance.
Local repair marks approximate scores.
```

### Phase 5 — Clustering

Deliver:

```text
weak components
label propagation splitting
cluster labels
cluster persistence
cluster status
graph-version-aware clusters
```

Acceptance:

```text
Large component splits deterministically.
Cluster labels are stable.
Clusters report exact/approx/stale state.
```

### Phase 6 — MCP Tools

Deliver:

```text
index_semantic_graph
refresh_semantic_graph
get_semantic_graph_status
get_semantic_context
explain_symbol_deep
find_related_symbols
get_cluster_map
explain_cluster
get_change_impact_graph
validate_graph_edge
```

Acceptance:

```text
Tools respect profile/mode gating.
Tool help includes parameters and examples.
Responses include snapshot, graph version, freshness, confidence, and evidence.
```

### Phase 7 — Existing Tool Integration

Deliver:

```text
get_repo_map uses persisted/effective scores when available
get_context delegates to semantic retrieval when available
analyze_blast_radius uses effective semantic graph expansion
get_health includes semantic index and live overlay status
edit tools emit live update events
```

Acceptance:

```text
Existing tools preserve fallback behavior.
No regression when semantic index disabled.
No blocking full index on normal tool use unless requested.
```

### Phase 8 — Compaction and Retention

Deliver:

```text
overlay compaction worker
snapshot retention policy
corruption recovery
large overlay recovery
```

Acceptance:

```text
Idle overlay compacts into new committed snapshot.
Previous snapshot preserved on failure.
Overlay clears only after successful commit.
```

### Phase 9 — Guardrails and Definition of Done

Deliver:

```text
GUARDRAILS.md
DoD.md
guardrail policy engine
safety receipts
MCP response warnings for missing required pre-checks
optional enforcement by profile/mode
```

Acceptance:

```text
Unsafe rename/delete/public API edit produces warning or enforcement response.
Required pre-check receipts are persisted in task/session trace.
Existing tools still work when guardrails are disabled.
```

### Phase 10 — Evaluation Harness

Deliver:

```text
eval runner
baseline/native/semantic/semantic_guarded modes
token, cost, latency, tool behavior, and safety metrics
JSON and Markdown reports
trace and patch capture
small local benchmark suite
optional SWE-bench-compatible runner
```

Acceptance:

```text
A benchmark can compare agent without Helix vs agent with Helix.
Reports show success rate, cost, latency, tool calls, and guardrail compliance.
Eval harness can run without external services for local fixtures.
```

### Phase 11 — Type Resolution and Access-Chain Resolver

Deliver:

```text
tiered type evidence model
access-chain resolver
fixpoint loop for chained calls/properties
comment fallback parsers for JSDoc/PHPDoc/YARD/Python comments where enabled
confidence-tiered RESOLVES_TO/CALLS/USES_TYPE edges
```

Acceptance:

```text
Resolver improves chained access edges in Go/TypeScript/Python fixtures.
Unresolved chains are represented honestly with low confidence or unresolved markers.
No heuristic edge is emitted as LSP-confirmed.
```

### Phase 12 — Pipeline DAG and Phase Validation

Deliver:

```text
phase graph runtime
Kahn-sorted phase execution
cycle detection
missing dependency detection
typed phase outputs
reverse-topological shutdown
bootstrap graph for daemon startup
semantic indexing graph
eval execution graph
```

Acceptance:

```text
Artificial cycle fails validation before startup.
Missing phase dependency fails validation.
Shutdown order is deterministic and tested.
```

## 33. Acceptance Criteria

The feature is production-ready when:

```text
1. Semantic index can be disabled completely.
2. Existing Helix tools continue working without semantic index.
3. Effective graph reads committed snapshot + live overlay correctly.
4. Full snapshot build works on Go, TypeScript/JavaScript, and Python fixtures.
5. Live single-file edit updates graph without full reindex.
6. File delete and rename behave correctly.
7. Incremental indexing handles changed/deleted/new files.
8. LSP crashes/timeouts do not corrupt snapshot or overlay.
9. DuckDB commit failure preserves previous snapshot.
10. PageRank and clusters are deterministic under fixed input.
11. Approximate/stale/exact score states are visible to tools.
12. MCP tools expose confidence, evidence, freshness, and graph version.
13. Metrics and traces are emitted with bounded labels.
14. Large repos degrade gracefully instead of crashing.
15. Golden tests cover symbol collisions across languages.
16. Integration tests cover at least one real LSP.
17. Compaction never loses overlay updates.
18. Edit tools emit graph update events after successful writes.
19. Guardrails and DoD are documented and machine-checkable.
20. Risky edits can produce safety receipts proving required pre-checks happened.
21. Evaluation harness compares baseline, native, semantic, and semantic_guarded modes.
22. Eval reports include success, cost, latency, token, tool-behavior, and safety metrics.
23. Type-resolution edges carry evidence and confidence tiers.
24. Access-chain resolver handles bounded chains without infinite loops.
25. Pipeline DAG detects cycles and missing dependencies before execution.
```

## 34. End-to-End Example

User asks:

```text
What code is relevant to authentication and what breaks if I change AuthMiddleware?
```

Execution:

```text
1. get_semantic_graph_status
2. refresh_semantic_graph if pending file events exist
3. search symbols for AuthMiddleware
4. validate target symbol with live LSP if freshness_mode requires it
5. load effective graph projection:
   CALLS, REFERENCES, USES_TYPE, IMPLEMENTS, IMPORTS
6. personalized PageRank seeded by AuthMiddleware
7. reverse reachability for impact
8. collect diagnostics/tests/callers/callees
9. return ranked evidence-backed context
```

Response shape:

```json
{
  "snapshot_id": "12345",
  "graph_version": 185,
  "overlay_active": true,
  "freshness": "partially_validated",
  "approximate_scores": true,
  "pending_lsp_files": 2,
  "subject": "AuthMiddleware",
  "direct_impact": [
    "router.SetupRoutes",
    "auth.RequireUser",
    "tests/auth_middleware_test.go"
  ],
  "related_clusters": [
    {
      "cluster_id": "7",
      "label": "auth middleware and JWT validation",
      "score": 0.91,
      "status": "approximate"
    }
  ],
  "evidence": [
    {
      "kind": "CALLS",
      "source": "lsp.call_hierarchy",
      "confidence": 1.0,
      "validation_state": "validated"
    },
    {
      "kind": "REFERENCES",
      "source": "tree_sitter",
      "confidence": 0.75,
      "validation_state": "pending"
    }
  ]
}
```

## 35. Implementation Skeleton

```go
package semantic

type Service struct {
    store       FactStore
    extractor   ExtractionRegistry
    enricher    LSPEnricher
    graphCache  *LiveGraphCacheRegistry
    ranker      Ranker
    clusterer   Clusterer
    retriever   Retriever
    snapshots   SnapshotManager
    liveQueue   LiveUpdateQueue
    lspQueue    LSPRevalidationQueue
    compactor   OverlayCompactor
    config      Config
}

func (s *Service) EnsureIndex(ctx context.Context, req EnsureIndexRequest) (*EnsureIndexResult, error) {
    repo := s.ResolveRepo(req.Root)
    latest := s.store.GetLatestSnapshot(ctx, repo.ID)

    plan, err := BuildIndexPlan(ctx, repo, latest)
    if err != nil {
        return nil, err
    }

    snap, err := s.snapshots.Begin(ctx, repo, plan)
    if err != nil {
        return nil, err
    }
    defer s.snapshots.AbortOnPanic(ctx, snap)

    extracted, err := s.Extract(ctx, snap, plan)
    if err != nil {
        s.snapshots.Abort(ctx, snap, err.Error())
        return nil, err
    }

    enriched, err := s.Enrich(ctx, snap, extracted)
    if err != nil && IsFatal(err) {
        s.snapshots.Abort(ctx, snap, err.Error())
        return nil, err
    }

    facts := MergeAll(extracted, enriched)

    if err := s.WriteFacts(ctx, snap, facts); err != nil {
        s.snapshots.Abort(ctx, snap, err.Error())
        return nil, err
    }

    if req.IncludeGraphScores {
        if err := s.ComputeScores(ctx, snap); err != nil {
            s.MarkPartial(snap, "graph scores partial: "+err.Error())
        }
    }

    if req.IncludeClusters {
        if err := s.ComputeClusters(ctx, snap); err != nil {
            s.MarkPartial(snap, "clusters partial: "+err.Error())
        }
    }

    if err := s.snapshots.Commit(ctx, snap); err != nil {
        s.snapshots.Abort(ctx, snap, err.Error())
        return nil, err
    }

    s.graphCache.Reload(repo.ID, snap)
    return s.BuildResult(ctx, snap), nil
}

func (s *Service) RefreshLiveGraph(ctx context.Context, req RefreshLiveGraphRequest) (*RefreshLiveGraphResult, error) {
    events := s.liveQueue.Drain(req.Paths)
    coalesced := CoalesceEvents(events)

    result := &RefreshLiveGraphResult{}
    for _, ev := range coalesced {
        update, err := s.ApplyLiveEvent(ctx, ev)
        if err != nil {
            result.Errors = append(result.Errors, err.Error())
            continue
        }
        result.Merge(update)
    }

    if req.WaitForLSP {
        s.lspQueue.Wait(ctx, req.MaxWait)
    }

    status := s.GetStatus(ctx, RootRef{RepoID: req.RepoID})
    result.GraphVersion = status.GraphVersion
    result.Freshness = status.Freshness
    return result, nil
}
```

## 36. Agent Safety Guardrails and Definition of Done

### 36.1 Purpose

Guardrails define safe and expected agent behavior when using Helix. They must exist as both documentation and runtime-checkable policy.

Artifacts:

```text
GUARDRAILS.md
DoD.md
internal/guardrails/policy.go
internal/guardrails/receipts.go
```

Guardrails are required because Helix exposes powerful edit and refactoring tools. The agent should not perform high-risk actions such as renames, deletes, public API edits, or broad fuzzy edits without first collecting semantic evidence.

### 36.2 Guardrail Rules

Initial rules:

```text
G-001 No rename-by-grep.
G-002 No delete without reference check.
G-003 No public API edit without blast-radius analysis.
G-004 No large fuzzy edit without reading target context first.
G-005 No dependency/security-sensitive change without diagnostics after edit.
G-006 Prefer symbol edits over line-number-only patches when a symbol tool is available.
G-007 Do not use stale semantic graph silently for impact analysis.
G-008 Do not claim exact semantic certainty for pending or heuristic graph edges.
G-009 For multi-file edits, verify diagnostics before declaring completion.
G-010 For generated files, edit source templates unless explicitly requested otherwise.
```

### 36.3 Definition of Done

`DoD.md` must define completion requirements for common task classes.

Example:

```text
Rename symbol:
  - identify target symbol by position or stable symbol ID
  - call find_references or analyze_blast_radius
  - use rename_symbol when supported
  - run diagnostics or verify_edit
  - report changed files and unresolved references

Delete symbol:
  - call safe_delete_symbol or find_references first
  - refuse or require override if references exist
  - run diagnostics after deletion

Public API change:
  - run analyze_blast_radius
  - inspect direct callers and implementations
  - apply edit
  - run diagnostics/tests where available
```

### 36.4 Safety Receipts

A safety receipt records that required checks happened before a risky operation.

```go
type SafetyReceipt struct {
    ReceiptID       string
    RepoID          string
    SessionID       string
    Operation       string
    TargetKind      string
    TargetStableKey string
    RequiredChecks  []string
    CompletedChecks []CompletedCheck
    Freshness       Freshness
    GraphVersion    GraphVersion
    Enforcement     GuardrailEnforcement
    Outcome         string
    CreatedAt       time.Time
}

type CompletedCheck struct {
    ToolName     string
    EvidenceID   string
    CompletedAt  time.Time
    Summary      string
}
```

### 36.5 Policy Engine

```go
type GuardrailPolicy interface {
    RequiredChecks(ctx context.Context, op RiskyOperation) ([]RequiredCheck, error)
    Evaluate(ctx context.Context, op RiskyOperation, receipts []SafetyReceipt) (*GuardrailDecision, error)
}

type GuardrailDecision struct {
    Allowed       bool
    Enforcement   GuardrailEnforcement
    MissingChecks []RequiredCheck
    Warnings      []string
    RequiresForce bool
}
```

Enforcement levels:

```text
off
warn
require_force
enforce
```

Default first release behavior:

```text
read/edit profiles: warn
review profile: require_force for missing high-risk pre-checks
admin profile: warn unless configured otherwise
```

### 36.6 Tool Integration

Examples:

```text
rename_symbol:
  requires find_references or analyze_blast_radius receipt

safe_delete_symbol:
  requires reference check; tool itself may create the receipt

replace_symbol_body on exported/public symbol:
  requires analyze_blast_radius receipt and verify_edit after edit

fuzzy_edit over large region:
  requires read_file/get_context receipt for target region
```

Guardrail response example:

```json
{
  "allowed": true,
  "guardrail": {
    "enforcement": "warn",
    "missing_checks": ["analyze_blast_radius"],
    "message": "Public API edit requested without blast-radius analysis. Proceeding because enforcement=warn."
  }
}
```

## 37. Evaluation Harness

### 37.1 Purpose

Helix must prove that agents using Helix perform better than agents without Helix. The evaluation harness measures correctness, cost, latency, tool behavior, safety compliance, and context quality.

The harness must support both external benchmark compatibility and local deterministic fixtures.

### 37.2 Eval Modes

```text
baseline
  Agent without Helix; file tools only.

native
  Agent with current Helix tools, no semantic index.

semantic
  Agent with Helix + Live Semantic Index.

semantic_guarded
  Agent with Helix + Live Semantic Index + guardrail policy receipts.
```

### 37.3 Package Layout

```text
eval/
  suites/
    helix_local/
    swebench/
  runners/
  agents/
  reports/

internal/eval/
  suite.go
  runner.go
  modes.go
  metrics.go
  report.go
  cost.go
  traces.go
```

### 37.4 Eval Task Schema

```go
type EvalTask struct {
    TaskID        string
    Suite         string
    RepoURI       string
    BaseCommit    string
    Prompt        string
    ExpectedFiles []string
    TestCommand   []string
    MaxDuration   time.Duration
    Metadata      map[string]string
}
```

### 37.5 Eval Result Schema

```go
type EvalResult struct {
    TaskID             string
    Mode               EvalMode
    Success            bool
    PatchApplies        bool
    TestsPass           bool
    DiagnosticsClean    bool
    Duration            time.Duration
    ToolCalls           int
    WrongToolCalls      int
    TokensIn            int64
    TokensOut           int64
    EstimatedCostUSD    float64
    EditCount           int
    FailedEditCount     int
    RollbackCount        int
    GuardrailViolations int
    RequiredChecksMet   float64
    ContextPrecision    float64
    ContextRecall       float64
    ReportPath          string
}
```

### 37.6 Metrics

Required metrics:

```text
task success rate
patch applies
tests pass
diagnostics clean
number of tool calls
wrong/redundant tool calls
tokens in/out
estimated cost
wall time
edit count
failed edit count
rollback count
reference/impact checks before risky edits
context precision
context recall
guardrail compliance
```

### 37.7 Tool Behavior Scoring

The eval harness must score whether agents used Helix safely and effectively.

Examples:

```text
Rename task:
  +1 if agent used rename_symbol
  +1 if agent called find_references or analyze_blast_radius first
  -1 if agent used grep-only rename

Delete task:
  +1 if agent used safe_delete_symbol
  -1 if agent deleted body without reference check

Public API edit:
  +1 if agent used analyze_blast_radius
  +1 if diagnostics were checked after edit
```

### 37.8 Runner Pseudocode

```go
func RunEvalSuite(ctx context.Context, suite EvalSuite, modes []EvalMode) (*EvalReport, error) {
    report := &EvalReport{}

    for _, task := range suite.Tasks {
        for _, mode := range modes {
            env := PrepareTaskWorkspace(task)
            agent := BuildAgentForMode(mode, env)
            recorder := NewTraceRecorder(task, mode)

            start := time.Now()
            outcome := agent.Run(ctx, task.Prompt, recorder)
            duration := time.Since(start)

            patchOK := CheckPatchApplies(env)
            testsOK := RunTaskTests(env, task.TestCommand)
            diagOK := CheckDiagnostics(env)
            safety := ScoreGuardrailCompliance(recorder.Events())
            context := ScoreContextQuality(recorder.ContextEvents(), task)

            result := EvalResult{
                TaskID:             task.TaskID,
                Mode:               mode,
                Success:            patchOK && testsOK && diagOK,
                PatchApplies:        patchOK,
                TestsPass:           testsOK,
                DiagnosticsClean:    diagOK,
                Duration:            duration,
                ToolCalls:           recorder.ToolCallCount(),
                TokensIn:            recorder.TokensIn(),
                TokensOut:           recorder.TokensOut(),
                EstimatedCostUSD:    recorder.EstimatedCost(),
                GuardrailViolations: safety.Violations,
                RequiredChecksMet:   safety.RequiredChecksMet,
                ContextPrecision:    context.Precision,
                ContextRecall:       context.Recall,
            }
            report.Add(result)
        }
    }

    return report, WriteEvalReport(report)
}
```

### 37.9 Reports

Outputs:

```text
eval_report.json
eval_report.md
cost_summary.json
tool_behavior.json
safety_compliance.json
traces/
patches/
```

Markdown report must include:

```text
mode comparison table
success/cost/latency summary
tool-call distribution
guardrail compliance
failure examples
recommendations
```

### 37.10 Policy as Eval Target

Guardrails must be evaluated directly.

Example tasks:

```text
Task: rename exported symbol
Expected: find_references or analyze_blast_radius before rename_symbol

Task: delete function
Expected: safe_delete_symbol or reference check first

Task: edit public interface
Expected: blast-radius analysis and diagnostics after edit
```

## 38. Type Resolution and Access-Chain Resolver

### 38.1 Purpose

The semantic index must improve best-effort type precision for dynamic and weakly typed languages. The goal is not perfect compiler-grade inference. The goal is confidence-tiered, evidence-backed resolution that improves graph edges for common access chains.

Important target pattern:

```text
a.b.c.d()
```

The resolver should infer as much as possible:

```text
root a → member b → member c → callable d
```

and emit `RESOLVES_TO`, `CALLS`, `USES_TYPE`, `READS`, or `WRITES` edges with explicit confidence.

### 38.2 Type Evidence Model

```go
type TypeEvidenceKind string

const (
    EvidenceLSPHover        TypeEvidenceKind = "lsp_hover"
    EvidenceLSPDefinition   TypeEvidenceKind = "lsp_definition"
    EvidenceAnnotation      TypeEvidenceKind = "annotation"
    EvidenceAssignment      TypeEvidenceKind = "assignment"
    EvidenceConstructorCall TypeEvidenceKind = "constructor_call"
    EvidenceImport          TypeEvidenceKind = "import"
    EvidenceDocComment      TypeEvidenceKind = "doc_comment"
    EvidenceHeuristic       TypeEvidenceKind = "heuristic"
)

type TypeCandidate struct {
    TypeName    string
    SymbolID    SymbolID
    NodeID      NodeID
    Evidence    []TypeEvidenceKind
    Confidence  float64
    SourceRange Range
}
```

Confidence tiers:

```text
1.00 LSP confirmed
0.90 explicit language type annotation
0.80 constructor/import resolved
0.70 assignment-inferred
0.60 doc-comment inferred
0.45 name/heuristic inferred
0.20 unknown/plausible
```

### 38.3 Access Chain Model

```go
type AccessChain struct {
    Root     ChainSegment
    Segments []ChainSegment
    Call     bool
    Location Range
}

type ChainSegment struct {
    Name     string
    Location Range
}

type ChainResolution struct {
    Resolved    bool
    Target      *SymbolFact
    Type        *TypeCandidate
    Confidence  float64
    Evidence    []TypeEvidenceKind
    FailedAt    int
    Reason      string
}
```

### 38.4 Fixpoint Resolution Algorithm

```go
func ResolveAccessChain(ctx context.Context, chain AccessChain, scope Scope) ChainResolution {
    if len(chain.Segments) > Config.TypeResolution.MaxChainDepth {
        return ChainResolution{Resolved: false, Confidence: 0.20, Reason: "chain too deep"}
    }

    state := ResolveRoot(ctx, chain.Root, scope)
    if state.Empty() {
        return ChainResolution{Resolved: false, Confidence: 0.20, FailedAt: 0, Reason: "root unresolved"}
    }

    for i, segment := range chain.Segments {
        candidates := ResolveMember(ctx, state.TypeCandidates, segment)
        if candidates.Empty() {
            return ChainResolution{
                Resolved:   false,
                Confidence: state.Confidence * 0.5,
                Evidence:   state.Evidence,
                FailedAt:   i + 1,
                Reason:     "member resolution failed: " + segment.Name,
            }
        }
        state = SelectBestCandidates(candidates)
    }

    return ChainResolution{
        Resolved:   true,
        Target:     state.BestSymbol,
        Type:       state.BestType,
        Confidence: state.Confidence,
        Evidence:   state.Evidence,
    }
}
```

### 38.5 Fixpoint Loop Across File Facts

Some chains require facts discovered later in the same file or package. Use bounded fixpoint refinement.

```go
func ResolveFileTypesFixpoint(ctx context.Context, facts *ExtractedFile) (*TypeResolutionResult, error) {
    state := InitialTypeState(facts)

    for iter := 0; iter < Config.TypeResolution.MaxFixpointIterations; iter++ {
        changed := false

        for _, chain := range facts.AccessChains {
            before := state.Hash(chain)
            res := ResolveAccessChainWithState(ctx, chain, state)
            state.Apply(chain, res)
            if state.Hash(chain) != before {
                changed = true
            }
        }

        if !changed {
            break
        }
    }

    return state.ToResult(), nil
}
```

### 38.6 Comment-Based Fallbacks

When enabled, parse type hints from comments and doc formats:

```text
JSDoc / TSDoc
PHPDoc
YARD
Python type comments and docstrings
```

Examples:

```text
@param {UserRepository} repo
@returns {Promise<User>}
/** @type {AuthService} */
# type: UserRepository
```

Comment-derived edges must never exceed confidence `0.60` unless independently confirmed.

### 38.7 Edge Emission

```go
func EmitTypeResolutionEdges(res ChainResolution, ref ReferenceFact) []EdgeFact {
    if !res.Resolved && !Config.TypeResolution.EmitUnresolvedEdges {
        return nil
    }

    confidence := res.Confidence
    validation := ValidationSkipped
    if HasEvidence(res.Evidence, EvidenceLSPDefinition) || HasEvidence(res.Evidence, EvidenceLSPHover) {
        validation = ValidationValidated
    }

    edges := []EdgeFact{}
    if res.Target != nil {
        edges = append(edges, EdgeFact{
            SrcNodeID:       ref.NodeID,
            DstNodeID:       res.Target.NodeID,
            EdgeKind:        "RESOLVES_TO",
            Source:          "type_resolution",
            ValidationState: validation,
            Confidence:      confidence,
            EvidenceRefID:   ref.RefID,
        })
    }

    if ref.RefKind == "call" && res.Target != nil {
        edges = append(edges, EdgeFact{
            SrcNodeID:       ref.ScopeNodeID,
            DstNodeID:       res.Target.NodeID,
            EdgeKind:        "CALLS",
            Source:          "type_resolution",
            ValidationState: validation,
            Confidence:      confidence,
            EvidenceRefID:   ref.RefID,
        })
    }

    return edges
}
```

## 39. Pipeline DAG and Phase Validation

### 39.1 Purpose

Helix has several order-sensitive workflows:

```text
daemon bootstrap
MCP tool registry setup
profile/mode middleware installation
LSP worker pool startup
semantic indexing
live update processing
overlay compaction
evaluation runs
shutdown
```

These workflows must be represented as typed phase DAGs to prevent implicit ordering bugs.

### 39.2 Phase Contract

```go
type PhaseID string

type PhaseSpec struct {
    ID       PhaseID
    Requires []PhaseID
    Provides []string
    Run      PhaseRunFunc
    Validate PhaseValidateFunc
    Shutdown PhaseShutdownFunc
}

type PhaseRunFunc func(ctx context.Context, deps PhaseDeps) (PhaseOutput, error)
type PhaseValidateFunc func(output PhaseOutput) error
type PhaseShutdownFunc func(ctx context.Context, output PhaseOutput) error
```

### 39.3 DAG Validation

Validate before execution:

```text
no duplicate phase IDs
all dependencies exist
no dependency cycles
required providers exist
phase outputs match declared providers
shutdown order is reverse topological order
```

```go
func ValidatePhaseGraph(phases []PhaseSpec) (*PhaseGraph, error) {
    g := BuildPhaseGraph(phases)

    if dup := g.FindDuplicateIDs(); dup != nil {
        return nil, PhaseGraphError{Kind: "duplicate_phase", Phase: *dup}
    }

    if missing := g.FindMissingDependencies(); len(missing) > 0 {
        return nil, PhaseGraphError{Kind: "missing_dependency", Missing: missing}
    }

    if cycle := g.FindCycle(); len(cycle) > 0 {
        return nil, PhaseGraphError{Kind: "cycle", Cycle: cycle}
    }

    order := KahnSort(g)
    shutdown := Reverse(order)

    return &PhaseGraph{Order: order, ShutdownOrder: shutdown}, nil
}
```

### 39.4 Bootstrap Phase Graph

Example:

```go
var BootstrapPhases = []PhaseSpec{
    PhaseConfig,
    PhaseTelemetry,
    PhaseStore,
    PhaseLanguageRegistry,
    PhaseLSPPool,
    PhaseMemory,
    PhaseToolRegistry,
    PhaseSemanticStore,
    PhaseLiveGraph,
    PhaseMCPServer,
    PhaseAdminServer,
}
```

### 39.5 Semantic Index Phase Graph

```text
discover_files
  → parse_tree_sitter
  → extract_symbols
  → resolve_imports
  → resolve_types
  → lsp_enrich
  → merge_facts
  → build_edges
  → write_snapshot
  → compute_scores
  → compute_clusters
  → initialize_graph_cache
```

### 39.6 Live Update Phase Graph

```text
collect_events
  → coalesce_events
  → classify_events
  → parse_changed_files
  → diff_effective_facts
  → write_overlay
  → repair_graph_cache
  → mark_scores_clusters
  → enqueue_lsp_revalidation
```

### 39.7 Eval Phase Graph

```text
prepare_workspace
  → configure_mode
  → run_agent
  → collect_trace
  → apply_patch_check
  → run_tests
  → run_diagnostics
  → score_tool_behavior
  → score_guardrails
  → aggregate_report
```

### 39.8 Execution Pseudocode

```go
func RunPhaseGraph(ctx context.Context, graph *PhaseGraph) (*PhaseGraphResult, error) {
    outputs := map[PhaseID]PhaseOutput{}
    result := &PhaseGraphResult{}

    for _, phase := range graph.Order {
        deps := BuildPhaseDeps(phase, outputs)
        start := time.Now()
        out, err := phase.Run(ctx, deps)
        duration := time.Since(start)

        result.Record(phase.ID, duration, err)
        if err != nil {
            ShutdownCompleted(ctx, graph, outputs)
            return result, err
        }

        if phase.Validate != nil {
            if err := phase.Validate(out); err != nil {
                ShutdownCompleted(ctx, graph, outputs)
                return result, err
            }
        }

        outputs[phase.ID] = out
    }

    return result, nil
}
```

### 39.9 DOT Debug Output

On validation failure, optionally emit:

```text
.helix/debug/phasegraph-bootstrap.dot
.helix/debug/phasegraph-semantic-index.dot
.helix/debug/phasegraph-eval.dot
```

This makes startup and pipeline ordering bugs visible.

## 40. Final Architecture Position

This feature turns Helix from:

```text
MCP-accessible IDE tools
```

into:

```text
MCP-accessible live semantic code intelligence platform
```

The final system is:

```text
tree-sitter
  → fast structural extraction and live update facts

LSPs
  → semantic confirmation and diagnostics

DuckDB
  → durable snapshots and live overlay facts

Custom Go graph engine
  → graph cache, PageRank, clusters, reachability

MCP semantic tools
  → agent-facing understanding layer

Guardrails and DoD
  → safe agent behavior and machine-checkable preconditions

Evaluation harness
  → proof that Helix improves agent success, cost, and safety

Type-resolution resolver
  → confidence-tiered semantic edges for chains and best-effort languages

Pipeline DAG
  → validated bootstrap, indexing, live-update, and eval ordering

Existing Helix tools
  → online validation, editing, diagnostics, and safety
```

The correct production direction is not a graph database bolted onto Helix. It is a live, evidence-backed semantic fact graph that makes Helix’s existing LSP/tree-sitter capabilities durable, current, explainable, and agent-usable.
