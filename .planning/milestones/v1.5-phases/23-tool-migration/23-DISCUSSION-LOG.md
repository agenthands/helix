# Phase 23: Tool Migration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 23-tool-migration
**Areas discussed:** Migration strategy, Kind mapping, Sentinel cleanup, Error message style

---

## Migration Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| By tool group (Recommended) | One plan per tool group: symbols (9), edit (6), fileops (6), diag (3), memory (7), workflow (2), profile (2), MCP core (3). 8 plans, each self-contained. | :white_check_mark: |
| By layer | 3 plans: kernel tools (24), skill tools (11), MCP core (3). Fewer plans, coarser grouping. | |
| Single plan | One large plan migrating everything. Simplest to track, but large diff. | |

**User's choice:** By tool group
**Notes:** Allows parallel execution in waves.

---

## Kind Mapping

| Option | Description | Selected |
|--------|-------------|----------|
| Semantic intent (Recommended) | Map by what the error MEANS. Claude uses judgment per error site. | :white_check_mark: |
| Strict categories | Define rigid rules upfront for each category. | |
| You decide | Let Claude classify with review in code review. | |

**User's choice:** Semantic intent
**Notes:** None.

---

## Sentinel Cleanup

| Option | Description | Selected |
|--------|-------------|----------|
| Remove in this phase (Recommended) | Once all tools import serr directly, the re-exports are dead code. Clean removal. | :white_check_mark: |
| Keep through Phase 24 | Leave re-exports until test migration. More conservative. | |
| You decide | Claude determines per-sentinel based on remaining references. | |

**User's choice:** Remove in this phase
**Notes:** None.

---

## Error Message Style

| Option | Description | Selected |
|--------|-------------|----------|
| Normalize (Recommended) | Standardize to lowercase, no trailing punctuation, consistent verb form. | :white_check_mark: |
| Preserve verbatim | Keep existing message strings exactly. Purely mechanical wrap. | |
| You decide | Normalize where cheap, preserve where it would cause test churn. | |

**User's choice:** Normalize

### Follow-up: Tool Name in Messages

| Option | Description | Selected |
|--------|-------------|----------|
| WithTool() only (Recommended) | Message is generic. Tool context comes from .WithTool(). | :white_check_mark: |
| Tool in message | Message includes tool name. Self-contained but redundant. | |

**User's choice:** WithTool() only
**Notes:** Keeps messages reusable, avoids duplication with the Tool field.

---

## Claude's Discretion

- Exact Kind classification per error site
- Whether to consolidate duplicate error messages within a tool group
- Order of tool groups across plans/waves
- Whether to add .WithDetail() for sites with useful context

## Deferred Ideas

None.
