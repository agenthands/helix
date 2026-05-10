package judge_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/agenthands/helix/internal/eval/judge"
	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/trace"
)

// makeTestInput builds a judge.Input with the provided trace events.
func makeTestInput(taskID, mode, kind string, events []trace.Event) judge.Input {
	return judge.Input{
		TaskID:          taskID,
		Mode:            mode,
		TaskKind:        kind,
		TaskDescription: "Test task title",
		Trace: trace.MergedTrace{
			SchemaVersion: "1",
			TaskID:        taskID,
			Mode:          mode,
			RunID:         "run-test",
			StartedAt:     time.Now(),
			EndedAt:       time.Now(),
			Outcome:       "success",
			Events:        events,
		},
		Result: report.EvalResult{
			TaskID:    taskID,
			Mode:      mode,
			Success:   true,
			TestsPass: true,
			Outcome:   "success",
		},
	}
}

// makeEvent builds a tool_call trace event.
func makeEvent(tool, argsSummary, outcome string) trace.Event {
	return trace.Event{
		T:           time.Now(),
		Source:      "daemon",
		Kind:        trace.KindToolCall,
		Tool:        tool,
		ArgsSummary: argsSummary,
		Outcome:     outcome,
	}
}

// TestJudgeBuildsPromptFromTraceOnly verifies the rendered prompt does NOT
// contain raw patch diff bytes or full task.md content.
func TestJudgeBuildsPromptFromTraceOnly(t *testing.T) {
	const patchContent = "--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-old\n+new"
	const taskBodyFull = "# Title\n\nThis is the full task.md body with all the details about what to do..."

	input := makeTestInput("t1", "native", "rename", []trace.Event{
		makeEvent("rename_symbol", "old=foo new=bar", "success"),
	})

	// Build the prompt text using the exported helper.
	promptText := judge.BuildPrompt(input)

	if strings.Contains(promptText, patchContent) {
		t.Error("prompt contains raw patch diff bytes (bias mitigation m4 violated)")
	}
	if strings.Contains(promptText, taskBodyFull) {
		t.Error("prompt contains full task.md body (bias mitigation m4 violated)")
	}
	// Must contain task kind and title.
	if !strings.Contains(promptText, "rename") {
		t.Error("prompt missing task kind 'rename'")
	}
	if !strings.Contains(promptText, "Test task title") {
		t.Error("prompt missing task description title")
	}
	// Must contain at least one trace event tool name.
	if !strings.Contains(promptText, "rename_symbol") {
		t.Error("prompt missing trace event tool name")
	}
}

// TestJudgeAggregatesPerTaskMode verifies Run produces one entry per (task, mode) pair.
func TestJudgeAggregatesPerTaskMode(t *testing.T) {
	// Stub Anthropic server always returns valid score JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scoreJSON := `{"scores":{"right_tool":1,"evidence":1,"blast_radius":1,"recovery":1},"reasoning":"good","flags":[]}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse(scoreJSON))
	}))
	defer srv.Close()

	inputs := []judge.Input{
		makeTestInput("task-a", "baseline", "rename", nil),
		makeTestInput("task-a", "native", "rename", nil),
		makeTestInput("task-b", "baseline", "delete", nil),
	}

	c := judge.NewClient(judge.Options{APIKey: "k", BaseURL: srv.URL})
	out := judge.Run(context.Background(), c, inputs, "claude-sonnet-4-6")

	if len(out.Tasks) != 3 {
		t.Errorf("Tasks count = %d; want 3", len(out.Tasks))
	}
	for _, entry := range out.Tasks {
		if entry.TaskID == "" {
			t.Error("entry.TaskID empty")
		}
		if entry.Mode == "" {
			t.Error("entry.Mode empty")
		}
	}
}

// TestJudgeOutputBoilerplate verifies __readme is exactly the INFORMATIONAL boilerplate.
func TestJudgeOutputBoilerplate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scoreJSON := `{"scores":{"right_tool":0,"evidence":0,"blast_radius":0,"recovery":0},"reasoning":"ok","flags":[]}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse(scoreJSON))
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{APIKey: "k", BaseURL: srv.URL})
	out := judge.Run(context.Background(), c, nil, "claude-sonnet-4-6")

	const wantReadme = "INFORMATIONAL — DO NOT USE FOR CI GATING"
	if out.Readme != wantReadme {
		t.Errorf("Readme = %q; want %q", out.Readme, wantReadme)
	}
}

// TestJudgeAPIErrorDoesNotPropagate verifies API errors are folded into the Output.
func TestJudgeAPIErrorDoesNotPropagate(t *testing.T) {
	// Server always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	inputs := []judge.Input{
		makeTestInput("task-x", "native", "rename", nil),
	}
	c := judge.NewClient(judge.Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		InitialWait: 1 * time.Millisecond,
	})
	out := judge.Run(context.Background(), c, inputs, "claude-sonnet-4-6")

	// Run must return Output (not error) — signature enforces this.
	// Output must signal failure without propagating as Go error.
	if !out.JudgeFailed {
		t.Error("JudgeFailed should be true when API errors exhaust retries")
	}
	if out.ErrorSummary == "" {
		t.Error("ErrorSummary should be non-empty on API error")
	}
}

// TestJudgeMissingAPIKeySkipsCleanly verifies empty API key produces a stub with judge_skipped.
func TestJudgeMissingAPIKeySkipsCleanly(t *testing.T) {
	c := judge.NewClient(judge.Options{APIKey: ""})
	out := judge.Run(context.Background(), c, nil, "claude-sonnet-4-6")

	if !out.JudgeSkipped {
		t.Error("JudgeSkipped should be true when API key is missing")
	}
	if out.Reason == "" {
		t.Error("Reason should explain skip")
	}
}

// TestNoJudgeFlagSkips verifies that passing noJudge=true skips the API call.
func TestNoJudgeFlagSkips(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{APIKey: "k", BaseURL: srv.URL})
	out := judge.RunWithOptions(context.Background(), c, nil, "claude-sonnet-4-6", judge.RunOptions{NoJudge: true})

	if called {
		t.Error("API was called despite --no-judge flag")
	}
	if !out.JudgeSkipped {
		t.Error("JudgeSkipped should be true when --no-judge is set")
	}
	if out.Reason == "" {
		t.Error("Reason should explain skip")
	}
}

// TestJudgeModelFlagAcceptsSonnetOpus verifies model mapping.
func TestJudgeModelFlagAcceptsSonnetOpus(t *testing.T) {
	tests := []struct {
		flag      string
		wantModel string
		wantErr   bool
	}{
		{"sonnet", "claude-sonnet-4-6", false},
		{"opus", "claude-opus-4-5", false},
		{"unknown-model", "", true},
	}

	for _, tt := range tests {
		model, err := judge.ResolveModel(tt.flag)
		if tt.wantErr {
			if err == nil {
				t.Errorf("flag=%q: expected error, got nil", tt.flag)
			}
		} else {
			if err != nil {
				t.Errorf("flag=%q: unexpected error: %v", tt.flag, err)
			}
			if model != tt.wantModel {
				t.Errorf("flag=%q: model=%q; want %q", tt.flag, model, tt.wantModel)
			}
		}
	}
}

// TestWriteJudgeReport verifies the file is written and is valid JSON.
func TestWriteJudgeReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool_behavior_judge.json")

	out := judge.Output{
		Readme:     "INFORMATIONAL — DO NOT USE FOR CI GATING",
		JudgeModel: "claude-sonnet-4-6",
		JudgedAt:   time.Now(),
		Tasks:      nil,
	}

	if err := judge.WriteJudgeReport(path, out); err != nil {
		t.Fatalf("WriteJudgeReport error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["__readme"] != "INFORMATIONAL — DO NOT USE FOR CI GATING" {
		t.Errorf("__readme = %v; want boilerplate", parsed["__readme"])
	}
}
