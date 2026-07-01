---
author: engineer
responsible: architect
phase: 135
plan: "01"
milestone: v2.12
status: complete
verified_by: architect (independent, uncached test run + code re-read)
parent_artifacts:
  - .planning/phases/135-extraction-foundation-cfamily-vartype-linkage/135-01-PLAN.md
---

# Phase 135 SUMMARY — Extraction foundation (langFromExt + C var→type linkage)

## Outcome: COMPLETE + independently verified

Both extraction-side blockers closed against the REAL C extractor.

## Changes

- **B1 — `langFromExt`** (`semantic_wiring.go:1812`): added `.c/.h→c`,
  `.cpp/.cc/.cxx/.hpp/.hh/.hxx→cpp`, `.cs→c_sharp`, `.java→java` (rust/kotlin/php/ruby
  stay `""` — v2.13). Doc comment updated.
- **B3 — `extract.SymbolFact.DeclaredType`** (in-memory only, `fact.go`): populated by
  C co-capture (`c/queries.scm` adds `type: (_) @declared.type` to field/variable/
  parameter patterns; `c/provider.go` sets `sf.DeclaredType = bareTypeName(...)`,
  Signature UNCHANGED). Dropped by `ToStoreFacts` (asserted); StableKey unchanged.
- **`linkVarTypes`** (`semantic_vartype_link.go`): pure deterministic helper —
  var/field/param symbol + non-empty DeclaredType → `(RefName, RefStableKey, TypeName)`.
  Slice-order iteration, no map iteration. The INPUT CONTRACT for Phase 136's producer.
- **Deviation (approved) — snapshot-level SymbolID dedup** (`factsFromExtracted` +
  `richerSymbol`): fixes a PRE-EXISTING latent crash B1 exposed. C `signatureHash`
  truncates at `{`, so a struct DEFINITION and a struct TYPE-USE hash identically →
  same StableKey → same SymbolID → PK violation `(snapshot_id, symbol_id)` in
  `WriteSnapshotFacts`. Any C file with a struct def+use aborted the snapshot write.
  Fix: dedup by SymbolID, deterministically preferring the RICHER row (signature with
  `{` = definition, then longer signature, then first-seen). Runs BEFORE `nameToNode`
  so the name index points at the definition node. Safe no-op for non-colliding langs.

## Deferred follow-up (noted, out of scope)

- The C extractor emits struct TYPE-USES as `definition.struct` symbols (root of the
  collision). Fixing it in the provider would churn the `with_fields` golden — deferred.

## Verification (architect, independent)

- `TestProductionBuildFn_ReachesCFile`, `TestFactsFromExtracted_CStructDedup`,
  `TestLinkVarTypes_*` (StructParam/TypedefVar/AntiVacuity/MixedAntiVacuity/
  Determinism/NilSafety), `TestToStoreFacts_DropsDeclaredType`, `TestLangFromExt_CFamily`
  — all PASS uncached (`-count=1 -v`).
- C goldens byte-identical: `TestProvider_Golden` 12 subtests PASS, no `-update`;
  `git status internal/semantic/extract/c/testdata/` empty.
- `go build ./...` clean; `make vet` clean (8 vettools, exit 0);
  `go.mod`/`go.sum` byte-unchanged.
- Ordering confirmed: dedup (`:2340`) before `nameToNode` (`:2413`).

## Handoff to Phase 136

`linkVarTypes(extracted) []varTypeLink` yields `(RefStableKey, TypeName)` per C
var/field/param. 136's producer: map RefStableKey → NodeID, build
`ChainRequest{RefNodeID, ChainTokens:[TypeName], RefKind:"RESOLVES_TO"}`, feed the
resolver (annotation tier consumes ChainTokens), populate typeIndex from `nameToNode`
(now pointing at deduped definition nodes), emit EdgeFacts.
