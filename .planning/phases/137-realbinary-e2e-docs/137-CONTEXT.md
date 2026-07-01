---
author: architect
responsible: architect
phase: 137
milestone: v2.12
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.12-ROADMAP.md
  - .planning/phases/136-resolver-chaintokens-producer-emit/136-01-SUMMARY.md
  - agent://VerbsAndE2E
---

# Phase 137 — Real-binary C-family E2E + docs

## Goal (the contract Q this phase establishes)

Prove, through a REAL `helix` binary driven as a subprocess against a live daemon,
that a C-family type reference resolves to a queryable `has_type` edge — the
production E2E the milestone is named for. Then make the docs honest.

**Hoare frame:** `{Phase 136 commits RESOLVES_TO edges for C var→type in the batch
build}` `helix index-semantic-graph → helix explain-symbol-deep` `{the CLI returns a
has_type edge from the referencing symbol to its type symbol}`.

## What's proven already (in-process, Phase 136)

`resolveTypeEdges` commits a `RESOLVES_TO` edge (p→Foo) for a real C fixture, verified
via `QueryStableKeyByNodeID`. Phase 137 proves the SAME through the real binary — the
read path (`explain-symbol-deep` → `SymbolEdgesAccessor` → `QuerySymbolEdgesOutgoing`,
no edge_kind filter → `MapInternalKind(RESOLVES_TO)==has_type`) is production-wired
(`semantic_wiring.go:285`).

## Decisions locked

- **Harness:** reuse `internal/cli/cli_e2e_test.go` + `internal/eval/sandbox`
  (HELIX_BIN-gated; `t.Skip` when no binary — matches the existing oracle). Seed a
  `.c` file (NOT the hardcoded `main.go`) — a new fixture builder or a parameterized
  variant of `newE2EFixture`.
- **Fixture:** a SINGLE `.c` file with `struct Foo { int a; };` + a function taking a
  `struct Foo *p` (flat translation-unit scope → C `SameScope` true → no cross-scope
  cap → validated edge). Keep def + use in one file.
- **Flow:** `mcpActivate` (activate_project) → get the graph indexed → run
  `helix explain-symbol-deep --seed-json='{"file_path":"<rel.c>","symbol_name":"p"}'`
  as a REAL subprocess → assert the JSON contains an edge with `edge_kind=="has_type"`
  (and, ideally, targeting `Foo`).
- **Mode gate (RISK — resolve empirically):** `index-semantic-graph` is review+ and
  excluded from the default `edit` mode. `switch-mode --target-mode=review` may NOT
  persist across separate `helix` subprocess invocations (each CLI call may be a fresh
  session). The executor MUST determine how session mode is keyed and choose a working
  mechanism: (a) if switch-mode persists on the daemon across CLI calls, use it;
  (b) else drive `index_semantic_graph` via `forwarder.CallTool` (MCP path, same
  session as activate) — the INDEX step MAY use the MCP path; (c) else start the daemon
  in a profile/mode where index is permitted. The HEADLINE assertion — `explain-symbol-deep`
  (read+, any mode) returning `has_type` via the REAL BINARY — MUST hold regardless of
  how indexing is triggered. Document the chosen mechanism.
- **Differential anti-vacuity (E1):** the suite MUST assert BOTH a positive fixture
  yields the `has_type` edge AND a negative fixture (a param of primitive type, no
  named type) yields NO `has_type` edge. Positive gates first.
- **Determinism:** re-index the same fixture → identical edge set (SC6).
- **Docs are part of DONE (not optional):** update `docs/type-resolution.md` to reflect
  the Phase-136 production path — the stale "daemon registers all languages at
  bootstrap" claim is WRONG now (that dispatcher was removed); resolution runs
  per-batch inside the index build (`factsFromExtracted`→`resolveTypeEdges`), fed by
  the C var→type linkage, producing committed `RESOLVES_TO` edges surfaced as
  `has_type` via `explain-symbol-deep`. State the deferred surfaces (Tier-1 LSP /
  `type_chain` / Schema-v6). Fix any remaining "7 languages" doc-drift in code
  comments (verify — the Phase-136 cutover may have already removed the stale ones).

## Centerpiece symbols (verified at HEAD)

- `internal/cli/cli_e2e_test.go` — `newE2EFixture` (`:99`, hardcodes main.go seed),
  `mcpActivate` (`:157`), `mcpCall` (`:181`), `runCLIVerbInDir` (`:214`),
  `resolveHelixBin` (`:72`).
- `internal/eval/sandbox` — `NewSandbox`/`Prepare`/`RepoFor`/`StartDaemon`/`SocketFor`.
- `explain-symbol-deep` verb → `--seed-json` flag (`verbs_gen.go:56`); handler
  `tools_explain_symbol.go`; `MapInternalKind(RESOLVES_TO)==has_type`
  (`edge_kind_surface.go:56`).
- `index-semantic-graph` (review+, `tools_index.go`); `switch-mode`/`switch_mode`
  (`verbs_gen.go:423`); mode gate (`mode_check.go`, `profile/modes/edit.yaml`).
- `docs/type-resolution.md` — the doc to reconcile.

## Anti-vacuity (differential)

Positive fixture → exactly the expected `has_type` edge; negative fixture (primitive
type param) → none. A green test that only proves "no spurious edge" while the
positive path emits nothing is the trap the red-team caught — the positive assertion
MUST fire first and MUST find the edge via the real binary.

## Acceptance

1. A real `helix explain-symbol-deep` subprocess returns a `has_type` edge for the C
   fixture after a full index (HEADLINE — SC4). At minimum C proven.
2. Differential anti-vacuity: positive has the edge, negative has none.
3. Determinism: re-index → identical edge set.
4. Mode-gate mechanism documented + working.
5. `docs/type-resolution.md` reconciled to the Phase-136 production path; "7 languages"
   doc-drift fixed (or confirmed already removed by the cutover).
6. HELIX_BIN-gated (`t.Skip` without a binary); zero new Go deps; `go build ./...` +
   `make vet` clean; the new E2E test passes with a freshly-built binary.

## Out of scope

- C++/C#/Java E2E fixtures beyond what time allows — C is the required proof; add the
  others IF the fixtures resolve intra-file (per v2.11 M1 intra-package limit).
- `Store.QueryEffectiveEdges` / Tier-1 LSP (L5); `type_chain` / Schema-v6 (L6).
