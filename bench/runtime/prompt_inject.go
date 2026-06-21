package runtime

import (
	"hash/fnv"

	"github.com/agenthands/helix/bench/canary"
)

// prompt_inject.go is the INFRA-05 PRODUCTION caller of the contamination canary
// (RESEARCH Open Q5): the score-time probe (bench/canary) shipped Sentinel /
// InjectPrompt / IsContaminated in Phase 86 but had NO production injector — nothing
// actually embedded the sentinel into real task prompts, so a contaminated model had
// nothing to echo. This file closes that gap with a PURE, deterministic selector that
// injects canary.Sentinel into a SELECT subset of task prompts before dispatch, so a
// model that memorised the prompt echoes the opaque marker into its completion (where
// rowCanary / IsContaminated already read it at score time and the aggregator excludes
// the cell from the headline).
//
// Design invariants:
//   - SINGLE SOURCE OF TRUTH: the sentinel is embedded ONLY via canary.InjectPrompt;
//     the literal is NEVER re-derived here (anti-pattern T-89-02-03 — injector/detector
//     divergence would silently disable the canary).
//   - PURE: selection is a deterministic function of the task key alone — NO clock, NO
//     RNG, NO os.Environ, NO map-iteration order. Same key -> same verdict, always, so
//     fixtures and reports stay byte-stable.
//   - SELECT subset: only every Kth task (by a stable hash bucket) is injected, so the
//     canary samples the matrix without contaminating every prompt.

// canaryInjectEveryK is the FIXED selection stride: a task is selected for canary
// injection iff its stable hash bucket is 0 mod K. K=7 samples roughly 1/7 of tasks —
// dense enough to catch a contaminated model, sparse enough to leave most prompts
// untouched. It is a documented constant (not a tunable / env var) so selection is
// reproducible across runs and machines.
const canaryInjectEveryK = 7

// canarySelected reports whether the task identified by taskKey is selected for canary
// injection. Selection is a PURE function of taskKey: an FNV-1a hash of the key mod K,
// selected iff the remainder is 0. FNV-1a is deterministic and stdlib (no new dep), so
// the same key always lands in the same bucket on every machine. The empty key hashes
// like any other string (FNV-1a offset basis) — it is not special-cased.
func canarySelected(taskKey string) bool {
	h := fnv.New32a()
	// Write never returns an error for fnv; the bytes are fully consumed.
	_, _ = h.Write([]byte(taskKey))
	return h.Sum32()%canaryInjectEveryK == 0
}

// InjectCanaryIfSelected is the production injection hook: for a task SELECTED by the
// deterministic canarySelected predicate it returns canary.InjectPrompt(prompt) (the
// original prompt verbatim with the opaque sentinel marker appended); for any other
// task it returns prompt UNCHANGED (byte-identical to the input). It is pure and
// idempotent in the sense the plan requires: the SELECTION verdict is stable across
// calls, so the same (taskKey, prompt) always yields the same output. The sentinel is
// referenced ONLY through canary.InjectPrompt — never re-derived here.
func InjectCanaryIfSelected(taskKey, prompt string) string {
	if !canarySelected(taskKey) {
		return prompt
	}
	return canary.InjectPrompt(prompt)
}
