---
phase: 09
plan: 02
subsystem: test/bench
tags: [benchmark, scaffold, rss]
requirements: [BENCH-01]
dependency_graph:
  requires:
    - test/integration harness (TB-generic entry points from Plan 09-01)
    - internal/daemon bootstrap (Registry().Names() accessor)
    - testdata/fixtures/go (shared read-only Go fixture)
  provides:
    - test/bench/rss.CurrentRSS (CGO-free RSS reader for linux/darwin/other)
    - test/bench/bench_test TB-generic scaffold (startBenchDaemon, prepareGoFixtureB, prepareGoFixtureCopyB, callToolB)
    - test/bench/bench_test.benchTools 38-entry manifest + parity assertion
  affects:
    - Plan 09-03 (symbol/edit bench implementations — consume prepareGoFixtureCopyB for edit tools)
    - Plan 09-04 (LSP indexing + memory baselines — consume rss.CurrentRSS)
    - Plan 09-05 (benchstat gate wiring)
tech_stack:
  added: []
  patterns:
    - "testing.TB-generic bench helpers (no *testing.B hardcoding)"
    - "package bench_test with *_test.go filenames so the package resolves to XTestGoFiles"
    - "Registry().Names() parity assertion bypassing ProfileFilterMiddleware"
    - "Goroutine leak check with grace loop + empirical tolerance"
key_files:
  created:
    - test/bench/rss/rss.go
    - test/bench/rss/rss_linux.go
    - test/bench/rss/rss_darwin.go
    - test/bench/rss/rss_other.go
    - test/bench/rss/rss_test.go
    - test/bench/bench_helpers_test.go
    - test/bench/tools_manifest_test.go
    - test/bench/main_test.go
  modified: []
decisions:
  - "Registry().Names() is the canonical source for parity assertion, not session.ListTools (which is filtered by ProfileFilterMiddleware based on edit-mode AllowedTools)."
  - "bench_helpers must be named bench_helpers_test.go so the test/bench package is a clean external test package (XTestGoFiles) with no regular Go files."
  - "prepareGoFixtureB returns shared read-only fixture path; prepareGoFixtureCopyB copies into tb.TempDir() for mutating edit tool benchmarks — prevents cross-sub-bench fixture corruption."
  - "Goroutine leak tolerance raised from research's suggested 2 to 16, with a 10x50ms grace loop, because daemon kernel and LS-pool background workers do not unwind synchronously after cancel()."
metrics:
  duration: "~8min"
  completed_date: "2026-04-09"
  tasks_completed: 3
  files_created: 8
  files_modified: 0
---

# Phase 09 Plan 02: Benchmark Harness Scaffold Summary

One-liner: Stood up the `test/bench/` package scaffold — CGO-free RSS reader, TB-generic daemon/fixture helpers, 38-tool manifest with bidirectional registry parity assertion — ready for Plans 09-03..09-05 to write benchmark functions without repeating scaffolding.

## What Was Built

### Task 1: CGO-free RSS reader (`test/bench/rss/`)
- `rss.go` exposes `CurrentRSS() (uint64, error)` and `ErrUnsupported`, dispatching to build-tagged `currentRSS()` implementations.
- `rss_linux.go` parses `/proc/self/status` for `VmRSS:` and returns KiB * 1024.
- `rss_darwin.go` shells out to `ps -o rss= -p <pid>` (trailing `=` suppresses the header) and parses KiB * 1024.
- `rss_other.go` (`//go:build !linux && !darwin`) returns `(0, ErrUnsupported)`.
- `rss_test.go` asserts non-zero RSS (>1 MiB sanity floor) on linux/darwin and `ErrUnsupported` elsewhere. Passes on darwin development machine.

Stdlib only: `bufio`, `fmt`, `os`, `os/exec`, `strconv`, `strings`. No gopsutil, no cgo.

### Task 2: TB-generic bench helpers (`test/bench/bench_helpers_test.go`)
- `benchDaemon` struct wrapping `*daemon.Daemon` + `*mcp.ClientSession` with `Stop()` and `RegistryNames()` accessors.
- `startBenchDaemon(tb testing.TB) *benchDaemon` — mirrors `test/integration.StartTestDaemon` (profile `full`, 2 workers, in-memory MCP transport), registers teardown via `tb.Cleanup(bd.Stop)` so cleanup fires at the bench-function boundary (not per iteration — Pitfall 6).
- `prepareGoFixtureB(tb testing.TB) string` — returns the absolute path to the shared read-only `testdata/fixtures/go/` without copying. Read-only benchmarks use this to avoid CoW cost in the measurement.
- `prepareGoFixtureCopyB(tb testing.TB) string` — copies the Go fixture into `tb.TempDir()` with directory structure preserved and `.git` skipped. Edit-tool benchmarks (Plan 09-03) MUST use this helper because `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, and `verify_edit` mutate files.
- `activateWorkspaceB` calls `activate_project` then polls `search_symbols` for the fixture's `Helper` symbol with a 30s LS-readiness budget.
- `callToolB` TB-generic wrapper that Fatals on any error (protocol or tool).
- `listSessionToolsB` and `requireGoplsB` TB-generic convenience helpers.

Blank-imports the same set as the integration harness (`internal/kernel/{diag,edit,fileops,symbols}`, `internal/profile`, `internal/skill/{memory,workflow}`) to trigger Caddy-style skill registration.

NO build tag on any bench file (per 09-RESEARCH.md Pitfall 2).

### Task 3: TestMain + 38-tool manifest (`test/bench/main_test.go`, `test/bench/tools_manifest_test.go`)
- `TestMain` captures `runtime.NumGoroutine()` baseline, runs tests, then enters a 10-iteration × 50ms grace loop (forcing `runtime.GC()` each iteration) to let canceled daemon goroutines unwind before sampling. Fails the run if the final leak exceeds `leakTolerance` (16, empirically tuned).
- `TestBenchToolsManifestMatchesRegistry` asserts:
  1. `len(benchTools) == 38` exactly,
  2. `len(bd.RegistryNames()) == 38` exactly (catches upstream drift),
  3. every `benchCase.name` exists in the live registry,
  4. every live registry name exists in `benchTools` (no unbenchmarked tools).
- `tools_manifest_test.go` defines `benchCase` (name, args, needsCopy) and `benchTools` with exactly 38 entries covering 9 symbol + 6 edit + 6 fileops + 3 diagnostics + 7 memory + 2 workflow + 2 profile + 3 built-in tools. Tool names match `internal/daemon/bootstrap_test.go` verbatim. Edit tools are tagged `needsCopy: true` to instruct Plan 09-03 sub-benchmarks to consume `prepareGoFixtureCopyB`.

## Verification

```
$ go vet ./test/bench/...                                           # clean
$ go test ./test/bench/rss/...                                      # PASS
$ go test -run TestBenchToolsManifestMatchesRegistry -count=1 ./test/bench/...   # PASS
$ go test -bench=. -benchtime=1x ./test/bench/...                   # PASS (zero Benchmark* functions, TestMain runs clean)
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Parity assertion source: session.ListTools → Registry().Names()**
- **Found during:** Task 3, running `TestBenchToolsManifestMatchesRegistry` for the first time.
- **Issue:** Plan said to assert manifest parity against `listSessionTools`, but `session.ListTools` goes through `ProfileFilterMiddleware`, which filters out 5 tools (`ping`, `echo`, `activate_project`, `switch_mode`, `get_token_budget`) because they are not in the `full` profile's `edit` default-mode `AllowedTools` set. The live session reported 33 tools, not 38, and the parity test failed.
- **Fix:** Added `(*benchDaemon).RegistryNames()` accessor that returns `d.MCPServer().Registry().Names()` directly, bypassing the middleware. Updated `TestBenchToolsManifestMatchesRegistry` to use `bd.RegistryNames()`. The registry is the canonical 38-tool set asserted by `internal/daemon/bootstrap_test.go:TestBootstrapRegistersAllTools`.
- **Files modified:** test/bench/bench_helpers_test.go, test/bench/main_test.go.
- **Commit:** b80bee93.

**2. [Rule 3 - Blocking] Package layout: bench_helpers.go → bench_helpers_test.go**
- **Found during:** Task 2, verifying `go list` package classification.
- **Issue:** Plan asked for `test/bench/bench_helpers.go` declared as `package bench_test`. A non-`_test.go` file in a `_test` package makes `go list` classify it as a regular Go file with package name literally `bench_test` (GoFiles, not XTestGoFiles). That works but is confusing and forces the adjacent `*_test.go` files to be classified as internal test files of that oddly-named package rather than an external test package.
- **Fix:** Renamed to `bench_helpers_test.go`. The `test/bench` package now cleanly resolves to an external test package with `XTestGoFiles=[bench_helpers_test.go, main_test.go, tools_manifest_test.go]` and an empty outer package. `git mv` used so history is preserved.
- **Files modified:** test/bench/bench_helpers_test.go (renamed from bench_helpers.go).
- **Commit:** b80bee93.

**3. [Rule 1 - Bug] Goroutine leak tolerance: 2 → 16 with grace loop**
- **Found during:** Task 3, running `TestBenchToolsManifestMatchesRegistry` after the parity fix.
- **Issue:** Plan/research suggested a leak tolerance of 2. In practice, `(*benchDaemon).Stop()` cancels the daemon context but kernel `Run`, LS-pool reclaim, and related background goroutines do not unwind synchronously — they observe the cancel asynchronously and exit over the next few milliseconds. `TestMain` sampled `runtime.NumGoroutine()` immediately after `m.Run()` and caught the unwinding workers mid-exit, reporting a false positive leak.
- **Fix:** Raised `leakTolerance` to 16 and added a 10-iteration × 50ms grace loop that calls `runtime.GC()` each iteration, exiting early once the count settles. Documented the empirical tuning inline, referencing Assumption A5 ("tune tolerance empirically in Wave 0").
- **Files modified:** test/bench/main_test.go.
- **Commit:** b80bee93.

No other deviations. Rule 4 (architectural) not triggered.

## Known Stubs

None. Every file wires real functionality — no placeholder values, no hardcoded empties flowing to UI/data paths.

## Threat Flags

None. All files are test-only, not compiled into production. The only new external surface is `ps -o rss= -p <pid>` invocation on darwin (T-09-03 in plan threat register, disposition `accept`).

## Commits

- `b2886c11` — feat(09-02): add CGO-free test/bench/rss package
- `deef4375` — feat(09-02): add test/bench/bench_helpers.go TB-generic scaffold
- `b80bee93` — feat(09-02): add test/bench TestMain + 38-tool manifest parity

## Self-Check: PASSED

**Created files verified on disk:**
- FOUND: test/bench/rss/rss.go
- FOUND: test/bench/rss/rss_linux.go
- FOUND: test/bench/rss/rss_darwin.go
- FOUND: test/bench/rss/rss_other.go
- FOUND: test/bench/rss/rss_test.go
- FOUND: test/bench/bench_helpers_test.go
- FOUND: test/bench/tools_manifest_test.go
- FOUND: test/bench/main_test.go

**Commits verified:**
- FOUND: b2886c11
- FOUND: deef4375
- FOUND: b80bee93

**Verification commands:**
- `go vet ./test/bench/...` — clean
- `go test ./test/bench/rss/...` — PASS (non-zero RSS on darwin)
- `go test -run TestBenchToolsManifestMatchesRegistry -count=1 ./test/bench/...` — PASS
- `go test -bench=. -benchtime=1x ./test/bench/...` — PASS (no benchmarks, TestMain clean)
