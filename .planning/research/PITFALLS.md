# Domain Pitfalls: Multi-Oracle Integration Test Harness (v1.4)

**Domain:** Adding multi-oracle test architecture to existing Go code intelligence platform
**Researched:** 2026-04-11
**Overall confidence:** MEDIUM-HIGH (direct codebase analysis of existing 23 test files + authoritative Go testing docs + LLM evaluation research)

## Scope Note

These pitfalls are specific to **adding a multi-oracle test harness** to a codebase that already has:
- 23 integration test files under `test/integration/` with `//go:build integration`
- Working harness (`StartTestDaemon`, `callTool`, `callToolExpectError`, `PrepareFixture`, `WaitForLS`)
- Golden file pattern (19 `.golden` files under `testdata/profiles/`)
- Table-driven error tests (`errCase` struct + `runErrCases`)
- Multi-language fixtures (Go, Python, TypeScript, Java, Rust under `testdata/fixtures/`)
- Three-tier concurrency tests (scenarios + fan-out + testing/synctest)
- Build-tag gating (`//go:build integration`)
- MCP Go SDK v1.5.0 with InMemory + HTTP transport coverage
- CI benchmark pipeline in `bench.yml` with pinned gopls

The risk profile is **different from greenfield test architecture**: the existing tests are proven and catch real bugs. The primary danger is breaking what works while adding what's new.

---

## Critical Pitfalls

### Pitfall 1: Breaking v1.1 Tests by Restructuring the Harness

**What goes wrong:** Refactoring `harness.go`, `helpers.go`, or `golden.go` to support new oracle layers introduces regressions in the 23 existing test files. All files depend on `StartTestDaemon` (with its `Options` struct), `callTool`, `callToolExpectError`, `assertGoldenTools`, `PrepareFixture`, and `WaitForLS`. Changing any signature, semantic, or side effect silently breaks existing coverage.

**Why it happens:** The new oracle layers (protocol correctness, schema validation, LLM judge) feel like they need modifications to the core harness. The `Options` struct needs new fields. The assertion helpers need new capabilities. Developers treat the harness as malleable internal code rather than as a stable API consumed by 23 files across 7 test categories.

**Consequences:** Loss of v1.1 regression coverage. The `//go:build integration` tag means breakage only surfaces in CI or explicit local invocations, creating a long feedback gap. Profile golden tests, concurrency tests, and three-band error tests silently stop working. False confidence from new oracle tests passing while foundational tests are broken.

**Prevention:**
- Freeze the existing harness API surface. `StartTestDaemon`, `callTool`, `callToolExpectError`, `assertGoldenTools`, `PrepareFixture`, `WaitForLS`, `requireGopls`, `requireLS`, `listSessionTools`, `textContent`, `callToolExpectError` -- these are the API. New oracle layers compose ON TOP, never modify.
- New oracle infrastructure goes in new files (`oracle_protocol.go`, `oracle_schema.go`, `oracle_llm.go`) in the same `integration_test` package.
- If `Options` needs new fields (e.g., `SchemaValidation bool`), only ADD fields with zero-value defaults that preserve existing behavior. Zero value of a new `bool` field must mean "v1.1 behavior."
- Add a CI step that runs v1.1 test names explicitly (`-run 'TestHarness|TestSymbols|TestConcurrency|TestProfile|TestMode|TestErrors|TestEdit|TestFileops|TestMemory|TestWorkflow|TestDiag|TestSmoke'`) as a regression gate separate from new oracle tests.

**Detection:** Any existing test file that stops compiling or changes behavior after a harness modification. Monitor test count: v1.1 has a known number of subtests; if the count drops, something broke.

**Phase to address:** Phase 1 (foundation) -- establish the "extend, don't modify" rule before any new code.

---

### Pitfall 2: Mixing LLM-Dependent Tests with Deterministic Tests in CI

**What goes wrong:** LLM behavioral tests and LLM judge scoring run in the same CI job as deterministic protocol/contract tests. An LLM API outage, rate limit, model behavior change, or missing API key fails the entire pipeline. Developers learn to ignore CI failures ("it's just the LLM tests again"), destroying trust in ALL tests including the deterministic ones that actually protect against regressions.

**Why it happens:** The codebase uses a single build tag (`//go:build integration`) for all integration tests. Adding LLM tests with the same tag is the path of least resistance. Separate build tags feel like over-engineering. "We'll separate them later" becomes permanent technical debt.

**Consequences:** CI becomes unreliable. PRs merge with real regressions because CI is always red. LLM API costs accumulate from CI retries. Fork PRs can never pass CI (GitHub Actions doesn't expose secrets to fork PRs). Developer velocity drops as everyone waits for flaky CI.

**Prevention:**
- Use three build tags from day one: `//go:build integration` (deterministic), `//go:build llm` (behavioral, needs API key), `//go:build llmjudge` (judge scoring, manual only).
- CI pipeline structure:
  - Job 1: `go test -tags integration -run 'TestProtocol|TestContract'` -- fast (<3min), no LS needed, every push
  - Job 2: `go test -tags integration -run 'TestScenario'` -- medium (<10min), needs LS, PR only
  - Job 3: `go test -tags integration -race -run 'TestConcurrency'` -- medium, PR only
  - Job 4: `go test -tags llm` -- slow, non-blocking, `continue-on-error: true`, PR only
  - Job 5: `go test -tags llmjudge` -- manual `workflow_dispatch` only
- LLM test failures MUST NOT block PR merge. Configure GitHub required status checks to exclude jobs 4 and 5.
- LLM tests gracefully skip with `t.Skip("ANTHROPIC_API_KEY not set")` when the env var is missing.

**Detection:** Any workflow YAML that runs `go test -tags integration` AND `go test -tags llm` in the same job. Any required status check that includes LLM test results.

**Phase to address:** Phase 1 (foundation) -- build tag taxonomy and workflow YAML before any test code.

---

### Pitfall 3: Golden File Explosion from Combinatorial Test Dimensions

**What goes wrong:** v1.1 has 19 golden files (5 profiles x ~4 modes), which is a well-bounded matrix. v1.4 plans per-tool contract goldens, per-language scenario goldens, and schema validation goldens. With 38 tools x 5 languages x multiple response scenarios, the count could reach 500+ golden files. Each golden becomes a maintenance liability: SDK upgrades, tool output format changes, gopls version bumps, or new tool additions require mass golden updates that produce unreadable PR diffs.

**Why it happens:** The profile golden pattern works beautifully and developers want to apply it everywhere. Golden files are seductive -- they catch exact regressions. But the pattern works because the profile/mode matrix is small (19 combinations) and changes rarely (only when profiles are redesigned). Tool output and LS responses change frequently and are inherently variable across LS versions.

**Consequences:** `GOLDEN_UPDATE=1` becomes a cargo-cult command run without reviewing diffs. PRs that add a single tool touch 30+ golden files. False negatives proliferate as developers accept goldens that encode bugs. New contributors are overwhelmed by the golden file count.

**Prevention:**
- Golden files ONLY for stable contract boundaries: profile/mode tool lists (keep the existing 19), MCP protocol init response shape, tool listing schema. These change rarely and intentionally.
- For per-tool output validation, use ASSERTION-BASED tests following the existing `symbols_test.go` pattern: `assert.Contains(t, text, "main.go")`, `assert.GreaterOrEqual(t, len(lines), 2)`. This is how v1.1 already tests tool output -- preserve this pattern.
- For schema validation, use Go struct type assertions (`tc, ok := c.(*mcp.TextContent)`) which leverage the Go type system as a built-in schema. No JSON Schema files needed.
- Set a hard ceiling: maximum 40 golden files total in the repository. If approaching the limit, refactor the newest additions to assertions first.
- Tool response content goldens should be reserved for tools where EXACT output is contractually part of the API (e.g., `get_token_budget`). For most tools, "contains file path," "has N results," "includes type signature" assertions are superior.

**Detection:** Golden file count exceeding 30. Any golden file that needs updating on a gopls version bump. Any golden file containing language-server-generated content (hover text, diagnostics text).

**Phase to address:** Phase 2 (contract testing) -- define the golden vs. assertion boundary before writing contract tests.

---

### Pitfall 4: Over-Engineering the Test Framework

**What goes wrong:** Instead of writing tests, the team builds a test framework: an oracle registry with `Register(oracle)`, a plugin system for validators, a DSL for scenario definitions, configurable assertion pipelines, abstract factory pattern for test daemons. Weeks pass with zero tests written but an elegant architecture exists for hypothetically writing tests.

**Why it happens:** The "multi-oracle" framing naturally suggests a registry pattern -- "each oracle implements the Oracle interface." Go developers love interfaces. The v1.1 harness is well-structured (helper functions, table-driven tests), which creates competitive pressure to make v1.4 "even more elegant." Five oracle types feel like they need a unified abstraction.

**Consequences:** Test code becomes harder to understand than production code. New contributors can't figure out how to add a test without understanding the framework. The framework itself needs tests (meta-tests). Debug cycles double because failures might be in the framework or the code under test. The "multi-oracle" concept, which is a mental model for test organization, gets reified into runtime code.

**Prevention:**
- The v1.1 pattern IS the ceiling of abstraction: helper functions + table-driven tests + `testing.T`. No oracle registry. No plugin system. No interfaces for test infrastructure.
- Each oracle type is a Go file with test functions that call existing harness helpers. Schema validation is a function, not an interface. LLM judge is a function, not a provider.
- "Multi-oracle" is a MENTAL MODEL manifested as file naming and build tags, not as runtime architecture:
  - `protocol_test.go` -- protocol correctness tests
  - `contract_test.go` -- per-tool contract assertions
  - `scenario_test.go` -- repository scenario matrix
  - `behavioral_test.go` -- LLM behavioral tests (build tag: `llm`)
  - `judge_test.go` -- LLM judge scoring (build tag: `llmjudge`)
- Rule of thumb: if a new test helper takes more than 3 parameters, it's too complex. Break it into specific functions. The existing `callTool(tb, session, name, args)` at 4 params is the maximum.
- No file named `oracle.go`, `registry.go`, `framework.go`, or `provider.go` in test code. No interface with more than one implementation in test code.

**Detection:** Any PR that adds test infrastructure without adding test cases. Any test helper with more than 4 parameters. Any interface definition in `test/integration/`.

**Phase to address:** Phase 1 (foundation) -- explicitly document that oracle layers are file organization, not runtime abstractions.

---

### Pitfall 5: MCP Go SDK Version Brittleness in Protocol Tests

**What goes wrong:** Protocol correctness tests assert on exact MCP SDK response structures, field names, or Go types. When the MCP Go SDK ships v1.6 or v2.0 (the MCP spec itself had 4 breaking releases in 2024-2025: removal of JSON-RPC batching, new auth model, transport changes), tests break not because Serena is wrong but because assertions are over-coupled to SDK internals.

**Why it happens:** It's natural to write `require.IsType(t, &mcp.TextContent{}, result.Content[0])` or assert on specific struct field names. The Go type system makes this easy. But the MCP protocol spec evolves rapidly, and the Go SDK tracks it with breaking changes. The MCP 2026 roadmap indicates further spec evolution (Tasks primitive, new lifecycle semantics).

**Consequences:** SDK upgrades become multi-day test-fixing sprints. Teams pin old SDK versions to avoid test churn, falling behind on protocol features and security fixes. Protocol tests give false negatives after upgrades, undermining confidence.

**Prevention:**
- Test BEHAVIOR, not STRUCTURE. Assert "tool call succeeded and returned text containing X" (which is how `symbols_test.go` already works), not "response.Content[0] is *mcp.TextContent with Text field matching regex Y."
- The existing helper pattern is correct: `textContent(result)` abstracts away the SDK's content type hierarchy. Extend this pattern for new content types rather than reaching into SDK structs directly.
- For protocol correctness tests, assert on MCP protocol SEMANTICS: initialize returns capabilities, tools/list returns tools, tool call returns result with isError boolean. These are protocol-level contracts that survive SDK versions.
- Isolate SDK-version-sensitive assertions in ONE file (`sdk_compat_test.go`) so upgrades have a bounded blast radius.
- Test protocol correctness via HTTP transport (`NewHTTPSession`) which tests the JSON wire format -- the actual inter-process contract -- not Go struct layouts.

**Detection:** Any test that type-asserts on MCP SDK types beyond `*mcp.TextContent` and `*mcp.CallToolResult`. Any test that asserts on field names rather than field values.

**Phase to address:** Phase 2 (protocol/contract testing) -- define the assertion boundary with SDK versioning in mind.

---

## Moderate Pitfalls

### Pitfall 6: Data-Driven Scenario Test Table Bloat

**What goes wrong:** The repository scenario matrix (Go, Python, TypeScript, Java, Rust, polyglot monorepo, unsupported language, degraded capability, name collisions) is implemented as a single massive table-driven test with a struct that has 10+ fields: `name`, `language`, `fixture`, `requiresLS`, `lsBinary`, `setupFunc`, `skipCondition`, `expectedTools`, `timeout`, `assertFunc`, `degradedMode`. The table itself becomes harder to understand than individual test functions.

**Why it happens:** Table-driven tests are idiomatic Go, and the existing `errors_test.go` uses the pattern successfully with `errCase`. But error cases share identical setup (`StartTestDaemon` with an `Options`); scenario tests have fundamentally DIFFERENT setup per language (different LS binaries, different readiness signals, different fixture structures, different timeouts).

**Prevention:**
- Use table-driven tests ONLY when scenarios share identical setup. For the language matrix, write separate test functions: `TestScenario_Go`, `TestScenario_Python`, `TestScenario_TypeScript`, `TestScenario_Polyglot`, `TestScenario_Degraded`. This follows the pattern already established by `symbols_test.go`, `python_test.go`, `typescript_test.go`, `java_test.go`, `rust_test.go` -- which are ALREADY separate files.
- Within each function, use subtests for specific assertions (tools return data, cross-references work, diagnostics fire). This is already the established pattern.
- The polyglot monorepo and degraded-capability scenarios each deserve their own file -- they have unique fixture construction and unique assertions.
- Maximum table struct fields: 5. More than that means the scenarios are too heterogeneous for a table.

**Detection:** Any test struct with more than 6 fields. Any table where more than 30% of cases define `setupFunc` or `skipCondition`.

**Phase to address:** Phase 3 (scenario matrix).

---

### Pitfall 7: Polyglot Fixture Cross-Contamination

**What goes wrong:** Language servers write artifacts into fixture directories during indexing: `.cache/`, `__pycache__/`, `node_modules/.cache/`, `target/`, `.tsbuildinfo`, `.gopls/`. When tests run in parallel, two tests using the same language can see each other's LS artifacts through shared worker pool state. Tests pass individually but fail in combination, or produce different results depending on execution order.

**Why it happens:** `PrepareFixture` correctly copies fixtures to `tb.TempDir()`, and each test creates its own `TestDaemon`. But the worker pool's "share-until-dirty" semantics mean a warm LS worker from one test might serve another test's request if both use the same language. LS lock files (e.g., `.gopls/lock`) can conflict.

**Consequences:** Flaky tests that pass with `-count 1` but fail with `-count 2` or `-parallel 4`. Order-dependent results. Mysterious "file locked" errors. CI flakiness that's impossible to reproduce locally.

**Prevention:**
- Each scenario test MUST create its own `TestDaemon` with `MaxWorkers: 1` to prevent LS worker sharing across tests. The speed tradeoff is acceptable for correctness.
- The existing `PrepareFixture` pattern is correct -- preserve it strictly.
- For the polyglot monorepo fixture, construct it programmatically by copying per-language subdirectories into a single temp dir. Do not maintain a separate checked-in monorepo fixture.
- Set per-language LS timeouts: Go/gopls ~10s, Python/pyright ~10s, TypeScript/tsserver ~15s, Java/jdtls ~45s, Rust/rust-analyzer ~20s. Do not use a single 30s timeout for all.
- Add cleanup in fixture teardown to remove LS-generated directories before assertions that list files.

**Detection:** Test failures mentioning "file locked," "resource busy," or "EACCES." Tests that pass with `-parallel 1` but fail with higher parallelism.

**Phase to address:** Phase 3 (scenario matrix).

---

### Pitfall 8: LLM Behavioral Test Flakiness

**What goes wrong:** LLM behavioral tests (tool selection, disambiguation, output interpretation) produce different results across runs even with temperature=0. Model updates silently change behavior. Rate limits cause timeouts. Prompt sensitivity means minor wording changes flip results. The test suite becomes a coin flip.

**Why it happens:** LLMs are inherently non-deterministic. Even with temperature=0 and seed parameters, different hardware, request batching, and model version updates produce different outputs. Testing a probabilistic system with `require.Equal` assertions is a category error.

**Consequences:** Tests flap in CI. Developers add retry loops (tests become slow and expensive). "It passed 3 out of 5 times" is not a test result. LLM API costs accumulate. Developers disable the tests rather than fix them.

**Prevention:**
- Use RUBRIC-BASED scoring, not pass/fail. A behavioral test returns a score (0-5) against a structured rubric. Regression is detected by score declining below a threshold across multiple runs, not by exact match. This aligns with current LLM-as-judge best practices.
- Run each behavioral test 3 times minimum and use majority voting. Document the expected pass rate (e.g., "tool_selection should score >= 3/5 in at least 2 of 3 runs").
- Set score thresholds with margin: if expected is 4/5, alert at 2/5. Never alert at 3.9/5.
- Pin model version explicitly (`claude-sonnet-4-20250514`, not `claude-sonnet-4-latest`).
- Use temperature=0.0 and seed parameter where supported.
- NEVER make these blocking for PR merge. They are regression trend signals.
- Cache LLM responses keyed by (prompt_hash, model_version) during local development to avoid burning API credits on iteration.
- Use descending rubric order in prompts and explicit scoring examples (few-shot calibration) to reduce scoring variance.

**Detection:** Any `require.Equal` on LLM output text. Any retry loop around an LLM API call. Any LLM test in the required CI checks.

**Phase to address:** Phase 4 (LLM behavioral testing).

---

### Pitfall 9: Schema Validation That's Either Too Strict or Too Loose

**What goes wrong:** JSON Schema validation of tool responses catches too much (false positives: valid responses rejected because gopls changed its hover format) or too little (false negatives: corrupted responses pass because the schema only checks `isError`). Either outcome destroys trust in the schema validation layer.

**Why it happens:** Tool responses contain free-form text generated by language servers. The text format varies by LS version, language, codebase state, and platform. Schema validation that checks text patterns will break on every gopls upgrade. Schema validation that only checks structural fields misses real corruption.

**Prevention:**
- Validate STRUCTURE, not CONTENT: response has `content` array, elements have `type` field, text elements have non-empty `text`, `isError` is correct type. Do NOT validate: text matches a regex, content has exactly N elements, specific strings appear in text.
- Use Go struct type assertions as the schema (already the pattern in `helpers.go` with `textContent`). The Go type system IS a schema. `tc, ok := c.(*mcp.TextContent)` is a schema check.
- For tool-specific semantic validation, create assertion helpers: `assertDefinitionResult(t, result, expectedFile)` checks "text mentions a file path" not "text equals this exact string."
- Test the validators: write explicit test cases with known-good and known-bad responses to verify the validation catches corruption but tolerates format variance.
- Schema validation is a COARSE filter. It catches wire-format corruption and type mismatches. Semantic correctness is the job of assertion-based tests and behavioral tests, not schema.

**Detection:** A schema test that breaks on gopls upgrade. A schema that accepts empty content for tools that should always return results.

**Phase to address:** Phase 2 (contract testing).

---

### Pitfall 10: CI Pipeline Staging That Kills Feedback Speed

**What goes wrong:** The 5-stage pipeline is structured as sequential jobs where each waits for the previous. Stage 1 (protocol) takes 2 min, Stage 2 (scenarios with LS startup) takes 12 min, Stage 3 (concurrency + race) takes 10 min. Total serial time: 30+ minutes before any LLM tests even start. Developers push and wait.

**Why it happens:** `needs:` dependencies in GitHub Actions are over-specified. "Protocol should pass before scenarios" sounds logical but isn't necessary -- if protocol is broken, scenarios will also fail with clear errors.

**Prevention:**
- Stage 1 (protocol + contract, no LS): run on every push, <3 min. This is the fast feedback signal.
- Stage 2 (scenarios) and Stage 3 (concurrency + race): run in PARALLEL on PR, no `needs:` dependency on each other. Each takes ~10 min but they overlap.
- Stage 2 depends on Stage 1 (scenarios assume protocol works). Stage 3 does NOT depend on Stage 1 or 2.
- Stage 4 (LLM behavioral): runs in parallel with Stages 2-3, non-blocking.
- Stage 5 (LLM judge): manual trigger only.
- Each stage is a separate GitHub Actions JOB, not steps within a single job. This enables independent retry and parallel execution on separate runners.
- First CI signal (Stage 1) should arrive within 3 minutes of push. Total blocking CI time should be max(Stage 2, Stage 3) + Stage 1 = ~13 min, not sum.
- Use `concurrency:` groups to cancel superseded runs when new commits push.

**Detection:** PR feedback time exceeding 15 minutes. Any pipeline where all 5 stages run sequentially. Any stage that shares a runner with another stage.

**Phase to address:** Phase 1 (foundation).

---

## Minor Pitfalls

### Pitfall 11: Test Helper Import Cycles

**What goes wrong:** New oracle files import internal kernel packages directly, creating circular dependencies or violating the integration test principle of testing through the public MCP protocol layer.

**Prevention:** All test files must follow the existing import pattern: `daemon`, `config`, `skill`, `mcp` SDK, blank-import skill packages for `init()` registration. Never import `internal/kernel/*` directly. The point of integration tests is to test through the MCP tool interface. If a test needs kernel internals, it's a unit test and belongs next to the kernel code.

**Phase to address:** Phase 1.

---

### Pitfall 12: Language Server Version Drift Breaking Scenarios

**What goes wrong:** gopls v0.22 returns different hover text, pyright v1.2 changes diagnostic format, rust-analyzer changes symbol kinds. Scenario tests that assert on exact LS output break without any Serena code change.

**Prevention:** Pin LS versions in CI (already done for gopls at v0.21.1 in `bench.yml`). Apply the same pattern to all LS binaries used in scenario tests. Document pinned versions in a single env block in the workflow YAML. Bump LS versions deliberately in dedicated PRs.

**Phase to address:** Phase 3.

---

### Pitfall 13: Test Naming Collisions Across Oracle Layers

**What goes wrong:** Multiple oracle layers test the same tool: `TestSearchSymbols` (contract), `TestScenario_Go_SearchSymbols` (scenario), `TestProtocol_SearchSymbols` (protocol). Test output becomes confusing. `-run` patterns match the wrong tests.

**Prevention:** Strict naming convention: `Test{Oracle}_{Category}_{Specific}`. e.g., `TestProtocol_Init_ReturnsCapabilities`, `TestContract_SearchSymbols_ReturnsResults`, `TestScenario_Go_SearchSymbols_CrossPackage`. The oracle prefix is mandatory and unique: `Protocol`, `Contract`, `Scenario`, `Behavioral`, `Judge`.

**Phase to address:** Phase 1.

---

### Pitfall 14: LLM API Key Management in CI

**What goes wrong:** LLM test API keys leak into logs, get hardcoded in workflow files, or aren't available for fork PRs (GitHub doesn't expose secrets to fork PRs). Tests fail cryptically instead of skipping gracefully.

**Prevention:** LLM tests use `t.Skip` when env vars are unset: `if os.Getenv("ANTHROPIC_API_KEY") == "" { t.Skip("ANTHROPIC_API_KEY not set") }`. Use GitHub environment-scoped secrets, not repository secrets. Never log API responses containing potential key reflections. Document the required env vars in a test README.

**Phase to address:** Phase 4.

---

### Pitfall 15: Neglecting the Existing errCase Pattern for New Error Tests

**What goes wrong:** New contract and scenario tests reinvent error assertion patterns instead of reusing `errCase` + `runErrCases` from `errors_test.go` and `callToolExpectError` from `helpers.go`. Error testing becomes inconsistent across oracle layers.

**Prevention:** New oracle layers that test error paths MUST use `callToolExpectError` for individual assertions and should follow the `errCase` + `runErrCases` pattern for systematic error coverage. The three-band pattern (category + destructive + read-only) should be extended, not replaced.

**Phase to address:** Phase 2.

---

## Phase-Specific Warning Matrix

| Phase Topic | Likely Pitfalls | Mitigation |
|-------------|----------------|------------|
| Foundation / harness setup | #1 (breaking v1.1), #4 (over-engineering), #2 (build tag separation), #10 (CI staging), #13 (naming) | Freeze existing API, no oracle registry, file-level organization, 3 build tags, 5-job pipeline, naming convention |
| Protocol correctness | #5 (SDK brittleness), #9 (schema scope) | Test behavior not structure, struct validation not JSON Schema |
| Contract testing | #3 (golden explosion), #9 (schema false pos/neg), #15 (error patterns) | Cap goldens at 40, assertions for outputs, reuse errCase |
| Scenario matrix | #6 (table bloat), #7 (fixture contamination), #12 (LS version drift) | Per-language functions, isolated daemons, pinned LS versions |
| LLM behavioral | #2 (mixing with deterministic), #8 (flakiness), #14 (API keys) | Separate build tag, rubric scoring, graceful skip |
| LLM judge | #8 (flakiness), #4 (over-engineering) | Manual trigger only, simple scoring function, no framework |

## Warning Signs Matrix

| Symptom | Likely Pitfall | First Check |
|---------|---------------|-------------|
| Existing v1.1 test stops compiling | #1 (harness breakage) | `git diff test/integration/harness.go` -- was the API changed? |
| CI always red | #2 (mixed LLM/deterministic) | Are LLM tests in required checks? |
| PR touches 30+ golden files | #3 (golden explosion) | Should these be assertions instead? |
| Test infra PR with zero test cases | #4 (over-engineering) | Does the PR add an interface or registry? |
| Tests break on SDK bump with no Serena code change | #5 (SDK brittleness) | Are assertions on struct types or on behavior? |
| Table struct has 8+ fields | #6 (table bloat) | Should this be separate test functions? |
| Tests pass alone, fail together | #7 (fixture contamination) | Are tests sharing a daemon or temp dir? |
| LLM test passes 60% of the time | #8 (flakiness) | Is it using `require.Equal` on LLM output? |
| Schema rejects valid gopls response | #9 (false positive) | Is schema checking content or structure? |
| First CI signal takes >15 minutes | #10 (staging) | Are stages sequential or parallel? |

## Sources

- Direct analysis of Serena codebase: `test/integration/harness.go`, `golden.go`, `helpers.go`, `errors_test.go`, `concurrency_test.go`, `mode_golden_test.go`, `profile_golden_test.go`, `symbols_test.go`, `python_test.go`, `java_test.go`, `rust_test.go`, `typescript_test.go`
- Existing CI patterns: `.github/workflows/bench.yml` (gopls pinning, GOMAXPROCS pinning, pitfall mitigation comments)
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) -- HIGH confidence
- [Anti-patterns in Go testing](https://quii.gitbook.io/learn-go-with-tests/meta/anti-patterns) -- HIGH confidence
- [LLM-As-Judge: 7 Best Practices](https://www.montecarlodata.com/blog-llm-as-judge/) -- MEDIUM confidence
- [Testing AI Agents: Validating Non-Deterministic Behavior](https://www.sitepoint.com/testing-ai-agents-deterministic-evaluation-in-a-non-deterministic-world/) -- MEDIUM confidence
- [Calibrating Scores of LLM-as-a-Judge](https://www.godaddy.com/resources/news/calibrating-scores-of-llm-as-a-judge) -- MEDIUM confidence
- [LLM Testing Tools and Frameworks 2026](https://contextqa.com/blog/llm-testing-tools-frameworks-2026/) -- MEDIUM confidence
- [MCP Specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25) -- HIGH confidence
- [MCP 2026 Roadmap](https://blog.modelcontextprotocol.io/posts/2026-mcp-roadmap/) -- HIGH confidence
- [Rulers: Evidence-Anchored Scoring for LLM Evaluation](https://arxiv.org/html/2601.08654v1) -- MEDIUM confidence
- [On the Flakiness of LLM-Generated Tests](https://www.arxiv.org/pdf/2601.08998) -- MEDIUM confidence
