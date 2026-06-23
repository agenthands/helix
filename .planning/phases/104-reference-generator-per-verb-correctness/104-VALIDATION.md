---
phase: 104
slug: reference-generator-per-verb-correctness
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-24
---

# Phase 104 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./cmd/helix-refgen/... ./cmd/helix-cligen/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/helix-refgen/...`
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite green + `go run ./cmd/helix-refgen --check` byte-clean (`git diff` empty)
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 104-01-01 | 01 | 0 | REFGEN-01 | — | N/A | unit (RED) | `go test ./cmd/helix-refgen/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — refined by the planner.*

---

## Wave 0 Requirements

- [ ] `cmd/helix-refgen/render_test.go` — override-map vacuity guards for REFGEN-01 (no override key is a non-verb; an overridden Output line differs from the old group default it replaced — deliberate break-the-invariant → assert-RED)

*Refined by the planner against the RESEARCH.md Validation Architecture section.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification (`go test`, `helix-refgen --check`, `reference ⊇ VerbToolNames()` contract, `verify-cligen` parity).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
