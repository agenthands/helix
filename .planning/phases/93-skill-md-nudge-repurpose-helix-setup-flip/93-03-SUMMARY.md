---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
plan: 03
subsystem: cli
tags: [setup, mcp-teardown, agent-skill, claude-code-hooks, idempotent, client-registrar]
status: complete

# Dependency graph
requires:
  - phase: 93-skill-md-nudge-repurpose-helix-setup-flip
    plan: 01
    provides: installSkill(targetDir) + skillTargetDir(claudeDir) + embeddedSkillMD (the embedded Agent Skill asset and atomic, containment-guarded writer this flip wires into setup)
provides:
  - teardownMCP per registrar (MCP-only, hook-preserving, best-effort no-op) on the ClientRegistrar interface
  - "helix setup <client>" flipped from MCP-registration to skill+hooks install with prior-MCP teardown, idempotently across all 7 clients
  - --no-skill flag + RegistrationConfig.NoSkill (mirrors --no-hooks)
  - teardownOnlyRegister shared helper for non-Claude clients (MCP-teardown only, no skill consumer)
affects: [94 (MCP head deletion — daemon forwarder/tool-handlers still intact after this plan)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP-only teardown that deliberately omits removeHooksFromSettings (Pitfall 3 / T-93-06): the same Register both removes the MCP entry and keeps the hooks"
    - "Best-effort teardown: missing config file/key, absent CLI → no-op, never a hard error (reuses already-idempotent removeFromJSONConfig)"
    - "Claude-family clients consume Agent Skills (skill write); non-Claude clients are MCP-teardown-only via a shared helper"

key-files:
  created: []
  modified:
    - internal/cli/setup_clients.go
    - internal/cli/setup.go
    - internal/cli/setup_test.go

key-decisions:
  - "Chose plan option (a): a per-registrar teardownMCP method (added to the ClientRegistrar interface) reusing each client's existing (path,key,\"helix\") removeFromJSONConfig triple, rather than a shared dispatcher — keeps each client's config-path resolution local and avoids a giant switch."
  - "claude-code teardown covers BOTH the project .mcp.json and the global ~/.claude/settings.json mcpServers paths, plus a best-effort `claude mcp remove` when the CLI is present, so a prior entry written via any path is removed; never hard-errors on a missing CLI/entry."
  - "Removed now-dead MCP-add machinery (serverConfigJSON, GeminiCLI.ensureEnabled) since the flip drops all MCP-server writes; kept mergeJSONConfig (still exercised by tests)."
  - "Non-Claude clients (gemini-cli, vscode, jetbrains, opencode, generic) became MCP-teardown-only via a shared teardownOnlyRegister helper — they have no Agent Skill consumer, so no skill is written (93-RESEARCH Open Q2)."
  - "Generic client no longer prints an MCP config to stdout (the flip removes the only stdout-writing registrar); with --output it removes any prior helix entry from the file."

patterns-established:
  - "Pattern: each Register = teardownMCP (hook-preserving) -> installSkill (Claude-family only) -> mergeHooksIntoSettings (claude-code only); all three honor DryRun."
  - "Pattern: tests force the deterministic direct-file path and isolate HOME via t.Setenv so the suite stays hermetic (no real claude/gemini CLI dependency, no user-home mutation)."

# Metrics
metrics:
  duration: ~9 min
  completed: 2026-06-21
  tasks: 2
  files-modified: 3
---

# Phase 93 Plan 03: `helix setup` Flip (skill + hooks, MCP-teardown) Summary

Flipped `helix setup <client>` from "register an MCP server" to "install the Agent Skill + hooks and tear down any prior MCP registration" — idempotently across all 7 supported clients (SKILL-02). Each registrar gained an MCP-only, hook-preserving `teardownMCP`; Claude-family `Register`s now teardown → install skill → merge hooks, while non-Claude `Register`s do MCP-teardown only. The daemon MCP head (forwarder dial path, tool handlers, `--mode http`) is untouched — Phase 94 owns its deletion.

## What Was Built

**Task 1 — `teardownMCP` per client (commit `93c11092`):**
- Added `teardownMCP(cfg RegistrationConfig) error` to the `ClientRegistrar` interface and implemented it on all 7 registrars.
- Each implementation reuses the client's existing `removeFromJSONConfig(path, key, "helix")` triple (already idempotent: missing file/key → no-op) and deliberately omits `removeHooksFromSettings` (Pitfall 3 / T-93-06).
- claude-code: project `.mcp.json` + global `~/.claude/settings.json` removal + best-effort `claude mcp remove` (only if the CLI is present). gemini-cli also marks helix disabled in the enablement file (best-effort). generic stdout config is a documented no-op; `--output` file removes the entry.

**Task 2 — wire teardown + skill into each `Register`; orchestration (commit `df6adee2`):**
- claude-code `Register`: `teardownMCP` → `installSkill(skillTargetDir(claudeDir(...)))` → `mergeHooksIntoSettings` (hook failure non-fatal per D-16). Dropped the `claude mcp add-json` / `mergeJSONConfig` MCP write entirely.
- claude-desktop `Register`: teardown → install skill into the global `~/.claude` skills dir (no hook installer here).
- gemini-cli / vscode / jetbrains / opencode / generic `Register`: MCP-teardown only via shared `teardownOnlyRegister` (no Agent Skill consumer; prints an Info line that the client does not consume Agent Skills).
- `setup.go`: rewrote command `Short`/`Long` help to describe the new behavior, added `--no-skill` flag plumbed into `RegistrationConfig.NoSkill`, updated success/failure messaging ("set up" not "registered").
- Removed now-dead `serverConfigJSON` and `GeminiCLIRegistrar.ensureEnabled` (MCP-add machinery the flip no longer uses).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Rewrote obsolete MCP-write Register tests**
- **Found during:** Task 2
- **Issue:** `TestVSCodeRegistrarRegister`, `TestJetBrainsRegistrarRegister`, `TestGenericRegistrarRegister`, and `TestGenericRegistrarStdout` asserted the *old* behavior (Register writes an MCP entry / prints MCP JSON to stdout), which the flip intentionally removes — they failed after the rewrite.
- **Fix:** Rewrote each to assert the new flip behavior (teardown removes a pre-seeded entry; non-Claude Register writes no skill; generic stdout is a no-op). This is required to complete the task, not a scope expansion.
- **Files modified:** internal/cli/setup_test.go
- **Commit:** df6adee2

**2. [Rule 1 - Cleanup] Removed dead MCP-add helpers**
- **Found during:** Task 2
- **Issue:** After dropping all MCP-server writes, `serverConfigJSON` and `GeminiCLIRegistrar.ensureEnabled` became unreferenced dead code.
- **Fix:** Deleted both. `mergeJSONConfig` was retained (still used by the merge-config tests). Build/vet confirm no remaining references and no unused imports.
- **Files modified:** internal/cli/setup_clients.go
- **Commit:** df6adee2

## Test Hermeticity Notes

Tests that touch claude-code/gemini teardown isolate `HOME` via `t.Setenv` so the global `~/.claude/settings.json` removal and any best-effort `claude mcp remove` operate on a sandbox, never the user's real home. The flip tests force the deterministic direct-file path so assertions do not depend on the presence of a real `claude`/`gemini` binary.

## Verification

- `go test ./internal/cli/ -run 'Setup|Teardown|ClientRegistry|Unregister' -count=1` → green.
- `go test ./internal/cli/ -count=1` → green; `go test ./...` → green (no FAIL).
- `go vet ./internal/cli/...` → clean; `go build ./...` → succeeds.
- `git diff go.mod go.sum` → empty (zero new deps); `git diff api/proto/` → empty (zero proto change).
- Scope confirmed: `git diff --name-only HEAD~2` touches only `internal/cli/{setup.go,setup_clients.go,setup_test.go}` — no daemon/forwarder/proto edits (Phase 94 owns MCP head deletion).

## Threat Mitigations Applied

- **T-93-06 (teardown stripping hooks):** `teardownMCP` omits `removeHooksFromSettings`; `TestTeardownPriorMCP_PreservesHooks` asserts a seeded `helix_managed` hook survives.
- **T-93-07 (clobbering unmanaged entries):** all writes go through `removeFromJSONConfig`/`mergeHooksIntoSettings` (read→mutate map→Marshal); the "other" entry is asserted preserved.
- **T-93-08 (non-idempotent re-run):** `TestSetupFlip_Idempotent` asserts byte-stable SKILL.md + settings.json and exactly one `helix_managed` matcher per event after a re-run.
- **T-93-09 (skill write escaping skills dir):** `installSkill`'s containment guard (from 93-01) is reused unchanged; target derived from the resolved client `.claude` dir.

## Known Stubs

None.

## Self-Check: PASSED
- internal/cli/setup_clients.go — FOUND
- internal/cli/setup.go — FOUND
- internal/cli/setup_test.go — FOUND
- commit 93c11092 — FOUND
- commit df6adee2 — FOUND
