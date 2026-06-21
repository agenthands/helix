# Phase 75: Schema, Fairness Contract & Tree Skeleton - Context

**Gathered:** 2026-06-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Foundation phase of milestone v1.12 (Bench Stack & Tool Evaluation). It establishes the
contracts every downstream phase (76–89) writes into: the versioned `result.v2.json` result
schema, the compile-time `fairness_contract.go` model-config pin, the `bench/` directory
skeleton, the `cost-table.yaml` / `PROVIDERS.md` cost-and-TOS contract, and the `helix-bench`
CLI skeleton — so no benchmark adapter ever defines its own model snapshot, temperature, or
cost row.

**Requirements locked here:** BENCH-01, BENCH-02, BENCH-03, BENCH-06, FAIR-01, FAIR-02,
FAIR-03, COST-01, INFRA-01, INFRA-02, INFRA-03 (see `.planning/REQUIREMENTS.md`).

This phase delivers contracts and skeletons, not benchmark adapters. WHAT/WHY are locked
upstream in REQUIREMENTS.md and v1.12-ROADMAP.md; the decisions below are the HOW.
</domain>

<decisions>
## Implementation Decisions

### Result schema (BENCH-03, BENCH-06)
- **D-01:** Schema name is `result.v2.json` — continues lineage from `internal/eval/`'s
  implicit v1. (Chosen over `result.v1.json` / `result.json`.)
- **D-02:** JSON Schema lives at `bench/schema/result.v2.schema.json` — a dedicated contract
  dir, symmetrical to the runtime dirs. (Not under `bench/evaluators/` or `bench/` root.)
- **D-03:** Bump policy is **strict additive-only = minor**. Adding optional fields or new enum
  values for forward-compat consumers → v2.x (minor). Renaming, removing, narrowing types,
  changing nullability, or repurposing a field → v3 (major). 14 downstream phases assume
  metric stability across the milestone.
- **D-04:** Version surfaces as a single top-level `schema_version: "v2"` string field in each
  result file (greppable). Minor/patch tracked in the `.schema.json` `$id` or a sidecar
  CHANGELOG — not in the result payload.

### Existing bench/ Phase 64 artifacts (INFRA-01, INFRA-03)
- **D-05:** Relocate the Phase 64 semantic-microbench artifacts (`bench/_blevprobe/`,
  `bench/fixtures/`, `bench/semantic_bench_*.go`, `bench/semantic_bench_REPORT.md`) to
  `internal/semantic/bench/` — frees top-level `bench/` for the v1.12 stack and matches the
  kernel/semantic boundary already enforced by `vet-nokernel2semantic`.
- **D-06:** Move mechanically via `git mv` preserving filenames, in a **single atomic commit**
  (e.g. `refactor(bench): relocate Phase 64 semantic microbench to internal/semantic/bench/`)
  so `git log --follow` continuity is preserved. This is the phase's **Wave 0** — must land
  before any new file is written under `bench/`.
- **D-07:** BENCH-01's `tree -d -L 2 bench/` acceptance asserts **six** dirs: `datasets`,
  `runners`, `languages`, `evaluators`, `reports`, `schema`. `schema/` is the contract dir;
  the other five are runtime. `bench/BENCH.md` documents the runtime-vs-contract distinction.
- **D-08:** The eval↔bench separation (INFRA-03) is enforced by **prose only** this phase: an
  INFRA-03 note in `bench/BENCH.md` plus a reciprocal pointer paragraph in `internal/eval/`'s
  EVAL doc. **No `vet-noeval2bench` analyzer** until a real leakage appears — matches the
  `internal/lint/` precedent (`nokernel2semantic`, `nosemantic2kernel`, `noduckdb`) of shipping
  analyzers only when they have something concrete to catch.

### Fairness contract & waiver mechanism (FAIR-01, FAIR-02, FAIR-03)
- **D-09:** Fairness contract is a **pure Go literal** in `bench/runners/fairness_contract.go`
  (`var DefaultContract = FairnessContract{...}`) — compile-time pin of `model_id`,
  `temperature`, `max_tokens`, `system_prompt_hash`, `retry_policy`, `cache_policy`. Snapshot
  bumps are a code change + PR + WaiverReason discussion, reviewable in
  `git log fairness_contract.go`. No YAML/env-var back-channels.
- **D-10:** Per-mode overrides use a `ModeOverride` struct with **free-text `WaiverReason` +
  `ApprovedBy`** maintainer signoff. The loader **fatals on empty `WaiverReason`**; every
  override is logged to `result.v2.json.fairness.overrides[]`. Free-text (not closed enum)
  allows emergent reasons (e.g. `baseline_rag` needing higher `max_tokens` for chunk-stuffing)
  without a code change to add enum values.
- **D-11:** FAIR-02 deprecation gate is a **static calendar**: a `deprecation_at: YYYY-MM-DD`
  field on each `(provider, model_id)` row of `bench/datasets/cost-table.yaml`. The gate fires
  when `deprecation_at − today < 30 days`. No live provider-API call — keeps runs reproducible
  from `--run-id`. Staleness bounded by the 90-day `valid_until`/`last_verified` rotation.
- **D-12:** FAIR-03 variance gate is **per-task**: across N≥3 runs of the same `(task, mode)`,
  flag when `(max(tokens_input) − min(tokens_input)) / mean(tokens_input) > 0.05`. Cached-input
  fields (`tokens_input_cached_read`, `tokens_input_cache_write`) are reported as **separate
  columns** so cache-hit-rate variance is visible alongside. Reuses STATS-01's N≥3 — no
  separate sampling apparatus. Warning flows to `cost_quality.md`.

### Cost-table & TOS attestation (FAIR-02, COST-01)
- **D-13:** `bench/datasets/cost-table.yaml` rows carry the **full pricing + staleness set**:
  `provider, model_id, input_per_mtok, output_per_mtok, cached_input_per_mtok, currency,
  valid_until, last_verified, deprecation_at`. `cached_input_per_mtok` pairs with D-12's
  cached-input columns; `valid_until`/`last_verified` are the 90-day rotation fields.
- **D-14:** `bench/PROVIDERS.md` is markdown with a **YAML frontmatter** block per provider
  (`provider, tos_url, attested_by, attested_on, benchmarking_permitted, publish_permitted`)
  parsed by `make verify-tos`, followed by human-readable prose notes. Machine-checkable yet
  documents caveats a TOS attestation needs.
- **D-15:** `cost-table.yaml` is **owned by bench/** and serves the bench stack only.
  `internal/eval/` stays independent (keeps `internal/eval/report/cost_summary.go`). Upholds
  the INFRA-03 boundary (D-08) — no shared pricing file straddles eval↔bench.
- **D-16:** `make verify-tos` and `make validate-cost-table` both **hard-fail (exit non-zero)**:
  missing/expired (>90d) TOS attestation, malformed cost-table, past `valid_until`, or
  unparseable rows block CI. Matches v1.12-ROADMAP Phase 75 success criterion #5.

### Claude's Discretion
- Exact field ordering and optional-vs-required marking inside `result.v2.schema.json` (within
  D-03's additive-only contract).
- Internal package layout of the `helix-bench` CLI and `fairness_contract.go` loader plumbing.
- `make verify-tos` / `validate-cost-table` implementation (Go validator vs. yq/grep) — only
  the hard-fail semantics (D-16) and parseable formats (D-13, D-14) are fixed.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements (locked WHAT/WHY — do not re-derive)
- `.planning/REQUIREMENTS.md` — BENCH-01/02/03/06, FAIR-01/02/03, COST-01, INFRA-01/02/03
  acceptance criteria. **BENCH-03 (line 19) needs a one-line v1→v2 fix — see planner note.**
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase 75" (lines 43–53) — Goal, Depends-on,
  Requirements, and the 5 Success Criteria this phase is graded against.
- `.planning/PROJECT.md` (v1.12 milestone section) — milestone scoping and `result.v2` naming.

### Milestone research (architectural verdicts)
- `.planning/research/SUMMARY.md` — milestone-level architectural verdicts (15-phase shape).
- `.planning/research/ARCHITECTURE.md` — bench-stack architecture.
- `.planning/research/STACK.md` — tooling/library choices.
- `.planning/research/PITFALLS.md` — known failure modes to design around.
- `.planning/research/FEATURES.md` — feature decomposition.

### Reused-pattern source (Depends-on)
- `internal/eval/` — v1.10 Phase 67 patterns (sandbox, trace tap, score DSL) the bench stack
  builds on. Specifically `internal/eval/report/cost_summary.go` is eval's independent cost
  path (D-15 keeps it separate).
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/eval/` (sandbox, `runner/inprocess.go`, `trace/`, `score/`, `report/`) — the
  bench stack reuses these patterns; bench is a sibling of eval, not a rewrite.
- `internal/eval/report/cost_summary.go` + `cost_summary_test.go` — eval's existing cost
  aggregation; reference for the bench cost-table consumer, but kept independent (D-15).

### Established Patterns
- `internal/lint/` analyzers (`nokernel2semantic`, `nosemantic2kernel`, `noduckdb`,
  `compactusesstore`) — the precedent for D-08: ship a `vet-*` boundary analyzer only when
  there's a concrete leakage to catch. INFRA-03 is prose-only this phase.
- `internal/semantic/bench/` is the destination for the relocated Phase 64 microbench (D-05);
  mirrors the kernel/semantic package boundary.

### Integration Points
- `bench/` currently holds **only** the Phase 64 artifacts (`_blevprobe/`, `fixtures/`,
  `semantic_bench_*.go`, `semantic_bench_REPORT.md`). Wave 0 `git mv` (D-06) must clear these
  before the six-dir skeleton (D-07) lands.
- `cmd/` — the `helix-bench` CLI entrypoint lands here (BENCH-02: `run`, `fetch-datasets`,
  `doctor`, `report`, `validate-cost-table`).
- Root `Makefile` — new `verify-tos` and `validate-cost-table` targets (D-16).
</code_context>

<specifics>
## Specific Ideas

- Wave 0 commit message form: `refactor(bench): relocate Phase 64 semantic microbench to
  internal/semantic/bench/ for v1.12 INFRA-03 separation`.
- `schema_version` is deliberately a flat greppable string `"v2"`, not a structured object —
  the user explicitly preferred simple/greppable over `{name, major, minor}`.
</specifics>

<deferred>
## Deferred Ideas

- **`vet-noeval2bench` static analyzer** — deferred until a real eval→bench (or reverse)
  import leakage shows up. INFRA-03 stays prose-only this phase (D-08).
- **`internal/eval/` adopting the shared `cost-table.yaml`** — not decided now; eval keeps its
  own pricing path (D-15). Revisit only if a future phase needs one canonical price source.

None of the above expand Phase 75 scope — discussion stayed within the foundation boundary.

### Planner notes (apply during plan-phase)
1. **REQUIREMENTS.md BENCH-03 (line 19) one-line fix:** change "versioned at `v1`" → "`v2`"
   to align with v1.12-ROADMAP / PROJECT.md and D-01. Schedule as a doc edit in the plan.
2. **`bench/schema/` is a 6th top-level dir** beyond BENCH-01's literal five
   (`datasets/runners/languages/evaluators/reports/`). The `tree -d -L 2 bench/` acceptance
   wording and `bench/BENCH.md` must reflect six dirs, marking `schema/` as contract-only.
3. **Wave 0 = the `git mv`** (D-05/D-06) — must precede every other wave that writes under
   `bench/`.
</deferred>

---

*Phase: 75-schema-fairness-contract-tree-skeleton*
*Context gathered: 2026-06-14*
</content>
</invoke>
