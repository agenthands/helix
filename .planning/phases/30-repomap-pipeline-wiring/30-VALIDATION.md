---
phase: 30
slug: repomap-pipeline-wiring
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-17
---

# Phase 30 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test runner |
| **Quick run command** | `go test ./internal/skill/repomap/...` |
| **Full suite command** | `go test ./internal/skill/repomap/... ./internal/kernel/repomap/...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/skill/repomap/...`
- **After every plan wave:** Run `go test ./internal/skill/repomap/... ./internal/kernel/repomap/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 30-01-01 | 01 | 1 | RMAP-04,05,06,07 | — | N/A | integration | `go test ./internal/skill/repomap/ -run TestPopulate` | ❌ W0 | ⬜ pending |
| 30-01-02 | 01 | 1 | RMAP-08 | — | N/A | integration | `go test ./internal/skill/repomap/ -run TestEnrich` | ❌ W0 | ⬜ pending |
| 30-01-03 | 01 | 1 | RMAP-10 | — | N/A | integration | `go test ./internal/skill/repomap/ -run TestRenderer` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Existing test infrastructure covers all phase requirements
- [ ] `internal/skill/repomap/skill_test.go` — integration tests for pipeline wiring

*Existing infrastructure covers basic requirements. Integration tests needed for wired pipeline.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| get_repo_map returns ranked symbols via MCP | RMAP-06 | Requires running daemon with real workspace | Start daemon, call get_repo_map via MCP client, verify non-empty output |
| get_context returns task-relevant symbols via MCP | RMAP-07 | Requires running daemon with real workspace | Start daemon, call get_context with seed files, verify relevant output |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
