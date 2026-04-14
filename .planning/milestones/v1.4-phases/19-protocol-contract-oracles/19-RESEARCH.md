# Phase 19: Protocol & Contract Oracles - Research

**Researched:** 2026-04-11
**Domain:** MCP protocol testing, JSON Schema validation, golden file contracts
**Confidence:** HIGH

## Summary

Phase 19 populates `test/oracle/protocol/` and `test/oracle/contract/` with deterministic correctness tests for the MCP protocol layer and all 34 exposed tools. The harness infrastructure from Phase 18 (`test/harness/`) provides `StartRunner`, `CallTool`, `CallToolExpectError`, `ListSessionTools`, `NewHTTPSession`, `AssertGolden`, and `PrepareFixture` -- all tested and ready.

The MCP Go SDK v1.5.0 exposes `ClientSession.InitializeResult()` returning `*InitializeResult` with `Capabilities`, `ProtocolVersion`, and `ServerInfo` -- giving direct access to everything needed for handshake assertion. The `ListToolsResult.Tools` field returns `[]*Tool` where each tool carries `InputSchema any`, `OutputSchema any`, `Name`, and `Description` -- all accessible from client-side for schema validation.

**Primary recommendation:** Use `santhosh-tekuri/jsonschema/v6` (v6.0.2) for Draft 2020-12 meta-schema validation. Build protocol tests as table-driven subtests in `test/oracle/protocol/`, contract tests (golden files, schema validation, error contracts, selectability) in `test/oracle/contract/`. Reuse harness primitives throughout -- do not duplicate them.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Transport parity uses handshake + one representative tool call on both InMemory and HTTP transports. No full mirror.
- **D-02:** Session isolation tests workspace + mode isolation -- two concurrent sessions with different workspaces and modes, verify tool results and mode restrictions apply independently.
- **D-03:** Reconnect testing uses client-side disconnect/reconnect -- drop MCP client session, create a new one against the same running daemon.
- **D-04:** Golden files capture normalized response shape -- field names, types, nesting structure with placeholders for non-deterministic values.
- **D-05:** Golden files live in `test/oracle/contract/testdata/golden/{tool_name}/`.
- **D-06:** Error goldens organized by error category -- one golden per category.
- **D-07:** Use `santhosh-tekuri/jsonschema/v6` for JSON Schema Draft 2020-12 validation.
- **D-08:** Validate both inputSchema and outputSchema against meta-schema. Validate tool results against declared outputSchema when present.
- **D-09:** Keep instance-validation and schema-validation as separate test paths.
- **D-10:** Oracle tests are the authoritative correctness layer. Existing integration tests continue unchanged.
- **D-11:** CONT-04 (tool description selectability) uses deterministic heuristics -- action verbs, uniqueness, disambiguation. No LLM.

### Claude's Discretion
- Test helper structure within protocol/ and contract/ packages (file split, internal types)
- Whether to add schema validation helpers to `test/harness/` or keep them local to contract/
- Specific normalization strategy for golden file placeholders (regex, json path masks, etc.)
- How to structure the transport parity test (table-driven vs separate functions)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PROTO-01 | MCP initialize/shutdown handshake tests validate capabilities, protocol version, and server info across InMemory and HTTP transports | `ClientSession.InitializeResult()` provides `Capabilities`, `ProtocolVersion`, `ServerInfo`; `runner.NewHTTPSession(t)` creates HTTP sessions; D-01 scopes to handshake + one tool call |
| PROTO-02 | tools/list validated -- schemas valid JSON Schema Draft 2020-12, names unique, descriptions non-empty | `ListToolsResult.Tools` returns `[]*Tool` with `InputSchema`, `OutputSchema`, `Name`, `Description`; `santhosh-tekuri/jsonschema/v6` compiles meta-schema for validation |
| PROTO-03 | Session isolation -- concurrent sessions with different workspaces/modes do not affect each other | `StartRunner` with separate `RunnerOptions` for each session; `runner.NewHTTPSession` or second `StartRunner` for separate session; D-02 scopes to workspace + mode |
| PROTO-04 | Reconnect -- disconnect/reconnect preserves daemon state, no worker leakage | D-03: drop client session, create new one against same daemon; `Pool.WorkerCount()` for leak detection |
| CONT-01 | Every exposed MCP tool has a golden output file capturing normalized response shape | 34 tools in full/admin profile; D-04/D-05 define shape normalization and testdata location; `harness.AssertGolden` handles comparison + update |
| CONT-02 | Every tool's inputSchema validated against Draft 2020-12 and cross-referenced against actual accepted arguments | D-07/D-08/D-09: jsonschema/v6 for meta-schema validation; separate test path for schema-validation vs instance-validation |
| CONT-03 | Error responses assert stable error class/code per category | D-06: one golden per error category (no_workspace, not_found, invalid_args, timeout, circuit_open, unsupported); `harness.CallToolExpectError` for error path |
| CONT-04 | Tool descriptions tested for LLM selectability | D-11: deterministic heuristics -- action verbs, uniqueness, disambiguation from neighbors |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Always run `go vet` and `go test` before completing any Go task
- Build: `go build ./cmd/serena`
- Test: `go test ./...` (default, no build tags) and `go test -tags integration ./...`
- Format: `gofmt -w .`
- Use `testify/require` for assertions, `t.Run` subtests for organization
- Build tag `//go:build integration || llm || llmjudge` for oracle test files
- External test packages (`protocol_test`, `contract_test`) for cross-package validation

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| santhosh-tekuri/jsonschema/v6 | v6.0.2 | JSON Schema Draft 2020-12 validation | Passes official JSON Schema Test Suite for 2020-12; user-selected (D-07) [VERIFIED: go list -m -versions] |
| modelcontextprotocol/go-sdk | v1.5.0 | MCP client/server protocol | Already in go.mod; provides Tool, ClientSession, ListTools, CallTool [VERIFIED: go.mod] |
| stretchr/testify | (existing) | Test assertions | Already used throughout test/ [VERIFIED: codebase] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| golang.org/x/sync/errgroup | (existing) | Concurrent test fan-out | PROTO-03 session isolation, PROTO-04 reconnect under load [VERIFIED: concurrency_test.go] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| santhosh-tekuri/jsonschema/v6 | xeipuuv/gojsonschema | xeipuuv only supports draft-7; no 2020-12 [ASSUMED] |
| santhosh-tekuri/jsonschema/v6 | google/jsonschema-go (already in SDK) | google/jsonschema-go is for schema generation not validation; SDK uses it internally for AddTool but doesn't expose validation API [VERIFIED: MCP SDK source] |

**Installation:**
```bash
go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.2
```

## Architecture Patterns

### Recommended Project Structure
```
test/oracle/
  protocol/
    doc.go                     # existing stub
    smoke_test.go              # existing harness import smoke
    handshake_test.go          # PROTO-01: initialize/capabilities/shutdown
    tools_list_test.go         # PROTO-02: schema validation, uniqueness, descriptions
    session_isolation_test.go  # PROTO-03: concurrent session independence
    reconnect_test.go          # PROTO-04: disconnect/reconnect resilience
  contract/
    doc.go                     # existing stub
    golden_test.go             # CONT-01: per-tool golden output files
    schema_test.go             # CONT-02: inputSchema/outputSchema meta-validation
    errors_test.go             # CONT-03: error category golden files
    selectability_test.go      # CONT-04: description heuristic checks
    testdata/
      golden/
        {tool_name}/
          success.golden       # normalized response shape
        errors/
          no_workspace.golden
          not_found.golden
          invalid_args.golden
          timeout.golden
          circuit_open.golden
          unsupported.golden
```

### Pattern 1: Handshake Assertion (PROTO-01)
**What:** Assert InitializeResult fields from `ClientSession.InitializeResult()` on both transports
**When to use:** Protocol correctness verification
**Example:**
```go
// Source: MCP SDK v1.5.0 client.go:327
func TestHandshake_InMemory(t *testing.T) {
    runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
    defer runner.Stop()

    result := runner.Session.InitializeResult()
    require.NotNil(t, result)
    require.NotEmpty(t, result.ProtocolVersion)
    require.NotNil(t, result.ServerInfo)
    require.Equal(t, "serena", result.ServerInfo.Name)
    require.NotNil(t, result.Capabilities)
    require.NotNil(t, result.Capabilities.Tools)
}
```
[VERIFIED: MCP SDK source -- InitializeResult() returns *InitializeResult with Capabilities, ProtocolVersion, ServerInfo]

### Pattern 2: Meta-Schema Validation (CONT-02)
**What:** Compile Draft 2020-12 meta-schema once, validate each tool's inputSchema/outputSchema
**When to use:** Schema correctness verification
**Example:**
```go
// Source: pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6
func compileMetaSchema(t *testing.T) *jsonschema.Schema {
    t.Helper()
    c := jsonschema.NewCompiler()
    c.DefaultDraft(jsonschema.Draft2020)
    sch, err := c.Compile("https://json-schema.org/draft/2020-12/schema")
    require.NoError(t, err)
    return sch
}

func TestSchemaValidation(t *testing.T) {
    runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
    defer runner.Stop()

    metaSchema := compileMetaSchema(t)
    ctx := context.Background()
    result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
    require.NoError(t, err)

    for _, tool := range result.Tools {
        t.Run(tool.Name+"_inputSchema", func(t *testing.T) {
            require.NotNil(t, tool.InputSchema)
            err := metaSchema.Validate(tool.InputSchema)
            require.NoError(t, err, "inputSchema for %s is not valid Draft 2020-12", tool.Name)
        })
    }
}
```
[CITED: pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6]

### Pattern 3: Golden File Shape Normalization (CONT-01)
**What:** Call each tool with known inputs, normalize non-deterministic values, assert against golden file
**When to use:** Contract stability verification
**Example:**
```go
// Normalize paths, timestamps, IDs to placeholders
func normalizeResponse(text string, workspaceDir string) string {
    text = strings.ReplaceAll(text, workspaceDir, "<WORKSPACE>")
    // Normalize absolute paths
    text = regexp.MustCompile(`/[^\s"]+/testdata/fixtures/`).ReplaceAllString(text, "<FIXTURE_ROOT>/")
    // Normalize timestamps
    text = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`).ReplaceAllString(text, "<TIMESTAMP>")
    return text
}
```
[ASSUMED -- normalization strategy is Claude's discretion per CONTEXT.md]

### Pattern 4: Session Isolation (PROTO-03)
**What:** Two concurrent sessions with different workspaces/modes, verify independence
**When to use:** Multi-session correctness
**Example:**
```go
func TestSessionIsolation(t *testing.T) {
    // Session A: Go workspace, edit mode
    runnerA := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: harness.PrepareFixture(t, "go"),
        Mode:         "edit",
    })
    // Session B: separate runner, read mode
    runnerB := harness.StartRunner(t, harness.RunnerOptions{
        WorkspaceDir: harness.PrepareFixture(t, "go"),
        Mode:         "read",
        SkipLS:       true, // separate daemon
    })
    // Verify mode restrictions apply independently
    // Verify tool results reflect correct workspace
}
```
[VERIFIED: harness.StartRunner creates independent daemon instances]

### Pattern 5: Reconnect (PROTO-04)
**What:** Drop client session, create new one against same running daemon
**When to use:** Disconnect resilience verification
**Example:**
```go
func TestReconnect(t *testing.T) {
    runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

    // Record initial state
    initialWorkers := runner.Daemon.KernelInstance().Pool().WorkerCount()

    // Drop client session (simulates disconnect)
    runner.Session.Close()

    // Create new session against same daemon
    newSession := runner.NewHTTPSession(t) // or new InMemory session

    // Verify daemon state preserved, new session functional
    tools := harness.ListSessionTools(t, newSession)
    require.NotEmpty(t, tools)

    // Verify no worker leakage
    currentWorkers := runner.Daemon.KernelInstance().Pool().WorkerCount()
    require.LessOrEqual(t, currentWorkers, initialWorkers)
}
```
[VERIFIED: runner.NewHTTPSession exists; Pool.WorkerCount() available for leak detection]

### Anti-Patterns to Avoid
- **String matching on error text:** Kernel does not yet expose typed errors (see errors_test.go TODO). Assert `IsError=true` and error category structure, not substring matches on messages. [VERIFIED: existing errors_test.go deviation note]
- **Testing all tools on both transports:** D-01 explicitly limits HTTP to handshake + one representative call. Do not duplicate the full contract suite on HTTP.
- **Inventing output schemas:** D-08 says validate outputSchema when declared. Do not create output schemas for tools that don't declare one.
- **Golden file exact bytes:** D-04 mandates normalized shape comparison. Never compare raw bytes with timestamps/paths.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Schema validation | Custom schema walker | santhosh-tekuri/jsonschema/v6 | Meta-schema compilation, draft detection, vocabulary support are deceptively complex |
| Golden file comparison | Custom diff logic | harness.AssertGolden with -update flag | Already handles update mode, directory creation, diff output |
| MCP client session | Raw JSON-RPC client | mcp.NewClient + InMemoryTransports/StreamableClientTransport | Protocol negotiation, capability exchange handled by SDK |
| Test daemon lifecycle | Manual daemon setup | harness.StartRunner | Handles config, skill init, kernel start, cleanup |

## Common Pitfalls

### Pitfall 1: InputSchema is `any` type
**What goes wrong:** `Tool.InputSchema` is `any` -- on the client side it deserializes as `map[string]any`, not a typed struct. Attempting type assertion to a struct type fails.
**Why it happens:** MCP SDK uses `any` for schema fields to support arbitrary JSON Schema objects.
**How to avoid:** Pass `tool.InputSchema` directly to `jsonschema.Schema.Validate()` which accepts `any`. For meta-schema validation, add the schema as a resource via `compiler.AddResource(toolName, tool.InputSchema)` then compile and validate.
**Warning signs:** Runtime panic on type assertion.
[VERIFIED: MCP SDK v1.5.0 protocol.go:1305 -- `InputSchema any`]

### Pitfall 2: Meta-schema network fetch
**What goes wrong:** `jsonschema.Compiler.Compile("https://json-schema.org/draft/2020-12/schema")` may attempt a network fetch in CI environments without internet.
**Why it happens:** The compiler resolves URIs to load schema definitions.
**How to avoid:** `santhosh-tekuri/jsonschema/v6` bundles the official meta-schemas -- `Draft2020` is a built-in constant. Set `c.DefaultDraft(jsonschema.Draft2020)` and the meta-schema is resolved locally. Verify this works offline in tests.
**Warning signs:** Test timeout or DNS resolution failure in CI.
[CITED: pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6]

### Pitfall 3: Session isolation requires separate daemons
**What goes wrong:** Two sessions against the same daemon share kernel state (workspace, workers). "Different workspaces" means the activate_project call changes state for both.
**Why it happens:** The current daemon is single-workspace (one active workspace at a time per daemon).
**How to avoid:** For PROTO-03, start two separate `StartRunner` instances (each creates its own daemon). Alternatively, if testing that the same daemon handles mode isolation, use the same daemon but don't assume separate workspaces.
**Warning signs:** Second session's activate_project overwrites first session's workspace.
[VERIFIED: harness.StartRunner creates independent daemon instances; runner.go shows workspace activation is daemon-scoped]

### Pitfall 4: Golden file creation on first run
**What goes wrong:** Tests fail on first run because golden files don't exist yet.
**Why it happens:** `AssertGolden` requires the golden file to exist unless `-update` is passed.
**How to avoid:** First run must use `GOLDEN_UPDATE=1 go test -tags integration ./test/oracle/contract/...` to generate baseline golden files. Document this in the plan.
**Warning signs:** "run with -update to create" error message.
[VERIFIED: harness/golden.go:97]

### Pitfall 5: Tool count varies by profile and mode
**What goes wrong:** Assuming 34 tools in all test contexts.
**Why it happens:** Profile filtering middleware removes tools based on active profile and mode.
**How to avoid:** Use `full` profile with `admin` mode for comprehensive tool testing (CONT-01, CONT-02). Use specific profiles for isolation tests.
**Warning signs:** Missing tools in golden comparison.
[VERIFIED: profile golden files show different tool counts per profile/mode]

## Code Examples

### Complete Meta-Schema Validation Setup
```go
// Source: santhosh-tekuri/jsonschema/v6 docs + MCP SDK Tool type
package contract_test

import (
    "context"
    "testing"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/santhosh-tekuri/jsonschema/v6"
    "github.com/stretchr/testify/require"

    "github.com/postfix/serena/test/harness"
)

func compileMetaSchema(t *testing.T) *jsonschema.Schema {
    t.Helper()
    c := jsonschema.NewCompiler()
    c.DefaultDraft(jsonschema.Draft2020)
    sch, err := c.Compile("https://json-schema.org/draft/2020-12/schema")
    require.NoError(t, err, "failed to compile Draft 2020-12 meta-schema")
    return sch
}

func TestAllTools_InputSchemaValid(t *testing.T) {
    runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
    defer runner.Stop()

    meta := compileMetaSchema(t)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
    require.NoError(t, err)

    for _, tool := range result.Tools {
        t.Run(tool.Name, func(t *testing.T) {
            require.NotNil(t, tool.InputSchema, "inputSchema must not be nil")
            err := meta.Validate(tool.InputSchema)
            require.NoError(t, err, "inputSchema is not valid Draft 2020-12")
        })
    }
}
```

### Selectability Heuristic Checks (CONT-04)
```go
// Source: D-11 from CONTEXT.md
package contract_test

import (
    "strings"
    "testing"
    "unicode"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

var actionVerbs = []string{
    "activate", "analyze", "create", "delete", "edit", "find", "format",
    "get", "go", "insert", "list", "onboard", "prepare", "read", "rename",
    "replace", "safe", "search", "switch", "verify", "write",
}

func hasActionVerb(desc string) bool {
    lower := strings.ToLower(desc)
    for _, v := range actionVerbs {
        if strings.Contains(lower, v) {
            return true
        }
    }
    return false
}

func TestSelectability_ActionVerbs(t *testing.T) {
    // ... list tools, for each assert hasActionVerb(tool.Description)
}

func TestSelectability_UniqueDescriptions(t *testing.T) {
    // ... list tools, collect descriptions, assert no duplicates
}

func TestSelectability_Disambiguation(t *testing.T) {
    // Similar tools should have distinguishing terms
    similarPairs := [][2]string{
        {"search_symbols", "find_references"},
        {"get_symbol_overview", "get_hover_info"},
        {"read_file", "read_memory"},
    }
    // For each pair, verify descriptions differ meaningfully
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| xeipuuv/gojsonschema (draft-7 only) | santhosh-tekuri/jsonschema/v6 (draft 2020-12) | v6.0.0 (2024) | Full 2020-12 support required by MCP SDK |
| MCP SDK v0.x manual handshake | MCP SDK v1.5.0 ClientSession.InitializeResult() | 2025 | Direct access to handshake results |
| Custom golden comparison | harness.AssertGolden with -update flag | Phase 18 | Standardized golden file workflow |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Golden file normalization via regex replacement of paths/timestamps is sufficient for shape comparison | Architecture Patterns (Pattern 3) | May need JSON-path-based masking if tool output structures are deeply nested with non-deterministic values at varying depths |
| A2 | `jsonschema.Compile("https://json-schema.org/draft/2020-12/schema")` resolves locally without network in v6.0.2 | Pitfalls (Pitfall 2) | Tests may fail in offline CI; would need to embed meta-schema manually |
| A3 | Session isolation via separate StartRunner instances is sufficient for PROTO-03 | Pitfalls (Pitfall 3) | If the intent is to test same-daemon multi-session isolation, approach needs redesign |
| A4 | xeipuuv/gojsonschema does not support draft 2020-12 | Alternatives Considered | Low risk -- well-known limitation |

## Open Questions (RESOLVED)

1. **Session isolation scope (PROTO-03)**
   - What we know: D-02 says "workspace + mode isolation". Current daemon is single-workspace.
   - What's unclear: Whether to test two separate daemons (true isolation) or same daemon with mode-only differences.
   - RESOLVED: Two separate StartRunner instances for full workspace isolation. Same daemon for mode-only isolation (one session in edit, another session connects and operates in read mode -- verifying tool visibility differs).

2. **OutputSchema coverage**
   - What we know: D-08 says validate outputSchema when declared. MCP SDK Tool has OutputSchema field.
   - What's unclear: How many tools currently declare outputSchema (may be none -- Serena tools mostly return text content).
   - RESOLVED: Iterate tools in the schema test, validate outputSchema only when non-nil. If none have it, the test still passes (no false failures).

3. **Error category coverage for timeout/circuit_open/unsupported**
   - What we know: D-06 lists 6 categories. Current error tests only cover no_workspace, not_found, invalid_args.
   - What's unclear: Whether timeout and circuit_open can be triggered deterministically in tests without LS load.
   - RESOLVED: Implement deterministic categories (no_workspace, not_found, invalid_args, unsupported) first. Mark timeout and circuit_open as needing runtime/scenario infrastructure from Phase 20 if they can't be triggered deterministically.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.x |
| Config file | None needed -- uses build tags |
| Quick run command | `go test -tags integration -run TestSmoke ./test/oracle/protocol/...` |
| Full suite command | `go test -tags integration -count=1 -timeout 5m ./test/oracle/protocol/... ./test/oracle/contract/...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROTO-01 | Handshake on InMemory + HTTP | integration | `go test -tags integration -run TestHandshake ./test/oracle/protocol/...` | No -- Wave 0 |
| PROTO-02 | tools/list schema validation | integration | `go test -tags integration -run TestToolsList ./test/oracle/protocol/...` | No -- Wave 0 |
| PROTO-03 | Session isolation | integration | `go test -tags integration -run TestSessionIsolation ./test/oracle/protocol/...` | No -- Wave 0 |
| PROTO-04 | Reconnect resilience | integration | `go test -tags integration -run TestReconnect ./test/oracle/protocol/...` | No -- Wave 0 |
| CONT-01 | Per-tool golden outputs | integration | `go test -tags integration -run TestGolden ./test/oracle/contract/...` | No -- Wave 0 |
| CONT-02 | Schema meta-validation | integration | `go test -tags integration -run TestSchema ./test/oracle/contract/...` | No -- Wave 0 |
| CONT-03 | Error category contracts | integration | `go test -tags integration -run TestError ./test/oracle/contract/...` | No -- Wave 0 |
| CONT-04 | Description selectability | integration | `go test -tags integration -run TestSelectability ./test/oracle/contract/...` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -tags integration -count=1 -timeout 3m ./test/oracle/protocol/... ./test/oracle/contract/...`
- **Per wave merge:** Full suite with `-race`: `go test -tags integration -race -count=1 -timeout 5m ./test/oracle/protocol/... ./test/oracle/contract/...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.2` -- new dependency
- [ ] `test/oracle/protocol/handshake_test.go` -- PROTO-01
- [ ] `test/oracle/protocol/tools_list_test.go` -- PROTO-02
- [ ] `test/oracle/protocol/session_isolation_test.go` -- PROTO-03
- [ ] `test/oracle/protocol/reconnect_test.go` -- PROTO-04
- [ ] `test/oracle/contract/golden_test.go` -- CONT-01
- [ ] `test/oracle/contract/schema_test.go` -- CONT-02
- [ ] `test/oracle/contract/errors_test.go` -- CONT-03
- [ ] `test/oracle/contract/selectability_test.go` -- CONT-04
- [ ] `test/oracle/contract/testdata/golden/` -- initial golden files (generated via -update)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A (test code only) |
| V3 Session Management | yes | PROTO-03 verifies session isolation; PROTO-04 verifies reconnect doesn't leak state |
| V4 Access Control | yes | PROTO-03 verifies mode restrictions apply independently per session |
| V5 Input Validation | yes | CONT-02 validates all inputSchemas are well-formed; CONT-03 validates error responses for bad input |
| V6 Cryptography | no | N/A |

### Known Threat Patterns for MCP Test Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Session state leakage between clients | Information Disclosure | PROTO-03 isolation tests |
| Worker pool leak on disconnect | Denial of Service | PROTO-04 WorkerCount assertion |
| Mode bypass via concurrent access | Elevation of Privilege | PROTO-03 mode restriction verification |
| Invalid schema accepting bad input | Tampering | CONT-02 meta-schema validation |

## Sources

### Primary (HIGH confidence)
- MCP Go SDK v1.5.0 source (`go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.5.0/mcp/`) -- Tool type, ClientSession, InitializeResult, ServerCapabilities
- `test/harness/` package source -- StartRunner, CallTool, AssertGolden, NewHTTPSession APIs
- `test/integration/` existing test patterns -- errors_test.go, concurrency_test.go, smoke_http_test.go, profile_golden_test.go
- `test/oracle/protocol/smoke_test.go` -- proven harness import chain

### Secondary (MEDIUM confidence)
- [pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6) -- API documentation, Compiler, Schema.Validate
- [github.com/santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema) -- v6.0.2 release notes, draft support matrix

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - jsonschema/v6 verified via go list, MCP SDK v1.5.0 verified in go.mod
- Architecture: HIGH - all harness APIs verified in source, MCP SDK types verified in source
- Pitfalls: HIGH - all pitfalls derived from verified source code behavior

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable domain, no fast-moving dependencies)
