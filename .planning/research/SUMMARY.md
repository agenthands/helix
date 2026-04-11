# Project Research Summary

**Project:** Serena
**Domain:** Multi-oracle integration test harness for MCP/LSP code intelligence platform
**Researched:** 2026-04-11
**Confidence:** HIGH

## Executive Summary

Serena v1.4 is a test-only milestone layering a multi-oracle integration test harness on top of the proven v1.1 test infrastructure (23 test files, 19 golden files, multi-language fixtures). The approach is conservative: only 2 new Go module dependencies (`santhosh-tekuri/jsonschema/v6` for schema validation, `anthropics/anthropic-sdk-go` for LLM behavioral tests), five oracle layers as separate packages with independent build tags, and a strict "extend, don't replace" rule for the existing harness.

The dominant risk is breaking existing regression coverage while restructuring. The critical prerequisite is extracting shared harness code from `test/integration/` (which uses an external test package and can't be imported) into an importable `test/harness/` package. All five oracle layers depend on this extraction. The architecture research identifies a strict dependency chain: harness extraction → protocol oracle → contract oracle + fixtures → scenario oracle → CI pipeline → LLM behavioral → LLM judge.

The pitfalls research converges on four critical warnings: (1) don't modify existing harness API signatures, (2) separate LLM tests from deterministic tests via build tags from day one, (3) don't apply golden files to every tool×language combination (use assertion-based tests for dynamic outputs), and (4) don't over-engineer — "multi-oracle" is a mental model for file organization, not a runtime framework.

## Key Findings

### Recommended Stack

Only 2 new direct dependencies needed. Everything else extends existing infrastructure.

**Core technologies:**
- `santhosh-tekuri/jsonschema/v6` v6.0.2: JSON Schema Draft 2020-12 validation for tool contract testing — structured `ValidationError` output for clear test failure messages
- `anthropics/anthropic-sdk-go` v1.28.0+: Official Anthropic Go SDK for LLM behavioral tests — native `tool_use` support, build-tag gated (`//go:build llm`)
- `gotestsum` (CI-only): Structured JUnit XML reporting for CI pipeline — install in workflow, not in go.mod

**No new libraries needed for:**
- Test framework (stdlib `testing.T` + `testify` sufficient)
- Golden file management (extend existing `golden.go` pattern)
- Fixture management (extend existing `PrepareFixture`)
- MCP client testing (existing MCP SDK `NewInMemoryTransports()`)

### Expected Features

**Must have (table stakes):**
- Protocol compliance tests (MCP init, tool listing, session isolation, reconnect)
- Per-tool contract tests with golden outputs and schema validation
- Error shape assertions with categories (extend existing `errCase`)
- Data-driven scenario matrix across 7+ repository shapes
- Polyglot honesty rules (no fake cross-language links, no silent omissions)
- Profile/mode behavior tests (mode gating actually blocks/allows correctly)
- 5-stage CI pipeline (fast deterministic → scenarios → -race → LLM behavioral → LLM judge)
- Build tag separation (`integration`, `llm`, `llmjudge`)

**Should have (differentiators):**
- LLM behavioral tests (tool selection accuracy, disambiguation, output interpretation)
- LLM-as-judge transcript scoring with structured rubrics
- Worker pool stress scenarios (sustained load, circuit breaker trips, pressure eviction)
- Degraded subsystem simulation (selective LS/memory/skill failure injection)
- Test coverage matrix report

**Defer:**
- LLM score regression tracking over time
- Multiple LLM providers for judge
- Full MCPAgentBench reproduction

### Architecture Approach

Five oracle layers as separate Go sub-packages under `test/oracle/{protocol,contract,scenario,behavioral,judge}/`, each with its own build tag. No oracle layer imports another; all import from `test/harness/` (extracted from existing `test/integration/`). YAML-driven scenarios use `filepath.Glob` auto-discovery. Golden files scale via hierarchical subdirectories. LLM layers use double gating (build tag + env var).

**Major components:**
1. `test/harness/` — Importable shared infrastructure (extracted from `test/integration/`)
2. `test/oracle/protocol/` — MCP session lifecycle tests (no LS dependency)
3. `test/oracle/contract/` — Per-tool schema validation, golden outputs, error shapes
4. `test/oracle/scenario/` — YAML-driven multi-step scenarios across repo shapes
5. `test/oracle/behavioral/` — LLM tool selection and output interpretation tests
6. `test/oracle/judge/` — LLM-as-judge transcript scoring with rubrics
7. `testdata/fixtures/` — Extended with polyglot, unsupported, collision, degraded fixtures
8. `.github/workflows/integration-v2.yml` — 5-stage CI pipeline

### Critical Pitfalls

1. **Breaking v1.1 tests by restructuring harness** — Extract to `test/harness/` as a copy-and-adapt, keep `test/integration/` untouched until extraction proven
2. **Mixing LLM and deterministic tests in CI** — Three build tags (`integration`, `llm`, `llmjudge`) and 5 separate CI jobs from day one
3. **Golden file explosion** — Use goldens only for stable contract boundaries (tool lists, error shapes, response structure); assertion-based tests for dynamic outputs
4. **Over-engineering the framework** — No oracle registry, no plugin interfaces, no abstract factories. Helper functions + table tests + `testing.T` is the ceiling
5. **MCP SDK version brittleness** — Assert on behavior ("tool call succeeded with text containing X"), not on Go struct types

## Implications for Roadmap

### Phase 1: Harness Extraction & Foundation
**Rationale:** Critical prerequisite — existing `test/integration/` is an external test package that can't be imported by new oracle packages
**Delivers:** `test/harness/` with exported `StartTestDaemon`, `PrepareFixture`, `callTool`, golden helpers; build tag taxonomy; naming conventions
**Avoids:** Pitfall #1 (breaking v1.1 tests) by extracting, not modifying

### Phase 2: Protocol Oracle
**Rationale:** No LS dependency, fast, validates the extracted infrastructure works
**Delivers:** MCP init/shutdown, tools/list schema validation, session isolation, reconnect tests
**Uses:** `test/harness/`, existing InMemory + HTTP transports

### Phase 3: Contract Oracle & Fixtures
**Rationale:** Builds on protocol layer; establishes golden vs assertion boundary for all downstream work
**Delivers:** Per-tool contracts, error shape assertions, 5 new fixture directories (polyglot, unsupported, collision, degraded, empty)
**Uses:** `santhosh-tekuri/jsonschema/v6`, existing golden infrastructure

### Phase 4: Scenario Oracle
**Rationale:** Depends on fixtures + contract assertions as building blocks
**Delivers:** YAML-driven multi-step scenarios, polyglot honesty rules, degraded mode tests, profile/mode behavior tests
**Avoids:** Pitfall #3 (golden explosion) by using assertion-based tests for scenarios

### Phase 5: CI Pipeline
**Rationale:** Needs all deterministic layers to exist before staging them
**Delivers:** 5-stage GitHub Actions workflow, build tag gating, first signal under 3 minutes

### Phase 6: LLM Behavioral Oracle
**Rationale:** Needs all deterministic layers stable; API key gated, non-blocking
**Delivers:** Tool selection tests, disambiguation tests, output interpretation tests
**Uses:** `anthropics/anthropic-sdk-go`
**Avoids:** Pitfall #2 (LLM flakiness killing CI) via `//go:build llm` + env var skip

### Phase 7: LLM Judge Oracle
**Rationale:** Depends on behavioral tests producing transcripts; manual trigger only
**Delivers:** Structured rubric scoring, transcript quality assessment
**Avoids:** Pitfall #4 (over-engineering) by keeping judge simple

### Phase Ordering Rationale

- Harness extraction must be first — it's the foundation everything else builds on
- Protocol oracle validates infrastructure before adding complexity
- Contract + fixtures can partially parallelize but establish patterns for scenarios
- Scenarios consume fixtures + contracts, so they follow
- CI pipeline stages what exists
- LLM layers are last because they depend on stable deterministic layers and must never block earlier work

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 4:** YAML assertion vocabulary design, multi-step scenario state management
- **Phase 6:** Non-deterministic test strategies, LLM response caching, Claude model selection (Haiku vs Sonnet)

Phases with standard patterns (skip research-phase):
- **Phase 1:** Mechanical refactor, well-understood Go package patterns
- **Phase 2:** Standard protocol testing, MCP spec is well-documented
- **Phase 3:** Extends existing golden file patterns
- **Phase 5:** Standard GitHub Actions workflow

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Only 2 new deps, both verified on pkg.go.dev |
| Features | HIGH | Clear spec from user, v1.1 patterns to extend |
| Architecture | HIGH | Direct codebase analysis of 23 existing test files |
| Pitfalls | MEDIUM-HIGH | Codebase-specific risks well-analyzed; LLM test flakiness less certain |

**Overall confidence:** HIGH

### Gaps to Address

- YAML scenario assertion vocabulary: exact assertion types need design during Phase 4 planning
- LLM behavioral test statistical assertions: how many runs constitute passing (3/5? 4/5?)
- Polyglot fixture design: which languages in monorepo fixture (Go + Python + TypeScript is safest)
- Build tag interaction with `go test ./...`: verify all tags compose correctly
- Claude model for behavioral tests: Haiku 4.5 (cheap/fast) vs Sonnet 4.6 (accurate)

## Sources

### Primary (HIGH confidence)
- Serena codebase analysis: 23 test files, harness patterns, golden file infrastructure
- MCP Specification 2025-11-25: Protocol reference for compliance tests
- Go Wiki: TableDrivenTests: Canonical Go testing pattern

### Secondary (MEDIUM confidence)
- MCPAgentBench (arXiv 2512.24565): LLM agent MCP tool use metrics
- Langfuse/Arize LLM-as-judge: Judge patterns, 80-90% human agreement
- Janix-ai/mcp-validator: MCP protocol compliance testing reference
- Specmatic: MCP servers lying about schemas — motivation for schema validation

### Tertiary (LOW confidence)
- MCPVerse (arXiv 2508.16260v2): Expanded MCP benchmark — less directly applicable

---
*Research completed: 2026-04-11*
*Ready for roadmap: yes*
