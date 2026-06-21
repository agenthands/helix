package swebench

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
)

// loadEval reads a testdata report fixture and returns the single InstanceEval
// under iid. It is a test helper for the hermetic gate proofs (no Docker, no
// HELIX_BIN — the committed fixtures are the SOLE authoritative proof per the
// VERIFIED-01 contract).
func loadEval(t *testing.T, name, iid string) InstanceEval {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	rep, err := ParseInstanceReport(b)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	ev, ok := rep[iid]
	if !ok {
		t.Fatalf("instance %q not in %s", iid, name)
	}
	return ev
}

// ★ SC#2 LOAD-BEARING hermetic proof (the reason this phase exists): a
// known-buggy patch that passes ONLY the canonical FAIL_TO_PASS tests (so the
// upstream harness reports resolved=true → task_success=true) is caught FALSE by
// the 3-condition verified_correctness gate because the UTBoost-augmented report
// has a FAIL_TO_PASS failure. task_success=true AND verified_correctness=false —
// the divergence proven hermetically, never via a live Docker run.
func TestVerified_BuggyPatch(t *testing.T) {
	const iid = "sympy__sympy-20590"

	canonical := loadEval(t, "report.buggy_canonical.json", iid)
	augmented := loadEval(t, "report.buggy_utboost.json", iid)

	// task_success comes from the canonical report's authoritative resolved bool
	// (the Plan 02 Ingest value) — assigned SEPARATELY from the gate, distinct
	// pointer.
	taskSuccess := canonical.Resolved

	res := Grade(canonical, &augmented)

	if taskSuccess != true {
		t.Fatalf("task_success = %v, want true (buggy patch resolved the canonical issue)", taskSuccess)
	}
	if res.VerifiedCorrectness == nil {
		t.Fatal("verified_correctness must be non-nil (fail-closed), got nil")
	}
	if *res.VerifiedCorrectness != false {
		t.Fatalf("verified_correctness = %v, want false (augmented FAIL_TO_PASS has a failure)", *res.VerifiedCorrectness)
	}

	// The two verdicts must be independent pointers, never aliased.
	if res.VerifiedCorrectness == &taskSuccess {
		t.Fatal("verified_correctness and task_success must be distinct pointers")
	}
}

// TestVerified_Gate exercises the 3-condition gate matrix: all-three-pass →
// &true; any single condition fail → &false; missing augmented oracle → explicit
// non-nil &false (fail-closed abstain, never nil, never a spurious true).
func TestVerified_Gate(t *testing.T) {
	const iid = "sympy__sympy-20590"

	mk := func(canonFTPFail, p2pFail, p2fFail []string, augFTPFail []string, augNil bool) ([]string, GateResult) {
		canonical := InstanceEval{
			Resolved: true,
			TestsStatus: TestsStatus{
				FailToPass: TestList{Success: []string{"ftp_ok"}, Failure: canonFTPFail},
				PassToPass: TestList{Success: []string{"p2p_ok"}, Failure: p2pFail},
				PassToFail: TestList{Failure: p2fFail},
			},
		}
		if augNil {
			return canonFTPFail, Grade(canonical, nil)
		}
		augmented := InstanceEval{
			TestsStatus: TestsStatus{
				FailToPass: TestList{Success: []string{"ftp_ok"}, Failure: augFTPFail},
			},
		}
		return canonFTPFail, Grade(canonical, &augmented)
	}

	want := func(t *testing.T, name string, r GateResult, exp bool) {
		t.Helper()
		if r.VerifiedCorrectness == nil {
			t.Fatalf("%s: verified_correctness nil, want non-nil %v", name, exp)
		}
		if *r.VerifiedCorrectness != exp {
			t.Fatalf("%s: verified_correctness = %v, want %v", name, *r.VerifiedCorrectness, exp)
		}
	}

	t.Run("all three pass -> true", func(t *testing.T) {
		_, r := mk(nil, nil, nil, nil, false)
		want(t, "all-pass", r, true)
		if r.CanonicalPass == nil || !*r.CanonicalPass {
			t.Errorf("CanonicalPass = %v, want &true", r.CanonicalPass)
		}
		if r.AugmentedPass == nil || !*r.AugmentedPass {
			t.Errorf("AugmentedPass = %v, want &true", r.AugmentedPass)
		}
		if r.NoRegress == nil || !*r.NoRegress {
			t.Errorf("NoRegress = %v, want &true", r.NoRegress)
		}
	})

	t.Run("canonical FAIL_TO_PASS fail -> false", func(t *testing.T) {
		_, r := mk([]string{"ftp_broke"}, nil, nil, nil, false)
		want(t, "canonical-fail", r, false)
	})

	t.Run("augmented FAIL_TO_PASS fail -> false", func(t *testing.T) {
		_, r := mk(nil, nil, nil, []string{"aug_broke"}, false)
		want(t, "augmented-fail", r, false)
	})

	t.Run("regress PASS_TO_PASS fail -> false", func(t *testing.T) {
		_, r := mk(nil, []string{"p2p_broke"}, nil, nil, false)
		want(t, "p2p-regress", r, false)
	})

	t.Run("regress PASS_TO_FAIL fail -> false", func(t *testing.T) {
		_, r := mk(nil, nil, []string{"p2f_broke"}, nil, false)
		want(t, "p2f-regress", r, false)
	})

	t.Run("augmented nil abstain -> explicit non-nil false", func(t *testing.T) {
		_, r := mk(nil, nil, nil, nil, true)
		want(t, "abstain", r, false)
		// Fail-closed: per-oracle pointers stay nil; oracles not consulted.
		if r.CanonicalPass != nil || r.AugmentedPass != nil || r.NoRegress != nil {
			t.Errorf("abstain must not consult oracles: canon=%v aug=%v noreg=%v",
				r.CanonicalPass, r.AugmentedPass, r.NoRegress)
		}
	})
}

// TestVerified_ApplyToMetrics proves the additive producer mutates ONLY
// VerifiedCorrectness on the existing Metrics record (clone of
// completion_gate.ApplyToMetrics), leaving TaskSuccess untouched.
func TestVerified_ApplyToMetrics(t *testing.T) {
	ts := true
	m := evaluators.Metrics{TaskSuccess: &ts}

	canonical := InstanceEval{Resolved: true, TestsStatus: TestsStatus{
		FailToPass: TestList{Success: []string{"a"}},
		PassToPass: TestList{Success: []string{"b"}},
	}}
	augmented := InstanceEval{TestsStatus: TestsStatus{
		FailToPass: TestList{Success: []string{"a"}},
	}}

	errs := Grade(canonical, &augmented).ApplyToMetrics(&m)
	if len(errs) != 0 {
		t.Errorf("ApplyToMetrics errs = %v, want none", errs)
	}
	if m.VerifiedCorrectness == nil || !*m.VerifiedCorrectness {
		t.Errorf("VerifiedCorrectness = %v, want &true", m.VerifiedCorrectness)
	}
	if m.TaskSuccess == nil || !*m.TaskSuccess {
		t.Errorf("TaskSuccess must be untouched, got %v", m.TaskSuccess)
	}
	// Distinct pointers — never aliased.
	if m.VerifiedCorrectness == m.TaskSuccess {
		t.Error("VerifiedCorrectness and TaskSuccess must be distinct pointers")
	}
}
