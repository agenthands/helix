// Package judge runs the informational LLM judge pass for Phase 67 evaluation.
// It sends each (task, mode) trace to the Anthropic API (default: claude-sonnet-4-6)
// with the family-specific rubric from eval/judge/prompts/rubric.md and records
// per-axis scores into tool_behavior_judge.json. The judge is NEVER a CI gate
// per EVAL-07: a judge failure does not affect make eval's exit code.
package judge

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"context"
)

//go:embed prompts/rubric.md
var embeddedRubric string

//go:embed prompts/tool_behavior.tmpl
var embeddedTemplate string

//go:embed prompts/few_shot.md
var embeddedFewShot string

// RubricContent returns the embedded rubric markdown content.
// Exported for use by tests and judge orchestration.
func RubricContent() string {
	return embeddedRubric
}

// TemplateContent returns the embedded prompt template content.
func TemplateContent() string {
	return embeddedTemplate
}

// FewShotContent returns the embedded few-shot examples content.
func FewShotContent() string {
	return embeddedFewShot
}

const (
	anthropicVersion = "2023-06-01"
	defaultBaseURL   = "https://api.anthropic.com/v1/messages"
	defaultModel     = "claude-sonnet-4-6"
	maxRetries       = 3
)

// Options configures a Client.
type Options struct {
	// APIKey is the Anthropic API key. Never logged.
	APIKey string
	// BaseURL overrides the Anthropic API endpoint (used for testing via httptest.Server).
	BaseURL string
	// Model overrides the default model (claude-sonnet-4-6).
	Model string
	// InitialWait sets the first retry delay (default 1s; override for tests).
	InitialWait time.Duration
	// Logger is the slog.Logger instance. Defaults to slog.Default() if nil.
	Logger *slog.Logger
}

// Client is a hand-rolled Anthropic Messages API client (no SDK dep per RESEARCH Pitfall 9).
type Client struct {
	baseURL     string
	apiKey      string
	model       string
	initialWait time.Duration
	httpc       *http.Client
	logger      *slog.Logger
}

// NewClient creates a new Client from opts.
func NewClient(opts Options) *Client {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := opts.Model
	if model == "" {
		model = defaultModel
	}
	initialWait := opts.InitialWait
	if initialWait <= 0 {
		initialWait = time.Second
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:     baseURL,
		apiKey:      opts.APIKey,
		model:       model,
		initialWait: initialWait,
		httpc:       &http.Client{},
		logger:      logger,
	}
}

// messagesRequest is the Anthropic Messages API request body.
type messagesRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system"`
	Messages  []apiMessage  `json:"messages"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// messagesResponse is the Anthropic Messages API response body (partial).
type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Score sends a single chat completion request to the Anthropic Messages API.
// It retries on 429 and 5xx responses (up to maxRetries attempts, exponential backoff).
// Terminal 4xx errors (e.g., 401) are returned immediately without retry.
// The API key is NEVER included in log output.
func (c *Client) Score(ctx context.Context, system, user string, maxTokens int) (string, error) {
	body := messagesRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages: []apiMessage{
			{Role: "user", Content: user},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("judge.Client.Score marshal: %w", err)
	}

	wait := c.initialWait
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff; honour context cancellation during wait.
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
		}

		text, retryable, err := c.doRequest(ctx, bodyBytes)
		if err == nil {
			return text, nil
		}

		lastErr = err

		if !retryable {
			// Terminal error (e.g. 401 Unauthorized) — do not retry.
			return "", err
		}

		c.logger.Debug("judge.Client.Score retrying",
			"attempt", attempt+1,
			"max_retries", maxRetries,
			"error", err,
		)
	}

	return "", fmt.Errorf("judge.Client.Score: exhausted %d retries: %w", maxRetries, lastErr)
}

// doRequest performs a single POST to the Anthropic API.
// Returns (text, retryable=false, nil) on success,
// (_, retryable=true/false, err) on failure.
func (c *Client) doRequest(ctx context.Context, bodyBytes []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", false, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpc.Do(req)
	if err != nil {
		// Network error: retryable.
		return "", true, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK:
		var parsed messagesResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return "", false, fmt.Errorf("parse response: %w", err)
		}
		for _, block := range parsed.Content {
			if block.Type == "text" {
				return block.Text, false, nil
			}
		}
		return "", false, fmt.Errorf("no text block in response")

	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		// Retryable.
		return "", true, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(respBody))

	default:
		// 4xx (except 429) — terminal, do not retry.
		return "", false, fmt.Errorf("HTTP %d (terminal): %s", resp.StatusCode, bytes.TrimSpace(respBody))
	}
}
