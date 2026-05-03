package pipelines

import (
	"github.com/agenthands/helix/internal/phasegraph"
)

// Eval phase IDs (SPEC §39.7, verbatim).
const (
	PhasePrepareWorkspace   phasegraph.PhaseID = "prepare_workspace"
	PhaseConfigureMode      phasegraph.PhaseID = "configure_mode"
	PhaseRunAgent           phasegraph.PhaseID = "run_agent"
	PhaseCollectTrace       phasegraph.PhaseID = "collect_trace"
	PhaseApplyPatchCheck    phasegraph.PhaseID = "apply_patch_check"
	PhaseRunTests           phasegraph.PhaseID = "run_tests"
	PhaseRunDiagnostics     phasegraph.PhaseID = "run_diagnostics"
	PhaseScoreToolBehavior  phasegraph.PhaseID = "score_tool_behavior"
	PhaseScoreGuardrails    phasegraph.PhaseID = "score_guardrails"
	PhaseAggregateReport    phasegraph.PhaseID = "aggregate_report"
)

// EvalPhases ships the SHAPE only — Run bodies are noopRun placeholders that
// Phase 67 (eval harness) replaces with real implementations (DAG-02
// contract).
//
// Chain (linear, per SPEC §39.7):
//
//	prepare_workspace → configure_mode → run_agent → collect_trace
//	→ apply_patch_check → run_tests → run_diagnostics → score_tool_behavior
//	→ score_guardrails → aggregate_report
var EvalPhases = []phasegraph.PhaseSpec{
	{ID: PhasePrepareWorkspace, Requires: nil, Provides: []string{"workspace"}, Run: noopRun},
	{ID: PhaseConfigureMode, Requires: []phasegraph.PhaseID{PhasePrepareWorkspace}, Provides: []string{"mode_config"}, Run: noopRun},
	{ID: PhaseRunAgent, Requires: []phasegraph.PhaseID{PhaseConfigureMode}, Provides: []string{"agent_run"}, Run: noopRun},
	{ID: PhaseCollectTrace, Requires: []phasegraph.PhaseID{PhaseRunAgent}, Provides: []string{"trace"}, Run: noopRun},
	{ID: PhaseApplyPatchCheck, Requires: []phasegraph.PhaseID{PhaseCollectTrace}, Provides: []string{"patch_result"}, Run: noopRun},
	{ID: PhaseRunTests, Requires: []phasegraph.PhaseID{PhaseApplyPatchCheck}, Provides: []string{"test_result"}, Run: noopRun},
	{ID: PhaseRunDiagnostics, Requires: []phasegraph.PhaseID{PhaseRunTests}, Provides: []string{"diag_result"}, Run: noopRun},
	{ID: PhaseScoreToolBehavior, Requires: []phasegraph.PhaseID{PhaseRunDiagnostics}, Provides: []string{"tool_behavior_score"}, Run: noopRun},
	{ID: PhaseScoreGuardrails, Requires: []phasegraph.PhaseID{PhaseScoreToolBehavior}, Provides: []string{"guardrails_score"}, Run: noopRun},
	{ID: PhaseAggregateReport, Requires: []phasegraph.PhaseID{PhaseScoreGuardrails}, Provides: []string{"report"}, Run: noopRun},
}
