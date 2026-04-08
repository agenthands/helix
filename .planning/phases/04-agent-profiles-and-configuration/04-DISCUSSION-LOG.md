# Phase 4: Agent Profiles and Configuration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.

**Date:** 2026-04-08
**Phase:** 04-agent-profiles-and-configuration
**Areas discussed:** Profile contents, Mode switching, Token budget

---

## Profile Contents

**User's choice:** Heavily curated + full/vanilla fallback.
**Notes:** "Profiles should be opinionated enough to noticeably change agent behavior, but never so opinionated that users can't switch to a neutral full-access profile." 5 profiles: claude-code, codex, ide-assistant, ci-bot, full.

## Mode Switching

**User's choice:** Explicit switch_mode MCP tool, dynamic registry under the hood.
**Notes:** 4 modes: read, edit, review, admin. "switch_mode being the only way to cross those boundaries." Per-session isolation, clean audit trail.

## Token Budget

**User's choice:** Aggregate get_token_budget tool, not per-tool schema metadata.
**Notes:** "MCP's Tool shape has no standard token-size field. An aggregate budget is closer to what actually matters operationally." Response includes per-tool breakdown for debugging.

## Claude's Discretion

- Specific tool subsets per profile
- Prompt override content
- Mode transition validation rules

## Deferred Ideas

None.
