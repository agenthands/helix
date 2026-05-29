# Phase 74: Close Gap — Wire P1 Tool Accessors in Production Daemon - Pattern Map

**Mapped:** 2026-05-29
**Files analyzed:** 5 new/modified files
**Analogs found:** 5 / 5

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/daemon/semantic_wiring.go` (extend lines 252-268 + new adapter structs) | service/adapter | request-response | `internal/daemon/semantic_wiring.go` (existing P0 adapters `semStoreAdapter`, `semQueueAdapter`) | exact |
| `internal/skill/semantic/export_p1_test.go` (extend: add `WiredAccessorsForTest`) | test-export | — | `internal/skill/semantic/export_p1_test.go` (existing `Handle*ForTest` exports, lines 1-60) | exact |
| `internal/daemon/p1_accessor_bootstrap_test.go` (new) | test | request-response | `internal/daemon/bootstrap_test.go` (existing `TestBootstrapRegistersAllTools`) | role-match |
| `internal/skill/semantic/p1_production_wiring_e2e_test.go` (new, or extend `p1_e2e_external_test.go`) | test/E2E | request-response | `internal/skill/semantic/p1_e2e_external_test.go` (Phase 73 full file) | exact |

---

## Pattern Assignments

### `internal/daemon/semantic_wiring.go` — Adapter structs + setters block extension

**Analog:** `internal/daemon/semantic_wiring.go` (existing P0 adapters, lines 383–473; setters block, lines 251–268)

**Imports pattern** (lines 38-61) — these are already present; new adapters do NOT add new imports:

```go
package daemon

import (
    "context"
    // ...
    semanticstore "github.com/agenthands/helix/internal/semantic/store"
    "github.com/agenthands/helix/internal/skill/semantic"
    "github.com/agenthands/helix/internal/semantic/integ"
    // ...
)
```

**Accessor factory constructor shape** (lines 385-401) — every new P1 adapter follows this exact pattern:

```go
// ----- Accessor adapter constructors -----

func (b *semanticBundle) storeAccessor() semantic.StoreAccessor {
    return &semStoreAdapter{store: b.store}
}
func (b *semanticBundle) schedulerAccessor() semantic.SchedulerAccessor {
    return &semSchedulerAdapter{rb: b.scheduler, store: b.store}
}
func (b *semanticBundle) queueAccessor() semantic.QueueAccessor {
    return &semQueueAdapter{q: b.queue}
}
```

New P1 factory constructors follow the same one-liner form:
```go
func (b *semanticBundle) symbolByNameAccessor() semantic.SymbolByNameAccessor {
    return &semP1SymbolByNameAdapter{store: b.store}
}
// ... one per adapter type
```

**Core adapter struct shape** (lines 406-462) — the canonical nil-check + delegate pattern used by ALL adapters:

```go
// semStoreAdapter wraps *semanticstore.Store with the narrow StoreAccessor
// surface the SemanticSkill consumes.
type semStoreAdapter struct {
    store *semanticstore.Store
}

func (a *semStoreAdapter) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
    if a == nil || a.store == nil {
        return 0, nil
    }
    return a.store.LatestCommittedSnapshot(ctx, repoID)
}
```

For P1 adapters that need a type-cast (e.g., `[]string` → `[]integ.SymbolID`), use the test-proven conversion pattern from `p1_e2e_external_test.go:317-330` (see below under test file analog).

**Setters block to extend** (lines 251-268) — the single source of truth for wiring; extend in-place, do NOT split:

```go
b.skill = semantic.GetSemanticSkill()
if b.skill != nil {
    b.skill.SetStore(storeAcc)
    b.skill.SetScheduler(b.schedulerAccessor())
    b.skill.SetQueue(b.queueAccessor())
    b.skill.SetLive(b.liveAccessor())
    b.skill.SetRunner(b.runner)
    b.skill.SetRetrieval(b.retrievalAdapter)
    b.skill.SetCompactor(b.compactorAccessor())
    b.skill.SetSessionAccessor(b.sessionAccessor())
    if logger != nil {
        logger.Info("semantic skill setters wired",
            "setters", 8,  // UPDATE to 14 after Phase 74
            "bleve_subdir", cfg.BleveSubdir,
            "index_timeout", cfg.IndexTimeout,
        )
    }
}
```

After Phase 74 the block gains 6 more calls (D-01) + `SetImpactLookup` + `SetSymbolEdges` + `SetClusterMembership` = 14 total. The `SetImpactLookup` call requires the `integSemanticLookup` struct to be accessible; see the `integLookupAccessor()` factory at line 1377:

```go
// integLookupAccessor returns a SemanticLookup adapter wired to the bundle.
func (b *semanticBundle) integLookupAccessor() integ.SemanticLookup {
    if b == nil {
        return integ.NoopLookup{}
    }
    return &integSemanticLookup{
        bundle:    b,
        store:     b.store,
        retrieval: b.retrievalAdapter,
        rank:      b.scheduler,
        enabledFn: func() bool { return b.store != nil },
        wsKeyFn:   b.wsKeyFn,
    }
}
```

The `integSemanticLookup` struct (line 782) already satisfies `ImpactLookupAccessor`; the setter call is:
```go
b.skill.SetImpactLookup(b.integLookupAccessor())  // *integSemanticLookup satisfies ImpactLookupAccessor
```

**Two-hop adapter pattern** (for `SymbolEdgesAccessor` and `ClusterMembershipAccessor`) — adapters need `QueryNodeIDByStableKey` (effective_graph.go:671) as the first hop:

```go
// *Store.QueryNodeIDByStableKey signature (effective_graph.go:671):
func (s *Store) QueryNodeIDByStableKey(ctx context.Context, repoID, stableKey string) (uint64, bool, error)
```

---

### `internal/skill/semantic/export_p1_test.go` — Add `WiredAccessorsForTest`

**Analog:** `internal/skill/semantic/export_p1_test.go` (lines 1-60, existing `Handle*ForTest` exports)

**File header + package declaration** (lines 1-18) — must be `package semantic` (NOT `package semantic_test`), which is what enables cross-package access from `internal/daemon/` tests:

```go
// export_p1_test.go — Phase 73-04 Task 2: test-only exports of the 6 P1
// handler unexported handler functions so the black-box `package semantic_test`
// E2E suite (p1_e2e_external_test.go) can invoke them without adding a new
// public method onto SemanticSkill.
//
// Pattern: standard Go `export_test.go` — file lives in `package semantic` and
// is compiled only for tests (the `_test.go` suffix), so the exported names are
// visible to the black-box `semantic_test` package without leaking into the
// production API.

package semantic

import (
    "context"

    mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)
```

**Handle*ForTest export shape** (lines 20-60) — pattern to follow for `WiredAccessorsForTest`:

```go
// HandleExplainSymbolDeepForTest invokes the unexported handleExplainSymbolDeep
// handler.
func HandleExplainSymbolDeepForTest(s *SemanticSkill, ctx context.Context, args ExplainSymbolDeepArgs) *mcpsdk.CallToolResult {
    return s.handleExplainSymbolDeep(ctx, args)
}
```

**WiredAccessorsForTest to add** — confirmed field names from `skill.go:27-68`:

```go
// WiredAccessorsBoolMap is a snapshot of which P1 accessor fields are non-nil
// on a SemanticSkill. Used by the D-03 runtime bootstrap test to assert
// production wiring without reflection.
type WiredAccessorsBoolMap struct {
    SymbolByName      bool
    ExtractorRun      bool
    ClusterMembership bool
    TypeChain         bool
    SymbolEdges       bool
    EdgeEvidence      bool
    ClusterMap        bool
    ClusterMember     bool
    ClusterPageRank   bool
    ImpactLookup      bool
}

// WiredAccessorsForTest returns a snapshot of accessor non-nil state under the
// skill mutex. Package-level (not method) so daemon-package tests can call it
// with a *SemanticSkill retrieved via GetSemanticSkill().
func WiredAccessorsForTest(s *SemanticSkill) WiredAccessorsBoolMap {
    s.mu.Lock()
    defer s.mu.Unlock()
    return WiredAccessorsBoolMap{
        SymbolByName:      s.symbolByName != nil,
        ExtractorRun:      s.extractorRun != nil,
        ClusterMembership: s.clusterMembership != nil,
        TypeChain:         s.typeChain != nil,
        SymbolEdges:       s.symbolEdges != nil,
        EdgeEvidence:      s.edgeEvidence != nil,
        ClusterMap:        s.clusterMap != nil,
        ClusterMember:     s.clusterMember != nil,
        ClusterPageRank:   s.clusterPageRank != nil,
        ImpactLookup:      s.impactLookup != nil,
    }
}
```

Field names verified against `skill.go:27-68` — `s.symbolByName`, `s.extractorRun`, `s.clusterMembership`, `s.typeChain`, `s.symbolEdges`, `s.edgeEvidence`, `s.clusterMap`, `s.clusterMember`, `s.clusterPageRank`, `s.impactLookup`.

---

### `internal/daemon/p1_accessor_bootstrap_test.go` (new)

**Analog:** `internal/daemon/bootstrap_test.go` (existing daemon bootstrap test pattern)
**Package:** `package daemon` — same package as `semantic_wiring.go`, so `newSemanticBundle` is accessible directly if `daemon.New` is too heavy.

**Pattern:** Construct a real `*semanticstore.Store` + call `newSemanticBundle` (or `daemon.New`) → retrieve skill via `semantic.GetSemanticSkill()` → call `semantic.WiredAccessorsForTest(skill)` → assert bool map.

Key imports the new test will need:
```go
package daemon

import (
    "testing"

    "github.com/stretchr/testify/require"
    "github.com/agenthands/helix/internal/skill/semantic"
)
```

Assertion style (table-driven, per CONTEXT.md discretion):
```go
// After wiring, assert the 8 folded accessors are non-nil, and the 2 deferred are nil.
wired := semantic.WiredAccessorsForTest(skill)
require.True(t, wired.SymbolByName,      "SetSymbolByName must be wired (D-01)")
require.True(t, wired.ExtractorRun,      "SetExtractorRun must be wired (D-02)")
require.True(t, wired.ClusterMap,        "SetClusterMap must be wired (D-01)")
require.True(t, wired.ClusterMember,     "SetClusterMember must be wired (D-01)")
require.True(t, wired.ClusterPageRank,   "SetClusterPageRank must be wired (D-01)")
require.True(t, wired.ImpactLookup,      "SetImpactLookup must be wired (D-01)")
require.True(t, wired.SymbolEdges,       "SetSymbolEdges must be wired (D-01a FOLD)")
require.True(t, wired.ClusterMembership, "SetClusterMembership must be wired (D-01a FOLD)")
require.False(t, wired.TypeChain,        "TypeChain DEFERRED to Phase 75")
require.False(t, wired.EdgeEvidence,     "EdgeEvidence DEFERRED to Phase 75")
```

---

### `internal/skill/semantic/p1_production_wiring_e2e_test.go` (new) or extension of `p1_e2e_external_test.go`

**Analog:** `internal/skill/semantic/p1_e2e_external_test.go` (Phase 73 full E2E, lines 1-460+)
**Package:** `package semantic_test` (black-box — same as analog, avoids import cycle)

**Imports pattern** (p1_e2e_external_test.go:36-59) — copy verbatim; `daemon` import is needed for `NewIntegSemanticLookupForTest`:

```go
package semantic_test

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/agenthands/helix/internal/daemon"
    "github.com/agenthands/helix/internal/mcp"
    "github.com/agenthands/helix/internal/obs"
    semanticpkg "github.com/agenthands/helix/internal/semantic"
    "github.com/agenthands/helix/internal/semantic/graph"
    "github.com/agenthands/helix/internal/semantic/integ"
    "github.com/agenthands/helix/internal/semantic/retrieval"
    semanticstore "github.com/agenthands/helix/internal/semantic/store"
    "github.com/agenthands/helix/internal/skill"
    "github.com/agenthands/helix/internal/skill/semantic"
    "github.com/agenthands/helix/internal/workspace"
)
```

**Store-backed adapter pattern** (p1_e2e_external_test.go:317-376) — the 5 store-backed adapters from Phase 73 are the exact templates for the 5 new production adapters in `semantic_wiring.go`. Copy the struct+method shape:

```go
// p1StoreSymbolByName wraps *Store.QuerySymbolByName, converting []string → []integ.SymbolID.
type p1StoreSymbolByName struct{ store *semanticstore.Store }

func (a *p1StoreSymbolByName) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
    raw, err := a.store.QuerySymbolByName(ctx, repoID, path, name)
    if err != nil {
        return nil, err
    }
    out := make([]integ.SymbolID, len(raw))
    for i, s := range raw {
        out[i] = integ.SymbolID(s)
    }
    return out, nil
}

// p1StoreExtractorRun wraps *Store.LatestExtractorRunID.
type p1StoreExtractorRun struct{ store *semanticstore.Store }

func (a *p1StoreExtractorRun) LatestExtractorRunID(ctx context.Context, repoID string) (string, error) {
    return a.store.LatestExtractorRunID(ctx, repoID)
}

// p1StoreClusterMap wraps *Store.QueryClusterSummaries.
type p1StoreClusterMap struct{ store *semanticstore.Store }

func (a *p1StoreClusterMap) QueryClusterSummaries(ctx context.Context, repoID, projection string, graphVersion uint64, topN int) ([]semantic.ClusterSummaryRow, error) {
    raw, err := a.store.QueryClusterSummaries(ctx, repoID, projection, graphVersion, topN)
    if err != nil {
        return nil, err
    }
    out := make([]semantic.ClusterSummaryRow, len(raw))
    for i, r := range raw {
        out[i] = semantic.ClusterSummaryRow{ClusterIntID: r.ClusterIntID, MemberCount: r.MemberCount}
    }
    return out, nil
}

// p1StoreClusterMember wraps *Store.QueryClusterMembers.
type p1StoreClusterMember struct{ store *semanticstore.Store }

func (a *p1StoreClusterMember) QueryClusterMembers(ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int) ([]semantic.ClusterMemberRow, error) {
    raw, err := a.store.QueryClusterMembers(ctx, repoID, projection, graphVersion, clusterIntID, limit)
    if err != nil {
        return nil, err
    }
    out := make([]semantic.ClusterMemberRow, len(raw))
    for i, r := range raw {
        out[i] = semantic.ClusterMemberRow{NodeID: r.NodeID, SymbolID: r.SymbolID}
    }
    return out, nil
}

// p1StoreClusterPageRank wraps *Store.QueryNodePageRanks (same return type — no conversion needed).
type p1StoreClusterPageRank struct{ store *semanticstore.Store }

func (a *p1StoreClusterPageRank) QueryNodePageRanks(ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error) {
    return a.store.QueryNodePageRanks(ctx, repoID, projection, graphVersion, nodeIDs)
}
```

**ImpactLookup adapter** (p1_e2e_external_test.go:292-296) — the test creates the production accessor via `daemon.NewIntegSemanticLookupForTest`; in the production wiring block `integLookupAccessor()` is used instead. For the E2E test, the construction is:

```go
impactLookup := daemon.NewIntegSemanticLookupForTest(store, ws)
s.SetImpactLookup(&p1IntegLookupAcc{lookup: impactLookup})
// where p1IntegLookupAcc is the thin adapter already defined in the test file
```

**SymbolEdges direction coverage** (p1_e2e_external_test.go:413-440) — the new E2E must cover all three directions per D-03a. The inline fake is the exact interface contract to satisfy in the production adapter:

```go
// CallersOf: CALLS-filtered incoming
func (a *p1InlineSymbolEdges) CallersOf(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
    // edge_kind = 'CALLS' filter; returns {From: caller, To: seed}
}

// IncomingEdgesOf: all edge kinds incoming
func (a *p1InlineSymbolEdges) IncomingEdgesOf(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
    // no edge_kind filter
}

// OutgoingEdgesOf: all edge kinds outgoing
func (a *p1InlineSymbolEdges) OutgoingEdgesOf(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
    // filter on src_node_id = sym.node_id
}
```

**buildP1E2EFixture** — reuse directly; it is already exported from the test file and provisions the full real `*Store` + bleve + 7 symbols + 3 CALL edges + clusters. The new E2E test calls `buildP1E2EFixture(t)` then replaces the 4 inline fakes (SymbolEdges, ClusterMembership, TypeChain, EdgeEvidence) with the new production adapters for the 2 FOLD ones and nil for the 2 DEFER ones.

---

## Shared Patterns

### Nil-safe adapter guard
**Source:** `internal/daemon/semantic_wiring.go:413-417` (every method on every adapter)
**Apply to:** All 8 new P1 adapter methods

```go
func (a *semXxxAdapter) MethodName(ctx context.Context, repoID string, ...) (...) {
    if a == nil || a.store == nil {
        return <zero>, nil
    }
    return a.store.MethodName(ctx, repoID, ...)
}
```

### Type conversion (store result → skill accessor type)
**Source:** `internal/skill/semantic/p1_e2e_external_test.go:320-330` (p1StoreSymbolByName)
**Apply to:** `SymbolByNameAccessor` adapter (`[]string` → `[]integ.SymbolID`), `ClusterMapAccessor` adapter (`ClusterSummaryResult` → `ClusterSummaryRow`), `ClusterMemberAccessor` adapter (`ClusterMemberResult` → `ClusterMemberRow`)

Pattern: create output slice, range-iterate raw results, cast/assign struct fields.
`QueryNodePageRanks` returns `map[uint64]float64` on both sides — no conversion needed.

### package semantic_test black-box constraint
**Source:** `internal/skill/semantic/p1_e2e_external_test.go:36` (`package semantic_test`)
**Apply to:** New production-path E2E test file

The E2E must be `package semantic_test` (NOT `package semantic`) to:
1. Avoid import cycles (can import `internal/daemon`)
2. Follow Phase 73 D-04a established black-box pattern
3. Access handler entry points only via `Handle*ForTest` exports

### export_p1_test.go must be `package semantic` (not `package semantic_test`)
**Source:** `internal/skill/semantic/export_p1_test.go:12` (`package semantic`)
**Apply to:** The `WiredAccessorsForTest` addition

This file is `package semantic` (not `_test`) so daemon-package tests can call `semantic.WiredAccessorsForTest(skill)` across package boundaries. If `WiredAccessorsForTest` were in `package semantic_test`, daemon tests would not see it.

---

## *Store Query Method Signatures (reference for adapter implementation)

All in `internal/semantic/store/effective_graph.go`:

```go
// line 595
func (s *Store) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]string, error)

// line 650
func (s *Store) LatestExtractorRunID(ctx context.Context, repoID string) (string, error)

// line 671
func (s *Store) QueryNodeIDByStableKey(ctx context.Context, repoID, stableKey string) (uint64, bool, error)

// line 810
func (s *Store) QueryClusterSummaries(ctx context.Context, repoID, projection string, graphVersion uint64, topN int) ([]ClusterSummaryResult, error)

// line 855
func (s *Store) QueryClusterMembers(ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int) ([]ClusterMemberResult, error)

// line 908
func (s *Store) QueryNodePageRanks(ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error)
```

`ClusterSummaryResult` fields: `ClusterIntID uint64`, `MemberCount int`
`ClusterMemberResult` fields: `NodeID uint64`, `SymbolID string`

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| New SQL inside `semP1SymbolEdgesAdapter` methods | adapter | CRUD | No production adapter for per-symbol directed edge queries exists yet; inline SQL using `semantic_edges` schema. Reference: RESEARCH.md Pattern 2 SQL sketch + migrations.go:204-227 for schema/indexes. |
| New SQL inside `semP1ClusterMembershipAdapter.ClusterIDOf` | adapter | CRUD | No production adapter for node→cluster lookup exists yet; two-hop SQL via `QueryNodeIDByStableKey` + `semantic_cluster_members` JOIN `semantic_clusters`. Reference: RESEARCH.md ClusterMembership SQL sketch + migrations.go:265-290. |

---

## Pitfalls to Avoid (for planner `<action>` sections)

1. **symbol_id vs node_id JOIN:** `semantic_edges.src_node_id` = `semantic_symbols.symbol_id` (NOT `node_id`). Always JOIN `ss.symbol_id = e.src_node_id`.
2. **CallersOf edge_kind filter:** `CallersOf` SQL must add `AND e.edge_kind = 'CALLS'`; `IncomingEdgesOf` uses no edge_kind filter. Same index (`idx_semantic_edges_dst`) serves both.
3. **ClusterMembership graph_version:** The adapter must call `*Store.CurrentGraphVersion(ctx, repoID)` internally — the accessor interface does not carry `graph_version` as a parameter.
4. **Setters count log line:** Update `"setters", 8` → `"setters", 14` at `semantic_wiring.go:263`.
5. **No separate wireP1Accessors() helper:** D-04 locks the single-block invariant; all Set* calls go inline in the existing `if b.skill != nil` block.
6. **WiredAccessorsForTest placement:** Must be in `export_p1_test.go` (package `semantic`, file has `_test.go` suffix) — NOT in `export_p1_wired_test.go` as a new file — so daemon-package tests can import the function.

---

## Metadata

**Analog search scope:** `internal/daemon/`, `internal/skill/semantic/`, `internal/semantic/store/`
**Files scanned:** 6 primary files + 3 supporting
**Pattern extraction date:** 2026-05-29
