package score

import (
	"strings"

	"github.com/agenthands/helix/internal/eval/trace"
)

// Delta records the score contribution of a single rule application.
type Delta struct {
	RuleID string
	Score  int
}

// Score is the aggregate result of applying a Rules set to a MergedTrace.
// Total is the sum of all Delta.Score values. Apply is deterministic — the
// same trace and rules always produce the same Score.
type Score struct {
	Total  int
	Deltas []Delta
}

// Apply evaluates all rules in r against the merged trace t and returns the
// aggregate Score. The order of Deltas mirrors the order of rules in r.
func Apply(t trace.MergedTrace, r Rules) Score {
	// Filter to daemon tool_call events — the only events relevant to scoring.
	var calls []trace.Event
	for _, ev := range t.Events {
		if ev.Source == "daemon" && ev.Kind == trace.KindToolCall {
			calls = append(calls, ev)
		}
	}

	var s Score

	// ExpectSequence: match ordered subsequences; +Score on hit.
	for _, rule := range r.ExpectSequence {
		if matchSubsequence(calls, rule.Pattern) {
			s.Deltas = append(s.Deltas, Delta{RuleID: rule.ID, Score: rule.Score})
			s.Total += rule.Score
		}
	}

	// ForbidSequence: same matcher but score is (already) negative.
	for _, rule := range r.ForbidSequence {
		if matchSubsequence(calls, rule.Pattern) {
			s.Deltas = append(s.Deltas, Delta{RuleID: rule.ID, Score: rule.Score})
			s.Total += rule.Score
		}
	}

	// ExpectSet: all listed tools must appear at least once (any order).
	for _, rule := range r.ExpectSet {
		if matchSet(calls, rule.Tools) {
			s.Deltas = append(s.Deltas, Delta{RuleID: rule.ID, Score: rule.Score})
			s.Total += rule.Score
		}
	}

	// ForbidSet: negative when trigger fires without a required prior tool.
	for _, rule := range r.ForbidSet {
		if applyForbidSet(calls, rule) {
			s.Deltas = append(s.Deltas, Delta{RuleID: rule.ID, Score: rule.Score})
			s.Total += rule.Score
		}
	}

	// Receipts: positive when trigger fires with non-empty receipts in args.
	for _, rule := range r.Receipts {
		if applyReceiptsRule(calls, rule) {
			s.Deltas = append(s.Deltas, Delta{RuleID: rule.ID, Score: rule.Score})
			s.Total += rule.Score
		}
	}

	return s
}

// matchSubsequence returns true if events contains all PatternSteps as an
// ordered subsequence (intervening events are allowed). Greedy left-to-right.
func matchSubsequence(events []trace.Event, pattern []PatternStep) bool {
	if len(pattern) == 0 {
		return true
	}
	pi := 0
	for _, ev := range events {
		if stepMatches(ev, pattern[pi]) {
			pi++
			if pi == len(pattern) {
				return true
			}
		}
	}
	return false
}

// stepMatches returns true if ev matches the PatternStep p.
// Tool must match exactly. ArgsMatch checks substring on ArgsSummary.
// ArgsMatchRegex applies the compiled regex.
func stepMatches(ev trace.Event, p PatternStep) bool {
	if ev.Tool != p.Tool {
		return false
	}
	for _, v := range p.ArgsMatch {
		if !strings.Contains(ev.ArgsSummary, v) {
			return false
		}
	}
	for _, re := range p.ArgsMatchRegex {
		if !re.MatchString(ev.ArgsSummary) {
			return false
		}
	}
	return true
}

// matchSet returns true if every tool in tools appears at least once in events.
func matchSet(events []trace.Event, tools []string) bool {
	seen := make(map[string]bool, len(tools))
	for _, ev := range events {
		seen[ev.Tool] = true
	}
	for _, tool := range tools {
		if !seen[tool] {
			return false
		}
	}
	return true
}

// applyForbidSet returns true when the trigger tool fires AND none of the
// required prior tools appeared before the trigger. When true, the negative
// score should be applied.
func applyForbidSet(events []trace.Event, rule ForbidSetRule) bool {
	// Find first occurrence of the trigger tool.
	triggerIdx := -1
	for i, ev := range events {
		if ev.Tool == rule.When.ToolUsed {
			triggerIdx = i
			break
		}
	}
	if triggerIdx < 0 {
		// Trigger never fired — rule doesn't apply.
		return false
	}

	// Check whether any required prior tool appeared before the trigger.
	for _, ev := range events[:triggerIdx] {
		for _, required := range rule.RequirePrior.AnyOf {
			if ev.Tool == required {
				// Required prior found — no negative score.
				return false
			}
		}
	}

	// Trigger fired but no required prior tool preceded it.
	return true
}

// applyReceiptsRule returns true when the trigger tool fires with non-empty
// receipts in its args_summary. Looks for the pattern receipts:[ followed by
// at least one quoted receipt ID.
func applyReceiptsRule(events []trace.Event, rule ReceiptRule) bool {
	for _, ev := range events {
		if ev.Tool != rule.When.ToolUsed {
			continue
		}
		if rule.Require.ReceiptsNonEmpty {
			// Check for receipts:["..."] — at least one quoted ID after the colon bracket.
			idx := strings.Index(ev.ArgsSummary, "receipts:[")
			if idx < 0 {
				return false
			}
			rest := ev.ArgsSummary[idx+len("receipts:["):]
			// Non-empty means there is at least one quoted character before the closing bracket.
			closingIdx := strings.Index(rest, "]")
			if closingIdx <= 0 {
				return false
			}
			inner := strings.TrimSpace(rest[:closingIdx])
			return inner != ""
		}
		// ReceiptsNonEmpty is false — just the presence of the tool call matches.
		return true
	}
	return false
}
