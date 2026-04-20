---
phase: 33
slug: fallback-extractor-wiring
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-20
---

# Phase 33 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test ./internal/repomap/... ./internal/skill/repomap/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/repomap/... ./internal/skill/repomap/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | 33-01 | 1 | RMAP-02 | — | Fallback fires for non-tree-sitter languages | integration | `go test ./internal/skill/repomap/ -run TestFallback` | No | Pending |
| TBD | 33-01 | 1 | RMAP-02 | — | Missing LSP logs debug, returns empty | unit | `go test ./internal/repomap/ -run TestFallbackExtractor` | Yes | Passing |
| TBD | 33-02 | 1 | RMAP-03 | — | Cache survives close/reopen | unit | `go test ./internal/repomap/ -run TestTagCache_Persistence` | Yes | Passing |

---

## Validation Architecture

### Coverage Strategy
- Existing unit tests for FallbackExtractor (5 cases) and TagCache persistence already pass
- New integration test needed: walkAndExtract with a language that has no tree-sitter grammar
- Compile-time assertion: WorkerLease satisfies SymbolRequester interface

### Risk Areas
- Context propagation through walkAndExtract (signature change)
- Language name mapping between GrammarRegistry and lspool key formats
- Race conditions if multiple walks try to acquire the same lease

### Feedback Loops
- `go vet ./...` after every edit
- Unit tests after each function implementation
- Full suite before marking complete
