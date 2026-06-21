---
phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - bench/evaluators/multiswebench/config.go
  - bench/evaluators/multiswebench/harness.go
  - bench/evaluators/multiswebench/ingest.go
  - bench/evaluators/multiswebench/report.go
  - bench/evaluators/terminalbench/harness.go
  - bench/evaluators/terminalbench/ingest.go
  - bench/evaluators/terminalbench/report.go
  - bench/longwall/checkpoint.go
  - bench/longwall/scheduler.go
  - bench/datasets/multi-swe-bench-mini/fetch.go
  - bench/datasets/multi-swe-bench-mini/pin.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 88: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

Reviewed the Phase 88 external-benchmark adapters: the Multi-SWE-bench config.json
producer + harness subprocess seam + report ingestion, the Terminal-Bench `tb run`
seam + ingestion, the net-new long-wall checkpoint/resume state machine, and the
Mini-set pinned-rev fetcher. All four packages build clean, vet clean, and pass
their hermetic test suites.

The high-risk net-new surfaces hold up well under adversarial reading:

- **config.go path validation** is total and fail-closed: all four scalar path
  fields plus every `PatchFiles[]`/`DatasetFiles[]` entry are `isValidPredictionsPath`-
  checked BEFORE marshal, and the write is temp+rename (verified by tests that assert
  no file is left on refusal and no leftover `.tmp-*` on success). The fixed argv
  `["-m","multi_swe_bench.harness.run_evaluation","--config",<path>]` re-validates
  the path at the os/exec boundary.
- **longwall checkpoint/resume** satisfies its five stated invariants: atomic
  temp+rename write, resume skips `StatusDone` cells (runner invoked 0 times), a
  FAILED/partial cell is persisted as `failed`/never-`done` and re-runs, re-entry is
  idempotent, and a read/parse error conservatively re-runs rather than skips. The
  SC#3 >24h guarantee is genuinely hermetic (injected clock, 48h-advance test, no
  real wall time).
- **mini-set fetch/pin** carries the audited SSRF/DoS/tamper discipline: pinned
  Host+DatasetID-only URL, `isHexSHA1` mutable-ref refusal before any network call,
  `io.LimitReader(cap+1)` overflow probe on both network and cache legs,
  `validatePathSegment` before every `filepath.Join`, atomic cache write, and a
  fail-closed content-digest layer. The placeholder `PinnedSHA`
  (all-zeros) is a documented deferral that cannot silently fetch the wrong thing —
  it is not a valid 40-hex commit on HF, so the live fetch fails loudly rather than
  serving wrong content (and `isHexSHA1` is satisfied only for shape, not identity).

No BLOCKER-class defects were found. Three WARNING-class issues concern a mis-wired
runner-kind seam, a documentation-vs-behavior gap on process-group kill (inherited
from the Phase 87 template), and a panic-aborts-the-run robustness gap in the
long-wall driver. Info items cover swallowed write errors and minor consistency
notes.

## Warnings

### WR-01: Terminal-Bench runner-kind seam is mis-wired — Detect can resolve `harbor` while Run always builds `tb` argv

**File:** `bench/evaluators/terminalbench/harness.go:161-184, 202-209`
**Issue:** `Detect()` probes PATH in order `[runnerKindTB, runnerKindHarbor]` and
returns `&Harness{bin: p}` for whichever binary is found first — but the `Harness`
struct stores ONLY `bin`, not the `runnerKind` that resolved it. `Run` → `RunArgs`
then unconditionally calls `defaultRunnerKind.spec()` (always `tb`'s spec). So when
`tb` is absent but `harbor` is on PATH, `Detect` returns the `harbor` binary, yet
`RunArgs` builds the `tb`-shaped argv (`--dataset-name terminal-bench-core`, etc.).
Today this is LATENT (both `spec()` results carry identical `datasetName` and the
argv structure is shared), so no test catches it — `TestRunnerKindSeam` only asserts
each `spec()` in isolation, never that Detect and Run agree. But the entire stated
purpose of the seam (per the package doc, O-1) is that harbor's binary name AND its
dataset-flag form can diverge as a "one-line swap." The moment harbor's spec
actually differs, a `harbor`-resolved harness will be invoked with `tb` argv —
silently wrong, with no compile or test failure to catch it.
**Fix:** Record the resolved kind on the harness and use it in `Run`:
```go
type Harness struct {
	bin     string
	kind    runnerKind // resolved by Detect; drives RunArgs
	runShim func(args []string) error
}

func Detect() (*Harness, error) {
	for _, k := range []runnerKind{runnerKindTB, runnerKindHarbor} {
		if p, err := exec.LookPath(k.spec().binary); err == nil {
			return &Harness{bin: p, kind: k}, nil
		}
	}
	return nil, errHarnessUnavailable
}

// RunArgs takes the kind (or read h.kind in Run) instead of defaultRunnerKind:
func RunArgs(k runnerKind, r HarnessRun) ([]string, error) {
	spec := k.spec()
	// ...unchanged...
}
```
Add a test asserting that a `harbor`-resolving Detect produces harbor's spec in the
built argv, so the seam's divergence point is actually exercised.

### WR-02: Process-group kill is documented but not delivered — `Setpgid:true` without a `cmd.Cancel` override leaves Docker descendants orphaned on context cancel

**File:** `bench/evaluators/multiswebench/harness.go:86-107`, `bench/evaluators/terminalbench/harness.go:211-244`
**Issue:** Both `Run` implementations set `cmd.SysProcAttr = procGroupAttr()`
(`Setpgid: true`) and the surrounding doc claims this lets "a context cancel
group-kill the harness AND the per-instance Docker descendants it spawns
(T-88-01-03 / T-88-02-06)." But `exec.CommandContext` with no `cmd.Cancel` override
kills only the single direct-child PID (`cmd.Process.Kill()`), and because
`Setpgid: true` places that child in its OWN new process group, the default kill
sends SIGKILL to the leader PID — NOT to `-pgid` — so the group is never reaped. The
python/tb child dies but its Docker-spawning descendants are orphaned (re-parented
to init), which is the opposite of the documented intent. The correct primitive is
already in the codebase: `bench/runtime/subprocess/ragserver.go:92,104` uses
`syscall.Kill(-pid, syscall.SIGKILL)`. NOTE: this is inherited verbatim from the
Phase-87 swebench template (same defect at `bench/evaluators/swebench/harness.go`),
so it is not a Phase-88 regression — but the net-new files restate the group-kill
claim, so the doc overstates the actual behavior.
**Fix:** Override `cmd.Cancel` (Go 1.20+) to signal the group, and ideally
`cmd.WaitDelay` for a SIGKILL escalation:
```go
cmd := exec.CommandContext(ctx, h.bin, args...)
cmd.SysProcAttr = procGroupAttr()
cmd.Cancel = func() error {
	if cmd.Process != nil {
		// Negative pid → whole process group (matches ragserver.go).
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return nil
}
cmd.WaitDelay = 10 * time.Second
```
Gate the `syscall.Kill` call behind the same `!windows` build tag as
`procGroupAttr`. If a full fix is out of scope, at minimum soften the doc comments
so they do not claim group-kill the code does not perform.

### WR-03: `cellKey` panics on a bad segment — a single malformed cell coordinate aborts the entire long-wall run

**File:** `bench/longwall/checkpoint.go:74-82`, `bench/longwall/scheduler.go:71-101`
**Issue:** `cellKey` calls `panic(err)` when any of `benchmark/lang/mode/task`
contains a path separator, `..`, or is empty. `Scheduler.Run` invokes
`c.Key()` (→ `cellKey`) inside the per-cell loop with no `recover`. The package
rationale is "a malformed coordinate is a programming error in the caller, never
untrusted input." But the long-wall driver's whole reason for existing is surviving
>24h runs and restarts, and `Lang`/`Task` coordinates are routinely sourced from
dataset DIRECTORY names and dataset-file contents (the package doc and the
multiswebench "language is the dataset-file directory" contract both say so) — i.e.
external data. A single task id containing `/` (or an empty lang from a malformed
dataset row) would `panic` and tear down the entire matrix run mid-flight, defeating
the resume guarantee for every OTHER cell in the same pass. Traversal is correctly
REJECTED (the security requirement is met), but the failure MODE is too violent for
a resilience-oriented driver.
**Fix:** Make `Key()` fallible and let the scheduler count the bad cell as Failed
(re-runnable / surfaced) instead of crashing the pass:
```go
func (c CellID) Key() (string, error) {
	return cellKeyChecked(c.Benchmark, c.Lang, c.Mode, c.Task, c.RunIndex)
}
// in Scheduler.Run:
key, err := c.Key()
if err != nil {
	sum.Failed++
	continue // a malformed coordinate fails THAT cell, not the whole run
}
```
Keep the validation total; only change panic → returned error at the driver boundary.

## Info

### IN-01: Scheduler swallows the checkpoint write error, conflating "ran + persisted done" with "ran but done was never recorded"

**File:** `bench/longwall/scheduler.go:88-98`
**Issue:** `_ = s.store.writeCheckpoint(...)` discards the write error. The behavior
is SAFE for correctness (a lost `done` write simply re-runs the cell on the next
pass — the conservative direction), and the package doc acknowledges "a write error
aborts the cell conservatively (counted Failed)." But the code does NOT actually
count it Failed: on a clean `oc.Success` it still does `sum.Ran++` even when the
`done` checkpoint failed to persist, so the Summary reports a cell as `Ran`
(implying durable completion) when nothing was checkpointed. The doc and code
disagree.
**Fix:** Branch on the write result so the Summary reflects reality:
```go
if werr := s.store.writeCheckpoint(cs); werr != nil {
	sum.Failed++ // ran but could not persist done → must re-run; not a durable Ran
	continue
}
if oc.Success { sum.Ran++ } else { sum.Failed++ }
```

### IN-02: `validateSegment` redundant `os.PathSeparator` check; `..` substring check is broader than the comment

**File:** `bench/longwall/checkpoint.go:61-66`
**Issue:** The separator guard tests `'/'`, `'\\'`, AND `os.PathSeparator` — on
every supported platform `os.PathSeparator` is already one of `'/'` or `'\\'`, so
the third clause is dead. Separately, `strings.Contains(seg, "..")` rejects any
embedded `..` (e.g. a legitimate task name like `a..b`), which is stricter than the
"traversal sequence" the comment describes. Neither is a bug (fail-closed,
conservative), but the redundancy and over-broad match are worth tightening for
clarity. Low priority — keep it conservative if benchmark coordinates never contain
`..`.
**Fix:** Drop the redundant `os.PathSeparator` clause; if `a..b` segments are ever
valid, scope the traversal check to the exact `..` segment rather than substring.

### IN-03: `resolveURL` interpolates `DatasetID` (which contains `/`) into the URL unvalidated

**File:** `bench/datasets/multi-swe-bench-mini/fetch.go:89-100`, `pin.go:28`
**Issue:** `DatasetID = "ByteDance-Seed/Multi-SWE-bench"` is `fmt.Sprintf`'d into the
resolve URL with no validation, while `rev` and `file` are rigorously checked. This
is SAFE today because `DatasetID` is a compile-time constant under the maintainers'
control (not caller input), so there is no SSRF vector. Flagged only for
completeness: if `DatasetID` ever becomes configurable, it must be validated (and
its embedded `/` accounted for) before reaching the URL.
**Fix:** No change needed while `DatasetID` is constant. If made configurable, add a
host/repo-shape validator mirroring `isValidHTTPSHost`/`validatePathSegment`.

### IN-04: `multiswebench.Run` hardcodes `resolveWorkDir("")`, diverging from the swebench template's caller-controlled WorkDir

**File:** `bench/evaluators/multiswebench/harness.go:96`
**Issue:** Unlike `swebench.Run` (which passes `r.WorkDir`), `multiswebench.Run`
calls `resolveWorkDir("")` with a hardcoded empty string, always using the default
`<cacheRoot>/multiswebench-runs`. This is acceptable — `Run` takes only `cfgPath`
(no `HarnessRun`), and the effective workdir is carried inside the validated
`config.json` `Workdir` field instead — but the unconditional empty arg makes the
`resolveWorkDir(workDir string)` parameter dead for this caller and could mislead a
future maintainer into thinking the harness honors a caller-supplied workdir. Minor
clarity nit.
**Fix:** Either inline the default (drop the unused parameter for this package) or
add a one-line comment at the call site noting the workdir lives in config.json.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
