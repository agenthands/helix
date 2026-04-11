# Phase 17: Usage & Install Guides - Research

**Researched:** 2026-04-11
**Domain:** Documentation (USAGE.md accuracy review + INSTALL.md rewrite)
**Confidence:** HIGH

## Summary

Phase 17 is a documentation-only phase with two deliverables: (1) review and update USAGE.md for accuracy against v1.2 code, adding a benchmark subsection under Observability Quickstart, and (2) replace the stale Python-based `llms-install.md` with a new `INSTALL.md` covering Go binary setup for six coding agents.

The USAGE.md is already well-structured at 631 lines with Observability Quickstart, Performance Tuning, and Config Reference sections. The primary work is an accuracy audit against actual code (several gaps found -- see Pitfalls section) and adding the benchmark workflow subsection. The INSTALL.md is a complete rewrite -- the current 29-line `llms-install.md` documents the Python/uv workflow and is entirely stale.

**Primary recommendation:** Audit USAGE.md config keys, metric names, and defaults against `internal/config/config.go` and `internal/obs/metrics.go` (verified gaps exist), add benchmark subsection referencing `test/bench/` harness, then write INSTALL.md with shared prerequisites + per-agent MCP config blocks.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Rename `llms-install.md` to `INSTALL.md` -- universally understood, standard naming
- **D-02:** Structure as shared prerequisites at top (install Go binary, verify PATH) then per-agent MCP config sections for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, and Antigravity
- **D-03:** Human-first audience -- standard markdown doc a developer reads, not machine-optimized numbered steps
- **D-04:** Complete rewrite -- current content is Python/uv-based, entirely stale for Go binary
- **D-05:** Cover running benchmarks locally and interpreting results only -- no CI gate details (that's CONTRIBUTING.md territory)
- **D-06:** Include: how to run `go test -bench`, read p50/p95/p99 output, compare with benchstat
- **D-07:** Place benchmark content as a subsection under Observability Quickstart (framed as measuring performance, groups monitoring-adjacent content)
- **D-08:** Current Observability Quickstart depth is sufficient -- review for accuracy against v1.2 code and fill any gaps, but no expansion to Grafana dashboards, alerting rules, or deployment patterns
- **D-09:** Review existing degradation config reference for completeness against actual code defaults
- **D-10:** No major structural changes -- new benchmark subsection goes under Observability Quickstart, rest stays as-is

### Claude's Discretion
- Exact wording and ordering of benchmark subsection content
- Per-agent config JSON format and any agent-specific notes in INSTALL.md
- Whether to add cross-references between USAGE.md and INSTALL.md

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| USAGE-01 | USAGE documents observability configuration (Prometheus, OTLP, admin listener) | Existing Observability Quickstart covers this; accuracy gaps found (missing `service_name` config key, missing 2 metric names). Review and fill gaps. |
| USAGE-02 | USAGE documents graceful degradation settings (budgets, GOMEMLIMIT, circuit breaker) | Existing Performance Tuning + Config Reference covers this; defaults verified against `internal/config/config.go`. Appears complete. |
| USAGE-03 | USAGE documents performance tuning and benchmark workflow | Performance Tuning section exists; benchmark subsection is NEW content. Research provides bench harness details and benchstat workflow. |
| INST-01 | Rename llms-install.md to agent-focused install guide | D-01 locks rename to INSTALL.md. Current file is 29 lines of stale Python/uv instructions. |
| INST-02 | Install guide covers Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity setup | MCP config formats for all 6 agents researched. See Architecture Patterns section for per-agent config details. |
</phase_requirements>

## Standard Stack

Not applicable -- this is a documentation-only phase. No libraries or packages are installed.

**Tools referenced in documentation:**
| Tool | Version | Purpose | Verification |
|------|---------|---------|-------------|
| `go` | 1.25+ | Build serena binary | [VERIFIED: local `go version` = 1.25.1] |
| `benchstat` | latest | Compare benchmark runs | [VERIFIED: referenced in `test/bench/baselines/README.md` and CONTRIBUTING.md] |
| `gopls` | latest | Required for Go benchmarks | [VERIFIED: `requireGoplsB(b)` guard in `tools_bench_test.go`] |

## Architecture Patterns

### USAGE.md Accuracy Gaps Found

These are verified discrepancies between current USAGE.md content and actual code.

#### Gap 1: Missing `service_name` config key
**Source:** `internal/config/config.go` line 54: `ServiceName string \`koanf:"service_name"\``
**USAGE.md:** The Observability Settings table (line 266-270) documents `admin_addr`, `enable_pprof`, `tracing_endpoint`, `tracing_sample_ratio` but NOT `service_name`. [VERIFIED: grep of USAGE.md confirms absence]
**Fix:** Add `observability.service_name` (string, default "serena") to the config key table and the example YAML.

#### Gap 2: Missing metrics in Prometheus table
**Source:** `internal/obs/metrics.go` registers 6 metric vectors.
**USAGE.md:** The Prometheus Metrics table (lines 511-516) lists only 4 metrics. Missing:
- `serena_tool_calls_total` (counter) -- MCP tool calls by outcome [VERIFIED: metrics.go line 57-61]
- `serena_lspool_restarts_total` (counter) -- LS worker restarts per language [VERIFIED: metrics.go line 96-99]
**Fix:** Add both metrics to the table. Also note label dimensions (the code uses 5 labels for tool metrics: tool_name, profile, mode, language, outcome).

#### Gap 3: Metrics label detail
**Source:** `internal/obs/metrics.go` line 28: `AllowedLabels = [5]string{"tool_name", "profile", "mode", "language", "outcome"}`
**USAGE.md:** Does not document metric label dimensions.
**Fix:** Add a brief note about available labels after the metrics table so users can build useful Prometheus queries (e.g., `rate(serena_tool_calls_total{outcome="error"}[5m])`).

#### Gap 4: Eviction metric has `reason` label
**Source:** `internal/obs/metrics.go` line 85: `[]string{"language", "reason"}` with closed enum {idle, pressure, crash, shutdown}
**USAGE.md:** Lists `serena_lspool_evictions_total` but does not mention the `reason` label.
**Fix:** Add label info to the metric description.

#### Degradation Config: Verified Complete
All 7 DegradationConfig fields in `internal/config/config.go` (lines 28-35) match the USAGE.md Degradation Settings table (lines 276-283). Defaults documented correctly. [VERIFIED: field-by-field comparison]

#### Worker Pool Config: Verified Complete
All 5 WorkerPoolConfig fields (lines 60-71) match the USAGE.md Worker Pool Settings table (lines 249-255). [VERIFIED: field-by-field comparison]

### Benchmark Subsection Content

Based on verified analysis of `test/bench/`:

**Directory structure:**
```
test/bench/
  baselines/          # Committed benchmark baselines (CI-captured)
  cmd/benchgate/      # Go-native regression gate tool
  tools_bench_test.go # 38-tool response-time benchmark (BENCH-02)
  lsp_index_bench_test.go  # LSP indexing throughput
  memory_bench_test.go     # Memory profiling benchmarks
  obs_bench_test.go        # Observability overhead benchmarks
  tracing_bench_test.go    # Tracing overhead benchmarks
  metrics_bench_test.go    # Metrics overhead benchmarks
  heap_snapshot_test.go    # Heap snapshot benchmarks
  fullrepo_smoke_test.go   # Full-repo smoke (skipped with -short)
```
[VERIFIED: `ls test/bench/`]

**Local benchmark workflow (for USAGE.md):**
1. Run: `go test -bench=. ./test/bench/ -timeout 300s`
2. Specific benchmark: `go test -bench=BenchmarkTools ./test/bench/ -timeout 300s`
3. With memory stats: `go test -bench=. -benchmem ./test/bench/ -timeout 300s`
4. Save output: redirect to file for benchstat comparison
5. Compare runs: `benchstat old.txt new.txt`
6. Install benchstat: `go install golang.org/x/perf/cmd/benchstat@latest`

**Key facts for documentation:**
- Benchmarks require `gopls` installed (guard: `requireGoplsB(b)`) [VERIFIED: tools_bench_test.go]
- Uses `testing.B.Loop` (Go 1.24+, available in project's Go 1.25+) [VERIFIED: CONTRIBUTING.md line 106]
- `-short` flag skips `BenchmarkFullRepoSmoke` [VERIFIED: baselines/README.md line 48]
- `-count=N` for statistical significance (CI uses 10) [VERIFIED: baselines/README.md line 67]
- `GOMAXPROCS=4` reduces noise on shared runners [VERIFIED: baselines/README.md line 76]

**What NOT to include (per D-05):**
- CI gate details (thresholds, benchgate, baseline capture) -- already in CONTRIBUTING.md
- Re-baseline procedure -- already in `test/bench/baselines/README.md`

### Per-Agent MCP Config Formats (for INSTALL.md)

#### Claude Code
**Config file:** `.claude/settings.json` [VERIFIED: README.md line 204]
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```

#### Codex
**Config file:** `.codex/config.json` [VERIFIED: README.md line 217]
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio", "--profile=codex"]
    }
  }
}
```

#### OpenCode
**Config file:** `opencode.json` (project root) or `~/.config/opencode/opencode.json` (global) [CITED: opencode.ai/docs/mcp-servers/]
```json
{
  "mcp": {
    "serena": {
      "type": "local",
      "command": ["serena", "--mode=stdio"],
      "enabled": true
    }
  }
}
```
**Note:** OpenCode uses `"mcp"` not `"mcpServers"`, and `command` is an array, not a string. [CITED: opencode.ai/docs/mcp-servers/]

#### Cursor
**Config file:** `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global) [CITED: cursor.com/docs/context/mcp]
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```
**Note:** Use absolute paths if `serena` is not in PATH. [CITED: cursor.com/docs/context/mcp]

#### Gemini CLI
**Config file:** `~/.gemini/settings.json` (global) or `.gemini/settings.json` (project) [CITED: geminicli.com/docs/tools/mcp-server/]
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```
**Note:** Gemini CLI supports `env`, `cwd`, `timeout`, and `trust` options. Environment variable expansion is supported in `env` block. [CITED: geminicli.com/docs/tools/mcp-server/]

#### Antigravity
**Config file:** `mcp_config.json` (accessible via Agent Panel > MCP Servers > Manage > Edit configuration) [CITED: antigravity.codes/blog/antigravity-mcp-tutorial]
```json
{
  "mcpServers": {
    "serena": {
      "command": "serena",
      "args": ["--mode=stdio"]
    }
  }
}
```
**Note:** Use absolute paths. Accessible via Antigravity UI: Agent Panel > "..." > MCP Servers. [CITED: antigravity.codes/blog/antigravity-mcp-tutorial]

### INSTALL.md Structure

Per D-02 and D-03:

```
# Install Guide

## Prerequisites
  - Go 1.25+ (or download pre-built binary)
  - go install command
  - Verify: serena --version / serena --help

## Agent Setup
  ### Claude Code
  ### Codex
  ### OpenCode
  ### Cursor
  ### Gemini CLI
  ### Antigravity
  ### HTTP Mode (generic)

## Verify Installation
  - Connect and run onboard_project
  - Expected output

## Next Steps
  - Link to USAGE.md for configuration
  - Link to README.md for feature overview
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Benchmark comparison | Manual eyeballing of ns/op | `benchstat old.txt new.txt` | Statistical significance, confidence intervals |
| Metric label cardinality | Free-form metric labels | AllowedLabels bounded array | Unbounded labels cause Prometheus cardinality explosion |

## Common Pitfalls

### Pitfall 1: Documenting config keys that don't exist in code
**What goes wrong:** USAGE.md documents a config key that was planned but never implemented, or uses a different name than the koanf tag.
**Why it happens:** Documentation written from plan documents rather than verified against code.
**How to avoid:** Every config key in USAGE.md must be verified against the `koanf:` struct tag in `internal/config/config.go`.
**Warning signs:** Config examples that use keys not found in the config structs.

### Pitfall 2: Agent config format differences
**What goes wrong:** Using the Claude Code JSON format for all agents, when some agents (OpenCode) use different key names.
**Why it happens:** Assuming all MCP clients use the same `mcpServers` key.
**How to avoid:** OpenCode uses `"mcp"` with `"type": "local"` and array `"command"`. Every other agent uses `"mcpServers"` with string `"command"` + `"args"` array. Verify each format against official docs.
**Warning signs:** Config snippets that look identical across all agents.

### Pitfall 3: Metric table drift from code
**What goes wrong:** Prometheus metric table in USAGE.md lists metrics that were renamed or have different labels than the code.
**Why it happens:** Metrics added in Phase 11-12, docs written in Phase 14, metrics may have been adjusted since.
**How to avoid:** Verify every metric name and label set against `internal/obs/metrics.go` `newMetrics()` function.
**Warning signs:** Metric names in docs that don't match `grep -r "Name:" internal/obs/metrics.go`.

### Pitfall 4: Stale README cross-references after rename
**What goes wrong:** After renaming `llms-install.md` to `INSTALL.md`, internal cross-references in README.md or other docs still point to the old filename.
**Why it happens:** Rename without grep for all references.
**How to avoid:** `grep -r "llms-install" .` after rename to find all stale references.
**Warning signs:** Broken links in markdown.

### Pitfall 5: Benchmark section duplicating CONTRIBUTING.md
**What goes wrong:** USAGE.md benchmark section covers CI gate thresholds, benchgate tool, baseline capture -- content that already lives in CONTRIBUTING.md.
**Why it happens:** Not checking existing docs for overlap.
**How to avoid:** D-05 is explicit: cover running locally and interpreting results ONLY. Link to CONTRIBUTING.md for CI details.
**Warning signs:** Any mention of `benchgate`, `--warn-only`, baseline capture, PR thresholds in USAGE.md.

## Code Examples

### Running benchmarks locally (for USAGE.md benchmark subsection)
```bash
# Source: test/bench/baselines/README.md + CONTRIBUTING.md

# Run all benchmarks
go test -bench=. ./test/bench/ -timeout 300s

# Run specific benchmark suite
go test -bench=BenchmarkTools ./test/bench/ -timeout 300s

# With memory allocation stats
go test -bench=. -benchmem ./test/bench/ -timeout 300s

# Multiple runs for statistical comparison (recommended: 5-10)
go test -bench=. -benchmem -count=5 ./test/bench/ -timeout 300s > bench-before.txt

# ... make changes ...
go test -bench=. -benchmem -count=5 ./test/bench/ -timeout 300s > bench-after.txt

# Compare with benchstat
go install golang.org/x/perf/cmd/benchstat@latest
benchstat bench-before.txt bench-after.txt
```
[VERIFIED: commands derived from test/bench/baselines/README.md and CONTRIBUTING.md]

### Example benchstat output interpretation
```
goos: darwin
goarch: arm64
pkg: github.com/postfix/serena/test/bench
                          | bench-before.txt | bench-after.txt          |
                          |     sec/op       |   sec/op    vs base      |
Tools/go_to_definition-10    2.145m +/- 5%     2.098m +/- 3%  ~ (p=0.151 n=5)
Tools/find_references-10     3.267m +/- 8%     3.189m +/- 6%  ~ (p=0.222 n=5)
```
[ASSUMED: output format based on standard benchstat output]

### Prometheus query examples (for USAGE.md)
```promql
# Source: internal/obs/metrics.go label definitions

# Error rate per tool (last 5 minutes)
rate(serena_tool_calls_total{outcome="error"}[5m])

# p95 tool latency
histogram_quantile(0.95, rate(serena_tool_duration_seconds_bucket[5m]))

# Active workers by language
serena_lspool_workers

# Eviction rate by reason
rate(serena_lspool_evictions_total[5m])
```
[VERIFIED: metric names and labels from internal/obs/metrics.go]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Python/uv install (`llms-install.md`) | `go install` single binary (`INSTALL.md`) | v1.2 (Go rewrite) | Complete rewrite needed |
| Manual benchmark eyeballing | `benchstat` statistical comparison | Go ecosystem standard | USAGE should reference benchstat |
| Classic `for i := 0; i < b.N; i++` | `testing.B.Loop` (Go 1.24+) | Go 1.24 | Document that benchmarks use B.Loop |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | benchstat output format shown in code example is accurate | Code Examples | LOW -- standard Go tool output, easily verified by running |
| A2 | Antigravity uses `mcp_config.json` file path | Architecture Patterns | MEDIUM -- based on web tutorial, may vary by version. Implementer should verify against latest Antigravity docs |
| A3 | OpenCode `command` field is an array not string | Architecture Patterns | HIGH if wrong -- would produce broken config examples. Source is official OpenCode docs page |

## Open Questions

1. **Antigravity config path stability**
   - What we know: Tutorial shows `mcp_config.json` accessible via UI [CITED: antigravity.codes/blog/antigravity-mcp-tutorial]
   - What's unclear: Whether there's also a file-system path users can edit directly, and whether the format has changed in v1.20.5
   - Recommendation: Document the UI path as primary, note that file location may vary by platform

2. **Should INSTALL.md recommend `--profile` per agent?**
   - What we know: README already shows `--profile=codex` for Codex, `--profile=ide-assistant` for IDE
   - What's unclear: Whether the install guide should recommend specific profiles or let users discover via USAGE.md
   - Recommendation: Include `--profile` in agent configs where a non-default profile is recommended (Codex, IDE-assistant), omit for agents where `full` default is fine

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (go 1.25.1) |
| Config file | N/A -- documentation phase |
| Quick run command | `go vet ./... && go test ./...` |
| Full suite command | `go test ./... -timeout 120s` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| USAGE-01 | Observability config documented accurately | manual-only | N/A -- verify config keys match `internal/config/config.go` | N/A |
| USAGE-02 | Degradation settings documented accurately | manual-only | N/A -- verify against `internal/config/config.go` | N/A |
| USAGE-03 | Benchmark workflow documented | manual-only | `go test -bench=BenchmarkTools ./test/bench/ -timeout 300s -short` (verify commands work) | N/A |
| INST-01 | llms-install.md renamed to INSTALL.md | smoke | `test -f INSTALL.md && ! test -f llms-install.md` | N/A |
| INST-02 | Install guide covers 6 agents | manual-only | `grep -c "### " INSTALL.md` (verify 6+ agent sections) | N/A |

**Justification for manual-only:** This is a documentation phase. Content accuracy is verified by code review against source files, not by automated tests. The quick run command validates that existing tests still pass (no code changes should break them).

### Sampling Rate
- **Per task commit:** `go vet ./... && go test -short ./...`
- **Per wave merge:** Full test suite
- **Phase gate:** Verify USAGE.md config keys match code, verify INSTALL.md agent configs match official docs

### Wave 0 Gaps
None -- documentation phase requires no test infrastructure.

## Security Domain

Not applicable -- documentation-only phase. No code changes, no new endpoints, no input handling.

## Sources

### Primary (HIGH confidence)
- `internal/config/config.go` -- all config struct definitions and koanf tags
- `internal/obs/metrics.go` -- all Prometheus metric definitions and labels
- `internal/obs/tracing.go` -- TracingConfig struct and OTLP setup
- `test/bench/tools_bench_test.go` -- benchmark harness structure and patterns
- `test/bench/baselines/README.md` -- baseline capture procedure and benchstat workflow
- `CONTRIBUTING.md` -- existing benchmark documentation (lines 88-110)
- `USAGE.md` -- existing content to be updated (631 lines)
- `llms-install.md` -- current stale install guide (29 lines)

### Secondary (MEDIUM confidence)
- [opencode.ai/docs/mcp-servers/](https://opencode.ai/docs/mcp-servers/) -- OpenCode MCP config format
- [cursor.com/docs/context/mcp](https://cursor.com/docs/context/mcp) -- Cursor MCP config format
- [geminicli.com/docs/tools/mcp-server/](https://geminicli.com/docs/tools/mcp-server/) -- Gemini CLI MCP config format
- [antigravity.codes/blog/antigravity-mcp-tutorial](https://antigravity.codes/blog/antigravity-mcp-tutorial) -- Antigravity MCP config format

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- USAGE.md accuracy gaps: HIGH -- verified by reading actual source code
- Benchmark workflow: HIGH -- verified against test/bench/ directory and existing docs
- Agent MCP configs (Claude Code, Codex): HIGH -- verified in current README.md
- Agent MCP configs (OpenCode, Cursor, Gemini CLI): MEDIUM -- from official documentation sites
- Agent MCP configs (Antigravity): MEDIUM -- from tutorial site, not verified locally

**Research date:** 2026-04-11
**Valid until:** 2026-05-11 (stable -- documentation against existing code)
