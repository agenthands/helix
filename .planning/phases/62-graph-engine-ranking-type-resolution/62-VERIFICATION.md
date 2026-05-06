---
phase: 62-graph-engine-ranking-type-resolution
verified: 2026-05-06T20:00:00Z
status: gaps_found
score: 18/22 must-haves verified
overrides_applied: 0
gaps:
  - truth: "Production rankStoreAdapter ships QueryEffectiveGraph / QueryEffectiveAdjacency / CountStaleScoreRows / MarkAllScoreRowsStale as no-op stubs returning empty results, masking 'no data' semantics with clean metrics"
    status: partial
    reason: "RankScheduler is structurally wired and runs per workspace, but the production adapter returns empty maps from all four read paths. This means every incremental repair computes an empty frontier and writes zero score rows; every full recompute deletes any prior score rows and commits an empty new generation (see WR-06). The lifecycle / determinism / per-workspace isolation invariants hold, but the rank surface is operationally inert. Operators monitoring helix_semantic_graph_repair_total{outcome='applied'} see clean success metrics that mask 'no data'. Plans documented this as a Phase 64 follow-up but the must_have 'GRAPH-04 incremental repair runs only inside a bounded max_local_pagerank_nodes frontier' is structurally true and operationally vacuous today."
    artifacts:
      - path: "internal/daemon/rank_wiring.go"
        issue: "Lines 251-273: 4 read methods are documented Phase 64 follow-up stubs returning empty"
    missing:
      - "Real QueryEffectiveAdjacency reading from semantic_live_overlay_edges joined with snapshot edges"
      - "Real CountStaleScoreRows reading from semantic_graph_scores with status filter"
      - "Real MarkAllScoreRowsStale UPDATE on semantic_graph_scores"
      - "OR a feature-flag / observability signal so operators can detect the gap (e.g., outcome='stub_no_data' metric, or sync.Once-gated WARN log on first stub call)"
  - truth: "Live handler post-commit hook fires graph.ComputeGraphRepair on a FileFactDiff that is currently empty in production — ApplyRepair always short-circuits via repair.IsEmpty()"
    status: partial
    reason: "Handler.UpdateChangedFile() declares 'var diff graphpkg.FileFactDiff' with a comment that the diff is empty until Phase 60 P04 (full FileFact upsert) and Phase 62 P05 type-resolver edges land. The hook compiles, runs nil-safe, and the contract is exercised by tests with synthetic non-empty diffs (TestApplyRepair_VersionMonotonic, TestApplyRepair_BodyOnlyNoBump, etc.). However, in the daemon's actual live update path the hook never fires ApplyRepair productively because the diff is unconditionally empty. The must_have 'graph_version advances exactly once per ApplyRepair call when repair is non-empty' is honored at the contract layer, but the trigger that would surface the contract in production is unwired."
    artifacts:
      - path: "internal/semantic/live/handler/handler.go"
        issue: "Line 233: 'var diff graphpkg.FileFactDiff' declares an empty struct that flows through ComputeGraphRepair → IsEmpty()=true → no ApplyRepair call"
    missing:
      - "Phase 60 P04 / Phase 62 P05 wiring that populates SymbolDiff entries (SignatureChanged, ExportedChanged, KindChanged, StableKeyChanged, BodyOnlyChanged) and AddedEdges/RemovedEdges from tx-scoped recorders during UpsertOverlaySymbols / MarkSymbolsDeleted / UpsertEdges / MarkEdgesDeleted"
  - truth: "RankScheduler.runIncrementalRepair holds the per-workspace overlay lock across CountStaleScoreRows callback (post-commit stale-fraction probe runs while lock is held) — latent self-deadlock once a real CountStaleScoreRows lands"
    status: failed
    reason: "Code review CR-01 finding. scheduler.go:237 acquires `release := s.store.LockWorkspace(s.repoID)` with `defer release()`; line 282 calls `s.store.CountStaleScoreRows(ctx, ...)` while the workspace lock is STILL held (defer fires only at function return). Today CountStaleScoreRows is a stub (returns 0,0,nil) so the deadlock is dormant — but the contract on LockWorkspace is the SAME per-workspace mutex BeginRepairTx + BeginOverlayTx use. The moment a real CountStaleScoreRows opens its own tx for the COUNT (or any future read path that takes the same lock), this becomes a hard self-deadlock."
    artifacts:
      - path: "internal/semantic/graph/scheduler.go"
        issue: "Lines 237-296: defer release() at 238 keeps lock held through commit (271) AND CountStaleScoreRows (282)"
    missing:
      - "Explicit release() after tx.Commit() (line 277) BEFORE the post-commit stale-fraction probe"
      - "OR documented contract on SchedulerStore.CountStaleScoreRows that it MUST NOT acquire the workspace lock — but that contract is fragile and should be tightened by structure, not convention"
  - truth: "Per-language guessFromName helpers in golang/typescript/python iterate map[string]string directly — silent determinism break if any future suffix overlap is added"
    status: failed
    reason: "Code review CR-03 finding. golang/resolver.go:259-280, python/resolver.go:198-221, typescript/resolver.go:193-211 each have `for short, long := range suffixMap` over a literal map. Today's suffix set ('Repo', 'Svc', 'Mgr', 'Ctrl', 'Cfg', 'Conf', 'Hdlr', 'Conn') happens not to overlap as suffixes of common identifiers, so a single iteration deterministically picks at most one match. Any future suffix addition that shares a tail with another (or a refactor introducing overlap) will silently break GRAPH-01 byte-equality and TYPES-04 determinism — the phase otherwise enforces sort-before-iterate religiously (frontier.go, pagerank.go, weak.go, util.go.sortedNodeIDs, internal/graph/pagerank.go D-03). These three helpers are the only places in Phase 62's surface where the rule is violated."
    artifacts:
      - path: "internal/semantic/types/golang/resolver.go"
        issue: "Lines 259-280: bare `for short, long := range suffixMap` over a map literal"
      - path: "internal/semantic/types/python/resolver.go"
        issue: "Lines 198-221: same pattern"
      - path: "internal/semantic/types/typescript/resolver.go"
        issue: "Lines 193-211: same pattern"
    missing:
      - "Replace map iteration with sorted slice-of-rules (`type rule struct{ short, long string }; rules := []rule{...}; for _, r := range rules { ... }`) in all three files, alphabetically sorted by short suffix"
deferred:
  - truth: "Cluster MCP tools (get_cluster_map, explain_cluster) deferred to v1.10.x"
    addressed_in: "v1.10.x (post-v1)"
    evidence: "ROADMAP / 62-CONTEXT 'Out of scope' explicitly defers; SUMMARY 04 confirms zero MCP tool registrations in cluster package; this is by-design scope, not a gap"
  - truth: "Real effective-graph queries (QueryEffectiveAdjacency / CountStaleScoreRows / MarkAllScoreRowsStale) populate live data"
    addressed_in: "Phase 64"
    evidence: "62-03 SUMMARY explicitly defers: 'the typed effective-graph reads land in Phase 64 alongside the MCP tool surface'; rank_wiring.go:251-273 stubs are commented as Phase 64 follow-ups. The wiring is in place; the queries are deferred to the natural callsite (MCP tools)."
  - truth: "FileFactDiff population via tx-scoped recorders during UpsertOverlaySymbols / UpsertEdges"
    addressed_in: "Phase 60 P04 + Phase 62 P05 (in-place; not yet executed)"
    evidence: "handler.go line 227-233 explicit comment: 'Phase 60 P02 surface writes only the file-row contentHash. Phase 60 P04 (full FileFact upsert) and Phase 62 P05 (type resolver edge emission) will fill this struct'. However Phase 62 P05 was completed without retrofitting the diff population; this is a real gap (see truth 2 above), NOT cleanly deferred."
---

# Phase 62: Graph Engine, Ranking & Type Resolution — Verification Report

**Phase Goal:** Ship deterministic generic PageRank engine, graph_version + ApplyRepair + score_status, per-workspace RankScheduler with WriteInvalidations consumer, weak-component clustering with overlay-tx persistence, and tiered type-resolution layer (Go/TS/Python full ladder; Java/PHP/Ruby stubs). Requirement IDs: GRAPH-01..GRAPH-06 + TYPES-01..TYPES-04.

**Verified:** 2026-05-06
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Deterministic generic PageRank engine ships at internal/graph with stdlib-only imports | VERIFIED | `internal/graph/{pagerank,options,personalize}.go` exist; `go list -deps ./internal/graph/... \| grep github.com/agenthands/helix \| grep -v 'internal/graph$'` returns empty (D-01); 3 sha256 goldens (golden_uniform/personalized/tiebreak); `go test ./internal/graph/... -count=10 -run TestPageRank_Deterministic` passes |
| 2 | Personalized PageRank with seed weighting (GRAPH-02) | VERIFIED | `internal/graph/personalize.go` — `RankNodes[T cmp.Ordered]` + `Options.Personalize map[any]float64`; epsilon=1e-6 honored; TestPageRank_Personalized passes |
| 3 | Tiebreak by sorted-key NodeID order (GRAPH-03 stable-key) | VERIFIED | TestPageRank_TieBreak passes; golden_tiebreak.txt pinned |
| 4 | internal/repomap/pagerank.go is a thin adapter over internal/graph (no math.Abs body) | VERIFIED | wc -l = 86 lines; grep -c "math.Abs" = 0; existing repomap tests pass against migrated engine |
| 5 | graph_version advances exactly once per ApplyRepair call when repair is non-empty | VERIFIED (contract) | apply_repair.go:148 is single BumpGraphVersion call site (production-grep returns 1); TestApplyRepair_VersionMonotonic + TestApplyRepair_SingleBumpSiteOnly pass |
| 6 | Body-only edits do NOT bump graph_version (repair.IsEmpty short-circuit) | VERIFIED | TestApplyRepair_BodyOnlyNoBump passes; ApplyRepair returns (gv, false, nil) on empty repair without acquiring mutex or opening tx |
| 7 | ApplyRepair runs under the same per-workspace overlay mutex as current_epoch (Phase 60 D-04) | VERIFIED | `e.store.LockWorkspace(repoID)` delegates to `Store.LockOverlayWorkspace` → `overlayLockFor(repoID)`; TestApplyRepair_ProductionMutexSerializes passes -race |
| 8 | score_status closed enum {exact, approximate, stale, missing} computed at READ time (D-07) | VERIFIED | status.go declares 4 constants; computeScoreStatus covers all 4; TestComputeScoreStatus_ClosedEnum passes; UpsertGraphScores rejects "missing" at write boundary |
| 9 | tx.UpsertGraphScores persists score rows under current_epoch but does NOT advance graph_version | VERIFIED | TestUpsertGraphScores_DoesNotBumpGraphVersion passes |
| 10 | tx.UpsertEdgesWithMerge enforces (src,dst,kind) merge predicate at SQL boundary (D-14) | VERIFIED | overlay.go:496+; 3 D-14 tests including LSPRefutesCommentAtDifferentDst (refutation rule); 5 cascade.go LSP edge sites migrated (lines 377/394/412/430/465) |
| 11 | RankScheduler is one goroutine per workspace; non-blocking Notify with drop-on-full | VERIFIED | scheduler.go:55+ RankScheduler; Notify uses select+default; TestRankScheduler_PerWorkspaceIsolation + TestRankScheduler_NoBlock_ChannelDropOnFull + TestRankScheduler_DebounceCoalesces all pass |
| 12 | 1-hop frontier with sort-before-iterate determinism + 5000-node overflow signal | VERIFIED | frontier.go:1-67 + util.go.sortedNodeIDs; TestComputeFrontier_DeterministicAcrossRuns runs 100×; TestComputeFrontier_OverflowReturnsTrue verifies overflow signal |
| 13 | Full recompute pre-empted by mid-run advance → status=approximate atomically | VERIFIED | full_recompute.go RunFullRecompute; TestRunFullRecompute_PreemptedMarksApproximate verifies the rewrite path; D-10 honored |
| 14 | Weak-component algorithm shipped with sorted union-find determinism (GRAPH-06) | VERIFIED | cluster/weak.go WeakComponents + Cluster types; 3 sha256 goldens (744cb4ae… / f112d312… / 3281f71a…); -count=10 determinism multiplier passes |
| 15 | Cluster persistence via overlay tx (UpsertClusters/UpsertClusterMembers/DeleteClustersForGraphVersion) | VERIFIED | overlay.go:631+; 5 round-trip tests pass; ClusterSummary/ClusterMemberRow boundary types resolve cluster→store import direction |
| 16 | 7-tier confidence ladder constants + CapCommentConfidence (TYPES-01, TYPES-03) | VERIFIED | ladder.go declares all 7 constants matching SPEC §38.2; CapCommentConfidence enforces 0.60 cap on EvidenceComment; TestEmitEdges_CommentNeverExceeds060 passes |
| 17 | Bounded chain depth=8 + fixpoint=8 with state-hash early-exit (TYPES-02) | VERIFIED | chain.go ResolveAccessChain; fixpoint.go FixpointResolve; tests verify depth bound, early-exit, non-converged → unresolved |
| 18 | Non-converged chains never emit "validated" (TYPES-04) | VERIFIED | TestFixpoint_NonConverged + TestEmitEdges_UnresolvedAlwaysHasState pass; emit.go defensive: any Resolved=false → state="unresolved" |
| 19 | All 7 languages registered in dispatcher with javascript→typescript alias (D-11) | VERIFIED | daemon.go:394-401 registers 7 entries; both javascript and typescript point to tsResolver; PHP/Ruby always-0.20 stubs; Java LSP-conditional stub |
| 20 | Daemon errgroup owns RankScheduler.Run + dispatcher constructed at boot | VERIFIED | daemon.go:773-775 `if d.rank != nil { g.Go(func() error { return d.rank.Run(gctx) }) }`; daemon.go:394 calls types.NewDispatcher; nil-safe when semantic config disabled |
| 21 | Production rankStoreAdapter actually surfaces real rank data | FAILED | rank_wiring.go:251-273 — 4 read methods are documented Phase 64 follow-up stubs returning empty maps and `0, 0, nil`. Incremental repair computes empty frontier; full recompute commits empty generation. No observability signal flags the gap. |
| 22 | Live handler post-commit hook fires ApplyRepair productively in production | FAILED | handler.go:233 — `var diff graphpkg.FileFactDiff` is unconditionally empty; the comment cites Phase 60 P04 / Phase 62 P05 as the populators. P05 shipped but did NOT retrofit the diff population, so production ApplyRepair always short-circuits via IsEmpty(). |

**Score:** 18/22 truths verified

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | Cluster MCP tools (get_cluster_map, explain_cluster) | v1.10.x | ROADMAP / 62-CONTEXT 'Out of scope' explicitly defers — by-design |
| 2 | Real effective-graph queries (QueryEffectiveAdjacency / CountStaleScoreRows) | Phase 64 | rank_wiring.go stubs commented as Phase 64 follow-ups; queries are the natural callsite for the MCP tool surface |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| internal/graph/pagerank.go | PageRank[T cmp.Ordered] | VERIFIED | 130 LOC; sort-before-iterate at entry; stdlib-only imports |
| internal/graph/options.go | Options struct | VERIFIED | 43 LOC |
| internal/graph/personalize.go | RankNodes + Ranked[T] | VERIFIED | 92 LOC |
| internal/graph/testdata/pagerank/ | 3 sha256 goldens | VERIFIED | 64-char hex strings each |
| internal/repomap/pagerank.go | Thin adapter | VERIFIED | 86 LOC; no math.Abs body |
| internal/semantic/graph/repair.go | GraphRepair + ComputeGraphRepair | VERIFIED | 165 LOC |
| internal/semantic/graph/apply_repair.go | Engine.ApplyRepair single bump | VERIFIED | 331 LOC; single call to BumpGraphVersion at line 148 |
| internal/semantic/graph/status.go | ScoreStatus closed enum | VERIFIED | 45 LOC; 4 constants |
| internal/semantic/graph/ranker.go | Ranker interface | VERIFIED | 124 LOC |
| internal/semantic/graph/scheduler.go | RankScheduler | VERIFIED | 373 LOC; debounce + long-idle + drop-on-full |
| internal/semantic/graph/frontier.go | ComputeFrontier | VERIFIED | 67 LOC; uses sortedNodeIDs |
| internal/semantic/graph/util.go | sortedNodeIDs[V any] | VERIFIED | 28 LOC |
| internal/semantic/graph/full_recompute.go | RunFullRecompute | VERIFIED | 187 LOC; D-10 preemption rewrite |
| internal/semantic/graph/invalidations_consumer.go | InvalidationsConsumer | VERIFIED | 92 LOC; derived path per RESEARCH OQ3 |
| internal/semantic/store/overlay.go (extensions) | UpsertGraphScores/UpsertEdgesWithMerge/BumpGraphVersion/UpsertClusters/UpsertClusterMembers/DeleteClustersForGraphVersion/DeleteScoresForProjection/CurrentGraphVersion | VERIFIED | All 8 methods present (lines 447, 496, 578, 631, 667, 702, 778) |
| internal/semantic/cluster/weak.go | WeakComponents | VERIFIED | 128 LOC; sorted union-find |
| internal/semantic/cluster/persist.go | RunClusterDetection | VERIFIED | 141 LOC |
| internal/semantic/cluster/testdata/ | 3 sha256 goldens | VERIFIED | 64-char hex each |
| internal/semantic/types/{ladder,chain,fixpoint,emit,resolver,doc}.go | Shared core | VERIFIED | 558 LOC total |
| internal/semantic/types/golang/{scope,comment,resolver}.go | Go full ladder | VERIFIED | 7 fixtures + tests |
| internal/semantic/types/typescript/{scope,comment,resolver}.go | TS/JS full ladder | VERIFIED | 7 fixtures + tests |
| internal/semantic/types/python/{scope,comment,resolver}.go | Python full ladder | VERIFIED | 7 fixtures + tests |
| internal/semantic/types/{java,php,ruby}/stub.go | 3 stubs (Java conditional, PHP/Ruby always-0.20) | VERIFIED | All present |
| internal/daemon/rank_wiring.go | rankBundle + adapters | ORPHANED (partial) | 344 LOC; production read methods are Phase 64 stubs (see truth 21) |
| internal/daemon/type_resolver_wiring.go | typeStoreAdapter + per-lang wrappers + SetSemanticGraph | VERIFIED | 136 LOC |
| internal/daemon/daemon.go | bundle bootstrap + dispatcher registration + errgroup wiring | VERIFIED | rank.engine, types.NewDispatcher, d.rank.Run all present |
| internal/config/defaults.go | 4 new keys (repair_debounce_ms, full_recompute_idle_ms, full_recompute_threshold, comment_parsers_enabled) | VERIFIED | All 4 keys present |
| internal/obs/metrics.go | 5 bounded-label metrics + helpers | VERIFIED | 5 Vecs + 5 drop-on-unknown helpers + allowlist guards |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| internal/repomap/pagerank.go | internal/graph.PageRank | direct call from FileGraph.PageRank method body | WIRED | grep "graph.PageRank" returns ≥1 in adapter |
| internal/graph/pagerank.go | stdlib only | imports cmp, math, sort | WIRED | go list -deps confirms zero project imports |
| internal/semantic/live/handler/handler.go | internal/semantic/graph.Engine.ApplyRepair | post-commit hook in updateChangedFile, AFTER tx.Commit | WIRED (compile) / DISCONNECTED (runtime) | hook is wired and nil-safe but consumes an empty FileFactDiff in production — see truth 22 |
| internal/semantic/graph/apply_repair.go | store.OverlayTx + per-workspace mutex | re-uses store.overlayLockFor(repoID) — NO second mutex map | WIRED | Engine.store.LockWorkspace → Store.LockOverlayWorkspace → overlayLockFor; production grep 'sync.Map\|map[string]\*sync.Mutex' on internal/semantic/graph/ excluding tests = empty |
| internal/semantic/graph/scheduler.go | internal/graph.PageRank | incremental + full recompute paths invoke graph.PageRank | WIRED | scheduler.go and full_recompute.go both call gengraph.PageRank |
| internal/semantic/graph/scheduler.go | apply_repair.go notifyVersion channel | scheduler reads each advance | WIRED | rank_wiring.go fan-out demuxer reads engine.notifyCh and dispatches Notify per repo |
| internal/daemon/daemon.go | internal/semantic/graph.RankScheduler | g.Go(func() error { return d.rank.Run(gctx) }) | WIRED | daemon.go:773-775 |
| internal/semantic/cluster/persist.go | internal/semantic/cluster.WeakComponents | RunClusterDetection calls WeakComponents | WIRED | persist.go calls WeakComponents on effective graph |
| internal/semantic/cluster/persist.go | store.OverlayTx | UpsertClusters + UpsertClusterMembers in single tx | WIRED | persist.go threads ClusterSummary/ClusterMemberRow through tx interface |
| internal/semantic/types/emit.go | store.OverlayTx.UpsertEdgesWithMerge | Two-phase comment merge predicate | WIRED | emit.go calls tx.UpsertEdgesWithMerge with EdgeRow batches |
| internal/semantic/types/resolver.go | per-language resolvers | Dispatcher routes by req.Language; "javascript" aliases to "typescript" | WIRED | daemon.go:394-401 + resolver.go alias logic |
| internal/semantic/types/java/stub.go | store.QueryEffectiveEdges | Reads existing RESOLVES_TO edges; short-circuits on Confidence>=1.0 + lsp.* | WIRED | stub.go iterates edges checking strings.HasPrefix(Source, "lsp.") |

### Data-Flow Trace (Level 4)

| Artifact | Data Source | Produces Real Data | Status |
|----------|-------------|--------------------|--------|
| RankScheduler.runIncrementalRepair | rankStoreAdapter.QueryEffectiveAdjacency (returns empty) | NO | DISCONNECTED — Phase 64 follow-up |
| RankScheduler.maybeFullRecompute → RunFullRecompute | rankStoreAdapter.QueryEffectiveGraph (returns empty) | NO | DISCONNECTED — Phase 64 follow-up |
| Handler.UpdateChangedFile post-commit hook → Engine.ApplyRepair | handler-local FileFactDiff (always empty struct) | NO | HOLLOW — diff never populated (Phase 60 P04 / Phase 62 P05 deferred) |
| internal/repomap/FileGraph.PageRank → graph.PageRank | FileGraph.Files + FileGraph.Edges (real data) | YES | FLOWING — repomap path runs end-to-end against real repos |
| Cluster.WeakComponents (algorithm pure-function) | RunClusterDetection.QueryEffectiveGraph (Phase 64 stub) | NO | DISCONNECTED at orchestrator (algorithm itself works on synthetic data in tests) |
| Type resolver dispatcher | typeStoreAdapter (Phase 57 store stubs return empty) | NO | DISCONNECTED — falls through every tier to "unknown" 0.20 in production |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whole-repo go vet clean (excluding tmp/) | `go vet ./...` | Only tmp/ test fixture warnings (pre-existing) | PASS |
| Whole-repo tests pass (excluding tmp/) | `go test ./... -count=1` | All real packages PASS; only tmp/GitNexus and tmp/graphify fixtures FAIL build (out of scope) | PASS |
| Phase 62 packages all pass | `go test ./internal/graph/... ./internal/semantic/graph/... ./internal/semantic/cluster/... ./internal/semantic/types/... -count=1` | All 10 packages PASS | PASS |
| -race clean on graph/types | `go test ./internal/semantic/graph/ -race -count=1 -run "TestApplyRepair_ProductionMutexSerializes\|TestRankScheduler_PerWorkspaceIsolation\|TestRankScheduler_NoBlock_ChannelDropOnFull"` | PASS | PASS |
| Determinism multiplier (PageRank) | `go test ./internal/graph/... -count=10 -run TestPageRank_Deterministic` | PASS | PASS |
| Determinism multiplier (cluster) | `go test ./internal/semantic/cluster/ -count=10 -run TestWeakComponents` | PASS | PASS |
| Helix binary builds | `go build ./cmd/helix` | binary built; only pre-existing Swift binding TOKEN_COUNT cosmetic warning | PASS |
| Single-bump invariant (D-06) | `grep -rE "BumpGraphVersion" internal/semantic/graph/ --include='*.go' \| grep -v _test.go` | 1 production call site (apply_repair.go:148) + 2 doc comments + 1 interface decl | PASS |
| D-01 stdlib-only invariant | `go list -deps ./internal/graph/... \| grep github.com/agenthands/helix \| grep -v 'internal/graph$'` | EMPTY | PASS |
| All 10 requirement IDs flipped to [x] | `grep -cE '^- \[x\] \*\*(GRAPH-0[1-6]\|TYPES-0[1-4])' .planning/REQUIREMENTS.md` | 10 | PASS |
| ROADMAP Phase 62 entry flipped | `grep '^- \[x\] Phase 62' .planning/ROADMAP.md` | 1 match: "(5/5 plans)" | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| GRAPH-01 | 62-01 | Deterministic CALL_GRAPH PageRank; byte-identical scores | SATISFIED | TestPageRank_HexDigest + 10× determinism multiplier; sha256 goldens pinned; D-01 stdlib-only confirmed |
| GRAPH-02 | 62-01 | Personalized PageRank with seed weighting | SATISFIED | TestPageRank_Personalized; Options.Personalize map[any]float64; epsilon=1e-6 honored |
| GRAPH-03 | 62-02 | graph_version + enrichment_level; tiebreaks via stable-key NodeID | SATISFIED | Ranker.Rank returns RankResponse{GraphVersion,EnrichmentLevel}; TestPageRank_TieBreak proves sorted-key ordering |
| GRAPH-04 | 62-03 | Incremental local repair within 5000-node frontier; mark stale + schedule full recompute on overflow | SATISFIED (contract) / BLOCKED (production) | Algorithm + scheduler structurally complete (TestComputeFrontier_OverflowReturnsTrue); but production rankStoreAdapter returns empty maps so the path runs empty (truth 21) |
| GRAPH-05 | 62-02 + 62-03 | Score persistence carries status closed enum | SATISFIED | ScoreStatus enum + computeScoreStatus + UpsertGraphScores write boundary check; tests pass |
| GRAPH-06 | 62-04 | Weak-component clustering shipped + deterministic | SATISFIED | WeakComponents + 3 sha256 goldens; -count=10 determinism passes; cluster MCP tools deferred to v1.10.x by design |
| TYPES-01 | 62-05 | RESOLVES_TO/CALLS/USES_TYPE edges with 7-tier ladder | SATISFIED | All 7 constants in ladder.go; 7 ladder fixtures per language for Go/TS/Python; Java/PHP/Ruby stubs |
| TYPES-02 | 62-05 | max_chain_depth=8 + max_fixpoint_iterations=8 + early-exit on no-progress | SATISFIED | chain.go bounded walker + fixpoint.go state-hash early-exit; tests verify both bounds |
| TYPES-03 | 62-05 | Comment-derived edges never exceed 0.60 unless LSP-confirmed | SATISFIED | CapCommentConfidence in ladder.go; storage-side merge in UpsertEdgesWithMerge; TestEmitEdges_CommentNeverExceeds060 |
| TYPES-04 | 62-05 | Non-converged chains emit unresolved, never validated | SATISFIED | FixpointResolve forces unresolved on non-converged; emit.go defensive override; TestFixpoint_NonConverged passes |

### Anti-Patterns Found

(Per code review 62-REVIEW.md; surfaced here only those that materially threaten goal achievement.)

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/semantic/graph/scheduler.go | 237-296 | Workspace lock held across post-commit CountStaleScoreRows callback (CR-01) | Blocker (latent) | Self-deadlock once production CountStaleScoreRows opens its own tx; dormant today because the production stub returns 0,0,nil |
| internal/semantic/types/golang/resolver.go | 259-280 | `for short, long := range suffixMap` over a map literal (CR-03) | Warning | Silent determinism break if any future suffix overlaps; phase-wide sort-before-iterate rule violated only here |
| internal/semantic/types/python/resolver.go | 198-221 | Same pattern (CR-03) | Warning | Same |
| internal/semantic/types/typescript/resolver.go | 193-211 | Same pattern (CR-03) | Warning | Same |
| internal/semantic/graph/scheduler.go | 161-321 | time.AfterFunc callbacks capture activation-time ctx; no WaitGroup-blocked drain on Run return (CR-02) | Warning | Potentially-orphan timer callbacks may run after scheduler shutdown declared complete; muddy lifecycle contract |
| internal/semantic/graph/scheduler.go | 176-296 | Incremental repair writes score rows stamped with s.lastSeenGV without re-checking CurrentGraphVersion after lock acquisition (CR-04) | Warning | If competing ApplyRepair lands between QueryEffectiveAdjacency and tx commit, rows are written under stale gv and read back as ScoreStatusStale immediately. RunFullRecompute handles this via D-10; runIncrementalRepair does NOT mirror the defense |
| internal/daemon/rank_wiring.go | 251-273 | 4 read methods are documented Phase 64 stubs returning empty without observability signal (WR-05) | Warning | Production rank surface is empty with no operator-visible "no data" indicator |
| internal/semantic/graph/full_recompute.go | 97 | DeleteScoresForProjection runs unconditionally even when nodes is empty (WR-06) | Warning | Every fresh-bootstrap or stub-empty full recompute wipes any pre-existing score rows |
| internal/semantic/lspenrich/cascade.go | 332+349 | prioritizeSymbols mutates input slice after UpsertSymbols already wrote it (WR-01) | Info | Confusing data-flow ordering; today's in-process store consumes synchronously so no observable bug |
| internal/semantic/graph/full_recompute.go | 14, 165 | `_ = time.Now() // hold the time import` placeholder (WR-02) | Info | Dead code; trivial cleanup |
| internal/semantic/graph/trace.go | 1-15 | traceApplyRepair declared but never invoked (WR-03) | Info | Dead helper; either wire at top of Engine.ApplyRepair or delete |
| internal/daemon/rank_wiring.go | 23, 344 | `var _ = fmt.Sprintf` import-keepalive (WR-04) | Info | Dead import; trivial cleanup |
| internal/semantic/graph/repair.go | 141-156 | nodeSet has no doc disclaimer about single-goroutine ownership (WR-07) | Info | Future caller using shared nodeSet across goroutines would race on s.seen / s.order |

### Gaps Summary

The phase ships every artifact the plans promised and all 22 must-haves resolve to working code at the contract / unit-test level. The shared core (PageRank engine, GraphRepair, ApplyRepair single-bump-site, score_status closed enum, RankScheduler, frontier, full-recompute, weak components, type-resolution ladder + dispatcher) is implemented, tested, and race-clean. All 10 requirement IDs flipped to [x] in REQUIREMENTS.md and ROADMAP.md Phase 62 entry reads `(5/5 plans)`.

Two structural gaps prevent the phase from reaching end-to-end "real data flowing" status:

1. **Production data-source disconnect (truth 21).** rank_wiring.go ships 4 read methods as no-op Phase 64 follow-ups returning empty. The scheduler runs, the lifecycle invariants hold, but every incremental repair sees an empty frontier and every full recompute commits an empty generation. SUMMARY 62-03 documents this explicitly. The phase plans frame it as a Phase 64 dependency. However the must_have wording for GRAPH-04 ("incremental repair runs only inside a bounded frontier") is structurally honored and operationally vacuous — calling this "complete" depends on whether the verifier accepts "wired but no data" as goal-achievement. Recommendation: WARNING, surface to developer.

2. **Handler diff never populated (truth 22).** The post-commit hook in `handler.UpdateChangedFile` declares `var diff graphpkg.FileFactDiff` and threads it through ComputeGraphRepair → IsEmpty()=true → no ApplyRepair. The comment cites Phase 60 P04 and Phase 62 P05 as the intended populators. Phase 62 P05 has shipped without retrofitting the diff — every type-resolver edge written via `tx.UpsertEdgesWithMerge` does so on its OWN tx (P05 emit.go), not threaded through the handler's tx-scoped diff. So the phase's "ApplyRepair fires on graph-changing edits" promise is unwired in production. Recommendation: BLOCKER unless explicitly accepted as a known cross-phase carry-forward.

Two code-quality concerns from the review (CR-01 latent self-deadlock, CR-03 unsorted map iteration in 3 guessFromName helpers) are dormant today but threaten the determinism / liveness invariants the phase establishes. CR-03 in particular is an outright violation of the phase's own "sort-before-iterate every node-keyed map" doctrine. Recommendation: BLOCKER on CR-03 (silent future-break of GRAPH-01 / TYPES-04 determinism contracts); WARNING on CR-01 (latent until a real CountStaleScoreRows lands).

The other code-review findings (CR-02, CR-04, WR-01..WR-08, IN-01..IN-05) are valid code-quality concerns but do not directly threaten goal achievement; they belong in a follow-up plan rather than this verifier's gap list.

### Human Verification Required

(None. All goal-relevant truths are verifiable programmatically. The data-flow gaps are observable via grep + go-list inspection; no UX / visual / external-service items.)

---

_Verified: 2026-05-06_
_Verifier: Claude (gsd-verifier)_
