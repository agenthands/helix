package types

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strconv"
)

// FixpointResolve runs up to maxIter passes over the per-package reference
// set. Each pass invokes lang.ResolveChain for refs whose response is not
// yet Resolved=true. The loop EXITS EARLY when a complete pass produces no
// state change (state.Hash() unchanged across two consecutive passes —
// TYPES-02 invariant).
//
// Cross-package boundary (D-13): refs whose target lives outside the current
// package emit at last-in-package confidence with validation_state=
// "unresolved". The PER-LANGUAGE resolver owns this guard (it has the file
// path); the fixpoint here only enforces TYPES-04 (any unresolved response
// carries ValidationState="unresolved").
//
// Caveat: cross-package work is explicitly DEFERRED past v1 — the per-
// language resolver caps the confidence at last-in-package and the fixpoint
// records that response without further iteration. A v2 phase will widen
// the package boundary; see D-13.
func FixpointResolve(ctx context.Context, lang Resolver, refs []ChainRequest, maxIter int) ([]ChainResponse, error) {
	responses := make([]ChainResponse, len(refs))
	if len(refs) == 0 {
		return responses, nil
	}
	if maxIter <= 0 {
		maxIter = 1
	}

	prev := stateHash(responses)
	for iter := 0; iter < maxIter; iter++ {
		for i, r := range refs {
			if responses[i].Resolved {
				continue
			}
			resp, err := lang.ResolveChain(ctx, r)
			if err != nil {
				return nil, err
			}
			responses[i] = resp
		}
		h := stateHash(responses)
		if h == prev {
			break // no progress — early exit
		}
		prev = h
	}

	// TYPES-04 invariant: every unresolved response carries
	// ValidationState="unresolved", regardless of what the per-language
	// resolver returned. Defensive: a sloppy resolver that left the state
	// empty (or — worst case — wrote "validated") gets corrected here.
	for i := range responses {
		if !responses[i].Resolved {
			responses[i].ValidationState = "unresolved"
		}
	}
	return responses, nil
}

// stateHash computes a sha256 of the canonical (ordered, index-aligned)
// serialization of the response slice. The fixpoint compares hashes across
// passes to detect no-progress.
func stateHash(resps []ChainResponse) [sha256.Size]byte {
	h := sha256.New()
	var buf [8]byte
	for i, r := range resps {
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		h.Write(buf[:])
		if r.Resolved {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
		binary.BigEndian.PutUint64(buf[:], uint64(r.Target))
		h.Write(buf[:])
		// Confidence is a float64; encode the bit pattern.
		binary.BigEndian.PutUint64(buf[:], math.Float64bits(r.Confidence))
		h.Write(buf[:])
		h.Write([]byte(r.EvidenceKind))
		h.Write([]byte(r.ValidationState))
		h.Write([]byte(r.Source))
		h.Write([]byte(r.Reason))
		h.Write([]byte(strconv.Itoa(i))) // belt-and-braces ordering tag
	}
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

