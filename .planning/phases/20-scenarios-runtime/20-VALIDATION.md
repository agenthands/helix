---
phase: 20
slug: scenarios-runtime
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (integration build tag) |
| **Config file** | none — uses existing test/harness/ infrastructure |
| **Quick run command** | `go vet ./... && go test -tags integration -count=1 -timeout 3m ./test/oracle/scenario/...` |
| **Full suite command** | `go vet ./... && go test -tags integration -race -count=1 -timeout 10m ./test/oracle/...` |
| **Estimated runtime** | ~60 seconds (quick), ~180 seconds (full with -race) |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./... && go test -tags integration -count=1 -timeout 3m ./test/oracle/scenario/...`
- **After every plan wave:** Run `go vet ./... && go test -tags integration -race -count=1 -timeout 10m ./test/oracle/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 20-01-01 | 01 | 1 | SCEN-01 | — | N/A | integration | `go test -tags integration -run TestFixture ./test/oracle/scenario/...` | ❌ W0 | ⬜ pending |
| 20-02-01 | 02 | 1 | SCEN-02 | — | N/A | integration | `go test -tags integration -run TestScenario ./test/oracle/scenario/...` | ❌ W0 | ⬜ pending |
| 20-03-01 | 03 | 1 | SCEN-03, SCEN-04 | — | Mode restrictions enforced | integration | `go test -tags integration -run "TestPolyglot\|TestProfile" ./test/oracle/scenario/...` | ❌ W0 | ⬜ pending |
| 20-04-01 | 04 | 2 | RUNT-01 | — | Circuit breaker limits cascading failures | integration | `go test -tags integration -run TestStress ./test/oracle/scenario/...` | ❌ W0 | ⬜ pending |
| 20-05-01 | 05 | 2 | RUNT-02 | — | Clean shutdown drains work | integration | `go test -tags integration -run TestShutdown ./test/oracle/scenario/...` | ❌ W0 | ⬜ pending |
| 20-06-01 | 06 | 2 | RUNT-03 | — | Degraded mode reports honestly | integration | `go test -tags integration -run TestDegraded ./test/oracle/scenario/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements — test/harness/ provides runner, tools, fixtures, golden helpers.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
