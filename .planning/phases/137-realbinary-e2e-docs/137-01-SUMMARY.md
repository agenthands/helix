---
author: engineer
responsible: architect
phase: 137
plan: "01"
milestone: v2.12
status: complete
verified_by: architect (independent, fresh-binary uncached E2E run + test-body re-read + docs re-read + build/vet)
parent_artifacts:
  - .planning/phases/137-realbinary-e2e-docs/137-01-PLAN.md
---

# Phase 137 SUMMARY — Real-binary C-family E2E + docs

## Outcome: COMPLETE + independently verified — MILESTONE DELIVERABLE PROVEN

A real `helix` binary, driven as a subprocess against a live daemon, returns a
`has_type` edge for a C var→type reference via `helix explain-symbol-deep`. The
production E2E the milestone is named for is green.

## Changes

- **New E2E oracle** `internal/cli/cli_type_resolution_e2e_test.go` (build `!windows`,
  pkg `cli_test`, HELIX_BIN-gated): `TestCLI_E2E_CTypeResolution`. Seeds one C file
  (`struct Foo{int a;};` + `int use(struct Foo *p)` + a primitive-param negative arm),
  activates, elevates to review, indexes + explains via REAL subprocesses.
- **`docs/type-resolution.md` reconciled** to the Phase-136 production path: new
  "Production wiring (v2.12 Phase 136 — per-batch, inside the index build)" section
  (replaces the stale "daemon registers all languages at bootstrap"); "Read surface"
  (RESOLVES_TO→has_type via `explain-symbol-deep`, citing the E2E); "Deferred surfaces"
  (Tier-1 LSP, `type_chain`+Schema-v6, cross-package); typeIndex section updated
  (now populated in production from `nameToNode`). Stale "7 languages" drift confirmed
  gone (removed by the Phase-136 cutover).

## Mode-gate mechanism (resolved empirically)

Session mode is GLOBAL daemon state (one shared `daemonSessionProvider`), so a switch
on any connection is seen by later CLI subprocesses on the same socket. The flat
`switch-mode` CLI verb has no `target_mode` flag, so the switch is issued over MCP
(`forwarder.CallTool switch_mode target_mode=review`, always-allowed +
`validateModeTransition`-gated — no authz weakening). Then `helix index-semantic-graph
--mode-arg=full` and the HEADLINE `helix explain-symbol-deep` run as REAL subprocesses.
Read (explain) is read+ and holds in any mode; only index needed the elevation.

## Discovery (honestly reported)

The `explain-symbol-deep` seed `file_path` must be ABSOLUTE — `semantic_files.path` is
stored verbatim from the full-walk (absolute); a relative seed returns
`resolution="not_found"`. Documented in the fixture helper.

## Verification (architect, independent)

- Fresh binary `go build -o /tmp/helix-v212-verify ./cmd/helix`; then
  `HELIX_BIN=/tmp/helix-v212-verify go test -count=1 -run TestCLI_E2E_CTypeResolution
  ./internal/cli/` → PASS (0.65s), uncached.
- Test body re-read: HEADLINE asserted FIRST — exactly 1 `has_type` edge,
  `internal_kind=="RESOLVES_TO"`, `from==seed p`, `to` contains `Foo`+`struct`.
  Differential negative: `n` (primitive) resolves `exact` (non-vacuous) → 0 has_type
  edges. Determinism: re-index → identical edge count + from/to. Not weakened.
- `go build ./...` clean; `make vet` clean (8 vettools, exit 0); `go.mod`/`go.sum`
  byte-unchanged.
- Docs re-read: reconciliation landed; stale bootstrap/7-languages claims gone.

## Milestone status

v2.12 build complete: 135 (extraction foundation) + 136 (resolver wiring, in-process
proof) + 137 (real-binary E2E + docs) all COMPLETE + independently verified. The
orphaned type-resolver subsystem is now a production consumer producing queryable
C-family `has_type` edges. Remaining: milestone audit + planning-state close.
