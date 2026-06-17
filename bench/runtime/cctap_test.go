package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSynthCCTapShape covers D-02: a synthesized CCTapResult from scripted
// StepResults has the right SessionInit -> (AssistantMsg + ToolResult)* ->
// Result shape, all Source:"cc", with IsError reflecting step.Err and zero
// Usage (scripted path; not fabricated).
func TestSynthCCTapShape(t *testing.T) {
	t0 := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(2 * time.Second)

	steps := []runner.StepResult{
		{Tool: "replace_in_file", AtTime: t0, Response: []byte(`{"ok":true}`)},
		{Tool: "read_file", AtTime: t1, Err: errors.New("boom")},
	}

	got := SynthCCTap(steps)

	// All events are Source:"cc".
	for i, ev := range got.Events {
		assert.Equal(t, "cc", ev.Source, "event %d must be Source:cc", i)
	}

	// Shape: SessionInit first, then per step (AssistantMsg + ToolResult),
	// then a final Result. For 2 steps: 1 + 2*2 + 1 = 6 events.
	require.Len(t, got.Events, 6, "expected SessionInit + 2*(AssistantMsg+ToolResult) + Result")

	assert.Equal(t, trace.KindSessionInit, got.Events[0].Kind, "first event must be session_init")

	// Step 1: AssistantMsg carrying ToolUse{Name: "replace_in_file"}, then ToolResult IsError=false.
	assert.Equal(t, trace.KindAssistantMsg, got.Events[1].Kind)
	require.Len(t, got.Events[1].ToolUses, 1, "assistant msg carries one tool use")
	assert.Equal(t, "replace_in_file", got.Events[1].ToolUses[0].Name)
	assert.Equal(t, trace.KindToolResult, got.Events[2].Kind)
	assert.False(t, got.Events[2].IsError, "step 1 has no error")

	// Step 2: AssistantMsg for read_file, then ToolResult IsError=true (step.Err != nil).
	assert.Equal(t, trace.KindAssistantMsg, got.Events[3].Kind)
	require.Len(t, got.Events[3].ToolUses, 1)
	assert.Equal(t, "read_file", got.Events[3].ToolUses[0].Name)
	assert.Equal(t, trace.KindToolResult, got.Events[4].Kind)
	assert.True(t, got.Events[4].IsError, "step 2 returned an error")

	// Final: Result.
	assert.Equal(t, trace.KindResult, got.Events[5].Kind, "last event must be result")

	// Usage is zero for the scripted path (Pitfall 6: not fabricated).
	assert.Equal(t, trace.Usage{}, got.Usage, "scripted Usage must be zero")
}

// TestSynthCCTapTimestamps asserts per-step AtTime drives event timestamps —
// two steps with distinct AtTime yield distinct event times (Pitfall 5: not a
// single batch time.Now()).
func TestSynthCCTapTimestamps(t *testing.T) {
	t0 := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Second)
	steps := []runner.StepResult{
		{Tool: "a", AtTime: t0},
		{Tool: "b", AtTime: t1},
	}

	got := SynthCCTap(steps)

	// The AssistantMsg for step 1 (index 1) and step 2 (index 3) must carry the
	// per-step timestamps, proving they are not all equal to one batch instant.
	assert.True(t, got.Events[1].T.Equal(t0), "step 1 assistant msg time = step 1 AtTime")
	assert.True(t, got.Events[3].T.Equal(t1), "step 2 assistant msg time = step 2 AtTime")
	assert.False(t, got.Events[1].T.Equal(got.Events[3].T), "distinct steps yield distinct times")
}

// TestCCLegPresentBothBranches is the WR-03 guard: it exercises ccLegPresent
// directly on a real trace.MergedTrace (not a re-implemented event count), so a
// regression that reverted the predicate to the vacuous "any Source==cc" form
// would fail here.
//
//   - empty steps  → SynthCCTap(nil) emits only SessionInit + Result (no
//     ToolResult, no ToolUses) → CCLegPresent == false.
//   - one scripted step → an AssistantMsg with a non-empty ToolUses plus a
//     ToolResult → CCLegPresent == true.
func TestCCLegPresentBothBranches(t *testing.T) {
	t0 := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)

	build := func(steps []runner.StepResult) trace.MergedTrace {
		mt, err := trace.Merge(trace.MergeInput{
			TaskID:    "internal-toolbench/IT-go-cclegpresent",
			Mode:      "your_agent_full",
			RunID:     "20260617T120000Z",
			StartedAt: t0,
			EndedAt:   t0.Add(time.Second),
			CC:        SynthCCTap(steps),
		})
		require.NoError(t, err)
		return mt
	}

	// False branch: empty/claude script — no cc-side tool event.
	emptyMT := build(nil)
	assert.False(t, ccLegPresent(emptyMT),
		"empty SynthCCTap(nil) carries no ToolResult/ToolUses → CCLegPresent must be false")

	// True branch: one scripted step carries a ToolUse + ToolResult.
	scriptedMT := build([]runner.StepResult{
		{Tool: "replace_in_file", AtTime: t0, Response: []byte(`{"ok":true}`)},
	})
	assert.True(t, ccLegPresent(scriptedMT),
		"one scripted step carries a cc ToolResult/ToolUses → CCLegPresent must be true")
}

// TestSynthCCTapMerge is the contract integration: feed the synthesized CC leg
// to trace.Merge alongside a fabricated daemon leg with >=1 KindToolCall; the
// merged trace must have ToolCallSummary.Total>=1 AND a CC leg present (>=1
// Source:"cc" event) — proving the 2-leg shape (D-02).
func TestSynthCCTapMerge(t *testing.T) {
	t0 := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	steps := []runner.StepResult{
		{Tool: "replace_in_file", AtTime: t0, Response: []byte(`{"ok":true}`)},
	}
	cc := SynthCCTap(steps)

	// Fabricated daemon leg: one daemon-side tool_call event (the thing Merge
	// counts into ToolCallSummary — merge.go:97-99).
	daemon := trace.DaemonTapResult{
		Events: []trace.Event{
			{
				T:       t0.Add(10 * time.Millisecond),
				Source:  "daemon",
				Kind:    trace.KindToolCall,
				Tool:    "replace_in_file",
				Outcome: "success",
				Pid:     4242,
			},
		},
	}

	mt, err := trace.Merge(trace.MergeInput{
		TaskID:         "internal-toolbench/IT-go-patch-apply-1",
		Mode:           "your_agent_full",
		RunID:          "20260617T120000Z",
		StartedAt:      t0,
		EndedAt:        t0.Add(2 * time.Second),
		Daemon:         daemon,
		CC:             cc,
		VerifyExitCode: 0,
	})
	require.NoError(t, err)

	// 2-leg assertion #1: tool calls counted from the daemon leg.
	assert.GreaterOrEqual(t, mt.ToolCallSummary.Total, 1, "ToolCallSummary.Total>=1 from daemon leg")

	// 2-leg assertion #2: the CC leg is present in the merged stream.
	ccCount := 0
	for _, ev := range mt.Events {
		if ev.Source == "cc" {
			ccCount++
		}
	}
	assert.GreaterOrEqual(t, ccCount, 1, "merged trace must carry >=1 Source:cc event (CC leg present)")

	// Synth must NOT double-count: the CC leg contributes no daemon tool_call,
	// so Total counts only the single fabricated daemon call.
	assert.Equal(t, 1, mt.ToolCallSummary.Total, "CC leg must not double-count tool calls")
}
