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
- [ ] **v1.10 Live Semantic Index** -- Phases 57-67 (planning)

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

### v1.10 Live Semantic Index (Planning) -- Phases 57-67

- [x] Phase 57: Semantic Store Foundation + Pipeline DAG Library (5/5 plans complete; verifier passed — gap-closure 57-05 closed SC-1 + CR-01/02 + WR-01/02/03; 57-REVIEW-57-05 follow-up pass closed BL-01 + 6 add'l findings) (completed 2026-05-06)
- [x] Phase 58: v1.9 Carryover -- Release & Distribution (4/4 plans) (completed 2026-05-04)
- [x] Phase 59: Tree-sitter Extraction & Stable Symbol IDs (7/7 plans) (completed 2026-05-04; 2026-05-08 D-06/D-07/D-08/D-11 delta added 59-06/59-07 — Phase 65 unblock complete)
- [x] Phase 59.1: drop-cgo-0-single-mode-cgo-1-build-release (6/6 plans, INSERTED) — Drop CGO=0 — single-mode CGO=1 build & release; re-verifier 2026-05-06 PASSED (cosign verify-blob against real v1.10.7 sigstore bundles + Gatekeeper xattr mechanism + linux gcc resolution all closed) (completed 2026-05-04, re-verified 2026-05-06)
  Plans:
  - [x] 59.1-00-PLAN.md — Wave 0 pre-execution probes (darwin zig cc, release-smoke target, D-14 lock)
  - [x] 59.1-01-PLAN.md — Wave 1 stub deletion + tag strip + Path-3 platform stub + daemon step-6a removal
  - [x] 59.1-02-PLAN.md — Wave 2 goreleaser v2 split/merge probe + .goreleaser.yaml CGO=1 flip (linux+windows zig cc; darwin Apple clang) + partial:by_target stanza + Makefile zig guard
  - [x] 59.1-03-PLAN.md — Wave 3 release.yml split-runner rewrite: release-linux (4 archives, ubuntu-22.04, zig cc) + release-darwin (2 archives, macos-14, Apple clang, tag-gated per D-16) + release-merge (cosign uniformly per D-19, gh release create)
  - [x] 59.1-04-PLAN.md — Wave 4 doc sweep with split-runner amendments (CLAUDE/README/CONTRIBUTING/CHANGELOG/PROJECT/REQUIREMENTS/deferred-items/51.1-SUMMARY/v1.10-ROADMAP) + Gatekeeper workaround in INSTALL.md (D-19) + 3 NEW deferred items (DEF-59-NOTARIZE/DARWIN-SMOKE/DARWIN-CANARY)
  - [x] 59.1-05-PLAN.md — Wave 5 release-smoke finalization + per-runner Pass-1≡Pass-2 reproducibility verification on BOTH runners (HARD GATE; D-09/D-10 amended; cross-runner byte-equality NOT a gate per Pitfall 8)
- [x] Phase 60: Live Update Pipeline (0/6 plans) (completed 2026-05-05)
  > **Phase 60 P04 obligation (from 62-09 closure):** the full FileFact upsert MUST record SymbolDiff entries via `internal/semantic/live/handler.FileFactDiffRecorder.RecordSymbol*` so Phase 62's post-commit `ApplyRepair` hook fires productively in production. Until P04 lands, `handler.UpdateChangedFile` emits a once-INFO log per workspace surfacing the empty-diff short-circuit (62-VERIFICATION.md truth #22).
  Plans:
  - [x] 60-01-PLAN.md — Wave 0 vet-nokernel2semantic analyzer enforcing LIVE-07 invariant #1 (kernel→semantic boundary)
  - [x] 60-02-PLAN.md — Wave 1 schema migration v2→v3 (current_epoch + 4× write_epoch + 4 indexes) + overlay writer (BeginOverlayTx + MarkFileDeleted/MarkSymbolsDeleted/MarkReferencesDeleted/MarkEdgesDeleted) + per-tx epoch contract under -race stress
  - [x] 60-03-PLAN.md — Wave 1 EditNotifier interface in internal/kernel + 8-tool wiring (5 edit + 3 fileops) + non-blocking integration tests
  - [x] 60-04-PLAN.md — Wave 2 live spine: signal/classifier/coalescer/handler/service + ScheduleIncremental fill + lspqueue handoff (TDD: CoalesceEvents + MergeChange + ClassifyPathChange)
  - [x] 60-05a-PLAN.md — Wave 3 (parallel with 60-05b) per-workspace fsnotify watcher + ENOSPC fallback + LIVE-02 editor-fixture suite (Vim/JetBrains/VS Code)
  - [x] 60-05b-PLAN.md — Wave 3 (parallel with 60-05a) manifest scanner + 3 new config keys + bounded-label metric + pipelines/live.go fill + daemon bootstrap + Phase 60 close-out checkpoint
- [x] Phase 61: LSP Enrichment Worker (5/5 plans complete; verifier passed — gap-closure 61-05 closed 61-VERIFICATION.md gap #1: Manager NewCascadeLSP production wiring) (completed 2026-05-06)
  Plans:
  - [x] 61-01-PLAN.md — LeaseAcquirer + ForegroundBusy + 2-lane queue + handler producer rewiring + bulk-suppression API + nosemantic2kernel analyzer [ENRICH-01, ENRICH-02]
  - [x] 61-02-PLAN.md — Worker goroutine + §14.4 cascade engine + Budget enforcement + readiness-gate honoring + per-file overlay commit [ENRICH-02, ENRICH-03, ENRICH-04]
  - [x] 61-03-PLAN.md — Metrics + trace spans + Status() accessor + Manager + Daemon bootstrap + 2 new config keys [ENRICH-01..ENRICH-04]
  - [x] 61-04-PLAN.md — ENRICH-05 stress test + acceptance closeout + REQUIREMENTS.md check-off [ENRICH-05, ENRICH-01..ENRICH-04]
  - [x] 61-05-PLAN.md — Gap closure: promote cascadeLSPShim to production (cascade_lsp_shim.go) + wire CascadeLSPFactory through Manager.SetCascadeLSPFactory + live_wiring.go production factory + end-to-end TestManagerProductionDispatch_Go integration test (severs Phase 64 dependency for production dispatch) [ENRICH-01..ENRICH-05]
- [x] Phase 62: Graph Engine, Ranking & Type Resolution (9/9 plans — 5 original + 4 gap closure)
  > **Cross-phase carry-forward (62-09 closure):** the live handler now threads a `FileFactDiffRecorder` through every overlay tx; populators in Phase 60 P04 (full FileFact upsert) and any future Phase 62 type-resolver live-edge retrofit MUST write through `internal/semantic/live/handler.FileFactDiffRecorder` so the post-commit `ApplyRepair` hook surfaces graph-changing edits in production. Until those populators land, the empty-diff once-INFO log per workspace at `helix.live.handler` surfaces the gap. Phase 60 entry above carries the populator obligation.
  Plans:
  - [x] 62-01-PLAN.md — Wave 1 — Shared deterministic PageRank engine at internal/graph + repomap migration + re-pinned vectors [GRAPH-01, GRAPH-02]
  - [x] 62-02-PLAN.md — Wave 2 — graph_version + ApplyRepair single-bump + score_status read API + UpsertGraphScores/UpsertEdgesWithMerge + handler post-commit hook + 4 config keys + 5 bounded-label metrics [GRAPH-03, GRAPH-05]
  - [x] 62-03-PLAN.md — Wave 3 — RankScheduler (debounce + long-idle + drop-on-full) + 1-hop frontier + full recompute preemption + WriteInvalidations consumer + daemon errgroup wiring [GRAPH-04, GRAPH-05]
  - [x] 62-04-PLAN.md — Wave 4 — Weak-component clustering algorithm + persistence (UpsertClusters/UpsertClusterMembers/DeleteClustersForGraphVersion) [GRAPH-06]
  - [x] 62-05-PLAN.md — Wave 5 — Type resolver shared core + Go/TS/Python full ladders + Java LSP-conditional stub + PHP/Ruby always-0.20 stubs + two-phase comment merge + dispatcher daemon wiring [TYPES-01, TYPES-02, TYPES-03, TYPES-04]
  - [x] 62-06-PLAN.md — Gap closure (CR-03) — sorted suffix-rule slice iteration in golang/python/typescript guessFromName helpers (sort-before-iterate doctrine restored) [GRAPH-01, TYPES-04]
  - [x] 62-07-PLAN.md — Gap closure (CR-01) — explicit lock release between tx.Commit and CountStaleScoreRows + SchedulerStore.CountStaleScoreRows lock contract [GRAPH-03, GRAPH-04, GRAPH-05]
  - [x] 62-08-PLAN.md — Gap closure (truth #21 / WR-05) — rankStoreAdapter stub observability via outcome=stub_no_data closed-enum extension + sync.Once-gated WARN per (workspace, method) + TODO(phase-64) anchor [GRAPH-04, GRAPH-05]
  - [x] 62-09-PLAN.md — Gap closure (truth #22) — FileFactDiffRecorder seam threaded through Handler.UpdateChangedFile + once-INFO empty-diff log [GRAPH-03, GRAPH-05]
- [x] Phase 63: Compaction & Retention (0/2 plans) (completed 2026-05-07)
  Plans:
  - [x] 63-01-PLAN.md — Wave 1 — Snapshot-write API on *Store (BeginSnapshot/WriteSnapshotFacts/CommitSnapshot/AbortSnapshot/DeleteSnapshotsBeyond) + synthetic fake-compactor fixture (TDD; foundation for P63-02) [COMPACT-01, COMPACT-04]
  - [x] 63-02-PLAN.md — Wave 2 — internal/semantic/compact/ package (compactor goroutine + CompactionGate.IsReady aggregator + VACUUM no-op piggyback) + 6 accessor additions on existing components + migration004 (last_vacuum_at) + daemon wiring + maintenance.* config + helix_semantic_compaction/vacuum metrics + vet-compact-uses-store analyzer + kill-mid-compact subprocess test + CAS interleave property test + long-repo bench fixture [COMPACT-01, COMPACT-02, COMPACT-03, COMPACT-04, COMPACT-05]
- [x] Phase 64: New MCP Tools (P0 set of 4) (8/8 plans) — VERIFIED PASSED-WITH-CARRYOVER 2026-05-08 (2 Phase-65 carryover items: production buildFn empty-Facts placeholder + zero-value WorkspaceKey from session adapter)
  **Goal:** Agents can index, refresh, inspect, and query the semantic graph through four new MCP tools (`index_semantic_graph`, `refresh_semantic_graph`, `get_semantic_graph_status`, `get_semantic_context`) whose responses always carry `freshness` + `graph_version` fields, profile/mode gating respects the existing matrix, and selection is deterministic.
  **Depends on:** Phase 62, Phase 63
  **Requirements:** TOOL-01, TOOL-02, TOOL-03, TOOL-04, TOOL-05
  **Success Criteria:**
  1. An agent calling `index_semantic_graph` (mode `review+`/`admin`) builds or refreshes a committed snapshot in `auto`/`full`/`incremental`/`refresh` modes and receives snapshot id, graph version, files indexed/reused, partial state, freshness, and duration.
  2. An agent calling `refresh_semantic_graph` (mode `read+`) applies pending live source changes without forcing a full reindex; supports `wait_for_lsp` and `paths` filters.
  3. An agent calling `get_semantic_context` receives `freshness=structurally_fresh_semantically_pending` after a single Helix edit and within the foreground-tool budget; the response includes `freshness_mode`, `graph_version`, `overlay_active`, `pending_lsp_files`, and per-candidate `evidence` + `confidence`.
  4. `tools/list` filters the four tools out for profiles that don't include them; `get_tool_help` returns parameter docs for each.
  Plans:
  - [x] 64-01-PLAN.md — [BLOCKING WAVE-0 GATE] bleve binary-size + 50k-symbol indexing throughput benchmark vs DuckDB FTS5; commits PASS/FAIL verdict per D-08 [TOOL-04]
  - [x] 64-02-PLAN.md — Wave 0 (TDD) effective-graph queries on *Store: QueryEffectiveAdjacency + CountStaleScoreRows + MarkAllScoreRowsStale + LatestCommittedSnapshot (deferred-from-63) [TOOL-03, TOOL-04]
  - [x] 64-03-PLAN.md — Wave 0 skill skeleton: SemanticSkill+init() registration, mode_check.go (NEW pattern), envelope.go closed enums, 5 profile YAMLs + 4 mode YAMLs updated for `semantic` skill (D-14) [TOOL-05]
  - [x] 64-04-PLAN.md — Wave 1 (TDD) IndexRunner singleflight (D-02) + sync-with-timeout dispatch (D-01/D-04) + tools_index.go handler with mode-tier check + path-traversal hardening [TOOL-01, TOOL-05]
  - [x] 64-05-PLAN.md — Wave 1 (TDD) tools_refresh.go: read+ tier, paths strict-subset (D-11), wait_for_lsp polling (D-12), no-snapshot/no-compactor invariant (D-09/D-13) [TOOL-02, TOOL-05]
  - [x] 64-06-PLAN.md — Wave 1 tools_status.go: SPEC §23.3 envelope fan-out across all read-only accessors; closed-enum freshness [TOOL-03, TOOL-05]
  - [x] 64-07-PLAN.md — Wave 1 (TDD) retrieval package: bleve scorch + corpus mapping (D-06) + weighted RRF (D-07) + dual-store recovery + tools_context.go with determinism harness (10× byte-identical) [TOOL-04, TOOL-05]
  - [x] 64-08-PLAN.md — Wave 2 daemon glue: semantic_wiring.go bundle + production buildFn + imports.go blank import + daemon.go register*SemanticGraph + rank_wiring stub-collapse + profile-filter test + get_tool_help test + 6 end-to-end integration tests closing CONTEXT.md acceptance #1/#2/#3/#5/#6 [TOOL-01..TOOL-05]
- [x] Phase 65: Existing-Tool Integration (Strangler Fig) (9/13 plans) (gap-closure 2026-05-08 — 4 follow-up plans for VERIFICATION.md gaps + REVIEW.md findings) (completed 2026-05-08)
  **Goal:** `get_repo_map`, `get_context`, `analyze_blast_radius`, and `get_health` consult the semantic graph when available — with zero source change to `internal/repomap` engine — and fall back to v1.9 behavior automatically when the index is disabled, building, or errored.
  **Depends on:** Phase 64
  **Requirements:** INTEG-01, INTEG-02, INTEG-03, INTEG-04, INTEG-05
  **Success Criteria:**
  1. With the semantic index populated, `get_repo_map` and `get_context` return ranked output sourced from persisted graph scores + clusters; with `semantic_index.enabled=false` they return the v1.9 tree-sitter + PageRank output (index-disabled goldens preserved).
  2. `analyze_blast_radius` returns `confidence` and `evidence` per impacted node when the semantic graph is available; on fallback, confidence drops to ≤ 0.6 and the result envelope says so.
  3. `get_health` includes a `semantic_index` section: store kind, latest snapshot status, graph version, overlay active flag, pending LSP count, last live-update latency, and last error.
  4. Every MCP envelope from a semantic-aware tool returns `source: semantic | tree_sitter | fallback` so callers can detect path drift.
  **Carryover from Phase 64:** production buildFn empty-Facts placeholder; zero-value WorkspaceKey from session adapter (deferred 2026-05-08 per phase 64 verification).
  Plans:
  - [x] 65-00-PLAN.md — Wave 0 vet allowlist: nokernel2semantic permits internal/semantic/integ (M-vet unblock for 65-03/65-06)
  - [x] 65-01-PLAN.md — Wave 0 (TDD) production buildFn: per-language extract + classifier walk + ToStoreFacts → WriteSnapshotFacts (D-09 carryover #1)
  - [x] 65-02-PLAN.md — Wave 0 (TDD) WorkspaceKey adapter: closure pass-through replaces zero-value return (D-09 carryover #2)
  - [x] 65-03-PLAN.md — Wave 1 (TDD) internal/semantic/integ types-only package + integSemanticLookup production adapter + read-tier grep canary (D-01/D-02/D-03; INTEG-01..05)
  - [x] 65-04-PLAN.md — Wave 1 (TDD) source-field envelope contract + ChooseSource priority ladder + closed-enum matrix tests (D-04/D-05; Pitfall §3; INTEG-05)
  - [x] 65-05-PLAN.md — Wave 2 (TDD) get_repo_map + get_context wired via SetSemanticLookup; JSON envelope wrap; index-disabled goldens preserved verbatim (INTEG-01/02/05)
  - [x] 65-06-PLAN.md — Wave 2 (TDD) analyze_blast_radius two-pass: lookup.ExpandFrom + LSP-validates-critical-edges; fallback confidence cap ≤ 0.6 (D-07/D-08; INTEG-03/05)
  - [x] 65-07-PLAN.md — Wave 2 (TDD) get_health semantic_index block additive (Phase 57 SC-1 preserved; WR-NEW-01 closed-enum last_error) (INTEG-04/05)
  - [x] 65-08-PLAN.md — Wave 3 acceptance closure: index-disabled tree text byte-identical + {source × fallback_reason} matrix across all 4 tools
  - [x] 65-09-PLAN.md — Wave 4 (TDD, gap closure) production-adapter E2E test scaffolding (Skipf gates) + BL-A populated-harness builder (NewE2EIntegLookupForTest with canonical syms keys) + REVIEW.md CR-02 closed-enum classifier fix (errors.Is(serr.ErrUnsupported))
  - [x] 65-10-PLAN.md — Wave 5 (TDD, gap closure) integSemanticLookup.RankFiles + RankFromSeeds real implementations + *Store.QueryRankedFiles + *Store.QuerySymbolPath + WR-04/WR-06/WR-07 fixes (INTEG-01/02)
  - [x] 65-11-PLAN.md — Wave 6 (TDD, gap closure) integSemanticLookup.SymbolID + ExpandFrom real implementations + *Store.QuerySymbolByLocation/QueryNodeIDByStableKey/QueryStableKeyByNodeID + WR-01/IN-04/WR-05/WR-03 fixes (INTEG-03)
  - [x] 65-12-PLAN.md — Wave 7 (TDD, gap closure) kernel-side LSP probe via lspProbeFn + integ.SemanticLookup.LocateSymbol + BL-1 confidence-ladder kernel-side test (consumes 65-09 BL-A harness) + production-adapter SourceSemantic + BlastRadiusConfidence E2E green + WR-02 go/parser canary (INTEG-03/05)
- [x] Phase 66: Agent Guardrails (G-001..G-005, warn-default) (0/6 plans) (completed 2026-05-09)
  Plans:
  - [x] 66-01-PLAN.md — Wave 1 (TDD) receipt foundation: ID/class enum/scope union, store (TTL/janitor/LRU/graph_version invalidation), ValidateReceiptForOperation, 5-layer enforcement resolver, issue sink, serr.GuardrailViolation, 3 receipt counters [GUARD-03/04/05/07]
  - [x] 66-02-PLAN.md — Wave 1 (TDD) SemanticLookup.Visibility + IsEntrypointReachable (OI-02/OI-03), GuardrailsConfig D-22 extension, per-profile YAML defaults (ci-bot=enforce; rest=warn) [GUARD-07]
  - [x] 66-03-PLAN.md — Wave 2 (TDD) five rule predicates G-001..G-005 + RuleEvaluator dispatch + G-005 catalogs (Go/TS/JS/Python via embed.FS) [GUARD-02/07]
  - [x] 66-04-PLAN.md — Wave 3 GuardrailMiddleware + production deps + skill + daemon step 14b.5 + TelemetryMiddleware outcome extension + 5-step LIFO regression test [GUARD-01/03/04/05/07]
  - [x] 66-05-PLAN.md — Wave 4 (parallel with 66-06) receipt issuance wired into 8 read/diagnostics tools + Receipts field on 6 destructive args [GUARD-02/03/05]
  - [x] 66-06-PLAN.md — Wave 4 (parallel with 66-05) GUARDRAILS.md + DoD.md + 6 get_tool_help topics + GUARD-02 SC-2 forwarder→daemon context-truncation E2E [GUARD-02/06]
- [x] Phase 67: Evaluation Harness (0/8 plans) (completed 2026-05-10)
  - [x] 67-01-baseline-profile-and-skeleton-PLAN.md — Wave 0 (TDD) baseline profile YAML + Assumption A1 verification + eval/ tree + internal/eval/* skeleton + cmd/helix-eval cobra + Makefile targets + EVAL.md TOS attestation [EVAL-02/03/04/05/06]
  - [x] 67-02-sandbox-and-agent-PLAN.md — Wave 1 (TDD) per-(task,mode) sandbox isolation (HOME/socket/repo/tmpdir) + claude CLI subprocess wrapper (--bare --strict-mcp-config) + budget watchdog (D-08 four-axis) [EVAL-02]
  - [x] 67-03-trace-tap-and-merge-PLAN.md — Wave 1 (TDD) typed trace schema + daemon JSONL stderr tap (pid-gated, T-67-04) + CC stream-json stdout tap + wall-clock merge (T-67-02 path-prefix invariant) [EVAL-01/02]
  - [x] 67-04-scorer-dsl-and-seed-corpus-PLAN.md — Wave 2 (TDD) heuristic rule DSL (KnownFields strict) + scorer (sequence/set/receipts) + 10 hand-authored seed tasks (Go+TS+Python; rename/delete/public-API/large-edit/security) [EVAL-01/04/05]
  - [x] 67-05-runner-and-reporters-PLAN.md — Wave 3 (TDD) ZDR corpus gate (EVAL-06) + EvalResult + runner filling phasegraph EvalPhases Run bodies + 5 aggregate reporters (eval_report.{json,md} cost_summary safety_compliance run_metadata tool_behavior) [EVAL-01/02/04/06]
  - [x] 67-06a-eval-quick-inprocess-PLAN.md — Wave 4 (TDD) in-process daemon over bufconn + scripted-agent (no claude, no subprocess) + 2 reference quick fixtures + Pitfall-6 four-layer harness-validation guard + per-mode daemon reuse [EVAL-03]
  - [x] 67-06b-eval-quick-fixtures-expansion-PLAN.md — Wave 4 (TDD) 8 expansion fixtures covering all 5 EVAL-05 families (rename/delete/public_api/large_edit/security) × Go/TS/Python; 10-fixture × 4-mode wall-time test asserts D-05 <30s budget [EVAL-03/05]
  - [x] 67-07-llm-judge-and-ci-PLAN.md — Wave 5 (TDD) hand-rolled Anthropic HTTP client + judge orchestration (trace-only, INFORMATIONAL boilerplate, signature-enforced exit-code isolation) + CI workflow (eval-quick PR-gate; grep gate forbids judge in CI) [EVAL-04/07]

**Full details:** `.planning/milestones/v1.10-ROADMAP.md`

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
| 57-67 | v1.10 | 23/31 | Planning | -- |

## Backlog

_No items in backlog._
