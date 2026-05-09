---
phase: 63
slug: compaction-retention
status: verified
threats_open: 0
threats_total: 12
threats_closed: 12
asvs_level: 1
created: 2026-05-07
verified: 2026-05-07
---

# Phase 63 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Phase 63 ships daemon-internal compaction & retention infrastructure. No MCP /
> network ingress, no PII, no auth surface — all threat work is on the in-process
> data plane.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Live overlay → Snapshot tx | Compactor reads from `semantic_live_overlay_*` tables and writes to `semantic_*` snapshot tables under a single per-snapshot `*sql.Tx` (encapsulated on `*Snapshot`). | repo-id-scoped per-workspace fact rows; no caller-supplied SQL fragments. |
| Compactor → Store package | `internal/semantic/compact/` calls `*Store` methods only — no direct `*sql.DB` or `duckdb-go` access. Boundary enforced by `cmd/vet-compact-uses-store`. | Typed accessor returns (`OverlayHasPendingRows`, `OverlayRowCount`, `Vacuum`, `Checkpoint`, `UpdateLastVacuumAt`). |
| Compactor goroutine ↔ Daemon errgroup | Per-workspace compactor runs under `compactBundle.Run(gctx)` joined to the daemon errgroup; activation/deactivation drives lifecycle through `gctx` cancellation. | `context.Context` + workspace identifier only. |
| Process death ↔ DuckDB ACID | If the process dies before `tx.Commit()` returns, DuckDB rollback restores prior snapshots, overlay rows, and retention deletes atomically. | None — atomicity guarantee. |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-63-01-01 | T (Tampering) | snapshot SQL inserts | mitigate | Parameterized `?` binds across `BeginSnapshot` / `WriteSnapshotFacts` / cascade DELETE / `ClearOverlayLE`; hardcoded SQL constants per table — no caller data flows into SQL string. Verified `internal/semantic/store/snapshot.go:285-396, 583-598, 644-656`. | closed |
| T-63-01-02 | I (Info disclosure) | snapshot tx isolation | mitigate | `BeginSnapshot` opens its own `*sql.Tx` (`snapshot.go:260`); pending rows carry `status='pending'` and only `CommitSnapshot` flips to `'committed'` — readers on a separate connection cannot see uncommitted rows under DuckDB ACID. Verified `snapshot.go:285,294-295,427-428`. | closed |
| T-63-01-03 | D (DoS) | unbounded `Facts` payload | accept | Caller (P63-02 compactor) gates payload size via pre-flight `OverlayRowCount` guard before invoking `BeginSnapshot`. Documented in package doc-comment `snapshot.go:40-43`. Caller-side guard verified `compactor.go:261-272`. | closed (accepted) |
| T-63-01-04 | E (Elevation) | duckdb-go import escape | mitigate | `internal/semantic/store/` is the only package allowed to import `duckdb-go`. Enforced by `internal/lint/noduckdb/analyzer.go:17-35` (`allowedPkgPrefix = ".../internal/semantic/store"`); wired via `cmd/vet-noduckdb/main.go` and `Makefile:33,38` `vet` target. | closed |
| T-63-02-01 | T (Tampering) | overlay row deletion bound | mitigate | `snap.ClearOverlayLE` parameterized `WHERE repo_id = ? AND write_epoch <= ?` bound to `capturedEpoch` only (`snapshot.go:630, 652-655`). Property test `TestCAS_InterleaveOverlayWritesWithCompaction` proves rows committed at `write_epoch > captured` survive (passes under `-race`); see `cas_property_test.go:38`. | closed |
| T-63-02-02 | T (Tampering) | retention atomicity | mitigate | `(*Snapshot).DeleteSnapshotsBeyond` runs INSIDE the snapshot tx (`snapshot.go:506-512`); compactor invokes Begin → ClearOverlayLE → DeleteSnapshotsBeyond → Commit on the same `*Snapshot` (`compactor.go:291-314`). DuckDB ACID rollback restores all snapshots if the tx fails. | closed |
| T-63-02-03 | D (DoS) | unbounded overlay → OOM during compaction | mitigate | Pre-flight `OverlayRowCount(ctx, repoID, capturedEpoch) > MaxOverlayRows` → emit `outcome="partial"` and skip (`compactor.go:261-272`). `MaxOverlayRows = 4000` default (`config.go:48`); `compactor_test.go:208-232` (`fakeOverlay{rows: 999_999}`) proves outcome=partial. Comfortably under DuckDB's 1 GiB `memory_limit` default. | closed |
| T-63-02-04 | D (DoS) | goroutine leak on workspace deactivation | mitigate (with follow-up) | `compactBundle.Run` blocks on `<-ctx.Done()` (`compact_wiring.go:128-138`); per-compactor goroutines spawn with `runCtx` (`compact_wiring.go:151-184`); errgroup wires `d.compact.Run(gctx)` (`daemon.go:827-831`). `Compactor.Run` blocks on `ctx.Done()` and stops timer (`compactor.go:134-142`). **Follow-up:** the named regression test `TestDaemon_CompactBundleSpawnsPerWorkspace` is not yet present (see [Follow-up Items](#follow-up-items) below). The structural mitigation IS in code; the test absence is tracked but does not reopen the threat under `block_on: high`. | closed |
| T-63-02-05 | D (DoS) | VACUUM extending lock window | accept | VACUUM shipped as documented no-op-by-DuckDB (planner decision); the lock-window threat does not manifest with no-op semantics. SEPARATE-tx invariant in place for future `COPY FROM DATABASE` repack: `store.Vacuum` opens its own tx (`internal/semantic/store/vacuum.go:33-43`); `internal/semantic/compact/vacuum.go:60-66` calls through `Store.Vacuum` (no piggyback on snapshot tx). Rationale documented at `vacuum.go:8-15`. | closed (accepted) |
| T-63-02-06 | I (Info disclosure) | metric label cardinality explosion | mitigate | `BlockedReason` and `outcome` are closed enums with drop-on-unknown helpers. `internal/obs/metrics.go:859-866` `blockedReasons` allowlist (6 values); `:887-895` `SemanticCompactionBlocked` drops on unknown; `:868-882` `SemanticCompactionObserve` drops on unknown outcome / negative seconds; `:900` `SemanticVacuumObserve` same. Cardinality tests at `metrics_labels_test.go:442` (`TestSemanticCompactionOutcomeCardinality`), `:484` (`TestSemanticCompactionBlockedCardinality`), `:467-471` (Vacuum). Allowlist test at `:164` primes vectors at `:215-217`. | closed |
| T-63-02-07 | E (Elevation) | duckdb-go import escape (compact pkg) | mitigate | `internal/lint/compactusesstore/analyzer.go:18-39` (`compactPkgPrefix = ".../internal/semantic/compact"`, `forbiddenImport = "github.com/duckdb/duckdb-go"`); wrapped by `cmd/vet-compact-uses-store/main.go:11`; `Makefile:33,38` `vet` target invokes `go vet -vettool=$(VETTOOL_COMPACT_USES_STORE) ./...`. Belt-and-braces over `vet-noduckdb`. | closed |
| T-63-02-08 | T (Tampering) | malformed `vacuum_interval` config | mitigate | koanf duration parse at config-load time (`internal/semantic/config.go:81-93`, `internal/config/defaults.go:104` default `"168h"`). Compactor's `time.ParseDuration` call falls back to defaults on error (`compact_wiring.go:91-103`). Zero-value-disable semantics per `config.go:70-71`. Test `internal/config/loader_test.go:488-498` (`TestLoad_MaintenanceDefaults`) verifies `VacuumEnabled=false` + `VacuumInterval="168h"`. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-63-01 | T-63-01-03 | `WriteSnapshotFacts` has no payload-size cap. The caller (P63-02 compactor) gates payload size via pre-flight `OverlayRowCount` guard before `BeginSnapshot` is invoked. P63-01 trusts the caller; the trust boundary is documented in `internal/semantic/store/snapshot.go:40-43`. Caller-side guard at `compactor.go:261-272` (`MaxOverlayRows=4000` → `outcome=partial` and skip). | Phase 63 plan author + auditor | 2026-05-07 |
| AR-63-02 | T-63-02-05 | VACUUM is shipped as a documented no-op-by-DuckDB per planner decision. The lock-window threat does not manifest with no-op semantics. All wiring/config/metric/span infrastructure in place; default `vacuum_enabled=false`. SEPARATE-tx invariant is already enforced (`store.Vacuum` opens its own tx) so a future swap to `COPY FROM DATABASE` repack does not require re-architecting. Rationale at `internal/semantic/compact/vacuum.go:8-15`, `internal/semantic/store/vacuum.go:10-14`. | Phase 63 plan author + auditor | 2026-05-07 |

---

## Follow-up Items

These items do NOT reopen any threat under `block_on: high`, but are tracked for closure parity with the rest of the v1.10 follow-up backlog.

| Item | Threat Ref | Description | Suggested Resolution |
|------|------------|-------------|----------------------|
| FU-63-01 | T-63-02-04 | Named regression test `TestDaemon_CompactBundleSpawnsPerWorkspace` is not yet present in `internal/daemon/wiring_test.go`. The structural mitigation (`compactBundle.Run` ctx-bound + per-compactor goroutines on `runCtx` + errgroup wiring) is in code, but a future refactor that breaks ctx propagation would not be caught by CI. `63-VALIDATION.md:75` carries an unchecked `[ ]`. | (a) Implement the test (activate 2 workspaces, assert `len(d.compact.subs) == 2`, deactivate, assert goroutine count returns to baseline), or (b) document the deferral explicitly in 63-VALIDATION.md / SUMMARY.md "deferred items" with the same rigor as COMPACT-05. |
| FU-63-02 | n/a | Long-repo bench fixture (COMPACT-03 100-file × 1000-cycle, growth_factor < 2.0×) deferred at SUMMARY.md:82 — local-only per MEMORY rule, outside merge-block scope. | Land in a follow-up closure when a bench-only branch is opened. |
| FU-63-03 | n/a | Kill-mid-compact subprocess test (COMPACT-05) deferred at SUMMARY.md:83, 139 — single-tx D-01 invariant is held by DuckDB ACID `tx.Commit()`; subprocess + sentinel-file orchestration is plumbing for explicit GREEN proof. | Land if the verifier requires explicit subprocess GREEN evidence. |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-05-07 | 12 | 12 | 0 | gsd-security-auditor (Phase 63 verification pass) |

**Audit notes (2026-05-07):**
- Register origin: authored at plan time (both `63-01-PLAN.md` and `63-02-PLAN.md` contained complete `<threat_model>` blocks with STRIDE category + disposition + mitigation per threat).
- Mode: verify mitigations exist (no retroactive STRIDE).
- Outcome: 12/12 structurally CLOSED with file:line evidence per threat. 1 MEDIUM warning (T-63-02-04 missing regression test) tracked as FU-63-01 — does not reopen the threat under the project's `block_on: high` policy because the structural mitigation is verified in code.
- No high-severity threats open → phase advancement not blocked.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log (AR-63-01, AR-63-02)
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-05-07
