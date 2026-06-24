# Phase 113: The Real Cost-Aware Run - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (skip_discuss) + investigation findings

<domain>
## Phase Boundary
Execute the v2.3 pipeline for real on the grown corpus — the headline billed phase — producing a real optimized program (RUN-01), an honest ON/OFF attribution delta with per-arm cost (RUN-02), and a SWE-bench Verified confirming agreement (RUN-03). Then Phase 114 turns these into the REPORT.
</domain>

<code_context>
## Environment probe (2026-06-24)
- **DeepSeek auth WORKS** (`deepseek-v4-flash` responded to a 1-token probe) — the run is viable on the primary provider. The OpenAI fallback key is invalid (401), but it is only the fallback; resolve_lm picks DeepSeek primary. The agent's llm.py falls back to OpenAI only on primary failure, which would 401 — so the run depends on DeepSeek staying up (acceptable; surfaced as a blocker if it drops mid-run).
- `helix` binary built + on PATH (`/home/john/bin/helix`). The agent shells `helix <verb>` via fixed-argv subprocess in the sandbox cwd (cold-starts the daemon on first call). Curated verbs: read (go-to-definition, find-references, search-symbols, get-symbol-overview, read-file, get-diagnostics) + edit (replace-in-file, fuzzy-edit, insert-before/after-symbol).
- Corpus materialized at `tools/dspy-tune/corpus/` (103 tasks: go 39 / python 34 / rust 30); `split_corpus(103)` → train 26 / val 26 / heldout 51 (>50). Graders shell `go test ./...` / `pytest` / `cargo test -- --include-ignored` (all toolchains present).
- Cost (research): GEPA `auto="light"` ≈600 rollouts; each rollout = one agent task-run (ReAct + native test). The attribution ON/OFF over the 51-task held-out split = 102 agent runs. Cost lever = `GEPA_MAX_METRIC_CALLS` + `AGENT_MAX_TURNS`.

## Cost-aware decisions
- **De-risk first**: a 1-task end-to-end smoke (agent OFF → grade) before any bulk spend; if the pipeline isn't wired end-to-end, that's fix-as-needed (in scope) or a surfaced blocker.
- **Bound the GEPA run**: `GEPA_MAX_METRIC_CALLS` modest (explicit hard cap) + `AGENT_MAX_TURNS` modest, so RUN-01 produces a real `optimized.json` + `val_size>50` proof without a multi-hour spend.
- **Attribution (RUN-02)**: the honest ON/OFF over the full sequestered held-out split is the trustworthy measure; a low absolute success rate is FINE — the milestone wants a real, honest delta/verdict, not necessarily a positive one.
- **SWE-bench (RUN-03)**: `podman system service --time=0 &` + `DOCKER_HOST`; K≈2 gold-patch smoke first; then a stratified K=25–50 slice. Fail-not-skip on a requested run that yields no result. Heaviest leg (image pulls ~3–5 GiB); if Podman/images unavailable it is a surfaced blocker, not a silent skip.
</code_context>

<decisions>
1. Add a reusable `run_real.py` dev-time driver (smoke + full ON/OFF attribution over the sequestered held-out), reusing `agent.ReActAgent`, `sandbox`, `grade_aider`, and `attribution.compute_attribution` + `MeteredLLM`.
2. Cost caps applied via env (`GEPA_MAX_METRIC_CALLS`, `AGENT_MAX_TURNS`, `GEPA_NUM_THREADS`) — Phase 112 surfaces.
3. All run artifacts land git-ignored under `tools/dspy-tune/output/` (optimized.json, heldout_test.json, attribution.json, swebench reports) for the Phase 114 REPORT.
4. Boundary (ADOPT-04): the run shells helix + the upstream harness as subprocesses only; zero new Go deps.
</decisions>

<specifics>
- New: `tools/dspy-tune/run_real.py` (smoke + attribution driver, gated like optimize.py/attribution.py).
- Outputs: `output/optimized.json` (RUN-01), `output/attribution.json` (RUN-02), `output/swebench_confirm.json` (RUN-03).
</specifics>

<deferred>
- The REPORT.md fill + boundary re-verify → Phase 114.
</deferred>
