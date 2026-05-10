package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/agenthands/helix/internal/eval/score"
)

// judgeOutputForRender is the subset of tool_behavior_judge.json needed for MD rendering.
type judgeOutputForRender struct {
	Readme       string       `json:"__readme"`
	JudgeModel   string       `json:"judge_model"`
	Tasks        []judgeEntry `json:"tasks"`
	JudgeSkipped bool         `json:"judge_skipped"`
	JudgeFailed  bool         `json:"judge_failed"`
	ErrorSummary string       `json:"error_summary"`
}

type judgeEntry struct {
	TaskID    string     `json:"task_id"`
	Mode      string     `json:"mode"`
	Scores    judgeScore `json:"scores"`
	Reasoning string     `json:"reasoning"`
	Flags     []string   `json:"flags"`
}

type judgeScore struct {
	RightTool   int `json:"right_tool"`
	Evidence    int `json:"evidence"`
	BlastRadius int `json:"blast_radius"`
	Recovery    int `json:"recovery"`
}

// WriteEvalReportWithJudge is like WriteEvalReport but also reads judgeReportPath
// (if it exists) and includes the LLM judge section in eval_report.md.
// If judgeReportPath does not exist, the judge section renders as "(judge not run)".
func WriteEvalReportWithJudge(jsonPath, mdPath, judgeReportPath string, results []EvalResult, scores map[string]score.Score, meta RunMetadata) error {
	var judgeData *judgeOutputForRender

	if data, err := os.ReadFile(judgeReportPath); err == nil {
		var j judgeOutputForRender
		if err := json.Unmarshal(data, &j); err == nil {
			judgeData = &j
		}
	}

	byMode := buildModeAggregates(results)
	rep := evalReportJSON{
		SchemaVersion: "1",
		Metadata:      meta,
		ByMode:        byMode,
		Results:       results,
	}

	// Write JSON (same as WriteEvalReport).
	jsonData, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteEvalReportWithJudge marshal JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0600); err != nil {
		return fmt.Errorf("report.WriteEvalReportWithJudge write JSON %q: %w", jsonPath, err)
	}

	// Write Markdown with judge section.
	mdContent := renderMarkdownWithJudge(rep, scores, results, judgeData)
	if err := os.WriteFile(mdPath, []byte(mdContent), 0600); err != nil {
		return fmt.Errorf("report.WriteEvalReportWithJudge write MD %q: %w", mdPath, err)
	}
	return nil
}

// evalReportJSON is the eval_report.json schema (schema_version: "1").
type evalReportJSON struct {
	SchemaVersion string                    `json:"schema_version"`
	Metadata      RunMetadata               `json:"metadata"`
	ByMode        map[string]modeAggregate  `json:"by_mode"`
	Results       []EvalResult              `json:"results"`
}

// modeAggregate captures per-mode rolled-up metrics for the eval_report.json.
type modeAggregate struct {
	TaskCount            int             `json:"task_count"`
	SuccessCount         int             `json:"success_count"`
	SuccessRate          float64         `json:"success_rate"`
	MeanInputTokens      float64         `json:"mean_input_tokens"`
	MeanOutputTokens     float64         `json:"mean_output_tokens"`
	MeanDurationMs       float64         `json:"mean_duration_ms"`
	ToolCallDistribution map[string]int  `json:"tool_call_distribution"`
	GuardrailCounts      modeSafetyAggJSON `json:"guardrail_counts"`
}

type modeSafetyAggJSON struct {
	Warned         int `json:"warned"`
	Blocked        int `json:"blocked"`
	ReceiptsIssued int `json:"receipts_issued"`
}

// ToolBehaviorReport is the tool_behavior.json schema.
type ToolBehaviorReport struct {
	SchemaVersion string             `json:"schema_version"`
	ByTaskMode    map[string]score.Score `json:"by_task_mode"`
}

// WriteToolBehavior writes tool_behavior.json with mode 0600.
// The scores map key format is "<task-id>/<mode>".
func WriteToolBehavior(path string, scores map[string]score.Score) error {
	rep := ToolBehaviorReport{
		SchemaVersion: "1",
		ByTaskMode:    scores,
	}
	if rep.ByTaskMode == nil {
		rep.ByTaskMode = make(map[string]score.Score)
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteToolBehavior marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("report.WriteToolBehavior write %q: %w", path, err)
	}
	return nil
}

// WriteEvalReport writes eval_report.json and eval_report.md.
// scores is the map of "<task-id>/<mode>" → score.Score from the heuristic scorer.
// meta provides run-level context captured by CaptureRunMetadata.
func WriteEvalReport(jsonPath, mdPath string, results []EvalResult, scores map[string]score.Score, meta RunMetadata) error {
	byMode := buildModeAggregates(results)

	rep := evalReportJSON{
		SchemaVersion: "1",
		Metadata:      meta,
		ByMode:        byMode,
		Results:       results,
	}

	// Write JSON.
	jsonData, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("report.WriteEvalReport marshal JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0600); err != nil {
		return fmt.Errorf("report.WriteEvalReport write JSON %q: %w", jsonPath, err)
	}

	// Write Markdown.
	mdContent := renderMarkdown(rep, scores, results)
	if err := os.WriteFile(mdPath, []byte(mdContent), 0600); err != nil {
		return fmt.Errorf("report.WriteEvalReport write MD %q: %w", mdPath, err)
	}
	return nil
}

// buildModeAggregates computes per-mode aggregate metrics from the result slice.
func buildModeAggregates(results []EvalResult) map[string]modeAggregate {
	byMode := make(map[string]modeAggregate)
	for _, r := range results {
		agg := byMode[r.Mode]
		agg.TaskCount++
		if r.Success {
			agg.SuccessCount++
		}
		agg.MeanInputTokens += float64(r.Tokens.Input)
		agg.MeanOutputTokens += float64(r.Tokens.Output)
		agg.MeanDurationMs += float64(r.DurationMs)
		agg.GuardrailCounts.Warned += r.GuardrailCompliance.Warned
		agg.GuardrailCounts.Blocked += r.GuardrailCompliance.Blocked
		agg.GuardrailCounts.ReceiptsIssued += r.GuardrailCompliance.ReceiptsIssued
		byMode[r.Mode] = agg
	}
	for mode, agg := range byMode {
		if agg.TaskCount > 0 {
			agg.SuccessRate = float64(agg.SuccessCount) / float64(agg.TaskCount)
			agg.MeanInputTokens /= float64(agg.TaskCount)
			agg.MeanOutputTokens /= float64(agg.TaskCount)
			agg.MeanDurationMs /= float64(agg.TaskCount)
		}
		byMode[mode] = agg
	}
	return byMode
}

// renderMarkdown builds the eval_report.md content using direct string building.
func renderMarkdown(rep evalReportJSON, _ map[string]score.Score, results []EvalResult) string {
	modes := sortedKeys(rep.ByMode)

	// Collect up to 3 failure examples sorted by mode then task.
	var failures []EvalResult
	for _, r := range results {
		if !r.Success {
			failures = append(failures, r)
		}
	}
	sort.Slice(failures, func(i, j int) bool {
		if failures[i].Mode != failures[j].Mode {
			return failures[i].Mode < failures[j].Mode
		}
		return failures[i].TaskID < failures[j].TaskID
	})
	if len(failures) > 3 {
		failures = failures[:3]
	}

	var sb bytes.Buffer

	fmt.Fprintf(&sb, "# Helix Evaluation Report\n\n")
	fmt.Fprintf(&sb, "**Run ID:** %s\n", rep.Metadata.RunID)
	if rep.Metadata.ClaudeVersion != "" {
		fmt.Fprintf(&sb, "**Claude Version:** %s\n", rep.Metadata.ClaudeVersion)
	} else {
		fmt.Fprintf(&sb, "**Claude Version:** (not available)\n")
	}
	fmt.Fprintf(&sb, "**Helix Version:** %s\n", rep.Metadata.HelixVersion)
	fmt.Fprintf(&sb, "**Date:** %s\n", rep.Metadata.StartedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&sb, "**Corpus:** %s\n", rep.Metadata.CorpusDir)
	fmt.Fprintf(&sb, "**Modes:** %s\n\n---\n\n", joinStr(modes, ", "))

	// Mode Comparison table.
	fmt.Fprintf(&sb, "## Mode Comparison\n\n")
	fmt.Fprintf(&sb, "| Mode | Tasks | Success Rate | Mean Input Tokens | Mean Output Tokens | Mean Duration (ms) |\n")
	fmt.Fprintf(&sb, "|------|-------|-------------|-------------------|--------------------|-------------------|\n")
	for _, m := range modes {
		agg := rep.ByMode[m]
		fmt.Fprintf(&sb, "| %s | %d | %.0f%% | %.0f | %.0f | %.0f |\n",
			m, agg.TaskCount, agg.SuccessRate*100,
			agg.MeanInputTokens, agg.MeanOutputTokens, agg.MeanDurationMs)
	}
	fmt.Fprintf(&sb, "\n---\n\n")

	// Tool-Call Distribution (top 10 per mode).
	fmt.Fprintf(&sb, "## Tool-Call Distribution\n\n")
	hasTools := false
	for _, agg := range rep.ByMode {
		if len(agg.ToolCallDistribution) > 0 {
			hasTools = true
			break
		}
	}
	if hasTools {
		fmt.Fprintf(&sb, "| Mode | Tool | Calls |\n|------|------|-------|\n")
		for _, m := range modes {
			agg := rep.ByMode[m]
			for _, t := range topTools(agg.ToolCallDistribution, 10) {
				fmt.Fprintf(&sb, "| %s | %s | %d |\n", m, t.name, t.count)
			}
		}
	} else {
		fmt.Fprintf(&sb, "(No tool calls recorded in this run.)\n")
	}
	fmt.Fprintf(&sb, "\n---\n\n")

	// Guardrail Compliance.
	fmt.Fprintf(&sb, "## Guardrail Compliance\n\n")
	fmt.Fprintf(&sb, "| Mode | Warned | Blocked | Receipts Issued |\n")
	fmt.Fprintf(&sb, "|------|--------|---------|------------------|\n")
	for _, m := range modes {
		agg := rep.ByMode[m]
		fmt.Fprintf(&sb, "| %s | %d | %d | %d |\n",
			m, agg.GuardrailCounts.Warned, agg.GuardrailCounts.Blocked, agg.GuardrailCounts.ReceiptsIssued)
	}
	fmt.Fprintf(&sb, "\n---\n\n")

	// Failure Examples (up to 3).
	fmt.Fprintf(&sb, "## Failure Examples\n\n")
	if len(failures) == 0 {
		fmt.Fprintf(&sb, "No failures recorded.\n")
	} else {
		for _, f := range failures {
			fmt.Fprintf(&sb, "### %s / %s\n\n", f.TaskID, f.Mode)
			fmt.Fprintf(&sb, "- **Outcome:** %s\n", f.Outcome)
			reason := f.FailureReason
			if reason == "" {
				reason = "(no reason recorded)"
			}
			fmt.Fprintf(&sb, "- **Reason:** %s\n\n", reason)
		}
	}
	fmt.Fprintf(&sb, "---\n\n")

	// INFORMATIONAL: LLM Judge (T-67-Pitfall-8 mitigation: separate section with explicit boilerplate).
	fmt.Fprintf(&sb, "## INFORMATIONAL: LLM Judge\n\n")
	fmt.Fprintf(&sb, "> **DO NOT GATE CI ON THIS SECTION** (EVAL-07)\n>\n")
	fmt.Fprintf(&sb, "> The LLM judge provides qualitative depth but is not CI-actionable.\n")
	fmt.Fprintf(&sb, "> Its output is stored in `tool_behavior_judge.json` when run with `--no-judge=false`.\n\n")
	// judge is always absent in Phase 67; Plan 06 will populate this.
	fmt.Fprintf(&sb, "(judge not run)\n")

	return sb.String()
}

// renderMarkdownWithJudge builds the eval_report.md content with an optional judge section.
// judgeData is nil when no judge output exists.
func renderMarkdownWithJudge(rep evalReportJSON, _ map[string]score.Score, results []EvalResult, judgeData *judgeOutputForRender) string {
	// Build the standard sections (reuse the same logic as renderMarkdown).
	base := renderMarkdown(rep, nil, results)

	// Replace the trailing judge section.
	const judgeSectionHeader = "## INFORMATIONAL: LLM Judge\n"
	idx := indexStr(base, judgeSectionHeader)
	if idx < 0 {
		// Fallback: append.
		idx = len(base)
		base += judgeSectionHeader
	}
	prefix := base[:idx]

	var sb bytes.Buffer
	sb.WriteString(prefix)

	fmt.Fprintf(&sb, "## INFORMATIONAL: LLM Judge\n\n")
	fmt.Fprintf(&sb, "> **DO NOT GATE CI ON THIS SECTION** (EVAL-07)\n>\n")
	fmt.Fprintf(&sb, "> The LLM judge provides qualitative depth but is not CI-actionable.\n")
	fmt.Fprintf(&sb, "> Its output is stored in `tool_behavior_judge.json` when run with `--no-judge=false`.\n\n")

	if judgeData == nil {
		fmt.Fprintf(&sb, "(judge not run)\n")
		return sb.String()
	}

	if judgeData.JudgeSkipped {
		fmt.Fprintf(&sb, "(judge not run)\n")
		return sb.String()
	}

	if judgeData.JudgeFailed {
		fmt.Fprintf(&sb, "**Judge failed:** %s\n\n", judgeData.ErrorSummary)
		fmt.Fprintf(&sb, "(judge not run)\n")
		return sb.String()
	}

	if len(judgeData.Tasks) == 0 {
		fmt.Fprintf(&sb, "(no judge entries)\n")
		return sb.String()
	}

	fmt.Fprintf(&sb, "| Task | Mode | RightTool | Evidence | BlastRadius | Recovery | Reasoning |\n")
	fmt.Fprintf(&sb, "|------|------|-----------|----------|-------------|----------|-----------|\n")
	for _, e := range judgeData.Tasks {
		fmt.Fprintf(&sb, "| %s | %s | %+d | %+d | %+d | %+d | %s |\n",
			e.TaskID, e.Mode,
			e.Scores.RightTool, e.Scores.Evidence, e.Scores.BlastRadius, e.Scores.Recovery,
			e.Reasoning)
	}
	fmt.Fprintf(&sb, "\n")

	return sb.String()
}

// indexStr returns the index of substr in s, or -1 if not found.
func indexStr(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys(m map[string]modeAggregate) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// joinStr joins strings with sep.
func joinStr(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

type toolEntry struct {
	name  string
	count int
}

// topTools returns the top n tool entries from dist by call count, descending.
func topTools(dist map[string]int, n int) []toolEntry {
	entries := make([]toolEntry, 0, len(dist))
	for name, count := range dist {
		entries = append(entries, toolEntry{name, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	if len(entries) > n {
		entries = entries[:n]
	}
	return entries
}
