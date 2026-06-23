package aggregator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// aider_edit_baseline.go owns the Phase 100 (BASELINE-01) deterministic renderer for
// the committed polyglot-edit baseline summary. RenderAiderEditBaseline takes the
// committed result.v2.json bytes and emits a FIXED-FORMAT BENCH-RESULTS.md keyed on
// the deterministic metrics only — no latency, no timestamp, no absolute path, no
// map-iteration-ordered field (every list is sorted before emit). It is a pure
// function: the same input always produces byte-identical output, which is what makes
// the committed BENCH-RESULTS.md byte-reproducible (proven by the double-render
// diff-empty test, mirroring byte_reproducible_test.go).

// aiderEditBaselineRow is the subset of result.v2 the baseline summary restates. Only
// deterministic fields are decoded; latency/timestamp fields are intentionally absent
// from this struct so they can never leak into the rendered summary.
type aiderEditBaselineRow struct {
	SchemaVersion     string `json:"schema_version"`
	TaskID            string `json:"task_id"`
	Mode              string `json:"mode"`
	Benchmark         string `json:"benchmark"`
	Language          string `json:"language"`
	Outcome           string `json:"outcome"`
	EditFormatApplied *bool  `json:"edit_format_applied"`
	Metrics           struct {
		TaskSuccess         *bool `json:"task_success"`
		VerifiedCorrectness *bool `json:"verified_correctness"`
	} `json:"metrics"`
}

// RenderAiderEditBaseline renders the deterministic BENCH-RESULTS.md summary for the
// committed aider-edit baseline from its result.v2.json bytes. A missing/invalid
// edit_format_applied or an unparseable document is a HARD error (fail-CLOSED) — the
// renderer never emits a summary that silently drops the load-bearing key.
func RenderAiderEditBaseline(resultV2 []byte) ([]byte, error) {
	var row aiderEditBaselineRow
	if err := json.Unmarshal(resultV2, &row); err != nil {
		return nil, fmt.Errorf("bench/aggregator: parse aider-edit baseline result.v2: %w", err)
	}
	if row.EditFormatApplied == nil {
		return nil, fmt.Errorf("bench/aggregator: aider-edit baseline result.v2 is missing edit_format_applied (fail-closed)")
	}

	boolStr := func(p *bool) string {
		if p == nil {
			return "n/a"
		}
		if *p {
			return "true"
		}
		return "false"
	}
	// Deterministic metric rows, sorted by name before emit so the order can never
	// drift on a map-iteration change (Pitfall 3). ONLY the truly reproducible quality
	// metrics are restated — the repo-derived patch_validator metrics
	// (files_modified / edit_locality / edit_distance_patch) are excluded from the
	// committed baseline (they vary with the live git working tree) and are nulled in
	// the result.v2 by assembleAiderEditResult.
	type kv struct{ k, v string }
	metricRows := []kv{
		{"edit_format_applied", boolStr(row.EditFormatApplied)},
		{"outcome", row.Outcome},
		{"task_success", boolStr(row.Metrics.TaskSuccess)},
		{"verified_correctness", boolStr(row.Metrics.VerifiedCorrectness)},
	}
	sort.Slice(metricRows, func(i, j int) bool { return metricRows[i].k < metricRows[j].k })

	var b strings.Builder
	b.WriteString("# Polyglot-Edit Committed Baseline (aider_edit)\n\n")
	b.WriteString("This is the **committed, byte-reproducible** polyglot-edit baseline (BASELINE-01).\n")
	b.WriteString("It is produced by a **deterministic scripted agent** that applies the exercism\n")
	b.WriteString("reference solution (`.meta/example.go`) to the solution stub through the helix\n")
	b.WriteString("`replace_in_file` EDIT verb against the warm daemon, then grades with the dataset's\n")
	b.WriteString("native `go test`. It carries **deterministic metrics only** — no live latency, no\n")
	b.WriteString("timestamp, no absolute path, no live tokens. Regenerate locally with\n")
	b.WriteString("`make bench-aider-edit`.\n\n")

	b.WriteString("## Cell\n\n")
	b.WriteString("| field | value |\n")
	b.WriteString("| --- | --- |\n")
	b.WriteString(fmt.Sprintf("| schema_version | %s |\n", row.SchemaVersion))
	b.WriteString(fmt.Sprintf("| benchmark | %s |\n", row.Benchmark))
	b.WriteString(fmt.Sprintf("| task_id | %s |\n", row.TaskID))
	b.WriteString(fmt.Sprintf("| mode | %s |\n", row.Mode))
	b.WriteString(fmt.Sprintf("| language | %s |\n", row.Language))
	b.WriteString("\n")

	b.WriteString("## Deterministic Metrics\n\n")
	b.WriteString("| metric | value |\n")
	b.WriteString("| --- | --- |\n")
	for _, r := range metricRows {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", r.k, r.v))
	}

	return []byte(b.String()), nil
}
