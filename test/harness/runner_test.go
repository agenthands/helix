//go:build integration || llm || llmjudge

package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestProjectRoot(t *testing.T) {
	root := ProjectRoot()
	require.DirExists(t, root, "ProjectRoot() should return an existing directory")

	goMod := filepath.Join(root, "go.mod")
	_, err := os.Stat(goMod)
	require.NoError(t, err, "go.mod should exist at project root")
}

func TestDefaultTestConfig(t *testing.T) {
	cfg := DefaultTestConfig(t)
	require.Equal(t, "full", cfg.Profile)
	require.Equal(t, 4, cfg.WorkerPool.MaxWorkers)
	require.Equal(t, 30, cfg.WorkerPool.BaseTTL)
	require.Equal(t, 2, cfg.Daemon.ShutdownTimeout)
}

func TestPrepareFixture(t *testing.T) {
	dir := PrepareFixture(t, "go")
	require.DirExists(t, dir, "PrepareFixture should create temp directory")

	mainGo := filepath.Join(dir, "main.go")
	_, err := os.Stat(mainGo)
	require.NoError(t, err, "go fixture should contain main.go")
}

func TestStartRunnerSmoke(t *testing.T) {
	runner := StartRunner(t, RunnerOptions{SkipLS: true})
	require.NotNil(t, runner.Session, "Runner.Session should not be nil")
	require.NotNil(t, runner.Daemon, "Runner.Daemon should not be nil")
	// Stop is called via t.Cleanup, but verify it doesn't panic when called explicitly.
	runner.Stop()
}

func TestCallToolSmoke(t *testing.T) {
	runner := StartRunner(t, RunnerOptions{SkipLS: true})
	tools := ListSessionTools(t, runner.Session)
	require.Greater(t, len(tools), 0, "should have registered tools")
}

func TestTextContent(t *testing.T) {
	t.Run("with text", func(t *testing.T) {
		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "hello"},
			},
		}
		require.Equal(t, "hello", TextContent(result))
	})

	t.Run("empty content", func(t *testing.T) {
		result := &mcp.CallToolResult{}
		require.Equal(t, "", TextContent(result))
	})
}

func TestGoldenStoreRoundTrip(t *testing.T) {
	gs := &GoldenStore{RootDir: t.TempDir()}

	// Write golden file via GOLDEN_UPDATE env var.
	t.Setenv("GOLDEN_UPDATE", "1")
	gs.AssertTools(t, "test-suite", []string{"b", "a", "c"})

	// Verify the golden file was created with sorted content.
	path := filepath.Join(gs.RootDir, "test-suite.tools.golden")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "golden file should exist after update")
	require.Equal(t, "a\nb\nc\n", string(data), "golden content should be sorted")

	// Read-only comparison (different order, same content after sort).
	t.Setenv("GOLDEN_UPDATE", "")
	gs.AssertTools(t, "test-suite", []string{"c", "a", "b"})
}
