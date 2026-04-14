# Requirements: Serena

**Defined:** 2026-04-14
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.5 Requirements

Requirements for v1.5 Typed Errors & Hardening. Each maps to roadmap phases.

### Error Taxonomy

- [ ] **ERR-01**: User receives a typed error kind (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout) from every tool failure
- [ ] **ERR-02**: User receives structured error fields (Kind, Message, Tool, Detail) that agents can parse programmatically
- [ ] **ERR-03**: User receives wrapped errors that preserve the underlying cause chain (LSP, filesystem, tree-sitter) with typed context

### Tool Migration

- [ ] **MIG-01**: All 9 symbol retrieval tools return typed errors instead of raw strings
- [ ] **MIG-02**: All 6 symbol editing tools return typed errors instead of raw strings
- [ ] **MIG-03**: All 6 file operation tools return typed errors instead of raw strings
- [ ] **MIG-04**: All 3 diagnostic tools return typed errors instead of raw strings
- [ ] **MIG-05**: All 7 memory tools return typed errors instead of raw strings
- [ ] **MIG-06**: All 2 workflow tools return typed errors instead of raw strings
- [ ] **MIG-07**: All 2 profile tools return typed errors instead of raw strings
- [ ] **MIG-08**: All 3 MCP core tools (ping, echo, activate_project) return typed errors instead of raw strings

### Validation & Testing

- [ ] **VAL-01**: User receives clear validation errors when providing invalid parameters to any tool
- [ ] **VAL-02**: Three-band error tests upgraded from string matching to typed error kind assertions
- [ ] **VAL-03**: Golden files assert error response shapes per error kind for regression detection

## Future Requirements

None deferred — this milestone is focused and self-contained.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Custom error recovery strategies | Agents handle recovery; Serena just reports errors clearly |
| Error telemetry/dashboards | Observability stack already exists (v1.2); typed errors integrate naturally |
| Client-facing error codes (numeric) | MCP protocol doesn't use numeric error codes; typed kinds are sufficient |
| Retry logic in tools | Agents decide retry policy; tools report errors and move on |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| ERR-01 | — | Pending |
| ERR-02 | — | Pending |
| ERR-03 | — | Pending |
| MIG-01 | — | Pending |
| MIG-02 | — | Pending |
| MIG-03 | — | Pending |
| MIG-04 | — | Pending |
| MIG-05 | — | Pending |
| MIG-06 | — | Pending |
| MIG-07 | — | Pending |
| MIG-08 | — | Pending |
| VAL-01 | — | Pending |
| VAL-02 | — | Pending |
| VAL-03 | — | Pending |

**Coverage:**
- v1.5 requirements: 14 total
- Mapped to phases: 0
- Unmapped: 14

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-14 after initial definition*
