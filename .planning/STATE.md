---
gsd_state_version: 1.0
milestone: v2.2
milestone_name: Agent-Facing Skill Quality & Prompt Tuning
current_phase: 105
current_phase_name: SKILL.md Decision-Matrix Rewrite
status: executing
stopped_at: v2.2 roadmap created (Phases 103-106)
last_updated: "2026-06-23T22:26:25.359Z"
last_activity: 2026-06-23
last_activity_desc: Phase 104 complete, transitioned to Phase 105
progress:
  total_phases: 4
  completed_phases: 2
  total_plans: 2
  completed_plans: 2
  percent: 50
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-23)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** Phase 104 — Reference Generator Per-Verb Correctness

## Current Position

Phase: 105 — SKILL.md Decision-Matrix Rewrite
Plan: Not started
Status: Ready to execute
Last activity: 2026-06-23 — Phase 104 complete, transitioned to Phase 105

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

## Accumulated Context

### Roadmap Evolution

- 2026-06-23: v2.2 roadmap created (Phases 103-106) from REQUIREMENTS.md (7 v2.2 REQs across BUNDLE/REFGEN/SKILL/TUNE) + research SUMMARY.md. Coarse granularity, sequential phase IDs, numbering continued from v2.1 (ended at 102). Dependency-driven, deterministic-before-exploratory order honored: 103 (BUNDLE-01/02 — installSkill closed-set allowlist + exact-set bundle test + move SKILL-ISSUE.md out of embed dir + harden reference-completeness contract to exact-count==50 BEFORE the churn), 104 (REFGEN-01 — per-verb generator override fixing the cmd/helix-cligen group-collapse; regenerated reference.md passes --check; MUST precede 105), 105 (SKILL-01/02/03 — hand-authored matrix rewrite: split QUERY/ACTION rows, "Not this" everywhere, indexed-graph prereqs, regroup by capability; preserve StripDecisionMatrix anchor + SKILL-04 size cap), 106 (TUNE-01 — exploratory DSPy dev-time/offline harness under tools/, parity-pinned Python adopt metric, overfit/gaming guards, may no-ship, vet-style leakage analyzer; strictly LAST against a frozen surface). 7/7 reqs mapped, 0 unmapped, 0 double-mapped. ZERO new Go deps for 103/104/105; DSPy quarantined out of the binary/module/CI. Both backlog candidates (BL-SKILL-01/02) promoted into this milestone. Phase 106 flagged for phase-level research (spike).
- 2026-06-22: v2.1 roadmap created (Phases 97-102) from REQUIREMENTS.md (23 v1 REQs across REF/STEER/AGENT/ADOPT/VENDOR/EDITBENCH/REPOEVAL/FUZZBENCH/BASELINE) + 5 research files. Two interleavable thrusts, intra-thrust order fixed by hard deps (reference→contract, vendor→baseline, corpus→eval). 6 phases under coarse granularity: 97 (REF+ADOPT-01 substrate+merge-gating contract), 98 (STEER+AGENT multi-agent+steering), 99 (VENDOR mixed-license fixtures), 100 (EDITBENCH+BASELINE-01 wiring+baseline), 101 (ADOPT-02 opt-in LLM scorecard), 102 (REPOEVAL+FUZZBENCH+BASELINE-02 evals+baselines). All 23 reqs mapped, 0 unmapped, 0 double-mapped. ZERO new Go deps; only structural change is skill.go string→embed.FS.
- 2026-06-21: v2.0 roadmap created from REQUIREMENTS.md (31 REQ-IDs across CLI/VERB/OUT/SEC/SKILL/RETIRE/DOCS/TEST) and the 5 research files. Strangler-fig 6-phase shape (90→95) honored; TEST-* threaded into 90/92/93 (E2E oracle early, contract oracle at output-freeze, behavioral oracle with the skill).
- 2026-06-21: v1.12 Bench Stack & Tool Evaluation functionally complete at Phase 89 (15/15 phases); formal `/gsd-complete-milestone` archival pending.
- Phase 96 added: Address v2.0 tech debt

### Critical Roadmap Constraints (v2.2 — Phases 103-106)

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

None yet.

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

## Session Continuity

Last session: 2026-06-23T21:49:51.922Z
Stopped at: v2.2 roadmap created (Phases 103-106)
Resume file: None

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

## Operator Next Steps

- Plan Phase 103 with /gsd-plan-phase 103 (Bundle Integrity & Non-Vacuous Reference Contract — BUNDLE-01/02)
