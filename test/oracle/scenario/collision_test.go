//go:build integration || llm || llmjudge

package scenario_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// waitForLSWithQuery polls search_symbols until the given query returns results.
// Used for fixtures where the default "main" query finds nothing.
func waitForLSWithQuery(t *testing.T, session *mcp.ClientSession, query string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr string
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("LS readiness timeout after %v for query %q (last: %s)", timeout, query, lastErr)
		case <-ticker.C:
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "search_symbols",
				Arguments: map[string]any{"query": query},
			})
			if err != nil {
				lastErr = fmt.Sprintf("call error: %v", err)
				continue
			}
			if result.IsError {
				lastErr = fmt.Sprintf("tool error: %s", harness.TextContent(result))
				continue
			}
			text := harness.TextContent(result)
			if len(result.Content) > 0 && text != "" && text != "(no results)" {
				t.Logf("LS ready for %q: %s", query, text[:min(len(text), 80)])
				return
			}
			lastErr = fmt.Sprintf("no results yet: %q", text)
		}
	}
}

// TestScenario_Collision_FullCycle (SCEN-03, T-20-03) exercises a full agent workflow
// against the name collision fixture where Config/Handler/Parse exist in Go, Python,
// and TypeScript. Verifies edits to one language don't affect others.
func TestScenario_Collision_FullCycle(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "collision")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true, // collision fixture has no "main" symbol; use custom wait
	})
	s := runner.Session
	waitForLSWithQuery(t, s, "Config", 30*time.Second)

	// Step 1 (search): find Config symbol.
	result := harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Config"})
	require.Contains(t, harness.TextContent(result), "Config")

	// Step 2 (read Go): read backend/config/main.go.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "backend/config/main.go"})
	goContent := harness.TextContent(result)
	require.Contains(t, goContent, "Config struct")

	// Step 3 (read Python): read worker/config/main.py.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "worker/config/main.py"})
	pyContent := harness.TextContent(result)
	require.Contains(t, pyContent, "class Config")

	// Step 4 (read TypeScript): read web/config/main.ts.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "web/config/main.ts"})
	tsContent := harness.TextContent(result)
	require.Contains(t, tsContent, "class Config")

	// Step 5 (edit Go): modify Go Config struct doc comment.
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "backend/config/main.go",
		"pattern":     "holds configuration for the backend service",
		"replacement": "holds updated configuration for the backend service",
	})

	// Step 6 (verify Go edit): confirm the edit.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "backend/config/main.go"})
	require.Contains(t, harness.TextContent(result), "updated configuration")

	// Step 7 (verify Python unchanged): Python file should be identical.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "worker/config/main.py"})
	require.Equal(t, pyContent, harness.TextContent(result), "Python file should not change when Go file is edited")

	// Step 8 (verify TypeScript unchanged): TypeScript file should be identical.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "web/config/main.ts"})
	require.Equal(t, tsContent, harness.TextContent(result), "TypeScript file should not change when Go file is edited")
}

// TestScenario_Collision_NoCrossContamination (SCEN-03, T-20-03) verifies that
// Go symbol queries for Config resolve only to Go files, not Python or TypeScript.
func TestScenario_Collision_NoCrossContamination(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "collision")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session
	waitForLSWithQuery(t, s, "Config", 30*time.Second)

	// go_to_definition on Config usage in Go (backend/config/main.go line 8, col 17 = "Config" in Handler param).
	result := harness.CallTool(t, s, "go_to_definition", map[string]any{
		"path":   "backend/config/main.go",
		"line":   8,
		"column": 17,
	})
	text := harness.TextContent(result)
	// Definition should be in the Go backend, not worker/ or web/.
	require.Contains(t, text, "backend/config/main.go", "Config definition should resolve to Go file")
	require.NotContains(t, text, "worker/", "Go definition must not resolve to Python worker")
	require.NotContains(t, text, "web/", "Go definition must not resolve to TypeScript web")

	// find_references for Config in Go (line 3, col 5 = "type Config struct").
	result = harness.CallTool(t, s, "find_references", map[string]any{
		"path":                "backend/config/main.go",
		"line":                3,
		"column":              5,
		"include_declaration": true,
	})
	text = harness.TextContent(result)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		require.NotContains(t, line, "worker/", "Go references must not include Python files")
		require.NotContains(t, line, "web/", "Go references must not include TypeScript files")
	}
}
