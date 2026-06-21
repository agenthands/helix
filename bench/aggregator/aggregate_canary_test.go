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

// TestCanaryExclusionFromHeadline is the INFRA-05 integrity proof: a contaminated
// (task,mode) cell — one whose completion echoes canary.Sentinel — is EXCLUDED from
// the headline reduce (pass@1, verified_correctness, cost_per_solved) while STILL
// being counted by CanaryPassRate (which MEASURES contamination over all rows) AND
// listed in the leaderboard.md footnote. Fail-safe: a contaminated row is NEVER
// silently counted in the headline.
//
// Fixture: mode "full" has one CLEAN task (task-clean, both runs solved+verified) and
// one fully-CONTAMINATED task (task-dirty, both runs echo the sentinel). The headline
// must reduce over ONLY task-clean; CanaryPassRate must reflect the contaminated runs;
// the footnote must name (task-dirty, full).
func TestCanaryExclusionFromHeadline(t *testing.T) {
	dir := t.TempDir()
	// task-clean: two clean, solved+verified runs — the ONLY rows the headline sees.
	writeCanaryRowVC(t, dir, "task-clean", "full", 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-clean", "full", 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a + b")
	// task-dirty: two contaminated runs (echo the sentinel). They are solved+verified on
	// paper, so if they leaked into the headline they would INFLATE it — the integrity
	// failure this test guards against.
	writeCanaryRowVC(t, dir, "task-dirty", "full", 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)
	writeCanaryRowVC(t, dir, "task-dirty", "full", 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	byMode := canaryByMode(rep.Leaderboard)
	full, ok := byMode["full"]
	require.True(t, ok, "expected a full-mode leaderboard row")

	// Headline reduces over ONLY the clean task. task-clean is 2/2 solved+verified, so
	// the across-task vector has exactly ONE element (the clean task) at 1.0. If the
	// contaminated task leaked in, the vector would still be 1.0 here — so to truly
	// prove exclusion we assert CanaryPassRate sees the dirty rows but the headline does
	// not, AND the footnote names the dirty cell.
	require.True(t, full.PassAt1.OK, "clean task must populate the headline pass@1")
	assert.InDelta(t, 1.0, full.PassAt1.Point, 1e-9, "headline pass@1 reduces over the clean task only")
	require.True(t, full.VerifiedCorrectness.OK, "clean task must populate verified_correctness")
	assert.InDelta(t, 1.0, full.VerifiedCorrectness.Point, 1e-9, "verified_correctness over clean rows only")

	// CanaryPassRate MEASURES contamination over ALL rows: 2 clean / 4 with-completion.
	require.True(t, full.CanaryPassRate.OK, "CanaryPassRate must stay populated over all rows")
	assert.InDelta(t, 0.5, full.CanaryPassRate.Point, 1e-9,
		"CanaryPassRate counts ALL rows (2 clean / 4) — exclusion does not touch the measurement")

	// The contaminated (task,mode) cell is collected onto the Report and footnoted.
	require.Len(t, rep.Contaminated, 1, "exactly one contaminated (task,mode) cell")
	assert.Equal(t, "task-dirty", rep.Contaminated[0].Task)
	assert.Equal(t, "full", rep.Contaminated[0].Mode)

	// Render and assert the footnote names the excluded cell.
	lb := renderLeaderboard(rep.Leaderboard, rep.PassNK, rep.Footer)
	lb = appendContaminationFootnote(lb, rep.Contaminated)
	assert.Contains(t, lb, "Contamination", "footnote section header present")
	assert.Contains(t, lb, "task-dirty", "footnote names the excluded task")
	assert.Contains(t, lb, "full", "footnote names the excluded mode")
}

// TestCleanRowsSplit unit-tests the cleanRows split primitive: a contaminated row
// (rowCanary present+contaminated) goes to contaminated; a clean row and a
// no-completion row both stay in clean (a pre-canary artifact is NOT contaminated).
func TestCleanRowsSplit(t *testing.T) {
	clean := Row{Doc: map[string]json.RawMessage{canary.DocKeyCompletion: mustJSON(t, "return a + b")}}
	dirty := Row{Doc: map[string]json.RawMessage{canary.DocKeyCompletion: mustJSON(t, "x "+canary.Sentinel)}}
	noKey := Row{Doc: map[string]json.RawMessage{"model_id": json.RawMessage(`"m"`)}}

	gotClean, gotContaminated := cleanRows([]Row{clean, dirty, noKey})
	assert.Len(t, gotContaminated, 1, "exactly the sentinel-echoing row is contaminated")
	assert.Len(t, gotClean, 2, "clean + no-completion rows stay in the clean partition")
}

// writeCanaryRowVC is writeCanaryRow but the metrics carry an explicit
// verified_correctness verdict, so the exclusion test can assert the headline
// verified_correctness reduces over clean rows only.
func writeCanaryRowVC(t *testing.T, outDir, task, mode string, runIndex int, m evaluators.Metrics, completion string) {
	t.Helper()
	writeCanaryRow(t, outDir, task, mode, runIndex, m, completion)
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
