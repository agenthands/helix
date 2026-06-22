---
phase: 93
slug: skill-md-nudge-repurpose-helix-setup-flip
status: passed
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-21
audited: 2026-06-22
---

# Phase 93 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Retroactively audited and closed 2026-06-22 (Phase 96 TD-05) — phase shipped + passed verification; this reconciles the Nyquist coverage map against the actual tests on disk.

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

| Plan | Requirement | Test Type | Automated Command | Status |
|------|-------------|-----------|-------------------|--------|
| 93-01 | SKILL-01 (SKILL.md go:embed + atomic installSkill + containment + drift gate) | unit | `go test ./internal/cli/ -run 'TestSkill\|TestInstallSkill'` (TestSkillEmbedNonEmpty, TestSkillFrontmatterValid, TestInstallSkill{WritesContent,Idempotent,Containment,CreatesNestedDir,NoTempLeftover}, TestSkillTargetDir, TestSkillVerbMembershipDrift, TestSkillNoVerbCountLiteral) | ✅ green |
| 93-02 | SKILL-02 (nudge repurpose: code-vs-noncode classifier, fail-open, additionalContext advisory) | unit (table) | `go test ./internal/cli/ -run TestClassifyBashTarget` (CodeTargets, NonCodeTargets, MixedTargetsConservative, FailOpen, PureDataNoExec, GrepPatternNotFile) | ✅ green |
| 93-03 | SKILL-03 (setup flip: skill+hooks install, MCP-only hook-preserving teardown, idempotent across 7 clients) | unit | `go test ./internal/cli/ -count=1` (setup_test.go registrar/teardown cases) | ✅ green |
| 93-04 | SKILL-04 (idle-cost bound ≤1536 chars) | unit | `go test ./internal/cli/ -run 'TestSkillIdleCostBound\|TestSkillDescriptionCap\|TestSkillTokenNote'` | ✅ green |
| 93-04 | TEST-03 (skill-vs-grep baseline tool-selection behavioral oracle) | behavioral (gated) | `go test -tags llm ./test/oracle/llm/ -run SkillTrigger` — `//go:build llm` + `SkipWithoutAPIKey` (RUNS when keyed; honest gate, never a vacuous pass) | ✅ automated (key-gated) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] PreToolUse `additionalContext` JSON shape confirmed — shipped in 93-02 as the `preToolUseOutput`/`hookSpecificOutput.additionalContext` envelope (json.Marshal), covered by the `TestClassifyBashTarget_*` suite + nudge advisory tests.
- [x] `test/oracle/llm/` extension points confirmed — 93-04 added `skill_trigger_test.go` + baseline/with transcripts, reusing the existing `SkipWithoutAPIKey`/`AskSingleTurn`/`FormatToolList` harness.

*Existing infrastructure (`go test`, `test/oracle/llm/`) covers phase requirements; no new framework needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-agent skill triggering in a live Claude Code session | SKILL-01 | Requires a live client, not reproducible in CI | Install via `helix setup claude-code`, observe skill fires on a code-nav prompt and stays dormant on an unrelated one |

*Behavioral-oracle automation (TEST-03) covers the measurable proxy; the live-client check is the manual backstop.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none — all COVERED)
- [x] No watch-mode flags
- [x] Feedback latency < 90s (untagged)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-22 (retroactive audit, Phase 96 TD-05)

## Validation Audit 2026-06-22

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All requirement areas (SKILL-01..04, TEST-03) have automated tests present on disk and green under `go test ./internal/cli/... -count=1` (exit 0); TEST-03's behavioral oracle is honestly `//go:build llm` + `SkipWithoutAPIKey` gated. No MISSING gaps → auditor not spawned (workflow §3 "no gaps → close"). The single manual-only item (live-client triggering) is a documented backstop with an automated proxy (TEST-03).
