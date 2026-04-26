# Phase 53: obs-metrics-gaps - Research

**Researched:** 2026-04-26
**Domain:** Prometheus metrics instrumentation (Go) on existing owned-registry infrastructure
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Cache-hit metric shape (lspool + repomap)**
- D-01: Single counter family per subsystem with `result={hit,miss}`, NOT hits-only counters. Final names:
  - `serena_lspool_cache_total{language, result, scope}`
  - `serena_repomap_cache_total{language, result}`
  - This deliberately RENAMES the families from the roadmap text. PromQL hit-rate is `rate(family{result="hit"}[5m]) / rate(family[5m])`.
- D-02: Labels:
  - lspool: `{language, result, scope}` — `result` ∈ {hit, miss}; `scope` ∈ {clean, dirty, crashed}.
  - repomap: `{language, result}` — `result` ∈ {hit, miss}.
  - `result` is a NEW closed-enum carve-out in `metrics_labels_test.go`. `scope` is a second carve-out applied only to the lspool family.
- D-03: Cardinality bounds — language ≤ 52, so 52 × 2 × 3 = 312 max series for lspool, 52 × 2 = 104 for repomap. Cardinality test asserts these caps.

**Session lifecycle (`serena_session_lifecycle_total`)**
- D-04: Closed phase set: {activate, deactivate, timeout, shutdown}. Each phase fires once per lifecycle event.
- D-05: Labels `{language, phase}`. `language` already in `AllowedLabels`. `phase` is a NEW closed-enum carve-out.
- D-06: workspace_path is REJECTED as a label (unbounded). Per-workspace insight goes through trace exemplars in Phase 55, not metric labels.

**Edit-tool outcome (`serena_edit_outcome_total`)**
- D-07: Closed outcome set: {success, fuzzy_applied, refused_ambiguous, failed}.
  - `success` — exact-match strategy applied
  - `fuzzy_applied` — one of the fuzzy cascade strategies (whitespace_normalized / indentation_flexible / ellipsis-placeholder) succeeded
  - `refused_ambiguous` — fuzzy cascade hit multiple candidates and refused
  - `failed` — catch-all (no match, IO/write error, LSP rejection)
- D-08: `tool` is a closed allowlist of edit-tool names; unknown values dropped at sink (mirror Phase 47 D-07 `RenameStrategyInc`). Allowlist lives next to the sink and is asserted in `metrics_labels_test.go`.
- D-09: Label set `{tool, outcome}`. No `language` here — file-path → language inference inside edit tools is non-trivial.

**RepoMap extraction histogram (`serena_repomap_extract_duration_seconds`)**
- D-10: Buckets = `prometheus.DefBuckets` (consistent with `serena_tool_duration_seconds`). No custom buckets in this phase.
- D-11: Labels `{language}`. Bounded by LS catalog.

**Sink wiring boundary**
- D-12: Per-package `MetricsSink` interfaces + NoopSink (mirror Phase 11 D-08). Three new sinks:
  - `internal/repomap/metrics.go` — `repomap.MetricsSink` with `RepoMapCacheInc(language, result string)` and `RepoMapExtractObserve(language string, seconds float64)`. Plus `NoopSink`.
  - `internal/kernel/edit/metrics.go` — `edit.MetricsSink` with `EditOutcomeInc(tool, outcome string)`. Plus `NoopSink`.
  - `internal/kernel/session_metrics.go` (or under existing kernel file — researcher to choose) — `kernel.SessionMetricsSink` with `SessionLifecycleInc(language, phase string)`. Plus `NoopSink`.
  - Each consumer package depends ONLY on its own sink interface, never on `internal/obs` directly.
- D-13: Helper methods on `*obs.Metrics` satisfy all four sinks: `LSPoolCacheInc(language, result, scope string)`, `RepoMapCacheInc`, `RepoMapExtractObserve`, `EditOutcomeInc`, `SessionLifecycleInc`. lspool's existing `MetricsSink` is EXTENDED with `LSPoolCacheInc` (the only breaking change to that frozen interface).
- D-14: `internal/daemon/wiring_test.go` adds compile-time assertions for all four sinks.

**Noop-default invariant**
- D-15: All new vectors constructed unconditionally in `newMetrics()`. Noop providers still hand consumers a real `*obs.Metrics`; vectors go unscraped. NO `if metrics != nil` branches at call sites.

**CI / test gates**
- D-16: New tests:
  1. Extend `TestMetricsLabelsAllowlist` with carve-outs for `result`, `scope`, `phase`, `outcome`, edit-`tool`.
  2. New cardinality test asserting max series per family.
  3. `wiring_test.go` block per D-14.
  4. Per-subsystem unit tests asserting the right vector & labels are touched on emission.

### Claude's Discretion

- Specific helper-method names (`LSPoolCacheInc`, `RepoMapCacheInc`, `RepoMapExtractObserve`, `EditOutcomeInc`, `SessionLifecycleInc`) — fixed in CONTEXT.md as a starting point; planner may rename if a stronger convention emerges in research.
- Final closed allowlist of edit-tool names — researcher to enumerate from `internal/kernel/edit/` and `internal/kernel/fileops/`.
- Exact location of `kernel.SessionMetricsSink` (new file vs existing) — left to researcher / planner.

### Deferred Ideas (OUT OF SCOPE)

- Per-workspace lifecycle insight via trace exemplars (Phase 55+).
- Custom repomap histogram buckets (post-Phase 54 if dashboards demand).
- Finer edit-failure outcome breakdown ({not_found, io_error, lsp_error}) — single bucket for now.
- Roadmap text reconciliation for `*_cache_total` rename — planner picks form during plan 53-01.
- Trace integration of new metrics (Phase 55 audit if needed).
- Migrating existing v1.2 metric families (no breaking renames of `serena_tool_*`, `serena_lspool_*`, `serena_rename_strategy_total`).
- Grafana dashboards / runbooks (Phase 54).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-03 | Close v1.2 metrics gaps — add cache hit-rate (lspool + repomap), RepoMap extraction latency histogram, session lifecycle counters, and edit-tool outcome counters with bounded labels. | All five metric families specified below; emission points identified in lspool, repomap, kernel, edit, fileops; bounded-label discipline + cardinality test pattern reused from Phase 11 D-13 / Phase 47 D-07. |
</phase_requirements>

## Summary

Phase 53 adds five Prometheus metric families to the existing `*obs.Metrics` infrastructure that v1.2 (Phase 11) shipped. The owned-registry, AllowedLabels-array, closed-enum carve-out, NoopSink, and `wiring_test.go` compile-time assertion patterns are all in place — this phase is a straight expansion with no architectural shift.

The five families and their emission sites are fully discoverable in the codebase:
- `serena_lspool_cache_total` emits at `lspool/pool.go:117` (clean shared lease = hit/clean) and the spawn-after-no-match branch at `pool.go:137` (miss/clean) plus the dirty branch (miss/dirty) and circuit-blocked branch (miss/crashed at `pool.go:127-129`).
- `serena_repomap_cache_total` and `serena_repomap_extract_duration_seconds` emit inside the `extractFn` closure at `internal/skill/repomap/skill.go:366` (the SQLite mtime-invalidation logic itself lives in `repomap/cache.go:GetOrExtract`).
- `serena_session_lifecycle_total` emits at four call sites: `kernel.go:ActivateWorkspace` (activate, lazy & explicit paths share this), the gRPC `DeactivateWorkspace` handler at `daemon.go:649` (deactivate), the idle-TTL path at `pool.go:checkTTLs` (timeout — though semantically this is worker timeout, not session — see Open Question 1), and the daemon shutdown sweep (shutdown).
- `serena_edit_outcome_total` emits inside each edit/fileops tool handler after the fuzzy/exact result is known. The fuzzy strategy → outcome mapping is mechanical: `StrategyExact → success`, `StrategyWhitespace|StrategyIndentationFlex → fuzzy_applied`, ambiguity error → `refused_ambiguous`, all other errors → `failed`.

**Primary recommendation:** Implement in three tightly-scoped plans: (53-01) extend `*obs.Metrics` with the five new vectors + helper methods + carve-outs in `metrics_labels_test.go` + cardinality test; (53-02) define the three new per-package sink interfaces and emit at the discovered call sites; (53-03) update `internal/daemon/wiring_test.go` with the four `var _` assertions + USAGE.md Observability section + roadmap text reconciliation.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Owned `*prometheus.Registry` and metric vector construction | `internal/obs` | — | Phase 11 froze prometheus/client_golang import to this package only (D-08 Phase 11). |
| LSP pool cache emission (hit/miss/scope) | `internal/kernel/lspool` | `internal/obs` (sink) | Pool already owns the share-until-dirty / circuit decisions where hit/miss arises. |
| Tag cache emission + extraction histogram | `internal/skill/repomap` (call site) + `internal/repomap` (sink interface lives here, alongside `cache.go`) | `internal/obs` (sink) | `walkAndExtract` in the skill is the only production caller of `cache.GetOrExtract`; the sink interface lives next to the cache for cohesion. |
| Workspace activate/deactivate emission | `internal/kernel` (activate, timeout) + `internal/daemon` (deactivate, shutdown) | `internal/obs` (sink) | Activation is kernel-owned; deactivation/shutdown surface only at the gRPC handler / signal-first sweep. |
| Edit outcome classification + emission | `internal/kernel/edit` (sink interface) + `internal/kernel/edit` & `internal/kernel/fileops` (call sites) | `internal/obs` (sink), `internal/fuzzy` (strategy enum source) | Closest cohesion to the fuzzy result; fileops imports edit's sink interface to keep one definition. |
| CI label-allowlist enforcement | `internal/obs/metrics_labels_test.go` | — | Existing CI gate; carve-outs are appended in place. |

## Standard Stack

This is internal instrumentation on existing infrastructure, not new library adoption. The relevant pinned versions are already in `go.mod`:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/prometheus/client_golang` | already pinned (v1.2 era) | CounterVec / HistogramVec / GaugeVec construction; owned `*Registry`. | Adopted in Phase 11; confined to `internal/obs` per D-08. [VERIFIED: internal/obs/metrics.go:22-23] |
| Go | 1.25.1 | Module language version. | [VERIFIED: go.mod] |

### Supporting
None — phase reuses existing patterns.

### Alternatives Considered
None — Phase 11 D-01..D-04 froze the design; this phase is a strict expansion.

**Installation:** N/A (libraries already pinned).

## Architecture Patterns

### System Architecture Diagram

```
                     ┌──────────────────────────────────────┐
                     │   *obs.Metrics (owned *Registry)     │
                     │  + vectors: ToolCalls, ToolDuration, │
                     │    LSPoolWorkers, ..., RenameStrategy│
                     │  + NEW: LSPoolCache, RepoMapCache,   │
                     │         RepoMapExtractDuration,      │
                     │         SessionLifecycle, EditOutcome│
                     │  + helper methods (LSPoolCacheInc,…) │
                     └─────────────┬────────────────────────┘
                                   │ implements (compile-time check in wiring_test.go)
        ┌──────────────────┬───────┼────────────────────┬────────────────────┐
        │                  │       │                    │                    │
        ▼                  ▼       ▼                    ▼                    ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────────┐ ┌───────────────────────┐
│lspool.MetricsSink│ │repomap.MetricsSink│ │kernel.SessionMetrics │ │edit.MetricsSink       │
│ (extended w/     │ │  (NEW)            │ │   Sink (NEW)         │ │  (NEW)                │
│  LSPoolCacheInc) │ │ + NoopSink        │ │ + NoopSink           │ │ + NoopSink            │
│ + NoopSink       │ │                   │ │                      │ │                       │
└────────┬─────────┘ └─────────┬─────────┘ └──────────┬───────────┘ └───────────┬───────────┘
         │                     │                      │                          │
         ▼                     ▼                      ▼                          ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────────┐ ┌───────────────────────┐
│ lspool/pool.go   │ │ skill/repomap/   │ │ kernel/kernel.go     │ │ edit/replace.go +     │
│ AcquireLease     │ │ skill.go:        │ │ ActivateWorkspace    │ │ insert.go + rename.go │
│  ├─share→ hit/   │ │  walkAndExtract  │ │   →activate          │ │ + delete.go +         │
│  │   {clean}     │ │   (extractFn)    │ │ daemon.go:           │ │ fileops/tools.go +    │
│  ├─circuit→miss/ │ │ → cache hit/miss │ │ DeactivateWorkspace  │ │ fileops/fuzzy_edit.go │
│  │   {crashed}   │ │ → time extract   │ │   →deactivate        │ │  → outcome from       │
│  ├─dirty→ miss/  │ │                  │ │ pool.checkTTLs       │ │    fuzzy.Strategy     │
│  │   {dirty}     │ │                  │ │   →timeout (per OQ-1)│ │                       │
│  └─spawn→ miss/  │ │                  │ │ daemon.shutdown      │ │                       │
│      {clean}     │ │                  │ │   →shutdown          │ │                       │
└──────────────────┘ └──────────────────┘ └──────────────────────┘ └───────────────────────┘
```

### Recommended Project Structure

```
internal/
├── obs/
│   ├── metrics.go               # ADD: 5 new *Vec fields + helpers (extends frozen contract)
│   ├── metrics_test.go          # extend RegisteredFamilies test
│   └── metrics_labels_test.go   # ADD: new carve-outs + cardinality assertion
├── kernel/
│   ├── lspool/
│   │   ├── metrics.go           # EXTEND interface with LSPoolCacheInc + result/scope constants
│   │   └── pool.go              # ADD emit calls in AcquireLease branches
│   ├── edit/
│   │   ├── metrics.go           # NEW: edit.MetricsSink + NoopSink + tool/outcome consts
│   │   ├── replace.go           # ADD emit at end of ReplaceBodyWithPlan
│   │   ├── insert.go / rename.go / delete.go / verify.go  # ADD emit at handler ends
│   ├── session_metrics.go       # NEW: kernel.SessionMetricsSink + NoopSink + phase consts
│   ├── kernel.go                # ADD activate emit at end of ActivateWorkspace
│   └── workspace.go             # (no changes — language detection already done)
├── repomap/
│   ├── metrics.go               # NEW: repomap.MetricsSink + NoopSink
│   └── cache.go                 # (no changes — extractFn closure in skill is the emission site)
├── skill/repomap/
│   └── skill.go                 # ADD emit around extractFn at skill.go:366
├── kernel/fileops/
│   ├── tools.go                 # ADD emit in registerReplaceInFile / registerFuzzyEdit handlers
│   └── fuzzy_edit.go            # (returns fuzzy.Result — caller in tools.go classifies)
└── daemon/
    ├── daemon.go                # ADD: pass new sinks at construction; add session emits in
    │                            #      ActivateWorkspace gRPC, DeactivateWorkspace gRPC,
    │                            #      and the signal-first shutdown sweep
    └── wiring_test.go           # ADD 3 new var _ assertions + extend lspool one
```

### Pattern 1: Owned-Registry Vector Construction

**What:** All vectors constructed once in `newMetrics()`, registered against a private `*prometheus.Registry`. Never touch `prometheus.DefaultRegisterer`.

**When to use:** Every new metric family in this phase.

**Example (extracted from `internal/obs/metrics.go:84-93`):**

```go
// [VERIFIED: internal/obs/metrics.go:62-117]
LSPoolEvictions: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "serena_lspool_evictions_total",
        Help: "LS worker evictions by reason (idle/pressure/crash/shutdown).",
    },
    []string{"language", "reason"},
),
```

New vector to add (template, derived from D-01..D-11):

```go
LSPoolCache: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "serena_lspool_cache_total",
        Help: "LS pool cache decisions by language, result (hit|miss), and scope (clean|dirty|crashed).",
    },
    []string{"language", "result", "scope"},
),
RepoMapCache: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "serena_repomap_cache_total",
        Help: "RepoMap tag cache decisions by language and result (hit|miss).",
    },
    []string{"language", "result"},
),
RepoMapExtractDuration: prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "serena_repomap_extract_duration_seconds",
        Help:    "RepoMap tag extraction latency by language.",
        Buckets: prometheus.DefBuckets,
    },
    []string{"language"},
),
SessionLifecycle: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "serena_session_lifecycle_total",
        Help: "Workspace session lifecycle transitions (activate|deactivate|timeout|shutdown).",
    },
    []string{"language", "phase"},
),
EditOutcome: prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "serena_edit_outcome_total",
        Help: "Edit-tool outcomes by tool (closed allowlist) and outcome (success|fuzzy_applied|refused_ambiguous|failed).",
    },
    []string{"tool", "outcome"},
),
```

### Pattern 2: Closed-Enum Carve-Out at Sink + CI Label Lint

**What:** Drop unknown values at the sink helper (return early); add the label NAME to `carveOuts` map in `metrics_labels_test.go` for that family only. The CI lint carves the label name; the value enum is enforced at emission. Mirror `RenameStrategyInc` at `internal/obs/metrics.go:171-176`.

**Example (verified, `internal/obs/metrics.go:171-176`):**

```go
func (m *Metrics) RenameStrategyInc(strategy string) {
    if strategy != "lsp-native" && strategy != "rust-client-side" {
        return
    }
    m.RenameStrategy.WithLabelValues(strategy).Inc()
}
```

New helper template (validates `result`/`scope` enums):

```go
func (m *Metrics) LSPoolCacheInc(language, result, scope string) {
    if result != "hit" && result != "miss" {
        return
    }
    if scope != "clean" && scope != "dirty" && scope != "crashed" {
        return
    }
    m.LSPoolCache.WithLabelValues(language, result, scope).Inc()
}
```

Apply analogously to `RepoMapCacheInc(language, result)`, `SessionLifecycleInc(language, phase)`, `EditOutcomeInc(tool, outcome)`. The edit-tool allowlist (D-08) is enforced at the same point: unknown `tool` value → drop.

### Pattern 3: Per-Package Sink + NoopSink + Compile-Time Assertion

**What:** Define a minimal interface in the consumer package; provide `NoopSink{}` for tests/bootstrap; assert `var _ MetricsSink = NoopSink{}` and `var _ MetricsSink = (*obs.Metrics)(nil)` in `wiring_test.go` to keep signatures pinned.

**Verified prior art:** `internal/kernel/lspool/metrics.go:1-63` and `internal/daemon/wiring_test.go:17`.

### Anti-Patterns to Avoid

- **Touching `prometheus.DefaultRegisterer`:** banned (Phase 11 PITFALLS meta-rule). Use only the registry owned by `*obs.Metrics`.
- **Branching on `metrics == nil`:** noop providers ALWAYS hand a real `*obs.Metrics` (`obs.Noop` constructs `newMetrics()`); call sites just call methods. (`internal/obs/obs.go:47-53`)
- **Adding `workspace_path` (or any path) as a label:** unbounded. Rejected by D-06.
- **Custom buckets without dashboard-driven evidence:** D-10 explicitly defers this.
- **Emitting metric inside a tight inner loop without a labels-cached vector reference:** `WithLabelValues` allocates a child metric per label tuple; for hot paths, pre-resolve. For Phase 53 emission sites (lease acquisition, file extraction, edit completion), call frequency is moderate — `WithLabelValues` per call is consistent with existing code (`internal/obs/metrics.go:147`, `internal/kernel/lspool/pool.go:298`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Counter / histogram primitives | A custom counter struct | `prometheus.NewCounterVec` / `NewHistogramVec` | Already imported and pinned in `internal/obs`. |
| Label-allowlist enforcement | A new lint test | Extend existing `lintLabels` in `metrics_labels_test.go` | The lint walks `Gather()` output and consults `carveOuts` map — drop new labels into that map. |
| Bucket selection | Hand-tuned histogram boundaries | `prometheus.DefBuckets` (per D-10) | Consistent with `serena_tool_duration_seconds`. |
| Strategy → outcome mapping | A new enum | Reuse `fuzzy.Strategy` constants from `internal/fuzzy/types.go:14-28` | Already public contract: `StrategyExact / StrategyWhitespace / StrategyIndentationFlex / StrategyFailed`. The ambiguity error path is a separate refusal (returned by `fuzzy.Match` at `match.go:56` via `ambiguityError(StrategyExact, hits)`). |
| Per-language language detection | New extension scanner | Use the language string already on the call site — `worker.Language()` for lspool, `repomap.LangFromExt(path)` for repomap, `len(rt.Languages())` first entry for kernel session lifecycle | All call sites already have `language` in scope. |

**Key insight:** Every emission point already has the label values it needs in scope. The phase is wiring, not new domain logic.

## Common Pitfalls

### Pitfall 1: Misclassifying lspool cache scope

**What goes wrong:** `scope` ∈ {clean, dirty, crashed} maps to specific branches in `Pool.AcquireLease` (`internal/kernel/lspool/pool.go:111-148`):
- `clean` + `hit` → `dirty=false` AND `workerForKeyLocked` returned a worker (line 117–122).
- `clean` + `miss` → `dirty=false` AND no warm worker → spawn (lines 137–142, after the `if !dirty` block).
- `dirty` + `miss` → `dirty=true` → always spawns dedicated (lines 137–142).
- `crashed` + `miss` → `cb.CanAttempt()` returned false (lines 127–129) — emitted BEFORE the function returns the circuit-open error.

**Why it happens:** It's tempting to emit a single `miss` after the spawn, losing the dirty-vs-crashed distinction.

**How to avoid:** Emit at four explicit branches in `AcquireLease`. Mirror the existing eviction-emit pattern at `pool.go:298` (one emit per branch).

**Warning signs:** Cardinality test reports `scope="crashed"` series never appearing; or `dirty` count ≈ `clean` miss count (likely conflated).

### Pitfall 2: RepoMap cache emission inside `extractFn` closure

**What goes wrong:** `cache.GetOrExtract` in `internal/repomap/cache.go:60-103` calls `extractFn()` only on cache MISS (lines 88-89). The cache HIT path returns early at line 78. So a naive emit inside `extractFn` only sees misses.

**Why it happens:** Reading `cache.go` makes it look like the closure is the hook point.

**How to avoid:** Emit `result=hit` when `cache.GetOrExtract` returns successfully WITHOUT having invoked `extractFn`. Two implementation options:
- (a) Wrap the closure: pass a `bool` extracted flag set inside `extractFn`, branch on it after `GetOrExtract` returns. Time the extraction inside the closure.
- (b) Plumb a sink into `TagCache` itself and emit at the cache.go boundary (preferred — keeps emission cohesive with the decision point and avoids the boolean dance). This is the cleaner pattern but adds a sink dependency to `internal/repomap` (not just the new `metrics.go` file).

**Recommendation:** Option (b). Add a `MetricsSink` field to `TagCache` (defaulted to `NoopSink{}` so existing tests don't need changes), and emit hit/miss + extract duration directly inside `GetOrExtract`. The `language` argument needs to be threaded — either via a new parameter on `GetOrExtract` (breaking) or by computing it inside via `LangFromExt(filePath)` (`render.go:124` already does this — verified). The latter avoids touching the public signature.

**Warning signs:** Hit-rate flat at 0%; or extraction duration histogram only populated on first run.

### Pitfall 3: Session lifecycle "timeout" semantics

**What goes wrong:** CONTEXT.md D-04 maps `timeout` to "adaptive-TTL / idle eviction in the workspace runtime." But `internal/kernel/workspace.go` does NOT have idle eviction — that lives at the WORKER level (`pool.go:checkTTLs`, `pool.go:355-356`). There is no per-WORKSPACE idle timeout in v1.2.

**Why it happens:** The mental model conflates session, workspace, and worker.

**How to avoid:** Two valid interpretations — planner picks one and locks it in plan 53-01:
- (a) Reuse `pool.checkTTLs` evict path as the source of `timeout`: emit `phase=timeout` when a worker is retired due to idle TTL (`pool.go:354-356`). This is the closest existing analog. Language is on the worker.
- (b) Drop `timeout` from the v1 enum and document it as "reserved for future per-workspace TTL." Cleaner semantics, smaller scope.

**Recommendation:** (a) for now — it's discoverable behavior operators care about (warm worker just got evicted) and there's no other natural emission point. Document explicitly in USAGE.md that `phase=timeout` means "warm worker idle-evicted" not "user session timed out" (no user sessions exist as first-class objects in v1.2).

**Warning signs:** Operators ask "what triggers `phase=timeout`?" and the answer requires three levels of indirection.

### Pitfall 4: Edit outcome at multiple tools — single call site or one per tool?

**What goes wrong:** There are 6 edit tools (`replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, `verify_edit`) plus 3 fileops tools that perform edits (`replace_in_file`, `fuzzy_edit`, `create_file`). Emitting at each handler duplicates classification logic.

**Why it happens:** Each handler returns `(*mcpsdk.CallToolResult, any, error)` and the outcome must be derived after the call returns.

**How to avoid:** Define a small classifier helper next to the sink:

```go
// in internal/kernel/edit/metrics.go
func ClassifyOutcome(strategy fuzzy.Strategy, err error) string {
    if err != nil {
        if isAmbiguityError(err) { return "refused_ambiguous" }
        return "failed"
    }
    switch strategy {
    case fuzzy.StrategyExact, "":  // empty = no fuzzy involved (LSP-native rename, exact body replace)
        return "success"
    case fuzzy.StrategyWhitespace, fuzzy.StrategyIndentationFlex:
        return "fuzzy_applied"
    case fuzzy.StrategyFailed:
        return "failed"
    }
    return "failed"
}
```

The ambiguity error is currently constructed at `internal/fuzzy/match.go:56` via `ambiguityError(StrategyExact, hits)`. Plan 53-02 must verify whether that error type is exported / detectable; if not, add a sentinel error or check via `errors.Is` against a new `fuzzy.ErrAmbiguous`. (Currently UNVERIFIED — researcher did not open `match.go` ambiguityError implementation; planner should confirm in plan 53-02 task 1.)

**Recommendation:** One emit call per handler, immediately before each return, using `ClassifyOutcome`. Tools that don't carry a fuzzy.Strategy (rename_symbol, insert_*, safe_delete_symbol, create_file, verify_edit) pass `""` and get `success` or `failed` based on err only.

**Warning signs:** Different tools producing inconsistent outcome bucketing; `fuzzy_applied` count zero on a workload that obviously hit the cascade.

### Pitfall 5: Forgetting to wire the sink in the tracing/test path

**What goes wrong:** `obs.NewForTest` and `obs.Noop` both call `newMetrics()`. If the new helper methods are added to `*obs.Metrics` but the test sets up its own sink type, the test bypasses the metric.

**Why it happens:** Tests sometimes use `lspool.NoopSink{}` directly instead of `obs.Noop(...).Metrics()`.

**How to avoid:** `wiring_test.go` already exercises `obs.Noop(nil).Metrics()` at line 23 — extend that pattern for the three new sinks (mirror `TestObsMetricsIsLSPoolSink`).

**Warning signs:** Test passes locally, CI flag-flips when the registry collector counts mismatch.

## Runtime State Inventory

This phase modifies code only — no rename, refactor, or migration of stored state. Section omitted as out of scope (no datastores, OS state, or build artifacts embed metric names externally).

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — Prometheus scrape side is stateless. | None. |
| Live service config | None — there is no operator-side config tied to current metric names yet (Phase 54 dashboards are not built). | None. |
| OS-registered state | None. | None. |
| Secrets/env vars | None. | None. |
| Build artifacts | None — no generated code references metric names. | None. |

**Roadmap-text reference to old names:** `ROADMAP.md` Phase 53 success criterion 1 names `serena_lspool_cache_hits_total` and `serena_repomap_cache_hits_total`. CONTEXT.md D-01 deliberately renames to `*_cache_total{result=...}`. Planner must reconcile: either edit the roadmap success criterion or document the divergence in 53-01. (Already captured in Deferred Ideas as "Roadmap text reconciliation.")

## Code Examples

### Emit cache decisions in `lspool.Pool.AcquireLease`

```go
// [VERIFIED: internal/kernel/lspool/pool.go:111-148 — annotated emission points]

func (p *Pool) AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*WorkerLease, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if !dirty {
        if w := p.workerForKeyLocked(wsKey); w != nil {
            // EMIT: hit/clean
            p.metrics.LSPoolCacheInc(wsKey.Language, "hit", "clean")
            lease := NewWorkerLease(sessionID, w, false)
            p.leases[sessionID] = lease
            return lease, nil
        }
    }

    cb := p.circuitForLanguage(wsKey.Language)
    if !cb.CanAttempt() {
        // EMIT: miss/crashed
        p.metrics.LSPoolCacheInc(wsKey.Language, "miss", "crashed")
        return nil, cb.CircuitOpenErr()
    }

    if len(p.workers) >= p.config.MaxWorkers {
        // EMIT: miss/clean (bounded by max-workers, but still "no warm worker available")
        p.metrics.LSPoolCacheInc(wsKey.Language, "miss", "clean")
        return nil, ErrMaxWorkersReached
    }

    worker, err := p.spawnWorkerLocked(ctx, wsKey)
    if err != nil {
        cb.RecordFailure()
        // EMIT: miss/clean (or "crashed" — planner choice; spawn-failure semantically closer to crashed)
        p.metrics.LSPoolCacheInc(wsKey.Language, "miss", "crashed")
        return nil, fmt.Errorf("spawning worker: %w", err)
    }
    cb.RecordSuccess()

    // EMIT: miss/{clean|dirty} (depending on dirty flag)
    scope := "clean"
    if dirty {
        scope = "dirty"
    }
    p.metrics.LSPoolCacheInc(wsKey.Language, "miss", scope)

    lease := NewWorkerLease(sessionID, worker, dirty)
    p.leases[sessionID] = lease
    return lease, nil
}
```

### Emit RepoMap cache + duration in `cache.GetOrExtract`

```go
// [VERIFIED: internal/repomap/cache.go:60-103 — annotated]
func (c *TagCache) GetOrExtract(filePath string, extractFn func() ([]Tag, error)) ([]Tag, error) {
    info, err := os.Stat(filePath)
    if err != nil { return nil, err }
    mtime := info.ModTime().UnixNano()
    lang := LangFromExt(filePath) // [VERIFIED: render.go:124 uses this same helper]

    c.mu.Lock()
    var cachedMtime int64
    err = c.db.QueryRow(/* … */).Scan(&cachedMtime)
    if err == nil && cachedMtime == mtime {
        tags, loadErr := c.loadTags(filePath)
        c.mu.Unlock()
        if loadErr != nil { return nil, loadErr }
        c.metrics.RepoMapCacheInc(lang, "hit") // EMIT
        return tags, nil
    }
    c.mu.Unlock()

    start := time.Now() // EMIT prep
    tags, err := extractFn()
    c.metrics.RepoMapExtractObserve(lang, time.Since(start).Seconds()) // EMIT
    c.metrics.RepoMapCacheInc(lang, "miss") // EMIT (after extract — even if it returned nil tags)
    if err != nil { return nil, err }

    c.mu.Lock()
    defer c.mu.Unlock()
    if err := c.storeTags(filePath, mtime, tags); err != nil { return nil, err }
    return tags, nil
}
```

Note: this requires a constructor change (`NewTagCache` gains a `metrics MetricsSink` parameter, defaulting to `NoopSink{}` if nil). Verified that `NewTagCache` is called from one production site (`internal/skill/repomap/skill.go`, near `NewTreeRenderer`) plus tests.

### Emit edit outcome from a tool handler

```go
// [TEMPLATE — patterned after registerReplaceBody at internal/kernel/edit/tools.go:221-272]

func registerReplaceBody(server *mcp.SerenaMCPServer, k *kernel.Kernel, extractor *BodyExtractor,
                         diagStore *diag.DiagnosticStore, wsKeyFn func() workspace.WorkspaceKey,
                         tracer trace.Tracer, sink MetricsSink /* NEW */) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name: "replace_symbol_body",
        // …
    }, kernel.WrapToolSpan(tracer, "replace_symbol_body",
        func(ctx context.Context, req *mcpsdk.CallToolRequest, args ReplaceBodyArgs) (*mcpsdk.CallToolResult, any, error) {
            info, err := /* … existing call to ReplaceBodyWithPlan … */
            outcome := ClassifyOutcome(strategyOf(info), err)
            sink.EditOutcomeInc("replace_symbol_body", outcome) // EMIT
            // … existing return …
        }))
}

func strategyOf(info *FuzzyMatchInfo) fuzzy.Strategy {
    if info == nil { return "" }
    return info.Strategy
}
```

For tools without a fuzzy result (`rename_symbol`, `insert_*`, `safe_delete_symbol`, `verify_edit`, `create_file`), pass `""` to `ClassifyOutcome`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 8 ad-hoc logging middleware | Phase 11 `TelemetryMiddleware` + owned `*prometheus.Registry` | Phase 11 (v1.2) | Defines all subsequent metric work; Phase 53 strictly extends. |
| Hits-only cache counters in roadmap text | `result={hit,miss}` on a single counter family | Phase 53 (this phase, D-01) | Roadmap text divergence; plan 53-01 reconciles. |

**Deprecated/outdated:** none.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The fuzzy-ambiguity error is detectable by callers (so `ClassifyOutcome` can map it to `refused_ambiguous`). | Pitfall 4, Code Examples | If `ambiguityError` returns an opaque `error` that's only string-matchable, planner needs to add an exported sentinel or `errors.Is` target in `internal/fuzzy/match.go`. Planner verifies in plan 53-02 task 1. [ASSUMED] |
| A2 | `Pool.AcquireLease`'s `ErrMaxWorkersReached` branch should book as `miss/clean`, and spawn failures should book as `miss/crashed`. CONTEXT.md D-02 lists `crashed` only for circuit-blocked, not spawn-failed. | Code Examples (lspool emission) | Cardinality stays bounded either way, but `crashed` semantics drift if spawn errors are recorded as crashed. Planner picks the convention in plan 53-02. [ASSUMED] |
| A3 | The 9 tools in the closed allowlist for `serena_edit_outcome_total` are: `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `rename_symbol`, `safe_delete_symbol`, `verify_edit` (from `internal/kernel/edit/tools.go`), `replace_in_file`, `fuzzy_edit`, `create_file` (from `internal/kernel/fileops/tools.go`). | Architectural Responsibility Map; Pitfall 4 | If planner decides `verify_edit` and `create_file` aren't "edits" in the OBS sense (e.g., verify is read-only diagnostics), they get dropped from the allowlist. Cardinality bound (9 × 4 = 36 series) shrinks but the test asserts the chosen number. [VERIFIED via `grep "Name:" tools.go` — all 9 tool names exist; ASSUMED on which subset counts as an "edit"] |
| A4 | Adding `MetricsSink` to `TagCache` constructor is preferable to wrapping `extractFn`. Constructor signature is internal (one production caller in `skill/repomap/skill.go`). | Pitfall 2 | If planner prefers the closure-wrapping option (less invasive on `cache.go`), the metric still works; the planning trade-off is "where does the sink dependency live." [ASSUMED preference] |
| A5 | `phase=timeout` should map to `pool.checkTTLs` worker idle eviction since there is no per-WORKSPACE idle timeout in v1.2. | Pitfall 3 | If planner picks alternative (b) (drop `timeout`), CONTEXT.md D-04 enum tightens from 4 values to 3, cardinality test bound shifts, and a follow-up phase reintroduces `timeout` when per-workspace TTL ships. [ASSUMED interpretation] |

## Open Questions

1. **`phase=timeout` semantics.** What does it represent in v1.2? See Pitfall 3 + Assumption A5. Recommendation: emit at `pool.checkTTLs` worker idle eviction; document in USAGE.md.
2. **Spawn-failure scope.** Should `spawnWorkerLocked` failure book as `miss/crashed` or `miss/clean`? Recommendation: `miss/crashed` — spawn failure is a circuit-relevant signal.
3. **`create_file` and `verify_edit` in the edit-tool allowlist.** Are they "edits" for OBS purposes? `create_file` writes a new file (yes); `verify_edit` runs diagnostics post-edit (arguably no — it's the oracle, not the actor). Recommendation: include `create_file`, exclude `verify_edit`.

## Environment Availability

Skipped — phase is code/config-only, no external dependencies.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + testify/assert (already in tree, e.g. `internal/fuzzy/fuzzy_test.go`). Go 1.25.1. |
| Config file | none — `go test` honors package layout. `go.mod` is the source of truth. |
| Quick run command | `go test ./internal/obs/... ./internal/kernel/lspool/... ./internal/kernel/edit/... ./internal/repomap/... ./internal/skill/repomap/... ./internal/daemon/...` |
| Full suite command | `go test ./...` (per CLAUDE.md "Always run go vet and go test before completing any Go task.") |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-03 | `/metrics` exposes `serena_lspool_cache_total` with `result` and `scope` populated | unit | `go test ./internal/obs -run TestMetrics_RegisteredFamilies -count=1` | ✅ extend existing |
| OBS-03 | `/metrics` exposes `serena_repomap_cache_total` and `serena_repomap_extract_duration_seconds` | unit | `go test ./internal/obs -run TestMetrics_RegisteredFamilies -count=1` | ✅ extend existing |
| OBS-03 | `/metrics` exposes `serena_session_lifecycle_total{phase}` | unit | `go test ./internal/obs -run TestMetrics_RegisteredFamilies -count=1` | ✅ extend existing |
| OBS-03 | `/metrics` exposes `serena_edit_outcome_total{tool,outcome}` | unit | `go test ./internal/obs -run TestMetrics_RegisteredFamilies -count=1` | ✅ extend existing |
| OBS-03 | All new labels are in `AllowedLabels` or carved out per-family | unit | `go test ./internal/obs -run TestMetricsLabelsAllowlist -count=1` | ✅ extend existing |
| OBS-03 | Cardinality bound: 312 / 104 / DefBuckets / 4·N_lang / 9·4 series caps respected | unit (NEW) | `go test ./internal/obs -run TestMetrics_CardinalityBounds -count=1` | ❌ Wave 0 |
| OBS-03 | `*obs.Metrics` satisfies all 4 sink interfaces | unit | `go build ./internal/daemon` (compile-time `var _` assertions) + `go test ./internal/daemon -run TestObsMetricsIsLSPoolSink -count=1` | ✅ extend existing |
| OBS-03 | lspool emits hit/clean on shared lease, miss/clean on fresh spawn, miss/crashed on circuit, miss/dirty on dirty acquire | unit (NEW) | `go test ./internal/kernel/lspool -run TestPool_CacheMetricsEmission -count=1` | ❌ Wave 0 |
| OBS-03 | TagCache emits `result=hit` on mtime-match, `result=miss` + extract duration on cold path | unit (NEW) | `go test ./internal/repomap -run TestTagCache_MetricsEmission -count=1` | ❌ Wave 0 |
| OBS-03 | Edit outcome classifier maps fuzzy.Strategy + err to {success, fuzzy_applied, refused_ambiguous, failed} | unit (NEW) | `go test ./internal/kernel/edit -run TestClassifyOutcome -count=1` | ❌ Wave 0 |
| OBS-03 | Session lifecycle emits `activate` from kernel.ActivateWorkspace, `deactivate` from gRPC handler, `timeout` from idle TTL eviction, `shutdown` from daemon shutdown sweep | unit (NEW) | `go test ./internal/daemon -run TestSessionLifecycleMetrics -count=1` | ❌ Wave 0 |
| OBS-03 (D-15) | Noop-default invariant — metrics are zero-alloc when observability is disabled | unit (NEW, allocs-based) | `go test ./internal/obs -run TestMetrics_NoopZeroAllocOnSinkPath -count=1 -benchmem` (uses `testing.AllocsPerRun`) | ❌ Wave 0 |
| OBS-03 (Success Criterion 3) | USAGE.md Observability section documents each new metric | manual review (no automated test) | n/a — checklist in plan 53-03 | n/a |

### Sampling Rate
- **Per task commit:** `go test ./internal/obs/... ./internal/kernel/lspool/... ./internal/kernel/edit/... ./internal/repomap/... ./internal/daemon/... -count=1`
- **Per wave merge:** `go test ./... -count=1` + `go vet ./...`
- **Phase gate:** `go test ./...` green + `go vet ./...` clean before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/obs/metrics_labels_test.go` — extend `carveOuts` with `result`, `scope`, `phase`, `outcome`, `tool` per-family carve-outs; extend `TestMetricsLabelsAllowlist` to prime the new vectors.
- [ ] `internal/obs/metrics_cardinality_test.go` (NEW) — `TestMetrics_CardinalityBounds`: register max-fanout series for each new family (52 langs × 2 results × 3 scopes for lspool, 52 × 2 for repomap, 52 × 4 phases for session, 9 tools × 4 outcomes for edit) and assert `Gather()` returns ≤ the computed cap.
- [ ] `internal/obs/metrics_alloc_test.go` (NEW) — `TestMetrics_NoopZeroAllocOnSinkPath`: use `testing.AllocsPerRun` to assert the noop sink methods cause zero allocations on the hot path. (The vector-write path inevitably allocates label tuples; the test must scope to the `NoopSink{}` path or to a non-emitting branch.)
- [ ] `internal/kernel/lspool/metrics_test.go` — extend with `TestPool_CacheMetricsEmission` covering the four scope/result branches with a fake sink recorder.
- [ ] `internal/repomap/metrics.go` (NEW) — sink interface + NoopSink + compile-time assertion.
- [ ] `internal/repomap/cache_test.go` — extend with `TestTagCache_MetricsEmission` exercising hit / miss / extract-duration emission via a fake sink.
- [ ] `internal/kernel/edit/metrics.go` (NEW) — sink interface + NoopSink + tool allowlist constants + `ClassifyOutcome` helper.
- [ ] `internal/kernel/edit/metrics_test.go` (NEW) — `TestClassifyOutcome` table-driven over `(fuzzy.Strategy, error)` → outcome.
- [ ] `internal/kernel/session_metrics.go` (NEW) — sink interface + NoopSink + phase constants.
- [ ] `internal/daemon/wiring_test.go` — add three new `var _` assertions and runtime smoke calls (mirror existing `TestObsMetricsIsLSPoolSink`).
- [ ] `internal/daemon/telemetry_metrics_test.go` (extend) — `TestSessionLifecycleMetrics` exercising activate/deactivate/timeout/shutdown emission paths via daemon construction harness.

## Project Constraints (from CLAUDE.md)

- **Go formatting / vet / tests:** Run `gofmt -w .`, `go vet ./...`, and `go test ./...` before completing any Go task. (Mandatory per CLAUDE.md "Go Development Commands".)
- **GSD workflow:** This phase is being researched under `/gsd-research-phase`; planner will continue under `/gsd-plan-phase`. Direct edits outside GSD are forbidden by CLAUDE.md "GSD Workflow Enforcement".
- **Single Go binary, no Python:** All implementation lands in `internal/` Go packages. The `legacy/` Python tree is read-only reference and is not touched by this phase.
- **No JetBrains / proprietary backends:** N/A for this phase (pure observability).
- **Tool registration patterns:** Kernel tools use `RegisterTools(server, …)` + `mcpsdk.AddTool`; this phase only ADDS sink dependencies to existing `RegisterTools` signatures (`edit.RegisterTools` gains a `MetricsSink` parameter; daemon wires `observability.Metrics()`). No new MCP tools registered.
- **Middleware install order is invariant:** LIFO order is `LazyInit → Suggestion → ProfileFilter → Telemetry → handler`. This phase does not touch middleware install order; verify any wiring changes preserve `internal/mcp/lazy_init.go:106-108`'s install-order comment.

## Sources

### Primary (HIGH confidence)
- `internal/obs/metrics.go` — frozen owned-registry pattern, AllowedLabels array, RenameStrategyInc closed-enum example. [VERIFIED]
- `internal/obs/obs.go` — `obs.Noop` always returns non-nil Metrics, contract-bound. [VERIFIED]
- `internal/obs/metrics_labels_test.go` — `carveOuts` map, `lintLabels` walker, drift-detection negative test. [VERIFIED]
- `internal/obs/metrics_test.go` — `TestMetrics_RegisteredFamilies` pattern for asserting new family presence. [VERIFIED]
- `internal/kernel/lspool/metrics.go` — sink + NoopSink + compile-time assertion + closed-enum constants. [VERIFIED]
- `internal/kernel/lspool/pool.go:111-148, 297-307, 354-356, 463-466, 484-488` — emission points for cache hit/miss + scope. [VERIFIED]
- `internal/repomap/cache.go:60-103` — `GetOrExtract` with hit/miss branches and `extractFn` invocation. [VERIFIED]
- `internal/repomap/render.go:124, 127` — `LangFromExt` already used as in-line language source. [VERIFIED]
- `internal/skill/repomap/skill.go:344-399` — `walkAndExtract` is the only production caller of `GetOrExtract`. [VERIFIED]
- `internal/kernel/kernel.go:59-93` — `ActivateWorkspace` lazy + explicit converge here. [VERIFIED]
- `internal/daemon/daemon.go:614-662` — gRPC `ActivateWorkspace` and `DeactivateWorkspace` handlers. [VERIFIED]
- `internal/daemon/wiring_test.go:17-33` — compile-time + runtime sink assertion pattern. [VERIFIED]
- `internal/fuzzy/types.go:14-28` — `Strategy` enum public contract. [VERIFIED]
- `internal/fuzzy/match.go:54-56` — exact-match success and ambiguity error site. [VERIFIED]
- `internal/kernel/edit/tools.go:223-457`, `internal/kernel/fileops/tools.go:232-437` — registered tool names enumerated. [VERIFIED via grep]
- `internal/kernel/edit/replace.go:38-114` — `FuzzyMatchInfo` carries `Strategy` back to handlers. [VERIFIED]
- `internal/mcp/middleware.go:18-87` — `RecordRenameStrategy` adapter pattern (sink wired via `setRenameStrategySink`). [VERIFIED prior art for closed-enum sink wiring; not directly mirrored in this phase since per-package sinks are preferred per D-12.]
- `.planning/REQUIREMENTS.md:33` — OBS-03 acceptance language. [VERIFIED]
- `.planning/ROADMAP.md` Phase 53 section — success criteria 1–4. [VERIFIED]
- `.planning/phases/53-obs-metrics-gaps/53-CONTEXT.md` — D-01..D-16 (all decisions). [VERIFIED]
- `.planning/phases/53-obs-metrics-gaps/53-DISCUSSION-LOG.md` — Q&A audit trail. [VERIFIED]

### Secondary (MEDIUM confidence)
- Inferred edit-tool allowlist composition (A3 in Assumptions Log) — names verified, "edit" classification of `verify_edit` and `create_file` is interpretive.

### Tertiary (LOW confidence)
- None — phase is on solid in-tree prior art; no external library research was needed.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — single in-tree library (`prometheus/client_golang`), already integrated, no version research needed.
- Architecture: HIGH — Phase 11 + Phase 47 prior art exactly mirrors what this phase does. All emission points verified.
- Pitfalls: HIGH (Pitfalls 1, 2, 3, 5) / MEDIUM (Pitfall 4) — Pitfall 4's recommendation depends on whether the ambiguity error is detectable; flagged as A1.

**Research date:** 2026-04-26
**Valid until:** 2026-05-26 (30 days; in-tree code is stable, no fast-moving external dependency)
