# Phase 24: Validation & Testing - Research

**Researched:** 2026-04-15
**Domain:** Go MCP tool input validation, typed error testing, golden file regression
**Confidence:** HIGH

## Summary

Phase 24 adds inline input validation to all 38+ MCP tools, upgrades the three-band error test suite from `IsError=true` assertions to typed Kind assertions, and extends the golden file infrastructure with per-Kind error shape templates.

The codebase is well-prepared for this phase. The error taxonomy from Phase 22 (`internal/errors/`) provides 7 error Kinds with sentinel matching (`errors.Is`), builder pattern (`New().WithTool().WithDetail()`), and JSON marshaling. Memory, profile, and workflow skill tools already have inline validation using this infrastructure. The kernel tools (symbols, edit, diag, fileops -- 24 tools total) have zero inline `InvalidArgs` validation; they rely entirely on the MCP SDK's JSON schema validation for missing required fields. The SDK schema catches missing keys but NOT empty strings, creating a validation gap that this phase fills.

**Primary recommendation:** Add per-tool inline validation at the top of each kernel tool handler following the memory skill pattern (`if !ok || val == ""` checks), upgrade the `errCase` struct with an `expectedKind` field that parses the error text prefix to extract Kind, and update the 4 existing per-Kind golden files to reflect the new typed error text format.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Per-tool inline validation. Each tool handler validates its own required args at the top of its exec function, following the existing `ValidatePath` pattern in fileops. No new abstraction, middleware, or shared validation helper -- just consistent if-checks before work begins.
- **D-02:** Audit all 38+ tools for missing required-field checks. Tools that already validate (e.g., fileops with `ValidatePath`, memory tools) get verified and skipped; tools with no validation get inline checks added. Every tool must have validation coverage.
- **D-03:** Zero-value numerics are valid. `line=0` and `column=0` are legitimate positions (LSP uses 0-based indexing). Only reject truly missing fields (key absent from args map) or wrong types.
- **D-04:** Empty strings for required string fields are rejected as InvalidArgs. An empty string for `path`, `symbol_name`, `query`, etc. is treated as missing -- agents should never pass empty strings for required fields.
- **D-05:** Optional fields are validated if present. If an optional field is provided with the wrong type, return InvalidArgs. Absent optional fields are silently ignored with defaults.
- **D-06:** Parse structured content from MCP error responses to extract the Kind field. Add an `expectedKind` field to the `errCase` struct and assert on it alongside `IsError=true`. The JSON error body carries Kind, Message, Tool, Detail -- parse and assert on Kind.
- **D-07:** Expand test coverage beyond the current ~30 cases. Every tool must have at least one InvalidArgs test case. Fill known gaps: verify_edit error paths, symbol_not_found with live LS where feasible, and InvalidArgs cases for each tool's required fields.
- **D-08:** Per-kind template golden files. One golden file per error Kind capturing the canonical JSON shape. All tools returning that Kind must match the template structure.
- **D-09:** Golden files live alongside success goldens in `test/oracle/contract/testdata/golden/errors/`.

### Claude's Discretion
- Exact validation code per tool (which fields are required, type assertions)
- Whether to consolidate repeated validation patterns within a tool group
- Which tools need expanded test cases beyond InvalidArgs
- Whether golden file matching uses exact JSON or structural wildcards for variable fields (message, detail)
- Plan decomposition and wave ordering

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| VAL-01 | User receives clear validation errors when providing invalid parameters to any tool | Kernel tools (symbols/edit/diag/fileops) need inline validation added; memory/profile/workflow already have it. MCP SDK catches missing required fields but NOT empty strings -- inline checks fill that gap. |
| VAL-02 | Three-band error tests upgraded from string matching to typed error kind assertions | `errCase` struct needs `expectedKind` field; error Kind is extractable from text prefix (`"kind: message"` format). Current `runErrCases` harness needs single-line change to parse and assert Kind. |
| VAL-03 | Golden files assert error response shapes per error kind for regression detection | 4 golden files already exist in `test/oracle/contract/testdata/golden/errors/`. They contain plain text error messages. Need update to reflect typed error format and add test cases for all 7 Kinds. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task**
- Build: `go build ./cmd/serena`
- Test: `go test ./...`
- Format: `gofmt -w .`
- Integration tests use `//go:build integration` tag
- Import alias: `serr "github.com/postfix/serena/internal/errors"`

## Architecture Patterns

### Current Error Flow (Critical for Understanding)

Errors flow from tool handlers to MCP clients as **plain text**, not structured JSON:

```
Tool handler error
  -> serr.New(serr.InvalidArgs, "message").WithTool("tool_name")
  -> err.Error() returns "invalid_args: message"
  -> errorResult(err.Error()) wraps in TextContent{Text: "invalid_args: message"}
  -> MCP response: {IsError: true, Content: [{text: "invalid_args: message"}]}
```

The `Error.MarshalJSON()` method exists but is NOT used in the MCP response path. All tool errors serialize via `.Error()` string method, producing `"kind: message"` or `"kind: message (detail)"` format. [VERIFIED: codebase grep of errorResult, textResult, and all tool handler return paths]

**Implication for D-06:** "Parse structured content from MCP error responses to extract the Kind field" means parsing the text prefix, NOT JSON parsing. The Kind is the prefix before the first `:` in the error text. For SDK validation errors (e.g., `validating "arguments": ...`), the Kind must be inferred from context (they are always `invalid_args`).

### Two Validation Layers

| Layer | What It Catches | Gap |
|-------|----------------|-----|
| MCP SDK JSON Schema | Missing required fields (non-omitempty struct fields) | Does NOT catch empty strings; error format is `validating "arguments": validating root: required: missing properties: ["field"]` |
| Inline tool validation | Empty strings, semantic constraints, cross-field validation | Currently missing from all 24 kernel tools |

**SDK validation fires BEFORE the tool handler runs.** When the SDK rejects a request, the tool handler never executes. This means:
- For kernel tools with typed arg structs (symbols, edit, diag, fileops): SDK catches `{}` (missing fields) but NOT `{"path": ""}` (empty string)
- For skill tools with `map[string]any` args (memory, workflow, profile): SDK does NO validation; all validation is inline

[VERIFIED: codebase analysis of mcpsdk.AddTool generic function, struct tags with json/jsonschema, and integration test golden files showing SDK validation error format]

### Tool Validation Audit Summary

| Package | Tools | Has Inline Validation? | Notes |
|---------|-------|----------------------|-------|
| `internal/kernel/symbols/` | 9 tools (go_to_definition, find_references, get_symbol_overview, search_symbols, get_hover_info, find_implementations, get_call_hierarchy, get_type_hierarchy, analyze_blast_radius) | NO | SDK catches missing fields; empty-string gap exists for `path`, `query` |
| `internal/kernel/edit/` | 6 tools (replace_symbol_body, insert_before_symbol, insert_after_symbol, rename_symbol, safe_delete_symbol, verify_edit) | NO | Empty-string gap for `path`, `symbol_name`, `new_body`, `content`, `new_name` |
| `internal/kernel/fileops/` | 6 tools (read_file, create_file, list_directory, find_files, search_in_files, replace_in_file) | PARTIAL | `ValidatePath` catches traversal but tools don't check for empty `path` before calling it; `ValidatePath("")` with non-empty root succeeds and passes `root/` which may produce confusing errors |
| `internal/kernel/diag/` | 3 tools (get_diagnostics, get_code_actions, format_code) | NO | Empty-string gap for `path` |
| `internal/skill/memory/` | 7 tools | YES | Full inline validation with `serr.InvalidArgs` |
| `internal/skill/workflow/` | 2 tools (onboard_project, prepare_for_new_conversation) | N/A | No required args (onboard), all optional (handoff) |
| `internal/profile/` | 2 tools (switch_mode, get_token_budget) | PARTIAL | switch_mode validates target_mode; get_token_budget has all optional args |
| `internal/mcp/` | 3 tools (ping, echo, activate_project) | PARTIAL | activate_project has workspace error; ping/echo have no required args |

[VERIFIED: grep and read of all tool handler files]

### Recommended Validation Pattern

Follow the memory skill pattern for skill tools (`map[string]any` args), and add inline checks at the top of kernel tool handlers (typed arg structs):

**Kernel tools (typed args -- validation for empty strings):**
```go
// Source: Pattern derived from existing memory skill validation + D-04
func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
    if args.Path == "" {
        return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
            WithTool("go_to_definition").Error()), nil, nil
    }
    // ... existing handler code
}
```

**Per D-03:** Do NOT validate `line == 0` or `column == 0` as invalid -- these are legitimate 0-based LSP positions. The SDK already ensures they are present (non-omitempty int fields default to 0 when missing from JSON, but the SDK marks them required so truly absent fields are caught).

**Important nuance:** Go's `json.Unmarshal` deserializes a missing `int` field as `0`. The MCP SDK's JSON schema validation catches the absence at the schema level BEFORE deserialization. So `{}` for a tool requiring `line` produces an SDK validation error, not `line=0`. However, `{"path": "main.go"}` without `line` and `column` also triggers SDK validation. Only `{"path": "main.go", "line": 0, "column": 0}` passes SDK validation with line/column at 0, which is valid per D-03.

### Recommended Project Structure (changes only)

```
internal/kernel/symbols/tools.go     # Add validation at top of each handler
internal/kernel/edit/tools.go        # Add validation at top of each handler
internal/kernel/diag/tools.go        # Add validation at top of each handler
internal/kernel/fileops/tools.go     # Add empty-string checks before ValidatePath
test/integration/errors_test.go      # Add expectedKind to errCase, upgrade runErrCases
test/oracle/contract/errors_test.go  # Update golden assertions for typed error format
test/oracle/contract/testdata/golden/errors/*.golden  # Update golden file content
```

### Anti-Patterns to Avoid
- **Shared validation middleware:** D-01 explicitly forbids this. Each tool validates its own args inline.
- **Validating `line=0` or `column=0` as invalid:** D-03 says these are valid LSP positions.
- **JSON-parsing error responses for Kind extraction:** Errors are plain text `"kind: message"` format, not JSON. Parse the text prefix.
- **Adding validation to tools that already have it:** Memory, profile, workflow tools already validate. Verify and skip.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Missing-field detection | Custom reflection-based validator | MCP SDK JSON schema (already handles it) + inline empty-string checks | SDK already generates schema from struct tags |
| Golden file comparison | Custom diff library | Existing `harness.AssertGolden` with `GOLDEN_UPDATE=1` | Well-tested, handles file creation, update mode |
| Error Kind extraction from text | Regex parser | `strings.SplitN(text, ":", 2)[0]` or `strings.HasPrefix` | Error format is deterministic: `"kind: message"` |
| Structured error assertions | Custom test matcher | `errors.Is(err, serr.ErrInvalidArgs)` sentinel matching | Already works in unit tests (see `fileops_test.go`) |

**Key insight:** The test harness already does the heavy lifting. The `callToolExpectError` helper returns the `CallToolResult`, and `textContent` extracts the text. Kind extraction from the text prefix is trivial.

## Common Pitfalls

### Pitfall 1: SDK Validation vs Inline Validation Ordering
**What goes wrong:** Adding inline empty-string validation that never fires because the SDK already rejected the request at the schema level for a different reason.
**Why it happens:** When a request has BOTH missing fields and empty strings, the SDK rejects first.
**How to avoid:** Tests must cover both `{}` (SDK catches, returns schema error format) and `{"path": ""}` (inline catches, returns `serr.InvalidArgs` format). Different error text formats for the same Kind.
**Warning signs:** Tests pass with `{}` args but fail with `{"path": ""}` args -- indicates inline validation is not actually being exercised.

### Pitfall 2: Two Different InvalidArgs Error Formats
**What goes wrong:** Golden files assume one format, but SDK validation produces `validating "arguments": validating root: required: missing properties: ["query"]` while inline validation produces `invalid_args: missing required field: path`.
**Why it happens:** SDK validation and tool-level validation use completely different error formatting.
**How to avoid:** The golden file for `invalid_args` must account for BOTH formats, or test cases must be explicit about which validation layer they target. Per D-08 (per-kind template golden files), recommend using the typed error format as the canonical shape, and treating SDK validation errors as a separate concern.
**Warning signs:** Golden file tests fail because error text format changed between SDK and inline validation.

### Pitfall 3: errCase Category Field vs expectedKind
**What goes wrong:** Confusing the existing `category` field (free-form string like "no_workspace", "invalid_args") with the new `expectedKind` field (must match `serr.Kind` constants like `serr.NoWorkspace`, `serr.InvalidArgs`).
**Why it happens:** The existing errCase struct uses category strings that look like Kind values but are not the same type.
**How to avoid:** Add `expectedKind serr.Kind` as a new field alongside `category`. Parse Kind from error text and compare with `expectedKind`.
**Warning signs:** Tests compile but assert on the wrong thing.

### Pitfall 4: Verify Edit Has No Error Paths
**What goes wrong:** Trying to add InvalidArgs tests for `verify_edit` that pass but the tool has no validation or workspace check.
**Why it happens:** As documented in the existing tests (errors_test.go line 186-192), verify_edit returns success even without a workspace.
**How to avoid:** This phase should ADD validation to verify_edit (empty path check, workspace check) as part of D-02, then add tests for those new error paths.
**Warning signs:** verify_edit tests pass without any changes to the tool code -- means the tool is not actually validating.

## Code Examples

### Example 1: Kernel Tool Inline Validation (Symbol Tool)
```go
// Source: Pattern from internal/skill/memory/skill.go adapted for typed args
// Add at top of go_to_definition handler:
if args.Path == "" {
    return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
        WithTool("go_to_definition").Error()), nil, nil
}
```

### Example 2: Updated errCase Struct with expectedKind
```go
// Source: test/integration/errors_test.go with D-06 upgrade
type errCase struct {
    name         string
    tool         string
    args         map[string]any
    category     string       // kept for backwards compat / grouping
    expectedKind string       // "invalid_args", "no_workspace", "not_found", etc.
    setupOpts    Options
    setupFunc    func(t *testing.T, td *TestDaemon)
}
```

### Example 3: Kind Extraction from Error Text
```go
// Source: Derived from internal/errors/errors.go Error() format
func extractKind(errorText string) string {
    // Typed errors: "invalid_args: message" or "invalid_args: message (detail)"
    if idx := strings.Index(errorText, ": "); idx > 0 {
        candidate := errorText[:idx]
        // Verify it's a known kind
        switch candidate {
        case "not_found", "invalid_args", "no_workspace", "unsupported",
             "internal", "circuit_open", "timeout":
            return candidate
        }
    }
    // SDK validation errors: "validating \"arguments\": ..."
    if strings.HasPrefix(errorText, "validating ") {
        return "invalid_args" // SDK schema validation is always an args problem
    }
    return "" // unknown format
}
```

### Example 4: Updated runErrCases with Kind Assertion
```go
// Source: test/integration/errors_test.go with D-06 upgrade
func runErrCases(t *testing.T, cases []errCase) {
    t.Helper()
    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            td := StartTestDaemon(t, tc.setupOpts)
            if tc.setupFunc != nil {
                tc.setupFunc(t, td)
            }
            result := callToolExpectError(t, td.Session, tc.tool, tc.args)
            assert.True(t, result.IsError, "[%s] expected IsError=true", tc.category)
            
            if tc.expectedKind != "" {
                text := textContent(result)
                gotKind := extractKind(text)
                assert.Equal(t, tc.expectedKind, gotKind,
                    "[%s] expected kind %s, got %s (text: %s)",
                    tc.name, tc.expectedKind, gotKind, text)
            }
        })
    }
}
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify/assert + testify/require |
| Config file | None (standard Go test infrastructure) |
| Quick run command | `go test ./internal/kernel/... -run TestValidat -count=1` |
| Full suite command | `go test ./... -count=1 && go vet ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VAL-01 | Invalid params return InvalidArgs before work begins | unit + integration | `go test ./internal/kernel/... -run TestValidat -count=1` (unit); `go test -tags integration ./test/integration/... -run TestErrors -count=1` (integration) | Unit: Wave 0; Integration: exists at test/integration/errors_test.go |
| VAL-02 | Three-band tests assert on error Kind | integration | `go test -tags integration ./test/integration/... -run TestErrors -count=1` | exists (needs upgrade) |
| VAL-03 | Golden files capture error response shape per kind | integration | `go test -tags integration ./test/oracle/contract/... -run TestError -count=1` | exists (needs update) |

### Sampling Rate
- **Per task commit:** `go test ./internal/kernel/... -count=1 && go vet ./internal/kernel/...`
- **Per wave merge:** `go test ./... -count=1 && go vet ./...`
- **Phase gate:** Full suite green + integration tests green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] Unit tests for inline validation per tool package (internal/kernel/symbols, edit, diag, fileops) -- currently zero validation unit tests
- [ ] `extractKind` helper function in test/integration/ for Kind assertion

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | Inline per-tool validation using serr.InvalidArgs |
| V6 Cryptography | no | N/A |

### Known Threat Patterns for MCP Tools

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via empty/crafted path | Tampering | ValidatePath + empty-string rejection |
| Empty symbol_name causing downstream panics | Tampering | Inline validation returning InvalidArgs before work begins |
| Type confusion in map[string]any args | Tampering | Type assertion with ok check (`val, ok := args["key"].(string)`) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | MCP SDK JSON schema validation marks non-omitempty Go struct fields as `required` | Architecture Patterns | If SDK does NOT mark them required, more inline validation needed for missing fields -- low risk since existing integration tests confirm SDK catches `{}` |
| A2 | `int` fields missing from JSON but present in schema as required are caught by SDK before deserialization to 0 | Architecture Patterns | If not, `line`/`column` would silently default to 0 -- but D-03 says 0 is valid anyway, so impact is minimal |

## Open Questions

1. **D-06 says "JSON error body carries Kind, Message, Tool, Detail"**
   - What we know: Error responses are plain text via `.Error()`, NOT JSON. The Error struct has `MarshalJSON()` but it's unused in the MCP response path.
   - What's unclear: Does D-06 expect us to CHANGE error serialization to JSON, or to extract Kind from the existing text format?
   - Recommendation: Extract Kind from existing text prefix (`"kind: message"` format). Changing serialization is a larger change that would break existing golden files and error consumers. If JSON serialization is desired, it should be a separate decision.

2. **Golden file format for updated errors**
   - What we know: Current golden files contain plain text (`"no active workspace -- activate a project first"`). After Phase 23 migration, errors use typed format (`"no_workspace: no active workspace"`).
   - What's unclear: Whether golden files should use structural wildcards for variable fields (message, detail) per D-08.
   - Recommendation: Use exact text matching (existing harness approach) with normalization. Variable fields (paths, timestamps) are already handled by `normalizeResponse`. The per-Kind golden files should capture one canonical example per Kind.

## Sources

### Primary (HIGH confidence)
- `internal/errors/kinds.go` - 7 Kind constants verified [VERIFIED: codebase read]
- `internal/errors/errors.go` - Error struct, builders, MarshalJSON [VERIFIED: codebase read]
- `internal/kernel/fileops/tools.go` - errorResult, textResult, noWorkspaceError patterns [VERIFIED: codebase read]
- `internal/kernel/symbols/tools.go` - All 9 tool handlers, zero inline validation [VERIFIED: codebase read]
- `internal/kernel/edit/tools.go` - All 6 tool handlers, zero inline validation [VERIFIED: codebase read]
- `internal/kernel/diag/tools.go` - All 3 tool handlers, zero inline validation [VERIFIED: codebase read]
- `internal/skill/memory/skill.go` - Full inline validation pattern [VERIFIED: codebase read]
- `internal/profile/skill.go` - Partial validation (switch_mode) [VERIFIED: codebase read]
- `test/integration/errors_test.go` - Three-band harness, errCase struct, runErrCases [VERIFIED: codebase read]
- `test/oracle/contract/errors_test.go` - Golden file error tests, 4 existing golden files [VERIFIED: codebase read]
- `test/harness/tools.go` - CallToolExpectError, TextContent [VERIFIED: codebase read]
- `test/harness/golden.go` - AssertGolden, GoldenStore, GOLDEN_UPDATE [VERIFIED: codebase read]

### Secondary (MEDIUM confidence)
- MCP Go SDK JSON schema validation behavior (non-omitempty = required) [VERIFIED: integration test golden file for invalid_args shows SDK validation format]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All code is in-repo Go, no external dependencies needed
- Architecture: HIGH - Error flow, validation patterns, test harness all verified by reading source
- Pitfalls: HIGH - Identified from actual code analysis (two error formats, verify_edit gap)

**Research date:** 2026-04-15
**Valid until:** 2026-05-15 (stable -- all findings are codebase-specific)
