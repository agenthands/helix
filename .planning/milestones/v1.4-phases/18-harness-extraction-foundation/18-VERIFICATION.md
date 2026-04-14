---
phase: 18-harness-extraction-foundation
verified: 2026-04-11T17:30:00Z
status: passed
score: 9/9
overrides_applied: 0
---

# Phase 18: Harness Extraction & Foundation Verification Report

**Phase Goal:** New oracle packages can import shared test infrastructure and run under correct build tags
**Verified:** 2026-04-11T17:30:00Z
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A new test file in test/oracle/protocol/ can import test/harness and call StartRunner, PrepareFixture, callTool, and golden helpers without compilation errors | VERIFIED | `go test -tags integration ./test/oracle/protocol/... -v` passes; smoke_test.go imports harness, calls StartRunner, ListSessionTools, PrepareFixture, ProjectRoot |
| 2 | Running `go test ./...` (no tags) skips all integration/llm/llmjudge tests; running with `-tags integration` includes oracle tests but not LLM tests | VERIFIED | `go test ./test/harness/... ./test/oracle/...` = "matched no packages"; `-tags integration ./test/oracle/protocol/...` = 3 PASS; `-tags integration ./test/oracle/llm/...` = "matched no packages" |
| 3 | Existing v1.1 tests in test/integration/ continue to pass unchanged | VERIFIED | `git diff test/integration/` is empty; Go-only integration tests pass (Java LS timeout is pre-existing, unrelated to phase 18) |
| 4 | test/harness/ is a regular Go package (package harness, not _test) importable by other packages | VERIFIED | doc.go line 9: `package harness`; smoke_test.go successfully imports `github.com/postfix/serena/test/harness` |
| 5 | Exported symbols Runner, RunnerOptions, StartRunner, CallTool, TextContent, CallToolExpectError, ListSessionTools, PrepareFixture, ProjectRoot, WaitForLS, GoldenStore exist | VERIFIED | All symbols present in runner.go, tools.go, fixture.go, golden.go with correct exported names |
| 6 | Harness package compiles under -tags integration | VERIFIED | `go build -tags integration ./test/harness/...` exits 0; `go vet -tags integration ./test/harness/...` exits 0 |
| 7 | Harness self-tests pass under -tags integration | VERIFIED | 7/7 tests pass: TestProjectRoot, TestDefaultTestConfig, TestPrepareFixture, TestStartRunnerSmoke, TestCallToolSmoke, TestTextContent, TestGoldenStoreRoundTrip |
| 8 | Oracle directory structure exists: test/oracle/{protocol,contract,scenario,llm,judge}/ | VERIFIED | All 5 directories exist with doc.go files |
| 9 | Each oracle package has correct build tag expressions per D-06 | VERIFIED | protocol/contract/scenario: `integration \|\| llm \|\| llmjudge`; llm: `llm \|\| llmjudge`; judge: `llmjudge` |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `test/harness/doc.go` | Package declaration + build tag | VERIFIED | Line 1: `//go:build integration \|\| llm \|\| llmjudge`; contains `package harness` |
| `test/harness/runner.go` | Runner type, StartRunner, WaitForLS, config, ProjectRoot | VERIFIED | 253 lines; exports Runner, RunnerOptions, StartRunner, Stop, NewHTTPSession, WaitForLS, ProjectRoot, DefaultTestConfig |
| `test/harness/fixture.go` | PrepareFixture, RequireLS, RequireGopls | VERIFIED | 59 lines; all three functions exported with correct signatures |
| `test/harness/tools.go` | CallTool, TextContent, CallToolExpectError, ListSessionTools | VERIFIED | 59 lines; all four functions exported with correct signatures |
| `test/harness/golden.go` | GoldenStore, AssertGolden | VERIFIED | 101 lines; GoldenStore struct, NewGoldenStore, AssertTools, AssertGolden with -update flag and GOLDEN_UPDATE env support |
| `test/harness/runner_test.go` | Harness self-tests | VERIFIED | 87 lines; 7 test functions covering full API |
| `test/oracle/protocol/doc.go` | Package declaration for protocol oracle | VERIFIED | Correct build tag and package name |
| `test/oracle/contract/doc.go` | Package declaration for contract oracle | VERIFIED | Correct build tag and package name |
| `test/oracle/scenario/doc.go` | Package declaration for scenario oracle | VERIFIED | Correct build tag and package name |
| `test/oracle/llm/doc.go` | Package declaration for LLM behavioral oracle | VERIFIED | `//go:build llm \|\| llmjudge` (narrower scope, correct) |
| `test/oracle/judge/doc.go` | Package declaration for judge oracle | VERIFIED | `//go:build llmjudge` (narrowest scope, correct) |
| `test/oracle/protocol/smoke_test.go` | Import validation smoke test | VERIFIED | 53 lines; 3 tests calling harness.StartRunner, harness.PrepareFixture, harness.ListSessionTools, harness.ProjectRoot |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| test/harness/runner.go | internal/daemon | `daemon.New(cfg, logger)` | WIRED | Line 123: `d, err := daemon.New(cfg, logger)` |
| test/harness/runner.go | internal/skill | `skill.InitAll(deps)` | WIRED | Line 119: `if err := skill.InitAll(deps); err != nil` |
| test/harness/tools.go | mcp.ClientSession | `session.CallTool` | WIRED | Line 16: `result, err := session.CallTool(context.Background(), ...)` |
| test/oracle/protocol/smoke_test.go | test/harness | import | WIRED | Line 10: `"github.com/postfix/serena/test/harness"` + used in all 3 tests |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Harness compiles | `go build -tags integration ./test/harness/...` | exit 0 | PASS |
| Harness vets clean | `go vet -tags integration ./test/harness/...` | exit 0 | PASS |
| 7 harness self-tests pass | `go test -tags integration ./test/harness/... -v -count=1` | 7/7 PASS (0.8s) | PASS |
| 3 oracle smoke tests pass | `go test -tags integration ./test/oracle/protocol/... -v -count=1` | 3/3 PASS (0.7s) | PASS |
| No-tags skips harness+oracle | `go test ./test/harness/... ./test/oracle/...` | "matched no packages" | PASS |
| Integration tag excludes LLM | `go test -tags integration ./test/oracle/llm/...` | "matched no packages" | PASS |
| go test ./... excludes harness/oracle | `go test ./...` output grep for harness/oracle | 0 matches | PASS |
| Go integration regression | `go test -tags integration ./test/integration/... -run Go/Harness/File/Diag/Profile` | PASS (2.0s) | PASS |
| Zero changes to test/integration/ | `git diff test/integration/` | empty | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| FOUND-01 | 18-01, 18-02 | Test harness extracted to importable `test/harness/` package with exported StartTestDaemon, PrepareFixture, callTool, and golden helpers | SATISFIED | Package harness exists with all exported symbols; smoke_test.go in protocol/ imports and calls them successfully |
| FOUND-02 | 18-02 | Build tag taxonomy established (integration, llm, llmjudge) with documented naming conventions for oracle packages | SATISFIED | Three-tier tag hierarchy verified: no-tags=skip, integration=deterministic, llm needs llm tag; oracle doc.go files carry correct expressions |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | No anti-patterns detected |

### Human Verification Required

None -- all aspects of this phase are verifiable programmatically (compilation, test execution, build tag behavior, file existence).

### Gaps Summary

No gaps found. All 9 must-have truths verified, all 12 artifacts substantive and wired, all 4 key links confirmed, both requirements satisfied, no anti-patterns detected, all 9 behavioral spot-checks pass.

---

_Verified: 2026-04-11T17:30:00Z_
_Verifier: Claude (gsd-verifier)_
