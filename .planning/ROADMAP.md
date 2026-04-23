# Roadmap: Serena

## Milestones

- [x] **v1.0 MVP** -- Phases 1-5 (shipped 2026-04-08)
- [x] **v1.1 Integration Testing** -- Phases 6-8 (shipped 2026-04-09)
- [x] **v1.2 Performance & Production Hardening** -- Phases 9-15 (shipped 2026-04-10)
- [x] **v1.3 Documentation Catchup** -- Phases 16-17 (shipped 2026-04-11)
- [x] **v1.4 Integration Testing v2** -- Phases 18-21 (shipped 2026-04-14)
- [x] **v1.5 Typed Errors & Hardening** -- Phases 22-24 (shipped 2026-04-15)
- [x] **v1.6 Context Intelligence & Resilient Editing** -- Phases 25-33 (shipped 2026-04-20)
- [x] **v1.7 Developer Experience & Auto-Setup** -- Phases 34-38 (shipped 2026-04-22)
- [ ] **v1.8 Documentation Overhaul** -- Phases 39-42 (in progress)

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

### v1.8 Documentation Overhaul (In Progress)

**Milestone Goal:** Rewrite all documentation to present Serena as its own Go-native product -- not a Python port -- reflecting 7 milestones of accumulated functionality, 41+ MCP tools, and unique capabilities.

- [x] **Phase 39: README Rewrite** - Rewrite README as standalone Go-native product identity document (completed 2026-04-23)
- [ ] **Phase 40: USAGE Refresh** - Update USAGE.md to reflect full current feature set through v1.7
- [ ] **Phase 41: Install & Contributing** - Update INSTALL.md and CONTRIBUTING.md for current codebase
- [ ] **Phase 42: Changelog & CLAUDE.md** - Audit CHANGELOG.md completeness and update CLAUDE.md to match reality

## Phase Details

### Phase 39: README Rewrite
**Goal**: README.md presents Serena as its own Go-native code intelligence platform with accurate capabilities, architecture, and quick start
**Depends on**: Nothing (first phase of v1.8)
**Requirements**: README-01, README-02, README-03, README-04, LEGC-01
**Success Criteria** (what must be TRUE):
  1. A reader encountering Serena for the first time understands it as a standalone Go product, not a Python port or rewrite
  2. The tool table lists all 41+ MCP tools with accurate current descriptions
  3. The architecture section reflects the full 4-layer stack including RepoMap, fuzzy editing, smart errors, and progressive descriptions
  4. The quick start section references `serena setup <client>` and lazy workspace initialization
  5. Python legacy is acknowledged briefly ("Originally inspired by") with no "port" or "rewrite" language anywhere in the document
**Plans:** 2/2 plans complete
Plans:
- [x] 39-01-PLAN.md -- Fix docgen missing imports and regenerate tool/language tables
- [x] 39-02-PLAN.md -- Rewrite README structure, content, and product positioning

### Phase 40: USAGE Refresh
**Goal**: USAGE.md comprehensively documents all features through v1.7 so users can discover and use every capability
**Depends on**: Phase 39
**Requirements**: USAGE-01, USAGE-02, USAGE-03
**Success Criteria** (what must be TRUE):
  1. v1.6 features are documented: fuzzy editing (4-strategy cascade, ellipsis support), RepoMap tools (get_repo_map, get_context), and 23 tree-sitter grammars
  2. v1.7 features are documented: `serena setup <client>`, get_health tool, Claude Code hooks, smart error suggestions, progressive tool descriptions, get_tool_help, lazy workspace init
  3. Troubleshooting section reflects current known issues (rust-analyzer rename, jdtls cold-start, gopls/Go 1.25 benchmark constraint) with workarounds
**Plans**: TBD

### Phase 41: Install & Contributing
**Goal**: INSTALL.md and CONTRIBUTING.md accurately guide new users and contributors through the current codebase
**Depends on**: Phase 39
**Requirements**: INST-01, INST-02, CONT-01, CONT-02
**Success Criteria** (what must be TRUE):
  1. INSTALL.md shows `serena setup <client>` as the primary installation method with current binary install paths
  2. INSTALL.md includes accurate MCP JSON configs for all 6 supported clients (Claude Code, Codex, VS Code, JetBrains, Claude Desktop, Gemini CLI) plus HTTP mode
  3. CONTRIBUTING.md reflects the current Go project structure (4-layer architecture, test/ directory, internal/ packages)
  4. CONTRIBUTING.md references the current test harness (oracle tests with protocol/contract/scenario/LLM layers, integration build tags, benchmark gates with benchstat)
**Plans**: TBD

### Phase 42: Changelog & CLAUDE.md
**Goal**: CHANGELOG.md accurately records all shipped milestones and CLAUDE.md reflects current project reality for AI assistants
**Depends on**: Phase 39
**Requirements**: CLOG-01, CLOG-02, CLMD-01, CLMD-02
**Success Criteria** (what must be TRUE):
  1. CHANGELOG.md has entries for all milestones v1.0 through v1.7 with dates and key accomplishments
  2. CHANGELOG.md v1.6 (RepoMap, fuzzy editing, 23 grammars) and v1.7 (setup CLI, health, hooks, smart errors, progressive descriptions, lazy init) entries are complete and accurate
  3. CLAUDE.md project description presents Serena as a standalone Go-native product matching the README identity
  4. CLAUDE.md architecture section includes RepoMap skill, fuzzy editing engine, setup CLI, health tools, and current tool count (41+)
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 39 -> 40 -> 41 -> 42

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
| 39. README Rewrite | v1.8 | 2/2 | Complete    | 2026-04-23 |
| 40. USAGE Refresh | v1.8 | 0/0 | Not started | - |
| 41. Install & Contributing | v1.8 | 0/0 | Not started | - |
| 42. Changelog & CLAUDE.md | v1.8 | 0/0 | Not started | - |
