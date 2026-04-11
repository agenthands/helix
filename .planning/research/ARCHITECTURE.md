# Architecture Patterns

**Domain:** Multi-oracle integration test harness for Go MCP platform
**Researched:** 2026-04-11

## Recommended Architecture

### Design Principle: Extend, Don't Replace

The existing `test/integration/` package is well-structured with battle-tested patterns (harness, golden files, helpers, build tags). The multi-oracle architecture layers on top of it rather than replacing it. The existing `StartTestDaemon`, `PrepareFixture`, `assertGoldenTools`, and `callTool` helpers become the shared foundation that all five oracle layers consume.

### Component Layout

```
test/
  integration/                  # EXISTING - stays as-is, becomes "foundation layer"
    harness.go                  # StartTestDaemon, PrepareFixture, WaitForLS
    golden.go                   # assertGoldenTools, -update flag
    helpers.go                  # callTool, textContent, listSessionTools
    a_doc.go                    # Package doc
    *_test.go                   # EXISTING tests (keep, don't migrate)

  harness/                      # NEW - importable test infrastructure (extracted)
    harness.go                  # StartTestDaemon, Options, TestDaemon (exported)
    fixture.go                  # PrepareFixture, projectRoot, FixtureRegistry
    golden.go                   # assertGolden*, -update flag, hierarchical goldens
    helpers.go                  # callTool, textContent, listSessionTools
    doc.go                      # Package doc

  oracle/                       # NEW - multi-oracle test harness
    doc.go                      # Package doc, build tag explanation
    shared.go                   # Cross-oracle helpers, YAML loader

    protocol/                   # Oracle Layer 1: Protocol correctness
      protocol_test.go          # //go:build integration

    contract/                   # Oracle Layer 2: Per-tool contracts
      contract_test.go          # //go:build integration
      schema.go                 # Schema validation + error shape assertions

    scenario/                   # Oracle Layer 3: Repository scenarios
      scenario_test.go          # //go:build integration && scenario
      loader.go                 # YAML scenario file loader
      runner.go                 # Data-driven scenario executor

    behavioral/                 # Oracle Layer 4: LLM behavioral
      behavioral_test.go        # //go:build llmtest
      client.go                 # Claude API client wrapper
      rubric.go                 # Scoring rubrics

    judge/                      # Oracle Layer 5: LLM judge
      judge_test.go             # //go:build llmjudge
      scorer.go                 # Structured judge prompts + parsing

testdata/
  fixtures/                     # EXISTING fixtures stay
    go/                         # EXISTING
    python/                     # EXISTING
    typescript/                 # EXISTING
    java/                       # EXISTING
    rust/                       # EXISTING
    polyglot/                   # NEW - multi-language monorepo fixture
    unsupported/                # NEW - language with no LS available
    degraded/                   # NEW - valid project with broken LS config
    collisions/                 # NEW - name collision scenarios

  profiles/                     # EXISTING golden files stay
    *.tools.golden              # EXISTING 19 goldens

  oracle/                       # NEW - oracle-specific test data
    protocol/                   # Protocol test expectations
      init_sequence.golden
      tool_listing.golden
      reconnect.golden

    contracts/                  # Per-tool contract goldens
      go/                       # Organized by fixture language
        search_symbols.golden
        go_to_definition.golden
        get_symbol_overview.golden
        ...
      python/
        ...
      typescript/
        ...
      error_shapes/             # Error response shape goldens
        no_workspace.golden
        symbol_not_found.golden
        invalid_args.golden

    scenarios/                  # YAML scenario definitions
      go_basic.yaml
      python_basic.yaml
      typescript_basic.yaml
      polyglot_cross_lang.yaml
      unsupported_graceful.yaml
      degraded_fallback.yaml
      collision_disambiguation.yaml

    behavioral/                 # LLM behavioral test data
      tool_selection/
      disambiguation/
      output_interpretation/

    judge/                      # Judge rubrics
      rubrics/
        tool_selection.yaml
        output_quality.yaml
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `test/harness/` (NEW, extracted) | Importable daemon lifecycle, fixture prep, golden helpers | All oracle layers + existing `test/integration/` |
| `test/integration/` (EXISTING) | Existing regression tests, thin wrappers over harness | `test/harness/` |
| `test/oracle/protocol/` | MCP init, tool listing, session isolation, reconnect | `test/harness/` |
| `test/oracle/contract/` | Per-tool input/output contracts, schema validation, error shapes | `test/harness/` + `testdata/oracle/contracts/` goldens |
| `test/oracle/scenario/` | Data-driven multi-step scenarios from YAML | `test/harness/` + `testdata/oracle/scenarios/` YAML |
| `test/oracle/behavioral/` | LLM tool selection, disambiguation, output interpretation | Claude API + `testdata/oracle/behavioral/` |
| `test/oracle/judge/` | LLM-as-judge scoring with structured rubrics | Claude API + `testdata/oracle/judge/rubrics/` |
| `testdata/fixtures/` | Repository fixtures (code to test against) | `PrepareFixture()` in harness |
| `testdata/oracle/` | Golden files, scenarios, rubrics | Oracle layers read these |

### Data Flow

```
                    YAML Scenarios
                         |
                         v
  +-----------+    +-----------+    +----------------+
  | Fixture   | -> | Harness   | -> | Oracle Layer   |
  | (testdata)|    | (test/    |    | (protocol/     |
  |           |    | harness/) |    |  contract/     |
  |           |    |           |    |  scenario)     |
  +-----------+    +-----------+    +----------------+
                         |                  |
                         v                  v
                   +----------+     +---------------+
                   | MCP      |     | Golden Files  |
                   | Session  |     | (testdata/    |
                   | (tool    |     |  oracle/)     |
                   | calls)   |     +---------------+
                   +----------+
                         |
            +------------+------------+
            |                         |
            v                         v
  +------------------+    +-------------------+
  | LLM Behavioral   |    | LLM Judge         |
  | (Claude API,     |    | (structured       |
  |  //go:build      |    |  rubric scoring,  |
  |  llmtest)        |    |  //go:build       |
  +------------------+    |  llmjudge)        |
                          +-------------------+
```

## Critical Prerequisite: Harness Extraction (Pattern 0)

The existing `test/integration/` uses `package integration_test` (external test package). External test packages CANNOT be imported by other packages. The helpers (`StartTestDaemon`, `PrepareFixture`, `callTool`, etc.) must be importable by oracle layers.

**Solution:** Create `test/harness/` as `package harness` (proper importable package). Move shared infrastructure there. Existing `test/integration/` becomes a thin consumer that imports `test/harness/`.

**Migration path:**
1. Create `test/harness/` with the shared types and functions.
2. Update `test/integration/*_test.go` to import from `test/harness/`.
3. All existing tests must pass unchanged.
4. Oracle layers import `test/harness/` directly.

**Build tag consideration:** The existing harness files all have `//go:build integration`. The extracted `test/harness/` should also use `//go:build integration` because oracle tests should never compile without that tag active. LLM layers (build tags `llmtest`/`llmjudge`) should use `integration || llmtest || llmjudge` to also compile the harness, OR the harness should have NO build tag and rely on consumers to be tag-gated. The latter is simpler and matches Go convention (importable packages are unconditionally compilable; consumers control when they compile).

**Recommendation:** `test/harness/` has NO build tags. It is a library. Consumers (`test/integration/`, `test/oracle/*/`) each have their own build tags.

## Patterns to Follow

### Pattern 1: YAML-Driven Scenario Files

**What:** Scenario definitions in YAML, loaded by a Go test runner that iterates them as subtests.

**When:** Oracle Layer 3 (scenarios) -- multi-step tool call sequences with assertions.

**Why:** Adding a new scenario requires only a YAML file, not Go code. This scales to hundreds of scenarios and enables non-Go-developers to contribute test cases. Go's `filepath.Glob` auto-discovers new YAML files.

**Example YAML:**
```yaml
# testdata/oracle/scenarios/go_basic.yaml
name: "Go basic symbol operations"
fixture: "go"
requires_ls: "gopls"
steps:
  - tool: "search_symbols"
    args:
      query: "Helper"
    assert:
      not_error: true
      contains: "Helper"
      min_lines: 1

  - tool: "go_to_definition"
    args:
      path: "main.go"
      line: 6
      column: 1
    assert:
      not_error: true
      contains: "main.go"

  - tool: "get_symbol_overview"
    args:
      path: "main.go"
    assert:
      not_error: true
      contains_all: ["Helper", "DemoStruct"]
```

**Example runner:**
```go
// test/oracle/scenario/runner.go
//go:build integration && scenario

package scenario

import (
    "os"
    "path/filepath"
    "testing"

    "gopkg.in/yaml.v3"
    "github.com/postfix/serena/test/harness"
)

type Scenario struct {
    Name       string  `yaml:"name"`
    Fixture    string  `yaml:"fixture"`
    RequiresLS string  `yaml:"requires_ls"`
    Steps      []Step  `yaml:"steps"`
}

type Step struct {
    Tool   string         `yaml:"tool"`
    Args   map[string]any `yaml:"args"`
    Assert Assertion      `yaml:"assert"`
}

type Assertion struct {
    NotError    bool     `yaml:"not_error"`
    IsError     bool     `yaml:"is_error"`
    Contains    string   `yaml:"contains"`
    ContainsAll []string `yaml:"contains_all"`
    MinLines    int      `yaml:"min_lines"`
    Golden      string   `yaml:"golden"`
}

func RunScenario(t *testing.T, s Scenario) {
    t.Helper()
    if s.RequiresLS != "" {
        harness.RequireLS(t, s.RequiresLS)
    }
    fixture := harness.PrepareFixture(t, s.Fixture)
    td := harness.StartTestDaemon(t, harness.Options{WorkspaceDir: fixture})

    for i, step := range s.Steps {
        t.Run(fmt.Sprintf("step_%d_%s", i, step.Tool), func(t *testing.T) {
            if step.Assert.IsError {
                result := harness.CallToolExpectError(t, td.Session, step.Tool, step.Args)
                applyAssertions(t, result, step.Assert)
            } else {
                result := harness.CallTool(t, td.Session, step.Tool, step.Args)
                applyAssertions(t, result, step.Assert)
            }
        })
    }
}
```

### Pattern 2: Hierarchical Golden File Organization

**What:** Golden files organized by `testdata/oracle/{layer}/{fixture-lang}/{tool}.golden` with auto-discovery.

**When:** Scaling from 19 profile goldens to hundreds of contract/scenario goldens.

**Why:** The existing flat `testdata/profiles/*.tools.golden` pattern works at 19 files but becomes unnavigable at 100+. Subdirectories per fixture language and per oracle layer keep things organized.

**Naming convention:**
```
testdata/oracle/contracts/{lang}/{tool}.golden          # happy path
testdata/oracle/contracts/{lang}/{tool}.{variant}.golden # variant
testdata/oracle/contracts/error_shapes/{category}.golden # error shapes
testdata/oracle/protocol/{aspect}.golden                 # protocol
```

**Update mechanism:** Extend the existing `-update` flag pattern. The flag is already wired as `flag.Bool("update", ...)` and checks `GOLDEN_UPDATE=1`. New oracle golden helpers respect the same flag.

### Pattern 3: Build Tag Layering for CI Stages

**What:** Multiple build tags control which oracle layers run, mapping to CI pipeline stages.

**Tag design:**

| Build Tag | Oracle Layers | What Runs |
|-----------|--------------|-----------|
| `integration` | Protocol + Contract (layers 1-2) | All deterministic non-LS tests |
| `integration` + LS installed | Protocol + Contract with LS | Deterministic LS-dependent tests (skip if LS absent) |
| `scenario` (implies `integration`) | Scenario (layer 3) | YAML-driven multi-step scenarios |
| `llmtest` | Behavioral (layer 4) | Claude API tool selection tests |
| `llmjudge` | Judge (layer 5) | Claude API judge scoring |

**CI stage mapping:**

```bash
# Stage 1: Fast deterministic (protocol + contract, ~30s)
go test -tags integration ./test/oracle/protocol/... ./test/oracle/contract/...

# Stage 2: Scenarios (needs LS installed, ~2-5min)
go test -tags "integration scenario" ./test/oracle/scenario/...

# Stage 3: Race detection on deterministic + scenarios (~5-10min)
go test -tags "integration scenario" -race ./test/oracle/...

# Stage 4: LLM behavioral (needs ANTHROPIC_API_KEY, ~1-3min)
go test -tags llmtest ./test/oracle/behavioral/...

# Stage 5: Optional LLM judge (needs ANTHROPIC_API_KEY, ~2-5min)
go test -tags llmjudge ./test/oracle/judge/...
```

**Stage dependency chain:**
```
Stage 1 (fast) -> Stage 2 (scenarios) -> Stage 3 (race)
                                              |
                                              v
                                      Stage 4 (LLM behavioral)
                                              |
                                              v
                                      Stage 5 (LLM judge, optional)
```

Stages 1-3 are blocking for merge. Stage 4 is informational (fail does not block). Stage 5 is optional/manual.

### Pattern 4: Environment-Gated LLM Tests

**What:** LLM tests gated by both build tag AND environment variable.

**Why:** Build tags prevent compilation (no Claude SDK import in deterministic builds). Environment variable provides runtime skip when API key is absent. Double gating prevents accidental CI cost.

```go
//go:build llmtest

package behavioral_test

func TestToolSelection_SearchIntent(t *testing.T) {
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        t.Skip("ANTHROPIC_API_KEY not set")
    }
    // ... test with Claude API
}
```

### Pattern 5: Fixture Registry

**What:** A registry mapping fixture names to metadata (language, required LS binary, capabilities).

**When:** Scenario loader needs to know which LS to check for.

**Why:** Avoids hardcoding `requireGopls`-style checks per test file. Scenarios declare `fixture: "go"` and the registry handles LS availability checks.

```go
// test/harness/fixtures.go
type FixtureMeta struct {
    Name       string
    Language   string
    LSBinary   string   // e.g., "gopls", "pylsp"
    Supports   []string // e.g., ["symbols", "edit", "diagnostics"]
    Degraded   bool     // true for fixtures testing degraded behavior
}

var Fixtures = map[string]FixtureMeta{
    "go":          {Name: "go", Language: "go", LSBinary: "gopls",
                    Supports: []string{"symbols", "edit", "diagnostics"}},
    "python":      {Name: "python", Language: "python", LSBinary: "pylsp",
                    Supports: []string{"symbols", "diagnostics"}},
    "typescript":  {Name: "typescript", Language: "typescript",
                    LSBinary: "typescript-language-server",
                    Supports: []string{"symbols", "edit", "diagnostics"}},
    "polyglot":    {Name: "polyglot", Language: "multi", LSBinary: "",
                    Supports: []string{"symbols"}},
    "unsupported": {Name: "unsupported", Language: "brainfuck", LSBinary: "",
                    Supports: []string{}},
    "degraded":    {Name: "degraded", Language: "go", LSBinary: "gopls",
                    Degraded: true},
    "collisions":  {Name: "collisions", Language: "go", LSBinary: "gopls",
                    Supports: []string{"symbols"}},
}
```

### Pattern 6: Oracle Layer Independence

**What:** Each oracle layer is a separate Go package with its own build tag. No oracle layer imports another oracle layer.

**When:** Always. This prevents cascading failures and keeps build times predictable.

**Why:** If the LLM behavioral layer imported scenario infrastructure directly, a broken scenario package would prevent LLM tests from compiling. Instead, shared infrastructure lives in `test/harness/` and each oracle layer imports only from there.

```
test/harness/     <-- shared foundation (no build tag)
     ^    ^    ^
     |    |    |
     |    |    +-- test/oracle/protocol/   (//go:build integration)
     |    +------- test/oracle/contract/   (//go:build integration)
     |    +------- test/oracle/scenario/   (//go:build integration && scenario)
     +------------ test/oracle/behavioral/ (//go:build llmtest)
     +------------ test/oracle/judge/      (//go:build llmjudge)
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Monolithic Oracle Package

**What:** All five oracle layers in a single `test/oracle/` package with conditional compilation.

**Why bad:** Build tag interactions become unpredictable. LLM dependencies leak into deterministic test compilation. `go test -tags integration ./test/oracle/...` would try to compile LLM code.

**Instead:** Separate sub-packages per oracle layer.

### Anti-Pattern 2: Duplicating Harness Code

**What:** Copying `StartTestDaemon` into oracle packages.

**Why bad:** Two sources of truth for daemon lifecycle. Harness changes don't propagate.

**Instead:** Extract to `test/harness/` and import.

### Anti-Pattern 3: Exact String Matching in YAML Scenarios

**What:** Putting exact expected output strings in scenario YAML files.

**Why bad:** Expected outputs change with LS versions, formatting. YAML becomes fragile.

**Instead:** Use assertion types (contains, min_lines, not_error, golden file reference). Reserve exact matching for golden files which have the `-update` workflow.

### Anti-Pattern 4: LLM Tests Blocking Deterministic CI

**What:** Running LLM tests in the same CI job as protocol/contract tests.

**Why bad:** API failures, rate limits, or cost spikes block feedback on deterministic tests.

**Instead:** Separate CI stages. LLM stages are informational/optional.

### Anti-Pattern 5: Golden Files Without `-update` Workflow

**What:** Golden files that must be manually edited when expectations change.

**Why bad:** At hundreds of files, manual edits are error-prone and demoralizing.

**Instead:** Every golden regenerable via `go test -tags "integration scenario" ./test/oracle/... -update`.

### Anti-Pattern 6: Sharing Daemon Instances Across Oracle Layers

**What:** One `TestMain`-scoped daemon serving all oracle tests to "save startup time."

**Why bad:** Cross-contamination between test cases. One test's workspace activation affects another's. The existing pattern of per-test `StartTestDaemon` with `t.Cleanup(td.Stop)` is correct.

**Instead:** Each test case gets its own daemon. The startup cost (~10ms without LS) is negligible for protocol/contract tests. For scenario tests with LS, share per-scenario (one daemon per YAML file, not per step).

## Integration Points with Existing Architecture

### What Stays Unchanged

| Component | Status | Rationale |
|-----------|--------|-----------|
| `test/integration/*_test.go` | Keep all tests | Existing regression coverage; v1.4 adds, not replaces |
| `test/bench/` | Untouched | Benchmarks orthogonal to oracle layers |
| `testdata/fixtures/{go,python,typescript,java,rust}/` | Keep as-is | Reused by scenario oracle |
| `testdata/profiles/*.tools.golden` | Keep as-is | Profile contract goldens remain |
| `//go:build integration` tag | Keep | Foundation tag for all deterministic tests |

### What Gets Modified

| Component | Change | Rationale |
|-----------|--------|-----------|
| `test/integration/harness.go` | Functions duplicated to `test/harness/`; original becomes thin import wrapper OR stays as-is if oracle layers import harness directly | Enable oracle layer imports |
| `test/integration/golden.go` | Core logic moves to `test/harness/golden.go`; extended with hierarchical support | Shared golden infrastructure |
| `test/integration/helpers.go` | Core logic moves to `test/harness/helpers.go` | Shared helper functions |
| `Makefile` | Add targets: `test-oracle`, `test-scenario`, `test-llm` | Developer convenience |

### What Gets Created

| Component | Purpose |
|-----------|---------|
| `test/harness/` | Importable test infrastructure package |
| `test/oracle/{protocol,contract,scenario,behavioral,judge}/` | Five oracle layer packages |
| `testdata/fixtures/{polyglot,unsupported,degraded,collisions}/` | New fixture repositories |
| `testdata/oracle/{protocol,contracts,scenarios,behavioral,judge}/` | Oracle-specific test data |

## Build Order

Dependencies between oracle layers dictate strict build order:

### Phase 0: Harness Extraction (prerequisite)

Extract `test/harness/` from `test/integration/`. Mechanical refactor. All existing tests pass unchanged.

- **Input:** Existing `test/integration/{harness,golden,helpers}.go`
- **Output:** `test/harness/` importable package
- **Depends on:** Nothing
- **Blocks:** Everything else
- **Risk:** LOW -- pure refactor, no behavioral change

### Phase 1: Protocol Oracle (Layer 1)

MCP init sequence, tool listing shape, session isolation, reconnect. No LS needed (SkipLS: true). Fast, deterministic.

- **Tests:** Init handshake fields, tool list schema, session state after reconnect
- **Depends on:** Phase 0
- **Blocks:** Nothing directly (validates infrastructure)
- **Risk:** LOW -- no LS dependency

### Phase 2: New Fixtures + Fixture Registry

Create polyglot, unsupported, degraded, collision fixtures. Build fixture registry in `test/harness/`.

- **Output:** 4 new `testdata/fixtures/` directories + `test/harness/fixtures.go`
- **Depends on:** Phase 0
- **Blocks:** Phase 4 (scenarios need fixtures)
- **Risk:** LOW for fixture creation, MEDIUM for polyglot fixture design (needs multiple language files in one repo)

### Phase 3: Contract Oracle (Layer 2)

Per-tool golden contracts. Start with Go fixture + gopls. One golden per tool per fixture language. Error shape goldens.

- **Tests:** Each tool's output against golden file, error response shapes
- **Golden files:** `testdata/oracle/contracts/{lang}/{tool}.golden`
- **Depends on:** Phase 0, Phase 1 (infrastructure confidence)
- **Blocks:** Phase 4 (scenarios build on contract assertion patterns)
- **Risk:** MEDIUM -- golden file content depends on LS version; needs `-update` workflow from day 1

### Phase 4: Scenario Oracle (Layer 3)

YAML-driven scenarios. Loader, runner, assertion engine. Fixture registry integration. `scenario` build tag.

- **Tests:** Multi-step tool call sequences from YAML, cross-fixture scenarios
- **Depends on:** Phase 2 (fixtures), Phase 3 (assertion patterns)
- **Blocks:** Phase 6 (LLM behavioral reuses scenario infrastructure for setup)
- **Risk:** MEDIUM -- YAML schema design is the critical decision

### Phase 5: CI Pipeline

GitHub Actions workflow with 5 stages. Wire build tags to stages. Define pass/fail criteria.

- **Output:** `.github/workflows/oracle.yml`
- **Depends on:** Phases 1-4 (deterministic layers exist)
- **Blocks:** Nothing (can be done incrementally)
- **Risk:** LOW

### Phase 6: LLM Behavioral Oracle (Layer 4)

Claude API integration. Tool selection tests, disambiguation, output interpretation. `llmtest` build tag. Environment variable gating.

- **Depends on:** Phase 4 (scenario setup infrastructure)
- **Blocks:** Phase 7
- **Risk:** MEDIUM -- non-deterministic outputs need statistical assertions (pass 4/5 runs)

### Phase 7: LLM Judge Oracle (Layer 5)

Structured rubric scoring. `llmjudge` build tag. Optional/informational only.

- **Depends on:** Phase 6 (Claude client wrapper)
- **Blocks:** Nothing
- **Risk:** LOW -- purely optional/informational

## Scalability Considerations

| Concern | At 19 goldens (current) | At 100 goldens | At 500+ goldens |
|---------|------------------------|----------------|-----------------|
| File organization | Flat directory works | Subdirectories by lang/layer needed | Auto-discovery essential |
| Update workflow | Manual `-update` flag | Same flag, per-layer targeting | CI job that auto-updates on LS version bump |
| CI runtime | ~30s | ~2min (parallel subtests) | ~5min (parallel + LS caching) |
| Golden review | PR diff readable | Manageable with directory grouping | Consider golden diff summary tool |
| Fixture management | 5 fixtures, manual | 10 fixtures, registry | Registry + CI matrix for LS versions |

## Sources

- Existing codebase: `test/integration/harness.go`, `golden.go`, `helpers.go`, `mode_golden_test.go`, `concurrency_test.go`, `errors_test.go` -- HIGH confidence (direct code read)
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) -- HIGH confidence
- [File-driven testing in Go - Eli Bendersky](https://eli.thegreenplace.net/2022/file-driven-testing-in-go/) -- HIGH confidence (auto-discovery pattern)
- [Extending go test for LLM Evaluation - Mattermost](https://mattermost.com/blog/extending-go-test-for-llm-evaluation/) -- MEDIUM confidence (env-var gating pattern)
- [Go build tags for CI - DEV Community](https://dev.to/enbis/how-to-use-build-tags-to-control-go-testing-with-a-gitlab-ci-use-case-584b) -- HIGH confidence
- [Beyond Traditional Testing: Non-Deterministic Software - AWS](https://dev.to/aws/beyond-traditional-testing-addressing-the-challenges-of-non-deterministic-software-583a) -- MEDIUM confidence
- [goldie - Golden file testing for Go](https://github.com/sebdah/goldie) -- HIGH confidence (pattern reference, not recommending as dependency)
