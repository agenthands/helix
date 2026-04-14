# Phase 18: Harness Extraction & Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 18-harness-extraction-foundation
**Areas discussed:** Package extraction strategy, Build tag taxonomy, Oracle package layout, Backward compatibility

---

## Package Extraction Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Copy and evolve | Copy harness.go, helpers.go, golden.go into test/harness/ as new package. Existing test/integration/ keeps its own copies untouched. New package evolves independently. | ✓ |
| Shared package, both import | Move shared code to test/harness/, refactor test/integration/ to import from it. Single source of truth but changes test/integration/. | |
| Thin wrapper in test/harness/ | test/harness/ re-exports from test/integration/ via wrapper functions. Minimal duplication but creates coupling. | |

**User's choice:** Copy and evolve
**Notes:** User emphasized this is a "new testing product" with different responsibilities (upstream-parity oracles, MCP transcript capture, schema/golden validation, multi-runtime fixture orchestration). Rule: "extract by duplication first, converge later only after the new harness stabilizes." Rename aggressively to new concepts (Runner, ParityOracle, Transcript, GoldenStore). Add tests for harness itself. Do not import back from test/integration/. Revisit convergence only after 2-3 iterations.

---

## Build Tag Taxonomy

| Option | Description | Selected |
|--------|-------------|----------|
| Additive layers | integration = deterministic oracles. llm = adds LLM behavioral. llmjudge = adds judge scoring. Each layer includes ones below via boolean expressions. | ✓ |
| Independent tags | Each tag independent, combine manually with -tags 'integration llm'. | |
| You decide | Claude picks. | |

**User's choice:** Additive layers via explicit boolean build constraints
**Notes:** User specified exact implementation: Go tags don't have native inheritance, so encode hierarchy in `//go:build` expressions. Deterministic: `integration || llm || llmjudge`. LLM: `llm || llmjudge`. Judge: `llmjudge`. CI result: `-tags=integration` = deterministic only, `-tags=llm` = deterministic + LLM, `-tags=llmjudge` = all three.

### Legacy Tag Sub-question

| Option | Description | Selected |
|--------|-------------|----------|
| Keep as-is | test/integration/ keeps //go:build integration unchanged. Both suites run with -tags integration. | ✓ |
| Split: legacy + oracle | Rename existing to integration_v1, new uses integration. | |
| You decide | Claude picks. | |

**User's choice:** Keep as-is
**Notes:** Safer migration path. Preserves backward compat for everyone running `go test -tags=integration ./...`. New oracle files use boolean expressions alongside unchanged legacy tag.

---

## Oracle Package Layout

| Option | Description | Selected |
|--------|-------------|----------|
| test/oracle/{layer}/ | test/harness/ for shared infra, test/oracle/protocol/, contract/, scenario/, llm/, judge/ — each its own Go package importing test/harness/ | ✓ |
| test/oracle/ flat | All oracle tests in one package with file-level organization | |
| You decide | Claude picks. | |

**User's choice:** test/oracle/{layer}/
**Notes:** Each layer as its own Go package. Clean separation matches the 5-layer oracle architecture.

---

## Backward Compatibility

| Option | Description | Selected |
|--------|-------------|----------|
| Zero changes | Not a single line in test/integration/ modified. New harness builds its own version of anything it needs. | ✓ |
| Minimal touchups allowed | Small changes OK to avoid significant duplication. No structural refactoring. | |
| You decide | Claude balances pragmatism with prior decisions. | |

**User's choice:** Zero changes
**Notes:** User stated test/integration/ is "frozen as a baseline suite." Duplication is a feature, not a bug — buys isolation while oracle system stabilizes. Only relax after: (1) duplicated helper is proven stable across several iterations, AND (2) obvious low-risk value in converging.

---

## Claude's Discretion

- Internal package structure of test/harness/ (file split, type names beyond suggested concepts)
- Whether to use testify or stdlib testing in the new harness
- Golden file directory layout under testdata/

## Deferred Ideas

None — discussion stayed within phase scope
