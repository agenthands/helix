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
- [ ] **v2.1 Agent Adoption & Aider-Derived Validation** -- Phases 97-102 (planned)

## Phases

### 🚧 v2.1 Agent Adoption & Aider-Derived Validation (Phases 97-102) — IN PROGRESS

**Milestone Goal:** Make AI coding agents reliably reach for `helix` verbs over standard tools (comprehensive generated reference + stronger steering + multi-agent coverage + a measured adoption contract), then prove the toolset works end-to-end with vendored Aider benchmarks and committed local baselines.

Two independent, interleavable thrusts; intra-thrust order is fixed by hard dependencies. **Thrust 1 (Adoption):** reference substrate → deterministic merge-gating contract → multi-agent + steering → opt-in LLM scorecard. **Thrust 2 (Aider validation):** vendor fixtures → EDIT-verb wiring + committed baseline → eval surfaces (RepoMap + fuzzy) with gold/drift corpora before their evaluators. The milestone is additive — ZERO new Go dependencies; the only structural code change is `internal/cli/skill.go` switching from an embedded `string` to an `embed.FS`.

**Cross-cutting exit gates baked into every relevant phase (anchored to named v1.12 failures, not new work):** (a) anti-vacuity — every gate ships a deliberate break-the-invariant → assert-RED test; (b) `HELIX_BIN` fail-not-skip + a hermetic golden sibling as the sole authoritative proof on every bench-surface phase; (c) `vet-ablation-leakage` leaf-import boundary respected by new bench/evaluator leaves; (d) benches stay local-only — no CI benchstat gate.

- [x] **Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract** - `embed.FS` skill bundle, `cmd/helix-refgen` + generated `reference.md`, merge-gating completeness + nudge-fires contract (completed 2026-06-22)
- [x] **Phase 98: Multi-Agent Coverage + Stronger Steering** - per-agent instruction files (Codex/Gemini/generic), Codex hook reuse, broadened nudge + SessionStart priming, negative-control coverage (completed 2026-06-23)
- [x] **Phase 99: Vendored Aider Fixtures + Mixed-License Gate** - MIT polyglot + Apache-2.0 edit-format fixtures, dual `verify-licenses` hard-fail + tamper test (completed 2026-06-23)
- [x] **Phase 100: Polyglot Edit Benchmark + Committed Baseline** - EDIT-verb `AgentFn` via reused `RunExercise`, `aider_edit` mode, `edit_format_applied` open key, byte-reproducible baseline (completed 2026-06-23)
- [x] **Phase 101: Opt-In LLM-Behavioral Adoption Scorecard** - choice/fallback-rate scorecard with sabotaged-skill revert-and-fail + negative judge exemplar, build-tag gated, never blocks merge (completed 2026-06-23)
- [x] **Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines** - gold/drift corpora authored from ground truth, `repomapeval` + `fuzzyrobust` leaves, reversed-ranker discriminators, committed baselines (completed 2026-06-23)

## Phase Details

### Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract

**Goal**: An agent has a complete, registry-generated per-verb reference installed alongside the terse skill, and a deterministic merge-gating contract proves both that the reference covers every frozen verb and that the nudge steers each standard-tool shape to the specific correct `helix` verb.
**Depends on**: Nothing new (reuses `cmd/docgen` plumbing, `skill.ToolProviders()`, `help.ExtractParamDocs`, `verbs_gen.go`)
**Requirements**: REF-01, REF-02, REF-03, ADOPT-01
**Success Criteria** (what must be TRUE):

  1. `helix-refgen` generates `internal/cli/skills/helix/reference.md` from the live tool registry covering every verb in `verbs_gen.go` (synopsis, args, output shape, worked example, "use this not that"); `helix-refgen --check` fails the build (make + CI) on a hand-edited or stale reference, inheriting the docgen blank-import-parity-with-daemon rule.
  2. `helix setup claude-code` installs `reference.md` atomically alongside `SKILL.md` from the `embed.FS` bundle, preserving the existing `withinSkillRoot` path-containment guarantee, with the SKILL-04 idle-cost bound still asserted on `SKILL.md` only.
  3. The deterministic adoption-contract test asserts `reference ⊇ VerbToolNames()` sourced from `verbs_gen.go` (the authority, NOT the generator's own output) AND a per-shape nudge-fires golden table mapping each standard-tool shape to the specific suggested verb, keyed on the emitted command, rejecting empty-bucket-as-pass — and it BLOCKS merge.
  4. Anti-vacuity proven: deleting one verb from `reference.md` turns the completeness gate RED, and a revert that breaks a nudge-shape mapping turns the contract RED (a deliberate break-the-invariant test ships in this phase).

**Plans**: 2 plans

Plans:
**Wave 1**

- [x] 97-01-PLAN.md — Generator + embed substrate: `VerbSpecsForDocs()` accessor + `cmd/helix-refgen` (registry walk, args from verbSpecs not InputSchema) + committed `reference.md` + `skill.go` string→`embed.FS` multi-file atomic install + `make verify-reference`/CI drift gate (REF-01/02/03)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 97-02-PLAN.md — Deterministic adoption contract (TDD): ADOPT-01a completeness (authority = `VerbToolNames()`) + revert-and-fail, ADOPT-01b per-shape nudge golden keyed on the emitted verb + revert-and-fail + empty-bucket floor; runs untagged, BLOCKS merge (ADOPT-01)

**UI hint**: no

### Phase 98: Multi-Agent Coverage + Stronger Steering

**Goal**: Non-Claude agents (Codex, Gemini, generic) receive the shared generated reference plus a per-agent instruction file installed without clobbering user content, Codex gets the reused advisory nudge hook, and the broadened steering classifier reaches more standard-tool shapes while provably never firing on legitimately-correct prose/log/config use.
**Depends on**: Phase 97 (shared reference is the substrate; the nudge envelope is portable to Codex)
**Requirements**: STEER-01, STEER-02, STEER-03, AGENT-01, AGENT-02, AGENT-03
**Success Criteria** (what must be TRUE):

  1. `helix setup` writes/updates per-agent instruction files via idempotent sentinel-delimited append (Codex `AGENTS.md` ≤32 KiB at the documented path, Gemini `GEMINI.md`, generic) that preserves pre-existing user content and does not duplicate the Helix block on re-run (golden round-trip + idempotency + cap tests).
  2. The generated verb reference is installable for non-Claude agents as a shared markdown reference (no per-agent bespoke skill engine); Codex's `PreToolUse` hook is wired to `helix nudge` reusing the existing advisory envelope, while Gemini/IDE/generic get instruction-file steering only and produce no fabricated hook artifact.
  3. The `PreToolUse` nudge's code-target classifier is broadened to steer more standard-tool invocations (grep/sed/cat/find/Read-shaped Bash) toward the specific equivalent `helix` verb, preserving the advisory exit-0 / fail-open contract (asserted exit-0 on every shape).
  4. A SessionStart priming surface presents the terse "use X not Y" decision matrix once per session, size-capped (SKILL-04-style idle-cost bound) and fail-open; negative-control golden classifier rows prove the nudge does NOT fire on prose/log/config/build-output targets (e.g. `grep TODO README.md`).

**Plans**: 2 plans

Plans:
**Wave 1** *(two file-disjoint thrusts run in parallel)*

- [x] 98-01-PLAN.md — STEER thrust (TDD): broaden `isGrepReadTool` token-anchored on `fields[0]` to steer Bash sed/cat (STEER-01); flip the DEFER-97-01 golden firing + remove the silent loop + bump floor 5→7 + revert-and-fail; STEER-03 prose/log/config negative controls (exit-0); SessionStart priming matrix constant, size-capped, fail-open (STEER-02)
- [x] 98-02-PLAN.md — AGENT thrust (TDD + 1 blocking human-verify): `EmbeddedReference()` shared reference (AGENT-01); idempotent sentinel-delimited `writeAgentInstructions` (no-clobber / no-duplicate / Codex AGENTS.md ≤32 KiB) (AGENT-02); `CodexRegistrar` (AGENTS.md + hooks.json→`helix nudge`, one engine) + flip gemini-cli/generic instruction-file-only + no-Gemini-hook proof + codex ValidArg (AGENT-03)

**UI hint**: no

### Phase 99: Vendored Aider Fixtures + Mixed-License Gate

**Goal**: A deterministic, offline, mixed-license vendored fixture tree (MIT Exercism polyglot subset + Apache-2.0 aider edit-format fixtures) lands in the tree with correct per-file SPDX headers, per-track attribution/NOTICE, and a manifest, guarded by an extended hard-fail license gate that goes RED on any tampered or missing header.
**Depends on**: Nothing new (reuses `pin.go`; must precede any committed baseline)
**Requirements**: VENDOR-01, VENDOR-02, VENDOR-03
**Success Criteria** (what must be TRUE):

  1. The MIT-licensed Exercism polyglot fixtures (a recorded, deterministic subset) are vendored under `bench/datasets/aider-polyglot/fixtures/` with `SPDX-License-Identifier: MIT`, per-track NOTICE/attribution, and a `VENDOR-MANIFEST.md` recording the exact exercise selection.
  2. Aider's Apache-2.0 edit-format fixtures are vendored with `SPDX-License-Identifier: Apache-2.0` + attribution, producing a documented mixed-license vendored tree (the user ratified the mixed-license decision).
  3. `make verify-licenses` is extended to hard-fail over the full vendored tree under both the MIT and Apache-2.0 dispositions.
  4. Anti-vacuity proven: a tamper test flipping a license header or removing a NOTICE turns the `verify-licenses` gate RED (deliberate break-the-invariant test ships in this phase).

**Plans**: 2 plans

- [x] 99-01-PLAN.md — Vendor the mixed-license tree: MIT polyglot 9-exercise subset (×py/go/rust) + Apache-2.0 aider edit-format subset + NOTICE/LICENSE sidecars + VENDOR-MANIFEST.md (real sha256) + dual-disposition LICENSE-AUDIT.md (VENDOR-01, VENDOR-02)
- [x] 99-02-PLAN.md — Extend the verify-licenses gate to dual disposition + bidirectional manifest-vs-disk sha256 walk + anti-vacuity tamper test + Makefile wiring (VENDOR-03)

**UI hint**: no

### Phase 100: Polyglot Edit Benchmark + Committed Baseline

**Goal**: The model's edit is routed through helix EDIT verbs against the warm daemon via the reused verb-agnostic `RunExercise` loader, surfaced as a new filesystem-table bench mode with an additive `edit_format_applied` result key, and a byte-reproducible committed polyglot-edit baseline is captured `HELIX_BIN`-gated, fail-not-skip.
**Depends on**: Phase 99 (vendored fixtures must exist before the bench can run offline)
**Requirements**: EDITBENCH-01, EDITBENCH-02, EDITBENCH-03, BASELINE-01
**Success Criteria** (what must be TRUE):

  1. An EDIT-verb `AgentFn` in `bench/runtime` (daemon-dialing) routes edits through `replace-symbol-body` / `fuzzy-edit` / `replace-in-file` / `insert-before-symbol` / `insert-after-symbol`, plugged into the existing `RunExercise` seam with the loader untouched and the WR-01 anti-tamper pristine-test restore preserved.
  2. A new `bench/runners/aider_edit/MODE.md` adds the polyglot-edit mode via the filesystem-as-table pattern with zero mode-resolver Go change, and an additive `edit_format_applied` (`*bool`, `omitempty`) open key is recorded on `result.v2.json` with no `schema_version` v3 bump.
  3. A committed polyglot-edit baseline (`bench/reports/<run>/BENCH-RESULTS.md` + `result.v2.json`) is captured `HELIX_BIN`-gated and fails (not silently SKIPs) when `HELIX_BIN` is set but no `result.v2.json` / empty bucket / missing metric line is produced; the baseline carries byte-reproducible deterministic metrics only (latency excluded → local `bench-micro`).
  4. Anti-vacuity proven: a hermetic golden sibling (no binary, no network) is the sole authoritative proof exercised by `go test ./bench/...`, a "did it RUN" sentinel proves the live leg ran when `HELIX_BIN` is set, and the new `bench/runtime` AgentFn respects the `vet-ablation-leakage` leaf-import boundary (kept outside the stdlib leaf).

**Plans**: 2 plans

Plans:
**Wave 1**

- [x] 100-01-PLAN.md — EDITBENCH wiring substrate (TDD): thin exported loader accessors (`LoadExercise`/`NativeTestCommand`, leaf boundary preserved) + deterministic daemon-dialing EDIT-verb `AgentFn` + live `TestFn` in `bench/runtime` + `edit_format_applied *bool` additive open key (mirror `swebench_raw_resolved`, no schema v3) + `bench/runners/aider_edit/MODE.md` (zero resolver change); hermetic proof only (EDITBENCH-01/02/03)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 100-02-PLAN.md — Live cell + committed baseline (TDD): `runAiderEditCell` mode-name branch (sibling of `runRAGCell`) driving `RunExercise` verbatim against the warm daemon (WR-01 preserved) + committed byte-reproducible baseline at `bench/reports/aider-edit-baseline/` (deterministic metrics only, `.gitignore` allowlist) + double-render byte-reproducibility golden + HELIX_BIN fail-not-skip "did it RUN" sentinel (EDITBENCH-01, BASELINE-01)

**UI hint**: no

### Phase 101: Opt-In LLM-Behavioral Adoption Scorecard

**Goal**: An opt-in, build-tag-gated LLM-behavioral scorecard measures an agent's helix-choice rate and standard-tool fallback rate using the reused v1.4 `llm`/`llmjudge` harness, with built-in failing anchors (sabotaged-skill revert-and-fail + negative judge exemplar) so the score can demonstrably fail — and it never blocks merge.
**Depends on**: Phase 97 (loads the skill body) and Phase 98 (deterministic contract + steering green first)
**Requirements**: ADOPT-02
**Success Criteria** (what must be TRUE):

  1. The scorecard (reusing `test/oracle/llm`, build-tag gated) reports both choice rate and fallback rate, keyed on the first emitted command line (not substring presence), asserts the two are complementary on a known fixture, and rejects empty/one-element task buckets as a pass.
  2. The judge rubric includes an explicit negative exemplar (a response that runs `grep -r` to find a definition scores 0 on adoption) so the rubric can return a failing score; the layer never blocks merge.
  3. Anti-vacuity proven: a sabotaged-skill revert-and-fail self-test runs the scorer against a skill body with the decision matrix stripped and asserts `choice_rate` drops materially — if the score is identical with and without the skill, the test fails (deliberate break-the-invariant test ships in this phase).

**Plans**: 2 plans

Plans:
**Wave 1**

- [x] 101-01-PLAN.md — Hermetic pure scorer (TDD): build-tag-FREE `test/oracle/adopt` package — lifted `FirstCommand`/`ClassifyChoice` (first-command prefix, not substring) + `Scorecard` (choice_rate/fallback_rate, empty-bucket floor) + `StripDecisionMatrix` (section-strip, len-guarded no-noop) + 12 committed fixtures + the 5 anti-vacuity tests (revert-and-fail material drop, complementary, empty-bucket, sabotage-non-noop, first-command-not-substring); runs in `go test ./...` with no key (ADOPT-02)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 101-02-PLAN.md — Tag-gated adapters (TDD): judge `adoption` dimension on `Score`/`ValidateScoreValues`/`ComputeVerdict`/`RubricPrompt` + grep-scores-0 negative exemplar + verdict proof (`//go:build llmjudge`) + aggregate dims; `//go:build llm` live capture leg over the SAME `adopt.Scorecard` (SkipWithoutAPIKey, informational); neither runs in `go test ./...`, never blocks merge (ADOPT-02)

**UI hint**: no

### Phase 102: RepoMap-Quality + Fuzzy-Robustness Evals + Baselines

**Goal**: Two stdlib-only leaf evaluators measure Helix's existing `internal/repomap` ranking quality and `internal/fuzzy` strategy-selection/ambiguity-refusal against broad multi-language gold/drift corpora authored from task ground truth (not tool output), each guarded by a reversed/random-ranker discriminator that must fail the corpus, with committed byte-reproducible baselines.
**Depends on**: Phase 99 (corpus can reuse vendored fixture ground truth); corpus authored before each evaluator (avoid self-confirming gold)
**Requirements**: REPOEVAL-01, REPOEVAL-02, FUZZBENCH-01, FUZZBENCH-02, BASELINE-02
**Success Criteria** (what must be TRUE):

  1. `bench/evaluators/repomapeval` (stdlib-only, respecting the `vet-ablation-leakage` no-kernel-import boundary) measures `get-repo-map` / `get-context` ranking quality (recall@k / MRR / nDCG) and token-budget fit against a broad multi-language gold corpus authored from task ground truth (e.g. exercism `files.solution`), independent of `get-repo-map` output, with a documented per-language size floor.
  2. `bench/evaluators/fuzzyrobust` (stdlib-only + `editsim.ES`, respecting the leaf-import boundary) measures `internal/fuzzy` 4-strategy selection and ambiguity refusal against a broad native drift corpus whose expected strategy is derived from the drift type (not observed behavior), including at least one known-ambiguous case that MUST be refused, with a documented size floor.
  3. Committed RepoMap-eval and fuzzy-robustness baseline artifacts are captured `HELIX_BIN`-gated, fail-not-skip, byte-reproducible, deterministic-metrics-only (routed through the existing deterministic `renderAll`, seeded resamples, sort-before-emit).
  4. Anti-vacuity proven: a reversed/random ranker MUST fail the RepoMap gold corpus and the known-ambiguous case MUST be refused by `fuzzyrobust`; each evaluator ships a hermetic golden sibling that runs with no binary and no network (deliberate break-the-invariant discriminators ship in this phase).

**Plans**: 3 plans

Plans:
**Wave 1**

- [x] 102-01-PLAN.md — repomapeval leaf: gold corpus from ground truth + recall@10/MRR/nDCG@10 + budget-fit + reversed/random discriminator + leaf self-test + capture regenerator (REPOEVAL-01, REPOEVAL-02)
- [x] 102-02-PLAN.md — fuzzyrobust leaf: per-tier perturbation + drift corpus + strategy/refusal scoring via editsim.ES + duplicate-block must-refuse + leaf self-test + capture harness (FUZZBENCH-01, FUZZBENCH-02)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 102-03-PLAN.md — committed baselines: aggregator renderers + double-render byte-reproducibility + stripped-metric anti-vacuity + .gitignore allowlist + Makefile regen targets (BASELINE-02)

**UI hint**: no

## Progress (v2.1)

**Execution Order:** Two interleavable thrusts; intra-thrust order fixed. Thrust 1: 97 → 98 → 101. Thrust 2: 99 → 100 → 102. (98 depends on 97; 101 depends on 97+98; 100 depends on 99; 102 depends on 99.)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 97. Generated Reference + Adoption Contract | v2.1 | 2/2 | Complete    | 2026-06-22 |
| 98. Multi-Agent Coverage + Stronger Steering | v2.1 | 2/2 | Complete    | 2026-06-22 |
| 99. Vendored Aider Fixtures + License Gate | v2.1 | 2/2 | Complete    | 2026-06-23 |
| 100. Polyglot Edit Benchmark + Baseline | v2.1 | 2/2 | Complete    | 2026-06-23 |
| 101. LLM-Behavioral Adoption Scorecard | v2.1 | 2/2 | Complete    | 2026-06-23 |
| 102. RepoMap + Fuzzy Evals + Baselines | v2.1 | 3/3 | Complete    | 2026-06-23 |

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

### Phase 75: Schema, Fairness Contract & Tree Skeleton

**Goal**: Every downstream phase has a versioned `result.v2.json` schema to write into and a single `fairness_contract.go` struct to load model config from — so no benchmark adapter ever defines its own model snapshot, temperature, or cost row.

**Depends on**: v1.10 Phase 67 (`internal/eval/` patterns — sandbox, trace tap, score DSL)

**Requirements**: BENCH-01, BENCH-02, BENCH-03, BENCH-06, FAIR-01, FAIR-02, FAIR-03, COST-01, INFRA-01, INFRA-02, INFRA-03

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 75)

### Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags

**Goal**: Land the only invasive code paths inside the daemon (kernel-level `disable_lsp_subsystem` / `disable_structured_edit_subsystem` flags) plus the 4 new bench profile YAMLs, with a static `vet-ablation-leakage` analyzer so they are stable before any downstream phase consumes them. `no_semantic` flag is intentionally deferred to Phase 81 because it depends on the v1.10 Phase 65 SemanticLookup wiring being un-wired cleanly.

**Depends on**: Phase 75, v1.10 Phase 65 (SemanticLookup seam — read for context only)

**Requirements**: ABLATE-02, ABLATE-05, ABLATE-07, ABLATE-08
**Plans:** 3/3 plans complete
**Wave 1**

- [x] 76-01-PLAN.md — Wave 1: kernel disable flags + accessors + structured-edit Unsupported guard + replace_in_file exact-match-only (TDD; ABLATE-07)
- [x] 76-02-PLAN.md — Wave 1: 4 bench profile YAMLs + Profile disable-flag fields + golden tool-surface tests + loader unknown-mode rejection (TDD; ABLATE-02)
- [x] 76-03-PLAN.md — Wave 1: vet-ablation-leakage analyzer + cmd + testdata green→red + make vet wiring (TDD; ABLATE-08)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 76-04-PLAN.md — Wave 2: no_lsp null-object daemon wiring + CLI override flags + config fields + zero-span trace-tap (TDD; ABLATE-05)

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 76)

### Phase 77: Bench Runtime & First E2E Smoke

**Goal**: Stand up the bench orchestrator end-to-end on a single Go ToolBench task in a single mode — no container, no per-language sprawl — so the runtime shape is forced into existence and proven before evaluators or ablations land on top.

**Depends on**: Phase 75 (schema, tree), Phase 76 (bench profile YAMLs exist so daemon subprocess can be started with `--profile=bench-full`)

**Requirements**: BENCH-04, BENCH-05

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 77)

**Plans:** 5/5 plans complete
Plans:
**Wave 1**

- [x] 77-01-PLAN.md — Foundation primitives: bench sandbox (embed eval), subprocess daemon lifecycle, mode->profile resolver + your_agent_full/MODE.md, and the one seed toolbench-go task

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 77-02-PLAN.md — result.v2.json builder (schema-valid, metric-sparse) + CCTapResult synthesis from scripted StepResults (TDD)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 77-03-PLAN.md — Cell orchestrator spine: forwarder drive -> kill -> PID-gated tap -> 2-leg Merge -> result write; BENCH-04 + criterion-#4 integration tests

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 77-04-PLAN.md — helix-bench run subcommand + flags + matrix expander (--parallel bounded) + wired-not-gating claude branch

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 77-05-PLAN.md — Makefile reconciliation (bench collision -> bench-micro) + bench/bench-quick/bench-<suite> targets + timed E2E smoke gate + BENCH.md key-names

### Phase 78: Internal ToolBench — Go First + LanguageRunner Interface

**Goal**: The deterministic ground truth for "Helix tools work" — 10 capability test classes, all 10 covered on Go (Helix's own language, tightest debug loop, no container), and a common `LanguageRunner` interface ready for the remaining 7 languages.

**Depends on**: Phase 77 (bench runtime is operational)

**Requirements**: TOOLBENCH-01, TOOLBENCH-02, TOOLBENCH-10

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 78)

**Plans:** 5 plans (4 waves)
Plans:
**Wave 1**

- [x] 78-01-PLAN.md — LanguageRunner interface + GoRunner (go test -json) + WithWorkingDir daemon option (the Phase 85 seam, D-03/D-10/D-11)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 78-02-PLAN.md — Matrix language axis + seed git mv + toolbench-go→internal-toolbench cutover + runner dispatch/store opt-in wiring (D-07/D-08/D-09/D-10)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 78-03-PLAN.md — 8 store-off Go capability fixtures (semantic view, diagnostics, rename, fuzzy, call graph, dependency graph, context min, failure handling) (D-04/D-05/D-06)
- [x] 78-04-PLAN.md — Store-ON incremental_update fixture (real overlay-drain refresh) + --parallel store-isolation integration test (D-01/D-02/D-03)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 78-05-PLAN.md — CAPABILITIES.md + PHASE67_CROSSWALK.md + corpus-coverage aggregator (Go 10/10) + full-run human-verify checkpoint (D-06/D-11, C1/C2/C4)

### Phase 79: Evaluators & Result-Schema Metrics Layer

**Goal**: Every per-task `result.v2.json` is populated with all 17 normalized metrics from real graders — including the load-bearing `tokens_input/output` from the provider's `usage` block (not Helix's MCP counter), `edit_locality_given_solved` as the headline locality metric, and a single merged OTel trace per `(task, mode, run_index)`.
**Depends on**: Phase 77 (bench runtime), Phase 78 (Go ToolBench produces real test results to grade)
**Requirements**: METRIC-01, METRIC-02, METRIC-03, METRIC-04, METRIC-05, METRIC-06
**Success Criteria** (what must be TRUE):

  1. `bench/evaluators/{test_runner,patch_validator,token_meter,tool_trace_analyzer,regression_checker}/` produce all 12 base metrics + 5 extended metrics (`semantic_tool_calls`, `edit_distance_patch`, `retry_count`, `compile_errors_before`, `compile_errors_after`) on a real task; missing metrics are explicit nulls, not omissions.
  2. A regression test asserts `tokens_input/output` source-of-truth is the provider response's `usage` block, NOT Helix's MCP-side counter; cached-input tokens (`tokens_input_cached_read`, `tokens_input_cache_write`) are reported as separate columns.
  3. `edit_locality` definition (`1 − (modified_files / total_files_in_repo_subtree)`) and `regression_rate` definition (`(failing_pre-existing_tests_post_patch / passing_pre-existing_tests_pre_patch)`) are unit-tested at edge cases (root-only = 1.0, all-files ≈ 0.0, synthetic regression case); both definitions documented in `bench/evaluators/METRICS.md`.
  4. Trace merging produces a single merged trace per `(task, mode, run_index)` from Helix daemon OTel + agent CLI subprocess + bench harness span; Jaeger import shows full continuity from `bench.run_id` root to LSP leaves; no orphan spans, no cross-cell PID leakage.

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 79)

**Plans:** 4/4 plans complete
Plans:
**Wave 1**

- [x] 79-01-PLAN.md — Foundation: typed nullable `Metrics`/`MetricError` + schema typing of all 17 metrics + `metric_errors[]` (additive minor bump) [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 79-02-PLAN.md — Exec/git graders: test_runner (exit-authoritative success), patch_validator (edit_locality/edit_distance), regression_checker (pre/post regression_rate) [Wave 2]
- [x] 79-03-PLAN.md — Trace graders: token_meter (provider-usage source-of-truth, scripted-null) + tool_trace_analyzer (trace-derived metrics, no re-merge) [Wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 79-04-PLAN.md — Coordinator (D-07 isolation) + result.v2 wiring + run_index path + pre-patch snapshot + METRICS.md + E2E gate [Wave 3]

### Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement

**Goal**: All 6 ablation modes are operational end-to-end on the Go ToolBench corpus, holding the same-model-same-budget invariant via the Phase 75 fairness contract. `your_agent_full` + `baseline_plain` + `no_lsp` + `no_structured_edit` produce real per-mode `result.v2.json` rows; `no_semantic` scaffolding is in place but the kernel flag (ABLATE-06) lands in Phase 81; `baseline_rag` runner stub exists but its real implementation lands in Phase 83.
**Depends on**: Phase 76 (bench profile YAMLs + no_lsp / no_structured_edit kernel flags), Phase 79 (evaluators produce real metrics), Phase 78 (Go ToolBench is the corpus)
**Requirements**: ABLATE-01, ABLATE-03
**Success Criteria** (what must be TRUE):

  1. Each of 6 modes runs end-to-end on a smoke task; mode definition lives in `bench/runners/<mode>/MODE.md`; ablation runs produce schema-valid `result.v2.json` rows tagged with the mode.
  2. `baseline_plain` reuses existing `internal/profile/profiles/baseline.yaml` (no new YAML); tool inventory for `baseline_plain` is empty except for the shell/grep/read/edit/test that the agent runtime exposes natively (documented decision in `bench/BENCH.md`).
  3. Same-model-same-budget invariant is enforced at runner-startup: per-task token budget is identical across modes; CI contract test asserts every runner's effective `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)` equals the fairness contract.
  4. Ablation deltas (`your_agent_full` − `baseline_plain`, `full` − `no_lsp`, `full` − `no_structured_edit`) compute correctly on the Go ToolBench corpus and surface in the per-mode result rows.

**Plans:** 5/5 plans complete
Plans:
**Wave 1**

- [x] 80-01-PLAN.md — 5 new bench/runners/<mode>/MODE.md definitions + resolver test + BENCH.md docs (ABLATE-01/03; D-01/02/03)
- [x] 80-02-PLAN.md — additive ablation_status field on the result.v2 builder + schema doc (TDD; D-03)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 80-03-PLAN.md — RunCell fairness Validate() startup gate + baseline_rag fail-close (no row) + ablation_status wiring (TDD; D-01/02/03/04)
- [x] 80-04-PLAN.md — unconditional CI effective-config contract test (TDD; D-04)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 80-05-PLAN.md — minimal 3-delta pass surfaced in per-mode rows + five-of-six multi-mode scripted smoke (TDD; D-05)

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 80)

### Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test

**Goal**: Close the only ablation mode that cannot be expressed by profile YAML alone — `disable_semantic_subsystem` is a kernel-level config gate that un-wires v1.10 Phase 65's `SetSemanticLookup` strangler-fig integration at daemon bootstrap, so `get_repo_map`/`get_context` fall back to the v1.9 tree-sitter path (envelope `source == "tree_sitter"`) and zero DuckDB reads happen during a `no_semantic` run.
**Depends on**: Phase 80 (5/6 ablation runners working), v1.10 Phase 65 (SemanticLookup wiring — the thing being disabled)
**Requirements**: ABLATE-06
**Success Criteria** (what must be TRUE):

  1. With `semantic_index.bench_disabled: true` (or equivalent kernel flag), daemon bootstrap skips `SetSemanticLookup`; `get_repo_map` and `get_context` return v1.9 tree-sitter path with `source == "tree_sitter"`; `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, `analyze_blast_radius` SemanticLookup paths all see `NoopLookup`.
  2. An E2E `no_semantic` smoke task makes zero queries against the DuckDB semantic store; runtime assertion (`helix_semantic_*` counter family == 0) logs and fails the bench cell if violated.
  3. A vet-style boundary guard in `internal/lint/` flags any code path that conditionally bypasses the `bench_disabled` gate (extension of the `vet-ablation-leakage` analyzer from Phase 76).
  4. `bench/runners/your_agent_no_semantic/MODE.md` documents the config-key gate + the strangler-fig consumer enumeration so a future contributor cannot accidentally rip out the bypass.

**Plans**: 7 plans (5 waves) — 5 original + 2 gap-closure (81-06, 81-07) added after 81-VERIFICATION found criterion #2 vacuous (WR-02) and criterion #1 partial (CR-01)

**Wave 1**

- [x] 81-01-PLAN.md — net-new helix_semantic_store_reads_total counter + DuckDB read-chokepoint instrumentation (D-05 verification target)
- [x] 81-02-PLAN.md — config surface: distinct bench_disabled field + profile field + CLI override + bench-no-semantic.yaml gate + assertion flip (D-01/D-03)
- [x] 81-03-PLAN.md — vet-ablation-leakage call-site gate check + green→red testdata (D-06, criterion #3)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 81-04-PLAN.md — effSemanticDisabled resolution at composition root + Noop/disabled-gate forcing on all 8 consumers (+a 4th guardrail hand-out), build-but-block (D-02/D-04, criterion #1)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 81-05-PLAN.md — E2E counter==0 bench-cell assertion + fail-cell + MODE.md rewrite (D-05, criteria #2 & #4)

**Wave 4** *(gap closure — 81-VERIFICATION.md GAP 1 / WR-02: criterion #2 runtime assertion was vacuous end-to-end)*

- [x] 81-06-PLAN.md — graceful DaemonHandle.Stop (SIGTERM) so d.shutdown() flushes the reads-total line on a real bench run + fail-CLOSED scrape/assert (absent line hard-fails the no_semantic arm) + real-daemon emission integration test (ABLATE-06, D-05)

**Wave 5** *(gap closure — 81-VERIFICATION.md GAP 2 / CR-01: criterion #1 background pipelines ungated; blocked on Wave 4's fail-closed teeth)*

- [x] 81-07-PLAN.md — gate SetActivateCallback background read pipelines + SetFileFactStore on effSemanticDisabled (build-but-block, store stays built per D-04) + store-ON no_semantic end-to-end regression proof (ABLATE-06)

### Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard

**Goal**: First externally-publishable artifact — `bench/aggregator/` consumes N ≥ 3 runs per (task, mode), computes BCa bootstrap CIs (≥ 10,000 resamples) and `pass@k` via the HumanEval closed-form, rolls up `cost_per_solved_task` against the Phase 75 cost table, and emits the first `leaderboard.md` from internal-ToolBench-Go data only.
**Depends on**: Phase 80 (real ablation data for `your_agent_full` + baselines), Phase 79 (metrics layer feeds the aggregator)
**Requirements**: STATS-01, STATS-02, STATS-03, STATS-04, COST-02, COST-03
**Success Criteria** (what must be TRUE):

  1. Default `N ≥ 3` runs per (task, mode) is enforced by the matrix runner; the aggregator refuses to write `reports/` if any cell has fewer than the configured minimum; schema validates `runs` array length ≥ N.
  2. BCa (bias-corrected accelerated) bootstrap CIs with `N_resamples ≥ 10,000` compute for every metric on every leaderboard row; unit tests pass against a closed-form known distribution; reports flag any cell where the BCa CI overlaps a neighbor (no claim of "X > Y" without non-overlapping CIs).
  3. `pass@1` and `pass@k` computed per HumanEval closed-form `1 − C(n-c, k)/C(n, k)`; unit test against published reference values passes.
  4. `cost_per_solved_task` = (sum across solved tasks of provider-`usage`-derived USD cost) / count(solved); matches a hand-computed example for a known run; `cost_quality.md` renders cost-per-solved-task per mode × benchmark with BCa CIs.

**Plans**: 7 plans (4 waves) — 1/7 complete

**Wave 1**

- [x] 82-01-PLAN.md — Foundation: move cost-table types to importable bench/cost (Open Q1) + ExpandMatrix N-cell axis + --runs flag (D-04, STATS-01 producer) ✅ (commits dccf8352, abcae78c, 88f62367)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 82-02-PLAN.md — BCa bootstrap CI (z0 + jackknife a, seeded, >=10k resamples) (TDD; STATS-02)
- [x] 82-03-PLAN.md — HumanEval unbiased pass@k (product form + lgamma cross-check; k>=2 anti-naive) (TDD; STATS-03)
- [x] 82-04-PLAN.md — cost-per-solved rollup (USD formula + freshness gate via bench/cost; golden 3.555) (TDD; COST-02)
- [x] 82-05-PLAN.md — aggregator loader + fail-closed N-gate (expectedN from flag, not disk) (TDD; STATS-01)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 82-06-PLAN.md — orchestrator (two-level reduce) + leaderboard.md/cost_quality.md render + STATS-04 overlap gate + FAIR-03 CV variance + determinism (TDD; STATS-02/03/04/COST-03)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 82-07-PLAN.md — helix-bench aggregate subcommand + registration + E2E human-verify checkpoint (D-02; STATS-01/COST-03)

### Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder

**Goal**: The RAG baseline is a competent grep + embedding-RAG control arm, not a strawman — implemented as a standalone `cmd/helix-bench-rag` MCP server with exactly 4 fixed tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`), backed by `chromem-go` and OpenAI `text-embedding-3-small` (with Ollama `nomic-embed-text` offline fallback). Self-contained; benefits from soaking before public benchmarks land.
**Depends on**: Phase 82 (aggregator reports `baseline_rag` rows in the leaderboard), Phase 80 (scaffolding for `baseline_rag` runner exists)
**Requirements**: ABLATE-04
**Success Criteria** (what must be TRUE):

  1. `cmd/helix-bench-rag --help` works; tool-list returns exactly 4 tools; a vet test asserts no import from `internal/kernel/` or `internal/semantic/` (`baseline_rag` is provably NOT a Helix profile and shares no code with the daemon's tool surface).
  2. Per-corpus embedding index is built once per `(corpus, embedder_model)` and cached at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`; `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` documents the model pin (OpenAI `text-embedding-3-small` primary, Ollama `nomic-embed-text` fallback) and chunking strategy.
  3. A `baseline_rag` ToolBench run on Go produces schema-valid `result.v2.json` rows; the embedder ID is recorded in every row so reviewer pushback on "weak embedder" can be addressed factually.
  4. The same-model-same-budget invariant holds: `baseline_rag` agent uses the identical fairness-contract model snapshot and budget as `your_agent_full`; embedding-API calls are NOT charged against the agent's per-task budget (documented in `BENCH.md`).

**Plans**: 3 plans
Plans:
**Wave 1**

- [x] 83-01-PLAN.md — bench/ragindex leaf package (chromem-go index build/cache, deterministic corpus_sha, embedder selection) + EMBED-CHOICE.md

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 83-02-PLAN.md — standalone cmd/helix-bench-rag MCP server (exactly 4 tools, --help) + no-kernel/no-semantic import-boundary vet gate

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 83-03-PLAN.md — bench wiring: embedder_id result key, real baseline_rag drive leg (out-of-band index, same contract+budget), flip deferral tests, BENCH.md/MODE.md rewrite

### Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard

**Goal**: The container infra needed only for public benchmarks — `os/exec` to `docker` (with podman drop-in), pinned-SHA256 per-instance images, cosign-signed mirror under `ghcr.io/agenthands/helix-bench-*` (reusing v1.10 Phase 58 cosign keyless infra), and a pre-flight disk-budget guard so a contributor's laptop doesn't melt before the first SWE-bench pull.
**Depends on**: Phase 75 (cost table, INFRA hygiene), Phase 82 (aggregator can ingest container-cell results), v1.10 Phase 58 (cosign keyless flow reused for the mirror)
**Requirements**: CONTAINER-01, CONTAINER-02, CONTAINER-03, CONTAINER-04
**Success Criteria** (what must be TRUE):

  1. `grep "github.com/docker/docker"` in `go.mod` returns empty; bench harness works with either `docker` or `podman` on PATH; arch-mismatch refusal (e.g., SWE-bench Verified amd64 image on arm64 host) refuses to run unless `BENCH_ARCH_MISMATCH_OK=1` is set.
  2. Per-instance images are pinned by SHA256 digest, not tag; image-cache state lives under `$HELIX_CACHE_DIR/bench-images/<sha>/`; a cache-hit test passes on re-run.
  3. A cosign-signed mirror of SWE-bench / Multi-SWE-bench / Terminal-Bench instance images is published to `ghcr.io/agenthands/helix-bench-*`; bench harness verifies the cosign signature before pulling; a tampered image is rejected.
  4. Disk-budget guard fails the run if available disk on the bench host is < 50 GB before a SWE-bench full run; the synthetic low-disk test trips the guard with a one-line remediation message.

**Plans**: 4 plans
Plans:
**Wave 1**

- [x] 84-01-PLAN.md — Package foundation + os/exec docker/podman engine shim + arch-mismatch gate + go.mod docker-SDK grep gate (CONTAINER-01)
- [x] 84-02-PLAN.md — SHA256 digest-pinned image cache at $HELIX_CACHE_DIR/bench-images/<sha>/ + cross-platform disk-budget guard (CONTAINER-02, CONTAINER-04)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 84-03-PLAN.md — In-process cosign/sigstore-go verify (canonical-error) + verify-then-pull crane wiring (CONTAINER-03 runtime half)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 84-04-PLAN.md — bench-mirror.yml CI publish+sign of the GHCR mirror + BENCH.md container docs (CONTAINER-03 CI/publish half)

### Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners

**Goal**: Cheapest external benchmark first — Aider Polyglot's 225 Exercism tasks across 6 languages requires no Docker and no upstream Python harness, just per-language test runners. Land the adapter AND simultaneously light up the remaining 7 ToolBench languages (Python, TypeScript, JavaScript, Java, C#, C++, Rust) since the per-language toolchain image work is the same dependency.
**Depends on**: Phase 78 (Go LanguageRunner is the template), Phase 79 (evaluators consume per-language test output), Phase 82 (aggregator handles per-language slicing)
**Requirements**: ADAPTER-AIDER-01, TOOLBENCH-03, TOOLBENCH-04, TOOLBENCH-05, TOOLBENCH-06, TOOLBENCH-07, TOOLBENCH-08, TOOLBENCH-09
**Success Criteria** (what must be TRUE):

  1. Aider Polyglot full run completes via `dataset-loader-only` adapter (shallow git clone of `Aider-AI/polyglot-benchmark` at pinned sha); 2-attempt protocol with stderr re-prompt; per-language pass-rate matches published sanity benchmarks for the pinned model.
  2. All 7 remaining languages have a `bench/languages/<L>/runner.go` implementing `LanguageRunner`: Python (`pytest --json-report`, ≥ 8/10 capabilities), TypeScript (`vitest --reporter=json`, ≥ 8/10 + tsserver diagnostics), JavaScript (`jest --json`, ≥ 8/10 + eslint diagnostics), Java (`mvn test`, ≥ 8/10 + jdtls semantic view), C# (`dotnet test --logger trx`, ≥ 6/10), C++ (`cmake/ctest`, ≥ 6/10 + clangd semantic view), Rust (`cargo test --message-format=json`, ≥ 8/10 + rust-analyzer semantic view).
  3. Pre-baked per-language toolchain images (offline-resolved deps via `mvn -o`, `cargo --offline`, `pnpm install --offline --frozen-lockfile`, `pip install --no-index --find-links=…`) ship; tests run with `--network=none`; non-hermetic tasks are flagged in the dataset.
  4. Per-track Exercism license audit lands in `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (per-track sha256 + redistribution clause excerpt); `make verify-licenses` is green.

**Plans**: TBD

### Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate

**Goal**: Mid-size completion-only public benchmarks via pure `dataset-loader-only` adapters — CrossCodeEval covers Python/Java/TS/C# (the only public coverage for C# we ship), RepoBench-R/-C/-P covers Python and Java. Multi-oracle gate (EM + edit-similarity + identifier match all required to pass, with abstain mode for low-confidence completions) is the v1.12-specific contribution.
**Depends on**: Phase 85 (Python + Java + TS + C# LanguageRunners), Phase 82 (aggregator), Phase 75 (HF dataset fetcher infra via `gomlx/go-huggingface` + arrow-go fallback)
**Requirements**: ADAPTER-CCE-01, ADAPTER-REPO-01, VERIFIED-03
**Success Criteria** (what must be TRUE):

  1. CrossCodeEval smoke run scores at least one task per language (Python, Java, TS, C#); EM + edit-similarity + identifier-match scorers are unit-tested against CCE paper examples; HF dataset fetched via the cached parquet pipeline.
  2. RepoBench smoke run for each sub-task (RepoBench-R retrieval `acc@k`, RepoBench-C completion `EM`/`ES`, RepoBench-P pipeline) covers Python and Java; metrics match published reference values on a sampled subset.
  3. Multi-oracle gate documented in `bench/evaluators/VERIFIED.md`: EM + edit-similarity + identifier match all required to pass; per-oracle threshold is configurable; abstain mode emits a `verified_correctness = false` row instead of a false-positive `true`.
  4. Both adapters use the canary-emission probe (per Phase 75 INFRA pattern) and flag potentially-contaminated tasks; canary pass-rate column populated in their leaderboard rows.

**Plans**: TBD

### Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness`

**Goal**: The headline external benchmark — SWE-bench Verified 500-task adapter via `subprocess-shellout` to upstream `python -m swebench.harness.run_evaluation`, with raw upstream score and UTBoost-augmented rescored score reported side-by-side. Multi-oracle `verified_correctness` (canonical tests pass AND augmented tests pass AND no pre-existing tests regress) ships in the same phase because they are intrinsically coupled — shipping the adapter without UTBoost regresses the milestone's `verified_correctness` claim.
**Depends on**: Phase 84 (container runtime + GHCR mirror), Phase 85 (Python LanguageRunner + pre-baked Python toolchain image), Phase 86 (multi-oracle pattern proven on completion benchmarks)
**Requirements**: ADAPTER-SWE-01, VERIFIED-01, VERIFIED-02
**Success Criteria** (what must be TRUE):

  1. SWE-bench Verified smoke run of 5 tasks completes via `subprocess-shellout` to upstream harness; `predictions.jsonl` produced by the agent; result JSON ingested into `result.v2.json` schema with container ID + exit code preserved.
  2. `verified_correctness` is computed independently of `task_success` — a known-buggy patch that passes only canonical tests gets `task_success=true` AND `verified_correctness=false`; the metric requires (a) canonical tests pass AND (b) UTBoost-augmented tests pass AND (c) no pre-existing tests regress.
  3. SWE-bench Verified report shows both raw upstream score and UTBoost-augmented rescored score side-by-side; UTBoost augmented suite is ingested from the published source and reproducible from a `--run-id`.
  4. Run-all-tests override is wired (not just PR-modified tests as upstream's default); `bench/evaluators/swebench/differential.go` consumes the gold patch alongside the agent patch and emits diff-overlap signal.

**Plans**: 4 plansPlans:
**Wave 1**

- [x] 87-01-PLAN.md — substrate: additive container_id/exit_code result.v2 keys + harness subprocess argv wrapper + UTBoost pin/fetch + A1-A5 human-verify

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 87-02-PLAN.md — predictions.jsonl producer + harness report parser + harness-JSON→result.v2 ingestion (hermetic, TDD)
- [x] 87-04-PLAN.md — aggregator raw-vs-rescored side-by-side column (additive, byte-stable goldens, TDD)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 87-03-PLAN.md — 3-condition verified_correctness gate (SC#2 load-bearing) + differential.go + rescore + VERIFIED.md (TDD)

### Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters

**Goal**: Final external coverage — Multi-SWE-bench (1,632 instances × Java/TS/JS/Go/Rust/C/C++; Mini set acceptable at ship, full set is the reach goal) and Terminal-Bench 2.0 (89 hard containerized long-horizon tasks driven through `tb run`). Both via `subprocess-shellout`. Per-language slicing is the v1.12 contribution; long-wall scheduler accommodates Terminal-Bench tasks whose wall-time exceeds a day.
**Depends on**: Phase 87 (subprocess-shellout pattern proven on SWE-bench), Phase 85 (7 per-language toolchain images + runners)
**Requirements**: ADAPTER-MULTI-01, ADAPTER-TERM-01
**Success Criteria** (what must be TRUE):

  1. Multi-SWE-bench Mini set runs end-to-end via `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`; per-language slicing (Java, TS, JS, Go, Rust, C, C++) is exposed in the reporter; full set documented as reach goal with the license resolution status (defer to v1.13 if unresolved per Phase 75 INFRA-02).
  2. Terminal-Bench 2.0 smoke run of ≥ 5 tasks completes via `tb run` CLI; container-isolation invariant holds (per-task fresh container, no cross-task filesystem leakage); `tb` JSON output ingested into `result.v2.json`.
  3. Long-wall scheduler accommodates tasks whose expected wall-time exceeds a day; per-cell checkpointing means a 24h Terminal-Bench task can resume after harness restart.
  4. Both adapters carry forward Phase 87's run-all-tests override pattern where applicable; per-language ablation slicing built into the reporter (not the upstream harness).

**Plans**: 4 plans (all wave 1, file-disjoint)

- [x] 88-01-PLAN.md — Multi-SWE-bench adapter (config.json producer + harness argv + resolved-gate ingestion + 7-language slicing through the existing aggregator)
- [x] 88-02-PLAN.md — Terminal-Bench 2.0 adapter (tb run argv with the runnerKind binary-name seam + results.json is_resolved ingestion + container-isolation gate)
- [x] 88-03-PLAN.md — Long-wall checkpoint/resume state machine (bench/longwall: atomic checkpoint + resume-skips-done + idempotent re-entry, injected clock, SC#3 with no 24h run)
- [x] 88-04-PLAN.md — Multi-SWE Mini-set fetcher (clone swebench-utboost) + bench/LICENSES.md rows (CC0 / Apache-2.0) + A1-A7 human-verify checkpoint

### Phase 89: Reports, CI Policy & Contamination Canary

**Goal**: The publication artifact — `leaderboard.md` + `per_language.md` + `ablations.md` + `cost_quality.md` byte-reproducible from a `--run-id`, with the CI cost-policy split that protects the milestone budget (`make bench-quick` ≤ 5 min on PR, full `make bench` nightly or maintainer-gated). Contamination canary closes the loop on Pitfall 1 — emit a known-novel pattern in select tasks; if a model emits it verbatim, the task is excluded from headline numbers with a footnote.
**Depends on**: Phase 82 (aggregator + first leaderboard), Phase 83 (baseline_rag rows), Phase 87 (SWE-bench Verified headline), Phase 88 (final external coverage)
**Requirements**: REPORT-01, REPORT-02, REPORT-03, REPORT-04, REPORT-05, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):

  1. `helix-bench report --run-id <id>` regenerates all 4 reports byte-identically (`diff` on regenerated vs original is empty); `leaderboard.md` shows `(mode × benchmark) → pass@1, verified_correctness, cost_per_solved` with BCa CIs and non-overlap markers; `per_language.md` lists languages with no benchmark coverage as `n/a`, not omitted.
  2. `ablations.md` delta tables (`full vs no_lsp`, `full vs no_semantic`, `full vs no_structured_edit`, `full vs baseline_plain`, `full vs baseline_rag`) compute correctly with CI overlap analysis; `cost_quality.md` scatter (cost vs verified_correctness) renders as ASCII/svg and cites cost-table `valid_until`.
  3. CI workflow file exists: `make bench-quick` runs on PR (ToolBench Go-only, no LLM cost, hard 5-min cap); full `make bench` runs nightly or on-demand, gated on a maintainer label; documented cost budget.
  4. Contamination canary: a known-novel pattern emitted in select tasks; a synthetic contaminated-response test trips the flag; flagged tasks are listed in `leaderboard.md` footnote and excluded from headline numbers.

**Plans**: 4 plans
Plans:
**Wave 1**

- [x] 89-01-PLAN.md — Report renderers: verified_correctness reduce + leaderboard column, per_language.md (n/a rows), ablations.md (5 deltas incl. aggregate-time no_semantic), cost_quality.md ASCII scatter [REPORT-01/02/03/04]
- [x] 89-04-PLAN.md — CI cost-policy workflow (.github/workflows/bench.yml: PR bench-quick 5-min cap + nightly/maintainer-gated full) + hermetic YAML-parse test + BENCH.md cost-budget/canary-policy doc [INFRA-04]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 89-02-PLAN.md — Contamination canary: aggregate-time exclusion of contaminated rows + leaderboard.md footnote + deterministic InjectPrompt production caller [INFRA-05]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 89-03-PLAN.md — helix-bench report --run-id: shared renderAll factoring, validated run-id, hermetic double-render byte-reproducibility, lockstep golden-guard updates [REPORT-05]

<details>
<summary>✅ v1.11 Semantic Index Completion & P1 MCP Tools (Phases 68-74) -- SHIPPED 2026-06-07</summary>

- [x] Phase 68: Precise FileFactDiff Populator (5/5 plans) — completed 2026-05-13
- [x] Phase 69: Production Status Accessors (6/6 plans) — completed 2026-05-14
- [x] Phase 70: Incremental Refresh Overlay-Drain (7/7 plans) — completed 2026-05-15
- [x] Phase 71: P1 Single-Symbol Read Tools (5/5 plans) — completed 2026-05-17
- [x] Phase 72: P1 Cluster & Impact Tools (5/5 plans) — completed 2026-05-19
- [x] Phase 73: P1 Tools Integration & E2E Verification (4/4 plans) — completed 2026-05-21
- [x] Phase 74: Close gap — wire P1 tool accessors in production daemon (6/6 plans) — completed 2026-06-03

**Full details:** `.planning/milestones/v1.11-ROADMAP.md`

</details>

## Backlog

### Candidate milestone — v2.2: Agent-Facing Skill Quality & Prompt Tuning

Seed scope for the next milestone (capture only — not yet planned via `/gsd-new-milestone`). Source analysis: `internal/cli/skills/helix/SKILL-ISSUE.md` (authored by the maintainer).

- **BL-SKILL-01 — Full SKILL.md + reference.md decision-matrix rewrite.** The current `internal/cli/skills/helix/SKILL.md` decision matrix is thin and incorrectly structured. Per `SKILL-ISSUE.md`: (1) split rows that mix QUERY verbs with ACTION verbs that mutate state (e.g. `get-semantic-graph-status` vs `index-semantic-graph`/`refresh-semantic-graph`; `read-memory`/`list-memories` vs `write-memory`; `search-memories` vs `rename`/`edit`/`delete-memory`; `switch-mode` vs `get-token-budget`); (2) add the missing "Not this" guidance to every row (8+ rows currently `—`); (3) fix the `reference.md` "Use this, not that" copy-paste errors and incorrect "Output" descriptions; (4) add prerequisite notes for the indexed-graph verbs (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context` all require `index-semantic-graph` first); (5) regroup the matrix by capability. Must stay consistent with the Phase 97 generator (`cmd/helix-refgen`) + `--check` drift gate and the `reference ⊇ VerbToolNames()` adoption contract — i.e. the rewrite likely means improving the generator/templates, not hand-editing generated output.

- **BL-SKILL-02 — DSPy-based offline prompt tuning of the agent-facing surface (exploratory).** Use DSPy (Stanford) to optimize the SKILL.md decision-matrix / nudge-steering prompt text against a measurable adoption metric. **Constraint:** Helix ships as a Go single binary with no Python/runtime deps — DSPy would be a **dev-time/offline optimization harness** (Python, under e.g. `tools/` or `bench/`) that emits an optimized, committed `SKILL.md`/`reference.md`, NOT a runtime dependency. **Metric already exists:** the Phase 101 opt-in LLM-behavioral adoption scorecard (`test/oracle/adopt` — `choice_rate`/`fallback_rate`, keyed on the first emitted command) is the natural DSPy objective, closing the loop from v2.1's measurement work to v2.2's optimization. Open questions for `/gsd-discuss-phase` when promoted: DSPy optimizer choice (MIPROv2 / BootstrapFewShot), train/dev task corpus source (reuse the adopt fixtures + vendored exercism tasks), and how to keep the optimized output reproducible/diffable under the existing `helix-refgen --check` gate.

_Promote via `/gsd-new-milestone` (after v2.1 is closed) or `/gsd-phase --add` once scoped._
