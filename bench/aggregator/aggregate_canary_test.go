package aggregator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/agenthands/helix/bench/canary"
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	"github.com/agenthands/helix/bench/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aggregate_canary_test.go is the hermetic proof of the ADDITIVE CanaryPassRate
// leaderboard column (Phase 86 Plan 05 Task 2). It mirrors the Phase 85 ByLanguage
// additive discipline: the column is reduced at SCORE TIME from each row's
// `completion` doc key (via bench/canary.IsContaminated), NEVER alters the existing
// (mode x benchmark) leaderboard/cost rows, and renders an em-dash when no row
// carries a completion. No network; no loader-source edit.

// writeCanaryRow writes a schema-valid result.v2.json AND injects the open
// `completion` provenance doc key carrying the given completion text, at
// <outDir>/<task>/<mode>/<runIndex>/result.v2.json. The completion key is added
// post-build (BuildResult has no completion field; the key is open-provenance,
// additive-minor — additionalProperties stays OPEN), exactly the seam the
// score-time canary reduce reads.
func writeCanaryRow(t *testing.T, outDir, task, mode string, runIndex int, m evaluators.Metrics, completion string) {
	t.Helper()
	b, err := runtime.BuildResult(runtime.ResultInput{
		TaskID:    task,
		Mode:      mode,
		Benchmark: "internal-toolbench",
		RunIndex:  runIndex,
		Outcome:   "pass",
		Metrics:   m,
		Fairness:  runners.FairnessContract{ModelID: goldenModelID},
	})
	require.NoError(t, err)
	require.NoError(t, runtime.Validate(b), "row must be schema-valid before the additive completion key is added")

	// Inject the additive `completion` open-provenance key.
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &doc))
	cj, err := json.Marshal(completion)
	require.NoError(t, err)
	doc[canary.DocKeyCompletion] = cj
	patched, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, runtime.Validate(patched), "patched row must STILL be schema-valid (additive-minor)")

	dir := filepath.Join(outDir, task, mode, strconv.Itoa(runIndex))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "result.v2.json"), patched, 0o600))
}

// canaryByMode indexes leaderboard rows by mode for assertions.
func canaryByMode(rows []LeaderRow) map[string]LeaderRow {
	m := map[string]LeaderRow{}
	for _, r := range rows {
		m[r.Mode] = r
	}
	return m
}

// TestRowCanaryReadsCompletion: rowCanary reads the open `completion` doc key,
// runs bench/canary.IsContaminated, and reports (contaminated, present). An absent
// completion key yields present=false (no canary signal for that row).
func TestRowCanaryReadsCompletion(t *testing.T) {
	clean := Row{Doc: map[string]json.RawMessage{
		canary.DocKeyCompletion: json.RawMessage(`"return a + b"`),
	}}
	cont, present := rowCanary(clean)
	assert.True(t, present, "a row carrying a completion key is present")
	assert.False(t, cont, "a clean completion is NOT contaminated")

	dirty := Row{Doc: map[string]json.RawMessage{
		canary.DocKeyCompletion: mustJSON(t, "leaked "+canary.Sentinel),
	}}
	cont, present = rowCanary(dirty)
	assert.True(t, present)
	assert.True(t, cont, "a sentinel-echoing completion IS contaminated")

	noKey := Row{Doc: map[string]json.RawMessage{"model_id": json.RawMessage(`"x"`)}}
	_, present = rowCanary(noKey)
	assert.False(t, present, "an absent completion key reads as not-present")
}

func mustJSON(t *testing.T, s string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return b
}

// TestAggregateCanaryPassRate: a synthetic (mode x benchmark) cell with one clean
// and one contaminated completion yields CanaryPassRate.Point == 0.5 (1 clean / 2
// rows-with-completion). The score-time reduce calls bench/canary.IsContaminated.
func TestAggregateCanaryPassRate(t *testing.T) {
	dir := t.TempDir()
	// N=2: one clean completion (canary pass), one sentinel-echoing (contaminated).
	writeCanaryRow(t, dir, "task-1", "full", 0, metric(true, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRow(t, dir, "task-1", "full", 1, metric(true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	byMode := canaryByMode(rep.Leaderboard)
	full, ok := byMode["full"]
	require.True(t, ok, "expected a full-mode leaderboard row")
	require.True(t, full.CanaryPassRate.OK, "a cell with completions must populate CanaryPassRate")
	assert.InDelta(t, 0.5, full.CanaryPassRate.Point, 1e-9, "1 clean / 2 with-completion -> 0.5")
}

// TestAggregateCanaryAbsentIsEmDash: rows with NO completion key leave
// CanaryPassRate as a NULL ci (OK==false) so the renderer prints an em-dash — never
// a fabricated 0 or a crash. The existing leaderboard is unharmed (additive).
func TestAggregateCanaryAbsentIsEmDash(t *testing.T) {
	dir := t.TempDir()
	writeCostedRow(t, dir, "task-1", "full", 0, metric(true, 1000, 100, 5, 3, 0.9))
	writeCostedRow(t, dir, "task-1", "full", 1, metric(false, 1000, 100, 5, 3, 0.9))

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	require.Len(t, rep.Leaderboard, 1, "absent-completion rows must not break the leaderboard")
	assert.False(t, rep.Leaderboard[0].CanaryPassRate.OK,
		"no completion data -> NULL CanaryPassRate (em-dash), never a fabricated 0")
}
