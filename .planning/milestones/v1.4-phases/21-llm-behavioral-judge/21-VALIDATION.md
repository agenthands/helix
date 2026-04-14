---
phase: 21
slug: llm-behavioral-judge
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-12
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — uses build tags |
| **Quick run command** | `go test -tags=llm -run TestToolSelection -count=1 ./test/oracle/llm/...` |
| **Full suite command** | `go test -tags=llm -count=1 ./test/oracle/llm/... && go test -tags=llmjudge -count=1 ./test/oracle/judge/...` |
| **Estimated runtime** | ~60-120 seconds (LLM API calls) |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./... && go build ./...` (compile check)
- **After every plan wave:** Run full suite command (requires ANTHROPIC_API_KEY)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 21-01-01 | 01 | 1 | LLM-01 | — | N/A | integration | `go test -tags=llm -run TestToolSelection ./test/oracle/llm/...` | ❌ W0 | ⬜ pending |
| 21-01-02 | 01 | 1 | LLM-02 | — | N/A | integration | `go test -tags=llm -run TestDisambiguation ./test/oracle/llm/...` | ❌ W0 | ⬜ pending |
| 21-02-01 | 02 | 1 | LLM-03 | — | N/A | integration | `go test -tags=llm -run TestOutputInterpretation ./test/oracle/llm/...` | ❌ W0 | ⬜ pending |
| 21-03-01 | 03 | 2 | LLM-04 | — | N/A | integration | `go test -tags=llmjudge -run TestJudgeScoring ./test/oracle/judge/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go get github.com/anthropics/anthropic-sdk-go` — Add Anthropic SDK dependency
- [ ] Existing `test/oracle/llm/doc.go` and `test/oracle/judge/doc.go` stubs already in place

*Existing test harness infrastructure covers shared fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| LLM judge scores are meaningful | LLM-04 | Requires human review of rubric calibration | Review 3+ judge scores for rubric anchor accuracy |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
