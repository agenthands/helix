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

### Module cache repair (environment, not repo)

The sandbox's Go module cache had two incompletely-extracted modules
(`go.opentelemetry.io/auto/sdk@v1.2.1` — only `internal/telemetry/` present;
`github.com/google/jsonschema-go@v0.4.2` — only `meta-schemas/` + `testdata/`
present), which made `go vet ./...` / `go test ./...` fail repo-wide with
"no required module provides package …". Repaired in-place by re-running
`go mod download` for both modules and `go clean -cache`. `go.mod`/`go.sum` were
left unchanged (restored after the verification side-effects). This is an
environment cache issue, not a repository defect.
