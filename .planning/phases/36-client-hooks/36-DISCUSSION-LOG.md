# Phase 36: Client Hooks - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-21
**Phase:** 36-client-hooks
**Areas discussed:** Hook installation mechanism, SessionStart activation behavior, PreToolUse nudge strategy, Stop cleanup scope
**Mode:** --auto (all decisions auto-selected)

---

## Hook Installation Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Extend ClaudeCodeRegistrar | Write hooks into settings.json alongside MCP registration | ✓ |
| Separate hook install command | Dedicated `serena hooks install` subcommand | |
| Manual instructions only | Print hook config for user to paste | |

**User's choice:** [auto] Extend ClaudeCodeRegistrar — hooks are part of standard setup
**Notes:** Keeps setup as single command. Uninstall removes hooks too. --no-hooks flag for opt-out.

---

## SessionStart Activation Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Shell command (serena activate) | New CLI subcommand that auto-starts daemon and activates workspace | ✓ |
| MCP tool call | Call activate_project via MCP in hook | |
| Daemon socket ping | Direct socket/gRPC call from hook script | |

**User's choice:** [auto] Shell command — consistent with CLI-first pattern from Phase 34
**Notes:** Idempotent activation. Reuses forwarder auto-start for daemon lifecycle.

---

## PreToolUse Nudge Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Counter-based threshold | Track grep/read calls, nudge after 5+ without symbolic tool usage | ✓ |
| Always nudge | Return nudge on every grep/read call | |
| Pattern-based | Only nudge when grep pattern looks like symbol search | |

**User's choice:** [auto] Counter-based threshold — balances helpfulness with noise
**Notes:** Advisory only (exit 0 always). Counter resets when symbolic tools used. Session-scoped tracking.

---

## Stop Cleanup Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Session data only | Clean counter files and temp markers, leave daemon running | ✓ |
| Full workspace deactivation | Deactivate workspace in daemon and clean local files | |
| Daemon shutdown | Stop daemon if no other sessions | |

**User's choice:** [auto] Session data only — daemon serves multiple sessions
**Notes:** Silent success if daemon already stopped. New deactivate subcommand.

---

## Claude's Discretion

- Hook JSON structure in settings.json (follow Claude Code documented format)
- Counter file format within .serena/
- Nudge message wording
- activate/deactivate subcommand nesting
- Edge case error handling

## Deferred Ideas

None — discussion stayed within phase scope
