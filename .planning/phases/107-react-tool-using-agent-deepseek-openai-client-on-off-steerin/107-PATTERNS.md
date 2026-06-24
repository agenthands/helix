# Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering - Pattern Map

**Mapped:** 2026-06-24
**Files analyzed:** 7 (6 new, 1 modified)
**Analogs found:** 7 / 7

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `tools/dspy-tune/agent/__init__.py` | package init / export barrel | n/a (re-export) | `tools/dspy-tune/scorer.py` (module-level public API: `score_choice_rate` thin wrapper) | role-match |
| `tools/dspy-tune/agent/llm.py` | service (LLM client) | request-response (HTTPS) | `tools/dspy-tune/optimize.py` (env-key + model-config-var + lazy-import + loud-vs-quiet guard) | role+flow match |
| `tools/dspy-tune/agent/react.py` | service (agent loop) | event-driven (think→act→observe) | `bench/runtime/subprocess/claude.go` (bounded agent loop, MaxToolCalls cap) — reference only; `optimize.py` for env/config/docstring conventions | partial (cross-language) |
| `tools/dspy-tune/agent/tools.py` | utility (subprocess shim) | request-response (argv → stdout) | `bench/runtime/subprocess/claude.go` (`exec.Command` argv discipline) reference; `scorer.py` for pure-function module shape | role-match |
| `tools/dspy-tune/agent/test_agent.py` | test (hermetic) | n/a | `tools/dspy-tune/test_degenerate.py` + `test_parity.py` (break-the-invariant pair, no-fixture pytest, script-runnable) | exact |
| `tools/dspy-tune/agent/conftest.py` | test config / fixtures | n/a | NO ANALOG (existing tests use NO fixtures — see "No Analog Found") | none |
| `tools/dspy-tune/requirements.txt` | config | n/a | itself (additive edit) | exact |

> NOTE on test location: the research/CONTEXT pin `test_agent.py` at `tools/dspy-tune/test_agent.py` (flat, sibling of `scorer.py`), matching the existing flat `test_*.py` convention. The orchestrator's file list says `tools/dspy-tune/agent/test_agent.py`. Planner should reconcile; the **flat sibling location** matches the existing pytest-discovery convention exactly. Likewise `conftest.py` is not part of the existing convention (tests are fixture-free) — only create it if a fixture is genuinely shared; otherwise inline fakes per the `test_degenerate.py` pattern.

## Pattern Assignments

### `tools/dspy-tune/agent/llm.py` (service, request-response)

**Analog:** `tools/dspy-tune/optimize.py` (env-key, model-config-var, lazy-import discipline)

**Model-as-config-var pattern** (optimize.py:33-34) — mirror `DSPY_LM_MODEL`:
```python
# Dev-time LM backend. Illustrative; any litellm-supported provider works.
LM_MODEL = os.environ.get("DSPY_LM_MODEL", "openai/gpt-4.1-mini")
```
New file uses the SAME `os.environ.get(VAR, default)` shape:
`AGENT_MODEL` default `deepseek-v4-flash` (alias-safe — `deepseek-chat` retires 2026/07/24 15:59 UTC), `AGENT_FALLBACK_MODEL` default `gpt-4.1-mini`.

**Env-key guard — INVERT the optimize.py behavior** (optimize.py:48-62): `optimize.py` does "key unset → print + `return 0`" (quiet skip, for the *optimizer entrypoint*). The agent's real `run()` path MUST do the OPPOSITE — **raise loudly** (RESEARCH Pitfall 2):
```python
key = os.environ.get("DEEPSEEK_API_KEY")
if not key:                                  # LOUD fail on a real run
    raise RuntimeError("DEEPSEEK_API_KEY absent; a real agent run cannot proceed")
```
Keep any "exit 0 on unset key" behavior ONLY at an optional `__main__` convenience wrapper, never inside `run()`/`chat()`.

**Lazy-import discipline** (optimize.py:64-69): `optimize.py` imports `dspy`/`scorer` lazily so the unset-key guard works without the dep installed. `llm.py` should `from openai import OpenAI` at module top (it is the agent's hard dep) — but the hermetic test injects a **duck-typed fake `LLM`** so it never constructs a client (RESEARCH Example 2, line 298).

**Provider fallback** (RESEARCH Example 2, lines 287-296): try/except around primary `create`, retry on the lazily-constructed OpenAI fallback client; raise if `OPENAI_API_KEY` also absent.

---

### `tools/dspy-tune/agent/tools.py` (utility, request-response)

**Analog:** `bench/runtime/subprocess/claude.go` (argv discipline, reference only) + `scorer.py` (pure-function module shape)

**Fixed-argv subprocess shim** (RESEARCH Example 3 / Pattern 2, lines 318-323) — NEVER `shell=True`, always a list:
```python
def run_verb(call, cwd, timeout=60):
    verb = call.function.name
    args = json.loads(call.function.arguments or "{}")
    argv = ["helix", verb, *_to_argv(verb, args)]   # deterministic argv map
    p = subprocess.run(argv, cwd=cwd, capture_output=True, text=True, timeout=timeout)
    return VerbResult(argv=argv, exit=p.returncode, stdout=p.stdout or p.stderr)
```
`claude.go` precedent: validates an **absolute** `--helix-bin` path (claude.go:68) and bounds via `MaxToolCalls` — the Go analog for argv hygiene + budget axis.

**Curated verb subset** (RESEARCH A3 / Open Q2): expose the CLAUDE.md routing-table verbs (`go-to-definition`, `find-references`, `search-symbols`, `get-symbol-overview`, `read-file`, `replace-in-file`, `fuzzy-edit`, `insert-before-symbol`, `insert-after-symbol`, `get-diagnostics`) as `TOOL_SCHEMAS`, NOT all 51. Canonical verb names live in `internal/cli/verbs_gen.go` (kebab form).

**Module-shape convention** (scorer.py): thin, pure, well-docstringed; `score_choice_rate` (scorer.py:86-92) is the "named entry point wraps internal" precedent for `run_verb` wrapping the subprocess call.

---

### `tools/dspy-tune/agent/react.py` (service, event-driven loop)

**Analog:** `bench/runtime/subprocess/claude.go` (bounded loop + `MaxToolCalls`, reference only — NOT reused) + `optimize.py` (docstring/quarantine header convention)

**Bounded ReAct loop** (RESEARCH Example 1, lines 249-267): growing `messages` list; assistant tool-call message appended BEFORE tool-result messages (Pattern 1 critical detail); `{"role":"tool","tool_call_id":...,"content":...}` per call; terminate on `done | max_turns | no_progress | tool_error_budget`.

**Two-bound termination** (RESEARCH Pitfall 3, mirrors claude.go:47-48 `MaxToolCalls`): hard `max_turns` cap AND a no-progress heuristic (last-N identical argv+output). Record the termination *reason* in the transcript.

**Steering ON/OFF with provably-omitted control** (RESEARCH Example 5, lines 343-355):
```python
STEERING_SENTINEL = "<<HELIX_STEERING>>"   # only ever in the steered prompt
OFF_CONTROL_PROMPT = (   # independent constant — NOT derived from steering text
    "You are a coding agent. Use the available tools to complete the task. "
    "Stop when the task is done."
)
def build_system_prompt(steering_text, steering: str) -> str:
    if steering == "on":
        return f"{STEERING_SENTINEL}\n{steering_text}"
    return OFF_CONTROL_PROMPT
```
Anti-pattern to avoid (Pitfall 4): building OFF by redacting the steering string.

**Quarantine docstring header** (optimize.py:1-22): copy the "lives under tools/ — NEVER linked into the helix binary / go.mod / `go test ./...`" preamble so the dev-time boundary is self-documenting.

---

### `tools/dspy-tune/agent/test_agent.py` (test, hermetic)

**Analog:** `tools/dspy-tune/test_degenerate.py` (break-the-invariant pair) + `test_parity.py` (planted-divergence anti-vacuity, no-fixture, script-runnable)

**No-fixture, script-runnable convention** (test_parity.py:94-97, test_degenerate.py:136-140) — every existing test is pytest-discoverable AND runs as `python3 test_X.py`:
```python
if __name__ == "__main__":
    test_python_go_parity()
    test_broken_classifier_diverges()
    print("parity OK")
```

**Break-the-invariant PAIR** (test_degenerate.py:105-133 flag/no-flag; test_parity.py:61-91 planted divergence). For Phase 107 the pair is (RESEARCH Validation Architecture):
1. `test_degenerate_always_grep_scores_zero` — a fake LLM that always emits a `grep`/non-`helix` action drives the loop; assert it terminates without success / emits no `helix` verb.
2. `test_steering_on_off_omission`:
```python
assert STEERING_SENTINEL in build_system_prompt(S, "on")
assert STEERING_SENTINEL not in build_system_prompt(S, "off")   # provably omitted
```

**Hermetic / no-network discipline** (RESEARCH lines 437-438): inject a duck-typed fake `LLM` (canned `chat()` returns) and a fake `helix` (monkeypatch the shim OR a stub script on a temp PATH). Construct NO `openai` client, reach NO socket; no API keys required (this sandbox shell has them UNSET, test must still pass).

**Deliberately-wrong control** (test_parity.py:36-43 `_broken_classify`): the precedent for "ship the broken variant and assert it diverges/RED" — apply to the loop-coerces-default-solved break for AGENT-01.

Test→behavior map (RESEARCH lines 422-428): `test_loop_emits_and_observes`, `test_no_progress_terminates`, `test_missing_key_raises`, `test_model_config_var`, `test_provider_fallback`, `test_steering_on_off_omission`, `test_degenerate_always_grep_scores_zero`.

---

### `tools/dspy-tune/agent/__init__.py` (package barrel)

**Analog:** `scorer.py` public-API convention. Export `ReActAgent`, `run()` (RESEARCH structure, lines 156-160). Keep it thin — re-export only; mirrors how `optimize.py` does `from scorer import score_choice_rate` (optimize.py:69) so the future Phase-108 metric can `from agent import ReActAgent` in-process.

---

### `tools/dspy-tune/requirements.txt` (config, MODIFIED)

**Analog:** itself — additive edit only. Current pins (lines 11-12):
```
dspy==3.2.1
pytest==8.3.5
```
Add ONE net-new pin with the existing dev-time-only comment discipline (lines 1-3):
```
openai==2.43.0       # NEW (Phase 107): agent tool-use client for DeepSeek + OpenAI
```
Keep existing pins. Boundary gate: `git diff go.mod` must stay empty.

---

## Shared Patterns

### Dev-time quarantine header
**Source:** `tools/dspy-tune/optimize.py:1-22`
**Apply to:** `llm.py`, `react.py`, `tools.py`
The module docstring states the file lives under `tools/` and is NEVER linked into the helix binary, `helix setup`, `go.mod`, or `go test ./...`. Self-documents the ADOPT-04 boundary gate.

### Env-var config (read-with-default)
**Source:** `tools/dspy-tune/optimize.py:34` (`DSPY_LM_MODEL`)
**Apply to:** `llm.py`
`os.environ.get(VAR, default)` for `AGENT_MODEL` / `AGENT_FALLBACK_MODEL`; keys (`DEEPSEEK_API_KEY`, `OPENAI_API_KEY`) read from env ONLY, never logged/committed/written to transcript (Security V7).

### Loud-fail vs quiet-skip discipline (INVERTED per path)
**Source:** `tools/dspy-tune/optimize.py:48-62` (quiet skip at the optimizer entrypoint)
**Apply to:** `llm.py`/`react.py` real `run()` path RAISES (Pitfall 2); only an optional `__main__` wrapper may keep the quiet-skip-exit-0 convenience.

### No-fixture, script-runnable pytest
**Source:** `tools/dspy-tune/test_parity.py:94-97`, `test_degenerate.py:136-140`
**Apply to:** `test_agent.py` — pytest-discoverable AND `python3 test_agent.py`-runnable; inline fakes rather than `conftest.py` fixtures.

### Break-the-invariant anti-vacuity pair
**Source:** `tools/dspy-tune/test_degenerate.py` (flag/no-flag) + `test_parity.py:61-91` (planted divergence)
**Apply to:** `test_agent.py` — degenerate-scores-0 + OFF-provably-omits-steering; fold code-review + fix BEFORE verify (CONTEXT locked constraint).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `tools/dspy-tune/agent/conftest.py` | test config / fixtures | n/a | The existing `tools/dspy-tune/test_*.py` suite uses NO pytest fixtures and NO `conftest.py` (verified: `test_parity.py`, `test_degenerate.py`, `test_split.py` are all fixture-free, inline-helper, script-runnable). Planner should prefer inlined fakes per the `test_degenerate.py` pattern and create `conftest.py` ONLY if a fake LLM / fake-`helix` fixture is genuinely shared across multiple test modules. If created, model it on the duck-typed fake-`LLM` + stub-`helix` design in RESEARCH lines 437-438. |
| (loop internals of) `react.py` no-progress heuristic | service | event-driven | No Python loop analog exists in-tree; the only loop analog is the Go `claude.go` (`MaxToolCalls`, reference only). Genuinely greenfield logic — use RESEARCH Example 1 + Pitfall 3 as the spec. |

## Metadata

**Analog search scope:** `tools/dspy-tune/` (optimize.py, scorer.py, requirements.txt, test_parity.py, test_degenerate.py); `bench/runtime/subprocess/claude.go`; `internal/cli/verbs_gen.go` (verb inventory, referenced not read).
**Files scanned:** 6 read in full + 1 grepped (claude.go).
**Pattern extraction date:** 2026-06-24
