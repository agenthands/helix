// Package token_meter is the Phase 79 trace-derived grader for the token
// metrics (METRIC-03). It sources tokens_input/tokens_output and the two
// cached-token columns EXCLUSIVELY from the provider `usage` block surfaced on
// trace.MergedTrace.Usage — never from a daemon/MCP-side counter (the
// tool-call tally or per-event byte sizes). The provider usage is the only
// trusted token source (D-02); the daemon counters are explicitly NOT a token
// source.
//
// trace.Usage is a plain value struct, so a genuine absence and a real 0 are
// indistinguishable from the struct alone. The coordinator (Plan 04) threads an
// out-of-band usagePresent signal (agent kind == "claude" AND a CC `result`
// event with a usage block was parsed) into this grader. When usagePresent is
// false (scripted corpus, no provider usage), MeterTokens returns all-nil
// pointers and a MetricError — an explicit JSON null, never a fabricated 0
// (D-01). When usagePresent is true, even zero cached tokens are returned as
// non-nil pointers-to-0 (present-and-zero), distinct from the scripted nil
// case (D-03).
package token_meter

import (
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/internal/eval/trace"
)

// graderName is stamped into every MetricError this package emits (D-07).
const graderName = "token_meter"

// MeterTokens returns the provider-sourced token pointers for a single run.
//
//   - usagePresent == true: tokens_input/tokens_output come from
//     mt.Usage.InputTokens/OutputTokens; the cached columns come from
//     mt.Usage.CacheReadTokens (tokens_input_cached_read) and
//     mt.Usage.CacheCreationTokens (tokens_input_cache_write). All four are
//     returned as non-nil pointers, so a present-and-zero usage value is a
//     pointer-to-0 (D-03), distinct from the scripted-absent case.
//   - usagePresent == false: there is no provider usage block (scripted run);
//     all four pointers are nil and a single token_meter MetricError is
//     recorded (D-01). The grader never fabricates a 0.
//
// The grader reads ONLY mt.Usage for token values — it never consults the
// merged trace's tool-call tally, its events, or any daemon-side byte counter
// (D-02).
func MeterTokens(mt trace.MergedTrace, usagePresent bool) (in, out, cachedRead, cacheWrite *int, errs []evaluators.MetricError) {
	if !usagePresent {
		// Scripted run: no provider usage block. Emit explicit null (never 0)
		// and annotate the offending metric (D-01/D-07).
		return nil, nil, nil, nil, []evaluators.MetricError{{
			Metric: "tokens_input",
			Grader: graderName,
			Reason: "no provider usage block (scripted run)",
		}}
	}

	// Provider usage present: every value is a present pointer, including 0.
	inputTokens := mt.Usage.InputTokens
	outputTokens := mt.Usage.OutputTokens
	cacheReadTokens := mt.Usage.CacheReadTokens
	cacheCreationTokens := mt.Usage.CacheCreationTokens

	return &inputTokens, &outputTokens, &cacheReadTokens, &cacheCreationTokens, nil
}
