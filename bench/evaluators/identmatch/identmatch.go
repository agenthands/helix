// Package identmatch is the pure CrossCodeEval IM-EM / IM-F1 (Identifier-Match)
// scorer. It is a stdlib-only leaf (regexp + strings) with NO I/O.
//
// Identifier-Match (CrossCodeEval; Ding et al., NeurIPS 2023, arXiv:2310.11248)
// scores a completion by its IDENTIFIER content rather than its raw text. Each
// completion string is tokenized to identifiers, a language-agnostic keyword set
// is removed, and the resulting identifier SETS are compared two ways:
//
//   - IM-EM (em): the predicted identifier set equals the gold identifier set.
//   - IM-F1 (f1): the harmonic mean of precision and recall over the two sets,
//     where precision = |pred ∩ gold| / |pred| and recall = |pred ∩ gold| / |gold|.
//
// # Tokenizer (cite this rule in VERIFIED.md)
//
// Identifiers are extracted with the compiled regular expression
//
//	[A-Za-z_][A-Za-z0-9_]*
//
// i.e. a leading letter or underscore followed by zero or more letters, digits,
// or underscores. This is the ASCII identifier rule shared by Python, Java,
// TypeScript, and C#; non-ASCII letters are NOT part of an identifier token
// (they act as separators), which keeps the rule deterministic across the four
// CCE languages. Tokens are matched, NOT obtained by whitespace splitting — a
// whitespace split would over-count punctuation and operators (RESEARCH
// Pitfall 4).
//
// # Keyword set (cite this list in VERIFIED.md)
//
// After tokenization, the following language-agnostic keywords — the common
// subset across Python / Java / TypeScript / C# — are removed before set
// comparison so a shared keyword neither helps nor hurts the score:
//
//	if else elif for while do switch case default break continue return
//	function func def class interface enum struct extends implements
//	import from package using namespace new delete try catch finally throw throws
//	public private protected static final const let var val void
//	int long short byte char float double bool boolean string
//	true false null nil none undefined this self super in is as of typeof instanceof
//	and or not async await yield lambda with pass raise global nonlocal del assert
//
// Keywords that survive as identifiers in some grammars are still dropped here
// by design: the goal is to score user-chosen NAMES, not language syntax.
//
// # Empty-set convention
//
// Two EMPTY identifier sets (both inputs empty, or both keyword-only) are an
// exact match: em==true and f1==1.0. When exactly one set is empty (and the
// other non-empty) the sets are disjoint: em==false and f1==0.0. Match is total
// on every input — including empty and unicode — and never panics.
package identmatch

import "regexp"

// identRE is the CCE identifier tokenizer: a leading letter/underscore followed
// by letters, digits, or underscores (ASCII).
var identRE = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// keywords is the language-agnostic keyword set removed before set comparison.
// See the package doc comment for the cited list and rationale.
var keywords = map[string]struct{}{
	"if": {}, "else": {}, "elif": {}, "for": {}, "while": {}, "do": {},
	"switch": {}, "case": {}, "default": {}, "break": {}, "continue": {}, "return": {},
	"function": {}, "func": {}, "def": {}, "class": {}, "interface": {}, "enum": {},
	"struct": {}, "extends": {}, "implements": {},
	"import": {}, "from": {}, "package": {}, "using": {}, "namespace": {},
	"new": {}, "delete": {}, "try": {}, "catch": {}, "finally": {}, "throw": {}, "throws": {},
	"public": {}, "private": {}, "protected": {}, "static": {}, "final": {},
	"const": {}, "let": {}, "var": {}, "val": {}, "void": {},
	"int": {}, "long": {}, "short": {}, "byte": {}, "char": {}, "float": {},
	"double": {}, "bool": {}, "boolean": {}, "string": {},
	"true": {}, "false": {}, "null": {}, "nil": {}, "none": {}, "undefined": {},
	"this": {}, "self": {}, "super": {}, "in": {}, "is": {}, "as": {}, "of": {},
	"typeof": {}, "instanceof": {},
	"and": {}, "or": {}, "not": {}, "async": {}, "await": {}, "yield": {}, "lambda": {},
	"with": {}, "pass": {}, "raise": {}, "global": {}, "nonlocal": {}, "del": {},
	"assert": {},
}

// Match reports the CrossCodeEval IM-EM (em) and IM-F1 (f1) over the identifier
// sets of pred and gold. em is true iff the identifier sets are equal; f1 is the
// harmonic mean of precision/recall over those sets. Empty-vs-empty ⇒ (true, 1.0);
// exactly-one-empty ⇒ (false, 0.0).
func Match(pred, gold string) (em bool, f1 float64) {
	ps := identifierSet(pred)
	gs := identifierSet(gold)

	// Both empty ⇒ exact match, F1 defined as 1.0 (empty-set convention).
	if len(ps) == 0 && len(gs) == 0 {
		return true, 1.0
	}
	// Exactly one empty ⇒ disjoint sets.
	if len(ps) == 0 || len(gs) == 0 {
		return false, 0.0
	}

	inter := 0
	for id := range ps {
		if _, ok := gs[id]; ok {
			inter++
		}
	}

	em = inter == len(ps) && inter == len(gs)

	precision := float64(inter) / float64(len(ps))
	recall := float64(inter) / float64(len(gs))
	if precision+recall == 0 {
		// Non-empty sets with zero overlap.
		return em, 0.0
	}
	f1 = 2 * precision * recall / (precision + recall)
	return em, f1
}

// identifierSet tokenizes s into the set of identifiers minus keywords.
func identifierSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, tok := range identRE.FindAllString(s, -1) {
		if _, isKw := keywords[tok]; isKw {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}
