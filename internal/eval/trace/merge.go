package trace

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agenthands/helix/internal/eval/budget"
)

// MergeInput is the full set of inputs required to build a MergedTrace.
type MergeInput struct {
	// Metadata
	TaskID        string
	Mode          string
	RunID         string
	ClaudeVersion string
	HelixVersion  string
	StartedAt     time.Time
	EndedAt       time.Time

	// Evidence streams
	Daemon DaemonTapResult
	CC     CCTapResult

	// Outcome resolution
	// VerifyExitCode is the exit code of verify.sh; 0 = pass, non-zero = fail.
	VerifyExitCode int
	// Budget is non-nil when a D-08 cap was breached; its String() method
	// returns the canonical "failed-with-cause: budget_<axis>" wording.
	Budget *budget.BreachReason

	// Security invariants
	// PatchPaths are the file paths the agent modified; checked against RepoRoot
	// to enforce the path-prefix invariant (T-67-02).
	PatchPaths []string
	// RepoRoot is the canonical repo directory for path-prefix checks.
	// Paths in PatchPaths must resolve to children of RepoRoot.
	RepoRoot string
}

// Merge combines daemon and CC evidence streams into a single canonical MergedTrace.
// It enforces the path-prefix invariant (T-67-02) and resolves the outcome
// with the priority: budget breach > verify exit code.
//
// Single-host wall-clock alignment invariant: Merge assumes both daemon and CC
// subprocesses run on the same host, so their clocks are colocated. Cross-host
// expansion would require correlation IDs (out of scope; documented per
// RESEARCH §"Trace Merge Schema" Pitfall-5 acceptance disposition).
func Merge(in MergeInput) (MergedTrace, error) {
	mt := MergedTrace{
		SchemaVersion: "1",
		TaskID:        in.TaskID,
		Mode:          in.Mode,
		RunID:         in.RunID,
		ClaudeVersion: in.ClaudeVersion,
		HelixVersion:  in.HelixVersion,
		StartedAt:     in.StartedAt,
		EndedAt:       in.EndedAt,
		DurationMs:    int64(in.EndedAt.Sub(in.StartedAt) / time.Millisecond),
		Usage:         in.CC.Usage,
		UsagePresent:  in.CC.UsagePresent,
	}

	// Step 1: Concatenate and stable-sort all events by t ASC.
	// On tie, daemon source wins (wall-clock ambiguity in the <1ms window
	// between CC tool_use emit and daemon tool_call dispatch).
	all := make([]Event, 0, len(in.Daemon.Events)+len(in.CC.Events))
	all = append(all, in.Daemon.Events...)
	all = append(all, in.CC.Events...)

	sort.SliceStable(all, func(i, j int) bool {
		ti := all[i].T
		tj := all[j].T
		if ti.Equal(tj) {
			// Tie-break: daemon wins (daemon source < cc alphabetically,
			// but we use explicit comparison for clarity and correctness).
			if all[i].Source == "daemon" && all[j].Source != "daemon" {
				return true
			}
			if all[j].Source == "daemon" && all[i].Source != "daemon" {
				return false
			}
			return false // stable: keep original order on full tie
		}
		return ti.Before(tj)
	})
	mt.Events = all

	// Step 2: Build ToolCallSummary from daemon tool_call events.
	summary := ToolCallSummary{
		ByTool:    make(map[string]int),
		ByOutcome: make(map[string]int),
	}
	var guardrails GuardrailCounts
	for _, ev := range all {
		if ev.Source == "daemon" && ev.Kind == KindToolCall {
			summary.Total++
			if ev.Tool != "" {
				summary.ByTool[ev.Tool]++
			}
			if ev.Outcome != "" {
				summary.ByOutcome[ev.Outcome]++
			}
			// Step 3: Build GuardrailCounts from outcomes.
			switch ev.Outcome {
			case "guardrail_warned":
				guardrails.Warned++
			case "guardrail_blocked":
				guardrails.Blocked++
			}
		}
		// F-08: receipt-issued events feed ReceiptsIssued counter.
		if ev.Source == "daemon" && ev.Kind == KindReceiptIssued {
			guardrails.ReceiptsIssued++
		}
	}
	mt.ToolCallSummary = summary
	mt.Guardrails = guardrails

	// Step 4: Path-prefix invariant (T-67-02 mitigation).
	// Every patch path must resolve to a child of RepoRoot.
	if len(in.PatchPaths) > 0 && in.RepoRoot != "" {
		for _, p := range in.PatchPaths {
			// Resolve to absolute if relative; filepath.Abs uses cwd but
			// patch paths from the agent should already be absolute.
			abs := p
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(in.RepoRoot, p)
			}
			rel, err := filepath.Rel(in.RepoRoot, abs)
			if err != nil || strings.HasPrefix(rel, "..") {
				mt.Outcome = "failed"
				mt.FailureReason = "patch_outside_repo"
				return mt, fmt.Errorf("patch path %q resolves outside repo root %q (T-67-02)", p, in.RepoRoot)
			}
		}
	}

	// Step 5: Outcome resolution (priority: budget breach > verify exit code).
	if in.Budget != nil {
		// D-08 wording: "failed-with-cause: budget_<axis>"
		mt.Outcome = in.Budget.String()
	} else if in.VerifyExitCode != 0 {
		mt.Outcome = "failed"
	} else {
		mt.Outcome = "success"
	}

	return mt, nil
}
