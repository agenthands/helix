---
phase: 24-validation-testing
reviewed: 2026-04-15T12:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - internal/kernel/diag/tools_test.go
  - internal/kernel/diag/tools.go
  - internal/kernel/edit/tools_test.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/fileops/validate_test.go
  - internal/kernel/symbols/tools_test.go
  - internal/kernel/symbols/tools.go
  - test/integration/errors_test.go
  - test/oracle/contract/errors_test.go
  - test/oracle/contract/testdata/golden/errors/invalid_args.golden
  - test/oracle/contract/testdata/golden/errors/no_workspace.golden
  - test/oracle/contract/testdata/golden/errors/not_found.golden
  - test/oracle/contract/testdata/golden/errors/unsupported.golden
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 24: Code Review Report

**Reviewed:** 2026-04-15T12:00:00Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Reviewed the validation testing infrastructure added in Phase 24: typed error construction across kernel tool packages (diag, edit, fileops, symbols), integration error tests, and contract golden tests. The code is well-structured with consistent error handling patterns using the `serr` typed error package. Most tools follow the same validation-then-execute pattern.

Key findings: the golden file for the "unsupported" error category captures an `internal:` error rather than `unsupported:`, indicating either a missing typed-error conversion in the LS pool or a golden baseline that was snapshotted before the error was properly typed. There is also an inconsistency in validation ordering between tool packages and one unnecessary `fmt.Sprintf` usage in the blast radius formatter.

## Warnings

### WR-01: Golden file for "unsupported" category captures wrong error kind

**File:** `test/oracle/contract/testdata/golden/errors/unsupported.golden:1`
**Issue:** The golden file content is `internal: definition` but the error category in the contract test is `"unsupported"`. The contract test (`TestError_CategoryContracts`) triggers this by calling `go_to_definition` on a `test.unknown` file. The LS pool apparently returns an `internal` error rather than an `unsupported` typed error. This means the golden file has been baselined against buggy behavior -- the contract test will pass but it validates the wrong error kind. When the pool is later fixed to return `unsupported:` errors, the golden will break unexpectedly.
**Fix:** Either (a) update the LS pool to wrap the "no language server for extension" error with `serr.New(serr.Unsupported, ...)` and regenerate the golden, or (b) add a comment in the golden file and the test case documenting this as a known deviation until the pool is fixed. The TODO comment at the top of `errors_test.go` partially acknowledges missing categories but does not call out this mismatch.

### WR-02: Inconsistent validation ordering between tool packages

**File:** `internal/kernel/diag/tools.go:81-89` vs `internal/kernel/edit/tools.go:145-156`
**Issue:** Diag tools check workspace first, then validate args (lines 81-89 of `tools.go`). Edit tools check args first, then workspace (lines 145-161 of `tools.go`). Symbols tools check args first, then workspace (line 191 of `tools.go`). Fileops tools check workspace first, then args (lines 96-102 of `tools.go`). This inconsistency means the same invalid input (empty path + no workspace) returns different error kinds depending on the tool family: `no_workspace` from diag/fileops tools vs `invalid_args` from edit/symbols tools. The integration tests in `errors_test.go` work around this (line 126 comment: "Since the no-workspace guard fires first for kernel tools") but the comment is only correct for diag/fileops, not edit/symbols.
**Fix:** Standardize the validation order across all tool packages. The recommended order is: validate args first (cheap, deterministic), then check workspace (requires runtime state). This is what edit and symbols already do. Update diag and fileops to match. This also simplifies integration test reasoning since `invalid_args` will always win over `no_workspace` when both conditions are true.

### WR-03: Integration test comment incorrectly describes validation ordering

**File:** `test/integration/errors_test.go:125-126`
**Issue:** The comment states "Since the no-workspace guard fires first for kernel tools, these cases still exercise the tool's error path." This is only true for diag and fileops tools. For edit and symbols tools, the args validation fires first. The `search_symbols_missing_query` case (line 128) sends empty args to `search_symbols`, which checks `args.Query == ""` before workspace -- so it returns `invalid_args` regardless of workspace state. The comment is misleading about which guard fires first.
**Fix:** Update the comment to clarify: "For symbols and edit tools, inline validation fires before the workspace check. For diag and fileops tools, the workspace check fires first. These cases exercise whichever guard fires first for each tool."

## Info

### IN-01: Unnecessary fmt.Sprintf for static strings

**File:** `internal/kernel/symbols/tools.go:448`
**Issue:** `sb.WriteString(fmt.Sprintf("\nCallers:\n"))` uses `fmt.Sprintf` with no format arguments. Same pattern at line 453 for Implementations header, though that one does use `%d`.
**Fix:** Replace with `sb.WriteString("\nCallers:\n")` (drop the Sprintf wrapper for the no-args case).

### IN-02: Unit tests only verify error construction, not actual tool handler behavior

**File:** `internal/kernel/diag/tools_test.go:1-50`, `internal/kernel/edit/tools_test.go:1-82`, `internal/kernel/symbols/tools_test.go:1-105`
**Issue:** All unit tests in the kernel packages construct `serr.New(...)` errors directly and verify the string format. They do not call the actual tool handlers. This means they test the error package, not the tools. The integration tests in `test/integration/errors_test.go` do exercise real handlers and provide stronger coverage. The unit tests are not wrong but provide limited additional value beyond what `internal/errors/errors_test.go` already covers.
**Fix:** Consider adding a comment to these test files noting they are "error format smoke tests" rather than handler tests, and that real handler coverage is in `test/integration/errors_test.go`. Alternatively, these could be removed if the integration tests are sufficient.

### IN-03: fileops noWorkspaceError helper does not include tool name

**File:** `internal/kernel/fileops/tools.go:87-88`
**Issue:** The `noWorkspaceError()` helper creates a typed error without `.WithTool(toolName)`, unlike other packages where each tool explicitly calls `.WithTool("tool_name")` on workspace errors. This means fileops no-workspace errors lack the tool name context that diag and edit errors include.
**Fix:** Either add a `toolName` parameter to `noWorkspaceError(tool string)` and pass it through, or add `.WithTool()` at each call site. Example:
```go
func noWorkspaceError(tool string) *mcpsdk.CallToolResult {
    return errorResult(serr.New(serr.NoWorkspace, "no active workspace").WithTool(tool).Error())
}
```

---

_Reviewed: 2026-04-15T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
