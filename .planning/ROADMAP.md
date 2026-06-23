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

## Phases

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

### Candidate milestone — v2.2: Agent-Facing Skill Quality & Prompt Tuning

Seed scope for the next milestone (capture only — not yet planned via `/gsd-new-milestone`). Source analysis: `internal/cli/skills/helix/SKILL-ISSUE.md` (authored by the maintainer).

- **BL-SKILL-01 — Full SKILL.md + reference.md decision-matrix rewrite.** The current `internal/cli/skills/helix/SKILL.md` decision matrix is thin and incorrectly structured. Per `SKILL-ISSUE.md`: (1) split rows that mix QUERY verbs with ACTION verbs that mutate state (e.g. `get-semantic-graph-status` vs `index-semantic-graph`/`refresh-semantic-graph`; `read-memory`/`list-memories` vs `write-memory`; `search-memories` vs `rename`/`edit`/`delete-memory`; `switch-mode` vs `get-token-budget`); (2) add the missing "Not this" guidance to every row (8+ rows currently `—`); (3) fix the `reference.md` "Use this, not that" copy-paste errors and incorrect "Output" descriptions; (4) add prerequisite notes for the indexed-graph verbs (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context` all require `index-semantic-graph` first); (5) regroup the matrix by capability. Must stay consistent with the Phase 97 generator (`cmd/helix-refgen`) + `--check` drift gate and the `reference ⊇ VerbToolNames()` adoption contract — i.e. the rewrite likely means improving the generator/templates, not hand-editing generated output. Also tighten `installSkill` to an explicit allowlist (SKILL.md + reference.md) + a bundle-contents test so stray files in `internal/cli/skills/helix/` can no longer leak into the binary/installed skill.

- **BL-SKILL-02 — DSPy-based offline prompt tuning of the agent-facing surface (exploratory).** Use DSPy (Stanford) to optimize the SKILL.md decision-matrix / nudge-steering prompt text against a measurable adoption metric. **Constraint:** Helix ships as a Go single binary with no Python/runtime deps — DSPy would be a **dev-time/offline optimization harness** (Python, under e.g. `tools/` or `bench/`) that emits an optimized, committed `SKILL.md`/`reference.md`, NOT a runtime dependency. **Metric already exists:** the Phase 101 opt-in LLM-behavioral adoption scorecard (`test/oracle/adopt` — `choice_rate`/`fallback_rate`, keyed on the first emitted command) is the natural DSPy objective, closing the loop from v2.1's measurement work to v2.2's optimization. Open questions for `/gsd-discuss-phase` when promoted: DSPy optimizer choice (MIPROv2 / BootstrapFewShot), train/dev task corpus source (reuse the adopt fixtures + vendored exercism tasks), and how to keep the optimized output reproducible/diffable under the existing `helix-refgen --check` gate.

_Promote via `/gsd-new-milestone` (after v2.1 is closed) or `/gsd-phase --add` once scoped._
