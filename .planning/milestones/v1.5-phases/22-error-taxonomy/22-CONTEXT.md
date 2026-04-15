# Phase 22: Error Taxonomy - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Define a typed error taxonomy package that all 38+ MCP tools will use for structured error reporting. This phase creates the types, constructors, and wrapping infrastructure. Phase 23 migrates tools to use it; Phase 24 adds validation and test coverage.

</domain>

<decisions>
## Implementation Decisions

### Error Granularity
- **D-01:** Flat error kinds only — 7 top-level Kind constants (`NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`) as `type Kind string`. No sub-kinds or hierarchical classification. The Detail and Message fields carry specifics; Kind is for programmatic matching by agents.

### Package Placement
- **D-02:** New dedicated package at `internal/errors/`. Clean import path, no coupling to MCP layer, usable by kernel, skills, and MCP alike. Convention: import alias `serr` (e.g., `import serr "github.com/postfix/serena/internal/errors"`). The existing sentinels in `internal/mcp/errors.go` migrate here.

### Detail Richness
- **D-03:** Detail field is a plain `string` with human-readable context. Agents match programmatically on Kind; Detail is for display and logging. No structured map or `any` type — keeps the API simple and predictable.

### Error Struct Design
- **D-04:** Single `Error` struct with fields: `Kind Kind`, `Message string`, `Tool string`, `Detail string`, `cause error` (unexported). Builder-style constructors: `serr.New(kind, message)` with `.WithTool()`, `.WithDetail()` chainable methods. Implements `error`, `errors.Unwrap()`, and `errors.Is()` (Kind-based matching).

### CircuitOpen Migration
- **D-05:** Flatten `CircuitOpenError` metadata (Language, BackoffRemaining, Failures, RetryAfter) into the Detail string. `CircuitOpenError` type in `lspool/` is replaced by a standard `serr.New(serr.CircuitOpen, ...)` call. `errors.Is(err, ErrCircuitOpen)` backward compatibility preserved via the new Error type's Kind-based `Is()` method.

### Claude's Discretion
- Constructor ergonomics (exact method signatures, whether to use functional options or builder pattern)
- File organization within `internal/errors/` (single file vs. split across errors.go, kinds.go, wrap.go)
- Whether to provide convenience constructors per Kind (e.g., `serr.NotFoundErr(...)`)
- JSON serialization approach for MCP responses (MarshalJSON on Error struct)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Error Infrastructure
- `internal/mcp/errors.go` — Current domain sentinels (ErrSessionExpired, ErrWorkspaceNotReady, etc.) and ErrorDetail struct to be superseded
- `internal/kernel/lspool/circuit_err.go` — CircuitOpenError typed error to be replaced/migrated

### Requirements
- `.planning/REQUIREMENTS.md` — ERR-01, ERR-02, ERR-03 define the acceptance criteria for this phase

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/mcp/errors.go` — ErrorDetail struct and 5 sentinel errors (partial foundation, will be superseded)
- `internal/kernel/lspool/circuit_err.go` — CircuitOpenError with `Is()` method (pattern to follow for Kind-based Is())

### Established Patterns
- `errors.Is`/`errors.As` used across codebase for error matching
- `fmt.Errorf` with `%w` wrapping is the current wrapping pattern (~100+ call sites across tool packages)
- Builder pattern used elsewhere in codebase (e.g., tool definitions)

### Integration Points
- All tool packages (`kernel/symbols/`, `kernel/edit/`, `kernel/fileops/`, `kernel/diag/`, `skill/memory/`, `skill/workflow/`) will import `internal/errors` in Phase 23
- `internal/mcp/` middleware will convert typed errors to MCP error responses
- `internal/kernel/lspool/pool.go` uses `ErrCircuitOpen` sentinel — backward compat needed during migration

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. User consistently chose the recommended (simplest) option for all decisions, indicating a preference for straightforward, conventional Go error patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 22-error-taxonomy*
*Context gathered: 2026-04-14*
