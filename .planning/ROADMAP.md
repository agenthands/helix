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
- [ ] **v2.2 Agent-Facing Skill Quality & Prompt Tuning** -- Phases 103-106 (planning)

## Phases

### 🚧 v2.2 Agent-Facing Skill Quality & Prompt Tuning (Phases 103-106) — PLANNING

**Milestone Goal:** Rewrite the helix agent-facing skill surface (hand-authored `SKILL.md` decision matrix + generated `reference.md`) for correctness and completeness, harden the skill bundle so only `{SKILL.md, reference.md}` ship, and explore DSPy-based offline tuning of that surface against the Phase 101 adoption scorecard. A content/codegen milestone, NOT a stack milestone: the three deterministic Go-side fixes add ZERO new Go dependencies (edits inside already-vendored packages); only the exploratory DSPy spike introduces anything new, and it stays strictly out of the shipped binary, the Go module graph, and the merge-gating CI path.

**Coverage:** 7/7 v2.2 requirements mapped (BUNDLE-01/02, REFGEN-01, SKILL-01/02/03, TUNE-01) — 0 unmapped, 0 double-mapped.

**Cross-cutting constraints (carried into success criteria where relevant):**

- **No runtime Python / single-binary preserved** — DSPy is dev-time/offline only; no `helix` subcommand shells to Python, no `go.mod`/`helix setup` edge; a `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path.
- **`reference.md` is generated, never hand-edited** — all reference corrections go through the generator; the Phase 97 `helix-refgen --check` byte-reproducibility gate and the `reference ⊇ VerbToolNames()` contract stay green.
- **Anti-vacuity (every gate)** — each new/hardened gate (closed-set bundle test, hardened exact-count contract, generator vacuity guards, leakage analyzer) ships a deliberate break-the-invariant → assert-RED test. A gate with only a green-path test is presumed broken (anchored to the repeated Phase 86/87/89 CR-01 vacuous-pass class).

**Phase ordering rationale (dependency-driven, deterministic-before-exploratory — load-bearing, from research SUMMARY):**

- **103 before 104/105:** the embed-glob leak is LIVE (the binary + every `helix setup` already ship the 18 KB `SKILL-ISSUE.md`); the closed-set allowlist must land BEFORE the dir churns, and the `reference ⊇ VerbToolNames()` contract must be hardened to exact-count==50 BEFORE the format rewrite or it can pass `∅ ⊇ ∅` vacuously.
- **104 (generator/reference) before 105 (SKILL.md):** the on-demand reference must be correct before the idle-tier matrix is re-authored, so the two tell one consistent story.
- **106 strictly last:** DSPy tunes against a frozen metric and the stabilized 104/105 surface; it is an exploratory spike with a possible no-ship outcome and a clean Go-search-loop fallback.

- [x] **Phase 103: Bundle Integrity & Non-Vacuous Reference Contract** - `installSkill` closed-set allowlist + exact-set bundle test, move `SKILL-ISSUE.md` out of the embed dir, harden the reference-completeness contract to exact-count (==50) discriminating a known-absent verb (completed 2026-06-23)
- [x] **Phase 104: Reference Generator Per-Verb Correctness** - fix the `cmd/helix-refgen`/`cmd/helix-cligen` group-collapse via per-verb overrides so every "use this/not that" + "Output" line is correct; regenerated `reference.md` passes `--check` byte-for-byte (completed 2026-06-23)
- [x] **Phase 105: SKILL.md Decision-Matrix Rewrite** - split QUERY/ACTION rows, "Not this" on every row, indexed-graph prerequisite notes, regroup by capability; preserve the `## Decision matrix` StripDecisionMatrix anchor + the SKILL-04 idle-cost size cap (completed 2026-06-23)
- [ ] **Phase 106: Exploratory DSPy Offline Tuning Harness (spike)** - opt-in dev-time-only Python harness under `tools/`, no runtime Python dep, off `go test ./...`; optimizes against the Phase 101 `adopt` scorecard (parity-pinned Python metric) with overfit/gaming guards; may no-ship; a `vet`-style analyzer blocks Python/runtime leakage

## Phase Details

### Phase 103: Bundle Integrity & Non-Vacuous Reference Contract

**Goal**: `helix setup` installs exactly the two skill-bundle files and nothing else, the embedded bundle can no longer leak stray files into the binary or onto users' disks, and the reference-completeness gate is hardened to be discriminating BEFORE any reference/skill text churns.
**Depends on**: Nothing (first phase of v2.2; the smallest-blast-radius safety patch)
**Requirements**: BUNDLE-01, BUNDLE-02
**Success Criteria** (what must be TRUE):

  1. After `helix setup`, the installed skill directory contains exactly `{SKILL.md, reference.md}` and no other file — a bundle-contents test asserts the installed set equals exactly that pair and goes RED on any stray file in `internal/cli/skills/helix/`.
  2. `SKILL-ISSUE.md` no longer compiles into the `helix` binary or installs to users — it has been moved out of the embed dir, and `installSkill`/`uninstallSkill` drive an explicit closed-set allowlist (filter applied once up front; atomic stage→rename and `withinSkillRoot` containment unchanged; uninstall filtered identically).
  3. The reference-completeness contract test is non-vacuous: it asserts the generated `reference.md` covers the frozen verb set by EXACT count (`== len(VerbToolNames())`, currently 50) and goes RED when a known verb is absent — proven by a deliberate break-the-invariant assertion that runs RED before the fix.
  4. The hardened contract lands BEFORE the Phase 104/105 reference and skill rewrites, so the superset check cannot pass vacuously (`∅ ⊇ ∅`) through the format churn.

**Plans**: 1/1 plans complete

- [x] 103-01-PLAN.md — closed-set bundleFiles allowlist on install+uninstall + RED-first closed-set bundle test, move SKILL-ISSUE.md out of the embed dir, seal the reference contract with a fabricated-absent-verb discriminator (BUNDLE-01, BUNDLE-02)

### Phase 104: Reference Generator Per-Verb Correctness

**Goal**: Every verb's "use this, not that" and "Output" lines in the generated `reference.md` are correct for that verb's actual semantics, fixed at the generator root cause (the group collapse), with the corrected `reference.md` regenerated, committed, and reproducible.
**Depends on**: Phase 103 (clean embed dir → no stray-file noise in the generated bundle or its tests)
**Requirements**: REFGEN-01
**Success Criteria** (what must be TRUE):

  1. The group-collapse root cause is fixed in the GENERATOR (`cmd/helix-refgen`/`cmd/helix-cligen`) via a per-verb override map with group-default fallback — NOT by hand-editing the generated `reference.md` — so verbs that previously inherited the wrong group's `memory`-query prose (e.g. `switch-mode`, `get-token-budget`, `onboard-project`, `get-health`, `get-tool-help`, mutating memory verbs) now read correctly.
  2. The regenerated `reference.md` is committed and passes `helix-refgen --check` byte-for-byte (`git diff` is empty after a fresh regen).
  3. The `reference ⊇ VerbToolNames()` contract and the Phase 97 `--check` drift gate stay green, and the blank-import parity between the generator and the daemon is re-verified when touching refgen.
  4. Vacuity guards prove the override map is real: no override key is a non-verb, and an overridden Output line differs from the old group default it replaced (deliberate break-the-invariant test).

**Plans**: 1/1 plans complete

- [x] 104-01-PLAN.md — RED override golden + SC #4 vacuity guards → per-verb override maps + extracted group-default helpers in cmd/helix-refgen/render.go → regenerate reference.md, --check/cligen/docgen parity green

### Phase 105: SKILL.md Decision-Matrix Rewrite

**Goal**: The hand-authored `SKILL.md` decision matrix routes an agent's single intent to a single correct tool, with no QUERY/ACTION row mixing, explicit "Not this" guidance on every row, indexed-graph prerequisite notes, and capability-based grouping — consistent with the now-correct generated reference.
**Depends on**: Phase 104 (the on-demand reference must be correct first, so SKILL.md and reference.md tell one consistent story)
**Requirements**: SKILL-01, SKILL-02, SKILL-03
**Success Criteria** (what must be TRUE):

  1. No decision-matrix row mixes a QUERY (read-state) verb with an ACTION (mutate-state) verb — query and action verbs occupy separate rows (resolves the SKILL-ISSUE.md rows 68/70/71/72/73 grouping errors).
  2. Every decision-matrix row carries explicit "Not this" guidance — no `—` placeholders remain (the concrete grep/sed/cat/find fallback each verb displaces is named, per the CLAUDE.md routing table as canonical source).
  3. Every indexed-graph verb (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`) carries a "requires `index-semantic-graph` first" prerequisite note, and the matrix is grouped by capability.
  4. The `## Decision matrix` heading (the `StripDecisionMatrix` anchor) is preserved, the rewritten `SKILL.md` stays under the SKILL-04 idle-cost (size) cap, and a SKILL.md↔`VerbToolNames()` cross-check test holds.

**Plans**: 1/1 plans complete

- [x] 105-01-PLAN.md — RED 3 anti-vacuity matrix guards (SKILL-01/02/03) → GREEN rewrite the SKILL.md decision matrix (split QUERY/ACTION, fill "Not this", graph prereqs, capability grouping); preserve anchor + cap + drift cross-check

### Phase 106: Exploratory DSPy Offline Tuning Harness (spike)

**Goal**: An opt-in, dev-time-only DSPy harness can optimize the agent-facing skill/steering text against the Phase 101 adoption scorecard metric and report whether tuning beats the deterministic baseline — with a possible no-ship outcome — while the shipped `helix` binary and `go test ./...` remain 100% Python-free.
**Depends on**: Phase 105 (tunes against the stabilized 104/105 surface and a frozen metric; tuning a moving target wastes optimizer budget)
**Requirements**: TUNE-01
**Success Criteria** (what must be TRUE):

  1. The DSPy harness lives under `tools/` as an opt-in dev-time tree (its own pinned `requirements.txt` + git-ignored venv/output), is excluded from `go test ./...`, and adds NO runtime Python dependency to the `helix` binary or `helix setup`.
  2. The harness optimizes the agent-facing skill/steering text against the Phase 101 adoption metric, re-implemented in Python with a golden parity cross-check against the Go `test/oracle/adopt` classifier (the same `choice_rate`/`fallback_rate` scorer drives both the Go gate and the Python optimizer).
  3. Overfit and metric-gaming guards are in place — a held-out TEST split the optimizer never sees, and a degenerate-steering inspection — and the harness may legitimately conclude no-ship (the clean fallback being a hand-rolled Go candidate-search loop keeping the milestone 100% Go).
  4. Any adopted output re-enters only as a human-reviewed commit through SKILL.md/refgen and passes `helix-refgen --check`; a `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path (the artifact is gated, never the optimizer process).

**Plans**: TBD
**Needs phase-level research**: yes — exploratory spike (corpus-split design, Python↔Go classifier parity contract, metric-AND-quality-oracle composition, leakage-analyzer design); highest uncertainty, mark as spike not a hard adoption-delta gate.

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 103. Bundle Integrity & Non-Vacuous Reference Contract | v2.2 | 1/1 | Complete   | 2026-06-23 |
| 104. Reference Generator Per-Verb Correctness | v2.2 | 1/1 | Complete    | 2026-06-23 |
| 105. SKILL.md Decision-Matrix Rewrite | v2.2 | 1/1 | Complete    | 2026-06-23 |
| 106. Exploratory DSPy Offline Tuning Harness | v2.2 | 0/TBD | Not started | - |

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

### ✅ Promoted — v2.2: Agent-Facing Skill Quality & Prompt Tuning

These two backlog candidates were promoted into the active **v2.2** milestone (Phases 103-106, see `## Phases` above) on 2026-06-23 via `/gsd-new-project`. They are retained here for lineage; the v2.2 phases supersede them.

- **BL-SKILL-01 → Phases 103-105** (BUNDLE-01/02 + REFGEN-01 + SKILL-01/02/03). Full SKILL.md + reference.md decision-matrix rewrite plus the `installSkill` allowlist + bundle-contents test. The original seed listed: (1) split rows that mix QUERY verbs with ACTION verbs that mutate state; (2) add the missing "Not this" guidance to every row (8+ rows currently `—`); (3) fix the `reference.md` "Use this, not that" copy-paste errors and incorrect "Output" descriptions; (4) add prerequisite notes for the indexed-graph verbs (all require `index-semantic-graph` first); (5) regroup the matrix by capability. Must stay consistent with the Phase 97 generator (`cmd/helix-refgen`) + `--check` drift gate and the `reference ⊇ VerbToolNames()` adoption contract — i.e. fix the GENERATOR, not hand-edit generated output. The allowlist + bundle-contents test became Phase 103; the generator fix became Phase 104; the SKILL.md matrix rewrite became Phase 105.

- **BL-SKILL-02 → Phase 106** (TUNE-01). DSPy-based offline prompt tuning of the agent-facing surface (exploratory). **Constraint:** Helix ships as a Go single binary with no Python/runtime deps — DSPy is a **dev-time/offline optimization harness** (Python, under `tools/`) that emits an optimized, committed `SKILL.md`/`reference.md`, NOT a runtime dependency. **Metric already exists:** the Phase 101 opt-in LLM-behavioral adoption scorecard (`test/oracle/adopt` — `choice_rate`/`fallback_rate`, keyed on the first emitted command) is the DSPy objective, closing the loop from v2.1's measurement work to v2.2's optimization. Promoted as the strictly-last, exploratory Phase 106 (possible no-ship; clean Go-search-loop fallback).
