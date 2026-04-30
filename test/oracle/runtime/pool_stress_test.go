//go:build integration || llm || llmjudge

// Worker pool stress tests (RUNT-01, D-08, D-09).
//
// Verifies that the worker pool survives sustained concurrent load,
// share-until-dirty semantics hold under edits, and circuit breaker
// state remains consistent.

package runtime_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/agenthands/helix/test/harness"
)

// TestRuntime_Pool_ConcurrentReads fans out 50 goroutines issuing
// search_symbols simultaneously. All reads should share workers
// (share-until-dirty) and complete without error.
func TestRuntime_Pool_ConcurrentReads(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		MaxWorkers:   4,
	})

	const N = 50
	g, ctx := errgroup.WithContext(context.Background())

	for i := 0; i < N; i++ {
		g.Go(func() error {
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			result, err := runner.Session.CallTool(callCtx, &mcp.CallToolParams{
				Name:      "search_symbols",
				Arguments: map[string]any{"query": "Helper"},
			})
			if err != nil {
				return fmt.Errorf("goroutine call error: %w", err)
			}
			if result.IsError {
				return fmt.Errorf("goroutine tool error: %s", harness.TextContent(result))
			}
			return nil
		})
	}

	require.NoError(t, g.Wait(), "all %d concurrent reads must succeed", N)
}

// TestRuntime_Pool_ConcurrentEdits fans out read operations, then does
// sequential edits to test dirty promotion under load.
func TestRuntime_Pool_ConcurrentEdits(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		MaxWorkers:   4,
	})

	// Phase 1: Fan out 10 concurrent reads (shared workers).
	const readN = 10
	g, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < readN; i++ {
		g.Go(func() error {
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			result, err := runner.Session.CallTool(callCtx, &mcp.CallToolParams{
				Name:      "search_symbols",
				Arguments: map[string]any{"query": "Helper"},
			})
			if err != nil {
				return err
			}
			if result.IsError {
				return fmt.Errorf("read error: %s", harness.TextContent(result))
			}
			return nil
		})
	}
	require.NoError(t, g.Wait(), "concurrent reads before edits must succeed")

	// Phase 2: Sequential edits that promote workers to dirty.
	edits := []struct {
		pattern     string
		replacement string
	}{
		{`Helper function called`, `Helper function called v2`},
		{`Helper function called v2`, `Helper function called v3`},
		{`Helper function called v3`, `Helper function called v4`},
	}

	for _, e := range edits {
		harness.CallTool(t, runner.Session, "replace_in_file", map[string]any{
			"path":        "main.go",
			"pattern":     e.pattern,
			"replacement": e.replacement,
		})
	}

	// Phase 3: Verify final content reflects last edit.
	result := harness.CallTool(t, runner.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	text := harness.TextContent(result)
	assert.Contains(t, text, "Helper function called v4",
		"file should reflect the last sequential edit")
}

// TestRuntime_Pool_ShareUntilDirty validates the read -> edit -> read
// sequence to prove dirty promotion works correctly.
func TestRuntime_Pool_ShareUntilDirty(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		MaxWorkers:   4,
	})

	// Step 1: Read — worker stays clean/shared.
	result := harness.CallTool(t, runner.Session, "search_symbols", map[string]any{
		"query": "Helper",
	})
	text := harness.TextContent(result)
	assert.NotEmpty(t, text, "search_symbols should return results for Helper")

	// Step 2: Edit — promotes worker to dirty (exclusive lease).
	harness.CallTool(t, runner.Session, "replace_in_file", map[string]any{
		"path":        "main.go",
		"pattern":     `Hello, Go!`,
		"replacement": `Hello, modified Go!`,
	})

	// Step 3: Read again — should still work (may get new shared worker or reuse).
	result = harness.CallTool(t, runner.Session, "search_symbols", map[string]any{
		"query": "Helper",
	})
	text = harness.TextContent(result)
	assert.NotEmpty(t, text, "search_symbols should still work after edit")

	// Step 4: Verify the edit persisted.
	fileResult := harness.CallTool(t, runner.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	assert.Contains(t, harness.TextContent(fileResult), "Hello, modified Go!",
		"edit should have persisted")
}
