---
phase: "66"
plan: "04"
subsystem: mcp-middleware
tags: [guardrails, middleware, telemetry, lifo, daemon, mcp]
dependency_graph:
  requires: ["66-01", "66-02", "66-03"]
  provides: ["guardrail-middleware-shell", "daemon-step-14b5", "outcome-enum-v9"]
  affects: ["internal/mcp", "internal/daemon", "internal/skill/guardrails"]
tech_stack:
  added:
    - "internal/mcp/guardrail_middleware.go — GuardrailMiddleware + MiddlewareDeps interface"
    - "internal/mcp/guardrail_middleware_test.go — 6 middleware tests"
    - "internal/daemon/guardrail_deps.go — production MiddlewareDeps impl (guardrailDepsImpl)"
    - "internal/skill/guardrails/skill.go — Caddy-style init() registration stub"
    - "internal/skill/guardrails/doc.go — package documentation"
  patterns:
    - "Duck typing via interface satisfaction across packages (avoids import cycle)"
    - "250ms context.WithTimeout goroutine pattern for fail-open evaluation"
    - "guardrailWarningSentinel string prefix in CallToolResult.Content for warn-mode detection"
    - "LIFO middleware composition test via pure function closures"
key_files:
  created:
    - "internal/mcp/guardrail_middleware.go"
    - "internal/mcp/guardrail_middleware_test.go"
    - "internal/daemon/guardrail_deps.go"
    - "internal/skill/guardrails/skill.go"
    - "internal/skill/guardrails/doc.go"
  modified:
    - "internal/mcp/middleware.go"
    - "internal/mcp/middleware_test.go"
    - "internal/mcp/telemetry_middleware_test.go"
    - "internal/daemon/daemon.go"
    - "internal/daemon/imports.go"
decisions:
  - "MiddlewareDeps interface placed in internal/mcp (not internal/guardrails) to avoid guardrails→rules→guardrails import cycle"
  - "Production MiddlewareDeps implementation placed in internal/daemon to satisfy duck typing without circular imports"
  - "guardrailWarningSentinel prefix approach chosen over a typed wrapper result to avoid modifying CallToolResult schema"
  - "LIFO regression test uses pure closure composition to avoid mcpsdk.Server setup complexity"
  - "GuardrailsSkill registered via Caddy init() even though it exposes no tools — enables future blank-import side effects"
metrics:
  duration: "~120 minutes (across two sessions)"
  completed: "2026-05-09"
  tasks_completed: 2
  files_changed: 10
---

# Phase 66 Plan 04: MCP Guardrail Middleware — Summary

Wire Wave 1/2 primitives (Plans 01-03) behind a real MCP middleware layer: GuardrailMiddleware shell with isDestructiveTool gate, warn/block dispatch, MiddlewareDeps interface, production deps adapter in daemon, GuardrailsSkill registration, outcome enum extension to 9 values, and daemon step 14b.5 insertion.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | GuardrailMiddleware shell + isDestructiveTool + MiddlewareDeps | `3a7150cf` | guardrail_middleware.go, guardrail_deps.go, skill/guardrails/skill.go |
| 2 | TelemetryMiddleware enum extension + LIFO test + daemon step 14b.5 | `a9035de8` | middleware.go, middleware_test.go, daemon.go, imports.go |

## Middleware Shell Architecture

### MiddlewareDeps Interface (internal/mcp/guardrail_middleware.go)

```go
type MiddlewareDeps interface {
    Evaluate(ctx context.Context, toolName string, rawArgs json.RawMessage, profileName string) ([]rules.Decision, error)
    OnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64)
    Logger() *slog.Logger
}
```

Interface is defined in `internal/mcp` (not `internal/guardrails`) to break the import cycle:
- `guardrails` → `guardrails/rules` → `guardrails` (circular)
- Solution: duck typing — `guardrailDepsImpl` in `internal/daemon` satisfies `helixMCP.MiddlewareDeps` without importing `mcp`

### isDestructiveTool Gate (D-08 closed list)

Six separate case statements covering the closed list:
- `rename_symbol`, `safe_delete_symbol`, `replace_symbol_body`
- `fuzzy_edit`, `replace_in_file`, `delete_file`

Non-destructive tools pass through immediately (zero evaluation cost).

### Warn/Block Dispatch

- **Block path**: Returns `serr.NewGuardrailViolation(rule, message, required, suggested, seeAlso)` — middleware returns this error directly, tool call fails.
- **Warn path**: Appends a `TextContent` with `guardrailWarningSentinel = "__guardrail_warning__:"` prefix to the result. Tool succeeds but TelemetryMiddleware detects the sentinel and records `guardrail_warned` outcome.

### Fail-Open Timeout (T-66-21)

```go
const guardrailEvalTimeout = 250 * time.Millisecond

evalCtx, cancel := context.WithTimeout(ctx, guardrailEvalTimeout)
defer cancel()

select {
case res := <-done:
    return res.decisions, res.err
case <-evalCtx.Done():
    d.metrics.GuardrailEvalTimeoutInc()
    d.logger.Warn("guardrail eval timeout — fail-open", ...)
    return nil, nil
}
```

On timeout: increment `helix_guardrail_eval_timeout_total`, log at Warn, return nil decisions (fail-open — tool proceeds).

## Daemon Step 14b.5 Insertion

### Before (install order at daemon.go):

```
Step 14  → InstallMiddleware (Telemetry + ProfileFilter)
Step 14b → InstallSuggestionMiddleware (Suggestion)
Step 14c → InstallLazyInitMiddleware (LazyInit)
```

### After (with Phase 66 step 14b.5):

```
Step 14   → InstallMiddleware (Telemetry + ProfileFilter)
Step 14b  → InstallSuggestionMiddleware (Suggestion)
Step 14b.5 → InstallGuardrailMiddleware (Guardrail)   ← NEW
Step 14c  → InstallLazyInitMiddleware (LazyInit)
```

Because `AddReceivingMiddleware` uses LIFO composition, execution order on incoming requests is:

```
LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler
```

This satisfies the invariant: LazyInit runs first (workspace activated), Guardrail runs second (evaluation uses warm workspace), Telemetry runs last (classifies final outcome after all middlewares).

**Line numbers in daemon.go (after merge):**
- Line ~708: `InstallSuggestionMiddleware` call
- Line ~741: `InstallGuardrailMiddleware` call (step 14b.5 block)
- Line ~773: `InstallLazyInitMiddleware` call

## Outcome Enum Extension (7 → 9 values)

`internal/mcp/middleware.go` const block extended:

```go
const (
    outcomeSuccess        = "success"
    outcomeInvalidArgs    = "invalid_args"
    outcomeNotFound       = "not_found"
    outcomeCircuitOpen    = "circuit_open"
    outcomeLSCrash        = "ls_crash"
    outcomeTimeout        = "timeout"
    outcomeInternal       = "internal"
    outcomeGuardrailWarned  = "guardrail_warned"   // Phase 66 GUARD-01
    outcomeGuardrailBlocked = "guardrail_blocked"  // Phase 66 GUARD-04
)
```

`classifyOutcome()` now checks `errors.Is(err, serr.ErrGuardrailViolation)` before the `outcomeInternal` fallback, and checks `hasGuardrailWarning(ctr)` for warn-mode sentinel detection.

`outcomeEnum` slice updated from 7 to 9 entries.

## LIFO Regression Test

`TestMiddleware_LIFOExecutionOrder_Phase66` in `internal/mcp/middleware_test.go` composes a pure recording middleware chain manually (no mcpsdk.Server):

```
chain := telemetry(handler)
chain = suggestion(chain)
chain = guardrail(chain)
chain = lazyInit(chain)
```

Asserts execution order: `["lazy_init", "guardrail", "suggestion", "telemetry", "handler"]`

## GuardrailsSkill Registration

`internal/skill/guardrails/skill.go` registers via Caddy-style init():

```go
func init() { skill.Register(&GuardrailsSkill{}) }
func (s *GuardrailsSkill) Tools() []*mcp.ToolDef { return nil }
```

Blank import added to `internal/daemon/imports.go`:

```go
_ "github.com/agenthands/helix/internal/skill/guardrails"
```

## Test Coverage

All 6 guardrail middleware tests pass under `-race`:
- `TestGuardrailMiddleware_NonToolsCall_PassesThrough`
- `TestGuardrailMiddleware_NonDestructiveTool_PassesThrough`
- `TestGuardrailMiddleware_DestructiveAllow_PassesThrough`
- `TestGuardrailMiddleware_DestructiveWarn_AttachesWarning`
- `TestGuardrailMiddleware_DestructiveBlock_ReturnsGuardrailViolation`
- `TestGuardrailMiddleware_EvaluateTimeout_FailsOpen`

LIFO order test passes: `TestMiddleware_LIFOExecutionOrder_Phase66`

Enum test updated and passes: `TestClassifyOutcome_AllSevenEnumValuesExist` (now asserts 9 values)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Import cycle: guardrails → rules → guardrails**
- **Found during:** Task 1 implementation
- **Issue:** Original plan placed `MiddlewareDeps` in `internal/guardrails/deps.go`; this package imports `guardrails/rules` which in turn imports `guardrails`, creating a cycle
- **Fix:** Removed `internal/guardrails/deps.go`; moved `MiddlewareDeps` interface to `internal/mcp/guardrail_middleware.go`; moved production implementation (`guardrailDepsImpl`) to `internal/daemon/guardrail_deps.go`
- **Files modified:** guardrail_middleware.go, guardrail_deps.go (new location in daemon), removed deps.go from guardrails
- **Commit:** `3a7150cf`

**2. [Rule 1 - Bug] CallToolRequest.Params type mismatch in test helper**
- **Found during:** Task 1 test compilation
- **Issue:** Test `makeCallToolRequest` used `*mcpsdk.CallToolParams` but `CallToolRequest = ServerRequest[*CallToolParamsRaw]`
- **Fix:** Changed to `&mcpsdk.CallToolParamsRaw{Name: toolName, Arguments: json.RawMessage(argsJSON)}`
- **Files modified:** guardrail_middleware_test.go
- **Commit:** `3a7150cf`

**3. [Rule 1 - Bug] LIFO test used non-existent mcpsdk.NewServer/NewInProcessClient APIs**
- **Found during:** Task 2 test compilation
- **Issue:** Initial LIFO test attempted to use SDK APIs that don't exist in the installed version
- **Fix:** Rewrote test to use pure function closure composition, avoiding all mcpsdk.Server setup
- **Files modified:** middleware_test.go
- **Commit:** `a9035de8`

**4. [Rule 1 - Bug] TestClassifyOutcome_AllSevenEnumValuesExist broke after adding 2 new outcomes**
- **Found during:** Task 2 test run
- **Issue:** Existing test asserted exactly 7 outcome enum values; adding guardrail_warned and guardrail_blocked caused it to fail
- **Fix:** Updated test to assert 9 values, explicitly listing the 2 Phase 66 additions
- **Files modified:** telemetry_middleware_test.go
- **Commit:** `a9035de8`

**5. [Rule 3 - Blocking] Worktree was missing Plans 01-03 work**
- **Found during:** Task 1 implementation (missing types from guardrails package)
- **Issue:** Worktree was based on commit `d60e40d3` but Plans 01-03 commits were on main at `25386115`
- **Fix:** `git merge main` fast-forward to bring Wave 1-3 commits into worktree
- **Commit:** N/A (merge commit)

## TODO(phase-66.x) Anchors

The following stubs are intentional and documented for future plans:

| Location | TODO | Plan |
|----------|------|------|
| `guardrail_deps.go:167` | Pre-warm catalogs at startup to avoid per-call load | phase-66.x |
| `guardrail_deps.go:175` | Thread active workspace key through session context for receipt validation | phase-66.x |
| `guardrail_deps.go:189` | Wire OnGraphVersionAdvance to actual graph-version publish/subscribe seam | phase-66.x |
| `guardrail_deps.go:207` | Replace noopOutlineProvider with real tree-sitter adapter | phase-66.x |
| `daemon.go step 14b.5` | Wire OnGraphVersionAdvance subscriber after workspace activation | phase-66.x |
| `guardrail_deps.go:179` | Wire GraphVersion from active workspace graph version | phase-66.x |

## Known Stubs

None that block the plan's goal. The GuardrailsSkill returns no tools (`nil`) intentionally — this skill exists solely for blank-import side effects (Caddy pattern). Future plans wire the skill to expose guardrail-management tools if needed.

## Threat Flags

None. No new network endpoints, auth paths, or schema changes at trust boundaries were introduced. GuardrailMiddleware sits fully within the existing MCP middleware stack and uses the existing `serr` error taxonomy.

## Pre-existing Test Failures (Out of Scope)

- `internal/kernel/jsonrpc TestConn_Call` — pre-existing, unrelated to Plan 04
- `test/bench` (2 tests) — pre-existing benchmark failures, unrelated to Plan 04
- These were present before any Plan 04 changes and are NOT caused by this plan

## Self-Check: PASSED

- `internal/mcp/guardrail_middleware.go` — FOUND
- `internal/mcp/guardrail_middleware_test.go` — FOUND
- `internal/daemon/guardrail_deps.go` — FOUND
- `internal/skill/guardrails/skill.go` — FOUND
- `internal/skill/guardrails/doc.go` — FOUND
- Task 1 commit `3a7150cf` — FOUND in git log
- Task 2 commit `a9035de8` — FOUND in git log
- `go build ./cmd/helix` — PASS
- `go test ./internal/mcp/... ./internal/daemon/...` — PASS (all new tests green)
