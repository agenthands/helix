# Feature Landscape: v1.4 Multi-Oracle Integration Test Harness

**Domain:** Multi-oracle integration test harness for MCP/LSP code intelligence server
**Researched:** 2026-04-11
**Scope:** Only features needed for the v1.4 milestone. Existing v1.1 test infrastructure (InMemory + HTTP harness, 38-tool dogfooding, multi-language fixtures, 19 profile goldens, three-tier concurrency, three-band error coverage) is the baseline; this milestone extends it with protocol, contract, scenario, behavioral, and judge oracle layers.

## Table Stakes

Features the test harness must have. Missing any of these means the multi-oracle architecture is incomplete.

### Protocol Compliance Testing

| Feature | Why Expected | Complexity | Depends On (v1.1) | Notes |
|---------|--------------|------------|-------------------|-------|
| MCP initialize/shutdown handshake tests | Protocol correctness is non-negotiable; Janix-ai/mcp-validator proves community demands this | Low | Existing InMemory + HTTP harness | Test capabilities negotiation, server info, protocol version in response. Both transports |
| tools/list schema validation | Specmatic research shows MCP servers commonly lie about their schemas; must validate all 38 tool inputSchemas match actual accepted args | Medium | Existing harness, tool registry | Parse tools/list response, validate each tool's inputSchema is valid JSON Schema Draft 2020-12, cross-reference against actual tool acceptance |
| Session isolation tests | Multiple clients must not see each other's state; critical for daemon model | Medium | Existing InMemory transport | Two concurrent sessions: activate different workspaces, verify tool results are scoped correctly |
| Reconnect/session lifecycle tests | Daemon survives client disconnects; core value prop needs testing | Medium | Existing harness | Connect, call tool, disconnect, reconnect, verify warm cache still works. Test both clean and abrupt disconnect |
| Error envelope shape validation | MCP spec defines error format; all errors must conform | Low | Existing error tests (30 cases) | Extend existing errCase to validate JSON-RPC error code, message structure, not just IsError bool |

### Per-Tool Contract Testing

| Feature | Why Expected | Complexity | Depends On (v1.1) | Notes |
|---------|--------------|------------|-------------------|-------|
| Golden output files for all 38 tools | Existing 19 profile goldens prove the pattern works; extending to tool response shapes catches regression | Medium | Existing `golden.go`, `-update` flag infrastructure | One `.golden` per (tool, fixture, scenario) triple. Reuse assertGolden + updateGolden pattern. Focus on structural shape, not dynamic content (paths, timestamps) |
| Response schema validation | Every tool response must match its declared output type (text content, structured data) | Medium | tools/list schema from protocol tests | Validate content type (text vs structured), field presence, nested structure. Use JSON Schema where structured output is declared |
| Error shape assertions with categories | v1.1 uses IsError bool; v1.4 must assert error categories (no_workspace, not_found, invalid_args, timeout, circuit_open) | Low | Existing `errCase` + `runErrCases` | Extend errCase struct with expectedErrCode/expectedErrSubstring fields. Foundation for future typed errors (TODO(#typed-errors)) |
| Idempotency contracts for read tools | Read tools (search_symbols, get_hover_info, etc.) must return identical results on repeated calls | Low | Existing tool tests | Call each read tool twice with same args, assert results match. Catches state leaks |

### Data-Driven Scenario Matrix

| Feature | Why Expected | Complexity | Depends On (v1.1) | Notes |
|---------|--------------|------------|-------------------|-------|
| YAML/Go-struct scenario definitions | Go table-driven pattern is standard; scenario files make the matrix reviewable and extensible | Medium | Existing table-driven patterns (errCase, symbols_test.go) | Each scenario: repo shape + tool sequence + expected outcomes. Go structs for type safety, YAML for large matrices |
| 7+ repository shape fixtures | v1.1 has 5 language fixtures; v1.4 needs polyglot monorepo, unsupported-language, name collisions, empty/malformed repos | Medium | Existing `testdata/fixtures/` + `PrepareFixture()` | New fixtures: polyglot-monorepo (Go+Python+TS), unsupported-only (e.g., Fortran), symbol-collision (same names across files), degraded-no-ls (fixture where required LS is absent), empty-repo |
| Cross-fixture tool coverage matrix | Every tool must be tested against every applicable fixture | Low | Scenario definitions + fixtures | Matrix of (tool x fixture x expected_outcome). Identifies coverage gaps during development |
| Multi-step scenario composition | Test realistic agent workflows: activate -> search -> read -> edit -> verify | Medium | All individual tool contracts stable | 5-10 realistic workflows as scenarios. Each is a sequence of tool calls with intermediate assertions. Catches state leaks between operations |

### Profile/Mode Contract Independence

| Feature | Why Expected | Complexity | Depends On (v1.1) | Notes |
|---------|--------------|------------|-------------------|-------|
| Mode-gated behavior tests | v1.1 goldens validate tool lists; v1.4 must validate that read-mode blocks edit calls, admin grants all | Medium | 19 existing golden files, mode_golden_test.go | Call blocked tools, assert proper rejection. Call allowed tools, assert success. Test mode transition rules |
| Profile-independent golden expectations | Golden files must be independent from runtime YAML to avoid self-approving bad changes | Low | Existing golden pattern (documented in D-01/D-02) | Already implemented in v1.1; v1.4 extends to tool response goldens with same independence principle |

### Polyglot Correctness Testing

| Feature | Why Expected | Complexity | Depends On (v1.1) | Notes |
|---------|--------------|------------|-------------------|-------|
| Cross-language honesty rules | Unique to Serena: verify no fake cross-language links, no silent omissions when LS is degraded | Medium | Multi-language fixtures, degraded-mode fixture | Test: when gopls absent, Go tools report degraded (not empty). When Python LS absent, no phantom Go symbols appear in Python fixture results |
| Degraded mode reporting | When LS unavailable, tools must explicitly report degraded status, not silently return partial/empty results | Medium | Degraded-no-ls fixture, daemon bootstrap | Verify error messages contain actionable information (which LS missing, how to install). No silent failures |
| Unsupported language handling | Tools called against unsupported language must fail clearly, not crash or hang | Low | Unsupported-only fixture | Test all tools against Fortran/COBOL fixture. Expect clean error, not panic |

### CI Pipeline Staging

| Feature | Why Expected | Complexity | Depends On (v1.1) | Notes |
|---------|--------------|------------|-------------------|-------|
| 5-stage pipeline definition | Mixed deterministic + non-deterministic tests need staging to prevent flaky-test fatigue and wasted CI budget | Medium | Existing bench.yml, pytest.yml | Stage 1: protocol+contract (always, fast). Stage 2: scenario matrix (always). Stage 3: -race concurrency (always). Stage 4: LLM behavioral (API-key gated). Stage 5: LLM judge (optional, report-only, never blocks merge) |
| Build tag separation for test tiers | `//go:build integration` already exists; add tags for `llm` and `llmjudge` | Low | Existing build tag pattern | `//go:build llm` for behavioral tests, `//go:build llmjudge` for judge tests. CI stages select by tag |
| API key gating for LLM stages | LLM tests must skip gracefully when ANTHROPIC_API_KEY (or equivalent) is absent | Low | LLM test infrastructure | `t.Skip("ANTHROPIC_API_KEY not set")` pattern. CI secret injection for authorized runs |

## Differentiators

Features that set the test harness apart. Not expected in typical MCP server test suites, but high value for Serena.

| Feature | Value Proposition | Complexity | Depends On (v1.1) | Notes |
|---------|-------------------|------------|-------------------|-------|
| LLM behavioral tests (tool selection accuracy) | Proves Serena's tool descriptions are machine-readable: given a coding task, does the LLM pick the right tool? MCPAgentBench shows this is frontier research | High | All 38 tools registered, profile system | Present task descriptions to an LLM, check it selects correct tool(s). Test disambiguation (search_symbols vs find_references). Metrics: invocation accuracy, tool selection accuracy. Non-deterministic, gated CI stage |
| LLM disambiguation tests | When multiple tools could apply, test that descriptions disambiguate correctly | High | LLM behavioral infrastructure | Pairs like (search_symbols vs get_symbol_overview), (find_references vs go_to_definition), (read_file vs get_hover_info). The LLM should choose correctly based on descriptions alone |
| LLM output interpretation tests | Test that tool outputs are LLM-parseable: given a tool result, can the LLM extract the answer? | High | LLM behavioral infrastructure | Feed real tool outputs to LLM, ask structured questions about them. Validates output format is machine-friendly |
| LLM-as-judge transcript scoring | Structured rubrics score end-to-end tool usage transcripts for quality (correctness, efficiency, completeness) | High | LLM behavioral tests producing transcripts | Point-wise rubric: Did agent use right tools? Avoid unnecessary calls? Interpret results correctly? 80-90% human agreement per Langfuse/Arize research. Never replaces deterministic assertions |
| Rubric-based scoring dimensions | Separate scoring for: tool_selection, argument_correctness, result_interpretation, efficiency, error_handling | Medium | LLM judge infrastructure | Each dimension scored 1-5 with reasoning. Aggregated per-tool and per-scenario. Drift tracking over time |
| Worker pool stress scenarios | Beyond v1.1's three-tier concurrency: sustained load with circuit breaker trips, pressure eviction, adaptive TTL | Medium | Existing concurrency_test.go | Scenarios: all workers busy + new request, circuit breaker open + recovery, memory pressure trigger, share-until-dirty under concurrent edits |
| Degraded subsystem simulation | Selectively disable LS, memory, or skills to verify graceful degradation | Medium | Daemon bootstrap, fail-fast/degraded-optional split | Inject failures: LS not found, memory DB corrupt, skill init failure. Verify daemon starts, tools report degraded status, no panics |
| Test coverage matrix report | Generate matrix showing (tool x fixture x scenario) coverage, identifying gaps | Low | Scenario + fixture infrastructure | Custom TestMain reporter or Go test output parser. Shows which tools lack polyglot coverage, which fixtures lack error cases |
| Regression snapshot for LLM scores | Track LLM judge scores over time to detect tool description quality regressions | Medium | LLM judge producing scores | Store scores as JSON per commit. Alert when any dimension drops > 1 point. Non-blocking but informative |

## Anti-Features

Features to explicitly NOT build.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| LLM tests as merge gates | Non-deterministic, API-key dependent, expensive ($0.10-1.00/run), flaky by nature. Would destroy CI reliability | Run LLM tests in gated stage, report-only, never block merge. Only deterministic stages block |
| Custom test framework / DSL | Go's testing package + testify is sufficient. Custom DSL adds learning curve and maintenance burden | Use table-driven Go tests with Go struct scenario definitions. YAML only for large data matrices |
| Mock language servers | Mocking LSP defeats Serena's core purpose -- real LS integration is the value prop. Mocks pass while real servers fail | Use real language servers with fixture repos. Skip tests when LS not installed (existing `requireGopls` pattern) |
| Snapshot testing for all tool outputs | Tool outputs contain timestamps, absolute paths, line numbers that change per environment. Snapshotting everything = constant golden churn | Use golden files only for stable outputs (tool lists, error shapes, response structure). Use structural assertions (contains, field presence, count) for dynamic outputs |
| Vendoring external MCP validators | Janix-ai/mcp-validator is Python-based, covers different protocol versions, adds external dependency. Would add complexity without proportional value | Build Serena-specific protocol tests in Go, covering the MCP operations Serena actually uses. Simpler, faster, type-safe |
| Fuzzing MCP inputs | Diminishing returns for a server consumed by trusted LLM clients, not adversarial web requests | Focus on contract testing with well-defined scenarios. Revisit if Serena ever faces untrusted input |
| Cross-process daemon testing | Running daemon as separate process adds IPC complexity, port conflicts, cleanup headaches in CI | Continue using in-process `StartTestDaemon` pattern from v1.1. HTTP transport smoke covers wire format |
| Full MCPAgentBench reproduction | Academic benchmark with 250+ tasks, custom sandbox, Docker orchestration. Overkill for validating 38 tools | Build targeted behavioral tests: 10-20 carefully chosen scenarios exercising Serena's actual tool descriptions and disambiguation edges |
| Multiple LLM providers for judge | Running judge with GPT-4, Claude, Gemini for comparison adds 3x cost and complexity | Use single LLM (Claude Sonnet) as judge. Switch if needed. One provider gives consistent scoring baseline |
| Human-in-the-loop test approval | Manual review gates for LLM test results block automation | LLM tests are fully automated with score thresholds. Human review only for score regressions flagged in PR comments |

## Feature Dependencies

```
Protocol compliance tests (no deps -- uses existing harness directly)
    |
    v
Per-tool contract tests + golden outputs (needs protocol layer stable)
    |
    +---> Error shape assertions upgrade (extends existing errCase, parallel work)
    |
    +---> Profile/mode behavior tests (extends existing goldens, parallel work)
    |
    v
Data-driven scenario matrix + new fixtures (needs contract tests as building blocks)
    |
    +---> Repository fixture management (parallel -- new fixtures feed scenarios)
    |         |
    |         +---> polyglot-monorepo fixture
    |         +---> unsupported-only fixture
    |         +---> symbol-collision fixture
    |         +---> degraded-no-ls fixture
    |         +---> empty-repo fixture
    |
    v
Multi-step scenario composition (needs scenarios + fixtures stable)
    |
    v
Polyglot honesty rules (needs multi-language fixtures + degraded simulation)
    |
    v
Degraded subsystem simulation (needs polyglot honesty as validation layer)
    |
    v
Build tags + CI stage 1-3 (needs all deterministic tests to exist)
    |
    v
LLM behavioral tests -- tool selection + disambiguation (needs deterministic layers stable)
    |
    v
LLM output interpretation tests (needs behavioral infra)
    |
    v
LLM-as-judge transcript scoring + rubrics (needs behavioral test transcripts)
    |
    v
5-stage CI pipeline complete (needs all test types to stage properly)
```

## MVP Recommendation

Prioritize (first 2-3 phases of milestone):

1. **Protocol compliance tests** -- Foundation layer. Validates MCP session lifecycle, tool listing schema, session isolation, reconnect. Low risk, high value, existing harness supports it directly. ~400 LOC.
2. **Per-tool contract tests with golden outputs** -- Extend existing golden infrastructure to cover all 38 tools' response shapes. Catches schema drift and regression. ~800 LOC.
3. **Error shape assertions upgrade** -- Low-cost extension of existing errCase to assert error categories/codes, not just IsError bool. ~200 LOC.
4. **Data-driven scenario matrix with new fixtures** -- YAML/struct-driven test cases across 7+ repository shapes. Reuses and extends existing table-driven patterns. ~600 LOC + fixture files.
5. **Polyglot honesty rules** -- Medium complexity but unique to Serena's value proposition. Proves cross-language integrity. No fake links, no silent omissions. ~500 LOC.
6. **Profile/mode behavior tests** -- Validate that mode gating actually works (read blocks edits, admin grants all). ~300 LOC.

Defer to later phases:

- **LLM behavioral tests**: Phase 3+. Requires all deterministic layers stable first. Needs API key infrastructure, cost management, non-deterministic test handling. HIGH value but HIGH complexity and long dependency chain. ~800 LOC.
- **LLM-as-judge scoring**: Phase 4+. Depends on behavioral tests existing and producing transcripts. Rubric design is research-heavy. Should never block earlier phases. ~600 LOC.
- **Worker pool stress scenarios**: Can be built incrementally on existing concurrency tests. Not blocking for harness MVP.
- **Test coverage matrix report**: Nice-to-have, build after the matrix exists to report on.
- **LLM score regression tracking**: Only meaningful after several LLM test runs exist.

## Complexity Budget

| Feature | Estimated LOC | New Test Files | New Fixtures | CI Changes |
|---------|--------------|----------------|--------------|------------|
| Protocol compliance | ~400 | 1 | 0 | 0 |
| Per-tool contracts + goldens | ~800 | 1-2 | 0 (uses existing) | 0 |
| Error shape upgrade | ~200 | 0 (extends existing) | 0 | 0 |
| Scenario matrix + driver | ~600 | 1 + scenario data | 3-5 new fixture dirs | 0 |
| Polyglot honesty | ~500 | 1 | 2 (degraded, collision) | 0 |
| Profile/mode behavior | ~300 | 1 (extends existing) | 0 | 0 |
| Build tags + CI stages 1-3 | ~100 | 0 (tag existing) | 0 | 1 workflow |
| LLM behavioral | ~800 | 1-2 | 0 | 1 CI stage |
| LLM judge + rubrics | ~600 | 1 | 0 | 1 CI stage |
| CI pipeline (5-stage) | ~200 | 0 | 0 | 1 workflow |
| **Total** | **~4,500** | **8-10 new files** | **5-7 new fixtures** | **2-3 CI files** |

## Dependencies on Existing v1.1 Test Infrastructure

| v1.1 Asset | How v1.4 Uses It | Extension Needed |
|------------|------------------|-----------------|
| `test/integration/harness.go` (StartTestDaemon, Options, TestDaemon) | Foundation for all v1.4 tests. Protocol, contract, scenario tests all start a TestDaemon | Add options for degraded-mode simulation (e.g., `DisableMemory`, `BlockLS`) |
| `test/integration/golden.go` (assertGoldenTools, -update flag) | Pattern reused for tool response goldens | Generalize to `assertGolden(t, name, actual string)` beyond just tool lists |
| `test/integration/harness.go` (PrepareFixture, projectRoot) | Fixture management for new repo shapes | No change needed; just add new fixture directories |
| `testdata/fixtures/{go,python,typescript,java,rust}` | Baseline language fixtures | Add 5 new fixtures alongside existing ones |
| `testdata/profiles/*.tools.golden` | Profile contract oracle | Extend pattern to `testdata/contracts/*.response.golden` for tool responses |
| `test/integration/errors_test.go` (errCase, runErrCases) | Error testing pattern | Extend errCase struct with `expectedErrCode` and `expectedErrSubstring` |
| `test/integration/helpers.go` (callTool, textContent) | Tool invocation helpers | No change needed |
| `test/integration/concurrency_test.go` | Concurrency test baseline | Optional: add stress scenarios alongside |
| `.github/workflows/bench.yml` | CI pipeline pattern | New workflow for integration test staging |
| InMemory + HTTP transports | Both transport paths | Protocol compliance tests must cover both |

## Sources

- [Janix-ai/mcp-validator](https://github.com/Janix-ai/mcp-validator) -- MCP protocol compliance testing reference, validates against 2025-06-18 spec
- [Specmatic: MCP servers lying about schemas](https://specmatic.io/demonstration/exposed-mcp-servers-are-lying-about-their-schemas/) -- Motivation for schema validation of tool declarations
- [MCP Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25) -- Protocol reference for compliance tests
- [MCPAgentBench](https://arxiv.org/abs/2512.24565) -- LLM agent MCP tool use benchmark with tool selection metrics
- [MCPVerse](https://arxiv.org/html/2508.16260v2) -- Expanded MCP benchmark covering Oracle/Standard/Max-Scale modes
- [Langfuse LLM-as-judge](https://langfuse.com/docs/evaluation/evaluation-methods/llm-as-a-judge) -- Judge patterns: point-wise scoring, rubric decomposition
- [Arize LLM-as-judge](https://arize.com/llm-as-a-judge/) -- Production deployment patterns for observation-level evaluators
- [Monte Carlo: LLM-as-Judge 7 Best Practices](https://www.montecarlodata.com/blog-llm-as-judge/) -- Criteria decomposition, bias mitigation, calibration
- [Confident AI: LLM Agent Evaluation](https://www.confident-ai.com/blog/llm-agent-evaluation-complete-guide) -- Tool selection accuracy, invocation accuracy, retrieval accuracy metrics
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) -- Canonical Go testing pattern
- [Eli Bendersky: File-driven testing in Go](https://eli.thegreenplace.net/2022/file-driven-testing-in-go/) -- Golden file and data-driven patterns
- [Parallel Table-Driven Tests in Go](https://www.glukhov.org/post/2025/12/parallel-table-driven-tests-in-go/) -- Loop variable capture, parallel subtest patterns
- [Berkeley Function Calling Leaderboard (BFCL) V4](https://gorilla.cs.berkeley.edu/leaderboard.html) -- Tool calling accuracy benchmarks for LLMs
