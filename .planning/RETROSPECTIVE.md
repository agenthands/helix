# Project Retrospective

## Milestone: v1.2 — Performance & Production Hardening

**Shipped:** 2026-04-10
**Phases:** 7 | **Plans:** 25 | **Commits:** 134

### What Was Built
- Benchmark harness with testing.B.Loop, 38-tool benchmarks, CI benchstat regression gate
- Observability foundation: stdlib-only internal/obs/, zero-alloc slog handler, admin listener
- Prometheus /metrics with RED histograms per tool, lspool gauges, bounded-label contract
- End-to-end tracing: otelgrpc propagation, telemetry middleware, per-tool sub-spans, OTLP exporter
- Graceful degradation: per-class timeout budgets, deadline propagation, typed ErrCircuitOpen, GOMEMLIMIT
- Documentation: README.md, USAGE.md, CHANGELOG.md with auto-generated tool/language tables
- Benchmark gate hardening: capture-baseline.yml workflow, blocking benchgate mode

### What Worked
- Benchmarks-first ordering constraint prevented observing-the-benchmarked-thing contamination
- Noop-default provider pattern kept hot-path at zero allocs until metrics/tracing explicitly enabled
- Decoupled MetricsSink interface kept lspool independent of internal/obs with compile-time assertion
- Phase-level benchstat delta gates caught allocation regressions early
- Auto-generated doc tables from registry prevent drift between code and documentation

### What Was Inefficient
- gopls v0.17.1 pinning caused CI build failures on Go 1.25 linux/amd64 — took multiple iterations to diagnose (fork workflow caching compounded the issue)
- Baseline capture required local fallback (darwin/arm64) instead of CI ubuntu-latest — cross-platform comparison is imprecise
- Some SUMMARY.md one-liners captured deviation notes instead of accomplishments, making automated extraction noisy

### Patterns Established
- Tiered benchmark thresholds: PR tier (15%/25%) relaxed for CI noise, release tier (10%/20%) strict
- Admin listener on dedicated loopback port, separate from MCP mux, bind failure non-fatal
- Telemetry middleware runs before profile filter so denied calls are observable
- Kernel spans set no attributes — middleware owns all labels (single responsibility)
- Shutdown ordering: kernel first, then trace flush (separate 5s context), then listener close

### Key Lessons
- Pin dependencies to versions compatible with your Go version before creating CI workflows
- Fork repos may serve stale workflow files from upstream — verify with API before debugging further
- Operational gaps (needing to trigger a workflow) are different from code gaps — don't create code phases for operational tasks

## Milestone: v1.3 — Documentation Catchup

**Shipped:** 2026-04-11
**Phases:** 2 | **Plans:** 4 | **Commits:** 25

### What Was Built
- README.md updated with Production & Observability section, corrected tool count from stale "38+" to accurate "35+"
- CONTRIBUTING.md rewritten as Go-native contributor guide (126 lines, covers dev commands, integration tests, benchmarks, tool/language addition)
- CHANGELOG.md v1.2 entry completed with Phase 15 Benchmark Gate Hardening gap fill
- USAGE.md accuracy gaps fixed: added service_name config key, 2 missing metrics, metric label documentation with PromQL examples, benchmarks subsection
- INSTALL.md created from scratch with per-agent MCP config for 6 coding agents (Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity) + HTTP mode
- Stale Python-based llms-install.md deleted

### What Worked
- Parallel executor agents for independent documentation plans (USAGE.md and INSTALL.md modified different files)
- Research phase caught 4 verified accuracy gaps by reading actual source code before planning
- Explicit "do NOT include" constraints (D-05: no CI gate details in USAGE benchmark section) prevented content duplication with CONTRIBUTING.md
- Per-agent config format research (especially OpenCode's different `"mcp"` key) caught a pitfall that would have produced broken examples

### What Was Inefficient
- Phase 16 summaries lacked `requirements-completed` frontmatter, requiring manual cross-referencing during milestone audit
- Integration checker found README.md has no links to INSTALL.md, USAGE.md, or CONTRIBUTING.md — this cross-reference gap should have been caught during planning
- HTTP mode flag style inconsistency across docs (--serve vs --mode=http) — not caught until integration check

### Patterns Established
- Documentation accuracy audits should always verify against source code, not prior documentation
- Per-agent MCP config sections should note format differences explicitly (OpenCode vs others)
- USAGE.md benchmark content covers local workflow only; CI gate details belong in CONTRIBUTING.md

### Key Lessons
- Documentation phases benefit from research that reads actual source code — found gaps that planning-only approaches would miss
- Cross-document link graphs should be verified as part of planning, not discovered during milestone audit
- Tool count claims drift over time — auto-generated tables are the source of truth

## Cross-Milestone Trends

| Metric | v1.0 | v1.1 | v1.2 | v1.3 |
|--------|------|------|------|------|
| Phases | 5 | 3 | 7 | 2 |
| Plans | 20 | 11 | 25 | 4 |
| Duration | 2 days | 1 day | 2 days | 1 day |
| Go LOC (cumulative) | ~26K | ~30K | ~36K | ~36K |
| Test coverage focus | Unit | Integration | Benchmark + E2E | Documentation |
