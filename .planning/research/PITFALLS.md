# Domain Pitfalls

**Domain:** Integration testing for Go MCP code intelligence platform with LSP backends
**Researched:** 2026-04-08
**Focus:** Adding end-to-end integration tests, dogfooding suite, and multi-language fixtures to existing v1.0 daemon

## Critical Pitfalls

Mistakes that cause unreliable test suites, wasted CI time, or false confidence in correctness.

### Pitfall 1: LS Cold Start Timing Creates Pervasive Flaky Tests

**What goes wrong:** Integration tests send LSP requests (definition, references, symbols) immediately after opening a file. The language server hasn't finished indexing yet, so the request returns empty results or errors. The test fails intermittently -- passes when the machine is fast, fails under CI load.

**Why it happens:** Language servers are async by design. After `textDocument/didOpen`, servers like gopls trigger background analysis (package loading, type checking, index building) that can take 500ms-10s depending on project size and CI machine speed. The LSP spec has no "indexing complete" notification -- `textDocument/publishDiagnostics` is the closest signal, but it may arrive in multiple waves.

**Consequences:** Test suite has 10-30% flake rate. Engineers stop trusting tests. CI pipeline requires 2-3 retries to go green. Tests that work locally fail in CI because CI runners are slower.

**Warning signs:** Tests pass when run individually but fail in parallel. Tests pass on developer machines but fail in CI. Adding `time.Sleep(2*time.Second)` "fixes" the test.

**Prevention:**
- Follow the gopls integration test pattern: use an `Awaiter` that tracks `$/progress` and `textDocument/publishDiagnostics` notifications. Block test assertions until the server signals completion.
- Implement a `WaitForReady(ctx context.Context, uri string)` helper that waits for diagnostics to stabilize (no new diagnostics for 500ms) or for `$/progress` work to complete.
- Never use `time.Sleep()` in tests. Use condition-based waiting with a hard timeout (30s in CI, 10s locally). When the timeout fires, include diagnostic state in the error message.
- For Serena specifically: the existing `WorkerState` machine (Starting -> Initializing -> Ready) gates request dispatch, but "Ready" means LSP handshake complete -- NOT that indexing is done. You need a second readiness signal per-file or per-workspace.
- Consider the `testing/synctest` package (Go 1.24+) for tests that involve timers, though it doesn't help with real LS process timing.

**Detection:** Run the full test suite 10 times in sequence. If any test fails even once, it has a timing dependency. Run under `go test -count=10 -failfast`.

**Phase:** Phase 1 (Test Harness) -- the harness must solve this before any tool tests are written.

**Sources:**
- [gopls integration test framework](https://pkg.go.dev/golang.org/x/tools/gopls/internal/test/integration) -- uses `AfterChange()` and `OnceMet()` patterns
- [Go blog: Testing Time](https://go.dev/blog/testing-time) -- async test timing pitfalls

---

### Pitfall 2: Test Fixture Mutation Pollutes Other Tests

**What goes wrong:** Test A opens `fixture/main.go`, sends `textDocument/didChange` to add a function, and asserts symbols. Test B runs next against the same fixture and finds unexpected symbols from Test A's uncommitted edit. Or worse: Test A calls `replace_symbol_body` which writes to disk, permanently mutating the fixture.

**Why it happens:** Serena's edit tools (`replace_symbol_body`, `insert_before_symbol`, etc.) modify real files on disk. The LS worker tracks document state in-memory via `didChange`/`didSave`. If tests share fixtures (same directory) or share LS workers (pool reuse), state leaks between tests.

**Consequences:** Test ordering affects results. Tests pass in isolation but fail in suite. Non-deterministic failures depending on which tests ran first.

**Warning signs:** `go test -shuffle=on` reveals failures. Adding a new test breaks an unrelated existing test.

**Prevention:**
- Copy fixtures to `t.TempDir()` before each test. Never run tests against the source fixture directory. This is non-negotiable for any test that calls edit/write tools.
- For read-only tests (symbol retrieval, references, hover), fixtures can be shared if tests use a fresh LS worker or verify no mutations were applied.
- Implement a `TestFixture` helper that: (1) copies a fixture template to temp dir, (2) activates the workspace in the kernel, (3) waits for LS readiness, (4) tears down cleanly via `t.Cleanup()`.
- For the dogfooding suite (testing against Serena's own codebase), only run read-only tools. Never run edit tools against the live repo in CI -- use a copy.
- The existing `lspool` share-until-dirty design helps: clean workers can be shared, dirty workers are isolated. But the test must still ensure fixture isolation at the filesystem level.

**Detection:** Run `go test -shuffle=on -count=5` on the integration suite. Any failure indicates shared state.

**Phase:** Phase 1 (Test Harness) -- fixture management is foundational.

---

### Pitfall 3: LS Worker Lifecycle Leak Between Tests

**What goes wrong:** Test starts an LS worker via `kernel.ActivateWorkspace()`, the test finishes (or panics), but the LS process isn't stopped. The next test starts a new worker. After 20 tests, there are 20 gopls processes consuming 2GB+ RAM. CI runner OOMs and kills the job.

**Why it happens:** The existing daemon bootstrap (`daemon.New()`) creates a kernel with a pool, but there's no per-test lifecycle management. The pool's idle TTL (default 300s) is much longer than a test run. If tests create the pool directly, they must also shut it down. If they share a pool, cleanup ordering matters.

**Consequences:** CI OOM kills. Zombie LS processes on developer machines. File descriptor exhaustion (each LS worker holds 3 FDs for stdin/stdout/stderr pipes).

**Warning signs:** `ps aux | grep gopls` shows dozens of processes after running tests. CI job killed with signal 9. Tests slow down progressively as the suite runs.

**Prevention:**
- Use a shared test kernel/pool for the entire test package, started in `TestMain()` and shut down after all tests complete. The pool's `Run(ctx)` should be launched with a context that's cancelled in `TestMain`'s cleanup.
- Reduce pool TTL for tests: `PoolConfig{BaseTTL: 5, CeilingTTL: 30}` so idle workers expire quickly.
- Set `MaxWorkers` low for tests (e.g., 3) to prevent resource exhaustion.
- Every test function that activates a workspace must use `t.Cleanup()` to release its lease explicitly.
- In the test harness, track active workers and assert `pool.WorkerCount() == 0` at suite end.
- The existing `TestE2ECleanShutdown` already checks goroutine counts -- extend this pattern to LS process counts.

**Detection:** Add a `TestMain` finalizer that checks for orphan LS processes. Run `pgrep -f "gopls\|pyright\|typescript-language-server"` before and after the suite.

**Phase:** Phase 1 (Test Harness) -- must be solved in the shared test infrastructure.

---

### Pitfall 4: Unix Socket Path Length Exceeds macOS 104-Byte Limit

**What goes wrong:** Tests use `t.TempDir()` which on macOS produces paths like `/var/folders/xx/yyyyyyy/T/TestE2ESymbolRetrieval_gopls_definition123456789/001/s.sock` -- easily exceeding the 104-byte limit for Unix domain sockets on Darwin. The test fails with a cryptic `bind: invalid argument` error.

**Why it happens:** macOS (and BSD) limits `struct sockaddr_un.sun_path` to 104 bytes. Go's `testing.TB.TempDir()` includes the full test name, subtests, and a random suffix. Test names in integration suites are often long and descriptive. This is a known Go issue (golang/go#62614).

**Consequences:** Tests pass on Linux (108-byte limit, shorter temp paths) but fail on macOS. Developers on macOS can't run the suite. The existing `shortSocketPath()` helper in `daemon_test.go` shows this was already encountered.

**Warning signs:** Tests fail only on macOS. Error message mentions "invalid argument" on socket operations. Tests with short names pass, long names fail.

**Prevention:**
- Follow the pattern already established in `daemon_test.go`: use `/tmp/serena-test-<short-id>/s.sock` instead of `t.TempDir()` for socket paths.
- Create a `testSocketPath(t *testing.T)` helper in the shared test infrastructure that generates a short deterministic path and registers cleanup via `t.Cleanup()`.
- For the gRPC listener in integration tests, use the daemon's `shortSocketPath` pattern consistently.
- Alternative: use `localhost:0` TCP for test daemon communication instead of Unix sockets, avoiding the path length issue entirely. This changes the transport but not the protocol.
- If using temp dirs for fixture copies (not sockets), `t.TempDir()` is fine -- the limit only applies to socket paths.

**Detection:** Run integration tests on macOS with long test names. The existing codebase already has the fix pattern -- the risk is forgetting to use it in new test files.

**Phase:** Phase 1 (Test Harness) -- bake into test helpers from the start.

**Sources:**
- [golang/go#62614: TB.TempDir paths too long for Unix sockets](https://github.com/golang/go/issues/62614)
- [macOS 104-byte socket path limit](https://github.com/dotnet/runtime/issues/79503)
- Existing pattern: `daemon_test.go:shortSocketPath()`

---

### Pitfall 5: CI Environment Missing Language Servers

**What goes wrong:** Integration tests require real language servers (gopls, pyright, typescript-language-server, rust-analyzer, jdtls). The CI runner doesn't have them installed. Tests either fail with "command not found" or the three-tier installer tries to auto-download, adding 30-60s latency and introducing network flakiness.

**Why it happens:** Language servers are external binaries not shipped with Go. Developers have them installed locally but CI environments are minimal. The existing `langregistry.Installer` has auto-install capability, but relying on it in CI means: (a) network dependency in tests, (b) download time variance, (c) version drift as servers update.

**Consequences:** CI builds are slow (LS downloads) or flaky (network timeouts). Tests pass locally with LS version X but fail in CI with version Y. Different developers have different LS versions, creating "works on my machine" bugs.

**Warning signs:** CI pipeline has a 40-60s "setup" phase that's actually downloading language servers. Tests fail with "connection refused" or "command not found" in CI. Flaky test rate correlates with npm/pip registry availability.

**Prevention:**
- Pin exact LS versions in a CI setup script. Install them explicitly in the CI workflow, not via the auto-installer. Example:
  ```
  go install golang.org/x/tools/gopls@v0.18.0
  npm install -g pyright@1.1.400
  npm install -g typescript-language-server@4.3.0 typescript@5.6.0
  ```
- Use build tags to categorize tests: `//go:build integration` for tests needing real LS, no tag for unit tests. CI runs `go test -tags=integration ./...` only after LS setup.
- Create a `testutil.RequireLS(t, "gopls")` helper that skips the test with `t.Skip("gopls not installed")` if the binary isn't in PATH. This prevents suite-level failures from a single missing LS.
- For the v1.1 milestone, start with gopls only (already available via `go install`). Add pyright and typescript-language-server in a second phase. Defer jdtls and rust-analyzer to later -- they have complex installation requirements.
- Lock LS versions in a `testdata/ls-versions.txt` file tracked in the repo. CI reads from this file.

**Detection:** Run integration tests in a fresh Docker container with only Go installed. Every test that fails reveals an undeclared dependency.

**Phase:** Phase 1 (CI Setup) -- must be solved before writing any LS-dependent tests.

**Sources:**
- [GitHub Actions: setup-go doesn't include gopls](https://github.com/actions/setup-go)
- Existing pattern: `langregistry.Installer` three-tier resolution

## Moderate Pitfalls

### Pitfall 6: Dogfooding Tests Couple to Codebase Structure

**What goes wrong:** The dogfooding suite tests `find_symbol("Daemon")` against Serena's own codebase and asserts it returns a result in `internal/daemon/daemon.go`. Someone renames the struct to `Server` or moves it to another package. The dogfooding test fails, but it's not a bug in Serena -- it's a test maintenance burden.

**Why it happens:** Dogfooding tests that assert specific symbol names, line numbers, or file paths are inherently brittle because the codebase under test evolves independently.

**Consequences:** Every refactor requires updating dogfooding tests. Engineers start avoiding refactors to keep tests green. The test suite becomes a change-prevention mechanism rather than a correctness tool.

**Prevention:**
- Dogfooding tests should assert behavioral properties, not structural ones:
  - "find_symbol returns a non-empty result for a symbol that exists" (not "returns result at line 52")
  - "get_definition on a known function call resolves to a file in the same package" (not "resolves to daemon.go:68")
  - "find_references on a widely-used type returns >5 references" (not "returns exactly 23 references")
- Use a small set of "anchor symbols" that are unlikely to change: `main()`, `Daemon`, `Kernel`. Document these as test anchors in a comment.
- For position-sensitive tests, use the multi-language fixtures (not the live codebase). Fixtures are stable by design.
- Dogfooding tests should focus on: "does the tool respond without error?", "is the response schema correct?", "does the daemon stay healthy after N tool invocations?"

**Detection:** If a dogfooding test fails after a non-Serena code change (e.g., renaming a variable), the test is too coupled.

**Phase:** Phase 2 (Dogfooding Suite) -- establish assertion patterns before writing many tests.

---

### Pitfall 7: Shared Global State in Skill Registry Across Tests

**What goes wrong:** The `skill.Register()` / `skill.InitAll()` pattern uses package-level global state (via `init()` registration). Test A calls `skill.InitAll(deps1)` with one ProjectDir. Test B calls `skill.InitAll(deps2)` with a different ProjectDir. Skills are re-initialized, but the global registry state from Test A may leak. The existing `daemon_integration_test.go` already shows this pattern -- it calls `skill.InitAll(deps)` per test.

**Why it happens:** Caddy-style `init()` registration is designed for one-time initialization at startup, not for repeated initialization in tests. The global `skill` registry doesn't have a `Reset()` function. Skills may hold internal state (file watchers, database connections) that doesn't get cleaned up on re-init.

**Consequences:** Memory skill from Test A's ProjectDir might serve stale data in Test B. SQLite FTS5 connections from one test leak into another. Tests pass in isolation but fail when run together.

**Warning signs:** Test order matters. Running a single test passes but running the full suite has failures. Memory tool returns data written by a previous test.

**Prevention:**
- Add a `skill.ResetAll()` function for testing that clears all skill state and closes resources (database connections, file watchers).
- Call `t.Cleanup(skill.ResetAll)` in every test that calls `skill.InitAll()`.
- For the test harness, consider a `TestDaemon` helper that wraps `daemon.New()` and handles skill lifecycle automatically.
- The memory skill's SQLite connection is particularly dangerous -- ensure `memory.Store.Close()` is called between tests.
- Long term: make skill initialization accept a context and return a cleanup function, rather than relying on global state.

**Detection:** Run `go test -count=2` on any package that calls `skill.InitAll()`. If the second run fails, there's leaked state.

**Phase:** Phase 1 (Test Harness) -- must be addressed in shared test infrastructure.

---

### Pitfall 8: MCP Round-Trip Tests Missing Error Path Coverage

**What goes wrong:** Integration tests only cover the happy path: call a tool, get a result, assert it's correct. But MCP tools can fail in many ways: LS not installed, file not found, symbol not found, timeout, workspace not activated. The test suite has 100% pass rate but misses that error responses are malformed, error codes are wrong, or errors crash the daemon.

**Why it happens:** Happy-path tests are easy and satisfying to write. Error-path tests require setting up failure conditions (stopping the LS mid-request, providing invalid URIs, filling disk). It's boring work that gets deferred.

**Consequences:** An agent sends an invalid request, gets a malformed error, retries in a loop, burns context tokens. Or: an error in one tool crashes the daemon, killing all other sessions.

**Prevention:**
- For every tool, write at least one error-path test:
  - Tool called before workspace activated -> structured MCP error, not panic
  - Tool called with non-existent file -> structured error with file path
  - Tool called with non-existent symbol -> structured error, not empty result
  - LS crashes during request -> circuit breaker activates, error returned, daemon survives
- Test that a sequence of 10 error calls doesn't leak goroutines or crash the daemon.
- Use the MCP structured error format consistently. Test that error responses include `code`, `message`, and `data` fields per MCP spec.
- The existing `diag.DiagnosticStore` and `edit.BodyExtractor` paths need particular attention -- tree-sitter parsing failures and LS timeouts are common edge cases.

**Detection:** Run a fuzzer that sends random tool names and malformed arguments to the MCP server for 60 seconds. The daemon must not crash.

**Phase:** Phase 2 (Tool Tests) -- after happy-path tests are working.

---

### Pitfall 9: Multi-Language Fixture Projects Too Large or Too Small

**What goes wrong:** Fixtures are either trivially small (single file, one function) and don't exercise real LS behavior (cross-file references, imports, type hierarchies), or they're copied from real projects and are large enough that LS indexing takes 10+ seconds, making tests slow.

**Why it happens:** There's no obvious "right size" for a test fixture. Too small means the LS doesn't behave like it does on real projects (gopls with a single file doesn't load packages the same way). Too large means every test pays a multi-second indexing cost.

**Consequences:** Small fixtures give false confidence -- tests pass but the tool fails on real projects. Large fixtures make the suite take 10+ minutes, discouraging frequent runs.

**Prevention:**
- Target "minimum viable project" per language:
  - **Go:** 3-4 files across 2 packages with a `go.mod`. Includes: struct with methods, interface implementation, cross-package call, test file. ~100 lines total.
  - **Python:** 3 files with imports between them, a class with inheritance, type hints. ~80 lines.
  - **TypeScript:** 3 files with ES module imports, interface, class, generic function. `tsconfig.json` + `package.json`. ~100 lines.
  - **Rust:** `Cargo.toml` + 2 files in `src/` with struct, trait impl, cross-module use. ~80 lines.
- Each fixture must have a `SYMBOLS.md` documenting: every symbol name, its location, what `find_references` should return, what `get_definition` should resolve to. This is the test oracle.
- Pre-index fixtures in CI setup (run the LS against each fixture once before tests start) to separate indexing time from test time.
- Store fixtures in `testdata/fixtures/<language>/` following Go convention. Git-track them.

**Detection:** Measure LS startup + indexing time per fixture. Target <3s for each. If any exceeds 5s, the fixture is too large.

**Phase:** Phase 1 (Fixtures) -- design fixtures before writing tool tests.

---

### Pitfall 10: Test Harness Couples to Daemon Internals

**What goes wrong:** Integration tests directly call `daemon.New()`, `skill.InitAll()`, `kernel.ActivateWorkspace()` etc., bypassing the MCP protocol layer. They test internal Go APIs, not the MCP interface that agents actually use. A refactor of internal APIs breaks all integration tests without any actual behavior change.

**Why it happens:** Testing through the full MCP protocol (stdio or HTTP) is harder to set up than calling Go functions directly. The existing `daemon_integration_test.go` uses this pattern -- it calls `executor.ExecuteTool()` directly rather than going through MCP JSON-RPC.

**Consequences:** Tests don't catch MCP serialization bugs, parameter validation issues, middleware behavior (profile filtering), or transport-level problems. Internal refactors break tests. Tests give false confidence about the MCP interface.

**Prevention:**
- Build the integration test harness around an MCP client, not around internal Go APIs. The test sends a JSON-RPC `tools/call` request and receives a JSON-RPC response, just like a real agent would.
- Use the MCP Go SDK's client to connect to the test daemon via HTTP transport (simpler than stdio for tests, no child process management).
- Start the daemon in-process with `httptest.Server` wrapping the MCP HTTP handler. Tests connect to `http://localhost:<random-port>/mcp`.
- Keep the existing direct-API tests as *unit* tests for internal components. Add a separate `integration_test` package that only uses the MCP client interface.
- This also tests profile filtering, middleware, session management, and error serialization for free.

**Detection:** Ask "would this test still pass if I completely reimplemented the daemon's internal structure?" If no, it's testing internals, not behavior.

**Phase:** Phase 1 (Test Harness) -- this is the single most important architectural decision for the test suite.

## Minor Pitfalls

### Pitfall 11: Parallel Tests Compete for Same TCP/Socket Ports

**What goes wrong:** Two parallel test packages both start a daemon on the same socket path or HTTP port. Second test fails with "address already in use."

**Prevention:** Each test gets a unique socket path (from `shortSocketPath` pattern) and uses `localhost:0` for HTTP (OS assigns a random free port). Never hardcode ports in tests. The `httptest.NewServer()` pattern handles this automatically.

**Phase:** Phase 1 (Test Harness).

---

### Pitfall 12: Tree-Sitter Grammar Binaries Missing in Test Environment

**What goes wrong:** Edit tools use `go-tree-sitter` for body extraction. Tree-sitter grammars are compiled C shared libraries. If CGO is disabled in CI (`CGO_ENABLED=0`) or the grammars aren't linked, edit tool tests fail at runtime.

**Prevention:** Verify CGO is enabled in the CI workflow. The `go-tree-sitter` bindings embed grammars as Go source via code generation -- confirm this works in the CI build step before running tests. Add a smoke test that parses a trivial Go file with tree-sitter as a canary.

**Phase:** Phase 1 (CI Setup).

---

### Pitfall 13: SQLite FTS5 and Memory Tests Leave Database Files

**What goes wrong:** Memory skill tests create SQLite databases in temp dirs. If cleanup fails or the test panics, database files (and WAL journals) persist, consuming disk space. On CI runners with limited `/tmp` space, this causes failures in later tests.

**Prevention:** Use `t.TempDir()` for all database paths (Go automatically cleans these up even on panic). Verify `memory.Store.Close()` is called via `t.Cleanup()`. The `modernc.org/sqlite` CGO-free driver should handle this correctly, but verify WAL mode doesn't leave journal files.

**Phase:** Phase 1 (Test Harness).

---

### Pitfall 14: Snapshot-Based Assertions Become Unmaintainable

**What goes wrong:** Tests capture the full JSON response from a tool call and compare it against a golden file. Any change to response format, LS version, or fixture structure requires updating dozens of snapshots. Engineers do bulk `--update-snapshots` without reviewing diffs, defeating the purpose.

**Prevention:** Assert specific fields, not full responses. Use `assert.Contains`, `assert.JSONPath`, or structured assertions: "response has a `name` field matching `Daemon`" rather than "response equals this 50-line JSON blob." Reserve snapshots for complex outputs where structural assertions are impractical, and keep snapshot count low (<20).

**Phase:** Phase 2 (Tool Tests).

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Test Harness | LS timing flakes (#1) | Build await/ready infrastructure FIRST, before any tool tests |
| Test Harness | Coupling to internals (#10) | Use MCP client over HTTP, not direct Go API calls |
| Test Harness | Fixture mutation (#2) | Copy-to-tempdir pattern, enforced by helper |
| Test Harness | Worker leak (#3) | Shared pool in TestMain with low TTL and worker count limit |
| Test Harness | Socket path length (#4) | Short socket path helper, tested on macOS CI |
| CI Setup | Missing LS (#5) | Explicit install step with pinned versions, skip helpers |
| CI Setup | Tree-sitter/CGO (#12) | Smoke test for tree-sitter in CI, verify CGO_ENABLED |
| Dogfooding Suite | Structural coupling (#6) | Behavioral assertions, anchor symbols, live-codebase is read-only |
| Tool Tests | Missing error paths (#8) | Require 1 error test per tool as a review checklist item |
| Fixtures | Size calibration (#9) | Minimum viable project per language, <3s indexing target |
| Skill Tests | Global state leak (#7) | skill.ResetAll() + t.Cleanup() pattern |

## Sources

- [gopls integration test framework](https://pkg.go.dev/golang.org/x/tools/gopls/internal/test/integration) -- gold standard for LS testing patterns (HIGH confidence)
- [golang/go#62614: TB.TempDir paths too long for Unix sockets](https://github.com/golang/go/issues/62614) -- macOS socket path issue (HIGH confidence)
- [Go blog: Testing Time](https://go.dev/blog/testing-time) -- async test timing (HIGH confidence)
- [golang/go#62244: flaky test support](https://github.com/golang/go/issues/62244) -- Go's approach to flaky tests (HIGH confidence)
- [Lifecycle management in Go tests](https://rednafi.com/go/lifecycle_management_in_tests/) -- cleanup patterns (MEDIUM confidence)
- [MCP Best Practices](https://modelcontextprotocol.info/docs/best-practices/) -- MCP testing guidance (MEDIUM confidence)
- [MCP QA Guide](https://testcollab.com/blog/model-context-protocol-mcp-a-guide-for-qa-teams) -- MCP testing for QA teams (LOW confidence)
- [Istio Test Flakes Wiki](https://github.com/istio/istio/wiki/Test-Flakes) -- large-scale Go flaky test patterns (MEDIUM confidence)
- Existing Serena codebase: `daemon_test.go` (shortSocketPath), `daemon_integration_test.go` (skill lifecycle), `pool_test.go` (worker lifecycle) -- direct evidence (HIGH confidence)
