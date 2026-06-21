---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/cli/skill.go
  - internal/cli/nudge.go
  - internal/cli/setup.go
  - internal/cli/setup_clients.go
  - internal/cli/skills/helix/SKILL.md
  - test/oracle/llm/skill_trigger_test.go
  - test/oracle/llm/prompt.go
findings:
  critical: 2
  warning: 5
  info: 3
  total: 10
status: issues_found
---

# Phase 93: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 93 (embedded SKILL.md + repurposed PreToolUse nudge + `helix setup` flip + LLM behavioral oracle) is well-structured and the security-sensitive paths are mostly sound: the nudge parses Bash command strings as DATA via `strings.Fields` (no `os/exec`, bounded, no ReDoS), it always exits 0, `installSkill` is atomic (temp+rename), and the JSON config writers use `encoding/json` round-trips rather than string concatenation. The SKILL.md decision table is accurate — all 50 cited verbs match the canonical `verbs_gen.go` registry exactly, and the 599-byte idle-cost claim verifies.

However the review surfaced two BLOCKER-tier correctness defects and several quality/robustness gaps:

1. **`ClaudeCodeRegistrar.Unregister` aborts before hook removal** when `claude mcp remove` fails — which is now the *common* case post-flip, since setup no longer registers an MCP entry. `--uninstall` therefore orphans hooks.
2. **The `helixSymbolicTools` detection map carries stale/wrong tool names** (4 of 9 never match the live MCP registry), so the symbolic-tool counter-reset is broken — and the test codifies the wrong names, making it a tautological green.

Plus: uninstall never removes the installed skill file, the containment guard is a weaker segment-substring check than its comment claims, the nudge's threshold comments/`Long` description no longer match the new always-emit behavior, the committed LLM transcripts regenerate non-deterministically on every keyed run (dirties the tree), and `mergeJSONConfig` is now dead production code.

---

## Critical Issues

### CR-01: `Unregister` returns early on `claude mcp remove` failure, orphaning hooks

**File:** `internal/cli/setup_clients.go:266-275`
**Issue:** Post-Phase-93, `Register` no longer registers an MCP server entry. So when a user runs `helix setup claude-code --uninstall`, `claude mcp remove helix --scope ...` will fail with a non-zero exit (no such MCP entry to remove). `Unregister` treats that as a hard error and returns at line 274 *before* reaching the hook-removal block (lines 277-287). Result: `--uninstall` fails and leaves the SessionStart/PreToolUse/Stop hooks installed — the exact teardown the flag promises. This is inconsistent with the new sibling `teardownMCP`, which deliberately ignores the same command's exit (`_ = rmCmd.Run()`, line 235) precisely because an absent entry is normal.
**Fix:** Make MCP removal best-effort in `Unregister` (mirror `teardownMCP`), then always proceed to hook removal:
```go
if _, err := exec.LookPath("claude"); err == nil {
    cmd := exec.Command("claude", cmdArgs...)
    cmd.Stdout = os.Stderr
    cmd.Stderr = os.Stderr
    _ = cmd.Run() // absent MCP entry is a no-op, not a fatal error
}
// also remove the direct-file entries (.mcp.json / global settings.json),
// matching teardownMCP, then continue to hook + skill removal below.
```

### CR-02: `helixSymbolicTools` map uses stale tool names — symbolic-tool detection silently broken

**File:** `internal/cli/nudge.go:244-255`
**Issue:** The map keys do not match the tool names the live MCP server actually registers (verified against `internal/kernel/symbols/`):
- `find_symbol` → no such tool; the real name is `search_symbols`
- `get_symbol_details` → no such tool at all
- `get_symbols_overview` (plural "symbols") → real name is `get_symbol_overview` (singular)
- `get_blast_radius` → real name is `analyze_blast_radius`

Only 5 of the 9 entries (`find_references`, `get_hover_info`, `find_implementations`, `get_call_hierarchy`, `get_type_hierarchy`) match reality. These names appear carried over from the pre-rename (Serena-lineage) codebase and were never updated. Consequence: when the agent actually uses `search_symbols` / `get_symbol_overview` / `analyze_blast_radius`, `isHelixSymbolicTool` returns false, so `runNudge` does NOT reset `GrepReadCount` (nudge.go:85-90, the D-12 reset). The "agent is using symbolic tools, stop nudging" signal is broken for the most common symbol verbs. Worse, the unit test (`internal/cli/nudge_test.go:61-73`) asserts the *stale* names are recognized, so it is a tautological test that locks in the bug rather than catching it.
**Fix:** Replace the map with the real registered tool names and update the test:
```go
var helixSymbolicTools = map[string]bool{
    "search_symbols":       true,
    "get_symbol_overview":  true,
    "find_references":      true,
    "get_hover_info":       true,
    "find_implementations": true,
    "get_call_hierarchy":   true,
    "get_type_hierarchy":   true,
    "analyze_blast_radius": true,
    "go_to_definition":     true,
}
```
Source the names from the canonical registry rather than hand-typing, and rewrite `nudge_test.go:61-73` to assert these real names (the test currently encodes the defect).

---

## Warnings

### WR-01: `--uninstall` never removes the installed SKILL.md

**File:** `internal/cli/setup_clients.go:253-290` (and there is no `uninstallSkill` helper anywhere)
**Issue:** Phase 93 makes the Agent Skill the primary artifact `setup` installs (`installSkill` writes `<claudeDir>/skills/helix/SKILL.md`). `Unregister` removes hooks and attempts MCP removal, but no code path deletes the skill file (confirmed: no `removeSkill`/`os.Remove(...SKILL.md)` exists). After `--uninstall` the skill remains active in the agent's context. The `--uninstall` flag help (setup.go:39) advertises removing "Helix registration from the client (MCP entry and hooks)" — it should also remove the skill now that the skill is what setup installs.
**Fix:** Add an `uninstallSkill(skillDir)` that removes `<skillDir>/SKILL.md` (and the now-empty `skills/helix` dir), call it from `ClaudeCodeRegistrar.Unregister` and `ClaudeDesktopRegistrar.Unregister`, and update the `--uninstall` flag help text.

### WR-02: `withinSkillRoot` is a segment-substring check, not real path containment

**File:** `internal/cli/skill.go:159-173`
**Issue:** The comment (skill.go:124-131) claims `installSkill` "refuses any targetDir that is not within the per-client skills root." But `withinSkillRoot` only checks whether the cleaned path contains the consecutive segments `skills`/`helix` *anywhere*. Verified by direct test: `/etc/skills/helix` and `../../../etc/skills/helix` both return `true`, and `/home/u/.claude/skills/helix/../../../../../tmp/skills/helix` collapses to `/tmp/skills/helix` and also passes. It is not currently attacker-reachable because the only caller constructs `targetDir` via `filepath.Join(claudeDir, "skills", "helix")`, which always ends in those literal segments (so the guard is a no-op that always passes for legitimate input). The defect is that the guard provides materially weaker protection than documented and would not stop an absolute-path escape — a latent gap if a future caller passes a less-trusted `targetDir`.
**Fix:** Validate true containment against an explicit root, e.g.:
```go
func withinSkillRoot(root, targetDir string) bool {
    rel, err := filepath.Rel(root, targetDir)
    return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
```
and pass the resolved per-client `skills/helix` root explicitly (the same `filepath.Rel` + `..`-prefix pattern the comment says it mirrors from `render.go`).

### WR-03: Nudge threshold comments and command help misdescribe the new always-emit behavior

**File:** `internal/cli/nudge.go:38, 50-51, 25` (and the `sessionStats` counters generally)
**Issue:** The Phase 93 advisory now fires on *every* grep/Read call and every Bash-on-code call (nudge.go:100-104) — there is no longer any `GrepReadCount >= 5` gate in `runNudge` (grep confirms the threshold is never consulted to gate emission). Yet `runNudge`'s doc comment still says "outputs a nudge message after 5+ calls without symbolic tool usage" (line 50-51) and the cobra `Long` says it nudges "when agents overuse grep/read" (line 38). Both describe the removed threshold behavior and now misrepresent the tool: it nudges on the *first* grep/read. The `GrepReadCount`/`HelixToolCount` counters are still tracked and persisted but no longer influence any output decision — they are effectively dead, misleading state (and CR-02's broken reset makes that worse).
**Fix:** Update the `runNudge` comment and the cobra `Long` to describe the per-call advisory behavior; either remove the now-unused threshold counters or wire them back into an emission gate if throttling is still intended (the SUMMARY/D-11 should state which).

### WR-04: Committed LLM transcripts regenerate non-deterministically, dirtying the working tree on every keyed run

**File:** `test/oracle/llm/transcript.go:31-49` + `test/oracle/llm/skill_trigger_test.go:74-142` (writes to `harness.ProjectRoot()/test/oracle/llm/testdata/transcripts/`); fixtures are git-TRACKED (verified `git check-ignore` → TRACKED)
**Issue:** `WriteTranscript` writes each transcript into the in-tree, committed `testdata/transcripts/` directory, and the `Response` field is the raw live-LLM output (skill_trigger_test.go:119,137) plus model-dependent blob byte counts (lines 80-82). Every run of the `llm`-tagged oracle with an API key overwrites these committed JSON files with fresh, non-deterministic model output, leaving the working tree dirty after a test run. Committing non-deterministic test output as fixtures is an anti-pattern: it produces spurious diffs, invites accidental commits of one run's idiosyncratic output, and makes the fixtures meaningless as golden references (nothing replays/asserts against them — `ReadTranscript` exists but no test in scope consumes these). Severity is WARNING (not BLOCKER): the test gating itself is honest (`SkipWithoutAPIKey` is the first line, `//go:build llm`, aggregate assertion is non-vacuous), so this is a hygiene/repo-cleanliness defect, not a false-green.
**Fix:** Write transcripts to a per-run temp dir (`t.TempDir()`) or `os.MkdirTemp`, OR gitignore `test/oracle/llm/testdata/transcripts/` and remove the committed copies. If they must be retained as artifacts, write them under a path that `.gitignore` excludes and document that they are run artifacts, not golden fixtures.

### WR-05: Oracle's `mentionsHelix` substring check can be fooled by prose mentioning "helix"

**File:** `test/oracle/llm/skill_trigger_test.go:20-22`
**Issue:** `mentionsHelix` returns true if the lowercased response contains the substring `"helix "` anywhere. The system prompt asks for "ONLY the single command line" (prompt.go:148), but models drift; a baseline/skill response like `# use helix for this` or `the helix tool would...` (prose, not an executed command) counts as a helix selection, and conversely a response wrapped without a trailing space (`helix\n`, or `` `helix` ``) is missed. Because the central deliverable assertion is `shifted > 0` and `skillHelix*2 > n` (lines 154-159), a loosely-matched "helix" mention can inflate the shift count and weaken the oracle's honesty. The committed baseline fixture shows the model emitting bare `rg ...` commands, so drift is plausible.
**Fix:** Tighten the detector to require the command to *start with* `helix ` (after trimming/stripping a leading fence or `$`), e.g. `strings.HasPrefix(strings.TrimSpace(strings.Trim(resp, "`$ ")), "helix ")`, mirroring the "ONLY the command line" contract.

---

## Info

### IN-01: `mergeJSONConfig` is now dead production code

**File:** `internal/cli/setup_clients.go:57-91`
**Issue:** After the flip, no `Register`/`teardownMCP` path writes an MCP server entry, so `mergeJSONConfig` is referenced only by `setup_test.go` (verified). It is dead production code retained only by its own tests.
**Fix:** Remove `mergeJSONConfig` and its now-orphaned tests, or document why it is intentionally retained (e.g., pending Phase 94). Dead code that exists only to satisfy its own test is misleading.

### IN-02: Classifier counts a grep *pattern* that looks like a code file as a file operand

**File:** `internal/cli/nudge.go:337-376`
**Issue:** `classifyBashTarget` cannot distinguish a grep regex from a file path. `grep main.go` (where `main.go` is the search pattern, not a file) is classified `isCode=true`, producing a spurious — but harmless, advisory-only — helix tip. Given the fail-open, advisory-only contract this is acceptable; flagged only for completeness. The conservative mixed-operand demotion and dir-only/no-operand fail-open paths are all correct (verified by direct testing).
**Fix:** Optional — for `grep`/`egrep`/`fgrep`/`ag`/`rg`, skip the first non-flag token (the pattern) before scanning for file operands. Low priority; current behavior is within the stated tolerance.

### IN-03: `--no-skill` is silently ignored for teardown-only clients

**File:** `internal/cli/setup_clients.go:130-138, 302-303, 411-413, 463-465, 607-609, 664-666`
**Issue:** For gemini-cli/vscode/jetbrains/opencode/generic, `Register` delegates to `teardownOnlyRegister`, which never writes a skill — so `--no-skill` is a no-op for them. This matches intent (those clients don't consume skills) but the flag is accepted without comment, which could confuse a user who passes it expecting a behavioral difference.
**Fix:** Optional — either note in the `--no-skill` flag help that it applies only to Claude-family clients (the flag help at setup.go:44 already says "Claude-family clients only", so this is mostly covered; just ensure no misleading success message implies a skill was considered).

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
