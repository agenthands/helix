package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a tiny helper that drops a string into a fresh tempdir file
// and returns the absolute path. Benchgate tests operate on synthetic
// benchfmt input so they are fast, hermetic, and free of dependencies on
// real benchmark runs.
func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// benchfmtHeader is the leading stanza every synthetic fixture needs so
// benchfmt's Reader accepts it as a well-formed file.
const benchfmtHeader = `goos: linux
goarch: amd64
pkg: github.com/postfix/serena/test/bench
cpu: Synthetic
`

// bench generates N samples of a single benchmark line at the given
// ns/op and allocs/op so we can exercise Welch's t-test with a controlled
// mean and variance. jitter applies to BOTH time and allocs via a small
// rotating sequence so no sample degenerates to zero variance (benchmath
// cannot compute a t-test on a truly constant sample).
func bench(name string, count int, nsPerOp, allocsPerOp, jitter float64) string {
	var b strings.Builder
	// pattern rotates through +1, -1, +2, -2 scaled by jitter to avoid a
	// perfectly alternating +/- pair which reduces effective variance
	// when count is small.
	pattern := []float64{1, -1, 2, -2, 0.5, -0.5, 1.5, -1.5, 0.25, -0.25}
	for i := 0; i < count; i++ {
		k := pattern[i%len(pattern)]
		ns := nsPerOp * (1 + k*jitter)
		al := allocsPerOp * (1 + k*jitter)
		fmt.Fprintf(&b, "%s 1000 %.2f ns/op %.4f allocs/op\n", name, ns, al)
	}
	return b.String()
}

// runCLI invokes benchgate's run() with captured stdout/stderr and
// returns (exitCode, stdout, stderr) for assertions.
func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestNoRegression(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_010_000, 10, 0.01))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s", code, out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK summary, got: %s", out)
	}
}

func TestTimeRegressionGatedAndSignificant(t *testing.T) {
	dir := t.TempDir()
	// Baseline around 1,000,000 ns/op with tight variance.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.005))
	// New run: 20% slower with tight variance → clearly significant.
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_200_000, 10, 0.005))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 1 {
		t.Fatalf("expected exit 1 (blocking breach), got %d; stdout=%s", code, out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected FAIL summary, got: %s", out)
	}
	if !strings.Contains(out, "sec/op") {
		t.Errorf("expected sec/op breach line, got: %s", out)
	}
}

func TestTimeRegressionBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	// Only 10% regression — below the 15% PR threshold.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.005))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_100_000, 10, 0.005))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s", code, out)
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("expected no FAIL, got: %s", out)
	}
}

func TestTimeRegressionAboveThresholdButNotSignificant(t *testing.T) {
	dir := t.TempDir()
	// Means are 20% apart but variance is so large the t-test cannot
	// reject the null hypothesis at p<0.05.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.50))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_200_000, 10, 0.50))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 0 {
		t.Fatalf("expected exit 0 (delta exceeds threshold but p >= alpha), got %d; stdout=%s", code, out)
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("expected no FAIL (high variance suppressed the gate), got: %s", out)
	}
}

func TestAllocsRegression(t *testing.T) {
	dir := t.TempDir()
	// Same time, but allocs jumps from 10 to 14 (40% > 25% PR threshold).
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.001))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 14, 0.001))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 1 {
		t.Fatalf("expected exit 1 from allocs breach, got %d; stdout=%s", code, out)
	}
	if !strings.Contains(out, "allocs/op") {
		t.Errorf("expected allocs/op breach line, got: %s", out)
	}
}

func TestBenchOnlyInBaseline(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.txt", benchfmtHeader+
		bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01)+
		bench("BenchmarkTools/dropped-4", 10, 1_000_000, 10, 0.01))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+
		bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 0 {
		t.Fatalf("expected exit 0 (missing-in-new is a warning, not a breach), got %d; stdout=%s", code, out)
	}
	if !strings.Contains(out, "not present in new run") {
		t.Errorf("expected warning about missing-in-new, got: %s", out)
	}
}

func TestBenchOnlyInNew(t *testing.T) {
	dir := t.TempDir()
	base := writeFile(t, dir, "base.txt", benchfmtHeader+
		bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+
		bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01)+
		bench("BenchmarkTools/brand_new-4", 10, 1_000_000, 10, 0.01))
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s", code, out)
	}
	if !strings.Contains(out, "not present in baseline") {
		t.Errorf("expected warning about new bench, got: %s", out)
	}
}

func TestReleaseTierThresholdOverride(t *testing.T) {
	dir := t.TempDir()
	// 12% regression: allowed under PR tier (15%), blocked under
	// release tier (10%). Tight variance ensures p < 0.05.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.005))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_120_000, 10, 0.005))

	// PR tier — allowed.
	if code, _, _ := runCLI("--baseline", base, "--new", neu); code != 0 {
		t.Fatalf("PR tier: expected 0, got %d", code)
	}
	// Release tier — blocked.
	if code, _, _ := runCLI("--baseline", base, "--new", neu, "--release-tier"); code != 1 {
		t.Fatalf("release tier: expected 1, got %d", code)
	}
}

func TestMalformedInputExitsTwo(t *testing.T) {
	dir := t.TempDir()
	// benchfmt only treats lines beginning with "Benchmark" as result
	// records. We therefore craft a syntactically broken result line so
	// the reader emits a SyntaxError, which benchgate converts to exit 2.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+"BenchmarkBroken-4 notanumber garbage ns/op\n")
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01))
	code, _, _ := runCLI("--baseline", base, "--new", neu)
	if code != 2 {
		t.Fatalf("expected exit 2 on malformed input, got %d", code)
	}
}

func TestMissingBaselineExitsTwo(t *testing.T) {
	dir := t.TempDir()
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.01))
	code, _, _ := runCLI("--baseline", filepath.Join(dir, "does-not-exist.txt"), "--new", neu)
	if code != 2 {
		t.Fatalf("expected exit 2 on missing baseline, got %d", code)
	}
}

func TestWarnOnlyAlwaysExitsZero(t *testing.T) {
	dir := t.TempDir()
	// Clear 20% regression, tight variance — would normally exit 1.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.005))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_200_000, 10, 0.005))
	code, out, _ := runCLI("--baseline", base, "--new", neu, "--warn-only")
	if code != 0 {
		t.Fatalf("expected exit 0 in warn-only mode, got %d; stdout=%s", code, out)
	}
	if !strings.Contains(out, "WARN-ONLY") {
		t.Errorf("expected WARN-ONLY banner, got: %s", out)
	}
	// The breach should still be reported (warn != silence).
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected breach still reported, got: %s", out)
	}
}

func TestCustomAlphaOverride(t *testing.T) {
	dir := t.TempDir()
	// Alpha is a gating knob. To verify it, we use a single fixture
	// with a clear delta (20%) above the 15% PR threshold and enough
	// samples that the p-value is ~0.04 with the default alpha=0.05 —
	// i.e. borderline. Then strict alpha (0.001) must flip the result
	// to "not significant" and return 0.
	//
	// Rather than hand-tune noise, we compare the alpha knob
	// independently from the time-threshold knob: hold the delta
	// constant at 20% with moderate variance, and show that a very
	// strict alpha (far below any p-value benchmath can produce here)
	// makes the gate pass. The relaxed-alpha branch is covered by the
	// default-alpha tests above and is omitted here to avoid brittle
	// variance tuning.
	base := writeFile(t, dir, "base.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_000_000, 10, 0.30))
	neu := writeFile(t, dir, "new.txt", benchfmtHeader+bench("BenchmarkTools/read_file-4", 10, 1_200_000, 10, 0.30))

	// Very strict alpha — the sample noise is high enough that p-value
	// is well above 1e-9, so the gate must pass.
	if code, _, _ := runCLI("--baseline", base, "--new", neu, "--alpha", "1e-9"); code != 0 {
		t.Fatalf("very strict alpha: expected 0 (not significant), got %d", code)
	}
	// Very relaxed alpha — any p < 1 counts as significant, so the
	// 20% delta (> 15% threshold) must fail the gate.
	if code, _, _ := runCLI("--baseline", base, "--new", neu, "--alpha", "0.99"); code != 1 {
		t.Fatalf("relaxed alpha: expected 1 (significant), got %d", code)
	}
}

func TestAllBreachesReported(t *testing.T) {
	dir := t.TempDir()
	// Three regressing benches — all must show up in the report.
	baseBody := benchfmtHeader +
		bench("BenchmarkA-4", 10, 1_000_000, 10, 0.005) +
		bench("BenchmarkB-4", 10, 2_000_000, 10, 0.005) +
		bench("BenchmarkC-4", 10, 3_000_000, 10, 0.005)
	newBody := benchfmtHeader +
		bench("BenchmarkA-4", 10, 1_300_000, 10, 0.005) +
		bench("BenchmarkB-4", 10, 2_500_000, 10, 0.005) +
		bench("BenchmarkC-4", 10, 3_600_000, 10, 0.005)
	base := writeFile(t, dir, "base.txt", baseBody)
	neu := writeFile(t, dir, "new.txt", newBody)
	code, out, _ := runCLI("--baseline", base, "--new", neu)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d; stdout=%s", code, out)
	}
	// benchfmt strips the "Benchmark" prefix from Name.Full(), so the
	// rendered rows show "A-4", "B-4", "C-4".
	for _, n := range []string{"A-4", "B-4", "C-4"} {
		if !strings.Contains(out, n) {
			t.Errorf("expected all three breaches in report, missing %s: %s", n, out)
		}
	}
}
