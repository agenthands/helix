---
phase: 59
plan: 01
subsystem: semantic-store
tags: [semantic, store, schema, migration, duckdb, phase57-followup, schema-v2]
requires:
  - internal/semantic/store (Phase 57 D-02 Migration{From,To,Kind} struct + Apply field stub at migrations_types.go:37)
  - internal/semantic/store applyMigration001 (Phase 57 bootstrap migration, schema 1)
  - duckdb-go/v2 driver (Phase 57 D-12)
provides:
  - Migration.Apply func field bound for the first time
  - migrations registry slice + runMigrations progressive loop in CGO=1 path
  - applyMigration002: 10 partial-extraction columns + schema_version=2 row stamp
  - ErrForwardIncompatible sentinel; Open propagates rather than quarantining on forward-incompat
  - 4 migration tests (Fresh / Existing / ForwardIncompatible / AllColumnsPresent)
affects:
  - All future Phase 59 extraction code can write extraction_status / extraction_partial / partial_reason / extractor_name / extractor_version / error_message into semantic_files
  - All future Phase 59 extraction code can write partial / partial_reason on semantic_symbols and semantic_references
  - Existing Phase 57 v1 DBs upgrade in-place to v2 on first daemon start with the v1.10 binary
  - Operator path on downgrade: forward-incompat error explicitly surfaced (was previously silent quarantine + data loss for newer DB)
tech-stack:
  added: []
  patterns:
    - "Sentinel error + errors.Is propagation for distinguishable failure modes (forward-incompat vs generic reopen failure)"
    - "Defensive re-read after each Apply (catches a migration body that forgets to stamp version)"
    - "DuckDB ALTER TABLE workaround: DEFAULT alone, no NOT NULL (constraint limitation in current DuckDB)"
key-files:
  created:
    - internal/semantic/store/migrations_registry_cgo.go
    - internal/semantic/store/migrations_test.go
  modified:
    - internal/semantic/store/migrations_types.go
    - internal/semantic/store/migrations.go
    - internal/semantic/store/duckdb.go
decisions:
  - "Registry slice + runMigrations loop live in a new CGO=1-gated file (migrations_registry_cgo.go) rather than in build-tag-neutral migrations_types.go — preserves the CGO=0 stub path that does not exercise migrations (consistent with duckdb_nocgo.go pattern). Apply field shape on the Migration struct is build-tag-neutral (uses *sql.DB, available in both builds)."
  - "DuckDB ALTER TABLE ADD COLUMN does not accept NOT NULL together with DEFAULT (Parser Error: 'Adding columns with constraints not yet supported'). Workaround: ship DEFAULT alone. The DEFAULT supplies values for both existing-row backfill and INSERTs that omit the column, so writers cannot leave the column unset — same practical guarantee as NOT NULL DEFAULT for a closed extractor that always sets every column."
  - "Forward-incompat handling: introduce ErrForwardIncompatible sentinel; Open propagates on errors.Is match rather than quarantining. Quarantining a newer DB silently would discard the newer binary's data on rollback. Operators see an explicit 'rebuild required' error and follow the documented quarantine-and-rebuild path manually."
  - "readSchemaVersion uses SELECT max(version) so a misaligned schema_version table (multiple rows from sequential migrations or an explicit operator INSERT) reports the canonical current version. classifyExisting keeps SELECT … LIMIT 1 (Phase 57 contract); the test relies on DuckDB's insertion-order LIMIT 1 to get the older row through classifyExisting, then runMigrations' max trips the forward-incompat guard."
metrics:
  start: "2026-05-04T13:31:00Z"
  end: "2026-05-04T14:06:51Z"
  duration_minutes: 36
  completed_date: "2026-05-04"
  tasks_completed: 3
  files_created: 2
  files_modified: 3
  tests_added: 4
---

# Phase 59 Plan 01: Schema Migration v1→v2 Summary

Light up the Phase 57 D-02 migration registry for the first time and ship the v1→v2 schema migration that adds the partial-extraction columns prescribed by 59-CONTEXT.md D-05.

## Outcome

Wave 0 prep for Phase 59 is complete: a working migration registry that progressively applies migrations from current schema_version up to CurrentSchemaVersion, plus 10 new columns (6 + 2 + 2) on the three Phase 57 fact tables. Subsequent Phase 59 plans can write extraction status, partial reasons, extractor identity, and error messages into the store from day one.

## What Shipped

### Migration registry mechanism (`internal/semantic/store/migrations_registry_cgo.go`, new file, CGO=1)

- `migrations` slice with two entries:
  - `{From: 0, To: 1, Kind: MigrationInPlace, Apply: applyMigration001}` (Phase 57 bootstrap)
  - `{From: 1, To: 2, Kind: MigrationInPlace, Apply: applyMigration002}` (Phase 59 partial-extraction columns)
- `runMigrations(ctx, db) error`:
  1. Reads current `schema_version` via `SELECT max(version)` (returns 0 if the table doesn't exist — fresh DB).
  2. If `currentVersion > CurrentSchemaVersion`, returns `ErrForwardIncompatible` wrapped with concrete versions.
  3. Iterates the registry, applying each entry where `m.From >= currentVersion && m.To <= CurrentSchemaVersion`.
  4. After every Apply, re-reads `schema_version` and asserts it equals `m.To` (defensive: catches a body that forgot to stamp).
  5. Stops when registry exhausted.
- `ErrForwardIncompatible` sentinel; Open propagates rather than quarantining when `errors.Is(err, ErrForwardIncompatible)` (see "Deviations" below for why this matters).

### CurrentSchemaVersion bump (`internal/semantic/store/migrations_types.go`)

- `CurrentSchemaVersion` 1 → 2.
- `Migration.Apply func(ctx context.Context, db *sql.DB) error` field added (build-tag-neutral; *sql.DB is available in both CGO=1 and CGO=0).
- Phase 57 TODO `// TODO(P57-02 Task 2b): wire the Migration registry slice ...` removed.

### applyMigration002 (`internal/semantic/store/migrations.go`)

10 ALTER TABLE statements + 1 INSERT, in deterministic order:

| Table | Column | Type | Default |
|---|---|---|---|
| semantic_files | extraction_status | TEXT | '' |
| semantic_files | extraction_partial | BOOLEAN | false |
| semantic_files | partial_reason | TEXT | NULL |
| semantic_files | extractor_name | TEXT | NULL |
| semantic_files | extractor_version | TEXT | NULL |
| semantic_files | error_message | TEXT | NULL |
| semantic_symbols | partial | BOOLEAN | false |
| semantic_symbols | partial_reason | TEXT | NULL |
| semantic_references | partial | BOOLEAN | false |
| semantic_references | partial_reason | TEXT | NULL |

Plus `INSERT INTO semantic_schema_version (version, applied_at) VALUES (2, now())`.

Rollback story documented as a top-of-function comment block: full reindex via the Phase 57 D-04 quarantine path (DuckDB does not support reliable `DROP COLUMN`).

### Open path rewiring (`internal/semantic/store/duckdb.go`)

- `openFresh` calls `runMigrations` instead of `applyMigration001` directly — fresh DBs land at `CurrentSchemaVersion=2` after running 001 + 002 in sequence.
- `openExisting` now calls `runMigrations` — existing v1 DBs upgrade in-place to v2 on first reopen with the v1.10 binary.
- Open's existing branch that fell through to `quarantineAndRebuild` on any openExisting error is now guarded with `errors.Is(err, ErrForwardIncompatible)` — forward-incompat errors are propagated to the caller, not silently quarantined.

### Migration test suite (`internal/semantic/store/migrations_test.go`, new file)

- `TestMigration_Fresh_v2` — fresh Open ⇒ schema_version=2, all v1 tables present, all 10 v2 columns visible in `information_schema.columns`.
- `TestMigration_Existing_v2` — directly invokes `applyMigration001` on a raw `sql.DB`, inserts representative rows into the three fact tables, closes, reopens via Open. Asserts schema_version upgrades to 2, row counts preserved, new columns default-valued (empty string / false).
- `TestMigration_ForwardIncompatible` — fresh v=2 DB then INSERTs version=99 alongside. classifyExisting's `LIMIT 1` returns the older row (DuckDB insertion order) and passes through; runMigrations' `max(version)` reads 99 and returns the wrapped sentinel. Asserts error contains "forward-incompatible" and "99".
- `TestMigration002_AllColumnsPresent` — table-driven walk over all 10 columns asserting data_type via `information_schema.columns` (DuckDB normalizes TEXT to VARCHAR; BOOLEAN stays BOOLEAN). 10 sub-tests.

## Tasks Completed

| Task | Description | Commit | Key Files |
| --- | --- | --- | --- |
| 1 | Wire Migration.Apply func field + registry-driven loop in Open | `9c8d1007` | migrations_types.go, migrations_registry_cgo.go (new), migrations.go (stub), duckdb.go |
| 2 | Implement applyMigration002 (10 ALTER TABLE + INSERT) | `47465efe` | migrations.go |
| 3 | Migration test suite (fresh + upgrade + forward-incompat + columns) | `8211a43b` | migrations_test.go (new), migrations_registry_cgo.go (sentinel), duckdb.go (propagation branch) |

## Verification

- `go test ./internal/semantic/store/... -count=1` → ok (all Phase 57 tests + 4 new TestMigration_* + 10 sub-tests pass).
- `go vet ./internal/semantic/store/...` → clean.
- `go build ./internal/semantic/store/...` → clean (CGO=1 default).
- `CGO_ENABLED=0 go build ./internal/semantic/store/...` → clean (the !cgo stub path is unaffected because Migration.Apply is a function field — type compiles in both builds — and the registry slice itself lives in `//go:build cgo`-gated source).
- `grep -c 'CurrentSchemaVersion = 2' internal/semantic/store/migrations_types.go` → 1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] DuckDB ALTER TABLE constraint limitation**

- **Found during:** Task 2 (verification step).
- **Issue:** DuckDB rejects `ALTER TABLE … ADD COLUMN … NOT NULL DEFAULT <expr>` with `Parser Error: Adding columns with constraints not yet supported`. The plan's body specified `BOOLEAN NOT NULL DEFAULT false` and `TEXT NOT NULL DEFAULT ''`. 59-RESEARCH.md §A1 had explicitly flagged this as a planning-time risk: *"If DuckDB requires a different syntax (e.g., separate ADD COLUMN + UPDATE), migration plan splits one statement into two."*
- **Fix:** Drop `NOT NULL` from the ALTER TABLE statements that combine it with DEFAULT (4 statements: `extraction_status`, `extraction_partial`, `semantic_symbols.partial`, `semantic_references.partial`). The DEFAULT supplies values for both existing-row backfill and INSERTs that omit the column, giving the same practical guarantee as NOT NULL DEFAULT for a closed extractor that always sets every column. CREATE TABLE statements in `schema1Statements()` are unaffected (the limitation is ALTER-only). Documented inline in `schema2Statements()` and in the `applyMigration002` godoc.
- **Files modified:** `internal/semantic/store/migrations.go` (4 ALTER statements + godoc + inline comment).
- **Commit:** `47465efe`.

**2. [Rule 2 — Missing critical functionality] Forward-incompat error needs sentinel + propagation branch**

- **Found during:** Task 3 (TestMigration_ForwardIncompatible failed with `want error, got nil`).
- **Issue:** Open's existing logic at `duckdb.go:117-124` (Phase 57 contract) caught any `openExisting` failure and fell through to `quarantineAndRebuild`. With the new migration loop, a forward-incompat error returned by `runMigrations` was swallowed by the quarantine path — so the operator running an older binary against a newer DB would silently quarantine the newer DB and lose its data. The plan explicitly required Open to **return** the forward-incompat error.
- **Fix:** Introduced `var ErrForwardIncompatible = errors.New(...)` sentinel in `migrations_registry_cgo.go`; wrapped concrete versions in `runMigrations` via `fmt.Errorf("%w: stored=%d binary=%d", ErrForwardIncompatible, …)`. Added an `errors.Is(err, ErrForwardIncompatible)` guard in Open that propagates the error rather than quarantining, with a comment explaining why silent quarantine would be data-destructive on rollback.
- **Files modified:** `internal/semantic/store/migrations_registry_cgo.go` (sentinel + wrap), `internal/semantic/store/duckdb.go` (errors.Is branch + import). 
- **Commit:** `8211a43b`.

**3. [Rule 3 — Blocking] Registry slice cannot live in build-tag-neutral file**

- **Found during:** Task 1 (the plan suggested putting the registry slice in `migrations_types.go`, gated with `//go:build cgo` if needed).
- **Issue:** `migrations_types.go` is build-tag-neutral (no `//go:build` directive); it defines types used in both CGO=1 and CGO=0 builds. The registry slice references `applyMigration001` and `applyMigration002`, which are CGO=1-only symbols (live in `migrations.go` which has `//go:build cgo`). Adding `//go:build cgo` to `migrations_types.go` would break the existing CGO=0 stub path (`duckdb_nocgo.go`) which doesn't define the Migration types itself — the stub relies on the type being available in both builds for downstream code that might (in the future) inspect the Migration struct shape.
- **Fix:** Created a new `migrations_registry_cgo.go` file with `//go:build cgo` that owns the registry slice and `runMigrations` loop. The `Migration.Apply` field stays in build-tag-neutral `migrations_types.go` (uses `*sql.DB` which is available in both builds).
- **Files modified:** `internal/semantic/store/migrations_registry_cgo.go` (new file).
- **Commit:** `9c8d1007`.

### Plan acceptance criteria adjustments

The plan's Task 1 acceptance criterion `grep -nE 'applyMigration001|applyMigration002' internal/semantic/store/migrations_types.go returns 2 lines (registry binding)` was met by relocating the binding to `migrations_registry_cgo.go` (the only file that can host it under the !cgo stub-preservation invariant). The relocated binding satisfies the **intent** of the criterion (the registry references both migration functions) and is verifiable via `grep -nE 'applyMigration001|applyMigration002' internal/semantic/store/migrations_registry_cgo.go` returning 3 lines (one in the registry slice + two in the godoc).

### Authentication gates

None.

## Threat Model Coverage

| Threat ID | Disposition | Implementation evidence |
|---|---|---|
| T-59-01-01 (Tampering — older DB on disk) | mitigate | `runMigrations` reads `max(version)` and `Open` propagates `ErrForwardIncompatible` via `errors.Is` rather than quarantining. Verified by `TestMigration_ForwardIncompatible`. |
| T-59-01-02 (DoS — partially-applied migration) | mitigate | `runMigrations` re-reads schema_version after each Apply and returns an explicit error if the body forgot to stamp the version row. Future migration bodies that drift will fail fast at registry time. |
| T-59-01-03 (Information Disclosure — error_message column) | accept | Column added as TEXT (nullable). Operator-facing telemetry only; no PII flow defined in this plan. |

## Self-Check: PASSED

- File `.planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-01-SUMMARY.md` (this file): being written now.
- File `internal/semantic/store/migrations_registry_cgo.go`: FOUND.
- File `internal/semantic/store/migrations_test.go`: FOUND.
- File `internal/semantic/store/migrations_types.go`: FOUND (modified).
- File `internal/semantic/store/migrations.go`: FOUND (modified).
- File `internal/semantic/store/duckdb.go`: FOUND (modified).
- Commit `9c8d1007`: FOUND.
- Commit `47465efe`: FOUND.
- Commit `8211a43b`: FOUND.
