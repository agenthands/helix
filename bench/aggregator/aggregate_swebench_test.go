package aggregator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	"github.com/agenthands/helix/bench/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aggregate_swebench_test.go is the hermetic proof of the ADDITIVE Phase 87
// (VERIFIED-02) raw-vs-UTBoost-rescored side-by-side leaderboard column. It clones
// the Phase 86 CanaryPassRate discipline (aggregate_canary_test.go): the two
// scores are reduced at SCORE TIME from each row's open-provenance doc keys
// runtime.SwebenchRawResolvedKey / runtime.SwebenchRescoredVerifiedKey (the SAME
// PINNED consts Plan 03's rescore.ApplyToRow stamps), NEVER alter the existing
// (mode x benchmark) leaderboard/cost rows or their sort, render an em-dash when no
// row carries the keys, and consume ZERO RNG. No network; no loader-source edit.
//
// These tests inject the open keys post-build via the SAME pinned consts the
// producer stamps, so the aggregator-side proof stays self-contained while reading
// the real key names. The end-to-end producer<->reader round-trip
// (rescore.ApplyToRow -> rowSwebenchScores with NO hand-injection) is owned by
// Plan 03's TestRescore_ApplyToRow_RoundTrip.

// writeSwebenchRow writes a schema-valid result.v2.json AND injects the additive
// raw/rescored open-provenance doc keys (runtime.SwebenchRawResolvedKey carrying
// the raw upstream resolved verdict, runtime.SwebenchRescoredVerifiedKey carrying
// the UTBoost verified_correctness verdict) post-build, at
// <outDir>/<task>/<mode>/<runIndex>/result.v2.json. The keys are open-provenance
// (NOT BuildResult fields; additionalProperties stays OPEN, additive-minor), so the
// patched doc stays schema-valid — exactly the seam rowSwebenchScores reads.
func writeSwebenchRow(t *testing.T, outDir, task, mode string, runIndex int, m evaluators.Metrics, raw, rescored bool) {
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
	require.NoError(t, runtime.Validate(b), "row must be schema-valid before the additive swebench keys are added")

	// Inject the additive raw/rescored open-provenance keys under the PINNED consts.
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &doc))
	rawJ, err := json.Marshal(raw)
	require.NoError(t, err)
	rescoredJ, err := json.Marshal(rescored)
	require.NoError(t, err)
	doc[runtime.SwebenchRawResolvedKey] = rawJ
	doc[runtime.SwebenchRescoredVerifiedKey] = rescoredJ
	patched, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, runtime.Validate(patched), "patched row must STILL be schema-valid (additive-minor)")

	dir := filepath.Join(outDir, task, mode, strconv.Itoa(runIndex))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "result.v2.json"), patched, 0o600))
}

// swebenchByMode indexes leaderboard rows by mode for assertions.
func swebenchByMode(rows []LeaderRow) map[string]LeaderRow {
	m := map[string]LeaderRow{}
	for _, r := range rows {
		m[r.Mode] = r
	}
	return m
}

// TestRowSwebenchScoresReadsKeys: rowSwebenchScores reads the open raw/rescored doc
// keys via the PINNED runtime consts and reports (raw, rescored, present). A row
// carrying neither key reads present=false (a non-SWE-bench artifact).
func TestRowSwebenchScoresReadsKeys(t *testing.T) {
	present := Row{Doc: map[string]json.RawMessage{
		runtime.SwebenchRawResolvedKey:      json.RawMessage(`true`),
		runtime.SwebenchRescoredVerifiedKey: json.RawMessage(`false`),
	}}
	raw, rescored, ok := rowSwebenchScores(present)
	require.True(t, ok, "a row carrying both swebench keys is present")
	require.NotNil(t, raw)
	require.NotNil(t, rescored)
	assert.True(t, *raw, "raw upstream resolved verdict is true")
	assert.False(t, *rescored, "UTBoost rescored verified verdict is false (the SC#2-shaped divergence)")

	noKey := Row{Doc: map[string]json.RawMessage{"model_id": json.RawMessage(`"x"`)}}
	_, _, ok = rowSwebenchScores(noKey)
	assert.False(t, ok, "a row carrying neither swebench key reads as not-present")
}

// TestSwebenchColumnsDivergence (Case A): rows with raw=true / rescored=false (the
// SC#2-shaped divergence) yield RawScore.Point > RescoredScore.Point, both OK==true.
func TestSwebenchColumnsDivergence(t *testing.T) {
	dir := t.TempDir()
	// N=2 over one task: raw resolved on both runs, but UTBoost rescore verifies only
	// one -> raw rate 1.0, rescored rate 0.5 (the divergence the column surfaces).
	writeSwebenchRow(t, dir, "task-1", "full", 0, metric(true, 1000, 100, 5, 3, 0.9), true, true)
	writeSwebenchRow(t, dir, "task-1", "full", 1, metric(true, 1000, 100, 5, 3, 0.9), true, false)

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	byMode := swebenchByMode(rep.Leaderboard)
	full, ok := byMode["full"]
	require.True(t, ok, "expected a full-mode leaderboard row")
	require.True(t, full.RawScore.OK, "a cell with swebench keys must populate RawScore")
	require.True(t, full.RescoredScore.OK, "a cell with swebench keys must populate RescoredScore")
	assert.InDelta(t, 1.0, full.RawScore.Point, 1e-9, "2/2 raw resolved -> 1.0")
	assert.InDelta(t, 0.5, full.RescoredScore.Point, 1e-9, "1/2 rescore verified -> 0.5")
	assert.Greater(t, full.RawScore.Point, full.RescoredScore.Point,
		"the UTBoost rescore must surface the (lower) rescored rate side-by-side")
}

// TestSwebenchColumnsAbsentIsEmDash (Case B): rows with NO swebench keys leave
// RawScore/RescoredScore as NULL ci (OK==false) so the renderer prints an em-dash —
// never a fabricated 0 or a crash. The existing leaderboard is unharmed (additive).
func TestSwebenchColumnsAbsentIsEmDash(t *testing.T) {
	dir := t.TempDir()
	writeCostedRow(t, dir, "task-1", "full", 0, metric(true, 1000, 100, 5, 3, 0.9))
	writeCostedRow(t, dir, "task-1", "full", 1, metric(false, 1000, 100, 5, 3, 0.9))

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	require.Len(t, rep.Leaderboard, 1, "absent-swebench-key rows must not break the leaderboard")
	assert.False(t, rep.Leaderboard[0].RawScore.OK,
		"no swebench data -> NULL RawScore (em-dash), never a fabricated 0")
	assert.False(t, rep.Leaderboard[0].RescoredScore.OK,
		"no swebench data -> NULL RescoredScore (em-dash), never a fabricated 0")
}

// TestSwebenchColumnsGoldenStable (Case C — the lockstep render guard, UPDATED in
// Phase 89-03): its intent SHIFTED from "the additive column is invisible / perturbs
// nothing" to "the REPORT-01 verified_correctness + cost_per_solved columns RENDER
// CORRECTLY in leaderboard.md". The Phase 89 (REPORT-01) verified_correctness +
// cost_per_solved columns ARE now rendered into leaderboard.md (they regenerated the
// committed golden), so this guard asserts (a) the column headers are present, (b)
// the rendered golden bytes are byte-stable, and (c) the additive Phase 87
// RawScore/RescoredScore fields — which are STILL not rendered — are populated on
// the Report without leaking into the rendered bytes. The guard is UPDATED, never
// bypassed/deleted (Pitfall 1).
func TestSwebenchColumnsGoldenStable(t *testing.T) {
	dir := goldenFixture(t)
	rep, err := Aggregate(dir, aggConfig(3))
	require.NoError(t, err)
	require.NotNil(t, rep)

	for _, name := range []string{"leaderboard.md", "cost_quality.md"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		goldenPath := filepath.Join("testdata", "leaderboard.golden.md")
		if name == "cost_quality.md" {
			goldenPath = filepath.Join("testdata", "cost_quality.golden.md")
		}
		want, err := os.ReadFile(goldenPath)
		require.NoError(t, err, "missing golden %s", goldenPath)
		assert.Equal(t, string(want), string(got),
			"%s drifted from its committed golden (byte-stable render of the REPORT-01 columns)", name)
	}

	// Intent shift: the REPORT-01 columns RENDER (header present), not invisible.
	lb, err := os.ReadFile(filepath.Join(dir, "leaderboard.md"))
	require.NoError(t, err)
	assert.Contains(t, string(lb), "verified_correctness",
		"leaderboard.md must RENDER the REPORT-01 verified_correctness column header")
	assert.Contains(t, string(lb), "cost_per_solved",
		"leaderboard.md must RENDER the REPORT-01 cost_per_solved column header")

	// The Phase 87 RawScore/RescoredScore fields are STILL additive-only (not rendered):
	// they exist on the Report but never leak into the rendered leaderboard bytes.
	assert.NotContains(t, string(lb), "raw_score",
		"RawScore stays an unrendered additive field (no header leak)")
	assert.NotContains(t, string(lb), "rescored_score",
		"RescoredScore stays an unrendered additive field (no header leak)")
}
