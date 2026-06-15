---
phase: 75-schema-fairness-contract-tree-skeleton
plan: 05
subsystem: cmd/helix-bench
tags: [cli, cobra, cost-table, tos-attestation, hard-fail-gate, tdd, injected-clock]
requires:
  - "75-01: bench/ six-dir skeleton (bench/datasets/, bench/runners/ present)"
  - "75-02: bench/PROVIDERS.md TOS attestation surface (verify-tos parses its frontmatter)"
  - "75-04: bench/runners.DefaultContract.ModelID (cost-table row model_id must match)"
provides:
  - "cmd/helix-bench: standalone cobra CLI with 5 BENCH-02 subcommands (run, fetch-datasets, doctor, report, validate-cost-table); newRootCmd exported for testing"
  - "cmd/helix-bench.validateCostTable(today, path) — injected-clock strict-decode HARD-FAIL cost-table validator (D-16)"
  - "cmd/helix-bench.verifyTOS(today, path) — injected-clock strict-decode HARD-FAIL TOS-freshness validator (D-16)"
  - "bench/datasets/cost-table.yaml — COST-01/D-13 pricing+staleness contract (the good committed table)"
  - "make validate-cost-table + make verify-tos — CI build gates (no continue-on-error)"
affects:
  - "Every downstream bench phase (80+) hangs subcommands on the helix-bench CLI (run/fetch-datasets/report are stubbed pending those phases)"
  - "CI: validate-cost-table + verify-tos become self-enforcing gates over the cost/TOS contract"
  - "Plan 04 DeprecationGate consumes the deprecation_at field authored on the cost-table row"
tech-stack:
  added: []
  patterns:
    - "cobra CLI tree mirroring cmd/helix-eval: single os.Exit in main(), every subcommand uses RunE"
    - "Injected-clock validators (today time.Time parameter, never time.Now() inside) — D-11 reproducibility"
    - "Strict yaml.v3 decode (KnownFields(true)) — unknown key hard-fails (D-16, T-75-10)"
    - "HARD-FAIL inversion of the warn-only eval-attestation-check (90d blocking vs 180d continue-on-error) — Pitfall 3"
    - "Direct main() dispatch for verify-tos so the BENCH-02 root command count stays at exactly 5"
    - "SilenceUsage on gate commands so validator failures print only the error, not usage text"
key-files:
  created:
    - cmd/helix-bench/main.go
    - cmd/helix-bench/main_test.go
    - cmd/helix-bench/validate_cost_table.go
    - cmd/helix-bench/verify_tos.go
    - cmd/helix-bench/validate_test.go
    - bench/datasets/cost-table.yaml
    - bench/datasets/testdata/cost-table-stale.yaml
    - bench/PROVIDERS-stale.testdata.md
  modified:
    - Makefile
    - .planning/phases/75-schema-fairness-contract-tree-skeleton/deferred-items.md
decisions:
  - "cost-table row model_id pinned to claude-sonnet-4-5-20260128 to match bench/runners.DefaultContract.ModelID (FAIR-02 dated snapshot)"
  - "verify-tos is a Makefile gate + direct-dispatch CLI entry, NOT a 6th root subcommand — keeps the --help count at exactly 5 (BENCH-02)"
  - "Both validators use a 90-day staleness window and exit NON-ZERO (D-16), the deliberate inversion of the 180d warn-only eval-attestation-check"
  - "Shared dateLayout (2006-01-02) and stalenessWindowDays (90) constants live in validate_cost_table.go, reused by verify_tos.go (REFACTOR folded into GREEN)"
  - "verifyTOS treats only `---` blocks containing `provider:` as attestation blocks, skipping the top-of-file field-set table and prose horizontal rules"
metrics:
  duration: ~25min
  completed: 2026-06-15
---

# Phase 75 Plan 05: helix-bench CLI + cost-table + hard-fail validators Summary

Built the `cmd/helix-bench` cobra CLI skeleton (5 BENCH-02 subcommands), authored the
`bench/datasets/cost-table.yaml` pricing+staleness contract (COST-01/D-13), and implemented
two HARD-FAIL validators — `validate-cost-table` and `verify-tos` — wired into the Makefile,
both using strict `yaml.v3` decode and an injected clock so they exit non-zero on
stale/expired/malformed input (D-16) and are deterministically testable.

## What was built

**Task 1 — CLI skeleton + cost-table data (commit `8c06cc77`):**
- `cmd/helix-bench/main.go`: `newRootCmd()` (exported for testing) wiring the five BENCH-02
  subcommands. `doctor` exits 0 on a clean host; `run`/`fetch-datasets`/`report` return a clear
  "not yet implemented" error via RunE. `main()` is the only `os.Exit` site.
- `cmd/helix-bench/main_test.go`: `TestHelixBenchHelpListsFiveSubcommands` (asserts all five names
  present and `len(Commands()) == 5`) and `TestHelixBenchDoctorExitsZero`.
- `bench/datasets/cost-table.yaml`: one Anthropic row with the full D-13 field set
  (provider, model_id, input_per_mtok, output_per_mtok, cached_input_per_mtok, currency,
  valid_until, last_verified, deprecation_at); `model_id` matches `DefaultContract.ModelID`.

**Task 2 — TDD hard-fail validators + Makefile gates (RED `003224ad`, GREEN `a7bb0eff`):**
- RED: `cmd/helix-bench/validate_test.go` (7 validator cases) + stale fixtures
  (`bench/datasets/testdata/cost-table-stale.yaml`, `bench/PROVIDERS-stale.testdata.md`).
  Package failed to compile (validators absent) — RED confirmed.
- GREEN: `validate_cost_table.go` (`CostRow`/`CostTable`, strict decode,
  `validateCostTable(today, path)` hard-failing on unknown key / unparseable date / past
  `valid_until` / >90d-stale `last_verified`) and `verify_tos.go` (`ProviderAttestation`,
  per-block strict frontmatter decode, `verifyTOS(today, path)` hard-failing on malformed block /
  missing field / >90d-stale `attested_on`). Both wrap via RunE; `make validate-cost-table` and
  `make verify-tos` added (no continue-on-error) and appended to `.PHONY`.

## Verification

- `go test ./cmd/helix-bench/...` — 9/9 pass (2 CLI + 7 validator).
- `make validate-cost-table` / `make verify-tos` — exit 0 on the good committed files.
- `go run ./cmd/helix-bench validate-cost-table bench/datasets/testdata/cost-table-stale.yaml`
  and the stale PROVIDERS fixture — both exit non-zero.
- `go vet ./...` — clean.
- RED (`test(75-05)` `003224ad`) precedes GREEN (`feat(75-05)` `a7bb0eff`) in git history.

## TDD Gate Compliance

RED commit (`003224ad`, `test(75-05)`) precedes GREEN commit (`a7bb0eff`, `feat(75-05)`) for the
validators. The REFACTOR gate was folded into GREEN (shared `dateLayout` / `stalenessWindowDays`
constants introduced directly), so no separate `refactor(...)` commit was needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] verify-tos dispatch to keep root subcommand count at 5**
- **Found during:** Task 2
- **Issue:** The plan requires `verify-tos` invocable via `go run ./cmd/helix-bench verify-tos`
  for the Makefile target, while BENCH-02 fixes the root `--help` subcommand count at exactly 5.
  Registering `verify-tos` as a normal cobra subcommand would make `len(Commands()) == 6` and
  break `TestHelixBenchHelpListsFiveSubcommands`.
- **Fix:** `main()` intercepts `os.Args[1] == "verify-tos"` and runs `newVerifyTOSCmd()` directly,
  bypassing the root tree. The command is fully functional from the CLI but not counted as a
  root subcommand.
- **Files modified:** cmd/helix-bench/main.go, cmd/helix-bench/verify_tos.go
- **Commit:** a7bb0eff

**2. [Rule 1 - Polish] SilenceUsage on gate commands**
- **Found during:** Task 2
- **Issue:** On a validation failure cobra dumped the full usage text alongside the error,
  cluttering CI gate output.
- **Fix:** Set `SilenceUsage: true` on the root and the verify-tos command so a non-zero exit
  prints only the error message.
- **Files modified:** cmd/helix-bench/main.go, cmd/helix-bench/verify_tos.go
- **Commit:** a7bb0eff

**3. [Cosmetic] cost-table.yaml list-item formatting**
- **Found during:** Task 1
- **Issue:** The plan acceptance grep `^(provider|...):` counts unindented field-name lines;
  an inline `- provider:` list-item dash prevents `provider` from matching cleanly.
- **Fix:** Put the YAML sequence dash on its own line (`  -` then indented keys) so all nine
  D-13 field names match the acceptance grep (count == 9). Semantically identical YAML.
- **Files modified:** bench/datasets/cost-table.yaml
- **Commit:** 8c06cc77

## Deferred Issues (out of scope — SCOPE BOUNDARY)

`go test ./...` shows two pre-existing failures in `test/bench`
(`TestBenchToolsManifestMatchesRegistry`: 53 live MCP tools vs 47 expected;
`TestToolDescriptionsGoldenFile`: stale golden), last touched in Phases 64/66. `cmd/helix-bench`
is a standalone CLI that registers ZERO MCP tools and `test/bench` does not reference it, so these
are unrelated to this plan. Logged to `deferred-items.md` (Plan 05 section); no fix attempted.

## Known Stubs

`run`, `fetch-datasets`, and `report` subcommands return a clear "not yet implemented (deferred to
a later Phase)" error by design — BENCH-02 only requires the CLI entrypoint to exist with the
five-subcommand surface this wave; their bodies are owned by later bench phases. This is the
intended skeleton scope, not an unfinished deliverable.

## Self-Check: PASSED

All 9 created files exist on disk; all 3 commits (8c06cc77, 003224ad, a7bb0eff) present in git.
