---
phase: 66
slug: agent-guardrails
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-09
---

# Phase 66 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./internal/guardrails/... ./internal/mcp/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30 seconds (quick), ~120 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run quick command for the package(s) modified
- **After every plan wave:** Run `go test ./... -race`
- **Before `/gsd-verify-work`:** Full suite must be green under `-race`
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

> Populated during Wave 0 of execution. Plans must emit task IDs in the form
> `66-{plan}-{task}` and reference them here. The planner is instructed to
> wire `<automated>` verify commands into every task; Wave 0 fills in any
> TBD-NN placeholders below.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD-01 | 01 | 0 | GUARD-01..07 | — | Wave 0 test scaffolding | unit | `go test ./internal/guardrails/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/guardrails/receipt_test.go` — receipt store TTL/LRU/janitor stubs
- [ ] `internal/guardrails/scope_test.go` — scope-match STRICT stubs (per CONTEXT.md D-08/D-09)
- [ ] `internal/guardrails/enforcement_test.go` — 5-layer precedence resolution stubs (per CONTEXT.md D-22)
- [ ] `internal/guardrails/predicates_test.go` — per-rule predicate stubs (G-001..G-005)
- [ ] `internal/mcp/middleware_test.go` — LIFO order regression assertion stub (5-step chain)
- [ ] `internal/mcp/guardrail_middleware_test.go` — middleware install + receipt-lookup stubs
- [ ] `internal/semantic/integ/visibility_test.go` — `SemanticLookup.Visibility()` per-language stubs (OI-02)
- [ ] `internal/kernel/help/topics_test.go` — TopicRegistry exact-match stubs (OI-04)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Context-truncation E2E proves ID-only forwarding survives compaction | GUARD-02, success criterion #2 | Requires forwarder→daemon round-trip with simulated context wipe | Wired as integration test using real forwarder + daemon subprocess; documented in PLAN as `type: integration` not manual |

*If integration test in plan covers this: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
