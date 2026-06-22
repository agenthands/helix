# Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate - Research

**Researched:** 2026-06-21
**Domain:** Go bench-stack adapters for completion-only public benchmarks (HF dataset loading + pure-logic scorers + multi-oracle gate)
**Confidence:** HIGH on in-tree architecture (codebase-verified); MEDIUM on dataset schemas (HF docs + papers, not byte-confirmed against live parquet)

## Summary

Phase 86 wires two `dataset-loader-only` completion benchmarks into the existing `bench/` stack: **CrossCodeEval** (CCE; Python/Java/TS/C#) and **RepoBench** (R/C/P sub-tasks; Python/Java), plus a **multi-oracle completion gate** (EM + edit-similarity + identifier-match all required → `verified_correctness`; abstain → `false`, never a false `true`). Unlike the test-bearing adapters (SWE-bench etc.), these benchmarks have **no test suites** — the ground truth is a single completion string per task, scored by three string-level oracles. This makes the scorers + gate **pure logic**, fully hermetically testable, and the load-bearing core of the phase.

**The single most important grounding finding (changes the plan shape):** the Phase 75 "HF dataset fetcher infra (`gomlx/go-huggingface` + arrow-go fallback)" that CONTEXT.md and the ROADMAP claim this phase *depends on* **DOES NOT EXIST IN-TREE**. Phase 75 was "Schema, Fairness Contract & Tree Skeleton" — it shipped the `result.v2` schema, the fairness contract, the cost-table gate, and a `cmd/helix-bench` cobra skeleton whose `fetch-datasets` subcommand is an explicit `notYetImplemented` stub. `gomlx/go-huggingface` is **not in `go.mod`**; `apache/arrow-go/v18` is present but `// indirect` with **zero direct source usage**. Likewise the **contamination-canary probe (SC#4) is a Phase 89 deliverable, not Phase 75** — it does not exist either. **Phase 86 must BUILD the HF fetch + parquet-read pipeline AND the canary probe; it cannot reuse them.**

**Primary recommendation:** Split the phase into (1) a pure-logic core — EM / edit-similarity / identifier-match scorers + the multi-oracle gate + the canary probe, hermetically unit-tested against committed CCE/RepoBench paper-example fixtures (the SOLE authoritative proof) — and (2) a network-gated HF fetch + parquet loader that reuses the `bench/ragindex` `$HELIX_CACHE_DIR` cache convention and SKIPs cleanly offline. Implement the fetcher as the first wave because everything else is downstream of having rows to score, but keep the live "metrics match published reference" run honestly recorded as network-gated.

## User Constraints (from CONTEXT.md)

### Locked Decisions
None — discuss phase was skipped (`workflow.skip_discuss`). All implementation choices are Claude's Discretion, guided by the ROADMAP goal, the four success criteria, and codebase conventions.

### Claude's Discretion
- Both adapters are `dataset-loader-only` (no Docker, no upstream harness) — load HF datasets via a fetch + cached-parquet pipeline into `$HELIX_CACHE_DIR`.
- The scorers (EM, edit-similarity/Levenshtein, identifier-match) and the multi-oracle gate (all-three-required + per-oracle configurable threshold + abstain → `verified_correctness=false`) are PURE logic — unit-test hermetically against CCE/RepoBench paper examples + committed reference values. These are the load-bearing, fully-testable core.
- The canary-emission probe reuses a contamination-canary pattern (research must locate it — **see Open Question 2: it does not yet exist; Phase 86 builds it**).
- **Environment reality:** HF fetching needs network + parquet; gate live fetch/smoke-run tests on network availability (SKIP cleanly when absent), with committed fixture rows so the loader + scorers + gate are hermetically tested without network. Honestly record the live "metrics match published reference" confirmation as network-gated. No live test may be the sole proof.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADAPTER-CCE-01 | CrossCodeEval adapter via `dataset-loader-only`; EM + edit-sim + identifier-match scoring; Python/Java/TS/C#. Acceptance: smoke run scores ≥1 task per language; scorers unit-tested against CCE paper examples. | CCE dataset schema + metric formulas (below); new pure scorers in `bench/evaluators/`; new `bench/datasets/crosscodeeval/` loader; Phase 85 LanguageRunners exist for the 4 languages. |
| ADAPTER-REPO-01 | RepoBench adapter via `dataset-loader-only`; RepoBench-R + -C + -P; Python/Java. Acceptance: smoke run for each sub-task; EM/ES metrics match published reference on a sampled subset. | RepoBench HF schema (`tianyang/repobench_*`); R=acc@k retrieval, C=EM/ES completion, P=pipeline; new `bench/datasets/repobench/` loader. |
| VERIFIED-03 | Multi-oracle gate for non-test-bearing benchmarks: EM + edit-sim + identifier match all required; abstain mode. Acceptance: gate documented in `bench/evaluators/VERIFIED.md` (does NOT yet exist); per-oracle threshold configurable. | `VerifiedCorrectness *bool` already in `evaluators.Metrics` + produced by `test_runner`; new completion-path grader sets it; new `VERIFIED.md`. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HF dataset fetch (network) | `bench/datasets/<name>/` loader (+ `cmd/helix-bench fetch-datasets`) | `$HELIX_CACHE_DIR` cache | Mirrors `bench/ragindex` + `aider-polyglot` leaf-package convention; fetch is I/O-bound, isolated from scoring. |
| Parquet decode | `bench/datasets/<name>/` loader | `apache/arrow-go/v18` (promote to direct require) | Parquet is the HF on-disk format; arrow-go already transitively present. |
| EM / edit-sim / identifier-match scoring | `bench/evaluators/` (new scorer subpkgs) | — | Pure string functions; mirror the 5 existing Phase 79 graders; no I/O. |
| Multi-oracle gate (→ `verified_correctness`) | `bench/evaluators/` (new completion grader) | `coordinator` (or a parallel completion coordinator) | `VerifiedCorrectness *bool` is the existing additive key; completion path bypasses `test_runner`. |
| Canary emission + contamination flag | `bench/datasets/<name>/` loader + new `bench/canary/` (or in-loader) | aggregator (new column) | Probe is dataset-level (inject known-novel pattern); aggregator surfaces canary-pass-rate. |
| Per-language slicing | `bench/aggregator` (`LanguageRow`, exists) | `language` result key (exists) | Reuse Phase 85 `language` additive key + `ByLanguage` slice verbatim. |
| Leaderboard canary column | `bench/aggregator/report.go` (`LeaderRow`) | — | Additive column; mirror Phase 85 additive-field discipline. |

## Standard Stack

### Core (in-tree, reuse — verified by codebase grep/read)
| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `$HELIX_CACHE_DIR` cache convention | `bench/ragindex/cache.go` (`cacheDir()`, 3-step precedence) | Per-dataset on-disk cache root | Already the canonical bench cache pattern; `aider-polyglot/clone.go` cloned it verbatim. |
| Leaf-package dataset loader | `bench/datasets/aider-polyglot/` (loader.go, clone.go, pin.go) | The closest existing dataset adapter (clones a git repo, not HF, but identical shape) | stdlib-only leaf, hermetic fixtures + network-gated live clone — the exact pattern SC asks for. |
| `evaluators.Metrics` (incl. `VerifiedCorrectness *bool`) | `bench/evaluators/metrics.go:21` | The nullable metric contract; `verified_correctness` ALREADY EXISTS | SC#3 only needs to *populate* it via a completion path; no schema bump. |
| Grader fan-out coordinator | `bench/evaluators/coordinator/coordinator.go` | Pattern for a pure grader that nulls-on-failure, never short-circuits | New multi-oracle gate follows the same `Result{...; Errs []MetricError}` shape. |
| `test_runner.Grade` | `bench/evaluators/test_runner/test_runner.go` | Shows how `VerifiedCorrectness` is produced for test-bearing benches | Completion gate is the non-test-bearing sibling; comment at line 37-40 explicitly anticipates a divergent `verified_correctness` producer. |
| Phase 85 LanguageRunners | `bench/languages/{python,java,typescript,csharp}/` + `registry.go` | The 4 CCE languages already have runners + capability coverage | No new runner needed; reuse for any per-language sanity. |
| Aggregator `LeaderRow` / `LanguageRow` / `ByLanguage` | `bench/aggregator/report.go:45,85,109` | Leaderboard rows + per-language slice | Add a `CanaryPassRate ciValue` column to `LeaderRow` (additive). |
| `make verify-*` hard-gate pattern | `Makefile` (`verify-tos`, `verify-licenses`, `verify-no-docker-sdk`) | CI hard-fail gate convention | A `verify-verified-md` or scorer-fixture gate follows the same shape. |

### Supporting (to add)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/apache/arrow-go/v18` | v18.5.1 (already in go.sum, currently `// indirect`) | Parquet read for HF dataset files | Promote to direct require (hand-edit go.mod, mirror the `chromem`/`crane` precedent in STATE decisions). Use `parquet/file` + `parquet/pqarrow`. |
| (HTTP fetch) | stdlib `net/http` | Download HF parquet via the resolve URL `https://huggingface.co/datasets/<repo>/resolve/<rev>/<path>` | Prefer stdlib over a HF SDK — keeps the loader a stdlib+arrow leaf (consistent with `ragindex`/`aider-polyglot` leaf invariant). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `net/http` + arrow-go parquet | `gomlx/go-huggingface` (ROADMAP's named dep) | **NOT in go.mod.** Adding a heavyweight ML-adjacent dep for a thin "download N parquet files" need violates the leaf-package discipline every other bench dataset follows. Recommend stdlib HTTP + arrow-go; cite `gomlx/go-huggingface` only as the ROADMAP's original intent, not a requirement. **[ASSUMED]** that go-huggingface is overkill — verify package legitimacy if the planner chooses it. |
| New string-Levenshtein scorer | `internal/mcp/suggest_lev.go` Levenshtein | That impl is unexported, tuned for short tool-name typos, and lives in a non-bench package. Re-implement a small, exported, well-tested edit-distance in `bench/evaluators/` rather than reach across package boundaries (mirrors how each Phase 85 runner was cloned, not shared). |
| `patch_validator.EditDistancePatch` | reuse for edit-similarity | NO — that is `git diff --numstat` line-count distance over a working tree, not normalized string edit-similarity over two completion strings. Different metric entirely. Do not conflate. |

**Installation (planner; hand-edit go.mod per the chromem/crane precedent — full `go mod tidy` is blocked by a pre-existing unrelated `s2a-go` failure noted repeatedly in STATE):**
```bash
go get github.com/apache/arrow-go/v18@v18.5.1   # promotes indirect -> direct
```

**Version verification:**
- `apache/arrow-go/v18 v18.5.1` — **[VERIFIED: go.sum]** present in-tree (indirect). Promote to direct.
- `gomlx/go-huggingface` — **[VERIFIED: go.mod absent]** NOT present. Do not assume availability.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| github.com/apache/arrow-go/v18 | Go modules (already in go.sum) | mature (Apache) | high | github.com/apache/arrow-go | OK | Approved — promote indirect→direct |
| gomlx/go-huggingface | Go modules | unknown this session | unknown | — | N/A | NOT recommended (avoid; use stdlib HTTP). If planner insists, gate behind `checkpoint:human-verify` + `go list -m`. **[ASSUMED]** |

**Packages removed due to SLOP verdict:** none
**Packages flagged suspicious:** none verified-suspicious; `gomlx/go-huggingface` is unverified this session — treat any choice to add it as `[ASSUMED]` and gate it.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌────────────────────────────────────────────────┐
                    │  HF dataset (parquet on hub)                    │
                    │  Vincentvmt/CrossCodeEval  (py/java/ts/csharp)  │
                    │  tianyang/repobench_python_v1.1 / _java_v1.1    │
                    └───────────────────────┬────────────────────────┘
                                            │ net/http GET resolve URL  (NETWORK — gated)
                                            ▼
   ┌──────────────────────────────────────────────────────────────────────┐
   │ bench/datasets/<name>/  (leaf: stdlib + arrow-go)                      │
   │   fetch.go   → download parquet to $HELIX_CACHE_DIR/<name>/<rev>/      │
   │   loader.go  → arrow-go parquet decode → []Task{prompt, groundtruth,   │
   │                language, split, canary_seed?}                          │
   └───────────────────────┬──────────────────────────────────────────────┘
                            │ []Task   (HERMETIC from here down)
                            ▼
   ┌──────────────────────────────────────┐     ┌─────────────────────────┐
   │ agent / completion producer          │     │ committed fixtures      │
   │ (model completion string per task)   │     │ (CCE/RepoBench paper     │
   └───────────────┬──────────────────────┘     │  examples → SOLE proof)  │
                   │ completion                  └───────────┬─────────────┘
                   ▼                                         │
   ┌──────────────────────────────────────────────────────────────────────┐
   │ bench/evaluators/  (PURE scorers — load-bearing core)                  │
   │   exactmatch.EM(pred, gold)            → bool                          │
   │   editsim.ES(pred, gold)               → float64 (0..1 normalized)     │
   │   identmatch.IDMatch(pred, gold)       → bool / F1                     │
   │   ┌────────────────────────────────────────────────────────────────┐  │
   │   │ multi-oracle GATE (VERIFIED-03):                               │  │
   │   │   pass = EM && ES>=esThresh && IDMatch    (all-three required) │  │
   │   │   abstain(low-confidence) ⇒ verified_correctness = FALSE       │  │
   │   │                              (NEVER a false TRUE)              │  │
   │   │   per-oracle thresholds configurable                          │  │
   │   └────────────────────────────────────────────────────────────────┘  │
   └───────────────────────┬──────────────────────────────────────────────┘
                           │ Metrics{VerifiedCorrectness, ...} + canary flag
                           ▼
   ┌──────────────────────────────────────────────────────────────────────┐
   │ result.v2.json   (verified_correctness already a schema key)           │
   │   + language key (Phase 85)  + canary_contaminated flag (new)          │
   └───────────────────────┬──────────────────────────────────────────────┘
                           ▼
   ┌──────────────────────────────────────────────────────────────────────┐
   │ bench/aggregator → leaderboard.md                                      │
   │   LeaderRow + new CanaryPassRate column ; ByLanguage per-lang slice    │
   └──────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
bench/
├── datasets/
│   ├── crosscodeeval/        # NEW: fetch.go, loader.go, pin.go(rev), fixtures/, *_test.go
│   └── repobench/            # NEW: fetch.go, loader.go (R/C/P split awareness), pin.go, fixtures/, *_test.go
├── evaluators/
│   ├── exactmatch/           # NEW: pure EM
│   ├── editsim/              # NEW: pure normalized edit-similarity (Levenshtein/Lev ratio)
│   ├── identmatch/           # NEW: pure identifier-set match (EM + F1)
│   ├── completion_gate/      # NEW: multi-oracle gate → verified_correctness (+ abstain)
│   └── VERIFIED.md           # NEW (SC#3 acceptance artifact — does not exist today)
└── aggregator/
    └── report.go             # EDIT: add CanaryPassRate column to LeaderRow
cmd/helix-bench/
    └── main.go               # EDIT: implement newFetchDatasetsCmd (currently notYetImplemented stub)
```

### Pattern 1: Hermetic fixture + network-gated live (Phase 85 aider-polyglot precedent)
**What:** Commit small, paper-shaped fixture rows; the loader + scorers + gate run fully on fixtures (hermetic, the SOLE authoritative proof). A separate live test fetches the real HF parquet and asserts "metrics match published reference," gated on a network env var, `t.Skip`-ping cleanly when offline.
**When to use:** Always, for both adapters (this is the explicit CONTEXT.md environment-reality directive).
**Example (gating idiom, from aider-polyglot live clone):**
```go
// Source: bench/datasets/aider-polyglot/clone_test.go pattern (HELIX_BENCH_NETWORK gate)
if os.Getenv("HELIX_BENCH_NETWORK") == "" { t.Skip("network-gated: set HELIX_BENCH_NETWORK") }
```

### Pattern 2: Multi-oracle gate with abstain (VERIFIED-03)
**What:** `verified_correctness` is TRUE only if EM AND edit-sim≥thresh AND identifier-match all pass. Low-confidence/abstain emits `verified_correctness = &false` (a real `*bool` false), never leaves it `true` and never nil-drops to a misleading absence.
**Critical:** abstain → explicit `false`, distinct from "could not score" (which is a `MetricError` + nil). The CONTEXT.md SC#3 wording is precise: "emits a `verified_correctness = false` row instead of a false-positive `true`."

### Anti-Patterns to Avoid
- **Reusing `patch_validator.EditDistancePatch` for edit-similarity** — it is git-numstat line distance, not string edit-similarity. Wrong metric.
- **Adding `gomlx/go-huggingface`** without verification — it's not in go.mod; stdlib HTTP keeps the leaf invariant.
- **Treating the live HF run as the proof** — CONTEXT.md forbids it ("No live test may be the sole proof"). Fixtures are authoritative.
- **Assuming Phase 75 built the fetcher/canary** — it did not (see Open Questions 1 & 2). Plan must build both.
- **Nil-dropping `verified_correctness` on abstain** — abstain is an explicit `false`, not an absence.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Parquet decode | A hand-rolled parquet reader | `apache/arrow-go/v18 parquet/pqarrow` | Parquet is a complex columnar format; arrow-go is already in the tree. |
| Cache dir resolution | New `$HELIX_CACHE_DIR` logic | Copy `bench/ragindex/cache.go` `cacheDir()` verbatim (aider-polyglot already did) | Single canonical 3-step precedence; consistency across adapters. |
| Per-language aggregation | New slicing | `aggregator.LanguageRow` + `ByLanguage` + the `language` result key | Phase 85 already built this; CCE/RepoBench just populate `language`. |
| `verified_correctness` schema field | New schema key + v3 bump | The existing `evaluators.Metrics.VerifiedCorrectness *bool` | Already in schema; populating it is additive-minor, no bump. |

**Key insight:** ~70% of this phase's surface is *pure string logic + reuse of existing bench plumbing*. The genuinely new I/O surface (HF fetch + parquet) is thin and isolated; the genuinely new value (multi-oracle gate + canary) is pure logic. Maximize the hermetic core.

## Runtime State Inventory

> Greenfield-ish additive phase (new packages + additive result keys). Not a rename/refactor. Minimal runtime state.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | HF parquet cached under `$HELIX_CACHE_DIR/<dataset>/<rev>/` (new) | None pre-existing; new cache dir, pinned by dataset rev (mirror aider-polyglot `<sha>/`). |
| Live service config | None — datasets are static HF parquet. | None. |
| OS-registered state | None. | None. |
| Secrets/env vars | HF may require no auth for public datasets; `HELIX_BENCH_NETWORK`-style gate env var (new, test-only). | Document the gate env var name in the loader. |
| Build artifacts | `go.mod` promotion of `arrow-go` indirect→direct. | Hand-edit go.mod (tidy blocked by pre-existing s2a-go failure — STATE-documented). |

## Common Pitfalls

### Pitfall 1: Assuming the Phase 75 fetcher exists
**What goes wrong:** Planner writes "reuse Phase 75 HF fetcher" tasks; there is nothing to reuse.
**Why it happens:** CONTEXT.md and ROADMAP line 179/182 assert the dependency; the actual Phase 75 ("Schema, Fairness Contract & Tree Skeleton") never built it. `fetch-datasets` is a `notYetImplemented` stub (`cmd/helix-bench/main.go:353-358`).
**How to avoid:** Plan the fetch + parquet pipeline as net-new work in Wave 1.
**Warning signs:** A task says "import the Phase 75 dataset fetcher" — it does not exist.

### Pitfall 2: Edit-similarity ≠ git diff distance
**What goes wrong:** Reusing `EditDistancePatch` (numstat line counts) for CCE's normalized token-level edit similarity → meaningless numbers, paper reference won't match.
**How to avoid:** Implement a fresh normalized Levenshtein ratio (1 - lev/max(len)) over the two completion strings, matching the CCE paper's CM-ES definition.
**Warning signs:** edit-similarity values cluster at integers or don't fall in [0,1].

### Pitfall 3: CCE HF dataset schema instability
**What goes wrong:** `Vincentvmt/CrossCodeEval` currently surfaces a HF cast error ("data files must have the same columns") on the dataset-viewer — auto-load may fail; per-language column drift between splits.
**Why it happens:** CCE's canonical distribution is the GitHub repo + raw JSONL, not a clean single HF parquet; the HF mirror is community-uploaded.
**How to avoid:** Pin a specific revision; load per-language files explicitly rather than `load_dataset(...)`-style auto-merge; consider the official `crosscodeeval.github.io` / amazon-science raw artifacts as the authoritative source for fixture values. **Verify the exact HF repo + rev at a `checkpoint:human-verify` before committing fixtures.**
**Warning signs:** parquet column set differs across py/java/ts/csharp files.

### Pitfall 4: Identifier-match tokenization
**What goes wrong:** Naive whitespace split for identifier-match over-counts keywords/punctuation; CCE's IM compares *identifiers* (variable/function names), reported as EM and F1.
**How to avoid:** Tokenize to identifiers (language-aware or a regex `[A-Za-z_][A-Za-z0-9_]*` minus keywords) before set-comparing; document the tokenizer in `VERIFIED.md`. Keep the rule identical to the CCE paper to make the reference match.

### Pitfall 5: Canary as a Phase 89 concept used early
**What goes wrong:** SC#4 requires a working canary-emission probe NOW, but the canary is a Phase 89 deliverable (ROADMAP line 210-218) and does not exist.
**How to avoid:** Build a minimal canary probe in Phase 86 (inject a known-novel pattern into select task prompts; if a completion emits it verbatim, flag the task `canary_contaminated=true`; populate a `canary_pass_rate` aggregator column). Coordinate naming with the eventual Phase 89 reporter so it is forward-compatible, but do not block on Phase 89.

## Code Examples

### Multi-oracle gate (sketch, mirrors test_runner.Result shape)
```go
// Source: pattern from bench/evaluators/test_runner/test_runner.go (Grade returns Result with *bool + Errs)
type GateConfig struct{ ESThreshold float64 } // per-oracle configurable (VERIFIED-03)

type GateResult struct {
    VerifiedCorrectness *bool
    EM, IDMatch         *bool
    ES                  *float64
    Errs                []evaluators.MetricError
}

func Grade(pred, gold string, cfg GateConfig, abstain bool) GateResult {
    if abstain { // low-confidence ⇒ explicit FALSE, never a false-positive true
        f := false
        return GateResult{VerifiedCorrectness: &f}
    }
    em := exactmatch.EM(pred, gold)
    es := editsim.ES(pred, gold)
    id := identmatch.Match(pred, gold)
    ok := em && es >= cfg.ESThreshold && id   // all three required
    return GateResult{VerifiedCorrectness: &ok, EM: &em, ES: &es, IDMatch: &id}
}
```

### Cache dir (copy verbatim — already cloned once for aider-polyglot)
```go
// Source: bench/ragindex/cache.go:32 — HELIX_CACHE_DIR > os.UserCacheDir()/helix > ~/.helix/cache
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| ROADMAP-named `gomlx/go-huggingface` fetcher | stdlib `net/http` + `arrow-go` parquet leaf | This phase (dep never landed) | Keeps the dataset loader a stdlib+arrow leaf, consistent with ragindex/aider-polyglot. |
| `load_dataset` Python auto-merge | Explicit per-language/per-split parquet file fetch | This phase | Avoids the CCE HF cast-error and column-drift trap. |

**Deprecated/outdated:**
- The CONTEXT.md/ROADMAP claim "Phase 75 HF dataset fetcher infra" — **outdated/incorrect**; never built. Treat Phase 86 as the builder.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ table tests), matching all of `bench/` |
| Config file | none — `go test ./...` |
| Quick run command | `go test ./bench/evaluators/... ./bench/datasets/...` |
| Full suite command | `go vet ./... && go test ./...` (per CLAUDE.md discipline) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ADAPTER-CCE-01 | EM/ES/identifier scorers vs CCE paper examples | unit (HERMETIC, authoritative) | `go test ./bench/evaluators/exactmatch/... ./bench/evaluators/editsim/... ./bench/evaluators/identmatch/...` | ❌ Wave 0 |
| ADAPTER-CCE-01 | smoke run scores ≥1 task/lang (py/java/ts/csharp) | smoke (NETWORK-gated, SKIP offline) | `HELIX_BENCH_NETWORK=1 go test ./bench/datasets/crosscodeeval/... -run Live` | ❌ Wave 0 |
| ADAPTER-REPO-01 | EM/ES match published reference on sampled subset | smoke (NETWORK-gated) | `HELIX_BENCH_NETWORK=1 go test ./bench/datasets/repobench/... -run Live` | ❌ Wave 0 |
| ADAPTER-REPO-01 | R(acc@k)/C(EM,ES)/P loaders parse fixture rows | unit (HERMETIC) | `go test ./bench/datasets/repobench/...` | ❌ Wave 0 |
| VERIFIED-03 | all-three-required gate + abstain→false + per-oracle threshold | unit (HERMETIC, authoritative) | `go test ./bench/evaluators/completion_gate/...` | ❌ Wave 0 |
| VERIFIED-03 | gate documented in `bench/evaluators/VERIFIED.md` | doc/static gate | `make verify-verified-md` (new) or a test asserting the file's required sections | ❌ Wave 0 |
| SC#4 | canary probe flags contaminated tasks; aggregator column populated | unit (HERMETIC) | `go test ./bench/aggregator/... ./bench/datasets/...` (synthetic contaminated completion trips flag) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/evaluators/... ./bench/datasets/...`
- **Per wave merge:** `go vet ./... && go test ./...`
- **Phase gate:** full suite green; the one-time live "metrics match published reference" run recorded HONESTLY as network-gated, NOT claimed as the proof.

### Wave 0 Gaps
- [ ] `bench/evaluators/exactmatch/exactmatch_test.go` — CCE EM paper fixtures (ADAPTER-CCE-01)
- [ ] `bench/evaluators/editsim/editsim_test.go` — normalized edit-similarity, [0,1] + paper values
- [ ] `bench/evaluators/identmatch/identmatch_test.go` — identifier set EM + F1
- [ ] `bench/evaluators/completion_gate/gate_test.go` — all-three + abstain→false + threshold knob (VERIFIED-03)
- [ ] `bench/datasets/crosscodeeval/loader_test.go` + committed `fixtures/` (py/java/ts/csharp shaped rows)
- [ ] `bench/datasets/repobench/loader_test.go` + committed `fixtures/` (R/C/P; py+java)
- [ ] aggregator canary-column test (synthetic contaminated row trips the flag — not a rubber stamp)
- [ ] `bench/evaluators/VERIFIED.md` (SC#3 acceptance artifact)

## Open Questions (RESOLVED)

1. **Does the Phase 75 HF dataset fetcher infra exist in-tree, or must Phase 86 build it?**
   - **RESOLVED — IT DOES NOT EXIST. Phase 86 must BUILD it.** Phase 75 was "Schema, Fairness Contract & Tree Skeleton" (ROADMAP line 43). `gomlx/go-huggingface` is **absent from go.mod**; `apache/arrow-go/v18` is present but `// indirect` with **zero direct source usage**; `cmd/helix-bench fetch-datasets` is an explicit `notYetImplemented` stub (`main.go:353-358`). **Recommendation:** plan a Wave-1 `bench/datasets/<name>/fetch.go` using stdlib `net/http` + `arrow-go` parquet (promote arrow-go to direct require), reusing `bench/ragindex/cache.go`'s `$HELIX_CACHE_DIR` convention. Treat the ROADMAP/CONTEXT "depends on Phase 75 fetcher" as inaccurate.

2. **Where is the Phase 75 INFRA contamination-canary pattern?**
   - **RESOLVED — IT DOES NOT EXIST YET; the canary is a Phase 89 deliverable** ("Reports, CI Policy & Contamination Canary", ROADMAP line 210-218), which is **Not started**. SC#4 mis-attributes it to Phase 75. **Recommendation:** Phase 86 builds a minimal forward-compatible canary probe in `bench/datasets/<name>/` (inject a known-novel pattern; flag `canary_contaminated`; populate a `canary_pass_rate` aggregator column). Name it to align with the eventual Phase 89 reporter, but do not block on Phase 89.

3. **Where do EM/ES/identifier scorers live and how do they hook into result.v2 metrics?**
   - **RESOLVED.** Phase 79 evaluators live in `bench/evaluators/` with a 5-grader fan-out in `coordinator/coordinator.go`. Each grader returns a `Result{ *pointers; Errs []MetricError }` and the coordinator assembles `evaluators.Metrics`. **Recommendation:** add EM/ES/identifier as new sibling scorer subpackages and a `completion_gate` grader; for completion-only benches, route through a thin completion path (a parallel coordinator or a guarded branch) since there is no `TestOutcome`. `VerifiedCorrectness` is the output key.

4. **Does `verified_correctness` exist on result.v2, or must it be added?**
   - **RESOLVED — IT ALREADY EXISTS.** `evaluators.Metrics.VerifiedCorrectness *bool` (`metrics.go:21`), produced today by `test_runner.Grade` and threaded by the coordinator (`coordinator.go:79`); also mirrored in `aggregator/load.go:32`. **Recommendation:** Phase 86 populates it via the new completion gate (additive, no schema bump). The `test_runner.go:37-40` comment explicitly anticipates a divergent producer — this is that producer.

5. **CCE dataset structure / fixture values?**
   - **RESOLVED (MEDIUM).** CCE (NeurIPS 2023, arXiv 2310.11248): Python/Java/TS/C#; cross-file completion; metrics = Code-Match EM (CM-EM) + Edit Similarity (CM-ES, normalized token-level), Identifier-Match EM (IM-EM) + F1. HF mirror: `Vincentvmt/CrossCodeEval` (community; **has a known viewer cast-error — pin a rev + load per-language explicitly**). Authoritative fixture source: `crosscodeeval.github.io` / amazon-science PDF / official GitHub raw JSONL. **Recommendation:** source fixture (prompt, groundtruth, expected EM/ES/ID) values from the paper/official repo, not the flaky HF auto-loader; gate the live HF fetch.

6. **RepoBench dataset structure / sub-tasks?**
   - **RESOLVED (MEDIUM).** `tianyang/repobench_python_v1.1` / `_java_v1.1` (ICLR 2024, v1.1 GitHub Oct–Dec 2023). Fields: `repo_name, file_path, context[], import_statement, cropped_code, all_code, next_line, gold_snippet_index, token_num, level`. Splits: `cross_file_first` / `cross_file_random` / `in_file`. Sub-tasks: **RepoBench-R** (retrieval, acc@k — rank gold snippet in `context[]` via `gold_snippet_index`), **RepoBench-C** (completion, EM/ES on `next_line`), **RepoBench-P** (pipeline = retrieve-then-complete). Also legacy `tianyang/repobench-r|-c|-p`. **Recommendation:** model all three sub-tasks; R scores acc@k against `gold_snippet_index`, C/P score EM/ES on `next_line`. Pin a rev; sample a subset for the network-gated reference-match test.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Network (HF hub) | Live fetch + reference-match smoke | runtime-dependent | — | SKIP cleanly (env-gated); hermetic fixtures are the proof |
| `apache/arrow-go/v18` | Parquet decode | ✓ (in go.sum, indirect) | v18.5.1 | Promote to direct require |
| `gomlx/go-huggingface` | (ROADMAP's named fetcher) | ✗ | — | Use stdlib `net/http` instead |
| Go toolchain | everything | ✓ | per repo | — |
| Phase 85 LanguageRunners (py/java/ts/csharp) | per-language sanity | ✓ in-tree | — | — |

**Missing dependencies with no fallback:** none (network is gated, not blocking).
**Missing dependencies with fallback:** `gomlx/go-huggingface` → stdlib HTTP; full `go mod tidy` → hand-edit go.mod (s2a-go pre-existing failure, STATE-documented).

## Security Domain

> `security_enforcement` not explicitly false in config; this is a Go data-loader phase with network fetch. Minimal but non-zero surface.

### Applicable controls
| Concern | Applies | Standard Control |
|---------|---------|-----------------|
| Path traversal via dataset rev/lang names into cache path | yes | Validate path segments before `filepath.Join` (mirror `aider-polyglot` `validatePathSegment` / `ragindex` hex-sha-no-separators); reject `..`, absolute, separators. |
| SSRF / arbitrary URL fetch | yes | Pin the HF host + dataset repo as constants (mirror the Phase 83 Ollama base-URL SSRF pin T-83-01-02); never fetch a caller-supplied URL. Pin dataset rev. |
| Untrusted parquet decode | low | arrow-go is the parser; cap downloaded size; do not exec dataset content. |
| Input validation (V5) | yes | Validate decoded rows have required columns before scoring; null-on-failure via `MetricError`, never panic. |

## Sources

### Primary (HIGH confidence — codebase-verified)
- `bench/ragindex/cache.go` — `$HELIX_CACHE_DIR` 3-step cache convention
- `bench/datasets/aider-polyglot/{clone,loader,pin}.go` — closest dataset-adapter precedent (network-gated live + hermetic fixtures)
- `bench/evaluators/metrics.go` — `VerifiedCorrectness *bool` already present
- `bench/evaluators/coordinator/coordinator.go`, `.../test_runner/test_runner.go` — grader fan-out + `verified_correctness` producer pattern
- `bench/aggregator/report.go` — `LeaderRow` / `LanguageRow` / `ByLanguage` (per-language slice + leaderboard columns)
- `cmd/helix-bench/main.go:353` — `fetch-datasets` is a `notYetImplemented` stub
- `go.mod` / `go.sum` — `arrow-go v18.5.1` indirect; `gomlx/go-huggingface` absent
- `.planning/milestones/v1.12-ROADMAP.md:43,179,210` — Phase 75 = skeleton; canary = Phase 89
- `Makefile` — `verify-*` hard-gate pattern

### Secondary (MEDIUM confidence — official docs/papers + HF)
- [CrossCodeEval paper (arXiv 2310.11248)](https://ar5iv.labs.arxiv.org/html/2310.11248) — metrics (CM-EM/ES, IM-EM/F1), 4 languages
- [crosscodeeval.github.io](https://crosscodeeval.github.io/) — official artifacts / fixture source
- [Vincentvmt/CrossCodeEval (HF)](https://huggingface.co/datasets/Vincentvmt/CrossCodeEval) — HF mirror (note: viewer cast-error; pin a rev)
- [tianyang/repobench_python_v1.1 (HF)](https://huggingface.co/datasets/tianyang/repobench_python_v1.1) — fields, splits
- [tianyang/repobench_java_v1.1 (HF)](https://huggingface.co/datasets/tianyang/repobench_java_v1.1) — Java portion
- [RepoBench-R](https://huggingface.co/datasets/tianyang/repobench-r) / [-C](https://huggingface.co/datasets/tianyang/repobench-c) / [-P](https://huggingface.co/datasets/tianyang/repobench-p) — sub-task splits

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | stdlib HTTP + arrow-go is preferable to `gomlx/go-huggingface` | Standard Stack | Low — if go-huggingface is desired, gate it behind a human-verify; stdlib path still works. |
| A2 | `Vincentvmt/CrossCodeEval` is the canonical HF mirror | Open Q5 | Medium — community mirror with a viewer cast-error; verify repo+rev at a checkpoint; fall back to official GitHub/JSONL for fixtures. |
| A3 | RepoBench-R scores acc@k via `gold_snippet_index` over `context[]` | Open Q6 | Medium — confirm the exact retrieval target/metric against the RepoBench paper before locking the R scorer. |
| A4 | CCE identifier-match tokenizer = `[A-Za-z_]\w*` minus keywords | Pitfall 4 | Medium — must match the CCE paper's IM definition exactly or the reference won't reproduce; document in VERIFIED.md. |
| A5 | Promoting arrow-go indirect→direct via `go get` won't trigger the s2a-go tidy failure | Installation | Low — STATE shows chromem/crane promoted the same way; `go get <pkg>` (not full tidy) is the established workaround. |

## Metadata

**Confidence breakdown:**
- In-tree architecture (fetcher-absent, scorers location, verified_correctness exists, cache/leaf patterns): HIGH — directly read from source.
- Dataset schemas & metric formulas: MEDIUM — HF dataset cards + papers, not byte-confirmed against pinned live parquet (network-gated, per design).
- Canary status: HIGH — ROADMAP places it in Phase 89; absent from tree.

**Research date:** 2026-06-21
**Valid until:** ~2026-07-21 (in-tree facts stable; HF dataset cards/revisions may drift — re-pin revs at planning time)

## RESEARCH COMPLETE

**Phase:** 86 - CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate
**Confidence:** HIGH (architecture) / MEDIUM (dataset schemas)

### Key Findings
- **The Phase 75 HF dataset fetcher infra DOES NOT EXIST in-tree.** Phase 75 = schema/fairness/tree skeleton only; `gomlx/go-huggingface` absent from go.mod; `arrow-go` present but indirect/unused; `fetch-datasets` is a `notYetImplemented` stub. **Phase 86 BUILDS the fetcher** (recommend stdlib HTTP + arrow-go parquet, reusing `bench/ragindex` `$HELIX_CACHE_DIR`).
- **The contamination canary is a Phase 89 deliverable, not Phase 75** — also absent. Phase 86 must build a minimal forward-compatible canary probe + aggregator column.
- **`verified_correctness` already exists** (`evaluators.Metrics.VerifiedCorrectness *bool`, produced by `test_runner`); SC#3's multi-oracle gate just adds a completion-path producer — additive, no schema bump. `VERIFIED.md` must be created (does not exist).
- Scorers/gate/canary are PURE logic → hermetic fixture tests are the SOLE authoritative proof; live HF fetch + reference-match is network-gated (SKIP offline). Phase 85 LanguageRunners + `language`/`ByLanguage` aggregator slice are reused verbatim.
- aider-polyglot (Phase 85) is the closest precedent: leaf package, committed fixtures + network-gated live run, cloned `cacheDir()`.

### File Created
`.planning/phases/86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat/86-RESEARCH.md`

### Ready for Planning
Research complete. The critical planner takeaway: this phase BUILDS the HF fetcher + canary (does not reuse them), while the scorers + multi-oracle gate + canary are pure-logic, hermetically-tested core.
