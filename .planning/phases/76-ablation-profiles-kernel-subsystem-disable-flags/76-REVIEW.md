---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
reviewed: 2026-06-16T00:00:00Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - cmd/vet-ablation-leakage/main.go
  - internal/cli/root.go
  - internal/config/config.go
  - internal/daemon/daemon.go
  - internal/daemon/daemon_test_export_test.go
  - internal/daemon/no_lsp_wiring_test.go
  - internal/errors/kinds.go
  - internal/kernel/kernel.go
  - internal/kernel/kernel_disable_flags_test.go
  - internal/kernel/edit/tools.go
  - internal/kernel/edit/structured_edit_disabled_test.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/fileops/replace_in_file_disabled_test.go
  - internal/lint/ablationleakage/analyzer.go
  - internal/lint/ablationleakage/analyzer_test.go
  - internal/profile/profile.go
  - internal/profile/loader.go
  - internal/profile/loader_test.go
  - internal/profile/bench_profiles_test.go
  - internal/profile/profiles/bench-full.yaml
  - internal/profile/profiles/bench-no-lsp.yaml
  - internal/profile/profiles/bench-no-semantic.yaml
  - internal/profile/profiles/bench-no-structured-edit.yaml
  - internal/skill/repomap/skill.go
  - Makefile
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 76: Code Review Report

**Reviewed:** 2026-06-16
**Depth:** standard
**Files Reviewed:** 24
**Status:** issues_found

## Summary

Phase 76 adds ablation infrastructure: two kernel subsystem-disable flags
(`DisableLSPSubsystem` / `DisableStructuredEditSubsystem`), composition-root
threading with CLI-OR-profile precedence, `subsystem_disabled:`-prefixed
runtime guards on the four structured-edit handlers plus the `replace_in_file`
exact-match fall-through, the no_lsp null-object injection (skip `buildLiveBundle`,
clear `SetEnrichFn`/`SetFallbackDeps`, neutralize the diag `leaseFn`), four bench
profile YAMLs, and the `vet-ablation-leakage` import-boundary analyzer.

The core wiring is sound. The kernel flags default to disabled-OFF correctly,
the accessors are nil-safe (read only `k.config`), the null-object clears on the
process-global RepoMapSkill singleton are idempotent (`SetEnrichFn(nil)` /
`SetFallbackDeps(nil)`), and the `live.startWorkspace` call in
`SetActivateCallback` is nil-receiver-safe (`live_wiring.go:128`) so the
`live == nil` arm under `effDisableLSP` cannot panic. The analyzer's
slash-boundary match form is correct and the lookalike regression is covered.

Three warnings: (1) the new `outcome="unsupported"` value emitted by the four
structured-edit guards is NOT in the closed `editOutcomeEnum`, so the
`helix_edit_outcome_total` metric silently drops every ablation-disabled call;
(2) the no_lsp arm has a defense-in-depth asymmetry — symbol-retrieval and
`rename_symbol` acquire live LS leases with NO kernel runtime guard, relying
solely on profile tool-filtering, while the structured-edit arm has both;
(3) `ProfileStore.Validate()` runs only inside `LoadEmbedded()`, never after
`LoadOverrides()`, so a user/project override that introduces an unknown
`default_mode` bypasses the ABLATE-02 fail-closed gate.

## Structural Findings (fallow)

No `<structural_findings>` block was provided with this review; none to report.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: `outcome="unsupported"` is dropped by the closed edit-outcome enum — ablation-disabled calls emit zero metrics

**File:** `internal/kernel/edit/tools.go:335,435,513` and `internal/kernel/fileops/tools.go:507`
**Issue:** All four structured-edit ablation guards set `outcome = "unsupported"`
before returning the `serr.Unsupported` error, e.g.:

```go
if k.StructuredEditDisabled() {
    outcome = "unsupported"
    return errorResult(serr.New(serr.Unsupported, "subsystem_disabled: ...")...), nil, nil
}
```

The deferred `mcp.RecordEditOutcome(ctx, <tool>, outcome, strategy)` ultimately
calls `Metrics.EditOutcomeInc`, whose closed enum is
`{"success","no_match","ambiguous_match","validation_failed","ls_error","internal"}`
(`internal/obs/metrics.go:683`). `"unsupported"` falls into the `default:`
branch and is **dropped** (`metrics.go:684-685` `return`). The middleware-side
`editOutcomeEnum` (`internal/mcp/middleware.go:226-233`) likewise lists only the
same six values. Result: every call to a disabled structured-edit tool under the
no_structured_edit ablation arm records NOTHING on `helix_edit_outcome_total` —
operators measuring the arm get no signal on how often agents attempt the
disabled tools, which is exactly the kind of behavior an ablation harness exists
to measure.

**Fix:** Either (a) add `"unsupported"` to the closed enum in both
`internal/mcp/middleware.go` (`editOutcomeEnum`) and
`internal/obs/metrics.go` (`EditOutcomeInc` switch), updating the cardinality
bound and the `TestMetrics_CardinalityBounds_EditOutcome` assertion noted in the
`ClassifyEditError` TODO; or (b) if a new enum value is undesirable this phase,
set `outcome = "internal"` (the documented catch-all) so the call is at least
counted:

```go
if k.StructuredEditDisabled() {
    outcome = "internal" // counted; "unsupported" is dropped by the closed enum
    return errorResult(serr.New(serr.Unsupported, "subsystem_disabled: ...")...), nil, nil
}
```

Option (a) is preferable because it preserves the distinction; option (b) is the
minimal correctness fix that stops the silent drop.

### WR-02: no_lsp arm has no kernel runtime guard on LS-leasing symbol/rename tools — zero-span guarantee rests solely on profile filtering

**File:** `internal/daemon/daemon.go:287-301` (flag wiring), `internal/kernel/symbols/tools.go:353-635` (ungated `AcquireSession`), `internal/kernel/edit/tools.go:594` (`rename_symbol` ungated `AcquireSession`)
**Issue:** Under `effDisableLSP`, the daemon neutralizes three back-channel seams
(live `EditNotifier`, repomap `SetEnrichFn`/`SetFallbackDeps`, diag `leaseFn`) so
no implicit LSP traffic is reachable. But the **direct** LSP tools —
`go_to_definition`, `find_references`, hover, implementations, call/type
hierarchy, blast radius (all `symbols/tools.go`), and `rename_symbol`
(`edit/tools.go:594`) — call `rt.AcquireSession(ctx, ...)` which leases a live LS
worker, with **no** `k.LSPSubsystemDisabled()` runtime guard at the handler.
Contrast the structured-edit arm, which has BOTH a profile exclusion AND a
kernel `StructuredEditDisabled()` backstop (`edit/tools.go:334`,
`fileops/tools.go:506`). The no_lsp arm's "zero `lspool.lsp.*` spans" guarantee
therefore depends entirely on the `ProfileFilterMiddleware` keeping these tools
out of `tools/list`; a back-channel `tools/call` for `go_to_definition` or
`rename_symbol` (e.g. a misbehaving client that calls a tool not in the list, or
a future code path) would acquire a live worker and emit `lspool.lsp.*` spans,
silently violating ROADMAP success criterion #2.

`TestNoLSPZeroSpans` does not catch this: it only inspects spans emitted at
daemon *construction* time (it never issues a `tools/call`), so it proves the
wiring is quiet at boot, not that the arm is closed under tool invocation.

This is partly a documented scope decision (D-09 enumerates exactly the
back-channel seams to neutralize; the kernel `disable_lsp_subsystem` *tool*
guard analogous to ABLATE-07 is not in this phase). Flagging it as a warning
because the asymmetry is a real robustness gap and the test gives false
confidence that the arm is structurally closed.

**Fix:** Add a `k.LSPSubsystemDisabled()` runtime backstop to the LS-leasing
symbol/rename handlers (mirroring the `StructuredEditDisabled()` guard pattern),
returning `serr.Unsupported` with the `subsystem_disabled:` prefix before
`AcquireSession`. At minimum, document explicitly (in `no_lsp_wiring_test.go` or
the phase SUMMARY) that the no_lsp closure relies on profile filtering for the
direct LSP tool surface and add a test that issues a `tools/call` for an
LS-leasing tool under the flag and asserts no `lspool.lsp.*` span.

### WR-03: `ProfileStore.Validate()` is not re-run after `LoadOverrides`, so a disk override can reintroduce the unknown-mode hole ABLATE-02 closes

**File:** `internal/profile/loader.go:28` (Validate called in `LoadEmbedded`), `internal/profile/loader.go:70-78` (`LoadOverrides` has no Validate), `internal/config/loader.go:89-91` (override path)
**Issue:** ABLATE-02 (T-76-03) fail-closes on a profile whose `default_mode` /
transitions reference an unknown mode, because a missed `store.Mode(name)` makes
`resolveAllowedToolsForMode` return `nil` (no allow-list = unfiltered tool
surface), defeating an ablation arm. `Validate()` enforces this — but only inside
`LoadEmbedded()` (`loader.go:28`). `ResolveProfile` then calls
`profile.LoadOverrides(...)` (`config/loader.go:90`) which merges YAML from
`~/.helix/profiles` and `~/.helix/modes` **without** re-validating. A user/project
override that sets `default_mode: typo` (or adds a transition to a non-existent
mode), or that introduces a brand-new profile via override, slips past the
fail-closed gate and silently produces a nil allow-list at session start — the
exact tampering ABLATE-02 was added to prevent. The override error is also
swallowed (`_ = profile.LoadOverrides(...)`, `config/loader.go:90`), so a
malformed override file is invisible.

**Fix:** Re-run `store.Validate()` after `LoadOverrides` in `ResolveProfile`
(or inside `LoadOverrides` itself), and surface the result rather than discarding
it:

```go
if globalDir != "" {
    if err := profile.LoadOverrides(store, profilesDir, modesDir); err != nil {
        return nil, nil, fmt.Errorf("loading profile overrides: %w", err)
    }
    if err := store.Validate(); err != nil {
        return nil, nil, fmt.Errorf("validating profiles after overrides: %w", err)
    }
}
```

## Info

### IN-01: `DefaultProfile()` uses a redundant blank assignment

**File:** `internal/profile/profile.go:127`
**Issue:** `p, _ := s.profiles["full"]` — the comma-ok blank is redundant for a
map read; `p := s.profiles["full"]` returns the zero value (nil) on a miss
identically. Minor style nit, not a bug. (Pre-existing; surfaced while tracing
the ABLATE-02 fallback path.)
**Fix:** `p := s.profiles["full"]`.

### IN-02: `activeWSLang` capture in diag `leaseFn` is dead under the no_lsp arm but reads a racy closure variable on the live arm

**File:** `internal/daemon/daemon.go:648-654`
**Issue:** On the enabled arm the diag `leaseFn` closes over `activeWSKey` /
`activeWSLang`, which are plain (non-atomic) locals mutated by
`lazyActivateFn` / `SetActivateCallback` from middleware goroutines and read here
from a tool-call goroutine. This is a pre-existing pattern shared by the repomap
enrich/fallback closures (the CR-03 comment at daemon.go:824-828 acknowledges the
by-reference capture is intentional and LazyInit-ordered), so it is not new to
this phase and not independently actionable here. Noting it only because the
Phase 76 `leaseFn` refactor (splitting into the `effDisableLSP` stub vs. the live
closure) touches these lines; the stub arm (lines 644-646) correctly ignores both
variables.
**Fix:** None required this phase; if the daemon ever moves to multi-workspace,
promote `activeWSKey`/`activeWSLang` to a mutex-guarded or atomic holder.

### IN-03: `vet-ablation-leakage` is proven only by testdata fixtures; no real consumer links it until Phase 80

**File:** `internal/lint/ablationleakage/analyzer.go:24-26`, `Makefile:60-61`
**Issue:** The analyzer correctly restricts its scan to the
`github.com/agenthands/helix/bench/runners` namespace (`analyzer.go:52`), which
does not yet exist in the tree — so `make vet`'s `-vettool=vet-ablation-leakage`
pass is a no-op against production packages today and the green→red flip is
demonstrated entirely by the `analysistest` fixtures. This is an explicitly
documented decision (analyzer.go:24-26), correct for this phase, and the
slash-boundary lookalike regression (`analyzer_test.go:32`) is covered. Recording
it so a future reader does not assume the boundary is enforced against live code
before Phase 80 lands the real runners.
**Fix:** None; ensure Phase 80 adds at least one real `bench/runners/*` package so
the analyzer guards production code, not just fixtures.

---

_Reviewed: 2026-06-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
