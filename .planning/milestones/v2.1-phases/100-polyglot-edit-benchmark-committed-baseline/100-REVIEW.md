---
phase: 100-polyglot-edit-benchmark-committed-baseline
reviewed: 2026-06-23T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - bench/runtime/aider_edit_agent.go
  - bench/runtime/aider_edit_cell.go
  - bench/runtime/aider_edit_baseline_regen.go
  - bench/aggregator/aider_edit_baseline.go
  - bench/datasets/aider-polyglot/loader.go
  - bench/runtime/cell.go
  - bench/runtime/result.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 100: Code Review Report

**Reviewed:** 2026-06-23
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Reviewed the Phase 100 polyglot-edit benchmark arm: the deterministic EDIT-verb
AgentFn (`aider_edit_agent.go`), the mode-name cell dispatch (`aider_edit_cell.go`
+ `cell.go`), the local-only byte-reproducible baseline regenerator
(`aider_edit_baseline_regen.go`), the deterministic summary renderer
(`aider_edit_baseline.go`), the two new exported loader wrappers (`loader.go`),
and the `EditFormatApplied *bool` plumbing (`result.go`).

Overall the code is careful and well-reasoned: session lifecycle is leak-free
(every `OpenSession` success is matched by a `defer sess.Close()`, and
`OpenSession` self-cleans on its own internal failures), the `*bool`
`EditFormatApplied` is read-after-write with no aliasing, the `is_regex:false`
literal path is metacharacter-safe, the loader wrappers are verbatim
pass-throughs, and the renderer sorts every list before emit.

The notable concern is **how the deterministic baseline reaches its
reproducibility** in `assembleAiderEditResult`: it calls `coordinator.Grade`
with `RepoDir:""`, which causes the patch_validator graders to silently run
`git` against the regenerator's *own* current working directory (the helix repo
working tree), then relies on a post-hoc null-out to discard those values. This
is correct only by accident of the happy-path environment and is the highest-risk
item below. There are no security vulnerabilities (no shell interpolation, no
untrusted-path joins — all path segments flow from the V5-validated config) and
no crash/data-loss bugs.

## Warnings

### WR-01: `assembleAiderEditResult` runs git against the wrong repo (the live helix tree) via `RepoDir:""`, masking it with a post-hoc null

**File:** `bench/runtime/aider_edit_cell.go:83-103`
**Issue:**
`assembleAiderEditResult` calls `coordinator.Grade(... GradeInput{RepoDir: ""})`,
then nulls `metrics.FilesModified` / `EditLocality` / `EditDistancePatch`
afterward (lines 101-103). The comment claims those metrics "are computed from a
LIVE git working tree (RepoDir)" and so are nulled — but `RepoDir` is the *empty
string*, not the cell's repo. Empirically confirmed: `patch_validator.EditLocality(ctx, "")`
and `EditDistancePatch(ctx, "")` run `git ls-files` / `git diff` with `cmd.Dir = ""`,
which makes git inherit the regenerator process's CWD — the helix repo root.
They **succeed** and compute real values from helix's *own* working tree,
including whatever uncommitted changes exist at regen time.

Two problems flow from this:

1. **Wasted/wrong computation masked by luck.** The values are semantically
   meaningless (they describe the helix repo, not the exercise) and only
   harmless because lines 101-103 overwrite them with `nil`. Anyone who later
   reorders the null-out, adds a fourth patch_validator metric, or reads the
   pre-null values inherits live-tree-dependent data into a "deterministic" path.

2. **Latent non-reproducibility in `metric_errors`.** Today the happy path
   (regen from a clean-enough helix repo with `git` on PATH) returns
   `locErr == nil` / `distErr == nil`, so `Grade` appends no patch_validator
   errors and the only patch_validator `metric_errors[]` entries are the 3
   explicit nulls. But if the regenerator ever runs where `git` is absent, or in
   a state where `git ls-files` errors, `Grade` appends a MetricError whose
   `Reason` embeds an environment-specific git error string (e.g.
   `"git ls-files failed: exec: \"git\": executable file not found in $PATH"`).
   That string lands in the committed `result.v2.json` / `BENCH-RESULTS.md`,
   breaking the byte-reproducibility guarantee the phase is built around — and it
   would not be caught by the double-render test (same env, same string) but
   would diverge across machines/CI.

**Fix:** Stop feeding the live tree into the deterministic path. Either pass a
`RepoDir` that fails *deterministically* (a guaranteed-nonexistent path, so the
git error is constant and machine-independent) and then drop ALL patch_validator
metric_errors before emit, or — cleaner — assemble the deterministic metrics
without invoking the patch_validator graders at all. For example, after `Grade`,
also strip any pre-existing patch_validator errors so only the 3 explicit nulls
remain:

```go
metrics.FilesModified = nil
metrics.EditLocality = nil
metrics.EditDistancePatch = nil
// Drop any patch_validator errors Grade may have produced from the (wrong/empty)
// RepoDir so the committed bytes never embed an environment-specific git error.
filtered := metricErrs[:0]
for _, e := range metricErrs {
    if e.Grader == "patch_validator" {
        continue
    }
    filtered = append(filtered, e)
}
metricErrs = append(filtered, /* the 3 explicit baseline nulls */ ...)
```

### WR-02: live apply seam treats a literal-no-match `replace_in_file` "success" as an applied edit

**File:** `bench/runtime/aider_edit_agent.go:122-134`
**Issue:**
The live apply seam passes the entire current stub body as the literal `pattern`
and only fails on `callErr != nil` or `callRes.IsError`. But `replace_in_file`
returns a **non-error success** (`textResult("0 replacement(s) made in …")`,
`IsError == false`) when the literal pattern matches zero times AND the fuzzy
fallback is disabled (`k.StructuredEditDisabled()` true — the ABLATE-07 path) or
the file is empty. In that case the seam returns `nil`, the shared core sets
`*applied = true`, and the row records `edit_format_applied: true` even though
**no edit was applied** — the stub still holds the original panic body.

For the committed `go/wordy` baseline this cannot fire (fuzzy fallback is on,
the stub is non-empty, and the whole-file pattern matches exactly once), so the
happy-path verification did not exercise it. It becomes a real false-green the
moment this arm is run under the structured-edit ablation, against an
empty/whitespace-only stub, or if a future fixture's stub content is normalized
(e.g. CRLF/trailing-newline drift) so the verbatim `pattern` fails to match and
fuzzy is unavailable. `edit_format_applied` is load-bearing precisely for the
"could NOT apply" verdict, so a false `true` here is exactly the signal the key
exists to protect.

**Fix:** Have the apply seam assert that at least one replacement was made rather
than trusting `!IsError`. The tool's text result reports the count
(`"N replacement(s) made in …"`); parse it (or have the seam re-read the stub and
compare against `body`) and fail closed when the post-edit content does not equal
the reference body:

```go
if callRes != nil && callRes.IsError {
    return fmt.Errorf("replace_in_file returned error: %s", toolErrText(callRes))
}
// Fail closed: confirm the edit actually landed (replace_in_file reports a
// non-error "0 replacement(s) made" when the literal pattern did not match).
post, rerr := os.ReadFile(filepath.Join(workDir, stub))
if rerr != nil {
    return fmt.Errorf("verify applied stub %q: %w", stub, rerr)
}
if string(post) != body {
    return fmt.Errorf("replace_in_file applied no edit to %q (stub unchanged)", stub)
}
```

### WR-03: `runAiderEditCell` ignores `cfg.Benchmark`, hardcoding `aiderEditBenchmark`

**File:** `bench/runtime/aider_edit_cell.go:127` (and the `runAiderEditCell` entry)
**Issue:**
`RunCell` validates `cfg.Benchmark` against path traversal and the rest of the
spine carries it through to the result row (`cell.go:783` uses `cfg.Benchmark`).
The aider-edit branch instead stamps the const `aiderEditBenchmark =
"aider-polyglot"` into `BuildResult` and never reads `cfg.Benchmark`. The
regenerator does set `Benchmark: "aider-polyglot"` so they agree today, but a
caller that passes a different `cfg.Benchmark` would get a row whose `benchmark`
field silently disagrees with the requested config — an undetected provenance
mismatch. The other real arms (`runRAGCell`, the daemon spine) honor
`cfg.Benchmark`.

**Fix:** Either use `cfg.Benchmark` (with a guard that it equals the expected
suite) or assert equality and fail closed when a caller passes a conflicting
benchmark:

```go
if cfg.Benchmark != "" && cfg.Benchmark != aiderEditBenchmark {
    return preserve(fmt.Errorf("bench/runtime: aider_edit cell requires benchmark %q, got %q",
        aiderEditBenchmark, cfg.Benchmark))
}
```

## Info

### IN-01: stub read for `pattern` uses local FS while the edit uses the daemon-resolved path — two sources of truth

**File:** `bench/runtime/aider_edit_agent.go:116` vs `122-127`
**Issue:** The apply seam reads the current stub with `os.ReadFile(filepath.Join(workDir, stub))`
(local filesystem) to build the `pattern`, but `replace_in_file` resolves
`"path": stub` relative to the daemon's `activate_project` workspace. These agree
today because the daemon was activated at `workDir`, but the coupling is
implicit. A drift between the harness CWD-relative read and the daemon-relative
write would produce a `pattern` that does not match the daemon's view of the
file, surfacing as the WR-02 silent no-op. A one-line comment asserting the
invariant (or routing the read through a `read_file` tool call over the same
session) would make the coupling explicit.

### IN-02: `len(exm) < len(sol)` guard permits index misalignment when example list is longer

**File:** `bench/runtime/aider_edit_agent.go:52-55, 58`
**Issue:** `newEditAgentWithApply` guards `len(exm) < len(sol)` and then indexes
`exm[i]` for each solution `i`. This is positionally correct only if
`Config.Files.Solution[i]` and `Config.Files.Example[i]` are meant to be paired
by index. The aider config.json does pair them this way for the current
fixtures, but nothing validates the pairing semantics (e.g. a config with 1
solution and 3 unrelated examples passes the guard and silently maps
`solution[0] -> example[0]`). This is acceptable for the pinned fixtures but
worth a comment that index-pairing is the dataset contract, not a checked
invariant.

### IN-03: comment at `aider_edit_cell.go:101` mislabels `RepoDir` as the live tree

**File:** `bench/runtime/aider_edit_cell.go:92-100`
**Issue:** The Pitfall-3 comment says the patch_validator metrics "are computed
from a LIVE git working tree (RepoDir)" — but `RepoDir` is `""` (see WR-01), so
git actually runs against the regenerator's CWD, not a per-cell repo dir. The
comment should be corrected to reflect that the grader is invoked with an empty
RepoDir (and that the values are discarded) so a future maintainer does not
assume a meaningful RepoDir was threaded.

### IN-04: empty/regex-disabled `WallTimeSeconds` and token errors flow into the deterministic row unverified

**File:** `bench/runtime/aider_edit_cell.go:83-90`
**Issue:** `assembleAiderEditResult` passes `Merged: trace.MergedTrace{}` and
`UsagePresent: false` into `Grade`, so `tool_trace_analyzer.Analyze` and
`token_meter.MeterTokens` run over zero-value inputs. These are deterministic
today (all-zero trace → zero/nil metrics + a constant usage-absent MetricError),
but the determinism rests on those graders being pure over the empty input —
a property the phase's double-render test confirms only for the current grader
set. Worth a one-line note that the deterministic guarantee assumes
`Analyze(MergedTrace{})` and `MeterTokens(MergedTrace{}, false)` remain
input-pure, since a future grader that consulted the clock or environment would
silently break the baseline.

---

_Reviewed: 2026-06-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
