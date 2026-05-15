---
phase: 70
plan: 02
subsystem: semantic-store
tags: [duckdb, migrations, schema, snapshot, overlay-epoch]
dependency_graph:
  requires:
    - "schema v5 (Phase 63 CR-03: semantic_snapshot_id_seq)"
    - "semantic_snapshots table (Phase 57 schema 1)"
    - "CommitSnapshot pattern (Phase 63 P63-01)"
  provides:
    - "schema v6: semantic_snapshots.base_overlay_epoch UBIGINT DEFAULT 0"
    - "SnapshotMeta.BaseOverlayEpoch field + SetBaseOverlayEpoch setter"
    - "Store.LatestCommittedSnapshotBaseEpoch(ctx, repoID) accessor"
    - "CommitSnapshot persists base_overlay_epoch atomically with status='committed'"
  affects:
    - "Plan 04 buildFn (consumer of LatestCommittedSnapshotBaseEpoch)"
    - "daemon StoreAccessor (Plan 03 wiring)"
tech-stack:
  added: []
  patterns:
    - "Schema migration: ALTER TABLE ADD COLUMN ... DEFAULT (DuckDB constraint: no NOT NULL on ALTER ADD COLUMN)"
    - "Setter-on-handle plumbing (Snapshot.SetBaseOverlayEpoch) — mirrors CapturedEpoch in Meta"
    - "Cold-start signal via DEFAULT 0 / (0, false, nil) accessor return"
    - "sql.ErrNoRows → (0, false, nil) idiom for missing-row accessors"
key-files:
  created: []
  modified:
    - "internal/semantic/store/migrations_types.go (CurrentSchemaVersion 5 → 6)"
    - "internal/semantic/store/migrations_registry.go (append {From: 5, To: 6})"
    - "internal/semantic/store/migrations.go (applyMigration006 + schema6Statements)"
    - "internal/semantic/store/migrations_test.go (+2 tests: fresh-v6 + upgrade-from-v5)"
    - "internal/semantic/store/snapshot.go (SnapshotMeta.BaseOverlayEpoch + SetBaseOverlayEpoch + CommitSnapshot UPDATE + LatestCommittedSnapshotBaseEpoch)"
    - "internal/semantic/store/snapshot_test.go (+6 tests: persist + 4 accessor scenarios + nil-guard)"
decisions:
  - "Plumbing path option (a): SnapshotMeta.BaseOverlayEpoch + (*Snapshot).SetBaseOverlayEpoch setter, NOT extending SnapshotSummary. SnapshotSummary is summary metadata (counts, durations); BaseOverlayEpoch is part of the snapshot's identity (it ends up on the snapshot row), so it belongs on Meta. Setter exists because Plan 04's buildFn computes the post-merge epoch AFTER BeginSnapshot but BEFORE CommitSnapshot."
  - "data_type asserted as UBIGINT (case-insensitive match) to mirror schema 3's current_epoch convention on semantic_live_overlay_meta."
  - "Accessor lives on *Store (not *Snapshot) — daemon StoreAccessor wraps it without a snapshot handle (matches OverlayChangedPathsSince discipline from Plan 01)."
  - "ORDER BY snapshot_id DESC LIMIT 1 instead of MAX(committed_at): the SEQUENCE-allocated snapshot_id is monotone across BeginSnapshot calls per Phase 63 CR-03, so the highest id is the most-recent committed snapshot — avoids the tie-break flakiness committed_at hits under sub-microsecond writes."
metrics:
  duration: "~25 min"
  completed: "2026-05-15"
---

# Phase 70 Plan 02: Snapshot Baseline-Epoch Persistence Summary

Schema migration 5 → 6 adds `semantic_snapshots.base_overlay_epoch UBIGINT DEFAULT 0`; `CommitSnapshot` now persists the baseline overlay epoch atomically with the `status='committed'` flip; `Store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)` exposes the seam Plan 04's incremental buildFn calls to feed `OverlayChangedPathsSince(repoID, baseEpoch)`.

## Tasks Completed

| Task | Name                                                                                           | Commits                                              |
| ---- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| 1    | Schema migration 5 → 6 (applyMigration006 + registry + version bump + migration test)         | RED `3d5cd4a6`, GREEN `5690c7cd`                     |
| 2    | SnapshotMeta.BaseOverlayEpoch + CommitSnapshot persistence + LatestCommittedSnapshotBaseEpoch  | RED `ee557380`, GREEN `93da7f61`                     |

## What Was Built

**Migration (v5 → v6)**

- `internal/semantic/store/migrations_types.go`: `CurrentSchemaVersion = 6`.
- `internal/semantic/store/migrations_registry.go`: appended `{From: 5, To: 6, Kind: MigrationInPlace, Apply: applyMigration006}`.
- `internal/semantic/store/migrations.go`: `applyMigration006` + `schema6Statements` issue exactly two statements:
  - `ALTER TABLE semantic_snapshots ADD COLUMN base_overlay_epoch UBIGINT DEFAULT 0`
  - `INSERT INTO semantic_schema_version (version, applied_at) VALUES (6, now())`

**Snapshot API extension**

- `SnapshotMeta.BaseOverlayEpoch uint64` — new field with detailed doc comment explaining the plumbing intent.
- `(*Snapshot).SetBaseOverlayEpoch(epoch)` — setter callers use between BeginSnapshot and CommitSnapshot to record the post-merge epoch.
- `CommitSnapshot` UPDATE now writes `base_overlay_epoch=?` in the same `UPDATE semantic_snapshots SET status='committed', committed_at=now(), base_overlay_epoch=?` statement.
- `func (s *Store) LatestCommittedSnapshotBaseEpoch(ctx, repoID) (uint64, bool, error)` — reads `SELECT base_overlay_epoch FROM semantic_snapshots WHERE repo_id=? AND status='committed' ORDER BY snapshot_id DESC LIMIT 1`. Returns `(epoch, true, nil)` on hit, `(0, false, nil)` on cold-start (`errors.Is(err, sql.ErrNoRows)`), wrapped error otherwise. Nil-store + empty-repoID rejected.

## Tests Added (8 new tests / sub-tests)

| Test                                                                | Coverage                                                 |
| ------------------------------------------------------------------- | -------------------------------------------------------- |
| `TestMigration006_BaseOverlayEpochColumn`                           | Fresh open → v6, column exists with type UBIGINT, DEFAULT 0 honored on bare INSERT |
| `TestMigration006_UpgradeFromV5_DefaultsExistingRows`               | Pre-v6 snapshot row picks up `base_overlay_epoch=0` after Open runs 5→6 |
| `TestCommitSnapshot_PersistsBaseOverlayEpoch/explicit_epoch_round_trips` | Setter value (42) → DB column                       |
| `TestCommitSnapshot_PersistsBaseOverlayEpoch/unset_defaults_to_zero` | No setter call → column = 0                              |
| `TestLatestCommittedSnapshotBaseEpoch_RoundTrip`                    | Two snapshots (epochs 7, 11) → accessor returns 11        |
| `TestLatestCommittedSnapshotBaseEpoch_NoSnapshot`                   | Cold-start: `(0, false, nil)`                            |
| `TestLatestCommittedSnapshotBaseEpoch_NilStore`                     | Nil-receiver guard                                       |
| `TestLatestCommittedSnapshotBaseEpoch_EmptyRepoID`                  | Empty-repoID guard                                       |
| `TestLatestCommittedSnapshotBaseEpoch_IgnoresPendingAndAborted`     | Pending snapshot with epoch=99 does NOT shadow committed=5 |

## Verification

- `go test ./internal/semantic/store/ -race -count=1` → PASS (4.87s, all existing + new tests)
- `go test ./internal/semantic/compact/ -race -count=1` → PASS (compactor consumes CommitSnapshot)
- `go vet ./internal/semantic/store/...` → exits 0
- `make vet` → exits 0 (all custom vet tools: noduckdb, nokernel2semantic, nosemantic2kernel, compact-uses-store)
- `go build ./...` → clean (only pre-existing Swift binding cgo warning)

## Grep Gates (acceptance criteria)

```
internal/semantic/store/migrations_types.go:24:const CurrentSchemaVersion = 6
internal/semantic/store/migrations_registry.go:33: {From: 5, To: 6, Kind: MigrationInPlace, Apply: applyMigration006},
internal/semantic/store/migrations.go:680: ALTER TABLE semantic_snapshots ADD COLUMN base_overlay_epoch UBIGINT DEFAULT 0
internal/semantic/store/migrations.go:683: INSERT INTO semantic_schema_version (version, applied_at) VALUES (6, now())
internal/semantic/store/snapshot.go:111: BaseOverlayEpoch uint64
internal/semantic/store/snapshot.go:451:    SET status='committed', committed_at=now(), base_overlay_epoch=?
internal/semantic/store/snapshot.go:788:func (s *Store) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (epoch uint64, ok bool, err error)
```

All seven gates match as required.

## Deviations from Plan

None — plan executed exactly as written. The plumbing-option decision (a vs b) was a planned in-task choice; documented rationale in the GREEN commit message and the decisions block above.

## Decisions Made

- **Plumbing path: option (a)** — SnapshotMeta field + (*Snapshot).SetBaseOverlayEpoch setter. `SnapshotSummary` is summary metadata (counts/durations); `BaseOverlayEpoch` is part of the snapshot's persisted identity, so it belongs on `Meta`. The setter exists because Plan 04's buildFn needs to compute the post-merge epoch AFTER BeginSnapshot.
- **Accessor ordering: `ORDER BY snapshot_id DESC`** (not `MAX(committed_at)`). The SEQUENCE-allocated `snapshot_id` is monotone across BeginSnapshot calls (Phase 63 CR-03), so it avoids tie-break flakiness `committed_at` exhibits under sub-microsecond writes. Same discipline as the `DeleteSnapshotsBeyond` survivor selection (Phase 63 CR-04).
- **`errors.Is(scanErr, sql.ErrNoRows)`** (not equality) for the cold-start path — wrapped-error tolerance per plan action step 4 and Go idiom.

## Known Stubs

None.

## Threat Flags

None — extends existing trusted seam (semantic_snapshots; package-store boundary), no new network/auth/file surface introduced. All SQL is parameterized via `?` binds (no string concat); the new column is purely additive.

## Self-Check: PASSED

Files exist and grep gates match:
- internal/semantic/store/migrations_types.go ✓ (CurrentSchemaVersion = 6)
- internal/semantic/store/migrations_registry.go ✓ ({From: 5, To: 6})
- internal/semantic/store/migrations.go ✓ (applyMigration006 + schema6Statements)
- internal/semantic/store/migrations_test.go ✓ (TestMigration006_*)
- internal/semantic/store/snapshot.go ✓ (BaseOverlayEpoch + SetBaseOverlayEpoch + LatestCommittedSnapshotBaseEpoch + base_overlay_epoch=?)
- internal/semantic/store/snapshot_test.go ✓ (TestCommitSnapshot_PersistsBaseOverlayEpoch + TestLatestCommittedSnapshotBaseEpoch_*)

Commits exist:
- 3d5cd4a6 ✓ test(70-02): add failing test for base_overlay_epoch v5→v6 migration
- 5690c7cd ✓ feat(70-02): land schema migration 5→6 base_overlay_epoch column
- ee557380 ✓ test(70-02): add failing tests for BaseOverlayEpoch + LatestCommittedSnapshotBaseEpoch
- 93da7f61 ✓ feat(70-02): persist base_overlay_epoch + add LatestCommittedSnapshotBaseEpoch accessor

## TDD Gate Compliance

Both tasks followed RED → GREEN sequence: test commit precedes implementation commit. No REFACTOR commits were needed (implementation landed cleanly on first GREEN pass; the migration and accessor are both isolated additive surfaces).
