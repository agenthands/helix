# Phase 38: Progressive Descriptions & Lazy Init - Research

**Researched:** 2026-04-22
**Domain:** MCP tool descriptions, middleware, workspace lifecycle
**Confidence:** HIGH

## Summary

Phase 38 adds two capabilities: (1) tiered tool descriptions with a `get_tool_help` deep-documentation tool, and (2) transparent first-call workspace activation via MCP middleware. Both are well-contained changes operating on established patterns -- middleware interception for tools/list and tools/call, and the existing ToolDef/ToolRegistry infrastructure.

The MCP Go SDK v1.5.0 (confirmed in go.mod) automatically sends `tools/changed` notifications when tools are added/removed via `changeAndNotify` in `Server.AddTool`. However, this phase does not add/remove tools at runtime -- it rewrites descriptions in the tools/list middleware response. The SDK's `listTools` returns `t.tool` references directly, so the `ProfileFilterMiddleware` already successfully mutates `tool.Description` in-place for profile overrides. The brief description rewrite follows the exact same pattern.

**Primary recommendation:** Extend `ToolDef` with `BriefDescription` and `HelpText` fields. Have `ProfileFilterMiddleware` replace `tool.Description` with `BriefDescription` during tools/list responses. Implement lazy init as a new middleware intercepting tools/call that checks workspace activation state via a closure over the daemon's `activeWSKey`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Extend `ToolDef` in `internal/mcp/registry.go` with a `BriefDescription` field. Keep existing `Description` as the detailed version. Brief descriptions are under 100 tokens each.
- **D-02:** Brief descriptions are universal per tool (not profile-specific). Profile `ToolDescriptionOverrides` remain available on top.
- **D-03:** Each tool registration site adds a brief description alongside the existing detailed description.
- **D-04:** New MCP tool `get_tool_help` accepts a `tool_name` string parameter and returns comprehensive documentation.
- **D-05:** Help content is stored as embedded Go strings co-located with tool registration.
- **D-06:** `get_tool_help` is a kernel-level tool registered via `RegisterTools` pattern. Needs access to tool schema registry.
- **D-07:** Parameter details extracted from MCP SDK's registered tool `InputSchema` at runtime. Only usage examples and patterns are authored manually.
- **D-08:** Golden-file snapshot tests comparing tool listings against known-good baselines.
- **D-09:** Tests live in `test/bench/` alongside existing `tools_manifest_test.go`.
- **D-10:** Test covers: (a) brief descriptions exist for all tools, (b) under 100 tokens, (c) golden-file match.
- **D-11:** Lazy init as MCP receiving middleware in `internal/mcp/` intercepting `tools/call` requests.
- **D-12:** Workspace path derived from tool arguments when present, falling back to daemon's configured project root.
- **D-13:** Thread-safety via `sync.Once` per workspace path for concurrent first calls.
- **D-14:** Lazy init is transparent -- identical behavior whether pre-activated or lazy-activated.

### Claude's Discretion
- Exact brief description wording for each tool (under 100 tokens, factual)
- Help text authoring style and depth per tool
- Whether `get_tool_help` returns markdown or plain text
- Internal structure of lazy init middleware (single file or separate package)
- Error handling when lazy init fails (degrade gracefully or return error to tool call)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DESC-01 | Tool descriptions have tiered detail levels (brief for tool listing, detailed on demand) | BriefDescription field on ToolDef, ProfileFilterMiddleware rewrites Description in tools/list response |
| DESC-02 | Agent can call `get_tool_help` MCP tool for deep documentation on any tool | New kernel-level tool using CollectToolSchemas() for InputSchema introspection + co-located helpText strings |
| DESC-03 | Progressive descriptions are gated by behavioral test coverage before deployment | Golden-file snapshot tests in test/bench/ extending tools_manifest_test.go pattern |
| LAZY-01 | First MCP tool call triggers workspace activation if setup wasn't run | New receiving middleware intercepting tools/call, checks activeWSKey state, calls ActivateWorkspace |
| LAZY-02 | Lazy init is thread-safe under concurrent first calls (sync.Once pattern) | sync.Once per workspace path in a map, serializes concurrent activation attempts |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Brief descriptions on tools/list | MCP middleware | Tool registration sites | Middleware rewrites Description field; registration sites provide BriefDescription data |
| get_tool_help deep documentation | MCP kernel tool | Tool registration sites | Kernel tool queries schema registry at runtime; help text co-located with tool definitions |
| Description regression tests | Test infrastructure | -- | Golden-file snapshots in test/bench/ |
| Lazy workspace activation | MCP middleware | Kernel | Middleware intercepts tools/call, kernel performs actual ActivateWorkspace |
| Thread-safe lazy init | MCP middleware | -- | sync.Once per workspace path, entirely within middleware |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| MCP Go SDK | v1.5.0 | Tool registration, middleware, schema introspection | Already in go.mod, provides AddReceivingMiddleware, Tool struct, InputSchema [VERIFIED: go.mod] |
| sync (stdlib) | go stdlib | sync.Once for thread-safe lazy init | Standard Go concurrency primitive [VERIFIED: stdlib] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json (stdlib) | go stdlib | InputSchema introspection for parameter documentation | Used in get_tool_help to extract parameter details from JSON Schema [VERIFIED: existing pattern in suggest.go] |

No new dependencies are needed. All work uses existing packages.

## Architecture Patterns

### System Architecture Diagram

```
Agent sends tools/list
       |
       v
  MCP SDK listTools()
       |
       v  returns []*Tool with full Description
  ProfileFilterMiddleware (tools/list)
       |
       +-- Replace tool.Description with BriefDescription (when populated)
       +-- Apply ToolDescriptionOverrides from profile (existing)
       +-- Filter by AllowedTools (existing)
       |
       v
  Agent receives brief descriptions

Agent sends tools/call "get_tool_help" {tool_name: "X"}
       |
       v
  LazyInitMiddleware (tools/call)    <-- NEW
       |
       +-- Check activeWSKey.RepoRoot != ""
       |   (if empty, resolve path and call ActivateWorkspace)
       |
       v
  get_tool_help handler
       |
       +-- Lookup tool in CollectToolSchemas()
       +-- Extract parameter info from InputSchema (json.Marshal -> map)
       +-- Combine: full Description + parameter details + co-located helpText
       |
       v
  Agent receives comprehensive documentation
```

### Recommended Project Structure
```
internal/mcp/
  registry.go         # ToolDef gains BriefDescription, HelpText fields
  middleware.go        # ProfileFilterMiddleware gains brief description rewrite
  lazy_init.go         # NEW: LazyInitMiddleware, sync.Once per workspace path
  server.go            # CollectToolSchemas() already exists, possibly add GetToolSchema()
internal/kernel/help/
  tools.go             # NEW: get_tool_help tool registration and handler
  help.go              # NEW: help text registry, schema-to-docs formatter
internal/kernel/symbols/
  tools.go             # Add BriefDescription + helpText to each tool
internal/kernel/edit/
  tools.go             # Add BriefDescription + helpText to each tool
internal/kernel/fileops/
  tools.go             # Add BriefDescription + helpText to each tool
internal/kernel/diag/
  tools.go             # Add BriefDescription + helpText to each tool
internal/kernel/health/
  tools.go             # Add BriefDescription + helpText to each tool
internal/skill/memory/
  skill.go             # Add BriefDescription + helpText to each tool
internal/skill/workflow/
  skill.go             # Add BriefDescription + helpText to each tool
internal/skill/repomap/
  skill.go             # Add BriefDescription + helpText to each tool
test/bench/
  tools_descriptions_test.go  # NEW: golden-file snapshot tests for descriptions
  testdata/tool_descriptions.golden  # NEW: golden file
```

### Pattern 1: BriefDescription Rewrite in ProfileFilterMiddleware
**What:** After existing AllowedTools filtering and description overrides, replace `tool.Description` with `BriefDescription` from a lookup map.
**When to use:** On every tools/list response.
**Example:**
```go
// Source: internal/mcp/middleware.go (existing pattern, extended)
// After existing AllowedTools filtering and ToolDescriptionOverrides...

// Apply brief descriptions (DESC-01).
if briefDescs != nil {
    for _, tool := range listResult.Tools {
        if brief, ok := briefDescs[tool.Name]; ok && brief != "" {
            tool.Description = brief
        }
    }
}
```

**Key design decision:** The middleware needs access to a brief description lookup. Two approaches:
1. Pass a `map[string]string` of brief descriptions built from ToolRegistry at daemon startup
2. Extend ProfileResolver to also return brief descriptions

Approach 1 is simpler and matches D-02 (brief descriptions are universal, not profile-specific). Build the map once from `ToolRegistry` after all tools are registered.

### Pattern 2: get_tool_help Schema Introspection
**What:** Extract parameter documentation from MCP SDK's registered `InputSchema` at runtime (D-07).
**When to use:** When `get_tool_help` is called for any tool.
**Example:**
```go
// Source: internal/mcp/suggest.go (existing pattern for schema parsing)
// Already proven in BuildToolSchemaMap:
data, err := json.Marshal(tool.InputSchema)
var schema map[string]any
json.Unmarshal(data, &schema)
props := schema["properties"].(map[string]any)
// Extract: name, type, description (from jsonschema tags), required, enum values
```

The existing `BuildToolSchemaMap` in `suggest.go` already does this exact pattern for parameter names and enums. `get_tool_help` extends it to also extract `description` and `type` from each property, plus the `required` array from the schema root.

### Pattern 3: LazyInitMiddleware with sync.Once
**What:** Intercept tools/call, check if workspace is active, auto-activate if not (LAZY-01, LAZY-02).
**When to use:** On every tools/call when no workspace has been activated yet.
**Example:**
```go
// Source: new internal/mcp/lazy_init.go
type LazyInitMiddleware struct {
    activateFn   func(ctx context.Context, path string) error
    isActiveFn   func() bool  // checks activeWSKey.RepoRoot != ""
    defaultRoot  string       // from config or cwd
    mu           sync.Mutex
    initOnce     map[string]*sync.Once  // per workspace path
}

func (m *LazyInitMiddleware) Middleware() mcpsdk.Middleware {
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            if method != "tools/call" {
                return next(ctx, method, req)
            }
            if !m.isActiveFn() {
                root := m.resolveRoot(req)
                if root != "" {
                    once := m.getOnce(root)
                    var initErr error
                    once.Do(func() {
                        initErr = m.activateFn(ctx, root)
                    })
                    if initErr != nil {
                        // Return error or degrade gracefully
                    }
                }
            }
            return next(ctx, method, req)
        }
    }
}
```

### Pattern 4: Golden-File Snapshot Testing
**What:** Snapshot test comparing tool names + brief descriptions against a golden file (D-08, D-10).
**When to use:** Regression gate for description changes.
**Example:**
```go
// Source: existing pattern in test/bench/tools_manifest_test.go
// The existing TestBenchToolsManifestMatchesRegistry already starts a daemon
// and gets live registry names. Extend to also capture descriptions.

func TestToolDescriptionsGoldenFile(t *testing.T) {
    bd := startBenchDaemon(t)
    // Get tools/list response which contains brief descriptions
    // Sort by name, format as "tool_name: brief_description"
    // Compare against testdata/tool_descriptions.golden
    // Also assert: len(brief) > 0 for all tools, token count < 100
}
```

### Anti-Patterns to Avoid
- **Storing help text in a separate data file:** Keep it co-located with tool registration per D-05. A centralized `help.yaml` would drift from actual tool behavior.
- **Dynamic description generation at runtime:** Brief descriptions should be static strings, not computed. Keeps tools/list fast and deterministic.
- **Using sync.Once globally instead of per-path:** D-13 specifies per-workspace-path. A global sync.Once would prevent activating a second workspace after the first.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema parsing for parameter docs | Custom schema parser | json.Marshal + map[string]any traversal | Proven pattern in suggest.go, handles nested schemas |
| Thread-safe once-init | Custom locking protocol | sync.Once per workspace | Standard Go primitive, exactly designed for this |
| tools/changed notification | Custom notification logic | SDK's built-in changeAndNotify | SDK v1.5.0 sends notifications automatically on AddTool/RemoveTools |
| Token counting for 100-token limit | Full tokenizer | strings.Fields word count as approximation | 100 tokens ~75 words for English; exact tokenization not needed for a lint test |

**Key insight:** The existing codebase already has every pattern needed. `ProfileFilterMiddleware` rewrites descriptions. `BuildToolSchemaMap` parses InputSchema. `tools_manifest_test.go` does golden-file parity testing. This phase composes existing patterns rather than inventing new ones.

## Common Pitfalls

### Pitfall 1: Mutating Shared Tool Pointers in tools/list
**What goes wrong:** `ProfileFilterMiddleware` already mutates `tool.Description` on the `*mcpsdk.Tool` objects returned by `listTools`. These are pointers to the same objects stored in the SDK's internal `tools` list. Mutations are visible to ALL subsequent tools/list calls.
**Why it happens:** The SDK's `listTools` returns `t.tool` (pointer), not a copy.
**How to avoid:** This is the existing behavior and it works because the middleware runs on every tools/list call, re-applying overrides each time. The brief description rewrite follows the same pattern: always overwrite, never rely on previous state.
**Warning signs:** A test that calls tools/list twice and expects different descriptions without re-setting state.

### Pitfall 2: Lazy Init Race with activate_project
**What goes wrong:** Agent calls `activate_project` and a tool simultaneously. Both try to activate the workspace.
**Why it happens:** Concurrent MCP requests through different middleware paths.
**How to avoid:** `sync.Once` per workspace path serializes activation. `kernel.ActivateWorkspace` is already idempotent (checks `if rt, ok := k.workspaces[hash]; ok { return rt, nil }`). The lazy init middleware should also check `isActiveFn()` before attempting activation.
**Warning signs:** "workspace activated" log appearing twice for the same root.

### Pitfall 3: Lazy Init Middleware Ordering
**What goes wrong:** Lazy init fires AFTER TelemetryMiddleware times out the request, causing the first call to always fail with a timeout.
**Why it happens:** Middleware ordering matters. TelemetryMiddleware injects deadline budgets.
**How to avoid:** Install LazyInitMiddleware AFTER TelemetryMiddleware (in `AddReceivingMiddleware` call order) so it runs BEFORE the deadline. The MCP SDK's middleware chain is LIFO -- last added runs first. So `AddReceivingMiddleware(LazyInit)` should be called LAST.
**Warning signs:** First tool calls timing out with `context.DeadlineExceeded`.

### Pitfall 4: Help Text Staleness
**What goes wrong:** Help text describes parameters that no longer exist, or misses new parameters.
**Why it happens:** Help text is manually authored but parameter schemas are auto-generated.
**How to avoid:** `get_tool_help` extracts parameter info from InputSchema at runtime (D-07). Only usage examples and patterns are static. The golden-file test catches description drift.
**Warning signs:** `get_tool_help` output mentioning a parameter not in the schema.

### Pitfall 5: Brief Description Token Counting
**What goes wrong:** "Under 100 tokens" is ambiguous -- different tokenizers give different counts.
**Why it happens:** Success criteria says "under 100 tokens" but Go has no built-in tokenizer.
**How to avoid:** Use word count as a conservative proxy. 100 tokens is approximately 75 English words. The test should assert `len(strings.Fields(brief)) < 80` as a safe proxy. Document the approximation.
**Warning signs:** A brief description that passes word-count but exceeds 100 tokens for a specific LLM tokenizer.

### Pitfall 6: Workspace Path Resolution for Lazy Init
**What goes wrong:** Lazy init cannot determine the workspace path because the tool call has no `repo_path` argument and no default is configured.
**Why it happens:** Most tools use relative `path` (e.g., `main.go`), not absolute `repo_path`. Only `activate_project` has `repo_path`.
**How to avoid:** D-12 specifies: derive from tool arguments when present, fall back to daemon's configured project root. The fallback should use the `cwd` of the forwarder process (passed via gRPC metadata or CLI flag), not a hardcoded path. If no path is resolvable, return an actionable error: "No workspace active. Call activate_project first."
**Warning signs:** "workspace not activated" errors on first use despite lazy init being enabled.

## Code Examples

### Example 1: Extended ToolDef
```go
// Source: internal/mcp/registry.go (to be modified)
type ToolDef struct {
    Name             string
    Description      string // detailed description (existing)
    BriefDescription string // under 100 tokens for tools/list (NEW, DESC-01)
    HelpText         string // usage examples and patterns (NEW, DESC-02)
    RegisterFn       func(server interface{}) error
}
```

### Example 2: BriefDescription Map Builder
```go
// Source: internal/mcp/registry.go (new method)
func (r *ToolRegistry) BriefDescriptions() map[string]string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    descs := make(map[string]string, len(r.tools))
    for name, def := range r.tools {
        if def.BriefDescription != "" {
            descs[name] = def.BriefDescription
        }
    }
    return descs
}
```

### Example 3: get_tool_help Parameter Extraction
```go
// Source: internal/kernel/help/help.go (new file)
type ParamDoc struct {
    Name        string
    Type        string
    Description string
    Required    bool
    EnumValues  []string
}

func ExtractParamDocs(tool *mcpsdk.Tool) []ParamDoc {
    data, err := json.Marshal(tool.InputSchema)
    if err != nil {
        return nil
    }
    var schema map[string]any
    if err := json.Unmarshal(data, &schema); err != nil {
        return nil
    }
    
    required := map[string]bool{}
    if reqArr, ok := schema["required"].([]any); ok {
        for _, r := range reqArr {
            if s, ok := r.(string); ok {
                required[s] = true
            }
        }
    }
    
    props, ok := schema["properties"].(map[string]any)
    if !ok {
        return nil
    }
    
    var docs []ParamDoc
    for name, propVal := range props {
        doc := ParamDoc{Name: name, Required: required[name]}
        if propMap, ok := propVal.(map[string]any); ok {
            if t, ok := propMap["type"].(string); ok {
                doc.Type = t
            }
            if d, ok := propMap["description"].(string); ok {
                doc.Description = d
            }
            if enumVals, ok := propMap["enum"].([]any); ok {
                for _, v := range enumVals {
                    if s, ok := v.(string); ok {
                        doc.EnumValues = append(doc.EnumValues, s)
                    }
                }
            }
        }
        docs = append(docs, doc)
    }
    return docs
}
```

### Example 4: Tool Help Text Co-location
```go
// Source: internal/kernel/symbols/tools.go (modified)
const goToDefinitionHelp = `Navigate to the definition of a symbol at a given position.

## Usage Examples

Find where a function is defined:
  go_to_definition(path="main.go", line=10, column=5)

Jump to a type definition from a variable reference:
  go_to_definition(path="handler.go", line=25, column=12)

## Common Patterns
- Use after find_references to understand where a symbol is defined
- Works across files within the same workspace
- Returns file:line:col of the definition location`

func registerGoToDefinition(...) {
    // ... existing registration ...
    server.Registry().Register(&mcp.ToolDef{
        Name:             "go_to_definition",
        Description:      "Go to the definition of a symbol at a given position",
        BriefDescription: "Jump to where a symbol is defined",
        HelpText:         goToDefinitionHelp,
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single long Description field | MCP spec supports Description + Title (SDK v1.5.0) | 2025 | Title field available but not used by most clients for tool selection |
| tools/changed via custom notification | SDK auto-sends via changeAndNotify in AddTool/RemoveTools | SDK v0.8.0+ | No custom notification needed |
| MCP SDK v0.8.0 Tool struct | v1.5.0 adds Title, Icons, Annotations, OutputSchema | 2025-2026 | Title could complement brief descriptions but Description is what agents use for tool selection |

**MCP SDK v1.5.0 Tool struct fields** [VERIFIED: go module cache]:
- `Name`, `Description`, `InputSchema` (core, existing)
- `Title` (new -- for UI display, not used by agents for tool selection)
- `OutputSchema` (new -- structured output validation)
- `Annotations` (new -- includes title, readOnlyHint, destructiveHint)
- `Icons` (new -- UI icons)
- `Meta` (new -- extensibility metadata)

The `Description` field remains the primary field agents use for tool selection decisions. Brief descriptions should go here in tools/list responses.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Word count (strings.Fields) is a reasonable proxy for token count at the 100-token threshold | Pitfalls | Test might pass descriptions that exceed 100 tokens for specific tokenizers; mitigation: use conservative 75-word limit |
| A2 | Agents primarily use Description (not Title) for tool selection | State of the Art | If agents use Title, we should set Title to brief and Description to detailed; low risk since Claude/Codex don't inspect Title |
| A3 | The forwarder can pass workspace path via gRPC metadata or CLI flag for lazy init fallback | Pitfalls | If no mechanism exists, lazy init may need the user to configure project root; check forwarder code during implementation |

## Open Questions

1. **Workspace path for lazy init fallback**
   - What we know: `activate_project` takes explicit `repo_path`. Most tool calls use relative `path`.
   - What's unclear: How the daemon knows the project root without `activate_project` being called first. The forwarder likely knows the cwd but may not pass it to the daemon.
   - Recommendation: During implementation, check if the forwarder's gRPC `ActivateWorkspace` RPC (already exists in daemon.go line 584) can be reused, or if the daemon's config/CLI flags include a project root. Worst case, the lazy init middleware requires the first tool call to include `repo_path` or returns an error.

2. **Benchmark tool count after adding get_tool_help**
   - What we know: Current count is 42 tools, locked by D-04 in test/bench.
   - What's unclear: Adding `get_tool_help` makes it 43. The manifest test must be updated.
   - Recommendation: Update `expectedCount` to 43 and add `get_tool_help` to `benchTools`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | go test (stdlib) |
| Config file | none -- standard Go test tooling |
| Quick run command | `go test ./internal/mcp/... ./test/bench/... -run TestToolDesc -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DESC-01 | Brief descriptions exist for all tools and are under 100 tokens | unit | `go test ./test/bench/... -run TestToolDescriptions -count=1` | Wave 0 |
| DESC-02 | get_tool_help returns comprehensive docs for any tool | unit | `go test ./internal/kernel/help/... -run TestGetToolHelp -count=1` | Wave 0 |
| DESC-03 | Golden-file snapshot matches baseline | unit | `go test ./test/bench/... -run TestToolDescriptionsGolden -count=1` | Wave 0 |
| LAZY-01 | First tool call activates workspace transparently | integration | `go test ./internal/mcp/... -run TestLazyInit -count=1` | Wave 0 |
| LAZY-02 | Concurrent first calls serialized | unit | `go test ./internal/mcp/... -run TestLazyInitConcurrent -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/mcp/... ./internal/kernel/help/... -count=1 -short`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `test/bench/tools_descriptions_test.go` -- golden-file tests for DESC-01, DESC-03
- [ ] `internal/kernel/help/tools_test.go` -- get_tool_help handler tests for DESC-02
- [ ] `internal/mcp/lazy_init_test.go` -- lazy init middleware tests for LAZY-01, LAZY-02
- [ ] `test/bench/testdata/tool_descriptions.golden` -- golden file baseline

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | no | Lazy init respects existing profile tool filtering |
| V5 Input Validation | yes | get_tool_help validates tool_name against registry; lazy init validates path |
| V6 Cryptography | no | -- |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tool name injection in get_tool_help | Tampering | Validate tool_name exists in registry before processing |
| Path traversal in lazy init workspace path | Tampering | Use filepath.Abs + os.Stat validation (existing pattern in daemon.go:595) |
| Denial-of-service via repeated lazy init | Denial | sync.Once ensures activation runs at most once per path |

## Sources

### Primary (HIGH confidence)
- MCP Go SDK v1.5.0 source code -- Tool struct fields, AddTool implementation, changeAndNotify, listTools [VERIFIED: go module cache at $(go env GOMODCACHE)/github.com/modelcontextprotocol/go-sdk@v1.5.0/mcp/]
- `internal/mcp/middleware.go` -- ProfileFilterMiddleware pattern, tools/list interception [VERIFIED: codebase]
- `internal/mcp/suggest.go` -- BuildToolSchemaMap InputSchema parsing pattern [VERIFIED: codebase]
- `internal/mcp/server.go` -- CollectToolSchemas, AddTool, AddSkillTool registration [VERIFIED: codebase]
- `internal/mcp/registry.go` -- ToolDef struct, ToolRegistry [VERIFIED: codebase]
- `internal/kernel/kernel.go` -- ActivateWorkspace idempotency [VERIFIED: codebase]
- `internal/daemon/daemon.go` -- Daemon bootstrap, middleware installation, activeWSKey closure [VERIFIED: codebase]
- `test/bench/tools_manifest_test.go` -- Golden-file parity test pattern [VERIFIED: codebase]
- `go.mod` -- MCP SDK v1.5.0 [VERIFIED: go.mod]

### Secondary (MEDIUM confidence)
- MCP SDK tools/changed notification automatic sending via changeAndNotify [VERIFIED: SDK source]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all existing patterns
- Architecture: HIGH -- every pattern is an extension of existing code
- Pitfalls: HIGH -- verified against actual middleware ordering and SDK behavior

**Research date:** 2026-04-22
**Valid until:** 2026-05-22 (stable -- no fast-moving dependencies)
