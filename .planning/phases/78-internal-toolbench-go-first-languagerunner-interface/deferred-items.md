# Deferred Items — Phase 78 Plan 02

Out-of-scope discoveries during 78-02 execution (NOT fixed; logged per executor
scope boundary). These pre-date this plan's changes and are unrelated to the
bench harness (`bench/`, `cmd/helix-bench`).

## Pre-existing test/bench failures (registry/golden drift)

- `test/bench` `TestBenchToolsManifestMatchesRegistry`: live MCP registry reports
  53 tools; the manifest expects exactly 47. Confirmed present with this plan's
  changes stashed — caused by tool-registry growth in other phases (likely SMTC
  capability tools), not by the bench harness.
- `test/bench` `TestToolDescriptionsGoldenFile`: tool descriptions golden file
  mismatch (`testdata/tool_descriptions.golden` out of date vs the live registry).

Resolution: regenerate the manifest expectation + golden file in a dedicated
tooling-registry sync task (`go test ./test/bench/... -run TestToolDescriptionsGoldenFile -update`
and update the expected tool count). Out of scope for the internal-toolbench
language-axis work.
