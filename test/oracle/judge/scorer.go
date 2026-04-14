//go:build llmjudge

package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/postfix/serena/test/harness"
	llm "github.com/postfix/serena/test/oracle/llm"
)

// ScoreTranscript calls the judge model to score a transcript via the rubric prompt.
// Retries once if score values are invalid (T-21-05).
func ScoreTranscript(ctx context.Context, client anthropic.Client, model string, tr *llm.Transcript) (*Score, error) {
	system := RubricPrompt()
	userPrompt := formatTranscriptForJudge(tr)

	responseText, _, err := llm.AskSingleTurn(ctx, client, model, system, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("judge API call for %s: %w", tr.ScenarioID, err)
	}

	score, err := ParseScoreJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("parsing judge response for %s: %w", tr.ScenarioID, err)
	}

	// Set metadata before validation.
	_, selfJudgedFromModel := llm.JudgeModel()
	score.ScenarioID = tr.ScenarioID
	score.SelfJudged = tr.SelfJudged || selfJudgedFromModel

	// Validate and retry once if invalid.
	if valErr := ValidateScoreValues(score); valErr != nil {
		retryPrompt := userPrompt + "\n\nYour previous response had invalid values. Each dimension MUST be exactly 0.0, 0.5, or 1.0."
		responseText2, _, err2 := llm.AskSingleTurn(ctx, client, model, system, retryPrompt)
		if err2 != nil {
			return nil, fmt.Errorf("judge retry API call for %s: %w", tr.ScenarioID, err2)
		}
		score2, err2 := ParseScoreJSON(responseText2)
		if err2 != nil {
			return nil, fmt.Errorf("parsing judge retry response for %s: %w (original error: %v)", tr.ScenarioID, err2, valErr)
		}
		score2.ScenarioID = tr.ScenarioID
		score2.SelfJudged = tr.SelfJudged || selfJudgedFromModel
		if valErr2 := ValidateScoreValues(score2); valErr2 != nil {
			return nil, fmt.Errorf("judge retry still invalid for %s: %w", tr.ScenarioID, valErr2)
		}
		score = score2
	}

	if err := ComputeVerdict(score); err != nil {
		return nil, fmt.Errorf("computing verdict for %s: %w", tr.ScenarioID, err)
	}
	return score, nil
}

// formatTranscriptForJudge builds the user prompt showing the transcript to judge.
func formatTranscriptForJudge(tr *llm.Transcript) string {
	var sb strings.Builder
	sb.WriteString("## Transcript to Judge\n\n")
	fmt.Fprintf(&sb, "Scenario: %s\n", tr.ScenarioID)
	fmt.Fprintf(&sb, "Category: %s\n\n", tr.Category)
	fmt.Fprintf(&sb, "System prompt given to subject:\n%s\n\n", tr.System)
	fmt.Fprintf(&sb, "User prompt given to subject:\n%s\n\n", tr.UserPrompt)
	fmt.Fprintf(&sb, "Subject's response:\n%s\n", tr.Response)
	return sb.String()
}

// ParseScoreJSON extracts a Score from a JSON object in the response text.
// Finds the first { and last } to extract the JSON substring.
func ParseScoreJSON(text string) (*Score, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response: %.100s", text)
	}

	jsonStr := text[start : end+1]
	var score Score
	if err := json.Unmarshal([]byte(jsonStr), &score); err != nil {
		return nil, fmt.Errorf("unmarshaling score JSON: %w (json: %s)", err, jsonStr)
	}
	return &score, nil
}

// WriteScore marshals a score to indented JSON and writes it to
// test/oracle/judge/testdata/scores/{scenario_id}.json (D-14).
func WriteScore(t *testing.T, score *Score) {
	t.Helper()

	dir := filepath.Join(harness.ProjectRoot(), "test", "oracle", "judge", "testdata", "scores")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating scores dir: %v", err)
	}

	data, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		t.Fatalf("marshaling score: %v", err)
	}

	path := filepath.Join(dir, score.ScenarioID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing score %s: %v", path, err)
	}
	t.Logf("score written: %s", path)
}
