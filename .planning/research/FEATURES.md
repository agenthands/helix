# Feature Research

**Domain:** Coding-agent adoption layer (skill/reference + steering + multi-agent) and Aider-derived benchmark validation for a CLI-first code-intelligence tool
**Researched:** 2026-06-22
**Confidence:** HIGH (existing Helix surface read from source; Aider mechanics confirmed against aider.chat docs; Claude Code hook semantics confirmed against current docs)

## Scope Note — What Already Exists (do NOT rebuild)

Read from source before scoping the gap:

- **`internal/cli/skills/helix/SKILL.md`** — terse v2.0 skill: YAML frontmatter (599-byte idle description, `allowed-tools: Bash(helix *)`) + a one-line-per-verb **decision matrix** ("use X not Y") + a pointer to `helix get-tool-help`. It is NOT a per-verb reference (no args, no output shape, no worked examples). **Per-verb help already exists at runtime via `helix get-tool-help`** (`internal/kernel/help/`), pulled from the tool registry.
- **`internal/cli/nudge.go`** — `PreToolUse` advisory nudge. Fires on EVERY grep/read/Bash-on-code call (no count threshold), classifies the Bash file operand (code vs prose/log/config via static ext allowlists `classifyBashTarget`), maps to the closest `helix` verb, and emits `additionalContext` with **exit 0 (advisory only, never blocks)**. Has a symbolic-tool reset list. No SessionStart priming, no deny path.
- **`internal/cli/setup_clients.go`** — 7 client registrars. **Only `claude-code` writes the skill + hooks.** vscode / jetbrains / gemini-cli / opencode / generic are **MCP-teardown-only** (they remove a stale Helix MCP registration; they install no skill, no reference, no steering).
- **`bench/datasets/aider-polyglot/`** (v1.12 Phase 85) — a **dataset-loader-only** adapter: pinned-sha shallow clone (`clone.go`), `.meta/config.json` mapping + `flagNonHermetic` (`loader.go`), the upstream **2-attempt + stderr-reprompt** protocol (`RunExercise`, `tries=2`, `timeout=180s`), native per-language test argv (`nativeTestCommand`), and the **anti-tamper pristine-test restore** (`restorePristineTests`, WR-01). It carries the `language` provenance field onto result.v2 and is HELIX_BIN/network-gated. **It does NOT exercise Helix EDIT verbs** — the `AgentFn` is whatever drives the edit; the adapter is verb-agnostic. It clones from upstream at runtime (it does NOT vendor fixtures into the tree).
- **`bench/` stack** (v1.12) — result.v2 schema, aggregator (BCa/pass@k/cost rollup), container runtime, contamination canary, `cmd/helix-bench`, fairness contract. **No RepoMap-quality eval and no fuzzy/edit-format robustness bench exist.**
- **`internal/fuzzy/`** — 4-strategy cascade (exact `lines.go`, whitespace-normalized, indentation-flexible `indent.go`, ellipsis-placeholder `ellipsis.go`) with ambiguity refusal + `diff.go`. Well unit-tested; **never benchmarked against an LLM-drift corpus.**
- **`test/oracle/llm/` + `test/oracle/judge/`** (v1.4, build tags `llm`/`llmjudge`) — tool-selection / disambiguation / output-interpretation behavioral tests with judge scoring, multi-provider (Anthropic + DeepSeek), never blocks merge.

**Net gap for v2.1:** (1) a comprehensive per-verb *reference* layer above the existing terse skill; (2) a *deterministic adoption contract* + an LLM-behavioral adoption score; (3) per-agent skill/reference surfaces for the 4 non-Claude clients; (4) the aider EDIT-verb mapping, a RepoMap-quality eval, a fuzzy-robustness bench, **vendored** fixtures, and a **committed baseline** — none of which the existing loader-only adapter covers.

## Feature Landscape

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Comprehensive per-verb reference** (Skill/Reference) | A "use X not Y" matrix tells an agent *which* verb but not *how* to call it; agents need synopsis + args + output shape + 1 worked example per verb to invoke correctly first try | MEDIUM | Generate, don't hand-write — derive from the tool registry (same source as `get-tool-help`) so it cannot drift. Covers all 50 verbs. Lives as a **linked reference file** (progressive disclosure: terse SKILL.md body stays idle-cheap; reference loaded on demand). Dep: `cmd/docgen` + tool registry; reuse `get-tool-help`. |
| **Progressive disclosure contract** (Skill/Reference) | Idle context cost must stay ~zero (the v2.0 win); a fat always-loaded reference would regress the 599-byte idle cost | LOW | Three tiers already implied: frontmatter description (always) → SKILL.md body decision-matrix (on trigger) → per-verb reference + `get-tool-help` (on demand). Formalize + assert the tier boundary. |
| **Deterministic adoption contract test** (Adoption-eval) | CI must gate that the skill/reference stays complete (every frozen verb present, every "not-this" mapping valid) and that the nudge actually fires on grep/read/sed/cat shapes | LOW–MEDIUM | Pure Go, no LLM, no API key — **blocks merge**. Extends existing `nudge_test.go` + a new "skill/reference ⊇ all 50 verbs" drift test. This is the "content/contract assertion + nudge-fires" layer. |
| **Multi-agent reference doc + per-agent instruction file** (Multi-agent) | Codex/Gemini/IDE/generic agents are first-class targets per PROJECT.md; today they get teardown-only | MEDIUM | **Minimum viable = one shared verb-reference doc + a thin per-agent instruction/config file** pointing the agent at the `helix` verbs (e.g. `AGENTS.md` for Codex, `GEMINI.md` for Gemini CLI, generic README snippet). NOT a full per-agent skill engine. Dep: `setup_clients.go` registrars (flip teardown-only → also-install). |
| **Vendored Aider fixtures with attribution** (Aider-edit-bench) | PROJECT.md explicitly requires copying Aider's exercism + edit-format fixtures into the tree (Apache-2.0) with license headers — reinforces lineage; removes the runtime-clone network dependency | LOW–MEDIUM | Existing adapter **clones at runtime**; v2.1 wants **vendored** (committed). Reuse the loader's `.meta/config.json` mapping + 2-attempt protocol unchanged; swap source clone → vendored tree. Add attribution headers (precedent: `LICENSE-AUDIT.md` + `make verify-licenses`). |
| **Polyglot edit benchmark wired to Helix EDIT verbs** (Aider-edit-bench) | The point is to prove the *toolset* works, not just that some agent solves exercism. The `AgentFn` must drive `replace-symbol-body`/`fuzzy-edit`/`replace-in-file`/`insert-*` to apply the model's diff | MEDIUM | **The real gap** — the v1.12 loader is verb-agnostic. v2.1 supplies an `AgentFn` routing the model's edit through Helix verbs; scores pass/fail per exercise (exit-0 test = pass, `tries=2`). Reuse `RunExercise` verbatim; do NOT touch the WR-01 anti-tamper restore. |
| **Committed baseline results artifact** (all bench surfaces) | PROJECT.md + the v1.9 local-only-bench rule: run `HELIX_BIN`-gated, commit a baseline (BENCH-RESULTS / benchstat) so regressions are visible | LOW | Reuse result.v2 + aggregator. Load-bearing risk: the **false-green** — bench smoke SKIPs without `HELIX_BIN` (MEMORY: helix-bench-smoke-false-green). The harness MUST fail/refuse rather than silently pass. |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Two-layer adoption eval** (Adoption-eval) | Cleanly separates "the contract is intact" (deterministic, blocks merge) from "a real model picks helix over grep/sed/cat" (LLM-behavioral, informational). No competitor grades CLI-tool adoption | MEDIUM–HIGH | Deterministic = content/contract assertions + nudge-fires tests. Behavioral = reuse v1.4 `llm`/`llmjudge` with a NEW rubric: **choice rate** (% of code questions answered with a helix verb), **fallback rate** (% that fell back to grep/sed/cat/Read), optionally **task success with vs without helix**. Never blocks merge (v1.4 precedent). |
| **Stronger steering with anti-fallback guardrails** (Steering) | Today's nudge is per-call advisory only; broadening code-target detection + SessionStart priming raises the adoption floor before the model reaches for grep | MEDIUM | SessionStart priming = inject the decision-matrix once at session start (cheap, high-leverage). Broaden `classifyBashTarget` (more shapes: `awk`, `head`/`tail`, pipelines). **Keep advisory exit-0** as default; deny/block is an anti-feature (below). |
| **RepoMap-quality eval** (RepoMap-eval) | Aider's RepoMap is a lineage influence; Helix already ships its own PageRank + token-budget binary-search RepoMap. An eval measuring *ranking correctness* (does the gold-relevant symbol rank highly?) and *token-budget fit* (map ≤ budget, no mid-symbol truncation) proves the kernel, not just the agent | MEDIUM–HIGH | Maps to `get-repo-map`/`get-context`. Metrics: top-k recall of a hand-labeled "relevant symbols for task T" gold set; rank-correlation; budget-adherence + no-mid-symbol-truncation invariant. Helix already does the binary-search fit (`internal/repomap`); the eval *measures* it — NOT a reimplementation of aider's repomap. |
| **Fuzzy/edit-format robustness bench** (Fuzzy-robustness-bench) | Aider's edit application tolerates specific LLM drift (whitespace, indentation, partial/ellipsis edits, slightly-wrong context). Helix's 4-strategy cascade claims the same; a drift corpus proves which strategy bites and the refusal-on-ambiguity behavior | MEDIUM | Corpus of (original, drifted-edit, expected-result) triples across the 4 strategies. Asserts: correct strategy selected + reported; ambiguous matches **refused** (not silently applied). Maps to `fuzzy-edit`/`replace-in-file`/`replace-symbol-body`. Reuse `internal/fuzzy` test fixtures as a seed corpus. |
| **Edit-format-applied-correctly signal** (Aider-edit-bench) | Aider reports "percent using correct edit format" distinct from "percent completed correctly". The Helix analog: did the model's intended edit get *applied by a helix verb* (vs the verb refusing/erroring)? Isolates tool-mechanics failures from reasoning failures | MEDIUM | Additive result.v2 field (precedent: `embedder_id`, `ablation_status`, `language` all additive open keys — no schema v3 bump). Distinguishes "fuzzy refused / verb errored" from "tests failed because the code was wrong". |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Deny/block PreToolUse hook (exit 2) for grep/sed/cat** | "Force" adoption — exit 2 deterministically blocks the call and feeds stderr back to the model | grep/sed/cat are legitimately correct for prose, logs, config, unknown-symbol discovery, build output (CLAUDE.md "when grep IS still correct"). Blocking breaks real workflows and trains the model to fight the tool. The existing nudge deliberately exits 0 (`nudge.go`: exit 2 makes CC treat it as a blocking error) | Keep **advisory exit-0** as default. Offer deny only as an explicit opt-in for a narrow positively-identified shape (e.g. `sed -i` on a code file), never the default. |
| **Per-agent bespoke skill engine** for Codex/Gemini/IDE/generic | "Full skill parity" across agents | Most non-Claude agents don't consume Claude-style Agent Skills; a bespoke skill runtime per agent is high cost / low return. Gemini/Codex read a markdown instruction file, not a skill | Shared verb-reference doc + thin per-agent instruction file (AGENTS.md / GEMINI.md / generic). Table-stakes, not a skill engine. |
| **Rebuild the aider polyglot adapter** | "Add the aider benchmark" reads like new work | The v1.12 loader-only adapter (clone + config-map + 2-attempt + anti-tamper) already exists and is correct. Rebuilding wastes effort and risks regressing the WR-01 anti-tamper invariant | **Scope v2.1 to the GAP only**: vendor the fixtures, supply the EDIT-verb `AgentFn`, add edit-format-applied scoring, commit a baseline. Reuse `RunExercise` verbatim. |
| **Reimplement Aider's RepoMap / PageRank in the eval** | "Match aider's repomap" | Helix already has its own PageRank + token-budget RepoMap (`internal/repomap`, v1.6+). The eval should *measure* Helix's RepoMap quality, not port aider's | Build a gold-labeled relevance set; measure top-k recall / budget-fit against `get-repo-map`/`get-context`. |
| **Make the LLM-behavioral adoption score a merge gate** | "Prove adoption in CI" | LLM nondeterminism flakes CI; v1.4 already decided LLM tests are informational/never-block | Deterministic contract layer gates merge; behavioral score is opt-in, informational, captured to a transcript. |
| **Live full 225-task polyglot run in CI** | "Run the whole benchmark" | Requires 6 toolchains + network + minutes-to-hours; the v1.12 adapter already records the live 225-task run as toolchain/network-gated | Hermetic fixture-set proof in CI (Phase 85 precedent: golden fixtures as sole authoritative proof); live run is local/`HELIX_BIN`-gated and produces the committed baseline. |

## Feature Dependencies

```
[Comprehensive per-verb reference]
    └──requires──> [tool registry / get-tool-help content]   (exists)
    └──enhances──> [terse SKILL.md decision matrix]           (exists; stays the idle-cheap tier)

[Deterministic adoption contract test]
    └──requires──> [Comprehensive per-verb reference]   (must exist to assert completeness)
    └──requires──> [nudge.go]                            (exists; extend nudge-fires assertions)

[LLM-behavioral adoption score]
    └──requires──> [v1.4 llm/llmjudge harness]           (exists)
    └──enhances──> [Deterministic adoption contract test] (two layers of one eval)

[Multi-agent reference + per-agent instruction file]
    └──requires──> [Comprehensive per-verb reference]    (shared doc is the substrate)
    └──requires──> [setup_clients.go registrars]         (exists; flip teardown-only → also-install)

[Polyglot edit benchmark wired to EDIT verbs]
    └──requires──> [aider-polyglot loader + RunExercise]  (exists, reuse verbatim)
    └──requires──> [Vendored Aider fixtures]              (new: clone → committed tree)
    └──requires──> [HELIX_BIN-gated runtime]              (exists; MUST fail-not-skip)
    └──produces──> [edit-format-applied-correctly signal] (additive result.v2 field)

[RepoMap-quality eval]    ──requires──> [get-repo-map / get-context + internal/repomap]  (exists)
[Fuzzy-robustness bench]  ──requires──> [internal/fuzzy 4-strategy cascade]              (exists)
[Committed baseline]      ──requires──> [all three bench surfaces + result.v2 + aggregator] (mostly exists)

[Deny/block hook] ──conflicts──> [legitimate grep/sed/cat use]  (anti-feature; keep advisory)
```

### Dependency Notes

- **Per-verb reference requires the tool registry, not new prose:** generating from the same source as `get-tool-help` guarantees the reference can't drift from the frozen 50-verb set and makes the deterministic completeness test trivial.
- **Deterministic contract test requires the reference first:** it asserts "every frozen verb appears with args + output-shape + example". Order: reference → contract test.
- **Multi-agent reuses the reference as the shared doc:** minimum viable per-agent surface = one shared verb doc + a thin per-agent pointer file; do not author the reference N times.
- **Polyglot edit bench reuses RunExercise verbatim** and only adds the EDIT-verb `AgentFn` + vendored fixtures + the applied-correctly field — the WR-01 anti-tamper pristine-test restore must NOT be touched.
- **Vendoring must precede the committed baseline run** if the baseline is to be reproducible offline.

## MVP Definition

### Launch With (v2.1 — Phase 97+)

Adoption layer:
- [ ] **Comprehensive per-verb reference** (generated from registry) — without it the skill tells *which* verb, not *how*.
- [ ] **Deterministic adoption contract test** (completeness + nudge-fires, blocks merge) — the measurable contract PROJECT.md demands.
- [ ] **Multi-agent shared reference + per-agent instruction file** for Codex/Gemini/IDE/generic — first-class per PROJECT.md.
- [ ] **Stronger steering** (broadened code-target detection + SessionStart priming, still advisory exit-0).

Aider validation:
- [ ] **Vendored Aider fixtures** (Apache-2.0 + attribution + license-audit gate).
- [ ] **Polyglot edit benchmark wired to Helix EDIT verbs** (reuse RunExercise; pass/fail per exercise; tries=2).
- [ ] **Committed baseline results artifact** (HELIX_BIN-gated, fail-not-skip).

### Add After Validation (v2.1 late / v2.2)

- [ ] **LLM-behavioral adoption score** (choice rate / fallback rate, opt-in, informational) — add once the deterministic contract is green and the rubric settles.
- [ ] **RepoMap-quality eval** (top-k recall + budget-fit) — needs a hand-labeled gold relevance set (the cost driver).
- [ ] **Fuzzy/edit-format robustness bench** (drift corpus across the 4 strategies) — high value, but the corpus must be curated.
- [ ] **Edit-format-applied-correctly signal** (additive result.v2 field) — pairs with the polyglot edit bench.

### Future Consideration (v2.2+)

- [ ] **Opt-in deny policy for narrow shapes** (e.g. `sed -i` on a code file) — only if advisory proves insufficient in the behavioral score.
- [ ] **Task-success-with-vs-without-helix A/B** in the behavioral harness — strongest adoption signal, most expensive to run.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Comprehensive per-verb reference | HIGH | MEDIUM | P1 |
| Deterministic adoption contract test | HIGH | LOW–MEDIUM | P1 |
| Multi-agent reference + per-agent file | HIGH | MEDIUM | P1 |
| Vendored Aider fixtures | MEDIUM | LOW–MEDIUM | P1 |
| Polyglot edit bench → EDIT verbs | HIGH | MEDIUM | P1 |
| Committed baseline artifact | HIGH | LOW | P1 |
| Stronger steering (priming + broader detect) | MEDIUM | MEDIUM | P2 |
| LLM-behavioral adoption score | HIGH | MEDIUM–HIGH | P2 |
| RepoMap-quality eval | MEDIUM | MEDIUM–HIGH | P2 |
| Fuzzy/edit-format robustness bench | MEDIUM | MEDIUM | P2 |
| Edit-format-applied-correctly signal | MEDIUM | MEDIUM | P2 |
| Opt-in deny policy | LOW | LOW | P3 |
| Task-success A/B | HIGH | HIGH | P3 |

**Priority key:** P1 = must have for the milestone; P2 = strong differentiator, add when the P1 substrate is green; P3 = future.

## Competitor / Prior-Art Feature Analysis

| Feature | Aider (lineage) | Playwright-CLI (v2.0 reference) | Helix v2.1 Approach |
|---------|-----------------|--------------------------------|---------------------|
| Agent reference | In-repo CONVENTIONS + edit-format prompts | SKILL.md alongside MCP | Generated per-verb reference + terse triggering SKILL.md (progressive disclosure) |
| Steering | Prompt-level edit-format instruction | None (relies on SKILL discovery) | PreToolUse advisory nudge (exit-0) + SessionStart priming |
| Edit benchmark | 225-exercise polyglot, pass/fail, tries=2, % correct-edit-format | n/a | Reuse aider's exercises+protocol; wire to Helix EDIT verbs; add applied-correctly field |
| RepoMap | tree-sitter tags → graph → PageRank → token-budget binary search | n/a | Helix has its own equivalent (`internal/repomap`); v2.1 *measures* its quality, doesn't port aider's |
| Edit-application robustness | diff/whole formats, tolerant application | n/a | 4-strategy fuzzy cascade with ambiguity refusal; v2.1 benches it against a drift corpus |

## Aider Mechanics (for downstream requirements precision)

- **Polyglot scoring:** 225 exercises from Exercism across C++(26)/Go(39)/Java(47)/JS(49)/Python(34)/Rust(30) — the hardest 225 of 697. Binary pass/fail per exercise = all unit tests pass. **Two attempts** (`tries=2`): attempt 1 from stub, attempt 2 after seeing capped error output. Two distinct headline metrics: **"percent completed correctly"** (solved) and **"percent using correct edit format"** (edit parsed/applied without malformation). The existing Helix loader already encodes `tries=2`, the 180s timeout, native per-language test argv, and the stderr-reprompt.
- **RepoMap construction:** tree-sitter extracts definition + reference tags → files are graph nodes, references are edges → a **PageRank** (personalized toward files in chat) ranks identifiers → the ranked tags are fit into a **token budget via binary search** over how many tags to render, eliding the rest. Helix's `internal/repomap` already implements this shape; the eval measures ranking correctness + budget fit, not the algorithm.
- **Edit-format drift tolerated:** aider applies whole-file and diff (search/replace) edits and tolerates models that mis-format slightly — the closest Helix analog is the 4-strategy fuzzy cascade (exact → whitespace-normalized → indentation-flexible → ellipsis-placeholder) with ambiguity refusal.

## Concrete Verb Mapping (for downstream requirements)

| Bench/eval surface | Helix verbs exercised | Existing component reused |
|---|---|---|
| Polyglot edit bench | `replace-symbol-body`, `fuzzy-edit`, `replace-in-file`, `insert-before-symbol`, `insert-after-symbol`, `create-file` | `bench/datasets/aider-polyglot/RunExercise` (verbatim), result.v2, aggregator |
| RepoMap-quality eval | `get-repo-map`, `get-context` | `internal/repomap` PageRank + token-budget renderer |
| Fuzzy-robustness bench | `fuzzy-edit`, `replace-in-file`, `replace-symbol-body` | `internal/fuzzy` 4-strategy cascade + ambiguity refusal |
| Steering / adoption | (none — steers TOWARD the above) | `internal/cli/nudge.go`, `setup_clients.go` |
| Per-verb reference | all 50 frozen verbs | tool registry, `get-tool-help` (`internal/kernel/help`), `cmd/docgen` |

## Sources

- Helix source (read directly): `internal/cli/skills/helix/SKILL.md`, `internal/cli/nudge.go`, `internal/cli/setup_clients.go`, `bench/datasets/aider-polyglot/{loader,clone}.go`, `bench/BENCH.md`, `internal/fuzzy/`, `.planning/PROJECT.md` (v2.1 milestone section)
- [Aider code-editing benchmark scoring (pass/fail, tries=2, edit formats)](https://aider.chat/docs/benchmarks.html)
- [Aider polyglot benchmark (225 exercises, 6 languages, Exercism, % correct edit format)](https://aider.chat/2024/12/21/polyglot.html)
- [Aider RepoMap overview (graph ranking, token budget)](https://aider.chat/docs/repomap.html)
- [Aider RepoMap technical construction (tree-sitter tags, PageRank, personalization, token-budget fit)](https://aider.chat/2023/10/22/repomap.html)
- [Claude Code hook control flow — additionalContext (exit 0, advisory) vs decision:block / exit 2 (deny)](https://stevekinney.com/courses/ai-development/claude-code-hook-control-flow)
- [Steering Claude Code: skills, hooks, rules, subagents (Anthropic)](https://claude.com/blog/steering-claude-code-skills-hooks-rules-subagents-and-more)

---
*Feature research for: coding-agent adoption layer + Aider-derived benchmark validation (Helix v2.1)*
*Researched: 2026-06-22*
