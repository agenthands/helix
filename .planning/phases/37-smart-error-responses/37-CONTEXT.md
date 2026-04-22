# Phase 37: Smart Error Responses - Context

**Gathered:** 2026-04-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Agents receive actionable parameter corrections when they misuse MCP tools. Error enrichment middleware wraps the existing typed error taxonomy (`internal/errors/`) with "did you mean" suggestions for parameter name typos and invalid enum values. Covers SERR-01, SERR-02, SERR-03.

</domain>

<decisions>
## Implementation Decisions

### Suggestion Matching Strategy
- **D-01:** Use Levenshtein distance + exact substring match on known parameter names per tool. A suggestion fires when the edit distance is below a threshold (e.g., 2) or the unknown parameter is a substring of a valid one.
- **D-02:** Cover both parameter name typos (fuzzy match against known params) and value corrections (for enum/boolean fields with known valid values from JSON Schema). Value correction only applies to fields with a closed set of valid values.

### Error Enrichment Architecture
- **D-03:** Implement as a new MCP receiving middleware in `internal/mcp/` that runs after the tool handler returns. It intercepts `CallToolResult` with `IsError=true` and enriches the text content with suggestions. This wraps the existing error taxonomy without modifying error kinds.
- **D-04:** Append suggestion as a separate line to existing error text — never mutate the original error message. Format: original error text + newline + suggestion line.

### Suggestion Payload Format
- **D-05:** Plain text suggestion appended to error response, e.g., `Did you mean: relative_path (instead of file_path)?`
- **D-06:** Show only the top 1 best suggestion — multiple candidates add noise for agents. If no suggestion meets the confidence threshold, return the original error unchanged.

### Parameter Knowledge Source
- **D-07:** Introspect the MCP SDK's registered tool schemas at middleware init time. Build a map of `tool_name -> valid_param_names` from each Tool's InputSchema (JSON Schema properties). This is computed once at startup, not per-request.
- **D-08:** Extract enum constraints from JSON Schema for fields that have them, enabling value correction suggestions (e.g., `direction` field with values `["incoming", "outgoing"]`).

### Scope Constraints
- **D-09:** Suggestions only correct parameters within the same tool — never redirect to a different tool (SERR-02 requirement).
- **D-10:** Enrichment middleware wraps existing typed error taxonomy — no new error kinds are introduced (SERR-03 requirement).

### Claude's Discretion
- Levenshtein distance threshold tuning (2-3 is typical)
- Whether to also suggest when parameters are valid but might be confused (e.g., `path` vs `relative_path`)
- Internal helper package structure (inline in middleware file vs separate `internal/mcp/suggest/` package)
- Unit test strategy and test fixture design

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The key UX target is that agents can self-correct on the next call without human intervention.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Error taxonomy
- `internal/errors/errors.go` — Typed Error struct with Kind, Message, Tool, Detail fields and builder pattern
- `internal/errors/kinds.go` — 7 error kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout) with sentinel errors

### Middleware architecture
- `internal/mcp/middleware.go` — Existing TelemetryMiddleware and ProfileFilterMiddleware patterns, outcome classification, middleware installation via InstallMiddleware()

### Tool registration and args
- `internal/kernel/symbols/tools.go` — 9 symbol tools with typed Args structs and errorResult() helper pattern
- `internal/kernel/edit/tools.go` — 6 edit tools with typed Args structs
- `internal/kernel/fileops/tools.go` — 6 file operation tools with typed Args structs
- `internal/kernel/diag/tools.go` — 3 diagnostic tools with typed Args structs
- `internal/kernel/health/tools.go` — Health tool with typed Args struct
- `internal/mcp/registry.go` — ToolRegistry with Register/Names/Count interface

### MCP server wiring
- `internal/daemon/daemon.go` — InstallMiddleware call site, skill init, tool registration orchestration

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/errors/Error` struct: Already has `Kind`, `Message`, `Tool`, `Detail` fields — the `Detail` field could carry suggestion text naturally
- `errorResult()` helper in `internal/kernel/symbols/tools.go`: All tools use this pattern for error responses
- `mcpsdk.Middleware` type: Established middleware chain pattern used by TelemetryMiddleware and ProfileFilterMiddleware
- `ToolRegistry.Names()`: Can enumerate all registered tool names for building the parameter knowledge map

### Established Patterns
- Middleware wraps `MethodHandler` and intercepts `tools/call` method specifically (see TelemetryMiddleware)
- Tools return `(*mcpsdk.CallToolResult, any, error)` where `IsError=true` signals a tool-level error
- All tool args are Go structs with `json:"field_name"` tags — these are the canonical parameter names
- Error taxonomy uses builder pattern: `serr.New(serr.InvalidArgs, "msg").WithTool("name").WithDetail("detail")`

### Integration Points
- `InstallMiddleware()` in `internal/mcp/middleware.go` — new middleware registers here
- `internal/daemon/daemon.go` — calls InstallMiddleware, passes the SDK server, registry, and logger
- Tool JSON Schemas are defined inline in `mcpsdk.AddTool()` calls via `&mcpsdk.Tool{InputSchema: ...}`

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 37-smart-error-responses*
*Context gathered: 2026-04-22*
