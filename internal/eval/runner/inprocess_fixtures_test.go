package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/score"
)

// TestRunQuickFullFixtureSetWallTime verifies that the 10-fixture eval-quick run
// completes within D-05's 30s wall-time budget across all 4 modes.
// T-67-Pitfall-7 mitigation: this test is the CI gate for the budget constraint.
func TestRunQuickFullFixtureSetWallTime(t *testing.T) {
	if testing.Short() {
		t.Skip("TestRunQuickFullFixtureSetWallTime: skipping in -short mode (CI resource constraint)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	outDir := t.TempDir()
	opts := runner.QuickOpts{
		FixturesDir: quickFixturesDir(t),
		Modes:       defaultModes(),
		OutDir:      outDir,
		RunID:       "test-full-wall-time",
	}

	start := time.Now()
	summary, err := runner.RunQuick(ctx, opts)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunQuick: %v", err)
	}

	// Assert 10 fixtures found.
	const wantFixtures = 10
	if summary.TotalFixtures != wantFixtures {
		t.Errorf("TotalFixtures = %d, want %d (D-05 requires ~10 fixtures)", summary.TotalFixtures, wantFixtures)
	}

	// Assert 10 fixtures × 4 modes = 40 results.
	wantResults := wantFixtures * len(defaultModes())
	if summary.TotalResults != wantResults {
		t.Errorf("TotalResults = %d, want %d", summary.TotalResults, wantResults)
	}

	// D-05 hard budget: 30s wall-time for 10-fixture set × 4 modes.
	// Allow 10% slack to tolerate CI jitter: effective gate is 33s.
	const budget = 30 * time.Second
	const slack = budget / 10
	if elapsed > budget+slack {
		t.Errorf("wall-time %v exceeds D-05 budget %v + 10%% slack %v", elapsed, budget, slack)
	} else if elapsed > budget {
		t.Logf("WARN: elapsed %v exceeds %v but within slack — check fixture sizing", elapsed, budget)
	} else {
		t.Logf("wall-time %v (budget %v) — OK", elapsed, budget)
	}
}

// TestRunQuickFullFixtureSetCoversAllFamilies verifies that the 10-fixture set
// covers all 5 EVAL-05 families (rename, delete, public_api, large_edit, security)
// and includes Go, TypeScript, and Python language coverage.
// T-67-Pitfall-8 mitigation: deletion of any family fails this test.
func TestRunQuickFullFixtureSetCoversAllFamilies(t *testing.T) {
	fixturesRoot := quickFixturesDir(t)

	// Collect task_kind values from all expected_tools.yaml files.
	seenFamilies := make(map[string]bool)
	seenExtensions := make(map[string]bool)

	pattern := filepath.Join(fixturesRoot, "quick-*")
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob %q: %v", pattern, err)
	}
	if len(dirs) == 0 {
		t.Fatalf("no quick-* fixture directories found under %s", fixturesRoot)
	}

	for _, dir := range dirs {
		// Load expected_tools.yaml to extract task_kind.
		rulesPath := filepath.Join(dir, "expected_tools.yaml")
		rules, err := score.LoadRules(rulesPath)
		if err != nil {
			t.Errorf("LoadRules(%q): %v", rulesPath, err)
			continue
		}
		if rules.TaskKind == "" {
			t.Errorf("fixture %q: task_kind is empty in expected_tools.yaml", filepath.Base(dir))
		}
		seenFamilies[rules.TaskKind] = true

		// Scan repo/ directory for file extensions to assert language coverage.
		repoDir := filepath.Join(dir, "repo")
		if _, err := os.Stat(repoDir); err != nil {
			continue
		}
		_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != "" {
				seenExtensions[ext] = true
			}
			return nil
		})
	}

	// Assert all 5 EVAL-05 families present.
	requiredFamilies := []string{"rename", "delete", "public_api", "large_edit", "security"}
	for _, fam := range requiredFamilies {
		if !seenFamilies[fam] {
			t.Errorf("EVAL-05 family %q is missing from the fixture set", fam)
		}
	}

	// Assert language coverage: Go (.go), TypeScript (.ts), Python (.py).
	requiredExtensions := []string{".go", ".ts", ".py"}
	for _, ext := range requiredExtensions {
		if !seenExtensions[ext] {
			t.Errorf("language coverage missing: no %s files found in fixture repos", ext)
		}
	}

	t.Logf("families seen: %v", seenFamilies)
	t.Logf("extensions seen: %v", seenExtensions)
}

// TestQuickSecurityFixtureLoadable verifies that the quick-security-001 fixture
// loads its rules and script without error.
func TestQuickSecurityFixtureLoadable(t *testing.T) {
	fixDir := filepath.Join(quickFixturesDir(t), "quick-security-001")

	// Validate rules file.
	rulesPath := filepath.Join(fixDir, "expected_tools.yaml")
	if _, err := score.LoadRules(rulesPath); err != nil {
		t.Fatalf("score.LoadRules(%q): %v", rulesPath, err)
	}

	// Validate script file.
	scriptPath := filepath.Join(fixDir, "scripted_agent.yaml")
	script, err := runner.LoadScript(scriptPath)
	if err != nil {
		t.Fatalf("runner.LoadScript(%q): %v", scriptPath, err)
	}
	if len(script.Steps) == 0 {
		t.Error("expected at least one step in quick-security-001 scripted_agent.yaml")
	}
}
