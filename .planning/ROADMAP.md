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
- [x] **Phase 48: bug-jdtls-warm-cache** -- Reuse a warm jdtls workspace across test runs so Java integration tests pass in default `go test ./...` (completed 2026-05-01)
- [x] **Phase 49: bug-grammar-registry-consolidation** -- Collapse 3 redundant `GrammarRegistry` instances into one canonical registry at daemon bootstrap (completed 2026-04-25)
- [x] **Phase 50: toolchain-go1.25-bench-local** -- Unblock Go 1.25 / gopls on `ubuntu-latest` for build/vet/test, and convert the benchmark harness to local-only (remove all CI bench plumbing) (completed 2026-04-28)
- [x] **Phase 51: packaging-goreleaser** -- Multi-arch signed release pipeline via goreleaser as the foundation for downstream channels (gaps_found 2026-04-29; gap-closure plans 51-03..51-06 added) (completed 2026-04-29)
- [x] **Phase 52: packaging-distribution-channels** -- Binary + product rename (`serena` → `helix`), in-binary self-upgrade (`helix update` / `helix upgrade`), embed-audit manifest (rescoped 2026-04-29 from original brew/scoop/native-Linux scope; PKG-02/03/04 deferred to PKG-DEFER-03/04/05) (completed 2026-04-30)
- [x] **Phase 53: obs-metrics-gaps** -- Close v1.2 metrics gaps (cache hit-rate, repomap latency, session lifecycle, edit outcomes) (completed 2026-04-30)
- [x] **Phase 54: obs-dashboards-runbooks** -- Ship Grafana dashboards in `deploy/grafana/` and operational runbooks in `docs/runbooks/` (completed 2026-05-01)
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
- [x] 48-01-PLAN.md — jdtlscache helper package (pure-stdlib hash + ResolveDataDir)
- [x] 48-02-PLAN.md — JdtlsAdapter env-var override (SERENA_TEST_JDTLS_DATA_DIR)
- [x] 48-03-PLAN.md — Drop build tags + wire Options.JdtlsDataDir + remove Short() skip
- [x] 48-04-PLAN.md — Makefile targets clean-jdtls-cache and bench-jdtls-warm
- [x] 48-05-PLAN.md — go-test.yml CI workflow + USAGE.md doc section

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

### Phase 50: toolchain-go1.25-bench-local
**Goal**: Serena builds/vets/tests green on `ubuntu-latest` with Go 1.25 + a compatible gopls, AND the benchmark harness is converted to a local-only flow with all hosted-CI bench plumbing removed.
**Depends on**: Nothing (independent of bug phases)
**Requirements**: TOOL-01, TOOL-02
**Sizing**: L
**Success Criteria** (what must be TRUE):
  1. A CI job on `ubuntu-latest` with Go 1.25 completes `go build ./...`, `go vet ./...`, and `go test ./...` green. Benchmarks are NOT run on CI.
  2. The gopls v0.17.1 linux/amd64 incompatibility is resolved with a documented strategy (upgrade, patch, or replacement) captured in `CONTRIBUTING.md`.
  3. `bench.yml` and `capture-baseline.yml` workflows are removed; `test/bench/baselines/v1.*-github-hosted.txt` files are removed or relocated to clearly mark them as historical-local artifacts (decided during planning).
  4. The bench harness runs locally via documented commands (e.g. `make bench`, `make bench-baseline`) and the local-only flow is documented in `CONTRIBUTING.md` and `test/bench/baselines/README.md`.
  5. The benchmarks tech-debt note in `PROJECT.md` is removed or rewritten to reflect the local-only stance.
**Plans**: 4 plans
Plans:
**Wave 1**
- [x] 50-01-PLAN.md — Hosted-CI bench plumbing teardown (delete bench.yml, capture-baseline.yml, benchgate package, v1.*-github-hosted baselines; final benchgate grep gate)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 50-02-PLAN.md — Local bench command surface (Makefile bench/bench-baseline targets, .gitignore local.txt entry, test/bench/baselines/README.md rewrite)
- [x] 50-03-PLAN.md — Documentation & tech-debt cleanup (CONTRIBUTING.md gopls subsection + Running Benchmarks rewrite + delete CI Gate section; USAGE.md gopls troubleshooting rewrite; PROJECT.md line 139 surgical edit)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 50-04-PLAN.md — CI green confirmation (local go vet + go test smoke; full grep matrix; checkpoint: go-test.yml green on ubuntu-latest Go 1.25.x)

Notes:
- Original Phase 50 design assumed a CI bench gate on ubuntu-latest; that design violates the project's local-only bench rule and was archived on 2026-04-28. See `.planning/phases/50-toolchain-go1.25-bench-local/_superseded/README.md` for the original artifacts.
- Phase 51 (packaging-goreleaser) "Depends on Phase 50" still holds — Phase 51 needs the green ubuntu-latest CI for build/vet/test (criterion 1), not the dropped bench gate.

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
**Plans**: 6 plans (51-01, 51-02 complete; 51-03..51-06 gap-closure)

Plans:
**Wave 1**
- [x] 51-01-pipeline-config-workflow-PLAN.md — `.goreleaser.yaml` (6-arch + minisign + reproducibility flags), `.github/workflows/release.yml` (tag-trigger + three-pass diff gate), `minisign.pub` placeholder, delete legacy `publish.yml`

**Wave 2** *(blocked on Wave 1 — references secret names + archive filenames + URLs from Plan 01)*
- [x] 51-02-docs-makefile-PLAN.md — INSTALL.md restructure (D-04 verify block, agenthands/helix URLs), README.md repo-identity fixes (lines 65, 71), Makefile `release-snapshot` target, CONTRIBUTING.md "Releasing" subsection

**Wave 3** *(gap closure from 51-VERIFICATION.md; all four plans run in parallel — no `files_modified` overlap)*
- [x] 51-03-cgo-optional-bindings-PLAN.md — DEF-51-01 / Concern A: //go:build cgo split for treesitter R + Swift bindings; CGO_ENABLED=0 builds support 21 languages without panicking; unblocks SC-1
- [x] 51-04-readme-identity-flags-PLAN.md — Concern D: README.md primary install path drops broken `go install postfix/serena`; HTTP-mode example reconciled with INSTALL.md (`--mode=http --http-addr=127.0.0.1:8080`); upstream attribution dedup'd (CR-01, WR-03, WR-04)
- [x] 51-05-checksums-signing-strict-PLAN.md — Concern B (build-config half) + IN-01: signs.artifacts changed from `archive` to `all` so checksums.txt is signed; INSTALL.md sha256sum step strict-grep instead of `--ignore-missing`; Makefile guards goreleaser missing (WR-02, WR-05, IN-01)
- [x] 51-06-release-yml-hardening-PLAN.md — Concern B (CI half) + Concern C + Concern E: PLACEHOLDER pre-flight; SHA-pinned third-party actions; minisign tarball SHA-256 verified before extraction; `/tmp/minisign.key` shred-on-always; uname -m guard; CONTRIBUTING.md reproducibility wording softened (CR-02, CR-03, CR-04, WR-01, WR-06, IN-02, IN-03, IN-04)

### Phase 51.1: cgo-treesitter-gate: gate internal/treesitter behind //go:build cgo with !cgo stub. Closes DEF-51-02 (resolution Path 2). Unblocks Phase 51 SC-1 (CGO_ENABLED=0 cross-compile so goreleaser produces 6 archives) and Phase 52. Acceptance: CGO=0 build of ./cmd/serena exits 0; CGO=1 byte-identical with all 23 treesitter languages still registered; goreleaser snapshot produces 6 archives; CGO=0 build smoke gated in CI. (INSERTED)

**Goal:** `internal/treesitter` is gated behind `//go:build cgo` with a `//go:build !cgo` stub set across `internal/treesitter`, `internal/repomap`, and `internal/kernel/edit`; the daemon refuses to start under `CGO_ENABLED=0` with a clear remediation message; `CGO_ENABLED=0 go build ./cmd/serena` exits 0; `CGO_ENABLED=1` builds remain byte-identical with all 23 grammars registered; `make release-snapshot` produces 6 archives; the existing release.yml reproducibility gate becomes the CGO=0 smoke gate (CONTRIBUTING.md expected-fail marker flipped); DEF-51-02 closes with Status: RESOLVED.
**Requirements**: DEF-51-02, SC-1-Phase51, AC-1, AC-2, AC-3, AC-4
**Depends on:** Phase 51
**Plans:** 1/1 plans complete

Plans:
- [x] 51.1-01-cgo-gate-PLAN.md — gate treesitter + repomap + kernel/edit behind //go:build cgo with !cgo stubs, daemon refusal hook, tagged tests, flip CONTRIBUTING.md marker, mark DEF-51-02 RESOLVED

### Phase 52: packaging-distribution-channels
**Goal**: Users can install Helix as a single self-contained signed binary (continued from Phase 51) and upgrade it in place via `helix upgrade`. The binary, env vars, config dirs, and MCP server registration name all flip from `serena` to `helix` as a hard-cut breaking change at v1.9. An embed-audit manifest documents what ships inside the binary versus what the binary downloads at runtime.
**Depends on**: Phase 51 (consumes goreleaser archives + minisign signing key + reproducibility CI gate)
**Requirements**: PKG-05, PKG-06, PKG-07 (PKG-02/03/04 deferred from v1.9 per phase rescope — see PKG-DEFER-03/04/05)
**Sizing**: L
**Success Criteria** (what must be TRUE):
  1. `cmd/helix/main.go` builds CGO=0 and produces a `helix` binary; `cmd/serena/` does not exist; the Go module path is `github.com/agenthands/helix`.
  2. `goreleaser --snapshot --clean` produces 6 `helix_v*` archives in `dist/` with no leftover `serena_v*` archives.
  3. `helix update` queries the GitHub Releases API and prints `current: vX / latest: vY` plus release-notes body without making any filesystem mutation.
  4. `helix upgrade` end-to-end: daemon-detect → permission probe → API fetch → semver compare (hard-refuse downgrade) → archive download → minisign verify → extract → atomic swap → `os.Exec` re-launch with same args minus `upgrade`.
  5. Tampered or wrong-key signatures are rejected with a single canonical error message; signature verification uses the build-time-synced embedded `minisign.pub`.
  6. `EMBED-AUDIT.md` exists in the phase directory classifying every runtime asset as embedded / external-by-design / gap; gap-flagged items are closed in-phase.
  7. INSTALL.md, CHANGELOG.md, README.md, USAGE.md, CONTRIBUTING.md, CLAUDE.md all use `helix` as the binary/product name; CHANGELOG v1.9 includes a Breaking Changes subsection enumerating the rename, env-var, config-dir, and MCP registration breaks.
  8. `go test ./...`, `go vet ./...`, `make verify-embed-pubkey`, `make release-snapshot` all green.
**Plans**: 6 plans
**Rescope rationale**: Original scope (PKG-02 Homebrew, PKG-03 Scoop, PKG-04 native Linux) abandoned during /gsd-discuss-phase 2026-04-29 in favor of self-contained-binary + in-binary self-upgrade. PKG-02/03/04 deferred to a future milestone (PKG-DEFER-03/04/05). Decision recorded in 52-CONTEXT.md.

Plans:
- [x] 52-01-PLAN.md — Wave 0 test scaffolding + Makefile embed-pubkey + CI gate
- [x] 52-02-PLAN.md — Wave 1 module path + cmd dir + goreleaser + version wiring
- [x] 52-03-PLAN.md — Wave 2 env vars + config dirs + MCP registration name flip
- [x] 52-04-PLAN.md — Wave 3 internal/upgrade/ package + cobra subcommands
- [x] 52-05-PLAN.md — Wave 3 EMBED-AUDIT.md manifest
- [x] 52-06-PLAN.md — Wave 3 docs + REQUIREMENTS/ROADMAP/CHANGELOG/INSTALL/README/USAGE/CONTRIBUTING/CLAUDE

### Phase 53: obs-metrics-gaps
**Goal**: Operators can observe cache hit-rate, RepoMap extraction latency, session lifecycle, and edit-tool outcomes via Prometheus metrics with bounded labels.
**Depends on**: Nothing (builds on existing Prom infra from v1.2)
**Requirements**: OBS-03
**Sizing**: M
**Success Criteria** (what must be TRUE):
  1. `/metrics` exposes new series: `helix_lspool_lookups_total{language, result}` (counter; `result` ∈ {hit, miss}), `helix_repomap_lookups_total{language, result}` (counter), `helix_repomap_extract_duration_seconds{language, extractor}` (histogram), `helix_session_lifecycle_total{phase, transport}` (counter), `helix_edit_outcome_total{tool_name, outcome, strategy}` (counter).
  2. All new labels are bounded (no unbounded cardinality) — a cardinality test asserts max series per metric.
  3. `USAGE.md` Observability section documents each new metric with its labels and semantics.
  4. Noop-default invariant preserved — metrics are zero-alloc when observability is disabled.
**Plans**: 6 plans

Plans:
- [x] 53-01-PLAN.md — Wave 1 obs vectors + helpers + cardinality lint (5 new families)
- [x] 53-02-PLAN.md — Wave 2 lspool MetricsSink extension + AcquireLease lookup emission
- [x] 53-03-PLAN.md — Wave 2 repomap MetricsSink + TagCache lookup + per-extractor latency observation
- [x] 53-04-PLAN.md — Wave 2 mcp.RecordEditOutcome + 7 edit/fileops handler instrumentation
- [x] 53-05-PLAN.md — Wave 2 forwarder stdio lifecycle + new http_session_middleware.go (Q-1 Option 2)
- [x] 53-06-PLAN.md — Wave 3 USAGE.md docs (5 metric rows + 2 PromQL examples + http best-effort caveat) + ROADMAP serena→helix correction

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
**Plans**: 5 plans

Plans:
- [x] 54-01-PLAN.md — Wave 0: add prometheus/prometheus@v3.11.0 dep + author internal/obs/dashboards_test.go validator (registry-driven, fail-closed) + drift-detection companion + .gitkeep stubs for new top-level dirs
- [x] 54-02-PLAN.md — Wave 1: deploy/grafana/helix-overview.json (RED + sessions + best-effort caveat text panel; 6–9 panels; ${DS_PROMETHEUS} + $language/$instance template vars) + remove dashboard env-var gate from validator
- [x] 54-03-PLAN.md — Wave 1: deploy/grafana/helix-engine.json (lspool + repomap + edits; verbatim Phase 53 D-01 hit-ratio + D-05/D-06 per-extractor p95 PromQL; 9 panels)
- [x] 54-04-PLAN.md — Wave 2: four runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction) with shared D-15 frontmatter + D-16 H2 order + D-17 ## Code references + remove runbook env-var gate from validator
- [x] 54-05-PLAN.md — Wave 3: capture docs/images/helix-overview-dashboard.png (manual checkpoint) + insert ### Grafana Dashboards and ### Runbooks H3 subsections into USAGE.md before ### Prometheus Metrics

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
**Plans**: 7 plans

Plans:
**Wave 1** (parallel — no files_modified overlap)
- [ ] 55-01-PLAN.md — Wrap jsonrpc.Conn.Call/Notify in lspool.lsp.{method} spans + thread tracer Pool→Worker→ProcessHandle (TDD)
- [ ] 55-02-PLAN.md — Wrap SerenaMCPServer.AddSkillTool handlers in kernel.tool.{name} spans (TDD)

**Wave 2** *(blocked on Wave 1)*
- [ ] 55-03-PLAN.md — Author internal/obs/trace_audit_test.go registry-driven coverage audit (5 tests + drift companion)

**Wave 3** *(parallel, blocked on Wave 2)*
- [ ] 55-04-PLAN.md — Author TRACE-AUDIT.md per-span attribute hygiene review (zero FAIL attributes)
- [ ] 55-05-PLAN.md — Insert ### Trace Sampling H3 in USAGE.md (after ### Enable Tracing per RESEARCH Pitfall 5)
- [ ] 55-06-PLAN.md — docs/runbooks/trace-smoke.md + Jaeger screenshot (manual checkpoint)

**Wave 4** *(blocked on Waves 1-3)*
- [ ] 55-07-PLAN.md — Final go vet + go test gate + invariant audit (D-01/D-07/D-11/D-17)

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
| 48 | v1.9 | 5/5 | Complete    | 2026-05-01 |
| 49 | v1.9 | 1/1 | Complete   | 2026-04-25 |
| 50 | v1.9 | 4/4 | Complete   | 2026-04-28 |
| 51 | v1.9 | 6/6 | Complete   | 2026-04-29 |
| 52 | v1.9 | 6/6 | Complete    | 2026-04-30 |
| 53 | v1.9 | 6/6 | Complete   | 2026-04-30 |
| 54 | v1.9 | 5/5 | Complete    | 2026-05-01 |
| 55 | v1.9 | 0/? | Not started | - |
| 56 | v1.9 | 4/4 | Complete    | 2026-05-01 |

### Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness

**Goal:** Wire jsonrpc.Conn.OnNotification in Worker.Start so QuirkAdapter notification handlers actually run in production, and ship a deterministic JdtlsAdapter.WaitUntilJavaReady(ctx) gate so Java integration tests pass in default `go test ./...`.
**Requirements**: LSDISP-01, LSDISP-02, LSDISP-03, LSDISP-04, LSDISP-04b, JDTLS-RDY-01a, JDTLS-RDY-01b, JDTLS-RDY-01c, JDTLS-RDY-02, LSDISP-REG-01
**Depends on:** Phase 55
**Plans:** 4/4 plans complete

Plans:
- [x] 56-01-PLAN.md — Restructure ProcessHandle.Start lifecycle + add StartListen(ctx) + process_test.go (D-02, D-03)
- [x] 56-02-PLAN.md — Wire dispatcher + regression assertion in Worker.Start + worker_test.go + extend codec_test.go (D-01, D-04, D-05, D-11, D-12)
- [x] 56-03-PLAN.md — Implement JdtlsAdapter.NotificationHandlers + WaitUntilJavaReady + extend quirks_test.go (D-06, D-07, D-08, D-09, D-13)
- [x] 56-04-PLAN.md — Java integration gate + rust-analyzer integration regression + final go vet/go test gate (D-10, D-14, D-15)

## Backlog

_No items in backlog._
