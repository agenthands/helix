---
phase: 62-graph-engine-ranking-type-resolution
plan: 02
subsystem: graph-engine
tags: [graph_version, apply_repair, score_status, ranker, edges-merge, d14, t5-bounded-labels]
requires:
  - .planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md
  - .planning/phases/62-graph-engine-ranking-type-resolution/62-PATTERNS.md
  - internal/semantic/graph/repair.go (Task 1 RED tests)
  - internal/semantic/store/overlay.go (per-workspace mutex + epoch contract from Phase 60)
provides:
  - internal/semantic/graph.Engine (single bump site for graph_version)
  - internal/semantic/graph.GraphRepair / FileFactDiff / SymbolDiff / GraphEdge
  - internal/semantic/graph.ComputeGraphRepair (D-06 decision policy)
  - internal/semantic/graph.ScoreStatus closed enum + computeScoreStatus
  - internal/semantic/graph.Ranker / RankReader / engineRanker
  - internal/semantic/graph.RepairStore / RepairTx narrow seams
  - internal/semantic/graph.GraphVersionAdvance (P03 notify channel payload)
  - internal/semantic/graph.MetricsSink / NoopMetrics
  - internal/semantic/store.ScoreRow / EdgeRow value types
  - internal/semantic/store.OverlayTx.{UpsertGraphScores, UpsertEdgesWithMerge, BumpGraphVersion}
  - internal/semantic/store.Store.{CurrentGraphVersion, LockOverlayWorkspace}
  - internal/semantic/live/handler.RankApplier interface + Handler.SetRankApplier setter
  - internal/semantic/lspenrich.Edge extended with SrcNodeID/DstNodeID/Weight
  - internal/semantic/lspenrich.CascadeTx.UpsertEdgesWithMerge (B3 — closes AC10)
  - 4 new config keys (pagerank.repair_debounce_ms, pagerank.full_recompute_idle_ms, pagerank.full_recompute_threshold, types.comment_parsers_enabled)
  - 5 new bounded-label Prometheus metrics + drop-on-unknown helpers
affects:
  - internal/semantic/store/overlay.go (extended)
  - internal/semantic/store/overlay_test.go (extended)
  - internal/semantic/live/handler/handler.go (post-commit hook)
  - internal/semantic/lspenrich/cascade.go (5 LSP edge sites migrated)
  - internal/semantic/lspenrich/{cascade,cascade_integration,cascade_overlay_epoch,integration_acceptance,stress,worker}_test.go (test fakes extended)
  - internal/daemon/live_wiring.go (production CascadeTx adapter extended)
  - internal/config/defaults.go (4 new keys)
  - internal/config/loader_test.go (assertions for new keys)
  - internal/semantic/config.go (PageRankConfig + new TypesConfig)
  - internal/obs/metrics.go (5 new Vecs + helpers)
  - internal/obs/metrics_labels_test.go (carve-out + prime calls)
tech_stack_added:
  - none (stdlib only — context, sync, sort, log/slog; storage via existing duckdb-go)
patterns_used:
  - narrow tx seam (RepairStore + RepairTx mirroring lspenrich.CascadeTx)
  - per-workspace mutex re-use via store.LockOverlayWorkspace (T4 — no second mutex map)
  - body-only short-circuit via repair.IsEmpty before mutex acquisition (D-06)
  - closed-enum status validation at write boundary; computed at read boundary (D-07)
  - storage-side merge predicate via SQL DELETE-then-INSERT (D-14)
  - drop-on-unknown closed-enum allowlists for bounded-label metrics (T5)
  - empty-list no-op shortcut on every batch helper (mirroring MarkSymbolsDeleted)
  - 63-bit FNV-1a edge_id synthesis to work around duckdb-go uint64 high-bit rejection
key_files:
  created:
    - internal/semantic/graph/doc.go
    - internal/semantic/graph/repair.go
    - internal/semantic/graph/apply_repair.go
    - internal/semantic/graph/status.go
    - internal/semantic/graph/ranker.go
    - internal/semantic/graph/metrics.go
    - internal/semantic/graph/trace.go
    - internal/semantic/graph/repair_test.go
    - internal/semantic/graph/apply_repair_test.go
    - internal/semantic/graph/status_test.go
    - internal/semantic/graph/ranker_test.go
    - internal/semantic/lspenrich/cascade_merge_test.go
  modified:
    - internal/semantic/store/overlay.go (+~280 LOC for Score/EdgeRow + 3 helpers + Lock helper + edge-id synth)
    - internal/semantic/store/overlay_test.go (+~230 LOC for 9 new tests)
    - internal/semantic/live/handler/handler.go (RankApplier interface, SetRankApplier setter, post-commit hook)
    - internal/semantic/lspenrich/cascade.go (5 call-site migrations, Edge struct extended, CascadeTx surface extended)
    - internal/semantic/lspenrich/{cascade,cascade_integration,cascade_overlay_epoch,integration_acceptance,stress,worker}_test.go (test-fake extensions)
    - internal/daemon/live_wiring.go (production CascadeTx adapter)
    - internal/config/defaults.go (+4 keys)
    - internal/config/loader_test.go (+4 assertions)
    - internal/semantic/config.go (+3 PageRankConfig fields, +TypesConfig)
    - internal/obs/metrics.go (+5 Vecs, +5 helpers, +allowlist maps)
    - internal/obs/metrics_labels_test.go (+5 carve-out entries, +5 prime calls)
decisions:
  - W2 LOCKED — handler-tracked diff, NO OverlayTx.Diff method. The handler is the single owner of the tx-scoped FileFactDiff.
  - Phase 62 P02 ships ApplyRepair logic; the local diff is currently empty (Phase 60 P02 only writes file-row contentHash). Phase 60 P04 + Phase 62 P05 will populate the diff as symbol-level upserts land. The hook fires + short-circuits on repair.IsEmpty().
  - Synthesized edge_id as 63-bit FNV-1a over (repo_id, src, dst, kind) to give the (src, dst, kind) merge boundary natural-key idempotency atop the schema's (repo_id, edge_id) PK; 63-bit mask works around the duckdb-go driver's "uint64 values with high bit set" rejection.
  - score_name (existing schema column) is set from the Phase 62 "projection" identifier — no schema change required.
  - snapshot_id=0 in semantic_graph_scores rows: Phase 62 score rows are overlay-side state keyed on (repo_id, graph_version); base-snapshot lineage is re-established by Phase 63 compaction.
  - Cascade Edge struct extended with SrcNodeID/DstNodeID/Weight in-place rather than introducing a new type — all existing fakes still construct trivial Edge{} values that work.
  - UpsertEdges retained as a backward-compat alias on CascadeTx forwarding to UpsertEdgesWithMerge so out-of-tree consumers compiled against the pre-Phase-62 interface keep working.
metrics:
  duration: ~110 min
  completed: "2026-05-06T15:35:00Z"
  tasks_total: 7
  tasks_completed: 7
  files_created: 12
  files_modified: 14
  loc_engine: 698     # internal/semantic/graph/ production
  loc_tests:  1062    # internal/semantic/graph/*_test.go + cascade_merge_test.go + overlay extensions
---

# Phase 62 Plan 02: graph_version advance machinery + score_status read API + Ranker seam

**One-liner:** Wires `Engine.ApplyRepair` as the SINGLE `graph_version` bump site, ships the read-time `ScoreStatus` closed enum, lands the `Ranker` interface Phase 64 will consume, extends `store.OverlayTx` with `UpsertGraphScores`/`UpsertEdgesWithMerge`/`BumpGraphVersion` (storage-side D-14 merge predicate), wires the post-commit hook in the live handler, migrates Phase 61 cascade LSP edge writes through the merge boundary (closes AC10), and ships 4 new config keys + 5 bounded-label metrics with drop-on-unknown allowlists (T5 mitigation).

## Files Added

| Path | LOC | Provides |
| ---- | --- | -------- |
| `internal/semantic/graph/doc.go` | 49 | Package doc citing D-05/D-06/D-07/D-14/T4 invariants |
| `internal/semantic/graph/repair.go` | 152 | `NodeID`, `GraphEdge`, `SymbolDiff`, `FileFactDiff`, `GraphRepair`, `ComputeGraphRepair`, `nodeSet` helper |
| `internal/semantic/graph/apply_repair.go` | 256 | `Engine`, `RepairStore`, `RepairTx`, `EdgeUpsert`, `MetricsSink`, `GraphVersionAdvance`, `ApplyRepair` (single bump site) |
| `internal/semantic/graph/status.go` | 47 | `ScoreStatus` closed enum + `computeScoreStatus` read-time decision (D-07) |
| `internal/semantic/graph/ranker.go` | 122 | `Ranker`, `RankReader`, `RankRequest`/`Response`/`Status`, `RankedNode`, `ProjectionStatus`, `engineRanker` |
| `internal/semantic/graph/metrics.go` | 22 | `NoopMetrics` test stub for `MetricsSink` |
| `internal/semantic/graph/trace.go` | 17 | `traceApplyRepair` seam (P03 wires real otel) |
| `internal/semantic/graph/repair_test.go` | 110 | 7 tests pinning the SymbolDiff → GraphRepair contract |
| `internal/semantic/graph/apply_repair_test.go` | 256 | 5 tests including W1 production-mutex serialization (-race runnable) |
| `internal/semantic/graph/status_test.go` | 41 | TestComputeScoreStatus_ClosedEnum 7-case matrix |
| `internal/semantic/graph/ranker_test.go` | 67 | 2 tests pinning Phase 64 Ranker contract |
| `internal/semantic/lspenrich/cascade_merge_test.go` | 198 | TestCascadeLSP_UpgradesCommentEdgeInPlace (closes AC10 end-to-end against production *store.Store) |

## Files Modified

| Path | Change |
| ---- | ------ |
| `internal/semantic/store/overlay.go` | +ScoreRow / EdgeRow types; +UpsertGraphScores (closed-enum status guard); +UpsertEdgesWithMerge (D-14 SQL merge with refutation rule); +BumpGraphVersion (UPDATE ... RETURNING under tx); +Store.CurrentGraphVersion (read-only); +Store.LockOverlayWorkspace (re-uses overlayLockFor — T4 invariant); +63-bit FNV-1a edgeIDForTriple to work around duckdb-go uint64 rejection; +strings import. |
| `internal/semantic/store/overlay_test.go` | +9 tests: TestUpsertGraphScores_RoundTrip / EmptyNoOp / DoesNotBumpGraphVersion / RejectsInvalidStatus, TestBumpGraphVersion_ReturnsNewValue, TestCurrentGraphVersion_ZeroForUnknownRepo, TestUpsertEdgesWithMerge_LSPSkipsCommentInsert / LSPDeletesCommentBeforeInsert / LSPRefutesCommentAtDifferentDst (D-14 refutation). |
| `internal/semantic/live/handler/handler.go` | +`RankApplier` interface, +`Handler.rankApplier` field + `SetRankApplier` setter (CR-04 nil-safe); +post-commit hook in `updateChangedFileWithKind` calling `graph.ComputeGraphRepair(localDiff)` then `h.rankApplier.ApplyRepair` when non-empty; +`graphpkg` import alias. |
| `internal/semantic/lspenrich/cascade.go` | Edge struct extended with `SrcNodeID, DstNodeID, Weight` (zero defaults preserve backward compat); CascadeTx interface extended with `UpsertEdgesWithMerge`; UpsertEdges retained as forward-compat alias; 5 LSP edge-write call sites migrated (hover/callHierarchy/typeHierarchy/implementation/definition) to `tx.UpsertEdgesWithMerge`; WriteInvalidations stub re-annotated with Phase 62 P02 explanation. |
| `internal/semantic/lspenrich/cascade_test.go`, `cascade_integration_test.go`, `cascade_overlay_epoch_test.go`, `integration_acceptance_test.go`, `stress_test.go`, `worker_test.go` | Each CascadeTx-implementing test fake (`fakeOverlayTx`, `realCascadeTxAdapter`, `integrationTx`, `accTx`, `stressTx`, `fakeCascadeTxWorker`, `workerMarkPendingProxy`) gains an `UpsertEdgesWithMerge` method (mostly forwarding to the existing UpsertEdges recorder). |
| `internal/daemon/live_wiring.go` | Production `storeCascadeTxAdapter.UpsertEdges` now forwards to `UpsertEdgesWithMerge`; new `UpsertEdgesWithMerge` method translates `lspenrich.Edge` → `semanticstore.EdgeRow` and routes through the OverlayTx merge boundary. |
| `internal/config/defaults.go` | +4 keys: `semantic_index.pagerank.repair_debounce_ms` (2000), `semantic_index.pagerank.full_recompute_idle_ms` (60000), `semantic_index.pagerank.full_recompute_threshold` (float64(0.25) per koanf gotcha), `semantic_index.types.comment_parsers_enabled` ([]string{tsdoc, jsdoc, godoc, python_type_comments, phpdoc, yard}). |
| `internal/config/loader_test.go` | +4 assertions inside the existing TestLoad_Defaults block; +use of `reflect.DeepEqual` for the slice. |
| `internal/semantic/config.go` | +3 fields on PageRankConfig (RepairDebounceMs, FullRecomputeIdleMs, FullRecomputeThreshold); +new TypesConfig sub-struct (CommentParsersEnabled); Config struct now has Types field. |
| `internal/obs/metrics.go` | +5 Vec declarations (SemanticGraphPagerankDurationVec, SemanticGraphScoreStatusVec, SemanticGraphRepairVec, SemanticGraphVersionGauge, SemanticTypesResolutionVec); +5 drop-on-unknown helpers (SemanticGraphPagerankObserve, SemanticGraphScoreStatusInc, SemanticGraphRepairInc, SemanticGraphVersionSet, SemanticTypesResolutionInc); +5 closed-enum allowlist maps. All 5 registered with the owned registry. |
| `internal/obs/metrics_labels_test.go` | +5 carve-out entries for new label names (scope, projection, status, confidence_tier; workspace_label mirrors the helix_semantic_store_* family); +5 prime calls inside TestMetricsLabelsAllowlist so Gather() emits non-empty families. |

## Test Results

| Suite | Result | Notes |
| ----- | ------ | ----- |
| `go vet ./internal/... ./cmd/...` | PASS | Only the preexisting Swift binding `TOKEN_COUNT` warning surfaces (out-of-scope). |
| `go test ./internal/semantic/graph/... -count=1` | PASS | 14 tests across repair / status / apply_repair / ranker. |
| `go test ./internal/semantic/graph/... -race -run TestApplyRepair_ProductionMutexSerializes` | PASS | W1 invariant confirmed (-race clean). |
| `go test ./internal/semantic/store/... -count=1` | PASS | 9 new store tests + every prior overlay test. |
| `go test ./internal/semantic/live/... -count=1` | PASS | Handler tests green after RankApplier wiring. |
| `go test ./internal/semantic/lspenrich/... -count=1` | PASS | TestCascadeLSP_UpgradesCommentEdgeInPlace closes AC10 end-to-end. |
| `go test ./internal/config/... -count=1` | PASS | TestLoad_Defaults validates all 4 new keys. |
| `go test ./internal/obs/... -count=1` | PASS | TestMetricsLabelsAllowlist clean after carve-out + prime updates. |
| `go test ./internal/... -count=1` | PASS | Whole-internal green (excluding `tmp/` test fixtures, untracked). |
| Single-bump invariant: `grep -rE "BumpGraphVersion" internal/semantic/graph/ --include='*.go' \| grep -v _test.go` | 4 hits | 1 interface decl + 2 doc comments + 1 actual call site (apply_repair.go:148). |
| D-14 boundary invariant: `go list -deps ./internal/semantic/graph/... \| grep -E 'marcboeker\|duckdb\|internal/kernel' \| grep -v internal/semantic/store` | EMPTY | Graph package only reaches DuckDB through `internal/semantic/store`; no kernel imports. |

## Acceptance Criteria

### GRAPH-03 ranked responses carry graph_version + enrichment_level
- `Ranker.Rank` returns `RankResponse{GraphVersion: <currentGV>, EnrichmentLevel: ...}`. Tested by `TestRanker_RankCarriesGraphVersion`.

### GRAPH-05 score persistence carries closed-enum status
- `ScoreStatus` typed string + 4 constants (`exact`, `approximate`, `stale`, `missing`) shipped in `internal/semantic/graph/status.go`.
- `computeScoreStatus` covers all 4 cases tested by `TestComputeScoreStatus_ClosedEnum` (7 cases).
- `Ranker.Status` returns `RankStatus{PerProjection: map[string]ProjectionStatus}` exposing per-projection counts. Tested by `TestRanker_StatusCarriesPerProjectionStatus`.

### D-05 separation
- `current_epoch` (Phase 60) and `graph_version` (Phase 62) are independent counters on `semantic_live_overlay_meta`. `UpsertGraphScores` advances `current_epoch` (via `BeginOverlayTx`) but does NOT advance `graph_version`. Tested by `TestUpsertGraphScores_DoesNotBumpGraphVersion`.

### D-06 single-bump site
- `Engine.ApplyRepair` is the only call site that invokes `BumpGraphVersion`. Production grep returns exactly one call site (`apply_repair.go:148`). Tested by `TestApplyRepair_SingleBumpSiteOnly` (10 random repairs → 10 bumps, never 11 or 9).

### D-07 read-time score_status
- `computeScoreStatus(currentGV, rowGV, hasRow, approximate)` returns one of the 4 constants. The closed enum is enforced at the storage write boundary too (`UpsertGraphScores` rejects "missing" — it is read-time only).

### D-14 storage-side merge predicate
- `UpsertEdgesWithMerge` enforces the predicate at SQL: comment.* writes are skipped when a validated lsp.* row at conf>=1.0 already covers the triple; lsp.* writes DELETE every comment.* row sharing (src, edge_kind) regardless of dst (the refutation rule). Tested end-to-end by `TestUpsertEdgesWithMerge_LSPRefutesCommentAtDifferentDst` (storage layer) and `TestCascadeLSP_UpgradesCommentEdgeInPlace` (cascade → store path, AC10).

### T4 mitigation: per-workspace mutex re-use
- `Engine.ApplyRepair` calls `e.store.LockWorkspace(repoID)` which delegates (in production) to `Store.LockOverlayWorkspace`, which acquires the existing per-workspace mutex via `s.overlayLockFor(repoID)`. No second mutex map exists in `internal/semantic/graph/`.
- Tested by `TestApplyRepair_HoldsMutex` (recording fake) and `TestApplyRepair_ProductionMutexSerializes` (real per-workspace mutex map, -race runnable).

### T5 mitigation: bounded-label metrics
- 5 new metrics; every helper validates label values against package-private allowlist maps BEFORE calling `WithLabelValues`. Unknown values drop the emission. Tested implicitly by `TestMetricsLabelsAllowlist` (Gather() rejects forbidden labels) and explicitly by carve-out documentation parity.

### Acceptance Criteria 4 (graph_version advances exactly once per ApplyRepair)
- `TestApplyRepair_VersionMonotonic` (5 sequential calls → gv 1,2,3,4,5).
- `TestApplyRepair_BodyOnlyNoBump` (empty repair → 0 bumps).

### Acceptance Criteria 5 (every read response carries one of the 4 score statuses)
- `TestComputeScoreStatus_ClosedEnum` walks the matrix; each return value is asserted to be one of the 4 declared constants.

## Commits

| Task | Hash | Message |
| ---- | ---- | ------- |
| 1 | `58dfb013` | `test(62-02): add failing tests for ApplyRepair version monotonic + body-only no-bump + score_status closed enum` |
| 2 | `00d1362d` | `feat(62-02): extend store.OverlayTx with UpsertGraphScores/UpsertEdgesWithMerge/BumpGraphVersion + Store.CurrentGraphVersion` |
| 3 | `10d4dbfd` | `feat(62-02): implement internal/semantic/graph (repair, apply_repair, status, ranker, metrics, trace)` |
| 4 | `7f1cd49e` | `feat(62-02): wire post-commit ApplyRepair hook in handler; preserve cascade WriteInvalidations seam` |
| 4.5 | `e2e10fb7` | `fix(62-02): route Phase 61 cascade LSP edge writes through UpsertEdgesWithMerge (B3 - closes AC10 in-place upgrade)` |
| 5 | `2635d7d8` | `feat(62-02): add 4 config keys + 5 bounded-label metrics for Phase 62 (T5 mitigation)` |
| 6 | `86865896` | `chore(62-02): wire production storeCascadeTxAdapter.UpsertEdgesWithMerge` |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Production cascade adapter missing `UpsertEdgesWithMerge`**
- **Found during:** Task 6 (project-wide gate).
- **Issue:** Adding `UpsertEdgesWithMerge` to the `lspenrich.CascadeTx` interface in Task 4.5 broke `internal/daemon/live_wiring.go`'s `storeCascadeTxAdapter` (production wraps `*store.OverlayTx`); `go vet ./internal/...` flagged the missing method.
- **Fix:** Added `storeCascadeTxAdapter.UpsertEdgesWithMerge` translating `lspenrich.Edge` → `semanticstore.EdgeRow` and routing through `tx.UpsertEdgesWithMerge`; the legacy `UpsertEdges` alias now forwards to the merge entrypoint so every production cascade write enters the D-14 boundary.
- **Files modified:** `internal/daemon/live_wiring.go`.
- **Commit:** `86865896` (Task 6 fix folded into the project-wide gate commit per the `chore(62-02): cleanup` plan-action note).

**2. [Rule 3 — Blocking] duckdb-go rejects high-bit uint64 values**
- **Found during:** Task 2.
- **Issue:** `TestUpsertEdgesWithMerge_LSPRefutesCommentAtDifferentDst` failed with `sql: converting argument $2 type: uint64 values with high bit set are not supported`. The natural-key-hashed `edge_id` returned by FNV-1a routinely sets the high bit.
- **Fix:** Mask the synthesized `edge_id` to 63 bits (`& 0x7FFFFFFFFFFFFFFF`). Collision probability for the per-workspace edge surface (< 10M edges) remains vanishingly small; same-triple writes always converge on the same `edge_id`.
- **Files modified:** `internal/semantic/store/overlay.go` (`edgeIDForTriple`).
- **Commit:** `00d1362d` (folded into the Task 2 commit).

### W2 LOCKED — handler-tracked diff (planned)

The plan locked W2 as "handler accumulates `FileFactDiff` LOCALLY during the tx span (no `OverlayTx.Diff()` method)." Implementation honours this verbatim — no method added to `OverlayTx`. The diff is currently empty because Phase 60 P02's `UpsertOverlayFile` is the only fact write today; `repair.IsEmpty()` short-circuits inside `ApplyRepair`. Phase 60 P04 + Phase 62 P05 will populate the diff as symbol-level upserts land. This is documented in the handler comment block above `var diff graphpkg.FileFactDiff`.

### Schema column naming (D-14 plumbing)

The plan template referenced `score_name` in some places and `projection` in others; the actual schema (Phase 57 migration v1) ships `score_name`. The implementation maps the Phase 62 "projection" identifier into the `score_name` column verbatim — no schema change required. Documented in the `UpsertGraphScores` comment.

## Auth Gates

None. Pure in-process Go work.

## Threat Flags

None — every surface introduced sits inside the trust boundaries the plan's `<threat_model>` already covers:

- **T-62-02-T2 (graph_version monotonicity)** — mitigated by single-bump invariant. Acceptance: `TestApplyRepair_VersionMonotonic` + `TestApplyRepair_SingleBumpSiteOnly` + grep gate (1 production call site).
- **T-62-02-T3 (Comment-edge upgrade race)** — mitigated by `tx.UpsertEdgesWithMerge` SQL boundary. Acceptance: 3 D-14 tests covering skip / delete / refutation paths.
- **T-62-02-T4 (Cross-workspace state bleed)** — mitigated by re-using `store.overlayLockFor`; no second mutex map. Acceptance: `TestApplyRepair_HoldsMutex` + `TestApplyRepair_ProductionMutexSerializes` (-race).
- **T-62-02-D2 (Label cardinality DoS)** — mitigated by closed-enum allowlists. Acceptance: 5 helper-method drop-on-unknown guards verified by `TestMetricsLabelsAllowlist`.
- **T-62-02-I2 (slog repo_id)** — accepted (existing log discipline; no new PII surface).

## Downstream Readiness

P03 (RankScheduler + 1-hop frontier + WriteInvalidations consumer) may now consume:

```go
import "github.com/agenthands/helix/internal/semantic/graph"

eng := graph.NewEngine(prodStore)              // Engine.ApplyRepair = single bump site
eng.SetMetrics(obsMetrics)                     // T5 bounded-label sink
eng.SetNotifyChannel(rankSchedulerCh)          // GraphVersionAdvance flow

// RankScheduler subscribes to advances on its in-channel
for adv := range rankSchedulerCh {
    // adv.RepoID, adv.Version, adv.ChangedNodes (sorted ascending)
}
```

`store.OverlayTx.UpsertGraphScores` is ready for the scheduler's score-row writes; `store.OverlayTx.UpsertEdgesWithMerge` is ready for P05's two-phase comment emission. `store.Store.CurrentGraphVersion` and `Ranker.Status` are ready for Phase 64's MCP tool path.

The post-commit hook in `internal/semantic/live/handler` automatically fires `Engine.ApplyRepair` whenever the local `FileFactDiff` carries graph-changing edits — the daemon wiring just needs to call `handler.SetRankApplier(graph.NewEngine(...))` once at bootstrap.

## Self-Check: PASSED

Verified files exist:
- `internal/semantic/graph/doc.go`, `repair.go`, `apply_repair.go`, `status.go`, `ranker.go`, `metrics.go`, `trace.go` ✓
- `internal/semantic/graph/repair_test.go`, `apply_repair_test.go`, `status_test.go`, `ranker_test.go` ✓
- `internal/semantic/lspenrich/cascade_merge_test.go` ✓
- `internal/semantic/store/overlay.go`, `overlay_test.go` (extended) ✓
- `internal/semantic/live/handler/handler.go` (extended) ✓
- `internal/semantic/lspenrich/cascade.go` (extended) ✓
- `internal/daemon/live_wiring.go` (extended) ✓
- `internal/config/defaults.go`, `loader_test.go` (extended) ✓
- `internal/semantic/config.go` (extended) ✓
- `internal/obs/metrics.go`, `metrics_labels_test.go` (extended) ✓

Verified commits exist (`git log --oneline`):
- `58dfb013` ✓
- `00d1362d` ✓
- `10d4dbfd` ✓
- `7f1cd49e` ✓
- `e2e10fb7` ✓
- `2635d7d8` ✓
- `86865896` ✓

## TDD Gate Compliance

- RED gate: `58dfb013` `test(62-02): ...` — package fails to compile (undefined NodeID, GraphRepair, RepairStore, Engine, ScoreStatus, etc.) ✓
- GREEN (storage) gate: `00d1362d` `feat(62-02): extend store.OverlayTx ...` — store-layer implementation lands first so Task 3's RepairTx interface has a concrete shape to mirror ✓
- GREEN (engine) gate: `10d4dbfd` `feat(62-02): implement internal/semantic/graph ...` — package compiles, all 14 RED tests turn GREEN ✓
- WIRING gate: `7f1cd49e` (handler), `e2e10fb7` (cascade B3), `2635d7d8` (config + metrics), `86865896` (production adapter) ✓

All gates present in the correct order. The plan is type=tdd at the plan level; the per-test RED → GREEN cycle for ApplyRepair / status / ranker is gate-clean.
