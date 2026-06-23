# Fuzzy-Robustness Committed Baseline (fuzzy_robust)

This is the **committed, byte-reproducible** fuzzy-robustness baseline (BASELINE-02).
It scores `internal/fuzzy`'s 4-strategy cascade selection and ambiguity refusal
against a committed py/go/rust drift corpus (each case a real vendored fixture block
transformed by a deterministic per-tier perturbation), gated by a duplicate-block
must-refuse `ambiguous_match` assertion (D-06). It carries **deterministic metrics
only** — no live latency, no timestamp, no absolute path, no live tokens. Regenerate
locally with `make bench-fuzzy-robust`.

## Cell

| field | value |
| --- | --- |
| schema_version | v2 |
| benchmark | fuzzy-robust |
| task_id | fuzzy-robust-corpus |
| mode | fuzzy_robust |
| language | multi |
| corpus_cases | 51 |
| corpus_languages | go, python, rust |

## Strategy Selection (deterministic)

| strategy | count |
| --- | --- |
| ambiguous_match | 3 |
| exact | 24 |
| indentation_flexible | 0 |
| no_match | 0 |
| whitespace_normalized | 24 |

## Ambiguity Refusal (anti-vacuity, D-06)

| field | value |
| --- | --- |
| ambiguous_cases | 3 |
| ambiguous_refused | true |
