# Phase 72: P1 Cluster & Impact Tools — Pattern Map

**Mapped:** 2026-05-17
**Files analyzed:** 12 new/modified files
**Analogs found:** 12 / 12

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/skill/semantic/tools_cluster_map.go` | handler | request-response (read+) | `internal/skill/semantic/tools_explain_symbol.go` | exact |
| `internal/skill/semantic/tools_explain_cluster.go` | handler | request-response (read+) | `internal/skill/semantic/tools_explain_symbol.go` | exact |
| `internal/skill/semantic/tools_change_impact.go` | handler | request-response (review+) | `internal/skill/semantic/tools_validate_edge.go` + `blast_radius_strangler.go` | role-match |
| `internal/skill/semantic/cluster_id.go` | utility | transform | `internal/skill/semantic/seed_resolve.go` | role-match |
| `internal/skill/semantic/accessors.go` | accessor (modified) | request-response | `internal/skill/semantic/accessors.go` (Phase 71 additions) | exact |
| `internal/skill/semantic/skill.go` | skill (modified) | — | `internal/skill/semantic/skill.go` (Phase 71 additions) | exact |
| `internal/skill/semantic/register.go` | registration (modified) | — | `internal/skill/semantic/register.go` | exact |
| `internal/skill/semantic/tools_cluster_map_test.go` | test | — | `internal/skill/semantic/tools_explain_symbol_test.go` | exact |
| `internal/skill/semantic/tools_explain_cluster_test.go` | test | — | `internal/skill/semantic/tools_explain_symbol_test.go` | exact |
| `internal/skill/semantic/tools_change_impact_test.go` | test | — | `internal/skill/semantic/tools_validate_edge_test.go` | exact |
| `internal/skill/semantic/readonly_gate_test.go` | test (modified) | — | `internal/skill/semantic/readonly_gate_test.go` | exact |
| `internal/skill/semantic/integration_test.go` | test (modified) | — | `internal/skill/semantic/integration_test.go` (TestThreeTools_*) | exact |

---

## Pattern Assignments

### `internal/skill/semantic/tools_cluster_map.go` (handler, request-response, read+)

**Analog:** `internal/skill/semantic/tools_explain_symbol.go`

**INVARIANT header** (lines 17–23 of tools_explain_symbol.go — copy and adapt):
```go
// INVARIANT (D-09 / D-13): get_cluster_map MUST NOT touch the snapshot-
// write surface of *Store. Specifically: no Begin/Commit/Abort/Write methods
// on snapshots, and no compactor flush trigger. The grep gate in CI enforces
// the absence of those identifier tokens in this file.
```

**Imports pattern** (lines 1–15 of tools_explain_symbol.go):
```go
package semantic

import (
    "context"

    mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
    "go.opentelemetry.io/otel/trace"

    "github.com/agenthands/helix/internal/guardrails"
    "github.com/agenthands/helix/internal/kernel"
    "github.com/agenthands/helix/internal/mcp"
    "github.com/agenthands/helix/internal/semantic/integ"
    "github.com/agenthands/helix/internal/semantic/types"
)
```
Note: Phase 72 `tools_cluster_map.go` does NOT need `types` or `integ` — omit those. Does need `fmt` for budget clamping. Does NOT import `internal/kernel/symbols` (vet-nokernel2semantic boundary).

**Cap constants pattern** (lines 28–31 of tools_explain_symbol.go):
```go
const (
    clusterMapDefaultTopN = 20
    clusterMapMaxTopN     = 100
    clusterMapMembersPreview = 5
)
```

**Args / Result struct pattern** (lines 34–97 of tools_explain_symbol.go — mirrored shape):
```go
type GetClusterMapArgs struct {
    Projection string `json:"projection,omitempty"`
    TopN       int    `json:"top_n,omitempty"`
}

type ClusterSummaryEntry struct {
    ClusterID              string            `json:"cluster_id"`
    MemberCount            int               `json:"member_count"`
    MembersPreview         []string          `json:"members_preview"`
    RepresentativeSymbols  []string          `json:"representative_symbols"`
    DominantEdgeKinds      []EdgeKindSurface `json:"dominant_edge_kinds"`
}

type GetClusterMapResult struct {
    TotalClusters  int                   `json:"total_clusters"`
    TopN           int                   `json:"top_n"`
    Clusters       []ClusterSummaryEntry `json:"clusters"`
    Freshness      FreshnessV2           `json:"freshness"`
    FallbackReason string                `json:"fallback_reason,omitempty"`
}
```

**Register function pattern** (lines 147–161 of tools_explain_symbol.go):
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

**Handler body pattern** (lines 183–349 of tools_explain_symbol.go — load-bearing order):
```go
func (s *SemanticSkill) handleGetClusterMap(ctx context.Context, args GetClusterMapArgs) *mcpsdk.CallToolResult {
    // 1. Mode-tier check (read+ — every session passes; retained for code-review
    //    visibility and Phase 66 GuardrailMiddleware precedent).
    snap := s.sessionSnapshot(ctx)
    if err := checkMode(snap, modeTierRead); err != nil {
        return errorResult(err.Error())
    }

    // 2. Workspace + repoID.
    ws := s.workspaceKey(ctx)
    repoID := ws.Hash()

    // 3. Clamp top_n.
    topN := args.TopN
    if topN <= 0 { topN = clusterMapDefaultTopN }
    if topN > clusterMapMaxTopN { topN = clusterMapMaxTopN }

    // 4. Read graph version for cluster_id encoding.
    // ...

    // 5. Query cluster summaries via accessor nil-guard (see nil-guard pattern).
    // ...

    // 6. assembleFreshness.
    freshness := s.assembleFreshness(ctx, repoID)

    // 7. Receipt + jsonResult.
    guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered, ...)
    return jsonResult(GetClusterMapResult{ ... })
}
```

**Nil-guard accessor pattern** (lines 219–226 of tools_explain_symbol.go):
```go
if cm := s.getClusterMap(); cm != nil {
    if rows, err := cm.QueryClusterSummaries(ctx, repoID, projection, gv, topN); err == nil {
        // process rows
    } else if s.logger != nil {
        s.logger.Warn("get_cluster_map: QueryClusterSummaries failed", "err", err)
    }
}
```

**assembleFreshness** (lines 357–405 of tools_explain_symbol.go) — call `s.assembleFreshness(ctx, repoID)` unchanged. No modification to that method.

---

### `internal/skill/semantic/tools_explain_cluster.go` (handler, request-response, read+)

**Analog:** `internal/skill/semantic/tools_explain_symbol.go`

**Imports pattern:** Same as tools_explain_symbol.go minus `types`; add `fmt` for stale-id error.

**Args / Result struct pattern**:
```go
type ExplainClusterArgs struct {
    ClusterID  string `json:"cluster_id"`
    MaxMembers int    `json:"max_members,omitempty"`
}

type ClusterMemberEntry struct {
    SymbolID    string  `json:"symbol_id"`
    PageRank    float64 `json:"pagerank"`
    IsEntryPoint bool   `json:"is_entry_point,omitempty"`
}

type ExplainClusterResult struct {
    ClusterID         string               `json:"cluster_id"`
    MemberCount       int                  `json:"member_count"`
    MembersReturned   int                  `json:"members_returned"`
    Members           []ClusterMemberEntry `json:"members"`
    Cohesion          float64              `json:"cohesion"`
    Separation        float64              `json:"separation"`
    DominantEdgeKinds []EdgeKindSurface    `json:"dominant_edge_kinds"`
    EntryPoints       []ClusterMemberEntry `json:"entry_points"`
    Freshness         FreshnessV2          `json:"freshness"`
    FallbackReason    string               `json:"fallback_reason,omitempty"`
}
```

**Stale-ID short-circuit pattern** (mirror not_found short-circuit lines 208–215 of tools_explain_symbol.go):
```go
// After decodeClusterID + getCurrentGraphVersion:
if decoded.GraphVersion != currentGV {
    return jsonResult(ExplainClusterResult{
        FallbackReason: "stale_cluster_id",
        Freshness:      s.assembleFreshness(ctx, repoID),
    })
}
```
This mirrors the ResolutionNotFound short-circuit at lines 209–214 of tools_explain_symbol.go exactly.

**Handler load-bearing order** — same 7-step sequence as tools_explain_symbol.go:
1. `checkMode(snap, modeTierRead)` — MUST be first
2. `decodeClusterID(args.ClusterID)` — parse + stale check
3. Workspace + repoID
4. Accessor reads (members, PageRanks, edges for cohesion/conductance)
5. Compute cohesion/separation in Go
6. `assembleFreshness`
7. `jsonResult`

---

### `internal/skill/semantic/tools_change_impact.go` (handler, request-response, review+)

**Analogs:**
- `internal/skill/semantic/tools_explain_symbol.go` — handler structure
- `internal/kernel/symbols/blast_radius_strangler.go` — `capConfidences` shape (NOT to import; to mimic the 5-LOC pattern)

**Imports pattern:** Same as tools_explain_symbol.go minus `types`; add `integ` for ExpandFrom.

**Cap constants** (mirror tools_explain_symbol.go lines 28–31):
```go
const (
    impactDefaultDepth = 2
    impactMaxDepth     = 5
    impactNodeCap      = 200
    impactEdgeCap      = 500
)
```

**Args / Result structs**:
```go
type GetChangeImpactGraphArgs struct {
    Seed      SeedInput `json:"seed"`
    MaxDepth  int       `json:"max_depth,omitempty"`
    EdgeKinds []string  `json:"edge_kinds,omitempty"`
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
    Reason string  `json:"reason"`
}

type GetChangeImpactGraphResult struct {
    Nodes         []ImpactNode   `json:"nodes"`
    Edges         []ImpactEdge   `json:"edges"`
    Truncated     bool           `json:"truncated"`
    ReachedDepth  int            `json:"reached_depth"`
    NodesCount    int            `json:"nodes_count"`
    EdgesCount    int            `json:"edges_count"`
    ConfidenceCap *ConfidenceCap `json:"confidence_cap,omitempty"`
    Freshness     FreshnessV2    `json:"freshness"`
    FallbackReason string        `json:"fallback_reason,omitempty"`
}
```

**review+ mode check pattern** (lines 186–189 of tools_explain_symbol.go, but with modeTierReview):
```go
snap := s.sessionSnapshot(ctx)
if err := checkMode(snap, modeTierReview); err != nil {
    return errorResult(err.Error())
}
```

**resolveSeed reuse** (lines 195–215 of tools_explain_symbol.go):
```go
ws := s.workspaceKey(ctx)
repoID := ws.Hash()
resolved, err := s.resolveSeed(ctx, ws, args.Seed)
if err != nil {
    return errorResult(err.Error())
}
if resolved.Resolution == ResolutionNotFound {
    return jsonResult(GetChangeImpactGraphResult{
        FallbackReason: "symbol_not_found",
        Freshness:      s.assembleFreshness(ctx, repoID),
    })
}
```

**capEdgeConfidences helper** — local to this file (NOT imported from blast_radius_strangler.go; that would violate vet-nokernel2semantic boundary). Pattern from `blast_radius_strangler.go` lines 370–382:
```go
// capEdgeConfidences clamps Confidence on all edges to ≤ cap.
// Mutates the slice in place (locally owned).
func capEdgeConfidences(edges []ImpactEdge, cap float64) {
    for i := range edges {
        if edges[i].Confidence > cap {
            edges[i].Confidence = cap
        }
    }
}
```

**surfaceToInternalKinds reuse** (tools_validate_edge.go lines 207–231) — call `surfaceToInternalKinds(EdgeKindSurface(k))` directly; same package `semantic`, no copy needed.

**MapInternalKind reuse** (edge_kind_surface.go lines 63–94) — `MapInternalKind(impact.Evidence.Edges[i].Kind)` for edge_kind field.

---

### `internal/skill/semantic/cluster_id.go` (utility, transform)

**Analog:** `internal/skill/semantic/seed_resolve.go`

**Imports pattern** (seed_resolve.go lines 19–26):
```go
package semantic

import (
    "fmt"
    "strconv"
    "strings"

    serr "github.com/agenthands/helix/internal/errors"
)
```

**Encode function** (seed_resolve.go resolveSeed short-circuit shape):
```go
// encodeClusterID produces the opaque composite token consumed by explain_cluster.
// Format: "{projection}:{graph_version}:{cluster_int_id}"
func encodeClusterID(projection string, graphVersion, clusterIntID uint64) string {
    return fmt.Sprintf("%s:%d:%d", projection, graphVersion, clusterIntID)
}
```

**Decode function + error path** (mirrors serr.New pattern from seed_resolve.go lines 81–86):
```go
type decodedClusterID struct {
    Projection   string
    GraphVersion uint64
    ClusterIntID uint64
}

// decodeClusterID parses a cluster_id token. Returns serr.InvalidArgs on
// parse failure (wrong number of parts or non-numeric fields).
func decodeClusterID(token string) (decodedClusterID, error) {
    parts := strings.SplitN(token, ":", 3)
    if len(parts) != 3 {
        return decodedClusterID{}, serr.New(serr.InvalidArgs,
            fmt.Sprintf("cluster_id %q: expected 3 colon-separated parts", token))
    }
    gv, err := strconv.ParseUint(parts[1], 10, 64)
    if err != nil {
        return decodedClusterID{}, serr.New(serr.InvalidArgs,
            fmt.Sprintf("cluster_id %q: graph_version not a uint64: %v", token, err))
    }
    id, err := strconv.ParseUint(parts[2], 10, 64)
    if err != nil {
        return decodedClusterID{}, serr.New(serr.InvalidArgs,
            fmt.Sprintf("cluster_id %q: cluster_int_id not a uint64: %v", token, err))
    }
    return decodedClusterID{Projection: parts[0], GraphVersion: gv, ClusterIntID: id}, nil
}
```

**checkClusterIDFresh** (mirrors ResolutionNotFound short-circuit in seed_resolve.go — returns nil meaning "proceed", non-nil meaning "return this result immediately"):
```go
// checkClusterIDFresh returns a pre-built stale error jsonResult when
// decoded.GraphVersion != currentGraphVersion. Returns nil when fresh.
func checkClusterIDFresh(decoded decodedClusterID, currentGraphVersion uint64, freshness FreshnessV2) *mcpsdk.CallToolResult {
    if decoded.GraphVersion != currentGraphVersion {
        return jsonResult(ExplainClusterResult{
            FallbackReason: "stale_cluster_id",
            Freshness:      freshness,
        })
    }
    return nil
}
```

---

### `internal/skill/semantic/accessors.go` (MODIFIED — add 4 new interfaces)

**Analog:** The existing `// ----- Phase 71-01 additions -----` block (lines 182–221 of accessors.go)

**Section comment pattern** (lines 182–188 of accessors.go):
```go
// ----- Phase 72-0X additions: read-only seams for the P1 cluster & impact tools. -----
//
// These four narrow interfaces are declared in this plan so wave-2 handler
// plans can compile against the seam before the production daemon wiring lands.
// Each interface is independent (Pattern F: narrow-interface convention).
```

**Interface declaration pattern** (lines 196–221 of accessors.go — one method per interface):
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

// ImpactLookupAccessor is the narrow seam for get_change_impact_graph traversal.
// Wraps ExpandFrom and Status to keep the skill layer free of the full
// integ.SemanticLookup interface (Pattern F).
type ImpactLookupAccessor interface {
    ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey,
        sym integ.SymbolID, depth int) ([]integ.Impact, error)
    Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error)
}
```

Also add row types alongside the accessor declarations (mirror `TypeChainRow` at lines 245–249, `SymbolEdgeRow` at lines 258–271):
```go
// ClusterSummaryRow is one cluster row returned by ClusterMapAccessor.
type ClusterSummaryRow struct {
    ClusterIntID uint64
    MemberCount  int
}

// ClusterMemberRow is one member row returned by ClusterMemberAccessor.
type ClusterMemberRow struct {
    NodeID   uint64
    SymbolID string // stable_key
}
```

---

### `internal/skill/semantic/skill.go` (MODIFIED — add 4 fields + 4 setters)

**Analog:** Phase 71 additions to skill.go (lines 36–57 for struct fields, lines 139–194 for setters)

**Struct field additions** (mirror lines 36–57):
```go
// Phase 72 additions: read-only seams for cluster & impact tools.
clusterMap      ClusterMapAccessor
clusterMember   ClusterMemberAccessor
clusterPageRank ClusterPageRankAccessor
impactLookup    ImpactLookupAccessor
```

**Setter pattern** (lines 139–145 for SetSymbolByName as model):
```go
// SetClusterMap wires the ClusterMapAccessor adapter (Phase 72 seam).
func (s *SemanticSkill) SetClusterMap(a ClusterMapAccessor) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.clusterMap = a
}
// (repeat for SetClusterMember, SetClusterPageRank, SetImpactLookup)
```

**Getter helpers** — add private getters following `getTypeChain` (lines 407–412) and `getSymbolEdges` (lines 414–419) pattern:
```go
func (s *SemanticSkill) getClusterMap() ClusterMapAccessor {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.clusterMap
}
// (repeat for getClusterMember, getClusterPageRank, getImpactLookup)
```

---

### `internal/skill/semantic/register.go` (MODIFIED — add 3 register calls)

**Analog:** Lines 19–30 of register.go (the existing RegisterAll body)

**Pattern to extend:**
```go
func RegisterAll(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
    if server == nil || s == nil {
        return
    }
    // ... existing 7 calls ...
    registerGetClusterMap(server, s, tracer)      // Phase 72 addition
    registerExplainCluster(server, s, tracer)     // Phase 72 addition
    registerGetChangeImpactGraph(server, s, tracer) // Phase 72 addition
}
```

---

### `internal/skill/semantic/tools_cluster_map_test.go` (test)

**Analog:** Per-tool test file pattern established in Phase 71

**Harness setup pattern** (integration_test.go lines 1353–1443 — newThreeToolsHarness):
```go
func newClusterMapHarness(t *testing.T, mode string) *clusterMapHarness {
    t.Helper()
    s := &SemanticSkill{}
    _ = s.Init(skill.SkillDeps{})
    sess := &mcp.SessionInfo{SessionID: "test-cluster-map", Mode: mode}
    ws := workspace.WorkspaceKey{RepoRoot: "/tmp/repo-cluster", Language: "go", Toolchain: "go1.22"}
    s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})
    store := &clusterMapStoreRec{t: t, graphVersion: 42, latestSnapshot: 7}
    s.SetStore(store)
    s.SetExtractorRun(...)
    s.SetClusterMap(&fixClusterMapAccessor{...})
    s.SetClusterPageRank(&fixClusterPageRankAccessor{...})
    return &clusterMapHarness{skill: s, store: store}
}
```

**Test case pattern** (inline table-driven test — mirror the existing tools tests):
```go
func TestGetClusterMap_HappyPath(t *testing.T) { ... }
func TestGetClusterMap_ModeTier(t *testing.T) { ... }       // read+ always passes
func TestGetClusterMap_AccessorNil(t *testing.T) { ... }    // nil accessor → graceful degrade
func TestGetClusterMap_FreshnessV2Present(t *testing.T) { ... }
func TestGetClusterMap_TopNClamping(t *testing.T) { ... }
```

**Read-only canary recorder** — embed a recorder struct identical to `threeToolsStoreRec` (integration_test.go ~lines 1370–1374) that fails the test if any write method is called.

---

### `internal/skill/semantic/tools_explain_cluster_test.go` (test)

**Analog:** Same per-tool test pattern; key additions:

```go
func TestExplainCluster_ValidToken(t *testing.T) { ... }       // happy path
func TestExplainCluster_StaleClustersID(t *testing.T) { ... }  // stale_cluster_id error shape
func TestExplainCluster_MaxMembersCap(t *testing.T) { ... }
func TestExplainCluster_CohesionConductance(t *testing.T) { ... }
func TestExplainCluster_FreshnessV2Present(t *testing.T) { ... }
```

**Stale token test** — build a `decodedClusterID` with `GraphVersion=1`, wire store returning `CurrentGraphVersion=2`, assert `FallbackReason=="stale_cluster_id"` and `IsError==false`.

---

### `internal/skill/semantic/tools_change_impact_test.go` (test)

**Analog:** `tools_validate_edge_test.go` pattern; adds review+ enforcement test

```go
func TestGetChangeImpactGraph_HappyPath(t *testing.T) { ... }
func TestGetChangeImpactGraph_ReviewTierEnforced(t *testing.T) { ... } // mode="read" → PermissionDenied
func TestGetChangeImpactGraph_DepthClamping(t *testing.T) { ... }
func TestGetChangeImpactGraph_NodeCap(t *testing.T) { ... }           // truncated=true
func TestGetChangeImpactGraph_EdgeKindsFilter(t *testing.T) { ... }
func TestGetChangeImpactGraph_ConfidenceCap(t *testing.T) { ... }     // degraded path → confidence_cap emitted
func TestGetChangeImpactGraph_SeedNotFound(t *testing.T) { ... }
```

**review+ enforcement test** — set `mode="read"`, call handler, assert `IsError==true` with `PermissionDenied` in text.

---

### `internal/skill/semantic/readonly_gate_test.go` (MODIFIED)

**Analog:** Lines 46–50 of readonly_gate_test.go — extend `gatedHandlerFiles`

**Exact modification:**
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
No other changes to this file.

---

### `internal/skill/semantic/integration_test.go` (MODIFIED — add cross-tool chain test)

**Analog:** `TestThreeTools_CrossConsistency` (lines 1448–1513) and `TestThreeTools_Concurrent` (lines 1518–1590)

**New function pattern** (add after existing TestThreeTools_* functions):
```go
// TestFourTools_ClusterToImpact — D7 (Phase 72). Four-step cross-tool chain:
// get_cluster_map → explain_cluster → get_change_impact_graph.
// Asserts FreshnessV2.GraphVersion identical across all three results.
func TestFourTools_ClusterToImpact(t *testing.T) {
    h := newFourToolsHarness(t)
    ctx := context.Background()

    // Step 1: get_cluster_map — pick top cluster.
    cmRes := h.skill.handleGetClusterMap(ctx, GetClusterMapArgs{TopN: 1})
    // ...unmarshal, assert not IsError...
    clusterID := cm.Clusters[0].ClusterID

    // Step 2: explain_cluster — pick representative symbol.
    ecRes := h.skill.handleExplainCluster(ctx, ExplainClusterArgs{ClusterID: clusterID})
    // ...unmarshal, assert not IsError...
    repSymbol := ec.Members[0].SymbolID

    // Step 3: get_change_impact_graph from representative symbol.
    igRes := h.skill.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
        Seed: SeedInput{SymbolID: repSymbol},
    })
    // ...unmarshal, assert not IsError...

    // Step 4: envelope agreement.
    if cm.Freshness.GraphVersion != ec.Freshness.GraphVersion ||
        ec.Freshness.GraphVersion != ig.Freshness.GraphVersion {
        t.Errorf("graph_version mismatch: cluster_map=%d explain=%d impact=%d",
            cm.Freshness.GraphVersion, ec.Freshness.GraphVersion, ig.Freshness.GraphVersion)
    }

    // Step 5: size sanity — cluster member_count >= 1 (seed exists in cluster).
    if ec.MemberCount < 1 {
        t.Errorf("explain_cluster: zero members")
    }
}
```

**Harness wiring pattern** — extend the `newThreeToolsHarness` pattern (lines 1354–1442) to add the four new Phase 72 accessors via the new SetClusterMap / SetClusterMember / SetClusterPageRank / SetImpactLookup setters, using the same fixture mock approach.

---

## Shared Patterns

### Mode-Tier Check
**Source:** `internal/skill/semantic/mode_check.go` lines 34–74
**Apply to:** All three new handler files (first call in handler body, before any other logic)
```go
snap := s.sessionSnapshot(ctx)
if err := checkMode(snap, modeTierRead); err != nil {  // or modeTierReview for change_impact
    return errorResult(err.Error())
}
```
`modeTierRead` (value 0, lines 21–22): every session passes.
`modeTierReview` (value 1, lines 22–23): passes only when `cur == "review" || cur == "admin"`.

### FreshnessV2 Envelope
**Source:** `internal/skill/semantic/envelope.go` lines 255–262, `tools_explain_symbol.go` lines 357–405
**Apply to:** All three new handler files — call `s.assembleFreshness(ctx, repoID)` unchanged.
```go
type FreshnessV2 struct {
    GraphVersion   uint64          `json:"graph_version,omitempty"`
    SnapshotID     uint64          `json:"snapshot_id,omitempty"`
    ExtractorRunID string          `json:"extractor_run_id,omitempty"`
    AsOfUnixMs     int64           `json:"as_of_unix_ms,omitempty"`
    Status         FreshnessStatus `json:"status,omitempty"`
    Source         FreshnessSource `json:"source,omitempty"`
}
```

### errorResult / jsonResult
**Source:** `internal/skill/semantic/handler_helpers.go` lines 22–44
**Apply to:** All three new handler files — use directly, no changes.
```go
func errorResult(msg string) *mcpsdk.CallToolResult  // IsError=true
func jsonResult(v any) *mcpsdk.CallToolResult         // marshals v, defensive on error
```

### MapInternalKind
**Source:** `internal/skill/semantic/edge_kind_surface.go` lines 63–94
**Apply to:** `tools_cluster_map.go` (dominant edge kinds), `tools_change_impact.go` (edge entries)
```go
MapInternalKind("CALLS")      // → EdgeKindCalls
MapInternalKind("RESOLVES_TO") // → EdgeKindHasType (Pitfall 2: NOT uses_type)
```

### surfaceToInternalKinds
**Source:** `internal/skill/semantic/tools_validate_edge.go` lines 207–231 (package-private, same package)
**Apply to:** `tools_change_impact.go` — call directly for `edge_kinds` input filter. No copy.
```go
internalKinds := surfaceToInternalKinds(EdgeKindSurface(k))
```

### resolveSeed
**Source:** `internal/skill/semantic/seed_resolve.go` lines 71–122
**Apply to:** `tools_change_impact.go` — call `s.resolveSeed(ctx, ws, args.Seed)` directly.

### Accessor Nil-Guard Pattern
**Source:** `internal/skill/semantic/tools_explain_symbol.go` lines 219–226 (typeChain nil guard), 257–288 (symbolEdges nil guard)
**Apply to:** All three new handlers — every accessor access must be nil-guarded with a Warn log on error.
```go
if acc := s.getClusterMap(); acc != nil {
    if rows, err := acc.QueryClusterSummaries(...); err == nil {
        // process
    } else if s.logger != nil {
        s.logger.Warn("get_cluster_map: QueryClusterSummaries failed", "err", err)
    }
}
```

### Read-Only Store Read Pattern (QueryRankedFiles as SQL analog)
**Source:** `internal/semantic/store/effective_graph.go` lines 428–484
**Apply to:** New `*Store.QueryClusterSummaries`, `QueryClusterMembers`, `QueryNodePageRanks` methods — follow the exact pattern:
- nil-guard `s == nil || s.db == nil` at function top
- `s.LatestCommittedSnapshot(ctx, repoID)` → return nil when 0
- Named const SQL query
- `s.db.QueryContext(ctx, q, args...)` + `defer rows.Close()`
- `rows.Next()` scan loop + `rows.Err()` check
- Wrap errors with `fmt.Errorf("QueryXxx(%q, %q): %w", ...)`

### Existing ClusterStatusForGraphVersion SQL Pattern
**Source:** `internal/semantic/store/effective_graph.go` lines 238–294
**Apply to:** `QueryClusterSummaries` — use the same `aggregateClusterStatus` helper shape; filter by `(repo_id, graph_version)` on `semantic_clusters`, sort by `score DESC` (score = MemberCount per overlay.go:835). **Do NOT read `score` as PageRank** (it is MemberCount — Pitfall 2 from RESEARCH.md).

### D-09 Read-Only Gate
**Source:** `internal/skill/semantic/readonly_gate_test.go` lines 36–50
**Apply to:** All three new handler files must have the INVARIANT comment header; `gatedHandlerFiles` must list all six files.

### Receipt Issuance on Success
**Source:** `internal/skill/semantic/tools_explain_symbol.go` lines 342–348
**Apply to:** All three new handler files — call `guardrails.IssueReceiptOnSuccess(...)` before `return jsonResult(result)`.
```go
guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
    guardrails.ContextGatheredScope{
        TargetSymbols:   []integ.SymbolID{...},
        TaskHash:        "...",
        TokenBudgetUsed: ...,
        MaxTokens:       0,
    }, "get_cluster_map")
```

---

## Store Read Methods Pattern (new `internal/semantic/store/` methods)

Three new `*Store` methods must be added following the `QueryRankedFiles` pattern (`effective_graph.go` lines 428–484).

**QueryClusterSummaries SQL shape:**
```sql
SELECT cluster_id, CAST(score AS INTEGER) AS member_count
  FROM semantic_clusters
 WHERE repo_id       = ?
   AND graph_version = ?
 ORDER BY score DESC
 LIMIT ?
```
`score` column holds MemberCount at write time per `overlay.go:835`.

**QueryClusterMembers SQL shape:**
```sql
SELECT node_id, symbol_id
  FROM semantic_cluster_members
 WHERE repo_id       = ?
   AND graph_version = ?
   AND cluster_id    = ?
 ORDER BY node_id ASC
 LIMIT ?
```
Schema from `migrations.go:266-289`: `(repo_id, graph_version, cluster_id, node_id, weight, role)`. `symbol_id` must be joined from `semantic_symbols` via `node_id`.

**QueryNodePageRanks SQL shape:**
```sql
SELECT node_id, score
  FROM semantic_graph_scores
 WHERE repo_id       = ?
   AND score_name    = ?
   AND graph_version = ?
   AND node_id IN (/* placeholders */)
```

---

## No Analog Found

All files have close analogs in the codebase. No entries.

---

## Metadata

**Analog search scope:** `internal/skill/semantic/`, `internal/semantic/store/`, `internal/kernel/symbols/`
**Files scanned:** 14 (tools_explain_symbol.go, envelope.go, mode_check.go, accessors.go, skill.go, register.go, handler_helpers.go, readonly_gate_test.go, integration_test.go, seed_resolve.go, edge_kind_surface.go, tools_validate_edge.go, populated_graph_fixture_test.go, blast_radius_strangler.go, effective_graph.go)
**Pattern extraction date:** 2026-05-17

---

## PATTERN MAPPING COMPLETE

**Phase:** 72 - p1-cluster-impact-tools
**Files classified:** 12
**Analogs found:** 12 / 12

### Coverage
- Files with exact analog: 9
- Files with role-match analog: 3 (`tools_change_impact.go`, `cluster_id.go`, new store methods)
- Files with no analog: 0

### Key Patterns Identified
- All three handlers follow the 7-step load-bearing order from `tools_explain_symbol.go` lines 183–349 exactly: `checkMode` → seed/token resolve → workspace+repoID → accessor reads → post-process → `assembleFreshness` → `jsonResult`
- Accessor interfaces follow Pattern F (one method per interface, nil-guarded in handler) from Phase 71 `accessors.go` lines 182–221 and `skill.go` lines 36–194
- `capEdgeConfidences` is a fresh 5-LOC local helper in `tools_change_impact.go` — NOT imported from `blast_radius_strangler.go` (vet-nokernel2semantic boundary)
- `surfaceToInternalKinds` and `MapInternalKind` are called directly (same package); no copy
- Stale cluster_id error mirrors the `ResolutionNotFound` short-circuit from `tools_explain_symbol.go` lines 209–214
- New `*Store` read methods follow `QueryRankedFiles` pattern from `effective_graph.go` lines 428–484
- `readonly_gate_test.go` `gatedHandlerFiles` slice must be extended to include all 6 Phase 71+72 handler files
- Integration test `TestFourTools_ClusterToImpact` chains `handleGetClusterMap` → `handleExplainCluster` → `handleGetChangeImpactGraph` and asserts `FreshnessV2.GraphVersion` is identical across all three (mirrors `TestThreeTools_CrossConsistency` lines 1448–1513)

### File Created
`/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/.planning/phases/72-p1-cluster-impact-tools/72-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. Planner can now reference analog patterns in PLAN.md files.
