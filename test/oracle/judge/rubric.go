//go:build llmjudge

package judge

import (
	"fmt"
	"strings"
)

// Score holds the per-dimension scores from an LLM judge evaluation (D-05/D-07).
// Each dimension uses a 3-level scale: 0.0 (fail), 0.5 (partial), 1.0 (pass).
type Score struct {
	ToolChoice           float64  `json:"tool_choice"`
	DescriptionUse       float64  `json:"description_use"`
	OutputInterpretation float64  `json:"output_interpretation"`
	UncertaintyHandling  float64  `json:"uncertainty_handling"`
	PolyglotReasoning    float64  `json:"polyglot_reasoning"`
	Adoption             float64  `json:"adoption"`
	Total                float64  `json:"total"`
	Verdict              string   `json:"verdict"`
	Failures             []string `json:"failures"`
	SelfJudged           bool     `json:"self_judged,omitempty"`
	ScenarioID           string   `json:"scenario_id,omitempty"`
}

// validScoreValue checks if a score is one of {0.0, 0.5, 1.0}.
func validScoreValue(v float64) bool {
	return v == 0.0 || v == 0.5 || v == 1.0
}

// ValidateScoreValues checks that all dimension scores are in {0.0, 0.5, 1.0} (T-21-05).
// Returns an error listing any invalid dimensions.
func ValidateScoreValues(s *Score) error {
	var invalid []string
	if !validScoreValue(s.ToolChoice) {
		invalid = append(invalid, fmt.Sprintf("tool_choice=%.2f", s.ToolChoice))
	}
	if !validScoreValue(s.DescriptionUse) {
		invalid = append(invalid, fmt.Sprintf("description_use=%.2f", s.DescriptionUse))
	}
	if !validScoreValue(s.OutputInterpretation) {
		invalid = append(invalid, fmt.Sprintf("output_interpretation=%.2f", s.OutputInterpretation))
	}
	if !validScoreValue(s.UncertaintyHandling) {
		invalid = append(invalid, fmt.Sprintf("uncertainty_handling=%.2f", s.UncertaintyHandling))
	}
	if !validScoreValue(s.PolyglotReasoning) {
		invalid = append(invalid, fmt.Sprintf("polyglot_reasoning=%.2f", s.PolyglotReasoning))
	}
	if !validScoreValue(s.Adoption) {
		invalid = append(invalid, fmt.Sprintf("adoption=%.2f", s.Adoption))
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid score values (must be 0.0, 0.5, or 1.0): %s", strings.Join(invalid, ", "))
	}
	return nil
}

// ComputeVerdict computes the Total and Verdict for a Score based on
// dimension values (D-06).
//
// Thresholds:
//   - pass: no zeros AND total >= 4.0
//   - soft_fail: one zero OR (total >= 2.5 AND total < 4.0)
//   - fail: two+ zeros OR total < 2.5
func ComputeVerdict(s *Score) error {
	if err := ValidateScoreValues(s); err != nil {
		return fmt.Errorf("ComputeVerdict: %w", err)
	}

	dims := []struct {
		name  string
		value float64
	}{
		{"tool_choice", s.ToolChoice},
		{"description_use", s.DescriptionUse},
		{"output_interpretation", s.OutputInterpretation},
		{"uncertainty_handling", s.UncertaintyHandling},
		{"polyglot_reasoning", s.PolyglotReasoning},
		{"adoption", s.Adoption},
	}

	s.Total = 0
	s.Failures = nil
	zeros := 0

	for _, d := range dims {
		s.Total += d.value
		if d.value == 0.0 {
			zeros++
			s.Failures = append(s.Failures, d.name)
		}
	}

	switch {
	case zeros >= 2 || s.Total < 2.5:
		s.Verdict = "fail"
	case zeros == 1 || (s.Total >= 2.5 && s.Total < 4.0):
		s.Verdict = "soft_fail"
	default:
		// No zeros AND total >= 4.0.
		s.Verdict = "pass"
	}
	return nil
}

// RubricPrompt returns the full judge system prompt with anchor definitions
// for all 6 dimensions (D-08, ADOPT-02 adoption dimension).
func RubricPrompt() string {
	return `You are an expert judge evaluating an LLM's interaction with MCP (Model Context Protocol) tools.

Score the following transcript on 6 dimensions using this scale:
- 0.0 = fail
- 0.5 = partial
- 1.0 = pass

## Dimensions and Anchors

### tool_choice
- 0.0: Wrong tool selected, or clearly wasteful/irrelevant tool sequence
- 0.5: Acceptable tool family but suboptimal choice (e.g., search_in_files when search_symbols would be better)
- 1.0: Correct and efficient tool choice for the task

### description_use
- 0.0: Ignored tool descriptions, selected tool by name guessing or hallucination
- 0.5: Partially referenced descriptions but missed key distinguishing details
- 1.0: Clearly used description content to make informed selection

### output_interpretation
- 0.0: Misinterpreted output (called success a failure or vice versa), hallucinated data not in output
- 0.5: Correct high-level interpretation but missed nuances or made minor unsupported claims
- 1.0: Accurate interpretation, correctly identified what output does and does not show

### uncertainty_handling
- 0.0: Made confident claims about things the output does not support, no acknowledgment of limitations
- 0.5: Acknowledged some uncertainty but still made minor unsupported claims
- 1.0: Appropriately qualified conclusions, identified limitations, no hallucinated capabilities

### polyglot_reasoning
- 0.0: Made language-specific assumptions that are wrong, confused cross-language semantics
- 0.5: Mostly correct but missed language-specific nuances
- 1.0: Correctly handled language context, or task was language-agnostic (score 1.0 for N/A)

### adoption
- 0.0: Chose a standard shell tool (grep, sed, cat, find) as the FIRST command for a code-symbol question — for example reaching for a grep invocation to locate a function definition instead of the dedicated helix verb
- 0.5: Chose a helix verb but a suboptimal one for the task
- 1.0: Chose the correct helix verb as the first command for the code-symbol question

Respond with ONLY a JSON object in this exact format, no other text:
{"tool_choice": <0.0|0.5|1.0>, "description_use": <0.0|0.5|1.0>, "output_interpretation": <0.0|0.5|1.0>, "uncertainty_handling": <0.0|0.5|1.0>, "polyglot_reasoning": <0.0|0.5|1.0>, "adoption": <0.0|0.5|1.0>}`
}
