# Phase 26: Fuzzy Edit Integration - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire the Phase 25 fuzzy engine (`internal/fuzzy/`) into the existing edit tool stack and expose a standalone `fuzzy_edit` MCP tool. Three requirements land here: FUZZ-04 (standalone tool), FUZZ-05 (replace_symbol_body fallback), FUZZ-06 (replace_content fallback). No changes to the fuzzy engine itself — this phase is purely integration and tool registration.

</domain>

<decisions>
## Implementation Decisions

### Fallback Behavior
- **D-01:** Auto-fallback — when exact matching fails in `replace_symbol_body` or `replace_content`, automatically try fuzzy matching. No agent-side opt-in needed. Report `match_strategy` in response so agents know fuzzy was used.
- **D-02:** `replace_symbol_body` gets a new optional `search_body` parameter. When provided AND tree-sitter body extraction succeeds, fuzzy-match `search_body` within the extracted body region, then apply `new_body` as replacement. When `search_body` is absent, behavior is unchanged (full body replace via tree-sitter range).
- **D-03:** `replace_content` fuzzy fallback activates ONLY when `is_regex=false` and literal string match returns 0 results. Regex failures stay as-is — fuzzy matching a regex pattern is nonsensical.

### Standalone fuzzy_edit Tool API
- **D-04:** Parameters: `path` (string, required), `search` (string, required), `replacement` (string, required), `allow_ellipsis` (bool, default true).
- **D-05:** Tool lives in `internal/kernel/fileops/` alongside `replace_content`. It operates on raw file content without symbol awareness — natural fit with other file operations.
- **D-06:** Tool reads file, calls `fuzzy.Match()`, writes result if match succeeds. Atomic write via existing `OverwriteFile`. Reports strategy/score in response.

### Response Reporting
- **D-07:** All three tools include `match_strategy` and `similarity_score` as structured fields in the JSON response when fuzzy matching is used. Agents can programmatically detect fuzzy usage and adjust behavior.
- **D-08:** When exact matching succeeds (no fuzzy fallback needed), existing response format is preserved — no strategy/score fields added for exact-match success paths.

### Claude's Discretion
- Error message formatting for fuzzy fallback failures (reuse engine's `serr.InvalidArgs` detail as-is or wrap with tool-specific context)
- Whether `fuzzy_edit` response includes the matched text region for agent inspection
- Integration test structure and fixture design

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Fuzzy Engine (Phase 25 output)
- `internal/fuzzy/types.go` — Strategy enum, Options, Result structs (the API surface)
- `internal/fuzzy/match.go` — Match() entry point, cascade logic, segmented dispatch
- `internal/fuzzy/indent.go` — Indentation reflow (called by engine, not by callers)

### Integration Points (modify in this phase)
- `internal/kernel/edit/replace.go` — ReplaceBody/ReplaceBodyWithPlan (FUZZ-05 fallback wiring)
- `internal/kernel/edit/tools.go` — ReplaceBodyArgs struct, registerReplaceBody (add search_body param)
- `internal/kernel/fileops/replace.go` — ReplaceInFile (FUZZ-06 fallback wiring)
- `internal/kernel/fileops/tools.go` — ReplaceInFileArgs struct, RegisterTools (add fuzzy_edit registration)

### Typed Errors (v1.5 convention)
- `internal/errors/` — serr package; fuzzy engine already returns `serr.InvalidArgs` for ambiguity/failure

### Prior Phase Context
- `.planning/phases/25-fuzzy-edit-engine/25-CONTEXT.md` — Engine design decisions (algorithm, segmentation, ambiguity, indentation)
- `.planning/REQUIREMENTS.md` — FUZZ-04, FUZZ-05, FUZZ-06

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/fuzzy/` — Complete fuzzy engine with `Match(source, search, opts)` API, no I/O
- `internal/kernel/fileops/replace.go:ReplaceInFile()` — Existing literal/regex replace with atomic write
- `internal/kernel/edit/replace.go:ReplaceBodyWithPlan()` — Tree-sitter body extraction + byte range replacement
- `internal/kernel/fileops/validate.go:ValidatePath()` — Path validation for workspace-relative paths
- `internal/kernel/fileops/replace.go:OverwriteFile()` — Atomic file write utility

### Established Patterns
- Tool registration via `RegisterTools()` + typed args structs + `mcpsdk.AddTool`
- Tool handlers wrapped with `kernel.WrapToolSpan` for tracing (Phase 12 convention)
- Error responses use `serr` typed errors, converted to MCP error responses at tool boundary
- Tool response via `textResult()` / `errorResult()` helpers

### Integration Points
- `internal/kernel/edit/tools.go:RegisterTools()` — add `search_body` to ReplaceBodyArgs, update handler
- `internal/kernel/fileops/tools.go:RegisterTools()` — add `registerFuzzyEdit` call
- `internal/daemon/imports.go` — no changes needed (fileops and edit already imported)
- Profile YAMLs — `fuzzy_edit` tool needs to appear in relevant profiles

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 26-fuzzy-edit-integration*
*Context gathered: 2026-04-16*
