---
phase: 30-repomap-pipeline-wiring
fixed_at: 2026-04-18T12:00:00Z
review_path: .planning/phases/30-repomap-pipeline-wiring/30-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 30: Code Review Fix Report

**Fixed at:** 2026-04-18T12:00:00Z
**Source review:** .planning/phases/30-repomap-pipeline-wiring/30-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: Race condition in ensureCache -- no mutex protection

**Files modified:** `internal/skill/repomap/skill.go`
**Commit:** 804f4491
**Applied fix:** Added check-lock-check pattern to `ensureCache()`. The method now acquires `s.mu` to check `cachePopulated` and read `rootDir` via `resolveRoot()`, releases the lock during the slow `walkAndExtract`, then re-acquires to set `cachePopulated` and `renderer`. This prevents races with concurrent `SetWorkspaceRoot` calls without holding the lock during the filesystem walk.

### WR-02: LSP enrichment queries position 0:0 for every file -- yields minimal results

**Files modified:** `internal/skill/repomap/skill.go`, `internal/daemon/daemon.go`
**Commit:** 804f4491
**Applied fix:** Added `Cache()` accessor to `RepoMapSkill` to expose the tag cache. Updated `enrichRepoMapFromLSP` to accept a `*TagCache` parameter and iterate over `TagDef` tags per file, querying LSP references at each symbol's actual line/column position instead of the fixed 0:0 position. Added `ctx.Err()` check inside the inner loop for early exit on timeout. The daemon callback closure now passes `rs.Cache()` through to the enrichment function.

### WR-03: Path traversal check is bypassable

**Files modified:** `internal/skill/repomap/skill.go`
**Commit:** 804f4491
**Applied fix:** Added an additional check after the existing `..` rejection: absolute paths that do not have the workspace root as a prefix are now rejected with an `InvalidArgs` error. This prevents passing paths like `/etc/passwd` as seed files to `get_context`.

### WR-04: RenderBudgeted binary search may select over-budget output

**Files modified:** `internal/repomap/render.go`
**Commit:** 804f4491
**Applied fix:** Added `withinBudget` variable and modified the selection condition to require `tokens > bestTokens` in both branches. This ensures an over-budget result only replaces a previous result if it also has more tokens (maximizing coverage), preventing a within-budget result from being replaced by a slightly-over-budget result that has fewer tokens. This is a logic fix that requires human verification.

## Verification

- `go vet ./internal/skill/repomap/... ./internal/repomap/... ./internal/daemon/...` -- passed (no output)
- `go test ./internal/repomap/... ./internal/skill/repomap/...` -- passed
- `go build ./cmd/serena` -- passed

---

_Fixed: 2026-04-18T12:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
