//go:build integration || llm || llmjudge

// Deferred CONT-03 error category tests (D-13).
//
// Covers the three error categories deferred from Phase 19:
//   - unsupported: LS-dependent tools fail cleanly for unsupported languages
//   - timeout: context deadline produces clean error
//   - circuit_open: circuit breaker exhaustion prevents attempts
//
// The unsupported category gets a deterministic golden file.
// Timeout and circuit_open validate error structure (IsError=true, non-empty,
// no misleading success phrases) rather than exact golden text.

package runtime_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/test/harness"
)

// successPhrases are strings that should never appear in error responses.
var successPhrases = []string{"Success", "completed successfully", "Done"}

func TestDeferred_ErrorCategories_Unsupported(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "unsupported")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	// Call an LS-dependent tool -- should fail because no LS handles .xyz files.
	result := harness.CallToolExpectError(t, runner.Session, "search_symbols", map[string]any{
		"query": "main",
	})

	text := harness.TextContent(result)
	require.NotEmpty(t, text, "unsupported error should have non-empty message")

	// Verify no misleading success phrases.
	for _, phrase := range successPhrases {
		assert.False(t, strings.Contains(text, phrase),
			"unsupported error should not contain success phrase %q: %s", phrase, text)
	}

	// Normalize paths for golden comparison.
	normalized := strings.ReplaceAll(text, fixtureDir, "<FIXTURE_DIR>")

	// Write/compare golden file.
	goldenPath := filepath.Join(
		harness.ProjectRoot(),
		"test", "oracle", "contract", "testdata", "golden", "errors", "unsupported.golden",
	)
	harness.AssertGolden(t, goldenPath, []byte(normalized))
}

func TestDeferred_ErrorCategories_Timeout(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})

	// Create a context that has already expired.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure deadline has passed

	// Call tool directly via session (bypass harness.CallTool which has its own timeout).
	result, err := runner.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_symbols",
		Arguments: map[string]any{"query": "main"},
	})

	// The error should be context deadline exceeded at the transport layer.
	// Either err is non-nil (transport-level deadline), or result.IsError is true.
	if err != nil {
		// Transport-level timeout -- expected.
		errText := err.Error()
		assert.True(t,
			strings.Contains(errText, "deadline") ||
				strings.Contains(errText, "timeout") ||
				strings.Contains(errText, "context"),
			"timeout error should mention deadline/timeout/context: %s", errText)
		return
	}

	// If no transport error, the tool should have returned an error result.
	require.NotNil(t, result, "expected either error or result")
	require.True(t, result.IsError, "expected error result for timed-out call")
	text := harness.TextContent(result)
	require.NotEmpty(t, text, "timeout error should have non-empty message")

	for _, phrase := range successPhrases {
		assert.False(t, strings.Contains(text, phrase),
			"timeout error should not contain success phrase %q: %s", phrase, text)
	}
}

func TestDeferred_ErrorCategories_CircuitOpen(t *testing.T) {
	// Test the circuit breaker at the unit level: exhaust the restart budget
	// and verify the breaker reports its state honestly (T-20-08).

	budget := 3
	cb := lspool.NewCircuitBreaker("test-lang", 30*time.Second, budget, nil)

	// Initially, the circuit should allow attempts.
	require.True(t, cb.CanAttempt(), "fresh breaker should allow attempts")
	require.Equal(t, 0, cb.Failures(), "fresh breaker should have 0 failures")

	// Record failures up to and beyond the budget.
	for i := 0; i < budget+1; i++ {
		cb.RecordFailure()
	}

	// After exceeding the restart budget, circuit should stay permanently open.
	assert.False(t, cb.CanAttempt(), "breaker should deny attempts after budget exhaustion")
	assert.Equal(t, budget+1, cb.Failures(), "breaker should report correct failure count")

	// Verify the typed CircuitOpenError reports state honestly.
	coe := cb.CircuitOpenErr()
	require.NotNil(t, coe, "CircuitOpenErr should return non-nil error")

	errText := coe.Error()
	assert.NotEmpty(t, errText, "circuit open error should have non-empty message")
	assert.Contains(t, errText, "test-lang", "error should mention the language")

	// Verify no misleading success phrases.
	for _, phrase := range successPhrases {
		assert.False(t, strings.Contains(errText, phrase),
			"circuit open error should not contain success phrase %q: %s", phrase, errText)
	}

	// Verify that RecordSuccess resets the breaker.
	cb.RecordSuccess()
	assert.True(t, cb.CanAttempt(), "breaker should allow attempts after RecordSuccess")
	assert.Equal(t, 0, cb.Failures(), "breaker should have 0 failures after RecordSuccess")
}
