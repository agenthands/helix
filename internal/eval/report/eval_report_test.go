package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/score"
	"github.com/agenthands/helix/internal/eval/trace"
)

// fixture builds a small matrix of EvalResults for testing.
func fixtureResults() []report.EvalResult {
	return []report.EvalResult{
		{
			TaskID:           "task-rename",
			Mode:             "baseline",
			Success:          false,
			TestsPass:        false,
			Outcome:          "failed",
			FailureReason:    "verify exited 1",
			GuardrailCompliance: trace.GuardrailCounts{Warned: 1},
			Tokens: struct {
				Input  int `json:"input"`
				Output int `json:"output"`
			}{Input: 100, Output: 50},
		},
		{
			TaskID:           "task-rename",
			Mode:             "native",
			Success:          true,
			TestsPass:        true,
			Outcome:          "success",
			GuardrailCompliance: trace.GuardrailCounts{},
			Tokens: struct {
				Input  int `json:"input"`
				Output int `json:"output"`
			}{Input: 120, Output: 60},
		},
	}
}

func fixtureMeta() report.RunMetadata {
	return report.RunMetadata{
		RunID:        "run-20260510",
		StartedAt:    time.Now().Add(-5 * time.Minute),
		EndedAt:      time.Now(),
		ClaudeVersion: "claude 1.2.3",
		HelixVersion: "v1.10.0",
		GOOS:         "linux",
		GOARCH:       "amd64",
		Modes:        []string{"baseline", "native"},
		EnvKeys:      []string{"PATH", "HOME"},
		CorpusDir:    "eval/corpus",
	}
}

// TestWriteEvalReportJSON verifies the JSON structure of eval_report.json.
func TestWriteEvalReportJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "eval_report.json")
	mdPath := filepath.Join(dir, "eval_report.md")

	results := fixtureResults()
	meta := fixtureMeta()
	scores := map[string]score.Score{
		"task-rename/baseline": {Total: -1, Deltas: []score.Delta{{RuleID: "r1", Score: -1}}},
	}

	if err := report.WriteEvalReport(jsonPath, mdPath, results, scores, meta); err != nil {
		t.Fatalf("WriteEvalReport: %v", err)
	}

	// Verify JSON file.
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read eval_report.json: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal eval_report.json: %v", err)
	}

	for _, field := range []string{"schema_version", "metadata", "by_mode", "results"} {
		if _, ok := m[field]; !ok {
			t.Errorf("missing field %q in eval_report.json", field)
		}
	}

	// Verify schema_version.
	if m["schema_version"] != "1" {
		t.Errorf("schema_version: got %v, want 1", m["schema_version"])
	}
}

// TestWriteEvalReportMarkdown verifies the Markdown report structure.
func TestWriteEvalReportMarkdown(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "eval_report.json")
	mdPath := filepath.Join(dir, "eval_report.md")

	results := fixtureResults()
	meta := fixtureMeta()
	scores := map[string]score.Score{}

	if err := report.WriteEvalReport(jsonPath, mdPath, results, scores, meta); err != nil {
		t.Fatalf("WriteEvalReport: %v", err)
	}

	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read eval_report.md: %v", err)
	}
	md := string(data)

	// Verify required sections.
	for _, section := range []string{
		"run-20260510",          // run-id in header
		"Mode Comparison",       // mode-comparison table
		"Tool-Call Distribution", // tool distribution section
		"Guardrail Compliance",  // guardrail section
		"Failure Examples",      // failure examples section
		"INFORMATIONAL",         // judge boilerplate absent → placeholder
	} {
		if !strings.Contains(md, section) {
			t.Errorf("eval_report.md missing expected section or text: %q", section)
		}
	}
}

// TestEvalReportRendersWithoutLLMJudge verifies that the markdown report
// renders without error when no judge data is present and notes "(judge not run)".
func TestEvalReportRendersWithoutLLMJudge(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "eval_report.json")
	mdPath := filepath.Join(dir, "eval_report.md")

	if err := report.WriteEvalReport(jsonPath, mdPath, fixtureResults(), nil, fixtureMeta()); err != nil {
		t.Fatalf("WriteEvalReport with no judge: %v", err)
	}

	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read eval_report.md: %v", err)
	}
	if !strings.Contains(string(data), "judge not run") {
		t.Errorf("expected '(judge not run)' in report when no judge data present")
	}
}

// TestEvalReportMarkdownIncludesJudgeSection verifies that when judge output
// exists in the report directory, the markdown includes the "Informational: LLM judge"
// section with scoring data. When absent, it renders "(judge not run)".
func TestEvalReportMarkdownIncludesJudgeSection(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "eval_report.json")
	mdPath := filepath.Join(dir, "eval_report.md")
	judgeReportPath := filepath.Join(dir, "tool_behavior_judge.json")

	// Write a judge report file to the same directory.
	judgeJSON := `{
  "__readme": "INFORMATIONAL — DO NOT USE FOR CI GATING",
  "judge_model": "claude-sonnet-4-6",
  "judged_at": "2026-05-10T15:00:00Z",
  "tasks": [
    {
      "task_id": "task-rename",
      "mode": "native",
      "scores": {"right_tool": 1, "evidence": 1, "blast_radius": 1, "recovery": 0},
      "reasoning": "used rename_symbol correctly",
      "flags": []
    }
  ]
}`
	if err := os.WriteFile(judgeReportPath, []byte(judgeJSON), 0600); err != nil {
		t.Fatalf("write judge report: %v", err)
	}

	if err := report.WriteEvalReportWithJudge(jsonPath, mdPath, judgeReportPath, fixtureResults(), nil, fixtureMeta()); err != nil {
		t.Fatalf("WriteEvalReportWithJudge: %v", err)
	}

	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read eval_report.md: %v", err)
	}
	md := string(data)

	// Must contain the judge section header and boilerplate.
	if !strings.Contains(md, "INFORMATIONAL") {
		t.Error("markdown missing INFORMATIONAL section")
	}
	if !strings.Contains(md, "DO NOT GATE CI") {
		t.Error("markdown missing 'DO NOT GATE CI' warning")
	}
	// Must not say "(judge not run)" when judge data is present.
	if strings.Contains(md, "(judge not run)") {
		t.Error("markdown incorrectly says '(judge not run)' when judge data is present")
	}

	// Now test without judge data.
	dir2 := t.TempDir()
	jsonPath2 := filepath.Join(dir2, "eval_report.json")
	mdPath2 := filepath.Join(dir2, "eval_report.md")
	noJudgePath := filepath.Join(dir2, "tool_behavior_judge.json") // does not exist

	if err := report.WriteEvalReportWithJudge(jsonPath2, mdPath2, noJudgePath, fixtureResults(), nil, fixtureMeta()); err != nil {
		t.Fatalf("WriteEvalReportWithJudge (no judge): %v", err)
	}
	data2, err := os.ReadFile(mdPath2)
	if err != nil {
		t.Fatalf("read eval_report.md (no judge): %v", err)
	}
	if !strings.Contains(string(data2), "judge not run") {
		t.Error("expected '(judge not run)' when no judge file exists")
	}
}

// TestWriteToolBehavior verifies the tool behavior JSON output.
func TestWriteToolBehavior(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tool_behavior.json")

	scores := map[string]score.Score{
		"task-rename/baseline": {Total: -1, Deltas: []score.Delta{{RuleID: "r1", Score: -1}}},
		"task-rename/native":   {Total: 2, Deltas: []score.Delta{{RuleID: "r2", Score: 2}}},
	}

	if err := report.WriteToolBehavior(path, scores); err != nil {
		t.Fatalf("WriteToolBehavior: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode: got %v, want 0600", info.Mode().Perm())
	}

	var m map[string]interface{}
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["schema_version"]; !ok {
		t.Error("missing schema_version")
	}
}
