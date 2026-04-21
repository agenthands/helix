---
phase: 34
slug: setup-cli-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-21
---

# Phase 34 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test infrastructure |
| **Quick run command** | `go test ./internal/cli/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/cli/...`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 34-01-01 | 01 | 1 | SETUP-01 | — | N/A | unit | `go test ./internal/cli/setup/...` | ❌ W0 | ⬜ pending |
| 34-01-02 | 01 | 1 | SETUP-02 | — | N/A | unit | `go test ./internal/cli/setup/...` | ❌ W0 | ⬜ pending |
| 34-02-01 | 02 | 1 | SETUP-03 | — | Config path validation | unit | `go test ./internal/cli/setup/...` | ❌ W0 | ⬜ pending |
| 34-02-02 | 02 | 1 | SETUP-04 | — | No arbitrary file writes | unit | `go test ./internal/cli/setup/...` | ❌ W0 | ⬜ pending |
| 34-03-01 | 03 | 2 | SETUP-05 | — | N/A | unit | `go test ./internal/cli/setup/...` | ❌ W0 | ⬜ pending |
| 34-03-02 | 03 | 2 | SETUP-06 | — | N/A | unit | `go test ./internal/cli/setup/...` | ❌ W0 | ⬜ pending |
| 34-04-01 | 04 | 2 | SETUP-07 | — | N/A | integration | `go test ./internal/cli/setup/... -run TestHealthCheck` | ❌ W0 | ⬜ pending |
| 34-04-02 | 04 | 2 | SETUP-08 | — | N/A | unit | `go test ./internal/cli/setup/... -run TestUninstall` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cli/setup/setup_test.go` — stubs for SETUP-01 through SETUP-08
- [ ] Test fixtures for client config formats (Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, generic)

*Existing Go test infrastructure covers framework needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `serena setup claude-code` registers in Claude Code | SETUP-01 | Requires live Claude Code installation | Run `serena setup claude-code` in a test project, verify tool appears in Claude Code |
| `serena setup vscode` writes correct settings.json | SETUP-03 | Requires VS Code installation | Run setup, check `.vscode/settings.json` has correct MCP entry |
| Health check verifies LS responses | SETUP-07 | Requires language servers installed | Run setup in a Go project, verify gopls status reported |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
