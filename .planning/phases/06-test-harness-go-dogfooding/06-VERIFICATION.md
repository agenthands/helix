---
phase: 06-test-harness-go-dogfooding
verified: 2026-04-08T22:15:00Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/4
  gaps_closed:
    - "Symbol retrieval, file operations, diagnostics, memory, workflow, and profile tools all return correct results when called against Serena's own Go source"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Remove extraneous 'scope' argument from TestHTTPSmoke_WithLS and rerun"
    expected: "search_symbols via HTTP returns results containing 'Helper'"
    why_human: "Verifier cannot modify code. Minor test bug -- extra arg rejected by HTTP schema validation. HTTP transport itself works (proven by ToolCallRoundTrip)."
---

# Phase 6: Test Harness + Go Dogfooding Verification Report

**Phase Goal:** Developers can run integration tests that spin up a real daemon and verify all tool categories against Serena's own Go code
**Verified:** 2026-04-08T22:15:00Z
**Status:** human_needed
**Re-verification:** Yes -- after gap closure (Plan 04 fixed Language field bug)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running `go test -tags integration ./...` executes integration tests while `go test ./...` skips them | VERIFIED | `go test ./...` produces 0 hits for test/integration; all 11 .go files have `//go:build integration` tag |
| 2 | A single test helper call starts a daemon in-process, connects an MCP client, and returns a ready-to-use test context with LS readiness polling | VERIFIED | `StartTestDaemon` in harness.go (line 83) creates daemon, wires InMemoryTransports, connects MCP client. `WaitForLS` (line 177) polls `search_symbols`. TestHarness_StartAndCallTool passes. |
| 3 | Symbol retrieval, file operations, diagnostics, memory, workflow, and profile tools all return correct results when called against Serena's own Go source | VERIFIED | All 9 symbol tools return real gopls data with strict assertions (callTool fatals on IsError). All 6 file ops pass with structural assertions. All 3 diagnostic tools pass. All 7 memory tools pass CRUD lifecycle. 2 workflow tools pass. 2 profile tools pass. Full test suite: 28/29 tests PASS, 1 FAIL (TestHTTPSmoke_WithLS -- minor test arg bug, not tool correctness). |
| 4 | Tests skip cleanly when gopls is not installed, enforce per-test timeouts, and leave no orphaned LS processes after teardown | VERIFIED | `requireGopls` (line 216) uses `t.Skip` with `exec.LookPath`. `WaitForLS` uses `context.WithTimeout`. `Stop()` cancels context stopping kernel Run goroutine and pool.stopAll. `runCtx` in pool.go ensures worker lifecycle is tied to pool, not request. |

**Score:** 4/4 truths verified

### Gap Closure Summary (Re-verification)

The single gap from the previous verification has been fully resolved:

| Previous Gap | Status | How Closed |
|-------------|--------|------------|
| Language field bug: all 9 symbol tools and 2/3 diagnostic tools returned circuit breaker errors | CLOSED | Plan 04 fixed three interconnected bugs: (1) workspace.go AcquireSession populates key.Language from detected languages, (2) daemon.go activeWSLang populated from rt.Languages(), (3) pool.go worker processes use pool lifecycle context instead of request context, (4) symbols/tools.go pathToURI resolves relative paths. Tests updated to strict assertions. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/daemon/daemon.go` | MCPServer() + KernelInstance() accessors + activeWSLang fix | VERIFIED | Line 215: MCPServer(), Line 218: KernelInstance(), Line 132: activeWSLang, Line 190: populated from rt.Languages() |
| `internal/kernel/workspace.go` | Language field populated in AcquireSession | VERIFIED | Lines 90-103: reads w.languages[0], sets key.Language before pool.AcquireLease |
| `internal/kernel/lspool/pool.go` | runCtx for worker lifecycle | VERIFIED | Line 54: runCtx field, Line 77: stored from Run(), Lines 264-265: used in spawnWorkerLocked |
| `test/integration/harness.go` | StartTestDaemon, WaitForLS, PrepareFixture, requireGopls, NewHTTPSession | VERIFIED | 279 lines, all functions present and substantive |
| `test/integration/helpers.go` | textContent, callTool, callToolExpectError | VERIFIED | 46 lines, all helpers present |
| `test/integration/harness_test.go` | Self-test proving harness works | VERIFIED | 33 lines, TestHarness_StartAndCallTool + TestHarness_FixtureActivateAndReadFile |
| `testdata/fixtures/go/main.go` | Known Go symbols | VERIFIED | 28 lines: main, Helper, DemoStruct, Field, Value, UsingHelper |
| `testdata/fixtures/go/pkg/greeter.go` | Cross-file symbols | VERIFIED | 21 lines: Greeter interface, SimpleGreeter, Greet, NewGreeter |
| `testdata/fixtures/go/go.mod` | Go module file | VERIFIED | 3 lines, module github.com/postfix/serena-fixture |
| `test/integration/symbols_test.go` | 9 symbol tool tests with strict assertions | VERIFIED | 166 lines, 9 subtests in TestSymbols_GoFixture + 2 in TestSymbols_CodebaseSmoke. Uses callTool (strict). No callToolBehavioral. No SkipLS:true. |
| `test/integration/fileops_test.go` | 6 file ops tests | VERIFIED | 115 lines, 6 subtests, all PASS with structural assertions |
| `test/integration/diag_test.go` | 3 diagnostic tests with strict assertions | VERIFIED | 65 lines, 3 subtests. Uses callTool (strict). No SkipLS:true. No manual LS poll. |
| `test/integration/memory_test.go` | 7 memory tool tests | VERIFIED | 74 lines, full CRUD lifecycle, all PASS |
| `test/integration/workflow_test.go` | 2 workflow tool tests | VERIFIED | 26 lines, onboard_project + prepare_for_new_conversation, all PASS |
| `test/integration/profile_test.go` | 2 profile tool tests | VERIFIED | 34 lines, get_token_budget + switch_mode, all PASS |
| `test/integration/smoke_http_test.go` | HTTP transport smoke test | VERIFIED | 83 lines, ToolCallRoundTrip PASS. WithLS fails due to minor test arg bug (extra "scope" param). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| harness.go | internal/daemon | daemon.New() + MCPServer() + KernelInstance() | WIRED | Lines 108, 64, 117 |
| harness.go | MCP SDK | NewInMemoryTransports + NewClient | WIRED | Lines 121, 131 |
| symbols_test.go | harness.go | StartTestDaemon + callTool (strict) | WIRED | Lines 19, 24 |
| symbols_test.go | testdata/fixtures/go/ | PrepareFixture | WIRED | Line 18 |
| smoke_http_test.go | server.go | HTTPHandler() via httptest.NewServer | WIRED | NewHTTPSession at harness.go |
| memory_test.go | skill/memory/ | MCP CallTool protocol path | WIRED | 7 tool calls via callTool helper |
| workspace.go | lspool/pool.go | AcquireSession passes key with Language to AcquireLease | WIRED | Line 102 |
| daemon.go | workspace.go | activate callback reads Languages() from runtime | WIRED | Line 190 |

### Data-Flow Trace (Level 4)

Not applicable -- test infrastructure phase, no dynamic data rendering.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build tag gating | `go test ./...` grep for test/integration | 0 matches | PASS |
| Production build | `go build ./cmd/serena` | BUILD OK | PASS |
| Go vet (integration) | `go vet -tags integration ./test/integration/...` | Clean exit | PASS |
| MCP round-trip (non-LS) | TestHarness_StartAndCallTool | PASS | PASS |
| Symbol tools (9, strict) | TestSymbols_GoFixture | All 9 subtests PASS with real gopls data | PASS |
| Codebase smoke | TestSymbols_CodebaseSmoke | 2 subtests PASS | PASS |
| File ops (6) | TestFileOps_GoFixture | All 6 subtests PASS | PASS |
| Diagnostics (3, strict) | TestDiag_GoFixture | All 3 subtests PASS | PASS |
| Memory CRUD (7) | TestMemory_CRUD | PASS | PASS |
| Workflow tools (2) | TestWorkflow_Onboard | 2 subtests PASS | PASS |
| Profile tools (2) | TestProfile_ModeAndBudget | 2 subtests PASS | PASS |
| HTTP round-trip | TestHTTPSmoke_ToolCallRoundTrip | PASS | PASS |
| HTTP + LS | TestHTTPSmoke_WithLS | FAIL (extra "scope" arg rejected by HTTP schema validation) | FAIL |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| HARN-01 | 06-01 | Build tag gating | SATISFIED | All 11 files have //go:build integration; go test ./... skips them |
| HARN-02 | 06-01 | Reusable harness starts daemon in-process | SATISFIED | StartTestDaemon creates daemon.New(), returns TestDaemon with Session |
| HARN-03 | 06-01 | Full MCP protocol round-trips | SATISFIED | InMemoryTransports + HTTP via StreamableClientTransport both work |
| HARN-04 | 06-01 | Tests skip when LS not installed | SATISFIED | requireGopls uses exec.LookPath + t.Skip |
| HARN-05 | 06-01 | Per-test timeouts and LS cleanup | SATISFIED | context.WithTimeout in WaitForLS; t.Cleanup(td.Stop) cancels context; runCtx manages worker lifecycle |
| HARN-06 | 06-01, 06-04 | LS readiness polling | SATISFIED | WaitForLS polls search_symbols until success. With Language fix, gopls indexes within timeout. |
| DOG-01 | 06-02, 06-04 | 9 symbol retrieval tools return correct results | SATISFIED | TestSymbols_GoFixture: all 9 pass with strict assertions, real gopls data |
| DOG-02 | 06-02 | 6 file operation tools work correctly | SATISFIED | TestFileOps_GoFixture: all 6 pass with structural assertions |
| DOG-03 | 06-02, 06-04 | 3 diagnostic tools return valid results | SATISFIED | TestDiag_GoFixture: all 3 pass with strict assertions |
| DOG-04 | 06-03 | Memory tools work end-to-end | SATISFIED | TestMemory_CRUD: full CRUD lifecycle (7 tools) passes |
| DOG-05 | 06-03 | Workflow tools execute successfully | SATISFIED | TestWorkflow_Onboard: onboard_project and prepare_for_new_conversation pass |
| DOG-06 | 06-03 | Profile tools function correctly | SATISFIED | TestProfile_ModeAndBudget: get_token_budget and switch_mode pass |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| smoke_http_test.go | 78 | Extra "scope" argument to search_symbols rejected by HTTP schema validation | Warning | Causes TestHTTPSmoke_WithLS to FAIL. Trivial fix: remove `"scope": "workspace"` from args. HTTP transport works (ToolCallRoundTrip proves it). |

### Human Verification Required

### 1. Fix TestHTTPSmoke_WithLS test argument bug

**Test:** Remove `"scope": "workspace"` from the search_symbols arguments in `test/integration/smoke_http_test.go` line 78, then run `go test -tags integration -count=1 -run TestHTTPSmoke_WithLS ./test/integration/... -timeout 120s`
**Expected:** Test passes -- search_symbols returns results containing "Helper" via HTTP transport
**Why human:** Verifier cannot modify code. This is a minor test bug (extra argument not in the tool schema), not a phase goal failure. The HTTP transport itself works correctly as proven by TestHTTPSmoke_ToolCallRoundTrip.

### Gaps Summary

No gaps remain from the previous verification. The Language field bug has been fully resolved by Plan 04, which fixed three interconnected bugs:

1. **workspace.go**: AcquireSession reads detected languages and populates key.Language
2. **daemon.go**: activeWSLang populated from rt.Languages() after activation; leaseFn uses it
3. **pool.go**: Worker processes use pool lifecycle context (runCtx) instead of request context
4. **symbols/tools.go**: pathToURI resolves relative paths against workspace root

All 9 symbol tools and all 3 diagnostic tools now return real gopls data with strict assertions. The callToolBehavioral workaround has been fully removed. All 12 requirements (HARN-01 through HARN-06, DOG-01 through DOG-06) are satisfied.

The only failing test (TestHTTPSmoke_WithLS) is a minor test argument bug unrelated to phase goal achievement.

---

_Verified: 2026-04-08T22:15:00Z_
_Verifier: Claude (gsd-verifier)_
