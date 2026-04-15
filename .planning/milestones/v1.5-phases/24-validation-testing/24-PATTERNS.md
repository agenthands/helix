# Phase 24: Validation & Testing - Patterns

**Generated:** 2026-04-15
**Source:** Codebase analysis of canonical references from 24-CONTEXT.md and 24-RESEARCH.md

---

## File Inventory

### Files to Modify

| File | Role | Closest Analog |
|------|------|----------------|
| `internal/kernel/symbols/tools.go` | Add inline validation to 9 symbol tool handlers | `internal/skill/memory/skill.go` (validation pattern) |
| `internal/kernel/edit/tools.go` | Add inline validation to 6 edit tool handlers | `internal/skill/memory/skill.go` (validation pattern) |
| `internal/kernel/diag/tools.go` | Add inline validation to 3 diagnostic tool handlers | `internal/skill/memory/skill.go` (validation pattern) |
| `internal/kernel/fileops/tools.go` | Add empty-string checks before existing ValidatePath calls | `internal/kernel/fileops/tools.go` (self -- augment existing pattern) |
| `test/integration/errors_test.go` | Add `expectedKind` to errCase, upgrade `runErrCases`, add new cases | `test/integration/errors_test.go` (self -- extend existing) |
| `test/oracle/contract/errors_test.go` | Update golden assertions for typed error format | `test/oracle/contract/errors_test.go` (self -- extend existing) |
| `test/oracle/contract/testdata/golden/errors/*.golden` | Update golden file content to reflect typed error text | Current files (update in place) |

### Files to Create

| File | Role | Closest Analog |
|------|------|----------------|
| Unit test files per kernel package (e.g., `internal/kernel/symbols/tools_test.go`) | Validation unit tests | `internal/kernel/fileops/fileops_test.go` |

---

## Pattern 1: Inline Validation in Kernel Tool Handlers (Typed Args)

**Target files:** `symbols/tools.go`, `edit/tools.go`, `diag/tools.go`, `fileops/tools.go`

Kernel tools receive typed arg structs from the MCP SDK. The SDK catches missing fields via JSON schema but does NOT catch empty strings. Inline validation must be added at the top of each handler, before any work begins.

### Current handler pattern (NO validation) -- `internal/kernel/symbols/tools.go:188-203`

```go
func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
    rt, err := acquireLease(ctx, k, wsKeyFn)
    if err != nil {
        return errorResult(err.Error()), nil, nil
    }
    lease, err := rt.AcquireSession(ctx, "default", false)
    if err != nil {
        return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
    }
    locs, err := GoToDefinition(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col)
    if err != nil {
        return errorResult(err.Error()), nil, nil
    }
    return textResult(formatLocations(locs)), nil, nil
}
```

### Reference pattern (WITH validation) -- `internal/skill/memory/skill.go:172-184`

```go
func (s *MemorySkill) execWrite(args map[string]interface{}) (string, error) {
    name, ok := args["name"].(string)
    if !ok || name == "" {
        return "", serr.New(serr.InvalidArgs, "'name' parameter is required").WithTool("write_memory")
    }
    content, ok := args["content"].(string)
    if !ok {
        return "", serr.New(serr.InvalidArgs, "'content' parameter is required").WithTool("write_memory")
    }
    // ... proceed with work
}
```

### Target pattern for kernel tools (typed args)

Add validation block at the top of the handler, BEFORE the `acquireLease` / workspace check:

```go
func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
    // --- NEW: inline validation (D-01, D-04) ---
    if args.Path == "" {
        return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
            WithTool("go_to_definition").Error()), nil, nil
    }
    // --- END validation ---

    rt, err := acquireLease(ctx, k, wsKeyFn)
    // ... rest unchanged
}
```

**Key rules:**
- D-03: Do NOT validate `args.Line == 0` or `args.Col == 0` -- these are valid 0-based LSP positions.
- D-04: Empty strings for required string fields ARE rejected.
- D-05: Optional fields (like `direction` in CallHierarchyArgs) are validated only if present AND wrong type. Since typed args handle type safety via Go structs, optional string fields only need empty-check if they have semantic constraints.
- Return format: `errorResult(serr.New(...).WithTool("tool_name").Error())` -- produces `"invalid_args: missing required field: path"` text.

### Required fields per tool (audit)

| Tool | Required String Fields | Required Numeric Fields (0 valid) |
|------|----------------------|----------------------------------|
| `go_to_definition` | path | line, column |
| `find_references` | path | line, column |
| `get_symbol_overview` | path | -- |
| `search_symbols` | query | -- |
| `get_hover_info` | path | line, column |
| `find_implementations` | path | line, column |
| `get_call_hierarchy` | path | line, column |
| `get_type_hierarchy` | path | line, column |
| `analyze_blast_radius` | path | line, column |
| `replace_symbol_body` | path, symbol_name, new_body | -- |
| `insert_before_symbol` | path, symbol_name, content | -- |
| `insert_after_symbol` | path, symbol_name, content | -- |
| `rename_symbol` | path, new_name | line, column |
| `safe_delete_symbol` | path, symbol_name | -- |
| `verify_edit` | path | -- |
| `read_file` | path | -- |
| `create_file` | path | -- |
| `list_directory` | path | -- |
| `find_files` | pattern | -- |
| `search_in_files` | pattern | -- |
| `replace_in_file` | path, pattern | -- |
| `get_diagnostics` | path | -- |
| `get_code_actions` | path | line, column |
| `format_code` | path | -- |

---

## Pattern 2: Error Taxonomy (Builder Pattern)

**Source:** `internal/errors/errors.go` and `internal/errors/kinds.go`

### Kind constants -- `internal/errors/kinds.go:13-21`

```go
const (
    NotFound    Kind = "not_found"
    InvalidArgs Kind = "invalid_args"
    NoWorkspace Kind = "no_workspace"
    Unsupported Kind = "unsupported"
    Internal    Kind = "internal"
    CircuitOpen Kind = "circuit_open"
    Timeout     Kind = "timeout"
)
```

### Builder pattern -- `internal/errors/errors.go:25-47`

```go
// Create:
serr.New(serr.InvalidArgs, "missing required field: path").WithTool("read_file")

// Wrap existing error:
serr.Wrap(serr.NoWorkspace, "workspace not activated", err)

// Error string output format:
//   "invalid_args: missing required field: path"
//   "no_workspace: workspace not activated"
//   "invalid_args: path outside workspace root (../etc/passwd outside /workspace)"  -- with detail
```

### Sentinel matching -- `internal/kernel/fileops/fileops_test.go:21-23`

```go
if !errors.Is(err, serr.ErrInvalidArgs) {
    t.Fatalf("expected InvalidArgs error, got: %v", err)
}
```

---

## Pattern 3: Existing ValidatePath Pattern (fileops)

**Source:** `internal/kernel/fileops/validate.go`

ValidatePath already returns typed errors but the tool handlers do NOT check for empty path before calling it. `ValidatePath("", path)` returns `NoWorkspace` (root is empty), and `ValidatePath(root, "")` resolves to `root/` which may produce confusing downstream errors.

### ValidatePath signature -- `validate.go:14`

```go
func ValidatePath(root, path string) (string, error) {
    if root == "" {
        return "", serr.New(serr.NoWorkspace, "workspace root is empty")
    }
    // ... resolves and checks path traversal
}
```

### Current fileops tool handler pattern (no empty-path check) -- `tools.go:94-113`

```go
func(..., args ReadFileArgs) (*mcpsdk.CallToolResult, any, error) {
    root := rootFn()
    if root == "" {
        return noWorkspaceError(), nil, nil
    }
    // MISSING: if args.Path == "" { return errorResult(...) }
    content, err := ReadFile(root, args.Path)
    if err != nil {
        return errorResult(err.Error()), nil, nil
    }
    return textResult(content), nil, nil
}
```

### Target: add empty-path check between workspace check and business logic

```go
root := rootFn()
if root == "" {
    return noWorkspaceError(), nil, nil
}
if args.Path == "" {
    return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
        WithTool("read_file").Error()), nil, nil
}
```

### noWorkspaceError helper -- `tools.go:86-88`

```go
func noWorkspaceError() *mcpsdk.CallToolResult {
    return errorResult(serr.New(serr.NoWorkspace, "no active workspace").Error())
}
```

### errorResult / textResult helpers (duplicated per package)

Each kernel package (`symbols`, `edit`, `diag`, `fileops`) has its own copy of `errorResult` and `textResult`. Validation uses `errorResult(serr.New(...).Error())` consistently.

---

## Pattern 4: Three-Band Error Test Harness

**Source:** `test/integration/errors_test.go`

### Current errCase struct -- `errors_test.go:32-40`

```go
type errCase struct {
    name      string
    tool      string
    args      map[string]any
    category  string         // "no_workspace" | "not_found" | "symbol_not_found" | "invalid_args"
    setupOpts Options
    setupFunc func(t *testing.T, td *TestDaemon)
}
```

### Current runErrCases harness -- `errors_test.go:43-64`

```go
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
            // TODO(#typed-errors): upgrade to errors.Is / structured-content field
            assert.True(t, result.IsError, "[%s] expected IsError=true", tc.category)
        })
    }
}
```

### Target: add expectedKind field and Kind assertion

```go
type errCase struct {
    name         string
    tool         string
    args         map[string]any
    category     string    // kept for grouping
    expectedKind string    // NEW: "invalid_args", "no_workspace", "not_found", etc.
    setupOpts    Options
    setupFunc    func(t *testing.T, td *TestDaemon)
}
```

### Kind extraction from error text (new helper)

Error text format is `"kind: message"` or `"kind: message (detail)"`. SDK validation errors use `"validating \"arguments\": ..."`.

```go
func extractKind(errorText string) string {
    if idx := strings.Index(errorText, ": "); idx > 0 {
        candidate := errorText[:idx]
        switch candidate {
        case "not_found", "invalid_args", "no_workspace", "unsupported",
             "internal", "circuit_open", "timeout":
            return candidate
        }
    }
    if strings.HasPrefix(errorText, "validating ") {
        return "invalid_args"
    }
    return ""
}
```

### Updated runErrCases with Kind assertion

```go
result := callToolExpectError(t, td.Session, tc.tool, tc.args)
assert.True(t, result.IsError, "[%s] expected IsError=true", tc.category)

if tc.expectedKind != "" {
    text := textContent(result)
    gotKind := extractKind(text)
    assert.Equal(t, tc.expectedKind, gotKind,
        "[%s] expected kind %s, got %s (text: %s)",
        tc.name, tc.expectedKind, gotKind, text)
}
```

### Existing test case patterns -- examples from `errors_test.go`

**no_workspace case (valid args, no workspace activated):**
```go
{
    name:      "search_symbols_no_workspace",
    tool:      "search_symbols",
    args:      map[string]any{"query": "Foo"},
    category:  "no_workspace",
    setupOpts: Options{SkipLS: true},
}
```

**invalid_args case (missing required fields):**
```go
{
    name:      "read_file_missing_path",
    tool:      "read_file",
    args:      map[string]any{},
    category:  "invalid_args",
    setupOpts: Options{SkipLS: true},
}
```

### callToolExpectError helper -- `test/integration/helpers.go:60-69`

```go
func callToolExpectError(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
    tb.Helper()
    result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
        Name:      name,
        Arguments: args,
    })
    require.NoError(tb, err, "tool %s call error (protocol level)", name)
    require.True(tb, result.IsError, "tool %s should have returned error but succeeded: %s", name, textContent(result))
    return result
}
```

---

## Pattern 5: Golden File Infrastructure

**Source:** `test/oracle/contract/golden_test.go`, `test/harness/golden.go`, `test/oracle/contract/errors_test.go`

### AssertGolden -- `test/harness/golden.go:77-101`

```go
func AssertGolden(t *testing.T, path string, got []byte) {
    t.Helper()
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        t.Fatalf("mkdir golden dir: %v", err)
    }
    if *updateGolden || os.Getenv("GOLDEN_UPDATE") == "1" {
        if err := os.WriteFile(path, got, 0o644); err != nil {
            t.Fatalf("update golden %s: %v", path, err)
        }
        t.Logf("updated golden file: %s", path)
        return
    }
    wantBytes, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
    }
    if string(wantBytes) != string(got) {
        t.Errorf("golden %s mismatch.\n--- want:\n%s\n+++ got:\n%s\n(run with -update to accept)",
            path, string(wantBytes), string(got))
    }
}
```

### Current error golden file content

```
# invalid_args.golden
validating "arguments": validating root: required: missing properties: ["query"]

# no_workspace.golden
no active workspace — activate a project first

# not_found.golden
file not found: nonexistent_file_xyz.go

# unsupported.golden
acquire session: spawning worker: no language server configured for
```

**Key observation:** `no_workspace.golden` uses the old pre-Phase-23 error text (`"no active workspace -- activate a project first"`) while the typed error format from `noWorkspaceError()` in fileops/tools.go produces `"no_workspace: no active workspace"`. The golden files need updating to match the current typed error text.

### Error golden test pattern -- `test/oracle/contract/errors_test.go:73-103`

```go
func TestError_CategoryContracts(t *testing.T) {
    gDir := errorGoldenDir()
    noWSRunner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
    fixtureDir := harness.PrepareFixture(t, "go")
    wsRunner := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: fixtureDir,
        SkipLS:       true,
    })
    for _, tc := range errorCategories() {
        tc := tc
        t.Run(tc.category, func(t *testing.T) {
            session := noWSRunner.Session
            wsDir := ""
            if tc.needsWorkspace {
                session = wsRunner.Session
                wsDir = fixtureDir
            }
            result := harness.CallToolExpectError(t, session, tc.tool, tc.args)
            text := harness.TextContent(result)
            normalized := normalizeResponse(text, wsDir)
            goldenPath := filepath.Join(gDir, tc.category+".golden")
            harness.AssertGolden(t, goldenPath, []byte(normalized))
        })
    }
}
```

### normalizeResponse for deterministic golden comparison -- `golden_test.go:24-42`

```go
func normalizeResponse(text string, workspaceDir string) string {
    if workspaceDir != "" {
        text = strings.ReplaceAll(text, workspaceDir, "<WORKSPACE>")
    }
    text = regexp.MustCompile(`/[^\s"]+/testdata/fixtures/`).ReplaceAllString(text, "<FIXTURE_ROOT>/")
    text = regexp.MustCompile(`(?:/tmp|/var/folders)/[^\s"]+`).ReplaceAllString(text, "<TMPDIR>")
    // ... more normalizations for timestamps, UUIDs, etc.
    return text
}
```

---

## Pattern 6: Unit Tests for Validation (fileops_test.go Reference)

**Source:** `internal/kernel/fileops/fileops_test.go`

The fileops tests use standard Go `testing` with `errors.Is` for typed error assertions. This is the pattern for unit-testing inline validation in kernel packages.

### errors.Is sentinel matching -- `fileops_test.go:20-27`

```go
func TestValidatePathRejectsOutsideRoot(t *testing.T) {
    root := t.TempDir()
    _, err := ValidatePath(root, "/etc/passwd")
    if err == nil {
        t.Fatal("expected error for path outside root")
    }
    if !errors.Is(err, serr.ErrInvalidArgs) {
        t.Fatalf("expected InvalidArgs error, got: %v", err)
    }
}
```

### Combined kind + message assertion -- `fileops_test.go:78-89`

```go
func TestReadFileNotFound(t *testing.T) {
    root := t.TempDir()
    _, err := ReadFile(root, "nonexistent.txt")
    if err == nil {
        t.Fatal("expected error for non-existent file")
    }
    if !errors.Is(err, serr.ErrNotFound) {
        t.Fatalf("expected NotFound error, got: %v", err)
    }
    if !strings.Contains(err.Error(), "file not found") {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

**Note:** Kernel tool handlers (symbols, edit, diag) do not expose their inner functions for direct unit testing the same way fileops does. Validation unit tests for these packages will need to either:
1. Test the validation logic by calling the tool handler via the MCP SDK (integration-style), or
2. Extract validation into testable helper functions within each package.

---

## Pattern 7: verify_edit Special Case

**Source:** `internal/kernel/edit/tools.go:320-341`

verify_edit currently has NO workspace check and NO path validation. It calls `wsKeyFn().RepoRoot` directly without guarding against an empty workspace key, and passes `args.Path` without empty-string check.

```go
func(..., args VerifyEditArgs) (*mcpsdk.CallToolResult, any, error) {
    uri := filePathToURI(wsKeyFn().RepoRoot, args.Path)  // NO workspace check
    result, err := VerifyEdit(ctx, diagStore, uri)
    if err != nil {
        return errorResult(err.Error()), nil, nil
    }
    if !result.HasErrors {
        return textResult("No errors found."), nil, nil  // returns success even without workspace
    }
    // ...
}
```

This needs both a workspace check AND an empty-path check added, following the pattern used by other edit tools (e.g., `registerReplaceBody` at line 145-148):

```go
wsKey := wsKeyFn()
rt, err := k.GetRuntime(wsKey)
if err != nil {
    return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
}
```

---

## Pattern 8: Diag Tools -- Legacy Error Format

**Source:** `internal/kernel/diag/tools.go:80-83`

The diag tools use a hardcoded string instead of the typed error builder for the no-workspace check:

```go
root := rootFn()
if root == "" {
    return errorResult("no active workspace - activate a project first"), nil, nil
}
```

This should be migrated to use the typed error pattern:

```go
if root == "" {
    return errorResult(serr.New(serr.NoWorkspace, "no active workspace").Error()), nil, nil
}
```

Or use a `noWorkspaceError()` helper like fileops does (line 86-88 of fileops/tools.go).

---

## Data Flow Summary

```
Agent sends MCP request
  |
  v
MCP SDK JSON Schema Validation
  - Catches: missing required fields (key absent from JSON)
  - Error format: 'validating "arguments": validating root: required: missing properties: ["field"]'
  - SDK marks non-omitempty struct fields as required
  |  (if passes)
  v
Tool Handler Entry
  |
  v
NEW: Inline Validation (Phase 24)
  - Catches: empty strings for required fields, semantic constraints
  - Error format: "invalid_args: missing required field: path"
  - Uses: serr.New(serr.InvalidArgs, msg).WithTool(name).Error()
  |  (if passes)
  v
Workspace Check (existing)
  - Catches: no active workspace
  - Error format: "no_workspace: no active workspace" (or "no_workspace: workspace not activated")
  |  (if passes)
  v
Business Logic (existing)
  - Catches: not_found, internal errors, etc.
```

---

*Phase: 24-validation-testing*
*Patterns extracted: 2026-04-15*
