// Package minhash implements MinHash fingerprinting and LSH for SIMILAR_TO edges.
// Ported from codebase-memory-mcp/src/simhash/minhash.c (MIT-licensed).
//
// Algorithm: walk tree-sitter AST collecting leaf node types, hash trigrams
// with structural weighting, produce K=64 MinHash signatures. Jaccard
// similarity is estimated as the fraction of matching hash values. LSH
// index uses 32 bands × 2 rows for O(1) candidate lookup.
package minhash

import (
	"encoding/hex"
	"math/bits"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/cespare/xxhash/v2"
)

// K is the number of hash permutations (MinHash signature size).
const K = 64

// MinNodes is the minimum leaf AST tokens required for a meaningful fingerprint.
const MinNodes = 30

// JaccardThreshold is the default threshold for SIMILAR_TO edge emission.
const JaccardThreshold = 0.95

// MaxEdgesPerNode prevents utility-function explosion.
const MaxEdgesPerNode = 10

// LSH parameters: b bands × r rows per band.
const (
	LSHBands = 32
	LSHRows  = 2
)

// Signature is a MinHash signature: K minimum hash values.
type Signature struct {
	Values [K]uint64
}

// ComputeSignature builds a MinHash fingerprint from a tree-sitter AST node.
// Walks leaf nodes, collects normalized types, hashes trigrams.
// Returns false if the body has too few leaf tokens for a meaningful signature.
func ComputeSignature(body *tree_sitter.Node, source []byte) (*Signature, bool) {
	tokens := make([]string, 0, 4096)
	collectLeafTokens(body, source, &tokens)
	if len(tokens) < MinNodes {
		return nil, false
	}
	sig := &Signature{}
	for i := range sig.Values {
		sig.Values[i] = ^uint64(0) // max uint64
	}
	hashTrigrams(tokens, sig)
	return sig, true
}

// Jaccard estimates the Jaccard similarity between two signatures.
// Returns value in [0.0, 1.0].
func Jaccard(a, b *Signature) float64 {
	if a == nil || b == nil {
		return 0
	}
	matches := 0
	for i := 0; i < K; i++ {
		if a.Values[i] == b.Values[i] {
			matches++
		}
	}
	return float64(matches) / float64(K)
}

// Hex encodes a signature as a hex string (512 chars).
func (s *Signature) Hex() string {
	buf := make([]byte, K*8)
	for i := 0; i < K; i++ {
		v := s.Values[i]
		buf[i*8+0] = byte(v >> 56)
		buf[i*8+1] = byte(v >> 48)
		buf[i*8+2] = byte(v >> 40)
		buf[i*8+3] = byte(v >> 32)
		buf[i*8+4] = byte(v >> 24)
		buf[i*8+5] = byte(v >> 16)
		buf[i*8+6] = byte(v >> 8)
		buf[i*8+7] = byte(v)
	}
	return hex.EncodeToString(buf)
}

// collectLeafTokens walks the AST in-order, collecting normalized leaf node types.
// Leaf-only: skips internal grammar nodes. Language-agnostic by design.
func collectLeafTokens(node *tree_sitter.Node, source []byte, tokens *[]string) {
	if node.ChildCount() == 0 {
		kind := normalizeKind(node.Kind())
		*tokens = append(*tokens, kind)
		return
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		if child := node.Child(i); child != nil {
			collectLeafTokens(child, source, tokens)
		}
	}
}

// normalizeKind maps tree-sitter node types to canonical short codes.
// Mirrors normalise_node_type() from minhash.c.
func normalizeKind(kind string) string {
	switch kind {
	case "identifier", "field_identifier", "type_identifier", "simple_identifier",
		"name", "variable_name", "namespace_identifier":
		return "I"
	case "string_literal", "interpreted_string_literal", "raw_string_literal",
		"string_content", "encapsed_string", "string":
		return "S"
	case "int_literal", "float_literal", "integer", "decimal_integer_literal",
		"hex_integer_literal", "number_literal", "real_literal":
		return "N"
	case "type", "primitive_type", "predefined_type", "builtin_type",
		"type_annotation", "named_type", "user_type", "type_arguments":
		return "T"
	default:
		return kind
	}
}

// isNormalizedToken returns true for the four generic types (I/S/N/T).
func isNormalizedToken(tok string) bool {
	return tok == "I" || tok == "S" || tok == "N" || tok == "T"
}

// trigramStructuralWeight returns the count of non-normalized tokens (0-3).
func trigramStructuralWeight(a, b, c string) int {
	w := 0
	if !isNormalizedToken(a) { w++ }
	if !isNormalizedToken(b) { w++ }
	if !isNormalizedToken(c) { w++ }
	return w
}

// hashTrigrams processes token trigrams into the MinHash signature.
// Uses xxHash for each seed, applies structural weighting.
func hashTrigrams(tokens []string, sig *Signature) {
	if len(tokens) < 3 {
		return
	}
	seen := make(map[uint64]bool, 2048)
	for i := 0; i < len(tokens)-2; i++ {
		a, b, c := tokens[i], tokens[i+1], tokens[i+2]
		weight := trigramStructuralWeight(a, b, c)
		if weight == 0 {
			continue
		}
		trigram := a + "\x00" + b + "\x00" + c
		trigHash := xxhash.Sum64String(trigram)
		if seen[trigHash] {
			continue
		}
		seen[trigHash] = true
		// Weighted MinHash: hash w times per seed, take min
		for seed := uint64(1); seed <= uint64(K); seed++ {
			for w := 0; w < weight; w++ {
				h := xxhash.Sum64String(trigram + "\x00" + string(rune(seed)) + "\x00" + string(rune(w)))
				if h < sig.Values[seed-1] {
					sig.Values[seed-1] = h
				}
			}
		}
	}
}

// LSHIndex provides locality-sensitive hashing for candidate lookup.
type LSHIndex struct {
	buckets [LSHBands][1 << 16][]Entry
}

// Entry is an item stored in the LSH index.
type Entry struct {
	NodeID uint64
	Sig    *Signature
}


// NewLSHIndex creates a new LSH index.
func NewLSHIndex() *LSHIndex { return &LSHIndex{} }

func (idx *LSHIndex) Insert(entry Entry) {
	for band := 0; band < LSHBands; band++ {
		bh := bandHash(entry.Sig, band)
		bucket := &idx.buckets[band][bh]
		*bucket = append(*bucket, entry)
	}
}

// Query returns candidate entries similar to the given fingerprint.
func (idx *LSHIndex) Query(sig *Signature, maxResults int) []Entry {
	seen := make(map[uint64]bool, 64)
	var results []Entry
	for band := 0; band < LSHBands; band++ {
		bh := bandHash(sig, band)
		for _, entry := range idx.buckets[band][bh] {
			if seen[entry.NodeID] {
				continue
			}
			seen[entry.NodeID] = true
			if Jaccard(sig, entry.Sig) >= JaccardThreshold {
				results = append(results, entry)
				if len(results) >= maxResults {
					return results
				}
			}
		}
	}
	return results
}

// bandHash computes a 16-bit hash for a band from the signature.
func bandHash(sig *Signature, band int) uint16 {
	start := band * LSHRows
	var h uint64
	for i := 0; i < LSHRows; i++ {
		h ^= sig.Values[start+i]
	}
	return uint16(h ^ (h >> 16) ^ (h >> 32) ^ (h >> 48))
}

// rotation-based jhash for band hashing (alternative — faster for some inputs)
func _() { bits.TrailingZeros64(0) }
