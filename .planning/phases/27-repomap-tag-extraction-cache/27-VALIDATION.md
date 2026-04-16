---
phase: 27
slug: repomap-tag-extraction-cache
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-16
---

# Phase 27 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `go test ./internal/repomap/... ./internal/treesitter/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/repomap/... ./internal/treesitter/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 27-01-01 | 01 | 1 | RMAP-01 | — | N/A | unit | `go test ./internal/treesitter/...` | ❌ W0 | ⬜ pending |
| 27-01-02 | 01 | 1 | RMAP-01 | — | N/A | unit | `go test ./internal/repomap/...` | ❌ W0 | ⬜ pending |
| 27-02-01 | 02 | 1 | RMAP-02 | — | N/A | unit | `go test ./internal/repomap/...` | ❌ W0 | ⬜ pending |
| 27-03-01 | 03 | 2 | RMAP-03 | — | N/A | unit | `go test ./internal/repomap/...` | ❌ W0 | ⬜ pending |
| 27-04-01 | 04 | 2 | RMAP-09 | — | N/A | unit | `go test ./internal/repomap/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/treesitter/registry_test.go` — stubs for shared grammar registry
- [ ] `internal/repomap/extractor_test.go` — stubs for tag extraction
- [ ] `internal/repomap/cache_test.go` — stubs for SQLite cache
- [ ] `internal/repomap/elision_test.go` — stubs for scope-aware elision

*Existing go test infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Daemon restart preserves cache | RMAP-03 | Requires daemon lifecycle | Start daemon, extract tags, restart, verify tags still cached |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
