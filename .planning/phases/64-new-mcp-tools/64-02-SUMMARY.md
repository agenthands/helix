---
phase: 64-new-mcp-tools
plan: 02
subsystem: semantic-store
tags: [duckdb, scheduler-store, effective-graph, pagerank-status, snapshot-iteration, tdd]

requires:
  - phase: 57
    provides: "Schema 1 semantic_edges, semantic_symbols, semantic_files; *Store open/close lifecycle"
  - phase: 60
    provides: "Schema 3 semantic_live_overlay_edges status='live'|'deleted' tombstone discriminator; D-04 CAS contract"
  - phase: 62
    provides: "graph.SchedulerStore interface (lines 43, 56, 61); rank_wiring.go stubs reserving the contract"
  - phase: 63
    provides: "Schema 5 semantic_snapshot_id_seq SEQUENCE; CompactionGate; status='committed' invariant"
provides:
  - "Production *Store.QueryEffectiveAdjacency, CountStaleScoreRows, MarkAllScoreRowsStale, LatestCommittedSnapshot, IterateCommittedSymbols"
  - "Exported SymbolRow type (semantic-store package surface)"
  - "Phase 64 P64-04 status surface, P64-06 context surface, P64-07 retrieval/recovery now have a stable Store dependency"
affects: [64-04-status, 64-06-context, 64-07-retrieval-recovery, 64-08-daemon-wiring]

tech-stack:
  added: []
  patterns:
    - "Effective-view UNION ALL across snapshot + overlay tables filtered by closed-enum tombstone status"
    - "Lock-free read queries on s.db (no LockOverlayWorkspace) for SchedulerStore probe methods"
    - "JOIN-resolved path for streamed symbol iteration (Path source-of-truth lives in semantic_files, not semantic_symbols)"

key-files:
  created:
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/effective_graph_test.go
  modified: []

key-decisions:
  - "edge_kind IS the projection axis for both semantic_edges and semantic_live_overlay_edges (the interface's `projection` parameter maps to edge_kind for edges and to score_name for scores; same string value, two columns)."
  - "Snapshot edges restricted to the LATEST committed snapshot for repoID via subquery on semantic_snapshots — semantic_edges has no repo_id column, so the join through semantic_snapshots is the canonical filter."
  - "Tombstone gesture for overlay edges is status='deleted' (closed-enum text), NOT a boolean column — matches the existing MarkEdgesDeleted contract."
  - "SymbolRow.Docstring kept on the type but emitted as empty string until a future migration materializes the column. P64-07 bleve corpus loader reads docstrings from source bytes via the LineStart anchor."
  - "Path JOINed in via semantic_files (LEFT JOIN, COALESCE to '') so SymbolRow is self-contained — the bleve corpus mapper does not need a second round-trip per symbol."
  - "LatestCommittedSnapshot returns 0+nil for an empty store (NOT an error) — pending/aborted snapshots are excluded; callers want a snapshot whose facts are actually visible."

patterns-established:
  - "Pattern 1: Effective-view UNION ALL with snapshot-side JOIN filter — repo-scoped reads against tables with no repo_id column resolve repoID through semantic_snapshots."
  - "Pattern 2: Lock-free SchedulerStore read methods — reads go on s.db (statement-scoped), not on a per-tx Tx; no LockOverlayWorkspace acquisition. Honors the scheduler_store.go:48-56 contract that prevents self-deadlock when the scheduler invokes after release."
  - "Pattern 3: Streaming iteration with abort-on-false fn(SymbolRow) bool — caller controls flow; ctx cancellation propagated via QueryContext; deterministic ORDER BY symbol_id ASC mirrors Phase 62 sort-before-iterate doctrine."

requirements-completed: [TOOL-03, TOOL-04]

duration: 13min
completed: 2026-05-07
---

# Phase 64 Plan 02: Effective-Graph Queries Summary

**Production implementation of FIVE deferred SchedulerStore methods on `*Store` — adjacency union (snapshot ⊕ overlay − tombstones), stale/total score-row counts, idempotent stale-flip, latest-committed-snapshot lookup, and stable-sorted streaming symbol iteration — all lock-free, all schema-aware.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-05-08T21:36Z
- **Completed:** 2026-05-08T21:49Z
- **Tasks:** 2/2 (RED + GREEN gates of a TDD pair)
- **Files created:** 2 (effective_graph.go, effective_graph_test.go)
- **Files modified:** 0

## Accomplishments

- 5 production methods on `*Store` close the contract slot Phase 62 reserved with `rankStoreAdapter` stubs.
- 14 tests (4 adjacency + 2 count + 2 mark + 2 latest + 4 iterate) — all passing under `-count=1` and the race-safety probe under `-race`.
- Established the schema-reality vs PLAN-pseudocode reconciliation pattern: PLAN authored against pseudocode column names; implementation honors the actual Schema 1-5 layout (edge_kind not projection; status='deleted' not tombstone bool; score_name not projection).
- P64-04 status surface, P64-06 context surface, and P64-07 retrieval/recovery (the next-wave plans) now have a stable Store-interface dependency to compile against.

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests for the 5 effective-graph queries** — `1c48c405` (test)
2. **Task 2 (GREEN): Production implementation of effective_graph.go** — `4bb095a4` (feat)

## Files Created/Modified

- `internal/semantic/store/effective_graph.go` — Five method bodies + exported `SymbolRow` type. ~270 LOC including doc comments. Sole package surface for the effective-graph queries; all reads go on `s.db` (statement-scoped) per the lock-free contract.
- `internal/semantic/store/effective_graph_test.go` — 14 tests + 5 seed helpers (`seedCommittedSnapshot`, `seedSnapshotEdge`, `seedOverlayEdge`, `seedScoreRow`, `seedCommittedSymbol`). White-box (package store) so direct `s.db.ExecContext` INSERTs can stamp arbitrary fixture data without coupling to the BeginSnapshot SEQUENCE allocator.

## Verification

- `go vet ./internal/semantic/store/...` — clean.
- `go test ./internal/semantic/store/...` — all tests pass (`-count=1 -timeout=120s`).
- `go test ./internal/semantic/store/ -run='TestCountStaleScoreRows_LockFree_RaceSafe' -race -count=1 -timeout=120s` — clean.
- `gofmt -l internal/semantic/store/effective_graph.go internal/semantic/store/effective_graph_test.go` — empty.
- `go run ./cmd/vet-noduckdb ./internal/semantic/...` — clean (file is in the `internal/semantic/store/` package, allowed).
- `go test ./internal/daemon/... -run='TestRankStoreAdapter|TestStubObserve'` — pass (existing stub-observability harness still green; the `rankStoreAdapter` stubs still emit `stub_no_data` because P64-08 has not collapsed them yet).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Schema-shape reconciliation between PLAN pseudocode and actual migrations**

- **Found during:** Task 2 GREEN (also surfaced during Task 1 helper authoring).
- **Issue:** PLAN's pseudocode used `repo_id` and `projection` as columns on `semantic_edges` / `semantic_live_overlay_edges`, and `tombstone BOOLEAN` for overlay tombstones, and `path`/`docstring` columns on `semantic_symbols`. None of those exist:
  - `semantic_edges` keyed by `(snapshot_id, edge_id)` — no `repo_id`.
  - Overlay edges use `status TEXT` ('live' | 'deleted') for tombstones.
  - Score table is `semantic_graph_scores`, not `semantic_pagerank_scores`; column is `score_name`, not `projection`.
  - Symbol table has neither `path` nor `docstring`.
- **Fix:** Resolved repoID for snapshot edges via JOIN on `semantic_snapshots` (filter `status='committed'` AND `snapshot_id = (SELECT MAX(...) ...)` for the latest committed snapshot); used `status='live'` as the non-tombstone discriminator for overlay edges; mapped the interface's `projection` argument to `edge_kind` for edges and to `score_name` for scores; LEFT JOIN to `semantic_files` for path resolution; emitted Docstring as empty string with a comment pointing P64-07's corpus loader at the source-bytes-by-LineStart strategy.
- **Files modified:** `internal/semantic/store/effective_graph.go`, `internal/semantic/store/effective_graph_test.go`.
- **Commits:** `1c48c405` (RED tests against actual schema), `4bb095a4` (GREEN implementation against actual schema).
- **Justification:** PLAN.md's `<action>` block explicitly authorizes this adjustment: "If migration uses different names or omits a column from the `semantic_symbols` projection (e.g., docstring lives in a sidecar table), adjust the SQL strings + `SymbolRow` columns and add a comment pointing at the migration file." The deviation is in-scope per the PLAN itself.

### Out-of-scope discoveries

None — no pre-existing failures or unrelated lint warnings surfaced during execution.

## TDD Gate Compliance

- **RED:** `1c48c405` — `test(64-02): add 14 failing tests for effective-graph queries`. All 14 tests fail with `errNotImplemented` against stubbed methods; `go vet` clean.
- **GREEN:** `4bb095a4` — `feat(64-02): implement effective-graph queries on *Store`. All 14 tests pass; race-safety probe clean under `-race`.
- **REFACTOR:** Not required — the implementation landed clean against the test set.

Both gate commits in chronological order — RED commit timestamp predates GREEN commit timestamp; `git log` confirms the sequence.

## Threat Flags

None — Phase 64-02 introduces no new network surface, auth path, or schema migration. The threat register (T-64-02-01..T-64-02-05) is fully addressed:

- T-64-02-01 (SQL injection): all queries use `?` parameter binds.
- T-64-02-02 (cross-repo data leak): repoID filter is the caller's responsibility; mode-tier check enforced upstream by P64-01..P64-05's skill handlers.
- T-64-02-03 (DoS via unbounded iteration): `IterateCommittedSymbols` streams via `QueryContext`; caller's `fn` controls flow.
- T-64-02-04 (concurrent-write race on score rows): single SQL `UPDATE` is atomic at the DuckDB tx level; lock-free reads honor the scheduler_store.go:48-56 contract.
- T-64-02-05 (slow consumer DoS): `Recoverer.Probe` controls iteration via `fn` return value; ctx cancellation honored.

## Cross-Phase Wires

- `rankStoreAdapter` (internal/daemon/rank_wiring.go:337-356) — currently still stubbed; P64-08 daemon wiring plan will collapse the stub bodies to verbatim `return a.store.X(...)` delegation. No change to that file in this plan.
- P64-04 status surface — will consume `LatestCommittedSnapshot` for the `latest_snapshot_id` field of the SPEC §23.3 envelope.
- P64-06 context surface — will consume `QueryEffectiveAdjacency` for graph-rank inputs.
- P64-07 retrieval/recovery — will consume `IterateCommittedSymbols` to rebuild the bleve segment from a committed snapshot when daemon-restart detects a mismatch (per CONTEXT.md D-08 dual-store recovery).

## Self-Check: PASSED

- `[x] internal/semantic/store/effective_graph.go` exists at expected path.
- `[x] internal/semantic/store/effective_graph_test.go` exists at expected path.
- `[x] commit 1c48c405` present in `git log` (Task 1 RED).
- `[x] commit 4bb095a4` present in `git log` (Task 2 GREEN).
- `[x] go vet` clean.
- `[x] go test` (all 14 tests + race probe) passing.
- `[x] gofmt -l` empty.
- `[x] vet-noduckdb` clean for `internal/semantic/...`.
- `[x] all 5 method bodies present in effective_graph.go (grep verification: QueryEffectiveAdjacency, CountStaleScoreRows, MarkAllScoreRowsStale, LatestCommittedSnapshot, IterateCommittedSymbols).`
- `[x] type SymbolRow struct present (substring match).`
