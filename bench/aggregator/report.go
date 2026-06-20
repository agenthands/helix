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
type Report struct {
	Leaderboard []LeaderRow
	Cost        []CostRow
	Footer      Footer
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
func renderLeaderboard(rows []LeaderRow, footer Footer) string {
	sorted := sortLeaderRows(rows)

	var b strings.Builder
	b.WriteString("# Leaderboard\n\n")
	b.WriteString("| mode | benchmark | task_success | pass@1 | pass@N | tokens_input | tokens_output | tool_calls | files_read | edit_locality |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.Mode, r.Benchmark,
			fmtCI(r.TaskSuccess), fmtCI(r.PassAt1), fmtCI(r.PassAtN),
			fmtCI(r.TokensInput), fmtCI(r.TokensOutput),
			fmtCI(r.ToolCalls), fmtCI(r.FilesRead), fmtCI(r.EditLocality))
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
