---
phase: 30-repomap-pipeline-wiring
reviewed: 2026-04-17T12:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/daemon/daemon.go
  - internal/repomap/render.go
  - internal/repomap/render_test.go
  - internal/skill/repomap/skill.go
  - internal/skill/repomap/skill_integration_test.go
  - internal/skill/repomap/skill_test.go
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 30: Code Review Report

**Reviewed:** 2026-04-17T12:00:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

The RepoMap pipeline wiring connects the skill layer to the daemon's kernel and LSP pool for enrichment. The code is well-structured with good separation of concerns, proper mutex usage in ensureGraph, and solid test coverage including integration tests. However, there are thread-safety gaps in ensureCache, a potentially incomplete path traversal check, and the LSP enrichment iterates files with position 0:0 which will produce limited results from most language servers.

## Warnings

### WR-01: Race condition in ensureCache -- no mutex protection

**File:** `internal/skill/repomap/skill.go:254-269`
**Issue:** `ensureCache()` reads and writes `s.cachePopulated`, `s.renderer`, and `s.rootDir` without holding `s.mu`. Meanwhile, `SetWorkspaceRoot()` (line 90-96) does hold `s.mu` when writing these same fields. If `ensureCache` is called concurrently with `SetWorkspaceRoot`, the `cachePopulated` check on line 255 and subsequent writes on lines 265-268 can race. Note that `ensureGraph()` correctly holds `s.mu` for its own state, but it calls `ensureCache()` (line 320) *before* acquiring the lock, so ensureCache remains unprotected.
**Fix:** Protect `ensureCache` with `s.mu`, or at minimum protect the `cachePopulated` read/write and `renderer` assignment. Since `walkAndExtract` can be slow, consider a pattern like check-lock-check or a separate `once`-style guard to avoid holding the lock during the walk:
```go
func (s *RepoMapSkill) ensureCache() error {
    s.mu.Lock()
    if s.cachePopulated {
        s.mu.Unlock()
        return nil
    }
    root := s.resolveRoot()
    s.mu.Unlock()

    if root == "" || root == "." {
        return serr.New(serr.Internal, "workspace root not set; call activate_project first")
    }
    if err := s.walkAndExtract(root); err != nil {
        return serr.Wrap(serr.Internal, "walking workspace for tag extraction", err)
    }

    s.mu.Lock()
    defer s.mu.Unlock()
    s.cachePopulated = true
    s.renderer = repomap.NewTreeRenderer(s.elider, s.cache, root)
    return nil
}
```

### WR-02: LSP enrichment queries position 0:0 for every file -- yields minimal results

**File:** `internal/daemon/daemon.go:554-564`
**Issue:** `enrichRepoMapFromLSP` sends `textDocument/references` with `Position{Line: 0, Character: 0}` for every file. Most language servers return references only for the symbol at the given cursor position. Position 0:0 is typically the package declaration (Go) or an import (Python/TS), so this will find cross-file references only for those specific symbols, missing all function/type/variable references in the file. The enrichment will be very sparse.
**Fix:** To get meaningful cross-file references, iterate over the tags already in the FileGraph (which have line/column info) and query LSP references for each defined symbol's position:
```go
for file := range g.Files {
    tags := g.TagsForFile(file) // expose tags from the graph or re-read from cache
    for _, tag := range tags {
        if tag.Kind != repomap.TagDef { continue }
        params := gen.ReferenceParams{
            TextDocumentPositionParams: gen.TextDocumentPositionParams{
                TextDocument: gen.TextDocumentIdentifier{URI: "file://" + file},
                Position:     gen.Position{Line: uint32(tag.Line), Character: uint32(tag.Column)},
            },
            Context: gen.ReferenceContext{IncludeDeclaration: true},
        }
        // ... rest of enrichment
    }
}
```

### WR-03: Path traversal check is bypassable

**File:** `internal/skill/repomap/skill.go:209`
**Issue:** The path traversal check `strings.Contains(fp, "..")` rejects paths containing `..` anywhere, including legitimate directory names like `my..dir`. More importantly, it does not prevent absolute path access -- a user can pass `/etc/passwd` as a seed file to `get_context`. While `get_context` uses these paths for personalization weighting (not file reads), the paths propagate into the graph and could cause confusion or information leaks if the graph later exposes them.
**Fix:** Also reject absolute paths that escape the workspace root:
```go
if strings.Contains(fp, "..") {
    return "", serr.New(serr.InvalidArgs, ...)
}
if filepath.IsAbs(fp) && !strings.HasPrefix(fp, s.resolveRoot()) {
    return "", serr.New(serr.InvalidArgs,
        fmt.Sprintf("file path %q is outside workspace root", fp)).WithTool("get_context")
}
```

### WR-04: RenderBudgeted binary search may select over-budget output

**File:** `internal/repomap/render.go:51-68`
**Issue:** The binary search loop updates `bestOutput` when `pctErr < okErr` (line 58), even when `tokens > budget`. This is intentional (15% tolerance per aider), but the condition `(tokens <= budget && tokens > bestTokens) || pctErr < okErr` can replace a within-budget result with an over-budget result. Consider a scenario: `mid=5` produces 900 tokens (under budget=1000, bestTokens=900), then `mid=7` produces 1100 tokens (10% over budget, pctErr=0.1 < 0.15). The 1100-token result replaces the 900-token result even though the 900-token one was within budget. This may not match the intended behavior of "maximize coverage within budget."
**Fix:** Add a tiebreaker that prefers within-budget results when both candidates are within tolerance:
```go
withinBudget := tokens <= budget
if (withinBudget && tokens > bestTokens) || (pctErr < okErr && tokens > bestTokens) {
    bestOutput = output
    bestTokens = tokens
}
```

## Info

### IN-01: Unused helper function renderFileEntry

**File:** `internal/repomap/render.go:233-246`
**Issue:** `renderFileEntry` is defined but never called. The tree rendering uses `renderNode` instead. This is dead code.
**Fix:** Remove the method or add a test that exercises it if it is intended for future use.

### IN-02: Hardcoded 5-second timeout in enrichRepoMapFromLSP callback

**File:** `internal/daemon/daemon.go:280`
**Issue:** The LSP enrichment callback uses a hardcoded `5*time.Second` timeout. For large repositories with many files in the graph, iterating all files with LSP requests may exceed this. The timeout is reasonable as a safety net but should be documented or made configurable.
**Fix:** Consider extracting as a constant or config value:
```go
const enrichTimeout = 5 * time.Second
```

---

_Reviewed: 2026-04-17T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
