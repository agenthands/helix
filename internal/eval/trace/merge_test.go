package trace_test

import (
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/budget"
	"github.com/agenthands/helix/internal/eval/trace"
)

var (
	t1 = time.Date(2026, 5, 10, 8, 30, 0, 0, time.UTC)
	t2 = time.Date(2026, 5, 10, 8, 30, 1, 0, time.UTC)
	t3 = time.Date(2026, 5, 10, 8, 30, 2, 0, time.UTC)
)

func baseInput() trace.MergeInput {
	return trace.MergeInput{
		TaskID:        "task-001",
		Mode:          "native",
		RunID:         "run-abc",
		ClaudeVersion: "claude-3-7-sonnet",
		HelixVersion:  "v1.10.0",
		StartedAt:     t1,
		EndedAt:       t3,
		RepoRoot:      "/repo",
	}
}

// TestMergeSortsByWallClock verifies events come out sorted by t ASC.
func TestMergeSortsByWallClock(t *testing.T) {
	t.Parallel()
	in := baseInput()
	// Daemon events out of order relative to CC
	in.Daemon = trace.DaemonTapResult{
		Events: []trace.Event{
			{T: t3, Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references", Outcome: "success"},
			{T: t1, Source: "daemon", Kind: trace.KindToolCall, Tool: "rename_symbol", Outcome: "success"},
		},
	}
	in.CC = trace.CCTapResult{
		Events: []trace.Event{
			{T: t2, Source: "cc", Kind: trace.KindAssistantMsg},
		},
	}

	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if len(mt.Events) != 3 {
		t.Fatalf("want 3 events, got %d", len(mt.Events))
	}
	for i := 1; i < len(mt.Events); i++ {
		if mt.Events[i].T.Before(mt.Events[i-1].T) {
			t.Errorf("events[%d].T (%v) < events[%d].T (%v): not sorted",
				i, mt.Events[i].T, i-1, mt.Events[i-1].T)
		}
	}
}

// TestMergeDaemonWinsOnTie verifies daemon-source events come before CC events
// when timestamps are identical.
func TestMergeDaemonWinsOnTie(t *testing.T) {
	t.Parallel()
	tSame := t1
	in := baseInput()
	in.Daemon = trace.DaemonTapResult{
		Events: []trace.Event{
			{T: tSame, Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references", Outcome: "success"},
		},
	}
	in.CC = trace.CCTapResult{
		Events: []trace.Event{
			{T: tSame, Source: "cc", Kind: trace.KindAssistantMsg},
		},
	}

	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if len(mt.Events) != 2 {
		t.Fatalf("want 2 events, got %d", len(mt.Events))
	}
	if mt.Events[0].Source != "daemon" {
		t.Errorf("first event Source = %q, want %q (daemon wins on tie)", mt.Events[0].Source, "daemon")
	}
	if mt.Events[1].Source != "cc" {
		t.Errorf("second event Source = %q, want %q", mt.Events[1].Source, "cc")
	}
}

// TestMergeBuildsToolCallSummary verifies ToolCallSummary aggregation.
func TestMergeBuildsToolCallSummary(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.Daemon = trace.DaemonTapResult{
		Events: []trace.Event{
			{T: t1, Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references", Outcome: "success"},
			{T: t2, Source: "daemon", Kind: trace.KindToolCall, Tool: "find_references", Outcome: "success"},
			{T: t3, Source: "daemon", Kind: trace.KindToolCall, Tool: "rename_symbol", Outcome: "guardrail_warned"},
		},
	}

	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if mt.ToolCallSummary.Total != 3 {
		t.Errorf("ToolCallSummary.Total = %d, want 3", mt.ToolCallSummary.Total)
	}
	if mt.ToolCallSummary.ByTool["find_references"] != 2 {
		t.Errorf("ByTool[find_references] = %d, want 2", mt.ToolCallSummary.ByTool["find_references"])
	}
	if mt.ToolCallSummary.ByTool["rename_symbol"] != 1 {
		t.Errorf("ByTool[rename_symbol] = %d, want 1", mt.ToolCallSummary.ByTool["rename_symbol"])
	}
	if mt.ToolCallSummary.ByOutcome["success"] != 2 {
		t.Errorf("ByOutcome[success] = %d, want 2", mt.ToolCallSummary.ByOutcome["success"])
	}
	if mt.ToolCallSummary.ByOutcome["guardrail_warned"] != 1 {
		t.Errorf("ByOutcome[guardrail_warned] = %d, want 1", mt.ToolCallSummary.ByOutcome["guardrail_warned"])
	}
}

// TestMergeBuildsGuardrailCounts verifies guardrail count aggregation.
func TestMergeBuildsGuardrailCounts(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.Daemon = trace.DaemonTapResult{
		Events: []trace.Event{
			{T: t1, Source: "daemon", Kind: trace.KindToolCall, Tool: "delete_file", Outcome: "guardrail_warned"},
			{T: t2, Source: "daemon", Kind: trace.KindToolCall, Tool: "write_file", Outcome: "guardrail_blocked"},
		},
	}

	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if mt.Guardrails.Warned != 1 {
		t.Errorf("Guardrails.Warned = %d, want 1", mt.Guardrails.Warned)
	}
	if mt.Guardrails.Blocked != 1 {
		t.Errorf("Guardrails.Blocked = %d, want 1", mt.Guardrails.Blocked)
	}
}

// TestMergeUsageFromCC verifies CC Usage is copied verbatim into MergedTrace.
func TestMergeUsageFromCC(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.CC = trace.CCTapResult{
		Usage: trace.Usage{
			InputTokens:  1234,
			OutputTokens: 567,
		},
		UsagePresent: true,
	}

	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if mt.Usage.InputTokens != 1234 {
		t.Errorf("Usage.InputTokens = %d, want 1234", mt.Usage.InputTokens)
	}
	if mt.Usage.OutputTokens != 567 {
		t.Errorf("Usage.OutputTokens = %d, want 567", mt.Usage.OutputTokens)
	}
	if !mt.UsagePresent {
		t.Error("UsagePresent = false, want true (threaded from CCTapResult)")
	}
}

// TestMergeUsageAbsentSignal verifies the UsagePresent presence flag is threaded
// independently of Usage values: an all-zero CC usage that WAS present stays
// present, and a CC result with no usage block stays absent (MD-01).
func TestMergeUsageAbsentSignal(t *testing.T) {
	t.Parallel()

	// Present-and-zero: a real provider usage block of all zeros.
	in := baseInput()
	in.CC = trace.CCTapResult{Usage: trace.Usage{}, UsagePresent: true}
	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if !mt.UsagePresent {
		t.Error("present-and-zero: UsagePresent = false, want true")
	}

	// Absent: no usage block (scripted leg).
	in = baseInput()
	in.CC = trace.CCTapResult{Usage: trace.Usage{}, UsagePresent: false}
	mt, err = trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if mt.UsagePresent {
		t.Error("absent: UsagePresent = true, want false")
	}
}

// TestMergeOutcomeFromBudgetBreach verifies that a budget breach sets
// Outcome to "failed-with-cause: budget_<axis>" per D-08.
func TestMergeOutcomeFromBudgetBreach(t *testing.T) {
	t.Parallel()
	breach := &budget.BreachReason{
		Axis:     "tool_calls",
		Limit:    100,
		Observed: 101,
	}
	in := baseInput()
	in.Budget = breach
	in.VerifyExitCode = 0 // budget breach should win over exit code

	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	want := "failed-with-cause: budget_tool_calls"
	if mt.Outcome != want {
		t.Errorf("Outcome = %q, want %q", mt.Outcome, want)
	}
}

// TestMergeOutcomeFromVerifyExitCode verifies exit code 0 = success, non-zero = failed.
func TestMergeOutcomeFromVerifyExitCode(t *testing.T) {
	t.Parallel()
	t.Run("exit_0_success", func(t *testing.T) {
		t.Parallel()
		in := baseInput()
		in.VerifyExitCode = 0
		mt, err := trace.Merge(in)
		if err != nil {
			t.Fatalf("Merge error: %v", err)
		}
		if mt.Outcome != "success" {
			t.Errorf("Outcome = %q, want %q", mt.Outcome, "success")
		}
	})
	t.Run("exit_nonzero_failed", func(t *testing.T) {
		t.Parallel()
		in := baseInput()
		in.VerifyExitCode = 1
		mt, err := trace.Merge(in)
		if err != nil {
			t.Fatalf("Merge error: %v", err)
		}
		if mt.Outcome != "failed" {
			t.Errorf("Outcome = %q, want %q", mt.Outcome, "failed")
		}
	})
}

// TestMergePathPrefixInvariant verifies that paths outside RepoRoot cause Merge
// to return an error and set Outcome="failed" with failure_reason="patch_outside_repo".
// This is the T-67-02 mitigation.
func TestMergePathPrefixInvariant(t *testing.T) {
	t.Parallel()
	t.Run("valid_paths_ok", func(t *testing.T) {
		t.Parallel()
		in := baseInput()
		in.RepoRoot = "/repo"
		in.PatchPaths = []string{"/repo/main.go", "/repo/internal/foo.go"}
		_, err := trace.Merge(in)
		if err != nil {
			t.Errorf("Merge should not error on valid paths, got: %v", err)
		}
	})
	t.Run("path_outside_repo_errors", func(t *testing.T) {
		t.Parallel()
		in := baseInput()
		in.RepoRoot = "/repo"
		in.PatchPaths = []string{"/repo/main.go", "../../../etc/passwd"}
		mt, err := trace.Merge(in)
		if err == nil {
			t.Error("Merge should error on path outside repo, got nil")
		}
		if mt.Outcome != "failed" {
			t.Errorf("Outcome = %q, want %q", mt.Outcome, "failed")
		}
		if !strings.Contains(mt.FailureReason, "patch_outside_repo") {
			t.Errorf("FailureReason = %q, want it to contain %q", mt.FailureReason, "patch_outside_repo")
		}
	})
}

// TestMergeSchemaVersion verifies SchemaVersion is always "1".
func TestMergeSchemaVersion(t *testing.T) {
	t.Parallel()
	mt, err := trace.Merge(baseInput())
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if mt.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", mt.SchemaVersion, "1")
	}
}

// TestMergeDurationMs verifies DurationMs is computed from StartedAt / EndedAt.
func TestMergeDurationMs(t *testing.T) {
	t.Parallel()
	in := baseInput()
	// t1 to t3 is 2 seconds = 2000ms
	mt, err := trace.Merge(in)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if mt.DurationMs != 2000 {
		t.Errorf("DurationMs = %d, want 2000", mt.DurationMs)
	}
}
