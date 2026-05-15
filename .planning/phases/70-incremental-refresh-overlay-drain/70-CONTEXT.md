# Phase 70 — Incremental Refresh Overlay-Drain — CONTEXT

**Date:** 2026-05-15
**Goal (from ROADMAP):** `refresh_semantic_graph` with `mode:"incremental"` consults the overlay-drain seam to refresh only changed files; full-walk becomes a verified fallback, not the default.
**Requirements:** REFRESH-01, REFRESH-02, REFRESH-03
**Depends on:** Phase 60 (overlay-drain in live pipeline), Phase 64 (refresh tool), Phase 68 (precise diffs make incremental refresh actually narrow)

## Domain

Phase 70 closes the incremental-refresh degraded path documented in `internal/daemon/semantic_wiring.go:1466-1480` (the "refresh-degraded" annotation cited in success criterion #5). Today, `collectCandidatePaths(ws, mode)` falls through to a full directory walk for `mode=="incremental"` because the live overlay does not expose an enumerable per-path drain surface to non-handler callers.

This phase ships the missing seam — a persistent, idempotent **since-epoch** accessor on `*Store` that returns the set of paths whose overlay rows have changed since a caller-supplied baseline epoch — and wires it into both production buildFn (incremental mode) and the `refresh_semantic_graph` tool's `files_updated` accounting.

**Phase 70 ships:**

1. **`*Store.OverlayChangedPathsSince` accessor** (D-09 read-path, lock-free) returning paths + the current overlay epoch. Empty result is a contract-preserving signal that the caller MUST fall back to full-walk.
2. **`collectCandidatePaths` rewrite** for `mode=="incremental"`: consult the seam, fall back to full-walk only when seam returns empty (overlay rotated, cold-start, or quiet workspace). Bounded-label warn metric on every fallback.
3. **`refresh_semantic_graph.files_updated` honesty fix** — the tool currently reports `len(args.Paths)` (or `0` for the unfiltered drain) per `internal/skill/semantic/tools_refresh.go:170-176`. With the new seam, the count comes from the seam's returned path set, restoring envelope honesty without changing the tool's read-only contract.
4. **`refresh-degraded` annotation removed** from `semantic_wiring.go` — the comment block that justified the full-walk-fallback default is replaced by a note pointing at the seam.
5. **Bench harness** in `internal/eval/runner/` reusing the Phase 64 P07 / Phase 67 fixture, scaled to 10k symbols, asserting p95 ≤ 200ms locally for the single-file-changed incremental case. Bench is `t.Skip`'d under CI per project rule (no CI benchmarks).
6. **`internal/eval/runner/refresh_incremental_test.go`** verifying both the incremental path (single-file change → only that file's facts touched) and the fallback path (empty seam → full-walk + bounded-label log with the fallback reason).

**Out of scope (deferred):**

- Per-language extractor changes — Phase 68 already lands precise FileFactDiff; Phase 70 only consumes the path set, not the per-symbol delta.
- Bench-CI integration — local-only by project rule (`feedback_no_ci_benchmarks` memory).
- Concurrent multi-file edits under refresh — covered by Phase 60 coalescer contract; not a Phase 70 obligation.
- Per-projection score recompute optimization — Phase 62 territory.

## Canonical Refs

**Downstream agents MUST read these before planning or implementing.**

### Phase 70 anchors
- `.planning/ROADMAP.md` — Phase 70 success criteria (5 items)
- `.planning/REQUIREMENTS.md` — REFRESH-01, REFRESH-02, REFRESH-03
- `internal/daemon/semantic_wiring.go:1466-1480` — current `collectCandidatePaths` "refresh-degraded" annotation (the comment block to remove)
- `internal/daemon/semantic_wiring.go:1380-1410` — buildFn dispatch for `mode=="incremental"` and `LatestCommittedSnapshot`
- `internal/skill/semantic/tools_refresh.go:130-237` — `handleRefreshSemanticGraph` (D-09/D-13 read-only invariant; `files_updated` wart at 170-176)
- `internal/skill/semantic/tools_index.go` — `index_semantic_graph` (review+/admin) consumer of buildFn

### Overlay-drain seam (the data this phase exposes)
- `internal/semantic/store/overlay.go` — `OverlayTx`, `BeginOverlayTx`, `OverlayHasPendingRows`, `CurrentOverlayEpoch` (existing accessors; new accessor lands alongside)
- `internal/semantic/store/overlay.go:1004-1019` — `CurrentOverlayEpoch` reference shape for the new accessor
- `internal/semantic/store/snapshot.go` — `Snapshot` shape; baseline-epoch storage location decision
- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` — D-04 monotonic `overlay_epoch` per workspace (the persisted contract Phase 70 reads from)

### Patterns to mirror
- `.planning/phases/68-precise-filefactdiff-populator/68-CONTEXT.md` — D-01 (Store accessor over handler-local cache; restart-safe)
- `.planning/phases/69-production-status-accessors/69-CONTEXT.md` — D1 (lock-free `*Store` read accessor pattern)
- `internal/semantic/store/filefact_accessor.go` — Phase 68 accessor implementation (template for the new accessor's signature + tests)

### Eval / bench fixture reuse
- `.planning/phases/64-new-mcp-tools/64-07-PLAN.md` + `64-07-SUMMARY.md` — `TestE2E_IndexThenContext_SymbolCount` 15-symbol bleve fixture (extend to 10k symbols for bench)
- `.planning/phases/67-evaluation-harness/67-05-runner-and-reporters-PLAN.md` — eval-runner harness shape
- `internal/eval/runner/runner.go`, `inprocess.go`, `inprocess_fixtures_test.go` — fixture-build patterns to extend
- `internal/eval/runner/scripted_agent.go` — bench-side agent driver for single-file-change scenarios

### Boundary invariants
- `.planning/codebase/CONVENTIONS.md` — D-09 (no Begin/Commit/Abort/Write on read path); closed-enum bounded labels for fallback metric
- `tools_refresh.go:14-23, 124-129` — D-09 / D-13 invariant comments (refresh stays overlay-only, never commits a snapshot)
- Build target / Makefile rule `vet-nokernel2semantic` — kernel↔semantic boundary; the new accessor lives semantic-side

## Decisions

### D1 — Both `index_semantic_graph(mode=incremental)` and `refresh_semantic_graph` consume the same overlay-drain seam

ROADMAP wording ("`refresh_semantic_graph` with `mode:'incremental'`") and the cited code anchor (`collectCandidatePaths`, called only by `index_semantic_graph`'s buildFn) describe the same underlying need: enumerate the changed-path set without walking the entire workspace. Phase 70 satisfies both by introducing one seam and wiring two consumers:

- **`index_semantic_graph(mode=incremental)`** consumes the seam through `collectCandidatePaths`. Closes REFRESH-01 directly.
- **`refresh_semantic_graph`** consumes the seam to compute its `files_updated` count honestly (today it returns `len(args.Paths)` or `0`).

**Why one seam, two consumers:**
- Honors the ROADMAP intent without adding a `mode` arg to `refresh` (which would blur the D-09/D-13 read-only boundary).
- Single source of truth — both tools agree on what "changed paths" means.
- Fixes a known reporting wart in `refresh` without scope creep.

**Rejected:** narrow scope (index-only). Acceptable but leaves `refresh.files_updated` lying. Rejected: adding `mode` arg to `refresh` (schema change; conflates read/admin tiers).

### D2 — Seam shape: since-epoch idempotent read on `*Store`

```go
// internal/semantic/store/overlay.go (or sibling overlay_drain.go)
func (s *Store) OverlayChangedPathsSince(
    ctx context.Context,
    repoID string,
    baseEpoch uint64,
) (paths []string, currentEpoch uint64, err error)
```

**Contract:**
- Returns the set of distinct file paths whose overlay rows have a `write_epoch > baseEpoch` for the given `repoID`, plus the current overlay epoch at read time.
- Lock-free read (no overlay tx); honors D-09 — no Begin/Commit/Abort/Write/snapshot calls.
- `baseEpoch == 0` → returns ALL current overlay paths (cold-start signal; caller falls back to full-walk).
- Empty `paths` with non-zero `baseEpoch` → workspace was quiet since baseline; caller falls back to full-walk for incremental snapshot completeness.
- Idempotent: multiple callers reading the same `(repoID, baseEpoch)` get identical results until a new overlay tx commits.

**Why since-epoch over drain-and-mark:**
- Two-consumer-safe (D1 requires this).
- Restart-safe: `overlay_epoch` is persisted (Phase 60 D-04, schema v3). The baseline survives daemon restart.
- Pattern matches Phase 68 (`GetLatestFileFact`) and Phase 69 (`ClusterStatusForGraphVersion`) — `*Store` accessors as the single source of truth.

**Rejected:** drain-and-mark consumed (mutates on read; single-consumer; breaks D-09). Coalescer-side in-memory accessor (lost across daemon restart; silently full-walks after restart — same hole Phase 68 D-01 rejected).

### D3 — Baseline epoch source: per-snapshot `base_overlay_epoch` field

For `index_semantic_graph(mode=incremental)`:
- The baseline epoch is the `overlay_epoch` captured at the time of the most recent committed snapshot for `repoID`.
- Stored on the snapshot row (new `base_overlay_epoch uint64` column on `snapshots` or sibling key — researcher picks the cheapest schema delta).
- Cold-start (no prior snapshot, or `base_overlay_epoch = 0`) → seam returns all overlay paths → if non-empty, drive incremental from those; if empty, fall back to full-walk.

For `refresh_semantic_graph`:
- The baseline epoch is the overlay epoch at the START of the refresh call (captured via `CurrentOverlayEpoch` before invoking `OnWorkspaceChanged`).
- After the drain completes, the seam is queried with that pre-drain epoch to enumerate paths that just landed → that's `files_updated`.
- This makes `files_updated` honest for both filtered (`args.Paths`) and unfiltered drains.

**Why per-snapshot baseline (not config-driven):**
- The "since" question is naturally "since I last committed" for the index tool, "since the start of this call" for the refresh tool. No global state needed.
- Snapshot table is the existing rendezvous point for incremental — `LatestCommittedSnapshot` is already called in buildFn at line ~1385.

**Researcher / planner discretion:** Whether `base_overlay_epoch` is a new column on `snapshots` or a sibling table indexed by `snapshot_id`. Smallest patch wins.

### D4 — Fallback policy: full-walk on empty seam, bounded-label reason metric

When `OverlayChangedPathsSince` returns empty for incremental mode, `collectCandidatePaths` falls back to the existing full-walk and emits a single bounded-label warn metric:

```
helix_incremental_refresh_fallback_total{reason, repo}

reason ∈ {"cold_start", "overlay_rotated", "empty_overlay"}
```

- `cold_start` — `baseEpoch == 0` (no prior committed snapshot for this repo).
- `overlay_rotated` — `baseEpoch > 0` but the current overlay's minimum tracked epoch exceeds `baseEpoch` (overlay was rotated/compacted past the baseline).
- `empty_overlay` — `baseEpoch > 0`, baseline still in range, but no rows changed since.

The fallback log line uses the same `reason` token (closed enum, bounded cardinality). Honors success criterion #3 verbatim.

**Detection of `overlay_rotated`:** The seam returns an additional sentinel (e.g., `(_, _, ErrOverlayRotated)` or a third return value) when the baseline is below the overlay's earliest retained epoch. Researcher to confirm the cleanest sentinel against Phase 63 compaction's epoch-floor accessor.

### D5 — Test surface

Two test files, per success criterion #4:

1. **`internal/eval/runner/refresh_incremental_test.go`** (REFRESH-03) — table-driven:
   - **Incremental path:** seed a workspace with 3 files, commit a snapshot (baseline epoch captured), edit 1 file, run incremental refresh. Assert exactly 1 path in the candidate set; full-walk did NOT execute (counted via a test-seam hook on `collectCandidatePaths`).
   - **Fallback paths (3 sub-tests):** cold-start (no prior snapshot), overlay-rotated (force compact past baseline), empty-overlay (baseline still in range, no edits). Each asserts: full-walk ran AND the fallback metric incremented with the matching `reason` label AND the bounded-label log line was emitted.
   - Race-clean (`go test -race` default).

2. **Bench harness** in `internal/eval/runner/bench_refresh_incremental_test.go` (REFRESH-02):
   - Reuses the Phase 64 P07 fixture builder, scaled to 10k symbols (parameterize `numSymbols` on the existing builder; researcher to confirm).
   - Edits 1 file, runs incremental refresh, samples 50+ iterations, asserts `p95 ≤ 200ms`.
   - Guarded by `t.Skip` when `os.Getenv("CI") != ""` (project rule: no CI benchmarks).

**Why eval-runner location:** Success criterion #4 names `internal/eval/runner/refresh_incremental_test.go` verbatim. Bench co-locates so they share fixture builders.

### D6 — `refresh-degraded` annotation removal

The `collectCandidatePaths` comment block at `semantic_wiring.go:1466-1480` is replaced by a brief note pointing to the seam:

```go
// collectCandidatePaths builds the candidate path set the production buildFn
// classifies + extracts.
//   - mode=full: walk ws.RepoRoot via filepath.WalkDir (.helix, .git, dot-dirs excluded).
//   - mode=incremental: query OverlayChangedPathsSince(baseEpoch); on empty
//     result, fall back to full-walk and emit the bounded-label fallback metric.
```

No multi-paragraph degraded-mode rationale remains. Closes success criterion #5.

### D7 — Boundary invariants preserved

- `refresh_semantic_graph` stays read-only on the snapshot surface. The seam is a read accessor; the existing CI grep-gate on `tools_refresh.go` continues to enforce absence of `Begin/Commit/Abort/Write` snapshot tokens.
- New accessor lives semantic-side (`internal/semantic/store/`); `vet-nokernel2semantic` stays green.
- `index_semantic_graph(mode=incremental)` continues to commit a new snapshot — the seam only changes WHICH paths it commits over, not WHETHER it commits.

## Implementation Notes (for researcher / planner)

- The accessor's underlying query: `SELECT DISTINCT path FROM overlay_files WHERE repo_id = ? AND write_epoch > ?`. Confirm `overlay_files` carries `write_epoch` (Phase 60 D-04 says yes); add an index `(repo_id, write_epoch)` if absent.
- `LatestCommittedSnapshot` already returns the snapshot ID at line ~1385 of buildFn. Extend the same call (or a sibling) to fetch `base_overlay_epoch` so buildFn has both in one round-trip.
- The `base_overlay_epoch` write happens at snapshot commit time — `CommitSnapshot` (semantic_wiring.go:~1447) captures `CurrentOverlayEpoch` and persists it on the snapshot row.
- For `refresh_semantic_graph`, the `OnWorkspaceChanged` call is asynchronous in spirit but Phase 60 D-01 guarantees the coalescer flush completes within `max_batch_delay_ms` of the call. The seam read after the call returns the post-flush state — researcher to confirm whether a small wait/poll is needed before the read, or whether `OnWorkspaceChanged` already blocks on flush.
- Closed-enum reason values for the fallback metric MUST be declared as Go constants in the same package as the metric registration (mirror Phase 68 D-08 pattern).
- Bench fixture scale: 10k symbols ≈ ~200 Go files of ~50 symbols each. Parameterize the Phase 64 P07 builder rather than hard-code a new generator.
- The Phase 64 P07 fixture is currently sized at 15 symbols — researcher to verify the builder is parameterizable; if not, the bench landing also lifts the symbol count from a const to a builder arg (small extraction).

## Deferred Ideas

- **Per-projection incremental refresh** — currently the seam is graph-wide; a future phase could narrow per-projection if score recompute becomes the bottleneck.
- **Streaming refresh progress** over MCP for long incremental runs — UX add-on, not in scope.
- **Cross-snapshot path-delta accessor** — useful for diff-style observability tools; out of scope for REFRESH-*.
- **Bench-CI integration** — explicitly rejected by project rule (`feedback_no_ci_benchmarks`); the bench stays local-only.
- **Coalescer-side in-memory cache** in front of the seam — defer until profiling shows DB read latency on the hot path matters (mirrors Phase 68's deferred LRU).

## Spec Lock

No SPEC.md present for Phase 70. Requirements come from `.planning/REQUIREMENTS.md` (REFRESH-01/02/03) and ROADMAP success criteria.

---

*Phase: 70-incremental-refresh-overlay-drain*
*Context gathered: 2026-05-15 via /gsd-discuss-phase*
