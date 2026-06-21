# Phase 75: Schema, Fairness Contract & Tree Skeleton - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-14
**Phase:** 75-schema-fairness-contract-tree-skeleton
**Areas discussed:** Result schema version & path, Existing bench/ Phase 64 artifacts, Fairness contract shape & waiver mechanism, Cost-table & TOS attestation format

---

## Result schema version & path

| Question | Options | Selected |
|----------|---------|----------|
| Schema name | `result.v2.json` ✓ / `result.v1.json` / `result.json` | `result.v2.json` |
| Schema path | `bench/schema/result.v2.schema.json` ✓ / `bench/evaluators/...` / `bench/...` | `bench/schema/result.v2.schema.json` |
| Major-bump definition | Strict additive-only = minor ✓ / Looser (new required = minor) / Defer to Phase 79 | Strict additive-only is minor |
| Version field shape | Single string field ✓ / Structured `{name,major,minor}` / Both string + `$schema` URL | Single `schema_version: "v2"` string |

**User's choice:** v2 lineage from eval's implicit v1; dedicated `schema/` contract dir; strict additive-only bump policy; flat greppable version string.
**Notes:** schema_version chosen flat/greppable over structured object. Side-effect: REQUIREMENTS.md BENCH-03 says "v1" and needs a v1→v2 fix.

---

## Existing bench/ Phase 64 artifacts

| Question | Options | Selected |
|----------|---------|----------|
| Fate of Phase 64 microbench | Move to `internal/semantic/bench/` ✓ / Move to `bench-semantic/` / Leave in place | Move to `internal/semantic/bench/` |
| Move mechanics | `git mv` single commit ✓ / `git mv` + drop `semantic_` prefix / Delete REPORT, keep tests | `git mv` preserving filenames, single atomic commit |
| BENCH-01 tree assertion | 6 dirs (5 runtime + schema) ✓ / 5 runtime only / 5 runtime + cmd ref | 6 dirs: datasets, runners, languages, evaluators, reports, schema |
| eval↔bench separation mechanism | Prose only + reciprocal pointer ✓ / Prose + `vet-noeval2bench` / Prose + Makefile grep | Prose only (INFRA-03 note + reciprocal pointer) |

**User's choice:** Relocate microbench to match kernel/semantic boundary; single `git mv` commit; six-dir bench skeleton; prose-only INFRA-03 enforcement.
**Notes:** Relocation becomes Wave 0 of the phase. No vet analyzer until real leakage — matches `internal/lint/` precedent.

---

## Fairness contract shape & waiver mechanism

| Question | Options | Selected |
|----------|---------|----------|
| Contract definition/loading | Pure Go literal ✓ / Go struct + sidecar YAML / Go literal + env-var override | Pure Go literal in `bench/runners/fairness_contract.go` |
| Override WaiverReason | Struct free-text + ApprovedBy ✓ / Closed enum / No overrides allowed | `ModeOverride` free-text WaiverReason + ApprovedBy; loader fatals on empty |
| FAIR-02 deprecation gate | Static calendar `deprecation_at` ✓ / Live provider-API check / Separate deprecations.yaml | Static `deprecation_at` field; fires < 30 days |
| FAIR-03 variance warning | Per-task max-min/mean on tokens_input ✓ / Per-mode aggregate / Both | Per-task `(max−min)/mean > 0.05`; cached-input as separate columns |

**User's choice:** Compile-time pinned contract; free-text waivers with maintainer signoff; static deprecation calendar; per-task input-token variance gate.
**Notes:** Free-text chosen so emergent reasons (e.g. baseline_rag max_tokens) need no enum edit. Reuses STATS-01 N≥3.

---

## Cost-table & TOS attestation format

| Question | Options | Selected |
|----------|---------|----------|
| cost-table.yaml columns | Full pricing + staleness set ✓ / Core pricing only / Minimal + deprecation | provider, model_id, input/output/cached_input per_mtok, currency, valid_until, last_verified, deprecation_at |
| PROVIDERS.md format | YAML frontmatter + prose ✓ / Structured YAML only / Free-form grep-checked | YAML frontmatter (parsed by verify-tos) + prose notes |
| eval/ cost-table sharing | bench owns; eval independent ✓ / Shared canonical table / Defer | bench owns cost-table.yaml; eval stays independent |
| Make-target failure mode | Hard-fail both ✓ / Hard-fail TOS, warn staleness / Warn-summary both | Hard-fail both (exit non-zero) |

**User's choice:** Complete pricing+staleness columns; frontmatter-parseable TOS attestation; bench-owned cost-table; hard-fail CI gates.
**Notes:** Hard-fail aligns with v1.12-ROADMAP Phase 75 success criterion #5. Keeping cost-table bench-owned upholds the INFRA-03 boundary.

---

## Claude's Discretion

- Exact field ordering / optional-vs-required marking inside `result.v2.schema.json` (within additive-only contract).
- Internal package layout of `helix-bench` CLI and the `fairness_contract.go` loader.
- `verify-tos` / `validate-cost-table` implementation (Go validator vs. yq/grep) — only hard-fail semantics and parseable formats are fixed.

## Deferred Ideas

- `vet-noeval2bench` static analyzer — until a real eval↔bench import leakage appears.
- `internal/eval/` adopting the shared cost-table.yaml — eval keeps its own pricing path for now.

## Planner notes (carried into CONTEXT.md)

1. REQUIREMENTS.md BENCH-03 (line 19) one-line fix: "v1" → "v2".
2. `bench/schema/` is a 6th top-level dir; acceptance wording + BENCH.md must reflect six dirs (schema/ contract-only).
3. Wave 0 = the `git mv` of the Phase 64 microbench, before any new file under `bench/`.
</content>
