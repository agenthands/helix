# Project Research Summary

**Project:** Helix
**Milestone:** v2.3 — Task-Success-Driven Skill Optimization
**Domain:** Dev-time/offline LLM optimization tooling (a DeepSeek/OpenAI tool-using agent driving the `helix` CLI; GEPA metric rewired from a gameable `choice_rate` proxy to real coding-benchmark task-success) bolted onto a shipped Go single-binary product without breaching the no-runtime-Python boundary.
**Researched:** 2026-06-24
**Confidence:** HIGH

## Executive Summary

v2.3 exists to fix a specific, documented failure: v2.2 concluded **no-ship** because its skill-tuning loop optimized a *gameable* proxy (`choice_rate` — "always emit `helix` first," inflated by a `"helix" in response` substring check) over a *tiny corpus* (`MinTasks=5`; the real split was TRAIN=8 / TEST=3). The four researchers converged tightly on the fix: replace the proxy reward with **real benchmark task-success** (Aider polyglot hidden-tests-green; SWE-bench FAIL_TO_PASS-flip + PASS_TO_PASS-no-regression), measured by a **net-new OpenAI-compatible tool-using agent** that drives `helix <verb>` via subprocess — the actual product surface — and attribute any gain to the skill text via a **mandatory ON-vs-OFF control arm** (`delta = success(ON) − success(OFF)`).

The cross-cutting architectural convergence is unanimous and load-bearing: the new agent + metric live **in Python under `tools/dspy-tune/agent/`** (a sibling of the existing `scorer.py`), NOT in Go `bench/runtime`. GEPA calls its metric **in-process per candidate** thousands of times, so the agent must be a plain Python import the metric can call; putting an LLM client in Go `bench/runtime` would add a forbidden runtime-side `go.mod` dep and force a process-spawn handoff into the optimizer's hot loop. A **single `openai==2.43.0` client** wraps both providers (DeepSeek primary via `base_url=https://api.deepseek.com`, OpenAI fallback) — DeepSeek is OpenAI-compatible. **Zero new Go deps**; the `toolsquarantine` analyzer needs no change (it is import-only and all new edges are Python/`subprocess`, inside the exempt `tools/` prefix). Pins: keep `dspy==3.2.1`, add `openai==2.43.0` + `swebench==4.1.0` to the dev venv only.

The highest-risk work is making the **task-success oracle honest**. A task-success metric removes the always-`helix` exploit but opens new gaming surfaces that silently inflate the delta: **vacuous passes** (0 tests actually ran → benign default coerced to "pass" — the exact Phase 81 false-green shape), **pass-without-using-helix** (agent solves with `cat`/`sed`, skill gets credit), and **in-sandbox test tampering** (agent edits the failing test). Mitigation is a **fail-not-skip, three-part oracle**: assert N>0 tests ran (hard ERROR otherwise), enforce the FAIL_TO_PASS/PASS_TO_PASS contract, and restore/`git checkout` gold tests from an agent-unwritable path before grading. The deeper overfitting fix is the **TUNE-FUT-01 `val_size>50` gate** (hard precondition for adoption, not a footnote) plus a **sequestered held-out TEST split** never passed to `compile()`. Every new gate ships a break-the-invariant → assert-RED test, with code-review+fix folded BEFORE verify (carrying the v2.2 mutation-test discipline that caught a tautological `x==x` Guard B and a real Python↔Go parity bug). SWE-bench is **NOT blocked** — `podman system service --time=0 &` + `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` (configuration, not a blocker); watch the dataset-org drift (`princeton-nlp/` datasets vs the `SWE-bench/` repo org).

## Key Findings

### Recommended Stack

All net-new dependencies are **dev-time Python in the `tools/dspy-tune/` venv only** — never `go.mod`, the binary, `helix setup`, or `go test ./...`. The Go side reuses existing assets (aider-polyglot loader, swebench harness wrapper, `bench/container` podman detect) with no new module deps. (See STACK.md.)

**Core technologies:**
- **DSPy `3.2.1`** (keep the existing pin): GEPA optimizer + `dspy.LM`. Metric signature `metric(gold,pred,trace,pred_name,pred_trace)->dspy.Prediction(score=,feedback=)` is stable 3.1→3.2; only the metric *body* changes.
- **OpenAI SDK `openai==2.43.0`** (NEW): the single tool-use client for BOTH DeepSeek (OpenAI-compatible `base_url`) primary and OpenAI fallback. Drives the agentic `tools=[...]`/`tool_calls` loop.
- **swebench `4.1.0`** (NEW pin): the upstream `python -m swebench.harness.run_evaluation` harness; pin is **mandatory** due to dataset-org drift.
- **Python 3.13** (host 3.13.5; the `>=3.10,<3.15` intersection of all three pins).
- **Podman 5.4.2** + `podman system service` socket via `DOCKER_HOST` for the SWE-bench arm. Per-language Aider toolchains (go/python/node/cargo/g++/openjdk via checked-in gradle wrapper) all verified present.

**Critical version note:** DeepSeek `deepseek-chat`/`-reasoner` aliases **retire 2026-07-24** (→ `deepseek-v4-flash`/`-pro`). Make the model a config var (`DSPY_LM_MODEL` precedent); prefer the explicit `deepseek-v4-*` id.

### Expected Features

(See FEATURES.md.)

**Must have (table stakes — without these the milestone repeats a v2.2-class no-ship):**
- OpenAI-compatible tool-calling agent loop driving `helix <verb>` via subprocess (DeepSeek primary, OpenAI fallback, one client)
- Hard turn/budget cap + deterministic termination + per-run trace/transcript capture
- Per-task sandboxed working copy (pristine-then-patched, reset between tasks)
- Aider-polyglot pass/fail oracle (hidden tests green; anti-tamper test restore)
- Task-success wired as the GEPA metric **replacing** `choice_rate`; sequestered held-out TEST split; vacuous-pass + degenerate-steering refusal
- **Mandatory ON-vs-OFF control arm** + attribution Δ report
- Human-gated adoption via `helix-refgen --check`; no auto-write; `toolsquarantine` green; single-binary / no-runtime-Python invariant preserved

**Should have (competitive):**
- SWE-bench oracle (F2P flip + P2P no-regression) via Podman — add once the Aider loop is stable
- GEPA reflective feedback keyed on *why* a task failed (test-level diagnosis)
- Steering-attribution (Δ ON-vs-OFF on held-out TEST) as the explicit ship/no-ship gate; per-arm cost/latency accounting

**Defer (v2+):**
- grep-sed-cat-only third control arm; dual-bench joined metric; larger SWE-bench instance lists / more providers

### Architecture Approach

The new agent and metric are **sibling modules inside `tools/dspy-tune/`** (NOT a new `tools/agent/`), because GEPA invokes the metric in-process exactly as it imports `score_choice_rate` today. The pattern is **in-process metric, subprocess everything-the-LLM-touches**: the metric calls the agent as a Python function; the agent shells `helix <verb>`; the graders shell the per-language test command (Aider) or the upstream swebench harness (SWE-bench). The candidate SKILL text reaches the agent as its **system prompt**; the winning text re-enters the shipped binary ONLY via a human-reviewed SKILL.md edit gated by `helix-refgen --check`. (See ARCHITECTURE.md.)

**Major components:**
1. **ReAct agent** (`tools/dspy-tune/agent/` — `react.py`, `llm.py`, `tools.py`) — DeepSeek/OpenAI loop emitting `helix <verb>` argv; system prompt = candidate skill text.
2. **Task-success metric** (`taskmetric.py` + `optimize.py` body swap) — runs agent per task, grades, returns `dspy.Prediction(score, feedback)`; replaces `score_choice_rate`.
3. **Graders** (`grade_aider.py` parity-pinned Python mirror of `NativeTestCommand`; `grade_swebench.py` direct call to the upstream harness via Podman) + `sandbox.py` per-task isolated work dir.
4. **Unchanged guards:** `toolsquarantine` analyzer (no new Go edge), `helix-refgen --check` adoption gate, `scorer.py` (retained as an optional pre-screen, NOT the reward).

### Critical Pitfalls

(Top 5 of 9 from PITFALLS.md.)

1. **Reward-hacking the task-success oracle** — fail-not-skip three-part oracle: assert N>0 tests ran (vacuous pass = hard ERROR), enforce FAIL_TO_PASS/PASS_TO_PASS, restore gold tests from an agent-unwritable path + reject test-file diffs. Plus a "did the agent actually call helix?" attribution check.
2. **Tiny-corpus overfit (the literal v2.2 no-ship cause)** — `val_size>50` as a HARD adoption gate; held-out TEST sequestered first and never passed to `compile()`; `test_split.py` RED-fails on a planted leak.
3. **Attribution failure (gain came from the LM, not the skill)** — mandatory ON-vs-OFF control arm, identical except steering text; `delta = ON − OFF`; OFF prompt provably omits the steering.
4. **Gating the optimizer process instead of the committed artifact** — gate `reference.md`/`SKILL.md` via `helix-refgen --check`, never re-run `optimize.py` in CI; unset-key exits 0 for hermetic gates but fails loudly on a real run.
5. **SWE-bench/Podman/dataset drift** — never "Docker → blocked" (auto-detected Podman); pin dataset ids to the correct org; assert fetch resolves and task-count > 0.

Cross-cutting: **anti-vacuity** (every new gate ships a break-the-invariant → assert-RED test; review+fix BEFORE verify) and the **HELIX_BIN fail-not-skip** lesson.

## Implications for Roadmap

Dependency chain is forced — **agent → grader+metric rewire → SWE-bench → adoption gate**. Phase numbering continues from 106.

### Phase 107: ReAct agent + LLM client (`tools/dspy-tune/agent/`)
**Rationale:** Metric, graders, and adoption gate all depend on it; standalone, no benchmark coupling.
**Delivers:** `llm.py` (DeepSeek-primary/OpenAI-fallback, model as config var, tool-call JSON validation, max-turns + no-progress cap), `react.py`, `tools.py` (subprocess `helix <verb>`), first-class `--steering on|off`.
**Uses:** `openai==2.43.0` in `requirements.txt` only.
**Avoids:** Pitfall 7 (pin `deepseek-v4-*`, exercise fallback, fail-loud on missing key for real runs); Pitfall 3 (ON/OFF from start); Pitfall 5 (`make vet` green, no `go.mod` diff).
**Anti-vacuity gate:** hermetic `test_agent.py` (fake LLM + fake `helix`, no network); degenerate always-grep agent scores 0; OFF prompt provably omits steering.

### Phase 108: Aider grader + sandbox + metric rewire
**Rationale:** Depends on 107; Aider needs no Podman → cheaper, hermetic-test-friendly grader before SWE-bench. Core `choice_rate → task-success` swap.
**Delivers:** Python mirror of `NativeTestCommand` + WR-01 restore; per-task sandbox; `taskmetric.py`; `optimize.py` body swap; shared `golden/aider_native_cmd.json` parity corpus asserted by a Go `*_test.go` (under `bench/datasets/aider-polyglot/`, NOT `tools/`) and `test_grade.py`; extended `test_split.py`; `val_size>50` gate.
**Avoids:** Pitfall 1, 2, 9.
**Anti-vacuity gate:** do-nothing agent fails Rust (proves `cargo test -- --include-ignored`); test-tampering cannot force green; planted TEST→TRAIN leak goes RED; `val_size=50` no-ships, `51`+delta adoptable.

### Phase 109: SWE-bench grader via Podman
**Rationale:** Heaviest dependency; depends on 107 + 108; pluggable behind `taskmetric.py`.
**Delivers:** mirror of dataset-name allowlist + fixed argv + env allowlist; `DOCKER_HOST→podman.sock`; predictions.jsonl from agent diff; harness-report parse.
**Uses:** `swebench==4.1.0`; `podman system service`.
**Avoids:** Pitfall 6 (dataset id pinned to the correct org, fetch resolves, task-count > 0, socket up).
**Anti-vacuity gate:** hermetic fake-harness test asserts exact argv + env allowlist; live run gated, skips offline (fail-not-skip on a *requested* real run).

### Phase 110: Human-gated adoption re-verification
**Rationale:** Gates pipeline output; nothing downstream depends on it. Re-verification of v2.2 invariant + REPORT.md verdict.
**Delivers:** confirm `optimize.py` writes only git-ignored `output/optimized.json`; adoption = human SKILL.md edit (≤1536 chars, `## Decision matrix` anchor) + `helix-refgen --check`; REPORT.md records ON/OFF/Δ/val_size + adopt/no-ship.
**Avoids:** Pitfall 4, 8 (`grep -E 'skills/helix|reference\.md' optimize.py == 0`, bundle allowlist).
**Anti-vacuity gate:** hand-edit `reference.md` out of sync → `--check` fails non-zero.

### Phase Ordering Rationale
- Dependency-forced; cheap Aider path proves the loop before heavy SWE-bench; adoption gates the output last.
- Each phase ships its break-the-invariant test before the next depends on it; review+fix folds in BEFORE verify.

### Research Flags
- **Phase 108:** honest-oracle design is the highest-risk surface + the v2.2 no-ship root cause — warrants a research pass.
- **Phase 109:** live SWE-bench-on-Podman (rootless quirks, `--network=none`, dataset-org resolution) needs implementation-time confirmation.
- **Phase 107:** standard ReAct/tool-call pattern, DeepSeek docs verified — skip research.
- **Phase 110:** unchanged v2.2 mechanism — skip research.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified vs PyPI + official docs + Context7; integration points from live source. |
| Features | HIGH | Existing harness read directly; v2.2 no-ship lessons from 106 artifacts. |
| Architecture | HIGH | All integration points from source; only the Python agent is greenfield. |
| Pitfalls | HIGH | Grounded in Phase 106 spike artifacts, carried memories, primary docs. |

**Overall confidence:** HIGH

### Gaps to Address (pin during requirements)
- **Exact corpus composition** (the single most important open question): which Aider exercises (and count) + which SWE-bench instance subset, sized so `val_size > 50` is clearable affordably. The literal v2.2 no-ship axis — pin before any GEPA run.
- **Role of `choice_rate`:** keep as a secondary/diagnostic pre-screen, or drop? MUST NOT be a (co-)optimization target. `scorer.py` is retained unchanged so either choice is cheap.
- **DeepSeek model id at implementation time** (alias retires 2026-07-24; config var makes cutover one line).
- **SWE-bench dataset-name allowlist edit** (`princeton-nlp/` datasets vs `SWE-bench/` repo org) — confirm/extend at Phase 109.

## Sources

**Primary (HIGH):** Context7 `/llmstxt/dspy_ai_llms_txt`; DeepSeek API docs (2026-06-24); SWE-bench official + HF datasets; PyPI JSON API (dspy 3.2.1 / openai 2.43.0 / swebench 4.1.0); live source (`tools/dspy-tune/*`, `bench/runtime/*`, `bench/datasets/aider-polyglot/loader.go`, `bench/evaluators/swebench/harness.go`, `internal/lint/toolsquarantine/analyzer.go`, `cmd/helix-refgen/*`); Phase 106 spike artifacts; project memories (anti-vacuity, HELIX_BIN, Podman, bundle allowlist); host probe (2026-06-24).
**Secondary (MEDIUM):** Rigorous Evaluation of Coding Agents on SWE-Bench (ACL 2025); swebench.com evaluation guide.

---
*Research completed: 2026-06-24*
*Ready for roadmap: yes*
