# Phase 70: Incremental Refresh Overlay-Drain — Pattern Map

**Mapped:** 2026-05-15
**Files analyzed:** 9 (4 modified, 3 new, 2 test-extensions)
**Analogs found:** 9 / 9 (all exact or strong role-match)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/semantic/store/overlay.go` (extend with `OverlayChangedPathsSince`) | store accessor | request-response read | `internal/semantic/store/overlay.go:1019-1038` `CurrentOverlayEpoch` | exact (same file, sibling accessor) |
| `internal/semantic/store/migrations.go` (add `applyMigration006`) | schema migration | batch DDL | `internal/semantic/store/migrations.go:495-535` `applyMigration003` / `schema3Statements` | exact (templates the ALTER+INDEX+stamp pattern) |
| `internal/semantic/store/migrations_registry.go` (append `{From:5,To:6,...}`) | config | static registry | `internal/semantic/store/migrations_registry.go:27-33` | exact (one-line append) |
| `internal/semantic/store/migrations_types.go` (`CurrentSchemaVersion = 6`) | config constant | n/a | same file:21 | exact (constant bump) |
| `internal/semantic/store/snapshot.go` (extend `SnapshotMeta`, `CommitSnapshot` stamp) | store mutator | transactional write | snapshot.go:86-98 `SnapshotMeta`, 408-449 `CommitSnapshot` | exact (extend existing types) |
| `internal/daemon/semantic_wiring.go` (`collectCandidatePaths` rewrite + buildFn baseline-epoch capture) | wiring/dispatcher | request-response | semantic_wiring.go:1378-1463 `makeProductionBuildFn`, 1466-1520 `collectCandidatePaths` | exact (rewrite in place) |
| `internal/obs/metrics.go` (add `IncrementalRefreshFallback` vec + Inc helper) | metric registration | event counter | metrics.go:346-355 `LiveFileFactDiff`, 768-778 `LiveFileFactDiffInc`, 526-527 `MustRegister` block | exact (Phase 68 D-07 mirror) |
| `internal/obs/metrics_labels_test.go` (carve-out for `reason` label) | test config | static allowlist | metrics_labels_test.go:108, 112 (Phase 68 carve-outs) | exact (one-line carve-out add) |
| `internal/skill/semantic/accessors.go` (`StoreAccessor` adds `OverlayChangedPathsSince` + `CurrentOverlayEpoch`) | interface | n/a | accessors.go:16-33 `StoreAccessor` | exact (extend interface) |
| `internal/skill/semantic/tools_refresh.go` (`files_updated` derives from seam, optional `FlushNow`) | tool handler | request-response | tools_refresh.go:130-237 `handleRefreshSemanticGraph`; line 156-160 LiveAccessor call; line 162-176 envelope shape | exact (in-place edit at line 170-176) |
| `internal/skill/semantic/accessors.go` (`LiveAccessor` adds `FlushNow`) | interface | n/a | accessors.go:64-72 `LiveAccessor` | exact (extend interface) |
| `internal/semantic/store/overlay_test.go` (`TestOverlayChangedPathsSince` — 4 sub-tests) | test | unit | `TestCurrentOverlayEpoch` siblings in `overlay_test.go` (mirror); `filefact_accessor_test.go` for the 4-arm shape | role-match |
| `internal/daemon/semantic_wiring_test.go` (`TestCollectCandidatePaths_Incremental*`) | test | integration | existing `semantic_wiring_test.go` patterns | role-match |
| `internal/skill/semantic/tools_refresh_test.go` (`TestRefresh_FilesUpdated_FromSeam`) | test | integration | existing `tools_refresh_test.go` recorder-mock pattern | role-match |
| `internal/eval/runner/refresh_incremental_test.go` (NEW — REFRESH-03) | e2e test | end-to-end | `integration_test.go:422-` `TestE2E_IndexThenContext_*` + `makeFixtureFacts` | role-match (eval-runner harness) |
| `internal/eval/runner/bench_refresh_incremental_test.go` (NEW — REFRESH-02) | bench | latency sampling | `inprocess_fixtures_test.go:18-65` `TestRunQuickFullFixtureSetWallTime` | role-match (manual sampling + skip on CI) |

## Pattern Assignments

### `internal/semantic/store/overlay.go` — new `OverlayChangedPathsSince`

**Analog:** `internal/semantic/store/overlay.go:1004-1038` (`CurrentOverlayEpoch`) + `internal/semantic/store/filefact_accessor.go:75-152` (multi-row read with rows.Scan loop)

**Imports pattern** (overlay.go top — already imports these; no additions needed):
```go
import (
    "context"
    "database/sql"
    "errors"
    "fmt"
)
```

**Nil-guard + error-wrap pattern** (overlay.go:1019-1037):
```go
func (s *Store) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
    if s == nil || s.db == nil {
        return 0, fmt.Errorf("CurrentOverlayEpoch: nil store")
    }
    if repoID == "" {
        return 0, fmt.Errorf("CurrentOverlayEpoch: empty repoID")
    }
    var ep uint64
    err := s.db.QueryRowContext(ctx, `
        SELECT current_epoch FROM semantic_live_overlay_meta
         WHERE repo_id = ?
    `, repoID).Scan(&ep)
    if err == sql.ErrNoRows {
        return 0, nil
    }
    if err != nil {
        return 0, fmt.Errorf("CurrentOverlayEpoch(%q): %w", repoID, err)
    }
    return ep, nil
}
```

**Multi-row scan pattern** (filefact_accessor.go:117-144):
```go
rows, err := s.db.QueryContext(ctx, `
    SELECT fact_json::VARCHAR
      FROM semantic_live_overlay_symbols
     WHERE repo_id = ? AND file_id = ? AND status = 'live'
       AND fact_json IS NOT NULL
`, repoID, fileID)
if err != nil {
    return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay symbol query: %w", repoID, path, err)
}
defer rows.Close()

for rows.Next() {
    var raw string
    if err := rows.Scan(&raw); err != nil {
        return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay symbol scan: %w", repoID, path, err)
    }
    // ... append to slice
}
if err := rows.Err(); err != nil {
    return PriorFileFact{}, false, fmt.Errorf("GetLatestFileFact(%q,%q): overlay symbol rows: %w", repoID, path, err)
}
```

**New accessor (compose the two):** Two `QueryRowContext`/`QueryContext` calls. Read `current_epoch` first (matches CurrentOverlayEpoch), then `SELECT DISTINCT path FROM semantic_live_overlay_files WHERE repo_id=? AND write_epoch>?` using the existing `idx_overlay_files_write_epoch` index (migrations.go:527). D-09 compliance: no `Begin/Commit/Abort/Write/Tx` tokens; pure read.

---

### `internal/semantic/store/migrations.go` — new `applyMigration006`

**Analog:** `migrations.go:495-535` (`applyMigration003` / `schema3Statements`)

**Migration body pattern (literal copy)** (migrations.go:495-503):
```go
func applyMigration003(ctx context.Context, db *sql.DB) error {
    stmts := schema3Statements()
    for i, stmt := range stmts {
        if _, err := db.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("applyMigration003: stmt %d (%s): %w", i+1, firstLine(stmt), err)
        }
    }
    return nil
}
```

**Statement-builder pattern** (migrations.go:513-535):
```go
func schema3Statements() []string {
    return []string{
        `ALTER TABLE semantic_live_overlay_meta ADD COLUMN current_epoch UBIGINT DEFAULT 0`,
        // ... more ALTERs
        `CREATE INDEX idx_overlay_files_write_epoch ON semantic_live_overlay_files(repo_id, write_epoch)`,
        // ... more INDEXes
        `INSERT INTO semantic_schema_version (version, applied_at) VALUES (3, now())`,
    }
}
```

**Constraint to mirror (migrations.go:482-487):** DuckDB rejects `NOT NULL DEFAULT <expr>` on `ALTER ... ADD COLUMN`. Use bare `DEFAULT 0`.

**For Phase 70:** schema6Statements returns 2 statements: one ALTER (`semantic_snapshots ADD COLUMN base_overlay_epoch UBIGINT DEFAULT 0`) and the version stamp.

---

### `internal/semantic/store/migrations_registry.go` / `migrations_types.go`

**Analog:** migrations_registry.go:27-33 (append one line); migrations_types.go:21 (bump constant).

**Registry append** (migrations_registry.go:27-33):
```go
var migrations = []Migration{
    {From: 0, To: 1, Kind: MigrationInPlace, Apply: applyMigration001},
    // ...
    {From: 4, To: 5, Kind: MigrationInPlace, Apply: applyMigration005},
    // Phase 70 adds:
    // {From: 5, To: 6, Kind: MigrationInPlace, Apply: applyMigration006},
}
```

**Constant bump** (migrations_types.go:21): `const CurrentSchemaVersion = 5` → `6`.

---

### `internal/semantic/store/snapshot.go` — `SnapshotMeta` + `CommitSnapshot` stamp

**Analog:** snapshot.go:86-98 (`SnapshotMeta`), 285-298 (`BeginSnapshot` INSERT site), 408-449 (`CommitSnapshot`)

**SnapshotMeta extension pattern** (snapshot.go:86-98):
```go
type SnapshotMeta struct {
    RepoID         string
    BaseSnapshotID uint64
    CapturedEpoch  uint64 // existing — Phase 63 captured-epoch field
    // Phase 70: BaseOverlayEpoch uint64 — captured at CommitSnapshot time.
}
```

**CommitSnapshot UPDATE site** (snapshot.go:426-432) — the place to add a `base_overlay_epoch=?` column to the UPDATE:
```go
if _, err := snap.tx.ExecContext(ctx, `
    UPDATE semantic_snapshots
       SET status='committed', committed_at=now()
     WHERE snapshot_id=?
`, snap.ID); err != nil {
    return fmt.Errorf("CommitSnapshot: flip status: %w", err)
}
```

**Recommendation (RESEARCH.md Pitfall 3):** buildFn reads `b.store.CurrentOverlayEpoch(ctx, repoID)` immediately before CommitSnapshot, passes via extended SnapshotSummary (or new `CommitSnapshotWithEpoch`); the UPDATE writes it in the same tx.

---

### `internal/daemon/semantic_wiring.go` — `collectCandidatePaths` rewrite

**Analog:** semantic_wiring.go:1378-1463 (`makeProductionBuildFn` — call site at line 1397), 1466-1520 (current `collectCandidatePaths`)

**Current call site** (line 1397) — receives `mode` and currently passes through:
```go
paths := b.collectCandidatePaths(ws, mode)
```

**Current implementation to rewrite** (line 1479-1520):
```go
func (b *semanticBundle) collectCandidatePaths(ws workspace.WorkspaceKey, _ string) []string {
    if ws.RepoRoot == "" {
        return nil
    }
    var paths []string
    _ = filepath.WalkDir(ws.RepoRoot, func(path string, d fs.DirEntry, walkErr error) error {
        // ... existing walk body (extract verbatim into fullWalkPaths)
    })
    return paths
}
```

**Baseline-epoch fetch site (new) — buildFn line ~1386:** the existing `LatestCommittedSnapshot` call already runs for `mode=="incremental"`; extend to fetch `base_overlay_epoch` in the same round-trip (or via a new sibling accessor `LatestCommittedSnapshotEpoch`).

**Rewrite shape (from RESEARCH.md §Code Examples):** dispatch on mode; extract the existing walk body verbatim into `fullWalkPaths`; for incremental, call `b.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)`; classify empty-result reason; emit `IncrementalRefreshFallbackInc(reason, repoID)`; fall back to `fullWalkPaths(ws)`.

**Annotation removal (D6):** Replace the multi-paragraph comment at lines 1466-1480 with a 3-line note pointing at the seam (verbatim text in CONTEXT.md D6).

---

### `internal/obs/metrics.go` — `IncrementalRefreshFallback` vec + Inc helper

**Analog:** metrics.go:346-355 (`LiveFileFactDiff` NewCounterVec); metrics.go:768-778 (`LiveFileFactDiffInc` helper); metrics.go:526-527 (`MustRegister` block).

**Vec registration pattern** (metrics.go:346-355):
```go
LiveFileFactDiff: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "helix_live_filefactdiff_total",
        Help: "Live FileFactDiff populator outcomes by tier (full/added-only/synthetic) and repo. Phase 68.",
    },
    // Phase 68 D-07: closed-enum "tier" + per-repo bounded label.
    // Carved out in metrics_labels_test.go; helper LiveFileFactDiffInc drops unknowns.
    []string{"tier", "repo"},
),
```

**Inc helper pattern (closed-enum drop-on-unknown)** (metrics.go:768-778):
```go
func (m *Metrics) LiveFileFactDiffInc(tier, repo string) {
    if m == nil || m.LiveFileFactDiff == nil {
        return
    }
    switch tier {
    case "full", "added-only", "synthetic":
    default:
        return
    }
    m.LiveFileFactDiff.WithLabelValues(tier, repo).Inc()
}
```

**MustRegister append** (metrics.go:526-527 — insertion point):
```go
m.LiveFileFactDiff,
m.LiveFileFactDiffSynRsn,
// Phase 70: m.IncrementalRefreshFallback,
```

**For Phase 70:** Field name `IncrementalRefreshFallback`; vec name `helix_incremental_refresh_fallback_total`; labels `[]string{"reason","repo"}`; helper accepts `reason ∈ {"cold_start","overlay_rotated","empty_overlay","error"}`.

---

### `internal/obs/metrics_labels_test.go` — `reason` label carve-out

**Analog:** metrics_labels_test.go:100, 108, 112 (Phase 63 / Phase 68 carve-outs).

**Carve-out entry pattern**:
```go
// Phase 68 D-07: closed-enum "tier" ∈ {full, added-only, synthetic}
// + bounded "repo" identifier on the precise FileFactDiff outcome
// counter. Neither label is in AllowedLabels; both are carved out
// here. Helper LiveFileFactDiffInc is the single emission site and
// drops unknown tier values.
"helix_live_filefactdiff_total": {"tier": true, "repo": true},
```

**For Phase 70:** add `"helix_incremental_refresh_fallback_total": {"reason": true, "repo": true}` with parallel comment.

---

### `internal/skill/semantic/accessors.go` — `StoreAccessor` + `LiveAccessor` interface extensions

**Analog:** accessors.go:18-33 (`StoreAccessor`), 64-72 (`LiveAccessor`).

**StoreAccessor pattern** (accessors.go:18-27):
```go
type StoreAccessor interface {
    LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error)
    CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
    OverlayHasPendingRows(repoID string) bool
    // Phase 70 additions:
    //   CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error)
    //   OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) (paths []string, currentEpoch uint64, err error)
    // ...
}
```

**LiveAccessor extension (for Pitfall 1 — synchronous flush):**
```go
type LiveAccessor interface {
    OnWorkspaceChanged(ws workspace.WorkspaceKey, paths []string) error
    LastFlushAt(ws workspace.WorkspaceKey) int64
    // Phase 70: FlushNow(ctx context.Context, ws workspace.WorkspaceKey) error
}
```

Daemon adapter in `internal/daemon/semantic_wiring.go` implements both new methods by delegating to `*Store` and `coalescer.FlushNow` respectively.

---

### `internal/skill/semantic/tools_refresh.go` — `files_updated` honesty fix

**Analog:** tools_refresh.go:130-237 (entire handler). Current wart at line 170-176:
```go
// files_updated semantics: when args.Paths is supplied, the strict-
// subset count is exactly len(args.Paths). When empty, the live drain
// processes whatever the coalescer flushes — without a return-value
// signal from the accessor we report 0 (the SPEC §23.2 envelope is
// best-effort here; consumers asking about specific files supply the
// paths argument).
filesUpdated := len(args.Paths)
```

**LiveAccessor call site (line 156-160) — pre-existing pattern to mirror:**
```go
if s.live != nil {
    if err := s.live.OnWorkspaceChanged(ws, args.Paths); err != nil {
        return errorResult(err.Error())
    }
}
```

**Replacement shape (RESEARCH.md §Code Examples):**
1. Before line 156: capture `preEpoch, _ := s.store.CurrentOverlayEpoch(ctx, ws.Hash())`.
2. Inside the `s.live != nil` block, after `OnWorkspaceChanged`, call `s.live.FlushNow(ctx, ws)` (non-fatal on error).
3. Replace line 176 with `changed, _, _ := s.store.OverlayChangedPathsSince(ctx, ws.Hash(), preEpoch); filesUpdated := len(changed)`.

**D-09/D-13 INVARIANT to preserve** (tools_refresh.go:14-23, 124-129): NO `Begin/Commit/Abort/Write` snapshot tokens added. Grep gate continues to scan this file.

---

### `internal/eval/runner/refresh_incremental_test.go` (NEW)

**Analog:** `integration_test.go:332-360` `makeFixtureFacts` (fixture builder) + `integration_test.go:422-460` `TestE2E_IndexThenContext_*` (E2E shape).

**Fixture-builder pattern** (integration_test.go:335-360):
```go
func makeFixtureFacts(symbolCount int) semanticstore.Facts {
    files := []semanticstore.FileFact{
        {FileID: 1, Path: "src/fixture.go", Language: "go", ContentHash: "fixture-hash"},
    }
    symbols := make([]semanticstore.SymbolFact, symbolCount)
    for i := 0; i < symbolCount; i++ {
        symbols[i] = semanticstore.SymbolFact{
            SymbolID: uint64(100 + i),
            // ...
        }
    }
    return semanticstore.Facts{Files: files, Symbols: symbols}
}
```

**RESEARCH.md says:** extract a SIBLING helper `makeFixtureFactsNFiles(numFiles, symbolsPerFile int)` (do NOT modify makeFixtureFacts — used by 6+ tests with 1-file shape).

**Sub-test pattern (from RESEARCH.md §Code Examples):** 4 sub-tests — single-file-changed, cold_start, overlay_rotated, empty_overlay. Each asserts metric label increments + bounded log line emission.

---

### `internal/eval/runner/bench_refresh_incremental_test.go` (NEW)

**Analog:** `internal/eval/runner/inprocess_fixtures_test.go:18-65` `TestRunQuickFullFixtureSetWallTime` (manual wall-time sampling pattern + skip).

**Skip-on-CI pattern** (inprocess_fixtures_test.go:19-21):
```go
if testing.Short() {
    t.Skip("TestRunQuickFullFixtureSetWallTime: skipping in -short mode (CI resource constraint)")
}
```

**For Phase 70 (per RESEARCH.md):** ALSO gate on `os.Getenv("CI") != ""` per `feedback_no_ci_benchmarks` project rule. Use 50-iteration manual `time.Since(start)` sampling + `sort.Slice` + `samples[int(0.95*len(samples))]` for p95.

## Shared Patterns

### D-09 read-only invariant (lock-free `*Store` accessors)

**Source:** `internal/semantic/store/overlay.go:1019-1038` (`CurrentOverlayEpoch`), `internal/semantic/store/filefact_accessor.go` (whole file is the Phase 68 precedent).

**Apply to:** the new `OverlayChangedPathsSince` accessor.

**Constraints:**
- NO `Begin/Commit/Abort/Write` tokens
- Nil-guard on `s == nil || s.db == nil`
- Empty-string-guard on `repoID`
- `sql.ErrNoRows` → `(zero, nil)` (NOT a wrapped error)
- Non-ErrNoRows errors → `fmt.Errorf("%s(%q): %w", funcName, repoID, err)`
- `vet-nokernel2semantic` boundary preserved: signature uses only `context.Context`, `string`, `uint64`, `[]string`, `error`.

### Closed-enum bounded-label metric (drop-on-unknown)

**Source:** `internal/obs/metrics.go:768-778` `LiveFileFactDiffInc` (Phase 68 D-07 template).

**Apply to:** new `IncrementalRefreshFallbackInc(reason, repo string)`.

**Discipline:**
- `switch reason { case "cold_start","overlay_rotated","empty_overlay","error": default: return }` — drop unknowns at the emission site
- Carve-out entry in `metrics_labels_test.go` for the new label name `reason` on the new metric family
- Const-declared reason values in the same package as the emission site (CONTEXT.md Implementation Notes)

### Schema migration registration

**Source:** `internal/semantic/store/migrations.go:495-535` + `migrations_registry.go:27-33` + `migrations_types.go:21`.

**Apply to:** Phase 70's `applyMigration006`.

**Three-step recipe:**
1. New `applyMigration006` body + `schema6Statements()` in `migrations.go`
2. Append registry entry `{From: 5, To: 6, Kind: MigrationInPlace, Apply: applyMigration006}` in `migrations_registry.go`
3. Bump `CurrentSchemaVersion = 6` in `migrations_types.go`
4. Final statement of `schema6Statements()` MUST be `INSERT INTO semantic_schema_version (version, applied_at) VALUES (6, now())` — `runMigrations` re-reads schema_version after Apply and fails loudly if not stamped (registry.go:75-81).

### Bench skip-on-CI

**Source:** Project rule `feedback_no_ci_benchmarks`; `inprocess_fixtures_test.go:19-21` for the `testing.Short()` pattern.

**Apply to:** `bench_refresh_incremental_test.go`.

**Combine BOTH gates:**
```go
if os.Getenv("CI") != "" {
    t.Skip("bench is local-only per project rule feedback_no_ci_benchmarks")
}
if testing.Short() {
    t.Skip("bench skipped under -short")
}
```

### Test recorder-mock for `tools_refresh` D-09/D-13 invariant

**Source:** `internal/skill/semantic/tools_refresh_test.go` (existing recorder-mock that fails on `Begin/Commit/Abort/Write` reach-through per accessors.go:127-131 comment).

**Apply to:** any new test in `tools_refresh_test.go` for Phase 70. The accessor extensions (`OverlayChangedPathsSince`, `CurrentOverlayEpoch`, `FlushNow`) are added to the recorder seam — but the existing fail-loud assertion on snapshot-write tokens must remain.

## No Analog Found

None. Every Phase 70 surface has a strong analog. The `coalescer.FlushNow` synchronous-flush addition is the only piece without a perfect in-tree precedent — RESEARCH.md flags it as Assumption A2 (needs Wave 0 verification) and proposes building on `Coalescer.SetOnFlush` (coalescer.go:124-132).

## Metadata

**Analog search scope:**
- `internal/semantic/store/` (overlay.go, snapshot.go, migrations*.go, filefact_accessor.go)
- `internal/daemon/semantic_wiring.go`
- `internal/skill/semantic/` (accessors.go, tools_refresh.go, integration_test.go)
- `internal/obs/` (metrics.go, metrics_labels_test.go)
- `internal/eval/runner/` (inprocess_fixtures_test.go)

**Files read:** 10 (targeted ranges, no full-file reads of large files)
**Pattern extraction date:** 2026-05-15
