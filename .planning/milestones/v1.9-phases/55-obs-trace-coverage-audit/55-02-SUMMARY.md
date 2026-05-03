---
phase: 55
plan: 02
subsystem: mcp
tags: [observability, tracing, mcp, skill-tools, tdd, OBS-04]
requires:
  - "internal/obs.Provider.Tracer (already shipped Phase 12)"
  - "internal/kernel.WrapToolSpan precedent (D-07: no attrs on kernel.tool spans)"
provides:
  - "kernel.tool.{name} spans for every skill-registered MCP tool"
  - "tracer-aware SerenaMCPServer constructor (D-01 compliant)"
affects:
  - "internal/mcp/server.go"
  - "internal/mcp/server_test.go"
  - "internal/mcp/server_trace_test.go (new)"
  - "internal/daemon/daemon.go"
tech-stack:
  added: []
  patterns:
    - "trace.Tracer constructor injection (D-01)"
    - "noop tracer fallback when nil (D-09/D-11/D-17)"
    - "RecordError-only on failure (consistent with WrapToolSpan; no SetStatus)"
key-files:
  created:
    - internal/mcp/server_trace_test.go
  modified:
    - internal/mcp/server.go
    - internal/mcp/server_test.go
    - internal/daemon/daemon.go
decisions:
  - "Constructor parameter (not options pattern) for tracer — explicit per D-01"
  - "Adapter shape (not direct WrapToolSpan reuse) because SkillToolExecutor's signature is map[string]any, not the typed mcpsdk.ToolHandlerFor[In, Out]"
  - "Zero attributes on the kernel.tool.{name} span (D-07); TelemetryMiddleware owns tool_name/profile/mode/language/outcome on daemon.mcp.tools.call"
metrics:
  duration: "~25 minutes"
  completed: "2026-05-02"
  tasks: 1
  files_changed: 4
---

# Phase 55 Plan 02: AddSkillTool kernel.tool.{name} Span Coverage Summary

## One-liner

Skill-registered MCP tools (memory_*, workflow_*, repomap-as-skill) now emit `kernel.tool.{name}` spans on invocation, closing the OBS-04 skill-tool span coverage gap with a tracer-injected `SerenaMCPServer` and a wrapped `AddSkillTool` closure.

## What Changed

### `internal/mcp/server.go`

- Added imports `go.opentelemetry.io/otel/trace` and `tracenoop "go.opentelemetry.io/otel/trace/noop"`.
- Added `tracer trace.Tracer` field to `SerenaMCPServer` (D-01: injected, never global).
- **Final constructor signature:**
  ```go
  func NewSerenaMCPServer(workspaces *workspace.Registry, logger *slog.Logger, tracer trace.Tracer) *SerenaMCPServer
  ```
  - `tracer == nil` → process-local noop tracer (`tracenoop.NewTracerProvider().Tracer("mcp-server-noop")`).
- Wrapped `AddSkillTool`'s registered closure body with:
  ```go
  ctx, span := s.tracer.Start(ctx, "kernel.tool."+toolName)
  defer span.End()
  result, err := executor.ExecuteTool(toolName, args)
  if err != nil {
      if span.IsRecording() {
          span.RecordError(err)
      }
      // ... IsError result path
  }
  ```
  - Mirrors `kernel.WrapToolSpan` (RecordError-only, no SetStatus).
  - Zero attributes on the span (D-07).

### `internal/daemon/daemon.go` (call-site update)

- Line 212: `helixMCP.NewSerenaMCPServer(workspaces, logger, observability.Tracer())` — threads `observability.Tracer()` (the existing `*obs.Provider.Tracer()` accessor) into the MCP server, mirroring how the kernel and fileops/diag tool registrations already receive it (lines 191, 243, 253).

### `internal/mcp/server_test.go` (existing test updates)

- Two callers (`TestNewSerenaMCPServer`, `TestSerenaMCPServer_DynamicToolAddRemove`) updated to pass `nil` as the third argument, exercising the noop fallback. No other test files in the repo construct `SerenaMCPServer` directly.

### `internal/mcp/server_trace_test.go` (NEW)

Three new tests use the `tracetest.InMemoryExporter` + `sdktrace.AlwaysSample()` pattern from `internal/kernel/spanwrap_test.go::newTestTracer`. Tools are invoked via the SDK's `NewInMemoryTransports` so the registered closure runs end-to-end:

- `TestAddSkillTool_EmitsKernelToolSpan` — registers a `memory_search` skill tool, calls it, asserts a `kernel.tool.memory_search` span with **zero attributes** (D-07).
- `TestAddSkillTool_NoopTracerSafeNoPanic` — constructs the server with `nil` tracer, registers and invokes a `workflow_handoff` skill tool; asserts no panic and successful invocation.
- `TestAddSkillTool_RecordErrorOnExecutorError` — fake executor returns an error; asserts the span carries an `"exception"` event (RecordError) and no SetStatus.

## Verification

| Check                                                                              | Result |
| ---------------------------------------------------------------------------------- | ------ |
| `go vet ./...`                                                                     | PASS   |
| `go test ./...` (full suite)                                                       | PASS   |
| `go test ./internal/mcp/... -count=1`                                              | PASS   |
| `grep -c 's\.tracer\.Start(ctx, "kernel\.tool\.' internal/mcp/server.go`           | 1      |
| `grep -F 'tracer           trace.Tracer' internal/mcp/server.go`                   | match  |
| `grep -F 'span.RecordError(err)' internal/mcp/server.go`                           | match  |
| `grep -F 'trace.WithAttributes' internal/mcp/server.go \| grep -c 'kernel.tool'`   | 0      |
| `otel.GetTracerProvider` / `otel.SetTracerProvider` in `internal/mcp/server.go`    | none (D-01) |

The single `otel.GetTracerProvider` hit in the file is inside the constructor docstring explaining that the helper deliberately does NOT use it.

## Verbatim Test Names + Commit Hashes

- RED commit: `faa9da00 test(55-02): add failing tests for AddSkillTool kernel.tool.{name} span`
- GREEN commit: `30ea6b0b feat(55-02): wrap AddSkillTool handlers in kernel.tool.{name} span`
- Tests added (all PASS):
  - `TestAddSkillTool_EmitsKernelToolSpan`
  - `TestAddSkillTool_NoopTracerSafeNoPanic`
  - `TestAddSkillTool_RecordErrorOnExecutorError`

## Non-listed test files that needed updating

Only `internal/mcp/server_test.go` (in-package tests for the server itself) needed updating to pass `nil` for the new tracer parameter. No other production or test files in the repo construct `SerenaMCPServer` directly — verified via:

```
grep -rn "NewSerenaMCPServer" --include="*.go" .
```

returning only `internal/mcp/server.go`, `internal/mcp/server_test.go`, `internal/mcp/server_trace_test.go`, and `internal/daemon/daemon.go`.

## Deviations from Plan

None — plan executed exactly as written. The plan suggested adding tests either to `server_test.go` or a new `server_trace_test.go`; the new file path was chosen for clean separation of tracing concerns (mirrors `internal/mcp/telemetry_span_test.go`).

Implementation note: the plan's Step 3 closure sketch is reproduced verbatim in `server.go`. The action surfaces a benign `_ = ctx` line because `SkillToolExecutor.ExecuteTool` does not consume `ctx`; the line documents the intent (ctx is propagated for future executors that may need it) and is consistent with the WrapToolSpan idiom of carrying ctx through the wrapper even when the inner handler ignores it. This does not violate any plan acceptance criterion.

## Threat Flags

None — change is observability-only, no new network surface, no new auth path, no schema change.

## Self-Check

- [x] `internal/mcp/server.go` modified (tracer field + AddSkillTool wrap)
- [x] `internal/mcp/server_test.go` modified (nil tracer in two call sites)
- [x] `internal/daemon/daemon.go` modified (observability.Tracer() threaded)
- [x] `internal/mcp/server_trace_test.go` created
- [x] Both commits present in `git log`:
  - `faa9da00` — test (RED)
  - `30ea6b0b` — feat (GREEN)
- [x] `go vet ./...` PASS
- [x] `go test ./...` PASS

## TDD Gate Compliance

- RED gate: `faa9da00 test(55-02): add failing tests for AddSkillTool kernel.tool.{name} span` — confirmed failing build before implementation.
- GREEN gate: `30ea6b0b feat(55-02): wrap AddSkillTool handlers in kernel.tool.{name} span` — confirmed passing after implementation.
- REFACTOR gate: not needed; implementation is minimal and matches the plan's Step 3 sketch directly.

## Self-Check: PASSED
