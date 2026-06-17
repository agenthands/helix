---
phase: quick
plan: 260617-t7x
subsystem: docs/bench
tags: [docgen, tool-manifest, golden, resync, semantic]
requires: [internal/skill/semantic registered ToolProvider (live registry)]
provides: [README tool table reflecting all 53 MCP tools, bench manifest+golden parity at 53]
affects: [cmd/docgen, test/bench, README.md, CLAUDE.md]
tech-stack:
  added: []
  patterns: [blank-import init() registration parity (docgen ↔ daemon/imports.go)]
key-files:
  created: []
  modified:
    - cmd/docgen/main.go
    - README.md
    - test/bench/main_test.go
    - test/bench/tools_manifest_test.go
    - test/bench/testdata/tool_descriptions.golden
    - CLAUDE.md
decisions:
  - "Trusted the live registry (53) as source of truth; README ToolProvider table reads 51 rows by design (excludes 3 built-ins ping/echo/activate_project, includes analyze_blast_radius) — not forced to 53."
metrics:
  duration: ~12min
  completed: 2026-06-17
---

# Phase quick Plan 260617-t7x: Tool/Capability Doc Resync Summary

JWT-free docs/golden/expectation resync — docgen now blank-imports the semantic skill (root cause of the README lag), and the README table, bench `expectedCount`, manifest entries, descriptions golden, and CLAUDE.md prose all match the live 53-tool MCP registry. No tool registration or runtime code changed.

## What Was Done

### Task 1 — docgen import + README regen + bench manifest/count bump (commit `ed0abb97`)
- Added `_ "github.com/agenthands/helix/internal/skill/semantic"` to `cmd/docgen/main.go` (parity with `internal/daemon/imports.go:13`). This was the root cause: without it `skill.ToolProviders()` never saw the semantic provider, so docgen emitted only the non-semantic tools.
- Regenerated the README tool table via `go run ./cmd/docgen` (table NOT hand-edited): **41 → 51 ToolProvider rows** (the 10 semantic tools now appear). `go run ./cmd/docgen --check` prints "README.md is up to date."
- Bumped `const expectedCount = 47 → 53` in `test/bench/main_test.go`.
- Added **6 new `benchCase` entries** to `test/bench/tools_manifest_test.go` (`get_cluster_map`, `explain_cluster`, `explain_symbol_deep`, `find_related_symbols`, `get_change_impact_graph`, `validate_graph_edge`) under a new "Semantic deep-context (6, Phase 65)" block; updated the header-comment breakdown to 53. The parity test compares NAMES only, so minimal schema-shaped args suffice.

### Task 2 — golden refresh + CLAUDE.md prose (commit `46a2a6fd`)
- Regenerated `test/bench/testdata/tool_descriptions.golden` via `go test ./test/bench/ -run TestToolDescriptionsGoldenFile -update`: **47 → 53 lines**. `git diff` confirmed the ONLY additions were the 6 new semantic-tool description lines — no existing description changed.
- Updated CLAUDE.md prose: "41+ MCP tools" → "53 MCP tools" (line 33) and "41+ callable tools" → "53 callable tools" (tool-inventory note). The auto-generated README table was NOT touched here.

## Final Numbers (reconciled against the live registry)

| View | Count | Notes |
|------|-------|-------|
| Live MCP registry (`benchDaemon.RegistryNames()`) | **53** | source of truth; INCLUDES 3 built-ins (ping/echo/activate_project) |
| bench `expectedCount` / `benchTools` entries / golden lines | **53** | full bidirectional name parity, no manifestOnly/registryOnly |
| README ToolProvider table rows | **51** | excludes 3 built-ins (not ToolProviders), includes analyze_blast_radius — pre-existing ToolProvider/registry view difference, by design |
| CLAUDE.md prose | **53** | describes the whole MCP surface |

The 51-vs-53 reconciliation the planner flagged held exactly as predicted: the README table is a `skill.ToolProviders()` view (51), the tests assert the full live registry (53).

## Deviations from Plan

None — plan executed exactly as written. The optional `guardrails` blank import was not added (explicitly not required for any test; semantic is the load-bearing one).

## Verification

- `go run ./cmd/docgen --check` → "README.md is up to date." ✅
- `go test ./test/bench/... -count=1` → all green; both previously-failing tests pass (`TestBenchToolsManifestMatchesRegistry`, `TestToolDescriptionsGoldenFile`) ✅
- `go build ./...` ✅
- `go vet ./...` ✅
- `make bench-quick` → 1/1 cells succeeded, schema-valid result.v2.json ✅
- `grep "41+ MCP tools" CLAUDE.md` → 0 matches ✅
- golden line count → 53 ✅
- `gofmt -w` applied to all changed .go files ✅

## Scope Compliance

Only permitted non-doc code edit was the single blank import in `cmd/docgen/main.go` (a doc-generation tool in `cmd/`, not a runtime package). No edits under `internal/skill/semantic/`, `internal/mcp/`, `internal/kernel/`, or `internal/daemon/`. Docs and expectations moved to match the registry, never the reverse.

## Self-Check: PASSED

- cmd/docgen/main.go — FOUND (committed in ed0abb97)
- README.md — FOUND (committed in ed0abb97)
- test/bench/main_test.go — FOUND (committed in ed0abb97)
- test/bench/tools_manifest_test.go — FOUND (committed in ed0abb97)
- test/bench/testdata/tool_descriptions.golden — FOUND (committed in 46a2a6fd)
- CLAUDE.md — FOUND (committed in 46a2a6fd)
- commit ed0abb97 — FOUND
- commit 46a2a6fd — FOUND
