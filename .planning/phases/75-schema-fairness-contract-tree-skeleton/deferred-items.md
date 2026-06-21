# Deferred Items — Phase 75

Out-of-scope discoveries logged during execution. NOT fixed (per executor scope boundary).

## From Plan 75-01 (Wave 0 microbench relocation)

### Pre-existing `test/bench` tool-inventory failures (NOT caused by relocation)

`go test ./test/bench/` fails on two tests that are unrelated to the Phase 64
microbench relocation:

- `TestBenchToolsManifestMatchesRegistry` — live MCP registry reports **53 tools;
  expected exactly 47**. New semantic tools (e.g. `explain_cluster`,
  `explain_symbol_deep`, `find_related_symbols`, `get_change_impact_graph`,
  `get_cluster_map`, `get_semantic_context`, `get_semantic_graph_status`,
  `index_semantic_graph`, `refresh_semantic_graph`, `validate_graph_edge`) were
  registered in prior phases without updating `test/bench/tools_manifest_test.go`.
- `TestToolDescriptionsGoldenFile` — `testdata/tool_descriptions.golden` is stale
  for the same reason (golden file lists 47 tools, registry now exposes 53).

**Proof it is pre-existing:** with all of Plan 75-01's changes stashed (pristine
pre-relocation tree at HEAD `582a6a57`), both tests fail identically (53 vs 47).
The relocation is a `git mv` of benchmark *test* files plus doc-comment edits; it
registers zero tools and touches zero tool descriptions.

**Fix owner:** whoever last added the semantic tool registrations, or a dedicated
golden-refresh task:
`go test ./test/bench/... -run TestToolDescriptionsGoldenFile -update` and bump the
expected count in `test/bench/tools_manifest_test.go` (and
`internal/daemon/bootstrap_test.go` if it also asserts a count). Out of scope for a
Wave 0 file relocation.

## From Plan 75-03 (result.v2 schema)

Re-confirmed the same `test/bench` failure above is STILL pre-existing during the
result.v2 schema work: it fails identically at the commit BEFORE this plan (HEAD~2)
and `test/bench` does not reference `bench/schema`. The result.v2 schema package
(`go test ./bench/schema/...`) is fully green. No fix attempted (SCOPE BOUNDARY).

## From Plan 75-04 (fairness contract)

Re-confirmed the same `test/bench` tool-inventory failure above is STILL pre-existing
during the fairness-contract work (registry reports 53 tools; manifest expects 47;
golden file stale). The new `bench/runners` package adds ZERO MCP tools — it is a plain
Go struct + loader + deprecation gate, not a ToolProvider — and `test/bench` does not
reference `bench/runners`. `go test ./bench/runners/...` is fully green. No fix
attempted (SCOPE BOUNDARY).

## From Plan 75-05 (helix-bench CLI + validators)

Re-confirmed the same `test/bench` tool-inventory failure above is STILL pre-existing
during the helix-bench CLI work (`TestBenchToolsManifestMatchesRegistry`: 53 live
tools vs 47 expected; `TestToolDescriptionsGoldenFile`: stale golden). The new
`cmd/helix-bench` package registers ZERO MCP tools — it is a standalone cobra CLI
binary (run/fetch-datasets/doctor/report/validate-cost-table subcommands + a
Makefile-only verify-tos entry), not a ToolProvider — and `test/bench` does not
reference `cmd/helix-bench`. `go test ./cmd/helix-bench/...` is fully green (9/9).
`go vet ./...` is green. No fix attempted (SCOPE BOUNDARY).

### Module cache repair (environment, not repo)

The sandbox's Go module cache had two incompletely-extracted modules
(`go.opentelemetry.io/auto/sdk@v1.2.1` — only `internal/telemetry/` present;
`github.com/google/jsonschema-go@v0.4.2` — only `meta-schemas/` + `testdata/`
present), which made `go vet ./...` / `go test ./...` fail repo-wide with
"no required module provides package …". Repaired in-place by re-running
`go mod download` for both modules and `go clean -cache`. `go.mod`/`go.sum` were
left unchanged (restored after the verification side-effects). This is an
environment cache issue, not a repository defect.
