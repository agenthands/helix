# Stack Research

**Domain:** Integration testing for Go MCP server with multi-language LSP fixtures
**Researched:** 2026-04-08
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `testing/synctest` | Go 1.25 stdlib | Deterministic concurrent test execution | GA in Go 1.25. Virtualizes time, controls goroutine scheduling. The MCP Go SDK itself uses it in `mcp_test.go`. Eliminates flaky timer-based waits in daemon lifecycle tests. Already available -- Go 1.25.1 is in use. |
| `mcp.NewInMemoryTransports()` | MCP Go SDK v1.5.0 | In-process MCP client-server transport | Already in go.mod. Returns paired transports connected via `net.Pipe()`. Connect server to one, client to the other -- full MCP protocol round-trips without sockets or HTTP. Zero new dependencies. |
| `mcp.Client` + `ClientSession.CallTool()` | MCP Go SDK v1.5.0 | MCP client for tool invocation in tests | The SDK's own client. `ClientSession.CallTool()` returns `*CallToolResult` with typed `Content` (TextContent, etc). Already a dependency. The SDK's `TestEndToEnd` uses exactly this pattern. |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions and requirements | Already in go.mod at this version. `require` for fatal checks, `assert` for soft checks. No suite -- Go subtests suffice. |
| `github.com/google/go-cmp` | v0.7.0 | Deep structural comparison with readable diffs | Already in go.sum as transitive dependency via MCP SDK. Promote to direct test dependency. Use for comparing complex tool result structures (symbol trees, reference lists) where testify's `Equal` produces unreadable output. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/tools/txtar` | latest | Text-based file archive for fixture definitions | Optional. When multi-file workspace fixtures need to be self-contained in a single test file. Gopls uses this pattern extensively. Lightweight -- just a parser, no framework. Consider only if fixture management becomes unwieldy. |
| `testing/fstest.MapFS` | Go 1.25 stdlib | In-memory filesystem for unit-level tests | For config/registry tests that don't need real disk. Not for LSP tests (language servers need real files on disk). |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go test -run TestIntegration -timeout 120s` | Run integration tests with adequate timeout | LSP startup takes 5-15s per language. Default 30s timeout will fail. Use `-short` flag to skip integration tests in quick dev loops. |
| `go test -count=1` | Disable test caching for integration tests | Tests with external LS processes should not be cached -- LS state is not deterministic across runs. |
| `-update` flag (custom) | Golden file update for tool output snapshots | Implement `var update = flag.Bool("update", false, "update golden files")` at package level. Standard Go pattern used by gopls, stdlib. |

## Integration Architecture

### Test Harness Wiring

The MCP Go SDK provides everything needed for the primary test pattern:

```
Test func --> mcp.Client --> InMemoryTransport --> SerenaMCPServer --> Daemon (kernel, pool, skills)
```

1. `mcp.NewInMemoryTransports()` creates a paired pipe (server transport, client transport)
2. Server transport connects to `SerenaMCPServer` via `Server.Connect()`
3. Client transport connects via `Client.Connect()`
4. `ClientSession.CallTool()` sends MCP requests, receives typed `*CallToolResult`
5. No sockets, no HTTP, no gRPC -- pure in-process with real MCP protocol framing

This is the exact pattern the MCP SDK uses for its own `TestEndToEnd`.

### Process Management for Language Servers

Language servers are real external processes (gopls, pyright, jdtls, rust-analyzer). No new dependencies needed:

| Concern | Approach | Why |
|---------|----------|-----|
| LS lifecycle | Let existing `lspool` manage LS processes | The worker pool already handles spawn, health check, circuit breaking, TTL. Don't reinvent for tests. |
| LS availability | `testing.Short()` skip + `exec.LookPath()` | Skip multi-language tests when LS not installed. Fail gracefully with `t.Skip("gopls not found")`. |
| Parallel safety | `t.Parallel()` with separate temp workspaces | Each test gets `t.TempDir()`. Pool handles concurrent LS access via share-until-dirty. |
| Cleanup | `t.Cleanup()` for daemon shutdown | Register cleanup in test setup. Daemon's existing signal-first shutdown handles kernel-first ordering. |
| Timeout | Per-test `context.WithTimeout` | 30s per tool call, 120s per test function. LS initialization is the bottleneck. |

### Fixture Strategy

**Reuse legacy fixtures.** `legacy/test/resources/repos/` contains 45 language fixture repos with known symbols. Copy the 4 target languages plus dogfood against Serena itself:

| Fixture Source | Language | What It Provides |
|----------------|----------|------------------|
| Serena's own codebase | Go | Dogfooding. 25,500 lines, 21 packages. Known symbols for every tool category. |
| `legacy/test/resources/repos/python/test_repo/` | Python | Models, services, utils with known class/function hierarchy. |
| `legacy/test/resources/repos/typescript/` | TypeScript | Type definitions, interfaces, modules. |
| `legacy/test/resources/repos/java/` | Java | Classes, interfaces, inheritance. |
| `legacy/test/resources/repos/rust/` | Rust | Structs, traits, impls. |

Copy to `testdata/fixtures/{lang}/` within the Go test package. Go's `testdata/` convention ensures `go build` ignores these files while `go test` can access them via relative paths.

## Installation

```bash
# Promote go-cmp from transitive to direct test dependency
go get github.com/google/go-cmp@v0.7.0

# Everything else is already in go.mod:
# - github.com/modelcontextprotocol/go-sdk v1.5.0 (Client, InMemoryTransport)
# - github.com/stretchr/testify v1.11.1
# - golang.org/x/sync v0.20.0 (errgroup for parallel fixture setup)

# Optional, only if txtar fixtures are adopted:
# go get golang.org/x/tools/txtar@latest
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `mcp.InMemoryTransport` | Streamable HTTP transport | When testing HTTP-specific behavior (CORS, session headers, auth). Not needed for tool correctness. |
| `mcp.InMemoryTransport` | Stdio transport via `os/exec` | When testing the full forwarder-daemon flow end-to-end. Slower, harder to debug. Use only for a single smoke test. |
| Go stdlib `testing` + subtests | `testify/suite` | Never for this project. Go subtests (`t.Run`) with table-driven patterns are simpler, more idiomatic, and match existing codebase conventions. |
| `testing/synctest` | Manual `time.Sleep` waits | Never. synctest eliminates flaky timing in concurrent tests. |
| Custom 20-line golden file helper | `sebdah/goldie` v2 | Only if golden file management becomes complex (50+ fixtures). Start simple. |
| Copy legacy fixtures to `testdata/` | Generate fixtures at test time | Only if fixtures need dynamic content (e.g., version-specific syntax). Static fixtures are simpler and reproducible. |
| `google/go-cmp` | `reflect.DeepEqual` | Never. go-cmp provides readable diffs, custom comparers for ignoring volatile fields (timestamps, IDs). |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `testcontainers-go` | Language servers are local binaries, not containers. Docker adds setup cost, CI complexity, no benefit for LSP testing. | Direct `lspool` management with `exec.LookPath` availability checks. |
| `rogpeppe/go-internal/testscript` | Designed for CLI command testing with shell scripts. MCP tool invocation is programmatic Go API calls, not shell commands. | Go subtests calling `ClientSession.CallTool()` directly. |
| `gomock` / `mockgen` | Integration tests must exercise real subsystems. Mocking the kernel or LS defeats the purpose of e2e testing. | Wire real daemon with real kernel. Skip when LS unavailable. |
| Custom JSON-RPC MCP client | Reimplementing MCP protocol framing is error-prone and unnecessary. | `mcp.Client` from the SDK speaks the exact protocol. |
| `httptest.Server` | HTTP layer complexity when in-memory transport exists. | `mcp.InMemoryTransport` -- zero network, real protocol. |
| `TestMain` re-exec pattern | Complex subprocess management for daemon startup. | In-process `daemon.New()` + InMemoryTransport. Existing `daemon_integration_test.go` proves this works. |
| `dgraph-io/ristretto` | Considered in v1.0 research but not adopted. Don't add for tests. | Standard `sync.Map` or mutex-guarded maps for any test-local caching. |

## Stack Patterns by Test Type

**Tool correctness tests (primary -- 38 tools):**
- `InMemoryTransport` + `Client.CallTool()` + `assert` on result content
- Fastest feedback loop, no process management, real MCP framing
- One daemon instance per test function, tools called sequentially

**Daemon lifecycle tests (shutdown, reconnect, goroutine leaks):**
- `daemon.New()` + `daemon.Run()` with `context.WithCancel` + goroutine counting
- Existing `TestE2ECleanShutdown` pattern, extend for reconnect scenarios
- Use `testing/synctest` for deterministic timing

**Multi-language fixture tests:**
- `testing.Short()` guard + `exec.LookPath()` for LS detection
- Table-driven: iterate languages, skip unavailable, test same tool matrix
- Separate `testdata/fixtures/{lang}/` directories

**Profile/mode filtering tests:**
- In-process `skill.ResolveTools()` assertions (no LS needed)
- Pure logic, no LSP round-trips, fast
- Already proven in `TestE2EProfileConfigLayering`

**Dogfooding tests (Serena's own codebase):**
- Point workspace at repo root, activate, exercise all 38 tools
- Requires gopls available -- skip with `testing.Short()`
- Golden file snapshots for symbol overview outputs

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| MCP Go SDK v1.5.0 | Go 1.25+ | Uses `testing/synctest` in its own tests. `InMemoryTransport` is stable API. `Client.Connect()` + `CallTool()` are the public testing surface. |
| `testing/synctest` | Go 1.25+ | Was experimental in 1.24 (`GOEXPERIMENT=synctest`). GA in 1.25 with `Test()` replacing `Run()`. |
| stretchr/testify v1.11.1 | Go 1.25 | Already in use across all existing test files. |
| google/go-cmp v0.7.0 | Go 1.25 | Already transitive dependency via MCP SDK. Promote to direct. |
| golang.org/x/tools/txtar | Go 1.25 | Minimal dependency (parser only). Optional. |

## Sources

- MCP Go SDK v1.5.0 source in module cache (`go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.5.0/mcp/`) -- verified `InMemoryTransport`, `Client`, `ClientSession.CallTool()`, `NewInMemoryTransports()` API directly. HIGH confidence.
- MCP Go SDK `mcp_test.go` line 61-64 -- confirmed `NewInMemoryTransports()` + `Client.Connect()` + `CallTool` end-to-end test pattern. HIGH confidence.
- MCP Go SDK `transport.go` lines 120-143 -- `InMemoryTransport` implementation using `net.Pipe()`. HIGH confidence.
- Existing Serena `internal/daemon/daemon_integration_test.go` -- confirmed in-process daemon construction, `skill.InitAll()`, `ExecuteTool()`, lifecycle testing patterns. HIGH confidence.
- Serena `go.mod` -- verified all dependency versions and existing transitive deps. HIGH confidence.
- [The Synctest Package (Go 1.25)](https://appliedgo.net/spotlight/go-1.25-the-synctest-package/) -- synctest GA status confirmed. HIGH confidence.
- [Testing concurrent code with testing/synctest](https://go.dev/blog/synctest) -- official Go blog on synctest design. HIGH confidence.
- [Gopls integration test framework](https://pkg.go.dev/golang.org/x/tools/gopls/internal/test/integration) -- txtar fixture pattern reference. MEDIUM confidence.
- [txtar package](https://pkg.go.dev/golang.org/x/tools/txtar) -- format specification. HIGH confidence.
- Legacy fixture repos at `legacy/test/resources/repos/` -- verified 45 language directories on disk. HIGH confidence.
- [Golden file testing in Go](https://ieftimov.com/posts/testing-in-go-golden-files/) -- `-update` flag pattern. HIGH confidence.

---
*Stack research for: Integration testing of Go MCP server with multi-language LSP fixtures*
*Researched: 2026-04-08*
