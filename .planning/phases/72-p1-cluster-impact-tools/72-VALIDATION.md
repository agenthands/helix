---
phase: 72
slug: p1-cluster-impact-tools
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-17
---

# Phase 72 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — go test built-in |
| **Quick run command** | `go test ./internal/skill/semantic/... -race -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -race -count=1` |
| **Estimated runtime** | ~120 seconds (full); ~20 seconds (semantic package only) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/skill/semantic/... -race -count=1`
- **After every plan wave:** Run `go vet ./... && go test ./... -race -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds for per-package quick run

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 72-01-01 | 01 | 1 | P1TOOL-03 | — | read+ mode tier enforced before any cluster read | unit | `go test ./internal/skill/semantic/ -run TestGetClusterMap -race -count=1` | ❌ W0 | ⬜ pending |
| 72-02-01 | 02 | 1 | P1TOOL-04 | — | stale_cluster_id structured error on graph_version mismatch | unit | `go test ./internal/skill/semantic/ -run TestExplainCluster -race -count=1` | ❌ W0 | ⬜ pending |
| 72-03-01 | 03 | 2 | P1TOOL-05 | — | review+ mode tier enforced before any impact traversal | unit | `go test ./internal/skill/semantic/ -run TestGetChangeImpactGraph -race -count=1` | ❌ W0 | ⬜ pending |
| 72-04-01 | 04 | 3 | P1TOOL-03,04,05 | — | cross-tool envelope agreement (same graph_version across all three tools) | integration | `go test ./internal/skill/semantic/ -run TestP1ClusterImpactIntegration -race -count=1` | ❌ W0 | ⬜ pending |
| 72-05-01 | 05 | 3 | P1TOOL-03,04,05 | — | static read-only gate: no snapshot-write canary triggers on read+ paths | static | `go test ./internal/skill/semantic/ -run TestReadOnlyGate -race -count=1` | ✅ (Phase 71) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/skill/semantic/tools_cluster_map_test.go` — unit tests for get_cluster_map handler
- [ ] `internal/skill/semantic/tools_explain_cluster_test.go` — unit tests for explain_cluster handler (incl. stale_cluster_id)
- [ ] `internal/skill/semantic/tools_change_impact_test.go` — unit tests for get_change_impact_graph handler
- [ ] `internal/skill/semantic/tools_p1_cluster_impact_integration_test.go` — cross-tool integration test
- [ ] Fixture: small in-memory cluster store with 3 weak components, ~10 nodes total, deterministic PageRank

*Existing Phase 71 fixtures (`internal/skill/semantic/testutil_*` / equivalent) extended, not duplicated.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| MCP tools/list surfacing under `read+` and `review+` profiles | P1TOOL-03/04/05 | Requires live daemon + forwarder + actual MCP client roundtrip | `helix daemon &` → `helix setup claude-code` → call `tools/list` via forwarder → assert `get_cluster_map`, `explain_cluster` present under `read+`; `get_change_impact_graph` present under `review+` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
