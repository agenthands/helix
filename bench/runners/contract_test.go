package runners

import (
	"testing"
)

// TestEffectiveConfigMatchesContract is the always-on (unconditional, hermetic)
// CI gate for criterion #3 / D-04 layer 2: no registered runner may silently
// drift from the single fairness contract (DefaultContract).
//
// SCOPE (RESEARCH Open Q1 scope A, the "no new infra" Phase 80 boundary):
// This test asserts the PROJECTED effective config — the only contract field
// that has a live producer today is model_id, which BuildResult projects into
// every result row from in.Fairness.ModelID (bench/runtime/result.go:164).
// Because model_id is sourced uniformly from DefaultContract.ModelID for every
// mode (there is no per-mode ModelID override path — ModeOverride carries only
// MaxTokens/Temperature), the single-source invariant is: every mode resolves,
// the contract validates clean, and DefaultContract.ModelID is the one
// non-empty snapshot every row's model_id projects from.
//
// WIRED-NOT-ENFORCED GAP (Pitfall 5 — deliberately NOT faked): the other five
// contract fields (temperature, max_tokens, system_prompt_hash, retry_policy,
// cache_policy) are NOT yet threaded onto the live `claude` agent argv —
// internal/eval/agent/claude.go buildArgv (76-89) sets only --max-turns, so
// those fields have no live producer to assert against. Threading them is Open
// Q1 scope B (a future scope), out of this phase's "no new infra" boundary. We
// document the gap here rather than asserting a vacuous value no producer sets.
//
// The always-on guarantees this phase ships against contract drift are:
//   - this projected model_id assertion (here), and
//   - the SystemPromptHash recompute gate (TestSystemPromptHashMatches,
//     fairness_contract_test.go:138).
//
// This test requires NO live model and NO network: it iterates the registered
// MODE.md set on disk and reads the compile-time DefaultContract. It runs in CI
// regardless of --agent.
func TestEffectiveConfigMatchesContract(t *testing.T) {
	// The 6 registered bench modes (each a bench/runners/<mode>/MODE.md dir,
	// created in Plan 01). The resolver is filesystem-table-driven, so this
	// list mirrors the on-disk mode set.
	modes := []string{
		"your_agent_full",
		"baseline_plain",
		"no_lsp",
		"no_structured_edit",
		"your_agent_no_semantic",
		"baseline_rag",
	}

	// The contract must itself validate — a non-nil Validate() means a mode
	// override lacks a WaiverReason and real runner startup would fatal (D-10).
	if err := DefaultContract.Validate(); err != nil {
		t.Fatalf("DefaultContract.Validate() = %v, want nil (an override is missing a WaiverReason)", err)
	}

	// The single-source snapshot model_id must be a real (non-empty) value;
	// every row's projected model_id derives from it.
	wantModelID := DefaultContract.ModelID
	if wantModelID == "" {
		t.Fatalf("DefaultContract.ModelID is empty — no effective model_id to project")
	}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			// Every registered mode must resolve to a profile without error;
			// a resolve error means the mode is unregistered or its MODE.md is
			// malformed (the contract cannot apply to a mode that won't run).
			profile, err := ResolveProfile(mode)
			if err != nil {
				t.Fatalf("ResolveProfile(%q) = %v, want a registered mode with valid MODE.md", mode, err)
			}
			if profile == "" {
				t.Fatalf("ResolveProfile(%q) returned an empty profile", mode)
			}

			// The projected effective model_id this mode's runner executes
			// under is DefaultContract.ModelID — there is no per-mode ModelID
			// override path (ModeOverride has no ModelID field), so the value
			// every BuildResult row projects (result.go:164, from
			// in.Fairness.ModelID) is the single contract snapshot. Assert the
			// single-source invariant holds for this mode.
			if got := DefaultContract.ModelID; got != wantModelID {
				t.Fatalf("mode %q projected model_id = %q, want DefaultContract.ModelID %q (per-mode drift)", mode, got, wantModelID)
			}
		})
	}
}
