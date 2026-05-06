# Phase 62: Graph Engine, Ranking & Type Resolution - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-06
**Phase:** 62-graph-engine-ranking-type-resolution
**Areas discussed:** PageRank engine reuse vs new, graph_version advance policy, PageRank trigger / scheduling, Type resolver placement + lang scope

---

## PageRank engine

| Option | Description | Selected |
|--------|-------------|----------|
| New engine, generic over NodeID | Fresh generic[T] PageRank in internal/semantic/graph/. Repomap engine untouched. ~80 LOC duplicated; future projections drop in trivially. | |
| Lift + refactor: shared engine in internal/graph/ | Extract a generic engine consumed by both repomap and semantic. Single algorithm to audit. Cost: touches repomap. | ✓ |
| Reuse repomap engine via adapter | Keep repomap engine + determinism patch; semantic calls it via adapter. Cheapest LOC. Couples two layers; loses NodeID type safety. | |

**User's choice:** Lift + refactor — shared engine at `internal/graph/`.
**Notes:** User explicitly accepted the migration cost in repomap to consolidate the algorithm into one auditable locus.

### Sub-choice: Engine home + repomap migration contract

| Option | Description | Selected |
|--------|-------------|----------|
| internal/graph/ + repomap migrates with byte-equal guarantee | Existing pagerank_test.go scores must remain byte-identical post-migration. | |
| internal/graph/ + repomap migrates, tests re-pinned | One-time accept of new deterministic outputs. | ✓ |
| internal/semantic/graph/ + repomap stays put | "Shared" only in name; defer migration. | |

**User's choice:** internal/graph/ + tests re-pinned.
**Notes:** Pragmatic — avoids forcing the determinism patch to preserve a possibly-undefined existing ordering.

### Sub-choice: Generic constraint

| Option | Description | Selected |
|--------|-------------|----------|
| comparable + Stringer | Sort by .String() — works for any type. | |
| cmp.Ordered (Go 1.21+) | Direct sort.Slice; faster; covers string + uint64. | ✓ |
| Custom Less function | Most flexible; more boilerplate at call sites. | |

**User's choice:** cmp.Ordered.

---

## graph_version advance policy

### Sub-choice: graph_version vs current_epoch relationship

| Option | Description | Selected |
|--------|-------------|----------|
| Separate counters | graph_version is its own column; advances only on edge changes. current_epoch keeps Phase 60 D-04 CAS role. | ✓ |
| Single counter (graph_version = current_epoch) | Drop current_epoch; bump per tx. Simpler schema; many false-stale markers. | |
| Single counter with smart-stale flag | Per-projection last-edge-change-version sidesteps false stale. More logic, less schema. | |

**User's choice:** Separate counters.
**Notes:** Cleaner semantics — graph_version reflects "would PR scores change", current_epoch reflects "any write happened". Phase 63 compaction CAS is unchanged.

### Sub-choice: Repair trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Edge writes + symbol stable-key/signature change | Per SPEC §17.2 verbatim. Symbol-body-only edits do NOT bump. | ✓ |
| Any overlay write (broadest) | Every commit triggers repair. Higher false-stale rate. | |
| Edge writes only | Symbol stable-key/signature changes deferred to periodic rescan. | |

**User's choice:** SPEC §17.2 invalidation rules verbatim.

---

## PageRank trigger / scheduling

### Sub-choice: Scheduler shape

| Option | Description | Selected |
|--------|-------------|----------|
| Idle debounce + on-demand at tool call | Background goroutine with debounce; reads return stale and queue repair; longer idle for full recompute. | ✓ |
| Sync repair after each enrichment commit | Phase 61 worker calls ApplyRepair post-commit. Always-fresh; re-introduces backpressure. | |
| On-demand only (lazy) | First-call latency on stale graph; cheapest. | |
| Phase 60 coalescer-driven | Hooks into existing live coalescer. Cross-package coupling. | |

**User's choice:** Idle debounce + on-demand-stale-return.

### Sub-choice: Repair frontier policy

| Option | Description | Selected |
|--------|-------------|----------|
| 1-hop from changed nodes | Direct neighbors over current edges; >5000 falls back to all-stale + full recompute. | ✓ |
| k-hop with k=2 | More distant ripple; ~10x larger frontier. | |
| Topological-distance up to convergence | Most accurate; unbounded compute. | |
| All-nodes incremental | Skip the cap; rejects GRAPH-04 acceptance. | |

**User's choice:** 1-hop with all-stale fallback above 5000.

---

## Type resolver placement + lang scope

### Sub-choice: Resolver layout

| Option | Description | Selected |
|--------|-------------|----------|
| Per-language under internal/semantic/types/<lang>/ + shared core | Mirrors extract/ layout; clean per-lang isolation. | ✓ |
| Single tree-walking resolver consuming fact tables | Language-agnostic; quirks become if-branches. | |
| Co-locate inside extract/<lang>/ | Blurs extract vs resolve cadences. | |

**User's choice:** Per-language under internal/semantic/types/<lang>/ + shared core.

### Sub-choice: v1 lang scope

| Option | Description | Selected |
|--------|-------------|----------|
| Go + TS/JS + Python full ladders | Match Phase 59 first-class extraction tier. | |
| Go-only in v1 | Smallest risk; closes ~1/3 of dynamic-language target. | |
| Go + TS/JS only; Python deferred | Lower v1 risk; partial Phase 64 coverage. | |
| All Phase 59 first-class + best-effort stubs for PHP/Ruby/Java | Three full ladders + zero-confidence stubs for everything else. Most ambitious. | ✓ |

**User's choice:** All Phase 59 first-class + best-effort stubs (Java short-circuits on Phase 61 LSP-confirmed facts; PHP/Ruby always emit 0.20 unresolved).

### Sub-choice: Fixpoint scope

| Option | Description | Selected |
|--------|-------------|----------|
| Per-package / compilation unit | Go: dir; TS: tsconfig project; Python: pkg via __init__.py. | ✓ |
| Per-file | Cheapest; misses cross-file chains in same package. | |
| Per-repo | Most accurate; expensive; only feasible at snapshot-rebuild cadence. | |
| Per-package with per-file fast-path | Try file first; escalate to package on cross-file ref. More logic. | |

**User's choice:** Per-package / compilation unit.

### Sub-choice: Comment edge gate (TYPES-03/04)

| Option | Description | Selected |
|--------|-------------|----------|
| Two-phase emission: 0.60 → in-place upgrade to 1.00 on LSP confirm | Comment edge written immediately; merged when matching LSP edge lands. Clean confidence semantics. | ✓ |
| Suppress comment edges when LSP edge exists for same (src,dst,kind) | Comment edges only fill gaps. Order-of-operations sensitive. | |
| Always emit both; consumer dedupes by max-confidence | Simplest emission; more rows; readers more complex. | |

**User's choice:** Two-phase emission with in-place upgrade.

---

## Claude's Discretion

The following implementation decisions were left to the planner / executor (documented in CONTEXT.md `### Claude's Discretion`):

- Package layout for the semantic-side rank engine call site (`internal/semantic/graph/` vs nested under `store/`).
- Goroutine-lifecycle hookup pattern (mirrors Phase 60 coalescer + Phase 61 worker).
- Metric label cardinality (closed-enum bounded labels per `internal/obs/`).
- Trace span naming (matches SPEC §28.2 conventions).
- Score-row write path (`tx.UpsertGraphScores` helper inside Phase 60's `BeginOverlayTx`).
- `Ranker` and `Resolver` API shape (consumed by Phase 64 MCP tools).
- Plan layout: 5-plan suggestion in CONTEXT.md `### Claude's Discretion`; planner may bundle/split.

## Deferred Ideas

Captured in CONTEXT.md `<deferred>`:

- Multi-projection PageRank (six other §18.1 projections deferred).
- Score fusion across projections (§18.5 deferred).
- Cluster MCP tools (`get_cluster_map`, `explain_cluster`) — v1.10.x.
- Cluster algorithms beyond weak-components.
- k-hop / convergence-distance frontier policies.
- Hard-cancel preemption of in-flight full recompute.
- Cross-package fixpoint resolution.
- Full PHP/Ruby type resolution (stub-only in v1).
- Adaptive priority promotion for `semantic_pending` files based on rank.
- Receipts / guardrails on rank-derived edges (Phase 66).
- MCP tool surface (Phase 64).
- Cross-repo / multi-workspace ranking.
- Custom-Less variant of the generic engine.

## Anomaly noted

`gsd-sdk init.phase-op 62` and `gsd-sdk roadmap.get-phase 62` both report `phase_found: false`; `gsd-sdk roadmap.analyze` returns `phase_count: 0` for the entire repo. The SDK's roadmap analyzer appears unable to parse the v1.10 phase block (`### v1.10 Live Semantic Index ...` followed by a flat `- [ ] Phase N` list outside `<details>` tags). Phase 62 is clearly defined in `.planning/ROADMAP.md:159` and `.planning/milestones/v1.10-ROADMAP.md:131-141`. This discussion proceeded against the file content directly. Worth a forensics or SDK fix in a follow-up.
