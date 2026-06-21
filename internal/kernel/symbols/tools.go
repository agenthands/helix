package symbols

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// --- Argument structs ---

// GoToDefinitionArgs is the input schema for the go_to_definition tool.
type GoToDefinitionArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col  int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
}

// FindReferencesArgs is the input schema for the find_references tool.
type FindReferencesArgs struct {
	Path        string `json:"path" jsonschema:"File path"`
	Line        int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col         int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
	IncludeDecl bool   `json:"include_declaration,omitempty" jsonschema:"Include the declaration itself in results"`
}

// SymbolOverviewArgs is the input schema for the get_symbol_overview tool.
type SymbolOverviewArgs struct {
	Path string `json:"path" jsonschema:"File path for symbol outline"`
}

// SearchSymbolsArgs is the input schema for the search_symbols tool.
type SearchSymbolsArgs struct {
	Query string `json:"query" jsonschema:"Symbol name or pattern to search"`
}

// HoverArgs is the input schema for the get_hover_info tool.
type HoverArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col  int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
}

// FindImplementationsArgs is the input schema for the find_implementations tool.
type FindImplementationsArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col  int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
}

// CallHierarchyArgs is the input schema for the get_call_hierarchy tool.
type CallHierarchyArgs struct {
	Path      string `json:"path" jsonschema:"File path"`
	Line      int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col       int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
	Direction string `json:"direction,omitempty" jsonschema:"incoming, outgoing, or both (default: both)"`
}

// TypeHierarchyArgs is the input schema for the get_type_hierarchy tool.
type TypeHierarchyArgs struct {
	Path      string `json:"path" jsonschema:"File path"`
	Line      int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col       int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
	Direction string `json:"direction,omitempty" jsonschema:"subtypes, supertypes, or both (default: both)"`
}

// BlastRadiusArgs is the input schema for the analyze_blast_radius tool.
type BlastRadiusArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (1-indexed, matches editor display)"`
	Col  int    `json:"column" jsonschema:"Column number (1-indexed, matches editor display)"`
}

// RegisterTools registers all 9 symbol retrieval tools with the MCP server.
// Each handler is wrapped with kernel.WrapToolSpan to produce kernel.tool.{name}
// sub-spans under the TelemetryMiddleware span (Phase 12, TRACE-03).
//
// Phase 65 65-06: lookupFn returns the daemon-wired integ.SemanticLookup
// (NoopLookup{} when semantic disabled). cfgGate carries the koanf-resolved
// SemanticIndex.Enabled flag. Both feed integ.ChooseSource(cfgGate, lookup,
// nil) at handler entry so the cfg-disabled → tree_sitter contract (D-04 +
// Pitfall §3) is honoured before any lookup call.
func RegisterTools(
	server *mcp.SerenaMCPServer,
	k *kernel.Kernel,
	wsKeyFn func() workspace.WorkspaceKey,
	lookupFn func() integ.SemanticLookup,
	cfgGate integ.ConfigGate,
) {
	tracer := k.Tracer()
	registerGoToDefinition(server, k, wsKeyFn, tracer)
	registerFindReferences(server, k, wsKeyFn, lookupFn, tracer)
	registerGetSymbolOverview(server, k, wsKeyFn, tracer)
	registerSearchSymbols(server, k, wsKeyFn, tracer)
	registerGetHoverInfo(server, k, wsKeyFn, tracer)
	registerFindImplementations(server, k, wsKeyFn, tracer)
	registerGetCallHierarchy(server, k, wsKeyFn, tracer)
	registerGetTypeHierarchy(server, k, wsKeyFn, tracer)
	registerAnalyzeBlastRadius(server, k, wsKeyFn, tracer, lookupFn, cfgGate)
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

func pathToURI(root, path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	if !strings.HasPrefix(path, "/") && root != "" {
		path = root + "/" + path
	}
	return "file://" + path
}

// userPosToLSP converts user-facing 1-indexed line/column to LSP 0-indexed
// values. Inputs ≤ 0 are clamped to LSP 0 to keep the call site valid even
// when callers (or tests) accidentally pass 0. The schema docstrings on each
// xxxArgs struct document the 1-indexed convention; the tool-result formatter
// in formatLocations converts back the same way (loc.Range.Start.Line + 1).
func userPosToLSP(line, col int) (int, int) {
	if line > 0 {
		line--
	} else {
		line = 0
	}
	if col > 0 {
		col--
	} else {
		col = 0
	}
	return line, col
}

func acquireLease(ctx context.Context, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey) (*kernel.WorkspaceRuntime, error) {
	// Phase 76 WR-02: runtime backstop for the no_lsp ablation arm. Every
	// LS-leasing symbol-retrieval handler routes through this shared chokepoint
	// before AcquireSession, so guarding here closes the direct-tool surface
	// (go_to_definition, find_references, hover, implementations, call/type
	// hierarchy, blast radius) under DisableLSPSubsystem with a single check —
	// mirroring the StructuredEditDisabled() backstop on the edit arm. If
	// reached via any back-channel tools/call while the flag is set, refuse
	// with a typed Unsupported error carrying the greppable subsystem_disabled:
	// prefix instead of leasing a live LS worker and emitting lspool.lsp.* spans.
	if k.LSPSubsystemDisabled() {
		return nil, serr.New(serr.Unsupported,
			"subsystem_disabled: this tool requires the LSP subsystem, which is disabled")
	}
	wsKey := wsKeyFn()
	rt, err := k.GetRuntime(wsKey)
	if err != nil {
		return nil, serr.Wrap(serr.NoWorkspace, "workspace not activated", err)
	}
	return rt, nil
}

// formatLocations formats SymbolLocation slice as "file:line:col" lines.
func formatLocations(locs []SymbolLocation) string {
	if len(locs) == 0 {
		return "(no results)"
	}
	var sb strings.Builder
	for _, loc := range locs {
		uri := loc.URI
		line := loc.Range.Start.Line + 1 // convert to 1-indexed for display
		col := loc.Range.Start.Character + 1
		if loc.Name != "" {
			sb.WriteString(fmt.Sprintf("%s:%d:%d — %s [%s]\n", uri, line, col, loc.Name, loc.Kind))
		} else if loc.Preview != "" {
			sb.WriteString(fmt.Sprintf("%s:%d:%d — %s\n", uri, line, col, loc.Preview))
		} else {
			sb.WriteString(fmt.Sprintf("%s:%d:%d\n", uri, line, col))
		}
	}
	return sb.String()
}

// formatOutline formats a SymbolOutline tree as indented text.
func formatOutline(outlines []SymbolOutline, indent int) string {
	var sb strings.Builder
	prefix := strings.Repeat("  ", indent)
	for _, o := range outlines {
		line := o.Range.Start.Line + 1
		sb.WriteString(fmt.Sprintf("%s%s %s (line %d)\n", prefix, o.Kind, o.Name, line))
		if len(o.Children) > 0 {
			sb.WriteString(formatOutline(o.Children, indent+1))
		}
	}
	return sb.String()
}

// formatHierarchy formats a HierarchyNode tree as indented text.
func formatHierarchy(nodes []HierarchyNode, indent int) string {
	var sb strings.Builder
	prefix := strings.Repeat("  ", indent)
	for _, n := range nodes {
		sb.WriteString(fmt.Sprintf("%s%s %s (%s)\n", prefix, n.Kind, n.Name, n.URI))
		if len(n.Children) > 0 {
			sb.WriteString(formatHierarchy(n.Children, indent+1))
		}
	}
	return sb.String()
}

// --- help text constants ---

const goToDefinitionHelp = `## Usage Examples

Jump to where a function is defined:
  go_to_definition(path="src/server.go", line=42, column=10)

Find the definition of a type used in an import:
  go_to_definition(path="src/handlers/auth.go", line=5, column=15)

## Common Patterns
- Use after hovering to navigate from usage to source
- Combine with find_references to understand how a symbol is used after finding its definition
- Works across files: follows imports and cross-package references`

const findReferencesHelp = `## Usage Examples

Find all usages of a function:
  find_references(path="src/auth/login.go", line=20, column=6)

Find references including the declaration itself:
  find_references(path="src/models/user.go", line=10, column=6, include_declaration=true)

## Common Patterns
- Use before renaming or deleting a symbol to understand impact
- Combine with go_to_definition to trace call chains
- Results show file:line:col for each reference location`

const getSymbolOverviewHelp = `## Usage Examples

List all symbols in a Go file:
  get_symbol_overview(path="src/server.go")

Get an outline of a Python module:
  get_symbol_overview(path="src/utils/helpers.py")

## Common Patterns
- Use to understand file structure before making edits
- Shows functions, classes, methods, variables with line numbers
- Nested symbols (methods inside classes) are shown indented`

const searchSymbolsHelp = `## Usage Examples

Find a function by name across the workspace:
  search_symbols(query="handleRequest")

Search for all types matching a pattern:
  search_symbols(query="UserService")

## Common Patterns
- Use to locate symbols when you know the name but not the file
- Returns file:line:col with symbol kind for each match
- Combine with get_symbol_overview to understand the file containing the result`

const getHoverInfoHelp = `## Usage Examples

Get type information for a variable:
  get_hover_info(path="src/main.go", line=15, column=8)

Check the signature of a function call:
  get_hover_info(path="src/handlers/api.go", line=30, column=12)

## Common Patterns
- Use to inspect types without navigating away from current file
- Shows function signatures, type definitions, and documentation
- Useful for understanding inferred types in dynamically typed languages`

const findImplementationsHelp = `## Usage Examples

Find all implementations of an interface:
  find_implementations(path="src/interfaces.go", line=10, column=6)

Find concrete implementations of an abstract method:
  find_implementations(path="src/base.py", line=25, column=8)

## Common Patterns
- Use to discover which types satisfy an interface
- Essential for understanding polymorphic code
- Combine with get_type_hierarchy for a broader view of inheritance`

const getCallHierarchyHelp = `## Usage Examples

Find all callers of a function:
  get_call_hierarchy(path="src/auth.go", line=15, column=6, direction="incoming")

Find all functions called by a method:
  get_call_hierarchy(path="src/service.go", line=30, column=6, direction="outgoing")

Find both callers and callees:
  get_call_hierarchy(path="src/handler.go", line=20, column=6)

## Common Patterns
- Use "incoming" to find who calls a function before modifying it
- Use "outgoing" to understand a function's dependencies
- Default direction is "both" which shows the full call graph`

const getTypeHierarchyHelp = `## Usage Examples

Get supertypes and subtypes of a class:
  get_type_hierarchy(path="src/models.py", line=10, column=6)

Find only subtypes:
  get_type_hierarchy(path="src/base.go", line=8, column=6, direction="subtypes")

## Common Patterns
- Use to understand inheritance chains and interface hierarchies
- Direction can be "subtypes", "supertypes", or "both" (default)
- Combine with find_implementations for a complete picture`

const analyzeBlastRadiusHelp = `## Usage Examples

Analyze impact of changing a function:
  analyze_blast_radius(path="src/core/engine.go", line=50, column=6)

Check how many files would be affected by modifying a type:
  analyze_blast_radius(path="src/models/user.go", line=12, column=6)

## Common Patterns
- Use before making changes to understand the scope of impact
- Shows direct references, callers, implementations, and affected files
- Higher total impact means more careful review is needed`

// --- tool registrations ---

func registerGoToDefinition(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "go_to_definition",
		Description: "Go to the definition of a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "go_to_definition", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("go_to_definition").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		locs, err := GoToDefinition(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "go_to_definition", Description: "Go to the definition of a symbol at a given position", BriefDescription: "Jump to where a symbol is defined", HelpText: goToDefinitionHelp})
}

func registerFindReferences(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, lookupFn func() integ.SemanticLookup, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "find_references",
		Description: "Find all references to a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "find_references", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindReferencesArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("find_references").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		locs, err := FindReferences(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol, args.IncludeDecl)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		// Phase 66 D-01 GUARD-03 / CR-04: resolve SymbolID via the wired
		// SemanticLookup so the receipt scope can satisfy STRICT validation
		// (validate.go:91-92) when a destructive tool runs against the same
		// symbol. On miss/unsupported the SymbolID falls back to empty —
		// the receipt is still issued (file-anchored validation paths still
		// match) but symbol-anchored consumers will skip it.
		var symID integ.SymbolID
		if lookupFn != nil {
			if lookup := lookupFn(); lookup != nil {
				if id, sErr := lookup.SymbolID(ctx, rt.Key(), args.Path, uint32(lspLine), uint32(lspCol)); sErr == nil {
					symID = id
				}
			}
		}
		guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassReferencesChecked,
			guardrails.ReferencesCheckedScope{
				SymbolID:     symID,
				RefCount:     len(locs),
				FilePath:     args.Path,
				IncludeTests: args.IncludeDecl,
			}, "find_references")
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "find_references", Description: "Find all references to a symbol at a given position", BriefDescription: "Find all references to a symbol", HelpText: findReferencesHelp})
}

func registerGetSymbolOverview(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_symbol_overview",
		Description: "Get a hierarchical outline of all symbols in a file",
	}, kernel.WrapToolSpan(tracer, "get_symbol_overview", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SymbolOverviewArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("get_symbol_overview").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		outlines, err := GetSymbolOverview(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path))
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if len(outlines) == 0 {
			return textResult("(no symbols found)"), nil, nil
		}
		return textResult(formatOutline(outlines, 0)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_symbol_overview", Description: "Get a hierarchical outline of all symbols in a file", BriefDescription: "List all symbols defined in a file with their types", HelpText: getSymbolOverviewHelp})
}

func registerSearchSymbols(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "search_symbols",
		Description: "Search for symbols across the workspace by name",
	}, kernel.WrapToolSpan(tracer, "search_symbols", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SearchSymbolsArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Query == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: query").
				WithTool("search_symbols").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		locs, err := SearchSymbols(ctx, lease, args.Query)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "search_symbols", Description: "Search for symbols across the workspace by name", BriefDescription: "Search for symbols by name across the workspace", HelpText: searchSymbolsHelp})
}

func registerGetHoverInfo(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_hover_info",
		Description: "Get hover/type information for a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "get_hover_info", func(ctx context.Context, req *mcpsdk.CallToolRequest, args HoverArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("get_hover_info").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		result, err := GetHover(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if result == nil {
			return textResult("(no hover information available)"), nil, nil
		}
		return textResult(result.Content), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_hover_info", Description: "Get hover/type information for a symbol at a given position", BriefDescription: "Get type information and documentation for a symbol", HelpText: getHoverInfoHelp})
}

func registerFindImplementations(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "find_implementations",
		Description: "Find all implementations of an interface or abstract method",
	}, kernel.WrapToolSpan(tracer, "find_implementations", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindImplementationsArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("find_implementations").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		locs, err := FindImplementations(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "find_implementations", Description: "Find all implementations of an interface or abstract method", BriefDescription: "Find all implementations of an interface or abstract method", HelpText: findImplementationsHelp})
}

func registerGetCallHierarchy(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_call_hierarchy",
		Description: "Get call hierarchy (callers and/or callees) for a symbol",
	}, kernel.WrapToolSpan(tracer, "get_call_hierarchy", func(ctx context.Context, req *mcpsdk.CallToolRequest, args CallHierarchyArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("get_call_hierarchy").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		direction := args.Direction
		if direction == "" {
			direction = "both"
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		nodes, err := GetCallHierarchy(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol, direction)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if len(nodes) == 0 {
			return textResult("(no call hierarchy available)"), nil, nil
		}
		return textResult(formatHierarchy(nodes, 0)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_call_hierarchy", Description: "Get call hierarchy (callers and/or callees) for a symbol", BriefDescription: "Find all callers or callees of a function or method", HelpText: getCallHierarchyHelp})
}

func registerGetTypeHierarchy(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_type_hierarchy",
		Description: "Get type hierarchy (subtypes and/or supertypes) for a symbol",
	}, kernel.WrapToolSpan(tracer, "get_type_hierarchy", func(ctx context.Context, req *mcpsdk.CallToolRequest, args TypeHierarchyArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("get_type_hierarchy").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		direction := args.Direction
		if direction == "" {
			direction = "both"
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		nodes, err := GetTypeHierarchy(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol, direction)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if len(nodes) == 0 {
			return textResult("(no type hierarchy available)"), nil, nil
		}
		return textResult(formatHierarchy(nodes, 0)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_type_hierarchy", Description: "Get type hierarchy (subtypes and/or supertypes) for a symbol", BriefDescription: "Get the type hierarchy (supertypes and subtypes)", HelpText: getTypeHierarchyHelp})
}

// registerAnalyzeBlastRadius wires the strangler-fig-aware analyze_blast_radius
// MCP handler. Phase 65 65-06.
//
// Source-selection at handler entry routes through integ.ChooseSource(cfgGate,
// lookup, nil) so the cfg-disabled → SourceTreeSitter contract (D-04 +
// Pitfall §3) is honoured BEFORE any semantic call. A bare
// `lookup == nil || !lookup.Available()` short-circuit would defeat that
// contract — the cfg gate MUST win.
//
// Three dispatch arms:
//
//   - SourceTreeSitter (cfg disabled): pure-LSP path via the existing
//     AnalyzeBlastRadius primitive (blast.go), confidence hard-capped at
//     fallbackConfidenceCap (D-08 + ROADMAP SC #2).
//   - SourceFallback (cfg enabled but lookup unavailable): same pure-LSP
//     path, same cap, but envelope reports source=fallback +
//     ChooseSource's classified reason (defensive D-05 row).
//   - SourceSemantic: two-pass orchestrator (analyzeBlastRadiusViaLookup).
//     A Pass 1 error re-classifies via ClassifyLookupErr and falls through
//     to the LSP fallback path with the cap.
func registerAnalyzeBlastRadius(
	server *mcp.SerenaMCPServer,
	k *kernel.Kernel,
	wsKeyFn func() workspace.WorkspaceKey,
	tracer trace.Tracer,
	lookupFn func() integ.SemanticLookup,
	cfgGate integ.ConfigGate,
) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "analyze_blast_radius",
		Description: "Analyze the blast radius (impact) of changing a symbol",
	}, kernel.WrapToolSpan(tracer, "analyze_blast_radius", func(ctx context.Context, req *mcpsdk.CallToolRequest, args BlastRadiusArgs) (*mcpsdk.CallToolResult, any, error) {
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("analyze_blast_radius").Error()), nil, nil
		}
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		lspLine, lspCol := userPosToLSP(args.Line, args.Col)
		uri := pathToURI(rt.Key().RepoRoot, args.Path)
		ws := rt.Key()

		// Resolve the wired lookup. nil is normalized to NoopLookup{} so the
		// priority-ladder gate at integ.ChooseSource always sees a valid
		// SemanticLookup interface value.
		var lookup integ.SemanticLookup
		if lookupFn != nil {
			lookup = lookupFn()
		}
		if lookup == nil {
			lookup = integ.NoopLookup{}
		}

		// D-04 + Pitfall §3: cfg gate is the FIRST decision. ChooseSource
		// returns SourceTreeSitter when the feature is off, SourceFallback +
		// FallbackReasonIndexDisabled when the feature is on but the lookup
		// is unavailable (defensive D-05 row), SourceSemantic otherwise.
		src, reason := integ.ChooseSource(cfgGate, lookup, nil)
		switch src {
		case integ.SourceTreeSitter, integ.SourceFallback:
			// Both non-semantic arms render the v1.9 LSP-derived BlastRadius
			// with the D-08 / ROADMAP SC #2 confidence cap applied uniformly.
			br, err := AnalyzeBlastRadius(ctx, lease, uri, lspLine, lspCol)
			if err != nil {
				return errorResult(err.Error()), nil, nil
			}
			capConfidences(br, fallbackConfidenceCap)
			out, marshErr := formatBlastRadiusEnvelope(br, src, reason)
			if marshErr != nil {
				return errorResult(serr.Wrap(serr.Internal, "marshal envelope", marshErr).Error()), nil, nil
			}
			// Phase 66 D-01 GUARD-03 / CR-04: best-effort SymbolID resolution
			// even on the LSP-only arm. When a non-Noop lookup is wired but
			// the cfg gate steered us to tree_sitter/fallback, lookup.SymbolID
			// can still translate (path,line,col) to the canonical SymbolID
			// so symbol-anchored receipt validation in destructive tools
			// succeeds. On miss, fall back to empty (file-anchored validation
			// paths still match).
			var lspArmSymID integ.SymbolID
			if lookup != nil {
				if id, sErr := lookup.SymbolID(ctx, ws, args.Path, uint32(lspLine), uint32(lspCol)); sErr == nil {
					lspArmSymID = id
				}
			}
			guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassImpactChecked,
				guardrails.ImpactCheckedScope{
					SymbolID:        lspArmSymID,
					RefCount:        br.TotalImpact,
					PublicAPI:       false,
					BlastNodes:      len(br.PerNode),
					MaxDepth:        0,
					IncludedCallers: len(br.Callers) > 0,
					IncludedTypes:   false,
				}, "analyze_blast_radius")
			return textResult(string(out)), nil, nil
		case integ.SourceSemantic:
			// fall through to the two-pass orchestrator below
		}

		// SourceSemantic arm: translate cursor → SymbolID, run two-pass.
		sym, sErr := lookup.SymbolID(ctx, ws, args.Path, uint32(lspLine), uint32(lspCol))
		if sErr != nil {
			// Pass 1-equivalent error — classify and fall back to LSP.
			fbReason := integ.ClassifyLookupErr(sErr)
			br, lspErr := AnalyzeBlastRadius(ctx, lease, uri, lspLine, lspCol)
			if lspErr != nil {
				return errorResult(lspErr.Error()), nil, nil
			}
			capConfidences(br, fallbackConfidenceCap)
			out, marshErr := formatBlastRadiusEnvelope(br, integ.SourceFallback, fbReason)
			if marshErr != nil {
				return errorResult(serr.Wrap(serr.Internal, "marshal envelope", marshErr).Error()), nil, nil
			}
			return textResult(string(out)), nil, nil
		}

		// Phase 65 65-12 Task 2: build the kernel-side lspProbeFn closure.
		// The closure captures the orchestrator-held lease + lookup.LocateSymbol
		// and feeds them to lspProbeForEdges, which performs the Pass-2 LSP
		// probe directly on the lease (daemon-side ValidateCriticalEdges is
		// a permanent passthrough — no LSP traffic).
		lspProbeFn := func(probeCtx context.Context, edges []integ.Edge) []integ.ValidatedEdge {
			locator := func(s integ.SymbolID) (string, uint32, uint32, bool) {
				p, l, c, ok, _ := lookup.LocateSymbol(probeCtx, ws, s)
				return p, l, c, ok
			}
			probe := func(pCtx context.Context, uri string, line, col int) ([]SymbolLocation, error) {
				return FindReferences(pCtx, lease, uri, line, col, false)
			}
			return lspProbeForEdges(probeCtx, edges, locator, probe, rt.Key().RepoRoot)
		}
		impacts, semSrc, semReason, semGraphVersion, expandErr := analyzeBlastRadiusViaLookup(ctx, lookup, ws, sym, lspProbeFn)
		if expandErr != nil {
			// Pass 1 error: drop to LSP fallback with the classified reason.
			br, lspErr := AnalyzeBlastRadius(ctx, lease, uri, lspLine, lspCol)
			if lspErr != nil {
				return errorResult(lspErr.Error()), nil, nil
			}
			capConfidences(br, fallbackConfidenceCap)
			out, marshErr := formatBlastRadiusEnvelope(br, semSrc, semReason)
			if marshErr != nil {
				return errorResult(serr.Wrap(serr.Internal, "marshal envelope", marshErr).Error()), nil, nil
			}
			return textResult(string(out)), nil, nil
		}
		out, marshErr := formatBlastRadiusEnvelopeFromImpacts(impacts, semSrc, semReason, semGraphVersion)
		if marshErr != nil {
			return errorResult(serr.Wrap(serr.Internal, "marshal envelope", marshErr).Error()), nil, nil
		}
		// Phase 66 D-01 GUARD-03: issue receipt on success path ONLY (semantic arm).
		guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassImpactChecked,
			guardrails.ImpactCheckedScope{
				SymbolID:        sym,
				RefCount:        len(impacts),
				PublicAPI:       false,
				BlastNodes:      len(impacts),
				MaxDepth:        0,
				IncludedCallers: true,
				IncludedTypes:   true,
			}, "analyze_blast_radius")
		return textResult(string(out)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "analyze_blast_radius", Description: "Analyze the blast radius (impact) of changing a symbol", BriefDescription: "Analyze the impact of changing a symbol", HelpText: analyzeBlastRadiusHelp})
}

// formatBlastRadius formats a BlastRadius as a human-readable summary.
func formatBlastRadius(br *BlastRadius) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Blast Radius: %d unique locations across %d files\n\n", br.TotalImpact, len(br.AffectedFiles)))

	sb.WriteString(fmt.Sprintf("Direct References (%d):\n", len(br.DirectRefs)))
	sb.WriteString(formatLocations(br.DirectRefs))

	if len(br.Callers) > 0 {
		sb.WriteString(fmt.Sprintf("\nCallers:\n"))
		sb.WriteString(formatHierarchy(br.Callers, 1))
	}

	if len(br.Implementations) > 0 {
		sb.WriteString(fmt.Sprintf("\nImplementations (%d):\n", len(br.Implementations)))
		sb.WriteString(formatLocations(br.Implementations))
	}

	if len(br.AffectedFiles) > 0 {
		sb.WriteString("\nAffected Files:\n")
		for _, f := range br.AffectedFiles {
			sb.WriteString(fmt.Sprintf("  %s\n", f))
		}
	}

	return sb.String()
}
