# Phase 74: Close gap — wire P1 tool accessors in production daemon - Research

**Researched:** 2026-05-29
**Domain:** Go daemon production wiring — accessor adapters in `internal/daemon/semantic_wiring.go`
**Confidence:** HIGH

## Summary

Phase 74 is a production-wiring phase. The v1.11 audit found that all 10 Phase 71/72
accessor setters have zero non-test callers in the production daemon.
`semantic_wiring.go:252-268` wires only 8 P0 setters; every P1 handler hits its nil-accessor
guard and returns a degraded `fallback_reason: *_unavailable` envelope.

Six of the 10 P1 accessors are immediately wireable: they back onto `*Store` methods that
already exist, are tested, and require only a thin adapter struct following the established
`storeAccessor`/`schedulerAccessor`/`queueAccessor` shape. Two of the remaining four
(`SymbolEdgesAccessor` and `ClusterMembershipAccessor`) can also be folded in Phase 74 because
the underlying `semantic_edges` and `semantic_cluster_members` tables already carry the necessary
columns and indexes. The other two (`TypeChainAccessor` and `EdgeEvidenceAccessor`) require
schema columns that do not exist in Schema v6, making them DEFER candidates.

The recommendation is SPLIT-ON-METHOD: fold 8 of the 10 accessors in Phase 74 (the 6 D-01
plus `SymbolEdgesAccessor` and `ClusterMembershipAccessor`), and defer `TypeChainAccessor` and
`EdgeEvidenceAccessor` to Phase 75.

**Primary recommendation:** Add thin store-backed adapters for 8 accessors to the setters block
at `semantic_wiring.go:252-268`, export `*SemanticSkill.WiredAccessorsForTest()`, add a runtime
bootstrap test asserting non-nil, and drive a production-path E2E reusing
`buildP1E2EFixture`'s data shape against the real `b.skill`.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01 (locked, 6 accessors):** Phase 74 wires the 6 accessors that already have
production `*Store` query methods:
- `SetSymbolByName` ← thin adapter over `*Store.QuerySymbolByName`
- `SetExtractorRun` ← thin adapter over `*Store.LatestExtractorRunID`
- `SetClusterMap` ← thin adapter over `*Store.QueryClusterSummaries`
- `SetClusterMember` ← thin adapter over `*Store.QueryClusterMembers`
- `SetClusterPageRank` ← thin adapter over `*Store.QueryNodePageRanks`
- `SetImpactLookup` ← existing `*integSemanticLookup` at `semantic_wiring.go:782`

**D-01a (research-gated):** The 4 accessors with no production `*Store` query layer
(`TypeChainAccessor`, `SymbolEdgesAccessor`, `EdgeEvidenceAccessor`,
`ClusterMembershipAccessor`) are decided by this research.

**D-02:** Wire `SetExtractorRun` against a thin adapter over
`*Store.LatestExtractorRunID(ctx, repoID)`. NOT an IndexRunner-cached run id.

**D-03:** Runtime bootstrap test constructs a real `semanticBundle` via the daemon
factory path, asserts every wired accessor on `b.skill` is non-nil after construction.
NOT a static source-scan.

**D-03a:** Lightweight production-path E2E reusing `buildP1E2EFixture` data shape against
the production `b.skill` (not test-only `Set*` fixtures), calling `Handle*ForTest`.
NOT a full daemon-boot + MCP-client invocation.

**D-03b:** For accessors that stay nil, the lightweight E2E asserts the documented
`fallback_reason` is emitted.

**D-04:** Extend the existing setters block at `semantic_wiring.go:252-268`. NOT a
separate `wireP1Accessors()` helper. Update the `"setters", 8` log line.

**D-04a:** New adapters follow the existing `storeAccessor`/`schedulerAccessor`/`queueAccessor`
shape. Workspace-key → repoID translation reuses the existing helper.

**D-05:** WR-04 / WR-05 (Phase 73 tech debt) are OUT OF SCOPE for Phase 74.

### Claude's Discretion

- Exact adapter struct names and file placement (one file per adapter vs grouping by Phase
  71/72 origin) — planner picks.
- Exact assertion style for the bootstrap-test non-nil gate (table-driven vs explicit subtests).

### Deferred Ideas (OUT OF SCOPE)

- WR-04: `representative_symbols`/`members_preview` emit decimal node-ID strings.
- WR-05: `computeDominantEdgeKinds` permanently returns empty.
- Phase 71 deferred bounded-label metric instrumentation.
- Full daemon-boot + MCP-client E2E.
- Closing the 4 missing-query accessors if research recommends a split (Phase 75).
</user_constraints>

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Accessor adapter construction | API / Backend (daemon layer) | — | `semantic_wiring.go` is the single production wiring file per D-04 |
| `*Store` SQL execution | Database / Storage | — | All 8 FOLD adapters delegate to `*semanticstore.Store` methods |
| Runtime bootstrap test | API / Backend (daemon package) | — | Must construct a real `semanticBundle` via `daemon.newSemanticBundle` |
| Production-path E2E test | API / Backend (skill package, black-box) | — | Lives in `package semantic_test` to avoid import cycles; uses `Handle*ForTest` exports |
| `WiredAccessorsForTest` export | API / Backend (skill package) | — | Added to `internal/skill/semantic/` alongside existing `export_p1_test.go` exports |

---

## Standard Stack

No new external packages. Phase 74 uses only the existing internal package surface.

### Core (all [VERIFIED: codebase inspection])

| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `*semanticstore.Store` | `internal/semantic/store/` | SQL backing for all 8 FOLD adapters | Single store for all semantic data; accessor interfaces declared against it |
| `semantic_wiring.go` | `internal/daemon/` | Setters block (lines 252-268) | Established pattern per D-04; no separate helper |
| `internal/skill/semantic/accessors.go` | — | All 10 P1 interface declarations | Seam definitions Phase 74 must satisfy |
| `internal/skill/semantic/skill.go:150-237` | — | 10 `Set*` methods to call | Direct counterparts to adapter construction |
| `*integSemanticLookup` | `semantic_wiring.go:782` | Already satisfies `ImpactLookupAccessor` | No new adapter needed; only `SetImpactLookup` call missing |

### Package Legitimacy Audit

> No external packages are introduced. All dependencies are existing in-tree packages.
> This section is intentionally empty.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

---

## D-01a: Missing-Query Accessor Schema Audit [VERIFIED: codebase inspection]

This is the primary research-gated decision. For each of the 4 accessors with no production
`*Store` method, the table below maps the test-fake location, the underlying schema gap,
estimated SQL complexity, and freshness contract.

### Schema v6 fact baseline

The current schema (migrations v1-v6) has:

- `semantic_symbols` — `(snapshot_id, symbol_id, node_id, stable_key, name, kind, …)` with
  indexes on `(snapshot_id, stable_key)`, `(snapshot_id, name)`
- `semantic_edges` — `(snapshot_id, edge_id, src_node_id, dst_node_id, edge_kind, weight,
  confidence, evidence_ref_id, evidence_file_id, source, reason)` with indexes on
  `(snapshot_id, src_node_id, edge_kind)` and `(snapshot_id, dst_node_id, edge_kind)`
- `semantic_references` — `(snapshot_id, ref_id, node_id, resolved_symbol_id, ref_kind,
  start_line, start_col, end_line, end_col, …)`
- `semantic_cluster_members` — `(repo_id, graph_version, cluster_id, node_id, weight, role)`
  PK `(repo_id, graph_version, cluster_id, node_id)`
- `semantic_clusters` — `(repo_id, graph_version, cluster_id, algorithm, score, …)`
  where `score` is overloaded to carry member count (per overlay.go comment at line 800)

No columns for `tree_sitter_kind`, `evidence_kind` / `tier` (type-resolver ladder), or a
dedicated type-chain table exist in any migration. [VERIFIED: full migrations.go scan, zero hits]

### Accessor gap table

| accessor.method | fake-impl location | *Store schema gap | SQL complexity | freshness contract |
|---|---|---|---|---|
| `TypeChainAccessor.TypeChainForSymbol` | `tools_explain_symbol_test.go:90-100` | **No type-chain table.** `semantic_edges` carries `edge_kind` and `source` but NOT `Tier` (type-resolver ladder string), `EvidenceKind`, or any per-symbol chain row. Schema would need a new `semantic_type_chains` table or new columns on `semantic_symbols`/`semantic_edges`. | **L** — new table + new extraction pipeline needed | Latest committed snapshot |
| `SymbolEdgesAccessor.CallersOf` | `tools_explain_symbol_test.go:113-118` | `semantic_edges` has `src_node_id`, `dst_node_id`, `edge_kind`, `source`. Missing: per-method SQL for `dst_node_id = sym.node_id` filtered to CALLS. Needs `QueryNodeIDByStableKey` (already exists at effective_graph.go:671) then edge query + reverse stable-key lookup. Indexes `idx_semantic_edges_dst` already exist. | **M** — two-hop: stable_key → node_id → edge rows → stable_keys for From/To | Latest committed snapshot |
| `SymbolEdgesAccessor.IncomingEdgesOf` | `tools_explain_symbol_test.go:119-128` | Same schema as CallersOf; broader filter (all incoming edge kinds, not just CALLS). Same two-hop pattern. | **M** | Latest committed snapshot |
| `SymbolEdgesAccessor.OutgoingEdgesOf` | `tools_explain_symbol_test.go:129-138` | Same schema; filter on `src_node_id = sym.node_id`. Index `idx_semantic_edges_src` exists. | **M** | Latest committed snapshot |
| `EdgeEvidenceAccessor.EvidenceForEdge` | `tools_validate_edge_test.go:86-113` | `semantic_edges` has `source TEXT` and `edge_kind`. **Missing**: `tree_sitter_kind`, `Tier`, `EvidenceKind`, `ASTAttested`, and `Range` (start_line/col/end_line/col per evidence row) are NOT columns in `semantic_edges`. Evidence range would require a JOIN to `semantic_references` via `evidence_ref_id`, but `semantic_references` carries ref-level location, not edge-level citation location. `TreeSitterKind` and `ASTAttested` have no backing column anywhere. | **L** — partial data only from existing schema; full `EdgeEvidenceRow` struct requires columns that do not exist | Latest committed snapshot |
| `ClusterMembershipAccessor.ClusterIDOf` | `tools_find_related_test.go:148-158` | `semantic_cluster_members` has `(repo_id, graph_version, cluster_id, node_id)`. Query: `QueryNodeIDByStableKey` (exists) → `SELECT cluster_id FROM semantic_cluster_members WHERE repo_id=? AND graph_version=? AND node_id=?` → join to `semantic_clusters` for `MemberCount` (via `CAST(score AS INTEGER)`). Both tables exist; PK on cluster_members is `(repo_id, graph_version, cluster_id, node_id)`. No additional index needed — WHERE clause on `node_id` will scan the PK but dataset is small. | **S** — two small lookups, all tables + PK exist | Latest `CurrentGraphVersion` per `semStoreAdapter` |

### Fold vs Defer decision per accessor

| Accessor | Decision | Rationale |
|---|---|---|
| `TypeChainAccessor` | DEFER → Phase 75 | Schema gap is large (new table). Cannot implement TypeChainRow.Tier/EvidenceKind from existing columns. |
| `SymbolEdgesAccessor` (all 3 methods) | FOLD → Phase 74 | All required tables/indexes exist. Two-hop SQL is M-complexity but straightforward. Existing helpers (`QueryNodeIDByStableKey`, `QueryStableKeyByNodeID`) already cover the translation pattern. |
| `EdgeEvidenceAccessor` | DEFER → Phase 75 | Schema gap: `tree_sitter_kind`, `Tier`, `EvidenceKind`, `ASTAttested` columns don't exist. A partial implementation returning only `Source` and `InternalKind` would degrade `evidence_status` to "partial" on every call — not the production-quality wiring Phase 74 targets. |
| `ClusterMembershipAccessor` | FOLD → Phase 74 | S-complexity. Tables exist. `QueryNodeIDByStableKey` is the only helper needed before the two-query lookup. |

---

## Architecture Patterns

### System Architecture Diagram

```
                           ┌──────────────────────────┐
                           │   MCP tool call           │
                           │  (P1 handler invoked)     │
                           └────────────┬─────────────┘
                                        │ Handle*ForTest / real MCP dispatch
                                        ▼
                           ┌──────────────────────────┐
                           │   SemanticSkill           │
                           │  (internal/skill/semantic) │
                           │                          │
                           │  symbolByName accessor   │──→ semP1SymbolByNameAdapter
                           │  extractorRun accessor   │──→ semP1ExtractorRunAdapter
                           │  clusterMap accessor     │──→ semP1ClusterMapAdapter
                           │  clusterMember accessor  │──→ semP1ClusterMemberAdapter
                           │  clusterPageRank accessor│──→ semP1ClusterPageRankAdapter
                           │  impactLookup accessor   │──→ *integSemanticLookup (existing)
                           │  symbolEdges accessor    │──→ semP1SymbolEdgesAdapter (NEW)
                           │  clusterMembership acc.  │──→ semP1ClusterMembershipAdapter (NEW)
                           │  typeChain accessor      │──→ nil (DEFERRED Phase 75)
                           │  edgeEvidence accessor   │──→ nil (DEFERRED Phase 75)
                           └──────────────┬───────────┘
                                          │
                               adapter.method(ctx, repoID, ...)
                                          ▼
                           ┌──────────────────────────┐
                           │  *semanticstore.Store     │
                           │  (internal/semantic/store) │
                           │                          │
                           │  QuerySymbolByName        │
                           │  LatestExtractorRunID     │
                           │  QueryClusterSummaries    │
                           │  QueryClusterMembers      │
                           │  QueryNodePageRanks       │
                           │  QueryNodeIDByStableKey   │  ← two-hop for SymbolEdges
                           │  QueryStableKeyByNodeID   │  ← reverse lookup for edge rows
                           │  CurrentGraphVersion      │  ← for ClusterMembership
                           └──────────────────────────┘
                                          │
                                      DuckDB SQL
                                          │
                    ┌─────────────────────┴──────────────────────┐
                    │               Schema v6                    │
                    │  semantic_symbols, semantic_edges,          │
                    │  semantic_cluster_members, semantic_clusters │
                    └────────────────────────────────────────────┘
```

### Recommended Project Structure

All new code lands in `internal/daemon/semantic_wiring.go` (adapters) and the
setter calls in the existing block at lines 252-268. Test additions go in:

```
internal/daemon/
├── semantic_wiring.go              # Extend: 8 new adapter structs + Set* calls
│                                   #   (lines 252-268 block)
└── p1_accessor_bootstrap_test.go   # NEW: D-03 runtime bootstrap test

internal/skill/semantic/
├── export_p1_test.go               # Already has Handle*ForTest exports
├── skill.go                        # POSSIBLY: add WiredAccessorsForTest() export
├── p1_accessor_e2e_test.go         # NEW: D-03a production-path E2E (package semantic_test)
│                                   # (or extend p1_e2e_external_test.go)
└── (export file for WiredAccessors)# NEW: export_p1_wired_test.go or extend export_p1_test.go
```

### Pattern 1: Existing P0 Adapter Shape (mandatory pattern for all P1 adapters)

```go
// Source: internal/daemon/semantic_wiring.go:406-412 (semStoreAdapter)
type semP1SymbolByNameAdapter struct {
    store *semanticstore.Store
}

func (a *semP1SymbolByNameAdapter) QuerySymbolByName(
    ctx context.Context, repoID, path, name string,
) ([]integ.SymbolID, error) {
    if a == nil || a.store == nil {
        return nil, nil
    }
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
```

[VERIFIED: internal/daemon/semantic_wiring.go:385-400 — establishes the nil-check + delegate pattern]

### Pattern 2: Two-Hop Adapter (SymbolEdgesAccessor and ClusterMembershipAccessor)

```go
// Source: internal/daemon/semantic_wiring.go (new pattern for Phase 74)
// Mirrors QueryNodeIDByStableKey (effective_graph.go:671) + edge SQL.

func (a *semP1SymbolEdgesAdapter) CallersOf(
    ctx context.Context, repoID string, sym integ.SymbolID,
) ([]semantic.SymbolEdgeRow, error) {
    if a == nil || a.store == nil {
        return nil, nil
    }
    nodeID, ok, err := a.store.QueryNodeIDByStableKey(ctx, repoID, string(sym))
    if err != nil || !ok {
        return nil, err
    }
    // SELECT src_node_id, dst_node_id, edge_kind FROM semantic_edges WHERE
    //   snapshot_id = <latest> AND dst_node_id = nodeID AND edge_kind = 'CALLS'
    // Then QueryStableKeyByNodeID for src_node_id → From stable_key
    // ...
}
```

[VERIFIED: QueryNodeIDByStableKey at effective_graph.go:671; idx_semantic_edges_dst at migrations.go:224]

### Pattern 3: ImpactLookup Setter (trivial — only call is missing)

```go
// Source: internal/daemon/semantic_wiring.go:782 (integSemanticLookup already satisfies interface)
// In the setters block at line 268, add:
if b.lookup != nil {
    b.skill.SetImpactLookup(b.lookup)  // *integSemanticLookup already satisfies ImpactLookupAccessor
}
```

[VERIFIED: 74-CONTEXT.md D-01 + accessors.go:401-412 ImpactLookupAccessor methods match integSemanticLookup.ExpandFrom + Status]

### Pattern 4: WiredAccessorsForTest Export

```go
// Source: internal/skill/semantic/export_p1_test.go (existing export pattern)
// New addition in export_p1_test.go (or a sibling file):
type WiredAccessorsBoolMap struct {
    SymbolByName       bool
    ExtractorRun       bool
    ClusterMembership  bool
    TypeChain          bool
    SymbolEdges        bool
    EdgeEvidence       bool
    ClusterMap         bool
    ClusterMember      bool
    ClusterPageRank    bool
    ImpactLookup       bool
}

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

[VERIFIED: skill.go:145-241 — all field names confirmed; export pattern from export_p1_test.go:1-60]

### Anti-Patterns to Avoid

- **Splitting the setters block:** D-04 locks the single-block invariant. No separate
  `wireP1Accessors()` function.
- **Using reflection for the bootstrap test:** D-03 "Specific Ideas" note explicitly prohibits
  reflection. Use the `WiredAccessorsForTest` export instead.
- **Importing `internal/semantic/store` from `internal/skill/semantic/`:** The E2E test lives
  in `package semantic_test` (black-box) specifically to avoid import cycles, and can import
  daemon. Do not change this; follow the `p1_e2e_external_test.go` pattern exactly.
- **Constructing a full daemon in bootstrap test:** Use `newSemanticBundle` (or a minimal
  store-backed construction path) to avoid full daemon startup complexity. Mirror how
  `NewIntegSemanticLookupForTest` wraps only the needed internals.
- **Passing workspace.WorkspaceKey to adapters:** The accessor interfaces receive `repoID string`
  directly (the handler calls `ws.Hash()` before calling the accessor). Adapters only need
  `*semanticstore.Store`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| stable_key → uint64 node_id translation | custom SQL | `*Store.QueryNodeIDByStableKey` (effective_graph.go:671) | Already exists, tested, nil-safe |
| uint64 node_id → stable_key translation | custom SQL | `*Store.QueryStableKeyByNodeID` (effective_graph.go:704) | Already exists, needed for SymbolEdgesAccessor From/To fields |
| Current graph version for ClusterMembership | re-implement | `*Store.CurrentGraphVersion` (overlay.go:984) | Already exposed on semStoreAdapter |
| Latest committed snapshot ID | custom query | `*Store.LatestCommittedSnapshot` | Used by all existing adapters |
| IntegSemanticLookup construction for tests | new constructor | `daemon.NewIntegSemanticLookupForTest` (integ_lookup_export.go:66) | Production constructor already exported for test use |

**Key insight:** The Store layer already provides all the building blocks. Phase 74 adds
adapters that wire these into the accessor interface; no new SQL primitives are needed for the
FOLD scope.

---

## Secondary Research Findings

### Bootstrap Test Scaffolding (D-03)

The existing `internal/daemon/bootstrap_test.go` calls `daemon.New(cfg, logger)` and then
inspects the tool registry. Phase 74's D-03 bootstrap test is different: it must assert that
specific accessor fields on `b.skill` are non-nil after construction. Since `b.skill` is the
registered `*SemanticSkill` singleton (retrieved via `semantic.GetSemanticSkill()`), the test can:

1. Call `daemon.New(cfg, logger)` (same pattern as `TestBootstrapRegistersAllTools`) to trigger
   the full wiring path including the setters block.
2. Retrieve the skill via `semantic.GetSemanticSkill()`.
3. Call `semantic.WiredAccessorsForTest(skill)` (the new export per Pattern 4 above).
4. Assert the returned bool map has the expected values.

The complication: `daemon.New` initialises a real `*semanticstore.Store` backed by DuckDB in
a temp dir. The bootstrap test must provide a real temp dir and a valid semantic config path.
The existing `newTestConfig(t)` helper in `bootstrap_test.go` already sets up a `t.TempDir()`
config — but semantic config is separate (DuckDB path, bleve path). The test will need a
minimal semantic config pointing at a `t.TempDir()`.

An alternative approach that avoids full daemon boot: call `newSemanticBundle(...)` directly
(it's package-private) with a constructed `*semanticstore.Store` from a temp dir. This is
lighter. The planner should decide which path to take based on whether `newSemanticBundle`
is easily accessible in `daemon_test.go` (same package).

[VERIFIED: bootstrap_test.go:1-119; semantic_wiring.go:200-270 — `newSemanticBundle` signature, `GetSemanticSkill()` at skill.go:398-400]

### E2E Fixture Reuse (D-03a)

`buildP1E2EFixture(t)` in `p1_e2e_external_test.go:114-311` builds:
- A real `*semanticstore.Store` in `t.TempDir()` (DuckDB)
- A real `*retrieval.Engine` (bleve) in same temp dir
- 7 symbols (Go + TS + Java), 3 CALL edges, scores, clusters committed
- A `*semantic.SemanticSkill` wired with test-only fakes for the 4 missing-query accessors
  (`p1InlineTypeChain`, `p1InlineSymbolEdges`, `p1InlineEdgeEvidence`, `p1InlineClusterMembership`)
  and store-backed adapters for the 6 D-01 accessors

Phase 74's production-path E2E (D-03a) must:
1. Build the same data shape (same symbols/edges/clusters) using `buildP1E2EFixture` directly.
2. Replace test-only `Set*` calls with the production adapters from `semantic_wiring.go`.
3. For the 2 FOLD accessors (`SymbolEdges`, `ClusterMembership`), wire the new production
   adapters against the fixture's real `*Store`.
4. For the 2 DEFER accessors (`TypeChain`, `EdgeEvidence`), leave nil and assert
   `fallback_reason` is the documented `*_unavailable` value (D-03b).

The existing `Handle*ForTest` exports in `export_p1_test.go` are already correct for this.
The E2E lives in `package semantic_test` (same as `p1_e2e_external_test.go`) and can import
`daemon` for `NewIntegSemanticLookupForTest`.

[VERIFIED: p1_e2e_external_test.go full read; export_p1_test.go:9-58]

### Workspace → repoID Resolution

The accessor interfaces take `repoID string` as a parameter. The handler resolves it via
`ws.Hash()` (e.g., `tools_explain_symbol.go:194`, `tools_cluster_map.go:161`). The daemon
adapter structs do NOT need to hold a `WorkspaceKey` or call `Hash()` themselves — they
receive `repoID` from the interface call. This is identical to how the P0 adapters work
(`semStoreAdapter` takes `repoID string` on every method call). [VERIFIED: tools_explain_symbol.go:194]

New P1 adapters therefore follow the exact same shape: struct holds `store *semanticstore.Store`,
methods take `repoID string`, delegate to `store.QueryXxx(ctx, repoID, ...)`.

### SymbolEdgesAccessor Direction Partitions

The `p1InlineSymbolEdges` fake in `p1_e2e_external_test.go:413-440` covers all three
direction partitions against the seeded data:

| Method | Test symbol | Returns |
|---|---|---|
| `CallersOf` | `p1SeedGoHandle` | `[{ServeHTTP → handle, CALLS}]` |
| `IncomingEdgesOf` | `p1SeedGoHandle` | same as callers (supersets callers) |
| `OutgoingEdgesOf` | `p1SeedGoServeHTTP` | `[{ServeHTTP → handle, CALLS}]` |

Phase 74's production E2E MUST cover at least one tool per direction partition (per D-03a
note in CONTEXT.md). `explain_symbol_deep` exercises all three directions (callers, incoming,
outgoing) against `p1SeedGoHandle` (callers path) and `p1SeedGoServeHTTP` (outgoing path).
The planner should select at least:
- `HandleExplainSymbolDeepForTest` with seed `p1SeedGoHandle` → exercises CallersOf + IncomingEdgesOf
- `HandleExplainSymbolDeepForTest` with seed `p1SeedGoServeHTTP` → exercises OutgoingEdgesOf

[VERIFIED: p1_e2e_external_test.go:413-440]

### Build/Vet Gates

- **`vet-nokernel2semantic`**: Blocks `internal/kernel/*` importing `internal/semantic/*`. Phase
  74 adds code in `internal/daemon/` (which already imports `internal/semantic/store`) — no
  new violations. [VERIFIED: lint/nokernel2semantic: checks `internal/kernel/` prefix]
- **`vet-noduckdb`**: Blocks DuckDB imports outside `internal/semantic/store/`. New adapter
  files in `internal/daemon/` already have this pattern (they delegate to `*Store` methods,
  never import duckdb directly). [VERIFIED: lint/noduckdb: allowedPkgPrefix = "internal/semantic/store"]
- **Race-clean**: All existing P0 adapters are stateless (hold only a `store` pointer) and
  lock-free. P1 adapters follow the same shape — no shared mutable state, so no race issues.
- **`go vet ./...` and `go test ./...`**: Required per CLAUDE.md before declaring work done.

[VERIFIED: Makefile:20-44; CLAUDE.md]

---

## Common Pitfalls

### Pitfall 1: node_id vs symbol_id confusion in semantic_symbols
**What goes wrong:** `semantic_symbols` has both `symbol_id` (uint64, PK) and `node_id`
(uint64). `QueryNodeIDByStableKey` returns `symbol_id` (not `node_id`), and `semantic_edges`
uses `src_node_id`/`dst_node_id` which correspond to `semantic_symbols.symbol_id` values in
the current fixture (the p1E2E fixture sets `NodeID: p1NodeServeHTTP` and `SymbolID:
p1NodeServeHTTP` to the same value). However, the field names matter: join on
`semantic_symbols.symbol_id = semantic_edges.src_node_id`. [VERIFIED: snapshot.go:357-371 confirms
INSERT uses symbol_id in the `symbol_id` column AND `node_id` as a separate column]
**Why it happens:** The schema's `semantic_nodes` concept is separate from `semantic_symbols`.
Node IDs and symbol IDs are both uint64 and happen to be equal in fixtures but may diverge.
**How to avoid:** Always JOIN via `symbol_id = src_node_id` (not `node_id = src_node_id`).
Check `QueryNodeIDByStableKey` which queries `SELECT symbol_id FROM semantic_symbols WHERE stable_key=?`.

### Pitfall 2: SymbolEdgesAccessor edge-kind filter for CallersOf vs IncomingEdgesOf
**What goes wrong:** `CallersOf` is specified as "CALLS-filtered IncomingEdgesOf" (accessors.go:278).
If the implementation uses the same SQL for both (no edge_kind filter), `CallersOf` will return
all incoming edges regardless of kind, violating the D2 callers-cap spec.
**Why it happens:** The distinction is in the interface docstring, not the method signature.
**How to avoid:** `CallersOf` SQL: `WHERE edge_kind = 'CALLS'`; `IncomingEdgesOf` SQL: all kinds.
The edge data in `semantic_edges` stores the raw extractor kind in `edge_kind`. The adapter
passes this through as `InternalKind` in `SymbolEdgeRow` — no mapping needed at adapter level.

### Pitfall 3: ClusterMembership graph_version staleness
**What goes wrong:** `ClusterMembershipAccessor.ClusterIDOf` needs `graph_version` to query
`semantic_cluster_members`. The interface signature does NOT take `graph_version` — the adapter
must fetch it. The handler already calls `CurrentGraphVersion` via the store accessor before
calling `ClusterIDOf` (verified in tools_find_related_test.go:453 comment). If the adapter
calls `CurrentGraphVersion` itself AND the handler also calls it, there is a TOCTOU gap.
**Why it happens:** The accessor interface doesn't carry graph_version; it's resolved inside.
**How to avoid:** The adapter should call `*Store.CurrentGraphVersion(ctx, repoID)` internally
(same as the test fake pattern: `recorderClusterMembership` records `gvAtClusterRead` by
reading from the recorder's graphVersion field). The handler's existing
`CurrentGraphVersion`-before-`ClusterIDOf` ordering is a test invariant check, not a
production guarantee. Adapter self-resolution is correct.

### Pitfall 4: Setters block count log line
**What goes wrong:** The `"setters", 8` log line at line 263 must be updated to the new count.
If Phase 74 folds 8 (D-01 × 6 + SymbolEdges + ClusterMembership), the count becomes 14.
**Why it happens:** Manual constant in a log line.
**How to avoid:** Update to `"setters", 14` (or `"setters", 16` if ImpactLookup + a nil-check
setter counts). Be explicit about which setters are being counted. CONTEXT.md says 14 if D-01
only, 18 if D-01a fully folds. With the SPLIT-ON-METHOD recommendation (8 FOLD), the count
is 14. [VERIFIED: semantic_wiring.go:263]

### Pitfall 5: Package cycle in bootstrap test
**What goes wrong:** A bootstrap test in `package daemon` can call `semantic.GetSemanticSkill()`
and then call `semantic.WiredAccessorsForTest(skill)` — but only if
`WiredAccessorsForTest` is in a `_test.go` file in `package semantic` (not `package semantic_test`).
The test in `package daemon` needs it as an exported function from `package semantic`. This
requires the WiredAccessors export to NOT be a black-box test function.
**Why it happens:** Go test package visibility rules: `package X` tests can see `package X`
test-only exports; `package X_test` cannot. Cross-package tests can only see regular exports
or exports in non-`_test.go` files.
**How to avoid:** Place `WiredAccessorsForTest` in a file like
`internal/skill/semantic/export_p1_test.go` (which is `package semantic`, not `package semantic_test`).
That file already uses this pattern for `HandleExplainSymbolDeepForTest` etc.
[VERIFIED: export_p1_test.go:1 — `package semantic` (not `package semantic_test`)]

---

## Code Examples

### Existing `storeAccessor()` helper (establishes the accessor factory pattern)

```go
// Source: internal/daemon/semantic_wiring.go:385-387
func (b *semanticBundle) storeAccessor() semantic.StoreAccessor {
    return &semStoreAdapter{store: b.store}
}
```

### Setters block to extend (current state, lines 252-268)

```go
// Source: internal/daemon/semantic_wiring.go:251-268
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

### QueryNodeIDByStableKey (building block for SymbolEdges + ClusterMembership adapters)

```go
// Source: internal/semantic/store/effective_graph.go:671-695
func (s *Store) QueryNodeIDByStableKey(ctx context.Context, repoID, stableKey string) (uint64, bool, error)
// Returns the symbol_id (uint64) that semantic_edges uses as src_node_id/dst_node_id.
```

### SymbolEdgesAccessor SQL sketch (for planner reference)

```go
// CallersOf: symbols whose outgoing edges point TO sym (CALLS only)
const callersQ = `
    SELECT ss_src.stable_key, ss_dst.stable_key, e.edge_kind
      FROM semantic_edges AS e
      JOIN semantic_symbols AS ss_src
        ON ss_src.snapshot_id = e.snapshot_id AND ss_src.symbol_id = e.src_node_id
      JOIN semantic_symbols AS ss_dst
        ON ss_dst.snapshot_id = e.snapshot_id AND ss_dst.symbol_id = e.dst_node_id
     WHERE e.snapshot_id = ?
       AND e.dst_node_id = ?
       AND e.edge_kind   = 'CALLS'
`
// IncomingEdgesOf: same but without the edge_kind filter
// OutgoingEdgesOf: WHERE src_node_id = ? (no edge_kind filter)
```

[VERIFIED: migrations.go:204-227 — semantic_edges schema + indexes]

### ClusterMembershipAccessor SQL sketch

```go
// ClusterIDOf: look up which cluster contains sym at graph_version
// Step 1: symbol_id → node_id (or symbol_id, same field)
nodeID, ok, err := s.QueryNodeIDByStableKey(ctx, repoID, string(symID))
// Step 2: find cluster
const clusterQ = `
    SELECT cm.cluster_id, CAST(c.score AS INTEGER) AS member_count
      FROM semantic_cluster_members AS cm
      JOIN semantic_clusters AS c
        ON c.repo_id = cm.repo_id AND c.graph_version = cm.graph_version
           AND c.cluster_id = cm.cluster_id
     WHERE cm.repo_id       = ?
       AND cm.graph_version = ?
       AND cm.node_id       = ?
     LIMIT 1
`
```

[VERIFIED: migrations.go:265-290 — semantic_clusters + semantic_cluster_members schemas]

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| P0 setters only (8) | Will be 14 after Phase 74 (SPLIT recommendation) | Phase 74 | P1 tools functional in production |
| Test-only `Set*` wiring for P1 tools | Production daemon wiring | Phase 74 | Closes BLOCKER-1 and BLOCKER-2 from v1.11 audit |
| `FreshnessV2` always `stale`/`unknown` for P1 | Can reach `current` once `SetExtractorRun` is wired | Phase 74 (D-02) | Closes BLOCKER-2 |
| `TypeChain`/`EdgeEvidence` accessor: nil | Remains nil (DEFER to Phase 75) | Phase 75 | `explain_symbol_deep` type_chain empty; `validate_graph_edge` degraded |

**Deprecated/outdated:**
- Phase 73 `wrapper_consistency_test.go` SC#3 gate (static string scan): will coexist with
  Phase 74's runtime bootstrap test (D-03). The static gate stays; the runtime gate adds
  a complementary runtime assertion.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `semantic_edges.edge_kind` stores raw extractor kind strings like `"CALLS"` (uppercase) matching `SymbolEdgesAccessor`'s expected `InternalKind` field | SymbolEdges SQL sketch | If edge_kind uses a different casing/format, CallersOf filter will return empty results |
| A2 | `semantic_symbols.symbol_id` is the same uint64 used as `src_node_id`/`dst_node_id` in `semantic_edges` (not `node_id`) | Pitfall 1 | Wrong JOIN produces empty results |
| A3 | `semantic_clusters.score` stores member count (CAST AS INTEGER) for all graph versions, not just the overlay-computed ones | ClusterMembershipAccessor | If score means something else for some versions, MemberCount will be wrong |
| A4 | The daemon-singleton pattern (`GetSemanticSkill()` returns the same instance before and after `newSemanticBundle` wires it) is stable under Phase 74 | D-03 bootstrap test | If wiring happens before test reads the accessor state, race condition |

---

## Open Questions (RESOLVED)

1. **Bootstrap test construction path**
   - What we know: `TestBootstrapRegistersAllTools` uses `daemon.New(cfg, logger)` which boots
     the full daemon including semantic wiring.
   - What's unclear: Whether `daemon.New` with a minimal config and temp-dir will successfully
     open DuckDB (it creates the `.helix/` subdirectory) without a pre-existing repo — the
     semantic store `Open` call should succeed on an empty path.
   - Recommendation: Try `daemon.New` path first; if DuckDB init fails for some config reason,
     fall back to calling `newSemanticBundle` directly in `package daemon` test.

2. **`lookupAccessor` field on semanticBundle**
   - What we know: `*integSemanticLookup` is at `semantic_wiring.go:782` and satisfies
     `ImpactLookupAccessor`. The `SetImpactLookup` call is missing.
   - What's unclear: Whether `semanticBundle` holds a named field for this struct that the
     setter can reference, or whether it needs to be constructed inline in the setters block.
   - Recommendation: Read `semantic_wiring.go:140-160` (semanticBundle struct fields) to
     confirm. If there's no named field, the planner should add `b.lookup = &integSemanticLookup{...}`
     before the setters block. The CONTEXT.md "Reusable Assets" says it already exists as
     `b.lookupAccessor` — planner should verify the field name during implementation.

---

## Environment Availability

> Step 2.6: No external tools beyond Go toolchain. All dependencies are in-tree.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All code | ✓ | host Go | — |
| DuckDB (via modernc.org/sqlite in store) | Bootstrap test (opens a real *Store) | ✓ | in-tree | — |
| make/vet tools | CI gates | ✓ | in Makefile | `go vet ./...` fallback |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `github.com/stretchr/testify` |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./internal/daemon/... ./internal/skill/semantic/... -count=1 -run TestP1Accessor` |
| Full suite command | `go test ./... -count=1 -race` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BLOCKER-1 (D-03) | All wired P1 accessor fields are non-nil on `b.skill` after production bootstrap | unit/integration | `go test ./internal/daemon/... -run TestP1AccessorBootstrap -count=1` | ❌ Wave 0 |
| BLOCKER-1 (D-03a) | P1 handlers against production wiring return non-degraded envelopes | integration E2E | `go test ./internal/skill/semantic/... -run TestP1ProductionWiring -count=1` | ❌ Wave 0 |
| BLOCKER-2 (D-02) | `SetExtractorRun` wired; `assembleFreshness` returns non-empty `runID` | covered by D-03a E2E | — | ❌ Wave 0 |
| D-03b | Nil D-01a accessors emit documented `fallback_reason` in production E2E | integration | part of D-03a test | ❌ Wave 0 |
| Setter count | Log line emits `"setters", 14` | unit | `go test ./internal/daemon/... -run TestBootstrap` (add log assertion) | ✅ (extends bootstrap_test.go) |

### Per-Accessor Validation Assertions

After Phase 74 ships, the following assertions must hold in the runtime tests:

**Non-nil assertions (D-03 bootstrap test):**
- `WiredAccessorsForTest(skill).SymbolByName == true`
- `WiredAccessorsForTest(skill).ExtractorRun == true`
- `WiredAccessorsForTest(skill).ClusterMap == true`
- `WiredAccessorsForTest(skill).ClusterMember == true`
- `WiredAccessorsForTest(skill).ClusterPageRank == true`
- `WiredAccessorsForTest(skill).ImpactLookup == true`
- `WiredAccessorsForTest(skill).SymbolEdges == true` (FOLD)
- `WiredAccessorsForTest(skill).ClusterMembership == true` (FOLD)
- `WiredAccessorsForTest(skill).TypeChain == false` (DEFER)
- `WiredAccessorsForTest(skill).EdgeEvidence == false` (DEFER)

**Production-path E2E assertions (D-03a), using `buildP1E2EFixture` data:**
- `explain_symbol_deep` (wired accessors): `fallback_reason == ""`; `type_chain` field is
  empty array (TypeChain deferred, nil accessor triggers the handler's nil-guard), NOT an error.
- `find_related_symbols` (wired ClusterMembership): `fallback_reason == ""` or
  `fallback_reason == "cluster_boost_unavailable"` ONLY if ClusterMembership returns (0,0,nil).
  If wired, it must NOT return `cluster_boost_unavailable` for the fixture's seeded symbols.
- `get_cluster_map` (wired ClusterMap + ClusterPageRank): `fallback_reason == ""`; `total_clusters > 0`.
- `explain_cluster` (wired ClusterMember + ClusterPageRank): `fallback_reason == ""`; cluster echoed.
- `get_change_impact_graph` (wired ImpactLookup): `fallback_reason != "impact_lookup_unavailable"`;
  `nodes` non-empty for the seeded ServeHTTP→handle call chain.
- `validate_graph_edge` (EdgeEvidence deferred): `fallback_reason == "edge_not_found"` (nil accessor
  causes the handler to return this — D-03b contract).

**FreshnessV2 BLOCKER-2 assertion:**
- After `SetExtractorRun` is wired and a committed snapshot exists: `freshness.status != "unknown"`.

### Sampling Rate
- **Per task commit:** `go test ./internal/daemon/... ./internal/skill/semantic/... -count=1`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green + `go vet ./...` before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/daemon/p1_accessor_bootstrap_test.go` — runtime D-03 bootstrap test
- [ ] `internal/skill/semantic/p1_production_wiring_e2e_test.go` — D-03a production-path E2E
      (or extend `p1_e2e_external_test.go` with a new test function block)
- [ ] `internal/skill/semantic/export_p1_test.go` — extend with `WiredAccessorsForTest` export

---

## Security Domain

> `security_enforcement` is not set to `false` in config.json (absent = enabled).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | All SQL uses positional bound parameters (established pattern in existing `*Store` methods; Phase 74 adapters inherit this) |
| V6 Cryptography | no | — |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via `repoID` or `stableKey` | Tampering | All `*Store` queries use `?` positional parameters (verified in QuerySymbolByName, QueryClusterSummaries, etc.) |
| Node ID confusion (uint64 aliasing) | Tampering | JOIN on explicit column names; never assume node_id == symbol_id |

---

## Sources

### Primary (HIGH confidence)

- `internal/daemon/semantic_wiring.go` — setters block, existing P0 adapter shape, `integSemanticLookup` struct [VERIFIED: codebase inspection]
- `internal/skill/semantic/accessors.go:189-412` — all 10 P1 accessor interfaces [VERIFIED]
- `internal/skill/semantic/skill.go:150-237` — all 10 `Set*` methods [VERIFIED]
- `internal/semantic/store/effective_graph.go:595-960` — all 5 existing `*Store.Query*` methods [VERIFIED]
- `internal/semantic/store/migrations.go` — full Schema v6 DDL [VERIFIED: all migrations scanned]
- `internal/skill/semantic/p1_e2e_external_test.go` — `buildP1E2EFixture`, inline fakes, store-backed adapters [VERIFIED]
- `internal/daemon/integ_lookup_export.go` — `NewIntegSemanticLookupForTest` [VERIFIED]
- `internal/daemon/bootstrap_test.go` — existing bootstrap test pattern [VERIFIED]
- `.planning/v1.11-MILESTONE-AUDIT.md` — audit driving this phase [VERIFIED]
- `.planning/phases/74-close-gap-wire-p1-tool-accessors-in-production-daemon/74-CONTEXT.md` — all decisions [VERIFIED]

### Secondary (MEDIUM confidence)

- `internal/skill/semantic/tools_explain_symbol_test.go:86-138` — fake TypeChain + SymbolEdges [VERIFIED]
- `internal/skill/semantic/tools_validate_edge_test.go:80-113` — fake EdgeEvidence [VERIFIED]
- `internal/skill/semantic/tools_find_related_test.go:130-168` — fake ClusterMembership [VERIFIED]
- `internal/lint/nokernel2semantic/`, `internal/lint/noduckdb/` — vet gate rules [VERIFIED]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all referenced files read directly
- D-01a schema audit: HIGH — full migrations.go + store method scan; no type-chain/evidence columns found anywhere
- Fold vs Defer recommendation: HIGH — SymbolEdges and ClusterMembership fold is grounded in confirmed schema; TypeChain and EdgeEvidence defer is grounded in confirmed absence of required columns
- Architecture / patterns: HIGH — existing P0 adapter shape is the exact template
- Test scaffolding: MEDIUM — WiredAccessorsForTest export shape is proposed, not yet verified against the actual private field names (planner must cross-check `skill.go` field names)

**Research date:** 2026-05-29
**Valid until:** 2026-06-29 (stable, schema-backed)

---

## Fold vs Defer Recommendation

**SPLIT-ON-METHOD (8 FOLD, 2 DEFER).**

Wire all 10 P1 accessors except `TypeChainAccessor` and `EdgeEvidenceAccessor` in Phase 74.
`SymbolEdgesAccessor` (all three methods) and `ClusterMembershipAccessor` fold because
`semantic_edges` carries `src_node_id`, `dst_node_id`, `edge_kind` with existing indexes, and
`semantic_cluster_members` carries `(repo_id, graph_version, cluster_id, node_id)` — both
require only a two-hop SQL query pattern that mirrors the existing `QueryNodeIDByStableKey`
helper. The SQL is M/S complexity and requires NO new migrations, NO new `*Store` methods
beyond the existing translation helpers.

`TypeChainAccessor` and `EdgeEvidenceAccessor` are DEFERred to Phase 75 because: type-chain
rows require tier/evidence-kind metadata that has NO backing column in any migration (Schema
v6 has no `tree_sitter_kind`, `tier`, or `evidence_kind` anywhere in `semantic_edges` or
`semantic_symbols`); edge evidence rows require `TreeSitterKind`, `ASTAttested`, and per-row
`Range` fields that similarly have no schema backing. Implementing partial stubs would produce
permanently-degraded `evidence_status: "partial"` envelopes — not the production-quality
wiring this phase targets.

After Phase 74 ships: the setters block log becomes `"setters", 14`, `explain_symbol_deep`
returns a live symbol with empty `type_chain` (nil TypeChain accessor → handler nil-guard
skips it), and `validate_graph_edge` returns `fallback_reason: "edge_not_found"` (nil
EdgeEvidence accessor → documented degradation). All four cluster/impact tools and
`find_related_symbols` return real data. The v1.11 audit's BLOCKER-1 and BLOCKER-2 are closed
for the 8 wired accessors; Phase 75 closes the remaining two.
