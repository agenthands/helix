# Phase 73: P1 Tools Integration & E2E Verification - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-20
**Phase:** 73-p1-tools-integration-e2e-verification
**Areas discussed:** Profile-filter golden test shape, E2E populated-workspace fixture strategy, get_tool_help extraction depth, Skill-wrapper consistency enforcement

---

## Profile-filter golden test shape

| Option | Description | Selected |
|--------|-------------|----------|
| Extend inline table-driven | Add the 6 P1 tools to the existing hasAll/hasNone table-driven pattern in profile_filter_test.go | ✓ |
| Single committed golden JSON | One matrix snapshot to testdata/profile_matrix.golden.json with -update regen | |
| Per-cell snapshot files | One golden file per (profile,mode) cell, 20 files in testdata/ | |

**User's choice:** Extend inline table-driven
**Notes:** Keeps consistency with the existing P0 `semanticToolNames` test; Go-readable diffs survive matrix changes better than JSON snapshots.

---

## E2E populated-workspace fixture strategy

| Option | Description | Selected |
|--------|-------------|----------|
| One shared real-store fixture | buildP1E2EFixture(t) in package semantic_test extending productionAdapterFixture, reused by all 6 tool E2E tests | ✓ |
| Per-tool real-store setup | Each of the 6 E2E tests builds its own tempdir Store | |
| Extend productionAdapterFixture in place | Grow the existing 3-file fixture to cover all 6 tools | |

**User's choice:** One shared real-store fixture
**Notes:** SC#4 explicitly demands real *Store + bleve + tempdir — the existing populated_graph_fixture_test.go is an in-memory mock and is insufficient. Honors the "DO NOT inline-copy this fixture" norm.

---

## get_tool_help extraction depth

| Option | Description | Selected |
|--------|-------------|----------|
| Verify-only, keep existing Help prose | Only add tests asserting get_tool_help returns param docs; keep *Help consts as-is | |
| Standardize Help consts to a template | Normalize all 6 *Help consts to a fixed section layout, then add coverage tests | ✓ |
| Audit + fix gaps only | Fill specific missing sections/param descriptions, no blanket rewrite | |

**User's choice:** Standardize Help consts to a template
**Notes:** All 6 P1 tools already carry hand-written *Help consts + jsonschema-typed args. Phase 73 normalizes their section layout (Usage Examples / Parameters / Returns) for uniform get_tool_help output. The jsonschema.For[T] extraction mechanism is locked by SC#2.

---

## Skill-wrapper consistency enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| In-tree static gate, readonly_gate style | wrapper_consistency_test.go scans all 6 P1 handlers asserting mode-check + FreshnessV2 + RegisterAll | ✓ |
| Manual audit checklist in VERIFICATION.md | gsd-verifier audits handlers at verification time, no standing test | |
| Extend readonly_gate to all 6 + add wrapper gate | Widen readonly_gate first, then add wrapper gate | |

**User's choice:** In-tree static gate, readonly_gate style
**Notes:** The repo has no external CI linter (established 71-01), so a standing in-tree test is the enforcement mechanism. readonly_gate's gatedHandlerFiles already covers all 6 handlers (Phase 72 extended it) — no further extension needed, only the new wrapper gate.

---

## Claude's Discretion

- Exact section headings/wording of the standardized *Help template (must be identical structure across all 6).
- Precise multi-language symbol/edge/cluster shape of buildP1E2EFixture (must yield non-degenerate results with all closed-enum envelope fields populated for every tool).

## Deferred Ideas

None — discussion stayed within phase scope. Phase 73 is verification/integration only.
