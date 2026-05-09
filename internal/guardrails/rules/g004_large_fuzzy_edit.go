package rules

import (
	"context"

	"github.com/agenthands/helix/internal/guardrails"
)

const (
	defaultG004MaxChangedLines = 50
	defaultG004MaxFiles        = 1
	defaultG004MaxRatio        = 0.30
)

// EvaluateG004 evaluates the large-fuzzy-edit rule (G-004, D-16 LOCKED).
//
// Trigger conditions (OR logic):
//  1. changed_lines > cfg.G004.MaxChangedLines (default 50).
//  2. len(touched_files) > cfg.G004.MaxFiles (default 1).
//  3. cfg.G004.EnableRatioTrigger && file_line_count > 0 &&
//     changed_lines/file_line_count > cfg.G004.MaxFileChangeRatio (default 0.30).
//
// Required receipt:
//   - Single file (len(touched_files) == 1): context_gathered.
//   - Multi-file (len(touched_files) > 1): structural_overview.
//
// G-004 has no degraded mode: its trigger is pure LOC math — no semantic index needed.
func EvaluateG004(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	cfg := sc.Config.G004

	maxLines := cfg.MaxChangedLines
	if maxLines <= 0 {
		maxLines = defaultG004MaxChangedLines
	}
	maxFiles := cfg.MaxFiles
	if maxFiles <= 0 {
		maxFiles = defaultG004MaxFiles
	}
	maxRatio := cfg.MaxFileChangeRatio
	if maxRatio <= 0 {
		maxRatio = defaultG004MaxRatio
	}

	triggered := false

	// Primary trigger 1: lines exceeded.
	if args.ChangedLines > maxLines {
		triggered = true
	}

	// Primary trigger 2: file count exceeded.
	if len(args.TouchedFiles) > maxFiles {
		triggered = true
	}

	// Secondary trigger: ratio (gated on EnableRatioTrigger).
	if !triggered && cfg.EnableRatioTrigger && args.FileLineCount > 0 {
		ratio := float64(args.ChangedLines) / float64(args.FileLineCount)
		if ratio > maxRatio {
			triggered = true
		}
	}

	if !triggered {
		return Decision{Action: Allow}
	}

	// Determine required receipt class based on file count.
	multiFile := len(args.TouchedFiles) > 1
	var requiredClass guardrails.ReceiptClass
	if multiFile {
		requiredClass = guardrails.ClassStructuralOverview
	} else {
		requiredClass = guardrails.ClassContextGathered
	}

	target := guardrails.Target{
		Path:         args.Path,
		TouchedFiles: args.TouchedFiles,
		RequiredClass: requiredClass,
	}
	if findReceiptCovering(sc, args, []guardrails.ReceiptClass{requiredClass}, target) {
		return Decision{Action: Allow}
	}

	level := resolveLevel(args, sc, "G-004")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	scopeHint := "get_context on " + args.Path
	suggestedTool := "get_context"
	if multiFile {
		scopeHint = "get_repo_map to understand multi-file structure"
		suggestedTool = "get_repo_map"
	}

	return Decision{
		Action:  action,
		Rule:    "G-004",
		Message: "large edit detected; gather context before proceeding",
		RequiredReceipts: []RequiredReceipt{
			{Class: requiredClass, ScopeHint: scopeHint},
		},
		SuggestedTools: []string{suggestedTool},
		SeeAlso: []SeeAlsoRef{
			{Tool: "get_tool_help", Args: map[string]string{"topic": "workflow:large-edit"}},
		},
	}
}
