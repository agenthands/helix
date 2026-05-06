// Package graph wires the Phase 62 graph_version advance machinery, the
// score_status read-time API, and the Ranker interface that Phase 64 will
// consume.
//
// Invariants (must hold across every plan in Phase 62 and beyond):
//
//   - D-05 (separation): graph_version is a counter on
//     semantic_live_overlay_meta SEPARATE from current_epoch. Phase 60 owns
//     current_epoch (per-tx CAS for compaction). Phase 62 owns graph_version
//     (rank-row keying).
//   - D-06 (single bump site): the ONLY code path that bumps graph_version
//     is Engine.ApplyRepair → tx.BumpGraphVersion. No score-only writes,
//     cluster writes, or compaction passes advance the counter. Body-only
//     symbol edits short-circuit (repair.IsEmpty()) without acquiring the
//     mutex or opening a tx.
//   - D-07 (read-time score_status): score_status ∈
//     {exact, approximate, stale, missing} is computed at READ time by
//     comparing a row's stored graph_version with the workspace's current
//     graph_version. "missing" is the no-row case; "approximate" is set
//     ONLY by the full-recompute scheduler when a mid-run preempt forces a
//     marker write (P03 D-10).
//   - D-14 (storage-side merge): comment-edge / LSP-edge merges run at the
//     SQL boundary inside store.OverlayTx.UpsertEdgesWithMerge, not in Go.
//     LSP edges always win; comment edges that get refuted (different dst
//     for the same src+edge_kind) are deleted, not preserved at lower
//     confidence.
//   - T4 (per-workspace mutex re-use): ApplyRepair acquires the SAME
//     mutex BeginOverlayTx uses (store.LockOverlayWorkspace). NO second
//     mutex map lives in this package.
//
// Layout:
//
//   - repair.go: GraphRepair / FileFactDiff / SymbolDiff / GraphEdge value
//     types + ComputeGraphRepair (D-06 decision policy: which symbol /
//     edge changes count as "graph-changing").
//   - apply_repair.go: Engine + RepairStore narrow seam + ApplyRepair single
//     bump-site implementation. The notify channel publishes
//     GraphVersionAdvance values to the Phase 62 P03 RankScheduler.
//   - status.go: ScoreStatus closed-enum + computeScoreStatus read-time
//     decision (D-07).
//   - ranker.go: Ranker interface + production engineRanker that joins
//     CurrentGraphVersion and the per-projection ProjectionStatus reader.
//   - metrics.go: MetricsSink narrow interface declaring the bounded-label
//     helpers Engine + Scheduler call. Production binds to *obs.Metrics.
//   - trace.go: per-operation otel span helpers ("semantic.graph.apply_repair").
//
// Cross-package fixpoint chains terminate at the last in-package
// confidence reached; D-13 keeps cross-package resolution OUT of Phase 62
// and Phase 64 picks it up.
package graph
