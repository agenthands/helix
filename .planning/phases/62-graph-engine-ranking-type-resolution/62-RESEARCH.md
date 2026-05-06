# Phase 62: Graph Engine, Ranking & Type Resolution - Research

**Researched:** 2026-05-06
**Domain:** Deterministic PageRank engine + `graph_version` advance + incremental repair scheduler + weak-component clustering + tiered type resolver (per-language) + two-phase comment-edge merge
**Confidence:** HIGH

## Summary

Phase 62 wires three sub-engines (rank, cluster, types) on top of foundations
already shipped: the `semantic_graph_scores` / `semantic_clusters` /
`semantic_cluster_members` tables (Phase 57 migration v1), the
`semantic_live_overlay_meta.graph_version` column (already pinned at 0), the
`OverlayTx` per-workspace mutex contract (Phase 60 D-04), the validated LSP
edges emitted at confidence=1.0 (Phase 61 D-07), and the typed
`tx.WriteInvalidations` no-op stub Phase 61 deliberately left for Phase 62 to
fill (`internal/semantic/lspenrich/cascade.go:441`).

The work is overwhelmingly *plumbing and adapter* — every load-bearing data
shape exists; what's missing is the deterministic algorithm extraction
(`internal/graph/`), the advance/read API for `graph_version` and
`score_status`, the per-workspace `RankScheduler` goroutine that consumes
invalidations and writes score rows, the weak-component pass, the per-language
type resolver dispatch under `internal/semantic/types/<lang>/`, and the
two-phase comment-edge merge predicate `(src_node_id, dst_node_id, edge_kind)`.

CONTEXT.md D-01..D-14 are exhaustive and locked. The 5-plan layout
(P01 engine + repomap migration; P02 graph_version + ApplyRepair + read API;
P03 RankScheduler + frontier + invalidation consumer; P04 weak-component
clustering; P05 type resolver core + per-language) maps cleanly to the
12-criterion acceptance contract.

**Primary recommendation:** Adopt CONTEXT.md's 5-plan layout verbatim. P01 ships
the determinism patch and re-pins repomap vectors atomically (single-wave gate).
P02 introduces the typed `GraphRepair` value and the `ApplyRepair` mutex
discipline that P03 then drives. P05 is the largest plan by LOC but the lowest
risk because the language quirks live entirely under `<lang>/` files behind a
shared `Resolver` interface — TDD it.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**PageRank engine:**
- D-01: Shared generic PageRank engine at `internal/graph/`. Public surface
  `func PageRank[T cmp.Ordered](nodes []T, edges map[T]map[T]float64,
  opts Options) map[T]float64` + `RankNodes` convenience wrapper. Imports:
  `math`, `sort`, `cmp` ONLY — no project imports. No DI of randomness. No
  goroutines. Pure function over inputs. Defaults match SPEC §18.2.
- D-02: Generic constraint is `cmp.Ordered` (Go 1.21+). Direct `sort.Slice`
  over node-id values. Works for `string` (repomap files) and `uint64`
  semantic NodeID. Future structured-id types deferred.
- D-03: Determinism via sort-before-iterate. Every iteration rebuilds from a
  pre-sorted node slice. Map iteration order never influences output.
  Tiebreak between equal scores = sorted-key order, which IS the stable-key
  NodeID per GRAPH-03.
- D-04: Repomap migrates to consume the shared engine. `internal/repomap/
  pagerank.go` rewritten as thin adapter; `internal/repomap/pagerank_test.go`
  vectors re-pinned (one-time accept of new deterministic outputs). Behavior
  change bounded to deterministic ordering — score *values* unchanged. Same
  wave as engine; repomap tests gate the determinism patch.

**`graph_version` advance policy:**
- D-05: `graph_version` is a SEPARATE counter from `current_epoch`. Column
  already exists (Phase 57 migration v1, pinned at 0). Phase 62 wires
  advance logic; Phase 60's `current_epoch` keeps its CAS role for compaction
  (Phase 63). MUST be monotonic non-decreasing per `repo_id`. MUST be
  observable to readers under the same overlay-mutex serialization that
  protects `current_epoch`. Phase 63 compaction MUST NOT touch
  `graph_version`.
- D-06: `graph_version` bumps inside `ApplyRepair`. SINGLE call site.
  `BeginOverlayTx` does NOT bump as side effect of every commit. Trigger
  set: edge add/remove/change OR symbol stable_key/signature/exported/kind
  changes (SPEC §17.2 verbatim). Symbol-body-only edits do NOT trigger.
  `ApplyRepair` runs under the per-workspace overlay mutex. The "is this a
  graph-changing tx" decision lives in coalescer or post-commit hook
  (planner picks); handler emits typed `GraphRepair`; `ApplyRepair` is pure
  plumbing.
- D-07: `score_status` computed at READ time, not write time. Score row's
  `graph_version` column is the version that produced it; read path
  compares to current `graph_version` on `overlay_meta`.
  `exact` ↔ equal; `stale` ↔ score_gv < current; `missing` ↔ no row;
  `approximate` ↔ row carries an explicit `approximate` marker. Closed enum
  — no new values in v1. `approximate` is set ONLY by the full-recompute
  scheduler when preempted; incremental local repair never emits
  `approximate`.

**PageRank scheduler:**
- D-08: Idle-debounce + on-demand background `RankScheduler`, ONE per
  workspace, never global. Subscribes to `graph_version` advances via typed
  channel. Each advance resets debounce timer (default
  `pagerank.repair_debounce_ms=2000`). Timer fires with no further advance
  → incremental local repair. When `score_status="stale"` rows exceed
  `pagerank.full_recompute_threshold` (default 0.25 of node count) AND
  `pagerank.full_recompute_idle_ms=60000` elapses → full recompute. Best-
  effort: shutdown mid-repair leaves consistent state (single overlay tx,
  atomic rollback). Read tools NEVER block on the scheduler; reads return
  whatever `score_status` rows currently carry.
- D-09: 1-hop incremental repair frontier. `frontier = changed_nodes ∪
  {direct in/out neighbors over edges in current graph}`. If `|frontier| >
  max_local_pagerank_nodes` (default 5000): mark every score row for the
  projection `score_status="stale"`, schedule full recompute on long-idle
  timer, return — no partial repair. Same `internal/graph.PageRank` engine,
  restricted to frontier subgraph + boundary teleport (SPEC §18.4). Write
  ONLY frontier rows. 5000 threshold logged at startup slog INFO so ops can
  audit overrides.
- D-10: Full recompute writes new `graph_version` generation in ONE tx.
  Reads current `graph_version` at start, runs PR over whole node set,
  writes new score rows under that `graph_version`, deletes prior rows for
  the projection in same tx. If a new `ApplyRepair` advances mid-run, the
  recompute marks its writes `score_status="approximate"` and scheduler
  immediately starts fresh incremental repair. ONE full recompute at a time
  per workspace (per-workspace mutex). Atomic stale → exact (or →
  approximate). Same `pagerank.epsilon=1e-6` and `maxIter` defaults.

**Type resolver placement + scope:**
- D-11: Layout under `internal/semantic/types/`:
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
  Shared core has zero language-specific code. Each `<lang>/resolver.go`
  exposes the same `Resolver` interface; dispatch by `fact.Language` at
  top-level. Stubs MUST NOT silently skip — they emit `confidence=0.20`
  unresolved edges.
- D-12: v1 lang scope = Go + TS/JS + Python full ladders + PHP/Ruby/Java
  best-effort stubs.
  | Confidence | Source                              | Languages w/ full coverage |
  |------------|-------------------------------------|----------------------------|
  | 1.00       | `lsp.<call>` (from Phase 61)        | All Phase 59 first-class   |
  | 0.90       | annotation (typed declaration)      | Go, TS, Python (PEP-484)   |
  | 0.80       | constructor return type             | Go, TS, Python             |
  | 0.70       | assignment-flow within fixpoint     | Go, TS, Python             |
  | 0.60       | doc-comment fallback                | Go (GoDoc), TS (TSDoc/JSDoc), Python (type comments) |
  | 0.45       | heuristic / name-shape              | Go, TS, Python             |
  | 0.20       | unknown / stub                      | All                        |
  Java stub short-circuits: if Phase 61 already emitted `RESOLVES_TO` at
  confidence=1.0 for a reference, Java does NOT emit a competing 0.20.
  PHP/Ruby ALWAYS emit 0.20 unresolved. Comment parsers hand-rolled,
  per-language, NO new third-party deps.
- D-13: Fixpoint scope is per-package / compilation unit.
  - **Go:** dir containing the file (Go's package-per-dir).
  - **TypeScript:** project anchored by nearest `tsconfig.json` (fallback to
    file dir).
  - **Python:** package anchored by `__init__.py`, walking upward (fallback
    to file dir).
  Cross-package chains DO NOT resolve in v1 — emit at last-in-package
  confidence with `validation_state="unresolved"`. Package-detection helpers
  under `<lang>/scope.go` — pure functions over file paths, no I/O beyond
  `Stat`. `max_chain_depth=8` and `max_fixpoint_iterations=8` per fixpoint
  run.
- D-14: Two-phase comment edge emission. Comment-derived edges emit
  immediately at `confidence=0.60, source="comment.<kind>",
  validation_state="unresolved"`. When `ApplyRepair` later sees a Phase 61
  LSP edge with same `(src, dst, kind)`:
  1. Delete the 0.60 comment row.
  2. Insert LSP edge at `confidence=1.0, source="lsp.<call>",
     validation_state="validated"`.
  3. Bump `graph_version`.
  Merge predicate is `(src_node_id, dst_node_id, edge_kind)`. LSP wins
  every merge. Refuted comment edges (LSP says different dst) are deleted
  — they do NOT survive at lower confidence. Merge runs inside
  `ApplyRepair` under same mutex.

### Claude's Discretion

- Package layout for the rank engine call site: `internal/semantic/graph/`
  vs sub-package under `internal/semantic/store/`. Engine itself locked at
  `internal/graph/`.
- Goroutine-lifecycle hookup: `RankScheduler` follows Phase 60 coalescer /
  Phase 61 worker template (closed-channel start/stop, structured slog,
  errgroup ownership in `daemon.go`).
- Metrics (closed-enum bounded labels):
  - `helix_semantic_graph_pagerank_duration_seconds{projection,scope}` —
    histogram, `scope ∈ {incremental, full}`, `projection ∈ {call_graph}`.
  - `helix_semantic_graph_score_status_total{projection,status}` — counter,
    `status ∈ {exact, approximate, stale, missing}`.
  - `helix_semantic_graph_repair_total{outcome}` — counter,
    `outcome ∈ {applied, frontier_overflow, preempted, error}`.
  - `helix_semantic_graph_version` — gauge per workspace.
  - `helix_semantic_types_resolution_total{language,confidence_tier}` —
    counter, `confidence_tier ∈ {1.00, 0.90, 0.80, 0.70, 0.60, 0.45, 0.20}`.
  All labels closed-enum, registered via `internal/obs/`.
- Trace spans: `semantic.graph.apply_repair`, `semantic.graph.pagerank_full`,
  `semantic.graph.pagerank_incremental`, `semantic.graph.cluster_detect`,
  `semantic.types.resolve`. SPEC §28.2 alignment.
- Score-row write path: scheduler opens overlay tx via `BeginOverlayTx`,
  calls new `tx.UpsertGraphScores(rows)`, commits. `current_epoch` advances
  per Phase 60 D-04; `graph_version` does NOT advance on score-only
  commits.
- `Ranker` and `Resolver` API for Phase 64:
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
  Both responses carry `graph_version`, `enrichment_level`, `score_status`
  per GRAPH-03.
- Plan layout (5 plans suggested): P01 engine + repomap migration; P02
  graph_version + ApplyRepair + score_status read API; P03 RankScheduler +
  1-hop frontier + WriteInvalidations consumer; P04 weak-component
  clustering; P05 type resolver core + per-language + two-phase merge.

### Deferred Ideas (OUT OF SCOPE)

- Multi-projection PageRank — only `CALL_GRAPH` ships in v1. Six other SPEC
  §18.1 projections deferred. Engine stays generic.
- Score fusion across projections (SPEC §18.5).
- Cluster MCP tools (`get_cluster_map`, `explain_cluster`) — v1.10.x.
- Cluster labeling / summary generation.
- Cluster algorithms beyond weak-components.
- k-hop / convergence-distance frontier policy (1-hop is v1 choice).
- Hard-cancel preemption of in-flight full recompute (mark approximate +
  next incremental finish; don't kill goroutine).
- Cross-package fixpoint resolution (per-package boundary in v1).
- Full PHP/Ruby type resolution (stub-only in v1).
- Adaptive priority promotion of `semantic_pending` files based on rank
  (Phase 65/66+).
- Receipts / guardrails on rank-derived edges (Phase 66).
- MCP tool surface (`index_semantic_graph`, `refresh_semantic_graph`,
  `get_semantic_graph_status`, `get_semantic_context`) — Phase 64.
- Cross-repo / multi-workspace ranking — workspaces are independent.
- Custom-Less variant of generic engine — `cmp.Ordered` covers v1.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| GRAPH-01 | Weighted PR (CALL_GRAPH) deterministic; same input → byte-identical scores. | D-01..D-04 patch. Hex-digest assertion (sorted-key score-vector serialization → sha256). Map-iteration audit pinpointed at `internal/repomap/pagerank.go:55` (uniform teleport loop), `:62-67` (re-normalize), `:75-78` (rank init), `:104` (newRank init), `:108-118` (link component) — all rely on map ordering. Sort `files` once (already done at `:28-31`), then iterate THAT slice everywhere. Repomap test re-pin under D-04. |
| GRAPH-02 | Personalized PR with seed weighting (files + symbol names) under `pagerank.epsilon=1e-6`. | Existing `personalization map[T]float64` carries through to the generic engine. `RankNodes` convenience wrapper exposes ranked node list. SPEC §18.3. |
| GRAPH-03 | Each ranked tool response carries `graph_version` + `enrichment_level`; tiebreaks by stable-key NodeID order. | D-03 sort-before-iterate IS the tiebreak (sorted-key order = stable-key NodeID). `Ranker.Rank` response includes `GraphVersion` + `EnrichmentLevel` + `ScoreStatus`. Phase 64 wraps. |
| GRAPH-04 | Incremental local PR repair runs only inside `max_local_pagerank_nodes` frontier (default 5000); above → mark stale + schedule full recompute. | D-09 frontier algorithm. `RankScheduler` short-circuits at threshold, marks all score rows stale, schedules full recompute on long-idle timer. Metric: `helix_semantic_graph_repair_total{outcome="frontier_overflow"}`. |
| GRAPH-05 | Score persistence carries `status` ∈ {exact, approximate, stale, missing}; MCP responses surface as `score_status`. | D-07 read-time computation. Closed enum. `semantic_graph_scores.status TEXT NOT NULL` column already exists (migration v1 line 257). `Ranker.Status` returns `RankStatus` carrying current `graph_version` + per-projection `score_status`. |
| GRAPH-06 | Weak-component pass deterministic; algorithm shipped, cluster MCP tools deferred. | SPEC §19.2 BFS over weak components. Determinism via sorted-node-id keys for union-find / BFS queue. Hex-digest assertion: (cluster_id, member_node_ids) serialized after sort. Persistence: `semantic_clusters` (PK repo_id, graph_version, cluster_id) + `semantic_cluster_members` (PK repo_id, graph_version, cluster_id, node_id) — both already shipped. |
| TYPES-01 | Tiered RESOLVES_TO/CALLS/USES_TYPE edges with confidence per SPEC §38.2 ladder. | D-12 ladder table. SPEC §38.2 `TypeEvidenceKind` enum. Edge emission per SPEC §38.7. Per-language emission only for tiers the language supports; PHP/Ruby always 0.20; Java short-circuits when Phase 61 LSP edge already exists at 1.0. |
| TYPES-02 | Access-chain `a.b.c.d()` up to `max_chain_depth=8`; fixpoint up to `max_fixpoint_iterations=8`; early exit on no progress. | D-13 per-package scope. SPEC §38.4 `ResolveAccessChain` (already specifies depth check at top — return `confidence=0.20, reason="chain too deep"`). SPEC §38.5 fixpoint loop with `state.Hash()` for change detection. Both bounds are config keys (already shipped). |
| TYPES-03 | Comment-based fallbacks (JSDoc/TSDoc/PHPDoc/YARD/Python type comments) ≤ 0.60 unless LSP-confirmed. | D-14 two-phase emission. Hand-rolled per-language parsers under `<lang>/comment.go` — no new third-party deps. SPEC §38.6 examples (`@param {UserRepository} repo`, `# type: UserRepository`). LSP wins merge predicate `(src, dst, kind)` deletes the 0.60 row, inserts 1.0 row. |
| TYPES-04 | Non-converged chains emit with low confidence + `unresolved` markers; NEVER `validated`. | D-12 stub policy. SPEC §38.7 `EmitTypeResolutionEdges` sets `validation = ValidationValidated` ONLY when evidence carries `EvidenceLSPDefinition` or `EvidenceLSPHover`; otherwise `ValidationSkipped`. Phase 62 maps `ValidationSkipped` → `validation_state="unresolved"` for the closed-enum write. |

## Project Constraints (from CLAUDE.md)

These directives have the same authority as locked decisions and constrain all
plans:

- **Always run `go vet ./...` and `go test ./...`** before completing any Go
  task (CLAUDE.md "Go Development Commands"). Each plan's verification gate
  must include both.
- **GSD workflow enforcement.** Before Edit/Write, planner must route work
  through a GSD command. Phase 62 plans assume `/gsd:execute-phase` driver.
- **SMTC-first tool routing.** For semantic Q's during implementation, prefer
  `mcp__smtc__*` over grep / Read. Helix is a Go (first-class SMTC) codebase
  — first-class semantic accuracy.
- **Build pipeline (Phase 59.1).** Source tree is single-mode `CGO_ENABLED=1`.
  Phase 62 stays compatible. The platform-conditional `internal/semantic/store/
  duckdb.go` `!(windows && arm64)` constraint (D-14) does NOT affect Phase 62
  source files because Phase 62 doesn't import duckdb-go directly.
- **Architectural invariants:**
  - `internal/semantic/...` MUST NOT import `internal/kernel` (only
    `internal/kernel/lspool` types and `internal/workspace`). Enforced by
    `internal/lint/nokernel2semantic`. Phase 62 inherits.
  - `internal/semantic/types/...` and `internal/semantic/graph/...` MUST NOT
    import `duckdb-go` directly. Enforced by `cmd/vet-noduckdb/`. They go
    through `internal/semantic/store/`.
  - `internal/graph/` MUST have zero project imports (D-01 invariant — the
    package is reusable from any layer).
- **Middleware order.** Phase 62 does NOT touch middleware (rank/types live
  below the MCP layer).
- **No second `GrammarRegistry`.** Type resolvers read facts already extracted
  by Phase 59 — they do NOT use tree-sitter directly (BUG-04 / EXTRACT-05).
- **Phase 60 D-04 must not regress.** `current_epoch` per-tx CAS contract is
  Phase 63 compaction's load-bearing input. Phase 62's score writes advance
  `current_epoch` correctly via `BeginOverlayTx` but MUST NOT advance
  `graph_version` themselves — only `ApplyRepair` bumps the rank version.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Generic deterministic PR algorithm | Layer 0 algorithm pkg (`internal/graph/`) | — | Pure math; reusable by repomap (Layer 1) AND semantic graph (Layer 2). No project imports per D-01. |
| Repomap PR adapter | Layer 1 RepoMap (`internal/repomap/`) | — | Existing `FileGraph` consumers continue to call `g.PageRank(...)` and `g.RankFiles(...)`; only the body delegates to `internal/graph`. |
| Semantic PR call site | Layer 2 Semantic (`internal/semantic/graph/`) | `internal/semantic/store/` | Wraps store reads (`QueryEffectiveEdges`) with PR engine; produces score rows the store persists. |
| `graph_version` advance | Layer 2 Store (`internal/semantic/store/overlay.go`) | Layer 2 Live handler | Counter lives in `semantic_live_overlay_meta`; bumped only inside `ApplyRepair`, called by post-commit hook in handler. |
| Repair decision (is this a graph-changing tx?) | Layer 2 Live handler (`internal/semantic/live/handler/`) | Layer 2 Coalescer | Same lane-decision pattern as Phase 61 D-05. Diff inspection happens here. |
| Background scheduling | Layer 2 RankScheduler (`internal/semantic/graph/scheduler.go` or sibling) | — | One goroutine per workspace, errgroup-owned; no other layer schedules PR. |
| Type resolver dispatch | Layer 2 Types (`internal/semantic/types/`) | — | Per-language behind shared `Resolver` interface; consumes facts from store, emits edges via `tx.UpsertEdgesWithMerge`. |
| MCP exposure (rank, types, status) | Layer 0 MCP runtime (Phase 64) | — | Phase 62 ships ZERO MCP tools. Only `Ranker`/`Resolver` interfaces. |

## Standard Stack

### Core (already vendored — no new deps)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `cmp` | Go 1.21+ stdlib | Generic constraint for `internal/graph` engine | D-02. `cmp.Ordered` covers `string` (file paths) AND `uint64` (NodeID) without custom Less variant. [VERIFIED: stdlib] |
| `sort` | stdlib | Sort-before-iterate determinism | D-03. `sort.Slice` over node-ids in every iteration. [VERIFIED: stdlib] |
| `math` | stdlib | PR convergence math | Existing `internal/repomap/pagerank.go` uses `math.Abs` for L1 distance. [VERIFIED: stdlib] |
| `crypto/sha256` | stdlib | Hex-digest assertion for GRAPH-01 | Standard determinism contract for byte-identical-output tests. [VERIFIED: stdlib] |
| `golang.org/x/sync/errgroup` | already vendored | RankScheduler goroutine ownership | Same pattern as Phase 60 coalescer + Phase 61 enrichment manager (`internal/daemon/daemon.go:645-659`). [VERIFIED: codebase] |
| `log/slog` | stdlib | Structured logging | Project standard. [VERIFIED: codebase] |
| `github.com/prometheus/client_golang` | already vendored | Bounded-label metrics | `internal/obs/metrics.go` registers all CounterVec/HistogramVec/GaugeVec via this. Phase 62 metrics follow same pattern. [VERIFIED: codebase] |

### Supporting (existing project packages)

| Package | Purpose | When to Use |
|---------|---------|-------------|
| `internal/semantic/store` | DuckDB facade for snapshot ⊕ overlay − tombstones reads + overlay writes | All Phase 62 fact reads + score/cluster/edge writes |
| `internal/semantic/store.OverlayTx` | Per-tx overlay write surface (D-04 epoch contract) | Score row writes, cluster row writes, edge merge writes |
| `internal/semantic/live/handler` | Post-commit hook fire site for `ApplyRepair` | Decides "is this a graph-changing tx" (D-06) |
| `internal/semantic/lspenrich` | Phase 61 worker emitting validated LSP edges | Source of `confidence=1.0` edges Phase 62 merges with comment-derived 0.60 edges |
| `internal/repomap` | Existing FileGraph consumer — Phase 62 keeps API surface | Migrates internals to `internal/graph` |
| `internal/obs` | Bounded-label metric registration | All Phase 62 metrics (closed-enum labels per `metrics_labels_test.go`) |
| `internal/config` (`SerenaConfig.SemanticIndex`) | koanf-bound config | All Phase 62 config keys (most ALREADY shipped — see Environment Availability) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Generic `cmp.Ordered` engine | Generic with custom `Less` func | Custom Less is more flexible BUT D-02 explicitly defers it (`cmp.Ordered` covers v1; future structured-id types add overload then). Reject. |
| Single PR algorithm | Library import (`gonum`) | gonum has stricter graph types and would couple `internal/graph` to a third-party API surface; D-01 explicitly mandates `math/sort/cmp` only. Hand-rolling is ~150 LOC (existing repomap proves this) and gives byte-equality control for GRAPH-01. Reject. |
| Comment parser libraries (TSDoc, JSDoc) | Hand-rolled | D-12 forbids new third-party deps. The grammars we need are tiny (extract `@param {Type}`, `@type {Type}`, `@returns {Type}`, `# type: Type`, GoDoc `// Foo returns ...` patterns). Hand-rolled is correct. |
| 1-hop frontier | k-hop frontier (k=2 per SPEC §18.4) | SPEC §18.4 does k=2. CONTEXT.md D-09 explicitly chose 1-hop for predictable budget over rank accuracy mid-edit. Reject k=2. |

**No new dependencies.** Every package needed is already in `go.sum`.

**Version verification:** Not required — Phase 62 introduces no new third-party
deps. Existing deps were verified in Phases 57–61.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌──────────────────────────────────┐
                    │ live/handler/handler.go          │
                    │  (post-commit hook)              │
                    │                                  │
                    │  diff → ComputeGraphRepair(diff) │
                    │       → typed GraphRepair        │
                    └────────┬─────────────────────────┘
                             │
                             │ (under per-workspace overlay mutex)
                             ▼
                    ┌──────────────────────────────────┐
                    │ semantic/graph.Engine.ApplyRepair│
                    │  - WriteInvalidations consumer   │
                    │    (Phase 61 stub fill)          │
                    │  - tx.UpsertEdgesWithMerge       │
                    │    (D-14 two-phase merge)        │
                    │  - bump graph_version            │
                    │  - emit graph_version channel    │
                    └────────┬─────────────────────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼                             ▼
   ┌─────────────────────┐       ┌─────────────────────┐
   │ RankScheduler       │       │ Reader path         │
   │  (1 goroutine / WS) │       │  - Ranker.Rank      │
   │                     │       │  - Ranker.Status    │
   │ debounce timer      │       │  - Resolver.*       │
   │  → 1-hop frontier   │       │                     │
   │  → internal/graph.  │       │  computes           │
   │    PageRank(...)    │       │  score_status @     │
   │  → tx.Upsert        │       │  read time (D-07)   │
   │    GraphScores      │       └─────────────────────┘
   │                     │
   │ long-idle timer     │
   │  → full recompute   │
   │    (preempted →     │
   │     approximate)    │
   └────────┬────────────┘
            │
            ▼
   ┌─────────────────────────────────────────────────┐
   │ internal/graph (pure)                           │
   │  PageRank[T cmp.Ordered](nodes, edges, opts)    │
   │   - sort nodes once at entry                    │
   │   - iterate sorted slice every iteration (D-03) │
   │   - dangling-mass redistribution                │
   │   - personalization carry-through               │
   └─────────────────────────────────────────────────┘
            ▲
            │ (adapter call)
            │
   ┌────────┴────────────┐         ┌────────────────────┐
   │ internal/repomap/   │         │ internal/semantic/ │
   │   pagerank.go       │         │   graph/ranker.go  │
   │ (FileGraph adapter, │         │ (NodeID adapter,   │
   │  re-pinned vectors) │         │  store-backed)     │
   └─────────────────────┘         └────────────────────┘

   ┌─────────────────────────────────────────────────┐
   │ internal/semantic/types/                        │
   │  resolver.go (dispatch by fact.Language)        │
   │     ├── golang/resolver.go     (full ladder)    │
   │     ├── python/resolver.go     (full ladder)    │
   │     ├── typescript/resolver.go (full ladder)    │
   │     ├── java/stub.go    (LSP-conditional 0.20)  │
   │     ├── php/stub.go     (always 0.20)           │
   │     └── ruby/stub.go    (always 0.20)           │
   │                                                 │
   │  shared core:                                   │
   │     chain.go (a.b.c.d access walk)              │
   │     fixpoint.go (bounded loop, max_iter=8)      │
   │     ladder.go (SPEC §38.2 7-tier model)         │
   │     emit.go (RESOLVES_TO/CALLS/USES_TYPE)       │
   │                                                 │
   │  Comment-derived edges: confidence=0.60 +       │
   │   validation_state="unresolved"                 │
   │  → ApplyRepair upgrades to 1.0 when matching    │
   │    (src, dst, kind) LSP edge lands              │
   └─────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── graph/                              # NEW (D-01)
│   ├── pagerank.go                     # PageRank[T cmp.Ordered]
│   ├── pagerank_test.go                # determinism + hex-digest tests
│   ├── options.go                      # Options struct (damping, epsilon, ...)
│   ├── personalize.go                  # Personalized PR + seed builder
│   └── testdata/pagerank/              # golden hex digests for GRAPH-01
├── repomap/
│   ├── pagerank.go                     # REWRITE: adapter over internal/graph
│   ├── pagerank_test.go                # RE-PINNED test vectors (D-04)
│   └── graph.go                        # UNCHANGED — FileGraph stays
├── semantic/
│   ├── graph/                          # NEW (Claude's Discretion: layout)
│   │   ├── doc.go
│   │   ├── ranker.go                   # Ranker interface + impl
│   │   ├── repair.go                   # GraphRepair type + ComputeGraphRepair
│   │   ├── apply_repair.go             # Engine.ApplyRepair (single-bump site)
│   │   ├── scheduler.go                # RankScheduler goroutine
│   │   ├── frontier.go                 # 1-hop frontier algorithm
│   │   ├── full_recompute.go           # full recompute path + preemption
│   │   ├── invalidations_consumer.go   # WriteInvalidations row → GraphRepair
│   │   ├── status.go                   # score_status read API (D-07)
│   │   ├── metrics.go                  # bounded-label metric registration
│   │   ├── trace.go                    # span helpers
│   │   └── testdata/repair/            # overlay-tx fixtures with predictable diffs
│   ├── cluster/                        # NEW (D-?)
│   │   ├── doc.go
│   │   ├── weak.go                     # WeakComponents over effective graph
│   │   ├── persist.go                  # tx.UpsertClusters + UpsertClusterMembers
│   │   └── weak_test.go                # determinism hex-digest test (GRAPH-06)
│   ├── types/                          # NEW (D-11)
│   │   ├── doc.go
│   │   ├── resolver.go                 # Resolver interface + dispatch by Language
│   │   ├── chain.go                    # access-chain walker (shared)
│   │   ├── fixpoint.go                 # bounded fixpoint loop (shared)
│   │   ├── ladder.go                   # SPEC §38.2 7-tier model
│   │   ├── emit.go                     # edge emission + two-phase merge
│   │   ├── golang/
│   │   │   ├── resolver.go             # full ladder
│   │   │   ├── scope.go                # package-by-dir
│   │   │   └── comment.go              # GoDoc parser
│   │   ├── python/
│   │   │   ├── resolver.go
│   │   │   ├── scope.go                # walk to __init__.py
│   │   │   └── comment.go              # type comments + docstring annotations
│   │   ├── typescript/
│   │   │   ├── resolver.go
│   │   │   ├── scope.go                # nearest tsconfig.json
│   │   │   └── comment.go              # TSDoc + JSDoc
│   │   ├── php/stub.go                 # always 0.20 unresolved
│   │   ├── ruby/stub.go                # always 0.20 unresolved
│   │   ├── java/stub.go                # LSP-conditional 0.20
│   │   └── testdata/<lang>/            # per-language fixture trees
│   └── store/
│       └── overlay.go                  # EXTEND: UpsertGraphScores,
│                                       #         UpsertClusters,
│                                       #         UpsertClusterMembers,
│                                       #         UpsertEdgesWithMerge,
│                                       #         WriteInvalidations (real impl)
└── daemon/
    └── daemon.go                        # EXTEND: RankScheduler bootstrap +
                                         #         setter wiring for Resolver
```

### Pattern 1: Sort-Before-Iterate Determinism Patch

**What:** Every iteration over node-keyed maps must consult a pre-sorted node
slice instead of ranging the map directly.

**When to use:** Inside `internal/graph.PageRank` — every place `internal/
repomap/pagerank.go` currently uses `for f := range edges` or `for _, f :=
range files` (where `files` was constructed via map iteration without sort).

**Example (hypothetical patch shape, do NOT take as final):**
```go
// Source: SPEC §18.2 + D-03 + existing internal/repomap/pagerank.go:28-31
package graph

import (
    "cmp"
    "math"
    "sort"
)

type Options struct {
    Damping       float64           // 0.85
    Epsilon       float64           // 1e-6
    MaxIter       int               // 100
    Personalize   map[any]float64   // optional; nil → uniform teleport
}

func PageRank[T cmp.Ordered](nodes []T, edges map[T]map[T]float64, opts Options) map[T]float64 {
    n := len(nodes)
    if n == 0 {
        return nil
    }
    // Sort ONCE — every loop below iterates this slice, never the map.
    sorted := append([]T(nil), nodes...)
    sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

    // Build teleport over sorted slice (deterministic dist).
    teleport := make(map[T]float64, n)
    if len(opts.Personalize) > 0 {
        // ... personalize, then re-normalize, iterating `sorted`
    } else {
        for _, k := range sorted {
            teleport[k] = 1.0 / float64(n)
        }
    }

    rank := make(map[T]float64, n)
    for _, k := range sorted {
        rank[k] = teleport[k]
    }

    outWeight := make(map[T]float64, n)
    for _, src := range sorted {
        var w float64
        // Iterate edges[src] — order doesn't matter here (associative sum).
        for _, ww := range edges[src] {
            w += ww
        }
        outWeight[src] = w
    }

    fn := float64(n)
    for it := 0; it < opts.MaxIter; it++ {
        var dangling float64
        for _, k := range sorted {
            if outWeight[k] == 0 {
                dangling += rank[k]
            }
        }
        next := make(map[T]float64, n)
        for _, k := range sorted {
            next[k] = (1-opts.Damping)*teleport[k] + opts.Damping*dangling/fn
        }
        for _, src := range sorted { // KEY: iterate sorted, not map.
            tw := outWeight[src]
            if tw == 0 {
                continue
            }
            contribution := opts.Damping * rank[src] / tw
            // edges[src] inner iteration order: doesn't matter (commutative sum).
            for dst, w := range edges[src] {
                next[dst] += contribution * w
            }
        }
        var diff float64
        for _, k := range sorted {
            diff += math.Abs(next[k] - rank[k])
        }
        rank = next
        if diff < opts.Epsilon {
            break
        }
    }
    return rank
}
```

The associativity of inner-edge sums (commutative addition) means the inner
`for dst, w := range edges[src]` is safe to leave map-ranged — its
contribution to `next[dst]` is order-independent. The outer iteration over
`sorted` is what guarantees byte-equality across runs.

### Pattern 2: GraphRepair Diff Computation in Handler

**What:** The post-commit hook in `internal/semantic/live/handler/handler.go`
inspects each committed `OverlayTx`'s diff and produces a typed `GraphRepair`
value when SPEC §17.2 trigger conditions are met.

**When to use:** EVERY commit returning from `BeginOverlayTx`. Body-only
edits return `GraphRepair{}` (empty), and `ApplyRepair` short-circuits if
nothing in the repair value would advance `graph_version`.

**Example shape (Source: SPEC §17.2):**
```go
// Source: SPEC §17.2 lines 1647-1695
type GraphRepair struct {
    RemovedNodes  []NodeID
    DirtyNodes    []NodeID
    UpsertedEdges []GraphEdge
    RemovedEdges  []GraphEdge
    InvalidatedIncoming []NodeID
    InvalidatedOutgoing []NodeID
}

func ComputeGraphRepair(diff FileFactDiff) GraphRepair {
    repair := GraphRepair{}
    for _, sym := range diff.RemovedSymbols {
        repair.RemoveNode(sym.NodeID)
        repair.InvalidateIncoming(sym.NodeID)
        repair.InvalidateOutgoing(sym.NodeID)
    }
    for _, sym := range diff.ChangedSymbols {
        if sym.SignatureChanged || sym.ExportedChanged ||
            sym.KindChanged || sym.StableKeyChanged {
            repair.InvalidateIncoming(sym.NodeID)
        }
        repair.InvalidateOutgoing(sym.NodeID)
        repair.MarkNodeDirty(sym.NodeID)
    }
    for _, edge := range diff.AddedEdges {
        repair.AddOrUpdateEdge(edge)
        repair.MarkNodeDirty(edge.SrcNodeID)
        repair.MarkNodeDirty(edge.DstNodeID)
    }
    return repair
}

func (r GraphRepair) IsEmpty() bool {
    return len(r.RemovedNodes) == 0 && len(r.DirtyNodes) == 0 &&
        len(r.UpsertedEdges) == 0 && len(r.RemovedEdges) == 0 &&
        len(r.InvalidatedIncoming) == 0 && len(r.InvalidatedOutgoing) == 0
}
```

`ApplyRepair` checks `repair.IsEmpty()` first — short-circuits with no
mutation if the diff was body-only. This keeps the bump-trigger discipline
honest.

### Pattern 3: RankScheduler Goroutine Lifecycle

**What:** One goroutine per workspace, mirrors `internal/semantic/live/
coalescer/coalescer.go` lifecycle: errgroup-owned, `ctx.Done()` shutdown,
two timers (debounce, long-idle), input channel from `ApplyRepair`.

**When to use:** Daemon bootstrap — wired in same errgroup as Phase 60
coalescer + Phase 61 enrichment manager (`internal/daemon/daemon.go:645-659`).

**Example (mirrors `coalescer.Coalescer.Run`):**
```go
// Source: internal/semantic/live/coalescer/coalescer.go:153-171 lifecycle pattern.
type RankScheduler struct {
    repoID   string
    in       chan GraphVersion        // bumps from ApplyRepair
    debounce time.Duration            // pagerank.repair_debounce_ms
    longIdle time.Duration            // pagerank.full_recompute_idle_ms
    ranker   *Engine
    logger   *slog.Logger
    metrics  MetricsSink
}

func (s *RankScheduler) Run(ctx context.Context) error {
    var debounceTimer, longIdleTimer *time.Timer
    var lastSeenGV GraphVersion
    for {
        select {
        case <-ctx.Done():
            if debounceTimer != nil { debounceTimer.Stop() }
            if longIdleTimer != nil { longIdleTimer.Stop() }
            return ctx.Err()
        case gv := <-s.in:
            lastSeenGV = gv
            if debounceTimer != nil { debounceTimer.Stop() }
            debounceTimer = time.AfterFunc(s.debounce, func() {
                s.runIncrementalRepair(ctx, lastSeenGV)
            })
            if longIdleTimer == nil {
                longIdleTimer = time.AfterFunc(s.longIdle, func() {
                    s.maybeFullRecompute(ctx)
                })
            }
        }
    }
}
```

### Anti-Patterns to Avoid

- **Iterating Go maps for rank computation.** Existing project rule
  (Pitfall C4 in `.planning/research/PITFALLS.md`); easy to slip in
  incremental repair. Sort once, iterate the slice everywhere.
- **Bumping `graph_version` from anywhere except `ApplyRepair`.** D-06 is a
  hard contract. `BeginOverlayTx` does NOT bump (would advance on every
  body-edit). Phase 63 compaction MUST NOT touch (D-05).
- **Cross-workspace `RankScheduler` serialization.** D-08: ONE goroutine
  per workspace, never global. Cross-workspace serialization of PR is a
  bug surface to avoid.
- **Reading score rows directly from `semantic_graph_scores` and trusting
  `graph_version`.** D-07: `score_status` is computed at READ time; reader
  must compare row.gv to current overlay_meta.gv.
- **Comment edge surviving at lower confidence after LSP refutation.**
  D-14: when LSP says different dst, comment row is DELETED, NOT preserved
  at lower confidence.
- **Java stub emitting 0.20 even when LSP edge already exists at 1.0.**
  D-12: Java stub short-circuits on LSP-confirmed facts.
- **Hand-iterating LSP edges to find merge candidates.** Use store helper
  `tx.UpsertEdgesWithMerge` so the merge predicate `(src_node_id,
  dst_node_id, edge_kind)` is enforced at the storage boundary, not in
  Go logic.
- **Tree-sitter access in type resolver.** Phase 59 already extracted
  facts; Phase 62 reads facts via `effective.go`. No new `GrammarRegistry`
  (CLAUDE.md project constraint).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-workspace mutex for score writes | A new mutex map | `*store.Store.overlayLockFor(repoID)` (existing per-workspace mutex acquired by `BeginOverlayTx`) | Phase 60 D-04 contract: bump + score-write happens UNDER the same mutex that protects `current_epoch`. Re-using avoids a second lock-ordering surface. |
| Two-phase merge predicate `(src, dst, kind)` | Application-side dedupe loop | New `tx.UpsertEdgesWithMerge` SQL helper that does `DELETE WHERE (src,dst,kind)=... AND source LIKE 'comment.%'; INSERT ...` in one tx | Race-free at the storage boundary. Mirrors how `MarkSymbolsDeleted` does per-id loop in SQL. |
| Coalescer timer bookkeeping | New time.Timer + sync.Mutex dance | Mirror `internal/semantic/live/coalescer/coalescer.go:193-202` | The closed-channel + reset-on-event pattern is already battle-tested. |
| Errgroup wiring for the scheduler | New goroutine bootstrap | Mirror `internal/daemon/daemon.go:655-659` (Phase 61 LSP enrichment manager pattern) | Same shutdown semantics, same observability. |
| Bounded-label metrics registration | Direct prometheus calls | Use `internal/obs/metrics.go` Vec helpers + `metrics_labels_test.go` carve-out | Project-wide closed-enum discipline. Adds a single helper method (e.g., `Metrics.SemanticGraphPagerankObserve(scope, projection, dur)`) and the corresponding label carve-out. |
| Comment grammar libraries | A Go TSDoc/JSDoc parser dep | Hand-roll per-language regexes for the small surface (`@param {Type}`, `@returns {Type}`, `@type {Type}`, `# type: Type`, GoDoc patterns) | D-12 forbids new third-party deps. The grammar surface is tiny. |
| Generic graph data structure | An adjacency-list framework | `map[T]map[T]float64` is the smallest correct shape (already in use by repomap) | D-01 hard invariant: zero project imports. Map-of-maps is sufficient for v1. |
| WriteInvalidations row format | New table | Phase 61 already chose the typed stub method (`WriteInvalidations(ctx)` on the cascade tx interface, line 84 of `internal/semantic/lspenrich/cascade.go`). Phase 62 fills with `INSERT INTO semantic_invalidations (...)` and a consumer that polls/peeks | Schema can be a reuse of an existing table OR a new one — planner picks. SPEC §17.2 doesn't dictate. |

**Key insight:** The biggest lift in Phase 62 is the *interface boundary
discipline*, not the algorithms. PageRank is ~150 LOC. Weak components is ~30
LOC. The fixpoint loop is bounded at iter=8. What's load-bearing is: (1)
keeping `internal/graph` import-free; (2) keeping the `graph_version` bump
in exactly one site; (3) keeping the comment merge `(src, dst, kind)`
predicate enforced at the storage boundary; (4) keeping the per-language
type resolver behind a single dispatch.

## Runtime State Inventory

This is a *new feature* phase (not a rename/refactor). No runtime state from
prior versions migrates. Confirmation per category:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — Phase 62 is the first writer of non-zero `graph_version`, score rows, cluster rows, and `RESOLVES_TO`/`USES_TYPE` edges. Existing rows in `semantic_graph_scores` and `semantic_clusters` are guaranteed empty (tables shipped Phase 57 but never populated — `migrations.go:248-289`). | None — Phase 62 starts from clean tables. |
| Live service config | None — no external service config carries Phase 62 state. | None. |
| OS-registered state | None — no daemons, scheduled tasks, or system services carry Phase 62 state. | None. |
| Secrets/env vars | None — Phase 62 introduces no secrets. | None. |
| Build artifacts | None — Phase 62 is pure source addition. The new `internal/graph/` package builds with the rest of the tree. | None. |

**The canonical question** *(after every file in the repo is updated, what
runtime systems still have the old string cached, stored, or registered?)*:
N/A — this is an additive feature, not a rename.

## Common Pitfalls

### Pitfall 1: Map-Iteration Slip in Incremental Repair

**What goes wrong:** The full-recompute path does sort-before-iterate
correctly, but the 1-hop frontier path computes `frontier := changed_nodes
∪ neighbors` and ranges the resulting `map[NodeID]struct{}` directly when
seeding personalization. The frontier subgraph is determined by the order
the seed map is iterated, which Go randomizes per process — same input,
different output, GRAPH-01 byte-equality fails.

**Why it happens:** D-03 mandates sort-before-iterate ONLY in the engine.
The frontier algorithm lives in `internal/semantic/graph/frontier.go` —
separate file, easy to forget the same discipline.

**How to avoid:** Audit every file that touches a node-keyed map for
iteration. Add a `sort.Slice` wrapper helper in `internal/semantic/graph/`
(`func sortedNodeIDs(m map[NodeID]struct{}) []NodeID`) and use it
universally. Add a test that runs incremental repair 100 times and asserts
the same hex digest.

**Warning signs:** GRAPH-01 hex-digest test passes; full-recompute
determinism test passes; incremental-repair determinism test FAILS or is
not present. Prom metric: `helix_semantic_graph_repair_total{outcome=
"applied"}` shows non-deterministic ranking output across runs.

### Pitfall 2: `graph_version` Bumped Twice for One Edit

**What goes wrong:** The handler's post-commit hook calls `ApplyRepair`,
which bumps `graph_version`. If the score-row write path inside the
incremental-repair scheduler ALSO opens a `BeginOverlayTx` and the bumped
counter on `semantic_live_overlay_meta.graph_version` is mistakenly
incremented again (e.g., scheduler does `UPDATE ... SET graph_version =
graph_version + 1` thinking it's a separate concept from the rank rows'
column), readers see a `graph_version` that no actual edit produced.

**Why it happens:** D-05 splits `current_epoch` from `graph_version`.
Scheduler opens overlay tx → epoch advances (correct); some patch could
also update `graph_version` (incorrect — only `ApplyRepair` does).

**How to avoid:** Acceptance test #4: commit N overlay txs (some edge-
changing, some not) and assert `graph_version == count(edge-changing-
txs)`. The scheduler does NOT touch `graph_version`; it reads it.

**Warning signs:** Acceptance test #4 fails. Prom gauge
`helix_semantic_graph_version` advances faster than `helix_semantic_live_
updates_total{kind=ChangeFileModified}`.

### Pitfall 3: Comment Edge Re-Inserted After LSP Refutes It

**What goes wrong:** Comment parser emits `(src=Foo, dst=A, kind=RESOLVES_
TO, conf=0.60)`. Later LSP cascade emits `(src=Foo, dst=B, kind=RESOLVES_
TO, conf=1.00)`. The two-phase merge correctly deletes the 0.60 row and
inserts the 1.00 row. Then on the next file edit, the comment parser runs
again, regenerates the 0.60 row, inserts it, and now both rows exist —
violating the merge predicate "different sources for the same triple are
NOT both allowed in `semantic_live_overlay_edges` after merge."

**Why it happens:** Comment parsing is a per-edit operation; nothing tells
the parser "an LSP-validated edge already exists for this triple, skip the
0.60 emission." D-12's Java short-circuit pattern needs to apply to
ALL languages' comment-derived rows, not just Java's stub.

**How to avoid:** `tx.UpsertEdgesWithMerge` checks for an existing row
with `confidence=1.00` AND matching `(src, dst, kind)` AND `source LIKE
'lsp.%'` BEFORE inserting the comment row. If found, it skips the insert
silently — the LSP fact already covers that triple. Acceptance test #10:
write a TSDoc type comment, assert 0.60 edge exists, inject matching
Phase 61 LSP edge, assert in-place upgrade to 1.0, then re-emit the
comment fact and assert NO 0.60 row appears.

**Warning signs:** Two rows in `semantic_live_overlay_edges` for same
`(src, dst, kind)` triple in integration tests.

### Pitfall 4: Frontier Overflow Race

**What goes wrong:** Threshold-overflow path marks all score rows
`stale`, schedules full recompute on long-idle timer, and returns. If a
new `ApplyRepair` arrives BEFORE the long-idle timer fires AND the new
repair's frontier is < 5000, the scheduler runs incremental repair on a
graph where most score rows are already `stale` — incremental repair
overwrites only frontier rows, leaving the rest stale, and the long-idle
timer never fires because it was reset by the new advance.

**Why it happens:** D-09 says "above threshold → mark stale + schedule
full recompute". D-08 says "each advance resets debounce timer". The
long-idle timer reset semantics aren't explicit.

**How to avoid:** The long-idle timer for full recompute is a SEPARATE
timer from the debounce timer; an advance resets DEBOUNCE only. The
long-idle timer is reset only when (a) a full recompute completes, or
(b) an incremental repair's frontier is < threshold AND the repair
covered all stale rows. Acceptance test #6: inject a 6000-node-frontier
change, assert short-circuit + full-recompute scheduled, then inject
multiple 100-node-frontier changes BEFORE the long-idle elapses, and
assert full recompute STILL fires when long-idle elapses.

**Warning signs:** Long bursts of incremental advances followed by score
rows stuck at `stale` for >> long-idle duration.

### Pitfall 5: Per-Package Fixpoint Cross-Dependency Cycle

**What goes wrong:** Two packages A and B; A depends on B, B depends on
A. Per-package fixpoint runs over A first, leaves cross-package chains
unresolved at confidence ≤ 0.60. Then runs over B, but state from B's
resolution doesn't propagate back to A. User sees inconsistent
confidences across the cycle.

**Why it happens:** D-13 explicitly defers cross-package fixpoint —
v1 emits at last-in-package confidence with `validation_state=
"unresolved"`. The cycle is deliberately accepted as an unresolved
case in v1.

**How to avoid:** Document explicitly in the type resolver's package doc
comment that cross-package chains stop at `last_in_package_confidence`.
Acceptance test #9 (chain depth=9) covers the depth bound; an
additional test asserting "package-A symbol referencing package-B
symbol resolves at most at 0.60 with `validation_state=unresolved`"
makes the cross-package boundary explicit. Phase 64's MCP responses
must surface `validation_state` so users can detect the unresolved
case.

**Warning signs:** Type-resolution acceptance tests pass within a
package but mixed-package chains return `confidence=0.20` when the
user expects 0.45+ (heuristic).

## Code Examples

Verified patterns from official sources / existing codebase:

### Example 1: BeginOverlayTx + UpsertGraphScores (extends Phase 60 contract)

```go
// Source: internal/semantic/store/overlay.go:79-141 (BeginOverlayTx) +
// Phase 62 D-08 score-row write path
//
// Score rows write through BeginOverlayTx — they advance current_epoch
// (Phase 60 D-04 contract) but do NOT advance graph_version (D-06).

func writeScoreRows(ctx context.Context, st *store.Store, repoID string,
    rows []ScoreRow) error {
    tx, err := st.BeginOverlayTx(ctx, repoID)
    if err != nil {
        return fmt.Errorf("score write: begin tx: %w", err)
    }
    if err := tx.UpsertGraphScores(ctx, rows); err != nil {
        _ = tx.Rollback()
        return err
    }
    return tx.Commit()
}
```

### Example 2: Two-Phase Comment Merge SQL (D-14)

```sql
-- Source: SPEC §38.7 + D-14 + existing semantic_live_overlay_edges schema
-- (migrations.go:349-364, PK = (repo_id, edge_id), index on (src,dst,kind))
--
-- Inside tx.UpsertEdgesWithMerge:

-- Step 1: if an LSP-validated row already exists for the same triple,
--          skip the insert (preserves LSP-wins invariant).
SELECT 1 FROM semantic_live_overlay_edges
 WHERE repo_id = ? AND src_node_id = ? AND dst_node_id = ?
   AND edge_kind = ?
   AND validation_state = 'validated'
   AND confidence >= 1.0
   AND source LIKE 'lsp.%'
   AND status = 'live'
 LIMIT 1;
-- if row found → skip the comment-derived insert.

-- Step 2: if a matching LSP row arrives later, delete any comment row
--          for the same triple before the LSP insert.
DELETE FROM semantic_live_overlay_edges
 WHERE repo_id = ? AND src_node_id = ? AND dst_node_id = ?
   AND edge_kind = ?
   AND source LIKE 'comment.%'
   AND status = 'live';

-- Step 3: insert the new edge (LSP or comment).
INSERT INTO semantic_live_overlay_edges (repo_id, edge_id, src_node_id,
    dst_node_id, edge_kind, status, validation_state, confidence, weight,
    source, fact_json, updated_at, write_epoch)
VALUES (?, ?, ?, ?, ?, 'live', ?, ?, ?, ?, ?, now(), ?);
```

### Example 3: Per-Language Resolver Dispatch

```go
// Source: D-11 layout + D-12 dispatch
// internal/semantic/types/resolver.go

type Resolver interface {
    ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error)
    ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error)
}

type registry struct {
    byLang map[string]Resolver
}

func NewDispatcher() *registry {
    return &registry{byLang: map[string]Resolver{
        "go":         golang.NewResolver(),
        "python":     python.NewResolver(),
        "typescript": typescript.NewResolver(),
        "javascript": typescript.NewResolver(), // shared with TS
        "java":       java.NewStub(),
        "php":        php.NewStub(),
        "ruby":       ruby.NewStub(),
    }}
}

func (r *registry) ResolveChain(ctx context.Context,
    req ChainRequest) (ChainResponse, error) {
    impl, ok := r.byLang[req.Language]
    if !ok {
        // Languages without coverage emit unresolved 0.20 rows so consumers
        // can detect "no type info" without missing rows (D-12 invariant).
        return ChainResponse{
            Resolved: false, Confidence: 0.20,
            ValidationState: "unresolved",
            Reason: "no resolver for language: " + req.Language,
        }, nil
    }
    return impl.ResolveChain(ctx, req)
}
```

### Example 4: Java Short-Circuit on LSP-Confirmed Facts

```go
// Source: D-12 Java stub + Phase 61 D-07 cascade emits at confidence=1.0
// internal/semantic/types/java/stub.go

type Stub struct{ store EffectiveReader }

func NewStub(store EffectiveReader) *Stub { return &Stub{store: store} }

func (s *Stub) ResolveChain(ctx context.Context,
    req ChainRequest) (ChainResponse, error) {
    // Check if Phase 61 already emitted a RESOLVES_TO edge for this ref.
    edges, err := s.store.QueryEffectiveEdges(ctx, store.EdgeQuery{
        RepoID:    req.RepoID,
        SrcNodeID: req.RefNodeID,
        EdgeKind:  "RESOLVES_TO",
    })
    if err != nil {
        return ChainResponse{}, err
    }
    for _, e := range edges {
        if e.Confidence >= 1.0 && strings.HasPrefix(e.Source, "lsp.") {
            // Java stub short-circuits — LSP already covered this
            // reference. Do NOT emit competing 0.20 row.
            return ChainResponse{
                Resolved:        true,
                Target:          e.DstNodeID,
                Confidence:      1.0,
                ValidationState: "validated",
                Source:          e.Source, // preserve "lsp.<call>" lineage
            }, nil
        }
    }
    // No LSP edge — emit 0.20 unresolved.
    return ChainResponse{
        Resolved: false, Confidence: 0.20,
        ValidationState: "unresolved",
        Reason: "java: no LSP fact + no static analyzer",
    }, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Repomap `(g *FileGraph).PageRank(...)` impl in repomap | Generic `internal/graph.PageRank[T cmp.Ordered]` consumed by repomap as adapter | Phase 62 D-01..D-04 | Single algorithm to audit; one locus for determinism patch; no duplicate-impl drift. |
| `graph_version` pinned at 0 | `graph_version` advances per `ApplyRepair` call | Phase 62 D-05/D-06 | Score rows now meaningfully versioned; readers can compute `score_status` (D-07). |
| `tx.WriteInvalidations` is no-op stub (Phase 60) | `tx.WriteInvalidations` writes typed invalidations; consumer turns rows into `GraphRepair` (Phase 62 D-09) | Phase 62 (this phase) | Phase 61 cascade can stay unchanged; consumer side fills in. |
| Comment-derived edges absent from overlay | Comment-derived edges emitted at `confidence=0.60` immediately, upgraded in-place to 1.0 when LSP confirms (D-14) | Phase 62 | Two-phase merge predicate `(src, dst, kind)` is the new contract. |
| No type resolver | Per-language resolvers under `internal/semantic/types/<lang>/` with shared core | Phase 62 D-11..D-13 | Dispatcher by `fact.Language`; PHP/Ruby always 0.20 unresolved; Java short-circuits on LSP facts. |

**Deprecated/outdated:** None — Phase 62 is purely additive on top of
Phase 60 + Phase 61. Existing repomap PR test vectors are re-pinned (D-04)
but the algorithm's mathematical properties are preserved.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `tx.UpsertEdgesWithMerge` is the planner-chosen helper name. CONTEXT.md doesn't mandate the exact API name; planner may choose `tx.MergeEdge` or `tx.UpsertEdgeIfHigherConfidence`. | Don't Hand-Roll table, Code Example 2 | LOW — name change only; the merge predicate `(src, dst, kind)` is the load-bearing invariant. |
| A2 | The post-commit handler hook is the natural fire site for `ApplyRepair`. CONTEXT.md says "the handler stays the single decision point for 'is this a graph-changing tx' (mirrors Phase 61 D-05's lane-decision pattern)". | Architecture Diagram, Pattern 2 | LOW — the alternative (coalescer post-flush) is also valid; planner picks. Either way, single-site. |
| A3 | The `WriteInvalidations` consumer is push-based: handler writes rows in the same overlay tx that committed the change, then notifies the scheduler via the `graph_version` channel. Pull-based polling is the alternative. | Pattern 3 | LOW — push is simpler and matches the coalescer template; planner can choose pull if simpler in practice. |
| A4 | Java's LSP short-circuit reads `RESOLVES_TO` edges via `QueryEffectiveEdges`. CONTEXT.md only says the stub "short-circuits on Phase 61 LSP-confirmed facts" without specifying the read path. | Code Example 4 | LOW — alternative is to receive LSP edge state through dispatcher; planner picks. |
| A5 | The byte-equality hex digest is sha256 of the sorted-key score-vector serialization. CONTEXT.md mentions "byte-identical outputs ... hex digest of sorted-key score-vector serialization". sha256 is the conventional choice; SPEC doesn't lock in. | Validation Architecture | LOW — any cryptographic hash works; sha256 is the project default. |

**If this table is empty:** All claims in this research were verified or
cited — no user confirmation needed.

## Open Questions (RESOLVED)

1. **Plan-to-wave mapping for the 5 plans.** CONTEXT.md provides 5 plans
   but does not lock wave structure. P01 (engine + repomap migration) is
   a self-contained unit. P02 (graph_version + ApplyRepair + read API)
   depends on P01 only for `internal/graph` use, not for repomap. P03
   (RankScheduler + frontier + invalidation consumer) depends on P02.
   P04 (clustering) depends on P02 (reads graph_version). P05 (types)
   depends on P02 (uses ApplyRepair for two-phase merge). Suggested wave
   order: Wave 1 = P01 + P02 (parallel-safe at the file level); Wave 2 =
   P03 + P04 + P05 (parallel-safe). Planner decides.
   - What we know: dependencies above.
   - What's unclear: whether planner prefers narrow waves (one plan per
     wave for sequencing) or wider waves (parallel work).
   - RESOLVED: planner picks; the dependency DAG above is correct.

2. **Test-fixture seeding for type resolver.** Per-language fixture trees
   under `internal/semantic/types/testdata/<lang>/` need fact-level
   inputs (extracted symbols/references/edges from Phase 59) plus
   expected outputs (resolved chain + confidence + validation_state).
   - What we know: Phase 59 already has per-language testdata under
     `internal/semantic/extract/<lang>/testdata/`; those fixtures could
     be reused as fact inputs.
   - What's unclear: whether fact extraction in tests should be a real
     extractor invocation (slow, integration-grade) or hand-written
     fact tables (fast, unit-grade).
   - RESOLVED: hand-written fact tables for unit tests covering
     the ladder; real extractor invocation for end-to-end acceptance
     tests covering TYPES-01..04.

3. **`semantic_invalidations` schema or reuse.** SPEC §17.2 defines a
   typed `GraphRepair` value but does NOT specify the on-disk
   invalidation row format. Phase 61 left `WriteInvalidations(ctx)` as
   a stub; Phase 62 fills the consumer. Planner can either: (a) add a
   new `semantic_invalidations` table (schema migration v3 → v4); (b)
   reuse existing tombstone rows on `semantic_live_overlay_edges` /
   `semantic_live_overlay_symbols` to derive the repair value.
   - What we know: Phase 60 D-04 + tombstone surface is rich enough to
     reconstruct a `GraphRepair` from current overlay rows.
   - What's unclear: whether planner prefers explicit invalidation rows
     (easier to test, replay) or derived (no schema migration).
   - RESOLVED: derived from existing overlay rows. The post-commit
     handler emits the typed `GraphRepair` directly from the `OverlayTx`
     write log; no separate table needed.

4. **`SetRankScheduler` / `SetTypeResolver` setter shape on daemon.**
   CONTEXT.md "Claude's Discretion" allows a bundle.
   - RESOLVED: a single `SetSemanticGraph(ranker Ranker, resolver Resolver)` setter wires both at once, mirroring `SetEnrichmentWorker`. Planner picks.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go stdlib `cmp` package | `internal/graph` generic constraint | ✓ | Go 1.21+ | — |
| `crypto/sha256` | hex-digest assertion test | ✓ | stdlib | — |
| `golang.org/x/sync/errgroup` | RankScheduler goroutine ownership | ✓ | already vendored | — |
| `github.com/prometheus/client_golang` | bounded-label metrics | ✓ | already vendored | — |
| `internal/semantic/store` (DuckDB-backed) | score row + cluster row + edge merge writes | ✓ | shipped Phase 57 | — |
| `internal/semantic/store/migrations.go` v1 (`semantic_graph_scores`, `semantic_clusters`, `semantic_cluster_members`, `semantic_live_overlay_meta.graph_version`) | persistence | ✓ | shipped Phase 57 | — |
| Phase 60 `BeginOverlayTx` per-workspace mutex | score-row write serialization | ✓ | shipped Phase 60 | — |
| Phase 61 `tx.WriteInvalidations` typed stub seam | Phase 62 consumer fill site | ✓ | shipped Phase 61 | — |
| Phase 61 cascade emitting `confidence=1.0, source="lsp.<call>"` edges | two-phase merge LSP side | ✓ | shipped Phase 61 | — |
| `cmp.Ordered` constraint covers `string` AND `uint64` | D-02 | ✓ | Go 1.21+ stdlib | — |
| `internal/lint/nokernel2semantic` analyzer | enforce semantic→kernel boundary | ✓ | shipped | — |
| `cmd/vet-noduckdb` analyzer | enforce duckdb-go boundary | ✓ | shipped | — |
| `internal/obs.MetricsLabels` carve-out test | bounded-label discipline | ✓ | shipped | — |
| Most config keys (`pagerank.damping`, `pagerank.epsilon`, `pagerank.max_iterations`, `clustering.*`, `type_resolution.max_chain_depth`, `type_resolution.max_fixpoint_iterations`, `graph.max_local_pagerank_nodes`) | runtime tunables | ✓ | shipped Phase 57 (`defaults.go:115-159`) | — |

**New config keys Phase 62 must add (NOT yet in `defaults.go`):**
- `pagerank.repair_debounce_ms` (default 2000) — D-08
- `pagerank.full_recompute_idle_ms` (default 60000) — D-08
- `pagerank.full_recompute_threshold` (default 0.25) — D-08
- `types.comment_parsers_enabled` (default `[tsdoc, jsdoc, godoc, python_type_comments, phpdoc, yard]`) — D-12

**Missing dependencies with no fallback:** None — Phase 62 introduces no
new third-party deps.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (`testing` stdlib) + testify (`github.com/stretchr/testify` already vendored) |
| Config file | none — Go's default `*_test.go` discovery |
| Quick run command | `go test -count=1 ./internal/graph/... ./internal/semantic/graph/... ./internal/semantic/cluster/... ./internal/semantic/types/...` |
| Full suite command | `go vet ./... && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GRAPH-01 | Same input → byte-identical PR scores | unit | `go test -count=1 -run TestPageRankDeterministicHexDigest ./internal/graph/...` | ❌ Wave 0 (`internal/graph/pagerank_test.go`) |
| GRAPH-01 | Repomap re-pinned vectors pass | unit | `go test -count=1 ./internal/repomap/...` | ✅ exists; **VECTORS RE-PINNED** in P01 |
| GRAPH-02 | Personalized PR seed weighting under epsilon=1e-6 | unit | `go test -count=1 -run TestPageRankPersonalized ./internal/graph/...` | ❌ Wave 0 |
| GRAPH-02 | Ranked node list returned via `RankNodes` | unit | `go test -count=1 -run TestRankNodesSortedDescending ./internal/graph/...` | ❌ Wave 0 |
| GRAPH-03 | Tie-break by stable-key NodeID | unit | `go test -count=1 -run TestPageRankTiebreakStableKey ./internal/graph/...` | ❌ Wave 0 |
| GRAPH-03 | `Ranker.Rank` response carries `graph_version` | unit | `go test -count=1 -run TestRankerRankCarriesGraphVersion ./internal/semantic/graph/...` | ❌ Wave 0 (`internal/semantic/graph/ranker_test.go`) |
| GRAPH-04 | Frontier > 5000 → mark all stale + schedule full | integration | `go test -count=1 -run TestRankSchedulerFrontierOverflow ./internal/semantic/graph/...` | ❌ Wave 0 |
| GRAPH-04 | Frontier ≤ 5000 → incremental repair writes only frontier rows | integration | `go test -count=1 -run TestRankSchedulerIncremental ./internal/semantic/graph/...` | ❌ Wave 0 |
| GRAPH-05 | `graph_version` monotonic non-decreasing per repo | integration | `go test -count=1 -run TestApplyRepairGraphVersionMonotonic ./internal/semantic/graph/...` | ❌ Wave 0 |
| GRAPH-05 | `score_status ∈ {exact, approximate, stale, missing}` for every read | contract | `go test -count=1 -run TestRankerStatusClosedEnum ./internal/semantic/graph/...` | ❌ Wave 0 |
| GRAPH-05 | `graph_version` advances exactly N times for N edge-changing txs | integration | `go test -count=1 -run TestGraphVersionAdvanceCount ./internal/semantic/graph/...` | ❌ Wave 0 |
| GRAPH-06 | Weak components deterministic — hex digest of sorted (cluster_id, member_node_ids) | unit | `go test -count=1 -run TestWeakComponentsDeterministicHexDigest ./internal/semantic/cluster/...` | ❌ Wave 0 |
| TYPES-01 | Per-language tier coverage emits each ladder confidence at least once | integration | `go test -count=1 -run TestTypeResolverLadderCoverage ./internal/semantic/types/golang/... ./internal/semantic/types/python/... ./internal/semantic/types/typescript/...` | ❌ Wave 0 |
| TYPES-02 | Chain depth=9 emits unresolved on last hop | integration | `go test -count=1 -run TestChainDepthBound ./internal/semantic/types/...` | ❌ Wave 0 |
| TYPES-02 | Fixpoint loop early-exits on no-progress | unit | `go test -count=1 -run TestFixpointEarlyExit ./internal/semantic/types/...` | ❌ Wave 0 |
| TYPES-03 | Comment edge ≤ 0.60 unless LSP-confirmed (in-place upgrade) | integration | `go test -count=1 -run TestCommentEdgeUpgrade ./internal/semantic/types/...` | ❌ Wave 0 |
| TYPES-03 | Java stub short-circuits on LSP-confirmed | integration | `go test -count=1 -run TestJavaStubLSPShortCircuit ./internal/semantic/types/java/...` | ❌ Wave 0 |
| TYPES-04 | Non-converged chains emit unresolved, NEVER validated | integration | `go test -count=1 -run TestNonConvergedAlwaysUnresolved ./internal/semantic/types/...` | ❌ Wave 0 |
| All — closed-enum metric labels carved out | metric label test | `go test -count=1 -run TestMetricsLabels ./internal/obs/...` | ✅ exists; **EXTEND** with Phase 62 labels |
| All — config defaults loaded | unit | `go test -count=1 -run TestLoad_PagerankDefaults ./internal/config/...` + `TestLoad_TypesDefaults` | ✅ exists for partial keys; **EXTEND** with new keys |

### Sampling Rate
- **Per task commit:** `go test -count=1 -run <focused-test-name> <pkg>` for the
  task's package (e.g., `go test -count=1 ./internal/graph/...` after the engine
  patch). Per CLAUDE.md: ALSO `go vet ./...`.
- **Per wave merge:** `go vet ./... && go test -count=1 ./internal/graph/...
  ./internal/repomap/... ./internal/semantic/...` — covers all packages
  Phase 62 touches.
- **Phase gate:** `go vet ./... && go test ./...` (full suite green) before
  `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/graph/pagerank.go` — engine source (D-01)
- [ ] `internal/graph/pagerank_test.go` — determinism, hex-digest, personalized,
      uniform, dangling, single-node tests (covers GRAPH-01, GRAPH-02, GRAPH-03)
- [ ] `internal/graph/testdata/pagerank/golden_*.txt` — hex digests for
      pinned input vectors
- [ ] `internal/repomap/pagerank.go` — rewritten as adapter
- [ ] `internal/repomap/pagerank_test.go` — re-pinned vectors (NOT a new file;
      planner must explicitly touch this with the migration patch)
- [ ] `internal/semantic/graph/ranker.go` — Ranker interface + impl
- [ ] `internal/semantic/graph/ranker_test.go` — `Ranker.Rank` carries
      `graph_version` test
- [ ] `internal/semantic/graph/repair.go` — `GraphRepair` type + `ComputeGraphRepair`
- [ ] `internal/semantic/graph/apply_repair.go` — `Engine.ApplyRepair` (single
      bump site)
- [ ] `internal/semantic/graph/apply_repair_test.go` — bump-count integration
- [ ] `internal/semantic/graph/scheduler.go` — RankScheduler goroutine
- [ ] `internal/semantic/graph/scheduler_test.go` — frontier overflow, debounce
      reset, full-recompute preemption tests
- [ ] `internal/semantic/graph/frontier.go` — 1-hop frontier algorithm
- [ ] `internal/semantic/graph/full_recompute.go` — full recompute path
- [ ] `internal/semantic/graph/invalidations_consumer.go` — turns rows into
      `GraphRepair`
- [ ] `internal/semantic/graph/status.go` — `score_status` read API (D-07)
- [ ] `internal/semantic/graph/metrics.go` — bounded-label metric helpers
- [ ] `internal/semantic/graph/testdata/repair/*.json` — overlay-tx fixtures
- [ ] `internal/semantic/cluster/weak.go` — weak component algorithm
- [ ] `internal/semantic/cluster/persist.go` — cluster row writes
- [ ] `internal/semantic/cluster/weak_test.go` — determinism + hex-digest
- [ ] `internal/semantic/types/resolver.go` — dispatcher
- [ ] `internal/semantic/types/chain.go` — access-chain walker
- [ ] `internal/semantic/types/fixpoint.go` — bounded fixpoint
- [ ] `internal/semantic/types/ladder.go` — SPEC §38.2 7-tier model
- [ ] `internal/semantic/types/emit.go` — edge emission + two-phase merge
- [ ] `internal/semantic/types/{golang,python,typescript}/{resolver,scope,comment}.go`
      — per-language full-ladder resolvers
- [ ] `internal/semantic/types/{php,ruby}/stub.go` — always 0.20 unresolved
- [ ] `internal/semantic/types/java/stub.go` — LSP-conditional 0.20
- [ ] `internal/semantic/types/testdata/<lang>/` — per-language fact fixtures
- [ ] `internal/semantic/store/overlay.go` — extend with `UpsertGraphScores`,
      `UpsertClusters`, `UpsertClusterMembers`, `UpsertEdgesWithMerge`,
      real `WriteInvalidations`
- [ ] `internal/semantic/store/overlay_test.go` — unit coverage for new
      methods
- [ ] `internal/config/defaults.go` — add 4 new config keys
- [ ] `internal/config/loader_test.go` — extend `TestLoad_PagerankDefaults` +
      add `TestLoad_TypesCommentParsersEnabled`
- [ ] `internal/obs/metrics.go` — register 5 new metric Vecs
- [ ] `internal/obs/metrics_labels_test.go` — carve out Phase 62 closed-enum
      labels
- [ ] `internal/daemon/daemon.go` — wire `RankScheduler` + `Resolver`

**Framework install:** none needed — Go test is stdlib; testify is already
vendored.

## Sources

### Primary (HIGH confidence)
- `internal/repomap/pagerank.go` — current ~150 LOC PageRank implementation
  (read in full for the determinism patch surface)
- `internal/repomap/graph.go` — `FileGraph` shape; Phase 62 keeps the
  public API
- `internal/repomap/pagerank_test.go` — current test vectors; planner
  re-pins (D-04)
- `internal/semantic/store/migrations.go:248-289` — `semantic_graph_scores`,
  `semantic_clusters`, `semantic_cluster_members` schema (already shipped)
- `internal/semantic/store/migrations.go:291-310` —
  `semantic_live_overlay_meta` (carries `graph_version` column)
- `internal/semantic/store/migrations.go:349-368` —
  `semantic_live_overlay_edges` shape (PK = (repo_id, edge_id), indices
  on src and dst)
- `internal/semantic/store/overlay.go:62-141` — `BeginOverlayTx` per-tx
  epoch contract + per-workspace mutex (Phase 60 D-04)
- `internal/semantic/lspenrich/cascade.go:79-88` — `CascadeTx` interface
  (the typed `WriteInvalidations(ctx)` stub seam)
- `internal/semantic/lspenrich/cascade.go:440-449` — `WriteInvalidations`
  stub call site Phase 62 fills
- `internal/semantic/live/handler/handler.go` — post-commit hook +
  `OverlayTx` interface (handler is the natural fire site for
  `ApplyRepair`)
- `internal/semantic/live/coalescer/coalescer.go` — closed-channel
  goroutine lifecycle template; `RankScheduler` mirrors
- `internal/semantic/lspenrich/manager.go:50-128` — Manager bootstrap +
  errgroup ownership; `RankScheduler` follows
- `internal/daemon/daemon.go:295-507` + `:645-695` — setter wiring
  (`SetEnrichFn`, `SetActivateCallback`) + errgroup composition
- `internal/config/defaults.go:115-159` — koanf precedence + most Phase 62
  config keys ALREADY shipped (`pagerank.damping`,
  `pagerank.epsilon`, `pagerank.max_iterations`, `clustering.*`,
  `type_resolution.max_chain_depth`, `type_resolution.max_fixpoint_
  iterations`, `graph.max_local_pagerank_nodes`)
- `internal/semantic/config.go:190-264` — `GraphConfig` + `PageRankConfig`
  + `ClusteringConfig` + `TypeResolutionConfig` Go struct shapes (already
  shipped)
- `internal/obs/metrics.go:144-330` — Prometheus Vec registration template
- `internal/lint/nokernel2semantic/` — semantic→kernel boundary analyzer
- `cmd/vet-noduckdb/` — duckdb-go import boundary analyzer
- `SPEC-DRAFT.md` §17 (lines 1624-1723) — Graph Cache + ApplyRepair
- `SPEC-DRAFT.md` §18 (lines 1725-1881) — PageRank + Personalized PR + Score Fusion
- `SPEC-DRAFT.md` §19 (lines 1883-2050) — Clustering + Weak Components
- `SPEC-DRAFT.md` §38 (lines 3887-4150) — Type Resolution + Access-Chain
  Resolver + Comment Fallbacks + Edge Emission
- `SPEC-DRAFT.md` §9.9 (lines 752-770) — `semantic_graph_scores` schema
- `SPEC-DRAFT.md` §9.10 (lines 773-798) — `semantic_clusters` /
  `semantic_cluster_members` schema
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md`
  — locked decisions D-01..D-14 + Claude's Discretion + deferred ideas
- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` — Phase 60 D-04
  per-workspace overlay mutex + `current_epoch` CAS contract
- `.planning/phases/61-lsp-enrichment-worker/61-CONTEXT.md` — Phase 61
  D-07 cascade + `WriteInvalidations` stub Phase 62 fills
- `.planning/REQUIREMENTS.md` §GRAPH (lines 56-63) + §TYPES (lines 91-96)
- `.planning/milestones/v1.10-ROADMAP.md` Phase 62 block (lines 131-141)
- `.planning/research/PITFALLS.md` C3, C4, C8, M3 — death-spiral patterns
  + ranking-engine pitfalls

### Secondary (MEDIUM confidence)
- N/A — every load-bearing claim above is verified against shipped code or
  the SPEC.

### Tertiary (LOW confidence)
- N/A.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dep is already vendored; no new third-party
  deps required by D-01/D-12.
- Architecture: HIGH — CONTEXT.md D-01..D-14 are exhaustive; every
  load-bearing structure (graph_version column, score table, overlay tx,
  cascade tx interface) verified in shipped code.
- Pitfalls: HIGH — drawn from `.planning/research/PITFALLS.md` (C3, C4, C8,
  M3) all of which are explicitly addressed in CONTEXT.md decisions.
- Validation: HIGH — every requirement maps to a concrete test command;
  test framework (Go stdlib + testify) is already in use across the
  semantic packages.

**Research date:** 2026-05-06
**Valid until:** 2026-06-05 (30 days — stable; no Go stdlib changes
expected, no upstream LSP API changes expected, all dependencies in
`go.sum` are pinned).
