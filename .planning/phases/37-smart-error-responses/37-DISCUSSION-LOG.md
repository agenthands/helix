# Phase 37: Smart Error Responses - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-22
**Phase:** 37-smart-error-responses
**Areas discussed:** Suggestion matching strategy, Error enrichment architecture, Suggestion payload format, Parameter knowledge source
**Mode:** --auto (all decisions auto-selected as recommended defaults)

---

## Suggestion Matching Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Levenshtein + substring | Fuzzy match with edit distance threshold plus substring matching | ✓ |
| Exact prefix match only | Only suggest when unknown param starts with a valid param prefix | |
| Jaro-Winkler similarity | More sophisticated string similarity metric | |

**User's choice:** [auto] Levenshtein + substring (recommended default)
**Notes:** Covers both parameter name typos and value corrections for enum/boolean fields.

---

## Error Enrichment Architecture

| Option | Description | Selected |
|--------|-------------|----------|
| New receiving middleware | Intercepts CallToolResult with IsError=true, enriches text | ✓ |
| Modify tool handlers directly | Add suggestion logic to each tool's error path | |
| Post-processing in daemon | Centralized error rewriting in daemon before MCP response | |

**User's choice:** [auto] New receiving middleware (recommended default)
**Notes:** Appends suggestion as separate line, never mutates original error message. Aligns with SERR-03 requirement.

---

## Suggestion Payload Format

| Option | Description | Selected |
|--------|-------------|----------|
| Plain text appended | "Did you mean: X (instead of Y)?" as extra line | ✓ |
| Structured JSON hint | JSON object with suggestion metadata | |
| Markdown formatted | Bold/italic suggestion text | |

**User's choice:** [auto] Plain text appended (recommended default)
**Notes:** Top 1 suggestion only. No suggestion if confidence too low.

---

## Parameter Knowledge Source

| Option | Description | Selected |
|--------|-------------|----------|
| Introspect SDK schemas at init | Build param map from registered Tool.InputSchema once at startup | ✓ |
| Hardcoded param registry | Manually maintain known params per tool | |
| Runtime reflection on Args structs | Use Go reflect to extract field names from typed args | |

**User's choice:** [auto] Introspect SDK schemas at init (recommended default)
**Notes:** Also extracts enum constraints from JSON Schema for value correction.

---

## Claude's Discretion

- Levenshtein distance threshold tuning
- Confused-parameter detection (e.g., path vs relative_path)
- Internal package structure
- Unit test strategy

## Deferred Ideas

None — discussion stayed within phase scope.
