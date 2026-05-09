---
phase: 65
plan: 05
subsystem: skill/repomap
tags:
  - repomap
  - get_repo_map
  - get_context
  - setter
  - strangler-fig
  - integ-seam
requirements:
  - INTEG-01
  - INTEG-02
  - INTEG-05
dependency_graph:
  requires:
    - "65-03 (integ.SemanticLookup interface + integSemanticLookup adapter)"
    - "65-04 (Source / FallbackReason closed enums + ChooseSource + MarshalEnvelope)"
  provides:
    - "RepoMapSkill.SetSemanticLookup(integ.SemanticLookup) (4th setter in family)"
    - "RepoMapSkill.SetConfigGate(integ.ConfigGate)"
    - "JSON-envelope wire shape for get_repo_map / get_context (Pitfall §2)"
    - "adaptRankedFiles + computeFreshness helpers for Phase 65 consumers"
    - "daemonCfgGate (small ConfigGate adapter)"
  affects:
    - "internal/skill/repomap (new strangler.go; skill.go execGetRepoMap / execGetContext rewritten)"
    - "internal/daemon (new step 12e; new daemonCfgGate type)"
tech_stack:
  added: []
  patterns:
    - "Setter post-init wiring (4th setter, mirrors SetEnrichFn / SetFallbackDeps / SetMetricsSink)"
    - "Nil-normalizer accessor (lookup() returns NoopLookup{} when unwired, like metricsSink())"
    - "Strangler-fig consumer-side branch (ChooseSource gates lookup.RankFiles vs v1.9 path)"
    - "JSON envelope merge via integ.MarshalEnvelope (tree text preserved verbatim under 'tree')"
    - "Sort-before-iterate (Phase 62 CR-03) preserved in adaptRankedFiles"
key_files:
  created:
    - "internal/skill/repomap/strangler.go (66 LOC: adaptRankedFiles + computeFreshness)"
    - "internal/skill/repomap/strangler_test.go (290 LOC: 6 RED tests + fakeLookup + fakeCfg)"
  modified:
    - "internal/skill/repomap/skill.go (+147 lines: 2 new fields, 4 new methods, exec* rewrites, renderV19 + ensureRenderer + workspaceKey helpers)"
    - "internal/skill/repomap/skill_integration_test.go (+18 lines: JSON-wrap mechanical update of 2 goldens)"
    - "internal/daemon/daemon.go (+18 lines: step 12e wiring)"
    - "internal/daemon/semantic_wiring.go (+25 lines: daemonCfgGate)"
decisions:
  - "Wired SetConfigGate ALWAYS at daemon post-init (even when sBndl is nil) so the v1.9 steady state renders source='tree_sitter' instead of source='fallback' + reason='index_disabled' (D-04 / Pitfall §3)"
  - "renderV19 helper accepts optional seedFiles to share the personalized PageRank path between execGetRepoMap and execGetContext"
  - "ensureRenderer added so the semantic arm bypasses ensureCache (lookup provides ranked list; cache walk unnecessary on the semantic path) but the renderer still gets constructed lazily"
  - "graph_version sourced from RankedFile[0].GraphVersion (Phase 62 D-07 contract — per-call stable) rather than a separate Status round-trip; falls back to lookup.Status when the ranked list is empty"
metrics:
  duration: "~25 min"
  tasks_completed: "RED + GREEN gates (REFACTOR not needed — renderV19 helper already extracted in GREEN)"
  completed_date: "2026-05-08"
---

# Phase 65 Plan 05: get_repo_map + get_context Strangler-Fig Integration Summary

JSON envelope contract for `get_repo_map` / `get_context` wired to the
`integ.SemanticLookup` seam via a new `RepoMapSkill.SetSemanticLookup` setter
(joins `SetEnrichFn` / `SetFallbackDeps` / `SetMetricsSink` family) plus
`SetConfigGate` for the priority-ladder gate.

## What was built

The two RepoMap-skill MCP tools now consult the daemon-side semantic engine
through the `integ.SemanticLookup` interface (INTEG-01 / INTEG-02). When the
config flag is on AND the lookup reports `Available()`, ranking delegates to
`lookup.RankFiles` (uniform PageRank) or `lookup.RankFromSeeds` (personalized)
— the existing `internal/repomap.TreeRenderer.RenderBudgeted` is the only
renderer in either branch. When the config flag is off OR the lookup
errors, the existing v1.9 tree-sitter + PageRank path runs.

Every result is JSON-wrapped via `integ.MarshalEnvelope` (Pitfall §2): the
tree text is preserved **verbatim** under the `"tree"` key; the envelope adds
the closed-enum `source` (`semantic` | `tree_sitter` | `fallback`),
`fallback_reason` (omitempty), `graph_version` (omitempty), and
`freshness` (omitempty). INTEG-05 closed for the two skill-resident tools.

## Files

| File | Status | Purpose |
|------|--------|---------|
| `internal/skill/repomap/skill.go` | MODIFIED | Add `semanticLookup` / `cfgGate` fields, `SetSemanticLookup` / `SetConfigGate` setters, nil-normalizer accessors `lookup()` / `configGate()`, rewrite `execGetRepoMap` + `execGetContext` to consult ChooseSource and JSON-wrap output. New helpers: `renderV19(budget, seedFiles)`, `ensureRenderer()`, `workspaceKey()`. |
| `internal/skill/repomap/strangler.go` | NEW | `adaptRankedFiles([]integ.RankedFile) []repomap.RankedFile` (sort-before-iterate, score desc + Path asc tiebreak); `computeFreshness(ctx, lookup, ws) string` (closed-enum freshness marker mapping). |
| `internal/skill/repomap/strangler_test.go` | NEW | 6 tests: semantic-on success, lookup-err fallback (`ErrIndexBuilding` → `index_building`), config-disabled `tree_sitter` (D-04 / Pitfall §3), `get_context` semantic via `RankFromSeeds`, defensive `index_disabled` arm, `adaptRankedFiles` sort doctrine. Uses hand-rolled `fakeLookup` + `fakeCfg` test doubles. |
| `internal/skill/repomap/skill_integration_test.go` | MODIFIED | JSON-wrap mechanical update of `TestGetRepoMap_WithWorkspace` and `TestGetContext_WithWorkspace`: parse JSON envelope, assert `env.Tree.Contains(".go")`, assert `env.Source == "tree_sitter"` (no cfg gate wired in fixture). |
| `internal/daemon/daemon.go` | MODIFIED | Step 12e: wire `repomapSkill.SetConfigGate(&daemonCfgGate{enabled: cfg.SemanticIndex.Enabled})` always; wire `repomapSkill.SetSemanticLookup(sBndl.integLookupAccessor())` when `sBndl != nil`. |
| `internal/daemon/semantic_wiring.go` | MODIFIED | New `daemonCfgGate` adapter wrapping `cfg.SemanticIndex.Enabled` (compile-time guard against `integ.ConfigGate`). |

## Behavior matrix (closed-enum source field)

| cfg gate enabled | lookup wired & Available() | RankFiles err | source | fallback_reason |
|---|---|---|---|---|
| false | n/a | n/a | `tree_sitter` | "" (omitted) |
| true | no | n/a | `fallback` | `index_disabled` (defensive D-05) |
| true | yes | nil | `semantic` | "" (omitted) |
| true | yes | `ErrNoSnapshot` | `fallback` | `no_snapshot_yet` |
| true | yes | `ErrIndexBuilding` | `fallback` | `index_building` |
| true | yes | `ErrBleveRebuilding` | `fallback` | `bleve_rebuilding` |
| true | yes | other err | `fallback` | `index_error` |

The classifier is `integ.ClassifyLookupErr` (called via `ChooseSource`); raw
error text never reaches the envelope (WR-NEW-01).

## Testing

- `go vet ./...` — PASS (only pre-existing Swift binding macro warning)
- `go build ./...` — PASS
- `go test ./internal/skill/repomap/... ./internal/daemon/... -count=1 -race` — PASS
  - 6 new tests in `strangler_test.go` (all green)
  - 2 updated goldens in `skill_integration_test.go` (mechanical JSON-wrap)
  - `TestIntegSemanticLookup_ReadTierCanary` still green (no write-tier
    tokens reachable from skill code)
- `git diff --stat internal/repomap/` — empty (INTEG-01: zero source change to engine)

## Dependency Graph (frontmatter mirror)

- **Requires:** 65-03 (`integ.SemanticLookup` + `integSemanticLookup` daemon adapter), 65-04 (`Source` / `FallbackReason` enums + `ChooseSource` + `MarshalEnvelope`)
- **Provides for downstream waves:**
  - 65-06 (kernel/symbols `analyze_blast_radius`) reuses `adaptRankedFiles` doctrine + the same setter pattern + the read-tier canary contract
  - 65-07 (`get_health` bridge) reuses `computeFreshness` + `daemonCfgGate`

## Deviations from Plan

None — plan executed as written. The plan suggested REFACTOR step "extract
`s.renderV19` helper for clarity if the body grows" — this extraction was
done as part of GREEN (the body grew enough during GREEN that the helper was
the natural shape), so no separate REFACTOR commit was needed.

## Self-Check: PASSED

- File `internal/skill/repomap/strangler.go` — FOUND
- File `internal/skill/repomap/strangler_test.go` — FOUND
- File `internal/skill/repomap/skill.go` — modified (verified by `grep -c SetSemanticLookup` returning 2)
- File `internal/skill/repomap/skill_integration_test.go` — modified (JSON envelope assertion present)
- File `internal/daemon/daemon.go` — modified (step 12e present, `SetSemanticLookup` count=2)
- File `internal/daemon/semantic_wiring.go` — modified (`daemonCfgGate` present)
- Commit `770267fb` (test RED gate) — FOUND in git log
- Commit `0f11d8a0` (feat GREEN gate) — FOUND in git log
- `git diff --stat internal/repomap/` empty — VERIFIED (INTEG-01 contract held)
- Read-tier canary `grep -nE "(BeginSnapshot|...)" skill.go strangler.go` — CLEAN
