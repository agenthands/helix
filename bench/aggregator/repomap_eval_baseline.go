package aggregator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// repomap_eval_baseline.go owns the Phase 102 (BASELINE-02) deterministic renderer for
// the committed RepoMap-eval baseline summary. RenderRepoMapEvalBaseline takes the
// committed result.v2.json bytes and emits a FIXED-FORMAT BENCH-RESULTS.md keyed on the
// deterministic ranking-quality metrics only — no latency, no timestamp, no absolute
// path, no map-iteration-ordered field (every list is sorted before emit). It is a pure
// function: the same input always produces byte-identical output, which is what makes the
// committed BENCH-RESULTS.md byte-reproducible (proven by the double-render diff-empty
// test, mirroring aider_edit_baseline_test.go). Mirrors RenderAiderEditBaseline
// field-by-field: decode ONLY deterministic fields, fail-CLOSED on the missing headline
// nDCG@10 (the EditFormatApplied == nil analog).

// repoMapEvalBaselineRow is the subset of result.v2 the baseline summary restates. Only
// deterministic ranking-quality metrics are decoded; any latency/timestamp field is
// intentionally absent from this struct so it can never leak into the rendered summary.
type repoMapEvalBaselineRow struct {
	SchemaVersion string `json:"schema_version"`
	TaskID        string `json:"task_id"`
	Mode          string `json:"mode"`
	Benchmark     string `json:"benchmark"`
	Language      string `json:"language"`
	RepoMapEval   struct {
		CorpusExercises int      `json:"corpus_exercises"`
		CorpusLanguages []string `json:"corpus_languages"`
		NDCGAt10        *float64 `json:"ndcg_at_10"`
		RecallAt10      *float64 `json:"recall_at_10"`
		MRR             *float64 `json:"mrr"`
		BudgetFitRatio  *float64 `json:"budget_fit_ratio"`
		Discriminator   struct {
			Margin               *float64 `json:"margin"`
			ForwardNDCGAt10      *float64 `json:"forward_ndcg_at_10"`
			ReversedNDCGAt10     *float64 `json:"reversed_ndcg_at_10"`
			SeededRandomNDCGAt10 *float64 `json:"seeded_random_ndcg_at_10"`
		} `json:"discriminator"`
	} `json:"repomap_eval"`
}

// RenderRepoMapEvalBaseline renders the deterministic BENCH-RESULTS.md summary for the
// committed RepoMap-eval baseline from its result.v2.json bytes. A missing/invalid headline
// nDCG@10 or an unparseable document is a HARD error (fail-CLOSED) — the renderer never
// emits a summary that silently drops the load-bearing metric.
func RenderRepoMapEvalBaseline(resultV2 []byte) ([]byte, error) {
	var row repoMapEvalBaselineRow
	if err := json.Unmarshal(resultV2, &row); err != nil {
		return nil, fmt.Errorf("bench/aggregator: parse repomap-eval baseline result.v2: %w", err)
	}
	re := row.RepoMapEval
	if re.NDCGAt10 == nil {
		return nil, fmt.Errorf("bench/aggregator: repomap-eval baseline result.v2 is missing ndcg_at_10 (fail-closed)")
	}

	fStr := func(p *float64) string {
		if p == nil {
			return "n/a"
		}
		return strconv.FormatFloat(*p, 'f', 4, 64)
	}
	// Deterministic metric rows, sorted by name before emit so the order can never drift
	// on a map-iteration change. ONLY the truly reproducible ranking-quality metrics are
	// restated — no latency, no timestamp, no absolute path.
	type kv struct{ k, v string }
	metricRows := []kv{
		{"budget_fit_ratio", fStr(re.BudgetFitRatio)},
		{"mrr", fStr(re.MRR)},
		{"ndcg_at_10", fStr(re.NDCGAt10)},
		{"recall_at_10", fStr(re.RecallAt10)},
	}
	sort.Slice(metricRows, func(i, j int) bool { return metricRows[i].k < metricRows[j].k })

	discRows := []kv{
		{"forward_ndcg_at_10", fStr(re.Discriminator.ForwardNDCGAt10)},
		{"margin", fStr(re.Discriminator.Margin)},
		{"reversed_ndcg_at_10", fStr(re.Discriminator.ReversedNDCGAt10)},
		{"seeded_random_ndcg_at_10", fStr(re.Discriminator.SeededRandomNDCGAt10)},
	}
	sort.Slice(discRows, func(i, j int) bool { return discRows[i].k < discRows[j].k })

	langs := append([]string(nil), re.CorpusLanguages...)
	sort.Strings(langs)

	var b strings.Builder
	b.WriteString("# RepoMap-Eval Committed Baseline (repomap_eval)\n\n")
	b.WriteString("This is the **committed, byte-reproducible** RepoMap ranking-quality baseline\n")
	b.WriteString("(BASELINE-02). It scores committed symbol-level gold (authored from each exercise's\n")
	b.WriteString("`.meta/example.*` reference solution) against committed captured `get-repo-map` /\n")
	b.WriteString("`get-context` rankings via recall@10 / MRR / nDCG@10 / budget-fit, gated by a\n")
	b.WriteString("reversed-AND-seeded-random discriminator that bites the corpus by a committed margin.\n")
	b.WriteString("It carries **deterministic metrics only** — no live latency, no timestamp, no\n")
	b.WriteString("absolute path, no live tokens. Regenerate locally with `make bench-repomap-eval`.\n\n")

	b.WriteString("## Cell\n\n")
	b.WriteString("| field | value |\n")
	b.WriteString("| --- | --- |\n")
	b.WriteString(fmt.Sprintf("| schema_version | %s |\n", row.SchemaVersion))
	b.WriteString(fmt.Sprintf("| benchmark | %s |\n", row.Benchmark))
	b.WriteString(fmt.Sprintf("| task_id | %s |\n", row.TaskID))
	b.WriteString(fmt.Sprintf("| mode | %s |\n", row.Mode))
	b.WriteString(fmt.Sprintf("| language | %s |\n", row.Language))
	b.WriteString(fmt.Sprintf("| corpus_exercises | %d |\n", re.CorpusExercises))
	b.WriteString(fmt.Sprintf("| corpus_languages | %s |\n", strings.Join(langs, ", ")))
	b.WriteString("\n")

	b.WriteString("## Deterministic Metrics\n\n")
	b.WriteString("| metric | value |\n")
	b.WriteString("| --- | --- |\n")
	for _, r := range metricRows {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", r.k, r.v))
	}
	b.WriteString("\n")

	b.WriteString("## Discriminator (anti-vacuity, nDCG@10)\n\n")
	b.WriteString("| field | value |\n")
	b.WriteString("| --- | --- |\n")
	for _, r := range discRows {
		b.WriteString(fmt.Sprintf("| %s | %s |\n", r.k, r.v))
	}

	return []byte(b.String()), nil
}
