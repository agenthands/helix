package edit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/fuzzy"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/diag"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/workspace"
)

// ClassifyEditError maps an edit/fileops handler error to one of the closed
// helix_edit_outcome_total "outcome" values (Phase 53 D-10).
//
// Source-of-truth bucketing (RESEARCH §"Edit-Tool Outcome Map"):
//
//   - fuzzy.ErrAmbiguous  → "ambiguous_match"
//   - fuzzy.ErrNoMatch    → "no_match"
//   - serr.NoWorkspace    → "internal" (no_workspace is a setup error, not
//     an edit-domain bucket)
//   - all other errors    → "internal"
//
// Q-3 (RESOLVED 2026-04-30): missing-required-field errors and other
// serr.InvalidArgs results that are NOT fuzzy ambiguity / no-match map to
// "internal" — preserves the locked D-10 6-value enum; classified as a
// known under-classification scheduled for v1.3 typed-error work.
//
// "ls_error" classification is BEST-EFFORT in v1.2: without typed
// "LS-failure" sentinels in internal/kernel/edit, every LS adapter
// failure currently flows as serr.Internal-wrapped errors that look
// identical to file-I/O errors. The bucket exists in the closed enum
// per D-10, but in v1.2 it is only emitted via direct outcome assignment
// at known LS call sites (e.g. didChange notify, apply rename edits) —
// not via this classifier. v1.3 will add LS-error sentinels and bridge
// them here.
//
// "validation_failed" is similarly NOT classified by this helper — it is
// flipped at the end of a happy-path handler when the post-edit verifier
// reports VerifyResult.HasErrors (see appendVerifyInfoWithStatus).
//
// Exported as ClassifyEditError so internal/kernel/fileops/ can reuse the
// helper without duplicating the bucket map (DRY: single source of truth
// for the D-10 outcome classifier).
func ClassifyEditError(err error) string {
	if err == nil {
		return "success"
	}
	// Order matters: the more specific sentinel must win over the broader
	// serr.InvalidArgs Kind match (both ambiguity and no-match have
	// Kind=InvalidArgs but distinct sentinels per Phase 53 Wave 0).
	if errors.Is(err, fuzzy.ErrAmbiguous) {
		return "ambiguous_match"
	}
	if errors.Is(err, fuzzy.ErrNoMatch) {
		return "no_match"
	}
	// Q-3 under-classification: missing-required-field validations
	// (serr.InvalidArgs but no fuzzy sentinel) bucket as "internal" rather
	// than introducing a 7th outcome value. Same for all other error
	// kinds (serr.Internal, serr.NoWorkspace, serr.NotFound, etc.). v1.3
	// will introduce typed errors for finer classification.
	//
	// TODO(v1.3, WR-06): When the typed-error layer lands, route
	// missing-field / invalid-arg errors to a new outcome="invalid_args"
	// bucket. Adding a 7th outcome requires updating:
	//   1. editOutcomeEnum in internal/mcp/middleware.go (currently 6 values)
	//   2. The EditOutcomeInc allowlist in internal/obs/metrics.go
	//   3. TestMetrics_CardinalityBounds_EditOutcome in internal/obs/
	//      (bound moves from 7×6×4=168 to 7×7×4=196)
	//   4. The helix_edit_outcome_total row in USAGE.md
	// Until then operators cannot distinguish "agent sent garbage args"
	// from "kernel imploded" via outcome="internal" alone.
	return "internal"
}

// --- Argument structs ---

// ReplaceBodyArgs is the input schema for the replace_symbol_body tool.
type ReplaceBodyArgs struct {
	Path       string                  `json:"path" jsonschema:"File path"`
	SymbolName string                  `json:"symbol_name" jsonschema:"Name of the symbol whose body to replace"`
	NewBody    string                  `json:"new_body" jsonschema:"New body content to replace with"`
	SearchBody string                  `json:"search_body,omitempty" jsonschema:"Optional: fuzzy-match this text within the symbol body before replacing. When absent, replaces the entire body."`
	Receipts   []guardrails.ReceiptID  `json:"receipts,omitempty" jsonschema:"Receipt IDs from prior find_references / analyze_blast_radius / get_context / get_repo_map / verify_edit calls. Required by guardrails (Phase 66 GUARD-03) for destructive operations on referenced or public-API symbols. Empty array when no receipts apply."`
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
	Path     string                 `json:"path" jsonschema:"File path where symbol is defined"`
	Line     int                    `json:"line" jsonschema:"Line number of symbol (1-indexed)"`
	Col      int                    `json:"column" jsonschema:"Column number of symbol (1-indexed)"`
	NewName  string                 `json:"new_name" jsonschema:"New name for the symbol"`
	Receipts []guardrails.ReceiptID `json:"receipts,omitempty" jsonschema:"Receipt IDs from prior find_references / analyze_blast_radius / get_context / get_repo_map / verify_edit calls. Required by guardrails (Phase 66 GUARD-03) for destructive operations on referenced or public-API symbols. Empty array when no receipts apply."`
}

// SafeDeleteArgs is the input schema for the safe_delete_symbol tool.
type SafeDeleteArgs struct {
	Path       string                 `json:"path" jsonschema:"File path"`
	SymbolName string                 `json:"symbol_name" jsonschema:"Name of the symbol to delete"`
	Force      bool                   `json:"force,omitempty" jsonschema:"Delete even if references exist (default: false)"`
	Receipts   []guardrails.ReceiptID `json:"receipts,omitempty" jsonschema:"Receipt IDs from prior find_references / analyze_blast_radius / get_context / get_repo_map / verify_edit calls. Required by guardrails (Phase 66 GUARD-03) for destructive operations on referenced or public-API symbols. Empty array when no receipts apply."`
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
	out, _ := appendVerifyInfoWithStatus(ctx, diagStore, uri, text)
	return out
}

// appendVerifyInfoWithStatus is the same as appendVerifyInfo but also
// reports whether the post-edit verifier found error-level diagnostics.
// Phase 53 D-10: edit-tool handlers use the boolean to flip outcome from
// "success" to "validation_failed" when the edit applied cleanly but the
// post-edit verifier flagged a regression.
func appendVerifyInfoWithStatus(ctx context.Context, diagStore *diag.DiagnosticStore, uri string, text string) (string, bool) {
	vr, err := VerifyEdit(ctx, diagStore, uri)
	if err != nil {
		// Verifier itself failed — surface as note, but do not flag
		// validation_failed (we have no positive signal of regression).
		return text + "\n\nVerification: (error: " + err.Error() + ")", false
	}
	if vr.HasErrors {
		var sb strings.Builder
		sb.WriteString(text)
		sb.WriteString(fmt.Sprintf("\n\nPost-edit verification: %d error(s)", vr.ErrorCount))
		for _, e := range vr.Errors {
			sb.WriteString(fmt.Sprintf("\n  L%d:%d [%s] %s", e.Line, e.Col, e.Source, e.Message))
		}
		return sb.String(), true
	}
	return text + "\n\nPost-edit verification: OK (no errors)", false
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
		// Phase 53 D-16 emission: defer captures the live (outcome, strategy)
		// values; each error branch updates `outcome` (strategy stays "none"
		// on the error path), and the success branch updates `strategy` from
		// fuzzyInfo when fuzzy ran.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "replace_symbol_body", outcome, strategy) }()

		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
		if args.SymbolName == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
		if args.NewBody == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: new_body").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
		// Phase 76 ABLATE-07 (D-04): runtime backstop for the structured-edit
		// ablation arm. If reached via any back-channel under the flag, refuse
		// with a typed Unsupported error carrying the greppable
		// subsystem_disabled: prefix (kinds.go convention).
		if k.StructuredEditDisabled() {
			outcome = "unsupported"
			return errorResult(serr.New(serr.Unsupported,
				"subsystem_disabled: replace_symbol_body requires the structured-edit subsystem; use replace_in_file").
				WithTool("replace_symbol_body").Error()), nil, nil
		}
		wsKey := wsKeyFn()
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx so the
		// compaction gate sees BlockedEditTxActive while this tool runs.
		defer k.BeginEditTx(wsKey)()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			outcome = "internal"
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		lang := detectLang(args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			// LS-side acquisition failure — bucket as ls_error (best-effort
			// per D-10; classifier limits documented in ClassifyEditError).
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeReplaceBody, args.NewBody)
		if err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: execute mutation on dirty lease using the plan's range.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		fuzzyInfo, err := ReplaceBodyWithPlan(ctx, dirtyLease, extractor, plan, lang, args.SearchBody)
		if err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		// Strategy from fuzzy match if it ran. Q-4: fuzzy.StrategyFailed
		// would never reach here (ReplaceBodyWithPlan returns ErrNoMatch
		// on the failed path before producing a Result), so string(...) is
		// always one of {exact, whitespace_normalized, indentation_flexible}.
		if fuzzyInfo != nil {
			strategy = string(fuzzyInfo.Strategy)
		}
		text := fmt.Sprintf("Replaced body of %q in %s", args.SymbolName, args.Path)
		// D-07: include strategy/score when fuzzy was used; D-08: omit for full body replace.
		if fuzzyInfo != nil {
			text += fmt.Sprintf("\nmatch_strategy: %s\nsimilarity_score: %.2f", fuzzyInfo.Strategy, fuzzyInfo.Score)
		}
		var verifyFailed bool
		text, verifyFailed = appendVerifyInfoWithStatus(ctx, diagStore, uri, text)
		if verifyFailed {
			// Edit applied cleanly but post-edit verifier flagged a
			// regression. Outcome flips, but the response remains a
			// (non-IsError) textResult so callers see the diagnostic detail.
			outcome = "validation_failed"
		}
		// Phase 60 D-03: fire-and-forget signal to semantic live service.
		// The on-disk file IS modified at this point even when the
		// post-edit verifier flagged regressions, so semantic still needs
		// the re-extraction signal. Errors are swallowed; correctness is
		// guaranteed by the watcher + manifest scanner (P02 + P05).
		if n := k.EditNotifier(); n != nil {
			_ = n.OnEdit(ctx, wsKey, []string{args.Path})
		}
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "replace_symbol_body", Description: "Replace a symbol's body with new content using tree-sitter for precise extraction", BriefDescription: "Replace the entire body of a function, method, or class", HelpText: replaceSymbolBodyHelp})
}

func registerInsertBefore(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "insert_before_symbol",
		Description: "Insert content immediately before a symbol",
	}, kernel.WrapToolSpan(tracer, "insert_before_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args InsertBeforeArgs) (*mcpsdk.CallToolResult, any, error) {
		// Phase 53 D-16 emission. insert_before_symbol is non-fuzzy; strategy
		// stays "none" throughout per D-11.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "insert_before_symbol", outcome, strategy) }()

		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
		if args.SymbolName == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
		if args.Content == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: content").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
		// Phase 76 ABLATE-07 (D-04): structured-edit ablation backstop.
		if k.StructuredEditDisabled() {
			outcome = "unsupported"
			return errorResult(serr.New(serr.Unsupported,
				"subsystem_disabled: insert_before_symbol requires the structured-edit subsystem; use replace_in_file").
				WithTool("insert_before_symbol").Error()), nil, nil
		}
		wsKey := wsKeyFn()
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKey)()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			outcome = "internal"
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeInsertBefore, args.Content)
		if err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: execute mutation on dirty lease.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		if err := InsertBeforeWithPlan(ctx, dirtyLease, plan); err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Inserted content before %q in %s", args.SymbolName, args.Path)
		var verifyFailed bool
		text, verifyFailed = appendVerifyInfoWithStatus(ctx, diagStore, uri, text)
		if verifyFailed {
			outcome = "validation_failed"
		}
		// Phase 60 D-03: fire-and-forget signal to semantic live service.
		// Errors are swallowed; correctness via watcher + manifest scanner.
		if n := k.EditNotifier(); n != nil {
			_ = n.OnEdit(ctx, wsKey, []string{args.Path})
		}
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "insert_before_symbol", Description: "Insert content immediately before a symbol", BriefDescription: "Insert code before a symbol definition", HelpText: insertBeforeSymbolHelp})
}

func registerInsertAfter(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "insert_after_symbol",
		Description: "Insert content immediately after a symbol",
	}, kernel.WrapToolSpan(tracer, "insert_after_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args InsertAfterArgs) (*mcpsdk.CallToolResult, any, error) {
		// Phase 53 D-16 emission. insert_after_symbol is non-fuzzy; strategy
		// stays "none" throughout per D-11.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "insert_after_symbol", outcome, strategy) }()

		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
		if args.SymbolName == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
		if args.Content == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: content").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
		// Phase 76 ABLATE-07 (D-04): structured-edit ablation backstop.
		if k.StructuredEditDisabled() {
			outcome = "unsupported"
			return errorResult(serr.New(serr.Unsupported,
				"subsystem_disabled: insert_after_symbol requires the structured-edit subsystem; use replace_in_file").
				WithTool("insert_after_symbol").Error()), nil, nil
		}
		wsKey := wsKeyFn()
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKey)()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			outcome = "internal"
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeInsertAfter, args.Content)
		if err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: execute mutation on dirty lease.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		if err := InsertAfterWithPlan(ctx, dirtyLease, plan); err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		text := fmt.Sprintf("Inserted content after %q in %s", args.SymbolName, args.Path)
		var verifyFailed bool
		text, verifyFailed = appendVerifyInfoWithStatus(ctx, diagStore, uri, text)
		if verifyFailed {
			outcome = "validation_failed"
		}
		// Phase 60 D-03: fire-and-forget signal to semantic live service.
		// Errors are swallowed; correctness via watcher + manifest scanner.
		if n := k.EditNotifier(); n != nil {
			_ = n.OnEdit(ctx, wsKey, []string{args.Path})
		}
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "insert_after_symbol", Description: "Insert content immediately after a symbol", BriefDescription: "Insert code after a symbol definition", HelpText: insertAfterSymbolHelp})
}

func registerRenameSymbol(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "rename_symbol",
		Description: "Rename a symbol across all files in the workspace",
	}, kernel.WrapToolSpan(tracer, "rename_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args RenameSymbolArgs) (*mcpsdk.CallToolResult, any, error) {
		// Phase 53 D-11 + D-16: rename_symbol is non-fuzzy; strategy stays
		// "none" throughout this metric. The orthogonal helix_rename_strategy_total
		// counter (Phase 47 D-07, kept verbatim) tracks LSP-native vs.
		// rust-client-side dispatch as a separate dimension.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "rename_symbol", outcome, strategy) }()

		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("rename_symbol").Error()), nil, nil
		}
		if args.NewName == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: new_name").
				WithTool("rename_symbol").Error()), nil, nil
		}
		// Phase 76 WR-02: rename_symbol leases a live LS worker, so it belongs
		// to the no_lsp ablation surface as well as the edit surface. Add the
		// LSPSubsystemDisabled() runtime backstop (mirroring the symbols
		// acquireLease guard) so a back-channel tools/call under the flag
		// refuses with a typed Unsupported error instead of leasing a worker
		// and emitting lspool.lsp.* spans.
		if k.LSPSubsystemDisabled() {
			outcome = "unsupported"
			return errorResult(serr.New(serr.Unsupported,
				"subsystem_disabled: rename_symbol requires the LSP subsystem, which is disabled").
				WithTool("rename_symbol").Error()), nil, nil
		}
		wsKey := wsKeyFn()
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKey)()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			outcome = "internal"
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		lease, err := rt.AcquireSession(ctx, "default", true) // dirty=true
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Convert from 1-indexed (user-facing) to 0-indexed (LSP).
		result, err := RenameSymbol(ctx, lease, uri, args.Line-1, args.Col-1, args.NewName)
		if err != nil {
			// RenameSymbol wraps LS-side failures in serr.Internal +
			// serr.Unsupported; classify as ls_error best-effort.
			outcome = "ls_error"
			return errorResult(err.Error()), nil, nil
		}
		// Phase 47 D-07 (PRESERVED) + Phase 53 D-11: rename emits BOTH families.
		// helix_rename_strategy_total tracks LSP-native vs. rust-client-side
		// dispatch (orthogonal dimension); helix_edit_outcome_total tracks the
		// outcome bucket. Both must coexist.
		mcp.RecordRenameStrategy(ctx, string(result.Strategy))
		text := fmt.Sprintf("Renamed to %q: %d files changed, %d edits applied\nFiles: %s\nstrategy: %s",
			args.NewName, result.FilesChanged, result.EditsApplied, strings.Join(result.Files, ", "), result.Strategy)
		var verifyFailed bool
		text, verifyFailed = appendVerifyInfoWithStatus(ctx, diagStore, uri, text)
		if verifyFailed {
			outcome = "validation_failed"
		}
		// Phase 60 D-03: fire-and-forget signal to semantic live service.
		// Rename can mutate many files via WorkspaceEdit; pass the full
		// per-file path slice collected by RenameSymbol. Errors are
		// swallowed; correctness via watcher + manifest scanner.
		if len(result.Files) > 0 {
			if n := k.EditNotifier(); n != nil {
				_ = n.OnEdit(ctx, wsKey, result.Files)
			}
		}
		return textResult(text), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "rename_symbol", Description: "Rename a symbol across all files in the workspace", BriefDescription: "Rename a symbol across the entire workspace", HelpText: renameSymbolHelp})
}

func registerSafeDelete(server *mcp.SerenaMCPServer, k *kernel.Kernel, diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "safe_delete_symbol",
		Description: "Delete a symbol if it has no references; reports reference count if blocked",
	}, kernel.WrapToolSpan(tracer, "safe_delete_symbol", func(ctx context.Context, req *mcpsdk.CallToolRequest, args SafeDeleteArgs) (*mcpsdk.CallToolResult, any, error) {
		// Phase 53 D-16 emission. safe_delete_symbol is non-fuzzy; strategy
		// stays "none" throughout per D-11.
		outcome, strategy := "success", "none"
		defer func() { mcp.RecordEditOutcome(ctx, "safe_delete_symbol", outcome, strategy) }()

		if args.Path == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
				WithTool("safe_delete_symbol").Error()), nil, nil
		}
		if args.SymbolName == "" {
			outcome = "internal"
			return errorResult(serr.New(serr.InvalidArgs, "missing required field: symbol_name").
				WithTool("safe_delete_symbol").Error()), nil, nil
		}
		wsKey := wsKeyFn()
		// Phase 63 P63-02 Task 1: stamp in-flight edit-tx for the gate.
		defer k.BeginEditTx(wsKey)()
		rt, err := k.GetRuntime(wsKey)
		if err != nil {
			outcome = "internal"
			return errorResult(serr.Wrap(serr.NoWorkspace, "workspace not activated", err).Error()), nil, nil
		}
		uri := filePathToURI(wsKey.RepoRoot, args.Path)
		// Phase 1: plan on clean (shared) lease for accurate symbol ranges and references.
		cleanLease, err := rt.AcquireSession(ctx, "plan-read", false)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire read session", err).Error()), nil, nil
		}
		plan, err := PlanEdit(ctx, cleanLease, uri, args.SymbolName, EditTypeDelete, "")
		if err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		// Phase 2: check references on clean lease, execute on dirty lease.
		dirtyLease, err := rt.AcquireSession(ctx, "default", true)
		if err != nil {
			outcome = "ls_error"
			return errorResult(serr.Wrap(serr.Internal, "acquire session", err).Error()), nil, nil
		}
		result, err := SafeDeleteWithPlan(ctx, cleanLease, dirtyLease, plan, args.Force)
		if err != nil {
			outcome = ClassifyEditError(err)
			return errorResult(err.Error()), nil, nil
		}
		if !result.Deleted {
			// Refused-with-references is NOT an edit failure — the tool
			// did exactly what it promised (it refused to delete a
			// referenced symbol). Bucket as success per D-10 (this is the
			// "operator-visible refusal" path, not an error path).
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
		var verifyFailed bool
		text, verifyFailed = appendVerifyInfoWithStatus(ctx, diagStore, uri, text)
		if verifyFailed {
			outcome = "validation_failed"
		}
		// Phase 60 D-03: fire-and-forget signal to semantic live service.
		// Only fires on the actual-deletion branch (refused-with-references
		// returns earlier without modifying the file). Errors swallowed;
		// correctness via watcher + manifest scanner.
		if n := k.EditNotifier(); n != nil {
			_ = n.OnEdit(ctx, wsKey, []string{args.Path})
		}
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
			// Phase 66 D-01 GUARD-03: issue diagnostics_clean receipt ONLY when ErrorCount==0 (D-18/T-66-25).
			guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassDiagnosticsClean,
				guardrails.DiagnosticsCleanScope{
					FileSet:         []string{args.Path},
					DiagnosticCount: 0,
					ErrorCount:      0,
					WarningCount:    0,
					Tool:            "verify_edit",
				}, "verify_edit")
			return textResult("No errors found."), nil, nil
		}
		// result.HasErrors is true: DO NOT issue receipt (ErrorCount > 0).
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%d error(s) found:\n", result.ErrorCount))
		for _, e := range result.Errors {
			sb.WriteString(fmt.Sprintf("  L%d:%d [%s] %s\n", e.Line, e.Col, e.Source, e.Message))
		}
		return textResult(sb.String()), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "verify_edit", Description: "Check for compilation errors after an edit; returns diagnostic summary", BriefDescription: "Check for compilation errors after an edit", HelpText: verifyEditHelp})
}
