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

## Milestone: v1.4 — Integration Testing v2

**Shipped:** 2026-04-14
**Phases:** 4 | **Plans:** 11

### What Was Built
- Extracted importable test harness (`test/harness/`) with Runner, golden store, fixture helpers, build tag taxonomy
- Protocol oracle tests (MCP handshake, tools/list, session isolation, reconnect)
- Contract oracle tests (JSON Schema validation, 23 golden outputs, error contracts, selectability heuristics)
- Scenario oracle matrix (14+ full-cycle agent workflows across Go, Python, TypeScript, C++, Swift, Zig, JavaScript, PHP, SQL, Markdown, polyglot, unsupported, collision, degraded)
- LLM behavioral tests with multi-provider support (Anthropic + DeepSeek) and judge scoring infrastructure

### What Worked
- Multi-oracle architecture separated deterministic from LLM tests cleanly
- Extension-based language detection fallback enabled marker-free languages like Markdown
- DeepSeek fallback achieved 97.4% pass rate, proving provider-agnostic tool descriptions
- LLM tests gated behind build tags, never blocking merge

### What Was Inefficient
- Quick tasks (C++/Swift/Zig/JS fixtures) were done separately from Phase 20 scenario work — could have been bundled
- REQUIREMENTS.md checkboxes drifted unchecked despite satisfied requirements

### Patterns Established
- Five oracle layers as separate packages under test/oracle/
- Marksman quirk adapter pattern for language-specific LS behavioral hooks

### Key Lessons
- Provider-agnostic tool descriptions pay off — multi-LLM test coverage validates tool design quality
- Quick task workflow is effective for fixture additions that don't warrant full phase planning

## Milestone: v1.5 — Typed Errors & Hardening

**Shipped:** 2026-04-15
**Phases:** 3 | **Plans:** 12

### What Was Built
- `internal/errors/` package with 7 error kinds, builder pattern, JSON serialization, cause-chain wrapping
- Full migration of all 38+ MCP tools from raw `fmt.Errorf` to typed `serr.New`/`serr.Wrap` across 10 packages
- Inline input validation at all 24 kernel tool handlers (empty-string checks before workspace/LS work)
- Kind-level test assertions with `extractKind` helper and 4 typed golden files

### What Worked
- Clean 3-phase decomposition: taxonomy → migration → testing was a natural dependency chain
- Builder pattern for errors (`.WithTool()`, `.WithDetail()`) made migration mechanical across packages
- Research phase identified the right error kind set upfront — no mid-milestone taxonomy changes
- 8-plan parallel migration (Phase 23) was effective — each package independent

### What Was Inefficient
- REQUIREMENTS.md checkboxes never checked despite all requirements being satisfied (tracking gap, caught by audit)
- Summary one-liner extraction from gsd-tools produced garbled output (tool parsing bug)
- 3 golden files deferred because error conditions can't be triggered deterministically without live LS

### Patterns Established
- Inline validation before workspace check — fail fast on invalid params, avoid LS startup for bad input
- `extractKind` helper for test assertions — handles both SDK validation and inline validation error formats
- Detail string flattening for complex metadata (CircuitOpen language/failures/backoff → single string)

### Key Lessons
- Error taxonomy design should happen once, upfront — the 7-kind set proved sufficient for all 38+ tools
- Mechanical migrations (same pattern across many files) benefit from high plan parallelism
- Golden file coverage is limited by testability — some error paths require live infrastructure

### Cost Observations
- 12 plans completed in ~1 day
- Highly parallel Phase 23 (8 plans) was the bulk of the work
- Sessions: ~3

## Cross-Milestone Trends

| Metric | v1.0 | v1.1 | v1.2 | v1.3 | v1.4 | v1.5 |
|--------|------|------|------|------|------|------|
| Phases | 5 | 3 | 7 | 2 | 4 | 3 |
| Plans | 20 | 11 | 25 | 4 | 11 | 12 |
| Duration | 2 days | 1 day | 2 days | 1 day | 4 days | 1 day |
| Go LOC (cumulative) | ~26K | ~30K | ~36K | ~36K | ~41K | ~48K |
| Test coverage focus | Unit | Integration | Benchmark + E2E | Documentation | Oracle tests | Error contracts |
