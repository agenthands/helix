# Phase 38: Progressive Descriptions & Lazy Init - Pattern Map

**Mapped:** 2026-04-22
**Files analyzed:** 15 (3 new, 12 modified)
**Analogs found:** 15 / 15

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/mcp/registry.go` | model | CRUD | self (existing) | exact |
| `internal/mcp/middleware.go` | middleware | request-response | self (existing) | exact |
| `internal/mcp/lazy_init.go` | middleware | request-response | `internal/mcp/suggest.go` | exact |
| `internal/mcp/server.go` | service | request-response | self (existing) | exact |
| `internal/kernel/help/tools.go` | controller | request-response | `internal/kernel/health/tools.go` | exact |
| `internal/kernel/help/help.go` | utility | transform | `internal/mcp/suggest.go` | exact |
| `internal/kernel/symbols/tools.go` | controller | request-response | self (existing) | exact |
| `internal/kernel/edit/tools.go` | controller | request-response | self (existing) | exact |
| `internal/kernel/fileops/tools.go` | controller | request-response | self (existing) | exact |
| `internal/kernel/diag/tools.go` | controller | request-response | self (existing) | exact |
| `internal/kernel/health/tools.go` | controller | request-response | self (existing) | exact |
| `internal/skill/memory/skill.go` | service | CRUD | self (existing) | exact |
| `internal/skill/workflow/skill.go` | service | request-response | self (existing) | exact |
| `internal/skill/repomap/skill.go` | service | request-response | self (existing) | exact |
| `test/bench/tools_descriptions_test.go` | test | batch | `test/bench/main_test.go` | exact |

## Pattern Assignments

### `internal/mcp/registry.go` (model, CRUD) -- MODIFY

**Analog:** self

**ToolDef struct to extend** (lines 9-15):
```go
type ToolDef struct {
	Name        string
	Description string
	// RegisterFn is called to register this tool with the MCP SDK server.
	// This is a callback because the SDK's AddTool is generic and requires type params.
	RegisterFn func(server interface{}) error
}
```
Add `BriefDescription string` and `HelpText string` fields after `Description`.

**BriefDescriptions method pattern** -- follow `Names()` (lines 49-57):
```go
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
```
New `BriefDescriptions() map[string]string` method uses same lock pattern, iterates `r.tools`, returns `name -> def.BriefDescription` map.

**Get method pattern** -- add `Get(name string) *ToolDef` following same `mu.RLock()` pattern as `Names()`.

---

### `internal/mcp/middleware.go` (middleware, request-response) -- MODIFY

**Analog:** self -- `ProfileFilterMiddleware` (lines 240-297)

**Description rewrite pattern** -- insert after existing description overrides (lines 283-293):
```go
// Apply description overrides from the profile.
if resolver != nil {
	overrides := resolver.ToolDescriptionOverrides(snap.Profile)
	if len(overrides) > 0 {
		for _, tool := range listResult.Tools {
			if desc, ok := overrides[tool.Name]; ok {
				tool.Description = desc
			}
		}
	}
}
```
Add a brief description rewrite block BEFORE profile overrides (so profile overrides win). The middleware needs a `briefDescs map[string]string` parameter. Pattern: iterate `listResult.Tools`, replace `tool.Description` with `briefDescs[tool.Name]` when populated.

**ProfileFilterMiddleware signature** will gain a `briefDescs map[string]string` parameter. The `InstallMiddleware` function (line 37) will build this map from `ToolRegistry.BriefDescriptions()`.

---

### `internal/mcp/lazy_init.go` (middleware, request-response) -- NEW

**Analog:** `internal/mcp/suggest.go` -- `SuggestionMiddleware` (lines 223-251)

**Middleware signature pattern** (lines 223-226):
```go
func SuggestionMiddleware(schemaMap *ToolSchemaMap, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
```

**Install function pattern** (lines 254-256):
```go
func InstallSuggestionMiddleware(server *mcpsdk.Server, schemaMap *ToolSchemaMap, logger *slog.Logger) {
	server.AddReceivingMiddleware(SuggestionMiddleware(schemaMap, logger))
}
```

**Request extraction pattern** from middleware.go (lines 104-110):
```go
func extractToolName(req mcpsdk.Request) string {
	ctr, ok := req.(*mcpsdk.CallToolRequest)
	if !ok || ctr == nil || ctr.Params == nil {
		return "unknown"
	}
	return ctr.Params.Name
}
```

**Imports pattern** from suggest.go (lines 1-10):
```go
package mcp

import (
	"context"
	"log/slog"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)
```
Add `"sync"` for `sync.Once` and `sync.Mutex`.

**Key design:** LazyInitMiddleware struct holds `activateFn func(ctx, path) error`, `isActiveFn func() bool`, `defaultRoot string`, `mu sync.Mutex`, `initOnce map[string]*sync.Once`. Middleware intercepts `tools/call`, checks `isActiveFn()`, resolves workspace path from request args, activates via `sync.Once` per path.

**Middleware ordering** per daemon.go (lines 307-326): lazy init middleware must be installed LAST via `AddReceivingMiddleware` so it runs FIRST in the LIFO chain (before TelemetryMiddleware's deadline).

---

### `internal/mcp/server.go` (service, request-response) -- MODIFY

**Analog:** self

**AddTool pattern** (lines 171-175):
```go
func (s *SerenaMCPServer) AddTool(tool *mcpsdk.Tool, handler mcpsdk.ToolHandler) {
	s.sdk.AddTool(tool, handler)
	s.registry.Register(&ToolDef{Name: tool.Name, Description: tool.Description})
	s.toolSchemas = append(s.toolSchemas, tool)
}
```
Modify `Register` call to also pass `BriefDescription` and `HelpText` when available.

**AddSkillTool pattern** (lines 191-215):
```go
func (s *SerenaMCPServer) AddSkillTool(name, description string, executor SkillToolExecutor) {
	// ...
	s.registry.Register(&ToolDef{Name: name, Description: description})
	s.toolSchemas = append(s.toolSchemas, tool)
}
```
Extend signature to accept `briefDescription` and `helpText` strings.

---

### `internal/kernel/help/tools.go` (controller, request-response) -- NEW

**Analog:** `internal/kernel/health/tools.go` (lines 1-48)

**Imports pattern** (lines 1-13):
```go
package health

import (
	"context"
	"encoding/json"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/mcp"
)
```

**Args struct pattern** (lines 16-18):
```go
type GetHealthArgs struct {
	Verbose bool `json:"verbose,omitempty" jsonschema:"Show all language servers including healthy ones. Default false returns only unhealthy LSes."`
}
```

**RegisterTools function pattern** (lines 21-48):
```go
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel) {
	tracer := k.Tracer()

	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_health",
		Description: "Get workspace health status and language server states",
	}, kernel.WrapToolSpan(tracer, "get_health", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetHealthArgs) (*mcpsdk.CallToolResult, any, error) {
		// ... handler logic ...
		return textResult(string(jsonBytes)), nil, nil
	}))

	server.Registry().Register(&mcp.ToolDef{
		Name:        "get_health",
		Description: "Get workspace health status and language server states",
	})
}
```

**Result helpers pattern** (lines 92-108):
```go
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
```

**Key difference:** `get_tool_help` needs access to `CollectToolSchemas()` and `ToolRegistry`, so `RegisterTools` signature becomes `RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel)` -- the server provides schema access via `server.CollectToolSchemas()` and `server.Registry()`.

---

### `internal/kernel/help/help.go` (utility, transform) -- NEW

**Analog:** `internal/mcp/suggest.go` -- `BuildToolSchemaMap` (lines 29-75)

**Schema introspection pattern** (lines 37-51):
```go
data, err := json.Marshal(t.InputSchema)
if err != nil {
	continue
}
var schema map[string]any
if err := json.Unmarshal(data, &schema); err != nil {
	continue
}

props, ok := schema["properties"].(map[string]any)
if !ok {
	continue
}
```

**Property extraction pattern** (lines 52-71):
```go
for name, propVal := range props {
	info.ValidParams = append(info.ValidParams, name)
	if propMap, ok := propVal.(map[string]any); ok {
		if enumVals, ok := propMap["enum"].([]any); ok {
			strs := make([]string, 0, len(enumVals))
			for _, v := range enumVals {
				if s, ok := v.(string); ok {
					strs = append(strs, s)
				}
			}
			// ...
		}
	}
}
```
Extend to also extract `"type"`, `"description"` from each property, plus `"required"` array from schema root.

---

### `internal/kernel/symbols/tools.go` (controller, request-response) -- MODIFY

**Analog:** self

**Tool registration pattern** (lines 185-209):
```go
func registerGoToDefinition(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "go_to_definition",
		Description: "Go to the definition of a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "go_to_definition", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
		// ... handler ...
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "go_to_definition", Description: "Go to the definition of a symbol at a given position"})
}
```

**Modification pattern:** Add `BriefDescription` and `HelpText` to the `Registry().Register()` call. Add `const goToDefinitionHelp = ...` near the function. Repeat for all 9 tools.

---

### `internal/skill/memory/skill.go` (service, CRUD) -- MODIFY

**Analog:** self

**Skill tool definition pattern** (lines 67-77):
```go
func (s *MemorySkill) writeMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "write_memory",
		Description: "Write information about this project that can be useful for future tasks to a memory in md format. " +
			"The memory name should be meaningful and can include \"/\" to organize into topics (e.g., \"auth/login/logic\"). " +
			"Use the \"global/\" prefix for writing a memory that is shared across projects.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}
```

**Modification pattern:** Add `BriefDescription` and `HelpText` fields to each returned `ToolDef`. Same pattern for all 7 memory tools.

---

### `test/bench/tools_descriptions_test.go` (test, batch) -- NEW

**Analog:** `test/bench/main_test.go` -- `TestBenchToolsManifestMatchesRegistry` (lines 75-140)

**Test infrastructure pattern** (lines 75-96):
```go
func TestBenchToolsManifestMatchesRegistry(t *testing.T) {
	const expectedCount = 42

	if got := len(benchTools); got != expectedCount {
		t.Fatalf("benchTools has %d entries; expected exactly %d (per D-04 / 09-02 must_haves). "+
			"Source of truth: internal/daemon/bootstrap_test.go", got, expectedCount)
	}

	bd := startBenchDaemon(t)

	liveNames := bd.RegistryNames()
	if got := len(liveNames); got != expectedCount {
		t.Fatalf("live MCP registry reports %d tools; expected exactly %d.", got, expectedCount)
	}
```

**Bidirectional diff pattern** (lines 98-139): builds `manifestSet` and `liveSet`, computes diffs in both directions. Apply same pattern for description golden-file comparison.

**Note:** `expectedCount` must be updated from 42 to 43 after `get_tool_help` is added. Update in both `main_test.go` and `tools_manifest_test.go`.

---

### `internal/daemon/daemon.go` (config, request-response) -- MODIFY

**Analog:** self

**Middleware installation pattern** (lines 307-326):
```go
serenaMCP.InstallMiddleware(mcpServer.SDK(), observability, profileStore, getSessionFn, budgetFn, logger)

// 14b. Install suggestion middleware
suggestionSchemaMap := serenaMCP.BuildToolSchemaMap(mcpServer.CollectToolSchemas())
serenaMCP.InstallSuggestionMiddleware(mcpServer.SDK(), suggestionSchemaMap, logger)
```
Add lazy init middleware installation AFTER suggestion middleware (so it runs FIRST in LIFO chain).

**Skill tools registration pattern** (lines 381-395):
```go
func registerSkillTools(server *serenaMCP.SerenaMCPServer, tp skill.ToolProvider, logger *slog.Logger) {
	executor, hasExecutor := tp.(SkillToolExecutor)
	for _, td := range tp.Tools() {
		if hasExecutor {
			server.AddSkillTool(td.Name, td.Description, executor)
		} else {
			server.Registry().Register(&serenaMCP.ToolDef{
				Name:        td.Name,
				Description: td.Description,
			})
		}
	}
}
```
Extend to also pass `td.BriefDescription` and `td.HelpText` through `AddSkillTool` and `Register`.

**ActivateCallback pattern** (lines 331-352): The lazy init middleware needs access to the same `activeWSKey` variable and `k.ActivateWorkspace` call. Wire via closure closures over `activeWSKey` and `k`.

---

## Shared Patterns

### MCP Middleware Structure
**Source:** `internal/mcp/suggest.go` lines 223-251, `internal/mcp/middleware.go` lines 240-297
**Apply to:** `lazy_init.go` (new)
```go
func MyMiddleware(deps ...) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			// pre-processing
			result, err := next(ctx, method, req)
			// post-processing
			return result, err
		}
	}
}
```

### Kernel Tool Registration
**Source:** `internal/kernel/health/tools.go` lines 21-48, `internal/kernel/symbols/tools.go` lines 185-209
**Apply to:** `internal/kernel/help/tools.go` (new)
```go
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel) {
	tracer := k.Tracer()
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "tool_name",
		Description: "...",
	}, kernel.WrapToolSpan(tracer, "tool_name", func(ctx context.Context, req *mcpsdk.CallToolRequest, args MyArgs) (*mcpsdk.CallToolResult, any, error) {
		// handler
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "tool_name", Description: "..."})
}
```

### Tool Result Helpers
**Source:** `internal/kernel/health/tools.go` lines 92-108
**Apply to:** `internal/kernel/help/tools.go` (new)
```go
func textResult(text string) *mcpsdk.CallToolResult { ... }
func errorResult(msg string) *mcpsdk.CallToolResult { ... }
```

### Schema Introspection
**Source:** `internal/mcp/suggest.go` lines 29-75
**Apply to:** `internal/kernel/help/help.go` (new)
```go
data, err := json.Marshal(tool.InputSchema)
var schema map[string]any
json.Unmarshal(data, &schema)
props := schema["properties"].(map[string]any)
```

### Error Handling in Tools
**Source:** `internal/kernel/symbols/tools.go` lines 190-192
**Apply to:** `internal/kernel/help/tools.go` -- validate `tool_name` param
```go
if args.ToolName == "" {
	return errorResult(serr.New(serr.InvalidArgs, "missing required field: tool_name").
		WithTool("get_tool_help").Error()), nil, nil
}
```

### Golden-File Test Pattern
**Source:** `test/bench/main_test.go` lines 75-140
**Apply to:** `test/bench/tools_descriptions_test.go` (new)
- Use `startBenchDaemon(t)` to get live daemon
- Use `bd.RegistryNames()` for tool enumeration
- Build sets, compare bidirectionally, `t.Fatalf` on mismatch

## No Analog Found

No files without analogs -- every new file has an exact pattern match in the codebase.

## Metadata

**Analog search scope:** `internal/mcp/`, `internal/kernel/`, `internal/skill/`, `internal/daemon/`, `test/bench/`
**Files scanned:** 15
**Pattern extraction date:** 2026-04-22
