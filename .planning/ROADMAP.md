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

- [x] Phase 57: Semantic Store Foundation + Pipeline DAG Library (0/4 plans) (completed 2026-05-03)
- [x] Phase 58: v1.9 Carryover -- Release & Distribution (4/4 plans) (completed 2026-05-04)
- [x] Phase 59: Tree-sitter Extraction & Stable Symbol IDs (0/0 plans) (completed 2026-05-04)
- [x] Phase 59.1: drop-cgo-0-single-mode-cgo-1-build-release (0/6 plans, INSERTED) — Drop CGO=0 — single-mode CGO=1 build & release (completed 2026-05-04)
  Plans:
  - [x] 59.1-00-PLAN.md — Wave 0 pre-execution probes (darwin zig cc, release-smoke target, D-14 lock)
  - [x] 59.1-01-PLAN.md — Wave 1 stub deletion + tag strip + Path-3 platform stub + daemon step-6a removal
  - [x] 59.1-02-PLAN.md — Wave 2 goreleaser v2 split/merge probe + .goreleaser.yaml CGO=1 flip (linux+windows zig cc; darwin Apple clang) + partial:by_target stanza + Makefile zig guard
  - [x] 59.1-03-PLAN.md — Wave 3 release.yml split-runner rewrite: release-linux (4 archives, ubuntu-22.04, zig cc) + release-darwin (2 archives, macos-14, Apple clang, tag-gated per D-16) + release-merge (cosign uniformly per D-19, gh release create)
  - [x] 59.1-04-PLAN.md — Wave 4 doc sweep with split-runner amendments (CLAUDE/README/CONTRIBUTING/CHANGELOG/PROJECT/REQUIREMENTS/deferred-items/51.1-SUMMARY/v1.10-ROADMAP) + Gatekeeper workaround in INSTALL.md (D-19) + 3 NEW deferred items (DEF-59-NOTARIZE/DARWIN-SMOKE/DARWIN-CANARY)
  - [x] 59.1-05-PLAN.md — Wave 5 release-smoke finalization + per-runner Pass-1≡Pass-2 reproducibility verification on BOTH runners (HARD GATE; D-09/D-10 amended; cross-runner byte-equality NOT a gate per Pitfall 8)
- [x] Phase 60: Live Update Pipeline (0/6 plans) (completed 2026-05-05)
  Plans:
  - [x] 60-01-PLAN.md — Wave 0 vet-nokernel2semantic analyzer enforcing LIVE-07 invariant #1 (kernel→semantic boundary)
  - [x] 60-02-PLAN.md — Wave 1 schema migration v2→v3 (current_epoch + 4× write_epoch + 4 indexes) + overlay writer (BeginOverlayTx + MarkFileDeleted/MarkSymbolsDeleted/MarkReferencesDeleted/MarkEdgesDeleted) + per-tx epoch contract under -race stress
  - [x] 60-03-PLAN.md — Wave 1 EditNotifier interface in internal/kernel + 8-tool wiring (5 edit + 3 fileops) + non-blocking integration tests
  - [x] 60-04-PLAN.md — Wave 2 live spine: signal/classifier/coalescer/handler/service + ScheduleIncremental fill + lspqueue handoff (TDD: CoalesceEvents + MergeChange + ClassifyPathChange)
  - [x] 60-05a-PLAN.md — Wave 3 (parallel with 60-05b) per-workspace fsnotify watcher + ENOSPC fallback + LIVE-02 editor-fixture suite (Vim/JetBrains/VS Code)
  - [x] 60-05b-PLAN.md — Wave 3 (parallel with 60-05a) manifest scanner + 3 new config keys + bounded-label metric + pipelines/live.go fill + daemon bootstrap + Phase 60 close-out checkpoint
- [x] Phase 61: LSP Enrichment Worker (0/4 plans) (completed 2026-05-06)
  Plans:
  - [x] 61-01-PLAN.md — LeaseAcquirer + ForegroundBusy + 2-lane queue + handler producer rewiring + bulk-suppression API + nosemantic2kernel analyzer [ENRICH-01, ENRICH-02]
  - [x] 61-02-PLAN.md — Worker goroutine + §14.4 cascade engine + Budget enforcement + readiness-gate honoring + per-file overlay commit [ENRICH-02, ENRICH-03, ENRICH-04]
  - [x] 61-03-PLAN.md — Metrics + trace spans + Status() accessor + Manager + Daemon bootstrap + 2 new config keys [ENRICH-01..ENRICH-04]
  - [x] 61-04-PLAN.md — ENRICH-05 stress test + acceptance closeout + REQUIREMENTS.md check-off [ENRICH-05, ENRICH-01..ENRICH-04]
- [ ] Phase 62: Graph Engine, Ranking & Type Resolution (0/0 plans)
- [ ] Phase 63: Compaction & Retention (0/0 plans)
- [ ] Phase 64: New MCP Tools (P0 set of 4) (0/0 plans)
- [ ] Phase 65: Existing-Tool Integration (Strangler Fig) (0/0 plans)
- [ ] Phase 66: Agent Guardrails (G-001..G-005, warn-default) (0/0 plans)
- [ ] Phase 67: Evaluation Harness (0/0 plans)

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
| 57-67 | v1.10 | 19/24 | Planning | -- |

## Backlog

_No items in backlog._
