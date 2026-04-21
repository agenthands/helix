---
phase: 36
slug: client-hooks
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-21
---

# Phase 36 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test infrastructure |
| **Quick run command** | `go test ./internal/cli/... -run Hook -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/cli/... -run Hook -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 36-01-01 | 01 | 1 | HOOK-04 | — | Hook entries merged safely into settings.json | unit | `go test ./internal/cli/... -run TestHookInstall -count=1` | ❌ W0 | ⬜ pending |
| 36-01-02 | 01 | 1 | HOOK-01 | — | SessionStart hook activates workspace | unit | `go test ./internal/cli/... -run TestActivate -count=1` | ❌ W0 | ⬜ pending |
| 36-01-03 | 01 | 1 | HOOK-02 | — | PreToolUse nudge returns message after threshold | unit | `go test ./internal/cli/... -run TestNudge -count=1` | ❌ W0 | ⬜ pending |
| 36-01-04 | 01 | 1 | HOOK-03 | — | Stop hook cleans session data | unit | `go test ./internal/cli/... -run TestDeactivate -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cli/hooks_test.go` — stubs for HOOK-01 through HOOK-04
- [ ] Existing test infrastructure covers framework needs

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Hook fires in live Claude Code session | HOOK-01 | Requires running Claude Code with Serena registered | 1. Run `serena setup claude-code` 2. Start new Claude Code session 3. Verify workspace activation message |
| Nudge appears in Claude context | HOOK-02 | Requires agent making grep calls in Claude Code | 1. Use grep 5+ times without Serena tools 2. Verify nudge message appears |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
