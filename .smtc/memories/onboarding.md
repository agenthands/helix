---
slug: onboarding
created_at: 2026-06-17T16:04:11Z
modified_at: 2026-06-17T16:04:11Z
---
# SMTC Onboarding — Helix

**Project root:** /home/john/go/src/github.com/agenthands/helix
**Onboarded:** 2026-06-17
**Analysis:** structural depth, cached (built 2026-06-17T15:10Z), 1468 files / 719 modules, not stale. `.smtc-cache/` ≈17M. Semantic/LSP families (gopls) build on demand per file.

## Primary language & fidelity (this environment)
- **Go = the active product** (single CGO=1 binary). first_class tier, has query pack.
- Also first_class here: Rust, Java, TypeScript, JavaScript, Python. best_effort (structural only): C, C++, PHP, Kotlin.
- `legacy/` = read-only **Python** Serena reference (NOT active). `testdata/` = multi-language fixtures.
- CAVEAT: the cached analysis is **structural-depth**; cross-file references/types use structural resolution until the LSP semantic family is built for a file. Weight results by each finding's confidence enum.

## Capabilities activated
- **architecture** — substrate-based: dependency_path, layer_check, blast_radius, change_impact, module_health, cycle_explain, who_depends_on.
- **security** — Go catalog loaded (271 sinks / 82 sources). Justified: Helix ingests untrusted input — agent-supplied file paths (guarded by the V5 path-traversal validators), MCP tool-call args, and the `internal/guardrails/` enforcement layer.
- NOT activated: **protocol** (available + relevant — Helix is lifecycle-heavy: lspool acquire/release, daemon lifecycle, circuit breakers; activate for guard/lifecycle audits) and **framework** (no catalogs registered).

## Top hotspots — CAVEAT: legacy-Python-dominated, NOT the Go product
`module_health` top-10 (all in dependency cycles, 0 public entry points) is skewed by the read-only `legacy/` Serena tree:
- `logging` (fan_in 307, hotspot 0.94) = `legacy/src/serena/util/logging.py`
- `ls`, `advanced_features`, `al_language_server`, `vue_language_server`, `ls_process`, `nixd_ls`, `marksman`, `bash_language_server` = legacy Serena/solidlsp Python LS wrappers
- `cli` (hotspot 0.76)
**For active Go orientation, ignore these.** PageRank starting points under `internal/`: `internal/semantic/live/lspqueue/queue.go`, `internal/guardrails/enforcement.go`. To find true Go hotspots, scope analysis to `internal/`, `cmd/`, `bench/` and exclude `legacy/`+`testdata/`.

## Layer rules
- `.smtc-rules.yaml`: **ABSENT** — no custom layer-boundary rules; `layer_check` uses defaults.

## Routing
Use the `smtc` skill for tool routing; prefer `mcp__smtc__*` over grep for Go semantic questions. Delegate heavy analysis to: smtc-explorer (navigation/reachability), smtc-architecture-reviewer (cycles/blast radius/layers), smtc-security-auditor (taint source→sink), smtc-impact-analyzer (pre-refactor blast radius). Admin mode active; 96 tools exposed.

## Stale-doc flag
This repo's CLAUDE.md SMTC section claims "no security capability ships for Go" — that is now FALSE (Go catalog: 271 sinks/82 sources). Treat Go security tools as available.