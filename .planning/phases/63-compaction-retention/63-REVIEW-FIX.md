---
phase: 63-compaction-retention
fixed_at: 2026-05-07T19:15:00Z
review_path: .planning/phases/63-compaction-retention/63-REVIEW.md
iteration: 1
findings_in_scope: 14
fixed: 14
skipped: 0
status: all_fixed
---

# Phase 63: Code Review Fix Report

**Fixed at:** 2026-05-07T19:15:00Z
**Source review:** `.planning/phases/63-compaction-retention/63-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 14 (4 critical, 6 warning, 4 info — `fix_scope=all`)
- Fixed: 14
- Skipped: 0
- Validation: `go test ./internal/semantic/store/... ./internal/semantic/compact/... ./internal/obs/...` passes; `go vet ./internal/...` clean (only a pre-existing tree-sitter swift binding macro warning unrelated to this phase).

## Fixed Issues

### CR-01: Compactor `captureEpoch` sentinel destroys CAS contract; deletes rows committed during compaction

**Files modified:** `internal/semantic/store/overlay.go`, `internal/semantic/compact/compactor.go`, `internal/semantic/compact/compactor_test.go`
**Commit:** d3bcc926
**Applied fix:** Added `Store.CurrentOverlayEpoch(ctx, repoID)` mirroring `CurrentGraphVersion` (reads `semantic_live_overlay_meta.current_epoch` on `s.db`, no tx). Extended `compact.OverlayOps` interface with `CurrentOverlayEpoch`. Replaced the `1<<62` sentinel in `runCompaction` with a real `CurrentOverlayEpoch` read issued BEFORE `BeginSnapshot` opens its tx, so concurrent `BeginOverlayTx` allocations after the read receive `write_epoch > capturedEpoch` and SURVIVE `ClearOverlayLE`. Deleted the `captureEpoch` method entirely. Added `CurrentOverlayEpoch` stub to `fakeOverlay` test fake.

### CR-02: `Snapshot.ClearOverlayLE` unconditionally resets pending-rows counter even when newer-epoch rows survive

**Files modified:** `internal/semantic/store/snapshot.go`
**Commit:** 387207d5
**Applied fix:** Replaced the unconditional `Store(0)` reset (`resetOverlayPendingRowsIfPresent`) with a per-table `RowsAffected()` accumulation. Sums `totalDeleted` across the four `DELETE`s and atomically subtracts that count from the in-memory pending-rows counter via a CAS loop, clamped at zero (defensive against under-counting writers). The subtract still happens AFTER all four DELETEs succeed (load-bearing ordering documented in the new helper `subtractOverlayPendingRowsIfPresent`). Rows surviving the CAS contract (`write_epoch > capturedEpoch`) now correctly keep `OverlayHasPendingRows` positive, preventing the gate from deadlocking at `BlockedOverlayEmpty`.

### CR-03: `BeginSnapshot` snapshot-id allocation race causes PRIMARY KEY collision under concurrent compactions

**Files modified:** `internal/semantic/store/migrations.go`, `internal/semantic/store/migrations_registry.go`, `internal/semantic/store/migrations_types.go`, `internal/semantic/store/snapshot.go`, `internal/semantic/store/snapshot_test.go`, `internal/semantic/store/phase63_accessors_test.go`
**Commit:** 7b87101a
**Applied fix:** Added `applyMigration005` / `schema5Statements` that `CREATE SEQUENCE semantic_snapshot_id_seq START <MAX(snapshot_id)+1>` (the `START` is read pre-migration to avoid colliding with rows seeded by pre-migration code). Bumped `CurrentSchemaVersion` 4 → 5 and registered the migration in `migrations_registry.go`. Replaced `BeginSnapshot`'s `(SELECT COALESCE(MAX(snapshot_id),0)+1 ...)` inside-tx allocator with `SELECT nextval('semantic_snapshot_id_seq')` issued on `s.db` (NOT inside the snapshot tx) — mirrors the `current_epoch` bump pattern in `overlay.go:116-136`. The pre-allocated id is then passed into the `INSERT` via a positional `?` bind. Updated `seedCommittedSnapshots` to use the same SEQUENCE so test fixtures share the production allocator path. Updated `phase63_accessors_test.go`'s schema-version assertion to use `CurrentSchemaVersion` rather than a hardcoded `4`.

### CR-04: Test `TestSnapshot_DeleteSnapshotsBeyondAtomic` is non-deterministic; ties on `created_at` make survivor set undefined

**Files modified:** `internal/semantic/store/snapshot.go` (production half), `internal/semantic/store/snapshot_test.go` (test seed half — already in CR-03 commit)
**Commit:** 514d5364 (production), seed already updated in 7b87101a
**Applied fix:** Added `, snapshot_id DESC` tiebreaker to the keep-set `ORDER BY` in `DeleteSnapshotsBeyond`. Among rows sharing identical `created_at`, the higher `snapshot_id` (more-recently-allocated) now wins deterministically, eliminating DuckDB's implementation-defined tie resolution as a source of flakiness. The `seedCommittedSnapshots` helper was already migrated in the CR-03 commit to bind strictly-monotone Go-side `time.Now()`-derived timestamps (1ms apart per row), eliminating sub-microsecond collisions in the test fixture itself.

### WR-01: Compactor execution order disagrees with snapshot-side fakeCompactor reference

**Files modified:** `internal/semantic/compact/compactor.go`
**Commit:** 96bd3880
**Applied fix:** Reordered `runCompaction` body from `Begin → Write → DeleteSnapshotsBeyond → ClearOverlayLE → Commit` to the canonical `Begin → Write → ClearOverlayLE → DeleteSnapshotsBeyond → Commit` order documented in `snapshot_fake_compactor_test.go`. Updated the function-level doc comment to reflect the canonical order and call out WR-01 as the change rationale. DuckDB serializes statements within the tx so the committed state is identical, but keeping production aligned with the fake-compactor witness prevents future "fix it back" drift.

### WR-02: `AbortSnapshot` silently ignores `ctx` parameter

**Files modified:** `internal/semantic/store/snapshot.go`
**Commit:** 610ab05f
**Applied fix:** Removed the dead `_ = ctx` line. Switched the abort log emission from `slog.Default().Info` to `slog.Default().InfoContext(ctx, ...)` so trace propagation reaches the abort line. After the best-effort `Rollback()` (still attempted on a cancelled ctx so the tx never leaks), check `ctx.Err()` and surface cancellation as the primary error if present, with the rollback error nested as supplementary context if both fail. `snap.aborted` is now flipped only on successful rollback so subsequent calls see the correct `aborted/committed` state.

### WR-03: `TestSnapshot_TxAccessor_Absent` shells out to `grep`; non-portable

**Files modified:** `internal/semantic/store/snapshot_test.go`
**Commit:** 0a5dbb31
**Applied fix:** Replaced `exec.Command("grep", "-nE", ...)` with `os.ReadFile("snapshot.go")` + a `(?m)`-anchored `regexp.MustCompile`. Same encapsulation invariant (no exported `func (* *Snapshot) Tx(`), no PATH dependency, portable across all OSes Go itself runs on. Removed `os/exec` from imports and added `os` + `regexp`.

### WR-04: `BeginSnapshot` insert binds `created_at = time.Now()` from Go but DB compares against DB-side timestamps elsewhere

**Files modified:** `internal/semantic/store/snapshot.go`
**Commit:** deacbf58
**Applied fix:** Switched `BeginSnapshot`'s `created_at` and `CommitSnapshot`'s `committed_at` bindings from Go-side `time.Now()` to DuckDB-side `now()` (literal in the SQL, not a bind value). Now all rows in `semantic_snapshots` carry timestamps from a single clock — the DB engine's — eliminating Go-vs-DB clock-skew incoherence under container time-namespaces and monotonic-vs-wall clock swaps. The seed helper continues to use Go-side monotone `time.Now()` for fixture determinism (CR-04), which is internally consistent within the test.

### WR-05: `DeleteSnapshotsBeyond` issues `len(doomed) × 7` round-trip DELETEs

**Files modified:** `internal/semantic/store/snapshot.go`
**Commit:** c3c5bdaa
**Applied fix:** Replaced the nested `for id { for table { ExecContext } }` loop with one `DELETE FROM <table> WHERE snapshot_id IN (?, ?, ...)` per table. The placeholder string is built from `len(doomed)` only via `strings.Repeat("?,", len(doomed))`; no caller-derived data flows into the SQL (table names come from a hardcoded slice). The doomed ids continue to flow through positional `?` binds, preserving the parameterization invariant. Collapses 7N round-trips to 7. Added `strings` import.

### WR-06: `compactor.fire()` calls `runCompaction(context.Background())` — bypasses daemon ctx; cancellation does not propagate

**Files modified:** `internal/semantic/compact/compactor.go`
**Commit:** 9c361437
**Applied fix:** Added `parentCtx context.Context` field on `Compactor` (guarded by `parentCtxMu sync.Mutex`). `Run(ctx)` captures the daemon ctx into `parentCtx` at goroutine entry. New `runContext()` helper returns the captured parent ctx (or `context.Background()` pre-`Run`). `fire()` and `PublicTriggerForTest` now both call `runCompaction(c.runContext())` so daemon shutdown cancellation propagates through to the compaction tx. No daemon-side wiring change was required: `compact_wiring.go`'s existing `g.Go(c.Run(gctx))` already passes the right ctx.

### IN-01: `errClosed` declared but never used

**Files modified:** `internal/semantic/compact/compactor.go`
**Commit:** 50373380
**Applied fix:** Removed `var errClosed = errors.New("compactor closed")` and the dead `var _ = errClosed` self-reference. Dropped the now-unused `"errors"` import. Future shutdown-error surfaces can declare a fresh sentinel when wired.

### IN-02: `cas_property_test.go` accumulates dead local variables

**Files modified:** `internal/semantic/compact/cas_property_test.go`
**Commit:** 16847a30
**Applied fix:** Removed `dbPath`, the duplicate `cfg`, `cwd`, and the `defer func() { _ = wsDir }()` block — all of which were declared and immediately discarded via `_ = ...`. Kept `wsDir`, `storeCfg`, `provider`, and `changeWD(t, wsDir)` (the only state actually consumed). Dropped the now-unused `path/filepath` import. Test behavior unchanged.

### IN-03: snapshot.go INSERT into `semantic_snapshots` hardcodes magic strings `'compact'` and `'phase63'`

**Files modified:** `internal/semantic/store/snapshot.go`
**Commit:** d3c0a4b2
**Applied fix:** Hoisted both literals into package-level constants `snapshotKindCompact = "compact"` and `snapshotIndexerVersion = "phase63"` at the top of `snapshot.go`. The `BeginSnapshot` `INSERT` now binds them through `?` placeholders rather than embedding them inline. `status='pending'` stays inline because it is bound to the `BeginSnapshot` vs `CommitSnapshot` lifecycle, not to a caller-controllable value. Future indexer-version bumps replace the constant in one place.

### IN-04: Compaction metric label `reason` not carved out in `metrics_labels_test.go`; allowlist test won't catch drift

**Files modified:** `internal/obs/metrics_labels_test.go`
**Commit:** e58ae0df
**Applied fix:** Added carve-out entries for `helix_semantic_compaction_blocked_total` (`{"reason": true}`), `helix_semantic_compaction_duration_seconds` (`{}` — `outcome` already in `AllowedLabels`), and `helix_semantic_vacuum_duration_seconds` (`{}` — same). Primed all three families inside `TestMetricsLabelsAllowlist` via `SemanticCompactionObserve("success", 0.1)`, `SemanticCompactionBlocked("overlay_empty")`, and `SemanticVacuumObserve("success", 0.1)`. Without the priming, `Gather()` drops the empty families and `lintLabels` never sees the `reason` label — the lint would silently pass at CI and trip only at runtime on a live registry once the daemon emitted its first `BlockedOverlayEmpty`.

---

_Fixed: 2026-05-07T19:15:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
