---
author: architect
responsible: architect
phase: 124
milestone: v2.8
status: complete
parent_artifacts:
  - .planning/phases/124-reclaim-dataflows-name/124-CONTEXT.md
---

# SUMMARY — Phase 124: Reclaim the DATA_FLOWS name (B0-a)

## Outcome

Clean cutover. The structural-shape edge that was mis-named `DATA_FLOWS` is now
`STRUCTURAL_TWIN`; the `DATA_FLOWS`/`data_flows` surface kind is **reserved**
(declared + validatable, producerless) for the true interprocedural arg-to-param
flow that v2.8 Workstream B's feature phases will emit. No shims, no stored-data
migration (the edge was build-transient with a single producer).

## What changed

- **Surface enum** (`edge_kind_surface.go`): added `EdgeKindStructuralTwin =
  "structural_twin"` + `MapInternalKind("STRUCTURAL_TWIN")` case; kept
  `EdgeKindDataFlows = "data_flows"` (now reserved, with a comment).
- **Emission** (`semantic_similarity_edges.go`): `dataFlowsEdges` →
  `structuralTwinEdges`; `dataFlowProfileThreshold` → `structuralTwinThreshold`;
  `EdgeKind:"DATA_FLOWS"` → `"STRUCTURAL_TWIN"` (Source stays `ast_profile`);
  header + threshold comments rewritten.
- **Wiring** (`semantic_wiring.go`): call site → `structuralTwinEdges`; comments.
- **validate-graph-edge** (`tools_validate_edge.go`): `structural_twin` added to
  `surfaceToInternalKinds`, the reverse map, the jsonschema enum help, and the
  error string; `data_flows` retained.
- **Tests**: `TestDataFlowsEdges_*` → `TestStructuralTwinEdges_*` (assert
  `STRUCTURAL_TWIN`); e2e `…_SimilarAndDataFlows` → `…_SimilarAndStructuralTwin`;
  `edge_kind_surface_test.go` roundtrip + closed-set lists add `structural_twin`.
- **Comments only**: `fact.go`, `fingerprint.go`, `classifier.go` doc text.
- **Generated** (regenerated, not hand-edited): `internal/cli/verbs_gen.go`,
  `internal/cli/skills/helix/reference.md` via `helix-cligen` / `helix-refgen`.

## Verification

- `go build ./...` clean; `go test ./internal/semantic/... ./internal/daemon/...
  ./internal/skill/semantic/... ./internal/cli/...` → **45 pkg ok**.
- `make vet` (8 vettools) clean; gofmt clean; **`go.mod`/`go.sum` unchanged**.
- `go run ./cmd/helix-cligen --check` + `helix-refgen --check` → both "up to
  date" (generated-file drift gate passes).
- `STRUCTURAL_TWIN` reachable via `explain-symbol-deep` (same unfiltered path as
  SEMANTICALLY_RELATED, proven by `shapeEdges` test).

## What's next (NOT in this phase)

B0-a (the rename) is the *only* decided, scoped piece. The **Workstream B feature
build** — true interprocedural DATA_FLOWS: intraprocedural def-use → arg-to-param
binding at resolved CALLS → interprocedural propagation / taint substrate — is a
genuinely high-effort 3-4 phase build (the scope doc + research both rate it
"High"). It needs its own roadmap + red-team cycle and carries open decisions
(B-Phase-2 storage: free-text `Reason` vs new metadata column; node granularity:
symbol-level vs variable-level). Deferred to a co-driver decision on scope/timing.
