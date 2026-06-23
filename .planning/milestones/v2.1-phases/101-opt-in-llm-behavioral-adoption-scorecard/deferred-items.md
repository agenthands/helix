# Deferred Items — Phase 101

## Out-of-scope, pre-existing failures (not caused by Plan 101-02)

### `cmd/helix-bench` TestRunSubcommandWiresDeltaPass — network 404

- **Discovered during:** Plan 101-02 final `go test ./...` run (2026-06-23).
- **Symptom:** FAIL with HTTP 404 fetching external HuggingFace dataset parquet
  files (crosscodeeval python/java/typescript/csharp; repobench python/java),
  e.g. `https://huggingface.co/datasets/Vincentvmt/CrossCodeEval/resolve/<rev>/python.parquet: HTTP 404`.
- **Why deferred:** `git diff <baseline> HEAD -- cmd/helix-bench/` is EMPTY —
  Plan 101-02 touches no bench files. The failure is a network/dataset-
  availability problem (pinned dataset revisions now return 404), not a code
  regression. It is out of scope per the executor SCOPE BOUNDARY (only auto-fix
  issues directly caused by the current task's changes).
- **Scope of 101-02:** test/oracle/{adopt,judge,llm} + internal/cli — all green.
