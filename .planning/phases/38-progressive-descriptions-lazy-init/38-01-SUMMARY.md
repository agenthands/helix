---
phase: 38-progressive-descriptions-lazy-init
plan: 01
subsystem: mcp-core
tags: [progressive-descriptions, lazy-init, tool-help, middleware]
dependency_graph:
  requires: []
  provides: [brief-descriptions, lazy-init-middleware, get-tool-help]
  affects: [daemon-wiring, profile-filter-middleware, tool-registry]
tech_stack:
  added: []
  patterns: [sync.Once-per-key, json-schema-introspection, middleware-LIFO-ordering]
key_files:
  created:
    - internal/mcp/lazy_init.go
    - internal/mcp/lazy_init_test.go
    - internal/kernel/help/help.go
    - internal/kernel/help/help_test.go
    - internal/kernel/help/tools.go
  modified:
    - internal/mcp/registry.go
    - internal/mcp/middleware.go
    - internal/mcp/middleware_test.go
    - internal/mcp/server.go
    - internal/daemon/daemon.go
decisions:
  - Used empty string for lazy init defaultRoot since config has no ProjectRoot field; lazy init requires repo_path in first tool call or returns error
  - LazyInitMiddleware installed last in LIFO chain so it runs first before TelemetryMiddleware deadline
  - help package placed under kernel/ following health/ pattern for kernel-aware tool registration
metrics:
  duration: 534s
  completed: 2026-04-22T18:57:42Z
  tasks_completed: 2
  tasks_total: 2
  test_count: 10
  files_changed: 10
---

# Phase 38 Plan 01: Progressive Descriptions and Lazy Init Core Infrastructure Summary

Extended ToolDef with BriefDescription/HelpText, wired brief descriptions into tools/list via ProfileFilterMiddleware, created LazyInitMiddleware with sync.Once-per-path concurrency safety, and built get_tool_help MCP tool with runtime InputSchema parameter introspection.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 434c6ab3 | Extend ToolDef with BriefDescription/HelpText, wire brief descriptions into middleware |
| 2 | 5cc44acf | Add lazy init middleware, get_tool_help tool, and daemon wiring |

## Task Details

### Task 1: Extend ToolDef, registry methods, middleware brief description rewrite, server.go plumbing

Most changes were already present in the codebase from prior work. The key fix was updating middleware_test.go to match the updated ProfileFilterMiddleware signature that now accepts `briefDescs map[string]string`.

**Changes:**
- registry.go: BriefDescription/HelpText fields, BriefDescriptions(), Get() methods (pre-existing)
- middleware.go: ProfileFilterMiddleware accepts briefDescs, InstallMiddleware passes registry (pre-existing)
- middleware_test.go: Fixed all 5 test functions to pass nil briefDescs parameter
- server.go: AddToolWithMeta, extended AddSkillTool (pre-existing)
- daemon.go: Registry passed to InstallMiddleware, registerSkillTools passes brief/help (pre-existing)

### Task 2: Lazy init middleware + get_tool_help tool + daemon wiring + unit tests

**Created internal/mcp/lazy_init.go:**
- LazyInitMiddleware struct with activateFn, isActiveFn, defaultRoot, sync.Once map
- resolveRoot extracts repo_path from CallToolRequest Arguments (json.RawMessage)
- Middleware() returns mcpsdk.Middleware intercepting tools/call
- InstallLazyInitMiddleware convenience function

**Created internal/mcp/lazy_init_test.go (6 tests):**
- TestLazyInitActivatesOnFirstCall: verifies activation and next handler called
- TestLazyInitSkipsWhenActive: verifies no activation when already active
- TestLazyInitConcurrent: 10 goroutines, sync.Once ensures single activation
- TestLazyInitResolveRoot: repo_path from arguments preferred over defaultRoot
- TestLazyInitPassthroughNonToolsCall: tools/list passes through without activation
- TestLazyInitActivationError: returns CallToolResult with IsError=true

**Created internal/kernel/help/ package:**
- help.go: ExtractParamDocs (json.Marshal/Unmarshal schema introspection), FormatHelp (markdown formatting)
- help_test.go: 4 tests covering extraction, empty schemas, formatting
- tools.go: get_tool_help MCP tool registration following health/tools.go pattern

**Daemon wiring:**
- help.RegisterTools added after health.RegisterTools
- InstallLazyInitMiddleware added after InstallSuggestionMiddleware (LIFO ordering)
- lazyActivateFn reuses activate callback logic (ActivateWorkspace, repomap, session)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed middleware_test.go for updated ProfileFilterMiddleware signature**
- Found during: Task 1
- Issue: Tests called ProfileFilterMiddleware without the new briefDescs parameter
- Fix: Added nil as briefDescs argument to all 5 test function calls
- Files modified: internal/mcp/middleware_test.go
- Commit: 434c6ab3

**2. [Rule 1 - Bug] Fixed lazy_init.go Arguments type mismatch**
- Found during: Task 2
- Issue: Plan assumed Arguments was map[string]any but SDK uses json.RawMessage
- Fix: Changed resolveRoot to json.Unmarshal the RawMessage before accessing repo_path
- Files modified: internal/mcp/lazy_init.go
- Commit: 5cc44acf

**3. [Rule 3 - Blocking] Used empty defaultRoot for lazy init**
- Found during: Task 2
- Issue: Plan referenced cfg.ProjectRoot which does not exist in config struct
- Fix: Used empty string; lazy init requires repo_path in arguments or skips activation
- Files modified: internal/daemon/daemon.go
- Commit: 5cc44acf

## Verification

- `go build ./...` exits 0
- `go vet ./...` clean (only tree-sitter C warnings)
- `go test ./internal/mcp/...` all 17 tests pass
- `go test ./internal/kernel/help/...` all 4 tests pass

## Self-Check: PASSED
