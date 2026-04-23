---
phase: 40-usage-refresh
reviewed: 2026-04-23T12:00:00Z
depth: standard
files_reviewed: 1
files_reviewed_list:
  - USAGE.md
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 40: Code Review Report

**Reviewed:** 2026-04-23T12:00:00Z
**Depth:** standard
**Files Reviewed:** 1
**Status:** issues_found

## Summary

Reviewed USAGE.md after the addition of a Feature Guide section (7 subsections), two troubleshooting entries, and the Tutorial 1 update to use `serena setup claude-code`. The document is well-structured, consistent in tone, and the markdown formatting is clean throughout. Three factual inaccuracies were found in the new Feature Guide section: two incorrect parameter names and one misleading code example.

## Warnings

### WR-01: Incorrect parameter name in RepoMap documentation

**File:** `USAGE.md:153-160`
**Issue:** The Feature Guide documents the `get_repo_map` parameter as `max_tokens` and the `get_context` default budget description implies the same naming. The actual parameter name in the source code (`internal/skill/repomap/skill.go:164,224`) is `token_budget`. Agents following this documentation will pass a parameter that gets silently ignored (or triggers a smart-error suggestion), causing the tool to use its default budget instead of the intended value.
**Fix:**
```markdown
- **`get_repo_map`** returns a structural overview of the repository, ranking symbols by importance. Accepts a `token_budget` parameter (default 4096, max 32768) to control output size.
- **`get_context`** takes a list of files relevant to your current task and returns ranked symbols from across the codebase using Personalized PageRank on the dependency graph. Default budget is 2048 tokens.

```
get_repo_map(token_budget=4096)
get_context(files=["src/api/handler.go", "src/models/user.go"])
```
```

### WR-02: Misleading fuzzy_edit code example uses positional arguments

**File:** `USAGE.md:146-147`
**Issue:** The example shows `fuzzy_edit` called with 5 positional arguments: `fuzzy_edit("src/handler.go", "func HandleRequest(", "...", "func HandleRequest(ctx context.Context,", "...")`. The actual tool (`internal/kernel/fileops/tools.go:57-62`) takes three named parameters: `path`, `search`, and `replacement`. Ellipsis (`...`) segments are embedded inside the `search` and `replacement` strings, not passed as separate arguments. An agent following this example would produce an invalid tool call.
**Fix:**
```markdown
```
fuzzy_edit(path="src/handler.go", search="func HandleRequest(\n...\n)", replacement="func HandleRequest(ctx context.Context,\n...\n)")
```
```

## Info

### IN-01: Strategy name simplification may cause confusion

**File:** `USAGE.md:141`
**Issue:** The third strategy is called "IndentFlex" in the documentation, but the actual strategy constant is `StrategyIndentationFlex` with string value `"indentation_flexible"` (`internal/fuzzy/types.go:21-23`). Since the tool reports the strategy name in its output (`match_strategy: indentation_flexible`), users comparing documentation to tool output may be confused by the different naming.
**Fix:** Consider using "IndentationFlex" or "Indentation-flexible" to more closely match the reported strategy name, or add a note that the reported name is `indentation_flexible`.

---

_Reviewed: 2026-04-23T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
