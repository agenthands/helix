package bench_test

// lsp_index_bench_test.go implements BENCH-03: cold + warm LSP indexing
// benchmarks against the Go fixture.
//
// Pattern reference: 09-RESEARCH.md Pattern 4 (Cold-vs-warm LSP benchmarks,
// lines 216-251). The cold variant spins up a fresh daemon per iteration and
// fences teardown with b.StopTimer/StartTimer so only the cold-index path is
// measured. The warm variant reuses a single daemon across all b.Loop
// iterations, priming the LS once outside the loop.
//
// Pitfall 4 (gopls cold-start dominates): BenchmarkLSPIndex_Cold should be
// treated as a TREND metric, not a tight regression gate. Cold numbers are
// heavily gopls-dependent and noisy across runners; the v1.2 delta gate
// (Plan 09-06) will only gate the warm bench.

import "testing"

// BenchmarkLSPIndex_Cold measures a full cold-start LSP indexing pass against
// the Go fixture. Each iteration spins up a fresh daemon, activates the
// workspace (which triggers the initial LS spawn + didOpen + indexing), then
// tears the daemon down — all inside the loop body, fenced with
// b.StopTimer/StartTimer so only the activateWorkspaceB call contributes to
// the measured ns/op.
//
// Run with `-benchtime=1x` per phase Q4 resolution — do NOT run in the default
// `-bench=.` PR suite. Cold-start is gopls-dominated (Pitfall 4); use as a
// trend metric, not a tight gate:
//
//	go test -bench=BenchmarkLSPIndex_Cold -benchtime=1x -run=^$ ./test/bench/...
func BenchmarkLSPIndex_Cold(b *testing.B) {
	requireGoplsB(b)
	fixture := prepareGoFixtureB(b)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		bd := startBenchDaemon(b)
		b.StartTimer()

		activateWorkspaceB(b, bd, fixture) // the cold index

		b.StopTimer()
		bd.Stop()
		b.StartTimer()
	}
}

// BenchmarkLSPIndex_Warm measures steady-state warm LSP calls against a
// single daemon that is primed once before the loop. Per phase Q6, three
// warmup calls run outside the timed section to discard the first-call
// latency spike. The loop body then issues the same call repeatedly so b.Loop
// can report a stable warm-path ns/op.
//
// Warm args reuse `search_symbols` with the same query ("Helper") that
// activateWorkspaceB polls during readiness, guaranteeing the fixture already
// has a matching symbol and preventing drift against the Plan 02 manifest.
func BenchmarkLSPIndex_Warm(b *testing.B) {
	requireGoplsB(b)
	bd := startBenchDaemon(b)
	fixture := prepareGoFixtureB(b)
	activateWorkspaceB(b, bd, fixture) // warm once, excluded from timing

	warmArgs := map[string]any{"query": "Helper"}
	// 3-call warmup per phase Q6: discard first-call latency spike.
	for i := 0; i < 3; i++ {
		_ = callToolB(b, bd.Session, "search_symbols", warmArgs)
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = callToolB(b, bd.Session, "search_symbols", warmArgs)
	}
}
