package rules

import (
	"context"
	"errors"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
)

// EvaluateG002 evaluates the delete-without-refs rule (G-002, D-14 LOCKED).
//
// Trigger conditions by tool:
//   - delete_file: tracked source file → always check; if exported symbols → also require impact_checked.
//   - safe_delete_symbol: always triggers.
//   - replace_symbol_body: triggers when refcount>0 OR CallerCount>0 OR Visibility.IsPublicLike()
//     OR IsEntrypointReachable.
//   - fuzzy_edit / replace_in_file: triggers when args.IsRemovingDecl is set.
//
// D-19 degraded mode (Available()==false): conservative Warn — cannot count refs.
func EvaluateG002(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	switch args.Tool {
	case "delete_file":
		return evaluateG002DeleteFile(ctx, args, sc)
	case "safe_delete_symbol":
		return evaluateG002SafeDelete(ctx, args, sc)
	case "replace_symbol_body":
		return evaluateG002ReplaceBody(ctx, args, sc)
	case "fuzzy_edit", "replace_in_file":
		if !args.IsRemovingDecl {
			return Decision{Action: Allow}
		}
		return evaluateG002SafeDelete(ctx, args, sc) // same receipt requirement
	default:
		return Decision{Action: Allow}
	}
}

func evaluateG002DeleteFile(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	if !args.IsTrackedSourceFile {
		return Decision{Action: Allow}
	}

	// Degraded mode: cannot verify refs, issue conservative Warn.
	if !sc.Lookup.Available() {
		return Decision{
			Action:  Warn, // D-19: always Warn in degraded mode regardless of configured level
			Rule:    "G-002",
			Message: "semantic index unavailable; conservatively warning before delete_file " + args.Path,
			RequiredReceipts: []RequiredReceipt{
				{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on symbols in " + args.Path},
			},
			SuggestedTools: []string{"find_references"},
			Warnings:       []string{"degraded mode: semantic index unavailable; verify references manually"},
		}
	}

	// Check for covering receipt.
	target := guardrails.Target{
		Path:         args.Path,
		TouchedFiles: []string{args.Path},
	}

	if args.HasExportedSymbols {
		// Exported symbols require impact_checked specifically.
		target.RequiredClass = guardrails.ClassImpactChecked
		if findReceiptCovering(sc, args, []guardrails.ReceiptClass{guardrails.ClassImpactChecked}, target) {
			return Decision{Action: Allow}
		}
	} else {
		target.RequiredClass = guardrails.ClassReferencesChecked
		if findReceiptCovering(sc, args, []guardrails.ReceiptClass{
			guardrails.ClassReferencesChecked,
			guardrails.ClassImpactChecked,
		}, target) {
			return Decision{Action: Allow}
		}
	}

	level := resolveLevel(args, sc, "G-002")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	required := []RequiredReceipt{
		{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on symbols in " + args.Path},
	}
	if args.HasExportedSymbols {
		required = []RequiredReceipt{
			{Class: guardrails.ClassImpactChecked, ScopeHint: "analyze_blast_radius on exported symbols in " + args.Path},
		}
	}
	return Decision{
		Action:           action,
		Rule:             "G-002",
		Message:          "delete_file on tracked source " + args.Path + "; verify references before deleting",
		RequiredReceipts: required,
		SuggestedTools:   []string{"find_references", "analyze_blast_radius"},
	}
}

func evaluateG002SafeDelete(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	// Degraded mode.
	if !sc.Lookup.Available() {
		return Decision{
			Action:  Warn,
			Rule:    "G-002",
			Message: "semantic index unavailable; conservatively warning before deletion of " + args.SymbolName,
			RequiredReceipts: []RequiredReceipt{
				{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on " + args.SymbolName},
			},
			SuggestedTools: []string{"find_references"},
			Warnings:       []string{"degraded mode: semantic index unavailable"},
		}
	}

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

	level := resolveLevel(args, sc, "G-002")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	return Decision{
		Action:  action,
		Rule:    "G-002",
		Message: "symbol deletion requires references check: verify no callers before removing " + args.SymbolName,
		RequiredReceipts: []RequiredReceipt{
			{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on " + args.SymbolName},
			{Class: guardrails.ClassImpactChecked, ScopeHint: "analyze_blast_radius on " + args.SymbolName},
		},
		SuggestedTools: []string{"find_references", "analyze_blast_radius"},
	}
}

func evaluateG002ReplaceBody(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	// Degraded mode: conservative warn.
	if !sc.Lookup.Available() {
		return Decision{
			Action:  Warn,
			Rule:    "G-002",
			Message: "semantic index unavailable; conservatively warning before replace_symbol_body",
			RequiredReceipts: []RequiredReceipt{
				{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on " + args.SymbolName},
			},
			SuggestedTools: []string{"find_references"},
			Warnings:       []string{"degraded mode: semantic index unavailable"},
		}
	}

	// Check trigger conditions.
	triggered := false
	if args.RefCount > 0 || args.CallerCount > 0 {
		triggered = true
	}
	if !triggered {
		vis, err := sc.Lookup.Visibility(ctx, sc.Workspace, args.SymbolID)
		if err == nil && vis.IsPublicLike() {
			triggered = true
		}
		if err != nil && !errors.Is(err, serr.ErrUnsupported) {
			// unexpected error: conservative trigger
			triggered = true
		}
	}
	if !triggered {
		isEP, err := sc.Lookup.IsEntrypointReachable(ctx, sc.Workspace, args.SymbolID)
		if err == nil && isEP {
			triggered = true
		}
	}

	if !triggered {
		return Decision{Action: Allow}
	}

	// Triggered: check for covering receipt.
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

	level := resolveLevel(args, sc, "G-002")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	return Decision{
		Action:  action,
		Rule:    "G-002",
		Message: "replace_symbol_body on symbol with callers or public visibility: verify references first",
		RequiredReceipts: []RequiredReceipt{
			{Class: guardrails.ClassReferencesChecked, ScopeHint: "find_references on " + args.SymbolName},
			{Class: guardrails.ClassImpactChecked, ScopeHint: "analyze_blast_radius on " + args.SymbolName},
		},
		SuggestedTools: []string{"find_references", "analyze_blast_radius"},
	}
}
