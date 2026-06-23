// Package fuzzyrobust is the stdlib-only (+ bench/evaluators/editsim) leaf
// evaluator that measures internal/fuzzy's 4-strategy cascade selection and
// ambiguity refusal against a committed multi-language DRIFT corpus.
//
// LEAF BOUNDARY (FUZZBENCH-01, D-05, Pitfall 1/5): this package imports the Go
// standard library plus EXACTLY ONE in-repo package — bench/evaluators/editsim
// (reused for matched-vs-expected text similarity; never re-implemented). It
// does NOT import internal/fuzzy (which imports internal/errors and is NOT
// stdlib-only — match.go:8), internal/repomap, or bench/runtime. Nothing in the
// build enforces this: vet-ablation-leakage (internal/lint/ablationleakage,
// checkedPkgPrefix = ".../bench/runners") does NOT gate bench/evaluators/*, so
// the TestLeafImports self-test is the SOLE boundary enforcer.
//
// The leaf scores COMMITTED-vs-COMMITTED (the Phase 100 determinism contract):
// the committed drift corpus (each case's EXPECTED strategy derived structurally
// from a deterministic per-tier perturbation — never read from observed tool
// behavior, D-05) against the committed CAPTURED outcomes (recorded OUTSIDE the
// leaf by the //go:build ignore bench/runtime/fuzzy_robust_capture_regen.go
// harness, which is the only code allowed to call internal/fuzzy.Match). The
// hermetic golden sibling (go test ./bench/evaluators/fuzzyrobust/, NO binary,
// NO network) is the sole authoritative proof.
//
// perturb.go holds the deterministic per-tier perturbation transforms used to
// AUTHOR the drift corpus. Each takes a real vendored fixture code block and
// produces a drifted SEARCH block whose EXPECTED strategy IS the transform's
// tier — the transform is the drift-type derivation. The transforms are pure,
// stdlib-only (strings), RNG-free, and time-free, so the corpus is
// byte-reproducible (T-102-06).
package fuzzyrobust

import "strings"

// The expected-strategy / outcome vocabulary. The first four are the live
// internal/fuzzy 4-tier cascade labels (StrategyExact / StrategyWhitespace /
// StrategyIndentationFlex from internal/fuzzy/types.go, plus the ellipsis tier
// reported via the AllowEllipsis path). AmbiguousMatch and NoMatch are the two
// non-strategy outcomes from the fuzzy sentinels (ErrAmbiguous → ambiguous_match,
// ErrNoMatch → no_match — internal/fuzzy/match.go:18,25). "failed" is NEVER a
// strategy label (Pitfall 6).
const (
	// StrategyExact is the identity tier: byte-for-byte equality.
	StrategyExact = "exact"
	// StrategyWhitespace is the whitespace-normalized tier (trailing/leading
	// whitespace drift, TrimSpace per line).
	StrategyWhitespace = "whitespace_normalized"
	// StrategyIndentationFlex is the indentation-flexible tier (leading
	// spaces<->tab / depth shift, TrimLeft(" \t") per line).
	StrategyIndentationFlex = "indentation_flexible"
	// StrategyEllipsis is the ellipsis tier (middle line(s) elided with "..."
	// on its own line, matched via Options.AllowEllipsis).
	StrategyEllipsis = "ellipsis"
	// OutcomeAmbiguous is the ambiguity-refusal outcome (ErrAmbiguous): N>1
	// candidate sites → the cascade refuses rather than silently picking one.
	OutcomeAmbiguous = "ambiguous_match"
	// OutcomeNoMatch is the cascade-exhausted outcome (ErrNoMatch).
	OutcomeNoMatch = "no_match"
)

// PerturbWhitespace produces a whitespace_normalized-tier drift: it appends two
// trailing spaces to every non-empty line and a single leading space to every
// non-blank line, changing ONLY leading/trailing whitespace. The non-whitespace
// content of every line is preserved exactly, so internal/fuzzy's whitespace
// strategy (TrimSpace per line) is the lowest tier that re-matches it. The
// transform is deterministic and byte-reproducible (no RNG, no time).
//
// EXPECTED strategy of the result: StrategyWhitespace (the derivation is the
// transform, never observed behavior — D-05).
func PerturbWhitespace(block string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			// Leave structurally-blank lines blank (no trailing whitespace) so
			// the drift stays purely "noise around real content".
			lines[i] = ""
			continue
		}
		lines[i] = " " + line + "  "
	}
	return strings.Join(lines, "\n")
}

// PerturbIndent produces an indentation_flexible-tier drift: it rewrites ONLY
// the leading indentation of each line, converting a run of leading spaces to a
// run of tabs (4 spaces -> 1 tab, then a tab per leftover space) and prepending
// one extra tab of depth. The non-whitespace content after the indentation, and
// any internal whitespace, are preserved exactly — so internal/fuzzy's
// indentation-flexible strategy (TrimLeft(" \t") per line) is the lowest tier
// that re-matches it (the whitespace tier would NOT, since it only trims, it
// does not normalize spaces<->tabs equivalently for a depth shift). The
// transform is deterministic and byte-reproducible.
//
// EXPECTED strategy of the result: StrategyIndentationFlex (D-05).
func PerturbIndent(block string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		leadCount := len(line) - len(trimmed)
		lead := line[:leadCount]
		// Convert every 4 leading spaces to a tab, leftover spaces to tabs,
		// and any existing tabs are preserved as tabs. Then prepend one extra
		// tab to shift depth.
		spaces := strings.Count(lead, " ")
		tabs := strings.Count(lead, "\t")
		newTabs := tabs + spaces/4 + spaces%4 + 1
		lines[i] = strings.Repeat("\t", newTabs) + trimmed
	}
	return strings.Join(lines, "\n")
}

// PerturbEllipsis produces an ellipsis-tier drift: it replaces the middle
// line(s) of a (>=3-line) block with a single "..." placeholder on its own line,
// leaving the first and last lines intact. internal/fuzzy matches this via
// Options.AllowEllipsis "..."-on-own-line segmentation. For a block with fewer
// than 3 lines the middle is empty, so the transform inserts a "..." line
// between the head and tail (still a structural ellipsis drift). The transform
// is deterministic and byte-reproducible.
//
// EXPECTED strategy of the result: StrategyEllipsis (D-05).
func PerturbEllipsis(block string) string {
	lines := strings.Split(block, "\n")
	if len(lines) < 3 {
		// Head + "..." + tail keeps the anchors; degenerate but still ellipsis.
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[0])
		out = append(out, "...")
		if len(lines) > 1 {
			out = append(out, lines[len(lines)-1])
		}
		return strings.Join(out, "\n")
	}
	out := make([]string, 0, 3)
	out = append(out, lines[0])
	out = append(out, "...")
	out = append(out, lines[len(lines)-1])
	return strings.Join(out, "\n")
}

// PerturbExact is the identity transform: the unchanged block. EXPECTED strategy
// of the result: StrategyExact (byte-for-byte equality re-matches at tier 1).
func PerturbExact(block string) string { return block }
