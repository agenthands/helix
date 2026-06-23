# Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-23
**Phase:** 102-RepoMap-Quality + Fuzzy-Robustness Evals + Baselines
**Areas discussed:** Gold-relevance granularity, Corpus breadth & size floor, Fuzzy drift corpus authoring, Headline metric & discriminator margin

> Note: Phases 99–101 auto-skipped discussion (workflow.skip_discuss). Phase 102 was discussed interactively at the user's explicit request.

---

## Gold-relevance granularity

### Q1 — Unit of a "relevant" hit for recall@k / MRR / nDCG

| Option | Description | Selected |
|--------|-------------|----------|
| Symbol-level | Gold = functions/types/methods the solution defines/edits; most discriminating, matches what RepoMap ranks | ✓ |
| File-level | Gold = solution file(s); simplest but saturates near 1.0 on tiny exercises (weak discriminator) | |
| Symbol-level + referenced context | Solution symbols plus stub/test-referenced symbols; richer but blurs "ground truth not tool output" | |

**User's choice:** Symbol-level
**Notes:** Vendored exercism exercises are 2–3 files, so file-level recall@k is near-trivial.

### Q2 — How gold symbols are pinned (stdlib-only leaf can't import the kernel extractor)

| Option | Description | Selected |
|--------|-------------|----------|
| Committed gold data | Author once, review, freeze as corpus data; evaluator reads committed gold + ranked output | ✓ |
| Offline authoring extractor | Stdlib-only mini-extractor parses files.solution offline → frozen gold; another thing to trust/maintain | |
| You decide | Leave to Claude's discretion | |

**User's choice:** Committed gold data
**Notes:** Cleanest "ground truth not tool output", byte-reproducible, audit-friendly.

---

## Corpus breadth & size floor

### Q3 — Language coverage for both corpora

| Option | Description | Selected |
|--------|-------------|----------|
| py/go/rust now | The Phase 99 vendored set; java reserved | ✓ |
| py/go/rust + java-ready | Author 3 now, structure for java later | |
| Add java this phase | Vendor java fixtures now (scope expansion) | |

**User's choice:** py/go/rust now
**Notes:** Adding java pulls Phase 99's reserved-java decision into 102 → deferred.

### Q4 — Documented per-language size floor

| Option | Description | Selected |
|--------|-------------|----------|
| Moderate | RepoMap ~10 tasks/lang (~30); Fuzzy ~4/strategy/lang (~48) + ambiguous | ✓ |
| Lean | RepoMap ~5/lang; Fuzzy ~2/strategy/lang; noisy, weak discriminator | |
| Substantial | RepoMap ~20/lang; Fuzzy ~6+/strategy/lang; most credible, heavy authoring | |

**User's choice:** Moderate

---

## Fuzzy drift corpus authoring

### Q5 — How drift cases are generated (expected strategy must follow from drift type)

| Option | Description | Selected |
|--------|-------------|----------|
| Programmatic perturbation | Deterministic per-tier transform of a real fixture block; transform = drift-type derivation | ✓ |
| Hand-authored cases | Manual triples; realistic but risks accidental cross-tier solvability | |
| Perturbation + hand-authored edge cases | Bulk programmatic + a few hand cases | |

**User's choice:** Programmatic perturbation

### Q6 — Known-ambiguous must-refuse case construction

| Option | Description | Selected |
|--------|-------------|----------|
| Duplicate-block from fixtures | Two drift-equivalent blocks + pattern matching both → fuzzy.Match must refuse | ✓ |
| Synthetic minimal file | Hand-crafted tiny file with two identical functions | |
| You decide | Leave to Claude's discretion | |

**User's choice:** Duplicate-block from fixtures

---

## Headline metric & discriminator margin

### Q7 — Headline metric of the committed RepoMap baseline + k

| Option | Description | Selected |
|--------|-------------|----------|
| nDCG@10 | Position-weighted; bites hardest on reversed ranker; recall@10 + MRR reported alongside | ✓ |
| recall@k | Simple presence in top-k; order-insensitive, softens discriminator | |
| MRR | First-hit reciprocal rank; under-rewards multi-symbol coverage | |

**User's choice:** nDCG@10

### Q8 — Token-budget fit representation in the deterministic baseline

| Option | Description | Selected |
|--------|-------------|----------|
| Reported ratio | gold-retention-within-budget fraction; continuous, pass/fail derivable | ✓ |
| Hard pass/fail per task | ≤ budget AND retains all gold → pass; brittle, throws away near-misses | |
| You decide | Leave to Claude's discretion | |

**User's choice:** Reported ratio

### Q9 — Discriminator strictness + which adversarial rankers ship

| Option | Description | Selected |
|--------|-------------|----------|
| Both rankers + documented margin | Reversed + seeded-random; forward must beat both on nDCG@10 by a committed absolute margin (e.g. ≥0.2) | ✓ |
| Strict inequality only | Any margin; a lucky near-tie could pass on a small corpus | |
| Reversed-only + margin | No random floor; loses better-than-chance signal | |

**User's choice:** Both rankers + documented margin

---

## Claude's Discretion

- Corpus on-disk storage format, exact exercise selection from the Phase 99 VENDOR-MANIFEST, report layout, precise per-tier perturbation parameters, and the exact committed discriminator margin value (≥0.2 as a guide).

## Deferred Ideas

- Java-language corpus coverage (requires vendoring java fixtures + license headers — own follow-up phase).
