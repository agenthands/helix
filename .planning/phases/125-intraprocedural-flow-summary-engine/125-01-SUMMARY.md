---
author: architect
responsible: architect
phase: 125
milestone: v2.9
status: complete
plan: 125-01
parent_artifacts:
  - .planning/phases/125-intraprocedural-flow-summary-engine/125-01-PLAN.md
---

# SUMMARY 125-01 — Intraprocedural flow-summary engine

## Outcome: DELIVERED

A pure-Go leaf package `internal/semantic/dataflow/` computes a case-1
param→{return, call-arg} flow summary over a function body, plumbed through the
shared `FingerprintBody` seam so all 11 providers inherit it with no per-provider
edit. Foundation for Phase 126's interprocedural DATA_FLOWS emission.

## What shipped

- **`internal/semantic/dataflow/summary.go`** (new, ~280 LOC) — `Summary`/`ParamFlow`/
  `CallArgTarget` types + `AnalyzeFlow(node, source) *Summary`:
  - Collects ordered param names from the declaration's parameter list.
  - Seeds taint (param → {its index}); forward-walks the body propagating taint
    through assignments, recording `Returns` (param value reaches a return) and
    `CallArgs` (param value reaches argument position N of a call to a named
    callee). Skips parameter-list subtrees so params aren't read as value uses.
  - Cross-language tree-sitter node-kind unions (mirrors `classifier.walkProfile`).
  - Deterministic: CallArgs dedup'd + sorted on (Callee, ArgPos); Params in index
    order; no map iteration in ordered output.
  - Anti-vacuity: a dead param (no target) is omitted; nil Summary when nothing
    reaches anything.
- **`internal/semantic/extract/fact.go`** — new `SymbolFact.FlowSummary *dataflow.Summary`
  field; fixed the stale fingerprint doc comment (structural edge is `STRUCTURAL_TWIN`,
  not DATA_FLOWS).
- **`internal/semantic/extract/fingerprint.go`** — `FingerprintBody` calls
  `dataflow.AnalyzeFlow(body, source)`, stamping `sf.FlowSummary`. All 11 providers
  inherit (they all call FingerprintBody).
- **`internal/daemon/semantic_similarity_edges.go`** — `fingerprintedNode` carries a
  new `flow *dataflow.Summary` field (Phase 126 reads it).
- **`internal/daemon/semantic_wiring.go`** — `fpNodes` collection populates `flow`;
  skip-condition now includes `FlowSummary` so small forwarding functions (nil
  MinHash but non-nil FlowSummary) are not dropped.

## Tests (all green)

- `internal/semantic/dataflow/dataflow_test.go` — 8 tests: param→return,
  param→call-arg, **transitive through a local** (`y := x; g(y)`), **dead-param
  anti-vacuity** (`f(x){return 7}` → nil), no-params→nil, determinism, nil-node,
  two-params-distinct.
- `internal/semantic/extract/golang/provider_flowsummary_test.go` — e2e proving the
  real Go provider stamps FlowSummary (`fromAccount` reaches
  `validateInsufficientLedger` arg0) and leaves a container kind nil.

## Verification

- `go build ./...` — clean.
- `go test ./internal/semantic/dataflow/... ./internal/semantic/extract/...
  ./internal/daemon/... ./internal/skill/semantic/...` — 15 pkgs ok, 0 fail.
- `go vet` on the affected packages — clean.
- **Zero new Go deps** — `go.mod`/`go.sum` byte-unchanged (dataflow imports only
  tree-sitter + stdlib; tree-sitter was already a dep).

## Red-team fold honored

- Case-1 only (Q1): exact syntactic param→X data dependence; no in-body origins
  (deferred variable-level work). Anti-vacuity is the load-bearing honesty guard.
- Param identity from the AST, not symbol emission order (the FlowSummary `Index`
  is the 0-based param position Phase 126 will align to the emit-order-adjacency
  callee-param resolution — FLOW-01f pin in the field doc).

## Out of scope (Phase 126)

- Any DATA_FLOWS edge emission, callee resolution, read-surface confirmation,
  reachability proof.
