---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
plan: 04
subsystem: infra
tags: [daemon, ablation, no-lsp, composition-root, null-object, trace-tap, tdd]

# Dependency graph
requires:
  - phase: 76-01
    provides: kernel.KernelConfig.DisableLSPSubsystem + LSPSubsystemDisabled() accessor
  - phase: 76-02
    provides: Profile.DisableLSPSubsystem / DisableStructuredEditSubsystem yaml fields + bench-no-lsp profile
  - phase: 60-semantic-live-service
    provides: EditNotifier nil-check contract reused by the no-op LSP wiring
provides:
  - "--disable-lsp-subsystem / --disable-structured-edit-subsystem CLI override flags"
  - config.SerenaConfig.DisableLSPSubsystem / DisableStructuredEditSubsystem koanf fields
  - Daemon composition-root effective-flag computation (cfg || profile) threaded into kernel.KernelConfig
  - no_lsp null-object wiring (skipped live bundle, cleared SetEnrichFn, cleared fallback deps, neutralized diag leaseFn)
  - TestNoLSPWiring + TestNoLSPZeroSpans + TestNoLSPDefaultArmUnchanged (ABLATE-05 trace-tap gate)
  - RepoMapSkill.HasEnrichFn() / HasFallbackDeps() wiring-state accessors
affects: [phase-77, phase-80, phase-81, bench-no-lsp]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Composition-root effective-flag resolution: (cfg.X || activeProfile.X) one-way force-disable (D-02/D-03)"
    - "D-09 structural null-object injection: no_lsp achieved by what the daemon does NOT wire, not per-callsite flag checks"
    - "D-10 audit: neutralize non-profile-gated pool-leasing paths (repomap fallback AcquireFn + diag leaseFn) under the flag"
    - "Trace-tap hard fail via tracetest.InMemoryExporter on an obs.NewForTest provider injected through NewWithObsProvider"

key-files:
  created:
    - internal/daemon/no_lsp_wiring_test.go
    - .planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/deferred-items.md
  modified:
    - internal/cli/root.go
    - internal/config/config.go
    - internal/daemon/daemon.go
    - internal/daemon/daemon_test_export_test.go
    - internal/skill/repomap/skill.go

decisions:
  - "Hoisted config.ResolveProfile from former step 8 to a new step 4b so the kernel can be constructed with effective disable flags (NewKernel is at step 5, before the original profile resolution)"
  - "Used explicit null-object CLEAR (SetEnrichFn(nil) / SetFallbackDeps(nil)) under no_lsp rather than a bare skip, because RepoMapSkill is a process-global singleton — clearing keeps the wiring idempotent across repeated daemon constructions (and across tests)"
  - "Skip buildLiveBundle entirely under no_lsp so kernel.EditNotifier() stays nil; all 4 fileops OnEdit hooks no-op via the existing nil-check contract (no per-callsite change needed)"
  - "Neutralized the non-profile-gated diag leaseFn to a stub returning serr.Unsupported (the *Error value, not .Error() string, since LeaseProvider returns error) without touching k.Pool()"

# Metrics
duration: ~50min
completed: 2026-06-16
---

# Phase 76 Plan 04: no_lsp Daemon Composition-Root Wiring Summary

**Two CLI ablation override flags + their SerenaConfig fields, composition-root effective-flag computation `(cfg || profile)` threaded into kernel.KernelConfig, and D-09 structural null-object suppression of every LSP-touching seam under `disable_lsp_subsystem` — proven by a zero-`lspool.lsp.*`-span trace-tap (ABLATE-05, ROADMAP success criterion #2).**

## Performance
- **Duration:** ~50 min
- **Started:** 2026-06-16T16:53:00Z
- **Completed:** 2026-06-16T17:43:00Z (approx)
- **Tasks:** 2 (Task 1 auto; Task 2 TDD RED → GREEN)
- **Files modified:** 5 source + 1 new test + 1 deferred-items log

## Accomplishments
- **Task 1 — flag threading:** Added `--disable-lsp-subsystem` / `--disable-structured-edit-subsystem` persistent bool flags on `rootCmd` (only inserted into the override map when set, mirroring the `--admin-addr` only-when-set guard so a blank invocation cannot wipe a profile/config value). Added `DisableLSPSubsystem` / `DisableStructuredEditSubsystem` `koanf`-tagged fields on `config.SerenaConfig`. Hoisted `config.ResolveProfile` to a new step 4b so the daemon computes `effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem` (and the SE analogue) and threads both into the `kernel.KernelConfig` literal at `NewKernel` (step 5).
- **Task 2 — no_lsp null-object wiring (D-09) + D-10 audit (TDD):**
  - Skipped `buildLiveBundle` under `effDisableLSP` → `kernel.EditNotifier()` stays nil → the 4 fileops `OnEdit` hooks no-op via the existing nil-check contract (no live notifier, no LSP-enrichment manager).
  - Cleared `rs.SetEnrichFn(nil)` under the flag so the RepoMap engine falls back to tree-sitter and never invokes `enrichRepoMapFromLSP` (the only enrichment seam that leases an LS worker).
  - Cleared `rs.SetFallbackDeps(nil)` (D-10 audit: the fallback `AcquireFn` calls `k.Pool().AcquireLease` and is NOT profile-gated).
  - Neutralized the non-profile-gated diag `leaseFn` to a stub returning `serr.Unsupported` without touching `k.Pool()` (D-10 audit).
  - Added `TestNoLSPWiring`, `TestNoLSPZeroSpans` (trace-tap on `obs.NewForTest` + `tracetest.InMemoryExporter`), and `TestNoLSPDefaultArmUnchanged`.

## Task Commits
1. **Task 1: CLI flags + SerenaConfig fields + KernelConfig threading** — `f3ddf581` (feat)
2. **Task 2 RED: failing no_lsp wiring + zero-span tests** — `fe23d97a` (test)
3. **Task 2 GREEN: null-object no_lsp wiring at the composition root** — `ea46db4d` (feat)

(Plan metadata commit: see final docs commit.)

## Files Created/Modified
- `internal/cli/root.go` — two persistent bool flags + their only-when-set override-map entries
- `internal/config/config.go` — `DisableLSPSubsystem` / `DisableStructuredEditSubsystem` koanf fields with doc comments
- `internal/daemon/daemon.go` — step 4b profile hoist + effective-flag computation + KernelConfig threading + conditional null-object wiring (live bundle skip, enrich/fallback clear, diag leaseFn stub)
- `internal/daemon/daemon_test_export_test.go` — `KernelForTest()` accessor
- `internal/daemon/no_lsp_wiring_test.go` — the 3 ABLATE-05 tests
- `internal/skill/repomap/skill.go` — `HasEnrichFn()` / `HasFallbackDeps()` wiring-state accessors

## Decisions Made
- **Profile hoist (step 4b):** `NewKernel` is at step 5 but profile resolution was at step 8; hoisting `ResolveProfile` (and removing the now-duplicate step 8 block) is the minimal reorder that makes `activeProfile.DisableLSPSubsystem` available before kernel construction.
- **Explicit null-object clear vs bare skip:** `RepoMapSkill` is a process-global singleton (`GetRepoMapSkill()`). A bare `if !effDisableLSP { Set... }` would leave a stale enrich/fallback fn from a prior daemon construction; calling `SetEnrichFn(nil)` / `SetFallbackDeps(nil)` under the flag makes the wiring idempotent and the no_lsp guarantee hold regardless of construction order.
- **Diag leaseFn returns the typed `*Error`, not `.Error()`:** `diag.LeaseProvider` returns `error`; `serr.New(...)` already implements `error`, so the stub returns the value directly (unlike the 76-01 handler guards which feed `errorResult` a string).

## Deviations from Plan

### Auto-fixed Issues
**1. [Rule 2 - Test isolation correctness] Used explicit null-object CLEAR instead of bare skip**
- **Found during:** Task 2 GREEN
- **Issue:** The plan suggested wrapping the wiring blocks in `if !effDisableLSP { ... }`. Because `RepoMapSkill` is a global singleton, a bare skip leaves a stale enrich/fallback fn set by an earlier daemon construction (observable across the test suite, and in any process that reconstructs the daemon). `HasEnrichFn()` would then report a leaked LSP callback under no_lsp.
- **Fix:** Under `effDisableLSP`, call `SetEnrichFn(nil)` / `SetFallbackDeps(nil)` (explicit null-object) so the suppression is idempotent and order-independent.
- **Files modified:** internal/daemon/daemon.go
- **Commit:** `ea46db4d`

**Total deviations:** 1 auto-fixed (correctness/idempotency of the null-object wiring). No scope creep.

## Deferred Issues
- **`test/bench` golden manifest/descriptions drift (53 vs 47 tools):** the phase-gate `go test ./...` surfaced `TestBenchToolsManifestMatchesRegistry` + `TestToolDescriptionsGoldenFile` failures. Verified PRE-EXISTING at base commit `7dd0c38a` (before this session) in a clean worktree — Plan 76-04 adds zero MCP tools. Out of scope (SCOPE BOUNDARY); logged in `deferred-items.md` with a suggested bench-manifest-refresh owner. The 76-04 package suite (`daemon`/`config`/`cli`) is green.

## Known Stubs
None — the diag `leaseFn` no_lsp branch is a deliberate `serr.Unsupported` null-object (the documented ablation behavior), not an unfinished stub. The `bench-no-semantic` `disable_semantic_subsystem` kernel guard remains Phase 81 (D-12), as specified — not wired here.

## Threat Flags
None — no new network endpoints, auth paths, or schema changes. The change REDUCES reachable surface under the flag (suppresses pool leases + enrichment). Threat register dispositions satisfied: T-76-07 (lspool.lsp.* span leak) mitigated by D-09 null-object + D-10 audit + the TestNoLSPZeroSpans hard-fail trace-tap; T-76-08 (config precedence) mitigated by the `(cfg || profile)` effective-flag computation + TestNoLSPDefaultArmUnchanged.

## TDD Gate Compliance
Task 2 followed RED → GREEN with separate commits: `fe23d97a` (test, RED — TestNoLSPWiring failed on the unconditional enrich/fallback wiring) → `ea46db4d` (feat, GREEN). No REFACTOR commit needed (code clean at GREEN). Task 1 is non-TDD (pure threading) per the plan's `type="auto"`.

## Verification
- `go build ./cmd/helix` — succeeds.
- `go vet ./...` — clean (whole module).
- `go test ./internal/daemon/... ./internal/config/... ./internal/cli/... -count=1` — pass, including TestNoLSPWiring / TestNoLSPZeroSpans / TestNoLSPDefaultArmUnchanged.
- D-10 audit: every `k.Pool().AcquireLease` reachable from a daemon-wired tool path is now either profile-excluded (bench-no-lsp omits symbol-retrieval + diagnostics skills, 76-02) or gated under `!effDisableLSP` (diag leaseFn, repomap fallback AcquireFn, enrichRepoMapFromLSP via the cleared enrichFn).
- `grep -n "disable-lsp-subsystem\|disable-structured-edit-subsystem" internal/cli/root.go` — shows both flags + both override entries.

## Self-Check: PASSED
(see appended verification below)

## Next Phase Readiness
- `--profile=bench-no-lsp` (76-02) + `disable_lsp_subsystem` now produce a daemon that emits zero `lspool.lsp.*` spans — ready for Phase 77's bench-runner subprocess launch.
- Phase 81 will add the kernel `disable_semantic_subsystem` guard paired with `bench-no-semantic` (deferred per D-12); the effective-flag computation pattern established here is the template.
- No blockers (the `test/bench` golden drift is pre-existing and tracked separately).

---
*Phase: 76-ablation-profiles-kernel-subsystem-disable-flags*
*Completed: 2026-06-16*
