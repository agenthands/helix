//go:build llm || llmjudge

package llm

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	oai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	// EnvAPIKey is the environment variable for the Anthropic API key.
	EnvAPIKey = "ANTHROPIC_API_KEY"
	// EnvDeepSeekKey is the environment variable for the DeepSeek API key.
	EnvDeepSeekKey = "DEEPSEEK_API_KEY"
	// EnvTestModel overrides the subject model for behavioral tests.
	EnvTestModel = "HELIX_TEST_MODEL"
	// EnvJudgeModel overrides the judge model for scoring.
	EnvJudgeModel = "HELIX_JUDGE_MODEL"
	// EnvInlineJudge enables inline judge mode when set to "1".
	EnvInlineJudge = "HELIX_INLINE_JUDGE"
	// EnvLLMProvider selects the LLM provider: "anthropic" (default) or "deepseek".
	EnvLLMProvider = "HELIX_LLM_PROVIDER"
	// DefaultSubjectModel is the cheapest Claude model for behavioral tests (D-11).
	DefaultSubjectModel = "claude-haiku-4-5-20251001"
	// DefaultDeepSeekModel is the default DeepSeek model.
	DefaultDeepSeekModel = "deepseek-chat"
)

// Provider returns the active LLM provider name.
func Provider() string {
	if p := os.Getenv(EnvLLMProvider); p != "" {
		return p
	}
	// Auto-detect: if DeepSeek key is set but Anthropic isn't, use DeepSeek.
	if os.Getenv(EnvAPIKey) == "" && os.Getenv(EnvDeepSeekKey) != "" {
		return "deepseek"
	}
	return "anthropic"
}

// SkipWithoutAPIKey skips the test if no LLM API key is available (D-18).
// Never logs the key value to avoid information disclosure (T-21-01).
func SkipWithoutAPIKey(t *testing.T) {
	t.Helper()
	p := Provider()
	switch p {
	case "deepseek":
		if os.Getenv(EnvDeepSeekKey) == "" {
			t.Skip("DEEPSEEK_API_KEY not set")
		}
	default:
		if os.Getenv(EnvAPIKey) == "" {
			t.Skip("ANTHROPIC_API_KEY not set")
		}
	}
}

// NewClient creates an Anthropic client that reads ANTHROPIC_API_KEY from env.
// Only used when provider is "anthropic".
func NewClient() anthropic.Client {
	return anthropic.NewClient()
}

// SubjectModel returns the model ID for behavioral tests.
// Uses HELIX_TEST_MODEL if set, otherwise defaults per provider (D-10/D-11).
func SubjectModel() string {
	if m := os.Getenv(EnvTestModel); m != "" {
		return m
	}
	if Provider() == "deepseek" {
		return DefaultDeepSeekModel
	}
	return DefaultSubjectModel
}

// JudgeModel returns the model ID for judge scoring and whether the run is self-judged.
// Uses HELIX_JUDGE_MODEL if set, otherwise falls back to SubjectModel (D-10/D-12).
func JudgeModel() (model string, selfJudged bool) {
	if m := os.Getenv(EnvJudgeModel); m != "" {
		return m, false
	}
	return SubjectModel(), true
}

// AskSingleTurn sends a single-turn prompt and returns the response text,
// stop reason, and any error. Routes to the active provider.
func AskSingleTurn(ctx context.Context, client anthropic.Client, model, system, userPrompt string) (responseText string, stopReason string, err error) {
	if Provider() == "deepseek" {
		return askDeepSeek(ctx, model, system, userPrompt)
	}
	return askAnthropic(ctx, client, model, system, userPrompt)
}

// askAnthropic sends via Anthropic API. Uses MaxTokens: 1024 (D-01).
func askAnthropic(ctx context.Context, client anthropic.Client, model, system, userPrompt string) (string, string, error) {
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

	for _, block := range message.Content {
		if v, ok := block.AsAny().(anthropic.TextBlock); ok {
			return v.Text, string(message.StopReason), nil
		}
	}

	return "", string(message.StopReason), fmt.Errorf("no text content in response")
}

// askDeepSeek sends via DeepSeek's OpenAI-compatible API.
func askDeepSeek(ctx context.Context, model, system, userPrompt string) (string, string, error) {
	client := oai.NewClient(
		option.WithAPIKey(os.Getenv(EnvDeepSeekKey)),
		option.WithBaseURL("https://api.deepseek.com"),
	)

	var messages []oai.ChatCompletionMessageParamUnion
	if system != "" {
		messages = append(messages, oai.SystemMessage(system))
	}
	messages = append(messages, oai.UserMessage(userPrompt))

	resp, err := client.Chat.Completions.New(ctx, oai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return "", "", fmt.Errorf("deepseek API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("no choices in deepseek response")
	}

	choice := resp.Choices[0]
	return choice.Message.Content, string(choice.FinishReason), nil
}

// InterCallDelay sleeps 250ms between API calls to avoid rate limits (D-19).
func InterCallDelay() {
	time.Sleep(250 * time.Millisecond)
}
