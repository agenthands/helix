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

## Phases

**Current: No active milestone** — v2.6 complete, awaiting next phase decision

### Phase 119: Increase Turn Budget (COMPLETE)

**Goal:** Give the agent more time to solve tasks and re-measure the steering delta.

**Result:** delta=+0.0000, same as v2.5. Both arms solve 0/51 tasks even with 20 turns.
The agent terminates on `no_progress` early — the turn budget isn't the limiting factor.
Corpus is too hard for current agent capability.

**Recommendation:** Phase 120 (Corpus Analysis) — filter to easier tasks or switch benchmarks.

### Phase 115: Task-Solving Prompt + Feedback Loop

**Goal:** Make the agent actually edit files and verify success — the foundation for all tuning.

**Why first:** You can't tune an agent that doesn't use tools. HARNESS-01 and HARNESS-02 fix the core harness.

**Requirements:**
- HARNESS-01a: System prompt names solution file
- HARNESS-01b: Success criterion = hidden tests pass
- HARNESS-01c: Prose forbidden
- HARNESS-01d: Anti-vacuity test (prose → FAIL)
- HARNESS-02a: run-tests in ReAct loop
- HARNESS-02b: get-diagnostics in ReAct loop
- HARNESS-02c: Agent uses test results to iterate
- HARNESS-02d: Anti-vacuity test (done-on-broken → FAIL)

**Success criteria:**
1. Agent tool-call rate ≥ 80% on held-out split (demonstrates engagement)
2. All anti-vacuity tests assert FAIL on broken harness
3. `make vet` + `go test ./...` green
4. No new Go deps (`go.mod` unchanged)

**Exit gate:** Agent demonstrably edits files AND runs tests AND uses results to iterate.

---

### Phase 116: Verb-Arg Hardening

**Goal:** Stop burning error budget on malformed calls.

**Why second:** Requires Phase 115 — can't harden verbs if agent doesn't use them.

**Requirements:**
- HARNESS-03a: Audit verb call sites for positional-arg misuse
- HARNESS-03b: Add error-budget tracking
- HARNESS-03c: Anti-vacuity test (malformed argv → FAIL)

**Success criteria:**
1. All verb call sites audited for `--flag=val` pattern
2. Error-budget counter tracks verb errors and aborts at threshold
3. Anti-vacuity test asserts FAIL on malformed argv
4. `make vet` + `go test ./...` green
5. No new Go deps

**Exit gate:** Verb errors are budgeted and bounded.

---

### Phase 117: Real GEPA Module

**Goal:** Make GEPA actually evolve — rebuild as a `dspy.Module` that emits a reflectable trace.

**Why third:** Requires Phases 115–116 — can't tune GEPA if agent still fails on basics.

**Requirements:**
- HARNESS-04a: Agent rebuilt as `dspy.Module` (not ad-hoc Python)
- HARNESS-04b: AgentProgram emits predictor trace
- HARNESS-04c: GEPA `forward` returns trace for reflective mutation
- HARNESS-04d: Anti-vacuity test (empty trace → FAIL)

**Success criteria:**
1. Agent is a subclass of `dspy.Module` with proper `forward` signature
2. `optimize.py` runs without errors on the real agent
3. Trace is non-empty and varies across candidates
4. Anti-vacuity test asserts FAIL on empty/constant trace
5. `make vet` + `go test ./...` green (Python tests in `tools/dspy-tune/`)
6. No new Go deps

**Exit gate:** GEPA can actually optimize the agent (non-trivial trace, reflective mutation proposes candidates).

---

### Phase 118: Re-run Attribution + Verdict

**Goal:** Measure a meaningful delta on a fixed harness, gate for adoption.

**Why fourth:** Requires Phases 115–117 — can't measure meaningful delta on broken agent.

**Requirements:**
- HARNESS-05a: Re-run v2.4 attribution pipeline on fixed harness
- HARNESS-05b: Verify agent tool-call rate ≥ 80% on held-out
- HARNESS-05c: Record ON/OFF delta with per-arm cost
- HARNESS-05d: Gate for TUNE-FUT-03 (adoption path ready if delta significant)

**Success criteria:**
1. Attribution pipeline runs end-to-end on fixed harness
2. Tool-call rate ≥ 80% demonstrated (the engagement bar)
3. ON-vs-OFF delta recorded with cost breakdown
4. REPORT.md updated with honest verdict and caveats
5. If delta > 0 and significant: SKILL.md adoption path documented
6. If delta ≈ 0 or noise: documented as "not yet ready for adoption"
7. `make vet` + `go test ./...` green
8. No new Go deps

**Exit gate:** Measured verdict with honest caveats. Adoption gate (TUNE-FUT-03) ready if delta is meaningful.

---

## Requirement Coverage

| REQ-ID | Phase | Status |
|--------|-------|--------|
| HARNESS-01a | 115 | planned |
| HARNESS-01b | 115 | planned |
| HARNESS-01c | 115 | planned |
| HARNESS-01d | 115 | planned |
| HARNESS-02a | 115 | planned |
| HARNESS-02b | 115 | planned |
| HARNESS-02c | 115 | planned |
| HARNESS-02d | 115 | planned |
| HARNESS-03a | 116 | planned |
| HARNESS-03b | 116 | planned |
| HARNESS-03c | 116 | planned |
| HARNESS-04a | 117 | planned |
| HARNESS-04b | 117 | planned |
| HARNESS-04c | 117 | planned |
| HARNESS-04d | 117 | planned |
| HARNESS-05a | 118 | planned |
| HARNESS-05b | 118 | planned |
| HARNESS-05c | 118 | planned |
| HARNESS-05d | 118 | planned |

**Coverage:** 19/19 REQs mapped (100%)

---

## Constraint Verification

| Constraint | Phase 115 | Phase 116 | Phase 117 | Phase 118 |
|------------|-----------|-----------|-----------|-----------|
| Zero new Go deps | ✓ | ✓ | ✓ | ✓ |
| Tuning in dev-venv Python | ✓ | ✓ | ✓ | ✓ |
| Anti-vacuity tests | ✓ (01d, 02d) | ✓ (03c) | ✓ (04d) | ✓ (embedded) |
| Dependency chain | — | after 115 | after 116 | after 117 |

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