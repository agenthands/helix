# Roadmap: Serena 2.0

## Milestones

- :white_check_mark: **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- :construction: **v1.1 Integration Testing** — Phases 6-8 (in progress)

## Phases

<details>
<summary>:white_check_mark: v1.0 MVP (Phases 1-5) — SHIPPED 2026-04-08</summary>

- [x] Phase 1: Foundation (3/3 plans) — Daemon, MCP runtime, gRPC forwarder, HTTP transport
- [x] Phase 2: Code Intelligence Kernel (6/6 plans) — LSP codegen, worker pool, 9 symbol + 6 edit + 6 file + 3 diag tools
- [x] Phase 3: Multi-Language and Skills (5/5 plans) — 52 languages, memory system, skill interface
- [x] Phase 4: Agent Profiles and Configuration (3/3 plans) — 5 profiles, 4 modes, token budget
- [x] Phase 5: Daemon Bootstrap Integration (3/3 plans) — Full daemon wiring, 38+ callable MCP tools

**Full details:** `.planning/milestones/v1.0-ROADMAP.md`

</details>

### :construction: v1.1 Integration Testing (In Progress)

**Milestone Goal:** Prove all 38 MCP tools work end-to-end by testing against Serena's own codebase and multi-language fixtures.

- [ ] **Phase 6: Test Harness + Go Dogfooding** — Build integration test infrastructure and exercise all tool categories against Serena's own Go codebase
- [ ] **Phase 7: Symbol Editing + Multi-Language Fixtures** — Verify edit round-trips and validate cross-language symbol operations against known fixtures
- [ ] **Phase 8: Advanced Testing** — Profile filtering, mode visibility, concurrency safety, and error path coverage

## Phase Details

### Phase 6: Test Harness + Go Dogfooding
**Goal**: Developers can run integration tests that spin up a real daemon and verify all tool categories against Serena's own Go code
**Depends on**: Phase 5 (v1.0 complete)
**Requirements**: HARN-01, HARN-02, HARN-03, HARN-04, HARN-05, HARN-06, DOG-01, DOG-02, DOG-03, DOG-04, DOG-05, DOG-06
**Success Criteria** (what must be TRUE):
  1. Running `go test -tags integration ./...` executes integration tests while `go test ./...` skips them
  2. A single test helper call starts a daemon in-process, connects an MCP client, and returns a ready-to-use test context with LS readiness polling
  3. Symbol retrieval, file operations, diagnostics, memory, workflow, and profile tools all return correct results when called against Serena's own Go source
  4. Tests skip cleanly when gopls is not installed, enforce per-test timeouts, and leave no orphaned LS processes after teardown
**Plans:** 4 plans
Plans:
- [x] 06-01-PLAN.md — Test harness infrastructure: daemon accessors, MCP client wiring, Go fixture, self-test
- [x] 06-02-PLAN.md — Symbol retrieval (9 tools), file operations (6 tools), diagnostics (3 tools) integration tests
- [x] 06-03-PLAN.md — Memory (7 tools), workflow (2 tools), profile (2 tools) integration tests + HTTP transport smoke
- [x] 06-04-PLAN.md — Gap closure: fix Language field bug in workspace key, update tests to strict assertions

### Phase 7: Symbol Editing + Multi-Language Fixtures
**Goal**: Developers can verify that edit operations are correct round-trips and that symbol tools work identically across Python, TypeScript, Java, and Rust fixture projects
**Depends on**: Phase 6
**Requirements**: EDIT-01, EDIT-02, EDIT-03, EDIT-04, LANG-01, LANG-02, LANG-03, LANG-04, LANG-05
**Success Criteria** (what must be TRUE):
  1. An edit round-trip test reads a symbol body, replaces it, re-reads, and confirms the new body is returned
  2. Insert before/after, rename, and safe-delete operations produce correct file state verified by re-reading symbols
  3. Each of 4 language fixtures (Python, TypeScript, Java, Rust) has known symbols that symbol retrieval tools resolve correctly
  4. Cross-file reference chains in multi-file fixtures return the expected reference sets
**Plans:** 3 plans
Plans:
- [ ] 07-01-PLAN.md — Edit tool integration tests (replace, insert, rename, safe delete, verify) against Go fixture
- [ ] 07-02-PLAN.md — Python + TypeScript fixtures and symbol/edit integration tests
- [ ] 07-03-PLAN.md — Java + Rust fixtures and symbol/edit integration tests

### Phase 8: Advanced Testing
**Goal**: Developers can verify that profile filtering, mode visibility, concurrency, and error handling all behave correctly under test
**Depends on**: Phase 6
**Requirements**: ADV-01, ADV-02, ADV-03, ADV-04
**Success Criteria** (what must be TRUE):
  1. Each of 5 agent profiles exposes exactly its declared tool subset -- no extra tools, no missing tools
  2. Each of 4 modes filters tool visibility to match its mode definition
  3. Concurrent tool calls from multiple goroutines complete without races (passes `go test -race`) or deadlocks
  4. Calling a tool before workspace activation, on a nonexistent file, or for an unknown symbol returns a structured MCP error (not a panic or hang)
**Plans:** 4 plans
Plans:
- [x] 08-01-PLAN.md — Wave 1 foundation: extend harness Options (Profile/Mode/MaxWorkers), add listSessionTools helper, create golden.go with -update flag
- [x] 08-02-PLAN.md — ADV-01 + ADV-02 profile and mode contract tests via golden files, plus mode-switch freshness and excluded-tool-invocability security tests
- [x] 08-03-PLAN.md — ADV-03 three-tier concurrency: scenario stress + pool saturation fan-out + synctest pool unit test + Makefile test-stress target
- [x] 08-04-PLAN.md — ADV-04 three-band error coverage: representative category matrix, exhaustive destructive tool matrix, read-only smoke

## Progress

**Execution Order:**
Phases execute in numeric order: 6 -> 7 -> 8

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-07 |
| 2. Code Intelligence Kernel | v1.0 | 6/6 | Complete | 2026-04-08 |
| 3. Multi-Language and Skills | v1.0 | 5/5 | Complete | 2026-04-08 |
| 4. Agent Profiles and Configuration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 5. Daemon Bootstrap Integration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 6. Test Harness + Go Dogfooding | v1.1 | 3/4 | In Progress | - |
| 7. Symbol Editing + Multi-Language Fixtures | v1.1 | 0/3 | Not started | - |
| 8. Advanced Testing | v1.1 | 0/4 | Not started | - |
