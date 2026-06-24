# Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering - Research

**Researched:** 2026-06-24
**Domain:** Dev-time Python OpenAI-compatible tool-using ReAct agent driving the `helix` CLI subprocess surface
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Agent lives in Python under `tools/dspy-tune/agent/` (NOT Go `bench/runtime`) — GEPA calls its metric in-process per candidate. Files: `react.py`, `llm.py`, `tools.py` (sibling of `scorer.py`).
- One `openai==2.43.0` client wraps both providers: **DeepSeek primary** (`base_url=https://api.deepseek.com`, explicit `deepseek-v4-*` model id as a config var — the `deepseek-chat`/`-reasoner` aliases retire **2026/07/24 15:59 UTC**), **OpenAI fallback**.
- Agent invokes helix via **CLI subprocess verbs** (the product surface): `helix <verb> ...`.
- Hard turn / no-progress cap; deterministic termination; per-run transcript/trace capture.
- A real run with the selected provider's API key absent **FAILS loudly** (not a silent skip). `DEEPSEEK_API_KEY` + `OPENAI_API_KEY` are present in the dev environment.
- First-class `--steering on|off`: ON injects candidate skill text as the system prompt; OFF uses a **provably steering-omitted** control prompt.
- **Anti-vacuity gate (required):** ship a deliberate break-the-invariant → assert-RED test — a hermetic `test_agent.py` (fake LLM + fake `helix`, no network) where a degenerate always-grep agent scores 0 and the OFF prompt provably omits the steering text. Fold code-review + fix BEFORE verify.
- **Boundary gate (ADOPT-04 cross-cutting):** `git diff go.mod` empty; `make vet` (`toolsquarantine`) green; no `helix` subcommand shells to Python.

### Claude's Discretion
All implementation choices below the locked constraints (loop shape, tool-schema authoring, no-progress heuristic, transcript schema, config-var names, retry/fallback trigger details, hermetic-test fixture design). Discuss phase was skipped (`workflow.skip_discuss: true`).

### Deferred Ideas (OUT OF SCOPE)
- The task-success metric rewire, Aider/SWE-bench graders, per-task sandbox (Phases 108–109).
- `choice_rate` removal/demotion (Phase 108 owns the metric body swap; `scorer.py` is untouched here).
- Human-gated SKILL.md adoption + `helix-refgen --check` (Phase 110).
- Any GEPA wiring of the agent into `optimize.py` — Phase 107 ships a **standalone, importable agent**, not the metric.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AGENT-01 | Agent drives `helix` CLI via **CLI subprocess verbs** in a bounded ReAct loop, observing each verb's output, capturing a per-run transcript/trace. | `tools.py` subprocess shim (§ Architecture Pattern 2, Code Example 3); bounded loop + transcript schema (Pattern 1, Code Example 1 & 4). Verb inventory: 51 canonical verbs from `README.md` / `internal/cli/verbs_gen.go`. |
| AGENT-02 | LM config-driven via one OpenAI-compatible client — DeepSeek primary / OpenAI fallback; hard turn / no-progress cap; loud failure on missing key for a real run. | `llm.py` single-`chat()`-helper + provider fallback (Pattern 3, Code Example 2); model as config var (`deepseek-v4-flash`, alias-safe); loud-fail-on-missing-key contract (§ Common Pitfalls #2, Code Example 2). |
| AGENT-03 | First-class steering ON/OFF — candidate skill text as system prompt (ON) vs a provably steering-omitted control prompt (OFF). | `--steering on\|off` plumbing into the system-prompt builder (§ Architecture Pattern 4, Code Example 5); provably-omitted OFF control + the hermetic assertion that proves omission (§ Validation Architecture, Pitfall #4). |
</phase_requirements>

## Summary

Phase 107 builds a **standalone, importable, dev-time Python tool-using agent** under `tools/dspy-tune/agent/` that drives the real `helix <verb>` CLI in a bounded ReAct loop. It is the foundation every later phase depends on (the metric, both graders, the adoption gate). The agent owns its own loop — it does NOT reuse `bench/runtime/subprocess/claude.go` (that path delegates to the external `claude` binary's loop and would need a Go LLM SDK). The agent is a plain Python package the future GEPA metric can `import` and call in-process, exactly as `optimize.py` imports `score_choice_rate` today.

The loop is the standard OpenAI **chat-completions tool-calling** pattern, verified against `/openai/openai-python` (Context7, 2026-06-24): a growing `messages` list, `client.chat.completions.create(model, messages, tools=[...])`, and on `choices[0].message.tool_calls` execute each call as a `subprocess.run(["helix", verb, *args], ...)` then append a `{"role":"tool","tool_call_id":...,"content":stdout}` message and loop. A **single `openai==2.43.0` client** wraps both providers because DeepSeek is OpenAI wire-compatible (`base_url=https://api.deepseek.com`); a `chat()` helper makes the loop provider-agnostic — DeepSeek primary, OpenAI fallback on error/timeout. The model id is a **config var defaulting to `deepseek-v4-flash`** (the `deepseek-chat` alias retires 2026/07/24 15:59 UTC; verified at api-docs.deepseek.com).

**Primary recommendation:** Ship three thin modules — `llm.py` (provider-agnostic `chat()` + loud-fail-on-missing-key + fallback), `tools.py` (fixed-argv `helix <verb>` subprocess shim + the OpenAI tool-schema list), `react.py` (the bounded think→act→observe loop + transcript) — plus a hermetic `test_agent.py` (fake LLM + fake `helix`, zero network) whose **break-the-invariant pair** proves (a) a degenerate always-grep agent scores 0 and (b) the OFF prompt provably omits the steering text. Add `openai==2.43.0` to `requirements.txt` only; assert `git diff go.mod` is empty and `make vet` stays green.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| LLM tool-calling loop | Dev-time Python (`tools/dspy-tune/agent/`) | — | GEPA invokes the metric→agent in-process; an LLM client in Go `bench/runtime` would add a forbidden runtime `go.mod` dep [VERIFIED: ARCHITECTURE.md, STACK.md]. |
| Provider selection / fallback | `llm.py` (Python `openai` SDK) | — | DeepSeek is OpenAI wire-compatible; one client, two `base_url`s [VERIFIED: STACK.md + DeepSeek docs]. |
| Tool execution (`helix <verb>`) | `tools.py` → `subprocess` → the shipped `helix` binary | The unchanged daemon/kernel | The locked transport is "CLI subprocess verbs"; the binary is the product surface, untouched [VERIFIED: CONTEXT.md, CLAUDE.md]. |
| Steering ON/OFF | `react.py` system-prompt builder | — | Steering is a prompt-construction concern, not a transport concern; the control arm must be provably omitted (AGENT-03). |
| Transcript / trace capture | `react.py` (in-memory list → JSON) | — | Per-run artifact for AGENT-01; no service tier involved. |
| Boundary enforcement | `internal/lint/toolsquarantine` (Go, unchanged) | `git diff go.mod`, `make vet` | Import-only analyzer; all new edges are Python/subprocess inside the exempt `tools/` prefix — no analyzer change [VERIFIED: ARCHITECTURE.md analyzer.go read]. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| OpenAI Python SDK | `openai==2.43.0` | The single chat-completions tool-calling client for BOTH DeepSeek (`base_url=https://api.deepseek.com`) and OpenAI fallback | DeepSeek is OpenAI wire-compatible; one SDK covers both providers and the fallback [VERIFIED: npm/PyPI not applicable — PyPI; STACK.md confirms latest 2.43.0, `requires-python>=3.9`] [CITED: api-docs.deepseek.com] |
| Python | `3.13` (host 3.13.5) | Interpreter for the dev venv | Intersection of all dev-venv pins; host satisfies it [VERIFIED: host probe `python3 --version` = 3.13.5] |
| pytest | `8.3.5` (already pinned) | Hermetic, LM-free, no-network agent test (`test_agent.py`) | Existing harness convention (`test_split.py`/`test_parity.py`/`test_degenerate.py` are pytest + script-runnable) [VERIFIED: requirements.txt read] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `subprocess` (stdlib) | — | Run `helix <verb> *args` and capture stdout/exit | Every tool call; the locked CLI transport [VERIFIED: CONTEXT.md] |
| `json` (stdlib) | — | Parse tool-call arguments; serialize the transcript | Tool-arg decode + trace write |
| `dspy==3.2.1` | unchanged | NOT used by the agent itself; only the future metric (Phase 108) imports the agent | Do not add agent→dspy coupling in Phase 107 [VERIFIED: optimize.py imports lazily] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `openai` SDK | DeepSeek native SDK / raw `requests` | Never — DeepSeek is OpenAI-compatible; one SDK covers both + fallback [VERIFIED: STACK.md] |
| `openai` SDK in the agent | `litellm` directly | litellm is transitive via dspy and is the right layer for `dspy.LM` (Phase 108), but the raw `openai` client gives the cleanest tool-call loop and mirrors DeepSeek's documented examples [CITED: STACK.md Detail 1] |
| Python agent | Go agent in `bench/runtime` reusing `aider_edit_cell.go` | Rejected — that is a deterministic scripted edit agent (not LLM); an LLM loop would force a runtime-side Go `go.mod` dep and a process-spawn handoff into GEPA's hot loop [VERIFIED: ARCHITECTURE.md, STACK.md "Where Does the Agent Live?"] |
| New agent loop | Reuse `claude.go` | Rejected — `claude.go` delegates to the external `claude` binary's OWN loop; `ANTHROPIC_API_KEY` is unset; the new agent owns its ReAct loop and shells `helix` directly [VERIFIED: REQUIREMENTS.md Out-of-Scope; claude.go read] |

**Installation:**
```bash
# Dev venv ONLY — never touches go.mod, the binary, or `helix setup`.
cd tools/dspy-tune
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt        # add: openai==2.43.0
```

`tools/dspy-tune/requirements.txt` after Phase 107 (additive — keep existing pins):
```
dspy==3.2.1          # GEPA optimizer + dspy.LM (unchanged; Phase 108 uses it)
openai==2.43.0       # NEW (Phase 107): agent tool-use client for DeepSeek + OpenAI
pytest==8.3.5        # hermetic LM-free gates
```

**Version verification:** `openai==2.43.0` is the latest PyPI release, `requires-python>=3.9` [VERIFIED: STACK.md PyPI JSON API probe 2026-06-24]. DeepSeek model `deepseek-v4-flash`/`-pro` are current; `deepseek-chat`/`-reasoner` are compatibility aliases retiring **2026/07/24 15:59 UTC** [CITED: api-docs.deepseek.com, fetched 2026-06-24].

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `openai` | PyPI | ~3 yrs (2.43.0 latest) | tens of millions/mo | github.com/openai/openai-python | OK | Approved — official OpenAI SDK |
| `dspy` | PyPI | unchanged pin (3.2.1) | high | github.com/stanfordnlp/dspy | OK | Approved — already in tree |
| `pytest` | PyPI | unchanged pin (8.3.5) | very high | github.com/pytest-dev/pytest | OK | Approved — already in tree |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

> Only one net-new pin (`openai==2.43.0`), and it is the canonical first-party OpenAI SDK confirmed via Context7 (`/openai/openai-python`, Source Reputation High, 584 snippets) AND official docs. No legitimacy gate concern. (The Package-Legitimacy seam could not be invoked in this session; verdict is from Context7 + official-repo confirmation + the existing STACK.md PyPI probe — treat as HIGH for `openai`, which is a household-name first-party SDK.)

## Architecture Patterns

### System Architecture Diagram

```
DEV-TIME / OFFLINE (tools/dspy-tune/agent/ — NEVER in the helix binary, go.mod, `go test ./...`)

  caller (Phase-107: a __main__/CLI entry; Phase-108: the GEPA metric, in-process)
        │  ReActAgent(system_prompt=<steering or control>, model=cfg).run(task, cwd)
        ▼
  ┌─────────────────────────────  react.py  ─────────────────────────────┐
  │  messages = [ {system: steered|control}, {user: task} ]              │
  │  loop (bounded by max_turns AND no-progress cap):                    │
  │     resp = llm.chat(messages, tools=tools.SCHEMAS)   ◄──┐            │
  │     if resp.tool_calls:                                  │            │
  │         for call in tool_calls:                          │            │
  │            out = tools.run_verb(call) ──subprocess──►  helix <verb>   │
  │            append {role:tool, tool_call_id, content:out.stdout}      │
  │            record transcript step (verb, argv, exit, stdout-tail)    │
  │         continue loop                                                │
  │     else: final answer → terminate (done)                           │
  │  termination: done | max_turns | no-progress | tool-error-budget    │
  └──────────┬───────────────────────────────┬──────────────────────────┘
             │ llm.chat()                     │ tools.run_verb()
             ▼                                ▼
   ┌──────────────────────┐        ┌──────────────────────────────────┐
   │ llm.py               │        │ tools.py                         │
   │  one openai.OpenAI   │        │  SCHEMAS: [{type:function,        │
   │  DeepSeek primary    │        │    function:{name:<verb>, ...}}]  │
   │  (base_url=…deepseek) │       │  run_verb(call) ->                │
   │  OpenAI fallback      │       │    subprocess.run(["helix", verb, │
   │  loud-fail on no key  │       │      *argv], cwd=, capture, text) │
   └──────────┬───────────┘        └──────────────┬───────────────────┘
              │ HTTPS                              │ argv (NOT import)
              ▼                                    ▼
     api.deepseek.com / api.openai.com    the SHIPPED `helix` binary → daemon
```

A reader traces the primary use case: caller builds a system prompt (steered or control) → `react.py` loop → `llm.chat()` returns tool calls → `tools.run_verb()` shells `helix <verb>` → stdout fed back → loop until done/budget → transcript returned.

### Recommended Project Structure
```
tools/dspy-tune/agent/        # NEW Python subpackage (sibling of scorer.py)
├── __init__.py               # exports ReActAgent, run(); keeps imports clean
├── llm.py                    # provider-agnostic chat(): DeepSeek primary + OpenAI fallback,
│                             #   model config var, loud-fail-on-missing-key
├── react.py                  # bounded think→act→observe loop + transcript/trace + steering
└── tools.py                  # OpenAI tool SCHEMAS for curated helix verbs + run_verb() subprocess
tools/dspy-tune/
├── test_agent.py             # NEW hermetic test: fake LLM + fake helix, no network;
│                             #   break-the-invariant pair (degenerate=0; OFF omits steering)
└── requirements.txt          # MODIFIED: add openai==2.43.0
```

**Why a subpackage, not `tools/agent/`:** the future metric calls the agent as an **in-process Python import** exactly as `optimize.py` does `from scorer import score_choice_rate` (optimize.py:69). Keeping `agent/` under `tools/dspy-tune/` matches that pattern and stays inside the one quarantined `tools/` namespace the analyzer already covers — a separate top-level `tools/agent/` forces a cross-package import for zero boundary benefit [VERIFIED: ARCHITECTURE.md "Structure Rationale"].

### Pattern 1: Bounded ReAct loop with multi-turn message accumulation
**What:** Maintain a growing `messages` list. Each turn calls `chat.completions.create(..., tools=...)`. If the assistant message carries `tool_calls`, append the assistant message verbatim, execute each call, append one `{"role":"tool","tool_call_id":...,"content":...}` per call, and loop. If no `tool_calls`, the assistant's text is the final answer — terminate.
**When to use:** Any OpenAI-compatible tool-using agent.
**Critical detail:** the assistant message that contains `tool_calls` MUST itself be appended to `messages` before the tool result messages, or the API rejects the next turn (tool messages must respond to a preceding assistant tool-call message) [VERIFIED: /openai/openai-python, `ChatCompletionToolMessageParam` requires `tool_call_id`].

### Pattern 2: Fixed-argv subprocess tool shim (the locked CLI transport)
**What:** `tools.py` maps each tool name (= a `helix` verb) to a fixed-argv `subprocess.run(["helix", verb, *args], cwd=workdir, capture_output=True, text=True, timeout=...)`. Never build the argv by string-concatenation or `shell=True`; pass a list. The verb name and arg schema come from a curated subset of the 51 canonical verbs.
**When to use:** Every tool execution.
**Trade-off:** subprocess-spawn-per-call overhead (acceptable — LLM latency dominates) [VERIFIED: ARCHITECTURE.md Pattern 1].

### Pattern 3: Provider-agnostic `chat()` with primary→fallback
**What:** One `chat(messages, tools)` helper hides provider choice. It constructs a DeepSeek `OpenAI(base_url="https://api.deepseek.com", api_key=DEEPSEEK_API_KEY)` client as primary and an `OpenAI(api_key=OPENAI_API_KEY)` client as fallback; on a DeepSeek error/timeout/rate-limit it retries the equivalent call against OpenAI. The model id is read from a config var defaulting to `deepseek-v4-flash`.
**When to use:** the whole agent — keeps `react.py` provider-agnostic so Phase 108's metric is too.
**Trade-off:** the fallback must map the DeepSeek model id to an OpenAI model id (a config var, e.g. `AGENT_FALLBACK_MODEL`) [VERIFIED: STACK.md Detail 1 "Provider fallback"].

### Pattern 4: Steering as a system-prompt switch with a provably-omitted control
**What:** `ReActAgent(system_prompt=...)`. The CLI/entry exposes `--steering on|off`. **ON** sets the system prompt to the candidate skill text (in Phase 108 this is the GEPA candidate; in Phase 107 a passed-in string / fixture). **OFF** sets a fixed *control* prompt that is constructed to **provably contain none of the steering text** — e.g. a minimal task-only instruction. The OFF prompt and the steering text are kept as separate constants so a test can assert `steering_text not in off_prompt` (substring/disjointness check).
**When to use:** AGENT-03; the foundation of the later ON-vs-OFF attribution delta.
**Anti-pattern it avoids:** building OFF by *redacting* the steering string at runtime (fragile — a near-miss leaves fragments). Build OFF as an independent constant and *assert* omission.

### Anti-Patterns to Avoid
- **Reusing `bench/runtime/subprocess/claude.go` or `helix-bench run` as the agent.** The first delegates to the `claude` binary's own loop (and `ANTHROPIC_API_KEY` is unset); the second is the whole matrix orchestrator. The new agent owns its loop and shells `helix` directly [VERIFIED: REQUIREMENTS.md Out-of-Scope; ARCHITECTURE.md anti-patterns].
- **`shell=True` / string-built argv.** Use a fixed `subprocess.run([...])` list — no shell injection surface, deterministic argv for the hermetic test [ASSUMED — standard secure-subprocess practice].
- **Silent skip on a missing key for a *real* run.** Hermetic tests run with NO key (fake LLM), but a real `run()` invoked with the selected provider's key absent must raise loudly, not return an empty/zero result (AGENT-02) [VERIFIED: CONTEXT.md, REQUIREMENTS.md].
- **`"helix" in response` substring scoring.** Irrelevant here (Phase 107 ships no metric) but tempting; the degenerate-agent hermetic check keys on *actual emitted argv* (always-grep), not substrings [VERIFIED: scorer.py/ARCHITECTURE.md substring-trap note].
- **A new `tools/*.go` file or any `go.mod` edit.** Either breaches the single-binary boundary; the quarantine analyzer is import-only and would only fire on a Go edge [VERIFIED: ARCHITECTURE.md analyzer semantics].
- **Building OFF by redacting the steering string.** Build OFF as an independent constant; assert omission (Pattern 4).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OpenAI/DeepSeek HTTP, retries, streaming, tool-call parsing | A hand-rolled `requests` client | `openai==2.43.0` SDK (`chat.completions.create`) | Handles both providers via `base_url`, typed `tool_calls`, `tool_call_id` [VERIFIED: /openai/openai-python] |
| Tool-call argument decode | Manual regex over the model's text | `message.tool_calls[i].function.arguments` (JSON string) → `json.loads` | The SDK already structures tool calls; arguments arrive as a JSON string [VERIFIED: /openai/openai-python] |
| Subprocess capture | Manual pipe plumbing | `subprocess.run(..., capture_output=True, text=True, timeout=)` | stdlib; deterministic; timeout-bounded [ASSUMED — stdlib] |
| Provider fallback orchestration | A bespoke circuit breaker | A simple try/except around the primary `create`, retry on the fallback client | The agent only needs primary→fallback, not a full breaker [VERIFIED: STACK.md] |

**Key insight:** the only genuinely new logic in Phase 107 is (a) the **bounded loop with a no-progress cap** and (b) the **provably-omitted OFF control** — everything else (HTTP, tool-call parsing, subprocess) is library/stdlib. Keep the modules thin.

## Common Pitfalls

### Pitfall 1: DeepSeek model-alias deprecation
**What goes wrong:** Hard-coding `deepseek-chat` breaks after **2026/07/24 15:59 UTC** when the alias retires.
**Why it happens:** training data and old examples use `deepseek-chat`.
**How to avoid:** read the model id from a **config var** (e.g. `AGENT_MODEL`, mirroring the `DSPY_LM_MODEL` precedent) defaulting to the explicit successor **`deepseek-v4-flash`** (non-thinking; the function-calling-capable successor to `deepseek-chat`). Cutover is then one line / one env var.
**Warning signs:** a 4xx "model not found" after the cutover date [CITED: api-docs.deepseek.com 2026-06-24].

### Pitfall 2: Silent skip vs loud failure on a missing key
**What goes wrong:** Copying `optimize.py`'s "key unset → print + `return 0`" guard into the *agent's real run path* would make a real run silently no-op — exactly the HELIX_BIN-fail-not-skip anti-pattern.
**Why it happens:** the existing harness deliberately exits 0 when `OPENAI_API_KEY` is unset so CI/executor stays green; that discipline is for the *optimizer entrypoint*, not the agent's `run()`.
**How to avoid:** the agent's `run()` (real path) **raises** when the selected provider's key is absent. The hermetic `test_agent.py` never needs a key because it injects a **fake LLM** (no `openai` client constructed). Keep the "exit 0 on unset key" behavior only at any optional `__main__` convenience wrapper, never inside `run()`.
**Warning signs:** a "real" run that returns an empty transcript with no error [VERIFIED: CONTEXT.md, REQUIREMENTS.md cross-cutting; optimize.py:48-62].

### Pitfall 3: Unbounded / non-progressing loop
**What goes wrong:** A model that keeps emitting the same failing tool call, or never emits a final answer, loops forever or burns budget.
**Why it happens:** ReAct loops need *two* bounds — a hard turn cap AND a no-progress cap.
**How to avoid:** (a) `max_turns` hard cap (deterministic termination); (b) a **no-progress** heuristic — e.g. terminate if the last N tool calls are byte-identical argv+output, or if a tool-error budget is exceeded. Record the termination *reason* in the transcript (`done | max_turns | no_progress | tool_error_budget`). The hermetic test drives a fake LLM that emits a repeating call and asserts the loop terminates with `no_progress` [VERIFIED: CONTEXT.md "no-progress cap"; mirrors `claude.go` MaxToolCalls].
**Warning signs:** a test that hangs; a transcript with no termination reason.

### Pitfall 4: OFF control not provably steering-omitted
**What goes wrong:** OFF still leaks steering text (e.g. a shared template that embeds a fragment), so an ON-vs-OFF delta is unattributable.
**Why it happens:** building OFF by editing/redacting the ON prompt.
**How to avoid:** OFF is an **independent constant**; `test_agent.py` asserts `STEERING_SENTINEL not in build_system_prompt(steering=off)` AND that ON contains it. Use a distinctive sentinel token in the steering fixture so the omission assertion is unambiguous [VERIFIED: CONTEXT.md AGENT-03; mirrors the `test_degenerate.py` flag/no-flag discriminating pair].
**Warning signs:** an ON/OFF test that only checks ON.

### Pitfall 5: Boundary breach (the cross-cutting ADOPT-04 gate)
**What goes wrong:** a stray `go.mod` change, a `tools/*.go` file, or a `helix` subcommand that shells Python.
**Why it happens:** muscle memory to "add a Go shim."
**How to avoid:** add `openai` to `requirements.txt` ONLY; assert `git diff go.mod` is empty and `make vet` (`vet-tools-quarantine`, the 7th vettool) is green as exit gates. All new edges are Python/subprocess inside the exempt `tools/` prefix — the import-only analyzer never sees them [VERIFIED: ARCHITECTURE.md analyzer.go read; Makefile vet chain].
**Warning signs:** a non-empty `git diff go.mod`; a new `.go` file under `tools/`.

## Code Examples

> The examples below are illustrative scaffolds grounded in the verified API shapes. Treat them as patterns, not drop-in code.

### Example 1: The bounded ReAct loop (`react.py`)
```python
# Pattern verified against /openai/openai-python (Context7, 2026-06-24):
# messages list, chat.completions.create(tools=...), message.tool_calls,
# {"role":"tool","tool_call_id":...,"content":...}.
def run(self, task: str, cwd: str, max_turns: int = 12) -> Transcript:
    messages = [
        {"role": "system", "content": self.system_prompt},   # steered OR control
        {"role": "user", "content": task},
    ]
    steps = []
    for turn in range(max_turns):
        msg = self.llm.chat(messages, tools=TOOL_SCHEMAS)      # llm.py
        if not msg.tool_calls:
            return Transcript(steps, final=msg.content, reason="done")
        messages.append(msg.model_dump(exclude_none=True))     # assistant msg FIRST
        for call in msg.tool_calls:
            result = run_verb(call, cwd=cwd)                   # tools.py → subprocess
            steps.append(Step(call.function.name, result.argv, result.exit, result.stdout))
            messages.append({"role": "tool", "tool_call_id": call.id,
                             "content": result.stdout[:STDOUT_CAP]})
        if self._no_progress(steps):                           # no-progress cap
            return Transcript(steps, final=None, reason="no_progress")
    return Transcript(steps, final=None, reason="max_turns")
```

### Example 2: Provider-agnostic chat with loud-fail + fallback (`llm.py`)
```python
import os
from openai import OpenAI            # dev venv only

DEFAULT_MODEL = os.environ.get("AGENT_MODEL", "deepseek-v4-flash")  # alias-safe
FALLBACK_MODEL = os.environ.get("AGENT_FALLBACK_MODEL", "gpt-4.1-mini")

class LLM:
    def __init__(self, provider="deepseek", model=DEFAULT_MODEL):
        if provider == "deepseek":
            key = os.environ.get("DEEPSEEK_API_KEY")
            if not key:                                  # LOUD fail on a real run
                raise RuntimeError("DEEPSEEK_API_KEY absent; a real agent run cannot proceed")
            self.primary = OpenAI(api_key=key, base_url="https://api.deepseek.com")
            self.model = model
        # OpenAI fallback constructed lazily on first failure
    def chat(self, messages, tools):
        try:
            return self.primary.chat.completions.create(
                model=self.model, messages=messages, tools=tools).choices[0].message
        except Exception:
            fb_key = os.environ.get("OPENAI_API_KEY")
            if not fb_key:
                raise RuntimeError("DeepSeek failed and OPENAI_API_KEY absent; no fallback")
            return OpenAI(api_key=fb_key).chat.completions.create(
                model=FALLBACK_MODEL, messages=messages, tools=tools).choices[0].message
```
*The hermetic test injects a fake `LLM` object (duck-typed `chat()`), so no key and no network are touched.* [VERIFIED: STACK.md Detail 1; loud-fail per CONTEXT.md/REQUIREMENTS.md]

### Example 3: Fixed-argv subprocess tool shim (`tools.py`)
```python
import json, subprocess

# Curated subset of the 51 canonical helix verbs (NOT all 51) — the routing-table
# verbs from CLAUDE.md (go-to-definition, find-references, search-symbols, read-file,
# replace-in-file, fuzzy-edit, get-symbol-overview, ...). Each becomes one function tool.
TOOL_SCHEMAS = [
    {"type": "function", "function": {
        "name": "find-references",
        "description": "Find all references to a symbol at relpath:line:col.",
        "parameters": {"type": "object",
            "properties": {"location": {"type": "string",
                "description": "relpath:line:col"}},
            "required": ["location"]}}},
    # ... more curated verbs ...
]

def run_verb(call, cwd, timeout=60):
    verb = call.function.name
    args = json.loads(call.function.arguments or "{}")
    argv = ["helix", verb, *_to_argv(verb, args)]      # deterministic argv map
    p = subprocess.run(argv, cwd=cwd, capture_output=True, text=True, timeout=timeout)
    return VerbResult(argv=argv, exit=p.returncode, stdout=p.stdout or p.stderr)
```
[VERIFIED: CONTEXT.md CLI-subprocess transport; verb inventory from README.md/verbs_gen.go]

### Example 4: Transcript / trace schema
```python
# A per-run JSON trace (AGENT-01). Deterministic enough for the hermetic test to assert on.
{
  "task": "...", "steering": "on|off", "model": "deepseek-v4-flash",
  "steps": [
    {"turn": 0, "verb": "find-references", "argv": ["helix","find-references","x.go:3:5"],
     "exit": 0, "stdout_tail": "..."}
  ],
  "final": "... or null",
  "reason": "done | max_turns | no_progress | tool_error_budget"
}
```

### Example 5: Steering ON/OFF with provably-omitted control (`react.py`)
```python
STEERING_SENTINEL = "<<HELIX_STEERING>>"   # distinctive token only ever in the steered prompt

OFF_CONTROL_PROMPT = (   # independent constant — NOT derived from the steering text
    "You are a coding agent. Use the available tools to complete the task. "
    "Stop when the task is done."
)
def build_system_prompt(steering_text: str | None, steering: str) -> str:
    if steering == "on":
        return f"{STEERING_SENTINEL}\n{steering_text}"
    return OFF_CONTROL_PROMPT
# Hermetic assertion (test_agent.py):
#   assert STEERING_SENTINEL in build_system_prompt(S, "on")
#   assert STEERING_SENTINEL not in build_system_prompt(S, "off")   # provably omitted
```
[VERIFIED: CONTEXT.md AGENT-03]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `deepseek-chat` / `deepseek-reasoner` model ids | `deepseek-v4-flash` / `deepseek-v4-pro` | aliases retire 2026/07/24 15:59 UTC | hard-coded `deepseek-chat` breaks; use a config var defaulting to `deepseek-v4-flash` [CITED: api-docs.deepseek.com] |
| Separate provider SDKs | One `openai` SDK + `base_url` for any OpenAI-compatible provider | stable | one client wraps DeepSeek + OpenAI [VERIFIED: STACK.md] |

**Deprecated/outdated:**
- `deepseek-chat`/`-reasoner`: compatibility aliases only until 2026/07/24 15:59 UTC.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `shell=True`-free fixed-argv subprocess is the right shim shape | Anti-Patterns, Example 3 | Low — standard secure practice; would only need a refactor, not a redesign |
| A2 | DeepSeek `deepseek-v4-flash` supports OpenAI-format function calling | Stack, Pitfall 1 | Medium — if flash lacks tool-calling, use `deepseek-v4-pro`; the model is a config var so cutover is one line. DeepSeek docs confirm tool-calling on the chat family but the *specific* `-v4-flash` tool-calling example should be confirmed at implementation time against api-docs.deepseek.com/guides/function_calling [CITED: STACK.md notes DeepSeek tool-calling support + historic flakiness] |
| A3 | A curated verb subset (routing-table verbs) is better than exposing all 51 | Structure, Example 3 | Low — fewer tools = clearer schema; if recall suffers, widen the set. STACK.md explicitly recommends "keep the toolset small and curated, NOT all 50" |
| A4 | `openai` legitimacy is HIGH despite the seam not being invokable this session | Package Legitimacy Audit | Low — first-party OpenAI SDK confirmed via Context7 + official repo |
| A5 | The no-progress heuristic = "last N identical argv+output" is sufficient | Pitfall 3, Example 1 | Low — tunable; any deterministic cap satisfies the locked "no-progress cap" requirement |

## Open Questions

1. **Exact `deepseek-v4-flash` tool-calling confirmation**
   - What we know: DeepSeek's chat family supports OpenAI-format `tools`/`tool_calls`; `deepseek-v4-flash` is the non-thinking successor to `deepseek-chat`.
   - What's unclear: whether a live `-v4-flash` call returns well-formed `tool_calls` on the first attempt (DeepSeek docs note historic tool-calling flakiness).
   - Recommendation: keep the model a config var; budget a retry + the OpenAI fallback; confirm with one live smoke call at implementation time (keys are present in the dev env). Strict-mode function calling is available at `https://api.deepseek.com/beta` with `"strict": true` if needed [CITED: api-docs.deepseek.com].

2. **Which verbs to expose as tools**
   - What we know: 51 canonical verbs exist; the CLAUDE.md routing table names the high-value subset.
   - What's unclear: the minimal set that lets the agent solve coding tasks without schema bloat.
   - Recommendation: start with the routing-table read/edit verbs (`go-to-definition`, `find-references`, `search-symbols`, `get-symbol-overview`, `read-file`, `replace-in-file`, `fuzzy-edit`, `insert-before/after-symbol`, `get-diagnostics`); widen if Phase 108 task-success is recall-limited. This is Phase-108 tunable, not a Phase-107 blocker.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Python 3 | the whole agent + test | ✓ | 3.13.5 | — |
| pip | venv install | ✓ | 25.1.1 | — |
| `openai` SDK | `llm.py` | ✗ (install via venv) | target 2.43.0 | none — required; `pip install` |
| `helix` binary | `tools.py` subprocess (real runs only) | build via `go build ./cmd/helix` | — | hermetic test uses a **fake `helix`** stub (no real binary needed for `test_agent.py`) |
| `DEEPSEEK_API_KEY` | real agent run (primary) | present in dev env (per CONTEXT.md); UNSET in this sandbox shell | — | OpenAI fallback; hermetic test needs neither |
| `OPENAI_API_KEY` | real agent run (fallback) | present in dev env (per CONTEXT.md) | — | none for fallback path; hermetic test needs neither |
| network | real runs only | n/a for tests | — | hermetic test is **no-network** by construction (fake LLM) |

**Missing dependencies with no fallback:** `openai` (install via `pip install -r requirements.txt` in the dev venv — never `go.mod`).
**Missing dependencies with fallback:** the `helix` binary and API keys are NOT needed for the hermetic `test_agent.py` (fake LLM + fake `helix`); they are needed only for a real run, which is outside the CI/executor gate.

## Validation Architecture

> `workflow.nyquist_validation: true` (config.json) — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | pytest 8.3.5 (already pinned) + plain-script runnability (the existing `test_*.py` convention) |
| Config file | none — tests live flat in `tools/dspy-tune/` and are pytest-discoverable + `python3 test_agent.py`-runnable |
| Quick run command | `pytest tools/dspy-tune/test_agent.py -x` |
| Full suite command | `pytest tools/dspy-tune/ -q` (existing split/parity/degenerate + new agent test) |
| Boundary gate | `make vet` (includes `vet-tools-quarantine`) + `git diff --exit-code go.mod` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AGENT-01 | bounded loop emits `helix` argv, observes stdout, captures transcript with a termination reason | unit (hermetic, fake LLM + fake helix) | `pytest tools/dspy-tune/test_agent.py::test_loop_emits_and_observes -x` | ❌ Wave 0 |
| AGENT-01 | no-progress cap deterministically terminates a repeating agent | unit | `pytest tools/dspy-tune/test_agent.py::test_no_progress_terminates -x` | ❌ Wave 0 |
| AGENT-02 | real run with the selected provider key absent FAILS loudly (raises) | unit (monkeypatch env, no client) | `pytest tools/dspy-tune/test_agent.py::test_missing_key_raises -x` | ❌ Wave 0 |
| AGENT-02 | model id is a config var defaulting to `deepseek-v4-flash` (alias-safe) | unit | `pytest tools/dspy-tune/test_agent.py::test_model_config_var -x` | ❌ Wave 0 |
| AGENT-02 | DeepSeek failure falls back to OpenAI client | unit (fake clients) | `pytest tools/dspy-tune/test_agent.py::test_provider_fallback -x` | ❌ Wave 0 |
| AGENT-03 | ON injects steering; OFF provably omits it | unit | `pytest tools/dspy-tune/test_agent.py::test_steering_on_off_omission -x` | ❌ Wave 0 |
| AGENT-03 (anti-vacuity) | a degenerate always-grep agent scores 0 (never emits a `helix` verb / never solves) | unit (break-the-invariant) | `pytest tools/dspy-tune/test_agent.py::test_degenerate_always_grep_scores_zero -x` | ❌ Wave 0 |
| ADOPT-04 (boundary) | no `go.mod` drift; `make vet` green; no `helix`→Python shell | gate | `make vet && git diff --exit-code go.mod` | ✅ (existing chain) |

### Anti-Vacuity (the required break-the-invariant → assert-RED pair)
The hermetic `test_agent.py` ships the discriminating pair (mirroring `test_degenerate.py`'s flag/no-flag and `test_parity.py`'s broken-classifier-diverges pattern):
1. **Degenerate scores 0:** a fake LLM that *always* emits a `grep` (or a non-`helix`, non-solving) action drives the loop; the assertion proves it terminates without success / emits no `helix` verb — i.e. a do-nothing agent cannot score. Break the invariant (make the loop coerce a default "solved") → the test goes RED.
2. **OFF provably omits steering:** `assert STEERING_SENTINEL in build_system_prompt(S,"on")` AND `assert STEERING_SENTINEL not in build_system_prompt(S,"off")`. Break the invariant (leak the steering into OFF) → RED.

### No-Network / Hermetic Discipline
- The test constructs NO `openai` client and reaches NO socket: it injects a **fake LLM** (duck-typed `chat()` returning canned tool-call/finish messages) and a **fake `helix`** (the subprocess shim is monkeypatched, OR a stub `helix` script on a temp `PATH`/cwd returns canned stdout/exit).
- No `DEEPSEEK_API_KEY`/`OPENAI_API_KEY` required (verified: this sandbox shell has them UNSET, and the test must still pass) — proving the loud-fail path and the hermetic path are distinct.

### Sampling Rate
- **Per task commit:** `pytest tools/dspy-tune/test_agent.py -x`
- **Per wave merge:** `pytest tools/dspy-tune/ -q && make vet && git diff --exit-code go.mod`
- **Phase gate:** full pytest suite green + `make vet` green + empty `go.mod` diff before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `tools/dspy-tune/test_agent.py` — covers AGENT-01/02/03 + the anti-vacuity pair (NEW)
- [ ] `tools/dspy-tune/agent/__init__.py`, `react.py`, `llm.py`, `tools.py` — the modules under test (NEW)
- [ ] `tools/dspy-tune/requirements.txt` — add `openai==2.43.0` (dev venv only)
- [ ] Dev-venv install of `openai` (`pip install -r requirements.txt`) for *real*-run smoke only; NOT required for the hermetic gate

*(Framework itself: pytest already pinned — no install gap.)*

## Project Constraints (from CLAUDE.md)

- **Single Go binary, no runtime Python:** the agent is strictly dev-time under `tools/`; nothing on `go.mod`, `helix setup`, default `go test ./...`, or the merge path.
- **`make vet` + `go test ./...` before completing any Go task** — but Phase 107 adds **no Go code**, so the relevant gate is `make vet` (must stay green, proving no Go regression) + the Python pytest suite. Run `go vet ./...` / `go test ./...` only to prove the Go side is unchanged.
- **Container engine = Podman** (not Docker) — irrelevant to Phase 107 (no containers); relevant to Phases 108–109.
- **GSD workflow enforcement:** edits go through a GSD command.
- **`toolsquarantine`** (`vet-tools-quarantine`, the 7th vettool) must stay green — it fires only on a Go import edge into `tools/`, which Phase 107 does not create.

## Security Domain

> `security_enforcement` not explicitly disabled — included; scope is narrow (dev-time tooling, no production surface, no untrusted network input).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No user auth surface (dev-time tool) |
| V3 Session Management | no | none |
| V4 Access Control | no | none |
| V5 Input Validation | yes (light) | Tool-call arguments are decoded with `json.loads` and mapped to a fixed argv list; never `shell=True`; never string-concatenated into a shell. Cap stdout fed back to the model. |
| V6 Cryptography | no | API keys read from env, never logged/committed; never embedded in the transcript |
| V7 Secrets | yes | `DEEPSEEK_API_KEY`/`OPENAI_API_KEY` from env only; never written to `output/`, the transcript, or git. Mirror the existing harness's env-only-key discipline. |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Shell injection via tool-call arguments | Tampering / EoP | Fixed-argv `subprocess.run([...])`, never `shell=True`; whitelist verbs via the tool schema (the model can only call the curated verb set) |
| API-key leakage into transcript/output | Information disclosure | Keys read from env only; transcript records argv + stdout, never env; never log the key |
| Unbounded resource use (runaway loop) | DoS | `max_turns` + no-progress cap + per-subprocess `timeout=` |
| Agent shelling arbitrary commands | EoP | The shim only ever runs `helix <verb>`; the verb is constrained to the schema's `name` set, args mapped through a fixed argv map — no free-form command path |

## Sources

### Primary (HIGH confidence)
- `/openai/openai-python` (Context7, 2026-06-24, Source Reputation High, 584 snippets) — `chat.completions.create(tools=...)`, `message.tool_calls`, `ChatCompletionToolMessageParam` (`role:"tool"` + `tool_call_id`).
- api-docs.deepseek.com (WebFetch, 2026-06-24) — base_url `https://api.deepseek.com`; models `deepseek-v4-flash`/`-pro`; `deepseek-chat`/`-reasoner` alias retirement **2026/07/24 15:59 UTC**; Anthropic-format base at `/anthropic`.
- Live source (read 2026-06-24): `tools/dspy-tune/{optimize.py,scorer.py,requirements.txt,test_split.py,test_parity.py,test_degenerate.py}`; `bench/runtime/subprocess/claude.go`; `internal/cli/verbs_gen.go` + `README.md` (51-verb inventory); `.planning/config.json`; `Makefile` vet chain.
- v2.3 milestone research (HIGH, 2026-06-24): `.planning/research/{SUMMARY,STACK,ARCHITECTURE}.md` — agent placement, single-`openai`-client decision, in-process-metric/subprocess pattern, quarantine semantics, DeepSeek/PyPI version probes.

### Secondary (MEDIUM confidence)
- Host probe (this session): `python3 3.13.5`, `pip 25.1.1`; env keys UNSET in the sandbox shell (CONTEXT.md states they are present in the dev environment).

### Tertiary (LOW confidence)
- DeepSeek `-v4-flash` specific tool-calling first-attempt reliability — confirm with a live smoke call at implementation time (Open Question 1).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `openai`/DeepSeek/Python versions verified vs PyPI + official docs + Context7; one net-new pin, a first-party SDK.
- Architecture: HIGH — agent placement, in-process-metric pattern, and quarantine semantics read from source in the upstream v2.3 research; only the Python agent is greenfield.
- Pitfalls: HIGH — grounded in CONTEXT.md locked constraints, the existing harness's loud-fail/anti-vacuity patterns, and the verified DeepSeek deprecation date.

**Research date:** 2026-06-24
**Valid until:** 2026-07-20 (the DeepSeek alias retirement on 2026/07/24 is the hard expiry for the `deepseek-chat` fallback value; the `deepseek-v4-flash` default is durable past it).
