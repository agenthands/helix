---
phase: 95
slug: identity-docs-rewrite-docgen-regen
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-22
---

# Phase 95 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + docgen/--check drift gate + grep-based doc assertions |
| **Config file** | none — `go.mod`, `Makefile`, `.github/workflows/go-test.yml` |
| **Quick run command** | `go run ./cmd/docgen --check` (drift gate) + `go test ./cmd/docgen/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` + `make verify-docs` (new target) |
| **Estimated runtime** | ~30–60 s |

---

## Sampling Rate

- **After every task commit:** `go run ./cmd/docgen --check` + `go test ./cmd/docgen/...`
- **After the docs rewrite:** grep assertions that no doc claims MCP as the *primary agent interface* (DOCS-01) and that the CLAUDE.md Helix routing cites real `helix` verbs (DOCS-02)
- **Before `/gsd-verify-work`:** `make verify-docs` green; full `go test ./...` green
- **Max feedback latency:** ~60 s

---

## Per-Task Verification Map

> Planner fills from real task IDs. Anchors:
> - DOCS-01: grep README/CLAUDE.md/PROJECT.md for "primary interface"/"primary agent interface"/"53 MCP tools" → zero MCP-as-primary claims; CLI-first framing present. Preserve the true "MCP-SDK/gRPC retained internally" statements (do not over-claim removal).
> - DOCS-02: the CLAUDE.md Helix-CLI routing section cites `helix <verb>` (real names from `cli.VerbToolNames()`); the EXTERNAL `mcp__smtc__*` SMTC matrix is LEFT INTACT (with a clarifier so they aren't conflated).
> - DOCS-03 (load-bearing): regenerate the README tool table via `cmd/docgen`; ADD a `docgen --check` CI step to `.github/workflows/go-test.yml` + a `make verify-docs` target (mirror the existing `verify-cligen` pattern); update `cmd/docgen/main_test.go` assertions from raw tool names to verb forms; docgen blank-imports reconciled with the daemon's (tool-set parity via the gate, cross-ref comment — NOT necessarily literal import equality).

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------|-------------------|--------|
| (planner fills) | | | DOCS-01..03 | drift-gate / grep / unit | `go run ./cmd/docgen --check` / `make verify-docs` / `go test ./cmd/docgen/...` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Add `docgen --check` CI step to `.github/workflows/go-test.yml` and a `make verify-docs` target (the v1.12 drift hole was never closed in CI — RESEARCH finding 1).
- [ ] Update `cmd/docgen/main_test.go` raw-tool-name assertions to the kebab verb forms.

*Otherwise existing infra (`go test`, `cmd/docgen --check`) covers phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Prose reads as genuinely CLI-first (not just keyword-swapped) | DOCS-01 | Editorial quality is not grep-checkable | Human skim of README/CLAUDE.md/PROJECT.md intros after the rewrite |

*The MCP-as-primary claim removal (the measurable part) is grep-asserted; the editorial-quality check is the manual backstop.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] `docgen --check` drift gate green AND wired into CI (`make verify-docs`)
- [ ] No doc claims MCP as the primary agent interface (grep-asserted)
- [ ] `git diff go.mod` empty
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
