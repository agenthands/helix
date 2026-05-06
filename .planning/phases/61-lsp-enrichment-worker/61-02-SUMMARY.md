---
phase: 61-lsp-enrichment-worker
plan: 02
subsystem: semantic-enrichment
tags: [phase-61, lsp-enrichment, worker, cascade, budget, readiness, lspool, errgroup]

# Dependency graph
requires:
  - phase: 61-01
    provides: "LaneQueue + LeaseAcquirer + Outcome enum + MetricsSink interface + OverlayStore interface (P01 types.go) + ProdMetricsSink"
  - phase: 60
    provides: "OverlayTx (BeginOverlayTx, UpsertSymbols, MarkFileSemanticPending, Commit) + per-tx epoch contract"
  - phase: 56
    provides: "*lspool.Pool / *lspool.WorkerLease + JdtlsAdapter + RustAnalyzerAdapter (used via the LeaseProvider seam)"
provides:
  - "lspenrich.Worker: drain loop honoring max_concurrent_workers via RunN(ctx, n) + per-job timeout context + outcome metrics"
  - "lspenrich.Cascade: §14.4 6-step orchestration (documentSymbol → diagnostics → hover/callHierarchy/typeHierarchy/implementation per symbol → definition per reference) with per-step yield + budget enforcement (already committed in mid-Wave-2 pause)"
  - "lspenrich.Budget: per-file deadline + total deadline + max_symbols_per_file + per-symbol/per-file reference caps (already committed)"
  - "lspenrich.WaitForLanguageReady: Java/Rust readiness dispatch + best-effort fall-through (already committed)"
  - "lspenrich.LeaseProvider: B2 fix-path-A — interface seam consumed by P03 Manager to own the cached per-(wsKey, lang) lease"
  - "lspenrich.CascadeLSPFactory: production-vs-test seam for the LSP shim wrapping a *lspool.WorkerLease"
  - "Real-LS integration tests (Go + Java) gated by //go:build integration"
affects: [phase-61-03, phase-61-04, phase-62]

# Tech tracking
tech-stack:
  added:
    - "golang.org/x/sync/errgroup (concurrency cap for RunN drain goroutines)"
  patterns:
    - "Worker errgroup pattern: RunN spawns N drain goroutines sharing one LaneQueue; first error or ctx cancel propagates"
    - "B2 lease lifecycle (fix-path-A): Worker.processOne calls LeaseProvider.AcquireFor (no Release) — Manager owns the cached handle and releases on workspace deactivation"
    - "Test-only CascadeLSPFactory injection: production wires lease.Request adapter; tests inject recording fakes without spawning real LSPs"
    - "Build-tag integration tests: //go:build integration runs against real gopls + jdtls, skips cleanly when LS missing"

key-files:
  created:
    - "internal/semantic/lspenrich/worker.go (Task 4 — Worker.RunN drain loop + processOne)"
    - "internal/semantic/lspenrich/worker_test.go (Task 4 — 12 W-tests with -race)"
    - "internal/semantic/lspenrich/cascade_integration_test.go (Task 5 — real-LSP shim + Go/Java integration tests)"
    - "internal/semantic/lspenrich/testdata/cascade/go/{go.mod,main.go} (Go fixture)"
    - "internal/semantic/lspenrich/testdata/cascade/java/{pom.xml,.gitignore,src/main/java/com/example/{A,B}.java} (Java fixture)"
  modified: []

key-decisions:
  - "Worker.NewCascadeLSP is a CascadeLSPFactory field (not a hard-coded production adapter) so unit tests inject recording fakes without spawning real LSPs and the production daemon (P03) supplies the real lease.Request shim — keeps Worker single-responsibility for orchestration."
  - "Worker.markPending uses CascadeStore.BeginCascadeTx (the same tx surface the cascade uses) instead of a separate OverlayStore method — eliminates a redundant interface and lets the Manager wire one adapter for both surfaces."
  - "Integration test's realLSPShim treats soft non-fatal LSP errors ('not a type name', 'is a function, not a method', etc.) as no-result rather than LS-unavailable — production gopls+jdtls return these on capability mismatches that aren't real LS failures."
  - "languageFor mapping is in the worker (not a shared util) since the only consumer is processOne; Phase 62 will replace with langregistry-driven detection when richer metadata is needed."

patterns-established:
  - "B2 lease ownership boundary: per-job acquire (cached by Manager) + zero per-job release. Asserted by tests W10/W11."
  - "W3 explicit Capabilities: Worker passes Capabilities: w.Capabilities into Cascade construction so MethodNotFound continuation never panics on a nil cache. Asserted behaviourally by W12."
  - "Outcome → metric mapping: Worker emits LSPEnrichmentTotal(lang, string(outcome)) where outcome is the closed-enum P01 Outcome — no fan-out switch, no per-outcome conditional metric calls."
  - "Integration testing without daemon bootstrap: tests construct *lspool.NewPool directly with stub MemoryPressure + nil installer; pool.Run in a goroutine; AcquireLease with sessionID prefix 'lsp-enrichment:integration:' to avoid foreground-busy stamping."

requirements-completed: [ENRICH-02, ENRICH-03, ENRICH-04]

# Metrics
duration: ~75min (resumption only — Tasks 4 + 5; Tasks 1-3 from prior session)
completed: 2026-05-06
---

# Phase 61 Plan 02: Worker + Cascade Integration Test Summary

**LSP enrichment Worker drains LaneQueue via RunN(ctx, n) errgroup, runs the §14.4 cascade with explicit Capabilities (W3) without ever releasing the cached lease (B2), and ships real-gopls/jdtls integration tests (Task 5) that produce ≥ 1 CALLS edge against the Go fixture (acceptance #9).**

## Performance

- **Duration:** ~75 min (resumption only — Tasks 1-3 were committed in the prior session before the mid-Wave-2 pause)
- **Started:** 2026-05-06T09:21:00Z (approx, resumption start)
- **Completed:** 2026-05-06T~10:35:00Z
- **Tasks executed in this resumption:** 2 (Task 4, Task 5)
- **Tasks already committed before resumption:** 3 (Tasks 1, 2, 3)
- **Files created in this resumption:** 7 (worker.go, worker_test.go, cascade_integration_test.go, 4 fixture files)

## Accomplishments

- **Worker drain loop (Task 4)**: RunN(ctx, n) errgroup spawns N drain goroutines sharing one LaneQueue. processOne pipeline: derive lang → per-job ctx with TimeoutPerFile → WaitForLanguageReady (Java/Rust) → LeaseProvider.AcquireFor (NO Release — B2 fix-path-A) → NewBudget → Cascade with explicit Capabilities → record outcome metrics.
- **B2 lease lifecycle invariant**: Worker never calls lease.Release. Tests W10 + W11 assert Release count == 0 after N successful jobs and after AcquireFor failure.
- **W3 explicit Capabilities**: Cascade is constructed with `Capabilities: w.Capabilities` (never nil). Test W12 asserts behaviourally via MethodNotFound continuation.
- **Outcome metric pipeline**: ErrCircuitOpen / ErrMaxWorkersReached → outcome=dropped (no partial_reason); other AcquireFor errors → outcome=partial_lsp_unavailable + markPending; cascade outcome strings flow through verbatim.
- **Lane depth gauges**: After every processed job the drain emits LSPEnrichmentLaneDepth("high", N) and ("background", N) so operators can alert on background-lane growth.
- **Real-LSP integration tests (Task 5)**: Go + Java cascade tests against real gopls + jdtls produce 3 symbols + 8 edges (multiple CALLS) on the Go fixture; Java fixture produces 3 symbols + 8 edges (CALLS + EXTENDS + TYPE_OF). All emitted edges meet the typed contract (Confidence == 1.0, ValidationState == "validated", Source prefix "lsp.").
- **Build-tag gating**: `//go:build integration` keeps the unit-test path fast (~2s); integration tests run with `-tags integration` and skip cleanly via t.Skip() when gopls / jdtls are absent.

## Task Commits

Each task was committed atomically:

1. **Task 1: Budget value type** — `a39e6708` (feat) — committed in prior session
2. **Task 2: Readiness probe + dispatch** — `234b47a5` (feat) — committed in prior session
3. **Task 3: Cascade engine §14.4 6-step + epoch test** — `8280d22a` (test, RED) + `ff4a7593` (feat, GREEN) — committed in prior session
4. **Task 4: Worker drain loop + outcome metrics + B2 lifecycle** — `67f563dd` (test, RED) + `fe958842` (feat, GREEN)
5. **Task 5: §14.4 cascade integration test (Go + Java)** — `401d6eca` (test)

**TDD gate compliance:** Task 4 ships RED→GREEN cycle (`test(61-02): add failing worker tests` → `feat(61-02): implement Worker drain loop`). Task 3's gate cycle was committed in the prior session. Task 5 is integration-only (no separate RED — the test exercises pre-existing cascade code against a real LSP, so a "passing test" without prior implementation is the expected and correct shape).

## Files Created/Modified

- `internal/semantic/lspenrich/worker.go` — Worker struct + RunN(ctx, n) errgroup drain + processOne pipeline + LeaseProvider interface (B2 seam) + CascadeLSPFactory (test-vs-prod seam) + languageFor extension dispatch + markPending fast path.
- `internal/semantic/lspenrich/worker_test.go` — 12 W-tests covering drain ctx.Done propagation, per-job timeout context, concurrency cap, Java/Rust readiness gating, ErrCircuitOpen → dropped, lane-depth metric emission, cascade outcome propagation, B2 release-count invariant (W10/W11), W3 capabilities-non-nil invariant (W12).
- `internal/semantic/lspenrich/cascade_integration_test.go` — realLSPShim + integrationTestPool helpers + TestCascade_GoIntegration + TestCascade_JavaIntegration (skip-when-LS-missing semantics; soft-error tolerance for "not a type name" / "is a function, not a method" / etc.).
- `internal/semantic/lspenrich/testdata/cascade/go/{go.mod,main.go}` — minimal Go fixture: main calls Greet + Farewell so callHierarchy yields ≥ 1 CALLS edge.
- `internal/semantic/lspenrich/testdata/cascade/java/{pom.xml,.gitignore,src/main/java/com/example/{A,B}.java}` — minimal Maven Java fixture: A.method() → B.method() call. .gitignore excludes the .jdtls-data workspace state and target/ build output.

## Decisions Made

- **CascadeLSPFactory injection seam (Worker.NewCascadeLSP field).** The plan suggests Worker constructs the cascade inline; this implementation adds an injectable factory so unit tests can drive the cascade with a recording fake (no real LSP) while production wires a small adapter dispatching to *lspool.WorkerLease.Request. Keeps Worker single-responsibility for orchestration; the LSP-translation surface is owned by P03 daemon bootstrap.
- **markPending reuses CascadeStore.BeginCascadeTx (not a parallel OverlayStore.BeginOverlayTx path).** The cascade already opens its own tx via CascadeStore; markPending (the readiness-timeout / pre-cascade-error fast path) reuses the same tx surface, which means the Manager only has to wire one adapter for both code paths.
- **Integration-test soft-error tolerance.** Real gopls returns errors like `"not a type name"` and `"Greet is a function, not a method"` from prepareTypeHierarchy / implementation when the cursor is on an inapplicable symbol. These are NOT MethodNotFound (the method exists; the input is just inapplicable) and NOT LS-unavailable. The realLSPShim maps a documented allow-list of these strings to nil-result (no edges) so the cascade continues. Production (P03) adapter will need the same tolerance — flagged for the P03 daemon-wiring planner.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Real gopls returns non-fatal errors that abort cascade**
- **Found during:** Task 5 (integration test runtime)
- **Issue:** The plan described prepareTypeHierarchy / implementation behavior in terms of MethodNotFound (-32601) only. Real gopls returns runtime errors like `"not a type name"` and `"Greet is a function, not a method"` when the cursor is on a non-type / non-method symbol. The cascade treated these as LS-unavailable (whole-cascade abort, partial_reason="lsp_unavailable"), causing the Go integration test to fail with OutcomePartialLSPUnavail.
- **Fix:** Added isNonFatalLSPError() helper in cascade_integration_test.go that maps a documented allow-list of strings ("not a type name", "is a function, not a method", "no implementation found", "no type hierarchy", etc.) to nil-result so the cascade continues. The CASCADE itself was not modified — only the realLSPShim's translation layer; production-wiring adapters will need the same tolerance.
- **Files modified:** internal/semantic/lspenrich/cascade_integration_test.go
- **Verification:** `go test -tags integration -run TestCascade_GoIntegration` produces "3 symbols, 8 edges (TYPE_OF + multiple CALLS)" and exits 0.
- **Committed in:** `401d6eca` (Task 5 commit)

**2. [Rule 1 — Bug] jdtls workspace pollution into unrelated testdata**
- **Found during:** Task 5 (integration test runtime — first Java run)
- **Issue:** jdtls writes its persistent workspace cache into a `.jdtls-data/` directory; the test was writing into `testdata/fixtures/java/` (a Phase 47 fixture) AND into the new `testdata/cascade/java/` fixture. The Phase 47 fixture's `.project` file was being normalized (tabs vs. spaces) and `bin/` class files were being created — both unrelated to Plan 61-02.
- **Fix:** Reverted the Phase 47 fixture changes via `git checkout`. Added `.gitignore` to `testdata/cascade/java/` excluding `.jdtls-data/` and `target/` so future test runs don't pollute the working tree.
- **Files modified:** Created `internal/semantic/lspenrich/testdata/cascade/java/.gitignore`; reverted `testdata/fixtures/java/.project` to its prior state.
- **Verification:** `git status --short` after a clean test run shows no untracked Phase 47 changes.
- **Committed in:** `401d6eca` (Task 5 commit, includes the .gitignore)

---

**Total deviations:** 2 auto-fixed (1 blocking — real-LS soft errors not pre-specified; 1 bug — jdtls workspace pollution).
**Impact on plan:** Both fixes are necessary for the integration tests to pass on a clean checkout. Neither modifies the cascade engine's contract; both are confined to the Task 5 integration-test layer.

## Issues Encountered

- **Phase 47 testdata pollution by jdtls.** Caught early during Task 5 development — the first Java integration test run modified `testdata/fixtures/java/.project` (tabs vs. spaces normalization) and created `testdata/fixtures/java/bin/*.class` files. Resolved by reverting and adding the targeted `.gitignore` (see Deviation #2).
- **gopls non-fatal errors on prepareTypeHierarchy / implementation.** Caught during Task 5 — see Deviation #1. Resolved by adding the soft-error allow-list in the realLSPShim.

## Next Phase Readiness

- **For 61-03 (Manager + observability):** lspenrich.LeaseProvider interface is published with the AcquireFor(ctx, wsKey, lang) signature. P03's *Manager will satisfy this interface, owning a sync.Map of cached *lspool.WorkerLease keyed by (wsKey, lang). The Worker's NewCascadeLSP factory field needs a production adapter that dispatches to *lspool.WorkerLease.Request — P03 owns this wiring step.
- **For 61-04 (stress testing):** Worker.RunN(ctx, n) is the entrypoint for stress harnesses. The B2 invariant (Release count == 0 after N successful jobs) is enforced by W10; the W3 invariant (cascade Capabilities non-nil) is enforced by W12.
- **Open carry-over for P03:** the production CascadeLSP adapter wrapping *lspool.WorkerLease.Request will need the same soft-error tolerance (`"not a type name"` etc.) that the integration test's realLSPShim implements. Flagged for the P03 planner as a daemon-wiring requirement.
- **All four verification gates pass:** `go build ./...` clean; `go test -race ./internal/semantic/lspenrich/...` exits 0 in 2s; `go test -tags integration -run TestCascade_(Go|Java)Integration ./internal/semantic/lspenrich/... -count=1 -timeout 120s` exits 0 in 12s; `make vet` clean across all four analyzers (default, noduckdb, nokernel2semantic, nosemantic2kernel).

## Self-Check

Verified after writing this SUMMARY:

- File `internal/semantic/lspenrich/worker.go`: FOUND
- File `internal/semantic/lspenrich/worker_test.go`: FOUND
- File `internal/semantic/lspenrich/cascade_integration_test.go`: FOUND
- File `internal/semantic/lspenrich/testdata/cascade/go/main.go`: FOUND
- File `internal/semantic/lspenrich/testdata/cascade/java/pom.xml`: FOUND
- Commit `67f563dd` (Task 4 RED): FOUND in `git log`
- Commit `fe958842` (Task 4 GREEN): FOUND in `git log`
- Commit `401d6eca` (Task 5): FOUND in `git log`

## Self-Check: PASSED

---
*Phase: 61-lsp-enrichment-worker*
*Completed: 2026-05-06*
