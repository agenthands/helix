package fileops

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/fuzzy"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/edit"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/workspace"
)

// ReadFileArgs is the input schema for the read_file tool.
type ReadFileArgs struct {
	Path      string `json:"path" jsonschema:"File path to read (relative to workspace root)"`
	StartLine int    `json:"start_line,omitempty" jsonschema:"Start line (1-indexed, optional)"`
	EndLine   int    `json:"end_line,omitempty" jsonschema:"End line (1-indexed, inclusive, optional)"`
}

// CreateFileArgs is the input schema for the create_file tool.
type CreateFileArgs struct {
	Path    string `json:"path" jsonschema:"File path to create (relative to workspace root)"`
	Content string `json:"content" jsonschema:"File content to write"`
}

// ListDirectoryArgs is the input schema for the list_directory tool.
type ListDirectoryArgs struct {
	Path string `json:"path" jsonschema:"Directory path (relative to workspace root)"`
}

// FindFilesArgs is the input schema for the find_files tool.
type FindFilesArgs struct {
	Pattern string `json:"pattern" jsonschema:"Glob pattern to match files (supports ** for recursive)"`
}

// SearchInFilesArgs is the input schema for the search_in_files tool.
type SearchInFilesArgs struct {
	Pattern      string `json:"pattern" jsonschema:"Regex pattern to search for"`
	IncludeGlob  string `json:"include_glob,omitempty" jsonschema:"Only search files matching this glob (optional)"`
	ExcludeGlob  string `json:"exclude_glob,omitempty" jsonschema:"Skip files matching this glob (optional)"`
	ContextLines int    `json:"context_lines,omitempty" jsonschema:"Number of context lines before/after match (optional)"`
	MaxResults   int    `json:"max_results,omitempty" jsonschema:"Maximum number of results (default 100, optional)"`
}

// ReplaceInFileArgs is the input schema for the replace_in_file tool.
type ReplaceInFileArgs struct {
	Path        string                 `json:"path" jsonschema:"File path (relative to workspace root)"`
	Pattern     string                 `json:"pattern" jsonschema:"Pattern to search for (literal or regex)"`
	Replacement string                 `json:"replacement" jsonschema:"Replacement string"`
	IsRegex     bool                   `json:"is_regex,omitempty" jsonschema:"Treat pattern as regex (default false)"`
	Receipts    []guardrails.ReceiptID `json:"receipts,omitempty" jsonschema:"Receipt IDs from prior find_references / analyze_blast_radius / get_context / get_repo_map / verify_edit calls. Required by guardrails (Phase 66 GUARD-03) for destructive operations on referenced or public-API symbols. Empty array when no receipts apply."`
}

// FuzzyEditArgs is the input schema for the fuzzy_edit tool.
type FuzzyEditArgs struct {
	Path            string                 `json:"path" jsonschema:"File path (relative to workspace root)"`
	Search          string                 `json:"search" jsonschema:"Text to search for (fuzzy matched with 4-strategy cascade: exact, whitespace-normalized, indentation-flexible)"`
	Replacement     string                 `json:"replacement" jsonschema:"Replacement text"`
	DisableEllipsis bool                   `json:"disable_ellipsis,omitempty" jsonschema:"Disable ... ellipsis segmentation (default false, meaning ellipsis is enabled)"`
	Receipts        []guardrails.ReceiptID `json:"receipts,omitempty" jsonschema:"Receipt IDs from prior find_references / analyze_blast_radius / get_context / get_repo_map / verify_edit calls. Required by guardrails (Phase 66 GUARD-03) for destructive operations on referenced or public-API symbols. Empty array when no receipts apply."`
}

// --- help text constants ---

const readFileHelp = `## Usage Examples

Read an entire file:
  read_file(path="src/main.go")

Read a specific line range:
  read_file(path="src/server.go", start_line=10, end_line=30)

## Common Patterns
- Use start_line/end_line to read only the relevant portion of large files
- Line numbers are 1-indexed and end_line is inclusive
- Combine with get_symbols_overview to find which lines to read`

const createFileHelp = `## Usage Examples

Create a new Go file:
  create_file(path="src/handlers/health.go", content="package handlers\n\nfunc HealthCheck() string {\n\treturn \"ok\"\n}\n")

Create a configuration file:
  create_file(path="config/defaults.yaml", content="server:\n  port: 8080\n")

## Common Patterns
- Errors if the file already exists (use replace_in_file or fuzzy_edit to modify existing files)
- Creates parent directories automatically
- Path is relative to workspace root`

const listDirectoryHelp = `## Usage Examples

List the project root:
  list_directory(path=".")

List a specific subdirectory:
  list_directory(path="src/handlers")

## Common Patterns
- Shows file type (FILE/DIR), size, modification time, and name
- Use to explore unfamiliar project structures
- Combine with find_files for recursive pattern matching`

const findFilesHelp = `## Usage Examples

Find all Go files recursively:
  find_files(pattern="**/*.go")

Find test files in a specific directory:
  find_files(pattern="src/**/*_test.go")

Find configuration files:
  find_files(pattern="*.yaml")

## Common Patterns
- Supports ** for recursive directory matching
- Returns file paths relative to workspace root
- Use to locate files before reading or editing them`

const searchInFilesHelp = `## Usage Examples

Search for a function name:
  search_in_files(pattern="func HandleRequest")

Search with context lines in specific files:
  search_in_files(pattern="TODO|FIXME", include_glob="*.go", context_lines=2)

Search excluding test files:
  search_in_files(pattern="db\\.Connect", exclude_glob="*_test.go", max_results=50)

## Common Patterns
- Pattern is a regex; escape special characters with backslash
- Use include_glob/exclude_glob to narrow the search scope
- Default max_results is 100; increase for broader searches`

const replaceInFileHelp = `## Usage Examples

Replace a string literal:
  replace_in_file(path="src/config.go", pattern="localhost:8080", replacement="0.0.0.0:9090")

Replace using regex:
  replace_in_file(path="src/api.go", pattern="v1\\.([a-z]+)", replacement="v2.$1", is_regex=true)

## Common Patterns
- Default is literal matching; set is_regex=true for regex patterns
- Replaces all occurrences in the file
- Falls back to fuzzy matching when literal match finds 0 hits`

const fuzzyEditHelp = `## Usage Examples

Replace a code block with fuzzy matching:
  fuzzy_edit(path="src/handler.go", search="if err != nil {\n  return err\n}", replacement="if err != nil {\n  return fmt.Errorf(\"handler: %w\", err)\n}")

Use ellipsis to skip middle content:
  fuzzy_edit(path="src/main.go", search="func main() {\n...\n  server.Start()\n}", replacement="func main() {\n...\n  server.StartTLS()\n}")

## Common Patterns
- Uses 4-strategy cascade: exact, whitespace-normalized, indentation-flexible, fuzzy
- Ellipsis (...) segments skip arbitrary content between anchors
- Reports match_strategy and similarity_score for transparency`

// RegisterTools registers all file operation tools with the MCP tool registry.
// The workspaceRoot function provides the active workspace root path.
// Each handler is wrapped with kernel.WrapToolSpan to produce kernel.tool.{name}
// sub-spans under the TelemetryMiddleware span (Phase 12, TRACE-03).
//
// Phase 60 D-03: the *kernel.Kernel and wsKeyFn parameters thread the
// EditNotifier accessor + active workspace key into the create_file,
// replace_in_file, and fuzzy_edit register* closures so they can fire
// the fire-and-forget OnEdit hook on the success path. Read-only and
// non-mutating tools (read_file, list_directory, find_files,
// search_in_files) ignore both new parameters.
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, workspaceRoot func() string, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	registerReadFile(server, workspaceRoot, tracer)
	registerCreateFile(server, k, workspaceRoot, wsKeyFn, tracer)
	registerListDirectory(server, workspaceRoot, tracer)
	registerFindFiles(server, workspaceRoot, tracer)
	registerSearchInFiles(server, workspaceRoot, tracer)
	registerReplaceInFile(server, k, workspaceRoot, wsKeyFn, tracer)
	registerFuzzyEdit(server, k, workspaceRoot, wsKeyFn, tracer)
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

func noWorkspaceError() *mcpsdk.CallToolResult {
	return errorResult(serr.New(serr.NoWorkspace, "no active workspace").Error())
}

func registerReadFile(server *mcp.SerenaMCPServer, rootFn func() string, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "read_file",
		Description: "Read a file's content, optionally a specific line range",
	}, kernel.WrapToolSpan(tracer, "read_file", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReadFileArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return noWorkspaceError(), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("read_file").Error()), nil, nil
		}

		if args.StartLine > 0 || args.EndLine > 0 {
			content, err := ReadFileRange(root, args.Path, args.StartLine, args.EndLine)
			if err != nil {
				return errorResult(err.Error()), nil, nil
			}
			return textResult(content), nil, nil
		}

		content, err := ReadFile(root, args.Path)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(content), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "read_file", Description: "Read a file's content, optionally a specific line range", BriefDescription: "Read the contents of a file", HelpText: readFileHelp})
}

func registerCreateFile(server *mcp.SerenaMCPServer, k *kernel.Kernel, rootFn func() string, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "create_file",
		Description: "Create a new file with content (errors if file already exists)",
	}, kernel.WrapToolSpan(tracer, "create_file", func(ctx context.Context, req *mcpsdk.CallToolRequest, args CreateFileArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return noWorkspaceError(), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("create_file").Error()), nil, nil
		}
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKeyFn())()

		if err := CreateFile(root, args.Path, args.Content); err != nil {
			return errorResult(err.Error()), nil, nil
		}
		// Phase 60 D-03: fire-and-forget signal to semantic live service.
		// Errors are swallowed; correctness via watcher + manifest scanner.
		if n := k.EditNotifier(); n != nil {
			_ = n.OnEdit(ctx, wsKeyFn(), []string{args.Path})
		}
		return textResult("created: " + args.Path), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "create_file", Description: "Create a new file with content (errors if file already exists)", BriefDescription: "Write content to a file, creating it if needed", HelpText: createFileHelp})
}

func registerListDirectory(server *mcp.SerenaMCPServer, rootFn func() string, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "list_directory",
		Description: "List directory contents with file type, size, and modification time",
	}, kernel.WrapToolSpan(tracer, "list_directory", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ListDirectoryArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return noWorkspaceError(), nil, nil
		}
		if args.Path == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("list_directory").Error()), nil, nil
		}

		entries, err := ListDirectory(root, args.Path)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		var sb strings.Builder
		for _, e := range entries {
			kind := "FILE"
			if e.IsDir {
				kind = "DIR "
			}
			sb.WriteString(fmt.Sprintf("%s  %8d  %s  %s\n", kind, e.Size, e.ModTime.Format("2006-01-02 15:04"), e.Name))
		}
		if sb.Len() == 0 {
			return textResult("(empty directory)"), nil, nil
		}
		return textResult(sb.String()), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "list_directory", Description: "List directory contents with file type, size, and modification time", BriefDescription: "List files and directories in a path", HelpText: listDirectoryHelp})
}

func registerFindFiles(server *mcp.SerenaMCPServer, rootFn func() string, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "find_files",
		Description: "Find files matching a glob pattern (supports ** for recursive matching)",
	}, kernel.WrapToolSpan(tracer, "find_files", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindFilesArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return noWorkspaceError(), nil, nil
		}
		if args.Pattern == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: pattern").
				WithTool("find_files").Error()), nil, nil
		}

		files, err := FindFiles(root, args.Pattern)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		if len(files) == 0 {
			return textResult("no files found matching: " + args.Pattern), nil, nil
		}
		return textResult(strings.Join(files, "\n")), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "find_files", Description: "Find files matching a glob pattern (supports ** for recursive matching)", BriefDescription: "Search for files by name pattern", HelpText: findFilesHelp})
}

func registerSearchInFiles(server *mcp.SerenaMCPServer, rootFn func() string, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "search_in_files",
		Description: "Search for a regex pattern across the codebase, with optional context lines",
	}, kernel.WrapToolSpan(tracer, "search_in_files", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SearchInFilesArgs) (*mcpsdk.CallToolResult, any, error) {
		root := rootFn()
		if root == "" {
			return noWorkspaceError(), nil, nil
		}
		if args.Pattern == "" {
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: pattern").
				WithTool("search_in_files").Error()), nil, nil
		}

		opts := SearchOpts{
			MaxResults:   args.MaxResults,
			ContextLines: args.ContextLines,
			IncludeGlob:  args.IncludeGlob,
			ExcludeGlob:  args.ExcludeGlob,
		}

		matches, err := SearchPattern(root, args.Pattern, opts)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		if len(matches) == 0 {
			return textResult("no matches found for: " + args.Pattern), nil, nil
		}

		var sb strings.Builder
		for _, m := range matches {
			sb.WriteString(fmt.Sprintf("%s:%d: %s\n", m.Path, m.Line, m.Text))
			for _, line := range m.ContextBefore {
				sb.WriteString(fmt.Sprintf("  - %s\n", line))
			}
			for _, line := range m.ContextAfter {
				sb.WriteString(fmt.Sprintf("  + %s\n", line))
			}
		}
		return textResult(sb.String()), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "search_in_files", Description: "Search for a regex pattern across the codebase, with optional context lines", BriefDescription: "Search file contents using regex patterns", HelpText: searchInFilesHelp})
}

func registerReplaceInFile(server *mcp.SerenaMCPServer, k *kernel.Kernel, rootFn func() string, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "replace_in_file",
		Description: "Replace all occurrences of a pattern in a file (literal or regex)",
	}, kernel.WrapToolSpan(tracer, "replace_in_file", func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceInFileArgs) (*mcpsdk.CallToolResult, any, error) {
		// Phase 53 D-16 emission. Strategy starts "none" (literal match path)
		// and is overwritten on the fuzzy fallback path with the real
		// fuzzy.Strategy value. Q-4: fuzzy.StrategyFailed cannot reach the
		// success path because Match returns ErrNoMatch on that branch.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "replace_in_file", outcome, strategy) }()

		root := rootFn()
		if root == "" {
			outcome = "internal"
			return noWorkspaceError(), nil, nil
		}
		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("replace_in_file").Error()), nil, nil
		}
		if args.Pattern == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: pattern").
				WithTool("replace_in_file").Error()), nil, nil
		}
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKeyFn())()

		count, err := ReplaceInFile(root, args.Path, args.Pattern, args.Replacement, args.IsRegex)
		if err != nil {
			outcome = edit.ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}

		// Fuzzy fallback: when literal match returns 0 hits and not regex (FUZZ-06).
		// Phase 76 ABLATE-07 (D-05): under the structured-edit ablation flag,
		// replace_in_file is exact-match only — skip the fuzzy cascade so a
		// no-match falls through to the plain "0 replacement(s) made" return.
		if count == 0 && !args.IsRegex && !k.StructuredEditDisabled() {
			content, readErr := ReadFile(root, args.Path)
			if readErr != nil {
				// File-read failure on the fuzzy-fallback path. WR-02 fix:
				// classify the outcome as "internal" so a real I/O error
				// (file vanished between the literal pass and this read,
				// permission flipped, transient EIO) does not report
				// outcome="success" on the helix_edit_outcome_total counter.
				// The textResult shape is preserved for caller backwards-compat;
				// only the metric classification changes.
				outcome = "internal"
				return textResult(fmt.Sprintf("0 replacement(s) made in %s", args.Path)), nil, nil
			}
			fResult, fErr := fuzzy.Match(content, args.Pattern, fuzzy.Options{
				Replacement:   args.Replacement,
				AllowEllipsis: false, // replace_in_file is literal-oriented
			})
			if fErr != nil {
				outcome = edit.ClassifyEditError(fErr)
				return errorResult(fErr.Error()), nil, nil
			}
			newContent := content[:fResult.StartByte] + fResult.ReplacementText + content[fResult.EndByte:]
			if wErr := OverwriteFile(root, args.Path, newContent); wErr != nil {
				outcome = "internal"
				return errorResult(wErr.Error()), nil, nil
			}
			// Q-4: fResult.Strategy is one of {exact, whitespace_normalized,
			// indentation_flexible} on the success branch. StrategyFailed
			// returns ErrNoMatch above before reaching here.
			strategy = string(fResult.Strategy)
			// Phase 60 D-03: fuzzy-fallback success path overwrote the file.
			// Fire-and-forget; errors swallowed; correctness via watcher.
			if n := k.EditNotifier(); n != nil {
				_ = n.OnEdit(ctx, wsKeyFn(), []string{args.Path})
			}
			text := fmt.Sprintf("1 replacement made in %s (fuzzy)\nmatch_strategy: %s\nsimilarity_score: %.2f",
				args.Path, fResult.Strategy, fResult.Score)
			return textResult(text), nil, nil
		}

		// Literal match success path: the literal substring matched, so
		// strategy is "exact" (Phase 53 D-11 semantics — exact byte match).
		if count > 0 {
			strategy = "exact"
			// Phase 60 D-03: literal-match success path mutated the file
			// (ReplaceInFile already wrote on disk). Fire-and-forget;
			// errors swallowed; correctness via watcher + manifest scan.
			// count == 0 (regex with no matches) does NOT emit — no write.
			if n := k.EditNotifier(); n != nil {
				_ = n.OnEdit(ctx, wsKeyFn(), []string{args.Path})
			}
		}
		return textResult(fmt.Sprintf("%d replacement(s) made in %s", count, args.Path)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "replace_in_file", Description: "Replace all occurrences of a pattern in a file (literal or regex)", BriefDescription: "Replace text in a file using exact string matching", HelpText: replaceInFileHelp})
}

func registerFuzzyEdit(server *mcp.SerenaMCPServer, k *kernel.Kernel, rootFn func() string, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "fuzzy_edit",
		Description: "Fuzzy-match and replace text in a file using 4-strategy cascade (exact, whitespace-normalized, indentation-flexible)",
	}, kernel.WrapToolSpan(tracer, "fuzzy_edit", func(ctx context.Context, req *mcpsdk.CallToolRequest, args FuzzyEditArgs) (*mcpsdk.CallToolResult, any, error) {
		// Phase 53 D-16 emission. fuzzy_edit ALWAYS runs fuzzy.Match, so the
		// strategy on the success path is always one of {exact,
		// whitespace_normalized, indentation_flexible}. Q-4: StrategyFailed
		// is filtered into outcome=no_match,strategy=none by the
		// fuzzy.ErrNoMatch path through ClassifyEditError BEFORE we touch
		// the strategy variable.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "fuzzy_edit", outcome, strategy) }()

		root := rootFn()
		if root == "" {
			outcome = "internal"
			return noWorkspaceError(), nil, nil
		}
		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("fuzzy_edit").Error()), nil, nil
		}
		if args.Search == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: search").
				WithTool("fuzzy_edit").Error()), nil, nil
		}
		// Phase 76 ABLATE-07 (D-04): fuzzy_edit is a structured-edit tool;
		// refuse with a typed Unsupported error under the ablation flag.
		if k.StructuredEditDisabled() {
			outcome = "unsupported"
			return errorResult(serr.New(serr.Unsupported,
				"subsystem_disabled: fuzzy_edit requires the structured-edit subsystem; use replace_in_file").
				WithTool("fuzzy_edit").Error()), nil, nil
		}
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKeyFn())()

		allowEllipsis := !args.DisableEllipsis
		result, err := FuzzyEdit(root, args.Path, args.Search, args.Replacement, allowEllipsis)
		if err != nil {
			// Q-4: ErrNoMatch / ErrAmbiguous classify to no_match /
			// ambiguous_match; strategy stays "none". Other errors
			// (file-I/O, validate-path) classify to "internal".
			outcome = edit.ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}

		// Success path: strategy is the matched cascade tier.
		strategy = string(result.Strategy)
		// Phase 60 D-03: FuzzyEdit() already overwrote the file on disk.
		// Fire-and-forget signal to semantic live service; errors swallowed;
		// correctness via watcher + manifest scanner.
		if n := k.EditNotifier(); n != nil {
			_ = n.OnEdit(ctx, wsKeyFn(), []string{args.Path})
		}

		text := fmt.Sprintf("Fuzzy edit applied to %s\nmatch_strategy: %s\nsimilarity_score: %.2f",
			args.Path, result.Strategy, result.Score)
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "fuzzy_edit", Description: "Fuzzy-match and replace text in a file using 4-strategy cascade (exact, whitespace-normalized, indentation-flexible)", BriefDescription: "Apply a fuzzy text edit using search/replace with context matching", HelpText: fuzzyEditHelp})
}
