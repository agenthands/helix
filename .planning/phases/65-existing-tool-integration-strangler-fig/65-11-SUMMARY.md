---
phase: 65-existing-tool-integration-strangler-fig
plan: 11
subsystem: semantic-strangler-fig
tags: [tdd, gap-closure, blast-radius, integ-03, integ-04, integ-05, wr-01, in-04, wr-2, wr-03, wr-05]
requires:
  - "*Store.LatestCommittedSnapshot (Phase 64 P64-02)"
  - "*Store.QueryEffectiveAdjacency (Phase 64 P64-02)"
  - "*Store.CurrentGraphVersion / OverlayHasPendingRows (Phase 63)"
  - "newE2EIntegLookup harness (65-09)"
  - "BL-2 fixture continuity contract (65-09 SUMMARY)"
  - "integSemanticLookup adapter shape (65-03)"
  - "analyzeBlastRadiusViaLookup two-pass orchestrator (65-06)"
provides:
  - "*Store.QuerySymbolByLocation(ctx, repoID, path, line, col) → (stable_key, ok, err)"
  - "*Store.QueryNodeIDByStableKey(ctx, repoID, stable_key) → (symbol_id, ok, err)"
  - "*Store.QueryStableKeyByNodeID(ctx, repoID, symbol_id) → (stable_key, ok, err)"
  - "integSemanticLookup.SymbolID real implementation (was 65-03 stub)"
  - "integSemanticLookup.ExpandFrom real implementation (BFS over QueryEffectiveAdjacency to depth=2)"
  - "bfsExpand + confidenceFromWeight helpers (Phase 62 closed-ladder mapping)"
  - "*semanticBundle.lastErrReason field + SetLastErrorReason setter (WR-2 / IN-04)"
  - "applyValidationVerdicts sawConfirmed/sawRefuted accumulator (WR-05)"
  - "formatBlastRadiusEnvelopeFromImpacts graphVersion stamping (WR-03)"
  - "analyzeBlastRadiusViaLookup 5-return form (impacts, source, reason, graphVersion, err)"
  - "semanticStoreProbe carries *semanticBundle so SELECT 1 failure stamps closed-enum reason"
  - "WR-01 Status mutex pattern: snapshot queue/live/lastErrReason in single critical section"
affects:
  - internal/semantic/store
  - internal/daemon
  - internal/kernel/symbols
tech-stack:
  added: []
  patterns:
    - inner-scope tiebreak via ORDER BY (end_byte - start_byte) ASC for overlap resolution
    - "BFS frontier expansion with visit-once semantics + Phase 62 closed-ladder confidence mapping"
    - "single-acquisition mutex pattern: snapshot fields under lock, call methods that take their own locks AFTER releasing"
    - "closed-enum LastError stamping via per-bundle setter; build-failure paths stamp; success paths clear"
    - "accumulator-based verdict semantics (sawConfirmed || sawRefuted) instead of break-on-first-match"
    - "envelope graph_version threading via single lookup.Status() at orchestrator boundary"
key-files:
  created: []
  modified:
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/effective_graph_test.go
    - internal/daemon/semantic_wiring.go
    - internal/daemon/semantic_wiring_test.go
    - internal/daemon/integ_lookup_e2e_test.go
    - internal/daemon/daemon.go
    - internal/kernel/symbols/blast_radius_strangler.go
    - internal/kernel/symbols/blast_radius_strangler_test.go
    - internal/kernel/symbols/tools.go
decisions:
  - "Phase 62 confidence ladder: weight buckets {≥1.0 → 0.95, ≥0.8 → 0.80, ≥0.6 → 0.70, else → 0.45}; the 1.00 value reserved for Pass-2 LSP-confirmed (set by applyValidationVerdicts in 65-06's orchestrator, not ExpandFrom)"
  - "ExpandFrom seed itself is NOT emitted as an Impact — only neighbors at distance ≥ 1; orchestrator treats the seed cursor specially"
  - "WR-01 single-lock-acquisition: snapshot bundle.queue/bundle.live/bundle.lastErrReason under bundle.mu in one critical section, then call DepthAll/LastFlushAt OUTSIDE the lock to avoid deadlock against those methods' own internal locks"
  - "WR-2 stamp-and-clear pattern: build-failure paths stamp FallbackReasonIndexError, successful CommitSnapshot clears via SetLastErrorReason(\"\"); Probe SELECT 1 mirrors the same shape"
  - "Edge weight 1.0 → confidence 0.95 (snapshot+overlay merged signal; Pass-2 LSP would flip to 1.00); weight 0.5 fillers → 0.45 (heuristic) — both fixture canonical-edge weights map onto closed-ladder values"
  - "Visit-once BFS semantics: a node first reached at depth d is NOT re-emitted at depth d+1; bounds the result set on graphs with high in-degree"
  - "WR-5 forward-reference cleanup honored: NO lspProbeFn parameter introduced in this plan; the comment on the orchestrator documents the deferral to 65-12 Task 2"
  - "Rule 3 blocking deviation: 65-09 fixture EdgeKind=\"CALLS\" did not match the QueryEffectiveAdjacency projection filter (\"call_graph\"); fixture re-aligned to \"call_graph\" so ExpandFrom's BFS sees the committed edges. Cross-plan impact: 65-12 Task 2 already references the fixture's canonical edge keys (confirmed_edge / refuted_edge); the EdgeKind alignment does not change the EdgeID hash inputs in any way that breaks BL-A canonical-key continuity (the local edgeIDForTripleLocal hash receives the new kind verbatim)."
metrics:
  start: 2026-05-08T20:50:00Z
  duration_min: 25
  tasks: 2
  files_changed: 9
  commits: 4
---

# Phase 65 Plan 11: Real SymbolID + ExpandFrom + sibling REVIEW fixes Summary

Replace the `integSemanticLookup.SymbolID` and `ExpandFrom` 65-03 stubs with
real implementations that drive `analyze_blast_radius` semantic-source path
end-to-end against a populated DuckDB snapshot. Fold in four sibling REVIEW
findings on the same files (WR-01 mutex, IN-04 LastErrorReason, WR-05
accumulator semantics, WR-03 graph_version stamping).

This plan turns the two `PENDING 65-11` Skipf gates from 65-09 into real
GREEN assertions and unlocks the kernel-side two-pass orchestrator
(`analyzeBlastRadiusViaLookup` in `blast_radius_strangler.go`) end-to-end —
the orchestrator was correct in isolation but unreachable in production
because `SymbolID` errored before it ran.

## Task 1 — *Store.QuerySymbolByLocation + Node/StableKey resolvers

Three new public readers at `internal/semantic/store/effective_graph.go`:

```go
func (s *Store) QuerySymbolByLocation(
    ctx context.Context, repoID, path string, line, col uint32,
) (string, bool, error)

func (s *Store) QueryNodeIDByStableKey(
    ctx context.Context, repoID, stableKey string,
) (uint64, bool, error)

func (s *Store) QueryStableKeyByNodeID(
    ctx context.Context, repoID string, nodeID uint64,
) (string, bool, error)
```

`QuerySymbolByLocation` joins `semantic_files` and `semantic_symbols` at the
latest committed snapshot for `repoID`, filters on
`(start_line < ?) OR (start_line = ? AND start_col <= ?)` paired with the
end-side counterpart, and orders by `(end_byte - start_byte) ASC LIMIT 1` so
the smallest enclosing scope wins on overlap. `line` / `col` are 1-based
matching LSP's external surface; `start_byte` / `end_byte` are derived in the
test fixture as `line*1000 + col` for deterministic inner-scope ordering.

`QueryNodeIDByStableKey` and `QueryStableKeyByNodeID` are simple inverse
lookups on the `(snapshot_id, stable_key)` and `(snapshot_id, symbol_id)`
indexes respectively. Both honor the latest-committed-snapshot scoping and
return `(zero, false, nil)` on miss or no-snapshot (consistent with
`QueryRankedFiles` / `QuerySymbolPath` from 65-10).

Eight RED→GREEN tests added at `effective_graph_test.go`:

- `TestStore_QuerySymbolByLocation_HitOnExactStart`
- `TestStore_QuerySymbolByLocation_HitInsideRange`
- `TestStore_QuerySymbolByLocation_MissOutsideRange` (before-range + after-range)
- `TestStore_QuerySymbolByLocation_PathMismatch`
- `TestStore_QuerySymbolByLocation_PrefersInnerScope` — outer (5-30) + inner
  (10-15); query at (12, 5) returns the inner symbol's stable_key
- `TestStore_QuerySymbolByLocation_NoCommittedSnapshot`
- `TestStore_QueryNodeIDByStableKey_HitAndMiss` — including no-snapshot path
- `TestStore_QueryStableKeyByNodeID_HitAndMiss`

A new helper `seedSnapshotSymbolWithRange` accepts explicit
`(line, col, line, col, stable_key)` coordinates plus a derived
`start_byte`/`end_byte` so the inner-scope assertion is deterministic.

## Task 2 — Real SymbolID + ExpandFrom + WR-01/IN-04/WR-2/WR-05/WR-03

### `integSemanticLookup.SymbolID`

Real implementation backed by `*Store.QuerySymbolByLocation`. Returns:

- `(stableKey, nil)` on hit at the latest committed snapshot.
- `("", integ.ErrNoSnapshot)` when `(path, line, col)` is outside every
  symbol's range OR no snapshot has been committed yet.
- `("", integ.ErrIndexErrored)` when `Available()==false`.
- `("", wrapped store error)` on a real SQL error.

The integ-boundary `SymbolID` IS the `stable_key` (Phase 59 EXTRACT-02
canonical identity); kernel callers thread it back through `ExpandFrom`
without ever round-tripping it through the store-side `uint64 graph.NodeID`
surface.

### `integSemanticLookup.ExpandFrom`

BFS over `*Store.QueryEffectiveAdjacency` for the `"call_graph"` projection,
to depth=2 (the `blastRadiusExpansionDepth` cap from
`blast_radius_strangler.go`). Algorithm:

1. Resolve seed `stable_key → symbol_id` via `QueryNodeIDByStableKey`. Miss
   here returns `integ.ErrNoSnapshot` (orchestrator falls through to LSP).
2. Pull the `(repoID, "call_graph")` effective adjacency once for the whole
   BFS (snapshot edges ⊕ live overlay edges, status='live' tombstoned overlay
   rows excluded).
3. Frontier-based traversal: visit-once semantics; per-frontier-node
   neighbors sorted ASC by NodeID for deterministic per-test ordering. Each
   visited target yields one `integ.Impact` carrying:
   - `SymbolID` = target's `stable_key` (via `QueryStableKeyByNodeID`)
   - `EdgeKind` = `"calls"`
   - `Confidence` from `confidenceFromWeight(weight)` per the Phase 62
     closed-ladder mapping
   - `Evidence.Edges` = `[{From, To, Kind, Confidence}]`
   - `Evidence.Ranks` = `[weight]`
4. The seed itself is NOT emitted as an Impact — only neighbors at distance
   ≥ 1 are. The orchestrator treats the seed cursor specially.

### Confidence ladder (`confidenceFromWeight`)

```
weight ≥ 1.00 → 0.95  (snapshot+overlay merged; Pass-2 LSP would flip to 1.00)
weight ≥ 0.80 → 0.80  (tree-sitter + local resolution)
weight ≥ 0.60 → 0.70  (tree-sitter only)
weight <  0.60 → 0.45  (heuristic / weak signal)
```

The `1.00` value is reserved for Pass-2 LSP-confirmed (set by
`applyValidationVerdicts` in the kernel orchestrator); ExpandFrom never emits
`1.00` itself because Pass 1 alone cannot LSP-confirm an edge.

### WR-01 Status mutex fix

The Status method now snapshots `bundle.queue`, `bundle.live`, and
`bundle.lastErrReason` under `bundle.mu` in a single critical section, then
calls `DepthAll()` / `LastFlushAt()` OUTSIDE the lock. Those methods take
their own internal locks and could deadlock against `bundle.mu` if invoked
while it was held. The pre-WR-01 form released `bundle.mu` between reads,
which raced with concurrent `SetLastErrorReason` / engine eviction.

### IN-04 / WR-2 LastErrorReason wiring

A new `lastErrReason integ.FallbackReason` field on `*semanticBundle`, plus
a `SetLastErrorReason(integ.FallbackReason)` setter (nil-safe). Read from
Status; written from EVERY enumerated build/live/overlay-flush error path:

| Site | What |
|---|---|
| `daemon.go:291` (semanticstore.Open ErrUnsupported warn) | NO STAMP — bundle does not yet exist (init-time, before `newSemanticBundle`); operator-visible via existing log line |
| `daemon.go:295` (opening semantic store fail-fast) | NO STAMP — INIT fail-fast; bundle never constructed; daemon refuses to start |
| `daemon.go:529` (skill init partial-fail warn) | NO STAMP — degraded skills, NOT semantic-bundle-relevant |
| `daemon.go:866` (Probe `p.s == nil` ErrUnsupported) | NO STAMP — semantic disabled; `p.bundle` is nil; nil-safe setter is a no-op |
| `daemon.go:870` (Probe `SELECT 1` SQL failure) | **STAMP** `FallbackReasonIndexError` via `p.bundle.SetLastErrorReason` |
| `daemon.go:872` (Probe success) | **CLEAR** via `p.bundle.SetLastErrorReason("")` |
| `semantic_wiring.go: BeginSnapshot fail` | **STAMP** `FallbackReasonIndexError` |
| `semantic_wiring.go: WriteSnapshotFacts fail` | **STAMP** `FallbackReasonIndexError` |
| `semantic_wiring.go: CommitSnapshot fail` | **STAMP** `FallbackReasonIndexError` |
| `semantic_wiring.go: CommitSnapshot success` | **CLEAR** via `b.SetLastErrorReason("")` |
| `daemon.go:1020-1024` (admin listener bind fail) | NO STAMP — admin-port lifecycle, NOT semantic-bundle-relevant |

The probe constructor at the `health.RegisterTools` call site now passes
`semanticStoreProbe{s: semanticStore, bundle: sBndl}`. `sBndl` is nil when
semantic is disabled — the bundle setter is nil-safe.

WR-2 mandated regression tests (both pass):

- `TestSemanticBundle_BuildFailure_StampsLastErrReason` — three subtests:
  - **Path A unit:** SetLastErrorReason writes the field observably; nil
    receiver is a no-op (no panic).
  - **Path A race:** 200 concurrent Set + 200 concurrent reads under
    `bundle.mu` — survives `-race`.
  - **Path B end-to-end:** stamp via SetLastErrorReason, drive
    `lookup.Status(ctx, ws)` against a real DuckDB store, assert
    `Status.LastErrorReason == string(integ.FallbackReasonIndexError)`.
- `TestGetHealthSemanticStoreStatus_StampsLastErrReason` — drives
  `health.ComputeSemanticIndexBlock` via `daemonSemIndexAccessor` against a
  bundle with `lastErrReason` stamped; asserts
  `semIdx.LastError == "index_error"` (the closed-enum value, NOT empty).

### WR-05 applyValidationVerdicts accumulator semantics

Replaced break-on-first-match with `sawConfirmed` / `sawRefuted` accumulators
evaluated AFTER walking every evidence edge:

```
sawRefuted=true  → Confidence=0.20, Refuted=true  (regardless of sawConfirmed)
sawConfirmed=true && !sawRefuted → Confidence=1.00 (Refuted unchanged)
neither → impact untouched
```

The doctrine: a single refutation taints the impact. The persisted graph
said the edge exists; the LSP says it doesn't. We trust the LSP probe over
the graph regardless of how many sibling edges were confirmed.

New tests pin the contract:

- `TestApplyValidationVerdicts_RefutationTaintsRegardlessOfConfirmation` —
  multi-edge impact with edge[0]=confirmed and edge[1]=refuted; asserts
  Confidence=0.20 + Refuted=true (would have failed on the pre-WR-05 form).
- `TestApplyValidationVerdicts_AllConfirmedFlipsToOne` — every-confirmed
  preserves the 1.00 path (regression guard for WR-05's accumulator
  rewrite).
- `TestApplyValidationVerdicts_NoMutation` (existing) — copy-before-mutate
  pinning Pitfall §6 still PASSES.

### WR-03 envelope graph_version stamping

`formatBlastRadiusEnvelopeFromImpacts` accepts a `graphVersion uint64`
parameter and stamps it on `integ.Envelope.GraphVersion`. `omitempty` fires
when the value is zero (e.g., when `lookup.Status` errors).

`analyzeBlastRadiusViaLookup` returns 5-tuple `(impacts, source, reason,
graphVersion, err)` — the orchestrator makes a single `lookup.Status(ctx,
ws)` call after Pass 1 succeeds and threads `status.GraphVersion` through
to the envelope. Status errors are non-fatal — the orchestrator returns
`graphVersion=0` and lets `MarshalEnvelope` omit the key.

WR-5: signature stays single-arg — NO `lspProbeFn` parameter (that lands
in 65-12 Task 2 which migrates the orchestrator to the LSP-validation arm).
The 1 grep hit on `lspProbeFn` in `blast_radius_strangler.go` is in the
docstring documenting the WR-5 deferral.

New test:

- `TestFormatBlastRadiusEnvelopeFromImpacts_StampsGraphVersion` — asserts
  envelope.graph_version == 4242 after a forced threading.

### Caller update (kernel/symbols/tools.go)

The `analyze_blast_radius` MCP handler:

```go
impacts, semSrc, semReason, semGraphVersion, expandErr := analyzeBlastRadiusViaLookup(ctx, lookup, ws, sym)
...
out, marshErr := formatBlastRadiusEnvelopeFromImpacts(impacts, semSrc, semReason, semGraphVersion)
```

The LSP-fallback `formatBlastRadiusEnvelope` keeps its 3-arg signature (no
graphVersion); the LSP path has no graph version to stamp.

### Skipf gates flipped to GREEN

Two of the remaining three 65-09 placeholder tests now have real assertions:

| Test | New behavior |
|---|---|
| `TestIntegSemanticLookup_E2E_SymbolID_RealStore` | Drives SymbolID for `confirmed_edge_from`; asserts `string(got) == pick.StableKey` (BL-A canonical-key contract via stable_key surface) |
| `TestIntegSemanticLookup_E2E_ExpandFrom_RealStore` | Drives ExpandFrom on the same seed at depth=2; asserts non-empty `[]Impact`, every entry has non-empty SymbolID + non-empty EdgeKind + Confidence in the Phase 62 closed-ladder set `{1.00, 0.95, 0.80, 0.70, 0.45}` |

The remaining `TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore`
stays Skipf'd for 65-12 (renamed to `*_PassthroughContract` per the 65-12
Task 2 plan).

The `_Status_RealStore`, `_RankFiles_RealStore`, `_RankFromSeeds_RealStore`
canaries continue to PASS.

## Skipf Gates Remaining (handled by 65-12)

| Test | File | Owner |
|---|---|---|
| `TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore` | `internal/daemon/integ_lookup_e2e_test.go` | 65-12 (rename to `*_PassthroughContract`) |
| `TestE2E_StranglerFig_ProductionAdapter_SourceSemantic` | `internal/skill/semantic/integration_test.go` | 65-12 Task 4 |
| `TestE2E_StranglerFig_ProductionAdapter_BlastRadiusConfidence` | `internal/skill/semantic/integration_test.go` | 65-12 Task 4 (smoke) + 65-12 Task 2 (strict, kernel-side) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] 65-09 fixture EdgeKind=\"CALLS\" did not match QueryEffectiveAdjacency projection filter**

- **Found during:** Task 2 implementation; the new
  `TestIntegSemanticLookup_E2E_ExpandFrom_RealStore` returned an empty
  frontier despite the fixture committing >= 5 edges.
- **Issue:** `newE2EIntegLookup` (65-09) writes overlay edges with
  `EdgeKind: \"CALLS\"`, but `*Store.QueryEffectiveAdjacency` filters on
  `edge_kind = ?` against the rank-scheduler projection key
  `\"call_graph\"`. The two strings don't match, so no edges came back from
  the BFS adjacency lookup. The plan's BL-2 fixture continuity contract
  forbids extending the helper, but the BL-2 fixture was wrong on this one
  axis — `\"CALLS\"` is not a projection key the production rank scheduler
  produces.
- **Fix:** Re-aligned the fixture's `confirmedEdgeKind` /
  `refutedEdgeKind` / filler-edge `EdgeKind` to `\"call_graph\"` so the
  BFS sees the committed edges. Cross-plan impact: 65-12 Task 2 references
  the fixture via the canonical edge keys (`confirmed_edge` /
  `refuted_edge`); the EdgeID hash inputs change accordingly (the local
  `edgeIDForTripleLocal` hash receives the new kind verbatim), but the BL-A
  canonical-key contract publishes the EdgeID through `syms[\"confirmed_edge\"]`
  / `syms[\"refuted_edge\"]` so cross-package callers continue to read the
  current value without hard-coding the kind string.
- **Files modified:** `internal/daemon/integ_lookup_e2e_test.go`
- **Commit:** `d6786928`

**2. [Rule 2 — Missing critical functionality] daemon.go SetLastErrorReason wiring on probe**

- **Found during:** Task 2 implementation; the plan listed daemon.go:866 as
  the WR-2 stamp site but `semanticStoreProbe` carried only `*semanticstore.Store`,
  not `*semanticBundle` — there was no way to reach the bundle's setter
  from the probe.
- **Issue:** The Probe-time `SELECT 1` failure path is the canonical
  \"semantic store is configured AND the connection is sick\" signal; without
  stamping the bundle, that signal couldn't reach get_health.
- **Fix:** Added `bundle *semanticBundle` to `semanticStoreProbe`; the
  probe constructor at `health.RegisterTools` passes the daemon's `sBndl`
  (nil-safe when semantic is disabled). On `SELECT 1` failure, stamps
  `FallbackReasonIndexError`; on success, clears via empty stamp. The
  TestGetHealthSemanticStoreStatus_StampsLastErrReason regression test
  confirms the closed-enum reason flows end-to-end through
  ComputeSemanticIndexBlock.
- **Files modified:** `internal/daemon/daemon.go`
- **Commit:** `d6786928`

No architectural changes were required (no Rule 4 escalations).

## Verification

```
$ go test ./internal/daemon/... ./internal/kernel/symbols/... ./internal/semantic/store/... -count=1
ok  	github.com/agenthands/helix/internal/daemon
ok  	github.com/agenthands/helix/internal/kernel/symbols
ok  	github.com/agenthands/helix/internal/semantic/store

$ go test ./internal/daemon/... ./internal/kernel/symbols/... ./internal/semantic/store/... -count=1 -race
ok  	github.com/agenthands/helix/internal/daemon	5.243s
ok  	github.com/agenthands/helix/internal/kernel/symbols	1.543s
ok  	github.com/agenthands/helix/internal/semantic/store	5.274s

$ go vet ./internal/daemon/... ./internal/kernel/symbols/... ./internal/semantic/store/...
# clean

$ go build ./...
# clean
```

Targeted runs:

```
$ go test ./internal/semantic/store/... -count=1 -run "TestStore_QuerySymbolByLocation|TestStore_QueryNodeIDByStableKey|TestStore_QueryStableKeyByNodeID" -v
--- PASS: TestStore_QuerySymbolByLocation_HitOnExactStart
--- PASS: TestStore_QuerySymbolByLocation_HitInsideRange
--- PASS: TestStore_QuerySymbolByLocation_MissOutsideRange
--- PASS: TestStore_QuerySymbolByLocation_PathMismatch
--- PASS: TestStore_QuerySymbolByLocation_PrefersInnerScope
--- PASS: TestStore_QuerySymbolByLocation_NoCommittedSnapshot
--- PASS: TestStore_QueryNodeIDByStableKey_HitAndMiss
--- PASS: TestStore_QueryStableKeyByNodeID_HitAndMiss

$ go test ./internal/daemon/... -count=1 -run "TestIntegSemanticLookup_E2E|TestSemanticBundle_BuildFailure|TestGetHealthSemanticStoreStatus|TestIntegSemanticLookup_ReadTierCanary" -v
--- PASS: TestIntegSemanticLookup_E2E_RankFiles_RealStore
--- PASS: TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore
--- PASS: TestIntegSemanticLookup_E2E_SymbolID_RealStore
--- PASS: TestIntegSemanticLookup_E2E_ExpandFrom_RealStore
--- SKIP: TestIntegSemanticLookup_E2E_ValidateCriticalEdges_RealStore (PENDING 65-12)
--- PASS: TestIntegSemanticLookup_E2E_Status_RealStore
--- PASS: TestIntegSemanticLookup_ReadTierCanary
--- PASS: TestSemanticBundle_BuildFailure_StampsLastErrReason
--- PASS: TestGetHealthSemanticStoreStatus_StampsLastErrReason

$ go test ./internal/kernel/symbols/... -count=1 -run "TestApplyValidationVerdicts|TestFormatBlastRadiusEnvelopeFromImpacts|TestAnalyzeBlastRadius" -v
--- PASS: TestAnalyzeBlastRadius_SemanticTwoPass
--- PASS: TestAnalyzeBlastRadius_CfgDisabled_TreeSitter
--- PASS: TestAnalyzeBlastRadius_LookupUnavailable_Fallback
--- PASS: TestAnalyzeBlastRadius_Pass1Error_FallsBackToLSP
--- PASS: TestAnalyzeBlastRadius_Pass2Error_KeepsPass1
--- PASS: TestApplyValidationVerdicts_RefutationTaintsRegardlessOfConfirmation
--- PASS: TestApplyValidationVerdicts_AllConfirmedFlipsToOne
--- PASS: TestFormatBlastRadiusEnvelopeFromImpacts_StampsGraphVersion
--- PASS: TestApplyValidationVerdicts_NoMutation
```

Done-criteria grep gates:

```
$ grep -F 'sawConfirmed' internal/kernel/symbols/blast_radius_strangler.go | wc -l
6     # >= 1

$ grep -F 'graphVersion uint64' internal/kernel/symbols/blast_radius_strangler.go | wc -l
2     # >= 1

$ grep -F 'lastErrReason' internal/daemon/semantic_wiring.go | wc -l
7     # >= 2

$ grep -F 'LastErrorReason:  ""' internal/daemon/semantic_wiring.go | wc -l
0     # IN-04 closed; the hard-coded empty string is gone

$ grep -F 'SetLastErrorReason' internal/daemon/daemon.go | wc -l
4     # >= 2 — the daemon.go:866 path (stamp on SELECT 1 fail + clear on success) plus the comment trail

$ grep -F 'func (s *Store) QuerySymbolByLocation(' internal/semantic/store/effective_graph.go | wc -l
1

$ grep -F 'func (s *Store) QueryNodeIDByStableKey(' internal/semantic/store/effective_graph.go | wc -l
1

$ grep -F 'func (s *Store) QueryStableKeyByNodeID(' internal/semantic/store/effective_graph.go | wc -l
1
```

The plan's `grep -F 'l.bundle.mu.Lock()' ≥ 2` criterion is met functionally
but not literally: the WR-01 doctrine wants the queue + live + lastErrReason
reads under one critical section. The implementation acquires the lock once
at line 1168 (Status) and snapshots all three values in one go before
calling DepthAll/LastFlushAt OUTSIDE the lock. The second mutex-touching
call on the bundle is `b.mu.Lock()` in `SetLastErrorReason` at line 157.
Both Lock() calls are on the same `*sync.Mutex` (`bundle.mu`); the literal
prefix differs because of the receiver name.

## Self-Check: PASSED

- `internal/semantic/store/effective_graph.go` — modified (FOUND;
  contains QuerySymbolByLocation + QueryNodeIDByStableKey + QueryStableKeyByNodeID)
- `internal/semantic/store/effective_graph_test.go` — modified (FOUND;
  8 new tests across the three readers, plus seedSnapshotSymbolWithRange helper)
- `internal/daemon/semantic_wiring.go` — modified (FOUND; real SymbolID +
  ExpandFrom + bfsExpand + confidenceFromWeight + Status WR-01/IN-04 fix +
  lastErrReason field + SetLastErrorReason setter + buildFn stamps)
- `internal/daemon/semantic_wiring_test.go` — modified (FOUND;
  TestSemanticBundle_BuildFailure_StampsLastErrReason + TestGetHealthSemanticStoreStatus_StampsLastErrReason)
- `internal/daemon/integ_lookup_e2e_test.go` — modified (FOUND; SymbolID +
  ExpandFrom Skipfs lifted; EdgeKind aligned to \"call_graph\")
- `internal/daemon/daemon.go` — modified (FOUND; semanticStoreProbe carries
  *semanticBundle; stamp+clear on SELECT 1)
- `internal/kernel/symbols/blast_radius_strangler.go` — modified (FOUND;
  WR-05 accumulator + WR-03 graph_version stamping + 5-return orchestrator)
- `internal/kernel/symbols/blast_radius_strangler_test.go` — modified
  (FOUND; 3 new tests + 3 existing-test 5-return updates)
- `internal/kernel/symbols/tools.go` — modified (FOUND; caller threads
  semGraphVersion to formatBlastRadiusEnvelopeFromImpacts)
- Commit `2931bacc` — FOUND (Task 1 RED)
- Commit `d2f017d4` — FOUND (Task 1 GREEN)
- Commit `3f6a5572` — FOUND (Task 2 RED)
- Commit `d6786928` — FOUND (Task 2 GREEN)
