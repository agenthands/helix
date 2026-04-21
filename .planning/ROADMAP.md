# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- ✅ **v1.2 Performance & Production Hardening** — Phases 9-15 (shipped 2026-04-10)
- ✅ **v1.3 Documentation Catchup** — Phases 16-17 (shipped 2026-04-11)
- ✅ **v1.4 Integration Testing v2** — Phases 18-21 (shipped 2026-04-14)
- ✅ **v1.5 Typed Errors & Hardening** — Phases 22-24 (shipped 2026-04-15)
- ✅ **v1.6 Context Intelligence & Resilient Editing** — Phases 25-33 (shipped 2026-04-20)
- 🚧 **v1.7 Developer Experience & Auto-Setup** — Phases 34-38 (in progress)

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

<details>
<summary>✅ v1.6 Context Intelligence & Resilient Editing (Phases 25-33) — SHIPPED 2026-04-20</summary>

- [x] Phase 25: Fuzzy Edit Engine (6/6 plans) — 4-strategy cascade, indentation preservation, ambiguity detection, ellipsis placeholders
- [x] Phase 26: Fuzzy Edit Integration (2/2 plans) — Standalone fuzzy_edit MCP tool, fallback wiring
- [x] Phase 27: RepoMap Tag Extraction & Cache (4/4 plans) — Tree-sitter extraction, SQLite cache, scope-aware elision
- [x] Phase 28: RepoMap Graph & MCP Tools (3/3 plans) — Cross-file reference graph, PageRank, token-budgeted MCP tools
- [x] Phase 29: Phase 25 Formal Verification (1/1 plan) — Verification gap closure
- [x] Phase 30: RepoMap Pipeline Wiring (2/2 plans) — Integration gap closure
- [x] Phase 31: Multi-language Grammar Expansion (4/4 plans) — 5→23 tree-sitter languages (full aider parity)
- [x] Phase 32: v1.6 Documentation Hygiene (1/1 plan) — Documentation gap closure
- [x] Phase 33: FallbackExtractor Wiring (2/2 plans) — LSP fallback + cache persistence

**Full details:** `.planning/milestones/v1.6-ROADMAP.md`

</details>

### v1.7 Developer Experience & Auto-Setup (In Progress)

**Milestone Goal:** Zero-friction onboarding with auto-detection, client hooks, real-time health visibility, and self-guiding tool surface for coding agents.

- [x] **Phase 34: Setup CLI Foundation** — One-command MCP registration for 6 clients with language detection and LS pre-install (completed 2026-04-21)
- [x] **Phase 35: Health & Status** — MCP health tool and CLI status command for workspace visibility (completed 2026-04-21)
- [ ] **Phase 36: Client Hooks** — Claude Code hook auto-installation for session lifecycle and tool nudging
- [ ] **Phase 37: Smart Error Responses** — Error enrichment middleware with parameter correction suggestions
- [ ] **Phase 38: Progressive Descriptions & Lazy Init** — Tiered tool descriptions, deep-dive help tool, and lazy workspace activation

## Phase Details

### Phase 34: Setup CLI Foundation
**Goal**: Users can register Serena with any supported coding agent in one command
**Depends on**: Phase 33
**Requirements**: SETUP-01, SETUP-02, SETUP-03, SETUP-04, SETUP-05, SETUP-06, SETUP-07, SETUP-08
**Success Criteria** (what must be TRUE):
  1. User can run `serena setup claude-code` and the MCP server appears in Claude Code's tool list without manual config editing
  2. User can run `serena setup <client>` for any of the 6 supported clients (claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic) and get a working MCP registration
  3. Setup detects project languages from the current directory and pre-installs available language servers automatically
  4. Setup uses client CLIs as subprocess (e.g., `claude mcp add-json`) rather than writing config files directly
**Plans**: 2 plans
Plans:
- [x] 34-01-PLAN.md — Setup command scaffold, 6 client registrars, output formatting, root wiring
- [x] 34-02-PLAN.md — Language detection, LS pre-install, health check, unit tests

### Phase 35: Health & Status
**Goal**: Agents and users can inspect workspace health and LS status at any time
**Depends on**: Phase 34
**Requirements**: HLTH-01, HLTH-02, HLTH-03, HLTH-04
**Success Criteria** (what must be TRUE):
  1. Agent can call `get_health` MCP tool and receive a list of active language servers with their status (running, crashed, indexing)
  2. `get_health` response includes per-workspace capabilities and indexing progress
  3. User can run `serena status` from the CLI and see a human-readable workspace health summary
  4. Health output defaults to error-only mode, surfacing only actionable failures unless verbose is requested
**Plans**: 2 plans
Plans:
- [x] 35-01-PLAN.md — Health data layer, get_health MCP tool, gRPC GetStatus RPC, daemon wiring
- [x] 35-02-PLAN.md — CLI status command with colored output, --json, --verbose flags

### Phase 36: Client Hooks
**Goal**: Claude Code sessions automatically activate Serena and guide agents toward symbolic tools
**Depends on**: Phase 34, Phase 35
**Requirements**: HOOK-01, HOOK-02, HOOK-03, HOOK-04
**Success Criteria** (what must be TRUE):
  1. Starting a new Claude Code session in a project with Serena setup triggers automatic workspace activation via SessionStart hook
  2. When an agent overuses grep/read for code navigation, PreToolUse hook nudges it toward Serena's symbolic tools (find_symbol, get_symbols_overview)
  3. Ending a Claude Code session triggers cleanup of session data via Stop hook
  4. Running `serena setup claude-code` installs all three hooks into Claude Code user settings automatically
**Plans**: 2 plans
Plans:
- [ ] 36-01-PLAN.md — Hook JSON helpers, nudge command, ClaudeCodeRegistrar hook integration, tests
- [ ] 36-02-PLAN.md — gRPC proto extension, activate/deactivate CLI commands, daemon handler, root wiring

### Phase 37: Smart Error Responses
**Goal**: Agents receive actionable parameter corrections when they misuse tools
**Depends on**: Phase 34
**Requirements**: SERR-01, SERR-02, SERR-03
**Success Criteria** (what must be TRUE):
  1. When an agent passes a wrong parameter name or value, the error response includes a "did you mean" suggestion with the correct parameter
  2. Error suggestions only correct parameters within the same tool — they never redirect to a different tool
  3. Smart error enrichment is implemented as middleware wrapping the existing typed error taxonomy, not modifying error kinds
**Plans**: 2 plans
Plans:
- [ ] 37-01-PLAN.md — [to be planned]
- [ ] 37-02-PLAN.md — [to be planned]

### Phase 38: Progressive Descriptions & Lazy Init
**Goal**: Tool surface is self-documenting for agents, and workspaces activate automatically on first use
**Depends on**: Phase 34, Phase 35
**Requirements**: DESC-01, DESC-02, DESC-03, LAZY-01, LAZY-02
**Success Criteria** (what must be TRUE):
  1. Tool listings show brief descriptions (under 100 tokens each), while detailed documentation is available on demand
  2. Agent can call `get_tool_help <tool_name>` and receive comprehensive documentation including usage examples and parameter details
  3. Description changes are gated by behavioral test regression — no description ships without passing tool selection tests
  4. If setup was not run, the first MCP tool call transparently triggers workspace activation before executing
  5. Concurrent first calls from multiple agents are safely serialized (no duplicate initialization or races)
**Plans**: 2 plans
Plans:
- [ ] 38-01-PLAN.md — [to be planned]
- [ ] 38-02-PLAN.md — [to be planned]

## Progress

**Execution Order:**
Phases execute in numeric order: 34 → 35 → 36 → 37 → 38

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
| 25. Fuzzy Edit Engine | v1.6 | 6/6 | Complete | 2026-04-16 |
| 26. Fuzzy Edit Integration | v1.6 | 2/2 | Complete | 2026-04-16 |
| 27. RepoMap Tag Extraction & Cache | v1.6 | 4/4 | Complete | 2026-04-16 |
| 28. RepoMap Graph & MCP Tools | v1.6 | 3/3 | Complete | 2026-04-17 |
| 29. Phase 25 Formal Verification | v1.6 | 1/1 | Complete | 2026-04-17 |
| 30. RepoMap Pipeline Wiring | v1.6 | 2/2 | Complete | 2026-04-17 |
| 31. Multi-language Grammar Expansion | v1.6 | 4/4 | Complete | 2026-04-20 |
| 32. v1.6 Documentation Hygiene | v1.6 | 1/1 | Complete | 2026-04-20 |
| 33. FallbackExtractor Wiring & Cache Persistence | v1.6 | 2/2 | Complete | 2026-04-20 |
| 34. Setup CLI Foundation | v1.7 | 2/2 | Complete   | 2026-04-21 |
| 35. Health & Status | v1.7 | 2/2 | Complete   | 2026-04-21 |
| 36. Client Hooks | v1.7 | 0/2 | Not started | - |
| 37. Smart Error Responses | v1.7 | 0/0 | Not started | - |
| 38. Progressive Descriptions & Lazy Init | v1.7 | 0/0 | Not started | - |
