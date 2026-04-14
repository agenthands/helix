# Requirements: Serena

**Defined:** 2026-04-11
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.4 Requirements

Requirements for Integration Testing v2. Multi-oracle test harness proving protocol correctness, contract stability, runtime safety, polyglot honesty, and LLM usability.

### Foundation

- [x] **FOUND-01**: Test harness extracted to importable `test/harness/` package with exported `StartTestDaemon`, `PrepareFixture`, `callTool`, and golden helpers
- [x] **FOUND-02**: Build tag taxonomy established (`integration`, `llm`, `llmjudge`) with documented naming conventions for oracle packages

### Protocol

- [x] **PROTO-01**: MCP initialize/shutdown handshake tests validate capabilities, protocol version, and server info across InMemory and HTTP transports
- [x] **PROTO-02**: tools/list response validated — all tool schemas are valid JSON Schema Draft 2020-12, names unique, descriptions non-empty
- [x] **PROTO-03**: Session isolation verified — concurrent sessions with different workspaces/modes do not affect each other
- [x] **PROTO-04**: Reconnect behavior tested — disconnect/reconnect preserves daemon state, recreates session state, no worker leakage

### Contracts

- [x] **CONT-01**: Every exposed MCP tool has a golden output file capturing normalized response shape
- [x] **CONT-02**: Every tool's declared inputSchema validated against JSON Schema Draft 2020-12 and cross-referenced against actual accepted arguments
- [x] **CONT-03**: Error responses assert stable error class/code, human-readable message, and no misleading success payload per category (no_workspace, not_found, invalid_args, timeout, circuit_open, unsupported)
- [x] **CONT-04**: Tool descriptions tested for LLM selectability — positive selection, disambiguation from neighboring tools, negative selection

### Scenarios

- [x] **SCEN-01**: Repository fixture matrix includes Go, Python, TypeScript, polyglot monorepo, unsupported language, degraded capability, and name collision fixtures
- [x] **SCEN-02**: Multi-step scenarios exercise realistic agent workflows (activate → search → read → edit → verify) with intermediate assertions
- [x] **SCEN-03**: Polyglot honesty rules enforced — no fake cross-language symbol links, no silent omissions, unsupported languages fail cleanly
- [x] **SCEN-04**: Profile/mode behavior tested — read mode blocks edit calls, admin grants all, switching updates tool visibility, concurrent sessions in different modes

### Runtime

- [x] **RUNT-01**: Worker pool stress tested under sustained load with circuit breaker trips, pressure eviction, and share-until-dirty under concurrent edits
- [x] **RUNT-02**: Clean shutdown drains in-flight work, releases workers, no goroutine leaks, no stuck LS subprocesses
- [x] **RUNT-03**: Degraded subsystem simulation — selective LS/memory/skill failure injection verifies daemon starts degraded, tools report status honestly, no panics

### LLM Behavioral

- [x] **LLM-01**: Tool selection accuracy — Claude selects correct tool given task description for every exposed tool
- [x] **LLM-02**: Disambiguation — Claude distinguishes similar tools (search_symbols vs find_references, get_symbol_overview vs explain_symbol) based on descriptions alone
- [x] **LLM-03**: Output interpretation — Claude correctly interprets tool results, distinguishes success/unknown/failure, does not hallucinate unsupported conclusions
- [x] **LLM-04**: LLM judge scores transcripts via structured rubrics (tool_choice, description_use, output_interpretation, uncertainty_handling, polyglot_reasoning) — manual trigger only, never blocks merge

## Future Requirements

### CI Pipeline

- **CI-01**: 5-stage GitHub Actions pipeline (protocol/contract → scenarios → race → LLM behavioral → LLM judge)
- **CI-02**: Build tag gating ensures `go test ./...` runs only deterministic tests by default
- **CI-03**: LLM stages skip gracefully when ANTHROPIC_API_KEY absent

### Metrics & Reporting

- **METR-01**: Test coverage matrix report showing (tool × fixture × scenario) coverage gaps
- **METR-02**: LLM score regression tracking over commits

## Out of Scope

| Feature | Reason |
|---------|--------|
| Custom test framework / DSL | Go testing + testify is sufficient; custom DSL adds maintenance burden |
| Mock language servers | Real LS integration is Serena's value prop; mocks pass while real servers fail |
| Snapshot testing for all tool outputs | Dynamic content (paths, timestamps) causes constant golden churn |
| External MCP validators (Janix-ai) | Python-based, different protocol versions, adds external dependency |
| Fuzzing MCP inputs | Diminishing returns for trusted LLM clients |
| Cross-process daemon testing | In-process StartTestDaemon is simpler and proven |
| Full MCPAgentBench reproduction | 250+ tasks overkill for 38 tools; targeted behavioral tests suffice |
| Multiple LLM providers for judge | Single provider (Claude) gives consistent scoring baseline |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 18 | Satisfied |
| FOUND-02 | Phase 18 | Satisfied |
| PROTO-01 | Phase 19 | Satisfied |
| PROTO-02 | Phase 19 | Satisfied |
| PROTO-03 | Phase 19 | Satisfied |
| PROTO-04 | Phase 19 | Satisfied |
| CONT-01 | Phase 19 | Satisfied |
| CONT-02 | Phase 19 | Satisfied |
| CONT-03 | Phase 19 | Satisfied |
| CONT-04 | Phase 19 | Satisfied |
| SCEN-01 | Phase 20 | Satisfied |
| SCEN-02 | Phase 20 | Satisfied |
| SCEN-03 | Phase 20 | Satisfied |
| SCEN-04 | Phase 20 | Satisfied |
| RUNT-01 | Phase 20 | Satisfied |
| RUNT-02 | Phase 20 | Satisfied |
| RUNT-03 | Phase 20 | Satisfied |
| LLM-01 | Phase 21 | Satisfied |
| LLM-02 | Phase 21 | Satisfied |
| LLM-03 | Phase 21 | Satisfied |
| LLM-04 | Phase 21 | Satisfied |

**Coverage:**
- v1.4 requirements: 21 total
- Satisfied: 21/21
- Mapped to phases: 21
- Unmapped: 0

---
*Requirements defined: 2026-04-11*
*Last updated: 2026-04-14 after v1.4 milestone audit*
