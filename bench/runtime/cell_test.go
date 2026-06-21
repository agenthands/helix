package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenthands/helix/bench/languages"
	"github.com/agenthands/helix/bench/runners"
	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// benchRunnersRootForTest returns the absolute path of bench/runners (which
// holds your_agent_full/MODE.md) derived from this test file's location, so the
// mode resolver works regardless of the test's working directory.
func benchRunnersRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	// thisFile = .../bench/runtime/cell_test.go -> repo .../bench/runners
	benchDir := filepath.Dir(filepath.Dir(thisFile)) // .../bench
	root := filepath.Join(benchDir, "runners")
	require.DirExists(t, root)
	return root
}

// removeAllForTest removes a preserved scratch dir at the end of a test.
func removeAllForTest(path string) error { return os.RemoveAll(path) }

// TestCellLayout asserts the durable artifact layout RunCell ACTUALLY writes:
// <out>/<task>/<mode>/<run_index>/{result.v2.json,trace.json} (D-08, Pitfall 3),
// computed by the production cellDurablePaths helper RunCell calls. This is a
// UNIT test — no daemon.
func TestCellLayout(t *testing.T) {
	outDir := t.TempDir()

	const task = "IT-go-patch-apply-1"
	const mode = "your_agent_full"
	const runIndex = 0

	wantResult := filepath.Join(outDir, task, mode, "0", "result.v2.json")
	wantTrace := filepath.Join(outDir, task, mode, "0", "trace.json")

	gotResult, gotTrace, err := cellDurablePaths(outDir, task, mode, runIndex)
	require.NoError(t, err)

	assert.Equal(t, wantResult, gotResult,
		"result.v2.json must live at <out>/<task>/<mode>/<run_index>/result.v2.json (D-08, Pitfall 3)")
	assert.Equal(t, wantTrace, gotTrace,
		"merged trace must live at <out>/<task>/<mode>/<run_index>/trace.json (D-08, Pitfall 3)")
}

// TestCellLayoutPathTraversalRejected confirms RunCell rejects task/mode/
// benchmark names that would escape the out dir BEFORE any filesystem work
// (V5 / T-77-08). validateCellKey must fire before sandbox creation.
func TestCellLayoutPathTraversalRejected(t *testing.T) {
	cases := []struct {
		name string
		cfg  CellConfig
	}{
		{"task-traversal", CellConfig{Task: "../escape", Mode: "your_agent_full", Benchmark: "internal-toolbench", Language: "go", HelixBin: "/bin/true"}},
		{"mode-traversal", CellConfig{Task: "IT-go-patch-apply-1", Mode: "../escape", Benchmark: "internal-toolbench", Language: "go", HelixBin: "/bin/true"}},
		{"benchmark-traversal", CellConfig{Task: "IT-go-patch-apply-1", Mode: "your_agent_full", Benchmark: "../escape", Language: "go", HelixBin: "/bin/true"}},
		{"task-separator", CellConfig{Task: "a/b", Mode: "your_agent_full", Benchmark: "internal-toolbench", Language: "go", HelixBin: "/bin/true"}},
		{"language-traversal", CellConfig{Task: "IT-go-patch-apply-1", Mode: "your_agent_full", Benchmark: "internal-toolbench", Language: "../escape", HelixBin: "/bin/true"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunCell(context.Background(), tc.cfg)
			require.Error(t, err, "path-traversal cell key must be rejected before any FS access")
		})
	}
}

// TestCellLayoutPreserveOnFailure forces an infrastructure failure (a missing
// seed dir makes CloneRepo fail) and asserts the D-08 preserve-on-failure
// contract: the ephemeral scratch dir is PRESERVED (not cleaned up) and its path
// is surfaced on the returned CellResult. On the success path scratch would be
// removed; here it must remain.
func TestCellLayoutPreserveOnFailure(t *testing.T) {
	outDir := t.TempDir()
	cfg := CellConfig{
		RunID:       "20060102T150405Z",
		Benchmark:   "internal-toolbench",
		Language:    "go",
		Task:        "IT-go-patch-apply-1",
		Mode:        "your_agent_full",
		HelixBin:    "/bin/true", // never spawned — CloneRepo fails first
		SeedDir:     filepath.Join(t.TempDir(), "does-not-exist"),
		OutDir:      outDir,
		RunnersRoot: benchRunnersRootForTest(t),
	}

	res, err := RunCell(context.Background(), cfg)
	require.Error(t, err, "RunCell must error when the seed dir is missing")
	require.True(t, res.ScratchPreserved, "scratch must be PRESERVED on failure (D-08)")
	require.NotEmpty(t, res.ScratchDir, "preserved scratch path must be surfaced")

	// The preserved scratch dir must still exist on disk (Cleanup not called).
	assert.DirExists(t, res.ScratchDir,
		"preserve-on-failure: the embedded Cleanup() must NOT have run")

	// Clean it up ourselves so the test leaves no /tmp residue.
	t.Cleanup(func() { _ = removeAllForTest(res.ScratchDir) })
}

// TestRunOneCellLangSeedJoin asserts the seed dir a Cell with Language=="go"
// resolves to is <root>/internal-toolbench/go/<task> (D-07 <lang> seed join).
func TestRunOneCellLangSeedJoin(t *testing.T) {
	c := Cell{
		Benchmark: "internal-toolbench",
		Language:  "go",
		Mode:      "your_agent_full",
		Task:      "IT-go-patch-apply-1",
	}
	got := cellSeedDir("/data", c)
	want := filepath.Join("/data", "internal-toolbench", "go", "IT-go-patch-apply-1")
	assert.Equal(t, want, got, "seed dir must include the <lang> segment (D-07)")
}

// TestWriteCellConfigStoreOptIn asserts writeCellConfig parameterizes
// semantic_index.enabled from storeOptIn (D-01/D-02): false -> enabled: false,
// true -> enabled: true.
func TestWriteCellConfigStoreOptIn(t *testing.T) {
	outDir := t.TempDir()
	sb, err := benchsandbox.New("20060102T150405Z", "/nonexistent/helix", outDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Cleanup() })

	const task = "IT-go-patch-apply-1"
	const mode = "your_agent_full"
	require.NoError(t, sb.Prepare(task, mode))

	t.Run("store-off", func(t *testing.T) {
		p, err := writeCellConfig(sb, task, mode, "bench-full", false)
		require.NoError(t, err)
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Contains(t, string(b), "enabled: false", "storeOptIn=false must disable the index")
	})

	t.Run("store-on", func(t *testing.T) {
		p, err := writeCellConfig(sb, task, mode, "bench-full", true)
		require.NoError(t, err)
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Contains(t, string(b), "enabled: true", "storeOptIn=true must enable the index")
	})
}

// TestCellRunnerDispatch asserts the D-10 dispatch wiring: a (benchmark, lang)
// with a registered runner resolves to that runner; an unregistered pair returns
// nil (the verify.sh-fallback signal). The Go runner is registered for
// (internal-toolbench, go) via Plan 01's init().
func TestCellRunnerDispatch(t *testing.T) {
	// Registered: the Go runner dispatches structured RunTests.
	if r := languages.RunnerFor("internal-toolbench", "go"); r == nil {
		t.Fatalf("RunnerFor(internal-toolbench, go) = nil; want the Go runner (Plan 01 registration)")
	}
	// Unregistered: nil -> RunCell falls back to runVerify(verify.sh).
	if r := languages.RunnerFor("internal-toolbench", "nonexistent-lang"); r != nil {
		t.Fatalf("RunnerFor(internal-toolbench, nonexistent-lang) = %v; want nil (verify.sh fallback)", r)
	}
}

// TestFairnessGate proves the D-04 startup fairness gate predicate RunCell wires:
// the committed runners.DefaultContract.Validate() == nil (so a normal cell is
// never aborted by the gate), and a contract carrying an override with an empty
// WaiverReason fails Validate() (so RunCell's `non-nil return is FATAL` wiring
// would refuse to run an unfair benchmark). This mirrors the unit-level pattern in
// runners.TestEmptyWaiverReasonFatal: RunCell calls the gate the same way (a pure
// predicate over the compile-time contract right after profile resolution), so
// asserting the predicate here proves the gate's behavior without spawning a daemon.
func TestFairnessGate(t *testing.T) {
	// The committed contract must pass — a normal cell is never aborted by the gate.
	require.NoError(t, runners.DefaultContract.Validate(),
		"committed DefaultContract must Validate() == nil so RunCell never fatals in CI")

	// A contract with an empty-WaiverReason override fails the gate predicate, the
	// exact non-nil return RunCell treats as fatal (refuse the unfair benchmark).
	tokens := 4096
	bad := runners.DefaultContract
	bad.Overrides = map[string]runners.ModeOverride{
		"your_agent_no_semantic": {
			MaxTokens:  &tokens,
			ApprovedBy: "maintainer",
			// WaiverReason intentionally empty.
		},
	}
	require.Error(t, bad.Validate(),
		"an override lacking a WaiverReason must fail the fairness gate predicate")
}

// TestAblationStatus asserts the D-03 deferral-marker helper RunCell uses at the
// BuildResult step: only the your_agent_no_semantic arm carries
// guarantee_pending_phase_81; honest modes leave it empty. It also round-trips
// BuildResult to confirm the no_semantic value lands under the `ablation_status`
// key and the honest mode omits the key entirely (omitempty).
func TestAblationStatus(t *testing.T) {
	assert.Equal(t, "guarantee_pending_phase_81", ablationStatusFor("your_agent_no_semantic"),
		"the no_semantic arm must carry the deferral marker (D-03)")
	assert.Equal(t, "", ablationStatusFor("your_agent_full"),
		"honest modes must leave ablation_status empty")
	assert.Equal(t, "", ablationStatusFor("baseline_plain"),
		"honest modes must leave ablation_status empty")

	// BuildResult round-trip: the no_semantic row carries the key; the full row omits it.
	t.Run("no_semantic row carries ablation_status", func(t *testing.T) {
		b, err := BuildResult(ResultInput{
			TaskID:         "IT-go-patch-apply-1",
			Mode:           "your_agent_no_semantic",
			Benchmark:      "internal-toolbench",
			Outcome:        "pass",
			Fairness:       runners.DefaultContract,
			AblationStatus: ablationStatusFor("your_agent_no_semantic"),
		})
		require.NoError(t, err)
		require.NoError(t, Validate(b))
		var doc map[string]any
		require.NoError(t, json.Unmarshal(b, &doc))
		assert.Equal(t, "guarantee_pending_phase_81", doc["ablation_status"],
			"no_semantic result.v2 must contain ablation_status: guarantee_pending_phase_81")
	})

	t.Run("full row omits ablation_status", func(t *testing.T) {
		b, err := BuildResult(ResultInput{
			TaskID:         "IT-go-patch-apply-1",
			Mode:           "your_agent_full",
			Benchmark:      "internal-toolbench",
			Outcome:        "pass",
			Fairness:       runners.DefaultContract,
			AblationStatus: ablationStatusFor("your_agent_full"),
		})
		require.NoError(t, err)
		require.NoError(t, Validate(b))
		var doc map[string]any
		require.NoError(t, json.Unmarshal(b, &doc))
		_, present := doc["ablation_status"]
		assert.False(t, present, "honest mode result.v2 must omit the ablation_status key (omitempty)")
	})
}

// TestBaselineRagFailClose asserts the D-02 fail-close: a RunCell for
// cfg.Mode == "baseline_rag" short-circuits BEFORE any sandbox/daemon, returning a
// nil error, res.Deferred == true, a DeferredReason naming Phase 83, and writing NO
// result.v2.json on disk at res.ResultPath. The cell must return before touching the
// helix binary, so a dummy HelixBin (never spawned) is sufficient.
func TestBaselineRagFailClose(t *testing.T) {
	outDir := t.TempDir()
	cfg := CellConfig{
		RunID:       "20060102T150405Z",
		Benchmark:   "internal-toolbench",
		Language:    "go",
		Task:        "IT-go-patch-apply-1",
		Mode:        "baseline_rag",
		HelixBin:    "/bin/true", // never spawned — the cell fail-closes first
		SeedDir:     filepath.Join(t.TempDir(), "unused-seed"),
		OutDir:      outDir,
		RunnersRoot: benchRunnersRootForTest(t),
	}

	res, err := RunCell(context.Background(), cfg)
	require.NoError(t, err, "baseline_rag must fail-close with a nil error (a registered stub, not an infra failure)")
	assert.True(t, res.Deferred, "baseline_rag must mark the cell Deferred")
	assert.Contains(t, res.DeferredReason, "Phase 83", "DeferredReason must name Phase 83 (ABLATE-04)")

	// No result.v2.json is written: the path is set for layout but the cell
	// short-circuits before BuildResult/writeDurable.
	require.NotEmpty(t, res.ResultPath, "ResultPath must be set for layout even on a deferred cell")
	_, statErr := os.Stat(res.ResultPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "a deferred baseline_rag cell must write NO result.v2.json on disk")
}

// TestDeferredOutcomeClassification asserts the matrix layer classifies a deferred
// cell as a distinct third outcome: a CellOutcome for a deferred RunCell result is
// Success == false AND Deferred == true (neither a success nor an infra error), so
// the smoke can assert "registered + deferred". It drives the dispatcher with an
// injected run that returns a deferred outcome (no real daemon).
func TestDeferredOutcomeClassification(t *testing.T) {
	cells := []Cell{{Benchmark: "b", Mode: "baseline_rag", Task: "t"}}
	run := func(ctx context.Context, c Cell) CellOutcome {
		res := CellResult{Task: c.Task, Mode: c.Mode, Deferred: true, DeferredReason: "deferred to Phase 83"}
		return CellOutcome{
			Cell:     c,
			Result:   res,
			Success:  false, // deferred stub: ResultValid==false → not Success
			Deferred: res.Deferred,
		}
	}
	sum, err := dispatch(context.Background(), cells, 1, run)
	require.NoError(t, err)
	require.Len(t, sum.Outcomes, 1)
	oc := sum.Outcomes[0]
	assert.False(t, oc.Success, "a deferred cell must NOT be counted as a success")
	assert.True(t, oc.Deferred, "a deferred cell must carry the distinct Deferred flag")
	assert.NoError(t, oc.Err, "a deferred cell is not an infra error")
	assert.Equal(t, 0, sum.Succeeded, "a deferred cell must not increment Succeeded")
}

// TestCellGoStaleComments guards Pitfall 5: the stale "ABSOLUTE per-cell store
// path" phrasing must not reappear in cell.go after reconciliation.
func TestCellGoStaleComments(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cellGo := filepath.Join(filepath.Dir(thisFile), "cell.go")
	b, err := os.ReadFile(cellGo)
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(string(b)), "absolute per-cell store path",
		"stale absolute-store-path comment must be reconciled (Pitfall 5)")
}
