package rules

import (
	"context"
	"time"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/catalogs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// Action is the guardrail decision outcome.
type Action int

const (
	// Allow means the tool call may proceed with no restriction.
	Allow Action = iota
	// Warn means the tool call may proceed but a warning is surfaced.
	Warn
	// Block means the tool call is refused until required receipts are satisfied.
	Block
)

// RequiredReceipt describes a receipt class that must be present before the
// tool call is allowed to proceed.
type RequiredReceipt struct {
	// Class is the receipt class from the guardrails closed enum.
	Class guardrails.ReceiptClass
	// ScopeHint is a human-readable hint: "find_references on symbol X".
	ScopeHint string
}

// SeeAlsoRef points the agent to a related tool invocation.
type SeeAlsoRef struct {
	Tool string
	Args map[string]string
}

// Decision is the output of a single rule predicate evaluation.
type Decision struct {
	Action           Action
	Rule             string            // "G-001".."G-005" or "" for Allow
	Message          string
	RequiredReceipts []RequiredReceipt
	SuggestedTools   []string
	SeeAlso          []SeeAlsoRef
	Warnings         []string // populated when Action==Warn
}

// OutlineSymbol is a symbol discovered in a file by the tree-sitter outline provider.
type OutlineSymbol struct {
	Name       string
	Kind       string // "function", "method", "class", "type", ...
	StartLine  int
	EndLine    int
	Visibility integ.Visibility
}

// OutlineProvider returns symbol names declared in a file.
// In production (Plan 04), an adapter wraps the existing tree-sitter outline extractor.
// In tests, a fake returns a constant slice.
type OutlineProvider interface {
	SymbolsInFile(ctx context.Context, ws workspace.WorkspaceKey, path string) ([]OutlineSymbol, error)
}

// RuleArgs captures the per-tool argument shape that predicates inspect.
// The middleware (Plan 04) populates this struct before predicate dispatch.
type RuleArgs struct {
	Tool         string
	Path         string
	Find         string         // fuzzy_edit/replace_in_file "find" arg
	Replace      string
	SymbolID     integ.SymbolID // resolved by middleware before predicate dispatch
	SymbolName   string
	NewName      string
	NewBody      string
	SearchBody   string
	ChangedLines int
	TouchedFiles []string

	// FileImports is the list of import paths in the target file, populated
	// by the middleware from the semantic index (when Available()).
	FileImports []string

	// FileImportsFromGrep is the import-grep fallback list populated by the
	// middleware when Lookup.Available()==false (D-19 degraded mode).
	FileImportsFromGrep []string

	// FileLineCount is the total line count of the target file, needed by
	// the G-004 ratio trigger. Cached per (path, mtime) by the middleware.
	FileLineCount int

	// Receipts is the list of receipt IDs the agent passed with this call.
	Receipts []guardrails.ReceiptID

	// SignatureChange indicates this is a signature-changing operation (G-003 trigger).
	SignatureChange bool

	// RefCount is the reference count of the target symbol (from semantic index).
	// Used by G-002 and G-003.
	RefCount int

	// CallerCount is the caller count of the target symbol.
	CallerCount int

	// IsTrackedSourceFile is true when the Path is a tracked source file
	// (not a generated or vendor file). Used by G-002 delete_file trigger.
	IsTrackedSourceFile bool

	// HasExportedSymbols is true when the file at Path contains at least one
	// exported/public symbol. Used by G-002 delete_file impact_checked escalation.
	HasExportedSymbols bool

	// IsRemovingDecl is true when the fuzzy_edit/replace_in_file operation
	// removes a declaration or body. Used by G-002 fuzzy_edit sub-trigger.
	IsRemovingDecl bool
}

// SessionContext is the predicate input bundle provided by the middleware.
type SessionContext struct {
	Workspace         workspace.WorkspaceKey
	Profile           string
	Lookup            integ.SemanticLookup
	Store             *guardrails.Store
	Config            semantic.GuardrailsConfig
	OutlineProvider   OutlineProvider
	GraphVersion      uint64
	Now               time.Time
	Catalogs          map[string]catalogs.Catalog // language → Catalog, populated by middleware
	ProfileEnforcement guardrails.EnforcementLevel // resolved profile-level enforcement
}

// ResolveAction maps an EnforcementLevel to the corresponding Action.
// LevelRequireForce is treated as Block (RESEARCH "Pitfall 6": until the
// force-override surface is implemented in a later plan, require_force == enforce).
func ResolveAction(level guardrails.EnforcementLevel) Action {
	switch level {
	case guardrails.LevelOff:
		return Allow
	case guardrails.LevelWarn:
		return Warn
	case guardrails.LevelEnforce, guardrails.LevelRequireForce:
		return Block
	default:
		return Warn // safe default
	}
}

// resolveLevel computes the effective enforcement level for a (tool, rule) pair
// using D-20 5-layer highest-precedence-set-wins logic:
//
//	CLI (unset in Plan 03) > per-tool > per-rule > profile > global
func resolveLevel(args RuleArgs, sc SessionContext, ruleID string) guardrails.EnforcementLevel {
	// CLI layer: always unset in Plan 03.
	cli := guardrails.EnforcementLayer{Set: false}

	// Per-tool layer.
	var perTool guardrails.EnforcementLayer
	if tc, ok := sc.Config.Tools[args.Tool]; ok && tc.Enforcement != "" {
		if lv, err := guardrails.ParseEnforcementLevel(tc.Enforcement); err == nil {
			perTool = guardrails.EnforcementLayer{Level: lv, Set: true}
		}
	}

	// Per-rule layer.
	var perRule guardrails.EnforcementLayer
	if rc, ok := sc.Config.Rules[ruleID]; ok && rc.Enforcement != "" {
		if lv, err := guardrails.ParseEnforcementLevel(rc.Enforcement); err == nil {
			perRule = guardrails.EnforcementLayer{Level: lv, Set: true}
		}
	}

	// Profile layer.
	var profileLayer guardrails.EnforcementLayer
	if sc.ProfileEnforcement != "" {
		profileLayer = guardrails.EnforcementLayer{Level: sc.ProfileEnforcement, Set: true}
	}

	// Global layer.
	var global guardrails.EnforcementLayer
	if sc.Config.Enforcement != "" {
		if lv, err := guardrails.ParseEnforcementLevel(sc.Config.Enforcement); err == nil {
			global = guardrails.EnforcementLayer{Level: lv, Set: true}
		}
	}

	return guardrails.ResolveGuardrailEnforcement(cli, perTool, perRule, profileLayer, global)
}
