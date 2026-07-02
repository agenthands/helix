---
author: architect
responsible: architect
phase: 139
milestone: v2.13
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.13-ROADMAP.md
  - .planning/milestones/v2.13-REQUIREMENTS.md
  - .planning/phases/138-origin-space-widening-unions/138-CONTEXT.md
  - agent://RedTeamV213
---

# Phase 139 — In-body + return-bridge emission, read surface, Go multi-hop proof

## Goal (the contract Q this phase establishes)

Emit the two new DATA_FLOWS edge kinds from Phase-138 summaries; prove they are a
distinct, honest, non-regressing edge set reachable from the CLI read surface; prove
multi-hop `src → transform → sink` reachability **in-process on real Go**; and correct the
now-false param-anchored invariants.

**Hoare frame:** `{a committed snapshot carries Phase-138 InBodyFlows + per-param Returns}`
`factsFromExtracted (+ inBodyDataFlowEdges, return-bridge pass)`
`{DATA_FLOWS gains function→param (def_use_inbody) + param→function (def_use_return) edges,
the v2.9 def_use param→param SET is unchanged for the 6 working langs, and src→sink is
reachable via in-body → return-bridge → in-body}`.

## Scope (files)

- `internal/daemon/semantic_similarity_edges.go` — new `inBodyDataFlowEdges` + return-bridge
  emission (mirroring `dataFlowEdges`, sharing its sorted `withFlow` list).
- `internal/daemon/semantic_wiring.go` — wire the new passes in `factsFromExtracted`
  **STRICTLY AFTER** the `dataFlowEdges` call (:2534); correct the M3 comment (:2810-2813).
- `internal/skill/semantic/accessors.go:285-293` — correct the M3 doc-comment.
- `internal/daemon/dataflow_reachability_test.go` — rename/repurpose
  `TestDataFlowReachability_FunctionSeedEmpty` (:138-161).
- New/extended daemon tests — distinctness, honesty/anti-mis-bind, def_use-SET
  non-regression, read-surface, Go multi-hop.

## Decisions locked (co-driver + red-team fold; grounded at HEAD)

- **Edge contracts (the load-bearing shapes):**
  - **in-body:** `Src=producer FUNCTION node` (via `nameToNode`, `nameCount==1`);
    `Dst=consumer PARAMETER node j` (via `nameToNode`→node, `nodeToParams[node][j]`);
    `SrcKind="function"`, `DstKind="parameter"`, `EdgeKind="DATA_FLOWS"`,
    `Source="def_use_inbody"`, `Confidence≈0.50`, `Weight` per sibling convention,
    `Reason="return of {producer} -> {consumer} param{j}"`.
  - **return-bridge:** `Src=the parameter node`, `Dst=the enclosing FUNCTION node`,
    `SrcKind="parameter"`, `DstKind="function"`, `EdgeKind="DATA_FLOWS"`,
    `Source="def_use_return"`, `Confidence≈0.55`, `Reason="param{i} -> return"`.
- **M1 (append order) — HARD constraint:** the new passes append **strictly after**
  `dataFlowEdges` (`semantic_wiring.go:2534`). EdgeID is a dense 1-based index over ALL
  edges in final append order; wiring the new passes anywhere but after `dataFlowEdges`
  shifts existing RESOLVES_TO/etc. EdgeIDs. This is not "beside the siblings" — it is
  strictly after.
- **M4 (return-bridge determinism) — HARD constraint:** the bridge pass MUST iterate the
  **same sorted `withFlow` node list** as `dataFlowEdges` (`semantic_similarity_edges.go:484`)
  and use `nodeToParams`/`nameToNode` for LOOKUP only — **NEVER range a Go map** in the
  output path. Directed-deduped on `(src,dst)`.
- **D-HONESTY (FLOW-05c):** external/overloaded/unresolved endpoint ⇒ endpoint node 0 ⇒
  gated out pre-append (no fabricated edge). Two same-named producers (`nameCount>1`) ⇒ no
  in-body edge (anti-mis-bind).
- **M1 non-regression (FLOW-05e) — reframed honestly:** the `Source=="def_use"` param→param
  edge **SET** (by content Src/Dst/SrcKind/DstKind/Source/Confidence/Weight/Reason) is
  UNCHANGED for the 6 already-working languages — asserted by an explicit def_use-subset
  test on a Go fixture (NO EdgeID golden exists today; a mis-wire would otherwise go
  uncaught). This is NOT a whole-snapshot byte-identity claim (impossible on mixed repos —
  EdgeIDs shift). Snapshot determinism (byte-identical for a FIXED input) is preserved.
  **Disclosure:** Ruby/Python/Kotlin *newly* emit v2.9 param→param edges as a consequence
  of Phase 138's union fix — a folded latent-v2.9-bug-fix, stated in the SUMMARY, not silent.
- **m1:** NO "total-edge budget" claim. Return-bridge volume is O(#params × #functions) —
  linear, bounded; FLOW-05b relies on `(src,dst)` directed dedup, not a budget.

## Anti-vacuity + distinctness (what makes this not theater)

- **FLOW-05d distinctness, mutation-confirmed:** an in-body-only body ⇒ in-body edges + ZERO
  v2.9 param→param edges; a param-only body ⇒ v2.9 edges + ZERO in-body edges. Both distinct
  from SIMILAR_TO / STRUCTURAL_TWIN / SEMANTICALLY_RELATED (different Source markers, kinds).
- **FLOW-05g Go multi-hop, mutation-confirmed:** in-process real Go provider → snapshot →
  walk: `src() → transform → sink` reachable via in-body → return-bridge → in-body. Break a
  hop → reachability breaks (revert-and-fail RED).

## Read surface (FLOW-05f)

`explain_symbol_deep`'s `IncomingEdgesOf`/`OutgoingEdgesOf` apply NO edge-kind filter
(proven for SEMANTICALLY_RELATED by `explain_symbol_relatededge_test.go`), and
`MapInternalKind("DATA_FLOWS")` already maps — so the new edges surface once committed. Assert
an in-body + a bridge edge appear in `edges_outgoing`/`edges_incoming`; assert
`trace_data_flow` BFS traverses function-anchored edges WITHOUT error (function-node
endpoints must not crash the param-seeded walk).

## M3 invariant correction (FLOW-05h) — the invariants that go FALSE this phase

Once `function → parameter` (in-body) and `parameter → function` (bridge) edges exist,
"DATA_FLOWS is purely param-anchored / a function seed cannot reach them" is FALSE. Correct
BOTH comments (`accessors.go:285-293`, `semantic_wiring.go:2810-2813`) and rename/repurpose
`TestDataFlowReachability_FunctionSeedEmpty` to reflect that a function seed now MAY reach
in-body targets. (Note: the v2.10 `trace_data_flow` verb's *documented* seed contract is
still a param; full function-seed verb support stays a fast-follow — Phase 140 proves the
mechanically-already-accepted function seed for the multi-hop payoff.)

## Acceptance

1. In-body + return-bridge edges emitted on a real Go fixture per the exact contracts above.
2. Distinctness (FLOW-05d) + honesty/anti-mis-bind (FLOW-05c) + def_use-SET non-regression
   (FLOW-05e) guards green; mutation ⇒ RED.
3. Multi-hop reachability proven in-process (FLOW-05g); broken-hop guard RED.
4. New edges reachable via `explain_symbol_deep`; `trace_data_flow` non-erroring on
   function-anchored edges; M3 comments + test corrected.
5. Append-order (M1) and map-free-output (M4) constraints honored — verify by reading the
   wired call site, not just by green tests.
6. Zero new deps; no schema migration; `make vet` clean; affected-package tests green;
   any generated-file `--check` gates pass.

## Out of scope

- Real-binary E2E across 11 languages (Phase 140).
- Full function-seed support in the `trace_data_flow` verb contract (fast-follow).
- Variable-level graph nodes, field/heap flow, source/sink taint (milestone out-of-scope).
