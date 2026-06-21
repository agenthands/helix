# Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 13 new + 2 modified
**Analogs found:** 13 / 13 (all have in-tree analogs; the parquet-decode aspect of the two `fetch.go` files is the only genuinely new I/O surface)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `bench/datasets/crosscodeeval/fetch.go` | dataset-loader (network) | file-I/O / request-response | `bench/datasets/aider-polyglot/clone.go` + `bench/ragindex/cache.go` | role-match (HTTP+parquet vs git clone) |
| `bench/datasets/crosscodeeval/loader.go` | dataset-loader | transform | `bench/datasets/aider-polyglot/loader.go` | exact |
| `bench/datasets/crosscodeeval/pin.go` | config (pinned rev) | — | `bench/datasets/aider-polyglot/pin.go` | exact |
| `bench/datasets/crosscodeeval/loader_test.go` + `fixtures/` | test | transform | `bench/datasets/aider-polyglot/loader_test.go` + `clone_test.go` | exact |
| `bench/datasets/repobench/fetch.go` | dataset-loader (network) | file-I/O | `bench/datasets/aider-polyglot/clone.go` + `bench/ragindex/cache.go` | role-match |
| `bench/datasets/repobench/loader.go` | dataset-loader | transform | `bench/datasets/aider-polyglot/loader.go` | exact |
| `bench/datasets/repobench/pin.go` | config | — | `bench/datasets/aider-polyglot/pin.go` | exact |
| `bench/datasets/repobench/loader_test.go` + `fixtures/` | test | transform | `bench/datasets/aider-polyglot/loader_test.go` | exact |
| `bench/evaluators/exactmatch/` (EM scorer) | evaluator (pure) | transform | `bench/evaluators/test_runner/test_runner.go` (grader Result shape) | role-match |
| `bench/evaluators/editsim/` (normalized Levenshtein) | evaluator (pure) | transform | `bench/evaluators/test_runner/test_runner.go` | role-match |
| `bench/evaluators/identmatch/` (identifier set EM+F1) | evaluator (pure) | transform | `bench/evaluators/test_runner/test_runner.go` | role-match |
| `bench/evaluators/completion_gate/` (multi-oracle gate) | evaluator / coordinator | transform | `bench/evaluators/test_runner/test_runner.go` + `coordinator/coordinator.go` | exact (the anticipated divergent producer) |
| `bench/evaluators/VERIFIED.md` | doc artifact | — | `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (verify-gate doc) | role-match |
| `bench/aggregator/report.go` (CanaryPassRate column) | MODIFY (aggregator) | transform | existing `LeaderRow` / `LanguageRow` (report.go:45,85) | exact (additive field) |
| `cmd/helix-bench/main.go` (fetch-datasets) | MODIFY (CLI cmd) | request-response | `newFetchDatasetsCmd` stub (main.go:353) | exact (replace stub body) |

## Pattern Assignments

### `bench/datasets/{crosscodeeval,repobench}/fetch.go` (dataset-loader, network)

**Analogs:** `bench/datasets/aider-polyglot/clone.go` (SSRF-pinned fetch + strict env + path safety) and `bench/ragindex/cache.go` (cache-dir precedence).

**Cache-dir pattern — copy VERBATIM** (`bench/ragindex/cache.go:18-41`, already re-cloned once in `aider-polyglot/clone.go:13-34`):
```go
const cacheDirEnv = "HELIX_CACHE_DIR"
const cloneSubdir = "crosscodeeval" // or "repobench"
func cacheDir() string {
	if d := os.Getenv(cacheDirEnv); d != "" { return d }
	if d, err := os.UserCacheDir(); err == nil { return filepath.Join(d, "helix") }
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".helix", "cache")
}
```

**Cache-path with rev-validation** (mirror `clone.go:40-45` `clonePath`): validate the dataset rev/sha shape BEFORE `filepath.Join`, fail-closed on a non-validated segment. Use `validatePathSegment` (cloned from `loader.go:81-89`) for the rev and per-language file names so a traversal segment cannot escape `<cacheDir>/crosscodeeval/<rev>/`.

**SSRF pin** (mirror `clone.go:51-56` `isValidGitURL` + `pin.go:6` `RepoURL` constant): pin the HF host + dataset repo as package constants; NEVER fetch a caller-supplied URL. Build the resolve URL `https://huggingface.co/datasets/<repo>/resolve/<rev>/<path>` from constants + a validated rev. Reject leading-`-` and require `https://`.

**Strict env / cap size** (mirror `clone.go:105-114` `gitEnv`): for `net/http`, set an explicit timeout/context and cap the downloaded body size (security: untrusted parquet decode). Use `exec.CommandContext`-equivalent cancellation via `http.NewRequestWithContext`.

**Parquet decode (genuinely new):** use `apache/arrow-go/v18` `parquet/file` + `parquet/pqarrow` (promote indirect→direct in go.mod, line 64; `go get github.com/apache/arrow-go/v18@v18.5.1`). No in-tree analog — this is the only net-new library surface. Keep it confined to `fetch.go`/`loader.go` so the package stays a stdlib+arrow leaf (mirror the `ragindex` leaf invariant doc comment at `cache.go:1-6`).

---

### `bench/datasets/{crosscodeeval,repobench}/loader.go` (dataset-loader, transform)

**Analog:** `bench/datasets/aider-polyglot/loader.go` (exact precedent).

**Package doc comment** (mirror `loader.go:1-15`): state `dataset-loader-only`, the LEAF invariant (stdlib + arrow only), and that the hermetic fixture test is the SOLE authoritative proof while the live fetch is network-gated and never the sole proof.

**Task struct** (mirror the `Exercise` struct `loader.go:54-74`): carry `Language string` (populates the result.v2 `language` provenance key consumed by aggregator `ByLanguage`), plus `Prompt`, `GroundTruth`, `Split`, and a `CanarySeed`/`canary_contaminated` flag. For RepoBench also carry `Context []string` + `GoldSnippetIndex` (R sub-task) and `NextLine` (C/P sub-tasks).

**Path-segment validation** (copy `loader.go:81-89` `validatePathSegment` + `copyFile` guard `loader.go:130-132`): run BEFORE any `filepath.Join` on rev/language/file names. This is the V5 control the security section requires.

**Decode-then-validate** (mirror `loadExercise` `loader.go:96-124`): read decoded rows, validate required columns are present (`len(...)==0 ⇒ error`), return a typed error (never panic) so the coordinator can null-on-failure.

---

### `bench/datasets/{crosscodeeval,repobench}/pin.go` (config)

**Analog:** `bench/datasets/aider-polyglot/pin.go` (exact).

**Pattern** (mirror `pin.go:1-36`): a `RepoURL`/dataset-repo constant + a `PinnedRev` constant (HF revision, not a mutable branch) with a doc comment explaining the mutable-ref tampering threat (`pin.go:8-14`) and a `isHexSHA1`-style total, regexp-free validator for the rev shape (`pin.go:22-36`). NOTE per RESEARCH Pitfall 3 / Open Q5: pin the CCE rev at a `checkpoint:human-verify` (community mirror `Vincentvmt/CrossCodeEval` has a viewer cast-error).

---

### `bench/datasets/*/loader_test.go` + `fixtures/` (test)

**Analogs:** `bench/datasets/aider-polyglot/loader_test.go` (hermetic fixtures) + `clone_test.go:174-181` (network gate).

**Hermetic fixture test** (mirror `loader_test.go:20-50`): commit small paper-shaped fixture rows under `fixtures/<lang>/`; the loader + scorers + gate run fully on fixtures. This is the SOLE authoritative proof.

**Network-gated live leg — copy the gate idiom VERBATIM** (`clone_test.go:174-181`):
```go
func TestLive...(t *testing.T) {
	if os.Getenv("HELIX_BENCH_NETWORK") == "" {
		t.Skip("set HELIX_BENCH_NETWORK=1 to run the live fetch (network-gated)")
	}
	// optional: quick reachability dial (3s) → t.Skipf on unreachable (clone_test.go:178-181)
}
```

**Fail-closed path tests** (mirror `loader_test.go:41-50` `TestLoadExercisePathSegmentRejected`): assert traversal segments in rev/lang/file names are rejected.

---

### `bench/evaluators/{exactmatch,editsim,identmatch}/` (pure scorers)

**Analog:** `bench/evaluators/test_runner/test_runner.go` (grader package shape) + `bench/evaluators/metrics.go` (the nullable contract).

**Package + Result shape** (mirror `test_runner.go:19-32`): each scorer is its own subpackage importing the parent `evaluators` for `MetricError`. Pure functions, no I/O:
- `exactmatch.EM(pred, gold string) bool`
- `editsim.ES(pred, gold string) float64` — normalized Levenshtein ratio `1 - lev/max(len)` in `[0,1]` (RESEARCH Pitfall 2: do NOT reuse `patch_validator.EditDistancePatch` — that is git-numstat line distance, wrong metric).
- `identmatch.Match(pred, gold string) (em bool, f1 float64)` — tokenize identifiers `[A-Za-z_]\w*` minus keywords (RESEARCH Pitfall 4), set-compare. Document the tokenizer in `VERIFIED.md`.

**Null-on-failure discipline** (mirror `test_runner.go:46-63`): return typed nullable pointers + `[]evaluators.MetricError` rather than a Go error; stamp a `graderName` const (`test_runner.go:21`).

---

### `bench/evaluators/completion_gate/` (multi-oracle gate → verified_correctness)

**Analogs:** `bench/evaluators/test_runner/test_runner.go` (the divergent producer this is explicitly anticipated to be — see `test_runner.go:37-40` comment) + `coordinator/coordinator.go` (fan-out, never-short-circuit discipline).

**KEY: `verified_correctness` ALREADY EXISTS** — `evaluators.Metrics.VerifiedCorrectness *bool` (`metrics.go:21`), produced today by `test_runner.Grade` (`test_runner.go:52-54`) and threaded by `coordinator.go:79`. This gate is the NON-test-bearing sibling producer; additive, no schema bump.

**Result shape** (mirror `test_runner.go:26-32`):
```go
type GateConfig struct{ ESThreshold float64 } // per-oracle configurable (VERIFIED-03)
type GateResult struct {
	VerifiedCorrectness *bool
	EM, IDMatch         *bool
	ES                  *float64
	Errs                []evaluators.MetricError
}
```

**Gate logic + abstain → explicit false** (the load-bearing VERIFIED-03 invariant; mirror `test_runner.go:46-63` `&value` pattern):
```go
func Grade(pred, gold string, cfg GateConfig, abstain bool) GateResult {
	if abstain { f := false; return GateResult{VerifiedCorrectness: &f} } // explicit FALSE, never a false-positive true, never nil-drop
	em := exactmatch.EM(pred, gold)
	es := editsim.ES(pred, gold)
	id, _ := identmatch.Match(pred, gold)
	ok := em && es >= cfg.ESThreshold && id // all three required
	return GateResult{VerifiedCorrectness: &ok, EM: &em, ES: &es, IDMatch: &id}
}
```
**Critical (RESEARCH Pattern 2):** abstain emits `&false`, distinct from "could not score" which is a `MetricError` + nil. Do NOT nil-drop on abstain.

**Wiring into Metrics** (mirror `coordinator.go:77-87`): for the completion path, assign `m.VerifiedCorrectness = gateRes.VerifiedCorrectness` via a thin completion coordinator or a guarded branch (there is no `TestOutcome` on this path). Never short-circuit; append `Errs`.

---

### `bench/evaluators/VERIFIED.md` (doc artifact, SC#3 acceptance)

**Analog:** `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (the doc that a `verify-*` Makefile gate strict-decodes).

**Pattern:** document the gate (EM + edit-sim≥thresh + identifier-match all required), the per-oracle threshold knob, the abstain→false rule, and the identifier tokenizer. If a hard-gate is warranted, add a `verify-verified-md` Makefile target mirroring `verify-licenses` (Makefile:312-319) that strict-decodes required sections and exits non-zero on a missing one.

---

## MODIFY targets

### `bench/aggregator/report.go` — add CanaryPassRate column

**Analog:** the existing additive `LeaderRow` (report.go:45-58) and `LanguageRow` (report.go:85-89) / `ByLanguage` (report.go:111-114).

Add a `CanaryPassRate ciValue` field to `LeaderRow` (report.go:50-57 shows the `ciValue` field convention). PURELY ADDITIVE — mirror the Phase 85 `ByLanguage` discipline (never alters the (mode x benchmark) leaderboard). The reduce logic mirrors `reduceLanguageRows` (`aggregate.go:225-272`) reading an additive doc key (`rowLanguage`, `aggregate.go:368-375`); the canary reduce reads a new `canary_contaminated`/`canary_pass` additive key the same way.

### `cmd/helix-bench/main.go` — implement fetch-datasets

**Analog:** the existing `newFetchDatasetsCmd` stub (`main.go:353-360`, currently `RunE: notYetImplemented("fetch-datasets")`).

Replace the `notYetImplemented` body with a `RunE` that invokes the two adapters' `fetch.go` Fetch functions (network-gated). Keep the cobra command shape (`Use`/`Short`/`RunE`) identical to the surrounding skeleton commands (`main.go:362-369`).

## Shared Patterns

### Cache-dir resolution (apply to BOTH fetch.go files)
**Source:** `bench/ragindex/cache.go:18-41` (already cloned verbatim into `aider-polyglot/clone.go:13-34`)
Three-step precedence `HELIX_CACHE_DIR` → `os.UserCacheDir()/helix` → `~/.helix/cache`. Copy verbatim.

### Path-segment / SSRF safety (apply to BOTH fetch.go + loader.go)
**Source:** `aider-polyglot/loader.go:81-89` (`validatePathSegment`), `clone.go:51-56` (URL guard), `pin.go:22-36` (rev-shape validator)
Validate every rev/language/path segment BEFORE `filepath.Join`; pin host+repo as constants; never fetch a caller-supplied URL.

### Nullable-grader / null-on-failure (apply to ALL evaluators subpkgs + the gate)
**Source:** `bench/evaluators/metrics.go:19-49` + `test_runner.go:26-63` + `coordinator.go:64-71`
Every metric is a `*pointer`; a grader that cannot compute leaves the pointer nil and appends a `MetricError{Metric, Grader, Reason}`. NEVER return a Go error from a grader; NEVER short-circuit; always return the full record.

### Hermetic-fixture + network-gated-live (apply to BOTH adapters' tests)
**Source:** `aider-polyglot/loader_test.go:20-50` (hermetic) + `clone_test.go:174-181` (`HELIX_BENCH_NETWORK` gate + reachability probe)
Fixtures are the SOLE authoritative proof; the live fetch SKIPs cleanly offline and is never the sole proof.

### Per-language slicing (reuse verbatim, no new code)
**Source:** `aggregator/aggregate.go:225-272` (`reduceLanguageRows`) + `:368-375` (`rowLanguage`) + Phase 85 `language` result key
CCE/RepoBench loaders just populate the `language` field on each Task; `ByLanguage` slices it automatically. Phase 85 `bench/languages/{python,java,typescript,csharp}/` runners already exist — no new runner.

## No Analog Found

| File aspect | Role | Reason |
|------|------|--------|
| parquet decode inside `fetch.go` | dataset-loader | `apache/arrow-go/v18 parquet/pqarrow` has ZERO existing direct usage in-tree (currently `// indirect`, go.mod:64). The fetch/cache/path-safety scaffolding has analogs; only the columnar decode is net-new. Confine it to `fetch.go`/`loader.go` to preserve the leaf invariant. Use RESEARCH §Code Examples + arrow-go docs (context7) for the decode API. |

## Metadata

**Analog search scope:** `bench/datasets/`, `bench/evaluators/`, `bench/aggregator/`, `bench/ragindex/`, `bench/languages/`, `cmd/helix-bench/`, `Makefile`, `go.mod`
**Files scanned:** ~14 (all line-anchored above)
**Pattern extraction date:** 2026-06-21
