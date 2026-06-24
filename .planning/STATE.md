---
gsd_state_version: 1.0
milestone: v2.3
milestone_name: Task-Success-Driven Skill Optimization
current_phase: 110
current_phase_name: Human-Gated Adoption + Boundary Re-Verification + Ship/No-Ship REPORT
status: phases_complete
stopped_at: "v2.3 at 4/4 — all phases (107-110) shipped & verified (passed). Phase 110 verdict NO-SHIP by design. Next: milestone lifecycle (audit → complete → cleanup)."
last_updated: "2026-06-24T10:40:37.588Z"
last_activity: 2026-06-24
last_activity_desc: Phase 110 executed inline (uv) — adoption gate proven + boundary re-verified + ship/no-ship REPORT (NO-SHIP)
progress:
  total_phases: 4
  completed_phases: 4
  total_plans: 4
  completed_plans: 4
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-23)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.3 paused at 2/4 — next is Phase 109 (SWE-bench Oracle via Podman + ON/OFF Attribution-Delta)

## Current Position

Phase: 110 complete (4/4 v2.3 phases done) — ALL PHASES COMPLETE
Plan: —
Status: 107 + 108 + 109 + 110 shipped & verified (passed). Phase 110 verdict NO-SHIP by design (corpus < val_size>50). Next = milestone lifecycle (audit → complete → cleanup).
Last activity: 2026-06-24 — Phase 110 executed inline with uv; adoption gate proven live, boundary re-verified, ship/no-ship REPORT written.

## Performance Metrics

**Velocity:** v1.12 closed at 15/15 phases. Per-plan history for shipped milestones lives in the archived milestone ROADMAPs; this table resets at the v2.0 start.

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 90 | 4 | - | - |
| 91 | 4 | - | - |
| 92 | 3 | - | - |
| 93 | 4 | - | - |
| 94 | 2 | - | - |
| 95 | 2 | - | - |
| 97 | 2 | - | - |
| 98 | 2 | - | - |
| 99 | 2 | - | - |
| 100 | 2 | - | - |
| 101 | 2 | - | - |
| 102 | 3 | - | - |
| 104 | 1 | - | - |
| 105 | 1 | - | - |
| 106 | 2 | - | - |

*Updated after each plan completion.*
| Phase 90 P01 | 5min | 2 tasks | 5 files |
| Phase 90 P02 | 3min | 1 tasks | 2 files |
| Phase 90 P02 | 3min | 1 tasks | 2 files |
| Phase 90 P03 | 6min | 2 tasks | 6 files |
| Phase 90 P04 | 35min | 3 tasks | 5 files |
| Phase 91 P01 | 17min | 3 tasks | 12 files |
| Phase 91 P02 | 3min | 2 tasks | 3 files |
| Phase 91 P03 | 3min | 2 tasks | 2 files |
| Phase 91 P04 | 6min | 2 tasks | 2 files |
| Phase 92 P01 | 20min | 3 tasks | 6 files |
| Phase 92 P02 | 6min | 2 tasks | 8 files |
| Phase 92 P03 | ~10min | 2 tasks | 9 files |
| Phase 93 P01 | 50m | 2 tasks | 3 files |
| Phase 93 P02 | 4min | 2 tasks | 2 files |
| Phase 93 P03 | 9min | 2 tasks | 3 files |
| Phase 93 P04 | 310 | 2 tasks | 21 files |
| Phase 94 P01 | 18min | 3 tasks | 14 files |
| Phase 94 P02 | 25min | 3 tasks | 28 files |
| Phase 95 P01 | 3min | 3 tasks | 6 files |
| Phase 95 P02 | 18 | 3 tasks | 3 files |
| Phase 96 P01 | 6min | 4 tasks | 10 files |
| Phase 102 P01 | 11 | 4 tasks | 16 files |
| Phase 102 P02 | 14 | 4 tasks | 17 files |
| Phase 102 P03 | 16 | 3 tasks | 10 files |
| Phase 103 P01 | 4m | 3 tasks | 5 files |
| Phase 104 P01 | 3 | 3 tasks | 4 files |
| Phase 105 P01 | 5min | 2 tasks | 2 files |
| Phase 106 P01 | 5min | 3 tasks | 10 files |

## Accumulated Context

### Roadmap Evolution

- 2026-06-24: v2.3 roadmap created (Phases 107-110) from REQUIREMENTS.md (10 v2.3 REQs across AGENT/ORACLE/TUNE/ADOPT) + research SUMMARY.md. Coarse granularity, sequential phase IDs, numbering continued from v2.2 (ended at 106). Driver: TUNE-FUT-02 — replace the gameable `choice_rate` adoption proxy (the v2.2 documented no-ship) with a real **agent task-success** optimization signal. Forced dependency chain honored (agent → Aider grader + metric rewire → SWE-bench grader → human-gated adoption): 107 (AGENT-01/02/03 — ReAct tool-using agent driving `helix <verb>` via subprocess, one `openai==2.43.0` client, DeepSeek-primary/OpenAI-fallback model-as-config-var, hard turn/no-progress cap, fail-loud-on-missing-key, first-class `--steering on|off`), 108 (ORACLE-01 + TUNE-02 + TUNE-03 — honest Aider hidden-tests oracle in a per-task sandbox with 0-tests-ran-is-ERROR + agent-unwritable gold-test restore, the core `choice_rate → task-success` GEPA metric swap, sequestered held-out TEST split + `val_size>50` hard adoption gate; **research-flagged** — highest-risk surface + v2.2 no-ship root cause), 109 (ORACLE-02 + TUNE-04 — heaviest SWE-bench oracle via upstream `swebench==4.1.0` harness on Podman with `DOCKER_HOST`→podman socket, FAIL_TO_PASS+PASS_TO_PASS contract + dataset-org pin + nonzero-task-count assert, ON-vs-OFF attribution delta; **research-flagged** — live SWE-bench-on-Podman needs implementation-time confirmation), 110 (ADOPT-03 + ADOPT-04 — human-gated adoption via `helix-refgen --check`, no auto-write of SKILL.md/reference.md, single-binary/no-runtime-Python re-verification as ADOPT-04's primary owner, ship/no-ship REPORT recording ON/OFF/Δ/val_size/per-arm-cost). 10/10 reqs mapped, 0 unmapped, 0 double-mapped. ZERO new Go deps (all new deps are dev-time Python pins — `openai==2.43.0` + `swebench==4.1.0` in the `tools/dspy-tune/` venv; keep `dspy==3.2.1`); the agent + optimizer stay strictly off `go.mod` / `helix setup` / default `go test ./...` / merge path. ADOPT-04 + anti-vacuity + HELIX_BIN-fail-not-skip are CROSS-CUTTING exit gates recurring in every relevant phase (ADOPT-04 owned for traceability by 110). Both v2.2 backlog candidates already shipped; v2.3 promoted as the TUNE-FUT-02 follow-on. Phases 108 + 109 flagged for phase-level research; 107 + 110 need none.
- 2026-06-23: v2.2 roadmap created (Phases 103-106) from REQUIREMENTS.md (7 v2.2 REQs across BUNDLE/REFGEN/SKILL/TUNE) + research SUMMARY.md. Coarse granularity, sequential phase IDs, numbering continued from v2.1 (ended at 102). Dependency-driven, deterministic-before-exploratory order honored: 103 (BUNDLE-01/02 — installSkill closed-set allowlist + exact-set bundle test + move SKILL-ISSUE.md out of embed dir + harden reference-completeness contract to exact-count==50 BEFORE the churn), 104 (REFGEN-01 — per-verb generator override fixing the cmd/helix-cligen group-collapse; regenerated reference.md passes --check; MUST precede 105), 105 (SKILL-01/02/03 — hand-authored matrix rewrite: split QUERY/ACTION rows, "Not this" everywhere, indexed-graph prereqs, regroup by capability; preserve StripDecisionMatrix anchor + SKILL-04 size cap), 106 (TUNE-01 — exploratory DSPy dev-time/offline harness under tools/, parity-pinned Python adopt metric, overfit/gaming guards, may no-ship, vet-style leakage analyzer; strictly LAST against a frozen surface). 7/7 reqs mapped, 0 unmapped, 0 double-mapped. ZERO new Go deps for 103/104/105; DSPy quarantined out of the binary/module/CI. Both backlog candidates (BL-SKILL-01/02) promoted into this milestone. Phase 106 flagged for phase-level research (spike).
- 2026-06-22: v2.1 roadmap created (Phases 97-102) from REQUIREMENTS.md (23 v1 REQs across REF/STEER/AGENT/ADOPT/VENDOR/EDITBENCH/REPOEVAL/FUZZBENCH/BASELINE) + 5 research files. Two interleavable thrusts, intra-thrust order fixed by hard deps (reference→contract, vendor→baseline, corpus→eval). 6 phases under coarse granularity: 97 (REF+ADOPT-01 substrate+merge-gating contract), 98 (STEER+AGENT multi-agent+steering), 99 (VENDOR mixed-license fixtures), 100 (EDITBENCH+BASELINE-01 wiring+baseline), 101 (ADOPT-02 opt-in LLM scorecard), 102 (REPOEVAL+FUZZBENCH+BASELINE-02 evals+baselines). All 23 reqs mapped, 0 unmapped, 0 double-mapped. ZERO new Go deps; only structural change is skill.go string→embed.FS.
- 2026-06-21: v2.0 roadmap created from REQUIREMENTS.md (31 REQ-IDs across CLI/VERB/OUT/SEC/SKILL/RETIRE/DOCS/TEST) and the 5 research files. Strangler-fig 6-phase shape (90→95) honored; TEST-* threaded into 90/92/93 (E2E oracle early, contract oracle at output-freeze, behavioral oracle with the skill).
- 2026-06-21: v1.12 Bench Stack & Tool Evaluation functionally complete at Phase 89 (15/15 phases); formal `/gsd-complete-milestone` archival pending.
- Phase 96 added: Address v2.0 tech debt

### Critical Roadmap Constraints (v2.3 — Phases 107-110)

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

None yet. (Open question to pin during Phase 108 planning: exact corpus composition — which Aider exercises + count, sized so `val_size > 50` is clearable affordably — the literal v2.2 no-ship axis. Pin the DeepSeek model id at implementation time, and confirm/extend the SWE-bench dataset-name allowlist at Phase 109.)

## Deferred Items

Items carried forward from the v1.12 milestone close (see prior STATE history / v1.12 deferred-items.md for full context):

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| v1.12 audit | Live execution of public-benchmark adapters (Docker/swebench/HF-network paths) honestly gated-skip; decision logic hermetically proven | Tracked | v1.12 close |
| uat (pre-existing) | Phase 75 (v1.12) `75-UAT.md` — status passed, 0 pending scenarios; file simply never formally closed at v1.12 archive | Acknowledged (benign) | v2.0 close |
| quick_task (pre-existing) | `260414-e5n-fix-requirements-md-checkboxes-check-all` — pre-v2.0 quick task, tracker status unknown | Acknowledged | v2.0 close |
| quick_task (pre-existing) | `260617-j29-re-pin-fairness-contract-modelid-to-auth` — pre-v2.0 (v1.12-era) quick task, tracker status unknown | Acknowledged | v2.0 close |
| quick_task (pre-existing) | `260617-t7x-resync-helix-tool-capability-documentation` — work was completed (docgen blank-import resync); tracker status reads unknown, not formally closed | Acknowledged (effectively done) | v2.0 close |

> These four items were surfaced by the v2.0 milestone-close open-artifact audit (2026-06-22). All are pre-existing (none from v2.0 phases 90–96); acknowledged and deferred so the v2.0 close can proceed. None are v2.0 blockers.

> **Re-surfaced at v2.1 close (2026-06-23):** the 3 quick-tasks (`260414-e5n`, `260617-j29`, `260617-t7x`) were re-flagged by the v2.1 open-artifact audit. Triage confirmed all three are **already completed work** (each has a SUMMARY.md with a completion date — `260617-j29` re-pinned `DefaultContract.ModelID` to `claude-sonnet-4-5-20250929`; `260617-t7x` did the docgen tool-manifest resync; `260414-e5n` did the REQUIREMENTS checkbox bookkeeping) — they are stale tracking entries lacking a `status: complete` marker, not real outstanding work. Re-acknowledged; not v2.1 blockers.

> **Re-surfaced again at v2.2 close (2026-06-24):** the same 3 quick-tasks were re-flagged by the v2.2 open-artifact audit (none originate from v2.2 phases 103–106). Triage unchanged — all already-completed stale tracking entries lacking a `status: complete` marker. Acknowledged and deferred so the v2.2 close can proceed; not v2.2 blockers. (Separately, the pre-existing `cmd/helix-bench/TestRunSubcommandWiresDeltaPass` network/HELIX_BIN failure remains out-of-scope tech debt, recorded in the v2.2 milestone audit.)

## Session Continuity

Last session: 2026-06-24 (resumed)
Stopped at: Phase 109 executed inline (uv) and verified (passed); v2.3 now 3/4. Next: Phase 110 (human-gated adoption + boundary re-verify + ship/no-ship REPORT), then milestone close.
Resume file: .planning/.continue-here.md (refreshed to the Phase 110 checkpoint)

## Decisions

- [Phase ?]: Phase 90-01: per-socket gofrs/flock startup lock + double-checked tryConnect in ConnectOrStartDaemon; N parallel cold callers spawn exactly one daemon (CLI-03)
- [Phase ?]: Phase 90-01: synctest seam pattern - startupGuard takes injectable seams; race test uses an in-process mutex locker since real OS flock deadlocks synctest virtual clock
- [Phase ?]: Phase 90-02: no-arg helix (mode=auto) prints grouped cobra help and exits 0 (CLI-04); explicit --mode=stdio still runs forwarder (Phase 94 owns head deletion); 3 command groups scaffold Phase 91 verbs
- [Phase ?]: 90-03: client transport placed in internal/forwarder (no cycle; clirpc fallback unneeded)
- [Phase ?]: 90-03: forwarder.CallTool takes version as a param to avoid an internal/cli↔forwarder import cycle
- [Phase ?]: 90-04: E2E oracle fixed 3 latent 90-03 spine bugs (search_for_pattern wrong tool name, no socket override, cold-start :8080 bind); CLI-02 SLO=max(p50*5,50ms), observed p50 ~12ms
- [Phase ?]: 91-01: AST scan (go/packages+go/ast) recovers tool-name->*Args binding; no manual per-tool table (VERB-04)
- [Phase ?]: 91-01: verbs_gen.go generated+committed; helix-cligen --check drift gate in CI + make verify-cligen (VERB-02)
- [Phase ?]: 91-01: internal/cli.VerbToolNames() is the read-only catalog seam for 91-03 integration tests
- [Phase ?]: SEC-01: tools/call authz returns typed serr.PermissionDenied as an error (not IsError) so errors.Is round-trips CLI-side
- [Phase ?]: ProfileEnforcementMiddleware installed between Guardrail and LazyInit (step 14b.6); LIFO keeps LazyInit first, ProfileEnforce before Guardrail
- [Phase ?]: SEC-02: CLI verb surface = intersection(cli.VerbToolNames(), listSessionTools(profile)); byte-identical to profile allowed set via 91-01 parity, compares cleanly vs unchanged goldens; oracle re-pointed in TestProfile_Contract_Golden
- [Phase ?]: SEC-02: hidden-AND-refused contract — TestProfile_CLI_Surface_Refusal asserts an out-of-profile destructive verb (replace_symbol_body) is absent from CLI surface AND CallTool returns a Go error (91-02 PermissionDenied)
- [Phase ?]: 91-03 deferred: TestProfile_ExcludedToolNotInvocable + TestProfile_ModeAndBudget/switch_mode fail pre-existing under 91-02 enforcement (expect IsError, now get Go error) — out of 91-03 file scope, logged to deferred-items.md
- [Phase ?]: 91-04: Phase 90 dial oracle re-pointed to flat helix <verb> (search-in-files/--pattern); call parent removed in 91-01
- [Phase ?]: 91-04: SEC-01 live proof — ci-bot daemon refuses real helix replace-symbol-body (non-zero exit + typed permission_denied); full profile allows the edit (CLI->gRPC->daemon round trip proven)
- [Phase ?]: 92-02: --json repurposed as a persistent dual-read flag (log-format on daemon path, verb-output JSON on verb path)
- [Phase ?]: 92-02: readSnippetLine clamps the CLI-side snippet read to the workspace root before opening (T-92-04)
- [Phase ?]: 92-03: Contract oracle re-targeted to real helix subprocess stdout goldens (HELIX_BIN-gated); MCP schema meta-validation replaced by default-suite typed-args to cobra-flags parity; OUT-04 chain + OUT-03 self-contained nav snippet proven end-to-end
- [Phase ?]: 93-01: SKILL.md ships via go:embed (string form); installSkill writes it atomically with skills/helix containment; zero new deps, drift-gated against VerbToolNames
- [Phase ?]: 93-02: PreToolUse nudge repurposed to per-call advisory steering grep/sed/cat to helix verbs via hookSpecificOutput.additionalContext, fail-open and exit-0 always (T-93-04)
- [Phase ?]: 93-02: classifyBashTarget uses a static code-extension allowlist (not per-call Registry) for the hot hook path; mixed code+non-code operands classify conservatively as non-code
- [Phase ?]: Phase 93-03: helix setup flipped to skill+hooks install with MCP-only hook-preserving teardown across all 7 clients; daemon MCP head intact for Phase 94
- [Phase ?]: 94-02: CLI is the sole agent-facing MCP surface — both stdio (RunForwarder) and HTTP (/mcp) heads deleted; daemon/wire/middleware engine retained behind forwarder.CallTool
- [Phase ?]: 94-02: --mode retained with only the auto arm; legacy stdio/http modes surface the unknown-mode error
- [Phase ?]: Phase 95-01: docgen derives helix verbs inline via ReplaceAll; docgen drift gate wired as make verify-docs + CI step (closes v1.12 hole); blank-import parity via --check gate + cross-ref comments, not literal equality (D-02 preserved)
- [Phase ?]: Phase 95-02: DOCS-03 satisfied by ADDING a Helix-CLI routing matrix to CLAUDE.md; external SMTC matrix left byte-for-byte intact (mcp__smtc__ count unchanged at 31)
- [Phase ?]: Phase 95-02: docs reframed CLI-first without over-claiming MCP removal — MCP Go SDK + gRPC IPC are retained internal daemon plumbing
- [Phase 96]: TD-01: validateAdminAddr refuses empty-host/wildcard binds via explicit empty-host switch case + !ip.IsUnspecified() guard, preserving the empty-ADDR error contract
- [Phase 96]: TD-03: classifyBashTarget grep-family leading-PATTERN skip gated to grep/rg/ag/egrep/fgrep only; cat/sed/find unchanged; fail-open preserved
- [Phase 96]: TD-04: reworded four get_tool_help metadata literals to 'Helix tool'; regenerated README + tool-descriptions golden via tooling (no hand-edit)
- [Phase ?]: RepoMap-eval gold is file:symbol (relpath:SymbolName, receiver-qualified); leaf is stdlib-only with a TestLeafImports self-test (vet-ablation-leakage does not gate bench/evaluators/*)
- [Phase ?]: RepoMap-eval DiscriminatorMargin=0.30 committed; reversed+seeded-random both bite (fwd 1.0 vs rev 0.005 / rnd 0.52)
- [Phase ?]: Phase 104: refgen per-verb override maps fix collapsed-prose in the generator, not via reference.md hand-edit; categoryToGroup untouched.
- [Phase 105]: 105-01: split get-context (RepoMap, no prereq) from get-semantic-context (Semantic graph, †) so the indexed-graph reader carries the marker without mis-tagging the repomap reader; 43 data rows (37 baseline + 6 net splits)
- [Phase 105]: 105-01: anti-vacuity matrix guards run each pure checker on the real embed (positive arm) + a synthetic fabricated offender (negative arm) parsed by the SAME parseMatrixRows; querySet/actionSet keyed to VerbToolNames() with a completeness gate (every frozen verb classified exactly once)
- [Phase ?]: Phase 106-01: corpus stores RAW first_command (not lowercased); ClassifyChoice lowercases once
- [Phase ?]: Phase 106-01: toolsquarantine analyzer is import-boundary-ONLY so the pip/pipx LS installer is never flagged
- [Phase 109]: grade_swebench.py is a Python parity MIRROR of harness.go (argv/dataset-allowlist/env-allowlist), pinned by shared golden/swebench_argv.json + asserted on BOTH sides (argv_parity_test.go); reuses grade_aider.GradeError and plugs into taskmetric via the same (passed, tests_run) contract
- [Phase 109]: resolution contract recomputed from tests_status (never a bare top-level resolved flag): resolved = non-empty FAIL_TO_PASS all-pass AND no PASS_TO_PASS regression; 0 tests evaluated => GradeError (vacuous-pass refusal, mutation-confirmed RED)
- [Phase 109]: attribution.py decide_ship gates on optimize.VAL_SIZE_GATE (single source of truth, strict >50) AND positive delta; MeteredLLM is a non-invasive cost wrapper (Phase-107 LLM untouched); filled REPORT artifact deferred to Phase 110 (corpus still < val_size>50 by design)
- [Phase 110]: ADOPT-03/04 are re-verification reqs — adoption gate (helix-refgen --check), git-ignored optimizer output, and boundary analyzers all already existed (97/104/106/109); 110 proved them end-to-end (break-the-invariant: desync reference.md => --check exit 1) + wrote the v2.3 ship/no-ship REPORT. NO new production code.
- [Phase 110]: go.mod/go.sum last touched at 7f20a874 feat(90-01) (v2.0) — untouched through v2.1/v2.2/v2.3 => zero new Go deps proven rigorously. openai-go v1.12.0 is a PRE-EXISTING dep, not the Python openai==2.43.0 pin.
- [Phase 110]: v2.3 ship/no-ship verdict = NO-SHIP by design (corpus val_size≈3 < strict >50 gate, the v2.2 root cause); pipeline complete + correctly gated, no fabricated delta. Re-entry precondition: grow corpus past val_size>50 (TUNE-FUT-01).

## Operator Next Steps

- Plan Phase 107 with /gsd-plan-phase 107 (ReAct agent + DeepSeek/OpenAI client + ON/OFF steering; no research needed).
