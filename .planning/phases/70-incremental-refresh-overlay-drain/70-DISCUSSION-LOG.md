# Phase 70 — Discussion Log

**Date:** 2026-05-15
**Facilitator:** /gsd-discuss-phase

## Areas Discussed

### 1. Tool target — which tool's behavior changes?

**Why ambiguous:** ROADMAP goal text says `refresh_semantic_graph` with `mode:"incremental"`, but `refresh_semantic_graph` (read+) has no `mode` arg today, and the cited code anchor (`collectCandidatePaths` at `internal/daemon/semantic_wiring.go:1466-1480`) is only called by `index_semantic_graph(mode="incremental")`'s buildFn (review+/admin).

**Options presented:**
- A: `index_semantic_graph(mode=incremental)` only — narrow scope, smallest patch.
- B: Both tools share the overlay-drain seam — single source of truth, fixes a known wart in `refresh.files_updated`.
- C: Add `mode` arg to `refresh_semantic_graph` — schema change, blurs D-09/D-13 boundary.

**Pros/cons walked through interactively** before user selected.

**Selected:** B — Both tools share the seam.

**Captured as:** D1 — One seam, two consumers (index for path enumeration in incremental snapshot build; refresh for honest `files_updated` accounting).

---

### 2. Drain-seam shape — `*Store` accessor signature

**Why ambiguous:** The Phase 60 overlay layer exposes `OverlayHasPendingRows` (boolean) and `OverlayRowCount` (int) but no enumerable per-path accessor. Three plausible shapes.

**Options presented:**
- A: Since-epoch idempotent read (`OverlayChangedPathsSince(repoID, baseEpoch) → (paths, currentEpoch)`).
- B: Drain-and-mark consumed (single-consumer; mutates on read).
- C: Coalescer-side in-memory accessor (no schema change but lost across restart).

**Pros/cons walked through interactively** before user selected.

**Selected:** A — Since-epoch idempotent read.

**Captured as:** D2 — Persistent, lock-free, two-consumer-safe accessor; D3 — baseline epoch persisted on the snapshot row; D4 — fallback policy on empty seam (full-walk + bounded-label `helix_incremental_refresh_fallback_total{reason}` metric).

---

### 3. Bench harness shape — REFRESH-02 (p95 ≤ 200ms, 10k symbols, 1 file changed)

**Options presented:**
- A: Synthesized fixture in `eval/runner` (no committed corpus).
- B: Reuse Phase 64 P07 / Phase 67 eval fixture, parameterized to 10k symbols.
- C: Defer bench, ship correctness only.

**Selected:** B — Reuse Phase 64 P07 / Phase 67 eval fixture.

**Captured as:** D5 — Two test files in `internal/eval/runner/`; bench guarded by `t.Skip` under CI per `feedback_no_ci_benchmarks` project rule.

---

## Boundary Reaffirmations (Carried Forward)

- **D-09 / D-13** (refresh stays read-only on snapshot surface) — unchanged. The new accessor is read-only; existing CI grep gate in `tools_refresh.go` continues to enforce.
- **`vet-nokernel2semantic`** — new accessor lives semantic-side; no kernel imports added.
- **Phase 68 D-01 pattern** (Store accessor over handler-local cache, restart-safe) — mirrored by the new accessor.
- **Phase 69 D1 pattern** (lock-free `*Store` read accessor) — mirrored.

## Deferred / Out of Scope

- Per-projection incremental refresh — future phase.
- Streaming refresh progress over MCP — UX add-on.
- Cross-snapshot path-delta accessor — observability add-on.
- Bench-CI integration — rejected by project rule (`feedback_no_ci_benchmarks`).
- Coalescer-side in-memory cache in front of the seam — defer until profiling justifies.

## Claude's Discretion (downstream)

- Exact schema delta for persisting `base_overlay_epoch` (new column on `snapshots` vs sibling key).
- Exact sentinel for the `overlay_rotated` fallback reason (third return value vs typed error).
- Whether the Phase 64 P07 fixture builder is already parameterizable for symbol count, or whether a small extraction lifts the const to an arg.
- Whether `OnWorkspaceChanged` blocks on coalescer flush (planner to confirm against Phase 60 D-01 contract; otherwise add a small post-call wait before reading the seam in `refresh_semantic_graph`).
