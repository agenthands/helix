// Package rules implements the five Phase 66 guardrail rule predicates (G-001..G-005)
// and the RuleEvaluator dispatch interface.
//
// Each predicate is a pure-Go function with signature:
//
//	EvaluateGXXX(ctx context.Context, args RuleArgs, sc SessionContext) Decision
//
// Predicates are stateless: all inputs are supplied by the middleware (Plan 04)
// before dispatch. The RuleEvaluator aggregates decisions from multiple rules
// for a single tool call; the middleware inspects the returned slice and blocks
// if any Decision.Action == Block.
//
// Rule trigger summary (D-13..D-17 LOCKED):
//
//	G-001: rename-by-grep — tool ∈ {fuzzy_edit, replace_in_file} AND find matches
//	       identifier regex AND find is a known symbol in OutlineProvider.SymbolsInFile.
//	       Required receipt: references_checked OR impact_checked.
//
//	G-002: delete-without-refs — tool ∈ {delete_file, safe_delete_symbol,
//	       replace_symbol_body} when symbol has refs/callers/public visibility.
//	       Required receipt: references_checked OR impact_checked.
//
//	G-003: public-API edit — target is public-like OR entry-point-reachable OR
//	       signature change with refcount>0. Required receipt: impact_checked
//	       (references_checked alone REJECTED per D-15).
//
//	G-004: large fuzzy edit — changed_lines > max OR touched_files > max.
//	       Required receipt: context_gathered (single-file) or structural_overview (multi).
//
//	G-005: security-sensitive — 3-signal classifier (path glob + identifier + import).
//	       Required receipt: context_gathered or structural_overview.
package rules
