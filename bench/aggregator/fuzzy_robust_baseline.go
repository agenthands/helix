package aggregator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// fuzzy_robust_baseline.go owns the Phase 102 (BASELINE-02) deterministic renderer for the
// committed fuzzy-robustness baseline summary. RenderFuzzyRobustBaseline takes the committed
// result.v2.json bytes and emits a FIXED-FORMAT BENCH-RESULTS.md keyed on the deterministic
// strategy-selection summary + the load-bearing ambiguous-refused assertion only — no
// latency, no timestamp, no absolute path, no map-iteration-ordered field (every list is
// sorted before emit). It is a pure function: the same input always produces byte-identical
// output, which makes the committed BENCH-RESULTS.md byte-reproducible (proven by the
// double-render diff-empty test). Mirrors RenderAiderEditBaseline field-by-field: decode ONLY
// deterministic fields, fail-CLOSED on the missing/false ambiguous_refused assertion (the
// EditFormatApplied == nil analog, D-06).

// fuzzyRobustBaselineRow is the subset of result.v2 the baseline summary restates. Only the
// deterministic per-strategy summary + the ambiguous-refused assertion are decoded; any
// latency/timestamp field is intentionally absent so it can never leak into the rendered
// summary.
type fuzzyRobustBaselineRow struct {
	SchemaVersion string `json:"schema_version"`
	TaskID        string `json:"task_id"`
	Mode          string `json:"mode"`
	Benchmark     string `json:"benchmark"`
	Language      string `json:"language"`
	FuzzyRobust   struct {
		CorpusCases      int            `json:"corpus_cases"`
		CorpusLanguages  []string       `json:"corpus_languages"`
		AmbiguousCases   int            `json:"ambiguous_cases"`
		AmbiguousRefused *bool          `json:"ambiguous_refused"`
		StrategyCounts   map[string]int `json:"strategy_counts"`
	} `json:"fuzzy_robust"`
}

// RenderFuzzyRobustBaseline renders the deterministic BENCH-RESULTS.md summary for the
// committed fuzzy-robustness baseline from its result.v2.json bytes. A missing or false
// ambiguous_refused assertion, or an unparseable document, is a HARD error (fail-CLOSED) —
// the renderer never emits a summary that silently drops the must-refuse assertion.
func RenderFuzzyRobustBaseline(resultV2 []byte) ([]byte, error) {
	var row fuzzyRobustBaselineRow
	if err := json.Unmarshal(resultV2, &row); err != nil {
		return nil, fmt.Errorf("bench/aggregator: parse fuzzy-robust baseline result.v2: %w", err)
	}
	fr := row.FuzzyRobust
	if fr.AmbiguousRefused == nil {
		return nil, fmt.Errorf("bench/aggregator: fuzzy-robust baseline result.v2 is missing ambiguous_refused (fail-closed)")
	}
	if !*fr.AmbiguousRefused {
		return nil, fmt.Errorf("bench/aggregator: fuzzy-robust baseline ambiguous_refused is false — the must-refuse assertion did not hold (fail-closed)")
	}

	// Deterministic per-strategy rows, sorted by strategy name before emit so the order can
	// never drift on a map-iteration change (Pitfall 3).
	type kv struct {
		k string
		v int
	}
	strategyRows := make([]kv, 0, len(fr.StrategyCounts))
	for name, count := range fr.StrategyCounts {
		strategyRows = append(strategyRows, kv{name, count})
	}
	sort.Slice(strategyRows, func(i, j int) bool { return strategyRows[i].k < strategyRows[j].k })

	langs := append([]string(nil), fr.CorpusLanguages...)
	sort.Strings(langs)

	var b strings.Builder
	b.WriteString("# Fuzzy-Robustness Committed Baseline (fuzzy_robust)\n\n")
	b.WriteString("This is the **committed, byte-reproducible** fuzzy-robustness baseline (BASELINE-02).\n")
	b.WriteString("It scores `internal/fuzzy`'s 4-strategy cascade selection and ambiguity refusal\n")
	b.WriteString("against a committed py/go/rust drift corpus (each case a real vendored fixture block\n")
	b.WriteString("transformed by a deterministic per-tier perturbation), gated by a duplicate-block\n")
	b.WriteString("must-refuse `ambiguous_match` assertion (D-06). It carries **deterministic metrics\n")
	b.WriteString("only** — no live latency, no timestamp, no absolute path, no live tokens. Regenerate\n")
	b.WriteString("locally with `make bench-fuzzy-robust`.\n\n")

	b.WriteString("## Cell\n\n")
	b.WriteString("| field | value |\n")
	b.WriteString("| --- | --- |\n")
	b.WriteString(fmt.Sprintf("| schema_version | %s |\n", row.SchemaVersion))
	b.WriteString(fmt.Sprintf("| benchmark | %s |\n", row.Benchmark))
	b.WriteString(fmt.Sprintf("| task_id | %s |\n", row.TaskID))
	b.WriteString(fmt.Sprintf("| mode | %s |\n", row.Mode))
	b.WriteString(fmt.Sprintf("| language | %s |\n", row.Language))
	b.WriteString(fmt.Sprintf("| corpus_cases | %d |\n", fr.CorpusCases))
	b.WriteString(fmt.Sprintf("| corpus_languages | %s |\n", strings.Join(langs, ", ")))
	b.WriteString("\n")

	b.WriteString("## Strategy Selection (deterministic)\n\n")
	b.WriteString("| strategy | count |\n")
	b.WriteString("| --- | --- |\n")
	for _, r := range strategyRows {
		b.WriteString(fmt.Sprintf("| %s | %d |\n", r.k, r.v))
	}
	b.WriteString("\n")

	b.WriteString("## Ambiguity Refusal (anti-vacuity, D-06)\n\n")
	b.WriteString("| field | value |\n")
	b.WriteString("| --- | --- |\n")
	b.WriteString(fmt.Sprintf("| ambiguous_cases | %d |\n", fr.AmbiguousCases))
	b.WriteString(fmt.Sprintf("| ambiguous_refused | %t |\n", *fr.AmbiguousRefused))

	return []byte(b.String()), nil
}
