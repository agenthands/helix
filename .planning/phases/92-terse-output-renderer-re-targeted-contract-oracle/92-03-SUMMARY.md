---
phase: 92-terse-output-renderer-re-targeted-contract-oracle
plan: 03
subsystem: testing
tags: [contract-oracle, golden-tests, cli-stdout, parity, terse-renderer, behavioral-chain, helix-bin-gated]

# Dependency graph
requires:
  - phase: 92-02
    provides: "renderResultFor terse seam (relpath:line:col<TAB>payload, --abs/--json, color gate, clamped nav snippet); per-kind exit codes; the FROZEN output shape this plan goldens"
  - phase: 92-01
    provides: "renderClassFor (locus-list vs tree/opaque), parseLocusLine + sortDedupLoci, exit-code mapper"
provides:
  - "TestGolden_CLIStdout: re-targeted contract oracle capturing REAL helix <verb> --flags subprocess stdout into per-verb goldens in the frozen terse shape (TEST-02)"
  - "--abs golden variant (absolute <WORKSPACE>/...:L:C form, OUT-07) and --color=never golden variant (zero 0x1b ESC bytes, OUT-06) — flag contracts frozen against real CLI output"
  - "cli_parity_test.go: untagged default-suite typed-args→cobra-flags parity (cli.VerbToolNames() set-equal to live registry) replacing MCP schema meta-validation"
  - "TestCLI_E2E_Chain (OUT-04): nav locus feeds a downstream verb VERBATIM; TestCLI_E2E_NavSelfContained (OUT-03/SC#2): nav output carries a snippet so no follow-up Read is forced"
  - "runCLIVerbInDir: CWD-aware subprocess runner so the renderer's os.Getwd() workspaceRoot relativizes loci"
affects: [93-skill-md]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Golden capture re-targeted from MCP harness.CallTool→TextContent to real helix subprocess stdout (sandbox daemon + HELIX_BIN gate + cmd.Dir=workspace root)"
    - "Default-suite parity oracle (untagged file in an otherwise integration-tagged test package) reads two pure in-process catalogs — no daemon, no socket, no LS"
    - "Copy-paste-chain behavioral oracle: parse one emitted relpath:line:col, feed it verbatim as the next verb's flags, assert exit 0 + result"

key-files:
  created:
    - test/oracle/contract/cli_parity_test.go
    - test/oracle/contract/testdata/golden/go_to_definition/abs.golden
    - test/oracle/contract/testdata/golden/find_references/color_never.golden
  modified:
    - test/oracle/contract/golden_test.go
    - internal/cli/cli_e2e_test.go
    - test/oracle/contract/testdata/golden/go_to_definition/success.golden
    - test/oracle/contract/testdata/golden/find_references/success.golden
    - test/oracle/contract/testdata/golden/search_symbols/success.golden
    - test/oracle/contract/testdata/golden/search_in_files/success.golden
    - test/oracle/contract/testdata/golden/get_symbol_overview/success.golden
    - test/oracle/contract/testdata/golden/get_hover_info/success.golden
    - .gitignore

key-decisions:
  - "Removed schema_test.go (the MCP inputSchema/outputSchema meta-validation) — the v2.0 contract is the CLI verb surface, so the typed-args→cobra-flags parity test supersedes it as the default-suite guarantee (plan: 'replacing MCP schema meta-validation')"
  - "Removed 17 orphaned MCP-era golden dirs (activate_project, list_memories, read_file, …) no longer exercised by any test — kept only the 6 dirs the re-targeted oracle captures plus errors/ (still owned by errors_test.go)"
  - "Re-targeted golden_test.go replicates the sandbox/forwarder bringup directly (the cli_e2e harness is in a different package, cli_test) — same StartDaemon + mcpActivate pattern, gated on HELIX_BIN, gopls-required"
  - "Chain selects the WORKSPACE-LOCAL locus (parseFirstLocusMatching substr='main.go') past stdlib hits that sort ahead of main.go; the relpath/line/col are still consumed VERBATIM — the predicate only picks WHICH locus, never rewrites values"
  - ".gitignore /test/oracle/contract/.helix/ — the activated daemon writes a cwd-relative semantic.duckdb under the test package dir (same precedent as /internal/cli/.helix/)"

patterns-established:
  - "Golden = real CLI subprocess stdout: the freeze is asserted against the binary, not the unit renderer or MCP TextContent"
  - "Flag-variant goldens (abs.golden / color_never.golden) live alongside success.golden in the same tool dir, freezing OUT-06/OUT-07 against real output"

requirements-completed: [TEST-02, OUT-03, OUT-04, OUT-06, OUT-07]

# Metrics
duration: ~10min
completed: 2026-06-21
status: complete
---

# Phase 92 Plan 03: Re-targeted Contract Oracle Summary

**Re-pointed the TEST-02 contract oracle from MCP `TextContent` goldens to REAL `helix <verb> --flags` subprocess stdout — per-verb goldens now freeze the terse `relpath:line:col<TAB>payload` shape (plus `--abs` absolute and `--color=never` zero-ANSI variants) against the binary; the MCP schema meta-validation is replaced by an untagged default-suite typed-args→cobra-flags parity test; and a behavioral chain proves a nav locus feeds a downstream verb verbatim (OUT-04) with a self-contained snippet (OUT-03).**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-06-21T18:37:41Z
- **Completed:** 2026-06-21T18:47:16Z
- **Tasks:** 2
- **Files modified:** 3 source/test files + 6 regenerated goldens + 2 new goldens + .gitignore

## Accomplishments

- **Re-targeted golden oracle (TEST-02):** `TestGolden_CLIStdout` brings up a sandbox daemon over a seeded Go fixture, activates it over MCP, then runs each kebab verb as a real `helix` subprocess (CWD=workspace root) and captures stdout. Goldens span all three render classes: locus-list terse (`go-to-definition`, `find-references`, `search-symbols`, `search-in-files`), tree shape-only (`get-symbol-overview`), opaque markdown (`get-hover-info`). HELIX_BIN-gated (SKIPs clean when unset; RUNS when the verify step builds + points HELIX_BIN). Determinism proven: consecutive `GOLDEN_UPDATE=1` regens are byte-identical (sha256-equal).
- **OUT-06 / OUT-07 frozen against real CLI output:** `go_to_definition/abs.golden` holds the absolute `<WORKSPACE>/main.go:11:6<TAB>func Helper() {` form; `find_references/color_never.golden` is asserted to contain zero `0x1b` ESC bytes.
- **Parity replaces schema meta-validation:** new untagged `cli_parity_test.go` runs in the default `go test ./...` suite (no daemon/socket/LS), asserting `cli.VerbToolNames()` is set-equal to the live tool registry — every live tool has a generated verb (no CLI-unreachable tool) and no verb is orphaned (no stale `verbs_gen.go`). `schema_test.go` removed.
- **Behavioral chain (OUT-04 + OUT-03):** `TestCLI_E2E_Chain` parses one workspace-local `relpath:line:col` from `search-symbols` stdout and feeds it verbatim into `find-references --path/--line/--column`, asserting exit 0 + result. `TestCLI_E2E_NavSelfContained` asserts `go-to-definition` stdout carries the locus AND the source-line snippet (`Helper`) on the same line.

## Task Commits

Each task committed atomically (test-tier work; both commits are `test(...)`):

1. **Task 1: Re-target contract oracle + parity** — `95522976` (test)
2. **Task 2: Behavioral chain (OUT-04/OUT-03)** — `809c21e8` (test)

## Files Created/Modified

- `test/oracle/contract/golden_test.go` — full re-target: sandbox daemon bringup, `resolveHelixBin` gate, `newGoldenEnv` (seeded Go fixture + gopls require + mcpActivate), `runVerb` (subprocess, CWD=repo, stdout-only capture), `normalizeResponse`/`filterWorkspaceLines`, `goldenCases()` (8 cases incl. `--abs`/`--color=never`), `assertRelpathForm` (no `file://` scheme; relpath:line:col present)
- `test/oracle/contract/cli_parity_test.go` (new) — untagged default-suite parity: `liveRegistryToolNames()` + `TestCLIParity_VerbsMatchRegistry` (set-equality) + `TestCLIParity_VerbToolNamesSorted`
- `internal/cli/cli_e2e_test.go` — `runCLIVerbInDir` (CWD-aware), `seedChainFixture`, `requireGoplsE2E`, `locusLineRe` + `parseFirstLocus`/`parseFirstLocusMatching`, `TestCLI_E2E_Chain`, `TestCLI_E2E_NavSelfContained`
- `test/oracle/contract/testdata/golden/{go_to_definition,find_references,search_symbols,search_in_files,get_symbol_overview,get_hover_info}/success.golden` — regenerated to the frozen terse shape
- `test/oracle/contract/testdata/golden/go_to_definition/abs.golden`, `find_references/color_never.golden` (new) — OUT-07 / OUT-06 flag-variant freezes
- `.gitignore` — ignore generated `/test/oracle/contract/.helix/`

## Decisions Made

- **schema_test.go removed, not kept alongside parity** — the plan's intent is "the parity test REPLACES the MCP schema meta-validation". In v2.0 the agent-facing contract is the CLI verb surface, so verb↔registry lockstep is the load-bearing guarantee; the Draft-2020 inputSchema validation is no longer the contract being frozen.
- **Orphaned golden dirs pruned** — 17 MCP-era golden dirs no longer captured by any test were removed (stale dead goldens are misleading). Kept the 6 the re-targeted oracle captures + `errors/` (owned by `errors_test.go`, untouched).
- **Replicated bringup rather than shared it** — the `cli_e2e_test.go` sandbox helpers are in package `cli_test`; the contract oracle is `contract_test`. Reused the identical `sandbox.NewSandbox/Prepare/StartDaemon` + `forwarder.CallTool` activation pattern rather than exporting a shared harness.

## Deviations from Plan

None — plan executed exactly as written. Two within-scope refinements during normal test bring-up (not deviations):

1. **Workspace-local locus selection in the chain:** `search-symbols --query=Helper` also surfaces stdlib hits whose relative paths (`../../../usr/local/go/src/os/env.go`) sort ahead of `main.go`. The chain test selects the `main.go` locus via `parseFirstLocusMatching(substr="main.go")` — the values are still consumed verbatim; only WHICH emitted locus is chosen. This mirrors the goldens' existing `workspaceOnly` stdlib-noise filter (the plan calls out search_symbols stdlib variance).
2. **.gitignore entry for the generated `.helix/` store:** running the activated daemon writes a cwd-relative `semantic.duckdb` under the test package dir; added an ignore entry following the existing `/internal/cli/.helix/` and `/bench/runtime/.helix/` precedents so the generated artifact is never tracked (T-92-06 spirit: no host-specific artifact committed).

## Issues Encountered

- First chain run fed the FIRST sorted locus, which was a stdlib `os/env.go` hit (not the seeded `Helper`), so `find-references` exited 70. Resolved by selecting the workspace-local locus (see refinement 1). NavSelfContained passed on the first run.

## TDD Gate Compliance

This plan is `type: execute` (not `type: tdd`) — the deliverable IS the test tier, so the RED→GREEN cycle does not apply. Both task commits are `test(...)`. The new tests RUN green with HELIX_BIN set and SKIP cleanly without it (verified both ways).

## Threat Surface

- **T-92-06 (golden path leak):** mitigated — `normalizeResponse` scrubs the workspace dir to `<WORKSPACE>` BEFORE `AssertGolden`, including the `--abs` variant (its golden is host-agnostic). No contributor absolute path is committed; the only host-specific identifier is the fixture module name `goldenfixture`/`chainfixture` (deterministic, in-test).
- **T-92-07 (flaky goldens):** structurally prevented — the 92-02 renderer sorts+dedups loci CLI-side; consecutive regens are sha256-identical.
- **T-92-SC (installs):** none — zero new packages (`git diff go.mod` empty); zero-proto invariant intact (`git diff api/proto/` empty).
- No new threat surface: the oracle drives the existing CLI→gRPC→daemon path under a hermetic sandbox socket.

## User Setup Required

None. `git diff go.mod` empty; `git diff api/proto/` empty.

## Next Phase Readiness

- The terse output contract is now frozen AND tested against the real binary (goldens), the verb surface is parity-locked to the registry (default suite), and the copy-paste / self-contained behaviors are proven (OUT-04/OUT-03). Phase 93's SKILL.md can cite the stable `relpath:line:col<TAB>payload` contract and real verb names with a tested guarantee behind them.
- Golden regeneration command for future maintainers: `HELIX_BIN=<built> GOLDEN_UPDATE=1 go test -tags integration -run TestGolden_CLIStdout ./test/oracle/contract/...`.

## Self-Check: PASSED

- FOUND: test/oracle/contract/golden_test.go, test/oracle/contract/cli_parity_test.go, internal/cli/cli_e2e_test.go
- FOUND: test/oracle/contract/testdata/golden/go_to_definition/abs.golden, find_references/color_never.golden
- FOUND commits: 95522976, 809c21e8
- Verification gate green: `go vet ./...` clean; `go test ./test/oracle/contract/... -run Parity -count=1` green WITHOUT HELIX_BIN; `HELIX_BIN=<built> go test -tags integration -run TestGolden ./test/oracle/contract/...` green (RUNS); `HELIX_BIN=<built> go test -run 'Chain|NavSelfContained' ./internal/cli/...` green (RUNS); consecutive golden regens byte-identical (sha256-equal); `git diff go.mod` empty; `git diff api/proto/` empty.

---
*Phase: 92-terse-output-renderer-re-targeted-contract-oracle*
*Completed: 2026-06-21*
