# EVAL-REVIEW — Phase 67: evaluation-harness

**Audit Date:** 2026-05-10
**AI-SPEC Present:** No (audited against EVAL-01..EVAL-07 in `.planning/REQUIREMENTS.md`, the phase CONTEXT decisions D-01..D-08, and the AI-evals reference framework `~/.claude/get-shit-done/references/ai-evals.md`)
**Overall Score:** 87/100
**Verdict:** PRODUCTION READY (with one warning on LLM-judge calibration)

> Note: Phase 67 *is* the evaluation harness for Helix's coding-agent surface. The audit therefore inverts the usual question: rather than "does the system have evals?", we ask "does the eval harness itself implement the evaluation strategy it promised — dimensions, dataset, tooling, guardrail wiring, CI integration, calibration?" That mapping is preserved below.

---

## Dimension Coverage

The "eval dimensions" are the seven product-level evaluation concerns Phase 67 promised to measure on agent runs. Each is mapped to an EVAL-0x requirement and to the relevant `ai-evals.md` dimension family.

| # | Dimension (what the harness measures) | Status | Measurement | Finding |
|---|---|---|---|---|
| D1 | **Task completion / patch correctness** (EVAL-01: success, patch applies, tests pass, diagnostics clean) | COVERED | Code (verify.sh exit code per task) | `internal/eval/report/eval_result.go` records success+patch+tests+diagnostics+duration+tokens+edits per task; per-task `verify.sh` is the deterministic oracle. Standard SWE-bench-shaped contract. |
| D2 | **Cost / token budget enforcement** (EVAL-01 tokens, D-08 four-axis caps) | COVERED | Code (live watchdog + post-hoc aggregation) | `internal/eval/budget/budget.go` enforces `max_input_tokens / max_output_tokens / max_seconds / max_tool_calls` with `failed-with-cause: budget_<axis>` outcome class; `internal/eval/report/cost_summary.go` aggregates. Per-task `budget.yaml` overrides confirmed in corpus + fixtures. |
| D3 | **Tool-use correctness — heuristic** (EVAL-05: rename/delete/public-API +1/-1 patterns) | COVERED | Code (DSL rules over merged trace) | `internal/eval/score/rules.go` + `score.go` implement the YAML DSL; starter rules cover all 5 task families per `starter_rules_test.go`. Output: `tool_behavior.json`. CI-actionable. |
| D4 | **Tool-use correctness — LLM judge** (EVAL-07: informational, never gates) | COVERED (with calibration warning — see Warnings) | LLM judge (Sonnet 4.6 default) | `internal/eval/judge/judge.go` returns `Output` only — no error path — structurally preventing judge failures from affecting `helix-eval` exit code. Output written to `tool_behavior_judge.json` with `__readme: "INFORMATIONAL — DO NOT USE FOR CI GATING"`. CI grep gate at `.github/workflows/go-test.yml:106-116` enforces non-reference in any other workflow. EVAL-07 honored at the structural level. |
| D5 | **Safety / guardrail compliance** (EVAL-01 guardrail compliance, ties to Phase 66 receipts) | COVERED | Code (aggregation of Phase 66 telemetry classes) | `internal/eval/report/safety_compliance.go` aggregates `guardrail_warned` / `guardrail_blocked` outcome counts and `ReceiptsIssued` per mode from the daemon trace tap. Reports against existing `TelemetryMiddleware` taxonomy — no new outcome classes invented. |
| D6 | **Trace fidelity / context faithfulness** (D-02: daemon telemetry merged with CC json) | COVERED | Code (dual-stream merge with schema test) | `internal/eval/trace/{tap,merge,schema}.go` produces `trace_daemon.jsonl` + `trace_cc.json` + merged `trace.json`. Daemon telemetry is source-of-truth for tool calls + tokens + guardrail outcomes; CC json supplies reasoning. Wall-clock alignment used per CONTEXT open-question resolution. |
| D7 | **Mode isolation / control validity** (EVAL-02: per-mode subprocess + baseline strips Helix tools) | COVERED | Code (per-mode HOME/socket/profile) | `internal/eval/sandbox/sandbox.go:86,96,206-227` enforces per-(task,mode) HOME, socket, config dir. `internal/profile/profiles/baseline.yaml` is `skills:[] tools:[]`; `TestBaselineExposesZeroHelixTools` is the regression oracle (A1). Without this isolation the four-mode comparison is meaningless; with it, the comparison is a real controlled experiment. |
| D8 | **Reference dataset adequacy** (D-04 corpus, D-05 size: ~10 quick / 50-100 full) | PARTIAL | Code (count + composition assertions) | `eval/corpus/` has 10 hand-authored tasks across Go/TS/Python covering rename/delete/public-API/large-edit/security families — **at the floor of D-05's "50-100 full" target, not within it**. `eval/fixtures/` has 9 quick fixtures (asserted `>=9`, within "~10" tolerance). Generators (`eval/gen/`) exist as scaffolding per D-04 but the full corpus has not been grown to 50-100. The harness is correct; the dataset is undersized for statistical power on the full run. |
| D9 | **CI integration / regression gating** (D-05: quick on every PR; full nightly/pre-release) | COVERED | Code (workflow step + grep-gate) | `.github/workflows/go-test.yml` runs `make eval-quick` on PRs (3.4s wall-time, well under 30s cap). `make eval` is explicitly excluded with comment `# This is the ONLY eval target allowed in PR-gating CI (project rule: benchmarks local-only)` — aligns with project rule "Benchmarks are local-only — never on CI". Nightly/pre-release wiring for the full run is **not** present in any workflow file; documented as local-only by intent. |
| D10 | **Provider TOS / data-retention attestation** (EVAL-06: synthetic-only, retention-zero) | COVERED | Doc (manual attestation) | `eval/EVAL.md` present with TOS attestation block; ZDR gate warns on OSS secondary corpus. Per EVAL-06 requirement this is documentation, not code; the documentation exists and matches the requirement shape. Re-attestation cadence is not automated — see Warnings. |

**Coverage Score:** 9 COVERED + 1 PARTIAL out of 10 → 9/10 (90%) hard COVERED; weighted 95% if PARTIAL counted as 0.5.

---

## Infrastructure Audit

| Component | Status | Finding |
|---|---|---|
| **Eval tooling** (hand-rolled `cmd/helix-eval`, scripted-agent for in-process, `claude` CLI subprocess for full) | ok | `cmd/helix-eval/main.go` builds; `run` and `validate-rules` subcommands present. Hand-rolled Anthropic Messages API client in `internal/eval/judge/client.go` (no SDK dep — deliberate per Phase 67-07 decision to avoid Pitfall 9). Tooling is invoked, not just imported. |
| **Reference dataset** | partial | 10 tasks (corpus) + 9 fixtures shipped vs. D-05's 50-100 target for full corpus. Composition (Go/TS/Python × rename/delete/public-API/large-edit/security) is correct; size is at the floor. Generators exist but have not produced expansion. |
| **CI/CD integration** | ok | `make eval-quick` wired in `.github/workflows/go-test.yml`; `make eval` explicitly excluded; judge grep-gate enforces EVAL-07 across all workflows. Self-exclusion in the grep gate confirmed (`--exclude=go-test.yml`). |
| **Online guardrails** | ok | Phase 67 does not introduce new online guardrails — it *reports against* Phase 66's receipt store and `guardrail_warned/_blocked` telemetry classes. The reporting path (`safety_compliance.go`) reads them correctly. The guardrails themselves live in Phase 66 (out of scope for this audit); the eval-side aggregation is implemented. |
| **Tracing** (daemon TelemetryMiddleware → trace tap → merge) | ok | `internal/eval/trace/tap.go` taps daemon telemetry; `merge.go` reconciles with `claude --output-format=json`; schema test validates structure. Trace wraps actual agent calls (the harness *is* the agent test bed). |

**Infrastructure Score:** 4 ok + 1 partial → (1+0.5+1+1+1)/5 × 100 = **90/100** for infra (with dataset undersize as the lone partial); strict count-only would be 80/100. Using midpoint: **80/100**.

---

## Score Calculation

```
coverage_score  = 9.5 / 10  × 100 = 95
infra_score     = 4.5 / 5   × 100 = 90
overall_score   = (95 × 0.6) + (90 × 0.4) = 57 + 36 = 93   (lenient)
overall_score   = (90 × 0.6) + (80 × 0.4) = 54 + 32 = 86   (strict, dataset undersize fully penalized)
```

Reported overall: **87/100** (strict reading, ceiling toward lenient where the test suite itself codifies the deviation as accepted with `assert >= 9`).

Verdict band: 80-100 = **PRODUCTION READY**.

---

## Critical Gaps

None at BLOCKER severity. All seven EVAL-0x acceptance criteria are satisfied by codebase evidence; the verifier's 10/10 truth check (67-VERIFICATION.md) reproduces. No eval dimension is MISSING.

---

## Warnings (PARTIAL findings)

### W1 — Reference dataset undersized for full-run statistical power (D8 / EVAL-05)

**Planned:** D-05 calls for "~10 in-process tasks" for `eval-quick` and **"50-100 subprocess tasks"** for `make eval`, sized so the full run gives "statistical power without becoming a multi-hour run."
**Found:** `eval/corpus/` ships 10 tasks. That is at the *floor* of the quick target, not the full target. `make eval` exists and runs the corpus, but a 10-task corpus across 4 modes (40 task-runs) is below the threshold where mode-comparison deltas have meaningful statistical confidence — exactly the hazard `ai-evals.md` flags under "Building comprehensive coverage on day one… start small (10-20 examples), expand from real failure modes."
**Why this is a WARNING not BLOCKER:** D-04 explicitly defers full-corpus expansion: "Hand-author seed corpus in Phase 67; generators ship as scaffolding. Don't generate the entire corpus from generators in this phase." The phase did exactly what it scoped. But the harness will not deliver the *signal* EVAL-05 promises until the corpus grows.
**Remediation:** Run the `eval/gen/` generators to produce 40-90 additional tasks toward the 50-100 target. Specifically: rename/delete/public-API combinatorial expansion across Go/TS/Python is the lowest-risk growth direction; existing `expected_tools.yaml` rule shapes already cover those families.

### W2 — LLM judge calibration vs. heuristic scorer not measured (D4 / EVAL-07)

**Planned:** EVAL-07 requires the judge to be informational and not gate CI. `ai-evals.md` further requires LLM judges to be "calibrated against human judgment before trusting" with target ≥0.7 correlation.
**Found:** Structural prevention is excellent — `judge.Run` returns `Output` only with no error path; CI grep-gate enforces non-reference; `__readme` boilerplate `INFORMATIONAL — DO NOT USE FOR CI GATING` ships with every judge artifact. **However**, no test in `internal/eval/judge/` or anywhere else compares judge scores to (a) the heuristic scorer's `tool_behavior.json` or (b) human-labeled outcomes. The judge's *agreement* with ground truth is unmeasured. EVAL-07 is satisfied; calibration hygiene is not.
**Why this is a WARNING not BLOCKER:** EVAL-07 is met as written. Calibration is an `ai-evals.md` best practice not in EVAL-07's text.
**Remediation:** Add a calibration harness (e.g., `internal/eval/judge/calibration_test.go`) that runs the judge and heuristic scorer over a held-out labeled subset of `eval/corpus/` (10 tasks is enough for an initial pass), computes Cohen's κ or Pearson r between judge verdict and heuristic verdict, and emits to a non-gating report file. Target ≥0.7 correlation; below that, prompt iteration is the next step. This converts the judge from "claims to be useful" to "demonstrably useful."

### W3 — Provider TOS attestation cadence is manual (D10 / EVAL-06)

**Planned:** EVAL-06 requires retention-zero attestation in `EVAL.md`.
**Found:** `eval/EVAL.md` exists with TOS attestation. There is no automated check that the attestation has been refreshed against current provider TOS, no expiry date in the file, no CI step that warns if the attestation is older than N months.
**Why this is a WARNING not BLOCKER:** EVAL-06 calls for the attestation to exist; it does. Cadence enforcement is not in the requirement.
**Remediation:** Add a date-stamp + 6-month staleness check. A trivial Make target (`make eval-attestation-check`) and a CI step that warns (not fails) when `EVAL.md`'s attestation date is >180 days old would close the loop without adding deployment friction.

---

## Remediation Plan

### Must fix before production:

None. All seven EVAL-0x requirements are satisfied; no dimension is MISSING.

### Should fix soon:

1. **Grow the full-run corpus from 10 → 50-100 tasks** (W1). Use the existing `eval/gen/` generator scaffolding. Priority: combinatorial rename/delete across Go/TS/Python first, then public-API and security families. Track corpus size as a CI-visible metric.
2. **Calibrate the LLM judge against the heuristic scorer** (W2). Add `internal/eval/judge/calibration_test.go` (or a `helix-eval calibrate` subcommand) producing a non-gating κ/r report. Target ≥0.7 before drawing conclusions from `tool_behavior_judge.json`.

### Nice to have:

3. **TOS attestation staleness check** (W3) — date-stamped `EVAL.md` with a CI warning step.
4. **Nightly/pre-release wiring for `make eval`** — currently the full run is local-only by intent (matches "benchmarks local-only" project rule). If the project later wants an opt-in nightly, a separate self-hosted-runner workflow is the path; do not move it onto GitHub-hosted runners.
5. **Smart sampling for production flywheel** — `ai-evals.md` recommends weighting toward concerning-signal interactions. Phase 67 ships the harness; the production flywheel is downstream of this phase.

---

## Files Found

**Eval engine (`internal/eval/`):**
- `pipeline.go`
- `agent/claude.go` — `claude --bare --strict-mcp-config` subprocess wrapper
- `budget/budget.go` — four-axis budget watchdog
- `judge/{client,judge}.go`, `judge/prompts/{rubric.md,tool_behavior.tmpl,few_shot.md}` — informational LLM judge
- `report/{eval_result,eval_report,cost_summary,safety_compliance,run_metadata}.go` — five reporters
- `runner/{runner,inprocess,scripted_agent}.go` — full + in-process modes
- `sandbox/sandbox.go` — per-(task,mode) HOME/socket/config isolation
- `score/{rules,score}.go` — DSL parser + heuristic scorer
- `trace/{tap,merge,schema}.go` — daemon telemetry + CC json merge

**Eval data + docs (`eval/`):**
- `corpus/` — 10 tasks (Go × 5, TS × 3, Py × 2), each with `task.md / repo / verify.sh / expected_tools.yaml / budget.yaml`
- `fixtures/` — 9 quick fixtures (each adds `scripted_agent.yaml`)
- `gen/` — generator scaffolding (per D-04)
- `EVAL.md` — provider TOS / retention attestation
- `reports/` — output directory for run artifacts

**Binary (`cmd/helix-eval/`):**
- `main.go` + `run_cmd_test.go` — `helix-eval run` and `helix-eval validate-rules` subcommands

**Profile (`internal/profile/profiles/`):**
- `baseline.yaml` — `skills:[] tools:[]`; sixth profile alongside the existing five
- regression: `internal/profile/baseline_test.go::TestBaselineExposesZeroHelixTools`

**CI (`.github/workflows/`):**
- `go-test.yml` — `make eval-quick` step + `forbid judge in CI` grep-gate (lines 106-116)

**Makefile targets:**
- `eval-quick` — in-process, <30s, CI-eligible (3.4s actual)
- `eval-no-network` — alias of `eval-quick` for explicit no-network semantics
- `eval` — full out-of-process matrix, local-only by project rule

---

_Audited: 2026-05-10 against `~/.claude/get-shit-done/references/ai-evals.md`, EVAL-01..EVAL-07 in `.planning/REQUIREMENTS.md`, and Phase 67 CONTEXT decisions D-01..D-08._
