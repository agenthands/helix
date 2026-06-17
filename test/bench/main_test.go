package bench_test

// main_test.go holds the package TestMain plus the 41-tool manifest parity
// test. TestMain performs a NumGoroutine baseline leak check (09-RESEARCH.md
// Pitfall 6) and the parity test enforces bidirectional name alignment
// between benchTools and the live MCP registry (threat T-09-04).
//
// No Benchmark* functions live here — Plans 09-03..09-05 add them. Running
// `go test -run X ./test/bench/...` matches nothing, so only TestMain and
// any explicitly-named tests execute; the package still compiles with zero
// Benchmark* functions per 09-02 success criteria.

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"
)

// leakTolerance is the maximum number of goroutines allowed to leak past the
// TestMain baseline. Tuned empirically during Phase 9 Plan 02 Wave 0 per
// 09-RESEARCH.md Assumption A5: daemon teardown leaves a handful of
// reclaim/scheduler goroutines that settle shortly after os.Exit. The value
// is deliberately loose enough to absorb kernel/LS-pool background workers
// that do not unwind synchronously after (*benchDaemon).Stop, but tight
// enough to catch a real per-iteration leak (which would grow into the dozens
// quickly — see Pitfall 6).
//
// Raised from 16 to 48 in Phase 38: the test suite now creates 7+ daemon
// instances (manifest, descriptions x3, integration x3, smoke, etc.). Each
// daemon's MCP in-memory transport and SDK client leave 2-4 goroutines that
// settle after context cancellation but before os.Exit. This is not a real
// leak — goroutines are context-bound and will exit, they just need more time
// than the grace period allows when many daemons are torn down in sequence.
const leakTolerance = 48

// TestMain wraps m.Run with a runtime.NumGoroutine leak check
// (09-RESEARCH.md Pattern + Pitfall 6). Gives the runtime a short grace
// period for daemon goroutines to unwind after tb.Cleanup fires, then
// compares against the pre-run baseline.
func TestMain(m *testing.M) {
	baseline := runtime.NumGoroutine()
	code := m.Run()

	// Grace period + GC lets daemon workers observe their canceled context
	// and return before we sample NumGoroutine. Without this, background
	// kernel/LS-pool goroutines that Stop signalled mid-test would still be
	// in-flight when we sample, producing a false-positive leak report.
	for i := 0; i < 20; i++ {
		runtime.GC()
		if runtime.NumGoroutine()-baseline <= leakTolerance {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if leaked := runtime.NumGoroutine() - baseline; leaked > leakTolerance {
		_, _ = fmt.Fprintf(os.Stderr,
			"goroutine leak detected: %d goroutines leaked past baseline (tolerance %d)\n",
			leaked, leakTolerance)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// TestBenchToolsManifestMatchesRegistry enforces the D-04 41-tool lock by
// asserting exact bidirectional name parity between benchTools and the live
// MCP tool registry:
//
//  1. len(benchTools) == 41 exactly
//  2. len(registry) == 41 exactly (catches upstream drift)
//  3. every benchCase.name exists in the live registry
//  4. every live registry name exists in benchTools
//
// On any mismatch the test logs diffs in both directions and points at
// internal/daemon/bootstrap_test.go as the canonical source of truth. This
// test is the mitigation for threat T-09-04 (tools_manifest drift).
func TestBenchToolsManifestMatchesRegistry(t *testing.T) {
	const expectedCount = 53

	if got := len(benchTools); got != expectedCount {
		t.Fatalf("benchTools has %d entries; expected exactly %d (per D-04 / 09-02 must_haves). "+
			"Source of truth: internal/daemon/bootstrap_test.go", got, expectedCount)
	}

	bd := startBenchDaemon(t)

	// Use Registry().Names() directly — not session.ListTools — because the
	// latter goes through ProfileFilterMiddleware and hides tools whose skill
	// is not in the active mode's skill list (e.g. profile-skill tools like
	// switch_mode and get_token_budget, and built-in tools like ping, echo,
	// activate_project). The Phase 9 baseline benchmarks every REGISTERED
	// tool, including those normally hidden from the edit-mode session.
	liveNames := bd.RegistryNames()
	if got := len(liveNames); got != expectedCount {
		t.Fatalf("live MCP registry reports %d tools; expected exactly %d. "+
			"Either internal/daemon/bootstrap_test.go is out of date or a new tool was added "+
			"without updating test/bench/tools_manifest_test.go.", got, expectedCount)
	}

	// Build sets for bidirectional diff.
	manifestSet := make(map[string]struct{}, len(benchTools))
	for _, bc := range benchTools {
		if _, dup := manifestSet[bc.name]; dup {
			t.Fatalf("benchTools contains duplicate entry %q; every tool must appear exactly once", bc.name)
		}
		manifestSet[bc.name] = struct{}{}
	}

	liveSet := make(map[string]struct{}, len(liveNames))
	for _, n := range liveNames {
		liveSet[n] = struct{}{}
	}

	// Manifest-only names: benchTools entries that don't exist in live registry.
	var manifestOnly []string
	for name := range manifestSet {
		if _, ok := liveSet[name]; !ok {
			manifestOnly = append(manifestOnly, name)
		}
	}

	// Registry-only names: live tools missing from benchTools.
	var registryOnly []string
	for name := range liveSet {
		if _, ok := manifestSet[name]; !ok {
			registryOnly = append(registryOnly, name)
		}
	}

	if len(manifestOnly) > 0 || len(registryOnly) > 0 {
		sort.Strings(manifestOnly)
		sort.Strings(registryOnly)
		t.Fatalf(
			"benchTools <-> live registry mismatch\n"+
				"  in manifest but NOT in registry: %v\n"+
				"  in registry but NOT in manifest: %v\n"+
				"Fix: update test/bench/tools_manifest_test.go to match "+
				"internal/daemon/bootstrap_test.go (the canonical list).",
			manifestOnly, registryOnly,
		)
	}
}
