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

// TestHandshake_InMemory validates the MCP initialize handshake over in-memory
// transport: correct capabilities, protocol version, and server info (PROTO-01, D-01).
func TestHandshake_InMemory(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	result := runner.Session.InitializeResult()
	require.NotNil(t, result, "InitializeResult must not be nil")
	require.NotEmpty(t, result.ProtocolVersion, "ProtocolVersion must not be empty")
	require.NotNil(t, result.ServerInfo, "ServerInfo must not be nil")
	require.Equal(t, "helix", result.ServerInfo.Name, "server name must be 'helix'")
	require.NotNil(t, result.Capabilities, "Capabilities must not be nil")
	require.NotNil(t, result.Capabilities.Tools, "Capabilities.Tools must not be nil")

	// One representative tool call to prove InMemory transport round-trip beyond handshake.
	harness.CallTool(t, runner.Session, "list_memories", map[string]any{})
}

// TestHandshake_GRPC validates the MCP initialize handshake over the retained
// gRPC/in-memory transport (the wire the CLI dials) and proves a full round-trip
// with a tool call (PROTO-01, D-01).
//
// Phase 94 RETIRE-02: this replaces the former TestHandshake_HTTP — the
// Streamable-HTTP /mcp head was deleted, so the second session now rides the
// same mcpServer.SDK() the gRPC StreamMCP wire funnels into.
func TestHandshake_GRPC(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	grpcSession := runner.NewGRPCSession(t)

	result := grpcSession.InitializeResult()
	require.NotNil(t, result, "gRPC InitializeResult must not be nil")
	require.NotEmpty(t, result.ProtocolVersion, "gRPC ProtocolVersion must not be empty")
	require.NotNil(t, result.ServerInfo, "gRPC ServerInfo must not be nil")
	require.Equal(t, "helix", result.ServerInfo.Name, "gRPC server name must be 'helix'")
	require.NotNil(t, result.Capabilities, "gRPC Capabilities must not be nil")
	require.NotNil(t, result.Capabilities.Tools, "gRPC Capabilities.Tools must not be nil")

	// One representative tool call to prove transport round-trip.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	toolResult, err := grpcSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_memories",
		Arguments: map[string]any{},
	})
	require.NoError(t, err, "list_memories gRPC call error")
	require.False(t, toolResult.IsError, "list_memories via gRPC returned error: %s", harness.TextContent(toolResult))
}

// TestHandshake_ProtocolVersionMatch verifies that the original in-memory session
// and a second session over the retained gRPC/in-memory transport return
// identical protocol version and server name from the same daemon.
func TestHandshake_ProtocolVersionMatch(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	inMemResult := runner.Session.InitializeResult()
	require.NotNil(t, inMemResult)

	grpcSession := runner.NewGRPCSession(t)
	grpcResult := grpcSession.InitializeResult()
	require.NotNil(t, grpcResult)

	require.Equal(t, inMemResult.ProtocolVersion, grpcResult.ProtocolVersion,
		"both sessions must return identical ProtocolVersion")
	require.Equal(t, inMemResult.ServerInfo.Name, grpcResult.ServerInfo.Name,
		"both sessions must return identical ServerInfo.Name")
}
