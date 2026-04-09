# Phase 8: Advanced Testing - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Verify that profile filtering, mode visibility, worker pool concurrency, and error handling all behave correctly. This is the final phase of v1.1 — after this the integration testing milestone closes.

</domain>

<decisions>
## Implementation Decisions

### Profile/Mode Source of Truth
- **D-01:** Use checked-in golden-file pattern for profile/mode contract tests. Each profile/mode gets its own golden file under `testdata/profiles/` (e.g., `testdata/profiles/claude-code.read.tools.golden`, `testdata/profiles/full.admin.tools.golden`) containing sorted tool name lists.
- **D-02:** Contract tests compare actual runtime tool list (from daemon/profile resolver) against the golden file. Use `-update` flag (or `GOLDEN_UPDATE=1`) for intentional updates — diffs must be reviewable in PR.
- **D-03:** DO NOT compute expected tool lists from runtime YAML (`.serena/profiles/*.yaml`) in contract tests — that would use the same source of truth as the code under test, masking bad YAML changes.
- **D-04:** Separate YAML loader/parser unit tests are allowed to read YAML dynamically (inheritance, defaults, merge semantics, validation). These test the loader, not the contract.

**Rationale:** A broken YAML change can keep the count identical but swap wrong tools in. Golden files provide independent oracles — any profile behavior change requires explicit human review of the golden file diff.

### Concurrency Test Design
- **D-05:** Layered three-tier approach:
  1. **Scenario stress tests** — `t.Parallel()` subtests with different tools against shared daemon (mixed reads/writes, mode switches)
  2. **Hot-path stress tests** — Explicit goroutine fan-out targeting specific pool code paths (worker checkout/checkin, queue saturation, timeout propagation, shutdown mid-work)
  3. **Deterministic unit tests** — Use `testing/synctest` for small scheduler-sensitive pool logic where available
- **D-06:** All concurrency tests must run under `-race`. CI has 3 jobs: `go test ./... -race` (baseline), stress-heavy fan-out with elevated counts, optional synctest job.
- **D-07:** Beware loop-variable capture after `t.Parallel()` — use `go vet` diagnostics to catch.

### Error Path Coverage
- **D-08:** Three bands of error coverage by risk level:
  1. **Representative per category** — Shared error matrix covers common framework errors: no workspace activated, invalid args, missing file, symbol not found, permission denied, timeout, cancellation, provider unavailable. One tool per category (symbols, edit, fileops, diag, memory, workflow, profile).
  2. **Exhaustive per tool for destructive/state-mutating tools** — edit (all 6), rename_symbol, safe_delete_symbol, write_memory. Full error matrix per tool.
  3. **Thin smoke for low-risk read-only tools** — Once category handlers covered, read-only tools get minimal error assertions.
- **D-09:** Use Go table-driven test style for error matrices — one well-written harness, not copy-pasted cases.
- **D-10:** Assert on error type/code (not just message text) to avoid brittleness.

**Rationale (OWASP alignment):** Integrity-sensitive operations deserve stronger negative testing — OWASP testing guidance explicitly calls out trying to read, edit, or remove impacted information when validating integrity protections.

### Claude's Discretion
- Exact structure of error matrix table (columns, fields)
- Golden file format (one tool per line vs JSON array)
- Fan-out goroutine count for stress tests (50, 100, 500?)
- Whether to add a separate `make test-stress` target

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 6 + 7 Harness (reuse)
- `test/integration/harness.go` — StartTestDaemon, WaitForLS, PrepareFixture, requireLS, requireGopls
- `test/integration/helpers.go` — callTool, callToolExpectError, textContent
- `testdata/fixtures/go/` — Go fixture for error path tests

### Profile System
- `internal/profile/` — ProfileResolver, profile YAML loading, mode filtering
- `.serena/profiles/` — 5 profile YAML files (claude-code, codex, ide-assistant, ci-bot, full)
- 4 modes: read, edit, review, admin
- `internal/mcp/server.go` — ProfileFilterMiddleware

### Worker Pool
- `internal/kernel/lspool/pool.go` — Pool lifecycle, acquire/release, circuit breaker
- `internal/kernel/lspool/worker.go` — Worker process management

### External Standards
- Google testing guidance — test planning as risk/cost tradeoff
- OWASP integrity testing — negative tests for mutating operations
- Go `testing/synctest` package for deterministic concurrency tests

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `StartTestDaemon` with profile/mode options for contract tests
- `ProfileResolver` can enumerate tools for a given profile/mode combination
- Existing profile tests in `internal/daemon/daemon_integration_test.go` (TestE2EProfileConfigLayering)

### Established Patterns
- `//go:build integration` on all integration test files
- `callTool` / `callToolExpectError` helpers for assertion patterns
- One daemon per test file, subtests for individual scenarios

### Integration Points
- New test files: `profile_golden_test.go`, `mode_golden_test.go`, `concurrency_test.go`, `errors_test.go`
- New golden files: `testdata/profiles/{profile}.{mode}.tools.golden` (5 profiles × 4 modes = 20 files, though some combinations may be redundant)
- Potential addition to harness: helper to list tools visible to the current session

</code_context>

<specifics>
## Specific Ideas

- Golden files updated via explicit `-update` flag with mandatory human review of diffs
- Scenario stress + hot-path fan-out + synctest layers
- Table-driven error matrices with type/code assertions

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-advanced-testing*
*Context gathered: 2026-04-09*
