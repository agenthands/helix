# Phase 37: Smart Error Responses - Pattern Map

**Mapped:** 2026-04-22
**Files analyzed:** 4 new files + 2 modified files
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/mcp/suggest.go` (NEW) | middleware | request-response | `internal/mcp/middleware.go` (ProfileFilterMiddleware) | exact |
| `internal/mcp/suggest_lev.go` (NEW) | utility | transform | None in codebase (textbook algorithm) | no-analog |
| `internal/mcp/suggest_test.go` (NEW) | test | request-response | `internal/mcp/telemetry_middleware_test.go` | exact |
| `internal/mcp/suggest_lev_test.go` (NEW) | test | transform | `internal/mcp/telemetry_middleware_test.go` | role-match |
| `internal/mcp/middleware.go` (MODIFY) | middleware | request-response | self | exact |
| `internal/daemon/daemon.go` (MODIFY) | config/wiring | N/A | self (line 318, InstallMiddleware call) | exact |

## Pattern Assignments

### `internal/mcp/suggest.go` (middleware, request-response)

**Analog:** `internal/mcp/middleware.go`

**Imports pattern** (lines 1-15):
```go
package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)
```
Note: No `serr` import needed -- this middleware does not create new error kinds (D-10). No `obs` import needed -- no metrics emitted (telemetry is handled by TelemetryMiddleware).

**Middleware signature pattern** (middleware.go lines 240-242 -- ProfileFilterMiddleware is the closest match because it also inspects results post-handler):
```go
func ProfileFilterMiddleware(resolver ProfileResolver, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
```
The SuggestionMiddleware should follow this exact closure structure. Parameters: `schemaMap *ToolSchemaMap, logger *slog.Logger`.

**Method gating pattern** (middleware.go line 248 -- ProfileFilterMiddleware gates on tools/list; SuggestionMiddleware gates on tools/call):
```go
if method != "tools/list" {
	return result, nil
}
```
For SuggestionMiddleware, use the same early-return guard but for `tools/call`:
```go
if method != "tools/call" {
	return next(ctx, method, req)
}
```

**Post-handler interception pattern** (middleware.go lines 243-246 -- ProfileFilterMiddleware calls next first, then inspects result):
```go
result, err := next(ctx, method, req)
if err != nil {
	return result, err
}
```
SuggestionMiddleware differs: it must NOT short-circuit on `err != nil` because protocol errors (SDK schema validation) are the primary enrichment target. Instead:
```go
result, err := next(ctx, method, req)
// Case 1: Protocol error (SDK schema validation failure) -- enrich and convert to tool error
if err != nil {
	return schemaMap.enrichProtocolError(req, result, err)
}
// Case 2: Tool error (IsError=true) -- append suggestion to existing text
if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil && ctr.IsError {
	schemaMap.enrichToolError(req, ctr)
}
return result, err
```

**Tool name extraction** (middleware.go lines 104-110 -- reuse existing `extractToolName` helper):
```go
func extractToolName(req mcpsdk.Request) string {
	ctr, ok := req.(*mcpsdk.CallToolRequest)
	if !ok || ctr == nil || ctr.Params == nil {
		return "unknown"
	}
	return ctr.Params.Name
}
```
This helper is already in `middleware.go` and is package-internal -- directly callable from `suggest.go`.

**CallToolResult construction pattern** (kernel/symbols/tools.go lines 107-114 -- errorResult helper):
```go
func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
```
When converting a protocol error to a tool error with suggestion, construct the result this way.

**ToolSchemaMap struct and BuildToolSchemaMap function** -- no analog exists. Follow RESEARCH.md Pattern 2 (lines 176-219) for the schema introspection approach. Key: marshal `Tool.InputSchema` (type `any`) to JSON, unmarshal to `map[string]any`, extract `properties` keys and `enum` arrays.

---

### `internal/mcp/suggest_lev.go` (utility, transform)

**Analog:** None -- hand-rolled Levenshtein is a textbook algorithm.

**Package and imports:**
```go
package mcp

import "strings"
```
Same package as the middleware. No external dependencies. Follow RESEARCH.md Pattern 3 (lines 228-248) for the Levenshtein implementation.

**Exported vs unexported:** All functions in this file should be unexported (package-internal):
- `levenshteinDistance(a, b string) int`
- `bestParamSuggestion(unknown string, validParams []string, maxDistance int) (string, int)`
- `bestValueSuggestion(value string, validValues []string, maxDistance int) (string, int)`
- `extractBadParams(errMsg string) []string`

Follow RESEARCH.md code examples (lines 303-347) for `extractBadParams` and `bestSuggestion`.

---

### `internal/mcp/suggest_test.go` (test, request-response)

**Analog:** `internal/mcp/telemetry_middleware_test.go`

**Package declaration** (telemetry_middleware_test.go line 1):
```go
package mcp_test
```
Uses external test package (`mcp_test` not `mcp`) -- follow this convention.

**Imports pattern** (telemetry_middleware_test.go lines 3-21):
```go
import (
	"context"
	"encoding/json"
	"log/slog"
	"io"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/internal/mcp"
)
```
No prometheus or obs imports needed for suggestion tests.

**Test request construction** (telemetry_middleware_test.go lines 25-32):
```go
func newCallToolReq(name string) *mcpsdk.CallToolRequest {
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(`{}`),
		},
	}
}
```
Reuse or duplicate this helper for suggestion middleware tests. May need to extend with custom arguments JSON.

**Discard logger helper** (telemetry_middleware_test.go lines 34-36):
```go
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
```

**Middleware test structure** (telemetry_middleware_test.go lines 38-63):
```go
func TestTelemetryMiddleware_toolsCallEmitsMetric(t *testing.T) {
	// 1. Set up dependencies
	// 2. Create middleware
	mw := mcp.TelemetryMiddleware(provider, getSession, nil, discardLogger())
	// 3. Define inner handler that returns controlled result
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}
	// 4. Wire and call
	handler := mw(inner)
	result, err := handler(context.Background(), "tools/call", newCallToolReq("find_symbol"))
	// 5. Assert
}
```
Follow this exact structure for each suggestion middleware test case. The inner handler should return either:
- `(nil, fmt.Errorf("...unexpected additional properties..."))` for protocol error tests
- `(&mcpsdk.CallToolResult{IsError: true, Content: [...]}, nil)` for tool error tests
- `(&mcpsdk.CallToolResult{Content: [...]}, nil)` for success pass-through tests

---

### `internal/mcp/suggest_lev_test.go` (test, transform)

**Analog:** Same test package pattern as `telemetry_middleware_test.go`.

**Structure:** Simple table-driven tests for pure functions:
```go
func TestLevenshteinDistance(t *testing.T) {
	tests := []struct{ a, b string; want int }{...}
	for _, tt := range tests {
		if got := mcp.LevenshteinDistanceForTest(tt.a, tt.b); got != tt.want {
			t.Errorf(...)
		}
	}
}
```
Note: Since functions are unexported, either (a) add `ForTest` exported wrappers (like `ClassifyOutcomeForTest` in middleware.go line 216-218) or (b) put tests in `package mcp` (internal test package). The project has precedent for both -- `middleware.go` exports test helpers:
```go
// ClassifyOutcomeForTest exposes classifyOutcome to the _test package.
func ClassifyOutcomeForTest(result mcpsdk.Result, err error) string {
	return classifyOutcome(result, err)
}
```
Follow this pattern: add `LevenshteinDistanceForTest`, `ExtractBadParamsForTest`, `BestParamSuggestionForTest` exports in `suggest_lev.go`.

---

### `internal/mcp/middleware.go` (MODIFY -- InstallMiddleware)

**Current InstallMiddleware** (middleware.go lines 37-42):
```go
func InstallMiddleware(server *mcpsdk.Server, provider *obs.Provider, resolver ProfileResolver, getSession func(ctx context.Context) *SessionInfo, budgetFn BudgetFunc, logger *slog.Logger) {
	server.AddReceivingMiddleware(TelemetryMiddleware(provider, getSession, budgetFn, logger))
	if resolver != nil {
		server.AddReceivingMiddleware(ProfileFilterMiddleware(resolver, getSession, logger))
	}
}
```
**Modification:** Do NOT add SuggestionMiddleware here. Per RESEARCH.md Pitfall 2 (timing), the schema map must be built AFTER all tools are registered. `InstallMiddleware` is called at daemon step 14, which is after tool registration (steps 10-11). Two options:
1. Add a separate `InstallSuggestionMiddleware(server, schemaMap, logger)` function.
2. Add schemaMap param to existing `InstallMiddleware`.

Option 1 is cleaner -- follows single-responsibility. Add to `middleware.go` or to `suggest.go`:
```go
func InstallSuggestionMiddleware(server *mcpsdk.Server, schemaMap *ToolSchemaMap, logger *slog.Logger) {
	server.AddReceivingMiddleware(SuggestionMiddleware(schemaMap, logger))
}
```

---

### `internal/daemon/daemon.go` (MODIFY -- wiring)

**Current middleware installation** (daemon.go line 318):
```go
serenaMCP.InstallMiddleware(mcpServer.SDK(), observability, profileStore, getSessionFn, budgetFn, logger)
```
**Modification:** Add SuggestionMiddleware installation AFTER this line (after step 14). The schema map must be built from the SDK server's registered tools. Per RESEARCH.md Open Question 1, the approach is to either:
- Call through middleware chain with synthetic ListToolsRequest, or
- Extend ToolRegistry to store `*mcpsdk.Tool` pointers, or
- Use `mcpServer.SDK()` internal state

The cleanest approach given existing code: add a method to `SerenaMCPServer` that collects tool schemas. The daemon wiring would look like:
```go
// After InstallMiddleware (step 14), build suggestion schema map and install.
schemaMap := serenaMCP.BuildToolSchemaMap(mcpServer.CollectToolSchemas())
serenaMCP.InstallSuggestionMiddleware(mcpServer.SDK(), schemaMap, logger)
```

## Shared Patterns

### Error Result Construction
**Source:** `internal/kernel/symbols/tools.go` lines 107-114
**Apply to:** `suggest.go` when converting protocol errors to tool errors
```go
func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
```

### Middleware Registration
**Source:** `internal/mcp/middleware.go` lines 37-42
**Apply to:** `suggest.go` for `InstallSuggestionMiddleware`
```go
server.AddReceivingMiddleware(SuggestionMiddleware(schemaMap, logger))
```

### Test Helper Exports for Unexported Functions
**Source:** `internal/mcp/middleware.go` lines 216-227
**Apply to:** `suggest_lev.go` for exposing Levenshtein and param extraction to `_test` package
```go
// ClassifyOutcomeForTest exposes classifyOutcome to the _test package.
func ClassifyOutcomeForTest(result mcpsdk.Result, err error) string {
	return classifyOutcome(result, err)
}
```

### extractToolName Reuse
**Source:** `internal/mcp/middleware.go` lines 104-110
**Apply to:** `suggest.go` -- call directly (same package)

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/mcp/suggest_lev.go` | utility | transform | Levenshtein distance is a new algorithm with no existing analog in the codebase. Use RESEARCH.md Pattern 3 (lines 228-248) for implementation. |

## Metadata

**Analog search scope:** `internal/mcp/`, `internal/kernel/`, `internal/errors/`, `internal/daemon/`
**Files scanned:** 15
**Pattern extraction date:** 2026-04-22
