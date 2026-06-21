# Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness` - Research

**Researched:** 2026-06-21
**Domain:** External benchmark adapter (subprocess shellout to a Python harness), multi-oracle test-execution verdict composition, additive result-schema provenance
**Confidence:** HIGH (codebase substrate verified by direct read; external harness/UTBoost field shapes cited from official sources)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
*(CONTEXT.md `## Decisions` is "Claude's Discretion" — discuss phase skipped per `workflow.skip_discuss`. The bullets below are the discretion-framed constraints the planner MUST still honor verbatim, because they encode the success criteria and the environment reality.)*

- The adapter is `subprocess-shellout` to upstream `python -m swebench.harness.run_evaluation` — **fixed argv + strict env allowlist** (reuse the Phase 84 `bench/container/engine.go` discipline / the aider/CCE clone discipline); ingest the harness result JSON → `result.v2.json` preserving **container ID + exit code**.
- The multi-oracle `verified_correctness` (VERIFIED-01/02) reuses the **Phase 86 gate PATTERN** but over **TEST-EXECUTION oracles**: (a) canonical tests pass AND (b) UTBoost-augmented tests pass AND (c) no pre-existing tests regress. It is computed **INDEPENDENTLY of `task_success`** (the existing `evaluators.Metrics.VerifiedCorrectness` field — **additive producer, no schema bump**). **Fail-closed**: any oracle missing/abstain → `verified_correctness=false`, never a false `true`.
- UTBoost augmented suite is **INGESTED from the published source (pinned)**, reproducible from a `--run-id`.
- `differential.go` consumes the **gold patch alongside the agent patch** and emits a **diff-overlap signal**; **run-all-tests override** (not just PR-modified tests).
- **Environment reality:** SWE-bench needs Docker + the upstream `swebench` Python package + per-instance images — **NOT available here**. The adapter code (subprocess wiring, predictions.jsonl producer, harness-result→result.v2 ingestion, UTBoost rescorer, the 3-condition verified_correctness producer, differential.go) MUST be **hermetically tested against committed fixtures** (a sample SWE-bench instance, a sample harness-output JSON, a sample UTBoost augmented suite, a known-buggy-patch case for SC#2). The live 5-task smoke run + container execution are gated on Docker/swebench/network availability and SKIP cleanly. **No live test may be the sole proof; the SC#2 known-buggy-patch (`task_success=true` AND `verified_correctness=false`) MUST be a hermetic test.**

### Claude's Discretion
All implementation choices are at Claude's discretion (discuss skipped). Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

### Deferred Ideas (OUT OF SCOPE)
None — discuss phase skipped. (No requirements beyond ADAPTER-SWE-01, VERIFIED-01, VERIFIED-02 and SC#1–4.)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **ADAPTER-SWE-01** | SWE-bench Verified adapter wired via `subprocess-shellout` — produces `predictions.jsonl`; shells out to `python -m swebench.harness.run_evaluation`; ingests `<run_id>.json`. _Acceptance:_ smoke run of 5 tasks completes; result JSON ingested into bench schema. | §Standard Stack (harness CLI + report filename), §Code Examples (predictions.jsonl producer, fixed-argv builder), §Architecture (ingestion path harness-JSON→result.v2), §Validation Architecture (hermetic ingestion fixture + Docker-gated live smoke) |
| **VERIFIED-01** | `verified_correctness` computed independently of `task_success` — multi-oracle: (a) canonical pass, (b) augmented pass, (c) no pre-existing regress. _Acceptance:_ known-buggy patch passing only canonical → `task_success=true`, `verified_correctness=false`. | §Architecture (the 3-condition producer mirroring `completion_gate.Grade`), §Don't Hand-Roll (reuse `regression_checker` for condition (c)), §Code Examples (the gate), §Validation Architecture (the load-bearing hermetic SC#2 test) |
| **VERIFIED-02** | SWE-bench Verified reports **both** raw upstream + UTBoost-augmented rescored side-by-side. _Acceptance:_ both columns present; UTBoost suite wired and reproducible. | §Standard Stack (UTBoost HF dataset + pinned rev), §Architecture (raw-vs-rescored aggregator column, additive like `CanaryPassRate`/`ByLanguage`), §Code Examples (rescorer) |
</phase_requirements>

## Summary

Phase 87 is a **subprocess-shellout adapter** to the upstream SWE-bench evaluation harness plus an **additive multi-oracle producer** of the already-existing `verified_correctness` metric. The entire phase's hermetically-testable surface is Go glue around two JSON contracts: the **prediction input** (`predictions.jsonl`: `{instance_id, model_name_or_path, model_patch}`) and the **harness output** (a run-report `{model}.{run_id}.json` plus per-instance `report.json` with `resolved` + `tests_status.{FAIL_TO_PASS,PASS_TO_PASS,...}.{success,failure}`). Both shapes are **verified from upstream source** and become committed fixtures.

The codebase already ships every reusable substrate: the **Phase 86 multi-oracle gate** (`bench/evaluators/completion_gate/gate.go`) is the EXACT pattern to clone for the 3-condition test-execution verdict (all-three-required, fail-closed abstain → explicit `&false`, additive `ApplyToMetrics`, no schema bump); the **`regression_checker`** already computes condition (c) "no pre-existing regress" verbatim; **`test_runner`** already proves `task_success` and `verified_correctness` are returned as distinct pointers *precisely so a UTBoost rescore can diverge `verified_correctness` without touching `task_success`* (test_runner.go:37-40); the **Phase 84 `container.Engine`** (fixed-argv + strict env allowlist + procgroup) is the os/exec discipline to mirror; the **aider/CCE pin+clone** pattern is the pinned-dataset-fetch idiom; and the **aggregator** already has two precedents for additive side-by-side columns (`ByLanguage`, `CanaryPassRate`) that never perturb the locked determinism/golden contract.

**Two confirmed schema facts the planner needs:** `result.v2` does **NOT** carry `container_id` or `exit_code` today (grep-verified) — they must be added as **additive open-provenance keys with `omitempty`**, exactly like `embedder_id`/`language`/`ablation_status` (additive-minor, `schema_version` stays `"v2"`, top-level `additionalProperties` stays OPEN, NOT added to `required`). No `differential.go` and no `bench/evaluators/swebench/` package exist yet — both are net-new.

**Primary recommendation:** Build `bench/evaluators/swebench/` as a leaf package of pure Go transforms (predictions producer, harness-report ingester, UTBoost rescorer, 3-condition `verified_correctness` gate cloned from `completion_gate`, `differential.go`), prove ALL of it against committed fixtures, add `container_id`/`exit_code` as `omitempty` open keys, add a raw-vs-rescored aggregator column mirroring `CanaryPassRate`, and gate the live 5-task smoke on Docker+swebench+network (SKIP-clean, never sole proof).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `predictions.jsonl` producer | bench/evaluators/swebench (pure Go transform) | — | Reads the agent's model_patch + instance_id from the cell; emits JSONL. No subprocess, no network → fully hermetic. |
| Shellout to `python -m swebench.harness.run_evaluation` | bench/evaluators/swebench subprocess seam | bench/container discipline (fixed argv, strict env, procgroup) | Upstream harness manages its **own** Docker per-instance (see Open Q4); we shell out to the Python CLI, NOT to `container.Engine.Run`. |
| Harness report JSON → result.v2 ingestion | bench/evaluators/swebench (pure transform) | bench/runtime/result.go (additive `container_id`/`exit_code` keys) | Parse `{model}.{run_id}.json` + per-instance `report.json` → `Metrics` + open provenance. Hermetic over a committed fixture. |
| UTBoost rescore | bench/evaluators/swebench (pure transform) | bench/datasets/swebench-utboost (pinned HF dataset fetch) | Second harness pass over the augmented dataset; rescore is a pure comparison of two report JSONs. |
| 3-condition `verified_correctness` | bench/evaluators/swebench gate (clone of completion_gate) | bench/evaluators/regression_checker (condition c) | Compose canonical∧augmented∧no-regress over test-execution oracles; additive producer of the existing `Metrics.VerifiedCorrectness`. |
| `differential.go` diff-overlap signal | bench/evaluators/swebench (pure transform) | — | Parse gold `patch` + agent `model_patch` unified diffs; emit file-overlap ratio. Hermetic. |
| raw-vs-rescored side-by-side report | bench/aggregator (additive column) | — | Mirror `ByLanguage`/`CanaryPassRate` additive discipline; never alter locked goldens/determinism. |

## Standard Stack

This phase adds **ZERO new Go module dependencies.** It shells out to a Python package and ingests its JSON. All Go work is stdlib + existing internal packages.

### Core (external, invoked via subprocess — NOT a Go dependency)
| Component | Version / Pin | Purpose | Why Standard |
|-----------|---------------|---------|--------------|
| `swebench` (PyPI / GitHub `SWE-bench/SWE-bench`) | latest pinned by tag at run time (NOT vendored) | `python -m swebench.harness.run_evaluation` — the canonical SWE-bench Verified evaluator | The official harness; it owns per-instance Docker image build + test execution + grading. We do not reimplement it. `[CITED: github.com/SWE-bench/SWE-bench]` |
| `princeton-nlp/SWE-bench_Verified` (HF dataset) | 500-task verified subset | `--dataset_name princeton-nlp/SWE-bench_Verified` (canonical run) | The headline benchmark target. `[CITED: huggingface.co/datasets/princeton-nlp/SWE-bench_Verified]` |
| `Bertsekas/SWE-Bench_Verified_UTBoost` (HF dataset) | pin git rev `4c21a4831d80b66e976f2a5ce946a0abded7a2aa` `[ASSUMED — see A1]` | The UTBoost-augmented Verified dataset; drop-in `--dataset_name Bertsekas/SWE-Bench_Verified_UTBoost` for the rescore pass | UTBoost (ACL'25) augments the test suite to catch patches that pass only canonical tests. `[CITED: github.com/CUHK-Shenzhen-SE/UTBoost]` |

### Supporting (existing internal packages — reuse, do not rebuild)
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `bench/evaluators/completion_gate` | The multi-oracle gate PATTERN (all-three-required, fail-closed abstain `&false`, additive `ApplyToMetrics`, no schema bump). | **Clone its structure** for the test-execution 3-condition gate. |
| `bench/evaluators/regression_checker` | `RegressionRate(pre, post)` — already computes "no pre-existing tests regress" (condition c), with the WR-01 skip policy and the empty-denominator null+MetricError path. | Wire as condition (c). Map FAIL_TO_PASS/PASS_TO_PASS lists into `languages.TestOutcome` rows, OR consume PASS_TO_PASS `failure` directly as the regress signal (see Open Q3). |
| `bench/evaluators/test_runner` | Returns `TaskSuccess` and `VerifiedCorrectness` as **distinct pointers** "so Plan 04 may later diverge verified_correctness (e.g. a UTBoost rescore) without touching task_success" (test_runner.go:37-40). | This phase IS that anticipated divergence. |
| `bench/container` (`engine.go`/`run.go`) | Fixed-argv + strict env allowlist (`PATH`/`HOME`/`HELIX_CACHE_DIR`) + `procGroupAttr()` + `errEngineUnavailable` SKIP sentinel. | Mirror the os/exec discipline for the `python -m swebench...` shellout. |
| `bench/datasets/aider-polyglot` (`pin.go`/`clone.go`) | Pinned-SHA fetch, `isHexSHA1` mutable-ref refusal, `HELIX_CACHE_DIR` cache precedence, `HELIX_BENCH_NETWORK`-gated live fetch. | Mirror for the UTBoost augmented-suite fetch (or HF parquet fetch à la crosscodeeval). |
| `bench/runtime/result.go` (`BuildResult`/`ResultInput`/`resultDoc`) | Additive open-provenance key pattern (`embedder_id`, `language`, `ablation_status` — all `omitempty`). | Add `container_id` + `exit_code` here, identically. |
| `bench/aggregator/report.go` | Additive column precedent: `LanguageRow`/`ByLanguage` and `CanaryPassRate` add columns WITHOUT touching locked goldens or the seeded-RNG determinism. | Mirror for the raw-vs-rescored side-by-side column. |
| `bench/evaluators/patch_validator` | Fixed-argv `git diff --name-only` / numstat over a repo working tree; the unified-diff-parsing mindset. | Reference for parsing patch file lists in `differential.go` (though `differential.go` parses patch TEXT, not a working tree — see Open Q5). |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Shell out to `python -m swebench...` | Reimplement grading in Go | Reimplementing the harness (Docker image build, test extraction, FAIL_TO_PASS grading) is exactly the "don't hand-roll" trap. The shellout is the locked decision. |
| Drop-in `--dataset_name Bertsekas/SWE-Bench_Verified_UTBoost` for the rescore pass | Patch the harness to inject augmented tests | UTBoost ships a **complete drop-in dataset** (no harness fork needed) `[CITED: github.com/CUHK-Shenzhen-SE/UTBoost]`. Two `run_evaluation` invocations (canonical dataset + UTBoost dataset) → two report JSONs → rescore compares them. |
| Add `container_id`/`exit_code` as additive open keys | Bump schema to v3 | The repo's additive-only=minor policy (schema `$comment`) makes new optional keys a minor change; v3 is reserved for breaking changes. `omitempty` keeps old artifacts byte-compatible. |

**Installation (Go side):** none — `go build ./...` only. No `go get`.
**External tools (live smoke only, NOT a build dep):**
```bash
# present only on a CI runner that runs the gated live 5-task smoke:
pip install swebench            # the upstream harness (subprocess target)
docker --version                # harness manages its own per-instance containers
```

**Version verification:**
- Go module deps: **none added** — verified by design (the harness is a subprocess, not an import).
- `princeton-nlp/SWE-bench_Verified`, `Bertsekas/SWE-Bench_Verified_UTBoost`: HF dataset names cited from official UTBoost README + the SWE-bench dataset card. The UTBoost pin rev is `[ASSUMED]` (A1) — confirm at plan/human-verify time.

## Package Legitimacy Audit

> This phase adds **ZERO Go module dependencies** (the harness is invoked as a subprocess; the datasets are fetched as data, not imported). There is no Go package to slopsquat-audit. The "packages" below are an external **Python** tool and two **HF datasets** invoked/fetched by the adapter — flagged here for the planner's human-verify gate, not auditable via `npm/pip/cargo` legitimacy seams against the Go tree.

| "Package" | Registry | Provenance | Verdict | Disposition |
|-----------|----------|-----------|---------|-------------|
| `swebench` (Python) | PyPI / GitHub `SWE-bench/SWE-bench` | Official Princeton/SWE-bench org; the canonical harness | OK (external subprocess target) | Approved — invoked only on the Docker-gated live runner |
| `princeton-nlp/SWE-bench_Verified` | Hugging Face | Official dataset card, 500 human-verified tasks | OK | Approved — canonical dataset |
| `Bertsekas/SWE-Bench_Verified_UTBoost` | Hugging Face | UTBoost (CUHK-Shenzhen-SE, ACL'25, MIT) README points here | OK `[ASSUMED — verify HF dataset owner + pin rev at human-verify]` | Approved pending pin confirmation (A1) |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none. **Planner action:** insert a `checkpoint:human-verify` before the first UTBoost fetch to confirm the HF dataset name `Bertsekas/SWE-Bench_Verified_UTBoost` and pin rev `4c21a4831d80b66e976f2a5ce946a0abded7a2aa` against the live UTBoost repo (the rev is `[ASSUMED]`).

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────────────────────┐
  agent model_patch ───► │ predictions producer (pure Go, hermetic)     │
  + instance_id          │  → predictions.jsonl                          │
                         │    {instance_id, model_name_or_path,          │
                         │     model_patch}                              │
                         └───────────────┬─────────────────────────────┘
                                         │ (file)
                  ┌──────────────────────┴───────────────────────────┐
                  │                                                   │
   CANONICAL pass │                                   AUGMENTED pass  │
                  ▼                                                   ▼
   subprocess: python -m swebench.harness.run_evaluation   ... (same) ...
     --dataset_name princeton-nlp/SWE-bench_Verified         --dataset_name
     --predictions_path predictions.jsonl                      Bertsekas/SWE-Bench_Verified_UTBoost
     --run_id <run_id>                                       --run_id <run_id>-utboost
     --instance_ids <5 ids>   [fixed argv + strict env]
                  │                                                   │
                  ▼  (harness owns its own Docker per-instance)       ▼
   {model}.{run_id}.json  +  logs/.../<iid>/report.json   {model}.{run_id}-utboost.json + report.json
   (resolved, tests_status.FAIL_TO_PASS/PASS_TO_PASS)      (augmented tests_status)
                  │                                                   │
                  └──────────────────────┬────────────────────────────┘
                                         ▼
            ┌────────────────────────────────────────────────────────┐
            │  ingest (pure Go transform, hermetic over fixtures):    │
            │   • parse run-report + per-instance report.json         │
            │   • task_success  ← canonical report.resolved           │
            │   • verified_correctness (3-condition gate):            │
            │       (a) canonical FAIL_TO_PASS all success            │
            │       AND (b) UTBoost augmented all success             │
            │       AND (c) regression_checker(no PASS_TO_PASS regress)│
            │       fail-closed: any oracle missing → &false          │
            │   • differential.go: overlap(gold patch, agent patch)   │
            │   • container_id, exit_code  → open provenance keys     │
            └───────────────────────────┬────────────────────────────┘
                                        ▼
              result.v2.json  (additive: container_id, exit_code omitempty)
                                        │
                                        ▼
              aggregator: raw-upstream-score | utboost-rescored-score
                          (additive column, mirrors CanaryPassRate)
```

### Recommended Project Structure
```
bench/evaluators/swebench/           # NET-NEW leaf package
├── predictions.go                   # predictions.jsonl producer (pure)
├── predictions_test.go
├── harness.go                       # fixed-argv builder + subprocess seam (engine.go discipline)
├── harness_test.go                  # hermetic argv golden; live run t.Skip on no-docker/no-swebench
├── report.go                        # harness {model}.{run_id}.json + report.json structs + parse
├── report_test.go                   # hermetic ingestion over committed fixture
├── verified.go                      # 3-condition gate (clone of completion_gate.Grade) + ApplyToMetrics
├── verified_test.go                 # ★ load-bearing SC#2: task_success=true AND verified=false
├── differential.go                  # gold-vs-agent patch overlap signal (pure)
├── differential_test.go
├── rescore.go                       # raw vs UTBoost-augmented rescore (pure compare of two reports)
├── rescore_test.go
├── VERIFIED.md                      # SC acceptance doc (mirror bench/evaluators/VERIFIED.md)
└── testdata/
    ├── instance.json                # one SWE-bench Verified instance row (patch, test_patch, FAIL_TO_PASS, PASS_TO_PASS)
    ├── run_report.json              # {model}.{run_id}.json fixture
    ├── report.canonical.json        # per-instance report: resolved + tests_status
    ├── report.utboost.json          # per-instance augmented report (buggy patch FAILS here)
    └── predictions.golden.jsonl     # expected producer output

bench/datasets/swebench-utboost/     # NET-NEW (or extend bench/datasets/swebench)
├── pin.go                           # PinnedRev + isHexSHA1, mirrors aider-polyglot/pin.go
├── fetch.go                         # HF fetch, HELIX_CACHE_DIR, HELIX_BENCH_NETWORK-gated
└── ...
```

### Pattern 1: Multi-oracle test-execution gate (clone of `completion_gate.Grade`)
**What:** Compose `(a) canonical pass AND (b) augmented pass AND (c) no regress` into one `*bool`, fail-closed.
**When to use:** The `verified.go` producer of `Metrics.VerifiedCorrectness`.
**Example:**
```go
// Source: bench/evaluators/completion_gate/gate.go (PATTERN to clone)
// All-three-required + fail-closed abstain → explicit &false, never nil-drop, no schema bump.
func Grade(canonicalPass, augmentedPass bool, regressed *float64, abstain bool) GateResult {
    if abstain { f := false; return GateResult{VerifiedCorrectness: &f} } // fail-closed
    noRegress := regressed != nil && *regressed == 0                       // condition (c)
    ok := canonicalPass && augmentedPass && noRegress
    okv := ok
    return GateResult{VerifiedCorrectness: &okv, /* per-oracle pointers populated */}
}
// ApplyToMetrics mutates ONLY m.VerifiedCorrectness — additive, no schema field added.
```
**Critical:** `task_success ← canonical report.resolved` is assigned SEPARATELY (distinct pointer), so the SC#2 known-buggy patch lands `task_success=true, verified_correctness=false`.

### Pattern 2: Additive open-provenance key (`container_id`/`exit_code`)
**What:** Add two `omitempty` fields to `ResultInput` + `resultDoc`, mirroring `embedder_id`/`language`.
**When to use:** `bench/runtime/result.go`.
**Example:**
```go
// Source: bench/runtime/result.go (resultDoc — mirror EmbedderID/Language exactly)
ContainerID string `json:"container_id,omitempty"`
ExitCode    *int   `json:"exit_code,omitempty"`  // pointer: 0 is a real exit code, omit only when truly absent
```
**Schema:** add both as OPTIONAL keys in `result.v2.schema.json` (additive-only=minor; `schema_version` stays `"v2"`; do NOT add to `required`; top-level `additionalProperties` stays OPEN). Old goldens stay byte-compatible because `omitempty` drops the key on non-SWE-bench rows.
**Note on `exit_code`:** use `*int` (not `string`/`int`) — a literal `0` is a valid, meaningful exit code, so a value type with `omitempty` would wrongly drop a legitimate `0`. A nil pointer = "no exit code captured".

### Pattern 3: Additive raw-vs-rescored aggregator column (clone of `CanaryPassRate`/`ByLanguage`)
**What:** Add a column that surfaces both scores WITHOUT perturbing the locked goldens or the seeded-RNG determinism.
**When to use:** `bench/aggregator/report.go`.
**Example:** Mirror `LeaderRow.CanaryPassRate` — a flat pooled rate populated on the struct, reduced at score time, that "consumes ZERO RNG so the determinism contract is untouched" and renders an em-dash (not 0) when absent. Add `RawScore`/`RescoredScore` similarly; render only into a SWE-bench-specific report path so existing leaderboard/cost goldens stay byte-for-byte.

### Pattern 4: Fixed-argv subprocess + strict env (clone of `container.Engine`)
**What:** Build the `python -m swebench.harness.run_evaluation ...` argv as a fixed slice, validate inputs before crossing os/exec, strict env allowlist, procgroup ownership, a `runShim` test seam.
**Example:**
```go
// Source: bench/container/engine.go (discipline to mirror)
args := []string{"-m", "swebench.harness.run_evaluation",
    "--dataset_name", datasetName,          // validate against an allowlist of 2 known names
    "--predictions_path", predPath,          // validate: clean, absolute, no '..', no ':'
    "--run_id", runID,                       // validate: [A-Za-z0-9_-]+
    "--instance_ids"}; args = append(args, instanceIDs...)  // each: isValidInstanceID
cmd := exec.CommandContext(ctx, pythonBin, args...)
cmd.SysProcAttr = procGroupAttr()
cmd.Env = allowlistEnv()  // PATH/HOME/HELIX_CACHE_DIR only
```

### Anti-Patterns to Avoid
- **Reading "no failing rows ⇒ pass" off an empty test list** — exactly the `test_runner` Pitfall 4. Authoritative gate for `task_success` is the harness `report.resolved` bool, NOT a count of `tests_status` rows.
- **Deriving `verified_correctness` from `task_success`** — they MUST be independent pointers (VERIFIED-01). Compute `verified` from the 3-condition gate, assign `task_success` separately.
- **Bumping the schema to v3 for `container_id`/`exit_code`** — they are additive-minor open keys.
- **Letting the live Docker smoke be the sole proof** — every load-bearing claim (especially SC#2) MUST have a hermetic fixture test; the live run only SKIP-confirms wiring.
- **Counting an already-failing pre-existing test as a regression** — `regression_checker` already handles this (FAIL_TO_FAIL / skip policy WR-01); reuse it, don't re-derive.
- **Using a mutable HF ref/branch for the UTBoost pin** — mirror `isHexSHA1` refusal (aider-polyglot/pin.go); a moving ref lets upstream swap augmented-test content under us.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SWE-bench grading (image build, test extraction, FAIL_TO_PASS scoring) | A Go reimplementation of the harness | `python -m swebench.harness.run_evaluation` subprocess | The harness is the canonical, validated evaluator; reimplementing it would diverge from the published numbers. |
| "No pre-existing tests regress" (condition c) | A fresh pre/post diff | `bench/evaluators/regression_checker.RegressionRate` | Already encodes the WR-01 skip policy, absent-test handling, and empty-denominator null+MetricError. |
| The all-three-required + fail-closed-abstain verdict shape | A bespoke bool combiner | Clone `bench/evaluators/completion_gate.Grade` + `ApplyToMetrics` | Proven pattern: explicit `&false` on abstain, never nil-drop, additive `Metrics.VerifiedCorrectness`, no schema bump. |
| Fixed-argv + strict-env subprocess | Raw `exec.Command("sh", "-c", ...)` | Clone `bench/container/engine.go` discipline | Shell-injection + env-leak mitigation already solved; `runShim` gives hermetic argv assertion. |
| Pinned dataset fetch | Ad-hoc `git clone` of a branch | Clone `bench/datasets/aider-polyglot` pin+clone (or crosscodeeval HF parquet fetch) | `isHexSHA1` mutable-ref refusal + `HELIX_CACHE_DIR` + `HELIX_BENCH_NETWORK` gating already solved. |
| Additive result field + side-by-side column | A schema v3 / a new golden | `omitempty` open keys (`embedder_id` pattern) + `CanaryPassRate` additive column | Keeps every locked golden byte-stable and determinism untouched. |

**Key insight:** This phase is ~90% **wiring existing patterns to two verified JSON contracts**. The genuinely net-new logic is small: the predictions producer, the report parser, the 3-condition gate (a 30-line clone), the rescore comparator, and `differential.go`. Everything load-bearing is hermetic over committed fixtures.

## Runtime State Inventory

> Not a rename/refactor/migration phase — greenfield additive adapter. The only "state" concerns are: (1) the additive schema keys (covered above — `omitempty`, no migration of old artifacts needed), and (2) the `HELIX_CACHE_DIR` cache for the fetched UTBoost dataset (new subdir, mirrors aider-polyglot — no collision with existing caches). **None of the 5 rename categories apply.**

## Common Pitfalls

### Pitfall 1: Conflating `task_success` and `verified_correctness`
**What goes wrong:** Assigning `verified = task_success` (the current `test_runner` default) silently passes SC#2.
**Why it happens:** `test_runner.Grade` sets both from `post.Passed` today; the naive ingestion copies that.
**How to avoid:** `task_success ← canonical report.resolved`; `verified_correctness ← 3-condition gate`. Keep distinct pointers (test_runner.go:37-40 explicitly anticipates this divergence).
**Warning signs:** The SC#2 hermetic test (`task_success=true AND verified=false`) fails or is absent.

### Pitfall 2: Dropping a legitimate `exit_code: 0`
**What goes wrong:** `ExitCode int json:"exit_code,omitempty"` drops the key when the harness exits 0 — losing the "ran clean" signal.
**How to avoid:** Use `*int` so nil="absent" is distinct from 0="clean exit".
**Warning signs:** A successful run's result.v2 has no `exit_code` key.

### Pitfall 3: Fail-OPEN on a missing oracle
**What goes wrong:** If the UTBoost (augmented) report is absent, defaulting `augmentedPass=true` yields a false `verified=true`.
**Why it happens:** Forgetting that a missing oracle is "unknown", which under fail-closed must drive `verified=false` (or nil+MetricError for "could not score", NOT a true).
**How to avoid:** Mirror `completion_gate`'s abstain path — any missing oracle → `verified=&false`. Never infer pass from absence.
**Warning signs:** A run with no augmented report still reports `verified=true`.

### Pitfall 4: Reproducibility broken by Go map iteration / unsorted JSONL
**What goes wrong:** `predictions.jsonl` emitted in nondeterministic order, or report parse iterates a map, breaking the per-`run_id` reproducibility (SC#3) and byte-stable goldens.
**How to avoid:** Sort instance_ids before emitting JSONL; sort any map keys before rendering (mirror `result.go:fairnessBlock` sort + `report.go` "everything SORTED before emit").
**Warning signs:** Two runs over the same input produce different-bytes `predictions.jsonl` or report.

### Pitfall 5: UTBoost pin is a mutable ref
**What goes wrong:** Pinning the HF dataset to `main`/a branch lets upstream change augmented-test content, making the rescore non-reproducible.
**How to avoid:** Pin a 40-hex commit, refuse non-hex via `isHexSHA1` (aider-polyglot/pin.go); re-pin deliberately.
**Warning signs:** `isHexSHA1` absent; pin value is a branch name.

### Pitfall 6: Live smoke as sole proof of SC#2
**What goes wrong:** The known-buggy-patch verdict only runs when Docker+swebench are present → false-green offline (the documented bench-smoke SKIP trap in MEMORY).
**How to avoid:** SC#2 MUST be a hermetic fixture test (committed buggy-patch report.json where canonical resolves but augmented fails). Live smoke is wiring confirmation only.
**Warning signs:** `go test ./bench/...` passes offline but never exercises the buggy-patch path.

## Code Examples

Verified contracts from official sources (these become committed fixtures):

### `predictions.jsonl` input row
```jsonc
// Source: github.com/SWE-bench/SWE-bench docs/guides/evaluation.md  [CITED]
// One JSON object per line:
{"instance_id": "sympy__sympy-20590", "model_name_or_path": "helix", "model_patch": "diff --git a/sympy/core/sympify.py ..."}
```

### Harness run command
```bash
# Source: github.com/SWE-bench/SWE-bench  [CITED]
python -m swebench.harness.run_evaluation \
  --dataset_name princeton-nlp/SWE-bench_Verified \
  --predictions_path predictions.jsonl \
  --run_id helix-smoke \
  --instance_ids sympy__sympy-20590 ... \
  --max_workers 4 \
  --cache_level instance        # none|base|env|instance
```

### Final run-report JSON (`{model}.{run_id}.json`)
```jsonc
// Source: swebench/harness/reporting.py make_run_report  [CITED]
// filename: model_name_or_path with '/'→'__', then ".{run_id}.json"
{
  "total_instances": 5, "submitted_instances": 5, "completed_instances": 5,
  "resolved_instances": 3, "unresolved_instances": 2,
  "empty_patch_instances": 0, "error_instances": 0,
  "completed_ids": [...], "incomplete_ids": [...], "empty_patch_ids": [...],
  "submitted_ids": [...], "resolved_ids": [...], "unresolved_ids": [...], "error_ids": [...],
  "schema_version": 2
  // + (only with a Docker client) "unstopped_instances","unstopped_containers","unremoved_images"
}
```

### Per-instance report JSON (`logs/run_evaluation/<run_id>/<model>/<instance_id>/report.json`)
```jsonc
// Source: swebench/harness/grading.py get_eval_report  [CITED]
{
  "sympy__sympy-20590": {
    "patch_is_None": false,
    "patch_exists": true,
    "patch_successfully_applied": true,
    "resolved": true,                      // ← task_success authoritative gate
    "tests_status": {
      "FAIL_TO_PASS": {"success": ["test_a"], "failure": []},   // canonical issue tests
      "PASS_TO_PASS": {"success": ["test_b"], "failure": []},   // pre-existing — any failure ⇒ regress
      "FAIL_TO_FAIL": {"success": [], "failure": []},
      "PASS_TO_FAIL": {"success": [], "failure": []}            // a non-empty failure here is a clear regress
    }
  }
}
```

### SWE-bench Verified instance row (the gold patch lives here — for `differential.go` + fixtures)
```jsonc
// Source: huggingface.co/datasets/princeton-nlp/SWE-bench_Verified README  [CITED]
{
  "repo": "sympy/sympy", "instance_id": "sympy__sympy-20590",
  "base_commit": "<sha>", "environment_setup_commit": "<sha>", "version": "1.7",
  "patch": "diff --git a/sympy/core/sympify.py ...",        // ← GOLD patch (differential.go consumes this)
  "test_patch": "diff --git a/sympy/.../test_x.py ...",     // test files the PR added
  "problem_statement": "<issue title+body>",
  "FAIL_TO_PASS": "[\"test_a\"]", "PASS_TO_PASS": "[\"test_b\"]"  // JSON-encoded string lists
}
```

### 3-condition `verified_correctness` (the gate to write — clone of completion_gate)
```go
// task_success and verified_correctness are INDEPENDENT (VERIFIED-01).
taskSuccess := canonical.Resolved                          // report.resolved

canonicalPass := allSucceed(canonical.TestsStatus.FailToPass)  // (a)
augmentedPass := augmentedReport != nil &&
    allSucceed(augmentedReport.TestsStatus.FailToPass)         // (b); nil ⇒ fail-closed below
noRegress := len(canonical.TestsStatus.PassToPass.Failure) == 0 &&
    len(canonical.TestsStatus.PassToFail.Failure) == 0         // (c)

abstain := augmentedReport == nil                          // missing oracle ⇒ fail-closed
gr := Grade(canonicalPass, augmentedPass, noRegress, abstain)  // → &false on abstain
gr.ApplyToMetrics(&m)                                       // additive; mutates only VerifiedCorrectness
m.TaskSuccess = &taskSuccess                               // assigned separately — divergence proven
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SWE-bench grading = "canonical PR tests pass" | UTBoost augmented suites catch patches passing only canonical tests | UTBoost, ACL'25 (arXiv 2506.09289, Jun 2025) | 15.7% more incorrect patches caught on Verified; `verified_correctness` must include the augmented oracle. `[CITED: arxiv.org/abs/2506.09289]` |
| SWE-bench harness with bespoke per-run code | `python -m swebench.harness.run_evaluation` CLI + `{model}.{run_id}.json` + per-instance `report.json` | Current `SWE-bench/SWE-bench` main | Stable JSON contract → committed fixtures are durable. `[CITED: github.com/SWE-bench/SWE-bench]` |

**Deprecated/outdated:**
- The org moved `princeton-nlp/SWE-bench` → `SWE-bench/SWE-bench` (GitHub). The **dataset** remains `princeton-nlp/SWE-bench_Verified` on HF (the `--dataset_name` value). Use the HF dataset name for `--dataset_name`; use the new GitHub org for harness source links.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | UTBoost augmented Verified dataset is `Bertsekas/SWE-Bench_Verified_UTBoost` (HF), pin rev `4c21a4831d80b66e976f2a5ce946a0abded7a2aa`, MIT | Standard Stack / Audit | Wrong rescore source → VERIFIED-02 reproducibility false. **Mitigation:** `checkpoint:human-verify` before first fetch (confirm owner + rev against github.com/CUHK-Shenzhen-SE/UTBoost README). |
| A2 | The augmented dataset is a complete drop-in for `--dataset_name` (no harness fork) | Architecture | If it needs a patched harness, the rescore pass changes shape. **Mitigation:** verify on the live runner; hermetic rescore (compare two report JSONs) is invariant either way. |
| A3 | Per-instance `report.json` path is `logs/run_evaluation/<run_id>/<model>/<instance_id>/report.json` | Code Examples | Wrong path → live ingestion can't find reports (hermetic tests unaffected — they read committed fixtures). **Mitigation:** confirm the exact log dir layout on the live runner. |
| A4 | `report.resolved` is THE authoritative `task_success` gate (not a tests_status count) | Architecture / Pitfalls | If `resolved` semantics differ, task_success is mis-derived. Low risk — `resolved` is the documented success bool. |
| A5 | UTBoost's "run-all-tests vs PR-modified-tests-only" override (SC#4) is expressed via the augmented dataset's FAIL_TO_PASS/PASS_TO_PASS lists (broader sets), not a separate harness flag | Open Q2 | SC#4 "run-all-tests override is wired" may need a different mechanism. **Mitigation:** Open Q2 below; confirm at human-verify. |

## Open Questions (RESOLVED)

1. **Does `result.v2` already carry `container_id`/`exit_code`?** — **RESOLVED: NO.** Grep-verified absent from `bench/schema/result.v2.schema.json` and `bench/runtime/result.go`. **Recommendation:** add both as **additive `omitempty` open-provenance keys** (`container_id string`, `exit_code *int`) mirroring `embedder_id`/`language`/`ablation_status` — `schema_version` stays `"v2"`, NOT added to `required`, top-level `additionalProperties` stays OPEN. Use `*int` for exit_code so a clean `0` is preserved.

2. **How is "run-all-tests override (not just PR-modified)" (SC#4) wired?** — **RESOLVED (with A5 caveat):** UTBoost's value is precisely that its augmented dataset carries a **broader FAIL_TO_PASS/PASS_TO_PASS set** than the canonical PR-modified-only tests; running the harness over the UTBoost dataset IS the "run-all-tests" override (the augmented tests exercise paths the PR's own tests miss). **Recommendation:** wire SC#4 as "the rescore pass uses `--dataset_name <UTBoost>` whose test sets are the augmented (run-all) sets", and prove `differential.go`'s diff-overlap separately. Confirm at human-verify whether UTBoost additionally exposes a harness flag (A5).

3. **Should condition (c) "no regress" reuse `regression_checker` (which wants `languages.TestOutcome` rows) or read `PASS_TO_PASS.failure` directly?** — **RESOLVED:** Read `PASS_TO_PASS.failure` (and `PASS_TO_FAIL.failure`) **directly from the harness report** for the gate's boolean condition (c) — it's the harness's own pre-existing-test verdict, simpler and exact. **Optionally** also map the lists into `languages.TestResult` rows and call `regression_checker.RegressionRate` to populate the numeric `regression_rate` metric for the report. Recommendation: use the direct read for the gate bool (SC#2 correctness), and optionally surface `regression_rate` via the existing checker for the leaderboard. (Don't make SC#2 depend on the numeric path.)

4. **Does the SWE-bench harness use our Phase 84 `container.Engine`, or its own Docker?** — **RESOLVED: its own.** The upstream harness builds and runs per-instance Docker images itself (`--cache_level`, `--max_workers`, and the report's `unstopped_containers`/`unremoved_images` keys confirm it manages containers internally). **Recommendation:** the adapter shells out to the **Python CLI** (mirroring `container.Engine`'s argv/env discipline) and does NOT route through `container.Engine.Run`. The Phase 84 container substrate is reused as a *discipline reference* (fixed argv, strict env, procgroup, `errEngineUnavailable`-style SKIP), not as the execution path. Gate the live smoke on `python -m swebench` + `docker` availability.

5. **What exactly does `differential.go` consume?** — **RESOLVED:** the **gold `patch`** (from the SWE-bench instance row) and the **agent `model_patch`** (from `predictions.jsonl`) — both unified-diff TEXT. Parse `diff --git a/<f> b/<f>` headers from each to get the touched-file sets; emit **diff-overlap = |gold_files ∩ agent_files| / |gold_files|** (and optionally Jaccard). This is a **pure text transform** (no working tree, unlike `patch_validator` which shells `git`), so it is fully hermetic over committed patch fixtures. **Recommendation:** stdlib-only diff-header parse; null/empty-gold → null signal + MetricError (mirror the empty-denominator discipline), never a fabricated 0 or 1.

## Environment Availability

| Dependency | Required By | Available (here) | Version | Fallback |
|------------|------------|------------------|---------|----------|
| Go toolchain | All Go code + tests | ✓ | repo go.mod | — |
| `python` + `swebench` pkg | Live 5-task smoke (SC#1) | ✗ | — | SKIP-clean (hermetic ingestion fixtures are the proof) |
| Docker / podman | Harness per-instance execution | ✗ | — | SKIP-clean (`errEngineUnavailable`-style) |
| Network (HF) | UTBoost + Verified dataset fetch | ✗ (gated) | — | `HELIX_BENCH_NETWORK`-gated; committed fixtures cover offline |
| Per-instance SWE-bench images | Live test execution | ✗ | — | SKIP-clean |

**Missing dependencies with no fallback:** none — every SWE-bench-side dependency has a SKIP-clean live path PLUS a hermetic fixture proof.
**Missing dependencies with fallback:** Docker, `swebench`, network, per-instance images — all gated and SKIP-clean; the hermetic fixture tests (ingestion, rescore, 3-condition verified, differential) are the authoritative proof per the locked decision.

## Validation Architecture

> `workflow.nyquist_validation` is `true` in config → section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `go test`) |
| Config file | none (Go convention) |
| Quick run command | `go test ./bench/evaluators/swebench/...` |
| Full suite command | `go vet ./... && go test ./...` (per CLAUDE.md: always run `go vet` + `go test`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| ADAPTER-SWE-01 | `predictions.jsonl` producer emits `{instance_id,model_name_or_path,model_patch}`, sorted | unit (hermetic) | `go test ./bench/evaluators/swebench/ -run TestPredictions -x` | ❌ Wave 0 |
| ADAPTER-SWE-01 | fixed-argv builder for `python -m swebench...` (golden argv, no live engine) | unit (hermetic) | `go test ./bench/evaluators/swebench/ -run TestHarnessArgv -x` | ❌ Wave 0 |
| ADAPTER-SWE-01 | harness report JSON → `result.v2` ingestion incl. `container_id`/`exit_code` | unit (hermetic, committed fixture) | `go test ./bench/evaluators/swebench/ -run TestIngest -x` | ❌ Wave 0 |
| ADAPTER-SWE-01 | live 5-task smoke completes | integration (Docker+swebench-gated) | `go test ./bench/evaluators/swebench/ -run TestLiveSmoke` (SKIPs without env) | ❌ Wave 0 |
| **VERIFIED-01** | **★ buggy patch: `task_success=true` AND `verified_correctness=false`** | **unit (hermetic — LOAD-BEARING)** | `go test ./bench/evaluators/swebench/ -run TestVerified_BuggyPatch -x` | ❌ Wave 0 |
| VERIFIED-01 | all-three-pass → `verified=true`; any single fail → `false`; missing oracle → fail-closed `&false` | unit (hermetic) | `go test ./bench/evaluators/swebench/ -run TestVerified_Gate -x` | ❌ Wave 0 |
| VERIFIED-02 | rescore surfaces raw + UTBoost-augmented side-by-side; reproducible per `run_id` | unit (hermetic) | `go test ./bench/evaluators/swebench/ -run TestRescore -x` | ❌ Wave 0 |
| VERIFIED-02 | aggregator raw-vs-rescored column (no golden perturbation) | unit | `go test ./bench/aggregator/ -run TestSwebenchColumns -x` | ❌ Wave 0 |
| SC#4 | `differential.go` diff-overlap(gold, agent) signal | unit (hermetic) | `go test ./bench/evaluators/swebench/ -run TestDifferential -x` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/evaluators/swebench/...` (hermetic, <5s)
- **Per wave merge:** `go vet ./... && go test ./...`
- **Phase gate:** full suite green offline; live smoke run once on a Docker+swebench runner (SKIP-clean elsewhere) before `/gsd-verify-work`. **Per MEMORY (bench false-green):** plain `go test ./...` SKIPs the live legs — the hermetic fixtures (especially SC#2) are the authoritative gate, not the SKIP-able live smoke.

### Wave 0 Gaps
- [ ] `bench/evaluators/swebench/testdata/{instance.json,run_report.json,report.canonical.json,report.utboost.json,predictions.golden.jsonl}` — committed fixtures (the SOLE authoritative proof). The buggy-patch fixture: `report.canonical.json` has `resolved=true` + canonical FAIL_TO_PASS all success, while `report.utboost.json` has augmented FAIL_TO_PASS with a `failure` → drives `verified=false` while `task_success=true`.
- [ ] `bench/evaluators/swebench/*_test.go` — all unit tests above.
- [ ] `bench/evaluators/swebench/VERIFIED.md` — SC acceptance doc (mirror `bench/evaluators/VERIFIED.md`; the existing `verify-verified-md` Makefile gate may need a sibling/extension).
- [ ] No framework install needed (Go stdlib `testing`).

## Security Domain

> `security_enforcement` not present in config → treat as enabled. This phase's surface is **subprocess invocation + untrusted-data ingestion**, not network services or auth.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | no | no auth surface |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | **yes** | Validate every value crossing os/exec: dataset name (allowlist of 2), `predictions_path`/log paths (clean+absolute+no `..`+no `:`, mirror `isValidMountPath`), `run_id` (`[A-Za-z0-9_-]+`), each `instance_id` (`<owner>__<repo>-<num>` shape). Validate the UTBoost pin via `isHexSHA1`. |
| V6 Cryptography | no | dataset pin is a content commitment, not a crypto control (verification is upstream's concern) |

### Known Threat Patterns for `subprocess-shellout + JSON ingestion`
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Argv flag-smuggling (a crafted instance_id/path parsed as a flag) | Tampering | Fixed argv slice + `--` terminator + leading-`-` rejection (mirror `container/run.go`); never `sh -c`. |
| Env leakage to the subprocess | Information disclosure | Strict env allowlist `PATH`/`HOME`/`HELIX_CACHE_DIR` (mirror `engine.go:allowlistEnv`); never the inherited env. |
| Malicious/oversized harness JSON or dataset parquet | DoS / Tampering | `io.LimitReader` on fetched bodies (mirror crosscodeeval 256MiB cap); strict struct unmarshal; treat all ingested JSON as untrusted. |
| Mutable dataset ref swap | Tampering | `isHexSHA1` pin refusal on the UTBoost rev (mirror aider-polyglot/pin.go). |
| Process orphaning on cancel | (availability) | `procGroupAttr()` Setpgid so a ctx cancel group-kills the harness + its Docker descendants (mirror `engine.go`). |

## Sources

### Primary (HIGH confidence)
- `bench/evaluators/completion_gate/gate.go`, `bench/evaluators/metrics.go`, `bench/evaluators/VERIFIED.md` — the multi-oracle gate pattern + the `VerifiedCorrectness` field (direct read).
- `bench/evaluators/test_runner/test_runner.go` (lines 37-40) — distinct task_success/verified pointers anticipating the UTBoost divergence (direct read).
- `bench/evaluators/regression_checker/regression_checker.go` — condition (c) "no pre-existing regress" (direct read).
- `bench/evaluators/coordinator/coordinator.go` — D-07 fan-out / non-short-circuit grading (direct read).
- `bench/runtime/result.go` — `BuildResult`/`resultDoc` additive open-key pattern; **grep-verified absence of `container_id`/`exit_code`** (direct read).
- `bench/container/engine.go`, `bench/container/run.go` — fixed-argv + strict-env + procgroup discipline (direct read).
- `bench/datasets/aider-polyglot/pin.go`, `clone.go` — pinned fetch + `isHexSHA1` + `HELIX_CACHE_DIR`/`HELIX_BENCH_NETWORK` gating (direct read).
- `bench/aggregator/report.go` — additive `CanaryPassRate`/`ByLanguage` column precedent (direct read).
- `bench/languages/runner.go` — `TestOutcome`/`TestResult` shape (direct read).

### Secondary (MEDIUM confidence — cited from official upstream sources)
- [github.com/SWE-bench/SWE-bench docs/guides/evaluation.md](https://github.com/SWE-bench/SWE-bench/blob/main/docs/guides/evaluation.md) — predictions.jsonl format, CLI flags.
- [swebench/harness/reporting.py make_run_report](https://raw.githubusercontent.com/SWE-bench/SWE-bench/main/swebench/harness/reporting.py) — run-report keys + `{model}.{run_id}.json` filename.
- [swebench/harness/grading.py get_eval_report](https://raw.githubusercontent.com/SWE-bench/SWE-bench/main/swebench/harness/grading.py) — per-instance report shape (resolved, tests_status).
- [huggingface.co/datasets/princeton-nlp/SWE-bench_Verified](https://huggingface.co/datasets/princeton-nlp/SWE-bench_Verified) — instance row fields (patch, test_patch, FAIL_TO_PASS, PASS_TO_PASS).
- [github.com/CUHK-Shenzhen-SE/UTBoost](https://github.com/CUHK-Shenzhen-SE/UTBoost) — UTBoost HF dataset names, drop-in integration, MIT.
- [arxiv.org/abs/2506.09289](https://arxiv.org/abs/2506.09289) — UTBoost (ACL'25): 15.7% more incorrect patches on Verified.

### Tertiary (LOW confidence — `[ASSUMED]`, confirm at human-verify)
- UTBoost pin rev `4c21a4831d80b66e976f2a5ce946a0abded7a2aa` (A1) — surfaced via web fetch, not confirmed against the live repo tag.
- Exact per-instance report.json log-dir path (A3) — confirm on the live runner.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new Go deps (verified by design); external harness/dataset names cited from official sources; UTBoost pin `[ASSUMED]`.
- Architecture: HIGH — every pattern is a verified clone of existing in-tree code; the two JSON contracts are cited from upstream source.
- Pitfalls: HIGH — derived from in-tree decisions (test_runner.go:37-40, MEMORY bench-false-green, completion_gate fail-closed) and the cited report shapes.
- Open questions: all RESOLVED with recommendations; 5 `[ASSUMED]` items logged for human-verify.

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable in-tree substrate; UTBoost/SWE-bench upstream is fast-moving — re-verify dataset names + pin if planning slips >30 days)

## RESEARCH COMPLETE

**Phase:** 87 - SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness`
**Confidence:** HIGH

### Key Findings
- **`result.v2` does NOT have `container_id`/`exit_code`** (grep-verified) — add both as additive `omitempty` open-provenance keys (`exit_code` as `*int` to preserve a clean `0`), exactly like `embedder_id`/`language`/`ablation_status`. No schema v3 bump.
- The **Phase 86 `completion_gate.Grade`** is the exact pattern to clone for the 3-condition test-execution `verified_correctness`; **`test_runner.go:37-40` explicitly anticipates this UTBoost divergence** (distinct task_success/verified pointers); **`regression_checker`** already computes condition (c).
- Both load-bearing JSON contracts are **verified from upstream source**: predictions `{instance_id, model_name_or_path, model_patch}`; run-report `{model}.{run_id}.json` keys; per-instance `report.json` `{resolved, tests_status.{FAIL_TO_PASS,PASS_TO_PASS,...}.{success,failure}}` → committed fixtures.
- **UTBoost integrates as a drop-in `--dataset_name Bertsekas/SWE-Bench_Verified_UTBoost`** (no harness fork); rescore = pure compare of two report JSONs; the aggregator raw-vs-rescored column mirrors the additive `CanaryPassRate`.
- The harness **manages its own Docker** — shell out to the Python CLI (mirroring `container.Engine` argv/env discipline), NOT through `container.Engine.Run`. Zero new Go module deps.

### File Created
`.planning/phases/87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver/87-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | No new Go deps (by-design verified); harness/dataset cited from official sources; UTBoost pin `[ASSUMED]` |
| Architecture | HIGH | Every pattern is a verified in-tree clone; JSON contracts cited from upstream source |
| Pitfalls | HIGH | Sourced from in-tree decisions + cited report shapes + the MEMORY false-green trap |

### Open-Question Disposition Summary
All 5 RESOLVED with recommendations; 5 `[ASSUMED]` items logged (A1–A5) for a single `checkpoint:human-verify` (confirm UTBoost dataset name+rev, drop-in vs fork, report log path, run-all-tests mechanism) — none block hermetic-fixture development.

### Ready for Planning
Research complete. The planner can build `bench/evaluators/swebench/` as a hermetic leaf package against committed fixtures, add the two additive result keys, clone the completion_gate verdict for the 3-condition `verified_correctness`, and gate the live 5-task smoke Docker+swebench+network SKIP-clean.
