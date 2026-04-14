# Phase 20: Scenarios & Runtime - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Realistic multi-step agent workflows pass across diverse repository shapes, and the runtime survives stress and degraded conditions. This phase populates `test/oracle/scenario/` with fixture-driven workflow tests and creates runtime stress/degraded tests. LLM behavioral tests belong in Phase 21.

</domain>

<decisions>
## Implementation Decisions

### Fixture Matrix (SCEN-01)
- **D-01:** Polyglot monorepo fixture is a composite of existing Go + Python + TypeScript subdirs combined into one fixture. Reuses proven code, tests real cross-language workspace behavior. Minimal new files needed.
- **D-02:** Unsupported language fixture uses a fake `.xyz` extension — deterministic, cannot accidentally become supported, environment-independent. Must use an extension not present in Serena-Go's shipped language manifest. Assertions: workspace activation succeeds, LS-dependent tools fail cleanly and honestly, non-LS tools continue to work, no fake language identification.
- **D-03:** Name collision fixture uses both patterns combined — same symbol names AND same filenames across languages. Structure: `backend/config/main.go`, `worker/config/main.py`, `web/config/main.ts` each defining `Config`, `Handler`, `Parse`. Keeps fixture intentionally minimal. Tests both symbol cross-contamination and path-based disambiguation.
- **D-04:** Degraded capability fixture — reuse existing Go fixture but with LS intentionally unavailable (via test double injection, not environment manipulation). Tests that file-level tools work while LS-dependent tools fail gracefully.

### Scenario Workflow Depth (SCEN-02)
- **D-05:** Full-cycle agent workflows: activate → search → read → edit → verify (go vet/compile). Each fixture gets one representative full-cycle scenario exercising the complete agent workflow including edits.
- **D-06:** Multi-step scenarios use `PrepareFixture` to copy fixtures to temp dirs (edits mutate copies, not originals).

### Profile/Mode Testing (SCEN-04)
- **D-07:** Profile/mode scenarios run against Go + one other language (Python). Tests read-mode blocks edits, admin grants all tools, switching updates visibility. Two-language coverage catches language-specific mode filtering bugs.

### Runtime Stress (RUNT-01)
- **D-08:** Tiered stress testing — two layers:
  - **Unit layer:** `testing/synctest` for deterministic circuit breaker/TTL/backoff logic tests
  - **Integration layer:** Real concurrent goroutines calling tools simultaneously against a real daemon for pool stress, share-until-dirty, pressure eviction
- **D-09:** Worker pool stress tests fan out N goroutines, verify circuit breaker trips under crashy workers, and confirm share-until-dirty semantics under concurrent edits.

### Clean Shutdown (RUNT-02)
- **D-10:** In-flight work + signal approach — start tool calls, send cancel/SIGTERM mid-flight, assert graceful drain. Verify no goroutine leaks (`runtime.NumGoroutine` before/after), no stuck LS subprocesses (`os.Process` checks).

### Degraded Mode Injection (RUNT-03)
- **D-11:** Interface-level test doubles for fault injection — inject failing implementations at daemon construction time. Clean, no process-level hacks. Tests the degraded startup path directly.
- **D-12:** All three subsystems tested independently:
  - **LS failure:** LS installer returns error / LS binary crashes on start. Symbol tools report honestly, file ops still work.
  - **Memory store failure:** Memory init fails. Memory tools report error, other tools unaffected.
  - **Skill init failure:** A skill's `Init()` returns error. Its tools are unavailable, other skills work.

### Deferred Error Categories (from Phase 19)
- **D-13:** Phase 19 deferred 3 CONT-03 error categories (timeout, circuit_open, unsupported) to Phase 20. These should be covered by runtime stress tests (timeout via deadline injection, circuit_open via crashy worker trigger, unsupported via the `.xyz` fixture).

### Claude's Discretion
- Test file organization within `test/oracle/scenario/` (one file per fixture vs one per requirement)
- Whether runtime stress tests live in `test/oracle/scenario/` or a separate `test/oracle/runtime/` package
- Specific concurrency parameters (N goroutines, timeout durations, retry counts)
- How to structure the degraded-mode test doubles (embedded in test file vs shared test helper)
- Whether synctest unit tests belong in `internal/kernel/lspool/` or in the oracle layer

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Harness Infrastructure (Phase 18)
- `test/harness/runner.go` — Runner, StartRunner, RunnerOptions, NewHTTPSession, ProjectRoot
- `test/harness/tools.go` — CallTool, TextContent, CallToolExpectError, ListSessionTools
- `test/harness/fixture.go` — PrepareFixture, RequireGopls
- `test/harness/golden.go` — AssertGolden

### Existing Fixtures
- `testdata/fixtures/go/` — Go fixture with main.go, pkg/ subdirectory
- `testdata/fixtures/python/` — Python fixture
- `testdata/fixtures/typescript/` — TypeScript fixture
- `testdata/fixtures/rust/` — Rust fixture
- `testdata/fixtures/java/` — Java fixture

### Runtime Internals
- `internal/kernel/lspool/circuit.go` — CircuitBreaker with decorrelated jitter backoff, restart budget
- `internal/kernel/lspool/pressure.go` — PressureLevel enum, platform-aware memory pressure detection
- `internal/kernel/lspool/pool.go` — Worker pool with share-until-dirty, adaptive TTL
- `internal/degrade/` — Degraded mode budget configuration
- `internal/daemon/daemon.go` — Daemon bootstrap, fail-fast core / degraded-optional startup

### Prior Phase Context
- `.planning/phases/19-protocol-contract-oracles/19-CONTEXT.md` — D-10 oracle authority, D-11 deterministic heuristics only
- `test/oracle/contract/errors_test.go` — TODO(phase-20) for timeout, circuit_open, unsupported error categories

### Existing Integration Tests (parallel safety net, not replaced)
- `test/integration/concurrency_test.go` — Existing concurrency tests
- `test/integration/errors_test.go` — Existing error path tests
- `test/integration/profile_test.go` — Existing profile tests

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `test/harness/` package: Full runner lifecycle, tool calling, golden file comparison, fixture preparation
- `testdata/fixtures/{go,python,typescript,rust,java}/`: Five language fixtures ready for scenario testing
- `internal/kernel/lspool/circuit.go`: CircuitBreaker already has exported API for testing
- `internal/kernel/lspool/pressure.go`: PressureLevel constants and detection functions

### Established Patterns
- Build tags `//go:build integration || llm || llmjudge` for oracle tests
- `harness.StartRunner` + `defer runner.Stop()` lifecycle pattern
- `harness.PrepareFixture(t, "go")` for mutable fixture copies
- `t.Run` subtests with `t.Parallel()` where no shared state
- `testify/require` for assertions

### Integration Points
- New fixtures created in `testdata/fixtures/{polyglot,unsupported,collision}/`
- Scenario tests in `test/oracle/scenario/` (doc.go already exists)
- Runtime tests may need new test doubles / interfaces in daemon constructor
- Deferred error category goldens connect back to `test/oracle/contract/testdata/golden/errors/`

</code_context>

<specifics>
## Specific Ideas

- Unsupported language fixture rule: extension must not be present in shipped Serena-Go language manifest AND not claimed by upstream Serena's documented language support list
- Name collision fixture structure: `backend/config/main.go`, `worker/config/main.py`, `web/config/main.ts` each defining `Config`, `Handler`, `Parse`
- Shutdown test must check both `runtime.NumGoroutine` and `os.Process` for stuck LS subprocesses

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 20-scenarios-runtime*
*Context gathered: 2026-04-11*
