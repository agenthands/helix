package swebench

import (
	"testing"

	"github.com/agenthands/helix/bench/runtime"
)

// TestRescore proves the raw-upstream verdict and the UTBoost-rescored 3-condition
// verdict are surfaced side-by-side as DISTINCT pointers (VERIFIED-02): raw =
// canonical.resolved; rescored = the 3-condition gate over canonical+augmented. A
// missing augmented report fails the rescored verdict CLOSED (&false) while leaving
// raw untouched.
func TestRescore(t *testing.T) {
	const iid = "sympy__sympy-20590"

	canonical := loadEval(t, "report.buggy_canonical.json", iid) // resolved=true
	augmented := loadEval(t, "report.buggy_utboost.json", iid)   // augmented FTP failure

	t.Run("SC#2 divergence: raw=true, rescored=false, distinct pointers", func(t *testing.T) {
		r := Rescore(canonical, &augmented)
		if r.RawVerdict == nil || *r.RawVerdict != true {
			t.Fatalf("RawVerdict = %v, want &true (canonical.resolved)", r.RawVerdict)
		}
		if r.RescoredVerdict == nil || *r.RescoredVerdict != false {
			t.Fatalf("RescoredVerdict = %v, want &false (augmented FTP failure)", r.RescoredVerdict)
		}
		if r.RawVerdict == r.RescoredVerdict {
			t.Fatal("RawVerdict and RescoredVerdict must be distinct pointers")
		}
	})

	t.Run("all-pass augmented: raw=true, rescored=true", func(t *testing.T) {
		clean := loadEval(t, "report.canonical.json", iid)
		aug := loadEval(t, "report.utboost.json", iid)
		r := Rescore(clean, &aug)
		if r.RawVerdict == nil || !*r.RawVerdict {
			t.Errorf("RawVerdict = %v, want &true", r.RawVerdict)
		}
		if r.RescoredVerdict == nil || !*r.RescoredVerdict {
			t.Errorf("RescoredVerdict = %v, want &true", r.RescoredVerdict)
		}
	})

	t.Run("missing augmented: rescored fail-closed &false, raw unaffected", func(t *testing.T) {
		r := Rescore(canonical, nil)
		if r.RawVerdict == nil || *r.RawVerdict != true {
			t.Errorf("RawVerdict = %v, want &true (unaffected by abstain)", r.RawVerdict)
		}
		if r.RescoredVerdict == nil || *r.RescoredVerdict != false {
			t.Errorf("RescoredVerdict = %v, want explicit &false (fail-closed)", r.RescoredVerdict)
		}
	})
}

// TestRescore_ApplyToRow proves the result-row PRODUCER stamps BOTH pinned open
// doc keys onto a runtime.ResultInput via the shared runtime.*Key consts (never a
// literal string) — the LIVE-run wiring the Plan 04 aggregator reads.
func TestRescore_ApplyToRow(t *testing.T) {
	const iid = "sympy__sympy-20590"
	canonical := loadEval(t, "report.buggy_canonical.json", iid)
	augmented := loadEval(t, "report.buggy_utboost.json", iid)

	in := &runtime.ResultInput{Benchmark: "swe-bench-verified", TaskID: iid}
	Rescore(canonical, &augmented).ApplyToRow(in)

	if in.SwebenchRawResolved == nil || *in.SwebenchRawResolved != true {
		t.Errorf("SwebenchRawResolved = %v, want &true", in.SwebenchRawResolved)
	}
	if in.SwebenchRescoredVerified == nil || *in.SwebenchRescoredVerified != false {
		t.Errorf("SwebenchRescoredVerified = %v, want &false", in.SwebenchRescoredVerified)
	}
}
