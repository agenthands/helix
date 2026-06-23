# Feature Research

**Domain:** Agent-facing tool-selection skill docs (decision matrices / steering) + DSPy offline prompt optimization
**Researched:** 2026-06-23
**Confidence:** HIGH (skill-doc patterns + existing-surface dependencies); HIGH (DSPy mechanics, Context7-verified `/websites/dspy_ai`)

Scope: the FOUR new v2.2 features only. Features 1–3 are deterministic skill-surface rewrites with hard `--check`/test gates; feature 4 is an exploratory, dev-time-only DSPy harness. Existing surface (50-verb CLI, `SKILL.md`+`reference.md`, `cmd/helix-refgen` with `--check` drift gate + `reference ⊇ VerbToolNames()` contract, Phase 101 `test/oracle/adopt` scorecard with `choice_rate`/`fallback_rate`) is a fixed dependency, NOT re-researched.

---

## Feature Landscape

### Table Stakes (A reliable tool-selection skill is expected to have these)

What makes an LLM coding agent reliably pick the right tool. These are the structural properties the maintainer analysis (SKILL-ISSUE.md) already identified as broken, and they match what "good" steering docs do.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **One-question-per-row matrix** (QUERY rows separate from ACTION rows) | An agent maps a single user intent → one tool; rows that bundle a query (`read-memory`) with a mutation (`write-memory`/`delete-memory`) force disambiguation the agent often gets wrong. SKILL-ISSUE.md flags 5 mixed rows (68/70/71/72/73). | LOW | Pure SKILL.md table edit. Splits 5 rows → 12; matrix 37→44 rows. Frontmatter description still under the 1,536-char idle-cost cap (SKILL-ISSUE §Impact). |
| **Explicit "use this NOT that" on every row** | The whole adoption thesis is steering the agent AWAY from grep/sed/cat/Read toward `helix <verb>`. A blank "Not this" cell (rows 68/69/71/72/73/75 carry `—`) removes the steering signal for exactly the verbs least familiar to agents (semantic-graph, memory, workflow). | LOW | Fill every `—` with the concrete fallback it displaces (e.g. `manual indexing`, `grep notes`, `mental math`). The product CLAUDE.md routing table is the canonical "Not this" source. |
| **Capability grouping that matches subsystem reality** | Agents pattern-match on section headers; conflating repomap (`get-repo-map`/`get-context`) with semantic-graph (`get-cluster-map`/`explain-cluster`/`index-semantic-graph`) tools makes the agent reach for a cold/unbuilt subsystem. | LOW | Regroup repomap vs semantic-graph vs memory vs workflow vs session. Footer already claims capability grouping; this makes the claim true. |
| **Precondition / prerequisite annotations** | Indexed-graph verbs (`explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`, `get-semantic-graph-status`) silently fail or degrade if `index-semantic-graph` hasn't run. Without a prereq note the agent picks the verb, gets a degraded/empty result, then falls back to grep — the exact anti-outcome the adoption metric punishes. | LOW–MED | Add a "Prerequisites" note/column for the 7 indexed-graph verbs. Wording must match real degraded-mode behavior (verify against `get-health` semantic_index block + `get-semantic-graph-status`). |
| **Per-verb accuracy in the generated reference** ("use this/not that" + "Output" lines correct per verb) | `reference.md` is the agent's deep-reference; 13 verbs share a copy-pasted "durable project/session memory" blurb and 22 share a copy-pasted FTS5 "Output" line — actively wrong for DELETE/EDIT/mode/budget/onboard verbs. Wrong reference text mis-routes and erodes trust. | MED | MUST be a generator fix in `cmd/helix-refgen/render.go` + per-verb template data, NOT a hand-edit — the Phase 97 `--check` drift gate would revert any hand-edit and the `reference ⊇ VerbToolNames()` contract must still hold. |
| **Token economy / idle-cost discipline** | A skill is loaded into every agent session; bloat is a permanent tax and dilutes the signal. Good steering docs stay terse and front-load the matrix. | LOW | The rewrite ADDS ~7 rows + prereq notes; keep frontmatter under cap, keep `reference.md` deltas minimal (SKILL-ISSUE estimates "minor"). Token budget is a constraint, not a feature to build. |
| **`installSkill` bundle allowlist + bundle-contents test** | Users expect the installed skill to be exactly {SKILL.md, reference.md}; a glob/`embed.FS` walk lets a stray scratch file (e.g. `SKILL-ISSUE.md` itself lives in that dir) leak into the binary and the on-disk installed skill. Predictable, minimal bundle is table-stakes hygiene. | LOW–MED | Tighten `installSkill` to an explicit allowlist; add a test asserting bundle contents == the allowlist. Depends on the current `embed.FS` skill-bundle + `helix setup` install path. |
| **Reproducible, gated output** (for any generated/tuned artifact) | Whatever produces `reference.md`/`SKILL.md` must round-trip through `helix-refgen --check` so CI can prove the committed file matches the generator. Non-reproducible generation = perpetual drift. | LOW (already exists for refgen) | The `--check` gate is the contract every feature must respect; for feature 4 it's the acceptance boundary (DSPy output is committed, then `--check` must pass). |

### Differentiators (Set this skill surface apart / close the measurement→optimization loop)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Adoption-metric-driven content** (rewrite validated against `choice_rate`/`fallback_rate`, not just maintainer taste) | Most skill docs are tuned by vibes. Helix already has a hermetic scorecard (`test/oracle/adopt`) + a live LLM leg; gating the rewrite on a measured choice-rate result turns "did we write a better matrix?" into a number. | MED | Feature 1–3 can be regression-checked against the scorecard; the live leg needs API keys so keep the hermetic fixture-based proof as the merge gate (it RUNS in default `go test ./...`, no network). |
| **DSPy offline prompt-tuning harness** (exploratory) | Auto-optimizes the decision-matrix / nudge text to MAXIMIZE `choice_rate` instead of hand-iterating. DSPy's optimizers (COPRO for instruction-only, MIPROv2 for joint instruction+demo, BootstrapFewShot for demos) are exactly built for "maximize a metric over a prompt." | HIGH | **Exploratory, dev-time/offline only.** Python lives outside the Go binary; output is a committed `SKILL.md`/`reference.md` snippet gated by `helix-refgen --check`. See "DSPy mechanics" below. Treat as a spike, not a guaranteed-ship feature. |
| **Metric = the existing adoption scorecard** (reuse, don't reinvent) | The DSPy `metric(example, pred, trace=None) -> float` can wrap the same first-command classifier as `test/oracle/adopt` (choice=1.0 when the first emitted command is a `helix` verb, fallback when it's grep/sed/cat/Read). One metric definition, two consumers (Go gate + Python optimizer). | MED | Closes the v2.1 measurement → v2.2 optimization loop explicitly (PROJECT.md key-context). The classifier logic is small and already specified (FirstCommand prefix match, never `strings.Contains`). |
| **Committed, `--check`-reproducible optimizer output** | DSPy normally lives at runtime; here the WIN is that the optimizer's chosen instructions/demos are frozen into the committed skill text and then enforced by `--check`, so the shipped binary stays Python-free and deterministic. | MED | The differentiator is the *discipline*: optimize offline → extract text → commit → gate. Not "ship DSPy." |

### Anti-Features (Tempting here, but wrong for THIS milestone)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Ship DSPy / Python as a runtime dependency** | "The optimizer is so useful, let agents run it live." | Breaks the core product invariant — Helix is a single Go binary, no Python/Docker/runtime deps. Adds an interpreter, model API calls, and non-determinism to a tool whose value is terse determinism. | Keep DSPy strictly dev-time/offline; commit only its text output; gate with `helix-refgen --check`. |
| **Hand-edit `reference.md` to fix the copy-paste/Output errors** | Fastest way to make the text correct. | The Phase 97 `--check` drift gate reverts hand-edits, and the `reference ⊇ VerbToolNames()` contract assumes generator authorship. Hand-edits create perpetual drift and a red CI. | Fix `cmd/helix-refgen/render.go` + per-verb template data; regenerate; commit generated output. |
| **Let the DSPy optimizer rewrite the matrix unattended into the committed file** | "Automate the whole rewrite." | DSPy can hill-climb the metric by producing text that games the classifier (e.g. stuffing the literal word `helix`) — the scorecard already guards against this (first-command prefix, never substring), but unattended commit removes human review of correctness/token cost and risks over-fitting the small fixture set. | Human-in-the-loop: optimizer PROPOSES, maintainer reviews against correctness + token budget, then commits; `--check` enforces. |
| **Expand the matrix to a per-verb mega-table (all 50 verbs, full prose each)** | "More guidance is better." | Idle-cost tax on every session + signal dilution; `reference.md` already serves the deep per-verb role. The matrix's job is fast routing, not exhaustive docs. | Keep the matrix a routing table (~44 rows); push depth to the generated `reference.md`. |
| **Glob-install the whole skills dir** (status quo) | Simpler install code. | Leaks stray files (e.g. `SKILL-ISSUE.md`) into binary/installed skill; non-deterministic bundle. | Explicit allowlist + bundle-contents test (table-stakes feature above). |
| **A second, separate "adoption metric" for the DSPy harness** | "The optimizer needs its own scoring." | Two metrics drift apart; the optimizer would maximize something CI doesn't gate. | Reuse the `test/oracle/adopt` classifier as the single source of truth for both the Go gate and the Python metric. |
| **Train/optimize on the live LLM leg in CI** | "Optimize against the real model." | Needs API keys + network, non-hermetic, slow, costs money per run, non-reproducible — incompatible with the default `go test ./...` gate. | Optimize offline against a fixed transcript fixture set; keep the hermetic scorecard as the merge gate. |

## Feature Dependencies

```
[1 SKILL.md matrix rewrite]
    └──validated-against──> [Phase 101 adopt scorecard] (existing)
    └──source-of-truth────> [SKILL-ISSUE.md proposed changes] (existing)

[2 reference.md generator fixes]
    └──requires──> [cmd/helix-refgen render.go + per-verb template data] (existing)
    └──must-preserve──> [Phase 97 --check drift gate]
    └──must-preserve──> [reference ⊇ VerbToolNames() contract]

[3 installSkill allowlist + bundle test]
    └──requires──> [embed.FS skill bundle + helix setup install path] (existing)
    └──independent-of──> [1], [2]   (can ship in parallel)

[4 DSPy offline harness] (EXPLORATORY)
    └──reuses-metric──> [test/oracle/adopt first-command classifier] (existing)
    └──optimizes──────> text of [1] and/or the PreToolUse nudge
    └──output-gated-by─> [2's helix-refgen --check]   (committed, reproducible)
    └──needs──> transcript train/dev fixture set (NEW; seed from test/oracle/adopt/testdata/transcripts)
```

### Dependency Notes

- **[2] requires the generator, not the file:** every correctness fix to `reference.md` must land in `cmd/helix-refgen/render.go` + structured per-verb data so `--check` passes. This is the hardest constraint in the milestone and the line between "fix" and "regression."
- **[1] and [3] are independent and low-risk:** the matrix rewrite is a hand-authored Markdown edit; the allowlist is install-path hardening. Either can ship first.
- **[4] depends on [2]'s gate as its acceptance boundary:** DSPy proposes text → maintainer commits → `helix-refgen --check` (and the matrix's own form) must still pass. [4] also depends on a NEW train/dev transcript corpus; the existing `test/oracle/adopt/testdata/transcripts` is the natural seed.
- **All four depend on the adoption scorecard as the measurement substrate** — the closed measurement→optimization loop is the milestone's thesis.

## MVP Definition

### Launch With (the deterministic core — features 1–3)

- [ ] **SKILL.md decision-matrix rewrite** — split QUERY/ACTION rows, fill every "Not this", regroup by subsystem, add indexed-graph prereq notes. (Essential: directly fixes the documented mis-routing.)
- [ ] **reference.md generator fixes** — correct the 13 "use this/not that" + 22 "Output" copy-paste errors in `helix-refgen`, regenerate, keep `--check` + `⊇ VerbToolNames()` green. (Essential: the per-verb deep reference is currently wrong.)
- [ ] **installSkill allowlist + bundle-contents test** — explicit {SKILL.md, reference.md} allowlist, test asserting no stray files. (Essential, low-cost hygiene; stops `SKILL-ISSUE.md`-class leaks.)

### Add After Validation (prove the rewrite moved the metric)

- [ ] **Adoption-scorecard regression check on the rewrite** — assert the rewritten matrix does not regress `choice_rate` on the hermetic fixture set; ideally show an improvement. (Trigger: features 1–2 merged.)

### Future Consideration (exploratory — feature 4)

- [ ] **DSPy offline prompt-tuning harness** — spike a COPRO/MIPROv2 program whose metric wraps the adopt classifier, optimize the matrix/nudge text offline, commit the chosen text, gate with `--check`. (Defer: highest complexity, exploratory, must not touch the Go binary; value is unproven until the deterministic rewrite establishes a baseline `choice_rate`.)

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| SKILL.md matrix rewrite | HIGH | LOW | P1 |
| reference.md generator fixes | HIGH | MEDIUM | P1 |
| installSkill allowlist + test | MEDIUM | LOW | P1 |
| Adoption-scorecard regression check on rewrite | HIGH | MEDIUM | P2 |
| DSPy offline tuning harness (exploratory) | MEDIUM | HIGH | P3 |

**Priority key:** P1 must-have for the milestone; P2 add when the rewrite lands; P3 exploratory spike, may not ship.

## DSPy Mechanics (Context7-verified — feature 4 grounding)

Source: Context7 `/websites/dspy_ai` (High reputation), 2026-06-23. These are the accurate primitives for the harness.

**Program & Signature.** A DSPy program is a `dspy.Module` composed of predictors; each predictor is typed by a `dspy.Signature` (input/output fields, e.g. `task -> command`). For this milestone the "program" is the decision-matrix/nudge text that produces a tool choice from a task description.

**Metric.** A metric is a plain Python callable:
`def metric(example, pred, trace=None) -> float | bool` — compares a gold `dspy.Example` to a `dspy.Prediction`, returns a score (higher = better). Docs example: `example.answer.lower() == pred.answer.lower()`. For Helix the metric wraps the `test/oracle/adopt` first-command classifier: return `1.0` when the first emitted command is a `helix` verb, `0.0` on a grep/sed/cat/Read fallback. Maximizing the mean metric == maximizing `choice_rate`. (GEPA additionally accepts/returns textual feedback `ScoreWithFeedback`; not required for a basic choice-rate objective.)

**Train/dev sets.** Lists of `dspy.Example(task=..., expected=...).with_inputs("task")`. `.with_inputs(...)` marks which fields are inputs vs labels. Natural seed corpus: `test/oracle/adopt/testdata/transcripts`.

**Optimizers (`dspy.teleprompt`), all driven via `optimizer.compile(student=program, trainset=..., valset=...)`:**

| Optimizer | What it tunes | Best when | Key knobs |
|-----------|---------------|-----------|-----------|
| **COPRO** | Instruction text only (generates candidate instructions, scores, keeps best) | Prompt *wording* is the bottleneck — matches "rewrite the steering text" | `prompt_model`, `metric`, `breadth=10`, `depth=3`, `init_temperature=1.4` |
| **MIPROv2** | Joint instruction + few-shot demos (Bayesian search) | Both wording and exemplars matter — SOTA general choice | `metric`, `auto="light"\|"medium"\|"heavy"`, `max_bootstrapped_demos`, `max_labeled_demos`, `prompt_model`/`task_model` |
| **BootstrapFewShot** | Few-shot demos (iteratively bootstraps demonstrations) | You want exemplars, not instruction rewrites | `metric`, `max_bootstrapped_demos=4`, `max_labeled_demos=16`, `max_rounds` |
| **BootstrapFewShotWithRandomSearch** | Demos + random search over candidate programs | Larger demo search; **requires a valset** | `metric`, `max_bootstrapped_demos`, `num_candidate_programs`, `num_threads` |

**Recommended starting point for THIS milestone:** **COPRO** — the objective is to optimize *instruction/steering wording* (the matrix + nudge), which is exactly COPRO's instruction-only target, and it avoids few-shot demo injection that would bloat the committed skill text. MIPROv2 (`auto="light"`) is the upgrade path if demos prove valuable. After `compile()`, extract the chosen instruction text, hand-review for correctness + token cost, commit into `SKILL.md`/template data, and let `helix-refgen --check` enforce reproducibility. Anti-vacuity guard already in place: the adopt classifier keys on the FIRST command prefix (never `strings.Contains`), so the optimizer cannot game the score by stuffing the literal word `helix` into prose.

## Competitor Feature Analysis

| Feature | Aider (lineage influence) | Generic agent skill docs | Our Approach |
|---------|---------------------------|--------------------------|--------------|
| Tool steering | Edit-format prompts + repo-map; implicit routing | Free-form "you can use X" prose | Explicit per-row "use this NOT that" routing table + deep generated reference |
| Measurement | Polyglot benchmark (correctness), not tool-choice | Usually none | Hermetic `choice_rate`/`fallback_rate` scorecard gating the rewrite |
| Prompt tuning | Manual iteration | Manual | Offline DSPy COPRO/MIPROv2 against the scorecard; committed + `--check`-gated output |

## Sources

- `internal/cli/skills/helix/SKILL-ISSUE.md` — maintainer analysis: 37 rows/50 verbs, mixed query/action rows, missing "Not this", reference.md copy-paste + Output errors, missing indexed-graph prereqs, token-impact (HIGH confidence, primary).
- `.planning/PROJECT.md` §"Current Milestone: v2.2" — the four target features, constraints (no breaking changes, Go single binary, DSPy dev-time only, scorecard as metric) (HIGH).
- `test/oracle/adopt/scorecard.go` — existing `choice_rate`/`fallback_rate` scorecard, first-command prefix classifier, anti-vacuity floors (`MinTasks=5`, `MaterialDrop=0.4`) (HIGH).
- Context7 `/websites/dspy_ai` — DSPy optimizers (COPRO/MIPROv2/BootstrapFewShot/BootstrapFewShotWithRandomSearch), metric signature `(example, pred, trace=None)->float|bool`, `dspy.Example.with_inputs`, `compile(student, trainset, valset)` (HIGH, verified 2026-06-23).
- CLAUDE.md "Helix CLI tool routing" table — canonical "Not this" fallback source (grep/sed/cat/Read mappings) (HIGH).

---
*Feature research for: agent tool-selection skill docs + DSPy offline prompt tuning (Helix v2.2)*
*Researched: 2026-06-23*
