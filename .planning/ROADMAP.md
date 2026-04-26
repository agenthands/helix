# Roadmap: Serena

## Milestones

- [x] **v1.0 MVP** -- Phases 1-5 (shipped 2026-04-08)
- [x] **v1.1 Integration Testing** -- Phases 6-8 (shipped 2026-04-09)
- [x] **v1.2 Performance & Production Hardening** -- Phases 9-15 (shipped 2026-04-10)
- [x] **v1.3 Documentation Catchup** -- Phases 16-17 (shipped 2026-04-11)
- [x] **v1.4 Integration Testing v2** -- Phases 18-21 (shipped 2026-04-14)
- [x] **v1.5 Typed Errors & Hardening** -- Phases 22-24 (shipped 2026-04-15)
- [x] **v1.6 Context Intelligence & Resilient Editing** -- Phases 25-33 (shipped 2026-04-20)
- [x] **v1.7 Developer Experience & Auto-Setup** -- Phases 34-38 (shipped 2026-04-22)
- [x] **v1.8 Documentation Overhaul** -- Phases 39-45 (shipped 2026-04-24)
- [ ] **v1.9 Polish & Infra** -- Phases 46-55 (in flight, kicked off 2026-04-24)

## Phases

### v1.9 Polish & Infra (Phases 46-55)

- [x] **Phase 46: bug-repomap-lua-fixture** -- Fix `get_repo_map` returning Lua testdata fixture instead of Go sources on Serena's own workspace (completed 2026-04-24)
- [x] **Phase 47: bug-rust-analyzer-rename** -- Make `rename_symbol` succeed on Rust symbols in temp workspaces (or ship a documented tool-level workaround) (completed 2026-04-24)
- [ ] **Phase 48: bug-jdtls-warm-cache** -- Reuse a warm jdtls workspace across test runs so Java integration tests pass in default `go test ./...`
- [x] **Phase 49: bug-grammar-registry-consolidation** -- Collapse 3 redundant `GrammarRegistry` instances into one canonical registry at daemon bootstrap (completed 2026-04-25)
- [x] **Phase 50: toolchain-go1.25-gopls-ci** -- Make CI green on ubuntu-latest with Go 1.25 and retire the CI benchmark gate (completed 2026-04-25)
- [x] **Phase 51: packaging-goreleaser** -- Multi-arch signed release pipeline via goreleaser as the foundation for downstream channels (completed 2026-04-26)
- [ ] **Phase 53: obs-metrics-gaps** -- Close v1.2 metrics gaps (cache hit-rate, repomap latency, session lifecycle, edit outcomes)
- [ ] **Phase 54: obs-dashboards-runbooks** -- Ship Grafana dashboards in `deploy/grafana/` and operational runbooks in `docs/runbooks/`
- [ ] **Phase 55: obs-trace-coverage-audit** -- Audit and close trace coverage gaps across MCP tool handlers and outbound LS calls

## Phase Details

### Phase 46: bug-repomap-lua-fixture
**Goal**: `get_repo_map` on Serena's own workspace returns a ranked view of `internal/` Go sources rather than the single deep Lua testdata fixture.
**Depends on**: Nothing
**Requirements**: BUG-01
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. Calling `get_repo_map` on `/Users/Janis_Vizulis/go/src/github.com/postfix/serena` surfaces top-ranked Go symbols from `internal/` (not `legacy/test/resources/repos/lua/...`).
  2. A regression test in `internal/repomap/` (or `test/oracle/`) asserts that on a polyglot workspace with Go + Lua testdata, the ranked output contains at least one `internal/` Go symbol and is not dominated by testdata fixtures.
  3. Root cause is documented in the phase review (PageRank starvation vs. extractor failure vs. workspace root vs. elide step).
  4. `go test ./...` and `go vet ./...` pass.
**Plans**: 3 plans
Plans:
- [x] 46-01-PLAN.md — Create synthetic polyglot fixture + failing reproduction unit tests (TDD red)
- [x] 46-02-PLAN.md — Apply F1-B ambiguity-weighted edges in graph.go BuildGraph (TDD green)
- [x] 46-03-PLAN.md — Oracle smoke test + phase RCA + orphan directory cleanup

### Phase 47: bug-rust-analyzer-rename
**Goal**: Users can rename Rust symbols via `rename_symbol` in temp workspaces, either by fixing the upstream quirk at the tool layer or by shipping a documented, deterministic workaround.
**Depends on**: Nothing
**Requirements**: BUG-02
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. The Rust integration test case for `rename_symbol` is green in default `go test ./...` (no `-short=false`, no skip).
  2. If a tool-level workaround is shipped instead of a true fix, it is documented in `USAGE.md` Troubleshooting and in a QuirkAdapter for `rust-analyzer`.
  3. Hover / references / search behavior on the same Rust symbol is unchanged (no regression).
  4. A trace of the successful path is recorded in the phase review (what was the "No references found at position" trigger, and what resolved it).
**Plans**: 3 plans
  - [x] 47-01-PLAN.md — RCA + readiness signal (capture wire trace, wire experimental/serverStatus into RustAnalyzerAdapter)
  - [x] 47-02-PLAN.md — QuirkAdapter RenameOverride + dispatcher + strategy metric
  - [x] 47-03-PLAN.md — Unskip rust rename integration test + USAGE.md troubleshooting + D-05 doc comments

### Phase 48: bug-jdtls-warm-cache
**Goal**: Java integration tests run as part of the default `go test ./...` suite because jdtls reuses a warm workspace across runs.
**Depends on**: Nothing
**Requirements**: BUG-03
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. `go test ./...` (no flags) runs the Java integration suite to completion without timing out.
  2. The warm-workspace strategy is isolated at the Serena/test layer — no upstream jdtls tuning is required (that is explicitly deferred to BUG-DEFER-01).
  3. A second consecutive `go test` run reuses the warm jdtls workspace measurably faster than the first (recorded in the phase review).
  4. CI wall-clock for the Java suite is documented before/after.
**Plans**: 5 plans
Plans:
- [ ] 48-01-PLAN.md — jdtlscache helper package (pure-stdlib hash + ResolveDataDir)
- [ ] 48-02-PLAN.md — JdtlsAdapter env-var override (SERENA_TEST_JDTLS_DATA_DIR)
- [ ] 48-03-PLAN.md — Drop build tags + wire Options.JdtlsDataDir + remove Short() skip
- [ ] 48-04-PLAN.md — Makefile targets clean-jdtls-cache and bench-jdtls-warm
- [ ] 48-05-PLAN.md — go-test.yml CI workflow + USAGE.md doc section

### Phase 49: bug-grammar-registry-consolidation
**Goal**: A single canonical `GrammarRegistry` is constructed at daemon bootstrap and shared by all consumers; the two redundant instances are deleted.
**Depends on**: Nothing
**Requirements**: BUG-04
**Sizing**: S
**Success Criteria** (what must be TRUE):
  1. `grep` / code search finds exactly one `NewGrammarRegistry(...)` (or equivalent constructor) call site in production code.
  2. Repomap, tagcache, and grammar consumers all receive the registry via dependency injection from the daemon.
  3. All existing tree-sitter-backed tests (repomap, tagcache, symbol editing for supported langs) remain green.
  4. `go test ./...` and `go vet ./...` pass.
**Plans**: 1 plan
Plans:
- [x] 49-01-PLAN.md — Inject canonical GrammarRegistry from daemon bootstrap into RepoMapSkill via SetRegistry; delete redundant FallbackDeps.Registry instantiation; verify D-10 grep + go vet + go test

### Phase 50: toolchain-go1.25-gopls-ci
**Goal**: Serena builds and tests green on `ubuntu-latest` with Go 1.25 (TOOL-01 via `go-test.yml`); the CI benchmark gate is retired (deletion of `bench.yml` + `capture-baseline.yml`); the local-first benchmark workflow and gopls v0.21.1 pin are documented in `CONTRIBUTING.md`; the benchmarks tech-debt sentence in `PROJECT.md` is rewritten to reflect the post-pivot state; TOOL-02 is cancelled.
**Depends on**: Nothing (independent of bug phases)
**Requirements**: TOOL-01, TOOL-02 (TOOL-02 cancelled by this phase per CONTEXT.md D-A10)
**Sizing**: S (post-pivot — was L pre-pivot)
**Success Criteria** (what must be TRUE) — post-pivot, supersedes the original four bullets and mirrors 50-CONTEXT.md D-A13:
  1. `go-test.yml` on the verification PR is green on `ubuntu-latest` with Go 1.25 (TOOL-01).
  2. `bench.yml` and `capture-baseline.yml` are absent from `.github/workflows/`.
  3. `CONTRIBUTING.md` has the new `## Benchmarks` and `## gopls pin` sections.
  4. `PROJECT.md` "Known tech debt" first sentence rewritten per D-A12.
  5. `REQUIREMENTS.md` shows TOOL-02 cancelled per D-A10.
  6. `test/bench/baselines/README.md` rewritten per D-A5.
**Plans**: 2 plans

Plans:
- [x] 50-01-PLAN.md — Delete `bench.yml` + `capture-baseline.yml`; rewrite `test/bench/baselines/README.md` for local-first workflow (D-A2, D-A5)
- [x] 50-02-PLAN.md — Add CONTRIBUTING.md `## Benchmarks` + `## gopls pin` (D-A7, D-A8); rewrite PROJECT.md tech-debt sentence (D-A12); mark TOOL-02 cancelled in REQUIREMENTS.md (D-A10); optional `bench-capture` / `bench-compare` Makefile targets (D-A9)

### Phase 51: packaging-goreleaser
**Goal**: GitHub Releases publish reproducible multi-arch signed binaries for darwin/linux/windows × amd64/arm64 via a goreleaser pipeline.
**Depends on**: Phase 50 (needs a green CI on ubuntu-latest to host the release job)
**Requirements**: PKG-01
**Sizing**: L
**Success Criteria** (what must be TRUE):
  1. Tagging a release (e.g. `v1.9.0-rc1`) triggers a goreleaser CI workflow that uploads 6 platform/arch binaries to GitHub Releases.
  2. Each binary ships with a SHA-256 checksum file and a cryptographic signature (cosign or minisign) — documented in `INSTALL.md`.
  3. A user following `INSTALL.md` can verify a downloaded binary's signature and checksum in one terminal session.
  4. The pipeline is reproducible — a second dry-run against the same tag produces byte-identical archives (modulo signatures).
**Plans**: 2 plans

Plans:
- [x] 51-01-PLAN.md — Add .goreleaser.yml + release.yml + delete publish.yml + internal/cli version scaffold
- [x] 51-02-PLAN.md — Amend INSTALL.md (download + verify) + add RELEASING.md + README.md pointer + Makefile ldflags

### Phase 53: obs-metrics-gaps
**Goal**: Operators can observe cache hit-rate, RepoMap extraction latency, session lifecycle, and edit-tool outcomes via Prometheus metrics with bounded labels.
**Depends on**: Nothing (builds on existing Prom infra from v1.2)
**Requirements**: OBS-03
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. `/metrics` exposes new series: `serena_lspool_cache_total{language,result,scope}`, `serena_repomap_cache_total{language,result}`, `serena_repomap_extract_duration_seconds` (histogram by language), `serena_session_lifecycle_total` (counter by language + phase), `serena_edit_outcome_total` (counter by tool + outcome). Hit-rate is computed in PromQL via `rate({result="hit"}[5m]) / rate(...[5m])` per CONTEXT.md D-01.
  2. All new labels are bounded (no unbounded cardinality) — a cardinality test asserts max series per metric.
  3. `USAGE.md` Observability section documents each new metric with its labels and semantics.
  4. Noop-default invariant preserved — metrics are zero-alloc when observability is disabled.
**Plans:** 3 plans

Plans:
- [ ] 53-01-PLAN.md — Extend *obs.Metrics with five new vectors + helpers + cardinality / zero-alloc tests
- [ ] 53-02-PLAN.md — Per-package sinks + emission at lspool/repomap/edit/kernel call sites + fuzzy.ErrAmbiguous sentinel
- [ ] 53-03-PLAN.md — Daemon wiring + four var _ assertions + USAGE.md Observability docs + ROADMAP reconciliation

### Phase 54: obs-dashboards-runbooks
**Goal**: Operators can import ready-made Grafana dashboards and follow written runbooks for the four most common failure modes.
**Depends on**: Phase 53 (dashboards consume the metrics landed there)
**Requirements**: OBS-01, OBS-02
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. `deploy/grafana/` contains at least two JSON dashboards covering RED metrics, lspool worker health, and workspace activity; they import cleanly into Grafana 10+.
  2. `docs/runbooks/` contains four runbooks: `ErrCircuitOpen.md`, `deadline-timeouts.md`, `ls-crash-restart.md`, `memory-pressure-eviction.md` — each with symptoms, triage steps, PromQL queries, and remediation.
  3. `USAGE.md` Observability section links to `deploy/grafana/` with a screenshot of the primary dashboard.
  4. Every PromQL expression in the dashboards and runbooks references a metric that actually exists in the registered Prom registry (validated by test).
**Plans**: TBD

### Phase 55: obs-trace-coverage-audit
**Goal**: Every MCP tool handler and every outbound LS call produces a span; sampling configuration is documented; trace attributes pass a hygiene review.
**Depends on**: Nothing (independent of metrics work, audits the tracing pipeline from v1.2)
**Requirements**: OBS-04
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. An automated audit (test or script) confirms every registered MCP tool emits a span on invocation and every outbound LS JSON-RPC call is wrapped in a child span.
  2. A review artifact (`.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md`) lists every span attribute and certifies: no PII, no unbounded cardinality (paths, IDs are hashed/bucketed where appropriate).
  3. `USAGE.md` Observability section documents sampling configuration (ratio, head vs. tail) and how to adjust it via `ObservabilityConfig`.
  4. A smoke trace captured against a live OTLP collector shows the full request path from MCP handler → LS call with no orphan spans.
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-5 | v1.0 | 20/20 | Complete | 2026-04-08 |
| 6-8 | v1.1 | 11/11 | Complete | 2026-04-09 |
| 9-15 | v1.2 | 25/25 | Complete | 2026-04-10 |
| 16-17 | v1.3 | 4/4 | Complete | 2026-04-11 |
| 18-21 | v1.4 | 11/11 | Complete | 2026-04-14 |
| 22-24 | v1.5 | 12/12 | Complete | 2026-04-15 |
| 25-33 | v1.6 | 22/22 | Complete | 2026-04-20 |
| 34-38 | v1.7 | 11/11 | Complete | 2026-04-22 |
| 39-45 | v1.8 | 19/19 | Complete | 2026-04-24 |
| 46 | v1.9 | 3/3 | Complete   | 2026-04-24 |
| 47 | v1.9 | 3/3 | Complete   | 2026-04-24 |
| 48 | v1.9 | 0/? | Not started | - |
| 49 | v1.9 | 1/1 | Complete   | 2026-04-25 |
| 50 | v1.9 | 2/2 | Complete   | 2026-04-25 |
| 51 | v1.9 | 2/2 | Complete   | 2026-04-26 |
| 53 | v1.9 | 0/3 | In progress | - |
| 54 | v1.9 | 0/? | Not started | - |
| 55 | v1.9 | 0/? | Not started | - |
| 56 | v1.9 | 4/4 | Complete   | 2026-04-25 |

### Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness

**Goal:** Wire jsonrpc.Conn.OnNotification in Worker.Start so QuirkAdapter notification handlers actually run in production, and ship a deterministic JdtlsAdapter.WaitUntilJavaReady(ctx) gate so Java integration tests pass in default `go test ./...`.
**Requirements**: LSDISP-01, LSDISP-02, LSDISP-03, LSDISP-04, LSDISP-04b, JDTLS-RDY-01a, JDTLS-RDY-01b, JDTLS-RDY-01c, JDTLS-RDY-02, LSDISP-REG-01
**Depends on:** Phase 55
**Plans:** 2/2 plans complete

Plans:
- [x] 56-01-PLAN.md — Restructure ProcessHandle.Start lifecycle + add StartListen(ctx) + process_test.go (D-02, D-03)
- [x] 56-02-PLAN.md — Wire dispatcher + regression assertion in Worker.Start + worker_test.go + extend codec_test.go (D-01, D-04, D-05, D-11, D-12)
- [x] 56-03-PLAN.md — Implement JdtlsAdapter.NotificationHandlers + WaitUntilJavaReady + extend quirks_test.go (D-06, D-07, D-08, D-09, D-13)
- [x] 56-04-PLAN.md — Java integration gate + rust-analyzer integration regression + final go vet/go test gate (D-10, D-14, D-15)

## Backlog

_No items in backlog._
