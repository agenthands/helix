---
phase: 80
slug: five-of-six-ablation-runners-fairness-enforcement
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-19
---

# Phase 80 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Pre-populated from `80-RESEARCH.md` § Validation Architecture (Nyquist enabled).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` (`go test`) — built-in, no install |
| **Config file** | none (Go convention) |
| **Quick run command** | `go test ./bench/runners/ ./bench/runtime/ -count=1` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30–90 seconds (quick); full suite minutes |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/runners/ ./bench/runtime/ -count=1 && go vet ./bench/...`
- **After every plan wave:** Run `go test ./bench/... ./internal/profile/... -count=1`
- **Before `/gsd-verify-work`:** `go test ./... && go vet ./...` green; `make bench-quick` exits 0 (hermetic scripted smoke); a multi-mode scripted smoke produces 4 real rows + 1 partial (no_semantic) + 0 baseline_rag rows
- **Max feedback latency:** ~90 seconds (quick run)

---

## Per-Task Verification Map

> Filled by the planner/executor once plan task IDs exist. Anchors below come from
> the research Requirements→Test map; map each to the concrete `{N}-PP-TT` task.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 80-PP-TT | PP | W | ABLATE-01 | T-80-01 (path traversal) | malformed/unknown mode name fails closed via `validateModeName` | unit | `go test ./bench/runners/ -run TestModeResolver -count=1` | ❌ W0 | ⬜ pending |
| 80-PP-TT | PP | W | ABLATE-01 | — | 4 real rows + 1 partial (no_semantic) + 1 fail-closed (baseline_rag, no row) on smoke | integration | `go test ./bench/runtime/ -run TestRunCell -count=1` + multi-mode smoke | ❌ W0 | ⬜ pending |
| 80-PP-TT | PP | W | ABLATE-03 | — | `baseline_plain` → profile `baseline`; empty Helix inventory | unit | `go test ./internal/profile/ -run TestBaselineExposesZeroHelixTools -count=1` + resolver assertion | ⚠️ partial | ⬜ pending |
| 80-PP-TT | PP | W | FAIR (D-04) | T-80-02 (stub scored as real) | startup `Validate()` fatal on non-nil contract | unit | `go test ./bench/runtime/ -run TestFairnessGate -count=1` | ❌ W0 | ⬜ pending |
| 80-PP-TT | PP | W | FAIR (D-04) | — | every runner's effective config == `DefaultContract` (unconditional) | unit (contract) | `go test ./bench/runners/ -run TestEffectiveConfigMatchesContract -count=1` | ❌ W0 | ⬜ pending |
| 80-PP-TT | PP | W | D-03 | T-80-02 | no_semantic row carries `ablation_status: guarantee_pending_phase_81`; schema valid | unit | `go test ./bench/runtime/ -run TestAblationStatus -count=1` | ❌ W0 | ⬜ pending |
| 80-PP-TT | PP | W | D-05 | — | 3 deltas compute correctly + surface in per-mode rows | unit | `go test ./bench/runtime/ -run TestAblationDeltas -count=1` | ❌ W0 | ⬜ pending |
| (regression) | — | — | — | — | system-prompt drift gate stays green | unit | `go test ./bench/runners/ -run TestSystemPromptHashMatches -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/runners/contract_test.go` — unconditional effective-config == `DefaultContract` (D-04)
- [ ] `bench/runtime/cell_test.go` (extend) — fairness gate fatal; baseline_rag no-row; no_semantic `ablation_status`
- [ ] `bench/runners/mode_resolver_test.go` (extend) — resolve the 5 new modes to their profiles
- [ ] `bench/runtime/` delta test — 3-delta arithmetic + per-mode-row write-back
- [ ] Multi-mode scripted smoke harness (4 real + 1 partial + 1 fail-closed)
- [ ] Framework install: none — Go `testing` is built-in.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real `claude`-agent fairness enforcement (effective `temperature`/`max_tokens`/etc. applied) | FAIR (D-04) | Hermetic CI gate uses the scripted agent (no model); the live `claude` path is opt-in and not exercised in CI | Run `make bench-quick` with `--agent=claude` locally and inspect the emitted `result.v2.json` effective-config block |

*The scripted-agent smoke + unconditional contract test cover all CI-gated behaviors automatically.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
