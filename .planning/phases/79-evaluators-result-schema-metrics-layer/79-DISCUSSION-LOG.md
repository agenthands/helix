# Phase 79: Evaluators & Result-Schema Metrics Layer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-18
**Phase:** 79-evaluators-result-schema-metrics-layer
**Areas discussed:** Scripted-agent token metrics, edit_locality denominator, regression_rate test strategy, Schema formalization + failure semantics

---

## Scripted-agent token metrics (METRIC-03)

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit null | tokens_* = null when no provider usage block exists; source-rule + regression test apply to real-LLM agents | ✓ |
| Zero tokens | report 0 for scripted runs | |
| Synthetic estimate | estimate from MCP I/O byte sizes | |

**User's choice:** Explicit null (Recommended)
**Notes:** The Phase 78 Go corpus runs the no-LLM scripted agent → no provider `usage`. Null keeps the corpus honest; the usage-source-of-truth assertion targets the `your_agent_full` real-LLM path. → D-01/D-02/D-03.

---

## edit_locality denominator (METRIC-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Git-tracked files in subtree | `git ls-files` under the task subtree; excludes generated/vendored/untracked; deterministic | ✓ |
| All files on disk in subtree | includes generated/vendored; inflates denominator; non-deterministic | |
| Source-only by extension | language-aware extension filter | |

**User's choice:** Git-tracked files in subtree (Recommended)
**Notes:** Deterministic + reproducible; reflects real source surface. Edge cases (root-only=1.0, all-files≈0.0) to unit-test. → D-04.

---

## regression_rate test strategy (METRIC-05)

| Option | Description | Selected |
|--------|-------------|----------|
| Full pre-existing suite, pre+post | run full set pre-patch (cache passing set) + post-patch; accurate | ✓ |
| Targeted/impacted subset | only tests touching changed files; cheaper, approximate | |
| Reuse fixture's own go test only | cheapest; conflates task_success with regression | |

**User's choice:** Full pre-existing suite, pre+post (Recommended)
**Notes:** Go ToolBench fixtures are tiny single-module repos → double-run cost negligible. Cost tradeoff to revisit at SWE-bench scale (Phase 87). → D-05.

---

## Schema formalization + failure semantics (METRIC-01)

| Option (schema) | Description | Selected |
|--------|-------------|----------|
| Formalize all 17 as typed nullable fields | add 13 missing metrics to schema; machine-enforce "explicit nulls not omissions"; minor bump | ✓ |
| Keep schema open/additive | evaluators write metrics but schema stays permissive | |

| Option (failure) | Description | Selected |
|--------|-------------|----------|
| Per-metric null + error annotation | failed grader's metric = null w/ error reason; other metrics populate; row still emitted | ✓ |
| Whole result fails/dropped | any grader failure drops the row | |
| Hybrid (core vs auxiliary) | core-grader failure drops row, auxiliary nulls the metric | |

**User's choice:** Formalize all 17 typed nullable fields + Per-metric null + error annotation (both Recommended)
**Notes:** The two pair: typed-nullable schema makes the per-metric-null failure mode machine-validatable. → D-06/D-07.

---

## Claude's Discretion

- Trace-merge reuse vs rebuild (strong default: reuse `internal/eval/trace.Merge` + taps per METRIC-06).
- Evaluator package boundaries, where each writes its output, and the error-annotation shape on null metrics.

## Deferred Ideas

- Targeted/cached regression strategy for large external repos → Phase 87.
- Multi-run aggregation / BCa / pass@k / cost rollup → Phase 82.
