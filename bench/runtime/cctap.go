package runtime

import (
	"time"

	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/trace"
)

// synthCCSessionID is the stable synthetic session id stamped on the synthesized
// CC leg. Real claude runs carry a UUID from the stream; the scripted path has
// no model session, so a fixed marker keeps the leg self-describing without
// fabricating a model-issued identifier.
const synthCCSessionID = "synth-scripted"

// SynthCCTap synthesizes a trace.CCTapResult from the scripted agent's executed
// []runner.StepResult (D-02). The scripted CI gate has no real claude transcript
// to tap, so this builds the agent-side ("2nd leg") evidence stream directly
// from the recorded tool-call sequence, mirroring the event shape TapCCStream
// produces for a real claude run so the leg looks identical:
//
//	SessionInit -> per step (AssistantMsg carrying ToolUse + ToolResult) -> Result
//
// All events are Source:"cc". Event timestamps come from each step's AtTime
// (NOT a single batch time.Now()) so the merge ordering is approximately
// faithful (Pitfall 5). Usage is the zero value — the scripted path drives no
// model, so tokens are legitimately 0 and are never fabricated (Pitfall 6).
//
// The synth deliberately emits NO Source:"daemon" KindToolCall events: those
// come from the real daemon-tap, and trace.Merge counts ToolCallSummary ONLY
// from daemon tool_call events (merge.go:97-99). The CC leg therefore adds
// agent-side continuity to the merged trace without double-counting tool calls.
func SynthCCTap(steps []runner.StepResult) trace.CCTapResult {
	// 1 SessionInit + 2 events per step (AssistantMsg + ToolResult) + 1 Result.
	events := make([]trace.Event, 0, 2+2*len(steps))

	// Anchor the session-init / final-result timestamps to the run's span.
	startT := initTime(steps)
	endT := finalTime(steps)

	// SessionInit.
	events = append(events, trace.Event{
		T:         startT,
		Source:    "cc",
		Kind:      trace.KindSessionInit,
		SessionID: synthCCSessionID,
	})

	// Per step: an AssistantMsg carrying the tool use, then its ToolResult.
	for _, step := range steps {
		toolUseID := step.Tool
		events = append(events, trace.Event{
			T:      step.AtTime,
			Source: "cc",
			Kind:   trace.KindAssistantMsg,
			ToolUses: []trace.ToolUse{
				{
					ID:    toolUseID,
					Name:  step.Tool,
					Input: step.Response, // scripted: the recorded response stands in for the call payload
				},
			},
		})
		events = append(events, trace.Event{
			T:         step.AtTime,
			Source:    "cc",
			Kind:      trace.KindToolResult,
			ToolUseID: toolUseID,
			IsError:   step.Err != nil,
		})
	}

	// Result: final event, zero Usage (scripted has no model — Pitfall 6).
	events = append(events, trace.Event{
		T:         endT,
		Source:    "cc",
		Kind:      trace.KindResult,
		SessionID: synthCCSessionID,
	})

	return trace.CCTapResult{
		Events:         events,
		Usage:          trace.Usage{}, // zero — not fabricated
		FinalSessionID: synthCCSessionID,
	}
}

// initTime returns the timestamp for the session-init event: the first step's
// AtTime when present, else the zero time. The result event reuses the last
// step's AtTime via finalTime.
func initTime(steps []runner.StepResult) (t time.Time) {
	if len(steps) > 0 {
		return steps[0].AtTime
	}
	return t
}

// finalTime returns the timestamp for the result event: the last step's AtTime
// when present, else the zero time.
func finalTime(steps []runner.StepResult) (t time.Time) {
	if len(steps) > 0 {
		return steps[len(steps)-1].AtTime
	}
	return t
}
