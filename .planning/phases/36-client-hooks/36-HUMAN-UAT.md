---
status: partial
phase: 36-client-hooks
source: [36-VERIFICATION.md]
started: 2026-04-21T20:00:00Z
updated: 2026-04-21T20:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. End-to-End Setup Hook Installation
expected: `serena setup claude-code` writes SessionStart, PreToolUse, and Stop hook entries into .claude/settings.json
result: [pending]

### 2. SessionStart Auto-Activation
expected: Starting a new Claude Code session in a project with Serena setup triggers daemon auto-start and workspace activation
result: [pending]

### 3. PreToolUse Nudge Behavior
expected: After 5+ grep/read calls without Serena symbolic tool usage, nudge message appears in Claude context
result: [pending]

### 4. Stop Hook Session Cleanup
expected: Ending a Claude Code session triggers deactivate which removes session-stats.json
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
