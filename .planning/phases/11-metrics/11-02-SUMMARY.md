---
phase: 11-metrics
plan: 02
subsystem: mcp-middleware
tags: [metrics, middleware, telemetry, observability, mcp, red]
requires: [11-01]
provides:
  - TelemetryMiddleware
  - classifyOutcome
  - outcomeEnum (7-value closed vocabulary)
  - SessionInfo.Language + SetLanguage
  - ClassifyOutcomeForTest / OutcomeEnumForTest
affects:
  - internal/mcp/session.go
  - internal/mcp/middleware.go
  - internal/mcp/server.go
  - internal/mcp/telemetry_middleware_test.go
  - internal/daemon/daemon.go
tech-stack:
  added:
    - prometheus/client_golang/prometheus/testutil (test-only, transitive via go.mod)
  patterns:
    - "Single-closure logging + metric emission (absorb-the-log-middleware)"
    - "Method-gated metric emission (tools/call only)"
    - "Closed outcome enum with hot-path-safe classification"
    - "Session snapshot atomicity under RWMutex for concurrent label reads"
key-files:
  created:
    - internal/mcp/telemetry_middleware_test.go
  modified:
    - internal/mcp/session.go
    - internal/mcp/middleware.go
    - internal/mcp/server.go
    - internal/daemon/daemon.go
    - go.mod
    - go.sum
decisions:
  - "Outcome enum is 7 values (success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal). 'denied' is intentionally absent: ProfileFilterMiddleware only filters tools/list in v1.2 so there is no rejection path at tools/call time."
  - "TelemetryMiddleware fully subsumes the Phase 8 logging closure; the old loggingMiddleware symbol is deleted so we traverse the middleware chain exactly once per request."
  - "Metric emission is method-gated on tools/call; tools/list, initialize and all other methods are pure log pass-through in v1.2 (T-11-09 accept disposition)."
  - "v1.2 only produces {success, timeout, circuit_open, internal} on the hot path. invalid_args / not_found / ls_crash are pre-declared as TODOs pending typed errors from the kernel in v1.3."
  - "MCP SDK accessor resolved: req.(*mcpsdk.CallToolRequest).Params.Name — CallToolRequest is a type alias for ServerRequest[*CallToolParamsRaw] so the Params pointer exposes Name directly."
  - "InstallMiddleware now owns both TelemetryMiddleware and ProfileFilterMiddleware wiring. Ordering is independent between the two because they handle disjoint methods; the old D-06 ordering constraint from CONTEXT.md is documented as obsolete inline."
  - "activate_project callback publishes the resolved primary workspace language into SessionInfo via SetLanguage so the 'language' metric label populates from the first workspace activation instead of remaining empty until a future wire-up."
metrics:
  duration_minutes: ~20
  tasks_completed: 2
  completed: "2026-04-09T21:16:32Z"
commits:
  - 5ec5a0b6: "feat(11-02): add Language field to SessionInfo for metric labels"
  - 17072a7f: "feat(11-02): add TelemetryMiddleware with RED metrics and 7-value outcome enum"
---

# Phase 11 Plan 02: MCP Telemetry Middleware Summary

TelemetryMiddleware now wraps every MCP request, emitting `serena_tool_calls_total` and `serena_tool_duration_seconds` with 5 labels on every `tools/call` while preserving the Phase 8 `request handled` / `request failed` log lines for every method in a single closure.

## What Shipped

### 1. SessionInfo.Language (Task 1)

`internal/mcp/session.go` now carries a `Language` field guarded by the existing `sync.RWMutex`. A new `SetLanguage(string)` setter mirrors `SetAllowedTools`, and `Snapshot()` copies the field inside the existing critical section so Phase 11 middleware gets a consistent read across profile / mode / language. Empty string is a valid Prometheus label value, so no branching is needed for the pre-activation window.

### 2. TelemetryMiddleware (Task 2)

`internal/mcp/middleware.go` exports `TelemetryMiddleware(provider *obs.Provider, getSession func(ctx) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware`. The closure:

1. Times the downstream handler.
2. Logs `request handled` / `request failed` with method + duration (Phase 8 regression guard — every test asserts the log line is preserved).
3. Returns early for any method != `"tools/call"`. No metric emission outside tool calls.
4. For tool calls, extracts the tool name from `req.(*mcpsdk.CallToolRequest).Params.Name`, resolves `profile` / `mode` / `language` from a single `SessionInfo.Snapshot()` under RLock, maps `(result, err)` through `classifyOutcome` into one of the 7 closed enum values, then calls `m.ToolCalls.WithLabelValues(...).Inc()` and `m.ToolDuration.WithLabelValues(...).Observe(duration.Seconds())`.

Classification order (cheapest checks first, most specific error first):

| Condition | Outcome |
|-----------|---------|
| `errors.Is(err, context.DeadlineExceeded)` | `timeout` |
| `errors.Is(err, lspool.ErrCircuitOpen)` | `circuit_open` |
| any other `err != nil` | `internal` |
| `*CallToolResult{IsError: true}` | `internal` |
| else | `success` |

`invalid_args`, `not_found`, and `ls_crash` are declared in the outcome enum but deliberately unreached on the v1.2 hot path. They land in v1.3 when typed errors from the kernel let us distinguish them without reading `err.Error()` into a label.

### 3. Wiring Changes

- `internal/mcp/server.go`: `NewSerenaMCPServer` no longer installs any middleware — the daemon owns that after session resolution.
- `internal/mcp/middleware.go`: `InstallMiddleware` signature changed to `(server, provider, resolver, getSession, logger)` and installs TelemetryMiddleware plus ProfileFilterMiddleware in one call. A code comment documents that ordering is irrelevant between the two because they handle disjoint methods.
- `internal/daemon/daemon.go`: constructs the `obs.Provider` first, calls the new `InstallMiddleware` after the session + profile are wired, and extends the `activate_project` callback to publish the resolved primary language into the session via `SetLanguage`.

## Resolved Open Questions (from 11-RESEARCH.md)

- **A1 — MCP SDK accessor for tool name:** `CallToolRequest` is a type alias for `ServerRequest[*CallToolParamsRaw]` (see `github.com/modelcontextprotocol/go-sdk@v1.3.0/mcp/requests.go:10` and `protocol.go:56`). `ctr.Params.Name` is the direct accessor; `extractToolName` falls back to `"unknown"` on a type-assert miss or nil `Params`.
- **A2 — Session carries Language:** Task 1 added the field + snapshot copy + setter. The daemon wires it at `activate_project` time; concurrent writers are safe thanks to the existing RWMutex.

## Outcome Enum (Closed, 7 Values)

```
success
invalid_args   // TODO v1.3: typed validation errors
not_found      // TODO v1.3: symbol-lookup misses
circuit_open
ls_crash       // TODO v1.3: lspool crash signals
timeout
internal
```

`denied` is **not** in the enum. `TestClassifyOutcome_AllSevenEnumValuesExist` enforces both the positive shape (all 7 present) and the negative assertion (`denied` is absent).

## Test Coverage

`internal/mcp/telemetry_middleware_test.go` adds 10 tests:

| Test | Asserts |
|------|---------|
| `TestTelemetryMiddleware_toolsCallEmitsMetric` | Success path increments counter + records histogram sample with full 5-label set |
| `TestTelemetryMiddleware_timeout` | `context.DeadlineExceeded` → `outcome=timeout` |
| `TestTelemetryMiddleware_circuitOpen` | `lspool.ErrCircuitOpen` → `outcome=circuit_open` |
| `TestTelemetryMiddleware_internalError` | generic `errors.New` → `outcome=internal` |
| `TestTelemetryMiddleware_toolResultIsError` | `CallToolResult{IsError: true}` → `outcome=internal` |
| `TestTelemetryMiddleware_skipsNonToolsCall` | `tools/list` is NOT metered but IS logged |
| `TestTelemetryMiddleware_initializeMethod` | `initialize` is pass-through (no metric, log preserved) |
| `TestClassifyOutcome` | Table test over the 5 reachable outcomes |
| `TestClassifyOutcome_AllSevenEnumValuesExist` | Closed enum shape + `denied` negative assertion |
| `TestTelemetryMiddleware_usesSnapshot_race` | Concurrent `SetLanguage` + 1000 tool calls under `-race` produces no mismatched labels |
| `TestTelemetryMiddleware_loggingMiddlewareGone` | `middleware.go` does not contain the string `loggingMiddleware` |

All pass under `go test -race ./internal/mcp/... ./internal/daemon/...`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added go.sum entry for prometheus testutil dependency**
- **Found during:** Task 2 `go vet` — the linker complained `missing go.sum entry for module providing package github.com/kylelemons/godebug/diff (imported by .../testutil)`.
- **Fix:** Ran `go get github.com/prometheus/client_golang/prometheus/testutil@v1.23.2` to surface the transitive dependency. `go.mod` and `go.sum` were updated.
- **Files modified:** `go.mod`, `go.sum`
- **Commit:** `17072a7f` (rolled into Task 2 commit)

**2. [Rule 2 - Missing critical functionality] activate_project now populates session Language**
- **Found during:** Task 2 wiring review — Plan 11-02 leaves "active-workspace → session.Language wiring is Claude's discretion" in Task 1. Without the wire-up the `language` label would be permanently empty, making the entire label useless.
- **Fix:** Extended the existing `activate_project` callback in `internal/daemon/daemon.go` to call `sessionProvider.CurrentSession().SetLanguage(activeWSLang)` on every successful activation. The callback already resolves `activeWSLang = rt.Languages()[0]` so this is a one-line addition.
- **Files modified:** `internal/daemon/daemon.go`
- **Commit:** `17072a7f` (rolled into Task 2 commit)

**3. [Rule 3 - Blocking] Removed stale `InstallMiddleware(server, logger)` call from `NewSerenaMCPServer`**
- **Found during:** Task 2 signature change — the old call site in `server.go` used the 2-arg signature and would no longer compile after the signature widened.
- **Fix:** Deleted the call from `NewSerenaMCPServer`; the daemon now owns all middleware installation after the session provider and observability are resolved. The test-only constructor path (used by `TestNewSerenaMCPServer` and a handful of older MCP tests) runs without middleware, which matches intent because those tests never exercise logging or metrics.
- **Files modified:** `internal/mcp/server.go`
- **Commit:** `17072a7f`

## Threat Model Status

| Threat | Disposition | How Addressed |
|--------|-------------|---------------|
| T-11-06 (Info disclosure via err.Error() label) | mitigate | `classifyOutcome` never touches `err.Error()`; only closed-enum strings reach labels |
| T-11-07 (Race on label resolution) | mitigate | Single `Snapshot()` call under RLock pulls profile+mode+language together; verified by `TestTelemetryMiddleware_usesSnapshot_race` under `-race` |
| T-11-08 (Per-call string allocation) | mitigate | Hot path has zero `fmt.Sprintf`, uses closure-captured `m := provider.Metrics()` + `WithLabelValues`. Allocation delta benchmark is a plan 11-04 concern |
| T-11-09 (Coverage gap for non-tool-call methods) | accept | tools/list + initialize remain log-only per v1.2 scope |

## Constraints Respected

- `git diff internal/obs/metrics.go` → empty (Metrics surface from Plan 01 is untouched).
- `grep 'fmt.Sprintf' internal/mcp/middleware.go` → zero matches.
- `grep 'loggingMiddleware' internal/mcp/middleware.go internal/mcp/server.go internal/daemon/daemon.go` → zero matches across all three files.
- `grep '"denied"' internal/mcp/middleware.go` → zero matches (the literal string is absent even from comments).
- `go vet ./...` clean, `go test -race ./internal/mcp/... ./internal/daemon/...` green, `go build ./cmd/serena` succeeds.

## Self-Check: PASSED

- FOUND: internal/mcp/session.go (Language field + SetLanguage + Snapshot copy)
- FOUND: internal/mcp/middleware.go (TelemetryMiddleware + classifyOutcome + 7-value enum)
- FOUND: internal/mcp/telemetry_middleware_test.go (10 test functions)
- FOUND: internal/mcp/server.go (middleware install removed from constructor)
- FOUND: internal/daemon/daemon.go (new InstallMiddleware call + SetLanguage on activate)
- FOUND: commit 5ec5a0b6 (SessionInfo.Language)
- FOUND: commit 17072a7f (TelemetryMiddleware)
- VERIFIED: `git diff internal/obs/metrics.go` is empty
- VERIFIED: `go test -race ./internal/mcp/... ./internal/daemon/...` passes
