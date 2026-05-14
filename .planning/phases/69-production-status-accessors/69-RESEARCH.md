# Phase 69: Production Status Accessors - Research

**Researched:** 2026-05-14
**Domain:** Go (semantic graph status surface) — bleve corpus metadata + DuckDB `semantic_clusters` read accessor
**Confidence:** HIGH (all claims `[VERIFIED:` against the live codebase via Read/Grep)

## Summary

Phase 69 closes the two Phase 64 W1 placeholders in `internal/daemon/semantic_wiring.go` (`semSchedulerAdapter.ClusterStatus` at line 456-462 and the retrieval-status absence at 441,450). The cluster side is straightforward: a new `*Store.ClusterStatusForGraphVersion` accessor reads the existing `semantic_clusters` rows that Phase 62 P04 already persists; `state` is derived from a comparison against `CurrentGraphVersion`. The retrieval side is more nuanced because **today the production buildFn does NOT call `engine.UpsertBatch` on commit** — bleve is populated only by `Recoverer.rebuildBlocking` at workspace activation. So `corpus_version` and `indexed_files` must be written from inside the Recoverer (matching the existing `last_indexed_snapshot_id` SetMeta call), not from the IndexRunner buildFn. `last_compact_at` is written from the compactor, which today has no bleve dep — we must inject one.

The "building" state for clusters has no in-flight signal in the current codebase: `RunClusterDetection` is a one-shot function with no daemon-resident scheduler. The honest choice for Phase 69 is to omit `"building"` (only emit `current` / `stale` / `unknown`) until a future phase adds a cluster scheduler.

**Primary recommendation:** Land four real production-layer pieces — (1) new `*Store.ClusterStatusForGraphVersion` accessor that reads `semantic_clusters` + COUNT(*) of `semantic_cluster_members`; (2) extend `Recoverer.rebuildBlocking` to also `SetMeta("corpus_version", gv)` and `SetMeta("indexed_files", n)` next to the existing `last_indexed_snapshot_id` write; (3) inject the per-workspace bleve `*Engine` into the compactor's deps and call `SetMeta("last_compact_at", unix_ms)` after `c.lastBaseID = snap.ID` on successful runCompaction; (4) wire the SchedulerAccessor and a new RetrievalStatusAccessor surface into `tools_status.go` and extend the Phase 64 P07 fixture to seed 2-3 `cluster_summary` + `cluster_members` rows.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|---|---|---|---|
| `ClusterStatusForGraphVersion` accessor | `internal/semantic/store` | — | `*Store` owns all DuckDB read-paths; D-09 forbids reaching past *Store from kernel/skill tiers `[VERIFIED: CONVENTIONS.md cited in CONTEXT.md L27]` |
| `cluster_status` envelope assembly | `internal/skill/semantic` (tools_status.go) | daemon adapter (semantic_wiring.go) | tools_status.go composes the response from accessor results; daemon adapter wires the concrete *Store → SchedulerAccessor seam `[VERIFIED: tools_status.go:181-196]` |
| `retrieval_status` envelope assembly | `internal/skill/semantic` (tools_status.go) | daemon adapter (semRetrievalAdapter) | mirrors cluster_status seam shape per D2 `[VERIFIED: 69-CONTEXT.md D2]` |
| `corpus_version` / `indexed_files` write | `internal/semantic/retrieval` (recovery.go) | — | Recoverer is the only producer of bleve corpus state today; production buildFn does NOT touch bleve `[VERIFIED: grep for UpsertBatch in internal/daemon/ returns only Recoverer + test helper]` |
| `last_compact_at` write | `internal/semantic/compact` (compactor.go) | daemon wiring (compact_wiring.go) | Compactor is the single writer for compaction outcomes; daemon must inject bleve handle as new dep `[VERIFIED: compactor.go:330-336]` |
| `indexed_symbols` read | `internal/semantic/retrieval` (bleve.go) | — | `bleve.Index.DocCount()` is the source-of-truth (one doc per symbol per MapSymbolToDoc) `[VERIFIED: bleve.go:46-47, corpus.go]` |

## Phase Requirements

| ID | Description | Research Support |
|---|---|---|
| STATUS-01 | `semSchedulerAdapter.ClusterStatus` returns real `{state, reason, computed_at, member_count}` from Phase 62 cluster engine | §New Accessor: `*Store.ClusterStatusForGraphVersion`; §State Derivation Logic |
| STATUS-02 | Retrieval status accessors at lines 441,450 return real `{corpus_version, indexed_files, indexed_symbols, last_compact_at}` from bleve | §Bleve Meta Writers (3 single-writers); §New RetrievalStatusAccessor surface |
| STATUS-03 | `get_semantic_graph_status` integration test asserts non-placeholder values on populated workspace | §Phase 64 P07 Fixture Extension |

## Standard Stack

This is a closure phase — no new dependencies; all changes are within the existing stack.

### Core
| Library | Version | Purpose | Why Standard |
|---|---|---|---|
| `github.com/blevesearch/bleve/v2` | v2.4.4 (pinned Phase 64 P01) | Internal metadata via `Index.SetInternal`/`GetInternal`; doc count via `Index.DocCount()` | Already in use as retrieval corpus owner `[VERIFIED: bleve.go:9]` |
| `modernc.org/sqlite` / DuckDB driver | (existing) | `semantic_clusters` / `semantic_cluster_members` read in new accessor | Phase 62 schema already populated `[VERIFIED: migrations.go:265-289]` |

### Supporting (existing)
| Library | Purpose | Use Case |
|---|---|---|
| `golang.org/x/sync` (existing) | Not required for read-path; concurrent test pattern uses stdlib `sync.WaitGroup` + `atomic.Bool` | Race-clean read-path test `[VERIFIED: effective_graph_test.go:252-292]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|---|---|---|
| `SetMeta("corpus_version", gv)` | New DuckDB column | bleve already owns corpus state (D3); adding SQL column requires migration |
| Compactor-injected bleve handle | Post-flush hook callback | Hook would skip the success-path guarantee — direct dep is simpler and matches existing `c.deps.Store` pattern |
| In-flight signal on cluster scheduler | Absence-of-rows + recent-write heuristic | **No cluster scheduler exists** `[VERIFIED: no daemon-resident caller of RunClusterDetection found except test code]` — omit `"building"` state for now |

## Architecture Patterns

### System Architecture Diagram

```
                    get_semantic_graph_status (MCP tool)
                              │
                              ▼
            ┌───────────────────────────────────────┐
            │ skill/semantic/tools_status.go        │
            │ handleGetSemanticGraphStatus          │
            │  - StoreAccessor (existing)           │
            │  - SchedulerAccessor.ClusterStatus ◄──┼──── NEW: returns real {State,
            │  - RetrievalAccessor (NEW methods)  ◄─┼──── NEW: corpus_version, indexed_*,
            │  - QueueAccessor / LiveAccessor       │     last_compact_at}
            └───────────────────────────────────────┘
                              │ (read-only, D-09)
                              ▼
   ┌──────────────────────────────────────────────────┐
   │ daemon/semantic_wiring.go adapters               │
   │  - semSchedulerAdapter.ClusterStatus ────────────┼──► *Store.ClusterStatusForGraphVersion
   │  - semRetrievalAdapter.RetrievalStatus (NEW) ────┼──► Engine.GetMeta + Index.DocCount
   └──────────────────────────────────────────────────┘
                              │                                  │
                              ▼                                  ▼
   ┌──────────────────────────────┐    ┌────────────────────────────────────────┐
   │ internal/semantic/store      │    │ internal/semantic/retrieval/bleve.go   │
   │  ClusterStatusForGraphVersion│    │  GetMeta / DocCount                    │
   │   SELECT computed_at, COUNT  │    │   (writers: Recoverer + Compactor)     │
   │   FROM semantic_clusters     │    └────────────────────────────────────────┘
   │   JOIN semantic_cluster_members              ▲                  ▲
   └──────────────────────────────┘               │ writes           │ writes
              ▲                                    │                  │
              │ writes (Phase 62 P04)              │                  │
              │                                    │                  │
   ┌──────────────────────────────┐    ┌──────────────────────┐  ┌────────────────────┐
   │ semantic/cluster/persist.go  │    │ retrieval/recovery.go│  │ semantic/compact   │
   │   RunClusterDetection        │    │  rebuildBlocking:    │  │  /compactor.go     │
   │  (NO daemon caller today)    │    │   SetMeta(corpus_v)  │  │  runCompaction:    │
   └──────────────────────────────┘    │   SetMeta(idx_files) │  │   SetMeta(last_c_at)│
                                       └──────────────────────┘  └────────────────────┘
```

### Recommended File Layout (deltas, not new tree)

```
internal/semantic/store/
├── effective_graph.go      # NEW method: ClusterStatusForGraphVersion (added beside existing read accessors)
└── effective_graph_test.go # NEW tests including LockFree_RaceSafe variant

internal/semantic/retrieval/
├── bleve.go                # No edits (existing GetMeta/SetMeta + new constants for meta keys in recovery.go)
├── recovery.go             # MODIFIED: rebuildBlocking writes corpus_version + indexed_files alongside last_indexed_snapshot_id

internal/semantic/compact/
├── accessors.go            # MODIFIED: add BleveMetaWriter interface for compactor dep
├── compactor.go            # MODIFIED: runCompaction calls deps.BleveMeta.SetMeta on success path

internal/skill/semantic/
├── accessors.go            # MODIFIED: extend SchedulerAccessor and RetrievalAccessor surfaces (additive)
├── envelope.go             # MODIFIED: extend ClusterStatus struct; add RetrievalStatus struct; extend StatusResult
├── tools_status.go         # MODIFIED: compose retrieval_status block; remove "phase-62-clustering-no-status-accessor" fallback path
├── tools_status_test.go    # MODIFIED: new RED→GREEN cases for non-placeholder values
└── integration_test.go     # MODIFIED: extend TestE2E_IndexThenContext_SymbolCount → seed clusters + assert envelope

internal/daemon/
├── semantic_wiring.go      # MODIFIED: replace 3 placeholder blocks at 408/441/450; pass bleve into compactor wiring
└── compact_wiring.go       # MODIFIED: thread bleve engine into Compactor deps
```

### Pattern 1: Read-Path Accessor on `*Store` (Phase 62 P02 doctrine)

**What:** New accessors on `*Store` are pure DuckDB reads with no overlay-tx and no mutex acquisition. The compile-time guard is that they appear on `*Store` directly (not on `OverlayTx`).
**When to use:** Every read accessor consumed by the SchedulerAccessor or StoreAccessor seams.
**Example:** Follow the shape of `CountStaleScoreRows` (`internal/semantic/store/effective_graph.go`) and its race-clean test `TestCountStaleScoreRows_LockFree_RaceSafe` (effective_graph_test.go:242-293) `[VERIFIED]`.

```go
// Source: internal/semantic/store/effective_graph.go (analog pattern)
func (s *Store) ClusterStatusForGraphVersion(ctx context.Context, repoID string, graphVersion uint64) (ClusterStatusRow, error) {
    // Pure read: SELECT MAX(computed_at), COUNT(*) etc.
    // No s.mu lock, no BeginOverlayTx — D-09 compile-time invariant.
}
```

### Pattern 2: Bleve Single-Writer Meta (Recoverer doctrine)

**What:** Every bleve meta key has exactly one writer. `last_indexed_snapshot_id` is owned by `Recoverer.rebuildBlocking` (`recovery.go:258`). `corpus_version` and `indexed_files` should join that same single-writer site — they share a transactional boundary (the rebuild commit is the moment all three move from "stale" to "current").
**When to use:** Any bleve corpus-state metadata.
**Example:**

```go
// Source: internal/semantic/retrieval/recovery.go:258 (existing) — add after the SetMeta block
if err := r.engine.SetMeta(metaKeyLastIndexed, []byte(strconv.FormatUint(snapshotID, 10))); err != nil { ... }
// NEW (Phase 69):
if err := r.engine.SetMeta("corpus_version", []byte(strconv.FormatUint(graphVersion, 10))); err != nil { ... }
if err := r.engine.SetMeta("indexed_files", []byte(strconv.FormatInt(distinctFileCount, 10))); err != nil { ... }
```

### Pattern 3: Compactor Dep Injection

**What:** The Compactor takes deps via the struct in `internal/semantic/compact/accessors.go`. Adding a `BleveMeta` writer follows the same shape as `Store`, `OverlayOps`, `Gate`, `Metrics`.
**When to use:** Any new collaborator the compactor needs.
**Anti-pattern:** Calling `bundle.engines[repoID]` directly from inside `runCompaction` — couples the compactor to the daemon's bundle layout. Inject a narrow interface instead.

### Anti-Patterns to Avoid

- **Putting `corpus_version` write in the IndexRunner buildFn:** today's buildFn does NOT call `engine.UpsertBatch` on commit `[VERIFIED: only Recoverer.rebuildBlocking and the test helper primeBleveCorpus call UpsertBatch]`. Writing `corpus_version` there would lie — the bleve corpus reflects whatever the last Recoverer rebuild saw, not what the IndexRunner just committed.
- **Adding a `member_count` column to `semantic_clusters`:** the schema already stores it implicitly. `UpsertClusters` writes `MemberCount` into the `score` column (`overlay.go:835` — `float64(c.MemberCount)`). Use that, or COUNT(*) over `semantic_cluster_members` for stronger correctness. Schema change is unnecessary.
- **Emitting `"building"` state without a signal:** there is no daemon-resident cluster scheduler. Returning `"building"` based on "absence of rows + recent prior write" requires also writing a `last_cluster_attempt_at` somewhere — out of scope for STATUS-01. Omit the state.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---|---|---|---|
| Indexed-symbol count | Iterate bleve docs | `bleve.Index.DocCount() (uint64, error)` | One-call API; bleve maintains the counter natively `[VERIFIED: bleve v2 docs — already in dep tree]` |
| Distinct file count over bleve | Scan + dedupe in Go | Single `SetMeta("indexed_files", N)` written by Recoverer during the rebuild loop (which already iterates symbols and sees their file_id) | O(1) read vs O(N) scan per status call |
| Member count over `semantic_cluster_members` | Cache in code | `SELECT COUNT(*) FROM semantic_cluster_members WHERE repo_id=? AND graph_version=?` | DuckDB COUNT(*) is fast; matches D-09 read-only path |
| Cluster computed_at | Track separately | Read `MAX(computed_at)` from `semantic_clusters` rows | Already populated by `UpsertClusters` via `now()` `[VERIFIED: overlay.go:829]` |

## Runtime State Inventory

Not applicable — this phase is purely additive (new accessor, new meta keys, struct field extensions). No string renames, no migrations, no OS-registered state, no env vars.

| Category | Items Found | Action Required |
|---|---|---|
| Stored data | None — schema unchanged; new bleve internal meta keys (`corpus_version`, `indexed_files`, `last_compact_at`) are added but not removed/migrated | None |
| Live service config | None | None |
| OS-registered state | None | None |
| Secrets/env vars | None | None |
| Build artifacts | None | None |

## Common Pitfalls

### Pitfall 1: Race between IndexRunner commit and Recoverer rebuild

**What goes wrong:** IndexRunner commits a new snapshot, but bleve still reflects the prior snapshot (Recoverer hasn't re-run). `corpus_version` would lag `graph_version`.
**Why it happens:** Two single-writer paths (IndexRunner → DuckDB; Recoverer → bleve) are not transactionally joined.
**How to avoid:** Be explicit in the closed-enum reason: `"corpus_version-lag"` when `bleve.corpus_version < store.CurrentGraphVersion`. This is honest about the asynchronous boundary.
**Warning signs:** Test where IndexRunner commits but Recoverer hasn't been triggered — `corpus_version` will be stale or zero.

### Pitfall 2: `bleve.Index.DocCount()` returns uint64, not int64

**What goes wrong:** Casting back to `int64` for the envelope could overflow on a 64-bit-full corpus (theoretical).
**How to avoid:** Use `int64` in the envelope (per D2); explicit cast `int64(n)` with the understanding that bleve will never realistically exceed `math.MaxInt64`. Document the cast.
**Warning signs:** Lint flag on the bare cast.

### Pitfall 3: COUNT(*) over `semantic_cluster_members` under concurrent writes

**What goes wrong:** A `RunClusterDetection` tx in flight while the read accessor runs could see an intermediate state.
**Why it happens:** DuckDB's MVCC means committed reads see a consistent snapshot, but the test must explicitly verify this.
**How to avoid:** The accessor is a single SELECT; DuckDB's tx isolation handles it. The race-clean test (`LockFree_RaceSafe` pattern from `effective_graph_test.go:242`) must include a concurrent `RunClusterDetection` writer to prove the contract.

### Pitfall 4: `omitempty` on `int64` zero values

**What goes wrong:** `ComputedAt int64 json:"computed_at,omitempty"` (per D4) — Go's `omitempty` drops zero values. For unset clusters this is the correct behavior, but tests asserting "field present, value > 0" must distinguish.
**How to avoid:** Tests assert non-zero rather than presence-only.

### Pitfall 5: Compactor's existing tests break when deps grow

**What goes wrong:** Compactor tests in `compact/compactor_test.go` construct `Compactor` with a `Deps` literal. Adding a new field (`BleveMeta`) without making it optional breaks every existing test.
**How to avoid:** Make `BleveMeta` field nil-safe (`if c.deps.BleveMeta != nil { _ = c.deps.BleveMeta.SetMeta(...) }`) — same pattern as the existing `c.deps.Metrics != nil` guards (compactor.go:223, 233).
**Warning signs:** Adding a new dep that gets dereferenced unconditionally.

## Code Examples

### New `*Store.ClusterStatusForGraphVersion` (target shape)

```go
// Source: derived from internal/semantic/store/effective_graph.go pattern
// (companion to CountStaleScoreRows / LatestCommittedSnapshot)

type ClusterStatusRow struct {
    GraphVersion uint64
    ComputedAt   int64 // unix ms; 0 when no rows
    MemberCount  int   // SUM(cluster_members.count) for the graph_version
    ClusterCount int   // COUNT(DISTINCT cluster_id)
}

// ClusterStatusForGraphVersion returns the persisted cluster state for the
// (repoID, graphVersion) pair, or a zero-value row when no rows exist. Pure
// read path — no overlay tx, no per-workspace mutex (D-09).
func (s *Store) ClusterStatusForGraphVersion(ctx context.Context, repoID string, graphVersion uint64) (ClusterStatusRow, error) {
    var row ClusterStatusRow
    err := s.db.QueryRowContext(ctx, `
        SELECT
          COALESCE(MAX(EXTRACT(EPOCH FROM c.computed_at) * 1000), 0)::BIGINT AS computed_at_ms,
          COUNT(DISTINCT c.cluster_id) AS cluster_count,
          (SELECT COUNT(*) FROM semantic_cluster_members m
            WHERE m.repo_id = ? AND m.graph_version = ?) AS member_count
        FROM semantic_clusters c
        WHERE c.repo_id = ? AND c.graph_version = ?
    `, repoID, graphVersion, repoID, graphVersion).Scan(&row.ComputedAt, &row.ClusterCount, &row.MemberCount)
    if err != nil {
        return ClusterStatusRow{}, fmt.Errorf("ClusterStatusForGraphVersion: %w", err)
    }
    row.GraphVersion = graphVersion
    return row, nil
}
```

### State Derivation (in `semSchedulerAdapter.ClusterStatus`)

```go
// Replaces the current placeholder at semantic_wiring.go:456-462
func (a *semSchedulerAdapter) ClusterStatus(repoID string) semantic.ClusterStatus {
    if a == nil || a.bundle == nil || a.bundle.store == nil {
        return semantic.ClusterStatus{State: "unknown", Reason: "no-store"}
    }
    ctx := context.Background() // status calls are short read-only DB queries
    gv, err := a.bundle.store.CurrentGraphVersion(ctx, repoID)
    if err != nil || gv == 0 {
        return semantic.ClusterStatus{State: "unknown", Reason: "no-graph-version"}
    }
    row, err := a.bundle.store.ClusterStatusForGraphVersion(ctx, repoID, gv)
    if err != nil {
        return semantic.ClusterStatus{State: "unknown", Reason: "accessor-error"}
    }
    if row.ClusterCount == 0 {
        // Check a recent older gv to detect "stale" (rows exist for gv-N).
        // For simplicity in v1: look only at current gv → if zero, "unknown".
        return semantic.ClusterStatus{State: "unknown", Reason: "no-cluster-rows"}
    }
    return semantic.ClusterStatus{
        State:       "current",
        ComputedAt:  row.ComputedAt,
        MemberCount: row.MemberCount,
    }
}
```

### Bleve Meta Write in Recoverer (extension of existing single-writer site)

```go
// Source: internal/semantic/retrieval/recovery.go:258 (existing block; add after)
if err := r.engine.SetMeta(metaKeyLastIndexed, []byte(strconv.FormatUint(snapshotID, 10))); err != nil {
    return fmt.Errorf("SetMeta(last_indexed=%d): %w", snapshotID, err)
}
// NEW (Phase 69):
gv, gvErr := r.store.CurrentGraphVersion(ctx, ws.Hash()) // requires extending StoreReader seam
if gvErr == nil && gv > 0 {
    if err := r.engine.SetMeta(metaKeyCorpusVersion, []byte(strconv.FormatUint(gv, 10))); err != nil {
        r.logger.Warn("retrieval.rebuild: SetMeta(corpus_version) failed", "err", err)
    }
}
// distinctFileCount accumulated during the rebuild loop (track during iterate):
if err := r.engine.SetMeta(metaKeyIndexedFiles, []byte(strconv.FormatInt(distinctFileCount, 10))); err != nil {
    r.logger.Warn("retrieval.rebuild: SetMeta(indexed_files) failed", "err", err)
}
```

### Bleve Meta Write in Compactor (new dep)

```go
// Source: internal/semantic/compact/compactor.go:332 (after c.lastBaseID = snap.ID, before maybeVacuum)
c.lastBaseID = snap.ID

// Phase 69: stamp last_compact_at on the workspace's bleve engine, if wired.
if c.deps.BleveMeta != nil {
    _ = c.deps.BleveMeta.SetMeta("last_compact_at",
        []byte(strconv.FormatInt(c.now().UnixMilli(), 10)))
    // Non-fatal: a SetMeta failure does not undo the compaction.
}

c.maybeVacuum(ctx)
```

### Phase 64 P07 Fixture Extension (integration test)

```go
// Source: extend internal/skill/semantic/integration_test.go around line 445
// (TestE2E_IndexThenContext_SymbolCount). Add a sibling test that seeds
// cluster_summary rows AFTER the index commit, then calls get_semantic_graph_status.

func TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval(t *testing.T) {
    h := newE2EHarness(t, "review", makeFixtureFacts(15))

    // 1. Index full → commits snapshot + Recoverer.Probe populates bleve.
    res := h.skill.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
    require.False(t, res.IsError)
    ir := decodeIndexResult(t, res)
    require.NotZero(t, ir.SnapshotID)

    // 2. Prime bleve corpus (Phase 64 P07 fixture path) so corpus_version is set.
    require.NoError(t, primeBleveCorpus(context.Background(), h.store, h.engine, ir.SnapshotID))

    // 3. Seed 2-3 cluster_summary + cluster_members rows for the current gv.
    repoID := h.ws.Hash()
    gv, _ := h.store.CurrentGraphVersion(context.Background(), repoID)
    tx, err := h.store.BeginOverlayTx(context.Background(), repoID)
    require.NoError(t, err)
    require.NoError(t, tx.UpsertClusters(context.Background(), "weak_components", gv,
        []semanticstore.ClusterSummary{{ID: 1, MemberCount: 3}, {ID: 2, MemberCount: 2}}))
    require.NoError(t, tx.UpsertClusterMembers(context.Background(), "weak_components", gv,
        []semanticstore.ClusterMemberRow{
            {ClusterID: 1, NodeID: 100}, {ClusterID: 1, NodeID: 101}, {ClusterID: 1, NodeID: 102},
            {ClusterID: 2, NodeID: 200}, {ClusterID: 2, NodeID: 201},
        }))
    require.NoError(t, tx.Commit())

    // 4. Replace scheduler/retrieval adapters with the production-shape ones.
    //    (Phase 64 P07 uses e2eSchedAcc which hard-codes the unknown placeholder
    //     — Phase 69 must inject the real *Store-backed adapter.)

    // 5. Call get_semantic_graph_status and assert non-placeholder values.
    sres := h.skill.handleGetSemanticGraphStatus(context.Background(), GetSemanticGraphStatusArgs{})
    sr := decodeStatusResult(t, sres)
    require.Equal(t, "current", sr.ClusterStatus.State)
    require.Equal(t, 5, sr.ClusterStatus.MemberCount)
    require.Greater(t, sr.ClusterStatus.ComputedAt, int64(0))
    require.Equal(t, gv, sr.RetrievalStatus.CorpusVersion)
    require.Greater(t, sr.RetrievalStatus.IndexedSymbols, int64(0))
}
```

### Race-Clean Read-Path Test (D-09 invariant)

```go
// Source: mirror internal/semantic/store/effective_graph_test.go:242-293

func TestClusterStatusForGraphVersion_LockFree_RaceSafe(t *testing.T) {
    if testing.Short() { t.Skip("race-safety stress; skipped under -short") }
    s, ctx, _ := openStoreForOverlayTest(t)
    repoID := "r-race"
    seedClusterRows(t, ctx, s, repoID, 1 /*gv*/, 5 /*cluster count*/)

    // Background overlay tx writer to create lock contention.
    var stop atomic.Bool
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        for !stop.Load() {
            tx, err := s.BeginOverlayTx(ctx, repoID)
            if err != nil { return }
            _ = tx.Commit()
        }
    }()

    // 100 concurrent reader probes — must NOT deadlock.
    var readers sync.WaitGroup
    for i := 0; i < 100; i++ {
        readers.Add(1)
        go func() {
            defer readers.Done()
            done := make(chan error, 1)
            go func() {
                _, e := s.ClusterStatusForGraphVersion(ctx, repoID, 1)
                done <- e
            }()
            select {
            case e := <-done:
                if e != nil { t.Errorf("ClusterStatusForGraphVersion under contention: %v", e) }
            case <-time.After(5 * time.Second):
                t.Errorf("ClusterStatusForGraphVersion deadlocked (>5s under contention)")
            }
        }()
    }
    readers.Wait()
    stop.Store(true)
    wg.Wait()
}
```

## Open Research Question Resolutions

### Q1. Cluster "building" state signal

**Resolution:** **No signal exists.** `RunClusterDetection` is a one-shot pure function `[VERIFIED: persist.go:58]`; there is no daemon-resident cluster scheduler analogous to `RankScheduler.IsQuiescent` (which is graph-rank-only `[VERIFIED: scheduler.go:413-420]`). Grep for callers of `RunClusterDetection` returns ONLY `cluster/persist_test.go` and `cluster/doc.go` — **the daemon never invokes it in production today**. Recommendation: emit only `current` / `stale` / `unknown` in Phase 69. Add `"building"` in a future phase that introduces a cluster scheduler.

### Q2. `indexed_files` derivation cost

**Resolution:** Option (b) — a `bleve_files` meta key updated per Recoverer rebuild. Rationale: bleve has no built-in distinct-field aggregation; computing it on every status call would require iterating the corpus. Since the only writer to bleve is `Recoverer.rebuildBlocking`, and it already iterates all symbols, accumulating a `map[uint64]struct{}` of distinct file_ids during the iterate-loop and writing the count once at the end is O(0) extra cost on the hot path. **Recommendation: option (b).**

### Q3. Compactor write site for `last_compact_at`

**Resolution:** `internal/semantic/compact/compactor.go:332` — immediately after `c.lastBaseID = snap.ID` and before `c.maybeVacuum(ctx)`. This is the success-path commit point. The compactor today has no bleve handle — inject one via `Deps.BleveMeta` (new field) wired through `daemon/compact_wiring.go`. `[VERIFIED: compactor.go:332-336]`

### Q4. IndexRunner write site for `corpus_version`

**Resolution:** **NOT in the IndexRunner.** Critical finding: the production buildFn in `semantic_wiring.go:1374-1460` does NOT call `engine.UpsertBatch`. The only paths that populate bleve are `Recoverer.rebuildBlocking` (recovery.go:217) and the test helper `primeBleveCorpus` (integ_lookup_e2e_helpers.go:188). Therefore `corpus_version` must be written from inside `rebuildBlocking`, right after the existing `SetMeta(metaKeyLastIndexed, ...)` call on line 258. The Recoverer is triggered by daemon activation (semantic_wiring.go:329-335) and would need a new trigger after each IndexRunner commit (out of scope; flagged as carryover).

**Carryover risk:** Until the IndexRunner triggers a Recoverer rebuild on commit, `corpus_version` will lag `graph_version` after each index. The honest envelope reason is `"corpus_version-lag"` when `bleve.corpus_version < store.CurrentGraphVersion`.

### Q5. Phase 64 P07 fixture extension

**Resolution:** Extend `internal/skill/semantic/integration_test.go` near line 445. The existing harness (`newE2EHarness`) constructs a real `*Store` + bleve `*Engine` in tempdirs. Seed `cluster_summary` + `cluster_members` rows via the existing `BeginOverlayTx` → `UpsertClusters` → `UpsertClusterMembers` → `Commit` sequence (these are the same surfaces used by `RunClusterDetection`). For retrieval-side: call `primeBleveCorpus` (already present in `integ_lookup_e2e_helpers.go:188`) to ensure bleve has docs and meta. **The existing harness must also have its scheduler/retrieval adapters swapped from the hard-coded placeholder e2e ones to production-shape adapters** — see `e2eSchedAcc.ClusterStatus` at integration_test.go:245-247 which today returns the unknown placeholder.

### Q6. Closed-enum `RetrievalStatus.Reason` values

**Resolution:** Recommend the following closed set:

| Reason | When emitted |
|---|---|
| `""` (empty / omitted) | All four fields present and consistent — `State` is effectively "current" |
| `"corpus_version-uninitialized"` | bleve `GetMeta("corpus_version")` returns nil (Recoverer never ran successfully) |
| `"corpus_version-lag"` | `bleve.corpus_version < store.CurrentGraphVersion` (IndexRunner committed; Recoverer has not yet caught up — known asynchronous boundary) |
| `"compactor-never-ran"` | bleve `GetMeta("last_compact_at")` returns nil |
| `"bleve-unavailable"` | `b.engines[repoRoot]` is nil (e.g., bleve init failed during workspace activation — semantic_wiring.go:315-319) |

The set is small, deterministic, and matches the SPEC §23.3 closed-enum doctrine. Each value names a single underlying condition.

### Q7. Race-clean read-path test pattern

**Resolution:** The canonical example is `TestCountStaleScoreRows_LockFree_RaceSafe` (`internal/semantic/store/effective_graph_test.go:242-293`) `[VERIFIED]`. Pattern: a background goroutine spins `BeginOverlayTx → Commit` to create contention; 100 concurrent reader goroutines call the new accessor; each reader wrapped in a `select` with a 5-second timeout. The accessor must not deadlock and must not return an error. Phase 69's new `ClusterStatusForGraphVersion` test mirrors this shape verbatim.

### Q8. Phase 62 cluster persistence shape

**Resolution:** `[VERIFIED: internal/semantic/store/migrations.go:265-289 + overlay.go:777-841]`

**`semantic_clusters` table:**
```
PRIMARY KEY (repo_id, graph_version, cluster_id)
Columns: repo_id, snapshot_id, graph_version, cluster_id, algorithm, label, summary, score, status, computed_at
```
- `computed_at TIMESTAMP NOT NULL` — **already present**, written via `now()` in `UpsertClusters` (overlay.go:829)
- `score DOUBLE` — **abused by Phase 62 P04** to carry per-cluster `MemberCount`: `float64(c.MemberCount)` (overlay.go:835)

**`semantic_cluster_members` table:**
```
PRIMARY KEY (repo_id, graph_version, cluster_id, node_id)
Columns: repo_id, graph_version, cluster_id, node_id, weight, role
```
- No `member_count` column exists — derive via `COUNT(*) WHERE repo_id=? AND graph_version=?`

**Recommendation for `MemberCount` (D4 field):** Use `SELECT COUNT(*) FROM semantic_cluster_members` for stronger correctness; do NOT trust `semantic_clusters.score` (its overloading is an internal Phase 62 quirk, not a documented contract).

### Q9. `semantic_wiring.go` placeholder interfaces

**Resolution:** `[VERIFIED]`

- **Line 408 (anchor comment):** `semSchedulerAdapter` doc comment — informational; remove the "W1 placeholders today — Phase 65/67 wires real sources" text.
- **Line 441-443 (anchor comment block):** `ScoreStatus` deliberate-companion comment — Phase 69 closes only the ClusterStatus side, so rewrite the comment to drop the "deliberate companion" framing.
- **Line 450-455 (the actual placeholder method):** `ClusterStatus` returning `{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}`. Implements `semantic.SchedulerAccessor.ClusterStatus` (accessors.go:52). Signature: `func (a *semSchedulerAdapter) ClusterStatus(repoID string) semantic.ClusterStatus`. Phase 69 replaces the body to call `a.bundle.store.ClusterStatusForGraphVersion` and translate to the envelope shape.

**For retrieval status (no placeholder method yet — must be added):** Extend `semantic.RetrievalAccessor` (accessors.go:96-111) with a new `RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus` method. Implementation in `semRetrievalAdapter` (semantic_wiring.go:580-625) reads `engine.GetMeta` for `corpus_version` / `indexed_files` / `last_compact_at`, and `engine.idx.DocCount()` for `indexed_symbols` (requires exposing `DocCount` on the `*Engine` wrapper — single new method on bleve.go).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|---|---|---|---|
| W1 placeholder returning hardcoded `{unknown, phase-62-clustering-no-status-accessor}` | Real `*Store`-backed accessor reading committed cluster rows | Phase 69 | Closes STATUS-01 |
| Retrieval-status absent from envelope | Nested `retrieval_status` block in `StatusResult` (D2) | Phase 69 | Closes STATUS-02 |
| Test relies on hardcoded `e2eSchedAcc` returning placeholder | Test harness swaps in production-shape adapter; asserts non-placeholder | Phase 69 | Closes STATUS-03 |

**Deprecated/outdated (post-Phase-69):**
- The "phase-62-clustering-no-status-accessor" reason string — should disappear from grep output.
- The W1-placeholder doc comments at semantic_wiring.go:407-409, 433-443, 450-455 — must all be removed (success criterion #5).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A1 | `semantic_clusters.score` column is safe to ignore in favor of `COUNT(*)` over `cluster_members` | Q8 | None — COUNT(*) is independently correct |
| A2 | `DocCount()` returns `(uint64, error)` per bleve v2 | Don't Hand-Roll table | LOW — easy to fix at compile time if signature differs |
| A3 | Compactor's `c.now` field is the canonical timestamp source for `last_compact_at` | Code Examples | LOW — `c.now` exists `[VERIFIED: compactor.go:221]` |
| A4 | Recoverer's `r.store` already exposes `CurrentGraphVersion` via the StoreReader seam | Code Examples | MEDIUM — actual `StoreReader` interface in recovery.go may not expose `CurrentGraphVersion`; planner must verify and extend the local seam if needed |

(A4 in particular should be verified during planning — if `StoreReader` is narrower than `*Store`, the new `corpus_version` write needs the seam extended.)

## Open Questions

1. **Should the Recoverer be re-triggered after each IndexRunner commit?**
   - What we know: today the Recoverer runs only at workspace activation (semantic_wiring.go:329). The buildFn does not touch bleve.
   - What's unclear: whether Phase 69 should also add a post-commit Recoverer trigger to keep `corpus_version` current, or defer that to a follow-up.
   - Recommendation: **defer** to a follow-up phase. Phase 69 documents the lag honestly via `Reason="corpus_version-lag"` and ships the read-path. Adding a post-commit Recoverer trigger touches the buildFn's success path and risks regressing Phase 64/65 invariants.

2. **Should `RetrievalStatus.Reason` carry multiple conditions?**
   - What we know: a single Reason field maps cleanly to single conditions.
   - What's unclear: if BOTH `corpus_version-uninitialized` AND `compactor-never-ran` are true, which wins?
   - Recommendation: deterministic priority order, emit the highest-priority Reason: `bleve-unavailable` > `corpus_version-uninitialized` > `corpus_version-lag` > `compactor-never-ran`. Document in tools_status.go.

3. **`ClusterStatus.MemberCount` — total members across all clusters, or list per cluster?**
   - What we know: D4 says "total members across all clusters for this (repo, graph_version)".
   - What's unclear: agents might want per-cluster cardinality for diagnostics. Out of scope per CONTEXT.md "deferred" section (per-projection cluster status).
   - Recommendation: ship D4 as written; add per-cluster surface in a future phase.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|---|---|---|---|---|
| Go toolchain | Build | ✓ | go1.22+ (CI matrix) | — |
| bleve v2.4.4 | retrieval engine | ✓ | v2.4.4 | — |
| DuckDB driver | semantic store | ✓ | (existing pin) | — |

No new external dependencies. All deltas are within the existing dep tree.

## Validation Architecture

### Test Framework
| Property | Value |
|---|---|
| Framework | Go `testing` stdlib + `testify/require` + `testify/assert` |
| Config file | `go.mod` / per-package; no central config |
| Quick run command | `go test ./internal/semantic/store/ ./internal/semantic/retrieval/ ./internal/skill/semantic/ -run "ClusterStatus\|RetrievalStatus\|GraphStatus" -count=1` |
| Full suite command | `go test ./... -count=1 -race` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|---|---|---|---|---|
| STATUS-01 | `*Store.ClusterStatusForGraphVersion` returns `{computed_at, member_count, cluster_count}` from persisted rows | unit | `go test ./internal/semantic/store/ -run TestClusterStatusForGraphVersion -count=1 -race` | ❌ Wave 0 (new test file or extend `effective_graph_test.go`) |
| STATUS-01 | `*Store.ClusterStatusForGraphVersion` is lock-free under concurrent overlay-tx writes (D-09 invariant) | unit (race-stress) | `go test ./internal/semantic/store/ -run TestClusterStatusForGraphVersion_LockFree_RaceSafe -count=1 -race -timeout 30s` | ❌ Wave 0 |
| STATUS-01 | `semSchedulerAdapter.ClusterStatus` returns `state="current"` when rows exist; `"unknown"` when not | unit | `go test ./internal/daemon/ -run TestSemSchedulerAdapter_ClusterStatus -count=1` | ❌ Wave 0 |
| STATUS-02 | `Recoverer.rebuildBlocking` writes `corpus_version` + `indexed_files` meta keys | unit | `go test ./internal/semantic/retrieval/ -run TestRebuild_WritesCorpusVersionAndFileCount -count=1` | ❌ Wave 0 |
| STATUS-02 | Compactor writes `last_compact_at` on success-path | unit | `go test ./internal/semantic/compact/ -run TestRunCompaction_WritesLastCompactAt -count=1` | ❌ Wave 0 |
| STATUS-02 | `semRetrievalAdapter.RetrievalStatus` returns `{corpus_version, indexed_files, indexed_symbols, last_compact_at}` populated | unit | `go test ./internal/daemon/ -run TestSemRetrievalAdapter_RetrievalStatus -count=1` | ❌ Wave 0 |
| STATUS-02 | Closed-enum `Reason` values emit deterministically for each degraded condition | table-driven unit | `go test ./internal/skill/semantic/ -run TestRetrievalStatus_ReasonEnum -count=1` | ❌ Wave 0 |
| STATUS-03 | E2E: populated workspace returns non-placeholder cluster_status + retrieval_status | integration | `go test ./internal/skill/semantic/ -run TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval -count=1 -race` | ❌ Wave 0 (extend `integration_test.go`) |
| STATUS-03 | Existing `TestE2E_IndexThenContext_SymbolCount` continues to pass after envelope additions | regression | `go test ./internal/skill/semantic/ -run TestE2E_IndexThenContext_SymbolCount -count=1 -race` | ✓ exists (integration_test.go:445) |
| Invariant | D-09: no `Begin/Commit/Abort/Write` on read path | static grep | `grep -E "\.(BeginSnapshot\|BeginOverlayTx\|CommitSnapshot\|WriteSnapshotFacts)" internal/skill/semantic/tools_status.go` returns no matches | grep audit |
| Invariant | Placeholder comments removed at semantic_wiring.go:408,441,450 | static grep | `grep -n "phase-62-clustering-no-status-accessor\|W1 placeholder" internal/daemon/semantic_wiring.go` returns no matches | grep audit |
| Project rule | `go vet ./...` + `gofmt -w .` clean (CLAUDE.md) | static | `go vet ./... && test -z "$(gofmt -l .)"` | CI |

### Sampling Rate
- **Per task commit:** `go test ./internal/semantic/store/ ./internal/semantic/retrieval/ ./internal/semantic/compact/ ./internal/skill/semantic/ ./internal/daemon/ -count=1 -race`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green + the two grep-audit invariants clean before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/semantic/store/effective_graph_test.go` — extend with `TestClusterStatusForGraphVersion_*` table (basic + lock-free race-safe variant)
- [ ] `internal/semantic/retrieval/recovery_test.go` — extend with `TestRebuild_WritesCorpusVersionAndFileCount`
- [ ] `internal/semantic/compact/compactor_test.go` — extend with `TestRunCompaction_WritesLastCompactAt` (requires fake BleveMeta in test deps)
- [ ] `internal/daemon/semantic_wiring_test.go` — extend with `TestSemSchedulerAdapter_ClusterStatus` + `TestSemRetrievalAdapter_RetrievalStatus`
- [ ] `internal/skill/semantic/tools_status_test.go` — extend with `TestRetrievalStatus_ReasonEnum` (table-driven closed-enum coverage)
- [ ] `internal/skill/semantic/integration_test.go` — extend with `TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval`
- [ ] Helper: `seedClusterRows(t, ctx, s, repoID, gv, n)` analog to `seedScoreRow` (already exists in overlay_test.go area)
- [ ] Helper: `fakeBleveMeta` for compactor tests (in-memory map satisfying the new `BleveMeta` interface)

*(No new framework install needed — Go stdlib `testing` + existing `testify` already in dep tree.)*

## Security Domain

Per `security_enforcement` evaluation: Phase 69 is a read-path closure of an existing tool surface. No new authentication, session management, or input-validation paths. ASVS categories:

| ASVS Category | Applies | Standard Control |
|---|---|---|
| V2 Authentication | no | MCP session auth is upstream, unchanged |
| V3 Session Management | no | unchanged |
| V4 Access Control | no | `mode_check` (existing) gates the tool — unchanged by Phase 69 |
| V5 Input Validation | minimal | tool has no caller-supplied parameters (the request body is empty per SPEC §23.3) |
| V6 Cryptography | no | unchanged |

### Known Threat Patterns for this surface

| Pattern | STRIDE | Standard Mitigation |
|---|---|---|
| Info disclosure of internal cluster topology via member_count | Information Disclosure | Counts only, no node identifiers — mitigated by D4 design |
| Race-condition during status read (Phase 62 P04 in flight) | Tampering / Denial of Service | Pure read on `*Store` with DuckDB MVCC isolation; race-clean test mandated (STATUS-01 row 2) |

No new threat surfaces introduced.

## Sources

### Primary (HIGH confidence — all VERIFIED via Read/Grep in this session)
- `internal/daemon/semantic_wiring.go:407-462` — placeholder block to replace
- `internal/daemon/semantic_wiring.go:580-625` — `semRetrievalAdapter` extension point
- `internal/daemon/semantic_wiring.go:1374-1460` — production buildFn (confirms NO bleve UpsertBatch)
- `internal/skill/semantic/envelope.go:62-112` — `ClusterStatus`, `StatusResult` shapes (D2/D4 targets)
- `internal/skill/semantic/tools_status.go:117-247` — handler shape, mode-tier check, freshness derivation
- `internal/skill/semantic/accessors.go:35-111` — `SchedulerAccessor`, `RetrievalAccessor` interfaces to extend
- `internal/semantic/cluster/persist.go:1-141` — `RunClusterDetection` orchestrator (confirms no daemon caller)
- `internal/semantic/store/overlay.go:777-902` — `ClusterSummary`, `UpsertClusters`, `UpsertClusterMembers`, `DeleteClustersForGraphVersion`
- `internal/semantic/store/migrations.go:265-289` — `semantic_clusters` + `semantic_cluster_members` schemas
- `internal/semantic/retrieval/bleve.go:178-202` — `GetMeta`/`SetMeta` API (single-writer pattern)
- `internal/semantic/retrieval/recovery.go:217-260` — `rebuildBlocking` + existing `last_indexed_snapshot_id` SetMeta site
- `internal/semantic/compact/compactor.go:204-339` — `runCompaction` success-path commit point
- `internal/semantic/graph/scheduler.go:413-420` — `IsQuiescent` is rank-only, NOT cluster (confirms no in-flight cluster signal)
- `internal/semantic/store/effective_graph_test.go:242-293` — `TestCountStaleScoreRows_LockFree_RaceSafe` (canonical race-clean test pattern)
- `internal/skill/semantic/integration_test.go:1-470` — E2E harness shape + `TestE2E_IndexThenContext_SymbolCount`
- `internal/daemon/integ_lookup_e2e_helpers.go:185-206` — `primeBleveCorpus` reference helper

### Secondary
- `.planning/phases/69-production-status-accessors/69-CONTEXT.md` — locked D1-D4 decisions
- `.planning/phases/64-new-mcp-tools/64-07-PLAN.md`, `64-08-PLAN.md`, `64-08-SUMMARY.md` — fixture provenance
- `.planning/REQUIREMENTS.md:30-32` — STATUS-01/02/03 wording
- `.planning/ROADMAP.md:155,180` — phase scoping

### Tertiary (training-knowledge only — flag for validation if relied upon)
- bleve v2 `Index.DocCount() (uint64, error)` signature — common knowledge; verify at compile time when writing the accessor.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all touched packages already in tree
- Architecture (read path): HIGH — pattern mirrors `CountStaleScoreRows` 1:1
- Architecture (write path): MEDIUM — `corpus_version` from Recoverer (not IndexRunner) is correct but introduces an honest lag boundary; this is the planner's primary design call
- Pitfalls: HIGH — all derived from inspecting actual code, not training data
- E2E fixture extension: HIGH — existing harness has the building blocks; only swap-in of production adapter is new
- Closed-enum reason set: MEDIUM — proposed enum is reasonable but planner should review against SPEC §23.3 style guide before locking

**Research date:** 2026-05-14
**Valid until:** 2026-06-13 (30 days — code surfaces touched are stable; Phase 65 already landed)
