# Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters - Research

**Researched:** 2026-06-21
**Domain:** Bench-stack subprocess-shellout adapters (Go), upstream Python/CLI harness wiring, per-language result slicing, long-wall checkpoint/resume state machine
**Confidence:** HIGH (in-tree template + reuse surface); MEDIUM (upstream config/JSON shapes — verified via official repos/docs but NOT runnable here; pin live-confirmation deferred per Phase 87 precedent)

## Summary

Phase 88 adds the **final two external adapters** by cloning the Phase 87 `bench/evaluators/swebench/` template. Both are `subprocess-shellout` wrappers: Multi-SWE-bench shells to `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`; Terminal-Bench 2.0 shells to the `tb run` CLI. The template (`harness.go` fixed-argv + strict env allowlist + controlled WorkDir + procGroupAttr + `Detect()` skip-gate; `ingest.go` harness-JSON → `runtime.ResultInput`; `report.go` size-capped strict JSON parsers; `predictions.go` deterministic sorted producer; `differential.go` run-all-tests/overlap signal) is directly reusable — the *shape* is identical, only the argv builder, the config/JSON struct definitions, and the per-instance→result mapping change.

The three genuinely **net-new** pieces are: (1) a **config.json producer** for Multi-SWE-bench (the harness is config-file-driven, not flag-driven like SWE-bench — this is the one structural divergence from the template); (2) the **`tb run` JSON ingestion** (different report shape); and (3) the **long-wall scheduler + per-cell checkpoint/resume state machine** — **confirmed NOT to exist anywhere in `bench/`** today (the only `Checkpoint` in-tree is the unrelated DuckDB WAL flush in `internal/semantic/store/overlay.go`). The checkpoint substrate already exists, though: `writeDurable` (cell.go:923) is exactly the atomic temp-file+rename primitive a checkpoint writer needs.

All of it is **hermetically testable against committed fixtures** (sample config.json golden, sample `final_report.json`, sample `tb` `results.json`, a checkpoint-resume case) with the live Mini-set + tb smoke runs SKIPping cleanly — Docker, `multi_swe_bench`, and `tb` are all absent here. This mirrors Phase 87 exactly (live runs gated, hermetic fixtures are sole proof).

**Primary recommendation:** Clone `bench/evaluators/swebench/` into `bench/evaluators/multiswebench/` and `bench/evaluators/terminalbench/`; keep `harness.go`/`ingest.go`/`report.go` structure verbatim, swap the argv builder for a **config.json producer** (Multi-SWE) / a `tb run` argv builder (Terminal-Bench), define the upstream-shaped report structs, and map per-instance → `runtime.ResultInput`. Build the checkpoint/resume state machine as a new pure package (`bench/longwall/` or `bench/runtime/checkpoint.go`) on top of `writeDurable`. Per-language slicing is **already done** by the aggregator — just emit one cell per language with `Cell.Language` set.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Multi-SWE config.json production | bench adapter (`multiswebench`) | — | Pure Go transform: instances + paths → validated JSON file the Python harness reads |
| Multi-SWE harness invocation | bench adapter subprocess seam | upstream Docker (harness-owned) | The harness owns its own Docker per-instance (same as Phase 87); we shell the Python CLI, never `container.Engine.Run` |
| `tb run` invocation | bench adapter subprocess seam | upstream Docker (DockerComposeManager) | tb owns per-task container isolation; we shell the CLI |
| harness/tb JSON → result.v2 | bench adapter (`ingest.go`) | `bench/runtime` builder | Pure transform over JSON the harness already produced |
| Per-language slicing | `bench/aggregator` (existing) | — | `reduceLanguageRows`/`rowLanguage` already slice the `language` doc key; adapters only need to stamp `Cell.Language` |
| Long-wall scheduler + checkpoint/resume | NEW pure package (`bench/longwall`) | `writeDurable` primitive | Net-new state machine; reuses the atomic write primitive, no Docker |
| Container isolation invariant (SC#2) | upstream tb (DockerComposeManager) | adapter assertion | tb spawns a fresh container per task; adapter asserts via tb's own per-task isolation, does NOT re-implement |
| License audit | `bench/LICENSES.md` (existing) | — | One appended row per dataset (INFRA-02) |

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADAPTER-MULTI-01 | Multi-SWE-bench via `python -m multi_swe_bench.harness.run_evaluation --config`; Java/TS/JS/Go/Rust/C/C++; Mini set at ship, full reach goal; per-language slicing in reporter | Config.json schema documented (18 fields, verbatim); Mini set = `bytedance-research/Multi-SWE-bench_mini` (400 instances, 8 langs) per upstream README; `final_report.json` mirrors SWE-bench `resolved_ids`/`unresolved_ids`; language comes from the per-language dataset-file directory (`go/`, `java/`, ...) → maps to `Cell.Language`; aggregator slicing already built (`reduceLanguageRows`) |
| ADAPTER-TERM-01 | Terminal-Bench 2.0 via `tb run`; ingest `tb` JSON; smoke ≥5 tasks; container isolation holds | `tb run` flags documented; `runs/<ts>/results.json` aggregate (`accuracy`,`n_resolved`,`n_unresolved`) + per-trial `<task_id>/<trial>/results.json` (`is_resolved`); 89-task dataset; DockerComposeManager per-task isolation; `tb.lock` is upstream's own resume snapshot |

## Standard Stack

This phase adds **ZERO new Go module dependencies** (consistent with every prior bench adapter). Both adapters are stdlib-only Go transform packages that shell out to external CLIs; the external tools (`multi_swe_bench`, `tb`/`harbor`, Docker) are runtime subprocess dependencies, not Go imports.

### Core (in-tree reuse, no install)
| Component | Path | Purpose | Why reuse |
|-----------|------|---------|-----------|
| swebench template | `bench/evaluators/swebench/` | fixed-argv subprocess + env allowlist + WorkDir + procGroup + ingest + strict JSON parse | Proven Phase 87 pattern; both adapters clone it |
| result.v2 builder | `bench/runtime/result.go` (`ResultInput`) | container_id/exit_code/language open keys already exist | No schema change needed (additive-minor keys present) |
| aggregator slicing | `bench/aggregator/aggregate.go` (`reduceLanguageRows`, `rowLanguage`) | per-language pass-rate slice on the `language` doc key | SC#1 per-language slicing already implemented |
| atomic write | `bench/runtime/cell.go` (`writeDurable`) | temp-file+rename atomic durable write | Exactly the checkpoint-write primitive |
| matrix scheduler | `bench/runtime/matrix.go` (`ExpandMatrix`/`RunMatrix`/`dispatch`) | bounded-parallel cell dispatch with per-cell outcome capture | Long-wall scheduler extends this, not replaces it |
| dataset pin/fetch idiom | `bench/datasets/swebench-utboost/{pin,fetch}.go` | pinned-SHA + SSRF host pin + `io.LimitReader` cap + `HELIX_BENCH_NETWORK`-gated live fetch + `HELIX_CACHE_DIR` precedence | Clone verbatim for the Mini-set dataset fetcher |

### Supporting (external runtime subprocess deps — NOT installed here)
| Tool | Invocation | Availability | Skip gate |
|------|-----------|--------------|-----------|
| `multi_swe_bench` Python pkg | `python -m multi_swe_bench.harness.run_evaluation --config <json>` | ✗ (absent) | `Detect()` PATH probe → `t.Skip` |
| `tb` CLI (Terminal-Bench) | `tb run --agent ... --dataset-name ... --dataset-version ...` | ✗ (absent) | `Detect()` PATH probe → `t.Skip` |
| Docker | harness-owned (Multi-SWE) / DockerComposeManager (tb) | ✗ (absent) | live tests skip when no engine |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `python -m multi_swe_bench.harness.run_evaluation` (CONTEXT-locked) | direct dataset eval re-impl | Locked by ADAPTER-MULTI-01 + CONTEXT; do NOT fork the harness |
| `tb run` (CONTEXT-locked) | `harbor run` (the 2.0-native framework) | **See Open Question O-1:** TB 2.0 leaderboard runs go through `harbor run`; `tb run` is the legacy 1.x CLI. CONTEXT locks `tb run`. Recommend: keep the argv builder behind a small `runnerKind` seam so the binary name is one constant — do NOT block on it; the JSON ingestion is what matters and is the same shape |
| new checkpoint state machine | reuse `tb.lock` (tb's own resume) | tb's `tb.lock` only resumes a single tb invocation; the long-wall scheduler checkpoints OUR cell matrix across harness restarts — net-new, but complementary |

**Installation:** None. `go build ./cmd/helix-bench` only. No `go get`, no go.mod edit (every prior adapter held this line; deviating would be a regression).

**Version verification:** N/A for Go deps (none added). External-tool versions are runtime concerns confirmed only on a Docker+network host (deferred, per Phase 87 precedent).

## Package Legitimacy Audit

> **Not applicable — zero external packages installed.** This phase adds no Go module dependencies (`go get` is forbidden by the established bench-adapter convention). The external runtime tools (`multi_swe_bench`, `tb`, Docker) are invoked as subprocesses, never imported. No registry/slopsquat surface exists to audit. The `package-legitimacy check` seam was therefore not run — there is nothing to install.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                          MULTI-SWE-BENCH ADAPTER (clone of swebench/)
  Cell{lang=go|java|...}                                    upstream (absent here)
        │                                                         │
        ▼                                                         │
  [config.go]  ── produce config.json ──┐                        │
   (NEW: file producer, not flag argv)  │  validated, atomic     │
        │                               ▼  writeDurable           ▼
  [harness.go] ── fixed argv ──▶ python -m multi_swe_bench ──▶ Docker (harness-owned)
   RunArgs: ["-m","multi_swe_bench.harness.run_evaluation",     per-instance images
             "--config", <path>]                                     │
   + allowlistEnv + procGroupAttr + WorkDir + Detect()/Skip          ▼
        │                                                      final_report.json
        ▼                                                      + per-instance reports
  [ingest.go] ◀────────── parse (size-capped, strict) ──────────────┘
   per-instance{resolved} → ResultInput{TaskSuccess, Language, ContainerID, ExitCode}
        │
        ▼
  [bench/runtime builder] ── result.v2.json (language key stamped from Cell.Language)
        │
        ▼
  [bench/aggregator] ── reduceLanguageRows ── ByLanguage[] (per-lang pass-rate)  ← SC#1


                          TERMINAL-BENCH 2.0 ADAPTER (clone of swebench/)
  Cell{task}
        │
        ▼
  [harness.go] ── fixed argv ──▶ tb run --agent.. --dataset-name terminal-bench-core
   + allowlistEnv + procGroupAttr      --dataset-version <v> --task-id <id> --output-path <dir>
        │                                              │
        │                                              ▼
        │                                   DockerComposeManager
        │                                   (FRESH container per task)  ← SC#2 invariant
        ▼                                              │
  [ingest.go] ◀── parse runs/<ts>/results.json ───────┘
   per-trial{is_resolved} → ResultInput{TaskSuccess, ContainerID}


                          LONG-WALL SCHEDULER + CHECKPOINT  (NEW, pure Go, no Docker)
  cells[]
        │
        ▼
  [longwall.Scheduler] ──for each cell──▶ readCheckpoint(cellKey)
        │                                       │
        │                              ┌────────┴────────┐
        │                          done?  no             yes → SKIP (idempotent re-entry)
        │                              ▼
        │                         run cell ──▶ writeCheckpoint(cellKey, DONE)  [atomic]
        ▼                                       (via writeDurable temp+rename)
   (harness restart at any point) ──resume──▶ re-read checkpoints, skip completed cells
```

### Recommended Project Structure
```
bench/evaluators/
├── multiswebench/          # NEW — clone of swebench/
│   ├── config.go           # NEW: config.json producer (the one structural divergence)
│   ├── config_test.go      #   golden config.json + validation refusals
│   ├── harness.go          # cloned: RunArgs builds ["-m","...run_evaluation","--config",<path>]
│   ├── ingest.go           # cloned: final_report/per-instance → ResultInput (+ Language)
│   ├── report.go           # cloned: strict size-capped parsers for final_report.json
│   ├── procgroup_unix.go / procgroup_windows.go  # copied verbatim
│   └── testdata/           # sample config.json (golden), sample final_report.json, per-instance reports per language
├── terminalbench/          # NEW — clone of swebench/
│   ├── harness.go          # cloned: RunArgs builds tb run argv
│   ├── ingest.go           # cloned: results.json → ResultInput
│   ├── report.go           # cloned: strict parsers for run results.json (aggregate + per-trial)
│   └── testdata/           # sample runs/<ts>/results.json + per-trial results.json
└── swebench/               # UNCHANGED template

bench/longwall/             # NEW pure package (or bench/runtime/checkpoint.go)
├── checkpoint.go           # readCheckpoint / writeCheckpoint (atomic) / cellKey
├── scheduler.go            # resume-aware dispatch over cells, idempotent re-entry
└── *_test.go               # hermetic: write→read→resume, mid-run-restart, idempotency

bench/datasets/multi-swe-bench-mini/   # NEW — clone of swebench-utboost/
├── pin.go                  # PinnedSHA + DatasetID + Host (SSRF pin)
└── fetch.go                # HELIX_BENCH_NETWORK-gated, io.LimitReader cap, HELIX_CACHE_DIR

bench/LICENSES.md           # +2 rows: multi-swe-bench (CC0), terminal-bench (Apache-2.0)
```

### Pattern 1: Config-file producer (Multi-SWE-bench's one divergence from the template)
**What:** Unlike SWE-bench (all flags), Multi-SWE-bench is driven by a single `--config <config.json>`. So the adapter's argv is trivial (`["-m","multi_swe_bench.harness.run_evaluation","--config",<path>]`) but it must FIRST *produce* a validated config.json. This is a pure deterministic transform (sorted keys for byte-stability), written via `writeDurable` for atomicity.
**When to use:** The Multi-SWE adapter only.
**Example (the config struct — verbatim field names from upstream):**
```go
// Source: github.com/multi-swe-bench/multi-swe-bench README config.json schema
// [CITED: github.com/multi-swe-bench/multi-swe-bench]
type Config struct {
    Mode                 string   `json:"mode"`                    // "evaluation" for our use
    Workdir              string   `json:"workdir"`
    PatchFiles           []string `json:"patch_files"`             // agent patches, JSONL
    DatasetFiles         []string `json:"dataset_files"`           // per-language dataset JSONL
    ForceBuild           bool     `json:"force_build"`
    OutputDir            string   `json:"output_dir"`              // final_report.json lands here
    Specifics            []string `json:"specifics"`               // [] = all
    Skips                []string `json:"skips"`
    RepoDir              string   `json:"repo_dir"`
    NeedClone            bool     `json:"need_clone"`
    GlobalEnv            []string `json:"global_env"`              // ["KEY=VALUE"]
    ClearEnv             bool     `json:"clear_env"`
    StopOnError          bool     `json:"stop_on_error"`
    MaxWorkers           int      `json:"max_workers"`
    MaxWorkersBuildImage int      `json:"max_workers_build_image"`
    MaxWorkersRunInstance int     `json:"max_workers_run_instance"`
    LogDir               string   `json:"log_dir"`
    LogLevel             string   `json:"log_level"`               // "INFO"
}
```
All path fields (`workdir`,`output_dir`,`repo_dir`,`log_dir`,`patch_files[]`,`dataset_files[]`) MUST pass the template's `isValidPredictionsPath` clean-absolute discipline BEFORE the JSON is marshalled — same fail-closed posture as the SWE-bench argv validator (a `..`/`:`/relative path is refused before reaching the harness).

### Pattern 2: Per-language slicing via the cell language axis (ZERO new aggregator code)
**What:** Multi-SWE-bench's instances are partitioned by upstream into language directories (`go/`,`java/`,`ts/`,`js/`,`rust/`,`c/`,`cpp/`); there is **no per-instance `language` JSON field** — the language is the dataset-file's directory. Map it to `Cell.Language` (the matrix language axis) when expanding the cell matrix. `cell.go:774` already stamps `Language: cfg.Language` into the result doc's `language` open key, and `reduceLanguageRows` already slices on it.
**When to use:** Multi-SWE adapter cell expansion.
**Example:**
```go
// Emit one cell per (language, instance); aggregator.reduceLanguageRows does the rest.
// language ∈ {go, java, ts, js, rust, c, cpp}  → flows verbatim into result.v2 `language` key
// → bench/aggregator ByLanguage[] per-language pass-rate (SC#1, no new code).
```

### Pattern 3: Long-wall checkpoint/resume state machine (NEW — net-new, pure Go)
**What:** A per-cell checkpoint file recording cell completion, written atomically after a cell finishes, read on resume to skip already-done cells. A >24h Terminal-Bench task that completes, then the harness restarts, must NOT re-run. Idempotent re-entry.
**When to use:** The long-wall scheduler wrapping `RunMatrix`.
**Recommended design (cleanest pure-Go, hermetically testable without a 24h run):**
```go
// Source: pattern built on bench/runtime/cell.go writeDurable (atomic temp+rename)
// Checkpoint state per cell: a small JSON sidecar keyed by a stable cellKey.
type CellState struct {
    CellKey   string `json:"cell_key"`   // sha-free stable: benchmark/lang/mode/task/run_index
    Status    string `json:"status"`     // "running" | "done" | "failed"
    ResultRef string `json:"result_ref"` // path to the cell's result.v2.json when done
    UpdatedAt string `json:"updated_at"` // RFC3339 (injected clock for hermetic tests)
}

// writeCheckpoint: marshal → writeDurable(<ckptDir>/<cellKey>.json, b)  [atomic, crash-safe]
// readCheckpoint:  os.ReadFile + json.Unmarshal; ENOENT → (zero, notFound) → run the cell
// Scheduler.Run: for each cell → if readCheckpoint==done → SKIP; else run → writeCheckpoint(done)
```
Three hermetic invariants the tests must prove (no Docker, no 24h):
1. **write→read round-trips:** a `done` checkpoint read back yields `done`.
2. **resume skips completed cells:** seed a `done` checkpoint, run the scheduler with a counting cell-runner, assert that cell's runner is invoked **0 times** (proves resume).
3. **idempotent re-entry / crash safety:** a partial (`running`, never `done`) checkpoint re-runs the cell; the atomic temp+rename guarantees a reader never sees a torn checkpoint (inherited from `writeDurable`).
Inject `now func() time.Time` so `UpdatedAt` is deterministic (golden-stable), mirroring the Phase 75 `DeprecationGate` injected-clock and Phase 84 `ArchGate` parameter-as-seam discipline.

### Anti-Patterns to Avoid
- **Re-implementing container isolation (SC#2):** tb owns its own DockerComposeManager (fresh container per task). The adapter MUST shell to `tb run` and assert isolation via tb's own per-task containers — do NOT route through `bench/container.Engine.Run` (the harness/tb own Docker, exactly as Phase 87's SWE-bench harness does; see Phase 87 Open Q4 / harness.go header comment).
- **Counting tests_status rows as success:** the authoritative `task_success` gate is the upstream `resolved` bool (Multi-SWE `final_report.json` resolved set) / `is_resolved` (tb per-trial) — NEVER a count of test rows (Phase 87 Pitfall 1, ingest.go:24).
- **`int` for exit code:** use `*int` so a clean `0` round-trips (Phase 87 Decision, result.go:124).
- **Forking the upstream harness:** per-language ablation slicing is built into the REPORTER, never the harness (CONTEXT decision).
- **A live run as sole proof:** the live Mini-set + tb smoke MUST skip cleanly; hermetic fixtures are the sole proof (CONTEXT environment-reality clause; Phase 87 precedent).
- **`go get` / go.mod edit:** every prior bench adapter added zero deps; this phase must too.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fixed-argv subprocess + env allowlist + procGroup | a fresh `os/exec` wrapper | clone `swebench/harness.go` | Already closes flag-smuggling/path-escape/env-leak vectors (T-87-01/02/05) |
| Atomic crash-safe checkpoint write | bare `os.WriteFile` | `writeDurable` (cell.go:923) | temp+rename avoids torn files a reader could observe |
| Per-language pass-rate slice | a new aggregator pass | `reduceLanguageRows` (existing) | SC#1 slicing already done; just stamp `Cell.Language` |
| Untrusted-JSON size cap + strict parse | naive `json.Unmarshal` | clone `report.go` (`maxReportBytes`+`checkReportSize`) | bounds a hostile/runaway upstream report (T-87-02-01) |
| Pinned dataset fetch (SSRF-safe, network-gated) | ad-hoc `http.Get` | clone `swebench-utboost/{pin,fetch}.go` | pinned-SHA + host pin + `io.LimitReader` + `HELIX_BENCH_NETWORK` gate |
| Bounded-parallel cell dispatch | new goroutine pool | `dispatch`/`RunMatrix` (matrix.go:194) | semaphore-bounded, per-cell outcome capture, cancellation-aware |

**Key insight:** Phase 88 is ~80% mechanical cloning. The genuinely new code is the config.json producer (one file), the two ingestion mappings, and the checkpoint state machine. Resist building anything the swebench template or `bench/runtime` already provides.

## Runtime State Inventory

> Phase 88 is greenfield (new adapter packages + new fixtures); it renames/migrates nothing. The categories below are answered explicitly for completeness.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — new packages write only to `HELIX_CACHE_DIR` caches + `bench/reports/` (gitignored) and committed `testdata/` fixtures. No existing keys/collections renamed. | None |
| Live service config | None — no external service config embeds a Phase-88 string. tb/multi_swe_bench own their own Docker; we don't register anything. | None |
| OS-registered state | None — no Task Scheduler / launchd / systemd / pm2 registration. | None |
| Secrets/env vars | None new as keys. The adapters READ `HELIX_CACHE_DIR`, `HELIX_BENCH_NETWORK`, `PATH`/`HOME`/`DOCKER_*` (existing allowlist). No secret key renamed. | None |
| Build artifacts | None — `go build ./cmd/helix-bench` only; no egg-info/compiled artifact carries an old name. | None |

**Canonical question (after every file is updated, what runtime state still has an old string?):** Nothing — this is purely additive new code. **None — verified by the greenfield nature of the two new adapter packages.**

## Common Pitfalls

### Pitfall 1: Multi-SWE-bench has no per-instance `language` field
**What goes wrong:** Trying to read `language` from the instance JSON yields nothing; per-language slicing silently buckets everything under `""`.
**Why it happens:** Upstream partitions by language *directory* (`go/`,`java/`,...), not a JSON field. The per-instance record carries `org`/`repo`/`number`/`instance_id`/`resolved_issues`/`fix_patch`/`test_patch`/`fixed_tests`/`p2p_tests`/`f2p_tests`/`run_result` — but **no `language`**.
**How to avoid:** Derive language from the dataset-file directory at cell-expansion time and stamp it into `Cell.Language`; the result builder copies it to the `language` key (`cell.go:774`). [VERIFIED: HF ByteDance-Seed/Multi-SWE-bench card + multi-swe-bench README dir layout]
**Warning signs:** `ByLanguage` has a single `Language==""` row instead of 7 rows.

### Pitfall 2: `tb run` vs `harbor run` for Terminal-Bench 2.0
**What goes wrong:** Wiring `tb run` and then finding the 2.0 leaderboard / 89-task dataset is driven through `harbor run`.
**Why it happens:** Terminal-Bench was rewritten as **Harbor**; `harbor run --dataset terminal-bench@2.0` is the 2.0-native path, while `tb run --dataset-name terminal-bench-core --dataset-version <v>` is the legacy 1.x CLI. CONTEXT locks `tb run`.
**How to avoid:** Keep the binary name and dataset-selection flags behind a single `runnerKind`/constant seam so swapping `tb`→`harbor` (or selecting `--dataset-name`/`--dataset-version` vs `--dataset`) is a one-line change. The JSON ingestion is the load-bearing part and is identical (`results.json` with `accuracy`/`is_resolved`). See Open Question O-1. [VERIFIED: laude-institute/terminal-bench README + harbor-framework/terminal-bench-2 + tbench.ai docs]
**Warning signs:** `Detect()` finds neither `tb` nor `harbor` (expected here — live test skips); a smoke run errors on an unknown `--dataset` flag shape.

### Pitfall 3: tb's `tb.lock` is NOT our checkpoint
**What goes wrong:** Assuming tb's resume (`tb.lock` "configuration snapshot for resuming") satisfies SC#3.
**Why it happens:** `tb.lock` resumes a *single* tb invocation's incomplete tasks; SC#3 wants OUR cell-matrix to resume across harness restarts (a different scope — the long-wall scheduler, possibly across multiple tb/multi_swe_bench invocations).
**How to avoid:** Build the net-new checkpoint state machine (Pattern 3). It is complementary to `tb.lock`, not a replacement. [VERIFIED: deepwiki terminal-bench output layout describes `tb.lock`]
**Warning signs:** A restarted long run re-executes already-`done` cells.

### Pitfall 4: Multi-SWE-bench config is file-driven, not flag-driven
**What goes wrong:** Cloning the SWE-bench flag-argv builder 1:1 and trying to pass `--dataset_name`/`--predictions_path`/`--instance_ids` flags — Multi-SWE-bench rejects them; it takes only `--config <json>`.
**Why it happens:** The two harnesses diverge here: SWE-bench is flags, Multi-SWE-bench is a config file.
**How to avoid:** Implement `config.go` (Pattern 1) producing the validated JSON; the argv is just `["-m","multi_swe_bench.harness.run_evaluation","--config",<path>]`. Validate every path field with `isValidPredictionsPath` BEFORE marshalling. [VERIFIED: multi-swe-bench README]
**Warning signs:** Harness exits with an unrecognized-argument error.

### Pitfall 5: `final_report.json` key names (resolved set is authoritative)
**What goes wrong:** Reading the wrong key for the success gate.
**Why it happens:** Multi-SWE `final_report.json` mirrors SWE-bench's `make_run_report` shape (`total_instances`/`resolved_instances`/`unresolved_instances`/`resolved_ids`/`unresolved_ids`), and per-instance the authoritative bool comes from the resolved set membership — NOT `run_result` test counts.
**How to avoid:** Gate `task_success` on resolved-set membership / per-instance `resolved`, exactly as the SWE-bench `ingest.go` gates on `eval.Resolved`. Commit a `final_report.json` fixture and a per-instance fixture and prove the mapping hermetically. [CITED: swebench reporting.py make_run_report; multi-swe-bench README final_report.json]
**Warning signs:** A resolved instance ingested as fail because the code counted `run_result` rows.

## Code Examples

### Multi-SWE config.json producer (deterministic, atomic)
```go
// Source: built on bench/runtime/cell.go writeDurable + Phase 87 isValidPredictionsPath
func WriteConfig(path string, c Config) error {
    // validate every path field (clean, absolute, no ".."/":") BEFORE marshal — fail-closed
    for _, p := range append([]string{c.Workdir, c.OutputDir, c.RepoDir, c.LogDir},
        append(c.PatchFiles, c.DatasetFiles...)...) {
        if !isValidPredictionsPath(p) {
            return fmt.Errorf("%w: config path %q must be clean+absolute", errBadConfig, p)
        }
    }
    b, err := json.MarshalIndent(c, "", "  ") // stable key order via struct decl order → golden-stable
    if err != nil { return err }
    return writeDurable(path, b) // atomic temp+rename (crash-safe)
}
```

### Terminal-Bench results ingestion
```go
// Source: deepwiki terminal-bench results schema [CITED]
type TBAggregate struct {
    Accuracy    float64 `json:"accuracy"`
    NResolved   int     `json:"n_resolved"`
    NUnresolved int     `json:"n_unresolved"`
}
type TBTrial struct {
    TaskID     string `json:"task_id"`     // per-trial results.json
    IsResolved bool   `json:"is_resolved"` // authoritative task_success gate
}
// Ingest maps TBTrial.IsResolved → ResultInput.Metrics.TaskSuccess (&local, distinct ptr),
// ContainerID carried verbatim, exactly like swebench/ingest.go.
```

### Resume-aware scheduler (hermetic)
```go
// Source: pattern on writeDurable + matrix dispatch
func (s *Scheduler) Run(ctx context.Context, cells []Cell, run func(Cell) CellOutcome) Summary {
    for _, c := range cells {
        if st, ok := s.read(cellKey(c)); ok && st.Status == "done" {
            continue // RESUME: skip completed cell, idempotent re-entry
        }
        oc := run(c)
        s.write(cellKey(c), CellState{Status: statusFor(oc), UpdatedAt: s.now().Format(time.RFC3339)})
    }
    // ... summary
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `tb run` (Terminal-Bench 1.x CLI) | `harbor run` (Harbor framework) for TB 2.0 | TB 2.0 release | CONTEXT locks `tb run`; keep a one-constant seam so harbor is a trivial swap (O-1) |
| SWE-bench flag argv | Multi-SWE-bench `--config <json>` file | Multi-SWE-bench design | The one structural divergence from the template — needs a config producer |

**Deprecated/outdated:**
- `tb run` for 2.0 leaderboard parity: superseded by `harbor run` upstream, but retained per CONTEXT (the JSON ingestion is identical, so the risk is low).

## Validation Architecture

> `nyquist_validation: true` in `.planning/config.json` — section included.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `testify` (already in tree) |
| Config file | none (Go convention) |
| Quick run command | `go test ./bench/evaluators/multiswebench/... ./bench/evaluators/terminalbench/... ./bench/longwall/...` |
| Full suite command | `go test ./...` then `go vet ./...` (CLAUDE.md mandate) |

### Phase Requirements → Test Map
| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| ADAPTER-MULTI-01 | config.json produced + golden-stable + path-validation refusals | unit (hermetic) | `go test ./bench/evaluators/multiswebench/ -run TestWriteConfig` | ❌ Wave 0 |
| ADAPTER-MULTI-01 | `final_report.json` + per-instance → ResultInput (resolved gate) | unit (hermetic fixture) | `go test ./bench/evaluators/multiswebench/ -run TestIngest` | ❌ Wave 0 |
| ADAPTER-MULTI-01 | per-language slicing (7 langs → 7 ByLanguage rows) | unit (hermetic) | `go test ./bench/aggregator/ -run TestAggregateByLanguage` (extend existing) | ✅ (extend) |
| ADAPTER-MULTI-01 | argv = `-m multi_swe_bench.harness.run_evaluation --config <path>` | unit (argv golden) | `go test ./bench/evaluators/multiswebench/ -run TestRunArgs` | ❌ Wave 0 |
| ADAPTER-MULTI-01 | live Mini-set run | integration (gated) | `go test ./bench/evaluators/multiswebench/ -run TestLive` (SKIP: no Docker/pkg) | ❌ Wave 0 (skip-gated) |
| ADAPTER-TERM-01 | `tb run` argv builder | unit (argv golden) | `go test ./bench/evaluators/terminalbench/ -run TestRunArgs` | ❌ Wave 0 |
| ADAPTER-TERM-01 | `results.json` (aggregate + per-trial) → ResultInput (is_resolved gate) | unit (hermetic fixture) | `go test ./bench/evaluators/terminalbench/ -run TestIngest` | ❌ Wave 0 |
| ADAPTER-TERM-01 | ≥5-task smoke | integration (gated) | `go test ./bench/evaluators/terminalbench/ -run TestSmoke` (SKIP: no tb) | ❌ Wave 0 (skip-gated) |
| SC#3 | checkpoint write→read round-trip | unit (hermetic) | `go test ./bench/longwall/ -run TestCheckpointRoundTrip` | ❌ Wave 0 |
| SC#3 | resume skips completed cells (counting runner ⇒ 0 invocations) | unit (hermetic) | `go test ./bench/longwall/ -run TestResumeSkipsDone` | ❌ Wave 0 |
| SC#3 | idempotent re-entry / partial-checkpoint re-runs | unit (hermetic) | `go test ./bench/longwall/ -run TestIdempotentReentry` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./bench/evaluators/... ./bench/longwall/...` (hermetic, <10s)
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** full suite green; live Mini-set + tb smoke recorded as Docker/tb-gated (skip cleanly), NOT graded as the sole proof.

### Wave 0 Gaps
- [ ] `bench/evaluators/multiswebench/{config,harness,ingest,report}_test.go` + `testdata/` (config golden, final_report.json, per-instance reports per language) — covers ADAPTER-MULTI-01
- [ ] `bench/evaluators/terminalbench/{harness,ingest,report}_test.go` + `testdata/` (results.json aggregate + per-trial) — covers ADAPTER-TERM-01
- [ ] `bench/longwall/{checkpoint,scheduler}_test.go` — covers SC#3
- [ ] extend `bench/aggregator/aggregate_test.go` with a 7-language Multi-SWE fixture (reuses existing `reduceLanguageRows`)
- [ ] no framework install needed (Go stdlib + testify present)

## Security Domain

> `security_enforcement` is not set to `false` in config — included. Helix is Go-native; no `java-security` capability applies (CLAUDE.md). The relevant threats are subprocess/argv/path-injection and untrusted-JSON parsing, all of which the cloned template already mitigates.

### Applicable controls (reused verbatim from the Phase 87 template)
| Control area | Standard control (in-tree) |
|--------------|---------------------------|
| Argv / flag injection | total regexp-free validators + leading-`-` refusal + fixed argv slice (`harness.go`); for Multi-SWE, validate config path fields before marshal |
| Path traversal | `isValidPredictionsPath` (clean+absolute, no `..`/`:`) on every path crossing to the harness |
| Env leakage | strict `allowlistEnv` (PATH/HOME/HELIX_CACHE_DIR + DOCKER_* when set) — never the inherited env |
| Untrusted report DoS | `maxReportBytes` (64 MiB) size-cap + `checkReportSize` before `json.Unmarshal`; live stream caller wraps `io.LimitReader` |
| SSRF (dataset fetch) | pinned Host + pinned-SHA + caller-supplied-URL refusal (clone `swebench-utboost/fetch.go`) |
| Process-group runaway | `procGroupAttr()` so a ctx cancel group-kills the harness + its Docker descendants |

### Known threat patterns for this stack
| Pattern | STRIDE | Mitigation |
|---------|--------|------------|
| Crafted config path (`../`, `:`, flag-shaped) | Tampering | validate before marshal; fail-closed (errBadConfig) |
| Hostile/runaway upstream report | Denial of Service | size cap + strict parse |
| Mutable-ref dataset swap | Tampering | pinned-SHA refusal (isHexSHA1) |
| Env-var exfiltration via subprocess | Information Disclosure | strict env allowlist |

## Open Questions (RESOLVED)

1. **O-1: `tb run` vs `harbor run` for Terminal-Bench 2.0?**
   - **RESOLVED:** Terminal-Bench was rewritten as **Harbor**; the 2.0 leaderboard / 89-task dataset is driven through `harbor run --dataset terminal-bench@2.0`, while `tb run --dataset-name terminal-bench-core --dataset-version <v>` is the legacy 1.x CLI. CONTEXT.md + ADAPTER-TERM-01 lock `tb run`.
   - **Recommendation:** Honor CONTEXT (`tb run`) but keep the binary name and dataset-selection flags behind a single `runnerKind` constant/seam so a future swap to `harbor run` is one line. The JSON ingestion (`results.json`: `accuracy`/`n_resolved`/`n_unresolved`, per-trial `is_resolved`) is identical and is the load-bearing part. Add a `checkpoint:human-verify` to confirm the live binary name on a Docker host (deferred, Phase 87 precedent). [VERIFIED: laude-institute/terminal-bench README + harbor-framework/terminal-bench-2 + evalscope TB v2 docs]

2. **O-2: Is the Multi-SWE-bench Mini set redistributable (Phase 75 INFRA-02)?**
   - **RESOLVED:** The Multi-SWE-bench dataset is licensed **CC0** ("subject to any intellectual property rights owned by ByteDance"), which **permits redistribution** (public-domain dedication); users must still respect each source project's own license. So the Mini set **is redistributable** for our purposes (we fetch+cache, we don't vendor the corpus into git anyway — `HELIX_BENCH_NETWORK`-gated fetch).
   - **Recommendation:** Ship the Mini set (`bytedance-research/Multi-SWE-bench_mini`, ~400 instances, 8 langs per upstream README). Append a `bench/LICENSES.md` row: `multi-swe-bench | huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench | CC0 | redistributable (CC0; respect source-project licenses) | <PinnedSHA> | 2026-06-21`. The full 1,632-instance set remains the documented reach goal (defer to v1.13 only if a *source-project* license issue surfaces — the CC0 dataset license itself is clear). [VERIFIED: HF ByteDance-Seed/Multi-SWE-bench license card]

3. **O-3: Does an existing checkpoint/resume mechanism exist in-tree to reuse?**
   - **RESOLVED: NO — it must be built.** Grep across `bench/` and `internal/` finds only `internal/semantic/store/overlay.go` `Checkpoint(ctx)` (a DuckDB WAL flush — unrelated) and tb's upstream `tb.lock` (single-invocation resume, wrong scope). There is **no cell-matrix checkpoint/resume state machine in the bench runtime** (Phases 77/80/82 added matrix dispatch, durable writes, and aggregation — but no resume).
   - **Recommendation:** Build it as a new pure package (`bench/longwall/`) on top of the existing `writeDurable` atomic primitive (cell.go:923). Pattern 3 above is the cleanest pure-Go approach: per-cell JSON checkpoint, atomic write, ENOENT→run, `done`→skip, injected clock for golden-stability. Fully hermetic (write→read→resume, mid-run-restart simulation, idempotency) — no 24h run, no Docker. [VERIFIED: codebase grep]

4. **O-4: Does the result.v2 schema need changes for these adapters?**
   - **RESOLVED: NO.** `container_id`, `exit_code` (`*int`), and `language` are already additive-minor open keys on `ResultInput` (result.go:104/113/90), added in Phases 85/87. `additionalProperties` stays OPEN; `schema_version` stays `v2`. The two new adapters reuse these verbatim. No `v3` bump. [VERIFIED: bench/runtime/result.go]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | building adapters + tests | ✓ | (host) | — |
| Docker | live Multi-SWE + tb runs | ✗ | — | hermetic fixtures (sole proof); live tests SKIP |
| `multi_swe_bench` Python pkg | live Mini-set run | ✗ | — | `Detect()` → `t.Skip` |
| `tb` / `harbor` CLI | live tb smoke | ✗ | — | `Detect()` → `t.Skip` |
| network (HF) | dataset fetch | ✗ (gated) | — | committed fixtures; `HELIX_BENCH_NETWORK`-gated live fetch |

**Missing dependencies with no fallback:** none (all blocking deps have a hermetic-fixture fallback; live runs skip cleanly).
**Missing dependencies with fallback:** Docker / `multi_swe_bench` / `tb` / network — all covered by committed fixtures + skip gates (CONTEXT environment-reality clause).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Multi-SWE config field names (`mode`,`patch_files`,`dataset_files`,`output_dir`,`repo_dir`,`need_clone`,`max_workers_*`,`log_dir`,`global_env`,`clear_env`,`stop_on_error`,`force_build`,`specifics`,`skips`,`log_level`) are exact | Code Examples / Pattern 1 | Config rejected by harness; fix at the human-verify checkpoint against the live README/example config before the live run. Hermetic golden still valid as a *contract* the planner pins. |
| A2 | `final_report.json` mirrors SWE-bench `make_run_report` (`resolved_ids`/`unresolved_ids`/`total_instances`) | Pitfall 5 | Ingestion maps wrong key; commit fixture from a real run on a Docker host to confirm. SWE-bench lineage makes this LOW risk. |
| A3 | tb `results.json` carries `accuracy`/`n_resolved`/`n_unresolved` (aggregate) + per-trial `is_resolved` | Code Examples | tb ingestion field names off; confirm against a real `runs/<ts>/` dir on a tb host. DeepWiki-sourced (MEDIUM). |
| A4 | Mini set HF id = `bytedance-research/Multi-SWE-bench_mini` (~400 instances, 8 langs) | O-2 | Wrong dataset id; the README references the Mini set but the canonical HF card shows only the full set — confirm the exact repo id at the dataset-fetch human-verify checkpoint. |
| A5 | Multi-SWE Mini license = CC0 (redistributable) | O-2 | If a source-project license restricts a specific repo's instances, exclude it; the CC0 dataset license itself is VERIFIED from the HF card. |
| A6 | Multi-SWE has NO per-instance `language` field (language = dir) | Pitfall 1 | If a `language` field exists, prefer it over dir-derivation (strictly easier); dir-derivation is the safe default. |
| A7 | TB 2.0 may require `harbor run` not `tb run` | O-1 | Binary-name seam makes this a one-line change; JSON ingestion unaffected. |

## Sources

### Primary (HIGH confidence)
- In-tree `bench/evaluators/swebench/{harness,ingest,report,predictions,differential}.go` + `testdata/` — the cloned template (read directly this session)
- In-tree `bench/runtime/{result.go,cell.go,matrix.go}`, `bench/aggregator/{aggregate.go,report.go}`, `bench/datasets/swebench-utboost/pin.go`, `bench/LICENSES.md` — reuse surface (read directly)
- Codebase grep confirming NO existing checkpoint/resume state machine in `bench/`

### Secondary (MEDIUM confidence — official repos/docs, not runnable here)
- github.com/multi-swe-bench/multi-swe-bench (README config.json schema, Mini set, languages)
- huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench (per-instance fields, CC0 license)
- github.com/laude-institute/terminal-bench (README `tb run` flags) + harbor-framework/terminal-bench-2
- deepwiki.com/laude-institute/terminal-bench (runs/ layout, results.json schema, DockerComposeManager isolation, `tb.lock`)
- evalscope TB v2 docs (89 tasks, harbor relationship); huggingface.co/datasets/harborframework/terminal-bench-2.0 (Apache-2.0)
- swebench.com reporting.py `make_run_report` (final-report shape lineage)

### Tertiary (LOW confidence — verify at human-verify checkpoint)
- Exact Mini-set HF repo id and the precise config/JSON field spellings (A1–A4) — confirm on a Docker+tb+network host before the live run (deferred, Phase 87 precedent).

## Metadata

**Confidence breakdown:**
- Standard stack (in-tree reuse): HIGH — read every template/reuse file directly
- Architecture (clone + config producer + checkpoint SM): HIGH — derived from the proven Phase 87 pattern
- Upstream config/JSON shapes: MEDIUM — verified via official repos/docs, not runnable here; pinned as contracts to confirm at human-verify (Phase 87 deferral precedent)
- Pitfalls: HIGH — cross-checked against Phase 87 decisions + upstream docs
- License (Multi-SWE CC0, TB Apache-2.0): HIGH — read from HF dataset cards

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable in-tree reuse) / 2026-06-28 for the upstream CLI/JSON shapes (fast-moving: tb→harbor migration is active)

## RESEARCH COMPLETE
