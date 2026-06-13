# Pitfalls Research

**Domain:** v1.12 Bench Stack & Tool Evaluation — adapting 6 public coding benchmarks + internal ToolBench into Helix's existing Go-only persistent-daemon MCP architecture, with 6-mode ablations across 8 Tier-1 languages.
**Researched:** 2026-06-13
**Confidence:** HIGH

> **Scope note.** Every pitfall here is grounded in either (a) Phase 67's own audit findings (F-01/F-05/F-07/F-08/F-09/F-10 — `v1.10-MILESTONE-AUDIT.md`), or (b) published evidence from the specific public benchmarks v1.12 will adopt. Generic ML-eval pitfalls (e.g. "label noise", "metric overfitting") are intentionally excluded; the user already knows those. This document targets pitfalls that arise *because* we are wiring these benchmarks into the specific Helix v1.11 daemon shape.

## Severity Tags

- **BLOCKER** — results are not publishable until prevented.
- **HIGH** — will distort headline numbers; reviewers will spot it.
- **MEDIUM** — operational annoyance; degrades velocity, not credibility.
- **LOW** — worth a note in the report's caveats section.

---

## Critical Pitfalls

### Pitfall 1: Public-benchmark training-data contamination (BLOCKER)

**What goes wrong:**
The frontier model used to run `your_agent_full` has already seen SWE-bench tasks (and likely Aider Polyglot tasks lifted from Exercism, and CrossCodeEval examples from public repos) during pretraining. Headline numbers reflect *memorization*, not Helix's tooling. SWE-bench Verified (the curated 500-task slice) partially mitigates this with human re-verification, but the other five benchmarks ship no contamination control beyond Big-Bench canary strings — which are a symbolic safeguard, not an enforced one. Reviewers will demand a contamination disclosure; without one, the v1.12 claim *"with Helix the agent solves more tasks"* is unfalsifiable.

**Why it happens:**
The benchmarks are public on GitHub / HuggingFace; commercial training corpora include GitHub. Researchers conflate "this benchmark was released after the model cut-off" with "the model has not seen it" — but the benchmark *content* (e.g. the original Exercism repos, the upstream issue PRs) predates the model cut-off even when the benchmark *packaging* does not.

**How to avoid:**
1. Adopt SWE-bench Verified as the **only** SWE-family headline result — the human re-verification is the strongest contamination defense available. Treat raw SWE-bench Lite results, if computed, as informational only.
2. For every other benchmark, run a **canary-emission probe** during baseline: prompt the model to complete each task with *zero context* (no Helix, no shell, no RAG, just the task statement). Any pass@1 > 0 on this probe is evidence of memorization. Report the canary pass rate alongside the headline pass rate in `leaderboard.md`.
3. **Delta-only reporting**: the load-bearing claim is *Helix vs. baseline_plain on the same model on the same tasks*. Contamination affects both; the delta is what's credible. Make this explicit in the report copy.
4. Embed each benchmark's published canary string (Big-Bench for Terminal-Bench, etc.) verbatim in `bench/datasets/<name>/CANARY.txt` and in a comment header of every report-generation script — so we never accidentally publish a tainted artifact upstream into a training crawl.

**Warning signs:**
- baseline_plain pass@1 on SWE-bench Lite is suspiciously close to published SOTA at the same model.
- Canary-emission probe pass@1 > 5% on any benchmark.
- Per-task pass rate has heavy bimodality: clustered near 0 or near 1 with little middle ground (a memorization fingerprint).

**Phase to address:**
Dataset-acquisition phase (`bench/datasets/` bootstrap) owns canary-string embedding and the contamination-probe runner. Reports phase owns the canary-pass-rate column.

---

### Pitfall 2: Patch-validation false positives — passing tests on a wrong patch (BLOCKER)

**What goes wrong:**
SWE-bench's per-instance test suite only re-runs the test files modified in the upstream PR — not the full test suite. Empirical work (UTBoost, PatchDiff) shows ~31% of "solved" instances in SWE-bench Verified pass with semantically wrong patches; running just the PR-modified tests overstates pass rates by 4–7% absolute. A v1.12 claim that "Helix solves 5% more tasks" is *inside the false-positive band* of the benchmark itself unless mitigated. Same risk class for Multi-SWE-bench and Aider Polyglot (Exercism tests are not adversarial).

**Why it happens:**
The benchmark harness was designed for grading agents, not for proving correctness. Test-suite-as-oracle is convenient but fundamentally weaker than test-suite-plus-differential. Implementers blindly inherit `swebench.harness.run_evaluation` without overriding the test-discovery hook.

**How to avoid:**
1. **Run all tests, not just modified ones.** Override SWE-bench's per-instance harness to invoke the project's full test suite (or at minimum, all tests in the module touched by the patch). This is a documented mitigation in the SWE-bench correctness study and is a one-flag change in newer harness versions.
2. **Differential test execution.** For SWE-bench Verified specifically, compare patch behavior against the gold patch using property-based / mutation-derived tests. Implement as `bench/evaluators/swebench/differential.go` consuming the gold patch alongside the agent patch.
3. **Verified-correctness metric is a 2-of-N vote, not a single oracle.** `verified_correctness` (which v1.12 spec calls out as a top-line metric) must require both `tests_pass` AND at least one of `{gold_patch_diff_overlap > τ, no_new_diagnostics, mutation_survival_rate ≤ ρ}`. Single oracle = single point of failure.
4. **Inherit Phase 67's F-09 lesson.** Phase 67 originally derived `DiagnosticsClean` from `TestsPass` (a tautology — fixed in commit `93426cc4`). Repeating this pattern in v1.12 — e.g. `verified_correctness = tests_pass` — collapses the metric to a tautology in the opposite direction. Wire it from independent evidence (LSP diagnostics tap, AST diff vs. gold).

**Warning signs:**
- `verified_correctness ≡ tests_pass` for every task in a dry run.
- The delta `tests_pass - verified_correctness` is < 1% — likely you're double-counting one signal.
- Same patch hash recurs in multiple "solved" rows — a sign that the agent is exploiting a test-suite weakness.

**Phase to address:**
SWE-bench adapter phase owns the "run all tests" override and the differential runner. The metrics-layer phase owns the multi-oracle `verified_correctness` definition.

---

### Pitfall 3: Ablation-mode leakage via profile-filter escape paths (BLOCKER)

**What goes wrong:**
`no_lsp` mode is supposed to prove Helix-without-LSP is worse than Helix-with-LSP. But Helix has *implicit* LSP consumers — `analyze_blast_radius` calls `lspProbeForEdges` via the strangler-fig integration adapter (Phase 65, `INTEG-01`), the RepoMap LSP-enrichment fn (`SetEnrichFn` post-init wiring), and the live update pipeline (Phase 60 OnEdit hooks) all touch LSP under the hood. Profile-filter middleware filters `tools/list`, not internal kernel calls; an agent that never invokes `lsp_diagnostics` directly can still benefit from LSP through these back-channels. The ablation looks clean but isn't.

**Why it happens:**
Helix's architecture intentionally hides LSP behind a unified kernel surface. The profile filter is a *visibility* filter (what the agent sees in `tools/list`), not an *enforcement* filter (what the kernel is allowed to do). The author of the ablation mode assumes "if the tool isn't visible, the LSP isn't touched" — false in this codebase.

**How to avoid:**
1. **Distinct "ablation profiles" with a hard `disable_lsp_subsystem: true` flag in profile YAML**, plumbed all the way to `kernel.NewWorkspace(opts)`. When set, the LS worker pool refuses to start, `SetEnrichFn` is wired to a no-op, `lspProbeForEdges` returns `errors.New("lsp_disabled_by_ablation")`. Same pattern for `no_semantic` (semantic store + bleve disabled) and `no_structured_edit` (kernel/edit tools refuse with `Unsupported`).
2. **"Escape path audit" test per ablation mode.** A test harness that boots the daemon in `no_lsp` mode, runs `make bench-quick`, and asserts `helix_lspool_workers_started_total == 0` and `helix_semantic_*` family counters are zero. Wire into `make vet` so it runs every PR.
3. **Cross-check with the existing `vet-noduckdb` analyzer pattern** (Phase 57 STORE-06). Add a `vet-ablation-leakage` analyzer that flags any code path which conditionally bypasses ablation flags.
4. **Trace-tap assertion.** Re-use the F-07 daemon-tap (Phase 67) — assert *zero* `lsp.*` spans in the merged trace for any task run in `no_lsp` mode. Trace evidence is harder to fake than counter evidence.

**Warning signs:**
- `no_lsp` mode shows non-zero `lsp_diagnostics_used` in the metrics layer.
- Ablation-mode pass rates are within noise of `your_agent_full` for `no_lsp` — strongly suggests LSP is still doing work.
- `helix_repomap_extract_duration_seconds` histogram has non-trivial values in `no_semantic` mode (semantic facts shouldn't be reachable).

**Phase to address:**
Ablation-mode wiring phase owns the kernel-level disable flags and the escape-path audit tests. Profile-filter middleware phase owns the visibility filter (already shipped at v1.5; only audit work needed).

---

### Pitfall 4: Same-model fairness drift — temperature, retry, and budget asymmetry (BLOCKER)

**What goes wrong:**
The headline claim is *"same model + same budget"*. Trivial-to-introduce asymmetries that invalidate this:
- `your_agent_full` mode uses `temperature=0.0` (deterministic, exploit-Helix-tools-strictly) while `baseline_plain` defaults to `temperature=1.0`.
- `your_agent_full` auto-retries on transient errors (LS reconnect, fuzzy-edit refusal); `baseline_plain` does not.
- `your_agent_full` consumes more cached input (Anthropic prompt caching), so its *effective* per-task budget is higher under a token cap.
- The `claude` CLI (v1.12's only runner per Phase 67 EVAL.md scope) silently injects different system prompts depending on profile; ablation modes inherit the wrong one.
- Provider-side A/B model routing (e.g. `claude-sonnet-4.5` vs `claude-sonnet-4.5-pro`) makes the "same model" claim untrue mid-run.

**Why it happens:**
The agent CLI and the bench harness are two different programs talking past each other. Defaults differ. "Same model" feels obvious but the *runtime context* the model sees differs across modes.

**How to avoid:**
1. **`bench/runners/fairness_contract.go` — a single struct that pins every model-facing variable**: model ID *with version pin* (e.g. `claude-sonnet-4-5-20260520`, not `claude-sonnet-4-5`), temperature, top-p, max_tokens, system_prompt_hash, tool-use mode, retry policy, cache policy. Every mode boots the runner from this struct. CI gate refuses to merge a mode that overrides any field without an explicit `WaiverReason` tag.
2. **Token-budget fairness is per-task, not per-mode.** A task that gets 50K tokens in one mode gets 50K tokens in every mode. Enforce by intercepting the agent loop and hard-stopping at the budget — no soft retry past the cap.
3. **Pin the model snapshot in `bench/datasets/MODEL_SNAPSHOTS.yaml` and emit it into every result row.** When the provider deprecates the snapshot (Anthropic's Opus 4 retired 2026-06-15 per the public deprecation policy), the bench refuses to run and surfaces a maintainer-action item.
4. **Cache parity:** disable Anthropic prompt caching globally for v1.12 headline runs (or enable it identically across every mode). A separate `with_cache` sweep can report cache benefit as a secondary analysis.
5. **Per-mode system-prompt golden file.** `bench/runners/goldens/<mode>.system.md` is the *only* system prompt the runner injects. Override at runtime fails the contract test.

**Warning signs:**
- Per-task token-spend distributions across modes have wildly different shapes (suggests one mode is retrying more).
- A mode's `wall_time_seconds` p95 is 3× another's — usually retry asymmetry.
- The same task produces deterministic results in one mode and nondeterministic in another — temperature drift.

**Phase to address:**
Fairness-contract phase (early — must land before any benchmark adapter). All benchmark adapters consume the contract; they cannot define their own.

---

### Pitfall 5: Token-counting attribution boundary (HIGH)

**What goes wrong:**
Helix's existing token meter (Phase 67 `cost_summary.json`) counts *MCP-level* tokens — what flows in and out of the daemon. The v1.12 headline claim *"with Helix the agent uses fewer tokens"* must count the *agent's tokens to the model* — what flows in and out of the LLM provider. These are different numbers: Helix's RepoMap can pre-summarize context (reduces model-facing tokens), but Helix's structured tool responses can be more verbose at the MCP layer (inflates daemon-side counting). Reporting the wrong number either over- or under-claims the benefit.

**Why it happens:**
The Phase 67 cost meter was designed to track Helix's own resource use, not the agent's. v1.12 inherits the wrong counter by default. Reviewers will diff Helix's reported token count against the provider's billing dashboard and catch the discrepancy.

**How to avoid:**
1. **Two counters, both reported:** `tokens_to_model` (the only headline number) sourced from the agent CLI's transcript / provider response usage block; `tokens_through_daemon` (informational only) from Helix's existing meter. Schema enforced via `bench/result_schema.json` — both fields required.
2. **`tokens_to_model` is sourced from the provider response, not estimated.** Anthropic returns `usage.input_tokens` / `usage.output_tokens` in the response envelope. Parse and persist verbatim. Never use a tokenizer-based estimate as the headline.
3. **Cache-input tokens reported separately.** Anthropic cached input is 0.1× base price; bundling it with regular input either over- or under-states cost. Schema: `tokens_input_uncached`, `tokens_input_cached_read`, `tokens_input_cache_write`, `tokens_output`.
4. **`cost_per_solved_task = (uncached_in × price_in) + (cached_in × price_cached) + (out × price_out)` with the price table pulled from a pinned snapshot file**, not a live API call. See Pitfall 6.

**Warning signs:**
- `tokens_to_model` / `tokens_through_daemon` ratio < 0.5 — daemon is counting bytes the model never sees.
- The reported `cost_per_solved_task` doesn't match the provider's billing for the same run window within 5%.
- Cached-input tokens are zero across every task in a multi-task run — likely the agent is reusing the system prompt and we're not capturing cache reads.

**Phase to address:**
Metrics-layer phase owns the two-counter schema. Cost-conversion phase owns the cached-input split.

---

### Pitfall 6: Static cost-table drift (HIGH)

**What goes wrong:**
The v1.12 spec calls for `cost_per_solved_task` and a static price table per provider × model. LLM API pricing has dropped an order of magnitude in two years and "the cuts often arrive without an email." A bench published in June and re-run in September with the old table reports inflated costs for the new pricing. Worse: Aider and Sourcegraph (peer projects) ship pricing tables that go stale within weeks; reviewers will check.

**Why it happens:**
Pricing tables are a config file, not an oracle. They have no expiration. Provider price pages change without API versioning. The CI never fails because the pricing is "correct" by the static file's own lights.

**How to avoid:**
1. **Pinned snapshot with a hard expiration date in the file header**: `valid_until: 2026-09-01`. `bench/cost/prices.yaml` parser refuses to load if `time.Now() > valid_until`. Makes staleness loud rather than silent.
2. **Pricing fetched fresh per headline run, with snapshot diff reporter.** Phase that runs `make bench` calls a thin scraper / Helicone-style API (or the providers' official pricing endpoints where available) at run start, diffs against the pinned snapshot, and refuses to run if drift > 0% on any used model. Maintainer must explicitly bump the snapshot.
3. **Effective cost is a derived column, not a stored one.** `cost_per_solved_task` is computed at report time from the raw token columns × the snapshot price. Re-running the report with a new snapshot re-computes costs without re-running the bench.
4. **Source the schema from one of the existing aggregators** (`pagecrawl.io`, `costgoat.com`, `morphllm.com`) rather than each provider's page — but pin a specific snapshot and never auto-update.

**Warning signs:**
- The pricing snapshot is older than 60 days at run time.
- `cost_per_solved_task` for the same task differs across two runs of the same model — usually the price changed.
- The headline `cost_per_solved_task` doesn't match `provider_billing_dashboard / tasks_solved` within 10%.

**Phase to address:**
Cost-conversion phase owns the snapshot expiry. Reports phase owns the derived-column pattern.

---

### Pitfall 7: Provider TOS / retention drift (HIGH)

**What goes wrong:**
Phase 67's EVAL.md attests "retention-zero verified for every commercial provider used." Provider TOS change. A retention-zero attestation made 2026-06-13 can flip to retention-30d by the time `make bench` runs in CI in September. v1.12 publishes benchmark results that train future models. Reviewers (and contributors with sensitive code) will catch the drift, and the v1.12 retention claim becomes a credibility breach.

**Why it happens:**
TOS is checked manually at planning time and never re-checked. There's no test that asserts "the TOS we attested against is still the live TOS." Changes are silent and policy-only (no API change to detect).

**How to avoid:**
1. **`bench/providers/tos_attestation.yaml`** per provider with `attested_at`, `tos_url`, `tos_sha256`, `retention_clause_excerpt`. CI gate refuses to run any bench against a provider whose attestation is older than 30 days.
2. **TOS hash check as a make target**: `make verify-tos` downloads each provider's TOS URL, hashes it, compares to the attestation. Fails on mismatch. Wire into `make bench` as a precondition.
3. **Zero-data-retention is opt-in per request via the provider API** for Anthropic / OpenAI enterprise tiers — assert via response header (`anthropic-version` and equivalent retention headers) on every request, not just at planning time.
4. **Public commit of the attestation file**: makes it falsifiable. Auditors can re-hash the live TOS and compare.

**Warning signs:**
- TOS attestation file > 30 days old at run time → CI refuses to run.
- Provider response is missing the expected `retention` header → harness aborts the task.
- A new model snapshot from the same provider lacks an attestation → refuses the model in fairness-contract loading.

**Phase to address:**
Provider-TOS phase owns the attestation file and the make verify-tos gate. CI workflow phase wires it into `make bench`.

---

### Pitfall 8: Container disk-space explosion + image-pull rate limits (HIGH)

**What goes wrong:**
SWE-bench's unoptimized full image set is 684 GiB; Verified is 189 GiB; even with the logicstar.ai 10×–6× optimization, it's 67 GiB / 30 GiB respectively. Multi-SWE-bench adds 7 languages worth more. Terminal-Bench is containerized too. A `make bench` invocation on a contributor laptop or CI runner that pulls the full image set thrashes layer cache and runs out of disk. Worse: pulling from Docker Hub triggers anonymous rate limits (100 pulls / 6 hours / IP); GHCR has its own limits. CI jobs flake without explanation.

**Why it happens:**
The SWE-bench harness defaults to `cache_level=env` and reuses images across runs — which is correct, but the user running `make bench-quick` for the first time on a 100 GB laptop disk has no idea what they're about to download.

**How to avoid:**
1. **`make bench-quick` uses only Verified + a 10-task subset of Multi-SWE-bench + 5 Terminal-Bench tasks** — pre-pulled image manifest under 30 GB total. Documented in INSTALL.md.
2. **Adopt the logicstar.ai optimized image set** (or replicate the optimization technique: layer dedup, base-image consolidation) so the full SWE-bench Verified set is 30 GB, not 189 GB.
3. **Cosign-verified image mirror in GHCR under `agenthands/helix-bench-*`** — Helix already has cosign keyless infra (Phase 58); reuse. Avoids Docker Hub rate limits, gives us a fixed-content-hash dataset surface, and matches Helix's existing release distribution pattern.
4. **Pre-flight disk check**: `make bench` refuses to start if free disk < `2× estimated_image_set_size`. Estimated size computed from the dataset manifest, not hard-coded.
5. **CI uses the `large-runner` GitHub Actions tier** for `make bench` jobs (separate from `make eval-quick` / `make bench-quick` which stay on `ubuntu-22.04`). Document in `.github/workflows/bench-full.yml`.

**Warning signs:**
- A `make bench-quick` run takes > 30 min in setup (image pull) — indicates default image set is too big.
- Docker Hub `toomanyrequests` HTTP 429 in CI logs — anonymous pull limit hit.
- CI runner reports disk pressure mid-job — image cache thrash.

**Phase to address:**
Container-runtime phase owns the optimized image set + GHCR mirror + disk-pre-flight. CI workflow phase owns the runner tier choice.

---

### Pitfall 9: Per-language test-runner brittleness (HIGH)

**What goes wrong:**
Aider Polyglot runs Exercism tests in 6 languages (C++, Go, Java, JS, Python, Rust). Multi-SWE-bench runs project-specific tests in 7 (Java, TS, JS, Go, Rust, C, C++). Internal ToolBench targets 8 (Python, TS, JS, Go, Java, C#, C++, Rust). Each language needs its own runtime + test runner — `mvn` / `gradle` / `dotnet` / `cargo` / `go test` / `pytest` / `npm test`. If these are installed at task-run time, every run is at the mercy of:
- Maven Central / npm registry / NuGet / crates.io being up.
- Network latency dominating wall_time_seconds (a fairness contaminant).
- A new release of a transitive dep silently changing test outcomes.
- A flaky network nullifying a 12-hour bench run.

This is exactly the Phase 48 / Phase 56 jdtls cold-start lesson at benchmark scale.

**Why it happens:**
The naive port of upstream benchmarks calls their `Dockerfile`s as-is, which install toolchains at image-build time but resolve deps at test-run time.

**How to avoid:**
1. **Pre-baked per-language toolchain images** with deps offline-resolved (`mvn -o`, `cargo --offline`, `pnpm install --offline --frozen-lockfile`, `pip install --no-index --find-links=...`). Built once during `make bench-setup`, signed with cosign, mirrored to GHCR.
2. **Hermetic test runs**: `--network=none` for the test phase of every task. If the test phase needs network, that's a task-level failure flagged in the dataset as "non-hermetic, skip in headline."
3. **Cache the dep resolution graph**, not the deps themselves. A pinned `Cargo.lock` / `package-lock.json` / `pom.xml` with `<dependencyManagement>` for every task instance — recorded in `bench/datasets/<name>/lockfiles/<task_id>/`. Replay produces byte-identical dep trees.
4. **Apply the Phase 48 jdtls-warm-cache pattern** to all 8 languages: pre-warm the test runner once per workspace, reuse for every (task × mode) in that workspace.

**Warning signs:**
- `wall_time_seconds` distribution has a fat tail in one language (network flake).
- Task fails with "could not resolve dependency" — not hermetic.
- Different (task, mode) cells in the same language report different wall_time_seconds by > 50% — runtime warming asymmetry.

**Phase to address:**
Per-language toolchain phase owns image pre-bake. Hermeticity-audit phase owns the `--network=none` enforcement.

---

### Pitfall 10: Edit-locality metric gaming (HIGH)

**What goes wrong:**
The v1.12 spec lists `edit_locality` as a top-line metric and `regression_rate` as another. The framing — *"fewer files modified"* — sounds like a positive signal. But an agent that ducks important cross-cutting refactors looks great on `edit_locality` while quietly breaking other tests. If `regression_rate` is computed only over the test files that were modified (same SWE-bench failure mode as Pitfall 2), `edit_locality` and `regression_rate` *both* look good while real-world correctness collapses.

**Why it happens:**
Edit-locality as a metric was popularized by Aider as a *correlate* of agent quality, not a *target*. Optimizing for it directly creates Goodhart's law dynamics. The spec doesn't distinguish "fewer edits because the agent is precise" from "fewer edits because the agent is lazy."

**How to avoid:**
1. **`edit_locality` is conditioned on success**: `edit_locality_given_solved` (median files modified across solved tasks) is the headline; raw `edit_locality` across all tasks is informational only. An agent that solves 0 tasks has perfect raw edit_locality.
2. **`regression_rate` is computed against the *full* test suite** (not just modified tests) — see Pitfall 2's mitigation. Otherwise the two metrics are tautologically correlated.
3. **Report `edit_locality` alongside `gold_patch_files_count`.** If the gold patch touches 5 files and the agent touches 1, that's not better — that's incomplete. Include `edit_locality_ratio = files_modified / gold_patch_files_count`; values << 1 are red flags.
4. **Adversarial subset**: a curated 10-task "must-refactor-across-files" subset where edit_locality < gold_patch_files_count is *definitionally* a failure, used as a fairness check.

**Warning signs:**
- `your_agent_full` has lower `edit_locality` than `baseline_plain` AND lower `task_success`. Almost certainly gaming.
- `edit_locality_ratio` median < 0.7 across solved tasks — agent is skipping real edits.
- The 10-task adversarial subset has > 50% solved with `edit_locality < gold_patch_files_count`.

**Phase to address:**
Metrics-layer phase owns the conditioning-on-success transformation. Adversarial-subset phase owns the must-refactor curation.

---

### Pitfall 11: Bootstrap CI math — too few resamples, naïve percentile, no bias correction (HIGH)

**What goes wrong:**
v1.12 spec calls for "bootstrap confidence intervals." Naïve percentile bootstrap (`np.percentile(boots, [2.5, 97.5])`) is biased for skewed estimators like `pass@1` on a benchmark of 225 tasks. Cohen's tools all use BCa (bias-corrected and accelerated) for this exact reason. Too-few resamples (e.g. N=200) means the CI itself has stochastic noise comparable to the effect we're measuring. Too many (e.g. N=100,000) wastes compute on closed-form-derivable bounds (`pass@1` on N independent tasks has a closed-form Wilson interval). Reviewers will check.

**Why it happens:**
The spec says "bootstrap" without specifying which kind. Default implementations in numpy / scipy.stats give percentile bootstrap. BCa is more code (jackknife for acceleration term) and the implementer skips it.

**How to avoid:**
1. **Use BCa, not percentile.** `scipy.stats.bootstrap(..., method='BCa')` or Go-side equivalent. The acceleration term is computed via jackknife. Document the choice in `bench/evaluators/stats.go` header.
2. **N_resamples ≥ max(10000, 1/alpha)** per the standard rule of thumb; default to N=10,000 for alpha=0.05. Wilson interval for `pass@1` on Bernoulli trials is a closed-form alternative — use it when applicable to save compute, BCa when the estimator is non-Bernoulli (e.g. cost per solved task).
3. **Report the CI alongside every headline number.** No bare point estimates. Format: `42.3% ± 3.1% (95% BCa)`.
4. **Power analysis as a precondition.** Before running, compute the minimum detectable effect size given N tasks and N runs per task. If MDE > expected effect, the bench is underpowered — refuse to publish a non-significant result as "no difference." `bench/evaluators/power.go` owns this.

**Warning signs:**
- CI widths differ wildly across two reruns of the same data — N_resamples too low.
- Reported CI is symmetric around the point estimate for a near-boundary value (e.g. 98.5% ± 2%) — used percentile, should be BCa.
- The minimum detectable effect at the chosen N is larger than the headline claim.

**Phase to address:**
Statistical-rigor phase owns the BCa implementation. Power-analysis phase is a precondition gate.

---

### Pitfall 12: Reproducibility — task ordering, time-of-day, model A/B routing (HIGH)

**What goes wrong:**
Same bench, same model, same task → different result tomorrow. Causes specific to v1.12:
- **Task ordering**: if the agent has a long-running conversation across tasks (it shouldn't, but the runner might leak state across `claude` invocations), task K's result depends on task K-1.
- **Time-of-day effects**: provider load varies. p95 latency at 3am vs 3pm differs by 2× on busy days; if `wall_time_seconds` is a headline, this contaminates it.
- **Provider-side A/B model routing**: Anthropic, OpenAI silently route to different model versions for load shedding. Same `claude-sonnet-4-5` ID → different weights mid-run.
- **Model deprecation mid-experiment**: Opus 4 retires 2026-06-15; if a multi-week bench straddles the date, half the runs use a model that no longer exists.

**Why it happens:**
The agent CLI hides session state inside its config dir. Time of day is an unspecified covariate. Provider routing is opaque. Deprecation is announced but not enforced at runtime.

**How to avoid:**
1. **Each (task × mode × run) gets a fresh sandbox HOME** (Phase 67's existing per-(task,mode) sandbox pattern — extend with `--run-id` for repeated runs). Asserts no state leak across tasks.
2. **Randomize task order per run; report seed.** Eliminates ordering bias. Seed pinned in result rows for reproducibility.
3. **Pin the model snapshot to a versioned ID** (e.g. `claude-sonnet-4-5-20260520`, not `claude-sonnet-4-5`). Anthropic exposes both; v1.12 must use the dated form. Fairness-contract (Pitfall 4) enforces.
4. **Capture provider response headers** including `anthropic-version` and any model-version-hint headers, persist in result rows. If the captured version drifts mid-run, abort.
5. **`wall_time_seconds` is reported with `network_latency_p50` from a separate noise probe** running in the same minute window. Decouples agent thinking time from provider latency.
6. **Deprecation calendar gate**: `bench/datasets/MODEL_SNAPSHOTS.yaml` has `deprecation_date`. Runner refuses to start a run that would not complete before deprecation.

**Warning signs:**
- Two consecutive runs of the same task differ by more than the documented N=3 multi-run noise band.
- Task K's pass rate correlates with task ordinal position — state leak.
- Captured `model_version` header shifts mid-run — A/B routing.

**Phase to address:**
Reproducibility-harness phase owns sandbox-per-run, task randomization, version-pin enforcement, deprecation calendar.

---

### Pitfall 13: Trace-merging race (Helix daemon OTel + claude CLI stream-json) (MEDIUM)

**What goes wrong:**
Phase 67 already wired the merged trace tap (`internal/eval/trace/`) and the F-07 close-out (commit `e7bc406f` + `ab8ad6cb`) proved subprocess daemon-tap works. v1.12 extends to 6 benchmarks × 6 modes × 8 languages — orders of magnitude more concurrent runs. The existing merge-by-timestamp logic on a shared box hits races: two daemons writing to the same JSONL inode if `$HELIX_LOG_DIR` is shared; clock skew between forwarder and daemon spans (sub-ms but enough to invert ordering); `claude` CLI's stream-json events interleaved with Helix's tool-call JSONL.

**Why it happens:**
JSONL append-merge is correct for a single-writer model. v1.12 has N concurrent (task × mode) cells, each with its own daemon subprocess. The shared-host assumption from Phase 67 breaks.

**How to avoid:**
1. **PID-gated per-cell JSONL paths**: `$BENCH_RUN_DIR/<task>/<mode>/<run>/{daemon.jsonl, agent.jsonl}` — already 70% of Phase 67's design via the `sandbox.StartDaemon` + PID gate. Tighten to include `run` in the path.
2. **Re-use the F-07 PID gate verbatim.** `trace.TapDaemonLog` already filters by daemon PID; the existing integration regression test (`internal/eval/runner/daemon_tap_integration_test.go`) is the right pattern. Add a cross-cell test that boots two daemons in parallel and asserts zero cross-talk in either merged trace.
3. **Use trace_id, not timestamp, for ordering.** Phase 65 already wired propagation.TraceContext{}; agent → daemon → LSP span chain shares trace_id. Merge on trace_id within a span tree, fall back to monotonic span_id for siblings. Document at `bench/runners/trace_merge.go`.
4. **Hard fence between concurrent cells**: parallel runner spawns ≤ N cells, where N = `runtime.NumCPU()/2`. Beyond that, throughput gains are eaten by daemon contention and LS port conflicts (each cell needs a unique gRPC forwarder socket).

**Warning signs:**
- Merged trace shows a span with `start_time > end_time` of its parent — clock skew, not handled.
- A daemon JSONL has events with a PID not equal to its expected daemon PID — cross-talk.
- Spans appear "from the future" in a merged trace — clock or PID gate broken.

**Phase to address:**
Trace-merge phase owns the per-cell JSONL paths and the trace_id-based merge.

---

### Pitfall 14: Dataset license traps (MEDIUM)

**What goes wrong:**
Each benchmark has different licensing; bundling them into the `helix` binary or even shipping a single `make bench-setup` that downloads them all is a potential redistribution violation. Specifics from the research:
- **Aider Polyglot**: exercises are "copyright © Exercism, used in accordance with Exercism's open source licenses" — not blanket MIT. Each language track on Exercism is individually licensed (most MIT, some Apache-2.0). Downstream re-bundling requires per-track audit.
- **SWE-bench**: research license; the *dataset* (HuggingFace) is one license, the *harness* (GitHub) is MIT, the *underlying repos* keep their own licenses (mostly MIT/Apache, but a few BSD/LGPL).
- **Multi-SWE-bench** (ByteDance): license not listed on the HuggingFace dataset card in our research — must be checked before bundling.
- **CrossCodeEval** (Amazon Science): permissively-licensed source repos only, but the curation script itself is a different license.
- **RepoBench**: not surveyed; must be audited.
- **Terminal-Bench**: Apache-2.0 dataset, with Big-Bench canary string.

If `make bench-setup` redistributes any of these from `agenthands/helix-bench-*`, we're a distributor under the relevant licenses.

**Why it happens:**
Maintainers focus on the model code and accept the benchmark's existence as a fait accompli. License audit is a one-time, low-glamour task that gets skipped.

**How to avoid:**
1. **Per-dataset `bench/datasets/<name>/LICENSE-AUDIT.md`** with: upstream license URL, sha256 of the LICENSE file at acquisition time, redistribution clause excerpt, our distribution decision (link-only / mirror with attribution / refuse to bundle).
2. **`make bench-setup` downloads from upstream, never redistributes from our infra** for datasets where redistribution is restricted. Cosign-verified mirrors only for datasets we have explicit redistribution rights for (Terminal-Bench Apache-2.0, SWE-bench harness MIT).
3. **CI license-check gate**: `make verify-licenses` parses every `LICENSE-AUDIT.md`, verifies the sha256 against a live download, fails if drift detected.
4. **`NOTICE` file in the Helix repo** consolidating attribution for every redistributed dataset.
5. **Refuse to include a dataset in v1.12 if its redistribution clause is ambiguous.** Defer to the next milestone after a maintainer resolution.

**Warning signs:**
- A dataset's LICENSE file changes upstream — sha256 drift in CI.
- `make bench-setup` includes a download from a non-upstream mirror without a documented redistribution right.
- A user reports a license complaint on the GitHub repo — already too late.

**Phase to address:**
License-audit phase owns the per-dataset audit + NOTICE file + CI gate. Dataset-acquisition phase consumes the audit before configuring download targets.

---

### Pitfall 15: Migrating Phase 67 corpus and the `eval/` → `bench/` rename (MEDIUM)

**What goes wrong:**
v1.12 spec says `bench/` is net-new, `eval/` is left as legacy, the synthetic 30-task Phase 67 corpus "may migrate later but is not the source of truth." But Phase 67's eval results are persisted with task IDs like `eval/corpus/T-67-04`. If the corpus migrates to `bench/datasets/internal-toolbench/T-67-04`, every historical result row is broken. If it doesn't migrate, the dual-tree confusion becomes a permanent operational tax (which Makefile target do I run? which trace tap do I read?). And `make eval` / `make eval-quick` already exist as commands users have muscle memory for.

**Why it happens:**
A new tree is easier than refactoring an existing one. The cost is silent until the second user hits the rename and reports old result rows as "missing."

**How to avoid:**
1. **`eval/` stays read-only at v1.12.** No new tasks added. Historical results in `eval/results/` are untouched. `make eval` / `make eval-quick` continue to work against the legacy corpus.
2. **`bench/datasets/internal-toolbench/` is a fresh corpus**, not a migration. New task IDs (e.g. `IT-py-rename-001`), new oracles, new format. No collision with Phase 67 IDs.
3. **One-way crosswalk file** `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` documents which Phase 67 tasks were the inspiration for which v1.12 internal-toolbench tasks. Historical results stay queryable; new results compute the new metrics.
4. **`make eval-quick` is deprecated, not removed**: prints a warning pointing users at `make bench-quick`, keeps working for one milestone. Removal scheduled for v1.13.
5. **Reuse Phase 67 infrastructure verbatim where possible**: the trace tap (`internal/eval/trace/`), the sandbox (`internal/eval/sandbox/`), the reporters (`internal/eval/report/`) — rename the package to `internal/bench/...` only if it's the *last* milestone before users notice. Otherwise leave the import path alone and add new packages under it.

**Warning signs:**
- A contributor opens an issue "where did task T-67-X go?" — corpus moved without crosswalk.
- `make eval` works but produces different numbers than before — quietly migrated, not preserved.
- Internal-toolbench task IDs collide with Phase 67 task IDs (e.g. both have a `T-04`).

**Phase to address:**
Migration-plan phase owns the eval-stays-frozen decision + crosswalk + deprecation timeline.

---

### Pitfall 16: Verified-correctness signal collapse via LLM judge (MEDIUM)

**What goes wrong:**
The spec lists `verified_correctness` as a top-line metric and Phase 67 has an informational LLM judge that's grep-gated to "non-reference" in CI (i.e., judge cannot be a release gate). v1.12 will be tempted to promote the judge to a `verified_correctness` contributor because writing real differential tests for 7 SWE-bench-family benchmarks is a lot of work. Model-feedback loops follow: if the judge is the same model family as the agent (`claude-*` judging `claude-*` output), the judge systematically rewards the agent's style. If the judge is a different model (e.g., GPT-5 judging Claude output), inter-rater agreement is moderate at best. Either way: the `verified_correctness` headline collapses into "the judge thinks so."

**Why it happens:**
Differential / mutation testing is expensive to build. LLM judges look like a cheap shortcut. The judge being "informational" in Phase 67 was a hard-fought constraint (EVAL-07); under v1.12 schedule pressure, the constraint will be re-litigated.

**How to avoid:**
1. **Inherit Phase 67 EVAL-07 verbatim**: LLM judge stays informational, grep-gated, never gates a release or contributes to `verified_correctness`. Audit at PR review.
2. **`verified_correctness` is composed of mechanical oracles only**: tests-pass + differential-test-pass + diagnostics-clean + edit-locality-ratio-in-bounds. Each oracle is a function whose source code is in the repo. No model in the loop.
3. **If a judge is computed, it goes in a separate `informational_quality_score` column.** Schema-enforced. Reports may show it; the headline number never includes it.
4. **Cross-family judge required if judge is used at all.** Anthropic agent → OpenAI judge (or vice-versa). Same-family judge is banned in the schema validator.

**Warning signs:**
- A PR proposes `verified_correctness = mean(tests_pass, judge_score)` — reject at review.
- `informational_quality_score` correlates with `verified_correctness` at r > 0.95 across all modes — judge is tracking tests, not adding signal.
- Judge model family matches agent model family in a release run — schema violation.

**Phase to address:**
Verified-correctness phase owns the mechanical-oracles-only composition. Judge phase (if any) owns the cross-family constraint.

---

### Pitfall 17: CI cost — full bench on every PR (MEDIUM)

**What goes wrong:**
Phase 67 `make eval-quick` is in-process and free; full eval is local-only. v1.12 has ~2,500 SWE-family tasks alone, × 6 modes, × N=3 runs minimum, × per-task LLM cost (~$0.10–$5 each at 2026 prices). One headline run is in the $1K–$10K range. If this runs in CI on every PR, monthly burn is enormous. If it runs only locally, only maintainers with deep pockets can publish results. If it runs only on `main` post-merge, regressions are detected late.

**Why it happens:**
The bench is too valuable to run never, too expensive to run always. The default `make bench` is interpreted as "what CI does" because that's the convention for every other make target.

**How to avoid:**
1. **Inherit Phase 67's split exactly**: `make bench-quick` is the in-process / no-LLM path that runs on every PR (token-counting smoke, harness smoke, schema lint, statistical-rigor self-tests). `make bench` is local-only or release-tag-only — never on PR. Document loudly in CONTRIBUTING.md.
2. **A reduced "smoke bench" subset** (~50 tasks, 1 run each, only `your_agent_full` mode, only `claude-haiku` or DeepSeek) runs on `main` post-merge daily. Cost cap: $20/day. Drift detector flags week-over-week pass-rate changes > 5%.
3. **The full bench runs on release-candidate tags only** (`v1.12.0-rc*`) — gated on a maintainer label `bench-full-ok`. Results published to `.planning/milestones/v1.12-BENCH/`. Cost is borne against milestone budget, not PR review budget.
4. **Self-hosted runner for bench-full**: a long-lived box with pre-warmed images, persistent dataset cache, dedicated provider keys with usage caps. Eliminates per-job image pull cost.
5. **Per-PR `bench-quick` budget enforced in CI**: if a contributor's PR adds tasks that push `bench-quick` past 5 min, CI fails. Keeps the on-every-PR path actually quick.

**Warning signs:**
- A maintainer opens a PR with "run full bench" in CI — should be impossible.
- Monthly provider invoice exceeds the milestone budget — likely the smoke-bench cap isn't enforced.
- `bench-quick` runtime > 5 min — accidentally pulling in expensive tasks.

**Phase to address:**
CI-policy phase owns the gating split. Smoke-bench phase owns the daily drift detector.

---

### Pitfall 18: F-07-class daemon telemetry capture in production benchmarks (MEDIUM)

**What goes wrong:**
Phase 67's F-07 finding — *"Phase 67 subprocess eval cannot observe daemon telemetry"* — was a BLOCKER that required two atomic commits (`e7bc406f` + `ab8ad6cb`) to close. The fix wired `TelemetryMiddleware` to emit tap-compatible JSONL and made `Runner.RunTask` boot a real daemon via `sandbox.StartDaemon`. v1.12 introduces ablation modes that boot daemons with *different* profiles (`no_lsp`, `no_semantic`, `no_structured_edit`). Each mode's daemon will have its own middleware stack. If `TelemetryMiddleware` is profile-scoped and a profile silently disables it, the F-07 regression is back — and now intermittent across modes.

**Why it happens:**
The profile YAML system is flexible enough to disable middlewares; an author of `no_lsp` profile might also strip `TelemetryMiddleware` for performance "because the bench doesn't need observability." Wrong: the bench *is* the observability consumer.

**How to avoid:**
1. **`TelemetryMiddleware` is non-optional** in every profile used by `bench/`. Enforce via a profile-loading invariant in `internal/profile/load.go`: profiles tagged `bench:true` MUST have telemetry middleware. CI gate.
2. **Re-use the existing F-07 regression test** (`internal/eval/runner/daemon_tap_integration_test.go`) verbatim, parametrize across all 6 ablation modes. Each mode must produce at least 1 `tool call` JSONL line.
3. **The F-08 receipt-issued JSONL** (guardrails) similarly must be emitted in every mode that has guardrails configured. Same parametric test.
4. **A "telemetry presence" oracle in the bench harness**: every (task × mode × run) result row must have a non-empty `daemon_tap_events_count`. Schema validator fails the row if zero.

**Warning signs:**
- `daemon_tap_events_count == 0` for any task in any mode — telemetry got dropped.
- Mode A and mode B for the same task produce same `daemon_tap_events_count` — suspicious, probably one is fake.
- A new profile YAML PR removes `telemetry` from the middleware list — reject at review.

**Phase to address:**
Profile-invariant phase owns the bench-tagged enforcement. Daemon-tap-regression phase parametrizes the F-07 test across modes.

---

### Pitfall 19: Real-test-runner brittleness via fuzzy-edit cascade (LOW)

**What goes wrong:**
Helix's fuzzy-edit cascade (Phase v1.6, 4-strategy: exact / whitespace / indentation / ellipsis) can silently apply slightly-wrong edits that still pass the test suite. Combined with Pitfall 2 (insufficient tests), the agent gets credit for a "solved" task when the edit didn't match what the agent intended — the strategy fell through to a permissive match. Helix already refuses *ambiguous* fuzzy matches; this is specifically about edits where the cascade fell from exact to ellipsis-placeholder and the resulting patch is acceptable to the tests but semantically drifted.

**Why it happens:**
The cascade is designed to tolerate LLM output drift — a feature for the IDE / agent use case, a contaminant for benchmarks where we want to know *what the model actually intended*.

**How to avoid:**
1. **Result schema includes per-edit `fuzzy_strategy_used`** (already reported by the tool). Aggregate `fuzzy_cascade_depth_p95` per task. Tasks where p95 > "whitespace" (i.e., needed indentation or ellipsis) are flagged.
2. **In `your_agent_full` headline mode for benchmarks, disable the indentation and ellipsis strategies.** Bench is not the IDE — we want a clean signal of what the model would produce against a strict edit tool. Wire via a config knob `fuzzy.max_strategy: whitespace` plumbed to `internal/fuzzy/`.
3. **Report a per-mode `fuzzy_edits_count`**; if it's a substantial fraction of total edits, headline number gets a footnote.

**Warning signs:**
- `fuzzy_cascade_depth_p95 == "ellipsis"` for > 10% of tasks — bench is exercising tolerance, not intent.
- Disabling fuzzy strategies drops `task_success` by > 5% — the headline was partly fuzzy-cascade-credit.

**Phase to address:**
Fuzzy-config phase owns the bench-strict mode.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Use Phase 67 `internal/eval/...` packages without rename | Reuse working trace tap, sandbox, reporters | Confused dual `eval/` + `bench/` directory + import-path schism; users don't know which Makefile target to run | First milestone (v1.12) only; rename in v1.13 with a migration script |
| Single-oracle `verified_correctness = tests_pass` | Easy to implement; matches SWE-bench upstream | F-09 regression class — tautology that overstates correctness; reviewers spot in a week | Never — Pitfall 2 mandates multi-oracle |
| Pull benchmark images from upstream Docker Hub directly | Avoids hosting cost; one less mirror | Anonymous rate limits (100/6h/IP) silently flake CI; no cosign signature path | Local dev only; release runs MUST use cosign-verified GHCR mirror |
| Naïve percentile bootstrap with N=1000 | One line of scipy; "good enough" | Reviewers diff against BCa and the CI band shifts; underpowered comparisons published as "no significant difference" | Internal informational reports only; never for the headline |
| LLM judge for `verified_correctness` | Cheap relative to writing 6 differential test suites | Model-feedback loop; same-family judge rewards agent style; headline becomes "judge thinks so" | Never; `informational_quality_score` only |
| Per-task fresh dep install (mvn / npm / cargo at run time) | No image pre-bake work upfront | Wall-time contamination, network flake, non-hermetic results | Smoke runs / local dev only |
| Reuse Phase 67 corpus IDs in v1.12 internal-toolbench | Saves a renumbering exercise | Historical Phase 67 results queries return inconsistent rows; tooling ambiguity | Never — collide-free namespacing only |
| Disable `TelemetryMiddleware` in ablation modes for "performance" | A handful of µs per call | F-07 regression class — bench loses observability into the kernel | Never — non-optional in bench profiles |
| Static price table without expiration | Loads cleanly; no scraper code | Drift → over-claimed cost savings; reviewer catches against billing dashboard | Never — hard expiration mandatory |
| Single-run per task ("just one run is fine to get started") | Halves wall-clock cost during dev | Bench is not publishable; the v1.12 spec explicitly requires N ≥ 3 | Dev iteration only; release MUST be N ≥ 3 |
| Skip TOS attestation refresh | Saves a make target | Retention claim becomes false silently; credibility breach | Never |

---

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Anthropic API | Reading `usage` from a `messages.create` response only; missing cache reads/writes | Parse `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens` separately; cached input is 0.1× base price |
| Anthropic API | Pinning to floating model name `claude-sonnet-4-5` | Pin to dated snapshot `claude-sonnet-4-5-20260520`; deprecation calendar gate before any run |
| `claude` CLI | Inherits user's `~/.claude/` config when bench runs as that user | Each (task × mode × run) gets fresh `HOME` sandbox; verified by F-07 close-out pattern |
| `claude` CLI | Different stream-json schema across versions | Pin `claude` CLI version in `bench/runners/AGENT_VERSIONS.yaml`; refuse to run with drift |
| Docker Hub | Anonymous pull rate limits (100/6h/IP) | Cosign-verified mirror in GHCR under `agenthands/`; pre-pull manifest as a make target |
| GHCR | 30-day cache eviction on free tier for unused images | Touch each image weekly via a CI cron; treat as content-addressed not time-addressed |
| HuggingFace datasets | Dataset card LICENSE missing or ambiguous | `LICENSE-AUDIT.md` per dataset; refuse to bundle if ambiguous |
| HuggingFace datasets | Public download rate limits | Mirror to GHCR-as-OCI-artifact for datasets with redistribution rights; fall back to upstream link otherwise |
| SWE-bench harness | `cache_level=env` default consumes 100 GB silently | Pre-flight disk check; expose `cache_level` as a documented knob in `make bench` help |
| SWE-bench harness | Default test discovery runs only PR-modified test files | Override to run module-scope tests; document the divergence from upstream in `bench/evaluators/swebench/README.md` |
| SWE-bench Verified | Treating as contamination-free | Still emit canary-probe baseline; Verified is *re-verified*, not *re-curated* |
| Multi-SWE-bench | License missing from HF dataset card in our research | Resolve before bundling; defer if unresolved |
| Aider Polyglot / Exercism | Bundling exercises without per-track license check | Audit each language track on Exercism; download-only if redistribution unclear |
| Terminal-Bench | Treating Big-Bench canary as enforced | It's symbolic; supplement with our own canary-probe baseline |
| CrossCodeEval | Assuming "minimal training-data overlap" claim still holds in 2026 | Re-run the canary-probe; trust-but-verify the upstream claim against modern frontier models |
| OpenTelemetry | Cross-process spans without trace_id propagation | Re-use Phase 65 `propagation.TraceContext{}` wiring; never merge by timestamp |
| SQLite (Phase 67 `internal/eval/`) | One DB across concurrent bench cells | One DB per (task × mode × run) sandbox; merge at report time |
| DuckDB (Phase 57 semantic store) | Concurrent writes from multiple daemons | Each daemon owns its own DuckDB path; bench harness asserts no shared `.duckdb` files across cells |
| Cosign sigstore | Treating bench mirror images as "internal" and skipping signing | Sign everything we publish from `agenthands/*` — including bench images. Reuses Phase 58 keyless flow |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Serial task execution | Bench takes 12 h instead of 2 h | Parallelize at the (task × mode × run) cell granularity; cap concurrency at `NumCPU()/2` | > 100 tasks per benchmark |
| Per-cell daemon cold-start | LS warm-up dominates wall_time_seconds | Pre-warm one daemon per language per benchmark; reuse across same-language tasks in a single run | Any multi-language benchmark |
| Per-task image pull | First task in each cell pays 5–30s pull tax | Pre-pull the entire dataset image manifest in `make bench-setup` | > 50 unique images |
| Logging to stdout from N concurrent daemons | Bench output is unreadable; stdout buffer pressure | Each daemon logs to its sandbox JSONL only; aggregator reads at report time | N > 10 concurrent cells |
| Loading entire dataset into memory | OOM on contributor laptops | Stream dataset rows; checkpoint progress to `bench-progress.jsonl` | Multi-SWE-bench (1632 tasks) |
| BCa bootstrap with N_resamples per task | Stats compute time exceeds bench compute time | Vectorize across tasks; sample resamples once per metric, not once per task-metric | > 500 tasks |
| Single shared `internal/eval/` SQLite | Lock contention across cells | Per-cell SQLite, merge at report time | N > 4 concurrent cells |
| Provider rate-limit hits → retry storm | 429s cascade; some cells finish, others stall | Token-bucket rate limiter shared across cells; per-mode budget cap | > 50 RPS sustained |
| Synchronous trace merge at end of run | 30-min merge phase after 2-h bench | Stream-merge as cells complete; report renders incrementally | > 1000 result rows |
| Bench-quick CI duration creep | PR CI takes > 15 min | Hard 5-min cap on `bench-quick`; CI fails on overrun | Anytime |

---

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Running benchmark tasks in shared sandbox | Task A's malicious test code reads task B's API keys | Per-cell sandbox HOME; per-cell secrets injection; `--network=none` for test phase |
| Provider API keys in clear text in result rows | Keys leak into published JSONL | Schema validator rejects rows containing `sk-ant-*`, `sk-*`, etc.; redact at emit |
| Bundling un-audited LICENSE files from external repos | License violation distributable | `LICENSE-AUDIT.md` per dataset; refuse to bundle if ambiguous |
| Running untrusted task code with daemon privileges | Task code escalates via daemon to host | Daemon runs in unprivileged container; task code in nested unprivileged container; rootless Docker / Podman |
| Cosign keyless signing of bench mirror images skipped | Future supply-chain attack on dataset reproducibility | Sign every image we publish via the existing Phase 58 keyless flow |
| Trusting upstream Docker Hub image content | Image swapped under us; bench results contaminated | Pin every image to sha256 digest, not tag; verify cosign signatures on every image where available |
| `claude` CLI persisting prompts to a shared dir | Future task's context contaminated by prior task's output | Fresh `HOME` per cell; `claude` CLI's local cache dir set to sandbox-scoped path |
| Dataset download over plain HTTP | MITM injection of poisoned tasks | HTTPS only; sha256 manifest per dataset; verify at download time |
| TOS attestation as a static file | Provider changes TOS; we keep claiming retention-zero | `make verify-tos` gate; 30-day expiration |
| Persisting full agent transcripts in published artifacts | PII / proprietary code in user prompts gets published | Redact transcripts at publish time; default to "metadata only" in `make bench` output |

---

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| `make bench` runs full $1K bench by default | Contributor's laptop melts; provider bill spikes | `make bench` requires `BENCH_FULL=1` env or interactive confirmation; default is `make bench-quick` |
| Bench output is a single huge JSONL | Reviewers can't find the headline | `leaderboard.md` + `per_language.md` + `ablations.md` + `cost_quality.md` (matches v1.12 spec); JSONL is the source, MD is the read path |
| Failed task is silently dropped from result aggregation | Pass rate over-states success | Failed tasks counted in denominator; `task_failure_reason` enum surfaces the cause |
| Per-task wall time and per-task cost not shown | Maintainer can't predict next run's cost | `make bench --dry-run` emits an estimated cost + wall time; refuses to run if estimate exceeds budget knob |
| Statistical significance reported without effect size | Reader can't tell if a "significant" result is meaningful | Always report effect size (Cohen's d or absolute delta) alongside p-value/CI |
| `your_agent_full` named as the brand position | Marketing leak into the technical artifact | Rename to `helix_full` or `helix_v1_12` in the schema before v1.12 ships externally |
| Multi-run report only shows the mean | Reader doesn't see N=3 was thin / variance was high | Report mean ± CI ± N alongside, in every table |
| No way to reproduce a single result row | Reviewer can't audit a suspicious row | Each result row carries `repro_cmd` (e.g. `helix bench rerun --task=X --mode=Y --seed=Z`) |
| Bench fails 80% through and loses progress | Maintainer re-runs the whole 12-hour thing | Per-cell checkpointing in `bench/results/<run>/`; `--resume` flag picks up where it left off |
| LLM-judge column placed adjacent to verified-correctness column | Reader conflates them | Schema and report column order: mechanical oracles in one block, informational signals in a clearly-labeled separate block |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **SWE-bench adapter:** Often missing the "run all tests, not just PR-modified" override — verify by checking the adapter's test-discovery hook against upstream defaults
- [ ] **Ablation mode `no_lsp`:** Often still exercising LSP via `analyze_blast_radius` / RepoMap enrichment / live-update — verify by asserting `helix_lspool_workers_started_total == 0` and zero `lsp.*` spans in trace
- [ ] **Ablation mode `no_semantic`:** Often still reading semantic store via strangler-fig fallback — verify by `helix_semantic_*` counter family is zero
- [ ] **`verified_correctness` metric:** Often defined as `tests_pass` (the F-09 tautology) — verify it requires ≥ 2 independent oracles
- [ ] **Token-counting:** Often using Helix's MCP-level counter for the headline — verify `tokens_to_model` is sourced from provider response `usage` block
- [ ] **Cached-input tokens:** Often bundled with regular input — verify `tokens_input_cached_read` exists as its own column
- [ ] **Cost table:** Often missing expiration — verify the YAML header has `valid_until`
- [ ] **Provider TOS attestation:** Often stale — verify `attested_at` is within 30 days at run time
- [ ] **Bootstrap CIs:** Often percentile not BCa — verify by reading the bootstrap method invocation
- [ ] **N_resamples:** Often ≤ 1000 — verify N ≥ 10,000 for headline numbers
- [ ] **Power analysis:** Often skipped — verify MDE is reported alongside N
- [ ] **Bench-quick budget:** Often unenforced — verify a CI step fails if `bench-quick` runtime > 5 min
- [ ] **Daemon-tap presence:** Often dropped in a new ablation profile — verify every result row has `daemon_tap_events_count > 0`
- [ ] **Model version pin:** Often floating — verify the pin is the dated snapshot form
- [ ] **Deprecation gate:** Often missing — verify runner refuses to start a run that would straddle a model's deprecation date
- [ ] **Per-language dep resolution:** Often network at task time — verify hermetic `--network=none` test phase
- [ ] **Image manifest:** Often unsigned — verify every image carries a cosign keyless signature
- [ ] **LICENSE audit:** Often missing per dataset — verify `LICENSE-AUDIT.md` exists with sha256 + redistribution decision
- [ ] **Canary-probe baseline:** Often skipped — verify `canary_pass_rate` column populated for each benchmark
- [ ] **Crosswalk:** Often missing — verify `bench/datasets/internal-toolbench/PHASE67_CROSSWALK.md` for any task that traces back to Phase 67
- [ ] **Fairness-contract:** Often loaded inconsistently — verify every mode's runner config is sourced from the single `fairness_contract.go` struct
- [ ] **Adversarial subset:** Often skipped — verify the 10-task must-refactor-across-files subset is included in the headline run
- [ ] **`make bench` cost guard:** Often missing — verify it errors without `BENCH_FULL=1` or an estimated-cost confirmation
- [ ] **Resumability:** Often missing — verify `--resume` skips already-completed cells

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Contamination evidence emerges post-publication | HIGH | Re-publish with contamination disclosure; revert headline claim to delta-only; canary baseline becomes a permanent column |
| False-positive patches discovered | HIGH | Re-score with differential oracle; publish corrected delta; document in a `v1.12-CORRIGENDUM.md` |
| Ablation leakage discovered | HIGH | Re-run the affected mode after disable flag fix; old results moved to `archive/`; published numbers re-derived |
| Fairness drift in a mode | HIGH | Re-run all modes through the corrected fairness contract; published headline updated |
| Token-counting boundary wrong | MEDIUM | Re-aggregate from raw provider-response data (cached in result rows); report regenerated; bench need not re-run |
| Cost-table drift | LOW | Refresh snapshot, re-derive `cost_per_solved_task` column at report time; no re-run needed |
| TOS drift detected post-run | MEDIUM | Disclose in the report's caveats section; re-attest under new TOS; future runs require fresh attestation |
| Disk-space explosion on contributor laptop | LOW | Document `cache_level=base` workaround; ship `make bench-prune` to reclaim |
| Dep-resolution non-hermeticity | MEDIUM | Re-run the affected language slice under `--network=none`; flag tasks that fail as "non-hermetic, excluded from headline" |
| Edit-locality gaming detected | MEDIUM | Re-derive `edit_locality_given_solved` as the headline; old raw column moves to informational |
| Bootstrap math error | LOW | Re-compute from raw result rows under BCa; published CIs corrected; bench need not re-run |
| Reproducibility delta between two runs | MEDIUM | Pin the model snapshot tighter; report headline as a 30-day window mean ± CI |
| Trace-merge race in concurrent cells | LOW | Re-merge from per-cell JSONLs (still on disk); switch merge to trace_id-based |
| License complaint | HIGH | Take down the affected dataset mirror immediately; re-audit; refuse to redistribute until resolved; community notice |
| Phase 67 corpus rename collision | MEDIUM | Crosswalk file becomes load-bearing; old-IDs query path through a translation table |
| Verified-correctness collapse to judge | MEDIUM | Strip judge contribution from the column; re-derive from mechanical oracles only |
| CI cost overrun | LOW | Lower smoke-bench cadence to weekly; tighten per-PR `bench-quick` budget |
| F-07-class regression in a new mode | LOW | Re-use the existing daemon-tap-integration-test pattern; add parametric coverage for the new mode |
| Fuzzy-cascade-credit detected | LOW | Re-run with `fuzzy.max_strategy: whitespace`; report headline footnote on the original |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Severity | Prevention Phase | Verification |
|---------|----------|------------------|--------------|
| 1 — Training-data contamination | BLOCKER | Dataset-acquisition phase | Canary-probe baseline column populated; per-dataset `CANARY.txt` exists |
| 2 — Patch-validation false positives | BLOCKER | SWE-bench adapter phase + metrics-layer phase | Differential oracle code committed; `verified_correctness ≠ tests_pass` test passes |
| 3 — Ablation-mode leakage | BLOCKER | Ablation-mode wiring phase | Escape-path audit tests pass for `no_lsp` / `no_semantic` / `no_structured_edit`; `vet-ablation-leakage` analyzer green |
| 4 — Same-model fairness drift | BLOCKER | Fairness-contract phase (must land before any benchmark adapter) | All modes load runner config from `fairness_contract.go`; CI contract test green |
| 5 — Token-counting attribution | HIGH | Metrics-layer phase | Schema requires both `tokens_to_model` (from provider `usage`) and `tokens_through_daemon` |
| 6 — Cost-table drift | HIGH | Cost-conversion phase | Pinned snapshot has `valid_until`; refuses to load past expiration |
| 7 — Provider TOS drift | HIGH | Provider-TOS phase | `make verify-tos` gate runs; refuses run if attestation > 30 days old |
| 8 — Container disk explosion | HIGH | Container-runtime phase | Pre-flight disk check refuses run if free disk < 2× estimated image set; GHCR mirror live |
| 9 — Per-language runner brittleness | HIGH | Per-language toolchain phase | Pre-baked images signed with cosign; `--network=none` enforced in test phase |
| 10 — Edit-locality gaming | HIGH | Metrics-layer phase | `edit_locality_given_solved` is the headline column; adversarial 10-task subset present |
| 11 — Bootstrap CI math | HIGH | Statistical-rigor phase | BCa implementation in `bench/evaluators/stats.go`; N ≥ 10,000 enforced |
| 12 — Reproducibility | HIGH | Reproducibility-harness phase | Per-(task,mode,run) sandbox; dated model snapshot; deprecation calendar gate |
| 13 — Trace-merging race | MEDIUM | Trace-merge phase | Per-cell JSONL paths; merge on `trace_id`; concurrent-cells regression test |
| 14 — Dataset license traps | MEDIUM | License-audit phase | `LICENSE-AUDIT.md` per dataset; `make verify-licenses` gate green |
| 15 — Phase 67 corpus migration | MEDIUM | Migration-plan phase | `eval/` frozen read-only; `bench/` net-new; `PHASE67_CROSSWALK.md` exists |
| 16 — LLM-judge signal collapse | MEDIUM | Verified-correctness phase | `verified_correctness` composed of mechanical oracles only; judge in `informational_quality_score` |
| 17 — CI cost | MEDIUM | CI-policy phase | `make bench-quick` ≤ 5 min on PR; `make bench` not run on PR; smoke-bench daily cap ≤ $20 |
| 18 — F-07-class daemon telemetry | MEDIUM | Profile-invariant phase + daemon-tap-regression phase | `TelemetryMiddleware` non-optional in bench profiles; F-07 test parametrized across modes |
| 19 — Fuzzy-cascade credit | LOW | Fuzzy-config phase | `fuzzy.max_strategy: whitespace` configurable; per-mode `fuzzy_edits_count` reported |

---

## Sources

Phase 67 internal artifacts (load-bearing):
- `.planning/v1.10-MILESTONE-AUDIT.md` — F-01 / F-05 / F-07 / F-08 / F-09 / F-10 findings + resolution commits
- `.planning/PROJECT.md` — v1.12 milestone scope, Phase 67 EVAL-04/EVAL-07 history, Phase 65 strangler-fig wiring, Phase 60 live update pipeline OnEdit hooks
- `.planning/milestones/v1.10-ROADMAP.md` — Phase 67 evaluation-harness scope (hand-rolled binary, 4-mode sandbox, trace tap, scorer DSL, 5 reporters)

Public-benchmark / external research (HIGH confidence):
- ["Are 'Solved Issues' in SWE-bench Really Solved Correctly?" (ICSE 2026)](https://software-lab.org/publications/icse2026_SWE-bench-correctness.pdf) — patch false-positive rate, UTBoost, differential test mitigation
- [Introducing SWE-bench Verified — OpenAI](https://openai.com/index/introducing-swe-bench-verified/) — human re-verification rationale
- [SWE-bench Verified Explained — benchmarkingagents.com](https://benchmarkingagents.com/swe-bench/) — methodology / caveats
- [How to run SWE-bench Verified in one hour — Epoch AI](https://epoch.ai/blog/swebench-docker) — disk and image-cache cost
- [How We Made SWE-Bench 50x Smaller — logicstar.ai](https://logicstar.ai/blog/how-we-made-swe-bench-50x-smaller) — 684 GiB unoptimized → 67 GiB optimized
- [Aider-AI/polyglot-benchmark — Aider](https://github.com/Aider-AI/polyglot-benchmark) — 225 Exercism exercises, per-track licensing
- [Aider Polyglot Leaderboard — llm-stats.com](https://llm-stats.com/benchmarks/aider-polyglot) — published methodology
- [polyglot-benchmark README](https://github.com/Aider-AI/polyglot-benchmark/blob/main/README.md) — Exercism license note
- [CrossCodeEval — crosscodeeval.github.io](https://crosscodeeval.github.io/) — Python/Java/TS/C# permissively-licensed source repos
- [CROSSCODEEVAL paper (NeurIPS 2023)](https://arxiv.org/pdf/2310.11248) — minimal training-data overlap claim, must be re-verified
- [Multi-SWE-bench: A Multilingual Benchmark for Issue Resolving](https://arxiv.org/pdf/2504.02605) — 1,632 instances, 7 languages, license not surfaced on HF card
- [ByteDance-Seed/Multi-SWE-bench — HuggingFace](https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench)
- [Terminal-Bench (ICLR 2026 paper)](https://openreview.net/pdf/417ac3236de7dbf3fc3414c51754dd239271663e.pdf) — Apache-2.0 dataset, Big-Bench canary string, contamination disclosure
- [Terminal-Bench 2.0 Verified — Hugging Face](https://huggingface.co/datasets/zai-org/terminal-bench-2-verified)
- [AI Provider Pricing Change Monitoring — PageCrawl.io](https://pagecrawl.io/blog/ai-provider-pricing-change-monitoring-openai-anthropic-google) — pricing-table drift evidence ("the cuts often arrive without an email")
- [LLM API Pricing Comparison 2026 — IntuitionLabs](https://intuitionlabs.ai/articles/llm-api-pricing-comparison-2025) — provider deprecation dates (Opus 4 retires 2026-06-15)
- [LLM API Cost Comparison — costgoat.com](https://costgoat.com/compare/llm-api) — cached-input pricing
- [BCa bootstrap (Efron) — sas blog](https://blogs.sas.com/content/iml/2017/07/12/bootstrap-bca-interval.html) — bias-corrected accelerated bootstrap rationale
- [BCa implementation — Sebastian Schöner blog](https://blog.s-schoener.com/2020-07-12-bootstrap/) — jackknife acceleration term implementation
- [arch.bootstrap confidence intervals](https://arch.readthedocs.io/en/latest/bootstrap/confidence-intervals.html) — N_resamples > 1/alpha rule of thumb

Personal / project experience (HIGH confidence within the codebase):
- Phase 48 / Phase 56 jdtls warm-cache lesson (CLAUDE.md, v1.9 audit) — applies directly to Pitfall 9 multi-language toolchain pre-warm
- Phase 51.1 / Phase 59.1 CGO build-mode lessons — applies indirectly to Pitfall 8 container reproducibility
- Phase 58 cosign keyless infrastructure (REL-01) — directly reusable for Pitfall 8 (image mirror) and Pitfall 14 (dataset distribution)
- Phase 60 EditNotifier OnEdit hooks (LIVE-01..LIVE-07) — directly relevant to Pitfall 3 (ablation leakage via live update pipeline)
- Phase 65 strangler-fig `lspProbeForEdges` (INTEG-01..05) — directly relevant to Pitfall 3 (ablation leakage via `analyze_blast_radius`)
- Phase 67 F-09 DiagnosticsClean tautology (resolved by commit `93426cc4`) — directly relevant to Pitfall 2 (multi-oracle `verified_correctness`)
- Phase 67 F-07 daemon-tap regression (resolved by commits `e7bc406f` + `ab8ad6cb`) — directly relevant to Pitfall 18 (parametric F-07 test across modes)

---
*Pitfalls research for: v1.12 Bench Stack & Tool Evaluation*
*Researched: 2026-06-13*
