# Codebase Concerns

**Analysis Date:** 2026-07-01

> Documentation of posture and debt for the current Go tree at HEAD. This is
> a map, not an audit: every concern below is grounded in a real Go path, a
> symbol, or a cited planning artifact. No runtime security scan was run.

## Tech Debt

**The tree is clean of in-code debt markers:**
- `grep -rIn 'TODO\|FIXME\|HACK' internal/ cmd/` returns **0 matches** (verified
  2026-07-01). There is no accumulated `TODO`/`FIXME` backlog to document — the
  debt in this codebase is architectural and cross-cutting, tracked in planning
  artifacts (`.planning/deferred-items.md`, milestone audits), not in comments.
- Stated honestly rather than manufactured: there is no bare-except / silent-swallow
  class of debt here (that was the removed Python tree). Go error handling is
  explicit `error` returns; the structured-error taxonomy lives in `internal/errors/`.

**Retained-lineage naming artifacts (intentional, NOT a bug):**
- `SerenaMCPServer` — the exported Go type in `internal/mcp/server.go:45` (plus
  `NewSerenaMCPServer`, `RegisterTools(server *SerenaMCPServer, ...)`). The name is
  a deliberately-frozen internal-API identifier (Phase 52-03 SUMMARY); the
  user-facing MCP `Implementation.Name` is `helix`. Renaming it is a churny,
  zero-value refactor that would touch every kernel `RegisterTools` call site.
- `api/proto/serena/v1/` — the gRPC IPC proto package directory (`proto`
  target in the `Makefile:12-15`). The directory name is a wire-format lineage
  artifact (Phase 52-03): it is baked into the generated `.pb.go` import paths
  and the on-wire fully-qualified message names, so renaming it is a
  wire-compatibility break, not a cleanup.
- These two identifiers are the ONLY `serena`-spelled surfaces that remain; they
  are documented quirks carried forward on purpose, not residue.

**CGO=1 single-mode build coupling:**
- Since Phase 59.1 the source tree is single-mode `CGO_ENABLED=1`; the previous
  CGO=0 stub apparatus was removed. The build now hard-depends on a host C
  compiler (`make build` uses host CC; cross-compile via `zig cc` is CI/release
  only). This is real build complexity: the semantic store links the DuckDB
  static library through CGO, so `go test ./...` and `make build` cannot run in a
  CGO-less environment.
- The single surviving build tag is **platform**-conditional, not
  CGO-conditional: `internal/semantic/store/duckdb.go` carries
  `//go:build !(windows && arm64)` and its sibling stub
  `internal/semantic/store/duckdb_winarm64.go` carries `//go:build windows && arm64`
  (returns `serr.Unsupported`). This keeps the 6-archive release matrix buildable
  while honestly signalling that the semantic store is unavailable on win/arm64.

## Known Bugs & Fragile Areas

**No open bug backlog in code.** There are no skipped-because-broken markers in
the Go suite. Language-server-dependent tests skip *cleanly* on tool
absence (e.g. `t.Skip("gopls not installed...")` in `internal/cli/cli_e2e_test.go`),
which is availability gating, not a masked failure.

**C-family SymbolID collision (fixed as a deviation, root cause deferred):**
- Source: v2.12 MILESTONE-AUDIT "Honest limitations" #2. C `signatureHash`
  truncates at `{`, so a `struct` definition and a type-use of that struct
  collided on `SymbolID` → primary-key violation once C was routed into the
  committed index path. Fixed by a deterministic snapshot-level SymbolID dedup
  (prefer-richer definition). The deeper cause — C emits struct type-uses as
  `definition.struct` symbols — is a documented deferred follow-up (fixing it
  would churn the `with_fields` extractor golden). Fragile because the dedup is
  a corrective layer over the extractor's shape, not a fix at the emit site.

**Absolute-path seed contract for deep queries:**
- Source: v2.12 MILESTONE-AUDIT "Honest limitations" #4. `explain-symbol-deep`'s
  `file_path` seed must be **absolute** — `semantic_files.path` is stored verbatim
  from the full-walk. Documented in the E2E; it is a usage contract, not a defect,
  but a relative seed silently finds nothing.

## Performance Bottlenecks

**No unresolved hot-path bottleneck is tracked.** The performance-sensitive
subsystems ship with their mitigation already in the design; the debt is the
*bound*, not a naive implementation:

- **LS worker pool** (`internal/kernel/lspool/`): share-until-dirty reuse, adaptive
  TTL, circuit breaking, and platform-aware memory-pressure eviction are the
  intended cost controls. The fragility is that these knobs interact; mis-tuning
  TTL or the pressure threshold trades warmth for memory.
- **RepoMap token budgeting** (`internal/repomap/`): output is fit to a token
  budget via binary search over the elided tag tree, and extraction is lazy
  (SQLite tag cache, mtime invalidation). Cost scales with cache coldness, not
  repository size — a cold cache pays the full tree-sitter walk once.
- **Semantic batch typeIndex is intra-package/TU only** (v2.12 M1 limit): the
  per-batch `types.NewDispatcher` + `FixpointResolve` resolve within a translation
  unit / package. Cross-package/cross-TU resolution is explicitly out of scope, so
  wide cross-module type queries return nothing rather than paying an unbounded
  whole-graph cost. This is a deliberate scaling boundary, documented in
  `docs/type-resolution.md`.

## Fragile Areas Requiring Careful Modification

**MCP middleware install order (LIFO invariant):**
- Files: `internal/daemon/daemon.go` (steps 14/14b/14c), `internal/mcp/lazy_init.go:106-108`.
- The SDK composes `AddReceivingMiddleware` in LIFO order, so install order
  `Telemetry+ProfileFilter → Suggestion → LazyInit` yields execution order
  `LazyInit → Suggestion → ProfileFilter → Telemetry → handler`. `LazyInitMiddleware`
  MUST execute first so the workspace is active before `TelemetryMiddleware`
  applies its per-tool deadline. Reordering the installs silently breaks the
  deadline-vs-activation invariant. Safe modification: preserve the documented
  install-order comment and its rationale.

**Architectural import boundaries enforced by the 7 `vet-*` gates:**
These are the load-bearing seams; each gate is the compile-time guard that a
refactor will trip. Wired into `make vet` (Makefile:55) and run in CI
(`.github/workflows/go-test.yml`). Do NOT introduce the forbidden edge:
- `vet-nokernel2semantic` (`internal/lint/nokernel2semantic`): `internal/kernel/`
  MUST NOT import `internal/semantic/` (Phase 60 LIVE-07 #1). One-directional —
  semantic→kernel is the architecture (semantic depends on kernel for typed IDs).
- `vet-nosemantic2kernel` (`internal/lint/nosemantic2kernel`):
  `internal/semantic/lspenrich/` MUST NOT import `internal/kernel/` except the
  `internal/kernel/lspool` carve-out (Phase 61 ENRICH-01 #1). Together with the
  sibling above, pins the kernel↔semantic boundary in BOTH directions.
- `vet-noduckdb` (`internal/lint/noduckdb`): the `duckdb-go` module may only be
  imported from `internal/semantic/store/*` (STORE-06). A stray DuckDB import
  anywhere else fails the build.
- `vet-compact-uses-store` (`internal/lint/compactusesstore`):
  `internal/semantic/compact/` MUST route all DB access through
  `internal/semantic/store` — no direct `duckdb-go` (Phase 63 P63-02).
  Belt-and-braces over `vet-noduckdb`, keeping the SQL boundary auditable in one
  package.
- `vet-ablation-leakage` (`internal/lint/ablationleakage`): the bench-runner
  namespace `bench/runners` MUST NOT import `internal/kernel/lspool` or
  `internal/semantic/store` (Phase 76 ABLATE-08) — a bench runner orchestrates a
  daemon subprocess, it never links the pool or store directly.
- `vet-bench-rag-leakage` (`internal/lint/benchragleakage`): the standalone
  control-arm binary `cmd/helix-bench-rag` MUST NOT import `internal/kernel` or
  `internal/semantic` (Phase 83 ABLATE-04 #1c) — it is a provably-isolated RAG
  baseline.
- `vet-tools-quarantine` (`internal/lint/toolsquarantine`): no package outside
  `github.com/agenthands/helix/tools` may import the dev-time `tools/` tree
  (Phase 106 TUNE-01), so the DSPy offline-tuning harness never leaks into the
  shipped binary or `go.mod`.

**Generated-artifact drift gates (hand-edits fail CI):**
- `internal/cli/verbs_gen.go` (`// Code generated by helix-cligen; DO NOT EDIT`) is
  the frozen 51-verb catalog; `verify-cligen` (`go run ./cmd/helix-cligen --check`)
  hard-fails on drift. The README tool table (`verify-docs`) and the skill
  `reference.md` (`verify-reference`) are the same discipline. Editing any of these
  by hand instead of regenerating breaks the build.

**Semantic dataflow append order (EdgeID stability):**
- Source: v2.13 MILESTONE-AUDIT (M1). The in-body + return-bridge edge passes in
  `semantic_similarity_edges.go` MUST append strictly after `dataFlowEdges`
  (`semantic_wiring.go:2541-2542`) to keep existing EdgeIDs unshifted. Reordering
  churns edge identity and breaks the deterministic re-index invariant.

## Security Considerations

> Posture documentation only — no penetration test or dependency CVE scan was
> run for this map. Findings below are structural, grounded in build/config.

**Runtime attack surface is minimal and unchanged:**
- Single Go binary, no runtime Python/Docker/interpreter dependency (CLAUDE.md
  Constraints). The agent-facing surface is the `helix` CLI dialing the daemon
  over gRPC `StreamMCP` on a unix socket / named pipe by default (opt-in
  loopback-gated TCP). The stdio MCP forwarder head and the Streamable-HTTP `/mcp`
  head were REMOVED in Phase 94 — only the internal gRPC wire remains, shrinking
  the exposed surface.
- v2.12 and v2.13 milestone security reviews both recorded **PASSED, 0 findings**
  (v2.13 MILESTONE-AUDIT: "Security review: PASSED, 0 findings (5/5 checks
  clean)"). The recent dataflow/type-resolver work added no new deps, no new I/O,
  and no schema change (pure in-memory transforms over already-extracted facts).

**Supply-chain posture (sigstore keyless attestation):**
- Release archives are signed with cosign keyless Sigstore attestation
  (`sigstore/sigstore-go` dep; `internal/upgrade/trusted_root.json`, refreshed via
  `make update-trust-root`). This is the ONLY signature on the darwin archives —
  Apple Developer ID signing/notarization is deferred (see Missing Critical
  Features / DEF-59-NOTARIZE).
- The bench container stack drives docker/podman purely via `os/exec`; the
  `verify-no-docker-sdk` gate (Makefile:422) hard-fails if the Docker Go SDK
  (`github.com/docker/docker`) ever enters `go.mod`, keeping the Engine SDK out of
  the supply chain. The `vet-tools-quarantine` gate keeps the dev-time tuning
  harness out of the shipped module.

**No secret handling in the shipped path:** benches that touch provider API keys
are nightly/on-demand only and never run on `pull_request` (`.github/workflows/bench.yml`
least-privilege `permissions: {contents: read}`); the PR-gating `bench-quick` is a
hermetic scripted-agent smoke with no API key.

## Missing Critical Features

These are documented, intentionally-deferred items with a cited source and a
trigger-to-reconsider — not silent gaps.

**C++/C#/Java variable→type linkage co-capture (no dedicated E2E):**
- Source: v2.12 MILESTONE-AUDIT "Honest limitations" #1. C++/C#/Java share the
  identical batch wiring + ChainTokens resolver path, but only **C** has the
  var→type co-capture linkage (Phase 135 scoped `DeclaredType` co-capture +
  `linkVarTypes` to C) and a dedicated real-binary E2E fixture
  (`internal/cli/cli_type_resolution_e2e_test.go`, `TestCLI_E2E_CTypeResolution`).
  C++/C#/Java var→type linkage is a stated fast-follow. Priority: Medium.

**`trace_data_flow` function-seed verb contract:**
- Source: v2.13 MILESTONE-AUDIT "Deferred / follow-ons". v2.13 proves the
  mechanically-accepted function seed for the multi-hop reachability path (the BFS
  filters `edge_kind='DATA_FLOWS'`, kind-agnostic on the seed), but the verb's
  *documented* seed contract remains a parameter. Aligning the documented contract
  with the accepted function seed is a fast-follow. Priority: Low/Medium.

**Variable-level graph nodes, field/heap flow, source/sink taint:**
- Source: v2.13 MILESTONE-AUDIT "Deferred / follow-ons". Explicitly milestone
  out-of-scope (the honest cut the co-driver locked): DATA_FLOWS edges anchor on
  existing function + parameter symbol nodes; the return value's identity IS the
  producer function node by design. There are no variable-level nodes and no
  taint source/sink model. Helix ships NO taint/CFG/IR/slice verbs (CLAUDE.md).
  Priority: out-of-scope until a milestone reopens it.

**Full 6-archive release matrix (linux/windows via zig):**
- Source: `DEF-59.1-LINUX-ZIG-LIBSTDCXX` (`.planning/deferred-items.md`). CI's
  `zig cc -target *-linux-musl` cannot link the prebuilt `libduckdb_static.a`
  (compiled against libstdc++ on glibc; zig musl ships libc++ only), producing
  ~25 undefined-symbol errors on the linux targets. The darwin path (Apple clang
  on macos-14) is end-to-end PASS. Resolution options 1–4 are enumerated in the
  deferred item. Priority: Medium (blocks full linux/windows distribution).

**Apple Developer ID signing + notarization; win/arm64 semantic store:**
- `DEF-59-NOTARIZE`: macOS Gatekeeper blocks the unsigned darwin binaries on first
  launch; users need the right-click→Open workaround (~$99/yr Apple Developer
  Program to fix). `DEF-59-WIN-ARM64-RESTORE`: native windows-arm64 semantic-store
  is stubbed to `serr.Unsupported` because `duckdb-go-bindings` ships no
  `lib/windows-arm64` artifact upstream. Both cited in `.planning/deferred-items.md`.

## Test Coverage Gaps

**C++/C#/Java type-resolution E2E:** only C has a dedicated real-binary E2E
fixture (`TestCLI_E2E_CTypeResolution`); C++/C#/Java are proven at the
engine/emission level but lack the equivalent end-to-end oracle (v2.12
MILESTONE-AUDIT limitation #1). Risk: a daemon-layer gate regression on those
languages would not be caught end-to-end — precisely the class of gap v2.13's
Phase 140 hit for the DATA_FLOWS `langFromExt` daemon gate (v2.13 MILESTONE-AUDIT
"one scope deviation").

**LS-backed behavioral chains are environment-gated:** the gopls/jdtls-dependent
E2E oracles in `internal/cli/cli_e2e_test.go` (`requireGoplsE2E`,
`TestCLI_DualRunParity` `needsLS` rows) `t.Skip` when the language server or the
`HELIX_BIN` binary is absent. On a runner without gopls/jdtls those paths are not
exercised, so LS-dependent regressions can escape a local `go test ./...`.

**Coverage is not enforced:** there is no `-coverprofile`/`-cover` gate in the
`Makefile` or any `.github/workflows/*.yml` (verified — 0 matches). Coverage is a
posture (extensive co-located `_test.go` + golden regression nets + real-binary
E2E), not a numeric threshold. Adding a coverage floor is an open opportunity, not
a current guarantee.

---

*Concerns map: 2026-07-01 — Go tree at HEAD.*
