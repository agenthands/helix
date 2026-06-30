// Package relatedidx implements Random Indexing (RI) over a function body's
// *vocabulary* — identifier and comment tokens — producing a fixed-dimension
// context vector whose cosine similarity captures SEMANTIC relatedness (shared
// domain vocabulary), as opposed to the STRUCTURAL near-clone signal that
// internal/semantic/minhash produces.
//
// The two engines are deliberate inverses:
//
//   - minhash.collectLeafTokens + normalizeKind ERASE vocabulary (every
//     identifier→"I", string→"S", number→"N", type→"T") and fingerprint the
//     remaining structural node-kind stream. → "same shape" → SIMILAR_TO.
//   - relatedidx KEEPS vocabulary (identifier subtokens + comment words),
//     ignores structure entirely (a bag-of-tokens projection). → "same
//     vocabulary / domain" → SEMANTICALLY_RELATED.
//
// This orthogonality is what makes the SEMANTICALLY_RELATED edge a distinct
// signal from SIMILAR_TO rather than a near-duplicate of it.
//
// Algorithm (classic Random Indexing): each unique token is assigned a sparse
// ternary index vector — Nonzeros deterministic ±1 entries whose positions and
// signs are seeded from the token's xxhash. A body's context vector is the
// elementwise sum of its tokens' index vectors. By the Johnson–Lindenstrauss
// lemma this random projection approximately preserves cosine similarity of the
// underlying bag-of-tokens representation, at a fraction of the dimensionality.
//
// Determinism: vectors accumulate into int32 (integer addition is associative
// and commutative, so the result is independent of token order and exact — no
// floating-point rounding hazard). float64 appears only inside Cosine. Identical
// token input therefore yields a byte-identical Vector across runs and hosts.
//
// Dependency boundary mirrors minhash/: stdlib + go-tree-sitter + xxhash (all
// already in go.mod). No internal/* imports; this is a leaf package.
package relatedidx

import (
	"encoding/binary"
	"math"
	"strings"
	"unicode"

	"github.com/cespare/xxhash/v2"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// D is the Random-Indexing dimensionality. 256 is comfortably above the
// Johnson–Lindenstrauss bound for the vocabulary sizes a single function body
// produces (tens to low-hundreds of distinct subtokens), keeps each vector at
// 1 KiB (256 × int32) so per-symbol storage stays bounded on large repos, and
// makes the position index a clean low-8-bits slice of the token hash. Revisit
// only if a measured perf/quality need appears.
const D = 256

// Nonzeros is the number of ±1 entries each token contributes (the RI "spray").
// Small and fixed: enough spread for near-orthogonal token vectors, cheap to
// compute, and keeps collisions rare at D=256.
const Nonzeros = 8

// MinTokens is the minimum vocabulary-token count for a meaningful context
// vector. Bodies below it (trivial getters, one-liners) carry no reliable
// vocabulary signal, so ComputeVector reports ok=false and no
// SEMANTICALLY_RELATED edge is ever drawn from trivia. Analogous in spirit to
// minhash.MinNodes (which gates on structural leaf tokens, a larger count).
const MinTokens = 6

// Vector is a Random-Indexing context vector. int32 accumulation is exact and
// order-independent — the determinism guarantee lives in this type choice.
type Vector [D]int32

// ComputeContextVector projects a token slice into a context vector via Random
// Indexing. Pure and deterministic: identical token multisets (in any order)
// produce byte-identical vectors. This is the engine core; callers that already
// hold tokens (tests, future non-AST sources) use it directly.
func ComputeContextVector(tokens []string) Vector {
	var v Vector
	for _, t := range tokens {
		if t == "" {
			continue
		}
		for k := 0; k < Nonzeros; k++ {
			// Derive an independent hash per (token, k). The spray index is
			// mixed in via explicit binary encoding (NOT string(rune(k)),
			// which folds out-of-range values to U+FFFD and would collide
			// distinct seeds) so the Nonzeros entries of one token are
			// decorrelated and the derivation is total over all k.
			hk := seededHash(t, uint64(k))
			pos := hk % D
			// Unbiased ±1 sign from one independent hash bit. A balanced sign
			// is essential: any positive bias would add a shared DC component
			// to every vector and inflate the cosine between UNRELATED bodies,
			// destroying the distinctness the whole edge depends on.
			if (hk>>32)&1 == 1 {
				v[pos]++
			} else {
				v[pos]--
			}
		}
	}
	return v
}

// seededHash returns a deterministic 64-bit hash of (token, seed) using
// explicit binary encoding of the seed. Avoids the string(rune(...)) idiom,
// which silently maps values outside the valid rune range to U+FFFD and would
// collide distinct seeds.
func seededHash(token string, seed uint64) uint64 {
	var buf [9]byte
	buf[0] = 0x00 // separator between token bytes and the seed
	binary.LittleEndian.PutUint64(buf[1:], seed)
	d := xxhash.New()
	_, _ = d.WriteString(token)
	_, _ = d.Write(buf[:])
	return d.Sum64()
}

// VectorFromTokens applies the MinTokens gate then projects. Returns ok=false
// (and a zero vector) when there is too little vocabulary to be meaningful.
func VectorFromTokens(tokens []string) (Vector, bool) {
	if len(tokens) < MinTokens {
		return Vector{}, false
	}
	return ComputeContextVector(tokens), true
}

// ComputeVector extracts vocabulary tokens from a function-body AST node and
// projects them. Returns ok=false when the body is nil or below MinTokens.
//
// MUST be called while the tree-sitter tree is still alive (the returned Vector
// is a value copy safe to carry after the tree closes; the *Node argument is
// not). Mirrors minhash.ComputeSignature's lifetime contract.
func ComputeVector(body *tree_sitter.Node, source []byte) (*Vector, bool) {
	if body == nil {
		return nil, false
	}
	var raw []string
	collectVocabTokens(body, source, &raw)
	// Drop boilerplate/keyword/trivial tokens so the vector reflects DOMAIN
	// vocabulary, not plumbing (see stopwords.go). The MinTokens gate then
	// applies to the surviving domain tokens — a body that is all boilerplate
	// yields no vector and draws no SEMANTICALLY_RELATED edge.
	tokens := raw[:0]
	for _, t := range raw {
		if keepToken(t) {
			tokens = append(tokens, t)
		}
	}
	v, ok := VectorFromTokens(tokens)
	if !ok {
		return nil, false
	}
	return &v, true
}

// Cosine returns the cosine similarity of two context vectors in [-1, 1]
// (typically [0, 1] for RI sums of mostly-overlapping vocabularies). 0 when
// either vector is all-zero. float64 is confined to this function so vector
// values themselves stay exact/deterministic.
func Cosine(a, b *Vector) float64 {
	if a == nil || b == nil {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < D; i++ {
		x := float64(a[i])
		y := float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// collectVocabTokens walks the AST collecting VOCABULARY: identifier subtokens
// (camelCase/snake_case split, lowercased) and comment words. It is the
// deliberate inverse of minhash.collectLeafTokens — it reads identifier/comment
// TEXT and ignores structural node kinds.
func collectVocabTokens(node *tree_sitter.Node, source []byte, tokens *[]string) {
	if node == nil {
		return
	}
	kind := node.Kind()
	// Comments may be non-leaf in some grammars; take the whole text and do not
	// recurse into them.
	if isCommentKind(kind) {
		appendWords(node.Utf8Text(source), tokens)
		return
	}
	if node.ChildCount() == 0 {
		if isIdentifierKind(kind) {
			appendSubtokens(node.Utf8Text(source), tokens)
		}
		return
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		collectVocabTokens(node.Child(i), source, tokens)
	}
}

// isIdentifierKind reports whether a tree-sitter leaf kind names an identifier
// across the 11 supported languages (Go, TS, Python, Java, C#, Rust, C, C++,
// Kotlin, PHP, Ruby). Superset by design — an unknown identifier-ish kind that
// slips through only adds vocabulary, never structure.
func isIdentifierKind(kind string) bool {
	switch kind {
	case "identifier", "field_identifier", "type_identifier", "package_identifier",
		"simple_identifier", "name", "variable_name", "namespace_identifier",
		"property_identifier", "shorthand_property_identifier", "constant",
		"instance_variable", "global_variable", "class_variable", "label_name":
		return true
	default:
		return false
	}
}

// isCommentKind reports whether a tree-sitter kind names a comment.
func isCommentKind(kind string) bool {
	switch kind {
	case "comment", "line_comment", "block_comment", "doc_comment":
		return true
	default:
		return false
	}
}

// appendSubtokens splits an identifier into lowercased camelCase/snake_case
// subtokens and appends them. "getUserName"→get,user,name; "HTTPServer"→http,server;
// "user_id"→user,id.
func appendSubtokens(ident string, tokens *[]string) {
	runes := []rune(ident)
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			*tokens = append(*tokens, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for i, r := range runes {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			flush()
			continue
		}
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			switch {
			case unicode.IsLower(prev) || unicode.IsDigit(prev):
				// lower→Upper boundary: getName | get,Name
				flush()
			case unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]):
				// acronym end: HTTPServer | HTTP,Server
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
}

// appendWords splits free comment text on non-alphanumeric runs and appends
// lowercased words.
func appendWords(text string, tokens *[]string) {
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			*tokens = append(*tokens, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
}
