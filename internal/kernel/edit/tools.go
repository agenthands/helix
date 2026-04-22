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
	SearchBody string `json:"search_body,omitempty" jsonschema:"Optional: fuzzy-match this text within the symbol body before replacing. When absent, replaces the entire body."`
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

// --- help text constants ---

const replaceSymbolBodyHelp = `## Usage Examples

Replace a function body entirely:
  replace_symbol_body(path="src/auth.go", symbol_name="Login", new_body="{\n\treturn nil\n}")

Replace a specific part of a function using search_body:
  replace_symbol_body(path="src/handler.go", symbol_name="HandleRequest", search_body="if err != nil {\n\treturn err\n}", new_body="if err != nil {\n\tlog.Error(err)\n\treturn fmt.Errorf(\"handle: %w\", err)\n}")

## Common Patterns
- Use get_symbols_overview first to find the exact symbol name
- Use search_body to replace only a portion of a large function body
- Post-edit verification runs automatically and reports any compilation errors`

const insertBeforeSymbolHelp = `## Usage Examples

Add a comment before a function:
  insert_before_symbol(path="src/api.go", symbol_name="HandleAuth", content="// HandleAuth authenticates incoming requests.\n")

Add an import before a class definition:
  insert_before_symbol(path="src/models.py", symbol_name="User", content="from datetime import datetime\n\n")

## Common Patterns
- Use to add documentation, decorators, or preceding definitions
- Content is inserted on the line immediately before the symbol
- Combine with get_symbols_overview to verify symbol names`

const insertAfterSymbolHelp = `## Usage Examples

Add a new function after an existing one:
  insert_after_symbol(path="src/utils.go", symbol_name="ParseConfig", content="\nfunc ValidateConfig(cfg *Config) error {\n\treturn nil\n}\n")

Add a test helper after a test function:
  insert_after_symbol(path="src/auth_test.go", symbol_name="TestLogin", content="\nfunc TestLogout(t *testing.T) {\n}\n")

## Common Patterns
- Use to add related functions near existing code
- Content is inserted on the line immediately after the symbol
- Include leading newline for proper spacing between symbols`

const renameSymbolHelp = `## Usage Examples

Rename a function across the workspace:
  rename_symbol(path="src/auth.go", line=15, column=6, new_name="AuthenticateUser")

Rename a struct type:
  rename_symbol(path="src/models.go", line=8, column=6, new_name="UserProfile")

## Common Patterns
- Updates all references across all files in the workspace
- Line and column are 1-indexed (matching editor display)
- Use find_references first to preview what will change`

const safeDeleteSymbolHelp = `## Usage Examples

Delete an unused function:
  safe_delete_symbol(path="src/legacy.go", symbol_name="OldHandler")

Force-delete even if references exist:
  safe_delete_symbol(path="src/deprecated.go", symbol_name="DeprecatedFunc", force=true)

## Common Patterns
- Checks for references before deleting; reports count if blocked
- Use force=true only when you have already updated all callers
- Combine with find_references to review usage before deletion`

const verifyEditHelp = `## Usage Examples

Check for errors after editing a file:
  verify_edit(path="src/auth.go")

Verify a test file compiles:
  verify_edit(path="src/auth_test.go")

## Common Patterns
- Called automatically after replace_symbol_body and other edit tools
- Use manually to check compilation status of any file
- Returns "No errors found" or lists errors with line:col and message`

// --- tool registrations ---

func registerReplaceBody(server *mcp.SerenaMCPServer, k *kernel.Kernel, extractor *BodyExtractor, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "replace_symbol_body",
		Description: "Replace a symbol's body with new content using tree-sitter for precise extraction",
	}, kernel.WrapToolSpan(tracer, "replace_symbol_body", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceBodyArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
		if args.SymbolName == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
		if args.NewBody == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: new_body").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
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
		fuzzyInfo, err := ReplaceBodyWithPlan(ctx, dirtyLease, extractor, plan, lang, args.SearchBody)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Replaced body of %q in %s", args.SymbolName, args.Path)
		// D-07: include strategy/score when fuzzy was used; D-08: omit for full body replace.
		if fuzzyInfo != nil {
			text += fmt.Sprintf("\nmatch_strategy: %s\nsimilarity_score: %.2f", fuzzyInfo.Strategy, fuzzyInfo.Score)
		}
		text = appendVerifyInfo(ctx, diagStore, uri, text)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "replace_symbol_body", Description: "Replace a symbol's body with new content using tree-sitter for precise extraction", BriefDescription: "Replace the entire body of a function, method, or class", HelpText: replaceSymbolBodyHelp})
}

func registerInsertBefore(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "insert_before_symbol",
		Description: "Insert content immediately before a symbol",
	}, kernel.WrapToolSpan(tracer, "insert_before_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args InsertBeforeArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
		if args.SymbolName == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
		if args.Content == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: content").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
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
	server.Registry().Register(&mcp.ToolDef{Name: "insert_before_symbol", Description: "Insert content immediately before a symbol", BriefDescription: "Insert code before a symbol definition", HelpText: insertBeforeSymbolHelp})
}

func registerInsertAfter(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "insert_after_symbol",
		Description: "Insert content immediately after a symbol",
	}, kernel.WrapToolSpan(tracer, "insert_after_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args InsertAfterArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
		if args.SymbolName == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
		if args.Content == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: content").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
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
	server.Registry().Register(&mcp.ToolDef{Name: "insert_after_symbol", Description: "Insert content immediately after a symbol", BriefDescription: "Insert code after a symbol definition", HelpText: insertAfterSymbolHelp})
}

func registerRenameSymbol(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "rename_symbol",
		Description: "Rename a symbol across all files in the workspace",
	}, kernel.WrapToolSpan(tracer, "rename_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args RenameSymbolArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("rename_symbol").Error()), nil, nil
		}
		if args.NewName == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: new_name").
				WithTool("rename_symbol").Error()), nil, nil
		}
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
	server.Registry().Register(&mcp.ToolDef{Name: "rename_symbol", Description: "Rename a symbol across all files in the workspace", BriefDescription: "Rename a symbol across the entire workspace", HelpText: renameSymbolHelp})
}

func registerSafeDelete(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "safe_delete_symbol",
		Description: "Delete a symbol if it has no references; reports reference count if blocked",
	}, kernel.WrapToolSpan(tracer, "safe_delete_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SafeDeleteArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("safe_delete_symbol").Error()), nil, nil
		}
		if args.SymbolName == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("safe_delete_symbol").Error()), nil, nil
		}
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
	server.Registry().Register(&mcp.ToolDef{Name: "safe_delete_symbol", Description: "Delete a symbol if it has no references; reports reference count if blocked", BriefDescription: "Remove a symbol definition from a file", HelpText: safeDeleteSymbolHelp})
}

func registerVerifyEdit(server *mcp.SerenaMCPServer, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "verify_edit",
		Description: "Check for compilation errors after an edit; returns diagnostic summary",
	}, kernel.WrapToolSpan(tracer, "verify_edit", func(ctx context.Context, req *mcpsdk.CallToolRequest, args VerifyEditArgs) (*mcpsdk.CallToolResult, any, error) {
		wsKey := wsKeyFn()
		if wsKey.RepoRoot == "" {
			return errorResult(serr.New(serr.NoWorkspace, "no active workspace").
				WithTool("verify_edit").Error()), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("verify_edit").Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
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
	server.Registry().Register(&mcp.ToolDef{Name: "verify_edit", Description: "Check for compilation errors after an edit; returns diagnostic summary", BriefDescription: "Check for compilation errors after an edit", HelpText: verifyEditHelp})
}
