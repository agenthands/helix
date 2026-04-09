# Phase 6: Test Harness + Go Dogfooding - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-08
**Phase:** 06-test-harness-go-dogfooding
**Areas discussed:** Test package location, MCP client wiring, Dogfood fixture strategy

---

## Test Package Location

| Option | Description | Selected |
|--------|-------------|----------|
| New top-level test/ package | Black-box: tests only public API. Clean separation. Similar to legacy test/ structure. Needs exported daemon accessors. | ✓ |
| internal/integration/ | Black-box but inside internal/. Can import internal packages. New package, clean slate. | |
| Extend internal/daemon/ | White-box: reuses existing newE2EConfig() pattern. Tests can access private fields. Risk of growing too large. | |
| You decide | Claude picks the best approach based on codebase patterns | |

**User's choice:** New top-level test/ package
**Notes:** Forces testing through public APIs, mirrors legacy structure.

---

## MCP Client Wiring

| Option | Description | Selected |
|--------|-------------|----------|
| InMemory only | NewInMemoryTransports() — fast, in-process, no network. Tests MCP protocol but not HTTP layer. Simplest to implement. | |
| HTTP only | httptest.NewServer + StreamableClientTransport — tests real HTTP path including serialization. Slower but catches more bugs. | |
| Both (layered) | InMemory for most tests (fast), HTTP for a smoke test subset. Best coverage but more harness complexity. | ✓ |
| You decide | Claude picks based on what catches the most bugs with least complexity | |

**User's choice:** Both (layered)
**Notes:** InMemory as default for speed, HTTP for smoke subset validating full transport path.

---

## Dogfood Fixture Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Serena's own codebase | Point gopls at the real repo root. Most realistic. Slower indexing (~5s for 25K LOC). Symbol positions may shift between commits. | |
| Small Go testdata/ | Minimal Go project in testdata/fixtures/go/ with known symbols. Fast indexing. Stable assertions. Less realistic. | |
| Both (tiered) | Small testdata/ for fast symbol/edit tests. Full codebase for a smoke test that proves real-world scale works. | ✓ |
| You decide | Claude picks based on test reliability vs realism tradeoff | |

**User's choice:** Both (tiered)
**Notes:** Small fixtures for deterministic structural assertions, full codebase for behavioral smoke tests.

---

## Claude's Discretion

- Build tag naming, LS readiness polling strategy, test helper API design, go-cmp promotion decision

## Deferred Ideas

None
