---
phase: 38
slug: progressive-descriptions-lazy-init
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-22
---

# Phase 38 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test toolchain |
| **Quick run command** | `go test ./internal/mcp/... ./internal/kernel/... -count=1 -short` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/mcp/... ./internal/kernel/... -count=1 -short`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 38-01-01 | 01 | 1 | DESC-01 | — | N/A | unit | `go test ./internal/mcp/... -run TestBriefDescription -count=1` | ❌ W0 | ⬜ pending |
| 38-01-02 | 01 | 1 | DESC-02 | — | N/A | unit | `go test ./internal/kernel/... -run TestGetToolHelp -count=1` | ❌ W0 | ⬜ pending |
| 38-01-03 | 01 | 1 | DESC-03 | — | N/A | golden | `go test ./test/bench/... -run TestToolDescriptions -count=1` | ❌ W0 | ⬜ pending |
| 38-02-01 | 02 | 2 | LAZY-01 | — | Lazy init transparent to tool handlers | integration | `go test ./internal/mcp/... -run TestLazyInit -count=1` | ❌ W0 | ⬜ pending |
| 38-02-02 | 02 | 2 | LAZY-02 | — | Concurrent init serialized via sync.Once | unit | `go test ./internal/mcp/... -run TestLazyInitConcurrency -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Test stubs for DESC-01 brief description validation
- [ ] Test stubs for DESC-02 get_tool_help comprehensive output
- [ ] Test stubs for DESC-03 golden-file description regression
- [ ] Test stubs for LAZY-01 lazy workspace activation
- [ ] Test stubs for LAZY-02 concurrent init safety

*Existing infrastructure covers test framework — only test files need creation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Agent tool selection quality with brief descriptions | DESC-03 | Requires LLM behavioral test | Run oracle judge test with brief descriptions and verify tool selection accuracy matches baseline |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
