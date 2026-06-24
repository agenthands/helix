# Phase 106: Exploratory DSPy Offline Tuning Harness (spike) - Research

**Researched:** 2026-06-24
**Domain:** Offline prompt optimization (DSPy) + Go↔Python scorer parity + go/analysis leakage gate + single-binary quarantine
**Confidence:** HIGH (codebase anchors all read first-hand and verified; DSPy API confirmed via Context7 against DSPy 3.1.3)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
This is a SPIKE — exploratory; a **no-ship conclusion is a legitimate, success-meeting outcome** (NOT a hard adoption-delta gate). Binding success criteria (from ROADMAP):

1. The DSPy harness lives under `tools/` as an opt-in dev-time tree (its own pinned `requirements.txt` + git-ignored venv/output), is excluded from `go test ./...`, and adds NO runtime Python dependency to the `helix` binary or `helix setup`.
2. The harness optimizes the agent-facing skill/steering text against the Phase 101 adoption metric, re-implemented in Python with a **golden parity cross-check** against the Go `test/oracle/adopt` classifier (the same `choice_rate`/`fallback_rate` scorer drives both the Go gate and the Python optimizer).
3. Overfit and metric-gaming guards are in place — a **held-out TEST split** the optimizer never sees, and a **degenerate-steering inspection** — and the harness may legitimately conclude no-ship (the clean fallback being a hand-rolled Go candidate-search loop keeping the milestone 100% Go).
4. Any adopted output re-enters only as a **human-reviewed commit through SKILL.md/refgen** and passes `helix-refgen --check`; a `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path (the artifact is gated, never the optimizer process).

### Milestone-wide invariant (enforced here)
**No runtime Python / single-binary preserved:** DSPy is dev-time/offline only — no `helix` subcommand shells to Python, no `go.mod`/`helix setup` edge, off the default `go test ./...` / merge path. **Gate the committed ARTIFACT, never the optimizer PROCESS** (LLM optimization isn't bit-reproducible).

### Claude's Discretion
All implementation choices — discuss phase was skipped. Use ROADMAP phase goal, success criteria, and codebase conventions.

### Deferred Ideas (OUT OF SCOPE)
- **TUNE-FUT-01**: upgrade DSPy optimizer from GEPA to MIPROv2/COPRO with a larger held-out set if GEPA's prose evolution proves insufficient (gated on `val_size > 50`).
- **TUNE-FUT-02**: a quality-joined optimization metric (adoption choice-rate AND task-success via the v2.1 Aider-derived benches) instead of adoption alone.
- Runtime Python / shipping DSPy in the binary, `helix setup`, or the default suite.
- Hand-editing generated `reference.md`.
- New Go module dependencies.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TUNE-01 | Opt-in, dev-time-only DSPy harness under `tools/` (gitignored output, excluded from `go test ./...`, no runtime Python dep in `helix` binary or `helix setup`) optimizes the agent-facing skill/steering text against the Phase 101 adoption scorecard metric (Python re-implementation parity-checked against Go `adopt` classifier), with overfit/metric-gaming guards; may conclude no-ship; any adopted output passes `helix-refgen --check`; a `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path. | §Standard Stack (DSPy 3.1.3 + GEPA), §Parity Contract, §Leakage Analyzer, §Corpus Split, §Artifact Re-Entry Gate, §Validation Architecture |
</phase_requirements>

## Summary

This phase quarantines a dev-time DSPy prompt optimizer in a new `tools/` Python tree while the shipped Go binary stays 100% Python-free. The entire phase pivots on **one small Go file already in the tree**: `test/oracle/adopt/scorecard.go` (157 LOC, build-tag-free, hermetic). Its classifier — `FirstCommand` (first non-blank line, fence/backtick/prompt-stripped) → `ClassifyChoice` (prefix `"helix "` = choice; prefix in `{grep ,sed ,cat ,find ,rg ,ls }` = fallback; else unclassified) → `Scorecard` (rejects buckets `< MinTasks=5`; reports `ChoiceRate`/`FallbackRate`) — is the **exact algorithm** the Python optimizer must re-implement at golden parity. The algorithm is small and fully specified by string operations, so byte/semantic parity is achievable and provable with shared JSON fixtures.

The dominant risk is **vacuity** (the named Phase 86/87/89 CR-01 class, re-stated in the v2.2 roadmap constraints): a parity check that only ever goes green, a leakage analyzer that flags nothing, an overfit guard with no held-out data. Every gate this phase adds MUST ship a deliberate break-the-invariant → assert-RED test. The codebase already provides the templates: `internal/lint/ablationleakage` (a `go/analysis` import-boundary analyzer with `testdata/` RED+GREEN fixtures and a slash-boundary lookalike guard) is the leakage-analyzer model; `test/oracle/adopt/scorecard_test.go` (sabotage-revert-and-fail, empty-bucket-rejected, non-substring) is the anti-vacuity model for the scorer.

**Primary recommendation:** Build a `tools/dspy-tune/` Python tree (pinned `requirements.txt` with `dspy==3.1.3`), re-implement the `adopt` classifier in `scorer.py`, prove parity via a **shared golden corpus** (`tools/dspy-tune/golden/parity_cases.json` listing `{response, expected_choice, expected_fallback}` cases) asserted by BOTH a Python `pytest` and a Go test in `test/oracle/adopt`. Use DSPy's **GEPA** optimizer (reflective prompt evolution — the right tool for evolving a single instruction string against a scalar metric) over a held-out TRAIN/TEST split (split is the FIRST harness task). Add a `cmd/vet-tools-quarantine` go/analysis analyzer (modeled on `ablationleakage`) asserting no runtime package imports `tools/` and `go.mod` carries no Python edge. Expect — and document as success — a likely **no-ship** given `MinTasks=5` is far too small for a trustworthy adoption delta; the clean fallback is a hand-rolled Go candidate-search loop.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Adoption metric (choice/fallback classification) | Go test oracle (`test/oracle/adopt`) | Python re-impl (`tools/dspy-tune/scorer.py`) | Go is the single source of truth + merge gate; Python is a parity-pinned mirror for the optimizer only |
| Prompt optimization loop | Dev-time Python (`tools/dspy-tune/`) | — | DSPy is offline/dev-only; never linked into the binary |
| Parity enforcement | Shared golden corpus + Go test + pytest | — | Both languages assert against the same committed fixtures |
| Leakage prevention | Go `make vet` analyzer (`cmd/vet-tools-quarantine`) | go test exclusion (no `.go` under `tools/`) | Static compile-time boundary; the runtime/merge path must never reach `tools/` |
| Artifact re-entry | `cmd/helix-refgen --check` + human commit to SKILL.md | — | Adopted text re-enters only through the generated/hand-authored surface, never a raw optimizer dump |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `dspy` | `3.1.3` `[VERIFIED: Context7 /websites/dspy_ai, latest indexed version 3.1.3]` | Declarative prompt optimization framework; ships `GEPA`, `MIPROv2`, `COPRO` optimizers | The canonical Stanford NLP framework for optimizing prompts/instructions against a scalar metric; GEPA is purpose-built for reflective single-prompt evolution |
| `pytest` | pin latest (e.g. `8.x`) `[ASSUMED]` | Python test runner for the parity cross-check + scorer unit tests | Standard Python test harness; the harness must self-verify its scorer against the golden corpus |
| `golang.org/x/tools/go/analysis` | `v0.43.0` (already in `go.mod`) `[VERIFIED: go.mod grep]` | The leakage `make vet` analyzer | Already the substrate for all 6 existing `vet-*` analyzers (`noduckdb`, `ablationleakage`, etc.) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `litellm` | transitive of `dspy` `[CITED: dspy.ai LM config tutorials]` | DSPy's LM backend abstraction (`dspy.LM(model="openai/...")`) | Configuring the dev-time LLM backend; pulled in by `dspy`, no separate pin needed unless a local model is targeted |

### DSPy optimizer selection (GEPA vs MIPROv2 vs COPRO)
| Optimizer | Fit for this phase | Verdict |
|-----------|--------------------|---------|
| **GEPA** | Reflective prompt evolution: mutates the instruction *text* using LLM-generated feedback, tracks Pareto scores on a valset. Purpose-built for evolving a single prose instruction against a scalar metric with small data. `[CITED: dspy.ai/api/optimizers/GEPA]` | **RECOMMENDED** (matches REQUIREMENTS deferred note "upgrade from GEPA" — GEPA is the starting point) |
| MIPROv2 | Bayesian joint optimization of instructions + few-shot demos; `auto={light,medium,heavy}`. `[CITED: dspy.ai/api/optimizers/MIPROv2]` | Deferred (TUNE-FUT-01) — heavier, wants more data |
| COPRO | Coordinate-ascent instruction-only refinement. `[CITED: dspy.ai]` | Deferred (TUNE-FUT-01) alternative |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| DSPy GEPA harness | **Hand-rolled Go candidate-search loop** (the documented clean fallback) | Keeps the milestone 100% Go, no Python tree at all; but is the FALLBACK only if DSPy proves not worth the quarantine cost or the spike concludes no-ship. The spike's job is to *find out*, so DSPy is built first. |
| `dspy==3.1.3` pin | unpinned `dspy` | Reproducible dev-time runs require an exact pin in `requirements.txt`; LLM *outputs* aren't bit-reproducible (hence "gate the artifact, not the process") but the *toolchain* must be pinned |

**Installation (dev-time only — NEVER in the binary or `helix setup`):**
```bash
# tools/dspy-tune/requirements.txt (pinned)
#   dspy==3.1.3
#   pytest==8.3.4   # or current
cd tools/dspy-tune && python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt
```

**Version verification:** `dspy==3.1.3` is the latest version indexed by Context7 (`/stanfordnlp/dspy` → `Versions: 3.1.3`). `[VERIFIED: Context7 ctx7 library dspy]`. The planner should run `pip index versions dspy` at plan time to confirm 3.1.3 is still current (DSPy is fast-moving — valid-until 7 days).

## Package Legitimacy Audit

> The phase installs Python packages into a **dev-time, git-ignored venv only**. They never enter the binary, `go.mod`, or the merge path, so the runtime supply-chain surface is unchanged. Pins still matter for reproducible dev runs.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `dspy` | PyPI | ~2 yrs (Stanford NLP) | high | github.com/stanfordnlp/dspy | [OK] `[ASSUMED — verify with `pip index versions dspy` at plan time]` | Approved (dev-time only) |
| `pytest` | PyPI | 10+ yrs | very high | github.com/pytest-dev/pytest | [OK] `[ASSUMED]` | Approved (dev-time only) |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*`dspy` and `pytest` were surfaced via Context7 (authoritative for `dspy`) and training data. The planner should run `gsd-tools query package-legitimacy check --ecosystem pypi dspy pytest` and `pip index versions dspy` to confirm before pinning. Because these live in a git-ignored dev venv outside the binary/module/merge path, a `checkpoint:human-verify` is advisory, not a runtime supply-chain gate.*

## The Phase 101 Adopt Classifier — Exact Algorithm (parity target)

**Source of truth:** `test/oracle/adopt/scorecard.go` (build-tag-free, hermetic, runs in `go test ./...`). Read first-hand. The Python re-implementation must match this **exactly**:

1. **`FirstCommand(response string) string`** — iterate `strings.Split(response, "\n")`; for each line: `TrimSpace`; skip if empty OR `HasPrefix("```")`; then `Trim(line, "`")` → `TrimSpace` → `TrimPrefix("$ ")` → `TrimPrefix("> ")`; return `TrimSpace` of the first surviving line. Returns `""` if none.
2. **`ClassifyChoice(response) (chose, fellBack bool)`** — `cmd := strings.ToLower(FirstCommand(response))`; `chose = HasPrefix(cmd, "helix ")`; `fellBack = true` iff `cmd` has any prefix in `fallbackPrefixes = {"grep ", "sed ", "cat ", "find ", "rg ", "ls "}` (trailing space is load-bearing — keys on the invocation token, so `"lsp"` is NOT a fallback). Prose/unrecognized first line → `(false, false)`.
3. **`Scorecard(buckets []Bucket) (ScorecardResult, error)`** — if `len(buckets) < MinTasks (=5)` → **error + zero result** (0/0 must NEVER read as `choice_rate=1.0` — Phase 87 CR-01). Else count choices/fallbacks/unclassified; `ChoiceRate = choices/total`, `FallbackRate = fallbacks/total`, `Total = len(buckets)`.
4. **`StripDecisionMatrix(skill string)`** — removes the `## Decision matrix` section (heading → next `\n## ` or EOF); returns input unchanged if heading absent (defensive no-op, externally guarded by `TestSabotageNonNoop`). This is the **sabotage transform** the metric-discrimination proof depends on.

**Key invariants the Python port MUST preserve (each a Pitfall):**
- Classification keys on the FIRST command PREFIX via `HasPrefix(FirstCommand(...))`, **never `strings.Contains` over the whole response** — the injected SKILL.md is full of the word "helix"; a Contains check would inflate `choice_rate` (101-RESEARCH Pitfall 2; `TestFirstCommandNotSubstring`).
- `ChoiceRate + FallbackRate == 1.0` only when every response is classifiable; an `Unclassified` bucket (prose / unlisted first command) makes the sum `< 1.0` observable (IN-02; `TestComplementary`).
- Sub-`MinTasks` buckets are an ERROR, not a pass.
- Lowercasing happens once on the extracted command, not the whole response.

**How the Go gate consumes it (two legs):**
- **Hermetic leg (default suite):** `test/oracle/adopt/scorecard_test.go` — `TestSabotagedSkillRevertAndFail` loads committed `testdata/transcripts/{intact,sabotaged}-*.json` (6 each, `Response` field only), asserts `intact.ChoiceRate - sabotaged.ChoiceRate >= MaterialDrop (=0.4)` ("scorecard measures nothing" if not). This is the SOLE authoritative non-vacuity proof.
- **Live leg (`//go:build llm`, invisible to `go test ./...`):** `test/oracle/llm/adoption_scorecard_test.go` drives a real model with intact vs `adopt.StripDecisionMatrix(body)` system prompts, feeds BOTH buckets to the SAME `adopt.Scorecard` (single source of truth), requires `len(tasks) >= adopt.MinTasks`.

## The Parity Contract Design

The Python optimizer's metric wraps a Python re-implementation of the scorer. To prove the two scorers agree, structure a **shared golden corpus** asserted from both sides:

```
tools/dspy-tune/golden/parity_cases.json     # the single shared fixture
  [
    { "response": "helix go-to-definition --symbol NewClient", "first_command": "helix go-to-definition --symbol newclient", "chose": true,  "fell_back": false },
    { "response": "grep -r CallTool .",                        "first_command": "grep -r calltool .",                       "chose": false, "fell_back": true  },
    { "response": "The helix tool would help, but let me grep.\ngrep -rn 'x' .", "first_command": "the helix tool would help, but let me grep.", "chose": false, "fell_back": false },  # substring trap
    { "response": "```\n$ helix find-references --symbol X\n```", ... },   # fence + prompt strip
    { "response": "lsp restart", "chose": false, "fell_back": false },     # "ls "-prefix lookalike: NOT a fallback
    ...
  ]
```

**Both sides assert against the SAME file:**
- **Go side** (`test/oracle/adopt/parity_test.go`, new): unmarshal `parity_cases.json`, for each case assert `ClassifyChoice(response) == (chose, fellBack)` AND `FirstCommand(response) == first_command`. This pins the golden corpus to the Go truth.
- **Python side** (`tools/dspy-tune/test_parity.py`, pytest): import `scorer.classify_choice` / `scorer.first_command`, assert identical results over the same JSON.

Because BOTH read the one committed file, agreement on the corpus ⇒ semantic parity on every listed case.

**Anti-vacuity guard (REQUIRED — the named CR-01 class):** ship a **planted-divergence test** proving a deliberately-wrong Python classifier FAILS parity. Concretely, `tools/dspy-tune/test_parity.py` includes a test that runs a `_broken_classify` variant (e.g. one using `"helix" in response` substring instead of `first_command` prefix) over the corpus and asserts it produces a DIFFERENT result on the substring-trap case — proving the corpus discriminates a real classifier from a broken one. Without a case that the broken classifier gets wrong, the parity check is vacuous. The substring-trap and the `"ls "`-lookalike cases are the load-bearing discriminators (mirror `TestFirstCommandNotSubstring`).

**Corpus must include at least one case per branch:** helix-choice, each of the 6 fallback prefixes, an unclassified-prose case, a fence-wrapped case, a `$ `/`> ` prompt case, the substring trap, and the `lsp`/`ls`-lookalike. Coverage of every classifier branch is what makes a planted bug detectable.

## The Corpus / Held-Out TEST Split

**Hard constraint:** `MinTasks=5`. The committed bucket is 6 intact + 6 sabotaged transcripts. This is **far too small for a statistically trustworthy adoption delta** — this is the central reason a **no-ship is the expected, legitimate outcome**, and the spike's honest job is to demonstrate *why*.

**Split design (the FIRST harness task per Spike discipline):**
- The held-out **TEST split the optimizer never sees** must be created and locked *before* any optimization runs, or overfit is undetectable. Recommended structure: a separate `tools/dspy-tune/data/` with `train.jsonl` / `test.jsonl`, where TEST is sequestered and the GEPA `valset`/`trainset` are drawn ONLY from TRAIN.
- With a corpus this small, a meaningful TEST split is hard: reserving even 2–3 held-out tasks leaves GEPA almost no signal. Document this honestly as a **finding**, not a defect: the corpus must grow (TUNE-FUT-01 gates on `val_size > 50`) before tuning can beat a deterministic baseline with confidence.
- The harness should **report** the train/test sizes and the TEST-split `choice_rate` of (a) the deterministic baseline SKILL.md and (b) the GEPA-optimized text, and **conclude no-ship** unless the optimized TEST `choice_rate` beats baseline by a material margin on data the optimizer never saw (mirror `MaterialDrop=0.4` discipline as a reporting threshold, not a hard CI gate — the spike is informational).
- **Metric-gaming guard:** pair `choice_rate` with a **degenerate-steering inspection** — flag optimized text that wins `choice_rate` by degenerate means (e.g. instructing the model to ALWAYS emit `helix` regardless of task, or text that collapses to a single verb). A held-out qualitative inspection step (and ideally a quality/correctness oracle per TUNE-FUT-02) defends the gameable first-command proxy. Document the inspection as a checklist gate in the harness output.

## The Leakage Analyzer (`make vet`-style)

**Template:** `internal/lint/ablationleakage/analyzer.go` + `cmd/vet-ablation-leakage` + `analyzer_test.go` with `testdata/src/...` RED/GREEN fixtures. Read first-hand. Model the new analyzer on it exactly.

**Design — `cmd/vet-tools-quarantine` + `internal/lint/toolsquarantine`:**
- **Forbidden edge:** any package OUTSIDE `tools/` (i.e. the runtime/merge path — `internal/...`, `cmd/...` except the analyzer itself) MUST NOT import any package rooted at `github.com/agenthands/helix/tools/...`. (Note: `tools/` here is a *Python* tree with no Go packages, so the import edge is the belt; the analyzer makes the boundary load-bearing and future-proof against a stray `.go` file under `tools/`.)
- **Match form:** exact-package OR slash-suffix subpath, copying the slash-boundary discipline (`path == forbidden || HasPrefix(path, forbidden+"/")`) so a lookalike sibling like `internal/toolsupport` is NOT flagged (mirror `TestAnalyzer_AllowsSlashBoundaryLookalike`).
- **Optional secondary checks (consider, but keep precise to avoid false-positives):**
  - **No DSPy/optimizer shell-out in runtime code:** scan runtime packages for `exec.Command`/`exec.CommandContext` whose first arg literal is `python`/`python3`/`dspy`/`pip install dspy` *in service of tuning*. **CRITICAL FALSE-POSITIVE TRAP:** `internal/langregistry/installer.go` ALREADY legitimately shells out to `pip`/`pipx` to install **language servers** (pyright etc.) — the analyzer MUST NOT flag this. Scope any shell-out check narrowly to optimizer/DSPy literals, or omit it and rely on the import-boundary + a `go.mod`-untouched check instead. `[VERIFIED: grep exec.Command internal/langregistry/installer.go:119,129]`
  - **`go.mod` carries no Python/DSPy edge:** trivially true (Go can't depend on a PyPI package); a one-line test asserting no `dspy`/python module path in `go.mod` is cheap insurance but low-value.

**Anti-vacuity guard (REQUIRED):** `testdata/src/github.com/agenthands/helix/internal/leakyruntime/imports.go` — a fixture runtime package that imports `.../tools/dspytune` with a `// want` directive → analyzer MUST flag it (RED fixture). A sibling GREEN fixture under `tools/` (or a runtime package importing only permitted packages) with NO `// want` directive → analyzer MUST stay silent (`analysistest` fails on any spurious diagnostic). Plus the slash-boundary lookalike fixture. The deliberate violation lives ONLY under `testdata/` (the go tool ignores `testdata/`), so `make vet` on the real tree stays green. This is the planted-leak → analyzer-trips proof; a green-only analyzer is presumed broken.

**Wiring:** add `cmd/vet-tools-quarantine/main.go` (`singlechecker.Main(toolsquarantine.Analyzer)` — copy `cmd/vet-noduckdb/main.go` verbatim, swap the analyzer), add `VETTOOL_TOOLS_QUARANTINE` to the Makefile `vet:` target alongside the existing 6 `go vet -vettool=...` lines (Makefile lines 47–72).

## The `tools/` Tree + `.gitignore` + `go test` Exclusion

**`tools/` does NOT exist yet** `[VERIFIED: ls tools/ → no such dir]`. Create `tools/dspy-tune/`.

- **`go test ./...` exclusion is automatic for Python files** — `go test ./...` only walks packages containing `.go` files. As long as `tools/dspy-tune/` contains **no `.go` files** and **no `go.mod`** (it is not a Go module / not in the workspace), it is invisible to the Go toolchain. The leakage analyzer's import-boundary check is the guard that keeps it that way. `[VERIFIED: go test semantics + ablationleakage testdata precedent]`
- **`.gitignore` already covers most Python artifacts** `[VERIFIED: cat .gitignore]`: `__pycache__/`, `*.py[cod]`, `.venv`, `venv/`, `.venv/`, `env/`, `*.egg-info/`, `.pytest_cache/`, `build/`, `dist/` are all present at repo root and apply recursively. **Action needed:** add an explicit ignore for the harness's **optimizer output dir** (e.g. `/tools/dspy-tune/output/` or `*.optimized.json` under `tools/dspy-tune/`) since DSPy `program.save()` writes a JSON artifact that must NOT be committed raw (it re-enters only via the human-reviewed SKILL.md/refgen path). **Commit:** the harness *source* (`scorer.py`, `optimize.py`, `test_parity.py`), the pinned `requirements.txt`, the golden corpus, and `train/test` data. **Ignore:** `.venv/`, optimizer output JSON, any captured LLM transcripts not meant for the corpus.
- **Forbidden:** a `go.mod` under `tools/` (would pull it into a multi-module workspace and risk `go test ./...` reach) and any `.go` file under `tools/` (would make it a package the analyzer must then exempt or flag).

## The Artifact Re-Entry Gate

Adopted optimizer output re-enters the shipped surface **only** through the existing generated/hand-authored path — never as a raw optimizer dump:

- **SKILL.md** is hand-authored and embedded via `//go:embed skills/helix/*` (`internal/cli/skill.go:21`); `EmbeddedSkillBody()` returns it. Any tuned *decision-matrix / steering prose* re-enters as a **human-reviewed edit to `internal/cli/skills/helix/SKILL.md`**, preserving the `## Decision matrix` `StripDecisionMatrix` anchor and the **SKILL-04 idle-cost cap of 1,536 chars** (`internal/cli/skill_test.go:121 const cap1536 = 1536`; `TestSkillIdleCostBound`). `[VERIFIED: skill_test.go]`
- **reference.md** is GENERATED, never hand-edited. `cmd/helix-refgen` renders it from the live tool registry; `go run ./cmd/helix-refgen --check` (Makefile `verify-reference`, line 90) exits 1 on drift. Any per-verb text correction goes through the **generator override**, then regenerate + commit. The re-entry gate is: **the adopted text must pass `helix-refgen --check` byte-for-byte** (i.e. if it touches reference content it must come from the generator, not a paste). `[VERIFIED: cmd/helix-refgen/main.go, Makefile]`
- **Net effect:** the optimizer can PROPOSE text; a human transcribes the adopted proposal into SKILL.md (size-capped, anchor-preserving) or the refgen override (drift-gated). The raw `optimized.json` is git-ignored and never ships.

## Architecture Patterns

### System Architecture Diagram

```
                        ┌─────────────────── COMMITTED (in-tree, dev-time) ────────────────────┐
  TRAIN transcripts ───►│  tools/dspy-tune/                                                     │
  (train.jsonl)         │    optimize.py ──► dspy.GEPA(metric=adopt_metric) ──► optimized.json  │ (git-ignored output)
                        │        │                    ▲                                         │
  golden corpus ───────►│        │              scorer.py  (Python re-impl of adopt classifier) │
  (parity_cases.json)   │        │                    ▲                                         │
                        │   dspy.LM(model=...) ◄── OPENAI_API_KEY (dev env only)                │
                        └────────┼────────────────────┼─────────────────────────────────────────┘
                                 │                     │ parity assert (pytest test_parity.py)
   HELD-OUT TEST ────────────────┘                     │ parity assert (Go parity_test.go)
   (test.jsonl, optimizer never sees)                  ▼
                                              test/oracle/adopt/scorecard.go  ◄── SINGLE SOURCE OF TRUTH
                                              (FirstCommand/ClassifyChoice/Scorecard, runs in go test ./...)
                                                          │
   degenerate-steering inspection ◄── optimized text ────┤  (no-ship decision)
                                                          ▼
   ADOPTED?  ── human review ──►  edit SKILL.md (≤1536, anchor preserved)  OR  refgen override ──► helix-refgen --check ──► commit ──► SHIPPED BINARY (100% Go, 0 Python)
                                                          ▲
   cmd/vet-tools-quarantine (make vet): asserts NO runtime/cmd package imports tools/ ; testdata RED fixture trips it
```

### Recommended Project Structure
```
tools/dspy-tune/                  # dev-time only, no .go, no go.mod
├── requirements.txt              # pinned: dspy==3.1.3, pytest==...   (COMMITTED)
├── scorer.py                     # Python re-impl of adopt classifier (COMMITTED)
├── optimize.py                   # GEPA harness loop                  (COMMITTED)
├── test_parity.py                # pytest: parity + planted-divergence (COMMITTED)
├── golden/parity_cases.json      # shared golden corpus               (COMMITTED)
├── data/{train,test}.jsonl       # held-out split; TEST sequestered   (COMMITTED)
├── README.md                     # how to run; "no-ship is OK"        (COMMITTED)
├── .venv/                        # (git-ignored)
└── output/optimized.json         # DSPy program.save() (git-ignored)

test/oracle/adopt/
├── scorecard.go                  # UNCHANGED source of truth
├── parity_test.go                # NEW: asserts ClassifyChoice over golden/parity_cases.json

internal/lint/toolsquarantine/    # NEW leakage analyzer (model: ablationleakage)
├── analyzer.go
├── analyzer_test.go
└── testdata/src/.../{leakyruntime(RED), goodruntime(GREEN), lookalike}/imports.go
cmd/vet-tools-quarantine/main.go  # NEW singlechecker (copy cmd/vet-noduckdb)
Makefile                          # add VETTOOL_TOOLS_QUARANTINE to vet:
```

### Pattern 1: DSPy GEPA harness loop
**What:** Evolve a single instruction string against the parity-checked scalar metric.
**When to use:** The core optimization run (dev-time).
**Example:**
```python
# tools/dspy-tune/optimize.py
# Source: dspy.ai/api/optimizers/GEPA + tutorials/gepa_aime  [CITED]
import os, dspy
from dspy import GEPA
from scorer import score_choice_rate   # parity-pinned Python re-impl wrapper

# Dev-time LM backend — reads OPENAI_API_KEY from the DEV environment ONLY.
# This config NEVER touches the shipped binary (it lives in tools/, off the Go path).
dspy.configure(lm=dspy.LM(model="openai/gpt-4.1-mini", api_key=os.environ["OPENAI_API_KEY"]))

def adopt_metric(gold, pred, trace=None, pred_name=None, pred_trace=None):
    # gold carries the task; pred.response is the model's emitted transcript.
    chose, fell = score_choice_rate(pred.response)   # uses the parity-checked classifier
    score = 1.0 if chose else 0.0
    feedback = "Emitted a helix verb first (good)." if chose else \
               "Fell back to grep/sed/cat — steer toward a helix verb."
    return dspy.Prediction(score=score, feedback=feedback)

optimizer = GEPA(metric=adopt_metric, auto="light", track_stats=True,
                 reflection_lm=dspy.LM(model="openai/gpt-4.1-mini",
                                       temperature=1.0, api_key=os.environ["OPENAI_API_KEY"]))
optimized = optimizer.compile(student=program, trainset=trainset, valset=valset)  # TEST excluded
optimized.save("output/optimized.json")   # git-ignored; re-enters only via human SKILL.md edit
```

### Pattern 2: GEPA custom metric signature
**What:** GEPA calls the metric with `(gold, pred, trace, pred_name, pred_trace)` and accepts a `float` OR a `dspy.Prediction(score=..., feedback=...)` for reflective text feedback.
**Source:** `dspy.ai/api/optimizers/GEPA/overview` `[CITED]` — verified signature above.

### Anti-Patterns to Avoid
- **`strings.Contains`/`"helix" in response` in the Python scorer** — must use first-command prefix matching (the substring trap; the whole point of the parity corpus's discriminator case).
- **Committing `optimized.json` directly into the skill bundle** — violates the re-entry gate and `helix-refgen --check`; the raw dump must be git-ignored and human-transcribed.
- **A `go.mod` or `.go` file under `tools/`** — pulls Python tooling into the Go toolchain's reach.
- **Flagging the existing `pip`/`pipx` LS installer in the leakage analyzer** — `internal/langregistry/installer.go` legitimately installs language servers; scope the analyzer to import-boundary + DSPy/optimizer literals only.
- **A green-only parity test or leakage analyzer** — the named CR-01 vacuity class; every gate ships a planted break → RED.
- **Treating no-ship as failure** — it is an explicit success-meeting outcome.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Prompt optimization loop | Custom mutation/scoring loop (in the DSPy leg) | `dspy.GEPA` | Reflective evolution, Pareto tracking, threading, budget control are non-trivial; DSPy is the canonical tool (the *Go* hand-rolled loop is the deliberate FALLBACK, chosen only if DSPy is rejected) |
| Adopt classifier (Go side) | A new Go scorer | `test/oracle/adopt` (unchanged) | It IS the single source of truth; the phase mirrors it in Python, never re-implements it in Go |
| go/analysis boundary analyzer scaffolding | Hand-walk AST imports | `golang.org/x/tools/go/analysis` + `analysistest` (already vendored) | Six existing `vet-*` analyzers prove the pattern; `analysistest` gives the RED/GREEN `// want` fixture harness for free |
| LM backend plumbing | Custom HTTP to an LLM | `dspy.LM(...)` / litellm | DSPy abstracts provider, retries, token accounting |

**Key insight:** The Go adopt classifier and the go/analysis analyzer family already exist and are the load-bearing pieces; this phase is 80% *mirroring and gating* existing Go truth into a quarantined Python tree, 20% net-new DSPy harness.

## Runtime State Inventory

> This is a greenfield-additive phase (new `tools/` tree + new analyzer + new parity test). It does not rename or migrate existing runtime state. The only "state" concerns are quarantine boundaries, covered below.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore touched. The corpus is committed JSON fixtures. | none |
| Live service config | DSPy's `dspy.LM` reads `OPENAI_API_KEY` from the **dev environment only** — never from the binary, `helix setup`, or any committed config. | Document in `tools/dspy-tune/README.md`; ensure no API key is committed |
| OS-registered state | None. | none — verified, no scheduler/daemon registration in this phase |
| Secrets/env vars | `OPENAI_API_KEY` (or chosen provider key) consumed at dev-time by `optimize.py` only. NOT read by any Go code. | `.env` already in `.gitignore`; never commit keys |
| Build artifacts | New `cmd/vet-tools-quarantine` binary installed to `$GOPATH/bin` by `make vet` (mirrors existing 6 vet tools); `tools/dspy-tune/.venv` + `output/*.json` are git-ignored dev artifacts. | Add Makefile install rule for the new vet tool; gitignore the venv/output |

## Common Pitfalls

### Pitfall 1: Substring-inflated parity (the CR-01 vacuity class)
**What goes wrong:** The Python scorer uses `"helix" in response` (or matches the whole transcript), so it agrees with the Go scorer on simple cases but diverges on a prose-mentions-helix-then-greps transcript — and the parity corpus has no such case to catch it.
**Why it happens:** The injected SKILL.md is saturated with the word "helix"; a Contains check counts that as adoption.
**How to avoid:** Re-implement `FirstCommand` + prefix matching exactly; include the substring-trap case in `parity_cases.json`; ship a planted-divergence test proving a broken classifier fails the corpus.
**Warning signs:** Parity passes but the substring-trap case is absent from the corpus, or `Unclassified` is never produced.

### Pitfall 2: Leakage analyzer false-positive on the LS installer
**What goes wrong:** A naive "no `exec.Command("pip"...)` anywhere" check flags `internal/langregistry/installer.go`, breaking `make vet` on the real tree.
**Why it happens:** The LS installer legitimately uses `pip`/`pipx` to install language servers.
**How to avoid:** Scope the analyzer to the import-boundary (`tools/` not imported by runtime) plus, if a shell-out check is added, DSPy/optimizer-specific literals only — never a blanket `pip` ban.
**Warning signs:** `make vet` fails on `internal/langregistry/installer.go`.

### Pitfall 3: Overfit invisible because the split came second
**What goes wrong:** GEPA is run on all data, then a "TEST" split is carved out afterward — overfit is undetectable and `choice_rate` looks great.
**Why it happens:** Skipping the "held-out TEST split is the FIRST harness task" discipline.
**How to avoid:** Create and lock `test.jsonl` before any optimization; GEPA's `trainset`/`valset` draw only from `train.jsonl`.
**Warning signs:** TEST `choice_rate` equals TRAIN `choice_rate` exactly; or there's no code path that excludes TEST from the optimizer.

### Pitfall 4: Metric-gaming via degenerate steering
**What goes wrong:** GEPA discovers that "always emit `helix` first regardless of the task" maximizes `choice_rate` — a degenerate win that would harm real usage.
**Why it happens:** `choice_rate` is a gameable first-command proxy with no quality/correctness joiner.
**How to avoid:** Add the degenerate-steering inspection checklist; pair with a quality oracle where feasible (TUNE-FUT-02 is the full version); conclude no-ship if the win is degenerate.
**Warning signs:** Optimized text instructs unconditional `helix` use, or `choice_rate` hits 1.0 on TEST while tasks include non-code prose where the skill should be dormant.

### Pitfall 5: Raw optimizer dump shipped
**What goes wrong:** `optimized.json` content is pasted into the skill bundle, bypassing review and `helix-refgen --check`.
**Why it happens:** Treating the optimizer output as a directly-shippable artifact.
**How to avoid:** git-ignore `output/`; adopted text re-enters ONLY via a human SKILL.md edit (anchor + 1536 cap preserved) or refgen override (drift-gated).
**Warning signs:** A commit adds optimizer-generated prose without passing `verify-reference` / without an `internal/cli/skills/helix/SKILL.md` edit.

## Code Examples

### Python scorer (parity-pinned re-implementation)
```python
# tools/dspy-tune/scorer.py — MUST mirror test/oracle/adopt/scorecard.go EXACTLY
FALLBACK_PREFIXES = ("grep ", "sed ", "cat ", "find ", "rg ", "ls ")  # trailing space load-bearing

def first_command(response: str) -> str:
    for raw in response.split("\n"):
        line = raw.strip()
        if line == "" or line.startswith("```"):
            continue
        line = line.strip("`").strip()
        if line.startswith("$ "): line = line[2:]
        elif line.startswith("> "): line = line[2:]
        return line.strip()
    return ""

def classify_choice(response: str):
    cmd = first_command(response).lower()
    chose = cmd.startswith("helix ")
    fell_back = any(cmd.startswith(t) for t in FALLBACK_PREFIXES)
    return chose, fell_back   # (False, False) => unclassified
```

### Go parity test (new, in the default suite)
```go
// test/oracle/adopt/parity_test.go — pins golden/parity_cases.json to the Go truth
func TestPythonGoParityCorpus(t *testing.T) {
    data, err := os.ReadFile(filepath.Join("..", "..", "..", "tools", "dspy-tune", "golden", "parity_cases.json"))
    require.NoError(t, err)
    var cases []struct{ Response string `json:"response"`; Chose, FellBack bool `json:"chose","fell_back"` }
    require.NoError(t, json.Unmarshal(data, &cases))
    require.GreaterOrEqual(t, len(cases), 8, "corpus must cover every classifier branch")
    for _, c := range cases {
        chose, fell := ClassifyChoice(c.Response)
        require.Equalf(t, c.Chose, chose, "choice mismatch on %q", c.Response)
        require.Equalf(t, c.FellBack, fell, "fallback mismatch on %q", c.Response)
    }
}
```
*(Note: path relative to `test/oracle/adopt/` — verify the relative depth at plan time; the file may instead live beside the corpus or load it via a build-relative helper.)*

### Leakage analyzer core (model: ablationleakage)
```go
// internal/lint/toolsquarantine/analyzer.go (sketch)
const toolsPrefix = "github.com/agenthands/helix/tools"
var Analyzer = &analysis.Analyzer{
    Name: "toolsquarantine",
    Doc:  "fails if a runtime/cmd package imports the dev-time tools/ tree",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if strings.HasPrefix(pass.Pkg.Path(), toolsPrefix) { return nil, nil } // tools/ may self-import
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                p := strings.Trim(imp.Path.Value, `"`)
                if p == toolsPrefix || strings.HasPrefix(p, toolsPrefix+"/") {
                    pass.Reportf(imp.Pos(), "runtime package %s must not import dev-time %s (got %s)",
                        pass.Pkg.Path(), toolsPrefix, p)
                }
            }
        }
        return nil, nil
    },
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual prompt engineering | DSPy declarative optimization (GEPA reflective evolution) | DSPy 2.5+→3.x; GEPA added recently | The blessed way to optimize a single instruction against a scalar metric |
| MIPROv2/COPRO for instruction tuning | GEPA for reflective single-prompt evolution on small data | DSPy 3.x | GEPA is the recommended starting optimizer here; MIPROv2 is the heavier deferred upgrade |

**Deprecated/outdated:**
- Older DSPy `dspy.teleprompt.*` import paths still work (`from dspy.teleprompt import MIPROv2`) but `from dspy import GEPA` is the current top-level form. `[CITED: dspy.ai]`

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `dspy==3.1.3` is the current/correct pin | Standard Stack | Low — verify with `pip index versions dspy` at plan time; DSPy moves fast |
| A2 | `pytest` is the right Python test runner (vs unittest) | Standard Stack | Negligible — either works; pytest is conventional |
| A3 | GEPA (not MIPROv2/COPRO) is the right optimizer for a single steering string on tiny data | Optimizer selection | Low — REQUIREMENTS explicitly names GEPA as the starting point (TUNE-FUT-01 = "upgrade from GEPA") |
| A4 | A degenerate-steering inspection + held-out TEST is a sufficient metric-gaming guard for a spike | Corpus Split | Medium — the proxy is gameable; full defense (quality oracle) is TUNE-FUT-02, explicitly deferred. Document the residual risk. |
| A5 | The provider is OpenAI-compatible (`openai/gpt-4.1-mini`) for the dev-time LM | Code Examples | Low — any litellm-supported provider works; the model id is illustrative, set at dev-time |
| A6 | No-ship is the most likely outcome given `MinTasks=5` | Corpus Split / Summary | Low — this is an explicit success-meeting outcome, not a risk |

## Open Questions (RESOLVED)

> **RESOLVED at plan time (106-01/106-02-PLAN.md pin these):**
> - **Q1 (LM backend):** RESOLVED — `optimize.py` parameterizes the LM via env vars with an unset-key guard; default small OpenAI model documented in the README; dev-time only, never the binary. Does not block the hermetic parity/leakage work.
> - **Q2 (corpus size):** RESOLVED — run on the small intact set first to honestly surface the "too small to trust → no-ship" finding; TEST split sequestered regardless of any optional TRAIN expansion.
> - **Q3 (golden corpus location):** RESOLVED — single committed file under `tools/dspy-tune/golden/parity_cases.json`, read by BOTH the Go parity test and pytest (no duplication; path pinned in both).

1. **Where should the dev-time LLM backend point, and is an API key available in the dev environment?**
   - What we know: DSPy needs an LM (`dspy.LM` + `OPENAI_API_KEY`); it must read from the dev env only, never the binary.
   - What's unclear: which provider/model the operator will use; whether a local model is preferred to avoid external API calls.
   - Recommendation: parameterize via env vars in `optimize.py`; default to a small OpenAI model in the README; document that the spike may run with any litellm provider. This does NOT block the parity/leakage/analyzer work, which is fully hermetic.

2. **Is the 6+6 fixture corpus the optimizer's data, or does the spike author a larger TRAIN corpus?**
   - What we know: `test/oracle/adopt/testdata/transcripts/` has 6 intact + 6 sabotaged; `MinTasks=5`.
   - What's unclear: whether the spike expands the corpus (authoring more tasks) or runs honestly on the tiny set to demonstrate the no-ship conclusion.
   - Recommendation: run on the small set first to honestly surface the "too small to trust" finding; optionally author a modestly larger TRAIN set, but keep TEST sequestered regardless. Either path satisfies the spike.

3. **Does the parity test belong in `test/oracle/adopt` (Go) reading across into `tools/`, or should the golden corpus live under `test/oracle/adopt/testdata`?**
   - What we know: both sides must read the SAME file.
   - What's unclear: cross-tree relative-path fragility vs. duplicating the corpus.
   - Recommendation: single committed file (no duplication) under `tools/dspy-tune/golden/` read by both, OR under `test/oracle/adopt/testdata/` read by Python via a stable relative path. Pick one canonical location at plan time and pin the path in both tests; do NOT duplicate (duplication defeats the parity guarantee).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `python3` | DSPy harness (dev-time) | likely ✓ | check at plan time | If absent, the harness can't run — but the spike's no-ship fallback is a Go loop, so absence doesn't block the milestone |
| `pip`/venv | installing `dspy` (dev-time) | likely ✓ | — | same as above |
| DSPy-compatible LLM API key | the optimization RUN only | unknown | — | Run only the hermetic legs (parity, analyzer, scorer unit tests); the optimization run is informational |
| `go` + `golang.org/x/tools` | the Go parity test + leakage analyzer | ✓ | go 1.25.1, x/tools v0.43.0 `[VERIFIED]` | none needed |

**Missing dependencies with no fallback:** none — the *hermetic* deliverables (parity test, leakage analyzer, scorer unit tests, no-ship report) require ONLY the Go toolchain (already present). The DSPy optimization RUN needs Python + an LLM key, but its absence does not block phase completion (no-ship is legitimate, and the Go fallback path is the clean alternative).
**Missing dependencies with fallback:** the DSPy run itself — fallback is the documented hand-rolled Go candidate-search loop (keeps the milestone 100% Go).

## Validation Architecture

> nyquist_validation is enabled (no `workflow.nyquist_validation: false` found). Section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework (Go) | Go `testing` + `testify/require` (existing), `golang.org/x/tools/go/analysis/analysistest` (for the analyzer) |
| Framework (Python) | `pytest` (new, dev-time only, under `tools/dspy-tune/`) |
| Config file | none new for Go; `tools/dspy-tune/requirements.txt` pins pytest |
| Quick run command (Go) | `go test ./test/oracle/adopt/... ./internal/lint/toolsquarantine/...` |
| Full suite command | `make vet && go test ./...` (Python pytest is OUT of `go test ./...` by design) |
| Python parity run (dev-time) | `cd tools/dspy-tune && . .venv/bin/activate && pytest test_parity.py` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TUNE-01 | Python scorer agrees with Go classifier on the golden corpus | unit (Go) | `go test ./test/oracle/adopt/ -run TestPythonGoParityCorpus -x` | ❌ Wave 0 (`parity_test.go` + corpus) |
| TUNE-01 | Python scorer agrees on the golden corpus (Python side) | unit (pytest) | `pytest tools/dspy-tune/test_parity.py -x` | ❌ Wave 0 |
| TUNE-01 | A deliberately-wrong Python classifier FAILS parity (anti-vacuity) | unit (pytest) | `pytest tools/dspy-tune/test_parity.py::test_broken_classifier_diverges -x` | ❌ Wave 0 |
| TUNE-01 | Runtime package importing `tools/` is FLAGGED (leakage RED) | analyzer (analysistest) | `go test ./internal/lint/toolsquarantine/ -run RejectsRuntimeImportingTools -x` | ❌ Wave 0 |
| TUNE-01 | Clean tree + slash-lookalike NOT flagged (leakage GREEN) | analyzer | `go test ./internal/lint/toolsquarantine/ -run Allows -x` | ❌ Wave 0 |
| TUNE-01 | `make vet` runs the new quarantine analyzer on the real tree (no false-positive on LS installer) | integration | `make vet` | ❌ Wave 0 (Makefile edit) |
| TUNE-01 | Held-out TEST split exists and the optimizer excludes it | unit (pytest) | `pytest tools/dspy-tune/test_split.py -x` (asserts TEST ⊄ trainset∪valset) | ❌ Wave 0 |
| TUNE-01 | Degenerate-steering inspection flags an always-`helix` text | unit (pytest) | `pytest tools/dspy-tune/test_degenerate.py -x` | ❌ Wave 0 |
| TUNE-01 | Adopted text (if any) passes refgen drift gate | integration | `go run ./cmd/helix-refgen --check` | ✅ (existing gate) |
| TUNE-01 | SKILL.md stays under the 1536 idle cap after any adopted edit | unit (Go) | `go test ./internal/cli/ -run TestSkillIdleCostBound -x` | ✅ (existing) |

### Sampling Rate
- **Per task commit:** `go test ./test/oracle/adopt/... ./internal/lint/toolsquarantine/...` + (for Python-touching tasks) `pytest tools/dspy-tune/`
- **Per wave merge:** `make vet && go test ./...`
- **Phase gate:** Full suite green + `make vet` (incl. new analyzer) + `helix-refgen --check` before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `test/oracle/adopt/parity_test.go` — Go side of the parity cross-check (covers TUNE-01)
- [ ] `tools/dspy-tune/golden/parity_cases.json` — the shared golden corpus (≥8 branch-covering cases incl. substring-trap + `ls`-lookalike)
- [ ] `tools/dspy-tune/scorer.py` + `tools/dspy-tune/test_parity.py` — Python re-impl + parity + planted-divergence (anti-vacuity)
- [ ] `tools/dspy-tune/test_split.py` + `test_degenerate.py` — overfit + metric-gaming guards
- [ ] `internal/lint/toolsquarantine/{analyzer.go,analyzer_test.go}` + `testdata/src/.../{leakyruntime,goodruntime,lookalike}` — leakage analyzer with RED/GREEN/lookalike fixtures (anti-vacuity)
- [ ] `cmd/vet-tools-quarantine/main.go` + Makefile `vet:` wiring + install rule
- [ ] `tools/dspy-tune/requirements.txt` (pinned `dspy==3.1.3`, `pytest`), `README.md` (no-ship is OK), `.gitignore` entry for `output/`
- [ ] Anti-vacuity break-the-invariant tests for EACH gate: planted parity-break → pytest RED; planted leak → analyzer trips; degenerate steering → flagged

## Security Domain

> `security_enforcement` posture for this phase: the phase adds NO network endpoint, no auth surface, no input-from-untrusted-user path in the shipped binary. The only external interaction is a **dev-time** LLM API call from `tools/`, off the runtime path.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | no auth surface added |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | minimal | the Python scorer parses committed JSON fixtures only (trusted, in-tree) |
| V6 Cryptography | no | no crypto; never hand-roll |
| V14 Configuration / Supply Chain | yes | DSPy/pytest live in a git-ignored dev venv; pinned `requirements.txt`; the leakage analyzer + import-boundary keep them OFF the shipped binary and `go.mod` (supply-chain quarantine is the core security property of this phase) |

### Known Threat Patterns for {Go binary + dev-time Python}
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Dev-time Python dep leaks into the shipped binary | Tampering / Elevation (supply chain) | `cmd/vet-tools-quarantine` import-boundary analyzer + `tools/` excluded from `go test`/`go.mod` + planted-leak RED fixture |
| LLM API key committed to the repo | Information Disclosure | `.env` git-ignored; key read from dev env only; never in committed config or the binary |
| Raw, unreviewed optimizer output shipped to users | Tampering | Artifact re-entry gate: human review + `helix-refgen --check` + SKILL-04 cap; `output/` git-ignored |

## Sources

### Primary (HIGH confidence)
- `test/oracle/adopt/scorecard.go` + `scorecard_test.go` — the exact classifier algorithm + anti-vacuity test patterns (read first-hand)
- `internal/lint/ablationleakage/analyzer.go` + `analyzer_test.go` + `testdata/` — the go/analysis leakage-analyzer template incl. slash-boundary + RED/GREEN fixture discipline (read first-hand)
- `cmd/helix-refgen/main.go` + Makefile `verify-reference` (line 90) — the `--check` artifact re-entry gate (read first-hand)
- `internal/cli/skill.go` (`//go:embed`, `EmbeddedSkillBody`) + `internal/cli/skill_test.go` (`cap1536=1536`, `TestSkillIdleCostBound`) — SKILL.md re-entry surface + size cap (read first-hand)
- `go.mod` (`golang.org/x/tools v0.43.0`, go 1.25.1), `.gitignore` (Python patterns present), `internal/langregistry/installer.go:119,129` (existing legitimate `pip`/`pipx` LS shell-out — the analyzer false-positive trap) (read first-hand)
- Context7 `/websites/dspy_ai` — GEPA optimizer signature, `compile(trainset, valset)`, custom metric `(gold, pred, trace, pred_name, pred_trace) -> float | ScoreWithFeedback`, `dspy.LM`/`dspy.configure`, `program.save()` (DSPy 3.1.3)

### Secondary (MEDIUM confidence)
- DSPy version `3.1.3` from `ctx7 library dspy` (`/stanfordnlp/dspy` Versions field) — confirm with `pip index versions dspy` at plan time

### Tertiary (LOW confidence)
- `pytest` exact version pin — pick current at plan time

## Metadata

**Confidence breakdown:**
- Standard stack (DSPy 3.1.3 + GEPA): HIGH — verified via Context7; GEPA is REQUIREMENTS' named starting optimizer
- Adopt classifier / parity contract: HIGH — algorithm read line-by-line from `scorecard.go`; parity is mechanically achievable
- Leakage analyzer: HIGH — direct template (`ablationleakage`) read first-hand incl. the LS-installer false-positive trap
- Artifact re-entry gate: HIGH — refgen `--check` + SKILL.md embed + 1536 cap all verified
- Corpus split / no-ship discipline: MEDIUM — design is sound but the tiny `MinTasks=5` corpus makes a trustworthy delta unlikely (which is the documented, expected no-ship finding)

**Research date:** 2026-06-24
**Valid until:** 2026-07-01 for DSPy version (fast-moving); 2026-07-24 for the Go codebase anchors (stable)
