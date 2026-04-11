# Requirements: Serena

**Defined:** 2026-04-11
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.3 Requirements

Requirements for documentation catchup. Each maps to roadmap phases.

### README

- [ ] **README-01**: README accurately reflects current tool count, architecture, and capabilities after v1.2
- [ ] **README-02**: README includes observability features (metrics, tracing, admin listener)
- [ ] **README-03**: README install instructions are current and correct

### USAGE

- [ ] **USAGE-01**: USAGE documents observability configuration (Prometheus, OTLP, admin listener)
- [ ] **USAGE-02**: USAGE documents graceful degradation settings (budgets, GOMEMLIMIT, circuit breaker)
- [ ] **USAGE-03**: USAGE documents performance tuning and benchmark workflow

### CHANGELOG

- [ ] **CHLOG-01**: CHANGELOG v1.2 entry is complete with all phases and key accomplishments

### CONTRIBUTING

- [ ] **CONTR-01**: CONTRIBUTING reflects current dev workflow (build, test, vet, benchmark commands)
- [ ] **CONTR-02**: CONTRIBUTING documents integration test and benchmark harness usage

### Install Guide

- [x] **INST-01**: Rename llms-install.md to agent-focused install guide (targets coding assistants, not "LLMs")
- [x] **INST-02**: Install guide covers Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity setup

## Future Requirements

None — documentation catchup milestone.

## Out of Scope

| Feature | Reason |
|---------|--------|
| API reference docs | Auto-generated from godoc, not hand-maintained |
| Tutorials / walkthroughs | Deferred to future milestone |
| Video documentation | Not in scope for text-based catchup |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| README-01 | Phase 16 | Pending |
| README-02 | Phase 16 | Pending |
| README-03 | Phase 16 | Pending |
| USAGE-01 | Phase 17 | Pending |
| USAGE-02 | Phase 17 | Pending |
| USAGE-03 | Phase 17 | Pending |
| CHLOG-01 | Phase 16 | Pending |
| CONTR-01 | Phase 16 | Pending |
| CONTR-02 | Phase 16 | Pending |
| INST-01 | Phase 17 | Complete |
| INST-02 | Phase 17 | Complete |

**Coverage:**
- v1.3 requirements: 11 total
- Mapped to phases: 11
- Unmapped: 0

---
*Requirements defined: 2026-04-11*
*Last updated: 2026-04-11 after roadmap creation*
