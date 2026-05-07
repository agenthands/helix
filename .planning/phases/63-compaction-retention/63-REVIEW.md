---
phase: 63-compaction-retention
reviewed: 2026-05-07T18:30:00Z
depth: standard
files_reviewed: 41
files_reviewed_list:
  - Makefile
  - cmd/vet-compact-uses-store/main.go
  - internal/config/defaults.go
  - internal/config/loader_test.go
  - internal/daemon/compact_wiring.go
  - internal/daemon/daemon.go
  - internal/daemon/live_wiring.go
  - internal/kernel/edit/tools.go
  - internal/kernel/edit_tx_count.go
  - internal/kernel/edit_tx_count_test.go
  - internal/kernel/fileops/tools.go
  - internal/lint/compactusesstore/analyzer.go
  - internal/obs/metrics.go
  - internal/obs/metrics_labels_test.go
  - internal/semantic/compact/accessors.go
  - internal/semantic/compact/cas_property_helpers_test.go
  - internal/semantic/compact/cas_property_test.go
  - internal/semantic/compact/compactor.go
  - internal/semantic/compact/compactor_test.go
  - internal/semantic/compact/config.go
  - internal/semantic/compact/gate.go
  - internal/semantic/compact/gate_test.go
  - internal/semantic/compact/vacuum.go
  - internal/semantic/config.go
  - internal/semantic/graph/scheduler.go
  - internal/semantic/graph/scheduler_quiescent_test.go
  - internal/semantic/live/coalescer/coalescer.go
  - internal/semantic/live/coalescer/last_flush_test.go
  - internal/semantic/live/service/service.go
  - internal/semantic/lspenrich/lane_accessors_test.go
  - internal/semantic/lspenrich/queue.go
  - internal/semantic/store/duckdb.go
  - internal/semantic/store/migrations.go
  - internal/semantic/store/migrations_registry.go
  - internal/semantic/store/migrations_test.go
  - internal/semantic/store/migrations_types.go
  - internal/semantic/store/overlay.go
  - internal/semantic/store/phase63_accessors_test.go
  - internal/semantic/store/snapshot.go
  - internal/semantic/store/snapshot_fake_compactor_test.go
  - internal/semantic/store/snapshot_test.go
  - internal/semantic/store/vacuum.go
findings:
  critical: 4
  warning: 6
  info: 4
  total: 14
status: issues_found
---

# Phase 63: Code Review Report

**Reviewed:** 2026-05-07T18:30:00Z
**Depth:** standard
**Files Reviewed:** 41
**Status:** issues_found

## Summary

Phase 63 lands the snapshot-write API (P63-01) and the compaction worker
on top of it (P63-02). The store-side primitives are well-structured: the
`*Snapshot` handle correctly encapsulates `*sql.Tx`, retention and overlay
clear run inside the same tx (atomicity invariant), and the vet analyzer
enforces the compact→store boundary.

However, the compactor itself contains a **CAS-contract violation** that
defeats the entire Phase 60 D-04 epoch-isolation design: `captureEpoch`
returns the sentinel `1 << 62`, which is a maximum that includes future
overlay writes rather than a snapshot of the current epoch. This causes
`ClearOverlayLE` to delete rows committed *during* compaction —
the explicit data-loss scenario the design exists to prevent. The
unconditional pending-rows counter reset in `Snapshot.ClearOverlayLE`
compounds the issue.

A second BLOCKER is a snapshot-id allocation race: `BeginSnapshot`
computes `MAX(snapshot_id)+1` inside the tx, so two concurrent
`BeginSnapshot` calls (different repos, same store) both observe the
same MAX and produce a primary-key collision when the second commits.

The retention-test fixture (`seedCommittedSnapshots`) inserts seven rows
in a tight loop using DuckDB `now()` for `created_at`, then asserts on
`ORDER BY created_at DESC` ordering with ties — this is non-deterministic
and the test will be flaky.

Also flagged: a stale callsite-order mismatch between `compactor.go`
runCompaction and the documented invariant; `AbortSnapshot` silently
discards its `ctx` parameter; the `TestSnapshot_TxAccessor_Absent` test
shells out to `grep`, which is not portable.

## Critical Issues

### CR-01: Compactor `captureEpoch` sentinel destroys CAS contract; deletes rows committed during compaction

**File:** `internal/semantic/compact/compactor.go:285-290` (used at `:195`)
**Issue:** `captureEpoch` returns the constant `1 << 62`. The same value
is then passed to `snap.ClearOverlayLE(ctx, repoID, capturedEpoch)` at
`:244`, which issues `DELETE FROM semantic_live_overlay_* WHERE repo_id=?
AND write_epoch <= ?`.

The Phase 60 D-04 contract (mirrored verbatim in `snapshot.go:514-519`,
`overlay.go:245-254`, and the test
`TestCAS_InterleaveOverlayWritesWithCompaction`) is: *"rows committed
during compaction (`write_epoch > capturedEpoch`) survive."* The
contract requires `capturedEpoch` to be a snapshot of the current epoch
**before** compaction begins, so any concurrent `BeginOverlayTx` that
allocates a higher epoch escapes the DELETE.

`1 << 62` (≈ 4.6e18) is the *opposite* of a snapshot — it is the
maximum allowed by the duckdb-go uint64 bind clamp. Any plausible
production epoch counter is a small integer (one bump per overlay tx).
A concurrent overlay write that races with the compactor will receive
e.g. `write_epoch = 142`, then ClearOverlayLE deletes it because
`142 <= (1<<62)` is true. This is silent data loss of every overlay
row written during the compaction window.

The compactor's own doc comment (`compactor.go:271-284`) acknowledges
the requirement (*"capturedEpoch is captured BEFORE the new tx opens"*)
but the implementation does the opposite. The downstream
`OverlayRowCount` pre-flight at `:198` is bound by the same sentinel,
so the size guard is also broken — it counts rows beyond the intended
window and may erroneously trip `outcome=partial`.

**Fix:** Read the current overlay epoch from
`semantic_live_overlay_meta.current_epoch` (or expose a typed
`Store.CurrentOverlayEpoch(ctx, repoID) (uint64, error)` accessor that
mirrors `CurrentGraphVersion` in `overlay.go:983`) BEFORE
`BeginSnapshot`:

```go
// Add to internal/semantic/store/overlay.go (mirrors CurrentGraphVersion):
func (s *Store) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
    if s == nil || s.db == nil {
        return 0, fmt.Errorf("CurrentOverlayEpoch: nil store")
    }
    if repoID == "" {
        return 0, fmt.Errorf("CurrentOverlayEpoch: empty repoID")
    }
    var ep uint64
    err := s.db.QueryRowContext(ctx, `
        SELECT current_epoch FROM semantic_live_overlay_meta WHERE repo_id = ?
    `, repoID).Scan(&ep)
    if err == sql.ErrNoRows { return 0, nil }
    if err != nil { return 0, fmt.Errorf("CurrentOverlayEpoch(%q): %w", repoID, err) }
    return ep, nil
}

// In compactor.go runCompaction, replace the sentinel:
capturedEpoch, err := c.deps.Store.CurrentOverlayEpoch(ctx, c.repoID)
if err != nil { /* error path */ }
```

Then extend the `OverlayOps` interface with `CurrentOverlayEpoch` and
delete `captureEpoch` entirely. The
`TestCAS_InterleaveOverlayWritesWithCompaction` happens to pass
because it uses a hand-set `capturedEpoch=5` and never exercises the
production path; this critical bug ships uncovered by tests.

### CR-02: `Snapshot.ClearOverlayLE` unconditionally resets pending-rows counter even when newer-epoch rows survive

**File:** `internal/semantic/store/snapshot.go:533-573` (specifically `:570`)
**Issue:** After the four DELETE statements,
`ClearOverlayLE` calls
`resetOverlayPendingRowsIfPresent(snap.store, repoID)` which sets the
in-memory counter to **0** unconditionally. The counter is the gate's
`OverlayHasPendingRows` proxy (`overlay.go:223-243`).

The CAS contract preserves rows with `write_epoch > capturedEpoch`. If
even one such row exists at clear time (the normal case under
concurrent overlay writes during compaction), the pending-rows counter
should remain positive — but it is forced to 0. The next
`OverlayHasPendingRows(repoID)` call returns false, and the gate then
permanently reports `BlockedOverlayEmpty`, blocking compaction even
though work is queued.

The counter is only ever bumped on a *new* overlay write (per
`overlay.go:399`, `:427`, `:461`, `:485`, `:576`, `:751`); a row whose
write happened *before* the reset but persists past the DELETE never
re-bumps. So the counter underflows the truth and the gate stays stuck
until the next concurrent overlay write enqueues something — which
re-stamps the counter but the previous rows are still uncounted.

The doc comment (`snapshot.go:525-532`) describes a contract about
"AFTER ordering" but says nothing about preserving the count when the
DELETE leaves rows in place. The comment is wrong about the semantics:
the reset assumes the DELETE drains all rows, which is only true when
`capturedEpoch == current_epoch` (synchronous, no concurrent writers).

**Fix:** Compute the post-DELETE residual via the per-table DELETE's
`RowsAffected()`, and instead of unconditional reset use atomic
subtraction by the deleted count, OR (simpler) re-query the actual
overlay row count after the DELETE and `Store(int64(remaining))`:

```go
// In ClearOverlayLE, replace resetOverlayPendingRowsIfPresent with:
var totalDeleted int64
for _, d := range overlayDeletes {
    res, err := snap.tx.ExecContext(ctx, d.sql, repoID, capturedEpoch)
    if err != nil { return fmt.Errorf("ClearOverlayLE(%s): %w", d.name, err) }
    if n, _ := res.RowsAffected(); n > 0 { totalDeleted += n }
}
if totalDeleted > 0 && snap.store != nil {
    snap.store.subtractOverlayPendingRows(repoID, totalDeleted)
}

// Add helper on *Store that does Add(-n) but clamps at zero:
func (s *Store) subtractOverlayPendingRows(repoID string, n int64) {
    s.overlayCountsMu.Lock()
    c, ok := s.overlayPendingRows[repoID]
    s.overlayCountsMu.Unlock()
    if !ok || c == nil { return }
    for {
        cur := c.Load()
        next := cur - n
        if next < 0 { next = 0 }
        if c.CompareAndSwap(cur, next) { return }
    }
}
```

Combined with CR-01's fix (reading a real `capturedEpoch`), the
counter then mirrors disk truth.

### CR-03: `BeginSnapshot` snapshot-id allocation race causes PRIMARY KEY collision under concurrent compactions

**File:** `internal/semantic/store/snapshot.go:230-246`
**Issue:** Snapshot IDs are allocated inline:

```sql
INSERT INTO semantic_snapshots (snapshot_id, ...)
VALUES ((SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots), ...)
```

This SELECT runs inside the snapshot's *own* tx. DuckDB's
SNAPSHOT/MVCC isolation means each tx sees a consistent view of the
table at tx start. Two concurrent `BeginSnapshot` calls — distinct
repoIDs, same `*Store` — both compute `MAX(snapshot_id)+1` against
the same pre-tx snapshot of the table, so they both produce the SAME
new ID. Whichever commits second hits a primary-key violation on
`semantic_snapshots.snapshot_id` and the entire compaction tx aborts
(retention deletes + overlay clear all roll back).

`overlay.go` carefully avoids this exact issue for `current_epoch`:
the bump is issued on `s.db` (NOT inside the tx) under a per-workspace
mutex (`overlay.go:105-136`), with the explicit comment *"the increment
is issued on s.db (NOT inside the per-tx \*sql.Tx) so it commits
independently of the user-visible transaction"*. `BeginSnapshot`
ignores this lesson.

The race is not theoretical: the daemon spins up one compactor
goroutine *per workspace* (`compact_wiring.go:140-187`), each with its
own AfterFunc timer. Two workspaces flushing within
`CompactAfterIdle` of each other will both call `BeginSnapshot`
concurrently and at least one tx will fail.

**Fix:** Use a DuckDB SEQUENCE for `snapshot_id` (added via a new
migration), or generate IDs out-of-band on `s.db` under a process-wide
mutex before opening the snapshot tx, mirroring the `current_epoch`
pattern in `overlay.go:116-136`:

```go
// In BeginSnapshot, before s.db.BeginTx:
var newID uint64
if err := s.db.QueryRowContext(ctx, `
    SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots
`).Scan(&newID); err != nil { /* handle */ }

// Then INSERT with a literal id binding inside the tx — but a process-wide
// mutex around the SELECT+INSERT is required, OR use a SEQUENCE:
//   migration005: CREATE SEQUENCE semantic_snapshot_id_seq START 1
//   INSERT ... VALUES (nextval('semantic_snapshot_id_seq'), ...)
```

A SEQUENCE is the cleanest fix — DuckDB sequences are tx-aware but
allocate non-conflicting values across concurrent txs.

### CR-04: Test `TestSnapshot_DeleteSnapshotsBeyondAtomic` is non-deterministic; ties on `created_at` make survivor set undefined

**File:** `internal/semantic/store/snapshot_test.go:455-478` (helper) and `:226-296` (consumer)
**Issue:** `seedCommittedSnapshots` inserts seven rows in a tight Go
loop, each using DuckDB `now()` for `created_at`. DuckDB's `now()`
returns the transaction-start timestamp at sub-second resolution, but
in a fast loop two `db.QueryRow` calls easily complete inside the same
microsecond bucket — the seven rows can carry identical or
near-identical `created_at` values.

`DeleteSnapshotsBeyond` then orders by `ORDER BY created_at DESC LIMIT
?` (snapshot.go:461). With ties, DuckDB's tiebreaker is implementation-
defined. The test asserts at `:251-275` that "the 4 most-recent prior
IDs (last 4 of priorIDs) survive together with snap.ID itself" — but
under tie-resolution that ordering is not guaranteed; the surviving
set could be any 4 of the 7 prior IDs.

The retention SQL also does NOT use `snapshot_id DESC` as a
tie-breaker, so the production behavior has the same flakiness:
`retain=5` with 7 same-microsecond snapshots will keep an arbitrary
subset.

**Fix:** Two parts.

1) Make the production ORDER deterministic — break ties on
`snapshot_id DESC`:

```go
// snapshot.go:454-464
rows, err := snap.tx.QueryContext(ctx, `
    SELECT snapshot_id FROM semantic_snapshots
     WHERE repo_id = ? AND snapshot_id != ?
       AND snapshot_id NOT IN (
           SELECT snapshot_id FROM semantic_snapshots
            WHERE repo_id = ? AND snapshot_id != ?
              AND status = 'committed'
            ORDER BY created_at DESC, snapshot_id DESC
            LIMIT ?
       )
`, snap.RepoID, snap.ID, snap.RepoID, snap.ID, retain-1)
```

2) Make the test seed produce monotone `created_at`:

```go
// snapshot_test.go: replace `now()` with an explicit timestamp:
err := s.db.QueryRow(`
    INSERT INTO semantic_snapshots (
        snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
        worktree_hash, schema_version, indexer_version, status,
        partial, created_at, committed_at
    ) VALUES (
        (SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots),
        ?, '', 0, 'compact', '', 1, 'test', 'committed',
        false, ?, ?
    )
    RETURNING snapshot_id
`, repoID, time.Now().Add(time.Duration(i)*time.Millisecond),
   time.Now().Add(time.Duration(i)*time.Millisecond)).Scan(&id)
```

## Warnings

### WR-01: Compactor execution order disagrees with snapshot-side fakeCompactor reference

**File:** `internal/semantic/compact/compactor.go:159-269`
**Issue:** The runCompaction body executes
`Begin → WriteSnapshotFacts → DeleteSnapshotsBeyond → ClearOverlayLE
→ CommitSnapshot` (lines 232, 238, 244, 250). The reference
`fakeCompactor.cycle` in
`internal/semantic/store/snapshot_fake_compactor_test.go:19-43`
documents the canonical order as
`Begin → Write → ClearOverlayLE → DeleteSnapshotsBeyond → Commit`.
The function-level doc comment in `compactor.go:155-158` states yet a
third order: *"BeginSnapshot → WriteSnapshotFacts →
snap.DeleteSnapshotsBeyond → snap.ClearOverlayLE → CommitSnapshot"*
(matches the body but contradicts the fake-compactor reference).

Both orderings produce the same final state inside a single tx (DuckDB
serializes statements within a tx), so this is not a correctness bug
today — but it drifts from the canonical fakeCompactor witness used to
prove the API is consumable. Future maintainers will read the
fake-compactor as authoritative and silently "fix" the production code
back to the documented order, potentially missing other invariants
that were tightened along the way.

**Fix:** Pick one order. The fake-compactor order
(`Clear → Retention → Commit`) is preferable because it minimizes the
window during which the new snapshot exists alongside obsolete overlay
rows. Update either the production code or the doc comments + fake
compactor so they agree, and add a one-line invariant test that asserts
the order matches across the two files.

### WR-02: `AbortSnapshot` silently ignores `ctx` parameter

**File:** `internal/semantic/store/snapshot.go:398-424` (specifically `:399`)
**Issue:** `AbortSnapshot` opens with `_ = ctx` then never references
the parameter again. `*sql.Tx.Rollback()` does not accept a context, so
the parameter is dead. This is a footgun: a caller that wires a
cancellation-prone parent ctx (e.g., the daemon's gctx during shutdown)
expects rollback to honor cancellation, but the rollback may block
indefinitely on the underlying driver if duckdb-go ever stalls.

This is also the only `*Snapshot` method that takes a ctx without
using it; the lint shouldn't have to accept it. More importantly, slog
emission at `:413-417` should be context-aware (e.g., for trace
propagation) but isn't.

**Fix:** Either remove the parameter (breaking-change for callers,
acceptable inside `internal/`) or use `context.Cause(ctx)` to fail-fast
if the ctx is already cancelled, and pipe the ctx through a
`slog.InfoContext`:

```go
func (s *Store) AbortSnapshot(ctx context.Context, snap *Snapshot, reason string) error {
    if err := ctx.Err(); err != nil {
        // Context already cancelled — still attempt rollback so the tx
        // doesn't leak, but surface the cause to the caller.
        _ = snap.tx.Rollback()
        snap.aborted = true
        return fmt.Errorf("AbortSnapshot: ctx cancelled: %w", err)
    }
    // ... existing body, with slog.InfoContext(ctx, ...)
}
```

### WR-03: `TestSnapshot_TxAccessor_Absent` shells out to `grep`; non-portable

**File:** `internal/semantic/store/snapshot_test.go:406-418`
**Issue:** The test invokes `exec.Command("grep", "-nE", ...)` to scan
`snapshot.go` for an exported `Tx()` method. This requires `grep` on
`PATH` and the `-nE` flag set (BSD vs GNU semantics differ). On a
minimal CI image without `grep`, on Windows without WSL/MSYS, or in a
restricted-PATH sandbox, this test fails for environmental reasons
unrelated to the encapsulation invariant under test.

The repository already uses platform-conditional helpers
(`cas_property_helpers_test.go`); the same approach can replace `grep`
with native Go I/O.

**Fix:** Read the file in Go and use a regex:

```go
func TestSnapshot_TxAccessor_Absent(t *testing.T) {
    src, err := os.ReadFile("snapshot.go")
    if err != nil {
        t.Fatalf("read snapshot.go: %v", err)
    }
    re := regexp.MustCompile(`(?m)^func \(\w+ \*Snapshot\) Tx\(`)
    if loc := re.FindIndex(src); loc != nil {
        t.Errorf("encapsulation violated — public Tx() accessor on *Snapshot at byte offset %d", loc[0])
    }
}
```

### WR-04: `BeginSnapshot` insert binds `created_at = time.Now()` from Go but DB compares against DB-side timestamps elsewhere

**File:** `internal/semantic/store/snapshot.go:242` and `:374`
**Issue:** `BeginSnapshot` binds `time.Now()` for `created_at`;
`CommitSnapshot` binds `time.Now()` for `committed_at`. Meanwhile the
test helper `seedCommittedSnapshots` uses DuckDB-side `now()`. The two
clocks can drift — DuckDB's `now()` is the transaction-start clock on
the DB process, `time.Now()` is the calling process's wall clock. On a
host where the daemon and the DuckDB engine see different system
clocks (rare but possible inside containers with skewed time
namespaces) or under monotonic-clock vs wall-clock swap, the
ORDER BY created_at retention reasoning becomes incoherent.

This is also why CR-04 is harder to fix at the seed layer than at the
production layer.

**Fix:** Standardize on DB-side `now()` for all timestamp columns
written through the store package:

```go
// In BeginSnapshot:
INSERT INTO semantic_snapshots (
    snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
    worktree_hash, schema_version, indexer_version, status,
    partial, created_at
) VALUES (
    (SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots),
    ?, '', ?, 'compact', '', ?, 'phase63', 'pending',
    false, now()
)
RETURNING snapshot_id
```

(Drop the `time.Now()` bind for created_at; the store.committed_at
flip in CommitSnapshot is similarly DB-clocked.)

### WR-05: `DeleteSnapshotsBeyond` issues `len(doomed) × 7` round-trip DELETEs

**File:** `internal/semantic/store/snapshot.go:504-510`
**Issue:** The cascade loops `for _, id := range doomed { for _, c := range cascade { snap.tx.ExecContext(...) } }`, issuing 7 round-trips per
doomed snapshot (one per fact table). With retention=5 and 100 prior
snapshots, that's 95 × 7 = 665 ExecContext calls. Within a single tx
this still runs serially through the DB driver; not catastrophic, but
out-of-line for a hot path that fires on every coalescer-flush idle
window.

A single `DELETE ... WHERE snapshot_id IN (...)` with the doomed IDs
materialized as a comma-separated list would collapse 7N round-trips
to 7. DuckDB's parameter expansion supports list binds via
`?` repetition.

**Fix:** Build a single DELETE per table using parameter expansion:

```go
if len(doomed) == 0 { return nil }
placeholders := strings.Repeat("?,", len(doomed))
placeholders = placeholders[:len(placeholders)-1]
args := make([]any, len(doomed))
for i, id := range doomed { args[i] = id }
for _, c := range []struct{ name, sql string }{
    {"semantic_files",   fmt.Sprintf(`DELETE FROM semantic_files WHERE snapshot_id IN (%s)`, placeholders)},
    // ... 6 more
} {
    if _, err := snap.tx.ExecContext(ctx, c.sql, args...); err != nil {
        return fmt.Errorf("DeleteSnapshotsBeyond(%s): %w", c.name, err)
    }
}
```

Note the `fmt.Sprintf` is on placeholder strings, not on caller data
— the parameterization invariant remains intact.

### WR-06: `compactor.fire()` calls `runCompaction(context.Background())` — bypasses daemon ctx; cancellation does not propagate

**File:** `internal/semantic/compact/compactor.go:143-145` and
`:149-151`
**Issue:** Both `fire()` (the `time.AfterFunc` callback) and
`PublicTriggerForTest` use `context.Background()` for `runCompaction`.
The daemon's `g.Go(func() error { return c.Run(gctx) })` wires a
cancellation-aware ctx, but the AfterFunc fires its goroutine outside
the errgroup context. On daemon shutdown, an in-flight compaction tx
will continue executing to completion (or until DuckDB's internal
timeout fires) rather than honoring the shutdown signal. For a long
VACUUM (when D-05 wires a real implementation in a future phase) this
is a multi-minute shutdown delay.

The compactor stores `runCtx` in `compactBundle.runCtx`
(`compact_wiring.go:151-156`) but the per-compactor instance has no
back-reference to the bundle, so it cannot pull the gctx into the
AfterFunc callback.

**Fix:** Pass the daemon ctx into the Compactor at construction so
fire() can derive a child:

```go
type Compactor struct {
    // ... existing
    parentCtx context.Context  // captured at NewCompactor
}

func NewCompactor(ws workspace.WorkspaceKey, repoID string, cfg Config, deps Deps, parentCtx context.Context) *Compactor {
    // ...
    c.parentCtx = parentCtx
    return c
}

func (c *Compactor) fire() {
    ctx := c.parentCtx
    if ctx == nil { ctx = context.Background() }
    c.runCompaction(ctx)
}
```

And update `compact_wiring.go:172-178` to pass `runCtx` through.

## Info

### IN-01: `errClosed` declared but never used

**File:** `internal/semantic/compact/compactor.go:299-303`
**Issue:** `var errClosed = errors.New("compactor closed")` followed
immediately by `var _ = errClosed // reserved for future shutdown-error
surfaces`. This is dead code today; the comment is a TODO without an
issue link.
**Fix:** Either delete or wire it up to actual shutdown-path returns.
A `var _ = ...` self-reference is a stronger signal than a comment to
keep dead identifiers around — remove both.

### IN-02: `cas_property_test.go` accumulates dead local variables

**File:** `internal/semantic/compact/cas_property_test.go:42-67`
**Issue:** Multiple `_ = dbPath`, `_ = cfg`, `_ = wsDir`, `_ = cwd`
underscore-suppressions mark variables that are unused. The block at
lines 42-67 declares `dbPath`, `cfg`, `wsDir`, `cwd` then immediately
discards each — the only meaningful state is `storeCfg` and the
`changeWD` call. The redundant declarations make the test hard to
read.
**Fix:** Remove unused locals; keep only `storeCfg` and
`changeWD(t, t.TempDir())`.

### IN-03: snapshot.go INSERT into `semantic_snapshots` hardcodes magic strings `'compact'` and `'phase63'`

**File:** `internal/semantic/store/snapshot.go:236-241`
**Issue:** The INSERT writes `kind='compact'` and
`indexer_version='phase63'` as string literals. These are control-plane
metadata that downstream readers (admin/status, future cluster-side
introspection) will need to match against; embedding the version
string `phase63` in code that lands in v1.x means a v1.10 binary still
writes `'phase63'` for compaction-origin snapshots. Use the
`semantic.IndexerVersion` constant (or add one) so the binary's
self-version lands in the row.
**Fix:** Define a constant in `internal/semantic/store/snapshot.go`
(or hoist from a shared package) and reference it; or accept a
`SnapshotMeta.IndexerVersion` field that the compactor populates.

### IN-04: Compaction metric label `reason` not carved out in `metrics_labels_test.go`; allowlist test won't catch drift

**File:** `internal/obs/metrics_labels_test.go:22-88` (carve-outs map),
emission at `internal/obs/metrics.go:407-413`
**Issue:** `helix_semantic_compaction_blocked_total` carries a
`reason` label (closed-enum BlockedReason — six values). `reason` is
NOT in `AllowedLabels` (which has 6 entries from D-04) and NOT in the
`carveOuts` map. `TestMetricsLabelsAllowlist` (`:149-201`) primes
every metric family it expects to scan — but the Phase 63 family is
NOT primed. Empty families are dropped by Gather() so the lint never
sees the forbidden label, and the test passes deceptively. The
moment the daemon emits its first `BlockedOverlayEmpty` in production,
`reason` becomes visible and the lint would fail — but only at runtime
on a live registry, not in CI.

**Fix:** Add the carve-out and prime the families inside
`TestMetricsLabelsAllowlist`:

```go
// In carveOuts map:
"helix_semantic_compaction_blocked_total":  {"reason": true},
"helix_semantic_compaction_duration_seconds": {},  // outcome already in AllowedLabels
"helix_semantic_vacuum_duration_seconds":     {},  // outcome already in AllowedLabels

// In TestMetricsLabelsAllowlist body, near the existing primings:
m.SemanticCompactionObserve("success", 0.1)
m.SemanticCompactionBlocked("overlay_empty")
m.SemanticVacuumObserve("success", 0.1)
```

---

_Reviewed: 2026-05-07T18:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
