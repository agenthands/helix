# Roadmap: Helix

## Milestones

- [x] **v1.0 MVP** -- Phases 1-5 (shipped 2026-04-08)
- [x] **v1.1 Integration Testing** -- Phases 6-8 (shipped 2026-04-09)
- [x] **v1.2 Performance & Production Hardening** -- Phases 9-15 (shipped 2026-04-10)
- [x] **v1.3 Documentation Catchup** -- Phases 16-17 (shipped 2026-04-11)
- [x] **v1.4 Integration Testing v2** -- Phases 18-21 (shipped 2026-04-14)
- [x] **v1.5 Typed Errors & Hardening** -- Phases 22-24 (shipped 2026-04-15)
- [x] **v1.6 Context Intelligence & Resilient Editing** -- Phases 25-33 (shipped 2026-04-20)
- [x] **v1.7 Developer Experience & Auto-Setup** -- Phases 34-38 (shipped 2026-04-22)
- [x] **v1.8 Documentation Overhaul** -- Phases 39-45 (shipped 2026-04-24)
- [x] **v1.9 Polish & Infra** -- Phases 46-56 (shipped 2026-05-03)
- [x] **v1.10 Live Semantic Index** -- Phases 57-67 (shipped 2026-05-12)
- [x] **v1.11 Semantic Index Completion & P1 MCP Tools** -- Phases 68-74 (shipped 2026-06-07) — see `.planning/milestones/v1.11-ROADMAP.md`
- [x] **v1.12 Bench Stack & Tool Evaluation** -- Phases 75-89 (shipped 2026-06-21) — see `.planning/milestones/v1.12-ROADMAP.md`
- [x] **v2.0 CLI-First — MCP Surface Retirement** -- Phases 90-96 (shipped 2026-06-22) — see `.planning/milestones/v2.0-ROADMAP.md`
- [x] **v2.1 Agent Adoption & Aider-Derived Validation** -- Phases 97-102 (shipped 2026-06-23) — see `.planning/milestones/v2.1-ROADMAP.md`
- [x] **v2.2 Agent-Facing Skill Quality & Prompt Tuning** -- Phases 103-106 (shipped 2026-06-24) — see `.planning/milestones/v2.2-ROADMAP.md`
- [x] **v2.3 Task-Success-Driven Skill Optimization** -- Phases 107-110 (shipped 2026-06-24) — see `.planning/milestones/v2.3-ROADMAP.md`
- [ ] **v2.4 Corpus Growth & Real Optimization Verdict** -- Phases 111-114 (in progress)

## Phases

### 🚧 v2.4 Corpus Growth & Real Optimization Verdict (Phases 111-114) — IN PROGRESS

4 phases, 10 requirements (CORPUS-01/02, SCALE-01/02/03, RUN-01/02/03, REPORT-01, ADOPT-05), 100% mapped. Grow the optimization corpus past the strict `val_size > 50` held-out gate and run the v2.3 task-success pipeline **for real** (cost-aware, both tracks — Aider-polyglot primary + SWE-bench_Verified confirming) to turn v2.3's "NO-SHIP by design" into an actual, numbers-backed adopt/no-adopt verdict. **REPORT-only**: adoption stays a separate human `helix-refgen --check`-gated step. Reuses the v2.3 `tools/dspy-tune/` pipeline (agent + Aider/SWE-bench graders + GEPA metric); **fix-as-needed** latitude on defects corpus-scale execution exposes. **Zero new Go deps, no runtime Python** — all new pins are dev-venv Python only (`dspy==3.2.1`, `openai==2.43.0`, `swebench==4.1.0`).

**Forced dependency chain (do not reorder):** corpus growth + sequestered split (111) → scale hardening (112) → the real billed run (113) → verdict + boundary re-verify (114). Each gate ships a break-the-invariant → assert-RED test; code-review + fix folds in BEFORE verify. Research (`.planning/research/SUMMARY.md`) pinned the model id (`deepseek-v4-flash`), the GEPA cost model (~$5–15/run; trainset free, valset is the lever), and SWE-bench logistics (K=25–50 stratified Verified slice; the harness "0-tests ⇒ resolved" footgun).

- [x] Phase 111: Corpus Growth & Sequestered Split (CORPUS-01, CORPUS-02) — completed 2026-06-24
- [ ] Phase 112: Scale Hardening for the Real Run (SCALE-01, SCALE-02, SCALE-03)
- [ ] Phase 113: The Real Cost-Aware Run (RUN-01, RUN-02, RUN-03)
- [ ] Phase 114: Verdict & Boundary Re-Verification (REPORT-01, ADOPT-05)

## Phase Details

### Phase 111: Corpus Growth & Sequestered Split

**Goal**: Materialize a ≥101-task Aider-polyglot task-success corpus at `AIDER_TASKS_DIR` so `optimize.py`'s 50/50 split clears `val_size > 50`, with a disjoint sequestered TEST/attribution split — the literal v2.2/v2.3 no-ship axis, proven before any spend.
**Depends on**: Nothing new (first v2.4 phase; builds on the v2.3 `tools/dspy-tune/` harness + the vendored `bench/datasets/aider-polyglot` loader)
**Requirements**: CORPUS-01, CORPUS-02
**Success Criteria** (what must be TRUE):

  1. `_load_aider_corpus` consumes a materialized corpus of **≥101** Aider tasks (reusing the vendored `bench/datasets/aider-polyglot` loader; ~225 tasks/6 tracks available), and an offline/dry `optimize.py` path reports `train`/`val` sizes with **`val_size > 50`** (the gate-cleared branch, not the no-ship short-circuit).
  2. A held-out **TEST/attribution split is sequestered and disjoint** from both `trainset` and `valset`; the corpus loader/splitter never passes it to `compile()` (research: GEPA leaks `valset` into candidate selection, so TEST must stay out of both).
  3. **Anti-vacuity gate**: a hermetic test asserts `train ∩ val ∩ test = ∅` AND a planted leak (a test example injected into train/val) goes RED — break-the-invariant, not green-path-only; the `val_size==50` no-ship / `51`+ adoptable boundary is preserved.
  4. Corpus provenance/licensing recorded (reuse the v1.12 Exercism `LICENSE-AUDIT`/`VENDOR-MANIFEST` discipline); no task content is hand-fabricated.
  5. **Boundary preserved (ADOPT-04 cross-cutting)**: `git diff go.mod` empty; `make vet` (`toolsquarantine`) green; corpus materialization is dev-time Python/loader only — no `helix` runtime edge.

**Plans**: 1 plan (111-01) — completed 2026-06-24
**Research**: false — the relevant external facts (corpus sizing math, GEPA split semantics) are pinned in `.planning/research/SUMMARY.md`.

### Phase 112: Scale Hardening for the Real Run

**Goal**: Make the for-real run correct and cost-bounded before spending — pin `deepseek-v4-flash`, bound cost (turns / concurrency / 429 / rollout cap), and defeat the SWE-bench "0 tests ⇒ resolved=true" footgun.
**Depends on**: the v2.3 pipeline (107–109); independent of Phase 111 but ordered before the real run (113)
**Requirements**: SCALE-01, SCALE-02, SCALE-03
**Success Criteria** (what must be TRUE):

  1. `optimize.py`'s program LM **and** `reflection_lm` use an explicit config-var pin (`DSPY_LM_MODEL`) defaulting to **`deepseek-v4-flash`**, DeepSeek-primary / OpenAI-fallback consistent with the Phase-107 agent (replaces the `openai/gpt-4.1-mini` default + `OPENAI_API_KEY`-only guard); a test asserts the default pin and that the deprecation-bound legacy alias is not used.
  2. The run is **cost-bounded**: the agent tool-turn cap (`max_iters`/`T`) is enforced, provider **concurrency is capped with HTTP-429 retry/backoff**, and a hard rollout cap is available (explicit `max_metric_calls` or `auto="light"`); a test asserts the caps are wired (not silently unbounded).
  3. `grade_swebench.py` **hard-errors when 0 expected tests were evaluated** — it independently asserts the `FAIL_TO_PASS` bucket is non-empty and matches expected ids, never trusting the harness `resolved` flag; shipped with a **break-the-invariant → assert-RED** test (empty-bucket eval refused).
  4. Prompt-prefix caching is enabled where supported (stable system prompt → ~50× cache-hit input discount), or its absence is explicitly documented as accepted.
  5. **Boundary preserved (ADOPT-04)**: hermetic tests run LM-free; zero new Go deps; `make vet` green; all changes stay in `tools/dspy-tune/`.

**Plans**: TBD
**Research**: false — model id, pricing, and the harness footgun are resolved in `SUMMARY.md`.

### Phase 113: The Real Cost-Aware Run

**Goal**: Execute the v2.3 pipeline for real on the grown corpus — the headline billed phase — producing a real optimized program, an honest ON/OFF attribution delta with per-arm cost, and a SWE-bench Verified confirming agreement.
**Depends on**: Phase 111 (corpus) + Phase 112 (hardening)
**Requirements**: RUN-01, RUN-02, RUN-03
**Success Criteria** (what must be TRUE):

  1. `optimize.py` is **executed for real** against the grown corpus; it writes git-ignored `output/optimized.json` and prints `train`/`val` sizes proving **`val_size > 50`** (gate cleared, not the no-ship short-circuit).
  2. An **ON-vs-OFF attribution** on the sequestered split records a real task-success delta (`success(ON) − success(OFF)`) with **per-arm cost** via the metered LLM wrapper.
  3. The **SWE-bench Verified confirming leg** runs a **K=25–50 stratified** instance slice on Podman (`podman system service --time=0` + `DOCKER_HOST`→podman socket), preceded by a **K≈2 gold-patch smoke**; it is **fail-not-skip** — a requested real run yielding no result FAILS loudly (no silent empty success).
  4. Run artifacts (optimized.json, attribution record, SWE-bench reports) are captured for the REPORT, with cost recorded per arm.
  5. **Boundary preserved (ADOPT-04)**: the run shells `helix <verb>` + the upstream harness as subprocesses only; zero new Go deps; `go.mod` untouched; no `helix` runtime Python edge.

**Plans**: TBD
**Research**: false — logistics resolved in `SUMMARY.md`; the K≈2 gold-patch smoke IS the implementation-time confirmation.

### Phase 114: Verdict & Boundary Re-Verification

**Goal**: Turn the run's numbers into the honest ship/no-ship REPORT and re-verify the single-binary / no-runtime-Python boundary at the new corpus scale — the REPORT-only close.
**Depends on**: Phase 113 (the delta + per-arm cost + SWE-bench agreement feed the REPORT) — nothing downstream depends on this phase
**Requirements**: REPORT-01, ADOPT-05
**Success Criteria** (what must be TRUE):

  1. `tools/dspy-tune/REPORT.md` records the **real verdict** — ON/OFF/Δ, `val_size` (>50), per-arm cost, and the SWE-bench confirming agreement — with an explicit adopt / no-ship recommendation. **REPORT-only**: no `SKILL.md`/`reference.md` adoption is committed.
  2. The **ADOPT-04 boundary is re-verified end-to-end at scale**: `git diff go.mod go.sum` empty (zero new Go deps), no `helix` subcommand shells to Python, agent/optimizer/graders off `helix setup` + default `go test ./...` + the merge path, `make vet` (`toolsquarantine`) green.
  3. The **human-gated adoption path is proven ready**: the optimizer writes only git-ignored output, and a break-the-invariant test shows a desynced `reference.md` makes `helix-refgen --check` exit non-zero (adoption gate live, not vacuous).
  4. If the verdict is positive + gate-clearing, the follow-on adoption (TUNE-FUT-03) is recorded as the next step (not executed this milestone).

**Plans**: TBD
**Research**: false — unchanged v2.3 mechanism; skip research.

### ✅ v2.3 Task-Success-Driven Skill Optimization (Phases 107-110) — SHIPPED 2026-06-24

4 phases, 10 requirements (AGENT-01/02/03, ORACLE-01/02, TUNE-02/03/04, ADOPT-03/04), 100% mapped. Fixed the v2.2 no-ship root cause by replacing the gameable `choice_rate` adoption proxy with a real **agent task-success** optimization signal: a dev-time OpenAI-compatible ReAct agent drives the `helix` CLI via subprocess (107); GEPA's metric was rewired from `choice_rate` to honest benchmark task-success — Aider hidden-tests-green (108) and SWE-bench FAIL_TO_PASS-flip + PASS_TO_PASS-no-regression on Podman (109); every gain is attributed via a mandatory ON-vs-OFF control arm on a sequestered `val_size>50` split (109); adoption is human-gated via `helix-refgen --check` (110). Zero new Go deps, no runtime Python — the agent + optimizer live entirely in `tools/dspy-tune/`, off `go.mod` / `helix setup` / default `go test ./...` / the merge path. Verdict: **NO-SHIP by design** (corpus still below the `val_size>50` gate; TUNE-FUT-01 grows it). Audit PASSED — 10/10 reqs, 4/4 phases, 5/5 integration seams, E2E wired.

- [x] Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering (completed 2026-06-24)
- [x] Phase 108: Aider Honest Task-Success Oracle + Sandbox + GEPA Metric Rewire + Sequestered Split (completed 2026-06-24)
- [x] Phase 109: SWE-bench Oracle via Podman + ON/OFF Attribution-Delta Report (completed 2026-06-24)
- [x] Phase 110: Human-Gated Adoption + Boundary Re-Verification + Ship/No-Ship REPORT (completed 2026-06-24)

**Full details:** `.planning/milestones/v2.3-ROADMAP.md`

### ✅ v2.2 Agent-Facing Skill Quality & Prompt Tuning (Phases 103-106) — SHIPPED 2026-06-24

4 phases, 7 requirements (BUNDLE-01/02, REFGEN-01, SKILL-01/02/03, TUNE-01), 100% mapped. A content/codegen milestone: rewrote the agent-facing skill surface (hand-authored `SKILL.md` decision matrix + generated `reference.md`) for correctness, hardened the skill bundle so only `{SKILL.md, reference.md}` ship, and explored a quarantined dev-time DSPy offline-tuning harness against the Phase 101 adoption scorecard. Zero new Go dependencies; the DSPy spike stays strictly out of the shipped binary, `go.mod`, and `go test ./...`. Every gate ships a deliberate break-the-invariant → assert-RED test (code review caught and fixed a vacuous Guard B in 104 and a real Python↔Go parity bug in 106). Audit PASSED — 7/7 reqs, 4/4 phases, integration wired, 3/3 E2E flows, Nyquist 4/4.

- [x] Phase 103: Bundle Integrity & Non-Vacuous Reference Contract (completed 2026-06-23)
- [x] Phase 104: Reference Generator Per-Verb Correctness (completed 2026-06-23)
- [x] Phase 105: SKILL.md Decision-Matrix Rewrite (completed 2026-06-23)
- [x] Phase 106: Exploratory DSPy Offline Tuning Harness (spike) — documented no-ship (completed 2026-06-24)

**Full details:** `.planning/milestones/v2.2-ROADMAP.md`

### ✅ v2.1 Agent Adoption & Aider-Derived Validation (Phases 97-102) — SHIPPED 2026-06-23

6 phases, 23 requirements, 100% mapped. Make AI coding agents reliably reach for `helix` verbs over standard tools (generated per-verb reference + stronger multi-agent steering + a measured adoption contract), and prove the toolset end-to-end with vendored Aider benchmarks and committed local baselines (polyglot edit bench + RepoMap-quality and fuzzy-robustness evals). Additive — zero new Go dependencies; the only structural change was `internal/cli/skill.go` switching from an embedded `string` to an `embed.FS`. Audit PASSED — 23/23 reqs, 6/6 phases, 2/2 cross-phase E2E flows wired.

- [x] Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract (completed 2026-06-22)
- [x] Phase 98: Multi-Agent Coverage + Stronger Steering (completed 2026-06-23)
- [x] Phase 99: Vendored Aider Fixtures + Mixed-License Gate (completed 2026-06-23)
- [x] Phase 100: Polyglot Edit Benchmark + Committed Baseline (completed 2026-06-23)
- [x] Phase 101: Opt-In LLM-Behavioral Adoption Scorecard (completed 2026-06-23)
- [x] Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines (completed 2026-06-23)

**Full details:** `.planning/milestones/v2.1-ROADMAP.md`

### ✅ v2.0 CLI-First — MCP Surface Retirement (Phases 90-96) — SHIPPED 2026-06-22

7 phases (6 feature + 1 inserted post-audit tech-debt cleanup), 31 v1 requirements, 100% mapped. The `helix` CLI is the only agent-facing surface; the MCP Go SDK + gRPC IPC are retained as internal daemon plumbing. Strangler-fig: the CLI head was built behind the still-live MCP surface (90–93), parity proven by dual-run, the agent-facing MCP heads deleted **last** (94), docs/identity + docgen regen run against the frozen surface (95), and the non-blocking audit tech debt cleared (96). Re-audit PASSED — 31/31 reqs, 7/7 phases, 4/4 E2E flows, Nyquist 7/7.

- [x] Phase 90: CLI One-Shot Dial Spine + Race-Free Warm Reuse (completed 2026-06-21)
- [x] Phase 91: Code-Generated Verb Surface + `tools/call` Profile/Mode Enforcement (completed 2026-06-21)
- [x] Phase 92: Terse Output Renderer + Re-Targeted Contract Oracle (completed 2026-06-21)
- [x] Phase 93: SKILL.md + Nudge Repurpose + `helix setup` Flip (completed 2026-06-21)
- [x] Phase 94: Retire the Agent-Facing MCP Surface (DELETE) (completed 2026-06-21)
- [x] Phase 95: Identity & Docs Rewrite + docgen Regen (completed 2026-06-21)
- [x] Phase 96: Address v2.0 tech debt — inserted post-audit cleanup (TD-01..TD-04 + Nyquist 93–95) (completed 2026-06-22)

**Full details:** `.planning/milestones/v2.0-ROADMAP.md`

### ✅ v1.12 Bench Stack & Tool Evaluation (Phases 75-89) — SHIPPED 2026-06-21

15 phases, 63 v1 requirements, 100% mapped. Headline claim: *"Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits."*

- [x] Phase 75: Schema, Fairness Contract & Tree Skeleton (completed 2026-06-15)
- [x] Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags (completed 2026-06-16)
- [x] Phase 77: Bench Runtime & First E2E Smoke
- [x] Phase 78: Internal ToolBench — Go First + LanguageRunner Interface (completed 2026-06-17)
- [x] Phase 79: Evaluators & Result-Schema Metrics Layer (completed 2026-06-18)
- [x] Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement (completed 2026-06-19)
- [x] Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test
- [x] Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard (completed 2026-06-20)
- [x] Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder (completed 2026-06-21)
- [x] Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard (completed 2026-06-21)
- [x] Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners (completed 2026-06-21)
- [x] Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate (completed 2026-06-21)
- [x] Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness` (completed 2026-06-21)
- [x] Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters (completed 2026-06-21)
- [x] Phase 89: Reports, CI Policy & Contamination Canary (completed 2026-06-21)

**Full details:** `.planning/milestones/v1.12-ROADMAP.md`

## Backlog

### ✅ Promoted — v2.3: Task-Success-Driven Skill Optimization

The v2.3 milestone (Phases 107-110) replaces the gameable `choice_rate` adoption proxy (the v2.2 documented no-ship, TUNE-FUT-02) with a real agent task-success optimization signal. It activates the deferred TUNE-FUT-01 `val_size > 50` gate (via TUNE-03) and builds directly on the v2.2 `tools/dspy-tune/` harness + the v2.1 Aider/SWE-bench bench assets. Deferred to a future milestone (recorded in REQUIREMENTS.md): TUNE-FUT-01 (GEPA→MIPROv2/COPRO optimizer upgrade), TUNE-FUT-03 (grep-sed-cat third control arm + dual-bench joined metric), TUNE-FUT-04 (larger SWE-bench instance lists / more providers).

### ✅ Promoted — v2.2: Agent-Facing Skill Quality & Prompt Tuning

These two backlog candidates were promoted into the **v2.2** milestone (Phases 103-106, shipped 2026-06-24, see `.planning/milestones/v2.2-ROADMAP.md`) on 2026-06-23 via `/gsd-new-project`. They are retained here for lineage; the v2.2 phases superseded and shipped them.

- **BL-SKILL-01 → Phases 103-105** (BUNDLE-01/02 + REFGEN-01 + SKILL-01/02/03). Full SKILL.md + reference.md decision-matrix rewrite plus the `installSkill` allowlist + bundle-contents test. The original seed listed: (1) split rows that mix QUERY verbs with ACTION verbs that mutate state; (2) add the missing "Not this" guidance to every row (8+ rows currently `—`); (3) fix the `reference.md` "Use this, not that" copy-paste errors and incorrect "Output" descriptions; (4) add prerequisite notes for the indexed-graph verbs (all require `index-semantic-graph` first); (5) regroup the matrix by capability. Must stay consistent with the Phase 97 generator (`cmd/helix-refgen`) + `--check` drift gate and the `reference ⊇ VerbToolNames()` adoption contract — i.e. fix the GENERATOR, not hand-edit generated output. The allowlist + bundle-contents test became Phase 103; the generator fix became Phase 104; the SKILL.md matrix rewrite became Phase 105.

- **BL-SKILL-02 → Phase 106** (TUNE-01). DSPy-based offline prompt tuning of the agent-facing surface (exploratory). **Constraint:** Helix ships as a Go single binary with no Python/runtime deps — DSPy is a **dev-time/offline optimization harness** (Python, under `tools/`) that emits an optimized, committed `SKILL.md`/`reference.md`, NOT a runtime dependency. **Metric already exists:** the Phase 101 opt-in LLM-behavioral adoption scorecard (`test/oracle/adopt` — `choice_rate`/`fallback_rate`, keyed on the first emitted command) is the DSPy objective, closing the loop from v2.1's measurement work to v2.2's optimization. Promoted as the strictly-last, exploratory Phase 106 (documented no-ship; clean Go-search-loop fallback).
