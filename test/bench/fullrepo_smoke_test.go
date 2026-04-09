package bench_test

// fullrepo_smoke_test.go implements D-05 full-Serena-codebase indexing smoke:
// index the ~25,779-LOC repo root as a real-world scale validation of the LSP
// indexing path. This is intentionally separate from BenchmarkLSPIndex_Cold
// (which targets the small Go fixture) because repo-scale indexing has a long
// tail that would balloon PR CI into 10+ minute runs.
//
// Pitfall 11 (full-repo smoke long tail): gated behind testing.Short() so the
// default `go test -short -bench=. ./test/bench/...` PR sweep skips it. The
// benchmark is intended to run only on release tags via explicit invocation:
//
//	go test -bench=BenchmarkFullRepoSmoke -benchtime=1x -run=^$ ./test/bench/...
//
// Threat T-09-08 (CI runtime DoS) is mitigated by this short-gate.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot walks up from this source file to the Serena repository root by
// locating the parent directory containing go.mod. Panics (via tb.Fatalf) if
// no go.mod is found, which would indicate the file was moved out of
// test/bench/ without updating this helper.
func repoRoot(tb testing.TB) string {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatalf("runtime.Caller(0) failed: cannot determine repo root")
	}
	// file is .../test/bench/fullrepo_smoke_test.go -> walk up to repo root.
	dir := filepath.Dir(file)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	tb.Fatalf("repoRoot: could not locate go.mod walking up from %s", file)
	return ""
}

// BenchmarkFullRepoSmoke indexes the full Serena codebase (~25,779 LOC per
// D-05) as a real-world scale validation. Short-gated per Pitfall 11 to keep
// PR CI under budget — each iteration takes 15-30s because gopls must load
// and type-check the entire module graph. Run manually with
// `go test -bench=FullRepo -benchtime=1x ./test/bench/...` on release tags
// only.
func BenchmarkFullRepoSmoke(b *testing.B) {
	if testing.Short() {
		b.Skip("full-repo smoke skipped in -short (Pitfall 11)")
	}
	requireGoplsB(b)
	root := repoRoot(b)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		bd := startBenchDaemon(b)
		b.StartTimer()

		activateWorkspaceB(b, bd, root)

		b.StopTimer()
		bd.Stop()
		b.StartTimer()
	}
}
