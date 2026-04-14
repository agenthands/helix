# Phase 19: Protocol & Contract Oracles - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Every MCP protocol interaction and every exposed tool has deterministic correctness assertions. This phase populates `test/oracle/protocol/` and `test/oracle/contract/` with tests covering handshake, capabilities, session isolation, reconnect, per-tool golden outputs, JSON Schema validation, error contracts, and description selectability heuristics. LLM-backed behavioral tests belong in Phase 20.

</domain>

<decisions>
## Implementation Decisions

### Protocol Test Scope
- **D-01:** Transport parity uses handshake + one representative tool call on both InMemory and HTTP transports. No full mirror — proves both paths work without duplicating the entire contract suite on HTTP.
- **D-02:** Session isolation tests workspace + mode isolation — two concurrent sessions with different workspaces and modes, verify tool results and mode restrictions apply independently. Not full state isolation (memory stores, worker assignments).
- **D-03:** Reconnect testing uses client-side disconnect/reconnect — drop MCP client session, create a new one against the same running daemon. Verify daemon state preserved, new session functional, no worker leakage. The common real-world scenario.

### Golden File Strategy
- **D-04:** Golden files capture normalized response shape — field names, types, nesting structure with placeholders for non-deterministic values (paths, timestamps, IDs). Tests assert shape matches, not exact bytes.
- **D-05:** Golden files live in `test/oracle/contract/testdata/golden/{tool_name}/`. Co-located with the contract oracle package, following Go `testdata/` convention.
- **D-06:** Error goldens organized by error category — one golden per category (no_workspace, not_found, invalid_args, timeout, circuit_open, unsupported) asserting error class/code and absence of misleading success payload. Not per-tool error goldens.

### Schema Validation
- **D-07:** Use `santhosh-tekuri/jsonschema/v6` for JSON Schema Draft 2020-12 validation. Compile the meta-schema once at test startup. Fail fast on invalid schemas.
- **D-08:** Validate both `inputSchema` and `outputSchema` (when declared) against the Draft 2020-12 meta-schema. Also validate actual structured tool results against declared `outputSchema` when present. Do not invent output schemas for tools that don't declare one — those rely on golden files and invariant checks.
- **D-09:** Keep instance-validation (tool results against outputSchema) and schema-validation (inputSchema/outputSchema against meta-schema) as separate test paths in the harness.

### Overlap with Existing Tests
- **D-10:** Oracle tests are the authoritative correctness layer. Existing `test/integration/` tests continue running unchanged (D-11 from Phase 18) as a parallel safety net. Over time they'll naturally become redundant but are never removed in v1.4.
- **D-11:** CONT-04 (tool description selectability) uses deterministic heuristics — assert descriptions contain action verbs, are unique across tools, disambiguate from neighboring tools. Pure string analysis, no LLM. LLM behavioral selectability tests come in Phase 20.

### Claude's Discretion
- Test helper structure within protocol/ and contract/ packages (file split, internal types)
- Whether to add schema validation helpers to `test/harness/` or keep them local to contract/
- Specific normalization strategy for golden file placeholders (regex, json path masks, etc.)
- How to structure the transport parity test (table-driven vs separate functions)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Harness Infrastructure (Phase 18 output)
- `test/harness/runner.go` — Runner, StartRunner, WaitForLS, ProjectRoot, DefaultTestConfig
- `test/harness/tools.go` — CallTool, TextContent, CallToolExpectError, ListSessionTools
- `test/harness/fixture.go` — PrepareFixture, RequireGopls, RequireLS
- `test/harness/golden.go` — GoldenStore, AssertGolden

### Existing Test Patterns
- `test/integration/smoke_http_test.go` — HTTP transport round-trip pattern
- `test/integration/profile_golden_test.go` — Per-profile golden tool list pattern
- `test/integration/errors_test.go` — Error category test pattern with tool name mapping
- `test/integration/concurrency_test.go` — Concurrent session test pattern

### Oracle Stubs (Phase 18 output)
- `test/oracle/protocol/doc.go` — Protocol oracle package stub
- `test/oracle/protocol/smoke_test.go` — Existing smoke test proving harness import chain
- `test/oracle/contract/doc.go` — Contract oracle package stub

### Requirements
- `.planning/REQUIREMENTS.md` §Protocol — PROTO-01 through PROTO-04
- `.planning/REQUIREMENTS.md` §Contracts — CONT-01 through CONT-04

### MCP SDK
- MCP Go SDK `mcp.Tool` type — InputSchema and OutputSchema fields
- `santhosh-tekuri/jsonschema/v6` — Draft 2020-12 validation library

### Phase 18 Context
- `.planning/phases/18-harness-extraction-foundation/18-CONTEXT.md` — Build tag taxonomy (D-05 through D-08), package layout (D-09, D-10), backward compatibility (D-11)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `test/harness/` package: Runner, CallTool, TextContent, GoldenStore — all importable, all tested
- `test/integration/errors_test.go`: Error category pattern with `errCase` struct — can inform contract error testing structure
- `test/integration/profile_golden_test.go`: Profile golden pattern — can inform per-tool golden structure
- Existing golden infrastructure: `-update` flag, `GOLDEN_UPDATE` env var, sorted output comparison

### Established Patterns
- Build tag: `//go:build integration || llm || llmjudge` for deterministic oracle tests
- External test packages: `protocol_test`, `contract_test` for cross-package import validation
- `testify/require` for assertions, `t.Run` subtests for organization
- `t.TempDir()` for isolation, `t.Cleanup()` for teardown, `t.Parallel()` for concurrency
- `t.Setenv()` for environment variable overrides in tests

### Integration Points
- `harness.StartRunner(t, opts)` — primary entry point for daemon + session
- `runner.NewHTTPSession(t)` — creates HTTP transport session against same daemon
- `runner.Session` — InMemory MCP client session
- `runner.Daemon` — access to daemon internals for state verification
- `mcp.CallToolParams` / `mcp.CallToolResult` — MCP tool call types
- `mcp.ListToolsParams` / `mcp.ListToolsResult` — tool listing with schema access

</code_context>

<specifics>
## Specific Ideas

- Schema validation should compile the Draft 2020-12 meta-schema once at startup, then validate each tool's inputSchema and outputSchema against it — fail fast on invalid schemas
- Keep instance-validation (results against outputSchema) and schema-validation (schemas against meta-schema) as separate concerns in the test code
- The user provided detailed rationale for santhosh-tekuri/jsonschema/v6 over alternatives — it passes the official JSON Schema Test Suite for drafts including 2019-09 and 2020-12
- Tool description selectability heuristics should cover: positive selection (right tool chosen), disambiguation (similar tools distinguished), negative selection (wrong tool rejected) — all via string analysis

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 19-protocol-contract-oracles*
*Context gathered: 2026-04-11*
