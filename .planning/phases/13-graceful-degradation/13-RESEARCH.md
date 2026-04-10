# Phase 13: Graceful Degradation - Research

**Researched:** 2026-04-10
**Domain:** Go resilience patterns (circuit breaking, deadline propagation, graceful shutdown)
**Confidence:** HIGH

## Summary

Phase 13 hardens Serena's existing 38-tool surface against LS worker failures, slow responses, and shutdown races. The codebase already has the key integration points wired: `CircuitBreaker` in lspool (needs jitter + restart budget + single-probe), `TelemetryMiddleware` in mcp (needs deadline wrapping before `next()`), `classifyOutcome` (already routes `ErrCircuitOpen` and `DeadlineExceeded`), and `shutdown.go` (two-phase shutdown with tracing flush).

The work is primarily enhancement of existing code, not greenfield. The `internal/degrade/` package is the only new package -- it owns the tool-to-class mapping and timeout budget constants. Everything else is modification: circuit breaker gains decorrelated jitter and restart budget, middleware gains `context.WithTimeout`, config gains `DegradationConfig`, and daemon gets `runtime/debug.SetMemoryLimit` wiring.

**Primary recommendation:** Build `internal/degrade/` first (tool-class map + budget constants), then wire deadlines into TelemetryMiddleware, then enhance CircuitBreaker, then wire config, then write the SIGTERM integration test last (it validates everything end-to-end).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Deadlines originate in the daemon's TelemetryMiddleware. The middleware wraps `context.WithTimeout` per tool class BEFORE calling the kernel handler. Single enforcement point. Forwarder passes through unchanged.
- **D-02:** Per-class timeout budgets are configurable via project.yml with hardcoded defaults: read 5s / search 15s / edit 10s / index 120s / diagnostics 20s. Config path: `degradation.timeout_{class}`.
- **D-03:** Tool-to-class mapping is a closed map in `internal/degrade/` -- each of the 28 tools maps to one of 5 classes (read, search, edit, index, diagnostics).
- **D-04:** Decorrelated jitter for backoff: `sleep = min(cap, random_between(base, sleep*3))`. AWS-recommended pattern prevents thundering herd when multiple workers crash simultaneously.
- **D-05:** Restart budget: 3 consecutive crashes then circuit stays open until TTL expiry or manual intervention. Prevents restart loops burning CPU.
- **D-06:** Single-probe half-open: when backoff expires, exactly one request is admitted as a probe. If it succeeds -> close circuit. If it fails -> reopen with increased backoff.
- **D-07:** Typed `ErrCircuitOpen` carries: Language, BackoffRemaining, Failures, RetryAfter. Structured envelope returned in MCP error response with retry-after hint.
- **D-08:** `classifyOutcome` already handles `ErrCircuitOpen` and `DeadlineExceeded`. Phase 13 makes them produce real typed errors instead of generic Go errors.
- **D-09:** Config-driven `runtime/debug.SetMemoryLimit` wired from `degradation.memory_limit` in project.yml. Two layers: Go GC budget + existing platform pressure eviction operate independently.
- **D-10:** SIGTERM triggers errgroup context cancellation. In-flight tool calls get their context cancelled. `ShutdownTimeout` bounds the drain. Kernel.Shutdown drains workers. Tracing flush follows with dedicated 5s context.
- **D-11:** Two-stage shutdown ordering preserved from Phase 12: kernel drain -> tracing flush -> listener close -> socket cleanup.

### Claude's Discretion
- Tool-to-class mapping assignments (which tools go in which timeout class)
- Specific jitter algorithm parameters (base, cap values)
- Config key naming for degradation settings
- Integration test design for SIGTERM mid-request drain

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DEGRADE-01 | `internal/degrade/` package with per-class timeout budgets | New package with ToolClass enum, tool-to-class map, Budget() func; wired into TelemetryMiddleware |
| DEGRADE-02 | Deadline propagation from forwarder -> daemon -> kernel -> LS | context.WithTimeout in TelemetryMiddleware before next(); forwarder passes through unchanged (D-01) |
| DEGRADE-03 | Typed `lspool.ErrCircuitOpen` error with structured envelope | Replace sentinel error with typed CircuitOpenError struct carrying Language/BackoffRemaining/Failures/RetryAfter |
| DEGRADE-04 | Circuit breaker tuning with decorrelated jitter and single-probe half-open | Modify CircuitBreaker.RecordFailure to use decorrelated jitter; CanAttempt becomes single-probe with atomic flag |
| DEGRADE-05 | LS crash recovery with restart budget | Add RestartBudget field to CircuitBreaker; after 3 consecutive failures, circuit stays open permanently |
| DEGRADE-06 | `runtime/debug.SetMemoryLimit` wired from config | DegradationConfig.MemoryLimitMB in config; daemon.New calls debug.SetMemoryLimit early |
| DEGRADE-07 | Graceful shutdown integration test (SIGTERM mid-request, spans flushed) | Integration test in daemon package: start daemon, inject slow tool, send SIGTERM, assert drain + span flush |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `context` (stdlib) | Go 1.25.1 | Deadline propagation via WithTimeout | Built-in, zero deps, already used everywhere |
| `runtime/debug` (stdlib) | Go 1.25.1 | SetMemoryLimit for GC pressure | Stdlib since Go 1.19, the standard way to set soft memory limits |
| `math/rand/v2` (stdlib) | Go 1.25.1 | Decorrelated jitter randomness | Go 1.22+ rand/v2 auto-seeds, no manual seed needed |
| `sync/atomic` (stdlib) | Go 1.25.1 | Single-probe half-open flag | Lock-free probe admission |

### Supporting
No new external dependencies required. Phase 13 uses only stdlib and existing project dependencies. [VERIFIED: codebase inspection]

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled circuit breaker | sony/gobreaker | External dep; existing CB is 90% there, just needs jitter+budget |
| context.WithTimeout | per-handler timers | D-01 mandates single enforcement point in middleware |

## Architecture Patterns

### New Package Structure
```
internal/degrade/
    budget.go       # ToolClass enum, tool-to-class map, Budget() func, DegradationConfig
    budget_test.go  # Table-driven tests for all 38 tools mapped
```

### Modified Files
```
internal/kernel/lspool/circuit.go    # Decorrelated jitter, restart budget, single-probe
internal/kernel/lspool/circuit_err.go # Typed CircuitOpenError (replaces sentinel)
internal/mcp/middleware.go            # context.WithTimeout wrapping in TelemetryMiddleware
internal/config/config.go            # DegradationConfig struct
internal/config/defaults.go          # Default timeout budgets + memory limit
internal/daemon/daemon.go            # Wire degrade config, SetMemoryLimit
internal/daemon/shutdown_test.go     # SIGTERM integration test (DEGRADE-07)
```

### Pattern 1: Deadline Wrapping in Middleware
**What:** TelemetryMiddleware calls `degrade.Budget(toolName)` to get the timeout, wraps `ctx` with `context.WithTimeout`, then calls `next(ctx, method, req)`.
**When to use:** Every tools/call invocation.
**Example:**
```go
// In TelemetryMiddleware, after extracting toolName, before calling next:
budget := degrade.Budget(toolName, cfg)
if budget > 0 {
    var budgetCancel context.CancelFunc
    ctx, budgetCancel = context.WithTimeout(ctx, budget)
    defer budgetCancel()
}
result, err := next(ctx, method, req)
```
[VERIFIED: middleware.go lines 141-146 show the exact insertion point]

### Pattern 2: Decorrelated Jitter Backoff
**What:** AWS-recommended jitter pattern: `sleep = min(cap, random_between(base, sleep*3))`. Prevents thundering herd.
**When to use:** CircuitBreaker.RecordFailure.
**Example:**
```go
// Decorrelated jitter (D-04):
// sleep = min(cap, random_between(base, prevSleep*3))
base := time.Second
prevSleep := cb.backoff
if prevSleep < base {
    prevSleep = base
}
jittered := base + time.Duration(rand.Int64N(int64(prevSleep*3-base)))
if jittered > cb.maxBackoff {
    jittered = cb.maxBackoff
}
cb.backoff = jittered
```
[CITED: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/ — decorrelated jitter formula]

### Pattern 3: Typed Error Replacing Sentinel
**What:** Replace `var ErrCircuitOpen = errors.New(...)` with a struct type that carries metadata. Keep `errors.Is` compatibility via `Is()` method.
**When to use:** DEGRADE-03.
**Example:**
```go
// CircuitOpenError is the typed replacement for the sentinel ErrCircuitOpen.
type CircuitOpenError struct {
    Language         string
    BackoffRemaining time.Duration
    Failures         int
    RetryAfter       time.Time
}

func (e *CircuitOpenError) Error() string {
    return fmt.Sprintf("circuit breaker open for %s: %d failures, retry after %s",
        e.Language, e.Failures, e.RetryAfter.Format(time.RFC3339))
}

// Is allows errors.Is(err, ErrCircuitOpen) to keep working.
func (e *CircuitOpenError) Is(target error) bool {
    return target == ErrCircuitOpen
}
```
[VERIFIED: pool.go line 39 shows current sentinel; middleware.go line 83 shows errors.Is check]

### Pattern 4: Single-Probe Half-Open
**What:** When backoff expires, `CanAttempt()` returns true for exactly one caller; concurrent callers get false until the probe completes.
**When to use:** DEGRADE-04/D-06.
**Example:**
```go
// Add to CircuitBreaker:
probing atomic.Bool

func (cb *CircuitBreaker) CanAttempt() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    if cb.failures == 0 {
        return true
    }
    if time.Since(cb.lastFailure) >= cb.backoff {
        // Single probe: only one goroutine gets through
        if cb.probing.CompareAndSwap(false, true) {
            cb.setStateLocked(CircuitHalfOpen)
            return true
        }
    }
    return false
}

// RecordSuccess/RecordFailure reset probing flag
```
[VERIFIED: circuit.go lines 81-93 show current CanAttempt; needs atomic probe guard]

### Anti-Patterns to Avoid
- **Deadline in every tool handler:** D-01 mandates a SINGLE enforcement point in middleware, not per-handler timeouts. This avoids deadline drift and inconsistent behavior.
- **Global rand seed:** Go 1.22+ `math/rand/v2` auto-seeds; never call `rand.Seed()`.
- **errors.As for classification in hot path:** `classifyOutcome` uses `errors.Is` which is cheaper. Keep it that way -- the typed error's `Is()` method handles the bridge.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Memory limit | Custom mmap monitoring | `runtime/debug.SetMemoryLimit` | Stdlib, GC-aware, respects GOMEMLIMIT env var |
| Jitter RNG | Custom PRNG | `math/rand/v2.Int64N` | Auto-seeded, fast, sufficient entropy for backoff |
| Timeout propagation | Per-handler timers | `context.WithTimeout` | Stdlib, composable, already used in codebase |

## Common Pitfalls

### Pitfall 1: Deadline Already Set by Caller
**What goes wrong:** If a client or forwarder already set a shorter deadline on the context, `context.WithTimeout` creates a child that inherits the shorter deadline, making the tool-class budget ineffective.
**Why it happens:** context.WithTimeout takes the minimum of parent and new deadline.
**How to avoid:** This is actually correct behavior -- a client-imposed deadline SHOULD win. Document this: the budget is a server-side maximum, not a guarantee.
**Warning signs:** Tools returning timeout faster than their class budget.

### Pitfall 2: Probe Leak in Half-Open State
**What goes wrong:** If the probing goroutine panics or the result is never recorded, the circuit stays in half-open forever with the probe flag set.
**Why it happens:** No cleanup path for the probe flag.
**How to avoid:** Add a `defer` in the caller that resets `probing` if the request panics. Or use a timeout on the probe flag itself.
**Warning signs:** Circuit stuck in half-open state for longer than the configured timeout.

### Pitfall 3: ErrCircuitOpen Backward Compatibility
**What goes wrong:** Replacing sentinel `ErrCircuitOpen` with a struct type breaks code that does `err == ErrCircuitOpen`.
**Why it happens:** The sentinel is currently `var ErrCircuitOpen = errors.New(...)`. Code using `errors.Is` will work if the struct has an `Is()` method, but direct `==` comparison will break.
**How to avoid:** Keep the sentinel var for `errors.Is` compatibility. The typed struct's `Is()` method bridges the two. Grep the codebase for `== ErrCircuitOpen` (currently only `errors.Is` is used in middleware.go:83).
**Warning signs:** Tests that compare `err == ErrCircuitOpen` fail after the change.

### Pitfall 4: SetMemoryLimit vs GOMEMLIMIT
**What goes wrong:** Both `debug.SetMemoryLimit` and the `GOMEMLIMIT` env var control the same thing. If both are set, the last call wins.
**Why it happens:** `GOMEMLIMIT` is read by the runtime at init; `SetMemoryLimit` overrides it.
**How to avoid:** D-09 says config-driven. Call `SetMemoryLimit` in daemon.New only if the config value is non-zero. Log the value. Document that GOMEMLIMIT env var is also respected (and overridden by config).
**Warning signs:** OOM kills despite config setting, because GOMEMLIMIT was set higher elsewhere.

### Pitfall 5: Shutdown Test Flakiness
**What goes wrong:** Integration test sends SIGTERM but the daemon hasn't finished starting yet, or spans aren't flushed before assertion.
**Why it happens:** Race between daemon startup and signal delivery.
**How to avoid:** Wait for readiness (socket file exists, or `ready.Store(1)` has been called) before sending signal. Use tracetest.SpanRecorder for span assertions.
**Warning signs:** Test passes locally, fails in CI.

### Pitfall 6: Tool Count Mismatch (28 vs 38)
**What goes wrong:** CONTEXT.md says "28 tools" for the tool-class map, but the codebase has 38 registered tool names.
**Why it happens:** The 28 likely refers to kernel tools only (symbols 9 + edit 6 + fileops 6 + diag 3 = 24, plus a few more). The remaining are skill tools (memory 7, workflow 2, profile 3, ping 1).
**How to avoid:** Map ALL 38 tools. Skill tools that don't touch the LS (memory, workflow, profile) go in the "read" class (fastest timeout) since they're in-process only. Only LS-backed tools need the longer budgets.
**Warning signs:** Unmapped tool name returns zero budget, bypassing timeout entirely.

## Code Examples

### Tool-to-Class Mapping (Claude's Discretion)
```go
// internal/degrade/budget.go

type ToolClass string

const (
    ClassRead        ToolClass = "read"
    ClassSearch      ToolClass = "search"
    ClassEdit        ToolClass = "edit"
    ClassIndex       ToolClass = "index"
    ClassDiagnostics ToolClass = "diagnostics"
)

// toolClassMap maps every registered tool name to its timeout class.
// Unmapped tools default to ClassRead (tightest budget) as a safety net.
var toolClassMap = map[string]ToolClass{
    // Symbol retrieval (LS-backed reads)
    "go_to_definition":      ClassRead,
    "find_references":       ClassSearch,  // can be slow on large repos
    "get_hover_info":        ClassRead,
    "find_implementations":  ClassSearch,
    "get_call_hierarchy":    ClassSearch,
    "get_type_hierarchy":    ClassSearch,
    "analyze_blast_radius":  ClassSearch,
    "search_symbols":        ClassSearch,
    "get_symbol_overview":   ClassRead,

    // Symbol editing (LS-backed writes)
    "replace_symbol_body":   ClassEdit,
    "insert_before_symbol":  ClassEdit,
    "insert_after_symbol":   ClassEdit,
    "rename_symbol":         ClassEdit,
    "safe_delete_symbol":    ClassEdit,
    "verify_edit":           ClassEdit,

    // File ops (no LS, fast)
    "read_file":             ClassRead,
    "write_file":            ClassEdit,
    "create_file":           ClassEdit,
    "delete_file":           ClassEdit,
    "list_directory":        ClassRead,
    "find_files":            ClassSearch,
    "search_in_files":       ClassSearch,
    "replace_in_file":       ClassEdit,

    // Diagnostics (LS-backed)
    "get_diagnostics":       ClassDiagnostics,
    "get_code_actions":      ClassDiagnostics,
    "format_code":           ClassDiagnostics,

    // Memory (in-process, fast)
    "write_memory":          ClassRead,
    "read_memory":           ClassRead,
    "list_memories":         ClassRead,
    "search_memories":       ClassSearch,
    "edit_memory":           ClassRead,
    "delete_memory":         ClassRead,
    "rename_memory":         ClassRead,

    // Workflow (in-process, fast)
    "onboard_project":       ClassIndex,  // may trigger LS indexing
    "prepare_for_new_conversation": ClassRead,

    // Misc
    "ping":                  ClassRead,
    "body":                  ClassRead,
    "special":               ClassRead,
}
```
[VERIFIED: tool names from grep of codebase; class assignments are Claude's Discretion per CONTEXT.md]

### DegradationConfig
```go
// internal/config/config.go addition:
type DegradationConfig struct {
    TimeoutRead        int `koanf:"timeout_read"`        // seconds, default 5
    TimeoutSearch      int `koanf:"timeout_search"`      // seconds, default 15
    TimeoutEdit        int `koanf:"timeout_edit"`        // seconds, default 10
    TimeoutIndex       int `koanf:"timeout_index"`       // seconds, default 120
    TimeoutDiagnostics int `koanf:"timeout_diagnostics"` // seconds, default 20
    MemoryLimitMB      int `koanf:"memory_limit_mb"`     // 0 = don't set (use GOMEMLIMIT env if present)
    RestartBudget      int `koanf:"restart_budget"`      // default 3
}
```
[ASSUMED: config key names are Claude's Discretion]

### Middleware Deadline Injection
```go
// In TelemetryMiddleware, tools/call branch, BEFORE calling next():
toolName := extractToolName(req)
if budget := degrade.BudgetFor(toolName, d.degradeCfg); budget > 0 {
    var deadlineCancel context.CancelFunc
    ctx, deadlineCancel = context.WithTimeout(ctx, budget)
    defer deadlineCancel()
}

// Existing: result, err := next(ctx, method, req)
```
[VERIFIED: middleware.go line 145 is the insertion point]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `math/rand.Intn` | `math/rand/v2.Int64N` | Go 1.22 (2024) | Auto-seeded, no `rand.Seed()` needed |
| `errors.New` sentinel | Typed error struct + `Is()` method | Go 1.13+ | Carries metadata while preserving errors.Is compatibility |
| Manual GOMEMLIMIT env | `runtime/debug.SetMemoryLimit` | Go 1.19 (2022) | Programmatic soft memory limit for GC |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Config key naming `degradation.timeout_read` etc. | Code Examples | Low -- naming is Claude's Discretion, easily renamed |
| A2 | `onboard_project` should be ClassIndex (may trigger LS indexing) | Code Examples | Medium -- if it doesn't touch LS, ClassRead is safer |
| A3 | Memory/workflow skill tools should get ClassRead timeout | Code Examples | Low -- they're in-process, 5s is generous |

## Open Questions

1. **How does degrade config reach TelemetryMiddleware?**
   - What we know: TelemetryMiddleware is a closure capturing `provider` and `getSession`. It needs to also capture degrade config.
   - What's unclear: Pass DegradationConfig directly or pass a `BudgetFunc` closure?
   - Recommendation: Pass `*DegradationConfig` as a new parameter to `TelemetryMiddleware()`. Simple, testable, consistent with existing pattern.

2. **Should the probe flag reset on timeout?**
   - What we know: Single-probe half-open uses an atomic bool. If the probe request times out, it will go through the normal `RecordFailure` path.
   - What's unclear: Does `RecordFailure` need to explicitly reset the probe flag?
   - Recommendation: Yes -- `RecordFailure` and `RecordSuccess` both reset the probe flag. This is the cleanup path.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.10+ |
| Config file | go.test flags in Makefile |
| Quick run command | `go test ./internal/degrade/ ./internal/kernel/lspool/ ./internal/mcp/ -run TestDegrade -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DEGRADE-01 | All 38 tools mapped to a class; Budget() returns correct duration | unit | `go test ./internal/degrade/ -run TestToolClassMap -count=1` | Wave 0 |
| DEGRADE-02 | Deadline propagated: context.DeadlineExceeded returned for slow handler | unit | `go test ./internal/mcp/ -run TestDeadlinePropagation -count=1` | Wave 0 |
| DEGRADE-03 | ErrCircuitOpen carries typed fields; errors.Is still works | unit | `go test ./internal/kernel/lspool/ -run TestCircuitOpenError -count=1` | Wave 0 |
| DEGRADE-04 | Jitter backoff within [base, prevSleep*3] range | unit | `go test ./internal/kernel/lspool/ -run TestDecorrelatedJitter -count=1` | Wave 0 |
| DEGRADE-05 | After 3 crashes, circuit stays open; CanAttempt returns false | unit | `go test ./internal/kernel/lspool/ -run TestRestartBudget -count=1` | Wave 0 |
| DEGRADE-06 | SetMemoryLimit called when config > 0 | unit | `go test ./internal/daemon/ -run TestMemoryLimit -count=1` | Wave 0 |
| DEGRADE-07 | SIGTERM mid-request: drain + span flush + clean exit | integration | `go test ./internal/daemon/ -run TestGracefulShutdownMidRequest -count=1 -timeout=30s` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test ./internal/degrade/ ./internal/kernel/lspool/ ./internal/mcp/ ./internal/daemon/ -count=1 -timeout=60s`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/degrade/budget_test.go` -- covers DEGRADE-01
- [ ] `internal/kernel/lspool/circuit_test.go` -- covers DEGRADE-03, DEGRADE-04, DEGRADE-05 (file may exist but needs new test cases)
- [ ] `internal/mcp/middleware_deadline_test.go` -- covers DEGRADE-02
- [ ] `internal/daemon/shutdown_graceful_test.go` -- covers DEGRADE-07

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | Validate timeout config values are positive; clamp to sane ranges |
| V6 Cryptography | no | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Denial of service via resource exhaustion | Denial of Service | SetMemoryLimit + timeout budgets + circuit breaker |
| Client sends huge timeout override | Tampering | Server-side budgets override client; D-01 single enforcement point |
| Restart loop CPU burn | Denial of Service | Restart budget (D-05): 3 crashes then circuit stays open |

## Sources

### Primary (HIGH confidence)
- Codebase inspection: `internal/kernel/lspool/circuit.go`, `pool.go`, `metrics.go` -- existing CB and error patterns
- Codebase inspection: `internal/mcp/middleware.go` -- TelemetryMiddleware structure, classifyOutcome routing
- Codebase inspection: `internal/daemon/shutdown.go`, `daemon.go` -- shutdown ordering, signal handling
- Codebase inspection: `internal/config/config.go`, `defaults.go` -- config struct patterns

### Secondary (MEDIUM confidence)
- AWS Architecture Blog: decorrelated jitter formula [CITED: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/]
- Go stdlib docs: `runtime/debug.SetMemoryLimit`, `context.WithTimeout`, `math/rand/v2` [CITED: Go 1.25 stdlib]

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all stdlib, no new external deps
- Architecture: HIGH - enhancement of existing code with clear insertion points verified in source
- Pitfalls: HIGH - derived from codebase inspection and established Go patterns

**Research date:** 2026-04-10
**Valid until:** 2026-05-10 (stable -- stdlib patterns, no external deps)
