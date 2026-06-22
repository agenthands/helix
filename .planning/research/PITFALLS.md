# Pitfalls Research

**Domain:** Coding-agent adoption layer (skill/reference + steering + multi-agent) and Aider-derived benchmark validation, added to a mature Go-native CLI-first code-intelligence tool (Helix v2.1)
**Researched:** 2026-06-22
**Confidence:** HIGH — every pitfall below is anchored either to a *named, already-shipped* failure in this exact bench stack (the v1.12 phase log records four distinct vacuous-pass CRITICALs caught only by revert-and-fail) or to an in-tree contract verified by direct source read. Lower-confidence items are tagged inline.

> **Reading note for the roadmapper.** This milestone is *additive to a system that has already been bitten by every class of failure listed here.* The v1.12 progress log (PROJECT.md lines 153–177) is a confession of four separate "gate failed OPEN / vacuous pass" CRITICALs (Phases 82, 86, 87, 89), each caught only by an *adversarial revert-and-fail* test, not by the happy-path test. The single most important meta-lesson: **for every gate this milestone adds, there must be a test that deliberately breaks the thing the gate protects and asserts the gate goes RED.** A gate with only a green-path test is presumed broken until proven otherwise. The same discipline applies verbatim to the adoption eval and the HELIX_BIN guard.

---

## Critical Pitfalls

### Pitfall 1: Adoption eval is *vacuous* — it passes without ever proving an agent chose `helix`

**What goes wrong:**
The headline deliverable of Thrust 1 is "a measurement proving an agent actually picks `helix` over grep/sed/cat" (PROJECT.md line 191). The failure mode is an eval that returns green while proving nothing. Concretely, several independent ways this happens:
- **Deterministic-contract tautology:** the "skill/reference ⊇ all 50 verbs" completeness test reads the verb list *from the same generator that produced the reference* — so it can never fail (it compares a set to itself). Or the "nudge fires on grep/sed/cat" test asserts `output != ""` rather than asserting the output names the *correct* `helix` verb for that shape.
- **LLM-judge rubric that can't return < pass:** the judge prompt is "did the model use a good tool?" with no negative anchor, so the judge rubber-stamps grep as "reasonable." Or the score is computed but the threshold is `>= 0.0`.
- **Detector that matches on substring, not on the chosen command:** `mentionsHelix(output)` returns true because the *task prompt itself* (or the injected SKILL.md) contains the word "helix," not because the model emitted a `helix` verb. (This is the exact shape of the Phase 86 zero-value-config collapse, transplanted to text.)
- **Empty task bucket counts as a pass:** the behavioral scorer iterates an empty task set and reports `choice_rate = 1.0` (0/0 treated as success) — identical to the **Phase 87 CR-01** "empty test bucket counted as a pass" CRITICAL.

**Why it happens:**
Adoption is fuzzy to measure, so authors reach for the weakest assertion that turns the bar green. LLM nondeterminism pushes authors toward lenient rubrics to avoid flakes. And because the LLM layer "never blocks merge" (a correct, locked decision), there is no CI pressure forcing it to be discriminating.

**How to avoid:**
- **Deterministic contract — make it adversarially non-tautological:**
  - Source the verb list for the completeness assertion from `internal/cli/verbs_gen.go` (the frozen registry, the *authority*), NOT from the reference generator's own output. Compare `referenceCovers(verbsFromRegistry)`. A revert test that drops one verb from the reference MUST turn it RED.
  - The nudge-fires test must assert the **specific suggested verb** per input shape (`grep "func X"` → suggests `search-symbols`; `sed -i` on `.go` → suggests `replace-in-file`/`fuzzy-edit`), via a golden table, not non-emptiness.
  - Add a **negative-control row**: a Bash target that is legitimately prose/log/config (`grep TODO README.md`) MUST assert the nudge does **not** fire. A nudge that fires on everything is as useless as one that fires on nothing (see Pitfall 6).
- **LLM-behavioral score — build in a failing anchor:**
  - The scorecard MUST include a **revert-and-fail self-test**: run the scorer against a *deliberately sabotaged* skill body (decision matrix stripped) and assert `choice_rate` drops materially. If the score is identical with and without the skill, the eval measures nothing. This is the direct analog of the Phase 89 "revert-and-fail to prove the integrity fix is non-vacuous."
  - The detector must key on the **first emitted command line** (`firstCommandLine`, already in `test/oracle/llm`), not substring presence anywhere in the transcript.
  - Score both **choice rate** (% of code questions answered with a `helix` verb) AND **fallback rate** (% that fell to grep/sed/cat/Read) and assert they are complementary on a known fixture, so a detector that double-counts or silently drops a transcript is caught.
  - The judge rubric must have an explicit **negative exemplar** ("a response that runs `grep -r` to find a definition scores 0 on adoption") so the rubric can demonstrably return 0.

**Warning signs:**
- The completeness test passes after you delete a verb from the reference.
- The behavioral score is numerically identical whether or not the skill is installed.
- The nudge test asserts `!= ""` anywhere.
- `choice_rate` is reported on an empty or one-element task set.
- No test in the suite is *expected* to be RED on a sabotaged input.

**Phase to address:**
Deterministic contract → the **adoption-contract phase** (the first adoption-eval phase, ~Phase 99). Behavioral anti-vacuity self-test → the **LLM-behavioral score phase** (later, ~Phase 101+). Both phases' VERIFICATION must include an explicit revert-and-fail step.

---

### Pitfall 2: HELIX_BIN false-green — every new bench surface SKIPs silently instead of failing

**What goes wrong:**
The single most-repeated trap in this codebase. Bench smoke/cell tests `t.Skip()` when `HELIX_BIN` is unset, so a plain `go test ./...` reports **green while exercising none of the new bench code**. This has already bitten the project (project memory: *helix-bench-smoke-false-green*; the *no_semantic SIGKILL-vacuous-gate* finding). v2.1 adds **three** new bench surfaces (polyglot EDIT-verb runner, RepoMap-quality eval, fuzzy/edit-format robustness) — each is a fresh opportunity to re-introduce the skip-instead-of-fail hole, plus a new `HELIX_BENCH_*_BIN` for any second binary (cf. Phase 83's `HELIX_BENCH_RAG_BIN`).

**Why it happens:**
`t.Skip` is the idiomatic Go way to handle "can't run here," and CI genuinely lacks the built binary unless the workflow builds it first. The skip looks responsible; the false-green is invisible until someone checks coverage.

**How to avoid:**
- **The Phase-85 precedent is the law: a committed HERMETIC golden fixture is the SOLE authoritative proof, and the live/HELIX_BIN leg has a hermetic sibling.** Every new runner/evaluator must have a golden-fixture test that runs with NO binary and NO network — so `go test ./bench/...` exercises real parsing/scoring logic even on CI. The live leg may skip; its logic is already covered by the sibling. ("no skip-only-without-sibling" — BENCH.md.)
- **A dedicated "is the guard real?" gate:** add a test that, when `HELIX_BIN` *is* set, asserts the live test actually RAN (e.g. via a sentinel side-effect or a ran-marker) — the Phase 81 fix proved this exact thing (`TestNoSemanticReadsTotalLineEmitted` "proven to RUN (not SKIP) and PASS").
- **Fail-closed on malformed/missing artifacts:** if the live leg DOES run, a missing `result.v2.json`, an empty run dir, or a missing metric line must be a hard error — never read as a zero/pass. This is the **Phase 82 CR-01** ("N-gate failed OPEN on zero-discovery — empty run dir produced empty reports + exit 0") and the **Phase 81 WR-02** ("scraper read a missing line as count=0") lessons. Reuse their fail-closed scrape/assert pattern.
- **CI documents the gate, doesn't fake it:** the `bench.yml` PR job runs `make bench-quick` with the binary BUILT first (Phase 89 INFRA-04 precedent), so the hermetic path is genuinely exercised; the full live matrix stays nightly/maintainer-gated and local-only.

**Warning signs:**
- `go test ./bench/...` passes in seconds with no `HELIX_BIN` and you cannot point to a non-skipped test that touched the new code.
- A new runner has only a `if os.Getenv("HELIX_BIN")=="" { t.Skip }`-guarded test and no golden sibling.
- A scraper/parser treats a missing line/file/dir as a zero or a pass.

**Phase to address:**
Each of the three bench-surface phases (polyglot EDIT runner, RepoMap eval, fuzzy-robustness) must ship its hermetic golden sibling in the SAME phase. A cross-cutting **"bench guard audit"** belongs in the committed-baseline phase (verify no surface is skip-only).

---

### Pitfall 3: Fixture vendoring license trap — Apache-2.0 vs MIT, missing attribution, oversized tree

**What goes wrong:**
The milestone brief *literally states the wrong license*: PROJECT.md line 194 says "vendor Aider's fixtures … (Apache-2.0)." That is factually wrong and STACK.md flags it (lines 21–34): the **polyglot fixtures redistribute Exercism content and are MIT**, byte-verified in the existing `bench/datasets/aider-polyglot/LICENSE-AUDIT.md`. Apache-2.0 applies only to the *aider TOOL repo* (`Aider-AI/aider`). Failure modes:
- Stamping vendored MIT fixtures with `SPDX-License-Identifier: Apache-2.0` (or vice-versa) → an inaccurate, possibly non-compliant attribution.
- Accidentally vendoring Apache-2.0 tool-repo files (edit-format coder prompts, `benchmark/` harness) into the otherwise-MIT fixture tree, mixing licenses without the required `LICENSE.txt` + `NOTICE`.
- Vendoring all six full Exercism tracks (~700 exercises) when the benches exercise a small subset → a bloated, hard-to-audit tree.
- Missing per-track attribution / `NOTICE` (MIT requires the copyright + permission notice be retained on redistribution).

**Why it happens:**
The brief itself is wrong, and "Aider" colloquially conflates the tool repo and the fixtures repo. Vendoring "everything" feels safer than curating a subset.

**How to avoid:**
- **Treat STACK.md's correction as binding:** vendored polyglot fixtures get `SPDX-License-Identifier: MIT` + a per-track `NOTICE` citing `exercism/<lang>@<sha>`, NOT Apache-2.0.
- **Do NOT vendor aider tool-repo code.** Re-derive the edit-format/fuzzy drift corpus *natively* against Helix's own 4-strategy cascade (STACK.md 2C, FEATURES.md). This keeps the tree single-license and avoids the Apache-2.0 `NOTICE` obligation entirely. (MEDIUM — recommendation, not a hard constraint; if tool code IS vendored, it needs Apache-2.0 SPDX + `LICENSE.txt` + `NOTICE`.)
- **Vendor only the exercised subset, recorded in a manifest** (STACK.md "vendoring scope guard"), so the committed baseline is reproducible and the NOTICE is auditable.
- **Extend `make verify-licenses` (Phase 85's HARD-FAIL gate) to cover the VENDORED tree**, not just the cloned tracks. The verifier already clones the Phase-75 `verify_tos.go` strict-decode discipline; point it at the committed fixtures and assert each track's SPDX + NOTICE + sha256. The Phase-85 verifier was *tamper-tested* — keep that: a test that flips a license header MUST fail the gate.

**Warning signs:**
- Any `SPDX-License-Identifier: Apache-2.0` header on a file under `fixtures/.../exercises/`.
- A file from `Aider-AI/aider` (tool repo) appearing in the vendored tree without `LICENSE.txt` + `NOTICE`.
- The vendored tree has far more exercises than the runners reference.
- `make verify-licenses` passes after you corrupt a license header (gate not tamper-proof).

**Phase to address:**
The **fixture-vendoring phase** (first Aider-validation phase, ~Phase 102). The extended `verify-licenses` gate + its tamper test ship in that same phase.

---

### Pitfall 4: Non-deterministic / non-reproducible committed baseline

**What goes wrong:**
The milestone commits a baseline results artifact (`BENCH-RESULTS.md` / `result.v2.json`). It is worthless — and actively misleading — if it can't be regenerated byte-identically. Failure modes:
- An evaluator or report uses an unseeded RNG (BCa bootstrap resamples, any shuffle) → a re-run diffs against the committed baseline and every CI/local check sees a spurious "regression."
- Non-deterministic map-iteration ordering leaks into the rendered report (the **Phase 80 WR-03** "latent non-deterministic `fairness.overrides[]` ordering" — already a known landmine here).
- The baseline bakes in machine-specific data (absolute paths, timestamps, hostname, wall-clock latency) → only reproducible on the author's machine.
- A regenerate command exists but isn't the *same* code path that produced the committed file, so they drift.

**Why it happens:**
Statistics imply randomness; timing is inherently machine-specific; Go map order is deliberately randomized. None of these are obvious in a passing local run.

**How to avoid:**
- **Reuse the existing seed-deterministic discipline, don't reinvent it.** The Phase 82/89 aggregator already produces **byte-identical golden `.md`** via a single zero-RNG `renderAll` (BCa seeded ≥10,000 resamples; `TestReportByteReproducible` double-renders and diffs-empty). Any new RepoMap/fuzzy report MUST route through the same deterministic renderer and ship its own byte-reproducible double-render test.
- **Seed every resample/shuffle explicitly** and assert determinism (the Phase 82 `fake-BCa discriminator` precedent shows the project already gates statistical correctness).
- **Sort before emit** for any map-derived collection (closes the Phase-80 WR-03 ordering class).
- **Separate "score" from "timing":** the committed baseline must record *outcome/quality* metrics (pass@k, recall@k, edit-sim, edit-format-applied) which are deterministic given fixtures — NOT raw latency, which is machine-specific and belongs to the local-only microbench (`make bench-micro`), never the committed correctness baseline.
- **One code path, two callers:** the regenerate command (`helix-bench report`) and the original (`aggregate`) must share the renderer (Phase 89 precedent), so the committed file and the regenerated file are the same bytes by construction.

**Warning signs:**
- Re-running the baseline produces a non-empty diff.
- A report contains an absolute path, a hostname, a timestamp, or a raw millisecond latency.
- An evaluator calls `rand.` / `math/rand` without a fixed seed.
- `range someMap` feeds directly into rendered output without a sort.

**Phase to address:**
The **committed-baseline phase** (last Aider-validation phase). Determinism tests for the *new* RepoMap/fuzzy reports ship in their respective surface phases.

---

### Pitfall 5: RepoMap-quality / fuzzy-robustness gold corpus is self-confirming

**What goes wrong:**
Both the RepoMap eval (top-k recall / MRR / nDCG of "relevant symbols for task T") and the fuzzy-robustness bench (drift corpus → expected strategy + result) need a **gold corpus**. The corpus is worthless if it encodes the current implementation's output:
- **Self-confirming RepoMap gold:** the "relevant symbols" label set is generated by *running `get-repo-map` today and recording its top-k* → the eval then measures whether `get-repo-map` agrees with its past self. A regression that drops the truly-relevant symbol still scores 100% because the gold was the buggy output. (This is the **Phase 89 canary** lesson in corpus form: a fixture that gives the right and wrong answers the *same* verdict proves nothing.)
- **Fuzzy gold encodes the cascade:** the "expected strategy" label is whatever the current cascade *happens* to pick, so a future change that picks a worse-but-still-passing strategy never trips.
- **Corpus too small to mean anything:** 3–5 hand-picked cases → noise; a single ambiguous case can't distinguish "refuses correctly" from "refuses everything."

**Why it happens:**
Hand-labeling relevance is expensive; the path of least resistance is to snapshot current output and call it gold. Small corpora pass fast and look done.

**How to avoid:**
- **Gold relevance must be authored from the TASK, independent of the tool.** For each RepoMap eval task, a human (or the task's own ground-truth solution file set, e.g. the exercism `files.solution`) defines which symbols/files are relevant — derived from *what solving the task requires*, never from `get-repo-map`'s current ranking. Document the labeling provenance in the corpus (the Phase-85 `TestJavaFixtureProvenance` "REAL-provenance-sourced + anti-tautology gate" is the exact pattern to clone).
- **Add an anti-tautology discriminator test:** assert the gold corpus would FAIL a deliberately bad ranker (e.g. reverse-sorted, or random) — if a broken ranker still scores ≥ threshold, the corpus encodes nothing. Mirror the Phase-86 `editsim` "provably NOT the same as X" discriminator and the Phase-82 `fake-BCa` discriminator.
- **Fuzzy corpus must include a known-ambiguous case that MUST be refused** and a known-unambiguous case that MUST apply via a *named* strategy, with the expected strategy derived from the drift type (whitespace drift → whitespace-normalized), not from observed behavior. Assert ambiguity refusal explicitly (the `internal/fuzzy` ambiguity-refusal contract is the spec).
- **Size floor:** enough cases per strategy / per language that one flake can't swing the headline; record the count and fail the build if a strategy has zero cases (cf. the empty-bucket trap, Pitfall 1/Phase 87).

**Warning signs:**
- The gold labels were produced by running the tool under test.
- A reversed/random ranker still passes the RepoMap eval.
- The fuzzy "expected strategy" column was copied from a test run, not derived from the drift type.
- Any strategy or language has zero corpus entries (silent empty bucket).

**Phase to address:**
RepoMap gold → the **RepoMap-eval phase** (~Phase 103). Fuzzy gold → the **fuzzy-robustness phase** (~Phase 104). Each phase owns its anti-tautology discriminator.

---

### Pitfall 6: Steering over-reach — nudging/denying grep/sed/cat where they are legitimately correct, or breaking fail-open

**What goes wrong:**
"Stronger steering" (PROJECT.md line 189) tempts three regressions:
- **Over-firing the nudge** on legitimate non-code use. CLAUDE.md is explicit ("when grep IS still correct"): free-text search in comments/READMEs/docstrings/logs, non-code files (YAML/JSON/TOML/Markdown/Dockerfiles/shell), unknown-symbol discovery, build/test output. Broadening `classifyBashTarget` to `awk`/`head`/`tail`/pipelines without preserving the prose/log/config allowlist trains the model to ignore the nudge (cry-wolf) or fights real workflows.
- **Breaking the fail-open exit-0 contract.** The existing nudge is *advisory, exit 0, never blocks* (`nudge.go`). FEATURES.md lists a deny/block hook (exit 2) as an explicit **anti-feature** — exit 2 makes Claude Code treat it as a blocking error and trains the model to evade the tool. Any change that lets the nudge exit non-zero (or a Codex hook return `permissionDecision:"deny"` by default) breaks the locked design.
- **SessionStart priming bloat.** Injecting the full decision matrix (or worse, the whole per-verb reference) at SessionStart regresses the 599-byte idle-cost win that was the *entire point* of v2.0's terse skill.

**Why it happens:**
"Stronger" reads as "more aggressive." Deny *feels* like it would raise adoption. Priming *feels* free.

**How to avoid:**
- **Keep advisory exit-0 as the default and assert it:** a test that runs the nudge on every shape and asserts exit code == 0 (deny is opt-in only, for a narrow positively-identified shape like `sed -i` on a `.go` file — FEATURES.md). For Codex, the hook emits `additionalContext` (camelCase, mirrors Claude's envelope — STACK.md 1B); a test asserts it never emits `permissionDecision:"deny"` by default.
- **Negative-control corpus for the classifier (the same one from Pitfall 1):** prose/log/config/build-output targets MUST NOT fire the nudge. Extend `nudge_test.go` with these rows; a broadened classifier that fires on `grep TODO README.md` fails the suite.
- **SessionStart priming, if added, injects ONLY the terse matrix, never the reference**, and a SKILL-04-style assertion caps the injected size. Keep the three tiers (idle frontmatter → on-trigger SKILL.md body → on-demand reference + `get-tool-help`) and assert the tier boundary (FEATURES.md "progressive disclosure contract").

**Warning signs:**
- The nudge fires on a Markdown/YAML/log target.
- The nudge (or Codex hook) can exit non-zero / deny by default.
- SessionStart injects more than the terse matrix; idle context cost grows past the v2.0 baseline.

**Phase to address:**
The **stronger-steering phase** (~Phase 100). Its VERIFICATION must include the exit-0 assertion + the negative-control classifier rows + an idle-cost cap.

---

### Pitfall 7: Multi-agent install clobbers user files / wrong locations / assumes hooks that don't exist

**What goes wrong:**
`helix setup codex|gemini-cli|generic` now *writes* instruction files (today they are teardown-only — FEATURES.md, STACK.md 1B). Failure modes:
- **Clobbering a user's existing `AGENTS.md` / `GEMINI.md`** by overwriting it wholesale instead of merging/appending a Helix section. Users keep real project instructions in those files.
- **Wrong file location / cap:** Codex `AGENTS.md` has a **32 KiB/file cap** (`project_doc_max_bytes`) and a specific discovery order (project root → cwd walk; `~/.codex/AGENTS.md`); Gemini's context filename is *configurable* via `context.fileName` in `settings.json` and lives at `~/.gemini/GEMINI.md` + workspace/parent dirs. Writing to the wrong path = silently ignored.
- **Assuming a PreToolUse-equivalent exists where it does not:** Gemini CLI has **no PreToolUse hook** (STACK.md 1B, HIGH). Wiring a Gemini "nudge hook" is impossible; steering there is context-file-only. Codex *does* have a PreToolUse hook but only `type:"command"` handlers run today.

**Why it happens:**
The Claude Code path (skill + hook) is the mental model; authors assume the other agents mirror it. File-writing setup tends to overwrite by default.

**How to avoid:**
- **Append a delimited, idempotent Helix block, never overwrite.** Use sentinel markers (`<!-- helix:begin -->` … `<!-- helix:end -->`) and re-write only between them; preserve everything else. Golden-file round-trip tests (the existing `test/harness/golden.go` pattern) for: (a) writing into an empty dir, (b) writing into a file with pre-existing user content (assert user content survives), (c) re-running setup (assert idempotent — no duplicate block).
- **Respect each agent's path + cap as a tested contract:** a test asserts the Codex block stays under 32 KiB and the file lands at the documented location; Gemini writes `GEMINI.md` at the documented path.
- **Encode capability differences in the registrar, not in hope:** Codex registrar writes `AGENTS.md` + `~/.codex/hooks.json` (`type:"command"` → `helix nudge`, reusing the existing nudge — do NOT write a second steering engine); Gemini/IDE/generic registrars write the instruction file ONLY (no hook). A test asserts no Gemini hook artifact is produced.

**Warning signs:**
- Setup truncates or replaces an existing `AGENTS.md`/`GEMINI.md`.
- Re-running setup duplicates the Helix block.
- A Gemini hook file is generated.
- The Codex instruction block exceeds 32 KiB.

**Phase to address:**
The **multi-agent coverage phase** (~Phase 100/101). Golden round-trip + idempotency + cap tests ship in that phase.

---

### Pitfall 8: Scope/overlap — duplicating the existing aider-polyglot adapter, the v1.12 evaluators, or `get_tool_help`

**What goes wrong:**
This milestone sits ON TOP of the v1.12 bench stack and the v2.0 skill. "Add the aider benchmark / add per-verb help / add edit-similarity scoring" all read like new work but already exist. Duplicating them wastes effort AND risks regressing load-bearing invariants:
- Rebuilding the **polyglot adapter** (`bench/datasets/aider-polyglot/{clone,loader,pin}.go`) risks regressing the **WR-01 anti-tamper pristine-test restore** and the **WR-02 Rust `--include-ignored`** vacuous-pass guards (Phase 85). FEATURES.md: reuse `RunExercise` verbatim; the v2.1 delta is *vendoring + the EDIT-verb `AgentFn` + the applied-correctly field*, not the driver.
- Re-implementing **edit-similarity** duplicates `bench/evaluators/editsim` (CM-ES, already discriminator-gated as "provably NOT git-numstat").
- Re-authoring **per-verb help prose** by hand duplicates `internal/kernel/help` (`get_tool_help`) AND drifts from the frozen 50-verb registry — the per-verb reference MUST be *generated* from the registry via `cmd/docgen` plumbing (STACK.md 1A; project memory *helix-tool-docs-drift*: docgen's blank imports must == the daemon's, or the generated docs silently lose tools).

**Why it happens:**
The milestone brief names "aider benchmark" and "per-verb reference" as deliverables without flagging the existing substrate; an author who doesn't read the v1.12 tree starts from scratch.

**How to avoid:**
- **Reuse-don't-fork is a stated repo ethos** (the v1.12 log repeats "clone the template ×N, swap only Detect/parse"). The roadmap's first Aider-validation phase must *start* by reusing `RunExercise`, `editsim`, the result.v2 additive-open-key pattern (`language`/`embedder_id` precedent — no schema v3 bump), and the aggregator. New work is strictly the *delta* (FEATURES.md "Scope Note — what already exists").
- **Per-verb reference is generated, never hand-written.** Drive it from `internal/cli/verbs_gen.go` + the registry the same way the README tool table is generated. A drift test (Pitfall 1) asserts coverage of the registry. Verify docgen's blank-import set matches the daemon's (project memory).
- **`AgentFn` drives Helix EDIT verbs** (`replace-symbol-body`/`fuzzy-edit`/`replace-in-file`/`insert-*`) — that is the actual gap (FEATURES.md "the real gap"); the loader stays verb-agnostic and untouched.
- A **"do not duplicate" checklist** in each Aider-validation phase's plan, citing the exact existing files to reuse.

**Warning signs:**
- A new clone/loader/pin under a different path.
- A second edit-similarity implementation.
- A hand-edited per-verb reference (the README table warns "do not hand-edit").
- A `result.v2` schema bump (v3) for something that should be an additive open key.
- `cmd/docgen` imports a different skill set than the daemon (silently drops verbs).

**Phase to address:**
Every Aider-validation phase, and the per-verb-reference phase (~Phase 98). The reuse checklist is a planning-time gate, not a code test.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `t.Skip` without a hermetic golden sibling | Test "passes" on CI with no binary | False-green; the surface is never exercised (project's most-repeated bug) | **Never** — Phase-85 precedent requires a hermetic sibling as the sole authoritative proof |
| Snapshot current tool output as the gold corpus | Instant "labels" for RepoMap/fuzzy eval | Eval becomes self-confirming; regressions invisible | **Never** for relevance/strategy labels; OK only for *byte-reproducibility* goldens (output-is-the-spec) |
| Hand-write the per-verb reference | Fast first draft | Drifts from the frozen 50-verb registry; the deterministic contract becomes a lie | **Never** — generate from registry |
| Vendor all six full Exercism tracks | "Complete" fixtures | Bloated tree, unauditable NOTICE, slow clones | Only if a manifest justifies each track; prefer exercised subset |
| LLM-judge rubric with no negative anchor | Fewer flaky reds | Rubric can't fail → eval proves nothing | **Never** — must include a 0-scoring exemplar |
| Lenient detector (`mentionsHelix` substring) | Higher, more "stable" choice-rate | Counts the prompt's own text as adoption | **Never** — key on first emitted command line |
| Skip the revert-and-fail test for a new gate | Ships faster | Gate presumed broken; four such gates already shipped OPEN here | **Never** for any merge-gating assertion |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Codex CLI | Assume its hook == Claude's exactly; or write a 2nd steering engine | Envelope IS the same (`additionalContext` camelCase); reuse `helix nudge` via `hooks.json` `type:"command"`; respect 32 KiB `AGENTS.md` cap |
| Gemini CLI | Wire a "PreToolUse nudge" | Gemini has **no** PreToolUse hook; steer via `GEMINI.md` context only |
| `AGENTS.md`/`GEMINI.md` | Overwrite the user's file | Append a sentinel-delimited Helix block; idempotent; preserve user content |
| Anthropic SDK / judge | Treat a flaky LLM score as a merge gate | Behavioral layer never blocks merge (v1.4 locked); deterministic layer gates |
| `result.v2.json` schema | Bump to v3 for a new field | Add an **additive open key** (precedent: `language`, `embedder_id`, `ablation_status`) |
| `make verify-licenses` | Audit only the cloned tracks | Extend to the **vendored** tree; keep the tamper test |
| `cmd/docgen` | Different blank-import set than daemon | Imports must match daemon's, or generated docs silently drop verbs (project memory) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Latency in the committed baseline | Baseline diffs across machines | Commit only deterministic quality metrics; latency → local `bench-micro` only | First re-run on a different machine |
| SessionStart priming bloat | Idle context cost > v2.0's 599 B | Prime terse matrix only; cap with a SKILL-04-style assertion | Every session, immediately |
| Vendoring all six tracks | Slow clones, huge diff, unauditable NOTICE | Vendor exercised subset + manifest | At review / git operations |
| Full 225-task polyglot run in CI | Minutes-to-hours, 6 toolchains, network | Hermetic fixture proof in CI; live run local/`HELIX_BIN`-gated | First CI run |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Codex hook deny-by-default | Trains model to evade tool; blocks legit grep | Advisory exit-0 default; deny opt-in, narrow shape only |
| Vendored fixture path traversal on load | Reads/writes outside fixture dir | Reuse existing `validatePathSegment`/`isHexSHA1` guards (Phase 84/85/86 precedent) |
| Wrong-license redistribution | Non-compliant attribution | MIT SPDX + per-track NOTICE; `verify-licenses` hard-fail gate with tamper test |
| Setup overwrites user instruction file | Data loss of user's project rules | Sentinel-delimited append; round-trip test preserves user content |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Nudge cries wolf on prose/logs/config | Model learns to ignore the nudge | Negative-control classifier rows; preserve prose/log/config allowlist |
| Deny hook blocks legitimate grep | Real workflows break; user fights tool | Keep advisory exit-0 default |
| Reference tells *which* verb, not *how* | Agent picks the verb but mis-calls it | Per-verb args + output shape + worked example (generated from registry) |
| Fat always-loaded reference | Idle context cost regresses | Progressive disclosure: on-demand reference file + `get-tool-help` |

## "Looks Done But Isn't" Checklist

- [ ] **Adoption contract:** passes after deleting a verb from the reference? Then it's tautological — re-source from the registry.
- [ ] **LLM-behavioral score:** identical with and without the skill installed? Then it measures nothing — add the sabotaged-skill revert-and-fail.
- [ ] **Every new bench surface:** has a hermetic golden sibling that runs with NO `HELIX_BIN` and NO network? If only a skip-guarded test exists, it's false-green.
- [ ] **Fail-closed:** does a missing `result.v2.json` / empty run dir / missing metric line hard-error, or read as 0/pass? (Phase 82/81 lesson.)
- [ ] **Vendored fixtures:** SPDX = MIT (not Apache-2.0)? Per-track NOTICE present? `verify-licenses` fails on a corrupted header?
- [ ] **Baseline:** re-runs byte-identically? No paths/timestamps/hostname/latency? Same renderer for `aggregate` and `report`?
- [ ] **Gold corpus:** would a reversed/random ranker FAIL it? If not, it encodes the implementation.
- [ ] **Nudge:** exit 0 on every shape? Does NOT fire on `grep TODO README.md`?
- [ ] **Multi-agent setup:** preserves pre-existing user `AGENTS.md`/`GEMINI.md` content? Idempotent on re-run? No Gemini hook artifact?
- [ ] **Empty bucket:** any task set / strategy / language with zero entries reported as a pass? (Phase 87 lesson.)

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Vacuous adoption eval shipped | MEDIUM | Add revert-and-fail self-test; re-source completeness from registry; re-run — expect prior green to go RED on sabotage |
| HELIX_BIN false-green | LOW | Add hermetic golden sibling per surface; add "did it RUN" sentinel; re-audit `go test ./bench/...` coverage |
| Wrong license stamped | LOW–MEDIUM | Correct SPDX to MIT; add per-track NOTICE; extend + tamper-test `verify-licenses` |
| Non-reproducible baseline | MEDIUM | Route through deterministic `renderAll`; seed RNG; sort-before-emit; strip latency/paths; re-commit |
| Self-confirming gold corpus | HIGH | Re-author labels from task ground truth (e.g. `files.solution`); add reversed-ranker discriminator; re-label is the cost driver |
| Over-firing/deny nudge shipped | LOW | Restore exit-0; add negative-control rows; gate deny behind narrow opt-in shape |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase (indicative — roadmapper assigns final numbers) | Verification |
|---------|------------------|--------------|
| 1. Adoption-eval vacuity | Adoption-contract phase (~99); behavioral self-test (~101+) | Revert-and-fail: sabotaged skill → score drops; deleting a verb → contract RED |
| 2. HELIX_BIN false-green | Each bench-surface phase (~102/103/104) + baseline phase | Hermetic golden runs with no binary; "did it RUN" sentinel when HELIX_BIN set |
| 3. License vendoring | Fixture-vendoring phase (~102) | `verify-licenses` covers vendored tree; tamper test fails on corrupted header; SPDX==MIT |
| 4. Non-reproducible baseline | Committed-baseline phase (last) + each surface | Double-render diff-empty; no paths/timestamps; shared renderer |
| 5. Self-confirming gold corpus | RepoMap-eval (~103) + fuzzy-robustness (~104) | Reversed/random ranker FAILS the corpus; labels provenance-documented |
| 6. Steering over-reach | Stronger-steering phase (~100) | Exit-0 on all shapes; negative-control rows don't fire; idle-cost cap |
| 7. Multi-agent install clobber | Multi-agent phase (~100/101) | Round-trip preserves user content; idempotent; no Gemini hook; 32 KiB cap |
| 8. Scope/overlap duplication | Per-verb-reference phase (~98) + every Aider phase | Reuse checklist cites existing files; reference generated from registry; no schema v3 bump |

## Sources

- In-tree, direct read (HIGH): `.planning/PROJECT.md` (v2.1 milestone + the v1.12 phase-by-phase progress log of four named vacuous-pass CRITICALs: Phase 82 CR-01 N-gate-fail-open, Phase 86 CR-01 zero-value-GateConfig, Phase 87 CR-01 empty-bucket-as-pass + `TestVerified_VacuousPass`, Phase 89 CR-01 canary-exclusion-partial-wiring + revert-and-fail); `bench/BENCH.md` (no-skip-only-without-sibling, fail-closed scrape, baseline rules); this cycle's `STACK.md` (license correction MIT≠Apache-2.0, Codex/Gemini hook capability matrix, no-new-deps) and `FEATURES.md` (anti-features: deny hook, rebuild-adapter, self-confirming eval; the existing-surface scope note)
- Project memory (HIGH): `helix-bench-smoke-false-green` (HELIX_BIN skip false-green + no_semantic SIGKILL-vacuous-gate), `helix-tool-docs-drift` (docgen blank-import == daemon or docs silently drop tools)
- CLAUDE.md (HIGH): "when grep/Bash/Read IS still correct" (prose/logs/config/build-output) — the steering-overreach boundary
- External, this cycle's STACK.md sources (HIGH): Claude Code Skills docs (1,536-char idle cap, <500-line body, progressive disclosure); OpenAI Codex AGENTS.md (32 KiB cap) + Hooks (`type:"command"`-only, `additionalContext`/`permissionDecision`); Gemini CLI GEMINI.md (no PreToolUse hook); Aider polyglot = Exercism MIT redistribution

---
*Pitfalls research for: Helix v2.1 — Agent Adoption & Aider-Derived Validation*
*Researched: 2026-06-22*
