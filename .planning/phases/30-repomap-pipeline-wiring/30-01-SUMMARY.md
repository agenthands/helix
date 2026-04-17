---
phase: 30-repomap-pipeline-wiring
plan: 01
subsystem: repomap-skill
tags: [repomap, pipeline, wiring, daemon, lsp-enrichment]
dependency_graph:
  requires: [27-repomap-tag-extraction-cache, 28-repomap-graph-renderer]
  provides: [wired-repomap-pipeline, daemon-workspace-propagation, lsp-enrichment-callback]
  affects: [internal/skill/repomap, internal/daemon, internal/repomap]
tech_stack:
  added: []
  patterns: [lazy-cache-population, workspace-walk-with-skipDirs, enrichFn-callback-pattern]
key_files:
  created: []
  modified:
    - internal/skill/repomap/skill.go
    - internal/skill/repomap/skill_test.go
    - internal/daemon/daemon.go
    - internal/repomap/render.go
    - internal/repomap/render_test.go
decisions:
  - Used enrichFn closure pattern (not ReferenceProvider interface) for daemon-to-skill LSP enrichment to avoid skill layer knowing lspool types
  - Single lease for all enrichment files (not per-file lease acquisition) to avoid lease contention
  - Symlink skip in walkAndExtract (T-30-02 mitigation) using d.Type()&fs.ModeSymlink check
metrics:
  duration: 366s
  completed: 2026-04-17T20:08:26Z
  tasks: 3/3
  files: 5
---

# Phase 30 Plan 01: RepoMap Pipeline Wiring Summary

Full data pipeline wiring connecting TagExtractor, TagCache, FileGraph, EnrichFromLSP, and TreeRenderer into RepoMapSkill execution path, with daemon workspace root and LSP enrichment callbacks.

## Task Completion

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Wire TagExtractor, ensureCache, SetWorkspaceRoot into RepoMapSkill | 4b1747df | internal/skill/repomap/skill.go, internal/repomap/render.go, internal/repomap/render_test.go |
| 2 | Wire daemon to call SetWorkspaceRoot and SetEnrichFn on RepoMapSkill | 2bdd2d37 | internal/daemon/daemon.go |
| 3 | Update tests to work with new skill structure | edfaddd3 | internal/skill/repomap/skill_test.go |

## Changes Made

### Task 1: RepoMapSkill Pipeline Wiring
- Added `extractor`, `renderer`, `cachePopulated`, `enrichFn` fields to RepoMapSkill struct
- Created TagExtractor in Init() (non-fatal if grammar issues)
- Added SetWorkspaceRoot() method that invalidates cache and renderer
- Added SetEnrichFn() for optional LSP enrichment callback
- Added GetRepoMapSkill() retrieval function (follows GetProfileSkill pattern)
- Added ensureCache() with lazy workspace walk via walkAndExtract()
- walkAndExtract uses skipDirs, LangFromExt filtering, symlink skip (T-30-02)
- Replaced inline renderBudgeted/renderTree/elideFile with TreeRenderer.RenderBudgeted delegation
- Deleted all dead inline rendering code from skill.go
- Exported LangFromExt in render.go (was langFromExt)

### Task 2: Daemon Wiring
- Added repomapSkill and repomapPkg imports to daemon.go
- After profile skill wiring (step 12b), wires enrichFn closure capturing kernel pool
- Inside SetActivateCallback, calls rs.SetWorkspaceRoot(repoPath) on workspace activation
- Added enrichRepoMapFromLSP helper function: acquires single lease, iterates graph files, queries textDocument/references, calls graph.EnrichFromLSP with converted locations
- 5-second timeout, skips silently if no warm LSP session available

### Task 3: Test Updates
- Set renderer and cachePopulated in newTestSkill for populated case
- Fixed EmptyCache test for new ensureCache behavior
- Added TestRepoMapSkill_SetWorkspaceRoot verifying cache invalidation
- Added TestRepoMapSkill_GetRepoMapSkillNil verifying skill retrieval
- Assert extractor is created in Init test
- All 11 tests pass

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] render_test.go referenced unexported langFromExt**
- **Found during:** Task 1
- **Issue:** After exporting langFromExt to LangFromExt in render.go, the test file render_test.go still called the old unexported name
- **Fix:** Updated render_test.go to call LangFromExt
- **Files modified:** internal/repomap/render_test.go
- **Commit:** 4b1747df

**2. [Rule 2 - Security] Symlink escape prevention (T-30-02)**
- **Found during:** Task 1
- **Issue:** Plan's threat model identified T-30-02 (symlink escape in walkAndExtract)
- **Fix:** Added `d.Type()&fs.ModeSymlink != 0` check to skip symlinks
- **Files modified:** internal/skill/repomap/skill.go
- **Commit:** 4b1747df

**3. [Rule 3 - Blocking] Pool.ReleaseLease instead of lease.Release**
- **Found during:** Task 2
- **Issue:** Plan assumed lease.Release() method exists, but lspool uses Pool.ReleaseLease(sessionID)
- **Fix:** Used k.Pool().ReleaseLease(sessionID) with defer in enrichRepoMapFromLSP
- **Files modified:** internal/daemon/daemon.go
- **Commit:** 2bdd2d37

## Verification Results

- `go build ./cmd/serena` -- exits 0
- `go test ./internal/skill/repomap/ -v -count=1` -- 11/11 tests pass
- `go test ./internal/repomap/... -v -count=1` -- all tests pass
- `go vet ./...` -- no warnings
