package rules

import (
	"context"

	"github.com/agenthands/helix/internal/guardrails"
)

// RuleEvaluator is the interface the middleware (Plan 04) invokes per tool call.
// Evaluate returns 0..N decisions, one per triggered rule. The middleware
// aggregates them: if any Decision.Action == Block, the call is blocked;
// otherwise warnings from Warn decisions accumulate in the response.
type RuleEvaluator interface {
	Evaluate(ctx context.Context, args RuleArgs, sc SessionContext) []Decision
}

// DefaultEvaluator dispatches to the correct rule subset per closed-enum tool name.
// Unknown tools return an empty slice (no rules apply; middleware passes through).
type DefaultEvaluator struct{}

// Evaluate dispatches to the correct rule subset for args.Tool.
// The dispatch table is closed-enum per D-13..D-17.
func (DefaultEvaluator) Evaluate(ctx context.Context, args RuleArgs, sc SessionContext) []Decision {
	var decisions []Decision

	switch args.Tool {
	case "rename_symbol":
		// G-001 exempt (rename_symbol is semantic).
		// G-002: check for refs/callers/public before deleting old symbol.
		// G-003: public-API edit.
		// G-004: single-symbol op — skip (no LOC trigger).
		// G-005: security-sensitive path.
		appendIfTriggered(&decisions, EvaluateG002(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG003(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG005(ctx, args, sc))

	case "safe_delete_symbol":
		appendIfTriggered(&decisions, EvaluateG002(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG003(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG005(ctx, args, sc))

	case "replace_symbol_body":
		appendIfTriggered(&decisions, EvaluateG002(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG003(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG004(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG005(ctx, args, sc))

	case "fuzzy_edit", "replace_in_file":
		appendIfTriggered(&decisions, EvaluateG001(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG002(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG003(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG004(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG005(ctx, args, sc))

	case "delete_file":
		appendIfTriggered(&decisions, EvaluateG002(ctx, args, sc))
		appendIfTriggered(&decisions, EvaluateG005(ctx, args, sc))

	default:
		// Unknown tool: no rules apply.
	}

	return decisions
}

// appendIfTriggered appends d to decisions only when d.Action != Allow.
func appendIfTriggered(decisions *[]Decision, d Decision) {
	if d.Action != Allow {
		*decisions = append(*decisions, d)
	}
}

// findReceiptCovering iterates args.Receipts and checks if any satisfies one of
// the required receipt classes for the given target (STRICT scope-validated via
// ValidateReceiptForOperation).
//
// Returns true if at least one receipt is valid, matches one of requiredClasses,
// and passes scope validation for target.
func findReceiptCovering(sc SessionContext, args RuleArgs, requiredClasses []guardrails.ReceiptClass, target guardrails.Target) bool {
	for _, rid := range args.Receipts {
		rcpt, ok := sc.Store.Get(sc.Workspace, rid)
		if !ok {
			continue
		}
		// Check if this receipt's class is in the required set.
		for _, required := range requiredClasses {
			// Set the target's RequiredClass for the validation check.
			t := target
			t.RequiredClass = required
			if rcpt.Class != required {
				continue
			}
			if err := guardrails.ValidateReceiptForOperation(rcpt, t, sc.GraphVersion, sc.Workspace, sc.Now); err == nil {
				return true
			}
		}
	}
	return false
}
