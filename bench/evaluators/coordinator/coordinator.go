// Package coordinator owns the Phase 79 metric coordinator (D-07): the single
// seam that fans the artifacts of one bench cell out to all five graders,
// captures each grader's per-metric failure annotation, and assembles the full
// nullable evaluators.Metrics record the runtime marshals into result.v2.json.
//
// It lives in its own package (not package evaluators) because the five grader
// subpackages — test_runner, patch_validator, token_meter, tool_trace_analyzer,
// regression_checker — all import the parent evaluators package for the
// Metrics/MetricError contract; a coordinator in package evaluators that
// imported those graders would form an import cycle. coordinator imports both
// the contract and the graders, which is acyclic.
//
// D-07 per-metric failure isolation is the load-bearing invariant: Grade NEVER
// returns a Go error and NEVER short-circuits. Every grader runs; a grader that
// cannot compute a metric leaves that metric's pointer nil and appends a
// MetricError naming the metric + grader. The full Metrics value (with whatever
// nils) is always returned so the (task, mode, run_index) row is always emitted
// and stays schema-valid.
package coordinator

import (
	"context"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/evaluators/patch_validator"
	"github.com/agenthands/helix/bench/evaluators/regression_checker"
	"github.com/agenthands/helix/bench/evaluators/test_runner"
	"github.com/agenthands/helix/bench/evaluators/token_meter"
	"github.com/agenthands/helix/bench/evaluators/tool_trace_analyzer"
	"github.com/agenthands/helix/bench/languages"
	"github.com/agenthands/helix/internal/eval/trace"
)

// GradeInput is the set of per-cell artifacts the runtime hands the coordinator.
// It carries everything the five graders need to fan out, sourced from RunCell:
// the post-patch + pre-patch test outcomes, the cell repo working tree, the
// already-merged trace (NEVER re-merged — METRIC-06), the out-of-band
// usagePresent signal (D-01/D-02), the agent kind, and the optional pre-patch
// compile-error count.
type GradeInput struct {
	// TestOutcome is the post-patch structured test outcome (the success oracle).
	TestOutcome languages.TestOutcome
	// PrePatchOutcome is the pre-patch test outcome snapshot captured BEFORE the
	// agent edited the repo (D-05 / Pitfall 6); regression_checker's denominator.
	PrePatchOutcome languages.TestOutcome
	// RepoDir is the cell's repo working tree (the git-tracked denominator for
	// patch_validator's edit_locality + edit_distance_patch).
	RepoDir string
	// Merged is RunCell's already-merged 2-leg trace. The trace graders consume it
	// as a pure transform; they MUST NOT call trace.Merge again (METRIC-06).
	Merged trace.MergedTrace
	// UsagePresent is the out-of-band signal that a provider usage block was
	// parsed (agent kind == claude AND a CC result event carried usage). When
	// false (the scripted Go-ToolBench corpus) token metrics are explicit null
	// rather than fabricated zeros (D-01).
	UsagePresent bool
	// Agent is the drive-leg kind ("scripted" | "claude"); informational here.
	Agent string
	// CompileErrorsBefore is the optional pre-patch compile-error count, threaded
	// into test_runner. nil when no pre-patch outcome was captured.
	CompileErrorsBefore *int
}

// Grade runs all five graders over in and assembles the full nullable Metrics
// record plus the per-grader failure annotations (D-07). It NEVER returns a Go
// error and NEVER short-circuits: a grader failure nulls only that grader's
// metric(s) and appends a MetricError; every other metric still populates and
// the full Metrics value is always returned.
func Grade(ctx context.Context, in GradeInput) (evaluators.Metrics, []evaluators.MetricError) {
	var m evaluators.Metrics
	var errs []evaluators.MetricError

	// --- test_runner: task_success / verified_correctness / compile_errors_*.
	// The pre-patch outcome feeds compile_errors_before; we pass a pointer to the
	// captured PrePatchOutcome so its compile-error count is derivable.
	pre := in.PrePatchOutcome
	tr := test_runner.Grade(in.TestOutcome, &pre)
	m.TaskSuccess = tr.TaskSuccess
	m.VerifiedCorrectness = tr.VerifiedCorrectness
	m.CompileErrorsBefore = tr.CompileErrorsBefore
	m.CompileErrorsAfter = tr.CompileErrorsAfter
	// Prefer the caller-supplied CompileErrorsBefore when test_runner could not
	// derive one (e.g. no parsed pre-patch rows) but the caller knows it.
	if m.CompileErrorsBefore == nil && in.CompileErrorsBefore != nil {
		m.CompileErrorsBefore = in.CompileErrorsBefore
	}
	errs = append(errs, tr.Errs...)

	// --- patch_validator: files_modified + edit_locality.
	// EditLocality may return a computable files_modified count TOGETHER with a
	// locality MetricError (the zero-tracked-files case, where the locality
	// denominator is undefined but the modified count is still known). Assign
	// files_modified whenever the grader produced a value, independent of the
	// locality error — D-07 nulls only the metric a grader could not compute, and
	// files_modified is a separate, fully-computed metric that must not be nulled
	// as collateral just because it shares a return tuple with edit_locality.
	// When files_modified itself is uncomputable (modified == nil, e.g. git
	// failed), the locErr is appended below so it carries a metric_errors[] entry
	// rather than being silently null (MD-02).
	loc, modified, locErr := patch_validator.EditLocality(ctx, in.RepoDir)
	if modified != nil {
		m.FilesModified = modified
	}
	if locErr != nil {
		errs = append(errs, *locErr)
		// If files_modified itself could not be computed (e.g. git absent / not a
		// repo, where EditLocality returns a nil modified count), the locality
		// error names only edit_locality — files_modified would otherwise be
		// silently null with no annotation. Emit a dedicated files_modified
		// metric_errors[] entry so every nulled metric carries its own provenance
		// (D-07 / MD-02). When modified IS computable (zero-tracked path), no extra
		// annotation is needed: the value is reported.
		if modified == nil {
			errs = append(errs, evaluators.MetricError{
				Metric: "files_modified",
				Grader: "patch_validator",
				Reason: locErr.Reason,
			})
		}
	} else {
		m.EditLocality = loc
	}

	// --- patch_validator: edit_distance_patch.
	dist, distErr := patch_validator.EditDistancePatch(ctx, in.RepoDir)
	if distErr != nil {
		errs = append(errs, *distErr)
	} else {
		m.EditDistancePatch = dist
	}

	// --- regression_checker: regression_rate (pre/post double-run).
	rr, rrErr := regression_checker.RegressionRate(in.PrePatchOutcome, in.TestOutcome)
	if rrErr != nil {
		errs = append(errs, *rrErr)
	} else {
		m.RegressionRate = rr
	}

	// --- token_meter: the 4 token metrics, sourced ONLY from the provider usage
	// block; all-nil + a MetricError when usage is absent (D-01/D-02/D-03).
	in0, out0, cachedRead, cacheWrite, tokErrs := token_meter.MeterTokens(in.Merged, in.UsagePresent)
	m.TokensInput = in0
	m.TokensOutput = out0
	m.TokensInputCachedRead = cachedRead
	m.TokensInputCacheWrite = cacheWrite
	errs = append(errs, tokErrs...)

	// --- tool_trace_analyzer: the 7 trace-shaped metrics, a pure transform over
	// the already-merged trace (no re-merge — METRIC-06).
	tm, traceErrs := tool_trace_analyzer.Analyze(in.Merged)
	m.ToolCalls = tm.ToolCalls
	m.WallTimeSeconds = tm.WallTimeSeconds
	m.FilesRead = tm.FilesRead
	m.BytesRead = tm.BytesRead
	m.LSPDiagnosticsUsed = tm.LSPDiagnosticsUsed
	m.SemanticToolCalls = tm.SemanticToolCalls
	m.RetryCount = tm.RetryCount
	errs = append(errs, traceErrs...)

	return m, errs
}
