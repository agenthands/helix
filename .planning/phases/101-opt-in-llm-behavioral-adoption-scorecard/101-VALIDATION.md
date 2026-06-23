---
phase: 101
slug: opt-in-llm-behavioral-adoption-scorecard
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-23
---

# Phase 101 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) — the PURE scorer + anti-vacuity tests run in the DEFAULT suite; the live model leg is `//go:build llm` opt-in |
| **Config file** | none — Go toolchain; live leg needs `ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY` (SkipWithoutAPIKey) |
| **Quick run command** | `go test ./test/oracle/adopt/...` (hermetic — no key) |
| **Full suite command** | `go vet ./... && go test ./...` (pure scorer runs; live leg excluded) |
| **Estimated runtime** | ~10–30 seconds (hermetic); live leg longer + key-gated |

---

## Sampling Rate

- **After every task commit:** `go test ./test/oracle/adopt/...` (hermetic, default suite)
- **After every plan wave:** `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** hermetic scorer + sabotaged-skill revert-and-fail + negative-exemplar + empty-bucket-rejection all GREEN in the DEFAULT suite (no API key); live leg builds under `-tags llm`
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills) | — | — | ADOPT-02 | — | never-blocks-merge; non-vacuous | unit (hermetic) | `go test ./test/oracle/adopt/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] NEW build-tag-FREE `test/oracle/adopt` package: the pure scorer (parse transcript → `firstCommandLine` detector → choice/fallback classify → choice_rate/fallback_rate), lifted from `skill_trigger_test.go:25-60` (can't import across the build tag)
- [ ] `StripDecisionMatrix(EmbeddedSkillBody())` pure helper (section-strip at SKILL.md `## Decision matrix`), guarded by `len(out) < len(in)` (no silent no-op)
- [ ] Committed fixture transcripts (helix-choosing + grep-falling-back) for the hermetic scorer
- [ ] **Sabotaged-skill revert-and-fail (HERMETIC, the anti-vacuity anchor):** `choice_rate(intact skill)` − `choice_rate(matrix-stripped)` ≥ material threshold (~0.4); identical scores → test FAILS
- [ ] **Negative judge exemplar:** rubric `adoption` dimension whose grep-finds-a-definition response scores 0.0 → `ComputeVerdict` → `fail` (rubric can demonstrably fail)
- [ ] Empty/one-element task bucket rejected as a pass (`MinTasks` floor ~5)
- [ ] Live model leg (`//go:build llm`) + judge extension (`//go:build llmjudge`) — thin adapters over the SAME pure scorer; SkipWithoutAPIKey; NEVER in `go test ./...`; never blocks merge

*Existing infrastructure (`AskSingleTurn`/`SkipWithoutAPIKey`, `Transcript`, `SkillSystemPrompt`/`GrepBaselineSystemPrompt`, `RubricPrompt`/`ComputeVerdict`, the first-command detector) is ~80% reuse.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live model adoption score against a real model | ADOPT-02 | Needs an API key + a live model; non-deterministic; opt-in/never-gating | `ANTHROPIC_API_KEY=... go test -tags llm ./test/oracle/adopt/...` — observe choice_rate/fallback_rate on a real model |

*The live leg is opt-in/key-gated (acceptable as human/optional). ALL anti-vacuity proofs (revert-and-fail, negative exemplar, detector, empty-bucket) are HERMETIC and run in the default suite without a key.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
