package judge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/judge"
)

// cannedResponse returns a valid Anthropic Messages API response body.
func cannedResponse(text string) []byte {
	resp := map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet-4-6",
		"stop_reason": "end_turn",
		"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// TestClientPostsCorrectShape verifies headers, model, max_tokens, system,
// and user message are present in the outbound request.
func TestClientPostsCorrectShape(t *testing.T) {
	var captured *http.Request
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse("ok"))
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{
		APIKey:  "test-key-123",
		BaseURL: srv.URL,
	})

	_, err := c.Score(context.Background(), "system prompt", "user message", 256)
	if err != nil {
		t.Fatalf("Score returned error: %v", err)
	}

	// Check headers.
	if captured.Header.Get("x-api-key") == "" {
		t.Error("missing x-api-key header")
	}
	if captured.Header.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version = %q; want 2023-06-01", captured.Header.Get("anthropic-version"))
	}
	if !strings.Contains(captured.Header.Get("content-type"), "application/json") {
		t.Errorf("content-type = %q; want application/json", captured.Header.Get("content-type"))
	}

	// Check body.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["model"] != "claude-sonnet-4-6" {
		t.Errorf("body.model = %v; want claude-sonnet-4-6", body["model"])
	}
	if body["max_tokens"] == nil {
		t.Error("body.max_tokens not set")
	}
	if body["system"] == nil {
		t.Error("body.system not set")
	}
	msgs, _ := body["messages"].([]any)
	if len(msgs) == 0 {
		t.Error("body.messages empty")
	}
}

// TestClientReturnsTextContent verifies text is extracted from the content array.
func TestClientReturnsTextContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse("hello world"))
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{APIKey: "k", BaseURL: srv.URL})
	text, err := c.Score(context.Background(), "sys", "user", 100)
	if err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q; want hello world", text)
	}
}

// TestClientHonorsContextDeadline verifies that a hanging server causes
// Score to return a context error when the deadline expires.
func TestClientHonorsContextDeadline(t *testing.T) {
	// Use a channel to signal when the handler should unblock on Close().
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait until either the request context is done or the test signals done.
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	defer func() {
		close(done)
		srv.Close()
	}()

	c := judge.NewClient(judge.Options{APIKey: "k", BaseURL: srv.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.Score(ctx, "sys", "user", 100)
	if err == nil {
		t.Fatal("expected error from deadline, got nil")
	}
}

// TestClientHandles429 verifies retry-with-backoff on rate-limit responses.
func TestClientHandles429(t *testing.T) {
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse("retried ok"))
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		InitialWait: 1 * time.Millisecond, // speed up test
	})
	text, err := c.Score(context.Background(), "sys", "user", 100)
	if err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if text != "retried ok" {
		t.Errorf("text = %q; want retried ok", text)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d; want 3", attempts)
	}
}

// TestClientHandles5xxAsRetryable verifies 503 is retried and succeeds on 2nd attempt.
func TestClientHandles5xxAsRetryable(t *testing.T) {
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse("5xx then ok"))
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		InitialWait: 1 * time.Millisecond,
	})
	text, err := c.Score(context.Background(), "sys", "user", 100)
	if err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if text != "5xx then ok" {
		t.Errorf("text = %q; want 5xx then ok", text)
	}
}

// TestClientHandles4xxAsTerminal verifies 401 is not retried.
func TestClientHandles4xxAsTerminal(t *testing.T) {
	var attempts int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := judge.NewClient(judge.Options{
		APIKey:      "k",
		BaseURL:     srv.URL,
		InitialWait: 1 * time.Millisecond,
	})
	_, err := c.Score(context.Background(), "sys", "user", 100)
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d; want 1 (no retry on 4xx)", attempts)
	}
}

// TestClientNeverLogsAPIKey asserts the API key value does not appear in log output.
func TestClientNeverLogsAPIKey(t *testing.T) {
	const secretKey = "sk-secret-never-log-this-value"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cannedResponse("ok"))
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := judge.NewClient(judge.Options{
		APIKey:  secretKey,
		BaseURL: srv.URL,
		Logger:  logger,
	})
	_, _ = c.Score(context.Background(), "sys", "user", 100)

	if strings.Contains(logBuf.String(), secretKey) {
		t.Errorf("API key leaked into log output: %s", logBuf.String())
	}
}

// TestRubricFamiliesPresent verifies rubric.md covers all five EVAL-05 families.
func TestRubricFamiliesPresent(t *testing.T) {
	rubric := judge.RubricContent()
	families := []string{"rename", "delete", "public_api", "large_edit", "security"}
	for _, f := range families {
		if !strings.Contains(rubric, f) {
			t.Errorf("rubric.md missing family %q", f)
		}
	}
}
