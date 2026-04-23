---
phase: 43-cross-doc-sync
plan: 04
subsystem: docs
tags: [changelog, setup-cli, truth-sync]
requires: []
provides:
  - "CHANGELOG v1.7 accurately reflects 7-client setup registry"
affects:
  - CHANGELOG.md
tech-stack:
  added: []
  patterns: []
key-files:
  created: []
  modified:
    - CHANGELOG.md
decisions:
  - "Only the v1.7 Setup CLI bullet was rewritten; v1.3 and v1.0 historical entries preserved verbatim because they describe the state at their own release and remain truthful."
metrics:
  duration: "~1 min"
  completed: "2026-04-23"
  tasks: 1
  files: 1
requirements:
  - CLOG-01
  - CLOG-02
---

# Phase 43 Plan 04: CHANGELOG Setup-CLI Client Count Fix Summary

One-liner: Corrected CHANGELOG v1.7 Setup CLI entry to report 7 clients (adding OpenCode) so it matches `internal/cli/setup_clients.go:37-46`, closing F-07 and the CHANGELOG component of F-01.

## What Changed

- CHANGELOG.md line 8 updated from "for 6 clients — Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, and generic MCP clients" to "for 7 clients — Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, OpenCode, and generic MCP clients".
- No other CHANGELOG lines touched: v1.3 manual-config entry (OpenCode/Cursor/Antigravity) and v1.0 "6 file operation tools" line are historically accurate and remain as-is.

## Tasks Completed

| Task | Name                                                    | Commit   | Files         |
| ---- | ------------------------------------------------------- | -------- | ------------- |
| 1    | Rewrite CHANGELOG v1.7 Setup CLI bullet (7-client list) | 71eb4406 | CHANGELOG.md  |

## Verification

```
$ grep "for 6 clients" CHANGELOG.md                           # empty
$ grep -c "for 7 clients" CHANGELOG.md                        # 1
$ grep -c "Gemini CLI, OpenCode, and generic" CHANGELOG.md    # 1
$ grep -c "6 file operation tools" CHANGELOG.md               # 1 (historical v1.0 line preserved)
```

All four acceptance checks pass.

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- FOUND: CHANGELOG.md modification at line 8 ("for 7 clients ... Gemini CLI, OpenCode, and generic MCP clients")
- FOUND: commit 71eb4406 in git log
