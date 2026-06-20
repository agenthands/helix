package aggregator

import "github.com/agenthands/helix/bench/cost"

// cost.go is the COST-02 rollup: it turns the canonical nullable token metrics
// (rowMetrics, mirrored from evaluators.Metrics in load.go) into per-result USD
// against a cost-table row, then reduces a per-solved-task USD map to the
// cost_per_solved_task headline. The cost-table parsing + the D-13 fail-closed
// freshness gate are NOT re-implemented here: they are delegated to the
// bench/cost package (Open Q1 — one source of truth). Callers in Plan 06 do
// `ct, _ := cost.LoadCostTable(path)` once, then `row, err := cost.PriceFor(ct,
// modelID, today)` per result (the error surfaces a stale/unknown row), then
// `perResultUSD(metrics, row)` for the USD.

// perResultUSD computes the USD cost of a single result from its token metrics
// and the priced cost-table row (D-12/D-14):
//
//	usd = ( tokens_input             * input_per_mtok
//	      + tokens_input_cached_read * cached_input_per_mtok
//	      + tokens_input_cache_write * input_per_mtok        // D-14 v1 approximation
//	      + tokens_output            * output_per_mtok
//	      ) / 1_000_000
//
// Each term is SKIPPED when its token pointer is nil — a nil is never fabricated
// as a 0-cost contribution that hides missing data (Pitfall 4). If ALL four
// token fields are nil the result has no computable cost and perResultUSD returns
// (0, false); the caller MUST treat ok==false as "absent", never as $0.
func perResultUSD(m rowMetrics, row cost.CostRow) (float64, bool) {
	var usd float64
	var any bool

	if m.TokensInput != nil {
		usd += float64(*m.TokensInput) * row.InputPerMtok
		any = true
	}
	if m.TokensInputCachedRead != nil {
		usd += float64(*m.TokensInputCachedRead) * row.CachedInputPerMtok
		any = true
	}
	if m.TokensInputCacheWrite != nil {
		// TODO(D-14): add cache_write_per_mtok column; v1 prices cache-write at the input rate (conservative)
		usd += float64(*m.TokensInputCacheWrite) * row.InputPerMtok
		any = true
	}
	if m.TokensOutput != nil {
		usd += float64(*m.TokensOutput) * row.OutputPerMtok
		any = true
	}

	if !any {
		return 0, false
	}
	return usd / 1_000_000, true
}

// costPerSolvedTask reduces a per-solved-task USD map (one mean-USD per task with
// task_success==true — that per-task reduction over a cell's N runs is the
// aggregator's job in Plan 06) to the COST-02 headline:
//
//	cost_per_solved_task = Σ(USD over solved tasks) / count(solved tasks)
//
// With no solved tasks it returns (0, false) — null/omitted, NEVER a NaN from a
// divide-by-zero (D-12). The caller renders ok==false as "—" in the leaderboard.
func costPerSolvedTask(perTaskUSD map[string]float64) (float64, bool) {
	if len(perTaskUSD) == 0 {
		return 0, false
	}
	var sum float64
	for _, usd := range perTaskUSD {
		sum += usd
	}
	return sum / float64(len(perTaskUSD)), true
}
