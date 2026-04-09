# Phase 8: Advanced Testing - Research

**Researched:** 2026-04-08
**Domain:** Go integration testing (profile contracts, concurrency, error paths)
**Confidence:** HIGH

## Summary

Phase 8 closes v1.1 by adding four classes of tests on top of the Phase 6 harness:
profile contract tests (ADV-01), mode contract tests (ADV-02), worker-pool concurrency
stress (ADV-03), and error-path coverage (ADV-04). CONTEXT.md has locked the major
architectural choices — golden files for contracts, three-tier concurrency, three-band
error coverage — so this research is prescriptive: document exactly HOW to implement
those decisions against the existing code, not WHICH approach to take.

The harness in `test/integration/harness.go` already exposes `StartTestDaemon`,
`callTool`, and `callToolExpectError`; the gaps are (1) a way to pass profile+mode
into `StartTestDaemon`, (2) a helper that enumerates tools visible to the current
session, (3) golden-file machinery with `-update` flag, and (4) a concurrency-safe
daemon config (`MaxWorkers` currently defaults to 2 which is too low for stress).

**Primary recommendation:** Build one shared `golden` helper package + one shared
`errmatrix` table harness, then instantiate them across the four new test files
(`profile_golden_test.go`, `mode_golden_test.go`, `concurrency_test.go`,
`errors_test.go`). Extend `StartTestDaemon` with `Profile`/`Mode`/`MaxWorkers`
options rather than building a second harness.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Profile/Mode source of truth:**
- **D-01:** Use checked-in golden-file pattern. Each profile/mode gets its own golden file under `testdata/profiles/` (e.g., `testdata/profiles/claude-code.read.tools.golden`, `testdata/profiles/full.admin.tools.golden`) containing sorted tool name lists.
- **D-02:** Contract tests compare actual runtime tool list (from daemon/profile resolver) against the golden file. Use `-update` flag (or `GOLDEN_UPDATE=1`) for intentional updates — diffs must be reviewable in PR.
- **D-03:** DO NOT compute expected tool lists from runtime YAML (`.serena/profiles/*.yaml`) in contract tests — that would use the same source of truth as the code under test, masking bad YAML changes.
- **D-04:** Separate YAML loader/parser unit tests are allowed to read YAML dynamically (inheritance, defaults, merge semantics, validation). These test the loader, not the contract.

**Concurrency test design:**
- **D-05:** Layered three-tier approach — (1) scenario stress tests via `t.Parallel()`, (2) hot-path stress tests via explicit goroutine fan-out, (3) deterministic unit tests via `testing/synctest` where applicable.
- **D-06:** All concurrency tests must run under `-race`. CI has 3 jobs: `go test ./... -race` (baseline), stress-heavy fan-out with elevated counts, optional synctest job.
- **D-07:** Beware loop-variable capture after `t.Parallel()` — use `go vet` diagnostics to catch.

**Error path coverage:**
- **D-08:** Three bands — (1) representative per category, (2) exhaustive per destructive tool (edit × 6, rename_symbol, safe_delete_symbol, write_memory), (3) thin smoke for read-only tools.
- **D-09:** Go table-driven test style for error matrices.
- **D-10:** Assert on error type/code, not just message text.

### Claude's Discretion
- Exact structure of error matrix table (columns, fields)
- Golden file format (one tool per line vs JSON array) — **recommend one-tool-per-line for trivial diff review**
- Fan-out goroutine count for stress tests — **recommend 100 baseline, 500 in CI stress job**
- Whether to add a separate `make test-stress` target — **recommend yes, with `-count=5 -timeout=10m`**

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADV-01 | Each of 5 agent profiles exposes exactly the expected tool subset | Golden-file pattern (§Architecture) + extended `StartTestDaemon` with `Profile` option + `listSessionTools` helper |
| ADV-02 | Each of 4 modes filters tool visibility correctly | Same golden infrastructure; dimension becomes `profile × mode` (up to 20 files, though some modes can be skipped per profile's `allowed_mode_transitions`) |
| ADV-03 | Worker pool handles concurrent tool calls without races or deadlocks | Three-tier concurrency strategy (§Concurrency Patterns); exercises `pool.AcquireLease`/`ReleaseLease` + share-until-dirty + circuit breaker |
| ADV-04 | Error paths tested — tool called before workspace activation, file not found, symbol not found | Three-band error matrix (§Error Testing) + existing `callToolExpectError` helper + per-category harness |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Go only:** No Python, no new languages. Tests must be pure Go.
- **Mandatory commands before task completion:** `go vet ./...` and `go test ./...` must pass.
- **Build tag:** Integration tests already use `//go:build integration`. New files must follow the same pattern.
- **Single binary:** Do not introduce external test runners, shell scripts, or Python helpers. Golden-update flag must be Go-native.
- **MCP is the primary interface:** Contract tests must go through the MCP client session (`tools/list`), not scrape internal registries, so the ProfileFilterMiddleware is exercised end-to-end.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard `testing` | 1.25.1 | Test runner, parallel, subtests | [VERIFIED: go.mod `go 1.25.1`] Only test runner in use |
| `testing/synctest` | stdlib (Go 1.25 stable) | Deterministic concurrency tests | [CITED: https://pkg.go.dev/testing/synctest] Graduated from experiment in Go 1.24, stable in 1.25 — matches project's Go version exactly |
| `go test -race` | stdlib | Race detection | [VERIFIED] Already enabled in CONTEXT D-06 |
| `github.com/stretchr/testify` | already in deps | `assert`, `require` | [VERIFIED: harness uses it] Drop-in, no new dep |
| `github.com/modelcontextprotocol/go-sdk/mcp` | already in deps | MCP client for tool calls + `tools/list` | [VERIFIED: harness.go] Already wired via `InMemoryTransports` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `flag` (stdlib) | — | `-update` flag for golden files | Standard Go pattern — see §Code Examples |
| `os.Getenv` | — | `GOLDEN_UPDATE=1` env var alternative | For CI pipelines that can't pass test flags easily |
| `sort` (stdlib) | — | Canonical ordering for golden files | Sort tool names before write/compare |
| `errors.Is` / `errors.As` | stdlib | Error type assertion (D-10) | Works with wrapped errors in kernel tools |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom golden helper | `github.com/sebdah/goldie/v2` | New dep for ~30 lines of logic; project prefers stdlib — skip |
| `testing/synctest` | `github.com/uber-go/goleak` | goleak catches leaked goroutines but not scheduler determinism; **use both** — goleak in cleanup, synctest for scheduler-sensitive unit tests |
| Fan-out via `sync.WaitGroup` | `golang.org/x/sync/errgroup` | errgroup already in deps (daemon uses it); cleaner error propagation — **use errgroup** |

**Installation:** No new dependencies required. Everything already in `go.mod`.

**Version verification:** [VERIFIED: go.mod] `go 1.25.1`, `stretchr/testify`, `modelcontextprotocol/go-sdk` all present. `testing/synctest` is stdlib in Go 1.25 — no install needed.

## Architecture Patterns

### Recommended File Layout

```
test/integration/
├── harness.go                 # EXTEND: add Profile, Mode, MaxWorkers to Options
├── helpers.go                 # EXTEND: add listSessionTools, requireProfile
├── golden.go                  # NEW: golden file read/write/compare helper
├── profile_golden_test.go     # NEW: ADV-01 (5 profiles)
├── mode_golden_test.go        # NEW: ADV-02 (profile × mode combinations)
├── concurrency_test.go        # NEW: ADV-03 (three tiers)
└── errors_test.go             # NEW: ADV-04 (three bands)

testdata/profiles/
├── claude-code.edit.tools.golden   # default mode per profile
├── claude-code.read.tools.golden
├── claude-code.review.tools.golden
├── codex.<mode>.tools.golden
├── ide-assistant.<mode>.tools.golden
├── ci-bot.<mode>.tools.golden
└── full.<mode>.tools.golden        # full.admin.tools.golden etc.
```

**Golden file format (recommended):** one tool name per line, LF-terminated, sorted ascending. Trivial `diff` review, no JSON noise. Example:

```
activate_project
find_file
find_symbol
get_symbols_overview
list_dir
read_file
search_for_pattern
```

### Pattern 1: Golden File Helper

**What:** A shared helper that reads expected tool list from a golden file, compares to actual, and writes on `-update`.

**When to use:** Both `profile_golden_test.go` and `mode_golden_test.go`.

**Example:**
```go
// test/integration/golden.go (build tag: integration)
package integration_test

import (
    "flag"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func assertGoldenTools(t *testing.T, name string, actual []string) {
    t.Helper()
    path := filepath.Join("..", "..", "testdata", "profiles", name+".tools.golden")
    sort.Strings(actual)
    got := strings.Join(actual, "\n") + "\n"

    if *updateGolden || os.Getenv("GOLDEN_UPDATE") == "1" {
        if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
            t.Fatalf("update golden %s: %v", path, err)
        }
        return
    }

    wantBytes, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
    }
    if string(wantBytes) != got {
        t.Errorf("golden %s mismatch.\n--- want\n%s\n+++ got\n%s\n(run with -update to accept)",
            name, string(wantBytes), got)
    }
}
```

[CITED: https://pkg.go.dev/flag + Go testing idioms] `-update` flag pattern is the canonical Go approach used by `gofmt`, `cmd/go`, and the stdlib itself.

### Pattern 2: Tool Enumeration via MCP ListTools

**What:** List tools the session can actually see (post-ProfileFilterMiddleware).

**Why not read from ProfileStore directly:** Violates D-03 — that uses the same source of truth as the code under test. We must go through the MCP client.

**Example:**
```go
// test/integration/helpers.go addition
func listSessionTools(t *testing.T, session *mcp.ClientSession) []string {
    t.Helper()
    result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
    require.NoError(t, err, "tools/list")
    names := make([]string, 0, len(result.Tools))
    for _, tool := range result.Tools {
        names = append(names, tool.Name)
    }
    return names
}
```

[VERIFIED: `internal/mcp/middleware.go:24-80`] Confirmed: `ProfileFilterMiddleware` filters `tools/list` results based on `session.AllowedTools`, so calling `ListTools` through the client session gives the post-filter view — exactly what contract tests need.

### Pattern 3: Extend Harness for Profile+Mode Injection

**Current state:** `StartTestDaemon` hardcodes `cfg.Profile = "full"` in `defaultTestConfig`. Must parameterize.

**Example:**
```go
// Options struct addition
type Options struct {
    WorkspaceDir string
    SkipLS       bool
    LSTimeout    time.Duration
    Profile      string // NEW: defaults to "full"
    Mode         string // NEW: defaults to profile's default_mode
    MaxWorkers   int    // NEW: defaults to 2, raise to 8+ for concurrency tests
}
```

Then in `defaultTestConfig`, override `cfg.Profile` from `opts.Profile` if set, and post-`StartTestDaemon` call `switch_mode` if `opts.Mode != ""` and differs from profile's `default_mode`.

### Pattern 4: Three-Tier Concurrency Structure

**Tier 1 — Scenario stress (`t.Parallel`):**
```go
func TestConcurrency_MixedScenarios(t *testing.T) {
    td := StartTestDaemon(t, Options{WorkspaceDir: ".", MaxWorkers: 8})
    requireGopls(t)

    scenarios := []struct {
        name string
        tool string
        args map[string]any
    }{
        {"find_symbol_pool", "find_symbol", map[string]any{"name_path": "Pool"}},
        {"find_symbol_worker", "find_symbol", map[string]any{"name_path": "Worker"}},
        {"search_pattern", "search_for_pattern", map[string]any{"pattern": "func"}},
        {"list_dir", "list_dir", map[string]any{"relative_path": "."}},
        // ... enough to saturate MaxWorkers
    }
    for _, sc := range scenarios {
        sc := sc // D-07: capture before t.Parallel
        t.Run(sc.name, func(t *testing.T) {
            t.Parallel()
            callTool(t, td.Session, sc.tool, sc.args)
        })
    }
}
```

**Tier 2 — Hot-path fan-out (errgroup):**
```go
func TestConcurrency_PoolSaturation(t *testing.T) {
    td := StartTestDaemon(t, Options{WorkspaceDir: ".", MaxWorkers: 4})
    requireGopls(t)

    const N = 100 // raise to 500 in stress CI
    g, ctx := errgroup.WithContext(context.Background())
    for i := 0; i < N; i++ {
        g.Go(func() error {
            _, err := td.Session.CallTool(ctx, &mcp.CallToolParams{
                Name: "find_symbol",
                Arguments: map[string]any{"name_path": "Pool"},
            })
            return err
        })
    }
    require.NoError(t, g.Wait())
}
```

**Tier 3 — Deterministic unit (`testing/synctest`):**
[CITED: https://pkg.go.dev/testing/synctest] `synctest.Test(t, func(t *testing.T) { ... })` creates an isolated "bubble" where time is virtual and goroutines are deterministically scheduled. Use for small pool logic (e.g., TTL expiration, lease timeout propagation). Live in `internal/kernel/lspool/pool_synctest_test.go`, NOT `test/integration/`, because they target internal logic without a real LS.

```go
//go:build go1.25

func TestPool_TTLExpiration_Synctest(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        pool := NewPool(PoolConfig{BaseTTL: 1}, /* ... */)
        lease, _ := pool.AcquireLease(ctx, "sess-1", wsKey, false)
        lease.Close()
        time.Sleep(2 * time.Second) // virtual time — instant
        // Assert worker was evicted
    })
}
```

### Anti-Patterns to Avoid
- **Don't read `.serena/profiles/*.yaml` in contract tests.** [D-03] Test the compiled result via MCP, not the source of that result.
- **Don't use `time.Sleep` for concurrency coordination.** Use `sync.WaitGroup`, `errgroup`, or channel signaling. Sleep hides races the `-race` flag can't catch.
- **Don't capture loop variables in `t.Parallel()` subtests without rebinding.** [D-07] Go 1.22+ fixed this in range loops but `go vet` still catches edge cases. In Go 1.25 it's safe for `range` loops but not for manual index loops — rebind defensively.
- **Don't use a single shared daemon for destructive error tests.** A failed edit can corrupt subsequent tests. Fresh daemon per destructive test group.
- **Don't assert on error message strings.** [D-10] `strings.Contains(err.Error(), "not found")` is brittle. Assert on `errors.Is(err, ErrNotFound)` or check the MCP `ToolResult.IsError` + structured content.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Golden file framework | Custom recursive diff, fuzzy match | Plain `os.ReadFile` + `string` compare | We only compare sorted lines; anything fancier hides bugs |
| Race detection | Custom data-race detector | `go test -race` | Built-in, free, required by D-06 |
| Goroutine leak detection | Manual `runtime.NumGoroutine` polling | `t.Cleanup` + `go.uber.org/goleak` (consider) OR rely on existing `td.Stop()` | Polling is flaky; leak detection libraries know about runtime-internal goroutines |
| Virtual time for concurrency | `clockwork` / custom clock | `testing/synctest` | Native Go 1.25, zero deps, purpose-built |
| Parallel test coordination | `sync.WaitGroup` ad-hoc | `errgroup.Group` | Already in deps; better error propagation |
| Diff output on golden mismatch | Custom line-by-line printer | `t.Errorf` with raw want/got OR `cmp.Diff` (already transitive) | Plain output is readable for sorted-line format |

**Key insight:** The tests themselves should be boring. Any cleverness goes into the harness (which is tested by running tests), not into individual test cases. Table-driven + shared helpers scales; clever per-test assertions fossilize.

## Runtime State Inventory

This phase is additive testing — no renames, migrations, or state transforms. Omitted.

## Common Pitfalls

### Pitfall 1: Golden files drift without anyone noticing
**What goes wrong:** Developer runs `go test -update` before understanding the diff, accepts a regression.
**Why it happens:** `-update` is easy to type and always "fixes" the test.
**How to avoid:** (1) Make `-update` write a file that appears in `git status` — the PR review catches it. (2) Add a CI job that runs tests WITHOUT `-update` and fails if anything changed. (3) Put a comment at the top of each golden file: `# Regenerated via: go test -tags integration -update. REVIEW THIS DIFF.` (4) Consider failing tests if golden file is newer than test source by more than a day (heuristic to flag drift).
**Warning signs:** Golden file touches in PRs with no corresponding code change; `-update` appears in commit messages.

### Pitfall 2: Race detector false sense of security
**What goes wrong:** Tests pass under `-race` but production still has races.
**Why it happens:** `-race` only reports races it actually observes. If the scheduler never interleaves the right way during the test, the race is invisible.
**How to avoid:** (1) High fan-out count (100+) in stress tests. (2) `-count=5` or higher in stress CI job — each run hits different schedules. (3) `GOMAXPROCS` set to at least 2 (default on CI but worth asserting). (4) Prefer `synctest` for logic races where ordering matters — it tests all interleavings in its bubble.
**Warning signs:** "It only flakes on CI"; tests that pass on laptop but fail under load.

### Pitfall 3: `MaxWorkers=2` is too low for concurrency stress
**What goes wrong:** Current `defaultTestConfig` sets `MaxWorkers=2`. At this size, `ErrMaxWorkersReached` fires constantly and the share-until-dirty path never gets exercised.
**Why it happens:** The default was picked for harness tests that only run one tool at a time.
**How to avoid:** Raise to 8 for concurrency tests via new `Options.MaxWorkers` field. Keep 2 for other tests (catches bugs where code assumes unlimited pool).
**Warning signs:** Fan-out tests fail with "maximum number of workers reached" rather than revealing actual races.

### Pitfall 4: Mode switching interacts with profile exclusions
**What goes wrong:** A profile excludes tool X; a mode includes tool X; post-merge, X is ambiguous.
**Why it happens:** `profileSkill.ExecuteSwitchMode` (see `internal/profile/skill.go:152-162`) concatenates `prof.Tools + mode.Tools` and `prof.ExcludeTools + mode.ExcludeTools`, then calls `skill.ResolveTools` which applies precedence. Test both orders.
**How to avoid:** Golden files MUST cover profile × mode combinations, not profiles alone. Include at least one test case where profile-exclude + mode-include collide.
**Warning signs:** A tool appears in one golden file but not another for the same profile across different modes — verify that's intentional per YAML.

### Pitfall 5: MCP `tools/list` is cached by some clients
**What goes wrong:** After `switch_mode`, the MCP client still sees old tool list.
**Why it happens:** Some SDK clients cache tool lists until an explicit `listChanged` notification. The Go SDK may or may not.
**How to avoid:** Always call `ListTools` fresh after `switch_mode` in contract tests. Verify via a dedicated test: switch mode → list tools → assert changed.
**Warning signs:** Golden assertions pass in isolation but fail when a mode switch precedes them.

### Pitfall 6: Destructive error tests leave fixtures dirty
**What goes wrong:** Band-2 exhaustive edit tests on `rename_symbol` mutate the fixture; next test sees mutated state.
**Why it happens:** Shared fixtures + stateful tests.
**How to avoid:** Use `PrepareFixture(t, "go")` per test — harness already copies fixtures to `t.TempDir()`. For tests that share a daemon, use per-subtest fresh copies.
**Warning signs:** Tests pass individually but fail in sequence; "symbol not found" errors that only appear in `-run` of multiple tests.

## Code Examples

### Error matrix table-driven harness
```go
// test/integration/errors_test.go
//go:build integration

package integration_test

import "testing"

type errCase struct {
    name      string
    tool      string
    args      map[string]any
    category  string // "no_workspace", "invalid_args", "not_found", etc.
    setupFunc func(t *testing.T, td *TestDaemon)
}

func TestErrors_CategoryMatrix(t *testing.T) {
    cases := []errCase{
        {
            name:     "find_symbol_no_workspace",
            tool:     "find_symbol",
            args:     map[string]any{"name_path": "Foo"},
            category: "no_workspace",
        },
        {
            name:     "read_file_missing",
            tool:     "read_file",
            args:     map[string]any{"relative_path": "does-not-exist.go"},
            category: "not_found",
        },
        // ... one per category per layer
    }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            // Fresh daemon per test; some tests need no workspace.
            td := StartTestDaemon(t, Options{SkipLS: true})
            if tc.setupFunc != nil {
                tc.setupFunc(t, td)
            }
            result := callToolExpectError(t, td.Session, tc.tool, tc.args)
            // Assert on structured content, not message text (D-10).
            assert.True(t, result.IsError)
            // If MCP errors gain structured codes, assert here.
        })
    }
}
```

### Profile contract test
```go
// test/integration/profile_golden_test.go
//go:build integration

package integration_test

import "testing"

func TestProfile_Contract_Golden(t *testing.T) {
    profiles := []string{"claude-code", "codex", "ide-assistant", "ci-bot", "full"}
    for _, name := range profiles {
        name := name
        t.Run(name, func(t *testing.T) {
            td := StartTestDaemon(t, Options{
                SkipLS:  true,
                Profile: name,
                // Mode left empty → profile's default_mode
            })
            tools := listSessionTools(t, td.Session)
            // Compose golden name: <profile>.<default_mode>
            // (discover default mode from the profile or hardcode in test table)
            assertGoldenTools(t, name+".edit", tools)
        })
    }
}
```

### Mode contract test
```go
func TestMode_Contract_Golden(t *testing.T) {
    cases := []struct{ profile, mode string }{
        {"full", "read"},
        {"full", "edit"},
        {"full", "review"},
        {"full", "admin"},
        {"claude-code", "read"},
        {"claude-code", "edit"},
        // ... add coverage for profiles with meaningfully different modes
    }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.profile+"_"+tc.mode, func(t *testing.T) {
            td := StartTestDaemon(t, Options{
                SkipLS:  true,
                Profile: tc.profile,
                Mode:    tc.mode,
            })
            tools := listSessionTools(t, td.Session)
            assertGoldenTools(t, tc.profile+"."+tc.mode, tools)
        })
    }
}
```

[Source: harness.go patterns + CONTEXT D-01/D-02]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `t.Parallel()` with loop-variable capture bug | Go 1.22+ auto-rebinds in `for range` loops | Go 1.22 (Feb 2024) | Safer; still rebind manually for non-range loops |
| `testing/synctest` experimental (`GOEXPERIMENT=synctest`) | Stable in `testing/synctest` package | Go 1.25 (Aug 2025) | [CITED: https://pkg.go.dev/testing/synctest] Can use without build constraints; project is on 1.25.1 |
| Manual goroutine coordination with channels | `errgroup.Group` with cancellation | Go 1.7+ `x/sync` | Cleaner error propagation; already in deps |
| JSON-format golden files | Plain-text sorted lines | — | Simpler diffs, easier review; JSON only when comparing structured data |

**Deprecated/outdated:**
- `GOEXPERIMENT=synctest` build flag — not needed in Go 1.25+
- `sync.WaitGroup` for fan-out tests where errors matter — use `errgroup`

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | MCP Go SDK does not cache `tools/list` client-side (always hits server) | Pitfall 5 | Contract tests after `switch_mode` could read stale list and give false greens. **Mitigation:** Test this explicitly in one dedicated test case. |
| A2 | `ListTools` on the client session returns post-middleware-filtered list (not raw registry) | Pattern 2 | If raw, contract tests would always see all tools and ADV-01/02 would trivially pass. **Mitigation:** Verified by reading `middleware.go:36-63`, but add a sanity test: switch to `read` mode with `full` profile, assert write tools are NOT in list. |
| A3 | `MaxWorkers=8` is enough to exercise share-until-dirty without flakiness | Pattern 4 Tier 2 | Too low → `ErrMaxWorkersReached`; too high → doesn't stress pool logic. **Mitigation:** Parameterize; tune empirically. |
| A4 | `testing/synctest.Test` signature and behavior in Go 1.25.1 matches docs | Pattern 4 Tier 3 | [CITED docs] but signature may have subtle differences. **Mitigation:** Keep Tier 3 scope small; Tier 1+2 provide primary coverage. |
| A5 | Destructive tool errors (edit/6, rename_symbol, safe_delete_symbol, write_memory) return `IsError=true` rather than panic | D-08 band 2 | If any tool panics on edge case, that's an ADV-04 bug the tests must surface. This IS the purpose of the test — but the harness must not crash. **Mitigation:** Use `defer recover()` in a custom wrapper for band-2 tests OR rely on `testing`'s built-in panic recovery. |

## Open Questions

1. **How many profile × mode combinations actually need golden files?**
   - What we know: 5 profiles × 4 modes = 20 theoretical files, but CONTEXT says "some combinations may be redundant."
   - What's unclear: Does each profile support all 4 modes, or are some locked out via `allowed_mode_transitions`?
   - Recommendation: **Planner should have a small task up front to enumerate valid (profile, mode) pairs by reading each profile's YAML.** Then generate one golden file per valid pair. Likely 14-18 files, not 20.

2. **Where should `testing/synctest` tests live?**
   - What we know: Tier 3 targets internal pool logic.
   - What's unclear: `test/integration/` (uses `//go:build integration`) or `internal/kernel/lspool/` (regular unit tests)?
   - Recommendation: `internal/kernel/lspool/pool_synctest_test.go` — they're unit tests of internal logic, not end-to-end MCP tests. No build tag needed. Keeps them fast (run on every `go test ./...`).

3. **Does the MCP Go SDK emit a `listChanged` notification on `switch_mode`?**
   - What we know: `ExecuteSwitchMode` updates `sess.AllowedTools` in-place.
   - What's unclear: Does the middleware trigger a `notifications/tools/list_changed` push to the client?
   - Recommendation: **Explicit test** — list tools, switch mode, list tools again, assert diff. If no notification fires, document it as an A1-related known behavior, not a bug.

4. **Should `-update` regenerate ALL golden files or only those whose tests ran?**
   - What we know: Standard Go pattern only regenerates files touched by `t.Run`.
   - Recommendation: Go with the standard — users must run full suite + `-update` to regenerate all. Add a `make goldens-update` target: `go test -tags integration ./test/integration/... -run Golden -update`.

5. **Error matrix: assert on what, exactly?**
   - What we know: D-10 says "error type/code, not message text."
   - What's unclear: Current kernel tools return `fmt.Errorf` with wrapped strings (see `internal/kernel/edit/replace.go:21-61`). There are no typed errors like `ErrNotFound` defined in the kernel.
   - Recommendation: **Two-track approach:** (a) Assert on `result.IsError == true` + category via structured content field if present; (b) file a follow-up issue to introduce typed errors (`var ErrFileNotFound = errors.New(...)`) in kernel tools. For Phase 8 tests, accept message-substring as a temporary bridge but tag each assertion with a `// TODO: typed error` comment. **Alternatively**, introduce typed errors as Wave 0 of the phase — small refactor, unblocks brittle-free assertions. Let the planner decide based on scope budget.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All tests | ✓ | 1.25.1 | — |
| `gopls` | Concurrency tests against real LS | ? | depends on dev machine / CI | `requireGopls(t)` already skips — existing pattern |
| `go test -race` | D-06 | ✓ | stdlib | — |
| `testing/synctest` | Tier 3 concurrency | ✓ | stdlib (Go 1.25) | Degrade to Tier 1+2 only |
| MCP Go SDK | All tests | ✓ | already in deps | — |

**Missing dependencies with no fallback:** None. Everything required is either stdlib, already in `go.mod`, or guarded by `requireLS`/`requireGopls` skips.

**Missing dependencies with fallback:** `gopls` — if not installed, tests that need a running LS skip via existing helpers. Concurrency tests that don't need symbol results (e.g., `list_dir` fan-out) can run without it.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testing/synctest` (Go 1.25.1 stdlib) |
| Config file | none — `//go:build integration` tag gates the suite |
| Quick run command | `go test -tags integration ./test/integration/... -run Golden -race` |
| Full suite command | `go test -tags integration -race -count=1 ./test/integration/...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ADV-01 | 5 profiles expose exact tool subsets | integration (golden) | `go test -tags integration ./test/integration/ -run TestProfile_Contract_Golden -race` | ❌ Wave 0 |
| ADV-02 | 4 modes filter tools per mode spec | integration (golden) | `go test -tags integration ./test/integration/ -run TestMode_Contract_Golden -race` | ❌ Wave 0 |
| ADV-03 | Concurrent tool calls → no race/deadlock | integration (stress) + unit (synctest) | `go test -tags integration ./test/integration/ -run TestConcurrency -race -count=3` + `go test ./internal/kernel/lspool/ -run Synctest -race` | ❌ Wave 0 |
| ADV-04 | Structured errors for bad-state calls | integration (table-driven) | `go test -tags integration ./test/integration/ -run TestErrors -race` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test -tags integration ./test/integration/... -race -run <NewTest>` (only new tests, fast feedback)
- **Per wave merge:** `go test -tags integration ./test/integration/... -race -count=1` (full integration suite) + `go test ./... -race` (all unit tests)
- **Phase gate:** Full suite green + stress CI job green (`-count=5 -timeout=10m`) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `test/integration/golden.go` — shared golden-file helper with `-update` flag
- [ ] `test/integration/harness.go` — extend `Options` with `Profile`, `Mode`, `MaxWorkers`
- [ ] `test/integration/helpers.go` — add `listSessionTools` helper
- [ ] `testdata/profiles/` directory — create, populate with initial golden files via `-update` run
- [ ] `test/integration/profile_golden_test.go` — ADV-01 tests
- [ ] `test/integration/mode_golden_test.go` — ADV-02 tests
- [ ] `test/integration/concurrency_test.go` — ADV-03 Tier 1+2
- [ ] `internal/kernel/lspool/pool_synctest_test.go` — ADV-03 Tier 3
- [ ] `test/integration/errors_test.go` — ADV-04 three bands
- [ ] Optional: `Makefile` target `test-stress` for elevated `-count` + stress CI job
- [ ] Optional: typed errors in kernel tools (`ErrFileNotFound`, etc.) to satisfy D-10 cleanly — see Open Question 5

## Security Domain

> ASVS categories are largely orthogonal to test code itself; this section covers security-relevant aspects of what the tests verify.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Tests run in-process; no auth surface |
| V3 Session Management | partial | Test MCP session has `AllowedTools` — verify filter is enforced, not just advertised |
| V4 Access Control | **yes** | Profile/mode filtering IS access control. ADV-01/02 tests directly verify that restricted sessions cannot see tools they shouldn't — this is the core security test for Serena's tool RBAC |
| V5 Input Validation | **yes** | ADV-04 error tests verify that malformed/invalid inputs are rejected with structured errors, not panics or hangs |
| V6 Cryptography | no | Not in scope for this phase |

### Known Threat Patterns for Go integration tests

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Tool visible in `tools/list` but not callable (or vice versa) | Elevation of Privilege | Golden test BOTH `tools/list` AND try to invoke excluded tool, expecting error. Otherwise attacker sees "no tool" in list but still calls it by name. |
| Mode downgrade leaks privileged tool list | Information Disclosure | After `switch_mode read`, re-list tools — asserts middleware re-filters. Covered by Pitfall 5 test. |
| Error messages leak internal paths / stack traces | Information Disclosure | ADV-04 assertions should check that error output does NOT contain absolute host paths beyond fixture dir. Consider adding a `requireNoLeakage` helper. |
| Concurrent tool calls race on session state → wrong `AllowedTools` applied | Tampering / EoP | Covered by ADV-03 Tier 1 scenario stress: mix of tool calls + mode switches in parallel, assert final state matches expectations. |

**Recommendation for planner:** Add an explicit "call excluded tool directly" test to the ADV-01 suite. Contract tests that only check `tools/list` output can miss the case where middleware filters listing but not invocation. Verify the excluded tool returns a structured error (probably "method not found" or "tool not allowed"), not a silent pass.

## Sources

### Primary (HIGH confidence)
- `internal/mcp/middleware.go:24-80` — ProfileFilterMiddleware implementation [VERIFIED via Read]
- `internal/profile/skill.go:117-177` — `ExecuteSwitchMode` merge logic [VERIFIED via Read]
- `internal/profile/profile.go` — ProfileStore interface [VERIFIED via Read]
- `internal/profile/loader.go` — YAML load flow [VERIFIED via Read]
- `internal/profile/profiles/claude-code.yaml`, `full.yaml`, `modes/read.yaml` — sample structure [VERIFIED via Read]
- `internal/kernel/lspool/pool.go:16-55, 103-146` — Pool lifecycle, AcquireLease/ReleaseLease, ErrMaxWorkersReached [VERIFIED via Read]
- `test/integration/harness.go` — current harness capabilities [VERIFIED via Read]
- `test/integration/helpers.go` — callTool, callToolExpectError, textContent [VERIFIED via Read]
- `test/integration/profile_test.go` — existing profile integration pattern [VERIFIED via Read]
- `go.mod` — `go 1.25.1` confirmed [VERIFIED via Read]
- https://pkg.go.dev/testing/synctest — `synctest.Test` signature, bubble semantics, virtual time [CITED]
- https://pkg.go.dev/flag — `-update` flag pattern [CITED]

### Secondary (MEDIUM confidence)
- Go 1.22 release notes — loop variable rebinding behavior [CITED from general Go release knowledge]
- Google testing guidance (per CONTEXT canonical_refs) — risk/cost tradeoff framing
- OWASP integrity testing guidance (per CONTEXT canonical_refs) — negative tests for mutating operations

### Tertiary (LOW confidence)
- Assumed MCP Go SDK does not client-cache `tools/list` — see Assumption A1, needs explicit verification test
- Assumed `listChanged` notification behavior — see Open Question 3

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all deps verified against `go.mod`, stdlib versions confirmed
- Architecture: HIGH — patterns extracted from existing harness + verified against middleware/profile code
- Pitfalls: HIGH — most derive from directly-read code (`defaultTestConfig` MaxWorkers=2, `ExecuteSwitchMode` merge order)
- Error testing detail: MEDIUM — kernel tools don't yet expose typed errors; recommendation may require a small refactor (see Open Question 5)
- MCP SDK runtime behavior (caching, listChanged): MEDIUM — verified via code read but not via live test

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (30 days — stable tech stack, but MCP Go SDK is evolving)
