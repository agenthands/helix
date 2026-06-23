# Project Research Summary

**Project:** Helix
**Domain:** Agent-facing tool-selection skill docs + generated-doc gates + (exploratory) offline DSPy prompt tuning, on a Go single-binary CLI
**Researched:** 2026-06-23
**Confidence:** HIGH

## Executive Summary

Helix v2.2 ("Agent-Facing Skill Quality & Prompt Tuning") is a content/codegen milestone, NOT a stack milestone. It carries four features: three deterministic Go-side skill-surface fixes (a SKILL.md decision-matrix rewrite, `reference.md` generator corrections, and an `installSkill` bundle allowlist) plus one exploratory, dev-time-only DSPy prompt-optimization harness. All four hang off the existing skill-bundle generate→install→measure spine — the 50-verb CLI, the `//go:embed skills/helix/*` bundle, `cmd/helix-refgen` with its `--check` drift gate and `reference ⊇ VerbToolNames()` contract, and the Phase 101 `test/oracle/adopt` `choice_rate`/`fallback_rate` scorecard. The three deterministic features need ZERO new Go dependencies (edits inside already-vendored packages); only the DSPy spike introduces anything new, and it must stay strictly out of the shipped binary, the Go module graph, and the merge-gating CI path.

The recommended approach is dependency-driven and sequences deterministic-before-exploratory. Land the `installSkill` allowlist FIRST: the embed-glob leak is live right now (the binary and every `helix setup` already ship the 18 KB `SKILL-ISSUE.md`), the existing bundle test is positive-only, and putting a closed-set allowlist in place before the rewrite stops further leakage during the dir churn. Then fix the `reference.md` generator at its root cause (`cmd/helix-refgen` per-verb prose driven off a `cmd/helix-cligen` group collapse that folds memory/workflow/health/help/profile into one `memory` group) BEFORE re-authoring the SKILL.md matrix, so the on-demand reference is already correct when the idle-tier matrix is rewritten and the two tell one consistent story. The DSPy harness comes last, tuning against a frozen metric and a stabilized surface — and it is explicitly a spike with a possible no-ship outcome (Go-search-loop fallback).

The dominant risk class is vacuous gates — a failure pattern this repo has been bitten by repeatedly (Phase 86/87/89 CR-01). Two hard rules govern the whole milestone: (1) `reference.md` corrections are GENERATOR fixes, never hand-edits — a hand-edit fails `--check` or certifies an irreproducible file; and (2) the contract test must be hardened to be DISCRIMINATING (exact count == 50, fails on a known-absent verb) BEFORE the format rewrite, or the superset check can go `∅ ⊇ ∅` vacuously green. For DSPy, the risks are overfit on a tiny corpus, metric-gaming of the gameable `choice_rate` proxy, LLM nondeterminism breaking reproducibility, and accidental runtime-Python coupling — all mitigated by treating DSPy as offline/human-in-the-loop whose only output that crosses into the binary is committed static bytes gated by `--check`.

## Key Findings

### Recommended Stack

v2.2 adds NO new Go dependencies. The three deterministic features are edits inside existing packages (`cmd/helix-refgen/render.go`, `cmd/helix-cligen/render.go`, `internal/cli/skill.go`, `internal/cli/skills/helix/`); `embed`, `os`, `path/filepath`, `cobra`, `testify`, `jsonschema/v6`, `anthropic-sdk-go` are all already vendored. The only new surface is the exploratory DSPy harness, quarantined to a dev-time/offline Python tree (recommended `tools/skill-tune/` or `tools/promptopt/`) with its own pinned `requirements.txt` and a git-ignored venv — never on the default `go test ./...` / merge path.

**Core technologies:**
- **Go 1.25.1 (existing)**: all shipped code; the deterministic features touch only existing packages — no new import wanted or needed.
- **DSPy (Python) 3.2.1 (stable)**: the offline prompt-optimizer that tunes SKILL.md/nudge prose against the adoption scorecard; dev-time only, output committed as static text. Pin stable 3.2.1 (3.3.0b1 beta exists; defer).
- **GEPA optimizer (`dspy.GEPA`, bundled)**: recommended for free-form instruction/prose tuning (which SKILL.md is) — fewer rollouts, no demo bank. MIPROv2 (`auto="light"`, needs Optuna) / COPRO are alternates if few-shot demos prove valuable. Reuses the existing `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` (same vars as `test/oracle/llm`, via LiteLLM).

### Expected Features

The maintainer analysis (`SKILL-ISSUE.md`) already identified what is broken; the table-stakes fixes are well-specified and low-complexity.

**Must have (table stakes):**
- **`installSkill` bundle allowlist + closed-set bundle test** — predictable, minimal `{SKILL.md, reference.md}` bundle; stops the live stray-file leak. Hygiene table-stakes.
- **One-question-per-row matrix** — split the 5 mixed QUERY/ACTION rows (37→44 rows); an agent maps one intent → one tool.
- **Explicit "use this NOT that" on every row** — fill every blank `—` cell (the semantic-graph/memory/workflow verbs least familiar to agents) with the concrete fallback it displaces; CLAUDE.md routing table is the canonical source.
- **Per-verb accuracy in the generated reference** — fix the 13 copy-pasted "use this/not that" + 22 copy-pasted FTS5 "Output" lines AT THE GENERATOR, keeping `--check` and `⊇ VerbToolNames()` green.
- **Indexed-graph prerequisite annotations** — 7 semantic-graph verbs silently degrade without `index-semantic-graph`; without a prereq note the agent gets an empty result and falls back to grep.
- **Reproducible, `--check`-gated output** — every generated/tuned artifact must round-trip through `helix-refgen --check`.

**Should have (competitive):**
- **Adoption-metric-driven content** — regression-check the rewrite against the hermetic `choice_rate`/`fallback_rate` scorecard (runs in default `go test ./...`, no network), not maintainer taste.
- **DSPy offline tuning harness** (exploratory) — auto-optimize the matrix/nudge text to maximize `choice_rate`; reuse the `test/oracle/adopt` first-command classifier as the single metric for both the Go gate and the Python optimizer.

**Defer (v2+ / exploratory):**
- **DSPy harness itself** — highest complexity, exploratory spike, possible no-ship; value unproven until the deterministic rewrite establishes a baseline `choice_rate`. Clean fallback: a hand-rolled Go candidate-search loop over the `adopt` scorer, keeping the milestone 100% Go.

### Architecture Approach

v2.2 is a subsequent-milestone, integrate-WITH note (phase numbering continues from 102). The generate-time spine (`cmd/helix-cligen` → `verbs_gen.go` → `VerbSpecsForDocs()`/`VerbToolNames()` → `cmd/helix-refgen/render.go` → `reference.md`, all `--check`-gated) and the runtime spine (`//go:embed skills/helix/*` → `installSkill`) are both touched only at well-isolated seams. The root cause of the reference.md "copy-paste errors" is that `outputShape(group)` and `useThisNotThat(group, verb)` are GROUP-keyed, and `cmd/helix-cligen`'s `categoryToGroup` collapses four distinct categories (memory, workflow, health, help, profile) into the single `memory` group — so `switch-mode`, `get-token-budget`, `onboard-project`, `get-health`, `get-tool-help`, and the mutating memory verbs all inherit the memory-query prose. The recommended fix is a per-verb override map in refgen (option 1, minimal blast radius), NOT an upstream group-taxonomy split (option 2, larger/riskier).

**Major components:**
1. **`internal/cli/skill.go` (`installSkill`/`uninstallSkill`)** — add a `bundleFiles` allowlist, filter the entry list ONCE up front, feed the same filtered slice to the unchanged 2-pass atomic stage→rename and `withinSkillRoot` containment; filter uninstall identically. New closed-set bundle test.
2. **`cmd/helix-refgen/render.go`** — add per-verb `Output` + "use this, not that" override maps with group-default fallback; lookups keyed (never ranged) so determinism holds. New vacuity-guard tests.
3. **`internal/cli/skills/helix/SKILL.md`** — hand-authored matrix rewrite (split rows, "Not this" everywhere, regroup by subsystem reality, indexed-graph prereqs); MUST keep the `## Decision matrix` heading (the `StripDecisionMatrix` anchor) and stay under the SKILL-04 idle-cost cap.
4. **`tools/skill-tune/` (new, exploratory)** — dev-time DSPy harness; NOT in the Go module, NOT embedded, NOT in CI; consumes the metric via a Python re-implementation of the `adopt` classifier pinned by a golden parity cross-check; output re-enters only as a human-reviewed commit through SKILL.md/refgen → `--check`.

### Critical Pitfalls

1. **Embed glob ships stray files (LIVE leak)** — `installSkill` writes every `ReadDir("skills/helix")` entry, so `SKILL-ISSUE.md` is already in the binary and on every user's disk; the existing test is positive-only. Fix: closed-set allowlist assertion (embedded FS == exactly `{SKILL.md, reference.md}`) + move `SKILL-ISSUE.md` out of the embed dir. Land EARLY.
2. **Hand-editing the generated `reference.md`** — it is generated and `--check`-gated; a hand-edit either reverts on next regen or certifies an irreproducible file. Fix `render.go`'s per-verb mapping, regenerate, commit both together; verify `git diff` empty after a fresh regen.
3. **The `reference ⊇ VerbToolNames()` contract goes vacuous after the format rewrite** — if the matcher's anchor format changes, the superset check becomes `∅ ⊇ ∅` true. HARDEN FIRST: assert matched count == 50, discriminate a known-absent verb, run the test RED against the old format to prove it bites — all BEFORE rewriting the format.
4. **DSPy overfit + metric-gaming on a tiny corpus** — with ~5 transcripts the optimizer memorizes surface cues (MIPROv2 only auto-protects when `val_size > 50`), and `choice_rate` is a gameable first-command proxy ("always emit helix"). Hold out a TEST split the optimizer never sees; pair `choice_rate` with a correctness/task-success oracle; inspect for degenerate steering.
5. **DSPy nondeterminism / runtime-Python coupling** — LLM optimization isn't bit-reproducible (even temp-0), so never make "re-run DSPy" a CI gate; gate the committed artifact, not the process. Keep DSPy out-of-band (no `helix` subcommand shells to Python, no `go.mod`/setup edge); add a `make vet`-style leakage analyzer.

## Implications for Roadmap

Based on combined research, the convergent suggested phase structure (continuing from Phase 102) is:

### Phase 103: installSkill allowlist + closed-set bundle test
**Rationale:** Pure safety patch, zero deps, smallest blast radius; the leak is live and the dir will churn during the rewrite, so the allowlist must precede it. Churns least.
**Delivers:** `bundleFiles` allowlist filtering install + uninstall (single up-front filter, atomicity/containment untouched), a closed-set bundle-contents test, `SKILL-ISSUE.md` moved out of the embed dir, AND a hardened `reference ⊇ VerbToolNames()` contract test (exact count == 50, discriminates a known-absent verb, RED-first).
**Addresses:** allowlist + bundle hygiene (table stakes).
**Avoids:** Pitfall 1 (live leak), Pitfall 2 (atomicity regression via uninstall parity test), Pitfall 5 (vacuous contract — harden it here, before the format changes).

### Phase 104: reference.md generator per-verb override fixes
**Rationale:** Fix the §4/§5 copy-paste errors at the GENERATOR root cause (the group collapse), not by hand-edit; MUST precede the SKILL.md rewrite so the reference is already correct when the matrix is re-authored.
**Delivers:** per-verb `Output` + "use this, not that" override maps in `cmd/helix-refgen/render.go` (group-default fallback retained), vacuity tests (no override key is a non-verb; overridden Output ≠ old group default), regenerated `reference.md`.
**Uses:** existing `cmd/helix-refgen` + `--check` gate; no new deps.
**Implements:** generator override seam (architecture option 1).
**Avoids:** Pitfall 3 (hand-edit), Pitfall 4 (blank-import parity re-verified when touching refgen).

### Phase 105: SKILL.md decision-matrix rewrite
**Rationale:** Hand-authored idle-tier rewrite; depends on 104 so SKILL.md and reference.md tell one consistent story.
**Delivers:** split QUERY/ACTION rows (37→44), "Not this" on every row, regroup by subsystem reality, indexed-graph prereq notes; keep the `## Decision matrix` StripDecisionMatrix anchor and the SKILL-04 idle-cost cap; SKILL.md↔VerbToolNames cross-check test.
**Addresses:** the core mis-routing fixes (table stakes).
**Avoids:** Pitfall 11 (over-split/token bloat — target ~44 rows, terse "Not this"), Pitfall 12 (ambiguous/stale steering), Pitfall 13 (missing prerequisites), the StripDecisionMatrix anchor break.

### Phase 106 (exploratory): DSPy offline tuning harness
**Rationale:** LAST — tunes against a frozen metric and the stabilized 104/105 surface; tuning against a moving target wastes optimizer budget.
**Delivers:** `tools/skill-tune/` dev-time harness (DSPy 3.2.1 + GEPA), a Python re-implementation of the `adopt` classifier pinned by a golden parity cross-check against the Go scorer, train/dev/held-out splits, offline optimize → human-reviewed commit → `--check`.
**Uses:** DSPy 3.2.1, GEPA, reused `ANTHROPIC_API_KEY`/`DEEPSEEK_API_KEY`.
**Avoids:** Pitfalls 6–10 (overfit, metric-gaming, nondeterminism, contamination, runtime-Python coupling).

### Phase Ordering Rationale

- **103 before 104/105:** a clean embed dir means no stray-file noise in the bundle-contents test or the generated reference; the allowlist stops leakage during the rewrite churn.
- **104 (generator/reference) before 105 (SKILL.md):** the on-demand reference must be correct before the idle-tier matrix is re-authored, avoiding a window where the two disagree.
- **106 strictly last:** DSPy tunes against a frozen metric and a stable surface; it is exploratory with a possible no-ship outcome and a Go-search-loop fallback.
- All three research files (ARCHITECTURE build order, PITFALLS phase mapping, FEATURES dependency graph) converge on this exact sequence.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 106 (DSPy harness):** exploratory spike — needs corpus-split design, the Python↔Go classifier parity contract, the metric-AND-quality-oracle composition, and a leakage-analyzer design. Highest uncertainty; mark as spike, not a hard adoption-delta gate.

Phases with standard patterns (skip research-phase):
- **Phase 103 (allowlist):** well-understood install hardening; the seam (filter entry list once, preserve atomicity) is fully specified.
- **Phase 104 (generator fix):** root cause and fix shape (per-verb override map) are identified and grounded in source.
- **Phase 105 (SKILL.md rewrite):** hand-authored Markdown against the maintainer-specified `SKILL-ISSUE.md` target; constraints (anchor, idle-cost cap) are explicit.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | DSPy version/extras/LM-config verified against PyPI + dspy.ai; Go "no new deps" read directly from the tree. |
| Features | HIGH | Skill-doc fixes grounded in `SKILL-ISSUE.md`; DSPy mechanics Context7-verified (`/websites/dspy_ai`). |
| Architecture | HIGH | All Go integration points (refgen, cligen, skill.go, scorecard) read directly from live source; group-collapse root cause pinned to file/line. |
| Pitfalls | HIGH | Generator/gate/bundle pitfalls verified against live source; DSPy pitfalls Context7/web-verified (MEDIUM-HIGH). |

**Overall confidence:** HIGH

### Gaps to Address

- **DSPy generalization on a tiny corpus:** the live adoption corpus is small (`MinTasks=5`); a held-out test split must be established (and likely the corpus grown) before any optimizer call. Handle in Phase 106 planning — the split is the FIRST harness task.
- **Optimizer-reproducibility vs `--check`:** the artifact/process split must be an explicit Phase 106 architecture decision (commit the tuned artifact, never re-run the optimizer in `--check`).
- **Per-verb override vs group-taxonomy split (104):** option 1 (override map) is recommended; if the 105 SKILL.md regroup independently demands new cobra groups, re-evaluate option 2 during 104/105 planning.
- **Stale codebase docs:** `.planning/codebase/{ARCHITECTURE,CONCERNS}.md` describe the removed Python Serena tree and were NOT used as sources — do not let them re-enter planning context.

## Sources

### Primary (HIGH confidence)
- Live Go tree (read directly): `cmd/helix-refgen/{main,render}.go`, `cmd/helix-cligen/render.go`, `internal/cli/{skill,verb}.go`, `internal/cli/reference_contract_test.go`, `internal/cli/skills/helix/{SKILL.md,SKILL-ISSUE.md}`, `test/oracle/adopt/scorecard.go`, `test/oracle/llm/*`, `go.mod`, `Makefile` — confirms "no new Go deps," the group-collapse root cause, the live leak, and the scorecard classifier.
- `.planning/PROJECT.md` §v2.2 + Constraints — four target features; single Go binary, no runtime Python; DSPy dev-time only; committed `--check`-reproducible artifact.
- Context7 `/websites/dspy_ai` — optimizers (GEPA/COPRO/MIPROv2/BootstrapFewShot), metric signature `(example, pred, trace=None)->float|bool`, `compile(student, trainset, valset)`, `save()` static artifact.
- https://pypi.org/project/dspy/ — DSPy 3.2.1 stable (2026-05), Python `>=3.10,<3.15`, `anthropic`/`optuna` extras.
- MEMORY: "Helix tool docs drift" (docgen blank-import parity trap); "Helix bench smoke false-green" / Phase 87/89 CR-01 (vacuous-gate failure class).

### Secondary (MEDIUM confidence)
- https://dspy.ai/getting-started/gepa-optimization/ , /api/optimizers/MIPROv2/ , /cheatsheet/ — GEPA/MIPROv2 usage, program save/load.
- https://www.morphllm.com/gepa-prompt-optimization — GEPA reflective prompt-evolution, fewer rollouts than MIPROv2.
- https://dspy.ai/deep-dive/optimizers/miprov2/ + DeepWiki — minibatch auto-enabled only when `val_size > 50` (overfit-on-small-sets warning).

### Tertiary (LOW confidence)
- https://arxiv.org/pdf/2506.09501 — even temp-0 greedy decoding is not bit-reproducible (FP/GPU nondeterminism); grounds the "gate the artifact, not the optimizer" decision.

---
*Research completed: 2026-06-23*
*Ready for roadmap: yes*
