package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	aiderpolyglot "github.com/agenthands/helix/bench/datasets/aider-polyglot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aider_edit_cell_test.go carries the Plan 02 (EDITBENCH-01 live half + BASELINE-01)
// behaviors for runAiderEditCell:
//
//   - TestAiderEditCellHermetic — the SOLE authoritative proof (Pitfall 1). It drives
//     the FULL aider-edit assembly path (LoadExercise → newEditAgentWithApply applies
//     the reference body in-process → RunExercise VERBATIM with the live native go
//     TestFn → grade → BuildResult+Validate) over the vendored go/wordy fixture with
//     NO HELIX_BIN, NO daemon, NO network. It asserts the produced result.v2 is
//     schema-valid, outcome=success, and carries edit_format_applied=true.
//   - TestAiderEditCellAntiTamper — anti-vacuity (revert-and-fail): feeding a WRONG
//     reference body (so the applied edit does NOT satisfy the tests) yields a
//     non-success outcome. The baseline pass is therefore non-vacuous.
//   - TestAiderEditCellLiveRan — the HELIX_BIN "did it RUN" sentinel (fail-CLOSED).
//     When HELIX_BIN is set it runs the REAL runAiderEditCell over go/wordy and
//     asserts a result.v2.json EXISTS, is schema-valid, and carries
//     edit_format_applied=true with a success outcome. A missing file / empty bucket
//     / missing edit_format_applied is a HARD failure, never a pass. When HELIX_BIN
//     is unset it t.Skip's ONLY because the hermetic sibling covers the logic.

// goWordyFixtureDir is the vendored hermetic go/wordy exercise dir (Phase 99).
func goWordyFixtureDir(t *testing.T) string {
	t.Helper()
	// The bench/runtime package dir is the test CWD; the fixtures live under the
	// sibling bench/datasets tree.
	dir := filepath.Join("..", "datasets", "aider-polyglot", "fixtures", "go",
		"exercises", "practice", "wordy")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err, "resolve go/wordy fixture dir")
	if _, err := os.Stat(filepath.Join(abs, ".meta", "config.json")); err != nil {
		t.Fatalf("go/wordy fixture missing at %s: %v", abs, err)
	}
	return abs
}

// cloneFixtureTree copies the whole fixture exercise tree into a per-test work dir
// (the hermetic analog of sb.CloneRepo) so RunExercise edits a working copy while
// the pristine SrcDir stays the fixture dir (WR-01 reads pristine from SrcDir).
func cloneFixtureTree(t *testing.T, srcDir string) string {
	t.Helper()
	work := t.TempDir()
	err := filepath.Walk(srcDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(srcDir, p)
		if rerr != nil {
			return rerr
		}
		dst := filepath.Join(work, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if merr := os.MkdirAll(filepath.Dir(dst), 0o755); merr != nil {
			return merr
		}
		return os.WriteFile(dst, b, 0o644)
	})
	require.NoError(t, err, "clone fixture tree")
	return work
}

// TestAiderEditCellHermetic is the SOLE authoritative proof. It drives the aider-edit
// assembly path end-to-end against the vendored go/wordy fixture with an in-process
// apply seam (no daemon / HELIX_BIN / network) and asserts a schema-valid result.v2
// carrying outcome=success + edit_format_applied=true.
func TestAiderEditCellHermetic(t *testing.T) {
	if testing.Short() {
		t.Skip("hermetic aider-edit cell runs go test on the fixture; skipped in -short")
	}
	srcDir := goWordyFixtureDir(t)
	workDir := cloneFixtureTree(t, srcDir)

	ex, err := aiderpolyglot.LoadExercise(srcDir, "go")
	require.NoError(t, err, "load go/wordy")

	// In-process apply seam: write the reference body straight into the work-dir stub.
	applied := false
	agentFn := newEditAgentWithApply(&applied, func(_ context.Context, wd, stub, body string) error {
		return os.WriteFile(filepath.Join(wd, stub), []byte(body), 0o644)
	})
	testFn := newNativeTestFn()

	res := aiderpolyglot.RunExercise(context.Background(), ex, workDir, testFn, agentFn)
	require.True(t, res.Passed, "go/wordy reference solution must pass the native tests; last output:\n%s", res.LastOutput)
	require.True(t, applied, "edit_format_applied must be true after a successful apply")

	// Assemble the result.v2 the cell would persist, then assert schema validity and
	// the carried open key. This exercises the SAME helper the live cell uses.
	resultBytes, err := assembleAiderEditResult(aiderEditResultInput{
		Task:     ex.Name,
		Language: "go",
		Passed:   res.Passed,
		Applied:  applied,
	})
	require.NoError(t, err, "assemble aider-edit result")
	require.NoError(t, Validate(resultBytes), "committed-shape result.v2 must be schema-valid")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))
	assert.Equal(t, "success", doc["outcome"], "outcome must be success on the reference solution")
	assert.Equal(t, true, doc["edit_format_applied"], "edit_format_applied must be present and true")
	assert.Equal(t, "aider_edit", doc["mode"])
	assert.Equal(t, "go", doc["language"])
	assert.Equal(t, "wordy", doc["task_id"])
}

// TestAiderEditCellAntiTamper proves the baseline pass is non-vacuous: a WRONG
// reference body (a stub that compiles but fails the cases) yields outcome != success
// even though the edit format WAS applied.
func TestAiderEditCellAntiTamper(t *testing.T) {
	if testing.Short() {
		t.Skip("anti-tamper runs go test on the fixture; skipped in -short")
	}
	srcDir := goWordyFixtureDir(t)
	workDir := cloneFixtureTree(t, srcDir)

	ex, err := aiderpolyglot.LoadExercise(srcDir, "go")
	require.NoError(t, err, "load go/wordy")

	// A WRONG body: a compiling but incorrect Answer implementation. The package name
	// must match so it compiles; the behavior is deliberately wrong so cases_test fails.
	wrong := "package wordy\n\nfunc Answer(q string) (int, bool) { return 0, false }\n"
	applied := false
	agentFn := newEditAgentWithApply(&applied, func(_ context.Context, wd, stub, _ string) error {
		return os.WriteFile(filepath.Join(wd, stub), []byte(wrong), 0o644)
	})
	testFn := newNativeTestFn()

	res := aiderpolyglot.RunExercise(context.Background(), ex, workDir, testFn, agentFn)
	require.False(t, res.Passed, "a wrong (but compiling) solution must NOT pass the native tests")
	require.True(t, applied, "edit_format_applied records apply-success even when the graded outcome is a fail")

	resultBytes, err := assembleAiderEditResult(aiderEditResultInput{
		Task:     ex.Name,
		Language: "go",
		Passed:   res.Passed,
		Applied:  applied,
	})
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))
	assert.NotEqual(t, "success", doc["outcome"], "a wrong edit must not grade as success (non-vacuous)")
	assert.Equal(t, true, doc["edit_format_applied"], "edit_format_applied stays true: apply succeeded, grade failed")
}

// TestAiderEditBaselineIsGitIndependent (WR-01 regression): the deterministic baseline
// assembly MUST NOT invoke git. It asserts (a) the assembled bytes are byte-identical
// whether or not git is on PATH, and (b) the bytes never embed an environment-specific
// git error string (e.g. "git ... not found in $PATH"). Before the fix,
// assembleAiderEditResult called coordinator.Grade with RepoDir:"" and ran the
// patch_validator git graders against the regenerator's CWD, so a host without git
// would leak a machine-specific error into the committed metric_errors and break
// byte-reproducibility. The fix skips the git graders entirely (SkipPatchValidator).
func TestAiderEditBaselineIsGitIndependent(t *testing.T) {
	in := aiderEditResultInput{Task: "wordy", Language: "go", Passed: true, Applied: true}

	withGit, err := assembleAiderEditResult(in)
	require.NoError(t, err, "assemble with git on PATH")

	// Strip git (and everything else) from PATH so any git invocation would FAIL with
	// an env-specific "executable file not found" error. If the bytes are unchanged,
	// no git was spawned.
	t.Setenv("PATH", "")
	noGit, err := assembleAiderEditResult(in)
	require.NoError(t, err, "assemble with PATH stripped of git")

	require.Equal(t, string(withGit), string(noGit),
		"baseline bytes diverged when git was removed from PATH — the deterministic path is still invoking git (WR-01)")

	// Belt-and-suspenders: the committed bytes must never carry a git error string.
	body := string(noGit)
	for _, needle := range []string{"git ls-files", "git diff", "executable file not found", "git-unavailable"} {
		assert.NotContains(t, body, needle,
			"baseline result.v2 embeds a git-derived error string (%q) — non-reproducible across hosts (WR-01)", needle)
	}

	// The only patch_validator metric_errors must be the 3 explicit baseline nulls,
	// each carrying the deterministic exclusion reason (never a git error reason).
	var doc struct {
		MetricErrors []struct {
			Metric string `json:"metric"`
			Grader string `json:"grader"`
			Reason string `json:"reason"`
		} `json:"metric_errors"`
	}
	require.NoError(t, json.Unmarshal(noGit, &doc))
	pvMetrics := map[string]bool{}
	for _, e := range doc.MetricErrors {
		if e.Grader != "patch_validator" {
			continue
		}
		pvMetrics[e.Metric] = true
		assert.Contains(t, e.Reason, "excluded from the deterministic committed baseline",
			"patch_validator metric_error %q carries a non-deterministic reason: %q", e.Metric, e.Reason)
	}
	assert.Equal(t, map[string]bool{
		"files_modified":      true,
		"edit_locality":       true,
		"edit_distance_patch": true,
	}, pvMetrics, "exactly the 3 explicit patch_validator baseline nulls must be present")
}

// TestAiderEditCellConflictingBenchmark (WR-03 regression): runAiderEditCell honors
// cfg.Benchmark and fails CLOSED when a caller passes a benchmark that conflicts with
// the aider-polyglot suite, rather than silently stamping the const and emitting a row
// whose benchmark field disagrees with the requested config.
func TestAiderEditCellConflictingBenchmark(t *testing.T) {
	cfg := CellConfig{
		RunID:     "aider-edit-conflict",
		Benchmark: "some-other-suite", // conflicts with aiderEditBenchmark
		Language:  "go",
		Task:      "wordy",
		Mode:      aiderEditMode,
		HelixBin:  "/nonexistent/helix", // never reached: the guard returns first
		SeedDir:   t.TempDir(),
		OutDir:    t.TempDir(),
	}
	_, err := runAiderEditCell(context.Background(), cfg, CellResult{})
	require.Error(t, err, "a conflicting cfg.Benchmark must fail closed (WR-03)")
	assert.Contains(t, err.Error(), aiderEditBenchmark, "error must name the required benchmark")
	assert.Contains(t, err.Error(), "some-other-suite", "error must name the conflicting benchmark the caller passed")
}

// TestAiderEditCellLiveRan is the HELIX_BIN "did it RUN" sentinel (fail-CLOSED).
func TestAiderEditCellLiveRan(t *testing.T) {
	helixBin := os.Getenv("HELIX_BIN")
	if helixBin == "" {
		t.Skip("HELIX_BIN unset — the hermetic sibling (TestAiderEditCellHermetic) is the sole authoritative proof")
	}
	srcDir := goWordyFixtureDir(t)
	outDir := t.TempDir()

	cfg := CellConfig{
		RunID:       "aider-edit-live",
		Benchmark:   "aider-polyglot",
		Language:    "go",
		Task:        "wordy",
		Mode:        aiderEditMode,
		RunIndex:    0,
		HelixBin:    helixBin,
		SeedDir:     srcDir,
		OutDir:      outDir,
		RunnersRoot: filepath.Join("..", "runners"),
		Agent:       "scripted",
	}

	res, err := RunCell(context.Background(), cfg)
	require.NoError(t, err, "live aider-edit cell must run without an infra error")

	// Fail-CLOSED: the result.v2.json MUST exist and be readable. A missing file is a
	// HARD failure, never read as a zero/pass.
	resultBytes, rerr := os.ReadFile(res.ResultPath)
	require.NoError(t, rerr, "live cell MUST produce a result.v2.json at %s (fail-closed)", res.ResultPath)
	require.NotEmpty(t, resultBytes, "result.v2.json must not be empty (fail-closed)")
	require.NoError(t, Validate(resultBytes), "live result.v2 must be schema-valid")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))
	efa, present := doc["edit_format_applied"]
	require.True(t, present, "edit_format_applied MUST be present on a live aider-edit row (fail-closed)")
	assert.Equal(t, true, efa, "edit_format_applied must be true on the deterministic arm")
	assert.Equal(t, "success", doc["outcome"], "the reference solution must grade success on the live cell")
}
