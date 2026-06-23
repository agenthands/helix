---
status: complete
phase: 98-multi-agent-coverage-stronger-steering
source: [98-VERIFICATION.md]
started: 2026-06-22
updated: 2026-06-22
---

## Current Test

[testing complete]

## Tests

### 1. Codex/Gemini live runtime round-trip (A1/A2 end-to-end)
expected: Codex loads AGENTS.md + fires .codex/hooks.json PreToolUse → `helix nudge`; Gemini loads GEMINI.md. Byte-shapes pinned from documented official conventions (HIGH confidence) but not exercised against a live install.
result: issue
reported: "Live Codex run hit: failed to parse hooks config ~/.codex/hooks.json: unknown field `SessionStart`, expected `hooks`. Investigation: the parse error is from a PRE-EXISTING ~/.codex/hooks.json (Jun 11, written by GSD's own Codex tooling in the stale Claude-style schema), NOT from helix. Our `helix setup codex` output is schema-correct (top-level `hooks` wrapper, command [bin,nudge], no deny, AGENTS.md 1635B/1 sentinel — proven). HOWEVER the live test surfaced a real Helix gap: writeCodexHooks OVERWRITES the whole hooks.json instead of merging, so it would clobber a user's existing Codex hooks (against AGENT-03's no-clobber goal)."
severity: major
root_cause: "writeCodexHooks (internal/cli/setup_agents.go) does os.WriteFile(fresh)→rename = full overwrite, unlike the idempotent sentinel-append used for AGENTS.md (AGENT-02). No merge into an existing Codex hooks.json."

## Summary

total: 1
passed: 0
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "helix setup codex installs the PreToolUse nudge hook WITHOUT clobbering a user's existing Codex hooks.json (other events / other matchers preserved)"
  status: failed
  reason: "writeCodexHooks overwrites the entire file; surfaced by live Codex test where a pre-existing ~/.codex/hooks.json (GSD tooling) coexists. Helix schema itself is correct."
  severity: major
  test: 1
  artifacts: ["internal/cli/setup_agents.go (writeCodexHooks)", "internal/cli/setup_agents_test.go"]
  missing: ["merge-into-existing-hooks.json logic (preserve other events/matchers)", "idempotent re-run (structural match on matcher=Bash + command ends with nudge, NOT an extra helix_managed field — Codex schema is strict)", "no-clobber for legacy/foreign existing file (back up to .bak rather than silently overwrite)", "tests: merge-preserves-others, idempotent-no-dup, legacy-backed-up, fresh-when-absent"]
