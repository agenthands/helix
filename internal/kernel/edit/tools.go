package edit

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/kernel/diag"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/workspace"
)

// --- Argument structs ---

// ReplaceBodyArgs is the input schema for the replace_symbol_body tool.
type ReplaceBodyArgs struct {
	Path       string `json:"path" jsonschema:"File path"`
	SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol whose body to replace"`
	NewBody    string `json:"new_body" jsonschema:"New body content to replace with"`
}

// InsertBeforeArgs is the input schema for the insert_before_symbol tool.
type InsertBeforeArgs struct {
	Path       string `json:"path" jsonschema:"File path"`
	SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol to insert before"`
	Content    string `json:"content" jsonschema:"Content to insert"`
}

// InsertAfterArgs is the input schema for the insert_after_symbol tool.
type InsertAfterArgs struct {
	Path       string `json:"path" jsonschema:"File path"`
	SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol to insert after"`
	Content    string `json:"content" jsonschema:"Content to insert"`
}

// RenameSymbolArgs is the input schema for the rename_symbol tool.
type RenameSymbolArgs struct {
	Path    string `json:"path" jsonschema:"File path where symbol is defined"`
	Line    int    `json:"line" jsonschema:"Line number of symbol (1-indexed)"`
	Col     int    `json:"column" jsonschema:"Column number of symbol (1-indexed)"`
	NewName string `json:"new_name" jsonschema:"New name for the symbol"`
}

// SafeDeleteArgs is the input schema for the safe_delete_symbol tool.
type SafeDeleteArgs struct {
	Path       string `json:"path" jsonschema:"File path"`
	SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol to delete"`
	Force      bool   `json:"force,omitempty" jsonschema:"Delete even if references exist (default: false)"`
}

// VerifyEditArgs is the input schema for the verify_edit tool.
type VerifyEditArgs struct {
	Path string `json:"path" jsonschema:"File path to verify after editing"`
}

// RegisterTools registers all 6 symbol editing tools with the MCP server.
// Each handler is wrapped with kernel.WrapToolSpan to produce kernel.tool.{name}
// sub-spans under the TelemetryMiddleware span (Phase 12, TRACE-03).
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, extractor *BodyExtractor, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey) {
	tracer := k.Tracer()
	registerReplaceBody(server, k, extractor, diagStore, wsKeyFn, tracer)
	registerInsertBefore(server, k, diagStore, wsKeyFn, tracer)
	registerInsertAfter(server, k, diagStore, wsKeyFn, tracer)
	registerRenameSymbol(server, k, diagStore, wsKeyFn, tracer)
	registerSafeDelete(server, k, diagStore, wsKeyFn, tracer)
	registerVerifyEdit(server, diagStore, wsKeyFn, tracer)
}

// --- helpers ---

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

func filePathToURI(root, path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	if !strings.HasPrefix(path, "/") && root != "" {
		path = root + "/" + path
	}
	return "file://" + path
}

// detectLang guesses the language from file extension.
func detectLang(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".py"):
		return "python"
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"),
		strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"):
		return "typescript"
	case strings.HasSuffix(path, ".rs"):
		return "rust"
	default:
		return ""
	}
}

// appendVerifyInfo runs VerifyEdit and appends results to the text.
func appendVerifyInfo(ctx context.Context, diagStore *diag.DiagnosticStore, uri string, text string) string {
	vr, err := VerifyEdit(ctx, diagStore, uri)
	if err != nil {
		return text + "\n\nVerification: (error: " + err.Error() + ")"
	}
	if vr.HasErrors {
		var sb strings.Builder
		sb.WriteString(text)
		sb.WriteString(fmt.Sprintf("\n\nPost-edit verification: %d error(s)", vr.ErrorCount))
		for _, e := range vr.Errors {
			sb.WriteString(fmt.Sprintf("\n  L%d:%d [%s] %s", e.Line, e.Col, e.Source, e.Message))
		}
		return sb.String()
	}
	return text + "\n\nPost-edit verification: OK (no errors)"
}

// --- tool registrations ---

func registerReplaceBody(server *mcp.SerenaMCPServer, k *kernel.Kernel, extractor *BodyExtractor, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "replace_symbol_body",
		Description: "Replace a symbol's body with new content using tree-sitter for precise extraction",
	}, kernel.WrapToolSpan(tracer, "replace_symbol_body", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceBodyArgs) (*mcpsdk.CallToolResult, any, error) {
		wsKey := wsKeyFn()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		lang := detectLang(args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeReplaceBody, args.NewBody)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: execute mutation on dirty lease using the plan's range.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		if err := ReplaceBodyWithPlan(ctx, dirtyLease, extractor, plan, lang); err != nil {
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Replaced body of %q in %s", args.SymbolName, args.Path)
		text = appendVerifyInfo(ctx, diagStore, uri, text)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "replace_symbol_body", Description: "Replace a symbol's body with new content using tree-sitter for precise extraction"})
}

func registerInsertBefore(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "insert_before_symbol",
		Description: "Insert content immediately before a symbol",
	}, kernel.WrapToolSpan(tracer, "insert_before_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args InsertBeforeArgs) (*mcpsdk.CallToolResult, any, error) {
		wsKey := wsKeyFn()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeInsertBefore, args.Content)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: execute mutation on dirty lease.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		if err := InsertBeforeWithPlan(ctx, dirtyLease, plan); err != nil {
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Inserted content before %q in %s", args.SymbolName, args.Path)
		text = appendVerifyInfo(ctx, diagStore, uri, text)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "insert_before_symbol", Description: "Insert content immediately before a symbol"})
}

func registerInsertAfter(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "insert_after_symbol",
		Description: "Insert content immediately after a symbol",
	}, kernel.WrapToolSpan(tracer, "insert_after_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args InsertAfterArgs) (*mcpsdk.CallToolResult, any, error) {
		wsKey := wsKeyFn()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeInsertAfter, args.Content)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: execute mutation on dirty lease.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		if err := InsertAfterWithPlan(ctx, dirtyLease, plan); err != nil {
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Inserted content after %q in %s", args.SymbolName, args.Path)
		text = appendVerifyInfo(ctx, diagStore, uri, text)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "insert_after_symbol", Description: "Insert content immediately after a symbol"})
}

func registerRenameSymbol(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "rename_symbol",
		Description: "Rename a symbol across all files in the workspace",
	}, kernel.WrapToolSpan(tracer, "rename_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args RenameSymbolArgs) (*mcpsdk.CallToolResult, any, error) {
		wsKey := wsKeyFn()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", true) // dirty=true
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Convert from 1-indexed (user-facing) to 0-indexed (LSP).
		result, err := RenameSymbol(ctx, lease, uri, args.Line-1, args.Col-1, args.NewName)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Renamed to %q: %d files changed, %d edits applied\nFiles: %s",
			args.NewName, result.FilesChanged, result.EditsApplied, strings.Join(result.Files, ", "))
		text = appendVerifyInfo(ctx, diagStore, uri, text)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "rename_symbol", Description: "Rename a symbol across all files in the workspace"})
}

func registerSafeDelete(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "safe_delete_symbol",
		Description: "Delete a symbol if it has no references; reports reference count if blocked",
	}, kernel.WrapToolSpan(tracer, "safe_delete_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SafeDeleteArgs) (*mcpsdk.CallToolResult, any, error) {
		wsKey := wsKeyFn()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges and references.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeDelete, "")
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: check references on clean lease, execute on dirty lease.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		result, err := SafeDeleteWithPlan(ctx, cleanLease, dirtyLease, plan, args.Force)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if !result.Deleted {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Cannot delete %q: %d reference(s) found\n", args.SymbolName, result.References))
			for _, ref := range result.RefLocations {
				sb.WriteString(fmt.Sprintf("  %s:%d:%d\n", ref.URI,
					ref.Range.Start.Line+1, ref.Range.Start.Character+1))
			}
			sb.WriteString("Use force=true to delete anyway.")
			return textResult(sb.String()), nil, nil
		}
		text := fmt.Sprintf("Deleted %q from %s", args.SymbolName, args.Path)
		text = appendVerifyInfo(ctx, diagStore, uri, text)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "safe_delete_symbol", Description: "Delete a symbol if it has no references; reports reference count if blocked"})
}

func registerVerifyEdit(server *mcp.SerenaMCPServer, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "verify_edit",
		Description: "Check for compilation errors after an edit; returns diagnostic summary",
	}, kernel.WrapToolSpan(tracer, "verify_edit", func(ctx context.Context, req *mcpsdk.CallToolRequest, args VerifyEditArgs) (*mcpsdk.CallToolResult, any, error) {
		uri := filePathToURI(wsKeyFn().RepoRoot, args.Path)
		result, err := VerifyEdit(ctx, diagStore, uri)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if !result.HasErrors {
			return textResult("No errors found."), nil, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%d error(s) found:\n", result.ErrorCount))
		for _, e := range result.Errors {
			sb.WriteString(fmt.Sprintf("  L%d:%d [%s] %s\n", e.Line, e.Col, e.Source, e.Message))
		}
		return textResult(sb.String()), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "verify_edit", Description: "Check for compilation errors after an edit; returns diagnostic summary"})
}
