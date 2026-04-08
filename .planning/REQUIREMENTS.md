# Requirements: Serena v1.1 Integration Testing

**Defined:** 2026-04-08
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.1 Requirements

Requirements for integration testing milestone. Each maps to roadmap phases.

### Test Harness Infrastructure

- [ ] **HARN-01**: Integration tests use `//go:build integration` tag so `go test ./...` skips them by default
- [ ] **HARN-02**: Reusable test harness starts daemon in-process via `daemon.New()` and returns connected test context
- [ ] **HARN-03**: Test harness supports full MCP protocol round-trips via `NewInMemoryTransports()` or `StreamableClientTransport`
- [ ] **HARN-04**: Tests skip gracefully when required language server is not installed (`t.Skip`)
- [ ] **HARN-05**: Tests enforce per-test timeouts and clean up LS worker processes on teardown
- [ ] **HARN-06**: LS readiness polling waits for language server to finish indexing before assertions

### Go Dogfooding

- [ ] **DOG-01**: All 9 symbol retrieval tools return correct results against Serena's own Go codebase
- [ ] **DOG-02**: All 6 file operation tools work correctly against Serena's own source files
- [ ] **DOG-03**: All 3 diagnostic tools return valid results against Serena's own Go code
- [ ] **DOG-04**: Memory tools (write, read, list, search, edit, rename, delete) work end-to-end
- [ ] **DOG-05**: Workflow tools (onboard, prepare handoff) execute successfully against own codebase
- [ ] **DOG-06**: Profile tools (switch mode, get token budget) function correctly

### Symbol Editing

- [ ] **EDIT-01**: Edit round-trip works: read symbol → replace body → re-read → verify change took effect
- [ ] **EDIT-02**: Insert before/after symbol places content at correct position
- [ ] **EDIT-03**: Rename symbol updates all references across files
- [ ] **EDIT-04**: Safe delete blocks when references exist and succeeds when unreferenced

### Multi-Language Fixtures

- [ ] **LANG-01**: Python fixture project with known symbols exercised by symbol retrieval tools
- [ ] **LANG-02**: TypeScript fixture project with known symbols exercised by symbol retrieval tools
- [ ] **LANG-03**: Java fixture project with known symbols exercised by symbol retrieval tools
- [ ] **LANG-04**: Rust fixture project with known symbols exercised by symbol retrieval tools
- [ ] **LANG-05**: Cross-file reference chains verified across multi-file fixtures

### Advanced Testing

- [ ] **ADV-01**: Each of 5 agent profiles exposes exactly the expected tool subset
- [ ] **ADV-02**: Each of 4 modes filters tool visibility correctly
- [ ] **ADV-03**: Worker pool handles concurrent tool calls without races or deadlocks
- [ ] **ADV-04**: Error paths tested — tool called before workspace activation, file not found, symbol not found

## Future Requirements

Deferred to v1.2+. Tracked but not in current roadmap.

### Stability & Performance

- **STAB-01**: Snapshot / golden file testing for output format stability
- **STAB-02**: Performance benchmarking suite for tool response times
- **STAB-03**: Transport-level tests (Streamable HTTP + gRPC forwarder)

### Adversarial

- **ADV-05**: Fuzz testing of MCP tool inputs
- **ADV-06**: Malformed JSON-RPC request handling

## Out of Scope

| Feature | Reason |
|---------|--------|
| Testing all 52 languages | Combinatorial explosion — 5 representative languages cover the value |
| Mock language servers | Defeats purpose — integration tests must hit real LS |
| Snapshot testing as primary strategy | Brittle with LSP — line numbers shift, LS versions change output |
| Testing legacy Python code | Legacy is reference only, not active code |
| End-to-end agent conversation tests | Non-deterministic LLM interactions, not testable |
| Per-tool unit test duplication | Existing unit tests already cover component logic with mocks |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| | | |

**Coverage:**
- v1.1 requirements: 24 total
- Mapped to phases: 0
- Unmapped: 24 ⚠️

---
*Requirements defined: 2026-04-08*
*Last updated: 2026-04-08 after initial definition*
