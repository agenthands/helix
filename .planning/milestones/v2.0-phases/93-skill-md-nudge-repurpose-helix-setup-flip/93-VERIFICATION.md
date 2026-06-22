---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
verified: 2026-06-21T00:00:00Z
status: passed
score: 21/21 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: false
---

# Phase 93: SKILL.md + Nudge Repurpose + helix setup Flip — Verification Report

**Phase Goal:** Ship an embedded `SKILL.md` (go:embed, frontmatter + decision table citing the frozen Phase 92 verbs + terse output) whose description fires on code-navigation/edit tasks without over-firing; repurpose the PreToolUse nudge to advisory-steer grep/sed/cat toward the equivalent `helix <verb>` (exit 0, fail-open); and flip `helix setup <client>` to install skill + hooks and idempotently tear down any prior MCP registration. Behavior verified empirically via the LLM behavioral harness.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Binary embeds non-empty SKILL.md via go:embed (compiled in) | ✓ VERIFIED | `internal/cli/skill.go:21` `//go:embed skills/helix/SKILL.md` → `embeddedSkillMD string`; `TestSkillEmbedNonEmpty` PASS |
| 2 | SKILL.md has valid YAML frontmatter (name/description/allowed-tools); description ≤1536 chars | ✓ VERIFIED | Frontmatter present (name: helix, description, allowed-tools: Bash(helix *)); `TestSkillFrontmatterValid`, `TestSkillDescriptionCap`, `TestSkillIdleCostBound` PASS |
| 3 | Every helix verb in the SKILL.md decision table is a real `cli.VerbToolNames()` member (no drift) | ✓ VERIFIED | `TestSkillVerbMembershipDrift` parses verb citations, asserts ⊆ `VerbToolNames()` (50 verbs) — non-tautological, PASS |
| 4 | `installSkill` writes SKILL.md verbatim under a contained dir, atomically (temp+rename), idempotently | ✓ VERIFIED | `skill.go:131` installSkill (temp+rename), `withinSkillRoot` real `filepath.Rel` containment (WR-02 fix); `TestInstallSkillContainment`, `TestInstallSkillIdempotent` PASS |
| 5 | SKILL.md records real measured idle-skill-cost + preloaded-full-schema numbers (SKILL-04) | ✓ VERIFIED | Token note: idle 599 bytes, preload 2467 bytes (39 tools), 1536-char bound; `TestSkillTokenNoteFilled` (no placeholder, digit-bearing) PASS |
| 6 | grep/sed/cat over CODE target → advisory `helix <verb>` via hookSpecificOutput.additionalContext on stdout | ✓ VERIFIED | `nudge.go` steerMessage+emitAdvisory; `TestNudgeAdvisory_BashCodeGrep_Suggests`, `_GrepTool_Suggests`, `_ReadTool_Suggests` PASS |
| 7 | grep over README/log/config (.md/.log/.txt/.json/.yaml/Dockerfile) → NO suggestion | ✓ VERIFIED | `TestClassifyBashTarget_NonCodeTargets`, `TestNudgeAdvisory_BashNonCodeGrep_Silent` PASS |
| 8 | Unparseable Bash / no operand / non-code target → fail open (no suggestion) | ✓ VERIFIED | `TestClassifyBashTarget_FailOpen`, `TestNudgeAdvisory_BashUnparseable_FailOpenSilent` PASS; runNudge returns nil on stdin decode failure |
| 9 | Nudge ALWAYS exits 0 (advisory, never blocks); exit 2 never returned | ✓ VERIFIED | `runNudge` returns nil on every path (read at `nudge.go:64-122`); `TestNudgeAdvisory_AlwaysExitZero` PASS |
| 10 | Command string is DATA only — never shelled out / exec'd (T-36-01) | ✓ VERIFIED | json.Decoder + strings.Fields parse, no os/exec on input; `TestClassifyBashTarget_PureDataNoExec` PASS |
| 11 | After `helix setup claude-code`: skill + hooks present, NO MCP server entry | ✓ VERIFIED | `Register` (setup_clients.go:173) does teardownMCP→installSkill→hooks, no MCP register; `TestSetupFlip_ClaudeCode_NoMCPEntry_SkillAndHooksPresent` PASS |
| 12 | Re-running `helix setup <client>` is idempotent (byte-stable) | ✓ VERIFIED | `TestSetupFlip_Idempotent`, `TestInstallSkillIdempotent`, `TestMergeHooksIntoSettings_Idempotent` PASS |
| 13 | MCP teardown removes ONLY the MCP half; does NOT strip hooks (Pitfall 3) | ✓ VERIFIED | `teardownMCP` omits removeHooksFromSettings (comment+code); `TestTeardownPriorMCP_PreservesHooks` PASS |
| 14 | Teardown covers all 7 supported clients' MCP-removal path | ✓ VERIFIED | 7 `teardownMCP` methods (ClaudeCode/Gemini/VSCode/JetBrains/ClaudeDesktop/OpenCode/Generic); `TestTeardownPriorMCP_*` for each PASS |
| 15 | Config writes use encoding/json Marshal (never string-concat); missing file/key → no-op | ✓ VERIFIED | removeFromJSONConfig used; `TestTeardownPriorMCP_MissingFileNoOp` PASS |
| 16 | `--uninstall` removes hooks even with no MCP entry (CR-01 fix) | ✓ VERIFIED | `Unregister` (setup_clients.go:253) MCP removal best-effort `_ = cmd.Run()`, ALWAYS proceeds to hook+skill removal |
| 17 | Behavioral oracle compares tool-selection with/without SKILL.md body; records shift toward helix vs grep baseline (TEST-03) | ✓ VERIFIED | KEYED run: `TestSkillVsBaseline` PASS — baseline grep 8/8, skill helix 8/8, shifted 8/8 (real DeepSeek API calls) |
| 18 | Oracle is `//go:build llm` + SkipWithoutAPIKey gated; runs with key, skips without (never vacuous) | ✓ VERIFIED | Build tag + SkipWithoutAPIKey first line; ran for real with DEEPSEEK_API_KEY present |
| 19 | SKILL.md records idle-cost + full-schema numbers + dependency-free char-cap bound (SKILL-04) | ✓ VERIFIED | Same as truth #5; non-gated `TestSkillIdleCostBound` provides the dependency-free proof |
| 20 | Skill idle cost asserted by non-gated unit test (≤1536-char bound, no API key) | ✓ VERIFIED | `TestSkillIdleCostBound` (no key needed) PASS |
| 21 | CR-02: helixSymbolicTools matches live registry; `TestHelixSymbolicTools_NoDrift` exists+passes | ✓ VERIFIED | Map has 9 real names (search_symbols, get_symbol_overview, analyze_blast_radius, go_to_definition + 5); `TestHelixSymbolicTools_NoDrift` asserts each ⊆ VerbToolNames(), PASS |

**Score:** 21/21 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/cli/skills/helix/SKILL.md` | Embedded skill: frontmatter + decision table + token note | ✓ VERIFIED | Valid frontmatter, `\| Question \| Use this \| Not this \|` table, real SKILL-04 numbers |
| `internal/cli/skill.go` | go:embed + installSkill/skillTargetDir/uninstallSkill/withinSkillRoot/skillDescription | ✓ VERIFIED | All functions present; real path containment (WR-02) |
| `internal/cli/skill_test.go` | embed/frontmatter/drift/token-note/idempotency tests | ✓ VERIFIED | All named tests PASS |
| `internal/cli/nudge.go` | classifyBashTarget + advisory additionalContext (exit 0) + real symbolic tool map | ✓ VERIFIED | CR-02 fix in code; runNudge always exits 0 |
| `internal/cli/nudge_test.go` | code→suggest/noncode→silent/fail-open/exit-0/NoDrift tests | ✓ VERIFIED | 12 behavioral tests + drift test PASS |
| `internal/cli/setup_clients.go` | teardownMCP (MCP-only) per client + wired Register/Unregister | ✓ VERIFIED | 7 teardownMCP; CR-01 best-effort Unregister |
| `internal/cli/setup.go` | runSetup wires skill install + --no-skill | ✓ VERIFIED | `TestSetupFlip_NoSkillFlag` PASS |
| `internal/cli/setup_test.go` | no-MCP/skill+hooks/idempotent/all-clients tests | ✓ VERIFIED | All TestSetupFlip_* / TestTeardownPriorMCP_* PASS |
| `test/oracle/llm/skill_trigger_test.go` | skill-vs-baseline comparison (//go:build llm) | ✓ VERIFIED | KEYED run PASS |
| `test/oracle/llm/prompt.go` | SkillSystemPrompt + helix-verb tasks | ✓ VERIFIED | Compiles (go vet -tags llm clean), drives keyed run |

### Key Link Verification

| From | To | Via | Status |
| --- | --- | --- | --- |
| skill_test.go | verb.go | SKILL.md verbs ⊆ VerbToolNames() drift gate | ✓ WIRED (`TestSkillVerbMembershipDrift`) |
| skill.go | SKILL.md | //go:embed skills/helix/SKILL.md | ✓ WIRED (`skill.go:21`) |
| nudge.go | PreToolUse contract | hookSpecificOutput.additionalContext + exit 0 | ✓ WIRED (preToolUseOutput envelope) |
| setup_clients.go Register | skill.go installSkill | installSkill(skillTargetDir(...)) | ✓ WIRED (setup_clients.go:195) |
| setup_clients.go teardownMCP | MCP-only removal | removeFromJSONConfig without hook strip | ✓ WIRED (Pitfall-3 preserved) |
| skill_trigger_test.go | embeddedSkillMD | SkillSystemPrompt loads skill body | ✓ WIRED (keyed run loaded body) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Build | `go build ./...` | OK | ✓ PASS |
| Vet (cli) | `go vet ./internal/cli/...` | clean | ✓ PASS |
| Vet (llm tag compiles) | `go vet -tags llm ./test/oracle/llm/...` | clean | ✓ PASS |
| Full untagged suite | `go test ./...` | all ok, no FAIL | ✓ PASS |
| cli suite | `go test ./internal/cli/...` | ok 3.1s | ✓ PASS |
| KEYED behavioral oracle (TEST-03) | `go test -tags llm ./test/oracle/llm/... -run Skill` | PASS, shift 8/8 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| SKILL-01 | 93-01 | Embedded SKILL.md + frontmatter + decision table, fires on code tasks | ✓ SATISFIED | Truths 1-3; drift test asserts frozen verbs |
| SKILL-02 | 93-03 | setup flip: install skill+hooks, idempotent, teardown prior MCP | ✓ SATISFIED | Truths 11-16; all setup-flip tests PASS |
| SKILL-03 | 93-02 | PreToolUse advisory nudge (exit 0, fail-open) | ✓ SATISFIED | Truths 6-10; 12 nudge tests + CR-02 fix |
| SKILL-04 | 93-01, 93-04 | Token-efficiency rationale backed by real measurement | ✓ SATISFIED | Truths 5,19,20; token note + non-gated bound test |
| TEST-03 | 93-04 | Behavior verified via LLM harness (tool-selection shift) | ✓ SATISFIED | Truths 17-18; KEYED run, shift 8/8 |

All 5 requirement IDs accounted for. (Note: REQUIREMENTS.md numbers SKILL-02 as the setup flip and SKILL-03 as the nudge — the inverse of the phase-goal prose ordering; both are covered.)

### Critical Invariants (Phase 94 boundary)

| Invariant | Status | Evidence |
| --- | --- | --- |
| Phase-93 commits did NOT touch api/proto/ | ✓ HELD | `git diff 6397b17a^..HEAD -- api/proto/` empty |
| Phase-93 commits did NOT touch internal/daemon/ | ✓ HELD | empty in phase-93 range |
| Phase-93 commits did NOT touch internal/forwarder/ | ✓ HELD | empty in phase-93 range |
| No new deps (go.mod/go.sum unchanged by phase 93) | ✓ HELD | empty in phase-93 range (the diff vs main is prior-phase) |
| Daemon-side MCP head left intact (deletion is Phase 94) | ✓ HELD | teardownMCP is registration-only; no head deletion |

### Anti-Patterns Found

None blocking. CR-01 and CR-02 from 93-REVIEW.md were confirmed FIXED in actual code (not merely claimed): CR-01 best-effort MCP removal in Unregister with always-proceed hook+skill teardown; CR-02 helixSymbolicTools now carries the 9 real registered tool names with a non-tautological drift test. WR-04 transcript fix confirmed: testdata/transcripts/ is gitignored (0 tracked .json) and the keyed run left the working tree CLEAN.

### Human Verification Required

None. All truths verified programmatically, including TEST-03 via a real keyed LLM run.

### Gaps Summary

No gaps. All 21 must-haves verified against the actual codebase. The phase goal is achieved: the embedded SKILL.md ships with a verb-drift-gated decision table and real SKILL-04 token numbers, the PreToolUse nudge is a fail-open exit-0 advisory steer, and `helix setup` flips to install skill+hooks while idempotently tearing down prior MCP registration (hook-preserving, all 7 clients). The empirical close (TEST-03) ran for real and showed a 8/8 tool-selection shift toward helix. The Phase 94 boundary is intact (no daemon/forwarder/proto edits).

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_
