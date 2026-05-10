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
