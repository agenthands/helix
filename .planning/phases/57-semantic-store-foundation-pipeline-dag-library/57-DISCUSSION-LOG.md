# Phase 57: Semantic Store Foundation + Pipeline DAG Library - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-03
**Phase:** 57-semantic-store-foundation-pipeline-dag-library
**Areas discussed:** Schema version shape, Track ordering / plan split, Quarantine observability (incl. reason enum), Config coverage test

---

## Schema version shape

| Option | Description | Selected |
|--------|-------------|----------|
| Integer monotone (Recommended) | Single `INTEGER NOT NULL` column. Forward-incompatible bumps move the version forward; backward-compatible bumps increment too — the migrations table records which version-jumps are reindex vs migrate. Simplest; lowest comparison logic. | ✓ |
| (major, minor) tuple | Two columns. Major bump = reindex; minor bump = in-place. Distinction encoded in the version itself (no migrations registry needed for the kind). More expressive but introduces tuple-compare semantics. | |
| Single string semver-like | `schema_version TEXT NOT NULL` storing 'M.m'. Same semantics as the tuple but parsed at load. Familiar but parsing+formatting on every read. | |

**User's choice:** Integer monotone (Recommended)
**Notes:** Confirmed via the rendered preview that the migrations registry is the auditable source of truth for InPlace vs Reindex. Phase 57 ships schema_version = 1.

---

## Track ordering / plan split

| Option | Description | Selected |
|--------|-------------|----------|
| 4 plans, parallel-ready (Recommended) | P01=phasegraph, P02=semantic store, P03=config keys, P04=vet-tool. Each plan independent; gsd-execute-phase can run waves in parallel. Highest throughput. | ✓ |
| 3 plans (merge T3 into T2) | P01=phasegraph, P02=semantic store + config keys, P03=vet-tool. Smaller surface per plan but P02 grows. | |
| Sequential 2 plans | P01=phasegraph, P02=everything semantic-store related. Lowest plan count, but P02 becomes the largest plan in v1.10 history. | |
| 5 plans (split T2) | Split T2 into skeleton/open/effective-read across three plans. More commits, more checkpoints, more friction. | |

**User's choice:** 4 plans, parallel-ready (Recommended)
**Notes:** Dependency graph confirmed via preview: P01 independent; P03 → P02; P04 → P02. Executor runs P01 in wave 1 alongside P02; P03 and P04 in wave 2.

---

## Quarantine observability

| Option | Description | Selected |
|--------|-------------|----------|
| Slog event + counter metric (Recommended) | Structured `slog.Warn` event + new `helix_semantic_store_quarantine_total{workspace_label,reason}` counter. Visible in get_health (P65), Prometheus dashboards, and traces. | ✓ |
| Slog event only | Structured slog event but no metric. Cheaper but get_health and dashboards have nothing to count over time. | |
| Stderr line only | Plain stderr, no slog, no metric. Inconsistent with v1.2 observability foundation. | |

**User's choice:** Slog event + counter metric (Recommended)
**Notes:** Counter ships in P02 alongside the store package. Phase 65 will surface it through `get_health`.

### Follow-up: `reason` label enum

| Option | Description | Selected |
|--------|-------------|----------|
| 4 values (Recommended) | `corrupt_file`, `schema_forward_incompat`, `schema_unreadable`, `unknown`. Tight, covers all current paths. | ✓ |
| 2 values (corruption / schema) | Collapse to `corruption` and `schema_mismatch`. Less diagnostic specificity. | |
| Open string (no allowlist) | Violates v1.2 bounded-label invariant; PromQL validator would reject. Not viable. | |

**User's choice:** 4 values (Recommended)
**Notes:** Allowlist is added to the existing v1.2/v1.9 cardinality allowlist + PromQL validator.

---

## Config coverage test

| Option | Description | Selected |
|--------|-------------|----------|
| Per-feature test (Recommended) | Add `TestLoad_SemanticIndexDefaults` mirroring `TestLoad_ObservabilityDefaults` style + `TestLoad_SemanticIndexPrecedence` covering 3–4 representative keys. Matches house style; tightest scope. | ✓ |
| Generic 'all keys have defaults' matrix | New `TestConfig_AllKeysHaveDefaults` walking `SerenaConfig` via reflection. Bigger ROI long-term, but a v1.10 architectural change beyond Phase 57's scope. | |
| Per-feature + add a TODO for the matrix | Ship per-feature test now; drop a `// TODO(v1.11): generic key-coverage matrix` comment. Conservative middle ground. | |

**User's choice:** Per-feature test (Recommended)
**Notes:** Both tests live in P03 (the plan that adds the keys). No generic matrix in v1.10 — explicitly deferred.

---

## Claude's Discretion

The user did not select "you decide" on any question, but research-locked these implementation details (no user input requested because the recommendation is unambiguous and the user's prior decisions in v1.9 already establish the pattern):

- CGO=0 stub mirrors `internal/repomap/extractor_nocgo.go` (Phase 51.1) verbatim.
- `internal/phasegraph/` is non-generic; `PhaseOutput` is `interface{}`, `PhaseDeps` is `map[PhaseID]any`.
- Daemon `OpenSemanticStore` step lives at step 6b (between step 6a CGO refusal and step 6 diagnostic store).
- Vet-tool is standalone (`cmd/vet-noduckdb/main.go`); no existing analyzer to co-locate.
- `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)` marker placed at top of `newDaemon`.

## Deferred Ideas

- Generic `TestConfig_AllKeysHaveDefaults` reflection-walk matrix — explicitly rejected for v1.10 scope; future phase if revisited.
- CGO=1 release-channel split (`duckdb-go-bindings` lacks windows-arm64 prebuilt libs) — Phase 58 or later release-planning concern.
- Bootstrap migration to `phasegraph.RunPhaseGraph(BootstrapPhases)` — v1.11 phase per DAG-04.
- Cluster MCP tools, multi-projection PageRank, P1 retrieval companions, comment-fallback type resolution — v1.10.x backlog.
- `get_health` semantic enrichment — Phase 65 (Strangler Fig).
- Snapshot retention / VACUUM scheduling — Phase 63 (Compaction & Retention).
