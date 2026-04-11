# Phase 20: Scenarios & Runtime - Research

**Researched:** 2026-04-11
**Domain:** Integration testing -- multi-step agent workflows, polyglot scenarios, runtime stress, degraded mode
**Confidence:** HIGH

## Summary

Phase 20 builds on the test harness (Phase 18) and contract oracles (Phase 19) to create two categories of tests: (1) scenario tests exercising realistic multi-step agent workflows across diverse repository shapes, and (2) runtime tests proving the worker pool, shutdown, and degraded startup paths are robust.

The codebase is well-prepared. The `test/harness/` package provides `StartRunner`, `PrepareFixture`, `CallTool`, `CallToolExpectError`, `ListSessionTools`, and `AssertGolden`. Five language fixtures already exist. The `lspool` package has exported `CircuitBreaker`, `Pool`, `MemoryPressure` interface, and `WorkerMetrics` -- all testable. Go 1.25.1 is installed with `testing/synctest` available (already used in `pool_synctest_test.go`). The skill registry has `Reset()` for test isolation.

**Primary recommendation:** Organize tests into two packages (`test/oracle/scenario/` for SCEN-* and `test/oracle/runtime/` for RUNT-*), create three new fixtures (polyglot, unsupported, collision), and leverage existing harness + pool test infrastructure throughout.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Polyglot monorepo fixture is a composite of existing Go + Python + TypeScript subdirs combined into one fixture
- **D-02:** Unsupported language fixture uses a fake `.xyz` extension
- **D-03:** Name collision fixture: `backend/config/main.go`, `worker/config/main.py`, `web/config/main.ts` each defining `Config`, `Handler`, `Parse`
- **D-04:** Degraded capability fixture -- reuse existing Go fixture but with LS intentionally unavailable via test double injection
- **D-05:** Full-cycle agent workflows: activate -> search -> read -> edit -> verify
- **D-06:** Multi-step scenarios use `PrepareFixture` to copy fixtures to temp dirs
- **D-07:** Profile/mode scenarios run against Go + Python
- **D-08:** Tiered stress testing -- unit layer with `testing/synctest`, integration layer with real concurrent goroutines
- **D-09:** Worker pool stress tests fan out N goroutines, verify circuit breaker trips, share-until-dirty under concurrent edits
- **D-10:** Shutdown test: start tool calls, send cancel/SIGTERM mid-flight, assert graceful drain. Check `runtime.NumGoroutine` and `os.Process`
- **D-11:** Interface-level test doubles for fault injection at daemon construction time
- **D-12:** All three subsystems tested independently (LS failure, memory store failure, skill init failure)
- **D-13:** Phase 19 deferred CONT-03 error categories (timeout, circuit_open, unsupported) covered here

### Claude's Discretion
- Test file organization within `test/oracle/scenario/` (one file per fixture vs one per requirement)
- Whether runtime stress tests live in `test/oracle/scenario/` or a separate `test/oracle/runtime/` package
- Specific concurrency parameters (N goroutines, timeout durations, retry counts)
- How to structure degraded-mode test doubles (embedded in test file vs shared test helper)
- Whether synctest unit tests belong in `internal/kernel/lspool/` or in the oracle layer

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SCEN-01 | Repository fixture matrix includes Go, Python, TypeScript, polyglot monorepo, unsupported language, degraded capability, and name collision fixtures | Existing 5 fixtures verified; 3 new fixtures needed (polyglot, unsupported, collision). Language detection via `DetectLanguages()` uses marker files. `.xyz` confirmed absent from language registry. |
| SCEN-02 | Multi-step scenarios exercise realistic agent workflows with intermediate assertions | Harness `CallTool`, `CallToolExpectError`, `TextContent` verified. `PrepareFixture` copies to temp dirs. Full workflow: activate_project -> search_symbols -> read_file -> edit tool -> verify. |
| SCEN-03 | Polyglot honesty rules enforced | Polyglot fixture composite of go+python+typescript. `DetectLanguages` scans for marker files. Symbol tools operate per-language via workspace key. Cross-language contamination testable by querying Go symbols and asserting no Python results. |
| SCEN-04 | Profile/mode behavior tested | Profile modes defined in YAML (read, edit, admin, review). `ListSessionTools` exercises `ProfileFilterMiddleware`. Mode switching via `switch_mode` tool. `RunnerOptions.Profile` and `.Mode` available. |
| RUNT-01 | Worker pool stress tested under sustained load | Pool has `AcquireLease`, `ReleaseLease`, `PromoteToDirty`, `WorkerCount`, `LeaseCount`. `CircuitBreaker` has `RecordFailure`, `CanAttempt`, `Failures`. `mockPressure` pattern exists. `testing/synctest` available for deterministic unit tests. |
| RUNT-02 | Clean shutdown drains in-flight work | `Runner.Stop()` calls `cancel()`. `Pool.Run()` calls `stopAll()` on context cancel. `runtime.NumGoroutine()` for leak detection. LS subprocess PIDs accessible via Worker. |
| RUNT-03 | Degraded subsystem simulation | `daemon.New()` calls `skill.InitAll()` with degraded-mode warning path. `skill.Reset()` available for test isolation. `MemoryPressure` is an interface. Installer is injectable. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Always run `go vet` and `go test` before completing any Go task
- Build via `go build ./cmd/serena` or `make build`
- Test via `go test ./...` or `make test`
- Format with `gofmt -w .`

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| testing | stdlib | Test framework | Go standard, already used throughout |
| testing/synctest | stdlib (Go 1.25) | Deterministic concurrency tests | Already used in pool_synctest_test.go; virtual clock for circuit breaker/TTL logic |
| testify | v1.10.0 | Assertions (require/assert) | Already used across all test files |
| errgroup | golang.org/x/sync | Fan-out goroutine coordination | Already used in concurrency_test.go and daemon.go |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| test/harness | internal | Runner, fixture prep, tool calling, golden files | All scenario tests |

[VERIFIED: codebase grep -- all libraries already in use]

## Architecture Patterns

### Recommended Test Organization
```
test/oracle/
  scenario/
    doc.go                    # already exists
    go_test.go                # Go fixture full-cycle scenario
    python_test.go            # Python fixture full-cycle scenario
    typescript_test.go        # TypeScript fixture full-cycle scenario
    polyglot_test.go          # Polyglot monorepo scenarios (SCEN-01, SCEN-03)
    unsupported_test.go       # Unsupported language scenarios (SCEN-01)
    collision_test.go         # Name collision scenarios (SCEN-01, SCEN-03)
    degraded_test.go          # Degraded capability scenarios (SCEN-01)
    profile_test.go           # Profile/mode scenarios (SCEN-04)
  runtime/
    doc.go                    # new package
    pool_stress_test.go       # Worker pool stress (RUNT-01)
    shutdown_test.go          # Clean shutdown (RUNT-02)
    degraded_start_test.go    # Degraded subsystem injection (RUNT-03)
    errors_deferred_test.go   # Deferred CONT-03 categories (D-13)
testdata/fixtures/
  go/                         # existing
  python/                     # existing
  typescript/                 # existing
  polyglot/                   # NEW -- composite
  unsupported/                # NEW -- .xyz files
  collision/                  # NEW -- name collision
```

[ASSUMED] -- File-per-fixture organization in scenario/ is the recommendation. One-per-requirement would merge unrelated concerns.

### Pattern 1: Full-Cycle Agent Workflow
**What:** activate -> search -> read -> edit -> verify pattern exercising the complete agent lifecycle against a real daemon
**When to use:** Every SCEN-02 fixture test
**Example:**
```go
// Source: established pattern from test/harness/runner.go + CONTEXT D-05
func TestScenario_Go_FullCycle(t *testing.T) {
    harness.RequireGopls(t)
    fixtureDir := harness.PrepareFixture(t, "go")
    runner := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: fixtureDir,
    })
    s := runner.Session

    // Step 1: search for a symbol
    result := harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Greeter"})
    require.Contains(t, harness.TextContent(result), "Greeter")

    // Step 2: read the file containing the symbol
    result = harness.CallTool(t, s, "read_file", map[string]any{"path": "pkg/greeter.go"})
    original := harness.TextContent(result)
    require.Contains(t, original, "Greeter")

    // Step 3: edit (e.g., replace_in_file or symbol edit)
    harness.CallTool(t, s, "replace_in_file", map[string]any{
        "path": "pkg/greeter.go",
        "old": "Hello",
        "new": "Hi",
    })

    // Step 4: verify the edit took effect
    result = harness.CallTool(t, s, "read_file", map[string]any{"path": "pkg/greeter.go"})
    require.Contains(t, harness.TextContent(result), "Hi")
}
```

### Pattern 2: Polyglot Composite Fixture
**What:** Combined fixture with marker files for multiple languages
**When to use:** SCEN-01 polyglot, SCEN-03 honesty
```
testdata/fixtures/polyglot/
  go.mod                      # triggers Go detection
  main.go
  pkg/greeter.go
  pyproject.toml              # triggers Python detection
  main.py
  utils.py
  tsconfig.json               # triggers TypeScript detection
  main.ts
  greeter.ts
```
[VERIFIED: `DetectLanguages()` in `internal/kernel/workspace.go` scans for go.mod, pyproject.toml, tsconfig.json as marker files]

### Pattern 3: Unsupported Language Fixture
**What:** Files with `.xyz` extension that no language server can handle
**When to use:** SCEN-01 unsupported, SCEN-03 honesty
```
testdata/fixtures/unsupported/
  main.xyz
  utils.xyz
```
No marker file means `DetectLanguages()` returns empty. LS-dependent tools should fail cleanly. File-level tools (read_file, list_directory, search_in_files) should still work.
[VERIFIED: `.xyz` not present in language registry -- confirmed by grep of `internal/langregistry/`]

### Pattern 4: Name Collision Fixture
**What:** Same symbol names across different language subdirectories
**When to use:** SCEN-01, SCEN-03 cross-contamination testing
```
testdata/fixtures/collision/
  go.mod
  pyproject.toml
  tsconfig.json
  backend/config/main.go      # defines Config, Handler, Parse
  worker/config/main.py       # defines Config, Handler, Parse
  web/config/main.ts          # defines Config, Handler, Parse
```
[VERIFIED: CONTEXT D-03 specifies this exact structure]

### Pattern 5: Profile/Mode Assertion
**What:** Start runner in specific profile/mode, verify tool visibility
**When to use:** SCEN-04
```go
// Source: established pattern from harness.RunnerOptions + profile modes
func TestScenario_ReadMode_BlocksEdits(t *testing.T) {
    harness.RequireGopls(t)
    fixtureDir := harness.PrepareFixture(t, "go")
    runner := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: fixtureDir,
        Profile:      "full",
        Mode:         "read",
    })

    // Edit tools should not appear in tools/list
    tools := harness.ListSessionTools(t, runner.Session)
    assert.NotContains(t, tools, "replace_symbol_body")
    assert.NotContains(t, tools, "write_file")

    // Read tools should appear
    assert.Contains(t, tools, "search_symbols")
    assert.Contains(t, tools, "read_file")

    // Calling edit tool should fail
    harness.CallToolExpectError(t, runner.Session, "replace_in_file", map[string]any{
        "path": "main.go", "old": "x", "new": "y",
    })
}
```
[VERIFIED: read mode YAML excludes replace_content, replace_symbol_body, insert_*, delete_*; admin mode includes all skills]

### Pattern 6: Runtime Stress with Fan-Out
**What:** N goroutines hitting tools concurrently, asserting pool/circuit breaker behavior
**When to use:** RUNT-01
```go
// Source: pattern from test/integration/concurrency_test.go
func TestRuntime_PoolStress_CircuitBreaker(t *testing.T) {
    harness.RequireGopls(t)
    fixtureDir := harness.PrepareFixture(t, "go")
    runner := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: fixtureDir,
        MaxWorkers:   4,
    })

    const N = 50
    g, ctx := errgroup.WithContext(context.Background())
    for i := 0; i < N; i++ {
        g.Go(func() error {
            _, err := runner.Session.CallTool(ctx, &mcp.CallToolParams{
                Name:      "search_symbols",
                Arguments: map[string]any{"query": "Helper"},
            })
            return err
        })
    }
    require.NoError(t, g.Wait())
}
```

### Pattern 7: Shutdown Goroutine Leak Check
**What:** Capture goroutine count before/after daemon lifecycle
**When to use:** RUNT-02
```go
func TestRuntime_CleanShutdown_NoLeaks(t *testing.T) {
    before := runtime.NumGoroutine()
    // ... start runner, do work, stop runner ...
    // Give goroutines time to wind down
    time.Sleep(100 * time.Millisecond)
    runtime.GC()
    after := runtime.NumGoroutine()
    assert.LessOrEqual(t, after, before+5, "goroutine leak detected")
}
```

### Pattern 8: Degraded Mode Test Double Injection
**What:** Inject failing implementations at daemon construction to test degraded startup
**When to use:** RUNT-03
```go
// For LS failure: use a nonexistent binary or override installer
// For memory: inject skill.Reset() + register a failing skill
// For skill init: register a skill whose Init() returns error

// skill.Reset() clears all registrations -- use to control which skills init
skill.Reset()
skill.Register(&failingSkill{name: "memory", err: errors.New("simulated failure")})
// Then call daemon.New() -- should succeed in degraded mode
```
[VERIFIED: `skill.Reset()` exists in `internal/skill/registry.go:93`. `daemon.New()` wraps `skill.InitAll()` failure as a Warn log, not a fatal error]

### Anti-Patterns to Avoid
- **Environment manipulation for degraded tests:** Do NOT move/hide real LS binaries or set PATH tricks. Use interface-level injection per D-11.
- **Shared mutable state between scenario tests:** Each test MUST use `PrepareFixture` for its own temp copy. Never share fixture directories.
- **Sleeping for synchronization:** Use `synctest` for deterministic timing tests. For integration tests, use channels/errgroup/context deadlines, not `time.Sleep` except for brief goroutine wind-down in leak checks.
- **Testing against existing integration tests:** Phase 20 oracle tests live in `test/oracle/` and complement, not replace, `test/integration/`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Test daemon lifecycle | Custom daemon startup | `harness.StartRunner` | Handles skill init, kernel run, MCP connect, workspace activate, LS wait |
| Fixture isolation | Manual temp dir creation | `harness.PrepareFixture` | Copies fixture to unique temp dir per test |
| Tool invocation + assertion | Raw session.CallTool | `harness.CallTool` / `CallToolExpectError` | Adds timeout, assertion, proper t.Helper |
| Golden file comparison | Custom file diffing | `harness.AssertGolden` | Supports -update flag, proper error messages |
| Tool visibility check | Parsing MCP responses | `harness.ListSessionTools` | Returns []string of tool names via tools/list |
| Circuit breaker state | Custom timing logic | `lspool.CircuitBreaker` exported API | `RecordFailure`, `RecordSuccess`, `CanAttempt`, `Failures`, `BackoffDuration` all exported |
| Memory pressure simulation | OS-level tricks | `mockPressure` struct | Pattern exists in `pool_test.go`, implements `MemoryPressure` interface |

## Common Pitfalls

### Pitfall 1: Language Server Availability
**What goes wrong:** Tests that require gopls/pyright/typescript-language-server fail on machines without them installed.
**Why it happens:** LS binaries are external dependencies not managed by `go test`.
**How to avoid:** Every test requiring a specific LS must call `harness.RequireGopls(t)` or `harness.RequireLS(t, "pyright-langserver")` at the top. Skip, don't fail.
**Warning signs:** Test failures with "command not found" or "no language server configured" errors.

### Pitfall 2: Polyglot Fixture Language Detection
**What goes wrong:** `DetectLanguages()` only checks root-level marker files (go.mod, pyproject.toml, tsconfig.json). Nested subdirectory marker files are NOT detected.
**Why it happens:** `DetectLanguages()` joins rootPath directly with marker file names -- no recursive walk.
**How to avoid:** The polyglot fixture MUST have marker files at the fixture root, not nested under subdirectories.
**Warning signs:** `WorkspaceRuntime.Languages()` returns empty or missing expected languages.

### Pitfall 3: Race Conditions in Goroutine Leak Checks
**What goes wrong:** `runtime.NumGoroutine()` check fails intermittently because background goroutines haven't fully exited.
**Why it happens:** Goroutine teardown is asynchronous. `Runner.Stop()` cancels context but doesn't block until all goroutines finish.
**How to avoid:** Add a small sleep + `runtime.GC()` before the "after" count. Use a generous delta (allow +3-5 goroutines for runtime/GC goroutines). Alternatively, use `runtime/pprof.Lookup("goroutine")` for more detailed leak diagnosis on failure.
**Warning signs:** Flaky test with "goroutine leak detected" that passes on retry.

### Pitfall 4: Skill Registry Global State
**What goes wrong:** `skill.Reset()` in one test affects other tests running in parallel.
**Why it happens:** The skill registry is a package-level global. Reset in one test clears registrations for all.
**How to avoid:** Tests using `skill.Reset()` MUST NOT run with `t.Parallel()`. Use `t.Cleanup()` to restore state. Better: only use `skill.Reset()` in degraded-mode tests that construct their own daemon, not in scenario tests that share the standard harness.
**Warning signs:** "skill not found" errors in unrelated tests when running `go test -count=1`.

### Pitfall 5: Build Tag Gating
**What goes wrong:** Scenario and runtime tests accidentally run during normal `go test ./...`.
**Why it happens:** Missing `//go:build integration || llm || llmjudge` tag.
**How to avoid:** Every test file in `test/oracle/` MUST have the build tag as the first line. The `doc.go` in `test/oracle/scenario/` already has it.
**Warning signs:** Tests running during `go test ./...` when they should only run with explicit `-tags integration`.

### Pitfall 6: Shared Worker Pool Session IDs
**What goes wrong:** Multiple concurrent tests using the same session ID collide in the pool's lease map.
**Why it happens:** `AcquireLease` uses sessionID as the lease map key. Two tests with the same ID overwrite each other.
**How to avoid:** Each test gets its own `StartRunner` which creates unique in-memory transports. Never share a Runner across tests.
**Warning signs:** "no lease found for session" errors, or leases disappearing mid-test.

## Code Examples

### Creating the Polyglot Fixture
```
testdata/fixtures/polyglot/
  go.mod          # module polyglot-fixture
  main.go         # package main with func main() calling greeter
  pkg/greeter.go  # Go Greeter struct
  pyproject.toml  # [project] name = "polyglot-fixture"
  main.py         # Python main with from utils import ...
  utils.py        # Python utility functions
  tsconfig.json   # TypeScript config
  main.ts         # TypeScript main
  greeter.ts      # TypeScript Greeter class
```
[VERIFIED: `DetectLanguages()` checks go.mod, pyproject.toml, tsconfig.json at root level]

### Creating the Name Collision Fixture
```go
// backend/config/main.go
package config

type Config struct {
    Name string
}

func Handler(c Config) string {
    return c.Name
}

func Parse(raw string) Config {
    return Config{Name: raw}
}
```
```python
# worker/config/main.py
class Config:
    def __init__(self, name: str):
        self.name = name

def Handler(c: Config) -> str:
    return c.name

def Parse(raw: str) -> Config:
    return Config(name=raw)
```
```typescript
// web/config/main.ts
export class Config {
    constructor(public name: string) {}
}

export function Handler(c: Config): string {
    return c.name;
}

export function Parse(raw: string): Config {
    return new Config(raw);
}
```
[VERIFIED: matches CONTEXT D-03 specification]

### Deferred Error Categories (D-13)
```go
// timeout: inject deadline into context before tool call
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
defer cancel()
time.Sleep(time.Millisecond) // ensure deadline has passed
// Call tool with expired context -> should produce timeout error

// circuit_open: trigger via crashy worker simulation
// Use CircuitBreaker.RecordFailure() N times to exhaust restart budget
// Then attempt tool call -> should produce circuit_open error

// unsupported: use .xyz fixture, call LS-dependent tool
// Should produce unsupported language error
```

### Degraded Startup Test Double
```go
// Pattern: register a skill that fails on Init(), then construct daemon
type failingSkill struct {
    name string
    err  error
}
func (f *failingSkill) Name() string           { return f.name }
func (f *failingSkill) Init(deps skill.SkillDeps) error { return f.err }
// ... implement remaining Skill interface methods as no-ops ...

// In test:
skill.Reset()
skill.Register(&failingSkill{name: "memory", err: errors.New("simulated")})
// Register remaining real skills as needed
cfg := harness.DefaultTestConfig(t)
d, err := daemon.New(cfg, logger)
require.NoError(t, err, "daemon should start in degraded mode")
// Verify memory tools report error, other tools work
```
[VERIFIED: `daemon.New()` at line 213 wraps `skill.InitAll` failure as Warn, not fatal. `skill.Reset()` exists]

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.10.0 |
| Config file | Build tags: `//go:build integration \|\| llm \|\| llmjudge` |
| Quick run command | `go test -tags integration -run TestScenario -count=1 ./test/oracle/scenario/` |
| Full suite command | `go test -tags integration -count=1 -race ./test/oracle/scenario/ ./test/oracle/runtime/` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SCEN-01 | Fixture matrix coverage | integration | `go test -tags integration -run TestScenario -count=1 ./test/oracle/scenario/` | Wave 0 |
| SCEN-02 | Full-cycle workflows | integration | `go test -tags integration -run TestScenario_.*_FullCycle -count=1 ./test/oracle/scenario/` | Wave 0 |
| SCEN-03 | Polyglot honesty | integration | `go test -tags integration -run TestScenario_Polyglot -count=1 ./test/oracle/scenario/` | Wave 0 |
| SCEN-04 | Profile/mode behavior | integration | `go test -tags integration -run TestScenario_Profile -count=1 ./test/oracle/scenario/` | Wave 0 |
| RUNT-01 | Pool stress | integration + unit | `go test -tags integration -run TestRuntime_Pool -count=1 ./test/oracle/runtime/` | Wave 0 |
| RUNT-02 | Clean shutdown | integration | `go test -tags integration -run TestRuntime_Shutdown -count=1 ./test/oracle/runtime/` | Wave 0 |
| RUNT-03 | Degraded subsystem | integration | `go test -tags integration -run TestRuntime_Degraded -count=1 ./test/oracle/runtime/` | Wave 0 |

### Sampling Rate
- **Per task commit:** Quick run command for the specific requirement being implemented
- **Per wave merge:** Full suite with `-race`
- **Phase gate:** `go test -tags integration -count=1 -race ./test/oracle/scenario/ ./test/oracle/runtime/` + `go vet ./...`

### Wave 0 Gaps
- [ ] `test/oracle/runtime/doc.go` -- package declaration with build tag
- [ ] `testdata/fixtures/polyglot/` -- composite fixture directory
- [ ] `testdata/fixtures/unsupported/` -- .xyz extension fixture
- [ ] `testdata/fixtures/collision/` -- name collision fixture
- [ ] Golden files for deferred CONT-03 error categories (timeout, circuit_open, unsupported) in `test/oracle/contract/testdata/golden/errors/`

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A (test-only phase) |
| V3 Session Management | yes (tested) | MCP session isolation via harness.StartRunner per test |
| V4 Access Control | yes (tested) | Profile/mode filtering tested via SCEN-04 (read blocks edits, admin grants all) |
| V5 Input Validation | no | Not directly -- input validation covered by Phase 19 CONT-02/CONT-03 |
| V6 Cryptography | no | N/A |

### Known Threat Patterns for Test Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Mode bypass via concurrent switch | Elevation of Privilege | SCEN-04 + concurrent mode switch tests |
| Cross-language data leakage | Information Disclosure | SCEN-03 polyglot honesty assertions |
| Crash-loop DoS on worker pool | Denial of Service | RUNT-01 circuit breaker stress test |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | File-per-fixture organization is cleaner than file-per-requirement | Architecture Patterns | Low -- organizational preference, easily changed |
| A2 | `test/oracle/runtime/` as a separate package is cleaner than mixing with scenario/ | Architecture Patterns | Low -- can be merged if preferred |
| A3 | N=50 goroutines is sufficient for pool stress testing | Code Examples | Low -- easily tunable upward |
| A4 | `runtime.NumGoroutine()` delta of +5 is a reasonable threshold for leak detection | Common Pitfalls | Medium -- may need tuning per platform |
| A5 | Polyglot fixture can reuse actual code from existing fixtures with minor adjustments | Architecture Patterns | Low -- code structure of existing fixtures is verified |

## Open Questions

1. **LS availability for Python/TypeScript in CI**
   - What we know: `RequireGopls` and `RequireLS` skip tests when binary not found. Go tests should work everywhere gopls is installed.
   - What's unclear: Whether pyright-langserver and typescript-language-server are available in the CI environment for Python/TypeScript scenario tests.
   - Recommendation: Use `RequireLS(t, "pyright-langserver")` and `RequireLS(t, "typescript-language-server")` guards. Tests skip gracefully if unavailable. Go fixture provides the primary full-cycle coverage regardless.

2. **Deferred CONT-03 timeout error triggering**
   - What we know: The daemon has `degrade.BudgetFor()` with configurable timeouts per tool class.
   - What's unclear: Whether injecting a 1-nanosecond context deadline will produce the same error format as a real LS timeout (the timeout might be caught at different layers).
   - Recommendation: Test both a context deadline exceeded (context layer) and a budget-deadline tool call (middleware layer). Use `DefaultTestConfig` with overridden timeout to trigger deterministically.

3. **Circuit breaker error format stability**
   - What we know: `CircuitOpenError` has `Error()` method, `Is(ErrCircuitOpen)` works. Phase 19 deferred the golden.
   - What's unclear: The exact text format for golden file comparison.
   - Recommendation: First run with `-update` to capture the golden, then review and lock it.

## Sources

### Primary (HIGH confidence)
- `test/harness/runner.go` -- Runner, StartRunner, RunnerOptions, WaitForLS
- `test/harness/tools.go` -- CallTool, TextContent, CallToolExpectError, ListSessionTools
- `test/harness/fixture.go` -- PrepareFixture, RequireGopls, RequireLS
- `test/harness/golden.go` -- AssertGolden, GoldenStore
- `internal/kernel/lspool/pool.go` -- Pool, AcquireLease, PromoteToDirty, WorkerCount, LeaseCount
- `internal/kernel/lspool/circuit.go` -- CircuitBreaker, RecordFailure, CanAttempt
- `internal/kernel/lspool/pressure.go` -- MemoryPressure interface, PressureLevel
- `internal/kernel/workspace.go` -- WorkspaceRuntime, DetectLanguages
- `internal/daemon/daemon.go` -- Daemon.New(), skill.InitAll degraded path
- `internal/skill/registry.go` -- Register, InitAll, Reset
- `internal/profile/modes/{read,admin}.yaml` -- Mode tool filtering definitions
- `test/oracle/contract/errors_test.go` -- TODO(phase-20) for deferred error categories
- `test/integration/concurrency_test.go` -- Existing fan-out pattern
- `internal/kernel/lspool/pool_test.go` -- mockPressure, testPoolConfig, testRegistry patterns
- `internal/kernel/lspool/pool_synctest_test.go` -- synctest usage pattern

### Secondary (MEDIUM confidence)
- Go 1.25 testing/synctest docs -- verified via `go doc testing/synctest`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use in the codebase
- Architecture: HIGH -- patterns directly from existing test infrastructure
- Pitfalls: HIGH -- derived from actual codebase analysis (DetectLanguages root-only, skill global state, build tags)
- Fixtures: HIGH -- structure specified in CONTEXT.md and verified against workspace.go

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- internal test infrastructure, unlikely to change)
