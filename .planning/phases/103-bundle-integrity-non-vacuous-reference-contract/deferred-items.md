# Deferred Items — Phase 103

Out-of-scope discoveries logged during execution (not caused by this phase's changes).

## Pre-existing test failure: cmd/helix-bench TestRunSubcommandWiresDeltaPass

- **Found during:** Task 3 (full `go test ./...` phase gate)
- **Symptom:** `TestRunSubcommandWiresDeltaPass` fails — `result.v2.json` rows
  missing `ablation_deltas` ("delta pass not wired into runBench"). The package
  also surfaces a HuggingFace HTTP 404 (`crosscodeeval` dataset fetch).
- **Out of scope proof:** Reproduced on baseline commit `37c6138f~1` (before any
  Phase 103 commit). Lives in `cmd/helix-bench/run_cmd_test.go`, unrelated to the
  `internal/cli` skill-bundle allowlist change. Consistent with the known
  env-dependent bench-suite behavior (HELIX_BIN / network gating).
- **Action:** Not fixed (SCOPE BOUNDARY). The real gate for this phase is the
  untagged `internal/cli` suite + refgen-check + adopt anchor, all green.
