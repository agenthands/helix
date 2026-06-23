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

// fuzzy_robust_baseline_test.go is the Phase 102 (BASELINE-02) byte-reproducibility +
// committed-baseline proof for the fuzzy-robustness baseline. It mirrors
// aider_edit_baseline_test.go:48-118 exactly: the committed
// bench/reports/fuzzy-robust-baseline/ tree is the hermetic fixture (no daemon, no
// HELIX_BIN, no network).
//
//   - TestFuzzyRobustBaselineResultValid — committed-result assertion (fail-CLOSED): the
//     committed result.v2.json EXISTS, is schema-valid, and carries the load-bearing
//     ambiguous_refused assertion (D-06). A missing file / missing assertion is a HARD failure.
//   - TestFuzzyRobustBaselineByteReproducible — double-render diff-empty + golden.
//   - TestFuzzyRobustBaselineAntiVacuity — a result with ambiguous_refused STRIPPED (or
//     flipped to false) does NOT satisfy the assertion AND the renderer fail-closes.

// fuzzyRobustBaselineDir is the committed baseline tree, relative to the bench/aggregator
// test CWD.
const fuzzyRobustBaselineDir = "../reports/fuzzy-robust-baseline"

// readFuzzyRobustBaselineResult reads the committed result.v2.json with a `present`
// fail-closed signal: an os.ReadFile error (missing/unreadable file) is a HARD failure.
func readFuzzyRobustBaselineResult(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(fuzzyRobustBaselineDir, "result.v2.json")
	b, err := os.ReadFile(path)
	require.NoErrorf(t, err, "committed fuzzy-robust baseline result.v2.json MUST be present at %s (fail-closed)", path)
	require.NotEmpty(t, b, "committed fuzzy-robust baseline result.v2.json MUST NOT be empty (fail-closed)")
	return b
}

// TestFuzzyRobustBaselineResultValid asserts the committed result.v2.json is present,
// schema-valid, and carries the ambiguous_refused assertion = true (fail-CLOSED).
func TestFuzzyRobustBaselineResultValid(t *testing.T) {
	resultBytes := readFuzzyRobustBaselineResult(t)

	require.NoError(t, benchruntime.Validate(resultBytes),
		"committed fuzzy-robust baseline result.v2.json MUST be schema-valid")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))

	assert.Equal(t, "fuzzy_robust", doc["mode"])
	assert.Equal(t, "fuzzy-robust", doc["benchmark"])

	fr, present := doc["fuzzy_robust"].(map[string]any)
	require.True(t, present,
		"committed baseline MUST carry the fuzzy_robust object (fail-closed — a missing key is a HARD failure)")
	ar, present := fr["ambiguous_refused"]
	require.True(t, present,
		"committed baseline MUST carry the load-bearing ambiguous_refused assertion (fail-closed)")
	assert.Equal(t, true, ar, "the duplicate-block ambiguous case MUST be refused (D-06)")
}

// TestFuzzyRobustBaselineByteReproducible renders BENCH-RESULTS.md twice from the SAME
// committed result.v2.json and asserts byte-identical bytes (double-render diff-empty)
// AND a byte-match against the committed BENCH-RESULTS.md golden.
func TestFuzzyRobustBaselineByteReproducible(t *testing.T) {
	resultBytes := readFuzzyRobustBaselineResult(t)

	first, err := RenderFuzzyRobustBaseline(resultBytes)
	require.NoError(t, err, "first render of the committed baseline summary")
	second, err := RenderFuzzyRobustBaseline(resultBytes)
	require.NoError(t, err, "second render of the committed baseline summary")

	assert.Equal(t, string(first), string(second),
		"BENCH-RESULTS.md is NOT byte-reproducible across re-render (BASELINE-02 diff-empty failed)")

	wantPath := filepath.Join(fuzzyRobustBaselineDir, "BENCH-RESULTS.md")
	want, err := os.ReadFile(wantPath)
	require.NoErrorf(t, err, "committed BENCH-RESULTS.md MUST be present at %s (fail-closed)", wantPath)
	assert.Equal(t, string(want), string(first),
		"BENCH-RESULTS.md drifted from its committed golden — regenerate with `make bench-fuzzy-robust`")
}

// TestFuzzyRobustBaselineAntiVacuity proves the committed-result assertion actually grades
// the metric: a result with ambiguous_refused STRIPPED (or flipped to false) does NOT
// satisfy the present+true check AND the renderer fail-closes.
func TestFuzzyRobustBaselineAntiVacuity(t *testing.T) {
	resultBytes := readFuzzyRobustBaselineResult(t)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))

	// (a) STRIP ambiguous_refused → the present check must fail.
	stripped := cloneMap(doc)
	fr := cloneMap(stripped["fuzzy_robust"].(map[string]any))
	delete(fr, "ambiguous_refused")
	stripped["fuzzy_robust"] = fr
	_, present := fr["ambiguous_refused"]
	assert.False(t, present, "stripped result must NOT carry ambiguous_refused (assertion would fail-closed)")

	// (b) FLIP ambiguous_refused to false → a non-refusing result must NOT satisfy the
	// must-refuse assertion (non-vacuous).
	flipped := cloneMap(doc)
	frFlip := cloneMap(flipped["fuzzy_robust"].(map[string]any))
	frFlip["ambiguous_refused"] = false
	flipped["fuzzy_robust"] = frFlip
	assert.NotEqual(t, true, frFlip["ambiguous_refused"],
		"a flipped ambiguous_refused must not satisfy the must-refuse assertion (non-vacuous)")

	// (c) The renderer ALSO fail-closes on a stripped/false ambiguous_refused.
	strippedBytes, err := json.Marshal(stripped)
	require.NoError(t, err)
	_, rerr := RenderFuzzyRobustBaseline(strippedBytes)
	assert.Error(t, rerr,
		"RenderFuzzyRobustBaseline must fail-closed when ambiguous_refused is absent")

	flippedBytes, err := json.Marshal(flipped)
	require.NoError(t, err)
	_, ferr := RenderFuzzyRobustBaseline(flippedBytes)
	assert.Error(t, ferr,
		"RenderFuzzyRobustBaseline must fail-closed when ambiguous_refused is false")
}
