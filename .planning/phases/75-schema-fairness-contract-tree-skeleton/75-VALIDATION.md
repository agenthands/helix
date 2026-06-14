---
phase: 75
slug: schema-fairness-contract-tree-skeleton
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-14
---

# Phase 75 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./bench/... ./cmd/helix-bench/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/... ./cmd/helix-bench/...`
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {N}-01-01 | 01 | 0 | INFRA-01 | — | N/A | integration | `git log --follow` continuity check | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*(Filled in by the planner per PLAN.md tasks.)*

---

## Wave 0 Requirements

- [ ] Relocation of Phase 64 semantic microbench to `internal/semantic/bench/` (D-05/D-06) lands before any new `bench/` file
- [ ] Existing `go test` infrastructure covers all phase requirements — no new framework install

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| TOS attestation accuracy (PROVIDERS.md content reflects real provider TOS) | FAIR-02 / COST-01 | Requires human reading of provider Terms of Service | Maintainer confirms each `tos_url` + `benchmarking_permitted` flag against the live TOS at attestation time |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
