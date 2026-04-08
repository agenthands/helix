# Deferred Items - Phase 06

## Workspace Key Language Routing

**Found during:** 06-01 Task 2
**Severity:** Blocks LS-backed tool testing (search_symbols, go_to_definition, find_references, etc.)
**Description:** `activeWSKey` in `internal/daemon/daemon.go` line 184 is set to `workspace.WorkspaceKey{RepoRoot: repoPath}` without a Language field. The LS worker pool requires `Language` to resolve the correct language server binary. All LS-backed MCP tools fail with "no language server configured for " when the workspace key has an empty language.
**Impact:** Plans 02 and 03 cannot exercise LS-backed symbol tools until this is resolved.
**Suggested fix:** The daemon's activate callback should either (a) set Language based on detected languages from `rt.Languages()`, or (b) the tool layer should resolve language from the file path being operated on.
