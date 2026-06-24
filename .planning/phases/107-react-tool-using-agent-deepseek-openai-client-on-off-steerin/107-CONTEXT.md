# Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

A dev-time, OpenAI-compatible tool-using agent drives the real `helix` CLI product surface in a bounded ReAct loop, with config-driven provider selection and a first-class steering ON/OFF switch — the foundation every grader and the metric rewire depend on.

Requirements: AGENT-01 (agent drives `helix <verb>` via CLI subprocess in a bounded ReAct loop + transcript), AGENT-02 (config-driven LM — DeepSeek primary / OpenAI fallback via one OpenAI-compatible client; turn / no-progress cap; loud failure on missing key for a real run), AGENT-03 (first-class steering ON/OFF switch — candidate skill text as system prompt vs a provably steering-omitted control prompt).

Lives entirely dev-time in Python under `tools/dspy-tune/agent/` (sibling of the existing `scorer.py`). Zero new Go module deps; nothing on `go.mod` / `helix setup` / default `go test ./...` / the merge path; `make vet` (`toolsquarantine`) stays green.
</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, the v2.3 research (`.planning/research/SUMMARY.md` + STACK/FEATURES/ARCHITECTURE/PITFALLS), and codebase conventions to guide decisions.

### Locked constraints (from milestone setup — not discretionary)
- Agent lives in Python under `tools/dspy-tune/agent/` (NOT Go `bench/runtime`) — GEPA calls its metric in-process per candidate.
- One `openai==2.43.0` client wraps both providers: DeepSeek primary (`base_url=https://api.deepseek.com`, explicit `deepseek-v4-*` model id as a config var — the `deepseek-chat`/`-reasoner` aliases retire 2026-07-24), OpenAI fallback.
- Agent invokes helix via **CLI subprocess verbs** (the product surface), e.g. `helix <verb> ...`.
- Hard turn / no-progress cap; deterministic termination; per-run transcript/trace capture.
- A real run with the selected provider's API key absent FAILS loudly (not a silent skip). `DEEPSEEK_API_KEY` + `OPENAI_API_KEY` are present in this environment.
- First-class `--steering on|off`: ON injects candidate skill text as the system prompt; OFF uses a provably steering-omitted control prompt.
- **Anti-vacuity gate (required):** ship a deliberate break-the-invariant → assert-RED test — a hermetic `test_agent.py` (fake LLM + fake `helix`, no network) where a degenerate always-grep agent scores 0 and the OFF prompt provably omits the steering text. Fold code-review + fix BEFORE verify.
- **Boundary gate (ADOPT-04 cross-cutting):** `git diff go.mod` empty; `make vet` (`toolsquarantine`) green; no `helix` subcommand shells to Python.
</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Known reusable building blocks: `tools/dspy-tune/{optimize.py,scorer.py,requirements.txt}` (existing DSPy GEPA harness; `DSPY_LM_MODEL` env pattern), `bench/runtime/subprocess/claude.go` (the existing Claude-Code agent — pattern reference only, NOT reused), the `helix` binary CLI surface (`internal/cli/verbs_gen.go` for the verb inventory), and `internal/lint/toolsquarantine` (the import-boundary analyzer that must stay green).
</code_context>

<specifics>
## Specific Ideas

No additional specific requirements beyond the locked constraints above — discuss phase skipped. Refer to the ROADMAP phase description, success criteria, and `.planning/research/SUMMARY.md` (Phase 107 section: `react.py`, `llm.py`, `tools.py` under `tools/dspy-tune/agent/`).
</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.
</deferred>
