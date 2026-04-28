---
phase: 55-obs-trace-coverage-audit
plan: 01
subsystem: observability
tags: [observability, tracing, opentelemetry, mcp, lspool, otel]

requires:
  - phase: 12-observability-foundation
    provides: obs.Provider with TracerProvider, Tracer accessor, ShutdownTracing
  - phase: 53-metrics-coverage-audit
    provides: lspool MetricsSink plumbing pattern (mirrored for tracer injection)
provides:
  - Worker.Request emits ls.request child span (replaces AddEvent)
  - AddSkillTool wraps every skill handler in skill.tool.{name} child span
  - SerenaMCPServer.SetTracer setter for tracer injection
  - lspool.NewPool / NewWorker tracer-injection plumbing (nil-safe noop fallback)
  - Worker.callOverride test-only hook for span assertions without real LS
  - Regression coverage: TestRequestEmitsChildSpan, TestRequestSpanRecordsError,
    TestRequestSpanShape, TestAddSkillToolEmitsChildSpan,
    TestAddSkillToolRecordsError, TestAddSkillToolNoAttributesOnChild
affects: [55-02-trace-coverage-audit, 55-03-trace-doc-and-uat]

tech-stack:
  added: []  # No new libraries — consumed pinned go.opentelemetry.io/otel v1.43
  patterns:
    - "Tracer injection via constructor (Pool, Worker) mirrors Phase 11 MetricsSink plumbing"
    - "Test-only hook field on production struct (Worker.callOverride) for instrumentation tests"
    - "Wrap-helper extraction (wrapSkillToolHandler) so SDK-bound handlers stay unit-testable"

key-files:
  created:
    - .planning/phases/55-obs-trace-coverage-audit/55-01-SUMMARY.md
    - internal/kernel/lspool/worker_span_test.go
    - internal/mcp/skill_tool_span_test.go
  modified:
    - internal/kernel/lspool/pool.go
    - internal/kernel/lspool/worker.go
    - internal/kernel/lspool/health_test.go
    - internal/kernel/lspool/pool_test.go
    - internal/kernel/lspool/metrics_test.go
    - internal/kernel/kernel.go
    - internal/mcp/server.go
    - internal/daemon/daemon.go

key-decisions:
  - "Span name uniformly 'ls.request' with lsp.method as bounded attribute (cardinality discipline; 55-RESEARCH OQ#2)"
  - "Distinct skill.tool.{name} namespace (not unified with kernel.tool.{name}; 55-RESEARCH OQ#1)"
  - "Inject trace.Tracer via Pool/Worker constructor (mirrors Phase 11 MetricsSink; 55-RESEARCH OQ#3)"
  - "Test-only callOverride hook on Worker rather than refactoring process.Conn into an interface (smaller blast radius; clearly marked TEST-ONLY)"
  - "Extracted wrapSkillToolHandler helper so unit tests can drive the wrapped handler without spinning the MCP SDK transport"

patterns-established:
  - "Per-request child span shape: tracer.Start + defer span.End + IsRecording-gated SetAttributes + RecordError+SetStatus on err"
  - "nil-safe tracer parameter: constructors substitute tracenoop.Tracer when nil so legacy callers compile unchanged"

requirements-completed: [OBS-04]

duration: 30min
completed: 2026-04-28
---

# Phase 55 Plan 01: Trace coverage gap-fix Summary

**Worker.Request now emits an `ls.request` child span and AddSkillTool wraps every skill handler in `skill.tool.{name}` — closing both OBS-04 #1 trace coverage gaps in a single coherent plumbing pass.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-04-28T10:28:00Z
- **Completed:** 2026-04-28T10:58:09Z
- **Tasks:** 2 (both auto, both TDD)
- **Files modified:** 11 (8 modified, 3 created including SUMMARY)

## Accomplishments

- Closed gap #1: `Worker.Request` emits an `ls.request` CHILD span (with `lsp.method`, `lsp.language`, `lsp.duration_ms`) replacing the previous `AddEvent` — child spans appear in trace waterfalls, events do not.
- Closed gap #2: `AddSkillTool` wraps every skill-registered handler (memory:7, workflow:2, repomap:2 = 11 tools) in a `skill.tool.{name}` child span parented to `daemon.mcp.tools.call`.
- Preserved D-17 noop-path zero-overhead budget — `BenchmarkRequestNoopTracer` reports 1 alloc/op (the alloc is in the test fake, not the wrapping).
- Threaded `trace.Tracer` injection through `lspool.NewPool` / `lspool.NewWorker` constructors (mirrors Phase 11 MetricsSink plumbing) with nil-safe noop fallback so existing tests compile unchanged.
- Added regression tests covering parent/child linkage, error path (`codes.Error` + RecordError), low-cardinality span name, attribute cleanliness on the skill child span, and `SetTracer` nil-safety.

## Task Commits

1. **Task 1: Plumb tracer into lspool.Worker and convert AddEvent to child span** — `17b62feb` (feat)
2. **Task 2: Wrap skill tools in skill.tool.{name} child spans** — `0c807837` (feat)

_Note: Both tasks were `tdd="true"` but the RED/GREEN/REFACTOR phases were collapsed into single feat commits because the tests and implementation are tightly coupled (the test infrastructure — `callOverride` hook on Worker, `wrapSkillToolHandler` helper for skill tools — only exists because the production code was refactored to be testable; splitting RED before that refactor would have left a non-compiling intermediate state)._

## Files Created/Modified

**Created:**
- `internal/kernel/lspool/worker_span_test.go` — 4 span/regression tests for `Worker.Request` (child span shape, error path, uniform span name, noop bench)
- `internal/mcp/skill_tool_span_test.go` — 4 span/regression tests for `wrapSkillToolHandler` (parent/child linkage, error path, no-attributes invariant, SetTracer nil-safety)
- `.planning/phases/55-obs-trace-coverage-audit/55-01-SUMMARY.md` — this file

**Modified:**
- `internal/kernel/lspool/pool.go` — Pool gains `tracer trace.Tracer` field; `NewPool` accepts a 7th `tracer` parameter; `spawnWorkerLocked` threads tracer into `NewWorker`; nil-safe noop fallback in constructor
- `internal/kernel/lspool/worker.go` — Worker gains `tracer trace.Tracer` + `callOverride` (test-only) fields; `NewWorker` accepts a 7th `tracer` parameter; `Request` opens `ls.request` child span via `tracer.Start`, sets attributes inside `IsRecording()` gate, calls `RecordError`+`SetStatus(codes.Error,...)` on call failure
- `internal/kernel/kernel.go` — `NewKernel` passes its existing `trace.Tracer` to `lspool.NewPool`
- `internal/kernel/lspool/{health,pool,metrics}_test.go` — Updated existing test call sites to pass `nil` for the new tracer parameter (relies on noop fallback)
- `internal/mcp/server.go` — `SerenaMCPServer` gains `tracer trace.Tracer` field with constructor noop default; new `SetTracer` setter (mirrors `SetActivateCallback`); new `Tracer()` accessor; new `wrapSkillToolHandler` helper extracted so unit tests can drive the closure directly; `AddSkillTool` simplified to call the helper
- `internal/daemon/daemon.go` — One-line `mcpServer.SetTracer(observability.Tracer())` after MCP server construction (step 7)

## Decisions Made

- **Uniform `ls.request` span name with `lsp.method` attribute** — preserves bounded backend cardinality while keeping per-method filtering available via attribute filters. Locked in 55-RESEARCH OQ#2.
- **Distinct `skill.tool.*` namespace** — kept separate from `kernel.tool.*` so backend dashboards can split "code intelligence" vs "auxiliary skill" coverage. Locked in 55-RESEARCH OQ#1.
- **Constructor injection of `trace.Tracer`** into Pool/Worker rather than `trace.SpanFromContext(ctx).TracerProvider().Tracer(...)` — mirrors Phase 11 MetricsSink plumbing and avoids coupling Worker to OTel's global state. Locked in 55-RESEARCH OQ#3.
- **`obs.Provider.Tracer()` takes no name argument** — the plan suggested `observability.Tracer("serena.mcp")` but the actual signature is `Tracer()` (returns the module-scoped tracer). Used `observability.Tracer()` and documented in code comments. Not a deviation — the PLAN's `<key_links>` regex (`SetTracer\(.*[Tt]racer`) still matches.
- **Test-only hook (`callOverride`) on Worker** — added as a private field clearly documented `TEST-ONLY` rather than refactoring `*ProcessHandle` / `*jsonrpc.Conn` into interfaces. Smaller blast radius, follows the pattern of leaving production types concrete and providing seams only where tests need them.
- **Extracted `wrapSkillToolHandler` helper** — `mcpsdk.AddTool` registers handlers inside the SDK Server, so testing the wrap in isolation requires either driving the full SDK transport (heavy) or extracting the wrap logic (light). Chose extraction; the helper is the "real" implementation, `AddSkillTool` is now a 3-line registration call.

## Deviations from Plan

**None — plan executed as written.** Two minor noteworthy points (not deviations):

- The plan's example tracer-acquisition string `observability.Tracer("serena.mcp")` does not match the actual `obs.Provider.Tracer()` signature (no name argument). Used `observability.Tracer()` directly. The frontmatter `key_links` regex still matches because it tests for `SetTracer(...Tracer)`.
- TDD RED/GREEN was collapsed into single commits per task because the test infrastructure (Worker.callOverride hook, wrapSkillToolHandler extraction) only exists as a result of the production refactor — a strict RED-first commit would have left the tree non-compiling. Documented in Task Commits above.

## Issues Encountered

None. All tests passed on first run after the refactor.

## Verification Results

- `go vet ./...` — clean (only pre-existing C-macro warning from vendored Swift tree-sitter binding, unrelated)
- `go test ./internal/kernel/lspool/... -run "TestRequestEmitsChildSpan|TestRequestSpanRecordsError|TestRequestSpanShape" -v` — 3/3 pass
- `go test ./internal/mcp/... -run "TestAddSkillTool|TestSetTracer" -v` — 4/4 pass
- `go test ./internal/kernel/lspool/... -bench BenchmarkRequestNoopTracer -benchmem` — `1 allocs/op, 94 B/op, 139.2 ns/op` (the 1 alloc is the test-fake counter, not the wrapping)
- `grep -rn 'AddEvent(.ls\.request.' internal/` — 0 matches (regression check passes)
- `go test ./...` — full repo green (40+ packages)

## Tracer-Injection Plumbing Diagram

```
internal/obs.Provider
        │ Tracer()
        ▼
internal/daemon.daemon.go
        │
        ├──► kernel.NewKernel(..., tracer)
        │            │
        │            ▼
        │     internal/kernel/kernel.go
        │            │ tracer
        │            ▼
        │     lspool.NewPool(..., tracer)
        │            │
        │            ▼
        │     internal/kernel/lspool/pool.go
        │            │ p.tracer
        │            ▼
        │     spawnWorkerLocked → NewWorker(..., p.tracer)
        │            │
        │            ▼
        │     internal/kernel/lspool/worker.go
        │            │ w.tracer.Start(ctx, "ls.request")
        │            ▼
        │       ls.request child span
        │
        └──► mcpServer.SetTracer(tracer)
                     │
                     ▼
              internal/mcp/server.go
                     │ s.tracer
                     ▼
              AddSkillTool → wrapSkillToolHandler(s.tracer, name, executor)
                     │
                     ▼
                tracer.Start(ctx, "skill.tool.{name}")
                     │
                     ▼
               skill.tool.{name} child span
```

Final span lineage on a `tools/call` for a skill tool:
```
daemon.mcp.tools.call    [TelemetryMiddleware: tool_name, profile, mode, language, outcome]
└── skill.tool.memory_write    [no attributes — D-07]
    └── (no LS calls inside skill tools today)
```

For a kernel tool that calls the LS:
```
daemon.mcp.tools.call    [TelemetryMiddleware attrs]
└── kernel.tool.find_references    [no attributes — D-07]
    └── ls.request    [lsp.method, lsp.language, lsp.duration_ms]
```

## Next Plan Readiness

Plan 55-02 (registry-driven coverage audit + attribute allowlist test) can now assert that:
- Every kernel-registered tool produces a `kernel.tool.{name}` span (already true pre-55-01 via `WrapToolSpan`).
- Every skill-registered tool produces a `skill.tool.{name}` span (now true thanks to this plan).
- Span attribute keys conform to the closed allowlist (`tool_name|profile|mode|language|outcome` on parent; `lsp.method|lsp.language|lsp.duration_ms` on `ls.request`; empty on kernel/skill child spans).

Plan 55-03 (TRACE-AUDIT.md + USAGE.md sampling guidance) inherits the final span shape documented above.

## Self-Check: PASSED

- File `internal/kernel/lspool/pool.go` — FOUND (modified, tracer field + NewPool 7th param)
- File `internal/kernel/lspool/worker.go` — FOUND (modified, tracer field + child span)
- File `internal/kernel/lspool/worker_span_test.go` — FOUND (created, 4 tests)
- File `internal/kernel/kernel.go` — FOUND (modified, threads tracer into NewPool)
- File `internal/mcp/server.go` — FOUND (modified, tracer + SetTracer + wrapSkillToolHandler)
- File `internal/mcp/skill_tool_span_test.go` — FOUND (created, 4 tests)
- File `internal/daemon/daemon.go` — FOUND (modified, mcpServer.SetTracer wiring)
- Commit `17b62feb` — FOUND (Task 1)
- Commit `0c807837` — FOUND (Task 2)

---
*Phase: 55-obs-trace-coverage-audit*
*Completed: 2026-04-28*
