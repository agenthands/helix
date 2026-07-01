---
author: engineer
responsible: architect
phase: 136
plan: "01"
milestone: v2.12
status: complete
verified_by: architect (independent, uncached test run + full code re-read of producer/driver + cutover grep)
parent_artifacts:
  - .planning/phases/136-resolver-chaintokens-producer-emit/136-01-PLAN.md
---

# Phase 136 SUMMARY — Resolver ChainTokens + producer/driver/emit (in-process proof)

## Outcome: COMPLETE + independently verified

The orphaned type-resolver dispatcher is now wired into the committed-snapshot batch
build. A C-family var→type reference becomes a real, queryable `RESOLVES_TO` edge —
proven in-process against the REAL C extractor + real `*Store`.

## Changes

- **B2 + A2 (7 full resolvers, `internal/semantic/types/{c,cpp,c_sharp,java,golang,python,typescript}/resolver.go`):**
  annotation tier prefers `req.ChainTokens[0]` when non-empty (Option A), else falls
  back to the language `parseAnnotation` — uniform, tier/evidence/confidence/D-12
  preserved. Added `NewResolverWithIndex(store, idx)`; `NewResolver` delegates with nil.
- **Daemon producer/driver (`type_resolver_wiring.go`, rewritten):** `batchEffectiveReader`
  (EffectiveReader over in-memory `out.Symbols`, `QueryEffectiveEdges` nil — L5);
  `newIndexedResolver`; `resolveTypeEdges` (typeIndex from `nameToNode` skipping
  `nameCount>1`; RefStableKey→NodeID via `CanonicalizeStableSymbolKey`; ChainRequests
  from `linkVarTypes`; sorted (lang,file) groups + sorted refs; per-batch
  `NewDispatcher`; `FixpointResolve`; `typeEdgeFromResponse` gates dst=0 / conf<0.45 /
  unresolved; appends before the dense EdgeID stamp).
- **B1 (`semantic_wiring.go`):** `semanticConfig` gains `TypeResolMaxFixpoint(8)`/
  `TypeResolMinConfidence(0.45)`/`TypeResolEmitUnresolved(false)` defaulted in
  `loadSemanticConfig`. `factsFromExtracted` gained a variadic `opts ...factsOption`
  (Task B1 decision — least churn: ~13 existing callers pass none, compile unchanged;
  the production buildFn opts in via `withTypeResolution`). `resolveTypeEdges` runs as
  the last pass before `return out` (after dedup + `nameToNode`).
- **C1 clean cutover:** removed the orphaned bootstrap dispatcher (`daemon.go:603-628`),
  `typeResolver`/`semanticGraphRanker` struct fields + assignment + the now-unused
  `semantic/types` import, and the dead `Daemon.TypeResolver()`/`SetSemanticGraph()` +
  `typeStoreAdapter` + per-language wrappers. Removed `type_resolver_plumbing_test.go`
  (its subjects `newTypeStoreAdapter`/`goTypeResolver` were deleted — no shim). Grep
  confirms zero non-test references remain.

## Task B1 decision

Variadic `opts ...factsOption` on `factsFromExtracted` running the driver INSIDE the
function where `nameToNode`/`nameCount` live — the ~13 existing test callers pass no
opts and compile byte-unchanged; only the production buildFn opts in. Chosen over
returning `nameToNode` (which would churn every caller).

## Verification (architect, independent)

- `TestResolveTypeEdges_PositiveCommitsRealEdge` (SC1, exactly one `RESOLVES_TO` p→Foo,
  dst stable key == Foo via `QueryStableKeyByNodeID`), `_NegativeNoEdge`,
  `_Deterministic` (byte-identical twice, non-vacuous), `_AntiMisBind` — all PASS
  uncached (`-count=1 -v`).
- `go test ./internal/semantic/types/... ./internal/daemon/...` — 12 packages ok
  (uncached). Ladder tests green with the fixture `ChainTokens:["x"]→[]` change (the
  old value was an inert var-name; pre-136 the annotation leaf ignored ChainTokens —
  emptying preserves each fixture's Signature-path intent; expected outputs unchanged).
- Cutover: `grep 'TypeResolver()|SetSemanticGraph|typeStoreAdapter'` (non-test) → none.
- `go build ./...` clean (whole tree → no broken caller from the removal);
  `make vet` clean (8 vettools, exit 0); `go.mod`/`go.sum` byte-unchanged; extractor
  goldens (`internal/semantic/extract/*/testdata/`) untouched.

## Handoff to Phase 137

The committed snapshot now carries `RESOLVES_TO` edges for C var→type. Phase 137 proves
it through the real `helix` binary: `activate_project` → `switch-mode review` →
`index-semantic-graph --mode=full` → `explain-symbol-deep --seed-json=…` → assert a
`has_type` edge (RESOLVES_TO maps to has_type via MapInternalKind); + docs.
