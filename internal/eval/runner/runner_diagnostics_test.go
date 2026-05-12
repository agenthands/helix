package runner

import (
	"testing"

	"github.com/agenthands/helix/internal/eval/trace"
)

// TestDiagnosticsCleanFromTrace covers F-09: the DiagnosticsClean signal now
// derives from daemon-tap evidence, falling back to TestsPass only when the
// tap is empty (no daemon evidence captured).
func TestDiagnosticsCleanFromTrace(t *testing.T) {
	cases := []struct {
		name      string
		merged    trace.MergedTrace
		testsPass bool
		want      bool
	}{
		{
			name:      "empty tap falls back to TestsPass=true",
			merged:    trace.MergedTrace{ToolCallSummary: trace.ToolCallSummary{Total: 0}},
			testsPass: true,
			want:      true,
		},
		{
			name:      "empty tap falls back to TestsPass=false",
			merged:    trace.MergedTrace{ToolCallSummary: trace.ToolCallSummary{Total: 0}},
			testsPass: false,
			want:      false,
		},
		{
			name: "tap with ls_crash outcome → not clean",
			merged: trace.MergedTrace{ToolCallSummary: trace.ToolCallSummary{
				Total:     2,
				ByOutcome: map[string]int{"success": 1, "ls_crash": 1},
			}},
			testsPass: true,
			want:      false,
		},
		{
			name: "tap with internal-error outcome → not clean",
			merged: trace.MergedTrace{ToolCallSummary: trace.ToolCallSummary{
				Total:     1,
				ByOutcome: map[string]int{"internal": 1},
			}},
			testsPass: true,
			want:      false,
		},
		{
			name: "tap with only success outcomes → clean regardless of TestsPass",
			merged: trace.MergedTrace{ToolCallSummary: trace.ToolCallSummary{
				Total:     3,
				ByOutcome: map[string]int{"success": 3},
			}},
			testsPass: false,
			want:      true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := diagnosticsCleanFromTrace(tc.merged, tc.testsPass)
			if got != tc.want {
				t.Errorf("diagnosticsCleanFromTrace(...) = %v, want %v", got, tc.want)
			}
		})
	}
}
