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
`ChainRequest.Language`. The daemon registers all languages at bootstrap
(`internal/daemon/daemon.go`, `type_resolver_wiring.go`).

Resolvers read the graph through the narrow `EffectiveReader` seam (two reads):
- `QueryEffectiveEdges` — the LIVE `RESOLVES_TO` edges (Phase-61 LSP cascade).
- `QueryEffectiveSymbol` — the LIVE symbol fact (`Signature`/`StableKey`/`Kind`/
  `Language`/`FilePath`) for the referenced node.

The daemon adapter (`typeStoreAdapter`, `type_resolver_wiring.go`) backs the second
read with `Store.QueryEffectiveSymbolFact` (`internal/semantic/store/effective_graph.go`)
— a snapshot JOIN (`semantic_symbols` ⋈ `semantic_files`) at the latest committed
snapshot. **Before v2.11 this adapter returned an empty `SymbolFact`, so every
resolver's tiers 2–6 collapsed to Tier 7 in production; v2.11 Phase 130 wired the
real read.**

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

**Note (pre-existing Phase-62 characteristic):** the cap depends on an optional
`typeIndex` (parsed-type-name → NodeID) that lets the resolver find the target
type's file. The production daemon leaves `typeIndex` nil (for ALL resolvers,
including Go/TS/Python), so in production the cap fires only when a `typeIndex` is
populated (tests). In production, resolvers emit at raw tier confidence.

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
- **End-to-end composition is inferred** — each resolver's logic is unit-tested
  (fakeStore) and the plumbing is real-store-tested (Phase 130, Go). The full
  production path (extraction → snapshot → resolver → emit → graph edge) is not yet
  exercised end-to-end for the four new languages.
