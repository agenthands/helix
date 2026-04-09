# Deferred Items - Phase 06

## ~~Workspace Key Language Routing~~ (RESOLVED in 06-04)

**Resolved by:** 06-04 Tasks 1-2

## edit/rename.go pathToURI does not resolve relative paths

**Found during:** 06-04 Task 2
**Severity:** Medium - edit tools may fail with relative paths against gopls
**Description:** `internal/kernel/edit/rename.go` has its own `pathToURI` that does not resolve relative paths against the workspace root. The same bug was fixed in `internal/kernel/symbols/tools.go` during 06-04.
**Impact:** Edit tools using file paths may produce incorrect URIs when given relative paths.
**Suggested fix:** Update `pathToURI` in edit/rename.go to accept a root parameter, matching the fix in symbols/tools.go.

## TestHTTPSmoke_WithLS schema validation failure (pre-existing)

**Found during:** 06-04 Task 2
**Severity:** Low - pre-existing test failure unrelated to 06-04 changes
**Description:** `TestHTTPSmoke_WithLS` fails with "unexpected additional properties [scope]" -- the search_symbols tool schema rejects the `scope` argument passed by the HTTP smoke test.
**Impact:** One integration test fails; not caused by 06-04 changes.
