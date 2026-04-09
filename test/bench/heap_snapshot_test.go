package bench_test

// heap_snapshot_test.go provides the heapSnapshot helper used by
// BenchmarkMemory (Plan 09-05) to write gzipped pprof heap profiles to
// test/bench/pprof/ at scenario boundaries per D-07/D-08 and
// 09-RESEARCH.md Pattern 5.
//
// File suffix note (deviation vs 09-05-PLAN.md): the plan spelled this file
// `heap_snapshot.go`, but the enclosing package is `bench_test`, which Go
// only permits in `_test.go` files (a non-test `*.go` file cannot declare
// `package bench_test`). Renaming to `heap_snapshot_test.go` is a Rule 3
// fix — the helper is only used by BenchmarkMemory, so living in a test
// file has zero observable effect on the compiled binary.
//
// Pitfall 8: snapshots under test/bench/pprof/*.pb.gz are git-ignored
// except for intentional `baseline-*.pb.gz` re-baseline commits. See
// test/bench/pprof/.gitignore and the comment block at the top of
// memory_bench_test.go.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
)

// gitShortSHA returns the short git SHA of HEAD, or the literal string
// "nogit" if git is unavailable or the command fails. Benchmarks must still
// run in git-less CI caches, so this never fatals the test (Pattern 5 +
// Pitfall 8 tolerate a missing SHA by degrading the filename).
func gitShortSHA(tb testing.TB) string {
	tb.Helper()
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "nogit"
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "nogit"
	}
	return sha
}

// heapSnapshot writes a gzipped pprof heap profile to
// test/bench/pprof/{scenario}-{sha}.pb.gz per D-07/D-08. Calls runtime.GC()
// first per Pattern 5 to stabilize short-lived allocations before sampling.
//
// Pitfall 8: the resulting files are git-ignored; committed baseline
// snapshots are a release-time concern handled by Plan 09-06 via
// `baseline-*.pb.gz` filenames that the .gitignore explicitly un-excludes.
func heapSnapshot(tb testing.TB, scenario string) {
	tb.Helper()
	runtime.GC()
	sha := gitShortSHA(tb)
	dir := filepath.Join("pprof")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatalf("mkdir pprof: %v", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.pb.gz", scenario, sha))
	f, err := os.Create(path)
	if err != nil {
		tb.Fatalf("create heap snapshot: %v", err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		tb.Fatalf("write heap profile: %v", err)
	}
	tb.Logf("heap snapshot: %s", path)
}
