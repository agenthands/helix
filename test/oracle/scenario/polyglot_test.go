//go:build integration || llm || llmjudge

package scenario_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestScenario_Polyglot_FullCycle exercises a complete agent workflow against the polyglot
// monorepo fixture: activate -> search -> read multiple languages -> edit Go file -> verify.
func TestScenario_Polyglot_FullCycle(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "polyglot")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (search): find the Go Helper symbol.
	result := harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Helper"})
	require.Contains(t, harness.TextContent(result), "Helper")

	// Step 2 (search): Greeter exists in both Go and TypeScript.
	result = harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Greeter"})
	require.Contains(t, harness.TextContent(result), "Greeter")

	// Step 3 (read): read Go main.go.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	require.Contains(t, harness.TextContent(result), "Helper")

	// Step 4 (read): read Python main.py.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.py"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 5 (edit): edit Go file.
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.go",
		"pattern":     "Helper function called",
		"replacement": "Helper function invoked",
	})

	// Step 6 (verify): confirm the edit.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	text := harness.TextContent(result)
	require.Contains(t, text, "invoked")
	require.NotContains(t, text, "Helper function called")
}

// TestScenario_Polyglot_NoCrossLanguageContamination (SCEN-03, T-20-03) verifies that
// Go symbol queries do not return Python or TypeScript results.
func TestScenario_Polyglot_NoCrossLanguageContamination(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "polyglot")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// go_to_definition on the Go Helper function call (main.go line 6, col 1 = "Helper()" call).
	result := harness.CallTool(t, s, "go_to_definition", map[string]any{
		"path":   "main.go",
		"line":   6,
		"column": 1,
	})
	text := harness.TextContent(result)
	// Definition should resolve to a .go file, not .py or .ts.
	require.Contains(t, text, ".go", "definition should be in a Go file")
	require.NotContains(t, text, ".py", "Go definition must not resolve to Python")
	require.NotContains(t, text, ".ts", "Go definition must not resolve to TypeScript")

	// find_references on Helper definition (main.go line 10, col 5 = "func Helper()").
	result = harness.CallTool(t, s, "find_references", map[string]any{
		"path":                "main.go",
		"line":                10,
		"column":              5,
		"include_declaration": true,
	})
	text = harness.TextContent(result)
	// All reference lines should be in .go files.
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		require.NotContains(t, line, ".py", "Go references must not include Python files")
		require.NotContains(t, line, ".ts", "Go references must not include TypeScript files")
	}
}

// TestScenario_Polyglot_FileOpsAcrossLanguages verifies that file-level tools see
// files from all languages in the polyglot workspace.
func TestScenario_Polyglot_FileOpsAcrossLanguages(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "polyglot")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// list_directory at root should see files from all three languages.
	result := harness.CallTool(t, s, "list_directory", map[string]any{"path": "."})
	text := harness.TextContent(result)
	require.Contains(t, text, ".go")
	require.Contains(t, text, ".py")
	require.Contains(t, text, ".ts")

	// search_in_files for "hello" should span multiple file types (Go prints "Hello, Go!",
	// Python helper returns "hello", TypeScript Greeter returns "Hello").
	result = harness.CallTool(t, s, "search_in_files", map[string]any{
		"pattern": "(?i)hello",
	})
	text = harness.TextContent(result)
	require.Contains(t, text, ".go")
	require.Contains(t, text, ".py")
}
