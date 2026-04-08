# Phase 6: Test Harness + Go Dogfooding - Research

**Researched:** 2026-04-08
**Domain:** Integration test infrastructure for MCP server with LSP-backed code intelligence
**Confidence:** HIGH

## Summary

This phase builds black-box integration test infrastructure that starts Serena's daemon in-process, connects an MCP client, and exercises all 38+ tool categories against Serena's own Go codebase. The MCP Go SDK v1.5.0 provides both `NewInMemoryTransports()` (fast, default) and `StreamableClientTransport` (HTTP, smoke tests) -- both verified in the SDK source. The existing codebase already has internal integration tests in `daemon_integration_test.go` that prove `daemon.New()` works in-process; the new tests extend this to use the full MCP protocol path via SDK client.

Two production code changes are required: exporting `MCPServer()` and `KernelInstance()` accessor methods on the Daemon struct (both fields exist as private). Everything else is additive -- new test packages, fixtures, and helpers. gopls v0.21.1 is installed and available. Go 1.25.1 provides `testing/synctest` at GA for deterministic concurrent testing.

**Primary recommendation:** Use `NewInMemoryTransports()` as the default test transport (fast, in-process, real MCP protocol). Add a small HTTP smoke test subset via `httptest.NewServer` + `StreamableClientTransport`. Build the harness in `test/` as a black-box package per D-01.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Integration tests live in a new top-level `test/` package (black-box). Tests only exercise public APIs -- the daemon must export necessary accessor methods (e.g., `MCPServer()`, `KernelInstance()`).
- **D-02:** Existing white-box tests in `internal/daemon/daemon_integration_test.go` remain as-is -- they test internal behavior. New integration tests complement, not replace.
- **D-03:** Use both `NewInMemoryTransports()` and `httptest.NewServer` + `StreamableClientTransport` in a layered approach.
- **D-04:** InMemory transport is the default for most tests (fast, in-process). HTTP transport used for a smoke test subset that validates the full serialization/HTTP path.
- **D-05:** Tiered approach: small Go project in `testdata/fixtures/go/` for fast deterministic symbol/edit tests, plus full Serena codebase as a smoke test proving real-world scale.
- **D-06:** Small fixture has known, stable symbol names and positions for structural assertions. Full codebase smoke test uses behavioral assertions ("returns non-empty", "contains expected symbol name").

### Claude's Discretion
- Build tag naming convention (`//go:build integration` vs custom tag)
- LS readiness polling strategy (which tool call to use for detecting gopls readiness)
- Test helper API design (`StartTestDaemon(t, opts)` signature and options)
- Whether to promote `google/go-cmp` to direct dependency for test diffs

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HARN-01 | Integration tests use `//go:build integration` tag so `go test ./...` skips them by default | Go build tags are standard; `integration` is the conventional name. Tests gated with `//go:build integration` at file top. |
| HARN-02 | Reusable test harness starts daemon in-process via `daemon.New()` and returns connected test context | `daemon.New(cfg, logger)` returns `*Daemon` with all subsystems wired. Need two exported accessors: `MCPServer()`, `KernelInstance()`. Harness wraps this with MCP client connection. |
| HARN-03 | Test harness supports full MCP protocol round-trips via `NewInMemoryTransports()` or `StreamableClientTransport` | Both verified in MCP SDK v1.5.0 source. InMemory uses `net.Pipe()`, HTTP uses `httptest.NewServer` + `StreamableClientTransport{Endpoint: url}`. |
| HARN-04 | Tests skip gracefully when required language server is not installed (`t.Skip`) | `exec.LookPath("gopls")` checks availability. Harness helper calls `t.Skip("gopls not found")`. |
| HARN-05 | Tests enforce per-test timeouts and clean up LS worker processes on teardown | `context.WithTimeout` per test. `t.Cleanup()` calls daemon shutdown which runs `pool.stopAll()` -> sends LSP `shutdown`/`exit` to all workers. |
| HARN-06 | LS readiness polling waits for language server to finish indexing before assertions | Poll `search_symbols` or `get_symbol_overview` with a known symbol name until success or timeout. 100ms poll interval, 30s timeout for first init. |
| DOG-01 | All 9 symbol retrieval tools return correct results against Serena's own Go codebase | Fixture has known symbols (structs, functions, methods). Full codebase smoke uses behavioral assertions. gopls v0.21.1 available. |
| DOG-02 | All 6 file operation tools work correctly against Serena's own source files | File ops don't need LSP -- work against any directory. Test against fixture and codebase. |
| DOG-03 | All 3 diagnostic tools return valid results against Serena's own Go code | `get_diagnostics`, `get_code_actions`, `format_code` require active LS. Test after readiness. |
| DOG-04 | Memory tools (write, read, list, search, edit, rename, delete) work end-to-end | No LSP needed. Existing `TestE2EMemoryToolInvocation` pattern via MCP client instead of direct executor. |
| DOG-05 | Workflow tools (onboard, prepare handoff) execute successfully against own codebase | No LSP needed. Onboarding scans project directory. |
| DOG-06 | Profile tools (switch mode, get token budget) function correctly | No LSP needed. Tests mode switching and token budget via MCP client. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.5.0 | MCP client (`NewClient`, `ClientSession.CallTool`) + transports (`NewInMemoryTransports`, `StreamableClientTransport`) | Already in go.mod. SDK's own tests use identical pattern. [VERIFIED: go.mod + SDK source in module cache] |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions (`require` for fatal, `assert` for soft) | Already in go.mod, used across all existing tests. [VERIFIED: go.mod] |
| `testing/synctest` | Go 1.25 stdlib | Deterministic concurrent test execution | GA in Go 1.25. MCP SDK itself uses it. Eliminates flaky timer waits. [VERIFIED: Go 1.25.1 installed, SDK source] |
| `net/http/httptest` | Go stdlib | Test HTTP server for StreamableClientTransport smoke tests | Standard library. Wraps `daemon.MCPServer().HTTPHandler()`. [VERIFIED: stdlib] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/google/go-cmp` | v0.7.0 | Deep structural comparison with readable diffs | For complex tool results (symbol trees, reference lists) where testify `Equal` gives unreadable output. Already transitive dep via MCP SDK. [VERIFIED: go.sum] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| InMemoryTransport | HTTP-only transport | HTTP adds ~10ms overhead per call and httptest server lifecycle. Use HTTP only for smoke tests per D-04. |
| `testify/assert` | `google/go-cmp` only | go-cmp is better for structured diffs; testify is better for simple checks. Use both. |
| `testing/synctest` | `time.Sleep` waits | synctest is deterministic; Sleep is flaky. Always prefer synctest for concurrent daemon lifecycle. |

**Installation:**
```bash
# Promote go-cmp from transitive to direct test dependency
go get github.com/google/go-cmp@v0.7.0
# Everything else already in go.mod
```

## Architecture Patterns

### Recommended Project Structure
```
test/                           # NEW: black-box integration tests (D-01)
  integration/                  # Package: integration_test
    harness.go                  # TestDaemon: Start/Stop, MCP client factory
    harness_test.go             # Self-test: start daemon, ping tool
    symbols_test.go             # DOG-01: 9 symbol retrieval tools
    fileops_test.go             # DOG-02: 6 file operation tools
    diag_test.go                # DOG-03: 3 diagnostic tools
    memory_test.go              # DOG-04: 7 memory tools
    workflow_test.go            # DOG-05: 2 workflow tools
    profile_test.go             # DOG-06: 2 profile tools
    smoke_http_test.go          # D-04: HTTP transport smoke subset
    helpers.go                  # Assertion helpers, result parsing

testdata/
  fixtures/
    go/                         # D-05: small Go project with known symbols
      go.mod
      main.go                   # Known: main(), Helper(), DemoStruct, Value()
      pkg/
        greeter.go              # Known: Greet(), Greeter interface
```

### Pattern 1: In-Memory MCP Client Wiring
**What:** Create daemon in-process, wire MCP client via `NewInMemoryTransports()`, call tools through full protocol path.
**When to use:** All integration tests (default transport per D-04).
**Example:**
```go
// Source: MCP Go SDK v1.5.0 mcp_example_test.go + Serena daemon.go
func StartTestDaemon(t *testing.T, opts Options) *TestDaemon {
    t.Helper()
    cfg := defaultTestConfig(t)
    // Apply options...
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))

    d, err := daemon.New(cfg, logger)
    require.NoError(t, err)

    // Start kernel pool in background
    ctx, cancel := context.WithCancel(context.Background())
    go d.KernelInstance().Run(ctx)

    // Create in-memory transport pair
    clientTransport, serverTransport := mcp.NewInMemoryTransports()

    // Connect server side first (must be ready before client init)
    _, err = d.MCPServer().SDK().Connect(ctx, serverTransport, nil)
    require.NoError(t, err)

    // Connect client
    client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0"}, nil)
    session, err := client.Connect(ctx, clientTransport, nil)
    require.NoError(t, err)

    td := &TestDaemon{daemon: d, session: session, cancel: cancel, t: t}
    t.Cleanup(td.Stop)
    return td
}
```
[VERIFIED: MCP SDK v1.5.0 source confirms `NewInMemoryTransports()` returns `(*InMemoryTransport, *InMemoryTransport)`, `Server.Connect()` takes `Transport`, `Client.Connect()` returns `*ClientSession`]

### Pattern 2: HTTP Transport Smoke Test
**What:** Use `httptest.NewServer` + `StreamableClientTransport` for a subset of tests that validate the full HTTP serialization path.
**When to use:** Small smoke test subset per D-04.
**Example:**
```go
// Source: MCP Go SDK v1.5.0 streamable.go + Serena mcp/server.go
func (td *TestDaemon) NewHTTPSession(t *testing.T) *mcp.ClientSession {
    ts := httptest.NewServer(td.daemon.MCPServer().HTTPHandler())
    t.Cleanup(ts.Close)

    client := mcp.NewClient(&mcp.Implementation{Name: "test-http", Version: "1.0"}, nil)
    session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
        Endpoint: ts.URL + "/mcp",
    }, nil)
    require.NoError(t, err)
    t.Cleanup(func() { session.Close() })
    return session
}
```
[VERIFIED: `StreamableClientTransport{Endpoint: string}` confirmed in SDK source. `SerenaMCPServer.HTTPHandler()` returns `http.Handler`]

### Pattern 3: LS Readiness Polling
**What:** After activating a workspace, poll a known symbol operation until gopls finishes indexing.
**When to use:** Before any symbol/diagnostic assertions in tests that need LSP.
**Example:**
```go
func WaitForLS(t *testing.T, session *mcp.ClientSession, timeout time.Duration) {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    ticker := time.NewTicker(200 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            t.Fatalf("LS readiness timeout after %v", timeout)
        case <-ticker.C:
            result, err := session.CallTool(ctx, &mcp.CallToolParams{
                Name:      "search_symbols",
                Arguments: map[string]any{"query": "main", "scope": "workspace"},
            })
            if err == nil && !result.IsError && len(result.Content) > 0 {
                return // LS is ready
            }
        }
    }
}
```
[ASSUMED: `search_symbols` is a good readiness probe -- it requires gopls to have indexed the workspace. Alternative: `get_hover_info` on a known symbol.]

### Pattern 4: Build Tag Gating
**What:** All integration test files use `//go:build integration` so `go test ./...` skips them.
**When to use:** Every file in `test/integration/`.
**Example:**
```go
//go:build integration

package integration_test
```
Run with: `go test -tags integration ./test/integration/...`
[VERIFIED: standard Go build constraint mechanism]

### Pattern 5: Fixture Copy-on-Use
**What:** Copy fixture from `testdata/fixtures/go/` to `t.TempDir()` before each test so edits don't corrupt shared state.
**When to use:** All tests that activate a project workspace.
**Example:**
```go
func PrepareFixture(t *testing.T, lang string) string {
    t.Helper()
    src := filepath.Join("testdata", "fixtures", lang)
    dst := t.TempDir()
    err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return err }
        rel, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, rel)
        if d.IsDir() { return os.MkdirAll(target, 0o755) }
        data, err := os.ReadFile(path)
        if err != nil { return err }
        return os.WriteFile(target, data, 0o644)
    })
    require.NoError(t, err)
    return dst
}
```
[VERIFIED: standard Go `filepath.WalkDir` + `t.TempDir()` pattern]

### Anti-Patterns to Avoid
- **Subprocess daemon:** Don't `exec.Command("serena", "daemon")` -- in-process via `daemon.New()` is faster and debuggable.
- **Mocking kernel/pool:** Defeats integration test purpose. Use real gopls against real fixtures.
- **Shared mutable fixture:** Always copy to `t.TempDir()`. Editing tests mutate files.
- **No readiness check:** gopls takes 2-5s to index. Immediate assertions after `activate_project` fail intermittently.
- **One daemon per test:** Daemon startup is ~50-100ms. Share one daemon per test package (or per subtest group). Each test gets its own MCP session.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MCP protocol client | Custom JSON-RPC client | `mcp.NewClient` + `ClientSession.CallTool` | SDK handles framing, schema validation, error codes |
| In-process transport | `net.Pipe()` wrapper | `mcp.NewInMemoryTransports()` | SDK pairs transports correctly with buffering |
| HTTP test server | Manual `net.Listen` | `httptest.NewServer` + `StreamableClientTransport` | stdlib handles port allocation, TLS, cleanup |
| Deep struct comparison | `reflect.DeepEqual` | `google/go-cmp` | Readable diffs, field ignorance, custom comparers |
| LS process management | `os/exec` gopls wrapper | Existing `lspool.Pool` | Pool handles spawn, health, circuit breaking, TTL |

**Key insight:** The MCP Go SDK provides the complete client testing surface. The harness is thin glue between `daemon.New()` and SDK client -- not a framework.

## Common Pitfalls

### Pitfall 1: Forgetting Server-Side Connect Before Client
**What goes wrong:** Client sends initialization before server is ready, causing handshake timeout.
**Why it happens:** `NewInMemoryTransports()` returns two ends of a pipe. The server must call `Server.Connect(ctx, serverTransport, nil)` before the client calls `Client.Connect(ctx, clientTransport, nil)`.
**How to avoid:** Always connect server first in harness setup. The SDK examples confirm this order.
**Warning signs:** "context deadline exceeded" during client.Connect.

### Pitfall 2: macOS Unix Socket Path Length
**What goes wrong:** Socket paths exceeding 104 bytes cause `bind: invalid argument` on macOS.
**Why it happens:** macOS `sockaddr_un.sun_path` is limited to 104 bytes. `t.TempDir()` paths can be long.
**How to avoid:** The existing `shortSocketPath()` pattern in `daemon_integration_test.go` handles this. For new tests using InMemory transport, this is irrelevant (no socket). Only matters if any test uses `daemon.Run()` directly.
**Warning signs:** `bind: invalid argument` errors on macOS.

### Pitfall 3: Test Cache Invalidation with LSP
**What goes wrong:** `go test` caches results, but LSP state is not deterministic across runs. Cached "pass" may mask real failures.
**Why it happens:** Go test caching doesn't know about external gopls process state.
**How to avoid:** Use `go test -count=1` to disable caching for integration tests. Document in Makefile target.
**Warning signs:** Tests pass locally but fail in CI, or pass after `go clean -testcache`.

### Pitfall 4: Orphaned gopls Processes
**What goes wrong:** Test failure or panic leaves gopls processes running, consuming memory.
**Why it happens:** `t.Cleanup()` not registered, or pool shutdown not called on test failure.
**How to avoid:** Register `t.Cleanup(td.Stop)` immediately after harness creation. The `Stop` method must cancel context (stopping `kernel.Run` goroutine which calls `pool.stopAll`).
**Warning signs:** `ps aux | grep gopls` shows orphaned processes after test runs.

### Pitfall 5: skill.InitAll Only Runs Once Per Process
**What goes wrong:** Second daemon creation in same test process fails or returns stale skill state because skills use `init()` registration.
**Why it happens:** Go `init()` functions run once per process. `skill.Register()` is called during init. `skill.InitAll()` can be called again but skills are already registered.
**How to avoid:** Share one daemon per test package. Don't try to reset/recreate skills between tests. This is already documented in `bootstrap_test.go` comments.
**Warning signs:** "skill already registered" panics, or nil skill references.

### Pitfall 6: Workspace Activation Is Global State
**What goes wrong:** Two tests activate different workspaces concurrently, but the daemon's `activeWSKey` is a single variable -- second activation overwrites first.
**Why it happens:** `daemon.go` line 132: `activeWSKey = workspace.WorkspaceKey{RepoRoot: repoPath}` is set in the activate callback.
**How to avoid:** Run workspace-dependent tests sequentially (don't use `t.Parallel()` for tests that activate different projects on the same daemon). Or use separate daemon instances per workspace.
**Warning signs:** Tests intermittently get wrong workspace results.

## Code Examples

### Complete Tool Call and Assertion Pattern
```go
// Source: MCP SDK v1.5.0 mcp_test.go lines 226-259 + Serena conventions
func TestSymbolRetrieval_GoToDefinition(t *testing.T) {
    td := StartTestDaemon(t, Options{})
    fixture := PrepareFixture(t, "go")

    // Activate and wait for LS
    result, err := td.session.CallTool(context.Background(), &mcp.CallToolParams{
        Name:      "activate_project",
        Arguments: map[string]any{"repo_path": fixture},
    })
    require.NoError(t, err)
    require.False(t, result.IsError, "activate_project failed: %v", textContent(result))

    WaitForLS(t, td.session, 30*time.Second)

    // Test go_to_definition
    result, err = td.session.CallTool(context.Background(), &mcp.CallToolParams{
        Name:      "go_to_definition",
        Arguments: map[string]any{"file_path": "main.go", "symbol_name": "Helper"},
    })
    require.NoError(t, err)
    require.False(t, result.IsError)
    assert.Contains(t, textContent(result), "Helper")
}

// textContent extracts text from CallToolResult.Content
func textContent(r *mcp.CallToolResult) string {
    for _, c := range r.Content {
        if tc, ok := c.(*mcp.TextContent); ok {
            return tc.Text
        }
    }
    return ""
}
```
[VERIFIED: `CallToolResult.Content` is `[]Content`, `TextContent` has `.Text` field -- confirmed in SDK protocol.go]

### Graceful Skip When gopls Missing
```go
// Source: standard Go testing pattern + exec.LookPath
func requireGopls(t *testing.T) {
    t.Helper()
    if _, err := exec.LookPath("gopls"); err != nil {
        t.Skip("gopls not installed, skipping LSP integration test")
    }
}
```
[VERIFIED: `exec.LookPath` is stdlib]

### Daemon Accessor Methods (Production Code Change)
```go
// Add to internal/daemon/daemon.go
// MCPServer returns the MCP server for test wiring (e.g., HTTPHandler, SDK().Connect).
func (d *Daemon) MCPServer() *serenaMCP.SerenaMCPServer { return d.mcpServer }

// KernelInstance returns the kernel for lifecycle management in tests.
func (d *Daemon) KernelInstance() *kernel.Kernel { return d.kernel }
```
[VERIFIED: `mcpServer` and `kernel` are private fields on Daemon struct in daemon.go]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `GOEXPERIMENT=synctest` | `testing/synctest` GA | Go 1.25 (2025) | No experiment flag needed, use directly |
| Custom MCP test client | `mcp.NewClient` + `NewInMemoryTransports` | MCP Go SDK v1.5.0 | SDK provides complete test surface |
| Direct `ExecuteTool()` calls (bypass protocol) | `ClientSession.CallTool()` via MCP protocol | This phase | Tests validate full JSON-RPC + schema path |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `search_symbols` is a good LS readiness probe | Architecture Patterns (Pattern 3) | Could use `get_hover_info` instead. Low risk -- any tool requiring LS indexing works. |
| A2 | gopls takes 2-5s to index a small fixture project | Common Pitfalls | If faster, timeout can be reduced. If slower, tests may need longer timeout. Low risk. |
| A3 | Single `activeWSKey` in daemon is safe if tests run sequentially | Pitfalls (6) | If tests need parallel workspace activation, daemon needs refactoring. Medium risk. |

## Open Questions

1. **Build tag: `integration` vs `e2e`?**
   - What we know: REQUIREMENTS.md specifies `//go:build integration`. CONTEXT.md leaves naming to Claude's discretion.
   - Recommendation: Use `integration` per HARN-01. It is the Go ecosystem convention.

2. **Promote `go-cmp` to direct dependency?**
   - What we know: Already in `go.sum` as transitive via MCP SDK. Not in `go.mod` directly.
   - Recommendation: Yes. `go get github.com/google/go-cmp@v0.7.0`. SDK's own tests use `cmp.Diff` for `CallToolResult` comparison -- this is the verified pattern.

3. **Should harness share one daemon or create per-test?**
   - What we know: `skill.InitAll()` / `skill.Register()` are effectively singletons. Creating multiple daemons in one process works (bootstrap_test.go does it) but skills are shared.
   - Recommendation: One daemon per test file/group. Use subtests for individual tool tests. Workspace activation is the per-test variable.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Everything | Yes | 1.25.1 | -- |
| gopls | Symbol, diagnostic, edit tests (DOG-01, DOG-03) | Yes | v0.21.1 | `t.Skip("gopls not found")` |
| MCP Go SDK | Test client + transports | Yes | v1.5.0 | -- |
| testify | Assertions | Yes | v1.11.1 | -- |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None -- all dependencies available.

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task** -- integration tests must pass vet.
- **`go test ./...`** runs unit tests -- integration tests must be excluded by build tag.
- **`go build ./cmd/serena`** must still work -- no breaking changes to production code.
- **stretchr/testify** is the established assertion library.
- **GSD workflow** must be followed for execution.

## Sources

### Primary (HIGH confidence)
- MCP Go SDK v1.5.0 module cache: `NewInMemoryTransports()`, `Client.Connect()`, `ClientSession.CallTool()`, `StreamableClientTransport`, `CallToolResult`, `TextContent` -- all API shapes verified directly in source.
- Existing `internal/daemon/daemon.go` -- Daemon struct, `New()`, `Run()`, private fields `mcpServer`/`kernel`, `HTTPHandler()` access path.
- Existing `internal/daemon/daemon_integration_test.go` -- In-process daemon construction, `skill.InitAll()`, `ExecuteTool()`, lifecycle patterns.
- Existing `internal/daemon/bootstrap_test.go` -- Tool registration count (38+), skill names, init() singleton behavior.
- `internal/mcp/server.go` -- `SerenaMCPServer` API: `SDK()`, `HTTPHandler()`, `Registry()`.
- `internal/kernel/kernel.go` -- `NewKernel()`, `ActivateWorkspace()`, `Run()`, `Pool()`.
- `internal/kernel/lspool/pool.go` -- `Pool.Run()`, `Pool.AcquireLease()`, `stopAll()` cleanup.
- go.mod -- All dependency versions verified.
- System: Go 1.25.1, gopls v0.21.1 confirmed installed.

### Secondary (MEDIUM confidence)
- Prior research documents: `.planning/research/STACK.md`, `.planning/research/ARCHITECTURE.md` -- detailed MCP SDK patterns, build order.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified in go.mod/go.sum and SDK source
- Architecture: HIGH -- patterns verified against SDK examples and existing daemon tests
- Pitfalls: HIGH -- derived from existing codebase analysis (singleton skills, activeWSKey, macOS socket paths)

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (stable -- Go 1.25 and MCP SDK v1.5.0 are current)
