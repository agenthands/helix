package runner_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agenthands/helix/internal/eval/budget"
	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/score"
)

// repoRoot returns the root of the repository by walking up from this file
// until a go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// Walk up from the test file directory until we find go.mod.
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod from test file path")
		}
		dir = parent
	}
}

// fixturesDir returns the path to eval/fixtures relative to the repo root.
func fixturesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "eval", "fixtures")
}

// fakeCaller is a minimal MCPCaller implementation for unit tests.
type fakeCaller struct {
	responses map[string]json.RawMessage
}

func (f *fakeCaller) CallTool(_ context.Context, name string, _ map[string]any) (json.RawMessage, error) {
	if f.responses != nil {
		if r, ok := f.responses[name]; ok {
			return r, nil
		}
	}
	return json.RawMessage(`{"ok":true}`), nil
}

// TestLoadScript verifies that a valid scripted_agent.yaml parses correctly.
func TestLoadScript(t *testing.T) {
	dir := fixturesDir(t)
	path := filepath.Join(dir, "quick-rename-001", "scripted_agent.yaml")
	script, err := runner.LoadScript(path)
	if err != nil {
		t.Fatalf("LoadScript(%q): %v", path, err)
	}
	if len(script.Steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(script.Steps))
	}
	if script.Steps[0].Tool == "" {
		t.Fatal("first step Tool is empty")
	}
}

// TestScriptedAgentDispatch verifies that ScriptedAgent.Run executes each step
// in order using the provided MCPCaller and returns one StepResult per step.
func TestScriptedAgentDispatch(t *testing.T) {
	steps := runner.Script{
		Steps: []runner.ScriptedStep{
			{Tool: "find_references", Args: map[string]any{"symbol": "Foo"}},
			{Tool: "rename_symbol", Args: map[string]any{"old_name": "Foo", "new_name": "Bar"}},
		},
	}
	caller := &fakeCaller{
		responses: map[string]json.RawMessage{
			"find_references": json.RawMessage(`{"refs": []}`),
			"rename_symbol":   json.RawMessage(`{"ok": true}`),
		},
	}
	agent := runner.NewScriptedAgent(caller)
	b := budget.Budget{MaxToolCalls: 10, MaxSeconds: 10}
	wd, cancel := budget.NewWatchdog(context.Background(), b)
	defer cancel()

	results, breach, err := agent.Run(context.Background(), steps, wd)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if breach != nil {
		t.Fatalf("unexpected breach: %v", breach)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Tool != "find_references" {
		t.Errorf("step 0 tool = %q, want %q", results[0].Tool, "find_references")
	}
	if results[1].Tool != "rename_symbol" {
		t.Errorf("step 1 tool = %q, want %q", results[1].Tool, "rename_symbol")
	}
}

// TestScriptedAgentRefusesUnknownStepKey verifies that a YAML with 'tools'
// instead of 'tool' key fails LoadScript (KnownFields strict mode).
func TestScriptedAgentRefusesUnknownStepKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	badYAML := []byte("steps:\n  - tools: find_references\n    args: {}\n")
	if err := os.WriteFile(path, badYAML, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := runner.LoadScript(path)
	if err == nil {
		t.Fatal("expected error for unknown 'tools' key, got nil")
	}
}

// TestScriptedAgentRespectsBudgetMaxToolCalls verifies that the agent stops
// after MaxToolCalls steps and reports a BreachReason{Axis:"tool_calls"}.
func TestScriptedAgentRespectsBudgetMaxToolCalls(t *testing.T) {
	steps := runner.Script{
		Steps: []runner.ScriptedStep{
			{Tool: "tool_a"},
			{Tool: "tool_b"},
			{Tool: "tool_c"},
			{Tool: "tool_d"},
			{Tool: "tool_e"},
			{Tool: "tool_f"},
		},
	}
	caller := &fakeCaller{}
	agent := runner.NewScriptedAgent(caller)
	b := budget.Budget{MaxToolCalls: 3, MaxSeconds: 10}
	wd, cancel := budget.NewWatchdog(context.Background(), b)
	defer cancel()

	results, breach, err := agent.Run(context.Background(), steps, wd)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if breach == nil {
		t.Fatal("expected breach, got nil")
	}
	if breach.Axis != "tool_calls" {
		t.Errorf("breach.Axis = %q, want %q", breach.Axis, "tool_calls")
	}
	if len(results) > 3 {
		t.Errorf("expected <= 3 results (stopped at breach), got %d", len(results))
	}
}

// TestQuickRenameFixtureLoadable verifies that the quick-rename-001 fixture
// loads its rules and script without error.
func TestQuickRenameFixtureLoadable(t *testing.T) {
	fixDir := filepath.Join(fixturesDir(t), "quick-rename-001")

	// Validate rules file.
	rulesPath := filepath.Join(fixDir, "expected_tools.yaml")
	if _, err := score.LoadRules(rulesPath); err != nil {
		t.Fatalf("score.LoadRules(%q): %v", rulesPath, err)
	}

	// Validate script file.
	scriptPath := filepath.Join(fixDir, "scripted_agent.yaml")
	script, err := runner.LoadScript(scriptPath)
	if err != nil {
		t.Fatalf("runner.LoadScript(%q): %v", scriptPath, err)
	}
	if len(script.Steps) == 0 {
		t.Error("expected at least one step in quick-rename-001 scripted_agent.yaml")
	}
}

// TestQuickDeleteFixtureLoadable verifies that the quick-delete-001 fixture
// loads its rules and script without error.
func TestQuickDeleteFixtureLoadable(t *testing.T) {
	fixDir := filepath.Join(fixturesDir(t), "quick-delete-001")

	// Validate rules file.
	rulesPath := filepath.Join(fixDir, "expected_tools.yaml")
	if _, err := score.LoadRules(rulesPath); err != nil {
		t.Fatalf("score.LoadRules(%q): %v", rulesPath, err)
	}

	// Validate script file.
	scriptPath := filepath.Join(fixDir, "scripted_agent.yaml")
	script, err := runner.LoadScript(scriptPath)
	if err != nil {
		t.Fatalf("runner.LoadScript(%q): %v", scriptPath, err)
	}
	if len(script.Steps) == 0 {
		t.Error("expected at least one step in quick-delete-001 scripted_agent.yaml")
	}
}
