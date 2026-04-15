package diag

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/mcp"
	gen "github.com/postfix/serena/protocol/gen"
)

// GetDiagnosticsArgs is the input schema for the get_diagnostics tool.
type GetDiagnosticsArgs struct {
	Path string `json:"path" jsonschema:"File path to check diagnostics for"`
}

// GetCodeActionsArgs is the input schema for the get_code_actions tool.
type GetCodeActionsArgs struct {
	Path    string `json:"path" jsonschema:"File path"`
	Line    int    `json:"line" jsonschema:"Line number (1-indexed)"`
	Col     int    `json:"column" jsonschema:"Column number (1-indexed)"`
	EndLine int    `json:"end_line,omitempty" jsonschema:"End line for range selection"`
	EndCol  int    `json:"end_column,omitempty" jsonschema:"End column for range selection"`
}

// FormatCodeArgs is the input schema for the format_code tool.
type FormatCodeArgs struct {
	Path      string `json:"path" jsonschema:"File path to format"`
	TabSize   int    `json:"tab_size,omitempty" jsonschema:"Tab size (default: 4)"`
	UseSpaces bool   `json:"use_spaces,omitempty" jsonschema:"Use spaces instead of tabs (default: true)"`
}

// LeaseProvider returns a worker lease for an LSP request against the given file URI.
// It abstracts the kernel/pool layer so tools don't depend on kernel directly.
type LeaseProvider func(ctx context.Context, uri string) (*lspool.WorkerLease, error)

// RegisterTools registers the 3 diagnostic MCP tools with the server.
// Each handler is wrapped with kernel.WrapToolSpan to produce kernel.tool.{name}
// sub-spans under the TelemetryMiddleware span (Phase 12, TRACE-03).
func RegisterTools(server *mcp.SerenaMCPServer, store *DiagnosticStore, workspaceRoot func() string, leaseFn LeaseProvider, tracer trace.Tracer) {
	registerGetDiagnostics(server, store, workspaceRoot, tracer)
	registerGetCodeActions(server, workspaceRoot, leaseFn, tracer)
	registerFormatCode(server, workspaceRoot, leaseFn, tracer)
}

func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}

func fileURI(root, path string) string {
	if strings.HasPrefix(path, "/") {
		return "file://" + path
	}
	return "file://" + root + "/" + path
}

func registerGetDiagnostics(server *mcp.SerenaMCPServer, store *DiagnosticStore, rootFn func() string, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_diagnostics",
		Description: "Returns current diagnostics (errors, warnings) for a file, formatted as severity:line:col: message",
	}, kernel.WrapToolSpan(tracer, "get_diagnostics", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetDiagnosticsArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return errorResult(serr.New(serr.NoWorkspace, "no active workspace").
				WithTool("get_diagnostics").Error()), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("get_diagnostics").Error()), nil, nil
		}

		uri := fileURI(root, args.Path)
		diags := store.GetDiagnostics(uri)
		if len(diags) == 0 {
			return textResult("no diagnostics for " + args.Path), nil, nil
		}

		var sb strings.Builder
		for _, d := range diags {
			sev := severityString(d.Severity)
			line := d.Range.Start.Line + 1
			col := d.Range.Start.Character + 1
			sb.WriteString(fmt.Sprintf("%s:%d:%d: %s\n", sev, line, col, d.Message))
		}
		return textResult(sb.String()), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{
		Name:        "get_diagnostics",
		Description: "Returns current diagnostics (errors, warnings) for a file",
	})
}

func registerGetCodeActions(server *mcp.SerenaMCPServer, rootFn func() string, leaseFn LeaseProvider, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_code_actions",
		Description: "Returns available code actions/quick fixes for a position or range in a file",
	}, kernel.WrapToolSpan(tracer, "get_code_actions", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetCodeActionsArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return errorResult(serr.New(serr.NoWorkspace, "no active workspace").
				WithTool("get_code_actions").Error()), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("get_code_actions").Error()), nil, nil
		}

		uri := fileURI(root, args.Path)
		lease, err := leaseFn(ctx, uri)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to get LS worker: %v", err)), nil, nil
		}

		// Convert from 1-indexed to 0-indexed.
		startLine := args.Line - 1
		startCol := args.Col - 1
		endLine := startLine
		endCol := startCol
		if args.EndLine > 0 {
			endLine = args.EndLine - 1
		}
		if args.EndCol > 0 {
			endCol = args.EndCol - 1
		}

		actions, err := GetCodeActions(ctx, lease, uri, startLine, startCol, endLine, endCol)
		if err != nil {
			return errorResult(fmt.Sprintf("code actions failed: %v", err)), nil, nil
		}

		if len(actions) == 0 {
			return textResult("no code actions available at this position"), nil, nil
		}

		var sb strings.Builder
		for i, a := range actions {
			pref := ""
			if a.IsPreferred {
				pref = " [preferred]"
			}
			sb.WriteString(fmt.Sprintf("%d. [%s] %s%s\n", i+1, a.Kind, a.Title, pref))
		}
		return textResult(sb.String()), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{
		Name:        "get_code_actions",
		Description: "Returns available code actions/quick fixes for a position or range",
	})
}

func registerFormatCode(server *mcp.SerenaMCPServer, rootFn func() string, leaseFn LeaseProvider, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "format_code",
		Description: "Formats a file via the language server and writes the result",
	}, kernel.WrapToolSpan(tracer, "format_code", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FormatCodeArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return errorResult(serr.New(serr.NoWorkspace, "no active workspace").
				WithTool("format_code").Error()), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("format_code").Error()), nil, nil
		}

		uri := fileURI(root, args.Path)
		lease, err := leaseFn(ctx, uri)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to get LS worker: %v", err)), nil, nil
		}

		tabSize := args.TabSize
		if tabSize <= 0 {
			tabSize = 4
		}

		opts := FormatOptions{
			TabSize:      tabSize,
			InsertSpaces: args.UseSpaces,
		}

		edits, err := FormatDocument(ctx, lease, uri, opts)
		if err != nil {
			return errorResult(fmt.Sprintf("formatting failed: %v", err)), nil, nil
		}

		if len(edits) == 0 {
			return textResult("file is already formatted: " + args.Path), nil, nil
		}

		// Resolve file path from root + relative path.
		filePath := args.Path
		if !strings.HasPrefix(filePath, "/") {
			filePath = root + "/" + filePath
		}

		if err := ApplyFormatEdits(filePath, edits); err != nil {
			return errorResult(fmt.Sprintf("applying format edits: %v", err)), nil, nil
		}

		return textResult(fmt.Sprintf("formatted %s (%d edits applied)", args.Path, len(edits))), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{
		Name:        "format_code",
		Description: "Formats a file via the language server and writes the result",
	})
}

func severityString(sev *gen.DiagnosticSeverity) string {
	if sev == nil {
		return "unknown"
	}
	switch *sev {
	case gen.DiagnosticSeverityError:
		return "error"
	case gen.DiagnosticSeverityWarning:
		return "warning"
	case gen.DiagnosticSeverityInformation:
		return "info"
	case gen.DiagnosticSeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}
