# Deferred / Out-of-Scope Items — Phase 106

## Pre-existing, out-of-scope test failure (NOT caused by Plan 106-01)

- **Package:** `cmd/helix-bench`
- **Test:** `TestRunSubcommandWiresDeltaPass`
- **Cause:** Network/dataset-dependent — fetches `crosscodeeval` / `repobench`
  parquet datasets from HuggingFace which return **HTTP 404**
  (e.g. `https://huggingface.co/datasets/Vincentvmt/CrossCodeEval/resolve/.../python.parquet`).
- **Why out of scope:** This plan added only `internal/lint/toolsquarantine`,
  `test/oracle/adopt/parity_test.go`, `cmd/vet-tools-quarantine`, the JSON
  corpus, and Makefile wiring. None of these are referenced by `cmd/helix-bench`
  (`grep -rl 'toolsquarantine|parity_cases|vet-tools-quarantine|dspy' cmd/helix-bench/`
  is empty). The failure is a pre-existing env-dependent dataset 404, matching
  the known "integration-tagged / network suite pre-existing failures" pattern.
- **Action:** None taken (Rule: only auto-fix issues directly caused by this
  task's changes). The authoritative Go gates for this plan
  (`./test/oracle/adopt/...` + `./internal/lint/toolsquarantine/...` + `make vet`)
  all pass.

## Plan 106-02 (Wave 2) re-confirmation

- Plan 106-02 changed **zero `.go` files** (only `.py/.jsonl/.json/.txt/.md`
  under `tools/dspy-tune/` + one `.gitignore` line), so it cannot affect
  `cmd/helix-bench`.
- Re-confirmed pre-existing on baseline commit `b7f86c2` (parent of the first
  106-02 commit): `TestRunSubcommandWiresDeltaPass` fails identically with
  "delta pass not wired into runBench" (`run_cmd_test.go:144`) before any
  Wave-2 change, alongside the HuggingFace 404 dataset fetches.
- Authoritative Go gates for 106-02 all green: `go vet ./...`,
  `go test ./test/oracle/adopt/...` (incl. `TestPythonGoParityCorpus`),
  `make vet` (incl. `vet-tools-quarantine`), `go run ./cmd/helix-refgen --check`.
