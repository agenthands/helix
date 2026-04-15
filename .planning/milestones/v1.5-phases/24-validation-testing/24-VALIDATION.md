---
phase: 24
slug: validation-testing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-15
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `go test ./internal/errors/... ./internal/kernel/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/errors/... ./internal/kernel/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 24-01-01 | 01 | 1 | VAL-01 | — | InvalidArgs returned for missing/invalid params | unit | `go test ./internal/kernel/...` | ❌ W0 | ⬜ pending |
| 24-02-01 | 02 | 2 | VAL-02 | — | Error Kind assertions replace string matching | integration | `go test ./test/integration/...` | ✅ | ⬜ pending |
| 24-03-01 | 03 | 2 | VAL-03 | — | Golden files detect error structure regressions | contract | `go test ./test/oracle/contract/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `test/integration/errors_test.go` — upgrade errCase struct with expectedKind field
- [ ] `test/oracle/contract/testdata/golden/errors/` — create directory for error golden files

*Existing infrastructure covers most phase requirements. Wave 0 extends existing test harnesses.*

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
