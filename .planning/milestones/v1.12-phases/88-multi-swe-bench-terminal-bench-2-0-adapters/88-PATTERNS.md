# Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 16 new / 2 modified
**Analogs found:** 16 / 16 (every new file clones an in-tree template; 100% coverage)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/evaluators/multiswebench/config.go` | producer (NEW divergence) | transform (struct→JSON file) | `swebench/predictions.go` + `swebench/harness.go` (validators) | role-match (composite) |
| `bench/evaluators/multiswebench/harness.go` | subprocess wrapper | request-response (exec) | `bench/evaluators/swebench/harness.go` | exact |
| `bench/evaluators/multiswebench/ingest.go` | transform | transform (JSON→ResultInput) | `bench/evaluators/swebench/ingest.go` | exact |
| `bench/evaluators/multiswebench/report.go` | parser | transform (strict JSON parse) | `bench/evaluators/swebench/report.go` | exact |
| `bench/evaluators/multiswebench/procgroup_{unix,windows}.go` | platform util | — | `bench/evaluators/swebench/procgroup_*.go` | exact (verbatim copy) |
| `bench/evaluators/terminalbench/harness.go` | subprocess wrapper | request-response (exec) | `bench/evaluators/swebench/harness.go` | exact |
| `bench/evaluators/terminalbench/ingest.go` | transform | transform (JSON→ResultInput) | `bench/evaluators/swebench/ingest.go` | exact |
| `bench/evaluators/terminalbench/report.go` | parser | transform (strict JSON parse) | `bench/evaluators/swebench/report.go` | exact |
| `bench/evaluators/terminalbench/procgroup_{unix,windows}.go` | platform util | — | `bench/evaluators/swebench/procgroup_*.go` | exact (verbatim copy) |
| `bench/longwall/checkpoint.go` | state machine (NET-NEW) | file-I/O (atomic write/read) | `cell.go` `writeDurable` + `swebench-utboost/fetch.go` `writeCacheAtomic` | role-match (primitive) |
| `bench/longwall/scheduler.go` | scheduler (NET-NEW) | event-driven (resume loop) | `bench/runtime/matrix.go` `dispatch`/`RunMatrix` | role-match |
| `bench/datasets/multi-swe-bench-mini/pin.go` | dataset pin | config | `bench/datasets/swebench-utboost/pin.go` | exact |
| `bench/datasets/multi-swe-bench-mini/fetch.go` | dataset fetch | file-I/O (network+cache) | `bench/datasets/swebench-utboost/fetch.go` | exact |
| `bench/aggregator/aggregate_test.go` (MODIFY) | test | — | existing `TestAggregateByLanguagePassRate` | extend |
| `bench/LICENSES.md` (MODIFY) | doc | — | existing seeded table | append 2 rows |

## Pattern Assignments

### `bench/evaluators/multiswebench/harness.go` (subprocess wrapper, request-response)

**Analog:** `bench/evaluators/swebench/harness.go` — clone the file structure verbatim; swap only the argv (now `["-m","multi_swe_bench.harness.run_evaluation","--config",<path>]`) and drop the SWE-bench-specific flag validators (dataset/runID/instanceID/cacheLevel) since Multi-SWE is config-driven.

**Package-doc + imports** (lines 1-22): copy verbatim; rename package and the harness module string.

**`isValidPredictionsPath`** (`harness.go:155-177`) — copy VERBATIM. This is the load-bearing path validator the config producer (`config.go`) calls on EVERY path field before marshalling (Pattern 1 / Pitfall 4 / Security: path traversal). It rejects empty, `:`, non-absolute, and non-clean (`..`) paths against the SLASH form.

**`Detect()` skip-gate** (`harness.go:246-258`) — copy structure; probe `multi_swe_bench`'s interpreter the same way (PATH probe for `python3`/`python`), return `errHarnessUnavailable`. Live tests `t.Skip` on this sentinel.

**`Run` body** (`harness.go:267-297`) — copy VERBATIM (build argv → optional `runShim` test seam → `resolveWorkDir` → `allowlistEnv` → `exec.CommandContext` with `procGroupAttr()` + `cmd.Env = env` + `cmd.Dir = workDir`). Change the default WorkDir name from `swebench-runs`.

**`allowlistEnv` + `envAllowlist`** (`harness.go:337-360`) — copy VERBATIM. Strict allowlist `{PATH, HOME, HELIX_CACHE_DIR, DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH}`; fail-closed when PATH empty. Multi-SWE-bench shells `docker` exactly like SWE-bench, so the DOCKER_* keys are required.

**`resolveWorkDir` + `harnessCacheRoot`** (`harness.go:304-329`) — copy VERBATIM (HELIX_CACHE_DIR → UserCacheDir/helix → ~/.helix/cache precedence).

**Argv builder (the one change):** replace `RunArgs` (`harness.go:192-229`) with a trivial fixed argv — no instance-id loop, no flag validators. The validation moves INTO `config.go` (path fields validated before JSON marshal).

---

### `bench/evaluators/multiswebench/config.go` (producer, NEW — the one structural divergence)

**Analog (composite):** `swebench/predictions.go` (deterministic byte-stable producer discipline) + `swebench/harness.go:155-177` (`isValidPredictionsPath` reused on every path field) + `cell.go:923` `writeDurable` (atomic write — but it is UNEXPORTED in `bench/runtime`, so copy `swebench-utboost/fetch.go:232-256` `writeCacheAtomic` into this leaf package instead).

**Config struct:** verbatim field names from RESEARCH Pattern 1 (Config: `mode`,`workdir`,`patch_files`,`dataset_files`,`force_build`,`output_dir`,`specifics`,`skips`,`repo_dir`,`need_clone`,`global_env`,`clear_env`,`stop_on_error`,`max_workers`,`max_workers_build_image`,`max_workers_run_instance`,`log_dir`,`log_level`). Struct declaration order = JSON key order = golden-stable (same property `predictions.go:14-21` documents).

**`WriteConfig` pattern** (RESEARCH Code Examples, built on these analogs):
- Validate every path field (`Workdir`, `OutputDir`, `RepoDir`, `LogDir`, and each of `PatchFiles[]`/`DatasetFiles[]`) with the copied `isValidPredictionsPath` BEFORE marshal — fail-closed with an `errBadConfig` sentinel (mirror `errBadHarnessArg` at `harness.go:50-55`).
- `json.MarshalIndent(c, "", "  ")` — stable key order via struct decl order (mirrors `predictions.go` `SetEscapeHTML(false)` byte-stability discipline; consider `SetEscapeHTML(false)` if path fields can hold `<>&`).
- Write via a local `writeCacheAtomic`-style temp+rename (copy `swebench-utboost/fetch.go:232-256`), NOT a bare `os.WriteFile` (Don't-Hand-Roll: atomic crash-safe write).

---

### `bench/evaluators/multiswebench/ingest.go` (transform, JSON→ResultInput)

**Analog:** `bench/evaluators/swebench/ingest.go` (read in full).

**Core mapping** (`ingest.go:36-56`) — copy the shape exactly:
- `taskSuccess := <resolved-set membership>` as a DISTINCT local → `&taskSuccess` (never aliased with `VerifiedCorrectness`, which stays nil). Gate on the `final_report.json` `resolved_ids` set membership / per-instance `resolved` bool — NEVER a count of `tests_status` rows (Pitfall 5, anti-pattern at `ingest.go:24`).
- `ContainerID` carried verbatim; `ExitCode *int` so a clean `0` round-trips (`ingest.go:44-49`, Pitfall: never `int`).
- **Divergence:** stamp `Language` from the dataset-file directory derivation (Pitfall 1 — Multi-SWE has NO per-instance `language` field). The language flows from `Cell.Language` (see Shared Pattern: Per-language slicing), NOT from `ingest.go` reading a JSON field — but `Ingest` should accept the language argument and set `runtime.ResultInput{Language: lang, ...}`.
- A missing instance → INGEST ERROR + zero-value `ResultInput` (TaskSuccess nil), never a fabricated success (`ingest.go:33-40`).
- `benchmarkName` const (`ingest.go:13`) → `"multi-swe-bench"`.

---

### `bench/evaluators/multiswebench/report.go` (parser, strict JSON parse)

**Analog:** `bench/evaluators/swebench/report.go` (read in full).

- Copy `maxReportBytes` (64 MiB) + `checkReportSize` (`report.go:19`, `report.go:129-137`) VERBATIM — untrusted-report DoS cap before `json.Unmarshal` (T-87-02-01). A live `io.Reader` caller wraps `io.LimitReader(r, maxReportBytes+1)`.
- Define `FinalReport` struct mirroring `RunReport` (`report.go:64-86`): `total_instances`/`resolved_instances`/`unresolved_instances`/`resolved_ids`/`unresolved_ids` (Pitfall 5 — SWE-bench `make_run_report` lineage). Plus a per-instance struct for the per-instance reports.
- `ParseFinalReport`/`ParseInstanceReport` mirror `ParseRunReport`/`ParseInstanceReport` (`report.go:94-123`): size-cap → strict unmarshal → tolerate unknown keys → empty/null body is an error.

---

### `bench/evaluators/terminalbench/harness.go` (subprocess wrapper, request-response)

**Analog:** `bench/evaluators/swebench/harness.go`.

- Same copy as multiswebench/harness.go (Detect/Run/allowlistEnv/resolveWorkDir/procGroupAttr verbatim).
- **Divergence — `runnerKind` binary-name seam** (RESEARCH O-1 / Pitfall 2): keep the binary name (`tb` vs `harbor`) and dataset-selection flags behind a single constant/seam so swapping to `harbor run` is one line. Argv = `tb run --agent ... --dataset-name terminal-bench-core --dataset-version <v> --task-id <id> --output-path <dir>`.
- `Detect()` probes for `tb` (and optionally `harbor`) on PATH; `t.Skip` when neither present (expected here).
- Reuse `isValidPredictionsPath` for `--output-path` and any path-shaped flag; reuse the leading-`-` refusal idiom for task-ids/dataset-version (mirror `isValidRunID` `harness.go:97-113`).

---

### `bench/evaluators/terminalbench/ingest.go` + `report.go`

**Analog:** `swebench/ingest.go` + `swebench/report.go`.

- `report.go`: `TBAggregate{accuracy,n_resolved,n_unresolved}` + `TBTrial{task_id,is_resolved}` (RESEARCH Code Examples). Reuse `maxReportBytes` + `checkReportSize` verbatim. Parsers for `runs/<ts>/results.json` (aggregate) and per-trial `<task_id>/<trial>/results.json`.
- `ingest.go`: map `TBTrial.IsResolved` → `&taskSuccess` (distinct ptr), `ContainerID` verbatim, `benchmarkName = "terminal-bench"`. Authoritative gate is `is_resolved` — never a row count (Pitfall, anti-pattern `ingest.go:24`).

---

### `bench/longwall/checkpoint.go` (state machine, NET-NEW — confirmed nothing in-tree)

**Analog (primitive only):** `cell.go:923` `writeDurable` (atomic temp+rename — the substrate) + `swebench-utboost/fetch.go:232-256` `writeCacheAtomic` (the actual COPYABLE atomic-write code, since `writeDurable` is unexported). `bench/longwall/` is a new leaf package — copy the temp+rename primitive locally; do NOT import `bench/runtime`.

**`CellState` struct** (RESEARCH Pattern 3): `{CellKey, Status("running"|"done"|"failed"), ResultRef, UpdatedAt(RFC3339)}`.

**Atomic write primitive** — copy `writeCacheAtomic` (`fetch.go:232-256`): MkdirAll → `os.CreateTemp(dir, ".tmp-...")` → Write → Close → `os.Rename`, removing the temp on every error path. This is exactly `writeDurable`'s shape (`cell.go:923-949`) minus the `bench/runtime` coupling.

**`writeCheckpoint`/`readCheckpoint`:** marshal `CellState` → atomic write to `<ckptDir>/<cellKey>.json`; read = `os.ReadFile` + `json.Unmarshal`, ENOENT → `(zero, notFound)` → run the cell.

**Injected clock** — mirror `internal/semantic/compact/gate.go:63,73-75`: a `now func() time.Time` field, `nil → time.Now`, so `UpdatedAt` is golden-stable in hermetic tests (RESEARCH Pattern 3 references the DeprecationGate/ArchGate injected-clock discipline).

---

### `bench/longwall/scheduler.go` (scheduler, NET-NEW)

**Analog:** `bench/runtime/matrix.go` `dispatch`/`RunMatrix` (`matrix.go:180-194`) — the long-wall scheduler EXTENDS, does not replace, the bounded-parallel dispatch. The resume loop (RESEARCH Code Examples):

```
for each cell: if read(cellKey(c)).Status=="done" → continue (RESUME skip)
               oc := run(c); write(cellKey(c), CellState{Status: statusFor(oc), UpdatedAt: now()...})
```

`cellKey` = stable `benchmark/lang/mode/task/run_index` (mirror `cell.go` cellDurablePaths keying). Three hermetic invariants (RESEARCH Pattern 3): write→read round-trip; resume skips done (counting runner ⇒ 0 invocations); partial (`running`) re-runs + torn-read safety from the atomic primitive.

---

### `bench/datasets/multi-swe-bench-mini/{pin,fetch}.go` (dataset pin/fetch)

**Analog:** `bench/datasets/swebench-utboost/{pin.go,fetch.go}` — clone VERBATIM, swap constants.

- `pin.go`: copy `isHexSHA1` (`pin.go:81-95`), `isValidHTTPSHost` (`pin.go:101-106`), `expectedDigest`/`PinnedContentDigests` (`pin.go:65-74`) verbatim. Set `DatasetID = "ByteDance-Seed/Multi-SWE-bench"` (Mini set; A4 — confirm exact id `bytedance-research/Multi-SWE-bench_mini` at human-verify), `Host = "https://huggingface.co"`, `PinnedSHA` (defer live confirmation per Phase 87 precedent), `fetchSubdir = "multi-swe-bench-mini"`.
- `fetch.go`: copy `cacheDir` precedence (`fetch.go:40-49`), `validatePathSegment` (`fetch.go:56-64`), `cachePath`, `resolveURL` (`fetch.go:88-99`, SSRF-safe), `Fetch` (`fetch.go:108-174`: mutable-ref refusal before network, `io.LimitReader` cap, content-digest assert, atomic cache write), `readCacheCapped`, `writeCacheAtomic` verbatim. `maxDatasetBytes` 256 MiB.

---

### `bench/aggregator/aggregate_test.go` (MODIFY — extend, zero production code)

**Analog:** existing `TestAggregateByLanguagePassRate` (`aggregate_test.go:350`) + `langByName` helper (`aggregate_test.go:326`). Add a 7-language Multi-SWE fixture (go/java/ts/js/rust/c/cpp) asserting 7 `ByLanguage` rows. NO new aggregator code — `reduceLanguageRows` (`aggregate.go:255`) + `rowLanguage` (`aggregate.go:517`, reads `Doc["language"]`) already slice it.

---

### `bench/LICENSES.md` (MODIFY — append 2 rows)

**Analog:** existing seeded table (`LICENSES.md:14-17`). Append (RESEARCH O-2):
- `multi-swe-bench | huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench | CC0 | redistributable (CC0; respect source-project licenses) | <PinnedSHA> | 2026-06-21`
- `terminal-bench | huggingface.co/datasets/harborframework/terminal-bench-2.0 | Apache-2.0 | redistributable (Apache-2.0, attribution) | <PinnedSHA> | 2026-06-21`

## Shared Patterns

### Fixed-argv subprocess + strict env + procgroup
**Source:** `bench/evaluators/swebench/harness.go` (`Run` :267-297, `allowlistEnv` :337-360, `procGroupAttr` in `procgroup_*.go`)
**Apply to:** both `multiswebench/harness.go` and `terminalbench/harness.go`
Fixed argv slice (never a shell), strict `{PATH,HOME,HELIX_CACHE_DIR,DOCKER_*}` allowlist (never inherited env), `procGroupAttr()` so a ctx-cancel group-kills the harness + its Docker descendants, `runShim` test seam for hermetic argv assertion. The harness/tb own their OWN Docker — never route through `bench/container.Engine.Run`.

### Path-field validation (fail-closed before crossing the boundary)
**Source:** `bench/evaluators/swebench/harness.go:155-177` (`isValidPredictionsPath`)
**Apply to:** every path crossing to a harness — argv flags (tb `--output-path`) AND `config.go` JSON path fields, validated BEFORE marshal/exec. Clean + absolute, no `..`/`:`, leading-`-` refusal.

### Atomic crash-safe write
**Source:** `bench/runtime/cell.go:923-949` `writeDurable` (the canonical primitive, UNEXPORTED) — copy the COPYABLE twin `bench/datasets/swebench-utboost/fetch.go:232-256` `writeCacheAtomic`
**Apply to:** `config.go` (config.json), `longwall/checkpoint.go` (checkpoint sidecar). temp+rename so a reader never sees a torn file.

### Untrusted-JSON size cap + strict parse
**Source:** `bench/evaluators/swebench/report.go:19,129-137` (`maxReportBytes` + `checkReportSize`)
**Apply to:** every report parser (`multiswebench/report.go`, `terminalbench/report.go`). Size-cap before `json.Unmarshal`; tolerate unknown keys; empty/null body is an error.

### Per-language slicing via the cell language axis (ZERO new aggregator code)
**Source:** `bench/runtime/cell.go:774` (`Language: cfg.Language` stamps the result `language` open key) + `bench/runtime/matrix.go:32,163` (`Cell.Language` axis) + `bench/aggregator/aggregate.go:255,517` (`reduceLanguageRows`/`rowLanguage`)
**Apply to:** Multi-SWE cell expansion — emit one cell per (language, instance) with `Cell.Language ∈ {go,java,ts,js,rust,c,cpp}` derived from the dataset-file directory (Pitfall 1). The aggregator's existing `ByLanguage[]` slice (SC#1) needs no change.

### SSRF-safe pinned dataset fetch
**Source:** `bench/datasets/swebench-utboost/{pin.go:81-106,fetch.go:88-174}`
**Apply to:** `multi-swe-bench-mini/{pin,fetch}.go`. Pinned Host+DatasetID+SHA only (never caller URL), `isHexSHA1` mutable-ref refusal before any network, `io.LimitReader` body cap, `HELIX_BENCH_NETWORK`-gated live leg.

### Injected clock for golden-stable timestamps
**Source:** `internal/semantic/compact/gate.go:63,73-75` (`now func() time.Time`, nil→`time.Now`)
**Apply to:** `longwall/checkpoint.go` `CellState.UpdatedAt` — deterministic in hermetic tests.

## No Analog Found

None. Every new file clones an in-tree template; the only NET-NEW logic (config.json producer, two ingestion mappings, checkpoint/scheduler state machine) is composed from existing primitives (`writeCacheAtomic`, `isValidPredictionsPath`, `dispatch`, the injected-clock gate), so there is no file the planner must source solely from RESEARCH.md.

## Metadata

**Analog search scope:** `bench/evaluators/swebench/`, `bench/datasets/swebench-utboost/`, `bench/runtime/{cell,matrix,result}.go`, `bench/aggregator/aggregate.go`, `bench/LICENSES.md`, `internal/semantic/compact/gate.go`
**Files scanned:** ~14 source files read; SMTC/grep used to locate `writeDurable` (cell.go:923), `Cell.Language` stamp (cell.go:774), `reduceLanguageRows`/`rowLanguage` (aggregate.go:255/517), and the swebench-utboost fetcher
**Pattern extraction date:** 2026-06-21
