package relatedidx

import "sync"

// SimHash LSH for cosine-nearest-neighbor candidate lookup over context
// vectors. Brute-force all-pairs cosine within a language is O(n^2) and
// unbounded on a large repo; this provides O(1)-ish candidate retrieval so
// SEMANTICALLY_RELATED emission stays bounded — the same role minhash.LSHIndex
// plays for SIMILAR_TO.
//
// Mechanism (random-hyperplane SimHash): project a vector onto SimHashBits
// fixed random ±1 hyperplanes; the sign of each projection is one signature
// bit. For two vectors with cosine c, P(bit agrees) = 1 - arccos(c)/π, so
// near-parallel vectors share most bits. Splitting the signature into Bands ×
// Rows and bucketing per band (candidate iff any band matches exactly) is
// standard LSH amplification: high recall at high cosine, few candidates at low
// cosine. SimHashBits = Bands × Rows.
//
// Deterministic: hyperplanes are derived from a fixed seed (seededHash),
// computed once. No RNG, no map-iteration in the value path.
//
// Band/row split: 16 bands × 4 rows. Recall P(candidate) = 1-(1-p^Rows)^Bands
// where p = 1-arccos(cos)/π. At cos≈0.8 (a real shared-vocabulary pair, see
// the daemon measurement test) p≈0.80 and recall≈0.9997 — effectively
// complete; at the 0.55 emission threshold recall is still ≈0.98. Unrelated
// bodies (cos≈0) become candidates ~64% of the time but are dropped by the
// exact-cosine recheck in semanticallyRelatedEdges, so precision is unaffected;
// only candidate-compare work rises, which the per-node fan-out cap bounds.
const (
	SimHashBits = 64
	SimBands    = 16
	SimRows     = 4 // SimBands * SimRows == SimHashBits
)

// hyperplanes[b][i] is the ±1 entry of hyperplane b at dimension i. Precomputed
// once; 64×256 = 16 KiB.
var (
	hyperplanesOnce sync.Once
	hyperplanes     [SimHashBits][D]int8
)

func initHyperplanes() {
	for b := 0; b < SimHashBits; b++ {
		for i := 0; i < D; i++ {
			// Deterministic ±1 from a per-(bit,dim) hash bit, via explicit
			// binary encoding (not string(rune(...)), which folds out-of-range
			// values to U+FFFD and would collide distinct (b,i) pairs).
			h := seededHash("relatedidx.hp", uint64(b)<<32|uint64(i))
			if h&1 == 1 {
				hyperplanes[b][i] = 1
			} else {
				hyperplanes[b][i] = -1
			}
		}
	}
}

// SimHash returns the SimHashBits-bit signature of v.
func SimHash(v *Vector) uint64 {
	hyperplanesOnce.Do(initHyperplanes)
	var sig uint64
	for b := 0; b < SimHashBits; b++ {
		var dot int64
		hp := &hyperplanes[b]
		for i := 0; i < D; i++ {
			dot += int64(v[i]) * int64(hp[i])
		}
		if dot >= 0 {
			sig |= 1 << uint(b)
		}
	}
	return sig
}

// simEntry is a node's SimHash signature paired with its store NodeID.
type simEntry struct {
	nodeID uint64
	sig    uint64
}

// SimLSH indexes SimHash signatures by band for candidate retrieval.
type SimLSH struct {
	// bands[b] maps a band's row-bits → the node IDs that share them.
	bands [SimBands]map[uint16][]uint64
	sigs  map[uint64]uint64 // nodeID → full signature (for the caller's exact recheck)
}

// NewSimLSH builds an index over the given (nodeID, vector) entries.
func NewSimLSH() *SimLSH {
	idx := &SimLSH{sigs: make(map[uint64]uint64)}
	for b := range idx.bands {
		idx.bands[b] = make(map[uint16][]uint64)
	}
	return idx
}

// bandKey extracts band b's SimRows bits from sig as a small key.
func bandKey(sig uint64, b int) uint16 {
	shift := uint(b * SimRows)
	mask := uint64((1 << SimRows) - 1)
	return uint16((sig >> shift) & mask)
}

// Insert adds a node's signature to every band bucket.
func (idx *SimLSH) Insert(nodeID, sig uint64) {
	idx.sigs[nodeID] = sig
	for b := 0; b < SimBands; b++ {
		k := bandKey(sig, b)
		idx.bands[b][k] = append(idx.bands[b][k], nodeID)
	}
}

// Candidates returns the distinct node IDs sharing at least one band with sig,
// excluding self. Order is unspecified; callers that need determinism sort the
// result (emission does).
func (idx *SimLSH) Candidates(self, sig uint64) []uint64 {
	seen := make(map[uint64]struct{})
	var out []uint64
	for b := 0; b < SimBands; b++ {
		for _, nid := range idx.bands[b][bandKey(sig, b)] {
			if nid == self {
				continue
			}
			if _, dup := seen[nid]; dup {
				continue
			}
			seen[nid] = struct{}{}
			out = append(out, nid)
		}
	}
	return out
}
