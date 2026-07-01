---
author: architect
responsible: architect
phase: 136
milestone: v2.12
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.12-ROADMAP.md
  - .planning/milestones/v2.12-REQUIREMENTS.md
  - .planning/phases/135-extraction-foundation-cfamily-vartype-linkage/135-01-SUMMARY.md
  - agent://RedTeamV212
---

# Phase 136 — Resolver ChainTokens + producer + driver + emit (in-process proof)

## Goal (the contract Q this phase establishes)

Wire the type-resolver dispatcher into the committed-snapshot batch build so a
C-family type reference becomes a real, committed `RESOLVES_TO` edge with a non-zero
DstNodeID — proven in-process against the REAL C extractor.

**Hoare frame:** `{Phase 135 yields (refNodeID, typeName) var→type links + a deduped
nameToNode index over committed symbols}` `producer → dispatcher(ChainTokens) →
converter → out.Edges` `{the committed snapshot carries a RESOLVES_TO edge
(src=refNodeID, dst=typeNodeID, validated) that QuerySymbolEdgesOutgoing returns}`.

## What Phase 135 delivered (the input contract)

- `linkVarTypes(extracted) []varTypeLink` → `{RefName, RefStableKey, TypeName}` per C
  var/field/param with a declared named type (deterministic, anti-vacuous).
- Snapshot-level SymbolID dedup runs BEFORE `nameToNode` is built, so `nameToNode`
  keys (bare `s.Name`, e.g. `"Foo"`) map to the DEFINITION node — the resolve target.
- `extract.SymbolFact.DeclaredType` carries the bare type name (in-memory only).

## Decisions locked

- **Option A (co-driver):** the declared type reaches the resolver via
  `ChainRequest.ChainTokens`, NOT by re-parsing the (bare) Signature. Each full
  resolver's annotation tier: if `req.ChainTokens[0]` is non-empty, use it as the
  annotation type name; else fall back to `parseAnnotation(Signature)`. Uniform across
  `c/cpp/c_sharp/java/golang/python/typescript` (no second convention).
- **Per-batch dispatcher (SC2):** `factsFromExtracted` builds its OWN
  `types.NewDispatcher` with per-batch resolvers carrying (a) the batch `typeIndex`
  (from `nameToNode`) and (b) a batch-backed `EffectiveReader` over `out.Symbols`.
  This is the dispatcher's real production consumer.
- **Clean cutover (delete the orphan):** the bootstrap dispatcher
  (`daemon.go:603-628`), the `d.typeResolver`/`d.semanticGraphRanker` struct fields,
  and the dead `Daemon.TypeResolver()` + `Daemon.SetSemanticGraph()` seams
  (`type_resolver_wiring.go` — verified ZERO non-test callers) are REMOVED. No second
  orphaned dispatcher beside the new consumer. If removal proves entangled, STOP and
  route up — do not leave a shim.
- **typeIndex injection:** add `NewResolverWithIndex(reader, typeIndex)` (or a
  `SetTypeIndex`) to each full resolver — uniform.
- **Batch-backed `EffectiveReader`:** resolves NodeID→SymbolFact (incl. FilePath) from
  the in-memory `out.Symbols`. For flat-scope C the D-13 file lookup isn't
  load-bearing (SameScope always true within a translation unit), but build it right
  so cross-package langs behave.
- **Determinism (D1):** the driver groups refs per (package, language) iterating
  SORTED keys, refs in a stable order (RefNodeID, then type name); converts +
  appends to `out.Edges` in that order BEFORE the dense EdgeID stamp
  (`semantic_wiring.go:1610`). No Go-map iteration in the emit path.
- **Confidence gate + D-12:** apply `cfg.MinConfidenceForEdge` (0.45) /
  `EmitUnresolvedEdges`; gate dst=0 (unresolved) rows out pre-append to avoid row
  bloat (L4) UNLESS `EmitUnresolvedEdges` requires them — prefer NOT persisting
  unreadable dst=0 rows.
- **Edge shape:** `EdgeKind="RESOLVES_TO"`, SrcKind/DstKind set (e.g. the referencing
  symbol kind / `"type"`), Source in the bounded allowlist (mirror `emit.go:sourceFallback`),
  Weight=1.0, Confidence from the tier, ValidationState from the response.

## Centerpiece symbols (verified at HEAD)

- `factsFromExtracted` (`semantic_wiring.go:1948`) — the wire point (after dedup + the
  `nameToNode` build at `:2413`, near `resolvePendingDst` `:2432`).
- `nameToNode` / `nameCount` (`:2413-2414`) — typeIndex source + anti-mis-bind guard.
- `types.NewDispatcher` (`resolver.go:148`), `FixpointResolve` (`fixpoint.go:27`).
- Each full resolver's annotation tier (e.g. `golang/resolver.go:76-79`,
  `c/resolver.go:79-82`) + `tierResponse` (`c/resolver.go:112`) + `NewResolver`.
- `cfg.SemanticIndex.TypeResolution.{MinConfidenceForEdge,EmitUnresolvedEdges,MaxFixpointIterations}`
  (`semantic/config.go:398-405`).
- Removal targets: `daemon.go:603-628`, `type_resolver_wiring.go` SetSemanticGraph/TypeResolver,
  struct fields `typeResolver`/`semanticGraphRanker` (`daemon.go:197,203`).
- Read-back: `QuerySymbolEdgesOutgoing` (`effective_graph.go`) + the in-process harness
  (`openDataFlowStore`/`commitFixtureSnapshot`, `semantic_wiring_test.go:114`).

## Anti-vacuity (differential — E1)

The SAME test suite MUST assert BOTH: (i) a POSITIVE C fixture (`struct Foo{};` +
`void g(struct Foo* p){}` in one file) yields EXACTLY ONE `RESOLVES_TO` edge from
`p`'s node to `Foo`'s node; AND (ii) a NEGATIVE fixture (a param of primitive type,
no named type) yields ZERO `RESOLVES_TO` edges. The positive (SC1) gates first — a
green anti-vacuity that only proves "nothing spurious" while nothing real is emitted
is the exact trap the red-team caught; it MUST NOT pass that way.

## Acceptance

1. In-process, REAL C extractor → `factsFromExtracted` → `WriteSnapshotFacts` →
   `QuerySymbolEdgesOutgoing`: the positive fixture commits a `RESOLVES_TO` edge whose
   DstNodeID resolves (via `QueryStableKeyByNodeID`) to `Foo`'s stable key.
2. The dispatcher is invoked by `factsFromExtracted` (SC2) — the per-batch dispatcher
   is the consumer; the old orphaned bootstrap dispatcher + dead seams are REMOVED.
3. typeIndex from `nameToNode`; `nameCount>1` ⇒ no target ⇒ edge gated (not fabricated).
4. Differential anti-vacuity (positive = exactly one edge; negative = zero).
5. Determinism: repeated in-process build → byte-identical `out.Edges` (sorted driver).
6. D-12 + confidence gate honored; dst=0 unreadable rows not persisted.
7. Zero new Go deps; `go build ./...` + `make vet` clean; affected-package tests green;
   no schema migration; existing extractor goldens byte-identical.

## Out of scope (Phase 137)

- Real-binary CLI E2E (`helix explain-symbol-deep`) — Phase 137.
- `docs/type-resolution.md` + "7 languages" doc-drift fix — Phase 137.
- C++/C#/Java linkage depth beyond what 135 delivered (C is the in-process proof;
  cpp/csharp/java ride the same producer where 135 co-capture exists).
- `Store.QueryEffectiveEdges` (Tier-1 LSP) — deferred (L5).
- `type_chain` response field + Schema-v6 migration — deferred (L6).
