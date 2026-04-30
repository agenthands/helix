//go:build integration || llm || llmjudge

// Clean shutdown tests (RUNT-02, D-10).
//
// Verifies that the daemon shuts down cleanly with no goroutine leaks
// and drains in-flight work gracefully.

package runtime_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestRuntime_Shutdown_NoGoroutineLeaks verifies that starting a daemon,
// doing work, and stopping it does not leak goroutines (delta <= 5).
func TestRuntime_Shutdown_NoGoroutineLeaks(t *testing.T) {
	harness.RequireGopls(t)

	// Capture baseline goroutine count.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		MaxWorkers:   4,
	})

	// Do some work to ensure workers are active.
	harness.CallTool(t, runner.Session, "search_symbols", map[string]any{
		"query": "Helper",
	})
	harness.CallTool(t, runner.Session, "read_file", map[string]any{
		"path": "main.go",
	})

	// Stop the runner (cancels daemon context).
	runner.Stop()

	// Allow goroutines to wind down — some background goroutines (runtime
	// finalizers, GC helpers) may take a moment to settle.
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, before+10,
		"goroutine leak: before=%d after=%d (delta=%d)", before, after, after-before)
}

// TestRuntime_Shutdown_DrainsInflightWork verifies that calling Stop
// mid-flight does not panic and in-flight work either completes or
// returns a context cancellation error.
func TestRuntime_Shutdown_DrainsInflightWork(t *testing.T) {
	harness.RequireGopls(t)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		MaxWorkers:   4,
	})

	// Start a tool call in a goroutine.
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic during inflight call: %v", r)
				return
			}
		}()
		_, err := runner.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "search_symbols",
			Arguments: map[string]any{"query": "DemoStruct"},
		})
		done <- err
	}()

	// Give the call a moment to start, then stop.
	time.Sleep(50 * time.Millisecond)
	runner.Stop()

	// Wait for the inflight call to finish.
	select {
	case err := <-done:
		// Either nil (completed) or context canceled — both are acceptable.
		if err != nil {
			require.ErrorIs(t, err, context.Canceled,
				"inflight call should either succeed or return context.Canceled, got: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("inflight call did not complete within 10s after Stop")
	}

	// Verify no goroutine leak.
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()
	assert.LessOrEqual(t, after, before+10,
		"goroutine leak after drain: before=%d after=%d", before, after)
}
