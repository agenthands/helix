package symbols

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/workspace"
)

// --- Argument structs ---

// GoToDefinitionArgs is the input schema for the go_to_definition tool.
type GoToDefinitionArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col  int    `json:"column" jsonschema:"Column number (0-indexed)"`
}

// FindReferencesArgs is the input schema for the find_references tool.
type FindReferencesArgs struct {
	Path        string `json:"path" jsonschema:"File path"`
	Line        int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col         int    `json:"column" jsonschema:"Column number (0-indexed)"`
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
	Line int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col  int    `json:"column" jsonschema:"Column number (0-indexed)"`
}

// FindImplementationsArgs is the input schema for the find_implementations tool.
type FindImplementationsArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col  int    `json:"column" jsonschema:"Column number (0-indexed)"`
}

// CallHierarchyArgs is the input schema for the get_call_hierarchy tool.
type CallHierarchyArgs struct {
	Path      string `json:"path" jsonschema:"File path"`
	Line      int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col       int    `json:"column" jsonschema:"Column number (0-indexed)"`
	Direction string `json:"direction,omitempty" jsonschema:"incoming, outgoing, or both (default: both)"`
}

// TypeHierarchyArgs is the input schema for the get_type_hierarchy tool.
type TypeHierarchyArgs struct {
	Path      string `json:"path" jsonschema:"File path"`
	Line      int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col       int    `json:"column" jsonschema:"Column number (0-indexed)"`
	Direction string `json:"direction,omitempty" jsonschema:"subtypes, supertypes, or both (default: both)"`
}

// BlastRadiusArgs is the input schema for the analyze_blast_radius tool.
type BlastRadiusArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col  int    `json:"column" jsonschema:"Column number (0-indexed)"`
}

// RegisterTools registers all 9 symbol retrieval tools with the MCP server.
// Each handler is wrapped with kernel.WrapToolSpan to produce kernel.tool.{name}
// sub-spans under the TelemetryMiddleware span (Phase 12, TRACE-03).
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey) {
	tracer := k.Tracer()
	registerGoToDefinition(server, k, wsKeyFn, tracer)
	registerFindReferences(server, k, wsKeyFn, tracer)
	registerGetSymbolOverview(server, k, wsKeyFn, tracer)
	registerSearchSymbols(server, k, wsKeyFn, tracer)
	registerGetHoverInfo(server, k, wsKeyFn, tracer)
	registerFindImplementations(server, k, wsKeyFn, tracer)
	registerGetCallHierarchy(server, k, wsKeyFn, tracer)
	registerGetTypeHierarchy(server, k, wsKeyFn, tracer)
	registerAnalyzeBlastRadius(server, k, wsKeyFn, tracer)
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

func acquireLease(ctx context.Context, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey) (*kernel.WorkspaceRuntime, error) {
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

// --- tool registrations ---

func registerGoToDefinition(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "go_to_definition",
		Description: "Go to the definition of a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "go_to_definition", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		locs, err := GoToDefinition(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "go_to_definition", Description: "Go to the definition of a symbol at a given position"})
}

func registerFindReferences(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "find_references",
		Description: "Find all references to a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "find_references", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindReferencesArgs) (*mcpsdk.CallToolResult, any, error) {
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		locs, err := FindReferences(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col, args.IncludeDecl)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "find_references", Description: "Find all references to a symbol at a given position"})
}

func registerGetSymbolOverview(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_symbol_overview",
		Description: "Get a hierarchical outline of all symbols in a file",
	}, kernel.WrapToolSpan(tracer, "get_symbol_overview", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SymbolOverviewArgs) (*mcpsdk.CallToolResult, any, error) {
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
	server.Registry().Register(&mcp.ToolDef{Name: "get_symbol_overview", Description: "Get a hierarchical outline of all symbols in a file"})
}

func registerSearchSymbols(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "search_symbols",
		Description: "Search for symbols across the workspace by name",
	}, kernel.WrapToolSpan(tracer, "search_symbols", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SearchSymbolsArgs) (*mcpsdk.CallToolResult, any, error) {
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
	server.Registry().Register(&mcp.ToolDef{Name: "search_symbols", Description: "Search for symbols across the workspace by name"})
}

func registerGetHoverInfo(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_hover_info",
		Description: "Get hover/type information for a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "get_hover_info", func(ctx context.Context, req *mcpsdk.CallToolRequest, args HoverArgs) (*mcpsdk.CallToolResult, any, error) {
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		result, err := GetHover(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if result == nil {
			return textResult("(no hover information available)"), nil, nil
		}
		return textResult(result.Content), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_hover_info", Description: "Get hover/type information for a symbol at a given position"})
}

func registerFindImplementations(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "find_implementations",
		Description: "Find all implementations of an interface or abstract method",
	}, kernel.WrapToolSpan(tracer, "find_implementations", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindImplementationsArgs) (*mcpsdk.CallToolResult, any, error) {
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		locs, err := FindImplementations(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "find_implementations", Description: "Find all implementations of an interface or abstract method"})
}

func registerGetCallHierarchy(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_call_hierarchy",
		Description: "Get call hierarchy (callers and/or callees) for a symbol",
	}, kernel.WrapToolSpan(tracer, "get_call_hierarchy", func(ctx context.Context, req *mcpsdk.CallToolRequest, args CallHierarchyArgs) (*mcpsdk.CallToolResult, any, error) {
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
		nodes, err := GetCallHierarchy(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col, direction)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if len(nodes) == 0 {
			return textResult("(no call hierarchy available)"), nil, nil
		}
		return textResult(formatHierarchy(nodes, 0)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_call_hierarchy", Description: "Get call hierarchy (callers and/or callees) for a symbol"})
}

func registerGetTypeHierarchy(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_type_hierarchy",
		Description: "Get type hierarchy (subtypes and/or supertypes) for a symbol",
	}, kernel.WrapToolSpan(tracer, "get_type_hierarchy", func(ctx context.Context, req *mcpsdk.CallToolRequest, args TypeHierarchyArgs) (*mcpsdk.CallToolResult, any, error) {
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
		nodes, err := GetTypeHierarchy(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col, direction)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		if len(nodes) == 0 {
			return textResult("(no type hierarchy available)"), nil, nil
		}
		return textResult(formatHierarchy(nodes, 0)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "get_type_hierarchy", Description: "Get type hierarchy (subtypes and/or supertypes) for a symbol"})
}

func registerAnalyzeBlastRadius(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "analyze_blast_radius",
		Description: "Analyze the blast radius (impact) of changing a symbol",
	}, kernel.WrapToolSpan(tracer, "analyze_blast_radius", func(ctx context.Context, req *mcpsdk.CallToolRequest, args BlastRadiusArgs) (*mcpsdk.CallToolResult, any, error) {
		rt, err := acquireLease(ctx, k, wsKeyFn)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", false)
		if err != nil {
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		br, err := AnalyzeBlastRadius(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), args.Line, args.Col)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(formatBlastRadius(br)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "analyze_blast_radius", Description: "Analyze the blast radius (impact) of changing a symbol"})
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
