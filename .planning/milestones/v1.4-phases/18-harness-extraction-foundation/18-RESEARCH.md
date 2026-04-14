# Phase 18: Harness Extraction & Foundation - Research

**Researched:** 2026-04-11
**Domain:** Go test package extraction, build tag taxonomy, test infrastructure design
**Confidence:** HIGH

## Summary

This phase extracts shared test infrastructure from `test/integration/` into an importable `test/harness/` package and establishes a three-tier build tag taxonomy (`integration`, `llm`, `llmjudge`). The existing code is well-structured and the extraction is mechanical: copy `harness.go`, `helpers.go`, `golden.go` into a new package, rename from `integration_test` (external test package, not importable) to `harness` (regular package, importable), and evolve the API with richer concepts (`Runner`, `Transcript`, `GoldenStore`).

The critical technical insight is that Go build constraints use boolean expressions (`//go:build integration || llm || llmjudge`) which achieve the additive layer semantics without any custom tooling. The new `test/oracle/` directory structure creates separate packages per oracle layer, each importing `test/harness/` but never cross-importing.

**Primary recommendation:** Copy-and-evolve from existing integration helpers. The new `test/harness/` package must be a regular (non-`_test`) package so it is importable. Every file in `test/harness/` and `test/oracle/*/` needs the correct `//go:build` expression. Verify with `go test ./...` (no oracle tests run) and `go test -tags integration ./test/oracle/...` (deterministic oracle tests run).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Copy and evolve -- duplicate `harness.go`, `helpers.go`, `golden.go` from `test/integration/` into `test/harness/` as a new importable package. Do NOT import back from `test/integration/`.
- **D-02:** Rename aggressively in the new package -- use clear concepts like `Runner`, `Transcript`, `GoldenStore` instead of preserving legacy naming.
- **D-03:** Add tests for the harness package itself -- it becomes a first-class package with its own contracts.
- **D-04:** Revisit convergence with `test/integration/` only after 2-3 iterations. Not before.
- **D-05:** Three tags: `integration`, `llm`, `llmjudge`. Additive layers via explicit boolean `//go:build` expressions.
- **D-06:** Tag expressions: deterministic = `integration || llm || llmjudge`; LLM behavioral = `llm || llmjudge`; judge scoring = `llmjudge`.
- **D-07:** `-tags=integration` runs deterministic only; `-tags=llm` runs deterministic + LLM; `-tags=llmjudge` runs all three.
- **D-08:** Put the hierarchy in the `//go:build` lines explicitly, not in directory/package naming.
- **D-09:** Directory structure: `test/harness/`, `test/oracle/protocol/`, `test/oracle/contract/`, `test/oracle/scenario/`, `test/oracle/llm/`, `test/oracle/judge/`.
- **D-10:** Each oracle package imports `test/harness/`. No cross-imports between oracle packages.
- **D-11:** Zero changes to `test/integration/` -- not a single line modified.
- **D-12:** `test/integration/` keeps `//go:build integration` unchanged. New oracle packages use boolean expression scheme.
- **D-13:** Duplication between `test/integration/` and `test/harness/` is a feature.

### Claude's Discretion
- Internal package structure of `test/harness/` (file split, type names beyond suggested concepts)
- Whether to use `testify` in the new harness or stay with stdlib `testing`
- Golden file directory layout under `testdata/`

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FOUND-01 | Test harness extracted to importable `test/harness/` package with exported `StartTestDaemon`, `PrepareFixture`, `callTool`, and golden helpers | Architecture Patterns section: package design, export strategy, import path. Code Examples: exact API signatures. |
| FOUND-02 | Build tag taxonomy established (`integration`, `llm`, `llmjudge`) with documented naming conventions for oracle packages | Architecture Patterns section: build tag expressions per layer. Common Pitfalls: tag expression mistakes. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Always run `go vet` and `go test` before completing any Go task
- Build: `go build ./cmd/serena`
- Test: `go test ./...`
- Format: `gofmt -w .`
- Single Go binary, Go 1.25.1 [VERIFIED: `go version` output]

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` (stdlib) | Go 1.25.1 | Test framework | Built-in, zero deps, `testing.TB` interface |
| `github.com/stretchr/testify` | v1.11.1 | Assertions (`require`, `assert`) | Already in go.mod, used by existing integration tests |
| `github.com/modelcontextprotocol/go-sdk/mcp` | (in go.mod) | MCP client/server for test sessions | Project's MCP SDK, required for `CallTool`, `ListTools` |

[VERIFIED: go.mod] -- testify v1.11.1 already a dependency. No new dependencies needed for this phase.

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/postfix/serena/internal/daemon` | local | Daemon creation for test harness | `Runner.Start()` creates daemon in-process |
| `github.com/postfix/serena/internal/config` | local | Test config construction | `defaultTestConfig` builds `SerenaConfig` |
| `github.com/postfix/serena/internal/skill` | local | Skill registration + init | Blank imports + `skill.InitAll(deps)` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| testify/require | stdlib testing only | testify already used everywhere; switching adds friction for no gain |
| Custom golden framework | go-golden or cupaloy | Existing golden pattern is simple and works; external dep not justified |

### Discretion Recommendation: Use testify

Rationale: testify v1.11.1 is already in `go.mod`, used by all existing integration helpers (`require.NoError`, `require.False`, `require.True`). The new harness should use it for consistency. Staying stdlib-only would create a jarring API difference between `test/harness/` and `test/integration/`. [VERIFIED: existing helpers.go uses `github.com/stretchr/testify/require`]

## Architecture Patterns

### Recommended Project Structure
```
test/
  harness/                    # package harness -- importable shared infrastructure
    runner.go                 # Runner (was TestDaemon), StartTestDaemon, config
    fixture.go                # PrepareFixture, fixture helpers
    tools.go                  # CallTool, TextContent, CallToolExpectError, ListSessionTools
    golden.go                 # GoldenStore, AssertGolden, -update flag
    runner_test.go            # Tests for harness itself (D-03)
    doc.go                    # Package doc + build tag
  oracle/
    protocol/                 # package protocol -- MCP handshake tests (Phase 19)
      doc.go                  # //go:build integration || llm || llmjudge
    contract/                 # package contract -- per-tool goldens (Phase 19)
      doc.go
    scenario/                 # package scenario -- multi-step workflows (Phase 20)
      doc.go
    llm/                      # package llm -- LLM behavioral (Phase 21)
      doc.go                  # //go:build llm || llmjudge
    judge/                    # package judge -- LLM judge scoring (Phase 21)
      doc.go                  # //go:build llmjudge
  integration/                # FROZEN -- zero changes (D-11)
    ...existing files...
  bench/                      # Existing benchmark suite
    ...existing files...
```

### Pattern 1: Package Importability (external test pkg vs regular pkg)

**What:** `test/integration/` uses `package integration_test` (external test package) which is NOT importable by other packages. The new `test/harness/` must use `package harness` (regular package) to be importable.

**Why this matters:** This is the core technical reason the extraction exists. External test packages (ending in `_test`) are only compiled during `go test` of that specific package. A regular package can be imported by any test file.

**Build tag on non-test files:** Files in `test/harness/` that are NOT `_test.go` files still need `//go:build integration || llm || llmjudge` so they are excluded from `go test ./...` (no tags). Without the build tag, `go build ./...` would try to compile them and fail because their dependencies (daemon, skill, etc.) might not be needed in the main binary path.

[VERIFIED: `go test ./test/integration/...` with no tags returns "matched no packages" -- confirming build tag gating works]

### Pattern 2: Build Tag Boolean Expressions (Go 1.17+)

**What:** Go build constraints support boolean expressions since Go 1.17. The `//go:build` directive uses `||` (OR), `&&` (AND), `!` (NOT), and parentheses.

**Tag expression hierarchy per D-06/D-07:**

```go
// Deterministic oracle tests -- run with ANY of the three tags
//go:build integration || llm || llmjudge

// LLM behavioral tests -- run with llm OR llmjudge
//go:build llm || llmjudge

// LLM judge scoring -- run ONLY with llmjudge
//go:build llmjudge
```

**Verification matrix:**

| Command | integration files | llm files | llmjudge files |
|---------|-------------------|-----------|----------------|
| `go test ./...` (no tags) | SKIP | SKIP | SKIP |
| `go test -tags integration ./...` | RUN | SKIP | SKIP |
| `go test -tags llm ./...` | RUN | RUN | SKIP |
| `go test -tags llmjudge ./...` | RUN | RUN | RUN |

[VERIFIED: Go 1.25.1 supports `//go:build` boolean expressions -- this is stable since Go 1.17]

### Pattern 3: Blank Imports for Skill Registration

**What:** The harness must replicate the Caddy-style `init()` registration pattern from `test/integration/harness.go`. Without these blank imports, `skill.InitAll` has nothing to initialize and tool calls fail silently.

```go
import (
    _ "github.com/postfix/serena/internal/kernel/diag"
    _ "github.com/postfix/serena/internal/kernel/edit"
    _ "github.com/postfix/serena/internal/kernel/fileops"
    _ "github.com/postfix/serena/internal/kernel/symbols"
    _ "github.com/postfix/serena/internal/profile"
    _ "github.com/postfix/serena/internal/skill/memory"
    _ "github.com/postfix/serena/internal/skill/workflow"
)
```

[VERIFIED: existing harness.go lines 25-31]

### Pattern 4: Naming Evolution (D-02)

**Mapping from legacy to new concepts:**

| Legacy (integration_test) | New (harness) | Rationale |
|---------------------------|---------------|-----------|
| `TestDaemon` | `Runner` | It runs test scenarios, not just a daemon |
| `StartTestDaemon` | `NewRunner` or `StartRunner` | Follows Go constructor conventions |
| `Options` | `RunnerOptions` or `Config` | Scoped to Runner |
| `callTool` | `CallTool` (exported) | Must be exported for cross-package use |
| `textContent` | `TextContent` (exported) | Same |
| `callToolExpectError` | `CallToolExpectError` (exported) | Same |
| `listSessionTools` | `ListSessionTools` (exported) | Same |
| `assertGoldenTools` | `GoldenStore.AssertTools` or `AssertGolden` | Generalized for per-tool goldens |
| `projectRoot` | `ProjectRoot` (exported) | Needed by oracle packages for testdata paths |
| `PrepareFixture` | `PrepareFixture` (keep) | Already well-named |

### Anti-Patterns to Avoid
- **Importing back from test/integration/:** Creates coupling. Copy, don't share. (D-01)
- **Using package name to imply tag inclusion:** Put `//go:build` expression in every file explicitly. (D-08)
- **Modifying test/integration/:** Frozen baseline. Zero changes. (D-11)
- **Cross-imports between oracle packages:** Each oracle package imports only `test/harness/`. (D-10)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Test assertions | Custom assertion helpers | `testify/require` and `testify/assert` | Already in project, well-tested, good error messages |
| Golden file management | Complex snapshot framework | Simple `os.ReadFile`/`os.WriteFile` with `-update` flag | Existing pattern works, matches project conventions |
| MCP client setup | Raw HTTP/JSON-RPC calls | `mcp.NewClient` + `mcp.NewInMemoryTransports` | SDK handles protocol correctly |
| Fixture isolation | Manual tmp dir management | `tb.TempDir()` + `tb.Cleanup()` | Go testing stdlib handles cleanup automatically |

## Common Pitfalls

### Pitfall 1: Forgetting Build Tags on Non-Test Files
**What goes wrong:** Files in `test/harness/` that are not `_test.go` (e.g., `runner.go`, `tools.go`) compile unconditionally. If they import daemon/kernel packages, `go build ./...` may pull them in unexpectedly or `go vet ./...` may check them when not intended.
**Why it happens:** `_test.go` files are automatically excluded from non-test builds. Regular `.go` files are not.
**How to avoid:** Every `.go` file in `test/harness/` and `test/oracle/*/` MUST have `//go:build integration || llm || llmjudge` (or narrower) as the first line.
**Warning signs:** `go build ./...` fails, or `go vet ./...` reports issues in test infrastructure.

### Pitfall 2: Package Name Collision with Build Tags
**What goes wrong:** If `test/harness/` has files with different build tags but the same package name, files with non-matching tags are excluded, and the remaining files may have unsatisfied dependencies.
**Why it happens:** Each file can have its own build constraint. If file A defines a type and has tag X, but file B uses that type and has tag Y, compiling with only tag Y fails.
**How to avoid:** All files in `test/harness/` should use the broadest tag expression (`integration || llm || llmjudge`) so the entire package is available whenever any oracle layer runs. Files that need narrower scope belong in oracle sub-packages, not in harness.
**Warning signs:** Compilation errors only under certain tag combinations.

### Pitfall 3: Unexported Functions After Copy
**What goes wrong:** Functions copied from `integration_test` package were lowercase (unexported) because they were package-private. In `package harness`, they must be exported (uppercase) to be importable.
**Why it happens:** In external test packages (`_test`), unexported is fine because nothing imports them. In a regular package, cross-package consumers need exported symbols.
**How to avoid:** Systematically uppercase every function/type that oracle packages will need. Use the naming evolution table above.
**Warning signs:** Compilation errors in oracle packages importing harness.

### Pitfall 4: Missing `projectRoot()` Portability
**What goes wrong:** `projectRoot()` uses `runtime.Caller(0)` to find the repo root by walking up from the source file location. After copying to `test/harness/runner.go`, the walk-up count changes (was 3 levels from `test/integration/harness.go`, now 2 levels from `test/harness/runner.go`).
**Why it happens:** `runtime.Caller` returns the file path of the calling source file. Different directory depth = different number of `filepath.Dir()` calls needed.
**How to avoid:** Adjust `projectRoot()` to walk up 2 levels from `test/harness/*.go` instead of 3.
**Warning signs:** Fixture paths resolve to wrong directory, `PrepareFixture` fails with "no such file."

### Pitfall 5: Oracle Package doc.go Files Missing
**What goes wrong:** `go test ./test/oracle/protocol/...` fails because the directory exists but has no Go files (or no files matching the build tag).
**Why it happens:** Empty directories are invisible to Go toolchain. At minimum, a `doc.go` with the package declaration and build tag is needed.
**How to avoid:** Create `doc.go` in each oracle directory during this phase, even if tests come in later phases.
**Warning signs:** "matched no packages" when running with correct tags.

## Code Examples

### Runner (evolved from TestDaemon)

```go
//go:build integration || llm || llmjudge

package harness

import (
    "context"
    "io"
    "log/slog"
    "os"
    "path/filepath"
    "runtime"
    "testing"
    "time"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/postfix/serena/internal/config"
    "github.com/postfix/serena/internal/daemon"
    "github.com/postfix/serena/internal/skill"

    // Blank imports trigger skill registration via init().
    _ "github.com/postfix/serena/internal/kernel/diag"
    _ "github.com/postfix/serena/internal/kernel/edit"
    _ "github.com/postfix/serena/internal/kernel/fileops"
    _ "github.com/postfix/serena/internal/kernel/symbols"
    _ "github.com/postfix/serena/internal/profile"
    _ "github.com/postfix/serena/internal/skill/memory"
    _ "github.com/postfix/serena/internal/skill/workflow"
)

// RunnerOptions configures a test Runner instance.
type RunnerOptions struct {
    WorkspaceDir string
    SkipLS       bool
    LSTimeout    time.Duration
    Profile      string
    Mode         string
    MaxWorkers   int
}

// Runner wraps a daemon instance with an MCP client session for tests.
type Runner struct {
    Daemon  *daemon.Daemon
    Session *mcp.ClientSession
    cancel  context.CancelFunc
    tb      testing.TB
}

// Stop cancels the Runner context and shuts down the daemon.
func (r *Runner) Stop() {
    r.cancel()
}
```

[Based on: test/integration/harness.go lines 35-63, with D-02 naming evolution applied]

### Build Tag Examples per Layer

```go
// test/harness/doc.go -- broadest scope, available to all oracle layers
//go:build integration || llm || llmjudge

// Package harness provides shared test infrastructure for oracle test packages.
package harness
```

```go
// test/oracle/protocol/doc.go -- deterministic oracle
//go:build integration || llm || llmjudge

// Package protocol tests MCP handshake, capabilities, and session lifecycle.
package protocol
```

```go
// test/oracle/llm/doc.go -- LLM behavioral layer
//go:build llm || llmjudge

// Package llm tests LLM tool selection accuracy and output interpretation.
package llm
```

```go
// test/oracle/judge/doc.go -- judge scoring layer
//go:build llmjudge

// Package judge implements LLM judge scoring via structured rubrics.
package judge
```

### Tool Helper (exported for cross-package use)

```go
//go:build integration || llm || llmjudge

package harness

import (
    "context"
    "testing"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/stretchr/testify/require"
)

// CallTool invokes an MCP tool and asserts success.
func CallTool(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
    tb.Helper()
    result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
        Name:      name,
        Arguments: args,
    })
    require.NoError(tb, err, "tool %s call error", name)
    require.False(tb, result.IsError, "tool %s failed: %s", name, TextContent(result))
    return result
}

// TextContent extracts text from the first TextContent element in a CallToolResult.
func TextContent(r *mcp.CallToolResult) string {
    for _, c := range r.Content {
        if tc, ok := c.(*mcp.TextContent); ok {
            return tc.Text
        }
    }
    return ""
}
```

[Based on: test/integration/helpers.go, with exports and D-02 naming]

### ProjectRoot Adjustment

```go
// ProjectRoot returns the repository root directory.
// From test/harness/*.go, the repo root is 2 levels up.
func ProjectRoot() string {
    _, file, _, ok := runtime.Caller(0)
    if !ok {
        panic("cannot determine project root via runtime.Caller")
    }
    // file is .../test/harness/fixture.go -> go up 2 levels
    return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
```

**Wait -- careful.** `test/harness/fixture.go` path is `<root>/test/harness/fixture.go`. `filepath.Dir` once = `<root>/test/harness`, twice = `<root>/test`, three times = `<root>`. So it is still 3 `filepath.Dir` calls, same as the original. The original was also at depth 3: `<root>/test/integration/harness.go`.

Correction: Both `test/integration/` and `test/harness/` are at the same depth (2 directories under root). The `projectRoot()` function needs the same 3 `filepath.Dir` calls. No adjustment needed.

[VERIFIED: counting directory levels -- test/integration/harness.go and test/harness/runner.go are both 2 dirs deep under repo root]

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go testing needs no config) |
| Quick run command | `go test -tags integration ./test/harness/... -v -count=1` |
| Full suite command | `go test -tags integration ./test/harness/... ./test/oracle/... -v -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | New test in `test/oracle/protocol/` imports harness, calls `StartTestDaemon`, `PrepareFixture`, `CallTool`, golden helpers without compilation errors | compilation + smoke | `go test -tags integration ./test/oracle/protocol/... -v -count=1` | Wave 0 |
| FOUND-01 | Harness package itself has tests (D-03) | unit | `go test -tags integration ./test/harness/... -v -count=1` | Wave 0 |
| FOUND-02 | `go test ./...` (no tags) skips all integration/llm/llmjudge tests | smoke | `go test ./test/harness/... ./test/oracle/... 2>&1` (should report "matched no packages") | Wave 0 |
| FOUND-02 | `-tags integration` includes oracle tests but not LLM tests | smoke | `go test -tags integration ./test/oracle/... -v -count=1 -run .` | Wave 0 |
| FOUND-02 | Existing v1.1 tests in `test/integration/` pass unchanged | regression | `go test -tags integration ./test/integration/... -v -count=1` | Existing |

### Sampling Rate
- **Per task commit:** `go vet ./... && go test -tags integration ./test/harness/... -v -count=1`
- **Per wave merge:** `go test -tags integration ./test/harness/... ./test/oracle/... ./test/integration/... -v -count=1`
- **Phase gate:** Full suite green including integration regression

### Wave 0 Gaps
- [ ] `test/harness/runner_test.go` -- harness self-tests (D-03)
- [ ] `test/oracle/protocol/smoke_test.go` -- import validation test for FOUND-01
- [ ] Oracle directory `doc.go` files -- package declarations with build tags

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `runtime.Caller(0)` path depth is 3 `filepath.Dir` calls for `test/harness/` | Code Examples | PrepareFixture would resolve wrong testdata path -- easy to catch in test |
| A2 | All oracle doc.go files should use package names matching directory (`protocol`, `contract`, etc.) | Architecture Patterns | Go compilation error if mismatch -- caught immediately |

## Open Questions

1. **Golden file location for oracle packages**
   - What we know: Existing goldens are in `testdata/profiles/`. New oracle packages will generate per-tool goldens (CONT-01, Phase 19).
   - What's unclear: Should `test/harness/` define a shared golden root, or should each oracle package manage its own `testdata/` subdirectory?
   - Recommendation: Claude's discretion per CONTEXT.md. Recommend `testdata/golden/{oracle-layer}/` under repo root for centralized golden management. Harness provides `GoldenStore` that takes a root path.

2. **Transcript capture design**
   - What we know: CONTEXT.md mentions `Transcript` as a future concept for the harness.
   - What's unclear: Exact shape of transcript capture (MCP request/response pairs, timing, metadata).
   - Recommendation: Stub the `Transcript` type in this phase with a simple slice of request/response pairs. Full implementation comes in Phase 19-20 when oracle packages need it.

## Sources

### Primary (HIGH confidence)
- `test/integration/harness.go` -- Existing StartTestDaemon, Options, TestDaemon, PrepareFixture, WaitForLS (source material for extraction)
- `test/integration/helpers.go` -- Existing callTool, textContent, callToolExpectError, listSessionTools
- `test/integration/golden.go` -- Existing assertGoldenTools with -update flag
- `go.mod` -- Module path `github.com/postfix/serena`, Go 1.25.1, testify v1.11.1
- `go test ./test/integration/...` (no tags) -- Verified: "matched no packages" confirms build tag gating

### Secondary (MEDIUM confidence)
- Go specification on build constraints -- boolean expression syntax stable since Go 1.17 [CITED: https://go.dev/ref/spec#Build_constraints]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in go.mod, no new deps needed
- Architecture: HIGH -- extraction is mechanical, patterns verified in existing code
- Pitfalls: HIGH -- pitfalls identified from direct code inspection of source material

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable Go patterns, no fast-moving dependencies)
