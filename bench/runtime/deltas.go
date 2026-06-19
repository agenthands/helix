// Package runtime — deltas.go owns the minimal in-phase post-run 3-delta pass
// (D-05). It runs at the MATRIX tier (the only layer that sees every mode for a
// task) strictly AFTER RunMatrix returns its Summary (wg.Wait() has completed —
// Pitfall 3): the cell layer sees one mode at a time and could never compute a
// cross-mode delta. For each task that has ALL FOUR real-mode rows on disk
// (your_agent_full, baseline_plain, your_agent_no_lsp,
// your_agent_no_structured_edit), it computes EXACTLY 3 deltas —
// full − baseline_plain, full − no_lsp, full − no_structured_edit — over a small
// fixed set of comparable numeric metrics, and surfaces them in EACH of the four
// per-mode result.v2.json rows under a new open top-level property
// `ablation_deltas` (re-validated via Validate, written atomically via
// writeDurable). A task missing any of the four real modes is SKIPPED (no panic,
// no nil-baseline arithmetic) and reported.
//
// SCOPE GUARD: this is single-run / 3-fixed-deltas ONLY. It is NOT the Phase 82
// aggregator — there is deliberately no multi-run aggregation, BCa bootstrap,
// pass@k, variance gate, or leaderboard here. The your_agent_no_semantic partial
// arm and the baseline_rag fail-closed stub (which writes no row) are excluded
// from the delta operands by construction: they are not among the four real modes.
package runtime

import (
	"encoding/json"
	"fmt"
	"os"
)

// The four real ablation modes whose rows the delta pass operates over. The
// your_agent_no_semantic partial arm and the baseline_rag deferred stub are
// deliberately absent: the 3 deltas are full vs the three honest ablations.
const (
	modeFull             = "your_agent_full"
	modeBaselinePlain    = "baseline_plain"
	modeNoLSP            = "your_agent_no_lsp"
	modeNoStructuredEdit = "your_agent_no_structured_edit"
)

// deltaComparisons names the three fixed comparisons in the surfaced
// ablation_deltas object, each `full − <other>`. The keys are stable, greppable
// snake_case (the Phase 82 aggregator and any report reads them by name).
var deltaComparisons = []struct {
	name  string // surfaced ablation_deltas key
	other string // the mode subtracted from full
}{
	{"full_minus_baseline_plain", modeBaselinePlain},
	{"full_minus_no_lsp", modeNoLSP},
	{"full_minus_no_structured_edit", modeNoStructuredEdit},
}

// requiredModes is the set of real modes a task must have rows for before any
// delta is computed (Pitfall 3 — skip a task missing any of them).
var requiredModes = []string{modeFull, modeBaselinePlain, modeNoLSP, modeNoStructuredEdit}

// comparableMetric names a numeric metric the delta operates on and its accessor
// into evaluators.Metrics. Both int and float metrics project to float64 deltas
// so the surfaced object is uniformly numeric. A metric that is null (nil) in
// EITHER operand is skipped for that comparison (no fabricated 0).
type comparableMetric struct {
	key string
	val func(rowMetrics) (float64, bool)
}

// rowMetrics is the subset of the metrics object the delta reads. It is decoded
// from the on-disk row's `metrics` object; absent/null fields stay nil.
type rowMetrics struct {
	TokensInput   *float64 `json:"tokens_input"`
	TokensOutput  *float64 `json:"tokens_output"`
	ToolCalls     *float64 `json:"tool_calls"`
	FilesModified *float64 `json:"files_modified"`
	EditLocality  *float64 `json:"edit_locality"`
}

// comparableMetrics is the small, clearly-scoped metric set the 3 deltas cover
// (RESEARCH "keep the metric set small"). Each is skipped when null in either
// operand. NOT the full 19-field Metrics record — only the comparable run-cost /
// behavior metrics that make a cross-mode ablation delta meaningful.
var comparableMetrics = []comparableMetric{
	{"tokens_input", func(m rowMetrics) (float64, bool) { return deref(m.TokensInput) }},
	{"tokens_output", func(m rowMetrics) (float64, bool) { return deref(m.TokensOutput) }},
	{"tool_calls", func(m rowMetrics) (float64, bool) { return deref(m.ToolCalls) }},
	{"files_modified", func(m rowMetrics) (float64, bool) { return deref(m.FilesModified) }},
	{"edit_locality", func(m rowMetrics) (float64, bool) { return deref(m.EditLocality) }},
}

func deref(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}

// SkippedTask records a task the delta pass could not process and why (a real
// mode's row was missing). It is reported in DeltaReport so the operator log /
// tests can assert "skipped, not silently dropped" (Pitfall 3).
type SkippedTask struct {
	Task   string
	Reason string
}

// DeltaReport summarizes a ComputeAndWriteDeltas pass: the tasks whose 3 deltas
// were computed + written back, and the tasks skipped for a missing real mode.
type DeltaReport struct {
	Computed []string
	Skipped  []SkippedTask
}

// loadedRow is a parsed per-mode row plus its on-disk path (for write-back).
type loadedRow struct {
	path    string
	doc     map[string]json.RawMessage // full row, preserved verbatim for write-back
	metrics rowMetrics
}

// ComputeAndWriteDeltas is the D-05 post-RunMatrix delta pass. It groups the
// matrix outcomes by task, and for every task with all four real-mode rows
// present on disk, computes the 3 fixed deltas over the comparable metrics and
// writes them back into EACH of the four per-mode rows under `ablation_deltas`
// (re-validated, atomic). A task missing any real mode is skipped and reported.
// The returned error is non-nil only for a write-back IO/validation failure on a
// COMPLETE task — a missing mode is a skip, never an error.
//
// MUST be called AFTER RunMatrix returns (the matrix barrier — Pitfall 3); the
// cell tier never sees more than one mode for a task.
func ComputeAndWriteDeltas(outcomes []CellOutcome) (DeltaReport, error) {
	var report DeltaReport

	// Group the per-mode result paths by task. A deferred/errored cell (e.g.
	// baseline_rag) has no ResultPath row on disk; we only index outcomes that
	// name one of the four real modes AND carry a result path.
	byTask := make(map[string]map[string]string) // task -> mode -> resultPath
	var taskOrder []string
	for _, oc := range outcomes {
		task := oc.Cell.Task
		mode := oc.Cell.Mode
		if !isRequiredMode(mode) {
			continue // no_semantic partial + baseline_rag stub are not delta operands
		}
		if oc.Result.ResultPath == "" {
			continue
		}
		if _, ok := byTask[task]; !ok {
			byTask[task] = make(map[string]string)
			taskOrder = append(taskOrder, task)
		}
		byTask[task][mode] = oc.Result.ResultPath
	}

	for _, task := range taskOrder {
		modes := byTask[task]

		// Pitfall 3: a task missing any of the four real modes is skipped, never
		// computed against a nil baseline.
		if missing := firstMissingMode(modes); missing != "" {
			report.Skipped = append(report.Skipped, SkippedTask{
				Task:   task,
				Reason: fmt.Sprintf("missing real-mode row %q", missing),
			})
			continue
		}

		rows, err := loadTaskRows(modes)
		if err != nil {
			// A row named in the outcomes but unreadable/missing on disk is a skip,
			// not a hard error — the matrix may have failed that cell's write.
			report.Skipped = append(report.Skipped, SkippedTask{Task: task, Reason: err.Error()})
			continue
		}

		deltas := computeTaskDeltas(rows)

		if err := writeBackDeltas(rows, deltas); err != nil {
			return report, fmt.Errorf("bench/runtime: write back deltas for task %q: %w", task, err)
		}
		report.Computed = append(report.Computed, task)
	}

	return report, nil
}

// computeTaskDeltas produces the 3 fixed deltas (full − baseline_plain,
// full − no_lsp, full − no_structured_edit) over the comparable metrics. A
// metric null in either operand is skipped for that comparison. The result is
// the surfaced ablation_deltas object: comparison name -> metric -> delta.
func computeTaskDeltas(rows map[string]loadedRow) map[string]map[string]float64 {
	full := rows[modeFull].metrics
	out := make(map[string]map[string]float64, len(deltaComparisons))
	for _, cmp := range deltaComparisons {
		other := rows[cmp.other].metrics
		md := make(map[string]float64)
		for _, m := range comparableMetrics {
			fv, fok := m.val(full)
			ov, ook := m.val(other)
			if !fok || !ook {
				continue // skip metrics that are null in either operand
			}
			md[m.key] = fv - ov
		}
		out[cmp.name] = md
	}
	return out
}

// writeBackDeltas writes the computed deltas into EACH of the four per-mode rows
// under `ablation_deltas`, re-marshals the row preserving all existing fields,
// re-validates it against the schema, and writes it atomically via writeDurable.
// All four rows carry the SAME deltas object (criterion #4: "surface in the
// per-mode result rows" — every mode's row reports the task's deltas).
func writeBackDeltas(rows map[string]loadedRow, deltas map[string]map[string]float64) error {
	deltaBytes, err := json.Marshal(deltas)
	if err != nil {
		return fmt.Errorf("marshal ablation_deltas: %w", err)
	}
	for _, mode := range requiredModes {
		row := rows[mode]
		// Preserve every existing field; add/overwrite ablation_deltas only.
		row.doc["ablation_deltas"] = json.RawMessage(deltaBytes)
		b, merr := json.MarshalIndent(row.doc, "", "  ")
		if merr != nil {
			return fmt.Errorf("marshal row %q: %w", row.path, merr)
		}
		if verr := Validate(b); verr != nil {
			return fmt.Errorf("row %q invalid after delta write-back: %w", row.path, verr)
		}
		if werr := writeDurable(row.path, b); werr != nil {
			return fmt.Errorf("write row %q: %w", row.path, werr)
		}
	}
	return nil
}

// loadTaskRows reads + decodes each of the four real-mode rows from disk. It
// preserves the full row as a map (for verbatim write-back) and extracts the
// comparable metrics subset for the arithmetic.
func loadTaskRows(modes map[string]string) (map[string]loadedRow, error) {
	rows := make(map[string]loadedRow, len(modes))
	for _, mode := range requiredModes {
		path := modes[mode]
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read row for mode %q: %w", mode, err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("decode row for mode %q: %w", mode, err)
		}
		var rm rowMetrics
		if raw, ok := doc["metrics"]; ok {
			if err := json.Unmarshal(raw, &rm); err != nil {
				return nil, fmt.Errorf("decode metrics for mode %q: %w", mode, err)
			}
		}
		rows[mode] = loadedRow{path: path, doc: doc, metrics: rm}
	}
	return rows, nil
}

// isRequiredMode reports whether mode is one of the four real delta operands.
func isRequiredMode(mode string) bool {
	for _, m := range requiredModes {
		if m == mode {
			return true
		}
	}
	return false
}

// firstMissingMode returns the first required real mode absent from modes, or ""
// when all four are present.
func firstMissingMode(modes map[string]string) string {
	for _, m := range requiredModes {
		if _, ok := modes[m]; !ok {
			return m
		}
	}
	return ""
}
