# Roadmap: Helix

## Milestones

- [x] **v1.0 MVP** -- Phases 1-5 (shipped 2026-04-08)
- [x] **v1.1 Integration Testing** -- Phases 6-8 (shipped 2026-04-09)
- [x] **v1.2 Performance & Production Hardening** -- Phases 9-15 (shipped 2026-04-10)
- [x] **v1.3 Documentation Catchup** -- Phases 16-17 (shipped 2026-04-11)
- [x] **v1.4 Integration Testing v2** -- Phases 18-21 (shipped 2026-04-14)
- [x] **v1.5 Typed Errors & Hardening** -- Phases 22-24 (shipped 2026-04-15)
- [x] **v1.6 Context Intelligence & Resilient Editing** -- Phases 25-33 (shipped 2026-04-20)
- [x] **v1.7 Developer Experience & Auto-Setup** -- Phases 34-38 (shipped 2026-04-22)
- [x] **v1.8 Documentation Overhaul** -- Phases 39-45 (shipped 2026-04-24)
- [x] **v1.9 Polish & Infra** -- Phases 46-56 (shipped 2026-05-03)
- [x] **v1.10 Live Semantic Index** -- Phases 57-67 (shipped 2026-05-12)
- [ ] **v1.11 Semantic Index Completion & P1 MCP Tools** -- Phases 68-73 (planning)

## Phases

<details>
<summary>v1.0 MVP (Phases 1-5) -- SHIPPED 2026-04-08</summary>

- [x] Phase 1: Foundation (3/3 plans) -- Daemon, MCP runtime, gRPC forwarder, HTTP transport
- [x] Phase 2: Code Intelligence Kernel (6/6 plans) -- LSP codegen, worker pool, 9 symbol + 6 edit + 6 file + 3 diag tools
- [x] Phase 3: Multi-Language and Skills (5/5 plans) -- 52 languages, memory system, skill interface
- [x] Phase 4: Agent Profiles and Configuration (3/3 plans) -- 5 profiles, 4 modes, token budget
- [x] Phase 5: Daemon Bootstrap Integration (3/3 plans) -- Full daemon wiring, 38+ callable MCP tools

**Full details:** `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>v1.1 Integration Testing (Phases 6-8) -- SHIPPED 2026-04-09</summary>

- [x] Phase 6: Test Harness + Go Dogfooding (4/4 plans)
- [x] Phase 7: Symbol Editing + Multi-Language Fixtures (3/3 plans)
- [x] Phase 8: Advanced Testing (4/4 plans)

**Full details:** `.planning/milestones/v1.1-ROADMAP.md`

</details>

<details>
<summary>v1.2 Performance & Production Hardening (Phases 9-15) -- SHIPPED 2026-04-10</summary>

- [x] Phase 9-15: Benchmarks, observability, metrics, tracing, degradation, docs, gate hardening

**Full details:** `.planning/milestones/v1.2-ROADMAP.md`

</details>

<details>
<summary>v1.3 Documentation Catchup (Phases 16-17) -- SHIPPED 2026-04-11</summary>

- [x] Phase 16: Core Documentation Update (2/2 plans)
- [x] Phase 17: Usage & Install Guides (2/2 plans)

**Full details:** `.planning/milestones/v1.3-ROADMAP.md`

</details>

<details>
<summary>v1.4 Integration Testing v2 (Phases 18-21) -- SHIPPED 2026-04-14</summary>

- [x] Phase 18-21: Harness extraction, protocol/contract oracles, scenarios, LLM behavioral

**Full details:** `.planning/milestones/v1.4-ROADMAP.md`

</details>

<details>
<summary>v1.5 Typed Errors & Hardening (Phases 22-24) -- SHIPPED 2026-04-15</summary>

- [x] Phase 22-24: Error taxonomy, tool migration, validation & testing

**Full details:** `.planning/milestones/v1.5-ROADMAP.md`

</details>

<details>
<summary>v1.6 Context Intelligence & Resilient Editing (Phases 25-33) -- SHIPPED 2026-04-20</summary>

- [x] Phase 25-33: Fuzzy editing, RepoMap, grammar expansion, verification, cache persistence

**Full details:** `.planning/milestones/v1.6-ROADMAP.md`

</details>

<details>
<summary>v1.7 Developer Experience & Auto-Setup (Phases 34-38) -- SHIPPED 2026-04-22</summary>

- [x] Phase 34: Setup CLI Foundation (2/2 plans)
- [x] Phase 35: Health & Status (2/2 plans)
- [x] Phase 36: Client Hooks (2/2 plans)
- [x] Phase 37: Smart Error Responses (2/2 plans)
- [x] Phase 38: Progressive Descriptions & Lazy Init (3/3 plans)

**Full details:** `.planning/milestones/v1.7-ROADMAP.md`

</details>

<details>
<summary>v1.8 Documentation Overhaul (Phases 39-45) -- SHIPPED 2026-04-24</summary>

- [x] Phase 39-45: README/USAGE/INSTALL/CONTRIBUTING/CHANGELOG/CLAUDE rewrite, cross-doc truth sync, gap closure

**Full details:** `.planning/milestones/v1.8-ROADMAP.md`

</details>

<details>
<summary>v1.9 Polish & Infra (Phases 46-56) -- SHIPPED 2026-05-03</summary>

- [x] Phase 46: bug-repomap-lua-fixture (3/3 plans)
- [x] Phase 47: bug-rust-analyzer-rename (3/3 plans)
- [x] Phase 48: bug-jdtls-warm-cache (5/5 plans)
- [x] Phase 49: bug-grammar-registry-consolidation (1/1 plans)
- [x] Phase 50: toolchain-go1.25-bench-local (4/4 plans)
- [x] Phase 51: packaging-goreleaser (6/6 plans)
- [x] Phase 51.1: cgo-treesitter-gate (1/1 plans, emergent)
- [x] Phase 52: packaging-distribution-channels — `serena → helix` rename + in-binary self-upgrade (6/6 plans)
- [x] Phase 53: obs-metrics-gaps (6/6 plans)
- [x] Phase 54: obs-dashboards-runbooks (5/5 plans)
- [x] Phase 55: obs-trace-coverage-audit (7/7 plans)
- [x] Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness (4/4 plans, emergent)

**Full details:** `.planning/milestones/v1.9-ROADMAP.md`

</details>

<details>
<summary>v1.10 Live Semantic Index (Phases 57-67) -- SHIPPED 2026-05-12</summary>

- [x] Phase 57: semantic-store-foundation-pipeline-dag-library (5/5 plans)
- [x] Phase 58: v1.9-carryover-release-distribution (4/4 plans)
- [x] Phase 59: tree-sitter-extraction-stable-symbol-ids (7/7 plans)
- [x] Phase 59.1: drop-cgo-0-single-mode-cgo-1-build-release (6/6 plans, emergent)
- [x] Phase 60: live-update-pipeline (6/6 plans)
- [x] Phase 61: lsp-enrichment-worker (5/5 plans)
- [x] Phase 62: graph-engine-ranking-type-resolution (9/9 plans)
- [x] Phase 63: compaction-retention (2/2 plans)
- [x] Phase 64: new-mcp-tools (8/8 plans)
- [x] Phase 65: existing-tool-integration-strangler-fig (13/13 plans)
- [x] Phase 66: agent-guardrails (5/5 plans)
- [x] Phase 67: evaluation-harness (8/8 plans + W1/W2/W3 + F-07 close-out)

**Full details:** `.planning/milestones/v1.10-ROADMAP.md`

</details>

### v1.11 Semantic Index Completion & P1 MCP Tools (Planning) -- Phases 68-73

- [x] **Phase 68: Precise FileFactDiff Populator** -- Replace Tier-3 synthetic-marker floor with Tier-1 (full diff) / Tier-2 (added-only) populators; pre-edit FileFact accessor; close DEF-67-F01-FULL-DIFF (completed 2026-05-13)
- [ ] **Phase 69: Production Status Accessors** -- Real ClusterStatus from Phase 62 cluster engine; real retrieval status from bleve + corpus; close TOOL-03/04 placeholders
- [ ] **Phase 70: Incremental Refresh Overlay-Drain** -- Wire `collectCandidatePaths` through overlay-drain seam so `refresh_semantic_graph mode:"incremental"` is truly incremental
- [ ] **Phase 71: P1 Single-Symbol Read Tools** -- `explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge` (all read+ tier, single-symbol seed)
- [ ] **Phase 72: P1 Cluster & Impact Tools** -- `get_cluster_map`, `explain_cluster`, `get_change_impact_graph` (cluster-aware; mixed read+/review+ tier)
- [ ] **Phase 73: P1 Tools Integration & E2E Verification** -- Profile/mode gating across the 6 tools; skill-wrapper consistency audit; full E2E test coverage with closed-enum envelopes

## Phase Details

### Phase 68: Precise FileFactDiff Populator
**Goal**: Live edits advance `graph_version` with precise per-symbol/per-edge deltas so downstream consumers (ApplyRepair, retrieval, P1 tools) receive accurate change information instead of synthetic markers.
**Depends on**: v1.10 Phase 60 (live update pipeline), Phase 62 (FileFactDiffRecorder seam, 62-09)
**Requirements**: DIFF-01, DIFF-02, DIFF-03, DIFF-04
**Success Criteria** (what must be TRUE):
  1. A live edit to a single Go/TypeScript symbol fires post-commit hook that calls `RecordSymbolChanged` exactly once with the precise pre↔post FileFact delta (Tier-1 path active).
  2. Pre-edit FileFact is readable from the post-commit hook via a stable seam, and `vet-nokernel2semantic` stays green (kernel↔semantic boundary intact).
  3. When extractor returns partial extraction, Tier-2 (added-only) path activates instead of erroring; outcome metric records `tier:"added-only"`.
  4. `internal/semantic/live/handler/handler_diff_e2e_test.go` proves end-to-end recorder traffic + non-empty ApplyRepair under `-race`.
  5. Tier-3 synthetic-marker fallback is restricted to cold-start / missing-pre-edit-FileFact cases and emits a bounded-label warn metric.
**Plans**: 5 plans
- [x] 68-01-PLAN.md — *Store.GetLatestFileFact accessor + PriorFileFact (DIFF-02)
- [x] 68-02-PLAN.md — extract.ExtractionPipeline.ExtractFile shim for Go + TS (DIFF-01 enabler)
- [x] 68-03-PLAN.md — obs.Metrics LiveFileFactDiff + synthetic-reason counters (DIFF-04 enabler)
- [x] 68-04-PLAN.md — Tier-1/Tier-2 populators, diffSymbols (Pitfall-3 guard), Tier-3 reason routing, Handler DI (DIFF-01, DIFF-04)
- [x] 68-05-PLAN.md — handler_diff_e2e_test.go end-to-end + DEF-67-F01-FULL-DIFF closure (DIFF-03)

### Phase 69: Production Status Accessors
**Goal**: `get_semantic_graph_status` returns real cluster and retrieval status from production engines instead of `{state:"unknown"}` placeholders.
**Depends on**: Phase 62 (cluster engine), Phase 64 (status tool wiring), v1.10 Phase 65 (strangler-fig integration)
**Requirements**: STATUS-01, STATUS-02, STATUS-03
**Success Criteria** (what must be TRUE):
  1. Calling `get_semantic_graph_status` against a populated workspace returns a non-placeholder `cluster_status` block with real `{state, computed_at, member_count}`.
  2. `retrieval_status` block carries real `{corpus_version, indexed_files, indexed_symbols, last_compact_at}` sourced from the bleve engine + corpus mapper.
  3. New `*Store.ClusterStatusForGraphVersion` (or equivalent) accessor lands with race-clean read-path tests; no `Begin/Commit/Abort/Write` on the read path (D-09 invariant preserved).
  4. E2E test asserts non-placeholder values across both status blocks on a populated workspace.
  5. The two existing `semantic_wiring.go:408,441,450` placeholder comments are removed and the surrounding code routes through the real accessors.
**Plans**: 6 plans
- [ ] 69-01-PLAN.md — *Store.ClusterStatusForGraphVersion accessor + race-clean tests (STATUS-01)
- [ ] 69-02-PLAN.md — Bleve corpus_version + indexed_files meta in Recoverer; DocCount on Engine (STATUS-02)
- [ ] 69-03-PLAN.md — Compactor last_compact_at write + BleveMeta dep injection (STATUS-02)
- [ ] 69-04-PLAN.md — Envelope additive fields + closed-enum RetrievalStatus.Reason (STATUS-01, STATUS-02)
- [ ] 69-05-PLAN.md — Adapter wiring + placeholder removal in semantic_wiring.go (STATUS-01, STATUS-02)
- [ ] 69-06-PLAN.md — E2E TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval (STATUS-03)

### Phase 70: Incremental Refresh Overlay-Drain
**Goal**: `refresh_semantic_graph` with `mode:"incremental"` consults the overlay-drain seam to refresh only changed files; full-walk becomes a verified fallback, not the default.
**Depends on**: Phase 60 (overlay-drain in live pipeline), Phase 64 (refresh tool), Phase 68 (precise diffs make incremental refresh actually narrow)
**Requirements**: REFRESH-01, REFRESH-02, REFRESH-03
**Success Criteria** (what must be TRUE):
  1. Incremental refresh against a 10k-symbol workspace with 1 file changed touches only that file's facts; full-walk does not run.
  2. Local bench harness records p95 ≤ 200ms for the single-file incremental case (benchmark local-only per project rule).
  3. When overlay-drain returns empty (e.g., overlay rotated), refresh falls back to full-walk and emits a bounded-label log with the fallback reason.
  4. `internal/eval/runner/refresh_incremental_test.go` (or eval-side equivalent) verifies both incremental and fallback paths.
  5. The `refresh-degraded` annotation in `semantic_wiring.go` is removed; the function exits the incremental path through the overlay-drain seam.
**Plans**: TBD

### Phase 71: P1 Single-Symbol Read Tools
**Goal**: Three new `read+` MCP tools answer agent questions about a single seed symbol — deep explanation, related symbols, and edge validation — backed by the v1.10 semantic graph + integ.SemanticLookup.
**Depends on**: Phase 62 (graph engine, ranking, type resolution), Phase 65 (integ.SemanticLookup seam), Phase 68 (precise diffs so freshness envelope is meaningful), Phase 69 (real status surfaces)
**Requirements**: P1TOOL-01, P1TOOL-02, P1TOOL-06
**Success Criteria** (what must be TRUE):
  1. `explain_symbol_deep` returns type chain (with ladder tier + resolved confidence), incoming + outgoing edges (classified by kind), callers, cluster membership, and a closed-enum freshness envelope for any Go/TypeScript/Java symbol.
  2. `find_related_symbols` returns ranked siblings via PageRank-from-seeds + cluster co-membership + RRF fusion; honors `paths` filter as strict-subset like refresh.
  3. `validate_graph_edge` answers `(from, to, kind)` claims with a confidence score + evidence path (LSP / AST / type-resolver citations).
  4. All three tools enforce `read+` mode tier at handler entry; tools/list filters per profile.
  5. Response envelopes carry closed-enum `freshness`, `source`, `fallback_reason` fields mirroring v1.10 conventions; race-clean under concurrent invocation.
**Plans**: TBD

### Phase 72: P1 Cluster & Impact Tools
**Goal**: Three new MCP tools answer workspace-level structural questions — cluster overview, per-cluster detail, and pre-edit blast-radius via the semantic graph.
**Depends on**: Phase 62 (cluster engine), Phase 69 (real ClusterStatus), Phase 71 (semantic skill envelope + mode-check patterns established for the read-only wave)
**Requirements**: P1TOOL-03, P1TOOL-04, P1TOOL-05
**Success Criteria** (what must be TRUE):
  1. `get_cluster_map` returns workspace-level weak-component overview: count, size distribution, top-N clusters with member counts, representative symbols, and dominant edge kinds.
  2. `explain_cluster` consumes a `cluster_id` from the map and returns full member list, per-member ranking, cohesion/separation scores, dominant entry-point symbols.
  3. `get_change_impact_graph` returns a graph subgraph (not file-level summary) for a pre-edit blast-radius query; differs in shape from existing `analyze_blast_radius`.
  4. `get_change_impact_graph` enforces `review+` mode tier; `get_cluster_map` / `explain_cluster` enforce `read+`; profile filtering applies.
  5. All three tools apply a confidence cap when results fall back to a degraded path (e.g., type-resolver tier 3 → cap at 0.6); freshness envelope present on all responses.
**Plans**: TBD

### Phase 73: P1 Tools Integration & E2E Verification
**Goal**: All 6 P1 tools are profile/mode gated correctly across the 5 profiles × 4 modes matrix, wrapped uniformly in the semantic skill, and verified end-to-end against a real populated workspace.
**Depends on**: Phase 71, Phase 72
**Requirements**: P1TOOL-07, P1TOOL-08, P1TOOL-09
**Success Criteria** (what must be TRUE):
  1. Profile-filter golden tests cover all 6 tools across the full 5 × 4 matrix; `tools/list` filters per active profile with no leakage across modes.
  2. `get_tool_help` returns parameter documentation for each of the 6 tools (extracted via `jsonschema.For[T]` like Phase 64 P0 tools).
  3. All 6 tools wrap via `internal/skill/semantic/` following the v1.10 pattern (envelope.go + mode_check.go + accessors.go); zero new daemon-bootstrap special-casing.
  4. E2E integration suite runs each tool against a real `*Store` + bleve + populated graph in a tempdir and asserts closed-enum envelope fields (`freshness`, `source`, `fallback_reason`, `confidence`) are present and within their declared enums.
  5. `vet-nokernel2semantic` and `vet-noduckdb` stay green; race-clean under `go test -race -count=1`.
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-5 | v1.0 | 20/20 | Complete | 2026-04-08 |
| 6-8 | v1.1 | 11/11 | Complete | 2026-04-09 |
| 9-15 | v1.2 | 25/25 | Complete | 2026-04-10 |
| 16-17 | v1.3 | 4/4 | Complete | 2026-04-11 |
| 18-21 | v1.4 | 11/11 | Complete | 2026-04-14 |
| 22-24 | v1.5 | 12/12 | Complete | 2026-04-15 |
| 25-33 | v1.6 | 22/22 | Complete | 2026-04-20 |
| 34-38 | v1.7 | 11/11 | Complete | 2026-04-22 |
| 39-45 | v1.8 | 19/19 | Complete | 2026-04-24 |
| 46-56 | v1.9 | 51/51 | Complete | 2026-05-03 |
| 57-67 | v1.10 | 79/79 | Complete | 2026-05-12 |
| 68-73 | v1.11 | 0/~18 | Not started | - |

## Backlog

_No items in backlog._
