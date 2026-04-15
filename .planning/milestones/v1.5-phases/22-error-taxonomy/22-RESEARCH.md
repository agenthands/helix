# Phase 22: Error Taxonomy - Research

**Researched:** 2026-04-14
**Domain:** Go error types, structured error reporting, MCP tool error handling
**Confidence:** HIGH

## Summary

Phase 22 creates a new `internal/errors/` package that defines a typed error taxonomy for all 38+ MCP tools. The package exports 7 error kinds as `type Kind string` constants, a single `Error` struct with builder-style constructors, and `errors.Is`/`errors.As` compatibility. The existing `CircuitOpenError` in `lspool/` and the sentinel errors in `internal/mcp/errors.go` are the primary migration targets.

The codebase currently uses two error patterns: (1) sentinel `errors.New` values checked with `errors.Is` (5 in `internal/mcp/errors.go`, 3 in `internal/kernel/lspool/`), and (2) `fmt.Errorf` with `%w` wrapping (~125 call sites in `internal/kernel/` alone). Tool handlers return plain string errors via `errorResult(msg)` helper functions that set `IsError: true` on `CallToolResult`. The existing `ErrorDetail` struct in `internal/mcp/errors.go` provides partial structured error info but is only used in `activate_project`. Integration tests (`test/integration/errors_test.go`) explicitly await typed errors via `TODO(#typed-errors)` comments.

**Primary recommendation:** Create `internal/errors/` with a flat Kind enum, single Error struct implementing `error`/`Unwrap`/`Is`/`MarshalJSON`, and builder-style constructors. Keep it zero-dependency (stdlib only) so every internal package can import it without cycles.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Flat error kinds only -- 7 top-level Kind constants (`NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`) as `type Kind string`. No sub-kinds or hierarchical classification.
- **D-02:** New dedicated package at `internal/errors/`. Import alias convention: `serr`. Existing sentinels in `internal/mcp/errors.go` migrate here.
- **D-03:** Detail field is plain `string`. Agents match on Kind; Detail is for display/logging.
- **D-04:** Single `Error` struct with fields: `Kind Kind`, `Message string`, `Tool string`, `Detail string`, `cause error` (unexported). Builder-style constructors: `serr.New(kind, message)` with `.WithTool()`, `.WithDetail()` chainable. Implements `error`, `errors.Unwrap()`, and `errors.Is()` (Kind-based matching).
- **D-05:** Flatten `CircuitOpenError` metadata into Detail string. Replace `CircuitOpenError` with `serr.New(serr.CircuitOpen, ...)`. Preserve `errors.Is(err, ErrCircuitOpen)` backward compat via Kind-based `Is()`.

### Claude's Discretion
- Constructor ergonomics (exact method signatures, whether to use functional options or builder pattern)
- File organization within `internal/errors/` (single file vs. split)
- Whether to provide convenience constructors per Kind (e.g., `serr.NotFoundErr(...)`)
- JSON serialization approach for MCP responses (MarshalJSON on Error struct)

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ERR-01 | User receives a typed error kind (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout) from every tool failure | Kind type + 7 constants in `internal/errors/kinds.go`; Error struct carries Kind field |
| ERR-02 | User receives structured error fields (Kind, Message, Tool, Detail) that agents can parse programmatically | Error struct with exported fields + MarshalJSON; JSON output tested |
| ERR-03 | User receives wrapped errors that preserve the underlying cause chain | unexported `cause error` field, `Unwrap()` method, `Wrap()` constructor; `errors.Is`/`errors.As` chain traversal verified in tests |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task**
- Build command: `go build ./cmd/serena`
- Test command: `go test ./...`
- Format command: `gofmt -w .`
- Single binary, Go-only, no CGO for core packages
- Same repo, Python in `legacy/` (not relevant here)

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `errors` (stdlib) | Go 1.25.1 | `errors.Is`, `errors.As`, `errors.New`, `errors.Unwrap` | Built-in error chain support since Go 1.13 [VERIFIED: go version output] |
| `encoding/json` (stdlib) | Go 1.25.1 | `MarshalJSON` for structured error serialization | Standard JSON encoding [VERIFIED: already used in codebase] |
| `fmt` (stdlib) | Go 1.25.1 | `fmt.Sprintf` for Detail string formatting | Standard formatting [VERIFIED: already used in codebase] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` (stdlib) | Go 1.25.1 | Unit tests for Error type | All test files |
| `github.com/stretchr/testify` | (in go.mod) | Assertions in tests | Already used project-wide [VERIFIED: test/integration/errors_test.go] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Builder pattern | Functional options (`WithTool(t) Option`) | Functional options add allocation per option; builder is simpler for 2-3 chainable fields. D-04 locks builder. |
| `type Kind string` | `type Kind int` with iota | String kinds are self-documenting in JSON and logs; no stringer needed. D-01 locks string. |

**Installation:** No new dependencies required -- stdlib only.

## Architecture Patterns

### Recommended Project Structure
```
internal/errors/
    errors.go     # Error struct, New(), Wrap(), builder methods, Unwrap(), Is(), MarshalJSON()
    kinds.go      # Kind type + 7 constants
    errors_test.go # Unit tests
```

Two files plus test keeps it navigable without over-splitting. `kinds.go` is separate so the Kind constants are easy to find and extend.

### Pattern 1: Kind-Based Error Type with Builder
**What:** Single Error struct with `type Kind string` for programmatic matching, builder methods for optional fields.
**When to use:** Every tool error return site.
**Example:**
```go
// Source: D-04 decision + Go conventions [VERIFIED: codebase patterns]
package errors

import (
    "encoding/json"
    "fmt"
)

type Kind string

const (
    NotFound    Kind = "not_found"
    InvalidArgs Kind = "invalid_args"
    NoWorkspace Kind = "no_workspace"
    Unsupported Kind = "unsupported"
    Internal    Kind = "internal"
    CircuitOpen Kind = "circuit_open"
    Timeout     Kind = "timeout"
)

type Error struct {
    Kind    Kind   `json:"kind"`
    Message string `json:"message"`
    Tool    string `json:"tool,omitempty"`
    Detail  string `json:"detail,omitempty"`
    cause   error  // unexported, preserved via Unwrap
}

func New(kind Kind, message string) *Error {
    return &Error{Kind: kind, Message: message}
}

func Wrap(kind Kind, message string, cause error) *Error {
    return &Error{Kind: kind, Message: message, cause: cause}
}

func (e *Error) WithTool(tool string) *Error {
    e.Tool = tool
    return e
}

func (e *Error) WithDetail(detail string) *Error {
    e.Detail = detail
    return e
}

func (e *Error) Error() string {
    if e.Detail != "" {
        return fmt.Sprintf("%s: %s (%s)", e.Kind, e.Message, e.Detail)
    }
    return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error {
    return e.cause
}

// Is enables errors.Is matching by Kind. When target is an *Error,
// matches if Kinds are equal. This makes errors.Is(err, &Error{Kind: NotFound})
// work, as well as matching sentinel-style.
func (e *Error) Is(target error) bool {
    if t, ok := target.(*Error); ok {
        return e.Kind == t.Kind
    }
    return false
}

func (e *Error) MarshalJSON() ([]byte, error) {
    type alias Error // break recursion
    return json.Marshal((*alias)(e))
}
```

### Pattern 2: Kind Sentinel Variables for errors.Is
**What:** Pre-built sentinel Error values per Kind, enabling `errors.Is(err, serr.ErrNotFound)`.
**When to use:** Callers checking error kind without extracting the full Error.
**Example:**
```go
// Source: CircuitOpenError.Is() pattern in lspool/circuit_err.go [VERIFIED: codebase]
var (
    ErrNotFound    = &Error{Kind: NotFound}
    ErrInvalidArgs = &Error{Kind: InvalidArgs}
    ErrNoWorkspace = &Error{Kind: NoWorkspace}
    ErrUnsupported = &Error{Kind: Unsupported}
    ErrInternal    = &Error{Kind: Internal}
    ErrCircuitOpen = &Error{Kind: CircuitOpen}
    ErrTimeout     = &Error{Kind: Timeout}
)

// Usage:
// if errors.Is(err, serr.ErrNotFound) { ... }
// This works because Error.Is() compares Kind values.
```

### Pattern 3: CircuitOpen Migration (D-05)
**What:** Replace `lspool.CircuitOpenError` with `serr.Error{Kind: CircuitOpen}`, formatting metadata into Detail.
**When to use:** `internal/kernel/lspool/pool.go` circuit breaker error path.
**Example:**
```go
// Before (lspool/circuit_err.go):
// return &CircuitOpenError{Language: lang, BackoffRemaining: dur, Failures: n, RetryAfter: t}

// After:
// return serr.New(serr.CircuitOpen, "circuit breaker open").
//     WithDetail(fmt.Sprintf("language=%s failures=%d retry_after=%s",
//         lang, n, t.Format(time.RFC3339)))

// Backward compat: errors.Is(err, lspool.ErrCircuitOpen) must keep working.
// The lspool.ErrCircuitOpen sentinel is replaced by serr.ErrCircuitOpen.
// Callers using errors.Is(err, lspool.ErrCircuitOpen) need updating to
// errors.Is(err, serr.ErrCircuitOpen) -- but that's Phase 23 scope.
// During transition, lspool can re-export: var ErrCircuitOpen = serr.ErrCircuitOpen
```

### Anti-Patterns to Avoid
- **Hierarchical error kinds:** Don't create sub-kinds like `NotFound.File` or `NotFound.Symbol`. The Detail string carries specifics; Kind is for coarse programmatic matching. [D-01]
- **`any` or `map[string]any` in Detail:** Don't make Detail structured -- it's a plain string for display. [D-03]
- **Importing `internal/errors` as `errors`:** Always use `serr` alias to avoid shadowing stdlib `errors`. 10 files in `internal/` import stdlib `errors`. [VERIFIED: grep count]
- **Returning `*Error` from functions:** Return `error` interface from functions; use `*Error` only in constructors. This ensures `nil` pointer semantics work correctly with interfaces.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Error wrapping chain | Manual cause tracking | `errors.Unwrap` protocol (implement `Unwrap() error`) | Stdlib chain traversal, `errors.Is`/`errors.As` work automatically [VERIFIED: Go stdlib] |
| JSON marshaling | Custom string formatting for MCP | `encoding/json` with struct tags + `MarshalJSON` | Consistent, tested serialization [VERIFIED: existing ErrorDetail pattern] |
| Kind-based matching | Switch on `err.Error()` string content | `errors.Is` with sentinel Error values | Type-safe, refactor-proof [VERIFIED: CircuitOpenError.Is() pattern in codebase] |

**Key insight:** Go's `errors.Is`/`errors.As` with custom `Is()` methods is the entire matching infrastructure -- no need for a matching library or custom traversal.

## Common Pitfalls

### Pitfall 1: nil *Error satisfies error interface as non-nil
**What goes wrong:** A function returning `error` that assigns a `*Error` nil pointer returns a non-nil interface with a nil concrete value. `err != nil` is true even though there's no error.
**Why it happens:** Go interfaces store (type, value) pairs; a typed nil pointer has type info.
**How to avoid:** Never return `*Error` from functions with `error` return type. Use `error` as the return type, return `nil` directly (not a `*Error` nil variable).
**Warning signs:** Tests pass but callers see unexpected non-nil errors.

### Pitfall 2: Broken errors.Is chain after wrapping
**What goes wrong:** `fmt.Errorf("context: %w", serrError)` creates a new wrapper. If callers use `errors.Is(err, serr.ErrNotFound)`, it must traverse the chain. The custom `Is()` must only be on the innermost `*Error`, not on the wrapper.
**Why it happens:** `errors.Is` walks the chain calling `Unwrap()`, then calls `Is()` on each unwrapped error.
**How to avoid:** Ensure `Error.Is()` compares Kind correctly. Test with `fmt.Errorf("outer: %w", serrError)` to verify chain traversal.
**Warning signs:** `errors.Is` returns false on wrapped typed errors.

### Pitfall 3: ErrCircuitOpen backward compatibility
**What goes wrong:** Code using `errors.Is(err, lspool.ErrCircuitOpen)` breaks if the sentinel is removed without a migration path.
**Why it happens:** Direct sentinel comparison fails when the sentinel is a different object.
**How to avoid:** During migration, have `lspool.ErrCircuitOpen` re-export `serr.ErrCircuitOpen`. The `middleware.go` and test files that check `lspool.ErrCircuitOpen` (3 call sites found) need updating in Phase 23.
**Warning signs:** Tests in `circuit_test.go` and `telemetry_middleware_test.go` fail.

### Pitfall 4: Package name shadows stdlib
**What goes wrong:** `internal/errors` package named `errors` shadows the stdlib `errors` package in any file that imports both.
**Why it happens:** Go resolves unqualified `errors` to the last imported package with that name.
**How to avoid:** Convention is already decided: import as `serr`. The package declaration should be `package errors` (standard Go convention for the directory name), but all import sites use `import serr "github.com/postfix/serena/internal/errors"`. Currently 10 files in `internal/` import stdlib `errors` -- none will need both once they switch to `serr` for typed errors (since `serr.Error` has its own `Unwrap`). If a file truly needs both, use `stderrors "errors"`.
**Warning signs:** Compilation errors about ambiguous `errors` package.

## Code Examples

### Creating a typed error
```go
// Source: D-04 decision [VERIFIED: codebase builder pattern]
err := serr.New(serr.NotFound, "symbol not found").
    WithTool("find_references").
    WithDetail("symbol 'Foo' not found in main.go")
```

### Wrapping a lower-level error
```go
// Source: ERR-03 requirement [VERIFIED: Go stdlib errors.Unwrap]
lspErr := someLSPCall()
err := serr.Wrap(serr.Internal, "LSP request failed", lspErr).
    WithTool("goto_definition")
// errors.Unwrap(err) == lspErr
// errors.Is(err, lspErr) == true
```

### Checking error kind
```go
// Source: CircuitOpenError.Is() pattern [VERIFIED: lspool/circuit_err.go]
if errors.Is(err, serr.ErrNotFound) {
    // handle not found
}

var serErr *serr.Error
if errors.As(err, &serErr) {
    log.Printf("tool=%s kind=%s detail=%s", serErr.Tool, serErr.Kind, serErr.Detail)
}
```

### JSON output for MCP
```go
// Source: D-02 MCP structured error requirement [VERIFIED: existing ErrorDetail.MarshalJSON]
data, _ := json.Marshal(err)
// {"kind":"not_found","message":"symbol not found","tool":"find_references","detail":"symbol 'Foo' not found in main.go"}
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (already in go.mod) |
| Config file | None needed -- Go convention |
| Quick run command | `go test ./internal/errors/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ERR-01 | 7 Kind constants exist and Error carries Kind | unit | `go test ./internal/errors/ -run TestKinds` | Wave 0 |
| ERR-02 | Error serializes to JSON with all 4 fields | unit | `go test ./internal/errors/ -run TestMarshalJSON` | Wave 0 |
| ERR-03 | Wrap preserves cause chain via errors.Is/As | unit | `go test ./internal/errors/ -run TestUnwrap` | Wave 0 |
| ERR-03 | errors.Is works through fmt.Errorf %w wrapping | unit | `go test ./internal/errors/ -run TestIsChain` | Wave 0 |
| -- | CircuitOpen sentinel backward compat | unit | `go test ./internal/errors/ -run TestCircuitOpenCompat` | Wave 0 |
| -- | Nil pointer interface pitfall doesn't bite | unit | `go test ./internal/errors/ -run TestNilError` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/errors/...`
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** Full suite green before verification

### Wave 0 Gaps
- [ ] `internal/errors/errors_test.go` -- covers ERR-01, ERR-02, ERR-03 and pitfall cases
- No framework install needed -- Go testing is built in

## Security Domain

This phase creates internal error types with no external input parsing, no authentication, no network exposure, and no cryptographic operations. The errors are consumed by the MCP layer which already handles response serialization.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | no | -- |
| V5 Input Validation | no | Kinds are a closed set; no user input parsed in this package |
| V6 Cryptography | no | -- |

### Known Threat Patterns
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Information leakage via error Detail | Information Disclosure | Detail is human-readable context, not stack traces. Cause chain is unexported. MCP layer controls what's sent to clients. |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `package errors` (matching directory name) won't cause issues since all imports use `serr` alias | Architecture | Compilation errors if any file imports without alias; low risk since D-02 mandates `serr` convention |

All other claims are verified from codebase grep or Go stdlib documentation.

## Open Questions

1. **Re-export strategy for lspool.ErrCircuitOpen**
   - What we know: 3 call sites use `lspool.ErrCircuitOpen` (middleware.go, 2 test files). D-05 says replace CircuitOpenError.
   - What's unclear: Whether to re-export in lspool during Phase 22 or defer re-export to Phase 23.
   - Recommendation: Create `var ErrCircuitOpen = serr.ErrCircuitOpen` in lspool during this phase to maintain backward compat immediately. Callers migrate in Phase 23.

2. **Sentinel migration for mcp/errors.go**
   - What we know: 5 sentinels exist (ErrSessionExpired, ErrWorkspaceNotReady, etc.). D-02 says they migrate to internal/errors.
   - What's unclear: Which sentinels map to which Kind (e.g., ErrWorkspaceNotReady -> NoWorkspace, ErrProjectNotFound -> NotFound).
   - Recommendation: Map in this phase: `ErrSessionExpired` -> `Timeout` or `Internal`, `ErrWorkspaceNotReady` -> `NoWorkspace`, `ErrToolNotAvailable` -> `Unsupported`, `ErrProjectNotFound` -> `NotFound`, `ErrLSCrashed` -> `Internal`. Re-export from `internal/mcp/errors.go` for backward compat.

## Sources

### Primary (HIGH confidence)
- Codebase grep: `internal/mcp/errors.go` -- 5 sentinel errors, ErrorDetail struct
- Codebase grep: `internal/kernel/lspool/circuit_err.go` -- CircuitOpenError with Is() method
- Codebase grep: `internal/kernel/lspool/pool.go:40` -- ErrCircuitOpen sentinel
- Codebase grep: `test/integration/errors_test.go` -- TODO(#typed-errors) markers
- Codebase grep: 125 `fmt.Errorf` sites in `internal/kernel/`
- Codebase grep: 10 files importing stdlib `"errors"` in `internal/`
- Go stdlib docs: `errors.Is`, `errors.As`, `errors.Unwrap` semantics [CITED: https://pkg.go.dev/errors]

### Secondary (MEDIUM confidence)
- Go 1.25.1 runtime verified via `go version` output

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- stdlib only, no external deps to verify
- Architecture: HIGH -- decisions are locked, patterns verified in codebase
- Pitfalls: HIGH -- nil interface, Is() chain, and shadowing are well-documented Go gotchas verified against codebase usage

**Research date:** 2026-04-14
**Valid until:** 2026-05-14 (stable -- stdlib error patterns don't change)
