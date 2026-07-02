# Testing Patterns

**Analysis Date:** 2026-07-01

## Test Framework

**Runner:**
- Go's built-in `go test`. No external test runner.
- Run commands (CLAUDE.md "Go Development Commands", `Makefile`):
  ```bash
  go test ./...            # run all Go tests
  go test ./... -count=1   # uncached (CI uses this; .github/workflows/go-test.yml)
  make test                # == `vet` + `go test ./...` (Makefile:17-18)
  make vet                 # go vet ./... + the 7 project vettools (Makefile:55)
  ```
- `make test` depends on `make vet`, so the architectural gates run BEFORE the
  test binaries (`test: vet` in `Makefile:17`).

**Assertion Library:**
- `github.com/stretchr/testify v1.11.1` (`go.mod`). `require.*` (fail-fast) and
  `assert.*` (continue). Verified in use across **83** internal test files; top
  calls tree-wide: `require.NoError` (693), `assert.Equal` (575),
  `assert.Contains` (247), `assert.True` (198), `require.NotNil` (129).
- Plain `t.Fatalf`/`t.Errorf` are also used directly (e.g. golden-file mismatch
  reporting in `internal/semantic/extract/testutil/golden.go`, scenario-count
  guards in provider tests).

**Test Selectors:**
- No custom test-tag/marker DSL. Selection is by package path and `-run <regex>`:
  ```bash
  go test -count=1 ./internal/semantic/dataflow/... ./internal/daemon/...
  HELIX_BIN=/tmp/helix go test -run TestCLI_E2E_InBody ./internal/cli/
  ```
- Build tags gate platform/heavy paths, e.g. `//go:build !windows` on the
  real-binary E2E files (`internal/cli/cli_type_resolution_e2e_test.go:1`).
- `testing.Short()` guards the heaviest E2E (e.g. `TestE2E_LiveEditFiresPreciseDiff`
  in `internal/semantic/live/handler/handler_diff_e2e_test.go` skips under `-short`).

## Test File Organization

**Location Pattern (Go co-location convention):**
- Tests live beside the code they test as `<name>_test.go` in the SAME package
  directory — there is no separate `test/` tree. **590** `*_test.go` files across
  `internal/` + `cmd/` + `bench/` + `protocol/` + `api/` (verified 2026-07-01;
  429 under `internal/` alone).
- Package-internal (white-box) tests share the package (`package fuzzy`); external
  (black-box) tests use `package <name>_test` (e.g. `package cli_test` in the CLI
  E2E files, `package handler_test` in the live-handler tests).
- `export_test.go` files expose unexported test seams within a package without
  widening the public API (e.g. `internal/semantic/live/handler/export_test.go`
  exports `LastRecorderSnapshotForTest`).

**Naming:**
- Test files: `<name>_test.go`; E2E oracles: `*_e2e_test.go`
  (`internal/cli/cli_e2e_test.go`, `cli_type_resolution_e2e_test.go`,
  `cli_dataflow_inbody_e2e_test.go`, `cli_sec_e2e_test.go`).
- Test funcs: `TestFeature_Scenario` (`TestSweepExact`,
  `TestResolveTypeEdges_PositiveCommitsRealEdge`, `TestCLI_E2E_CTypeResolution`).
- Subtests via `t.Run(name, ...)` inside table loops.

**Directory shape (representative):**
```
internal/
  fuzzy/
    strategies.go
    strategies_test.go            # table-driven, testify
  semantic/extract/
    golang/
      provider.go
      provider_test.go            # iterates testdata scenarios
      stable_id_test.go
      testdata/<scenario>/before.go + expected.json
    testutil/golden.go            # shared golden-compare helper
  cli/
    cli_e2e_test.go               # HELIX_BIN-gated real-binary harness
    cli_type_resolution_e2e_test.go
  semantic/live/handler/
    handler.go
    export_test.go                # test seams
    handler_diff_e2e_test.go
```

## Test Structure

**Table-driven, from `internal/fuzzy/strategies_test.go` (real, verbatim shape):**
```go
func TestSweepExact(t *testing.T) {
	tests := []struct {
		name  string
		whole []string
		part  []string
		want  []int
	}{
		{"single hit", []string{"a", "b", "c", "d"}, []string{"b", "c"}, []int{1}},
		{"ambiguous 2 hits", []string{"x", "y", "z", "x", "y"}, []string{"x", "y"}, []int{0, 3}},
		{"no hit", []string{"a", "b", "c"}, []string{"z"}, nil},
		{"1-line at EOF (off-by-one)", []string{"a", "b", "c"}, []string{"c"}, []int{2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sweepExact(tt.whole, tt.part))
		})
	}
}
```

**Patterns:**
- A `[]struct{ name; inputs...; want }` slice, then `for _, tt := range tests`
  with `t.Run(tt.name, ...)`. Cases name their edge (off-by-one, ambiguity,
  empty input) rather than the happy path only.
- `require.*` for preconditions that must hold before the assertion is meaningful
  (a nil-check that would otherwise panic); `assert.*` for the value under test.
- E2E oracles assert the DIFFERENTIAL: the positive arm yields exactly one edge
  (asserted first), the negative arm yields zero — anti-vacuity
  (`internal/cli/cli_type_resolution_e2e_test.go` `cTypeSeed` carries both arms).

## Fixtures

**Go has no fixture-injection framework; setup is helper constructors + `t.Cleanup`.**

**Real-binary E2E fixture (`internal/cli/cli_e2e_test.go`):**
```go
func newE2EFixture(t *testing.T, runID string, daemonOpts ...sandbox.DaemonOption) *e2eFixture {
	t.Helper()
	helixBin := resolveHelixBin()          // HELIX_BIN env, else `helix` on PATH
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build -o helix ./cmd/helix'); skipping E2E oracle")
	}
	// stands up a sandbox + real daemon via internal/eval/sandbox.StartDaemon;
	// daemon reaped (h.Kill) and sandbox removed (sb.Cleanup) via t.Cleanup so
	// no orphan process survives a failed run.
	...
}
```
- `resolveHelixBin()` prefers `HELIX_BIN` (points at a freshly-built binary
  without polluting PATH), then falls back to `helix` on PATH; returns `""` → the
  caller `t.Skip`s. Same convention as the bench harness.
- Cleanup is registered with `t.Cleanup` (not `defer` in a fixture), so teardown
  runs even on subtest failure.

**Test seams instead of fixtures for DI (`export_test.go` pattern):**
- `internal/semantic/live/handler/export_test.go` exports
  `LastRecorderSnapshotForTest(h)` and `DiffSymbolsForTest(...)`; the handler
  captures its recorder snapshot into `h.lastRecorderSnapshot` at commit time
  (`handler.go:467-470`) purely so the E2E can observe it.
- `internal/daemon/daemon.go` exposes `sessionRunner`/`serveSession` seams (nil in
  production, stubbed in tests to bypass the MCP runtime); `SetEnrichFn`,
  `SetCollectCandidatePathsHook` are post-init wiring hooks tests toggle.

## Mocking

**Framework:** none. There is NO `gomock`, `mockery`, or `testify/mock` in
`go.mod` (verified — 0 matches). Tests run against real subsystems or hand-written
fakes/seams.

**Patterns:**
- **Real HTTP boundary via `net/http/httptest`** — the upgrade client's network
  path is tested against a real in-process server, not a mock
  (`internal/upgrade/api_test.go`, `upgrade_test.go`, `verify_test.go`:
  `httptest.NewServer(...)`; 12 internal files use `httptest`). This exercises the
  actual redirect/auth-strip/rate-limit/`Retry-After` logic.
- **Real daemon subprocess** for CLI E2E (`newE2EFixture` above) — the shipped
  binary is driven as a subprocess, never a fake CLI.
- **Interface seams** injected only where a real dependency is unavailable or
  non-deterministic (the `export_test.go` / `Set*Hook` seams above).

**What is NOT mocked:**
- The `helix` binary (real subprocess), the daemon, the semantic store (real
  `*Store` in `TestResolveTypeEdges_*`), tree-sitter extraction (real providers
  over `testdata/` fixtures), the HTTP upgrade path (`httptest`).

## Test Data & Fixtures

**Per-package `testdata/` directories** (Go's conventional, `go`-ignored fixture
dir). 47 `testdata/` dirs across `internal/` + `bench/` (verified). Real ones:
- **Per-language extractor goldens:** `internal/semantic/extract/<lang>/testdata/`
  for c, cpp, csharp, golang, java, kotlin, php, python, ruby, rust, typescript.
  Each scenario is a directory holding `before.<ext>` (+ optional `after.<ext>`)
  and an `expected.json` golden (e.g.
  `internal/semantic/extract/golang/testdata/function_basic/{before.go,expected.json}`;
  ~44 scenarios: `call_reference`, `generic_type_param`, `rename_churns_id`,
  `whitespace_edit_preserves_id`, ...).
- **Type-resolver goldens:** `internal/semantic/types/{golang,python,typescript}/testdata`.
- **Graph/PageRank goldens:** `internal/graph/testdata`, `internal/semantic/graph/testdata`,
  `internal/semantic/cluster/testdata` (`golden_*.txt`).
- **Lint analyzer fixtures:** `internal/lint/<gate>/testdata/` for the 7 vet gates
  (analysistest-style: source files that must or must not trigger the analyzer).
- **Bench fixtures:** `bench/languages/<lang>/testdata`, `bench/evaluators/*/testdata`,
  `bench/datasets/*/testdata`, `bench/container/testdata`, `bench/schema/testdata`.
- **Repo/skill fixtures:** `internal/skill/repomap/testdata`, `internal/upgrade/testdata`,
  `internal/semantic/{live,lspenrich,compact}/testdata`; Java LS fixtures under
  `testdata/fixtures/java/` (referenced by the CI warm-cache key).

**Shared golden helper (`internal/semantic/extract/testutil/golden.go`):**
- `NormalizeForGolden(ef)` produces deterministic JSON (sorts records by
  (file, range, kind, name), strips absolute paths to relative, two-space indent).
- `GoldenCompare(t, root, scenario, actual)` reads `testdata/<scenario>/expected.json`
  and diffs; `ListScenarios`/`FindBefore`/`FindAfter` discover scenarios by the
  presence of a `before.*` file.
- A dev-only `-update` flag (`var Update = flag.Bool("update", ...)`) regenerates
  the goldens from the current emit; **CI runs WITHOUT it**, so the goldens are the
  regression net.

## Coverage

**Requirements:** Not enforced. No `-cover`/`-coverprofile` in `Makefile` or any
`.github/workflows/*.yml` (verified — 0 matches). Coverage is a posture (dense
co-located tests + golden regression + real-binary E2E), not a numeric gate.

**View coverage locally:**
```bash
go test ./... -coverprofile=cover.out && go tool cover -func=cover.out
```

## Test Types

**Unit / white-box:** table-driven tests in-package over pure functions
(`internal/fuzzy/strategies_test.go` `sweepExact`/`sweepWhitespace`/`sweepIndentFlex`;
extractor stable-ID tests `internal/semantic/extract/golang/stable_id_test.go`).

**Golden regression:** deterministic-emit comparisons against committed
`expected.json` / `golden_*.txt` (extractor providers, PageRank, clustering).

**In-process integration:** real subsystem, no subprocess — e.g.
`TestResolveTypeEdges_PositiveCommitsRealEdge` runs the resolver against a real
`*Store` and asserts exactly one `RESOLVES_TO` edge; `TestE2E_LiveEditFiresPreciseDiff`
drives a real edit through `Handler.populateRecorderForFile → tryFullDiff →
diffSymbols → ComputeGraphRepair`.

**Real-binary E2E:** the `HELIX_BIN`-gated `internal/cli/*_e2e_test.go` oracles
drive the shipped `helix` binary as a subprocess against a live daemon
(`TestCLI_E2E_OneShot`, `TestCLI_DualRunParity`, `TestCLI_E2E_CTypeResolution`,
`TestCLI_E2E_InBodyDataFlow`/`InBodyMultiHop`). These prove the agent-facing CLI
surface end-to-end (parse → gRPC `StreamMCP` → daemon → result).

**Architectural gate tests:** the 7 `vet-*` singlecheckers (`cmd/vet-*` wrapping
`internal/lint/*`) run under `make vet` as part of `make test`; each has its own
`internal/lint/<gate>/*_test.go` + `testdata/` proving it flags the forbidden
import edge and passes the allowed ones.

**Milestone bench harness:** `bench/` (291 `.go` files) is a separate evaluator
stack (SWE-bench, Multi-SWE-bench, Terminal-Bench, aider-polyglot, repobench,
crosscodeeval), container-backed via Podman/Docker auto-detect. Its hermetic
smoke (`make bench-quick`, scripted agent, no API key, ≤90s/≤5m CI cap) is the only
bench path in PR-gating CI; the full `make bench` is nightly/on-demand only.

## Common Patterns

**Subprocess-with-cleanup:** stand up a real daemon/binary in a helper, register
teardown with `t.Cleanup` so orphans never survive a failed test
(`newE2EFixture`).

**Clean skips, not silent broken-test markers:** tool-availability gates skip with a reason
rather than masking a failure —
```go
if _, err := exec.LookPath("gopls"); err != nil {
	t.Skip("gopls not installed, skipping LS-backed behavioral chain oracle")
}
```
(`internal/cli/cli_e2e_test.go` `requireGoplsE2E`); the `HELIX_BIN` gate skips the
same way. A skipped LS test is "environment lacks the LS", never "known broken".

**Differential anti-vacuity:** a single fixture carries a positive arm and a
negative arm so an assertion can prove BOTH that the wanted edge appears AND that
an unwanted one does not (`cTypeSeed`: `struct Foo`-typed param → one `has_type`;
primitive param → zero).

**Determinism assertions:** re-run / re-index and assert byte-identical output
(sorted-before-range drivers; `TestResolveTypeEdges_Deterministic`; the golden
files themselves encode a canonical order).

**Uncached verification:** milestone audits re-run with `-count=1` against a
freshly-built binary to defeat the stale-binary / test-cache hazard
(`HELIX_BIN=<fresh> go test -count=1 ...`).

## Snapshot Testing

**Golden files ARE the snapshots** (no syrupy/`.ambr` equivalent — this is Go).
Two shapes:
- **JSON goldens:** `testdata/<scenario>/expected.json` (+ `expected_after.json`
  when an `after.<ext>` fixture exists), compared via
  `testutil.GoldenCompare`/`GoldenCompareAfter` after `NormalizeForGolden`
  canonicalizes the emit.
- **Text goldens:** `golden_*.txt` for deterministic numeric/graph output
  (`internal/graph/testdata/pagerank/golden_{uniform,personalized,tiebreak}.txt`;
  `internal/semantic/cluster/testdata/golden_{single_component,three_components,isolated_nodes}.txt`).
- Regeneration is the explicit dev-only `-update` flag on the extractor providers
  (`internal/semantic/extract/testutil/golden.go`); CI never passes it, so a diff
  is a hard failure.

## Language-Server Tests Organization

Helix's product IS an LSP orchestrator, so several E2E oracles need a real language
server on PATH and gate on it:
- **gopls** — the Go LSP-backed behavioral chains (`requireGoplsE2E`,
  `TestCLI_DualRunParity` `needsLS: true` rows for `go_to_definition` /
  `find_references`). Skips cleanly when gopls is absent.
- **jdtls** (Eclipse JDT.LS) — the Java integration path. CI installs a pinned
  jdtls (`JDTLS_VERSION: 1.57.0`, requires Java 21) via a self-contained wrapper
  and caches a warm workspace keyed on `testdata/fixtures/java/**`
  (`.github/workflows/go-test.yml`); `make bench-jdtls-warm` /
  `make clean-jdtls-cache` manage warm/cold runs locally.
- The 11-language type-resolution / dataflow E2E fixtures embed source inline
  (`cTypeSeed`) and drive them through the daemon's own extractor/index path
  (tree-sitter), not per-language external LSs, so they run without installing 11
  language servers.

## CI/Environment Handling

**Workflows (`.github/workflows/`):**
- `go-test.yml` — PR/push gate on `main`: installs jdtls + Java 21, runs
  `make vet` (go vet + the project vettools), `go test ./... -count=1`, the three
  drift gates (`helix-cligen --check`, `docgen --check`, `helix-refgen --check`),
  `make eval-quick` (in-process scripted-agent harness, ≤2m), plus warn-only
  hygiene steps. A grep gate forbids referencing the LLM judge in any workflow
  (EVAL-07).
- `bench.yml` — `bench-quick` hermetic smoke on PRs (≤5m, no secrets); full
  `bench` nightly/`workflow_dispatch` only. `permissions: {contents: read}`.
- `bench-mirror.yml` — publishes the cosign-signed GHCR bench-image mirror.
- `release.yml` — split-runner build (ubuntu-22.04 zig-cross for linux+windows,
  macos-14 native darwin) + merge-job cosign keyless attestation over all 6
  archives; per-target within-runner reproducibility (Pass-1 ≡ Pass-2 sha256).
- `codeql.yml` — CodeQL static analysis. `codespell.yml` — spelling
  (config in the codespell workflow's referenced settings, not a runtime concern).

**Environment gates in tests:**
- `HELIX_BIN` env → resolve/skip the real-binary E2E (`resolveHelixBin`).
- `exec.LookPath("gopls")` / jdtls presence → skip LS-backed oracles.
- `//go:build !windows` on the subprocess E2E files.
- `testing.Short()` short-circuits the heaviest live-handler E2E.
- `HELIX_TEST_LS_TIMEOUT` (CI sets `2m`) bounds cold LS startup.

---

*Testing map: 2026-07-01 — Go tree at HEAD.*
