//go:build llm || llmjudge

package llm

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	// EnvAPIKey is the environment variable for the Anthropic API key.
	EnvAPIKey = "ANTHROPIC_API_KEY"
	// EnvTestModel overrides the subject model for behavioral tests.
	EnvTestModel = "SERENA_TEST_MODEL"
	// EnvJudgeModel overrides the judge model for scoring.
	EnvJudgeModel = "SERENA_JUDGE_MODEL"
	// EnvInlineJudge enables inline judge mode when set to "1".
	EnvInlineJudge = "SERENA_INLINE_JUDGE"
	// DefaultSubjectModel is the cheapest Claude model for behavioral tests (D-11).
	DefaultSubjectModel = "claude-haiku-4-20250414"
)

// SkipWithoutAPIKey skips the test if ANTHROPIC_API_KEY is not set (D-18).
// Never logs the key value to avoid information disclosure (T-21-01).
func SkipWithoutAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv(EnvAPIKey) == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
}

// NewClient creates an Anthropic client that reads ANTHROPIC_API_KEY from env.
func NewClient() anthropic.Client {
	return anthropic.NewClient()
}

// SubjectModel returns the model ID for behavioral tests.
// Uses SERENA_TEST_MODEL if set, otherwise DefaultSubjectModel (D-10/D-11).
func SubjectModel() string {
	if m := os.Getenv(EnvTestModel); m != "" {
		return m
	}
	return DefaultSubjectModel
}

// JudgeModel returns the model ID for judge scoring and whether the run is self-judged.
// Uses SERENA_JUDGE_MODEL if set, otherwise falls back to SubjectModel (D-10/D-12).
func JudgeModel() (model string, selfJudged bool) {
	if m := os.Getenv(EnvJudgeModel); m != "" {
		return m, false
	}
	return SubjectModel(), true
}

// AskSingleTurn sends a single-turn prompt and returns the response text,
// stop reason, and any error. Uses MaxTokens: 1024 (D-01).
func AskSingleTurn(ctx context.Context, client anthropic.Client, model, system, userPrompt string) (responseText string, stopReason string, err error) {
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}

	message, err := client.Messages.New(ctx, params)
	if err != nil {
		return "", "", fmt.Errorf("anthropic API call failed: %w", err)
	}

	// Extract text from first TextBlock in response content.
	for _, block := range message.Content {
		if v, ok := block.AsAny().(anthropic.TextBlock); ok {
			return v.Text, string(message.StopReason), nil
		}
	}

	return "", string(message.StopReason), fmt.Errorf("no text content in response")
}

// InterCallDelay sleeps 250ms between API calls to avoid rate limits (D-19).
func InterCallDelay() {
	time.Sleep(250 * time.Millisecond)
}
