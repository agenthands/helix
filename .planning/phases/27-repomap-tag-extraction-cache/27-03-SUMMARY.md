---
phase: 27-repomap-tag-extraction-cache
plan: 03
subsystem: repomap
tags: [sqlite, tag-cache, mtime-invalidation, persistence]
dependency_graph:
  requires: [treesitter.GrammarRegistry, repomap.Tag]
  provides: [repomap.TagCache, repomap.GetOrExtract]
  affects: [internal/daemon]
tech_stack:
  added: []
  patterns: [SQLite WAL mode, mtime-based invalidation, transaction batch insert]
key_files:
  created:
    - internal/repomap/schema.go
    - internal/repomap/cache.go
    - internal/repomap/cache_test.go
  modified: []
decisions:
  - TagCache uses sync.Mutex (not RWMutex) matching memory/index.go pattern since reads and writes both touch SQLite
  - GetOrExtract unlocks mutex during extractFn call to avoid holding lock during potentially slow extraction
  - Batch insert uses prepared statement inside transaction for atomicity and performance
metrics:
  duration: 1m42s
  completed: 2026-04-16
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 0
---

# Phase 27 Plan 03: SQLite Tag Cache Summary

SQLite-backed tag cache with WAL mode, mtime-based invalidation, transaction batch inserts, and parameterized queries surviving daemon restarts.

## Task Results

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | SQLite schema and TagCache implementation | 88b378e8 | internal/repomap/schema.go, internal/repomap/cache.go |
| 2 | Tag cache tests | 4a85e2a5 | internal/repomap/cache_test.go |

## What Was Built

### SQL Schema (internal/repomap/schema.go)
- `file_tags` table with AUTOINCREMENT id, file_path, mtime_ns, name, kind, line, col, start_byte, end_byte
- CHECK constraint on kind column: `IN ('def', 'ref')`
- Two indexes: `idx_file_tags_path` (file_path) and `idx_file_tags_name` (name)

### TagCache (internal/repomap/cache.go)
- `NewTagCache(dbPath)` constructor following `internal/memory/index.go` pattern exactly
- WAL mode (`PRAGMA journal_mode=WAL`) and busy timeout (`PRAGMA busy_timeout=5000`)
- `GetOrExtract(filePath, extractFn)` with mtime-based cache invalidation per D-10
- Unlocks mutex during extraction to avoid holding lock during slow operations
- Batch insert via transaction with prepared statement for atomicity
- `InvalidateFile(filePath)` for manual cache clearing per file
- `Clear()` for full cache wipe
- `Close()` for database connection cleanup
- All SQL uses parameterized queries only (T-27-05 mitigation)

### Tests (internal/repomap/cache_test.go)
- 7 tests covering all cache behaviors:
  - `TestTagCache_NewAndClose`: db file exists on disk after creation
  - `TestTagCache_GetOrExtract_CacheMiss`: 3 tags extracted, counter verified at 1
  - `TestTagCache_GetOrExtract_CacheHit`: second call skips extractFn (counter stays 1)
  - `TestTagCache_GetOrExtract_MtimeInvalidation`: file touch triggers re-extraction with fresh tags
  - `TestTagCache_InvalidateFile`: manual invalidation forces re-extraction
  - `TestTagCache_Clear`: clears all files, both require re-extraction
  - `TestTagCache_Persistence`: close + reopen cache at same dbPath, tags survive (RMAP-03)

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

```
ok  github.com/postfix/serena/internal/repomap   0.270s (7 tests)
go vet: clean
```

## Known Stubs

None - all functionality is fully wired.

## Self-Check: PASSED
