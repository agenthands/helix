# Phase 57: Semantic Store Foundation + Pipeline DAG Library - Context

**Gathered:** 2026-05-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 57 lays four orthogonal foundations for v1.10 in a single phase, with **no consumer-facing surface** beyond the daemon refusing to start cleanly under known-bad states:

1. **`internal/semantic/store/`** — DuckDB embedded fact store opened at daemon bootstrap (new step 6b) when `semantic_index.enabled=true`. Sole owner of the `duckdb-go` import. Three-tier open: existing+clean → quarantine+rebuild → hard fail. Schema versioning stamped per snapshot row. Effective-read API surface (`QueryEffective{Files,Symbols,References,Edges}`) for files, symbols, references, edges — Phase 59-60 populate the data the API reads.
2. **`internal/phasegraph/`** — stdlib-only pipeline-DAG library (`PhaseSpec`, `PhaseGraph`, `ValidatePhaseGraph`, `RunPhaseGraph`). Phase 60/67 consume it; daemon bootstrap stays imperative with a `// TODO(v1.11)` migration marker.
3. **`semantic_index.*` config keys** — every key in SPEC §25 threaded through `internal/config/defaults.go` + `SerenaConfig` struct fields, resolving via existing 4-layer koanf precedence (CLI > project `.helix/project.yml` > user `~/.helix/helix_config.yml` > profile defaults).
4. **`cmd/vet-noduckdb/`** — `singlechecker`-based analyzer wired into `make vet` via `-vettool` that fails the build if any package outside `internal/semantic/store/` imports `duckdb-go`.

**Out-of-scope (deferred to later v1.10 phases):** tree-sitter extraction (P59), live overlay watcher (P60), LSP enrichment worker (P61), graph engine + ranking + type resolution (P62), compaction + retention (P63), new MCP tools (P64), `get_health`/`get_repo_map` strangler-fig integration (P65), `GuardrailMiddleware` (P66), eval harness (P67), bootstrap migration to `phasegraph.RunPhaseGraph(BootstrapPhases)` (v1.11).

</domain>

<decisions>
## Implementation Decisions

### Schema Versioning (STORE-03)

- **D-01: Integer monotone version.** `semantic_schema_version` table holds a single `INTEGER NOT NULL` column; every snapshot row is stamped with `schema_version INTEGER NOT NULL`. Phase 57 ships **schema_version = 1**.
- **D-02: Explicit migrations registry.** A `migrations.go` slice declares each version transition with kind: `Migration{From, To, Kind}` where `Kind ∈ {InPlace, Reindex}`. The kind is decoded by the registry, not by the integer itself — this keeps the version field cheap to compare (`stored < current → look up the kind` for each step) and keeps the forward-vs-backward decision auditable in source.
- **D-03: Unknown forward version → quarantine + rebuild.** If `stored_version` exceeds the highest version this binary knows about, treat it as a forward-incompatible state, quarantine via the standard `.corrupt.<unix-ts>` rename, and rebuild fresh. (The user has rolled back to an older binary against a newer DB — by SPEC §29.1 we never refuse to start.)

### Plan Structure (P01–P04)

- **D-04: Four parallel-ready plans.** The phase decomposes into four orthogonal tracks landing as four plans:
  - **P01 — `internal/phasegraph/` library.** Stdlib only. `phase.go`, `dag.go`, `validate.go`, `run.go`, `shutdown.go`, `dot.go`, plus `pipelines/{semantic,live,eval}.go` declaring the typed phase ID constants and `Requires`/`Provides` for DAG-02. Test coverage for cycle / missing-dep / duplicate-id (DAG-03). **Independent of P02–P04.**
  - **P02 — `internal/semantic/{config,store,types}` skeleton.** Adds `github.com/duckdb/duckdb-go/v2@v2.10502.0` to `go.mod` (per D-12). Lays down `internal/semantic/config.go` (typed mirror of koanf `semantic_index.*`), `internal/semantic/types.go` (SnapshotID, FileID, SymbolID, Freshness), and the full `store/` package: `duckdb.go` (CGO=1 open/close/quarantine/rebuild), `duckdb_nocgo.go` (CGO=0 stub returning `serr.ErrUnsupported`), `migrations.go`, `snapshot.go`, `overlay.go` skeleton, `effective.go` (snapshot ⊕ overlay − tombstones), `store_test.go`. Wires `OpenSemanticStore` into daemon bootstrap at **step 6b** (between the existing CGO=0 refusal at step 6a and the diagnostic store at step 6). **Also lands a stub `SemanticIndex semantic.Config` field on `SerenaConfig`** so the daemon step 6b reference (`cfg.SemanticIndex.Enabled`) compiles at end of wave 1; P03 then populates that field's defaults and koanf bindings.
  - **P03 — `semantic_index.*` config keys.** Adds every key from SPEC §25 to `internal/config/defaults.go`, attaches the koanf binding to the existing-but-empty `SerenaConfig.SemanticIndex` field landed by P02, adds `TestLoad_SemanticIndexDefaults` and `TestLoad_SemanticIndexPrecedence` in `internal/config/loader_test.go`. **Depends on P02** for the typed `semantic.Config` mirror and the `SerenaConfig.SemanticIndex` field that consumes the resolved values.
  - **P04 — `cmd/vet-noduckdb/` analyzer.** `cmd/vet-noduckdb/main.go` invoking `singlechecker.Main(noduckdb.Analyzer)`; analyzer body in `internal/lint/noduckdb/{analyzer.go,analyzer_test.go}` using `analysistest` with a fixture that has a non-store file importing `duckdb-go`. `Makefile` adds `make vet` building the analyzer first then passing it via `-vettool=`. **Depends on P02** so the legitimate `internal/semantic/store/` import path exists for the analyzer to allowlist. Analyzer's `forbiddenImport` constant MUST match the exact module path locked in D-12.
- **D-05: Plan dependency graph.** `P01 → independent`. `P03 → P02`. `P04 → P02`. Executor can run P01 in wave 1 alongside P02; P03 and P04 run after P02 lands.

### Quarantine Observability (STORE-01, must-not-regress)

- **D-06: Structured slog event AND new metric.** On every quarantine, emit `slog.Warn("semantic store quarantined", "workspace", ..., "original", ..., "quarantine", ..., "reason", ..., "old_version", ..., "new_version", ...)` and increment a new Prometheus counter:

  ```
  helix_semantic_store_quarantine_total{workspace_label, reason}
  ```

- **D-07: Bounded `reason` enum.** The `reason` label is allowlisted to **exactly four values**:
  - `corrupt_file` — DuckDB IO/parse error not classifiable as a stale lock
  - `schema_forward_incompat` — stored version exceeds current binary's knowledge (D-03)
  - `schema_unreadable` — `semantic_schema_version` table missing or garbled
  - `unknown` — catch-all for any future failure mode (deliberately reserved)

  These values are added to the v1.2/v1.9 cardinality allowlist and PromQL validator. Counter and label set must round-trip through the existing `internal/obs/` helpers — no bespoke registration path.
- **D-08: Counter integration is in P02.** The metric ships with the store package, not as a separate plan. Phase 65 will surface it through `get_health`; Phase 57 only needs the metric to exist and increment correctly.

### Config Coverage Test (STORE-05)

- **D-09: Per-feature defaults test.** Add `TestLoad_SemanticIndexDefaults` to `internal/config/loader_test.go` mirroring the `TestLoad_ObservabilityDefaults` style — assert every `semantic_index.*` default is present and matches SPEC §25 verbatim.
- **D-10: Per-feature precedence test.** Add `TestLoad_SemanticIndexPrecedence` covering 3–4 representative keys (one per layer per key) confirming CLI > project > user > profile resolution. **No generic key-coverage matrix in v1.10** — that is its own future phase if we want it; do not slip it into P57.
- **D-11: Both tests live in P03.** P03 is the natural home — it is the plan that adds the keys.

### DuckDB Module Path Lock (STORE-04, STORE-06)

- **D-12: Pin the duckdb-go module path verbatim.** The `go.mod` import path is `github.com/duckdb/duckdb-go/v2` at version `v2.10502.0` (DuckDB 1.5.2, released 2026-04-14). This is the canonical DuckDB Foundation Go binding repo per RESEARCH.md (the historical `marcboeker/go-duckdb` repository was archived 2025-10-20 and MUST NOT be used). The exact string `github.com/duckdb/duckdb-go/v2` MUST appear verbatim in:
  - **P02 Task 1** `<action>` — `go get github.com/duckdb/duckdb-go/v2@v2.10502.0` and the resulting `go.mod` require directive.
  - **P02 Task 2b** `internal/semantic/store/duckdb.go` — the `import "github.com/duckdb/duckdb-go/v2"` driver registration.
  - **P04 Task 2** `internal/lint/noduckdb/analyzer.go` — the `forbiddenImport = "github.com/duckdb/duckdb-go"` constant (prefix-match form, omits the `/v2` suffix so any future version bump still matches).
  - **P04 Task 1** `internal/lint/noduckdb/testdata/src/badpkg/imports.go` and the `goodpkg` fixture — `import _ "github.com/duckdb/duckdb-go/v2"` verbatim.

  Locking this here removes the analyzer-string-mismatch risk for STORE-06: P02 and P04 can be authored independently because both reference D-12 rather than a stringly-typed handoff via SUMMARY.md. Any future bump (e.g., to v3) requires explicit revision of D-12.

### Claude's Discretion (research-locked, no user input needed)

- **CGO=0 stub pattern** mirrors `internal/repomap/extractor_nocgo.go` (Phase 51.1) verbatim for `internal/semantic/store/duckdb_nocgo.go`. Stub methods return `serr.ErrUnsupported` as defense-in-depth; daemon already refuses CGO=0 at step 6a so the stub is only exercised in unit tests built with CGO=0 explicitly.
- **`internal/phasegraph/` is non-generic.** `PhaseOutput` is `interface{}`; `PhaseDeps` is `map[PhaseID]any`. Consumers in P60/P67 type-assert at the read site. No Go generics in v1.10 — keeps the API simple and the dependency-resolution map composable across heterogenous phase outputs.
- **Daemon `OpenSemanticStore` lives at step 6b**, immediately after the CGO=0 refusal hook (step 6a, current line 193) and before the diagnostic store / body extractor (step 6, current line 205). Fail-fast core subsystem with the same "compile out under CGO=0" property as kernel / language registry.
- **Vet-tool is standalone** (`cmd/vet-noduckdb/main.go`); no existing custom analyzer in the repo to co-locate with.
- **`// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases)`** marker placement: top of `newDaemon` in `internal/daemon/daemon.go`, where the imperative step list begins (current ~line 160).
- **Snapshot retention / VACUUM are NOT in P57.** Compaction (Phase 63) owns those; P57 only ships the table shape so future work has somewhere to write.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements (load-bearing)

- `.planning/REQUIREMENTS.md` §STORE — STORE-01 through STORE-06 phrasing is the contract.
- `.planning/REQUIREMENTS.md` §DAG — DAG-01 through DAG-04 phrasing is the contract.
- `.planning/milestones/v1.10-ROADMAP.md` Phase 57 block — Goal, Depends on, Requirements, 5 Success Criteria.

### Specification (source of truth for shapes and key names)

- `SPEC-DRAFT.md` §4 — DuckDB ADR-001 rationale (why DuckDB over SQLite/sqlc).
- `SPEC-DRAFT.md` §5 — Layer 1.5 placement and architectural diagram.
- `SPEC-DRAFT.md` §6 — `internal/semantic/` directory layout.
- `SPEC-DRAFT.md` §7 — `SnapshotID`, `FileID`, `SymbolID`, `Freshness` type definitions.
- `SPEC-DRAFT.md` §8 — DuckDB schema (semantic_snapshots, semantic_schema_version, files, symbols, references, edges, overlay, tombstones).
- `SPEC-DRAFT.md` §9.1–§9.2 — schema-version table shape and per-snapshot stamping rule.
- `SPEC-DRAFT.md` §10 — `QueryEffective*` pseudocode (snapshot ⊕ overlay − tombstones, exact contract).
- `SPEC-DRAFT.md` §25 — every `semantic_index.*` config key with default. **All keys must round-trip.**
- `SPEC-DRAFT.md` §29.1 — corruption-recovery flow (detect → quarantine → log → allow rebuild).
- `SPEC-DRAFT.md` §32 Phase 0/12 — bootstrap migration deferral context.
- `SPEC-DRAFT.md` §39.2–§39.3 — `internal/phasegraph/` API signatures and Kahn pseudocode.
- `SPEC-DRAFT.md` §39.5–§39.7 — phase ID lists for semantic-index, live-update, eval pipelines.
- `SPEC-DRAFT.md` §39.9 — DAG validation error contract and `.helix/debug/phasegraph-*.dot` convention.

### Research input (full implementation guidance)

- `.planning/phases/57-semantic-store-foundation-pipeline-dag-library/57-RESEARCH.md` — full technical research; covers duckdb-go-bindings posture, Phase 51.1 stub pattern, three-tier open ergonomics, vet-tool mechanics, alternatives considered.

### Pattern templates (must mirror)

- `internal/repomap/extractor_nocgo.go` — CGO=0 stub canonical template; `duckdb_nocgo.go` mirrors verbatim.
- `internal/kernel/edit/treesitter_nocgo.go` — second CGO=0 stub instance; cross-check shape.
- `internal/daemon/daemon.go` — bootstrap step ordering (step 6a CGO refusal at line 193; step 6 diagnostic store at line 205); `newDaemon` is the imperative orchestrator.
- `internal/config/loader_test.go` — `TestLoad_ObservabilityDefaults` is the per-feature-defaults template for `TestLoad_SemanticIndexDefaults`.
- `internal/errors/kinds.go:17` — `serr.Unsupported` already exists; reuse, don't redefine.
- `internal/obs/` — bounded-label metric registration helpers; new counter must use this path.

### Architectural invariants (must-not-regress)

- `.planning/milestones/v1.9-MILESTONE-AUDIT.md` — Phase 49 (single canonical `GrammarRegistry`), Phase 51.1 (CGO=0 single-binary policy), v1.2/v1.9 bounded-label cardinality rule.
- `CLAUDE.md` "Middleware Execution Order (LIFO)" section — Phase 57 does not modify middleware; cross-check that no plan accidentally reorders it.

### v1.10 strategic context

- `.planning/milestones/v1.10-ROADMAP.md` Phase 58 block — Phase 58 ships in parallel with Phase 57; release/distribution work is independent. **No release-pipeline files (`.goreleaser.yaml`, signing config) are touched by Phase 57.**

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`serr.Unsupported`** (`internal/errors/kinds.go:17`) — every CGO=0 stub method and disabled-path tool returns this kind with the existing remediation field. Already wired through the typed-error taxonomy from v1.5.
- **`internal/obs/` metrics helpers** — bounded-label counter registration; the new `helix_semantic_store_quarantine_total` plugs in the same way `helix_lspool_*` and `helix_repomap_*` counters do.
- **koanf 4-layer precedence** (`internal/config/`) — already loads CLI > project > user > profile; only the **default values** in `defaults.go` and the **struct field set** on `SerenaConfig` need to grow. No precedence machinery changes.
- **`singlechecker.Main(...)` from `golang.org/x/tools/go/analysis/`** — already in the transitive dependency tree; no new dep needed for the vet-tool. Standard idiom.
- **`database/sql` + `sql.OpenDB`** — duckdb-go hooks into `database/sql`; init callbacks for PRAGMAs and connection-pool sizing flow through the standard driver registration.

### Established Patterns

- **CGO=0 stub pattern (Phase 51.1).** Two source files in the same package, mutually exclusive via `//go:build cgo` and `//go:build !cgo`. Real implementation in CGO=1 file; empty stub in CGO=0 file returning `serr.ErrUnsupported`. Daemon refuses CGO=0 at boot (step 6a) so the stub is unreachable in production. Mirror exactly.
- **Three-tier resolution (v1.0 Phase 3).** `langregistry.NewInstaller` follows `PATH lookup > managed download > helpful error`. Mirror for `OpenSemanticStore`: existing+clean → quarantine+rebuild → hard fail.
- **Per-feature `TestLoad_*Defaults` (v1.2)** — every config-bearing subsystem gets its own per-feature defaults assertion test. Match the style; no reflection-based generic matrix.
- **Imperative bootstrap with comment-numbered steps** (`internal/daemon/daemon.go`). Steps are numbered in source comments (`// 1.`, `// 6a.`, `// 14b.`); inserting `// 6b. Open semantic fact store.` follows the existing convention. Git-blame continuity is explicitly preserved.

### Integration Points

- **Daemon bootstrap step 6b** is the one new bootstrap line. It receives the resolved `semantic.Config`, calls `store.Open(ctx, cfg)`, and stores the resulting `*semantic.Store` (or nil when disabled) on the daemon struct. Fail-fast on Tier-3 errors; structured-log on Tier-2 quarantines.
- **`SerenaConfig` struct fields** in `internal/config/` get a new `SemanticIndex semantic.Config` block (or equivalent — the typed mirror lives in `internal/semantic/config.go` and is referenced from the central config struct). The koanf binding decorates with `koanf:"semantic_index"`. **Per the revised plan structure (D-04 update):** the typed field is added in P02 (as a stub with zero values, so daemon step 6b compiles); P03 wires the koanf binding tag and populates defaults so the field actually carries SPEC §25 values at runtime.
- **`Makefile`** gains a `vet` target that builds `cmd/vet-noduckdb` to a temp path, then runs `go vet -vettool=<path> ./...`. CI vet job runs the same target.
- **Phase 60/67 consumers of `internal/phasegraph/`** declare typed phase ID constants (e.g., `phasegraph.PhaseStore`, `phasegraph.PhaseEnrich`) and pass `PhaseSpec`s with `Requires`/`Provides`/`Run`/`Validate`/`Shutdown`. Phase 57 ships the **shapes** (typed ID constants + Requires/Provides decls) for `pipelines/semantic.go`, `pipelines/live.go`, `pipelines/eval.go`; the `Run` bodies are empty/`return nil, nil` placeholders P60/P67 will fill.

</code_context>

<specifics>
## Specific Ideas

- **Schema 1 ships empty-but-correct.** P02's first migration step is "create all SPEC §8 tables and stamp `semantic_schema_version = 1`" — no data write paths yet (those are P59/P60). The test asserts the tables exist, the version row is `1`, and the effective-read API returns the empty result for every query.
- **`helix_semantic_store_quarantine_total` is one of two metrics introduced by Phase 57.** The other is whatever the planner decides for the open-success counter (e.g., `helix_semantic_store_open_total{outcome=opened|quarantined|created}`); both go through the same bounded-label allowlist.
- **`go vet` runs on every `make test`.** The vet-tool is a build gate, not a separate CI job — adding `make vet` as a prerequisite of `make test` (or composing both in `make ci`) keeps the boundary enforced from day 1.
- **`store_test.go` integration test seeds a corrupt `.duckdb`** (truncated header bytes) and confirms quarantine + rebuild + counter increment + slog event. Use Go's `t.TempDir()` for the workspace; do not pollute the project tree.
- **`internal/phasegraph/` test fixtures are minimal.** A handful of in-memory `PhaseSpec` slices covering: happy path (3 nodes, 2 edges), duplicate ID, missing dep, single-node cycle, multi-node cycle. `phasegraph_test.go` keeps fixture data inline — no `testdata/` directory.

</specifics>

<deferred>
## Deferred Ideas

- **Generic `TestConfig_AllKeysHaveDefaults` reflection-walk matrix** — discussed and explicitly rejected for v1.10 scope. Belongs in its own future phase if we decide we want it. P57 ships per-feature only.
- **CGO=1 release-channel split** — research finding: `duckdb-go-bindings` lacks windows-arm64 prebuilt static libs, so any future "ship a CGO=1 release" plan must drop windows-arm64 from the CGO=1 matrix or cross-compile DuckDB from source. **Belongs in v1.10 release planning (Phase 58 or later)**, not Phase 57. Captured here so it does not blindside the eventual CGO=1-release phase.
- **Bootstrap migration to `phasegraph.RunPhaseGraph(BootstrapPhases)`** — explicit `// TODO(v1.11)` per DAG-04. **v1.11 phase, not Phase 57.**
- **Cluster MCP tools, multi-projection PageRank, P1 retrieval companions, comment-fallback type resolution** — v1.10.x backlog (post-v1.10 dot-releases).
- **`get_health` semantic enrichment** — Phase 65 (Existing-Tool Integration / Strangler Fig). P57 emits the slog event and counter; P65 reads them.
- **Snapshot retention / VACUUM scheduling** — Phase 63 (Compaction & Retention). P57 lays the table shapes; P63 owns lifecycle.

</deferred>

---

*Phase: 57-semantic-store-foundation-pipeline-dag-library*
*Context gathered: 2026-05-03*
*Revised: 2026-05-03 — added D-12 (duckdb-go module path lock) per checker B-03; updated D-04 to reflect P02 lands stub `SerenaConfig.SemanticIndex` field for wave-1 build green*
