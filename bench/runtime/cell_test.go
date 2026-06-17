package runtime

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenthands/helix/bench/languages"
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

// TestCellLayout asserts the durable artifact layout is
// <out>/<task>/<mode>/{result.v2.json,trace.json} (D-08), exercised through the
// bench sandbox path helpers that RunCell uses. This is a UNIT test — no daemon.
func TestCellLayout(t *testing.T) {
	outDir := t.TempDir()
	sb, err := benchsandbox.New("20060102T150405Z", "/nonexistent/helix", outDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Cleanup() })

	const task = "sum-doubler"
	const mode = "your_agent_full"

	wantResult := filepath.Join(outDir, task, mode, "result.v2.json")
	wantTrace := filepath.Join(outDir, task, mode, "trace.json")

	assert.Equal(t, wantResult, sb.ResultPath(task, mode),
		"result.v2.json must live at <out>/<task>/<mode>/result.v2.json (D-08)")
	assert.Equal(t, wantTrace, sb.MergedTracePath(task, mode),
		"merged trace must live at <out>/<task>/<mode>/trace.json (D-08)")
}

// TestCellLayoutPathTraversalRejected confirms RunCell rejects task/mode/
// benchmark names that would escape the out dir BEFORE any filesystem work
// (V5 / T-77-08). validateCellKey must fire before sandbox creation.
func TestCellLayoutPathTraversalRejected(t *testing.T) {
	cases := []struct {
		name string
		cfg  CellConfig
	}{
		{"task-traversal", CellConfig{Task: "../escape", Mode: "your_agent_full", Benchmark: "toolbench-go", HelixBin: "/bin/true"}},
		{"mode-traversal", CellConfig{Task: "sum-doubler", Mode: "../escape", Benchmark: "toolbench-go", HelixBin: "/bin/true"}},
		{"benchmark-traversal", CellConfig{Task: "sum-doubler", Mode: "your_agent_full", Benchmark: "../escape", HelixBin: "/bin/true"}},
		{"task-separator", CellConfig{Task: "a/b", Mode: "your_agent_full", Benchmark: "toolbench-go", HelixBin: "/bin/true"}},
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
		Benchmark:   "toolbench-go",
		Task:        "sum-doubler",
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
