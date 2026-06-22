package help

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
)

// GetToolHelpArgs is the input schema for the get_tool_help tool.
type GetToolHelpArgs struct {
	ToolName string `json:"tool_name,omitempty" jsonschema:"Name of the tool to get help for"`
	Topic    string `json:"topic,omitempty" jsonschema:"Documentation topic name (alternative to tool_name): guardrails, dod, workflow:rename, workflow:delete, workflow:large-edit, workflow:security-sensitive-edit"`
}

// RegisterTools registers the get_tool_help MCP tool with the server.
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel) {
	tracer := k.Tracer()

	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_tool_help",
		Description: "Get comprehensive documentation for any Helix tool including parameters, types, and usage examples",
	}, kernel.WrapToolSpan(tracer, "get_tool_help", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetToolHelpArgs) (*mcpsdk.CallToolResult, any, error) {
		// Dispatch on Topic first (D-24 additive path).
		if args.Topic != "" {
			t, ok := defaultTopics.Get(args.Topic)
			if !ok {
				available := defaultTopics.Names()
				return errorResult(fmt.Sprintf(
					"Topic %q not found. Available topics: %s",
					args.Topic,
					strings.Join(available, ", "),
				)), nil, nil
			}
			// If both Topic and ToolName are set, prefer Topic with a note.
			content := t.Content
			if args.ToolName != "" {
				content = fmt.Sprintf("(Note: both 'topic' and 'tool_name' were provided; returning topic %q. Set only one to avoid ambiguity.)\n\n%s", args.Topic, content)
			}
			return textResult(content), nil, nil
		}

		// Existing tool_name dispatch path — preserved unchanged.
		if args.ToolName == "" {
			return errorResult("specify tool_name or topic — both are empty"), nil, nil
		}

		// Look up tool definition from registry (T-38-01: validate tool_name exists).
		def := server.Registry().Get(args.ToolName)
		if def == nil {
			// Provide available tool names hint.
			names := server.Registry().Names()
			sort.Strings(names)
			return errorResult(fmt.Sprintf(
				"Tool %q not found. Available tools: %s",
				args.ToolName,
				strings.Join(names, ", "),
			)), nil, nil
		}

		// Look up schema for parameter introspection.
		var params []ParamDoc
		for _, tool := range server.CollectToolSchemas() {
			if tool.Name == args.ToolName {
				params = ExtractParamDocs(tool.InputSchema)
				break
			}
		}

		helpOutput := FormatHelp(args.ToolName, def.Description, params, def.HelpText)
		return textResult(helpOutput), nil, nil
	}))

	server.Registry().Register(&mcp.ToolDef{
		Name:             "get_tool_help",
		Description:      "Get comprehensive documentation for any Helix tool including parameters, types, and usage examples",
		BriefDescription: "Get detailed help and usage examples for a tool",
		HelpText: `## Usage

Call get_tool_help with the name of any registered MCP tool to get:
- Full description
- Parameter documentation (name, type, required/optional, allowed values)
- Usage examples and patterns

Example: {"tool_name": "find_symbol"}`,
	})
}

// textResult returns a successful MCP tool result with the given text.
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

// errorResult returns an error MCP tool result with the given message.
func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
