# Phase 68: Precise FileFactDiff Populator - Context

**Gathered:** 2026-05-13
**Status:** Ready for planning
**Source:** /gsd-discuss-phase

<domain>
## Phase Boundary

Phase 68 wires the production **FileFactDiff populator** into the
existing Phase 62-09 `FileFactDiffRecorder` seam so live edits advance
`graph_version` with precise per-symbol/per-edge deltas instead of the
Tier-3 synthetic marker that ships today.

The infrastructure already exists (62-09 closure):
- `FileFactDiffRecorder` tx-scoped recorder seam on `Handler`
- `populateRecorderForFile` dispatch point with Tier-1/Tier-2/Tier-3
  scaffolding in `internal/semantic/live/handler/difffacts.go`
- `ApplyRepair` post-commit hook + `graph_version` bump on non-empty
  recorder (Phase 62 P02)
- `ExtractionStatus` enum (`ready`/`partial`/`unsupported`/`failed`)
  on `internal/semantic/extract.ExtractedFile`
- Tier-3 synthetic-marker fallback (currently active in production)

**Phase 68 ships:**

1. **Pre-edit FileFact accessor** on `*Store` — `GetLatestFileFact(ctx,
   repoID, path) (extract.FileFact, bool, error)` reading
   `semantic_overlay_facts` first with snapshot-table fallback. Survives
   daemon restart. Returns `(_, false, nil)` when no prior fact exists
   (cold-start).
2. **Synchronous per-file extractor invocation** inside the live tx.
   Handler calls a new per-language `ExtractFile(ctx, repoID, path)
   (extract.ExtractedFile, error)` shim after `UpsertOverlayFile` and
   before `tx.Commit()`. Tier-1/2 diff is precise per edit.
3. **Tier-1 (full diff) populator** — when prior FileFact found AND
   extraction `ready`: diff `priorFact.Symbols`↔`newFact.Symbols` →
   `RecordSymbolAdded/Removed/Changed`; diff edges →
   `RecordEdgeAdded/Removed`.
4. **Tier-2 (added-only) populator** — when extraction `partial`:
   record every extracted symbol via `RecordSymbolAdded` (no diff
   against prior).
5. **Tier-3 restriction** — synthetic marker fires ONLY when prior
   FileFact missing (cold-start) OR extraction `failed` /
   `unsupported`. Bounded-label warn metric on this path.
6. **Outcome metric** — single counter
   `helix_live_filefactdiff_total{tier="full|added-only|synthetic",
   repo}` with bounded labels.
7. **E2E test** — `internal/semantic/live/handler/handler_diff_e2e_test
   .go`: Go-only fixture, `-race` clean, edits a single Go symbol,
   asserts `RecordSymbolChanged` fires exactly once and `ApplyRepair`
   receives non-empty `GraphRepair`.

**Out of scope (deferred):**

- TypeScript E2E fixture — Go-only proof of contract this phase;
  TS verified by unit-level recorder tests.
- Hybrid lane policy (sync HelixEdit / async background) — rejected in
  favor of uniform synchronous extraction across both lanes.
- Handler-local LRU cache in front of the Store accessor — rejected;
  Store accessor is the single source of truth. Cache can be added later
  if profiling shows DB read latency on hot path matters.
- Concurrent same-file edit subtest — covered by existing recorder
  concurrency contract (single-goroutine tx ownership); not a Phase 68
  obligation.

</domain>

<decisions>
## Implementation Decisions

### Pre-edit FileFact seam

- **D-01: New `*Store.GetLatestFileFact` accessor.** Adds a method on
  `internal/semantic/store.Store` with signature:
  ```go
  func (s *Store) GetLatestFileFact(ctx context.Context, repoID string, path string) (extract.FileFact, bool, error)
  ```
  Reads `semantic_overlay_facts` first (live overlay state). On miss,
  falls back to the latest committed snapshot tables. Returns
  `(_, false, nil)` on cold-start (no prior fact). Errors surface only
  for DB I/O failures.

  **Why store accessor over handler-local cache:**
  - Survives daemon restart (cache would be cold after every restart).
  - Single source of truth — overlay + snapshot already authoritative.
  - Matches existing Store API surface (`Begin*Tx`, `Write*Facts`,
    etc.).
  - Cache can be layered later under the same accessor signature
    without rewriting call sites.

- **D-02: Vet-boundary invariant.** The accessor lives in
  `internal/semantic/store` (semantic side). The handler call site
  already lives in `internal/semantic/live/handler`. No new
  `internal/kernel/*` imports added. `vet-nokernel2semantic` MUST stay
  green; the planner MUST include a verification step that runs the
  vet target.

### Extractor invocation surface

- **D-03: Synchronous per-file extractor shim, in-tx.** Adds an
  `ExtractFile(ctx, repoID, path) (extract.ExtractedFile, error)`
  method on the per-language provider surface
  (`internal/semantic/extract/{golang,typescript,...}`). The handler
  invokes it inside `updateChangedFileWithKind` after
  `UpsertOverlayFile` and before `tx.Commit()`. The result drives
  Tier-1/2 dispatch. The provider already extracts per-file in
  scheduler batch flow; the shim is a thin wrapper exposing the same
  computation on demand.

  **Why synchronous in-tx over scheduler-driven async:**
  - Tier-1 needs the post-edit FileFact synchronously to diff against
    the pre-edit fact captured by D-01. Async drift breaks the
    "graph_version advances per edit" invariant.
  - Deterministic test seam — E2E test can assert exact recorder call
    counts without waiting for scheduler tick.
  - Tx span widening is bounded (one extract call per edit; ≤ a few
    ms on a single file).

- **D-04: Same policy for HelixEdit lane and background lanes.** No
  hybrid sync/async split. Both lanes go through the synchronous
  populator. Simpler dispatch + single test surface.

### Tier dispatch policy

- **D-05: Tier mapping.**

  | Tier | Trigger | Recorder population |
  |------|---------|---------------------|
  | Tier-1 (full) | prior FileFact found AND `ExtractionStatusReady` | symbol+edge diff of prior↔new |
  | Tier-2 (added-only) | `ExtractionStatusPartial` (regardless of prior) | every extracted symbol → `RecordSymbolAdded` |
  | Tier-3 (synthetic) | prior FileFact missing (cold-start) OR `ExtractionStatusFailed` OR `ExtractionStatusUnsupported` | single `RecordSymbolChanged{KindChanged: true}` marker |

  Tier-3 emits a bounded-label warn metric so operators see
  cold-start vs persistent extractor failure separately (per
  success criterion 5).

- **D-06: graph_version contract preserved.** Recorder is non-empty in
  all three tiers (Tier-3 still records the synthetic marker), so
  `ApplyRepair` continues to fire on every live edit and
  `graph_version` continues to advance. Phase 62's single
  `BumpGraphVersion` call site (`apply_repair.go`) is untouched.

### Outcome metric

- **D-07: Single counter with bounded `tier` label.**
  - Name: `helix_live_filefactdiff_total`
  - Labels: `tier ∈ {"full","added-only","synthetic"}`, `repo`
  - One increment per `updateChangedFileWithKind` invocation,
    keyed by the tier the populator landed in.
  - Registered alongside existing live-handler metrics; bounded
    cardinality (3 tiers × N repos).

- **D-08: Tier-3 warn-metric distinction.** Inside the
  `tier="synthetic"` path, emit an additional bounded-label warn
  metric `helix_live_filefactdiff_synthetic_reason_total{reason ∈
  {"cold_start","extract_failed","extract_unsupported"}}` so
  operators can distinguish expected cold-start from persistent
  extractor breakage.

### E2E test

- **D-09: `handler_diff_e2e_test.go` shape.**
  - Path: `internal/semantic/live/handler/handler_diff_e2e_test.go`
  - Fixture: tiny Go workspace with one source file containing one
    exported function.
  - Edit: rewrite the function body so signature stays stable but
    `KindChanged`/`SignatureChanged`/`ExportedChanged` flip
    deterministically.
  - Assertions:
    1. `RecordSymbolChanged` fires exactly once (count via test
       seam).
    2. `ApplyRepair` is called with non-empty `GraphRepair`
       (`DirtyNodes` non-empty).
    3. Outcome metric records `tier="full"`.
  - Runs under `go test -race` by default in CI. Race-clean by
    construction — single-goroutine tx ownership invariant from
    62-09 already guarantees it; no concurrent subtest needed.

- **D-10: Test seam reuse.** Use the existing
  `SetPopulateRecorderForTest` export seam from
  `internal/semantic/live/handler/export_test.go` ONLY where unit
  tests need to inject a fake recorder. The E2E test exercises the
  REAL populator path — production code must be the path under test.

### Deferred follow-ups

- **D-11: DEF-67-F01-FULL-DIFF closure.** Phase 68 closes the
  remaining "Tier 1/Tier 2 scaffolding-only" debt left by the 62-09
  close-out. After Phase 68 lands, mark `DEF-67-F01-FULL-DIFF` as
  resolved in `.planning/deferred-items.md`.

### Claude's Discretion

- Exact column shape returned by `GetLatestFileFact` (whether it
  returns `extract.FileFact` directly or a store-side
  `FileFactRow`). Planner/researcher to choose what makes the diff
  call site cleanest.
- Diff algorithm internals — symbol matching by `stable_key`, edge
  matching by `(src,dst,kind)`. Researcher to confirm against
  existing `graphpkg.SymbolDiff` / `GraphEdge` types.
- Exact metric registration site (alongside `apply_repair` metrics
  vs new `difffacts.go` registration block).
- Per-language extractor shim implementation: whether `ExtractFile`
  wraps the existing batch extractor or is a parallel code path.
  Planner to choose smallest patch surface.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 62-09 closure (the seam Phase 68 populates)
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-09-PLAN.md` — original recorder-seam plan
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-09-SUMMARY.md` — what 62-09 shipped, what it deferred
- `internal/semantic/live/handler/handler.go` — `FileFactDiffRecorder` type, `updateChangedFileWithKind`
- `internal/semantic/live/handler/difffacts.go` — Tier-1/2/3 dispatch scaffolding (the file Phase 68 fills in)
- `internal/semantic/live/handler/export_test.go` — `SetPopulateRecorderForTest` test seam

### Phase 60 (live update pipeline)
- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` — D-02 per-event error policy + invariants
- `.planning/phases/60-live-update-pipeline/60-04-SUMMARY.md` — current FileFact upsert surface

### Phase 62 (graph engine + ApplyRepair)
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md` — graph_version advance machinery + `ApplyRepair`
- `internal/semantic/graph/repair.go` — `ComputeGraphRepair`, `IsEmpty`, `SymbolDiff` bit flags
- `internal/semantic/graph/apply_repair.go` — `BumpGraphVersion` (D-06 single call site)

### Snapshot store + extraction
- `internal/semantic/store/snapshot.go` — `FileFact`, `Snapshot`, snapshot tables (D-01 fallback path)
- `internal/semantic/extract/fact.go` — `ExtractedFile`, `ExtractionStatus` enum, `FileFact`
- `.planning/REQUIREMENTS.md` — DIFF-01..04, DEF-67-F01-FULL-DIFF history

### Vet boundary
- Build target / Makefile rule `vet-nokernel2semantic` — kernel↔semantic boundary invariant

</canonical_refs>

<specifics>
## Specific Ideas

- New `*Store` method name: `GetLatestFileFact` (mirrors existing
  `Begin*` / `Write*` naming on `internal/semantic/store.Store`).
- New per-language provider method name: `ExtractFile` (already
  referenced in `internal/semantic/extract/fact.go:184` as the
  expected provider surface).
- E2E test file path: `internal/semantic/live/handler/handler_diff_
  e2e_test.go` (matches success criterion 4 verbatim).
- Outcome metric name: `helix_live_filefactdiff_total` with bounded
  `tier` label.
- Synthetic-reason metric: `helix_live_filefactdiff_synthetic_reason
  _total`.

</specifics>

<deferred>
## Deferred Ideas

- **Handler-local LRU cache** in front of `GetLatestFileFact` —
  defer until profiling shows DB read latency on hot path matters.
- **TypeScript E2E fixture** — Go-only this phase; add TS fixture
  in a follow-up if per-language regression risk emerges.
- **Hybrid sync/async lane policy** — rejected; revisit only if
  synchronous extract widens tx span past observable budget.
- **Concurrent same-file edit subtest** — covered by 62-09
  concurrency contract; revisit only if a real race is observed.
- **Multi-projection PageRank metric label** on the outcome counter
  — out of scope; single CALL_GRAPH projection ships in v1.

</deferred>

---

*Phase: 68-precise-filefactdiff-populator*
*Context gathered: 2026-05-13 via /gsd-discuss-phase*
