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
- [x] **v2.4 Corpus Growth & Real Optimization Verdict** -- Phases 111-114 (shipped 2026-06-24) — see `.planning/milestones/v2.4-ROADMAP.md`
- [x] **v2.5 Agent Harness Rebuild** -- Phases 115-118 (shipped 2026-06-26) — see `.planning/milestones/v2.5-ROADMAP.md`
- [x] **v2.6 Turn Budget Tuning** -- Phase 119 (shipped 2026-06-26)
- [x] **v2.7 Corpus Analysis & Benchmark Pivot** -- Phase 120 (shipped 2026-06-26)
- [x] **v2.8 Graph Intelligence Depth (cbm-mcp parity)** -- Phases 121-124 (shipped 2026-06-30) — see `.planning/milestones/v2.8-ROADMAP.md`
- [x] **v2.9 Interprocedural DATA_FLOWS (param-flow reachability substrate)** -- Phases 125-126 (shipped 2026-06-30, `6618de08`) — see `.planning/milestones/v2.9-REQUIREMENTS.md`
- [x] **v2.10 trace_data_flow verb (DATA_FLOWS read surface)** -- Phases 127-129 (shipped 2026-06-30, `12a9ec3c`) — see `.planning/milestones/v2.10-REQUIREMENTS.md`
- [x] **v2.11 Type-resolution depth (C-family)** -- Phases 130-134 (shipped 2026-07-01, `d43d7ce7`..`97fc4bc9`) — see `.planning/milestones/v2.11-REQUIREMENTS.md`
- [x] **v2.12 Production wiring for the type resolvers (C-family E2E)** -- Phases 135-137 (shipped 2026-07-01) — see `.planning/milestones/v2.12-ROADMAP.md` + `.planning/milestones/v2.12-MILESTONE-AUDIT.md`
- [x] **v2.13 True intraprocedural DATA_FLOWS (in-body origins, variable-level)** -- Phases 138-140 (shipped 2026-07-01) — see `.planning/milestones/v2.13-ROADMAP.md` + `.planning/milestones/v2.13-MILESTONE-AUDIT.md`
- [x] **v2.14 Codebase-Map Re-mapping (Go tree)** -- Phases 141-143 (shipped 2026-07-01) — see `.planning/milestones/v2.14-REQUIREMENTS.md` + `.planning/milestones/v2.14-MILESTONE-AUDIT.md`

## Phases

**Current: v2.14 Codebase-Map Re-mapping (Phases 141-143) — SHIPPED 2026-07-01, MILESTONE-AUDIT PASSED.** An
assess-and-document milestone: re-mapped all 7 `.planning/codebase/*.md` files from the
**removed pre-Go Python `serena` tree** (2026-04-07, the map's sole commit — ~13
milestones stale) to the **current Go tree** at HEAD (85,472 non-test LOC; 24
`internal/` packages; largest subsystem `internal/semantic/` at 30.8k LOC; 16 `cmd/`
binaries; 51 frozen `helix` verbs). Flagged HIGH by the v2.13 MILESTONE-AUDIT.
Documentation-only — no source change; every claim source-grounded at HEAD; residue
clean (only labeled retained-lineage remains). Bundled the planning-integrity fix that
regenerated this very section (the stale v2.5-era detail here is what made
`roadmap analyze` manufacture phantom incomplete phases 121-123).

> **Per-milestone phase detail is canonical in `.planning/milestones/` and per-phase
> `CONTEXT.md`/`SUMMARY.md`, not duplicated here.** This section carries only the
> in-flight milestone; shipped milestones are summarized under **Earlier Milestones**
> below and linked to their `.planning/milestones/vX.Y-*` records. (Prior to v2.14
> this section retained stale detail for already-shipped phases 115-126, which the
> tooling mis-read as incomplete — do not reintroduce that pattern.)

### Phase 141: Structural + Stack skeleton (MAP-01, MAP-03)

**Goal:** Re-map `STRUCTURE.md` (current Go directory layout: 16 `cmd/` binaries, 24
`internal/` packages with per-package purpose, `api/proto`, `protocol/{gen,patch}`,
`bench/`, `tools/dspy-tune/`, with 4-layer mapping + LOC/scale context) and `STACK.md`
(Go 1.25.1, CGO=1 split-runner build, direct deps from `go.mod`, no runtime Python).
The factual skeleton the deeper files reference.

### Phase 142: Architecture + Integrations + Conventions (MAP-02, MAP-04, MAP-05)

**Goal:** Re-map `ARCHITECTURE.md` (4 layers, daemon bootstrap + LIFO middleware
stack, the `internal/semantic/` index surveyed fresh, key abstractions, entry points,
error handling, cross-cutting concerns), `INTEGRATIONS.md` (LSP three-tier installer,
tree-sitter grammars, gRPC IPC, sigstore, prometheus/otel, container auto-detect, LLM
SDKs dev-time-only), and `CONVENTIONS.md` (Go naming, gofmt/vet + custom `cmd/vet-*`
gates, typed errors, skill `init()` registration, generated-code discipline).

### Phase 143: Concerns + Testing + integrity gate (MAP-06, MAP-07, MAP-08)

**Goal:** Re-map `CONCERNS.md` (real tech debt / fragile seams / security posture /
vet-enforced invariants / retained-lineage naming artifacts — cite source or omit)
and `TESTING.md` (`go test`, testify, `testdata/` fixtures, vet-gate suite, real-binary
E2E, `bench/` harness + CI workflows). Then the integrity gate: remove all staleness
banners, refresh every `Analysis Date`, and grep-confirm zero Python-serena residue
beyond the deliberately-retained lineage artifacts.

## Requirement Coverage

| REQ-ID | Phase | Status |
|--------|-------|--------|
| MAP-01 | 141 | planned |
| MAP-03 | 141 | planned |
| MAP-02 | 142 | planned |
| MAP-04 | 142 | planned |
| MAP-05 | 142 | planned |
| MAP-06 | 143 | planned |
| MAP-07 | 143 | planned |
| MAP-08 | 143 | planned |

**Coverage:** 8/8 REQs mapped (100%)

---

## Earlier Milestones

### ✅ v2.4 Corpus Growth & Real Optimization Verdict (Phases 111-114) — SHIPPED 2026-06-24

4 phases, 10 requirements (CORPUS-01/02, SCALE-01/02/03, RUN-01/02/03, REPORT-01, ADOPT-05), 100% mapped. Grew the optimization corpus past the strict `val_size > 50` held-out gate and ran the v2.3 task-success pipeline **for real** (cost-aware, $0.23) — turning v2.3's "NO-SHIP by design" into an actual, numbers-backed verdict: **SHIP-by-rule, marginal** (ON-vs-OFF attribution delta **+0.0392**, ON 3/51 vs OFF 1/51, on a sequestered held-out split of 51 > 50), with a SWE-bench Verified gold-patch confirm on Podman (**2/2 resolved**, fail-not-skip proven). **REPORT-only** — adoption NOT recommended on this thin/noise margin; it stays a separate human `helix-refgen --check`-gated step. The milestone's real win: the v2.3 pipeline was never functional end-to-end (its hermetic fakes hid three integration gaps — agent verb argv, a `helix activate`→`activate_project` **real product bug**, and the GEPA candidate→agent steering thread); all three were fixed. **Zero new Go deps** (go.mod untouched since v2.0); the only Go change is the user-approved product-bug fix. Audit PASSED — 10/10 reqs, 4/4 phases, E2E real run.

**Full details:** `.planning/milestones/v2.4-ROADMAP.md`

### ✅ v2.3 Task-Success-Driven Skill Optimization (Phases 107-110) — SHIPPED 2026-06-24

4 phases, 10 requirements (AGENT-01/02/03, ORACLE-01/02, TUNE-02/03/04, ADOPT-03/04), 100% mapped. Fixed the v2.2 no-ship root cause by replacing the gameable `choice_rate` adoption proxy with a real **agent task-success** optimization signal: a dev-time OpenAI-compatible ReAct agent drives the `helix` CLI via subprocess (107); GEPA's metric was rewired from `choice_rate` to honest benchmark task-success — Aider hidden-tests-green (108) and SWE-bench FAIL_TO_PASS-flip + PASS_TO_PASS-no-regression on Podman (109); every gain is attributed via a mandatory ON-vs-OFF control arm on a sequestered `val_size>50` split (109); adoption is human-gated via `helix-refgen --check` (110). Zero new Go deps, no runtime Python — the agent + optimizer live entirely in `tools/dspy-tune/`, off `go.mod` / `helix setup` / default `go test ./...` / the merge path. Verdict: **NO-SHIP by design** (corpus still below the `val_size>50` gate; TUNE-FUT-01 grows it). Audit PASSED — 10/10 reqs, 4/4 phases, 5/5 integration seams, E2E wired.

**Full details:** `.planning/milestones/v2.3-ROADMAP.md`

### ✅ v2.2 Agent-Facing Skill Quality & Prompt Tuning (Phases 103-106) — SHIPPED 2026-06-24

4 phases, 7 requirements (BUNDLE-01/02, REFGEN-01, SKILL-01/02/03, TUNE-01), 100% mapped. A content/codegen milestone: rewrote the agent-facing skill surface (hand-authored `SKILL.md` decision matrix + generated `reference.md`) for correctness, hardened the skill bundle so only `{SKILL.md, reference.md}` ship, and explored a quarantined dev-time DSPy offline-tuning harness against the Phase 101 adoption scorecard. Zero new Go dependencies; the DSPy spike stays strictly out of the shipped binary, `go.mod`, and `go test ./...`. Every gate ships a deliberate break-the-invariant → assert-RED test (code review caught and fixed a vacuous Guard B in 104 and a real Python↔Go parity bug in 106). Audit PASSED — 7/7 reqs, 4/4 phases, integration wired, 3/3 E2E flows, Nyquist 4/4.

**Full details:** `.planning/milestones/v2.2-ROADMAP.md`

### ✅ v2.1 Agent Adoption & Aider-Derived Validation (Phases 97-102) — SHIPPED 2026-06-23

6 phases, 23 requirements, 100% mapped. Make AI coding agents reliably reach for `helix` verbs over standard tools (generated per-verb reference + stronger multi-agent steering + a measured adoption contract), and prove the toolset end-to-end with vendored Aider benchmarks and committed local baselines (polyglot edit bench + RepoMap-quality and fuzzy-robustness evals). Additive — zero new Go dependencies; the only structural change was `internal/cli/skill.go` switching from an embedded `string` to an `embed.FS`. Audit PASSED — 23/23 reqs, 6/6 phases, 2/2 cross-phase E2E flows wired.

**Full details:** `.planning/milestones/v2.1-ROADMAP.md`

### ✅ v2.0 CLI-First — MCP Surface Retirement (Phases 90-96) — SHIPPED 2026-06-22

7 phases (6 feature + 1 inserted post-audit tech-debt cleanup), 31 v1 requirements, 100% mapped. The `helix` CLI is the only agent-facing surface; the MCP Go SDK + gRPC IPC are retained as internal daemon plumbing. Strangler-fig: the CLI head was built behind the still-live MCP surface (90–93), parity proven by dual-run, the agent-facing MCP heads deleted **last** (94), docs/identity + docgen regen run against the frozen surface (95), and the non-blocking audit tech debt cleared (96). Re-audit PASSED — 31/31 reqs, 7/7 phases, 4/4 E2E flows, Nyquist 7/7.

**Full details:** `.planning/milestones/v2.0-ROADMAP.md`

### ✅ v1.12 Bench Stack & Tool Evaluation (Phases 75-89) — SHIPPED 2026-06-21

15 phases, 63 v1 requirements, 100% mapped. Headline claim: *"Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits."*

**Full details:** `.planning/milestones/v1.12-ROADMAP.md`

---

## Follow-ons (Non-Blocking)

- **TUNE-FUT-03:** Human-gated SKILL.md adoption (separate milestone, after meaningful delta)
- **TUNE-FUT-05:** Larger SWE-bench K for confirmation
- **Significance test:** Add statistical significance to `decide_ship`