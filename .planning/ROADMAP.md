# Roadmap: Serena 2.0

## Milestones

- ⏳ **v1.0 MVP** — Phases 1-5 (gap closure in progress)

## Phases

<details>
<summary>✅ v1.0 MVP Core (Phases 1-4) — Code complete 2026-04-08</summary>

- [x] Phase 1: Foundation (3/3 plans) — Daemon, MCP runtime, gRPC forwarder, HTTP transport
- [x] Phase 2: Code Intelligence Kernel (6/6 plans) — LSP codegen, worker pool, 9 symbol + 6 edit + 6 file + 3 diag tools
- [x] Phase 3: Multi-Language and Skills (5/5 plans) — 52 languages, memory system, skill interface
- [x] Phase 4: Agent Profiles and Configuration (3/3 plans) — 5 profiles, 4 modes, token budget

**Full details:** `.planning/milestones/v1.0-ROADMAP.md`

</details>

- [ ] **Phase 5: Daemon Bootstrap Integration** — Wire all Phase 2-4 components into daemon startup (gap closure)

## Phase Details

### Phase 5: Daemon Bootstrap Integration
**Goal**: The daemon binary wires all existing components (kernel, tools, skills, profiles) into a working runtime — closing the integration gap between individually-tested subsystems and the production entry point
**Depends on**: Phases 1-4 (all code exists, needs wiring)
**Requirements**: SYM-01..09, EDT-01..06, FIL-01..06, DGN-01..03, DMN-07..11, MEM-01..05, WFL-01..03, PRF-01..05, LNG-01..03, WRK-02..03
**Gap Closure**: Closes all gaps from v1.0 milestone audit
**Success Criteria** (what must be TRUE):
  1. Skill ToolDef RegisterFn actually registers tools with the MCP SDK; profile YAMLs reference skill names that exist in the registry; pool calls Installer.Resolve() for three-tier LS resolution
  2. Daemon creates langregistry.Registry, kernel.Kernel, imports skill packages for init(), calls skill.InitAll(), registers all kernel and skill tools with MCP server, installs ProfileFilterMiddleware, calls ResolveProfile(), and shuts down kernel/pool on exit
  3. All 6 E2E flows work: stdio transport with real tools, HTTP transport with real tools, symbol retrieval, memory operations, mode switching, --profile config layering
**Plans**: 3 plans

Plans:
- [ ] 05-01-PLAN.md — Fix component contracts (RegisterFn, profile skill names, pool-installer wiring)
- [ ] 05-02-PLAN.md — Daemon bootstrap wiring (kernel, skills, tools, middleware, config, shutdown)
- [ ] 05-03-PLAN.md — Integration smoke tests for all 6 E2E flows

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-07 |
| 2. Code Intelligence Kernel | v1.0 | 6/6 | Complete | 2026-04-08 |
| 3. Multi-Language and Skills | v1.0 | 5/5 | Complete | 2026-04-08 |
| 4. Agent Profiles and Configuration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 5. Daemon Bootstrap Integration | v1.0 | 0/3 | Pending | - |
