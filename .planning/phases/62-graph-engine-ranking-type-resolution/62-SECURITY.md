---
phase: 62
slug: graph-engine-ranking-type-resolution
status: verified
threats_open: 0
asvs_level: standard
created: 2026-05-07
---

# Phase 62 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Phase 62 ships the graph engine (Engine.ApplyRepair, RankScheduler), weak-component clustering, and the per-language type-resolution ladder. All work is in-process Go; no new network endpoints, no new auth surface, no new file-system surface beyond existing overlay-store paths.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| in-process | All Phase 62 work is in-process Go (graph engine, scheduler, cluster, types). No external surface. | Caller-provided NodeIDs, edges, personalization vectors. No PII / no secret material. |
| storage→Go | All overlay writes go through `store.OverlayTx` opened via `store.BeginOverlayTx(repoID)`, which acquires the SAME per-workspace mutex Phase 60 D-04 defines (no second mutex map in any Phase 62 package). | EdgeRow, ScoreRow, ClusterSummary/MemberRow batches; `graph_version` advances. |
| comment→graph | Type resolver two-phase merge: comment-derived edges enter at confidence ≤ 0.60 and are upgraded only by LSP-validated rows; merge predicate `(src,dst,kind)` is enforced at the SQL boundary by `tx.UpsertEdgesWithMerge`, NOT in Go. | Comment doc-strings already truncated by Phase 59 extractor. |
| metric→Prometheus | Five new metric families (pagerank duration, score_status, repair_outcome, graph_version, types_resolution) emit only through helper methods that drop unknown label values against package-private closed-enum allowlists. | Bounded label cells: scope×projection, projection×status, outcome, workspace_label, language×confidence_tier. |

---

## Threat Register

### P01 — `internal/graph/` PageRank engine

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-62-01-T1 | Tampering | `pagerank.go` determinism | mitigate | `sort.Slice` over node keys before every map iteration (`pagerank.go:37`, `personalize.go:79,85`). `TestPageRank_HexDigest` (pagerank_test.go:94) gates byte-equality; `TestPageRank_Deterministic` (pagerank_test.go:72) runs `-count=10`; `TestPageRank_TieBreak` (pagerank_test.go:167) pins stable-key ordering. | closed |
| T-62-01-I1 | Information disclosure | personalization map | accept | Personalization values are caller-provided floats; engine has no logging. See accepted risk R-62-01. | closed |
| T-62-01-D1 | Denial of service | pathological graph | accept | Pure CPU; `MaxIter` cap default 100 (`options.go:39-40`); no goroutines, no I/O. See accepted risk R-62-02. | closed |

### P02 — `internal/semantic/graph/apply_repair.go` + `internal/obs/metrics.go`

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-62-02-T2 | Tampering | `graph_version` monotonicity | mitigate | Single bump site invariant: 1 production call to `tx.BumpGraphVersion` in `internal/semantic/graph/` (apply_repair.go:254). `TestApplyRepair_VersionMonotonic` (apply_repair_test.go:98) and `TestApplyRepair_SingleBumpSiteOnly` (apply_repair_test.go:141) verify D-06 invariant. | closed |
| T-62-02-T3 | Tampering | comment-edge upgrade race (early form) | mitigate | `tx.UpsertEdgesWithMerge` enforces merge predicate `(src,dst,kind)` at SQL boundary. `TestUpsertEdgesWithMerge_LSPSkipsCommentInsert` and `TestUpsertEdgesWithMerge_LSPDeletesCommentBeforeInsert` (overlay_test.go:537, 585) verify both directions. | closed |
| T-62-02-T4 | Tampering | cross-workspace state bleed | mitigate | `Engine.ApplyRepair` re-uses the per-workspace mutex via `store.LockWorkspace(repoID)` (apply_repair.go:198). NO production `sync.Map` / `map[string]*sync.Mutex` in `internal/semantic/graph/` (verified by grep on non-test files). `TestApplyRepair_HoldsMutex` (apply_repair_test.go:156) verifies. | closed |
| T-62-02-D2 | Denial of service (label cardinality) | Prometheus metrics | mitigate | All five new metric helpers validate against closed-enum allowlists (`pagerankScopes`, `pagerankProjections`, `graphScoreStatuses`, `graphRepairOutcomes`, `typesConfidenceTiers`) BEFORE `WithLabelValues` (metrics.go:742-799). `TestMetricsLabelsAllowlist` (metrics_labels_test.go:149) and `TestMetricsLabelsAllowlist_catchesDrift` (line 207) gate cardinality. | closed |
| T-62-02-I2 | Information disclosure | structured slog logs | accept | repo_id is an established label across Phase 60/61. No PII / secret material. See accepted risk R-62-03. | closed |

### P03 — `internal/semantic/graph/scheduler.go`, `frontier.go`, `full_recompute.go`

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-62-03-T1 | Tampering | frontier sort-before-iterate | mitigate | `sortedNodeIDs` helper centralized in `util.go:18`; frontier uses it for both adjacency directions (`frontier.go:53,59,66`). Grep confirms ZERO bare `for k := range adjacency*` in `frontier.go`. `TestComputeFrontier_DeterministicAcrossRuns` (frontier_test.go:83) runs 100 trials. | closed |
| T-62-03-T4 | Tampering | cross-workspace state bleed | mitigate | RankScheduler delegates locking to `Store.LockWorkspace`/`LockOverlayWorkspace` (same Phase-60 mutex). NO production `sync.Map` / second mutex map in `internal/semantic/graph/`. `TestRankScheduler_PerWorkspaceIsolation` (scheduler_test.go:241) exercises 3 concurrent workspaces. | closed |
| T-62-03-D1 | Denial of service (channel saturation) | Notify under burst | mitigate | Non-blocking send with `default:` drop arm + `SemanticGraphRepairInc("error")` (scheduler.go:114-118). `TestRankScheduler_NoBlock_ChannelDropOnFull` (scheduler_test.go:215) bursts 100 sends into a buffer-of-4 channel. | closed |
| T-62-03-D2 | Denial of service (rank drift mid-edit) | stale rows during heavy bursts | accept | D-08/D-09 explicit choice: readers see `score_status=stale` and choose to wait or proceed. See accepted risk R-62-04. | closed |
| T-62-03-D3 | Denial of service (label cardinality) | repair_outcome metric | mitigate | Closed-enum allowlist `graphRepairOutcomes` = {applied, frontier_overflow, preempted, error, stub_no_data} (metrics.go:712); guard at metrics.go:773 drops unknown values BEFORE `WithLabelValues`. | closed |
| T-62-03-T5 | Tampering | pre-empted full recompute | mitigate | RunFullRecompute marks rows `score_status=approximate` atomically inside the same tx when graph_version advances mid-run (full_recompute.go:113-156). `TestRunFullRecompute_PreemptedMarksApproximate` (full_recompute_test.go:185) verifies the atomic contract. | closed |

### P04 — `internal/semantic/cluster/`

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-62-04-T1 | Tampering | weak-component determinism | mitigate | Sorted union-find with smaller-ID-becomes-root tiebreak; sorted iteration of nodes/edges/parent map (weak.go:45,94,102,121). `TestWeakComponents_HexDigest` (weak_test.go:190) + `TestWeakComponents_DeterministicAcrossRuns` (weak_test.go:219) prove byte-equality with three pinned sha256 goldens. | closed |
| T-62-04-T4 | Tampering | cross-workspace cluster bleed | mitigate | `RunClusterDetection` writes via `store.BeginOverlayTx(repoID)` (persist.go:74) — same per-workspace mutex Phase 60 protects. NO `sync.Map` / second mutex map in `internal/semantic/cluster/` (grep returns empty). | closed |
| T-62-04-I1 | Information disclosure | cluster member NodeIDs | accept | NodeIDs are existing stable-ID hashes of file:span:kind tuples (Phase 59); no PII exposure. See accepted risk R-62-05. | closed |
| T-62-04-D1 | Denial of service (large graphs) | union-find on all nodes | accept | O(N + E·α(N)) with path compression + union-by-rank; sub-second for typical repos. See accepted risk R-62-06. | closed |

### P05 — `internal/semantic/types/`

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-62-05-T3 | Tampering | comment-edge upgrade race (full form) | mitigate | Merge predicate enforced at SQL boundary by `tx.UpsertEdgesWithMerge`. `EmitEdges` runs every row through `CapCommentConfidence` (emit.go:41, ladder.go:22) — comment edges NEVER exceed 0.60 in-flight. `TestEmitEdges_CommentNeverExceeds060` (emit_test.go:64) verifies. | closed |
| T-62-05-T6 | Tampering | non-converged chain marked validated | mitigate | `FixpointResolve` forces `ValidationState="unresolved"` on any `Resolved=false` post-fixpoint (fixpoint.go:60-61). `EmitEdges` defensive: any unresolved response → state=unresolved (emit.go:44-46). `TestFixpoint_NonConverged` (fixpoint_test.go:31) and `TestEmitEdges_UnresolvedAlwaysHasState` (emit_test.go:83) verify. | closed |
| T-62-05-T7 | Tampering | cross-package chain falsely "validated" | mitigate | Per-language `SamePackage` check in golang/python/typescript resolvers (golang/resolver.go:138, python/resolver.go:101, typescript/resolver.go:102). java/php/ruby are stubs that emit `Resolved=false / ValidationState=unresolved` only (except java's LSP-confirmed short-circuit, which is unconditional `validated`). `TestGoResolver_CrossPackageStopsAtLastInPackage` (golang/resolver_test.go:115) verifies. | closed |
| T-62-05-T4 | Tampering | cross-workspace state bleed | mitigate | All edge writes go through `tx.UpsertEdgesWithMerge` opened via `store.BeginOverlayTx(repoID)`. Resolvers are stateless functions over (req, store). NO `sync.Map` / second mutex map in `internal/semantic/types/` (grep returns empty). | closed |
| T-62-05-D2 | Denial of service (label cardinality) | types_resolution metric | mitigate | `SemanticTypesResolutionInc` validates `language` against `allowedExtractionLanguages` and `confidenceTier` against `typesConfidenceTiers` (7 tiers) before `WithLabelValues` (metrics.go:792-799). Unknown language coerced to "other"; unknown tier drops the emission. | closed |
| T-62-05-D3 | Denial of service (regex pathological inputs) | comment parsers | accept | Hand-rolled regexes are simple (`@type {(\w+)}`, `# type: (\w+)`); no nested quantifiers. Inputs bounded by Phase 59 extractor (truncated at extraction time). See accepted risk R-62-07. | closed |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-62-01 | T-62-01-I1 | Personalization values are caller-provided floats; no PII / secret material; engine has no logging. | gsd-security-auditor | 2026-05-07 |
| R-62-02 | T-62-01-D1 | PageRank engine is pure CPU; caller bounds inputs; `MaxIter` cap (default 100) prevents unbounded iteration; no goroutines, no I/O. | gsd-security-auditor | 2026-05-07 |
| R-62-03 | T-62-02-I2 | Logs include `repo_id` (already an established label across Phase 60/61). No PII / secret material. Existing log discipline. | gsd-security-auditor | 2026-05-07 |
| R-62-04 | T-62-03-D2 | D-08/D-09 explicit design choice: stale rows during heavy bursts are accepted; readers see `score_status=stale` and choose to wait or proceed (no-block contract). Re-evaluate if drift latency becomes user-visible. | gsd-security-auditor | 2026-05-07 |
| R-62-05 | T-62-04-I1 | Cluster member NodeIDs are existing hashes of file:span:kind tuples (Phase 59 stable IDs); no PII exposure. Existing observability discipline. | gsd-security-auditor | 2026-05-07 |
| R-62-06 | T-62-04-D1 | Algorithm is O(N + E·α(N)) with path compression + union-by-rank. Sub-second for typical repos (<100k nodes). Add early-exit threshold if future workloads stress. | gsd-security-auditor | 2026-05-07 |
| R-62-07 | T-62-05-D3 | Hand-rolled regexes (`@type {(\w+)}`, `# type: (\w+)`) have no nested quantifiers. Inputs are bounded by Phase 59-extracted doc-comment fields (truncated at extraction time). | gsd-security-auditor | 2026-05-07 |

---

## Unregistered Flags (from SUMMARY.md `## Threat Flags`)

None. Per-plan SUMMARY threat flag sections (P01, P02, P03, P06, P07) explicitly state no new attack surface beyond the threat register. P04, P05, P08, P09 SUMMARYs lack a dedicated `## Threat Flags` section but their plans either contain the threat register (P04, P05) or are pure refactor / docs work with no new surface (P08 = gap-closure docs only, P09 = ROADMAP flips only — confirmed by reading SUMMARYs).

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-05-07 | 24 | 24 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log (7 entries)
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-05-07
