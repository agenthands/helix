---
phase: 08-advanced-testing
plan: 04
subsystem: test/integration
tags: [testing, error-paths, mcp, integration, destructive-tools, owasp]
requirements: [ADV-04]
dependency_graph:
  requires: [08-01]
  provides: [ADV-04 three-band error path coverage]
  affects: [test/integration/errors_test.go]
tech_stack:
  added: []
  patterns:
    - "Table-driven error-path tests with shared runErrCases harness"
    - "Structured MCP error oracle (IsError=true) rather than string matching"
    - "Three-band taxonomy: representative / destructive-exhaustive / read-only-smoke"
key_files:
  created:
    - test/integration/errors_test.go
  modified: []
decisions:
  - "Corrected kernel tool names and arg field names during execution (plan draft used aspirational names like find_symbol / list_dir / name_path that do not exist in the canonical registry). Actual names used: search_symbols, list_directory, find_files, search_in_files, get_symbol_overview; arg keys path / symbol_name / new_body / content / line / column / new_name."
  - "Dropped verify_edit from Band 3 and delete_memory nonexistent from Band 2 because the current implementations have no reachable IsError=true path for those scenarios. Per plan guidance (do NOT soften tests to match buggy behavior) both are reported here as ADV-04 hardening follow-ups."
  - "Maintained >=2 cases for every destructive tool by substituting delete_memory_empty_name for the missing not_found case."
metrics:
  duration_minutes: 7
  completed_date: "2026-04-09"
  total_test_cases: 30
  bands: 3
---

# Phase 08 Plan 04: Three-Band Error Path Coverage Summary

One-liner: ADV-04 ships as a single `test/integration/errors_test.go` with a shared `runErrCases` harness and three table-driven functions covering representative (Band 1), destructive-exhaustive (Band 2), and read-only-smoke (Band 3) error paths — all assertions use the structured `result.IsError` oracle so the suite survives future error-message rewording.

## What Shipped

**File:** `test/integration/errors_test.go` (`//go:build integration`)

**Types / helpers:**
- `errCase` struct — single row describing tool name, args, category, setup options.
- `runErrCases(t, cases)` — the only assertion site; carries the `TODO(#typed-errors)` marker that tracks the follow-up upgrade to `errors.Is` / structured content checks once kernel tools expose typed error values.

**Test functions (three bands, 30 total cases):**

| Band | Test | Cases | Purpose |
|------|------|-------|---------|
| 1 | `TestErrors_CategoryMatrix` | 5 | Representative `no_workspace` and `invalid_args` cases across symbols (`search_symbols`), fileops (`read_file`), and diag (`get_diagnostics`) tool families. |
| 2 | `TestErrors_DestructiveExhaustive` | 18 | Exhaustive matrix for all 9 destructive tools (5 edit + 4 memory) — each tool gets at least 2 cases exercising distinct error paths. |
| 3 | `TestErrors_ReadOnlySmoke` | 7 | Thin smoke coverage for read-only tools: `list_directory`, `find_files`, `search_in_files`, `get_symbol_overview`, `find_references`, `read_memory`, `search_memories`. |

**Destructive tools in Band 2 (canonical names verified against source):**

Edit (5): `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`
Memory (4): `write_memory`, `rename_memory`, `edit_memory`, `delete_memory`

**Assertion oracle (D-10 drift accepted):** every case runs through `runErrCases`, which asserts `result.IsError == true`. No test uses `strings.Contains` on error text. A single `TODO(#typed-errors)` comment at the harness assertion site documents the one-line upgrade path once kernel tools expose `ErrNoWorkspace` / `ErrFileNotFound` / `ErrSymbolNotFound` / `ErrInvalidArgs`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan draft used non-canonical tool and arg names**

- **Found during:** Task 1 (first `go test` run surfaced `unknown tool "find_symbol"`, `unknown tool "list_dir"`, `unknown tool "find_file"`, `unknown tool "search_for_pattern"`, `unknown tool "get_symbols_overview"`).
- **Issue:** The plan's `<interfaces>` block listed aspirational tool names and snake_case args (`find_symbol`, `list_dir`, `find_file`, `search_for_pattern`, `get_symbols_overview`, `name_path`, `relative_path`, `body`) that do not match the canonical names registered by `internal/kernel/*/tools.go` and the struct `json:"..."` tags used by the kernel Go structs.
- **Fix:** Grepped `internal/kernel/fileops/tools.go`, `internal/kernel/symbols/tools.go`, `internal/kernel/edit/tools.go`, `internal/kernel/diag/tools.go`, and `internal/skill/memory/skill.go` to extract the canonical registry. Re-wrote the Band 1 / Band 3 test tables and the entire Band 2 matrix with the canonical names and arg keys:

  | Plan draft | Canonical |
  |------------|-----------|
  | `find_symbol` | `search_symbols` (arg: `query`) |
  | `list_dir` | `list_directory` (arg: `path`) |
  | `find_file` | `find_files` (arg: `pattern`) |
  | `search_for_pattern` | `search_in_files` (arg: `pattern`) |
  | `get_symbols_overview` | `get_symbol_overview` (arg: `path`) |
  | `name_path` arg | `symbol_name` (edit tools) / `query` (search) |
  | `relative_path` arg | `path` |
  | `body` arg | `new_body` (replace) / `content` (insert) |

- **Files modified:** `test/integration/errors_test.go` (prominent deviation note added at the top of the file explaining the mapping for future readers).
- **Commits:** 390e720d (Task 1), 27b725f4 (Task 2).

**2. [Rule 3 - Blocking] Plan acceptance criteria referenced non-existent tools for forbidden list**

- **Issue:** The plan's acceptance criteria said `grep -n "replace_regex\|delete_lines\|insert_at_line" test/integration/errors_test.go` must return no matches — and correctly noted those tools "do not exist". No code change needed; verified via Grep.

## ADV-04 Hardening Findings (reported, not asserted)

Per plan guidance ("do NOT soften the test to match buggy behavior"), these are documented rather than asserted:

1. **`verify_edit` has no reachable negative error path.**
   Current implementation (`internal/kernel/edit/tools.go` `registerVerifyEdit`) neither checks for an active workspace nor validates the file path. It returns `textResult("No errors found.")` (`IsError=false`) whenever the diagnostic store is empty — including when called with no workspace and a bogus path. Impact: Band 3 cannot exercise an error case for `verify_edit` today. Recommended follow-up: add the same `root := rootFn(); if root == ""` guard the other edit tools use, plus `ErrFileNotFound` for missing paths.

2. **`delete_memory` on a nonexistent name returns success.**
   `s.store.Delete("does-not-exist")` currently returns nil, producing `Memory "does-not-exist" deleted.` (`IsError=false`). This is a classic idempotent-delete pattern, but it means a client typo silently "deletes" the wrong memory name with no feedback. Recommended follow-up: decide whether `delete_memory` should be strict (return `ErrMemoryNotFound`) or keep idempotent semantics; if the latter, add a distinct `IsAlreadyAbsent` field to the result so callers can distinguish. Band 2 still has 2 cases for `delete_memory` (`delete_memory_missing_name` and `delete_memory_empty_name`, both exercising the required-string guard — the case that actually matters for preventing accidental destruction).

3. **Kernel tool no-workspace guard runs BEFORE arg validation.**
   This means most `invalid_args` cases where a workspace is absent still hit the no-workspace guard first. The oracle is `IsError=true` either way, so tests stay green, but the `category` label on those cases is informational — not a ground-truth category tag. This is tracked by the single `TODO(#typed-errors)` comment and will stop mattering once tools expose typed errors.

## Parallel-agent Commit Hygiene Note

The Task 2 commit (`27b725f4`) also landed files from parallel worktree agents (`internal/mcp/middleware.go`, `internal/mcp/session.go`, `internal/profile/skill.go`, `test/integration/concurrency_test.go`) that were staged in the shared index before this plan started. I only explicitly staged `test/integration/errors_test.go` via `git add <file>`; the other files rode along because `git commit` flushes the whole index. The tests still pass and `go vet ./...` is clean, so the commit is accepted, but this is flagged here so the phase verifier can map those files back to the correct plans (08-03 concurrency and 08-02 profile tests).

## Verification Results

**Commands run:**

```
go vet ./...
go vet -tags integration ./test/integration/...
go test -tags integration ./test/integration/... -run '^TestErrors_CategoryMatrix$|^TestErrors_ReadOnlySmoke$' -race -count=1 -timeout=2m
go test -tags integration ./test/integration/... -run '^TestErrors_DestructiveExhaustive$' -race -count=1 -timeout=3m
go test -tags integration ./test/integration/... -run '^TestErrors' -race -count=1 -timeout=3m
```

**Results:**
- `go vet ./...` — clean
- `go vet -tags integration ./test/integration/...` — clean
- Band 1 + Band 3: PASS
- Band 2: PASS
- Full `^TestErrors` suite: PASS (ok 2.884s typical)

**Acceptance criteria check:**
- `grep -c "tool:" test/integration/errors_test.go` -> 30 (>=30 required) OK
- `grep -n "func TestErrors_CategoryMatrix"` matches OK
- `grep -n "func TestErrors_ReadOnlySmoke"` matches OK
- `grep -n "func TestErrors_DestructiveExhaustive"` matches OK
- `grep -n "result.IsError"` matches OK
- `grep -n "TODO(#typed-errors)"` matches (3 occurrences) OK
- `grep -n "replace_regex|delete_lines|insert_at_line"` returns no code matches OK
- `grep -n "strings.Contains.*err.*Error()"` returns no code matches OK
- All 9 destructive tool names present in file OK

## Commits

| Hash | Task | Scope |
|------|------|-------|
| 390e720d | Task 1 | Band 1 + Band 3 + shared `runErrCases` harness |
| 27b725f4 | Task 2 | Band 2 destructive exhaustive + 1 bonus Band 3 case (plus parallel-agent files — see hygiene note) |

## Self-Check: PASSED

- File exists: `test/integration/errors_test.go` FOUND
- Commit 390e720d exists in `git log --oneline --all` FOUND
- Commit 27b725f4 exists in `git log --oneline --all` FOUND
- All three `TestErrors_*` functions present FOUND
- `runErrCases` helper present FOUND
- `TODO(#typed-errors)` marker present FOUND
- Full `^TestErrors` suite passes under `-race -count=1` FOUND
