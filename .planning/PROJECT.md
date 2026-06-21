# Helix

## What This Is

A Go-native code intelligence platform for MCP: universal LSP gateway at the core, agent skills as plugins. Single binary, persistent daemon, 41+ callable MCP tools, 52-language support across 23 tree-sitter grammars. Ships as `helix` (renamed from `serena` at v1.9 — module path `github.com/agenthands/helix`, env vars `HELIX_*`, config dir `~/.helix/`) with reproducible multi-arch signed releases via goreleaser, in-binary self-upgrade (`helix update` / `helix upgrade` with sigstore cosign-keyless bundle verify + atomic swap), full Prometheus + OpenTelemetry observability (5 RED metric families plus cache/repomap/session/edit families, 2 packaged Grafana dashboards, 4 runbooks), and per-MCP-tool + per-LS-call tracing. Targets coding agents (Claude Code, Codex, IDE assistants) that need semantic code operations — symbol-level retrieval, editing, refactoring — backed by real language servers with warm persistent caching. Every tool returns typed, structured errors for reliable programmatic error handling.

## Core Value

Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## Requirements

### Validated

- ✓ Go rewrite with daemon skeleton, MCP runtime, gRPC forwarder, HTTP transport — v1.0 Phase 1
- ✓ Single binary distribution via `go build` — v1.0 Phase 1
- ✓ Python Serena moved to `legacy/` as reference — v1.0 Phase 1
- ✓ LSP 3.17 metamodel codegen (324 structs, 216 union types) — v1.0 Phase 2
- ✓ JSON-RPC codec for LS communication — v1.0 Phase 2
- ✓ LS worker pool with adaptive TTL, circuit breaking, pressure eviction — v1.0 Phase 2
- ✓ 9 symbol retrieval tools (definition, references, overview, search, hover, implementations, call/type hierarchy, blast radius) — v1.0 Phase 2
- ✓ 6 symbol editing tools with tree-sitter body surgery — v1.0 Phase 2
- ✓ 6 file operation tools — v1.0 Phase 2
- ✓ 3 diagnostic tools (diagnostics, code actions, formatting) — v1.0 Phase 2
- ✓ 52-language embedded registry with YAML override — v1.0 Phase 3
- ✓ Three-tier LS installer (PATH/download/error) — v1.0 Phase 3
- ✓ Memory system (markdown + SQLite FTS5) — v1.0 Phase 3
- ✓ Skill interface (Skill/ToolProvider/WorkflowProvider) — v1.0 Phase 3
- ✓ 7 memory MCP tools + onboarding/handoff workflows — v1.0 Phase 3
- ✓ 5 agent profiles (claude-code, codex, ide-assistant, ci-bot, full) — v1.0 Phase 4
- ✓ 4 modes (read/edit/review/admin) with switch_mode tool — v1.0 Phase 4
- ✓ Token budget reporting via get_token_budget — v1.0 Phase 4
- ✓ Layered config precedence (CLI > project > user > profile) — v1.0 Phase 4
- ✓ Dynamic tool registry with pluggable skill packs — v1.0
- ✓ Centralized daemon bootstrap with 38+ callable MCP tools — v1.0 Phase 5
- ✓ Kernel tool skill adapters for uniform composition model — v1.0 Phase 5
- ✓ Integration test harness — end-to-end MCP round-trips (InMemory + HTTP) — v1.1 Phase 6
- ✓ Go dogfooding suite — all 38 tools tested against own codebase — v1.1 Phase 6
- ✓ Multi-language test fixtures — Python, TypeScript, Java, Rust — v1.1 Phase 7
- ✓ Symbol editing round-trip tests — all 6 edit tools — v1.1 Phase 7
- ✓ Profile/mode contract tests — golden file pattern with 19 goldens — v1.1 Phase 8
- ✓ Three-tier concurrency tests — scenarios + fan-out + testing/synctest — v1.1 Phase 8
- ✓ Three-band error path coverage — 30 cases across categories and destructive tools — v1.1 Phase 8

- ✓ README.md with capabilities, install, auto-generated tool/language tables — v1.2 Phase 14
- ✓ USAGE.md with profiles, workflows, troubleshooting, observability, performance tuning — v1.2 Phase 14
- ✓ CHANGELOG.md with v1.0, v1.1, v1.2 entries — v1.2 Phase 14
- ✓ Benchmark harness with testing.B.Loop, CI benchstat gate, baseline capture workflow — v1.2 Phase 9, 15
- ✓ Tool response time benchmarks (p50/p95/p99) for all 38 tools — v1.2 Phase 9
- ✓ LSP indexing throughput benchmarks (cold/warm) — v1.2 Phase 9
- ✓ Memory profiles (4 scenarios with dual Go/kernel RSS + pprof) — v1.2 Phase 9
- ✓ Observability foundation (internal/obs/, trace-aware slog, admin listener) — v1.2 Phase 10
- ✓ Prometheus /metrics with RED histograms, lspool gauges, bounded labels — v1.2 Phase 11
- ✓ End-to-end tracing (otelgrpc, telemetry middleware, per-tool spans, OTLP exporter) — v1.2 Phase 12
- ✓ Graceful degradation (per-class budgets, deadline propagation, ErrCircuitOpen, GOMEMLIMIT) — v1.2 Phase 13

- ✓ README.md updated with Production & Observability section, corrected tool count to 35+ — v1.3 Phase 16
- ✓ CONTRIBUTING.md rewritten as Go-native contributor guide — v1.3 Phase 16
- ✓ CHANGELOG.md v1.2 entry completed (Phase 15 gap filled) — v1.3 Phase 16
- ✓ USAGE.md accuracy gaps fixed (6 metrics, service_name config, PromQL examples, benchmarks subsection) — v1.3 Phase 17
- ✓ INSTALL.md created with per-agent MCP config for 6 coding agents + HTTP mode — v1.3 Phase 17
- ✓ Test harness extraction (test/harness/ package with Runner, tools, golden, fixture helpers) — v1.4 Phase 18
- ✓ Protocol oracle tests (handshake, tools/list, session isolation, reconnect) — v1.4 Phase 19
- ✓ Contract oracle tests (schema meta-validation, selectability heuristics, golden outputs, error contracts) — v1.4 Phase 19
- ✓ Scenario oracle tests (multi-language runtime correctness across 14+ languages/fixtures) — v1.4 Phase 20
- ✓ LLM behavioral tests (tool selection, disambiguation, output interpretation) + judge scoring infrastructure — v1.4 Phase 21
- ✓ Multi-provider LLM support (Anthropic + DeepSeek) for behavioral tests — v1.4
- ✓ Extension-based language detection fallback for marker-free languages (Markdown) — v1.4
- ✓ Marksman quirk adapter for Markdown LSP support — v1.4
- ✓ Typed error taxonomy with 7 error kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout) — v1.5 Phase 22
- ✓ All 38+ MCP tools migrated from raw strings to typed errors across 10 packages — v1.5 Phase 23
- ✓ Inline input validation at all 24 kernel tool boundaries — v1.5 Phase 24
- ✓ Kind-level error test assertions with extractKind helper and 4 typed golden files — v1.5 Phase 24
- ✓ Cause-chain wrapping preserving errors.Is/As for LSP, filesystem, and tree-sitter errors — v1.5 Phase 22

- ✓ RepoMap overview tool — structural map of repo with ranked symbol importance via get_repo_map — v1.6
- ✓ RepoMap context selection tool — task-focused token-budgeted context with PageRank via get_context — v1.6
- ✓ Hybrid data source — tree-sitter fast path (23 languages), LSP enrichment when warm, SQLite cache — v1.6
- ✓ Fuzzy edit fallback in existing tools — whitespace-normalized matching in replace_symbol_body / replace_in_file — v1.6
- ✓ Standalone fuzzy edit MCP tool — raw text fuzzy matching with strategy reporting — v1.6
- ✓ 23-language tree-sitter grammar support — full aider parity with tag and body queries — v1.6

- ✓ README.md rewritten as standalone Go-native product identity document (hero, Quick Start, architecture, auto-generated tool table with 41+ tools) — v1.8 Phase 39
- ✓ USAGE.md refreshed through v1.7 feature set (fuzzy editing 4-strategy cascade, RepoMap, 23 grammars, smart errors, progressive descriptions, lazy init) + Troubleshooting (jdtls/gopls/rust-analyzer) — v1.8 Phase 40
- ✓ INSTALL.md Quick Start + manual configuration for 9 clients; CONTRIBUTING.md project structure and Go-native contributor guide — v1.8 Phase 41
- ✓ CHANGELOG.md v1.6/v1.7 entries; CLAUDE.md architecture matches reality across all 4 layers — v1.8 Phase 42
- ✓ Cross-doc truth sync: closed F-01/F-03/F-04/F-05/F-07/F-08/F-10/F-11 drift between README/USAGE/CLAUDE/CHANGELOG/CONTRIBUTING and source of truth (setup_clients.go, fuzzy/strategies.go) — v1.8 Phase 43
- ✓ Re-verified Phase 41 (41-VERIFICATION.md created) and USAGE-02; v1.8 integration-check re-run with 0 open criticals/warnings — v1.8 Phase 44
- ✓ Closed F-02 (README manual-config explicit 7-client pointer to INSTALL.md#manual-configuration), F-12 (README→CHANGELOG + USAGE→INSTALL cross-links), confirmed F-06 rust-analyzer troubleshooting metadata current — v1.8 Phase 45
- ✓ Python legacy acknowledgment scoped to a single CLAUDE.md disclaimer; no "port" or "rewrite" language anywhere else — v1.8 Phase 42

- ✓ `get_repo_map` returns ranked Go sources (not Lua testdata fixture) on polyglot workspaces — v1.9 Phase 46 (BUG-01)
- ✓ `rename_symbol` succeeds on Rust symbols via experimental/serverStatus readiness + RenameOverride QuirkAdapter — v1.9 Phase 47 (BUG-02)
- ✓ Java integration tests run in default `go test ./...` via warm jdtls cache (`jdtlscache` helper + env-var override + `Options.JdtlsDataDir`) — v1.9 Phase 48 + Phase 56 (BUG-03)
- ✓ Single canonical `GrammarRegistry` injected from daemon bootstrap into all consumers — v1.9 Phase 49 (BUG-04)
- ✓ Go 1.25 + gopls green on `ubuntu-latest`; benchmark harness converted to local-only (CI bench plumbing removed) — v1.9 Phase 50 (TOOL-01, TOOL-02)
- ✓ Reproducible multi-arch signed release pipeline via goreleaser — 6 archives × darwin/linux/windows × amd64/arm64 with minisign signing + reproducibility gate — v1.9 Phase 51 (PKG-01, 3/4 SC verified; SC-3 deployment-gated)
- ✓ CGO=0 build path preserved via `//go:build cgo` stubs across treesitter/repomap/edit; daemon refuses CGO=0 with remediation — v1.9 Phase 51.1 (DEF-51-01, emergent)
- ✓ Product rename `serena → helix`: binary, module path `github.com/agenthands/helix`, env vars `SERENA_* → HELIX_*`, config dir `~/.serena → ~/.helix`, MCP server identity — hard-cut breaking change at v1.9 — v1.9 Phase 52 (PKG-05)
- ✓ In-binary self-upgrade: `helix update` (read-only check) + `helix upgrade` (sigstore cosign-keyless bundle verify + atomic swap + downgrade refusal + daemon-aware re-launch) — v1.9 Phase 52 (PKG-06), cosign rewrite v1.10 Phase 58 (REL-01)
- ✓ EMBED-AUDIT.md manifest classifying every runtime asset; embedded sigstore TUF trust root with CI gate — v1.9 Phase 52 (PKG-07), trust-root refresh v1.10 Phase 58 (REL-01)
- ✓ 5 new Prometheus metric families (lspool/repomap cache hit-rate, repomap extract latency histogram, session lifecycle, edit outcomes — all bounded labels with cardinality test) — v1.9 Phase 53 (OBS-03)
- ✓ 2 Grafana dashboards (`helix-overview.json`, `helix-engine.json`) + 4 runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction); registry-driven PromQL validator — v1.9 Phase 54 (OBS-01, OBS-02)
- ✓ Full per-MCP-tool + per-outbound-LS-call trace coverage; TRACE-AUDIT.md hygiene review (no PII, bounded cardinality); real Jaeger smoke capture — v1.9 Phase 55 (OBS-04, application chain fully verified; forwarder span deferred to v1.10)
- ✓ LS notification dispatch wired in production: `jsonrpc.Conn.OnNotification` set in `Worker.Start` (was silently dropped); `JdtlsAdapter.WaitUntilJavaReady(ctx)` deterministic gate — v1.9 Phase 56 (LSDISP-01..04b, JDTLS-RDY-01a/b/c, JDTLS-RDY-02, LSDISP-REG-01, emergent)

### Active

_v1.12 requirements to be defined via `/gsd-new-milestone`._

Carry-over follow-ups (resolved at v1.10 Phase 58):
- [x] ~~**PKG-01 SC-3** (deployment): maintainer minisign keypair + first v* tag~~ — replaced by sigstore cosign keyless (D-02 hard cut, no minisign coexistence). First signed release `v1.10.0-rc1` pushed 2026-05-04 via Phase 58 REL-01.
- [x] ~~**Phase 51 reproducibility gate** (architectural): extend gate scope to real-release-vs-Pass-3~~ — Phase 58 REL-05 documents Pass-3 limitation in CONTRIBUTING.md (won't-do per D-04); reopen path captured if/when needed.
- [x] ~~**Phase 55 forwarder.tools.call span** (architectural): unify with gRPC server span~~ — Phase 58 REL-06 wired real TracerProvider + per-handler `propagation.TraceContext{}`; `TestE2ETraceContinuity` proves trace continuity.
- [x] ~~**PKG-DEFER-03/04/05**: Homebrew tap, Scoop bucket, native Linux package~~ — won't-do per Phase 58 D-01 (single-binary distribution intent; cosign-verifiable goreleaser archives are the canonical channel).

### Out of Scope

- JetBrains plugin backend — dropped, LSP-only going forward
- Python compatibility layer — native Go, no Python interop
- Mobile/embedded targets — server-side only
- Custom language server implementations — wrap existing LSP servers, don't reimplement
- Knowledge graphs — CodeGraphContext/GitNexus own this space
- Vector/embedding search — Augment Context Engine does this better
- Git operations — GitHub MCP Server handles git comprehensively

## Current State

**Shipped:** v1.10 Live Semantic Index (2026-05-12) — 12 phases (57–67, including emergent 59.1), 79 plans, 64/64 in-scope REQs satisfied (3 won't-do: REL-02/03/04 superseded by Phase 59.1). 11/11 cross-phase flows wired. Audit status: `resolved`, verdict `PRODUCTION-READY`. See `.planning/milestones/v1.10-ROADMAP.md` and `.planning/v1.10-MILESTONE-AUDIT.md`.

**Previously shipped:** v1.9 Polish & Infra (2026-05-03) — 12 phases (46–56, including emergent 51.1), 51 plans, 14/14 in-scope REQs satisfied.

> **Last hygiene sweep:** 2026-05-12 (F-58 close-out). All minisign and PKG-DEFER-03/04/05 references in this document are historical context; no open deferrals remain. See lines 112 / 115 / 171 / 174 for the explicit won't-do dispositions citing Phase 58 D-01 (single-binary distribution) and Phase 58 D-02 (cosign hard-cut).

**v1.10 progress (2026-05-03):** Phase 57 complete — Semantic Store Foundation + Pipeline DAG Library. Stdlib `internal/phasegraph/` library validates pipeline DAGs (cycle/dup/missing-dep) and ships three pipeline shape declarations (semantic 12 / live 9 / eval 10) ready for Phase 60/67 to fill. `internal/semantic/{config,store,types}` skeleton lands as the sole owner of `duckdb-go` (D-12), with three-tier open (existing+clean → quarantine+rebuild → hard-fail), Schema 1 empty-but-correct, CGO=0 stub mirror, and two new bounded-label metrics. `cmd/vet-noduckdb` standalone analyzer enforces the STORE-06 boundary mechanically; `make test` chains through `make vet`. `semantic_index.*` config keys (65 defaults) flow through the existing 4-layer koanf precedence. SC-1 `get_health` clause deferred to Phase 65 (which explicitly owns the strangler-fig integration). 2 critical code-review findings (CR-01 path-traversal mitigation no-op, CR-02 missing schema-probe timeout) triaged for follow-up hardening pass.

Helix now ships as a single self-contained signed binary with reproducible multi-arch goreleaser releases (6 archives × darwin/linux/windows × amd64/arm64), in-binary self-upgrade with sigstore cosign-keyless bundle verification + atomic swap + daemon-aware re-launch, full Prometheus + OpenTelemetry observability (5 new metric families, 2 packaged Grafana dashboards, 4 runbooks, full trace coverage), and CGO=0 build path preserved via tree-sitter stubs. All 4 known LSP/tooling bugs (BUG-01..BUG-04) closed. Product rename `serena → helix` executed as a hard-cut breaking change at v1.9 (binary, module path `github.com/agenthands/helix`, env vars `HELIX_*`, config dir `~/.helix/`, MCP server identity).

**v1.10 progress (2026-05-04):** Phase 58 complete — v1.9 carryover & release distribution. Sigstore cosign keyless replaces minisign at v1.10.0 (hard cut, no coexistence per D-02). First signed release `v1.10.0-rc1` cut 2026-05-04: 14 release assets, all `.sigstore.json` bundles in proto schema with RFC3161 timestamps from sigstore public-good TSA, three independent verify paths agreed (cosign verify-blob CLI, sigstore-go strict in-process verifier, real `helix upgrade` against the rc1 release). Required four post-rc1 hardening fixes (WR-07 Linux repro-gate drift, WR-08 cosign `--new-bundle-format`, WR-09 RFC3161 TSA timestamp required, WR-10 use sigstore public-good TSA + refresh trust root from live TUF). Forwarder→daemon OTel trace continuity now wired via real TracerProvider + per-handler `propagation.TraceContext{}` (REL-06). Phase 51 Pass-3 reproducibility limitation documented as won't-do (REL-05). REL-02/03/04 (Homebrew/Scoop/native Linux) explicitly recorded won't-do per D-01.

**v1.10 progress (2026-05-05):** Phase 60 complete — Live Update Pipeline. LIVE-01..LIVE-07 all validated. Schema v3 ships the `current_epoch` (overlay meta) + 4× `write_epoch` (overlay rows) contract under `-race` concurrent stress; `BeginOverlayTx` allocates the epoch on `s.db` (not the per-tx Tx) so rollback preserves monotonicity. `internal/kernel/notifier.go` defines the `EditNotifier` seam wired into 5 edit + 4 fileops tools (9 OnEdit hook sites, fire-and-forget); `internal/lint/nokernel2semantic` go/analysis vet analyzer mechanically enforces the kernel⊥semantic boundary, run from `make vet` alongside `vet-noduckdb`. The live spine (`internal/semantic/live/`) ships Signal/Classifier/Coalescer/Handler/Service + scheduler.ScheduleIncremental, with per-workspace fsnotify watcher (Vim/JetBrains/VS Code editor-fixture corpus, `inotify ENOSPC` one-shot warn + manifest-poll fallback) and xxhash64 manifest scanner (immediate-first-scan + diff-emit + `helix_semantic_live_updates_total{kind, outcome}` bounded-label counter at coalescer drop/applied/error sites). Daemon bootstrap calls `kernel.SetEditNotifier(liveService)` and validates wiring via `phasegraph.RunPhaseGraph(BuildLiveUpdatePhases(...))` to fail-fast on missing components. Two intentional Phase 62 stubs documented: `storeFileHashLookup.EffectiveContentHash` and `scannerStoreLookup.KnownFiles`. End-to-end smoke test (`internal/daemon/live_e2e_test.go`) drives `kernel.EditNotifier().OnEdit` through coalescer→handler→overlay tx, asserting `current_epoch` advance, `write_epoch` stamp, and `lspqueue.Len()==1` (Phase 61 producer-side promise honored). 4 code-review BLOCKERs (CR-01 metric emission, CR-02 watcher symlink rejection, CR-03 phase-graph validator invocation, CR-04 lspqueue producer wiring) closed inline with regression tests before phase mark-complete. 33/33 must-haves verified.

**v1.10 progress (2026-05-08):** Phase 65 complete — Existing-Tool Integration (Strangler Fig). INTEG-01..INTEG-05 all validated 4/4 must-haves on re-verification (waves 4-7 closed both prior BLOCKERs from initial `gaps_found` verdict). New types-only `internal/semantic/integ` package owns the read-only `SemanticLookup` seam with closed-enum `Source`/`FallbackReason`, `RankedFile`, `Impact`, `Edge`, `ValidatedEdge`, `SemanticStatus`, `SymbolID`, error sentinels, and `NoopLookup` default. Production `integSemanticLookup` adapter at `internal/daemon/semantic_wiring.go` wraps `*semanticstore.Store`, `*retrieval.Engine`, the rank bundle, and the `wsKeyFn` closure (Phase 64 carryover #2 closed). All 4 strangler-fig tools consult the lookup with v1.9 fallback preserved: `get_repo_map`/`get_context` route through `RepoMapSkill.SetSemanticLookup` (zero source change to `internal/repomap` engine, JSON-wrapped envelope with `source`/`fallback_reason`/`graph_version`/`freshness`); `analyze_blast_radius` uses a two-pass orchestrator (`ExpandFrom` → kernel-side `lspProbeForEdges` via `FindReferences` against the orchestrator-held lease); `get_health` adds the additive `semantic_index` block (Phase 57 SC-1 `semantic_store` block preserved verbatim). Real implementations replace 65-03 stubs: `RankFiles` reads `*Store.QueryRankedFiles`, `RankFromSeeds` performs RRF fusion (k=60) over persisted scores + bleve, `SymbolID`/`ExpandFrom`/`LocateSymbol` traverse the persisted graph via BFS over `QueryEffectiveAdjacency`. Production buildFn empty-Facts placeholder (Phase 64 carryover #1) replaced by per-language extract → classifier walk → `ToStoreFacts` → `WriteSnapshotFacts`. `internal/lint/nokernel2semantic` allowlists `internal/semantic/integ` so kernel-side blast-radius can import the seam. Six production-adapter E2E tests + the BL-1 kernel-side confidence-ladder regression run with no Skipfs. Two REVIEW.md BLOCKERs deferred for follow-up: CR-01 (`lspProbeForEdges` validates inverted edge relation — references edge.From and checks containment in edge.To, answering the wrong direction) and CR-02 (`integ_lookup_export.go` + `integ_lookup_e2e_helpers.go` import `testing` from production package per Rule-3 deviation; linker DCE removes the named test fixtures from the helix binary, residual cost is symbol-table footprint only).

**v1.11 progress (2026-05-21):** Phase 73 complete — P1 Tools Integration & E2E Verification, the final phase of milestone v1.11. All 6 P1 MCP tools (`explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph`) are now exposed through `SemanticSkill.Tools()` (10-tool ToolProvider surface) and gated across the full 5-profile × 4-mode matrix — `get_change_impact_graph` (review+) excluded from read/edit mode YAMLs, the other five visible at read+. Verified by an inline table-driven profile-filter golden suite (40 subtests). All 6 `*Help` consts standardized to a 4-section template with `get_tool_help` param-doc coverage tests via `jsonschema.For[T]`. SC#3 enforced by a new in-tree static gate (`wrapper_consistency_test.go`) asserting every P1 handler calls `checkMode` + emits `FreshnessV2` + is registered in `RegisterAll`. SC#4 proven by a real-store E2E suite (`buildP1E2EFixture`: real DuckDB `*Store` + bleve in a tempdir) running all 6 tools with closed-enum envelope assertions; `get_change_impact_graph` additionally asserts a non-degenerate subgraph. P1TOOL-07/08/09 all validated; 5/5 must-haves verified; both custom vet gates green; race-clean. Code review: 0 critical, 5 warning (WR-01/04/05 pre-existing Phase 72 code), 6 info.

**v1.12 progress (2026-06-15):** Phase 75 complete — Schema, Fairness Contract & Tree Skeleton, the foundation phase. All 11 in-scope REQs (BENCH-01/02/03/06, FAIR-01/02/03, COST-01, INFRA-01/02/03) accounted for; 5/5 ROADMAP success criteria + 11/11 must-haves verified. Wave 0 relocated the Phase 64 semantic microbench to `internal/semantic/bench/` via atomic `git mv` (history continuity preserved, eval↔bench reciprocal INFRA-03 links). The six-dir `bench/` skeleton (`datasets/runners/languages/evaluators/reports/schema`) now hosts the contracts every downstream phase (76–89) writes into: `bench/schema/result.v2.schema.json` (Draft 2020-12, required `schema_version` const "v2", additive-only=minor / breaking=v3, FAIR-03 substrate cached-token columns + `fairness.overrides[]`) with a self-enforcing golden-validate test; `bench/runners/fairness_contract.go` (`var DefaultContract` pinning a dated `ModelID`, `ModeOverride` waiver loader that fatals on empty `WaiverReason`, injected-clock 30-day deprecation gate); and the `cmd/helix-bench` cobra CLI (5 subcommands) + `bench/datasets/cost-table.yaml` + hard-fail `make validate-cost-table` / `make verify-tos` gates. FAIR-03 ships schema substrate only (variance detector → Phase 82, `cost_quality.md` warning → Phase 89). TOS attestation flags set to permissive defaults and confirmed by the user (live-TOS legal verification handled going forward); a code-review BLOCKER (CR-01: `verify-tos` silently skipping the Anthropic attestation via a Markdown horizontal-rule fence desync) was found and fixed with a regression test. Zero new dependencies (jsonschema/v6, yaml.v3, cobra all pre-vendored). Two pre-existing `test/bench` failures (MCP registry 53-vs-47 tool drift) are unrelated and logged in deferred-items.md.

**v1.12 progress (2026-06-16):** Phase 76 complete — Ablation Profiles + Kernel Subsystem Disable Flags, the only invasive-daemon phase of v1.12. All 4 in-scope REQs verified (4/4 must-haves, all 4 ROADMAP success criteria + both hard-fail gates): ABLATE-07 (two kernel flags `KernelConfig.DisableLSPSubsystem`/`DisableStructuredEditSubsystem` + accessors; structured-edit handlers return greppable `subsystem_disabled:`-prefixed `serr.Unsupported` under the flag; `replace_in_file` falls through to exact-match-only), ABLATE-02 (4 bench profile YAMLs `bench-full`/`bench-no-lsp`/`bench-no-semantic`/`bench-no-structured-edit` + first-class `Profile` disable fields + golden tool-surface tests + fail-closed loader rejection of unknown mode names), ABLATE-05 (no_lsp enforced by null-object injection at the daemon composition root — skip `SetEnrichFn`/`SetFallbackDeps`, no-op `EditNotifier`, neutralized diag `leaseFn` — proven by a zero-`lspool.lsp.*`-span trace-tap test), ABLATE-08 (`vet-ablation-leakage` import-boundary analyzer wired into `make vet`, forbidden edge `bench/runners → lspool|semantic/store` NOT the legitimate `kernel→fuzzy`, green→red via analysistest testdata). All TDD (RED→GREEN per plan). Per D-11/D-12, `bench-no-semantic` ships tool-filter-only; the kernel `disable_semantic_subsystem` flag (ABLATE-06) is deferred to Phase 81 (Phase 65 SemanticLookup un-wiring dependency). Code review: 0 blockers, 3 warnings (WR-01: structured-edit guards emit `outcome="unsupported"` which is outside the closed `editOutcomeEnum` → metric silently dropped; WR-02: direct LS-leasing tools lack a kernel runtime guard under no_lsp; WR-03: `Validate()` not re-run after `LoadOverrides()`) flagged for a follow-up `--fix` pass. Pre-existing `test/bench` 53-vs-47 tool-drift failures (last touched Phase 66) confirmed unrelated; logged in deferred-items.md.

**v1.12 progress (2026-06-17):** Phase 77 complete — Bench Runtime & First E2E Smoke, the last phase of the v1.12 active roadmap. Both in-scope REQs (BENCH-04, BENCH-05) and all 4 ROADMAP success criteria verified by real execution (16/16 must-haves). The `notYetImplemented("run")` stub at `cmd/helix-bench/main.go` is now a working `run` subcommand (`--benchmarks`/`--modes`/`--tasks`/`--parallel`/`--out`/`--agent`): `helix-bench run --benchmarks=toolbench-go --modes=your_agent_full --tasks=sum-doubler` runs one cell end-to-end in ~0.4–2s (≤30s) and writes a schema-valid `result.v2.json` (outcome+fairness+tokens+`trace_ref`+`model_id`+`schema_version:"v2"`; rich metrics edit_locality/regression_rate/pass@k intentionally absent → Phase 79). `bench/runtime/sandbox` struct-embeds `internal/eval/sandbox.Sandbox` (reuse-don't-fork, D-07); `bench/runtime/subprocess` spawns the per-cell `helix` daemon over a Unix domain socket (HTTP disabled → no port collisions by construction, D-06); the cell spine (drive→kill→PID-gated `TapDaemonLog`→2-leg `trace.Merge`→`BuildResult`/`Validate`→durable write under `bench/reports/<run_id>/<task>/<mode>/`, preserve-on-failure D-08) mirrors `internal/eval/runner/daemon_tap_integration_test.go`. The agent-tap leg is synthesized as a `CCTapResult` from the scripted agent's `[]StepResult` (D-02) so the Nyquist 3-assertion boundary (2-leg trace `ToolCallSummary.Total>=1` + CC leg present; `RejectedForeignPid==0`; result.v2 schema-valid) is a hermetic CI gate — proven by `TestDaemonTap` + `TestCrossCell` (run for real with a built helix, SKIP cleanly without one). `make bench-quick` exits 0 in ~4s (≤90s, 1/1 task) on the hermetic scripted `your_agent_full` path; the `make bench` target-name collision with the existing Go microbench was reconciled (microbench → `bench-micro`, new `bench`/`bench-<suite>` invoke `helix-bench run`). Real `claude` CLI path wired (`--agent=claude`, `ErrClaudeNotFound` clean) but NOT a CI gate (D-01). One in-scope deviation: per-cell `semantic_index` disabled via `--config` to avoid a shared cwd-relative DuckDB-store deadlock under parallel cells (bench-full tool surface otherwise intact; flagged for Phase 78+). Code review: 0 blockers, 6 warnings (WR-01 timeout-as-failure signal conflation, WR-02/03 drive.go goroutine-leak/out-of-order handling, WR-04 dotfile in dataset dir, WR-05 process-group `Setpgid` reaping, WR-06 trace start-time sampling) — all advisory refinements that bite only at Phase-78+ corpus/parallel scale; recommend tracking WR-01/WR-05, logged for a follow-up `--fix` pass. TDD plans 77-01/77-02 followed RED→GREEN.

**v1.12 progress (2026-06-19):** Phase 80 complete — Five-of-Six Ablation Runners + Fairness Enforcement. Both in-scope REQs (ABLATE-01, ABLATE-03) + all 4 ROADMAP success criteria verified by real execution (20/20 must-haves; `TestFiveOfSixSmoke` ran live with a spawned daemon). Grew Phase 77's filesystem-as-table mode resolver from 1→6 modes with ZERO resolver Go change — 5 new `bench/runners/<mode>/MODE.md` dirs (`baseline_plain`→baseline, `no_lsp`/`no_structured_edit`/`your_agent_no_semantic`→their `bench-*` profiles, `baseline_rag`→baseline placeholder). Two asymmetric deferred modes (D-02): `your_agent_no_semantic` emits a REAL row marked `ablation_status: guarantee_pending_phase_81` (kernel zero-DuckDB guarantee ABLATE-06 → Phase 81); `baseline_rag` is a fail-closed stub emitting NO row + a distinct matrix `Deferred` outcome (real RAG arm ABLATE-04 → Phase 83). Fairness is two layers (D-04): `RunCell` calls `runners.DefaultContract.Validate()` at startup (fatal, before sandbox) + a new unconditional hermetic CI contract test (`bench/runners/contract_test.go::TestEffectiveConfigMatchesContract`) asserting projected `model_id == DefaultContract.ModelID` (Open-Q1 scope A — the live `claude` argv projects only model_id today; the other 5 contract fields documented wired-not-enforced). Additive `ablation_status` field on the result.v2 builder + open schema (no v3 bump, D-03). Minimal single-run 3-delta pass (`bench/runtime/deltas.go::ComputeAndWriteDeltas`, D-05) computes `full − baseline_plain` / `full − no_lsp` / `full − no_structured_edit` post-`RunMatrix` and writes them back into each real row; the full aggregator/BCa/pass@k/variance gate stays Phase 82. baseline_plain reuses `internal/profile/profiles/baseline.yaml` (ABLATE-03, no new YAML; empty inventory documented in `bench/BENCH.md`). All TDD plans (80-02/03/04/05) followed RED→GREEN. Code review: 0 blockers, 5 warnings (WR-01/02 durable-layout test-path drift, WR-03 latent non-deterministic `fairness.overrides[]` ordering, WR-04/05 stale comments) — advisory, logged for a follow-up `--fix` pass.

**v1.12 progress (2026-06-20):** Phase 81 complete — `no_semantic` Kernel Flag + E2E Config-Gate Test. ABLATE-06 satisfied; all 4 ROADMAP success criteria verified by real execution (4/4 must-haves on re-verification). The `disable_semantic_subsystem` gate resolves `effSemanticDisabled` once at the daemon composition root (`daemon.go:294`, mirroring Phase 76's `effDisableLSP`) and un-wires Phase 65's `SetSemanticLookup` strangler-fig for all 8 enumerated consumers — under **D-04 build-but-block** the store and semantic bundle stay BUILT (only reads are gated), so the zero-reads proof is non-vacuous. Two gap-closure plans (81-06/81-07) closed the gaps the first verification found: **WR-02** (criterion #2 was vacuous end-to-end — bench SIGKILLed the daemon so the `helix_semantic_store_reads_total` line was never emitted and the scraper read a missing line as count=0) closed by a graceful `DaemonHandle.Stop` (SIGTERM flushes the line via `d.shutdown()`) + a fail-CLOSED scrape/assert; **CR-01** (criterion #1 was partial — daemon-internal background read pipelines `SetActivateCallback` drivers + `SetFileFactStore` were ungated) closed by gating all six read-drivers on `backgroundSemanticReadsDisabled(effSemanticDisabled)`. Runtime proofs are HELIX_BIN-gated real-daemon tests (`TestNoSemanticReadsTotalLineEmitted`, `TestNoSemanticStoreOnZeroReads`) proven to RUN (not SKIP) and PASS; criterion #3 is the extended `vet-ablation-leakage` call-site gate. Code review on the gap-closure changes found 1 BLOCKER (concurrent `(*exec.Cmd).Wait()` from Stop-then-Kill — would flake the new fail-closed gate) + 4 warnings, all fixed and re-verified race-clean (single owned daemon Wait goroutine; graceful-stop budget sized to kernel+trace-flush worst case; malformed-count fail-closed).

**v1.12 progress (2026-06-21):** Phase 82 complete — Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard. STATS-01..04 + COST-02/03 satisfied; all 4 ROADMAP success criteria verified by real execution (4/4 must-haves). The first externally-publishable bench artifact lands as a new pure-Go `bench/aggregator/` package (mostly unit-testable without HELIX_BIN) consumed by a `helix-bench aggregate <run_dir>` subcommand: it reads N≥3 `result.v2.json` runs per (task,mode) and emits `leaderboard.md` + `cost_quality.md`. Statistical core (hand-rolled, stdlib-only per the repo ethos): a genuine **BCa bootstrap** (bias-correction z0 via `math.Erfinv` + jackknife acceleration, seeded ≥10,000 resamples, the fake-BCa discriminator test asserts BCa≠percentile on a skewed sample); the **HumanEval unbiased pass@k** as the c-term product `1−Π_{i=n−c+1}^{n}(1−k/i)` (NOT the naive `1−(1−p)^k` — a plan-checker BLOCKER caught and corrected the k-term-loop before execution; anchored to PassAtK(10,3,5)=0.91667 + anti-naive + lgamma-agreement tests); `cost_per_solved_task` joined to `bench/cost` (cost-table types moved out of `cmd/helix-bench` into an importable package) with a fail-closed freshness gate (golden 3.555 USD). STATS-04 CI-overlap honesty gate suppresses unjustified "X>Y" claims; FAIR-03 CV>0.05 variance warning renders in cost_quality.md; reports are seed-deterministic (byte-identical golden .md). Producer side: `ExpandMatrix` now emits N cells per (task,mode) (`--runs`, default 3); the aggregator is **fail-closed** — a deficient cell OR an empty/missing run dir is a hard error, never an empty success report. Code review found 1 BLOCKER (N-gate failed OPEN on zero-discovery — empty run dir produced empty reports + exit 0) + 6 findings, all fixed and re-verified. Known pre-existing debt surfaced (NOT a Phase 82 regression): `TestRunSubcommandWiresDeltaPass` (Phase 80-05 RED whose GREEN never landed) fails under HELIX_BIN because `runBench` never wired the single-run delta pass — tracked for a Phase 80 follow-up.

**v1.12 progress (2026-06-21):** Phase 83 complete — `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder. ABLATE-04 satisfied; all 4 ROADMAP success criteria verified goal-backward by real execution (4/4 must-haves). The RAG baseline is now a competent control arm, not a strawman: a new leaf package `bench/ragindex/` (chromem-go v0.7.0, the only new dependency — zero-dep embeddable vector DB with built-in OpenAI `text-embedding-3-small` + Ollama `nomic-embed-text` embedders, plus a deterministic `stub-deterministic` embedder for offline CI) builds and caches a per-corpus index once per `(corpus_sha, embedder)` at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/` (warm reopen proven zero-re-embed). A standalone `package main` `cmd/helix-bench-rag` MCP server exposes EXACTLY 4 tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`) via the MCP Go SDK directly — provably sharing no code with the daemon: enforced by BOTH a dynamic transitive `go/packages NeedDeps` test AND a static `internal/lint/benchragleakage` analyzer wired into `make vet` (no import of `internal/{kernel,semantic,mcp}`). The Phase-80 fail-close in `RunCell` is replaced by a real drive leg (`subprocess.StartRAGServer`) that builds the index OUT-OF-BAND before the timed span (embedding cost excluded from the per-task budget, documented in `bench/BENCH.md`), reuses `runners.DefaultContract` verbatim (same-model-same-budget, SC#4), and records an additive `embedder_id` open-provenance key on every `result.v2.json` row (SC#3); the Phase-80 `TestDeferred*` deferral tests were flipped to assert a real row, and `baseline_rag` was added as a `full_minus_baseline_rag` delta operand. All three plans TDD (RED→GREEN). Code review: 0 blockers, 5 warnings — all 5 fixed and re-verified non-regressive (WR-01 warm-reopen embedder pinned fail-closed to the recorded `embedder_id`; WR-02 `validatePath` now resolves symlinks; WR-03 byte caps on `read_file`/`rag_search`; WR-04 RAG-server stdout reaped at drive-end; WR-05 grep skips binary files). HELIX_BIN+HELIX_BENCH_RAG_BIN-gated bench/runtime cell tests proven to RUN (not the MEMORY false-green skip) and pass.

**v1.12 progress (2026-06-21):** Phase 84 complete — Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard. CONTAINER-01..04 satisfied; all 4 ROADMAP success criteria verified goal-backward (4/4 must-haves; verifier ran build/test/`make vet` itself). A new leaf package `bench/container/` delivers the public-benchmark container substrate WITHOUT the Docker Go SDK: SC#1 — docker→podman engine detection driven purely by `os/exec` (fixed argv, strict 3-key env allowlist), an anchored `verify-no-docker-sdk` make-vet gate that hard-fails on `github.com/docker/docker` (the allowed transitive `docker/cli`/`distribution`/credential-helpers from crane are excluded by the `[[:space:]]` anchor), and an arch-mismatch refusal with a `BENCH_ARCH_MISMATCH_OK=1` escape; SC#2 — SHA256 **digest**-pinned images (never tags) cached at `$HELIX_CACHE_DIR/bench-images/<sha>/` with an `isHexSHA256` path-escape guard, cache-hit-on-rerun, and an `Ensure` made idempotent under concurrency/crash-recovery; SC#3 — in-process cosign verification via `sigstore-go` (cloning `internal/upgrade/verify.go`'s pinned-issuer + re-pinned-SAN + single-canonical-error discipline) wired verify-BEFORE-pull into the cache fetch closure so a verify failure publishes zero bytes, proven hermetically with VirtualSigstore tampered/unsigned/wrong-org/wrong-issuer fixtures, plus a `.github/workflows/bench-mirror.yml` cosign-keyless publisher whose minted SAN byte-matches the runtime verifier pin (the live published-mirror confirmation is honestly DEFERRED until the GHCR namespace is published — a ROADMAP-sanctioned boundary, decision logic already hermetically proven); SC#4 — a cross-platform 50 GiB disk-budget guard (`x/sys` Statfs / GetDiskFreeSpaceEx, build-tag split, injectable `availFn`) with a single-line remediation. Daemon-free throughout (crane for digest+arch inspection), so the whole phase is unit-testable with no docker/cosign/network present; live tests SKIP cleanly with hermetic siblings. A background commit-security review flagged a MEDIUM argv flag-smuggling vector (a `-`-prefixed repo) — fixed inline with an `isValidRepo` validator + a `--` end-of-options separator + a dedicated rejection test. Code review: 0 blockers, 4 warnings — all fixed and re-verified race-clean (WR-01 cache `os.Rename`-over-existing wedge → idempotent adopt-winner; WR-02 bench-mirror signed the dest-repo digest not `RepoDigests[0]`; WR-03 un-wired `VerifyThenPull` guarded by canonical `ErrNotConfigured`; WR-04 windows group-kill comment downgraded to a known-limitation note).

## Current Milestone: v1.12 Bench Stack & Tool Evaluation

**Goal:** Prove Helix's semantic / LSP / edit tooling improves agent success, cost, and safety on real public benchmarks across 8 Tier-1 languages — with controlled ablations and an internal ToolBench as the deterministic source of truth. Headline claim to land: *"Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits."*

**Target features:**

*Foundation — Internal ToolBench (Option A):*

- **Net-new `bench/` tree** — `datasets/`, `runners/`, `languages/`, `evaluators/`, `reports/`. `eval/` (Phase 67 / v1.10) left as legacy; synthetic corpus may migrate later but is not the source of truth.
- **Internal ToolBench** — deterministic, per-language tool-contract tests for 10 capabilities: semantic view, LSP diagnostics, rename safety, fuzzy search, call graph, dependency graph, patch apply, context minimization, incremental update, failure handling.
- **8 Tier-1 languages** — Python, TypeScript, JavaScript, Go, Java, C#, C++, Rust.
- **6-mode ablation matrix** — `baseline_plain` (shell+grep+read+edit+test), `baseline_rag` (grep + embeddings + chunk RAG), `your_agent_full` (semantic + LSP + fuzzy + symbol graph + structured edits + diagnostics), `no_lsp`, `no_semantic`, `no_structured_edit`. Same model + same budget across all modes.
- **Metrics layer (12+)** — `task_success`, `verified_correctness`, `tokens_input/output`, `tool_calls`, `wall_time_seconds`, `files_read`, `bytes_read`, `files_modified`, `edit_locality`, `regression_rate`, `lsp_diagnostics_used`, `semantic_tool_calls`, `edit_distance_patch`, `retry_count`. Normalized per-task result object.
- **Statistical rigor** — multi-run per task (N ≥ 3 by default), bootstrap confidence intervals, `pass@1` and `pass@k`. Single-run results are not publishable.
- **Dollar-cost conversion** — static price table per provider × model, computed `cost_per_solved_task` (reopens Phase 67 deferred item).

*Public benchmarks (Option B):*

- **Aider Polyglot** — 225 Exercism tasks across C++, Go, Java, JS, Python, Rust.
- **CrossCodeEval** — cross-file completion across Python, Java, TS, C#.
- **RepoBench** — retrieval + completion across Python, Java.

*External credibility (Option C):*

- **SWE-bench Verified** — 500 human-validated Python tasks.
- **Multi-SWE-bench** — 1,632 instances across Java, TS, JS, Go, Rust, C, C++.
- **Terminal-Bench 2.0** — 89 hard containerized long-horizon tasks.

*Reports:*

- `leaderboard.md`, `per_language.md`, `ablations.md`, `cost_quality.md`.

**Out of scope (deferred or won't-do):**

- HumanEval-style toy benchmarks as primary scoring (smoke-only via MultiPL-E / HumanEval-X / McEval if needed).
- Tier-2 (PHP, Ruby, Kotlin, Swift, C, Scala) and Tier-3 languages — future milestones.
- LLM judge as CI gate (per Phase 67 EVAL-07, judge stays informational).
- Comparing against Claude Code / Cursor as black boxes — controlled baselines using the *same* base model only.
- Public-benchmark *leaderboard submissions* — generating local results sufficient; submission infra deferred.

**Source of truth:** This PROJECT.md milestone section, the upcoming v1.12 `.planning/REQUIREMENTS.md`, and the user's BENCH-PLAN dump captured in the discuss-milestone transcript (2026-06-13).

## Context

Shipped v1.0 through v1.9. Single binary (`helix`, renamed from `serena` at v1.9), 4-layer architecture, persistent daemon. 41+ MCP tools, 52-language support, 23 tree-sitter grammars. Multi-oracle test harness (protocol, contract, scenario, LLM behavioral, judge scoring) plus per-MCP-tool + per-LS-call trace coverage with TRACE-AUDIT.md hygiene review. Reproducible signed multi-arch releases via goreleaser (6 archives × darwin/linux/windows × amd64/arm64 with sigstore cosign keyless; minisign retired at v1.10 per Phase 58 D-02). In-binary self-upgrade. Full observability: 5 new Prometheus metric families landed at v1.9 (cache hit-rate, repomap latency, session lifecycle, edit outcomes — all bounded labels), 2 packaged Grafana dashboards (`helix-overview.json`, `helix-engine.json`), 4 runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction).

Tech stack: Go 1.25 (gopls compatibility resolved at v1.9), official MCP Go SDK, koanf v2, modernc.org/sqlite, go-tree-sitter (23 grammars; CGO=0 stub path preserved via Phase 51.1), gRPC, prometheus/client_golang, OpenTelemetry (otelgrpc + otlptrace), goreleaser, sigstore cosign keyless (replaces minisign at v1.10 per Phase 58 D-02; release-side via cosign-installer in CI, verifier-side via embedded sigstore-go + live-TUF-sourced trust root).

Architecture: 4-layer (MCP runtime → Code intelligence kernel → Skills → Agent profiles). Persistent daemon with stdio/HTTP edge adapters. Worker pool with share-until-dirty, adaptive TTL, platform-aware pressure eviction, and **production-wired LS notification dispatch** (`jsonrpc.Conn.OnNotification` set in `Worker.Start`, regression-asserted; was silently dropped pre-v1.9). Single canonical `GrammarRegistry` injected from daemon bootstrap. RepoMap skill with PageRank using ambiguity-weighted edges (Phase 46 fix) + token-budgeted tree rendering. Fuzzy edit engine (4-strategy cascade) integrated into 3 MCP tools. Worker pool consumes structured LS readiness signals (rust-analyzer `experimental/serverStatus`, jdtls `language/status: ServiceReady`).

**Resolved at v1.10 (Phase 58, 2026-05-04):**
- ~~PKG-01 SC-3 deployment-gated on maintainer minisign keypair~~ → sigstore cosign keyless (D-02 hard cut), first signed release `v1.10.0-rc1` cut 2026-05-04
- ~~Phase 51 reproducibility gate snapshot-vs-snapshot, not real-release-vs-Pass-3~~ → documented as won't-do in CONTRIBUTING.md (REL-05)
- ~~Phase 55 forwarder.tools.call span emits via Noop tracer~~ → REL-06: real TracerProvider + per-handler `propagation.TraceContext{}` wired; `TestE2ETraceContinuity` proves trace continuity
- ~~PKG-DEFER-03/04/05: Homebrew tap, Scoop bucket, native Linux package~~ → won't-do per Phase 58 D-01 (single-binary distribution; cosign-verifiable goreleaser archives are canonical)

**Resolved at v1.9:** rust-analyzer rename quirk (Phase 47), jdtls cold-start in `go test ./...` (Phases 48 + 56), Go 1.25 / gopls linux/amd64 (Phase 50), GrammarRegistry duplication (Phase 49), repomap polyglot ranking (Phase 46).

The `legacy/` directory contains the original Python-based prototype (Serena name retained there as a historical reference). Helix is its own standalone Go-native product, not a port or rewrite.

## Constraints

- **Language**: Go — single binary, native concurrency
- **Protocol**: MCP (Model Context Protocol) — primary interface
- **LSP only**: No JetBrains or proprietary backends

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go implementation | Runtime/lifecycle correctness is the foundation problem | ✓ Good — clean single binary, native concurrency |
| Daemon + edge adapters (gopls pattern) | Warm cache, multi-client, reconnect | ✓ Good — worker pool with adaptive TTL |
| Drop JetBrains backend | Simplify to LSP-only | ✓ Good — 52 languages via LSP |
| 4-layer architecture | Prevents monolith; skills as plugins | ✓ Good — clean separation |
| gRPC internal IPC | Typed APIs between forwarder and daemon | ✓ Good — clean proto contract |
| Tree-sitter for body surgery | LSP-only editing was fragile (Serena pain point) | ✓ Good — precise body extraction for 4 languages |
| Full LSP metamodel codegen | Forward-sync with spec, no hand-maintained types | ✓ Good — 324 structs, 216 unions generated |
| Markdown + SQLite FTS5 for memory | Humans own content, engine owns search | ✓ Good — rebuildable index |
| Caddy-style skill registration | Compiled-in plugins without go-plugin overhead | ✓ Good — clean init() pattern |
| Centralized daemon bootstrap | Daemon owns all MCP registration; skills describe, daemon binds | ✓ Good — 38 tools registered, profile filtering, clean shutdown |
| Kernel tools as skill adapters | Uniform ToolProvider interface for profiles and modes | ✓ Good — 4 thin adapters, consistent composition model |
| Integration tests in top-level `test/` package | Black-box testing through public API only | ✓ Good — forced daemon accessor exports, cleaner API |
| Both InMemory + HTTP transports in test harness | InMemory for speed, HTTP smoke for serialization path | ✓ Good — catches both protocol and wire bugs |
| Tiered Go fixture + codebase smoke | Small testdata for deterministic assertions, own codebase for scale | ✓ Good — both layers found real bugs |
| Golden file pattern for profile/mode contracts | Independent oracle from production YAML prevents self-approving bad changes | ✓ Good — 19 goldens, diff-reviewable updates |
| Three-tier concurrency (scenarios + fan-out + synctest) | Layered coverage: realistic + targeted + deterministic | ✓ Good — caught SessionInfo data race |
| Three-band error coverage (category + destructive + read-only) | Risk-weighted testing per Google/OWASP guidance | ✓ Good — 30 cases with low duplication |
| Structured IsError oracle (defer typed errors) | Kernel lacks typed errors yet, tracked as TODO(#typed-errors) | ⚠ Revisit — acceptable short-term, needs upgrade path |
| Multi-oracle test architecture | Five oracle layers as separate packages under test/oracle/ | ✓ Good — clean separation of deterministic vs LLM tests |
| Extension-based language detection fallback | Marker-free languages (Markdown) need file extension scanning | ✓ Good — enables any language with registered extensions |
| Multi-provider LLM tests (Anthropic + DeepSeek) | DeepSeek as fallback when Anthropic credits unavailable | ✓ Good — 97.4% pass rate with DeepSeek, proves provider-agnostic tool descriptions |
| LLM tests never block merge | Build-tag gated, informational judge scoring only | ✓ Good — avoids flaky CI from LLM nondeterminism |
| Benchmarks-first ordering | Observing-the-benchmarked-thing taints results; Phase 9 baseline before any obs code | ✓ Good — clean pre-instrumentation baseline |
| Noop-default observability | Metrics/tracing opt-in, zero overhead when disabled | ✓ Good — 0 allocs/op on fast path |
| Tiered benchmark thresholds | PR tier (15%/25%) relaxed for CI noise; release tier (10%/20%) tight | ✓ Good — reduces false positives |
| Dedicated admin listener | Separate from MCP mux; bind failure non-fatal | ✓ Good — clean separation of concerns |
| Decoupled MetricsSink interface | lspool has zero imports of internal/obs | ✓ Good — compile-time assertion enforces |
| Single typed error (ErrCircuitOpen) | Full migration deferred to v1.3+ | ✓ Good — completed in v1.5 with full 7-kind taxonomy |
| Kind-based error taxonomy (internal/errors/) | Typed errors for agent consumption, builder pattern, JSON serialization | ✓ Good — 7 kinds, cause-chain wrapping, all tools migrated |
| Inline validation before workspace check | Fail fast on invalid params, avoid LS startup for bad input | ✓ Good — 24 kernel tools validate at entry |
| extractKind test helper for Kind assertions | Replace brittle string matching with structured error validation | ✓ Good — covers SDK and inline validation formats |
| One phase per bug for BUG-01..BUG-04 (v1.9) | Smaller blast radius, easier rollback, clear ownership per LS quirk | ✓ Good — all 4 bugs closed cleanly |
| Phase 50: bench harness local-only (no CI) | Project's local-only bench rule — hosted-runner baselines are misleading | ✓ Good — `bench.yml`/`capture-baseline.yml` removed; `make bench` documented |
| Phase 51.1 emergent: CGO=0 build path via `//go:build cgo` stubs (v1.9) | DEF-51-01 blocker — goreleaser cross-compile fails when treesitter requires CGO | ~~✓ Good — daemon refuses CGO=0 with remediation; CGO=1 byte-identical~~ — superseded by Phase 59.1 (v1.10): CGO=0 build path removed entirely, single-mode CGO=1 source tree |
| Phase 59.1: drop CGO=0 stubs, single-mode CGO=1 release pipeline (split-runner; FALLBACK-B-MULTI-BUILD-ID per D-20) (v1.10) | INT-BLOCKER-01 from v1.10-MILESTONE-AUDIT.md — stub apparatus added cost without value once goreleaser pinned CGO=0; GoReleaser Pro lock-in surfaced in Wave 2 forced FALLBACK-B (multi-build-id) over the original split-release sketch | ✓ Good — single-mode CGO=1 source tree (12 `_nocgo.go` deletions, 24 `//go:build cgo` strips, daemon step-6a removed); ubuntu-22.04 zig cc for linux+windows + macos-14 Apple clang for darwin; uniform cosign keyless attestation across 6 archives in the release-merge job; per-runner Pass-1≡Pass-2 reproducibility (D-17); REL-01 artifact shape preserved |
| Phase 52 rescope: serena → helix hard-cut + self-upgrade (v1.9) | Original Homebrew/Scoop/Linux scope abandoned in favor of self-contained binary + in-binary upgrade — better leverage for the work, defer distros until release shape proves out | ✓ Good — clean rename, working `helix update`/`helix upgrade` with minisign + atomic swap + downgrade refusal |
| Module path rename via mechanical perl rewrite + go build gate | gopls rename does not operate on module paths; layered build/vet/test catches misses | ✓ Good — clean rename across 200+ files, no functional regressions |
| Build-time embed-copy for `minisign.pub` with CI gate | Embedded pubkey must match checked-in source of truth; placeholder must fail loudly before release | ✓ Good — `make embed-pubkey` + `make verify-embed-pubkey` + `release.yml` PLACEHOLDER pre-flight |
| OBS-03 (metrics) before OBS-01/02 (dashboards/runbooks) (v1.9) | Dashboards must reference metrics that exist | ✓ Good — registry-driven PromQL validator (fail-closed) catches drift |
| Registry-driven PromQL validator for dashboards/runbooks | Compile-time-equivalent check that every PromQL expression references a real registered metric | ✓ Good — fail-closed, runs in `go test` |
| Phase 56 emergent: wire LS notification dispatch (v1.9) | Phase 55 trace audit surfaced that `jsonrpc.Conn.OnNotification` was never set in production — all `QuirkAdapter.NotificationHandlers()` were silently dropped | ✓ Good — fix shipped with regression assertion in `worker_test.go`; `JdtlsAdapter.WaitUntilJavaReady(ctx)` deterministic gate |
| PKG-01 SC-3 acceptance as deployment-gated, not engineering-gated (v1.9) | End-to-end signed-release verify requires real maintainer keypair + first v* tag — not a code change | ✓ Good — CI pre-flight grep-fails on PLACEHOLDER, so a placeholder-signed release cannot publish |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone:**
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-06-21 — Phase 84 (container runtime + cosign-signed GHCR mirror + disk-budget guard; CONTAINER-01..04) complete and verified; v1.12 milestone underway (10/15 phases).*
