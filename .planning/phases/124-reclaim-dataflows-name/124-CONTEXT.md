---
author: architect
responsible: architect
phase: 124
phase_type: implementation
hard_bar: false
security_relevant: false
design_fork: false
status: in_progress
parent_artifacts:
  - .planning/research/semantically-related-and-true-dataflows-scope.md
  - .planning/REQUIREMENTS.md
---

# Phase 124 CONTEXT — Reclaim the DATA_FLOWS name (B0-a)

## Decision (co-driver, 2026-06-30)

B0-a: **rename the structural-similarity edge** (the `ast_profile` cosine edge
currently mis-named `DATA_FLOWS`) to **`STRUCTURAL_TWIN`** / surface
`structural_twin`, and **reserve `DATA_FLOWS`/`data_flows`** for the real
arg-to-param interprocedural flow that Workstream B's feature phases will emit.

## Why STRUCTURAL_TWIN (not fold into SIMILAR_TO)

The structural edge is a DISTINCT signal from SIMILAR_TO: SIMILAR_TO is MinHash
token-trigram near-clone (Jaccard ≥ 0.95); the structural edge is control-flow /
expression-shape profile cosine (≥ 0.985). Folding them conflates two different
notions of "alike". Keep it a separate, honestly-named edge.

## Clean cutover (no shims)

`DATA_FLOWS` currently has exactly one producer (`dataFlowsEdges`, Source
`ast_profile`) and is build-transient (never persisted), so there is no stored
data to migrate and no back-compat surface to preserve. Rename outright:
- `EdgeKind` string `"DATA_FLOWS"` → `"STRUCTURAL_TWIN"` at the emission site.
- surface enum: add `EdgeKindStructuralTwin = "structural_twin"`; KEEP
  `EdgeKindDataFlows = "data_flows"` (now producerless, reserved for real flow —
  `validate-graph-edge` still accepts it).
- Go identifiers: `dataFlowsEdges`→`structuralTwinEdges`,
  `dataFlowProfileThreshold`→`structuralTwinThreshold`.
- Update all comments that describe the edge.

## Blast radius (grep-verified)

- `internal/skill/semantic/edge_kind_surface.go` (+`_test.go`) — enum + mapper.
- `internal/skill/semantic/tools_validate_edge.go` — surfaceToInternalKinds, the
  jsonschema enum help, the error string.
- `internal/daemon/semantic_similarity_edges.go` — emission fn, threshold, comments.
- `internal/daemon/semantic_wiring.go` — call site + comments.
- `internal/daemon/semantic_similarity_edges_test.go`,
  `semantic_similarity_e2e_test.go` — rename tests + edge-kind assertions.
- `internal/semantic/extract/{fact.go,fingerprint.go}`,
  `internal/semantic/classifier/classifier.go` — doc comments only.
- **Generated** (regenerate, do NOT hand-edit — CI `--check` gate):
  `internal/cli/verbs_gen.go`, `internal/skill/helix/reference.md` via
  `go run ./cmd/helix-cligen` + the refgen/docgen.

## Acceptance

1. No emission of `EdgeKind:"DATA_FLOWS"` remains; the structural edge emits
   `"STRUCTURAL_TWIN"` (Source `ast_profile`).
2. `structural_twin` is in the closed surface enum + `MapInternalKind`; `data_flows`
   retained (reserved, producerless).
3. Generated `verbs_gen.go` + `reference.md` regenerated; `helix-cligen`/refgen
   `--check` (and `make verify-docs` if present) clean — no drift.
4. `go build ./...`, full test sweep, `make vet`, gofmt all green; `go.mod` unchanged.
5. RESUME depth-gap #3 updated: the structural edge is now `STRUCTURAL_TWIN`;
   `DATA_FLOWS` reserved for the real-flow work.

## Out of scope
The actual interprocedural DATA_FLOWS feature (def-use, arg→param, propagation) —
that is Workstream B's feature phases, a separate roadmap + red-team cycle.
