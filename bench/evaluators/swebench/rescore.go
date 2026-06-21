package swebench

import (
	"github.com/agenthands/helix/bench/runtime"
)

// rescore.go is the UTBoost rescorer (VERIFIED-02): a PURE compare of the raw
// upstream verdict against the UTBoost-augmented 3-condition verdict, surfaced
// side-by-side as DISTINCT pointers, plus the result-row PRODUCER (ApplyToRow)
// that stamps both verdicts onto the row's two PINNED open doc keys.
//
//	RawVerdict      = canonical.Resolved (the upstream harness's own verdict)
//	RescoredVerdict = the 3-condition verified_correctness gate over
//	                  (canonical, augmented) — Grade(...)  .VerifiedCorrectness
//
// The two are NEVER aliased (clone of the gate's no-shared-address discipline):
// the headline divergence the Plan 04 aggregator renders side-by-side is the
// SC#2 buggy-patch row where raw=true (upstream said resolved) but rescored=false
// (the UTBoost-augmented tests caught the bug). A MISSING augmented report makes
// RescoredVerdict the gate's fail-closed &false while RawVerdict (sourced only
// from the canonical report) is unaffected.

// RescoreResult bundles the raw and rescored verdicts as distinct pointers.
type RescoreResult struct {
	// RawVerdict is the upstream harness's own resolved verdict (canonical.Resolved).
	RawVerdict *bool
	// RescoredVerdict is the 3-condition verified_correctness gate verdict over the
	// canonical + UTBoost-augmented reports — an explicit fail-closed &false when the
	// augmented report is missing (never nil on that path).
	RescoredVerdict *bool
}

// Rescore composes the raw-upstream verdict and the UTBoost-rescored 3-condition
// verdict over the canonical report and the (possibly nil) augmented report,
// returning both as DISTINCT pointers. raw is sourced ONLY from canonical.Resolved
// (so it is unaffected by a missing augmented report); rescored is the
// Grade(canonical, augmented).VerifiedCorrectness gate verdict, which fails closed
// to &false when augmented is nil.
func Rescore(canonical InstanceEval, augmented *InstanceEval) RescoreResult {
	// Distinct local → distinct pointer; never alias raw with rescored.
	raw := canonical.Resolved
	rescored := Grade(canonical, augmented).VerifiedCorrectness
	return RescoreResult{
		RawVerdict:      &raw,
		RescoredVerdict: rescored,
	}
}

// ApplyToRow is the result-row PRODUCER (clone of completion_gate.ApplyToMetrics's
// additive shape + result.go's additive-open-key discipline): it STAMPS the two
// verdicts onto the result row's PINNED open doc keys —
// runtime.SwebenchRawResolvedKey ← RawVerdict and
// runtime.SwebenchRescoredVerifiedKey ← RescoredVerdict — via the ResultInput
// fields whose json tags ARE those pinned const VALUES. The SAME pinned consts are
// what Plan 04's rowSwebenchScores reads, so producer and reader cannot drift on
// the spelling (the bench/canary.DocKeyCompletion shared-const precedent — never a
// literal string at the call site, never "agree by comment").
//
// It mutates ONLY the two SWE-bench open keys — no other field, no schema bump
// (schema_version stays "v2", additionalProperties stays OPEN, nothing added to
// required). A nil verdict stamps nothing for that key (the *bool stays nil →
// omitempty drops it), never a fabricated default; a literal false (the SC#2
// rescored verdict — the WHOLE POINT) is PRESERVED because the field is a *bool,
// not a value-type omitempty that would drop a load-bearing false (Pitfall 2).
func (r RescoreResult) ApplyToRow(in *runtime.ResultInput) {
	in.SwebenchRawResolved = r.RawVerdict
	in.SwebenchRescoredVerified = r.RescoredVerdict
}
