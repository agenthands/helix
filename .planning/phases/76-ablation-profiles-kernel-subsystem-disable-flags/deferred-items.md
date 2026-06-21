# Deferred Items — Phase 76

## Pre-existing failures (out of scope for 76-04)

### test/bench: TestBenchToolsManifestMatchesRegistry + TestToolDescriptionsGoldenFile

- **Discovered during:** Plan 76-04 phase-gate full-suite run (`go test ./...`).
- **Symptom:** `test/bench/main_test.go:100` — "live MCP registry reports 53
  tools; expected exactly 47. Either internal/daemon/bootstrap_test.go is out
  of date or a new tool was added without updating
  test/bench/tools_manifest_test.go." Plus a paired golden-file mismatch in
  `tools_descriptions_test.go`.
- **Root cause:** Tool-count drift (53 vs 47) from semantic-store / cluster-map
  tools added in earlier phases; the `test/bench` golden manifest + descriptions
  golden file were never updated to match the live registry.
- **Verified PRE-EXISTING:** Reproduced at base commit `7dd0c38a` (pause point
  before this session, prior to any 76-04 work) in a clean worktree — same
  "53 tools; expected 47" failure. Plan 76-04 adds NO MCP tools (it adds two CLI
  flags, two SerenaConfig fields, daemon wiring, and two unexported test-only
  RepoMapSkill accessors).
- **Scope decision:** Out of scope for 76-04 (SCOPE BOUNDARY — not caused by this
  plan's changes). The 76-04 package suite
  (`./internal/daemon/... ./internal/config/... ./internal/cli/...`) is green,
  including TestNoLSPWiring / TestNoLSPZeroSpans / TestNoLSPDefaultArmUnchanged.
- **Suggested owner:** A bench-manifest refresh plan (regenerate the
  `test/bench` tool manifest + descriptions golden from the live registry).
