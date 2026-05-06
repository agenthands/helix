---
phase: 62
slug: graph-engine-ranking-type-resolution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-06
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
| 62-01-01 | 01 | 1 | GRAPH-01 | — | Deterministic PR (sort-before-iterate) | unit (TDD) | `go test ./internal/graph/ -run TestPageRank_Deterministic -count=10` | ❌ W0 | ⬜ pending |
| 62-01-02 | 01 | 1 | GRAPH-01 | — | Hex-digest byte-equal across runs | unit (TDD) | `go test ./internal/graph/ -run TestPageRank_HexDigest -count=1` | ❌ W0 | ⬜ pending |
| 62-01-03 | 01 | 1 | GRAPH-02 | — | Personalized PR seed weighting | unit (TDD) | `go test ./internal/graph/ -run TestPageRank_Personalized -count=1` | ❌ W0 | ⬜ pending |
| 62-01-04 | 01 | 1 | GRAPH-01 | — | Repomap migration; existing FileGraph API preserved | unit | `go test ./internal/repomap/ -count=1` | ✅ | ⬜ pending |
| 62-02-01 | 02 | 2 | GRAPH-05 | — | graph_version monotonic non-decreasing | unit (TDD) | `go test ./internal/semantic/graph/ -run TestApplyRepair_VersionMonotonic` | ❌ W0 | ⬜ pending |
| 62-02-02 | 02 | 2 | GRAPH-05 | — | Bump only on edge change OR symbol stable_key/sig/exported/kind | integration | `go test ./internal/semantic/graph/ -run TestApplyRepair_BodyOnlyNoBump` | ❌ W0 | ⬜ pending |
| 62-02-03 | 02 | 2 | GRAPH-03 | — | score_status enum at read time | unit (TDD) | `go test ./internal/semantic/graph/ -run TestScoreStatus_ReadTime` | ❌ W0 | ⬜ pending |
| 62-02-04 | 02 | 2 | GRAPH-05 | — | UpsertGraphScores tx helper; current_epoch advances; graph_version does not | integration | `go test ./internal/semantic/store/ -run TestUpsertGraphScores -count=1` | ✅ | ⬜ pending |
| 62-03-01 | 03 | 3 | GRAPH-04 | — | RankScheduler closed-channel start/stop | integration | `go test ./internal/semantic/graph/ -run TestRankScheduler_Lifecycle -race` | ❌ W0 | ⬜ pending |
| 62-03-02 | 03 | 3 | GRAPH-04 | — | 1-hop frontier deterministic | unit (TDD) | `go test ./internal/semantic/graph/ -run TestFrontier_OneHop` | ❌ W0 | ⬜ pending |
| 62-03-03 | 03 | 3 | GRAPH-04 | — | >5000 frontier → all-stale + full recompute | integration | `go test ./internal/semantic/graph/ -run TestFrontier_Overflow` | ❌ W0 | ⬜ pending |
| 62-03-04 | 03 | 3 | GRAPH-04 | — | Full-recompute preempted → approximate | integration | `go test ./internal/semantic/graph/ -run TestFullRecompute_Preempted` | ❌ W0 | ⬜ pending |
| 62-03-05 | 03 | 3 | GRAPH-05 | — | WriteInvalidations consumer turns stub rows into GraphRepair | integration | `go test ./internal/semantic/graph/ -run TestInvalidations_Consumer` | ❌ W0 | ⬜ pending |
| 62-04-01 | 04 | 4 | GRAPH-06 | — | Weak-component clustering deterministic | unit (TDD) | `go test ./internal/semantic/cluster/ -run TestWeakComponent_Deterministic -count=10` | ❌ W0 | ⬜ pending |
| 62-04-02 | 04 | 4 | GRAPH-06 | — | Cluster (cluster_id, members) hex-digest byte-equal | unit (TDD) | `go test ./internal/semantic/cluster/ -run TestWeakComponent_HexDigest` | ❌ W0 | ⬜ pending |
| 62-04-03 | 04 | 4 | GRAPH-06 | — | Cluster persistence via UpsertClusters/UpsertClusterMembers | integration | `go test ./internal/semantic/store/ -run TestUpsertClusters` | ❌ W0 | ⬜ pending |
| 62-05-01 | 05 | 5 | TYPES-01 | — | 7-tier confidence ladder Go | integration | `go test ./internal/semantic/types/golang/ -run TestResolver_Ladder` | ❌ W0 | ⬜ pending |
| 62-05-02 | 05 | 5 | TYPES-01 | — | 7-tier ladder TypeScript | integration | `go test ./internal/semantic/types/typescript/ -run TestResolver_Ladder` | ❌ W0 | ⬜ pending |
| 62-05-03 | 05 | 5 | TYPES-01 | — | 7-tier ladder Python | integration | `go test ./internal/semantic/types/python/ -run TestResolver_Ladder` | ❌ W0 | ⬜ pending |
| 62-05-04 | 05 | 5 | TYPES-02 | — | Access chain depth 8 + fixpoint depth 8 | unit (TDD) | `go test ./internal/semantic/types/ -run TestFixpoint_Bounded` | ❌ W0 | ⬜ pending |
| 62-05-05 | 05 | 5 | TYPES-02 | — | Chain length 9 → unresolved on last hop | integration | `go test ./internal/semantic/types/ -run TestChain_DepthExceeded` | ❌ W0 | ⬜ pending |
| 62-05-06 | 05 | 5 | TYPES-03 | — | Comment edge ≤0.60; in-place LSP upgrade to 1.0 | integration | `go test ./internal/semantic/types/ -run TestCommentEdge_TwoPhaseMerge` | ❌ W0 | ⬜ pending |
| 62-05-07 | 05 | 5 | TYPES-03 | — | Comment refutation (LSP says different dst) → delete; no preserve | integration | `go test ./internal/semantic/types/ -run TestCommentEdge_Refutation` | ❌ W0 | ⬜ pending |
| 62-05-08 | 05 | 5 | TYPES-04 | — | Non-converged → unresolved (never validated) | unit (TDD) | `go test ./internal/semantic/types/ -run TestFixpoint_NonConverged` | ❌ W0 | ⬜ pending |
| 62-05-09 | 05 | 5 | TYPES-01 | — | Java stub short-circuits on LSP-confirmed; PHP/Ruby always 0.20 | unit (TDD) | `go test ./internal/semantic/types/java/ ./internal/semantic/types/php/ ./internal/semantic/types/ruby/ -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/graph/pagerank.go` + `internal/graph/pagerank_test.go` — algorithm + determinism unit tests (GRAPH-01, GRAPH-02)
- [ ] `internal/graph/testdata/pagerank/` — golden hex digests for byte-equal assertion
- [ ] `internal/semantic/graph/repair.go` + `_test.go` — ApplyRepair + version bump rule (GRAPH-05)
- [ ] `internal/semantic/graph/scheduler.go` + `_test.go` — RankScheduler lifecycle (GRAPH-04)
- [ ] `internal/semantic/graph/frontier.go` + `_test.go` — 1-hop frontier (GRAPH-04)
- [ ] `internal/semantic/graph/testdata/repair/` — overlay-tx fixtures
- [ ] `internal/semantic/cluster/weak.go` + `_test.go` — weak-component algorithm (GRAPH-06)
- [ ] `internal/semantic/types/{resolver,chain,fixpoint,ladder,emit}.go` + `_test.go` — shared core (TYPES-01..04)
- [ ] `internal/semantic/types/{golang,typescript,python}/resolver.go` + `_test.go`
- [ ] `internal/semantic/types/{golang,typescript,python}/testdata/` — chain fixtures hitting every confidence tier
- [ ] `internal/semantic/types/{java,php,ruby}/stub.go` + `_test.go`
- [ ] `internal/semantic/store/overlay.go` extension — `UpsertGraphScores`, `UpsertClusters`, `UpsertClusterMembers`, `UpsertEdgesWithMerge`
- [ ] No new framework install — `go test` already in use.

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

(Optional — operator may sanity-check `helix_semantic_graph_pagerank_duration_seconds` and `helix_semantic_graph_score_status_total` metrics labels via `curl <metrics-endpoint>` after a smoke run, but the bounded-label discipline is enforced at registration time by `internal/obs/`.)

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s for quick / 300s for full
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
