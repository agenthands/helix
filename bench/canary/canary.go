// Package canary is the MINIMAL forward-compatible contamination-canary probe
// for the benchmark harness (Phase 86 Plan 05 Task 1). The full contamination
// reporter is a Phase 89 deliverable (RESEARCH Open Q2); this package ships only
// the pure, stdlib-only primitives that the eventual reporter — and the Phase 86
// Plan 05 Task 2 aggregator reduce — build on, named so they are
// forward-compatible.
//
// The probe implements a textbook canary: inject a KNOWN-NOVEL sentinel string
// into select task prompts (InjectPrompt). If a model later emits that sentinel
// verbatim in its completion (IsContaminated), the prompt — and therefore the
// surrounding task — was very likely memorised from training data rather than
// solved, so the completion is flagged canary_contaminated=true. A model that
// genuinely solves the task never echoes the opaque sentinel, so a clean
// completion is NOT flagged (teeth, not a rubber stamp).
//
// The probe is a PURE helper consumed at SCORE TIME: the dataset loaders
// (bench/datasets/crosscodeeval, bench/datasets/repobench) are NOT edited by this
// package; the score-time wiring (the aggregator reduce in Plan 05 Task 2) reads
// each result row's completion and calls IsContaminated to derive the additive
// canary_contaminated doc key. Keeping the probe a leaf package with no bench
// dependencies is what makes that score-time consumption — and the Phase 89
// reporter — possible without a loader edit.
package canary

import "strings"

// Sentinel is the documented KNOWN-NOVEL canary marker. It is a fixed, opaque
// token engineered NOT to occur in any legitimate code completion: an UUID-shaped
// random core wrapped in a self-describing prefix/suffix so a human reading a
// flagged completion immediately understands why it tripped. A model that did not
// memorise the injected prompt has no reason to ever produce this exact string,
// which is precisely what gives IsContaminated its teeth.
//
// FORWARD COMPATIBILITY (Phase 89): this constant is the single source of truth
// for the sentinel. The Phase 89 reporter MUST reference Sentinel rather than
// re-deriving it, so re-pinning the marker stays a one-line change here.
const Sentinel = "HELIX-CANARY-7f1a2c9e-4b6d-4e8a-9c3f-0a1b2c3d4e5f-END"

// DocKeyCompletion is the open-provenance result-row doc key carrying the model's
// raw completion text that the score-time reduce feeds to IsContaminated. It is
// additive (additionalProperties stays OPEN, schema_version stays "v2"); a row
// written without it simply yields no canary signal. Pinned here so the Phase 89
// reporter and the Plan 05 Task 2 aggregator read the SAME key name.
const DocKeyCompletion = "completion"

// DocKeyContaminated is the open-provenance result-row doc key the aggregator
// reduce writes/reads to record the per-row canary verdict (true == the
// completion echoed the sentinel). Pinned here for Phase 89 forward
// compatibility; mirrors the DocKeyCompletion additive-minor discipline.
const DocKeyContaminated = "canary_contaminated"

// InjectPrompt deterministically embeds the canary Sentinel into a task prompt so
// a contaminated model that memorised the prompt would echo the sentinel back. It
// preserves the original prompt verbatim and appends a clearly-delimited canary
// instruction line; the embedding is deterministic (same input -> same output) so
// fixtures stay byte-stable. A real harness injects this into only a SELECT
// subset of prompts; this helper just performs the embedding.
func InjectPrompt(prompt string) string {
	// Append rather than prepend so the original prompt's leading context (often a
	// file header the model conditions on) is untouched. The marker line is opaque
	// to a solver but trivially memorisable to a contaminated model.
	return prompt + "\n// canary: " + Sentinel + "\n"
}

// IsContaminated reports whether completion echoes the canary Sentinel verbatim.
// It returns true IFF the sentinel appears as a substring — a model that solved
// the task without memorising the injected prompt has no reason to emit the
// opaque marker, so a clean (or empty) completion returns false. This is the
// teeth: it is NOT a rubber stamp that always returns one value.
func IsContaminated(completion string) bool {
	if completion == "" {
		return false
	}
	return strings.Contains(completion, Sentinel)
}
