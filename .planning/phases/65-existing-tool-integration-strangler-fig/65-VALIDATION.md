---
phase: 65
slug: existing-tool-integration-strangler-fig
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-08
---

# Phase 65 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> See `65-RESEARCH.md` § Validation Architecture for the {source × fallback_reason} coverage matrix and harness reuse pattern (`internal/skill/semantic/integration_test.go:438` 15-symbol fixture).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — Go's built-in test runner |
| **Quick run command** | `go test ./internal/semantic/integ/... ./internal/skill/repomap/... ./internal/skill/semantic/... ./internal/kernel/symbols/... ./internal/kernel/health/... -count=1 -race` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~60s quick / ~300s full (race-on) |

---

## Sampling Rate

- **After every task commit:** Run quick command (subtree of touched packages)
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green; goldens diff-clean
- **Max feedback latency:** 60s for quick run

---

## Per-Task Verification Map

> Plan IDs are placeholders; planner finalizes shape. Filled in during planning step.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 65-00-* | 00 | 0 | — | M-vet | nokernel2semantic permits `internal/semantic/integ` only | unit | `go test ./internal/lint/nokernel2semantic/...` | ❌ W0 | ⬜ pending |
| 65-01-* | 01 | 0 | INTEG-01..05 prereq | M7 path-drift | production buildFn never panics, partial-coverage degrades not crashes | integration | `go test ./internal/daemon/... -run BuildFn` | ❌ W0 | ⬜ pending |
| 65-02-* | 02 | 0 | INTEG-01..05 prereq | M-key | WorkspaceKey resolves real key, never zero | unit | `go test ./internal/daemon/... -run WorkspaceKey` | ❌ W0 | ⬜ pending |
| 65-03-* | 03 | 1 | INTEG-01..05 | M-readtier | lookup methods never call BeginSnapshot/Commit | unit + grep canary | `go test ./internal/semantic/integ/...` | ❌ W0 | ⬜ pending |
| 65-04-* | 04 | 1 | INTEG-05 | M7 path-drift | source ∈ {semantic,tree_sitter,fallback}; fallback_reason closed-enum | unit | `go test ./internal/semantic/integ/... -run Envelope` | ❌ W0 | ⬜ pending |
| 65-05-* | 05 | 2 | INTEG-01, INTEG-02 | M7, M-cold | Available()=false → tree_sitter path; index_disabled goldens preserved | integration + golden | `go test ./internal/skill/repomap/... -run Integration` | ✅ | ⬜ pending |
| 65-06-* | 06 | 2 | INTEG-03 | M-confcap | confidence ≤ 0.6 on fallback; refuted edges flip to 0.20 | integration | `go test ./internal/kernel/symbols/... -run BlastRadius` | ✅ | ⬜ pending |
| 65-07-* | 07 | 2 | INTEG-04 | M-additive | semantic_index block extends Phase 57 SC-1 envelope additively | unit | `go test ./internal/kernel/health/...` | ✅ | ⬜ pending |
| 65-08-* | 08 | 3 | INTEG-01..05 | M7 | every {source × fallback_reason} pair covered | integration | `go test ./internal/skill/semantic/... -run StranglerFig` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Threat key:**
- M7 (path-drift) — semantic vs v1.9 path divergence; `source` field is the operational signal.
- M-vet — nokernel2semantic analyzer collision with new `internal/semantic/integ` package (research finding #2).
- M-key — WorkspaceKey zero-value regression breaks workspace-scoped retrieval.
- M-readtier — read+ tier breach if any lookup method writes a snapshot (D-03 / Phase 64 D-09).
- M-cold — cold-start background indexing forbidden (D-06 / SPEC §7).
- M-confcap — fallback-path confidence cap ≤ 0.6 (ROADMAP success criterion #2).
- M-additive — get_health field-add must not break Phase 57 SC-1 consumers.

---

## Wave 0 Requirements

- [ ] `internal/lint/nokernel2semantic/analyzer_test.go` — analysistest fixture for the integ allowlist amendment
- [ ] `internal/daemon/semantic_wiring_test.go` — buildFn integration harness extending the existing tempdir + bleve pattern
- [ ] `internal/daemon/workspacekey_adapter_test.go` — registry-lookup unit test
- [ ] `internal/semantic/integ/` — package created (types-only) by 65-03; Wave 0 test harness consumes the interface
- [ ] `internal/skill/semantic/integration_test.go` — extend `TestE2E_IndexThenContext_SymbolCount` 15-symbol fixture for the strangler-fig matrix (Wave 3 cell consumes this)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live cold-start UX (semantic enabled, no snapshot) | D-06 | Requires running daemon + MCP forwarder + observed agent call log | Start `helix daemon`; `helix setup claude-code`; from a fresh repo with no `~/.helix/` graph snapshot, call `get_repo_map` and assert `source=fallback fallback_reason=no_snapshot_yet`; assert no background indexer process started (check via `helix status --verbose`) |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter (set after planner completes per-task verification map)

**Approval:** pending
