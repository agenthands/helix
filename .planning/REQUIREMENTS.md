# Requirements: Helix v2.1 — Agent Adoption & Aider-Derived Validation

**Defined:** 2026-06-22
**Core Value:** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.

**Milestone goal:** Make AI coding agents reliably reach for `helix` verbs over standard tools (comprehensive generated reference + stronger steering + multi-agent coverage + a measured adoption contract), then prove the toolset works end-to-end with vendored Aider benchmarks and committed local baselines.

**Cross-cutting invariants (apply to every requirement below; not separate line items):**
- **Anti-vacuity:** every gate this milestone adds ships a deliberate break-the-invariant → assert-RED test (the v1.12 lesson — four named vacuous-pass CRITICALs were caught only by adversarial revert-and-fail tests).
- **HELIX_BIN fail-not-skip:** every bench surface ships a hermetic golden sibling as the sole authoritative proof AND fails (not silently SKIPs) when `HELIX_BIN` is set but the run produces no `result.v2.json` / empty bucket / missing metric line.
- **Local-only benches:** no CI benchstat gate is (re)introduced (firm rule since v1.9); baselines are captured + committed locally.
- **Generate-don't-hand-write / reuse-don't-fork / measure-don't-port:** reference generated from the registry; polyglot adapter (`RunExercise`) reused verbatim; existing `internal/repomap` + `internal/fuzzy` engines measured, not reimplemented.

---

## v1 Requirements

### Per-Verb Reference & Skill Bundle (REF)

- [ ] **REF-01**: A new `cmd/helix-refgen` generates an agent-facing `reference.md` from the live tool registry (`skill.ToolProviders()` + `internal/kernel/help`), covering every verb in `internal/cli/verbs_gen.go` with synopsis, args, output shape, a worked example, and explicit "use this not that" guidance.
- [ ] **REF-02**: `internal/cli/skill.go` switches from a single embedded `string` to an `embed.FS` skill bundle; `installSkill` ships `reference.md` alongside `SKILL.md` atomically with the existing path-containment guarantee.
- [ ] **REF-03**: `helix-refgen --check` is a merge-gating drift gate (wired into `make` + CI) sourced from `verbs_gen.go` as the authority, inheriting the docgen blank-import-parity-with-daemon rule; a hand-edited or stale `reference.md` fails the gate.

### Steering & Nudge (STEER)

- [ ] **STEER-01**: The `PreToolUse` nudge's code-target classifier is broadened to steer more standard-tool invocations (grep/sed/cat/find/Read-shaped Bash) toward the specific equivalent `helix` verb, preserving the advisory exit-0 / fail-open contract.
- [ ] **STEER-02**: A SessionStart priming surface presents the terse "use X not Y" decision matrix once per session, size-capped (SKILL-04-style idle-cost bound) and fail-open.
- [ ] **STEER-03**: Negative-control coverage proves the nudge does NOT fire on legitimately-correct standard-tool use (prose/log/config/build-output targets, e.g. `grep TODO README.md`), enforced by golden classifier rows.

### Multi-Agent Coverage (AGENT)

- [ ] **AGENT-01**: The generated verb reference is installable for non-Claude agents (shared markdown reference, not a per-agent bespoke skill engine).
- [ ] **AGENT-02**: `helix setup` writes/updates per-agent instruction files (Codex `AGENTS.md` ≤32 KiB, Gemini `GEMINI.md`, generic) via idempotent sentinel-delimited append that never clobbers a user's existing file.
- [ ] **AGENT-03**: Codex's `PreToolUse` hook is wired to `helix nudge` reusing the existing advisory envelope; agents with no PreToolUse-equivalent (Gemini, IDE, generic) get instruction-file steering only — no fabricated hook.

### Adoption Evaluation (ADOPT)

- [ ] **ADOPT-01**: A deterministic, merge-gating adoption-contract test asserts (a) `reference ⊇ VerbToolNames()` completeness and (b) a per-shape nudge-fires golden table mapping each standard-tool shape to the specific suggested verb — non-vacuous (revert-and-fail proven), keyed on the emitted command, rejecting empty-bucket-as-pass.
- [ ] **ADOPT-02**: An opt-in LLM-behavioral adoption scorecard (reusing the v1.4 `llm`/`llmjudge` harness, build-tag gated, never blocks merge) measures an agent's helix-choice rate and standard-tool fallback rate, with a sabotaged-skill revert-and-fail self-test and a negative judge exemplar so the score can actually fail.

### Aider Fixture Vendoring (VENDOR)

- [ ] **VENDOR-01**: The MIT-licensed Exercism polyglot fixtures (a recorded, deterministic subset) are vendored into the tree under `bench/datasets/aider-polyglot/fixtures/` with `SPDX-License-Identifier: MIT`, per-track NOTICE/attribution, and a `VENDOR-MANIFEST.md`.
- [ ] **VENDOR-02**: Aider's Apache-2.0 edit-format fixtures are vendored with `SPDX-License-Identifier: Apache-2.0` + attribution, producing a documented mixed-license vendored tree.
- [ ] **VENDOR-03**: `make verify-licenses` is extended to hard-fail over the full vendored tree (dual MIT + Apache-2.0 disposition), with a tamper test proving the gate goes RED on a missing/incorrect SPDX header or NOTICE.

### Polyglot Edit Benchmark (EDITBENCH)

- [ ] **EDITBENCH-01**: An EDIT-verb `AgentFn` (in `bench/runtime`, daemon-dialing) routes the model's edit through helix edit verbs (`replace-symbol-body` / `fuzzy-edit` / `replace-in-file` / `insert-before-symbol` / `insert-after-symbol`), plugged into the existing `RunExercise` verb-agnostic seam — loader untouched, WR-01 anti-tamper pristine-test restore preserved.
- [ ] **EDITBENCH-02**: A new `bench/runners/aider_edit/MODE.md` adds the polyglot-edit bench mode via the filesystem-as-table pattern with zero mode-resolver Go change.
- [ ] **EDITBENCH-03**: An additive `edit_format_applied` (`*bool`, `omitempty`) open key is recorded on `result.v2.json` with no `schema_version` v3 bump.

### RepoMap-Quality Eval (REPOEVAL)

- [ ] **REPOEVAL-01**: A new `bench/evaluators/repomapeval` leaf measures `get-repo-map` / `get-context` ranking quality (recall@k / MRR / nDCG) and token-budget fit against a gold corpus, stdlib-only and respecting the `vet-ablation-leakage` no-kernel-import boundary.
- [ ] **REPOEVAL-02**: A broad, multi-language gold corpus is authored from task ground truth (e.g. exercism `files.solution`) independent of `get-repo-map` output, with a documented per-language size floor and a reversed/random-ranker discriminator that MUST fail the corpus.

### Fuzzy / Edit-Format Robustness Bench (FUZZBENCH)

- [ ] **FUZZBENCH-01**: A new `bench/evaluators/fuzzyrobust` leaf measures `internal/fuzzy` 4-strategy selection and ambiguity refusal (reusing `editsim.ES`), stdlib-only and respecting the leaf-import boundary.
- [ ] **FUZZBENCH-02**: A broad native drift corpus (across the 4 strategies and multiple languages) is authored with expected strategy derived from the drift type (not observed behavior), including at least one known-ambiguous case that MUST be refused, with a documented size floor.

### Committed Baselines (BASELINE)

- [ ] **BASELINE-01**: A committed polyglot-edit baseline artifact (`bench/reports/<run>/BENCH-RESULTS.md` + `result.v2.json`) is captured `HELIX_BIN`-gated, fail-not-skip, with byte-reproducible deterministic metrics only (latency excluded — local `bench-micro`).
- [ ] **BASELINE-02**: Committed RepoMap-eval and fuzzy-robustness baseline artifacts are captured under the same guards (fail-not-skip, byte-reproducible, deterministic-metrics-only).

---

## v2 Requirements

Deferred to a future milestone. Tracked, not in this roadmap.

### Adoption (ADOPT)

- **ADOPT-03**: Continuous adoption telemetry from real agent sessions (opt-in metric on nudge acceptance / verb-vs-grep ratio over time).

### Benchmark Coverage (EDITBENCH / AGENT)

- **EDITBENCH-04**: Full 6-track / 225-task vendored polyglot expansion beyond the recorded v2.1 subset.
- **AGENT-04**: First-class skill/hook install for additional agent runtimes beyond Codex/Gemini/generic as their hook surfaces mature.

---

## Out of Scope

Explicitly excluded (anti-features from research). Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Deny/block `PreToolUse` hook (exit 2) | grep/sed/cat/Read are legitimately correct for prose, logs, config, and build output; steering stays advisory exit-0 / fail-open |
| Per-agent bespoke skill engine | Non-Claude agents read a markdown instruction file + shared reference; a second engine is needless surface |
| Rebuilding the v1.12 aider-polyglot adapter | `RunExercise` is reused verbatim; rebuild would regress the WR-01 anti-tamper invariant |
| Reimplementing Aider's RepoMap / edit algorithm | v2.1 MEASURES Helix's existing `internal/repomap` + `internal/fuzzy`; it does not port aider's algorithm |
| Making the LLM-behavioral adoption score a merge gate | LLM nondeterminism must never block merge; the deterministic contract (ADOPT-01) is the gate |
| CI benchstat gate / live full polyglot run in CI | Firm local-only bench rule since v1.9; baselines committed locally, `HELIX_BIN`-gated |
| Fabricating a Gemini `PreToolUse` hook | Gemini CLI has no PreToolUse-equivalent; instruction-file steering only |
| New Go dependencies | Research confirmed zero new deps; the work is conventions, vendored data, and glue |

---

## Traceability

Populated during roadmap creation (phases continue from 97). Each requirement maps to exactly one phase.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REF-01 | TBD | Pending |
| REF-02 | TBD | Pending |
| REF-03 | TBD | Pending |
| STEER-01 | TBD | Pending |
| STEER-02 | TBD | Pending |
| STEER-03 | TBD | Pending |
| AGENT-01 | TBD | Pending |
| AGENT-02 | TBD | Pending |
| AGENT-03 | TBD | Pending |
| ADOPT-01 | TBD | Pending |
| ADOPT-02 | TBD | Pending |
| VENDOR-01 | TBD | Pending |
| VENDOR-02 | TBD | Pending |
| VENDOR-03 | TBD | Pending |
| EDITBENCH-01 | TBD | Pending |
| EDITBENCH-02 | TBD | Pending |
| EDITBENCH-03 | TBD | Pending |
| REPOEVAL-01 | TBD | Pending |
| REPOEVAL-02 | TBD | Pending |
| FUZZBENCH-01 | TBD | Pending |
| FUZZBENCH-02 | TBD | Pending |
| BASELINE-01 | TBD | Pending |
| BASELINE-02 | TBD | Pending |

**Coverage:**
- v1 requirements: 23 total
- Mapped to phases: 0 (roadmap pending)
- Unmapped: 23 ⚠️ (resolved at roadmap creation)

---
*Requirements defined: 2026-06-22*
*Last updated: 2026-06-22 after initial definition*
