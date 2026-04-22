---
phase: 37-smart-error-responses
reviewed: 2026-04-22T14:30:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/mcp/suggest_lev.go
  - internal/mcp/suggest.go
  - internal/mcp/suggest_test.go
  - internal/mcp/server.go
  - internal/daemon/daemon.go
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 37: Code Review Report

**Reviewed:** 2026-04-22T14:30:00Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 37 adds "did you mean?" suggestion middleware for MCP tool errors -- enriching protocol errors (unknown parameters) and tool errors (bad enum values) with Levenshtein-distance-based corrections. The implementation is clean, well-structured, and thoroughly tested.

Two warnings relate to potential data races with shared mutable state in the daemon wiring and an inconsistency between `AddTool`/`RemoveTool` in server.go. Two info items flag code duplication and a dead variable in tests. No critical issues found.

## Warnings

### WR-01: Data race on shared activeWSKey/activeWSLang variables

**File:** `internal/daemon/daemon.go:223-226, 331-345`
**Issue:** `activeWSKey` and `activeWSLang` are closed-over local variables shared between multiple closures: the `wsKeyFn`/`workspaceRootFn` read closures (passed to tool registration, invoked on every tool call) and the `activateCallback` write closure (invoked when `activate_project` is called). With concurrent MCP sessions via Streamable HTTP, these reads and writes can race. Go's race detector would flag this under concurrent load.
**Fix:** Protect the shared state with a `sync.RWMutex` or use `atomic.Value`:
```go
type activeWS struct {
    mu   sync.RWMutex
    key  workspace.WorkspaceKey
    lang string
}

var aws activeWS
wsKeyFn := func() workspace.WorkspaceKey {
    aws.mu.RLock()
    defer aws.mu.RUnlock()
    return aws.key
}
// In activateCallback:
aws.mu.Lock()
aws.key = workspace.WorkspaceKey{RepoRoot: repoPath}
aws.lang = activeWSLang
aws.mu.Unlock()
```

### WR-02: RemoveTool does not remove from toolSchemas slice

**File:** `internal/mcp/server.go:178-181`
**Issue:** `RemoveTool` calls `s.sdk.RemoveTools(name)` and `s.registry.Unregister(name)` but does not remove the tool from `s.toolSchemas`. While the suggestion `ToolSchemaMap` is built once at startup and not rebuilt, the `toolSchemas` slice becomes inconsistent with the actual registered tools. If `CollectToolSchemas()` is ever called again (e.g., for hot-reload), the schema map would contain stale entries for removed tools, leading to incorrect "did you mean" suggestions referencing parameters from tools that no longer exist.
**Fix:** Either filter `toolSchemas` in `RemoveTool`, or document that `CollectToolSchemas` must only be called once at startup:
```go
func (s *SerenaMCPServer) RemoveTool(name string) {
    s.sdk.RemoveTools(name)
    s.registry.Unregister(name)
    // Remove from toolSchemas to keep suggestion middleware consistent.
    for i, t := range s.toolSchemas {
        if t.Name == name {
            s.toolSchemas = append(s.toolSchemas[:i], s.toolSchemas[i+1:]...)
            break
        }
    }
}
```

## Info

### IN-01: Duplicated bestParamSuggestion and bestValueSuggestion functions

**File:** `internal/mcp/suggest_lev.go:65-107`
**Issue:** `bestParamSuggestion` and `bestValueSuggestion` are identical in logic -- they both find the best match from a list of valid strings using Levenshtein distance with substring bypass. The only difference is naming. This is textbook code duplication.
**Fix:** Consolidate into a single `bestSuggestion(unknown string, candidates []string, maxDistance int) (string, int)` function, and have both exported ForTest wrappers call it. The separate names in the ForTest exports can remain for clarity.

### IN-02: Dead variable in test

**File:** `internal/mcp/suggest_test.go:463-464`
**Issue:** `originalText` is assigned on line 463 but immediately suppressed with `_ = originalText` on line 464. The variable serves no purpose -- the assertion on line 465 checks the SDK error text directly.
**Fix:** Remove the dead variable:
```go
// Remove lines 463-464:
// originalText := "original error message about file_pth"
// _ = originalText
```

---

_Reviewed: 2026-04-22T14:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
