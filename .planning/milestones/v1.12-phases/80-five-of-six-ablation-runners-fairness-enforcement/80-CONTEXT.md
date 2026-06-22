# Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement - Context

**Gathered:** 2026-06-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Make all **6 ablation modes** runnable end-to-end on the Go ToolBench corpus by
growing Phase 77's table-driven mode→profile resolver — add the remaining five
`bench/runners/<mode>/MODE.md` directories — then **enforce the
same-model-same-budget fairness invariant at runner startup** and **compute the
three ablation deltas** so the attribution story holds on Go ToolBench alone (no
new infra).

**Four modes are fully real this phase** — `your_agent_full` (already seeded in
P77), `baseline_plain`, `no_lsp`, `no_structured_edit` — each produces a real,
schema-valid `result.v2.json` row tagged with its mode. **Two are deferred and
asymmetric:** `no_semantic` is *scaffolding* (runs via the existing
`bench-no-semantic.yaml` profile but the kernel-level zero-DuckDB guarantee
lands in **Phase 81**, ABLATE-06), and `baseline_rag` is a *registered stub*
(real standalone `cmd/helix-bench-rag` impl lands in **Phase 83**, ABLATE-04).

This phase is **growth, not invention**: the resolver, fairness contract struct,
ablation profiles, kernel disable flags, and per-cell result/trace pipeline all
already exist from Phases 75–77. Phase 80 wires them into 5 new mode dirs + a
startup fairness gate + a CI contract test + a minimal cross-mode delta pass.

**Requirements locked here:** ABLATE-01, ABLATE-03 (see `.planning/REQUIREMENTS.md`
lines 45, 47). WHAT/WHY are locked upstream in REQUIREMENTS.md and
v1.12-ROADMAP.md (4 success criteria); the decisions below are the HOW.

**Graded against (v1.12-ROADMAP Phase 80 success criteria):**
1. Each of 6 modes runs end-to-end on a smoke task; mode definition lives in
   `bench/runners/<mode>/MODE.md`; ablation runs produce schema-valid
   `result.v2.json` rows tagged with the mode.
2. `baseline_plain` reuses existing `internal/profile/profiles/baseline.yaml`
   (no new YAML); tool inventory is empty except the shell/grep/read/edit/test
   the agent runtime exposes natively (documented in `bench/BENCH.md`).
3. Same-model-same-budget invariant enforced at runner-startup; CI contract test
   asserts every runner's effective `(model_id, temperature, max_tokens,
   system_prompt_hash, retry_policy, cache_policy)` equals the fairness contract.
4. Ablation deltas (`your_agent_full − baseline_plain`, `full − no_lsp`,
   `full − no_structured_edit`) compute correctly on Go ToolBench and surface in
   the per-mode result rows.

</domain>

<decisions>
## Implementation Decisions

### baseline_plain runtime architecture (criterion #2; ABLATE-03)
- **D-01:** **`baseline_plain` still spawns a Helix daemon — with the existing
  `baseline.yaml` profile (zero Helix tools) — keeping the cell pipeline
  uniform.** It does NOT fork the cell path or skip the daemon. `baseline.yaml`
  already strips every Helix tool via the empty-lists mechanism
  (`skills:[] tools:[] exclude_tools:[]` → `ProfileFilterMiddleware` returns zero
  tools; verified by `TestBaselineExposesZeroHelixTools`, Phase 67 D-02), so the
  agent gets only the shell/grep/read/edit/test its own runtime exposes natively.
  - **Why:** one code path for all modes; the daemon-tap leg is still present so
    `baseline_plain`'s merged trace has the same 2-leg shape as the other modes
    (no special-casing in trace merge or result assembly). The "no Helix tools"
    property is enforced by the profile filter, not by omitting the daemon.
  - `baseline_plain` resolves via a new `bench/runners/baseline_plain/MODE.md`
    with `profile: baseline` (NOT a new `bench-*` YAML — ABLATE-03 forbids one).
  - Document the empty-inventory decision in `bench/BENCH.md` (criterion #2).

### no_semantic / baseline_rag scaffolding boundary (criterion #1)
- **D-02:** **Asymmetric resolution — the two deferred modes are NOT treated
  identically**, matching the phase-title parenthetical
  (`… + scaffolding for no_semantic`) and the Goal text (`no_semantic
  scaffolding … baseline_rag runner stub`).
  - **`no_semantic` RUNS end-to-end** via the existing `bench-no-semantic.yaml`
    profile (Phase 76) and produces a real, schema-valid `result.v2.json` row —
    but tagged with an explicit deferred-guarantee marker (D-03). The
    kernel-level `disable_semantic_subsystem` flag (ABLATE-06) that makes the
    ablation *honest* (zero DuckDB reads) is **Phase 81**; until then the daemon
    may still read the semantic store via back-channel SemanticLookup /
    RepoMap `SetEnrichFn` / RankFiles paths, so the row is NOT yet a clean
    no_semantic measurement.
  - **`baseline_rag` is a registered fail-closed stub.** It gets a
    `bench/runners/baseline_rag/MODE.md` + resolver entry, but its runner returns
    a clear `deferred to Phase 83` status and produces **NO result row** — the
    real RAG arm is the standalone `cmd/helix-bench-rag` binary + embedding index
    built in Phase 83 (ABLATE-04).
  - **Smoke assertion:** 4 real rows (`your_agent_full`, `baseline_plain`,
    `no_lsp`, `no_structured_edit`) + 1 marked-partial row (`no_semantic`) +
    `baseline_rag` registered-and-fail-closed (no row, asserts the deferral
    status). "Five-of-six" = five modes have a runnable runner this phase.

### Deferred-guarantee marker for no_semantic (criterion #1; honesty discipline)
- **D-03:** **Belt-and-suspenders marker — a machine-checkable field in the
  result row AND prose in MODE.md.** Add an `ablation_status` field (e.g.
  `guarantee_pending_phase_81`) to the `no_semantic` `result.v2.json` row so the
  Phase 82 aggregator can programmatically distinguish a partial row from a clean
  one; ALSO document the deferral in `bench/runners/your_agent_no_semantic/MODE.md`
  prose. Phase 81 flips the field to `enforced` when the kernel flag lands.
  - **Schema impact:** `bench/schema/result.v2.schema.json` is additive-only with
    top-level `additionalProperties` OPEN (D-03/D-04 versioning policy), so
    `ablation_status` lands as a new OPTIONAL field — **no schema major bump**.
    Real (fully-honest) modes either omit it or set it to `enforced`.

### Fairness enforcement (criterion #3; FAIR-01/02/03)
- **D-04:** **Two layers — runtime startup gate + static CI contract test.**
  1. **Startup gate:** each runner calls `runners.DefaultContract.Validate()` at
     startup and treats a non-nil return as **fatal** (refuse to run an unfair
     benchmark). This is the runtime fail-closed guard the result.v2 schema
     comment already anticipates ("rejected at load time by the runtime fairness
     loader (Phase 80+)"). The cell does NOT call `Validate()` today — Phase 80
     wires it in.
  2. **CI contract test:** asserts every runner's *effective*
     `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy,
     cache_policy)` equals `DefaultContract` (modulo justified `Overrides[]`
     entries that carry a non-empty `WaiverReason` + `ApprovedBy`).
  - **Scripted vs real:** the hermetic scripted-agent smoke has no real model, so
    the gate's *enforcement* bites the real `claude` path; the CI contract test
    is the always-on guarantee that no runner silently drifts from the contract.
    (Planner: decide whether `Validate()` is called unconditionally at cell
    startup or only when `--agent=claude` — Claude's discretion, but the contract
    test must be unconditional.)

### Ablation delta computation (criterion #4; boundary with Phase 82)
- **D-05:** **Minimal in-phase post-run delta pass — NOT the full aggregator.**
  After all modes for a task complete, a small post-run step computes exactly the
  three deltas (`your_agent_full − baseline_plain`, `full − no_lsp`,
  `full − no_structured_edit`) and surfaces them in the per-mode result rows
  (criterion #4 says "surface in the per-mode result rows" — so write back into /
  alongside the rows, not a standalone-only artifact). Phase 80 ships ONLY the
  minimal cross-mode delta needed for criterion #4.
  - **Boundary:** the full multi-run aggregator, BCa bootstrap, pass@k, >5%
    variance gate, and first leaderboard stay **Phase 82**. Phase 80's delta pass
    must NOT grow into that — it is single-run, three-fixed-deltas only. (Planner:
    avoid duplicating logic Phase 82 will own; keep the delta helper small and
    clearly scoped.)

### Claude's Discretion
- Full ABLATE-01 `MODE.md` frontmatter schema beyond `mode` + `profile` (Phase 77
  D-05 explicitly handed the "full convention" to Phase 80). Keep it minimal and
  table-driven; the resolver's strict `KnownFields(true)` parser means any added
  key must be reflected in `modeFrontmatter`. Candidate fields: an
  ablation-target / disabled-subsystem note, a delta-baseline pointer. Do NOT
  break the existing `your_agent_full/MODE.md` (it must keep resolving).
- Exact `ablation_status` field name / enum values (D-03) — pick a clear,
  greppable convention; Phase 81 consumes it.
- Whether the startup `Validate()` call is unconditional or gated on the real
  `claude` agent (D-04) — the CI contract test must be unconditional regardless.
- Exact shape/location of the delta helper and whether deltas are written into
  the row or into the row + a sibling `deltas.json` (D-05) — as long as they
  surface in the per-mode rows per criterion #4.
- The per-mode directory name for no_semantic: roadmap Phase 81 criterion #4
  references `bench/runners/your_agent_no_semantic/MODE.md` — prefer that exact
  path for forward-compat with Phase 81.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements (locked WHAT/WHY — do not re-derive)
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase 80" (lines 104–113) — Goal,
  Depends-on, Requirements, and the **4 Success Criteria** this phase is graded
  against; plus the "why this order" note (line 245: `no_semantic` is the
  trickiest mode and gets its own Phase 81).
- `.planning/ROADMAP.md` §"Phase 80" — active-roadmap entry (promoted from the
  milestone roadmap on 2026-06-18 to unblock discuss/plan tooling).
- `.planning/REQUIREMENTS.md` — **ABLATE-01** (line 45: 6 modes, each
  end-to-end + `bench/runners/<mode>/MODE.md`) and **ABLATE-03** (line 47:
  `baseline_plain` reuses `baseline.yaml`, empty inventory). Boundary rows:
  **ABLATE-02/05/07** (lines 46/49/51 → Phase 76, DONE: profiles + no_lsp +
  no_structured_edit kernel flags), **ABLATE-06** (line 50 → **Phase 81**:
  `disable_semantic_subsystem`), **ABLATE-04** (line 48 → **Phase 83**:
  standalone `cmd/helix-bench-rag`), **ABLATE-08** (line 52: `vet-ablation-leakage`
  analyzer, already wired into `make vet`).

### Contracts produced by prior phases (write into / extend these — do not redefine)
- `bench/runners/mode_resolver.go` — the table-driven `ResolveProfile(mode)` /
  `ResolveProfileFromRoot(root, mode)` that reads `bench/runners/<mode>/MODE.md`
  frontmatter. **Phase 80 adds 5 new mode dirs; the resolver needs NO change**
  (it's filesystem-as-table). `modeFrontmatter` uses strict `KnownFields(true)` —
  any new frontmatter key must be added to the struct.
- `bench/runners/your_agent_full/MODE.md` — the seed mode (P77 D-05); the
  template for the 5 new MODE.md files.
- `bench/runners/fairness_contract.go` — `DefaultContract` (single config
  source), `Validate()` (fatal on unjustified override), `DeprecationGate()`
  (injected clock), `ModeOverride` (pointer fields, free-text WaiverReason).
  **Phase 80 wires `Validate()` into runner startup (D-04).**
- `bench/runners/system_prompt.txt` + `fairness_contract_test.go` — the
  `SystemPromptHash` source and the `TestSystemPromptHashMatches` drift gate;
  the new CI contract test sits alongside (D-04).
- `bench/runtime/cell.go` — `RunCell` end-to-end cell (resolve mode→profile →
  sandbox → `StartDaemon` → agent → merge → write `result.v2.json`). **Phase 80's
  fairness gate (D-04) and the delta pass (D-05) attach here / around here.**
  Result path is keyed per `(task, mode, run_index)`:
  `<OutDir>/<task>/<mode>/<run_index>/result.v2.json`.
- `bench/schema/result.v2.schema.json` — additive-only (top-level
  `additionalProperties` OPEN); `ablation_status` lands as a new OPTIONAL field
  with NO major bump (D-03). Already has `mode` and `fairness.overrides[]`.
- `internal/profile/profiles/baseline.yaml` — the zero-Helix-tools control
  profile (P67 D-02); `baseline_plain` → `profile: baseline` (D-01).
- `internal/profile/profiles/bench-full.yaml`, `bench-no-lsp.yaml`,
  `bench-no-semantic.yaml`, `bench-no-structured-edit.yaml` — Phase 76 ablation
  profiles the new modes resolve to. `no_lsp`/`no_structured_edit` profiles carry
  the kernel disable flags (ABLATE-05/07); `no_semantic` resolves to
  `bench-no-semantic.yaml` but WITHOUT the kernel guarantee until Phase 81.
- `bench/BENCH.md` — operator-side contract; document `baseline_plain`'s empty
  inventory (criterion #2) and the no_semantic deferred-guarantee here.

### Prior-phase context (closest analogs — read before planning)
- `.planning/phases/77-bench-runtime-first-e2e-smoke/77-CONTEXT.md` — the cell
  pipeline, scripted-vs-real agent split (D-01), 2-leg merged trace (D-02), seed
  task (D-03), and the explicit "Phase 80 adds the other 5 modes + full ABLATE-01
  MODE.md convention" hand-off (D-05 + Deferred notes).
- `.planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/76-CONTEXT.md`
  — the 4 `bench-*.yaml` profiles + locked filenames + kernel disable-flag
  semantics the modes consume.
- `.planning/phases/75-schema-fairness-contract-tree-skeleton/75-CONTEXT.md` —
  the result.v2 schema versioning policy (additive-only, D-03/D-04) and the
  fairness-contract single-source design (FAIR-01/02/03).
- `.planning/research/PITFALLS.md` — milestone failure modes: fairness-contract
  drift, `tokens_to_model` vs `tokens_through_daemon` counting, daemon-tap PID
  cross-talk (relevant to the new modes' trace legs).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `bench/runners/mode_resolver.go` — table-driven resolver; 5 new `MODE.md` dirs
  drop in with zero Go change (the filesystem IS the table).
- `bench/runners/fairness_contract.go` — `DefaultContract` + `Validate()`
  already exist; Phase 80 only needs to CALL `Validate()` at startup and add the
  effective-config CI contract test (D-04).
- `internal/profile/profiles/baseline.yaml` — drop-in zero-tools profile for
  `baseline_plain` (D-01); `TestBaselineExposesZeroHelixTools` proves the
  empty-inventory property.
- `bench/runtime/cell.go` `RunCell` — the uniform per-cell path all 6 modes
  flow through; the daemon-tap + merge + result-write are mode-agnostic (D-01).

### Established Patterns
- **MODE.md frontmatter (`mode` + `profile`), strict `KnownFields(true)`** — fail
  closed on malformed/unknown keys; mode names are path-traversal-validated
  before any FS access (`validateModeName`, T-77-01).
- **One daemon per `(task, mode)` over a per-cell Unix socket, HTTP disabled** —
  no TCP ports → no collisions on `--parallel`; every mode (incl. baseline_plain)
  uses it (D-01, P77 D-06).
- **`result.v2.json` is provenance-complete, schema-valid, additive** — new
  fields land OPTIONAL with `additionalProperties` OPEN (D-03).
- **Fairness = single compile-time source (`DefaultContract`)** — no adapter
  defines its own model snapshot; overrides require a `WaiverReason` (D-04).

### Integration Points
- `bench/runners/<mode>/MODE.md` (×5 new): `baseline_plain` (profile: baseline),
  `no_lsp` (bench-no-lsp), `no_structured_edit` (bench-no-structured-edit),
  `your_agent_no_semantic` (bench-no-semantic, marked partial),
  `baseline_rag` (stub, fail-closed).
- `bench/runtime/cell.go` — wire `DefaultContract.Validate()` at startup (D-04);
  attach/trigger the post-run delta pass (D-05); set `ablation_status` on the
  no_semantic row (D-03); fail-close the baseline_rag runner (D-02).
- `bench/runners/` (new test) — CI contract test asserting effective config ==
  `DefaultContract` for every runner (D-04).
- `bench/schema/result.v2.schema.json` — additive `ablation_status` field (D-03).
- `bench/BENCH.md` — document baseline_plain empty inventory + no_semantic
  deferred guarantee (criteria #1/#2).

</code_context>

<specifics>
## Specific Ideas

- The phase-title parenthetical is the source of truth for the asymmetry:
  `(baseline_plain + your_agent_full + no_lsp + no_structured_edit + scaffolding
  for no_semantic)` = 4 real + no_semantic scaffold; baseline_rag is the sixth,
  effectively absent until Phase 83 (D-02).
- Prefer the directory name `bench/runners/your_agent_no_semantic/` for the
  no_semantic mode — Phase 81 criterion #4 already references that exact path.
- The result.v2 schema comment already names "the runtime fairness loader
  (Phase 80+)" as the rejecter of empty `waiver_reason` — confirms D-04's runtime
  gate is the planned home, not a new invention.

</specifics>

<deferred>
## Deferred Ideas

- **Kernel `disable_semantic_subsystem` flag (ABLATE-06) — the honest no_semantic
  ablation (zero DuckDB reads)** → **Phase 81**. Phase 80 only marks the
  no_semantic row `guarantee_pending_phase_81` (D-02/D-03); it does NOT un-wire
  SemanticLookup.
- **Real `baseline_rag`: standalone `cmd/helix-bench-rag` binary, 4 fixed tools,
  chromem-go vector store, OpenAI/Ollama embedding index (ABLATE-04)** → **Phase
  83**. Phase 80 ships only a fail-closed stub runner (D-02).
- **Multi-run aggregator, BCa bootstrap, pass@k, >5% variance gate, cost rollup,
  first leaderboard** → **Phase 82**. Phase 80's delta pass is single-run,
  three-fixed-deltas only — it must NOT grow into the aggregator (D-05).
- **Container runtime / per-language runners / external benchmarks** → Phases
  84–88; out of the "Go ToolBench corpus, no new infra" Phase 80 boundary.

None of the above expand Phase 80 scope — discussion stayed within the
five-of-six ablation-runner boundary.

### Planner notes (apply during plan-phase)
1. **Phase 80 GROWS, it does not refactor.** Add 5 `MODE.md` dirs onto the P77
   resolver (no Go change to the resolver), wire `Validate()` into the existing
   cell, add a CI contract test, add a minimal delta pass. Reuse the P75–77
   substrate; invent nothing new.
2. **Asymmetric deferred modes (D-02):** no_semantic RUNS (via existing profile,
   marked partial); baseline_rag is a fail-closed stub (no row). Do NOT force them
   symmetric.
3. **Fairness is two layers (D-04):** runtime startup `Validate()` (fatal) +
   unconditional CI contract test asserting effective config == `DefaultContract`.
4. **Deltas are minimal & in-phase (D-05):** exactly 3 deltas, single-run,
   surfaced in per-mode rows. The aggregator is Phase 82 — keep the helper small.
5. **baseline_plain reuses `baseline.yaml` (D-01, ABLATE-03):** no new YAML; empty
   inventory enforced by the profile filter; still spawns the daemon for trace
   symmetry. Document in `bench/BENCH.md`.
6. **`ablation_status` is additive (D-03):** new OPTIONAL field, no schema major
   bump; Phase 81 flips it to `enforced`.

</deferred>

---

*Phase: 80-five-of-six-ablation-runners-fairness-enforcement*
*Context gathered: 2026-06-18*
