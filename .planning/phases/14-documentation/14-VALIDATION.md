---
phase: 14
slug: documentation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-10
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test infrastructure |
| **Quick run command** | `go test ./cmd/docgen/...` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/docgen/...`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | DOC-01 | — | N/A | manual | `grep '## What is Serena' README.md` | ❌ W0 | ⬜ pending |
| 14-01-02 | 01 | 1 | DOC-02 | — | N/A | manual | `grep 'go install' README.md` | ❌ W0 | ⬜ pending |
| 14-01-03 | 01 | 1 | DOC-03 | — | N/A | integration | `go run ./cmd/docgen && grep 'BEGIN TOOLS' README.md` | ❌ W0 | ⬜ pending |
| 14-01-04 | 01 | 1 | DOC-04 | — | N/A | integration | `go run ./cmd/docgen && grep 'BEGIN LANGUAGES' README.md` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 1 | DOC-05 | — | N/A | manual | `test -f USAGE.md && grep '## Profiles' USAGE.md` | ❌ W0 | ⬜ pending |
| 14-02-02 | 02 | 1 | DOC-06 | — | N/A | manual | `grep '## Troubleshooting' USAGE.md` | ❌ W0 | ⬜ pending |
| 14-02-03 | 02 | 1 | DOC-07 | — | N/A | manual | `grep '## Workflows' USAGE.md` | ❌ W0 | ⬜ pending |
| 14-02-04 | 02 | 1 | DOC-08 | — | N/A | manual | `grep -i 'observability' USAGE.md` | ❌ W0 | ⬜ pending |
| 14-02-05 | 02 | 1 | DOC-09 | — | N/A | manual | `grep -i 'performance' USAGE.md` | ❌ W0 | ⬜ pending |
| 14-03-01 | 03 | 1 | DOC-10 | — | N/A | manual | `grep 'v1.0' CHANGELOG.md && grep 'v1.1' CHANGELOG.md && grep 'v1.2' CHANGELOG.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/docgen/main.go` — codegen tool for tool/language tables
- [ ] `cmd/docgen/main_test.go` — tests for codegen output correctness

*Existing Go test infrastructure covers all other phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| README readability | DOC-01 | Subjective — requires human reading | Read README.md end-to-end, verify a new user can follow install → configure → first tool call |
| USAGE.md completeness | DOC-05-09 | Subjective — requires human assessment | Read USAGE.md sections, verify each topic has actionable content |
| CHANGELOG accuracy | DOC-10 | Historical — requires cross-referencing milestones | Verify dates and feature lists match ROADMAP.md milestones |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
