# Feature Landscape

**Domain:** Integration testing for MCP code intelligence platform (Serena v1.1)
**Researched:** 2026-04-08

## Table Stakes

Features that must exist for the integration test suite to be credible and useful. Missing any of these means the test suite cannot fulfill its purpose.

| Feature | Why Expected | Complexity | Dependencies |
|---------|--------------|------------|--------------|
| MCP round-trip test harness | Core deliverable: spin up daemon, connect MCP client, call tools via `CallTool`, assert responses. Without this, nothing else works. | Med | Official MCP Go SDK `NewInMemoryTransports()` for in-process, or HTTP transport for out-of-process |
| Daemon lifecycle management in tests | Tests must start/stop the daemon cleanly. Already partially done in `daemon_integration_test.go` but needs extraction into reusable `testutil` package. | Low | Existing `daemon.New()` + `daemon.Run()` |
| Go dogfooding suite (self-test) | Exercise all 38+ tools against Serena's own codebase. The project has ~25,500 lines of Go with known symbols (`Daemon`, `SerenaMCPServer`, `Kernel`, etc.) -- the richest fixture available. | Med | gopls installed in test env, daemon harness |
| Multi-language fixture projects | Small projects with known symbols for Go, Python, TypeScript, Rust, Java. Legacy already has fixture repos under `test/resources/repos/` for 40+ languages -- port the 4-5 most important ones. | Med | Language servers installed (gopls, pyright/pylsp, tsserver, rust-analyzer, jdtls) |
| Tool correctness assertions | Each of the 38+ tools must have at least one test calling the tool against a known fixture and verifying the response shape and content. Not snapshot testing -- explicit assertions on known symbols. | High | Fixtures with stable, known symbol names/positions |
| Build tag / test tag separation | Integration tests are slow (real language servers). Must use `//go:build integration` so `go test ./...` skips them by default but CI runs them with `-tags integration`. | Low | None |
| Timeout and cleanup handling | LSP servers can hang. Tests need per-test timeouts, context cancellation, and process cleanup to avoid zombie language servers. | Low | Go `t.Deadline()`, `context.WithTimeout` |
| CI-compatible test execution | Tests must run in GitHub Actions with language servers installable via apt/brew/go install. Must handle missing LS gracefully (skip, not fail). | Med | `testing.Short()` or build tags, conditional skip logic |

## Differentiators

Features that elevate the test suite beyond basic correctness. Not expected for v1.1 MVP but high value.

| Feature | Value Proposition | Complexity | Dependencies |
|---------|-------------------|------------|--------------|
| MCP client-level e2e tests (full protocol) | Current tests use `ExecuteTool()` bypassing MCP wire protocol. True e2e calls `tools/call` via MCP client through `mcp.NewInMemoryTransports()` to verify JSON schema, arg parsing, error codes. | Med | Official MCP SDK client, in-memory transport |
| Snapshot / golden file testing for output stability | Capture tool outputs as golden files. Detect unintended response format changes. Legacy uses syrupy for this. Go equivalent: `testutil/golden` pattern or `go-cmp`. | Med | `go-cmp` (already in MCP SDK deps) or custom golden file helper |
| Profile-filtered tool visibility tests | Verify each profile (claude-code, codex, ci-bot, ide-assistant, full) exposes exactly the expected tool subset. Already partially tested in `TestE2EProfileConfigLayering`. | Low | Existing profile system |
| Worker pool stress testing | Start multiple concurrent tool calls against the same workspace. Verify share-until-dirty semantics, no races, no deadlocks. | High | `t.Parallel()`, `-race` flag |
| Cross-file reference chain testing | Call `find_symbol` -> get definition -> call `find_references` -> verify bidirectional link. Multi-file fixture needed. | Med | Multi-file fixtures with known cross-references |
| Edit round-trip testing | `get_symbol_overview` -> `replace_symbol_body` -> `get_symbol_overview` again -> verify edit took effect. Tests the full read-edit-read cycle. | Med | Tree-sitter body extraction, writable temp fixture copies |
| Diagnostic tool testing | Call `get_diagnostics` against a fixture with known errors. Verify error locations and messages. | Med | Fixtures with intentional syntax/type errors |
| Memory + workflow cross-skill integration | Write memory -> onboard project -> verify onboarding incorporates written memory. | Low | Existing memory/workflow skills |
| Transport-level tests (HTTP + gRPC) | Test MCP over Streamable HTTP and gRPC forwarder, not just in-memory. Catches serialization bugs. | High | HTTP server startup, gRPC forwarder startup |
| Language server availability checks | Auto-detect which language servers are installed, generate skip reasons. Follow legacy pattern from `conftest.py` `_determine_disabled_languages()`. | Low | `exec.LookPath()` checks |

## Anti-Features

Features to explicitly NOT build for v1.1.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Testing all 52 languages | Combinatorial explosion. Most LS behaviors are identical -- testing 52 languages adds CI cost with diminishing returns. | Test 5 representative languages: Go (dogfood), Python, TypeScript, Rust, Java. These cover the 4 tree-sitter-supported languages plus the most popular LS. |
| Mock language servers | Mocking LSP defeats the purpose. Integration tests must hit real language servers to catch real bugs (initialization sequences, capability negotiation, encoding quirks). | Use real language servers against small fixture projects. Mocks already exist in `retrieval_test.go` for unit tests. |
| Snapshot testing as primary strategy | Snapshots are brittle with LSP -- line numbers shift, URIs change, LS versions change output format. Updates become meaningless noise. | Use structural assertions: "response contains symbol named X at file Y" not "response equals this exact JSON blob". Use snapshots only for output format stability monitoring. |
| Performance benchmarking suite | Wrong milestone. v1.1 is about correctness, not performance. | Defer to v1.2 or later. Benchmarks need stable correctness tests first. |
| Fuzzing MCP inputs | Premature. Need working correctness tests before adversarial testing. | Defer. Focus on happy-path and known-error-path coverage. |
| Testing legacy Python code | Legacy is a reference, not active code. Testing it wastes effort. | Only port fixture repos and test patterns from legacy. Do not test Python Serena itself. |
| End-to-end agent conversation tests | Testing an LLM calling Serena tools in a loop is non-deterministic and expensive. | Test individual tool calls with deterministic inputs/outputs. |
| Per-tool unit test duplication | Existing unit tests (mock-based) in `retrieval_test.go`, `edit_test.go`, `fileops_test.go` already cover component logic. | Integration tests should focus on real LS round-trips. Do not duplicate what unit tests already verify. |

## Feature Dependencies

```
Build tags + test separation ──> Everything else (all integration tests need this)

Daemon lifecycle testutil ──> MCP round-trip harness ──> All tool tests
                          ──> Go dogfooding suite
                          ──> Multi-language fixture tests

Multi-language fixtures ──> Tool correctness assertions (for non-Go languages)
                       ──> Cross-file reference testing
                       ──> Edit round-trip testing
                       ──> Diagnostic tool testing

Go dogfooding (self-test) ──> No fixture dependency (uses Serena's own codebase)
                          ──> Depends on: daemon harness, gopls availability

MCP client-level e2e ──> mcp.NewInMemoryTransports() from SDK v1.5.0
                     ──> Daemon harness (to register tools on server side)

Profile visibility tests ──> Existing profile system (no new deps)

Edit round-trip tests ──> Temp dir fixture copies (writes must not mutate fixtures)
                      ──> Tree-sitter (already a dependency)

Worker pool stress ──> Daemon harness + multiple concurrent clients
```

## MVP Recommendation

### Phase 1: Foundation (must ship)

1. **Test harness with daemon lifecycle** -- Extract `newE2EConfig` pattern from `daemon_integration_test.go` into `internal/testutil/` package. Add `StartTestDaemon(t, opts)` that returns a connected test context with `ExecuteTool()` capability. Add `RequireLanguageServer(t, "gopls")` helper.
2. **Build tag separation** -- `//go:build integration` on all integration test files. Makefile target `make test-integration`. Keep existing unit tests running with plain `go test ./...`.
3. **Go dogfooding suite** -- Test all 38 tools against Serena's own Go codebase. This is the highest-value test because it uses the actual project (no fixture needed) and exercises gopls (the most critical LS). Covers: symbol retrieval (find `Daemon`, `Kernel`, `SerenaMCPServer`), symbol editing (replace body in temp copy), file ops (read/list/search own source), diagnostics (format own code), memory (write/read/delete), workflow (onboard own project), profile (switch modes, list tools).
4. **CI skip logic** -- `t.Skip("gopls not available")` when a required LS is missing. Tests degrade gracefully -- skip, never fail. Follow legacy pattern from `_determine_disabled_languages()`.

### Phase 2: Multi-language + correctness

5. **Port 4 fixture repos from legacy** -- Python, TypeScript, Rust, Java mini-projects from `legacy/test/resources/repos/`. Place in `testdata/fixtures/{lang}/`. Each fixture must have: known symbol names, known cross-file references, known parent-child relationships.
6. **Tool correctness assertions per language** -- One test per tool category (symbols, editing, fileops, diagnostics) per fixture language. Structural assertions: "definition of `Helper` is at `main.go`", "references to `DemoStruct` include `main.go` and `usage.go`".
7. **Cross-file reference testing** -- Multi-file fixtures where symbol A in file1 references symbol B in file2. Verify `find_references` returns both files. Verify `find_implementations` works for interfaces.

### Phase 3: Protocol-level + advanced

8. **MCP client-level e2e** -- Use `mcp.NewInMemoryTransports()` to test the full MCP wire protocol. Call `tools/call` through the SDK client, verify JSON-RPC framing, arg validation, error codes.
9. **Edit round-trip testing** -- Copy fixture to temp dir, call `replace_symbol_body`, re-read symbol, verify the edit. Tests tree-sitter body surgery end-to-end.
10. **Profile visibility tests** -- Each of 5 profiles exposes exactly the expected tool subset. Each of 4 modes filters correctly. Extend existing `TestE2EProfileConfigLayering`.

### Defer to v1.2+

- Snapshot / golden file testing: Add after correctness suite stabilizes and output formats settle.
- Worker pool stress tests: Needs correctness suite as baseline.
- Transport-level tests (HTTP/gRPC): Catches serialization issues, but MCP round-trip harness covers 90% of value.
- Performance benchmarks: Requires stable test infrastructure.
- Diagnostic tool testing with known errors: Requires crafted error fixtures per language.

## Sources

- Existing integration tests: `internal/daemon/daemon_integration_test.go` -- 6 e2e tests covering memory, mode switching, token budget, shutdown, profile layering, onboarding
- Legacy test infrastructure: `legacy/test/conftest.py` -- language-parameterized fixtures, LS lifecycle management, `_determine_disabled_languages()` skip logic, session-scoped LS fixtures
- Legacy fixture repos: `legacy/test/resources/repos/` -- 40+ language mini-projects with known symbols (Go fixture has `main`, `Helper`, `DemoStruct`, `Value`)
- Legacy snapshot tests: `legacy/test/serena/test_symbol_editing.py` -- syrupy-based snapshot testing for edit operations
- Official MCP Go SDK v1.5.0: `mcp.NewInMemoryTransports()` for in-process client-server testing, `mcp.NewClient()` + `c.Connect()` for client-side test setup
- Official MCP Go SDK examples: `mcp/client_example_test.go` -- shows roots, sampling, elicitation patterns via in-memory transport
- Existing unit tests: `internal/kernel/symbols/retrieval_test.go` -- mock-based LSP response testing with `testLease` pattern
- Go testing conventions: `//go:build integration` tags, `testdata/` directories, `testutil` helper packages, `t.Helper()`, `t.Cleanup()`
