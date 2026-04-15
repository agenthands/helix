# Phase 23: Tool Migration - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate all 38+ MCP tools from raw `fmt.Errorf`/`errors.New` error strings to typed `serr.New(kind, msg)` errors using the taxonomy created in Phase 22. Every error return path in every tool package must go through the typed error constructors. Remove deprecated sentinel re-exports once migration is complete.

</domain>

<decisions>
## Implementation Decisions

### Migration Strategy
- **D-01:** Split migration into 8 plans by tool group: symbols (9 tools), edit (6), fileops (6), diag (3), memory (7), workflow (2), profile (2), MCP core (3). Each plan is self-contained and can execute in parallel waves.

### Kind Mapping
- **D-02:** Map raw errors to the 7 Kind values by semantic intent — classify based on what the error MEANS, not rigid category rules. E.g., "workspace not activated" -> NoWorkspace (missing prerequisite), "file outside workspace" -> InvalidArgs (bad input from caller). Claude uses judgment per error site based on context.

### Sentinel Cleanup
- **D-03:** Remove deprecated re-exports in `internal/mcp/errors.go` (ErrSessionExpired, ErrWorkspaceNotReady, ErrToolNotAvailable, ErrProjectNotFound, ErrLSCrashed, ErrorDetail) and `internal/kernel/lspool/circuit_err.go` (ErrCircuitOpen re-export) during this phase. Once all tools import `serr` directly, the bridge aliases are dead code. Update any remaining references to use `serr.ErrXxx` directly.

### Error Message Style
- **D-04:** Normalize error message strings during migration: lowercase, no trailing punctuation, consistent verb form (e.g., "workspace not activated" not "Workspace is not activated!"). Agents match on Kind; messages are for human consumption.
- **D-05:** Tool name excluded from message text — use `.WithTool("tool_name")` for tool attribution. Messages stay generic and reusable (e.g., "symbol not found" not "find_symbol: symbol not found").

### Claude's Discretion
- Exact Kind classification per error site (guided by semantic intent principle)
- Whether to consolidate duplicate error messages within a tool group
- Order of tool groups across plans/waves
- Whether to add `.WithDetail()` for error sites that already carry useful context (e.g., file paths, symbol names)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Error Taxonomy (Phase 22 output)
- `internal/errors/kinds.go` -- 7 Kind constants and sentinel errors
- `internal/errors/errors.go` -- Error struct, New(), WithTool(), WithDetail() builders

### Deprecated Sentinels (to remove)
- `internal/mcp/errors.go` -- Bridge aliases and deprecated ErrorDetail struct
- `internal/kernel/lspool/circuit_err.go` -- ErrCircuitOpen re-export

### Tool Packages (migration targets)
- `internal/kernel/symbols/` -- 9 symbol retrieval tools (~13 error sites)
- `internal/kernel/edit/` -- 6 symbol editing tools (~30 error sites)
- `internal/kernel/fileops/` -- 6 file operation tools (~33 error sites)
- `internal/kernel/diag/` -- 3 diagnostic tools (~8 error sites)
- `internal/skill/memory/` -- 7 memory tools (~21 error sites)
- `internal/skill/workflow/` -- 2 workflow tools (~1 error site)
- `internal/profile/` -- 2 profile tools (~31 error sites in loader.go + skill.go)
- `internal/mcp/` -- MCP core tools (~3 error sites in test files)

### Requirements
- `.planning/REQUIREMENTS.md` -- MIG-01 through MIG-08

### Phase 22 Context
- `.planning/phases/22-error-taxonomy/22-CONTEXT.md` -- Error taxonomy design decisions (D-01 through D-05)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/errors/` package -- fully built in Phase 22 with Kind type, Error struct, builders, and sentinels
- `serr.New(kind, msg).WithTool().WithDetail()` builder chain ready for use
- `errors.Is()` works with Kind-based matching via Error.Is() method

### Established Patterns
- Phase 22 already migrated `internal/kernel/lspool/circuit.go` -- use as reference for how to convert raw errors to `serr.New()` calls
- `internal/mcp/errors.go` shows the re-export bridge pattern that will be removed
- `fmt.Errorf("...", %w)` wrapping is the current pattern at ~115 sites; each becomes `serr.New(kind, msg).WithDetail(...)`

### Integration Points
- All tool packages import `serr` and use typed constructors
- Test files that check error strings will need updating (golden files, string assertions)
- `internal/mcp/` middleware already understands `*serr.Error` for MCP response conversion

</code_context>

<specifics>
## Specific Ideas

No specific requirements -- user consistently chose recommended (simplest) options, indicating preference for mechanical, predictable migration with normalization.

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 23-tool-migration*
*Context gathered: 2026-04-15*
