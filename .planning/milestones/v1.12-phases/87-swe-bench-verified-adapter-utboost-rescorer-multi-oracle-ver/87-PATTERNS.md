# Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness` - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 11 new + 2 modified = 13
**Analogs found:** 13 / 13 (every file clones a verified in-tree analog)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/evaluators/swebench/predictions.go` (NEW) | producer/transform | file-I/O (emit JSONL) | `bench/runtime/result.go` (sorted, deterministic emit) | role-match |
| `bench/evaluators/swebench/harness.go` (NEW) | subprocess seam | request-response (os/exec) | `bench/container/engine.go` (`pullArgs`/`allowlistEnv`/procgroup) | exact |
| `bench/evaluators/swebench/report.go` (NEW) | ingester/transform | transform (JSON→struct) | `bench/evaluators/test_runner/test_runner.go` (pure transform over outcome) | role-match |
| `bench/evaluators/swebench/verified.go` (NEW) | gate/producer | transform (3-oracle compose) | `bench/evaluators/completion_gate/gate.go` (`Grade`+`ApplyToMetrics`) | exact (CLONE) |
| `bench/evaluators/swebench/differential.go` (NEW) | transform | transform (diff-text parse) | `bench/evaluators/regression_checker/regression_checker.go` (pure null+MetricError on empty denom) | role-match |
| `bench/evaluators/swebench/rescore.go` (NEW) | transform | transform (compare two reports) | `bench/evaluators/completion_gate/gate.go` (distinct-pointer compose) | role-match |
| `bench/evaluators/swebench/*_test.go` (NEW) | test | — | `bench/evaluators/completion_gate/gate_test.go`, `bench/datasets/aider-polyglot/clone_test.go` (network-gated live leg) | exact |
| `bench/evaluators/swebench/testdata/*` (NEW) | fixture | — | `bench/datasets/crosscodeeval/testdata/`, `bench/container/testdata/` | role-match |
| `bench/evaluators/swebench/VERIFIED.md` (NEW) | doc | — | `bench/evaluators/VERIFIED.md` | exact |
| `bench/datasets/swebench-utboost/pin.go` (NEW) | config/pin | — | `bench/datasets/aider-polyglot/pin.go` (`isHexSHA1`+`PinnedSHA`) | exact |
| `bench/datasets/swebench-utboost/fetch.go` (NEW) | fetcher | file-I/O + network (gated) | `bench/datasets/crosscodeeval/fetch.go` (`LimitReader`+cache+gate) | exact |
| `bench/runtime/result.go` (MODIFY) | model/schema | transform | self — `EmbedderID`/`Language`/`AblationStatus` additive keys | exact (in-file precedent) |
| `bench/schema/result.v2.schema.json` (MODIFY) | schema | — | self — `language`/`ablation_status` optional-open-key entries | exact (in-file precedent) |

## Pattern Assignments

### `bench/evaluators/swebench/verified.go` (gate, CLONE of completion_gate)

**Analog:** `bench/evaluators/completion_gate/gate.go` — clone the EXACT structure (`GateResult`, `Grade`, `ApplyToMetrics`). This is the load-bearing analog for VERIFIED-01.

**Fail-closed abstain + all-three-required** (gate.go:103-137): the abstain branch returns `VerifiedCorrectness: &f` (explicit non-nil `*bool` to `false`) WITHOUT consulting oracles. The oracle branch computes `ok := em && es >= cfg.ESThreshold && id` then binds independent locals (`emv, idv, okv, esv := ...`) so no pointer aliases another. For this phase, swap the three completion oracles for the three test-execution oracles:
- (a) `canonicalPass` = all `FAIL_TO_PASS.failure` empty in the canonical report
- (b) `augmentedPass` = augmented report non-nil AND its `FAIL_TO_PASS.failure` empty
- (c) `noRegress` = `len(PASS_TO_PASS.failure)==0 && len(PASS_TO_FAIL.failure)==0` (read DIRECTLY from canonical report per Open Q3)
- `abstain := augmentedReport == nil` → drives `&false` (Pitfall 3: never infer pass from absence)

**Additive producer — no schema bump** (gate.go:147-150):
```go
func (r GateResult) ApplyToMetrics(m *evaluators.Metrics) []evaluators.MetricError {
	m.VerifiedCorrectness = r.VerifiedCorrectness
	return r.Errs
}
```
Mutates ONLY `VerifiedCorrectness`. The doc comment (gate.go:33-39) explicitly notes "introduces NO new result.v2.schema.json field" — mirror that.

**CRITICAL divergence point** (test_runner.go:37-40 + 46-55): `task_success` and `verified_correctness` are DISTINCT pointers. test_runner.go currently sets `success := post.Passed; verified := post.Passed` and returns `&success`/`&verified` separately, with the comment "returned as distinct pointers so Plan 04 may later diverge verified_correctness (e.g. a UTBoost rescore) without touching task_success." **This phase IS that anticipated divergence.** Assign `taskSuccess ← canonical.Resolved` (the harness `report.resolved` bool — Pitfall 1, NOT a tests_status count) SEPARATELY from the gate verdict, so SC#2's buggy patch lands `task_success=true, verified_correctness=false`.

---

### `bench/evaluators/swebench/harness.go` (subprocess seam, mirror container.Engine)

**Analog:** `bench/container/engine.go` lines 48-96 + `bench/container/run.go` lines 34-44, 106-109.

**Fixed-argv builder with pre-exec validation** (engine.go:48-60, the `pullArgs` seam): build the argv as a fixed slice, fail-closed on every value BEFORE crossing os/exec, return `([]string, error)` so a hermetic test asserts the golden argv without a live process. Mirror this for the `python -m swebench.harness.run_evaluation` argv. Validate per the Security Domain: dataset name against an allowlist of 2 known names; `--predictions_path`/log paths via an `isValidMountPath`-style check (clean+absolute+no `..`, run.go:34-44 rejects leading `-`); `run_id` `[A-Za-z0-9_-]+`; each `instance_id`. Use a `--` separator where the CLI accepts it (run.go:106-109).

**Strict env allowlist** (engine.go:88-96):
```go
func allowlistEnv() []string {
	env := make([]string, 0, 3)
	for _, k := range []string{"PATH", "HOME", "HELIX_CACHE_DIR"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}
```
Never forward the inherited env.

**Procgroup + ctx cancel** (engine.go:72-78): `cmd := exec.CommandContext(ctx, bin, args...); cmd.SysProcAttr = procGroupAttr(); cmd.Env = allowlistEnv()`. `procGroupAttr()` is build-tag-split (`procgroup_unix.go`/`procgroup_windows.go`) — reuse the same package idiom so a ctx cancel group-kills the harness + its Docker descendants.

**SKIP sentinel** (engine.go:24-26, 33-40): mirror `errEngineUnavailable = errors.New(...)` and `Detect()` returning it when the tool is absent — the live smoke `t.Skip`s on no-`python`/no-swebench/no-docker.

---

### `bench/evaluators/swebench/report.go` (ingester, mirror test_runner pure-transform)

**Analog:** `bench/evaluators/test_runner/test_runner.go` (pure transform, no subprocess) + the upstream JSON contracts in RESEARCH.md §Code Examples (the committed fixtures).

Define strict structs for the run-report (`{model}.{run_id}.json`) and per-instance `report.json` (`{resolved, tests_status.{FAIL_TO_PASS,PASS_TO_PASS,FAIL_TO_FAIL,PASS_TO_FAIL}.{success,failure}}`). Parse over committed fixtures only (hermetic). Like test_runner, this owns NO subprocess — it consumes JSON the harness produced. **Pitfall 1:** `task_success` is `report.resolved` authoritative, never a row count (the same exit-authoritative discipline as test_runner.go:7-11, 73-78).

---

### `bench/evaluators/swebench/differential.go` (transform, mirror regression_checker null discipline)

**Analog:** `bench/evaluators/regression_checker/regression_checker.go` lines 49-65 (the empty-denominator null+MetricError path).

Pure text transform (per Open Q5): parse `diff --git a/<f> b/<f>` headers from the gold `patch` and agent `model_patch` to get touched-file sets; emit `overlap = |gold ∩ agent| / |gold|`. On empty/nil gold, return `(nil, *MetricError)` mirroring regression_checker.go:59-65:
```go
if denom == 0 {
	return nil, &evaluators.MetricError{Metric: metricName, Grader: graderName, Reason: "..."}
}
```
Never fabricate a 0 or 1. Stamp a `graderName` const (regression_checker.go:35) into every MetricError.

---

### `bench/evaluators/swebench/rescore.go` (transform)

**Analog:** `bench/evaluators/completion_gate/gate.go` (distinct-pointer compose). Pure comparison of two report JSONs (canonical vs UTBoost-augmented) producing the raw + rescored verdicts as independent pointers. Reproducible per `run_id` (Pitfall 4: sort any map keys before emit).

---

### `bench/datasets/swebench-utboost/pin.go` + `fetch.go`

**pin.go analog:** `bench/datasets/aider-polyglot/pin.go` lines 15-22 — `const PinnedSHA = "..."` (the UTBoost rev `4c21a4831d80b66e976f2a5ce946a0abded7a2aa` `[ASSUMED — human-verify A1]`) + `isHexSHA1(s)` (exactly-40-lowercase-hex). Pitfall 5: refuse a mutable ref.

**fetch.go analog:** `bench/datasets/crosscodeeval/fetch.go` lines 23-24, 94, 124 — `io.LimitReader(resp.Body, maxParquetBytes+1)` (256 MiB cap on untrusted bodies), `HELIX_CACHE_DIR` cache precedence (`cacheDir()`/`cachePath()`), `isHexSHA1(rev)` pre-fetch refusal, `isValidHTTPSHost`. Network leg gated.

---

### `bench/runtime/result.go` (MODIFY — additive keys, mirror EmbedderID exactly)

**Analog:** SELF — `EmbedderID`/`Language`/`AblationStatus` (result.go:57-93 in `ResultInput`, 134-149 in `resultDoc`, 196-198 in `BuildResult`).

Add to `resultDoc` (result.go:~149, mirroring lines 138/145/149):
```go
ContainerID string `json:"container_id,omitempty"`
ExitCode    *int   `json:"exit_code,omitempty"`  // *int: a clean 0 is real, omit only when truly absent
```
Add matching `ResultInput` fields (mirror result.go:57-84) and wire them in `BuildResult`'s `doc := resultDoc{...}` literal (result.go:196-198 add `ContainerID: in.ContainerID, ExitCode: in.ExitCode`). **Pitfall 2:** `exit_code` MUST be `*int` (NOT `int`) — a value type with `omitempty` would wrongly drop a legitimate `0`; nil = "no exit code captured."

---

### `bench/schema/result.v2.schema.json` (MODIFY — optional open keys)

**Analog:** SELF — the `language` (line 182) and `ablation_status` (line 178) optional-key entries. Add `container_id` (string) and `exit_code` (`["integer","null"]` to match the `*int`) as OPTIONAL properties with descriptions citing "Additive-only (no schema major bump; schema_version stays \"v2\"; additionalProperties left OPEN; NOT added to required)." The `$comment` versioning policy (line 6) and `"required": ["schema_version"]` (line 8) stay UNCHANGED.

---

### `bench/aggregator/report.go` (raw-vs-rescored column — mirror CanaryPassRate/ByLanguage)

**Analog:** `LeaderRow.CanaryPassRate` (report.go:59-70) and `LanguageRow`/`Report.ByLanguage` (report.go:90-102, 124-127).

Add `RawScore`/`RescoredScore` as ADDITIVE fields that "NEVER alter the existing leaderboard" (report.go:64) and render an em-dash (`emDash = "—"`, report.go:33) when absent — never a fabricated 0 (report.go:37). Mirror `CanaryPassRate`'s "consumes ZERO RNG so the determinism contract is untouched" property. Render only into a SWE-bench-specific report path so existing leaderboard/cost goldens stay byte-for-byte. Pitfall 4: sort before emit (the package's "everything SORTED" discipline, `sortLeaderRows` report.go:181).

---

## Shared Patterns

### Distinct-pointer metric production (fail-closed, additive)
**Source:** `bench/evaluators/completion_gate/gate.go:75-137`, `bench/evaluators/test_runner/test_runner.go:26-55`
**Apply to:** `verified.go`, `report.go`, `rescore.go`, `differential.go`
Every metric is a nullable `*bool`/`*float64`/`*int`; locals bound independently (no aliasing); `task_success` and `verified_correctness` NEVER share a pointer; abstain/missing-oracle → explicit `&false` (never nil-drop, never a spurious true); could-not-score → nil + `MetricError{Metric, Grader, Reason}`.

### Fixed-argv + strict-env + procgroup subprocess
**Source:** `bench/container/engine.go:48-96`, `bench/container/run.go:34-44,106-109`, `bench/datasets/aider-polyglot/clone.go:74-117`
**Apply to:** `harness.go`, `swebench-utboost/fetch.go`
Build argv as a fixed slice; validate every value pre-exec (allowlist/regex/clean-path, reject leading `-`); `--` terminator; `allowlistEnv()` (PATH/HOME/HELIX_CACHE_DIR only); `procGroupAttr()`; `errEngineUnavailable`-style SKIP sentinel; `runShim`-style seam returning argv for hermetic golden assertion.

### Pinned-fetch with cache + network gate
**Source:** `bench/datasets/aider-polyglot/pin.go:15-22`, `clone.go:25-26,105-117`, `bench/datasets/crosscodeeval/fetch.go:23-24,94,124`, `clone_test.go:171-181`
**Apply to:** `swebench-utboost/pin.go`, `swebench-utboost/fetch.go`, the live-leg tests
`const PinnedSHA` + `isHexSHA1` mutable-ref refusal; `HELIX_CACHE_DIR` precedence; `io.LimitReader` body cap; live leg `t.Skip` unless `HELIX_BENCH_NETWORK` set (clone_test.go:175-176).

### Additive open-provenance key (no schema bump)
**Source:** `bench/runtime/result.go:57-149,196-198`, `bench/schema/result.v2.schema.json:6,178-184`
**Apply to:** `result.go` (`container_id`/`exit_code`), `result.v2.schema.json`
`omitempty` Go field + OPTIONAL schema key; `schema_version` stays `"v2"`; NOT added to `required`; `additionalProperties` stays OPEN; old goldens byte-stable. Pointer types where a zero value is meaningful (`exit_code *int`).

### Network/Docker-gated live test, hermetic fixture as sole proof
**Source:** `bench/datasets/aider-polyglot/clone_test.go:171-181`, `bench/evaluators/completion_gate/gate_test.go`
**Apply to:** all `swebench/*_test.go`
Live legs `t.Skip` cleanly (per MEMORY bench-false-green: plain `go test` SKIPs them). The SC#2 buggy-patch verdict (`task_success=true AND verified_correctness=false`) MUST be a hermetic fixture test (`report.canonical.json` resolved=true + canonical pass; `report.utboost.json` augmented FAIL_TO_PASS has a `failure`) — never the live smoke as sole proof (Pitfall 6).

## No Analog Found

None. Every file in this phase clones a verified in-tree analog; the only net-new logic (predictions producer, report parser, 3-condition gate, rescore comparator, differential) is small glue over the two upstream JSON contracts captured as fixtures.

## Metadata

**Analog search scope:** `bench/evaluators/{completion_gate,test_runner,regression_checker,metrics.go}`, `bench/container/`, `bench/datasets/{aider-polyglot,crosscodeeval}/`, `bench/runtime/result.go`, `bench/aggregator/report.go`, `bench/schema/result.v2.schema.json`
**Files scanned:** ~12 source files (direct read + targeted grep)
**Pattern extraction date:** 2026-06-21
