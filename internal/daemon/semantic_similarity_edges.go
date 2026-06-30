package daemon

// semantic_similarity_edges.go holds the batch-level edge derivations that
// run AFTER factsFromExtracted has built its name→node index and collected
// per-symbol fingerprints:
//
//   - resolvePendingDst: fills DstNodeID on the shallow IMPLEMENTS / EXTENDS /
//     IMPORTS / classifier-call edges (HTTP_CALLS / ASYNC_CALLS / EMITS /
//     LISTENS_ON) by resolving their target NAME against the batch name index.
//     Pre-2026-06-30 these edges shipped with DstNodeID=0 ("unresolved — type
//     resolver fills later"), but the type resolver is a Phase-57 stub that
//     returns no edges, so the targets were never filled. Resolving against the
//     same batch index the TESTS / HANDLES edges already use turns every
//     in-repo target into a real endpoint; out-of-repo targets (stdlib types,
//     external packages) correctly stay DstNodeID=0.
//
//   - similarToEdges: SIMILAR_TO near-clone edges via MinHash + LSH over the
//     per-symbol signatures the providers fingerprinted at extraction time.
//
//   - structuralTwinEdges: STRUCTURAL_TWIN edges via ASTProfile structural-
//     profile similarity. NOTE on semantics: this is STRUCTURAL-PROFILE
//     similarity (control-flow / expression / literal shape), NOT
//     interprocedural data/taint flow — it was renamed from the misnamed
//     "DATA_FLOWS" (B0-a); the DATA_FLOWS surface kind is now reserved for the
//     true arg-to-param interprocedural flow of v2.8 Workstream B. The mechanism
//     here is classifier.ComputeProfile. Confidence is held low to reflect that
//     the edge is a structural-similarity signal.
import (
	"math"
	"sort"
	"strconv"

	"github.com/agenthands/helix/internal/semantic/classifier"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/minhash"
	"github.com/agenthands/helix/internal/semantic/relatedidx"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// pendingDstResolve records an emitted edge whose DstNodeID was left 0 because
// its target is a NAME that can only be resolved once the whole batch's
// name→node index exists. resolvePendingDst patches out.Edges[edgeIdx] in
// place when a candidate name resolves.
type pendingDstResolve struct {
	edgeIdx    int      // index into out.Edges
	candidates []string // target names to try, most-specific first
	srcNodeID  uint64   // to reject self-edges
	confidence float64  // confidence to stamp when resolved
	weight     float64  // weight to stamp when resolved
}

// resolvePendingDst walks the pending list and, for each, resolves the first
// candidate name that maps to an in-repo node (and is not the source itself),
// stamping DstNodeID + the resolved confidence/weight onto the edge. Edges
// whose target resolves to nothing keep DstNodeID=0 and their original
// (low) confidence — an honest "external / unresolved" signal rather than a
// fabricated endpoint.
func resolvePendingDst(edges []semanticstore.EdgeFact, pending []pendingDstResolve, nameToNode map[string]uint64) {
	for _, p := range pending {
		if p.edgeIdx < 0 || p.edgeIdx >= len(edges) {
			continue
		}
		for _, cand := range p.candidates {
			if cand == "" {
				continue
			}
			tgt, ok := nameToNode[cand]
			if !ok || tgt == 0 || tgt == p.srcNodeID {
				continue
			}
			edges[p.edgeIdx].DstNodeID = tgt
			edges[p.edgeIdx].Confidence = p.confidence
			edges[p.edgeIdx].Weight = p.weight
			break
		}
	}
}

// lastNameSegment returns the trailing identifier of a (possibly) qualified
// name: "io.Reader"→"Reader", "pkg::Base"→"Base", "A\\B"→"B", "a/b"→"b".
// The batch name index keys on the bare symbol Name, so a qualified heritage
// target / import path must be reduced to its last segment to match.
func lastNameSegment(qualified string) string {
	if qualified == "" {
		return ""
	}
	best := -1
	for _, sep := range []byte{'.', ':', '\\', '/'} {
		for i := len(qualified) - 1; i >= 0; i-- {
			if qualified[i] == sep {
				if i > best {
					best = i
				}
				break
			}
		}
	}
	if best < 0 || best == len(qualified)-1 {
		return qualified
	}
	return qualified[best+1:]
}

// importTargetCandidates returns the in-repo resolution candidates for an
// import: the named-imported symbols ({ Foo, Bar } from "./mod" / from mod
// import Foo), most-specific first, plus each symbol's last segment. A bare
// module import (import "fmt") carries no named symbol and returns nil, so
// its IMPORTS edge stays module-level (DstNodeID=0) rather than mis-resolving
// to an unrelated same-named symbol.
func importTargetCandidates(imp extract.ImportFact) []string {
	if len(imp.Symbols) == 0 {
		return nil
	}
	out := make([]string, 0, len(imp.Symbols)*2)
	for _, s := range imp.Symbols {
		if s == "" {
			continue
		}
		out = append(out, s)
		if seg := lastNameSegment(s); seg != s {
			out = append(out, seg)
		}
	}
	return out
}

// fingerprintedNode pairs a resolved store NodeID with the fingerprints the
// provider computed for that symbol's body. factsFromExtracted collects one
// per fingerprintable symbol (function / method with a body), zipping
// ef.Symbols[k] (carrying MinHash / Profile / ContextVec) onto
// single.Symbols[k] (carrying the masked NodeID) — the two slices are 1:1 in
// order. sig/profile feed structure edges (SIMILAR_TO / DATA_FLOWS); vec feeds
// the vocabulary edge (SEMANTICALLY_RELATED).
type fingerprintedNode struct {
	nodeID   uint64
	kind     string
	language string
	sig      *minhash.Signature
	profile  *classifier.ASTProfile
	vec      *relatedidx.Vector
}

// similarToEdges emits SIMILAR_TO edges between near-clone function bodies.
// Builds a MinHash LSH index over every node carrying a signature, then for
// each node queries the index for candidates whose Jaccard estimate clears
// minhash.JaccardThreshold. Each unordered pair is emitted once (lower NodeID
// = source) and per-node fan-out is capped at minhash.MaxEdgesPerNode to stop
// utility-function explosion. Deterministic: nodes are processed in ascending
// NodeID order and emitted pairs are de-duplicated on (min,max).
func similarToEdges(nodes []fingerprintedNode) []semanticstore.EdgeFact {
	withSig := make([]fingerprintedNode, 0, len(nodes))
	for _, n := range nodes {
		if n.sig != nil && n.nodeID != 0 {
			withSig = append(withSig, n)
		}
	}
	if len(withSig) < 2 {
		return nil
	}
	sort.Slice(withSig, func(i, j int) bool { return withSig[i].nodeID < withSig[j].nodeID })

	idx := minhash.NewLSHIndex()
	for _, n := range withSig {
		idx.Insert(minhash.Entry{NodeID: n.nodeID, Sig: n.sig})
	}

	type pair struct{ lo, hi uint64 }
	seenPair := make(map[pair]bool)
	fanout := make(map[uint64]int)
	var edges []semanticstore.EdgeFact

	for _, n := range withSig {
		if fanout[n.nodeID] >= minhash.MaxEdgesPerNode {
			continue
		}
		// Query a few more than the cap so self + already-capped peers can be
		// skipped without starving the result.
		cands := idx.Query(n.sig, minhash.MaxEdgesPerNode+1)
		for _, c := range cands {
			if c.NodeID == n.nodeID {
				continue
			}
			lo, hi := n.nodeID, c.NodeID
			if hi < lo {
				lo, hi = hi, lo
			}
			pr := pair{lo, hi}
			if seenPair[pr] {
				continue
			}
			if fanout[n.nodeID] >= minhash.MaxEdgesPerNode || fanout[c.NodeID] >= minhash.MaxEdgesPerNode {
				continue
			}
			seenPair[pr] = true
			fanout[n.nodeID]++
			fanout[c.NodeID]++
			edges = append(edges, semanticstore.EdgeFact{
				SrcNodeID:  lo,
				DstNodeID:  hi,
				EdgeKind:   "SIMILAR_TO",
				Source:     "minhash",
				Confidence: 0.45,
				Weight:     0.3,
			})
			if fanout[n.nodeID] >= minhash.MaxEdgesPerNode {
				break
			}
		}
	}
	return edges
}

// structuralTwinThreshold is the cosine-similarity floor for a STRUCTURAL_TWIN
// edge. Held high (near-identical structural profile) so the edge stays a
// meaningful "these functions have the same control/expression shape" signal
// rather than linking every pair of small functions.
const structuralTwinThreshold = 0.985

// structuralTwinEdges emits STRUCTURAL_TWIN edges between functions whose
// ASTProfile structural vectors are near-identical (cosine >=
// structuralTwinThreshold) — i.e. they share control-flow / expression SHAPE.
// This is a structural signal (renamed from the earlier misnamed "DATA_FLOWS";
// the DATA_FLOWS name is reserved for true interprocedural arg-to-param flow,
// v2.8 Workstream B). To stay bounded on large repos it buckets nodes by a
// coarse quantized profile key (+ language) and only compares within a bucket;
// per-node fan-out is capped at minhash.MaxEdgesPerNode. Deterministic: buckets
// and members are processed in sorted order, pairs de-duplicated on (min,max).
func structuralTwinEdges(nodes []fingerprintedNode) []semanticstore.EdgeFact {
	type keyed struct {
		key  string
		node fingerprintedNode
		norm float64
	}
	buckets := make(map[string][]keyed)
	for _, n := range nodes {
		if n.profile == nil || n.nodeID == 0 {
			continue
		}
		norm := profileNorm(n.profile)
		if norm == 0 {
			continue // all-zero profile carries no structural signal
		}
		k := n.language + "|" + profileBucketKey(n.profile)
		buckets[k] = append(buckets[k], keyed{key: k, node: n, norm: norm})
	}
	if len(buckets) == 0 {
		return nil
	}

	bucketKeys := make([]string, 0, len(buckets))
	for k := range buckets {
		bucketKeys = append(bucketKeys, k)
	}
	sort.Strings(bucketKeys)

	type pair struct{ lo, hi uint64 }
	seenPair := make(map[pair]bool)
	fanout := make(map[uint64]int)
	var edges []semanticstore.EdgeFact

	for _, bk := range bucketKeys {
		members := buckets[bk]
		if len(members) < 2 {
			continue
		}
		sort.Slice(members, func(i, j int) bool { return members[i].node.nodeID < members[j].node.nodeID })
		for i := range members {
			a := members[i]
			if fanout[a.node.nodeID] >= minhash.MaxEdgesPerNode {
				continue
			}
			for j := i + 1; j < len(members); j++ {
				b := members[j]
				if fanout[a.node.nodeID] >= minhash.MaxEdgesPerNode {
					break
				}
				if fanout[b.node.nodeID] >= minhash.MaxEdgesPerNode {
					continue
				}
				if profileCosine(a.node.profile, b.node.profile, a.norm, b.norm) < structuralTwinThreshold {
					continue
				}
				lo, hi := a.node.nodeID, b.node.nodeID
				if hi < lo {
					lo, hi = hi, lo
				}
				pr := pair{lo, hi}
				if seenPair[pr] {
					continue
				}
				seenPair[pr] = true
				fanout[a.node.nodeID]++
				fanout[b.node.nodeID]++
				edges = append(edges, semanticstore.EdgeFact{
					SrcNodeID:  lo,
					DstNodeID:  hi,
					EdgeKind:   "STRUCTURAL_TWIN",
					Source:     "ast_profile",
					Confidence: 0.35,
					Weight:     0.3,
				})
			}
		}
	}
	return edges
}

// profileNorm returns the Euclidean norm of an ASTProfile vector.
func profileNorm(p *classifier.ASTProfile) float64 {
	var sum float64
	for _, v := range p {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		return 0
	}
	return math.Sqrt(sum)
}

// profileCosine returns the cosine similarity of two profile vectors given
// their precomputed norms (both assumed non-zero).
func profileCosine(a, b *classifier.ASTProfile, na, nb float64) float64 {
	if na == 0 || nb == 0 {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot / (na * nb)
}

// profileBucketKey quantizes the dominant structural features into a coarse
// string key so only plausibly-similar functions are compared. Features
// chosen are the high-signal control/structure counts; quantizing caps the
// per-bucket comparison set without dropping near-identical profiles (they
// quantize to the same key).
func profileBucketKey(p *classifier.ASTProfile) string {
	// Indices mirror classifier.ComputeProfile's feature layout.
	const (
		fIfDepth   = 0
		fLoopDepth = 1
		fStmtCount = 8
		fCallCount = 10
		fLength    = 24
	)
	q := func(v float32, band float32) int { return int(v / band) }
	return strconv.Itoa(q(p[fIfDepth], 1)) + "," +
		strconv.Itoa(q(p[fLoopDepth], 1)) + "," +
		strconv.Itoa(q(p[fStmtCount], 5)) + "," +
		strconv.Itoa(q(p[fCallCount], 3)) + "," +
		strconv.Itoa(q(p[fLength], 10))
}

// semanticRelatedThreshold is the cosine floor for a SEMANTICALLY_RELATED edge.
// Vocabulary cosine over RI vectors is noisier than structural cosine, so this
// sits well below the DATA_FLOWS structural threshold (0.985): we want bodies
// that share a real domain vocabulary, not identical token sets.
//
// Calibrated against measured real-Go bodies (TestRelatedness_DistinctAndSelective):
// with the relatedidx stop-list active, a clearly-unrelated pair scores ~0.00
// (often slightly negative) and a different-structure/shared-domain-vocabulary
// pair scores ~0.80. 0.55 sits squarely in that gap — high enough that
// boilerplate-only overlap cannot clear it (the vacuity risk), low enough that
// genuine domain cognates link without re-collapsing onto the near-identical
// structural edges.
const semanticRelatedThreshold = 0.55

// semanticallyRelatedEdges emits SEMANTICALLY_RELATED edges between functions
// whose Random-Indexing context vectors are cosine-similar — i.e. they share
// domain VOCABULARY (identifiers + comments), independent of structural shape.
// This is the deliberate complement of similarToEdges (MinHash, structure):
// the same node pair may be SIMILAR_TO, SEMANTICALLY_RELATED, both, or neither.
//
// Bounded via SimHash LSH (relatedidx.SimLSH): candidates come from shared LSH
// bands, not an all-pairs scan, then are confirmed by exact cosine. Per-node
// fan-out is capped, each unordered pair emitted once (lower NodeID = source),
// and processing is deterministic (ascending NodeID, dedup on (min,max)).
func semanticallyRelatedEdges(nodes []fingerprintedNode) []semanticstore.EdgeFact {
	type vnode struct {
		nodeID uint64
		vec    *relatedidx.Vector
		sig    uint64
	}
	withVec := make([]vnode, 0, len(nodes))
	for _, n := range nodes {
		if n.vec != nil && n.nodeID != 0 {
			withVec = append(withVec, vnode{nodeID: n.nodeID, vec: n.vec, sig: relatedidx.SimHash(n.vec)})
		}
	}
	if len(withVec) < 2 {
		return nil
	}
	sort.Slice(withVec, func(i, j int) bool { return withVec[i].nodeID < withVec[j].nodeID })

	byID := make(map[uint64]*relatedidx.Vector, len(withVec))
	idx := relatedidx.NewSimLSH()
	for i := range withVec {
		idx.Insert(withVec[i].nodeID, withVec[i].sig)
		byID[withVec[i].nodeID] = withVec[i].vec
	}

	type pair struct{ lo, hi uint64 }
	seenPair := make(map[pair]bool)
	fanout := make(map[uint64]int)
	var edges []semanticstore.EdgeFact

	for _, n := range withVec {
		if fanout[n.nodeID] >= minhash.MaxEdgesPerNode {
			continue
		}
		cands := idx.Candidates(n.nodeID, n.sig)
		sort.Slice(cands, func(i, j int) bool { return cands[i] < cands[j] })
		for _, c := range cands {
			lo, hi := n.nodeID, c
			if hi < lo {
				lo, hi = hi, lo
			}
			pr := pair{lo, hi}
			if seenPair[pr] {
				continue
			}
			if fanout[n.nodeID] >= minhash.MaxEdgesPerNode || fanout[c] >= minhash.MaxEdgesPerNode {
				continue
			}
			if relatedidx.Cosine(n.vec, byID[c]) < semanticRelatedThreshold {
				continue
			}
			seenPair[pr] = true
			fanout[n.nodeID]++
			fanout[c]++
			edges = append(edges, semanticstore.EdgeFact{
				SrcNodeID:  lo,
				DstNodeID:  hi,
				EdgeKind:   "SEMANTICALLY_RELATED",
				Source:     "random_index",
				Confidence: 0.50,
				Weight:     0.3,
			})
			if fanout[n.nodeID] >= minhash.MaxEdgesPerNode {
				break
			}
		}
	}
	return edges
}
