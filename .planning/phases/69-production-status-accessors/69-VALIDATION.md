---
phase: 69
slug: production-status-accessors
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-14
---

# Phase 69 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — `go.mod` at repo root |
| **Quick run command** | `go test ./internal/semantic/cluster/... ./internal/semantic/retrieval/... ./internal/semantic/compact/... ./internal/daemon/... ./internal/skill/semantic/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60s quick / ~5min full |

---

## Sampling Rate

- **After every task commit:** Run quick command on the touched package
- **After every plan wave:** Run quick command across all Phase 69 packages
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 90 seconds for the quick command set

---

## Per-Task Verification Map

> Per-task rows are filled by the planner — every task in PLAN.md must have a row here with a concrete `automated_command` or a Wave 0 dependency.

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| TBD     | TBD  | TBD  | STATUS-01   | N/A             | unit      | TBD               | ❌ W0       | ⬜ pending |
| TBD     | TBD  | TBD  | STATUS-02   | N/A             | unit      | TBD               | ❌ W0       | ⬜ pending |
| TBD     | TBD  | TBD  | STATUS-03   | N/A             | integration | TBD             | ❌ W0       | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/semantic/cluster/persist_status_test.go` — stubs for `ClusterStatusForGraphVersion` (STATUS-01) including race-clean read-path test mirroring `TestCountStaleScoreRows_LockFree_RaceSafe`
- [ ] `internal/semantic/retrieval/bleve_meta_test.go` — stubs for `corpus_version` / `last_compact_at` / `indexed_files` derivations (STATUS-02)
- [ ] `internal/skill/semantic/envelope_test.go` — stubs verifying additive `ClusterStatus.{ComputedAt,MemberCount}` and new nested `RetrievalStatus` JSON shape (STATUS-01, STATUS-02)
- [ ] `internal/mcp/integ/semantic_status_e2e_test.go` (or extension of `64-07` fixture) — STATUS-03 E2E asserting non-placeholder values on populated workspace

*If existing files cover these, link them and mark ✅.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|

*None — all Phase 69 success criteria have automated verification (unit + integration).*

---

## D-09 Read-Path Invariant Check

The new `*Store.ClusterStatusForGraphVersion` accessor MUST NOT call `Begin/Commit/Abort/Write`. Enforce via:

- [ ] `go vet ./...` (covers project-local `vet-nokernel2semantic` analyzer)
- [ ] Read-path grep audit pattern (Phase 64 reference): `! grep -nE 'Begin|Commit|Abort|Exec.*INSERT|Exec.*UPDATE' internal/semantic/cluster/persist.go | grep ClusterStatusForGraphVersion`
- [ ] Race test mirrors `TestCountStaleScoreRows_LockFree_RaceSafe` (effective_graph_test.go:242-293) — concurrent reads under `go test -race`

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING test files listed above
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
