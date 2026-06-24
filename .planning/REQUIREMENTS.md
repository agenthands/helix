# Requirements: Helix — v2.3 Task-Success-Driven Skill Optimization

**Defined:** 2026-06-24
**Core Value:** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Driver:** TUNE-FUT-02 — replace the gameable `choice_rate` adoption proxy (v2.2 no-ship) with a real **agent task-success** optimization signal measured by a net-new tool-using agent driving the `helix` CLI.

## v2.3 Requirements

### Tool-Using Agent (AGENT)

- [ ] **AGENT-01**: A dev-time tool-using agent drives the `helix` CLI via **CLI subprocess verbs** (the product surface) in a bounded ReAct-style loop, observing each verb's output and capturing a per-run transcript/trace.
- [ ] **AGENT-02**: The agent's LM is config-driven via one OpenAI-compatible client — **DeepSeek primary** (explicit `deepseek-v4-*` model id, alias-deprecation-safe), **OpenAI fallback** — with a hard turn / no-progress cap and a loud failure (not a silent skip) when the selected provider's API key is absent on a real run.
- [ ] **AGENT-03**: The agent exposes a first-class **steering ON/OFF** switch — the candidate skill text injected as the system prompt (ON) versus a provably steering-omitted control prompt (OFF) — so any measured effect is attributable to the skill text.

### Honest Task-Success Oracle (ORACLE)

- [ ] **ORACLE-01**: Aider-polyglot task-success is graded by running the exercise's native hidden tests in a **per-task sandbox**; a run where **0 tests executed is a hard ERROR** (no vacuous pass), and gold test files are restored from an agent-unwritable path before grading (agent test-tampering cannot force green).
- [ ] **ORACLE-02**: SWE-bench task-success is graded via the **upstream harness on Podman** (`podman system service` + `DOCKER_HOST`→podman socket), enforcing the **FAIL_TO_PASS + PASS_TO_PASS** resolution contract; the dataset id is pinned to the correct org and fetch-resolution plus a nonzero task-count are asserted before any run.

### Task-Success Optimization Metric (TUNE)

- [ ] **TUNE-02**: The DSPy GEPA optimization metric is rewired from `choice_rate` to **agent task-success** on the benchmark corpus (`choice_rate` retained at most as a non-optimized diagnostic pre-screen, never a co-optimized reward).
- [ ] **TUNE-03**: The optimization corpus enforces a **sequestered held-out TEST split** never passed to `compile()`, with **`val_size > 50`** as a hard precondition gate for any adoption recommendation (closes the v2.2 tiny-corpus no-ship root cause; honors TUNE-FUT-01).
- [ ] **TUNE-04**: Any reported improvement is an **ON-vs-OFF attribution delta** (`success(ON) − success(OFF)`) measured on the held-out split and recorded in a ship/no-ship REPORT alongside `val_size` and per-arm cost.

### Adoption & Boundary (ADOPT)

- [ ] **ADOPT-03**: Optimized skill text is adopted **only** via a human-reviewed `SKILL.md` edit gated by `helix-refgen --check` (the optimizer writes only git-ignored output; never auto-writes `SKILL.md`/`reference.md`); the `## Decision matrix` anchor and SKILL size cap are preserved.
- [ ] **ADOPT-04**: The **single-binary / no-runtime-Python invariant** is preserved — zero new Go module dependencies; no `helix` subcommand shells to Python; the agent/optimizer stay off `go.mod`, `helix setup`, the default `go test ./...`, and the merge path; the `make vet` import-boundary (`toolsquarantine`) analyzer stays green.

> **Cross-cutting (applies to every requirement's exit gate):** **Anti-vacuity** — each gate a phase adds (vacuous-pass refusal, test-tamper restore, TEST-split sequestration, `val_size>50` precondition, ON/OFF attribution, `--check` adoption gate, boundary analyzer) MUST ship a deliberate break-the-invariant → assert-RED test; a green-path-only gate is presumed broken. Fold code-review + fix BEFORE verify (the repeated v2.2 vacuous-pass / parity-bug class). **HELIX_BIN fail-not-skip** — any bench surface that requires `HELIX_BIN` FAILS loudly when it's set but the run produced no result, rather than silently skipping.

## Future Requirements (deferred)

- **TUNE-FUT-01**: upgrade the DSPy optimizer from GEPA to MIPROv2/COPRO with a larger held-out set if GEPA's prose evolution proves insufficient (gated on a corpus big enough for minibatch overfit protection, `val_size > 50`). *(Partially activated by TUNE-03's `val_size>50` gate; the optimizer upgrade itself remains deferred.)*
- **TUNE-FUT-03**: a grep-sed-cat-only third control arm and a dual-bench *joined* metric (Aider AND SWE-bench co-weighted) once the single-bench loop is proven.
- **TUNE-FUT-04**: larger SWE-bench instance lists / additional providers beyond DeepSeek+OpenAI.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Runtime Python / shipping DSPy or the agent in the `helix` binary, `helix setup`, or default `go test ./...` / merge path | Single-binary identity is non-negotiable; the agent + optimizer are strictly dev-time/offline (ADOPT-04). |
| Reusing `bench/runtime/subprocess/claude.go` (Claude Code) as the agent | `ANTHROPIC_API_KEY` is unset; that path delegates to the `claude` binary's own loop. The new agent owns its ReAct loop and shells `helix` directly. |
| A Go-resident LLM agent in `bench/runtime` | GEPA calls its metric in-process per candidate; a Go LLM client would force a new runtime `go.mod` dep and a process-spawn handoff. Agent lives in Python under `tools/dspy-tune/`. |
| Hand-editing generated `reference.md` | Fails `--check`; all reference corrections go through `cmd/helix-refgen`. |
| New Go module dependencies | All new deps are dev-time Python in the `tools/dspy-tune/` venv only. |
| Auto-adopting optimizer output into `SKILL.md` | Human-gated adoption only (ADOPT-03); LLM optimization is not bit-reproducible — gate the committed artifact, never the process. |
| `choice_rate` as a (co-)optimization target | Gameable first-command proxy that caused the v2.2 no-ship; demoted to at most a diagnostic pre-screen. |
| Changing the nudge-steering classifier behavior | v2.1 owns the exit-0 advisory classifier; v2.3 touches only the skill TEXT and the offline harness. |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| AGENT-01 | Phase 107 | Pending |
| AGENT-02 | Phase 107 | Pending |
| AGENT-03 | Phase 107 | Pending |
| ORACLE-01 | Phase 108 | Pending |
| ORACLE-02 | Phase 109 | Pending |
| TUNE-02 | Phase 108 | Pending |
| TUNE-03 | Phase 108 | Pending |
| TUNE-04 | Phase 109 | Pending |
| ADOPT-03 | Phase 110 | Pending |
| ADOPT-04 | Phase 110 | Pending |

> **Cross-cutting recurrence (informational; primary owners above):** ADOPT-04 (single-binary / no-runtime-Python boundary) is re-verified as an exit gate in **every** phase (107–110) and owned for traceability by Phase 110. The anti-vacuity (break-the-invariant → assert-RED) and HELIX_BIN-fail-not-skip disciplines recur in every phase that adds a gate or touches a bench surface (107, 108, 109).
