package rules

import (
	"context"
	"regexp"

	"github.com/agenthands/helix/internal/guardrails"
)

// identifierRe is the G-001 identifier gate: matches strings that look like
// Go/TS/JS/Python identifiers (3+ chars, starts with letter or underscore).
// T-66-12 mitigation: a find string that does NOT match this regex takes an
// arguably-safer non-symbolic-replacement path and is exempt from G-001.
var identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{2,}$`)

// g001Tools is the closed set of tools that G-001 applies to.
var g001Tools = map[string]bool{
	"fuzzy_edit":      true,
	"replace_in_file": true,
}

// EvaluateG001 evaluates the rename-by-grep rule (G-001, D-13 LOCKED).
//
// Trigger conditions:
//  1. Tool is fuzzy_edit or replace_in_file.
//  2. args.Find matches `^[A-Za-z_][A-Za-z0-9_]{2,}$` (identifier shape).
//  3. args.Find is a substring-of or equal-to a symbol name returned by
//     OutlineProvider.SymbolsInFile(args.Path).
//
// If all three conditions hold and no covering receipt (references_checked or
// impact_checked) is present, returns Warn or Block (per enforcement level).
// Otherwise returns Allow.
//
// rename_symbol is unconditionally exempt (the tool itself is semantic; agents
// using it are doing the right thing).
func EvaluateG001(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	// Condition 0: exempt tools.
	if args.Tool == "rename_symbol" || !g001Tools[args.Tool] {
		return Decision{Action: Allow}
	}

	// Condition 1: identifier shape check.
	if !identifierRe.MatchString(args.Find) {
		return Decision{Action: Allow}
	}

	// Condition 2: is args.Find a known symbol in this file?
	symbols, err := sc.OutlineProvider.SymbolsInFile(ctx, sc.Workspace, args.Path)
	if err != nil || len(symbols) == 0 {
		// Cannot determine outline: safe to allow (no false positives).
		return Decision{Action: Allow}
	}
	var matchedSymbol string
	for _, sym := range symbols {
		if sym.Name == args.Find || containsIdentifier(sym.Name, args.Find) {
			matchedSymbol = sym.Name
			break
		}
	}
	if matchedSymbol == "" {
		// args.Find is not a known symbol — safe substring replacement.
		return Decision{Action: Allow}
	}

	// Condition 3: check for covering receipt.
	target := guardrails.Target{
		SymbolID:      args.SymbolID,
		Path:          args.Path,
		TouchedFiles:  []string{args.Path},
		RequiredClass: guardrails.ClassReferencesChecked,
	}
	if findReceiptCovering(sc, args, []guardrails.ReceiptClass{
		guardrails.ClassReferencesChecked,
		guardrails.ClassImpactChecked,
	}, target) {
		return Decision{Action: Allow}
	}

	// Trigger: emit Block or Warn.
	level := resolveLevel(args, sc, "G-001")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	return Decision{
		Action:  action,
		Rule:    "G-001",
		Message: "rename-by-grep detected: find=" + args.Find + " matches symbol " + matchedSymbol + "; run find_references or analyze_blast_radius first",
		RequiredReceipts: []RequiredReceipt{
			{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on symbol " + matchedSymbol},
			{Class: guardrails.ClassImpactChecked, ScopeHint: "analyze_blast_radius on symbol " + matchedSymbol},
		},
		SuggestedTools: []string{"find_references", "analyze_blast_radius"},
		SeeAlso: []SeeAlsoRef{
			{Tool: "get_tool_help", Args: map[string]string{"topic": "workflow:rename-by-grep"}},
		},
	}
}

// containsIdentifier returns true if the symbol name equals or contains find
// as an identifier substring (whole-word boundary match by string equality
// in the simplified Plan 03 implementation).
// For Plan 03 we use simple string equality (the outline already filters to
// declared symbols; callers check sym.Name == args.Find OR we can expand later).
func containsIdentifier(symName, find string) bool {
	// Simple equality: the find string IS the symbol name.
	return symName == find
}
