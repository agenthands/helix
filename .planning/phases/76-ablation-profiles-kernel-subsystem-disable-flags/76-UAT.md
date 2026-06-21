---
status: complete
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
source:
  - 76-01-SUMMARY.md
  - 76-02-SUMMARY.md
  - 76-03-SUMMARY.md
  - 76-04-SUMMARY.md
started: 2026-06-18T14:40:58Z
updated: 2026-06-18T14:40:58Z
mode: developer-facing (CLI/build/test evidence; auto-verified by orchestrator)
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: `go build ./cmd/helix` succeeds from scratch; binary boots.
result: pass
evidence: build exit=0.

### 2. Ablation CLI override flags present
expected: `--disable-lsp-subsystem` and `--disable-structured-edit-subsystem` are accepted flags.
result: pass
evidence: both registered on the root (daemon entrypoint) command — `helix --help` lists them (internal/cli/root.go:75-76). Note: they live on root, not `helix daemon --help`.

### 3. Bench ablation profiles exist and load
expected: bench-full, bench-no-lsp, bench-no-semantic, bench-no-structured-edit profiles exist and load.
result: pass
evidence: 4 YAMLs present under internal/profile/profiles/; TestBenchProfiles PASS (golden tool-surface per arm).

### 4. Fail-closed profile loader
expected: loader rejects unknown mode names at LoadEmbedded.
result: pass
evidence: TestLoadEmbeddedValidatesModes, TestLoaderRejectsUnknownMode, TestValidateModeTransition_* all PASS.

### 5. Structured-edit Unsupported runtime guard
expected: with DisableStructuredEditSubsystem on, replace_symbol_body / insert_before_symbol / insert_after_symbol / fuzzy_edit return serr.Unsupported (greppable `subsystem_disabled:`); flag off falls through normally.
result: pass
evidence: TestStructuredEditDisabled PASS (flag ON returns Unsupported for all tools; flag OFF skips guard).

### 6. no_lsp daemon composition-root wiring (trace-tap)
expected: under disable_lsp_subsystem the daemon wires no LSP seams — zero `lspool.lsp.*` spans; default arm unchanged.
result: pass
evidence: TestNoLSPWiring, TestNoLSPZeroSpans, TestNoLSPDefaultArmUnchanged all PASS (ABLATE-05).

### 7. vet-ablation-leakage import-boundary analyzer
expected: `make vet` runs the analyzer green; bench/runners may not import internal/kernel/lspool or internal/semantic/store.
result: pass
evidence: `make vet` exit=0 across all 5 analyzers including vet-ablation-leakage; analyzer_test green->red flip PASS.

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
