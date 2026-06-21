---
phase: 81-no-semantic-kernel-flag-e2e-config-gate-test
plan: 02
subsystem: config/profile/cli
tags: [ablation, config-gate, semantic, ABLATE-06]
requires: []
provides:
  - "semantic_index.bench_disabled koanf field (semantic.Config.BenchDisabled)"
  - "Profile.DisableSemanticSubsystem (yaml: disable_semantic_subsystem)"
  - "--disable-semantic-subsystem CLI override (only-when-set -> semantic_index.bench_disabled)"
  - "bench-no-semantic.yaml carries disable_semantic_subsystem: true"
affects:
  - internal/daemon (Plan 81-04 reads both the koanf field and the profile field into effSemanticDisabled)
tech-stack:
  added: []
  patterns:
    - "Distinct opt-in disable gate (D-01) — NOT a reuse of the production Enabled flag"
    - "Only-when-set CLI override (mirror Phase 76 root.go:172-181)"
    - "Precedence CLI > profile YAML > default-off (D-03)"
key-files:
  created: []
  modified:
    - internal/semantic/config.go
    - internal/profile/profile.go
    - internal/cli/root.go
    - internal/profile/profiles/bench-no-semantic.yaml
    - internal/profile/bench_profiles_test.go
decisions:
  - "Koanf key is the NESTED semantic_index.bench_disabled (struct field BenchDisabled on semantic.Config), distinct from the top-level disable_lsp_subsystem path (D-01)"
  - "Profile field name DisableSemanticSubsystem (yaml: disable_semantic_subsystem), mirroring DisableLSPSubsystem"
metrics:
  duration: ~10m
  completed: 2026-06-20
---

# Phase 81 Plan 02: no_semantic Config-Gate Surface Summary

Landed the distinct `semantic_index.bench_disabled` config gate, `DisableSemanticSubsystem` profile field, `--disable-semantic-subsystem` only-when-set CLI override, the `disable_semantic_subsystem: true` field on `bench-no-semantic.yaml`, and flipped the Phase 76 deferral assertion in `bench_profiles_test.go` — establishing the CLI > profile > default-off precedence chain that Plan 04 resolves into `effSemanticDisabled` at the daemon composition root.

## What Was Built

### Task 1 — config field + profile field + CLI override (commit b95ad86b)
- `internal/semantic/config.go`: added `BenchDisabled bool \`koanf:"bench_disabled"\`` adjacent to `Enabled`, with a doc-comment stating it is a DISTINCT ablation gate (D-01), build-but-block under it (D-04), default-off = ENABLED, resolved at the composition root OR'd with the profile field + CLI override (D-02/D-03).
- `internal/profile/profile.go`: added `DisableSemanticSubsystem bool \`yaml:"disable_semantic_subsystem"\`` mirroring `DisableLSPSubsystem`'s doc-comment shape.
- `internal/cli/root.go`: added the `--disable-semantic-subsystem` flag decl, the `runDaemon` read, and the only-when-set override `overrides["semantic_index.bench_disabled"] = true` (note the NESTED koanf path, distinct from the top-level `disable_lsp_subsystem`).

### Task 2 — YAML field + assertion FLIP (commit fd0bf568)
- `internal/profile/profiles/bench-no-semantic.yaml`: added `disable_semantic_subsystem: true` and rewrote the header to state the Phase 81 kernel gate has landed (dropped the Phase 76 D-11/D-12 "tool-filter-only" caveat). The `exclude_tools` list is retained (defense-in-depth: tool-filter AND kernel gate).
- `internal/profile/bench_profiles_test.go`: FLIPPED the assertion from `assert.False(...DisableSemanticSubsystem...)` to `assert.True(...)` (deliberate behavior change per VALIDATION Wave-0). Kept `DisableLSPSubsystem`/`DisableStructuredEditSubsystem` `False` (this arm ablates only semantic). Updated the line-134 comment.

## Key Outputs for Plan 04

- Koanf key path: `semantic_index.bench_disabled` (struct: `semantic.Config.BenchDisabled`)
- Profile field: `Profile.DisableSemanticSubsystem`
- Plan 04 resolves: `effSemanticDisabled := cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem`

## Verification

- `go build ./...` — succeeds
- `go vet ./...` — clean
- `go test ./internal/profile/... ./internal/semantic/... ./internal/cli/... -count=1` — green
- `TestBenchProfiles/bench-no-semantic` — passes with the FLIPPED (true) assertion
- All acceptance-criteria greps satisfied (BenchDisabled+koanf tag; DisableSemanticSubsystem+yaml tag; flag decl+read; nested override path; YAML field; new positive assert)

## Deviations from Plan

None - plan executed exactly as written. The plan's note about a potential coverage/labels test asserting the koanf key set verbatim was checked: `internal/config/loader_test.go` (`TestLoad_SemanticIndexDefaults`) asserts only specific default values positively and does not enumerate the full key set, so adding the new default-false field required no lockstep test change.

## Self-Check: PASSED
- internal/semantic/config.go — FOUND (BenchDisabled present)
- internal/profile/profile.go — FOUND (DisableSemanticSubsystem present)
- internal/cli/root.go — FOUND (disable-semantic-subsystem + semantic_index.bench_disabled present)
- internal/profile/profiles/bench-no-semantic.yaml — FOUND (disable_semantic_subsystem: true present)
- internal/profile/bench_profiles_test.go — FOUND (positive assert present)
- Commit b95ad86b — FOUND
- Commit fd0bf568 — FOUND
