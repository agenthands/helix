# Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines - Context

**Gathered:** 2026-06-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Two stdlib-only leaf evaluators that **measure existing Helix subsystems** against corpora **authored from task ground truth (not tool output)**, each guarded by an anti-vacuity discriminator, with committed byte-reproducible baselines:

- `bench/evaluators/repomapeval` — measures `internal/repomap` ranking quality (`get-repo-map` / `get-context`) via recall@k / MRR / nDCG + token-budget fit against a multi-language **gold** corpus.
- `bench/evaluators/fuzzyrobust` — measures `internal/fuzzy` 4-strategy selection and ambiguity refusal (reusing `editsim.ES`) against a multi-language **drift** corpus.

**Requirements:** REPOEVAL-01, REPOEVAL-02, FUZZBENCH-01, FUZZBENCH-02, BASELINE-02

This phase clarifies HOW to author the corpora and define "quality" — not new capabilities. The metric *family*, the reversed/random-ranker discriminator, the ≥1 must-be-refused ambiguous case, and the leaf/determinism contracts are already fixed by the ROADMAP success criteria and Phase 100 precedent (see Canonical References).

</domain>

<decisions>
## Implementation Decisions

### Gold-relevance granularity (RepoMap corpus)
- **D-01: Symbol-level gold.** A "relevant" hit (the unit recall@k / MRR / nDCG score against) is the specific **functions/types/methods the solution defines or edits** (derived from each exercise's `files.solution`), NOT file-level. Rationale: vendored exercism exercises are tiny (2–3 files), so file-level recall@k saturates near 1.0 for any non-broken ranker and is a weak discriminator; symbol-level matches what RepoMap actually ranks.
- **D-02: Committed gold data, independent of tool output.** The gold symbol set is authored once (per task: `file:symbol` IDs), reviewed, and **frozen as corpus data** (committed). The evaluator reads committed gold + the ranked output and scores. The stdlib-only leaf MUST NOT import `internal/repomap`'s extractor (`vet-ablation-leakage`), and gold must not be derived from `get-repo-map` output — committed data satisfies both.

### Corpus breadth & size floor (both corpora)
- **D-03: Languages = py / go / rust now.** Cover exactly the set Phase 99 vendored. Java stays **reserved** (adding it means vendoring new fixtures + license headers — out of scope; see Deferred).
- **D-04: Moderate, documented per-language size floor.**
  - RepoMap gold corpus: **~10 tasks/language (~30 total)**.
  - Fuzzy drift corpus: **~4 cases per strategy per language** (4 strategies × 4 × 3 langs ≈ 48) **plus** the known-ambiguous must-refuse case(s).
  - The floor is documented as a known minimum in the corpus manifest; enough for recall@k/MRR/nDCG and strategy-selection to be statistically meaningful and for the discriminator margin to bite, without heavy authoring cost.

### Fuzzy drift corpus authoring (fuzzyrobust)
- **D-05: Programmatic perturbation, expected strategy derived from drift type.** Each case takes a real vendored fixture code block and applies a **deterministic per-tier transform** — trailing/leading whitespace → `whitespace_normalized`; reindent → `indentation_flexible`; elide middle → ellipsis-placeholder; no drift → `exact`. The transform IS the drift-type derivation; the expected strategy is structural and never read from observed tool behavior. Deterministic and byte-reproducible.
- **D-06: Known-ambiguous case via duplicate-block from fixtures.** Construct/select a fixture containing two drift-equivalent code blocks plus a search pattern that matches both sites → `fuzzy.Match` MUST refuse (ambiguity refusal). The refusal follows structurally from the duplicate, not from observed behavior. At least one such case ships and is asserted to be refused (anti-vacuity).

### Headline metric & discriminator margin (repomapeval)
- **D-07: nDCG@10 is the headline metric** of the committed RepoMap baseline; **recall@10 and MRR are computed and reported alongside**. nDCG is position-weighted so it is most sensitive to ordering, making the reversed-ranker discriminator bite hardest. k = 10 fits symbol-level gold on small exercises.
- **D-08: Token-budget fit = reported ratio.** Recorded as a continuous deterministic metric: fraction of gold symbols retained in the rendered map within the token budget (gold-retention-within-budget). A pass/fail is derivable later if wanted; the committed baseline carries the ratio.
- **D-09: Discriminator = both rankers + documented absolute margin.** Ship BOTH a **reversed** ranker (adversarial worst case) and a **seeded random** ranker (chance floor). The discriminator passes only if the forward ranker beats both on nDCG@10 by a **documented absolute margin** (e.g. ≥ 0.2). The margin is committed so a near-tie on a small corpus cannot sneak through. This is the REPOEVAL anti-vacuity "MUST fail the corpus" proof; the fuzzy side's anti-vacuity proof is the must-refuse ambiguous case (D-06).

### Inherited (locked) — carried forward, not re-decided
- **Determinism contract (Phase 100):** committed baselines are driven by a **deterministic/scripted path (not a live LLM)**, captured `HELIX_BIN`-gated **fail-not-skip** (fail when `HELIX_BIN` is set but no result / empty bucket / missing metric is produced), with a **hermetic golden sibling** (no binary, no network) as the sole authoritative `go test ./bench/...` proof, routed through the existing deterministic `renderAll` / seeded RNG / sort-before-emit so artifacts regenerate byte-identically. Commit deterministic metrics only (exclude live-model latency/tokens).
- **Leaf boundary:** both evaluators are stdlib-only leaves respecting `vet-ablation-leakage` (no kernel import). `fuzzyrobust` may additionally use `bench/evaluators/editsim` (`editsim.ES`). RepoMap ranked output is consumed as data, not by importing the kernel.

### Claude's Discretion
- Corpus on-disk storage format (JSON/txt layout), exact exercise selection from the Phase 99 `VENDOR-MANIFEST.md`, report layout, the precise per-tier perturbation parameters, and the exact committed discriminator margin value (≥0.2 is a guide, not a hard mandate) — all at Claude's discretion within the constraints above. Resolve during plan/research.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase spec & requirements
- `.planning/ROADMAP.md` — Phase 102 Details (Goal + 4 Success Criteria; the metric family, discriminator, must-refuse case, and leaf/determinism contracts are fixed here).
- `.planning/REQUIREMENTS.md` — REPOEVAL-01, REPOEVAL-02, FUZZBENCH-01, FUZZBENCH-02, BASELINE-02.

### Determinism / baseline precedent (reuse the pattern)
- `.planning/phases/100-polyglot-edit-benchmark-committed-baseline/100-CONTEXT.md` — committed-baseline determinism contract: deterministic driver, `HELIX_BIN` fail-not-skip, hermetic golden sibling, `renderAll`/seed/sort-before-emit.
- `bench/aggregator/` — deterministic `renderAll` / seeded resamples (route reports through this).

### Subsystems under evaluation (measured, NOT imported by the leaves)
- `internal/repomap/` — PageRank ranking, `elide.go` token-budget fitting, `extractor.go`, `render.go` (the ranking quality + token-budget-fit subject).
- `internal/fuzzy/` — 4-strategy cascade (`strategies.go`, `types.go`: `exact` / `whitespace_normalized` / `indentation_flexible` / ellipsis) + ambiguity refusal (the strategy-selection/refusal subject).

### Reuse
- `bench/evaluators/editsim/editsim.go` — `editsim.ES` (CM-ES normalized edit similarity); reuse in `fuzzyrobust`, do not re-implement.
- `bench/datasets/aider-polyglot/fixtures/` + `bench/datasets/aider-polyglot/VENDOR-MANIFEST.md` — Phase 99 vendored py/go/rust fixtures (the ground-truth source for both corpora); offline.
- `bench/evaluators/metrics.go` + `bench/evaluators/METRICS.md` — existing metrics conventions to mirror.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `bench/evaluators/editsim.ES` — stdlib-only normalized edit-similarity scorer; the FUZZBENCH similarity primitive.
- `bench/datasets/aider-polyglot/fixtures/` (py/go/rust) + `VENDOR-MANIFEST.md` — committed, offline ground-truth source for authoring both corpora.
- `bench/aggregator` deterministic `renderAll` / seeded RNG — the determinism + byte-reproducibility plumbing the committed baselines route through.
- Existing leaf evaluators under `bench/evaluators/*` (exactmatch, token_meter, editsim) — the stdlib-only leaf package shape + test/golden conventions to mirror.

### Established Patterns
- **Leaf-import boundary (`vet-ablation-leakage`):** evaluator leaves are stdlib-only; consume tool/ranker output as data, never import the kernel (`internal/repomap`, `internal/fuzzy`).
- **Filesystem-as-table + additive-open-key + hermetic-golden-sibling** (Phases 80/100) — bench surfaces add modes/keys without resolver changes; the hermetic golden is the authoritative `go test ./bench/...` proof.
- **Anti-vacuity discriminators** (v1.12 lesson, reinforced Phases 97/99/100/101) — a deliberate break-the-invariant test must turn RED: here the reversed/random ranker must fail the gold corpus (D-09) and the ambiguous case must be refused (D-06).

### Integration Points
- New packages: `bench/evaluators/repomapeval/`, `bench/evaluators/fuzzyrobust/`, plus committed corpus + baseline artifacts (e.g. under `bench/reports/...`, `.gitignore` allowlisted per Phase 100 precedent).
- The deterministic driver that produces ranked output / strategy selection for the committed baseline dials the warm daemon `HELIX_BIN`-gated (live leg) while the hermetic golden runs binary-free.

</code_context>

<specifics>
## Specific Ideas

- Reversed AND seeded-random rankers both ship as discriminators (not one); forward must beat both on nDCG@10 by a committed margin.
- Symbol-level gold is `file:symbol`-keyed committed data; nDCG@10 with recall@10 + MRR reported; token-budget fit as a gold-retention ratio.
- Fuzzy drift derived by deterministic per-tier perturbation of real fixture blocks; ambiguity via duplicate-block-from-fixtures, must be refused.

</specifics>

<deferred>
## Deferred Ideas

- **Java-language corpus coverage** — requires vendoring java fixtures + SPDX/license headers (Phase 99 deliberately reserved java). Belongs in its own follow-up phase/vendoring step, not Phase 102.

</deferred>

---

*Phase: 102-RepoMap-Quality + Fuzzy-Robustness Evals + Baselines*
*Context gathered: 2026-06-23*
