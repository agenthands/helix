---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 01
subsystem: observability / semantic-store
tags: [ablation, metrics, prometheus, duckdb, ABLATE-06, tdd]
requires:
  - "internal/obs Metrics (prometheus counter family)"
  - "internal/semantic/store *Store DuckDB read paths"
provides:
  - "helix_semantic_store_reads_total Prometheus counter (net-new, labelless)"
  - "Metrics.SemanticStoreReadsInc() helper (nil-safe)"
  - "store.queryContext / store.queryRowContext read chokepoint wrappers"
  - "TestReadCounter (increments on a real read; stays 0 with no read)"
affects:
  - "81-04 (gate wiring — relies on the read counter to verify Noop forcing)"
  - "81-05 (bench-cell assertion — asserts this counter == 0 on no_semantic)"
tech-stack:
  added: []
  patterns:
    - "Single read chokepoint: thin *Store wrapper methods that Inc the counter then delegate to s.db.Query*"
    - "Labelless prometheus counter for a faithful 'any read happened' signal (D-05)"
    - "Reads-only by construction: writes/maintenance deliberately NOT routed (T-81-01-02)"
key-files:
  created:
    - internal/semantic/store/reads_counter_test.go
  modified:
    - internal/obs/metrics.go
    - internal/obs/metrics_labels_test.go
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/overlay.go
    - internal/semantic/store/filefact_accessor.go
    - internal/semantic/store/snapshot.go
decisions:
  - "Chokepoint design: reads do NOT funnel through one existing helper; introduced s.queryContext / s.queryRowContext wrappers and routed every genuine s.db SELECT through them (single Inc site each)."
  - "Counter is labelless — a faithful 'a semantic-store read happened' signal, lowest cardinality; the no_semantic arm asserts the total == 0."
  - "Scope expansion (Rule 2): routed read sites in filefact_accessor.go + snapshot.go beyond the plan's declared files_modified to achieve zero false negatives (T-81-01-01 mitigation)."
metrics:
  duration: "~20 min"
  completed: 2026-06-20
  tasks: 2
  files: 7
---

# Phase 81 Plan 01: helix_semantic_store_reads_total Counter + Read Chokepoint Instrumentation Summary

Net-new labelless `helix_semantic_store_reads_total` Prometheus counter plus a single-chokepoint instrumentation of the `internal/semantic/store` DuckDB read paths, so every semantic-store read increments it and the Phase 81 `no_semantic` bench cell can assert the total == 0 (D-05) against a real signal rather than a phantom metric (Pitfall 1).

## What Was Built

- **Counter + helper (`internal/obs/metrics.go`):** registered `helix_semantic_store_reads_total` (a labelless `prometheus.Counter`, mirroring the `helix_semantic_store_open_total` registration block), added it to `MustRegister`, and added the nil-receiver-safe `Metrics.SemanticStoreReadsInc()` helper alongside `SemanticStoreOpenInc`.
- **Read chokepoint (`internal/semantic/store/effective_graph.go`):** added two thin `*Store` wrapper methods — `queryContext` and `queryRowContext` — that call `s.metrics.SemanticStoreReadsInc()` (nil-guarded) then delegate to `s.db.QueryContext` / `s.db.QueryRowContext`. Every genuine semantic-store SELECT now routes through these.
- **Routing:** replaced direct `s.db.Query*` calls at the read sites with the wrappers (see chokepoint list below).
- **Labels test (`internal/obs/metrics_labels_test.go`):** primed the labelless counter (so `Gather()` returns its family) and added a documentation-parity carve-out entry.
- **Test (`internal/semantic/store/reads_counter_test.go`):** `TestReadCounter` — subtest "increments on a real read" (drives `QueryEffectiveAdjacency` on an empty store, asserts the counter bumps) and "stays 0 when no read occurs" (opens the store, performs no read, asserts the counter is exactly 0).

## Task 0 (Wave 0) — Read Chokepoint Discovery (exact file:line)

The semantic-store reads do **NOT** funnel through a single pre-existing helper — they call `s.db.QueryContext` / `s.db.QueryRowContext` directly across several files. The single faithful chokepoint was therefore **introduced** as `*Store.queryContext` / `*Store.queryRowContext` (the single Inc site), and the read call sites were routed through them.

**Routed read sites (the minimal complete set covering RankFiles / ExpandFrom / retrieval read paths):**

| File:Line | Method | Covers |
|-----------|--------|--------|
| `internal/semantic/store/effective_graph.go:145` | `QueryEffectiveAdjacency` | rankStoreAdapter / RankScheduler (RankFiles path) |
| `internal/semantic/store/effective_graph.go` (CountStaleScoreRows, IterateCommittedSymbols, cluster/impact/score/symbol reads — all 22 `s.db.Query*` sites) | effective-graph reads | ExpandFrom / cluster / impact / retrieval corpus |
| `internal/semantic/store/overlay.go:275` | `OverlayRowCount` | compaction-gate effective read |
| `internal/semantic/store/overlay.go:992` | `CurrentGraphVersion` | overlay meta read |
| `internal/semantic/store/overlay.go:1028` | `CurrentOverlayEpoch` | overlay meta read |
| `internal/semantic/store/overlay.go:1071,1082` | `OverlayChangedPathsSince` | incremental-refresh read seam |
| `internal/semantic/store/filefact_accessor.go:102,117,169,180` | `GetLatestFileFact` (readOverlayFileFact + readSnapshotFileFact) | FileFact retrieval read |
| `internal/semantic/store/snapshot.go:795` | `LatestCommittedSnapshotBaseEpoch` | incremental-refresh base-epoch read |

**Deliberately NOT routed (reads-only by construction, T-81-01-02):**
- `duckdb.go:310` — schema-version read during `Open` (open/maintenance, not a tool read; keeps the "stays 0 after open" contract true).
- `overlay.go:129` — `BeginOverlayTx` `UPDATE ... RETURNING` epoch bump (a write).
- `snapshot.go:270` — `nextval(...)` sequence allocation (a write-side allocation).
- `snapshot.go:558`, `overlay.go:694,967` — `t.tx.Query*` tx-scoped snapshot-build queries (write transactions, not the `s.db` read handle).
- `migrations.go`, `migrations_registry.go` — schema/migration maintenance.

## How It Works

`ChooseSource` / RankFiles / ExpandFrom / retrieval all ultimately execute their SELECTs on `*Store` via `s.db.Query*`. Every such read is now a single `Inc` on the labelless counter. The counter starts at 0 after `Open` (open-time schema-version read is excluded), so the bench-cell assertion `helix_semantic_store_reads_total == 0` is faithful: it proves no read happened, not that there was nothing to read (the D-04 build-but-block store is still constructed by later plans).

## Deviations from Plan

### Auto-fixed / scope-completion

**1. [Rule 2 - missing critical functionality] Routed read sites in `filefact_accessor.go` and `snapshot.go` beyond the plan's declared `files_modified`.**
- **Found during:** Task 0 chokepoint enumeration.
- **Issue:** The plan's `files_modified` listed `effective.go`, `effective_graph.go`, `overlay.go`. But `GetLatestFileFact` (filefact_accessor.go) and `LatestCommittedSnapshotBaseEpoch` (snapshot.go) are genuine semantic-store read paths consumed by retrieval / incremental refresh. Leaving them un-instrumented would be a T-81-01-01 information-disclosure leak (a read path bypassing the counter → a false-negative on the zero-read assertion).
- **Fix:** Routed their `s.db.Query*` read calls through the same `queryContext` / `queryRowContext` chokepoint wrappers. `effective.go` was untouched (it is documentation-only; the implementation it documents lives in `duckdb.go`/`effective_graph.go`).
- **Files modified:** `internal/semantic/store/filefact_accessor.go`, `internal/semantic/store/snapshot.go`
- **Commit:** 1837af75

## TDD Gate Compliance

- RED commit `82b4e3cf test(81-01): add failing read-counter test` — `TestReadCounter/increments_on_a_real_read` failed (counter absent), as captured.
- GREEN commit `1837af75 feat(81-01): ...` — `TestReadCounter` passes.
- No REFACTOR commit needed: the single-wrapper design is already the consolidated form.

## Verification

- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./internal/semantic/store/... ./internal/obs/... -count=1` — PASS (store 5.4s, obs 10.0s)
- `go test ./internal/semantic/store/ -run TestReadCounter -count=1` — PASS
- `grep -c "helix_semantic_store_reads_total" internal/obs/metrics.go` = 2 (≥ 1)
- `git log` shows `test(81-01)` preceding `feat(81-01)` (RED→GREEN gate)

## Commits

- `82b4e3cf` test(81-01): add failing read-counter test
- `1837af75` feat(81-01): add helix_semantic_store_reads_total + instrument DuckDB read chokepoint

## Self-Check: PASSED

All created/modified files present on disk; both task commits present in git history.
