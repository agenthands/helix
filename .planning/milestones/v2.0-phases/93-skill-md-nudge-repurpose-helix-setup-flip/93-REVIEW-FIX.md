---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/93-skill-md-nudge-repurpose-helix-setup-flip/93-REVIEW.md
iteration: 1
findings_in_scope: 7
fixed: 7
skipped: 0
deferred: 3
status: all_fixed
---

# Phase 93: Code Review Fix Report

**Source review:** `93-REVIEW.md`
**Scope:** Critical + Warning (the 3 Info findings deferred by request)
**Iteration:** 1

## Summary

- Findings in scope: 7 (2 critical, 5 warning)
- Fixed: 7
- Skipped: 0
- Deferred (out of scope, Info-tier): 3

Verification after all fixes:
- `gofmt -l` (touched files): clean
- `go build ./...`: OK
- `go vet ./internal/cli/...`: clean
- `go vet -tags llm ./test/oracle/llm/...`: clean
- `go vet -tags llmjudge ./test/oracle/judge/... ./test/oracle/llm/...`: clean
- `go test ./internal/cli/... -count=1`: green
- `go test -tags llm ./test/oracle/llm/...`: green (KEYED run via DEEPSEEK_API_KEY) and left the working tree CLEAN — confirming the WR-04 transcript fix.
- Invariants: `git diff go.mod go.sum` empty, `git diff api/proto/` empty.

## Fixed Issues

### CR-01: `Unregister` returned early on `claude mcp remove` failure, orphaning hooks

**Files modified:** `internal/cli/setup_clients.go`, `internal/cli/setup.go`
**Commit:** `afdd777d`
**Applied fix:** Made MCP removal best-effort in `ClaudeCodeRegistrar.Unregister` (mirrors `teardownMCP`: ignore the `claude mcp remove` exit, plus idempotent direct-file `.mcp.json`/global `settings.json` removal), then ALWAYS proceeds to hook removal and skill removal. A missing `claude` CLI is no longer fatal. `--uninstall` now tears down hooks even when no MCP entry exists (the normal post-flip case). The `ClaudeDesktopRegistrar.Unregister` MCP removal was likewise made non-fatal. (Tested by the existing `internal/cli` Unregister/hook tests, which remain green.)

### CR-02: `helixSymbolicTools` map used stale tool names — symbolic-tool detection silently broken

**Files modified:** `internal/cli/nudge.go`, `internal/cli/nudge_test.go`
**Commit:** `8978a7b4`
**Applied fix:** Replaced the 4 stale pre-rename keys (`find_symbol`, `get_symbol_details`, `get_symbols_overview`, `get_blast_radius`) with the REAL registered names (`search_symbols`, `get_symbol_overview`, `analyze_blast_radius`, `go_to_definition`, plus the 5 already-correct names). All 9 were verified against the live registry (`verbs_gen.go` `toolName` fields / `VerbToolNames()`). Rewrote the tautological `TestIsHelixSymbolicTool` to assert the real names AND that the stale names are now rejected, and added `TestHelixSymbolicTools_NoDrift`, which asserts every map key is a member of `VerbToolNames()` so a future rename can't silently rot the set.

### WR-01: `--uninstall` never removed the installed SKILL.md

**Files modified:** `internal/cli/skill.go` (helper), `internal/cli/setup_clients.go` (wiring), `internal/cli/setup.go` (help text)
**Commits:** `2d8b1082` (helper), `afdd777d` (wiring + help)
**Applied fix:** Added `uninstallSkill(targetDir)` — best-effort, path-contained removal of `<targetDir>/SKILL.md` plus pruning of the now-empty `skills/helix` dir (parent dirs left intact; missing file/dir is a no-op for idempotency). Wired it into both `ClaudeCodeRegistrar.Unregister` and `ClaudeDesktopRegistrar.Unregister`, and updated the `--uninstall` flag help to mention the skill.

### WR-02: `withinSkillRoot` was a segment-substring check, not real containment

**Files modified:** `internal/cli/skill.go`
**Commit:** `2d8b1082`
**Applied fix:** Replaced the substring scan with a real `filepath.Rel` containment check. The declared `skills/helix` root is derived from the LEXICAL (pre-`Clean`) path; the CLEANED path is then required to resolve at-or-under it (`rel == "."` or a non-`..`-escaping subpath). A `..` escape such as `<root>/skills/helix/../../../etc` now cleans OUTSIDE its declared root and is rejected — mirroring `render.go` `readSnippetLine`'s `filepath.Rel` + `..`-prefix guard. The existing `TestInstallSkillContainment` / nested-dir / write-content tests remain green.

### WR-03: Nudge threshold comments and command help misdescribed the always-emit behavior

**Files modified:** `internal/cli/nudge.go`, `internal/cli/nudge_test.go`
**Commit:** `8978a7b4`
**Applied fix:** Updated the `runNudge` doc comment, the cobra `Long`, and the `sessionStats` doc comment to describe the per-call advisory behavior (no "5+ calls" gate). The `GrepReadCount`/`HelixToolCount` counters are RETAINED and now explicitly justified as lightweight session telemetry (a symbolic-tool call still resets `GrepReadCount` per D-12) rather than an emission throttle. The two stale "threshold" tests were renamed and rewritten to assert the telemetry/round-trip + D-12 reset semantics (using a real symbolic tool name).

### WR-04: Committed LLM transcripts regenerated non-deterministically, dirtying the tree

**Files modified:** `test/oracle/llm/transcript.go`; added `test/oracle/llm/testdata/transcripts/.gitignore` + `.gitkeep`; `git rm --cached` of 98 previously-tracked `*.json`
**Commit:** `f4ef7bce`
**Applied fix:** The transcripts are non-deterministic run artifacts, not golden fixtures. `t.TempDir()` (the review's first preference) was NOT viable because the offline judge (`-tags=llmjudge`, `test/oracle/judge/judge_test.go`) reads them back via `TranscriptDir()` across a SEPARATE `go test` invocation — a temp dir is gone by then. So the second documented option was applied: gitignore `testdata/transcripts/` (keeping `.gitignore` + `.gitkeep` so the dir stays tracked) and untrack the committed copies. `WriteTranscript` still writes to the persistent path for the two-stage flow; its doc comment now states they are local run artifacts, not golden references. Verified: a KEYED `-tags=llm` run regenerates the files but leaves the working tree clean.

### WR-05: Oracle's `mentionsHelix` substring check could be fooled by prose

**Files modified:** `test/oracle/llm/skill_trigger_test.go`
**Commit:** `f4ef7bce`
**Applied fix:** Tightened `mentionsHelix` to require the first command line — after stripping a leading code fence, `$ `/`> ` shell-prompt decoration, and surrounding backticks/whitespace (via a new `firstCommandLine` helper) — to START WITH `helix `, matching the "ONLY the single command line" contract. Prose mentions ("the helix tool would...") no longer count as a helix selection.

## Deferred Issues (out of scope — Info-tier, per request)

### IN-01: `mergeJSONConfig` is now dead production code

**File:** `internal/cli/setup_clients.go:57-91`
**Reason:** Info-tier; explicitly out of scope. Note: the review flags it as dead, but removing it now risks colliding with Phase 94 (which owns the daemon-side MCP head). Left for Phase 94 to remove or document.

### IN-02: Classifier counts a grep *pattern* that looks like a code file as a file operand

**File:** `internal/cli/nudge.go:337-376`
**Reason:** Info-tier; explicitly out of scope. Harmless under the advisory-only, fail-open contract (review's own assessment).

### IN-03: `--no-skill` is silently ignored for teardown-only clients

**File:** `internal/cli/setup_clients.go`
**Reason:** Info-tier; explicitly out of scope. Matches intent (those clients don't consume skills) and the flag help already says "Claude-family clients only".

---

_Fixed: 2026-06-21_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
