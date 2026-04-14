//go:build integration || llm || llmjudge

package scenario_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Java_FullCycle exercises a complete agent workflow against the Java fixture:
// activate -> search -> read -> edit -> verify read-back.
// jdtls requires Maven/Gradle for workspace/symbol indexing on simple projects,
// so the search step uses search_in_files (grep) instead of search_symbols.
func TestScenario_Java_FullCycle(t *testing.T) {
	harness.RequireLS(t, "jdtls")

	fixtureDir := harness.PrepareFixture(t, "java")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	// jdtls creates .jdtls-data that may not be fully released during cleanup.
	// Stop the runner explicitly and give jdtls time to shut down.
	t.Cleanup(func() {
		runner.Stop()
		time.Sleep(1 * time.Second)
		_ = os.RemoveAll(filepath.Join(fixtureDir, ".jdtls-data"))
	})
	s := runner.Session

	// Step 1 (search): find helper via file content search.
	// jdtls workspace/symbol needs Maven/Gradle for indexing; search_in_files works immediately.
	result := harness.CallTool(t, s, "search_in_files", map[string]any{
		"pattern": "helper",
	})
	text := harness.TextContent(result)
	require.Contains(t, text, "helper")

	// Step 2 (read): read Main.java containing helper.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "Main.java"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 3 (edit): change "hello" to "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "Main.java",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "Main.java"})
	readText := harness.TextContent(result)
	require.Contains(t, readText, `"hi"`)
	require.NotContains(t, readText, `return "hello"`)
}
