package judge

import (
	"time"

	"github.com/agenthands/helix/internal/eval/trace"
)

// LabeledFixture is a (trace, expected-judge-scores, heuristic-verdict) tuple
// used by the offline calibration test. The trace simulates a run; the
// HeuristicVerdict field is the ground-truth label we expect the heuristic
// scorer to produce on that trace; ExpectedJudgeScores is the canned response
// the httptest fake Anthropic server returns for that fixture.
//
// "Good" fixtures use the canonical tool sequence (find_references → rename_symbol
// → verify_edit) and earn HeuristicVerdict=true. "Bad" fixtures use grep-based
// patterns (search_for_pattern → replace_in_file) and earn HeuristicVerdict=false.
type LabeledFixture struct {
	TaskID              string
	Mode                string
	Kind                string
	Events              []trace.Event
	HeuristicVerdict    bool
	ExpectedJudgeScores Scores
}

// BuiltInLabeledFixtures returns 10 hand-authored labeled fixtures: 5 "good"
// (correct semantic-tool sequence) and 5 "bad" (grep-and-replace anti-pattern).
// All fixtures use the rename family so a single rule set suffices.
func BuiltInLabeledFixtures() []LabeledFixture {
	now := time.Now()
	mkEvent := func(tool, args string) trace.Event {
		return trace.Event{
			T:           now,
			Source:      "daemon",
			Kind:        trace.KindToolCall,
			Tool:        tool,
			ArgsSummary: args,
			Outcome:     "success",
		}
	}
	good := func(symbol string) []trace.Event {
		return []trace.Event{
			mkEvent("find_references", "symbol="+symbol),
			mkEvent("rename_symbol", "old_name="+symbol+" new_name=NewName"),
			mkEvent("verify_edit", "path=main.go"),
		}
	}
	bad := func(symbol string) []trace.Event {
		return []trace.Event{
			mkEvent("search_for_pattern", "pattern="+symbol),
			mkEvent("replace_in_file", "find_regex="+symbol+" replace=NewName"),
		}
	}
	allOnes := Scores{RightTool: 1, Evidence: 1, BlastRadius: 1, Recovery: 1}
	allZeros := Scores{RightTool: 0, Evidence: 0, BlastRadius: 0, Recovery: 0}

	return []LabeledFixture{
		{TaskID: "calib-good-1", Mode: "native", Kind: "rename", Events: good("Alpha"), HeuristicVerdict: true, ExpectedJudgeScores: allOnes},
		{TaskID: "calib-good-2", Mode: "native", Kind: "rename", Events: good("Beta"), HeuristicVerdict: true, ExpectedJudgeScores: allOnes},
		{TaskID: "calib-good-3", Mode: "native", Kind: "rename", Events: good("Gamma"), HeuristicVerdict: true, ExpectedJudgeScores: allOnes},
		{TaskID: "calib-good-4", Mode: "semantic", Kind: "rename", Events: good("Delta"), HeuristicVerdict: true, ExpectedJudgeScores: allOnes},
		{TaskID: "calib-good-5", Mode: "semantic", Kind: "rename", Events: good("Epsilon"), HeuristicVerdict: true, ExpectedJudgeScores: allOnes},
		{TaskID: "calib-bad-1", Mode: "baseline", Kind: "rename", Events: bad("Zeta"), HeuristicVerdict: false, ExpectedJudgeScores: allZeros},
		{TaskID: "calib-bad-2", Mode: "baseline", Kind: "rename", Events: bad("Eta"), HeuristicVerdict: false, ExpectedJudgeScores: allZeros},
		{TaskID: "calib-bad-3", Mode: "baseline", Kind: "rename", Events: bad("Theta"), HeuristicVerdict: false, ExpectedJudgeScores: allZeros},
		{TaskID: "calib-bad-4", Mode: "native", Kind: "rename", Events: bad("Iota"), HeuristicVerdict: false, ExpectedJudgeScores: allZeros},
		{TaskID: "calib-bad-5", Mode: "native", Kind: "rename", Events: bad("Kappa"), HeuristicVerdict: false, ExpectedJudgeScores: allZeros},
	}
}
