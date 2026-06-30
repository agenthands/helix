---
author: architect
responsible: architect
phase: 121
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
status: in_progress
parent_artifacts:
  - .planning/milestones/v2.8-ROADMAP.md
  - .planning/REQUIREMENTS.md
---

# Phase 121 CONTEXT — Random-Indexing Engine

## Contract (Hoare frame)

**{P}** A function-body tree-sitter node + source exist (the same input
`minhash.ComputeSignature` / `classifier.ComputeProfile` already consume).

**{S}** Build `internal/semantic/relatedidx/` — a Random-Indexing engine that
projects a body's *vocabulary* (identifier + comment tokens) into a fixed-dim
context vector.

**{Q}** For two bodies, `cosine(vecA, vecB)` is high iff they share vocabulary/
domain, and is **independent of structural shape** — the property that makes the
resulting SEMANTICALLY_RELATED edge orthogonal to SIMILAR_TO (MinHash, which is
pure structure). The vector is **deterministic** (byte-identical across runs).

## Why this is the keystone phase

RI is the only new algorithm with real risk. Everything downstream (122 plumbing,
123 emission) is mechanical once the engine is proven. So it ships as an isolated
leaf package, fully unit-tested, before any wiring.

## The distinctness invariant (load-bearing — this is the milestone's whole point)

MinHash (`minhash.go`): `normalizeKind` collapses every identifier→"I",
string→"S", number→"N", type→"T", then trigrams over the **structural node-kind
stream**. It deliberately **discards vocabulary**. → SIMILAR_TO = "same shape".

RI (this phase): keep the **identifier/comment text**, discard structure. →
SEMANTICALLY_RELATED = "same vocabulary/domain". The two signals are orthogonal
by construction. The Phase-123 distinctness guard exists to *prove* this on real
bodies; this phase must make the orthogonality real, not just intended.

## Design decisions (locked)

- **Algorithm:** classic Random Indexing. Each unique token → a sparse ternary
  index vector (a few deterministic ±1 entries, positions+signs seeded from the
  token hash). A body's context vector = elementwise sum of its token index
  vectors (bag-of-tokens projection; Johnson–Lindenstrauss preserves cosine).
- **Determinism — the hazard and the fix:** float summation is order-sensitive.
  Accumulate into an **`int32` array** (integer add is associative + commutative
  → order-independent, exact); convert to float64 **only** inside `cosine`. No Go
  map iteration in the hot path that affects the vector value.
- **Dimension:** `D = 256` named const + rationale comment (ample for JL at our
  vocabulary sizes; 1KB/symbol as int32 — bounded; revisit only if perf shows it).
- **Nonzeros per token:** small fixed `K` (e.g. 8) seeded ±1 — the RI "spray".
- **Token extraction:** walk AST leaves; for identifier-kind leaves take the
  **text**, split camelCase + snake_case into subtokens, lowercase; include
  comment tokens; **skip** punctuation/operators/keywords-as-structure. This is
  the deliberate inverse of `collectLeafTokens`+`normalizeKind`.
- **Min-token gate:** mirror `minhash.MinNodes` — too few vocabulary tokens → no
  vector (`ok=false`), so trivia never gets a relatedness signal.
- **Dep boundary:** stdlib + `github.com/tree-sitter/go-tree-sitter` +
  `github.com/cespare/xxhash/v2` (already in go.mod). No `internal/*` deps, no new
  module. Leaf package exactly like `minhash/`.

## Acceptance (verifiable)

1. `internal/semantic/relatedidx/relatedidx.go` + `_test.go` exist; package is a leaf.
2. `ComputeContextVector(tokens)` / a body-level `ComputeVector(body, source) (*Vector, bool)` are deterministic — re-run yields byte-identical vectors (test).
3. `Cosine(v,v)==1` (within float epsilon); same-vocabulary fixture pair `>` unrelated pair (test).
4. **Orthogonality test:** two bodies with identical structure but disjoint vocabulary score LOW on RI cosine (proving RI ≠ MinHash). Two bodies with shared vocabulary but different structure score HIGH.
5. Token extraction keeps identifier/comment text, not normalized codes (test asserts a known identifier subtoken is present in the token stream).
6. Min-token gate returns `ok=false` for a tiny body.
7. `go test ./internal/semantic/relatedidx/` green; `make vet` clean; `go.mod` byte-unchanged.

## Out of scope
Plumbing into providers (122), edge emission (123), any neural model.
