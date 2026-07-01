---
author: architect
responsible: architect
phase: 135
milestone: v2.12
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.12-ROADMAP.md
  - .planning/milestones/v2.12-REQUIREMENTS.md
  - agent://RedTeamV212
---

# Phase 135 — Extraction foundation: reach C-family files + link var→type

## Goal (the contract Q this phase establishes)

Two extraction-side prerequisites for wiring the type resolvers into production,
proven in isolation against the REAL C extractor:

1. **B1 — reachability:** the production index actually extracts C-family files
   (today `langFromExt` drops `.c/.h/.cpp/.cs/.java` before the extractor runs).
2. **B3 — var→type linkage:** each variable/field/parameter symbol is paired with
   its declared type NAME, so Phase 136's reference producer has a
   `(referencing-symbol NodeID, type-name)` tuple to build a `ChainRequest` from.

**Hoare frame:** `{a C source file with a `struct Foo {…}` decl and a `Foo`-typed
declaration}` `extract → factsFromExtracted-linkage` `{the production pipeline yields
the Foo-typed variable/param symbol AND a correct, deterministic (varNodeID, "Foo")
association derivable from the extracted data}`.

## Why this phase is first (risk)

Both blockers are strictly upstream of all resolver/producer/edge work. If C files
aren't extracted (B1) or there's no usable var→type link (B3), nothing downstream can
succeed. The v2.11 "tiers 2-6 work" proof used a FABRICATED signature
(`type_resolver_plumbing_test.go:25`, `"var x Foo"`) that no real extractor emits —
this phase replaces that fiction with a real-extractor foundation.

## Scope

- **B1:** `langFromExt` (`internal/daemon/semantic_wiring.go:1812-1825`) —
  add `.c/.h→c`, `.cpp/.cc/.cxx/.hpp/.hh/.hxx→cpp`, `.cs→c_sharp`, `.java→java`.
  Update its doc comment. (Rust/Kotlin/PHP/Ruby stay out — v2.13.)
- **B3:** the var→type linkage. **Preferred design (co-capture):** extend each
  C-family extractor's tree-sitter query + provider so a variable/field/parameter
  symbol carries its declared type name on a NEW **in-memory-only**
  `extract.SymbolFact.DeclaredType string` field (populated inside the provider while
  the AST is alive, exactly like the `Fingerprint*` fields). Because the field is
  read by the daemon producer from the raw `ef` **before** `ToStoreFacts`, it need
  NOT be persisted, MUST NOT enter `StableKey`/`SignatureHash`/the store row, and MUST
  NOT change any existing golden. Scope B3 to **C** in this phase (the proof
  language); C++/C#/Java co-capture is a fast-follow within 135 if cheap, else the
  linkage helper generalizes in 136.

## Decisions locked (from v2.12 REQUIREMENTS + red-team fold)

- **Option A (co-driver):** the declared type reaches the resolver via `ChainTokens`
  in Phase 136. THIS phase only produces the (symbol, type-name) association; it does
  NOT touch resolvers, the producer, or edge emission.
- **No StableKey churn (hard constraint):** `DeclaredType` is in-memory only. Existing
  extractor goldens MUST remain byte-identical. If a chosen mechanism would change a
  golden, it is the wrong mechanism — stop and reconsider.
- **Co-capture over range-matching:** prefer tree-sitter co-capture (declared type +
  declarator in one match → same `m.Captures`) over fragile post-hoc range-matching.
  Range-matching is the fallback ONLY if a language's grammar makes co-capture
  infeasible — and must then be deterministic (sorted, same-line nearest-preceding
  type-ref, documented).
- **Determinism (D1 precursor):** the linkage is order-stable; no Go-map iteration in
  the association path.
- **B1 is C-family only:** do not enable rust/kotlin/php/ruby extraction (their
  resolvers are v2.13 stubs; enabling extraction now adds untested surface).

## Centerpiece symbols (ground at PLAN time via SMTC/grep)

- `langFromExt` (`semantic_wiring.go:1812`) — B1 edit point.
- `classifyAndExtract` (`semantic_wiring.go:1842`) — confirms the `lang==""` skip.
- C extractor: `internal/semantic/extract/c/provider.go` (match loop `:94-140`,
  symbol build `:143-186`) + `internal/semantic/extract/c/queries.scm`
  (variable capture at the `declaration`/`init_declarator`; `type.annotation` at
  `(declaration type: (_) @type.annotation)`).
- `extract.SymbolFact` (`internal/semantic/extract/fact.go`) — add `DeclaredType`.
- `extract.ToStoreFacts` (`internal/semantic/extract/to_store.go`) — CONFIRM it does
  NOT copy `DeclaredType` into the store row (it should be dropped at this boundary).
- Golden dir `internal/semantic/extract/c/testdata/` — MUST stay byte-identical.

## Anti-vacuity (the invariant that makes this not theater)

A declaration with a resolvable type yields a NON-empty `(var, typeName)` association;
a declaration with NO type reference (e.g. a bare `int x;` where the type is a C
primitive, or a name with no type) yields NO association. Break-the-linkage guard:
if the helper reported an association for a type-less declaration, it goes RED. This
distinguishes a real linkage from a stub that fabricates a target.

## Acceptance

1. **B1:** an in-process test drives the production extraction path
   (`classifyAndExtract` or the buildFn) over a real `.c` fixture and gets non-empty
   extracted symbols (before this phase: zero, because `langFromExt` returns "").
2. **B3:** for a real C fixture `struct Foo { … };` + `Foo` used as a
   variable/param/field type, the extractor output exposes the correct declared type
   name (`"Foo"`) on the referencing symbol (via `DeclaredType`), and a helper maps it
   to the `(referencing symbol, "Foo")` association.
3. **Anti-vacuity:** a type-less declaration yields no association (guard RED if it does).
4. **Determinism:** repeated extraction/linkage over the same fixture → identical output.
5. **No churn:** every existing `internal/semantic/extract/c/testdata/*` golden is
   byte-identical; `StableKey`/`SignatureHash` unchanged; `DeclaredType` absent from
   the store row (assert `ToStoreFacts` drops it).
6. Zero new Go deps; `go build ./...` + `make vet` (8 vettools) clean;
   affected-package `go test` green.

## Out of scope (Phase 136+)

- Resolver `ChainTokens` consumption (B2) — Phase 136.
- The reference producer, driver, typeIndex injection, edge emission — Phase 136.
- Any `RESOLVES_TO` edge — Phase 136.
- C++/C#/Java linkage IF co-capture there isn't cheap — generalized in 136.
- Real-binary E2E — Phase 137.
