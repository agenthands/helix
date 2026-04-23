---
phase: 39
slug: readme-rewrite
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-23
---

# Phase 39 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `go vet ./...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go vet ./...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 39-01-01 | 01 | 1 | README-01 | — | N/A | manual | `grep "Go-native" README.md` | ✅ | ⬜ pending |
| 39-01-02 | 01 | 1 | README-02 | — | N/A | manual | `grep -c "^\|" README.md` (tool table rows) | ✅ | ⬜ pending |
| 39-01-03 | 01 | 1 | README-03 | — | N/A | manual | `grep "architecture" README.md` | ✅ | ⬜ pending |
| 39-01-04 | 01 | 1 | README-04 | — | N/A | manual | `grep "serena setup" README.md` | ✅ | ⬜ pending |
| 39-01-05 | 01 | 1 | LEGC-01 | — | N/A | manual | `grep -i "port\|rewrite" README.md \| grep -v "Originally inspired"` (should be empty) | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| README reads as standalone Go product | README-01 | Subjective reading comprehension | Read full README, verify no "port" or "rewrite" framing |
| Tool table accuracy | README-02 | Requires comparing against live tool registrations | Run `go run ./cmd/serena docgen` and compare output |
| Architecture diagram completeness | README-03 | Visual/structural review | Verify 4-layer stack, RepoMap, fuzzy editing, smart errors mentioned |
| Quick start correctness | README-04 | Requires testing commands | Run `serena setup claude-code --dry-run` and verify output matches docs |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
