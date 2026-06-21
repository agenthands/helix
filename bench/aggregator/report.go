package aggregator

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// report.go is the STATS-04/COST-03 render layer: it turns the orchestrator's
// reduced rows (aggregate.go) into the two byte-stable markdown artifacts —
// leaderboard.md and cost_quality.md — and owns the honesty gates that ride on
// the rendered claims:
//
//   - STATS-04 overlap gate (D-17): adjacent leaderboard rows whose sort-metric
//     BCa CIs overlap are annotated and NO directional "X>Y" claim is made.
//   - FAIR-03 variance gate (D-15): a (task,mode) whose per-run USD coefficient
//     of variation exceeds 0.05 renders a cost_quality.md fairness warning.
//
// Determinism (D-08): everything is SORTED before emit (rows by sort-metric then
// mode name, columns in a fixed order, footer fields in a fixed order) and the
// files are written atomically (temp+rename, mirroring cell.go:writeDurable) so a
// concurrent reader never observes a torn report.

// cvThreshold is the FAIR-03 between-run variance cutoff (D-15): a coefficient of
// variation (stddev/mean) of per-run USD strictly greater than this flags the cell.
const cvThreshold = 0.05

// emDash is rendered for a null CI — a metric absent across all runs of all
// tasks. NEVER a fabricated 0 (Pitfall 4 / null discipline).
const emDash = "—"

// ciValue is one rendered confidence interval: a point estimate plus [Lo, Hi]
// BCa endpoints. OK==false marks a NULL CI (the metric was nil across every
// resampling unit) — such a cell renders an em-dash, never [0,0].
type ciValue struct {
	Point float64
	Lo    float64
	Hi    float64
	OK    bool
}

// LeaderRow is one (mode x benchmark) leaderboard row, each headline metric a CI.
type LeaderRow struct {
	Mode      string
	Benchmark string

	TaskSuccess  ciValue // primary sort metric (success-rate)
	PassAt1      ciValue
	PassAtN      ciValue
	TokensInput  ciValue
	TokensOutput ciValue
	ToolCalls    ciValue
	FilesRead    ciValue
	EditLocality ciValue

	// CanaryPassRate is the Phase 86 (Plan 05) ADDITIVE contamination-canary column:
	// the fraction of this (mode x benchmark)'s rows-carrying-a-completion whose
	// completion did NOT echo the injected canary sentinel (canary "pass" == clean).
	// It is reduced at SCORE TIME from the open `completion` doc key via
	// bench/canary.IsContaminated (reduceCanaryRate), mirroring the Phase 85
	// ByLanguage additive discipline: it NEVER alters the existing leaderboard
	// columns or their sort, and a cell whose rows carry NO completion key yields a
	// NULL ci (OK==false) rendered as an em-dash — never a fabricated 0. It is a
	// flat pooled rate, not a BCa CI (so it consumes no RNG and cannot perturb the
	// locked determinism contract); the bootstrapped canary CI is downstream
	// (Phase 89).
	CanaryPassRate ciValue

	// RawScore and RescoredScore are the Phase 87 (VERIFIED-02) ADDITIVE raw-vs-
	// UTBoost-rescored side-by-side columns. RawScore is the pooled fraction of this
	// (mode x benchmark)'s SWE-bench rows whose raw upstream `report.resolved` verdict
	// was true; RescoredScore is the pooled fraction whose UTBoost-rescored
	// verified_correctness verdict was true. Both are reduced at SCORE TIME from the
	// open doc keys runtime.SwebenchRawResolvedKey / runtime.SwebenchRescoredVerifiedKey
	// stamped by Plan 03's rescore.ApplyToRow (the REAL producer — NOT "TBD"), via
	// rowSwebenchScores/reduceSwebenchScores, mirroring the CanaryPassRate precedent
	// above exactly. They NEVER alter the existing leaderboard columns or their sort.
	// A cell whose rows carry NO swebench keys yields a NULL ci (OK==false) rendered
	// as an em-dash — never a fabricated 0. Each is a flat pooled rate, not a BCa CI,
	// so it consumes ZERO RNG and cannot perturb the locked determinism contract; the
	// SWE-bench-specific render of these columns is downstream (Phase 89).
	RawScore      ciValue
	RescoredScore ciValue

	// VerifiedCorrectness is the Phase 89 (REPORT-01) ADDITIVE leaderboard column:
	// the pooled fraction of this (mode x benchmark)'s rows whose decoded
	// verified_correctness *bool (load.go:32) is true, over the rows whose verdict
	// is non-nil. It is reduced at AGGREGATE time by reduceVerifiedCorrectness,
	// mirroring the CanaryPassRate/RawScore pooled-rate precedent: a nil verdict is
	// excluded from BOTH numerator and denominator (never fabricated), a cell with NO
	// verdict anywhere is a NULL ci (OK==false) rendered as an em-dash, and it is a
	// flat pooled rate (NOT a BCa CI) so it consumes ZERO RNG and cannot perturb the
	// locked determinism contract. Unlike the SWE-bench raw/rescored columns above it
	// IS rendered (REPORT-01), so adding it regenerates leaderboard.golden.md.
	VerifiedCorrectness ciValue

	// CostPerSolved is the Phase 89 (REPORT-01) ADDITIVE leaderboard column,
	// SINGLE-SOURCED from the matching CostRow.CostPerSolved (keyed by mode +
	// benchmark) — NOT a second cost reduce (Pitfall 3 / IN-02 discipline). The
	// leaderboard cost MUST equal cost_quality.md's cost for the same row. A null
	// cost (no priced/solved task) renders an em-dash.
	CostPerSolved ciValue
}

// VarianceFlag names a (task,mode) cell whose per-run USD CV exceeds the FAIR-03
// threshold, with the computed CV for transparency.
type VarianceFlag struct {
	Task string
	Mode string
	CV   float64
}

// CostRow is one (mode x benchmark) cost_quality.md row: cost_per_solved_task as
// a BCa CI plus any FAIR-03 variance flags for the cells that fed it.
type CostRow struct {
	Mode          string
	Benchmark     string
	CostPerSolved ciValue
	VarianceFlags []VarianceFlag
}

// LanguageRow is one per-language pass-rate slice (Phase 85, ADAPTER-AIDER-01):
// the language axis read from the additive `language` doc key, the aggregate
// pass-rate across that language's runs, and N (the number of runs that
// contributed a non-nil task_success). It is the SC#1 substrate ("Python
// pass-rate") — additive, never altering the (mode x benchmark) leaderboard. A
// pre-language run (no `language` key) buckets under Language=="". Rendering this
// slice into a report file is downstream Phase 89; here it only needs to exist
// and be correct.
type LanguageRow struct {
	Language string
	PassRate float64
	N        int
}

// Footer is the shared provenance block both reports cite for reproducibility
// (D-08/D-16): seed, bootstrap iterations, CI level, run count, and the
// cost-table valid_until snapshot.
type Footer struct {
	Seed                uint64
	Iterations          int
	CILevel             float64
	Runs                int
	CostTableValidUntil string
}

// Report is the orchestrator's output: the reduced rows for both artifacts plus
// the shared footer. Aggregate builds it; the renderers consume it.
//
// PassNK is the actual k rendered in the leaderboard's pass@<k> column — the
// largest configured k <= ExpectedN (pickKN), which need not equal ExpectedN
// (IN-01). The renderer prints pass@<PassNK> so the header is self-describing.
type Report struct {
	Leaderboard []LeaderRow
	Cost        []CostRow
	// ByLanguage is the Phase 85 (ADAPTER-AIDER-01) per-language pass-rate slice —
	// the SC#1 substrate. Purely additive: it never alters the (mode x benchmark)
	// Leaderboard. A pre-language run buckets under Language=="".
	ByLanguage []LanguageRow
	// Ablations is the Phase 89 (REPORT-03) aggregate-time delta slice: one
	// AblationRow per fixed comparison (full vs {no_lsp, no_semantic,
	// no_structured_edit, baseline_plain, baseline_rag}). The full vs no_semantic
	// pair is computed HERE (deltas.go deliberately omits it). Additive: it never
	// alters the leaderboard.
	Ablations []AblationRow
	PassNK    int
	Footer    Footer
}

// AblationRow is one Phase 89 (REPORT-03) full-vs-other delta: the two modes' BCa
// task_success CIs (re-reduced at aggregate-time), their point delta, and the
// STATS-04 ciOverlap marker. Present==false when the `other` mode is absent from
// the loaded tree (an em-dash delta — never a fabricated 0).
type AblationRow struct {
	Comparison string  // e.g. "full_minus_no_lsp"
	FullCI     ciValue // full mode's task_success BCa CI
	OtherCI    ciValue // the other mode's task_success BCa CI (null if absent)
	Present    bool    // false when the other mode's rows are absent
}

// ablationComparison names one fixed full-vs-other comparison and the canonical
// on-disk mode names of its two operands.
type ablationComparison struct {
	name  string // surfaced comparison key (full_minus_<other>)
	full  string // the full operand mode (your_agent_full)
	other string // the mode subtracted from full
}

// ablationComparisons is the FIXED, ordered Phase 89 (REPORT-03) comparison set:
// full vs each of the 5 ablation arms. full vs no_semantic is INCLUDED here even
// though bench/runtime/deltas.go deliberately omits it as a delta operand — this
// comparison is computed at aggregate-time from the full + no_semantic BCa CIs in
// the loaded tree (Pitfall 2). The operand mode names are the canonical on-disk
// names (your_agent_*, baseline_*) the matrix writes.
var ablationComparisons = []ablationComparison{
	{"full_minus_no_lsp", "your_agent_full", "your_agent_no_lsp"},
	{"full_minus_no_semantic", "your_agent_full", "your_agent_no_semantic"},
	{"full_minus_no_structured_edit", "your_agent_full", "your_agent_no_structured_edit"},
	{"full_minus_baseline_plain", "your_agent_full", "baseline_plain"},
	{"full_minus_baseline_rag", "your_agent_full", "baseline_rag"},
}

// tier1Languages is the FIXED, sorted canonical Tier-1 language set — the 8
// directories under bench/languages: cpp, csharp, go, java, javascript, python,
// rust, typescript. There is deliberately NO `c` (the research example list of 9
// was wrong; verified against the on-disk registry). renderPerLanguage iterates
// this list so a no-coverage language renders n/a rather than being omitted.
var tier1Languages = []string{
	"cpp", "csharp", "go", "java", "javascript", "python", "rust", "typescript",
}

// ciOverlap is the STATS-04 interval-intersection predicate (D-17): two CIs
// overlap iff lo_a <= hi_b && lo_b <= hi_a. A null CI on either side cannot
// support an overlap claim, so it returns false (no comparison is possible).
func ciOverlap(a, b ciValue) bool {
	if !a.OK || !b.OK {
		return false
	}
	return a.Lo <= b.Hi && b.Lo <= a.Hi
}

// coefVariation is the FAIR-03 detector statistic (A2): the coefficient of
// variation (sample stddev / mean) of per-run USD across a (task,mode)'s N runs.
// An empty input or a zero mean returns 0 (no meaningful variance / never a
// NaN/Inf leak from divide-by-zero).
func coefVariation(perRunUSD []float64) float64 {
	n := len(perRunUSD)
	if n < 2 {
		return 0
	}
	var mean float64
	for _, v := range perRunUSD {
		mean += v
	}
	mean /= float64(n)
	if mean == 0 {
		return 0
	}
	var sumSq float64
	for _, v := range perRunUSD {
		d := v - mean
		sumSq += d * d
	}
	// Sample standard deviation (n-1 denominator).
	stddev := math.Sqrt(sumSq / float64(n-1))
	return stddev / mean
}

// fmtCI renders a CI cell as "point [lo, hi]" at a fixed precision, or the
// em-dash for a null CI. Fixed precision keeps the markdown byte-stable (D-08).
func fmtCI(c ciValue) string {
	if !c.OK {
		return emDash
	}
	return fmt.Sprintf("%.4f [%.4f, %.4f]", c.Point, c.Lo, c.Hi)
}

// sortLeaderRows sorts rows by task_success DESC with the mode name as a stable
// tiebreaker (D-16). A null sort-metric CI sorts last (its point is treated as
// the lowest), keeping the ordering total and deterministic.
func sortLeaderRows(rows []LeaderRow) []LeaderRow {
	out := make([]LeaderRow, len(rows))
	copy(out, rows)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := sortKey(out[i].TaskSuccess), sortKey(out[j].TaskSuccess)
		if pi != pj {
			return pi > pj // descending
		}
		return out[i].Mode < out[j].Mode
	})
	return out
}

// sortKey is the descending-sort key for a CI: its point if present, else -Inf so
// a null sorts last.
func sortKey(c ciValue) float64 {
	if !c.OK {
		return math.Inf(-1)
	}
	return c.Point
}

// renderLeaderboard renders the STATS-02/03/04 leaderboard.md: (mode x benchmark)
// rows sorted task_success desc, each metric a `point [lo, hi]` CI (null -> em-
// dash), with the STATS-04 overlap gate annotating adjacent rows whose
// task_success CIs overlap (suppressing the X>Y claim) and a provenance footer.
func renderLeaderboard(rows []LeaderRow, passNK int, footer Footer) string {
	sorted := sortLeaderRows(rows)

	var b strings.Builder
	b.WriteString("# Leaderboard\n\n")
	// IN-01: render the ACTUAL pass@k column header (pickKN result), not a
	// hard-coded "pass@N", so an intermediate k (e.g. KValues={1,2}, N=5 -> k=2)
	// is labelled honestly in the published artifact.
	fmt.Fprintf(&b, "| mode | benchmark | task_success | verified_correctness | pass@1 | pass@%d | tokens_input | tokens_output | tool_calls | files_read | edit_locality | cost_per_solved |\n", passNK)
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.Mode, r.Benchmark,
			fmtCI(r.TaskSuccess), fmtCI(r.VerifiedCorrectness),
			fmtCI(r.PassAt1), fmtCI(r.PassAtN),
			fmtCI(r.TokensInput), fmtCI(r.TokensOutput),
			fmtCI(r.ToolCalls), fmtCI(r.FilesRead), fmtCI(r.EditLocality),
			fmtCI(r.CostPerSolved))
	}

	// STATS-04 overlap gate (D-17): walk adjacent rows in the sorted order; when
	// the higher-ranked row's task_success CI overlaps the next row's CI, emit a
	// warning that suppresses the directional superiority claim for that pair.
	overlaps := overlapWarnings(sorted)
	if len(overlaps) > 0 {
		b.WriteString("\n## CI overlap warnings (STATS-04)\n\n")
		for _, w := range overlaps {
			b.WriteString(w)
			b.WriteByte('\n')
		}
	}

	b.WriteString("\n")
	b.WriteString(renderFooter(footer))
	return b.String()
}

// overlapWarnings returns one warning line per adjacent row pair whose
// task_success CIs overlap. The slice is already in sorted (rank) order so the
// pairing and the emitted lines are deterministic.
func overlapWarnings(sorted []LeaderRow) []string {
	var out []string
	for i := 0; i+1 < len(sorted); i++ {
		a, next := sorted[i], sorted[i+1]
		if ciOverlap(a.TaskSuccess, next.TaskSuccess) {
			out = append(out, fmt.Sprintf(
				"- %s vs %s: task_success CI overlap %s vs %s — no X>Y claim",
				a.Mode, next.Mode, fmtCI(a.TaskSuccess), fmtCI(next.TaskSuccess)))
		}
	}
	return out
}

// sortCostRows sorts cost rows by cost_per_solved ascending (cheaper first) with
// mode name as the tiebreaker; a null cost CI sorts last. Deterministic.
func sortCostRows(rows []CostRow) []CostRow {
	out := make([]CostRow, len(rows))
	copy(out, rows)
	sort.SliceStable(out, func(i, j int) bool {
		ci, cj := out[i].CostPerSolved, out[j].CostPerSolved
		ki, kj := costSortKey(ci), costSortKey(cj)
		if ki != kj {
			return ki < kj // ascending (cheaper first)
		}
		return out[i].Mode < out[j].Mode
	})
	return out
}

// costSortKey sorts a present cost CI by its point; a null cost sorts last (+Inf).
func costSortKey(c ciValue) float64 {
	if !c.OK {
		return math.Inf(1)
	}
	return c.Point
}

// renderCostQuality renders the COST-03 cost_quality.md: (mode x benchmark) rows
// with cost_per_solved_task as a BCa CI (null -> em-dash), a FAIR-03 variance
// section naming any cell whose per-run USD CV exceeds the threshold, and the
// cost-table valid_until cited in the footer.
func renderCostQuality(rows []CostRow, footer Footer) string {
	sorted := sortCostRows(rows)

	var b strings.Builder
	b.WriteString("# Cost vs Quality\n\n")
	b.WriteString("| mode | benchmark | cost_per_solved_task |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, r := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.Mode, r.Benchmark, fmtCI(r.CostPerSolved))
	}

	// FAIR-03 between-run variance warnings (D-15): collect every flagged cell
	// across all rows, sort for determinism, and render a named warning section.
	flags := collectVarianceFlags(sorted)
	if len(flags) > 0 {
		b.WriteString("\n## FAIR-03 between-run variance warnings (CV > 0.05)\n\n")
		for _, f := range flags {
			fmt.Fprintf(&b, "- %s / %s: per-run USD coefficient of variation %.4f exceeds %.2f\n",
				f.Task, f.Mode, f.CV, cvThreshold)
		}
	}

	b.WriteString("\n")
	b.WriteString(renderFooter(footer))
	return b.String()
}

// collectVarianceFlags flattens and sorts all FAIR-03 flags across rows so the
// rendered warning section is deterministic (task then mode).
func collectVarianceFlags(rows []CostRow) []VarianceFlag {
	var flags []VarianceFlag
	for _, r := range rows {
		flags = append(flags, r.VarianceFlags...)
	}
	sort.SliceStable(flags, func(i, j int) bool {
		if flags[i].Task != flags[j].Task {
			return flags[i].Task < flags[j].Task
		}
		return flags[i].Mode < flags[j].Mode
	})
	return flags
}

// renderFooter emits the shared provenance footer with a FIXED field order for
// byte-stability (D-08).
func renderFooter(f Footer) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "seed: %d\n", f.Seed)
	fmt.Fprintf(&b, "bootstrap_iterations: %d\n", f.Iterations)
	fmt.Fprintf(&b, "ci_level: %.2f\n", f.CILevel)
	fmt.Fprintf(&b, "runs: %d\n", f.Runs)
	fmt.Fprintf(&b, "cost_table_valid_until: %s\n", f.CostTableValidUntil)
	return b.String()
}

// renderPerLanguage renders the REPORT-02 per_language.md: ALL 8 Tier-1
// languages (tier1Languages) in fixed order, each row carrying the pooled
// pass_rate (%.4f) + n when the language has benchmark coverage in byLang, or
// `n/a` when it has none (NOT omitted — never a fabricated 0). The "" / non-Tier-1
// buckets in byLang are intentionally NOT rendered (only the fixed Tier-1 axis).
// RNG-free and byte-stable: it indexes byLang into a map then iterates the fixed
// slice, so no map-iteration order leaks into the output.
func renderPerLanguage(byLang []LanguageRow, footer Footer) string {
	idx := make(map[string]LanguageRow, len(byLang))
	for _, r := range byLang {
		idx[r.Language] = r
	}

	var b strings.Builder
	b.WriteString("# Per-Language Pass Rate\n\n")
	b.WriteString("| language | pass_rate | n |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, lang := range tier1Languages {
		if r, ok := idx[lang]; ok {
			fmt.Fprintf(&b, "| %s | %.4f | %d |\n", lang, r.PassRate, r.N)
		} else {
			// No-coverage Tier-1 language: n/a, never a fabricated 0 (REPORT-02).
			fmt.Fprintf(&b, "| %s | n/a | n/a |\n", lang)
		}
	}

	b.WriteString("\n")
	b.WriteString(renderFooter(footer))
	return b.String()
}

// renderAblations renders the REPORT-03 ablations.md: one delta table per fixed
// ablationComparisons entry (full vs {no_lsp, no_semantic, no_structured_edit,
// baseline_plain, baseline_rag}). Each present row shows the two operands'
// task_success CIs, their point delta, and a STATS-04 CI-overlap marker (via the
// reused ciOverlap predicate — no new overlap logic). A row whose `other` operand
// is absent (Present==false) renders an em-dash delta, never a fabricated 0. Rows
// are emitted in the fixed comparison order (already deterministic). RNG-free and
// byte-stable.
func renderAblations(rows []AblationRow, footer Footer) string {
	var b strings.Builder
	b.WriteString("# Ablation Deltas\n\n")
	b.WriteString("| comparison | full_task_success | other_task_success | delta | ci_overlap |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		delta := emDash
		overlap := emDash
		if r.Present && r.FullCI.OK && r.OtherCI.OK {
			delta = fmt.Sprintf("%.4f", r.FullCI.Point-r.OtherCI.Point)
			if ciOverlap(r.FullCI, r.OtherCI) {
				overlap = "CI overlap — no X>Y claim"
			} else {
				overlap = "disjoint"
			}
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			r.Comparison, fmtCI(r.FullCI), fmtCI(r.OtherCI), delta, overlap)
	}

	b.WriteString("\n")
	b.WriteString(renderFooter(footer))
	return b.String()
}

// writeReport atomically writes report markdown into runDir via a temp-file +
// rename, mirroring cell.go:writeDurable so a concurrent reader never observes a
// torn report file (T-82-06-03). Markdown is NOT schema-validated — these are
// human reports, not result.v2 rows, so there is no Validate call.
func writeReport(runDir, name, content string) error {
	path := filepath.Join(runDir, name)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return fmt.Errorf("aggregate: mkdir %q: %w", runDir, err)
	}
	tmp, err := os.CreateTemp(runDir, ".tmp-"+name+"-*")
	if err != nil {
		return fmt.Errorf("aggregate: create temp for %q: %w", path, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("aggregate: write temp for %q: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("aggregate: close temp for %q: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("aggregate: rename temp into %q: %w", path, err)
	}
	return nil
}
