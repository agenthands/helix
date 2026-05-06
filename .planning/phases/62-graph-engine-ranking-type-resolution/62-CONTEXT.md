# Phase 62: Graph Engine, Ranking & Type Resolution - Context

**Gathered:** 2026-05-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 62 ships the **deterministic ranking + clustering + type-resolution
layer** that consumes the validated edges Phase 61 produces and exposes
ranked, evidence-backed responses to Phase 64's MCP tools (`get_semantic_
context`, `index_semantic_graph`, `get_semantic_graph_status`).

**Phase 62 ships three sub-engines, all internal-only:**

1. **Generic deterministic PageRank engine** at `internal/graph/`,
   constrained over `cmp.Ordered` node-id types. Single CALL_GRAPH
   projection in v1; multiple-projection extensibility preserved.
   Repomap migrates to consume this same engine.
2. **`graph_version` advance machinery** — separate counter on
   `semantic_live_overlay_meta` (column already shipped, currently
   pinned at 0). Bumps inside `ApplyRepair` per SPEC §17.3 only when
   an overlay tx produces edge add/remove/change OR symbol
   stable_key/signature/exported/kind changes (SPEC §17.2 verbatim).
3. **Background incremental-repair scheduler** — idle-debounce
   goroutine drains stale rows; 1-hop frontier from changed nodes
   capped at `max_local_pagerank_nodes=5000`; >5000 falls back to
   all-stale + full recompute on longer idle. `score_status ∈ {exact,
   approximate, stale, missing}` per GRAPH-05.
4. **Personalized PageRank** with seed weighting (files + symbol
   names) under `pagerank.epsilon=1e-6`; tiebreak by stable-key
   NodeID per GRAPH-03.
5. **Weak-component clustering pass** — algorithm only, deterministic,
   shipped under `internal/semantic/cluster/`. Cluster MCP tools
   (`get_cluster_map`, `explain_cluster`) deferred to v1.10.x per the
   v1.10 roadmap.
6. **Per-language type resolvers** under `internal/semantic/types/
   {golang,python,typescript}/` plus a shared core in
   `internal/semantic/types/` (chain walker, fixpoint loop, evidence
   ladder). `RESOLVES_TO`, `CALLS`, `USES_TYPE` edges emitted with
   confidence per the SPEC §38.2 ladder.
7. **Access-chain resolver** with bounded fixpoint:
   `max_chain_depth=8`, `max_fixpoint_iterations=8`. Scope is
   per-package / compilation unit (Go: dir, TS: tsconfig project,
   Python: package via `__init__.py`).
8. **Two-phase comment-edge emission**. Comment-derived edges
   (TSDoc/JSDoc, Python type comments, GoDoc) emit at `confidence=0.60,
   source="comment.<kind>", validation_state="unresolved"`. When a
   matching LSP edge for the same `(src,dst,kind)` lands later via
   Phase 61 enrichment, `ApplyRepair` upgrades the comment edge
   in-place to `confidence=1.0, source="lsp.<call>", validation_state=
   "validated"`.
9. **`WriteInvalidations` consumer** — Phase 61 writes invalidations
   as a typed no-op stub. Phase 62 fills the consumer that turns
   invalidations into the `GraphRepair` input (SPEC §17.2).
10. **Best-effort stubs** for PHP/Ruby/Java type resolution. Java's
    LSP-derived `RESOLVES_TO`/`CALLS` edges already land at
    `confidence=1.0` via Phase 61, so the Java stub short-circuits on
    LSP-confirmed facts and emits `unresolved` for everything else.
    PHP/Ruby stubs return `unresolved` always in v1.

**Out of scope (deferred):**

- **Cluster MCP tools** (`get_cluster_map`, `explain_cluster`) — v1.10.x.
- **Multi-projection PageRank** — only `CALL_GRAPH` ships in v1; the
  other six SPEC §18.1 projections (REFERENCE_PAGERANK,
  FILE_DEPENDENCY_PAGERANK, TYPE_HIERARCHY_PAGERANK,
  CHANGE_IMPACT_PAGERANK, SECURITY_SURFACE_PAGERANK,
  RETRIEVAL_CONTEXT_PAGERANK) are deferred. Engine stays
  generic enough for follow-up phases to add them.
- **MCP tool surface** — `index_semantic_graph`, `refresh_semantic_
  graph`, `get_semantic_graph_status`, `get_semantic_context`
  (TOOL-01..04) are Phase 64. Phase 62 ships the underlying engines
  + a typed `Ranker` / `Resolver` API for Phase 64 to consume.
- **Receipts / guardrails on rank-derived edges** — Phase 66.
- **Cross-repo / multi-workspace ranking** — workspaces are
  independent.
- **Score-fusion across projections** (SPEC §18.5) — single projection
  in v1, no fusion needed.
- **Cluster labeling / summary generation** — out per the v1 roadmap.
- **Adaptive prioritization of `semantic_pending` files** based on
  rank — Phase 65/66+.

</domain>

<decisions>
## Implementation Decisions

### PageRank engine

- **D-01: Shared generic PageRank engine at `internal/graph/`.** A new
  top-level package `internal/graph/` exports a deterministic weighted
  PageRank over a generic node-id type. Both `internal/repomap` and
  `internal/semantic/graph` (or wherever Phase 62 wires the call site)
  consume it. Single algorithm to audit, single locus for the
  determinism patch.

  **Hard invariants:**
  - The package depends on `math`, `sort`, `cmp` only — no project
    imports. Keeps it usable from any layer.
  - Public surface: `func PageRank[T cmp.Ordered](nodes []T, edges
    map[T]map[T]float64, opts Options) map[T]float64` plus
    `RankNodes` convenience wrapper. Options carry damping, epsilon,
    maxIter, personalization. All defaults match SPEC §18.2.
  - No DI of randomness. No goroutines. Pure function over inputs.

- **D-02: Generic constraint is `cmp.Ordered` (Go 1.21+).** Direct
  `sort.Slice` over node-id values. Works for `string` (repomap files)
  and `uint64` semantic NodeID. Future structured-id types will need
  a custom-Less variant; deferred.

- **D-03: Determinism via sort-before-iterate.** Every iteration
  rebuilds from a pre-sorted node slice. Map iteration order never
  influences output. Tiebreak between equal scores is the sorted-key
  order, which IS the stable-key NodeID per GRAPH-03.

- **D-04: Repomap migrates to consume the shared engine.**
  `internal/repomap/pagerank.go` is rewritten as a thin adapter that
  builds the file-keyed inputs and calls `internal/graph.PageRank`.
  `internal/repomap/pagerank_test.go` test vectors are re-pinned
  (one-time accept of new deterministic outputs). Behavior change is
  bounded to deterministic ordering — score values are the same; only
  ordering across equal scores becomes stable.

  **Hard invariants:**
  - `internal/repomap` keeps its `FileGraph` type and public API.
    Only the algorithm body moves.
  - The migration ships in the same wave as the new engine so repomap
    tests gate the determinism patch.
  - The re-pinned test vectors live alongside the algorithm so
    future contributors see the determinism contract immediately.

### `graph_version` advance policy

- **D-05: `graph_version` is a separate counter from `current_epoch`.**
  The `semantic_live_overlay_meta.graph_version UBIGINT NOT NULL`
  column already exists (Phase 57 migration v1, currently pinned at 0).
  Phase 62 wires the advance logic. Phase 60's `current_epoch` keeps
  its CAS role for compaction (Phase 63).

  **Hard invariants:**
  - `graph_version` MUST be monotonic non-decreasing per `repo_id`.
  - `graph_version` MUST be observable to readers under the same
    overlay-mutex serialization that protects `current_epoch`.
  - Phase 63 compaction must NOT touch `graph_version`; it advances
    only inside `ApplyRepair`.

- **D-06: `graph_version` bumps inside `ApplyRepair`.** The single
  call site that advances `graph_version` is the new
  `Engine.ApplyRepair(repair)` method per SPEC §17.3. No other code
  path bumps it. `BeginOverlayTx` does NOT bump it as a side effect of
  every commit.

  **Hard invariants:**
  - `ApplyRepair` is called by the post-commit handler when an overlay
    tx produces edge changes OR symbol stable-key/signature/
    exported/kind changes (SPEC §17.2 verbatim). Symbol-body-only
    edits do NOT trigger.
  - `ApplyRepair` runs under the per-workspace overlay mutex (same
    one Phase 60 D-04 acquires). The bump + score-row writes are
    serialized with `current_epoch` advances.
  - The decision of "was this a graph-changing tx" lives in the
    coalescer or post-commit hook (planner picks). The handler emits
    a typed `GraphRepair` value built from the diff; `ApplyRepair`
    is pure plumbing.

- **D-07: `score_status` is computed at read time, not write time.**
  A score row's `graph_version` is the version that produced it; the
  read path compares it to the current `graph_version` on
  `overlay_meta`. `exact` ↔ equal; `stale` ↔ score's gv < current;
  `missing` ↔ no row; `approximate` ↔ row carries an `approximate`
  marker (used by the >5000-frontier full-recompute fallback below).

  **Hard invariants:**
  - The four `score_status` values are a closed enum. No new values
    in v1.
  - `approximate` is set ONLY by the full-recompute scheduler when
    it short-circuits (e.g., starts but is preempted by a new
    `ApplyRepair` mid-run). The incremental local repair never
    emits `approximate`.

### PageRank scheduler

- **D-08: Idle-debounce + on-demand background scheduler.** Phase 62
  ships a `RankScheduler` goroutine (one per workspace) that:
  - Subscribes to `graph_version` advances via a typed channel.
  - On each advance, resets a debounce timer (default
    `pagerank.repair_debounce_ms=2000`).
  - When the timer fires with no further advance, runs an incremental
    local repair (D-09 below).
  - When the count of `score_status="stale"` rows exceeds
    `pagerank.full_recompute_threshold` (default 0.25 of node count)
    and a longer idle (`pagerank.full_recompute_idle_ms=60000`) has
    elapsed, schedules a full recompute.

  **Hard invariants:**
  - The scheduler MUST be one goroutine per workspace, never global.
    Cross-workspace serialization of PR is a bug surface to avoid.
  - The scheduler is best-effort: a daemon shutdown mid-repair must
    leave the score table in a consistent state — incremental writes
    happen inside a single overlay tx so rollback is atomic.
  - Read tools NEVER block on the scheduler. They return whatever
    `score_status` the rows currently carry; the read path
    optionally enqueues a "please repair soon" hint that resets the
    debounce.

- **D-09: 1-hop incremental repair frontier.** The frontier for one
  incremental repair is `changed_nodes ∪ {direct in/out neighbors over
  edges in the current graph}`. If `|frontier| > max_local_pagerank_
  nodes` (default 5000), the scheduler:
  1. Marks every score row for the projection `score_status="stale"`.
  2. Schedules a full recompute on the long-idle timer.
  3. Returns; no partial repair attempted.

  **Hard invariants:**
  - The frontier is computed deterministically from the current graph
    + the typed `GraphRepair` value. Same inputs → same frontier.
  - Local repair runs the same `internal/graph.PageRank` engine,
    restricted to the frontier subgraph + boundary teleport from
    surrounding ranks (SPEC §18.4). Output writes ONLY frontier
    rows; non-frontier rows untouched.
  - The 5000 threshold is a config value (`max_local_pagerank_nodes`)
    and is recorded at startup slog INFO so ops can audit overrides.

- **D-10: Full recompute writes a new `graph_version` generation of
  rows in one tx.** The full recompute reads the current
  `graph_version` at start, runs PR over the whole node set, writes
  the new score rows under that `graph_version`, and deletes prior
  rows for the projection in the same tx. If a new `ApplyRepair`
  bumps `graph_version` mid-run, the recompute marks the rows it
  wrote `score_status="approximate"` and the scheduler immediately
  starts a fresh incremental repair.

  **Hard invariants:**
  - One full recompute at a time per workspace. The scheduler
    serializes via a per-workspace mutex.
  - The transaction MUST advance the score rows from `stale → exact`
    (or `→ approximate` if pre-empted) atomically. Partial rows are
    not allowed.
  - The full recompute respects the same `pagerank.epsilon=1e-6`
    and `maxIter` defaults as the incremental path.

### Type resolver placement + scope

- **D-11: Per-language resolvers under `internal/semantic/types/<lang>/`
  + shared core in `internal/semantic/types/`.** Layout mirrors
  `internal/semantic/extract/`:
  ```
  internal/semantic/types/
    resolver.go        # Resolver interface + dispatch
    chain.go           # Access-chain walker (shared)
    fixpoint.go        # Bounded fixpoint loop (shared)
    ladder.go          # SPEC §38.2 7-tier confidence model
    emit.go            # Edge emission + two-phase comment merge
    golang/resolver.go
    python/resolver.go
    typescript/resolver.go
    php/stub.go        # zero-confidence stub
    ruby/stub.go       # zero-confidence stub
    java/stub.go       # short-circuit on LSP-confirmed facts
  ```

  **Hard invariants:**
  - The shared core has zero language-specific code. Language quirks
    live entirely under `<lang>/`.
  - Each `<lang>/resolver.go` exposes the same `Resolver` interface;
    dispatch is by `fact.Language` at the top-level `resolver.go`.
  - Stubs MUST NOT silently skip emission — they emit `unresolved`
    edges with `confidence=0.20` so consumers can see the gap.

- **D-12: v1 lang scope = Go + TypeScript/JavaScript + Python full
  ladders + PHP/Ruby/Java best-effort stubs.** The full ladder per
  language:

  | Confidence | Source                              | Languages with full coverage |
  |------------|-------------------------------------|------------------------------|
  | 1.00       | `lsp.<call>` (from Phase 61)        | All Phase 59 first-class     |
  | 0.90       | annotation (typed declaration)      | Go, TS, Python (PEP-484)     |
  | 0.80       | constructor return type             | Go, TS, Python               |
  | 0.70       | assignment-flow within fixpoint     | Go, TS, Python               |
  | 0.60       | doc-comment fallback                | Go (GoDoc), TS (TSDoc/JSDoc), Python (type comments) |
  | 0.45       | heuristic / name-shape              | Go, TS, Python               |
  | 0.20       | unknown / stub                      | All                          |

  **Hard invariants:**
  - Java stub short-circuits: if Phase 61 already emitted a
    `RESOLVES_TO` edge with `confidence=1.0` for a given reference,
    Java stub does NOT emit a competing 0.20 row. It only emits the
    0.20 edge for references that have NO LSP-derived edge.
  - PHP/Ruby stubs ALWAYS emit `confidence=0.20, validation_state=
    "unresolved"`. They exist purely so consumers can detect "no type
    info available" without missing rows.
  - Comment parsers are hand-rolled, small, per-language. No new
    third-party dependencies. The shared core has zero comment
    knowledge.

- **D-13: Fixpoint scope is per-package / compilation unit.** One
  fixpoint run loads all symbols/references/edges for one package and
  iterates within that scope:
  - **Go:** the directory containing the file (Go's package-per-dir).
  - **TypeScript:** the project anchored by the nearest `tsconfig.json`
    (or fallback to the file's directory if absent).
  - **Python:** the package anchored by `__init__.py`, walking
    upward from the file. Falls back to the file's directory.

  **Hard invariants:**
  - Cross-package chains DO NOT resolve in v1. They emit at the last
    in-package confidence level reached (typically 0.45–0.60) with
    `validation_state="unresolved"`. SPEC §38.5 cross-file fixpoint
    is in-scope BUT bounded by package.
  - The package-detection helpers live under `<lang>/scope.go`. They
    are pure functions over file paths — no I/O beyond `Stat`.
  - `max_chain_depth=8` and `max_fixpoint_iterations=8` are honored
    PER fixpoint run (per package). Both are config values.

- **D-14: Two-phase comment edge emission.** Comment-derived edges
  emit immediately at `confidence=0.60, source="comment.<kind>",
  validation_state="unresolved"`. When `ApplyRepair` later sees a
  Phase 61 LSP edge with the same `(src, dst, kind)`:
  1. Delete the 0.60 comment row.
  2. Insert the LSP edge with `confidence=1.0, source="lsp.<call>",
     validation_state="validated"` (or update in-place if the same row
     can be upgraded — planner picks based on storage cost).
  3. Bump `graph_version` per D-06.

  **Hard invariants:**
  - The merge predicate is `(src_node_id, dst_node_id, edge_kind)`.
    Different sources for the same triple are NOT both allowed in
    `semantic_live_overlay_edges` after merge.
  - LSP edges WIN every merge. A comment-derived 0.60 edge NEVER
    survives an LSP-confirmed write for the same triple.
  - The merge runs inside `ApplyRepair`, under the same mutex as
    `graph_version` advance, so readers never see a half-merged
    state.
  - If a comment edge is later refuted by LSP (i.e., LSP says the
    chain resolves to a DIFFERENT dst), the comment edge is deleted
    and the LSP edge takes its place — the comment edge does NOT
    survive at lower confidence.

### Acceptance criteria (must hold at end of phase)

1. `internal/graph/PageRank` produces byte-identical outputs across
   runs for the same inputs. Verified by a unit test asserting hex
   digest of sorted-key score-vector serialization.
2. `internal/repomap/pagerank.go` is a thin adapter over
   `internal/graph.PageRank`; `internal/repomap/pagerank_test.go`
   passes against re-pinned vectors.
3. Personalized PR with seed weighting (files + symbol names)
   returns a ranked node list under `pagerank.epsilon=1e-6`.
4. `semantic_live_overlay_meta.graph_version` advances exactly once
   per `ApplyRepair` call. Verified by an integration test that
   commits N overlay txs (some edge-changing, some not) and asserts
   `graph_version == count(edge-changing-txs)`.
5. `score_status` is one of `{exact, approximate, stale, missing}`
   in every read response. Verified by a contract test against the
   read API.
6. Incremental local repair runs only inside the
   `max_local_pagerank_nodes=5000` frontier. Verified by an
   integration test that injects a 6000-node-frontier change and
   asserts the scheduler short-circuits to all-stale + full
   recompute.
7. Weak-component pass over the effective graph identifies clusters
   deterministically. Verified by a unit test asserting hex digest
   of (cluster_id, member_node_ids) serialization.
8. `RESOLVES_TO`, `CALLS`, `USES_TYPE` edges emit with confidence
   per the SPEC §38.2 ladder for Go, TS/JS, Python. Per-language
   integration tests assert each tier emits at least once.
9. Access chains up to `max_chain_depth=8` resolve under a fixpoint
   loop bounded by `max_fixpoint_iterations=8`. Integration test
   uses a chain of length 9 and asserts the resolver emits
   `unresolved` on the last hop.
10. Comment-derived edges never exceed `confidence=0.60` unless
    independently confirmed by LSP. Integration test writes a TSDoc
    type comment, asserts 0.60 edge, then injects a matching Phase 61
    LSP edge and asserts in-place upgrade to 1.0.
11. Non-converged chains emit with low confidence + `unresolved`
    markers; NEVER `validated`. Verified by per-language tests.
12. PHP/Ruby stubs always emit `confidence=0.20, validation_state=
    "unresolved"` for type-resolution requests. Java stub
    short-circuits on LSP-confirmed facts (asserted in integration
    test).
13. GRAPH-01..06 + TYPES-01..04 all marked `Done` in
    `.planning/REQUIREMENTS.md` after Phase 62 close.

### Claude's Discretion (no user input needed)

- **Package layout for the rank engine:** the call site for semantic
  PR may live as `internal/semantic/graph/` or as a sub-package under
  `internal/semantic/store/`. The shared engine itself is locked at
  `internal/graph/`.
- **Goroutine-lifecycle hookup:** `RankScheduler` follows the
  Phase 60 coalescer / Phase 61 worker template (closed-channel
  start/stop, structured slog, errgroup ownership in `daemon.go`).
- **Metrics (closed-enum bounded labels):**
  - `helix_semantic_graph_pagerank_duration_seconds{projection,scope}` —
    histogram, `scope ∈ {incremental, full}`, `projection ∈ {call_graph}`
    (closed enum reserves space for future projections).
  - `helix_semantic_graph_score_status_total{projection,status}` —
    counter, `status ∈ {exact, approximate, stale, missing}`.
  - `helix_semantic_graph_repair_total{outcome}` — counter,
    `outcome ∈ {applied, frontier_overflow, preempted, error}`.
  - `helix_semantic_graph_version` — gauge per workspace.
  - `helix_semantic_types_resolution_total{language,confidence_tier}` —
    counter, `confidence_tier ∈ {1.00, 0.90, 0.80, 0.70, 0.60, 0.45, 0.20}`
    bucketed.
  All labels closed-enum, registered via the same `internal/obs/`
  bounded-label discipline Phase 60/61 used.
- **Trace spans:** `semantic.graph.apply_repair`,
  `semantic.graph.pagerank_full`, `semantic.graph.pagerank_incremental`,
  `semantic.graph.cluster_detect`, `semantic.types.resolve`. SPEC §28.2
  alignment.
- **Score-row write path:** scheduler opens an overlay tx via
  Phase 60's `BeginOverlayTx`, calls a new `tx.UpsertGraphScores(rows)`
  helper (added to `internal/semantic/store/overlay.go`), commits.
  `current_epoch` advances per Phase 60 D-04; `graph_version` does
  NOT advance on score-only commits.
- **`Ranker` and `Resolver` API for Phase 64:**
  ```go
  // internal/semantic/graph/ranker.go
  type Ranker interface {
      Rank(ctx, RankRequest) (RankResponse, error)        // GRAPH-02
      Status(ctx, repoID) (RankStatus, error)             // GRAPH-05
  }
  // internal/semantic/types/resolver.go
  type Resolver interface {
      ResolveChain(ctx, ChainRequest) (ChainResponse, error)
      ResolveSymbol(ctx, SymbolRequest) (SymbolResponse, error)
  }
  ```
  Phase 64's MCP tools wrap these. Both responses carry
  `graph_version`, `enrichment_level`, `score_status` per GRAPH-03.
- **Plan layout:** the planner decides wave structure. Suggested 5
  plans:
  P01 `internal/graph/` shared engine + repomap migration + re-pinned
      test vectors + determinism unit tests (GRAPH-01 byte-equal
      hex-digest assertion).
  P02 `graph_version` advance machinery: `ApplyRepair` plumbing,
      bump-on-edge-change rule, `score_status` read API, score-row
      write path through overlay tx (GRAPH-03, GRAPH-05).
  P03 `RankScheduler` + 1-hop frontier + full-recompute fallback +
      `WriteInvalidations` consumer (GRAPH-02, GRAPH-04).
  P04 Weak-component clustering + cluster persistence in existing
      tables (GRAPH-06).
  P05 Type resolver shared core + per-language resolvers (Go, TS,
      Python full + PHP/Ruby/Java stubs) + two-phase comment-edge
      emission + fixpoint (TYPES-01..04).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` §GRAPH (lines 58–63) — GRAPH-01 through
  GRAPH-06. Each maps directly to acceptance tests above.
- `.planning/REQUIREMENTS.md` §TYPES (lines 93–96) — TYPES-01 through
  TYPES-04.
- `.planning/milestones/v1.10-ROADMAP.md` Phase 62 block (lines
  131–141) — Goal, Depends on (Phase 60 + 61), Requirements, 5
  Success Criteria.

### Specification (source of truth for shapes and rules)

- `SPEC-DRAFT.md` §17 (lines 1624–1724) — Graph Cache and Repair.
  - §17.1 — `LiveGraphCache` shape.
  - §17.2 — Repair rules (the canonical "what to invalidate"
    catalog). D-06 implements verbatim.
  - §17.3 — `ApplyRepair` flow + `g.GraphVersion++` site. D-05/D-06
    implement.
- `SPEC-DRAFT.md` §18 (lines 1725–1882) — PageRank and Ranking.
  - §18.1 — projections list (Phase 62 ships ONLY `CALL_GRAPH`).
  - §18.2 — weighted PageRank algorithm. D-01 implements.
  - §18.3 — personalized PR. GRAPH-02.
  - §18.4 — incremental local repair. D-09 implements.
  - §18.5 — score fusion (deferred; single projection in v1).
- `SPEC-DRAFT.md` §19 (lines 1883–2050) — Clustering. Weak-component
  algorithm only in v1.
- `SPEC-DRAFT.md` §38 (lines 3887–4150) — Type Resolution and
  Access-Chain Resolver.
  - §38.1 — purpose.
  - §38.2 — 7-tier evidence ladder. D-12 table replicates.
  - §38.3 — access chain model.
  - §38.4 — fixpoint resolution algorithm.
  - §38.5 — fixpoint loop across file facts. D-13 bounds at package.
  - §38.6 — comment-based fallbacks (TSDoc/JSDoc, Python comments,
    PHPDoc/YARD). D-14 implements two-phase emission.
  - §38.7 — edge emission (`RESOLVES_TO`, `CALLS`, `USES_TYPE`).
- `SPEC-DRAFT.md` §9.9 (lines 752–767) — `semantic_graph_scores`
  schema.
- `SPEC-DRAFT.md` §9.10 (lines 773–798) — `semantic_clusters` /
  `semantic_cluster_members` schema.
- `SPEC-DRAFT.md` §12.1 (lines 1039–1065) — Core edge kinds
  vocabulary.
- `SPEC-DRAFT.md` §12.3 — edge confidence semantics.
- `SPEC-DRAFT.md` §28.1 — graph/types metric naming conventions.
- `SPEC-DRAFT.md` §28.2 — trace span naming.

### Phase 60 lock-down (must not regress)

- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md`:
  - D-04 `current_epoch` per-tx CAS contract — Phase 62 does NOT
    bump this; `graph_version` is separate.
  - `BeginOverlayTx` API — Phase 62 score writes use it.
- `internal/semantic/store/overlay.go:99-130` — current_epoch +
  graph_version columns on `semantic_live_overlay_meta`.

### Phase 61 lock-down (must not regress)

- `.planning/phases/61-lsp-enrichment-worker/61-CONTEXT.md`:
  - D-07 (full §14.4 cascade) — Phase 61 emits validated LSP edges
    at `confidence=1.0`, `source="lsp.<call>"`. Phase 62's two-phase
    merge consumes these.
  - "WriteInvalidations is a typed no-op stub Phase 62 fills" —
    Phase 62 owns the consumer.
  - Java/Rust readiness gates — Phase 62 does NOT touch these
    (rank engine doesn't issue LSP calls).
- `internal/semantic/lspenrich/` — Phase 61 worker. Phase 62 reads
  the `validated` edges it emits.

### Phase 59 lock-down (must not regress)

- `.planning/phases/59-tree-sitter-extraction-stable-symbol-ids/`
  CONTEXT — D-01 (no `internal/repomap` import inside
  `internal/semantic/extract/`); Phase 62 inherits the same
  invariant for `internal/semantic/types/`.
- `internal/semantic/types.go` (existing typed identifiers) — Phase
  62 may extend (`GraphVersion`, `ScoreStatus`, `ConfidenceTier`).

### Schema + storage

- `internal/semantic/store/migrations.go:248-289` — `semantic_graph_
  scores` and `semantic_clusters` / `semantic_cluster_members`
  tables. ALREADY shipped; Phase 62 fills them. NO schema change
  required for v1.
- `internal/semantic/store/migrations.go:291-310` —
  `semantic_live_overlay_meta` (carries `graph_version` column,
  currently pinned at 0).
- `internal/semantic/store/overlay.go` — `BeginOverlayTx` API.
  Phase 62 extends with `tx.UpsertGraphScores`, `tx.UpsertClusters`,
  `tx.UpsertClusterMembers`, `tx.UpsertEdgesWithMerge` (the last for
  D-14 two-phase comment merge).
- `internal/semantic/store/effective.go` — snapshot ⊕ overlay −
  tombstones read API. Phase 62 reads through this for the rank +
  cluster computation inputs.
- `internal/semantic/store/duckdb.go` — fact-store open path. Phase
  62 does not change open semantics.

### Pitfall catalog

- `.planning/research/PITFALLS.md` — review for any rank/type-related
  death-spiral patterns. The C8 prescription (no flood after
  bulk_update) is Phase 61's job; Phase 62 inherits it via the
  scheduler debounce (D-08).

### Architectural invariants

- `CLAUDE.md` "Middleware Execution Order (LIFO)" — Phase 62 does
  NOT touch middleware; rank/types live below the MCP layer.
- `internal/lint/nokernel2semantic/` — `internal/semantic/...` MUST
  NOT import `internal/kernel/...`. Phase 62 inherits.
- `cmd/vet-noduckdb/` — `internal/semantic/types/...` and
  `internal/semantic/graph/...` MUST NOT import `duckdb-go`
  directly. They go through `internal/semantic/store/`.
- `internal/treesitter/registry_cgo.go` — Phase 62 does NOT use
  tree-sitter directly; type resolvers read facts already extracted
  by Phase 59.

### Pattern templates (must mirror)

- `internal/repomap/pagerank.go` — current implementation. Phase 62
  rewrites this as an adapter over `internal/graph.PageRank` and
  re-pins test vectors. Same public API for `FileGraph`.
- `internal/semantic/extract/` per-language layout — Phase 62
  mirrors with `internal/semantic/types/<lang>/`.
- `internal/semantic/live/coalescer/` — closed-channel goroutine
  lifecycle, debounce timer, structured slog. `RankScheduler`
  follows.
- `internal/semantic/lspenrich/` — Phase 61 worker bootstrap +
  errgroup ownership. Phase 62's `RankScheduler` follows.
- `internal/daemon/daemon.go` setter pattern (`SetEnrichFn`,
  `SetActivateCallback`, `SetEditNotifier`,
  `SetEnrichmentWorker`) — Phase 62 adds
  `SetRankScheduler` / `SetTypeResolver` (or the planner picks a
  bundle).
- `internal/kernel/lspool/circuit.go` — circuit-breaker pattern.
  Phase 62 ignores (no LSP calls from the rank/types engines).

### Configuration

- `internal/config/defaults.go` — Phase 62 adds (under existing
  `semantic_index.*` block):
  - `pagerank.damping: 0.85`
  - `pagerank.epsilon: 1e-6`
  - `pagerank.max_iter: 100`
  - `pagerank.repair_debounce_ms: 2000`
  - `pagerank.full_recompute_idle_ms: 60000`
  - `pagerank.full_recompute_threshold: 0.25`
  - `pagerank.max_local_pagerank_nodes: 5000`
  - `types.max_chain_depth: 8`
  - `types.max_fixpoint_iterations: 8`
  - `types.comment_parsers_enabled: [tsdoc, jsdoc, godoc,
    python_type_comments, phpdoc, yard]`
- `SerenaConfig.SemanticIndex` extends with `Pagerank` and `Types`
  sub-structs.
- Per-feature defaults test (`TestLoad_PagerankDefaults`,
  `TestLoad_TypesDefaults`) mirrors Phase 60/61 pattern.

### Test fixtures (must seed)

- New: `internal/graph/testdata/pagerank/` — fixed-input PR test
  vectors (golden hex digests for GRAPH-01 byte-equal assertion).
- New: `internal/semantic/types/testdata/<lang>/` — per-language
  fixture trees with chains hitting every confidence tier.
- New: `internal/semantic/graph/testdata/repair/` — overlay tx
  fixtures producing predictable `GraphRepair` shapes.
- Reuse: `internal/repomap/testdata/` — repomap PR vectors must
  pass against the migrated engine (re-pinned).
- Reuse: Phase 59 + Phase 61 fixtures for end-to-end resolver tests
  (Go, Java, Python).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/repomap/pagerank.go`** — current ~150 LOC PageRank
  implementation. Phase 62 lifts the algorithm (with a determinism
  patch) into `internal/graph/` and rewrites this file as a thin
  adapter. Test vectors re-pin once.
- **`internal/repomap/graph.go`** — `FileGraph` shape, edge weight
  semantics. Phase 62 keeps this; only the algorithm body moves.
- **`internal/semantic/store/migrations.go:248-289`** —
  `semantic_graph_scores` and `semantic_clusters` /
  `semantic_cluster_members` tables already shipped in v1
  migration. Phase 62 fills them. NO schema change.
- **`internal/semantic/store/overlay.go`** — Phase 60's
  `BeginOverlayTx` API. Phase 62 extends with `UpsertGraphScores`,
  `UpsertClusters`, `UpsertClusterMembers`, `UpsertEdgesWithMerge`.
- **`internal/semantic/store/effective.go`** — snapshot ⊕ overlay −
  tombstones read API. Phase 62 reads PR/cluster inputs through
  this.
- **`internal/semantic/lspenrich/`** — Phase 61 worker that emits
  validated LSP edges. Phase 62's two-phase comment merge consumes
  these.
- **`internal/semantic/extract/golang|python|typescript/`** — Phase
  59 extractors. Phase 62's per-language type resolvers mirror this
  layout under `internal/semantic/types/`.
- **`internal/semantic/live/coalescer/`** — closed-channel goroutine
  lifecycle, debounce timer. `RankScheduler` mirrors.
- **`internal/config/defaults.go`** — koanf precedence chain. Phase
  62 adds two new config sub-structs in the existing semantic_index
  block.
- **`internal/obs/`** — bounded-label metric registration. All Phase
  62 metrics route through here.

### Established Patterns

- **Generic narrow interfaces** between `internal/semantic` and
  callers. `Ranker` and `Resolver` in Phase 62 are the templates for
  Phase 64's MCP tool consumption.
- **Setter-style cross-package wiring** (`SetEnrichFn`,
  `SetActivateCallback`, `SetEditNotifier`,
  `SetEnrichmentWorker`). Phase 62 adds `SetRankScheduler` /
  `SetTypeResolver` (or planner-bundled).
- **Bounded-label metrics with closed enum.** Every Phase 62 metric
  has a documented closed-enum label set; no string interpolation
  into label values.
- **Per-feature defaults test.** Phase 62 adds
  `TestLoad_PagerankDefaults` and `TestLoad_TypesDefaults` for the
  new keys.
- **Per-workspace mutexes** for overlay writes (Phase 60 D-04).
  `RankScheduler` mutex co-aligns: bump + score-write happens under
  the same mutex.
- **Nil-safe queue/worker fields** (Phase 60 CR-04). Phase 62
  bootstrap path stays nil-safe.

### Integration Points

- **Daemon bootstrap (`internal/daemon/daemon.go`)** — after Phase
  61's enrichment worker, Phase 62 adds:
  - Construct `RankScheduler` (one per workspace, or workspace-keyed
    map — planner picks).
  - Construct `Resolver` (stateless or workspace-keyed).
  - Wire them via setter into Phase 64-eligible consumers.
  - Start scheduler goroutines in the existing errgroup.
- **`SerenaConfig.SemanticIndex`** — Phase 62 populates two new
  sub-structs (`Pagerank`, `Types`) via
  `internal/config/defaults.go` and per-feature defaults tests.
- **`internal/semantic/live/handler/handler.go`** — post-overlay-
  commit hook fires `ApplyRepair` when the diff carries edge
  changes or symbol stable-key/signature/exported/kind changes. The
  handler stays the single decision point for "is this a graph-
  changing tx" (mirrors Phase 61 D-05's lane-decision pattern).
- **Phase 61 `WriteInvalidations`** — Phase 62 fills the consumer
  that turns the typed invalidation rows into a `GraphRepair` value
  for `ApplyRepair`. Phase 61 keeps writing through the typed
  no-op-stub seam; Phase 62 swaps the stub for the consumer.

### Constraints

- Phase 62 must NOT import `internal/kernel` from
  `internal/semantic/...` — only `internal/kernel/lspool` types
  and `internal/workspace`. Enforced by
  `internal/lint/nokernel2semantic`.
- Phase 62 must NOT import `duckdb-go` outside
  `internal/semantic/store/` — `cmd/vet-noduckdb/` analyzer
  enforces. The `internal/graph/` shared engine has zero project
  imports.
- Phase 62 must NOT regress Phase 60 D-04 (`current_epoch` per-tx
  CAS contract). `graph_version` is a separate counter; both
  advance under the same per-workspace mutex.
- Phase 62 must NOT touch middleware (CLAUDE.md "Middleware
  Execution Order (LIFO)").
- Phase 62 must NOT introduce a second `GrammarRegistry` (BUG-04 /
  EXTRACT-05) — type resolvers read facts already extracted by
  Phase 59.
- Phase 62's score writes MUST advance `current_epoch` correctly
  (Phase 60 D-04 contract). Phase 63 compaction CAS depends on
  this.
- Phase 62's score writes MUST NOT advance `graph_version`
  themselves — only `ApplyRepair` bumps the rank version.
- Phase 62's `RankScheduler` is per-workspace; cross-workspace
  coordination is OUT.
- Phase 62 ships ZERO MCP tools. `index_semantic_graph`,
  `refresh_semantic_graph`, `get_semantic_graph_status`, and
  `get_semantic_context` are Phase 64's job.
- Phase 62 ships ZERO cluster MCP tools (`get_cluster_map`,
  `explain_cluster`) — deferred to v1.10.x. Algorithm only.
- Phase 62 must keep PHP/Ruby type-resolution stubs nonzero — they
  emit `confidence=0.20, validation_state="unresolved"` so
  consumers can detect "no type info" without missing rows.
- Comment-derived edges MUST never exceed `confidence=0.60` unless
  upgraded in-place by an LSP-confirmed write (TYPES-03 invariant).
- Non-converged chains MUST NEVER be emitted as `validated`
  (TYPES-04 invariant).

</code_context>

<specifics>
## Specific Ideas

- **The user wants a single shared PageRank engine, even if it
  costs a one-time test-vector re-pin in repomap.** The framing:
  one algorithm to audit, one locus for the determinism patch, no
  duplicate-implementation drift. Repomap's existing
  `pagerank_test.go` is treated as informational, not a hard
  byte-equal contract.
- **Determinism is the GRAPH-01 hard gate.** Sort-before-iterate +
  stable-key tiebreak. Hex-digest assertion in tests is the
  canonical proof.
- **`graph_version` semantics matter more than `current_epoch`
  semantics for Phase 63.** The user's separation choice (D-05)
  reflects: `graph_version` is "would scores change", `current_
  epoch` is "any write happened". Phase 63 compaction CAS is owned
  by `current_epoch`. Phase 62 score-row keys are owned by
  `graph_version`.
- **Idle-debounce scheduler with on-demand-stale-return is the
  no-block contract.** Read tools NEVER block on rank repair. The
  scheduler is best-effort; readers see whatever `score_status`
  the row carries today and can choose to wait or proceed.
- **1-hop frontier with all-stale fallback above 5000.** The user
  rejected k-hop and convergence-distance for v1: predictable
  budget over rank accuracy mid-edit. Stale rows during heavy
  bursts are explicitly accepted; readers see `stale` and can
  decide.
- **All Phase 59 first-class langs get the full ladder + best-
  effort stubs for PHP/Ruby/Java.** The user's most-ambitious-lang-
  scope choice. Java's stub short-circuits on Phase 61 LSP-
  confirmed facts; PHP/Ruby ALWAYS emit 0.20 unresolved so
  consumers can detect the gap.
- **Per-package fixpoint scope.** The user picked the middle path:
  not per-file (too narrow for cross-file chains), not per-repo
  (too expensive for live edits). Package boundary aligns with how
  LSPs naturally see scope.
- **Two-phase comment-edge emission with in-place LSP upgrade.**
  The user wants comments to fill gaps NOW (not wait for LSP) but
  upgrade silently when LSP catches up. Merge predicate is
  `(src_node_id, dst_node_id, edge_kind)`. LSP always wins. Comment
  edges that get refuted (LSP says different dst) are deleted —
  they don't survive at lower confidence.

</specifics>

<deferred>
## Deferred Ideas

- **Multi-projection PageRank** — only `CALL_GRAPH` ships in v1.
  The other six SPEC §18.1 projections (REFERENCE_PAGERANK,
  FILE_DEPENDENCY_PAGERANK, TYPE_HIERARCHY_PAGERANK,
  CHANGE_IMPACT_PAGERANK, SECURITY_SURFACE_PAGERANK,
  RETRIEVAL_CONTEXT_PAGERANK) are deferred. Engine generic enough
  to drop them in.
- **Score fusion across projections (SPEC §18.5)** — single
  projection, no fusion in v1.
- **Cluster MCP tools** (`get_cluster_map`, `explain_cluster`) —
  v1.10.x. Algorithm only.
- **Cluster labeling / summary generation** — out per the v1
  roadmap.
- **Cluster algorithms beyond weak-components** — label propagation
  refinement, deterministic split policy. Out of scope.
- **k-hop / convergence-distance frontier policy for incremental
  repair** — 1-hop is the v1 choice. Revisit if benchmarks show
  rank drift mid-edit hurts result quality.
- **Hard-cancel preemption of in-flight full recompute** — v1 marks
  pre-empted runs `approximate` and lets the next incremental
  finish; doesn't kill the goroutine. Revisit if long full
  recomputes block too many edits.
- **Cross-package fixpoint resolution** — per-package boundary in
  v1. Cross-package chains emit at last-in-package confidence
  (typically 0.45–0.60) with `validation_state="unresolved"`.
- **Full PHP/Ruby type resolution** — stub-only in v1. PHPDoc/YARD
  parsers ship with the framework but only Go/TS/Python feed them
  in v1.
- **Adaptive priority promotion for `semantic_pending` files
  based on rank** — Phase 65/66+. Phase 62 emits the rank that
  feeds the future heuristic.
- **Receipts / guardrails on rank-derived edges** — Phase 66.
- **MCP tool surface** (`index_semantic_graph`, `refresh_semantic_
  graph`, `get_semantic_graph_status`, `get_semantic_context`) —
  Phase 64. Phase 62 ships the underlying `Ranker` + `Resolver`.
- **Cross-repo / multi-workspace ranking** — workspaces are
  independent. Phase 60 cascade applies; no cross-workspace shared
  scheduler.
- **Custom-Less variant of the generic engine** — `cmp.Ordered`
  covers v1. If a future structured-id type needs custom ordering,
  add an overload then.

</deferred>

---

*Phase: 62-graph-engine-ranking-type-resolution*
*Context gathered: 2026-05-06*
