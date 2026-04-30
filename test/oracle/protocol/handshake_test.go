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
	require.Equal(t, "serena", result.ServerInfo.Name, "server name must be 'serena'")
	require.NotNil(t, result.Capabilities, "Capabilities must not be nil")
	require.NotNil(t, result.Capabilities.Tools, "Capabilities.Tools must not be nil")

	// One representative tool call to prove InMemory transport round-trip beyond handshake.
	harness.CallTool(t, runner.Session, "list_memories", map[string]any{})
}

// TestHandshake_HTTP validates the MCP initialize handshake over HTTP transport
// and proves a full HTTP round-trip with a tool call (PROTO-01, D-01).
func TestHandshake_HTTP(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	httpSession := runner.NewHTTPSession(t)

	result := httpSession.InitializeResult()
	require.NotNil(t, result, "HTTP InitializeResult must not be nil")
	require.NotEmpty(t, result.ProtocolVersion, "HTTP ProtocolVersion must not be empty")
	require.NotNil(t, result.ServerInfo, "HTTP ServerInfo must not be nil")
	require.Equal(t, "serena", result.ServerInfo.Name, "HTTP server name must be 'serena'")
	require.NotNil(t, result.Capabilities, "HTTP Capabilities must not be nil")
	require.NotNil(t, result.Capabilities.Tools, "HTTP Capabilities.Tools must not be nil")

	// One representative tool call via HTTP to prove transport round-trip.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	toolResult, err := httpSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_memories",
		Arguments: map[string]any{},
	})
	require.NoError(t, err, "list_memories HTTP call error")
	require.False(t, toolResult.IsError, "list_memories via HTTP returned error: %s", harness.TextContent(toolResult))
}

// TestHandshake_ProtocolVersionMatch verifies that InMemory and HTTP transports
// return identical protocol version and server name from the same daemon.
func TestHandshake_ProtocolVersionMatch(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	inMemResult := runner.Session.InitializeResult()
	require.NotNil(t, inMemResult)

	httpSession := runner.NewHTTPSession(t)
	httpResult := httpSession.InitializeResult()
	require.NotNil(t, httpResult)

	require.Equal(t, inMemResult.ProtocolVersion, httpResult.ProtocolVersion,
		"InMemory and HTTP sessions must return identical ProtocolVersion")
	require.Equal(t, inMemResult.ServerInfo.Name, httpResult.ServerInfo.Name,
		"InMemory and HTTP sessions must return identical ServerInfo.Name")
}
