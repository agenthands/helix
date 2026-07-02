---
author: architect
responsible: architect
phase: 140
milestone: v2.13
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.13-ROADMAP.md
  - .planning/milestones/v2.13-REQUIREMENTS.md
  - .planning/phases/139-inbody-returnbridge-emission-go-proof/139-CONTEXT.md
  - .planning/phases/137-realbinary-e2e-docs/137-01-SUMMARY.md
  - agent://RedTeamV213
---

# Phase 140 — All-11-language real-binary E2E + function-seeded multi-hop + docs

## Goal (the contract Q this phase establishes)

CONFIRM, through a REAL `helix` binary driven as a subprocess against a live daemon, that
in-body-origin DATA_FLOWS surfaces via `helix explain-symbol-deep` for the languages proven
in Phase 138 (target all 11); prove the function-seeded multi-hop payoff through the binary
for ≥1 language; document the capability + honest per-language limits.

**Hoare frame:** `{Phase 139 commits def_use_inbody + def_use_return edges in the batch
build}` `helix index-semantic-graph → helix explain-symbol-deep / trace_data_flow`
`{the CLI surfaces a data_flows edge producer.function → consumer.param for each proven
language, and a producer→transform→sink multi-hop reachability for ≥1 language}`.

## What's proven already (in-process)

- Phase 138: `AnalyzeFlow` records the InBodyFlow across all 11 grammars (the all-11 unit
  matrix — this phase is CONFIRMATION, not discovery).
- Phase 139: emission + read surface + Go multi-hop proven in-process (`factsFromExtracted`
  → snapshot → walk). This phase proves the SAME through the shipped CLI.

## Decisions locked (grounded at HEAD + v2.12 precedent)

- **Harness (reuse, do NOT reinvent):** the v2.12 E2E oracle
  `internal/cli/cli_type_resolution_e2e_test.go` is the exact template. Its helpers
  (`newE2EFixture`, `switchModeReview` via MCP `switch_mode`, `indexFull` =
  `index-semantic-graph --mode-arg=full`, `explainSymbol` = `explain-symbol-deep
  --seed-json`, absolute seed path, `hasTypeEdges`) transfer directly. Build tag
  `!windows`, pkg `cli_test`, HELIX_BIN-gated (`t.Skip` when no binary).
- **Surface kind:** `MapInternalKind("DATA_FLOWS") == "data_flows"` (edge_kind_surface.go:66).
  Assert `edge_kind=="data_flows"` with `internal_kind=="DATA_FLOWS"`, `Source` distinguishing
  in-body (`def_use_inbody`) is NOT on the surface envelope — assert via the edge's endpoints
  (function→param) and the seed direction.
- **Seed to surface the in-body edge:** a `def_use_inbody` edge is `producer.function →
  consumer.param`. Seed `explain-symbol-deep` on the **consumer parameter** so the edge appears
  as an INCOMING `data_flows` edge (explain applies no kind filter, scans both directions).
  Confirm empirically per language; some grammars may name the param differently — assert the
  data_flows edge exists with the right endpoint kinds, not a hardcoded name if it's fragile.
- **Fixture per language (FLOW-06a):** `local := producer(); sink(local)` idiomatically, inside
  ONE resolvable caller, with `producer` and `sink` defined as resolvable functions in the same
  file (intra-file, flat scope — matching the v2.12 SameScope precedent; cross-file adds
  resolution risk this confirmation phase should avoid). Each language's file wrapped so
  producer/sink/caller are top-level resolvable (class-wrapped for Java/C#/Kotlin).
- **Function-seeded multi-hop (FLOW-06c, M2):** for ≥1 language (Go is the safe choice —
  proven in 139), seed `trace_data_flow` at the producer FUNCTION node and assert the sink is
  reachable via in-body → return-bridge → in-body. The function seed is already mechanically
  accepted (`resolveSeed`/`handleTraceDataFlow` have no kind gate — REQUIREMENTS M2). Broken-hop
  revert-and-fail. Fix any stale seed docs surfaced.
- **Per-language honesty ledger (FLOW-06b):** a language whose tree-shape does not resolve
  E2E is a TESTED, RECORDED limitation (`t.Skip`/subtest skip with a reason) — carried from the
  Phase-138 FLOW-04i ledger. The proven set is stated in the SUMMARY + audit (v2.12 C-proven
  precedent). GOAL: all 11 green; a skip must be justified, not silent.
- **Determinism (FLOW-06d):** re-index the same fixture → identical data_flows edge set.
- **Docs are part of DONE (FLOW-06e):** update the DATA_FLOWS documentation — in-body origins,
  the two `Source` markers (`def_use_inbody`, `def_use_return`), the function-seed interaction
  (now partially usable), and the still-deferred surfaces (variable-level nodes, field/heap
  flow, source/sink taint).

## Anti-vacuity (differential, per the v2.12 precedent)

Each language's positive fixture MUST surface the in-body `data_flows` edge (assert FIRST). A
negative arm — a caller with NO in-body flow (e.g. `sink(literal)` or a pure-param forward) —
MUST surface NO in-body edge while still resolving `exact` (the zero is a real absence, not a
missing symbol). The positive assertion fires first and MUST find the edge via the real binary.

## Acceptance

1. Real `helix explain-symbol-deep` returns an in-body `data_flows` edge (producer.function →
   consumer.param) for the proven set — target all 11; exceptions tested + documented (FLOW-06a/b).
2. Differential anti-vacuity per language: positive has the edge, negative has none.
3. Function-seeded multi-hop proven through the binary for ≥1 language (FLOW-06c); broken-hop RED.
4. Determinism guard green (FLOW-06d).
5. Docs updated with in-body origins + both Source markers + honest deferred-surface ledger.
6. HELIX_BIN-gated (`t.Skip` without a binary); zero new Go deps; `go build ./...` + `make vet`
   clean; the new E2E test passes with a freshly-built binary; per-language proven/skip ledger
   recorded in the SUMMARY.

## Out of scope

- Full function-seed support in the `trace_data_flow` verb's documented contract (fast-follow —
  the multi-hop proof uses the mechanically-accepted function seed only).
- Variable-level graph nodes, field/heap flow, source/sink taint (milestone out-of-scope).
- Any engine/emission change (that was 138/139) — this phase is confirmation + docs only, save
  for stale-doc fixes surfaced by FLOW-06c.
