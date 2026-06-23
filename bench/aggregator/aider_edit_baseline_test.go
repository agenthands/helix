package aggregator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	benchruntime "github.com/agenthands/helix/bench/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aider_edit_baseline_test.go is the Phase 100 (BASELINE-01) byte-reproducibility +
// committed-baseline proof. It mirrors byte_reproducible_test.go:25-77: the committed
// bench/reports/aider-edit-baseline/ tree is the hermetic fixture (no daemon, no
// HELIX_BIN, no network — byte-reproducibility is proven by the golden test alone).
//
//   - TestAiderEditBaselineResultValid — committed-result assertion (fail-CLOSED): the
//     committed result.v2.json EXISTS, is schema-valid (Validate passes), and carries
//     outcome=success + edit_format_applied=true. A missing file / missing
//     edit_format_applied is a HARD failure, never a pass (Pitfall 2).
//   - TestAiderEditBaselineByteReproducible — double-render diff-empty + golden: rendering
//     BENCH-RESULTS.md TWICE from the SAME committed result.v2.json yields byte-identical
//     bytes AND matches the committed BENCH-RESULTS.md exactly. A render that drifts on
//     re-render (RNG leak, map-order, unsorted emit, embedded timestamp/latency) fails HERE.
//   - TestAiderEditBaselineAntiVacuity — a result with edit_format_applied STRIPPED (or
//     outcome flipped) does NOT satisfy the committed-baseline assertion, proving the
//     assertion actually grades the metric.

// baselineDir is the committed baseline tree, relative to the bench/aggregator test CWD.
const baselineDir = "../reports/aider-edit-baseline"

// readBaselineResult reads the committed result.v2.json with a `present` fail-closed
// signal: an os.ReadFile error (missing/unreadable file) is a HARD failure, never read
// as a zero/pass (Pitfall 2, mirrors scrapeSemanticReadsTotal's present signal).
func readBaselineResult(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(baselineDir, "result.v2.json")
	b, err := os.ReadFile(path)
	require.NoErrorf(t, err, "committed baseline result.v2.json MUST be present at %s (fail-closed)", path)
	require.NotEmpty(t, b, "committed baseline result.v2.json MUST NOT be empty (fail-closed)")
	return b
}

// TestAiderEditBaselineResultValid asserts the committed result.v2.json is present,
// schema-valid, and carries outcome=success + edit_format_applied=true (fail-CLOSED).
func TestAiderEditBaselineResultValid(t *testing.T) {
	resultBytes := readBaselineResult(t)

	require.NoError(t, benchruntime.Validate(resultBytes),
		"committed baseline result.v2.json MUST be schema-valid")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))

	efa, present := doc["edit_format_applied"]
	require.True(t, present,
		"committed baseline MUST carry edit_format_applied (fail-closed — a missing key is a HARD failure)")
	assert.Equal(t, true, efa, "edit_format_applied must be true on the deterministic baseline")
	assert.Equal(t, "success", doc["outcome"], "the reference-solution baseline must grade success")
	assert.Equal(t, "aider_edit", doc["mode"])
	assert.Equal(t, "aider-polyglot", doc["benchmark"])
	assert.Equal(t, "go", doc["language"])
	assert.Equal(t, "wordy", doc["task_id"])
}

// TestAiderEditBaselineByteReproducible renders BENCH-RESULTS.md twice from the SAME
// committed result.v2.json and asserts byte-identical bytes (double-render diff-empty)
// AND a byte-match against the committed BENCH-RESULTS.md golden.
func TestAiderEditBaselineByteReproducible(t *testing.T) {
	resultBytes := readBaselineResult(t)

	first, err := RenderAiderEditBaseline(resultBytes)
	require.NoError(t, err, "first render of the committed baseline summary")
	second, err := RenderAiderEditBaseline(resultBytes)
	require.NoError(t, err, "second render of the committed baseline summary")

	// Double-render diff-empty: the two renders are byte-identical.
	assert.Equal(t, string(first), string(second),
		"BENCH-RESULTS.md is NOT byte-reproducible across re-render (BASELINE-01 diff-empty failed)")

	// And the render matches the committed golden (no drift from the frozen bytes).
	wantPath := filepath.Join(baselineDir, "BENCH-RESULTS.md")
	want, err := os.ReadFile(wantPath)
	require.NoErrorf(t, err, "committed BENCH-RESULTS.md MUST be present at %s (fail-closed)", wantPath)
	assert.Equal(t, string(want), string(first),
		"BENCH-RESULTS.md drifted from its committed golden — regenerate with `make bench-aider-edit`")
}

// TestAiderEditBaselineAntiVacuity proves the committed-result assertion actually grades
// the metric: a result with edit_format_applied STRIPPED (or outcome flipped) does NOT
// satisfy the present + value checks the real assertion enforces.
func TestAiderEditBaselineAntiVacuity(t *testing.T) {
	resultBytes := readBaselineResult(t)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))

	// (a) STRIP edit_format_applied → the present check must fail.
	stripped := cloneMap(doc)
	delete(stripped, "edit_format_applied")
	_, present := stripped["edit_format_applied"]
	assert.False(t, present, "stripped result must NOT carry edit_format_applied (assertion would fail-closed)")

	// (b) FLIP outcome → the outcome check must fail.
	flipped := cloneMap(doc)
	flipped["outcome"] = "failed"
	assert.NotEqual(t, "success", flipped["outcome"],
		"a flipped-outcome result must not satisfy the success assertion (non-vacuous)")

	// (c) The renderer ALSO fail-closes on a stripped edit_format_applied: rendering a
	// summary from bytes missing the key is a HARD error, never a silent drop.
	strippedBytes, err := json.Marshal(stripped)
	require.NoError(t, err)
	_, rerr := RenderAiderEditBaseline(strippedBytes)
	assert.Error(t, rerr,
		"RenderAiderEditBaseline must fail-closed when edit_format_applied is absent")
}

// cloneMap returns a shallow copy of a decoded JSON object so a per-case mutation does
// not bleed into other cases.
func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
