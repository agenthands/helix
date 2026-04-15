# Phase 24: Validation & Testing - Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Tools validate inputs before execution (returning InvalidArgs typed errors for missing/invalid parameters), the three-band error test suite asserts on error Kind instead of just IsError=true, and golden files capture the canonical error response shape per error Kind for regression detection.

</domain>

<decisions>
## Implementation Decisions

### Validation Scope
- **D-01:** Per-tool inline validation. Each tool handler validates its own required args at the top of its exec function, following the existing `ValidatePath` pattern in fileops. No new abstraction, middleware, or shared validation helper — just consistent if-checks before work begins.
- **D-02:** Audit all 38+ tools for missing required-field checks. Tools that already validate (e.g., fileops with `ValidatePath`, memory tools) get verified and skipped; tools with no validation get inline checks added. Every tool must have validation coverage.

### Validation Strictness
- **D-03:** Zero-value numerics are valid. `line=0` and `column=0` are legitimate positions (LSP uses 0-based indexing). Only reject truly missing fields (key absent from args map) or wrong types.
- **D-04:** Empty strings for required string fields are rejected as InvalidArgs. An empty string for `path`, `symbol_name`, `query`, etc. is treated as missing — agents should never pass empty strings for required fields.
- **D-05:** Optional fields are validated if present. If an optional field is provided with the wrong type, return InvalidArgs. Absent optional fields are silently ignored with defaults.

### Test Upgrade Strategy
- **D-06:** Parse structured content from MCP error responses to extract the Kind field. Add an `expectedKind` field to the `errCase` struct and assert on it alongside `IsError=true`. The JSON error body carries Kind, Message, Tool, Detail — parse and assert on Kind.
- **D-07:** Expand test coverage beyond the current ~30 cases. Every tool must have at least one InvalidArgs test case. Fill known gaps: verify_edit error paths, symbol_not_found with live LS where feasible, and InvalidArgs cases for each tool's required fields.

### Error Golden Files
- **D-08:** Per-kind template golden files. One golden file per error Kind (invalid_args.golden, not_found.golden, no_workspace.golden, etc.) capturing the canonical JSON shape. All tools returning that Kind must match the template structure. Fewer files, catches structural regressions across releases.
- **D-09:** Golden files live alongside success goldens in `test/oracle/contract/testdata/golden/errors/`. Extends the existing golden_test.go infrastructure.

### Claude's Discretion
- Exact validation code per tool (which fields are required, type assertions)
- Whether to consolidate repeated validation patterns within a tool group
- Which tools need expanded test cases beyond InvalidArgs
- Whether golden file matching uses exact JSON or structural wildcards for variable fields (message, detail)
- Plan decomposition and wave ordering

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Error Taxonomy (Phase 22 output)
- `internal/errors/kinds.go` -- 7 Kind constants and sentinel errors
- `internal/errors/errors.go` -- Error struct, New(), WithTool(), WithDetail() builders

### Existing Validation Pattern
- `internal/kernel/fileops/write.go` -- ValidatePath usage pattern (reference for inline validation)
- `internal/kernel/fileops/fileops_test.go` -- ValidatePath unit tests (reference for validation tests)

### Three-Band Error Tests
- `test/integration/errors_test.go` -- Current three-band tests with errCase struct, runErrCases harness, TODO(#typed-errors) markers

### Existing Golden Infrastructure
- `test/oracle/contract/golden_test.go` -- Golden file test infrastructure
- `test/oracle/contract/testdata/golden/` -- Success golden files (extend with errors/ subdirectory)

### Tool Packages (validation audit targets)
- `internal/kernel/symbols/` -- 9 symbol retrieval tools
- `internal/kernel/edit/` -- 6 symbol editing tools
- `internal/kernel/fileops/` -- 6 file operation tools
- `internal/kernel/diag/` -- 3 diagnostic tools
- `internal/skill/memory/` -- 7 memory tools
- `internal/skill/workflow/` -- 2 workflow tools
- `internal/profile/` -- 2 profile tools

### Requirements
- `.planning/REQUIREMENTS.md` -- VAL-01, VAL-02, VAL-03

### Phase 22-23 Context
- `.planning/phases/22-error-taxonomy/22-CONTEXT.md` -- Error taxonomy design decisions (D-01 through D-05)
- `.planning/phases/23-tool-migration/23-CONTEXT.md` -- Tool migration decisions (D-01 through D-05)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/kernel/fileops/ValidatePath()` -- Path validation with traversal protection, returns `serr.InvalidArgs` on failure
- `test/integration/errors_test.go` -- Three-band harness (`runErrCases`, `errCase` struct, `callToolExpectError`) ready for Kind assertion upgrade
- `test/oracle/contract/golden_test.go` -- Golden file infrastructure for structural comparison
- `internal/errors/` package -- Full typed error infrastructure from Phase 22

### Established Patterns
- Builder pattern: `serr.New(serr.InvalidArgs, "missing required field: path").WithTool("read_file")`
- `errors.Is(err, serr.ErrInvalidArgs)` for sentinel matching
- Golden files use `.golden` extension with JSON content in `test/oracle/contract/testdata/golden/`
- Integration tests use `//go:build integration` tag

### Integration Points
- All tool exec functions in kernel/ and skill/ packages need validation at top
- `test/integration/errors_test.go` errCase struct needs `expectedKind` field
- `test/oracle/contract/testdata/golden/errors/` directory to be created alongside existing success goldens
- MCP error response serialization (how Kind reaches the JSON response) must be verified before golden file design

</code_context>

<specifics>
## Specific Ideas

No specific requirements -- user consistently chose the recommended (simplest) option for all decisions, indicating preference for straightforward patterns with full coverage.

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 24-validation-testing*
*Context gathered: 2026-04-15*
