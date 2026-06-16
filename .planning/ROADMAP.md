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
- [x] **v1.11 Semantic Index Completion & P1 MCP Tools** -- Phases 68-74 (shipped 2026-06-07) — see `.planning/milestones/v1.11-ROADMAP.md`
- [ ] **v1.12 Bench Stack & Tool Evaluation** -- Phases 75-89 (active, started 2026-06-13) — see `.planning/milestones/v1.12-ROADMAP.md`

## Phases

### 🚧 v1.12 Bench Stack & Tool Evaluation (Phases 75-89) — ACTIVE

15 phases, 63 v1 requirements, 100% mapped. Headline claim: *"Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits."*

- [x] Phase 75: Schema, Fairness Contract & Tree Skeleton (completed 2026-06-15)
- [ ] Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags
- [ ] Phase 77: Bench Runtime & First E2E Smoke
- [ ] Phase 78: Internal ToolBench — Go First + LanguageRunner Interface
- [ ] Phase 79: Evaluators & Result-Schema Metrics Layer
- [ ] Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement
- [ ] Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test
- [ ] Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard
- [ ] Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder
- [ ] Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard
- [ ] Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners
- [ ] Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate
- [ ] Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness`
- [ ] Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters
- [ ] Phase 89: Reports, CI Policy & Contamination Canary

**Full details:** `.planning/milestones/v1.12-ROADMAP.md`

### Phase 75: Schema, Fairness Contract & Tree Skeleton

**Goal**: Every downstream phase has a versioned `result.v2.json` schema to write into and a single `fairness_contract.go` struct to load model config from — so no benchmark adapter ever defines its own model snapshot, temperature, or cost row.

**Depends on**: v1.10 Phase 67 (`internal/eval/` patterns — sandbox, trace tap, score DSL)

**Requirements**: BENCH-01, BENCH-02, BENCH-03, BENCH-06, FAIR-01, FAIR-02, FAIR-03, COST-01, INFRA-01, INFRA-02, INFRA-03

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 75)

### Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags

**Goal**: Land the only invasive code paths inside the daemon (kernel-level `disable_lsp_subsystem` / `disable_structured_edit_subsystem` flags) plus the 4 new bench profile YAMLs, with a static `vet-ablation-leakage` analyzer so they are stable before any downstream phase consumes them. `no_semantic` flag is intentionally deferred to Phase 81 because it depends on the v1.10 Phase 65 SemanticLookup wiring being un-wired cleanly.

**Depends on**: Phase 75, v1.10 Phase 65 (SemanticLookup seam — read for context only)

**Requirements**: ABLATE-02, ABLATE-05, ABLATE-07, ABLATE-08

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 76)

<details>
<summary>✅ v1.11 Semantic Index Completion & P1 MCP Tools (Phases 68-74) -- SHIPPED 2026-06-07</summary>

- [x] Phase 68: Precise FileFactDiff Populator (5/5 plans) — completed 2026-05-13
- [x] Phase 69: Production Status Accessors (6/6 plans) — completed 2026-05-14
- [x] Phase 70: Incremental Refresh Overlay-Drain (7/7 plans) — completed 2026-05-15
- [x] Phase 71: P1 Single-Symbol Read Tools (5/5 plans) — completed 2026-05-17
- [x] Phase 72: P1 Cluster & Impact Tools (5/5 plans) — completed 2026-05-19
- [x] Phase 73: P1 Tools Integration & E2E Verification (4/4 plans) — completed 2026-05-21
- [x] Phase 74: Close gap — wire P1 tool accessors in production daemon (6/6 plans) — completed 2026-06-03

**Full details:** `.planning/milestones/v1.11-ROADMAP.md`

</details>

## Backlog

_No items in backlog._
