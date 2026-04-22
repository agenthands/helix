---
phase: 37
slug: smart-error-responses
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-22
---

# Phase 37 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test runner |
| **Quick run command** | `go test ./internal/mcp/... -run Suggest -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/mcp/... -run Suggest -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 37-01-01 | 01 | 1 | SERR-01 | — | Parameter name suggestions only from same tool | unit | `go test ./internal/mcp/... -run TestLevenshtein -count=1` | ❌ W0 | ⬜ pending |
| 37-01-02 | 01 | 1 | SERR-02 | — | Never redirect to different tool | unit | `go test ./internal/mcp/... -run TestSameToolOnly -count=1` | ❌ W0 | ⬜ pending |
| 37-01-03 | 01 | 1 | SERR-03 | — | Middleware wraps errors, no new kinds | unit | `go test ./internal/mcp/... -run TestMiddleware -count=1` | ❌ W0 | ⬜ pending |
| 37-02-01 | 02 | 1 | SERR-01 | — | Schema introspection builds param map | unit | `go test ./internal/mcp/... -run TestSchemaMap -count=1` | ❌ W0 | ⬜ pending |
| 37-02-02 | 02 | 1 | SERR-01 | — | Enum value suggestions for constrained fields | unit | `go test ./internal/mcp/... -run TestEnumSuggest -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/mcp/suggest_test.go` — stubs for SERR-01, SERR-02, SERR-03
- [ ] Existing test infrastructure covers all other needs

*Existing Go test infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-to-end MCP error enrichment via real agent | SERR-01 | Requires MCP client sending malformed params | Start daemon, send malformed tool call via MCP, verify suggestion in response |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
