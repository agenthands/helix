# Feature Research

**Domain:** Dev-time agent harness + task-success-driven prompt optimization (DeepSeek/OpenAI tool-using agent driving the `helix` CLI; GEPA metric rewire from `choice_rate` → real benchmark task-success; human-gated SKILL.md adoption)
**Researched:** 2026-06-24
**Confidence:** HIGH (existing harness read directly; SWE-bench/DeepSeek behavior cross-checked against current sources; v2.2 no-ship lessons read from 106-VERIFICATION.md / 106-02-SUMMARY.md / optimize.py / scorer.py)

> Scope note: this file covers ONLY the three net-new v2.3 capabilities. It deliberately does NOT re-research the already-built reuse surface: `tools/dspy-tune/` (GEPA harness + parity-pinned `scorer.py`), `bench/datasets/aider-polyglot/` (loader), `bench/runtime/aider_edit_*.go` (edit harness), `bench/runtime/subprocess/claude.go` + `internal/eval/agent` (Claude-Code-only agent), `bench/container` (podman-aware), `test/oracle/adopt` (scorecard). Those are dependencies, called out per-feature.

## Feature Landscape

### Table Stakes (Users Expect These)

Features without which the milestone is not honest — a v2.2-class no-ship would repeat.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **OpenAI-compatible tool-calling agent loop** driving `helix <verb>` via subprocess | The whole milestone premise: measure *real* helix use, not a first-command proxy. DeepSeek + OpenAI both speak the OpenAI `tools=[...]` / `tool_calls` schema, so one client wraps both (DeepSeek base URL + model, OpenAI fallback) | MEDIUM | Standard ReAct/tool-call loop: system prompt = the SKILL.md/steering text *under test*; tools = a `run_shell(cmd)` (or per-verb `helix_*`) tool that shells out + a `submit`/`done` tool; observe stdout/stderr/exit, append as `tool` role message, iterate. Reuses NOTHING from `claude.go` (that delegates to the `claude` binary's own loop — the new agent owns the loop itself). Depends on: `DEEPSEEK_API_KEY` primary, `OPENAI_API_KEY` fallback (`ANTHROPIC_API_KEY` unset). Lives dev-time/offline — **must not** enter `go.mod`/the binary/`go test ./...` (toolsquarantine boundary, carried from Phase 106). |
| **Hard turn/iteration + token/wall-clock budget** with deterministic termination | Without a cap the loop runs forever / burns API spend on a stuck task; needed for cost predictability across a corpus. `claude.go` already models this as `MaxToolCalls` → `--max-turns` | LOW | Terminate on: agent emits `submit`/`done`, OR max-iterations hit, OR repeated identical tool call (loop detection). Record termination reason in the trace. Mirror the existing `BudgetParams{MaxToolCalls}` axis so benches stay comparable. |
| **Per-task sandboxed working copy** (one repo checkout per task, reset between tasks) | Tasks mutate files; cross-task contamination silently inflates/destroys success. The aider/SWE-bench oracles assume a pristine-then-patched tree | MEDIUM | Aider polyglot: reuse the `bench/datasets/aider-polyglot` loader's per-exercise SrcDir + pristine-test restore (the `aider_edit` harness already does the WR-01 test restore — **never** let the agent edit `files.test`). SWE-bench: per-instance container via `bench/container` (podman-aware; do NOT report "Docker blocked"). |
| **Aider-polyglot pass/fail oracle = hidden unit tests green after edits** | Canonical aider definition of success; the loader already carries solution stubs + `.meta/example` reference + the test files | MEDIUM | Run the exercise's own test command after the agent's edits; pass = exit 0 / all tests green. The agent must NOT see or edit the test files (anti-tamper) — the harness restores pristine tests before scoring. Reuse the existing exercise model; add the *run-tests-and-read-result* step the deterministic edit agent skips. |
| **SWE-bench pass/fail oracle = FAIL_TO_PASS flip + PASS_TO_PASS no-regression** | The de-facto SWE-bench resolution definition: apply the agent's diff, then *every* FAIL_TO_PASS test must flip fail→pass AND *every* PASS_TO_PASS test must stay green (no new failures). A patch that only passes F2P but regresses P2P is NOT resolved | HIGH | Run inside the per-instance image (correct deps/commit). Easiest path: point the upstream `swebench` Python harness at Podman's docker-compatible socket (`DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock`, `podman system service --time=0 &`) — configuration, not a blocker. Parse the harness's per-test report; do NOT re-implement the test-status diffing from scratch if the upstream harness already emits it. |
| **Held-out TEST split sequestered from the optimizer** | The Phase 106 guard that survived the no-ship: `optimize.py` draws train/val ONLY from `train.jsonl`; `test.jsonl` is loaded for the final report and NEVER passed to `compile()`. v2.3 keeps this, now over real tasks | LOW | Already implemented for the proxy corpus; extend the same train/val/held-out-TEST discipline to the Aider/SWE-bench task lists. `test_split.py` disjointness gate is the regression guard. **This is the overfitting defense — non-negotiable per the quality gate.** |
| **Metric-gaming defense baked into the metric itself** | The literal v2.2 no-ship cause: `choice_rate` rewards "always say helix" and a substring `"helix" in response` check inflates it. A task-success oracle is *intrinsically* harder to game (you can't fake a green test suite), but the harness must still refuse degenerate steering | MEDIUM | Task-success removes the "always emit helix" exploit (emitting a verb that doesn't solve the task scores 0). Keep `test_degenerate.py`-style flag/no-flag pairs for the *steering text* itself. Add: refuse to count a task "passed" unless tests actually ran (guard against "no tests collected → vacuous pass"). Every gate ships a break-the-invariant → assert-RED test (carried discipline). |
| **Honest baseline / control arm: helix-steering ON vs OFF (and vs grep-sed-cat)** | An improvement is only attributable to the *skill text* if the same agent, same tasks, same budget runs with steering vs without. Otherwise you're measuring the model, not the skill | MEDIUM | A/B the system prompt only: arm A = SKILL.md steering injected; arm B = no steering (or a plain "you have a shell" prompt) / grep-sed-cat-only toolset. Report Δsuccess with the corpus sizes. This is the difference between "we tuned a prompt" and "we proved the prompt helps." Pairs directly with the `test/oracle/adopt` philosophy. |
| **Trace/transcript capture per run** (prompt, every tool call + observation, exit, score) | Required to debug why a task failed, to audit metric-gaming, and to feed GEPA's reflective feedback | LOW | `claude.go`/`internal/eval/agent` already capture stdout/stderr under the cell mode dir — mirror the layout. The transcript is also the artifact a human reviews before adopting steering. |
| **Human-gated adoption via `cmd/helix-refgen` + `helix-refgen --check`; no auto-write** | Carried v2.2 invariant: `optimize.py` writes only git-ignored `output/optimized.json`; adopted text re-enters ONLY via a human-reviewed SKILL.md edit / refgen override that passes `helix-refgen --check`. `reference.md` stays generated-not-hand-edited | LOW | Already enforced (Phase 106 truth #10: `grep skills/helix\|reference.md optimize.py` = 0). v2.3 keeps it verbatim. The optimizer NEVER touches the embedded bundle. |
| **No-runtime-Python / single-binary invariant preserved** | The product's defining constraint; DSPy + the new agent are strictly dev-time. The `toolsquarantine` analyzer mechanically guards it from `make vet` | LOW | The new agent can be Python (alongside DSPy under `tools/`) OR a quarantined Go tool excluded from the binary — but it MUST stay off `go.mod`/`helix setup`/`go test ./...`/the merge path. Python-under-`tools/` is the lower-friction path that matches the existing DSPy quarantine exactly; Go-under-`tools/` would need the analyzer to also exclude it from the build. |

### Differentiators (Competitive Advantage)

Features that make the result genuinely informative, beyond a bare pass/fail number.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Dual-bench task-success metric (Aider polyglot + SWE-bench) joined into one GEPA signal** | Aider is fast/cheap/multi-language and stresses *edit correctness*; SWE-bench stresses *real repo-scale localization + regression safety*. Optimizing against both resists overfitting to one task shape | HIGH | Define the metric as a weighted/normalized success rate across both corpora (or run them as separate optimization targets and report both). Aider gives high-N cheap signal; SWE-bench gives low-N expensive ground truth. Start Aider-only to de-risk, add SWE-bench once the loop is stable. |
| **GEPA reflective feedback keyed on *why* the task failed** | GEPA evolves the prompt using natural-language feedback, not just a scalar. "Agent grepped instead of `find-references`, missed call site X, F2P test Y stayed red" is far more steering-useful than `score=0` | MEDIUM | The metric returns `dspy.Prediction(score=, feedback=)` (already the shape in `optimize.py`). Upgrade `feedback` from the current one-liner to a task-success diagnosis (which tests failed, did the agent use symbolic verbs). Depends on the trace capture. |
| **Steering-attribution report (Δ ON-vs-OFF) as the ship/no-ship gate** | Turns the milestone deliverable from "a tuned prompt" into "evidence the skill text moves task-success by N% on real benches" — the thing v2.2 could not produce | MEDIUM | REPORT.md analog: per-bench success for ON / OFF / grep-baseline arms, corpus sizes, and the held-out-TEST delta. A *positive, significant* held-out delta is ship; otherwise a documented no-ship (still legitimate). |
| **Cost/latency accounting per arm** | Real agentic runs cost money + time; reporting tokens + wall-clock per task per arm makes the spend auditable and prevents "we ran 5 tasks" hand-waving | LOW | Sum API usage from the OpenAI-compatible response; record per-run. Also surfaces when a corpus is too small to trust (the MinTasks=5 lesson). |

### Anti-Features (Commonly Requested, Often Problematic)

Features that look like progress but reintroduce the v2.2 failure mode or break invariants.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Keep `choice_rate` as the (co-)optimization target** | It's already wired, parity-pinned, and cheap (no test execution) | It is *the* gameable proxy the milestone exists to kill ("always say helix"); co-optimizing it lets the optimizer chase the cheap signal and re-inflate the no-ship | Use task-success as the optimization signal. `choice_rate` may stay as a *diagnostic* (did the agent even use helix?) but MUST NOT be the reward. |
| **Auto-adopt the optimizer's best prompt into SKILL.md** | "Close the loop" / save a manual step | Breaks the human-review re-entry gate; lets an overfit/degenerate prompt ship; `reference.md` is generated-not-hand-edited | Optimizer writes git-ignored `output/` only; human transcribes into SKILL.md and runs `helix-refgen --check`. (Already the invariant — do not relax it.) |
| **Tiny corpus (MinTasks≈5) "to move fast"** | Fast iteration, low API cost | The exact documented v2.2 no-ship cause — too small to distinguish skill effect from noise; held-out delta is meaningless | Size train/val/held-out splits so the held-out delta is interpretable (TUNE-FUT-01 flagged `val_size>50`). Aider polyglot's high task count makes this affordable; report N and treat under-powered runs as no-ship. |
| **Let the agent see / edit the hidden test files** | "It would solve more tasks" | Destroys the oracle — the agent games the test instead of fixing the code; success becomes meaningless | Anti-tamper: restore pristine tests before scoring (Aider harness already does this for `files.test`); SWE-bench applies the test patch separately from the model patch. |
| **Spawn the new agent through `bench/runtime/subprocess/claude.go`** | "We already have an agent runner" | `claude.go` delegates to the `claude` binary's *own* internal loop over MCP, and `ANTHROPIC_API_KEY` is unset — wrong transport (MCP, not CLI verbs) and wrong provider | Build a net-new agent that owns its ReAct loop and shells `helix <verb>` directly (the product surface). Reuse only the sandbox/trace *layout* conventions, not the claude code path. |
| **Use only the *easiest* SWE-bench split or a single instance** | Faster, fits in budget | A non-representative slice produces a delta that doesn't generalize; the rigorous-evaluation literature flags exactly this | Use a documented, fixed instance list (e.g. a SWE-bench-Verified subset) with the split frozen and reported; never silently change which instances are scored between arms. |
| **Re-implement the SWE-bench test-status diffing in-house** | Avoid the Python harness dependency | Re-deriving FAIL_TO_PASS/PASS_TO_PASS resolution is exactly where subtle scoring bugs (and false-green) hide | Drive the upstream `swebench` harness via Podman's docker socket; parse its emitted per-test report. `bench/container` already auto-detects podman. |
| **Single provider hard-coded (DeepSeek only)** | Simpler client | A DeepSeek outage / rate-limit stalls the whole optimization; the milestone explicitly wants OpenAI fallback | One OpenAI-compatible client, DeepSeek base URL primary + OpenAI fallback on error/unset-key, model selected via env (mirrors `optimize.py`'s `DSPY_LM_MODEL` env pattern). |

## Feature Dependencies

```
[OpenAI-compatible tool-calling agent loop]   <-- foundational; nothing else works without it
    |--requires--> [per-task sandboxed working copy]
    |--requires--> [turn/budget cap + deterministic termination]
    |--produces--> [trace/transcript capture]
    |
    +--feeds--> [Aider pass/fail oracle (hidden tests green)]
    +--feeds--> [SWE-bench pass/fail oracle (F2P flip + P2P no-regression)]
                     |--requires--> [Podman socket / bench/container per-instance image]
                     |
   [Aider oracle] + [SWE-bench oracle]
       |--define--> [task-success metric]   <-- REPLACES choice_rate as GEPA reward
                        |--requires--> [held-out TEST split sequestered]      (overfit defense)
                        |--requires--> [metric-gaming / vacuous-pass refusal]  (anti-vacuity)
                        |--enhanced-by--> [GEPA reflective failure feedback]
                        |
                        +--scored against--> [baseline control arm: steering ON vs OFF vs grep-baseline]
                                                 |--produces--> [steering-attribution Δ report] = ship/no-ship gate
                                                                    |--gated-by--> [human adoption via helix-refgen --check]

[no-runtime-Python / toolsquarantine boundary] --constrains--> ALL of the above (dev-time only)
[choice_rate as reward] --conflicts--> [task-success metric]  (do not co-optimize)
```

### Dependency Notes

- **Agent loop requires sandboxed working copy:** tasks mutate files; without per-task reset, cross-task contamination corrupts every downstream score.
- **Task-success metric requires the held-out split + vacuity refusal:** these two are the literal defenses against the v2.2 no-ship; the metric is only trustworthy with both in place.
- **task-success metric conflicts with choice_rate-as-reward:** co-optimizing the gameable proxy re-opens the exploit the milestone closes.
- **SWE-bench oracle requires Podman config, NOT new infra:** `bench/container` auto-detects podman; the only setup is pointing the upstream harness at the podman docker socket.
- **Steering-attribution Δ requires the baseline control arm:** without ON-vs-OFF on identical tasks/budget, an improvement is unattributable.

## MVP Definition

### Launch With (v1 of this milestone)

Minimum to produce an *honest, attributable* task-success signal — not necessarily a positive one.

- [ ] OpenAI-compatible tool-calling agent loop (DeepSeek primary, OpenAI fallback) shelling `helix <verb>` — foundational
- [ ] Turn/budget cap + deterministic termination + trace capture — without these the loop is unsafe to run
- [ ] Per-task sandbox + Aider-polyglot pass/fail oracle (hidden tests green; anti-tamper test restore) — cheapest real success signal
- [ ] Task-success wired as the GEPA metric replacing `choice_rate`; held-out TEST split kept; vacuous-pass + degenerate-steering refusal — the core rewire + carried defenses
- [ ] Baseline control arm (steering ON vs OFF) + attribution Δ report — turns the run into evidence
- [ ] Human-gated adoption via `helix-refgen --check`; no auto-write; toolsquarantine boundary green — carried invariants

### Add After Validation (v1.x)

- [ ] SWE-bench oracle (F2P flip + P2P no-regression) via Podman — adds repo-scale ground truth once the Aider loop is stable (trigger: Aider arm produces a clean, reproducible Δ)
- [ ] grep-sed-cat-only baseline arm as a third control — sharpens the attribution (trigger: ON-vs-OFF Δ exists and you want to isolate "helix vs no-tools")
- [ ] GEPA reflective failure feedback upgraded from one-liner to test-level diagnosis — improves convergence (trigger: first optimization run shows weak signal)
- [ ] Dual-bench joined metric + per-arm cost/latency accounting

### Future Consideration (v2+)

- [ ] Larger SWE-bench instance lists / multi-language Aider expansion — defer until the single-bench loop reliably attributes (TUNE-FUT-01 corpus-size gating)
- [ ] Additional providers beyond DeepSeek/OpenAI — defer; two OpenAI-compatible providers cover the milestone

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| OpenAI-compatible agent loop driving `helix` CLI | HIGH | MEDIUM | P1 |
| Turn/budget cap + termination + trace | HIGH | LOW | P1 |
| Per-task sandbox + Aider pass/fail oracle | HIGH | MEDIUM | P1 |
| Task-success as GEPA metric (replace choice_rate) | HIGH | MEDIUM | P1 |
| Held-out split + vacuous/degenerate refusal (carried) | HIGH | LOW | P1 |
| Baseline control arm (ON vs OFF) + Δ report | HIGH | MEDIUM | P1 |
| Human-gated adoption via helix-refgen (carried) | HIGH | LOW | P1 |
| toolsquarantine / no-runtime-Python (carried) | HIGH | LOW | P1 |
| SWE-bench oracle via Podman | HIGH | HIGH | P2 |
| GEPA reflective failure feedback | MEDIUM | MEDIUM | P2 |
| grep-baseline third arm | MEDIUM | LOW | P2 |
| Dual-bench joined metric + cost accounting | MEDIUM | MEDIUM | P2/P3 |

**Priority key:** P1 = must have for the milestone to be honest; P2 = should have, add when the P1 loop is stable; P3 = nice to have.

## Competitor Feature Analysis

| Feature | SWE-bench official harness | Aider benchmark | Our Approach |
|---------|----------------------------|-----------------|--------------|
| Success oracle | F2P flip + P2P no-regression in per-instance container | Hidden unit tests pass after edits | Both — Aider first (cheap/high-N), SWE-bench second (ground truth) via the upstream harness on Podman |
| Agent transport | Whatever the submitter ships (patch only) | aider's own LLM edit loop | Net-new OpenAI-compatible loop shelling `helix <verb>` — the product surface under test |
| Anti-tamper | Test patch applied separately from model patch | Pristine tests restored before scoring | Reuse Aider loader's pristine-test restore; for SWE-bench let the harness apply the test patch |
| Attribution | N/A (absolute leaderboard) | N/A | Steering ON-vs-OFF control arm — the differentiator, since we measure a *prompt's* effect, not a model's |

## Sources

- [SWE-bench dataset (FAIL_TO_PASS / PASS_TO_PASS semantics)](https://huggingface.co/datasets/SWE-bench/SWE-bench)
- [Rigorous Evaluation of Coding Agents on SWE-Bench (ACL 2025) — patch-apply + no-regression discipline](https://aclanthology.org/2025.acl-long.189.pdf)
- [DeepSeek API — Function Calling (OpenAI-compatible tool use)](https://api-docs.deepseek.com/guides/function_calling)
- [DeepSeek API docs — OpenAI-compatible base URL / multi-turn agent loops](https://api-docs.deepseek.com/)
- Internal: `tools/dspy-tune/{optimize.py,scorer.py}` (current GEPA harness + parity-pinned classifier), `bench/runtime/subprocess/claude.go` + `internal/eval/agent/` (claude-only agent shape), `bench/runtime/aider_edit_agent.go` + `bench/datasets/aider-polyglot/loader.go` (Aider harness + loader), `.planning/milestones/v2.2-phases/106-*` (the documented choice_rate no-ship + carried defenses), CLAUDE.md (Podman/`bench/container` auto-detect, no-runtime-Python invariant)

---
*Feature research for: v2.3 Task-Success-Driven Skill Optimization*
*Researched: 2026-06-24*
