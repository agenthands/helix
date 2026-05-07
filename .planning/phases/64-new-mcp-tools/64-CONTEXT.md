# Phase 64: New MCP Tools (P0 set of 4) - Context

**Gathered:** 2026-05-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 64 ships **four MCP tools** that wrap the Phase 60-63 semantic engine
(live overlay, graph engine, type resolver, compaction) so agents can index,
refresh, inspect, and query the semantic graph. Every response carries
`freshness` + `graph_version`. Profile/mode gating respects the existing
matrix. Selection is deterministic.

**Phase 64 ships:**

1. **`index_semantic_graph`** (mode `review+` / `admin`) — builds or refreshes
   a committed snapshot. Modes: `auto`, `full`, `incremental`, `refresh`.
   Returns snapshot id, graph version, files indexed/reused, partial state,
   freshness, duration. SPEC §23.1 input/output shape.

2. **`refresh_semantic_graph`** (mode `read+`) — applies pending live source
   changes without forcing a full reindex. Supports `wait_for_lsp` and
   `paths` filters. Returns graph version, files updated, deltas, pending
   LSP, freshness. SPEC §23.2 input/output shape. **Overlay-only — never
   commits a snapshot.**

3. **`get_semantic_graph_status`** (mode `read+`) — returns the SPEC §23.3
   status object: latest snapshot id, graph version, overlay state, pending
   LSP count, freshness, per-projection score status, cluster status,
   last-live-update latency.

4. **`get_semantic_context`** (mode `read+`) — ranked, evidence-backed
   context for a task/symbol/file selection under a token budget. Response
   always includes `freshness_mode`, `graph_version`, `overlay_active`,
   `freshness`, `pending_lsp_files`, and per-candidate `evidence` +
   `confidence`. SPEC §23.4 input/output shape.

5. **Profile/mode wiring** — all four tools respect SPEC §30.2 gating;
   `tools/list` filters them out for profiles that don't include them;
   `get_tool_help` returns parameter docs for each (TOOL-05).

6. **Effective-graph queries** consumed from this phase (deferred to Phase
   64 by Phase 63 CONTEXT.md): `QueryEffectiveAdjacency`,
   `CountStaleScoreRows`, `MarkAllScoreRowsStale` on `*Store` — needed by
   `get_semantic_context` to query the live overlay + committed snapshot
   union.

**Out of scope (deferred to later phases):**

- **Strangler-fig integration** of `get_repo_map`, `get_context`,
  `analyze_blast_radius`, `get_health` over committed snapshots — Phase 65.
  Phase 64 ships only the four NEW tools; existing tools keep their v1.9
  behavior until Phase 65.
- **Agent guardrails** (G-001..G-005) and `GuardrailMiddleware` — Phase 66.
  Phase 64 tools are warn-default-clean.
- **Evaluation harness** for the four tools — Phase 67. Phase 64 ships
  with unit + integration tests; eval-mode comparison comes later.
- **Tools 5-10** from SPEC §23.5-23.10 (`explain_symbol_deep`,
  `find_related_symbols`, `get_cluster_map`, `explain_cluster`,
  `get_change_impact_graph`, `validate_graph_edge`) — deferred to v1.10.x
  per REQUIREMENTS.md line 73.
- **MCP push notifications** for graph state changes — no consumer today;
  deferred until an evident need.
- **Multi-projection PageRank** — deferred per REQUIREMENTS.md line 73.
  Phase 64 surfaces only the projections Phase 62 ships.

</domain>

<decisions>
## Implementation Decisions

### `index_semantic_graph` dispatch model

- **D-01: Sync-with-timeout dispatch.** The tool blocks in-process up to
  the request's `max_duration_ms` (default 120000ms, capped by the
  TelemetryMiddleware `BudgetFunc` per-tool deadline). On timeout, return
  `partial=true` with `files_indexed` and `files_reused` counts so far,
  the in-progress `snapshot_id`, `freshness=stale`, `duration_ms=elapsed`,
  and `status=building`. The build keeps running in the background under
  the single-flight lease; commits when done. Agents that care about
  completion poll `get_semantic_graph_status`. Matches SPEC §23.1 input
  shape (single round-trip for the common case where indexing finishes
  in <2s) and reuses TelemetryMiddleware budget semantics already in
  place.

- **D-02: Single-flight join for concurrent calls.** Use
  `golang.org/x/sync/singleflight.Group` keyed by `(workspace, mode)`. A
  second agent calling `index_semantic_graph` while the first is still
  building **attaches** to the in-flight build; both return the same
  `snapshot_id` when the build commits (or both time out together). No
  redundant builds, no rejected calls. Pattern matches Phase 61
  LeaseAcquirer + Phase 62 RankScheduler drop-on-full doctrine.

- **D-03: `mode=auto` resolution rule.** `auto = (full if no committed
  snapshot exists) else incremental`. The dispatcher reads `semantic_meta`
  to check for a latest committed snapshot; if absent, route to the SPEC
  §15.1-15.3 full pipeline. If present, route to the incremental pipeline.
  Matches lazy-init doctrine (`semantic_index.indexing.mode=lazy` from
  SPEC §25). Avoids surprise multi-minute work when an existing snapshot
  is just slightly stale.

- **D-04: Timeout response includes background-continuation signal.** On
  `partial=true` timeout, the response carries `status=building` and the
  daemon continues the build under the existing `singleflight.Group`
  lease. The build's eventual commit is observable via
  `get_semantic_graph_status` (snapshot id flips from "building" to
  "committed"). Cancellation only happens on workspace deactivation or
  daemon shutdown — not on tool-call timeout. Argument: a 60% complete
  index becoming 0% on timeout wastes work and forces the agent into a
  retry loop.

### `get_semantic_context` retrieval strategy

- **D-05: Hybrid retrieval = bleve + Phase 62 PageRank.** The `task`
  string drives a bleve full-text query against an indexed corpus of
  symbol facts; results fuse with personalized PageRank scores from the
  Phase 62 graph engine for the final ranking. Anchors (`files`,
  `symbols`) seed the personalized PageRank surface and bias bleve
  scoring (so a `task` with anchors retrieves task-relevant code in the
  anchored neighborhood).

- **D-06: Indexed text fields = name + docstring + path + nearby comment
  window.** For each symbol fact, bleve indexes (camelCase/snake_case
  tokenized symbol name, full docstring/JSDoc/godoc comment, tokenized
  file path components, ~5-line comment window above/below the
  declaration). Catches natural-language `task` strings whose terms live
  in identifiers, paths, OR inline comments. Index size grows ~3-4x over
  name-only but recall justifies it.

- **D-07: Score fusion = weighted RRF (defaults equal, weights
  Go-internal).** Underlying algorithm is weighted Reciprocal Rank
  Fusion: `score = w_text/(K + rank_text) + w_graph/(K + rank_graph)`.
  Defaults: `K=60, w_text=1.0, w_graph=1.0` (i.e., classic unweighted
  RRF). Weights live as Go-internal constants in an `RRFConfig` struct;
  no `semantic_index.*` config keys exposed this phase. Phase 67
  evaluation harness is the natural place to gather signal on whether
  weights should become tunable. Implementing the weighted formula from
  day one keeps the surface flexible without creating a knob agents will
  prematurely tune.

- **D-08: Researcher gates on bleve before planner locks.** Phase 64's
  researcher must validate before the planner commits to bleve:
  - bleve version (likely v2.x with scorch backend)
  - scorch vs upside-down backend choice (scorch is the modern recommendation)
  - dependency size + transitive footprint (must not balloon binary)
  - license compliance (Apache-2.0 should be fine)
  - indexing throughput on a 50k-symbol fixture vs DuckDB FTS baseline
  - dual-store crash-recovery story: the snapshot in `semantic.duckdb`
    and the bleve segment in `.helix/semantic.bleve/` are NOT
    transactionally bound. On daemon restart, if the bleve index is
    missing or older than the latest committed snapshot, the daemon
    must rebuild the bleve segment from the snapshot before serving
    `get_semantic_context`. The recovery procedure must be specified.

  If research surfaces a hard blocker (e.g., dependency >50MB binary
  growth, indexing >5x slower than DuckDB FTS, or a license issue), the
  researcher flags the conditional fallback to DuckDB FTS and the
  planner reroutes.

### `refresh` vs `index --mode=incremental` boundary

- **D-09: `refresh_semantic_graph` is overlay-only.** read+ tier; drains
  the live queue into the overlay, optionally waits for LSP
  revalidation, bumps `graph_version` via Phase 62 ApplyRepair, returns
  deltas. **NEVER commits a snapshot. NEVER triggers Phase 63
  compaction.** The mode-gating already enforces this — refresh is read+
  and committing snapshots is a privileged operation. Cleanest
  interpretation of SPEC §23.1 vs §23.2.

- **D-10: `index_semantic_graph --mode=incremental` owns snapshot
  commits.** review+/admin tier; drains the live queue + writes a new
  committed snapshot atop the previous via the Phase 63 snapshot-write
  API (`BeginSnapshot` → `WriteSnapshotFacts` → `CommitSnapshot`). The
  resulting snapshot is the new compaction baseline. Phase 63's
  per-workspace compactor goroutine is **not** invoked by this path —
  index produces a snapshot directly; the compactor's idle-debounced
  trigger continues to fire independently per its own gate.

- **D-11: `paths` filter on refresh = strict subset.** When `paths` is
  provided, refresh processes ONLY those paths from the live queue
  (or triggers fresh re-classification for paths not yet queued); leaves
  other queued changes pending. The remaining queue gets drained on the
  next refresh call or by the natural Phase 60 coalescer flush.
  `files_updated` in the response counts only the requested paths. Most
  predictable contract: agent asks about file X, gets exactly file X.

- **D-12: `wait_for_lsp:true` blocks up to `max_wait_ms`.** Default
  `max_wait_ms=3000` (matches SPEC §23.2 input shape). Refresh blocks on
  Phase 61 LSPQueue draining for the requested paths up to
  `max_wait_ms`. On timeout, return `pending_lsp:true` with the count of
  files still pending. Freshness reflects what completed: `fresh` if all
  requested paths LSP-validated, `structurally_fresh_semantically_pending`
  if structural ok but LSP timed out.

- **D-13: Refresh never short-circuits the Phase 63 compactor.** Phase 63
  owns compaction trigger ownership via the per-workspace compactor
  goroutine, fired by coalescer-flush + idle-debounce +
  `CompactionGate.IsReady`. Refresh does NOT call `compactor.OnFlush()`,
  does NOT inline a compaction tx, and does NOT force-bump the gate's
  idle timer. Read+ stays read-only with respect to committed state.

### Default profile visibility

- **D-14: All five profiles get all four tools by default.** Symmetric
  matrix — `claude-code`, `codex`, `ide-assistant`, `ci-bot`, `full` all
  see `index_semantic_graph`, `refresh_semantic_graph`,
  `get_semantic_graph_status`, `get_semantic_context` in their
  `tools/list` responses. SPEC §30.2 mode-gating handles privilege
  inside the call: a profile in `read` mode still cannot call
  `index_semantic_graph` (mode `review+`/`admin`) — the tool returns a
  mode-violation error envelope. Simpler matrix, no per-profile
  reasoning, mode-gating is the right axis for restriction.

### Acceptance criteria (must hold at end of phase)

1. **TOOL-01 closed:** `index_semantic_graph` registered with profile
   filtering; `auto`, `full`, `incremental`, `refresh` modes all dispatch
   correctly; sync-with-timeout returns the SPEC §23.1 envelope; on
   timeout, response carries `partial=true, status=building` and
   background build commits. Single-flight join asserted by an
   integration test that fires two concurrent `mode=full` calls and
   verifies both receive the same `snapshot_id`.
2. **TOOL-02 closed:** `refresh_semantic_graph` registered with read+
   gating; never produces a new committed snapshot (assert via
   `latest_snapshot_id` invariant test); `paths` filter is strict
   subset; `wait_for_lsp:true` blocks up to `max_wait_ms` and reflects
   accurate `freshness` field.
3. **TOOL-03 closed:** `get_semantic_graph_status` returns the SPEC
   §23.3 envelope under both empty-store and populated-store states;
   `freshness` enum values match SPEC §26.2; `score_status` per
   projection consumed from Phase 62 RankScheduler.
4. **TOOL-04 closed:** `get_semantic_context` returns ranked,
   evidence-backed context under a token budget; bleve+RRF ranking is
   deterministic (asserted by an integration test that runs the same
   query twice and asserts identical result order); `freshness_mode`
   enum (`allow_stale | require_current | validate_live`) honored.
5. **TOOL-05 closed:** all four tools filtered correctly by
   `ProfileFilterMiddleware` for each of the five profiles;
   `get_tool_help` returns parameter docs for each (verified by a
   protocol-level test that calls `get_tool_help` per tool).
6. **Researcher report on bleve dependency:** `64-RESEARCH.md` includes
   bleve version + backend + dependency size + recovery procedure
   verdict before planner emits a PLAN.md. If verdict flips to
   "DuckDB FTS fallback," planner reroutes accordingly.
7. **Mode-violation envelope:** integration test calls
   `index_semantic_graph` from a profile in `read` mode; tool returns
   structured mode-violation error (not crash); error envelope
   compatible with the existing SuggestionMiddleware error shape.
8. **Determinism harness:** a small bench/test that runs the same
   `get_semantic_context` query 10x and asserts byte-identical result
   order (catches non-deterministic map iteration, score-tie ordering
   bugs).

### Claude's Discretion (no user input needed)

- **Package layout:** `internal/skill/semantic/` for the four skill
  packages (one per tool, or one shared package — planner's call), each
  registering via `init()` per the Caddy-style pattern. Daemon imports
  via blank import in `internal/daemon/imports.go`. If any tool needs
  kernel state, it gets a `skill_adapter.go` bridge in
  `internal/kernel/...` like `internal/kernel/health/` and
  `internal/kernel/help/` already do.
- **Stable-key tiebreak ordering** for deterministic selection: score
  → graph_version → symbol_id (matches Phase 62 sort-before-iterate
  doctrine). Planner may refine if a profile-tier-specific tiebreak
  becomes warranted.
- **Effective-graph query implementation:** `QueryEffectiveAdjacency`,
  `CountStaleScoreRows`, `MarkAllScoreRowsStale` land on `*Store` in
  `internal/semantic/store/`; signatures and bodies are planner's call
  as long as they consume the Phase 60 D-04 CAS contract.
- **`evidence` field shape per candidate:** at minimum, source ranks
  (text rank, graph rank), matched bleve terms (deduped), and top
  contributing graph edges (capped at ~5 to keep envelope size sane).
  Planner refines based on token-budget pressure.
- **Token-budget packing:** greedy-by-fused-score under
  `max_tokens` is the simplest correct shape; if testing reveals
  pathological underfill, planner can switch to RepoMap-style binary
  search over an elided candidate tree. Default greedy.
- **bleve recovery procedure exact mechanism:** detect via comparing
  bleve segment metadata vs `semantic_meta.latest_snapshot_id` at
  workspace activation; if mismatched, rebuild bleve from the latest
  snapshot in a goroutine and gate `get_semantic_context` on
  rebuild completion (block with structured "rebuilding" envelope or
  return stale results — planner picks; recommend: block with
  `freshness=stale, retrieval_pending=true`).
- **`status=building` field name** in the partial-timeout response of
  `index_semantic_graph`: planner may choose any name as long as
  agents can disambiguate from a successful commit. Recommended:
  add a top-level `status` field with closed enum `committed |
  building | failed`.
- **Bench fixture for determinism:** synthetic 50k-symbol Go
  workspace with a fixed seed; the same fixture used by Phase 64's
  determinism harness can be reused by Phase 67's evaluation
  harness later.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` (lines 67-71) — TOOL-01 through TOOL-05
  phrasing is the contract. Each requirement maps to an acceptance
  test (D-acceptance #1-#8 above).
- `.planning/ROADMAP.md` (lines 176-184) — parent ROADMAP Phase 64
  block (Goal, Depends on, Requirements, 4 Success Criteria; expanded
  this phase from the milestone detail file).
- `.planning/milestones/v1.10-ROADMAP.md` (lines 154-163) — milestone
  detail file (same content as the parent ROADMAP entry; left in
  place as the milestone-level reference).

### Specification (source of truth for shapes and rules)

- `SPEC-DRAFT.md` §15 (Indexing Pipeline) — §15.1-15.3 specifies the
  full/incremental snapshot pipeline that `index_semantic_graph`
  routes to.
- `SPEC-DRAFT.md` §17 (Graph Cache and Repair) — `refresh_semantic_graph`
  consumes the repair rules and the `ApplyRepair` post-commit hook
  Phase 62 wired.
- `SPEC-DRAFT.md` §18 (PageRank and Ranking) — Phase 62 ships these;
  `get_semantic_context` consumes the persisted scores.
- `SPEC-DRAFT.md` §22 (Overlay Compaction) — boundary with
  `index_semantic_graph --mode=incremental`. Phase 63 owns
  compaction trigger; Phase 64 routes around it.
- `SPEC-DRAFT.md` §23.1 (`index_semantic_graph`) — input/output shape.
- `SPEC-DRAFT.md` §23.2 (`refresh_semantic_graph`) — input/output
  shape; `wait_for_lsp` + `paths` semantics.
- `SPEC-DRAFT.md` §23.3 (`get_semantic_graph_status`) — output object
  shape (overlay state, score_status, cluster_status, last live
  update latency).
- `SPEC-DRAFT.md` §23.4 (`get_semantic_context`) — input/output
  shape; required envelope fields (`freshness_mode`, `graph_version`,
  `overlay_active`, `freshness`, `pending_lsp_files`, per-candidate
  `evidence` + `confidence`).
- `SPEC-DRAFT.md` §25 (Configuration) — `semantic_index.*` keys; Phase
  64 adds NO new config keys (RRF weights stay Go-internal).
- `SPEC-DRAFT.md` §26.2 (Stale and Pending States) — closed enum for
  the `freshness` field across all four tools.
- `SPEC-DRAFT.md` §30.2 (MCP Mode Gating) — exact mode tier per tool.

### Phase 60 / 62 / 63 lock-down (must not regress)

- `.planning/phases/60-live-update-pipeline/60-CONTEXT.md` — D-04
  overlay_epoch CAS contract; Phase 64's effective-graph queries
  consume this. Live overlay tables (§9.11) are the read source.
- `.planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md`
  — RankScheduler + ApplyRepair + graph_version single-bump.
  `refresh_semantic_graph` consumes ApplyRepair; `get_semantic_context`
  consumes persisted PageRank scores.
- `.planning/phases/63-compaction-retention/63-CONTEXT.md` — snapshot-
  write API, `CompactionGate.IsReady`, per-workspace compactor.
  `index_semantic_graph --mode=incremental` calls into the snapshot-
  write API (D-01 in 63-CONTEXT.md). Phase 63 explicitly defers MCP
  tool wrappers and effective-graph queries to Phase 64.
- `.planning/phases/61-lsp-enrichment-worker/61-CONTEXT.md` — LSPQueue
  + LeaseAcquirer + 2-lane queue. `refresh_semantic_graph`'s
  `wait_for_lsp` blocks on Phase 61's queue draining via
  `Depth(ws)` accessor (added in Phase 63).

### Architectural invariants

- `CLAUDE.md` "Middleware Execution Order (LIFO)" — Phase 64 does NOT
  add or reorder middleware. Profile/mode gating goes through the
  existing `ProfileFilterMiddleware` (step 14) and is enforced inside
  the tool handler for mode-tier checks (review+/admin enforcement
  on `index_semantic_graph` happens inside the tool, not in
  middleware).
- `CLAUDE.md` "Tool Registration" — kernel tools use
  `RegisterTools(server *mcp.SerenaMCPServer, ...)`; skill tools use
  `ToolProvider.Tools()` returning `[]*mcp.ToolDef`. Phase 64 ships
  as skill tools registered via Caddy-style `init()` per the
  precedent of `internal/skill/repomap/`, `internal/skill/memory/`.
- `internal/daemon/daemon.go` — Phase 64 wiring lands after Phase 63's
  compactor wiring; new bootstrap calls register the four skill
  packages via blank imports in `internal/daemon/imports.go`.

### Pattern templates (must mirror)

- `internal/skill/repomap/` — skill-package pattern with `ToolProvider`
  + `init()` registration; serves as template for the four new skill
  packages.
- `internal/kernel/health/skill_adapter.go` and
  `internal/kernel/help/skill_adapter.go` — kernel-resident tool
  wrapped as a skill adapter when the tool needs kernel state. If
  any of Phase 64's four tools need kernel-private state (e.g.,
  workspace activation status for status reporting), they get a
  similar adapter.
- `internal/mcp/middleware.go` — `ProfileFilterMiddleware` already
  filters `tools/list` by active profile; Phase 64 wires the four
  tools into the per-profile tool subset YAML at
  `internal/profile/profiles/*.yml` (or wherever the profile
  definitions live).

### External research targets (not yet read; researcher's job)

- **`github.com/blevesearch/bleve`** — pure-Go FTS library. Researcher
  validates: version (v2.x scorch backend recommended); dependency
  size; license (Apache-2.0); indexing throughput on a 50k-symbol
  fixture vs DuckDB FTS baseline; dual-store crash-recovery
  procedure (rebuild bleve segment from latest snapshot on daemon
  start when mismatched).
- **`golang.org/x/sync/singleflight`** — for D-02 concurrent-call
  coordination. Already a Go subrepository; minimal research needed,
  but planner verifies the call shape matches what
  `index_semantic_graph` needs (key by `(workspace, mode)`, return
  shared result + error).

### Validation tooling

- `cmd/vet-noduckdb/` (Phase 57) — Phase 64 skill packages MUST NOT
  import `duckdb-go` directly; they go through `internal/semantic/
  store/`. Vet analyzer enforces.
- `cmd/vet-nokernel2semantic` (Phase 59 D-02) — kernel does not
  import internal/semantic. Phase 64 skill packages don't violate
  this; if a kernel adapter is added, it lives in `internal/kernel/`
  and depends on the skill via interface, not the other way around.
- (potential new) `cmd/vet-semantic-mcp/` — if planner judges
  warranted, a small additional analyzer can pin the
  `internal/skill/semantic/` → `internal/semantic/store/` boundary
  (no leaks back into `internal/kernel` from skill-resident
  semantic tool code).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **Phase 60 live overlay** (`internal/semantic/store/overlay.go`,
  `internal/semantic/live/`) — `refresh_semantic_graph` drains the live
  queue via existing classifier + coalescer. Effective-graph queries
  (`QueryEffectiveAdjacency`, etc.) extend the existing `*Store`
  surface.
- **Phase 61 LSPQueue** (`internal/semantic/lspqueue/` or wherever it
  lands) — `Depth(ws)` and `LastEnqueueAt(ws)` accessors exist (added
  in Phase 63). `refresh_semantic_graph`'s `wait_for_lsp` blocks on
  queue depth.
- **Phase 62 RankScheduler** (`internal/semantic/rank/scheduler.go`)
  — already has `IsQuiescent(ws)`. `get_semantic_graph_status`
  surfaces score_status per projection consumed from this.
- **Phase 62 graph engine** (`internal/graph/`) — PageRank + clustering
  + persisted scores. `get_semantic_context` reads persisted scores
  via the existing read API; no recomputation in the hot path.
- **Phase 62 ApplyRepair post-commit hook** — `refresh_semantic_graph`
  fires after overlay updates; the hook bumps `graph_version`. Phase
  64 reads the bumped version into the response envelope.
- **Phase 63 snapshot-write API**
  (`internal/semantic/store/snapshot.go`) — `index_semantic_graph
  --mode=incremental` calls `BeginSnapshot` → `WriteSnapshotFacts` →
  `CommitSnapshot` to produce a new committed snapshot.
- **`ProfileFilterMiddleware` and `BriefDescriptions()`** in
  `internal/mcp/middleware.go` — already filters `tools/list` by
  active profile. Phase 64's profile/mode wiring leverages this
  unchanged.
- **`get_tool_help` tool** at `internal/kernel/help/` — already serves
  tool documentation. Phase 64 adds the four tools to its registry
  surface so `get_tool_help` works for them out of the box (TOOL-05).
- **`BudgetFunc` in `TelemetryMiddleware`** — per-tool deadline injection.
  `index_semantic_graph`'s `max_duration_ms` is bounded by this.
- **`internal/mcp/lazy_init.go`** — workspace activation on first tool
  call. Phase 64 tools rely on this; no special activation path
  needed.

### Established Patterns

- **Caddy-style `init()` registration** — skill packages register via
  `skill.Register(&Skill{})`; daemon imports blank in
  `internal/daemon/imports.go`. Phase 64 follows this for the four
  new skill packages.
- **`ToolProvider.Tools()` returning `[]*mcp.ToolDef`** — Phase 64
  skills implement this interface; daemon registers tools centrally
  via the kernel registration pipeline.
- **Constructor injection for shared dependencies** (Phase 59 D-02) —
  Phase 64 skills receive their store/scheduler/queue dependencies
  via constructor; no global state, no init-side service location.
- **Bounded-label metrics with closed enum** (Phase 57 D-07, Phase
  62 truth #21) — Phase 64 emits e.g.
  `helix_mcp_tool_duration_seconds{tool, outcome}` with closed-enum
  outcome (success | timeout | mode_violation | internal). Reuses
  existing TelemetryMiddleware infrastructure.
- **Sort-before-iterate doctrine** (Phase 62 CR-03) — deterministic
  result order in `get_semantic_context` ranking iteration; the
  determinism harness asserts this.
- **Per-workspace ownership** (Phase 60 D-02 / D-05) — bleve segment
  per workspace; `internal/skill/semantic/index_runner` (or wherever
  the indexing runner lives) tracks per-workspace `singleflight.Group`
  for concurrent index calls.

### Integration Points

- **`internal/daemon/daemon.go`** — Phase 64 adds (in order):
  - Construct bleve index handles per workspace (lazy on
    activation); register them via `SetActivateCallback` so a fresh
    workspace gets a fresh bleve segment.
  - Register the four skill packages via blank imports
    (`internal/daemon/imports.go`).
  - Register `helix_mcp_tool_duration_seconds` (or extend the
    existing tool-duration metric with the four new tool labels) via
    `internal/obs/`.
- **`ProfileFilterMiddleware`** — the four new tools are added to the
  per-profile tool subset (`internal/profile/profiles/*.yml`); D-14
  decides the subset = all four tools for all five profiles.
- **`get_tool_help` registry** — the four tools register their help
  metadata so `get_tool_help` returns parameter docs (TOOL-05
  acceptance).
- **Phase 62 RankScheduler `IsQuiescent`** — `get_semantic_graph_status`
  surfaces this through the response envelope.
- **Phase 63 `CompactionGate.Status()`** — `get_semantic_graph_status`
  surfaces `last_live_update_ms` from the gate's accessors. Note:
  Phase 63 ships only the accessor; Phase 64 wires it into the MCP
  response.

### Constraints

- Phase 64 must NOT touch middleware order (`CLAUDE.md` "Middleware
  Execution Order (LIFO)").
- Phase 64 must NOT import `duckdb-go` outside
  `internal/semantic/store/` (Phase 57 D-12 + `cmd/vet-noduckdb/`).
- Phase 64 must NOT add any new `semantic_index.*` config keys for
  RRF weights (D-07).
- Phase 64 must NOT call into Phase 63 compactor from refresh
  (D-13). Read+ stays read-only with respect to committed state.
- Phase 64 must NOT regress `tools/list` profile filtering — the
  five existing profiles' current tool surface is the baseline; the
  four new tools are additions.
- Phase 64 must NOT enforce mode tier in middleware — mode-tier
  enforcement happens inside each tool handler so the error
  envelope can include actionable metadata (current mode, required
  mode, hint to elevate).

</code_context>

<specifics>
## Specific Ideas

- **Sync-with-timeout matches the existing TelemetryMiddleware
  contract.** `BudgetFunc` already injects per-tool deadlines; reusing
  it for `max_duration_ms` keeps the timeout story consistent across
  every MCP tool, not just the four new ones.

- **Single-flight join is the universal pattern in this codebase.**
  Phase 61 LeaseAcquirer + Phase 62 RankScheduler drop-on-full all
  follow the same "concurrent callers don't redo work" doctrine.
  `golang.org/x/sync/singleflight.Group` keyed by `(workspace, mode)`
  is the natural Go idiom for it.

- **Bleve over DuckDB FTS optimizes for ranking quality and pure-Go
  determinism, not minimum dependencies.** The user picked bleve
  knowing this — pure-Go BM25 with deterministic sort-key tiebreak,
  on-disk persistence (no rebuild on daemon restart in the happy
  path), and a richer query language than DuckDB FTS or SQLite FTS5.
  The dual-store reconciliation cost is accepted as a tradeoff.

- **Weighted RRF with equal defaults is the "ship simple, leave room"
  shape.** Implementing the weighted formula from day one means Phase
  67 eval data can flip a constant without breaking the tool API.
  Exposing weights as config now would be premature — agents and
  users would tune blindly without comparison data.

- **`refresh` overlay-only / `index --mode=incremental` snapshot-
  commit boundary mirrors the read+/review+ tier split exactly.**
  The mode gating in SPEC §30.2 isn't arbitrary — read+ tools must
  not mutate committed state, and the cleanest way to honor that is
  for refresh to live entirely in overlay-land. Anyone who wants a
  new committed snapshot calls index.

- **All five profiles getting all four tools is the lowest-friction
  default.** SPEC §30.2 mode gating provides the necessary safety
  net; per-profile tool-set carving would be a downstream
  optimization (e.g., Phase 67 eval might show ci-bot never
  benefits from `get_semantic_context` and we could trim later).

</specifics>

<deferred>
## Deferred Ideas

- **Tools 5-10 from SPEC §23.5-23.10** — `explain_symbol_deep`,
  `find_related_symbols`, `get_cluster_map`, `explain_cluster`,
  `get_change_impact_graph`, `validate_graph_edge`. Deferred to
  v1.10.x per REQUIREMENTS.md line 73.
- **Multi-projection PageRank surface in `get_semantic_context`.**
  Phase 64 surfaces only the projections Phase 62 ships
  (CALL_GRAPH_PAGERANK, REFERENCE_PAGERANK,
  FILE_DEPENDENCY_PAGERANK). Multi-projection blending in the
  ranking path is a future enhancement.
- **RRF weight tuning as exposed config keys.** Phase 67 eval
  harness is the natural place to gather signal. If tuning becomes
  warranted, add `semantic_index.retrieval.rrf_w_text` and
  `semantic_index.retrieval.rrf_w_graph` to `internal/config/
  defaults.go` then.
- **MCP push notifications for graph state changes.** No consumer
  today. Could surface via Phase 64+ if an evident need (e.g.,
  long-running indexing UI in IDE) emerges.
- **Per-profile tool subset narrowing.** Today (D-14): all five
  profiles get all four tools. If Phase 67 eval shows ci-bot
  never benefits from some tools, narrow per-profile then.
- **Hybrid-mode flip to DuckDB FTS as a runtime fallback.** Phase
  64 commits to bleve (or fallback at researcher's recommendation).
  A runtime flip via config doesn't ship — the tradeoff is
  binary-time decision, not per-deployment.
- **`paths`-filter inclusive mode** for refresh. Phase 64 ships
  strict-subset (D-11); if real workloads show inclusive behavior
  is more useful, add a `drain_remainder:true` request flag in a
  future phase.
- **`get_semantic_context` evidence shape v2.** Phase 64 ships
  ranks + matched terms + top edges (capped). If agent feedback
  reveals other evidence (e.g., per-candidate trace ID for
  follow-up `explain_symbol_deep` queries), add fields then —
  the envelope is forward-compatible.
- **Phase 65 strangler-fig integration.** `get_repo_map`,
  `get_context`, `analyze_blast_radius`, `get_health` consulting
  the semantic index when available — explicit Phase 65 scope,
  not Phase 64.
- **Phase 66 guardrails.** `GuardrailMiddleware` at daemon step
  14b.5 (LIFO order preserves LazyInit-last); explicit Phase 66
  scope.
- **Phase 67 evaluation harness.** Out-of-process subprocess
  daemons, 4 modes (`baseline / native / semantic /
  semantic_guarded`); explicit Phase 67 scope. Phase 64 ships
  with unit + integration tests but no eval-mode comparison.
- **Audit / redesign of profile semantics for v1.11.** The user's
  "WTF is ci-bot" question surfaced ambiguity in profile/client
  separation. Worth a focused review in a future milestone.

### Reviewed Todos (not folded)

None — no todos matched Phase 64 scope per the cross-reference step
(`gsd-sdk query todo.match-phase 64` returned `todo_count: 0`).

</deferred>

---

*Phase: 64-new-mcp-tools*
*Context gathered: 2026-05-07*
