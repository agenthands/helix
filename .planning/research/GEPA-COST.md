# Research — DSPy GEPA (`dspy==3.2.1`) call-volume scaling & cost (verified against installed source)

**Bottom line:** Total LLM cost ≈ `R·(T+1)·per-agent-call + N·per-reflection-call`, where `R = max_metric_calls` (program rollouts), `T` = avg agent tool-turns/task, `N` = trials. For `auto="light"`, a **1-predictor** program with **valset≈55** ⇒ **R ≈ 600 rollouts**, `N ≈ 10` reflection calls. Trainset size is **free** under `auto` (only valset + predictor-count drive the budget). The biggest controllable cost levers: keep valset just above the gate (≈51–55), and **cap the agent's `max_iters` (T)** — it multiplies every one of the 600 rollouts.

## The budget formula (auto="light", n=6)
`auto_budget(num_preds, num_candidates=n, valset_size=V, minibatch_size=35, full_eval_steps=5)`:
```
num_trials N = int(max(2*(num_preds*2)*log2(n), 1.5*n))
total = V                      # initial full eval
      + n*5                    # bootstrap
      + N*35                   # minibatch evals  (35 is hard-coded)
      + (periodic_fulls + extra_final) * V   # periodic + final full evals
```
| p | V | N | **R = rollouts** |
|---|---|---|---|
| 1 | 55 | 10 | **600** |
| 1 | 30 | 10 | 500 |
| 2 | 55 | 20 | 1060 |

Headline (p=1,V=55): `55 + 30 + 350(=10·35) + 165(=3·55) = 600`.

## Metric / split semantics (load-bearing for the sequestered TEST split)
- Metric is the GEPA feedback protocol: `(gold, pred, trace, pred_name, pred_trace) -> float | dspy.Prediction(score=, feedback=)`. Called **once per rollout** (metric-call count ≡ R). Our `make_gepa_metric`/`taskmetric.py` already matches this.
- **trainset** = reflective updates; **valset** = Pareto candidate scoring/selection (NOT reflected on, but **leaked into model selection**). `valset=None` falls back to trainset (overfit warning).
- **The sequestered TEST split must NEVER be passed as trainset OR valset** — anything passed as valset biases candidate selection. `optimize.py` already asserts train∩val=∅; TEST (attribution split) stays separate.

## Worked cost estimate (deepseek-v4-flash pricing)
R=600, T≈8 ⇒ ~5,400 agent LLM calls. At ~4k in / 1k out per call (cache-miss worst case):
- input: 5400·4000 = 21.6M tok × $0.14/1M ≈ **$3.0** (much less with prompt-prefix cache hits)
- output: 5400·1000 = 5.4M tok × $0.28/1M ≈ **$1.5**
- reflection: ~10 calls × long context (≈32k) ≈ **<$0.10**
**⇒ a single `auto="light"` run ≈ $5–15 worst case; cache hits push it well under.** "Cost-aware real run" is easily affordable, and re-runnable.

## How to keep the run bounded
- Prefer an explicit `max_metric_calls` integer for a hard cap (vs `auto`).
- Cap agent `max_iters` (T) — highest leverage after V.
- Point `reflection_lm` at a cheaper model (few calls, long context).
- Grow **trainset** freely (no `auto`-budget cost) for better reflection; keep **valset** ≈51–55.
- Enable prompt-prefix caching (stable system prompt) for the ~50× cache-hit input discount.

## Unknowns
- Exact reflection-call count is ≈N by design (merge calls `use_merge=True`/`max_merge_invocations=5` may add a few) — confirm with `track_stats=True`/`log_dir` on one real run.
- `reflection_minibatch_size=3` (examples shown per reflection) ≠ the budget's hard-coded `minibatch_size=35` — do not conflate.
- Actual tokens/call are agent-specific — measure from a dry run to turn formulas into dollars. Pin `pip show gepa` alongside `dspy==3.2.1` (inner search loop is the separate `gepa` package).

## Source
- Installed `dspy==3.2.1` source: `dspy/teleprompt/gepa/gepa.py` (`AUTO_RUN_SETTINGS`, `auto_budget()`, `GEPAFeedbackMetric`, compile docstring). Corroborated by DSPy GEPA overview docs (dspy.ai) + GEPA paper.
