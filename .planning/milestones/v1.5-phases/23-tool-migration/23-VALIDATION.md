---
phase: 23
slug: tool-migration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-15
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `go test ./internal/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/...`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 23-01-01 | 01 | 1 | MIG-01 | — | N/A | unit | `go test ./internal/kernel/symbols/...` | ✅ | ⬜ pending |
| 23-02-01 | 02 | 1 | MIG-02 | — | N/A | unit | `go test ./internal/kernel/edit/...` | ✅ | ⬜ pending |
| 23-03-01 | 03 | 1 | MIG-03 | — | N/A | unit | `go test ./internal/kernel/fileops/...` | ✅ | ⬜ pending |
| 23-04-01 | 04 | 1 | MIG-04 | — | N/A | unit | `go test ./internal/kernel/diag/...` | ✅ | ⬜ pending |
| 23-05-01 | 05 | 2 | MIG-05 | — | N/A | unit | `go test ./internal/skill/memory/...` | ✅ | ⬜ pending |
| 23-06-01 | 06 | 2 | MIG-06 | — | N/A | unit | `go test ./internal/skill/workflow/...` | ✅ | ⬜ pending |
| 23-07-01 | 07 | 2 | MIG-07 | — | N/A | unit | `go test ./internal/profile/...` | ✅ | ⬜ pending |
| 23-08-01 | 08 | 3 | MIG-08 | — | N/A | unit | `go test ./internal/mcp/...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
