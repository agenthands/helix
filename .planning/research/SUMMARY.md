# Project Research Summary

**Project:** Helix v2.1 — Agent Adoption & Aider-Derived Validation
**Domain:** Coding-agent adoption layer (skill/reference + steering + multi-agent) and Aider-derived benchmark validation, added to a mature Go-native CLI-first code-intelligence platform
**Researched:** 2026-06-22
**Confidence:** HIGH

## Executive Summary

Helix v2.1 is a **subsequent milestone — additive integration, not greenfield.** Both of its thrusts attach to existing seams in a mature Go single-binary product and require **ZERO new Go dependencies.** Thrust 1 (Agent Adoption) hangs off the `internal/cli/` skill+nudge+setup surface and reuses `cmd/docgen`, `get_tool_help`, and the `test/oracle` LLM harness. Thrust 2 (Aider-Derived Validation) hangs off the `bench/` stack — specifically the v1.12 aider-polyglot loader (`RunExercise`), the `bench/runtime` cell spine, the `bench/evaluators/*` leaves, and the `bench/runners/<mode>` filesystem-table. The real v2.1 work is **file-format conventions, vendored data, and Go-native glue**, not library acquisition. The single structural code change in the whole milestone is switching `internal/cli/skill.go` from an embedded `string` to an `embed.FS` so the skill can ship `reference.md` alongside `SKILL.md`.

The recommended approach is **reuse-don't-fork, generate-don't-hand-write, measure-don't-port.** The per-verb reference MUST be generated from the frozen 50-verb tool registry (same source as `get_tool_help` and the README table) — never hand-authored — or it drifts. The polyglot edit benchmark MUST reuse `RunExercise` verbatim (the v2.1 delta is supplying an EDIT-verb `AgentFn`, vendoring fixtures, adding an additive `edit_format_applied` open key, and committing a baseline) — never rebuilt, or it regresses the WR-01 anti-tamper invariant. The RepoMap-quality and fuzzy-robustness evals **measure** Helix's existing `internal/repomap` and `internal/fuzzy` engines against curated corpora — they do NOT port aider's algorithm.

The dominant risk is **vacuity.** v1.12 shipped four named vacuous-pass CRITICALs (Phases 82/86/87/89) caught only by adversarial revert-and-fail tests, never by the happy-path test. The meta-lesson is binding: **for every gate v2.1 adds (adoption contract, HELIX_BIN guard, license gate, bench baselines), there must be a test that deliberately breaks the protected invariant and asserts the gate goes RED.** Two correction flags must also be ratified in requirements: (1) the milestone brief mis-states the fixture license — the Aider **polyglot fixtures are MIT** (Exercism redistribution, byte-verified in the in-tree `LICENSE-AUDIT.md`), NOT Apache-2.0; only the aider *tool repo* is Apache-2.0, and the recommendation is to NOT vendor tool-repo code so the fixture tree stays single-license MIT. (2) "Vendor vs clone" is inferred from "committed baseline" intent and should be confirmed.

## Key Findings

### Recommended Stack

**No new Go dependencies.** Every capability is served by modules already in `go.mod`: the v1.12 bench stack, the v1.4 `llm`/`llmjudge` harness, the v2.0 skill + nudge, and stdlib `embed`. The genuinely new artifacts are **file-format conventions and data**, not libraries. See [STACK.md](STACK.md).

**Core technologies (all reused):**
- `anthropic-sdk-go v1.35.0` — drives the opt-in LLM-behavioral adoption score + judge scorer (already a dep; DeepSeek multi-provider path exists)
- `modelcontextprotocol/go-sdk v1.5.0` — typed tool-registry args drive per-verb reference generation
- stdlib `embed` — the one structural change: `embeddedSkillMD string` -> `embed.FS` to bundle `reference.md` with `SKILL.md`
- `chromem-go` (via `bench/ragindex`) — optional embedding baseline to contrast against PageRank in the RepoMap eval
- `bench/aggregator` (hand-rolled BCa bootstrap + pass@k) — NOT `benchstat` (its CI gate was deliberately removed at v1.9; benches are local-only)

**Verified conventions (HIGH):** Claude Code skill 1,536-char idle cap + <500-line body + progressive disclosure; Codex `AGENTS.md` (32 KiB cap) + `PreToolUse` hook (`type:"command"`-only, same `additionalContext`/`permissionDecision` envelope as Claude); Gemini CLI `GEMINI.md` (NO PreToolUse-equivalent — context-file steering only).

### Expected Features

See [FEATURES.md](FEATURES.md). The net gap is narrow because the v1.12 bench stack and v2.0 skill already exist.

**Must have (table stakes / P1):**
- Comprehensive per-verb reference (generated from registry, progressive-disclosure tier)
- Deterministic adoption-contract test (completeness + nudge-fires; **blocks merge**)
- Multi-agent shared reference + per-agent instruction file (Codex/Gemini/IDE/generic — flip teardown-only -> also-install)
- Vendored Aider fixtures (MIT + attribution + license-audit gate)
- Polyglot edit benchmark wired to Helix EDIT verbs (reuse `RunExercise`; pass/fail; tries=2)
- Committed baseline results artifact (HELIX_BIN-gated, **fail-not-skip**)

**Should have (competitive / P2):**
- Two-layer adoption eval — deterministic contract (blocks) + LLM-behavioral choice/fallback rate (informational, never blocks)
- Stronger steering — broadened code-target detection + SessionStart priming, still advisory exit-0
- RepoMap-quality eval (top-k recall / MRR / nDCG + budget-fit) — needs hand-labeled gold set
- Fuzzy/edit-format robustness bench — drift corpus across the 4 strategies
- `edit_format_applied` additive result.v2 open key (no schema v3 bump)

**Defer / anti-features:**
- Deny/block PreToolUse hook (exit 2) — anti-feature; grep/sed/cat are legitimately correct for prose/logs/config/build output
- Per-agent bespoke skill engine — anti-feature; non-Claude agents read a markdown instruction file
- Rebuilding the polyglot adapter, reimplementing aider's RepoMap, or making the LLM score a merge gate — all anti-features
- Live full 225-task polyglot run in CI — local/HELIX_BIN-gated only

### Architecture Approach

v2.1 adds **NO new architectural layer.** Both thrusts are leaf additions and small edits against existing seams. The hard-dependency chains are: **reference -> contract-test**, **vendor -> baseline**, **output/corpus -> eval**. The two thrusts are independent and interleavable. See [ARCHITECTURE.md](ARCHITECTURE.md).

**Major components:**
1. `cmd/helix-refgen` (NEW) — generates `reference.md` from `skill.ToolProviders()` + `help.ExtractParamDocs`, with a `--check` drift gate; mirrors `cmd/docgen` and inherits its blank-import-parity-with-daemon rule
2. `internal/cli/skill.go` (MODIFIED) — `string` -> `embed.FS`; `installSkill` walks+copies the bundle (the milestone's only structural code change)
3. `internal/cli/setup_clients.go` + `setup_agents.go` (MODIFIED/NEW) — Claude path copies reference; Codex/Gemini/generic flip teardown-only -> also-write `AGENTS.md`/`GEMINI.md` (+ Codex `hooks.json` -> `helix nudge`)
4. EDIT-verb `AgentFn` in `bench/runtime` (NEW) — routes the model's edit through `replace-symbol-body`/`fuzzy-edit`/`replace-in-file`/`insert-*` against the warm daemon; plugs into `RunExercise`'s verb-agnostic seam (loader untouched, WR-01 preserved)
5. `bench/evaluators/repomapeval` + `fuzzyrobust` (NEW leaves) — stdlib-only (+ `editsim.ES`); measure `get-repo-map`/`get-context` and `internal/fuzzy`; respect the `vet-ablation-leakage` no-kernel-import boundary
6. `bench/runners/aider_edit/MODE.md` (NEW) — filesystem-table mode, zero resolver Go change

### Critical Pitfalls

See [PITFALLS.md](PITFALLS.md). Every item is anchored to a named v1.12 failure or an in-tree contract.

1. **Adoption eval is vacuous** — passes without proving an agent chose `helix`. Avoid: source the completeness verb list from `verbs_gen.go` (the authority), NOT the generator's own output; assert the *specific* suggested verb per shape; add a sabotaged-skill revert-and-fail self-test for the behavioral score; key detectors on the first emitted command line, not substring; reject empty-bucket-as-pass.
2. **HELIX_BIN false-green** — every new bench surface SKIPs silently instead of failing. Avoid: ship a hermetic golden sibling (no binary, no network) per surface as the sole authoritative proof; add a "did it RUN" sentinel when HELIX_BIN is set; fail-closed on missing `result.v2.json`/empty run dir/missing metric line.
3. **License vendoring trap** — the brief says Apache-2.0; the fixtures are **MIT**. Avoid: stamp `SPDX-License-Identifier: MIT` + per-track NOTICE; do NOT vendor Apache-2.0 tool-repo code (re-derive the drift corpus natively); vendor only the exercised subset via a manifest; extend `make verify-licenses` to the vendored tree with a tamper test.
4. **Non-reproducible committed baseline** — unseeded RNG, map-order leakage, machine-specific paths/timestamps/latency. Avoid: route through the existing deterministic `renderAll`; seed every resample; sort-before-emit; commit only deterministic quality metrics (latency -> local `bench-micro`); one renderer for `aggregate` and `report`.
5. **Self-confirming gold corpus** — labels snapshotted from the tool under test prove nothing. Avoid: author relevance from the task ground truth (e.g. exercism `files.solution`), independent of `get-repo-map`; add a reversed/random-ranker discriminator that MUST fail the corpus; size floor per strategy/language; derive fuzzy "expected strategy" from the drift type, not observed behavior.
6. **Steering over-reach** — over-firing the nudge on legitimate prose/log/config, or breaking the fail-open exit-0 contract. Avoid: keep advisory exit-0 (assert it); negative-control classifier rows (`grep TODO README.md` MUST NOT fire); SessionStart primes only the terse matrix with a SKILL-04-style size cap.

## Implications for Roadmap

Two interleavable thrusts; within each, order is fixed by hard dependencies. Phase numbering continues from the v2.0 close (~Phase 97+).

### Phase A1: embed.FS switch + helix-refgen + generated reference.md
**Rationale:** The substrate everything else asserts and installs; depends on nothing new.
**Delivers:** `cmd/helix-refgen` (with `--check`), generated `internal/cli/skills/helix/reference.md`, `skill.go` switched to `embed.FS`.
**Addresses:** Comprehensive per-verb reference (P1).
**Avoids:** Pitfall 8 (generate from registry, not by hand); Pitfall 1 (re-source completeness from `verbs_gen.go`).

### Phase A2: Deterministic adoption-contract test
**Rationale:** Reference must exist before completeness can be asserted; this is the merge-gating contract PROJECT.md demands.
**Delivers:** `reference superset-of VerbToolNames()` drift gate, nudge-fires golden table (per-shape correct verb), idle-cost cap, negative-control rows. **Blocks merge.**
**Avoids:** Pitfall 1 (non-tautological, revert-and-fail); Pitfall 6 (exit-0 + negative-control).

### Phase A3: Multi-agent instruction files + Codex hook + stronger steering
**Rationale:** Shared reference is the substrate; nudge envelope is portable to Codex.
**Delivers:** `setup_agents.go` writes sentinel-delimited `AGENTS.md`/`GEMINI.md` + Codex `hooks.json` (-> `helix nudge`); broadened `classifyBashTarget`; per-agent install goldens.
**Avoids:** Pitfall 7 (append not overwrite, idempotent, 32 KiB cap, no Gemini hook); Pitfall 6 (advisory exit-0).

### Phase B1: Vendor fixtures + MIT SPDX/NOTICE + extended verify-licenses
**Rationale:** Committed offline data must precede the baseline; reuses `pin.go`.
**Delivers:** vendored exercism subset under `bench/datasets/aider-polyglot/fixtures/`, per-track MIT NOTICE/SPDX, `VENDOR-MANIFEST.md`, extended hard-fail license gate + tamper test.
**Avoids:** Pitfall 3 (MIT not Apache-2.0; subset not all six tracks).

### Phase B2: EDIT-verb AgentFn + aider_edit mode + edit_format_applied key
**Rationale:** Wires `RunExercise` to helix verbs; depends on vendored fixtures.
**Delivers:** EDIT-verb `AgentFn` (daemon-dialing, in `bench/runtime`), `bench/runners/aider_edit/MODE.md`, additive `edit_format_applied *bool` open key.
**Avoids:** Pitfall 8 (reuse `RunExercise` verbatim, WR-01 untouched, no schema v3 bump).

### Phase B3: Committed polyglot baseline
**Rationale:** Must run the wired bench offline after vendoring + wiring.
**Delivers:** HELIX_BIN-gated local capture -> `bench/reports/<run>/BENCH-RESULTS.md` + `result.v2.json`.
**Avoids:** Pitfall 2 (fail-not-skip, hermetic golden sibling, bench-guard audit); Pitfall 4 (byte-reproducible, deterministic metrics only).

### Phase P2-1: LLM-behavioral adoption scorecard (opt-in)
**Rationale:** Add once the deterministic contract is green and the rubric settles.
**Delivers:** `test/oracle/llm` scorecard — choice rate / fallback rate, build-tag gated, never blocks merge.
**Avoids:** Pitfall 1 (sabotaged-skill revert-and-fail; negative judge exemplar; first-command-line detector).

### Phase P2-2: RepoMap-gold corpus -> repomapeval leaf -> baseline
**Rationale:** Corpus must be authored before the eval; corpus is the cost driver.
**Delivers:** `bench/datasets/repomap-gold/` + `bench/evaluators/repomapeval/` (recall@k/MRR/nDCG + budget-fit).
**Avoids:** Pitfall 5 (gold from task ground truth; reversed-ranker discriminator); Pitfall 2; Pitfall 4.

### Phase P2-3: Fuzzy-drift corpus -> fuzzyrobust leaf -> baseline
**Rationale:** Corpus before eval; reuses `editsim.ES`.
**Delivers:** native drift corpus + `bench/evaluators/fuzzyrobust/` asserting strategy selection + ambiguity refusal.
**Avoids:** Pitfall 5 (expected strategy from drift type; known-ambiguous-must-refuse case; size floor); Pitfall 2; Pitfall 4.

### Phase Ordering Rationale

- **Hard dependencies drive order within each thrust:** reference -> contract-test (can't assert completeness without the reference); vendor -> baseline (offline reproducibility); corpus -> eval (avoid self-confirming gold).
- **Thrusts T1 and T2 are independent** and can interleave or run in parallel waves; only intra-thrust order is fixed.
- **P2 work (LLM score, RepoMap eval, fuzzy bench) is gated behind the P1 substrate** because the curated gold/drift corpora are the milestone's main cost driver and labeling shouldn't block the milestone.
- **Cross-cutting rule for every bench phase:** the HELIX_BIN fail-not-skip guard and the `vet-ablation-leakage` leaf-import boundary are pre-existing gates the new code must satisfy in its exit criteria — not new work.

### Research Flags

Phases likely needing deeper research during planning (`/gsd-plan-phase --research-phase <N>`):
- **P2-2 (RepoMap-gold corpus / repomapeval):** the gold-relevance labeling methodology and anti-self-confirming discriminator design are the highest-uncertainty, highest-cost items; corpus size floor is TBD.
- **P2-3 (Fuzzy-drift corpus / fuzzyrobust):** mapping drift types -> expected strategy and authoring a discriminating ambiguity-refusal case need care.
- **P2-1 (LLM-behavioral scorecard):** rubric design (negative exemplar, sabotaged-skill self-test) and detector keying need a research pass to avoid vacuity.

Phases with standard patterns (skip research-phase — patterns verified in-tree):
- **A1/A2 (refgen + contract test):** `cmd/docgen` blank-import + `--check` + golden-test patterns are established.
- **B1/B2/B3 (vendor + AgentFn + baseline):** the v1.12 aider-polyglot loader, result.v2 additive-key pattern, filesystem-table mode, and license-gate pattern all have direct precedents.
- **A3 (multi-agent + steering):** Codex/Gemini conventions are HIGH-confidence; sentinel-append + idempotency + nudge-reuse are well-scoped.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | `go.mod` + `bench/` + `test/oracle/` inspected directly; agent-runtime conventions verified against official Claude Code / Codex / Gemini docs |
| Features | HIGH | Existing Helix surface read from source; Aider mechanics confirmed against aider.chat; hook semantics against current docs |
| Architecture | HIGH | Every integration point read from real in-tree source; this is an integration map, not a domain survey |
| Pitfalls | HIGH | Every pitfall anchored to a named shipped v1.12 vacuous-pass CRITICAL or a byte-verified in-tree contract |

**Overall confidence:** HIGH

### Gaps to Address

- **License correction (MIT != Apache-2.0):** the brief is factually wrong; the fixtures are MIT (in-tree byte-verified). **Flag for requirements to ratify** the MIT SPDX + per-track NOTICE decision and the "do not vendor Apache-2.0 tool code" recommendation. (Confidence on the fact is HIGH; the recommendation is MEDIUM.)
- **Vendor-vs-clone:** inferred from "committed baseline" + "vendor fixtures" intent (MEDIUM). Confirm in requirements that vendoring (not runtime clone) is wanted.
- **Corpus size floors:** the RepoMap-gold and fuzzy-drift corpus sizes (per language / per strategy) are the milestone's main cost driver and are TBD — set a defensible floor in requirements so empty/under-sized buckets can't pass vacuously.
- **Vendored exercise selection:** the deterministic subset must be recorded in `VENDOR-MANIFEST.md`; the exact exercise list is a planning-time decision.

## Sources

### Primary (HIGH confidence)
- In-tree source (direct read): `internal/cli/{skill,nudge,setup_clients}.go`, `internal/cli/verbs_gen.go`, `cmd/docgen/main.go`, `internal/kernel/help/help.go`, `bench/datasets/aider-polyglot/{clone,loader,pin}.go`, `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (MIT, byte-verified), `bench/runtime/{result,cell,mode_resolver}.go`, `bench/evaluators/editsim/editsim.go`, `bench/aggregator/`, `bench/BENCH.md`, `go.mod`, `Makefile`, `.planning/PROJECT.md` (v2.1 milestone + v1.12 four-CRITICAL progress log)
- [Claude Code — Skills](https://code.claude.com/docs/en/skills) — 1,536-char idle cap, <500-line body, progressive disclosure, bundled reference files, `Bash(helix *)` form
- [OpenAI Codex — AGENTS.md](https://developers.openai.com/codex/guides/agents-md) + [Hooks](https://developers.openai.com/codex/hooks) — 32 KiB cap, discovery order, `PreToolUse` `additionalContext`/`permissionDecision`, `type:"command"`-only
- [Gemini CLI — GEMINI.md](https://geminicli.com/docs/cli/gemini-md/) + [configuration](https://geminicli.com/docs/reference/configuration/) — context file location, `context.fileName`, no PreToolUse hook
- [Aider polyglot benchmark](https://aider.chat/2024/12/21/polyglot.html) + [scoring](https://aider.chat/docs/benchmarks.html) + [RepoMap](https://aider.chat/2023/10/22/repomap.html) — Exercism MIT redistribution, 225 exercises, tries=2, % correct edit format, tree-sitter PageRank token-budget
- Project memory: `helix-bench-smoke-false-green` (HELIX_BIN fail-not-skip), `helix-tool-docs-drift` (docgen blank-import == daemon)

### Secondary (MEDIUM confidence)
- "Vendor vs clone" recommendation — milestone intent inferred from "committed baseline" + "vendor fixtures"
- "Re-derive edit-format drift corpus natively over vendoring aider Apache-2.0 fixtures" — recommendation to keep the fixture tree single-license MIT; not a hard constraint

---
*Research completed: 2026-06-22*
*Ready for roadmap: yes*
