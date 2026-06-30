---
author: architect
responsible: architect
phase: 125
milestone: v2.9
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.9-REQUIREMENTS.md
  - .planning/research/semantically-related-and-true-dataflows-scope.md
---

# Phase 125 — Intraprocedural flow-summary engine

## Goal (the contract Q this phase establishes)

A pure-Go, deterministic per-function **case-1 flow summary** over the tree-sitter
body: for each parameter, does it *reach* the return, and/or reach the argument
position of an in-body call? This summary is the input the Phase 126 emission
composes into interprocedural `DATA_FLOWS` edges.

**Hoare frame:** `{body is a fingerprintable function/method}` `AnalyzeFlow` `{FlowSummary
holds, per param, the exact set of {reaches-return, reaches-call(name,argPos)} derivable
from syntactic def-use — case-1 only, no in-body origins}`.

## Scope

- New leaf pkg `internal/semantic/dataflow/` (stdlib + tree-sitter only).
- `AnalyzeFlow(body *tree_sitter.Node, source []byte) FlowSummary`.
- New `SymbolFact.FlowSummary` field (`internal/semantic/extract/fact.go`); computed in
  the `FingerprintBody` path so all 11 providers inherit with no per-provider edit.
- Carry on the daemon-side `fingerprintedNode` (`semantic_similarity_edges.go`).

## Decisions locked (from v2.9 REQUIREMENTS)

- **Case-1 only:** param → {return, call-arg}. No `src()`/in-body origins (deferred to
  variable-level nodes). This is the cheaper honest cut (red-team Q1).
- **Param identity from the AST, not symbol emission:** `AnalyzeFlow` derives the
  function's param names/positions from the function-definition's parameter-list child
  node. It does NOT depend on the order symbols are emitted into `single.Symbols`
  (that order is consumed separately by Phase 126's emit-order-adjacency resolution).
- **Determinism is structural:** no Go-map-iteration in the analysis path (FLOW-01d).
- **Param-index pinning (FLOW-01f):** the summary's arg-position encoding MUST match the
  param-node emit order Phase 126 uses. Pin this contract here.

## Centerpiece symbols (ground at PLAN time via SMTC/grep)

- `extract.FingerprintBody` (`fingerprint.go:38`) — the seam to extend.
- `extract.IsFingerprintableKind` (`fingerprint.go:55`) — the function/method gate.
- `extract.SymbolFact` (`fact.go:90`) — add `FlowSummary`.
- The per-provider `FingerprintBody` call site (e.g. `golang/provider.go`) — confirm no
  per-provider edit needed (the seam inherits it).
- Sibling leaf pkgs `minhash/`, `relatedidx/`, `classifier/` — the boundary to mirror.

## Anti-vacuity (the invariant that makes this not theater)

A parameter that is **not forwarded** yields an empty summary for that param.
`f(x){}` → no flows. Revert-and-fail: if the analyzer reported a flow for a dead param,
the guard goes RED. This is what distinguishes a real def-use walk from a stub.

## Acceptance

1. `f(x){return x}` → x reaches return.
2. `f(x){sink(x)}` → x reaches call `sink` arg0.
3. `f(x){y:=x; g(y)}` → x reaches call `g` arg0 (transitive through a local).
4. `f(x){}` → **no flows** (anti-vacuity).
5. Determinism: re-extract → identical summary (structural, not seeded).
6. A provider e2e test: `FlowSummary` populated for a Go function body, nil for a
   container kind.
7. Pure unit tests; no network; zero new deps; `go build ./...` + `make vet` clean.

## Out of scope

- Emitting any edge (that is Phase 126).
- Interprocedural composition / reachability (Phase 126).
- In-body origins / variable-level nodes (deferred — milestone out-of-scope).
