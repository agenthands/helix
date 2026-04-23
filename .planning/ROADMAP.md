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
- [x] **Phase 40: USAGE Refresh** - Update USAGE.md to reflect full current feature set through v1.7 (completed 2026-04-23)
- [x] **Phase 41: Install & Contributing** - Update INSTALL.md and CONTRIBUTING.md for current codebase (completed 2026-04-23)
- [x] **Phase 42: Changelog & CLAUDE.md** - Audit CHANGELOG.md completeness and update CLAUDE.md to match reality (completed 2026-04-23)
- [x] **Phase 43: Cross-Doc Truth Sync** - Close F-01/F-03/F-07/F-08/F-10/F-11 drift identified by v1.8 milestone audit (completed 2026-04-23)
- [x] **Phase 44: Re-Verify Phase 41 and USAGE-02** - Close F-13 orphan requirements and confirm Phase 43 fixes (completed 2026-04-23)
- [x] **Phase 45: Cross-link & Manual-Config Polish** - Close F-02/F-06/F-12 optional-but-actionable findings (completed 2026-04-23)

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
**Plans:** 3/3 plans complete
Plans:
- [x] 40-01-PLAN.md -- Add Feature Guide section (7 subsections) and update Tutorial 1 for setup CLI
- [x] 40-02-PLAN.md -- Add jdtls and gopls troubleshooting entries
- [x] 40-03-PLAN.md -- Fix RepoMap param name, fuzzy_edit example, add rust-analyzer rename troubleshooting (gap closure)

### Phase 41: Install & Contributing
**Goal**: INSTALL.md and CONTRIBUTING.md accurately guide new users and contributors through the current codebase
**Depends on**: Phase 39
**Requirements**: INST-01, INST-02, CONT-01, CONT-02
**Success Criteria** (what must be TRUE):
  1. INSTALL.md shows `serena setup <client>` as the primary installation method with current binary install paths
  2. INSTALL.md includes accurate MCP JSON configs for all 6 supported clients (Claude Code, Codex, VS Code, JetBrains, Claude Desktop, Gemini CLI) plus HTTP mode
  3. CONTRIBUTING.md reflects the current Go project structure (4-layer architecture, test/ directory, internal/ packages)
  4. CONTRIBUTING.md references the current test harness (oracle tests with protocol/contract/scenario/LLM layers, integration build tags, benchmark gates with benchstat)
**Plans:** 2/2 plans complete
Plans:
- [x] 41-01-PLAN.md -- Rewrite INSTALL.md with setup CLI as primary path
- [x] 41-02-PLAN.md -- Update CONTRIBUTING.md project structure and test documentation

### Phase 42: Changelog & CLAUDE.md
**Goal**: CHANGELOG.md accurately records all shipped milestones and CLAUDE.md reflects current project reality for AI assistants
**Depends on**: Phase 39
**Requirements**: CLOG-01, CLOG-02, CLMD-01, CLMD-02
**Success Criteria** (what must be TRUE):
  1. CHANGELOG.md has entries for all milestones v1.0 through v1.7 with dates and key accomplishments
  2. CHANGELOG.md v1.6 (RepoMap, fuzzy editing, 23 grammars) and v1.7 (setup CLI, health, hooks, smart errors, progressive descriptions, lazy init) entries are complete and accurate
  3. CLAUDE.md project description presents Serena as a standalone Go-native product matching the README identity
  4. CLAUDE.md architecture section includes RepoMap skill, fuzzy editing engine, setup CLI, health tools, and current tool count (41+)
**Plans:** 2/2 plans complete
Plans:
- [x] 42-01-PLAN.md -- Audit CHANGELOG.md and add v1.3-v1.7 milestone entries (CLOG-01, CLOG-02)
- [x] 42-02-PLAN.md -- Refresh CLAUDE.md Project + Architecture for v1.6/v1.7 subsystems (CLMD-01, CLMD-02)

### Phase 43: Cross-Doc Truth Sync
**Goal**: All public docs and CLAUDE.md agree with the source-of-truth code on setup-CLI client count (7), file-ops tool count (7), fuzzy-edit strategy names, Layer 3 label, and overall tool count.
**Depends on**: Phase 42
**Requirements**: README-04, USAGE-02, INST-01, INST-02, CLOG-01, CLOG-02, CLMD-01, CLMD-02
**Gap Closure**: Closes F-01 (critical), F-11, F-07, F-08, F-10, F-03 from `.planning/v1.8-MILESTONE-AUDIT.md`
**Success Criteria** (what must be TRUE):
  1. README, INSTALL, USAGE, CHANGELOG all enumerate 7 setup-CLI clients including `opencode`
  2. CLAUDE.md and README agree on file-ops tool count (7) and total tool count
  3. Fuzzy-edit strategy names identical across USAGE, CHANGELOG, CLAUDE.md
  4. CLAUDE.md Layer 3 label matches README exactly
  5. `internal/cli/setup_clients.go:36` comment says 7 registrars, not 6
**Plans:** 6/6 plans complete
Plans:
- [x] 43-01-PLAN.md — README.md client list, tool count, Layer 3 label, fuzzy strategy names
- [x] 43-02-PLAN.md — INSTALL.md Quick Start + Supported-clients enumeration (add opencode/generic)
- [x] 43-03-PLAN.md — USAGE.md Supported-clients list + fuzzy strategy vocabulary rewrite
- [x] 43-04-PLAN.md — CHANGELOG.md v1.7 Setup CLI bullet (6 → 7 clients, add OpenCode)
- [x] 43-05-PLAN.md — CLAUDE.md fileops count, Layer 3 client count, fuzzy strategy naming
- [x] 43-06-PLAN.md — internal/cli/setup_clients.go:36 comment (6 → 7 registrars)

### Phase 44: Re-Verify Phase 41 and USAGE-02
**Goal**: Phase 41 has a complete 41-VERIFICATION.md; Phase 40 USAGE-02 is re-verified after the strategy-naming and setup-CLI fixes land; milestone integration check is re-run.
**Depends on**: Phase 43
**Requirements**: USAGE-02, INST-01, INST-02, CONT-01, CONT-02
**Gap Closure**: Closes F-13 (critical) and resolves the 4 orphaned + 1 unsatisfied requirements from the v1.8 audit
**Success Criteria** (what must be TRUE):
  1. `.planning/phases/41-install-contributing/41-VERIFICATION.md` exists and passes INST-01, INST-02, CONT-01, CONT-02
  2. `40-VERIFICATION.md` is re-run with `gaps_found` resolved for USAGE-02
  3. Re-running the v1.8 integration check produces no remaining critical findings
**Plans:** 3 plans
Plans:
- [ ] 44-01-PLAN.md — Write 41-VERIFICATION.md from scratch (INST-01, INST-02, CONT-01, CONT-02)
- [ ] 44-02-PLAN.md — Re-run 40-VERIFICATION.md in place (flip 3 gaps to SATISFIED, status → passed)
- [ ] 44-03-PLAN.md — Re-run v1.8 integration check (phases_verified=[39,40,41,42], defer residuals to Phase 45)

### Phase 45: Cross-link & Manual-Config Polish
**Goal**: Non-blocking v1.8 integration warnings cleared — README manual-config has reasonable coverage, docs cross-link symmetrically, rust-analyzer troubleshooting metadata is current.
**Depends on**: Phase 43
**Requirements**: README-03, USAGE-03, INST-02
**Gap Closure**: Closes F-02, F-06, F-12 (warning/info severity) from the v1.8 audit
**Success Criteria** (what must be TRUE):
  1. README manual-config block either covers all 7 clients or explicitly points to INSTALL.md
  2. README links to CHANGELOG; USAGE links to INSTALL
  3. USAGE rust-analyzer troubleshooting metadata reflects current behavior
**Plans:** 1/1 plans complete
Plans:
- [x] 45-01-PLAN.md — F-02 README 7-client pointer + F-12 README→CHANGELOG and USAGE→INSTALL cross-links + F-06 rust-analyzer read-only verification

## Progress

**Execution Order:**
Phases execute in numeric order: 39 -> 40 -> 41 -> 42 -> 43 -> 44 -> 45

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
| 40. USAGE Refresh | v1.8 | 3/3 | Complete   | 2026-04-23 |
| 41. Install & Contributing | v1.8 | 2/2 | Complete   | 2026-04-23 |
| 42. Changelog & CLAUDE.md | v1.8 | 2/2 | Complete   | 2026-04-23 |
| 43. Cross-Doc Truth Sync | v1.8 | 6/6 | Complete   | 2026-04-23 |
| 44. Re-Verify Phase 41 and USAGE-02 | v1.8 | 0/3 | In Progress | — |
| 45. Cross-link & Manual-Config Polish | v1.8 | 1/1 | Complete   | 2026-04-23 |

## Backlog

### Phase 999.1: repomap returns lua fixture instead of go sources (BACKLOG)

**Goal:** Fix `get_repo_map` returning only `legacy/test/resources/repos/lua/test_repo/src/utils.lua` when called on Serena's own workspace. Expected: ranked view of `internal/` Go sources.
**Requirements:** TBD

**Repro:** Call `mcp__serena__get_repo_map` on `/Users/Janis_Vizulis/go/src/github.com/postfix/serena` — output surfaces a single deeply-nested Lua test fixture, zero Go files from `internal/`.

**Suspected causes (ordered):**
1. PageRank ranking broken/starved — no cross-file refs picked up, so ranker degenerates to lexicographic or leaf-weighted selection
2. Tag extractor only succeeded for Lua; Go queries failed silently
3. Workspace root resolution wrong
4. Default token budget too small, elide step nukes everything except one file

**Investigation path:** Read `internal/repomap/{pagerank,render,extractor,cache,graph,elide}.go`. Trace `get_repo_map` on Serena's own workspace with a large explicit token budget and focus path. Compare against aider's repomap output.

**Defer until:** after v1.8 Documentation Overhaul ships.

**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)
