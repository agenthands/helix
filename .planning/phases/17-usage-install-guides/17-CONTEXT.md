# Phase 17: Usage & Install Guides - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Update USAGE.md with production feature documentation (observability accuracy review, graceful degradation tuning, benchmark workflow) and overhaul the install guide from legacy Python to Go binary with multi-agent setup instructions. No new features — documentation only.

</domain>

<decisions>
## Implementation Decisions

### Install Guide
- **D-01:** Rename `llms-install.md` to `INSTALL.md` — universally understood, standard naming
- **D-02:** Structure as shared prerequisites at top (install Go binary, verify PATH) then per-agent MCP config sections for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, and Antigravity
- **D-03:** Human-first audience — standard markdown doc a developer reads, not machine-optimized numbered steps
- **D-04:** Complete rewrite — current content is Python/uv-based, entirely stale for Go binary

### USAGE Benchmark Section
- **D-05:** Cover running benchmarks locally and interpreting results only — no CI gate details (that's CONTRIBUTING.md territory)
- **D-06:** Include: how to run `go test -bench`, read p50/p95/p99 output, compare with benchstat
- **D-07:** Place benchmark content as a subsection under Observability Quickstart (framed as measuring performance, groups monitoring-adjacent content)

### USAGE Observability
- **D-08:** Current Observability Quickstart depth is sufficient — review for accuracy against v1.2 code and fill any gaps, but no expansion to Grafana dashboards, alerting rules, or deployment patterns
- **D-09:** Review existing degradation config reference for completeness against actual code defaults

### USAGE Structure
- **D-10:** No major structural changes — new benchmark subsection goes under Observability Quickstart, rest stays as-is

### Claude's Discretion
- Exact wording and ordering of benchmark subsection content
- Per-agent config JSON format and any agent-specific notes in INSTALL.md
- Whether to add cross-references between USAGE.md and INSTALL.md

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Update Targets
- `USAGE.md` — Current usage guide (631 lines), has Observability Quickstart and Performance Tuning sections
- `llms-install.md` — Legacy Python install guide (29 lines), to be replaced by INSTALL.md

### Source of Truth (for accuracy verification)
- `internal/obs/` — Observability package, verify config keys and defaults match USAGE docs
- `internal/kernel/lspool/` — Worker pool, circuit breaker, verify degradation defaults
- `internal/daemon/daemon.go` — Daemon bootstrap, verify config wiring
- `internal/config/` — 4-layer config system, verify documented keys match actual koanf bindings
- `test/bench/` — Benchmark harness, source for benchmark workflow docs

### Prior Phase Context
- `.planning/phases/16-core-documentation-update/16-CONTEXT.md` — Phase 16 decisions, especially D-03 (admin endpoints detail belongs in USAGE)

### Existing Documentation (cross-reference)
- `CONTRIBUTING.md` — Updated in Phase 16, has CI benchmark gate details (USAGE should not duplicate)
- `README.md` — Updated in Phase 16, has Production & Observability overview (USAGE provides depth)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- USAGE.md already has well-structured Observability Quickstart with metric tables, scrape configs, and pprof examples
- Performance Tuning section has worker pool sizing and timeout budget tables
- Config Reference has complete key tables for observability and degradation settings

### Established Patterns
- Config examples use YAML format with comments
- Tool class timeout tables with default values and tuning guidance
- Prometheus metric tables with type and description columns

### Integration Points
- `internal/obs/provider.go` — observability provider initialization (verify config key names)
- `internal/kernel/lspool/pool.go` — worker pool config (verify defaults in docs)
- `test/bench/` — benchmark test files (source for run commands and output format)
- `cmd/serena/` — CLI flags (verify flag names match docs)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches within the decisions above.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 17-usage-install-guides*
*Context gathered: 2026-04-11*
