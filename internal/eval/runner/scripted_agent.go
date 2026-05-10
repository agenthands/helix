// Package runner scripted_agent.go implements the in-process scripted-agent
// path for eval-quick (Plan 67-06a). A scripted agent executes a hard-coded
// sequence of MCP tool calls (loaded from scripted_agent.yaml) without any
// real LLM in the loop.
//
// WARNING: The scripted agent does NOT measure real Claude Code agent behavior.
// It is harness-validation tooling only. See eval/EVAL.md (Pitfall 6).
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/agenthands/helix/internal/eval/budget"
	"gopkg.in/yaml.v3"
)

// ScriptedStep is one tool-call entry in a scripted_agent.yaml file.
type ScriptedStep struct {
	// Tool is the MCP tool name to invoke.
	Tool string `yaml:"tool"`
	// Args is the argument map passed verbatim to the tool.
	Args map[string]any `yaml:"args"`
	// ExpectError marks a step that is expected to return an error.
	// The step is still executed; ExpectError only affects result recording.
	ExpectError bool `yaml:"expect_error"`
}

// Script is the parsed content of a scripted_agent.yaml file.
type Script struct {
	Steps []ScriptedStep `yaml:"steps"`
}

// LoadScript reads and strictly-decodes a scripted_agent.yaml file.
// Unknown YAML keys cause an error (KnownFields strict mode).
func LoadScript(path string) (Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Script{}, fmt.Errorf("scripted_agent: read %q: %w", path, err)
	}

	var s Script
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Script{}, fmt.Errorf("scripted_agent: parse %q: %w", path, err)
	}
	return s, nil
}

// StepResult records the outcome of executing one scripted step.
type StepResult struct {
	// Tool is the name of the tool that was called.
	Tool string
	// AtTime is the wall-clock instant when the call was dispatched.
	AtTime time.Time
	// Response is the raw JSON response from the tool.
	Response json.RawMessage
	// Err is non-nil when the tool call returned an error.
	Err error
}

// MCPCaller is the interface through which ScriptedAgent dispatches tool calls.
// In production, the implementation wraps the in-process MCP server.
// In unit tests, it is replaced by a fake.
type MCPCaller interface {
	CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error)
}

// ScriptedAgent replays a Script against an MCPCaller.
type ScriptedAgent struct {
	caller MCPCaller
}

// NewScriptedAgent creates a ScriptedAgent that dispatches calls via caller.
func NewScriptedAgent(caller MCPCaller) *ScriptedAgent {
	return &ScriptedAgent{caller: caller}
}

// Run executes each step of s in order. For each step it calls
// wd.RecordToolCall() before dispatching. If the watchdog reports a breach
// after recording, Run returns immediately with the BreachReason.
//
// Run returns (results, breach, error). Infrastructure failures (not tool-call
// errors) are reported via the error return. Breach is non-nil when the budget
// is exceeded. Tool-call errors are recorded per-step in StepResult.Err and do
// not abort the run unless ExpectError=false and a fatal budget breach occurs.
func (a *ScriptedAgent) Run(ctx context.Context, s Script, wd *budget.Watchdog) ([]StepResult, *budget.BreachReason, error) {
	var results []StepResult

	for _, step := range s.Steps {
		// Enforce budget before the call.
		wd.RecordToolCall()
		if br := wd.Breach(); br != nil {
			// Budget exceeded — return what we have so far.
			return results, br, nil
		}

		// Check context cancellation (watchdog may have cancelled it).
		select {
		case <-ctx.Done():
			if br := wd.Breach(); br != nil {
				return results, br, nil
			}
			return results, nil, ctx.Err()
		default:
		}

		sr := StepResult{
			Tool:   step.Tool,
			AtTime: time.Now(),
		}

		resp, err := a.caller.CallTool(ctx, step.Tool, step.Args)
		sr.Response = resp
		sr.Err = err

		results = append(results, sr)
	}

	return results, nil, nil
}
