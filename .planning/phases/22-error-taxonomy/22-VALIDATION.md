---
phase: 22
slug: error-taxonomy
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-14
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test toolchain |
| **Quick run command** | `go test ./internal/errors/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/errors/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | ERR-01 | — | N/A | unit | `go test ./internal/errors/... -run TestKinds` | ❌ W0 | ⬜ pending |
| 22-01-02 | 01 | 1 | ERR-02 | — | N/A | unit | `go test ./internal/errors/... -run TestStructuredFields` | ❌ W0 | ⬜ pending |
| 22-01-03 | 01 | 1 | ERR-03 | — | N/A | unit | `go test ./internal/errors/... -run TestCauseChain` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/errors/errors_test.go` — test stubs for ERR-01, ERR-02, ERR-03
- [ ] Existing `go test` infrastructure covers framework needs

*Existing infrastructure covers framework requirements — Go test toolchain already in place.*

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
