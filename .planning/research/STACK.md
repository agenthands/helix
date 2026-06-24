# Stack Research

**Domain:** Dev-time/offline LLM optimization tooling for a Go-native CLI product (Helix v2.3 "Task-Success-Driven Skill Optimization")
**Researched:** 2026-06-24
**Confidence:** HIGH (versions verified against PyPI + DeepSeek/DSPy/SWE-bench official docs + Context7; integration points read from live source)

> **Scope guard.** This research covers ONLY the THREE net-new v2.3 capabilities: (1) a DeepSeek/OpenAI tool-using agent that drives the `helix` CLI, (2) rewiring the `tools/dspy-tune/` GEPA metric to agent task-success on Aider polyglot + SWE-bench, (3) human-gated SKILL.md adoption via `cmd/helix-refgen`. It does NOT re-research the validated building blocks (aider-polyglot loader, aider_edit harness, bench/container, swebench adapter, test/oracle/adopt).
>
> **The load-bearing constraint that governs every choice below:** Helix ships as a single Go binary. DSPy + the optimizer agent are STRICTLY dev-time/offline. ZERO new Go module deps that touch the binary; NO `helix` subcommand shelling to Python; OFF `go.mod` / `helix setup` / default `go test ./...` / the merge path. The `internal/lint/toolsquarantine` analyzer (wired into `make vet` at v2.2 Phase 106) mechanically enforces that no runtime/cmd package imports `github.com/agenthands/helix/tools/...`.

---

## The Central Architecture Decision: Where Does the Agent Live?

This is the question the downstream roadmapper most needs answered. There are two candidate homes, and they pull in **opposite stack directions**. The verified recommendation is a **split**:

| Concern | Recommendation | Home |
|---------|---------------|------|
| **GEPA optimization loop + task-success metric** | **Python**, under `tools/dspy-tune/` | dev-time, quarantined |
| **The tool-using agent (DeepSeek/OpenAI loop driving `helix` verbs)** | **Python**, alongside `tools/dspy-tune/` (NOT Go `bench/runtime`) | dev-time, quarantined |

### Why the agent must be Python, not Go `bench/runtime`

The instinct is to put the agent in `bench/runtime` to reuse `aider_edit_cell.go` and the daemon-dial plumbing. **Reject this.** Reasoning, verified against source:

1. **DSPy is the metric's caller, and DSPy is Python.** The v2.3 headline is rewiring `optimize.py`'s GEPA `metric=` from `score_choice_rate` to *agent task-success*. GEPA invokes the metric **in-process, per candidate prompt, thousands of times** (`optimizer.compile(...)`). The metric must run the agent. If the agent lived in Go `bench/runtime`, every GEPA metric call would have to shell `go test`/`go run` or a compiled Go bench binary — a process-spawn-per-evaluation tax inside the optimizer's hot loop, plus a brittle text-protocol handoff. Keeping the agent in Python lets GEPA call it as a function.

2. **Putting a DeepSeek/OpenAI agent in `bench/runtime` adds a Go LLM dependency to a package that `go test ./...` compiles.** `bench/runtime` is in-tree Go that the default test command builds. A Go OpenAI client (e.g. `github.com/sashabaranov/go-openai`) added there is a **new `go.mod` dependency on the runtime side** — exactly what the hard constraint forbids. The existing `bench/runtime/subprocess/claude.go` only gets away with shelling the external `claude` *binary* (zero Go LLM SDK); a DeepSeek/OpenAI agent would need an HTTP/SDK client in Go. That crosses the line.

3. **The transport is already subprocess.** The locked decision is "agent drives `helix` via CLI subprocess verbs." A Python agent shelling `helix go-to-definition …` via `subprocess.run` is the *native* expression of that transport — no daemon-dial Go code, no `forwarder.OpenSession` reuse needed. The `claude.go` pattern (delegate to a process, capture stdout, parse) is exactly what the Python agent re-expresses, but the "process" is `helix <verb>` instead of `claude`.

4. **`bench/runtime` reuse is a mirage for this task.** `aider_edit_cell.go` / `aider_edit_agent.go` implement a *deterministic, scripted* edit agent (it copies the reference solution into the stub — `newDeterministicEditAgent`). It is NOT an LLM agent and shares almost no logic with an agentic tool-use loop. The genuinely reusable assets are the **dataset loaders and graders** (`aider-polyglot` `LoadExercise`/`NativeTestCommand`, `bench/evaluators/swebench` harness wrapper), which the Python agent invokes *as subprocesses / as fixtures*, not as linked Go code.

> **Net:** the agent is a new Python package, e.g. `tools/agent/` (or a module inside `tools/dspy-tune/`), reachable only from the dev venv. It shells `helix <verb>` for tool calls and shells the Go graders (`helix-bench` / the swebench harness) for scoring. The Go side gains the agent NOTHING it must compile — preserving `go test ./...`, `go.mod`, and the `toolsquarantine` boundary unchanged.

---

## Recommended Stack

### Core Technologies (all dev-time Python, `tools/` venv only)

| Technology | Version (pin) | Purpose | Why Recommended |
|------------|---------------|---------|-----------------|
| **DSPy** | `dspy==3.2.1` | GEPA prompt optimizer + LM abstraction | Already the pinned harness dep (`tools/dspy-tune/requirements.txt`). GEPA top-level API (`from dspy import GEPA`, `metric(gold,pred,trace,pred_name,pred_trace)->dspy.Prediction(score=,feedback=)`, `compile(student,trainset,valset)`) is stable 3.1.x→3.2.x. **Keep the existing 3.2.1 pin** — latest PyPI is also 3.2.1, no bump needed. Requires Python `>=3.10,<3.15`. |
| **OpenAI Python SDK** | `openai==2.43.0` | The HTTP client for BOTH DeepSeek (OpenAI-compatible `base_url`) and OpenAI (fallback). Drives the agentic tool-use loop (`chat.completions.create(..., tools=[...], tool_choice=...)`). | DeepSeek's API is **OpenAI-compatible**: one SDK, two `base_url`s. This is the single client for the whole agent. Latest PyPI `2.43.0`, requires Python `>=3.9`. DSPy's own LM layer uses litellm and does NOT need this — the AGENT uses `openai` directly; DSPy uses its own `dspy.LM`. |
| **swebench** | `swebench==4.1.0` | The SWE-bench evaluation harness (`python -m swebench.harness.run_evaluation`) the Go adapter already shells to, and that the task-success metric scores against. | Already the assumed upstream by `bench/evaluators/swebench/harness.go`. Latest PyPI `4.1.0`, requires Python `>=3.10`. **PIN IT** in the dev venv — see the dataset-name drift warning below. |
| **Python** | `3.13` (host has 3.13.5; require `>=3.10,<3.15`) | Interpreter for the venv | The intersection of dspy (`<3.15,>=3.10`), openai (`>=3.9`), swebench (`>=3.10`) is `>=3.10,<3.15`. Host 3.13.5 satisfies it. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **pytest** | `pytest==8.3.5` | Hermetic LM-free gates (parity, split, degenerate) | Already pinned. Add new gates for the agent loop + metric wiring (each a break-the-invariant→RED test per the anti-vacuity rule). |
| **litellm** | (transitive via `dspy`) | DSPy's LM backend; how `dspy.LM("openai/<model>", api_base=…, api_key=…)` reaches DeepSeek | Do NOT pin directly — let dspy own it. Relevant only because the **DSPy reflection/student LM** config for DeepSeek goes through it (see DeepSeek-as-DSPy-LM below). |

> **No new Go dependencies.** The Go side reuses only what already exists: `bench/datasets/aider-polyglot` (loader/grader), `bench/evaluators/swebench` (harness wrapper), `bench/container` (podman detect), and the `helix` binary itself. Nothing is added to `go.mod`.

### Development Tools / Environment

| Tool | Purpose | Notes |
|------|---------|-------|
| **Podman** | Container engine for SWE-bench | Host has `podman 5.4.2`. `bench/container.Detect()` already finds it (`docker` then `podman`). |
| **`podman system service`** | Docker-compatible API socket for the upstream swebench harness | The harness talks the Docker API. Stand up the socket (see SWE-bench section). |
| **Go toolchain** | `go1.26.0` | For the reused Go graders; unchanged. |
| **Per-language test toolchains** (Aider polyglot) | Run exercise tests for pass/fail scoring | **Verified present on host:** `go1.26`, `python3 3.13.5` (`pytest` via venv), `node v20.19.6` (JS uses `./npm-test.sh`), `cargo 1.96` (Rust), `g++ 14.2` (C++/ctest), `java/openjdk 21` (Java uses `./gradlew test` — the **gradle wrapper is checked into each exercise**, so system `mvn`/`gradle` are NOT required). **`javac`, `mvn`, `tsc` absence is NOT a blocker** — see the toolchain analysis below. |

---

## Installation

```bash
# Dev venv ONLY — never touches go.mod, the binary, or `helix setup`.
cd tools/dspy-tune        # (and the new tools/agent/ if split out)
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
```

`tools/dspy-tune/requirements.txt` (proposed v2.3 contents — add the two new pins):

```
# Dev-time-only pins. NEVER enters the helix binary, go.mod, `helix setup`, or `go test ./...`.
dspy==3.2.1          # GEPA optimizer + dspy.LM (keep — already current)
openai==2.43.0       # NEW: agent's tool-use client for DeepSeek + OpenAI
swebench==4.1.0      # NEW: SWE-bench eval harness scored by the task-success metric
pytest==8.3.5        # hermetic LM-free gates
```

> The `tools/` tree ships **no `.go`/`go.mod`** (it is invisible to the Go build); `toolsquarantine` is the belt that keeps any future `tools/*.go` from leaking. The `requirements.txt` is dev-only and is installed into a git-ignored `.venv`.

---

## Detail 1 — DeepSeek + OpenAI agent (the tool-using loop)

**Client:** `openai==2.43.0` for both providers — DeepSeek is OpenAI-compatible.

```python
from openai import OpenAI
import os

# Primary: DeepSeek (DEEPSEEK_API_KEY is SET in this env).
deepseek = OpenAI(
    api_key=os.environ["DEEPSEEK_API_KEY"],
    base_url="https://api.deepseek.com",        # OpenAI-compatible; verified official
)
# Fallback: OpenAI (OPENAI_API_KEY is SET).
openai_fb = OpenAI(api_key=os.environ["OPENAI_API_KEY"])
```

**Verified facts (DeepSeek official docs, api-docs.deepseek.com, 2026-06-24):**
- **Base URL:** `https://api.deepseek.com` (OpenAI-compatible). Strict-mode function calling uses `https://api.deepseek.com/beta` with `"strict": true` in the tool schema.
- **Models:** `deepseek-chat` and `deepseek-reasoner` are documented as **deprecated 2026/07/24**, succeeded by **`deepseek-v4-flash`** (non-thinking) and **`deepseek-v4-pro`** (thinking). **Recommendation: target `deepseek-v4-flash` as primary** (the successor to `deepseek-chat`, the model with confirmed function-calling examples), but make the model name a config var (`DSPY_LM_MODEL` precedent) so the deprecation cutover is a one-line change. Verify the live model name at implementation time.
- **Tool calling:** YES — `tools=[{type:"function",...}]` + `tool_choice`, OpenAI-format `tool_calls` / `tool_call_id` for multi-turn. This is exactly the agentic loop primitive needed. `deepseek-chat`/`-v4-flash` support it. (DeepSeek docs note historic flakiness in tool-calling; budget a retry + a max-turns cap — mirror `claude.go`'s `MaxToolCalls`.)

**The loop (Python, shelling `helix` verbs as tools):**
1. Define each `helix` verb the agent may use as an OpenAI function tool (name = verb, params = the verb's args). Keep the toolset small and curated (the verbs in CLAUDE.md's routing table), NOT all 50.
2. `chat.completions.create(model=…, messages=…, tools=…)`.
3. On a `tool_call`, execute `subprocess.run(["helix", verb, *args], cwd=workdir, capture_output=True)` — the **CLI subprocess transport** (locked decision). Feed stdout back as a `role:"tool"` message.
4. Loop until the model emits a final answer or `MaxToolCalls` is hit.
5. Score the resulting workspace state with the task-success oracle (Detail 3/4).

**Provider fallback:** wrap the `create` call; on DeepSeek error/timeout, retry against `openai_fb` with an equivalent OpenAI model. Keep both behind one `chat(...)` helper so the GEPA metric is provider-agnostic. (Mirrors the v1.4 "DeepSeek as Anthropic fallback" decision already in PROJECT.md Key Decisions.)

> **Why `openai` and NOT a Go client:** putting an LLM client in Go `bench/runtime` adds a runtime-side `go.mod` dep (forbidden); Python keeps it in the quarantined `tools/` venv and lets DSPy call the agent as an in-process function. See "Where Does the Agent Live?" above.

---

## Detail 2 — DSPy GEPA wiring for a task-success metric

**How GEPA metrics work (verified, DSPy 3.2.x official + Context7):**

A GEPA feedback metric has this signature and return contract:

```python
def metric(gold, pred, trace=None, pred_name=None, pred_trace=None) -> dspy.Prediction:
    # return dspy.Prediction(score=<float>, feedback=<str>)
    ...
```

- `score` is a float (use `1.0` solved / `0.0` unsolved, or a partial-credit fraction).
- `feedback` is free text GEPA's reflection LM reads to mutate the prompt — this is GEPA's edge over plain bootstrapping; **populate it with WHY the task failed** (e.g. "agent used `grep` instead of `helix find-references`; missed call site X"), not just the score.
- The existing `adopt_metric` in `optimize.py` already returns `dspy.Prediction(score=, feedback=)` — **the rewire keeps the signature and return type identical; only the body changes** from `score_choice_rate(pred.response)` to "run the agent, run the task's tests, score pass/fail."

**Plugging a non-trivial (subprocess-run, container-backed) evaluation as the metric:**

GEPA does NOT care that the metric is expensive — it just calls it. The metric body becomes:

```python
def task_success_metric(gold, pred, trace=None, pred_name=None, pred_trace=None):
    # pred carries the candidate steering text GEPA is optimizing.
    steering = pred.<field>                       # the SKILL.md-bound instruction under optimization
    workdir  = setup_task_workspace(gold)         # clone exercise / SWE-bench instance
    run_agent(steering, workdir)                  # Detail-1 loop: DeepSeek drives `helix` verbs
    solved, why = grade(gold, workdir)            # Aider: NativeTestCommand; SWE-bench: harness
    return dspy.Prediction(score=1.0 if solved else 0.0,
                           feedback=("solved" if solved else f"unsolved: {why}"))
```

**Practical constraints this introduces (flag for the roadmapper):**
- **Cost/latency:** GEPA calls the metric many times; each call now spawns an LLM agent loop + a test/container run (seconds–minutes each, vs the old `choice_rate`'s microseconds). Keep the **trainset tiny** (the existing harness already guards `len(examples)<2` and uses `auto="light"`). Consider Aider polyglot (cheap, ~seconds/exercise) for the optimization loop and SWE-bench (expensive, container-backed) for a **final held-out report only** — mirroring the existing `test.jsonl` sequestration discipline.
- **Determinism / caching:** the agent is non-deterministic (LLM). Document this; do NOT assert byte-stable scores. The hermetic gates must be the LM-free split/parity/degenerate tests (the existing pattern), NOT the agent run.
- **Key-absence guard:** the existing harness exits 0 when `OPENAI_API_KEY` is unset. v2.3 must extend this to **`DEEPSEEK_API_KEY` (primary)** — exit 0 cleanly when neither key is set, so CI/executor stays green and the metric never crashes.

**DeepSeek-as-DSPy-LM configuration (verified, dspy.ai official):**

DSPy reaches any OpenAI-compatible provider via the `openai/` litellm prefix + `api_base`:

```python
import dspy
# Student/program LM AND the GEPA reflection_lm both point at DeepSeek.
lm = dspy.LM("openai/deepseek-chat",                 # or "openai/deepseek-v4-flash" post-cutover
             api_key=os.environ["DEEPSEEK_API_KEY"],
             api_base="https://api.deepseek.com")     # note: api_base, NOT base_url, for dspy.LM
dspy.configure(lm=lm)
reflection_lm = dspy.LM("openai/deepseek-chat", temperature=1.0,
                        api_key=os.environ["DEEPSEEK_API_KEY"],
                        api_base="https://api.deepseek.com")
optimizer = GEPA(metric=task_success_metric, auto="light",
                 track_stats=True, reflection_lm=reflection_lm)
```

> Note the two distinct knobs: `dspy.LM(..., api_base=...)` is litellm's parameter name (NOT `base_url`); the raw `openai` client (Detail 1) uses `base_url`. They are different libraries — don't conflate. The existing `optimize.py` reads `DSPY_LM_MODEL` from env; extend it to default to a DeepSeek model and read `DEEPSEEK_API_KEY`.

---

## Detail 3 — SWE-bench harness on Podman

The upstream `swebench` harness talks the **Docker API**, not the docker CLI argv. Podman exposes a Docker-compatible API socket; point the harness at it.

**Exact setup (verified against CLAUDE.md's recorded recipe + swebench docs + host probe):**

```bash
# 1. Start Podman's docker-compatible API socket (host has NO running socket by default — verified).
podman system service --time=0 &
#   socket lands at: $XDG_RUNTIME_DIR/podman/podman.sock   (host: /run/user/1000/podman/podman.sock)

# 2. Point the harness at it. The Go wrapper already forwards DOCKER_HOST through its env allowlist.
export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"

# 3. Run the harness (the Go adapter shells exactly this; or run directly in the dev venv).
python -m swebench.harness.run_evaluation \
    --dataset_name <dataset> \
    --predictions_path <abs-clean-path>.jsonl \
    --run_id <id> \
    --max_workers <n> \
    --cache_level base
```

**Verified integration points in the existing Go adapter (`bench/evaluators/swebench/harness.go`):**
- It already builds exactly `["-m","swebench.harness.run_evaluation","--dataset_name",…,"--predictions_path",…,"--run_id",…,"--max_workers",…,"--cache_level",…,"--instance_ids",…]`.
- Its env allowlist **already forwards `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`** when set (`envAllowlist`), and fails closed on empty `PATH`. So Podman support is **already wired** — the only operational step is starting `podman system service`. **Do NOT report "Docker not installed → blocked."**

**Dataset names — VERSION/NAME DRIFT WARNING (important for the roadmapper):**
- The Go adapter's allowlist currently pins **`princeton-nlp/SWE-bench_Verified`** and `Bertsekas/SWE-Bench_Verified_UTBoost` (`allowedDatasetNames`, harness.go:30–33).
- **Upstream SWE-bench has since migrated the HF org to `SWE-bench/…`.** Verified current names: SWE-bench Verified = **`SWE-bench/SWE-bench_Verified`**, SWE-bench Lite = **`princeton-nlp/SWE-bench_Lite`** (the `princeton-nlp` mirror still resolves for some suites; the `SWE-bench/` org is canonical going forward).
- **Action for v2.3:** when scoring against SWE-bench, update `allowedDatasetNames` to include the current `SWE-bench/SWE-bench_Verified` (and add `princeton-nlp/SWE-bench_Lite` if Lite is used) — and pin `swebench==4.1.0` in the venv so the harness module path / dataset expectations are stable. This is a small Go allowlist edit (additive, in the already-existing bench package — NOT a new dep) plus a venv pin. The Phase 87 notes already flagged the dataset name as `[ASSUMED]`/deferred-to-live-confirmation.

**Version pinning:** `swebench==4.1.0` (latest, `requires-python>=3.10`). Pin in `requirements.txt`; the harness module path `swebench.harness.run_evaluation` is stable across recent majors.

---

## Detail 4 — Aider polyglot task-success scoring

**How pass/fail is determined (verified from `bench/datasets/aider-polyglot/loader.go`):**

Each exercise is graded by running the dataset's **NATIVE per-language test command** in the work dir; `Passed = (exit code == 0)`. The native commands (`nativeTestCommand`, loader.go:299–318):

| Language | Test argv | Host toolchain | Present? |
|----------|-----------|----------------|----------|
| `python` | `pytest` | `pytest` (from venv) + `python3 3.13.5` | ✅ (venv) |
| `go` | `go test ./...` | `go 1.26.0` | ✅ |
| `rust` | `cargo test -- --include-ignored` (acceptance tests are `#[ignore]`) | `cargo 1.96.0` | ✅ |
| `java` | `./gradlew test` | **gradle wrapper checked into the exercise** + JRE/JDK | ✅ `openjdk 21` (no system gradle/mvn needed) |
| `javascript` | `./npm-test.sh` | `node v20.19.6` | ✅ |
| `cpp` | (ctest/JUnit per loader) | `g++ 14.2.0` | ✅ |

**Toolchain verdict — NOT blocked:** all six tracks' required runtimes are present. The earlier-flagged absences (`javac`, `mvn`, `tsc`) are **not used by these commands**: Java runs via the **checked-in `./gradlew` wrapper** (downloads its own gradle; uses `java`/the JDK, present as openjdk 21), JS runs `./npm-test.sh` (uses `node`, present), and TypeScript isn't a separate track here (the JS track covers it). The pristine-test anti-tamper (`restorePristine`, `restorePristineTests`) and the 2-attempt reprompt protocol are already in the loader.

**Reuse for the metric:** the Python agent (Detail 1) edits the exercise via `helix` verbs, then the metric shells the per-language test command — easiest path is to call the Go grader: `aiderpolyglot.NativeTestCommand(lang)` is exported, OR the Python metric replicates the tiny argv map. Prefer invoking the existing Go path through `helix-bench` to keep the single source of truth, but a small Python argv mirror is acceptable if parity-pinned (the project's established pattern — cf. `scorer.py` mirroring `scorecard.go` against a shared golden corpus).

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Agent in **Python** (`tools/`) | Agent in **Go** `bench/runtime` reusing `aider_edit_cell.go` | Only if the agent were *deterministic/scripted* (no LLM) — then Go + zero LLM SDK works. For an LLM tool-use loop it forces a runtime-side Go LLM dep (forbidden) and a process-spawn handoff to the Python GEPA metric. Rejected. |
| **`openai` SDK** for DeepSeek | DeepSeek's own SDK / raw `requests` | Never — DeepSeek is OpenAI-compatible; one SDK covers both providers and the fallback. |
| **`openai` SDK** for the agent | `litellm` directly in the agent | litellm is fine and is already transitive via dspy; but the raw `openai` client gives the cleanest tool-call loop and exactly mirrors DeepSeek's documented examples. Use litellm only inside `dspy.LM`. |
| Pin **`swebench==4.1.0`** | Unpinned `pip install swebench` | Never — dataset-name/harness drift (the `princeton-nlp`→`SWE-bench` org migration) makes pinning mandatory. |
| **`deepseek-v4-flash`** (config var) | Hard-code `deepseek-chat` | `deepseek-chat` is deprecated 2026/07/24; keep it as the value only until the cutover, behind the `DSPY_LM_MODEL` env knob. |
| **SWE-bench Verified** as held-out | SWE-bench full / Multi-SWE | Verified is the curated, container-reproducible set the Go adapter already targets; full set is too expensive for an optimization loop. Lite is a cheaper option for smoke. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| A Go LLM SDK (`go-openai`, etc.) in `bench/runtime` or anywhere in-tree | New runtime-side `go.mod` dependency; violates the single-binary / no-runtime-LLM-dep constraint; `toolsquarantine` + `go test ./...` would still compile it | Python `openai` SDK in the `tools/` venv |
| A `helix` subcommand that shells to Python / DSPy | Violates "no `helix` subcommand shells to Python"; puts dev-time tooling on the product surface | Keep the optimizer/agent invocable ONLY from the dev venv (`python optimize.py`) |
| Adding `dspy`/`openai`/`swebench` to `go.mod` or any Go import | They are Python; this is structurally impossible but the *intent* (any new merge-path dep) is forbidden | `tools/dspy-tune/requirements.txt`, git-ignored `.venv` |
| `"helix" in response` style substring scoring (the old proxy's trap) | The whole v2.3 thesis is that `choice_rate` is gameable; do not carry its substring/first-command logic into the task-success metric | Real test-execution pass/fail (Aider `NativeTestCommand`, SWE-bench `report.resolved`) |
| Auto-adopting optimized text into `SKILL.md`/`reference.md` | Violates the human-gated invariant carried from v2.2 | `cmd/helix-refgen` + `helix-refgen --check`; git-ignored `output/optimized.json`; human transcribes |
| Reporting "Docker not installed → blocked" for SWE-bench | Host has Podman 5.4.2; `bench/container` auto-detects it; the Go adapter already forwards `DOCKER_HOST` | `podman system service --time=0 &` + `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` |

## Stack Patterns by Variant

**If the optimization loop must stay cheap (the common case):**
- Use **Aider polyglot** as the GEPA trainset/valset (seconds/exercise, no containers), DeepSeek as the agent LM.
- Reserve **SWE-bench (Verified, via Podman)** for a final held-out report only (sequestered like the existing `test.jsonl`).

**If `DEEPSEEK_API_KEY` is unset (CI / executor):**
- The harness exits 0 with an informative message (extend the existing `OPENAI_API_KEY` guard to cover `DEEPSEEK_API_KEY` primary). Hermetic LM-free gates (split/parity/degenerate + new agent-loop unit tests) run without any key.

**If DeepSeek tool-calling flakes or rate-limits:**
- Fall back to the OpenAI client (`OPENAI_API_KEY` set) behind the single `chat(...)` helper; cap turns via `MaxToolCalls` (mirror `claude.go`).

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `dspy==3.2.1` | Python `>=3.10,<3.15` | Host 3.13.5 OK. GEPA API stable 3.1→3.2. |
| `openai==2.43.0` | Python `>=3.9` | One client for DeepSeek (`base_url=https://api.deepseek.com`) + OpenAI. |
| `swebench==4.1.0` | Python `>=3.10`; Docker API (Podman socket via `DOCKER_HOST`) | Pin mandatory due to dataset-org drift (`princeton-nlp`→`SWE-bench`). |
| DSPy ↔ DeepSeek | `dspy.LM("openai/deepseek-…", api_base="https://api.deepseek.com")` | `api_base` (litellm), NOT `base_url`. |
| Go side | `go1.26.0`; no new `go.mod` deps | Reuses aider-polyglot loader, swebench wrapper, container detect. `toolsquarantine` boundary unchanged. |

## Sources

- `/llmstxt/dspy_ai_llms_txt` (Context7, benchmark 87.12) — DSPy GEPA metric signature/return contract; generic OpenAI-compatible LM config (`dspy.LM("openai/...", api_base=...)`). **HIGH**
- https://api-docs.deepseek.com/ + /guides/function_calling (DeepSeek official, 2026-06-24) — base_url `https://api.deepseek.com`, models (`deepseek-v4-flash`/`-pro`; `deepseek-chat`/`-reasoner` deprecated 2026/07/24), tool-calling support, strict-mode `/beta`. **HIGH**
- https://github.com/SWE-bench/SWE-bench (official) — `python -m swebench.harness.run_evaluation` flags; dataset names (`SWE-bench/SWE-bench_Verified`, `princeton-nlp/SWE-bench_Lite`); Docker-based. **HIGH**
- PyPI JSON API (2026-06-24) — latest+requires-python: `dspy 3.2.1` (`>=3.10,<3.15`), `openai 2.43.0` (`>=3.9`), `swebench 4.1.0` (`>=3.10`). **HIGH**
- Live source (read 2026-06-24): `tools/dspy-tune/{optimize.py,scorer.py,requirements.txt}`, `bench/runtime/subprocess/claude.go`, `bench/runtime/aider_edit_agent.go`, `bench/datasets/aider-polyglot/loader.go`, `bench/evaluators/swebench/harness.go`, `bench/container/engine.go`, `internal/lint/toolsquarantine/analyzer.go`. **HIGH**
- Host probe (2026-06-24): `podman 5.4.2` (no running socket), `go 1.26.0`, `python3 3.13.5`, `node v20.19.6`, `cargo 1.96.0`, `g++ 14.2.0`, `openjdk 21`; `javac`/`mvn`/`tsc` absent (NOT required); `DEEPSEEK_API_KEY`+`OPENAI_API_KEY` set, `ANTHROPIC_API_KEY` unset. **HIGH**

---
*Stack research for: Helix v2.3 Task-Success-Driven Skill Optimization (dev-time/offline LLM optimization tooling for a single-binary Go product)*
*Researched: 2026-06-24*
