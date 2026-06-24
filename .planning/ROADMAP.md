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
- [ ] **v2.3 Task-Success-Driven Skill Optimization** -- Phases 107-110 (in progress)

## Phases

### 🚧 v2.3 Task-Success-Driven Skill Optimization (Phases 107-110) — IN PROGRESS

4 phases, 10 requirements (AGENT-01/02/03, ORACLE-01/02, TUNE-02/03/04, ADOPT-03/04), 100% mapped. Fix the v2.2 no-ship root cause: replace the gameable `choice_rate` adoption proxy with a real **agent task-success** optimization signal. A net-new OpenAI-compatible tool-using agent drives the `helix` CLI via subprocess; GEPA's optimization metric is rewired from `choice_rate` to honest benchmark task-success (Aider hidden-tests-green; SWE-bench FAIL_TO_PASS-flip + PASS_TO_PASS-no-regression); any gain is attributed to the skill text via a mandatory ON-vs-OFF control arm on a sequestered `val_size>50` held-out split; adoption is human-gated via `helix-refgen --check`. The agent + optimizer live entirely dev-time in Python under `tools/dspy-tune/` — **zero new Go deps, no runtime Python**, off `go.mod` / `helix setup` / default `go test ./...` / the merge path.

**Forced dependency chain (do not reorder):** agent (107) → Aider honest oracle + metric rewire (108) → SWE-bench oracle (109) → human-gated adoption (110). Each phase ships its break-the-invariant → assert-RED test before the next depends on it; code-review + fix folds in BEFORE verify (the repeated v2.2 vacuous-pass / parity-bug class).

- [ ] Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering (AGENT-01/02/03)
- [ ] Phase 108: Aider Honest Task-Success Oracle + Sandbox + GEPA Metric Rewire + Sequestered Split (ORACLE-01, TUNE-02, TUNE-03) — **research**
- [ ] Phase 109: SWE-bench Oracle via Podman + ON/OFF Attribution-Delta Report (ORACLE-02, TUNE-04) — **research**
- [ ] Phase 110: Human-Gated Adoption + Boundary Re-Verification + Ship/No-Ship REPORT (ADOPT-03, ADOPT-04)

## Phase Details

### Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering
**Goal**: A dev-time, OpenAI-compatible tool-using agent drives the real `helix` CLI product surface in a bounded ReAct loop, with config-driven provider selection and a first-class steering ON/OFF switch — the foundation every grader and the metric rewire depend on.
**Depends on**: Nothing (first v2.3 phase; builds on the v2.2 `tools/dspy-tune/` harness)
**Requirements**: AGENT-01, AGENT-02, AGENT-03
**Success Criteria** (what must be TRUE):
  1. The agent (under `tools/dspy-tune/agent/`) drives `helix <verb>` via CLI subprocess in a bounded ReAct loop, observes each verb's stdout/exit, and writes a per-run transcript/trace; a hard turn / no-progress cap deterministically terminates the loop.
  2. The LM is config-driven via one `openai==2.43.0` client — DeepSeek primary (explicit `deepseek-v4-*` model id as a config var, alias-deprecation-safe), OpenAI fallback — and a real run with the selected provider's API key absent FAILS loudly (not a silent skip).
  3. The agent exposes a first-class `--steering on|off` switch: ON injects the candidate skill text as the system prompt, OFF uses a provably steering-omitted control prompt, so any measured effect is attributable to the skill text.
  4. **Anti-vacuity gate**: a hermetic `test_agent.py` (fake LLM + fake `helix`, no network) proves a deliberately degenerate always-grep agent scores 0 and the OFF prompt provably omits the steering text — break-the-invariant → assert-RED, not green-path-only.
  5. **Boundary preserved (ADOPT-04 cross-cutting)**: `git diff go.mod` is empty, `make vet` (`toolsquarantine`) stays green, and no `helix` subcommand shells to Python — the agent is dev-time Python only.
**Plans**: 1 plan
Plans:
- [ ] 107-01-PLAN.md — Standalone dev-time ReAct agent (`tools/dspy-tune/agent/{llm,tools,react,__init__}.py`): bounded loop driving `helix <verb>` via fixed-argv subprocess, DeepSeek-primary/OpenAI-fallback config-driven LM with loud-fail-on-missing-key, steering ON/OFF with a provably-omitted control, + hermetic anti-vacuity `test_agent.py`
**Research**: false

### Phase 108: Aider Honest Task-Success Oracle + Sandbox + GEPA Metric Rewire + Sequestered Split
**Goal**: Replace the gameable `choice_rate` reward with honest Aider-polyglot task-success — the core v2.3 swap — graded in a per-task sandbox with anti-tamper test restore, wired as the GEPA metric over a sequestered held-out TEST split gated by `val_size > 50`.
**Depends on**: Phase 107 (the metric runs the agent per task)
**Requirements**: ORACLE-01, TUNE-02, TUNE-03
**Success Criteria** (what must be TRUE):
  1. Aider-polyglot task-success is graded by running the exercise's native hidden tests in a per-task sandbox (Python mirror of `NativeTestCommand`, parity-pinned via a shared `golden/aider_native_cmd.json` corpus asserted by a Go `*_test.go` under `bench/datasets/aider-polyglot/`); a run where 0 tests executed is a hard ERROR (no vacuous pass), and gold test files are restored from an agent-unwritable path before grading.
  2. The DSPy GEPA optimization metric (`taskmetric.py` + `optimize.py` body swap) is rewired from `choice_rate` to agent task-success on the benchmark corpus; `choice_rate` is retained at most as a non-optimized diagnostic pre-screen, never a co-optimized reward.
  3. The corpus enforces a sequestered held-out TEST split never passed to `compile()`, with `val_size > 50` as a hard precondition gate for any adoption recommendation (closes the v2.2 tiny-corpus no-ship root cause).
  4. **Anti-vacuity gate**: break-the-invariant → assert-RED tests prove a do-nothing agent fails Rust (the `cargo test -- --include-ignored` path actually runs), agent test-tampering cannot force green, a planted TEST→TRAIN leak goes RED in `test_split.py`, and `val_size=50` no-ships while `51`+delta is adoptable.
  5. **Boundary preserved (ADOPT-04 cross-cutting)**: zero new Go module deps; the grader + metric stay in `tools/dspy-tune/` (off `go.mod`, `helix setup`, default `go test ./...`); `make vet` (`toolsquarantine`) stays green.
**Plans**: TBD
**Research**: true — honest-oracle design is the highest-risk surface and the v2.2 no-ship root cause; warrants a research pass.

### Phase 109: SWE-bench Oracle via Podman + ON/OFF Attribution-Delta Report
**Goal**: Add the heaviest grader — SWE-bench task-success via the upstream harness on Podman — behind the metric, and make every reported improvement an honest ON-vs-OFF attribution delta on the held-out split.
**Depends on**: Phase 107 (agent) + Phase 108 (metric, sandbox, split, attribution scaffold)
**Requirements**: ORACLE-02, TUNE-04
**Success Criteria** (what must be TRUE):
  1. SWE-bench task-success is graded via the upstream `swebench==4.1.0` harness on Podman (`podman system service` + `DOCKER_HOST`→podman socket), enforcing the FAIL_TO_PASS + PASS_TO_PASS resolution contract; the dataset id is pinned to the correct org and fetch-resolution plus a nonzero task-count are asserted before any run (SWE-bench is NOT blocked — Podman provides the docker-compat socket).
  2. Any reported improvement is an ON-vs-OFF attribution delta (`success(ON) − success(OFF)`) measured on the held-out split and recorded alongside `val_size` and per-arm cost.
  3. **Anti-vacuity gate**: a hermetic fake-harness test asserts the exact upstream argv + env allowlist + dataset-org pin (break-the-invariant → assert-RED); the live SWE-bench leg is gated and skips when offline but FAILS loudly on a *requested* real run that produces no result.
  4. **HELIX_BIN fail-not-skip**: any bench surface requiring `HELIX_BIN` FAILS loudly when it is set but the run produced no result, rather than silently skipping (Phase 81 false-green class).
  5. **Boundary preserved (ADOPT-04 cross-cutting)**: `swebench`/`openai` stay dev-time Python pins; zero new Go module deps; `make vet` (`toolsquarantine`) stays green; the SWE-bench grader is pluggable behind `taskmetric.py` with no Go runtime edge.
**Plans**: TBD
**Research**: true — live SWE-bench-on-Podman (rootless quirks, `--network=none`, dataset-org resolution) needs implementation-time confirmation.

### Phase 110: Human-Gated Adoption + Boundary Re-Verification + Ship/No-Ship REPORT
**Goal**: Gate the pipeline's output: optimized skill text is adopted only via a human-reviewed `SKILL.md` edit behind `helix-refgen --check`, the single-binary / no-runtime-Python invariant is re-verified end-to-end, and a ship/no-ship REPORT records the ON/OFF/Δ verdict.
**Depends on**: Phase 109 (the attribution delta + per-arm cost feed the REPORT) — nothing downstream depends on this phase
**Requirements**: ADOPT-03, ADOPT-04
**Success Criteria** (what must be TRUE):
  1. Optimized skill text is adopted only via a human-reviewed `SKILL.md` edit gated by `helix-refgen --check` — the optimizer writes only git-ignored output (`output/optimized.json`) and never auto-writes `SKILL.md`/`reference.md`; the `## Decision matrix` anchor and the SKILL size cap are preserved.
  2. The single-binary / no-runtime-Python invariant is re-verified end-to-end as the primary owner of ADOPT-04: zero new Go module deps, no `helix` subcommand shells to Python, the agent/optimizer stay off `go.mod` / `helix setup` / the default `go test ./...` / the merge path, and the `make vet` import-boundary (`toolsquarantine`) analyzer stays green (`grep -E 'skills/helix|reference\.md' optimize.py == 0`).
  3. A ship/no-ship REPORT records the ON/OFF attribution delta, `val_size`, and per-arm cost alongside an explicit adopt / no-ship verdict.
  4. **Anti-vacuity gate**: a hand-edit of `reference.md` out of sync with the generator makes `helix-refgen --check` exit non-zero (break-the-invariant → assert-RED), proving the adoption gate is live and not vacuous.
**Plans**: TBD
**Research**: false — unchanged v2.2 mechanism; skip research.

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
