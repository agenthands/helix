---
phase: 63
plan: 01
subsystem: semantic-store
tags: [duckdb, snapshot, fact-store, semantic-index, compaction, tdd]
dependency-graph:
  requires:
    - "internal/semantic/store/overlay.go (BeginOverlayTx + per-workspace lock pattern)"
    - "internal/semantic/store/migrations.go (Schema 3 column inventory; semantic_snapshots, semantic_files/symbols/references/edges, semantic_live_overlay_*)"
    - "internal/semantic/store/duckdb.go (*Store handle, *sql.DB)"
  provides:
    - "Store.BeginSnapshot / WriteSnapshotFacts / CommitSnapshot / AbortSnapshot"
    - "(*Snapshot).DeleteSnapshotsBeyond — retention atomic with snapshot creation"
    - "(*Snapshot).ClearOverlayLE — overlay drain bounded by captured_epoch, closes Phase 60 D-04 CAS"
    - "SnapshotMeta / Snapshot / Facts / FileFact / SymbolFact / ReferenceFact / EdgeFact / SnapshotSummary types"
    - "fakeCompactor test fixture demonstrating the consumable shape (CONTEXT.md D-02)"
  affects:
    - "Phase 63 P63-02 (compactor) consumes this API directly — no other callers today"
tech-stack:
  added: []
  patterns:
    - "BeginSnapshot mirrors overlay.BeginOverlayTx — *sql.Tx encapsulated on a per-tx handle, terminate via Commit/Abort"
    - "INSERT...RETURNING for snapshot_id allocation (COALESCE(MAX,0)+1) — no LastInsertId() reliance"
    - "database/sql parameterized inserts only — never duckdb-go Appender (RESEARCH.md Landmine 6)"
    - "Per-table cascade DELETE inside the snapshot tx for retention (no FK ON DELETE CASCADE)"
    - "Hardcoded SQL constants per table — no string interpolation of table names"
    - "Forward-compatible hook (resetOverlayPendingRowsIfPresent) reserved for P63-02 counter wiring"
key-files:
  created:
    - "internal/semantic/store/snapshot_test.go (478 LOC, 12 tests)"
    - "internal/semantic/store/snapshot_fake_compactor_test.go (125 LOC, 1 fixture test)"
  modified:
    - "internal/semantic/store/snapshot.go (replaced 16-line stub with 604-LOC implementation)"
decisions:
  - "Captured epoch is carried on the in-memory Snapshot handle (Meta.CapturedEpoch), NOT persisted as a column on semantic_snapshots — Schema 3 has no captured_epoch column and ClearOverlayLE consumes the value at the boundary, so a column add is unnecessary at this layer"
  - "snapshot_id allocated via INSERT...RETURNING with `(SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots)` — DuckDB has no AUTOINCREMENT and no LastInsertId() guarantee for non-sequence PKs"
  - "Hardcoded SQL constants per overlay/cascade table (instead of `\"DELETE FROM \"+t+\" WHERE ...\"`) so the parameterization grep gate stays clean — no string concatenation flows through the SQL boundary even when only a hardcoded table name is being interpolated"
  - "WriteSnapshotFacts has no payload-size cap (T-63-01-03 disposition: accept). The pre-flight overlay-row guard is the caller's job (P63-02). Documented in package doc-comment so reviewers can find the trust boundary"
metrics:
  duration: "~25 minutes (single uninterrupted session)"
  completed: "2026-05-07"
---

# Phase 63 Plan 01: Snapshot-Write API Summary

The empty Phase 59 stub at `internal/semantic/store/snapshot.go` is now the full snapshot-write contract the Phase 63 compactor will consume — Begin/Write/Commit/Abort on `*Store` plus DeleteSnapshotsBeyond/ClearOverlayLE on the per-tx `*Snapshot` handle. No public Tx() accessor; the underlying `*sql.Tx` stays fully encapsulated.

## Outcome

**ok** — both TDD gates passed in order, all 13 unit tests pass, full store package tests pass without regression, vet-noduckdb clean.

## RED

Commit `09a3ece6 test(63-01): add failing tests for snapshot-write API` introduced 12 tests in `snapshot_test.go` and 1 end-to-end fixture in `snapshot_fake_compactor_test.go`:

- `TestBeginSnapshot_NilStore` / `TestBeginSnapshot_EmptyRepoID` — guards
- `TestBeginSnapshot_AllocatesPendingRow` — pending row + abort-vanishes
- `TestSnapshot_BeginWriteCommit` — happy path with 5 files + 50 symbols
- `TestSnapshot_BeginAbortRollsBack` — DuckDB ACID rollback verified
- `TestSnapshot_DoubleCommit` / `TestSnapshot_CommitAfterAbort` — idempotency guards
- `TestSnapshot_DeleteSnapshotsBeyondAtomic` — 7 prior snapshots + new + retain=5; commit keeps 5, abort restores 7
- `TestSnapshot_ClearOverlayLE_DeletesUpToCapturedEpoch` — write_epoch ≤ 3 vanish, ≥ 4 survive
- `TestSnapshot_ClearOverlayLE_AtomicWithCommit` — abort restores cleared overlay rows
- `TestSnapshot_ClearOverlayLE_AfterCommit` — guard that ClearOverlayLE rejects post-commit
- `TestSnapshot_TxAccessor_Absent` — encapsulation invariant via grep on snapshot.go
- `TestFakeCompactor_EndToEnd` — 7 cycles of Begin → Write → ClearOverlayLE → DeleteSnapshotsBeyond → Commit, asserting overlay drain + retention=5

The RED gate landed as a clean compile failure: every test referenced not-yet-existing types (Snapshot, SnapshotMeta, Facts, FileFact, …) and methods (BeginSnapshot, WriteSnapshotFacts, CommitSnapshot, AbortSnapshot, DeleteSnapshotsBeyond, ClearOverlayLE). `go test` exited with `[build failed]` — that compile failure is the RED proof.

## GREEN

Commit `f7f344a3 feat(63-01): implement snapshot-write API for compactor consumption` replaced the 16-line stub with a 604-LOC implementation in `internal/semantic/store/snapshot.go`. Highlights:

- `BeginSnapshot` opens `s.db.BeginTx(ctx, nil)`, runs `INSERT INTO semantic_snapshots ... RETURNING snapshot_id` with `COALESCE(MAX(snapshot_id),0)+1` allocation, and returns the `*Snapshot` handle bound to that tx. Required NOT-NULL columns (repo_root, kind, worktree_hash, schema_version, indexer_version) are filled with sensible placeholders (`''`, `'compact'`, `'phase63'`, `CurrentSchemaVersion`) so the call works against the existing schema without a migration.
- `WriteSnapshotFacts` runs four per-table loops (files / symbols / references / edges), each issuing `INSERT INTO semantic_<table> (...) VALUES (?, ?, ...)` on `snap.tx.ExecContext`. Helper `nullIfEmpty` / `nullIfZero` map zero-values to SQL NULL for nullable columns (signature, partial_reason, owner_symbol_id, …).
- `CommitSnapshot` flips status with `UPDATE ... SET status='committed', committed_at=?` then commits the tx; sentinel-error guards against double-commit and commit-after-abort.
- `AbortSnapshot` slogs at info level then rolls back; guards against double-abort and abort-after-commit.
- `(*Snapshot).DeleteSnapshotsBeyond` materializes the doomed snapshot_id list inside the tx, then runs hardcoded `DELETE FROM semantic_<table> WHERE snapshot_id=?` per cascade table (files, symbols, references, edges, nodes, diagnostics, snapshots). All on `snap.tx` so retention commits or rolls back atomically with the new snapshot's commit.
- `(*Snapshot).ClearOverlayLE` runs four hardcoded `DELETE FROM semantic_live_overlay_<table> WHERE repo_id = ? AND write_epoch <= ?` statements on `snap.tx`. Reset hook `resetOverlayPendingRowsIfPresent` is a no-op today; P63-02 will wire it once the counter field exists.

Final LOC count for snapshot.go: **604 lines** (target was ≥ 230).

Acceptance grep gates all pass:
- `grep -c '^func (s \*Store)' snapshot.go` → 4 (BeginSnapshot, WriteSnapshotFacts, CommitSnapshot, AbortSnapshot)
- `grep -c '^func (snap \*Snapshot)' snapshot.go` → 2 (DeleteSnapshotsBeyond, ClearOverlayLE)
- `grep -cE '^func \(\w+ \*Snapshot\) Tx\(' snapshot.go` → 0 (encapsulation invariant)
- `grep -cE 'duckdb/duckdb-go' snapshot.go` → 0 (vet-noduckdb boundary)
- `grep -nE 'fmt\.Sprintf.*"(INSERT|UPDATE|DELETE)|"(INSERT|UPDATE|DELETE)[^"]*"\s*\+' snapshot.go` → 0 hits (parameterization invariant)
- The only Appender mention is in a comment explicitly stating we do NOT use it (Landmine 6 reference).

Tests results:
- `go test ./internal/semantic/store/... -run 'TestBeginSnapshot|TestSnapshot_|TestFakeCompactor_' -count=1` → ok 1.19s
- `go test ./internal/semantic/store/... -count=1 -timeout 120s` → ok 2.22s (no regression in pre-existing overlay / migration / store_test cases)
- `go vet ./internal/semantic/store/...` → clean
- `go run ./cmd/vet-noduckdb ./internal/semantic/store/...` → clean

## REFACTOR

One mid-cycle deviation: the initial GREEN implementation used `"DELETE FROM "+t+" WHERE ..."` patterns in DeleteSnapshotsBeyond and ClearOverlayLE for the per-table loops. That tripped the parameterization-invariant grep gate (the `+` between literals counts even when only hardcoded table names interpolate). Restructured to a slice of struct literals with hardcoded full statements per table so the grep gate stays clean. No behavior change, no test impact — purely a parameterization-style adjustment to keep the SQL boundary auditable. Tracked as `[Rule 3 - Acceptance Gate] string-concat SQL grep gate`.

## Commits

| Step | Commit | Message |
|---|---|---|
| RED | `09a3ece6` | test(63-01): add failing tests for snapshot-write API |
| GREEN | `f7f344a3` | feat(63-01): implement snapshot-write API for compactor consumption |

The TDD gate sequence (`test(63-01)` → `feat(63-01)`) is verified in `git log --oneline -3`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Schema mismatch] `captured_epoch` column does not exist on `semantic_snapshots`**
- **Found during:** Task 2 (GREEN), pre-implementation schema review
- **Issue:** The plan's BeginSnapshot SQL specified `INSERT INTO semantic_snapshots (..., captured_epoch, ...) VALUES (...)`, but the Schema 1+2+3 column inventory in `migrations.go` shows no `captured_epoch` column. Inserting into a non-existent column would have made every BeginSnapshot call fail at runtime.
- **Fix:** Carry `CapturedEpoch` only on the in-memory `Snapshot.Meta` field. `ClearOverlayLE` consumes it directly — the value never needs to round-trip through DuckDB. No new migration needed; ClearOverlayLE already takes `capturedEpoch` as a parameter.
- **Files modified:** `internal/semantic/store/snapshot.go` (BeginSnapshot SQL omits captured_epoch; doc-comment on `SnapshotMeta.CapturedEpoch` explains the in-memory-only carrier)
- **Commit:** `f7f344a3`

**2. [Rule 3 - Schema completeness] `semantic_snapshots` requires several NOT NULL columns the plan didn't enumerate**
- **Found during:** Task 2 (GREEN), pre-implementation schema review
- **Issue:** The plan specified inserting only `(repo_id, base_snapshot_id, status, captured_epoch, created_at)`, but the schema demands NOT-NULL values for `snapshot_id`, `repo_root`, `kind`, `worktree_hash`, `schema_version`, `indexer_version`, and `partial`. Missing any of those would fail the INSERT.
- **Fix:** Provide minimal-but-valid placeholders (`repo_root=''`, `kind='compact'`, `worktree_hash=''`, `schema_version=CurrentSchemaVersion`, `indexer_version='phase63'`, `partial=false`). P63-02 may extend `SnapshotMeta` later if it needs to set real values; today's tests don't require them.
- **Files modified:** `internal/semantic/store/snapshot.go` (BeginSnapshot SQL)
- **Commit:** `f7f344a3`

**3. [Rule 3 - DuckDB feature] No AUTOINCREMENT or reliable LastInsertId for snapshot_id**
- **Found during:** Task 2 (GREEN), implementation
- **Issue:** The plan suggested `id, err := result.LastInsertId()`. DuckDB's `database/sql` driver does not reliably surface `LastInsertId` for non-sequence primary keys.
- **Fix:** Use `INSERT ... RETURNING snapshot_id` with `(SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots)` for the value. Mirrors how `BumpGraphVersion` in overlay.go already uses RETURNING.
- **Files modified:** `internal/semantic/store/snapshot.go` (BeginSnapshot)
- **Commit:** `f7f344a3`

**4. [Rule 3 - Acceptance Gate] String-concatenation pattern tripped the SQL parameterization grep gate**
- **Found during:** Task 2 (GREEN), post-implementation grep verification
- **Issue:** Initial implementation used `"DELETE FROM "+t+" WHERE snapshot_id=?"` with a per-table loop. The grep gate `'(INSERT|UPDATE|DELETE)[^"]*"\s*\+'` matched even though only a hardcoded table-name constant was being interpolated (no caller-derived data flows into the SQL).
- **Fix:** Restructured to a slice of `{name, sql}` struct literals with hardcoded full statements per cascade/overlay table. No behavior change.
- **Files modified:** `internal/semantic/store/snapshot.go` (DeleteSnapshotsBeyond and ClearOverlayLE)
- **Commit:** `f7f344a3`

No architectural deviations (Rule 4) were required.

## Authentication Gates

None — this is a daemon-internal data-plane change with no MCP / network surface.

## Requirements Addressed

- **COMPACT-01** (partial): The `BeginSnapshot → WriteSnapshotFacts → CommitSnapshot` primitive that lets the compactor materialize a new committed snapshot in a single tx is in place. The Phase 63 compactor goroutine that orchestrates the full base+overlay merge lands in P63-02.
- **COMPACT-04** (partial): The `DeleteSnapshotsBeyond` retention primitive runs DELETE inside the same `*Snapshot` tx so retention commits atomically with the new snapshot's commit (and rolls back together with abort). The compactor schedule that calls it lands in P63-02.

Full COMPACT-01..05 closure happens in P63-02.

## Handoff to P63-02

The snapshot-write API surface is ready for the compactor:

- **Begin on `*Store`:** `snap, err := store.BeginSnapshot(ctx, SnapshotMeta{RepoID, BaseSnapshotID, CapturedEpoch})`
- **Per-tx handle methods on `*Snapshot`:**
  - `store.WriteSnapshotFacts(ctx, snap, Facts{Files, Symbols, References, Edges})` — bulk insert
  - `snap.ClearOverlayLE(ctx, repoID, snap.Meta.CapturedEpoch)` — overlay drain bounded by captured epoch, atomic with the snapshot commit/abort
  - `snap.DeleteSnapshotsBeyond(ctx, retain)` — retention DELETE inside the snapshot tx
- **Terminate via:** `store.CommitSnapshot(ctx, snap, summary)` or `store.AbortSnapshot(ctx, snap, reason)` — both idempotency-guarded
- **Encapsulation:** there is NO public `Tx()` accessor on `*Snapshot`. P63-02 calls `snap.ClearOverlayLE(...)` to drain overlay rows; it never touches the underlying `*sql.Tx` directly.
- **Pre-flight overlay-size guard:** P63-01 trusts the caller (P63-02) to apply the `OverlayRowCount` guard before invoking `BeginSnapshot`; T-63-01-03 disposition is `accept` per the plan's threat register.
- **Forward-compatible counter hook:** `resetOverlayPendingRowsIfPresent(snap.store, repoID)` is a no-op today and reserved for P63-02's `overlayPendingRowsFor(repoID) *atomic.Int64` reset. P63-02 redeclares the helper (or inlines the reset) once the counter field exists.

The `fakeCompactor` test fixture in `snapshot_fake_compactor_test.go` demonstrates the full consumption shape end-to-end and serves as the reference implementation for P63-02's compactor goroutine.

## TDD Gate Compliance

- RED commit (`test(63-01): ...`): present
- GREEN commit (`feat(63-01): ...`): present
- Order: RED → GREEN, verified via `git log --oneline -3`
- REFACTOR: not separately committed — the parameterization restructure was folded into the GREEN commit because it was a precondition for the acceptance grep gate to pass

## Self-Check: PASSED

- `internal/semantic/store/snapshot.go` exists (604 LOC)
- `internal/semantic/store/snapshot_test.go` exists (478 LOC, 12 test functions)
- `internal/semantic/store/snapshot_fake_compactor_test.go` exists (125 LOC, 1 test + fakeCompactor)
- Commits `09a3ece6` (RED) and `f7f344a3` (GREEN) present in `git log`
- `go test ./internal/semantic/store/... -count=1 -timeout 120s` exits 0
- `go vet ./internal/semantic/store/...` exits 0
- `go run ./cmd/vet-noduckdb ./internal/semantic/store/...` exits 0
