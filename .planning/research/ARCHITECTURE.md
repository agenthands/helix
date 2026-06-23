# Architecture Research

**Domain:** Helix v2.2 — Agent-facing skill quality & offline prompt tuning (Go single binary, dev-time Python harness)
**Milestone:** v2.2 Agent-Facing Skill Quality & Prompt Tuning
**Researched:** 2026-06-23
**Confidence:** HIGH (Go integration points read directly from source; DSPy patterns cross-checked against dspy.ai)

> NOTE: `.planning/codebase/ARCHITECTURE.md` is STALE — it describes the pre-rename Python Serena (2026-04-07), not the current Go `helix`. All integration claims below are grounded in the live Go tree (`cmd/helix-refgen/`, `cmd/helix-cligen/`, `internal/cli/skill.go`, `internal/cli/verb.go`, `test/oracle/adopt/scorecard.go`, `test/oracle/llm/`), not that file.

## Scope

This is a **subsequent-milestone, integrate-WITH** research note. Three deterministic Go-side features plus one exploratory Python harness, all hanging off the EXISTING skill-bundle generation/install/measurement spine. No new runtime dependency, no breaking change (minor version), phase numbering continues from 102.

---

## Existing Architecture (the spine v2.2 hooks into)

```
                         GENERATE-TIME (dev / CI)                         RUNTIME (helix binary)
┌──────────────────────────────────────────────────┐   ┌──────────────────────────────────────────┐
│ live tool registry (skill.ToolProviders())        │   │ //go:embed skills/helix/*                  │
│   ↑ blank-imports (same set as daemon/imports.go)  │   │   → embeddedSkillFS (embed.FS)             │
│                                                    │   │       ├─ SKILL.md  (hand-authored)         │
│ cmd/helix-cligen  ──generates──▶ verbs_gen.go      │   │       ├─ reference.md (generated)          │
│   (verbSpecs: verb→tool, GroupID via              │   │       └─ SKILL-ISSUE.md  ◀── LEAKS today   │
│    categoryToGroup[category])                      │   │                                            │
│       │ --check drift gate (verify-cligen)         │   │ installSkill(targetDir):                    │
│       ▼                                            │   │   ReadDir("skills/helix") → writes EVERY    │
│ cli.VerbSpecsForDocs() / cli.VerbToolNames()       │   │   entry atomically (2-pass stage+rename),  │
│       │  (read-only doc seam: Verb, GroupID,       │   │   withinSkillRoot() containment guard       │
│       │   Short, Flags)                            │   │                                            │
│       ▼                                            │   │ EmbeddedSkillBody()  → SKILL.md bytes       │
│ cmd/helix-refgen/render.go  ──renders──▶           │   │ EmbeddedReference()  → reference.md bytes   │
│   reference.md  (synopsis + Args + Output +        │   │   (multi-agent instruction surface)         │
│    Example + "Use this, not that")                 │   └──────────────────────────────────────────┘
│       │  Output line = outputShape(GroupID)        │
│       │  UseThisNotThat = useThisNotThat(GroupID)  │   MEASUREMENT (the v2.2 optimization metric)
│       ▼                                            │   ┌──────────────────────────────────────────┐
│   --check drift gate (verify-reference, REF-03)    │   │ test/oracle/adopt (NO build tag, hermetic) │
│   contract: reference ⊇ VerbToolNames() (50/50)    │   │   Scorecard([]Bucket) → {ChoiceRate,       │
└──────────────────────────────────────────────────┘   │     FallbackRate, ...}                      │
                                                         │   FirstCommand / ClassifyChoice /          │
                                                         │   StripDecisionMatrix                      │
                                                         │ test/oracle/llm (//go:build llm)           │
                                                         │   live-capture leg → feeds same Scorecard  │
                                                         │   SkillSystemPrompt(body), task corpus     │
                                                         └──────────────────────────────────────────┘
```

### Component responsibilities (verified against source)

| Component | Responsibility | File |
|-----------|----------------|------|
| `verbSpecs` + `categoryToGroup` | verb→tool catalog with `GroupID` derived from ToolProvider category | `cmd/helix-cligen/render.go:60-73`, generated into `internal/cli/verbs_gen.go` |
| `VerbSpecsForDocs()` / `VerbDoc` | read-only doc seam: `Verb, ToolName, GroupID, Short, Flags` (fresh copies, deterministic sort) | `internal/cli/verb.go:103-171` |
| `renderReference()` / `renderVerb()` | render reference.md; **`Output` = `outputShape(d.GroupID)`; "Use this, not that" = `useThisNotThat(d.GroupID, d.Verb)`** | `cmd/helix-refgen/render.go:17-95, 203-242` |
| `--check` gate (`referenceStale`) | byte-compare on-disk vs freshly-rendered; exit 1 if drift | `cmd/helix-refgen/main.go:60-88` |
| `referenceMissingVerbs` contract | reference ⊇ `VerbToolNames()` (50 frozen verbs) | `internal/cli/reference_contract_test.go:32-62` |
| `installSkill` | write embedded bundle to disk, 2-pass atomic, `withinSkillRoot` containment | `internal/cli/skill.go:180-257` |
| `adopt.Scorecard` | choice_rate/fallback_rate over classified transcripts; floor `MinTasks=5`; `MaterialDrop=0.4` | `test/oracle/adopt/scorecard.go:124-156` |
| `test/oracle/llm` live leg | captures real-model transcripts intact vs matrix-stripped, feeds same Scorecard | `test/oracle/llm/adoption_scorecard_test.go` |

---

## Root-cause findings (what actually produces the buggy lines)

### (a) Generator fixes — `Output` and "Use this, not that" are GROUP-keyed, not verb-keyed

The SKILL-ISSUE.md "copy-paste errors" are NOT hand-edit mistakes in reference.md — **reference.md is generated**, and the generator keys those two lines off `GroupID` only:

- `cmd/helix-refgen/render.go:204-220 outputShape(group)` — a 7-arm switch on group; `memory` → `"the memory body or a ranked FTS5 search result set, one entry per line."`
- `cmd/helix-refgen/render.go:224-242 useThisNotThat(group, verb)` — `memory` → `"…for durable project/session memory instead of ad-hoc scratch notes."`

**The leak:** `cmd/helix-cligen/render.go:68-72` collapses FOUR distinct ToolProvider categories into the single `memory` group:
```
"memory"   → groupMemory
"workflow" → groupMemory   // onboard-project, prepare-for-new-conversation
"health"   → groupMemory   // get-health
"help"     → groupMemory   // get-tool-help
"profile"  → groupMemory   // switch-mode, get-token-budget
```
So `switch-mode`, `get-token-budget`, `onboard-project`, `prepare-for-new-conversation`, `get-health`, `get-tool-help`, AND the mutating memory verbs (`write/edit/rename/delete-memory`) all inherit the memory-query Output + "durable memory" steering text. Every SKILL-ISSUE.md finding §4/§5 is one symptom of this single grouping collapse.

**Fix shape (deterministic, contract-preserving):** keep both lines **table-driven, generated**. Two viable seams:
1. **Per-verb override map keyed in the generator** — add `outputShapeFor(verb, group)` / `useThisNotThatFor(verb, group)` that consult a `map[verb]string` first, falling back to the group default. Smallest blast radius; lives entirely in `cmd/helix-refgen/render.go`.
2. **Split the group taxonomy upstream** — give `categoryToGroup` finer groups (`memory-query` vs `memory-mutate`, `workflow`, `session`, `health`, `help`) in `cmd/helix-cligen/render.go`, add matching `cobra.Group`s in `internal/cli/root.go`, and extend the `outputShape`/`useThisNotThat` switches. Cleaner conceptually but touches the cobra root command grouping (and the SKILL.md footer "grouped by capability" claim).

**Recommendation: option 1 (per-verb override map in refgen).** It is the minimal change that (i) keeps `GroupID` and the cobra root grouping untouched, (ii) keeps everything generated (no hand-edits to reference.md), and (iii) is trivially deterministic. Option 2 is a larger, riskier refactor better deferred unless SKILL.md regrouping (feature 1) independently demands new cobra groups.

**Determinism & contract invariants the fix MUST preserve:**
- `renderReference()` is already deterministic (sorted `VerbSpecsForDocs()` iteration, map-free output assembly) — any override map must be a literal `map[string]string` read in sorted-verb order, never iterated for output. ✓ trivially satisfied since lookups are keyed, not ranged.
- `--check` (`referenceStale`, `main.go:82`) stays a pure byte-compare; regenerate-then-commit is the workflow, exactly as today.
- `reference ⊇ VerbToolNames()` (`reference_contract_test.go`) is unaffected — the override only changes Output/UseThisNotThat PROSE, never whether a verb section exists. Existing `TestRenderCoversAllVerbs` + `TestReferenceCoversAllVerbs` keep passing.
- Add a **vacuity guard**: a test asserting NO override key is a non-existent verb and that each overridden verb's rendered Output differs from its old group default (so a future verb rename can't silently re-stale the override).

### (b) installSkill hardening — `ReadDir` writes EVERYTHING; the allowlist is a filter pass

`installSkill` (`skill.go:189, 212-236`) does `embeddedSkillFS.ReadDir("skills/helix")` and stages/writes **every non-dir entry**. Today `skills/helix/` contains `SKILL.md`, `reference.md`, AND `SKILL-ISSUE.md` (the 18 KB maintainer analysis). So:
- `SKILL-ISSUE.md` is **embedded into the binary** (via `//go:embed skills/helix/*`) and **installed to every user's `.claude/skills/helix/`** on `helix setup`.
- `EmbeddedReference()` is safe (reads `reference.md` by name), but the bundle install + `uninstallSkill` both walk the whole dir.

**Fix shape:** introduce a single allowlist constant and filter the `ReadDir` loop in BOTH `installSkill` and `uninstallSkill`:
```go
// bundleFiles is the EXACT set shipped to disk. Any other entry in skills/helix/
// (e.g. SKILL-ISSUE.md maintainer notes) is embedded-but-never-installed.
var bundleFiles = map[string]bool{"SKILL.md": true, "reference.md": true}
```
Filter `if !bundleFiles[e.Name()] { continue }` inside the existing loops. This:
- **Does NOT touch** the 2-pass stage→rename atomicity (`pending []staged`, the rename-revert path at `skill.go:242-255`) — the filter runs BEFORE staging, so the all-or-nothing property over the *allowlisted* set is preserved.
- **Does NOT touch** `withinSkillRoot` / `lexicalSkillRoot` / `containedIn` containment (`skill.go:273-306`) — orthogonal path-traversal guard, still runs first.
- Entry names still come ONLY from the embedded FS (the T-97-01 posture holds; the allowlist just narrows it further).

**Bundle-contents test (the new gate):** a hermetic `_test.go` in `internal/cli` that walks `embeddedSkillFS` (or installs into a `t.TempDir()` via `installSkill`) and asserts the installed set is EXACTLY `{SKILL.md, reference.md}`. This is the regression that fails if a future stray `.md` (issue notes, scratch) lands in the embed dir. Pairs naturally with moving `SKILL-ISSUE.md` OUT of `skills/helix/` (it is planning/maintainer content, belongs under `.planning/` or a `docs/` sibling) — but the allowlist makes the bundle robust even if it stays.

**Order note:** the allowlist (feature 3) is independent of the generator fix (feature 2) and the SKILL.md rewrite (feature 1) — it can land first as a pure safety patch.

### (c) DSPy harness — where it lives, how it reads the metric, how output re-enters --check

**Where (new component, isolated):** a dev-time/offline Python tree that is NOT part of the Go module and NOT embedded. Recommended `tools/skill-tune/` (sibling to `cmd/`, `bench/`, `test/`) with its own `pyproject.toml`/`requirements.txt`, a `README.md` stating "dev-time only, never shipped, no runtime dependency," and ignored from the Go build (Go ignores non-`.go` dirs automatically; add to `.gitignore` only for venv/artifacts, NOT the source). This mirrors how `bench/` already hosts dev-time corpora and how `eval/` hosts attestation tooling — Helix already has the "dev-time non-Go subtree" precedent.

**How it consumes the Scorecard as its METRIC (data flow):** DSPy optimizers (MIPROv2 / GEPA) take `(program, dataset, metric)` and search instructions to maximize the metric (dspy.ai). The Helix optimization metric is `adopt.Scorecard.ChoiceRate` (higher = more `helix` adoption, lower fallback). Two seams to expose it to Python:

1. **Pure-text classifier parity (preferred):** the DSPy metric must classify a model response with the SAME logic as `adopt.ClassifyChoice` / `adopt.FirstCommand` (first-command-prefix, `fallbackPrefixes = {grep,sed,cat,find,rg,ls}`). Re-implement that ~15-line classifier in Python AND pin it to the Go source of truth with a **golden cross-check**: a committed fixture of `(response, chose, fellBack)` triples that BOTH `test/oracle/adopt` (Go) and the Python metric must agree on. This keeps the Go scorecard the single authority while letting DSPy run offline without CGO/subprocess-into-Go.
2. **(Alternative) shell out to a tiny Go scorer:** add a `cmd/helix-score` (or `helix-bench score`) subcommand that reads transcripts on stdin and prints `choice_rate`, and have the DSPy metric invoke it. Heavier (process per eval), but zero classifier duplication. Defer unless parity-drift becomes a real problem.

The DSPy program's **dataset** is the existing task corpus: `test/oracle/llm/prompt.go SkillTaskDescriptions()` (the helix-appropriate code tasks) + `SkillSystemPrompt(body)` template. The optimizer's "instructions" being tuned ARE the SKILL.md decision-matrix / steering text.

**How optimized output re-enters the Go --check-gated surface (the critical loop closure):**

```
tools/skill-tune/ (offline, manual, gated by API key — NEVER in CI gate)
  DSPy program (signature: task → command)
  + dataset = SkillTaskDescriptions()
  + metric  = choice_rate (parity classifier)
       │ optimizer.compile() → search instructions
       ▼
  optimized_program.save("optimized.json")   ← DSPy artifact (gitignored or archived, NOT shipped)
       │  human extracts the improved decision-matrix / steering prose
       ▼
  HUMAN edits internal/cli/skills/helix/SKILL.md   (hand-authored — DSPy PROPOSES, human COMMITS)
  and/or the refgen Output/UseThisNotThat override map (feature 2)
       │
       ▼
  go run ./cmd/helix-refgen        ← regenerate reference.md from the (possibly tuned) registry/templates
  go run ./cmd/helix-refgen --check  +  go test ./...  (adopt + contract gates)
       │  --check byte-reproducible, reference ⊇ VerbToolNames() still 50/50
       ▼
  git commit  →  the committed, deterministic surface
```

**Key invariant:** DSPy output is a SUGGESTION fed back through the existing hand-authored SKILL.md / generated-reference pipeline; it NEVER writes reference.md directly (that would break `--check` determinism and the generated-from-registry contract). The DSPy `optimized.json` is a dev artifact, not a shipped file. This keeps "Helix stays a Go single binary, no Python runtime dep" (PROJECT.md L192) literally true: nothing in `tools/skill-tune/` is imported, embedded, or invoked by the binary or the CI gate.

---

## New vs Modified components (explicit)

| Component | New / Modified | What |
|-----------|----------------|------|
| `internal/cli/skills/helix/SKILL.md` | **Modified** | decision-matrix rewrite (split query/action rows, "Not this" everywhere, indexed-graph prereq notes) — hand-authored; must keep `## Decision matrix` heading (the `StripDecisionMatrix` anchor) and stay under the 1,536-char idle-cost cap (SKILL-04) |
| `cmd/helix-refgen/render.go` | **Modified** | add per-verb override maps for `Output` + "Use this, not that"; group-default fallback retained |
| `cmd/helix-refgen/*_test.go` | **New tests** | vacuity guard (no override key is a non-verb; overridden Output ≠ old group default) |
| `internal/cli/skills/helix/reference.md` | **Regenerated** | not hand-edited — output of `go run ./cmd/helix-refgen` after the override change |
| `internal/cli/skill.go` | **Modified** | add `bundleFiles` allowlist; filter `installSkill` + `uninstallSkill` ReadDir loops |
| `internal/cli/skill_bundle_test.go` | **New** | hermetic bundle-contents test: installed set == `{SKILL.md, reference.md}` |
| `internal/cli/skills/helix/SKILL-ISSUE.md` | **Moved (recommended)** | out of the embed dir to `.planning/` — the allowlist makes this optional but cleaner |
| `cmd/helix-cligen/render.go` `categoryToGroup` | **Modified ONLY IF** option 2 chosen (finer groups) — NOT recommended for v2.2 | — |
| `tools/skill-tune/` (Python: `pyproject.toml`, `program.py`, `metric.py`, `dataset.py`, `README.md`) | **New** | exploratory DSPy harness; dev-time only, not in Go module, not in CI gate |
| `tools/skill-tune/testdata/classifier_parity.json` + Go cross-check | **New** | golden fixture asserting Python metric ≡ `adopt.ClassifyChoice` |
| `test/oracle/adopt` | **Unchanged** | remains the single source of truth for the metric; Python mirrors it |
| `installSkill` 2-pass atomicity + `withinSkillRoot` | **Unchanged** | allowlist is a pre-filter; both invariants preserved |

---

## Suggested build order (phases 103+)

Dependency-driven: deterministic safety + correctness FIRST, exploratory DSPy LAST (it consumes the rewritten surface and the metric, both of which should be stable before tuning against them).

| Phase | Feature | Rationale / deps |
|-------|---------|------------------|
| **103** | installSkill allowlist + bundle-contents test (+ move SKILL-ISSUE.md out of embed dir) | Pure safety patch, **zero deps**, smallest blast radius. Stops the SKILL-ISSUE.md leak immediately and gives later phases a clean embed dir. Preserves 2-pass atomicity + containment. |
| **104** | reference.md generator fixes (per-verb Output / "Use this, not that" override map in `cmd/helix-refgen`) + vacuity tests + regenerate | Fixes the §4/§5 copy-paste errors at the GENERATOR (root cause), not by hand-edit. Must land BEFORE the SKILL.md rewrite so the generated reference is already correct when the matrix is re-authored. Preserves `--check` byte-reproducibility and `reference ⊇ VerbToolNames()`. |
| **105** | SKILL.md decision-matrix rewrite (split query/action, "Not this" everywhere, regroup, indexed-graph prereqs) | Hand-authored. Keep `## Decision matrix` heading (StripDecisionMatrix anchor) + idle-cost cap (SKILL-04). The hermetic `test/oracle/adopt` MaterialDrop test re-validates the rewritten matrix still materially drives adoption. Depends on 104 (reference prose already correct) so SKILL + reference tell ONE consistent story. |
| **106 (exploratory)** | DSPy offline tuning harness (`tools/skill-tune/`) + Python↔Go classifier parity golden | LAST: consumes the stabilized metric (`adopt.Scorecard`) and the rewritten surface from 104/105. Dev-time only, no runtime dep, output re-enters via human → SKILL.md/refgen → `--check`. Marked exploratory: success = a reproducible harness + a parity-pinned metric, NOT a hard adoption-delta gate (live models are nondeterministic, mirroring the informational `//go:build llm` leg). |

**Ordering invariants:**
- 103 before 104/105 → clean embed dir, no stray-file noise in the bundle-contents test or the generated reference.
- 104 (generator/reference) before 105 (SKILL.md) → the on-demand reference is already correct when the idle-tier matrix is re-authored, avoiding a window where SKILL.md and reference.md disagree.
- 106 strictly last → DSPy tunes AGAINST a frozen metric and a stable surface; tuning against a moving target wastes optimizer budget and muddies attribution.

---

## Pitfalls / risks for downstream phases

| Risk | Mitigation |
|------|------------|
| Treating reference.md "copy-paste errors" as hand-edits | They are GENERATED — fix `outputShape`/`useThisNotThat` in `cmd/helix-refgen`, never edit reference.md by hand (a hand-edit fails `--check`). |
| Override map iterated for output → non-determinism | Lookups are KEYED, never ranged; render order stays the sorted `VerbSpecsForDocs()` walk. Determinism test already exists (`TestRenderDeterministic`). |
| Allowlist filter placed after staging → breaks atomicity reasoning | Filter BEFORE the stage loop so all-or-nothing is over the allowlisted set; do not touch the rename-revert path. |
| SKILL.md rewrite renames `## Decision matrix` heading | Breaks `adopt.StripDecisionMatrix` anchor (silent vacuity). Keep the literal heading; the `TestSabotageNonNoop` guard catches a rename. |
| SKILL.md rewrite blows the 1,536-char idle-cost cap | SKILL-ISSUE.md estimates +7 rows ≈ +51 bytes to frontmatter; the cap applies to the FRONTMATTER description only (`skillDescription()`), not the body — verify with the existing SKILL-04 unit test. |
| DSPy harness drifts from the Go classifier | Pin with a committed `(response → chose/fellBack)` golden cross-checked by BOTH Go (`test/oracle/adopt`) and Python; do NOT let DSPy invent its own classifier. |
| DSPy output written to reference.md / embedded | Forbidden — DSPy PROPOSES prose; human commits to SKILL.md / refgen overrides; output re-enters only through `--check`. Keep `tools/skill-tune/` out of the Go module and CI gate. |

## Sources

- Live Go source (read directly): `cmd/helix-refgen/{main,render}.go`, `cmd/helix-cligen/render.go`, `internal/cli/{skill,verb}.go`, `internal/cli/skills/helix/{SKILL.md,SKILL-ISSUE.md}`, `test/oracle/adopt/scorecard.go`, `test/oracle/llm/{adoption_scorecard_test,prompt}.go`, `internal/cli/reference_contract_test.go`, `Makefile`, `.github/workflows/go-test.yml`. Confidence: HIGH.
- [GEPA optimization — DSPy](https://dspy.ai/getting-started/gepa-optimization/)
- [MIPROv2 — DSPy](https://dspy.ai/api/optimizers/MIPROv2/)
- [DSPy cheatsheet (compile / save program JSON)](https://dspy.ai/cheatsheet/)
