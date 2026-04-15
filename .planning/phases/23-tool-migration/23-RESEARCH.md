# Phase 23: Tool Migration - Research

**Researched:** 2026-04-15
**Domain:** Go error handling migration -- raw `fmt.Errorf`/`errors.New` to typed `serr.New()` constructors
**Confidence:** HIGH

## Summary

This phase is a mechanical migration of ~137 raw error construction sites across 24 files in 8 tool groups to use the `serr` package created in Phase 22. The `serr` package (`internal/errors/`) provides `New(kind, msg)`, `Wrap(kind, msg, cause)`, `WithTool()`, and `WithDetail()` builders with 7 Kind constants: NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout.

The migration has two layers at each tool: (1) **implementation functions** (e.g., `read.go`, `retrieval.go`) that return `error` to callers, and (2) **tool handler registrations** (e.g., `tools.go`) that convert errors to MCP `CallToolResult` via `errorResult(err.Error())`. The implementation layer needs `fmt.Errorf` replaced with `serr.New()`/`serr.Wrap()`. The handler layer currently stringifies errors via `.Error()` -- once implementation functions return `*serr.Error`, the handler can pass them through directly or continue stringifying (since `serr.Error.Error()` produces structured output).

After migration, deprecated sentinel re-exports in `internal/mcp/errors.go` and `internal/kernel/lspool/circuit_err.go` become dead code and should be removed. Three test files contain string-matching assertions that will need updating. MCP core tools only have raw errors in test files (3 sites) -- these are test scaffolding, not production error paths.

**Primary recommendation:** Migrate bottom-up -- implementation files first (where `fmt.Errorf` calls live), then tool handler files (where `errorResult()` calls live), then cleanup deprecated re-exports last.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Split migration into 8 plans by tool group: symbols (9 tools), edit (6), fileops (6), diag (3), memory (7), workflow (2), profile (2), MCP core (3). Each plan is self-contained and can execute in parallel waves.
- **D-02:** Map raw errors to the 7 Kind values by semantic intent -- classify based on what the error MEANS, not rigid category rules.
- **D-03:** Remove deprecated re-exports in `internal/mcp/errors.go` and `internal/kernel/lspool/circuit_err.go` during this phase. Update remaining references to use `serr.ErrXxx` directly.
- **D-04:** Normalize error message strings during migration: lowercase, no trailing punctuation, consistent verb form.
- **D-05:** Tool name excluded from message text -- use `.WithTool("tool_name")` for tool attribution. Messages stay generic and reusable.

### Claude's Discretion
- Exact Kind classification per error site (guided by semantic intent principle)
- Whether to consolidate duplicate error messages within a tool group
- Order of tool groups across plans/waves
- Whether to add `.WithDetail()` for error sites that already carry useful context (e.g., file paths, symbol names)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MIG-01 | All 9 symbol retrieval tools return typed errors | 13 error sites across hierarchy.go, overview.go, retrieval.go, search.go, tools.go; handler wrappers in tools.go |
| MIG-02 | All 6 symbol editing tools return typed errors | ~30 error sites across delete.go, insert.go, planner.go, rename.go, replace.go, treesitter.go; handlers in tools.go |
| MIG-03 | All 6 file operation tools return typed errors | ~33 error sites across find.go, list.go, read.go, replace.go, search.go, validate.go, write.go; handlers in tools.go |
| MIG-04 | All 3 diagnostic tools return typed errors | ~8 error sites across actions.go, format.go; handler in skill_adapter.go |
| MIG-05 | All 7 memory tools return typed errors | ~21 error sites in skill.go |
| MIG-06 | All 2 workflow tools return typed errors | ~1 error site in skill.go |
| MIG-07 | All 2 profile tools return typed errors | ~31 error sites across loader.go, skill.go |
| MIG-08 | All 3 MCP core tools return typed errors | ~3 error sites in test files only (telemetry_middleware_test.go, telemetry_span_test.go) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Always run `go vet` and `go test` before completing any Go task
- Build with `go build ./cmd/serena`
- Test with `go test ./...`
- Format with `gofmt -w .`

## Architecture Patterns

### Two-Layer Error Architecture

Every tool group has a consistent two-layer pattern: [VERIFIED: codebase grep]

**Layer 1 -- Implementation functions** (e.g., `read.go`, `retrieval.go`):
- Return `error` interface
- Currently use `fmt.Errorf("context: %w", err)` for wrapping
- These become `serr.Wrap(kind, "message", err)` or `serr.New(kind, "message").WithDetail(context)`

**Layer 2 -- Tool handlers** (e.g., `tools.go`):
- Call implementation functions
- Convert errors to MCP responses via `errorResult(err.Error())`
- Currently also have inline `errorResult(fmt.Sprintf("...: %v", err))` calls
- These should convert to `errorResult(err.Error())` where err is already `*serr.Error`

### Handler Error Pattern (tools.go files)

All kernel tool packages (symbols, edit, fileops, diag) share an identical pattern: [VERIFIED: codebase read]

```go
// Current pattern in every tool handler:
rt, err := acquireLease(ctx, k, wsKeyFn)  // or k.GetRuntime(wsKey)
if err != nil {
    return errorResult(err.Error()), nil, nil  // <- workspace not activated
}
lease, err := rt.AcquireSession(ctx, "default", false)
if err != nil {
    return errorResult(fmt.Sprintf("acquire session: %v", err)), nil, nil
}
result, err := SomeOperation(ctx, lease, ...)
if err != nil {
    return errorResult(fmt.Sprintf("operation: %v", err)), nil, nil
}
```

After migration:
```go
rt, err := acquireLease(ctx, k, wsKeyFn)
if err != nil {
    return errorResult(err.Error()), nil, nil  // err is already *serr.Error
}
lease, err := rt.AcquireSession(ctx, "default", false)
if err != nil {
    return errorResult(serr.Wrap(serr.Internal, "acquire session", err).
        WithTool("tool_name").Error()), nil, nil
}
result, err := SomeOperation(ctx, lease, ...)
if err != nil {
    return errorResult(err.Error()), nil, nil  // err is already *serr.Error from impl
}
```

### Skill Tool Pattern (memory, workflow, profile)

Skill tools use `ExecuteTool(name string, params map[string]interface{}) (string, error)` -- they return `error` directly rather than `*mcpsdk.CallToolResult`. The daemon wraps these into MCP responses. [VERIFIED: codebase read]

```go
// Current pattern:
return "", fmt.Errorf("write_memory: 'name' parameter is required (string)")

// After migration:
return "", serr.New(serr.InvalidArgs, "'name' parameter is required").
    WithTool("write_memory").WithDetail("expected string")
```

### Reference Migration (Phase 22 Example)

The `circuit.go` migration from Phase 22 serves as the canonical example: [VERIFIED: codebase read]

```go
// internal/kernel/lspool/circuit.go
import serr "github.com/postfix/serena/internal/errors"

func (cb *CircuitBreaker) CircuitOpenErr() *serr.Error {
    return serr.New(serr.CircuitOpen, "circuit breaker open").WithDetail(detail)
}
```

### `errorResult()` Helper -- Shared Across Packages

Each tool package defines its own `errorResult(msg string)` helper. These can remain as-is since they accept a string and `serr.Error.Error()` produces a well-formatted string. No signature change needed. [VERIFIED: codebase read]

### Recommended Kind Mapping

Based on semantic intent analysis of all 137 error sites: [ASSUMED]

| Error Pattern | Kind | Rationale |
|--------------|------|-----------|
| "workspace not activated" / "no active workspace" | NoWorkspace | Missing prerequisite |
| "file not found" / "directory not found" | NotFound | Resource doesn't exist |
| "symbol not found" / "declaration not found" | NotFound | Resource doesn't exist |
| "file already exists" | InvalidArgs | Caller supplied wrong path |
| "path outside workspace root" | InvalidArgs | Bad input (security violation) |
| "file exceeds size limit" | InvalidArgs | Input constraint violation |
| "invalid regex pattern" / "invalid byte range" | InvalidArgs | Bad input format |
| "missing required parameter" | InvalidArgs | Incomplete input |
| "unknown tool" / "unknown mode" / "unknown profile" | InvalidArgs | Bad input value |
| "unsupported language for tree-sitter" | Unsupported | Feature not available for input |
| "tree-sitter parse failed" | Internal | Implementation failure |
| "reading file" / "writing file" / "stat file" | Internal | I/O failure (wrap cause) |
| "formatting request" / "codeAction request" | Internal | LSP communication failure (wrap cause) |
| "didChange notification" | Internal | LSP notification failure (wrap cause) |
| "acquire session" | Internal | Session pool failure (wrap cause) |
| "result limit reached" | Internal | Traversal limit (non-error sentinel) |

### Anti-Patterns to Avoid

- **Tool name in message text:** Per D-05, use `.WithTool()` not message prefix. Do NOT write `serr.New(serr.InvalidArgs, "write_memory: name required")`.
- **Uppercase / punctuation in messages:** Per D-04, messages are lowercase, no trailing period. "file not found" not "File not found."
- **Returning typed nil:** Per `serr.Error` doc comment, never assign `*serr.Error` nil to an `error` variable and return it. Always return `nil` directly for no-error paths.
- **Double wrapping:** When an implementation function already returns `*serr.Error`, the handler should not re-wrap it. Just pass through via `errorResult(err.Error())`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Error classification | Custom error type per package | `serr.New(kind, msg)` | Phase 22 built the taxonomy; all packages share it |
| Error chain traversal | Manual type switches | `errors.Is(err, serr.ErrNotFound)` | `serr.Error.Is()` matches by Kind |
| Error JSON serialization | Per-package JSON marshalers | `serr.Error.MarshalJSON()` | Built into the Error struct |
| Tool attribution | Message prefix conventions | `.WithTool("name")` | Structured field, not string convention |

## Common Pitfalls

### Pitfall 1: Typed nil interface trap
**What goes wrong:** Assigning `var e *serr.Error = nil` then returning it as `error` produces a non-nil error interface value.
**Why it happens:** Go interface semantics -- a nil pointer of a concrete type wrapped in an interface is not nil.
**How to avoid:** Always return bare `nil` for success paths. Never declare `*serr.Error` variables that might be nil and returned as `error`.
**Warning signs:** Tests passing nil checks when they should fail; false error reports on success paths.

### Pitfall 2: Forgetting to update test assertions
**What goes wrong:** Tests that assert `strings.Contains(err.Error(), "file not found")` break when the message format changes to `not_found: file not found`.
**Why it happens:** `serr.Error.Error()` prepends `kind: ` to the message.
**How to avoid:** Update test assertions to either (a) use `errors.Is(err, serr.ErrNotFound)` for kind checking, or (b) update the substring to match the new format.
**Warning signs:** `fileops_test.go` has 5 such assertions; `memory/skill_test.go` has 1; `workflow/skill_test.go` has 1.

### Pitfall 3: Sentinel error for "result limit reached"
**What goes wrong:** `fileops/find.go:56` and `fileops/search.go:97` return `fmt.Errorf("result limit reached")` as a control flow signal (used by `filepath.WalkDir` to stop walking), not a real error.
**Why it happens:** Go's `filepath.WalkDir` uses the error return to signal early termination.
**How to avoid:** Keep this as a package-internal sentinel (e.g., `var errLimitReached = errors.New("result limit reached")`) and do NOT convert it to `serr.New()`. It is never exposed to MCP clients.
**Warning signs:** If converted to `*serr.Error`, the `walkErr` check in search.go would incorrectly treat limit-reached as a typed error.

### Pitfall 4: Breaking the middleware lspool.ErrCircuitOpen reference
**What goes wrong:** `internal/mcp/middleware.go:88` uses `errors.Is(err, lspool.ErrCircuitOpen)`. If the re-export is removed before updating the reference, the middleware breaks.
**Why it happens:** D-03 says remove deprecated re-exports, but middleware.go depends on one of them.
**How to avoid:** Update `middleware.go` to use `serr.ErrCircuitOpen` directly BEFORE removing `lspool.ErrCircuitOpen`. Also update `telemetry_middleware_test.go`.
**Warning signs:** `go vet` and `go test` will catch this if run after each change.

### Pitfall 5: Profile loader.go errors are not MCP-facing
**What goes wrong:** Converting all 18 `fmt.Errorf` sites in `loader.go` to `serr.New()` when most are startup/configuration errors, not tool execution errors.
**Why it happens:** `loader.go` loads YAML profiles at init time, not during tool execution.
**How to avoid:** Only migrate errors in `loader.go` that can propagate to tool execution paths. Startup-only errors (loading embedded profiles, parsing YAML) can remain as `fmt.Errorf` since they surface as fatal startup errors, not MCP tool responses. Focus `serr.New()` on `skill.go` which has the `ExecuteTool` handler.
**Warning signs:** Excessive migration scope in loader.go without MCP impact.

## Error Site Inventory

Detailed count per file, verified by grep: [VERIFIED: codebase grep]

### Symbols (13 sites)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| hierarchy.go | 6 | `fmt.Errorf("opName: %w", err)` -- LSP call wrappers |
| overview.go | 1 | `fmt.Errorf("documentSymbol: %w", err)` |
| retrieval.go | 4 | `fmt.Errorf("opName: %w", err)` -- LSP call wrappers |
| search.go | 1 | `fmt.Errorf("workspace/symbol: %w", err)` |
| tools.go | 1 | `fmt.Errorf("workspace not activated: %w", err)` in acquireLease |

### Edit (30 sites)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| delete.go | 5 | Mixed: plan/read/write/didChange wrappers |
| insert.go | 8 | Duplicated across InsertBefore/InsertAfter: plan/read/write/didChange |
| planner.go | 2 | get symbol overview + "symbol not found" |
| rename.go | 5 | rename + apply edits + read/write file |
| replace.go | 5 | plan/read/validate range/write/didChange |
| treesitter.go | 5 | unsupported language, parse failure, declaration/field not found |

### Fileops (33 sites)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| find.go | 2 | result limit sentinel + walking error |
| list.go | 4 | not found, stat, not-a-directory, reading dir |
| read.go | 9 | not found, stat, is-a-directory, size limit, read error, line range |
| replace.go | 2 | invalid regex + write error |
| search.go | 3 | invalid regex + result limit sentinel + walk error |
| validate.go | 5 | workspace root empty, resolve errors, path outside workspace |
| write.go | 8 | already exists, mkdir, write, temp file, rename |

### Diag (8 sites)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| actions.go | 4 | codeAction request, no workspace edit, apply error, not applied |
| format.go | 4 | formatting request, read file, write temp, rename temp |

### Memory (21 sites)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| skill.go | 21 | init error + "unknown tool" + per-tool parameter validation + operation wrappers |

### Workflow (1 site)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| skill.go | 1 | "unknown workflow tool" |

### Profile (31 sites)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| loader.go | 18 | YAML loading/parsing errors (startup, not MCP-facing) |
| skill.go | 13 | ExecuteTool dispatch, mode validation, profile lookup |

### MCP Core (3 sites -- test files only)
| File | Sites | Dominant Pattern |
|------|-------|------------------|
| telemetry_middleware_test.go | 2 | `errors.New("boom")` / `errors.New("kaboom")` |
| telemetry_span_test.go | 1 | `errors.New("tool execution failed")` |

## Code Examples

### Pattern 1: Simple error with context (implementation layer)
```go
// Source: verified pattern from circuit.go (Phase 22 reference)
import serr "github.com/postfix/serena/internal/errors"

// Before:
return "", fmt.Errorf("file not found: %s", path)

// After:
return "", serr.New(serr.NotFound, "file not found").WithDetail(path)
```

### Pattern 2: Wrapping a cause error (implementation layer)
```go
// Before:
return nil, fmt.Errorf("definition: %w", err)

// After:
return nil, serr.Wrap(serr.Internal, "definition", err)
```

### Pattern 3: Parameter validation (skill ExecuteTool)
```go
// Before:
return "", fmt.Errorf("write_memory: 'name' parameter is required (string)")

// After (per D-04 and D-05):
return "", serr.New(serr.InvalidArgs, "'name' parameter is required").
    WithTool("write_memory")
```

### Pattern 4: Workspace not activated (handler layer)
```go
// Before:
func acquireLease(ctx context.Context, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey) (*kernel.WorkspaceRuntime, error) {
    wsKey := wsKeyFn()
    rt, err := k.GetRuntime(wsKey)
    if err != nil {
        return nil, fmt.Errorf("workspace not activated: %w", err)
    }
    return rt, nil
}

// After:
func acquireLease(ctx context.Context, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey) (*kernel.WorkspaceRuntime, error) {
    wsKey := wsKeyFn()
    rt, err := k.GetRuntime(wsKey)
    if err != nil {
        return nil, serr.Wrap(serr.NoWorkspace, "workspace not activated", err)
    }
    return rt, nil
}
```

### Pattern 5: Internal sentinel (DO NOT MIGRATE)
```go
// fileops/find.go and fileops/search.go use this for filepath.WalkDir control flow.
// This is NOT an MCP error -- keep as-is:
return fmt.Errorf("result limit reached")
```

### Pattern 6: Test assertion update
```go
// Before:
if !strings.Contains(err.Error(), "file not found") {

// After -- prefer Kind-based matching:
if !errors.Is(err, serr.ErrNotFound) {

// Or if testing specific message:
if !strings.Contains(err.Error(), "not_found: file not found") {
```

## Deprecated Sentinel Cleanup

### Files to modify
| File | What to do |
|------|-----------|
| `internal/mcp/errors.go` | Remove ErrSessionExpired, ErrWorkspaceNotReady, ErrToolNotAvailable, ErrProjectNotFound, ErrLSCrashed sentinels and ErrorDetail struct |
| `internal/kernel/lspool/circuit_err.go` | Remove entire file (ErrCircuitOpen re-export) |
| `internal/mcp/middleware.go:88` | Change `lspool.ErrCircuitOpen` to `serr.ErrCircuitOpen` |
| `internal/mcp/telemetry_middleware_test.go:90,192` | Change `lspool.ErrCircuitOpen` to `serr.ErrCircuitOpen` |

### Current references to deprecated sentinels [VERIFIED: codebase grep]
- `lspool.ErrCircuitOpen`: 2 references (middleware.go, telemetry_middleware_test.go)
- `mcp.ErrSessionExpired`, `mcp.ErrWorkspaceNotReady`, etc.: 0 external references (only defined in errors.go itself)
- `mcp.ErrorDetail`: 0 external references

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + testify |
| Config file | none (Go conventions) |
| Quick run command | `go test ./internal/kernel/... ./internal/skill/... ./internal/profile/... ./internal/mcp/...` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MIG-01 | Symbol tools return typed errors | unit | `go test ./internal/kernel/symbols/ -run Test -count=1` | yes (retrieval_test.go) |
| MIG-02 | Edit tools return typed errors | unit | `go test ./internal/kernel/edit/ -run Test -count=1` | yes (edit_test.go, treesitter_test.go) |
| MIG-03 | Fileops tools return typed errors | unit | `go test ./internal/kernel/fileops/ -run Test -count=1` | yes (fileops_test.go) |
| MIG-04 | Diag tools return typed errors | unit | `go test ./internal/kernel/diag/ -run Test -count=1` | yes (diag_test.go) |
| MIG-05 | Memory tools return typed errors | unit | `go test ./internal/skill/memory/ -run Test -count=1` | yes (skill_test.go) |
| MIG-06 | Workflow tools return typed errors | unit | `go test ./internal/skill/workflow/ -run Test -count=1` | yes (skill_test.go) |
| MIG-07 | Profile tools return typed errors | unit | `go test ./internal/profile/ -run Test -count=1` | yes (implicit in package tests) |
| MIG-08 | MCP core tools return typed errors | unit | `go test ./internal/mcp/ -run Test -count=1` | yes (telemetry_middleware_test.go) |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test ./internal/kernel/... ./internal/skill/... ./internal/profile/... ./internal/mcp/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
None -- existing test infrastructure covers all phase requirements. Tests that need updating (string assertions to Kind assertions) are part of the migration work itself, not gaps.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Kind mapping table (NotFound for "file not found", InvalidArgs for "path outside workspace", etc.) | Recommended Kind Mapping | Wrong classification means agents match incorrectly; low risk since D-02 gives Claude discretion |
| A2 | loader.go errors are startup-only and should not be migrated to serr | Common Pitfalls (Pitfall 5) | If some loader errors propagate to MCP tool responses, they'd remain as raw strings; medium risk |
| A3 | "result limit reached" sentinels in find.go/search.go are internal control flow only | Common Pitfalls (Pitfall 3) | If they leak to MCP, they'd remain untyped; low risk since callers check specifically for this |

## Open Questions

1. **Should `loader.go` errors be migrated?**
   - What we know: loader.go has 18 error sites, all related to YAML loading at startup
   - What's unclear: Whether any loader errors can surface through `ExecuteTool` calls at runtime
   - Recommendation: Migrate only `skill.go` errors (13 sites) initially; leave `loader.go` for validation phase or defer

2. **MCP core test errors -- are they in scope?**
   - What we know: 3 error sites in test files use `errors.New()` for test doubles
   - What's unclear: D-01 says "MCP core (3)" which matches these test sites, but test doubles don't need typed errors
   - Recommendation: Update test assertions to use `serr.New(serr.Internal, "boom")` for consistency, since the middleware classifies errors by Kind

## Sources

### Primary (HIGH confidence)
- `internal/errors/kinds.go` - 7 Kind constants, sentinel errors [VERIFIED: codebase read]
- `internal/errors/errors.go` - Error struct, New(), Wrap(), WithTool(), WithDetail() [VERIFIED: codebase read]
- `internal/mcp/errors.go` - Deprecated re-exports to remove [VERIFIED: codebase read]
- `internal/kernel/lspool/circuit_err.go` - ErrCircuitOpen re-export to remove [VERIFIED: codebase read]
- `internal/kernel/lspool/circuit.go` - Phase 22 reference migration [VERIFIED: codebase read]
- All tools.go files across 4 kernel packages - handler patterns [VERIFIED: codebase read]
- All skill.go files across memory/workflow/profile - ExecuteTool patterns [VERIFIED: codebase read]
- All implementation files - error site counts via grep [VERIFIED: codebase grep, 137 total sites]
- Deprecated sentinel reference audit [VERIFIED: codebase grep, 2 external references]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - no new dependencies, purely internal refactoring using Phase 22 output
- Architecture: HIGH - all patterns verified by reading actual source files
- Pitfalls: HIGH - identified through systematic codebase analysis (control flow sentinels, test assertions, middleware references, startup-vs-runtime distinction)

**Research date:** 2026-04-15
**Valid until:** 2026-05-15 (stable -- internal refactoring, no external dependencies)
