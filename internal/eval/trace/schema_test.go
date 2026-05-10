package trace_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/trace"
)

// TestEventJSONRoundTrip verifies every defined EventKind round-trips through
// JSON marshal/unmarshal byte-identically.
func TestEventJSONRoundTrip(t *testing.T) {
	t.Parallel()
	kinds := []trace.EventKind{
		trace.KindSessionInit,
		trace.KindAssistantMsg,
		trace.KindToolCall,
		trace.KindToolResult,
		trace.KindResult,
		trace.KindAPIRetry,
	}

	for _, kind := range kinds {
		kind := kind
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ev := trace.Event{
				T:      time.Date(2026, 5, 10, 8, 30, 0, 0, time.UTC),
				Source: "daemon",
				Kind:   kind,
			}
			b, err := json.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			var got trace.Event
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			b2, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("second marshal failed: %v", err)
			}
			if string(b) != string(b2) {
				t.Errorf("round-trip mismatch for kind %s:\n  first:  %s\n  second: %s", kind, b, b2)
			}
		})
	}
}

// TestMergedTraceShapeMatchesResearch asserts that a MergedTrace encodes to
// JSON with all required top-level keys from RESEARCH §"Merged trace.json Shape".
func TestMergedTraceShapeMatchesResearch(t *testing.T) {
	t.Parallel()
	mt := trace.MergedTrace{
		SchemaVersion: "1",
		TaskID:        "task-001",
		Mode:          "native",
		RunID:         "run-abc",
		ClaudeVersion: "claude-3-7-sonnet",
		HelixVersion:  "v1.10.0",
		StartedAt:     time.Now().UTC(),
		EndedAt:       time.Now().UTC().Add(5 * time.Second),
		DurationMs:    5000,
		Outcome:       "success",
		Events:        []trace.Event{},
		Usage:         trace.Usage{InputTokens: 100, OutputTokens: 50},
		ToolCallSummary: trace.ToolCallSummary{
			Total:     0,
			ByTool:    map[string]int{},
			ByOutcome: map[string]int{},
		},
		Guardrails: trace.GuardrailCounts{},
	}

	b, err := json.Marshal(mt)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	required := []string{
		"schema_version", "task_id", "mode", "run_id",
		"claude_version", "helix_version", "started_at", "ended_at",
		"duration_ms", "outcome", "events", "usage",
		"tool_call_summary", "guardrails",
	}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			t.Errorf("missing required top-level key: %q", key)
		}
	}
}

// TestOutcomeClosedEnum verifies IsValidOutcome accepts valid outcomes and
// rejects unknown ones. Closed enum from middleware.go.
func TestOutcomeClosedEnum(t *testing.T) {
	t.Parallel()
	valid := []string{
		"success", "invalid_args", "not_found", "circuit_open",
		"ls_crash", "timeout", "internal",
		"guardrail_warned", "guardrail_blocked",
	}
	for _, v := range valid {
		if !trace.IsValidOutcome(v) {
			t.Errorf("IsValidOutcome(%q) = false, want true", v)
		}
	}
	invalid := []string{"badword", "", "SUCCESS", "guardrail", "warn"}
	for _, v := range invalid {
		if trace.IsValidOutcome(v) {
			t.Errorf("IsValidOutcome(%q) = true, want false", v)
		}
	}
}

// TestEventKindClosedEnum verifies IsValidEventKind accepts valid event kinds
// and rejects unknown ones.
func TestEventKindClosedEnum(t *testing.T) {
	t.Parallel()
	valid := []string{
		"session_init", "assistant_message", "tool_call",
		"tool_result", "result", "api_retry",
	}
	for _, v := range valid {
		if !trace.IsValidEventKind(v) {
			t.Errorf("IsValidEventKind(%q) = false, want true", v)
		}
	}
	invalid := []string{"unknown", "", "TOOL_CALL", "init", "assistant"}
	for _, v := range invalid {
		if trace.IsValidEventKind(v) {
			t.Errorf("IsValidEventKind(%q) = true, want false", v)
		}
	}
}
