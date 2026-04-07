package mcp

import (
	"context"
	"log/slog"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/internal/workspace"
)

// SerenaMCPServer wraps the official MCP SDK server with Serena's tool registry,
// structured errors, and middleware (MCP-01, MCP-02, MCP-03, MCP-05, MCP-06).
type SerenaMCPServer struct {
	sdk      *mcpsdk.Server
	registry *ToolRegistry
	logger   *slog.Logger
}

// PingArgs is the input schema for the ping diagnostic tool.
type PingArgs struct {
	Message string `json:"message" jsonschema:"Message to echo back"`
}

// EchoArgs is the input schema for the echo diagnostic tool.
type EchoArgs struct {
	Text string `json:"text" jsonschema:"Text to echo back"`
}

// ActivateProjectArgs is the input schema for the activate_project tool (WRK-01).
type ActivateProjectArgs struct {
	RepoPath string `json:"repo_path" jsonschema:"Path to the repository root"`
}

// NewSerenaMCPServer creates a new MCP server with dummy tools registered.
func NewSerenaMCPServer(workspaces *workspace.Registry, logger *slog.Logger) *SerenaMCPServer {
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "serena", Version: "2.0.0-dev"},
		nil,
	)

	registry := NewToolRegistry(logger)

	s := &SerenaMCPServer{
		sdk:      server,
		registry: registry,
		logger:   logger,
	}

	// Install middleware for logging (MCP-04)
	InstallMiddleware(server, logger)

	// Register dummy tools
	s.registerPingTool()
	s.registerEchoTool()
	s.registerActivateProjectTool(workspaces)

	return s
}

// registerPingTool adds the ping diagnostic tool.
func (s *SerenaMCPServer) registerPingTool() {
	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "ping",
		Description: "Echo a message back (diagnostic tool)",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, args PingArgs) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "pong: " + args.Message},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: "ping", Description: "Echo a message back (diagnostic tool)"})
}

// registerEchoTool adds the echo diagnostic tool.
func (s *SerenaMCPServer) registerEchoTool() {
	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "echo",
		Description: "Echo arguments back as-is (diagnostic tool)",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, args EchoArgs) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: args.Text},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: "echo", Description: "Echo arguments back as-is (diagnostic tool)"})
}

// registerActivateProjectTool adds the activate_project tool (WRK-01).
func (s *SerenaMCPServer) registerActivateProjectTool(workspaces *workspace.Registry) {
	mcpsdk.AddTool(s.sdk, &mcpsdk.Tool{
		Name:        "activate_project",
		Description: "Activate a workspace for a given repository path",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, args ActivateProjectArgs) (*mcpsdk.CallToolResult, any, error) {
		key := workspace.WorkspaceKey{
			RepoRoot: args.RepoPath,
			Language: "generic",
		}
		ws, err := workspaces.ActivateWorkspace(key)
		if err != nil {
			detail := ErrorDetail{
				Code:       "WORKSPACE_ACTIVATION_FAILED",
				Cause:      err.Error(),
				Suggestion: "Check that the repository path exists and is accessible",
			}
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: detail.Cause},
				},
				IsError: true,
			}, nil, nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "workspace activated: " + ws.Key.RepoRoot + " (status: " + ws.Status + ")"},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: "activate_project", Description: "Activate a workspace for a given repository path"})
}

// SDK returns the underlying MCP SDK server for direct use (e.g., transport wiring).
func (s *SerenaMCPServer) SDK() *mcpsdk.Server {
	return s.sdk
}

// Registry returns the tool registry for dynamic tool management.
func (s *SerenaMCPServer) Registry() *ToolRegistry {
	return s.registry
}

// RunStdio runs the MCP server over stdio transport (MCP-01).
func (s *SerenaMCPServer) RunStdio(ctx context.Context) error {
	return s.sdk.Run(ctx, &mcpsdk.StdioTransport{})
}

// HTTPHandler returns an http.Handler for Streamable HTTP transport (MCP-02).
func (s *SerenaMCPServer) HTTPHandler() http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		return s.sdk
	}, nil)
}

// AddTool registers a new tool dynamically at runtime (MCP-07).
func (s *SerenaMCPServer) AddTool(tool *mcpsdk.Tool, handler mcpsdk.ToolHandler) {
	s.sdk.AddTool(tool, handler)
	s.registry.Register(&ToolDef{Name: tool.Name, Description: tool.Description})
}

// RemoveTool removes a tool by name at runtime (MCP-07).
func (s *SerenaMCPServer) RemoveTool(name string) {
	s.sdk.RemoveTools(name)
	s.registry.Unregister(name)
}
