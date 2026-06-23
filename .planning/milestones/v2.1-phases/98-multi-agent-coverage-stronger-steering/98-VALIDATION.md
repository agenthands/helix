---
phase: 98
slug: multi-agent-coverage-stronger-steering
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-22
---

# Phase 98 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib testing) |
| **Config file** | none — Go toolchain |
| **Quick run command** | `go test ./internal/cli/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | ~60–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/cli/...`
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite + `helix-refgen --check` green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills) | — | — | STEER-01/02/03, AGENT-01/02/03 | — | exit-0 fail-open; no-clobber append | unit | `go test ./internal/cli/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/cli/nudge_test.go` — STEER-01 sed/cat steering (broaden `isGrepReadTool`); flip Phase 97 silent sub-cases → firing (bump floor ≥7); STEER-03 prose/log/config negative-control rows (exit-0, non-firing)
- [ ] `internal/cli/setup_agents_test.go` (new) — AGENT-02 idempotent sentinel-append round-trip (no-clobber, no-duplicate-on-rerun, AGENTS.md 32 KiB cap); AGENT-01 shared reference install; AGENT-03 Codex hook present + NO fabricated Gemini hook
- [ ] STEER-02 SessionStart priming — terse matrix appended to `helix activate` stdout, size-capped (≤~2 KiB constant), fail-open
- [ ] Anti-vacuity: wrong-verb-goes-RED on the new steering golden cases

*Existing infrastructure (go test, `runNudgeCapture`/`parseAdvisory`, `mergeHooksIntoSettings` idempotent-block precedent, `withinSkillRoot`/atomic-write) covers most phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Codex `hooks.json` byte-schema + discovery location (flag A1) | AGENT-03 | External runtime convention not re-verified live in research | Install into a real Codex config dir; confirm Codex loads the PreToolUse hook and it invokes `helix nudge` |
| Codex `AGENTS.md` / Gemini `GEMINI.md` path + 32 KiB cap (flag A2) | AGENT-02 | External runtime file locations not re-verified live | Confirm each agent discovers its instruction file at the written path |

*If the planner pins these against committed fixtures + golden bytes, they may move to automated; otherwise they surface as human_needed (acceptable per autonomous routing).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
