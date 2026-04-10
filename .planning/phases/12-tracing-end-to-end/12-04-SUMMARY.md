---
phase: 12-tracing-end-to-end
plan: 04
subsystem: kernel-tracing
tags: [tracing, otel, kernel, lspool, span-events]
dependency_graph:
  requires: [12-01, 12-02]
  provides: [kernel-tool-spans, ls-request-events, kernel-tracer-plumbing]
  affects: [internal/kernel, internal/kernel/symbols, internal/kernel/edit, internal/kernel/fileops, internal/kernel/diag, internal/kernel/lspool, internal/daemon]
tech_stack:
  added: []
  patterns: [WrapToolSpan-generic-helper, span-event-not-child-span, IsRecording-gate]
key_files:
  created:
    - internal/kernel/spanwrap.go
    - internal/kernel/spanwrap_test.go
  modified:
    - internal/kernel/kernel.go
    - internal/kernel/symbols/tools.go
    - internal/kernel/edit/tools.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/diag/tools.go
    - internal/kernel/lspool/worker.go
    - internal/daemon/daemon.go
decisions:
  - "Tracer plumbed as positional constructor parameter on NewKernel (not Deps struct -- matches existing positional style)"
  - "WrapToolSpan uses Go generics with mcpsdk.ToolHandlerFor[In, Out] type alias for zero-friction wrapping"
  - "fileops and diag packages accept tracer as explicit parameter (no circular kernel import needed since kernel does not import sub-packages)"
  - "ls.request uses span.AddEvent (not tracer.Start) per D-06 contract"
metrics:
  duration_seconds: 530
  completed: "2026-04-10T06:50:04Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 7
---

# Phase 12 Plan 04: Kernel Tool Sub-Spans + LS Span Events Summary

Kernel constructor accepts a trace.Tracer via positional parameter; generic WrapToolSpan helper wraps all 24 kernel tool handlers at registration time producing kernel.tool.{name} sub-spans; lspool.Worker.Request emits ls.request span events gated on IsRecording.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | e98dbaf6 | Add tracer to Kernel + WrapToolSpan helper + wire daemon |
| 2 | de06dae0 | Wrap all 24 kernel tools with WrapToolSpan + ls.request events |

## Implementation Details

### Kernel Tracer Plumbing (Task 1)

- Extended `Kernel` struct with `tracer trace.Tracer` field
- `NewKernel` gains a `tracer trace.Tracer` positional parameter (last argument)
- Nil-guard: falls back to `tracenoop.NewTracerProvider().Tracer("kernel-fallback")`
- `Kernel.Tracer()` accessor returns the tracer (never nil)
- `daemon.New` passes `observability.Tracer()` to `NewKernel`

### WrapToolSpan Helper (Task 1)

- Located in `internal/kernel/spanwrap.go`
- Generic signature: `WrapToolSpan[In, Out any](tracer, toolName, fn ToolHandlerFor[In, Out]) ToolHandlerFor[In, Out]`
- Creates `kernel.tool.{toolName}` span via `tracer.Start`
- Records errors via `span.RecordError` only when `span.IsRecording()` (D-17)
- NO attributes set on kernel spans (D-07 cardinality contract)
- 4 tests: SpanCreated, NoopDoesNotPanic, ErrorRecorded, ParentSpanLinked

### Tool Wrapping (Task 2)

All 24 AddTool call sites wrapped:
- **symbols/tools.go**: 9 tools (go_to_definition, find_references, get_symbol_overview, search_symbols, get_hover_info, find_implementations, get_call_hierarchy, get_type_hierarchy, analyze_blast_radius)
- **edit/tools.go**: 6 tools (replace_symbol_body, insert_before_symbol, insert_after_symbol, rename_symbol, safe_delete_symbol, verify_edit)
- **fileops/tools.go**: 6 tools (read_file, create_file, list_directory, find_files, search_in_files, replace_in_file)
- **diag/tools.go**: 3 tools (get_diagnostics, get_code_actions, format_code)

### LS Span Events (Task 2)

- `lspool.Worker.Request` emits `ls.request` span event AFTER the JSON-RPC call completes
- Gated on `span.IsRecording()` (D-17 hot-path budget)
- Attributes: `lsp.method`, `lsp.language`, `lsp.duration_ms` only
- Uses `trace.SpanFromContext(ctx).AddEvent` -- NOT `tracer.Start` (D-06: event, not child span)
- No sensitive data (no file paths, URIs, symbol names, or params)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] fileops/diag tracer parameter approach**
- **Found during:** Task 2
- **Issue:** fileops and diag packages don't hold a kernel reference, so `k.Tracer()` was unavailable
- **Fix:** Pass `trace.Tracer` as explicit parameter to `RegisterTools` and `observability.Tracer()` from daemon call site. Import `kernel` package for `WrapToolSpan` access (no circular dependency since kernel doesn't import sub-packages).
- **Files modified:** internal/kernel/fileops/tools.go, internal/kernel/diag/tools.go, internal/daemon/daemon.go
- **Commit:** de06dae0

## Self-Check: PASSED

All 9 files verified present. Both commits (e98dbaf6, de06dae0) verified in git log. Build and all tests pass.
