---
phase: 18
slug: harness-extraction-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + testify v1.11.1 |
| **Config file** | none — uses build tags |
| **Quick run command** | `go test ./test/harness/...` |
| **Full suite command** | `go test ./... && go test -tags integration ./test/...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./test/harness/...`
- **After every plan wave:** Run `go test ./... && go test -tags integration ./test/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 1 | FOUND-01 | — | N/A | compilation | `go build ./test/harness/...` | ❌ W0 | ⬜ pending |
| 18-01-02 | 01 | 1 | FOUND-01 | — | N/A | unit | `go test ./test/harness/...` | ❌ W0 | ⬜ pending |
| 18-02-01 | 02 | 1 | FOUND-02 | — | N/A | integration | `go test -tags integration ./test/oracle/protocol/...` | ❌ W0 | ⬜ pending |
| 18-02-02 | 02 | 1 | FOUND-02 | — | N/A | compilation | `go test -run=^$ ./test/oracle/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `test/harness/` package — extracted from test/integration/ with exported symbols
- [ ] `test/oracle/protocol/` — stub package with build tags to verify import and tag taxonomy

*Existing test/integration/ infrastructure stays frozen — no modifications needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `go test ./...` skips integration tests | FOUND-02 | Requires running without tags to confirm skip | Run `go test ./...` and verify no integration tests execute |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
