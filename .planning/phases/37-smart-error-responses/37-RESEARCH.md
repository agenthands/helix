# Phase 37: Smart Error Responses - Research

**Researched:** 2026-04-22
**Domain:** MCP middleware, string similarity, JSON Schema introspection
**Confidence:** HIGH

## Summary

Phase 37 adds "did you mean" suggestions to MCP tool error responses when agents pass wrong parameter names or invalid enum values. The implementation is a new receiving middleware in `internal/mcp/` that intercepts error responses and enriches them with suggestions computed via Levenshtein distance against a pre-built parameter knowledge map.

The architecture is well-constrained by CONTEXT.md decisions: middleware pattern (D-03), append-only suggestion format (D-04/D-05), single best suggestion (D-06), startup schema introspection (D-07/D-08), and scope boundaries (D-09/D-10). The codebase has an established middleware pattern (TelemetryMiddleware, ProfileFilterMiddleware) that this phase directly follows.

**Primary recommendation:** Implement a `SuggestionMiddleware` in `internal/mcp/` that builds a `map[string]ToolParamInfo` at init time from registered tool schemas, then intercepts both protocol errors (`jsonrpc2.ErrInvalidParams`) and tool errors (`IsError=true`) containing parameter mismatch signals, enriching them with Levenshtein-based "did you mean" suggestions.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Use Levenshtein distance + exact substring match on known parameter names per tool. Suggestion fires when edit distance < threshold (e.g., 2) or unknown param is substring of a valid one.
- **D-02:** Cover both parameter name typos (fuzzy match against known params) and value corrections (for enum/boolean fields with known valid values from JSON Schema).
- **D-03:** Implement as new MCP receiving middleware in `internal/mcp/` that runs after the tool handler returns. Intercepts `CallToolResult` with `IsError=true` and enriches text content with suggestions. Wraps existing error taxonomy without modifying error kinds.
- **D-04:** Append suggestion as a separate line to existing error text -- never mutate the original error message. Format: original error text + newline + suggestion line.
- **D-05:** Plain text suggestion appended to error response, e.g., `Did you mean: relative_path (instead of file_path)?`
- **D-06:** Show only the top 1 best suggestion. If no suggestion meets confidence threshold, return original error unchanged.
- **D-07:** Introspect MCP SDK's registered tool schemas at middleware init time. Build map of `tool_name -> valid_param_names` from each Tool's InputSchema. Computed once at startup, not per-request.
- **D-08:** Extract enum constraints from JSON Schema for fields that have them, enabling value correction suggestions.
- **D-09:** Suggestions only correct parameters within the same tool -- never redirect to a different tool (SERR-02).
- **D-10:** Enrichment middleware wraps existing typed error taxonomy -- no new error kinds (SERR-03).

### Claude's Discretion
- Levenshtein distance threshold tuning (2-3 is typical)
- Whether to also suggest when parameters are valid but might be confused (e.g., `path` vs `relative_path`)
- Internal helper package structure (inline in middleware file vs separate `internal/mcp/suggest/` package)
- Unit test strategy and test fixture design

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SERR-01 | Error responses include "did you mean" suggestions when agents misuse tool parameters | Middleware intercepts error responses and appends Levenshtein-matched suggestions for parameter names and enum values |
| SERR-02 | Error responses suggest parameter corrections (never redirect to different tools) | D-09 constraint enforced by middleware only looking up params for the tool named in the CallToolRequest |
| SERR-03 | Error enrichment middleware wraps existing typed error taxonomy with suggestion payloads | D-10 constraint -- middleware appends to error text, no new error kinds in `internal/errors/kinds.go` |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Parameter name suggestion | MCP Middleware | -- | Runs post-handler in the middleware chain, same tier as TelemetryMiddleware |
| Enum value suggestion | MCP Middleware | -- | Same middleware intercepts enum mismatches from schema validation errors |
| Tool schema introspection | MCP Middleware (init) | ToolRegistry | Schema map built at middleware init from registered tools |
| Error text enrichment | MCP Middleware | -- | Append-only mutation of error text content in CallToolResult |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| MCP Go SDK | v0.8.0 | Middleware types, CallToolResult, Tool schema | Already in project deps [VERIFIED: go.mod] |
| stdlib `encoding/json` | go1.25.1 | Parse InputSchema `any` field to extract properties | Zero-dependency JSON parsing [VERIFIED: go version] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Hand-rolled Levenshtein | N/A | Edit distance computation (~15 lines) | Parameter name and enum value fuzzy matching |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled Levenshtein | github.com/agnivade/levenshtein | External dep for 15 lines of code -- not worth it |
| Post-handler middleware | Pre-handler validation | D-03 specifies post-handler; pre-handler would duplicate SDK validation |

**No new dependencies required.** Levenshtein distance is a 15-line function with no edge cases worth importing a library for.

## Architecture Patterns

### System Architecture Diagram

```
Agent Request (tools/call)
    |
    v
[TelemetryMiddleware] -- metrics + logging + deadline
    |
    v
[ProfileFilterMiddleware] -- tools/list filtering (pass-through for tools/call)
    |
    v
[SuggestionMiddleware] -- NEW: intercepts error responses
    |
    v
[SDK callTool handler] -- schema validation + tool dispatch
    |
    v
[Tool Handler] -- business logic, returns CallToolResult
    |
    v (response flows back up)
    |
[SuggestionMiddleware] <-- intercepts here on way back
    |
    | if err != nil && is InvalidParams:
    |   parse "unexpected additional properties" from error text
    |   extract bad param names -> fuzzy match against tool's known params
    |   convert protocol error to CallToolResult{IsError:true} with suggestion
    |
    | if result.IsError && text contains param mismatch signals:
    |   extract param names -> fuzzy match -> append suggestion line
    |
    v
[TelemetryMiddleware] <-- classifies outcome, emits metrics
    |
    v
Agent receives enriched error response
```

### Recommended Project Structure

```
internal/mcp/
    suggest.go          # SuggestionMiddleware + ToolSchemaMap builder
    suggest_lev.go      # levenshteinDistance function + matchParam/matchValue helpers
    suggest_test.go     # Unit tests for middleware, fuzzy matching, schema extraction
```

**Rationale for inline (not separate package):** The middleware needs access to `mcpsdk.Middleware`, `mcpsdk.CallToolRequest`, `mcpsdk.CallToolResult` and the `extractToolName` helper already in `internal/mcp/middleware.go`. A separate `internal/mcp/suggest/` sub-package would create unnecessary import ceremony for a tightly coupled feature. [ASSUMED]

### Pattern 1: Receiving Middleware (Post-Handler Interception)

**What:** MCP SDK's `Middleware` type wraps `MethodHandler` and can intercept both request and response. The suggestion middleware calls `next()` first, then inspects the result/error.

**When to use:** When you need to enrich responses without modifying the request path.

**Example:**
```go
// Source: internal/mcp/middleware.go (existing TelemetryMiddleware pattern)
func SuggestionMiddleware(schemaMap *ToolSchemaMap, logger *slog.Logger) mcpsdk.Middleware {
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            // Only intercept tools/call
            if method != "tools/call" {
                return next(ctx, method, req)
            }
            
            result, err := next(ctx, method, req)
            
            // Case 1: Protocol error from SDK schema validation
            if err != nil {
                return schemaMap.EnrichProtocolError(req, result, err)
            }
            
            // Case 2: Tool error (IsError=true)
            if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil && ctr.IsError {
                schemaMap.EnrichToolError(req, ctr)
            }
            
            return result, err
        }
    }
}
```

### Pattern 2: Schema Map Initialization at Startup

**What:** Build parameter knowledge map once from SDK's ListTools result (internal call). The `Tool.InputSchema` field is `any` but JSON-marshals to a JSON Schema object with `properties` and optional `enum` constraints.

**When to use:** At middleware installation time, after all tools are registered.

**Example:**
```go
// Source: derived from MCP SDK Tool.InputSchema type [VERIFIED: protocol.go:927]
type ToolParamInfo struct {
    ValidParams []string              // known parameter names
    EnumValues  map[string][]string   // param_name -> valid enum values
}

type ToolSchemaMap struct {
    tools map[string]*ToolParamInfo
}

// BuildFromTools constructs the schema map from registered tool definitions.
// InputSchema is any; remarshal to map[string]any to extract properties.
func BuildToolSchemaMap(tools []*mcpsdk.Tool) *ToolSchemaMap {
    m := &ToolSchemaMap{tools: make(map[string]*ToolParamInfo)}
    for _, t := range tools {
        info := &ToolParamInfo{}
        // Marshal InputSchema -> JSON -> unmarshal to map
        data, err := json.Marshal(t.InputSchema)
        if err != nil { continue }
        var schema map[string]any
        if err := json.Unmarshal(data, &schema); err != nil { continue }
        
        props, ok := schema["properties"].(map[string]any)
        if !ok { continue }
        
        for name, propVal := range props {
            info.ValidParams = append(info.ValidParams, name)
            // Extract enum values if present
            if propMap, ok := propVal.(map[string]any); ok {
                if enumVals, ok := propMap["enum"].([]any); ok {
                    strs := make([]string, 0, len(enumVals))
                    for _, v := range enumVals {
                        if s, ok := v.(string); ok { strs = append(strs, s) }
                    }
                    if len(strs) > 0 {
                        if info.EnumValues == nil { info.EnumValues = make(map[string][]string) }
                        info.EnumValues[name] = strs
                    }
                }
            }
        }
        m.tools[t.Name] = info
    }
    return m
}
```

### Pattern 3: Levenshtein Distance

**What:** Standard dynamic programming edit distance, comparing two strings character by character.

**Example:**
```go
// levenshteinDistance returns the minimum edit distance between two strings.
func levenshteinDistance(a, b string) int {
    if len(a) == 0 { return len(b) }
    if len(b) == 0 { return len(a) }
    
    prev := make([]int, len(b)+1)
    curr := make([]int, len(b)+1)
    for j := range prev { prev[j] = j }
    
    for i := 1; i <= len(a); i++ {
        curr[0] = i
        for j := 1; j <= len(b); j++ {
            cost := 1
            if a[i-1] == b[j-1] { cost = 0 }
            curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
        }
        prev, curr = curr, prev
    }
    return prev[len(b)]
}
```

### Anti-Patterns to Avoid
- **Modifying error kinds:** D-10 explicitly forbids adding new error kinds. The middleware only appends text.
- **Cross-tool suggestions:** D-09 forbids suggesting a different tool. Only match against the tool named in the request.
- **Per-request schema introspection:** D-07 requires one-time startup computation. Never call ListTools on every request.
- **Multiple suggestions:** D-06 requires exactly 1 or 0 suggestions. Multiple candidates add noise for agents.
- **Mutating original error text:** D-04 requires append-only. Always `originalText + "\n" + suggestion`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema parsing | Custom schema parser | `json.Marshal` + `json.Unmarshal` to `map[string]any` | SDK's InputSchema is `any`; standard JSON roundtrip extracts properties reliably |
| MCP middleware chain | Custom interceptor | `mcpsdk.Middleware` type + `server.AddReceivingMiddleware` | Established pattern, integrates with SDK's chain ordering |

**Key insight:** Levenshtein distance IS appropriate to hand-roll here. It's a textbook algorithm, 15 lines, zero edge cases for ASCII parameter names. Adding a dependency would be over-engineering.

## Common Pitfalls

### Pitfall 1: Protocol Errors vs Tool Errors
**What goes wrong:** The middleware only checks `CallToolResult.IsError` and misses SDK schema validation failures which come back as `error` (not result).
**Why it happens:** The MCP SDK's `AddTool` with typed args generates schemas with `additionalProperties: false`. When an agent sends unknown params, the SDK itself rejects with `jsonrpc2.ErrInvalidParams` before the tool handler runs. This is a protocol error (`err != nil`), not a tool error (`IsError=true`). [VERIFIED: server.go:263-266, validate.go:456-464]
**How to avoid:** The middleware MUST handle both paths: (1) `err != nil` containing "unexpected additional properties" from SDK validation, and (2) `result.IsError == true` from tool handlers returning validation errors.
**Warning signs:** Tests only cover IsError path; agents get raw JSON-RPC errors for typos.

### Pitfall 2: Schema Map Timing
**What goes wrong:** SuggestionMiddleware is installed before all tools are registered, resulting in an incomplete schema map.
**Why it happens:** In `daemon.go`, `InstallMiddleware` is called at step 14, but kernel tools are registered at step 10 and skill tools at step 11. The map must be built AFTER steps 10-11.
**How to avoid:** Either (a) build the schema map lazily on first tools/call request (sync.Once), or (b) ensure `InstallMiddleware` is called after all tool registrations, or (c) pass a function that builds the map lazily.
**Warning signs:** Some tools don't get suggestions while others do.

### Pitfall 3: Converting Protocol Errors to Tool Errors
**What goes wrong:** When the middleware intercepts a protocol error (from SDK validation) and wants to enrich it with a suggestion, it must convert it to a `CallToolResult{IsError:true}` because protocol errors bypass the LLM's error-handling path. The MCP spec says tool errors should use IsError, not protocol errors. [VERIFIED: protocol.go:94-105]
**Why it happens:** Agents cannot self-correct from protocol errors because the client typically shows them as connection/protocol failures rather than tool failures.
**How to avoid:** When enriching a protocol error with a suggestion, return `(&CallToolResult{IsError:true, Content:[TextContent{enriched_msg}]}, nil)` instead of the original `(nil, err)`.
**Warning signs:** Agents get JSON-RPC error frames instead of readable error text with suggestions.

### Pitfall 4: Case Sensitivity in Parameter Matching
**What goes wrong:** Agent sends `Path` instead of `path`, Levenshtein says distance=1, but JSON unmarshal into Go struct is case-insensitive by default.
**Why it happens:** `json.Unmarshal` in Go does case-insensitive key matching for struct fields.
**How to avoid:** Compare parameter names case-insensitively when computing distance. Or only trigger suggestions when the SDK has actually rejected the parameter (i.e., only enrich existing errors, never create new ones).
**Warning signs:** False positive suggestions for parameters that would have worked.

### Pitfall 5: Skill Tools with `map[string]any` Args
**What goes wrong:** Skill tools registered via `AddSkillTool` use `map[string]any` as the args type, which generates a permissive schema (`additionalProperties` not set to false). The SDK won't reject unknown params for these tools.
**Why it happens:** `AddSkillTool` calls `mcpsdk.AddTool[map[string]any, any]` -- the inferred schema for `map[string]any` has `type: "object"` with no properties constraint. [VERIFIED: server.go:230-231]
**How to avoid:** For skill tools, the middleware can still help with IsError responses that mention parameter issues, but won't get SDK validation errors for unknown params. Document this limitation.
**Warning signs:** Memory/workflow tools never get suggestions for typos.

## Code Examples

### Extracting Bad Parameter Names from SDK Error

```go
// Source: derived from jsonschema validate.go:464 error format [VERIFIED]
// Error format: 'validating "arguments": unexpected additional properties ["file_path"]'
func extractBadParams(errMsg string) []string {
    const prefix = `unexpected additional properties [`
    idx := strings.Index(errMsg, prefix)
    if idx < 0 { return nil }
    rest := errMsg[idx+len(prefix):]
    end := strings.Index(rest, "]")
    if end < 0 { return nil }
    // Parse quoted strings: "file_path" "other_param"
    raw := rest[:end]
    var params []string
    for _, part := range strings.Split(raw, " ") {
        p := strings.Trim(part, `"`)
        if p != "" { params = append(params, p) }
    }
    return params
}
```

### Finding Best Suggestion

```go
// bestSuggestion returns the best matching parameter name and its distance,
// or ("", maxInt) if no match meets the threshold.
func bestSuggestion(unknown string, validParams []string, maxDistance int) (string, int) {
    best := ""
    bestDist := maxDistance + 1
    
    for _, valid := range validParams {
        // Exact substring match (D-01)
        if strings.Contains(valid, unknown) || strings.Contains(unknown, valid) {
            d := levenshteinDistance(unknown, valid)
            if d < bestDist {
                best, bestDist = valid, d
            }
            continue
        }
        d := levenshteinDistance(unknown, valid)
        if d <= maxDistance && d < bestDist {
            best, bestDist = valid, d
        }
    }
    return best, bestDist
}
```

### Middleware Installation in daemon.go

```go
// In daemon.go, after step 14 (InstallMiddleware):
// Build suggestion schema map from all registered tools.
// Must happen AFTER steps 10-11 (tool registration).
schemaMap := serenaMCP.BuildToolSchemaMapFromServer(mcpServer.SDK())
serenaMCP.InstallSuggestionMiddleware(mcpServer.SDK(), schemaMap, logger)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Raw error strings | Typed error taxonomy (serr package) | v1.0 Phase 5 | Programmatic error matching, but no suggestions |
| No middleware | TelemetryMiddleware + ProfileFilterMiddleware | v1.2 Phase 8/11 | Established middleware pattern to follow |

**Current gap:** Error responses for parameter typos are either raw JSON-RPC protocol errors or terse `invalid_args: missing required field: X` messages. Neither tells the agent what the correct parameter name is.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Levenshtein threshold of 2 is sufficient for parameter name typos across 42 tools | Architecture Patterns | May miss valid suggestions (too strict) or produce false positives (too loose) -- tunable |
| A2 | Inline in `internal/mcp/` (not separate sub-package) is the right structure | Project Structure | Low risk -- easy to refactor later if the file grows too large |
| A3 | SDK schema validation error format `unexpected additional properties ["X"]` is stable across versions | Pitfall 1 | If error format changes in SDK upgrade, param extraction regex breaks -- mitigated by defensive parsing |

## Open Questions

1. **How to get tool list from SDK server without a client session?**
   - What we know: `Server.tools` is unexported `*featureSet[*serverTool]`. There's no public `Server.ListTools()` method callable from the server side. `listTools` is unexported.
   - What's unclear: Whether we need to call through the server's internal handler or build our own list.
   - Recommendation: Build the schema map from `ToolRegistry` by extending `ToolDef` to capture InputSchema at registration time. Alternative: use the fact that `listTools` is reachable through the middleware chain itself -- the middleware can call the handler with a synthetic `ListToolsRequest` at init. Best approach: extend `ToolRegistry.Register` to also store the `*mcpsdk.Tool` pointer, then iterate at middleware init.

2. **Should protocol errors be converted to tool errors with suggestions?**
   - What we know: SDK returns `jsonrpc2.ErrInvalidParams` for schema validation failures. Agents handle tool errors (`IsError=true`) better than protocol errors.
   - Recommendation: YES -- convert to `CallToolResult{IsError:true}` when we can enrich with a suggestion. This gives the agent a chance to self-correct. When we have no suggestion, let the protocol error pass through unchanged.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (go1.25.1) |
| Config file | None needed (standard `go test`) |
| Quick run command | `go test ./internal/mcp/... -count=1 -run TestSuggest` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SERR-01 | Middleware suggests "did you mean" for param name typos | unit | `go test ./internal/mcp/... -run TestSuggestMiddleware_ParamTypo -count=1` | Wave 0 |
| SERR-01 | Middleware suggests "did you mean" for invalid enum values | unit | `go test ./internal/mcp/... -run TestSuggestMiddleware_EnumValue -count=1` | Wave 0 |
| SERR-02 | Suggestions only reference same tool's params | unit | `go test ./internal/mcp/... -run TestSuggestMiddleware_SameToolOnly -count=1` | Wave 0 |
| SERR-03 | No new error kinds introduced | unit | `go test ./internal/errors/... -run TestErrorKinds -count=1` | Already exists |
| SERR-03 | Original error text preserved, suggestion appended | unit | `go test ./internal/mcp/... -run TestSuggestMiddleware_AppendOnly -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/mcp/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green + `go vet ./...`

### Wave 0 Gaps
- [ ] `internal/mcp/suggest_test.go` -- covers SERR-01, SERR-02, SERR-03
- [ ] Levenshtein unit tests in `internal/mcp/suggest_lev_test.go` or inline

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | no | -- |
| V5 Input Validation | yes | Middleware only reads existing error text and request params; never executes or evaluates them |
| V6 Cryptography | no | -- |

### Known Threat Patterns for MCP middleware

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Information disclosure via suggestion text | Information Disclosure | Suggestions only reference parameter names already in the tool's public schema -- no internal state exposed |
| Denial of service via crafted param names | Denial of Service | Levenshtein is O(n*m) but param names are short (<50 chars) and the map is bounded by tool count (~42) -- negligible cost |

## Sources

### Primary (HIGH confidence)
- MCP Go SDK v0.8.0 source code -- `protocol.go`, `server.go`, `tool.go`, `features.go` (read directly from `go/pkg/mod`)
- `internal/mcp/middleware.go` -- existing TelemetryMiddleware and ProfileFilterMiddleware patterns
- `internal/errors/errors.go` and `kinds.go` -- error taxonomy
- `google/jsonschema-go@v0.3.0/jsonschema/validate.go` -- validation error format for `additionalProperties: false`
- `google/jsonschema-go@v0.3.0/jsonschema/infer_test.go` -- confirms `ForType` generates `additionalProperties: falseSchema()` for struct types

### Secondary (MEDIUM confidence)
- `internal/daemon/daemon.go` -- tool registration ordering and middleware installation

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new deps, all code paths verified in SDK source
- Architecture: HIGH -- follows established middleware pattern exactly, all integration points inspected
- Pitfalls: HIGH -- verified SDK validation behavior, error formats, and schema generation in source code

**Research date:** 2026-04-22
**Valid until:** 2026-05-22 (stable domain, no external API dependencies)
