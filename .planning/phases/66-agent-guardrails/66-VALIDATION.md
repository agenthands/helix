---
phase: 66
slug: agent-guardrails
status: draft
nyquist_compliant: false
wave_0_complete: true
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
| 66-01-T1 | 01 | 1 | GUARD-03/04/05/07 | T-66-01..08 | Receipt ID + closed-enum + scope union | unit | `go test ./internal/guardrails/ -run "TestParseReceiptID\|TestNewReceiptID\|TestReceiptClassExhaustive" -count=1 -race && go vet ./internal/guardrails/...` | ❌ W0 | ⬜ pending |
| 66-01-T2 | 01 | 1 | GUARD-03/04/05 | T-66-02/04/05/06 | Receipt store TTL/janitor/LRU/graph_version | unit | `go test ./internal/guardrails/ -run TestStore -count=1 -race && go vet ./internal/guardrails/... ./internal/obs/...` | ❌ W0 | ⬜ pending |
| 66-01-T3 | 01 | 1 | GUARD-04/05/07 | T-66-01..06 | Validation predicate + 5-layer enforcement + issue sink + GuardrailViolation Kind | unit | `go test ./internal/guardrails/ -run "TestValidate\|TestResolveGuardrailEnforcement\|TestParseEnforcementLevel\|TestValidateScope_AllClasses" -count=1 -race && go test ./internal/errors/ -count=1 -race && go vet ./internal/guardrails/... ./internal/errors/...` | ❌ W0 | ⬜ pending |
| 66-02-T1 | 02 | 1 | GUARD-07 | T-66-01..06 | SemanticLookup extension + Visibility closed enum | unit | `go build ./... && go test ./internal/semantic/integ/ ./internal/guardrails/ -run "TestParseVisibility\|TestVisibility_AliasParity\|TestIsPublicLike" -count=1 -race && go vet ./...` | ❌ W0 | ⬜ pending |
| 66-02-T2 | 02 | 1 | GUARD-07 | T-66-01..06 | GuardrailsConfig D-22 extension + defaults + profile YAMLs | unit | `go build ./... && go test ./internal/semantic/ ./internal/profile/ ./internal/config/ -count=1 -race && go vet ./...` | ❌ W0 | ⬜ pending |
| 66-03-T1 | 03 | 2 | GUARD-02/07 | T-66-01..06 | G-005 catalogs + embed.FS loader | unit | `go test ./internal/guardrails/catalogs/ -count=1 -race && go vet ./internal/guardrails/catalogs/...` | ❌ W0 | ⬜ pending |
| 66-03-T2 | 03 | 2 | GUARD-02/07 | T-66-01..06 | G-001..G-004 predicates | unit | `go test ./internal/guardrails/rules/ -run "TestG001\|TestG002\|TestG003\|TestG004" -count=1 -race && go vet ./internal/guardrails/rules/...` | ❌ W0 | ⬜ pending |
| 66-03-T3 | 03 | 2 | GUARD-02/07 | T-66-01..06 | G-005 3-signal classifier + RuleEvaluator dispatch | unit | `go test ./internal/guardrails/rules/ -run "TestG005\|TestEvaluator\|TestIsSecuritySensitive" -count=1 -race && go test ./internal/guardrails/... -count=1 -race && go vet ./internal/guardrails/...` | ❌ W0 | ⬜ pending |
| 66-04-T1 | 04 | 3 | GUARD-01/03/04/05/07 | T-66-18..23 | GuardrailMiddleware shell + production deps + skill | integration | `go build ./... && go test ./internal/mcp/ -run TestGuardrailMiddleware -count=1 -race && go test ./internal/guardrails/... -count=1 -race && go vet ./...` | ❌ W0 | ⬜ pending |
| 66-04-T2 | 04 | 3 | GUARD-01 | T-66-18/19/22 | Telemetry outcome enum + LIFO regression + daemon step 14b.5 | integration | `go build ./... && go test ./internal/mcp/ -run "TestMiddleware_LIFO\|TestGuardrailMiddleware\|TestTelemetry_GuardrailOutcome" -count=1 -race && go test ./internal/daemon/ -count=1 -race && go vet ./...` | ❌ W0 | ⬜ pending |
| 66-05-T1 | 05 | 4 | GUARD-02/03/05 | T-66-24..28 | Read-tool issuance (5 tools) | unit | `go build ./... && go test ./internal/kernel/symbols/ ./internal/kernel/edit/ ./internal/kernel/diag/ -count=1 -race && go vet ./...` | ❌ W0 | ⬜ pending |
| 66-05-T2 | 05 | 4 | GUARD-02/03/05 | T-66-24..28 | Skill-side issuance + destructive args extension | unit | `go build ./... && go test ./internal/skill/... ./internal/kernel/edit/ ./internal/kernel/fileops/ -count=1 -race && go vet ./...` | ❌ W0 | ⬜ pending |
| 66-05-T3 | 05 | 4 | GUARD-02/03/05 | T-66-24 | Issuance smoke test (8 tools each increment helix_receipt_issued_total) | integration | `go test ./internal/mcp/integ/ -run TestReceiptIssuanceSmoke -count=1 -race` | ❌ W0 | ⬜ pending |
| 66-06-T1 | 06 | 4 | GUARD-02/06 | T-66-29..32 | GUARDRAILS.md + DoD.md + 6 embedded topic docs | structural | `test -f GUARDRAILS.md && test -f DoD.md && test -d internal/kernel/help/docs && test "$(ls internal/kernel/help/docs/*.md \| wc -l \| tr -d ' ')" = "6" && grep -q "G-001" GUARDRAILS.md && grep -q "G-005" GUARDRAILS.md && grep -q "Definition of Done" DoD.md && go build ./internal/kernel/help/...` | ❌ W0 | ⬜ pending |
| 66-06-T2 | 06 | 4 | GUARD-06 | T-66-31 | TopicRegistry refactor on get_tool_help | unit | `go test ./internal/kernel/help/ -count=1 -race && go vet ./internal/kernel/help/...` | ❌ W0 | ⬜ pending |
| 66-06-T3 | 06 | 4 | GUARD-02 | T-66-30/32 | Final VALIDATION.md consistency check + context-truncation E2E | integration | `go test -tags=integration ./test/harness/ -run TestContextTruncationE2E_GuardrailEnforcedWithoutReceipt -count=1 -timeout 120s` | ❌ W0 | ⬜ pending |

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
