# Requirements: Helix v2.2 Agent-Facing Skill Quality & Prompt Tuning

**Milestone goal:** Rewrite the helix agent-facing skill surface (hand-authored `SKILL.md` decision matrix + generated `reference.md`) for correctness and completeness, harden the skill bundle, and explore DSPy-based offline tuning of that surface against the existing adoption scorecard. Source analysis: `internal/cli/skills/helix/SKILL-ISSUE.md`. Research: `.planning/research/SUMMARY.md`.

**Constraints carried forward:** Helix stays a Go single binary with no Python/runtime deps (DSPy is dev-time/offline only); `reference.md` is generated, never hand-edited (preserve the Phase 97 `--check` drift gate and `reference ⊇ VerbToolNames()` contract); anti-vacuity discipline (every gate ships a deliberate break-the-invariant → assert-RED test).

## v2.2 Requirements

### Bundle Integrity (BUNDLE)

- [x] **BUNDLE-01**: `helix setup` installs exactly the two skill-bundle files (`SKILL.md` + `reference.md`) — `installSkill`/`uninstallSkill` use an explicit closed-set allowlist, and a bundle-contents test asserts the installed set equals **exactly** `{SKILL.md, reference.md}` (goes RED on any stray file in `internal/cli/skills/helix/`). `SKILL-ISSUE.md` is moved out of the embed dir so it no longer compiles into the binary or installs to users.
- [x] **BUNDLE-02**: the reference-completeness contract test is hardened to be non-vacuous — it asserts the generated `reference.md` covers the frozen verb set by **exact count** (`== len(VerbToolNames())`, currently 50) and goes RED when a known verb is absent — landed **before** the reference/skill format rewrite so the gate cannot pass vacuously through the churn.

### Reference Generator Correctness (REFGEN)

- [x] **REFGEN-01**: every verb's "use this, not that" and "Output" lines in the generated `reference.md` are correct for that verb's actual semantics — fixing the `cmd/helix-cligen`/`cmd/helix-refgen` group-collapse root cause via a per-verb override in the **generator** (not hand-edits); the corrected `reference.md` is regenerated, committed, and passes `helix-refgen --check` byte-for-byte.

### Skill Decision-Matrix Quality (SKILL)

- [x] **SKILL-01**: no `SKILL.md` decision-matrix row mixes a QUERY (read-state) verb with an ACTION (mutate-state) verb — query and action verbs occupy separate rows (resolves the SKILL-ISSUE.md rows 68/70/71/72/73 grouping errors).
- [x] **SKILL-02**: every decision-matrix row carries explicit "Not this" guidance — no `—` placeholders remain.
- [x] **SKILL-03**: every indexed-graph verb (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`) carries a "requires `index-semantic-graph` first" prerequisite note, and the matrix is grouped by capability — while preserving the `## Decision matrix` StripDecisionMatrix anchor and the SKILL-04 idle-cost (size) cap.

### Offline Prompt Tuning (TUNE) — exploratory

- [ ] **TUNE-01**: an opt-in, dev-time-only DSPy harness (under `tools/`, gitignored output, excluded from `go test ./...`, with NO runtime Python dependency in the `helix` binary or `helix setup`) optimizes the agent-facing skill/steering text against the Phase 101 adoption scorecard metric (re-implemented in Python with a parity cross-check against the Go `adopt` classifier), with overfit/metric-gaming guards; the harness may conclude **no-ship**, and any adopted output is committed and passes `helix-refgen --check`. A `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path.

## Future Requirements (deferred)

- **TUNE-FUT-01**: upgrade the DSPy optimizer from GEPA to MIPROv2/COPRO with a larger held-out set if GEPA's prose evolution proves insufficient (gated on a corpus big enough for minibatch overfit protection, `val_size > 50`).
- **TUNE-FUT-02**: a quality-joined optimization metric (adoption choice-rate AND task-success, e.g. via the v2.1 Aider-derived benches) instead of adoption alone.

## Out of Scope

- **Runtime Python / shipping DSPy** in the `helix` binary, `helix setup`, or the default `go test ./...` / merge path — single-binary identity is non-negotiable; DSPy is strictly dev-time/offline.
- **Hand-editing generated `reference.md`** — fails `--check`; all reference corrections go through the generator.
- **New Go module dependencies** — the rewrite/allowlist work uses only already-vendored packages.
- **Changing nudge steering classifier behavior** — v2.1 already broadened steering; v2.2 touches only the skill/reference TEXT, not the exit-0 advisory classifier logic.
- **Refreshing the stale `.planning/codebase/` Serena-era maps** — a separate `/gsd-map-codebase` task, not part of this milestone.

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BUNDLE-01 | Phase 103 | Complete |
| BUNDLE-02 | Phase 103 | Complete |
| REFGEN-01 | Phase 104 | Complete |
| SKILL-01 | Phase 105 | Complete |
| SKILL-02 | Phase 105 | Complete |
| SKILL-03 | Phase 105 | Complete |
| TUNE-01 | Phase 106 | Pending |
