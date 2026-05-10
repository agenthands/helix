package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/trace"
)

// boilerplate is the INFORMATIONAL notice embedded in every judge output per
// EVAL-07 and Pitfall 8 mitigation. Exact string is asserted by TestJudgeOutputBoilerplate.
const boilerplate = "INFORMATIONAL — DO NOT USE FOR CI GATING"

// modelMap maps short flag values to full Anthropic model identifiers.
var modelMap = map[string]string{
	"sonnet": "claude-sonnet-4-6",
	"opus":   "claude-opus-4-5",
}

// ResolveModel maps a short model flag ("sonnet", "opus") to the full model string.
// Returns an error for unknown values.
func ResolveModel(flag string) (string, error) {
	if m, ok := modelMap[flag]; ok {
		return m, nil
	}
	return "", fmt.Errorf("judge: unknown --judge-model value %q; valid values: sonnet, opus", flag)
}

// Input is the per-(task, mode) data fed into the judge.
type Input struct {
	TaskID          string
	Mode            string
	TaskKind        string // from expected_tools.yaml
	TaskDescription string // first line of task.md (TITLE ONLY — bias mitigation m4)
	Trace           trace.MergedTrace
	Result          report.EvalResult
}

// Scores holds the per-axis rubric scores.
type Scores struct {
	RightTool   int `json:"right_tool"`
	Evidence    int `json:"evidence"`
	BlastRadius int `json:"blast_radius"`
	Recovery    int `json:"recovery"`
}

// Entry is a single (task, mode) judge result.
type Entry struct {
	TaskID    string   `json:"task_id"`
	Mode      string   `json:"mode"`
	Scores    Scores   `json:"scores"`
	Reasoning string   `json:"reasoning"`
	Flags     []string `json:"flags"`
}

// Output is the top-level tool_behavior_judge.json schema.
// The Readme field always contains the INFORMATIONAL boilerplate.
// Run returns Output only (no error) so judge failures structurally
// cannot propagate into helix-eval's exit code (EVAL-07 hard constraint).
type Output struct {
	Readme       string    `json:"__readme"`
	JudgeModel   string    `json:"judge_model"`
	JudgedAt     time.Time `json:"judged_at"`
	Tasks        []Entry   `json:"tasks"`
	JudgeSkipped bool      `json:"judge_skipped,omitempty"`
	JudgeFailed  bool      `json:"judge_failed,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	ErrorSummary string    `json:"error_summary,omitempty"`
}

// RunOptions configures optional judge.Run behaviour.
type RunOptions struct {
	// NoJudge skips all API calls and produces a stub output with judge_skipped=true.
	NoJudge bool
}

// promptData is passed to the tool_behavior.tmpl template.
type promptData struct {
	TaskKind        string
	TaskDescription string
	Events          []trace.Event
	Outcome         string
	TestsPass       bool
	Rubric          string
}

// promptTemplate is parsed once at init from the embedded template.
var promptTemplate *template.Template

func init() {
	promptTemplate = template.Must(template.New("tool_behavior").Parse(embeddedTemplate))
}

// BuildPrompt renders the scoring prompt for the given input.
// Only trace events, task kind, and task title are included — patch diff and
// full task.md body are explicitly excluded (bias mitigation m4, T-67-05).
func BuildPrompt(inp Input) string {
	data := promptData{
		TaskKind:        inp.TaskKind,
		TaskDescription: inp.TaskDescription, // TITLE ONLY, never full body
		Events:          inp.Trace.Events,
		Outcome:         inp.Trace.Outcome,
		TestsPass:       inp.Result.TestsPass,
		Rubric:          embeddedRubric,
	}

	var buf bytes.Buffer
	if err := promptTemplate.Execute(&buf, data); err != nil {
		return fmt.Sprintf("prompt render error: %v", err)
	}
	return buf.String()
}

// scoreResponse is the expected JSON structure returned by the LLM judge.
type scoreResponse struct {
	Scores struct {
		RightTool   int `json:"right_tool"`
		Evidence    int `json:"evidence"`
		BlastRadius int `json:"blast_radius"`
		Recovery    int `json:"recovery"`
	} `json:"scores"`
	Reasoning string   `json:"reasoning"`
	Flags     []string `json:"flags"`
}

// Run executes the judge pass for all inputs and returns a structured Output.
//
// EVAL-07 CONSTRAINT (enforced by signature):
// Run NEVER returns an error. All failures (API errors, timeouts, skipped runs)
// are folded into the Output struct. This structurally prevents judge failures
// from propagating into helix-eval's exit code.
func Run(ctx context.Context, c *Client, inputs []Input, model string) Output {
	return RunWithOptions(ctx, c, inputs, model, RunOptions{})
}

// RunWithOptions is like Run but accepts additional options (e.g. NoJudge flag).
func RunWithOptions(ctx context.Context, c *Client, inputs []Input, model string, opts RunOptions) Output {
	base := Output{
		Readme:     boilerplate,
		JudgeModel: model,
		JudgedAt:   time.Now().UTC(),
	}

	// --no-judge flag: skip silently.
	if opts.NoJudge {
		base.JudgeSkipped = true
		base.Reason = `explicit --no-judge`
		return base
	}

	// Missing API key: skip silently.
	if c.apiKey == "" {
		base.JudgeSkipped = true
		base.Reason = "no api key"
		return base
	}

	var entries []Entry
	var lastErr error

	for _, inp := range inputs {
		// Each call uses a 60s per-item timeout so a slow API cannot stall the run.
		itemCtx, cancel := context.WithTimeout(ctx, 60*time.Second)

		prompt := BuildPrompt(inp)
		text, err := c.Score(itemCtx, "You are an expert code-intelligence evaluator.", prompt, 512)
		cancel()

		if err != nil {
			lastErr = err
			// Record a stub entry rather than propagating.
			entries = append(entries, Entry{
				TaskID:    inp.TaskID,
				Mode:      inp.Mode,
				Reasoning: fmt.Sprintf("judge API error: %v", err),
				Flags:     []string{"judge_api_error"},
			})
			continue
		}

		var resp scoreResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			// LLM returned non-JSON — record as stub.
			entries = append(entries, Entry{
				TaskID:    inp.TaskID,
				Mode:      inp.Mode,
				Reasoning: fmt.Sprintf("judge parse error: %v; raw: %.200s", err, text),
				Flags:     []string{"judge_parse_error"},
			})
			continue
		}

		entries = append(entries, Entry{
			TaskID: inp.TaskID,
			Mode:   inp.Mode,
			Scores: Scores{
				RightTool:   resp.Scores.RightTool,
				Evidence:    resp.Scores.Evidence,
				BlastRadius: resp.Scores.BlastRadius,
				Recovery:    resp.Scores.Recovery,
			},
			Reasoning: resp.Reasoning,
			Flags:     resp.Flags,
		})
	}

	base.Tasks = entries

	// If ALL inputs resulted in errors, mark judge_failed.
	if lastErr != nil && len(inputs) > 0 {
		allFailed := true
		for _, e := range entries {
			hasErrFlag := false
			for _, f := range e.Flags {
				if f == "judge_api_error" || f == "judge_parse_error" {
					hasErrFlag = true
					break
				}
			}
			if !hasErrFlag {
				allFailed = false
				break
			}
		}
		if allFailed {
			base.JudgeFailed = true
			base.ErrorSummary = lastErr.Error()
		}
	}

	return base
}

// WriteJudgeReport marshals o to JSON and writes it to path with mode 0600.
// The file ALWAYS exists after a run (even as a stub) so downstream
// report rendering can reason about its presence.
func WriteJudgeReport(path string, o Output) error {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("judge.WriteJudgeReport marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("judge.WriteJudgeReport write %q: %w", path, err)
	}
	return nil
}
