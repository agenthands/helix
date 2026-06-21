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
// Fixture (DISCRIMINATING — the contaminated task carries a verdict that, if counted,
// would INFLATE every headline, so exclusion is genuinely asserted not masked): mode
// "full" has TWO CLEAN tasks and ONE fully-CONTAMINATED task.
//
//   - task-clean-a: both runs UNSOLVED + verified_correctness=FALSE  (clean rate 0.0)
//   - task-clean-b: both runs SOLVED   + verified_correctness=TRUE   (clean rate 1.0)
//   - task-dirty:   both runs SOLVED   + verified_correctness=TRUE, echoing the sentinel
//
// Clean-only:  pass@1 across-task vector {0.0, 1.0} -> mean 0.50; VC pooled 2/4 = 0.50.
// If the dirty solved/verified rows LEAKED in: pass@1 {0.0,1.0,1.0} -> 0.667; VC 4/6 = 0.667.
// So 0.50 vs 0.667 genuinely discriminates the exclusion (unlike the prior fixture where
// clean and dirty shared the same verdict and 1.0==1.0 masked the leak). The two-element
// clean vector also keeps BCa non-degenerate so the ablation FullCI stays OK.
func TestCanaryExclusionFromHeadline(t *testing.T) {
	dir := t.TempDir()
	// task-clean-a: two clean UNSOLVED + verified=FALSE runs.
	writeCanaryRowVC(t, dir, "task-clean-a", "full", 0, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-clean-a", "full", 1, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	// task-clean-b: two clean SOLVED + verified=TRUE runs.
	writeCanaryRowVC(t, dir, "task-clean-b", "full", 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a - b")
	writeCanaryRowVC(t, dir, "task-clean-b", "full", 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a - b")
	// task-dirty: two contaminated runs (echo the sentinel), SOLVED + verified=TRUE on
	// paper. If they leaked into the headline they would push pass@1 / verified_correctness
	// from 0.50 UP to 0.667 — exactly the inflation the canary exclusion must prevent.
	writeCanaryRowVC(t, dir, "task-dirty", "full", 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)
	writeCanaryRowVC(t, dir, "task-dirty", "full", 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	byMode := canaryByMode(rep.Leaderboard)
	full, ok := byMode["full"]
	require.True(t, ok, "expected a full-mode leaderboard row")

	// Headline reduces over the TWO clean tasks ONLY ({0.0, 1.0} -> 0.50). If the
	// contaminated solved/verified task leaked in, BOTH of these would be 0.667, not 0.50.
	require.True(t, full.PassAt1.OK, "clean tasks must populate the headline pass@1")
	assert.InDelta(t, 0.5, full.PassAt1.Point, 1e-9,
		"headline pass@1 reduces over the clean tasks ONLY ({0,1}->0.5), never 0.667 from the dirty task")
	require.True(t, full.VerifiedCorrectness.OK, "clean tasks must populate verified_correctness")
	assert.InDelta(t, 0.5, full.VerifiedCorrectness.Point, 1e-9,
		"verified_correctness pools the clean rows ONLY (2/4=0.5), never 4/6=0.667 from the dirty task")

	// CanaryPassRate MEASURES contamination over ALL rows: 4 clean / 6 with-completion.
	require.True(t, full.CanaryPassRate.OK, "CanaryPassRate must stay populated over all rows")
	assert.InDelta(t, 4.0/6.0, full.CanaryPassRate.Point, 1e-9,
		"CanaryPassRate counts ALL rows (4 clean / 6) — exclusion does not touch the measurement")

	// WR-02 per_language: the per-language pass-rate pools across every (task,mode) cell.
	// All tasks bucket under the empty language key (""). With exclusion the clean rows are
	// the 4 task-clean-* runs (2 solved -> 2/4 = 0.5); if the 2 dirty solved runs leaked in
	// it would be 4/6 == 0.667.
	var emptyLang *LanguageRow
	for i := range rep.ByLanguage {
		if rep.ByLanguage[i].Language == "" {
			emptyLang = &rep.ByLanguage[i]
		}
	}
	require.NotNil(t, emptyLang, "the empty-language bucket must be present")
	assert.InDelta(t, 0.5, emptyLang.PassRate, 1e-9,
		"per_language pass_rate pools the clean rows ONLY (2/4=0.5), never 4/6=0.667")
	assert.Equal(t, 4, emptyLang.N, "per_language n counts ONLY the 4 clean rows, not the 2 contaminated")

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

// TestCanaryExclusionFromAblations is the WR-01 integrity proof for the published
// ablations.md table: the full-mode (your_agent_full) task_success vector that feeds
// every comparison's FullCI must reduce over CLEAN rows only. The fixture gives
// your_agent_full a DISCRIMINATING split — one clean UNSOLVED task and one clean
// SOLVED task ({0.0, 1.0} -> 0.5), plus one fully-CONTAMINATED SOLVED task that, if
// counted, would push the vector to {0.0, 1.0, 1.0} -> 0.667. your_agent_no_lsp is
// the OTHER operand so the comparison is Present (FullCI is asserted directly).
func TestCanaryExclusionFromAblations(t *testing.T) {
	dir := t.TempDir()
	const full = "your_agent_full"
	const other = "your_agent_no_lsp"
	// your_agent_full clean tasks: one unsolved, one solved -> {0.0, 1.0} -> 0.5.
	writeCanaryRowVC(t, dir, "task-clean-a", full, 0, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-clean-a", full, 1, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-clean-b", full, 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a - b")
	writeCanaryRowVC(t, dir, "task-clean-b", full, 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a - b")
	// your_agent_full contaminated SOLVED task: would inflate the FullCI to 0.667 if counted.
	writeCanaryRowVC(t, dir, "task-dirty", full, 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)
	writeCanaryRowVC(t, dir, "task-dirty", full, 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "memorised "+canary.Sentinel)
	// your_agent_no_lsp: a clean OTHER operand so the comparison renders as Present.
	writeCanaryRowVC(t, dir, "task-clean-a", other, 0, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-clean-a", other, 1, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-clean-b", other, 0, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a - b")
	writeCanaryRowVC(t, dir, "task-clean-b", other, 1, metricVC(true, true, 1000, 100, 5, 3, 0.9), "return a - b")
	writeCanaryRowVC(t, dir, "task-dirty", other, 0, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")
	writeCanaryRowVC(t, dir, "task-dirty", other, 1, metricVC(false, false, 1000, 100, 5, 3, 0.9), "return a + b")

	rep, err := Aggregate(dir, aggConfig(2))
	require.NoError(t, err)
	require.NotNil(t, rep)

	var noLsp *AblationRow
	for i := range rep.Ablations {
		if rep.Ablations[i].Comparison == "full_minus_no_lsp" {
			noLsp = &rep.Ablations[i]
		}
	}
	require.NotNil(t, noLsp, "the full_minus_no_lsp comparison must be present")
	require.True(t, noLsp.Present, "both operands present -> comparison renders")
	require.True(t, noLsp.FullCI.OK, "the full operand task_success must populate the FullCI")
	assert.InDelta(t, 0.5, noLsp.FullCI.Point, 1e-9,
		"ablation full task_success reduces over the clean tasks ONLY ({0,1}->0.5), never 0.667 from the dirty task")
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

// TestAggregateCanaryAbsentIsEmDash (lockstep render guard, UPDATED in Phase 89-03):
// its intent SHIFTED from "the absent canary leaves a NULL field (em-dash)" alone to
// "the contamination footnote RENDERS CORRECTLY when nothing is contaminated — i.e.
// it is absent". Rows with NO completion key leave CanaryPassRate a NULL ci
// (OK==false, em-dash discipline — never a fabricated 0) AND, because no row echoed
// the canary sentinel, the rendered leaderboard.md carries NO INFRA-05 contamination
// footnote (em-dash discipline at the render layer: no footnote is fabricated when
// nothing was excluded). The guard is UPDATED, never bypassed/deleted (Pitfall 1).
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

	// Intent shift: nothing contaminated -> the rendered footnote section is ABSENT
	// (the footnote renders correctly by NOT being fabricated).
	assert.Empty(t, rep.Contaminated,
		"no canary-echoing rows -> no contaminated cells carried on the Report")
	lb, err := os.ReadFile(filepath.Join(dir, "leaderboard.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(lb), "Contamination canary exclusions",
		"an uncontaminated run must render NO INFRA-05 footnote (never fabricated)")
}
