# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- ✅ **v1.2 Performance & Production Hardening** — Phases 9-15 (shipped 2026-04-10)
- ✅ **v1.3 Documentation Catchup** — Phases 16-17 (shipped 2026-04-11)
- ✅ **v1.4 Integration Testing v2** — Phases 18-21 (shipped 2026-04-14)
- ✅ **v1.5 Typed Errors & Hardening** — Phases 22-24 (shipped 2026-04-15)
- 🚧 **v1.6 Context Intelligence & Resilient Editing** — Phases 25-30 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-5) — SHIPPED 2026-04-08</summary>

- [x] Phase 1: Foundation (3/3 plans) — Daemon, MCP runtime, gRPC forwarder, HTTP transport
- [x] Phase 2: Code Intelligence Kernel (6/6 plans) — LSP codegen, worker pool, 9 symbol + 6 edit + 6 file + 3 diag tools
- [x] Phase 3: Multi-Language and Skills (5/5 plans) — 52 languages, memory system, skill interface
- [x] Phase 4: Agent Profiles and Configuration (3/3 plans) — 5 profiles, 4 modes, token budget
- [x] Phase 5: Daemon Bootstrap Integration (3/3 plans) — Full daemon wiring, 38+ callable MCP tools

**Full details:** `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v1.1 Integration Testing (Phases 6-8) — SHIPPED 2026-04-09</summary>

- [x] Phase 6: Test Harness + Go Dogfooding (4/4 plans) — MCP round-trip harness, 38-tool dogfooding, Language field gap closure
- [x] Phase 7: Symbol Editing + Multi-Language Fixtures (3/3 plans) — Edit round-trips, Python/TypeScript/Java/Rust fixtures
- [x] Phase 8: Advanced Testing (4/4 plans) — Profile/mode golden contracts, three-tier concurrency, three-band errors

**Full details:** `.planning/milestones/v1.1-ROADMAP.md`

</details>

<details>
<summary>✅ v1.2 Performance & Production Hardening (Phases 9-15) — SHIPPED 2026-04-10</summary>

- [x] Phase 9: Benchmark Harness & v1.1 Baseline (6/6 plans) — testing.B.Loop benchmarks, CI benchstat gate, v1.1 baseline
- [x] Phase 10: Observability Foundation (3/3 plans) — internal/obs/, trace-aware slog, admin listener, health endpoints, pprof
- [x] Phase 11: Metrics (4/4 plans) — Prometheus /metrics, RED histograms, lspool gauges, bounded-label contract
- [x] Phase 12: Tracing End-to-End (5/5 plans) — otelgrpc propagation, telemetry middleware, per-tool spans, OTLP exporter
- [x] Phase 13: Graceful Degradation (3/3 plans) — Per-class budgets, deadline propagation, ErrCircuitOpen, GOMEMLIMIT
- [x] Phase 14: Documentation (3/3 plans) — README.md, USAGE.md, CHANGELOG.md with auto-generated tables
- [x] Phase 15: Benchmark Gate Hardening (1/1 plans) — capture-baseline.yml workflow, blocking benchstat gate

**Full details:** `.planning/milestones/v1.2-ROADMAP.md`

</details>

<details>
<summary>✅ v1.3 Documentation Catchup (Phases 16-17) — SHIPPED 2026-04-11</summary>

- [x] Phase 16: Core Documentation Update (2/2 plans) — README Production & Observability section, Go-native CONTRIBUTING, CHANGELOG v1.2 gap fill
- [x] Phase 17: Usage & Install Guides (2/2 plans) — USAGE.md accuracy fixes + benchmarks, INSTALL.md with 6-agent setup

**Full details:** `.planning/milestones/v1.3-ROADMAP.md`

</details>

<details>
<summary>✅ v1.4 Integration Testing v2 (Phases 18-21) — SHIPPED 2026-04-14</summary>

- [x] Phase 18: Harness Extraction & Foundation (2/2 plans) — Importable test/harness/, build tag taxonomy
- [x] Phase 19: Protocol & Contract Oracles (3/3 plans) — MCP handshake, 23 goldens, schema validation, error contracts
- [x] Phase 20: Scenarios & Runtime (4/4 plans) — 14+ agent workflow scenarios, pool stress, degraded injection
- [x] Phase 21: LLM Behavioral & Judge (2/2 plans) — Tool selection, disambiguation, interpretation, judge scoring

**Full details:** `.planning/milestones/v1.4-ROADMAP.md`

</details>

<details>
<summary>✅ v1.5 Typed Errors & Hardening (Phases 22-24) — SHIPPED 2026-04-15</summary>

- [x] Phase 22: Error Taxonomy (2/2 plans) — internal/errors/ package with 7 error kinds, builder pattern, cause-chain wrapping
- [x] Phase 23: Tool Migration (8/8 plans) — All 38+ tools migrated from raw strings to typed errors across 10 packages
- [x] Phase 24: Validation & Testing (2/2 plans) — Inline input validation on 24 kernel tools, Kind-level test assertions, typed golden files

**Full details:** `.planning/milestones/v1.5-ROADMAP.md`

</details>

### v1.6 Context Intelligence & Resilient Editing (In Progress)

**Milestone Goal:** Give agents a ranked, token-budgeted view of any codebase and make edit tools resilient to LLM output drift.

- [x] **Phase 25: Fuzzy Edit Engine** - Core fuzzy matching engine with 4-strategy cascade, indentation preservation, ambiguity detection, and ellipsis placeholders (completed 2026-04-16)
- [x] **Phase 26: Fuzzy Edit Integration** - Standalone fuzzy_edit MCP tool and fallback wiring into replace_symbol_body and replace_content (completed 2026-04-16)
- [x] **Phase 27: RepoMap Tag Extraction & Cache** - Tree-sitter tag extraction with LSP fallback, SQLite cache, and scope-aware elision (completed 2026-04-16)
- [x] **Phase 28: RepoMap Graph & MCP Tools** - Cross-file reference graph, PageRank ranking, token-budgeted MCP tools, and LSP enrichment (completed 2026-04-17)
- [ ] **Phase 29: Phase 25 Formal Verification** - Run formal verification on fuzzy edit engine to close FUZZ-01/02/03/07/08 (gap closure)
- [ ] **Phase 30: RepoMap Pipeline Wiring** - Wire TagExtractor, FallbackExtractor, EnrichFromLSP, and TreeRenderer in RepoMapSkill (gap closure)

## Phase Details

### Phase 25: Fuzzy Edit Engine
**Goal**: Agents can perform fuzzy text matching and replacement that tolerates whitespace drift, indentation changes, and ellipsis placeholders in search blocks
**Depends on**: Nothing (independent of RepoMap work)
**Requirements**: FUZZ-01, FUZZ-02, FUZZ-03, FUZZ-07, FUZZ-08
**Success Criteria** (what must be TRUE):
  1. Agent can submit a search block with minor whitespace differences from actual file content and the fuzzy matcher finds the correct location using the 4-strategy cascade (exact, whitespace-normalized, indentation-flexible, fail-with-diff)
  2. Tool response includes match_strategy and similarity_score fields so the agent knows how the match was resolved
  3. Replacement text inherits the original file's indentation level when a fuzzy match succeeds, not the indentation from the agent's search block
  4. When the search text matches multiple locations in the file, the tool refuses the edit and reports the ambiguity instead of silently picking one
  5. Agent can use ellipsis/placeholder markers in search blocks to skip unchanged code sections, matching only the anchoring lines around the placeholder
**Plans**: 6 plans
- [x] 25-01-PLAN.md — Types & Options (Strategy enum, Options, Result structs) [Wave 1]
- [x] 25-02-PLAN.md — Line splitter & ellipsis segmenter [Wave 2]
- [x] 25-03-PLAN.md — 4-strategy cascade sweeps (exact / whitespace / indent-flex) [Wave 2]
- [x] 25-04-PLAN.md — Indentation reflow (common-prefix dedent + reapply) [Wave 2]
- [x] 25-05-PLAN.md — Fail-with-diff + ambiguity formatters [Wave 2]
- [x] 25-06-PLAN.md — Cascade orchestrator + Match() entry point [Wave 3]

### Phase 26: Fuzzy Edit Integration
**Goal**: Existing edit tools gracefully fall back to fuzzy matching when exact matching fails, and agents have a standalone fuzzy edit tool for arbitrary text operations
**Depends on**: Phase 25
**Requirements**: FUZZ-04, FUZZ-05, FUZZ-06
**Success Criteria** (what must be TRUE):
  1. Agent can call the standalone `fuzzy_edit` MCP tool to perform raw text fuzzy matching on any file, independent of symbol boundaries
  2. When `replace_symbol_body` receives a search block that does not exactly match content within the tree-sitter-located body, it falls back to fuzzy matching and succeeds if a fuzzy match is found
  3. When `replace_content` receives a search string that does not match exactly or via regex, it falls back to fuzzy matching and succeeds if a fuzzy match is found
**Plans**: 2 plans
Plans:
- [x] 26-01-PLAN.md — Standalone fuzzy_edit MCP tool + replace_in_file fuzzy fallback [Wave 1]
- [x] 26-02-PLAN.md — replace_symbol_body fuzzy fallback with search_body parameter [Wave 1]

### Phase 27: RepoMap Tag Extraction & Cache
**Goal**: The system can extract, cache, and elide structural tags (definitions and references) from source files across multiple languages
**Depends on**: Nothing (independent of fuzzy editing work)
**Requirements**: RMAP-01, RMAP-02, RMAP-03, RMAP-09
**Success Criteria** (what must be TRUE):
  1. Tree-sitter .scm queries extract def/ref tags from Go, Python, TypeScript, and Rust source files with correct symbol names and locations
  2. Languages without tree-sitter grammars fall back to LSP documentSymbol for tag extraction, producing compatible tag data
  3. Extracted tags persist in SQLite with mtime-based invalidation, surviving daemon restarts and client reconnects without re-extraction of unchanged files
  4. Tag output uses scope-aware elision showing signatures without bodies (via tree-sitter), keeping output compact for token-budgeted consumption
**Plans**: 4 plans
Plans:
- [x] 27-01-PLAN.md — Shared grammar registry, Tag types, tree-sitter .scm queries, TagExtractor, BodyExtractor refactor [Wave 1]
- [x] 27-02-PLAN.md — LSP documentSymbol fallback extractor [Wave 1]
- [x] 27-03-PLAN.md — SQLite tag cache with mtime-based invalidation [Wave 2]
- [x] 27-04-PLAN.md — Scope-aware elision renderer [Wave 2]

### Phase 28: RepoMap Graph & MCP Tools
**Goal**: Agents can request ranked, token-budgeted structural overviews and task-focused context from any codebase via MCP tools
**Depends on**: Phase 27
**Requirements**: RMAP-04, RMAP-05, RMAP-06, RMAP-07, RMAP-08, RMAP-10
**Success Criteria** (what must be TRUE):
  1. Agent can call `get_repo_map` and receive a structural overview of the repository with symbols ranked by importance via PageRank, fitting within a specified token budget
  2. Agent can call `get_context` with a task description or set of files and receive the most relevant symbols for that task, ranked and token-budgeted
  3. Cross-file reference graph correctly links definitions to references across files, with edges weighted by reference frequency
  4. When LSP sessions are warm, the reference graph is enriched with precise cross-file references beyond what tree-sitter tags provide
  5. Token budget parameter controls output size via binary search to maximize symbol coverage within the specified budget
**Plans**: 3 plans
Plans:
- [x] 28-01-PLAN.md — Cross-file reference graph, PageRank, LSP enrichment [Wave 1]
- [x] 28-02-PLAN.md — Token-budgeted tree renderer with binary search [Wave 2]
- [x] 28-03-PLAN.md — RepoMapSkill MCP tools (get_repo_map, get_context) + daemon wiring [Wave 2]

### Phase 29: Phase 25 Formal Verification
**Goal**: Close 5 unsatisfied FUZZ requirements by running formal verification on Phase 25 — code exists and tests pass, only VERIFICATION.md is missing
**Depends on**: Phase 25
**Requirements**: FUZZ-01, FUZZ-02, FUZZ-03, FUZZ-07, FUZZ-08
**Gap Closure:** Closes verification gaps from v1.6 audit
**Success Criteria** (what must be TRUE):
  1. VERIFICATION.md exists for Phase 25 with evidence that all 5 FUZZ requirements are satisfied
  2. Each requirement has test evidence or code inspection confirming implementation
**Plans**: 1 plan
Plans:
- [ ] 29-01-PLAN.md — Run tests and create 25-VERIFICATION.md with evidence for FUZZ-01/02/03/07/08 [Wave 1]

### Phase 30: RepoMap Pipeline Wiring
**Goal**: Make get_repo_map and get_context functional by wiring the tag extraction pipeline, LSP enrichment, and TreeRenderer into RepoMapSkill
**Depends on**: Phase 27, Phase 28
**Requirements**: RMAP-04, RMAP-05, RMAP-06, RMAP-07, RMAP-08, RMAP-10
**Gap Closure:** Closes integration and flow gaps from v1.6 audit
**Success Criteria** (what must be TRUE):
  1. RepoMapSkill.Init() instantiates TagExtractor and FallbackExtractor, populating the cache with real file data
  2. get_repo_map returns ranked symbols (not "No files found") for a workspace with Go/Python/TypeScript files
  3. get_context returns task-relevant symbols (not "No files found") when given seed files or task description
  4. EnrichFromLSP is called when LSP sessions are warm, adding precise cross-file references
  5. TreeRenderer.RenderBudgeted is used (not inline copy), with 15% tolerance binary search
  6. ProjectDir is derived from workspace path, not hardcoded
**Plans**: 1 plan
Plans:
- [ ] 29-01-PLAN.md — Run tests and create 25-VERIFICATION.md with evidence for FUZZ-01/02/03/07/08 [Wave 1]

## Progress

**Execution Order:**
Phases execute in numeric order: 25 → 26 → 27 → 28
Note: Phases 25-26 (fuzzy) and 27-28 (repomap) are independent tracks. Within each track, order is sequential.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-07 |
| 2. Code Intelligence Kernel | v1.0 | 6/6 | Complete | 2026-04-08 |
| 3. Multi-Language and Skills | v1.0 | 5/5 | Complete | 2026-04-08 |
| 4. Agent Profiles and Configuration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 5. Daemon Bootstrap Integration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 6. Test Harness + Go Dogfooding | v1.1 | 4/4 | Complete | 2026-04-09 |
| 7. Symbol Editing + Multi-Language Fixtures | v1.1 | 3/3 | Complete | 2026-04-09 |
| 8. Advanced Testing | v1.1 | 4/4 | Complete | 2026-04-09 |
| 9. Benchmark Harness & v1.1 Baseline | v1.2 | 6/6 | Complete | 2026-04-09 |
| 10. Observability Foundation | v1.2 | 3/3 | Complete | 2026-04-09 |
| 11. Metrics | v1.2 | 4/4 | Complete | 2026-04-09 |
| 12. Tracing End-to-End | v1.2 | 5/5 | Complete | 2026-04-10 |
| 13. Graceful Degradation | v1.2 | 3/3 | Complete | 2026-04-10 |
| 14. Documentation | v1.2 | 3/3 | Complete | 2026-04-10 |
| 15. Benchmark Gate Hardening | v1.2 | 1/1 | Complete | 2026-04-10 |
| 16. Core Documentation Update | v1.3 | 2/2 | Complete | 2026-04-11 |
| 17. Usage & Install Guides | v1.3 | 2/2 | Complete | 2026-04-11 |
| 18. Harness Extraction & Foundation | v1.4 | 2/2 | Complete | 2026-04-11 |
| 19. Protocol & Contract Oracles | v1.4 | 3/3 | Complete | 2026-04-11 |
| 20. Scenarios & Runtime | v1.4 | 4/4 | Complete | 2026-04-12 |
| 21. LLM Behavioral & Judge | v1.4 | 2/2 | Complete | 2026-04-12 |
| 22. Error Taxonomy | v1.5 | 2/2 | Complete | 2026-04-15 |
| 23. Tool Migration | v1.5 | 8/8 | Complete | 2026-04-15 |
| 24. Validation & Testing | v1.5 | 2/2 | Complete | 2026-04-15 |
| 25. Fuzzy Edit Engine | v1.6 | 6/6 | Complete   | 2026-04-16 |
| 26. Fuzzy Edit Integration | v1.6 | 2/2 | Complete   | 2026-04-16 |
| 27. RepoMap Tag Extraction & Cache | v1.6 | 4/4 | Complete   | 2026-04-16 |
| 28. RepoMap Graph & MCP Tools | v1.6 | 3/3 | Complete   | 2026-04-17 |
| 29. Phase 25 Formal Verification | v1.6 | 0/0 | Pending | — |
| 30. RepoMap Pipeline Wiring | v1.6 | 0/0 | Pending | — |
