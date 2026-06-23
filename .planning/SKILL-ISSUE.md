# SKILL.md Decision Matrix Review

**Date:** 2024-06-23  
**Reviewer:** Analysis of `internal/cli/skills/helix/SKILL.md` vs `internal/cli/verbs_gen.go`

## Executive Summary

The SKILL.md decision matrix has **37 rows** covering **50 verbs**. Several rows incorrectly group verbs with different semantic purposes (actions vs queries), and many rows lack "Not this" guidance. Additionally, `reference.md` has copy-paste errors in the "Use this, not that" section for some verbs.

---

## Findings

### 1. Verb Coverage

- **Total verbs in `verbs_gen.go`:** 50
- **Verbs in SKILL.md decision matrix:** 50 (all covered)
- **Rows in decision matrix:** 37 (some rows combine multiple verbs)

**All 50 verbs are present in both files.** No missing verbs.

---

### 2. Semantic Grouping Issues

Several rows combine verbs that have fundamentally different purposes (ACTION vs QUERY):

| Row | Current Verbs | Issues |
|-----|---------------|--------|
| Row 68 | `get-semantic-graph-status` / `index-semantic-graph` / `refresh-semantic-graph` | `get-semantic-graph-status` is a QUERY (reads state); `index-semantic-graph` and `refresh-semantic-graph` are ACTIONS (mutate state) |
| Row 70 | `read-memory` / `write-memory` / `list-memories` | `read-memory` and `list-memories` are QUERIES; `write-memory` is an ACTION |
| Row 71 | `search-memories` / `rename-memory` / `edit-memory` / `delete-memory` | `search-memories` is a QUERY; `rename-memory`, `edit-memory`, `delete-memory` are ACTIONS |
| Row 72 | `onboard-project` / `prepare-for-new-conversation` | Unrelated purposes: onboarding introduces a project; handoff summarizes a session |
| Row 73 | `switch-mode` / `get-token-budget` | `switch-mode` is an ACTION; `get-token-budget` is a QUERY |

**Problem:** Grouping actions with queries under one "Question" confuses the user about what the verbs actually do. An LLM reading this may not distinguish when to use a query vs when to use an action.

---

### 3. Missing "Not this" Guidance

Rows with `—` in the "Not this" column provide no alternative guidance:

| Row | Verbs | Current "Not this" |
|-----|-------|-------------------|
| Row 68 | `get-semantic-graph-status` / `index-semantic-graph` / `refresh-semantic-graph` | `—` |
| Row 69 | `validate-graph-edge` | `—` |
| Row 70 | `read-memory` / `write-memory` / `list-memories` | `scratch files` |
| Row 71 | `search-memories` / `rename-memory` / `edit-memory` / `delete-memory` | `—` |
| Row 72 | `onboard-project` / `prepare-for-new-conversation` | `—` |
| Row 73 | `switch-mode` / `get-token-budget` | `—` |
| Row 74 | `get-health` | `guess` |
| Row 75 | `get-tool-help` | `—` |

**Problem:** The "Not this" column is meant to steer agents away from poor alternatives (e.g., `grep` instead of `helix find-references`). Missing entries reduce the decision matrix's utility.

---

### 4. Copy-Paste Errors in reference.md

`reference.md` contains incorrect "Use this, not that" text for several verbs:

| Verb | Current (Wrong) | Should Be |
|------|-----------------|-----------|
| `switch-mode` | "for durable project/session memory instead of ad-hoc scratch notes" | "to change operational constraints instead of editing profile config manually" |
| `get-token-budget` | "for durable project/session memory instead of ad-hoc scratch notes" | "to inspect token allocation instead of guessing from profile files" |
| `delete-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | Wrong — this is for DELETING memory, not creating durable memory |
| `edit-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | Wrong — this is for EDITING memory, not creating it |
| `read-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | Acceptable but could be clearer |
| `list-memories` | "for durable project/session memory instead of ad-hoc scratch notes" | Acceptable but could be clearer |
| `search-memories` | "for durable project/session memory instead of ad-hoc scratch notes" | Acceptable but could be clearer |

**Root cause:** The memory-related verbs all share the same copy-pasted text, but DELETE and EDIT have the opposite purpose of "durable project/session memory" (they modify/remove it).

---

### 5. Incorrect "Output" in reference.md

Several verbs have the wrong output description:

| Verb | Current Output | Should Be |
|------|----------------|-----------|
| `switch-mode` | "the memory body or a ranked FTS5 search result set, one entry per line" | Mode switch confirmation + profile info |
| `get-token-budget` | "the memory body or a ranked FTS5 search result set, one entry per line" | Token budget breakdown for current mode |
| `delete-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | Deletion confirmation |
| `edit-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | Edit confirmation + diff |
| `rename-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | Rename confirmation |
| `onboard-project` | "the memory body or a ranked FTS5 search result set, one entry per line" | Project analysis + onboarding instructions |
| `prepare-for-new-conversation` | "the memory body or a ranked FTS5 search result set, one entry per line" | Handoff summary (optionally saved as memory) |

**Root cause:** All these verbs share a generic output description that's copy-pasted from memory read tools. Only `read-memory`, `list-memories`, and `search-memories` actually return memory content or FTS5 results.

---

### 6. Missing Prerequisite Notes

Semantic graph tools (rows 63-69) assume the semantic index is built, but the matrix doesn't note this:

- `get-semantic-graph-status` — requires `index-semantic-graph` run first
- `explain-cluster` — requires indexed graph
- `explain-symbol-deep` — requires indexed graph
- `get-change-impact-graph` — requires indexed graph
- `validate-graph-edge` — requires indexed graph
- `find-related-symbols` — requires indexed graph
- `get-semantic-context` — requires indexed graph

**Recommendation:** Add a note row or a "Prerequisites" column.

---

### 7. Grouping by Capability

The SKILL.md footer says:

> "Verbs are grouped by capability (navigation, edit, fileops, diagnostics, repomap, memory)."

But the current matrix ordering doesn't strictly follow capability grouping:

| Rows | Capability | Current Order |
|------|------------|---------------|
| 39-46 | navigation | ✓ correct |
| 47-58 | edit | ✓ correct |
| 59-61 | diagnostics | ✓ correct |
| 62-69 | repomap/semantic | Mixed: `get-repo-map` is repomap, but `get-context` through `validate-graph-edge` are semantic graph |
| 70-75 | memory/workflow | Mixed: memory tools (70-71), onboarding (72), mode switches (73), daemon (74), help (75) |

**Semantics vs Repomap:** The matrix conflates "repomap" tools (`get-repo-map`, `get-context`) with "semantic graph" tools (`get-cluster-map`, `explain-cluster`, `get-semantic-graph-status`, etc.). These are different subsystems.

---

## Proposed Changes

### SKILL.md Decision Matrix

**Before (rows 63-75):**
```markdown
| Context around a symbol/task | `helix get-context` / `helix get-semantic-context` | grep chain |
| Cluster map of the codebase | `helix get-cluster-map` / `helix explain-cluster` | manual grouping |
| Deep explanation of a symbol | `helix explain-symbol-deep` | read every caller |
| Symbols related to one symbol | `helix find-related-symbols` | recursive grep |
| Change-impact graph | `helix get-change-impact-graph` | manual trace |
| Semantic graph status / build | `helix get-semantic-graph-status` / `helix index-semantic-graph` / `helix refresh-semantic-graph` | — |
| Validate a graph edge | `helix validate-graph-edge` | — |
| Project memory (read/write/list) | `helix read-memory` / `helix write-memory` / `helix list-memories` | scratch files |
| Search / rename / edit / delete memory | `helix search-memories` / `helix rename-memory` / `helix edit-memory` / `helix delete-memory` | — |
| Onboard a project / new conversation | `helix onboard-project` / `helix prepare-for-new-conversation` | — |
| Switch profile mode / token budget | `helix switch-mode` / `helix get-token-budget` | — |
| Daemon / LS health | `helix get-health` | guess |
| Full tool help | `helix get-tool-help` | — |
```

**After (split by purpose, add "Not this" guidance):**
```markdown
| Context around a symbol/task | `helix get-context` / `helix get-semantic-context` | grep chain |
| Cluster map of the codebase | `helix get-cluster-map` / `helix explain-cluster` | manual grouping |
| Deep explanation of a symbol | `helix explain-symbol-deep` | read every caller |
| Symbols related to one symbol | `helix find-related-symbols` | recursive grep |
| Change-impact graph | `helix get-change-impact-graph` | manual trace |
| Semantic graph status | `helix get-semantic-graph-status` | — |
| Build / refresh semantic graph | `helix index-semantic-graph` / `helix refresh-semantic-graph` | manual indexing |
| Validate a graph edge | `helix validate-graph-edge` | manual trace |
| Read project memory | `helix read-memory` / `helix list-memories` | scratch files |
| Write project memory | `helix write-memory` | ad-hoc notes |
| Search memory | `helix search-memories` | grep notes |
| Edit / rename / delete memory | `helix edit-memory` / `helix rename-memory` / `helix delete-memory` | manual file edits |
| Onboard a new project | `helix onboard-project` | manual exploration |
| Prepare session handoff | `helix prepare-for-new-conversation` | ad-hoc summaries |
| Switch operational mode | `helix switch-mode` | manual config edit |
| View token budget | `helix get-token-budget` | mental math |
| Daemon / LS health | `helix get-health` | guess |
| Full tool help | `helix get-tool-help` | — |
```

**Changes:**
1. Split row 68 into 2 rows: status (query) vs index/refresh (action)
2. Split row 70 into 2 rows: read/list (query) vs write (action)
3. Split row 71 into 2 rows: search (query) vs edit/rename/delete (action)
4. Split row 72 into 2 rows: onboard vs handoff (different purposes)
5. Split row 73 into 2 rows: switch-mode (action) vs token-budget (query)
6. Add "Not this" guidance where missing
7. **Note:** Row count increases from 37 to 44 rows

---

### reference.md "Use this, not that" Fixes

**Memory verbs:**

| Verb | Current (Wrong) | Proposed |
|------|-----------------|----------|
| `read-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | "to recall persisted project knowledge instead of re-reading source files" |
| `write-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | "to persist project knowledge for future sessions instead of ad-hoc notes" |
| `list-memories` | "for durable project/session memory instead of ad-hoc scratch notes" | "to enumerate saved memories instead of searching filesystem manually" |
| `search-memories` | "for durable project/session memory instead of ad-hoc scratch notes" | "to query persisted knowledge instead of grep through notes" |
| `edit-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | "to modify persisted knowledge instead of editing memory files manually" |
| `rename-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | "to rename memory files instead of manual file operations" |
| `delete-memory` | "for durable project/session memory instead of ad-hoc scratch notes" | "to remove obsolete memories instead of manual file deletion" |

**Workflow verbs:**

| Verb | Current (Wrong) | Proposed |
|------|-----------------|----------|
| `onboard-project` | "for durable project/session memory instead of ad-hoc scratch notes" | "to get an onboarding guide for the current project instead of manual exploration" |
| `prepare-for-new-conversation` | "for durable project/session memory instead of ad-hoc scratch notes" | "to create a session handoff summary instead of manual context collection" |

**Session verbs:**

| Verb | Current (Wrong) | Proposed |
|------|-----------------|----------|
| `switch-mode` | "for durable project/session memory instead of ad-hoc scratch notes" | "to switch operational constraints (read/edit/review/admin) instead of editing profile config manually" |
| `get-token-budget` | "for durable project/session memory instead of ad-hoc scratch notes" | "to inspect token allocation for the current profile/mode instead of calculating manually" |

---

### reference.md "Output" Fixes

**Memory verbs:**

| Verb | Current (Wrong) | Proposed |
|------|-----------------|----------|
| `read-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | ✓ Correct — returns memory content |
| `write-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | "confirmation + memory name" |
| `list-memories` | "the memory body or a ranked FTS5 search result set, one entry per line" | ✓ Correct — returns FTS5 results |
| `search-memories` | "the memory body or a ranked FTS5 search result set, one entry per line" | ✓ Correct — returns FTS5 results |
| `edit-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | "edit confirmation + diff" |
| `rename-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | "rename confirmation" |
| `delete-memory` | "the memory body or a ranked FTS5 search result set, one entry per line" | "deletion confirmation" |

**Workflow verbs:**

| Verb | Current (Wrong) | Proposed |
|------|-----------------|----------|
| `onboard-project` | "the memory body or a ranked FTS5 search result set, one entry per line" | "project analysis + onboarding instructions" |
| `prepare-for-new-conversation` | "the memory body or a ranked FTS5 search result set, one entry per line" | "handoff summary (optionally saved as memory)" |

**Session verbs:**

| Verb | Current (Wrong) | Proposed |
|------|-----------------|----------|
| `switch-mode` | "the memory body or a ranked FTS5 search result set, one entry per line" | "mode switch confirmation + profile info" |
| `get-token-budget` | "the memory body or a ranked FTS5 search result set, one entry per line" | "token budget breakdown for current mode" |

---

### Output Description Pattern

The following verbs all share the same incorrect output description:

```markdown
**Output:** the memory body or a ranked FTS5 search result set, one entry per line.
```

This is only correct for memory QUERY verbs (`read-memory`, `list-memories`, `search-memories`). All other verbs should have specific output descriptions.

**Affected verbs:**
- `delete-memory`
- `edit-memory`
- `explain-cluster`
- `explain-symbol-deep`
- `find-related-symbols`
- `get-change-impact-graph`
- `get-cluster-map`
- `get-context`
- `get-health`
- `get-semantic-context`
- `get-semantic-graph-status`
- `get-token-budget`
- `index-semantic-graph`
- `list-memories`
- `onboard-project`
- `prepare-for-new-conversation`
- `read-memory`
- `refresh-semantic-graph`
- `rename-memory`
- `search-memories`
- `switch-mode`
- `write-memory`

---

## Impact

### Row Count Change

| Before | After |
|--------|-------|
| 37 rows | 44 rows |

**New rows added:** 7 (from splitting semantically mixed groups)

### Token Count Change

| Metric | Before | After | Delta |
|--------|--------|-------|-------|
| SKILL.md frontmatter description | 599 bytes | ~650 bytes | +51 bytes |
| Decision matrix rows | 37 | 44 | +7 rows |

The idle-cost cap (1,536 chars) is still respected — frontmatter description is well under the limit.

### reference.md Token Count

| Metric | Count |
|--------|-------|
| Total verbs with copy-paste errors | 13 |
| Total "Output" fields to fix | 22 |
| Estimated change | Minor (already 975 lines) |

---

## Recommendations

1. **Split semantically mixed rows** — Each row should answer ONE question, not combine actions with queries
2. **Add "Not this" guidance** — Every row should have actionable alternative guidance
3. **Fix reference.md copy-paste errors** — Regenerate with correct per-verb text
4. **Add prerequisite notes** — Semantic graph tools should note they require `index-semantic-graph`
5. **Group by capability properly** — Separate repomap tools from semantic graph tools

---

## Files to Update

| File | Changes Required |
|------|-----------------|
| `internal/cli/skills/helix/SKILL.md` | Split 5 rows into 12, add "Not this" guidance |
| `internal/cli/skills/helix/reference.md` | Fix 13 "Use this, not that" entries, fix 22 "Output" descriptions |
| `cmd/helix-refgen/render.go` | Update generator to emit correct per-verb descriptions |

---

## Appendix: All 50 Verbs Categorized by Purpose

### Navigation (QUERY)
- `go-to-definition`
- `find-references`
- `find-implementations`
- `get-type-hierarchy`
- `get-hover-info`
- `get-symbol-overview`
- `analyze-blast-radius`
- `search-symbols`
- `get-call-hierarchy`

### File Operations (ACTION/QUERY mix)
- `find-files` — QUERY
- `list-directory` — QUERY
- `read-file` — QUERY
- `create-file` — ACTION
- `search-in-files` — QUERY
- `replace-in-file` — ACTION

### Edit (ACTION)
- `rename-symbol`
- `replace-symbol-body`
- `fuzzy-edit`
- `insert-before-symbol`
- `insert-after-symbol`
- `safe-delete-symbol`
- `verify-edit`

### Diagnostics (QUERY)
- `get-diagnostics`
- `get-code-actions`
- `format-code` — ACTION

### Repomap (QUERY)
- `get-repo-map`
- `get-context`

### Semantic Graph (QUERY or ACTION)
- `get-semantic-context` — QUERY
- `get-cluster-map` — QUERY
- `explain-cluster` — QUERY
- `explain-symbol-deep` — QUERY
- `find-related-symbols` — QUERY
- `get-change-impact-graph` — QUERY
- `get-semantic-graph-status` — QUERY
- `index-semantic-graph` — ACTION
- `refresh-semantic-graph` — ACTION
- `validate-graph-edge` — QUERY

### Memory (QUERY or ACTION)
- `read-memory` — QUERY
- `write-memory` — ACTION
- `list-memories` — QUERY
- `search-memories` — QUERY
- `edit-memory` — ACTION
- `rename-memory` — ACTION
- `delete-memory` — ACTION

### Workflow (ACTION or QUERY mix)
- `onboard-project` — QUERY (returns analysis)
- `prepare-for-new-conversation` — ACTION (generates handoff)

### Session (QUERY or ACTION)
- `switch-mode` — ACTION
- `get-token-budget` — QUERY
- `get-health` — QUERY
- `get-tool-help` — QUERY