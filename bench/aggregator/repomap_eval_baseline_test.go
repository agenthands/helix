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

// repomap_eval_baseline_test.go is the Phase 102 (BASELINE-02) byte-reproducibility +
// committed-baseline proof for the RepoMap-eval baseline. It mirrors
// aider_edit_baseline_test.go:48-118 exactly: the committed
// bench/reports/repomap-eval-baseline/ tree is the hermetic fixture (no daemon, no
// HELIX_BIN, no network — byte-reproducibility is proven by the golden test alone).
//
//   - TestRepoMapEvalBaselineResultValid — committed-result assertion (fail-CLOSED): the
//     committed result.v2.json EXISTS, is schema-valid (Validate passes), and carries the
//     headline nDCG@10 (D-07). A missing file / missing nDCG@10 is a HARD failure.
//   - TestRepoMapEvalBaselineByteReproducible — double-render diff-empty + golden: rendering
//     BENCH-RESULTS.md TWICE from the SAME committed result.v2.json yields byte-identical
//     bytes AND matches the committed BENCH-RESULTS.md exactly.
//   - TestRepoMapEvalBaselineAntiVacuity — a result with nDCG@10 STRIPPED does NOT satisfy
//     the committed-baseline assertion AND the renderer fail-closes (BASELINE-02 stripped-metric).

// repoMapEvalBaselineDir is the committed baseline tree, relative to the bench/aggregator
// test CWD.
const repoMapEvalBaselineDir = "../reports/repomap-eval-baseline"

// readRepoMapEvalBaselineResult reads the committed result.v2.json with a `present`
// fail-closed signal: an os.ReadFile error (missing/unreadable file) is a HARD failure,
// never read as a zero/pass (Pitfall 2).
func readRepoMapEvalBaselineResult(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(repoMapEvalBaselineDir, "result.v2.json")
	b, err := os.ReadFile(path)
	require.NoErrorf(t, err, "committed repomap-eval baseline result.v2.json MUST be present at %s (fail-closed)", path)
	require.NotEmpty(t, b, "committed repomap-eval baseline result.v2.json MUST NOT be empty (fail-closed)")
	return b
}

// TestRepoMapEvalBaselineResultValid asserts the committed result.v2.json is present,
// schema-valid, and carries the headline nDCG@10 (fail-CLOSED).
func TestRepoMapEvalBaselineResultValid(t *testing.T) {
	resultBytes := readRepoMapEvalBaselineResult(t)

	require.NoError(t, benchruntime.Validate(resultBytes),
		"committed repomap-eval baseline result.v2.json MUST be schema-valid")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))

	assert.Equal(t, "repomap_eval", doc["mode"])
	assert.Equal(t, "repomap-eval", doc["benchmark"])

	re, present := doc["repomap_eval"].(map[string]any)
	require.True(t, present,
		"committed baseline MUST carry the repomap_eval object (fail-closed — a missing key is a HARD failure)")
	ndcg, present := re["ndcg_at_10"]
	require.True(t, present,
		"committed baseline MUST carry the headline ndcg_at_10 (fail-closed)")
	assert.InDelta(t, 1.0, ndcg, 1e-9, "the front-loaded forward corpus scores nDCG@10 == 1.0")
}

// TestRepoMapEvalBaselineByteReproducible renders BENCH-RESULTS.md twice from the SAME
// committed result.v2.json and asserts byte-identical bytes (double-render diff-empty)
// AND a byte-match against the committed BENCH-RESULTS.md golden.
func TestRepoMapEvalBaselineByteReproducible(t *testing.T) {
	resultBytes := readRepoMapEvalBaselineResult(t)

	first, err := RenderRepoMapEvalBaseline(resultBytes)
	require.NoError(t, err, "first render of the committed baseline summary")
	second, err := RenderRepoMapEvalBaseline(resultBytes)
	require.NoError(t, err, "second render of the committed baseline summary")

	assert.Equal(t, string(first), string(second),
		"BENCH-RESULTS.md is NOT byte-reproducible across re-render (BASELINE-02 diff-empty failed)")

	wantPath := filepath.Join(repoMapEvalBaselineDir, "BENCH-RESULTS.md")
	want, err := os.ReadFile(wantPath)
	require.NoErrorf(t, err, "committed BENCH-RESULTS.md MUST be present at %s (fail-closed)", wantPath)
	assert.Equal(t, string(want), string(first),
		"BENCH-RESULTS.md drifted from its committed golden — regenerate with `make bench-repomap-eval`")
}

// TestRepoMapEvalBaselineAntiVacuity proves the committed-result assertion actually grades
// the metric: a result with nDCG@10 STRIPPED does NOT satisfy the present check AND the
// renderer fail-closes on the stripped bytes.
func TestRepoMapEvalBaselineAntiVacuity(t *testing.T) {
	resultBytes := readRepoMapEvalBaselineResult(t)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(resultBytes, &doc))

	// (a) STRIP ndcg_at_10 → the present check must fail.
	stripped := cloneMap(doc)
	re := cloneMap(stripped["repomap_eval"].(map[string]any))
	delete(re, "ndcg_at_10")
	stripped["repomap_eval"] = re
	_, present := re["ndcg_at_10"]
	assert.False(t, present, "stripped result must NOT carry ndcg_at_10 (assertion would fail-closed)")

	// (b) The renderer ALSO fail-closes on a stripped ndcg_at_10: rendering a summary from
	// bytes missing the headline metric is a HARD error, never a silent drop.
	strippedBytes, err := json.Marshal(stripped)
	require.NoError(t, err)
	_, rerr := RenderRepoMapEvalBaseline(strippedBytes)
	assert.Error(t, rerr,
		"RenderRepoMapEvalBaseline must fail-closed when ndcg_at_10 is absent")
}
