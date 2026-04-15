# Phase 22: Error Taxonomy - Pattern Map

**Mapped:** 2026-04-14
**Phase:** 22-error-taxonomy

## Files to Create/Modify

### 1. `internal/errors/kinds.go` (CREATE)

**Role:** Constants file -- defines the `Kind` type and 7 sentinel constants.
**Data flow:** Imported by `errors.go` (same package), consumed by every tool package via `serr` alias.
**Closest analog:** `internal/kernel/lspool/pool.go:39-40` (sentinel constant pattern)

```go
// pool.go:39-40 -- sentinel error constant
var ErrCircuitOpen = errors.New("circuit breaker is open; retry after backoff")
```

Also parallels the sentinel var block in `internal/mcp/errors.go:9-15`:

```go
var (
    ErrSessionExpired    = errors.New("session expired")
    ErrWorkspaceNotReady = errors.New("workspace not ready")
    ErrToolNotAvailable  = errors.New("tool not available in current mode")
    ErrProjectNotFound   = errors.New("project not found at specified path")
    ErrLSCrashed         = errors.New("language server crashed")
)
```

**Structural notes:** Package declaration `package errors`. Exports only the `Kind` type and 7 `const` values plus 7 `var` sentinel `*Error` values (`ErrNotFound`, etc.).

---

### 2. `internal/errors/errors.go` (CREATE)

**Role:** Core type file -- `Error` struct, constructors (`New`, `Wrap`), builder methods (`WithTool`, `WithDetail`), interface implementations (`Error()`, `Unwrap()`, `Is()`, `MarshalJSON()`).
**Data flow:** Imported as `serr` by all tool packages, kernel, mcp middleware. Produces `*Error` values that flow upward through tool handlers to MCP layer.
**Closest analog:** `internal/kernel/lspool/circuit_err.go` (typed error struct with `Is()` method)

```go
// circuit_err.go -- typed error struct with Is()
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

func (e *CircuitOpenError) Is(target error) bool {
    return target == ErrCircuitOpen
}
```

**Second analog:** `internal/mcp/errors.go:17-27` (struct with `MarshalJSON` breaking recursion with type alias)

```go
type ErrorDetail struct {
    Code       string `json:"code"`
    Cause      string `json:"cause"`
    Suggestion string `json:"suggestion,omitempty"`
}

func (e ErrorDetail) MarshalJSON() ([]byte, error) {
    type Alias ErrorDetail
    return json.Marshal((*Alias)(&e))
}
```

**Key difference from analogs:** The new `Error.Is()` compares `Kind` field (not sentinel pointer equality). This makes `errors.Is(err, serr.ErrNotFound)` work because `ErrNotFound = &Error{Kind: NotFound}` and `Is()` checks `e.Kind == t.Kind`.

---

### 3. `internal/errors/errors_test.go` (CREATE)

**Role:** Unit tests for Error struct, Kind constants, Is/Unwrap chain, MarshalJSON.
**Closest analog:** `internal/kernel/lspool/circuit_test.go` (tests for typed error with `errors.Is` chain verification)

```go
// circuit_test.go -- Is() test pattern
func TestCircuitOpenError_Is(t *testing.T) {
    e := &CircuitOpenError{Language: "go", Failures: 1}
    if !errors.Is(e, ErrCircuitOpen) {
        t.Error("errors.Is(&CircuitOpenError{}, ErrCircuitOpen) should return true")
    }
}

func TestCircuitOpenError_Unwrap(t *testing.T) {
    inner := &CircuitOpenError{Language: "rust", Failures: 2}
    wrapped := fmt.Errorf("%w: extra context", inner)
    if !errors.Is(wrapped, ErrCircuitOpen) {
        t.Error("errors.Is on wrapped CircuitOpenError should still find ErrCircuitOpen")
    }
}
```

**Second analog:** `internal/mcp/server_test.go:27-48` (JSON marshaling tests for ErrorDetail)

```go
func TestErrorDetail_JSON(t *testing.T) {
    detail := ErrorDetail{
        Code:       "WORKSPACE_NOT_READY",
        Cause:      "workspace is still initializing",
        Suggestion: "wait and retry in a few seconds",
    }
    data, err := detail.MarshalJSON()
    assert.NoError(t, err)
    assert.Contains(t, string(data), `"code":"WORKSPACE_NOT_READY"`)
    assert.Contains(t, string(data), `"suggestion"`)
}
```

---

### 4. `internal/mcp/errors.go` (MODIFY)

**Role:** Migrate sentinels to re-exports from `internal/errors`. Keep `ErrorDetail` temporarily (Phase 23 removes it).
**Data flow:** Downstream consumers (`internal/mcp/server.go`, `internal/mcp/server_test.go`) continue importing from `internal/mcp` during transition. Sentinels now delegate to `serr.ErrXxx`.
**Closest analog for re-export pattern:** The recommended lspool re-export pattern from RESEARCH.md.

**Current code (to be modified):**

```go
package mcp

import (
    "encoding/json"
    "errors"
)

var (
    ErrSessionExpired    = errors.New("session expired")
    ErrWorkspaceNotReady = errors.New("workspace not ready")
    ErrToolNotAvailable  = errors.New("tool not available in current mode")
    ErrProjectNotFound   = errors.New("project not found at specified path")
    ErrLSCrashed         = errors.New("language server crashed")
)
```

**Migration target:** Re-export sentinels from `serr`:

```go
import serr "github.com/postfix/serena/internal/errors"

var (
    ErrSessionExpired    = serr.New(serr.Timeout, "session expired")
    ErrWorkspaceNotReady = serr.ErrNoWorkspace
    ErrToolNotAvailable  = serr.ErrUnsupported
    ErrProjectNotFound   = serr.ErrNotFound
    ErrLSCrashed         = serr.ErrInternal
)
```

---

### 5. `internal/kernel/lspool/circuit_err.go` (MODIFY)

**Role:** Replace `CircuitOpenError` struct with `serr.Error{Kind: CircuitOpen}`. Re-export `ErrCircuitOpen` from `serr` for backward compat.
**Data flow:** `pool.go` creates circuit errors -> `middleware.go` checks via `errors.Is` -> telemetry classification.

**Current code (to be replaced):**

```go
package lspool

import (
    "fmt"
    "time"
)

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

func (e *CircuitOpenError) Is(target error) bool {
    return target == ErrCircuitOpen
}
```

**Migration target:**

```go
package lspool

import serr "github.com/postfix/serena/internal/errors"

// ErrCircuitOpen re-exports the canonical sentinel for backward compat.
// Callers should migrate to serr.ErrCircuitOpen in Phase 23.
var ErrCircuitOpen = serr.ErrCircuitOpen
```

**Callers using `ErrCircuitOpen` (3 sites, updated in Phase 23):**
- `internal/mcp/middleware.go:88` -- `errors.Is(err, lspool.ErrCircuitOpen)`
- `internal/mcp/telemetry_middleware_test.go:90,192` -- test stubs returning `lspool.ErrCircuitOpen`
- `internal/kernel/lspool/circuit_test.go:29-38` -- unit tests

---

### 6. `internal/kernel/lspool/pool.go` (MODIFY -- line 39-40 only)

**Role:** Remove the old `ErrCircuitOpen` sentinel (now re-exported from `serr` via `circuit_err.go`).

**Current code:**

```go
// pool.go:39-40
var ErrCircuitOpen = errors.New("circuit breaker is open; retry after backoff")
```

**Migration:** Delete these 2 lines. The sentinel is now defined in `circuit_err.go` as a re-export of `serr.ErrCircuitOpen`.

---

## Data Flow

```
Tool handler (symbols/, edit/, fileops/, diag/)
    |
    | returns error (currently plain string via errorResult())
    | Phase 22: creates serr.New(kind, msg).WithTool(name)
    v
Kernel layer (kernel/)
    |
    | propagates error interface upward
    v
MCP middleware (mcp/middleware.go)
    |
    | classifyOutcome() checks errors.Is(err, serr.ErrCircuitOpen), etc.
    | Phase 23: will use errors.As(err, &serr.Error{}) for richer classification
    v
MCP response (mcpsdk.CallToolResult)
    |
    | json.Marshal(serrError) -> structured JSON in Content
    v
Agent client (Claude Code, Codex, etc.)
```

---

## Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Package name | Matches directory | `package errors` |
| Import alias | `serr` (mandated by D-02) | `import serr "github.com/postfix/serena/internal/errors"` |
| Kind constants | PascalCase noun/adjective | `NotFound`, `InvalidArgs`, `CircuitOpen` |
| Kind values (string) | snake_case | `"not_found"`, `"invalid_args"`, `"circuit_open"` |
| Sentinel vars | `Err` + PascalCase Kind | `ErrNotFound`, `ErrCircuitOpen` |
| Constructors | `New(kind, msg)`, `Wrap(kind, msg, cause)` | Follows `errors.New()` convention |
| Builder methods | `With` + field name | `WithTool()`, `WithDetail()` |
| Test functions | `Test` + struct + `_` + behavior | `TestError_Is`, `TestError_MarshalJSON` |

---

## Import Patterns

**Existing pattern in tool packages (unchanged):**

```go
import (
    "context"
    "fmt"

    mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
    "go.opentelemetry.io/otel/trace"

    "github.com/postfix/serena/internal/kernel"
    "github.com/postfix/serena/internal/mcp"
    "github.com/postfix/serena/internal/workspace"
)
```

**New import for Phase 23 (tool packages):**

```go
import (
    serr "github.com/postfix/serena/internal/errors"
)
```

**Import grouping:** stdlib, external, internal -- three-group convention already used across codebase.

---

## Structural Patterns

### Error constructor returns pointer, functions return interface

```go
// Constructor returns *Error (concrete)
func New(kind Kind, message string) *Error { ... }

// But tool handlers return error (interface)
func someToolHandler() error {
    return serr.New(serr.NotFound, "symbol not found")
}
```

### Builder chain pattern (from tool registration)

```go
// Existing builder pattern in tool registration (tools.go files)
mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
    Name:        "get_diagnostics",
    Description: "Returns current diagnostics...",
}, ...)

// Error builder follows same style
serr.New(serr.NotFound, "symbol not found").
    WithTool("find_references").
    WithDetail("symbol 'Foo' not found in main.go")
```

### MarshalJSON anti-recursion (from ErrorDetail)

```go
// Existing pattern in internal/mcp/errors.go:25-27
func (e ErrorDetail) MarshalJSON() ([]byte, error) {
    type Alias ErrorDetail
    return json.Marshal((*Alias)(&e))
}

// New Error follows identical pattern
func (e *Error) MarshalJSON() ([]byte, error) {
    type alias Error
    return json.Marshal((*alias)(e))
}
```

### errors.Is with custom Is() method (from CircuitOpenError)

```go
// Existing: compares against sentinel pointer
func (e *CircuitOpenError) Is(target error) bool {
    return target == ErrCircuitOpen
}

// New: compares Kind field (richer matching)
func (e *Error) Is(target error) bool {
    if t, ok := target.(*Error); ok {
        return e.Kind == t.Kind
    }
    return false
}
```

---

*Phase: 22-error-taxonomy*
*Patterns mapped: 2026-04-14*
