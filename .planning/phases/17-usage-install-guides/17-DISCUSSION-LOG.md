# Phase 17: Usage & Install Guides - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 17-usage-install-guides
**Areas discussed:** Install guide identity, USAGE benchmark section, USAGE observability depth, USAGE structure changes

---

## Install Guide Identity

### Q1: What should llms-install.md be renamed to?

| Option | Description | Selected |
|--------|-------------|----------|
| AGENTS.md | Short, clear. Matches agent-centric framing. | |
| INSTALL.md | Generic but universally understood. | ✓ |
| agent-setup.md | Descriptive, lowercase-kebab style. | |

**User's choice:** INSTALL.md
**Notes:** Standard naming convention preferred.

### Q2: How deep should each agent's setup section be?

| Option | Description | Selected |
|--------|-------------|----------|
| Config JSON + one-liner | Minimal, scannable, copy-pasteable. | |
| Full walkthrough per agent | Step-by-step install through verify. | |
| Shared prereqs + per-agent config | Common steps once, then per-agent MCP config. | ✓ |

**User's choice:** Shared prereqs + per-agent config
**Notes:** Balance of depth and DRY.

### Q3: Written for humans or AI agents?

| Option | Description | Selected |
|--------|-------------|----------|
| Human-first | Standard markdown for developers. | ✓ |
| Agent-first | Machine-parseable numbered steps. | |
| Both with sections | Human overview + machine-parseable quick setup. | |

**User's choice:** Human-first
**Notes:** Standard developer documentation approach.

---

## USAGE Benchmark Section

### Q1: How deep should benchmark documentation go?

| Option | Description | Selected |
|--------|-------------|----------|
| Run + interpret only | Local run, read output, compare with benchstat. No CI details. | ✓ |
| Full workflow including CI | Local + CI gate + baselines + thresholds. | |
| Pointer to CONTRIBUTING | Brief mention then cross-reference. | |

**User's choice:** Run + interpret only
**Notes:** CI gate details belong in CONTRIBUTING.md.

---

## USAGE Observability Depth

### Q1: What's missing or needs expansion?

| Option | Description | Selected |
|--------|-------------|----------|
| Current depth is sufficient | Review for accuracy, fill gaps. | ✓ |
| Add Grafana + alerting examples | Sample dashboard JSON and alert rules. | |
| Add production deployment patterns | Reverse proxy, Docker/K8s, log aggregation. | |

**User's choice:** Current depth is sufficient
**Notes:** Existing sections already have concrete config examples. Just verify accuracy.

---

## USAGE Structure Changes

### Q1: Where should new benchmark content go?

| Option | Description | Selected |
|--------|-------------|----------|
| Under Performance Tuning | Subsection of existing Performance Tuning. | |
| New top-level section | Standalone Benchmarks section. | |
| Under Observability | Subsection of Observability Quickstart. | ✓ |

**User's choice:** Under Observability
**Notes:** Groups monitoring-adjacent content together.

---

## Claude's Discretion

- Exact wording and ordering of benchmark subsection
- Per-agent config JSON format and agent-specific notes
- Cross-references between USAGE.md and INSTALL.md

## Deferred Ideas

None — discussion stayed within phase scope.
