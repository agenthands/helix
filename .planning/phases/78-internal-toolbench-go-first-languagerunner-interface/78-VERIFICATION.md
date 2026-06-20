---
phase: 78-internal-toolbench-go-first-languagerunner-interface
verified: 2026-06-17T17:18:11Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
---

# Phase 78: Internal ToolBench (Go-first) + LanguageRunner Interface — Verification Report

**Phase Goal:** The deterministic ground truth for "Helix tools work" — 10 capability test classes, all 10 covered on Go, and a common `LanguageRunner` interface ready for the remaining 7 languages.
**Verified:** 2026-06-17T17:18:11Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `CAPABILITIES.md` documents 10 capability classes; each has ≥1 deterministic Go test | ✓ VERIFIED | `CAPABILITIES.md` (132 lines) has 10 numbered sections, each naming the capability enum, Helix tool(s), and Go fixture dir. All 10 fixtures exist under `go/` with the matching `capability` field. |
| 2 | ToolBench-Go full run passes; 10/10 capabilities have a Go fixture; `go/runner.go` wraps `go test ./... -json`; coverage reported per language | ✓ VERIFIED | `helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted` → `10/10 cells succeeded` exit 0. `GoRunner.RunTests` runs `exec.CommandContext("go","test","./...","-json")` (runner.go:66). `Coverage()` reports Go 10/10 (`TestCoverageGoIsTenOfTen` PASS). |
| 3 | `LanguageRunner` interface (Detect, Setup, RunTests, Capabilities) defined in `bench/languages/`; `go vet` + conformance test passes for Go | ✓ VERIFIED | `runner.go:60-78` defines all 4 methods. `var _ languages.LanguageRunner = (*GoRunner)(nil)` compiles (go/runner.go:27). `go vet ./...` exit 0. `go test ./bench/languages/go/` all PASS. |
| 4 | Task IDs use `IT-go-<capability>-<n>`; zero `T-67-*` collision; `PHASE67_CROSSWALK.md` documents inspiration mapping | ✓ VERIFIED | All 10 task.json ids match `^IT-go-`; none match `^T-67-` (`TestNamespaceIsITGoZeroT67` PASS). `PHASE67_CROSSWALK.md` (46 lines) maps T-67-* → IT-go-* (12 T-67 refs, 11 IT-go refs). |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `bench/languages/runner.go` | LanguageRunner interface + Capability enum + TestOutcome/TestResult | ✓ VERIFIED | 78 lines; interface with 4 methods, 10 capability constants, structured outcome types |
| `bench/languages/registry.go` | Register/RunnerFor Caddy-style registry | ✓ VERIFIED | 36 lines; RWMutex map keyed by benchmark/lang; nil = D-10 fallback signal |
| `bench/languages/go/runner.go` | GoRunner; `go test -json` wrapper + parser | ✓ VERIFIED | 141 lines; init() registers, test2json parser, exit-code-as-gate, compile-fail handling |
| `bench/languages/go/runner_test.go` | conformance + behavioral tests | ✓ VERIFIED | conformance assert + Detect/RunTests-pass/fail/compile-fail/fallback tests all PASS |
| `bench/languages/coverage.go` | per-language declared∩covered aggregator | ✓ VERIFIED | 97 lines; reads `capability` field (not id — non-vacuous), gap detection |
| `bench/datasets/internal-toolbench/CAPABILITIES.md` | 10-class doc (≥30 lines) | ✓ VERIFIED | 132 lines; all 10 classes documented with tool + fixture |
| `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` | inspiration map (contains IT-go) | ✓ VERIFIED | 46 lines; T-67-* → IT-go-* inspiration-only mapping |
| 10× `go/IT-go-*/task.json` | id=IT-go-*, capability=<enum>, benchmark, language | ✓ VERIFIED | All 10 present; distinct capabilities covering the full set |
| `bench/runtime/store_isolation_test.go` | --parallel store-isolation test | ✓ VERIFIED | asserts distinct `.helix/semantic.duckdb` + RejectedForeignPid==0; PASS (10.45s) with HELIX_BIN set |
| `internal/eval/sandbox/sandbox.go` | WithWorkingDir DaemonOption | ✓ VERIFIED | sandbox.go:243-248 additive option setting cmd.Dir |
| `bench/runtime/matrix.go` / `cell.go` | Cell.Language, 4-way ExpandMatrix, RunnerFor dispatch, StoreOptIn | ✓ VERIFIED | Language field + 4-way product (matrix.go:144), RunnerFor dispatch (cell.go:350), StoreOptIn derive + WithWorkingDir wiring |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `go/runner.go` | `languages.LanguageRunner` | compile-time conformance assert | ✓ WIRED | `var _ languages.LanguageRunner = (*GoRunner)(nil)` compiles + go vet clean |
| `cell.go RunCell` | `languages.RunnerFor(benchmark, lang)` | RunTests when non-nil, runVerify fallback otherwise | ✓ WIRED | cell.go:350 dispatch; `TestRunnerForFallbackSignal` PASS |
| `coverage.go` | task.json `capability` × `GoRunner.Capabilities()` | declared∩covered = 10/10 | ✓ WIRED | `TestCoverageGoIsTenOfTen` PASS; `TestCoverageDetectsGap` proves non-vacuous |
| `incremental-update task.json` | `StoreOptIn=true → enabled:true + WithWorkingDir` | derive-from-capability | ✓ WIRED | `deriveStoreOptIn` (matrix.go:280) + cell.go:271 WithWorkingDir on StoreOptIn |
| each fixture `scripted_agent.yaml` | capability's Helix MCP tool | explicit `tool:` step | ✓ WIRED | call-graph→get_call_hierarchy(incoming); failure→expect_error; incremental→refresh_semantic_graph(wait_for_lsp) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `Coverage()` report | covered capabilities | reads on-disk `task.json` `capability` field per fixture | Yes — 10 real fixtures on disk, decoded from JSON | ✓ FLOWING |
| corpus run cells | per-cell TestOutcome | `GoRunner.RunTests` runs real `go test` subprocess in each fixture repo | Yes — 10/10 cells executed real `go test`, exit 0 | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full Go corpus run 10/10 | `go run ./cmd/helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted --helix-bin=$(pwd)/helix` | `10/10 cells succeeded`, exit 0 | ✓ PASS |
| Coverage 10/10 + namespace | `go test ./bench/languages/ -run 'Coverage\|Namespace\|Docs' -v` | 4 tests PASS | ✓ PASS |
| Go runner conformance + behavior | `go test ./bench/languages/go/ -v` | 6 tests PASS (conformance, Detect, RunTests pass/fail/compile-fail, fallback) | ✓ PASS |
| Store-isolation under --parallel=2 | `HELIX_BIN=$(pwd)/helix go test ./bench/runtime/ -run StoreIsolationParallel -v` | PASS (10.45s) — distinct duckdb, RejectedForeignPid==0 | ✓ PASS |
| All bench tests green | `go test ./bench/... -count=1` | all ok | ✓ PASS |
| Whole-tree vet clean | `go vet ./...` | exit 0 | ✓ PASS |
| helix binary builds | `go build -o helix ./cmd/helix` | exit 0 | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` declared for this phase; verification is via the corpus run + Go test suite above. Not applicable.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| TOOLBENCH-01 | 78-03, 78-04, 78-05 | 10 capability test classes + CAPABILITIES.md | ✓ SATISFIED | 10 fixtures + 132-line CAPABILITIES.md documenting all 10 classes |
| TOOLBENCH-02 | 78-01–78-05 | Go tier-1: all 10 fixtures, runner wraps `go test -json`, full run passes | ✓ SATISFIED | 10/10 corpus run exit 0; RunTests wraps go test -json; coverage 10/10 |
| TOOLBENCH-10 | 78-01 | LanguageRunner interface (Detect/Setup/RunTests/Capabilities); go vet + conformance | ✓ SATISFIED | Interface defined; Go conformance test PASS; go vet clean. (Acceptance mentions "all 8 languages" — only Go built this phase per the goal's "ready for the remaining 7"; conformance proven for Go, interface ready for the rest. Consistent with phase scope.) |

No orphaned requirements: TOOLBENCH-03..09 are explicitly unchecked and scoped to Phase 85 (forward) in REQUIREMENTS.md.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | none | — | No TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER in any phase-modified file under `bench/languages/`, `bench/datasets/internal-toolbench/`, `bench/runtime/store_isolation_test.go`, or `internal/eval/sandbox/sandbox.go` |

### Human Verification Required

None blocking. The one Manual-Only Verification listed in 78-VALIDATION.md (prose accuracy of CAPABILITIES.md / PHASE67_CROSSWALK.md) is doc-quality review; both docs carry automated existence/structure checks (`TestDocsExist`, namespace test) which pass, and inspection confirms all 10 classes are described with matching tools/fixtures and the crosswalk is inspiration-only. No graded behavior depends on human verification — all 4 success criteria have passing automated proofs.

### Out-of-Scope Note (not counted against this phase)

`go test ./...` surfaces two pre-existing failures in `test/bench/` — `TestBenchToolsManifestMatchesRegistry` and `TestToolDescriptionsGoldenFile` (MCP tool-registry drift). Confirmed these live in `test/bench/` (the tool-registry test tree), NOT in `bench/` (the corpus tree this phase builds). They are unrelated to phase 78 and already logged in `.planning/deferred-items.md`. `go test ./bench/...` is fully green.

### Gaps Summary

No gaps. All 4 success criteria are verified by passing automated proofs against the actual codebase:
- C1: 10 documented + fixtured capability classes (CAPABILITIES.md + 10 fixtures).
- C2: full Go corpus run `10/10 cells succeeded` (exit 0); `RunTests` wraps `go test ./... -json`; coverage 10/10.
- C3: `LanguageRunner` interface with all 4 methods, Go conformance test + `go vet ./...` clean.
- C4: every id `^IT-go-`, zero `^T-67-` collision, crosswalk present.

The interface is genuinely "ready for the remaining 7 languages" (registry + closed Capability enum + nil-fallback contract), with Go as the proven first-class reference tier. Store-isolation under `--parallel=2` is proven (distinct duckdb, no foreign-PID rejection).

---

_Verified: 2026-06-17T17:18:11Z_
_Verifier: Claude (gsd-verifier)_
