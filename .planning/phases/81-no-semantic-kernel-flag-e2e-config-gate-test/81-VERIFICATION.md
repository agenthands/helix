---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
verified: 2026-06-20T16:30:00Z
status: gaps_found
score: 3/4 must-haves verified
overrides_applied: 0
gaps:
  - truth: "Criterion #2 — the bench cell asserts helix_semantic_store_reads_total == 0 after a no_semantic run and LOGS + FAILS the cell on any violation (independent RUNTIME verification, ABLATE-06 acceptance: 'runtime assertion logs and fails')"
    status: failed
    reason: >
      The runtime assertion is VACUOUS end-to-end on a real bench run (CONFIRMS review WR-02).
      The counter value is emitted ONLY in the daemon's graceful shutdown() path
      (internal/daemon/shutdown.go:60-62), which runs only after g.Wait() returns
      (daemon.go:1276) — i.e. on SIGTERM/SIGINT (daemon.go:1179) or an errgroup error.
      But the bench tears the daemon down via DaemonHandle.Kill() (cell.go:535), which sends
      SIGKILL to the process group (sandbox.go:203-204). SIGKILL is untrappable, so shutdown()
      NEVER runs on the bench path and the "semantic store reads total" line is NEVER written to
      daemon.log. scrapeSemanticReadsTotal then returns 0 for the missing line (cell.go:101-103,
      default count=0; behavior codified by test "scrape_missing_line_yields_zero"), so
      assertNoSemanticReads(noSemanticMode, 0) ALWAYS passes. The gate can never observe a real
      read on a real bench run. The only test exercising the path (no_semantic_zero_reads_test.go)
      feeds SYNTHETIC log bodies via os.WriteFile — there is NO test that drives a real daemon
      through Kill() and asserts the line is actually emitted.
    artifacts:
      - path: internal/daemon/shutdown.go
        issue: "Counter line emitted at lines 60-62 inside d.shutdown(), reachable only via SIGTERM/SIGINT graceful path (daemon.go:1276)"
      - path: bench/runtime/cell.go
        issue: "RunCell:535 tears down via h.Kill() (SIGKILL) with no graceful stop first; scraper:101-103 treats missing line as count=0"
      - path: internal/eval/sandbox/sandbox.go
        issue: "DaemonHandle.Kill() (lines 193-228) sends syscall.Kill(-pid, SIGKILL) + cmd.Process.Kill() — untrappable, shutdown() never runs"
      - path: bench/runtime/no_semantic_zero_reads_test.go
        issue: "All cases use synthetic os.WriteFile log bodies; no real-daemon-through-Kill() emission test exists"
    missing:
      - "Either tear the bench daemon down gracefully (send SIGTERM and wait for shutdown() before Kill) so the counter line is actually emitted on a real run"
      - "OR emit the counter line on a path SIGKILL cannot bypass (e.g. periodic flush, or assert the LINE'S PRESENCE not just its value so a missing line is a hard failure, not a silent pass)"
      - "A real-daemon integration test that drives the no_semantic arm through the actual teardown path and asserts the reads-total line is present in daemon.log"
deferred: []
---

# Phase 81: no_semantic Kernel-Flag Config Gate + E2E Config-Gate Test — Verification Report

**Phase Goal:** Land the `no_semantic` kernel-flag config gate + an E2E config-gate test that proves the `no_semantic` ablation arm makes ZERO back-channel reads against the DuckDB semantic store (ABLATE-06).
**Verified:** 2026-06-20T16:30:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (phase criteria)

| # | Truth (criterion) | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Resolve `effSemanticDisabled` once at the composition root; un-wire the 8 back-channel read consumers → NoopLookup + disabled ConfigGate, store stays BUILT (D-04) | ⚠️ PARTIAL (verified for tool-facing consumers; ungated daemon-internal pipelines — WR-02/CR-01) | `internal/daemon/semantic_gate.go:33,43,58` (resolveSemanticDisabled / gatedSymbolsLookupFn / gatedCfgGate); `daemon.go` threads it into symbols, health, repomap, guardrail + `semantic_wiring.go:262` accessor block. Bundle-build guards (daemon.go:324/518) untouched (D-04). `TestEffSemanticDisabled/TestSemanticGateForcesNoop/TestSemanticGateChoosesTreeSitter/TestSemanticSkillAccessorsGated` PASS. **BUT** SetActivateCallback pipelines (daemon.go:967-1003) + `SetFileFactStore` (daemon.go:425) reach the counted chokepoint UNGATED — see CR-01 below. |
| 2 | Independent RUNTIME verification: bench cell asserts `helix_semantic_store_reads_total == 0` after a no_semantic run; LOGS + FAILS on violation | ✗ FAILED (vacuous end-to-end) | Assertion code present (cell.go:119-130, 555-565) and unit-passes on synthetic input, but the counter LINE is never emitted on a real bench run because teardown is SIGKILL (no graceful shutdown()). Gate always reads 0. **WR-02 CONFIRMED.** |
| 3 | Extend the `vet-ablation-leakage` analyzer to a call-site gate check | ✓ VERIFIED (with WR-03 precision caveat) | `internal/lint/ablationleakage/analyzer.go:47-68,123-208` flags direct `ExpandFrom/RankFiles/ValidateCriticalEdges` calls outside the allowlist not routed through `ChooseSource`. `go test ./internal/lint/ablationleakage/` PASS (badgate flagged, goodgate silent); `make vet` exit 0 on real tree. WR-03: `ValidateCriticalEdges` is a no-read passthrough yet flagged (over-broad tripwire, not a coverage breach). |
| 4 | Rewrite `your_agent_no_semantic/MODE.md` (gate key + 8-consumer strangler-fig enumeration) | ✓ VERIFIED | All 8 consumers + `semantic_index.bench_disabled` present; `guarantee_pending_phase_81` absent (count 0). Frontmatter `mode: no_semantic` / `profile: bench-no-semantic`. |

**Score:** 3/4 truths verified (criterion #1 PARTIAL counted as a WARNING, criterion #2 FAILED as a BLOCKER).

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/semantic/config.go` | `BenchDisabled bool koanf:"bench_disabled"` (distinct gate, D-01) | ✓ VERIFIED | Line 44, doc-comment states distinct/build-but-block. |
| `internal/profile/profile.go` | `DisableSemanticSubsystem bool yaml:"disable_semantic_subsystem"` | ✓ VERIFIED | Line 60. |
| `internal/cli/root.go` | `--disable-semantic-subsystem` only-when-set → `semantic_index.bench_disabled` | ✓ VERIFIED | Lines 82, 159, 193 (nested override). |
| `internal/profile/profiles/bench-no-semantic.yaml` | `disable_semantic_subsystem: true` | ✓ VERIFIED | Line 62. (Minor: line 15 carries a stale "Tool-filter-only this phase" comment — IN, doc-only.) |
| `internal/obs/metrics.go` | `helix_semantic_store_reads_total` + `SemanticStoreReadsInc`/`SemanticStoreReadsValue` | ✓ VERIFIED | Lines 363, 751, 766. |
| `internal/semantic/store/effective_graph.go` | counter increment at the DuckDB read chokepoint | ✓ VERIFIED | `queryContext`/`queryRowContext` (lines 61-72) both call `SemanticStoreReadsInc()`. |
| `internal/lint/ablationleakage/analyzer.go` | call-site gate check | ✓ VERIFIED | `semanticReadMethods` + `ChooseSource` routing check (lines 47-208). |
| `internal/daemon/semantic_gate.go` | `resolveSemanticDisabled` + gated helpers | ✓ VERIFIED | Lines 33/43/58. |
| `bench/runtime/cell.go` | counter scrape + `== 0` assertion; deferral marker removed | ⚠️ PRESENT-BUT-VACUOUS | `scrapeSemanticReadsTotal`/`assertNoSemanticReads` present; marker count 0. Wiring is dead end-to-end (WR-02). |
| `bench/runners/your_agent_no_semantic/MODE.md` | gate key + 8-consumer enumeration | ✓ VERIFIED | All present; deferral section deleted. |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `internal/semantic/store` chokepoint | `SemanticStoreReadsInc` | counter increment | ✓ WIRED | effective_graph.go:63,72. |
| `daemon.go effSemanticDisabled` | symbols/health/repomap/guardrail + accessor block | single bool threaded | ✓ WIRED (tool-facing) | gate points confirmed in daemon.go + semantic_wiring.go. |
| `daemon.go SetActivateCallback` | counted chokepoint (background pipelines) | NOT gated | ✗ NOT_WIRED (gap, CR-01) | ScheduleInitialExtraction/startWorkspace/ensureRetrieval/SetFileFactStore reach `s.queryRowContext` ungated. |
| `bench cell` | `helix_semantic_store_reads_total` value | daemon-log shutdown line | ✗ BROKEN (WR-02) | Line only emitted on graceful shutdown(); bench uses SIGKILL → line never written → scrape returns 0. |
| `MODE.md` | the 8 consumers | documented enumeration | ✓ WIRED | All 8 named. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Project builds | `go build ./...` | exit 0 | ✓ PASS |
| Gate helper tests | `go test ./internal/daemon/ -run 'TestEffSemanticDisabled\|TestSemanticGate...'` | ok | ✓ PASS |
| Analyzer fixtures | `go test ./internal/lint/ablationleakage/` | ok (badgate flagged, goodgate silent) | ✓ PASS |
| Counter + labels | `go test ./internal/obs/... ./internal/semantic/store/ -run 'TestReadCounter\|Labels'` | ok | ✓ PASS |
| no_semantic cell assertion (unit) | `HELIX_BIN=… go test ./bench/runtime/ -run TestNoSemanticZeroReads` | ok (SYNTHETIC input only) | ✓ PASS (but proves nothing E2E — WR-02) |
| Profile flip | `go test ./internal/profile/...` | ok | ✓ PASS |
| Custom vet incl. ablation-leakage | `make vet` | exit 0 | ✓ PASS |

### Mandatory-Scrutiny Findings (independently confirmed)

**WR-02 (severe — confirms criterion #2 is VACUOUS end-to-end): CONFIRMED.**
- `internal/daemon/shutdown.go:60-62` emits the count line only inside `d.shutdown()`.
- `internal/daemon/daemon.go:1276` calls `d.shutdown()` only after `g.Wait()` returns; `g.Wait()` unblocks via SIGTERM/SIGINT (daemon.go:1179) or errgroup error.
- `internal/eval/sandbox/sandbox.go:203-204` — `Kill()` sends `syscall.Kill(-pid, SIGKILL)` + `cmd.Process.Kill()`. SIGKILL is untrappable.
- `bench/runtime/cell.go:535` calls `h.Kill()` directly (the comment at 532-533 even states "SIGKILL terminates immediately and cannot flush"). No graceful stop precedes it.
- `bench/runtime/cell.go:101-103` + the "scrape_missing_line_yields_zero" test (no_semantic_zero_reads_test.go:91-101) codify: missing line → count 0, no error.
- Net: on every real bench run the line is absent → scrape returns 0 → `assertNoSemanticReads("your_agent_no_semantic", 0)` passes unconditionally. The gate has no teeth E2E. This is a BLOCKER for criterion #2 and for the ABLATE-06 acceptance clause "runtime assertion logs and fails."

**CR-01 (latent — confirms criterion #1 is PARTIAL, not complete): CONFIRMED.**
- `effSemanticDisabled` gates the tool-facing `SemanticLookup` hand-outs + the SemanticSkill accessor block only.
- `daemon.go SetActivateCallback` (967-1003) unconditionally drives `ScheduleInitialExtraction`, `live.startWorkspace`, `rank.ensureScheduler`, `compactBndl.ensureCompactor`, `sBndl.ensureRetrieval` — guarded only by nil-checks, and under D-04 build-but-block the bundle is NON-nil.
- `daemon.go:425 SetFileFactStore(semanticStore)` is gated only by `if semanticStore != nil`, not by `effSemanticDisabled`.
- These reach the COUNTED chokepoint: `GetLatestFileFact` (filefact_accessor.go:75→102/117/169/180), `LatestCommittedSnapshot` (effective_graph.go:413→424), `QueryEffectiveAdjacency`/`CountStaleScoreRows` (effective_graph.go:145/197) — all `s.queryContext`/`s.queryRowContext`.
- LATENT because `StoreOptIn` is task-capability-driven (matrix.go:255-258), and the shipped five_of_six smoke uses store-OFF seed `IT-go-patch-apply-1` (five_of_six_test.go:40) → store never opens → chokepoint never reached. But a store-ON `no_semantic` cell (e.g. an `incremental_update` task) would read the store through these ungated pipelines. Criterion #1's "ZERO back-channel reads" is therefore PARTIAL.
- Interaction note: CR-01 and WR-02 mask each other — even if CR-01 produced a real background read, WR-02 guarantees it is never observed. Both must be fixed for the ABLATE-06 guarantee to actually hold.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| ABLATE-06 | 81-01, 81-02, 81-03, 81-04, 81-05 | Kernel `disable_semantic_subsystem` flag prevents any back-channel semantic-store read; E2E no_semantic makes zero queries; runtime assertion logs and fails | ⚠️ PARTIAL | Static gate (criteria #1/#3/#4) landed for tool-facing consumers. The acceptance clause "E2E … zero queries … runtime assertion logs and fails" is NOT met: the runtime assertion is vacuous end-to-end (WR-02) and the gate is incomplete against a store-ON arm (CR-01). REQUIREMENTS.md line 50/172 marks it `[x]/Complete` — this verification refutes that for the runtime-acceptance clause. |

All requirement IDs declared in PLAN frontmatter (`ABLATE-06` in all 5 plans) are accounted for. No orphaned requirements: REQUIREMENTS.md maps ABLATE-06 solely to Phase 81 (81-04, 81-05).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| bench/runtime/cell.go | 101-103 | "missing line → count 0, not an error" | 🛑 Blocker (root of WR-02) | Makes the zero-reads gate fail-open: a daemon that never emits the line (every SIGKILL teardown) silently passes. |
| internal/daemon/daemon.go | 967-1003, 425 | background read pipelines ungated by effSemanticDisabled | ⚠️ Warning (CR-01) | Store-ON no_semantic cell would read the counted store via internal pipelines. |
| internal/lint/ablationleakage/analyzer.go | 57-61 | `ValidateCriticalEdges` flagged but is a no-read passthrough (WR-03) | ℹ️ Info | Over-broad tripwire; analyzer still has teeth (badgate flagged). |
| internal/profile/profiles/bench-no-semantic.yaml | 15 | stale "Tool-filter-only this phase" comment | ℹ️ Info | Doc drift; the kernel gate DID land this phase. |
| bench/runtime/five_of_six_test.go | 131 | stale "1 partial row" wording (IN-01) | ℹ️ Info | no_semantic row is now clean, not partial. |

No `TBD`/`FIXME`/`XXX` debt markers found in phase-81 source files.

### Human Verification Required

None required to resolve the verdict — the blocker (WR-02) and the warning (CR-01) are statically observable from the code with file:line evidence above. (A future fix could be confirmed by an actual real-daemon bench run on a store-ON `no_semantic` cell asserting the reads-total line is present and equals the true count, but that is fix-validation, not goal-verification.)

### Gaps Summary

Three of four criteria landed cleanly as static structure: the distinct config/profile/CLI/YAML gate surface (criterion #1 plumbing), the composition-root `effSemanticDisabled` resolution with tool-facing consumers forced to NoopLookup + disabled ConfigGate under build-but-block (criterion #1 tool path), the extended call-site `vet-ablation-leakage` analyzer (criterion #3), and the rewritten MODE.md with the full 8-consumer enumeration (criterion #4). Build, `go vet`, `make vet`, and all targeted tests are green.

The phase goal — "an E2E config-gate test that PROVES the no_semantic arm makes ZERO back-channel reads against the DuckDB store" — is NOT achieved:

1. **Criterion #2 is vacuous end-to-end (BLOCKER, WR-02).** The runtime proof depends on a counter line that is only emitted in the graceful `shutdown()` path, which the bench's SIGKILL teardown never reaches. The "zero reads" gate therefore reads 0 on every run and can never observe a real read — the one thing the phase exists to prove. The only test covering it injects synthetic log lines and never exercises real teardown.

2. **Criterion #1 is partial against the stated guarantee (WARNING, CR-01).** Daemon-internal activation pipelines and the file-fact store hit the counted chokepoint ungated; "zero back-channel reads" holds only for the tool surface and only because the current corpus runs store-OFF.

The static guard (criterion #3) and documentation (criterion #4) are sound; the dynamic guarantee (criterion #2) — the load-bearing claim of the phase — is not real. Recommend `gaps_found`: do not treat ABLATE-06's runtime-acceptance clause as satisfied until criterion #2 has end-to-end teeth (and, for completeness, the CR-01 background pipelines are gated or the smoke matrix exercises a store-ON no_semantic cell).

---

_Verified: 2026-06-20T16:30:00Z_
_Verifier: Claude (gsd-verifier)_
