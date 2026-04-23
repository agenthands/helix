# Requirements: Serena

**Defined:** 2026-04-23
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.8 Requirements

Requirements for documentation overhaul. Each maps to roadmap phases.

### README

- [ ] **README-01**: README.md presents Serena as a standalone Go-native code intelligence platform, not a port
- [ ] **README-02**: README.md accurately lists all 41+ MCP tools with current descriptions
- [ ] **README-03**: README.md reflects current architecture (4-layer, daemon, RepoMap, fuzzy editing, smart errors, progressive descriptions)
- [ ] **README-04**: README.md includes quick start that references `serena setup <client>` and lazy init

### USAGE

- [x] **USAGE-01**: USAGE.md documents all v1.6 features (fuzzy editing, RepoMap/get_context, 23 tree-sitter grammars)
- [ ] **USAGE-02**: USAGE.md documents all v1.7 features (setup CLI, health tool, hooks, smart errors, progressive descriptions, lazy init)
- [x] **USAGE-03**: USAGE.md troubleshooting section is current with known issues and workarounds

### INSTALL

- [ ] **INST-01**: INSTALL.md reflects current install paths and `serena setup <client>` as primary method
- [ ] **INST-02**: INSTALL.md has accurate MCP configs for all 6 supported clients

### CONTRIBUTING

- [ ] **CONT-01**: CONTRIBUTING.md reflects current Go codebase structure and dev workflow
- [ ] **CONT-02**: CONTRIBUTING.md references current test harness (oracle tests, integration tags, benchmark gates)

### CHANGELOG

- [ ] **CLOG-01**: CHANGELOG.md has accurate entries for all milestones v1.0-v1.7
- [ ] **CLOG-02**: CHANGELOG.md v1.6 and v1.7 entries are complete with all shipped features

### CLAUDE.md

- [ ] **CLMD-01**: CLAUDE.md project description updated to match current product identity
- [ ] **CLMD-02**: CLAUDE.md architecture section reflects current state (RepoMap, fuzzy editing, setup CLI, health tools)

### Legacy Framing

- [ ] **LEGC-01**: All docs acknowledge Python origin briefly ("Originally inspired by") with no "port" or "rewrite" language

## Future Requirements

None -- documentation milestone is self-contained.

## Out of Scope

| Feature | Reason |
|---------|--------|
| API reference / godoc | Generated from code, not hand-maintained docs |
| Tutorial / cookbook | Beyond current milestone scope |
| Localization / i18n | English-only documentation |
| Video / interactive guides | Text-first documentation |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| README-01 | Phase 39 | Pending |
| README-02 | Phase 39 | Pending |
| README-03 | Phase 39 | Pending |
| README-04 | Phase 39 | Pending |
| LEGC-01 | Phase 39 | Pending |
| USAGE-01 | Phase 40 | Complete |
| USAGE-02 | Phase 40 | Pending |
| USAGE-03 | Phase 40 | Complete |
| INST-01 | Phase 41 | Pending |
| INST-02 | Phase 41 | Pending |
| CONT-01 | Phase 41 | Pending |
| CONT-02 | Phase 41 | Pending |
| CLOG-01 | Phase 42 | Pending |
| CLOG-02 | Phase 42 | Pending |
| CLMD-01 | Phase 42 | Pending |
| CLMD-02 | Phase 42 | Pending |

**Coverage:**
- v1.8 requirements: 16 total
- Mapped to phases: 16
- Unmapped: 0

---
*Requirements defined: 2026-04-23*
*Last updated: 2026-04-23 after roadmap creation*
