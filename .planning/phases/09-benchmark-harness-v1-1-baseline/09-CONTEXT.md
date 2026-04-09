# Phase 9: Benchmark Harness & v1.1 Baseline - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the benchmark infrastructure that captures pre-instrumentation v1.1 baselines for all 38 MCP tools, LSP indexing throughput (cold and warm), and memory profiles. This phase MUST ship before any observability code lands — it's the one-way door for v1.2's per-phase delta gates.

</domain>

<decisions>
## Implementation Decisions

### Benchmark Runner Strategy
- **D-01:** Tiered runner approach: GitHub-hosted (`ubuntu-latest`) for every PR with relaxed regression gate (15% time / 25% allocs at p<0.05); self-hosted runner for release-tag jobs with tight gate (10% time / 20% allocs).
- **D-02:** Self-hosted runner setup is documented but not blocking — Phase 9 ships with GitHub-hosted gate; self-hosted enabled before v1.2 release.
- **D-03:** PR runs use `-count=10` minimum, release runs use `-count=20` for tighter confidence intervals.

### Benchmark Scope
- **D-04:** All 38 MCP tools get full p50/p95/p99 benchmarks against the Go fixture from v1.1 (`testdata/fixtures/go/`).
- **D-05:** LSP indexing benchmarks cover both cold start (fresh worker) and warm reuse against the Go fixture, plus a full Serena codebase smoke test (~25,779 LOC) for real-world scale validation.
- **D-06:** Memory benchmarks capture: baseline RSS (daemon idle), per-workspace RSS (after activation + indexing), per-LS-worker RSS (after first tool call), and post-100-call RSS (drift detection).

### Memory Profile Granularity
- **D-07:** Full pprof heap profiles: every benchmark uses `b.ReportAllocs()`; major scenarios (cold-start, warm pool, full workspace index, post-100-calls) capture pprof heap snapshots committed as CI artifacts under `test/bench/pprof/`.
- **D-08:** Heap snapshots use `runtime/pprof.WriteHeapProfile` at scenario boundaries, named by scenario + git short SHA for traceability.
- **D-09:** RSS captured via `runtime.ReadMemStats` for sys/heap and platform-specific calls (macOS `vm_stat`, Linux `/proc/self/status`) for true RSS.

### Claude's Discretion
- Exact bucket configuration for histograms (deferred to Phase 11 metrics phase)
- Whether to use `golang.org/x/perf/cmd/benchstat` directly or wrap in a Make target
- Specific p99 confidence interval handling (PITFALLS noted p99 is noisy — use as warning, not gate)
- Self-hosted runner provisioning (cloud VM vs local hardware)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Research
- `.planning/research/STACK.md` — `testing.B.Loop`, `benchstat`, no third-party bench framework
- `.planning/research/PITFALLS.md` — Compiler elision (#3), env noise (#10), goroutine leaks (#11), p99 noisiness
- `.planning/research/ARCHITECTURE.md` — Hybrid macro/micro placement, `test/bench/` as `package bench_test`

### Existing Patterns (reuse)
- `test/integration/harness.go` — StartTestDaemon, PrepareFixture, callTool (benchmarks reuse the harness)
- `test/integration/helpers.go` — listSessionTools, requireGopls, callToolBehavioral
- `testdata/fixtures/go/` — Go fixture with 8+ known symbols for deterministic benches
- `internal/kernel/lspool/worker.go` — WorkerMetrics for synctest reference

### External
- [go.dev/blog/testing-b-loop](https://go.dev/blog/testing-b-loop) — Why `testing.B.Loop` over `for i := 0; i < b.N; i++`
- [Eli Bendersky: Common pitfalls in Go benchmarking](https://eli.thegreenplace.net/2023/common-pitfalls-in-go-benchmarking/)
- `golang.org/x/perf/cmd/benchstat` — Statistical comparison of benchmark runs

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 6+7+8 integration harness — `StartTestDaemon`, `PrepareFixture`, `callTool` work in benchmarks
- `testdata/fixtures/go/` — Known-symbol Go fixture for deterministic benchmark assertions
- v1.1 Go 1.25 baseline — `testing.B.Loop` is stable stdlib

### Established Patterns
- `//go:build integration` for integration tests — benchmarks use `package bench_test` (no build tag, runs with `go test -bench=.`)
- Per-test `t.TempDir()` for fixture isolation — same pattern in benchmarks via PrepareFixture
- Daemon lifecycle managed by harness — benchmarks share one daemon per benchmark function via `b.Cleanup`

### Integration Points
- New package `test/bench/` (top-level, mirrors `test/integration/` convention)
- New directory `test/bench/baselines/` for committed v1.1 numbers
- New directory `test/bench/pprof/` for committed heap snapshots
- New CI workflow `.github/workflows/bench.yml` (or addition to existing CI)
- New Make target `make bench` and `make bench-stat` for local + CI use

</code_context>

<specifics>
## Specific Ideas

- Tier the gate: PR (relaxed 15%/25%, GitHub-hosted) and release (tight 10%/20%, self-hosted)
- All 38 tools fully benchmarked — no shortcuts for the baseline
- Full pprof heap profiles per major scenario, committed as CI artifacts
- v1.1 baseline as the immutable reference for all v1.2 delta gates

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 09-benchmark-harness-v1-1-baseline*
*Context gathered: 2026-04-09*
