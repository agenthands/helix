package swebench

import (
	"github.com/agenthands/helix/bench/evaluators"
)

// verified.go is the 3-condition test-execution verified_correctness gate
// (VERIFIED-01) — a CLONE of bench/evaluators/completion_gate/gate.go's
// GateResult/Grade/ApplyToMetrics shape, swapping the three CrossCodeEval
// completion oracles (EM/ES/IDMatch) for three SWE-bench test-execution oracles
// read DIRECTLY off the harness reports:
//
//	(a) canonicalPass = the canonical report's FAIL_TO_PASS.failure is empty
//	    (all the canonical issue tests pass)
//	(b) augmentedPass = the UTBoost-augmented report is present AND its
//	    FAIL_TO_PASS.failure is empty (the broader augmented issue tests pass)
//	(c) noRegress     = the canonical report's PASS_TO_PASS.failure AND
//	    PASS_TO_FAIL.failure are both empty (no pre-existing test regressed)
//
// verified_correctness is true IFF all three conditions hold. It is computed
// INDEPENDENTLY of task_success — task_success is the upstream report.resolved
// bool (Plan 02 Ingest), this gate is the sole producer of verified_correctness,
// and the two are DISTINCT pointers (the anticipated VERIFIED-01 divergence;
// test_runner.go:37-40 anticipates it). The ★ load-bearing SC#2 hermetic test
// (verified_test.go TestVerified_BuggyPatch) proves a known-buggy patch that
// passes only the canonical tests lands task_success=true AND
// verified_correctness=false.
//
// # Fail-closed abstain (T-87-03-01, the security-relevant invariant)
//
// A MISSING augmented (UTBoost) report is an abstain: Grade returns
// VerifiedCorrectness=&false (a real non-nil *bool to false) WITHOUT consulting
// the oracles, exactly mirroring completion_gate.Grade's abstain branch. A
// false-positive verified_correctness=true is the worst benchmark failure, so the
// gate fails CLOSED to an explicit false rather than ever risking a spurious true
// — and never nil-drops the metric (Pitfall 3). This clones gate.go:104-108.
//
// # Additive producer (no schema bump)
//
// evaluators.Metrics.VerifiedCorrectness *bool already exists (metrics.go:21);
// GateResult.ApplyToMetrics only adds a test-execution-path producer for it,
// mutating ONLY VerifiedCorrectness. NO result.v2.schema.json field is added.

// GateResult is the typed bundle the verified gate produces. Every oracle metric
// is a nullable *bool so a per-condition outcome is a DISTINCT pointer (never
// conflated). Errs carries any per-metric annotations (empty on the normal
// oracle/abstain paths; the could-not-score path is reserved). It mirrors
// completion_gate.GateResult exactly, swapping the three completion oracles for
// the three test-execution conditions.
type GateResult struct {
	// VerifiedCorrectness is the composite verdict: &true only when all three
	// conditions pass; an explicit &false on any single condition failure OR on
	// abstain. Never nil on those paths.
	VerifiedCorrectness *bool
	// CanonicalPass / AugmentedPass / NoRegress are the per-condition outputs,
	// surfaced so a caller (and the hermetic test) can audit why the composite
	// verdict landed where it did. They stay nil on the abstain path (oracles not
	// consulted).
	CanonicalPass *bool
	AugmentedPass *bool
	NoRegress     *bool
	// Errs carries per-metric failure annotations; empty on the normal paths.
	Errs []evaluators.MetricError
}

// passed reports whether a test bucket has no failures (an all-success bucket).
func passed(l TestList) bool { return len(l.Failure) == 0 }

// Grade composes the three test-execution conditions over the canonical report
// and the (possibly nil) augmented report, returning the 3-condition verdict.
//
// When the augmented report is nil the gate ABSTAINS and fails closed: it returns
// VerifiedCorrectness=&false WITHOUT consulting the conditions (the per-condition
// pointers stay nil), because a missing augmented oracle must never be reported as
// verified — and must never nil-drop the metric (T-87-03-01; clones
// gate.go:104-108).
//
// Otherwise it reads:
//
//	canonicalPass = canonical.FAIL_TO_PASS.failure empty
//	augmentedPass = augmented.FAIL_TO_PASS.failure empty
//	noRegress     = canonical.PASS_TO_PASS.failure empty AND
//	                canonical.PASS_TO_FAIL.failure empty
//
// and sets verified_correctness to (canonicalPass && augmentedPass && noRegress)
// — all-three-required. All four pointers are bound to independent locals so no
// pointer aliases another (clones gate.go:129-136).
//
// Grade NEVER touches task_success: the caller assigns task_success separately
// from canonical.Resolved (the Plan 02 Ingest value). The two are distinct
// pointers.
func Grade(canonical InstanceEval, augmented *InstanceEval) GateResult {
	// Fail-closed abstain: a missing augmented oracle → explicit false, conditions
	// not consulted, never nil (T-87-03-01 / Pitfall 3).
	if augmented == nil {
		f := false
		return GateResult{VerifiedCorrectness: &f}
	}

	canonicalPass := passed(canonical.TestsStatus.FailToPass)
	augmentedPass := passed(augmented.TestsStatus.FailToPass)
	noRegress := passed(canonical.TestsStatus.PassToPass) && passed(canonical.TestsStatus.PassToFail)

	ok := canonicalPass && augmentedPass && noRegress

	// Bind locals so each pointer is independent (no shared-address aliasing).
	cv, av, nv, okv := canonicalPass, augmentedPass, noRegress, ok
	return GateResult{
		VerifiedCorrectness: &okv,
		CanonicalPass:       &cv,
		AugmentedPass:       &av,
		NoRegress:           &nv,
	}
}

// ApplyToMetrics is the additive test-execution-path producer: it assigns the
// gate's verdict onto an existing evaluators.Metrics.VerifiedCorrectness and
// returns any accumulated MetricErrors, mirroring completion_gate.ApplyToMetrics.
// It mutates ONLY VerifiedCorrectness — no other metric (task_success in
// particular is left untouched), and NO schema field is added. The verdict pointer
// (explicit-false on abstain, true/false on the oracle path) is carried through
// verbatim so an abstain's explicit false is never silently dropped.
func (r GateResult) ApplyToMetrics(m *evaluators.Metrics) []evaluators.MetricError {
	m.VerifiedCorrectness = r.VerifiedCorrectness
	return r.Errs
}
