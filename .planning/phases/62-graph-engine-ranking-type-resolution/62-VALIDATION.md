---
phase: 62
slug: graph-engine-ranking-type-resolution
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-05-06
audited: 2026-05-07
---

# Phase 62 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Detailed Validation Architecture lives in `62-RESEARCH.md` § "Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.21+) |
| **Config file** | none — repo-default `go test ./...` |
| **Quick run command** | `go test ./internal/graph/... ./internal/semantic/graph/... ./internal/semantic/types/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` |
| **Estimated runtime** | ~30s quick / ~3–5 min full |

---

## Sampling Rate

- **After every task commit:** Run quick command (scoped to packages the task touched)
- **After every plan wave:** Run `go test ./internal/graph/... ./internal/semantic/... ./internal/repomap/... -count=1 -race`
- **Before `/gsd-verify-work`:** Full suite must be green AND `go vet ./...` clean
- **Max feedback latency:** 30 seconds (quick); 300 seconds (full)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 62-01-01 | 01 | 1 | GRAPH-01 | — | Deterministic PR (sort-before-iterate) | unit (TDD) | `go test ./internal/graph/ -run TestPageRank_Deterministic -count=10` | ✅ | ✅ green |
| 62-01-02 | 01 | 1 | GRAPH-01 | — | Hex-digest byte-equal across runs | unit (TDD) | `go test ./internal/graph/ -run TestPageRank_HexDigest -count=1` | ✅ | ✅ green |
| 62-01-03 | 01 | 1 | GRAPH-02 | — | Personalized PR seed weighting | unit (TDD) | `go test ./internal/graph/ -run TestPageRank_Personalized -count=1` | ✅ | ✅ green |
| 62-01-04 | 01 | 1 | GRAPH-01 | — | Repomap migration; existing FileGraph API preserved | unit | `go test ./internal/repomap/ -count=1` | ✅ | ✅ green |
| 62-02-01 | 02 | 2 | GRAPH-05 | — | graph_version monotonic non-decreasing | unit (TDD) | `go test ./internal/semantic/graph/ -run TestApplyRepair_VersionMonotonic -count=1` | ✅ | ✅ green |
| 62-02-02 | 02 | 2 | GRAPH-05 | — | Bump only on edge change OR symbol stable_key/sig/exported/kind | integration | `go test ./internal/semantic/graph/ -run TestApplyRepair_BodyOnlyNoBump -count=1` | ✅ | ✅ green |
| 62-02-03 | 02 | 2 | GRAPH-03 | — | score_status enum at read time (closed enum, all 4 values) | unit (TDD) | `go test ./internal/semantic/graph/ -run 'TestComputeScoreStatus_ClosedEnum\|TestRanker_StatusCarriesPerProjectionStatus' -count=1` | ✅ | ✅ green |
| 62-02-04 | 02 | 2 | GRAPH-05 | — | UpsertGraphScores tx helper; current_epoch advances; graph_version does not | integration | `go test ./internal/semantic/store/ -run TestUpsertGraphScores -count=1` | ✅ | ✅ green |
| 62-03-01 | 03 | 3 | GRAPH-04 | — | RankScheduler closed-channel start/stop | integration | `go test ./internal/semantic/graph/ -run TestRankScheduler_Lifecycle -race -count=1` | ✅ | ✅ green |
| 62-03-02 | 03 | 3 | GRAPH-04 | — | 1-hop frontier deterministic (incoming + outgoing union) | unit (TDD) | `go test ./internal/semantic/graph/ -run 'TestComputeFrontier_OneHopUnion\|TestComputeFrontier_DeterministicAcrossRuns\|TestComputeFrontier_IncludesIncoming' -count=1` | ✅ | ✅ green |
| 62-03-03 | 03 | 3 | GRAPH-04 | — | >5000 frontier → all-stale + full recompute (overflow flag) | integration | `go test ./internal/semantic/graph/ -run TestComputeFrontier_OverflowReturnsTrue -count=1` | ✅ | ✅ green |
| 62-03-04 | 03 | 3 | GRAPH-04 | — | Full-recompute preempted → approximate | integration | `go test ./internal/semantic/graph/ -run TestRunFullRecompute_PreemptedMarksApproximate -count=1` | ✅ | ✅ green |
| 62-03-05 | 03 | 3 | GRAPH-05 | — | WriteInvalidations consumer turns stub rows into GraphRepair | integration | `go test ./internal/semantic/graph/ -run TestInvalidationsConsumer -count=1` | ✅ | ✅ green |
| 62-04-01 | 04 | 4 | GRAPH-06 | — | Weak-component clustering deterministic | unit (TDD) | `go test ./internal/semantic/cluster/ -run TestWeakComponents_DeterministicAcrossRuns -count=10` | ✅ | ✅ green |
| 62-04-02 | 04 | 4 | GRAPH-06 | — | Cluster (cluster_id, members) hex-digest byte-equal | unit (TDD) | `go test ./internal/semantic/cluster/ -run TestWeakComponents_HexDigest -count=1` | ✅ | ✅ green |
| 62-04-03 | 04 | 4 | GRAPH-06 | — | Cluster persistence via UpsertClusters/UpsertClusterMembers | integration | `go test ./internal/semantic/store/ -run 'TestUpsertClusters\|TestUpsertClusterMembers\|TestRunClusterDetection_RoundTrip' -count=1 && go test ./internal/semantic/cluster/ -run TestRunClusterDetection_RoundTrip -count=1` | ✅ | ✅ green |
| 62-05-01 | 05 | 5 | TYPES-01 | — | 7-tier confidence ladder Go (LSP/Annotation/Constructor/Assignment/GoDoc/Heuristic/Unknown) | integration | `go test ./internal/semantic/types/golang/ -run TestGoResolver_LadderTier -count=1` | ✅ | ✅ green |
| 62-05-02 | 05 | 5 | TYPES-01 | — | 7-tier ladder TypeScript | integration | `go test ./internal/semantic/types/typescript/ -run TestTSResolver_LadderTier -count=1` | ✅ | ✅ green |
| 62-05-03 | 05 | 5 | TYPES-01 | — | 7-tier ladder Python | integration | `go test ./internal/semantic/types/python/ -run TestPyResolver_LadderTier -count=1` | ✅ | ✅ green |
| 62-05-04 | 05 | 5 | TYPES-02 | — | Access chain depth 8 + fixpoint depth 8 | unit (TDD) | `go test ./internal/semantic/types/ -run 'TestFixpoint_BoundedAtMaxIter\|TestResolveAccessChain_DepthBound' -count=1` | ✅ | ✅ green |
| 62-05-05 | 05 | 5 | TYPES-02 | — | Chain length 9 → unresolved on last hop | integration | `go test ./internal/semantic/types/ -run TestResolveAccessChain_DepthBound -count=1` | ✅ | ✅ green |
| 62-05-06 | 05 | 5 | TYPES-03 | — | Comment edge ≤0.60; in-place LSP upgrade to 1.0 | integration | `go test ./internal/semantic/store/ -run 'TestUpsertEdgesWithMerge_LSPSkipsCommentInsert\|TestUpsertEdgesWithMerge_LSPDeletesCommentBeforeInsert' -count=1 && go test ./internal/semantic/types/ -run TestEmitEdges_CommentNeverExceeds060 -count=1` | ✅ | ✅ green |
| 62-05-07 | 05 | 5 | TYPES-03 | — | Comment refutation (LSP says different dst) → delete; no preserve | integration | `go test ./internal/semantic/store/ -run TestUpsertEdgesWithMerge_LSPRefutesCommentAtDifferentDst -count=1` | ✅ | ✅ green |
| 62-05-08 | 05 | 5 | TYPES-04 | — | Non-converged → unresolved (never validated) | unit (TDD) | `go test ./internal/semantic/types/ -run TestFixpoint_NonConverged -count=1` | ✅ | ✅ green |
| 62-05-09 | 05 | 5 | TYPES-01 | — | Java stub short-circuits on LSP-confirmed; PHP/Ruby always 0.20 | unit (TDD) | `go test ./internal/semantic/types/java/ ./internal/semantic/types/php/ ./internal/semantic/types/ruby/ -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/graph/pagerank.go` + `internal/graph/pagerank_test.go` — algorithm + determinism unit tests (GRAPH-01, GRAPH-02)
- [x] `internal/graph/testdata/pagerank/` — golden hex digests for byte-equal assertion
- [x] `internal/semantic/graph/apply_repair.go` + `_test.go` — ApplyRepair + version bump rule (GRAPH-05)
- [x] `internal/semantic/graph/scheduler.go` + `_test.go` — RankScheduler lifecycle (GRAPH-04)
- [x] `internal/semantic/graph/frontier.go` + `_test.go` — 1-hop frontier (GRAPH-04)
- [x] `internal/semantic/graph/{repair,status,full_recompute,ranker,invalidations_consumer}_test.go` — supporting integration tests
- [x] `internal/semantic/cluster/weak.go` + `_test.go` — weak-component algorithm (GRAPH-06)
- [x] `internal/semantic/cluster/persist_test.go` — RunClusterDetection round-trip
- [x] `internal/semantic/types/{resolver,chain,fixpoint,ladder,emit}.go` + `_test.go` — shared core (TYPES-01..04)
- [x] `internal/semantic/types/{golang,typescript,python}/resolver.go` + `_test.go`
- [x] `internal/semantic/types/{golang,typescript,python}/testdata/` — chain fixtures hitting every confidence tier
- [x] `internal/semantic/types/{java,php,ruby}/stub.go` + `_test.go`
- [x] `internal/semantic/store/overlay.go` extension — `UpsertGraphScores`, `UpsertClusters`, `UpsertClusterMembers`, `UpsertEdgesWithMerge` covered by `overlay_test.go`
- [x] No new framework install — `go test` already in use.

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

(Optional — operator may sanity-check `helix_semantic_graph_pagerank_duration_seconds` and `helix_semantic_graph_score_status_total` metrics labels via `curl <metrics-endpoint>` after a smoke run, but the bounded-label discipline is enforced at registration time by `internal/obs/`.)

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s for quick / 300s for full
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ✅ approved (audited 2026-05-07)

---

## Validation Audit 2026-05-07

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Tasks audited | 25 |
| Tests green | 25 |
| Tests red | 0 |
| Tests missing | 0 |

**Audit method:** Cross-referenced each per-task verification entry against actual test names in `internal/graph/`, `internal/semantic/{graph,cluster,store,types,types/{golang,typescript,python,java,php,ruby}}/`. All 25 tasks have passing automated coverage. Test commands updated to match the test names that landed during execution (drafted-up-front names were aspirational; actual names use the implementation's verb-noun convention, e.g. `TestComputeFrontier_OneHopUnion` instead of `TestFrontier_OneHop`). Wave 0 deliverables (algorithm files + testdata fixtures + overlay-tx extension) all exist.

**Test run:** `go test ./internal/graph/... ./internal/semantic/graph/... ./internal/semantic/cluster/... ./internal/semantic/types/... ./internal/semantic/store/... ./internal/repomap/... -count=1` — all packages PASS.
