# Phase 9: Benchmark Harness & v1.1 Baseline - Research

**Researched:** 2026-04-08
**Domain:** Go benchmarking, statistical regression gating, Go runtime memory profiling, CI tier strategy
**Confidence:** HIGH

## Summary

Phase 9 builds `test/bench/` from scratch. The stack is entirely stdlib plus `golang.org/x/perf/cmd/benchstat` — no third-party benchmark framework. Go 1.25.1 is already the project baseline [VERIFIED: go.mod line 3], so `testing.B.Loop` is stable and mandated by the locked decisions. Zero benchmarks exist in the repo today [VERIFIED: grep for `b\.Loop\(\)` matched only planning docs, no Go source].

The dominant risk is a **harness reuse trap**: `test/integration/harness.go` uses `package integration_test` with `//go:build integration` and every entry point takes `*testing.T`, not `*testing.B` or `testing.TB` [VERIFIED: `func StartTestDaemon(t *testing.T, ...)` at harness.go:90, `func PrepareFixture(t *testing.T, ...)` at harness.go:244, `func callTool(t *testing.T, ...)` at helpers.go:34]. CONTEXT.md says "benchmarks reuse the harness" but this is literally impossible without a refactor to `testing.TB`. This is the single most important finding in the phase — it must be addressed in Wave 0 before any benchmark can be written.

The second risk is Serena's architectural shape: benchmarks for "38 MCP tools" actually measure **full stack latency** (client session → JSON-RPC over in-process transport → MCP server → kernel tool → LSP worker → gopls). Isolating what's being measured requires careful scenario design — an LSP indexing cold-start bench is dominated by gopls, not by Serena code.

**Primary recommendation:** Refactor `harness.go` / `helpers.go` entry points to accept `testing.TB` (the interface `*testing.T` and `*testing.B` both satisfy), then build `test/bench/` as `package bench_test` with a shared `BenchMain`-style daemon fixture. Use `testing.B.Loop` everywhere, `-benchmem` always, benchstat for all comparisons, and gate PRs on relative delta not absolute numbers.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**Benchmark Runner Strategy**
- **D-01:** Tiered runner approach — GitHub-hosted (`ubuntu-latest`) for every PR with relaxed regression gate (15% time / 25% allocs at p<0.05); self-hosted runner for release-tag jobs with tight gate (10% time / 20% allocs).
- **D-02:** Self-hosted runner setup is documented but not blocking — Phase 9 ships with GitHub-hosted gate; self-hosted enabled before v1.2 release.
- **D-03:** PR runs use `-count=10` minimum, release runs use `-count=20` for tighter confidence intervals.

**Benchmark Scope**
- **D-04:** All 38 MCP tools get full p50/p95/p99 benchmarks against the Go fixture from v1.1 (`testdata/fixtures/go/`).
- **D-05:** LSP indexing benchmarks cover cold start (fresh worker) and warm reuse against the Go fixture, plus a full Serena codebase smoke test (~25,779 LOC) for real-world scale validation.
- **D-06:** Memory benchmarks capture baseline RSS (daemon idle), per-workspace RSS (after activation + indexing), per-LS-worker RSS (after first tool call), and post-100-call RSS (drift detection).

**Memory Profile Granularity**
- **D-07:** Every benchmark uses `b.ReportAllocs()`; major scenarios (cold-start, warm pool, full workspace index, post-100-calls) capture pprof heap snapshots committed as CI artifacts under `test/bench/pprof/`.
- **D-08:** Heap snapshots use `runtime/pprof.WriteHeapProfile` at scenario boundaries, named by scenario + git short SHA for traceability.
- **D-09:** RSS captured via `runtime.ReadMemStats` for sys/heap and platform-specific calls (macOS `vm_stat`, Linux `/proc/self/status`) for true RSS.

### Claude's Discretion
- Exact bucket configuration for histograms (deferred to Phase 11)
- Whether to use `golang.org/x/perf/cmd/benchstat` directly or wrap in a Make target
- Specific p99 confidence interval handling (PITFALLS noted p99 is noisy — use as warning, not gate)
- Self-hosted runner provisioning (cloud VM vs local hardware)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BENCH-01 | Benchmark harness in `test/bench/` using `testing.B.Loop` (Go 1.25) | Stack section (`testing.B.Loop`), Architecture (package layout), Wave 0 harness refactor |
| BENCH-02 | Tool response time benchmarks for all 38 tools (p50/p95/p99) | Scenario design (macro table-driven bench iterating tool registry), Pitfalls #2 and #3 |
| BENCH-03 | LSP indexing throughput benchmarks (cold and warm) for Go fixture | LSP indexing scenario section, Pitfall: gopls dominates cold-start noise |
| BENCH-04 | Memory profile benchmarks (baseline, per-workspace, per-LS-worker) | RSS capture pattern (`ReadMemStats` + platform RSS), pprof scenario section |
| BENCH-05 | CI benchstat regression gate (fails on >10% time or >20% allocs at p<0.05) | benchstat section, tiered CI strategy, `benchstat -alpha 0.05` |
| BENCH-06 | v1.1 baselines committed to `test/bench/baselines/` | Baseline capture workflow, file naming, format (`benchstat` text output) |

## Project Constraints (from CLAUDE.md)

- **Go build commands**: `go build ./cmd/serena`, `go test ./...`, `go vet ./...`, `gofmt -w .`
- **Always run `go vet` and `go test` before completing any Go task** — this includes benchmark code. Benchmarks must pass `go vet`.
- **Tool Registration pattern**: Kernel tools use `RegisterTools(server *mcp.SerenaMCPServer, ...)` with typed args + `mcpsdk.AddTool`; skill tools use `ToolProvider.Tools()` returning `[]*mcp.ToolDef`. Benchmark macro loops can iterate this registry.
- **Worker pool contract**: share-until-dirty, adaptive TTL, circuit breaking, pressure eviction. Cold-vs-warm benchmarks must explicitly control the share-until-dirty path.
- **Make targets exist**: `make build`, `make test` — add `make bench` and `make bench-stat` as siblings, don't break existing targets.
- **Python legacy isolated**: `legacy/` is Python-only; no bench work touches it.
- **GSD workflow**: Phase 9 work must go through `/gsd:execute-phase` — no ad-hoc edits.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` (stdlib) | Go 1.25.1 | `testing.B`, `testing.B.Loop`, `B.ReportAllocs()` | `testing.B.Loop` added in Go 1.24, stable in 1.25, **prevents compiler elision and auto-excludes setup**. Mandated by D-locks and BENCH-01. [VERIFIED: go.mod `go 1.25.1`] |
| `runtime/pprof` (stdlib) | Go 1.25.1 | `WriteHeapProfile`, CPU profiling | Stdlib-native heap snapshots; mandated by D-07/D-08. [CITED: pkg.go.dev/runtime/pprof] |
| `runtime` (stdlib) | Go 1.25.1 | `ReadMemStats`, `NumGoroutine`, `GC` | Baseline process-level memory stats per D-09. |
| `golang.org/x/perf/cmd/benchstat` | latest (installed via `go install golang.org/x/perf/cmd/benchstat@latest`) | Statistical comparison of `go test -bench` output with Welch's t-test at configurable alpha | **The** canonical Go benchmark comparison tool. Mandated by BENCH-05. [CITED: pkg.go.dev/golang.org/x/perf/cmd/benchstat] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os/exec` (stdlib) | — | Shell out to `vm_stat` on macOS | Platform RSS per D-09 |
| `bufio` + `os` (stdlib) | — | Parse `/proc/self/status` on Linux | Platform RSS per D-09 |
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.5.0 [VERIFIED: go.mod] | Client session + tool invocation inside benches | Reuse the same call path as integration tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `testing.B.Loop` | classic `for i := 0; i < b.N; i++` with `runtime.KeepAlive(sink)` | Works, but vulnerable to elision and mixes setup with measurement. Explicitly forbidden by D-locks and BENCH-01. |
| `benchstat` | hand-written p50/p95/p99 reporter | benchstat is the only tool with standard Welch's-t p-value gating accepted by the Go community. Hand-rolling means inventing a stats pipeline. **Don't hand-roll.** |
| `prometheus/client_golang` histograms inside benchmarks | direct latency capture into slices | Cardinality explosion (PITFALLS #2) and adds observability dependency into the phase that must land **before** observability. Forbidden by meta-pitfall ordering. |
| Third-party framework (e.g. `hyperfine`, `wrk`) | — | Wrong abstraction level — those are black-box tools. We need in-process `testing.B` to compare allocs/op. |
| `pkg/profile` (Dave Cheney) | — | Wraps `runtime/pprof` with convenience; stdlib is fine and introduces zero new deps. Not worth the import. |

**Installation:**
```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

**Version verification:**
- `testing.B.Loop`: shipped in Go 1.24, stable in 1.25. Project is `go 1.25.1` [VERIFIED: go.mod line 3 read this session]. No further check needed.
- `benchstat`: fetch latest from `golang.org/x/perf` at CI provision time. The tool's CLI surface is stable since 2023 (`-alpha`, `-row`, `-col`, `-filter` flags) [CITED: pkg.go.dev/golang.org/x/perf/cmd/benchstat].
- `golang.org/x/perf` module is active (part of the Go sub-repos). [ASSUMED: no breaking changes since training cutoff]

## Architecture Patterns

### Recommended Project Structure

```
test/
├── bench/                          # new (BENCH-01)
│   ├── main_test.go                # TestMain / shared daemon + fixtures
│   ├── tools_bench_test.go         # all-38-tools p50/p95/p99 (BENCH-02)
│   ├── lsp_index_bench_test.go     # cold/warm indexing (BENCH-03)
│   ├── memory_bench_test.go        # RSS + pprof scenarios (BENCH-04, D-06)
│   ├── fullrepo_smoke_test.go      # Serena codebase smoke (D-05)
│   ├── rss/                        # platform RSS helper (D-09)
│   │   ├── rss.go                  # public: CurrentRSS() (uint64, error)
│   │   ├── rss_linux.go            # //go:build linux — /proc/self/status
│   │   ├── rss_darwin.go           # //go:build darwin — ps/vm_stat
│   │   └── rss_other.go            # //go:build !linux && !darwin — fallback
│   ├── harness/                    # TB-generic wrappers over test/integration
│   │   └── harness.go              # wraps StartTestDaemon/PrepareFixture/callTool for testing.TB
│   ├── baselines/                  # committed (BENCH-06, D-locks)
│   │   ├── v1.1-github-hosted.txt  # benchstat-format baseline
│   │   └── v1.1-self-hosted.txt    # (post-phase, release-only)
│   └── pprof/                      # committed (D-07)
│       ├── cold-start-<sha>.pb.gz
│       ├── warm-pool-<sha>.pb.gz
│       ├── full-index-<sha>.pb.gz
│       └── post-100-calls-<sha>.pb.gz
└── integration/                    # existing — MINIMAL refactor to TB
    ├── harness.go                  # change t *testing.T → tb testing.TB
    └── helpers.go                  # change t *testing.T → tb testing.TB
```

### Pattern 1: `testing.B.Loop` — the only loop form allowed

**What:** Replace every `for i := 0; i < b.N; i++` with `for b.Loop() { ... }`. Setup outside the loop is automatically excluded from timing.

**When to use:** Every benchmark in this phase. No exceptions (BENCH-01 locked).

**Example:**
```go
// Source: https://go.dev/blog/testing-b-loop (authoritative)
func BenchmarkFindReferences(b *testing.B) {
    td := startBenchDaemon(b)       // setup — excluded from timing
    args := map[string]any{
        "relative_path": "pkg/math.go",
        "line":          10,
        "column":        5,
    }
    b.ReportAllocs()
    for b.Loop() {                  // timing begins on first iteration
        res, err := callToolB(b, td.Session, "find_references", args)
        if err != nil {
            b.Fatal(err)
        }
        _ = res                     // sink; b.Loop() prevents elision
    }
}
```

**Key properties of `b.Loop()`** [CITED: go.dev/blog/testing-b-loop]:
1. Keeps loop body alive — compiler cannot elide even if result unused.
2. Automatically excludes pre-loop setup from reported ns/op.
3. Internal iteration count replaces `b.N`.
4. Still compatible with `b.ReportAllocs()`, `b.ResetTimer()` (rarely needed now), `b.StopTimer()`/`b.StartTimer()`.

### Pattern 2: Shared daemon with `BenchMain`

**What:** Benchmarks cannot start a new daemon per iteration — gopls cold-start is multi-second. One daemon per benchmark *function*, reused across `b.Loop()` iterations.

**When to use:** All tool benchmarks (BENCH-02). LSP cold-start bench (BENCH-03 cold) is the only case that needs per-outer-iteration daemon restart.

**Example:**
```go
// Source: project convention + testing.B semantics
func BenchmarkAll38Tools(b *testing.B) {
    td := startBenchDaemon(b)        // ONE daemon for the whole bench function
    fixture := prepareGoFixtureB(b)
    _ = activateWorkspaceB(b, td, fixture)

    for _, tool := range benchToolRegistry() {  // 38 sub-benchmarks
        b.Run(tool.name, func(b *testing.B) {
            b.ReportAllocs()
            for b.Loop() {
                _ = callToolB(b, td.Session, tool.name, tool.args)
            }
        })
    }
}
```

### Pattern 3: `testing.TB` refactor (Wave 0 prerequisite)

**What:** The existing integration harness takes `*testing.T`. Every helper (`StartTestDaemon`, `PrepareFixture`, `callTool`, `callToolExpectError`, `callToolBehavioral`, `listSessionTools`, `requireGopls`) must accept `testing.TB` — the interface satisfied by both `*testing.T` and `*testing.B`.

**When to use:** One-shot refactor at start of Phase 9. Every existing integration test continues to compile unchanged because `*testing.T` satisfies `testing.TB`.

**Example diff:**
```go
// BEFORE — test/integration/harness.go:90
func StartTestDaemon(t *testing.T, opts Options) *TestDaemon {
    t.Helper()
    // ...
}

// AFTER
func StartTestDaemon(tb testing.TB, opts Options) *TestDaemon {
    tb.Helper()
    // ...
}
```

**Gotcha:** The TestDaemon struct currently stores `t *testing.T` [VERIFIED: harness.go:55]. Change field to `tb testing.TB`. Any usage of `td.t.Run(...)` must be guarded because `testing.TB.Run` does not exist — call sites that need `Run` must receive `*testing.T` or `*testing.B` directly.

### Pattern 4: Cold-vs-warm LSP benchmarks

**What:** The share-until-dirty worker pool means a second tool call on the same workspace hits a warm worker. Cold and warm are separate benchmarks.

**Cold** — must destroy the worker between iterations. `testing.B.Loop` will still iterate; each iteration restarts the LS worker.

**Warm** — single LS worker, repeated tool calls in the loop.

**Example:**
```go
func BenchmarkLSPIndex_Cold(b *testing.B) {
    fixture := prepareGoFixtureB(b)
    b.ReportAllocs()
    for b.Loop() {
        b.StopTimer()
        td := startBenchDaemon(b)      // fresh daemon == fresh LS worker
        b.StartTimer()
        activateWorkspaceB(b, td, fixture)   // this is the cold index
        b.StopTimer()
        td.Stop()
        b.StartTimer()
    }
}

func BenchmarkLSPIndex_Warm(b *testing.B) {
    td := startBenchDaemon(b)
    fixture := prepareGoFixtureB(b)
    activateWorkspaceB(b, td, fixture)  // warm once, excluded from timing
    b.ReportAllocs()
    for b.Loop() {
        // call a tool that triggers re-use of the warm index
        _ = callToolB(b, td.Session, "find_symbol", warmArgs)
    }
}
```

**Note:** Cold is the only bench that uses `b.StopTimer/StartTimer` — `testing.B.Loop` handles exclusion of setup automatically otherwise.

### Pattern 5: pprof heap snapshots at scenario boundaries

**What:** D-07/D-08 mandate committed heap snapshots. Capture via `runtime.GC(); pprof.WriteHeapProfile(f)` after each scenario. File name includes git short SHA so snapshots are traceable.

**Example:**
```go
// Source: pkg.go.dev/runtime/pprof#WriteHeapProfile
func snapshotHeap(tb testing.TB, scenario string) {
    tb.Helper()
    sha := gitShortSHA(tb)  // `git rev-parse --short HEAD`
    path := filepath.Join("pprof", fmt.Sprintf("%s-%s.pb.gz", scenario, sha))
    f, err := os.Create(path)
    if err != nil { tb.Fatal(err) }
    defer f.Close()
    runtime.GC()                      // stabilize before snapshot
    if err := pprof.WriteHeapProfile(f); err != nil {
        tb.Fatal(err)
    }
}
```

**Critical:** `runtime.GC()` before `WriteHeapProfile` is required to avoid counting short-lived allocations that would be collected momentarily. [CITED: pkg.go.dev/runtime/pprof#WriteHeapProfile]

### Pattern 6: RSS capture across macOS and Linux

**What:** `runtime.ReadMemStats` reports Go-managed memory (HeapSys, Sys). True process RSS requires platform calls. D-09 mandates both.

**Linux:** Parse `/proc/self/status` for `VmRSS:` line. Value is in KiB.

**macOS:** `runtime.ReadMemStats` alone is insufficient. Use `ps -o rss= -p <pid>` or parse `vm_stat` output. `ps` is simpler — output is RSS in KiB, one integer per line.

**Build-tagged files:**
```go
// rss_linux.go
//go:build linux
package rss
// reads /proc/self/status

// rss_darwin.go
//go:build darwin
package rss
// shells out to `ps -o rss= -p <pid>`
```

**Pitfall:** macOS `ps` reports in KiB by default but the column width varies. Use `-o rss=` (with trailing `=`) to get a header-less numeric column. Test in CI on both platforms before relying on the parser.

### Pattern 7: benchstat regression gate

**What:** PR CI runs `go test -bench=. -benchmem -count=10 ./test/bench/... > new.txt`, then `benchstat -alpha 0.05 baselines/v1.1-github-hosted.txt new.txt` and fails if any `sec/op` row exceeds the configured delta.

**Gate thresholds (from D-locks):**
- PR (GitHub-hosted, `-count=10`): time +15%, allocs +25%, p < 0.05
- Release (self-hosted, `-count=20`): time +10%, allocs +20%, p < 0.05

**benchstat does not natively enforce thresholds** — it computes p-values and deltas. A wrapper script parses benchstat output and exits non-zero on threshold breach.

**Wrapper strategy:** A small Go program at `test/bench/cmd/benchgate/main.go` reads benchstat's machine-readable `-format csv` output and applies the threshold. [ASSUMED: benchstat supports `-format csv`; verify at implementation time — it may require the newer `benchstat -row .name -col .config` matrix form instead.]

### Anti-Patterns to Avoid

- **Running a benchmark inside a non-bench file:** Every file in `test/bench/` must end in `_test.go` and every function must start with `Benchmark`. Otherwise `go test -bench=.` will not pick it up.
- **Reusing the integration `//go:build integration` tag:** `test/bench/` must NOT use a build tag. Benchmarks are discovered by `go test -bench=. ./test/bench/...` without special flags.
- **Calling `b.ResetTimer()` after `b.Loop()`:** `b.Loop` already handles this. Adding `ResetTimer` is a code smell that implies someone is not sure the pattern was understood.
- **Gating PRs on absolute ns/op:** Gate on **relative delta** vs committed baseline. Absolute numbers move with runner hardware and cause flakes.
- **Benchmarking tools that require real network/DB:** All 38 tools run in-process against `testdata/fixtures/go/` — no external services. Confirmed by D-04.
- **Reporting p99 as a CI gate:** PITFALLS explicitly warns p99 is noisy. Use p99 for **trend watching**, not gating. Gate on ns/op mean + benchstat p-value.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Statistical comparison of benchmark runs | Custom mean/variance/CI calculator | `benchstat` with `-alpha 0.05` | Welch's t-test, geomean, and Go community's expected format. Rolling your own means inventing a standard. |
| Compiler-elision prevention | `runtime.KeepAlive`, global sinks, `_ = result` | `testing.B.Loop` | The stdlib now handles this; fighting it is strictly worse. |
| Heap snapshots | Manual `MemStats` + diff math | `runtime/pprof.WriteHeapProfile` | pprof format is the only one `go tool pprof` and CI artifact viewers understand. |
| RSS on Linux | `gopsutil` | Parse `/proc/self/status` directly | Eight lines of bufio beats a transitive dependency tree. |
| RSS on macOS | `gopsutil` or mach syscalls via CGO | Shell out to `ps -o rss=` | CGO-free constraint from project. `ps` exists everywhere. |
| Benchmark fixture management | New fixture system | Reuse `testdata/fixtures/go/` + `PrepareFixture` | Already known-good and symbol-annotated for v1.1. |
| Daemon lifecycle in benches | New daemon spin-up code | Reuse `StartTestDaemon` (after TB refactor) | Same code path as integration tests = same behavior. |
| p-value computation | — | `benchstat -alpha 0.05` | Use the flag. |

**Key insight:** This phase is almost entirely "glue stdlib to existing harness." The only novel code is (a) the 38-tool registry iteration, (b) the RSS package, (c) the benchstat wrapper script. Everything else is composition of stdlib + existing test infrastructure.

## Common Pitfalls

### Pitfall 1: Harness is hard-typed to `*testing.T` (Wave 0 blocker)
**What goes wrong:** You try to call `integration.StartTestDaemon(b, opts)` from a benchmark, Go compiler rejects it because `*testing.B` is not `*testing.T`.
**Why it happens:** `test/integration/harness.go:90` has signature `func StartTestDaemon(t *testing.T, opts Options)` [VERIFIED this session]. Same for `PrepareFixture:244`, `callTool` at helpers.go:34.
**How to avoid:** First task of the phase is a TB refactor: every `*testing.T` in `harness.go` and `helpers.go` becomes `testing.TB`. Both `*testing.T` and `*testing.B` satisfy the interface so integration tests keep compiling. The only field that cannot be TB is anything that calls `.Run(...)` — `testing.TB` does not expose `Run`.
**Warning signs:** Benchmarks fail to compile with "cannot use b (variable of type *testing.B) as *testing.T".
**Mitigation task:** `T-09-01-refactor-harness-tb` must be in Wave 0.

### Pitfall 2: `//go:build integration` tag trap
**What goes wrong:** You copy the integration test file header into a bench file, the bench is hidden behind a build tag, and `go test -bench=. ./test/bench/...` reports "no tests to run."
**Why it happens:** `test/integration/harness.go` has `//go:build integration` at line 1 [VERIFIED this session]. Copy-paste of the file header into bench files silently excludes them.
**How to avoid:** `test/bench/` files have **no build tag**. They run by default when you pass `-bench`. Integration harness stays tagged — benchmarks import the (now TB-typed) harness code directly.
**Warning signs:** Empty benchstat output. `-v` flag shows no "Benchmark..." lines.

### Pitfall 3: Compiler elision hiding real work (PITFALLS #1)
**What goes wrong:** Benchmark reports sub-nanosecond ops because the compiler eliminated the call.
**Why it happens:** Classic `for i := 0; i < b.N; i++ { Foo() }` with no sink. Made worse by small pure functions.
**How to avoid:** Mandate `testing.B.Loop`. Loop prevents elision. Still assign result to a local so it's observed. Mandated by BENCH-01 and D-lock.
**Warning signs:** ns/op < 1 for a function that hits JSON-RPC + LSP. Any tool benchmark reporting < 100ns is almost certainly elided — real MCP round-trip involves goroutine scheduling and gRPC-like call routing, minimum hundreds of microseconds.
**Detection:** `go test -bench=. -gcflags='-m' ./test/bench/... 2>&1 | grep 'inlining call'` — review inlining decisions manually.

### Pitfall 4: gopls cold-start dominates the "Serena LSP indexing" benchmark
**What goes wrong:** `BenchmarkLSPIndex_Cold` reports 3-8 seconds per iteration. 90% of that is gopls binary startup + initialize + workspace scan. The "regression gate" on this benchmark mostly tracks gopls version drift, not Serena changes.
**Why it happens:** Cold indexing = `exec.Command("gopls", ...)` + LSP `initialize` + `initialized` + `workspace/didChangeWorkspaceFolders` + first symbol index. Serena is the thin orchestration layer.
**How to avoid:**
- Split the metric: report `daemon_cold_start_ms` separately from `index_ready_ms`. Only the first is Serena-attributable.
- Use the warm benchmark as the primary Serena regression signal. Cold is a smoke / trend metric, not a tight gate.
- Pin gopls version in CI (`go install golang.org/x/tools/gopls@vX.Y.Z`) so gopls updates are intentional events, not noise.
**Warning signs:** Benchstat comparison shows 20% cold-start regression after a PR that only touched skill config — the real cause is a gopls upgrade on the runner.

### Pitfall 5: Benchmark environment noise on `ubuntu-latest` (PITFALLS #10)
**What goes wrong:** GitHub-hosted runners share CPU with neighboring jobs; variance is ±20%; CI flakes constantly; team learns to re-run until green.
**Why it happens:** `ubuntu-latest` is a shared VM on Azure; no CPU pinning, no turbo control, no performance governor.
**How to avoid:** D-locks already resolved this — PR gate is relaxed (15%/25% at `-count=10`) because it runs on noisy GitHub-hosted. Tight gate (10%/20% at `-count=20`) is release-only on self-hosted. Do not tighten PR thresholds.
- Also: discard a warm-up iteration, pin `GOMAXPROCS` explicitly (`GOMAXPROCS=4 go test -bench=...`), set `GOGC=off` for latency benchmarks to avoid GC-induced variance.
- Report both runners' baselines — they will not match.
**Warning signs:** Same PR shows +18% one run and -4% the next. Retry-until-green culture emerging.

### Pitfall 6: Goroutine leaks across benchmark iterations (PITFALLS #11)
**What goes wrong:** Each iteration starts a new TestDaemon or forgets to close one. After 10 iterations of a 100-call benchmark you have 100 leaked goroutines; GC pressure rises; later benchmarks run 30% slower than earlier ones.
**Why it happens:** `b.Cleanup` runs at end of bench function, not end of iteration. If setup is in the loop by mistake, the cleanup accumulates.
**How to avoid:**
- One daemon per bench function (in `startBenchDaemon`, registered via `b.Cleanup(td.Stop)`).
- Never start a daemon inside `for b.Loop()` except in the cold-start scenario (which explicitly calls `td.Stop()` inside the loop).
- Add `runtime.NumGoroutine()` assertion in `main_test.go` TestMain exit hook — fail if leaked > baseline.
**Warning signs:** `ns/op` trends upward across sub-benchmarks within the same run.

### Pitfall 7: `runtime.ReadMemStats` gives you Go-managed bytes, not RSS
**What goes wrong:** D-09 says "RSS". You report `memstats.Sys` and think you're done. `Sys` is what Go has asked the OS for; kernel RSS is what's actually resident. They diverge significantly after GC due to `MADV_FREE` pages not being reclaimed immediately.
**Why it happens:** Confusion between Go heap accounting and OS accounting.
**How to avoid:** Report **both**. `runtime.ReadMemStats` for Go's view. Platform RSS for kernel's view. Label them clearly in the baseline file: `rss_sys_bytes`, `rss_kernel_bytes`. D-09 explicitly calls for platform-specific calls.
**Warning signs:** A "memory regression" alarm where Go stats are flat but kernel RSS grew — that's a leak in cgo land or the MADV_FREE quirk, not a Go-side bug. Investigate both angles.

### Pitfall 8: Committed `pprof/*.pb.gz` balloons the repo
**What goes wrong:** D-07 says commit heap snapshots under `test/bench/pprof/`. Every CI run creates 4 new files (cold-start, warm-pool, full-index, post-100-calls). Over 100 merges the directory has 400+ files and several hundred MB.
**Why it happens:** "Committed as CI artifacts" was interpreted as "commit to git on every run."
**How to avoid:**
- **Only commit baseline snapshots** (v1.1 once, then updated only on intentional re-baseline — typically once per minor version).
- PR runs **upload snapshots as GitHub Actions artifacts**, they do NOT push to git.
- Release tag jobs regenerate the baseline snapshots and commit in a single designated PR.
- Use `.gitattributes` to mark `*.pb.gz` as binary (no diff in git UI) and add to a separate cleanup strategy if size becomes an issue.
**Warning signs:** `git log --stat` showing `test/bench/pprof/` churn every PR. Clone times rising.

### Pitfall 9: benchstat CSV schema assumptions
**What goes wrong:** You write a wrapper script that parses benchstat output, the next benchstat release changes column order, CI breaks on unrelated PRs.
**Why it happens:** benchstat's text output is human-oriented; its structured output has evolved.
**How to avoid:**
- Pin the benchstat version used in CI (`go install golang.org/x/perf/cmd/benchstat@<commit-sha>`) — not `@latest`.
- Prefer stable row/col names: `-row .fullname -col .config`.
- Consider writing the wrapper as a Go program using `golang.org/x/perf/benchfmt` (the library benchstat is built on) — parsing raw benchmark output is more stable than parsing benchstat's report.
**Warning signs:** CI breaks on a Monday with no code changes; the runner pulled a newer benchstat.

### Pitfall 10: 38 tools actually need different args (BENCH-02 complexity)
**What goes wrong:** "Benchmark all 38 tools" sounds like a for-loop over a registry. In reality, each tool has a different argument schema: `find_references` needs a file+line+column; `write_file` needs a file+content; `search_for_pattern` needs a regex; memory tools need a topic. You cannot call them uniformly.
**Why it happens:** Tool registration is typed per-tool via `mcpsdk.AddTool` [VERIFIED: 82 occurrences across 15 files this session].
**How to avoid:**
- Build a **bench manifest**: `var benchTools = []benchCase{{name: "find_symbol", args: map[string]any{"name_path": "KnownType"}}, ...}` — hand-curated against `testdata/fixtures/go/` so each tool gets args known to produce a real-work response.
- Manifest lives in `test/bench/tools_manifest.go`. Each entry cites which fixture symbol it touches.
- Smoke: assert `len(benchTools) == 38` in TestMain. If the tool registry grows, the test fails until manifest is updated.
**Warning signs:** Benchmarks "pass" but results show some tools returning errors — silent error swallowing makes the latency meaningless. Always `b.Fatal` on tool error.

### Pitfall 11: Full-repo smoke (D-05) has long tail
**What goes wrong:** Indexing all ~25,779 LOC of Serena itself takes 15-30 seconds per iteration. `testing.B.Loop` will run this enough times to get a stable measurement, meaning a single PR CI run spends 5+ minutes just on this benchmark.
**Why it happens:** gopls scales with workspace size.
**How to avoid:**
- Gate the full-repo smoke behind `-bench=FullRepo` pattern — not part of default `-bench=.` PR run.
- Run it on release tag only, or on a nightly scheduled job.
- If we must run on PR, wrap in `if testing.Short() { b.Skip("full-repo smoke skipped in -short") }` and pass `-short` in PR runs.
**Warning signs:** CI PR job duration creeping into the tens of minutes.

## Code Examples

Verified patterns adapted from Go stdlib and official docs.

### TestMain scaffold with goroutine leak detection
```go
// Source: pkg.go.dev/testing#Main + project convention
package bench_test

import (
    "os"
    "runtime"
    "testing"
)

func TestMain(m *testing.M) {
    baseline := runtime.NumGoroutine()
    code := m.Run()
    runtime.GC()
    if leaked := runtime.NumGoroutine() - baseline; leaked > 2 {
        // tolerate <=2 for test-framework internals
        _, _ = os.Stderr.WriteString("goroutine leak detected\n")
        code = 1
    }
    os.Exit(code)
}
```

### Table-driven 38-tool benchmark using b.Loop
```go
// Source: project pattern + go.dev/blog/testing-b-loop
func BenchmarkTools(b *testing.B) {
    td := startBenchDaemon(b)
    fixture := prepareGoFixtureB(b)
    activateWorkspaceB(b, td, fixture)

    for _, tc := range benchTools {  // 38 entries, asserted in TestMain
        b.Run(tc.name, func(b *testing.B) {
            b.ReportAllocs()
            var result any
            for b.Loop() {
                r, err := callToolB(b, td.Session, tc.name, tc.args)
                if err != nil {
                    b.Fatal(err)
                }
                result = r
            }
            _ = result
        })
    }
}
```

### Heap snapshot at scenario boundary
```go
// Source: pkg.go.dev/runtime/pprof
func heapSnapshot(tb testing.TB, name string) {
    tb.Helper()
    runtime.GC()
    path := filepath.Join("pprof", fmt.Sprintf("%s-%s.pb.gz", name, gitShortSHA()))
    f, err := os.Create(path)
    if err != nil {
        tb.Fatal(err)
    }
    defer f.Close()
    if err := pprof.WriteHeapProfile(f); err != nil {
        tb.Fatal(err)
    }
}
```

### Linux RSS reader
```go
//go:build linux
// Source: Linux kernel /proc/[pid]/status documentation
package rss

import (
    "bufio"
    "fmt"
    "os"
    "strconv"
    "strings"
)

func CurrentRSS() (uint64, error) {
    f, err := os.Open("/proc/self/status")
    if err != nil {
        return 0, err
    }
    defer f.Close()
    s := bufio.NewScanner(f)
    for s.Scan() {
        line := s.Text()
        if strings.HasPrefix(line, "VmRSS:") {
            fields := strings.Fields(line)
            if len(fields) < 2 {
                return 0, fmt.Errorf("malformed VmRSS line")
            }
            kib, err := strconv.ParseUint(fields[1], 10, 64)
            if err != nil {
                return 0, err
            }
            return kib * 1024, nil
        }
    }
    return 0, fmt.Errorf("VmRSS not found")
}
```

### macOS RSS reader
```go
//go:build darwin
// Source: ps(1) man page — `-o rss=` prints headerless RSS in KiB
package rss

import (
    "fmt"
    "os"
    "os/exec"
    "strconv"
    "strings"
)

func CurrentRSS() (uint64, error) {
    pid := os.Getpid()
    out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
    if err != nil {
        return 0, err
    }
    s := strings.TrimSpace(string(out))
    kib, err := strconv.ParseUint(s, 10, 64)
    if err != nil {
        return 0, fmt.Errorf("parse ps output %q: %w", s, err)
    }
    return kib * 1024, nil
}
```

### benchstat gate invocation (Makefile target)
```makefile
# Source: golang.org/x/perf/cmd/benchstat CLI
bench:
	go test -bench=. -benchmem -count=10 ./test/bench/... | tee new.txt

bench-stat: bench
	benchstat -alpha 0.05 test/bench/baselines/v1.1-github-hosted.txt new.txt

bench-gate: bench
	go run ./test/bench/cmd/benchgate \
	    -baseline test/bench/baselines/v1.1-github-hosted.txt \
	    -new new.txt \
	    -time-threshold 0.15 \
	    -alloc-threshold 0.25 \
	    -alpha 0.05
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `for i := 0; i < b.N; i++` with sink vars | `for b.Loop() { ... }` | Go 1.24 (Feb 2025) | Elision-proof, auto setup exclusion. Project is Go 1.25.1 [VERIFIED], so we use this. |
| benchcmp (deprecated) | benchstat | ~2017 | benchstat is the only supported tool now. |
| Custom p-value math | `benchstat -alpha 0.05` | — | Alpha flag has been stable for years. |
| cgo-based RSS readers (`gopsutil`) | `/proc/self/status` + `ps` | project CGO-free constraint | Keep single-binary promise. |
| Hand-rolled tool registries in docs | `go generate` reading `ToolDef` registry | v1.2 DOC-02 | Not Phase 9 scope but bench manifest can share the convention. |

**Deprecated/outdated:**
- `runtime.SetBlockProfileRate` as a benchmark helper: noisy, rarely informative; skip unless chasing a specific contention report.
- Using `httptest.NewServer` as a transport for benchmarks: project uses in-process MCP client session via the SDK; no HTTP transport in default test path.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `benchstat` latest has stable `-alpha` and `-row`/`-col` flags | Standard Stack, Pattern 7 | Wrapper script would need rewriting; mitigation is to pin a specific benchstat commit SHA and document it. |
| A2 | `benchstat` supports a machine-readable output format usable by a gate wrapper | Pattern 7 | If only text output is available, gate wrapper must parse text; alternative is to use `golang.org/x/perf/benchfmt` directly (bypass benchstat CLI for gating). |
| A3 | macOS `ps -o rss=` returns RSS in KiB consistently | Pattern 6, Code Examples | If KiB vs bytes differs by macOS version, reported numbers drift; verify at implementation time with a sanity assertion (RSS > 1 MiB). |
| A4 | Pinning gopls version via `go install golang.org/x/tools/gopls@vX.Y.Z` works in GitHub-hosted CI | Pitfall #4 | Standard Go tooling; low risk. |
| A5 | `runtime.NumGoroutine()` baseline of ±2 is tolerable in TestMain leak check | Pattern, Pitfall #6 | If framework internals leak more, the assertion flakes — tune tolerance empirically in Wave 0. |
| A6 | The Serena codebase is ~25,779 LOC as CONTEXT.md claims | Full-repo smoke scope | If materially larger, the smoke bench duration estimate is wrong; CI runtime may blow out. Measure once during Wave 0. |
| A7 | All 38 MCP tools can be called in-process against `testdata/fixtures/go/` with pre-curated args producing non-error responses | Pitfall #10, BENCH-02 | Some tools may require non-Go fixtures (e.g., memory tools operate on markdown store). Manifest construction must handle tool domain diversity. |
| A8 | Existing integration harness refactor from `*testing.T` to `testing.TB` is source-compatible with all current call sites | Pattern 3, Wave 0 task | If any integration test uses `t.Run` via the stored field, TB refactor may need a split. Quick audit during Wave 0 resolves this. |
| A9 | `runtime/pprof.WriteHeapProfile` output compresses well enough that one snapshot is < 1 MiB | Pitfall #8 | If snapshots are many MiB each, repo bloat strategy (artifacts not git) must be stricter. |

**User confirmation needed for:** A6 (LOC count for time budgeting), A7 (whether BENCH-02 must cover memory/workflow tools that aren't code-operation tools in the same way).

## Open Questions

1. **Does BENCH-02 "all 38 tools" include memory + workflow tools?**
   - What we know: D-04 says all 38. The 38 tools span kernel (24: 9 symbols + 6 edit + 6 fileops + 3 diag) + skills (7 memory + ~7 workflow).
   - What's unclear: memory tools operate on a markdown store, not the Go fixture. Workflow tools (onboarding, handoff) have side effects and may not be latency-sensitive at all.
   - Recommendation: benchmark them, but allow `outcome=skip` cases documented in the manifest. Confirm during planning that "skip with rationale" counts as "benchmarked."

2. **How do we handle the self-hosted runner provisioning in Phase 9?**
   - What we know: D-02 says self-hosted is documented but non-blocking for Phase 9 ship.
   - What's unclear: whether "documented" means a markdown runbook, a Terraform script, or nothing at all.
   - Recommendation: ship a `docs/bench-runner-setup.md` with manual steps (cpupower, GOMAXPROCS, benchstat install, disable turbo). Cloud VM vs local hardware is a later decision.

3. **Where does the benchstat-gating wrapper live?**
   - What we know: benchstat itself doesn't enforce thresholds.
   - What's unclear: separate Go binary under `test/bench/cmd/benchgate/`, or a shell script, or a Make target with awk.
   - Recommendation: Go binary — testable, can use `golang.org/x/perf/benchfmt` directly, aligns with CLAUDE.md "Language: Go" constraint.

4. **Should cold-start LSP bench use `b.StopTimer`/`b.StartTimer` or sub-benchmarks?**
   - What we know: `testing.B.Loop` auto-excludes pre-loop setup but not per-iteration setup.
   - What's unclear: whether mixing StopTimer with b.Loop introduces artifacts.
   - Recommendation: test during implementation; if artifacts appear, fall back to one-shot benchmarks (`-benchtime=1x`) and report LSP cold-start as a fixed measurement rather than an averaged sample.

5. **What's the p99 treatment in the gate?**
   - What we know: PITFALLS flags p99 as noisy.
   - What's unclear: CONTEXT.md says "use as warning, not gate" but doesn't specify how to *display* p99 without gating on it.
   - Recommendation: report p99 in a separate section of the benchstat report titled "Trend watch — non-gating" so humans see it without CI failing on it.

6. **Do the 38-tool benchmarks need per-tool warmup calls?**
   - What we know: share-until-dirty means the first call to any tool on a workspace pays indexing cost.
   - What's unclear: whether per-tool setup drop + `b.Loop` double-counts the indexing.
   - Recommendation: single pre-loop warmup call (`callToolB(b, ..., "find_symbol", warmArgs)`) to guarantee the index is hot before any `b.Loop()` runs. Document the warm assumption in each bench.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go` | build, test, bench | ✓ | 1.25.1 [VERIFIED: `go version`] | — |
| `testing.B.Loop` | BENCH-01 | ✓ | Go 1.24+, project is 1.25.1 | — |
| `benchstat` | BENCH-05 | ✗ (local) | not installed | `go install golang.org/x/perf/cmd/benchstat@latest` — low friction |
| `gopls` | BENCH-03 LSP bench | ? (system dependent) | — | Three-tier LS resolution already handles install via `internal/langregistry` |
| macOS `ps` | RSS reader on darwin | ✓ | system binary | — |
| Linux `/proc/self/status` | RSS reader on linux | ✓ (on Linux CI) | kernel feature | — |
| `git rev-parse --short HEAD` | heap snapshot naming (D-08) | ✓ | assumed present in any git checkout | — |
| GitHub Actions `ubuntu-latest` | PR bench gate (D-01) | ✓ (CI) | — | — |
| Self-hosted runner | release bench gate (D-02) | ✗ (not yet provisioned) | — | D-02 explicitly non-blocking for Phase 9 |

**Missing dependencies with no fallback:** none — every missing item has an install path or is explicitly deferred by D-locks.

**Missing dependencies with fallback:**
- `benchstat` — install in CI workflow step before running gate.
- Self-hosted runner — Phase 9 ships GitHub-hosted only; release tag job is wired but disabled until runner provisioned.

## Security Domain

> Benchmark code does not process user input, does not listen on network ports not already listened to, and does not handle credentials. ASVS surface is minimal.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — (bench runs against in-process daemon, no auth boundary) |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | benchmarks call tools with hard-coded args; no user input |
| V6 Cryptography | no | — |
| V7 Error Handling | minimal | benchmarks must `b.Fatal` on any tool error so silent failures don't produce meaningless numbers |
| V11 Business Logic | no | — |
| V14 Configuration | yes (minor) | committed pprof snapshots must not contain absolute filesystem paths of the CI runner; verify via `go tool pprof -text` on a sample before committing baseline |

### Known Threat Patterns for Go benchmark harness

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Committed pprof leaks CI runner paths | Information Disclosure | Review first-baseline heap profiles for `/home/runner/` or `/Users/` paths; scrub or regenerate on a clean path if present |
| Test fixture path traversal | Tampering | Fixture paths derived from `t.TempDir()` via `PrepareFixture` — already safe |
| Benchmark-only code shipped in release binary | — | `_test.go` files are automatically excluded from `go build`; no action needed, just a reminder to never move bench code into a non-test file |

## Sources

### Primary (HIGH confidence)
- [go.dev/blog/testing-b-loop](https://go.dev/blog/testing-b-loop) — authoritative rationale for `b.Loop` (cited in CONTEXT.md)
- [pkg.go.dev/testing](https://pkg.go.dev/testing) — `testing.B`, `testing.TB`, `B.Loop`, `B.ReportAllocs`
- [pkg.go.dev/runtime/pprof](https://pkg.go.dev/runtime/pprof) — `WriteHeapProfile` semantics
- [pkg.go.dev/golang.org/x/perf/cmd/benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) — CLI reference
- Codebase verification (this session): `go.mod` (Go 1.25.1), `test/integration/harness.go` (harness signatures), `internal/**/tools.go` (82 `mcpsdk.AddTool` occurrences across 15 files), grep confirming zero pre-existing `b.Loop()` usage in Go source

### Secondary (MEDIUM confidence)
- [Eli Bendersky — Common pitfalls in Go benchmarking](https://eli.thegreenplace.net/2023/common-pitfalls-in-go-benchmarking/) — cited in CONTEXT.md and PITFALLS
- Project PITFALLS.md (Pitfalls #1, #10, #11 directly relevant)
- Project SUMMARY.md phase-ordering rationale

### Tertiary (LOW confidence — flagged for verification)
- benchstat machine-readable output format (A2) — verify actual CLI flags at implementation time
- macOS `ps -o rss=` KiB consistency across macOS versions (A3)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib or well-known x/perf; versions verified
- Architecture: HIGH — pattern follows existing `test/integration/` convention; harness reuse path identified
- Pitfalls: HIGH — both meta-pitfall and benchmark-specific pitfalls are well-documented upstream
- Code examples: MEDIUM-HIGH — stdlib patterns are cited; bench-specific glue is project-new
- Harness reuse: MEDIUM — TB refactor is mechanically straightforward but hasn't been done yet
- Tool manifest (38 tools): MEDIUM — construction is hand-curated; correctness depends on per-tool fixture knowledge

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (30 days — stable stdlib territory, tooling rarely breaks)
