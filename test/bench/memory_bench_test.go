package bench_test

// memory_bench_test.go implements BenchmarkMemory per BENCH-04 and D-06.
//
// File suffix note (deviation vs 09-05-PLAN.md): the plan listed this file
// as `test/bench/memory_bench_test.go`, which matches the _test.go naming
// convention here. The sibling `heap_snapshot.go` in the plan was renamed
// to `heap_snapshot_test.go` for the same reason — `package bench_test`
// only compiles in _test.go files.
//
// === pprof artifact policy (CI-uploaded, NOT committed) =====================
//
// Scenario snapshots produced by this benchmark — `s1-idle-*.pb.gz`,
// `s2-workspace-*.pb.gz`, `s3-first-call-*.pb.gz`, `s4-post-100-calls-*.pb.gz`
// — are CI-uploaded artifacts ONLY. They are surfaced via the
// `upload-artifact` step in `.github/workflows/bench.yml` (added by Plan
// 09-06). They are NOT committed to the repository per 09-RESEARCH.md
// Pitfall 8 (committed pprof/*.pb.gz balloons the repo history).
//
// The .gitignore in test/bench/pprof/ excludes `*.pb.gz` globally and then
// un-excludes `baseline-*.pb.gz` via a `!baseline-*.pb.gz` override. Only
// Plan 09-06's explicit baseline snapshot — named with the `baseline-`
// prefix — is committed, and that commit happens ONLY during intentional
// re-baseline PRs. This comment is load-bearing documentation: reviewers
// must understand the artifact-vs-commit boundary at a glance before
// approving anything in this file.
//
// === D-06 scenarios =========================================================
//
// BenchmarkMemory walks four scenarios in order, each producing one RSS
// report pair (Go-managed + kernel per D-09 / Pitfall 7) and one pprof
// heap snapshot:
//
//  1. s1_idle              — daemon just started, no workspace activated
//  2. s2_workspace         — after activate_project + LS readiness wait
//  3. s3_first_call        — after the first find_symbol tool call
//  4. s4_post_100_calls    — after 100 additional find_symbol calls (drift)
//
// Run with `-benchtime=1x` for clean one-shot scenario capture. The b.Loop
// body runs exactly once per scenario boundary; looping is present for
// BENCH-01 compliance (09-RESEARCH.md Pattern 1), not for averaging.

import (
	"runtime"
	"testing"

	"github.com/agenthands/helix/test/bench/rss"
)

// reportMemory logs and ReportMetric's both Go-managed sys bytes
// (runtime.ReadMemStats.Sys) and kernel RSS (rss.CurrentRSS). Labels are
// distinct per Pitfall 7 — never conflate `rss_go_sys_bytes` (what Go asked
// the OS for) with `rss_kernel_bytes` (what's actually resident). On a
// GOOS without a platform implementation rss.ErrUnsupported degrades to an
// "unavailable" log line without failing the benchmark.
func reportMemory(b *testing.B, label string) {
	b.Helper()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	kernelRSS, err := rss.CurrentRSS()
	if err != nil {
		b.Logf("%s: rss_go_sys_bytes=%d rss_kernel_bytes=unavailable (%v)", label, ms.Sys, err)
	} else {
		b.Logf("%s: rss_go_sys_bytes=%d rss_kernel_bytes=%d", label, ms.Sys, kernelRSS)
	}
	// Surface as custom benchmark metrics so benchstat picks them up.
	b.ReportMetric(float64(ms.Sys), label+"_go_sys_B")
	if err == nil {
		b.ReportMetric(float64(kernelRSS), label+"_kernel_B")
	}
}

// benchToolArgsFor returns the args map from benchTools for a given tool
// name, panicking if the tool is not in the manifest. Keeping BenchmarkMemory
// anchored to the manifest prevents arg drift between the latency bench
// (Plan 09-03) and the memory bench.
func benchToolArgsFor(name string) map[string]any {
	for _, tc := range benchTools {
		if tc.name == name {
			return tc.args
		}
	}
	panic("benchToolArgsFor: tool not in manifest: " + name)
}

// BenchmarkMemory captures the four D-06 memory scenarios with dual RSS
// reporting (D-09 / Pitfall 7) and per-scenario pprof heap snapshots
// (D-07/D-08). Intended for `-benchtime=1x` one-shot capture — the
// b.Loop body is used for BENCH-01 compliance, not statistical averaging.
func BenchmarkMemory(b *testing.B) {
	requireGoplsB(b)
	b.ReportAllocs()

	for b.Loop() {
		// Scenario 1: daemon idle (just started, no workspace).
		bd := startBenchDaemon(b)
		reportMemory(b, "s1_idle")
		heapSnapshot(b, "s1-idle")

		// Scenario 2: post-workspace-activation (LS running, indexed).
		fixture := prepareGoFixtureB(b)
		activateWorkspaceB(b, bd, fixture)
		reportMemory(b, "s2_workspace")
		heapSnapshot(b, "s2-workspace")

		// Scenario 3: post-first-tool-call (LS worker warm).
		_ = callToolB(b, bd.Session, "find_references", benchToolArgsFor("find_references"))
		reportMemory(b, "s3_first_call")
		heapSnapshot(b, "s3-first-call")

		// Scenario 4: post-100-calls (drift detection).
		for i := 0; i < 100; i++ {
			_ = callToolB(b, bd.Session, "find_references", benchToolArgsFor("find_references"))
		}
		reportMemory(b, "s4_post_100_calls")
		heapSnapshot(b, "s4-post-100-calls")

		bd.Stop()
	}
}
