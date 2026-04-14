# Phase 18: Harness Extraction & Foundation - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Extract the existing test harness from `test/integration/` into an importable `test/harness/` package and establish the build tag taxonomy (`integration`, `llm`, `llmjudge`) so new oracle packages under `test/oracle/` can import shared test infrastructure and run under correct build tags. Existing `test/integration/` tests must continue to pass unchanged.

</domain>

<decisions>
## Implementation Decisions

### Package Extraction Strategy
- **D-01:** Copy and evolve — duplicate `harness.go`, `helpers.go`, `golden.go` from `test/integration/` into `test/harness/` as a new importable package. Do NOT import back from `test/integration/`.
- **D-02:** Rename aggressively in the new package — use clear concepts like `Runner`, `Transcript`, `GoldenStore` instead of preserving legacy naming. The new harness is a new testing product, not a refactored helper.
- **D-03:** Add tests for the harness package itself — it becomes a first-class package with its own contracts.
- **D-04:** Revisit convergence with `test/integration/` only after 2-3 iterations when the new package shape stops moving. Not before.

### Build Tag Taxonomy
- **D-05:** Three tags: `integration`, `llm`, `llmjudge`. Additive layers implemented via explicit boolean `//go:build` expressions (Go has no native tag inheritance).
- **D-06:** Tag expressions encode the hierarchy in each file:
  - Deterministic oracle tests: `//go:build integration || llm || llmjudge`
  - LLM behavioral tests: `//go:build llm || llmjudge`
  - Judge scoring tests: `//go:build llmjudge`
- **D-07:** CI result: `-tags=integration` runs deterministic only; `-tags=llm` runs deterministic + LLM; `-tags=llmjudge` runs all three layers.
- **D-08:** Do not rely on package naming or directory structure to imply tag inclusion — put the hierarchy in the `//go:build` lines explicitly.

### Oracle Package Layout
- **D-09:** Directory structure: `test/oracle/{layer}/` with each layer as its own Go package:
  - `test/harness/` — importable shared infrastructure
  - `test/oracle/protocol/` — MCP handshake, tools/list validation
  - `test/oracle/contract/` — per-tool goldens, schema validation
  - `test/oracle/scenario/` — multi-step agent workflows
  - `test/oracle/llm/` — LLM behavioral tests
  - `test/oracle/judge/` — LLM judge scoring
- **D-10:** Each oracle package imports `test/harness/` for shared helpers. No cross-imports between oracle packages.

### Backward Compatibility
- **D-11:** Zero changes to `test/integration/` — not a single line modified. Frozen as baseline suite.
- **D-12:** `test/integration/` keeps `//go:build integration` unchanged. New oracle packages use the boolean expression scheme alongside it.
- **D-13:** Duplication between `test/integration/` and `test/harness/` is a feature, not a bug — buys isolation while the new oracle system stabilizes.

### Claude's Discretion
- Internal package structure of `test/harness/` (file split, type names beyond the suggested concepts)
- Whether to use `testify` in the new harness or stay with stdlib `testing`
- Golden file directory layout under `testdata/`

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Test Infrastructure
- `test/integration/harness.go` — Current StartTestDaemon, PrepareFixture, WaitForLS implementation (source material for extraction)
- `test/integration/helpers.go` — Current callTool, textContent, callToolExpectError, listSessionTools (source material)
- `test/integration/golden.go` — Current assertGoldenTools with -update flag pattern (source material)

### Requirements
- `.planning/REQUIREMENTS.md` §Foundation — FOUND-01 (importable harness), FOUND-02 (build tag taxonomy)

### Project Decisions
- `.planning/STATE.md` §Accumulated Context — Multi-oracle architecture, extend-don't-replace, build tag gating decisions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `test/integration/harness.go`: `StartTestDaemon`, `Options`, `TestDaemon`, `PrepareFixture`, `WaitForLS` — copy as starting point for new harness
- `test/integration/helpers.go`: `callTool`, `textContent`, `callToolExpectError`, `listSessionTools` — copy and evolve
- `test/integration/golden.go`: `assertGoldenTools`, `-update` flag pattern — copy and generalize for per-tool golden outputs
- `test/bench/main_test.go`: separate benchmark harness pattern — shows how independent test packages coexist

### Established Patterns
- All integration files use `//go:build integration` tag — new files follow same convention with boolean expressions
- Package `integration_test` (external test package) — new harness will be regular package `harness` for importability
- Blank imports for skill registration (`_ "github.com/postfix/serena/internal/kernel/symbols"` etc.) — harness must replicate this
- `testify/require` used for assertions in helpers
- `tb.TempDir()` for test isolation, `tb.Cleanup()` for teardown

### Integration Points
- `daemon.New(cfg, logger)` + `daemon.Daemon.MCPServer()` — core daemon creation path
- `mcp.NewInMemoryTransports()` / `mcp.StreamableClientTransport` — both transport types
- `skill.InitAll(deps)` — skill initialization required before daemon
- `config.SerenaConfig` — test configuration structure
- `testdata/fixtures/{lang}/` — fixture source directory for PrepareFixture

</code_context>

<specifics>
## Specific Ideas

- New harness should have concepts like `Runner`, `ParityOracle`, `Transcript`, `GoldenStore` — not just copies of legacy helper names
- The harness will grow responsibilities around MCP transcript capture, schema/golden validation, and multi-runtime fixture orchestration — design for that evolution
- Harness package should have its own tests — it's a first-class testing product

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 18-harness-extraction-foundation*
*Context gathered: 2026-04-11*
