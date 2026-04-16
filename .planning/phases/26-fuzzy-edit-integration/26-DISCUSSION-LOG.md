# Phase 26: Fuzzy Edit Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-16
**Phase:** 26-fuzzy-edit-integration
**Areas discussed:** Fallback behavior, Standalone fuzzy_edit tool API, Response reporting

---

## Fallback Behavior

### Q1: How should fuzzy fallback activate?

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-fallback | If exact match fails, automatically try fuzzy. Report match_strategy in response. Zero agent-side changes needed. | ✓ |
| Opt-in parameter | Add `fuzzy: true` parameter to both tools. Agent must explicitly request fuzzy matching. | |
| Always fuzzy | Route ALL matching through the fuzzy engine (exact is strategy 1 anyway). | |

**User's choice:** Auto-fallback
**Notes:** Preferred zero-friction approach — existing tool calls get smarter silently.

### Q2: How should replace_symbol_body get a search block for fuzzy matching?

| Option | Description | Selected |
|--------|-------------|----------|
| Add search_body param | Optional `search_body` parameter. When provided AND tree-sitter fails to match, fuzzy-match within extracted body. | ✓ |
| Use new_body as search hint | Treat agent's new_body as containing both old and new code. | |
| Fuzzy only in replace_content | Skip fuzzy fallback for replace_symbol_body entirely. | |

**User's choice:** Add search_body param
**Notes:** Clean separation — search_body is what to find, new_body is what to replace with.

### Q3: Should replace_content fuzzy fallback respect is_regex flag?

| Option | Description | Selected |
|--------|-------------|----------|
| Fuzzy only on literal fail | Fall back to fuzzy only when is_regex=false and literal match returns 0. | ✓ |
| Fuzzy on both | Fall back to fuzzy for both literal and regex failures. | |

**User's choice:** Fuzzy only on literal fail
**Notes:** Fuzzy matching a regex pattern doesn't make sense.

---

## Standalone fuzzy_edit Tool API

### Q1: What parameter set?

| Option | Description | Selected |
|--------|-------------|----------|
| Practical set | path + search + replacement + allow_ellipsis (bool, default true) | ✓ |
| Minimal | path + search + replacement only. Ellipsis always enabled. | |
| Full control | path + search + replacement + allow_ellipsis + dry_run + min_score | |

**User's choice:** Practical set
**Notes:** Covers FUZZ-04 fully. Ellipsis is the only toggle agents realistically need.

### Q2: Where should the tool be registered?

| Option | Description | Selected |
|--------|-------------|----------|
| In fileops package | Lives alongside replace_content in internal/kernel/fileops/ | ✓ |
| New fuzzyedit package | New internal/kernel/fuzzyedit/ package | |
| In edit package | Lives in internal/kernel/edit/ alongside replace_symbol_body | |

**User's choice:** In fileops package
**Notes:** Operates on raw file content without symbol awareness — natural fit.

---

## Response Reporting

### Q1: What should tool responses include when fuzzy matching kicks in?

| Option | Description | Selected |
|--------|-------------|----------|
| Structured fields | Include match_strategy and similarity_score as top-level JSON fields | ✓ |
| Text annotation only | Append a line like '[matched via whitespace_normalized, score: 0.95]' | |
| Silent success | Normal success response with no fuzzy indication | |

**User's choice:** Structured fields
**Notes:** Agents can programmatically detect fuzzy usage and adjust behavior.

---

## Claude's Discretion

- Error message formatting for fuzzy fallback failures
- Whether fuzzy_edit response includes matched text region
- Integration test structure and fixture design

## Deferred Ideas

None — discussion stayed within phase scope.
