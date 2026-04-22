package mcp

import (
	"context"
	"log/slog"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/workspace"
)

// ActivateCallback is called when a project is activated via the activate_project tool.
// It allows the daemon to wire kernel workspace activation alongside the registry.
type ActivateCallback func(ctx context.Context, repoPath string) error

// SkillToolExecutor is implemented by skills that support direct tool execution.
type SkillToolExecutor interface {
	ExecuteTool(name string, args map[string]interface{}) (string, error)
}

// SerenaMCPServer wraps the official MCP SDK server with Serena's tool registry,
// structured errors, and middleware (MCP-01, MCP-02, MCP-03, MCP-05, MCP-06).
type SerenaMCPServer struct {
	sdk              *mcpsdk.Server
	registry         *ToolRegistry
	logger           *slog.Logger
	activateCallback ActivateCallback
	toolSchemas      []*mcpsdk.Tool // stored for suggestion middleware schema introspection (D-07)
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

	// Middleware (TelemetryMiddleware for METRIC-02 + logging, and optional
	// ProfileFilterMiddleware) is installed by the daemon after the profile
	// and session wiring are resolved. The test-only constructor here leaves
	// the chain empty.

	// Register dummy tools
	s.registerPingTool()
	s.registerEchoTool()
	s.registerActivateProjectTool(workspaces)

	return s
}

// registerPingTool adds the ping diagnostic tool.
func (s *SerenaMCPServer) registerPingTool() {
	tool := &mcpsdk.Tool{
		Name:        "ping",
		Description: "Echo a message back (diagnostic tool)",
	}
	mcpsdk.AddTool(s.sdk, tool, func(ctx context.Context, req *mcpsdk.CallToolRequest, args PingArgs) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "pong: " + args.Message},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: "ping", Description: "Echo a message back (diagnostic tool)"})
	s.toolSchemas = append(s.toolSchemas, tool)
}

// registerEchoTool adds the echo diagnostic tool.
func (s *SerenaMCPServer) registerEchoTool() {
	tool := &mcpsdk.Tool{
		Name:        "echo",
		Description: "Echo arguments back as-is (diagnostic tool)",
	}
	mcpsdk.AddTool(s.sdk, tool, func(ctx context.Context, req *mcpsdk.CallToolRequest, args EchoArgs) (*mcpsdk.CallToolResult, any, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: args.Text},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: "echo", Description: "Echo arguments back as-is (diagnostic tool)"})
	s.toolSchemas = append(s.toolSchemas, tool)
}

// registerActivateProjectTool adds the activate_project tool (WRK-01).
func (s *SerenaMCPServer) registerActivateProjectTool(workspaces *workspace.Registry) {
	tool := &mcpsdk.Tool{
		Name:        "activate_project",
		Description: "Activate a workspace for a given repository path",
	}
	mcpsdk.AddTool(s.sdk, tool, func(ctx context.Context, req *mcpsdk.CallToolRequest, args ActivateProjectArgs) (*mcpsdk.CallToolResult, any, error) {
		key := workspace.WorkspaceKey{
			RepoRoot: args.RepoPath,
			Language: "generic",
		}
		ws, err := workspaces.ActivateWorkspace(key)
		if err != nil {
			activErr := serr.Wrap(serr.NoWorkspace, "workspace activation failed", err).
				WithDetail("check that the repository path exists and is accessible")
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: activErr.Error()},
				},
				IsError: true,
			}, nil, nil
		}
		// Notify callback (kernel workspace activation) if set.
		if s.activateCallback != nil {
			if err := s.activateCallback(ctx, args.RepoPath); err != nil {
				s.logger.Warn("activate callback failed", "error", err)
			}
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "workspace activated: " + ws.Key.RepoRoot + " (status: " + ws.Status + ")"},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: "activate_project", Description: "Activate a workspace for a given repository path"})
	s.toolSchemas = append(s.toolSchemas, tool)
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
	s.toolSchemas = append(s.toolSchemas, tool)
}

// RemoveTool removes a tool by name at runtime (MCP-07).
func (s *SerenaMCPServer) RemoveTool(name string) {
	s.sdk.RemoveTools(name)
	s.registry.Unregister(name)
}

// SetActivateCallback installs a callback invoked when activate_project succeeds.
// The daemon uses this to activate the kernel workspace alongside the registry.
func (s *SerenaMCPServer) SetActivateCallback(cb ActivateCallback) {
	s.activateCallback = cb
}

// AddSkillTool registers a skill-provided tool with a generic ExecuteTool handler.
// Uses the generic mcpsdk.AddTool so the SDK auto-generates an input schema.
func (s *SerenaMCPServer) AddSkillTool(name, description string, executor SkillToolExecutor) {
	toolName := name // capture for closure
	tool := &mcpsdk.Tool{
		Name:        toolName,
		Description: description,
	}
	mcpsdk.AddTool(s.sdk, tool, func(ctx context.Context, req *mcpsdk.CallToolRequest, args map[string]any) (*mcpsdk.CallToolResult, any, error) {
		result, err := executor.ExecuteTool(toolName, args)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: err.Error()},
				},
				IsError: true,
			}, nil, nil
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: result},
			},
		}, nil, nil
	})
	s.registry.Register(&ToolDef{Name: name, Description: description})
	s.toolSchemas = append(s.toolSchemas, tool)
}

// CollectToolSchemas returns all registered tool definitions with their InputSchema
// for suggestion middleware schema introspection (D-07). Called once at daemon
// startup after all tools are registered.
func (s *SerenaMCPServer) CollectToolSchemas() []*mcpsdk.Tool {
	return s.toolSchemas
}
