---
gsd_state_version: 1.0
milestone: v2.5
milestone_name: Agent Harness Rebuild
status: phase_complete
stopped_at: Phase 115-01 SUMMARY committed
last_updated: "2026-06-26T14:30:00.000Z"
last_activity: 2026-06-26 — Phase 115-01 complete (HARNESS-01/02 implemented, all tests pass)
progress:
  total_phases: 4
  completed_phases: 1
  total_plans: 1
  completed_plans: 1
  percent: 25
current_phase: 115
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-26)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.5 Agent Harness Rebuild — fix the harness so the agent actually solves tasks by editing.

## Current Position

Phase: 115-01 COMPLETE (Task-Solving Prompt + Feedback Loop)
Plan: 115-01 SUMMARY committed
Status: Ready for Phase 116
Last activity: 2026-06-26 — Phase 115-01 complete with all HARNESS-01/02 tests passing

## Performance Metrics

**Velocity:** v2.4 closed at 4/4 phases. Per-plan history for shipped milestones lives in the archived milestone ROADMAPs; this table resets at the v2.5 start.

*Updated after each plan completion.*

## Accumulated Context

### Roadmap Evolution

- 2026-06-26: v2.5 roadmap to be created from investigation findings (agent makes 0 tool calls on ~40% of tasks; no feedback loop; GEPA reflection is a no-op). Driver: **TUNE-FUT-06** — rebuild the harness to make the agent actually solve by editing: task-solving system prompt, feedback loop (run-tests/get-diagnostics in ReAct), verb-arg hardening, real GEPA agent module. THEN re-run attribution for a meaningful delta.
- 2026-06-24: v2.4 roadmap created (Phases 111-114) from REQUIREMENTS.md (10 v2.4 REQs across CORPUS/SCALE/RUN/REPORT/ADOPT) + research SUMMARY.md (DEEPSEEK/GEPA-COST/SWEBENCH). Driver: **TUNE-FUT-01** — grow the optimization corpus past the strict `val_size>50` held-out gate and run the v2.3 task-success pipeline FOR REAL (cost-aware, both tracks) to turn v2.3's "NO-SHIP by design" into an actual numbers-backed adopt/no-adopt verdict. **REPORT-only** (adoption stays a separate human `helix-refgen --check` step); **fix-as-needed** latitude on the v2.3 agent/graders. Forced dependency chain honored: 111 (CORPUS-01/02 — materialize ≥101-task Aider corpus at `AIDER_TASKS_DIR` so the 50/50 split clears `val_size>50`, reusing the vendored `bench/datasets/aider-polyglot` loader; disjoint sequestered TEST/attribution split with a planted-leak RED test — research: GEPA leaks valset into selection, so TEST stays out of train AND val), 112 (SCALE-01/02/03 — pin `DSPY_LM_MODEL=deepseek-v4-flash` for program+reflection LM DeepSeek-primary/OpenAI-fallback; cost bounds: agent max_iters cap + concurrency cap + 429 backoff + hard rollout cap; `grade_swebench.py` hard-errors on the harness "0-tests⇒resolved=true" footgun with a break-the-invariant RED test), 113 (RUN-01/02/03 — execute `optimize.py` for real → git-ignored `output/optimized.json` proving `val_size>50`; ON-vs-OFF attribution with per-arm cost via the metered LLM wrapper; SWE-bench Verified K=25–50 stratified confirming slice on Podman with a K≈2 gold-patch smoke, fail-not-skip), 114 (REPORT-01 + ADOPT-05 — fill `tools/dspy-tune/REPORT.md` with the real ON/OFF/Δ/val_size/per-arm-cost/SWE-bench-agreement verdict REPORT-only; re-verify the ADOPT-04 single-binary/no-runtime-Python boundary at scale + prove the `helix-refgen --check` human-gated adoption path ready). 10/10 reqs mapped, 0 unmapped, 0 double-mapped. **ZERO new Go deps** (go.mod untouched since v2.0 Phase 90); all new pins are dev-venv Python only (`dspy==3.2.1`, `openai==2.43.0`, `swebench==4.1.0`). Research resolved the 3 cost/logistics unknowns (model pin `deepseek-v4-flash` — legacy aliases retire 2026-07-24 + `deepseek-reasoner` breaks tool-calls; GEPA `auto="light"` ≈600 rollouts ≈ $5–15/run, trainset free / valset is the lever; SWE-bench K=25–50 stratified Verified slice + the harness 0-tests footgun), so 0 phases flagged for phase-level research.

### Critical Roadmap Constraints (v2.5 — TUNE-FUT-06)

Cross-cutting exit gates (an agent-harness rebuild on the v2.3/v2.4 pipeline; carries forward all v2.3/v2.4 discipline):

- **Forced dependency chain (do not reorder):** task-solving prompt + feedback loop (Phase 1) → verb-arg hardening (Phase 2) → real GEPA module (Phase 3) → re-run attribution (Phase 4). Phase 4 is gated on Phases 1–3 completing successfully.
- **System prompt MUST force editing (HARNESS-01):** the prompt names the solution file, declares "success = hidden tests pass", forbids prose answers, and says "not done until implemented". A break-the-invariant test (agent answers in prose → assert FAIL) ships with Phase 1.
- **Feedback loop MUST verify completion (HARNESS-02):** run-tests / get-diagnostics in the ReAct loop, so the agent knows when it's done. A break-the-invariant test (no feedback loop → agent declares done on broken code → assert FAIL) ships with Phase 1.
- **Verb-arg usage MUST be hardened (HARNESS-03):** the error budget is finite; malformed calls burn it. A break-the-invariant test (malformed argv passes silently → assert FAIL) ships with Phase 2.
- **GEPA MUST evolve (HARNESS-04):** the `AgentProgram` must emit a reflectable trace so GEPA's reflective mutation has something to optimize. A break-the-invariant test (trace is empty/constant → assert FAIL) ships with Phase 3.
- **Re-run attribution after harness is fixed (HARNESS-05):** Phase 4 re-runs the v2.4 attribution pipeline ON THE FIXED HARNESS. The delta is only meaningful if the agent demonstrably uses helix on most tasks.
- **ADOPT-04 single-binary / no-runtime-Python (carried forward):** zero new Go deps; all tuning stays in dev-venv Python (`tools/dspy-tune/`); no `helix` subcommand shells to Python.
- **Corpus is already past the gate:** the 103-task Aider corpus (train 26 / val 26 / held-out 51) is already materialized and past `val_size>50`. No corpus growth needed.

### Critical Roadmap Constraints (v2.4 — Phases 111-114, historical but still relevant)

Cross-cutting exit gates (a corpus-growth + for-real-run milestone on the v2.3 pipeline; carries the v2.3 boundary discipline forward):

- **Forced dependency chain (do not reorder):** corpus growth + sequestered split (111) → scale hardening (112) → the real billed run (113) → verdict + boundary re-verify (114). 112 is independent of 111 but ordered before 113.
- **The corpus is the spine + the literal no-ship axis:** the GEPA reward corpus is the **Aider-polyglot** set loaded via `_load_aider_corpus(AIDER_TASKS_DIR)` (the `data/{train,test}.jsonl` 8/3 rows are only the `choice_rate` diagnostic pre-screen). `optimize.py` splits 50/50, so the corpus needs **≥101 tasks** for `valset>50`. Reuse the vendored loader (~225 tasks/6 tracks); do NOT hand-fabricate tasks.
- **Sequestered split (research-load-bearing):** GEPA reflects on **trainset** and scores/selects candidates on **valset** — valset is leaked into model selection. The held-out TEST/attribution split must NEVER be passed to `compile()` as trainset OR valset. Ship a planted-leak → assert-RED test (`train∩val∩test=∅`).
- **ADOPT-04 single-binary / no-runtime-Python (cross-cutting; primary owner P114):** zero new Go module deps (go.mod untouched since v2.0 Phase 90); no `helix` subcommand shells to Python; agent/optimizer/graders stay off `go.mod` / `helix setup` / default `go test ./...` / merge path; `make vet` (`toolsquarantine`) green. New pins are dev-venv Python ONLY: `dspy==3.2.1`, `openai==2.43.0`, `swebench==4.1.0`.
- **Anti-vacuity (every gate that ADDS/changes a gate):** each new gate (planted-leak split test P111, model-pin/cost-cap tests P112, the `grade_swebench` 0-tests footgun refusal P112, `helix-refgen --check` desync P114) MUST ship a break-the-invariant → assert-RED test. Fold code-review + fix BEFORE verify (the repeated v2.2/v2.3 vacuous-pass / parity-bug class).
- **HELIX_BIN / requested-run fail-not-skip (P113):** the live SWE-bench leg skips when offline but FAILS loudly on a *requested* real run that yields no result (Phase 81 false-green class). Gold-patch K≈2 smoke first.
- **Model-id pin (P112, research-pinned):** `DSPY_LM_MODEL=deepseek-v4-flash` explicit (legacy `deepseek-chat`/`-reasoner` aliases retire 2026-07-24; `deepseek-reasoner` has NO function-calling → would break the ReAct loop). DeepSeek-primary/OpenAI-fallback, base_url `https://api.deepseek.com`, concurrency-limited (2500 flash) → cap in-flight + 429 backoff.
- **SWE-bench footgun (P112, research-pinned):** the upstream v4.1.0 harness scores **0 tests evaluated as `resolved=True`** (`compute_fail_to_pass` returns 1.0 on `total==0`). `grade_swebench.py` MUST independently assert the FAIL_TO_PASS bucket is non-empty + matches expected ids (the Phase-109 "0 tests ⇒ GradeError" rule) — never trust the harness `resolved` flag. Dataset pin `princeton-nlp/SWE-bench_Verified` still resolves; always pass `--dataset_name` (v4.1.0 default moved to `SWE-bench/SWE-bench_Lite`).
- **Gate the committed ARTIFACT, never the optimizer PROCESS (P113/P114):** LLM optimization is not bit-reproducible — `optimize.py` writes only git-ignored `output/optimized.json`. Never re-run it in CI. REPORT-only: no `SKILL.md`/`reference.md` adoption committed this milestone; adoption (TUNE-FUT-03) is a separate human `helix-refgen --check`-gated step.

### Critical Roadmap Constraints (v2.3 — Phases 107-110, historical)

Cross-cutting exit gates baked into every relevant phase (a dev-time/offline optimization milestone bolted onto the shipped Go single binary WITHOUT breaching the no-runtime-Python boundary):

- **Forced dependency chain (do not reorder):** agent (107) → Aider honest oracle + metric rewire (108) → SWE-bench oracle (109) → human-gated adoption (110). GEPA calls its metric in-process per candidate, so the agent + metric live in **Python under `tools/dspy-tune/`** (a sibling of `scorer.py`/`optimize.py`), NOT Go `bench/runtime` — a Go LLM client would force a forbidden runtime `go.mod` dep + a process-spawn handoff into the optimizer's hot loop. Pattern: in-process metric, subprocess everything-the-LLM-touches (agent shells `helix <verb>`; graders shell the per-language test command / the upstream swebench harness).
- **ADOPT-04 single-binary / no-runtime-Python (cross-cutting; primary owner P110):** zero new Go module deps; no `helix` subcommand shells to Python; the agent/optimizer stay off `go.mod`, `helix setup`, default `go test ./...`, and the merge path; `make vet` (`toolsquarantine` import-boundary analyzer — needs no change, all new edges are Python/`subprocess` inside the exempt `tools/` prefix) stays green. New deps are dev-time Python pins ONLY: `openai==2.43.0` + `swebench==4.1.0`; keep `dspy==3.2.1`. Re-verified as an exit gate in EVERY phase (107–110).
- **Anti-vacuity (every gate that ADDS a gate):** each new gate (vacuous-pass refusal P108, test-tamper restore P108, TEST-split sequestration P108, `val_size>50` precondition P108, ON/OFF attribution P109, exact-argv+env-allowlist P109, `--check` adoption gate P110, boundary analyzer all) MUST ship a deliberate break-the-invariant → assert-RED test; a green-path-only gate is presumed broken. Fold code-review + fix BEFORE verify (the repeated v2.2 vacuous-pass / Python↔Go parity-bug class — caught a tautological `x==x` Guard B in 104 and a real parity bug in 106).
- **HELIX_BIN fail-not-skip (every bench-surface phase: 109, also 108's grader):** any bench surface requiring `HELIX_BIN` FAILS loudly when it is set but the run produced no result, rather than silently skipping (the Phase 81 false-green / SIGKILL-vacuous-gate class). The live SWE-bench leg skips when offline but FAILS on a *requested* real run that yields no result.
- **Reuse the existing assets, don't fork:** `bench/datasets/aider-polyglot/loader.go` (Aider corpus), `bench/evaluators/swebench/harness.go` (DOCKER_HOST allowlist + dataset pin), `bench/container` (podman-aware auto-detect), `tools/dspy-tune/{optimize.py,scorer.py}` (GEPA metric body swap; `scorer.py` retained as an optional non-optimized pre-screen), `cmd/helix-refgen --check` (the adoption gate). SWE-bench is NOT blocked — `podman system service --time=0 &` + `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` is configuration, not a blocker; watch dataset-org drift (`princeton-nlp/` datasets vs the `SWE-bench/` repo org).
- **Model-id is a config var (P107):** DeepSeek `deepseek-chat`/`-reasoner` aliases retire 2026-07-24 → `deepseek-v4-flash`/`-pro`; make the model a config var (`DSPY_LM_MODEL` precedent) and prefer the explicit `deepseek-v4-*` id so the cutover is one line.
- **Gate the committed ARTIFACT, never the optimizer PROCESS (P110):** LLM optimization is not bit-reproducible — `optimize.py` writes only git-ignored `output/optimized.json`; adoption = a human-reviewed SKILL.md edit (≤ size cap, `## Decision matrix` anchor preserved) gated by `helix-refgen --check`. Never re-run `optimize.py` in CI; unset-key exits 0 for hermetic gates but fails loudly on a real run. `grep -E 'skills/helix|reference\.md' optimize.py == 0`.

### Critical Roadmap Constraints (v2.2 — Phases 103-106, historical)

Cross-cutting exit gates baked into every relevant phase (a content/codegen milestone, NOT a stack milestone):

- **Anti-vacuity (every gate):** each gate a phase adds/hardens (closed-set bundle test P103, exact-count==50 reference contract P103, generator vacuity guards P104, SKILL↔VerbToolNames cross-check P105, Python↔Go parity + leakage analyzer P106) MUST ship a deliberate break-the-invariant → assert-RED test. A gate with only a green-path test is presumed broken (anchored to the repeated Phase 86/87/89 CR-01 vacuous-pass class).
- **No runtime Python / single-binary preserved (P106, but enforced milestone-wide):** DSPy is dev-time/offline only — no `helix` subcommand shells to Python, no `go.mod`/`helix setup` edge, off the default `go test ./...` / merge path. A `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path. Gate the committed ARTIFACT, never the optimizer PROCESS (LLM optimization isn't bit-reproducible).
- **`reference.md` generated, never hand-edited (P104, P106):** all reference corrections go through `cmd/helix-refgen` (per-verb override map, group-default fallback retained, lookups keyed not ranged for determinism); the Phase 97 `--check` byte-reproducibility gate and the `reference ⊇ VerbToolNames()` contract stay green; blank-import parity between generator and daemon re-verified when touching refgen (the v1.12 docgen-drift lesson).
- **Harden-the-contract-BEFORE-the-rewrite (P103 → 104/105):** the `reference ⊇ VerbToolNames()` contract MUST be hardened to exact-count==50 + known-absent-verb discriminator + RED-first proof BEFORE the format rewrite, or it goes vacuously `∅ ⊇ ∅` true through the churn. Land the closed-set bundle allowlist FIRST (the embed-glob leak is LIVE — the binary + every `helix setup` already ship the 18 KB `SKILL-ISSUE.md`).
- **Generator-before-matrix (P104 → 105):** the on-demand `reference.md` must be correct before the idle-tier `SKILL.md` matrix is re-authored, so the two tell one consistent story.
- **Preserve the StripDecisionMatrix anchor + SKILL-04 idle-cost cap (P105):** keep the `## Decision matrix` heading and stay under the SKILL-04 size cap during the matrix rewrite (split rows ~37→44, terse "Not this" everywhere — guard against token bloat).
- **Spike discipline (P106):** exploratory, possible no-ship; the corpus is small (`MinTasks=5`) so a held-out TEST split the optimizer never sees is the FIRST harness task; pair `choice_rate` with a correctness/quality oracle to defend against metric-gaming the gameable first-command proxy; clean fallback is a hand-rolled Go candidate-search loop keeping the milestone 100% Go.

### Critical Roadmap Constraints (v2.1 — Phases 97-102, historical)

Cross-cutting exit gates baked into every relevant phase (anchored to named v1.12 vacuous-pass CRITICALs — gates, not new work):

- **Anti-vacuity (every gate):** each gate a phase adds (adoption contract P97, license gate P99, bench baseline P100/P102, eval discriminator P102, LLM scorecard P101) MUST ship a deliberate break-the-invariant → assert-RED test. A gate with only a green-path test is presumed broken.
- **HELIX_BIN fail-not-skip (every bench-surface phase: 100, 102):** a committed hermetic golden sibling (no binary, no network) is the SOLE authoritative proof; the live leg FAILS (never silently SKIPs) when `HELIX_BIN` is set but no `result.v2.json` / empty bucket / missing metric line is produced; add a "did it RUN" sentinel.
- **Leaf-import boundary (P102):** new `bench/evaluators/{repomapeval,fuzzyrobust}` leaves are stdlib-only (+ `editsim.ES`) and respect `vet-ablation-leakage` — no `internal/kernel`/`internal/semantic`/`bench/runtime` imports; the daemon-dialing EDIT AgentFn (P100) lives OUTSIDE the leaf in `bench/runtime`.
- **Local-only benches:** no CI benchstat gate re-introduced; baselines captured + committed locally, byte-reproducible, deterministic-metrics-only (latency → local `bench-micro`).
- **Reuse-don't-fork / generate-don't-hand-write / measure-don't-port:** reuse `RunExercise` verbatim (WR-01 anti-tamper untouched, no schema v3 bump — `edit_format_applied` is an additive `*bool` open key); generate `reference.md` from the registry via `helix-refgen` (blank-import parity with daemon); measure existing `internal/repomap`+`internal/fuzzy`, don't reimplement aider's algorithm.
- **Mixed-license vendoring (P99):** user RATIFIED the mixed-license tree — MIT Exercism polyglot fixtures (SPDX MIT + per-track NOTICE) AND Apache-2.0 aider edit-format fixtures (SPDX Apache-2.0 + attribution); vendor only the exercised subset via `VENDOR-MANIFEST.md`; extend `verify-licenses` to the full vendored tree with a tamper test.
- **Steering stays advisory exit-0 (P98):** broaden the classifier but keep the fail-open exit-0 contract; negative-control golden rows prove the nudge does NOT fire on prose/log/config/build-output; no deny/block (exit 2) hook; no fabricated Gemini PreToolUse hook; per-agent instruction files appended via sentinel-delimited idempotent writes that never clobber user content (Codex `AGENTS.md` ≤32 KiB).

### Critical Roadmap Constraints (v2.0 — carried forward to Phase 90+, historical)

- **Zero-proto invariant (Phase 90):** the one-shot `tools/call` rides the existing gRPC `StreamMCP` wire; `git diff api/proto/` must stay empty. Fix the cross-process daemon-spawn race (`dial.go` has no lock today) and set a 2nd-call latency SLO.
- **Security gate co-located with verbs (Phase 91):** `ProfileFilterMiddleware` only filters `tools/list` — there is no `tools/call` rejection path. The moment the always-visible generated verbs land, profile/mode enforcement at `tools/call` MUST ship in the same phase, or a read-mode/ci-bot agent could invoke destructive edit verbs. Do NOT defer SEC-01/02.
- **Output shape freezes before SKILL.md (Phase 92 → 93):** the terse renderer is the load-bearing product work and precedes SKILL.md authoring (the decision table cites real verb names + real output shape).
- **Delete MCP heads LAST (Phase 94):** stdio forwarder head + Streamable-HTTP `/mcp` removed only after dual-run parity proves CLI is the sole surface (RETIRE-03 is the gate). RETIRE-04 optional gRPC TCP bind rides with the retirement phase. Keep the SDK, gRPC IPC, 5 middlewares, and all tool handlers behind the wire.
- **Docs/identity + docgen regen against the FROZEN surface (Phase 95):** keep docgen's blank imports == the daemon's (the v1.12 docgen-drift lesson).
- **Tool count = 53** (live registry, reconciled at v1.12 quick 260617-t7x). VERB-01 acceptance is generated-count == live-registry-count enumerated by name, not a hardcoded number.

### Pending Todos

None yet.

### Blockers/Concerns

**RESOLVED (Phase 113, 2026-06-24): workspace-activation gap — fixed (user-approved Go product fix: `helix activate` now also calls `activate_project`; committed + E2E regression-tested). The real run completed: SHIP verdict, delta +0.0392, val_size=51, $0.23 + SWE-bench 2/2 gold confirm on Podman.** [Historical detail of the blocker:]

**(Phase 113, 2026-06-24): workspace-activation gap stopped the real run.** The v2.3 dev-time agent was never actually run end-to-end against real helix — its hermetic fakes masked two real defects. (1) FIXED: `agent/tools.py` passed `location` as a positional, but helix verbs take named `--flags` (`read-file --path X`) → every verb failed `required flag --path not set`. (2) BLOCKER: the file/edit verbs return `no_workspace` because the daemon's file-tool active-workspace state is set ONLY by the MCP `activate_project` tool (`repo_path`, persists globally per-daemon via `ActivateCallback`), and there is **no `activate_project` CLI verb**. `helix activate` (gRPC `ActivateWorkspace`) sets only kernel/LS state, NOT the file-tool workspace (confirmed empirically + by the verb.go:153 E2E comment). A one-shot `helix <verb>` sends no cwd, so LazyInit (which keys on `repo_path`) can't auto-activate. This is also a likely **real product bug**: the CLI-first file/edit verbs cannot obtain an active workspace from the shipped CLI surface (SessionStart's `helix activate` does not enable them). DeepSeek auth works (probe OK); corpus + hardening (111/112) are done. Awaiting user decision on how to unblock (see Session Continuity).

Watch items for v2.5:

- **HARNESS-01 system prompt:** must force editing, forbid prose, name the file, success = hidden tests pass. A prose-answer test MUST fail.
- **HARNESS-02 feedback loop:** must wire run-tests/get-diagnostics into ReAct. A no-feedback test MUST fail.
- **HARNESS-03 verb hardening:** must stop burning error budget. A malformed-call test MUST fail.
- **HARNESS-04 GEPA evolution:** AgentProgram MUST emit a predictor trace. An empty-trace test MUST fail.
- **HARNESS-05 re-run attribution:** only meaningful after HARNESS-01/02/03/04 complete. Delta on a broken agent is noise.

## Session Continuity

Last session: 2026-06-26T07:30:00.000Z
Stopped at: Phase 115 PLAN committed
Resume file: .planning/phases/115-task-solving-prompt-feedback-loop/115-PLAN.md

## Decisions

- **D-01:** Solution file naming (explicit or extracted from task description)
- **D-02:** Success criterion "hidden tests must pass" declared in prompt
- **D-03:** Agent-driven test execution (both verbs: get-diagnostics, run-tests)
- **D-04:** Retry nudge then fail on repeat prose
- **D-05:** Both verbs for test discovery (get-diagnostics + run-tests)

## Operator Next Steps

- Execute Plan 115-01 (Task-Solving System Prompt)
- Execute Plan 115-02 (Feedback Loop) after 115-01 completes
- Run anti-vacuity tests to verify both gates work
