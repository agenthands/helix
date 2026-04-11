---
phase: 17
slug: usage-install-guides
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-11
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — documentation phase, validation is content-based |
| **Quick run command** | `go vet ./... && go test ./...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./... && go test ./...`
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | USAGE-01 | — | N/A | content | `grep -c "Prometheus\|OTLP\|admin listener" docs/USAGE.md` | ❌ W0 | ⬜ pending |
| 17-01-02 | 01 | 1 | USAGE-02 | — | N/A | content | `grep -c "GOMEMLIMIT\|circuit.breaker\|per-class" docs/USAGE.md` | ❌ W0 | ⬜ pending |
| 17-01-03 | 01 | 1 | USAGE-03 | — | N/A | content | `grep -c "benchmark\|benchstat\|go test -bench" docs/USAGE.md` | ❌ W0 | ⬜ pending |
| 17-02-01 | 02 | 1 | INST-01 | — | N/A | content | `test -f docs/INSTALL.md` | ❌ W0 | ⬜ pending |
| 17-02-02 | 02 | 1 | INST-02 | — | N/A | content | `grep -c "Claude Code\|Codex\|OpenCode\|Cursor\|Gemini CLI\|Antigravity" docs/INSTALL.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements. This is a documentation phase — validation is content-based grep checks, no test framework additions needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Config examples are copy-pasteable | USAGE-01 | Requires human judgment on config correctness | Review each YAML/env example for syntactic validity |
| Agent setup instructions work end-to-end | INST-02 | Requires actual agent installations | Follow install steps for at least one agent |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
