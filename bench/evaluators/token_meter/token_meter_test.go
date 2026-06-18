package token_meter

import (
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadFixtureUsage parses the captured CC stream-json fixture under testdata/
// and returns a MergedTrace whose Usage was populated from the provider
// `result`-event usage block (tap.go:277-294). It deliberately does NOT call
// trace.Merge — token_meter must source tokens from mt.Usage alone.
func loadFixtureUsage(t *testing.T) trace.MergedTrace {
	t.Helper()
	res, err := trace.TapCCStream(filepath.Join("testdata", "cc_stream_with_usage.jsonl"))
	require.NoError(t, err)
	return trace.MergedTrace{Usage: res.Usage}
}

// TestProviderUsageSourceOfTruth covers METRIC-03/D-02: tokens come from the
// provider `usage` block (mt.Usage), and a daemon-side counter present in the
// trace is IGNORED as a token source.
func TestProviderUsageSourceOfTruth(t *testing.T) {
	mt := loadFixtureUsage(t)

	in, out, cachedRead, cacheWrite, errs := MeterTokens(mt, true)

	require.Empty(t, errs, "usage present → no MetricError")
	require.NotNil(t, in)
	require.NotNil(t, out)
	require.NotNil(t, cachedRead)
	require.NotNil(t, cacheWrite)

	// Provider usage values from the fixture (input/output/cache_read/cache_creation).
	assert.Equal(t, 1234, *in, "tokens_input from provider usage block")
	assert.Equal(t, 567, *out, "tokens_output from provider usage block")
	assert.Equal(t, 89, *cachedRead, "tokens_input_cached_read from cache_read_input_tokens")
	assert.Equal(t, 42, *cacheWrite, "tokens_input_cache_write from cache_creation_input_tokens")

	// Negative — NOT from MCP counter (METRIC-03): a trace carrying a large
	// daemon-side byte/tool-call total but a DISTINCT provider usage must yield
	// the provider value, never the daemon count.
	mtWithDaemonCounters := trace.MergedTrace{
		Usage: trace.Usage{InputTokens: 7, OutputTokens: 9},
		ToolCallSummary: trace.ToolCallSummary{
			Total:  9999,
			ByTool: map[string]int{"read_file": 9999},
		},
		Events: []trace.Event{
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "read_file", ResultSizeBytes: 1_000_000},
		},
	}
	in2, out2, _, _, errs2 := MeterTokens(mtWithDaemonCounters, true)
	require.Empty(t, errs2)
	require.NotNil(t, in2)
	require.NotNil(t, out2)
	assert.Equal(t, 7, *in2, "tokens_input must equal provider Usage.InputTokens, not the daemon counter")
	assert.Equal(t, 9, *out2, "tokens_output must equal provider Usage.OutputTokens, not the daemon counter")
	assert.NotEqual(t, 9999, *in2, "daemon ToolCallSummary.Total must not leak into tokens")
	assert.NotEqual(t, 1_000_000, *in2, "daemon ResultSizeBytes must not leak into tokens")
}

// TestScriptedNullTokens covers D-01: a scripted run (usagePresent=false) yields
// explicit nil token pointers (never *int(0)) plus a token_meter MetricError;
// and D-03: with usagePresent=true but zero cached tokens, cached columns are
// non-nil pointers to 0 (present), distinct from the scripted nil case.
func TestScriptedNullTokens(t *testing.T) {
	// Scripted (no provider usage block) → all-nil + MetricError.
	scripted := trace.MergedTrace{} // zero Usage, but usagePresent=false drives the nulls
	in, out, cachedRead, cacheWrite, errs := MeterTokens(scripted, false)

	assert.Nil(t, in, "scripted tokens_input must be nil, not *int(0)")
	assert.Nil(t, out, "scripted tokens_output must be nil, not *int(0)")
	assert.Nil(t, cachedRead, "scripted cached_read must be nil")
	assert.Nil(t, cacheWrite, "scripted cache_write must be nil")
	require.NotEmpty(t, errs, "scripted run must record a MetricError")
	assert.Equal(t, "token_meter", errs[0].Grader)
	assert.Equal(t, "no provider usage block (scripted run)", errs[0].Reason)

	// D-03: usage present with zero cached tokens → cached columns are present
	// pointers-to-0, NOT nil. This distinguishes "present and zero" from "absent".
	zeroCached := trace.MergedTrace{
		Usage: trace.Usage{InputTokens: 10, OutputTokens: 20, CacheReadTokens: 0, CacheCreationTokens: 0},
	}
	in2, out2, cr2, cw2, errs2 := MeterTokens(zeroCached, true)
	require.Empty(t, errs2)
	require.NotNil(t, in2)
	require.NotNil(t, out2)
	require.NotNil(t, cr2, "cached_read present-and-zero must be a non-nil pointer to 0")
	require.NotNil(t, cw2, "cache_write present-and-zero must be a non-nil pointer to 0")
	assert.Equal(t, 0, *cr2)
	assert.Equal(t, 0, *cw2)
	assert.Equal(t, 10, *in2)
	assert.Equal(t, 20, *out2)
}
