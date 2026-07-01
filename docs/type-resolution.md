# Type Resolution Reference

Helix's semantic graph resolves the static TYPE behind a symbol reference so that
`RESOLVES_TO` / `USES_TYPE` / `CALLS` edges carry a graded confidence instead of a
flat "unresolved". The engine lives in `internal/semantic/types/`; per-language
resolvers live in its subpackages (one per language). The engine is
language-agnostic — only the per-language tier depth varies.

## The contract

Each language implements `types.Resolver` (`internal/semantic/types/resolver.go`):

```
ResolveChain(ctx, ChainRequest)  -> ChainResponse   // resolve an access chain a.b.c
ResolveSymbol(ctx, SymbolRequest) -> SymbolResponse  // resolve one symbol's type
```

The `Dispatcher` (`types.NewDispatcher`) routes a request to the resolver for
`ChainRequest.Language`.

### Production wiring (v2.12 Phase 136 — per-batch, inside the index build)

Type resolution runs **per batch, inside the committed-snapshot index build** —
there is NO daemon-bootstrap dispatcher (the Phase-62 bootstrap dispatcher and
the dead `SetSemanticGraph`/`TypeResolver`/`typeStoreAdapter` seams were removed
in the Phase-136 clean cutover). The path is:

1. The production `buildFn` (`internal/daemon/semantic_wiring.go
   makeProductionBuildFn`) extracts the batch and calls `factsFromExtracted`
   with `withTypeResolution(...)` — the ONLY caller that opts in; the ~20 test
   callers pass no option and skip the driver.
2. As the LAST pass of `factsFromExtracted` (after dedup + the `nameToNode`
   index build, so type names point at definition nodes), `resolveTypeEdges`
   (`internal/daemon/type_resolver_wiring.go`) runs the producer + driver:
   - the C-family **var→type linkage** (`linkVarTypes`, Phase 135) yields the
     `(referencing var/param → type name)` requests;
   - each request's type name feeds the resolver's **ChainTokens annotation
     tier** (Option A, Phase 136) via `NewResolverWithIndex`, which carries the
     batch `typeIndex` (`type name → NodeID`) so the annotation tier binds a
     REAL target node;
   - a per-`(scope, language)` sorted `FixpointResolve` produces
     `ChainResponse`s, and `typeEdgeFromResponse` converts each into a committed
     `RESOLVES_TO` `EdgeFact` (gating `dst=0` / `confidence < 0.45` /
     unresolved), appended to `out.Edges` before the dense EdgeID stamp.
3. `WriteSnapshotFacts` + `CommitSnapshot` persist the edges in the snapshot.

The resolver's `EffectiveReader` seam is backed **in-batch** by
`batchEffectiveReader` (over the current batch's in-memory `out.Symbols`, keyed
by NodeID) rather than the committed store (which is stale at
`factsFromExtracted` time). `QueryEffectiveSymbol` returns the batch symbol fact
(`Signature`/`StableKey`/`Kind`/`Language`/`FilePath`); `QueryEffectiveEdges`
returns nil — the Tier-1 LSP-cascade read stays dead in-batch (deferred, below).

### Read surface (how agents see it)

A committed `RESOLVES_TO` edge surfaces to agents as **`has_type`** via
`helix explain-symbol-deep`: the read path
(`SemanticSkill.handleExplainSymbolDeep` → `SymbolEdgesAccessor.OutgoingEdgesOf`,
no edge-kind filter → `MapInternalKind("RESOLVES_TO") == "has_type"`,
`internal/skill/semantic/edge_kind_surface.go`). This is proven end-to-end
through the REAL `helix` binary for C by
`internal/cli.TestCLI_E2E_CTypeResolution` (Phase 137): a `struct Foo *p`
parameter yields exactly one `has_type` edge p→Foo; a primitive-typed parameter
yields none.

### Deferred surfaces

- **Tier-1 LSP (`QueryEffectiveEdges`)** — the in-batch reader returns nil, so
  the LSP-confirmed `RESOLVES_TO` tier is not consulted during the batch build
  (L5). Wiring the committed/live overlay edges into the batch reader is future
  work.
- **`type_chain` response field** — `explain-symbol-deep` exposes the resolved
  edges (`has_type`), but the richer per-symbol `type_chain` field is backed by
  a separate `TypeChainAccessor` that has no Schema-5 materialization; surfacing
  it needs the Schema-v6 migration.
- **Cross-package / cross-file resolution** — the batch reader + `typeIndex` are
  intra-package/translation-unit only (the v2.11 M1 limit); a type defined in a
  different package/TU is not bound.

## The 7-tier confidence ladder

`internal/semantic/types/ladder.go` (SPEC §38.2). Walked top-down; the first tier
that produces a signal wins:

| Tier | EvidenceKind | Confidence | Signal |
|------|--------------|-----------|--------|
| 1 | `lsp` | 1.00 | LSP-confirmed `RESOLVES_TO` edge (Phase-61 cascade) |
| 2 | `annotation` | 0.90 | explicit typed declaration (`Foo x`, `var x Foo`) |
| 3 | `constructor` | 0.80 | type derivable from a constructor (`new Foo()`, `x := NewFoo()`) |
| 4 | `assignment` | 0.70 | assignment-flow (`y = x`) |
| 5 | `comment` | 0.60 | doc-comment fallback (**capped at 0.60**) |
| 6 | `heuristic` | 0.45 | name-shape match (`userRepo` → `Repository`) |
| 7 | `unknown` | 0.20 | no signal — **emitted, never silently skipped (D-12)** |

Comment edges are capped at 0.60 (`CapCommentConfidence`, TYPES-03) unless
independently LSP-confirmed.

## Per-language resolver depth

11 first-class languages. Resolver depth varies:

| Depth | Languages | Tiers populated |
|-------|-----------|-----------------|
| Full (v1.10) | Go, TypeScript/JavaScript, Python | 1–7 (incl. comment: GoDoc / JSDoc / docstring) |
| **Full (v2.11)** | **Java, C#, C, C++** | 1,2,3,4,6 (**no comment tier — see below**) |
| Stub (LSP-conditional) | PHP, Ruby, Rust, Kotlin | 1 + 7 only (0.20-or-LSP) — v2.12 targets |

The stubs emit `1.00 validated` on an LSP-confirmed edge, else `0.20 unresolved`;
they never silently drop (D-12).

### v2.11 C-family tier tables

Each C-family resolver (`internal/semantic/types/{c,cpp,c_sharp,java}/resolver.go`)
walks the ladder with language-specific parsing:

| Lang | T2 annotation (typed decl) | T3 constructor | T4 assignment | T6 heuristic |
|------|---------------------------|----------------|---------------|--------------|
| C | `struct Foo x`, `Foo *x`, `const Foo x` | — (no ctor syntax) | `y = x` | suffix name-shape |
| C++ | `Foo x`, `Foo x{...}`, `std::vector<int> v` | `new Foo()`, `auto x = Foo{}` | `y = x` | suffix + PascalCase self |
| C# | `Foo x`, `List<W> items`, `Foo[] arr` (`[Attr]` stripped) | `new Foo()`, object init, records | `var x = y`, `x = y` | suffix + PascalCase + `I`-prefix |
| Java | `Foo x`, `List<String> items` (`@Annot` stripped) | `new Foo()`, diamond `<>` | `var x = y`, `x = y` | `*Impl`→iface, `Abstract*`→base, `get*`→prop, suffix, PascalCase |

**Name normalization:** C++/C#/Java strip template/generic args (`Foo<T>`→`Foo`),
namespace/package qualification (`ns::Foo`/`java.util.List`→last segment), and
array/pointer/nullable sigils before matching.

### No comment tier in the v2.11 C-family (T5 deferred)

The four C-family resolvers ship tiers **1,2,3,4,6 — NOT tier 5 (comment)**. There
is no `doc_comment` column in the `semantic_symbols` schema, so the comment tier is
unreachable without a schema migration + extraction backfill. Per the v2.11 red-team
(B2) this is deferred to a future migration milestone. (The Go/TS/Python resolvers,
built earlier against their own extraction, do populate tier 5.)

## Scope models (the D-13 cross-package guard)

A resolved lower-tier type is capped to `unresolved` when it lives outside the
reference's scope (D-13 + Pitfall 5). Scope is per-language:

| Lang | Scope | Cap behavior |
|------|-------|--------------|
| Go | package = directory (`PackageScope` = `filepath.Dir`) | cross-package → unresolved |
| Java | package ≈ directory (strong Java convention) | **intra-package only** (M1) |
| C# | namespace ≈ directory (projection lacks declared namespace) | **intra-namespace only** (M1) |
| C, C++ | flat / program-wide (header linkage) | never caps — **uplift is C/C++-heavy** (M1) |

**Note (updated v2.12 Phase 136):** the cap depends on a `typeIndex`
(parsed-type-name → NodeID) that lets the resolver find the target type's node.
Before Phase 136 the production daemon left `typeIndex` nil (the cap only fired
in tests). Phase 136's batch driver now POPULATES it: `resolveTypeEdges` builds
the index from `nameToNode` (skipping names with `nameCount > 1` — the anti-
mis-bind guard) and injects it via `NewResolverWithIndex`, so the annotation
tier binds a real target node in production. The C/C++ scope is flat (program-
wide / header linkage) so their edges never cap; the batch index is
intra-package/TU only (cross-package targets stay unbound — see deferred
surfaces above).

## Invariants

- **D-12 (always emit):** every reference produces an emit decision; worst case is
  Tier 7 (`unknown`, 0.20). No reference is silently dropped.
- **Leaf boundary:** each resolver depends only on stdlib +
  `internal/semantic/{graph,types}` — no tree-sitter grammar import (facts are
  pre-extracted by Phase 59), no new third-party deps.
- **Deterministic:** regex/name-shape rules are sorted (CR-03); Go's `regexp` is
  RE2 (linear-time, no catastrophic backtracking).

## Honest limitations

- **Comment tier (T5) absent for the C-family** — no `doc_comment` column (above).
- **Heuristic (T6) is best-effort (0.45)** — idiomatic snake_case (esp. C) rarely
  matches the camelCase suffix rules; the tier is a fallback, not a primary signal.
- **End-to-end composition proven for C; inferred for C++/C#/Java** — each
  resolver's logic is unit-tested (fakeStore) and the plumbing is real-store-
  tested (Phase 130, Go). The full production path (extraction → snapshot →
  batch resolver → committed `RESOLVES_TO` → `has_type` via
  `helix explain-symbol-deep`) is proven end-to-end for **C** through the real
  binary (Phase 136 in-process `TestResolveTypeEdges_PositiveCommitsRealEdge` +
  Phase 137 real-binary `TestCLI_E2E_CTypeResolution`). The other three C-family
  languages (C++/C#/Java) share the identical batch wiring but do not yet have a
  dedicated real-binary E2E fixture.
