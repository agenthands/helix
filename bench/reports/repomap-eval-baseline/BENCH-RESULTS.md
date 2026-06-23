# RepoMap-Eval Committed Baseline (repomap_eval)

This is the **committed, byte-reproducible** RepoMap ranking-quality baseline
(BASELINE-02). It scores committed symbol-level gold (authored from each exercise's
`.meta/example.*` reference solution) against committed captured `get-repo-map` /
`get-context` rankings via recall@10 / MRR / nDCG@10 / budget-fit, gated by a
reversed-AND-seeded-random discriminator that bites the corpus by a committed margin.
It carries **deterministic metrics only** — no live latency, no timestamp, no
absolute path, no live tokens. Regenerate locally with `make bench-repomap-eval`.

## Cell

| field | value |
| --- | --- |
| schema_version | v2 |
| benchmark | repomap-eval |
| task_id | repomap-eval-corpus |
| mode | repomap_eval |
| language | multi |
| corpus_exercises | 28 |
| corpus_languages | go, python, rust |

## Deterministic Metrics

| metric | value |
| --- | --- |
| budget_fit_ratio | 1.0000 |
| mrr | 1.0000 |
| ndcg_at_10 | 1.0000 |
| recall_at_10 | 1.0000 |

## Discriminator (anti-vacuity, nDCG@10)

| field | value |
| --- | --- |
| forward_ndcg_at_10 | 1.0000 |
| margin | 0.3000 |
| reversed_ndcg_at_10 | 0.0048 |
| seeded_random_ndcg_at_10 | 0.5243 |
