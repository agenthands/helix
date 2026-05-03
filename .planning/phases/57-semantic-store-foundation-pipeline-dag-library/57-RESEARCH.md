# Phase 57: Semantic Store Foundation + Pipeline DAG Library — Research

**Researched:** 2026-05-03
**Domain:** DuckDB embedded fact store + pipeline DAG library + CGO=0 fallback gating + 4-layer config plumbing
**Confidence:** HIGH on duckdb-go-bindings posture, daemon bootstrap integration points, stub-pattern, vet-tool mechanics; MEDIUM on schema-version semantics (one open question), corruption-detection ergonomics (DuckDB upstream is silent on a clean "this file is corrupt" signal); LOW on whether the existing config tests should be extended versus a new per-key coverage test (project precedent is per-feature defaults assertions, not a coverage matrix).

## Summary

Phase 57 lays four orthogonal foundations for v1.10 in a single phase: (1) `internal/semantic/store/` opens a DuckDB fact store at daemon start when `semantic_index.enabled=true`, behind the same `//go:build cgo` stub gate v1.9 Phase 51.1 established for tree-sitter; (2) `internal/phasegraph/` ships a stdlib-only pipeline-DAG library that the new semantic graphs (Phase 60 live update, Phase 67 eval) use immediately while the daemon's imperative bootstrap stays put with a `// TODO(v1.11)` migration marker; (3) every `semantic_index.*` key in SPEC §25 flows through the existing 4-layer koanf precedence; (4) a `go vet -vettool` analyzer fails the build if anything outside `internal/semantic/store/` imports `duckdb-go`. Nothing in this phase touches grammar wiring (Phase 59), middleware install order (Phase 66), or `get_health` semantic enrichment (Phase 65) — those are explicit downstream scope.

Three findings change the planner's mental model:

1. **The currently-published Helix release matrix is CGO=0 across all 6 archives** [VERIFIED: `.goreleaser.yaml` env block]. Because of v1.9 Phase 51.1, those binaries already refuse to start (treesitter unavailable). So the user-facing surface for `semantic_index.enabled=true` in v1.10 is **CGO=1 builds only** — local `go build`, `make build`, or any future split CGO=1 release. STORE-02 is satisfied at compile time on CGO=0: the stub package returns `serr.Unsupported` from every Store method and the daemon refuses to start (already does, since Phase 51.1). On CGO=1, STORE-02 is satisfied at config time: when `semantic_index.enabled=false`, the Service is `nil` and every consumer guards.
2. **`duckdb-go-bindings` has prebuilt static libs for darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64 — but NOT windows-arm64** [VERIFIED: github.com/duckdb/duckdb-go-bindings README]. Helix's release matrix includes windows-arm64. Any future "ship a CGO=1 release" plan needs to either (a) drop windows-arm64 from the CGO=1 matrix, or (b) cross-compile DuckDB from source for windows-arm64. This is NOT a Phase 57 concern (current matrix is uniformly CGO=0), but it is a **planner heads-up for v1.10 release planning** so the eventual CGO=1-release phase doesn't get blindsided.
3. **There is no formal "every config key has a default + every key documented" coverage test today** [VERIFIED: `internal/config/loader_test.go` is per-feature `TestLoad_Defaults`, `TestLoad_ObservabilityDefaults`, etc.]. STORE-05 says new keys are "covered by the existing config-coverage test" — that test does not exist. Two paths: (a) extend the per-feature pattern with a `TestLoad_SemanticIndexDefaults` test asserting every `semantic_index.*` default present and resolves through layers, or (b) introduce a generic key-coverage assertion in v1.10. **Recommendation: (a)** mirrors the project's existing house style (one test per config struct), keeps Phase 57 scope tight, and does not risk green-vs-existing-tests false negatives.

**Primary recommendation:** Land Phase 57 as **four independent tracks** so plans can run in parallel: (T1) `internal/phasegraph/` library + tests — zero external surface, can land first; (T2) `internal/semantic/{config,store,types}` package skeleton + `//go:build cgo`/`!cgo` split + DuckDB driver dependency added + schema migration + open/quarantine/rebuild logic; (T3) `semantic_index.*` config keys threaded through `defaults.go` + struct fields on `SerenaConfig` + per-feature defaults test; (T4) `cmd/vet-noduckdb/` analyzer wired into `make vet` via `-vettool`. T2 is the load-bearing track and contains all the DuckDB risk; T1, T3, T4 are mechanical and independent of T2.

## User Constraints

> No CONTEXT.md exists for Phase 57 yet (this research runs ahead of `/gsd-discuss-phase`). The constraints below are inferred from `.planning/REQUIREMENTS.md` (STORE-01..06, DAG-01..04), `.planning/milestones/v1.10-ROADMAP.md` (Phase 57 description, dependency map, must-not-regress invariants), and `SPEC-DRAFT.md` (§4, §6, §8, §10, §25, §29.1, §32 Phase 0/12, §39). When `/gsd-discuss-phase` produces CONTEXT.md, this section will be re-stated verbatim from user decisions.

### Locked decisions (from REQUIREMENTS.md + ROADMAP.md)

- **Package boundary:** `internal/semantic/store/` is the sole owner of `duckdb-go` imports. Vet-checked (STORE-06).
- **Database location:** `<workspace>/.helix/semantic.duckdb` (STORE-01, SPEC §25 `semantic_index.store.path`).
- **Corruption recovery:** auto-quarantine to `.corrupt.<ts>` and rebuild fresh — daemon does NOT refuse to start on corruption (STORE-01, SPEC §29.1).
- **Disabled path:** every semantic-dependent tool returns `Kind: Unsupported` with remediation text when `semantic_index.enabled=false` OR when CGO=0 (STORE-02). Disabled-when-CGO=0 is **automatic** (compile-time stub), not a runtime config knob.
- **Schema versioning:** stamped per snapshot row; forward-incompatible bump triggers full reindex (clear log message); backward-compatible bump migrates in place (STORE-03).
- **Effective-read API:** `committed snapshot ⊕ live overlay − tombstones` for files, symbols, references, edges (STORE-04, SPEC §10). Phase 57 ships the API; Phase 59-60 populate the data.
- **Config layering:** every `semantic_index.*` key flows through CLI > project > user > profile (STORE-05).
- **Pipeline DAG library:** `internal/phasegraph/` with `PhaseSpec`, `PhaseGraph`, `ValidatePhaseGraph`, `RunPhaseGraph` — stdlib-only (DAG-01, SPEC §39.2-39.3).
- **DAG declarations:** semantic-index, live-update, eval pipelines each declare typed phase DAGs (DAG-02). Phase 57 declares the **shapes**; Phase 60/67 wire `Run`/`Validate`/`Shutdown` bodies.
- **DAG validation:** duplicate phase ID, missing dependency, cycle all fail before execution; error includes offending IDs and (when configured) writes `.helix/debug/phasegraph-*.dot` (DAG-03, SPEC §39.9).
- **Bootstrap stays imperative:** daemon bootstrap remains imperative in v1.10 with `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)` marker (DAG-04, ADR-010 + SPEC §32 Phase 12).
- **Must-not-regress invariants** (carried from v1.9):
  - Single-binary + CGO=0 stub policy (Phase 57 STORE-02 reuses Phase 51.1 pattern).
  - Single canonical `GrammarRegistry` (BUG-04) — Phase 57 does NOT touch grammar wiring.
  - Middleware install LIFO order — Phase 57 does NOT add middleware.
  - Bounded-label metrics with no source content — any new `helix_semantic_*` metric must respect the v1.2/v1.9 cardinality allowlist + PromQL validator.

### Claude's discretion

- Schema version representation: integer monotone counter (recommended) vs. semver-like `(major, minor)`. SPEC says "forward-incompatible vs backward-compatible" — pick a representation that makes that distinction unambiguous.
- Vet-tool integration shape: standalone `cmd/vet-noduckdb/main.go` invoked by `make vet -vettool=...` (recommended), or co-located with another existing analyzer if one exists. **No existing custom analyzer found** in the repo, so standalone is the only realistic shape.
- Where in the daemon bootstrap step sequence the `OpenSemanticStore` call lives. Recommendation: **step 6b**, immediately after the CGO=0 refusal hook (step 6a) and before the diagnostic store / grammar registry creation (step 6) — the store is a fail-fast core subsystem like the kernel and language registry, with the same "compile out under CGO=0" property.
- Logging structure for quarantine event: structured `slog` event + a `helix_semantic_store_quarantine_total` counter (recommended) vs stderr line only.
- Whether `internal/phasegraph/` uses Go generics. Recommendation: **no generics in v1.10** — `PhaseOutput` as `interface{}` keeps the API simple; Phase 60/67 consumers can add type assertions inline. Generics raise the cognitive load without buying anything when `Run` returns `(PhaseOutput, error)` and `Validate` consumes `PhaseOutput`.
- Exact `PhaseDeps` type. SPEC §39.2 leaves `PhaseDeps` undefined. Recommendation: `PhaseDeps map[PhaseID]PhaseOutput` — a downstream phase reads `deps[phasegraph.PhaseStore]` and asserts the concrete type.

### Deferred (OUT OF SCOPE for Phase 57)

- Tree-sitter extraction (Phase 59, EXTRACT-*).
- Live overlay watcher / coalescer (Phase 60, LIVE-*).
- LSP enrichment worker (Phase 61, ENRICH-*).
- Graph engine, ranking, type resolution (Phase 62, GRAPH-*/TYPES-*).
- Compaction worker, retention, VACUUM (Phase 63, COMPACT-*).
- New MCP tools (Phase 64, TOOL-*).
- `get_health` semantic enrichment, `get_repo_map` strangler-fig (Phase 65, INTEG-*).
- `GuardrailMiddleware` install at step 14b.5 (Phase 66, GUARD-*).
- Eval harness subprocess orchestration (Phase 67, EVAL-*).
- Bootstrap migration to `phasegraph.RunPhaseGraph(BootstrapPhases)` — explicit `// TODO(v1.11)` marker (DAG-04).
- Cluster MCP tools, multi-projection PageRank, P1 retrieval companions, comment-fallback type resolution — all v1.10.x.
- v1.9 carry-overs (PKG-01 SC-3, distros, reproducibility gate, forwarder.tools.call span) — those are Phase 58, run in parallel.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STORE-01 | Store opens at daemon start; auto-quarantine `.corrupt.<ts>` + rebuild fresh on corruption | DuckDB upstream uses `IO Error: Could not set lock on file` for both stale-lock AND corruption — the wrapper must distinguish (lock = retry-then-fail, corruption = quarantine). See "Corruption detection ergonomics" finding below. |
| STORE-02 | Disabled path returns `Kind: Unsupported` with remediation; verified by integration test | `serr.Unsupported` already exists (`internal/errors/kinds.go:17`). v1.9 Phase 51.1 stub pattern (`internal/repomap/extractor_nocgo.go`, `internal/kernel/edit/treesitter_nocgo.go`) is the template. |
| STORE-03 | Schema versioning: forward-incompatible → full reindex + log; backward-compatible → in-place migrate; version stamped per snapshot row | SPEC §9.1-§9.2: `semantic_schema_version` table + `schema_version INTEGER NOT NULL` column on `semantic_snapshots`. Open question: integer monotone vs (major, minor) — recommendation in this doc. |
| STORE-04 | Effective-read API for files/symbols/references/edges; per-entity unit test | SPEC §10 has the exact pseudocode for `QueryEffectiveEdges`. Phase 57 implements the API surface; Phase 59-60 populate the data the API reads. |
| STORE-05 | All `semantic_index.*` keys flow through 4-layer precedence; covered by config-coverage test | SPEC §25 lists 30+ keys across 11 sub-blocks. **No "every key has a default" test exists** — recommendation: per-feature `TestLoad_SemanticIndexDefaults` mirroring `TestLoad_ObservabilityDefaults` style. |
| STORE-06 | `go vet`-runnable lint fails build if `duckdb-go` imported outside `internal/semantic/store/` | `golang.org/x/tools/go/analysis/singlechecker` is the standard pattern; analyzer compiles to a binary, invoked via `go vet -vettool=path/to/binary`. |
| DAG-01 | `internal/phasegraph/` provides `PhaseSpec`, `PhaseGraph`, `ValidatePhaseGraph`, `RunPhaseGraph`; stdlib only | SPEC §39.2-§39.3 has Go signatures + Kahn pseudocode. ~80-150 LOC. |
| DAG-02 | Semantic-index, live-update, eval pipelines each declared as typed phase DAG with `Requires`/`Provides`/`Run`/`Validate`/`Shutdown` | SPEC §39.5-§39.7 has phase ID lists. Phase 57 ships shapes (typed phase ID constants + `Requires`/`Provides` decls); Phase 60/67 wire `Run` bodies. |
| DAG-03 | Duplicate ID, missing dep, cycle each fail validation; error returns offending IDs; optional `.helix/debug/phasegraph-*.dot` | SPEC §39.9 names the DOT file convention. `phase_graph.dump_dot_on_error` config key in SPEC §25 controls write. |
| DAG-04 | Bootstrap stays imperative; `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)` marker recorded | Pure source-comment requirement. Where in `daemon.go` the marker lives is Claude's discretion — recommendation: top of `newDaemon`. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| DuckDB file open / quarantine / rebuild | Layer 1.5 (`internal/semantic/store/`) | — | Fact store; sole `duckdb-go` importer per STORE-06. |
| Schema migrations | Layer 1.5 (`internal/semantic/store/migrations.go`) | — | Lives next to store; SPEC §6 layout. |
| Effective-read API | Layer 1.5 (`internal/semantic/store/effective.go`) | — | Reads snapshot ⊕ overlay; consumers in Phase 60+. |
| `semantic_index.*` config keys | Layer 0/3 (`internal/config/`) | Layer 1.5 (`internal/semantic/config.go` mirror) | koanf flows from `internal/config/` to a typed `semantic.Config` consumed by the service. |
| CGO=0 stub for store | Layer 1.5 (`internal/semantic/store/duckdb_nocgo.go`) | — | Mirrors `repomap/extractor_nocgo.go` + `edit/treesitter_nocgo.go` pattern. |
| Daemon `OpenSemanticStore` step | Layer 0 (`internal/daemon/daemon.go` step 6b) | Layer 1.5 (returns `*semantic.Store` to daemon) | Fail-fast core subsystem; opens at bootstrap before workspace activation. |
| `internal/phasegraph/` library | Layer 1.5 (`internal/phasegraph/`) | — | Stdlib-only library; consumed by Phase 60/67 from Layer 1.5 also. |
| Vet-tool `cmd/vet-noduckdb/` | Layer 0 (`cmd/`) | — | Standalone analyzer binary; invoked from `make vet` via `-vettool`. |
| `go vet` integration in Makefile | Build / CI | — | `make vet` builds the analyzer then passes it via `-vettool=`. |

**Note on Layer 1.5 placement:** SPEC §5 puts the "Live Semantic Service" between the Code Intelligence Kernel (Layer 1) and Agent Semantic Tools (which become a Layer 2 skill in Phase 64). Existing CLAUDE.md has 4 layers (MCP runtime, Kernel, Skills, Profiles). The semantic service is conceptually between Kernel and Skills — neither lives. SUMMARY.md calls this "Layer 1.5". For Phase 57, no architectural layer renaming is required — the directory `internal/semantic/` is sufficient signal of placement; the planner does not need to touch CLAUDE.md's 4-layer text in this phase.

## Standard Stack

### Core (new in v1.10)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/duckdb/duckdb-go/v2` | v2.10502.0 (DuckDB 1.5.2, released 2026-04-14) | DuckDB driver via `database/sql` | Official DuckDB Foundation repo since v2.5.0; `marcboeker/go-duckdb` archived 2025-10-20. Bundled prebuilt static libs for darwin/linux × amd64/arm64 + windows-amd64. CGO=1 required. |
| `github.com/duckdb/duckdb-go-bindings` | (transitive via duckdb-go/v2) | C bindings + prebuilt static libs | Pulled in automatically; do not import directly. |
| `golang.org/x/tools/go/analysis/singlechecker` | (already transitive via existing deps) | `go vet -vettool` analyzer harness | Standard Go toolchain pattern for custom analyzers. |

**Verification:**
```bash
npm view  # N/A — Go modules
# Verified via:
# WebFetch https://github.com/duckdb/duckdb-go → "Latest Version: v2.10502.0 (released April 14, 2026)"
# WebFetch https://github.com/duckdb/duckdb-go-bindings → platforms list
```

### Supporting (already vendored)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/knadh/koanf/v2` | (existing) | 4-layer config precedence | Add `semantic_index.*` keys to `defaults.go` + `SerenaConfig` struct fields. |
| `log/slog` | stdlib | Structured logging for quarantine events | Standard project log style. |
| `database/sql` | stdlib | DuckDB connection lifecycle | duckdb-go hooks into `database/sql`; use `sql.OpenDB` for init callbacks. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `duckdb-go/v2` (CGO=1) | `modernc.org/sqlite` (CGO-free, already used for memory FTS5) | SPEC §4 ADR-001 explicitly chooses DuckDB for its analytical-query strengths. SQLite would lose the columnar engine, JSON-as-typed-storage, and DuckDB-specific `CHECKPOINT`/`VACUUM` semantics. **Rejected by ADR-001.** |
| `singlechecker` standalone analyzer | Inline `go vet` — no custom analyzer | Inline vet has no way to enforce import-path restrictions. **Rejected.** |
| Generic `phasegraph.PhaseSpec[T]` | Non-generic `PhaseSpec` with `interface{}` outputs | Generics complicate the dependency-resolution map (`map[PhaseID]any` is fine; `map[PhaseID]T` doesn't compose across phase outputs). **Recommend non-generic for v1.10.** |

**Installation (Go modules — go.mod additions):**
```bash
go get github.com/duckdb/duckdb-go/v2@v2.10502.0
# transitively pulls duckdb/duckdb-go-bindings
```

**Version verification:** v2.10502.0 confirmed via `https://github.com/duckdb/duckdb-go` (latest tag April 2026). DuckDB version is 1.5.2.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌──────────────────────────┐
                         │     daemon bootstrap     │
                         │    (imperative, v1.10)   │
                         └────────────┬─────────────┘
                                      │
                  ┌───────────────────┼───────────────────┐
                  │                   │                   │
        step 6a   ▼          step 6b  ▼         step 6+   ▼
       ┌─────────────────┐  ┌──────────────────┐  ┌────────────────┐
       │ CGO=0 refusal   │  │ OpenSemanticStore│  │ existing steps │
       │ (Phase 51.1)    │  │  (NEW Phase 57)  │  │ (langreg, kern)│
       └────────┬────────┘  └─────────┬────────┘  └───────┬────────┘
                │                     │                   │
                ▼                     ▼                   ▼
   ┌──────────────────────────────────────────────────────────────┐
   │                  internal/semantic/store/                    │
   │  ┌────────────┐ ┌────────────┐ ┌──────────┐ ┌─────────────┐ │
   │  │ duckdb.go  │ │migrations  │ │snapshot  │ │  overlay    │ │
   │  │  (open/    │ │  (schema   │ │   .go    │ │   .go       │ │
   │  │  quarantine│ │  versions) │ │          │ │             │ │
   │  │  /rebuild) │ │            │ │          │ │             │ │
   │  └────────────┘ └────────────┘ └──────────┘ └─────────────┘ │
   │  ┌────────────────────────────────────────────────────────┐ │
   │  │ effective.go (snapshot ⊕ overlay − tombstones, §10)    │ │
   │  └────────────────────────────────────────────────────────┘ │
   │  ┌────────────────────────────────────────────────────────┐ │
   │  │ duckdb_nocgo.go  //go:build !cgo  →  serr.Unsupported  │ │
   │  └────────────────────────────────────────────────────────┘ │
   └──────────────────────────────────────────────────────────────┘
                                      │
                                      ▼ (sql.DB or nil-on-disabled)
   ┌──────────────────────────────────────────────────────────────┐
   │     consumers (Phase 59-67) — read store via interface       │
   └──────────────────────────────────────────────────────────────┘

                         ┌──────────────────────────┐
                         │  internal/phasegraph/    │
                         │  (stdlib-only library)   │
                         │ ┌─────────┐ ┌──────────┐ │
                         │ │phase.go │ │  dag.go  │ │
                         │ │PhaseSpec│ │ Kahn topo│ │
                         │ └─────────┘ └──────────┘ │
                         │ ┌──────────┐ ┌─────────┐ │
                         │ │validate  │ │  run    │ │
                         │ │ .go      │ │  .go    │ │
                         │ └──────────┘ └─────────┘ │
                         │ ┌──────────────────────┐ │
                         │ │   shutdown.go        │ │
                         │ │ (reverse topo)       │ │
                         │ └──────────────────────┘ │
                         └────────────┬─────────────┘
                                      │
                                      ▼ (consumed by Phase 60/67 — NOT bootstrap)
                       Live update DAG + Eval DAG
                       (semantic-index DAG also declared as shape)

                                      ▲
                                      │  (vet)
                         ┌──────────────────────────┐
                         │  cmd/vet-noduckdb/       │
                         │  singlechecker analyzer  │
                         │  fails non-store imports │
                         └──────────────────────────┘
```

### Recommended Project Structure (Phase 57 surface only)

```
internal/semantic/
├── config.go              # typed Config struct mirroring koanf semantic_index.*
├── types.go               # SnapshotID, FileID, SymbolID, Freshness, etc. (SPEC §7)
└── store/
    ├── duckdb.go          # //go:build cgo  — Open/Close, schema apply, quarantine
    ├── duckdb_nocgo.go    # //go:build !cgo — Open returns serr.ErrUnsupported
    ├── migrations.go      # //go:build cgo  — schema_version table, in-place vs reindex
    ├── snapshot.go        # //go:build cgo  — BeginSnapshot/CommitSnapshot/AbortSnapshot
    ├── overlay.go         # //go:build cgo  — OverlayTx skeleton (no-op rows in v1.10 P57)
    ├── effective.go       # //go:build cgo  — QueryEffective{Files,Symbols,References,Edges}
    ├── store_test.go      # //go:build cgo  — open/quarantine/rebuild integration test
    └── README.md          # cross-process rule + duckdb-go boundary

internal/phasegraph/
├── phase.go               # PhaseSpec, PhaseID, PhaseOutput, PhaseDeps types
├── dag.go                 # PhaseGraph build, FindDuplicateIDs, FindMissingDeps, FindCycle
├── validate.go            # ValidatePhaseGraph (entry point)
├── run.go                 # RunPhaseGraph + Kahn ordering
├── shutdown.go            # Reverse-topo shutdown helper
├── dot.go                 # WriteDOT for .helix/debug/phasegraph-*.dot
├── phasegraph_test.go     # cycle/missing/dup/happy-path coverage
└── pipelines/             # OPTIONAL: typed phase ID constants per pipeline
    ├── semantic.go        # SemanticIndexPhases = []PhaseSpec{discover_files, ...}
    ├── live.go            # LiveUpdatePhases = []PhaseSpec{collect_events, ...}
    └── eval.go            # EvalPhases = []PhaseSpec{prepare_workspace, ...}

cmd/vet-noduckdb/
└── main.go                # singlechecker.Main(noduckdb.Analyzer)

internal/lint/noduckdb/    # (or wherever the analyzer body lives)
├── analyzer.go            # *analysis.Analyzer that walks AST imports
└── analyzer_test.go       # analysistest fixture (testdata/src/badpkg/imports.go)
```

### Pattern 1: CGO=0 Stub (mirror Phase 51.1)

**What:** Two source files in the same package, mutually exclusive via `//go:build cgo` and `//go:build !cgo`. The CGO=1 file holds the real implementation; the CGO=0 file holds an empty stub that returns `serr.ErrUnsupported` from any method that gets called (defense-in-depth — daemon refuses CGO=0 at step 6a anyway).

**When to use:** Always when introducing a new CGO-dependent package — preserves the v1.9 single-binary policy and keeps `go build` clean across the goreleaser matrix.

**Example (template — verbatim from `internal/repomap/extractor_nocgo.go`):**
```go
// Source: internal/repomap/extractor_nocgo.go (v1.9 Phase 51.1)
//go:build !cgo

package repomap

import (
    "fmt"

    "github.com/agenthands/helix/internal/treesitter"
)

// TagExtractor under !cgo is a stub; the daemon refuses to start before any
// instance is exercised at runtime (see internal/daemon/daemon.go).
type TagExtractor struct{}

// NewTagExtractor under !cgo returns a non-nil empty stub to mirror sibling
// stubs (NewBodyExtractor, NewElisionRenderer). Loud failure is delivered by
// the daemon refusal hook before any caller is exercised; methods on this
// stub still return errors as defense-in-depth if the refusal is bypassed.
func NewTagExtractor(_ *treesitter.GrammarRegistry) (*TagExtractor, error) {
    return &TagExtractor{}, nil
}

// Extract is unreachable under !cgo at runtime (daemon refuses). Returns an
// error sentinel for safety if the stub is ever exercised directly.
func (*TagExtractor) Extract(_ []byte, _ string, _ string) ([]Tag, error) {
    return nil, fmt.Errorf("repomap.TagExtractor.Extract: tree-sitter unavailable (CGO_ENABLED=0)")
}

// Close is a no-op under !cgo.
func (*TagExtractor) Close() {}
```

**Mirror for Phase 57:** `internal/semantic/store/duckdb_nocgo.go` returns `serr.ErrUnsupported` from `OpenSemanticStore`; daemon step 6b sees the error and (per current step 6a precedent) the binary refuses to start under CGO=0 — except CGO=0 binaries already refuse at step 6a, so the stub here is only ever exercised in unit tests that build with CGO=0 explicitly.

### Pattern 2: Three-tier store open (mirror three-tier LS installer)

**What:** `OpenSemanticStore(path)` follows three resolution tiers analogous to v1.0 Phase 3's `langregistry.NewInstaller`:

1. **Tier 1 — open existing:** if `<workspace>/.helix/semantic.duckdb` exists and opens cleanly, validate schema_version, apply backward-compatible migrations in place, return.
2. **Tier 2 — quarantine + rebuild:** if open fails with a corruption signature OR schema_version is forward-incompatible, rename to `<path>.corrupt.<unix-ts>`, log a structured event with the failure reason, create a fresh DB at the original path, return the fresh handle.
3. **Tier 3 — hard fail:** if Tier 2 also fails (disk full, permission denied on the directory), return error and the daemon refuses to start.

**Why standard:** Mirrors the v1.0 three-tier LS installer (`PATH lookup > managed download > helpful error`) and the SPEC §29.1 "1. Detect open/query failure. 2. Move database to timestamped .corrupt backup. 3. Report degraded semantic index status. 4. Allow rebuild." flow.

**When to use:** Single entry point at daemon bootstrap step 6b.

**Example (signature):**
```go
// Source: SPEC §8 + §29.1, plus mirror of langregistry.NewInstaller pattern
package store

type OpenOptions struct {
    Path          string         // <workspace>/.helix/semantic.duckdb
    MemoryLimit   string         // semantic_index.store.memory_limit
    Threads       int            // semantic_index.store.threads
    Logger        *slog.Logger
    OnQuarantine  func(QuarantineEvent) // metric hook for helix_semantic_store_quarantine_total
}

type QuarantineEvent struct {
    OriginalPath  string
    QuarantinePath string
    Reason        string  // "corruption" | "schema_forward_incompatible"
}

// Open returns a ready-to-use store. On corruption or forward-incompatible
// schema, quarantines the existing file and rebuilds fresh. Returns an error
// only when both open and rebuild fail.
func Open(ctx context.Context, opts OpenOptions) (*Store, error) { ... }
```

### Pattern 3: phasegraph library (Kahn topo + reverse-topo shutdown)

**What:** Pure-Go DAG with deterministic Kahn topological sort. ~80-150 LOC. SPEC §39.3 already supplies the algorithm sketch.

**Example (signature, verbatim from SPEC §39.2-39.3 + adaptation):**
```go
// Source: SPEC §39.2-§39.3
package phasegraph

type PhaseID string

type PhaseOutput interface{} // intentionally generic; consumers type-assert

type PhaseDeps map[PhaseID]PhaseOutput

type PhaseRunFunc func(ctx context.Context, deps PhaseDeps) (PhaseOutput, error)
type PhaseValidateFunc func(output PhaseOutput) error
type PhaseShutdownFunc func(ctx context.Context, output PhaseOutput) error

type PhaseSpec struct {
    ID       PhaseID
    Requires []PhaseID
    Provides []string
    Run      PhaseRunFunc
    Validate PhaseValidateFunc  // optional
    Shutdown PhaseShutdownFunc  // optional
}

type PhaseGraph struct {
    Order         []PhaseSpec
    ShutdownOrder []PhaseSpec  // reverse of Order
}

type PhaseGraphError struct {
    Kind    string    // "duplicate_phase" | "missing_dependency" | "cycle"
    Phase   PhaseID
    Missing []PhaseID
    Cycle   []PhaseID
}

func ValidatePhaseGraph(phases []PhaseSpec) (*PhaseGraph, error) { ... }
func RunPhaseGraph(ctx context.Context, graph *PhaseGraph) (*PhaseGraphResult, error) { ... }
func WriteDOT(w io.Writer, graph *PhaseGraph) error { ... }
```

### Pattern 4: vet-tool analyzer (singlechecker)

**What:** Custom `*analysis.Analyzer` that walks each package's import block; if the package path is NOT under `internal/semantic/store/` AND any import matches `github.com/duckdb/duckdb-go/v2` (or any sub-path), emit a diagnostic. Compiled to `cmd/vet-noduckdb/main.go` and invoked via `go vet -vettool=$(BIN)/vet-noduckdb ./...`.

**Example (verbatim singlechecker template):**
```go
// Source: pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker
package main

import (
    "github.com/agenthands/helix/internal/lint/noduckdb"
    "golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(noduckdb.Analyzer) }
```

**Analyzer body (sketch):**
```go
package noduckdb

import (
    "go/ast"
    "strings"

    "golang.org/x/tools/go/analysis"
)

const allowedPkgPrefix = "github.com/agenthands/helix/internal/semantic/store"
const forbiddenImport = "github.com/duckdb/duckdb-go"

var Analyzer = &analysis.Analyzer{
    Name: "noduckdb",
    Doc:  "fails if duckdb-go is imported outside internal/semantic/store/",
    Run: func(pass *analysis.Pass) (interface{}, error) {
        if strings.HasPrefix(pass.Pkg.Path(), allowedPkgPrefix) {
            return nil, nil
        }
        for _, file := range pass.Files {
            for _, imp := range file.Imports {
                path := strings.Trim(imp.Path.Value, `"`)
                if strings.HasPrefix(path, forbiddenImport) {
                    pass.Reportf(imp.Pos(),
                        "duckdb-go may only be imported from %s (got %s)",
                        allowedPkgPrefix, pass.Pkg.Path())
                }
            }
        }
        return nil, nil
    },
}
```

**Makefile integration:**
```make
VETTOOL=$(shell go env GOPATH)/bin/vet-noduckdb
vet: $(VETTOOL)
	$(GO) vet ./...
	$(GO) vet -vettool=$(VETTOOL) ./...
$(VETTOOL): cmd/vet-noduckdb/main.go internal/lint/noduckdb/*.go
	$(GO) install ./cmd/vet-noduckdb
```

### Anti-Patterns to Avoid

- **Importing `duckdb-go` from `internal/semantic/`** (the parent of `store/`). The vet rule must catch this — semantic must always go through `store/` so the cross-process rule (C5) is enforced architecturally.
- **Schema versioning via free-form string ("v1", "v1.1", "alpha")** — not orderable, not comparable. Use a single integer monotone counter (recommendation; see Open Question #1).
- **Touching the daemon's middleware install order** in this phase. STORE-02 + STORE-05 do not require any middleware — they are bootstrap-step + config concerns. Phase 66 owns middleware.
- **Adding `semantic_index.*` config keys without a default in `internal/config/defaults.go`.** koanf's confmap provider requires a default for every key under the precedence chain; missing defaults cause silent zero-value reads.
- **Treating `IO Error: Could not set lock on file` as corruption.** This error is the lock-collision signature (DuckDB single-writer). Corruption produces different errors (`Catalog Error`, `Serialization Error`, `Could not parse the database`). The store wrapper must distinguish — see "Common Pitfalls" below.
- **Hand-rolling a topological sort with `for-loop+visited-map`.** Kahn's algorithm (SPEC §39.3) is correct, deterministic, and 30 lines.
- **Using Go generics for `PhaseSpec`.** `PhaseOutput interface{}` plus type-assertion in consumers is simpler and matches the SPEC signature.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Embedded analytical SQL store | bytes-on-disk + custom query layer | `duckdb-go/v2` | Decades of MVCC, columnar engine, `CHECKPOINT`/`VACUUM`, JSON-as-typed-storage. ADR-001. |
| 4-layer config precedence | custom YAML merger | existing `koanf` flow in `internal/config/loader.go` | Already shipped, tested, and documented. STORE-05 explicitly says "existing 4-layer precedence". |
| Custom Go static analyzer driver | hand-roll AST walker + CLI | `golang.org/x/tools/go/analysis/singlechecker` | Standard Go toolchain pattern; integrates with `go vet -vettool` for free. |
| Topological sort | DFS-with-visited or recursive walker | Kahn's algorithm (SPEC §39.3 has the sketch) | Kahn is iterative, deterministic, surfaces cycles cleanly, and pairs naturally with reverse-topo for shutdown. |
| Corruption-detection heuristic | regex on error messages | typed error inspection from `duckdb-go` driver | Use `errors.As` on the driver's typed errors where available; only fall back to message inspection for edge cases. See Open Question #3. |
| CGO=0 stub system | runtime `if cgoAvailable { ... }` checks | `//go:build cgo` / `//go:build !cgo` files | Compile-time elimination; v1.9 Phase 51.1 set the precedent. |

**Key insight:** Phase 57 is overwhelmingly **integration**, not invention. Every load-bearing pattern (CGO stub, koanf layering, Kahn topo, singlechecker, three-tier installer) has a verbatim precedent in the existing codebase or in standard Go tooling. The only genuinely new mechanical work is the DuckDB driver wrapper (open/quarantine/rebuild + schema_version compare) and the analyzer body (~50 LOC AST walk).

## Runtime State Inventory

> Phase 57 is greenfield (introduces new packages and a new file format). No rename / refactor / migration work is in scope. There is **no pre-existing runtime state** to inventory.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no `<workspace>/.helix/semantic.duckdb` files exist before this phase. | None. |
| Live service config | None — Phase 57 introduces `semantic_index.*` keys; no prior keys to migrate. | None. |
| OS-registered state | None — no Task Scheduler / launchd / pm2 / systemd entries reference semantic anything. | None. |
| Secrets / env vars | None — no new env vars in this phase (existing `HELIX_*` env-var renaming was completed in v1.9 Phase 52). | None. |
| Build artifacts | After this phase, `cmd/vet-noduckdb` will produce a binary in `$GOPATH/bin`. Treated as a build artifact, not source. | Document `make vet` builds it on demand. |

**Nothing found in any category** — confirmed by the absence of `internal/semantic/` and `internal/phasegraph/` directories in the current tree (verified by `ls`).

## Common Pitfalls

### Pitfall 1: DuckDB lock-vs-corruption error confusion (C5 + STORE-01 collision)

**What goes wrong:** STORE-01 says "auto-quarantine then rebuild" on corruption. But DuckDB's `IO Error: Could not set lock on file "..."` message is the **lock-collision** signature when another process holds the file (the C5 cross-process rule). If the store wrapper treats lock collisions as corruption, it will quarantine a perfectly healthy file every time a developer accidentally opens the daemon twice or runs the eval harness against the dev daemon's workspace.

**Why it happens:** DuckDB upstream returns `IOException`/`IO Error` for both. Distinguishing requires inspecting the message body (`"Could not set lock"` vs `"Catalog Error"` / `"could not parse the database file"` / `"Serialization Error"`).

**How to avoid:**
- Open path: try open. On error, classify:
  - Message contains `"Could not set lock"` → **lock collision**, return error to caller (daemon refuses to start with "another helix daemon may be running on this workspace; check `lsof` or stop the other daemon"). Do NOT quarantine.
  - Message contains `"Catalog Error"` / `"Serialization"` / `"could not parse"` / open succeeds but `SELECT 1` fails → **corruption**, quarantine + rebuild.
  - Open succeeds and `SELECT 1` succeeds but `SELECT version FROM semantic_schema_version` returns a forward-incompatible version → **schema-incompatible**, quarantine + rebuild fresh (full reindex; STORE-03).
- Document this taxonomy in `internal/semantic/store/README.md` and a unit test that injects each failure shape via a mock.

**Warning signs:**
- `helix_semantic_store_quarantine_total` increments on a developer machine after a clean restart → almost certainly a lock-collision misclassification.
- Quarantined `.duckdb.corrupt.<ts>` file actually opens cleanly when copied elsewhere → misclassification.

### Pitfall 2: Phase 57 STORE-02 verification false-positive (CGO=0 path tests pass without exercising store)

**What goes wrong:** STORE-02 says an integration test must run with `semantic_index.enabled=false` and prove every semantic-dependent tool returns `Kind: Unsupported`. Phase 57 has **no semantic-dependent tools yet** (those land in Phase 64). A naive integration test that boots a daemon with `enabled=false` and checks `get_health` will pass without exercising any of the surface STORE-02 actually protects.

**Why it happens:** Phase 57 ships the store; consumers (Phase 64) ship the tools. The verification surface is split across phases.

**How to avoid:**
- For Phase 57, the verifiable invariant is narrower: **the store remains uninitialized when `enabled=false`** (i.e., no `.helix/semantic.duckdb` file appears) AND **the store remains uninitialized under CGO=0** (compile-time stub). Test:
  - CGO=1, `enabled=true`: file appears, quarantine path works, `Open` returns non-nil.
  - CGO=1, `enabled=false`: file does NOT appear, `Open` returns `nil, nil` (or an explicit "disabled" sentinel — pick one in the planning phase).
  - CGO=0 build: `go test -tags=!cgo ./internal/semantic/store/...` returns `serr.ErrUnsupported` from every Store method.
- The full STORE-02 acceptance test (semantic-dependent tools return `Kind: Unsupported`) lands in Phase 64 alongside the tools themselves. **The Phase 57 RESEARCH explicitly flags this** so the planner does not over-scope.

### Pitfall 3: Schema-version representation locks the migration story (STORE-03 ambiguity)

**What goes wrong:** SPEC §9.1 has `version INTEGER` but says nothing about whether the integer encodes "single counter" (v=1, v=2, v=3 — every bump is forward-incompatible) or "major.minor packed" (v=10000, v=10001, v=10002 — bumps under same major are backward-compatible). Until this is locked, `migrations.go` cannot be written.

**How to avoid:**
- **Recommendation:** single integer monotone counter, with a hand-maintained `migrations.go` slice `[]Migration{{From: 1, To: 2, Compatible: true, Apply: ...}, {From: 2, To: 3, Compatible: false, Apply: nil}}`. The `Compatible: bool` field encodes forward-compatibility. A `Compatible: false` migration means "DROP and rebuild" (full reindex). A `Compatible: true` migration runs `Apply` in place.
- This avoids a (major, minor) packing scheme that has historically caused issues (gopls's snapshot version did exactly this and switched to a single counter).
- Add a unit test that asserts every consecutive pair of migrations has either a non-nil `Apply` or `Compatible: false` (otherwise the chain is broken).

### Pitfall 4: phasegraph DAG declared but never validated → DAG-03 silently un-enforced

**What goes wrong:** DAG-02 requires the semantic-index, live-update, and eval pipelines be declared as typed phase DAGs. DAG-03 requires duplicate/missing-dep/cycle to fail validation. If the declarations are made but no test calls `ValidatePhaseGraph` on each, a regression in (say) Phase 60 that introduces a cycle would not be caught.

**How to avoid:**
- For each declared pipeline (`SemanticIndexPhases`, `LiveUpdatePhases`, `EvalPhases`), add a `TestValidatePhaseGraph_<Pipeline>` test that calls `ValidatePhaseGraph(phases)` and asserts no error.
- Also add three negative tests using synthetic phase slices: one with a duplicate ID, one with a missing dep, one with a cycle. These tests live in `internal/phasegraph/phasegraph_test.go` (not pipeline-specific) and exercise the validator directly.

### Pitfall 5: Daemon bootstrap-migration TODO marker drifts (DAG-04)

**What goes wrong:** DAG-04 says a `// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)` marker is recorded. If the marker is in the wrong file or in a comment block that gets reformatted, the v1.11 plan loses its anchor.

**How to avoid:**
- Place the marker at the top of `func newDaemon(...)` in `internal/daemon/daemon.go`, line ~145, as a leading comment block. This is the natural anchor for the future migration: every step inside `newDaemon` becomes a phase, the function becomes `RunPhaseGraph(ctx, BootstrapGraph)`.
- Add a one-line grep test in `internal/daemon/daemon_test.go`: `assertMarkerPresent(t, "TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)")`. Drift is then caught at test time.

### Pitfall 6: m5 daemon-config drift (cited by ROADMAP cross-check, owned by Phase 57 STORE-05)

**What goes wrong:** SPEC §25 declares ~30 `semantic_index.*` keys. Adding them to `defaults.go` but not to `SerenaConfig` struct means koanf silently fails to unmarshal them. Adding them to `SerenaConfig` but not to `defaults.go` means a missing project config falls through to zero values (`""` for strings, `0` for ints, `false` for bools), which for `semantic_index.enabled` happens to be the right thing but for `semantic_index.store.threads=4` is a bug (zero threads != 4 threads).

**How to avoid:**
- Add `semantic_index.*` keys to `defaults.go` AND to a new `SemanticIndexConfig` struct nested under `SerenaConfig.SemanticIndex`.
- Add `TestLoad_SemanticIndexDefaults` mirroring `TestLoad_ObservabilityDefaults` — assert every documented key resolves to the SPEC §25 default.
- Add `TestLoad_SemanticIndex_4Layer` that proves CLI > project > user > defaults precedence on at least one representative key per sub-block (`enabled`, `store.path`, `live_updates.bulk_change_threshold`).

## Code Examples

### Daemon bootstrap step 6b — proposed insertion point

```go
// Source: NEW for Phase 57; insertion after step 6a in internal/daemon/daemon.go:203
// (immediately following the CGO=0 refusal hook; before step 6 grammar registry).

// 6b. Open semantic fact store when enabled (Phase 57, STORE-01..05).
//     Fail-fast core subsystem; on corruption auto-quarantines to
//     `<path>.corrupt.<ts>` and rebuilds fresh (SPEC §29.1, STORE-01).
//     When semantic_index.enabled=false, returns nil; consumers must guard.
//     Under CGO=0 the call site is reached only by tests built with the !cgo
//     tag — production CGO=0 binaries already refused at step 6a.
var semanticStore *semanticstore.Store
if cfg.SemanticIndex.Enabled {
    workspaceRoot := "" // resolved at lazy-activate time; for now use process cwd
    if wd, err := os.Getwd(); err == nil {
        workspaceRoot = wd
    }
    storePath := filepath.Join(workspaceRoot, ".helix", "semantic.duckdb")
    semanticStore, err = semanticstore.Open(context.Background(), semanticstore.OpenOptions{
        Path:        storePath,
        MemoryLimit: cfg.SemanticIndex.Store.MemoryLimit,
        Threads:     cfg.SemanticIndex.Store.Threads,
        Logger:      logger,
        OnQuarantine: func(ev semanticstore.QuarantineEvent) {
            // metric hook (Phase 65 wires the actual metric)
            logger.Warn("semantic store quarantined and rebuilt",
                "original", ev.OriginalPath,
                "quarantine", ev.QuarantinePath,
                "reason", ev.Reason)
        },
    })
    if err != nil {
        return nil, fmt.Errorf("opening semantic store: %w", err)
    }
}
// semanticStore is non-nil only when CGO=1 AND enabled=true; consumers must
// guard. It is wired into the Daemon struct and consumed by Phase 64 tools.
```

### Effective-read API skeleton (SPEC §10 verbatim)

```go
// Source: SPEC §10 (verbatim pseudocode adapted to Phase 57's typed signatures)
package store

func (s *Store) QueryEffectiveEdges(ctx context.Context, req EdgeQuery) ([]EdgeFact, error) {
    base, err := s.querySnapshotEdges(ctx, req.SnapshotID, req.Filters)
    if err != nil {
        return nil, fmt.Errorf("query snapshot edges: %w", err)
    }
    overlay, err := s.queryOverlayEdges(ctx, req.RepoID, req.Filters)
    if err != nil {
        return nil, fmt.Errorf("query overlay edges: %w", err)
    }

    deleted := map[EdgeID]bool{}
    upserted := map[EdgeID]EdgeFact{}
    for _, e := range overlay {
        if e.Status == "deleted" {
            deleted[e.EdgeID] = true
            continue
        }
        upserted[e.EdgeID] = e.ToFact()
    }

    out := make([]EdgeFact, 0, len(base)+len(upserted))
    for _, e := range base {
        if deleted[e.EdgeID] {
            continue
        }
        if replacement, ok := upserted[e.EdgeID]; ok {
            out = append(out, replacement)
            delete(upserted, e.EdgeID)
            continue
        }
        out = append(out, e)
    }
    for _, e := range upserted {
        out = append(out, e)
    }
    return out, nil
}
```

### phasegraph DAG validation entry (SPEC §39.3 verbatim)

```go
// Source: SPEC §39.3 (verbatim pseudocode)
package phasegraph

func ValidatePhaseGraph(phases []PhaseSpec) (*PhaseGraph, error) {
    g := buildPhaseGraph(phases)

    if dup := g.findDuplicateIDs(); dup != nil {
        return nil, &PhaseGraphError{Kind: "duplicate_phase", Phase: *dup}
    }
    if missing := g.findMissingDependencies(); len(missing) > 0 {
        return nil, &PhaseGraphError{Kind: "missing_dependency", Missing: missing}
    }
    if cycle := g.findCycle(); len(cycle) > 0 {
        return nil, &PhaseGraphError{Kind: "cycle", Cycle: cycle}
    }
    order := kahnSort(g)
    return &PhaseGraph{Order: order, ShutdownOrder: reverse(order)}, nil
}
```

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All | ✓ | 1.25+ (v1.9 Phase 50) | — |
| C compiler (CGO=1 builds) | `duckdb-go/v2` link step | ✓ on dev (clang/gcc); ✗ on CGO=0 release archives by design | system clang/gcc | CGO=0 binaries refuse at daemon step 6a (Phase 51.1) — no in-binary fallback. Users wanting semantic_index must `go build` locally with CGO=1, or wait for a future CGO=1 split release. |
| `golang.org/x/tools/go/analysis/singlechecker` | `cmd/vet-noduckdb` | ✓ (transitive via existing tooling) | (latest) | — |
| Disk write to `<workspace>/.helix/` | STORE-01 | ✓ on dev / CI; subject to per-workspace permissions | — | Open returns error → daemon refuses to start with a clear "permission denied on .helix/" message. |

**Missing dependencies with no fallback:** None for the dev/CI loop. The CGO=1-required-for-semantic constraint is **architectural** and explicitly accepted per Phase 51.1; it is not a missing dependency that this phase is responsible for resolving.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (project default — no external test framework). |
| Config file | `go.mod` (no separate test config). |
| Quick run command | `go test ./internal/semantic/store/... ./internal/phasegraph/... ./internal/lint/noduckdb/...` |
| Full suite command | `go test ./...` (matches `make test`). |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STORE-01 | Open clean DB | unit | `go test ./internal/semantic/store/ -run TestOpen_Fresh` | ❌ Wave 0 |
| STORE-01 | Quarantine corrupt DB + rebuild | integration | `go test ./internal/semantic/store/ -run TestOpen_QuarantineCorrupt` | ❌ Wave 0 |
| STORE-01 | Hard fail when both open and rebuild fail | unit (mocked fs) | `go test ./internal/semantic/store/ -run TestOpen_HardFailWhenRebuildFails` | ❌ Wave 0 |
| STORE-02 | CGO=0 stub returns serr.ErrUnsupported | unit | `CGO_ENABLED=0 go test ./internal/semantic/store/...` | ❌ Wave 0 |
| STORE-02 | CGO=1 + enabled=false: store is nil, no file created | integration | `go test ./internal/daemon/ -run TestNewDaemon_SemanticDisabled_NoFile` | ❌ Wave 0 |
| STORE-03 | Forward-incompatible version triggers full reindex | unit | `go test ./internal/semantic/store/ -run TestMigrate_ForwardIncompatible_Reindex` | ❌ Wave 0 |
| STORE-03 | Backward-compatible bump migrates in place | unit | `go test ./internal/semantic/store/ -run TestMigrate_BackwardCompatible_InPlace` | ❌ Wave 0 |
| STORE-03 | schema_version stamped in every snapshot row | unit | `go test ./internal/semantic/store/ -run TestSnapshot_StampsSchemaVersion` | ❌ Wave 0 |
| STORE-04 | Effective files: snapshot ⊕ overlay − tombstones | unit | `go test ./internal/semantic/store/ -run TestEffectiveFiles_OverlayOverridesSnapshot` | ❌ Wave 0 |
| STORE-04 | Same for symbols, references, edges (4 separate per-entity tests) | unit | `go test ./internal/semantic/store/ -run TestEffective` | ❌ Wave 0 |
| STORE-05 | Every `semantic_index.*` key has a default in `defaults.go` | unit | `go test ./internal/config/ -run TestLoad_SemanticIndexDefaults` | ❌ Wave 0 |
| STORE-05 | 4-layer precedence works on representative key per sub-block | unit | `go test ./internal/config/ -run TestLoad_SemanticIndex_4Layer` | ❌ Wave 0 |
| STORE-06 | `duckdb-go` import in non-allowed package fails analyzer | golden / `analysistest` | `go test ./internal/lint/noduckdb/ -run TestAnalyzer` | ❌ Wave 0 |
| STORE-06 | `make vet` runs the analyzer end-to-end | smoke (build + run) | `make vet` | Makefile target Wave 0 |
| DAG-01 | `ValidatePhaseGraph` happy path | unit | `go test ./internal/phasegraph/ -run TestValidate_Happy` | ❌ Wave 0 |
| DAG-01 | `RunPhaseGraph` runs in topological order | unit | `go test ./internal/phasegraph/ -run TestRun_TopologicalOrder` | ❌ Wave 0 |
| DAG-01 | Reverse-topo shutdown order | unit | `go test ./internal/phasegraph/ -run TestShutdown_ReverseTopologicalOrder` | ❌ Wave 0 |
| DAG-02 | `SemanticIndexPhases` validates without error | unit | `go test ./internal/phasegraph/pipelines/ -run TestSemanticIndex_ValidatesClean` | ❌ Wave 0 |
| DAG-02 | `LiveUpdatePhases` validates without error | unit | `go test ./internal/phasegraph/pipelines/ -run TestLiveUpdate_ValidatesClean` | ❌ Wave 0 |
| DAG-02 | `EvalPhases` validates without error | unit | `go test ./internal/phasegraph/pipelines/ -run TestEval_ValidatesClean` | ❌ Wave 0 |
| DAG-03 | Duplicate ID fails | unit | `go test ./internal/phasegraph/ -run TestValidate_DuplicateID` | ❌ Wave 0 |
| DAG-03 | Missing dependency fails with offending IDs | unit | `go test ./internal/phasegraph/ -run TestValidate_MissingDependency` | ❌ Wave 0 |
| DAG-03 | Cycle fails with cycle path | unit | `go test ./internal/phasegraph/ -run TestValidate_Cycle` | ❌ Wave 0 |
| DAG-03 | DOT debug file written when configured | unit (tmpdir) | `go test ./internal/phasegraph/ -run TestValidate_WritesDOTOnError` | ❌ Wave 0 |
| DAG-04 | TODO marker present in `daemon.go` | unit (regex grep) | `go test ./internal/daemon/ -run TestBootstrap_PhasegraphMigrationMarkerPresent` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/semantic/store/... ./internal/phasegraph/... ./internal/lint/noduckdb/... ./internal/config/...` (~5-10s).
- **Per wave merge:** `go vet ./... && go test ./...` + `make vet` (vet-tool exercised end-to-end).
- **Phase gate:** Full `make test` + `make vet` green; CGO=0 build verified by `CGO_ENABLED=0 go build ./cmd/helix`; integration test boots a daemon with `semantic_index.enabled=true` and asserts `<workspace>/.helix/semantic.duckdb` exists with the expected schema_version row.

### False-positive verification risks (Nyquist heads-up)

| Risk | Mitigation |
|------|------------|
| Store unit tests pass but daemon never actually opens the store at bootstrap | Add `TestNewDaemon_SemanticEnabled_FileAppears` integration test that boots a daemon with `semantic_index.enabled=true` against a tmpdir workspace and asserts the file exists post-bootstrap. |
| Vet rule passes because the analyzer never sees the bad import (e.g., the test fixture doesn't compile) | Use `golang.org/x/tools/go/analysis/analysistest` with a `testdata/src/badpkg/imports.go` fixture that imports `github.com/duckdb/duckdb-go/v2`; assert exactly one diagnostic. |
| phasegraph DAG validation tests all pass on synthetic graphs but the actual `SemanticIndexPhases` declaration drifts to invalid | The per-pipeline `TestSemanticIndex_ValidatesClean` etc. catch this. |
| STORE-02 "every semantic-dependent tool returns Unsupported" passes vacuously because Phase 57 has no semantic-dependent tools yet | Explicitly scope STORE-02 verification in Phase 57 to the **store-level** invariant (file does not appear on disabled, stub returns Unsupported on CGO=0). The full tool-level acceptance lands in Phase 64 as part of TOOL-* verification. **This split is documented in this RESEARCH.md and must be preserved in PLAN.md.** |
| Schema-version test passes because every test always uses version=1 | Add a parameterized test that walks the migrations slice and asserts every adjacent pair (n, n+1) has either `Apply` non-nil or `Compatible: false`. |

### Wave 0 Gaps

- [ ] `internal/semantic/store/store_test.go` — covers STORE-01, STORE-03, STORE-04
- [ ] `internal/semantic/store/duckdb_test.go` — covers Open/quarantine/rebuild
- [ ] `internal/semantic/store/migrations_test.go` — covers STORE-03 schema migrations
- [ ] `internal/semantic/store/effective_test.go` — covers STORE-04 per-entity overlay/snapshot disagreement
- [ ] `internal/semantic/store/duckdb_nocgo_test.go` (build tag `!cgo`) — covers STORE-02 stub path
- [ ] `internal/phasegraph/phasegraph_test.go` — covers DAG-01, DAG-03 (validate, run, shutdown, dup, missing, cycle, DOT)
- [ ] `internal/phasegraph/pipelines/{semantic,live,eval}_test.go` — covers DAG-02 per-pipeline declaration validation
- [ ] `internal/lint/noduckdb/analyzer_test.go` + `testdata/src/badpkg/`, `testdata/src/goodpkg/` — covers STORE-06
- [ ] `internal/config/loader_test.go` extension: `TestLoad_SemanticIndexDefaults` + `TestLoad_SemanticIndex_4Layer` — covers STORE-05
- [ ] `internal/daemon/bootstrap_test.go` extension: `TestNewDaemon_SemanticEnabled_FileAppears`, `TestNewDaemon_SemanticDisabled_NoFile`, `TestBootstrap_PhasegraphMigrationMarkerPresent` — covers DAG-04, store wiring
- [ ] Makefile `vet` target: builds `cmd/vet-noduckdb` then runs `go vet -vettool=...`. Covers STORE-06 end-to-end.

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task.** Plans must include both as task-level checks.
- **Single canonical `GrammarRegistry`** (BUG-04, Phase 49) — Phase 57 does NOT touch grammar wiring (deferred to Phase 59).
- **Middleware install LIFO order** preserved — Phase 57 does NOT add middleware (deferred to Phase 66).
- **CGO=0 stub policy** — daemon refuses CGO=0 with remediation text; new CGO-dependent packages mirror the `//go:build cgo` / `//go:build !cgo` split established in Phase 51.1.
- **GSD workflow:** all file edits via `/gsd:execute-phase` after planning; `Edit`/`Write` outside GSD only with explicit user opt-in.
- **SMTC-first tool routing** for code-aware operations (Helix is a Go project; SMTC's Go-tier first-class support applies).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `marcboeker/go-duckdb` | `github.com/duckdb/duckdb-go/v2` | v2.5.0 (project moved); `marcboeker/go-duckdb` archived 2025-10-20 | All new code must use the duckdb/duckdb-go path; SUMMARY.md and ARCHITECTURE.md still mention the legacy path — Phase 57 locks the canonical import. |
| Hand-rolled `for-loop+visited-map` topological sort | Kahn's algorithm | (canonical algorithm, not "new") | SPEC §39.3 prescribes Kahn; the recommendation is to follow the SPEC sketch verbatim. |
| Ad-hoc `*.go` file inclusion via runtime checks | `//go:build cgo` build tags | v1.9 Phase 51.1 (DEF-51-01) | Compile-time elimination is mandatory; runtime checks are the pre-Phase-51.1 anti-pattern. |

**Deprecated/outdated:**
- Any reference to `marcboeker/go-duckdb` outside legacy commits — replace with `github.com/duckdb/duckdb-go/v2`.
- `STACK.md` and `ARCHITECTURE.md` mentioning `duckdb-go-bindings v2.10502.0` as the import: that is the **transitive** dependency, not the import. The import path is `github.com/duckdb/duckdb-go/v2`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | DuckDB corruption errors include `"Catalog Error"` / `"could not parse"` / `"Serialization Error"` while lock collisions include `"Could not set lock"` | Common Pitfalls 1 | Medium — quarantine logic may fire on lock collisions OR fail to fire on real corruption. Mitigation: write the classification logic as a `func classifyOpenError(err) corruptionVerdict` with a unit test per shape; iterate on real corruption shapes during Wave 0. [ASSUMED based on DuckDB upstream issue messages; not verified against source] |
| A2 | `internal/phasegraph/PhaseDeps` should be `map[PhaseID]PhaseOutput` | Pattern 3 | Low — consumers can wrap with helpers if the map shape proves awkward. Easy to refactor in Phase 60. [ASSUMED — SPEC §39.2 leaves PhaseDeps undefined] |
| A3 | The vet-tool integration is via Makefile `make vet` orchestrating `go vet -vettool=...`, not via a separate CI step | Pattern 4 | Low — the Makefile pattern matches existing `make build`/`make test` ergonomics. Discoverability is a slight concern; mitigated by README documentation. [ASSUMED — no existing custom analyzer in repo to copy from] |
| A4 | Schema versioning is best represented as a single integer monotone counter with a hand-maintained `[]Migration` slice | Pitfall 3 | Medium — if v1.10.x or v1.11 introduces frequent schema bumps, a (major, minor) packing might prove easier to reason about. Mitigated by the recommendation being deferred to a single struct shape (`Migration{From, To, Compatible, Apply}`) which can be re-packed without touching call sites. [ASSUMED — SPEC §9.1 leaves the encoding open] |
| A5 | Daemon bootstrap insertion is at step 6b (after CGO=0 refusal, before grammar registry) | Architectural Responsibility Map + Code Examples | Low — this is a comment/ordering choice; alternatives (step 11ish, after kernel) are also valid. Recommendation justified by "fail-fast core subsystem like langregistry" criterion. [ASSUMED — needs user/discuss-phase confirmation] |
| A6 | Generics are NOT used for `PhaseSpec[T]` | Claude's Discretion | Low — generics could be added later without breaking the non-generic API. [ASSUMED — SPEC §39.2 uses non-generic shape] |
| A7 | windows/arm64 is not in scope for any future CGO=1 release | Summary finding #2 | High **for v1.10 release planning** (not for Phase 57 execution). Phase 57 binaries continue to be CGO=0 = semantic disabled. The risk surfaces only when someone proposes a CGO=1 release. [VERIFIED that duckdb-go-bindings has no windows-arm64 prebuilt; not yet decided what Helix does with this fact] |
| A8 | `internal/config/` should add a typed `SemanticIndexConfig` struct nested under `SerenaConfig.SemanticIndex` rather than `map[string]interface{}` | Pattern + Pitfall 6 | Low — typed struct matches the shape of every other sub-block (Daemon, Logging, Observability, Degradation, WorkerPool). [ASSUMED — house style] |

**Of these:** A1, A4, A5, A8 should be confirmed in `/gsd-discuss-phase` before locking the plan. A2, A3, A6 are mechanical recommendations — defaults are fine. A7 is a release-planning concern and out of Phase 57 scope.

## Open Questions

1. **Schema versioning representation: integer monotone vs (major, minor) packed.**
   - What we know: SPEC §9.1 has `version INTEGER`; SPEC §9.2 has `schema_version INTEGER NOT NULL` per snapshot row.
   - What's unclear: how the integer encodes "forward-incompatible" vs "backward-compatible" semantics (STORE-03).
   - Recommendation: integer monotone counter + hand-maintained `[]Migration{{From, To, Compatible, Apply}}` slice. Lock in `/gsd-discuss-phase`.

2. **`go vet`-tool integration shape.**
   - What we know: STORE-06 says "go vet-runnable lint rule". `singlechecker` produces a binary invoked via `go vet -vettool=...`.
   - What's unclear: whether the build invocation is via `make vet` (recommended) or via a separate `cmd/lint/main.go` umbrella.
   - Recommendation: standalone `cmd/vet-noduckdb/main.go` + Makefile `vet` target builds and invokes it. Document in CONTRIBUTING.md.

3. **Quarantine event surface: structured log + metric, or stderr only?**
   - What we know: SPEC §29.1 says "Report degraded semantic index status."
   - What's unclear: whether to surface via `helix_semantic_store_quarantine_total` counter (requires a metric registration in v1.10) or `slog.Warn` + structured event field only.
   - Recommendation: both. The metric is cheap and aligns with v1.9's bounded-label discipline (`reason` label with allowlist `corruption`/`schema_forward_incompatible`). The slog event provides operator visibility.

4. **Where exactly in `daemon.go` step sequence does `OpenSemanticStore` slot?**
   - What we know: existing steps 1-15 in `internal/daemon/daemon.go`. Step 6a is the CGO=0 refusal hook; steps 6-11 build kernel + skills + register tools.
   - What's unclear: before kernel (step 6b → before step 6 grammar registry creation) or after kernel (post-step-11).
   - Recommendation: **step 6b**, immediately after step 6a. The store is a fail-fast core subsystem analogous to language registry (step 1) and kernel (step 5) — both come before skill init. Putting it after kernel risks coupling failure modes.

5. **Should `internal/phasegraph/` use Go generics?**
   - What we know: DAG-01 says "typed phase DAG". SPEC §39.2 has non-generic Go signatures.
   - What's unclear: whether `PhaseSpec[T]` would meaningfully improve the consumer API.
   - Recommendation: **no generics in v1.10**. SPEC sketch is non-generic; consumer Phase 60/67 can layer typed helpers if needed.

## Sources

### Primary (HIGH confidence)

- `SPEC-DRAFT.md` — §4 ADRs, §6 package layout, §7 core types, §8 store contract, §9 data model, §10 effective read, §25 configuration, §29.1 corruption recovery, §32 Phase 0 / Phase 12, §35 implementation skeleton, §39 pipeline DAG.
- `.planning/REQUIREMENTS.md` — STORE-01..06, DAG-01..04 verbatim.
- `.planning/milestones/v1.10-ROADMAP.md` — Phase 57 goal/success criteria/dependency map; pitfall mitigation cross-check (C5/m1/m5); must-not-regress invariants.
- `.planning/research/SUMMARY.md` — v1.10 research synthesis (Layer 1.5 placement, DuckDB CGO posture, SUMMARY's "Gaps to Address During Planning" lock recommendations).
- `.planning/research/PITFALLS.md` — C5 (cross-process lock), C6 (DuckDB JSON growth — phase 63 concern but flagged here), m1 (pipeline DAG half-migration), m5 (config drift).
- `internal/repomap/extractor_nocgo.go`, `internal/kernel/edit/treesitter_nocgo.go`, `internal/treesitter/available_{cgo,nocgo}.go` — the Phase 51.1 stub-pattern source of truth.
- `internal/daemon/daemon.go` (lines 1-421) — current bootstrap step order; CGO=0 refusal hook at lines 193-203.
- `internal/config/loader.go`, `internal/config/defaults.go`, `internal/config/loader_test.go` — koanf 4-layer precedence; existing per-feature defaults assertion style.
- `internal/errors/kinds.go` — `serr.Unsupported` Kind already exists.
- `internal/kernel/health/tools.go`, `internal/kernel/lspool/health.go` — `get_health` integration seam (Phase 65 concern, not Phase 57).
- `.goreleaser.yaml` — current CGO=0 release matrix verified.
- [duckdb/duckdb-go README (v2.10502.0)](https://github.com/duckdb/duckdb-go) — driver import path, CGO requirement, Open examples, version.
- [duckdb/duckdb-go-bindings README](https://github.com/duckdb/duckdb-go-bindings) — prebuilt platform list (no windows/arm64).
- [`golang.org/x/tools/go/analysis/singlechecker`](https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker) — vet-tool harness signature + boilerplate.

### Secondary (MEDIUM confidence)

- DuckDB issues [#13017](https://github.com/duckdb/duckdb/issues/13017), [#17158](https://github.com/duckdb/duckdb/issues/17158), [discussion #12296](https://github.com/duckdb/duckdb/discussions/12296) — lock-collision message shapes (used to inform Pitfall 1 classification).
- [marcboeker/go-duckdb archive note](https://github.com/marcboeker/go-duckdb) — confirms repo is read-only since 2025-10-20.

### Tertiary (LOW confidence — flagged for confirmation)

- DuckDB corruption-error message shapes (A1) — based on issue search, not on source-code grep of `duckdb-go/v2` driver. Confirm during Wave 0 by writing failing-test fixtures and capturing actual error strings.

## Metadata

**Confidence breakdown:**

| Area | Level | Reason |
|------|-------|--------|
| DuckDB driver path + CGO posture | HIGH | Verified via `WebFetch` against duckdb-go README; bindings list verbatim. |
| Daemon bootstrap insertion point | HIGH | Existing `daemon.go` step 6a (CGO=0 refusal) is the natural anchor; pattern verified in source. |
| CGO=0 stub pattern | HIGH | Verbatim mirror of v1.9 Phase 51.1 (`extractor_nocgo.go`, `treesitter_nocgo.go`, `available_{cgo,nocgo}.go`). |
| Config 4-layer precedence | HIGH | Existing `loader.go` flow + per-feature defaults test pattern verified. |
| `go vet -vettool` mechanics | HIGH | Standard `singlechecker` template from go.dev pkg docs. |
| Phasegraph library shape | HIGH | SPEC §39 has signatures + algorithm. |
| Schema-version representation | MEDIUM | SPEC leaves it open; recommendation justified but not pre-committed. |
| Corruption-vs-lock error classification | MEDIUM | Based on issue-tracker shapes; needs Wave 0 empirical validation. |
| Existence of "every-key-has-default" coverage test | HIGH (negative) | Confirmed absent via direct grep of `loader_test.go`; project precedent is per-feature defaults assertions. |
| windows/arm64 CGO=1 release viability | LOW (out of Phase 57 scope) | Verified bindings have no prebuilt; downstream impact is for v1.10 release planning, not this phase. |

**Research date:** 2026-05-03
**Valid until:** 2026-06-03 (30 days for stable findings; corruption-error classification may need refresh if duckdb-go releases a typed error API in the interim)
