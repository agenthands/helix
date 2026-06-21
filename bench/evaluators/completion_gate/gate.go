// Package completion_gate is the multi-oracle completion verification gate
// (VERIFIED-03): the non-test-bearing sibling producer of verified_correctness
// that bench/evaluators/test_runner/test_runner.go:37-40 explicitly anticipates.
// Where test_runner derives verified_correctness from a TestOutcome exit code,
// THIS gate derives it from agreement across three CrossCodeEval completion
// oracles — exact-match, edit-similarity, and identifier-match — composed from
// the Plan 01 leaf scorers (exactmatch.EM / editsim.ES / identmatch.Match).
//
// # All-three-required (T-86-02-02)
//
// verified_correctness is true if and only if ALL THREE oracles pass:
//
//	EM(pred, gold) == true
//	  AND ES(pred, gold) >= GateConfig.ESThreshold
//	  AND identifier-match(pred, gold) == true
//
// A single passing oracle never masquerades as full verification: any one
// failing oracle drives the composite to an explicit false. The
// edit-similarity threshold is a per-oracle CONFIGURABLE knob (GateConfig.
// ESThreshold), not a hardcoded constant.
//
// # Fail-closed abstain (T-86-02-01, the security-relevant invariant)
//
// When a completion is abstained (low confidence), Grade emits an EXPLICIT
// verified_correctness=&false — a real non-nil *bool pointing to false — and
// does NOT consult the oracles. A false-positive verified_correctness=true on a
// wrong/low-confidence completion is the worst failure a benchmark can produce;
// the gate fails closed to false rather than ever risking a spurious true, and
// never nil-drops the metric. This is distinct from a "could-not-score" path
// (reserved below) where a MetricError is appended and the pointer stays nil —
// that is "unknown", not "verified false".
//
// # Additive completion-path producer (no schema bump)
//
// evaluators.Metrics.VerifiedCorrectness *bool already exists (metrics.go:21).
// This package only adds a completion-path producer for it via
// GateResult.ApplyToMetrics, mirroring the coordinator's non-short-circuiting
// assignment discipline (coordinator.go:77-87). It introduces NO new
// result.v2.schema.json field.
package completion_gate

import (
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/evaluators/editsim"
	"github.com/agenthands/helix/bench/evaluators/exactmatch"
	"github.com/agenthands/helix/bench/evaluators/identmatch"
)

// graderName is stamped into every MetricError this package emits (D-07).
const graderName = "completion_gate"

// DefaultESThreshold is the default per-oracle edit-similarity threshold used
// when a caller does not override GateConfig.ESThreshold. It is documented in
// bench/evaluators/VERIFIED.md.
const DefaultESThreshold = 0.9

// GateConfig carries the per-oracle, caller-tunable knobs for the gate. Today
// the sole knob is ESThreshold — the minimum normalized edit similarity (in
// [0,1]) the edit-similarity oracle must meet to count as passing (VERIFIED-03).
// It is a live field, not a hardcoded constant: the same (pred, gold) pair can
// pass or fail purely by moving this threshold.
type GateConfig struct {
	// ESThreshold is the minimum editsim.ES value (inclusive) for the
	// edit-similarity oracle to pass. Default DefaultESThreshold when zero-valued
	// callers want the documented default; a caller may pass any value in [0,1].
	ESThreshold float64
}

// GateResult is the typed bundle the gate produces. Every oracle metric is a
// nullable pointer mirroring test_runner.Result so a per-oracle outcome is a
// distinct *bool/*float64 (never conflated). Errs carries any per-metric
// failure annotations (D-07); for the present total-on-every-input scorers it is
// empty, but the field preserves the "could-not-score ⇒ nil + MetricError" path
// for future non-total oracles.
type GateResult struct {
	// VerifiedCorrectness is the composite gate verdict: true only when all three
	// oracles pass; an explicit false on any single-oracle failure OR on abstain.
	// Never nil on the abstain/oracle paths (it is nil only on a future
	// could-not-score path, alongside a MetricError).
	VerifiedCorrectness *bool
	// EM / IDMatch / ES are the per-oracle outputs, surfaced so a caller (and the
	// hermetic test) can inspect why the composite verdict landed where it did.
	EM      *bool
	IDMatch *bool
	ES      *float64
	// Errs carries per-metric failure annotations (D-07); empty on the normal
	// oracle/abstain paths.
	Errs []evaluators.MetricError
}

// Grade composes the three CrossCodeEval completion oracles over (pred, gold)
// under cfg and returns the multi-oracle verdict.
//
// When abstain is true the gate fails closed: it returns
// VerifiedCorrectness=&false WITHOUT consulting the oracles (the per-oracle
// pointers stay nil), because a low-confidence completion must never be reported
// as verified — and must never nil-drop the metric (VERIFIED-03 / T-86-02-01).
//
// Otherwise it computes em := exactmatch.EM, es := editsim.ES, id := the IM-EM
// bool from identmatch.Match, and sets verified_correctness to
// (em && es >= cfg.ESThreshold && id) — all-three-required (T-86-02-02). All
// four pointers are populated so the verdict is fully auditable.
func Grade(pred, gold string, cfg GateConfig, abstain bool) GateResult {
	// Fail-closed abstain: explicit false, oracles not consulted, never nil.
	if abstain {
		f := false
		return GateResult{VerifiedCorrectness: &f}
	}

	em := exactmatch.EM(pred, gold)
	es := editsim.ES(pred, gold)
	id, _ := identmatch.Match(pred, gold) // IM-EM bool; IM-F1 not gated on here.

	ok := em && es >= cfg.ESThreshold && id

	// Bind locals so each pointer is independent (no shared-address aliasing).
	emv, idv, okv, esv := em, id, ok, es
	return GateResult{
		VerifiedCorrectness: &okv,
		EM:                  &emv,
		IDMatch:             &idv,
		ES:                  &esv,
	}
}

// ApplyToMetrics is the additive completion-path producer: it assigns the gate's
// verdict onto an existing evaluators.Metrics.VerifiedCorrectness and returns
// any accumulated MetricErrors, mirroring the coordinator's non-short-circuiting
// assignment (coordinator.go:77-87). It mutates ONLY VerifiedCorrectness — no
// other metric, and NO schema field is added. The verdict pointer (which is the
// explicit-false on abstain, true/false on the oracle path, or nil only on a
// future could-not-score path) is carried through verbatim so an abstain's
// explicit false is never silently dropped.
func (r GateResult) ApplyToMetrics(m *evaluators.Metrics) []evaluators.MetricError {
	m.VerifiedCorrectness = r.VerifiedCorrectness
	return r.Errs
}
