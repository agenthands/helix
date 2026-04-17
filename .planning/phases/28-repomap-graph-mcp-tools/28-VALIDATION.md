---
phase: 28
slug: repomap-graph-mcp-tools
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-17
---

# Phase 28 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none (stdlib, `go test ./...`) |
| **Quick run command** | `go test ./internal/repomap/... ./internal/skill/repomap/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/repomap/... ./internal/skill/repomap/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1 && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 28-01-01 | 01 | 1 | RMAP-04 | T-28-01 | Path traversal check on file paths | unit | `go test ./internal/repomap/ -run TestBuildGraph -count=1` | ❌ W0 | ⬜ pending |
| 28-02-01 | 02 | 1 | RMAP-05 | — | N/A | unit | `go test ./internal/repomap/ -run TestPageRank -count=1` | ❌ W0 | ⬜ pending |
| 28-03-01 | 03 | 2 | RMAP-10 | T-28-02 | Token budget cap enforced | unit | `go test ./internal/repomap/ -run TestBudgetFitting -count=1` | ❌ W0 | ⬜ pending |
| 28-04-01 | 04 | 2 | RMAP-06 | T-28-01 | Workspace-scoped paths only | unit+integration | `go test ./internal/skill/repomap/ -run TestGetRepoMap -count=1` | ❌ W0 | ⬜ pending |
| 28-04-02 | 04 | 2 | RMAP-07 | — | N/A | unit+integration | `go test ./internal/skill/repomap/ -run TestGetContext -count=1` | ❌ W0 | ⬜ pending |
| 28-05-01 | 05 | 3 | RMAP-08 | — | N/A | unit | `go test ./internal/repomap/ -run TestEnrichFromLSP -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/repomap/graph_test.go` — stubs for RMAP-04 (graph building from mock tags)
- [ ] `internal/repomap/pagerank_test.go` — stubs for RMAP-05 (convergence, personalization, dangling nodes)
- [ ] `internal/repomap/render_test.go` — stubs for RMAP-10 (binary search budget, tree formatting)
- [ ] `internal/skill/repomap/skill_test.go` — stubs for RMAP-06, RMAP-07 (tool dispatch, parameter validation)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| LSP enrichment on warm sessions | RMAP-08 | Requires live LSP server with warm session | Start daemon, open Go project, call get_repo_map, verify LSP edges present in graph |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
