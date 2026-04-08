# Architecture Research: Integration Testing for Daemon-Based MCP Server

**Domain:** End-to-end integration testing for a daemon-based MCP code intelligence platform
**Researched:** 2026-04-08
**Confidence:** HIGH (based on direct codebase analysis + MCP Go SDK v1.5.0 source)

## System Overview: Test Architecture Layered on Existing System

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     TEST ORCHESTRATION LAYER (NEW)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────────┐   │
│  │ Test Harness │  │  Fixture     │  │  Assertion                  │   │
│  │ (testharness │  │  Manager     │  │  Helpers                    │   │
│  │  package)    │  │  (fixtures/) │  │  (tool result validators)   │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────────┬──────────────┘   │
│         │                 │                          │                  │
├─────────┴─────────────────┴──────────────────────────┴──────────────────┤
│                     MCP CLIENT LAYER (NEW)                              │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  mcpsdk.Client + StreamableClientTransport (from MCP Go SDK)   │    │
│  │  Connects via HTTP to test daemon instance                      │    │
│  └────────────────────────────┬────────────────────────────────────┘    │
│                               │ HTTP /mcp                              │
├───────────────────────────────┴─────────────────────────────────────────┤
│              EXISTING DAEMON (unmodified, started by harness)           │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌──────────┐    │
│  │MCP Srvr │  │ Kernel  │  │LS Pool  │  │ Skills  │  │ Profiles │    │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘  └──────────┘    │
├─────────────────────────────────────────────────────────────────────────┤
│              FIXTURE PROJECTS (NEW, on disk)                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────────┐      │
│  │ Go (self)│  │ Python   │  │TypeScript│  │ Rust / Java       │      │
│  │ dogfood  │  │ fixture  │  │ fixture  │  │ fixtures          │      │
│  └──────────┘  └──────────┘  └──────────┘  └───────────────────┘      │
└─────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

### New Components

| Component | Responsibility | Location |
|-----------|----------------|----------|
| **Test Harness** | Daemon lifecycle (start/stop), MCP client management, workspace activation | `internal/testharness/` |
| **Fixture Manager** | Copy/setup/teardown fixture projects in temp dirs | `internal/testharness/fixtures.go` |
| **Assertion Helpers** | Validate MCP tool responses (CallToolResult parsing, error checking) | `internal/testharness/assert.go` |
| **Fixture Projects** | Small projects with known symbols, references, diagnostics | `testdata/fixtures/{go,python,typescript,rust,java}/` |
| **E2E Test Suites** | Actual test files exercising tool categories | `tests/e2e/` |
| **Dogfood Suite** | Tests exercising all tools against Serena's own codebase | `tests/e2e/dogfood_test.go` |

### Existing Components (unmodified except two accessor methods)

| Component | Role in Testing |
|-----------|----------------|
| `daemon.New()` + `daemon.Run()` | Started by harness; provides full MCP server |
| `mcpsdk.Server` + `HTTPHandler()` | Serves MCP protocol over HTTP for test client |
| `kernel.Kernel` | Activates workspace, manages LS workers for fixture projects |
| `lspool.Pool` | Spawns real language servers for fixture languages |
| `skill.*` | All skills exercised end-to-end through MCP client |
| `profile.*` | Profile filtering tested via client tool list assertions |

## Recommended Project Structure

```
internal/
└── testharness/              # NEW: reusable test infrastructure
    ├── harness.go            # TestDaemon struct: start, stop, client access
    ├── client.go             # MCP client helpers: CallTool, ListTools wrappers
    ├── fixtures.go           # Fixture project copy/setup/teardown
    ├── assert.go             # Tool result assertion helpers
    └── harness_test.go       # Self-tests for the harness itself

testdata/
└── fixtures/                 # NEW: small projects with known symbols
    ├── go/                   # Go fixture: go.mod, main.go, pkg/
    ├── python/               # Python fixture: pyproject.toml, src/
    ├── typescript/            # TypeScript fixture: tsconfig.json, src/
    ├── rust/                  # Rust fixture: Cargo.toml, src/
    └── java/                  # Java fixture: pom.xml, src/

tests/
└── e2e/                      # NEW: end-to-end test suites
    ├── suite_test.go         # TestMain with shared daemon setup
    ├── symbols_test.go       # Symbol retrieval tool tests
    ├── edit_test.go          # Symbol editing tool tests
    ├── fileops_test.go       # File operation tool tests
    ├── diag_test.go          # Diagnostic tool tests
    ├── memory_test.go        # Memory skill tool tests
    ├── workflow_test.go      # Workflow skill tool tests
    ├── profile_test.go       # Profile/mode switching tests
    └── dogfood_test.go       # All tools against Serena's own codebase
```

### Structure Rationale

- **`internal/testharness/`:** Reusable across test suites. Keeps daemon lifecycle logic out of individual test files. Internal package prevents external consumption of test infrastructure.
- **`testdata/fixtures/`:** Go convention for test data. Each fixture is a self-contained project that language servers can index. Committed to repo, never modified by tests (copied to temp dirs).
- **`tests/e2e/`:** Separate from unit tests. Run with build tag `//go:build e2e` so `go test ./...` skips them by default (they need real language servers installed). `TestMain` shares one daemon instance across all tests in the package.

## Architectural Patterns

### Pattern 1: Shared Daemon Per Test Package (httptest.Server pattern)

**What:** Start one daemon + HTTP server in `TestMain`, share across all tests in the package. Each test gets its own MCP client session.
**When to use:** All E2E tests. Daemon startup is expensive (skill init, pool creation).
**Trade-offs:** Faster (one daemon per package run), but tests must not interfere with each other's state. Workspace activation is per-session, so each test can activate a different fixture.

**Example:**
```go
// tests/e2e/suite_test.go
package e2e_test

import (
    "os"
    "testing"
    "github.com/postfix/serena/internal/testharness"
)

var harness *testharness.TestDaemon

func TestMain(m *testing.M) {
    var err error
    harness, err = testharness.Start(testharness.Options{
        Profile:  "full",
        HTTPAddr: "127.0.0.1:0", // random port
    })
    if err != nil {
        panic(err)
    }
    code := m.Run()
    harness.Stop()
    os.Exit(code)
}
```

### Pattern 2: Per-Test MCP Client Sessions

**What:** Each test function creates a fresh MCP client session via `StreamableClientTransport`. The client connects to the shared daemon's HTTP endpoint. Tests call `activate_project` to set their fixture as the active workspace.
**When to use:** Every individual E2E test. Ensures session isolation.
**Trade-offs:** Slight overhead per test for MCP handshake (~10ms), but ensures clean session state (mode, workspace).

**Example:**
```go
func TestGoToDefinition(t *testing.T) {
    ctx := context.Background()
    fixture := harness.PrepareFixture(t, "go")
    session := harness.NewSession(t, ctx)
    defer session.Close()

    session.ActivateProject(t, fixture.Root)
    session.WaitForLS(t, "go", 30*time.Second)

    result := session.CallTool(t, "go_to_definition", map[string]any{
        "file_path":   "main.go",
        "symbol_name": "Greet",
    })

    testharness.AssertNoError(t, result)
    testharness.AssertContains(t, result, "greet.go")
}
```

### Pattern 3: Fixture-as-Snapshot (Copy-on-Use)

**What:** Fixture projects in `testdata/fixtures/` are pristine templates. The harness copies them to `t.TempDir()` before each test (or test group) so that editing tests can modify files without affecting other tests.
**When to use:** All tests that activate a project. Critical for editing tests that mutate files.
**Trade-offs:** Disk I/O per test, but temp dirs are fast and `t.TempDir()` auto-cleans. For read-only tests, an optimization can share one copy across a subtest group.

### Pattern 4: Build Tag Gating for CI Control

**What:** E2E tests use `//go:build e2e` so they only run when explicitly requested: `go test -tags e2e ./tests/e2e/...`. This prevents CI failures when language servers are not installed.
**When to use:** All E2E tests. Unit tests (`go test ./...`) must remain fast and self-contained.
**Trade-offs:** Developers must remember to run with `-tags e2e`. Mitigated by Makefile target: `make test-e2e`.

### Pattern 5: LS Readiness Polling

**What:** After `activate_project`, language servers need time to initialize and index. The harness must wait for LS readiness before running assertions. Use a polling approach: call a cheap LSP operation (like `search_symbols` with a known symbol) until it succeeds or times out.
**When to use:** Any test that exercises kernel tools (symbols, edit, diag). Not needed for memory/workflow/profile tests.
**Trade-offs:** Adds latency to tests. Use short poll intervals (100ms) with a generous timeout (30-60s for first LS init, 5s for subsequent).

## Data Flow

### E2E Test Request Flow

```
Test Function
    |
    v
testharness.Session.CallTool("go_to_definition", args)
    |
    v
mcpsdk.ClientSession.CallTool(ctx, &CallToolParams{Name: ..., Arguments: ...})
    |
    v (HTTP POST /mcp, Streamable HTTP transport)
    |
mcpsdk.Server (MCP SDK) -> ProfileFilterMiddleware -> Tool Handler
    |
    v
symbols.GoToDefinition handler
    |
    v
kernel.GetRuntime(wsKey) -> WorkspaceRuntime.AcquireSession -> Pool.AcquireLease
    |
    v
Worker (gopls process) <- LSP textDocument/definition request
    |
    v (LSP response)
    |
CallToolResult{Content: [TextContent{Text: "..."}]}
    |
    v (HTTP response, SSE stream)
    |
testharness.Session -> test assertions
```

### Daemon Lifecycle in Tests

```
TestMain:
    testharness.Start()
        -> config.SerenaConfig{HTTPAddr: ":0"}
        -> daemon.New(cfg, logger)
        -> httptest.NewServer(daemon.MCPServer().HTTPHandler())
        -> daemon.KernelInstance().Run(ctx) in background goroutine
        -> return TestDaemon{URL, cancel}

Each Test:
    harness.PrepareFixture(t, "go")
        -> copies testdata/fixtures/go/ to t.TempDir()
        -> returns FixtureProject{Root: tmpDir}

    harness.NewSession(t, ctx)
        -> mcpsdk.NewClient(impl, nil)
        -> client.Connect(ctx, &StreamableClientTransport{Endpoint: harness.URL})
        -> returns Session{clientSession}

    session.ActivateProject(t, root)
        -> clientSession.CallTool(ctx, "activate_project", {repo_path: root})
        -> daemon activates workspace, detects languages, pool ready

    session.CallTool(t, toolName, args)
        -> clientSession.CallTool(ctx, params)
        -> returns parsed result

TestMain cleanup:
    harness.Stop()
        -> cancel context
        -> daemon.shutdown() (kernel shutdown -> pool drain -> workers stop)
        -> httptest.Server.Close()
```

### LS Worker Lifecycle During Tests

```
First test activating Go fixture:
    activate_project -> kernel.ActivateWorkspace -> DetectLanguages(["go"])
    First CallTool(symbol tool) -> Pool.AcquireLease -> no worker exists
        -> Pool.spawnWorker("go", fixtureRoot) -> exec gopls
        -> LSP initialize/initialized handshake
        -> Worker state: Ready
        -> Process request, return result

Subsequent tests reusing Go (share-until-dirty):
    Pool.AcquireLease -> existing worker is clean -> reuse
    (No new gopls process needed)

Editing test (marks worker dirty):
    replace_symbol_body -> worker marked dirty
    Next AcquireLease -> dirty worker evicted, new worker spawned
    (Or: next test activates fresh fixture copy, new workspace key)

Shutdown:
    harness.Stop() -> kernel.Shutdown -> pool drains all workers
    -> gopls processes receive shutdown/exit LSP messages
    -> Worker state: Stopped
```

## Integration Points: New Components to Existing Architecture

### Where New Code Touches Existing Code

| New Component | Existing Component | Integration Type | Notes |
|---------------|-------------------|------------------|-------|
| TestDaemon.Start() | `daemon.New()` | Direct constructor call | Uses same config struct, no modifications needed |
| TestDaemon.Start() | `daemon.Run()` | Partial -- skip Run(), use HTTP handler directly | Run() manages socket + signals. Tests only need HTTP. Use `httptest.NewServer(d.MCPServer().HTTPHandler())` instead of `d.Run()`. Kernel.Run() must still be called for pool lifecycle. |
| Session.CallTool() | `mcpsdk.ClientSession.CallTool()` | Thin wrapper | Adds test-friendly assertion sugar |
| FixtureManager | `kernel.ActivateWorkspace()` | Via MCP protocol (activate_project tool) | No direct kernel access -- pure E2E via MCP client |
| LS Readiness | `lspool.Pool` | Indirect -- poll via MCP tools | No new pool APIs needed; readiness detected by tool success |
| Dogfood suite | Serena's own repo root | `activate_project` with repo root path | Tests run from repo root; gopls indexes the full project |

### Critical Integration Decision: httptest vs Full daemon.Run()

**Recommendation: httptest.NewServer + manual kernel.Run().**

`daemon.Run()` does three things: (1) starts kernel pool in errgroup, (2) listens on Unix socket, (3) optionally listens on HTTP. For tests, we need (1) and a test-controlled HTTP server, but NOT (2) -- Unix socket paths are fragile in tests, and gRPC forwarder testing is a separate concern.

The harness should:
1. Call `daemon.New(cfg, logger)` to get a fully wired daemon
2. Start `daemon.KernelInstance().Run(ctx)` in a background goroutine for pool lifecycle
3. Use `httptest.NewServer(daemon.MCPServer().HTTPHandler())` for the MCP endpoint

**Modification needed in existing code:** Add two accessor methods to `internal/daemon/daemon.go`:
```go
func (d *Daemon) MCPServer() *serenaMCP.SerenaMCPServer { return d.mcpServer }
func (d *Daemon) KernelInstance() *kernel.Kernel { return d.kernel }
```

This is the **only change** to existing production code. Everything else is additive.

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Test harness <-> Daemon | Go API (constructor + accessors) | Harness creates daemon in-process, not as subprocess |
| Test <-> MCP Server | HTTP (Streamable HTTP MCP transport) | Full protocol stack exercised |
| MCP Server <-> Kernel | In-process function calls (existing) | Unchanged |
| Kernel <-> Language Servers | stdio (LSP JSON-RPC) | Real LS processes; must be installed on test machine |
| Fixture Manager <-> Filesystem | os.CopyFS / file copy | Pristine fixtures copied to t.TempDir() |

## Anti-Patterns

### Anti-Pattern 1: Subprocess Daemon

**What people do:** Start the daemon as a child process (`exec.Command("serena", "daemon")`), connect to its socket.
**Why it's wrong:** Adds process management complexity, port coordination, flaky cleanup on test failure, cannot debug daemon internals, slow startup.
**Do this instead:** In-process daemon via `daemon.New()` + `httptest.NewServer`. Same code path, but test-controllable.

### Anti-Pattern 2: Mocking the Kernel or Pool

**What people do:** Create mock workers, fake LSP responses, stub the pool.
**Why it's wrong:** Defeats the purpose of E2E integration testing. You want to prove that real gopls/pyright/typescript-language-server produce correct results through the full Serena stack.
**Do this instead:** Use real language servers against real (small) fixture projects. Reserve mocking for unit tests only.

### Anti-Pattern 3: Shared Mutable Fixture

**What people do:** Point all tests at the same fixture directory without copying.
**Why it's wrong:** Editing tests mutate files, corrupting state for subsequent tests. Test ordering becomes fragile.
**Do this instead:** Copy fixtures to `t.TempDir()` per test. For read-only test groups, can share one copy per subtest.

### Anti-Pattern 4: No LS Readiness Check

**What people do:** Call `activate_project` then immediately call symbol tools.
**Why it's wrong:** gopls needs seconds to index even a small project. Tests fail intermittently.
**Do this instead:** Poll a known symbol operation with retry + timeout. The harness encapsulates this as `session.WaitForLS()`.

### Anti-Pattern 5: One Daemon Per Test

**What people do:** Start and stop a fresh daemon for every test function.
**Why it's wrong:** Daemon startup includes skill.InitAll(), language registry creation, profile resolution. Takes ~50-100ms per startup. With 50+ tests, this wastes minutes and skills use init()-based registration that only runs once per process.
**Do this instead:** Share one daemon per test package via `TestMain`. Each test gets a fresh MCP session.

## Suggested Build Order

Based on dependency analysis, build bottom-up:

### Phase 1: Test Harness Foundation
**Build:**
1. Add `MCPServer()` and `KernelInstance()` accessor methods to `daemon.go`
2. `internal/testharness/harness.go` -- TestDaemon with Start/Stop using httptest + kernel.Run
3. `internal/testharness/client.go` -- Session wrapper around mcpsdk.ClientSession
4. Self-test: harness starts daemon, connects client, calls `ping` tool

### Phase 2: Fixture Infrastructure + File Ops
**Build:**
1. `testdata/fixtures/go/` -- minimal Go project with known symbols
2. `internal/testharness/fixtures.go` -- copy fixture to temp dir
3. `internal/testharness/assert.go` -- CallToolResult parsing helpers
4. `tests/e2e/fileops_test.go` -- file ops (read_file, list_directory, search_in_files) against Go fixture

### Phase 3: Go Dogfood (Symbols + Diagnostics)
**Build:**
1. `tests/e2e/symbols_test.go` -- go_to_definition, find_references, get_symbol_overview against fixture
2. `tests/e2e/diag_test.go` -- get_diagnostics, format_code against fixture
3. `tests/e2e/dogfood_test.go` -- all read-only tools against Serena's own repo

### Phase 4: Editing + Multi-Language
**Build:**
1. `tests/e2e/edit_test.go` -- replace_symbol_body, insert_before_symbol, rename_symbol against fixture copies
2. `testdata/fixtures/{python,typescript}/` -- multi-language fixtures
3. Multi-language variants of symbol/edit tests

### Phase 5: Skills + Profiles
**Build:**
1. `tests/e2e/memory_test.go` -- write/read/search/delete memory via MCP
2. `tests/e2e/workflow_test.go` -- onboard_project, prepare_for_new_conversation
3. `tests/e2e/profile_test.go` -- mode switching, tool filtering, profile-specific behavior

## Performance Expectations

| Operation | Expected Duration |
|-----------|------------------|
| Daemon startup (in-process) | ~50-100ms |
| First gopls initialization for small fixture | ~2-5s |
| Subsequent gopls reuse (share-until-dirty) | ~0ms (existing worker) |
| MCP client connect + initialize | ~10ms |
| Single tool call round-trip | ~50-200ms |
| Full dogfood suite (38 tools against self) | ~30-60s |
| Full E2E suite (all languages) | ~2-5min |

## Sources

- MCP Go SDK v1.5.0 source: `StreamableClientTransport`, `Client.Connect()`, `ClientSession.CallTool()` patterns verified in module cache
- Existing test patterns: `internal/daemon/daemon_integration_test.go` (E2E memory/mode/shutdown tests using direct skill executor calls)
- Existing bootstrap tests: `internal/daemon/bootstrap_test.go` (tool registration count verification)
- Daemon architecture: `internal/daemon/daemon.go` (New/Run/shutdown lifecycle, private fields needing accessors)
- MCP server: `internal/mcp/server.go` (HTTPHandler, AddSkillTool, activate_project callback)
- Kernel: `internal/kernel/kernel.go` (ActivateWorkspace, Run, Pool)
- Worker pool: `internal/kernel/lspool/pool.go` (AcquireLease, share-until-dirty, Run lifecycle)
- Workspace runtime: `internal/kernel/workspace.go` (DetectLanguages, AcquireSession, NextDocVersion)

---
*Architecture research for: Integration testing of daemon-based MCP code intelligence platform*
*Researched: 2026-04-08*
