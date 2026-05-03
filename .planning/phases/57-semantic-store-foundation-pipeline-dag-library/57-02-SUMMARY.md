---
phase: 57
plan: 02
subsystem: internal/semantic
tags: [semantic, duckdb, store, cgo, daemon, observability, tdd]
wave: 1
status: complete
completed: 2026-05-03

dependency_graph:
  requires:
    - "internal/errors (serr.ErrUnsupported)"
    - "internal/obs (Metrics counter-vec helpers)"
    - "internal/daemon (newDaemon bootstrap step ordering)"
    - "internal/config (SerenaConfig top-level struct)"
  provides:
    - "internal/semantic.Config (typed mirror of koanf semantic_index.*)"
    - "internal/semantic.{SnapshotID,FileID,SymbolID,ReferenceID,EdgeID,Freshness}"
    - "internal/semantic/store.Store + Open(ctx,cfg,logger,metrics)"
    - "internal/semantic/store.CurrentSchemaVersion = 1"
    - "helix_semantic_store_quarantine_total{workspace_label,reason} metric"
    - "helix_semantic_store_open_total{workspace_label,outcome} metric"
    - "daemon bootstrap step 6b (cfg.SemanticIndex.Enabled-gated)"
    - "Daemon.SemanticStore() accessor"
    - "SerenaConfig.SemanticIndex stub field (no koanf tag — P03 wires it)"
    - "DAG-04 // TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases) marker"
  affects: ["internal/semantic", "internal/obs", "internal/daemon", "internal/config"]

tech_stack:
  added:
    - "github.com/duckdb/duckdb-go/v2 v2.10502.0 (D-12 lock — canonical DuckDB Foundation Go binding)"
  patterns:
    - "CGO=0 stub via //go:build !cgo + serr.ErrUnsupported (mirrors Phase 51.1)"
    - "Three-tier resolution (existing+clean / quarantine+rebuild / hard fail) — mirrors langregistry installer"
    - "Bounded-cardinality workspace_label (SHA-256 truncation, T-57-02-06)"
    - "Closed-enum carve-outs in carveOuts map (Phase 47/53 convention)"
    - "Imperative bootstrap with comment-numbered steps (// 6b. between 6a and 6.)"

key_files:
  created:
    - "internal/semantic/types.go"
    - "internal/semantic/config.go"
    - "internal/semantic/store/doc.go"
    - "internal/semantic/store/duckdb.go (//go:build cgo)"
    - "internal/semantic/store/duckdb_nocgo.go (//go:build !cgo)"
    - "internal/semantic/store/migrations.go (//go:build cgo, applyMigration001 + schema1Statements)"
    - "internal/semantic/store/migrations_types.go (Migration types — both build configs)"
    - "internal/semantic/store/snapshot.go (P59 placeholder)"
    - "internal/semantic/store/overlay.go (P60 placeholder)"
    - "internal/semantic/store/effective.go (Schema 1 read API contract doc)"
    - "internal/semantic/store/store_test.go (9 CGO=1 integration tests)"
    - "internal/semantic/store/duckdb_nocgo_test.go (2 CGO=0 stub tests)"
    - "internal/daemon/daemon_semantic_test.go (//go:build cgo && integration smoke)"
  modified:
    - "go.mod / go.sum (duckdb-go + transitive deps)"
    - "internal/obs/metrics.go (SemanticStoreQuarantine + SemanticStoreOpen vectors + Inc helpers)"
    - "internal/obs/metrics_labels_test.go (carveOuts entries + vector priming)"
    - "internal/config/config.go (stub SerenaConfig.SemanticIndex field, no koanf tag)"
    - "internal/daemon/daemon.go (DAG-04 marker + step 6b + Daemon.SemanticStore accessor)"

decisions:
  - "D-01..D-08, D-12 from 57-CONTEXT.md applied as written"
  - "Schema 1 ships empty-but-correct: 16 SPEC §8 tables created, no data write paths (P59/P60 land those)"
  - "Quarantine reasons closed enum {corrupt_file, schema_forward_incompat, schema_unreadable, unknown}"
  - "Open outcomes closed enum {opened, quarantined, created}"
  - "P02 adds the stub SerenaConfig.SemanticIndex field WITHOUT a koanf binding tag — P03 wires the tag and SPEC §25 defaults. End-of-wave-1 build is green via Go's bool-zero-value safe-degradation path"
  - "duckdb-go module path locked at github.com/duckdb/duckdb-go/v2 v2.10502.0 verbatim (D-12) — referenced by P04 analyzer's forbiddenImport prefix constant"
  - "Quarantine workspace_label is SHA-256-truncated (12 hex chars) of the workspace abs path — bounded cardinality (T-57-02-06)"

metrics:
  duration_minutes: ~50
  task_count: 4
  files_created: 13
  files_modified: 5
  commits: 4
---

# Phase 57 Plan 02: Semantic Store Foundation Summary

**One-liner:** Land the `internal/semantic/{config,store,types}` skeleton with three-tier DuckDB open (clean / quarantine+rebuild / hard fail), Schema 1 ships all 16 SPEC §8 tables empty-but-correct, CGO=0 stub mirrors Phase 51.1, two new bounded-label metrics ship together, daemon step 6b wires the store into bootstrap, and the SerenaConfig stub field keeps the wave-1 build green without P03.

## Tasks Executed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Scaffold internal/semantic skeleton + DAG-04 v1.11 migration marker + duckdb-go dep | `6aac291b` |
| 2a | RED — 11 failing store_test.go tests + minimal package skeleton | `c30a2098` |
| 2b | GREEN — DuckDB Open + Schema 1 migration + CGO=0 stub + 2 metrics | `6fea720b` |
| 3 | Stub SerenaConfig.SemanticIndex + daemon step 6b + integration test | `36a97460` |

## What Was Built

### `internal/semantic/` package

- `types.go` — `SnapshotID`, `FileID`, `SymbolID`, `ReferenceID`, `EdgeID` typed `uint64` aliases plus the `Freshness` closed enum (`fresh / stale / unknown`). SPEC §7.
- `config.go` — typed mirror of koanf `semantic_index.*` (SPEC §25). Every leaf carries a `koanf:"..."` tag; 14 nested sub-structs; one struct field per SPEC §25 sub-block.

### `internal/semantic/store/` package — SOLE owner of duckdb-go (D-12, STORE-06)

- `doc.go` — package documentation citing D-12 and the locked `github.com/duckdb/duckdb-go/v2` module path verbatim. Documents the three-tier open contract, schema-version stamping, CGO=0 stub policy, and concurrency/symlink-refusal threats.
- `duckdb.go` (`//go:build cgo`) — real `Open(ctx, cfg, logger, metrics) (*Store, error)` with three-tier classification:
  - Tier 1 fresh path: `applyMigration001` runs all 16 CREATE TABLE statements and inserts the schema-version row; counter `helix_semantic_store_open_total{outcome="created"}`.
  - Tier 1 reopen: schema-version row read and validated against `CurrentSchemaVersion`; counter `outcome="opened"`.
  - Tier 2 quarantine + rebuild: classify via `classifyExisting` into one of three reasons (`corrupt_file`, `schema_forward_incompat`, `schema_unreadable`) or `unknown`; `os.Rename` to `<path>.corrupt.<unix-ts>` (T-57-02-02 symlink refusal); `slog.Warn` with full context; counters `helix_semantic_store_quarantine_total{reason=...}` + `helix_semantic_store_open_total{outcome="quarantined"}`.
  - Tier 3 hard fail: returned to caller (daemon refuses to start).
- `duckdb_nocgo.go` (`//go:build !cgo`) — verbatim mirror of `internal/repomap/extractor_nocgo.go` substituting `serr.ErrUnsupported`. Every method on the stub returns `serr.ErrUnsupported`; `Available()` returns `false`.
- `migrations.go` (`//go:build cgo`) — `applyMigration001` + `schema1Statements()` with all 16 SPEC §9.1-§9.11 CREATE TABLE statements verbatim plus 13 CREATE INDEX statements + the schema-version row stamp.
- `migrations_types.go` (no build tag) — `Migration` struct + `MigrationKind` enum (in_place / reindex), `CurrentSchemaVersion = 1`. Compiles in both CGO configurations.
- `snapshot.go`, `overlay.go`, `effective.go` — placeholder package-doc files marking the future homes of P59 (snapshot writes), P60 (overlay writes), and the SPEC §10 effective-read API. Schema 1 read API returns empty results from the CGO=1 implementation.

### `internal/obs/` updates

- Two new counter vectors registered on the owned Prometheus registry:
  - `helix_semantic_store_quarantine_total{workspace_label, reason}` — closed enum reason ∈ {corrupt_file, schema_forward_incompat, schema_unreadable, unknown}
  - `helix_semantic_store_open_total{workspace_label, outcome}` — closed enum outcome ∈ {opened, quarantined, created}
- Helper-method emitters (`SemanticStoreQuarantineInc`, `SemanticStoreOpenInc`) drop unknown enum values per the Phase 53 T-53-01 mitigation pattern.
- `carveOuts` map in `metrics_labels_test.go` updated for both families; `TestMetricsLabelsAllowlist` primes both vectors so they show up in `Gather()` (Pitfall #3 from Phase 53 D-13).

### `internal/config/` updates

- Added `SerenaConfig.SemanticIndex semantic.Config` stub field. **No koanf binding tag** — P03 will add `koanf:"semantic_index"` and the SPEC §25 default values into `internal/config/defaults.go`. With Go's bool zero value, `SemanticIndex.Enabled` defaults to `false`, so daemon step 6b's conditional becomes dead code and the wave-1 build is green without P03.

### `internal/daemon/` updates

- DAG-04 marker `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)` at the top of `newDaemon` (single occurrence).
- Step 6b inserted between step 6a (CGO=0 refusal at line 202) and step 6 (diag store at line 231). Calls `semanticstore.Open(ctx, cfg.SemanticIndex, logger, observability.Metrics())` only when `cfg.SemanticIndex.Enabled` is true. Errors propagate as `fmt.Errorf("opening semantic store: %w", err)`.
- New `Daemon.semanticStore *semanticstore.Store` field plus `SemanticStore()` accessor (returns nil when feature disabled — P64+ consumers MUST nil-check).
- New integration smoke test `daemon_semantic_test.go` under `//go:build cgo && integration` exercising the full New(cfg) → Open path with `SemanticIndex.Enabled = true`.

## Test Results

| Path | Mode | Result |
|------|------|--------|
| `./internal/semantic/store/...` | CGO=1 | 9/9 pass |
| `./internal/semantic/store/...` | CGO=1 -race | 9/9 pass |
| `./internal/semantic/store/...` | CGO=0 | 2/2 pass |
| `./internal/obs/...` | CGO=1 | all pass (carve-out lints accept the two new vectors) |
| `./internal/daemon/...` | CGO=1 | all pre-existing tests still pass |
| `./internal/daemon/... -tags=integration` | CGO=1 | TestDaemon_SemanticStore_Open_FromConfig passes |
| `go build ./...` | CGO=1 | exit 0 (warnings from pre-existing tree-sitter Swift binding only) |
| `go vet ./...` | CGO=1 | exit 0 |
| `go build ./...` | CGO=0 | exit 0 |
| `CGO_ENABLED=0 go build ./internal/semantic/store/...` | CGO=0 | exit 0 |

## TDD Gate Compliance

Plan-level type was `tdd`; the RED/GREEN cycle is satisfied:

1. **RED** — `c30a2098 test(57-02): RED — add 11 failing store_test.go tests + minimal package skeleton` (test commit, tests fail with assertion errors against stubs)
2. **GREEN** — `6fea720b feat(57-02): GREEN — implement DuckDB store Open + Schema 1 + CGO=0 stub + quarantine/open metrics` (feat commit, all 11 tests now pass)

No REFACTOR commit was needed — the GREEN implementation is the production form; per the planner rule a separate refactor commit is only required if cleanup changes are made after GREEN.

## Deviations from Plan

### Auto-fixed scope adjustments (Rule 2 — missing critical functionality)

**1. [Rule 2 — Critical] Schema DDL placement (planner expected `migrations.go`; refactor preserved acceptance gates)**
- **Found during:** Task 2b
- **Issue:** Plan acceptance grep gates target `internal/semantic/store/migrations.go` for the per-table presence check, but the plan also says `applyMigration001` lives in `migrations.go`. Initial implementation placed the SQL in a separate `migration_001.go` file to keep `migrations.go` short, but that broke the literal grep gates.
- **Fix:** Renamed: original `migrations.go` (type definitions only) → `migrations_types.go`, and `migration_001.go` (DDL + applyMigration001) → `migrations.go`. Both files compile under their respective build tags; per-table grep gates now pass. The 16-CREATE-TABLE belt-and-braces gate (with comment filtering) returns exactly 16.
- **Files modified:** `internal/semantic/store/{migrations,migrations_types}.go`
- **Commit:** `6fea720b` (Task 2b)

### Auth gates: none

No authentication gates were encountered.

## Known Stubs

This plan **intentionally** ships read-only stubs as part of the Schema 1 contract:

| Path | Stub | Reason | Resolved by |
|------|------|--------|-------------|
| `internal/semantic/store/snapshot.go` | Empty placeholder doc | Phase 57 Schema 1 has no write paths by design | P59 (snapshot writes) |
| `internal/semantic/store/overlay.go` | Empty placeholder doc | Phase 57 Schema 1 has no overlay write paths by design | P60 (overlay writes) |
| `internal/semantic/store/duckdb.go QueryEffective*` | Returns empty `[]any` / nil result | Schema 1 has no data, so the effective queries have nothing to return | P59/P60 populate data, then P64 wires the typed query layer |
| `internal/config/config.go SemanticIndex` | Field exists with no `koanf:"..."` tag | P02 lands the typed field for build-graph correctness; P03 owns the binding/defaults | P03 |

These stubs are documented in `57-02-PLAN.md` and `57-CONTEXT.md` as the intentional Phase 57 boundary; downstream plans (P59/P60/P64) close them in order.

## Threat Model Compliance

The plan's threat register lists 6 mitigated + 1 accepted threat. Implementation status:

| Threat ID | Mitigation status |
|-----------|-------------------|
| T-57-02-01 (path traversal) | Mitigated via `filepath.Clean` normalization in `Open` |
| T-57-02-02 (symlink quarantine) | Mitigated — `os.Lstat` check before rename refuses symlink targets |
| T-57-02-03 (schema_version forgery) | Accepted per plan (out-of-band attack — DB-write access defeats simpler attacks) |
| T-57-02-04 (concurrent daemon DB lock) | Documented in `doc.go`; DuckDB's own file lock surfaces as Tier-3 |
| T-57-02-05 (quarantine disk fill) | Documented in `doc.go` + slog event payload; P63 owns pruning |
| T-57-02-06 (workspace_label cardinality) | Mitigated via SHA-256-truncated hash (`workspaceLabel` helper, 12 hex chars → 16M label-space ceiling) |
| T-57-02-07 (privilege elevation) | n/a — Store has no privileged operations |

No new threat surface was introduced beyond what the threat model anticipates.

## Self-Check: PASSED

- [x] `internal/semantic/types.go` exists (FOUND)
- [x] `internal/semantic/config.go` exists (FOUND)
- [x] `internal/semantic/store/doc.go` exists with D-12 + module path citation (FOUND)
- [x] `internal/semantic/store/duckdb.go` `//go:build cgo` (FOUND, line 1)
- [x] `internal/semantic/store/duckdb_nocgo.go` `//go:build !cgo` (FOUND, line 1)
- [x] `internal/semantic/store/migrations.go` contains all 16 enumerated CREATE TABLE statements (FOUND, exact count 16 after comment filtering)
- [x] `internal/semantic/store/migrations_types.go` (CurrentSchemaVersion + Migration types) (FOUND)
- [x] `internal/semantic/store/store_test.go` — 9 CGO=1 tests (FOUND)
- [x] `internal/semantic/store/duckdb_nocgo_test.go` — 2 CGO=0 tests (FOUND)
- [x] `internal/obs/metrics.go` carries `SemanticStoreQuarantine` + `SemanticStoreOpen` vectors + helper methods (FOUND)
- [x] `internal/obs/metrics_labels_test.go` carveOuts entries for both new families (FOUND)
- [x] `internal/config/config.go` — `SemanticIndex semantic.Config` field present, no koanf tag (FOUND)
- [x] `internal/daemon/daemon.go` — DAG-04 marker on newDaemon; step 6b inserted between 6a and 6 (FOUND)
- [x] `internal/daemon/daemon_semantic_test.go` — integration build tags `cgo && integration` (FOUND)
- [x] Commit `6aac291b` exists (FOUND)
- [x] Commit `c30a2098` exists (FOUND)
- [x] Commit `6fea720b` exists (FOUND)
- [x] Commit `36a97460` exists (FOUND)
