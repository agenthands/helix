// Phase 70-07 local-only bench: TestBench_RefreshIncremental_10kSymbols_P95Under200ms
// proves that the single-file-changed incremental refresh seam stays under
// p95 ≤ 200ms against a 10k-symbol workspace (200 files × 50 symbols).
//
// The bench is INVISIBLE TO CI by construction:
//   - t.Skip when os.Getenv("CI") != "" (project rule feedback_no_ci_benchmarks)
//   - t.Skip under testing.Short() (mirrors TestRunQuickFullFixtureSetWallTime convention)
//
// Per Plan 70-07 deviation note (Rule 3 — package boundary): the plan named
// internal/eval/runner/bench_refresh_incremental_test.go, but the refresh
// harness from Plan 70-06 lives in package daemon (because semanticBundle is
// unexported in that package). The bench reuses that harness verbatim, so
// the test file must live in package daemon too. This mirrors the Plan 70-06
// deviation and is justified by the same constraint.
//
// Plan 70-07 also describes the harness method as `h.refresh(t, nil)`; the
// actual Plan 70-06 harness exposes `h.triggerCollect(t) []string`. The
// signature is semantically identical for the purpose of measuring refresh
// latency — both drive the production collectCandidatePaths dispatcher
// against the captured BaseOverlayEpoch. The bench uses triggerCollect.

package daemon

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

// TestBench_RefreshIncremental_10kSymbols_P95Under200ms is the REFRESH-02
// local-only bench harness. It builds a 200-file × 50-symbol fixture (10k
// symbols total) ONCE, then samples 50 iterations of single-file-edit +
// incremental refresh, computes p50/p95/p99, and asserts p95 ≤ 200ms.
//
// Run locally:
//
//	CI= go test ./internal/daemon/ \
//	  -run TestBench_RefreshIncremental_10kSymbols_P95Under200ms \
//	  -count=1 -timeout 5m -v
func TestBench_RefreshIncremental_10kSymbols_P95Under200ms(t *testing.T) {
	// Skip gate 1: CI. Project rule feedback_no_ci_benchmarks — never run
	// benchmarks on hosted runners.
	if os.Getenv("CI") != "" {
		t.Skip("bench is local-only per project rule feedback_no_ci_benchmarks")
	}
	// Skip gate 2: -short. Mirrors TestRunQuickFullFixtureSetWallTime convention.
	if testing.Short() {
		t.Skip("bench skipped under -short")
	}

	// Build fixture ONCE — Pitfall 5 mitigation. The per-iteration cost
	// must reflect refresh latency, not snapshot construction.
	// 10k symbols = 200 files × 50 symbols/file.
	h := newRefreshHarness(t, 200, 50)
	h.indexFull(t)

	const iterations = 50
	samples := make([]time.Duration, 0, iterations)

	for i := 0; i < iterations; i++ {
		// Land an overlay row by editing a rotating file index.
		h.editFile(t, i%200, fmt.Sprintf("// iter %d\n", i))

		start := time.Now()
		_ = h.triggerCollect(t)
		samples = append(samples, time.Since(start))
	}

	// Compute percentiles.
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p50 := samples[len(samples)/2]
	p95 := samples[int(0.95*float64(len(samples)))]
	p99 := samples[len(samples)-1]

	// Log all three for local performance regression tracking, even when
	// the assertion passes.
	t.Logf("p50=%v p95=%v p99=%v", p50, p95, p99)

	// Budget assertion.
	if p95 > 200*time.Millisecond {
		t.Errorf("p95 %v exceeds 200ms budget", p95)
	}
}
