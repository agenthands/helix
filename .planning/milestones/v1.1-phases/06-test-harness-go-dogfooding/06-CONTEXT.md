# Phase 6: Test Harness + Go Dogfooding - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Build integration test infrastructure (harness, build tags, LS readiness, cleanup) and exercise all 38 MCP tool categories against Serena's own Go codebase. This phase delivers a working test harness and Go dogfooding suite — multi-language fixtures and advanced tests come in later phases.

</domain>

<decisions>
## Implementation Decisions

### Test Package Location
- **D-01:** Integration tests live in a new top-level `test/` package (black-box). Tests only exercise public APIs — the daemon must export necessary accessor methods (e.g., `MCPServer()`, `KernelInstance()`).
- **D-02:** Existing white-box tests in `internal/daemon/daemon_integration_test.go` remain as-is — they test internal behavior. New integration tests complement, not replace.

### MCP Client Wiring
- **D-03:** Use both `NewInMemoryTransports()` and `httptest.NewServer` + `StreamableClientTransport` in a layered approach.
- **D-04:** InMemory transport is the default for most tests (fast, in-process). HTTP transport used for a smoke test subset that validates the full serialization/HTTP path.

### Dogfood Fixture Strategy
- **D-05:** Tiered approach: small Go project in `testdata/fixtures/go/` for fast deterministic symbol/edit tests, plus full Serena codebase as a smoke test proving real-world scale.
- **D-06:** Small fixture has known, stable symbol names and positions for structural assertions. Full codebase smoke test uses behavioral assertions ("returns non-empty", "contains expected symbol name").

### Claude's Discretion
- Build tag naming convention (`//go:build integration` vs custom tag)
- LS readiness polling strategy (which tool call to use for detecting gopls readiness)
- Test helper API design (`StartTestDaemon(t, opts)` signature and options)
- Whether to promote `google/go-cmp` to direct dependency for test diffs

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Test Patterns
- `internal/daemon/daemon_integration_test.go` — Current integration test pattern with `newE2EConfig()`, `ExecuteTool()`, skill lifecycle
- `internal/daemon/bootstrap_test.go` — Tool registration count verification
- `internal/kernel/symbols/retrieval_test.go` — Mock-based unit test pattern for LSP responses

### Legacy Reference
- `legacy/test/conftest.py` — Language availability detection, `_determine_disabled_languages()`, LS lifecycle management
- `legacy/test/resources/repos/go/test_repo/` — Go fixture with known symbols (`main`, `Helper`, `DemoStruct`, `Value`)

### MCP SDK
- MCP Go SDK `NewInMemoryTransports()` — In-process client-server test transport
- MCP Go SDK `StreamableClientTransport` — HTTP-based client transport

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `newE2EConfig()` in `daemon_integration_test.go` — Config creation pattern with temp dirs and short socket paths
- `skill.InitAll(deps)` — Skill initialization with test-scoped temp directories
- `daemon.New(cfg, logger)` — In-process daemon construction
- `shortSocketPath()` pattern — Avoids macOS 104-byte Unix socket path limit

### Established Patterns
- `t.TempDir()` for fixture isolation (used in memory skill tests)
- `SkillToolExecutor` interface for direct tool invocation
- `stretchr/testify` for assertions (`require` + `assert`)

### Integration Points
- Daemon needs exported `MCPServer()` and `KernelInstance()` accessors (currently private fields)
- MCP server's `HTTPHandler()` can be wrapped in `httptest.NewServer`
- Worker pool lifecycle needs test-friendly configuration (low TTL, limited MaxWorkers)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches informed by research findings.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-test-harness-go-dogfooding*
*Context gathered: 2026-04-08*
