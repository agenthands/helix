# Pitfalls Research

**Domain:** Agent-facing skill quality + generated-doc gates + offline DSPy prompt-tuning for a Go single-binary code-intelligence CLI (Helix v2.2)
**Researched:** 2026-06-23
**Confidence:** HIGH (generator/gate/bundle pitfalls verified against live source; DSPy pitfalls Context7/web-verified MEDIUM-HIGH)

> Scope note: these are pitfalls specific to **ADDING the four v2.2 features to THIS system** — (1) SKILL.md decision-matrix rewrite, (2) reference.md generator fixes, (3) installSkill bundle allowlist, (4) exploratory offline DSPy tuning. They carry forward the anti-vacuity / reproducibility / no-runtime-Python discipline that v1.12–v2.1 already encoded (`test/oracle/adopt/scorecard.go`, the `helix-refgen --check` gate, the embed bundle). `.planning/codebase/CONCERNS.md` is **stale** (describes the removed Python Serena tree) and was not used as a source.
>
> Phase numbering continues from **102** per PROJECT.md. Phase names below are descriptive; the roadmapper assigns final numbers.

---

## Critical Pitfalls

### Pitfall 1: The embed glob ships stray files into the binary AND the installed skill (the SKILL-ISSUE.md leak is live RIGHT NOW)

**What goes wrong:**
`internal/cli/skill.go` declares `//go:embed skills/helix/*` and `installSkill` loops over **every** entry from `embeddedSkillFS.ReadDir("skills/helix")`, writing each to the user's `.claude/skills/helix/` dir. Today that directory contains `SKILL.md`, `reference.md`, **and `SKILL-ISSUE.md`** (an 18 KB maintainer analysis doc). So the binary already embeds the issue doc, and `helix setup` already writes it onto every user's disk. Any future file dropped in that dir (a scratch `.bak`, a `NOTES.md`, an editor swapfile, a half-finished `reference.md.new`) leaks the same way. The maintainer doc that *names* this leak is itself being leaked — the canonical irony.

**Why it happens:**
A wildcard embed + a "loop over all entries" installer is the path of least resistance, and it was correct when the dir held exactly the two intended files. The bundle test (`TestInstallSkillWritesBundle`) only asserts SKILL.md and reference.md are **present and non-empty** — it never asserts the set is **exactly** those two, so the leak is invisible to CI.

**How to avoid:**
Convert to an explicit allowlist in BOTH directions: (a) replace the wildcard with named embeds `//go:embed skills/helix/SKILL.md skills/helix/reference.md` (or keep the glob but filter the install loop against a hardcoded `allowed := map[string]bool{"SKILL.md":true, "reference.md":true}`), and (b) add a **closed-set** bundle test asserting `ReadDir` over the *embedded FS* yields EXACTLY the allowlist — fail on any extra entry. The closed-set assertion is the load-bearing one: a positive-only "are the two files present" test is the vacuous-gate pattern this repo has been bitten by repeatedly (Phase 87 CR-01, Phase 89 CR-01). Move `SKILL-ISSUE.md` out of `skills/helix/` entirely (e.g. to `.planning/` or a `docs/` sibling) so it is neither embeddable nor installable.

**Warning signs:**
- `helix setup claude-code` then `ls ~/.claude/skills/helix/` shows more than two files.
- `go tool nm helix | grep SKILL-ISSUE` or `strings helix | grep "SKILL.md Decision Matrix Review"` hits.
- A bundle test that only does `for _, name := range []string{"SKILL.md","reference.md"}` (present-check) with no reverse "no other files" check.

**Phase to address:**
Phase 102 (Bundle allowlist + closed-set test) — and it should run EARLY in the milestone because the SKILL.md rewrite and refgen fixes will add/remove files in that dir, widening the leak window if the allowlist isn't in place first.

---

### Pitfall 2: Adding the allowlist breaks the atomic two-pass install (torn bundle / orphaned `.tmp` / containment regression)

**What goes wrong:**
The current `installSkill` is a carefully-built two-pass atomic install (stage all temp siblings → rename all → best-effort revert on failure, with a `filepath.Rel` ".."-escape containment guard, T-93-01). The naive way to add an allowlist — early-`continue` inside the existing loop, or a second loop — can (a) leave a renamed SKILL.md next to a NOT-renamed reference.md if the filter logic is wrong (torn bundle), (b) leave `.tmp` siblings on a filtered-out file's error path, or (c) move the containment check so a crafted target escapes. `uninstallSkill` has the SAME loop and must be filtered identically or it will orphan a now-un-allowlisted file (e.g. leave `SKILL-ISSUE.md` on disk forever after an upgrade that stopped shipping it).

**Why it happens:**
The atomicity and containment invariants are subtle and spread across staging/rename/cleanup passes; a "just skip the file" edit looks trivial but sits inside that machinery. install and uninstall are separate functions that must stay in lockstep.

**How to avoid:**
Filter the **entry list once, up front** (derive `allowed := filterAllowlist(entries)`) and feed the SAME filtered slice to the existing staging/rename/cleanup passes unchanged — do not sprinkle `continue`s through the passes. Apply the identical filter to `uninstallSkill`, and add a regression test that an upgrade which drops a file removes it from disk (install old-set → install new-set → assert dropped file gone). Keep the containment guard as the first statement, untouched. Re-run the existing `TestInstallSkillContainment` and the atomic-revert tests after the change; they are the non-vacuity proof.

**Warning signs:**
- A filtered file leaves a `.tmp` sibling after an injected mid-install failure.
- `uninstallSkill` leaves a file the allowlist no longer recognizes.
- Diff touches the rename/revert loop bodies rather than the entry-list construction.

**Phase to address:**
Phase 102 (same phase as Pitfall 1 — the allowlist and its atomicity are one unit of work).

---

### Pitfall 3: Hand-editing the generated `reference.md` to "fix" the copy-paste errors (breaks `--check`, defeats the whole point)

**What goes wrong:**
SKILL-ISSUE.md lists 13 wrong "use this, not that" strings and 22 wrong "Output:" descriptions in `reference.md`. The tempting fix is to open `internal/cli/skills/helix/reference.md` and edit the strings directly. But `reference.md` is **generated** by `cmd/helix-refgen` and guarded by `go run ./cmd/helix-refgen --check` (exit 1 if the file would change). A hand-edit either (a) is immediately reverted the next time anyone runs the generator, or (b) makes `--check` pass against a hand-edited committed file while the generator's `render.go` still emits the wrong text — so the gate now certifies a file the generator can't reproduce. Both outcomes silently re-introduce the bug.

**Why it happens:**
The wrong strings live in `reference.md` (the visible artifact), but the ROOT lives in `cmd/helix-refgen/render.go` (`useThisNotThat(group, verb)` and the `**Output:**` emitter), which is one layer removed. The copy-paste errors are *generated* — every memory verb shares one `groupID`, so `useThisNotThat` emits the same "durable project/session memory" string for `delete-memory` and `edit-memory` even though those have the OPPOSITE purpose. The fix MUST be in the renderer's per-verb/per-group mapping.

**How to avoid:**
Fix `render.go` (the `useThisNotThat` group→string map and the Output emitter) so the *generator* produces the correct per-verb text, THEN run `go run ./cmd/helix-refgen` to regenerate, THEN commit both the render.go change and the regenerated reference.md together. Add the row-split / query-vs-action distinction at the generator level (a verb's `groupID` or a new per-verb override field) — never as a post-hoc text patch. The `--check` gate then certifies the file is byte-reproducible from source. Verify by deleting reference.md, regenerating, and confirming `git diff` is empty.

**Warning signs:**
- A PR edits `reference.md` but not `cmd/helix-refgen/render.go`.
- `go run ./cmd/helix-refgen --check` fails on a freshly-pulled tree.
- After regeneration, `git diff internal/cli/skills/helix/reference.md` is non-empty (proves a prior hand-edit or generator drift).

**Phase to address:**
Phase 103 (reference.md generator fixes) — the render.go `useThisNotThat`/Output mapping rewrite.

---

### Pitfall 4: The docgen/refgen/cligen blank-import-parity-with-daemon trap (generated surface silently diverges from the live tool set)

**What goes wrong:**
`cmd/helix-refgen` enumerates verbs by blank-importing skill packages to fire their `init()` registration, EXACTLY as `internal/daemon/imports.go` does. If the milestone adds/moves a verb or a skill package and updates the daemon's import list but NOT refgen's (or vice-versa), the generated `reference.md` describes a different tool SET than the daemon actually serves. This already bit the repo once: a missing `internal/skill/semantic` blank import in `cmd/docgen` produced docs out of sync with the live 53-tool registry (MEMORY: "Helix tool docs drift" — root cause was a missed blank import, not a missed regen). The same trap exists for `cmd/helix-cligen` and `cmd/docgen`.

**Why it happens:**
The import lists are maintained by hand in 3+ places (`daemon/imports.go`, `cmd/helix-refgen/main.go`, `cmd/docgen`, `cmd/helix-cligen`) and only "reciprocal note" comments tie them together. Literal equality is NOT required (health/help are non-blank in the daemon, guardrails contributes zero rows), which makes a naive "diff the import lists" check produce false positives and lull maintainers into ignoring real drift.

**How to avoid:**
The real protection is the `--check` gate **plus** the `reference ⊇ VerbToolNames()` contract test (Pitfall 5) — together they fail CI if the generated set drifts from the live verb registry, regardless of import-list cosmetics. When touching skill packages in v2.2, treat "did I update refgen's blank imports to match the daemon's?" as a checklist item, and rely on the contract test rather than eyeballing imports. Do NOT add `internal/semantic/extract/*` to refgen's imports (per the in-file D-02 note: a second GrammarRegistry breaks the singleton). Run the full generator + `--check` + contract suite after any skill-package change.

**Warning signs:**
- `go run ./cmd/helix-refgen --check` passes locally but a verb is missing from `reference.md`.
- A skill package import was added to `daemon/imports.go` in the diff but not to `cmd/helix-refgen/main.go`.
- The `reference ⊇ VerbToolNames()` test is green but a human notices a verb absent from the doc (means the contract test itself regressed — see Pitfall 5).

**Phase to address:**
Phase 103 (reference.md generator fixes) — the phase that touches refgen is where import parity must be re-verified.

---

### Pitfall 5: The `reference ⊇ VerbToolNames()` adoption contract silently weakens (the gate that guarantees "every verb is documented" turns vacuous)

**What goes wrong:**
v2.1 shipped a merge-gating contract that `reference.md` covers (is a superset of) every name in `VerbToolNames()`. The v2.2 SKILL.md rewrite SPLITS rows (37→44) and the refgen rewrite changes how verbs are rendered. If the rewrite changes the heading/anchor format the contract test greps for (e.g. it matches `### helix <verb>` and the new template emits `## helix-<verb>`), the test can pass while actually matching nothing — the superset check becomes `∅ ⊇ ∅` vacuously true. Same risk if a verb is split across two rows and the matcher counts the row, not the verb. The repo's own history is littered with exactly this failure class: Phase 86 CR-01 (`es >= 0.0` always-true oracle collapse), Phase 89 CR-01 (contaminated-row exclusion wired into only some reduces), Phase 101 `MaterialDrop = 0.4` (deliberately NOT `>= 0.0`).

**Why it happens:**
The contract test couples to a *textual* shape of the generated artifact; the artifact's shape is exactly what this milestone rewrites. A matcher that finds zero anchors reads as "all covered" instead of "matched nothing."

**How to avoid:**
Before changing the SKILL.md/reference.md format, add a **discriminating** assertion to the contract test: assert the matcher finds a KNOWN-PRESENT verb AND fails loudly when given a KNOWN-ABSENT verb (anti-tautology, the `TestGrade_ZeroValueConfig` / `TestSabotageNonNoop` pattern already in the tree). Assert the matched-verb COUNT equals `len(VerbToolNames())` (50), not just "non-empty superset" — an exact-count floor catches the vacuous-empty case. Update the matcher and the format in the SAME commit, and run the contract test in its RED state first (point it at the old format, confirm it fails) to prove it bites.

**Warning signs:**
- The contract test passes but `reference.md` is visibly missing a verb.
- The matcher regex was changed in the same diff as the format with no count assertion.
- Coverage count is asserted as `> 0` rather than `== 50`.

**Phase to address:**
Phase 102/103 boundary — whichever phase changes the reference/SKILL format must first harden the contract test. Recommend folding a "contract test discriminates" success criterion into the refgen phase (103).

---

### Pitfall 6: DSPy overfits the SKILL/steering prompt to a tiny dev set (the 5–25 adoption transcripts), so `choice_rate` rises offline but generalization doesn't

**What goes wrong:**
The adoption scorecard floor is `MinTasks = 5` (`test/oracle/adopt/scorecard.go`); the live fixture corpus is small. DSPy optimizers (MIPROv2, BootstrapFewShot) maximize the metric over whatever set you hand them. With a handful of transcripts, the optimizer will happily craft a SKILL.md / nudge string that scores `choice_rate ≈ 1.0` on those exact tasks by memorizing their surface cues (specific filenames, specific phrasings) — a prompt that looks great offline and helps nothing on unseen agent interactions. MIPROv2 only auto-enables minibatch protection when `val_size > 50`; below that it evaluates candidates against the full tiny set every trial, maximizing overfit.

**Why it happens:**
Prompt optimization is "fit a function to examples"; with few examples and a high-capacity instruction space (free-text SKILL.md), the optimizer overfits exactly like any ML model on a small training set. The metric (`choice_rate`) is cheap to game when the eval set is the same set you optimize on.

**How to avoid:**
Hold out a TEST set the optimizer never sees: split the transcript corpus into train (optimize) / dev (DSPy's internal validation) / **held-out test** (report only). Report `choice_rate` on the held-out test, never on train. Grow the corpus before tuning — aim for materially more than `MinTasks` per split. Treat any offline gain that does not survive the held-out test as noise, and NEVER auto-commit an optimized artifact that only beat baseline on train/dev. Frame DSPy as EXPLORATORY (PROJECT.md already does) — its output is a *candidate*, gated by the same revert-and-fail discipline (`MaterialDrop`) the scorecard already enforces, measured on data the optimizer didn't touch.

**Warning signs:**
- Train `choice_rate` ≫ held-out `choice_rate` (the textbook overfit gap).
- The optimized SKILL.md contains task-specific tokens (a fixture's filename, a specific symbol name).
- "It scored 1.0!" with no held-out number reported.
- Optimizing and evaluating on the same `test/oracle/adopt` fixtures.

**Phase to address:**
Phase 104+ (exploratory DSPy harness) — the train/dev/test split is the FIRST thing the harness must establish, before any optimizer call.

---

### Pitfall 7: Metric gaming — DSPy optimizes `choice_rate` in ways that don't generalize (the classifier is gameable)

**What goes wrong:**
The scorecard classifies a transcript as a "helix choice" iff `FirstCommand` starts with `"helix "` (prefix on first emitted command). An optimizer told to maximize `choice_rate` can learn degenerate strategies: a SKILL.md that instructs the model to ALWAYS emit a `helix` command first regardless of task fit (inflating `choice_rate` while producing wrong/empty actions), or steering text that games the FIRST-command detector specifically. The scorecard deliberately does NOT use `strings.Contains` (Pitfall 2 in its own design) precisely because the metric surface is gameable; an optimizer is an adversary that will find the next gap.

**Why it happens:**
Any cheap proxy metric becomes a target the optimizer attacks (Goodhart). `choice_rate` measures "did it reach for helix first," not "did it solve the task correctly with helix" — the gap is exploitable.

**How to avoid:**
Pair `choice_rate` with a CORRECTNESS / task-success signal in the optimization metric so "always say helix" doesn't win (reuse the v2.1 Aider-derived edit/repomap benches or the multi-oracle `verified_correctness` pattern as the second term — the exact "metric that resists gaming by joining a quality oracle" lesson from Phase 86/87). Keep `fallback_rate` and `Unclassified` in view (a spike in `Unclassified` or a drop in actual task success alongside a `choice_rate` rise is the tell). Manually inspect the optimized SKILL.md for degenerate "always emit helix" instructions. Gate any candidate on BOTH the scorecard AND an independent quality measure on held-out data.

**Warning signs:**
- `choice_rate` up but task-success / `verified_correctness` flat or down.
- Optimized SKILL.md says "always run helix first" or similar unconditional steering.
- `Unclassified` count drops to zero suspiciously (model emitting helix even for non-code prose).

**Phase to address:**
Phase 104+ (DSPy harness) — the optimization metric design (single phase concern: define the metric as scorecard-AND-quality before optimizing).

---

### Pitfall 8: LLM-in-the-loop optimization is nondeterministic and costly, so the "optimized artifact" isn't reproducible (breaks the committed + `--check`-gated invariant)

**What goes wrong:**
PROJECT.md requires the DSPy output to be a "committed, `--check`-reproducible reference/skill." But DSPy optimization is LLM-driven: MIPROv2 makes many LLM calls to *propose* candidate instructions and to *evaluate* them, and even temperature-0 greedy decoding is not bit-reproducible across runs (floating-point / GPU-kernel nondeterminism — verified). Run the optimizer twice and you get two different SKILL.md texts. If the v2.2 pipeline tries to make "re-run DSPy" a `--check` gate, the gate will flap forever. It is ALSO expensive (each trial × each minibatch × proposer calls = real API spend), so re-running it in CI is a non-starter.

**Why it happens:**
Conflating two different artifacts: (a) the *optimizer run* (nondeterministic, offline, expensive, dev-time) and (b) its *committed output* (a static SKILL.md the build must reproduce byte-for-byte). The `--check` gate belongs to (b) — the generated reference.md from the committed SKILL.md — NOT to (a).

**How to avoid:**
Make DSPy a **dev-time, human-in-the-loop** step whose output is a committed static artifact, exactly like a captured benchmark baseline (the v1.9 "local-only bench" / committed-baseline precedent). The `helix-refgen --check` gate verifies `reference.md` is reproducible **from the committed SKILL.md** — it never re-runs DSPy. A human runs the optimizer offline, reviews the candidate SKILL.md, accepts/edits it, commits it; from there the existing deterministic generator + `--check` chain takes over. For optimizer reproducibility during the dev session, pin the model id + seed + temperature=0 and CACHE LLM responses (DSPy caches by default; commit the chosen artifact, not the process). Never put a DSPy invocation in a merge-gating CI job.

**Warning signs:**
- A CI job that runs `dspy.compile(...)` or hits an LLM API on every PR.
- `--check` flaps green/red across identical commits.
- No committed artifact — the "optimized skill" only exists as an optimizer script.
- Optimizer cost shows up as a recurring API bill tied to CI.

**Phase to address:**
Phase 104+ (DSPy harness) — establish "offline optimizer → reviewed committed artifact → deterministic --check gate" as the architecture decision up front.

---

### Pitfall 9: Train/dev/test contamination — the optimizer sees the eval data, or the SKILL.md being measured includes the fixtures

**What goes wrong:**
Two contamination modes. (a) **Optimizer↔eval contamination:** DSPy's `valset` overlaps the held-out adoption test, so the reported generalization number is inflated (the optimizer already tuned to those tasks). (b) **Artifact↔metric contamination:** the scorecard already guards against the SKILL.md's own "helix" text inflating `choice_rate` (it keys on FIRST command, not `Contains` — `test/oracle/adopt` comment, Pitfall 2/T-101-03). If the DSPy harness builds a NEW classifier or feeds whole transcripts to a judge, it can re-introduce that leak (the injected skill body is full of "helix" and example commands). The contamination canary work in v1.12 (Phase 89 INFRA-05) is the same lesson from the benchmark side.

**Why it happens:**
Small corpora tempt reuse of the same examples for optimize + report. And the SKILL.md-being-optimized literally contains the target token, so any whole-response metric leaks.

**How to avoid:**
Strict, disjoint splits with a documented provenance for each transcript (which split it belongs to), enforced in code (a split that overlaps is a hard error, mirroring the fail-closed split discipline elsewhere). Reuse the EXISTING `test/oracle/adopt` classifier verbatim (it already resists the artifact↔metric leak) rather than building a new judge in the DSPy harness; if a judge is unavoidable, strip/neutralize the injected skill body before scoring, and add a `TestSabotageNonNoop`-style assertion that the metric still drops when the decision matrix is removed. Keep the optimizer's `valset` provably disjoint from the reporting `testset`.

**Warning signs:**
- The same transcript file appears in both the optimizer config and the report config.
- A new DSPy-side classifier uses `Contains("helix")` over the whole response.
- Held-out scores improbably high and identical to dev scores.

**Phase to address:**
Phase 104+ (DSPy harness) — split hygiene + reuse-the-hardened-classifier as explicit success criteria.

---

### Pitfall 10: Accidental runtime Python coupling — DSPy (Python) leaks from a dev-time tool into a runtime dependency

**What goes wrong:**
Helix's defining constraint is "Go single binary, no Python/Docker/runtime deps" (CLAUDE.md, PROJECT.md Constraints). DSPy is Python. The risk is that the prompt-tuning harness — meant to be offline/dev-time — accretes into the runtime: a `helix` subcommand that shells to `python -m dspy...`, a `setup` step that pip-installs DSPy, an embedded Python invocation, or CI that the *product build* depends on. Any of these breaks the single-binary promise that the whole product identity rests on.

**Why it happens:**
"It's just a script, let's wire it in" convenience; and because v1.12's bench stack DOES legitimately shell to Python (`swebench`, `multi_swe_bench`) for *benchmarking*, there's a local precedent that can be over-generalized into the *product*. The line is: benchmark/dev tooling MAY use Python out-of-band; the shipped `helix` binary and its `setup`/runtime path MUST NOT.

**How to avoid:**
Quarantine DSPy entirely outside the Go module's runtime surface — a separate `tools/` or `dev/` dir, its own `requirements.txt`/venv, invoked only by a human or a non-product-gating dev workflow. NO `helix` subcommand imports or shells to it. NO `go.mod` / build-pipeline edge to Python. Add a mechanical guard in the spirit of the existing `vet-noduckdb` / `benchragleakage` / `nokernel2semantic` analyzers: a check that the `helix` binary's runtime packages have zero reference to the DSPy harness, and that `helix setup` never invokes Python. The committed artifact (SKILL.md/reference.md) is the ONLY thing that crosses from the Python world into the Go binary, and it crosses as static bytes via `//go:embed`.

**Warning signs:**
- A `helix <verb>` that calls `exec.Command("python", ...)` for tuning.
- DSPy / Python in `helix setup`'s install path.
- `requirements.txt` referenced by the release pipeline or `make build`.
- The single-binary "no Python runtime" claim in README/CLAUDE drifts.

**Phase to address:**
Phase 104+ (DSPy harness) — the no-runtime-Python boundary is a phase-entry constraint AND should ship a mechanical leakage analyzer (precedent: every prior "external thing" in this repo got a `make vet` boundary gate).

---

## Moderate Pitfalls (Decision-Matrix Design)

### Pitfall 11: Over-splitting decision-matrix rows / token bloat that breaches the idle-cost cap

**What goes wrong:**
SKILL-ISSUE.md proposes splitting 5 rows into 12 (37→44 rows) to separate QUERY from ACTION verbs. Over-correcting — one row per verb (50 rows), or verbose "Not this" prose on every row — bloats SKILL.md. The frontmatter `description` is hard-capped at ≤1,536 chars (SKILL-04, the idle-cost upper bound asserted by `skillDescription()` parsing). The matrix body is below the frontmatter and loads on-use, but a bloated body still costs context every time the skill triggers and dilutes the steering signal.

**Why it happens:**
"Split everything for clarity" momentum; each individual split looks justified.

**How to avoid:**
Split ONLY where a row genuinely conflates a query and an action that an agent would pick between (the SKILL-ISSUE.md set of 5→12 is the calibrated target, not a license to atomize). Keep "Not this" terse (a tool name, not a sentence). Re-run the SKILL-04 ≤1,536-char description assertion after editing; keep the matrix body proportionate. Measure before/after token count (SKILL-ISSUE.md already estimates +51 bytes / +7 rows — stay near that).

**Phase to address:** Phase 102 (SKILL.md rewrite).

---

### Pitfall 12: Ambiguous or stale "use X not Y" steering (the matrix tells the agent to use a verb that doesn't fit, or cites a removed tool)

**What goes wrong:**
A "use X not Y" row where X doesn't actually answer the question, or where the QUERY/ACTION distinction is blurred (SKILL-ISSUE.md's core finding: grouping `read-memory` with `write-memory` confuses when to read vs write). Also: steering that references a verb name that drifts from `VerbToolNames()` (a typo'd or renamed verb in the prose). An ambiguous matrix is worse than none — it sends the agent to the wrong tool confidently.

**Why it happens:**
Copy-paste grouping (the documented root cause), and hand-authored prose that isn't cross-checked against the live verb registry.

**How to avoid:**
One row answers ONE question with ONE primary verb; the QUERY (reads state) vs ACTION (mutates state) split is the organizing principle (SKILL-ISSUE.md §2). Cross-check every verb cited in SKILL.md against `VerbToolNames()` — ideally a test that greps SKILL.md's `helix <verb>` mentions and asserts each is a real frozen verb (the prime.go invariant: "every `helix <verb>` cited MUST be a real frozen verb"). Add the indexed-graph prerequisite note for the 7 semantic-graph verbs (SKILL-ISSUE.md §6) so the agent doesn't call `explain-cluster` before `index-semantic-graph`.

**Phase to address:** Phase 102 (SKILL.md rewrite) — add the SKILL.md↔VerbToolNames cross-check test here.

---

### Pitfall 13: Stale prerequisites — semantic-graph verbs documented without the "requires index" precondition

**What goes wrong:**
7 verbs (`explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`, `get-semantic-graph-status`) require `index-semantic-graph` to have run. The matrix doesn't say so (SKILL-ISSUE.md §6), so an agent gets an empty/error result and falls back to grep — the exact failure the skill exists to prevent.

**Why it happens:** The prerequisite is implicit in the subsystem, invisible in the doc.

**How to avoid:** Add a prerequisite note/column for the semantic-graph group. Keep it in sync with the actual gating behavior (if a verb later auto-indexes, the note must update — tie it to the generator so it can't drift).

**Phase to address:** Phase 102 (SKILL.md) for the prose; consider emitting the prerequisite from the generator (Phase 103) so it's not a hand-maintained island.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hand-edit `reference.md` strings instead of fixing `render.go` | Fix visible in seconds | `--check` flaps or certifies an irreproducible file; bug returns on next regen | **Never** — the generator is the source of truth |
| Keep the `skills/helix/*` wildcard embed, just "remember not to drop files there" | No code change | Next stray file leaks into the binary + every user's disk; relies on human vigilance | **Never** — convert to allowlist + closed-set test |
| Positive-only bundle test ("are SKILL.md + reference.md present") | Quick green | Misses extra-file leak; vacuous gate (the repo's recurring failure class) | Never as the SOLE test — must add a closed-set "no other files" assertion |
| Report DSPy `choice_rate` on the optimize set | One impressive number | Overfit shipped as "improvement"; doesn't generalize | Never for the reported number; fine as an internal training signal |
| Make "re-run DSPy" a CI gate | Feels rigorous | Nondeterministic flap + recurring API cost; breaks single-binary build | **Never** — gate the committed artifact, not the optimizer |
| Wire the DSPy harness behind a `helix` subcommand "for convenience" | One entrypoint | Runtime Python coupling; breaks the single-binary identity | Never — keep it out-of-band dev tooling |
| Split every verb into its own matrix row | Maximal clarity per verb | Token bloat, diluted steering, idle-cost pressure | Only where a row truly conflates query+action |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `cmd/helix-refgen` ↔ daemon | Update daemon blank imports, forget refgen's (or vice-versa) → generated set ≠ live set | Rely on `--check` + `reference ⊇ VerbToolNames()` contract; treat import parity as a checklist item per the in-file reciprocal note |
| `reference.md` ↔ `--check` gate | Hand-edit the artifact | Fix `render.go`, regenerate, commit both together; verify `git diff` empty after regen |
| `installSkill`/`uninstallSkill` | Add allowlist filter to install but not uninstall | Filter the entry list once, feed both; add upgrade-drops-file regression test |
| SKILL.md ↔ `VerbToolNames()` | Cite a renamed/typo'd/removed verb in prose | Cross-check test: every `helix <verb>` in SKILL.md is a real frozen verb (prime.go invariant) |
| DSPy ↔ Helix runtime | Shell to `python -m dspy` from a verb or `setup` | Out-of-band dev tooling only; mechanical `make vet` leakage guard; artifact crosses as static embedded bytes |
| DSPy optimizer ↔ adoption scorecard | Build a new whole-response judge that `Contains("helix")` | Reuse the hardened `test/oracle/adopt` FIRST-command classifier verbatim |

## "Looks Done But Isn't" Checklist

- [ ] **Bundle allowlist:** Often missing the **closed-set** assertion — verify a test fails when a stray file is added to `skills/helix/`, not just that the two expected files are present.
- [ ] **reference.md fix:** Often missing the generator change — verify `cmd/helix-refgen/render.go` was edited and `git diff reference.md` is empty after a fresh regen, not just that the strings look right.
- [ ] **Contract test:** Often vacuous after a format change — verify it asserts exact coverage count (== 50 / `len(VerbToolNames())`) and discriminates a known-absent verb, not `> 0`.
- [ ] **SKILL.md rewrite:** Often missing the idle-cost re-check — verify SKILL-04 ≤1,536-char assertion still passes and every cited verb is real.
- [ ] **Semantic-graph prerequisites:** Often missing — verify the 7 indexed-graph verbs carry the "requires index-semantic-graph" note.
- [ ] **DSPy generalization:** Often missing the held-out number — verify a test-set `choice_rate` is reported, distinct from train/dev, and the gain survives revert-and-fail (`MaterialDrop`).
- [ ] **DSPy reproducibility:** Often missing the artifact/process split — verify the committed SKILL.md is byte-reproducible through `--check` WITHOUT re-running the optimizer, and no CI job invokes DSPy.
- [ ] **No runtime Python:** Often missing the mechanical guard — verify a `make vet`-style analyzer proves the `helix` binary and `setup` path have zero DSPy/Python edge.
- [ ] **uninstall parity:** Often missing — verify upgrading from an old bundle set removes the dropped file from disk.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| SKILL-ISSUE.md / stray file already leaked into a release | MEDIUM | Move file out of `skills/helix/`, add allowlist + closed-set test, cut a patch; `helix setup` re-run overwrites the user dir (uninstall parity removes the orphan) |
| Hand-edited reference.md merged | LOW | Fix render.go, regenerate, commit; `--check` goes green and stays green |
| DSPy overfit artifact committed | LOW-MEDIUM | Revert the SKILL.md to prior committed version (it's static bytes); re-run optimizer with held-out split before re-attempting |
| Runtime Python coupling shipped | HIGH | Rip the Python edge out of the runtime/setup path; re-quarantine to dev tooling; restore single-binary build — expensive because it may have spread |
| Contract test went vacuous | MEDIUM | Add discriminating + exact-count assertions; run RED-first to prove it bites; audit what slipped through while it was vacuous |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. Embed glob ships stray files | Phase 102 (bundle allowlist, EARLY) | Closed-set test: embedded FS == {SKILL.md, reference.md}; `strings helix` clean |
| 2. Allowlist breaks atomic install | Phase 102 | Existing containment + atomic-revert tests green; upgrade-drops-file test |
| 3. Hand-editing generated reference.md | Phase 103 (refgen fixes) | `--check` green on fresh tree; `git diff` empty after regen |
| 4. docgen/refgen import-parity drift | Phase 103 | `--check` + contract suite green after skill-package touch |
| 5. `reference ⊇ VerbToolNames()` goes vacuous | Phase 102/103 boundary | Contract test asserts count == 50 + discriminates known-absent verb; RED-first |
| 6. DSPy overfits tiny dev set | Phase 104+ (DSPy harness) | Held-out test `choice_rate` reported, distinct from train; gain survives MaterialDrop |
| 7. Metric gaming of `choice_rate` | Phase 104+ | Optimization metric = scorecard AND a quality/task-success oracle; manual degenerate-instruction inspection |
| 8. Nondeterministic/expensive optimizer breaks `--check` | Phase 104+ | No CI job runs DSPy; committed artifact byte-reproducible via `--check`; pinned model+seed+temp0+cache |
| 9. Train/dev/test contamination | Phase 104+ | Disjoint splits enforced in code; reuse hardened FIRST-command classifier; sabotage-non-noop assertion |
| 10. Runtime Python coupling | Phase 104+ | `make vet`-style analyzer: zero DSPy/Python edge from helix runtime + setup |
| 11. Row over-split / token bloat | Phase 102 | SKILL-04 ≤1,536-char assertion; row count near 44 |
| 12. Ambiguous / stale "use X not Y" | Phase 102 | SKILL.md↔VerbToolNames cross-check test (prime.go invariant) |
| 13. Missing indexed-graph prerequisites | Phase 102 (prose) / 103 (generator) | 7 semantic verbs carry prerequisite note; generator-emitted to prevent drift |

## Sources

- `internal/cli/skill.go` (`//go:embed skills/helix/*`, `installSkill` two-pass atomic + containment loop over all entries — leak confirmed) — HIGH
- `internal/cli/skill_test.go` (`TestInstallSkillWritesBundle` positive-only; `TestInstallSkillContainment`) — HIGH
- `internal/cli/skills/helix/SKILL-ISSUE.md` (maintainer analysis: 37→44 rows, copy-paste + Output errors, missing prerequisites) — HIGH
- `cmd/helix-refgen/main.go` + `render.go` (`--check` gate, `useThisNotThat(group, verb)` per-group string source, blank-import-parity reciprocal note) — HIGH
- `test/oracle/adopt/scorecard.go` (`MinTasks=5`, `MaterialDrop=0.4`, FIRST-command classifier, `StripDecisionMatrix`/`TestSabotageNonNoop` anti-vacuity pattern) — HIGH
- PROJECT.md v2.2 milestone section + Constraints (single Go binary, no runtime Python; DSPy dev-time only; committed `--check`-reproducible artifact) — HIGH
- MEMORY: "Helix tool docs drift" (docgen missing `internal/skill/semantic` blank import — import-parity trap precedent) — HIGH
- MEMORY: "Helix bench smoke false-green" / Phase 87 CR-01 / Phase 89 CR-01 (vacuous-gate recurring failure class) — HIGH
- [MIPROv2 Optimizer — DSPy](https://dspy.ai/deep-dive/optimizers/miprov2/) and [MIPROv2 | DeepWiki](https://deepwiki.com/stanfordnlp/dspy/4.4-miprov2:-instruction-and-parameter-optimization) (minibatch auto-enabled only when val_size > 50; iterative LLM-proposed instructions; overfit on small sets) — MEDIUM-HIGH
- [Finishing Optimization: Saving and Loading DSPy Programs](https://codesignal.com/learn/courses/how-to-optimize-with-dspy/lessons/finishing-optimization-saving-and-loading-dspy-programs) (compiled program = static loadable artifact distinct from the optimizer run) — MEDIUM
- [Understanding and Mitigating Numerical Sources of Nondeterminism in LLM Inference](https://arxiv.org/pdf/2506.09501) (even temp-0 greedy decoding is not bit-reproducible — FP/GPU-kernel nondeterminism) — HIGH

---
*Pitfalls research for: Helix v2.2 Agent-Facing Skill Quality & Prompt Tuning*
*Researched: 2026-06-23*
