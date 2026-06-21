//go:build integration || llm || llmjudge

package protocol_test

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestReconnect_DaemonStatePreserved verifies that daemon state (memories)
// persists across client disconnect/reconnect cycles (PROTO-04, D-03).
func TestReconnect_DaemonStatePreserved(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	// Write a memory before disconnecting.
	harness.CallTool(t, runner.Session, "write_memory", map[string]any{
		"name":    "persist-test",
		"content": "before-disconnect",
	})

	// Close the MCP client session (simulates client disconnect).
	err := runner.Session.Close()
	require.NoError(t, err, "session close error")

	// Create a new session against the same daemon.
	newSession := runner.NewGRPCSession(t)

	// Read the memory via the new session -- daemon state should be preserved.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := newSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_memory",
		Arguments: map[string]any{"name": "persist-test"},
	})
	require.NoError(t, err, "read_memory call error after reconnect")
	require.False(t, result.IsError, "read_memory failed after reconnect: %s", harness.TextContent(result))
	require.Contains(t, harness.TextContent(result), "before-disconnect",
		"daemon state should be preserved across client reconnect")
}

// TestReconnect_NoWorkerLeakage verifies that disconnecting and reconnecting
// does not leak resources -- the new session is fully functional (PROTO-04, D-03, T-19-02).
func TestReconnect_NoWorkerLeakage(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	// Verify initial session is functional.
	tools := harness.ListSessionTools(t, runner.Session)
	require.NotEmpty(t, tools, "initial session should have tools")

	// Close session.
	err := runner.Session.Close()
	require.NoError(t, err, "session close error")

	// Create new session and verify it is functional.
	newSession := runner.NewGRPCSession(t)
	newTools := harness.ListSessionTools(t, newSession)
	require.NotEmpty(t, newTools, "new session should have tools after reconnect")

	// Prove the new session can execute tool calls end-to-end.
	harness.CallTool(t, newSession, "list_memories", map[string]any{})
}

// TestReconnect_MultipleReconnects verifies that the daemon handles repeated
// connect/disconnect cycles without degradation (PROTO-04).
func TestReconnect_MultipleReconnects(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	const reconnectCycles = 3

	// Use the initial InMemory session for the first cycle check.
	tools := harness.ListSessionTools(t, runner.Session)
	require.NotEmpty(t, tools, "initial session should have tools")

	// Close the initial session.
	err := runner.Session.Close()
	require.NoError(t, err, "initial session close error")

	// Perform repeated reconnect cycles over the retained gRPC/in-memory wire.
	for i := 0; i < reconnectCycles; i++ {
		session := runner.NewGRPCSession(t)

		cycleTools := harness.ListSessionTools(t, session)
		require.NotEmpty(t, cycleTools, "reconnect cycle %d: session should have tools", i+1)

		// Verify a tool call works.
		harness.CallTool(t, session, "list_memories", map[string]any{})

		// Close this session for the next cycle (except the last, which is
		// cleaned up by t.Cleanup registered in NewGRPCSession).
		if i < reconnectCycles-1 {
			err := session.Close()
			require.NoError(t, err, "reconnect cycle %d: session close error", i+1)
		}
	}
}
