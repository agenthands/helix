---
status: testing
phase: 98-multi-agent-coverage-stronger-steering
source: [98-VERIFICATION.md]
started: 2026-06-22
updated: 2026-06-22
---

## Current Test

number: 1
name: Codex/Gemini live runtime round-trip (A1/A2 end-to-end)
expected: |
  `helix setup codex` installs AGENTS.md (sentinel block, ≤32 KiB) + `.codex/hooks.json`
  with command ["<bin>","nudge"]; a real Codex session loads AGENTS.md and fires the
  PreToolUse hook invoking `helix nudge` (advisory additionalContext, no deny).
  `helix setup gemini-cli` installs GEMINI.md (no hook); a real Gemini CLI session loads it.
awaiting: user response

## Tests

### 1. Codex/Gemini live runtime round-trip (A1/A2 end-to-end)
expected: Codex loads AGENTS.md + fires .codex/hooks.json PreToolUse → `helix nudge`; Gemini loads GEMINI.md. Byte-shapes pinned from documented official conventions (HIGH confidence) but not exercised against a live install.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
