package rules

import (
	"context"
	"errors"
	"unicode"

	"github.com/agenthands/helix/internal/guardrails"
)

// EvaluateG003 evaluates the public-API-edit rule (G-003, D-15 LOCKED).
//
// Trigger conditions:
//   - target Visibility.IsPublicLike(), OR
//   - IsEntrypointReachable, OR
//   - args.SignatureChange && args.RefCount > 0.
//
// Key D-15 invariant: references_checked receipt alone is REJECTED.
// Only impact_checked satisfies G-003.
//
// D-19 degraded mode (Available()==false):
// Falls back to conservative warn when SymbolName starts with an uppercase
// letter (Go exported-naming heuristic).
func EvaluateG003(ctx context.Context, args RuleArgs, sc SessionContext) Decision {
	triggered := false
	degraded := !sc.Lookup.Available()

	if degraded {
		// D-19 conservative heuristic: uppercase first letter → possibly exported.
		if len(args.SymbolName) > 0 && unicode.IsUpper(rune(args.SymbolName[0])) {
			triggered = true
		}
	} else {
		// Check Visibility.
		vis, err := sc.Lookup.Visibility(ctx, sc.Workspace, args.SymbolID)
		if err == nil && vis.IsPublicLike() {
			triggered = true
		}
		if err != nil && !errors.Is(err, errors.ErrUnsupported) {
			// Unexpected error: conservative trigger.
			triggered = true
		}

		// Check entry-point reachability.
		if !triggered {
			isEP, err := sc.Lookup.IsEntrypointReachable(ctx, sc.Workspace, args.SymbolID)
			if err == nil && isEP {
				triggered = true
			}
		}
	}

	// Signature change with refs trigger.
	if !triggered && args.SignatureChange && args.RefCount > 0 {
		triggered = true
	}

	if !triggered {
		return Decision{Action: Allow}
	}

	// D-15: only impact_checked satisfies G-003.
	// references_checked alone is REJECTED.
	target := guardrails.Target{
		SymbolID:      args.SymbolID,
		Path:          args.Path,
		TouchedFiles:  []string{args.Path},
		RequiredClass: guardrails.ClassImpactChecked,
	}
	if findReceiptCovering(sc, args, []guardrails.ReceiptClass{guardrails.ClassImpactChecked}, target) {
		return Decision{Action: Allow}
	}

	// Triggered and no valid impact_checked receipt: determine action.
	level := resolveLevel(args, sc, "G-003")
	action := ResolveAction(level)
	if action == Allow {
		return Decision{Action: Allow}
	}

	// For degraded mode, downgrade Block to Warn (D-19 conservative, not punitive).
	if degraded && action == Block {
		action = Warn
	}

	return Decision{
		Action:  action,
		Rule:    "G-003",
		Message: "public-API edit requires impact_checked receipt (analyze_blast_radius); references_checked alone is not sufficient (D-15)",
		RequiredReceipts: []RequiredReceipt{
			{Class: guardrails.ClassImpactChecked, ScopeHint: "analyze_blast_radius on " + args.SymbolName},
		},
		SuggestedTools: []string{"analyze_blast_radius"},
		SeeAlso: []SeeAlsoRef{
			{Tool: "get_tool_help", Args: map[string]string{"topic": "workflow:public-api-edit"}},
		},
	}
}
