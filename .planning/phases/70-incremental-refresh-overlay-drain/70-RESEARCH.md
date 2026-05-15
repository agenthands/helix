# Phase 70: Incremental Refresh Overlay-Drain — Research

**Researched:** 2026-05-15
**Domain:** DuckDB-backed semantic store accessor + live-update pipeline + eval-runner bench harness
**Confidence:** HIGH (all anchors verified in source)

## Summary

Phase 70 closes the "refresh-degraded" hole in `internal/daemon/semantic_wiring.go:1466-1480`
by introducing one new `*Store` read accessor — `OverlayChangedPathsSince(ctx, repoID, baseEpoch)`
— and wiring two consumers (`index_semantic_graph(mode=incremental)` buildFn,
`refresh_semantic_graph.files_updated`). The work is mechanically small:

- ~50 lines of new accessor code mirroring `CurrentOverlayEpoch` (overlay.go:1019) and
  `GetLatestFileFact` (filefact_accessor.go) patterns. [VERIFIED: source read]
- A schema-6 migration adding `base_overlay_epoch UBIGINT DEFAULT 0` to
  `semantic_snapshots`, plus a `CommitSnapshot` change to capture the current
  overlay epoch at commit time. [VERIFIED: migrations.go:495-535 pattern]
- A `collectCandidatePaths` rewrite that takes `mode` seriously (today the
  parameter is `_ string`, line 1479). [VERIFIED: source read]
- A two-line edit to `tools_refresh.go:170-176` that swaps
  `len(args.Paths)` for the seam's returned path count.
- New bounded-label metric `helix_incremental_refresh_fallback_total{reason, repo}`
  added to `internal/obs/metrics.go` mirroring `LiveFileFactDiff` (Phase 68 D-07).
- Two test files in `internal/eval/runner/` — a correctness table-test and a
  `t.Skip`-on-CI bench harness.

The risk is concentrated in (a) the **flush-timing** question for `refresh_semantic_graph`
(the live `Service.OnWorkspaceChanged` is fire-and-forget, so a naive read-after-call
race with the coalescer timer), and (b) the choice between an in-memory baseline-epoch
cache vs. a persisted `base_overlay_epoch` column on `semantic_snapshots`. CONTEXT.md
locks the persisted-column option (D3); this research confirms the schema delta is a
single ALTER + a tweak to `BeginSnapshot`/`CommitSnapshot`.

**Primary recommendation:** Ship the `*Store.OverlayChangedPathsSince` accessor in
the same file as `CurrentOverlayEpoch` (overlay.go), the `base_overlay_epoch` schema
delta as `applyMigration006`, the `collectCandidatePaths` rewrite gated by a test seam,
and the bench harness as `internal/eval/runner/bench_refresh_incremental_test.go` with
`t.Skip` when `os.Getenv("CI") != ""`. Address flush-timing by adding a synchronous
flush hook to the coalescer (or polling `CurrentOverlayEpoch` until it advances past
the pre-drain capture, capped at the configured `max_batch_delay_ms` plus slack).

## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D1** — Both `index_semantic_graph(mode=incremental)` and `refresh_semantic_graph`
  consume the SAME overlay-drain seam. Single source of truth; no `mode` arg on `refresh`.
- **D2** — Seam shape:
  ```go
  func (s *Store) OverlayChangedPathsSince(
      ctx context.Context, repoID string, baseEpoch uint64,
  ) (paths []string, currentEpoch uint64, err error)
  ```
  Lock-free read (no overlay tx); D-09 compliant (no `Begin/Commit/Abort/Write` on the
  read path). `baseEpoch==0` → returns ALL current overlay paths (cold-start signal).
  Empty result with non-zero base → caller falls back to full-walk. Idempotent.
- **D3** — Baseline epoch stored as a new `base_overlay_epoch UBIGINT` column on
  `semantic_snapshots`. For `index`: captured at `CommitSnapshot` time from
  `CurrentOverlayEpoch`. For `refresh`: captured at the START of the call (pre-drain).
- **D4** — Fallback policy: full-walk on empty seam + bounded-label warn metric
  `helix_incremental_refresh_fallback_total{reason ∈ {cold_start, overlay_rotated, empty_overlay}, repo}`.
- **D5** — Two test files: `refresh_incremental_test.go` (correctness, race-clean) +
  `bench_refresh_incremental_test.go` (10k symbols, p95 ≤ 200ms, `t.Skip` on CI).
- **D6** — Remove the `refresh-degraded` annotation at semantic_wiring.go:1466-1480.
- **D7** — D-09 / D-13 invariants preserved; new accessor lives semantic-side; CI grep
  gate on `tools_refresh.go` for `Begin/Commit/Abort/Write` tokens stays green.

### Claude's Discretion

- **D3 follow-up** — "Whether `base_overlay_epoch` is a new column on `snapshots` or a
  sibling table indexed by `snapshot_id`. Smallest patch wins." → **Research recommends
  the column-on-snapshots option**: one `ALTER TABLE` matches the existing v3 migration
  pattern (60-CONTEXT.md D-04, migrations.go:495), reads in a single
  `SELECT base_overlay_epoch FROM semantic_snapshots WHERE snapshot_id=?`. A sibling
  table would require an extra JOIN per buildFn call and a second pending-row sync at
  `CommitSnapshot`.
- **D4 follow-up** — "Detection of `overlay_rotated` ... cleanest sentinel against Phase
  63 compaction's epoch-floor accessor." → No `EarliestRetainedEpoch` accessor currently
  exists; compaction uses `Snapshot.ClearOverlayLE(WHERE write_epoch <= capturedEpoch)`.
  Recommendation: classify `overlay_rotated` as `baseEpoch > 0 && currentEpoch > baseEpoch
  && len(paths) == 0` (all rows ≤ baseEpoch were swept by compaction; new rows >
  baseEpoch produce paths, none returned ⇒ rotation). Alternatively, add a sibling
  accessor `MinRetainedOverlayEpoch` if Phase 63 already has the data — verify in the
  pre-Wave-0 explore step.
- **Bench fixture scale-up** — `makeFixtureFacts` already takes `symbolCount int` (one
  file, N symbols). The bench needs N FILES with K symbols each. Extract a sibling
  helper `makeFixtureFactsNFiles(numFiles, symbolsPerFile int)` that emits realistic
  multi-file fixtures; do NOT extend the existing helper (it's used by 6+ tests with
  the 1-file shape).
- **Flush-timing for `refresh_semantic_graph`** — CONTEXT.md flags this open. See
  Pitfall 1 below; research recommends synchronous flush via a coalescer
  `FlushNow(ctx, ws)` method exposed through `LiveAccessor`, NOT a polling fallback
  (polling is fragile and adds latency on the hot path).

### Deferred Ideas (OUT OF SCOPE)

- Per-projection incremental refresh
- Streaming refresh progress over MCP
- Cross-snapshot path-delta accessor
- Bench-CI integration (`feedback_no_ci_benchmarks` memory)
- Coalescer-side in-memory LRU in front of the seam

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REFRESH-01 | `collectCandidatePaths` consults overlay-drain seam for incremental mode | `OverlayChangedPathsSince` accessor + buildFn `LatestCommittedSnapshot` baseline-epoch pivot + `collectCandidatePaths` rewrite (drops the `_ string` mode placeholder) |
| REFRESH-02 | 10k-symbol single-file incremental refresh p95 ≤ 200ms locally | Bench harness in `internal/eval/runner/bench_refresh_incremental_test.go`; extends `makeFixtureFacts` to multi-file; `t.Skip` on CI |
| REFRESH-03 | Fallback path verified — empty seam → full-walk + bounded-label log | `refresh_incremental_test.go` with 3 sub-tests (cold_start, overlay_rotated, empty_overlay); `helix_incremental_refresh_fallback_total` metric registered in `internal/obs/metrics.go` |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| `OverlayChangedPathsSince` SQL accessor | semantic store (`internal/semantic/store/`) | — | DuckDB read-path; D-09 boundary; no kernel deps |
| `base_overlay_epoch` schema delta + write | semantic store migration + `CommitSnapshot` | — | Same package as schema; `CurrentOverlayEpoch` already reachable |
| `collectCandidatePaths` rewrite | daemon wiring (`internal/daemon/`) | semantic store (read) | Bundle owns buildFn; calls into `*Store` accessor |
| `files_updated` honesty fix | semantic skill (`internal/skill/semantic/tools_refresh.go`) | semantic store (read) | Handler already has `StoreAccessor`; extend the interface |
| Fallback metric registration | observability (`internal/obs/metrics.go`) | daemon wiring (emission) | Mirrors `LiveFileFactDiff` pattern (Phase 68 D-07) |
| Test fixtures + bench harness | eval-runner (`internal/eval/runner/`) | semantic skill (E2E) | Matches success criterion #4 verbatim |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `database/sql` | 1.25.1 | DuckDB queries via `*Store.db` | Existing pattern (`CurrentOverlayEpoch`, `GetLatestFileFact`) [VERIFIED] |
| `marcboeker/go-duckdb` | per go.mod | DuckDB CGO driver | Same driver every Phase 57+ accessor uses [VERIFIED] |
| `prometheus/client_golang` | per go.mod | `CounterVec` for fallback metric | Owned by `internal/obs/metrics.go`; do NOT register vectors outside `obs` (line 1-19 doc) [VERIFIED] |
| `log/slog` | stdlib | Bounded-label warn log | Existing convention; `slog.Default().Warn(..., "reason", ...)` [VERIFIED in compactor.go] |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` + `t.TempDir()` | stdlib | DuckDB-on-disk fixtures | All store tests; `effective_graph_test.go` pattern |
| `os.Getenv("CI")` | stdlib | Bench skip gate | Project rule `feedback_no_ci_benchmarks` |
| `time.Now()` p95 sampling | stdlib | 50+ iteration latency aggregation | Manual sampling (no `testing.B` because we want explicit p95 assertions, not allocs/op) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Persisted `base_overlay_epoch` column | In-memory map keyed by `(repoID, snapshotID)` | Lost across daemon restart → silent full-walk fallback. Same hole Phase 68 D-01 rejected. [VERIFIED via 68-CONTEXT.md] |
| Single accessor `(paths, currentEpoch, err)` | Drain-and-mark mutating accessor | Mutates on read → breaks D-09; single-consumer only → breaks D1. Rejected in CONTEXT.md D2. |
| `testing.B` benchmarks | Manual p95 sampling | `testing.B` reports allocs/op + ns/op; we need p50/p95 latency assertions. Manual sampling matches the project pattern used by `TestRunQuickFullFixtureSetWallTime` (line 18-65). |

**Installation:** No new dependencies. All work is internal package + 1 new
`obs.Metrics` field + 1 schema migration.

**Version verification:**
```bash
go list -m github.com/marcboeker/go-duckdb
go list -m github.com/prometheus/client_golang
```
Run before plan-writing if dependency versions matter; this phase touches no
new imports.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌────────────────────────────────────┐
                         │  refresh_semantic_graph (read+)    │
                         │  (internal/skill/semantic)         │
                         └──────────────┬─────────────────────┘
                                        │ 1. epoch0 = CurrentOverlayEpoch
                                        │ 2. live.OnWorkspaceChanged(paths)
                                        │ 3. wait for coalescer flush
                                        │ 4. paths,_ = OverlayChangedPathsSince(epoch0)
                                        │ 5. files_updated = len(paths)
                                        ▼
                         ┌────────────────────────────────────┐
                         │ semantic.live.Service (coalescer)  │ -- fire-and-forget enqueue
                         │ (internal/semantic/live/service)   │    + timer-based flush
                         └──────────────┬─────────────────────┘
                                        │ on flush: handler writes
                                        │ semantic_live_overlay_files (write_epoch=N)
                                        ▼
   ┌────────────────────────────────────────────────────────────────┐
   │ DuckDB: semantic_live_overlay_files / _symbols / _refs / _edges │
   │   PK (repo_id, path)   write_epoch UBIGINT                      │
   │   idx_overlay_files_write_epoch ON (repo_id, write_epoch)       │
   │   semantic_live_overlay_meta.current_epoch UBIGINT              │
   │   semantic_snapshots.base_overlay_epoch UBIGINT  [Phase 70 NEW] │
   └─────────────────────────┬──────────────────────────────────────┘
                             │ reads (lock-free)
                             ▼
              ┌─────────────────────────────────────────┐
              │ *Store.OverlayChangedPathsSince()  NEW  │
              │   SELECT DISTINCT path                  │
              │     FROM semantic_live_overlay_files    │
              │    WHERE repo_id=? AND write_epoch>?    │
              │   + SELECT current_epoch FROM _meta     │
              └──────────────┬──────────────────────────┘
                             │
              ┌──────────────┴──────────────────────────┐
              │                                         │
              ▼                                         ▼
  ┌─────────────────────────┐         ┌───────────────────────────┐
  │ index_semantic_graph    │         │ refresh_semantic_graph    │
  │ buildFn (mode=incr)     │         │ tools_refresh.go:130-237  │
  │ collectCandidatePaths   │         │ files_updated = len(paths)│
  │   → seam or full-walk   │         └───────────────────────────┘
  │   + fallback metric     │
  └─────────────────────────┘
            │ commits new snapshot with
            │ base_overlay_epoch = CurrentOverlayEpoch
            ▼
       semantic_snapshots row
```

### Recommended Project Structure

```
internal/semantic/store/
├── overlay.go                   # NEW: OverlayChangedPathsSince alongside CurrentOverlayEpoch (line ~1040)
├── snapshot.go                  # MODIFIED: BeginSnapshot accepts/persists base_overlay_epoch; CommitSnapshot stamps it
├── migrations.go                # MODIFIED: applyMigration006 / schema6Statements adding base_overlay_epoch column
├── migrations_registry.go       # MODIFIED: append {From:5, To:6, Apply: applyMigration006}
├── migrations_types.go          # MODIFIED: CurrentSchemaVersion = 6
└── overlay_test.go              # MODIFIED: TestOverlayChangedPathsSince (4 sub-tests: cold-start, single-change, multi-epoch, rotation)

internal/obs/
└── metrics.go                   # MODIFIED: IncrementalRefreshFallback *CounterVec + IncrementalRefreshFallbackInc helper

internal/daemon/
└── semantic_wiring.go           # MODIFIED: collectCandidatePaths rewrite; refresh-degraded annotation removed; buildFn captures+passes base_overlay_epoch

internal/skill/semantic/
├── accessors.go                 # MODIFIED: StoreAccessor adds OverlayChangedPathsSince + CurrentOverlayEpoch
└── tools_refresh.go             # MODIFIED: files_updated derives from seam result (line 170-176 swap)

internal/eval/runner/
├── refresh_incremental_test.go         # NEW: REFRESH-03 (4 sub-tests; race-clean)
└── bench_refresh_incremental_test.go   # NEW: REFRESH-02 (10k symbols, p95 ≤ 200ms, t.Skip on CI)
```

### Pattern 1: `*Store` Lock-Free Read Accessor (mirrors `CurrentOverlayEpoch`)

**What:** A `*Store` method that runs a bounded `SELECT` outside any tx, returns
`(zero, nil)` on empty, wraps non-`ErrNoRows` errors.

**When to use:** Any read accessor consumed by tools or buildFns; D-09 invariant
(no `Begin/Commit/Abort/Write`).

**Example (the new accessor):**
```go
// Source: pattern from internal/semantic/store/overlay.go:1019-1038
// (CurrentOverlayEpoch) + internal/semantic/store/filefact_accessor.go:75-94
// (GetLatestFileFact dispatch)
func (s *Store) OverlayChangedPathsSince(
    ctx context.Context, repoID string, baseEpoch uint64,
) (paths []string, currentEpoch uint64, err error) {
    if s == nil || s.db == nil {
        return nil, 0, fmt.Errorf("OverlayChangedPathsSince: nil store")
    }
    if repoID == "" {
        return nil, 0, fmt.Errorf("OverlayChangedPathsSince: empty repoID")
    }

    // Read current_epoch first so a concurrent overlay tx that commits between
    // the two reads is observed in the SAME direction (epoch only ever moves
    // forward, so currentEpoch >= last write_epoch we surface).
    if err = s.db.QueryRowContext(ctx, `
        SELECT current_epoch FROM semantic_live_overlay_meta
         WHERE repo_id = ?
    `, repoID).Scan(&currentEpoch); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, 0, nil // no overlay activity for this repo
        }
        return nil, 0, fmt.Errorf("OverlayChangedPathsSince(%q): epoch read: %w", repoID, err)
    }

    rows, err := s.db.QueryContext(ctx, `
        SELECT DISTINCT path
          FROM semantic_live_overlay_files
         WHERE repo_id = ? AND write_epoch > ?
    `, repoID, baseEpoch)
    if err != nil {
        return nil, currentEpoch, fmt.Errorf("OverlayChangedPathsSince(%q): path query: %w", repoID, err)
    }
    defer rows.Close()

    for rows.Next() {
        var p string
        if err := rows.Scan(&p); err != nil {
            return nil, currentEpoch, fmt.Errorf("OverlayChangedPathsSince(%q): path scan: %w", repoID, err)
        }
        paths = append(paths, p)
    }
    if err := rows.Err(); err != nil {
        return nil, currentEpoch, fmt.Errorf("OverlayChangedPathsSince(%q): path rows: %w", repoID, err)
    }
    return paths, currentEpoch, nil
}
```

**Index already exists:** `idx_overlay_files_write_epoch ON semantic_live_overlay_files(repo_id, write_epoch)`
(migrations.go:527). No new index needed. [VERIFIED]

### Pattern 2: `applyMigration00N` schema delta (mirrors Phase 60 / Phase 63 migrations)

```go
// Source: internal/semantic/store/migrations.go:495-535 (applyMigration003 / schema3Statements)
func applyMigration006(ctx context.Context, db *sql.DB) error {
    stmts := schema6Statements()
    for i, stmt := range stmts {
        if _, err := db.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("applyMigration006: stmt %d (%s): %w", i+1, firstLine(stmt), err)
        }
    }
    return nil
}

func schema6Statements() []string {
    return []string{
        `ALTER TABLE semantic_snapshots ADD COLUMN base_overlay_epoch UBIGINT DEFAULT 0`,
        `INSERT INTO semantic_schema_version (version, applied_at) VALUES (6, now())`,
    }
}
```

**Bump `CurrentSchemaVersion` to 6** in `migrations_types.go:21`. **Append the registry
entry** in `migrations_registry.go:27-33`: `{From: 5, To: 6, Kind: MigrationInPlace, Apply: applyMigration006}`.
DuckDB rejects `NOT NULL DEFAULT <expr>` on `ALTER ... ADD COLUMN` — use `DEFAULT 0`
exactly like Phase 60 (see migrations.go:482-487 commentary). [VERIFIED]

### Pattern 3: `SnapshotMeta` / `CommitSnapshot` plumbing for the baseline epoch

`SnapshotMeta` (snapshot.go:86-98) already carries `CapturedEpoch uint64` for the
compactor's CAS contract. The same struct gets one more field:

```go
type SnapshotMeta struct {
    RepoID           string
    BaseSnapshotID   uint64
    CapturedEpoch    uint64
    BaseOverlayEpoch uint64 // NEW: persisted on the snapshot row at CommitSnapshot.
}
```

`BeginSnapshot` (snapshot.go:242-308) writes the row with `status='pending'`. The
choice: write `base_overlay_epoch` at `BeginSnapshot` (current_epoch may advance
during the build window, so the snapshot's baseline = epoch-at-build-START), or at
`CommitSnapshot` (baseline = epoch-at-build-END). **Recommendation: write at
`CommitSnapshot`** by adding a fresh `b.store.CurrentOverlayEpoch(ctx, repoID)` read
just before `CommitSnapshot` in buildFn (semantic_wiring.go:~1446), then pass it via
`SnapshotSummary` or a new `CommitSnapshot` arg. Rationale: the snapshot CAPTURES
overlay state at commit time, so subsequent incremental builds answer "what changed
since the last snapshot committed" correctly. CONTEXT.md D3 phrases this as "the
overlay_epoch captured at the time of the most recent committed snapshot."

### Anti-Patterns to Avoid

- **Running `OverlayChangedPathsSince` inside an existing tx** — the seam is
  intentionally lock-free; calling it from inside `BeginOverlayTx` deadlocks the
  per-workspace mutex. The handler `ForeachUpdater` is a Go-level callback, not a tx
  consumer.
- **Treating `(nil, 0, nil)` and `([], N, nil)` identically** — the first means
  "no overlay activity ever" (cold-start); the second means "no rows changed since
  baseEpoch but the workspace HAS had activity." Both fall back to full-walk, BUT
  the fallback `reason` label differs (`cold_start` vs `empty_overlay`).
- **Walking the workspace just to count `files_updated` in `refresh`** — defeats
  the whole point. `len(paths)` from the seam is the count.
- **Adding a sibling `base_overlay_epoch` table** — requires an extra JOIN per
  buildFn read, an extra INSERT at CommitSnapshot, and a second migration to backfill.
  The column option is one ALTER.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Overlay path enumeration | Custom in-memory set in coalescer | The DuckDB query in `OverlayChangedPathsSince` | DuckDB owns the persistent state; in-memory caches lose state across daemon restart (the exact hole Phase 68 D-01 rejected) |
| Path-set diffing | Recompute via `tools_index.go` walk | The seam's `WHERE write_epoch > ?` clause | The index `idx_overlay_files_write_epoch` already makes this O(log n + k) |
| Bench latency histogram | `testing.B` benchmarks | Manual p95 sampling with `sort.Float64s` + index `int(0.95*len(samples))` | We need explicit p95 assertions, not allocs/op; matches project bench pattern (`TestRunQuickFullFixtureSetWallTime`) |
| Test fixture for 10k symbols | Hand-written file generators | Parameterize `makeFixtureFacts` → `makeFixtureFactsNFiles(numFiles, symbolsPerFile)` | The existing helper already produces deterministic IDs; extending it is one loop |
| Flush-synchronization in `refresh` | Polling loop on `CurrentOverlayEpoch` | Add `LiveAccessor.FlushNow(ws) error` that blocks on the coalescer's current batch | Polling is fragile under load and adds 50ms+ tail latency; the coalescer already has a flush callback (`SetOnFlush`, coalescer.go:124) — exposing a synchronous Drain is a 20-line addition |

**Key insight:** Every piece of this phase's data state is already in DuckDB (epochs,
paths, snapshots). The "incremental" part is just choosing which rows to read. Custom
in-memory accounting is not needed and is actively harmful (restart-loss).

## Runtime State Inventory

Not applicable — this is an additive phase (new accessor, new column, new metric,
new tests). No renames, refactors, or string replacements. No data migration of
existing rows beyond the `DEFAULT 0` backfill the DuckDB ALTER performs. No
OS-registered state. No env vars. No build artifacts.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `semantic_snapshots` rows pre-Phase-70 have `base_overlay_epoch=0` (DEFAULT) | None — `baseEpoch==0` is the "cold-start" signal the seam handles by design |
| Live service config | None | — |
| OS-registered state | None | — |
| Secrets/env vars | None | — |
| Build artifacts | None — pure source addition | — |

## Common Pitfalls

### Pitfall 1: Read-after-fire-and-forget race in `refresh_semantic_graph`

**What goes wrong:** `live.Service.OnWorkspaceChanged` is **fire-and-forget**
(service/service.go:184-186). It enqueues to the per-workspace coalescer
(coalescer.go:174 `Enqueue`); the coalescer flushes on a timer, not on the call.
A naive sequence in `refresh`:
1. `epoch0 = CurrentOverlayEpoch(...)`
2. `live.OnWorkspaceChanged(ws, args.Paths)`  ← returns immediately
3. `paths, _ = OverlayChangedPathsSince(repoID, epoch0)`  ← runs before flush
4. `files_updated = 0`  ← wrong

**Why it happens:** Phase 60 designed coalescer flushes to amortize batch cost over
`max_batch_delay_ms`. The refresh tool today bypasses the issue by returning
`len(args.Paths)` (the wart we're fixing).

**How to avoid:** Add a synchronous flush API to the coalescer. The plumbing already
exists — `SetOnFlush(fn func())` registers a post-flush hook (coalescer.go:124).
Expose a `FlushNow(ctx) error` that:
- Cancels the active timer
- Runs `makeFlush(ctx)` synchronously
- Returns when the post-flush hook has fired

Wire it through `LiveAccessor.FlushNow(ws) error` (new method on the interface in
`internal/skill/semantic/accessors.go`). Daemon adapter calls
`coalescer.FlushNow(ctx)` from `internal/semantic/live/service/service.go`.

**Warning signs:** Flaky `refresh_incremental_test.go` under `-race`; non-deterministic
`files_updated == 0` reports in production telemetry; bench p95 dominated by
`max_batch_delay_ms` (which is the default, not the bug).

### Pitfall 2: `overlay_rotated` mis-classification under high churn

**What goes wrong:** The classifier for the fallback `reason` label is:
```
if baseEpoch == 0:             reason = "cold_start"
elif len(paths) == 0:
    if currentEpoch > baseEpoch: reason = "overlay_rotated"
    else:                        reason = "empty_overlay"
```
But "empty after rotation" and "empty because nothing changed AND a compaction ran
that bumped current_epoch via its own writes" are indistinguishable without an
`EarliestRetainedEpoch` accessor.

**Why it happens:** Phase 63 compaction's `Snapshot.ClearOverlayLE(WHERE write_epoch
<= capturedEpoch)` removes rows but doesn't itself bump `current_epoch` (the
captured value is the live one). However any concurrent overlay write between the
baseline and the read DOES bump current_epoch. So `currentEpoch > baseEpoch` is
loosely "things have happened since," not strictly "rows below baseEpoch were
purged."

**How to avoid:** Document this as a known approximation in the metric Help text
("reason='overlay_rotated' indicates the workspace had overlay activity since the
baseline that produced no surviving rows visible to the read, MOST commonly
compaction; could also be a write that was immediately superseded"). If the metric
proves noisy in practice, add `MinRetainedOverlayEpoch(ctx, repoID) (uint64, error)`
in a follow-up phase. Don't over-engineer in Phase 70.

**Warning signs:** Production dashboards show `overlay_rotated` ≫ `empty_overlay` —
likely means compaction is over-eager; verify with `helix_semantic_compaction_*`.

### Pitfall 3: `BeginSnapshot` vs `CommitSnapshot` baseline-epoch timing

**What goes wrong:** Capturing `base_overlay_epoch` at `BeginSnapshot` time means a
new incremental build can race a live edit that happens during the build window:
the live edit bumps current_epoch, lands in overlay, but is NOT in the
just-committed snapshot. The NEXT incremental build with baseline = BeginSnapshot
epoch DOES see the row → re-processed correctly. The next baseline = CommitSnapshot
epoch MISSES the row → silently dropped.

**Why it happens:** The contract "incremental processes everything that changed
since the LAST SNAPSHOT" requires the baseline equal the overlay state visible at
commit time, not start time.

**How to avoid:** Capture `BaseOverlayEpoch = b.store.CurrentOverlayEpoch(ctx, repoID)`
**immediately before** `b.store.CommitSnapshot(ctx, snap, summary)` in buildFn
(semantic_wiring.go:1446). Pass via an extended `SnapshotSummary` or a new
`CommitSnapshotWithEpoch` method. Write it on the snapshot row in the same tx as
the status flip so it lands atomically.

**Warning signs:** A test that edits a file DURING an incremental build (between
`paths := collectCandidatePaths(...)` and `CommitSnapshot`) and verifies the next
incremental build still processes that file; the bug = the file is skipped.

### Pitfall 4: D-09 invariant violation via `vet-nokernel2semantic`

**What goes wrong:** The new accessor signature accidentally imports a kernel
package, or the metric helper in `obs` adds a kernel dep, breaking the boundary.

**Why it happens:** Refactor convenience; reaching for `kernel.SomeType` because
it's "close enough."

**How to avoid:** Keep the accessor signature in `internal/semantic/store/`-only
types: `context.Context`, `string`, `uint64`, `[]string`, `error`. The fallback
metric in `internal/obs/` already lives outside both kernel and semantic packages —
fine. Run `make vet-nokernel2semantic` (Makefile:21,43) as part of the Wave 0 gate.

**Warning signs:** `vet-nokernel2semantic` failure on the new file; that vet target
is fast (single binary, runs in <1s).

### Pitfall 5: Bench fixture inflates DuckDB checkpoint cost

**What goes wrong:** Generating 10k symbols by inserting 10k rows in 200 files at
test setup time triggers DuckDB CHECKPOINT and inflates wall time before the
incremental refresh even runs.

**Why it happens:** `WriteSnapshotFacts` writes one row per `INSERT` (snapshot.go:326);
no batch insert path exists today.

**How to avoid:** Build the 10k-symbol fixture ONCE in `TestMain` or a `sync.Once`
wrapper, then reset only the overlay state between bench iterations (call
`Snapshot.ClearOverlayLE` or its read-path equivalent). The committed snapshot is
re-used across all 50+ iterations.

**Warning signs:** Bench p95 grows linearly with iteration count → fixture is being
rebuilt; first iteration is 10× slower than the rest → checkpoint-on-first-write.

## Code Examples

### `collectCandidatePaths` rewrite

```go
// Source: rewrite of internal/daemon/semantic_wiring.go:1466-1520
//
// Old behavior (line 1479): func collectCandidatePaths(ws, _ string) — ignores mode.
// New behavior: dispatch on mode; consult OverlayChangedPathsSince for incremental;
// fall back to full-walk with bounded-label metric.

func (b *semanticBundle) collectCandidatePaths(ctx context.Context, ws workspace.WorkspaceKey, mode string, baseEpoch uint64) []string {
    if ws.RepoRoot == "" {
        return nil
    }
    if mode != "incremental" {
        return b.fullWalkPaths(ws)
    }

    repoID := ws.Hash()
    paths, currentEpoch, err := b.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)
    if err != nil {
        b.logger.Warn("collectCandidatePaths: overlay seam error; falling back to full-walk",
            "repo", repoID, "err", err)
        b.metrics.IncrementalRefreshFallbackInc("error", repoID)
        return b.fullWalkPaths(ws)
    }

    if len(paths) > 0 {
        // HOT PATH: seam returned a non-empty change set. Resolve absolute paths
        // from workspace-relative if needed (overlay rows store the form that
        // BeginOverlayTx wrote — confirm in Wave 0 explore whether they are
        // workspace-relative or absolute; the existing classifier accepts both).
        return paths
    }

    // Empty set ⇒ classify why and fall back.
    reason := "empty_overlay"
    switch {
    case baseEpoch == 0:
        reason = "cold_start"
    case currentEpoch > baseEpoch:
        reason = "overlay_rotated"
    }
    b.logger.Warn("collectCandidatePaths: incremental fell back to full-walk",
        "repo", repoID, "reason", reason,
        "base_epoch", baseEpoch, "current_epoch", currentEpoch)
    b.metrics.IncrementalRefreshFallbackInc(reason, repoID)
    return b.fullWalkPaths(ws)
}

// fullWalkPaths is the existing filepath.WalkDir body extracted verbatim from
// semantic_wiring.go:1480-1519. No behavior change.
func (b *semanticBundle) fullWalkPaths(ws workspace.WorkspaceKey) []string {
    var paths []string
    _ = filepath.WalkDir(ws.RepoRoot, func(path string, d fs.DirEntry, walkErr error) error {
        // ... (unchanged from current implementation)
    })
    return paths
}
```

### `tools_refresh.go` `files_updated` fix

```go
// Source: replacement for internal/skill/semantic/tools_refresh.go:170-176

// Old (line 176):
//   filesUpdated := len(args.Paths)
//
// New:
// Capture pre-drain overlay epoch.
preEpoch, _ := s.store.CurrentOverlayEpoch(ctx, ws.Hash())

// Drain pending live changes via LiveAccessor (D-11).
if s.live != nil {
    if err := s.live.OnWorkspaceChanged(ws, args.Paths); err != nil {
        return errorResult(err.Error())
    }
    // Pitfall 1 mitigation: synchronously wait for the coalescer flush.
    if err := s.live.FlushNow(ctx, ws); err != nil {
        // Non-fatal: stale read just means a slightly low files_updated count.
        // The user's edit still lands; the next refresh call sees it.
    }
}

// Query the seam for paths that landed since the pre-drain epoch.
changed, _, _ := s.store.OverlayChangedPathsSince(ctx, ws.Hash(), preEpoch)
filesUpdated := len(changed)
```

### Fallback metric registration (mirrors `LiveFileFactDiff`, Phase 68 D-07)

```go
// Source: addition to internal/obs/metrics.go (mirrors lines 346-354)

IncrementalRefreshFallback *prometheus.CounterVec

// In newMetrics():
IncrementalRefreshFallback: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_incremental_refresh_fallback_total",
        Help: "Incremental refresh fell back to full-walk; reason is one of cold_start|overlay_rotated|empty_overlay|error. Phase 70.",
    },
    []string{"reason", "repo"},
),

// Helper (mirrors LiveFileFactDiffInc, line 768):
func (m *Metrics) IncrementalRefreshFallbackInc(reason, repo string) {
    if m == nil || m.IncrementalRefreshFallback == nil {
        return
    }
    switch reason {
    case "cold_start", "overlay_rotated", "empty_overlay", "error":
    default:
        return // drop unknown — bounded-cardinality discipline
    }
    m.IncrementalRefreshFallback.WithLabelValues(reason, repo).Inc()
}
```

Also: register the metric in the `m.registry.MustRegister(...)` block around
metrics.go:526, and **add the carve-out** in
`internal/obs/metrics_labels_test.go` for the `"reason"` label name (the existing
allowlist `AllowedLabels` is `{"tool_name","profile","mode","language","outcome","lane"}`
and does NOT include `"reason"`).

### Test surface (REFRESH-03 correctness)

```go
// Source: new file internal/eval/runner/refresh_incremental_test.go
// Race-clean (default -race in CI).

func TestRefreshIncremental_SingleFileChanged_OnlyThatFileTouched(t *testing.T) {
    // Setup: 3-file workspace, commit baseline snapshot.
    h := newRefreshHarness(t, 3 /*files*/, 5 /*symbolsPerFile*/)
    h.indexFull(t)

    // Edit file[1].
    h.editFile(t, 1, "// new content\n")

    // Run refresh; capture candidate paths via test-seam hook.
    var captured []string
    h.bundle.SetCollectCandidatePathsHook(func(p []string) { captured = p })
    h.refresh(t, nil /*paths*/)

    if len(captured) != 1 || !strings.HasSuffix(captured[0], "file_1.go") {
        t.Errorf("expected exactly 1 candidate (file_1.go), got %v", captured)
    }

    // Negative: full-walk did NOT run.
    if h.metrics.IncrementalRefreshFallbackCount() != 0 {
        t.Errorf("fallback metric incremented; full-walk should NOT have run")
    }
}

func TestRefreshIncremental_Fallback_ColdStart(t *testing.T) {
    h := newRefreshHarness(t, 3, 5)
    // No prior snapshot → baseEpoch=0 → seam returns paths but reason=cold_start.
    h.refresh(t, nil)
    if got := h.metrics.IncrementalRefreshFallbackLabel("cold_start"); got != 1 {
        t.Errorf("reason=cold_start: got %d, want 1", got)
    }
}

func TestRefreshIncremental_Fallback_OverlayRotated(t *testing.T) { /* ... */ }
func TestRefreshIncremental_Fallback_EmptyOverlay(t *testing.T)   { /* ... */ }
```

### Bench (REFRESH-02)

```go
// Source: new file internal/eval/runner/bench_refresh_incremental_test.go

func TestBench_RefreshIncremental_10kSymbols_P95Under200ms(t *testing.T) {
    if os.Getenv("CI") != "" {
        t.Skip("bench is local-only per project rule feedback_no_ci_benchmarks")
    }
    if testing.Short() {
        t.Skip("bench skipped under -short")
    }

    h := newRefreshHarness(t, 200 /*files*/, 50 /*symbolsPerFile*/) // 10k symbols total
    h.indexFull(t)

    const iterations = 50
    samples := make([]time.Duration, 0, iterations)
    for i := 0; i < iterations; i++ {
        h.editFile(t, i%200, fmt.Sprintf("// iter %d\n", i))
        start := time.Now()
        h.refresh(t, nil)
        samples = append(samples, time.Since(start))
    }

    sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
    p95 := samples[int(0.95*float64(len(samples)))]
    t.Logf("p50=%v p95=%v p99=%v", samples[len(samples)/2], p95, samples[len(samples)-1])
    if p95 > 200*time.Millisecond {
        t.Errorf("p95 %v exceeds 200ms budget", p95)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `collectCandidatePaths` ignores `mode` → full-walk for incremental | Dispatch on `mode`, consult seam | Phase 70 | REFRESH-01 closure; ~10× speedup on 10k-symbol workspace per success criterion |
| `refresh.files_updated = len(args.Paths)` | `files_updated = len(seam-returned paths)` | Phase 70 | Envelope honesty; users get real counts |
| In-memory baseline-epoch maps proposed in early DIFF/STATUS discussions | Persisted column on `semantic_snapshots` | Phase 68 D-01 established the pattern | Restart-safe; no silent full-walks after daemon bounce |

**Deprecated/outdated:**
- The "refresh-degraded" multi-paragraph comment block at semantic_wiring.go:1466-1480 —
  replaced by a one-line pointer to the seam (D6).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Overlay rows store paths in a form the buildFn's classifier accepts directly (i.e., either absolute or workspace-relative, but consistent across producer + consumer) | `collectCandidatePaths` rewrite | Plan must include a Wave 0 explore step to verify; if mismatch, add a `filepath.Join(ws.RepoRoot, p)` translation in the seam consumer. [ASSUMED — needs `internal/semantic/live/handler/handler.go` read in Wave 0] |
| A2 | `Coalescer` exposes (or can cheaply expose) a synchronous `FlushNow(ctx)` building on `SetOnFlush` (coalescer.go:124) | Pitfall 1 mitigation | If not feasible without re-entry deadlock, fall back to polling `CurrentOverlayEpoch` until it advances past `preEpoch`, capped at `max_batch_delay_ms + 200ms`. [ASSUMED — needs coalescer.go full read in Wave 0] |
| A3 | The 10k-symbol fixture fits in a single committed snapshot within typical macOS/Linux laptop memory + DuckDB defaults | Bench harness | If memory-tight, split into per-iteration fixtures; bench p95 would still be valid for the incremental path. [ASSUMED — needs local dry run] |
| A4 | DuckDB ALTER TABLE ADD COLUMN UBIGINT DEFAULT 0 succeeds on existing `semantic_snapshots` rows without table rewrite | Pattern 2 | If table-rewrite required, migration becomes Kind=MigrationReindex per migrations_types.go:33 — much larger blast radius. [ASSUMED — Phase 60 D-04 added 4 such columns successfully, so HIGH confidence] |
| A5 | Bench can rely on `editFile` mutating workspace + driving live.Service to land an overlay row without subprocess plumbing | Bench harness | If the in-process eval-runner doesn't wire `live.Service` to its workspace, the bench needs explicit `live.Service.OnEdit` calls. [ASSUMED — needs eval-runner inprocess.go read in Wave 0] |

## Open Questions

1. **Is the live overlay path form workspace-relative or absolute?**
   - What we know: `UpsertOverlayFile` (overlay.go:387) stores `path` as a column;
     overlay rows have PK `(repo_id, path)`.
   - What's unclear: Whether path normalization happens before insert; whether the
     handler writes the absolute path or a workspace-relative one.
   - Recommendation: Wave 0 explore — read `internal/semantic/live/handler/handler.go`
     and `OverlayTx.UpsertOverlayFile` call sites. Plan task 1 reads + documents.

2. **Does `Coalescer.FlushNow(ctx)` admit a clean synchronous implementation?**
   - What we know: `SetOnFlush(fn func())` hook exists (coalescer.go:124-132).
   - What's unclear: Whether the timer-cancel + manual-flush dance is re-entrant safe
     under concurrent `Enqueue` calls.
   - Recommendation: Wave 0 explore — read `coalescer.go` end-to-end. If non-trivial,
     fall back to bounded polling (Assumption A2 fallback).

3. **Should the `repo` label use `ws.Hash()` or a shortened form?**
   - What we know: `LiveFileFactDiff` uses bounded `repo` per Phase 68 D-07; the
     existing convention is `workspace_label` (bounded, hashed; see metrics.go:80-81).
   - What's unclear: Whether `repo` and `workspace_label` are the same value or differ.
   - Recommendation: Mirror Phase 68 verbatim — same hash, same length cap. Plan task
     should reference the exact helper used by `LiveFileFactDiffInc`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All build/test | ✓ | 1.25.1 | — |
| DuckDB (via go-duckdb CGO) | Store accessor tests | ✓ | per go.mod | — |
| `make vet-nokernel2semantic` | D-09 boundary check | ✓ | local install (`go install ./cmd/vet-nokernel2semantic`) | — |
| Prometheus client_golang | New metric | ✓ | per go.mod | — |

**No missing dependencies.** All work is pure Go + existing CGO setup.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external test framework) |
| Config file | none (table-driven tests; harness helpers per-package) |
| Quick run command | `go test ./internal/semantic/store/ ./internal/skill/semantic/ ./internal/daemon/ ./internal/obs/ -race -run 'Overlay|Refresh|Snapshot' -count=1` |
| Full suite command | `go test ./... -race -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REFRESH-01 (accessor) | `OverlayChangedPathsSince` returns paths with `write_epoch > base`, plus `currentEpoch` | unit | `go test ./internal/semantic/store/ -race -run TestOverlayChangedPathsSince -count=1` | ❌ Wave 0 |
| REFRESH-01 (schema) | Migration 5→6 adds `base_overlay_epoch UBIGINT DEFAULT 0` cleanly | unit | `go test ./internal/semantic/store/ -race -run TestMigration006 -count=1` | ❌ Wave 0 |
| REFRESH-01 (buildFn) | `collectCandidatePaths(mode="incremental")` calls seam; falls back on empty | integration | `go test ./internal/daemon/ -race -run TestCollectCandidatePaths_Incremental -count=1` | ❌ Wave 0 |
| REFRESH-01 (refresh) | `tools_refresh` files_updated reflects seam count, not `len(args.Paths)` | integration | `go test ./internal/skill/semantic/ -race -run TestRefresh_FilesUpdated_FromSeam -count=1` | ❌ Wave 0 |
| REFRESH-02 (bench) | 10k symbols + 1-file edit → p95 ≤ 200ms locally | bench | `go test ./internal/eval/runner/ -run TestBench_RefreshIncremental_10kSymbols_P95Under200ms -count=1` | ❌ Wave 0 |
| REFRESH-03 (correctness) | Single-file change → 1 path. Empty seam → full-walk + metric increment with correct `reason` label | e2e | `go test ./internal/eval/runner/ -race -run TestRefreshIncremental -count=1` | ❌ Wave 0 |
| D-09 invariant | `tools_refresh.go` does NOT add `Begin/Commit/Abort/Write` tokens | grep gate | `grep -E '\b(Begin|Commit|Abort|Write)Snapshot\b' internal/skill/semantic/tools_refresh.go && exit 1 || exit 0` | ✅ (existing) |
| Boundary | `internal/semantic/store/` does not import kernel | vet | `make vet-nokernel2semantic` | ✅ (existing) |
| Metric label allowlist | New `reason` label declared in carve-out | unit | `go test ./internal/obs/ -race -run TestMetricsLabelsAllowlist -count=1` | ✅ (existing — needs update to allow `reason`) |

### Sampling Rate

- **Per task commit:** `go test ./internal/semantic/store/ ./internal/daemon/ ./internal/skill/semantic/ ./internal/obs/ -race -count=1` (~15s)
- **Per wave merge:** `go test ./... -race -count=1` + `make vet-nokernel2semantic` + `go vet ./...`
- **Phase gate:** Full suite green + bench run locally (≥ 1 run with p95 reported) before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/semantic/store/overlay_test.go` — extend with `TestOverlayChangedPathsSince` (4 sub-tests)
- [ ] `internal/semantic/store/migrations_test.go` — extend with `TestMigration006_BaseOverlayEpochColumn`
- [ ] `internal/daemon/semantic_wiring_test.go` — `TestCollectCandidatePaths_Incremental*` (new file or extend existing)
- [ ] `internal/skill/semantic/tools_refresh_test.go` — `TestRefresh_FilesUpdated_FromSeam`
- [ ] `internal/eval/runner/refresh_incremental_test.go` — REFRESH-03 (NEW file)
- [ ] `internal/eval/runner/bench_refresh_incremental_test.go` — REFRESH-02 (NEW file)
- [ ] `internal/obs/metrics_labels_test.go` — extend allowlist to permit `reason` label on `helix_incremental_refresh_fallback_total`
- [ ] Coalescer flush API decision (Assumption A2) — explore + implement before Wave 1 wires `tools_refresh.go`

No framework install needed — Go stdlib `testing` plus the existing harness helpers
(`newE2EHarness`, `makeFixtureFacts`).

## Security Domain

Not applicable for this phase. No new untrusted input surfaces, no auth/authz
changes, no cryptography. The new accessor reads from a process-local DuckDB file
with the same trust boundary as every other `*Store` method. Path validation in
`refresh_semantic_graph` (T-64-05-01 mitigation, tools_refresh.go:140) is unchanged
and still applies to `args.Paths`. SMTC's `java-security` capability is not
applicable to this Go codebase per CLAUDE.md guidance.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — (no auth surface) |
| V3 Session Management | no | — |
| V4 Access Control | no | mode-tier check already enforced at tools_refresh.go:134; unchanged |
| V5 Input Validation | no | `validatePaths` (tools_refresh.go:140); unchanged |
| V6 Cryptography | no | — |

No new threat patterns introduced — the seam is read-only over an existing trust
boundary.

## Project Constraints (from CLAUDE.md)

- **`go vet ./...` and `go test ./...`** before completing any Go task — verification
  steps in every plan must include these.
- **SMTC-first tool routing** for code intelligence — applies to research/discovery,
  not to runtime behavior of this phase. The new accessor is a DuckDB read; SMTC is
  unrelated.
- **GSD workflow enforcement** — all edits routed through `/gsd:execute-phase`. Plans
  must be executed via the GSD harness, not direct edits.
- **No CI benchmarks** (`feedback_no_ci_benchmarks` memory + REQUIREMENTS.md "Out of
  Scope" table) — the bench MUST `t.Skip` when `os.Getenv("CI") != ""`. The plan
  cannot add a CI workflow that runs the bench.
- **No package-manager distribution** — not applicable; this phase ships no binaries.
- **Single Go binary, no Python** — applicable: all work is Go.
- **D-09 / D-13 invariants on `tools_refresh.go`** (CLAUDE.md "Key Patterns" +
  CONTEXT.md D7) — the file MUST NOT acquire `Begin/Commit/Abort/Write` snapshot
  tokens. Plans MUST include the existing grep gate as a verification step.
- **`vet-nokernel2semantic` boundary** — new `*Store` accessor lives in
  `internal/semantic/store/`; no kernel imports.

## Sources

### Primary (HIGH confidence — read in-session)

- `internal/semantic/store/overlay.go` (lines 220-280, 1004-1054) — `OverlayHasPendingRows`,
  `OverlayRowCount`, `CurrentOverlayEpoch`, `LockOverlayWorkspace` reference patterns
- `internal/semantic/store/filefact_accessor.go` (full file, 235 lines) — Phase 68 accessor
  template for the new accessor's signature, error wrapping, and test shape
- `internal/semantic/store/snapshot.go` (lines 86-308, 408-460) — `SnapshotMeta`,
  `BeginSnapshot`, `CommitSnapshot` plumbing for `base_overlay_epoch`
- `internal/semantic/store/migrations.go` (lines 67-100, 295-373, 472-535) — schema
  shape, applyMigration003 pattern for adding columns + indexes, applyMigration004 +
  005 simpler patterns
- `internal/semantic/store/migrations_registry.go` (lines 27-33) — registry append point
- `internal/semantic/store/migrations_types.go` — `CurrentSchemaVersion = 5` (bump to 6)
- `internal/semantic/store/effective_graph.go` (lines 379-403) — `LatestCommittedSnapshot`
  signature + test shape
- `internal/daemon/semantic_wiring.go` (lines 1369-1520) — buildFn structure +
  `collectCandidatePaths` current implementation + refresh-degraded annotation
- `internal/skill/semantic/tools_refresh.go` (full file) — D-09/D-13 invariant block;
  files_updated wart at line 176; handler structure to extend
- `internal/skill/semantic/integration_test.go` (lines 325-481) — `makeFixtureFacts`,
  `newE2EHarness`, `TestE2E_IndexThenContext_SymbolCount` (the Phase 64 P07 fixture
  reuse anchor)
- `internal/semantic/live/service/service.go` (lines 170-232) — `OnWorkspaceChanged`
  fire-and-forget semantics (Pitfall 1 root cause)
- `internal/semantic/live/coalescer/coalescer.go` (lines 124-300) — `SetOnFlush` hook
  for the synchronous-flush solution
- `internal/obs/metrics.go` (lines 1-200, 340-410, 750-790) — `LiveFileFactDiff` + 
  `LiveFileFactDiffInc` patterns + `AllowedLabels` carve-out discipline
- `internal/eval/runner/inprocess_fixtures_test.go` + `inprocess.go` — bench wall-time
  pattern (manual sampling, `t.Skip` discipline)
- `Makefile` (lines 21, 43-44) — `vet-nokernel2semantic` install target
- `.planning/phases/70-incremental-refresh-overlay-drain/70-CONTEXT.md` — locked decisions
- `.planning/REQUIREMENTS.md` (lines 36-38) — REFRESH-01/02/03 acceptance
- `.planning/ROADMAP.md` (lines 198-208) — Phase 70 success criteria
- `.planning/phases/68-precise-filefactdiff-populator/` — Phase 68 D-01/D-02/D-07/D-08
  precedent patterns for store accessor + bounded-label metric

### Secondary (MEDIUM confidence)

- `.planning/codebase/CONVENTIONS.md` (referenced but not fully read in-session) —
  D-09 / D-13 invariants assumed to mirror what's quoted in 70-CONTEXT.md and
  tools_refresh.go header comments

### Tertiary (LOW confidence — flagged in Assumptions Log)

- Assumed: overlay row path normalization (A1); coalescer FlushNow feasibility (A2);
  10k-fixture memory profile (A3); ALTER TABLE row-rewrite behavior (A4); eval-runner
  live-service wiring (A5)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every pattern verified against in-tree source
- Architecture: HIGH — accessor + migration + metric all mirror established Phase 60 / 63 / 68 templates
- Pitfalls: MEDIUM — flush-timing (Pitfall 1) is the highest-risk item; needs Wave 0 verification
- Test surface: HIGH — fixture builder exists and is parameterizable

**Research date:** 2026-05-15
**Valid until:** 2026-06-14 (30 days; stable subsystem)

## Open Questions (RESOLVED 2026-05-15)

### A1 — Overlay path form (RESOLVED: ABSOLUTE)

**Question:** Are overlay rows written with absolute or workspace-relative paths?

**Resolution:** **Absolute paths.** The fsnotify watcher receives `ev.Name` as the
absolute path that was passed to `fsnotify.Watcher.Add(absRoot)`; this value is
recorded into the pending set verbatim
(`internal/semantic/live/watcher/watcher.go:225` — `w.pending[ev.Name] = struct{}{}`)
and emitted via `live.WorkspaceChangeSignal.Paths` at watcher.go:289. The producer
(`Service.OnWorkspaceChanged` at `internal/semantic/live/service/service.go:184`)
passes the slice through unchanged into the per-workspace coalescer. The handler's
`UpdateChangedFile` consumes the same string and writes it directly into the
overlay row via `tx.UpsertOverlayFile(ctx, path, hash)`
(`internal/semantic/live/handler/handler.go:417`). No normalization step exists on
the producer or consumer side. **Citation chain:** watcher.go:225 → service.go:184
→ handler.go:392-417.

**Consequence for Plan 04:** `OverlayChangedPathsSince` returns absolute paths.
`collectCandidatePaths` for `mode="incremental"` may return them as-is; the
existing classifier (`fullWalkPaths`'s consumers in buildFn, semantic_wiring.go)
already operates on the absolute paths that `filepath.WalkDir(ws.RepoRoot, …)`
produces, so the two code paths agree on shape. No `filepath.Join` translation
is needed. Plan 04 Task 2's `<read_first>` MUST include `handler.go` so the
implementer confirms this invariant before committing.

**Consequence for the test surface (Plan 06):** `editFile` in the harness must
fire `OnWorkspaceChanged` with absolute paths (constructed via
`filepath.Join(h.tempdir, "src/file_N.go")`) so the overlay row's PK matches what
the seam later returns.

### A2 — Coalescer.FlushNow re-entrancy (RESOLVED: SAFE)

**Question:** Can `Coalescer.FlushNow(ctx)` be implemented synchronously without
deadlocking against the coalescer's own mutex/queue under concurrent `Enqueue`?

**Resolution:** **Yes, safely.** Three observations from a full read of
`internal/semantic/live/coalescer/coalescer.go` (316 lines):

1. **`makeFlush` releases `c.mu` before dispatching** (lines 248-273): it locks,
   snapshots `c.pending` into a local slice, resets state, stops `c.maxTimer`,
   then `c.mu.Unlock()` BEFORE the `CoalesceEvents` + dispatch loop. The post-flush
   hook (`fireOnFlush`, lines 299-306) takes only `c.onFlushMu`, never `c.mu`.

2. **`Enqueue` does not take `c.mu`** (lines 174-183): it is a non-blocking
   channel send (`c.in <- ev`). Concurrent producers cannot deadlock against a
   FlushNow caller that briefly holds `c.mu` for the snapshot step.

3. **`accept` takes `c.mu`** (lines 214-244) but runs only on the `Run` goroutine
   (channel receive at line 209). Run is serialized with FlushNow only insofar as
   both can attempt to acquire `c.mu`; standard mutex semantics apply — no
   deadlock. The risk surface is "FlushNow runs while Run is in the middle of
   `accept`" → FlushNow blocks briefly, then proceeds.

**Implementation pattern for Plan 03 Task 1 (LOCKED):**

```go
func (c *Coalescer) FlushNow(ctx context.Context) error {
    if c == nil {
        return nil
    }
    if err := ctx.Err(); err != nil {
        return err
    }
    c.mu.Lock()
    // Cancel both timers so they do not double-fire on the same batch.
    if c.timer != nil { c.timer.Stop(); c.timer = nil }
    if c.maxTimer != nil { c.maxTimer.Stop(); c.maxTimer = nil }
    if len(c.pending) == 0 {
        c.mu.Unlock()
        c.lastFlushNanos.Store(time.Now().UnixNano())
        c.fireOnFlush()
        return nil
    }
    c.lastFlushNanos.Store(time.Now().UnixNano())
    snapshot := make([]live.SourceChangeEvent, 0, len(c.pending))
    for _, v := range c.pending { snapshot = append(snapshot, v) }
    c.pending = make(map[string]live.SourceChangeEvent)
    c.mu.Unlock()

    merged := CoalesceEvents(snapshot, c.cfg.BulkChangeThreshold)
    for _, ev := range merged {
        if err := c.handler.Dispatch(ctx, ev); err != nil {
            c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "error")
            c.logger.Warn("coalescer FlushNow: dispatch error",
                "workspace", c.workspaceID, "path", ev.Path, "err", err)
        } else {
            c.metrics.SemanticLiveUpdatesInc(string(ev.Kind), "applied")
        }
    }
    c.fireOnFlush()
    return nil
}
```

This duplicates `makeFlush`'s body verbatim with two differences: (a) takes `ctx`
as an argument rather than capturing one at closure-construction time, (b) cancels
timers up-front to prevent the closure-captured `flush` from re-firing on the same
batch. The duplication is acceptable; a follow-up refactor can extract `flushLocked`
if desired (Plan 03 Task 1 action step 2 notes "reuse if already present").

**Citations:** coalescer.go:174-183 (Enqueue non-blocking), coalescer.go:214-244
(accept under mu), coalescer.go:246-294 (makeFlush mu-release-before-dispatch),
coalescer.go:299-306 (fireOnFlush separate mutex).

### A3 — `repo` label form (RESOLVED: `string(repoID)` == `ws.Hash()`)

**Question:** Should the `repo` label on `helix_incremental_refresh_fallback_total`
use `ws.Hash()`, raw workspace path, or some shortened/`workspace_label` form?

**Resolution:** **Use `string(repoID)`, which IS `ws.Hash()`.** Trace:

1. `LiveFileFactDiffInc(tier, repo string)` declares the label as `"repo"` and the
   parameter name is `repo` (`internal/obs/metrics.go:768`, `metrics.go:109` doc
   comment confirms "repo is a bounded per-workspace identifier").
2. The single production call site is
   `internal/semantic/live/handler/difffacts.go:260`:
   `h.FileFactDiffMetrics.LiveFileFactDiffInc(tier, string(repoID))`.
3. `repoID` is `semantic.RepoID` (an alias for `string`), and its value is the
   workspace hash — every entrypoint into the handler converts a
   `workspace.WorkspaceKey` to `semantic.RepoID(ws.Hash())` before calling
   handler methods (see semantic_wiring.go's `semLiveAdapter.OnWorkspaceChanged`
   for the conversion pattern at line ~490).
4. **`repo` is NOT the same as `workspace_label`** (metrics.go:79-81): the latter
   is a separate bounded identifier used by `helix_lspool_*` and
   `SemanticStoreQuarantine` (lines 317, 446); they happen to be derived from the
   same workspace key but live in different label-namespace conventions.
   `LiveFileFactDiff` (Phase 68 D-07) deliberately chose `repo` (shorter, mirrors
   the row's `repo_id` column) over `workspace_label`. Phase 70 follows Phase 68
   verbatim per CONTEXT.md "Implementation Notes" (closed-enum reason values
   mirror Phase 68 D-08 pattern).

**Consequence for Plan 04:** `b.metrics.IncrementalRefreshFallbackInc(reason,
ws.Hash())` (Plan 04 Task 2 action step 2 already names `repoID := ws.Hash()`,
then passes `repoID` to `IncrementalRefreshFallbackInc`). The repo value is
`ws.Hash()` (string), bounded by the workspace-hash space (N workspaces, not
N × paths). No additional hashing or truncation is needed.

**Consequence for the labels allowlist test (Plan 03 Task 2):** the carve-out
entry mirrors the `LiveFileFactDiff` carve-out: `{"reason": true, "repo": true}`
under the key `"helix_incremental_refresh_fallback_total"`. The carve-out test
verifies bounded cardinality; no per-call cardinality bound is enforced (same as
Phase 68).

**Citations:** metrics.go:80-81 (workspace_label doc), metrics.go:105-110
(LiveFileFactDiff repo label doc), metrics.go:768-778 (LiveFileFactDiffInc
signature + emission), handler/difffacts.go:260 (production call site).
