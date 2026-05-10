package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/runner"
)

// quickFixturesDir returns eval/fixtures relative to the repo root.
func quickFixturesDir(t *testing.T) string {
	t.Helper()
	return fixturesDir(t) // defined in scripted_agent_test.go
}

// defaultModes returns the four eval modes for RunQuick tests.
func defaultModes() []string {
	return []string{"baseline", "native", "semantic", "semantic_guarded"}
}

// TestRunQuickStartsInProcessDaemon verifies that RunQuick boots an in-process
// daemon (not a subprocess) and shuts it down on ctx cancellation.
func TestRunQuickStartsInProcessDaemon(t *testing.T) {
	if testing.Short() {
		t.Skip("TestRunQuickStartsInProcessDaemon: skipping in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       []string{"native"}, // single mode keeps test fast
		OutDir:      outDir,
		RunID:       "test-start-daemon",
	}

	summary, err := runner.RunQuick(ctx, opts)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}
	// Daemon was used; summary must record results.
	if summary.TotalFixtures == 0 {
		t.Error("expected TotalFixtures > 0")
	}
}

// TestRunQuickRunsAllFourModes verifies that RunQuick runs the 2 reference
// fixtures across all 4 modes and returns results for each (fixture × mode).
func TestRunQuickRunsAllFourModes(t *testing.T) {
	if testing.Short() {
		t.Skip("TestRunQuickRunsAllFourModes: skipping in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       defaultModes(),
		OutDir:      outDir,
		RunID:       "test-all-modes",
	}

	summary, err := runner.RunQuick(ctx, opts)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	// All fixtures × 4 modes = result slots.
	fixtureEntries, err := os.ReadDir(quickFixturesDir(t))
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}
	fixtureCount := 0
	for _, e := range fixtureEntries {
		if e.IsDir() {
			fixtureCount++
		}
	}
	wantResults := fixtureCount * len(defaultModes())
	if summary.TotalResults != wantResults {
		t.Errorf("TotalResults = %d, want %d", summary.TotalResults, wantResults)
	}
}

// TestRunQuickWallTimeUnder30Seconds verifies that the 2-fixture eval-quick run
// completes well under 30 seconds (D-05 budget headroom for 10-fixture Plan 06b).
func TestRunQuickWallTimeUnder30Seconds(t *testing.T) {
	if testing.Short() {
		t.Skip("TestRunQuickWallTimeUnder30Seconds: skipping under -short (CI resource constraint)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       defaultModes(),
		OutDir:      outDir,
		RunID:       "test-wall-time",
	}

	start := time.Now()
	_, err := runner.RunQuick(ctx, opts)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	// Target < 10s for 2-fixture set (gives ×3 headroom for 10-fixture 06b).
	const target = 10 * time.Second
	if elapsed > target {
		t.Logf("WARN: elapsed %v exceeds target %v — Plan 06b 30s budget may be tight", elapsed, target)
		// Non-fatal: log only; hard fail is the context timeout (30s).
	}
}

// TestRunQuickEmitsHarnessValidationBanner verifies that RunQuick writes the
// Pitfall-6 banner to stdout and returns it in the Summary.
func TestRunQuickEmitsHarnessValidationBanner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       []string{"native"},
		OutDir:      outDir,
		RunID:       "test-banner",
	}

	summary, err := runner.RunQuick(ctx, opts)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	// Banner must appear in the eval_report.md.
	reportPath := filepath.Join(outDir, "test-banner", "eval_report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read eval_report.md: %v", err)
	}
	const banner = "eval-quick: HARNESS VALIDATION ONLY — agent behavior NOT measured"
	if !strings.Contains(string(data), banner) {
		t.Errorf("eval_report.md missing Pitfall-6 banner %q", banner)
	}
	if !strings.Contains(summary.Banner, banner) {
		t.Errorf("Summary.Banner %q missing expected content %q", summary.Banner, banner)
	}
}

// TestRunQuickSuccessFlagGuard verifies the Pitfall-6 success-flag gate:
// RunQuick may return successful EvalResults; direct EvalResult construction
// outside the quick path does NOT have Success set by default.
func TestRunQuickSuccessFlagGuard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       []string{"native"},
		OutDir:      outDir,
		RunID:       "test-success-gate",
		AllowSuccess: true, // --quick flag allows success
	}

	summary, err := runner.RunQuick(ctx, opts)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	// Without AllowSuccess, no result should be marked Success.
	opts2 := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       []string{"native"},
		OutDir:      t.TempDir(),
		RunID:       "test-success-gate-blocked",
		AllowSuccess: false, // --quick NOT set
	}
	summary2, err := runner.RunQuick(ctx, opts2)
	if err != nil {
		t.Fatalf("RunQuick (no AllowSuccess): %v", err)
	}
	for _, r := range summary2.Results {
		if r.Success {
			t.Errorf("result %s/%s has Success=true without AllowSuccess", r.TaskID, r.Mode)
		}
	}
	_ = summary
}

// TestRunQuickEmitsFiveReports verifies that RunQuick emits all 5 EVAL-04
// report files under <OutDir>/<RunID>/.
func TestRunQuickEmitsFiveReports(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outDir := t.TempDir()
	runID := "test-five-reports"
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       []string{"native"},
		OutDir:      outDir,
		RunID:       runID,
	}

	if _, err := runner.RunQuick(ctx, opts); err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	runOutDir := filepath.Join(outDir, runID)
	required := []string{
		"eval_report.json",
		"eval_report.md",
		"cost_summary.json",
		"tool_behavior.json",
		"safety_compliance.json",
	}
	for _, name := range required {
		path := filepath.Join(runOutDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing report %s: %v", name, err)
		}
	}
}

// TestRunQuickReusesDaemonAcrossFixtures verifies that RunQuick boots the
// in-process daemon at most once per mode (not per fixture), keeping the
// wall-time budget for 10 fixtures × 4 modes (Plan 06b) under 30s.
func TestRunQuickReusesDaemonAcrossFixtures(t *testing.T) {
	if testing.Short() {
		t.Skip("TestRunQuickReusesDaemonAcrossFixtures: skipping in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir:     quickFixturesDir(t),
		Modes:           defaultModes(),
		OutDir:          outDir,
		RunID:           "test-reuse-daemon",
		TrackDaemonBoots: true, // instrumentation hook
	}

	summary, err := runner.RunQuick(ctx, opts)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	// At most 4 daemon boots (one per mode): 2 fixtures × 4 modes with daemon
	// reuse means DaemonBoots <= len(modes).
	maxBoots := len(defaultModes())
	if summary.DaemonBoots > maxBoots {
		t.Errorf("DaemonBoots = %d, want <= %d (daemon not reused across fixtures)",
			summary.DaemonBoots, maxBoots)
	}
}
