# Technology Stack — v1.4 Multi-Oracle Integration Test Harness

**Project:** Serena (Go MCP code intelligence platform)
**Milestone:** v1.4 — Integration Testing v2 (multi-oracle test architecture)
**Researched:** 2026-04-11
**Go version:** 1.25.1

## Philosophy

v1.4 is a **test-only milestone** — no runtime production code changes. Every addition lands in `test/` or CI tooling. The existing v1.1 test infrastructure (InMemory + HTTP transports, golden file pattern, `testify`, `PrepareFixture`) is the foundation. New libraries extend it for contract validation, LLM behavioral testing, and structured reporting. Nothing touches `go.mod`'s production dependency set except test-only imports.

**Hard rules:**
- No new test framework replacing `testing.T`. Stdlib `testing` + `testify` remain the runners.
- No test database. Golden files + in-memory assertions only.
- No test orchestrator (Terratest, etc.). Go subtests with build tags handle staging.
- LLM client is test-only, guarded by build tag, skippable without API key.

---

## Recommended Stack Additions

### 1. JSON Schema Validation for Tool Contract Testing

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Validate MCP tool input/output schemas against JSON Schema drafts | Best-in-class Go JSON Schema validator. Supports draft-2020-12 (MCP spec alignment), detailed `ValidationError` with `BasicOutput()`/`DetailedOutput()` for clear test failure messages. Compiler-based API (`NewCompiler() -> Compile() -> Validate()`) fits test setup/run pattern. |

**Why not alternatives:**

| Alternative | Why not |
|-------------|---------|
| `google/jsonschema-go` (already indirect dep via MCP SDK) | Its validation API (`Schema -> Resolve -> Validate`) works but is designed for SDK schema inference, not standalone schema-file-based validation. No structured error output (just `error`). `santhosh-tekuri` has richer validation error introspection needed for clear test failure reporting. |
| `kaptinlin/jsonschema` | Good Draft 2020-12 support but smaller community, less battle-tested. `santhosh-tekuri` has been maintained since 2017 with 5 major versions. |
| `xeipuuv/gojsonschema` | Only supports up to draft-7. MCP tool schemas use draft-2020-12 features. |
| Hand-rolled validation | Reinventing JSON Schema. The schemas are already defined by the MCP SDK `inputSchema` fields. |

**Integration point:** Contract tests load tool `inputSchema` from `ListTools()`, compile with `santhosh-tekuri`, then validate actual call arguments and response shapes. Golden schema files in `testdata/contracts/` serve as the oracle for expected tool schemas across versions.

**Usage pattern:**
```go
c := jsonschema.NewCompiler()
sch, err := c.Compile("testdata/contracts/go_to_definition.input.schema.json")
require.NoError(t, err)
err = sch.Validate(actualArgs)
// ValidationError gives structured output for test diff
```

---

### 2. LLM API Client for Behavioral Testing

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/anthropics/anthropic-sdk-go` | v1.28.0+ (latest) | Claude API calls for behavioral tests (tool selection, disambiguation, output interpretation) | Official Anthropic Go SDK. Type-safe, supports tool_use natively (Claude returns structured tool calls), built-in retry/error handling. MIT licensed. |

**Integration point:** Behavioral tests live behind `//go:build llm` build tag. Tests construct a Claude client, present it with Serena's tool list (from `ListTools()`), pose scenarios ("find the definition of function X in this Go project"), and assert Claude selects the correct tool with valid arguments. The LLM judge layer uses Claude to score response quality against rubrics.

**Guard pattern:**
```go
//go:build llm

func TestLLM_ToolSelection(t *testing.T) {
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        t.Skip("ANTHROPIC_API_KEY not set, skipping LLM behavioral test")
    }
    // ...
}
```

**Why not alternatives:**

| Alternative | Why not |
|-------------|---------|
| `sashabaranov/go-openai` | OpenAI client, not Anthropic. Test target is Claude. |
| Raw `net/http` + JSON | Type-safe SDK handles tool_use response parsing, retries, streaming. No reason to hand-roll. |
| `liushuangls/go-anthropic` | Community client. Official SDK exists and is actively maintained by Anthropic. |

**Cost/risk note:** LLM tests cost real API money. CI pipeline puts them in stage 4 (optional), never blocking PRs. Local runs require explicit `-tags llm` and env var.

---

### 3. Structured Test Reporting

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `go test -json` (stdlib) | Go 1.25 | JSON-structured test output for CI parsing | Built into `go test` since Go 1.10. No dependency. Emits `test2json` events with Action/Package/Test/Elapsed/Output fields. |
| `gotest.tools/gotestsum` | v1.12.x (CI tool) | Pretty test output + JUnit XML for CI dashboards | Wraps `go test -json`, produces JUnit XML for GitHub Actions annotations. CI-only binary install, not a module import. |

**What NOT to add:**

| Alternative | Why not |
|-------------|---------|
| Custom test reporter framework | `go test -json` already emits structured events. Parse with `jq` or `gotestsum`. |
| `onsi/ginkgo` + `onsi/gomega` | BDD framework. Alien to existing `testing.T` + `testify` patterns. Would force rewrite of all existing tests. |
| `smarty/goconvey` | Same problem — different test paradigm. |
| Test metrics to Prometheus | Test metrics are ephemeral. JUnit XML + `go test -json` artifacts in CI are sufficient. |

**Integration point:** CI pipeline captures `go test -json` output per stage, `gotestsum` converts to JUnit XML for GitHub Actions test annotations. No in-code changes needed.

---

### 4. Test Fixture Management for Multi-Repo Scenarios

**No new library required.** Extend existing `PrepareFixture()` pattern from `test/integration/harness.go`.

| Approach | Purpose | Why |
|----------|---------|-----|
| Embedded fixtures via `testdata/` | Deterministic, version-controlled, fast | Already works for Go/Python/TypeScript/Java/Rust fixtures. Add polyglot monorepo and edge-case fixtures as new directories. |
| `testing.TB.TempDir()` (stdlib) | Per-test isolation for fixture copies | Already used in `PrepareFixture()`. Auto-cleaned. |
| `os.MkdirAll` + file copy (stdlib) | Fixture setup for multi-repo scenarios | Already implemented in `PrepareFixture()` with `filepath.WalkDir`. |

**What NOT to add:**

| Alternative | Why not |
|-------------|---------|
| `testcontainers-go` | No Docker containers needed. Serena wraps local LS binaries. |
| `ory/dockertest` | Same — no containerized deps. |
| `go-git/go-git` | Fixtures don't need git history. Plain directory copies suffice. |
| Fixture generation tools | Fixtures are small, hand-crafted, deterministic. Generation adds complexity. |

**New fixture directories to create:**

```
testdata/fixtures/
  go/           # existing
  python/       # existing
  typescript/   # existing
  java/         # existing
  rust/         # existing
  polyglot/     # NEW: monorepo with Go + Python + TypeScript
  unsupported/  # NEW: .cobol/.fortran files (no LS available)
  collisions/   # NEW: same symbol names across languages
  degraded/     # NEW: valid code but broken go.mod / missing deps
```

**Multi-repo scenario pattern:** `PrepareFixture(t, "polyglot")` copies the polyglot fixture. Tests activate project and assert cross-language behavior (no fake cross-language links, proper per-language scoping).

---

### 5. MCP Client-Side Testing

**No new library required.** The MCP Go SDK v1.5.0 already provides everything needed:

| Existing capability | How v1.4 uses it |
|---------------------|------------------|
| `mcp.NewInMemoryTransports()` | Protocol correctness tests (init sequence, tool listing, session isolation) |
| `mcp.NewClient()` + `ClientSession` | All client-side operations (ListTools, CallTool, reconnect scenarios) |
| `httptest.NewServer()` + `StreamableClientTransport` | HTTP transport protocol tests |
| `mcp.LoggingTransport` | Debug logging for protocol-level test failures |

**Protocol correctness tests use existing InMemory transport:**
```go
// Session isolation: two clients, same daemon, separate sessions
t1, t2 := mcp.NewInMemoryTransports()
// ... connect client A
t3, t4 := mcp.NewInMemoryTransports()
// ... connect client B
// Assert: client A's workspace activation doesn't affect client B
```

**Reconnect testing:** Create client, connect, disconnect (`session.Close()`), reconnect with new transport pair. Assert daemon state survives.

---

### 6. Golden File Pattern Extension

**No new library required.** Extend existing `golden.go` pattern.

| Existing | Extension for v1.4 |
|----------|---------------------|
| `assertGoldenTools()` — tool name lists | Add `assertGoldenJSON()` — full JSON response comparison with diff |
| `-update` flag / `GOLDEN_UPDATE=1` env | Same mechanism, same flag |
| `testdata/profiles/*.tools.golden` | Add `testdata/contracts/*.golden.json` for tool contract outputs |

**Golden JSON comparison approach:**
```go
func assertGoldenJSON(t *testing.T, name string, actual any) {
    // Marshal actual to indented JSON
    // Compare with testdata/contracts/<name>.golden.json
    // On -update, rewrite golden
    // On mismatch, show unified diff
}
```

**Why no golden file library:**

| Alternative | Why not |
|-------------|---------|
| `sebdah/goldie` | Adds a dep for ~50 lines of code we already have the pattern for. Our `golden.go` handles update flags, directory management, and diff output. |
| `bradleyjkemp/cupaloy` | Snapshot testing library. Different paradigm (auto-names snapshots). Our explicit golden naming is better for contract testing where file names map to tool names. |
| `maxatome/go-testdeep` | Deep comparison library. `testify/assert` + `encoding/json` already handle this. |

---

## Data-Driven Scenario Tests

**No new library required.** Go's `testing.T` subtests with table-driven patterns are the standard.

```go
// Scenario matrix: each entry is a test case
scenarios := []struct {
    name     string
    fixture  string
    tool     string
    args     map[string]any
    wantErr  bool
    golden   string
}{
    {"go_definition_main", "go", "go_to_definition", map[string]any{...}, false, "go_def_main"},
    {"python_references", "python", "find_references", map[string]any{...}, false, "py_refs"},
    // ...
}
for _, sc := range scenarios {
    t.Run(sc.name, func(t *testing.T) {
        // ...
    })
}
```

**Why not test frameworks:**

| Alternative | Why not |
|-------------|---------|
| `cucumber/godog` | BDD/Gherkin. Adds feature file parsing, step definitions, a different test paradigm. Go subtests are simpler and the whole team already knows them. |
| `onsi/ginkgo` | Already rejected above. Same reasoning. |
| `DATA-DOG/godog` | Same as cucumber/godog. |

---

## Complete go.mod Delta

```bash
# JSON Schema validation (test-only, but go.mod doesn't distinguish)
go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.2

# LLM behavioral testing (test-only, guarded by build tag)
go get github.com/anthropics/anthropic-sdk-go@latest
```

**Net dependency additions:** 2 direct modules. Both are test-only in practice (used only in `test/` packages with appropriate build tags), but Go's module system includes them in `go.mod` regardless.

**CI-only tools (not in go.mod):**
```bash
go install gotest.tools/gotestsum@latest
```

---

## Integration with Existing v1.1 Test Infrastructure

| Existing v1.1 component | v1.4 relationship |
|--------------------------|-------------------|
| `test/integration/harness.go` (`StartTestDaemon`, `PrepareFixture`, `WaitForLS`) | **Reuse directly.** All new oracle layers build on this harness. |
| `test/integration/golden.go` (`assertGoldenTools`, `-update` flag) | **Extend.** Add `assertGoldenJSON()` for contract output goldens. |
| `test/integration/helpers.go` (`callTool`, `textContent`, `callToolExpectError`) | **Reuse directly.** These are the test-side MCP client wrappers. |
| `testdata/fixtures/{go,python,typescript,java,rust}` | **Extend.** Add polyglot, unsupported, collisions, degraded fixtures. |
| `testdata/profiles/*.tools.golden` (19 goldens) | **Keep.** Profile/mode contract tests remain independent from new layers. |
| `//go:build integration` tag | **Keep for deterministic tests.** Add `//go:build llm` for behavioral layer. |
| `stretchr/testify` (require/assert) | **Keep.** All assertion patterns stay consistent. |
| InMemory + HTTP dual transport pattern | **Extend.** Protocol correctness tests exercise both transports. |

---

## What NOT to Add

Protecting the dependency budget for a test-only milestone:

- **No test framework** (ginkgo, godog, goconvey). Go subtests + testify is the pattern.
- **No container runtime** (testcontainers, dockertest). Tests run against local LS binaries.
- **No mock framework** (gomock, mockery). Integration tests use real daemon instances.
- **No snapshot testing library** (cupaloy, goldie). Existing golden pattern extends cleanly.
- **No git library** (go-git). Fixtures are plain directories.
- **No test database** (SQLite for test results). JUnit XML + `go test -json` artifacts suffice.
- **No custom test CLI**. `go test` with build tags and `-run` patterns handles all staging.
- **No OpenAI/multi-LLM client**. Tests target Claude only. One SDK.

---

## 5-Stage CI Pipeline (Build Tag Strategy)

| Stage | Build tags | Deps used | Speed |
|-------|-----------|-----------|-------|
| 1. Fast deterministic | `integration` | testify, jsonschema | ~30s |
| 2. Scenario matrix | `integration` | testify, jsonschema, fixtures | ~2-5min |
| 3. Concurrency + race | `integration` + `-race` | testify | ~3-5min |
| 4. LLM behavioral | `llm` | anthropic-sdk-go, testify | ~1-2min (API latency) |
| 5. LLM judge (optional) | `llm,judge` | anthropic-sdk-go, testify | ~2-5min (API latency) |

Stages 1-3 are deterministic and free. Stage 4-5 require `ANTHROPIC_API_KEY` and cost money. Stage 5 never blocks CI.

---

## Confidence Assessment

| Decision | Confidence | Basis |
|----------|------------|-------|
| `santhosh-tekuri/jsonschema/v6` for schema validation | HIGH | Battle-tested since 2017, v6 supports draft-2020-12, structured error output, well-documented. Verified on pkg.go.dev. |
| `anthropics/anthropic-sdk-go` for LLM testing | HIGH | Official Anthropic SDK, MIT licensed, actively maintained (v1.28.0), native tool_use support. Verified on GitHub. |
| `gotestsum` for CI reporting | HIGH | De facto standard for Go CI test reporting. JUnit XML output. CI-only install. |
| Extend existing golden pattern (no library) | HIGH | Pattern already proven with 19 goldens in v1.1. Extension to JSON is ~50 lines. |
| Extend existing `PrepareFixture` (no library) | HIGH | Already handles 5 language fixtures. Directory-based approach scales to 9+. |
| MCP SDK InMemory transports for protocol testing | HIGH | Already used in v1.1. MCP SDK v1.5.0 provides all needed client capabilities. |
| No test framework change | HIGH | Existing `testing.T` + `testify` is the standard Go pattern. No justification for migration. |
| Build tag staging (`integration`, `llm`, `judge`) | MEDIUM | Sound pattern but `llm` build tag is less common in Go projects. Env-var skip (`t.Skip`) is the fallback if build tags prove unwieldy. |

---

## Sources

- [santhosh-tekuri/jsonschema on pkg.go.dev](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6) — v6.0.2, Draft 2020-12 support
- [santhosh-tekuri/jsonschema releases](https://github.com/santhosh-tekuri/jsonschema/releases) — latest v6.0.2 (May 2025)
- [google/jsonschema-go on pkg.go.dev](https://pkg.go.dev/github.com/google/jsonschema-go/jsonschema) — v0.4.2 (already indirect dep)
- [Google Open Source Blog: JSON Schema for Go](https://opensource.googleblog.com/2026/01/a-json-schema-package-for-go.html)
- [anthropics/anthropic-sdk-go on GitHub](https://github.com/anthropics/anthropic-sdk-go) — v1.28.0+
- [Anthropic Go SDK docs](https://platform.claude.com/docs/en/api/sdks/go)
- [MCP Go SDK on pkg.go.dev](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) — v1.5.0
- [gotestyourself/gotestsum on GitHub](https://github.com/gotestyourself/gotestsum) — JUnit XML, structured output
- [Go test2json format](https://pkg.go.dev/cmd/test2json)
