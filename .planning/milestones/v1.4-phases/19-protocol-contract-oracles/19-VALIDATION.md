---
phase: 19
slug: protocol-contract-oracles
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-11
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — uses build tags |
| **Quick run command** | `go test -tags integration ./test/oracle/protocol/... ./test/oracle/contract/...` |
| **Full suite command** | `go test -tags integration -count=1 -race ./test/oracle/...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -tags integration ./test/oracle/protocol/... ./test/oracle/contract/...`
- **After every plan wave:** Run `go test -tags integration -count=1 -race ./test/oracle/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | PROTO-01 | — | N/A | integration | `go test -tags integration -run TestHandshake ./test/oracle/protocol/...` | ❌ W0 | ⬜ pending |
| 19-01-02 | 01 | 1 | PROTO-02 | — | N/A | integration | `go test -tags integration -run TestToolsList ./test/oracle/protocol/...` | ❌ W0 | ⬜ pending |
| 19-01-03 | 01 | 1 | PROTO-03 | — | N/A | integration | `go test -tags integration -run TestSessionIsolation ./test/oracle/protocol/...` | ❌ W0 | ⬜ pending |
| 19-01-04 | 01 | 1 | PROTO-04 | — | N/A | integration | `go test -tags integration -run TestReconnect ./test/oracle/protocol/...` | ❌ W0 | ⬜ pending |
| 19-02-01 | 02 | 2 | CONT-01 | — | N/A | integration | `go test -tags integration -run TestGolden ./test/oracle/contract/...` | ❌ W0 | ⬜ pending |
| 19-02-02 | 02 | 2 | CONT-02 | — | N/A | integration | `go test -tags integration -run TestSchema ./test/oracle/contract/...` | ❌ W0 | ⬜ pending |
| 19-02-03 | 02 | 2 | CONT-03 | — | N/A | integration | `go test -tags integration -run TestError ./test/oracle/contract/...` | ❌ W0 | ⬜ pending |
| 19-02-04 | 02 | 2 | CONT-04 | — | N/A | integration | `go test -tags integration -run TestSelectability ./test/oracle/contract/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- Existing `test/harness/` infrastructure covers all phase requirements
- `test/oracle/protocol/` and `test/oracle/contract/` package stubs exist from Phase 18

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
