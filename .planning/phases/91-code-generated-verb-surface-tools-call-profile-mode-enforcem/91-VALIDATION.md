---
phase: 91
slug: code-generated-verb-surface-tools-call-profile-mode-enforcement
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-21
---

# Phase 91 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`); generator `cmd/helix-cligen` with `--check` drift gate |
| **Config file** | none — repo `go.mod` |
| **Quick run command** | `go test ./internal/cli/... ./internal/mcp/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Drift gate** | `go run ./cmd/helix-cligen --check` (exit non-zero on drift) |
| **Profile goldens** | `go test ./test/integration/ -run Profile` (and CLI verb-surface equivalents) |
| **Estimated runtime** | ~30–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched package(s)
- **After every plan wave:** Run `go vet ./... && go test ./...` plus `go run ./cmd/helix-cligen --check`
- **Before `/gsd-verify-work`:** Full suite green + drift gate green + profile goldens green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Requirement | Test Type | Automated Command | Status |
|---------|------|-------------|-----------|-------------------|--------|
| 91-xx | TBD | VERB-01 (generated verb count == live registry count, by name) | parity unit | `go test ./internal/cli/... -run Parity` | ⬜ pending |
| 91-xx | TBD | VERB-02 (`--check` drift gate fails on un-regenerated Args change) | drift gate | `go run ./cmd/helix-cligen --check` | ⬜ pending |
| 91-xx | TBD | VERB-03 (grouped --help; missing required flag errors pre-dial) | unit | `go test ./internal/cli/... -run Verb` | ⬜ pending |
| 91-xx | TBD | VERB-04 (arg-struct-derived flags from AST scan) | generator unit | `go test ./cmd/helix-cligen/...` | ⬜ pending |
| 91-xx | TBD | SEC-01 (tools/call profile/mode enforcement; typed PermissionDenied) | unit | `go test ./internal/mcp/... -run Enforce` | ⬜ pending |
| 91-xx | TBD | SEC-02 (per-profile verb-surface goldens; outside-profile hidden+refused) | golden | `go test ./test/integration/ -run Profile` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs finalized by the planner; this map is the requirement→test contract.*

---

## Wave 0 Requirements

- [ ] `cmd/helix-cligen` generator scaffold (go/packages + go/ast scan of `AddTool` sites) — mirrors `cmd/docgen` blank-import discipline
- [ ] tools/call enforcement middleware test scaffold (clone of guardrail_middleware_test pattern)
- [ ] re-pointed per-profile verb-surface golden fixtures

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `helix replace-symbol-body` refused under read mode, succeeds under edit mode | SEC-01 | Primary automated, but a live smoke confirms the typed error reaches the CLI surface | Run the verb under each mode against a live daemon; confirm typed PermissionDenied + correct exit code under read, success under edit |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
