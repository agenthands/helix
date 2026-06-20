---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
reviewed: 2026-06-20T00:00:00Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - bench/runtime/cell.go
  - bench/runtime/cell_test.go
  - bench/runtime/no_semantic_zero_reads_test.go
  - bench/runtime/five_of_six_test.go
  - internal/cli/root.go
  - internal/daemon/daemon.go
  - internal/daemon/daemon_test_export_test.go
  - internal/daemon/semantic_gate.go
  - internal/daemon/semantic_gate_test.go
  - internal/daemon/semantic_wiring.go
  - internal/daemon/shutdown.go
  - internal/lint/ablationleakage/analyzer.go
  - internal/lint/ablationleakage/analyzer_test.go
  - internal/obs/metrics.go
  - internal/obs/metrics_labels_test.go
  - internal/profile/bench_profiles_test.go
  - internal/profile/profile.go
  - internal/semantic/config.go
  - internal/semantic/store/effective_graph.go
  - internal/semantic/store/filefact_accessor.go
  - internal/semantic/store/overlay.go
  - internal/semantic/store/reads_counter_test.go
  - internal/semantic/store/snapshot.go
findings:
  critical: 1
  warning: 4
  info: 4
  total: 9
status: issues_found
---

# Phase 81: Code Review Report

**Reviewed:** 2026-06-20
**Depth:** standard
**Files Reviewed:** 24 (23 in scope + daemon.go composition-root + 2 supporting store files)
**Status:** issues_found

## Summary

Phase 81 wires a labelless `helix_semantic_store_reads_total` counter at a single
DuckDB read chokepoint (`s.queryContext` / `s.queryRowContext`), resolves an
`effSemanticDisabled` ablation gate once at the daemon composition root, forces
the tool-facing semantic read consumers to `NoopLookup{}` + a disabled
`ConfigGate` (build-but-block), extends the `ablationleakage` analyzer with a
call-site gate, and adds a bench cell that asserts the counter == 0 on the
`no_semantic` arm.

The counter plumbing, metrics allowlist, gate-helper unit tests, and the lint
analyzer are well constructed. The central correctness concern is a **scope gap
between what the gate disables and what the counter counts**: the gate is applied
only to the tool-facing read consumers (the `SetSemanticLookup` hand-outs), but
several daemon-internal background pipelines that survive the build-but-block
("store stays built") path ALSO route reads through the counted chokepoint and
are NOT gated. On a store-on (`StoreOptIn=true`) `no_semantic` cell this can make
the counter non-zero through a legitimate, non-tool read path, which the bench
cell would (correctly, per its own contract) treat as a hard failure — i.e. the
"zero reads" invariant can be violated by code the phase did not gate. The
currently-shipped five-of-six smoke uses a store-OFF seed task, so the gap is
latent today but is a real defect for the phase's stated guarantee.

## Critical Issues

### CR-01: Build-but-block leaves daemon-internal read pipelines ungated; counter can be non-zero on a store-on `no_semantic` cell

**File:** `internal/daemon/daemon.go:948-1003` (also `:967-975`, `:979`, `:1001-1003`); read sites `internal/daemon/semantic_wiring.go:381-423` (`ensureRetrieval` → recovery `Probe`), `internal/semantic/store/effective_graph.go:413` (`LatestCommittedSnapshot`)

**Issue:**
The gate (`effSemanticDisabled`) is threaded into exactly the four tool-facing
`SemanticLookup` hand-outs (symbols, repomap, health, guardrail) plus the
`SemanticSkill` accessor block. Per D-04 build-but-block, the store IS opened
(`daemon.go:324`) and `sBndl` is non-nil even under the gate. But
`SetActivateCallback` unconditionally drives daemon-internal pipelines that issue
reads through the **counted** chokepoint, with no `effSemanticDisabled` guard:

- `daemon.go:1001-1003` → `sBndl.ensureRetrieval(...)` → `retrieval.Recoverer.Probe`
  → `store.LatestCommittedSnapshot` → `s.queryRowContext` → `SemanticStoreReadsInc()`.
  (`Probe` calls `LatestCommittedSnapshot` on every workspace activation; verified
  in `internal/semantic/retrieval/recovery.go:141-149`.)
- `daemon.go:979` → `live.startWorkspace(...)`; the live handler holds
  `SetFileFactStore(semanticStore)` (`daemon.go:425`), so an edit-driven
  `GetLatestFileFact` (`filefact_accessor.go:75` → `s.queryRowContext` /
  `s.queryContext`) is a counted read.
- `daemon.go:967-975` → `ScheduleInitialExtraction` → rank scheduler →
  `QueryEffectiveAdjacency` / `CountStaleScoreRows` (`effective_graph.go:113`,
  `:183`), both counted reads.

`StoreOptIn` is driven by the seed task's capability (`matrix.go:258`), **not** by
mode, so a `no_semantic` cell on a store-on task (e.g. an `incremental_update`
task) runs with `semantic_index.enabled=true`. The store opens, the workspace
activates, and the above background reads fire — incrementing
`helix_semantic_store_reads_total` even though every *tool* lookup is correctly
forced to Noop. `bench/runtime/cell.go:560` (`assertNoSemanticReads`) then fails
the cell hard, mislabeling a correctly-gated tool surface as a gate breach
(T-81-05-01 false positive) — OR, read the other way, the phase's "zero
semantic-store reads on the no_semantic arm" guarantee is simply not true for
store-on tasks.

The five-of-six smoke passes only because its seed (`IT-go-patch-apply-1`) is
store-OFF (`semantic_index.enabled=false` → store never opened → chokepoint never
reached). The defect is therefore latent against the current corpus but is a real
correctness gap for the stated invariant.

**Fix:** Either (a) gate the background read pipelines under `effSemanticDisabled`
the same way the tool consumers are gated, so build-but-block blocks ALL reads,
not just tool reads:

```go
// daemon.go SetActivateCallback
if semanticScheduler != nil && !effSemanticDisabled {
    semanticScheduler.ScheduleInitialExtraction(...)
}
if !effSemanticDisabled {
    live.startWorkspace(ctx, activeWSKey, logger)
}
if rank != nil && !effSemanticDisabled {
    rank.ensureScheduler(ctx, repoPath)
}
if compactBndl != nil && !effSemanticDisabled {
    compactBndl.ensureCompactor(ctx, repoPath, activeWSKey)
}
if sBndl != nil && !effSemanticDisabled {
    sBndl.ensureRetrieval(ctx, activeWSKey)
}
```

or (b) if these pipelines are intentionally allowed to read under the gate
(build-but-block must keep maintaining the store), then the counter / assertion
contract is wrong: the bench cell cannot assert `== 0` against a counter that
legitimately moves. In that case scope the counter to *tool-path* reads (or
assert against a tool-read-only sub-counter) so the invariant matches reality.
Pick one; the current code commits to neither and the two halves contradict on a
store-on arm. At minimum, add a store-ON `no_semantic` cell to the smoke matrix
so this path is actually exercised before the guarantee is claimed.

## Warnings

### WR-01: `SemanticStoreReadsValue()` reports daemon-lifetime total, not per-cell — multi-workspace / warm-daemon reuse inflates the assertion

**File:** `internal/obs/metrics.go:766-778`, `internal/daemon/shutdown.go:60-62`

**Issue:** The counter is a process-global monotone total emitted once at daemon
shutdown. The bench spawns a fresh per-cell daemon (`cell.go:453`), so today the
total ≈ per-cell, but the assertion `== 0` is exact and brittle: any future reuse
of a daemon across cells, any pre-activation warm read, or any daemon-level
health/readiness probe that touches the store would make the total non-zero for
reasons unrelated to the cell under test. The "exact zero" assertion has no
baseline subtraction.

**Fix:** Snapshot the counter at cell start (post-daemon-boot, pre-drive) and
assert `after - before == 0`, or document and enforce the one-daemon-per-cell
invariant the exact-zero assertion silently depends on.

### WR-02: `scrapeSemanticReadsTotal` treats a missing reads-total line as 0 reads — a daemon that crashes before shutdown silently passes the gate

**File:** `bench/runtime/cell.go:77-109` (and the asserted behavior in
`no_semantic_zero_reads_test.go:91-101`)

**Issue:** If the daemon is `SIGKILL`ed (`cell.go:535` `h.Kill()`) before it runs
its graceful `shutdown()` (which emits the reads-total line at
`shutdown.go:60-62`), no line is written and the scraper returns `(0, nil)` —
"treated as zero reads, not an error." But `RunCell` kills the daemon with
`h.Kill()` and only THEN taps the log; if `Kill()` is SIGKILL rather than a
graceful signal, the shutdown hook never runs and the reads-total line is never
emitted, so the zero-reads gate is vacuously satisfied for EVERY run — it can
never observe a real non-zero count. The gate's teeth depend entirely on
`h.Kill()` delivering a signal that triggers graceful shutdown.

**Fix:** Verify `subprocess.Handle.Kill()` sends a graceful signal (SIGTERM) and
waits for the graceful-shutdown path, not SIGKILL. If termination is SIGKILL,
the read-counter line is never written and the entire `assertNoSemanticReads`
gate is dead. Add a positive test that a real (non-synthetic) daemon run emits
the line, or assert the line's presence (not just its value) on the
`no_semantic` arm so a missing line is a failure, not a silent pass.

### WR-03: `ValidateCriticalEdges` is in the analyzer's `semanticReadMethods` but is a documented no-read passthrough — name-based gate over-broad

**File:** `internal/lint/ablationleakage/analyzer.go:57-61`; method at
`internal/daemon/semantic_wiring.go:1266-1275`

**Issue:** `semanticReadMethods` flags `ValidateCriticalEdges` as a "data-bearing
semantic read," but the production implementation is a permanent passthrough that
issues no store traffic (`semantic_wiring.go:1251-1275`: "returns the input edges
with LSPConfirmed=false, without issuing any LSP traffic"). Conversely, genuine
counted reads like `RankFromSeeds`, `LocateSymbol`, `SymbolID`, and `ExpandFrom`'s
deeper helpers are NOT in the set. The analyzer's call-site gate is a coarse
name-match approximation (acknowledged in the doc comment), but the chosen set
both over-includes a no-op and under-includes real reads — so the static gate
gives a false sense of coverage relative to the runtime counter (CR-01).

**Fix:** Align the flagged set with the methods that actually reach the counted
chokepoint, or document explicitly (in the analyzer doc) that the set is a
representative tripwire, not an exhaustive read inventory, and that the runtime
counter (Plan 05) is the authoritative coverage mechanism.

### WR-04: `gatedCfgGate` silently ignores `effSemanticDisabled` precedence vs `cfg.SemanticIndex.Enabled` only via AND — no guard if a future caller passes a nil cfg with the gate off

**File:** `internal/daemon/semantic_gate.go:58-61`

**Issue:** `gatedCfgGate` computes `enabled := cfg != nil && cfg.SemanticIndex.Enabled && !effSemanticDisabled`.
This is correct for the two intended states, but the `gatedSymbolsLookupFn` /
`gatedCfgGate` pair encode the same `effSemanticDisabled` decision in two places
(lookup forcing in one helper, gate disabling in the other). If a future edit
changes one and not the other, the lookup and the gate can disagree
(Noop-lookup + enabled-gate, or real-lookup + disabled-gate), and
`integ.ChooseSource`'s ladder would pick an inconsistent source. The unit test
(`semantic_gate_test.go`) covers the matched pairs but not the mismatched ones.

**Fix:** Consider folding both decisions behind a single `gatedSemanticWiring(cfg,
effSemanticDisabled)` constructor returning the matched `(lookupFn, cfgGate)` pair
so they cannot drift, or add an explicit invariant assertion that a Noop lookup
always accompanies a disabled gate at each call site.

## Info

### IN-01: Stale "1 partial row" comment after the no_semantic arm stopped being partial

**File:** `bench/runtime/five_of_six_test.go:131`

**Issue:** The comment "Exactly 4 real rows + 1 partial row = 5 rows on disk" and
the assertion message "4 real-with-deltas + 1 no_semantic row" describe the
no_semantic row as "partial," but Phase 81's whole point is that this row is now a
clean (non-partial) measurement (the deferral marker was removed). The body and
the rest of the file are correct; only the "partial" wording is stale.

**Fix:** Replace "1 partial row" with "1 clean no_semantic row" to match the
post-Phase-81 semantics asserted two blocks above.

### IN-02: `SemanticStoreReadsValue` float→int truncation documented as exact but uses `int(*float64)` with no overflow note for 32-bit builds

**File:** `internal/obs/metrics.go:766-778`

**Issue:** The doc says "a counter only ever increments, so the float-to-int
truncation is exact for any count the process can reach." On a 32-bit platform
`int` is 32-bit; a count > 2^31 would wrap. Practically unreachable for a
short-lived bench daemon, but the "exact for any count" claim is stronger than
the type guarantees.

**Fix:** Either narrow the comment ("exact for any count a bench daemon reaches")
or return `int64` and have the scraper read `int64`.

### IN-03: Two-place duplication of the gate decision string in logs/comments increases drift risk (doc-only)

**File:** `internal/daemon/semantic_wiring.go:262-317`

**Issue:** The gated/non-gated branches list all 16 `Set*Accessor(nil)` calls
twice (once real, once nil-reset). This is intentional (Pitfall 5 idempotent
reset for the process-global singleton) but is a long literal duplication; adding
a 17th accessor requires editing both arms and the `"setters", 14` log count,
which is already out of sync (16 setters are listed but the log says 14, with two
deferred — TypeChain/EdgeEvidence — explaining the delta). The count literal is a
maintenance trap.

**Fix:** Derive the setter count or drop the numeric literal; at minimum add a
test asserting the two arms cover the same setter set so they cannot drift.

### IN-04: `assertNoSemanticReads` couples to the mode-name string constant `your_agent_no_semantic` in three files

**File:** `bench/runtime/cell.go:61`, `bench/runtime/no_semantic_zero_reads_test.go:24`,
`internal/profile/bench_profiles_test.go:145` (profile name `bench-no-semantic`)

**Issue:** The runtime gate keys off the bench *mode* name (`your_agent_no_semantic`)
while the profile that actually sets `DisableSemanticSubsystem` is `bench-no-semantic`;
the mode→profile mapping lives in the MODE.md resolver. If the mode is renamed or
the resolver maps `your_agent_no_semantic` to a different profile, the runtime
zero-reads gate and the profile's disable flag would silently decouple (the gate
would assert against a daemon that never set the flag, or vice versa).

**Fix:** Add an integration assertion that the `your_agent_no_semantic` mode
resolves to a profile with `DisableSemanticSubsystem==true`, pinning the
mode↔flag coupling the gate depends on.

---

_Reviewed: 2026-06-20_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
