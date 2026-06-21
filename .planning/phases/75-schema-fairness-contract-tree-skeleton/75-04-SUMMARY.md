---
phase: 75-schema-fairness-contract-tree-skeleton
plan: 04
subsystem: bench/runners
tags: [fairness, contract, tdd, deprecation-gate, system-prompt-hash]
requires:
  - "75-01: bench/ six-dir skeleton (bench/runners/ present)"
provides:
  - "bench/runners.FairnessContract — single compile-time effective-config source for all benchmark runners"
  - "bench/runners.DefaultContract — pinned dated-snapshot ModelID + temperature + max_tokens + system_prompt_hash + retry/cache"
  - "bench/runners.FairnessContract.Validate() — empty-WaiverReason loader gate (D-10)"
  - "bench/runners.FairnessContract.DeprecationGate(today, deprecationAt) — injected-clock 30d EOL gate (D-11/FAIR-02)"
  - "bench/runners/system_prompt.txt — canonical prompt bytes anchored by sha256 recompute test"
affects:
  - "Phases 80+: every benchmark runner loads its effective model config from DefaultContract and calls Validate() at startup"
  - "Plan 75-05: cost-table.yaml row supplies the deprecationAt string passed into DeprecationGate"
tech-stack:
  added: []
  patterns:
    - "Compile-time config pin (var DefaultContract literal) — RESEARCH Pattern 2"
    - "Injected-clock deprecation gate (today time.Time parameter, never time.Now() inside) — RESEARCH Pattern 3"
    - "sha256 recompute drift guard (pin the hash, commit the prompt file) — T-75-07"
    - "Validate returns error (unit-testable) rather than bare log.Fatal — real startup fatals on non-nil"
key-files:
  created:
    - bench/runners/fairness_contract.go
    - bench/runners/fairness_contract_test.go
    - bench/runners/system_prompt.txt
  modified:
    - .planning/phases/75-schema-fairness-contract-tree-skeleton/deferred-items.md
decisions:
  - "ModelID pinned to claude-sonnet-4-5-20260128 (dated snapshot per FAIR-01 example), never the bare alias claude-sonnet-4-6 (FAIR-02 anti-pattern)"
  - "Temperature 0.0, MaxTokens 8192, Retry{3, 1000ms}, Cache ephemeral_5m as the shared baseline; Overrides seeded empty so every runner shares an identical budget by default"
  - "DeprecationGate boundary is inclusive of exactly-30-days-out (dep.Sub(today) < 30d fails; == 30d passes)"
  - "Validate returns error instead of log.Fatal so the D-10 gate is unit-testable; doc comment instructs real runner startup to treat non-nil as fatal"
metrics:
  duration: ~10min
  completed: 2026-06-15
---

# Phase 75 Plan 04: Single Compile-Time Fairness Contract Summary

Authored `bench/runners/fairness_contract.go` — the single compile-time source of
effective model config every benchmark runner (Phases 80+) loads from: a pinned
`DefaultContract` literal (dated ModelID snapshot, temperature, max_tokens,
system_prompt_hash, retry/cache), a free-text-waiver `Validate()` loader gate that
errors on any override without a `WaiverReason` (D-10), and an injected-clock 30-day
`DeprecationGate` (D-11/FAIR-02), with the system prompt anchored to
`system_prompt.txt` by a sha256 recompute test (T-75-07). Delivered TDD: RED test
commit preceded GREEN implementation commit.

## What Was Built

- **`bench/runners/fairness_contract.go`** (package `runners`):
  - `FairnessContract`, `ModeOverride` (pointer override fields + free-text
    `WaiverReason`/`ApprovedBy`), `RetryPolicy`, `CachePolicy` structs.
  - `var DefaultContract` — `ModelID: "claude-sonnet-4-5-20260128"` (dated, not an
    alias), `Temperature: 0.0`, `MaxTokens: 8192`, `SystemPromptHash` = sha256 of
    `system_prompt.txt`, `Retry{3, 1000}`, `Cache{ephemeral_5m}`, empty `Overrides`.
  - `Validate() error` — iterates `Overrides`; returns an error on any
    whitespace-trimmed-empty `WaiverReason`. Returns nil clean.
  - `DeprecationGate(today time.Time, deprecationAt string) error` — parses the date
    with layout `2006-01-02`; errors (naming model + date) when
    `dep.Sub(today) < 30*24h`; errors on an unparseable date.
  - `systemPromptHash(path)` helper for re-pinning after a prompt edit.
- **`bench/runners/fairness_contract_test.go`** — table tests:
  `TestEmptyWaiverReasonFatal`, `TestDeprecationGate`,
  `TestDeprecationGateRejectsBadDate`, `TestModelIDIsDatedSnapshot`,
  `TestSystemPromptHashMatches`, `TestDefaultContractValidates`.
- **`bench/runners/system_prompt.txt`** — canonical shared bench system prompt;
  sha256 `7873294ae45a555ff47a6164d7c8eba2eb5e13cfa163c623b6556e6ad313d636` pinned in
  `DefaultContract.SystemPromptHash`.

## TDD Gate Compliance

- RED: `2a39cd1d test(75-04): add failing fairness-contract tests` — package failed to
  compile (production file absent), confirming a genuine RED.
- GREEN: `b8fcef79 feat(75-04): pin fairness contract + waiver loader + deprecation gate`
  — all six tests pass.
- REFACTOR: none needed — `dateLayout`/`deprecationWindow` consts and doc comments were
  authored directly in the GREEN file; no follow-up tidy changed behavior.
- Gate order verified in git history (RED `test` precedes GREEN `feat` on
  `bench/runners/`).

## Verification

- `go test ./bench/runners/...` → ok (all six tests pass).
- `go vet ./...` → exit 0.
- `go test ./...` → one pre-existing, unrelated failure in `test/bench`
  (`TestBenchToolsManifestMatchesRegistry`: registry 53 vs manifest 47;
  `TestToolDescriptionsGoldenFile` stale golden). The `bench/runners` package
  registers ZERO MCP tools and `test/bench` does not reference it. Logged to
  `deferred-items.md` (already tracked from plans 75-01/75-03); NOT fixed per SCOPE
  BOUNDARY.
- Acceptance greps all pass: `var DefaultContract = FairnessContract{`, dated ModelID
  (`-\d{8}$`), `Validate()`, `DeprecationGate(today time.Time`, `WaiverReason` +
  `ApprovedBy`, no bare alias in `ModelID`, non-empty `system_prompt.txt`.

## Deviations from Plan

None — plan executed exactly as written. No auto-fixes, no checkpoints, no auth gates.

## Threat Mitigations Applied

- **T-75-07 (prompt drift):** `TestSystemPromptHashMatches` recomputes sha256 of the
  committed prompt and asserts equality with the pinned literal — an unre-pinned prompt
  edit is a hard test failure.
- **T-75-08 (unjustified override):** `Validate()` errors on any empty `WaiverReason`.
- **T-75-09 (accepted, injection via free-text fields):** doc comment documents that
  `WaiverReason`/`ApprovedBy` are display/attestation-only and never exec'd/shelled/queried.

## Known Stubs

None. `Overrides` is intentionally seeded empty (every runner shares the identical
budget by default); this is the fair-by-default state, not a stub — modes add justified
entries downstream.

## Self-Check: PASSED

- FOUND: bench/runners/fairness_contract.go
- FOUND: bench/runners/fairness_contract_test.go
- FOUND: bench/runners/system_prompt.txt
- FOUND commit: 2a39cd1d (RED)
- FOUND commit: b8fcef79 (GREEN)
