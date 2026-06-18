package tool_trace_analyzer

import (
	"testing"

	"github.com/agenthands/helix/internal/eval/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraceMergeContinuity covers METRIC-06: tool_calls is derived from the
// already-merged trace's ToolCallSummary.Total exactly — the analyzer consumes
// the passed MergedTrace and never re-merges (no second TapDaemonLog / PID
// gate). A re-merge would risk a tool_calls value that disagrees with
// trace.json; this test pins parity with the merged object.
func TestTraceMergeContinuity(t *testing.T) {
	mt := trace.MergedTrace{
		ToolCallSummary: trace.ToolCallSummary{
			Total: 7,
			ByTool: map[string]int{
				"go_to_definition": 3,
				"read_file":        4,
			},
		},
	}

	tm, errs := Analyze(mt)

	require.Empty(t, errs)
	require.NotNil(t, tm.ToolCalls)
	assert.Equal(t, mt.ToolCallSummary.Total, *tm.ToolCalls,
		"tool_calls must equal MergedTrace.ToolCallSummary.Total exactly (no re-merge divergence)")
	assert.Equal(t, 7, *tm.ToolCalls)
}

// TestTraceDerivedMetrics covers the remaining trace derivations
// (METRIC-01/02): wall_time_seconds, semantic_tool_calls, files_read,
// bytes_read, lsp_diagnostics_used, retry_count — all from MergedTrace fields.
func TestTraceDerivedMetrics(t *testing.T) {
	mt := trace.MergedTrace{
		DurationMs: 2500,
		ToolCallSummary: trace.ToolCallSummary{
			Total: 5,
			ByTool: map[string]int{
				// 3 semantic counts (registry-anchored symbol tools):
				//   go_to_definition(1) + find_references(2) = 3.
				// 2 non-semantic counts: write_file(1) + read_file(1).
				"go_to_definition": 1,
				"find_references":  2,
				"write_file":       1,
				"read_file":        1,
			},
		},
		Events: []trace.Event{
			// files_read / bytes_read: two read/list events carrying ResultSizeBytes.
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "read_file", ResultSizeBytes: 100},
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "list_dir", ResultSizeBytes: 250},
			// lsp_diagnostics_used: one get_diagnostics event.
			{Source: "daemon", Kind: trace.KindToolCall, Tool: "get_diagnostics"},
			// retry_count: two KindAPIRetry events.
			{Source: "cc", Kind: trace.KindAPIRetry},
			{Source: "cc", Kind: trace.KindAPIRetry},
		},
	}

	tm, errs := Analyze(mt)
	require.Empty(t, errs)

	// wall_time_seconds = DurationMs / 1000.
	require.NotNil(t, tm.WallTimeSeconds)
	assert.InDelta(t, 2.5, *tm.WallTimeSeconds, 1e-9, "wall_time_seconds = DurationMs/1000")

	// semantic_tool_calls: go_to_definition(1) + find_references(2) are in the
	// registry-anchored semantic set; write_file/read_file are not.
	// Total semantic = 3.
	require.NotNil(t, tm.SemanticToolCalls)
	assert.Equal(t, 3, *tm.SemanticToolCalls,
		"semantic_tool_calls counts only registry-anchored semantic tool names")

	// files_read / bytes_read from read/list events.
	require.NotNil(t, tm.FilesRead)
	require.NotNil(t, tm.BytesRead)
	assert.Equal(t, 2, *tm.FilesRead, "two read/list events")
	assert.Equal(t, 350, *tm.BytesRead, "sum of ResultSizeBytes (100+250)")

	// lsp_diagnostics_used: at least the one get_diagnostics event.
	require.NotNil(t, tm.LSPDiagnosticsUsed)
	assert.GreaterOrEqual(t, *tm.LSPDiagnosticsUsed, 1, "one get_diagnostics event counted")

	// retry_count: two KindAPIRetry events.
	require.NotNil(t, tm.RetryCount)
	assert.Equal(t, 2, *tm.RetryCount, "two KindAPIRetry events")
}
