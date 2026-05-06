---
phase: 57-semantic-store-foundation-pipeline-dag-library
verified: 2026-05-06T00:00:00Z
status: passed
score: 5/5 success criteria verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 4/5
  gaps_closed:
    - "SC-1: get_health did not report semantic store readiness"
  gaps_remaining: []
  regressions: []
findings_closed:
  - id: SC-1
    status: CLOSED
    evidence: "internal/kernel/health/tools.go now defines SemanticStoreProbe interface, SemanticStoreStatus struct (state ∈ {disabled, ready, unhealthy}), and ComputeSemanticStoreStatus; daemon.go line 365 passes semanticStoreProbe{s: semanticStore} into health.RegisterTools; closed-enum Reason field (probe_timeout / nil_handle / db_error / unknown) prevents raw err.Error() leaks (WR-NEW-01)."
  - id: CR-01
    status: CLOSED
    evidence: "internal/semantic/store/duckdb.go:170-177 — explicit filepath.IsAbs guard rejects absolute paths AND a forward-slash split rejects any '..' segment, both with serr.ErrInvalidArgs. TestOpen_RejectsParentTraversal (4 sub-cases) + TestOpen_RejectsAbsolutePath (2 sub-cases) PASS. Commits ef5c3fa3 + 37e11ad8."
  - id: CR-02
    status: CLOSED
    evidence: "internal/semantic/store/duckdb.go:288-289 — schema-version probe runs in a 5s child context; deferred qcancel() in place. awk classifyExisting block contains 2 occurrences of context.WithTimeout(ctx, 5*time.Second). Commit 361bb7df."
  - id: WR-01
    status: CLOSED
    evidence: "internal/semantic/store/migrations.go:9-26 defines applyStatementsTx (BeginTx → ExecContext loop → Commit / deferred Rollback); applyMigration001 delegates to it (line 57-58). TestApplyMigration001_RollsBackOnFailure + TestApplyMigration001_UsesTransactionalHelper PASS. Commits 3be9ec80 + 3716a8c1 + c637f0f3."
  - id: WR-02
    status: CLOSED
    evidence: "internal/lint/noduckdb/analyzer.go:32 matcher reads `path == forbiddenImport || strings.HasPrefix(path, forbiddenImport+\"/\")` — exact-package OR slash-prefix only. siblingpkg fixture imports both `duckdb-go-sibling` (bare) AND `duckdb-go-sibling/v2` (slash subpath); TestAnalyzer_AllowsSiblingNamespacePackage PASSES with no //want directive (analysistest fails if any diagnostic is reported). Commits 66e4c31f + f52c86f4 + 269a51ab."
  - id: WR-03
    status: CLOSED
    evidence: "internal/semantic/store/duckdb.go:74-96 defines classifyReopenError returning reopenTransient / reopenCorruption / reopenUnknown; lines 219-251 wire it into Open's reopen path with a 250ms time.After backoff and a single retry. WR-NEW-02: decideReopenRetry hard-fails on persistent-transient (line 240-242 'transient errors persist across retry; refusing to quarantine clean DB'). TestClassifyReopenError (8 sub-cases) + TestClassifyReopenError_ETXTBSY + TestDecideReopenRetry_PersistentTransientHardFails (6 sub-cases) PASS. Commits 3fdd5d0c + 6d20469f + b115fa00."
  - id: BL-01
    status: CLOSED
    evidence: "Absolute-path rejection (the original 57-05 gap) was added in commits 5d81645f (RED) + 37e11ad8 (GREEN). filepath.IsAbs guard at duckdb.go:170 runs BEFORE the '..' segment scan; semantic.config.go doc 'absolute paths and `..` segments are rejected at Open time' now matches the code. TestOpen_RejectsAbsolutePath PASSES."
deferred:
  - finding: "WR-04: quarantineAndRebuild symlink-refusal comment misleads (check is on source, not target)"
    addressed_in: "Out of scope of 57-05 (option-C scope decision); not introduced by 57-05; safe to triage in a follow-up hardening pass."
  - finding: "WR-05: QueryEffective* API surface uses `any` for query/result types"
    addressed_in: "Phase 59/60 land snapshot + overlay write paths; the typed read API will be locked then."
  - finding: "WR-06: kahnSort defensive zero-init is dead code"
    addressed_in: "Cosmetic; not in scope of 57-05; safe to land in any future phasegraph touch."
  - finding: "WR-07: findCycle doc comment / code drift"
    addressed_in: "Cosmetic; not in scope of 57-05; tests pass via containment-only assertion."
---

# Phase 57: Semantic Store Foundation + Pipeline DAG Library — Re-Verification Report

**Phase Goal:** A semantic fact store opens at daemon start, configuration flows through the existing 4-layer precedence, and a stdlib pipeline-DAG library is available for the new graphs to consume — without touching the existing imperative bootstrap.

**Verified:** 2026-05-06
**Status:** passed
**Re-verification:** Yes — after Plan 57-05 (gap-closure + hardening) and 57-REVIEW-57-05 follow-up fix run

## Re-Verification Summary

The prior verification (2026-05-03) returned `gaps_found` because SC-1 was only half-met: the store opened but `get_health` did not report its readiness. Plan 57-05 plus the 57-REVIEW-57-05 follow-up fix run have now landed (11 commits — 66e4c31f through 4de3c36e — plus the BL-01 / WR-NEW-* / IN-NEW-* follow-up hashes captured in `findings_closed`). All six original closure items (SC-1, CR-01, CR-02, WR-01, WR-02, WR-03) AND the seven follow-up findings (BL-01, WR-NEW-01..03, IN-NEW-01..03) verify CLOSED against the live source tree.

## Goal Achievement — ROADMAP Success Criteria

| #   | Truth (SC) | Status | Evidence |
| --- | ---------- | ------ | -------- |
| 1   | enabled=true opens (or quarantines+rebuilds) `<workspace>/.helix/semantic.duckdb` AND `get_health` reports store as ready | VERIFIED | Store half: 9 store_test.go tests pass under `cgo` (incl. fresh / corrupt-header / forward-incompat / quarantine paths). get_health half (NEW): `internal/kernel/health/tools.go` lines 19-113 define SemanticStoreProbe, SemanticStoreStatus, and ComputeSemanticStoreStatus; daemon.go:365 wires `semanticStoreProbe{s: semanticStore}` into health.RegisterTools; envelope at tools.go:159-165 emits `"semantic_store"` JSON block alongside the LS HealthReport. 7 new tests in `tools_semantic_test.go` cover ready / disabled / nil-probe / unhealthy + 3 closed-enum reason buckets + JSON shape — all PASS. |
| 2   | enabled=false: every semantic-dependent tool returns `Kind: Unsupported`; rest of Helix continues to serve | VERIFIED (foundation half — downstream deferred to P64+) | TestDaemon_SemanticStore_FromYAML_Disabled passes under `cgo,integration`; daemon.SemanticStore() returns nil; Available() returns false; ComputeSemanticStoreStatus(...) returns state="disabled" without panic. The "every semantic-dependent tool" half remains downstream — no semantic-dependent MCP tools exist in P57 (P64 owns them). Foundation contract is met. |
| 3   | A `go vet`-runnable lint fails the build if any package outside `internal/semantic/store/` imports `duckdb-go` | VERIFIED | `cmd/vet-noduckdb` builds; analyzer matches via `path == forbiddenImport \|\| strings.HasPrefix(path, forbiddenImport+"/")` (analyzer.go:32) — exact-package OR slash subpath only, no sibling-namespace bleed. Three analysistest fixtures (badpkg / goodpkg / siblingpkg with both bare and /v2 forms) all PASS. `go vet -vettool=$GOPATH/bin/vet-noduckdb ./internal/... ./cmd/...` exits 0. |
| 4   | Three pipelines as typed phase DAGs validated by phasegraph; daemon bootstrap remains imperative with TODO(v1.11) marker | VERIFIED | semantic / live / eval pipelines ship 12 / 9 / 10 PhaseSpecs respectively; TestSemanticIndexPipelineValidates / TestLiveUpdatePipelineValidates / TestEvalPipelineValidates PASS. Cycle / dup / missing-dep coverage via TestValidate_Rejects{Cycle,DuplicateID,MissingDep}. DAG-04 marker present at top of newDaemon. Bootstrap remains imperative. |
| 5   | Every new `semantic_index.*` config key resolves under the documented 4-layer precedence | VERIFIED | 65 leaf defaults in defaults.go; SerenaConfig.SemanticIndex koanf tag wired; TestLoad_SemanticIndexDefaults + TestLoad_SemanticIndexPrecedence PASS. Five SPEC §25 floats wrapped float64(...) per koanf gotcha. |

**Score:** 5/5 success criteria fully verified.

## Explicit Closure Verdicts

| Finding | Verdict | Source |
| ------- | ------- | ------ |
| SC-1 — `get_health` semantic_store wiring | **CLOSED** | `internal/kernel/health/tools.go:19-113`, `internal/daemon/daemon.go:365` + `:564-588`; 7 tests in `tools_semantic_test.go` PASS |
| CR-01 — path-traversal rejection (incl. absolute paths) | **CLOSED** | `internal/semantic/store/duckdb.go:170-177`; `TestOpen_RejectsParentTraversal` (4 sub-cases) + `TestOpen_RejectsAbsolutePath` (2 sub-cases) PASS; commits ef5c3fa3 + 37e11ad8 |
| CR-02 — schema-probe 5s timeout | **CLOSED** | `internal/semantic/store/duckdb.go:288-289`; awk classifyExisting block has 2 `context.WithTimeout(ctx, 5*time.Second)` occurrences (ping + schema probe); commit 361bb7df |
| WR-01 — transactional `applyMigration001` | **CLOSED** | `internal/semantic/store/migrations.go:15-26` defines `applyStatementsTx` (BeginTx + deferred Rollback + Commit); `applyMigration001` (line 57-58) delegates; `TestApplyMigration001_RollsBackOnFailure` + `TestApplyMigration001_UsesTransactionalHelper` PASS; commits 3be9ec80 + 3716a8c1 + c637f0f3 |
| WR-02 — noduckdb analyzer prefix (exact-package + /-prefix) | **CLOSED** | `internal/lint/noduckdb/analyzer.go:32`; siblingpkg fixture imports both bare and /v2 sibling forms; `TestAnalyzer_AllowsSiblingNamespacePackage` PASSES; commits 66e4c31f + f52c86f4 + 269a51ab |
| WR-03 — transient-error retry classifier (with persistent-transient hard-fail) | **CLOSED** | `internal/semantic/store/duckdb.go:74-96` (classifier) + `:60-67` (decideReopenRetry) + `:240-245` (Open hot-path); hard-fail message `"transient errors persist across retry"` present (1 occurrence); `TestClassifyReopenError` (8 cases) + `TestClassifyReopenError_ETXTBSY` + `TestDecideReopenRetry_PersistentTransientHardFails` (6 cases) PASS; commits 3fdd5d0c + 6d20469f + b115fa00 |
| BL-01 — absolute-path follow-up to CR-01 | **CLOSED** | `filepath.IsAbs` guard at `duckdb.go:170` runs before the `..` segment scan; `internal/semantic/config.go` doc now matches code; `TestOpen_RejectsAbsolutePath` PASSES; commits 5d81645f + 37e11ad8 |

### Boundary Integrity

`grep -r 'github.com/agenthands/helix/internal/semantic' internal/kernel/health/*.go` returns ZERO matches outside doc comments — the kernel package remains free of `internal/semantic` imports. The `SemanticStoreProbe` interface is the seam; the daemon-side `semanticStoreProbe` adapter (defined in daemon.go:564 alongside the existing imports) satisfies it.

### WR-NEW / IN-NEW Follow-up Findings (from 57-REVIEW-57-05.md)

| Finding | Verdict | Evidence |
| ------- | ------- | -------- |
| WR-NEW-01 — Probe.Reason raw error leak | CLOSED | `tools.go:50-81` defines closed-enum constants (`probe_timeout` / `nil_handle` / `db_error` / `unknown`) and `classifySemanticProbeError`; raw `err.Error()` is logged via `slog.Warn` but NEVER returned in the JSON `Reason` field; `TestSemanticStoreStatus_Unhealthy_*` (NilHandle / ProbeTimeout / generic) PASS. Commit 02c7313b. |
| WR-NEW-02 — persistent-transient quarantines clean DB | CLOSED | `decideReopenRetry` hard-fails (returns `reopenRetryHardFail`) when both attempts classify as transient — duckdb.go:240-242 surfaces "transient errors persist across retry; refusing to quarantine clean DB" instead of renaming the file. `TestDecideReopenRetry_PersistentTransientHardFails` PASSES across all 4 transient sentinels + corruption + unknown. Commit b115fa00. |
| WR-NEW-03 — analyzer slash-boundary reverse-direction lock | CLOSED | siblingpkg fixture now imports BOTH `duckdb-go-sibling` AND `duckdb-go-sibling/v2`; the single test asserts NO diagnostic on either form, locking both directions of the slash boundary. Commit 269a51ab. |
| IN-NEW-01 — Store.DB() cross-build contract drift | CLOSED | duckdb.go:444-454 documents `nil iff store not open; on windows/arm64 always nil because no DuckDB build`; duckdb_winarm64.go:37-42 cross-references the contract. Commit 8a0152f2. |
| IN-NEW-02 — Probe error path nil-handle disambiguation | CLOSED | daemon.go:581-588 — unreachable `db == nil` branch removed; surviving `p.s == nil` branch wrapped with `serr.ErrUnsupported` so the closed-enum classifier in kernel/health can disambiguate via `errors.Is`. Commit b453b91b. |
| IN-NEW-03 — migration test anchor to applyMigration001 | CLOSED | `TestApplyMigration001_UsesTransactionalHelper` reads `migrations.go` at test time and asserts `applyStatementsTx` appears ≥ 2 times (definition + applyMigration001 call). Commit c637f0f3. |

### Out-of-scope follow-ups (intentionally deferred per 57-05 option-C scope)

These were called out in the original 57-REVIEW.md but explicitly excluded from 57-05's scope. They are NOT new gaps introduced by 57-05; they are pre-existing items intentionally pushed out:

- **WR-04** — `quarantineAndRebuild` symlink-refusal comment misleads (check is on source, not target). Pre-existing; not regressed by 57-05.
- **WR-05** — `QueryEffective*` uses `any` for query/result types. Schema 1 has no write paths; typed lock will land with P59/P60 write-path work.
- **WR-06** — `kahnSort` redundant zero-init. Cosmetic / dead code.
- **WR-07** — `findCycle` comment/code drift. Cosmetic.

These do not contradict any P57 ROADMAP success criterion. No new follow-up phase is required for v1.10 closure.

## Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/kernel/health/tools.go` | SemanticStoreProbe + SemanticStoreStatus + ComputeSemanticStoreStatus + closed-enum reason | VERIFIED | Lines 19-113; ZERO imports of internal/semantic; envelope wraps existing HealthReport additively |
| `internal/kernel/health/tools_semantic_test.go` | All-three-state coverage + closed-enum reasons | VERIFIED | 7 test funcs covering ready / disabled / nil-probe / unhealthy / nil-handle / probe-timeout / json-shape — all PASS |
| `internal/daemon/daemon.go` | SemanticStore() accessor + semanticStoreProbe adapter wrapping serr.ErrUnsupported on nil | VERIFIED | Accessor at :558; adapter at :564-588 (Probe wraps `p.s == nil` with `: %w` of serr.ErrUnsupported); RegisterTools wired at :365 |
| `internal/semantic/store/duckdb.go` | filepath.IsAbs + `..` segment refusal + 5s schema probe + reopen classifier + persistent-transient hard-fail + DB() doc | VERIFIED | Lines 60-96 (classifier + retry decision), 170-177 (path rejection), 219-251 (Open retry hot-path with hard-fail message), 288-289 (5s queryCtx), 444-460 (DB() cross-build doc) |
| `internal/semantic/store/duckdb_winarm64.go` | DB() stub + cross-reference | VERIFIED | Lines 37-42 |
| `internal/semantic/store/migrations.go` | applyStatementsTx (BeginTx/Commit/Rollback) + applyMigration001 delegate | VERIFIED | Lines 9-26 + 57-58 |
| `internal/semantic/store/migrations_test.go` | RollsBackOnFailure + UsesTransactionalHelper | VERIFIED | Both tests PASS |
| `internal/semantic/store/duckdb_internal_test.go` | classifyReopenError + decideReopenRetry coverage | VERIFIED | TestClassifyReopenError (8) + ETXTBSY + TestDecideReopenRetry_PersistentTransientHardFails (6) — all PASS |
| `internal/lint/noduckdb/analyzer.go` | exact-package + slash-prefix matcher | VERIFIED | Line 32 |
| `internal/lint/noduckdb/testdata/src/siblingpkg/imports.go` | bare AND /v2 sibling imports, no //want | VERIFIED | Both forms imported; explanatory comment in place |
| `internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go-sibling{/,/v2/}sibling.go` | sibling stub fixtures | VERIFIED | Both directories exist |
| `internal/phasegraph/{phase,dag,validate,run,shutdown,dot}.go` | DAG-01 type set | VERIFIED | (unchanged from prior verification) |
| `internal/phasegraph/pipelines/{semantic,live,eval}.go` | 12/9/10 PhaseSpecs | VERIFIED | (unchanged from prior verification) |

## Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `Daemon.SemanticStore()` | `get_health` MCP tool | `semanticStoreProbe` → `health.SemanticStoreProbe` interface → `ComputeSemanticStoreStatus` → JSON envelope | **WIRED** | Confirmed at `daemon.go:365` with ZERO `internal/semantic` imports inside `internal/kernel/health/` (boundary preserved). |
| `internal/semantic/store.Open` | `serr.ErrInvalidArgs` | path-rejection error wrapping | WIRED | `errors.Is(err, serr.ErrInvalidArgs)` true for both abs + `..` paths |
| `applyMigration001` | `applyStatementsTx` | direct delegation | WIRED | One-line body |
| `Open` reopen path | `classifyReopenError` + `decideReopenRetry` | switch on enum class + persistent-transient hard-fail | WIRED | Matches plan + WR-NEW-02 fix |
| (existing) daemon step 6b | `semanticstore.Open` | `cfg.SemanticIndex.Enabled` gate | WIRED | (unchanged) |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Targeted package suite (semantic/store + kernel/health + daemon + lint/noduckdb + config + phasegraph) | `go test -short -timeout 180s ./internal/semantic/store/... ./internal/kernel/health/... ./internal/daemon/... ./internal/lint/noduckdb/... ./internal/config/... ./internal/phasegraph/...` | all packages OK | PASS |
| CGO + integration daemon TestDaemon_SemanticStore_* | `go test -tags 'cgo integration' -timeout 240s -run TestDaemon_SemanticStore ./internal/daemon/...` | ok | PASS |
| Custom noduckdb analyzer on full source | `go install ./cmd/vet-noduckdb && go vet -vettool=$GOPATH/bin/vet-noduckdb ./internal/... ./cmd/...` | exit 0 (only pre-existing tree-sitter swift `TOKEN_COUNT` macro warning) | PASS |
| Project-wide build | `go list ./... \| grep -v tmp/ \| xargs go build` | exit 0 | PASS |
| Project-wide vet (excluding tmp fixtures) | `go vet ./internal/... ./cmd/...` | exit 0 (only pre-existing swift warning) | PASS |
| Path-rejection focused tests | `go test -run "TestOpen.*Reject"` | TestOpen_RejectsParentTraversal (4) + TestOpen_RejectsAbsolutePath (2) — all PASS | PASS |
| WR-NEW-02 hard-fail message presence | `grep -c "transient errors persist across retry" internal/semantic/store/duckdb.go` | 1 | PASS |

## Requirements Coverage

| Requirement | Description | Status | Evidence |
| ----------- | ----------- | ------ | -------- |
| STORE-01 | Store opens at daemon start; quarantine + rebuild on corruption | SATISFIED | Three-tier open verified; quarantine path via `quarantineAndRebuild`; `TestDaemon_SemanticStore_FromYAML_Enabled` passes; **NEW**: get_health surfaces readiness |
| STORE-02 | enabled=false → semantic-dependent tools return Kind: Unsupported | SATISFIED (foundation; downstream deferred to P64) | `_FromYAML_Disabled` passes; semanticStoreProbe.Probe returns `serr.ErrUnsupported` when store nil |
| STORE-03 | Schema versioning + forward-incompat reindex + migration | SATISFIED | `CurrentSchemaVersion = 1`; `reasonSchemaForwardIncompat` quarantine path; transactional `applyMigration001` (NEW) |
| STORE-04 | Effective-read API surface returns snapshot ⊕ overlay − tombstones | PARTIAL — API surface present, write paths deferred to P59/P60 by design | `QueryEffective*` methods return empty results per Schema 1 contract (WR-05 typing concern formally deferred) |
| STORE-05 | 4-layer config precedence | SATISFIED | (unchanged) |
| STORE-06 | Vet/lint rule blocks duckdb-go imports outside store/ | SATISFIED | exact-package + slash-prefix matcher (NEW); siblingpkg fixture locks both directions |
| DAG-01 | phasegraph library | SATISFIED | (unchanged) |
| DAG-02 | Three pipelines as typed phase DAGs | SATISFIED | (unchanged) |
| DAG-03 | Cycle/missing/duplicate fail validation | SATISFIED | (unchanged) |
| DAG-04 | Imperative bootstrap + TODO(v1.11) marker | SATISFIED | (unchanged) |

No orphaned requirements. STORE-04 remains partial-by-design (Schema 1 has no write paths until P59 lands snapshot writes and P60 lands overlay writes).

## Anti-Patterns Found

None blocking. The original WR-04 / WR-05 / WR-06 / WR-07 findings remain on the deferred follow-up list and are not regressions introduced by 57-05.

## Human Verification Required

None — all closure items are programmatically verifiable and all targeted commands exit cleanly.

## Gaps Summary

**No remaining gaps.** Plan 57-05 (six tasks: SC-1, CR-01, CR-02, WR-01, WR-02, WR-03) plus the 57-REVIEW-57-05 follow-up (BL-01, WR-NEW-01..03, IN-NEW-01..03) closes every finding raised in the original 57-VERIFICATION.md and the subsequent code review of 57-05 itself. The phase goal — "semantic fact store opens at daemon start, configuration flows through 4-layer precedence, stdlib pipeline-DAG library available" — is fully satisfied. The literal Phase 57 SC-1 ROADMAP text ("`get_health` reports the store as ready") is now honored by the new envelope in `internal/kernel/health/tools.go` plus the daemon-side `semanticStoreProbe` adapter — without breaching the kernel ↔ semantic package boundary.

WR-04..07 from the original review remain intentional out-of-scope items per the 57-05 option-C scope decision; none contradict a documented Phase 57 success criterion and they are not new gaps.

---

_Verified: 2026-05-06_
_Verifier: Claude (gsd-verifier)_
_Re-verification iteration: 2 (initial 2026-05-03 → re-verify 2026-05-06)_
