# Spike REPORT — DSPy offline tuning harness (Phase 106)

**Spike:** exploratory-dspy-offline-tuning-harness-spike
**Requirement:** TUNE-01
**Date:** 2026-06-24
**This file IS the spike deliverable conclusion** — it records the outcome
whether or not the LM optimization run executes.

## Conclusion: NO-SHIP (default)

A **no-ship** conclusion is the spike's expected, **legitimate, success-meeting**
outcome (not a failure, not a hard adoption-delta gate). The DSPy GEPA harness,
the parity-pinned Python scorer, the held-out split, and the overfit /
metric-gaming / planted-divergence guards are all built and green; the spike's
honest finding is that the **corpus is too small to trust a tuned adoption
delta**, so no optimizer output should be adopted into the shipped surface at
this time.

### Why no-ship

- **`MinTasks=5` is the floor; the corpus is far below a trustworthy size.** The
  committed adoption bucket is 6 intact + 6 sabotaged transcripts (`test/oracle/
  adopt/testdata/transcripts/`), and this harness's own split is small
  (TRAIN=8, TEST=3). A 6+6 / 8+3 corpus cannot yield a statistically
  trustworthy adoption delta on a held-out TEST split.
- **A meaningful held-out split starves the optimizer.** Reserving even 2–3 TEST
  tasks leaves GEPA almost no signal to evolve against — so a "win" on TEST
  would be noise, not a real improvement.
- **The proxy is gameable.** `choice_rate` keys on the first emitted command; an
  optimizer can win it with degenerate unconditional-`helix` steering. The
  `test_degenerate.py` guard flags that, but the residual risk (no quality/
  correctness joiner) is real — full defense is TUNE-FUT-02 (deferred).
- **The clean fallback keeps the milestone 100% Go.** A hand-rolled Go
  candidate-search loop avoids the Python quarantine cost entirely.

### Gate before a real tuning run (TUNE-FUT-01)

Grow the corpus to `val_size > 50` and (per TUNE-FUT-02) join the adoption
metric with a task-success/quality oracle before a GEPA/MIPROv2 run can beat a
deterministic baseline with confidence.

## Split sizes (this harness)

| Split | Source | Count | Seen by optimizer? |
|-------|--------|-------|--------------------|
| TRAIN (train∪val) | `data/train.jsonl` | 8 | yes (val carved from train) |
| TEST (held-out)   | `data/test.jsonl`  | 3 | **no — sequestered** |

The held-out split was created as the FIRST harness step; `test_split.py`
asserts TEST ⊄ train∪val, and `optimize.py` never passes `test.jsonl` to
`optimizer.compile()`.

## Gates (all green, LM-free)

| Gate | File | Proof it is non-vacuous |
|------|------|-------------------------|
| Python↔Go parity | `test_parity.py::test_python_go_parity` | asserts scorer vs the SAME `golden/parity_cases.json` the Go `parity_test.go` pins |
| Planted-divergence (anti-vacuity) | `test_parity.py::test_broken_classifier_diverges` | a `"helix" in response` substring classifier DIVERGES from the real scorer on the substring-trap case |
| Overfit guard | `test_split.py` | TEST disjoint from train∪val; RED on a leak |
| Metric-gaming guard | `test_degenerate.py` | flag/no-flag pair: degenerate always-`helix` flagged, conditional steering not |

## If the dev runs `optimize.py` with an LM key

> Placeholder — fill in after a dev-time run with `OPENAI_API_KEY` set.

- Baseline (deterministic SKILL.md steering) TEST `choice_rate`: _TBD_
- GEPA-optimized TEST `choice_rate`: _TBD_
- Delta on held-out TEST: _TBD_ (adopt only if it beats baseline by a material
  margin on data the optimizer never saw — mirror `MaterialDrop=0.4` as a
  reporting threshold, not a CI gate)
- Degenerate-steering inspection of the optimized text: _TBD_ (must pass
  `test_degenerate.py`'s `is_degenerate_steering(optimized_text) is False`)

## Re-entry (if any text is ever adopted)

Adopted steering re-enters the shipped surface **only** via a human-reviewed
`internal/cli/skills/helix/SKILL.md` edit (≤1536 chars, `## Decision matrix`
anchor preserved) or a refgen override, passing `go run ./cmd/helix-refgen
--check`. The raw `output/optimized.json` is git-ignored and **never
auto-adopted**.

## Toolchain note

Research pinned `dspy==3.1.3`; the harness uses `dspy==3.2.1` (the then-current
release at plan time). The GEPA top-level API used by `optimize.py` is unchanged
across 3.1.x -> 3.2.x.
