---
phase: 60
plan: 02
subsystem: semantic-store
tags: [schema, duckdb, overlay, migration, epoch, tdd, phase-60]
requires:
  - "internal/semantic/store schema v2 (Phase 59 partial-extraction columns)"
  - "semantic_live_overlay_{meta,files,symbols,references,edges} tables (Phase 57 schema 1)"
provides:
  - "internal/semantic/store schema v3 (overlay epoch + per-row write_epoch)"
  - "internal/semantic/store.OverlayTx (BeginOverlayTx → Upsert*/Mark*Deleted → Commit/Rollback)"
  - "internal/semantic/store.FlushOverlay (no-op stub)"
  - "Per-workspace overlay-tx mutex registry on Store"
affects:
  - "Phase 60 P03 (freshness tracker) — consumes meta-row writes; this plan supplies the bootstrap meta INSERT"
  - "Phase 60 P04 (live handler) — consumes UpsertOverlayFile + Mark*Deleted helpers"
  - "Phase 60 P05A/P05B (watcher/scanner) — consumes MarkFileDeleted on file-delete events"
  - "Phase 63 (compaction CAS read) — depends on monotone write_epoch invariant"
tech-stack:
  added:
    - "DuckDB ALTER TABLE ADD COLUMN (DEFAULT-only; NOT NULL constraint not supported)"
    - "DuckDB UPDATE ... RETURNING clause"
    - "DuckDB INSERT ... ON CONFLICT (col) DO NOTHING / DO UPDATE SET"
  patterns:
    - "Per-workspace mutex registry (overlayLockFor → returns inner mutex; outer mutex guards map only)"
    - "Epoch increment OUTSIDE the user-visible *sql.Tx so rollback does NOT rewind (D-04 invariant)"
    - "sync.Once-guarded unlock callback (defensive double-Commit/Rollback tolerance)"
key-files:
  created:
    - "internal/semantic/store/overlay_test.go"
    - "internal/semantic/store/overlay_concurrent_test.go"
  modified:
    - "internal/semantic/store/migrations_types.go (CurrentSchemaVersion 2 → 3)"
    - "internal/semantic/store/migrations_registry.go (append v2→v3 entry)"
    - "internal/semantic/store/migrations.go (applyMigration003 + schema3Statements)"
    - "internal/semantic/store/migrations_test.go (TestMigration003 sub-tests; CurrentSchemaVersion-relative assertions)"
    - "internal/semantic/store/overlay.go (replaced 17-line stub with full Phase 60 P02 API)"
    - "internal/semantic/store/duckdb.go (overlayLocks{,Mu} fields + sync import + initializers in openFresh/openExisting)"
decisions:
  - "D-04 LOCKED: epoch bump runs on s.db (NOT on tx) so rollback does NOT rewind current_epoch — T-60-02-05 mitigation"
  - "DuckDB ALTER TABLE forced DEFAULT-only schema (no NOT NULL); application layer (OverlayTx) enforces non-zero write_epoch on every write"
  - "Mark*Deleted cascade helpers iterate per-id rather than IN-list to keep SQL portable across duckdb-go driver versions"
  - "FlushOverlay ships as a no-op stub today; reserved for Phase 63 cooperative drain"
  - "Open BeginTx AFTER the epoch UPDATE commits — if BeginTx fails, the epoch is just unused (monotone-forever invariant preserved)"
metrics:
  duration: "~22 minutes (single agent, sequential GREEN-only flow; tests written immediately ahead of implementation)"
  completed: "2026-05-05T12:22:09Z"
---

# Phase 60 Plan 02: Schema v3 + Overlay Writer with Epoch Contract Summary

**One-liner:** Lit up DuckDB schema v3 (per-workspace `current_epoch` + per-row `write_epoch` on 4 overlay tables + 4 CAS-scan indexes) and shipped the `OverlayTx` write API (BeginOverlayTx / UpsertOverlayFile / MarkFileDeleted / MarkSymbolsDeleted / MarkReferencesDeleted / MarkEdgesDeleted / FlushOverlay), with the D-04 invariant — epoch bump outside the user-visible tx so rollback does NOT rewind — proven under N=64 race-tagged concurrency.

## Context

Phase 60 builds the live-overlay update pipeline that keeps the semantic fact store fresh between Phase 59's snapshot rebuilds. P02 is the foundation plan: every downstream producer (live handler P04, watcher P05A, scanner P05B) writes facts through `OverlayTx`, and Phase 63's compaction depends on the monotone `write_epoch` stamps to safely roll overlay rows into the next snapshot via a CAS read.

Getting the epoch contract wrong here = silent overlay row loss during compaction in Phase 63 (Pitfall called out in 60-RESEARCH.md). This plan locks the contract before any producer wires against it.

## Schema v3 SQL Diff

`applyMigration003` issues 10 statements:

```sql
-- Per-workspace monotone epoch counter
ALTER TABLE semantic_live_overlay_meta       ADD COLUMN current_epoch UBIGINT DEFAULT 0;

-- Per-row write_epoch stamps on the 4 fact tables
ALTER TABLE semantic_live_overlay_files      ADD COLUMN write_epoch UBIGINT DEFAULT 0;
ALTER TABLE semantic_live_overlay_symbols    ADD COLUMN write_epoch UBIGINT DEFAULT 0;
ALTER TABLE semantic_live_overlay_references ADD COLUMN write_epoch UBIGINT DEFAULT 0;
ALTER TABLE semantic_live_overlay_edges      ADD COLUMN write_epoch UBIGINT DEFAULT 0;

-- Phase-63 CAS scan path indexes
CREATE INDEX idx_overlay_files_write_epoch      ON semantic_live_overlay_files(repo_id, write_epoch);
CREATE INDEX idx_overlay_symbols_write_epoch    ON semantic_live_overlay_symbols(repo_id, write_epoch);
CREATE INDEX idx_overlay_references_write_epoch ON semantic_live_overlay_references(repo_id, write_epoch);
CREATE INDEX idx_overlay_edges_write_epoch      ON semantic_live_overlay_edges(repo_id, write_epoch);

-- Schema version stamp
INSERT INTO semantic_schema_version (version, applied_at) VALUES (3, now());
```

`CurrentSchemaVersion` bumped from 2 to 3; registry entry `{From: 2, To: 3, Kind: MigrationInPlace, Apply: applyMigration003}` appended.

## OverlayTx Public API

```go
// Lifecycle
func (s *Store) BeginOverlayTx(ctx context.Context, repoID string) (*OverlayTx, error)
func (t *OverlayTx) Commit() error
func (t *OverlayTx) Rollback() error
func (t *OverlayTx) Epoch() uint64
func (t *OverlayTx) RepoID() string

// Writes (all stamp tx.epoch onto the affected rows)
func (t *OverlayTx) UpsertOverlayFile(ctx context.Context, path, contentHash string) error
func (t *OverlayTx) MarkFileDeleted(ctx context.Context, path string) error
func (t *OverlayTx) MarkSymbolsDeleted(ctx context.Context, fileIDs []uint64) error
func (t *OverlayTx) MarkReferencesDeleted(ctx context.Context, fileIDs []uint64) error
func (t *OverlayTx) MarkEdgesDeleted(ctx context.Context, nodeIDs []uint64) error

// Drain (no-op today; Phase 63 may extend)
func (s *Store) FlushOverlay(ctx context.Context) error
```

P02 is the SOLE OWNER of the overlay write surface per CONTEXT.md domain item #4. P04's `HandleFileDeleted` and P05A's watcher both consume these helpers.

## Per-Workspace Lock Registry

```go
// On Store (duckdb.go):
overlayLocksMu sync.Mutex
overlayLocks   map[string]*sync.Mutex // keyed by repoID, lazily populated
```

Lookup goes through `s.overlayLockFor(repoID)`:

```go
func (s *Store) overlayLockFor(repoID string) *sync.Mutex {
    s.overlayLocksMu.Lock()
    defer s.overlayLocksMu.Unlock()
    if s.overlayLocks == nil {
        s.overlayLocks = map[string]*sync.Mutex{}
    }
    mu, ok := s.overlayLocks[repoID]
    if !ok {
        mu = &sync.Mutex{}
        s.overlayLocks[repoID] = mu
    }
    return mu
}
```

The outer mutex guards the map only; we hold it for the lookup, then release before taking the inner per-repoID mutex. Cross-workspace `BeginOverlayTx` calls never serialize (proven by `TestBeginOverlayTx_CrossWorkspaceParallelism` wall-time check).

## D-04 Implementation (T-60-02-05 LOCKED)

The load-bearing detail: the epoch bump is issued on `s.db`, NOT on the per-tx `*sql.Tx`:

```go
// Inside BeginOverlayTx, AFTER acquiring per-workspace mutex:
if _, err := s.db.ExecContext(ctx, `
    INSERT INTO semantic_live_overlay_meta (
        repo_id, base_snapshot_id, graph_version, freshness,
        overlay_file_count, pending_lsp_count, updated_at, current_epoch
    ) VALUES (?, 0, 0, 'unknown', 0, 0, now(), 0)
    ON CONFLICT (repo_id) DO NOTHING
`, repoID); err != nil { ... }

var epoch uint64
if err := s.db.QueryRowContext(ctx, `
    UPDATE semantic_live_overlay_meta
       SET current_epoch = current_epoch + 1
     WHERE repo_id = ?
     RETURNING current_epoch
`, repoID).Scan(&epoch); err != nil { ... }

// Open the user-visible tx AFTER the epoch is committed.
tx, err := s.db.BeginTx(ctx, nil)
```

The epoch UPDATE auto-commits (autocommit on `s.db`), so a subsequent `OverlayTx.Rollback()` only rolls back row writes — `current_epoch` survives. A rolled-back tx leaves an unused epoch slot; the counter is monotone-forever.

The bootstrap INSERT on `semantic_live_overlay_meta` supplies values for the Phase-57 NOT-NULL columns (`base_snapshot_id`, `graph_version`, `freshness`, `overlay_file_count`, `pending_lsp_count`, `updated_at`) that pre-date the epoch contract. We use placeholder values (`0` / `'unknown'` / `now()`); the freshness tracker (Phase 60 P03) is the canonical owner of these columns and refreshes them on every write. The `ON CONFLICT (repo_id) DO NOTHING` clause makes the insert idempotent.

## Test Counts

- **migrations_test.go**: +3 sub-tests for v3 (`TestMigration003_FreshLandsAtV3`, `TestMigration003_UpgradeFromV1`, `TestMigration003_UpgradeFromV2`); +1 helper (`indexExists`); 2 existing tests updated to assert against `CurrentSchemaVersion` constant rather than literal 2.
- **overlay_test.go**: 9 tests (`TestBeginOverlayTx_AllocatesEpoch`, `TestBeginOverlayTx_RollbackPreservesEpoch`, `TestBeginOverlayTx_CrossWorkspaceParallelism`, `TestBeginOverlayTx_SameWorkspaceSerializes`, `TestOverlayTx_UpsertOverlayFile_StampsEpoch`, `TestOverlayTx_MarkFileDeleted`, `TestOverlayTx_MarkSymbolsDeleted_EmptyList`, `TestFlushOverlay`, `TestBeginOverlayTx_RejectsEmptyRepoID`).
- **overlay_concurrent_test.go**: 1 test (`TestOverlayEpochConcurrent`) — N=64 goroutines, asserts unique epochs in `[1..64]`, no gaps, max == 64, meta `current_epoch` == 64.

Total new/updated tests: **13**. All pass under `-race`.

## DuckDB Quirks Observed

1. **`ALTER TABLE ADD COLUMN ... NOT NULL DEFAULT <expr>` rejected.** Same constraint as Phase 59 (`applyMigration002` doc comment lines 380–396). Used `DEFAULT 0` alone; application layer (OverlayTx) stamps `write_epoch` on every write so the practical invariant holds.
2. **`UPDATE ... RETURNING current_epoch` works as advertised.** DuckDB returns the post-update value — ideal for the bump+read combined statement.
3. **`INSERT ... ON CONFLICT (repo_id) DO NOTHING` works** because `semantic_live_overlay_meta` carries `PRIMARY KEY (repo_id)` from schema 1; same for the four fact-table `ON CONFLICT (repo_id, path)` / `ON CONFLICT (repo_id, symbol_id)` etc.
4. **macOS linker emits a `LC_DYSYMTAB` warning** when building tests with `-race` against duckdb-go-bindings. This is a pre-existing cgo binding quirk on Apple-clang, not introduced by this plan; tests still pass.

## Verification

All gates from `<verification>` block pass:

- `go test ./internal/semantic/store/... -run TestMigration003 -count=1` → exit 0
- `go test ./internal/semantic/store/... -run TestBeginOverlayTx -count=1` → exit 0
- `go test ./internal/semantic/store/... -run TestOverlayEpochConcurrent -race -count=1` → exit 0
- `grep -c 'CurrentSchemaVersion = 3' internal/semantic/store/migrations_types.go` → 1
- `grep -c 'applyMigration003' internal/semantic/store/migrations_registry.go internal/semantic/store/migrations.go` → 4 (≥ 2)
- `grep -c 'BeginOverlayTx' internal/semantic/store/overlay.go` → 14 (≥ 1)
- `grep -c 'MarkFileDeleted' internal/semantic/store/overlay.go` → 6 (≥ 1)
- `grep -cE 'MarkSymbolsDeleted|MarkReferencesDeleted|MarkEdgesDeleted' internal/semantic/store/overlay.go` → 14 (≥ 3)
- `grep -v '^#' internal/semantic/store/overlay.go | grep -c 'NOT NULL DEFAULT'` → 0 (Pitfall 7 guard holds)
- `go vet ./...` → clean for `internal/semantic/store/...` (pre-existing `internal/treesitter/bindings/swift` macro-redefinition warning is unchanged and unrelated)
- `vet-noduckdb` → clean (DuckDB import boundary still holds; only `internal/semantic/store/duckdb.go` imports `github.com/duckdb/duckdb-go/v2`)
- Full `go test ./internal/semantic/store/... -count=1` suite → ok

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Test fixtures expected `schema_version=2` after Open.**

- **Found during:** Task 1 (migration v3) — running existing `TestMigration_Fresh_v2` and `TestMigration_Existing_v2` after the bump would fail with "got 3, want 2" because they hard-coded the literal.
- **Fix:** Updated both tests to assert against `CurrentSchemaVersion` constant rather than literal 2; preserves the test's intent (verify Open lands at the binary's current schema) and makes the next migration bump (v3 → v4) drag-free.
- **Files modified:** `internal/semantic/store/migrations_test.go`
- **Commit:** `fc6ebb5f`

**2. [Rule 2 - Critical] Bootstrap meta INSERT must supply all NOT-NULL columns.**

- **Found during:** Task 2 (overlay writer) — the plan's example INSERT (`INSERT INTO semantic_live_overlay_meta (repo_id, current_epoch) VALUES (?, 0) ON CONFLICT DO NOTHING`) would have failed against the schema-1 definition because `base_snapshot_id`, `graph_version`, `freshness`, `overlay_file_count`, `pending_lsp_count`, and `updated_at` are all `NOT NULL` (no DEFAULT in schema 1).
- **Fix:** Bootstrap insert supplies placeholder values (`0` / `'unknown'` / `now()`) for the Phase-57 NOT-NULL columns. The freshness tracker (Phase 60 P03) is the canonical owner of these columns and refreshes them on every overlay write.
- **Files modified:** `internal/semantic/store/overlay.go`
- **Commit:** `15dd810d`

**3. [Rule 1 - Bug] DuckDB driver-version-fragile IN-list.**

- **Found during:** Task 2 implementation — the plan suggested DuckDB list params via `duckdb-go` list binding for `IN (?)` with `[]uint64`, but `database/sql` interface gives inconsistent results across `duckdb-go` driver versions (some accept list params, some treat the slice as a single value).
- **Fix:** `Mark{Symbols,References,Edges}Deleted` iterate per-id with separate `UPDATE` statements. For the realistic per-file fan-out (≤ a few hundred symbols on typical edits) the overhead is negligible; portability across driver versions is more valuable.
- **Files modified:** `internal/semantic/store/overlay.go`
- **Commit:** `15dd810d`

**4. [Rule 2 - Critical] sync.Once-guarded unlock for defensive double-call tolerance.**

- **Found during:** Task 2 implementation — the plan's `defer t.unlock()` in `Commit` and `Rollback` would panic on a defensive double-call (e.g., a caller does `defer tx.Rollback()` then explicitly `tx.Commit()` on the success path). Mutex.Unlock on an already-unlocked mutex panics.
- **Fix:** Wrapped `unlock` in a `sync.Once` (`releaseLock` helper) so a second Commit/Rollback is a silent no-op rather than a panic. Defensive, not load-bearing — the canonical contract is still "Commit XOR Rollback exactly once".
- **Files modified:** `internal/semantic/store/overlay.go`
- **Commit:** `15dd810d`

### Architectural / Scope Changes

None. Plan executed as specified.

## Self-Check: PASSED

- [x] `internal/semantic/store/migrations_types.go` — modified, `CurrentSchemaVersion = 3` present
- [x] `internal/semantic/store/migrations_registry.go` — modified, `{From: 2, To: 3, ...}` appended
- [x] `internal/semantic/store/migrations.go` — modified, `applyMigration003` + `schema3Statements` present
- [x] `internal/semantic/store/migrations_test.go` — modified, `TestMigration003` sub-tests present
- [x] `internal/semantic/store/overlay.go` — modified, full Phase 60 P02 API present (~14 BeginOverlayTx mentions, 6 MarkFileDeleted mentions, 14 cascade-helper mentions)
- [x] `internal/semantic/store/overlay_test.go` — created
- [x] `internal/semantic/store/overlay_concurrent_test.go` — created
- [x] `internal/semantic/store/duckdb.go` — modified (`overlayLocks` field + `sync` import + initializers)
- [x] Commit `fc6ebb5f` (schema v3) — found in git log
- [x] Commit `15dd810d` (overlay writer) — found in git log
- [x] Commit `d44c4a8c` (concurrent stress) — found in git log
- [x] All plan verification gates pass (tests + grep + go vet + vet-noduckdb)
