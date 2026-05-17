# Phase 72: P1 Cluster & Impact Tools — Research

**Researched:** 2026-05-17
**Domain:** Go MCP tool handlers in `internal/skill/semantic/` backed by the Phase 62/63/65 cluster engine + semantic graph store
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D1** — `cluster_id` is an opaque composite token `"{projection}:{graph_version}:{id}"`. `explain_cluster` decodes the token and refuses with a structured `stale_cluster_id` error if `graph_version` is stale.

**D2** — top-N by MemberCount; per-member rank = PageRank; cohesion = intra-edge density; separation = conductance; rep symbols = top-3 by PageRank; dominant edge kinds = top-3 by intra-edge count.

**D3** — `get_change_impact_graph` closed input (symbol_id OR (file_path, symbol_name), max_depth 2/5, optional edge_kinds filter) + closed output (nodes[], edges[], truncated, reached_depth, counts). No would_break. No critical path. No file rollup.

**D4** — size budgets: top_n 20/100 for get_cluster_map; members 5 preview / 200/1000 for explain_cluster; nodes 200, edges 500, depth 2/5 for get_change_impact_graph. Confidence cap 0.6 with reason `type_resolver_tier_3`.

**D5 (inherited from Phase 71)** — All Phase 71 decisions (D1 seed addressing, D3 edge-kind enum, D5 FreshnessV2 envelope, D6 read-only invariant + mode tier, D7 test surface) apply verbatim to Phase 72.

### Claude's Discretion

None explicitly deferred to Claude's discretion — all decisions locked via D1–D5.

### Deferred Ideas (OUT OF SCOPE)

- `get_change_impact_graph` hypothetical-edit metadata (edit_kind: signature_change | rename | delete)
- Cluster naming/labelling beyond representative symbols (LLM-derived cluster summaries)
- `get_change_impact_graph` per-node `would_break` flag
- Cluster ID stability across graph rebuilds
- Modularity / PageRank-sum ranking variants
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| P1TOOL-03 | `get_cluster_map` MCP tool — workspace-level overview: count, size distribution, top-N labeled clusters with member counts + representative symbols + dominant edge kinds. Mode tier: `read+`. | Cluster read accessors needed; new `ClusterMapAccessor` interface in `accessors.go`; handler in `tools_cluster_map.go` |
| P1TOOL-04 | `explain_cluster` MCP tool — per-cluster detail: full member list, ranking, cohesion/separation, dominant entry-point symbols. Mode tier: `read+`. | cluster_id token codec in new `cluster_id.go`; stale-id check at handler entry; new `ClusterDetailAccessor` in `accessors.go` |
| P1TOOL-05 | `get_change_impact_graph` MCP tool — pre-edit blast-radius subgraph, NOT file-level summary. Mode tier: `review+`. Confidence cap on degraded path. | Reuses `integ.SemanticLookup.ExpandFrom` + BFS walker; `capConfidences` helper in `blast_radius_strangler.go` is kernel-side only; Phase 72 needs a parallel cap in `tools_change_impact.go` |
</phase_requirements>

---

## Summary

Phase 72 ships three MCP tools (`get_cluster_map`, `explain_cluster`, `get_change_impact_graph`) as the second wave of the v1.11 P1 tool set. The codebase already contains all required infrastructure from Phases 62, 63, 65, and 71. The handler pattern, accessor protocol, envelope types, mode-check call, edge-kind surface enum, FreshnessV2 struct, `resolveSeed` helper, `assembleFreshness` helper, `errorResult`/`jsonResult` helpers, and `RegisterAll` entry point are all present and verified working in `internal/skill/semantic/`.

The primary new work is (1) declaring four new narrow accessor interfaces in `accessors.go` and adding their setters to `skill.go`, (2) writing three handler files following the `tools_explain_symbol.go` skeleton exactly, (3) adding a `cluster_id.go` codec file, and (4) extending `readonly_gate_test.go` and `integration_test.go` for the three new tools.

The key risk is that no `*Store.QueryClusterRows`, `QueryClusterMembers`, or `QueryNodePageRank` read methods exist today. The store currently exposes only `ClusterStatusForGraphVersion` (counts/status) and `UpsertClusters`/`UpsertClusterMembers` (write side). Three new read queries must be added to `internal/semantic/store/` or exposed through the accessor interface via on-demand computation in the handler.

**Primary recommendation:** Declare new narrow read accessors in `accessors.go`; implement the production backing methods on `*Store` directly; wire them via new setters on `SemanticSkill`. The three tool files follow the Phase 71 handler skeleton exactly.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| get_cluster_map response assembly | `internal/skill/semantic/` | `internal/semantic/store/` | Handler computes presentation from raw rows; store owns SQL read |
| explain_cluster member details + cohesion/separation | `internal/skill/semantic/` | `internal/semantic/store/` | Cohesion/conductance computed from member + edge counts available via accessor |
| cluster_id token encode/decode | `internal/skill/semantic/` | — | Codec is a skill-layer concern; opaque string to external callers |
| get_change_impact_graph traversal | `internal/skill/semantic/` | `internal/semantic/integ/` | Uses `integ.SemanticLookup.ExpandFrom`; stays semantic-side per vet-nokernel2semantic boundary |
| Confidence cap (type_resolver_tier_3) | `internal/skill/semantic/` (Phase 72 copy) | `internal/kernel/symbols/` (existing capConfidences) | blast_radius_strangler.go `capConfidences` operates on `*BlastRadius` (kernel type); Phase 72 needs its own flat-slice cap over `[]ImpactNode` |
| Mode tier enforcement (read+ / review+) | `internal/skill/semantic/mode_check.go` | — | `checkMode(snap, modeTierRead)` / `checkMode(snap, modeTierReview)` pattern established Phase 64/71 |
| Tool registration | `internal/skill/semantic/register.go` | — | `RegisterAll` must be extended with three new register*() calls |
| Test coverage | `internal/skill/semantic/` | — | Co-located with handlers, Phase 71 D7 pattern |

---

## Standard Stack

### Core (inherited — no new packages needed)

| Library | Purpose | Provenance |
|---------|---------|------------|
| `github.com/modelcontextprotocol/go-sdk/mcp` | MCP SDK: `mcpsdk.AddTool`, `CallToolResult`, `TextContent` | Phase 64+ pattern; already imported |
| `go.opentelemetry.io/otel/trace` | Tracing span wrapper via `kernel.WrapToolSpan` | Phase 71 pattern |
| `github.com/agenthands/helix/internal/mcp` | `SerenaMCPServer`, `ToolDef`, `SessionSnapshot` | Phase 64 |
| `github.com/agenthands/helix/internal/semantic/integ` | `SemanticLookup`, `SymbolID`, `ExpandFrom` | Phase 65 |
| `github.com/agenthands/helix/internal/semantic/types` | `CapCommentConfidence`, `ConfidenceComment`, `EvidenceKind` | Phase 62 |
| `github.com/agenthands/helix/internal/guardrails` | `IssueReceiptOnSuccess` | Phase 64 pattern |

No new external dependencies. No new daemon-bootstrap wiring (P1TOOL-08 invariant). [VERIFIED: codebase grep]

### Package Legitimacy Audit

No new external packages are installed in this phase.

| Package | Registry | slopcheck | Disposition |
|---------|----------|-----------|-------------|
| (none) | — | — | No new packages |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

---

## Architecture Patterns

### System Architecture Diagram

```
Agent → MCP tools/call
           │
           ▼
   LazyInitMiddleware  (activates workspace on first call)
           │
           ▼
   SuggestionMiddleware (parameter typo hints)
           │
           ▼
   ProfileFilterMiddleware (tools/list gating; brief descriptions)
           │
           ▼
   TelemetryMiddleware (deadline injection; RED metrics)
           │
           ├──► handleGetClusterMap(ctx, args)
           │        │ checkMode(snap, modeTierRead)
           │        │ workspaceKey(ctx) → repoID
           │        │ ClusterMapAccessor.QueryTopClusters(ctx, repoID, projection, graphVersion, topN)
           │        │   └─► *Store.QueryClusterSummaries(...)  [new SQL read]
           │        │ assemble representative symbols via ClusterRepAccessor.QueryRepSymbols(...)
           │        │ assemble dominant edge kinds via ClusterEdgesAccessor.QueryIntraEdgeKinds(...)
           │        │ assembleFreshness(ctx, repoID)
           │        └──► jsonResult(GetClusterMapResult)
           │
           ├──► handleExplainCluster(ctx, args)
           │        │ checkMode(snap, modeTierRead)
           │        │ decodeClusterID(args.ClusterID) → {projection, graphVersion, clusterIntID}
           │        │ compare graphVersion to current → stale_cluster_id error if stale
           │        │ ClusterDetailAccessor.QueryClusterMembers(ctx, repoID, projection, gv, clusterIntID, limit)
           │        │ compute cohesion = intra_edges / max_possible
           │        │ compute separation = conductance
           │        │ identify entry-point symbols (is_exported / PageRank)
           │        │ assembleFreshness(ctx, repoID)
           │        └──► jsonResult(ExplainClusterResult)
           │
           └──► handleGetChangeImpactGraph(ctx, args)
                    │ checkMode(snap, modeTierReview)
                    │ resolveSeed(ctx, ws, args.Seed)
                    │ integ.SemanticLookup.ExpandFrom(ctx, ws, sym, maxDepth)
                    │ filter by args.EdgeKinds (surface enum → internal kinds)
                    │ cap nodes/edges; set truncated + reached_depth
                    │ detect type_resolver_tier_3 degradation → capEdgeConfidences(0.6)
                    │ assembleFreshness(ctx, repoID)
                    └──► jsonResult(GetChangeImpactGraphResult)
```

### Recommended Project Structure

No new packages. All new files land in `internal/skill/semantic/`:

```
internal/skill/semantic/
├── cluster_id.go                    # NEW: encodeClusterID / decodeClusterID / stale-id error
├── tools_cluster_map.go             # NEW: get_cluster_map handler + register fn + arg/result types
├── tools_explain_cluster.go         # NEW: explain_cluster handler + register fn + arg/result types
├── tools_change_impact.go           # NEW: get_change_impact_graph handler + register fn + arg/result types
├── tools_cluster_map_test.go        # NEW: per-tool unit tests
├── tools_explain_cluster_test.go    # NEW: per-tool unit tests
├── tools_change_impact_test.go      # NEW: per-tool unit tests
├── accessors.go                     # MODIFIED: +4 new accessor interfaces + production-wiring note
├── skill.go                         # MODIFIED: +4 new accessor fields + 4 SetXxx setters
├── register.go                      # MODIFIED: +3 register* calls in RegisterAll
├── readonly_gate_test.go            # MODIFIED: gatedHandlerFiles += three new Phase 72 files
└── integration_test.go              # MODIFIED: +TestFourTools_* cross-tool chain test
```

No changes under `internal/semantic/store/` are strictly required IF the accessor implementations can be provided via on-demand computation in the handler. However, new `*Store` methods are recommended for `QueryClusterSummaries` and `QueryClusterMembers` — see "Cluster Data Wiring" section.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Seed resolution | Custom file+name → SymbolID lookup | `s.resolveSeed(ctx, ws, args.Seed)` | Already exists in `seed_resolve.go`; handles exact/ambiguous/not_found |
| FreshnessV2 envelope | Custom freshness struct | `s.assembleFreshness(ctx, repoID)` | Already exists in `tools_explain_symbol.go:357`; shared by all Phase 71 tools |
| Edge-kind surface mapping | Custom string map | `MapInternalKind(internal string)` in `edge_kind_surface.go` | Phase 71 verified; 9 internal kinds mapped |
| Mode check | Custom mode string comparison | `checkMode(snap, modeTierRead)` / `checkMode(snap, modeTierReview)` | Phase 64 `mode_check.go`; handles review/admin elevation suggestion |
| JSON response serialization | Custom JSON building | `jsonResult(v any)` in `handler_helpers.go` | Handles marshal errors defensively |
| Error results | Custom error struct | `errorResult(msg string)` in `handler_helpers.go` | Produces `IsError=true` CallToolResult |
| Per-node confidence cap | Custom loop | New `capImpactConfidences(nodes []ImpactNode, cap float64)` in `tools_change_impact.go` | `capConfidences` in `blast_radius_strangler.go` operates on `*BlastRadius` (kernel type); Phase 72 needs its own flat-slice variant; ~5 LOC, trivially correct |
| Blast-radius traversal | Custom BFS from scratch | `integ.SemanticLookup.ExpandFrom(ctx, ws, sym, depth)` | Returns `[]integ.Impact` carrying edges + confidence; Phase 65 seam |

---

## Cluster Data Wiring

### What Exists Today [VERIFIED: codebase grep]

**Write side (Phase 62/63):**
- `store.ClusterSummary{ID uint64, MemberCount int}` — in `internal/semantic/store/overlay.go:787`
- `store.ClusterMemberRow{ClusterID uint64, NodeID uint64}` — in `internal/semantic/store/overlay.go:794`
- `OverlayTx.UpsertClusters(ctx, projection, graphVersion, []ClusterSummary)` — overlay write
- `OverlayTx.UpsertClusterMembers(ctx, projection, graphVersion, []ClusterMemberRow)` — overlay write
- `OverlayTx.DeleteClustersForGraphVersion(ctx, projection, graphVersion)` — overlay write

**Read side (Phase 69):**
- `Store.ClusterStatusForGraphVersion(ctx, repoID, graphVersion) (ClusterStatusRow, error)` — returns aggregate counts only: `ClusterCount`, `MemberCount`, `ComputedAt`, `IsCurrent`
- Schema table `semantic_clusters`: `(repo_id, graph_version, cluster_id, algorithm, label, summary, score, status, computed_at)` — `score` is overloaded with MemberCount at write time per overlay.go:835 comment

**Schema `semantic_cluster_members`:** `(repo_id, graph_version, cluster_id, node_id, weight, role)`

**Schema `semantic_graph_scores`:** `(repo_id, graph_version, node_id, score_name, score)` — stores PageRank per node per projection

**Accessor used by Phase 71 for cluster membership (single-symbol):**
- `ClusterMembershipAccessor.ClusterIDOf(ctx, repoID, symbolID) (clusterID, size, err)` — narrow seam already wired

### What is Missing — New Read Methods Needed

Phase 72 needs three new read operations that do not exist on `*Store` today:

1. **`QueryClusterSummaries(ctx, repoID, projection string, graphVersion uint64, topN int) ([]ClusterSummaryRow, error)`** — returns top-N `ClusterSummary` rows sorted by MemberCount DESC. Underlying SQL is a SELECT on `semantic_clusters` filtered by `(repo_id, graph_version)` sorted by `score DESC` (score = MemberCount per overlay.go:835). [ASSUMED — method name; SQL is straightforward given schema]

2. **`QueryClusterMembers(ctx, repoID, projection string, graphVersion, clusterID uint64, limit int) ([]ClusterMemberRow, error)`** — returns member rows for one cluster. SQL is a SELECT on `semantic_cluster_members` filtered by `(repo_id, graph_version, cluster_id)` with LIMIT. [ASSUMED — method name]

3. **`QueryNodePageRank(ctx, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error)`** — returns PageRank scores for a set of nodeIDs from `semantic_graph_scores`. Needed for representative symbols (top-3 by PageRank per cluster) and per-member rank in `explain_cluster`. [ASSUMED — method name]

**Recommendation:** Implement all three as methods on `*Store` (not computed in the handler). This keeps SQL out of the skill layer, follows the existing `QueryRankedFiles` pattern, and allows future reuse. Each method is a simple SELECT — no new tables, no transactions.

These three methods must be exposed through new narrow accessor interfaces in `accessors.go` and set on `SemanticSkill` via new setters in `skill.go`.

### Cohesion / Conductance Computation

**Cohesion (intra-edge density):** `intra_edges / max_possible_intra_edges` where `max_possible = n × (n-1)` for directed graphs (n = member count). `intra_edges` = count of edges in `semantic_cluster_members × semantic_edges` JOIN where both src and dst are members of the cluster.

**Separation (conductance):** `edges_leaving_cluster / (2 × intra_edges + edges_leaving_cluster)`. `edges_leaving_cluster` = edges where src is a member but dst is NOT a member (or vice versa depending on convention — recommend: sum of both directions for undirected conductance).

**Decision required by planner:** Where to compute these. Two options:

- **Option A — at detection time in `internal/semantic/cluster/`:** Extend `RunClusterDetection` to compute cohesion/conductance and persist alongside `ClusterSummary` in a new column or in `ClusterSummary.Score`. Pro: cached, zero cost at read time. Con: requires schema migration or column reuse; `ClusterSummary.Score` is already overloaded with MemberCount. Requires a non-trivial Phase 72 change.
- **Option B — on-demand in the handler from persisted members + effective edges:** Handler reads member list + calls `StoreAccessor.QueryEffectiveAdjacency` (already exposed) to get the effective graph, computes cohesion/conductance in Go. Pro: no schema change. Con: `QueryEffectiveAdjacency` returns the ENTIRE effective graph, which may be expensive for large workspaces. Cost O(E) per explain_cluster call.

**Research recommendation:** Option B for Phase 72 (simpler, no schema migration). Document the cost with a TODO for caching. The `QueryEffectiveAdjacency` call is already available through `StoreAccessor` (in `accessors.go`). For clusters with N < 1000, the computation is fast.

### Dominant Entry-Point Symbols

Per D2, entry-point symbols are members with `is_exported = true` or flagged by `find_entry_points` semantics, sorted by PageRank. The `semantic_symbols` table has no `is_exported` column in the current schema. The fallback: use top-by-PageRank members as entry points (proxy heuristic). OR inspect symbol_id stable_key for exported naming (capital letter after last `:` — same heuristic as `isExported()` in `blast_radius_strangler.go:292`). [ASSUMED — schema check confirmed no `is_exported` column; `isExported` heuristic is available]

---

## cluster_id Token Codec

### Location

New file: `internal/skill/semantic/cluster_id.go`

### Encoding

```go
// encodeClusterID produces the opaque composite token consumed by explain_cluster.
// Format: "{projection}:{graph_version}:{cluster_int_id}"
func encodeClusterID(projection string, graphVersion, clusterIntID uint64) string {
    return fmt.Sprintf("%s:%d:%d", projection, graphVersion, clusterIntID)
}
```

### Decoding + Stale-ID Check

```go
type decodedClusterID struct {
    Projection    string
    GraphVersion  uint64
    ClusterIntID  uint64
}

// decodeClusterID parses a cluster_id token produced by encodeClusterID.
// Returns serr.InvalidArgs on parse failure.
func decodeClusterID(token string) (decodedClusterID, error)

// checkClusterIDFresh returns a structured stale_cluster_id error envelope
// (as jsonResult) if the decoded graphVersion != currentGraphVersion.
// Returns nil when fresh.
func checkClusterIDFresh(decoded decodedClusterID, currentGraphVersion uint64) *mcpsdk.CallToolResult
```

The stale-id check is a single equality predicate at handler entry in `handleExplainCluster`:
```go
currentGV, err := s.getStore().CurrentGraphVersion(ctx, repoID)
if decoded.GraphVersion != currentGV {
    return jsonResult(ExplainClusterResult{
        Error:      "stale_cluster_id",
        NextAction: "call get_cluster_map",
        Freshness:  s.assembleFreshness(ctx, repoID),
    })
}
```

`Error` is a closed-enum field alongside Phase 71's existing `fallback_reason` reasons. The existing pattern in `tools_explain_symbol.go` uses `FallbackReason string` for this; the same field can carry `"stale_cluster_id"` with the `NextAction` hint as an additional field or embedded in the `FallbackReason` string.

---

## Handler Skeleton Recipes Per Tool

### Tool 1: `get_cluster_map`

**File:** `internal/skill/semantic/tools_cluster_map.go`
**Analog:** `tools_explain_symbol.go` (same package, same pattern; read+ mode)

```go
// INVARIANT (D-09 / D-13): get_cluster_map MUST NOT touch snapshot-write surface.
// No Begin/Commit/Abort/Write methods. Read+ stays read-only.

type GetClusterMapArgs struct {
    Projection string `json:"projection,omitempty"` // default "weak_components"
    TopN       int    `json:"top_n,omitempty"`       // default 20, max 100
}

type ClusterSummaryEntry struct {
    ClusterID           string            `json:"cluster_id"`           // opaque encoded token
    MemberCount         int               `json:"member_count"`
    MemberCountTotal    int               `json:"member_count_total"`   // for preview truncation
    MembersPreview      []string          `json:"members_preview"`      // top-5 by PageRank, as symbol_ids
    RepresentativeSymbols []string        `json:"representative_symbols"` // top-3 by PageRank
    DominantEdgeKinds   []EdgeKindSurface `json:"dominant_edge_kinds"`  // top-3 by intra-count
}

type GetClusterMapResult struct {
    TotalClusters    int                   `json:"total_clusters"`
    TopN             int                   `json:"top_n"`
    Clusters         []ClusterSummaryEntry `json:"clusters"`
    Freshness        FreshnessV2           `json:"freshness"`
    FallbackReason   string                `json:"fallback_reason,omitempty"`
}

func (s *SemanticSkill) handleGetClusterMap(ctx context.Context, args GetClusterMapArgs) *mcpsdk.CallToolResult {
    // 1. checkMode(snap, modeTierRead)
    // 2. workspaceKey → repoID
    // 3. CurrentGraphVersion → gv
    // 4. clamp top_n to [1, 100], default 20
    // 5. QueryClusterSummaries(ctx, repoID, projection, gv, topN) → []ClusterSummaryRow
    // 6. For each cluster: QueryNodePageRank for members → rep symbols + preview
    // 7. For each cluster: QueryIntraEdgeKinds → dominant edge kinds via MapInternalKind
    // 8. encode cluster_id tokens via encodeClusterID
    // 9. assembleFreshness
    // 10. return jsonResult(GetClusterMapResult)
}
```

### Tool 2: `explain_cluster`

**File:** `internal/skill/semantic/tools_explain_cluster.go`
**Analog:** `tools_explain_symbol.go`; read+ mode; stale-id check at step 1b

```go
// INVARIANT (D-09 / D-13): explain_cluster MUST NOT touch snapshot-write surface.

type ExplainClusterArgs struct {
    ClusterID string `json:"cluster_id"` // opaque token from get_cluster_map
    MaxMembers int   `json:"max_members,omitempty"` // default 200, max 1000
}

type ClusterMemberEntry struct {
    SymbolID    string  `json:"symbol_id"`
    PageRank    float64 `json:"pagerank"`
    IsEntryPoint bool   `json:"is_entry_point,omitempty"`
}

type ExplainClusterResult struct {
    ClusterID           string               `json:"cluster_id"`
    MemberCount         int                  `json:"member_count"`
    MembersReturned     int                  `json:"members_returned"`
    Members             []ClusterMemberEntry `json:"members"`
    Cohesion            float64              `json:"cohesion"`    // intra_density
    Separation          float64              `json:"separation"`  // conductance
    DominantEdgeKinds   []EdgeKindSurface    `json:"dominant_edge_kinds"`
    EntryPoints         []ClusterMemberEntry `json:"entry_points"`
    Freshness           FreshnessV2          `json:"freshness"`
    FallbackReason      string               `json:"fallback_reason,omitempty"`
}

func (s *SemanticSkill) handleExplainCluster(ctx context.Context, args ExplainClusterArgs) *mcpsdk.CallToolResult {
    // 1. checkMode(snap, modeTierRead)
    // 1b. decodeClusterID(args.ClusterID) — serr.InvalidArgs on parse fail
    // 1c. CurrentGraphVersion → stale-id check → stale_cluster_id error if mismatched
    // 2. workspaceKey → repoID
    // 3. clamp max_members to [1, 1000], default 200
    // 4. QueryClusterMembers(ctx, repoID, projection, gv, clusterIntID, limit)
    // 5. QueryNodePageRank for member node_ids → sort → ClusterMemberEntry list
    // 6. Compute cohesion: read edge count from effective adjacency (StoreAccessor.QueryEffectiveAdjacency)
    // 7. Compute conductance from intra/total edges
    // 8. Identify entry points via isExported heuristic OR top-by-PageRank
    // 9. Dominant edge kinds: count intra-cluster edges by internal_kind → MapInternalKind → top-3
    // 10. assembleFreshness
    // 11. return jsonResult(ExplainClusterResult)
}
```

### Tool 3: `get_change_impact_graph`

**File:** `internal/skill/semantic/tools_change_impact.go`
**Analog:** `tools_explain_symbol.go` but review+ mode; uses `integ.SemanticLookup.ExpandFrom`

```go
// INVARIANT (D-09 / D-13): get_change_impact_graph MUST NOT touch snapshot-write surface.
// review+ tools MUST still be read-only on graph state.

const (
    impactDefaultDepth = 2
    impactMaxDepth     = 5
    impactNodeCap      = 200
    impactEdgeCap      = 500
)

type GetChangeImpactGraphArgs struct {
    Seed      SeedInput `json:"seed"`               // reuses Phase 71 D1 shape
    MaxDepth  int       `json:"max_depth,omitempty"` // default 2, max 5
    EdgeKinds []string  `json:"edge_kinds,omitempty"` // optional filter, MCP surface enum values
}

type ImpactNode struct {
    SymbolID      string  `json:"symbol_id"`
    QualifiedName string  `json:"qualified_name,omitempty"`
    Package       string  `json:"package,omitempty"`
    PageRank      float64 `json:"pagerank,omitempty"`
}

type ImpactEdge struct {
    From         string          `json:"from"`
    To           string          `json:"to"`
    EdgeKind     EdgeKindSurface `json:"edge_kind"`
    InternalKind string          `json:"internal_kind"`
    Confidence   float64         `json:"confidence"`
}

type ConfidenceCap struct {
    Value  float64 `json:"value"`
    Reason string  `json:"reason"` // "type_resolver_tier_3"
}

type GetChangeImpactGraphResult struct {
    Nodes          []ImpactNode   `json:"nodes"`
    Edges          []ImpactEdge   `json:"edges"`
    Truncated      bool           `json:"truncated"`
    ReachedDepth   int            `json:"reached_depth"`
    NodesCount     int            `json:"nodes_count"`
    EdgesCount     int            `json:"edges_count"`
    ConfidenceCap  *ConfidenceCap `json:"confidence_cap,omitempty"`
    Freshness      FreshnessV2    `json:"freshness"`
    FallbackReason string         `json:"fallback_reason,omitempty"`
}

func (s *SemanticSkill) handleGetChangeImpactGraph(ctx context.Context, args GetChangeImpactGraphArgs) *mcpsdk.CallToolResult {
    // 1. checkMode(snap, modeTierReview) — first call
    // 2. resolveSeed(ctx, ws, args.Seed) — short-circuit on not_found
    // 3. clamp maxDepth to [1, 5], default 2
    // 4. lookup.ExpandFrom(ctx, ws, sym, maxDepth) → []integ.Impact
    // 5. filter by args.EdgeKinds (surface enum filter → surfaceToInternalKinds lookup)
    // 6. cap nodes (200) and edges (500); set truncated + reached_depth
    // 7. detect tier-3 degradation predicate → capEdgeConfidences(0.6, "type_resolver_tier_3")
    // 8. assemble ImpactNode + ImpactEdge slices using MapInternalKind for edge_kind
    // 9. assembleFreshness
    // 10. return jsonResult(GetChangeImpactGraphResult)
}
```

### Handler Ordering Contract (load-bearing)

1. `checkMode(snap, modeTierRead)` or `checkMode(snap, modeTierReview)` — MUST be first
2. Seed/cluster-ID resolution — before any accessor reads
3. All accessor reads
4. Confidence capping — after accessor reads, before envelope assembly
5. `assembleFreshness` — after accessor reads
6. `jsonResult(result)` — last

---

## get_change_impact_graph Traversal

### ExpandFrom Seam [VERIFIED: codebase read]

`integ.SemanticLookup.ExpandFrom(ctx, ws, sym, depth int) ([]integ.Impact, error)` is the correct traversal entrypoint. It is already used by `analyzeBlastRadiusViaLookup` in `internal/kernel/symbols/blast_radius_strangler.go:107`.

`integ.Impact` carries:
- `SymbolID integ.SymbolID`
- `Confidence float64`
- `Refuted bool`
- `Evidence integ.Evidence` (contains `Edges []integ.Edge` with `From`, `To`, `Kind`, `Confidence`)

The `depth` parameter controls BFS depth. For Phase 72: pass `clampedDepth` (clamped to [1, 5]).

### Node/Edge Cap and Truncated Contract

After `ExpandFrom` returns `[]integ.Impact`:
- `NodesCount = len(impacts)` (before cap)
- `EdgesCount = sum(len(impact.Evidence.Edges) for all impacts)` (before edge cap)
- Cap nodes: `nodes = impacts[:min(impactNodeCap, len(impacts))]`
- Cap edges per node: collect all edges from capped nodes, total-cap at `impactEdgeCap`
- `Truncated = NodesCount > impactNodeCap || EdgesCount > impactEdgeCap`
- `ReachedDepth` — `ExpandFrom` does not return the actual depth reached; set to `min(maxDepth, actualDepth)`. If `integ.Impact` carries depth metadata, use it; otherwise `ReachedDepth = maxDepth` (conservative assumption). [ASSUMED — need to verify Impact struct for depth field]

### Type-Resolver Tier 3 Predicate [ASSUMED — detection path]

D4 specifies `confidence_cap.reason = "type_resolver_tier_3"` when falling back to a degraded path. The type resolver tiers are defined in `internal/semantic/types/resolver.go:14-21`:
- `EvidenceConstructor` = `"constructor"` = tier 3 (confidence 0.80)

The predicate "tier 3" maps to `EvidenceKind == EvidenceConstructor` in Phase 62 TYPES-04 semantics. In the context of `get_change_impact_graph`, the degradation signal comes from the `integ.Impact.Confidence` values returned by `ExpandFrom`. The cap should be applied when any impact has `Confidence < 0.80` AND was derived via type-resolver (not direct LSP). The practical detection: if `ExpandFrom` returns any impact with `Confidence <= 0.6` (TYPES-04 cap already applied by the daemon side), apply the envelope-level `confidence_cap` signal.

**Open Question OQ-1 (planner decision required):** The exact predicate for "tier 3 degraded" is not fully pin-pointed from the `integ.Impact` struct alone — `Impact.Confidence` is already capped to ≤0.6 by the daemon adapter on degraded paths. The handler should: if `max(impact.Confidence for all impacts) < 0.8`, apply the cap. Recommendation: detect `any impact with Confidence < 0.8` and emit `confidence_cap{value: 0.6, reason: "type_resolver_tier_3"}`. Cap is `min(existing, 0.6)` per edge.

### capEdgeConfidences helper

```go
// capEdgeConfidences walks edges and clamps Confidence to ≤ cap.
// Does NOT copy: mutates in-place (impacts slice is locally owned).
func capEdgeConfidences(edges []ImpactEdge, cap float64) {
    for i := range edges {
        if edges[i].Confidence > cap {
            edges[i].Confidence = cap
        }
    }
}
```

This is the Phase 72 counterpart to `blast_radius_strangler.go:capConfidences`. It operates on `[]ImpactEdge` (skill-layer type), so it does NOT depend on kernel types.

---

## Edge-Kind Enum Mapping

### Existing Table [VERIFIED: codebase read]

`internal/skill/semantic/edge_kind_surface.go` defines `EdgeKindSurface` (8 values) and `MapInternalKind`. The current mapping:

| Internal | Surface |
|----------|---------|
| `CALLS` | `calls` |
| `REFERENCES` | `references` |
| `IMPORTS` | `references` |
| `IMPLEMENTS` | `implements` |
| `EXTENDS` | `extends` |
| `RESOLVES_TO` | `has_type` |
| `USES_TYPE` | `uses_type` |
| `CONTAINS` | `contains` |
| `DEFINED_IN` | `contains` |
| (default) | `other` |

**Phase 72 usage:**
- `get_cluster_map` dominant edge kinds: `MapInternalKind(edgeKind)` for each intra-cluster edge, aggregate top-3 surface-enum values
- `get_change_impact_graph` edge entries: `MapInternalKind(impact.Evidence.Edges[i].Kind)` → `EdgeKind` field; raw kind → `InternalKind` field
- `get_change_impact_graph` `edge_kinds` input filter: use `surfaceToInternalKinds(surface)` (already defined in `tools_validate_edge.go:207`) to convert the surface-enum filter to internal kinds for ExpandFrom filtering. **Note:** `surfaceToInternalKinds` is defined inline in `tools_validate_edge.go` as package-private. Phase 72 can use it directly since all code is in the same `package semantic`.

---

## Mode Tier Wiring

### read+ (get_cluster_map, explain_cluster)

```go
snap := s.sessionSnapshot(ctx)
if err := checkMode(snap, modeTierRead); err != nil {
    return errorResult(err.Error())
}
```

`modeTierRead` always returns nil (every session passes), but the call is REQUIRED per D6 (Phase 71) for code-review visibility and Phase 66 GuardrailMiddleware precedent.

### review+ (get_change_impact_graph)

```go
snap := s.sessionSnapshot(ctx)
if err := checkMode(snap, modeTierReview); err != nil {
    return errorResult(err.Error())
}
```

`modeTierReview` passes when `session.Mode` is `"review"` or `"admin"`. Returns a structured `PermissionDenied` error otherwise, suggesting `switch_mode(target_mode="review")`.

### Profile Filter Registration

No profile filter changes in Phase 72 — Phase 73 owns the 5×4 matrix (P1TOOL-07). The tools are registered in `RegisterAll` with no profile restriction at this phase.

### New Accessor Interfaces in accessors.go

Four new narrow interfaces must be declared (following the `// ----- Phase 7X-0Y additions -----` section comment pattern):

```go
// ClusterMapAccessor returns top-N cluster summaries for the workspace.
type ClusterMapAccessor interface {
    QueryClusterSummaries(ctx context.Context, repoID, projection string,
        graphVersion uint64, topN int) ([]ClusterSummaryRow, error)
}

// ClusterMemberAccessor returns full member rows for one cluster.
type ClusterMemberAccessor interface {
    QueryClusterMembers(ctx context.Context, repoID, projection string,
        graphVersion, clusterIntID uint64, limit int) ([]ClusterMemberRow, error)
}

// ClusterPageRankAccessor returns PageRank scores for a set of node IDs.
type ClusterPageRankAccessor interface {
    QueryNodePageRanks(ctx context.Context, repoID, projection string,
        graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error)
}

// ImpactLookupAccessor is the SemanticLookup seam for get_change_impact_graph.
// Wraps ExpandFrom and Status for graph_version propagation.
type ImpactLookupAccessor interface {
    ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey,
        sym integ.SymbolID, depth int) ([]integ.Impact, error)
    Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error)
}
```

**Note:** `ImpactLookupAccessor` overlaps with `integ.SemanticLookup`. The cleanest option is to inject the full `integ.SemanticLookup` handle into `SemanticSkill` (Phase 65 already wires it into kernel-side blast-radius via `integSemanticLookup` in `semantic_wiring.go`). The daemon can wire the same handle to Phase 72's accessor. [ASSUMED — daemon wiring already has this handle; need to confirm it is accessible from the semantic skill injection path]

**Open Question OQ-2 (planner decision):** Should Phase 72 inject `integ.SemanticLookup` directly as a new field on `SemanticSkill`, or define the narrower `ImpactLookupAccessor`? Recommendation: define `ImpactLookupAccessor` (narrow interface, mirrors Phase 71 pattern). The daemon wires it by wrapping `integSemanticLookup`.

### New Fields + Setters in skill.go

```go
// Phase 72 additions in SemanticSkill struct:
clusterMap      ClusterMapAccessor
clusterMember   ClusterMemberAccessor
clusterPageRank ClusterPageRankAccessor
impactLookup    ImpactLookupAccessor

// Setters (follow SetSymbolByName pattern):
func (s *SemanticSkill) SetClusterMap(a ClusterMapAccessor)
func (s *SemanticSkill) SetClusterMember(a ClusterMemberAccessor)
func (s *SemanticSkill) SetClusterPageRank(a ClusterPageRankAccessor)
func (s *SemanticSkill) SetImpactLookup(a ImpactLookupAccessor)
```

All four fields are nil-guarded in handlers (graceful degrade to empty response + FallbackReason, not panics).

---

## Common Pitfalls

### Pitfall 1: Reusing capConfidences from blast_radius_strangler.go

**What goes wrong:** Importing `internal/kernel/symbols` from `internal/skill/semantic/` violates the `vet-nokernel2semantic` boundary. `capConfidences` in `blast_radius_strangler.go` operates on `*BlastRadius` (kernel type) and cannot be imported.

**Prevention:** Define a fresh `capEdgeConfidences(edges []ImpactEdge, cap float64)` in `tools_change_impact.go`. It is ~5 LOC and trivially correct.

**Warning signs:** Import of `internal/kernel` in `go build ./internal/skill/semantic/` fails vet-nokernel2semantic gate.

### Pitfall 2: ClusterSummary.Score is MemberCount, not a PageRank score

**What goes wrong:** `overlay.go:835` overloads `score` with the planning-time MemberCount. Using `score` as a ranking signal in `get_cluster_map` is CORRECT (sort by score DESC = sort by MemberCount DESC). But using it as a per-member PageRank value is WRONG.

**Prevention:** Per-member PageRank comes from `semantic_graph_scores` (separate table), not `semantic_clusters.score`.

**Warning signs:** Top-N clusters have wildly inconsistent member counts that don't match their purported PageRank scores.

### Pitfall 3: cluster_id token must encode graphVersion, not snapshotID

**What goes wrong:** If `cluster_id` encodes `snapshot_id` instead of `graph_version`, the stale-id check fails silently — snapshot IDs are monotonic but clusters are keyed by `graph_version`, not `snapshot_id`. The schema primary key is `(repo_id, graph_version, cluster_id)`.

**Prevention:** Token format is `"{projection}:{graph_version}:{cluster_int_id}"`. Freshness envelope carries `GraphVersion` (from `assembleFreshness`). Stale-id check compares `decoded.GraphVersion != currentGV` (from `StoreAccessor.CurrentGraphVersion`).

**Warning signs:** `explain_cluster` returns data from a different graph version; cross-tool envelope `graph_version` disagreement in integration test.

### Pitfall 4: ExpandFrom depth is not the same as get_change_impact_graph reached_depth

**What goes wrong:** `ExpandFrom` expands ALL nodes within depth BFS hops. After node/edge capping, the result may appear shallower. Setting `reached_depth = maxDepth` is always correct as an upper bound but may mislead agents.

**Prevention:** If `integ.Impact` does not carry a `Depth` field, set `reached_depth = args.MaxDepth` and document that it represents the expansion depth requested, not the truncated result depth. If `Impact` does carry depth, use `max(impact.Depth)` over capped nodes.

**Warning signs:** Agents see `reached_depth = 5` but `nodes_count = 1` (seed only) because all expansion was truncated.

### Pitfall 5: surfaceToInternalKinds is inline in tools_validate_edge.go

**What goes wrong:** The reverse surface→internal mapping is a package-private function in `tools_validate_edge.go`. Phase 72 can call it directly (same package), but the handler using it must be in the same `package semantic`. Do not copy the function body.

**Prevention:** Call `surfaceToInternalKinds(EdgeKindSurface(k))` directly from `tools_change_impact.go`. Both files are in `package semantic`.

### Pitfall 6: Read-only gate test must be extended

**What goes wrong:** `readonly_gate_test.go` only scans `gatedHandlerFiles` (three Phase 71 files). The three Phase 72 handler files must be added to `gatedHandlerFiles`. Forgetting means the D-09 gate does not cover Phase 72 handlers.

**Prevention:** `gatedHandlerFiles` in `readonly_gate_test.go` must be extended in the same task that creates the three new handler files.

---

## Code Examples

### Register pattern (mirror Phase 71) [VERIFIED: register.go:147-161]

```go
func registerGetClusterMap(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "get_cluster_map",
        Description: "Workspace-level weak-component cluster overview (read+).",
    }, kernel.WrapToolSpan(tracer, "get_cluster_map",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetClusterMapArgs) (*mcpsdk.CallToolResult, any, error) {
            return s.handleGetClusterMap(ctx, args), nil, nil
        }))
    server.Registry().Register(&mcp.ToolDef{
        Name:             "get_cluster_map",
        Description:      "Workspace-level weak-component cluster overview (read+).",
        BriefDescription: "Cluster map overview",
        HelpText:         getClusterMapHelp,
    })
}
```

### assembleFreshness (existing, shared) [VERIFIED: tools_explain_symbol.go:357]

All three Phase 72 handlers call `s.assembleFreshness(ctx, repoID)` — no changes needed to this method.

### Nil-guard accessor pattern [VERIFIED: tools_explain_symbol.go:219-226]

```go
if cm := s.getClusterMap(); cm != nil {
    if rows, err := cm.QueryClusterSummaries(ctx, repoID, projection, gv, topN); err == nil {
        // ... process rows
    } else if s.logger != nil {
        s.logger.Warn("get_cluster_map: QueryClusterSummaries failed", "err", err)
    }
}
```

---

## Test Surface

### Per-Tool Unit Test Files (new)

| File | Phase 71 Analog | What to test |
|------|-----------------|--------------|
| `tools_cluster_map_test.go` | `tools_explain_symbol_test.go` | Happy path: fixture with 3 clusters, top-N=2 returns 2; cluster_id token encodes correctly; no-clusters returns empty; FreshnessV2 present; accessor-nil graceful degrade |
| `tools_explain_cluster_test.go` | `tools_explain_symbol_test.go` | Valid token: returns members sorted by PageRank; stale cluster_id returns fallback_reason="stale_cluster_id"; max_members cap; cohesion/conductance values from fixture; FreshnessV2 present |
| `tools_change_impact_test.go` | `tools_validate_edge_test.go` | Happy path: ExpandFrom returns 3 nodes; depth clamping; edge_kinds filter; node cap at 200; truncated=true when exceeded; confidence_cap emitted on degraded path; review+ mode enforcement; resolveSeed not_found short-circuit |

### Cross-Tool Integration Test (extend integration_test.go)

Per D7 (Phase 71): a new function `TestFourTools_ClusterToImpact` in `integration_test.go`:

```
Step 1: handleGetClusterMap → pick top cluster → cluster_id token
Step 2: handleExplainCluster(cluster_id) → pick a representative symbol
Step 3: handleGetChangeImpactGraph(seed={symbol_id: representative}) → assert impact graph
Step 4: Assert FreshnessV2.GraphVersion is identical across all three results
Step 5: Assert explain_cluster.member_count ≥ len(change_impact.nodes) (seed's cluster members ⊇ direct impact neighbors at depth 1)
```

### Read-Only Gate Extension [VERIFIED: readonly_gate_test.go:46-50]

Extend `gatedHandlerFiles` in `readonly_gate_test.go`:

```go
var gatedHandlerFiles = []string{
    "tools_explain_symbol.go",
    "tools_find_related.go",
    "tools_validate_edge.go",
    "tools_cluster_map.go",       // Phase 72 addition
    "tools_explain_cluster.go",   // Phase 72 addition
    "tools_change_impact.go",     // Phase 72 addition
}
```

### Race Testing

All tests MUST pass `go test -race -count=1 ./internal/skill/semantic/`. Phase 71 verifies 6.13s for the full package with 3 tools + integration. Phase 72 adds 3 more tools; expect ~8-10s.

Concurrent invocation test: extend the existing `TestThreeTools_Concurrent` pattern (or add `TestPhase72Tools_Concurrent`) to spin N goroutines hitting each of the three new tools and assert byte-identical responses + zero race detector hits.

### Fixture Extension

The `populated_graph_fixture_test.go` must be extended to expose:
- `ClusterMapAccessor` mock: returns 2-3 clusters with known member lists and edge kinds
- `ClusterMemberAccessor` mock: returns per-cluster member rows
- `ClusterPageRankAccessor` mock: returns static PageRank map for each node
- `ImpactLookupAccessor` mock: wraps a canned `ExpandFrom` response (3-4 impacts from a known seed)

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package (no separate framework) |
| Config file | none — `go test` directly |
| Quick run command | `go test ./internal/skill/semantic/ -run TestGetClusterMap -race -count=1` |
| Full suite command | `go test ./internal/skill/semantic/ -race -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| P1TOOL-03 | `get_cluster_map` returns top-N clusters with member counts, rep symbols, dominant edge kinds, FreshnessV2 | unit | `go test ./internal/skill/semantic/ -run TestGetClusterMap -race -count=1` | No — Wave 0 |
| P1TOOL-03 | Mode tier read+ enforced at handler entry | unit | `go test ./internal/skill/semantic/ -run TestGetClusterMap_ModeTier -race -count=1` | No — Wave 0 |
| P1TOOL-04 | `explain_cluster` returns full member list, cohesion/separation, entry points | unit | `go test ./internal/skill/semantic/ -run TestExplainCluster -race -count=1` | No — Wave 0 |
| P1TOOL-04 | stale_cluster_id error when graph_version changed | unit | `go test ./internal/skill/semantic/ -run TestExplainCluster_StaleClustersID -race -count=1` | No — Wave 0 |
| P1TOOL-05 | `get_change_impact_graph` returns subgraph, not file rollup | unit | `go test ./internal/skill/semantic/ -run TestGetChangeImpactGraph -race -count=1` | No — Wave 0 |
| P1TOOL-05 | Mode tier review+ enforced (permission denied in read mode) | unit | `go test ./internal/skill/semantic/ -run TestGetChangeImpactGraph_ReviewTierEnforced -race -count=1` | No — Wave 0 |
| P1TOOL-05 | confidence_cap emitted on degraded path | unit | `go test ./internal/skill/semantic/ -run TestGetChangeImpactGraph_ConfidenceCap -race -count=1` | No — Wave 0 |
| P1TOOL-05 | node/edge cap with truncated=true | unit | `go test ./internal/skill/semantic/ -run TestGetChangeImpactGraph_Truncation -race -count=1` | No — Wave 0 |
| Cross-tool | FreshnessV2.GraphVersion identical across all 3 tools | integration | `go test ./internal/skill/semantic/ -run TestFourTools_ClusterToImpact -race -count=1` | No — Wave 0 |
| D-09 gate | No snapshot-write tokens in 3 Phase 72 handler files | static | `go test ./internal/skill/semantic/ -run TestReadOnlyGate_Phase71Handlers -race -count=1` | MODIFIED — extend gatedHandlerFiles |
| Race-clean | All 6 tools + integration pass under -race | concurrent | `go test ./internal/skill/semantic/ -race -count=1` | Modified |

### Sampling Rate

- **Per task commit:** `go test ./internal/skill/semantic/ -run TestGetClusterMap\|TestExplainCluster\|TestGetChangeImpactGraph -race -count=1`
- **Per wave merge:** `go test ./internal/skill/semantic/ -race -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `tools_cluster_map_test.go` — covers P1TOOL-03 unit
- [ ] `tools_explain_cluster_test.go` — covers P1TOOL-04 unit (including stale_cluster_id)
- [ ] `tools_change_impact_test.go` — covers P1TOOL-05 unit (mode, truncation, cap)
- [ ] `cluster_id_test.go` — covers encode/decode round-trips, stale-id error shape
- [ ] Extend `populated_graph_fixture_test.go` with cluster + impact mocks
- [ ] Extend `integration_test.go` with `TestFourTools_ClusterToImpact`
- [ ] Extend `readonly_gate_test.go` `gatedHandlerFiles` list (3 new files)

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Cluster detection on demand | `RunClusterDetection` persists clusters at graph_version time; handlers read from store | Phase 62 | Handlers don't run detection; they read |
| Raw integer cluster IDs | Opaque composite token `{projection}:{gv}:{id}` | Phase 72 D1 | Stale-ID errors surfaced clearly |
| File-level blast radius only | Subgraph output via get_change_impact_graph | Phase 72 D3 | Agents can traverse impact programmatically |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `*Store.QueryClusterSummaries`, `QueryClusterMembers`, `QueryNodePageRanks` methods do not exist today and must be created | Cluster Data Wiring | If they already exist under different names, the plan wastes a task creating duplicates |
| A2 | `is_exported` column does not exist on `semantic_symbols`; isExported heuristic is the fallback for entry-point detection | Cluster Data Wiring | If the column exists, the implementation can be simpler |
| A3 | `integ.Impact` does not carry a `Depth` field; `reached_depth` will be reported as `maxDepth` (requested) | get_change_impact_graph Traversal | If Impact.Depth exists, use it for a more accurate reached_depth |
| A4 | `integSemanticLookup` handle from `semantic_wiring.go` is injectable into `SemanticSkill` via a new `SetImpactLookup` setter | Mode Tier Wiring / New Accessor Interfaces | If the daemon wiring cannot expose this handle to the semantic skill path, a new daemon wiring step is needed |
| A5 | Type-resolver tier 3 degradation is detected by `impact.Confidence < 0.8` on any returned impact | get_change_impact_graph Traversal | If tier-3 is not reflected in ExpandFrom confidence values, a different detection mechanism is needed |
| A6 | `surfaceToInternalKinds` in `tools_validate_edge.go` is package-accessible from the same `package semantic` | Edge-Kind Enum Mapping | True by Go package visibility rules; confirmed by reading tools_validate_edge.go:207 [VERIFIED] |

**If this table is empty:** n/a — see above.

---

## Open Questions

1. **OQ-1: Exact type_resolver_tier_3 detection predicate**
   - What we know: TYPES-04 caps confidence ≤ 0.6 on degraded paths. `ExpandFrom` returns capped impacts. `EvidenceConstructor` (tier 3) has confidence = 0.80, above the cap.
   - What's unclear: Is the `confidence_cap` reason in D4 tied to "any impact with confidence < 1.0" (too broad) or "specifically the constructor tier path" (needs tier metadata in Impact)?
   - Recommendation: Apply cap when `max(impact.Confidence) < 0.8`. Emit `confidence_cap{value:0.6, reason:"type_resolver_tier_3"}`. If planner needs a tighter predicate, expose a `DegradedPath bool` from ExpandFrom.

2. **OQ-2: ImpactLookupAccessor vs injecting full integ.SemanticLookup**
   - What we know: `integ.SemanticLookup` is already wired in the daemon as `integSemanticLookup`. Phase 71 handlers do not use it (they have their own narrow accessor seams). Phase 72's get_change_impact_graph needs ExpandFrom.
   - What's unclear: Whether the daemon's `integSemanticLookup` handle can be cleanly passed to SemanticSkill via a new setter.
   - Recommendation: Define `ImpactLookupAccessor` (narrow: ExpandFrom + Status). Wire daemon side via `ss.SetImpactLookup(integSemanticLookup)` in `semantic_wiring.go`. No changes to kernel-boundary invariants.

3. **OQ-3: cohesion/conductance persist vs compute**
   - What we know: D4 says compute on demand if not persisted, document cost.
   - What's unclear: For large workspaces (10k+ nodes, 50k+ edges), `QueryEffectiveAdjacency` returning the full graph per explain_cluster call may be costly.
   - Recommendation: For Phase 72, compute on demand via `QueryEffectiveAdjacency`. Add a TODO comment noting the O(E) cost and suggesting a future `intra_edge_count` / `leaving_edge_count` column on `semantic_clusters`. The planner should gate this with a note that production performance testing is needed.

---

## Environment Availability

Step 2.6 SKIPPED (no external dependencies identified — Phase 72 is code-only changes within the existing Go binary; no new CLIs, databases, or external services required).

---

## Sources

### Primary (HIGH confidence)

- `internal/skill/semantic/tools_explain_symbol.go` — handler skeleton, assembleFreshness, getTypeChain, getSymbolEdges pattern, confidence cap, INVARIANT header [VERIFIED: file read]
- `internal/skill/semantic/register.go` — RegisterAll pattern [VERIFIED: file read]
- `internal/skill/semantic/mode_check.go` — checkMode, modeTierRead, modeTierReview [VERIFIED: file read]
- `internal/skill/semantic/accessors.go` — narrow accessor interface pattern, Phase 71 additions [VERIFIED: file read]
- `internal/skill/semantic/skill.go` — SemanticSkill struct, SetXxx setter pattern [VERIFIED: file read]
- `internal/skill/semantic/envelope.go` — FreshnessV2, FreshnessStatus, FreshnessSource [VERIFIED: file read]
- `internal/skill/semantic/edge_kind_surface.go` — EdgeKindSurface enum, MapInternalKind [VERIFIED: file read]
- `internal/skill/semantic/seed_resolve.go` — resolveSeed, SeedInput, Resolution [VERIFIED: file read]
- `internal/skill/semantic/handler_helpers.go` — errorResult, jsonResult [VERIFIED: file read]
- `internal/skill/semantic/readonly_gate_test.go` — gatedHandlerFiles, D-09 gate [VERIFIED: file read]
- `internal/skill/semantic/integration_test.go:1448` — TestThreeTools_CrossConsistency pattern [VERIFIED: file read]
- `internal/semantic/cluster/persist.go` — ClusterStore, ClusterTx, RunClusterDetection [VERIFIED: file read]
- `internal/semantic/cluster/weak.go` — Cluster struct, WeakComponents [VERIFIED: file read]
- `internal/semantic/store/effective_graph.go` — ClusterStatusRow, ClusterStatusForGraphVersion, QueryRankedFiles [VERIFIED: file read]
- `internal/semantic/store/overlay.go` — ClusterSummary, ClusterMemberRow, UpsertClusters, UpsertClusterMembers, ScoreRow, schema comments [VERIFIED: grep]
- `internal/semantic/store/migrations.go:266-289` — semantic_clusters + semantic_cluster_members schema [VERIFIED: file read]
- `internal/semantic/integ/lookup.go` — SemanticLookup.ExpandFrom contract [VERIFIED: file read]
- `internal/kernel/symbols/blast_radius_strangler.go` — capConfidences, analyzeBlastRadiusViaLookup pattern [VERIFIED: file read]
- `internal/semantic/types/resolver.go` — EvidenceKind constants, tier mapping [VERIFIED: file read]
- `.planning/phases/71-p1-single-symbol-read-tools/71-VERIFICATION.md` — verified Phase 71 patterns [VERIFIED: file read]
- `.planning/phases/72-p1-cluster-impact-tools/72-CONTEXT.md` — locked decisions D1-D5 [VERIFIED: file read]

### Secondary (MEDIUM confidence)

- `internal/skill/semantic/tools_validate_edge.go:207` — surfaceToInternalKinds (package-private, accessible in Phase 72 handler) [VERIFIED: file read]
- `internal/skill/semantic/populated_graph_fixture_test.go` — fixture builder pattern to extend [VERIFIED: file read]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages, all existing libraries verified
- Architecture patterns: HIGH — Phase 71 handler skeleton is verified and fully readable
- Cluster data wiring: MEDIUM — existing schema and write-side verified; new read methods are ASSUMED (need creation)
- get_change_impact_graph: HIGH — ExpandFrom contract verified; confidence cap logic is ASSUMED (tier-3 detection predicate)
- Test surface: HIGH — Phase 71 D7 pattern fully verified; fixture extension is clear

**Research date:** 2026-05-17
**Valid until:** 2026-06-17 (stable internal patterns; 30-day estimate)
