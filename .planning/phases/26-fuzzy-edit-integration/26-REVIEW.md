---
phase: 26-fuzzy-edit-integration
reviewed: 2026-04-16T12:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/kernel/edit/replace_fuzzy_test.go
  - internal/kernel/edit/replace.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/fuzzy_edit_test.go
  - internal/kernel/fileops/fuzzy_edit.go
  - internal/kernel/fileops/skill.go
  - internal/kernel/fileops/tools.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 26: Code Review Report

**Reviewed:** 2026-04-16T12:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

The fuzzy edit integration adds two main capabilities: (1) a `search_body` parameter to `replace_symbol_body` for scoped sub-region replacement within symbol bodies, and (2) a standalone `fuzzy_edit` tool plus a fuzzy fallback in `replace_in_file`. The fuzzy engine (`internal/fuzzy`) is pure and well-designed. The integration code is generally clean, with proper path validation, error propagation, and consistent response formatting.

Three warnings relate to: file permission hardcoding in `replace.go`, the fuzzy fallback in `replace_in_file` returning an error result on no-match instead of the zero-replacement "success" that the literal path returns, and the `replace_in_file` fuzzy fallback silently swallowing the `ReadFile` error. Two info items note minor improvements.

## Warnings

### WR-01: Fuzzy fallback in replace_in_file returns error on no-match, breaking behavioral contract

**File:** `internal/kernel/fileops/tools.go:293-294`
**Issue:** When literal `ReplaceInFile` returns 0 matches, the code falls through to `fuzzy.Match`. If `fuzzy.Match` also fails (no match found), it returns `errorResult(fErr.Error())` (line 294). This changes the behavioral contract: the literal path returns `"0 replacement(s) made"` as a non-error text result (line 305), but the fuzzy fallback path returns an error. An agent calling `replace_in_file` with a non-matching pattern will see different error semantics depending on whether the pattern looks "close enough" to trigger a fuzzy near-miss vs. a complete miss. This inconsistency could confuse callers that check `IsError`.
**Fix:** When `fuzzy.Match` returns a no-match error, fall back to the zero-replacement text result instead of surfacing the fuzzy error:
```go
if fErr != nil {
    // Fuzzy also failed -- report 0 replacements (same as literal miss)
    return textResult(fmt.Sprintf("0 replacement(s) made in %s", args.Path)), nil, nil
}
```

### WR-02: Fuzzy fallback silently swallows ReadFile error

**File:** `internal/kernel/fileops/tools.go:285-287`
**Issue:** When `ReadFile` fails inside the fuzzy fallback (line 285-286), the code silently returns `"0 replacement(s) made"` as a success text result. This swallows a potentially important error (e.g., permission denied, file deleted between the first `ReplaceInFile` call and the re-read). The caller gets a misleading "0 replacements" message when the real issue is an I/O error.
**Fix:** Return the read error to the caller:
```go
if readErr != nil {
    return errorResult(readErr.Error()), nil, nil
}
```

### WR-03: Hardcoded 0644 file permissions in replace.go

**File:** `internal/kernel/edit/replace.go:84,103`
**Issue:** `os.WriteFile` is called with hardcoded `0644` permissions. If the original file had different permissions (e.g., `0600` for sensitive files, or `0755` for executable scripts), the replacement will silently change them. The `fileops` package uses `OverwriteFile` which may handle this correctly, but the `edit` package writes directly.
**Fix:** Stat the original file before writing and preserve its mode:
```go
fi, err := os.Stat(filePath)
if err != nil {
    return nil, serr.Wrap(serr.Internal, "stat file", err)
}
if err := os.WriteFile(filePath, result, fi.Mode()); err != nil {
```

## Info

### IN-01: Unused import alias in test file

**File:** `internal/kernel/edit/replace_fuzzy_test.go:10`
**Issue:** The `gen` import alias (`gen "github.com/postfix/serena/protocol/gen"`) uses a named alias. This is consistent with the rest of the codebase so it is fine stylistically, but worth noting this matches the project convention.
**Fix:** No action needed -- matches project convention.

### IN-02: FuzzyEditArgs uses double-negative for ellipsis control

**File:** `internal/kernel/fileops/tools.go:62`
**Issue:** The field `DisableEllipsis` with `allowEllipsis := !args.DisableEllipsis` (line 328) creates a double-negative that is slightly harder to reason about. The tool description says "default false, meaning ellipsis is enabled" which adds to the cognitive load.
**Fix:** Consider renaming to `AllowEllipsis` with default `true` if the MCP schema supports defaults. Otherwise acceptable as-is since the current naming makes the "opt-out" nature explicit for agent callers.

---

_Reviewed: 2026-04-16T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
