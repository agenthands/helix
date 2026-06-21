---
phase: 93
slug: skill-md-nudge-repurpose-helix-setup-flip
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-21
---

# Phase 93 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — `go.mod` at repo root |
| **Quick run command** | `go test ./internal/cli/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./... -count=1` |
| **Gated suites** | behavioral oracle: `go test -tags llm ./test/oracle/llm/...` (SkipWithoutAPIKey — needs an LLM API key, not vacuous); setup/E2E paths may be `HELIX_BIN`-gated — build `helix` and export `HELIX_BIN` so they RUN, never count a SKIP as a pass |
| **Estimated runtime** | ~30–90 s untagged; +network/LLM for the `llm`-tagged oracle |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/cli/... -count=1`
- **After every plan wave:** full suite (`go vet ./... && go test ./... -count=1`)
- **Before `/gsd-verify-work`:** full suite green; behavioral oracle run with an API key (or honestly reported as key-gated)
- **Max feedback latency:** ~90 s (untagged)

---

## Per-Task Verification Map

> The planner populates this from the actual PLAN.md task IDs. Each behavior-adding task needs an `<automated>` verify command or a Wave 0 dependency. Known anchors:
> - SKILL-01 (SKILL.md schema + go:embed): `go test ./internal/cli/... -run Skill` asserting frontmatter validity + embed presence.
> - SKILL-02 (nudge repurpose): table-driven test over grep/sed/cat-on-code vs README/log → suggestion vs none; exit always 0; fail-open on unparseable Bash.
> - SKILL-03 (setup flip): idempotent install (skill+hooks present, no MCP entry) + MCP-only teardown across clients; re-run is a no-op.
> - SKILL-04 / TEST-03 (behavioral oracle): extend `test/oracle/llm/` to compare skill-present vs grep/sed/cat baseline tool selection; record an idle-skill-cost (≤1,536-char description bound, or exact count_tokens when keyed) number.

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | Status |
|---------|------|------|-------------|-----------|-------------------|--------|
| (planner fills) | | | SKILL-01..04, TEST-03 | unit / behavioral | `go test ...` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Confirm exact PreToolUse `additionalContext` JSON shape for the target Claude Code version (RESEARCH open question 1).
- [ ] Confirm `test/oracle/llm/` extension points for the skill-vs-baseline comparison.

*Otherwise existing infrastructure (`go test`, `test/oracle/llm/`) covers phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-agent skill triggering in a live Claude Code session | SKILL-01 | Requires a live client, not reproducible in CI | Install via `helix setup claude-code`, observe skill fires on a code-nav prompt and stays dormant on an unrelated one |

*Behavioral-oracle automation (TEST-03) covers the measurable proxy; the live-client check is the manual backstop.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s (untagged)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
