// Package exactmatch is the pure CrossCodeEval CM-EM (Code-Match Exact-Match)
// scorer. It is a stdlib-only leaf with NO I/O: a single total function over
// two completion strings.
//
// CM-EM (CrossCodeEval; Ding et al., NeurIPS 2023, arXiv:2310.11248) is exact
// string equality between the predicted completion and the ground-truth
// completion. This package deliberately performs NO normalization: it compares
// the raw completion strings byte-for-byte. Any trimming, case-folding, or
// whitespace normalization is the caller's responsibility (and must be applied
// identically to the live reference to reproduce the published numbers); the
// scorer itself is the literal equality oracle.
//
// EM-on-empty semantics: EM("", "") == true — two empty completions are an
// exact match (the empty string equals the empty string). EM is total on every
// input, including unicode, and never panics.
package exactmatch

// EM reports whether pred is an exact (byte-for-byte) match of gold. This is the
// CrossCodeEval CM-EM oracle: a direct string equality over the raw completion
// strings with no normalization. Both-empty is an exact match (true).
func EM(pred, gold string) bool {
	return pred == gold
}
