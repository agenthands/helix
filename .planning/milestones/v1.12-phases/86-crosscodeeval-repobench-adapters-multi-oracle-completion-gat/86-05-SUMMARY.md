---
phase: 86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat
plan: 05
subsystem: bench
tags: [canary, contamination, aggregator, fetch-datasets, additive-column, score-time]
requires:
  - bench/aggregator (Phase 82/85: LeaderRow + reduceLanguageRows additive discipline)
  - bench/datasets/crosscodeeval.Fetch (Phase 86 Plan 03)
  - bench/datasets/repobench.Fetch (Phase 86 Plan 04)
provides:
  - bench/canary (minimal forward-compatible contamination-canary probe)
  - aggregator LeaderRow.CanaryPassRate (additive column, score-time reduced)
  - helix-bench fetch-datasets (real network-gated command)
affects:
  - Phase 89 reporter (consumes Sentinel + DocKey* + CanaryPassRate)
tech-stack:
  added: []  # zero new dependencies (stdlib + prior-wave leaf packages only)
  patterns:
    - "score-time canary derivation in the aggregator path (NO loader edit)"
    - "additive ciValue column mirroring Phase 85 ByLanguage / rowLanguage"
    - "flat pooled rate (no RNG) so the IN-03 determinism contract is untouched"
    - "network-gated CLI fetch over pinned-constant adapter Fetch funcs"
key-files:
  created:
    - bench/canary/canary.go
    - bench/canary/canary_test.go
    - bench/aggregator/aggregate_canary_test.go
    - cmd/helix-bench/fetch_datasets_test.go
  modified:
    - bench/aggregator/report.go
    - bench/aggregator/aggregate.go
    - cmd/helix-bench/main.go
decisions:
  - "CanaryPassRate exists on LeaderRow + is populated, but is NOT rendered into leaderboard.md (Phase 89 reporter owns rendering) — keeping the existing leaderboard/cost goldens byte-for-byte, exactly the ByLanguage discipline."
  - "Canary 'pass' == clean (completion did NOT echo the sentinel); rate = #clean / #rows-with-a-completion-key; a cell with no completion key yields a NULL ci (em-dash), never a fabricated 0."
  - "CanaryPassRate is a flat pooled rate (degenerate [point,point] ci), NOT a BCa CI — it consumes ZERO RNG so it cannot perturb the locked IN-03 metric-order/presence determinism contract; the bootstrapped canary CI is downstream Phase 89."
  - "The contamination flag is DERIVED at score time inside the aggregator (rowCanary -> canary.IsContaminated over the open `completion` doc key); the CCE/RepoBench loader.go sources are NOT edited (Plans 03/04 own them)."
metrics:
  duration: ~7min
  tasks: 2
  files: 7
  completed: 2026-06-21
---

# Phase 86 Plan 05: Canary Probe + Aggregator CanaryPassRate + Real fetch-datasets Summary

Minimal forward-compatible contamination-canary probe (`bench/canary`: novel `Sentinel`, deterministic `InjectPrompt`, teeth-bearing `IsContaminated`) + an ADDITIVE `CanaryPassRate` aggregator column derived at SCORE TIME from the open `completion` doc key (no loader edit) + a real network-gated `helix-bench fetch-datasets` invoking the CrossCodeEval + RepoBench `Fetch` funcs — closing Phase 86 SC#4. Zero new dependencies; hermetic tests are the sole proof.

## What Was Built

### Task 1 — `bench/canary` minimal contamination-canary probe (TDD RED→GREEN)
- `Sentinel`: a documented, fixed, known-novel marker (`HELIX-CANARY-<uuid>-END`) engineered not to occur in any legitimate completion.
- `InjectPrompt(prompt)`: deterministically appends a delimited canary line embedding the sentinel; preserves the original prompt verbatim.
- `IsContaminated(completion)`: returns true IFF the completion echoes the sentinel verbatim — a clean (or empty) completion returns false (teeth, not a rubber stamp).
- `DocKeyCompletion` (`"completion"`) and `DocKeyContaminated` (`"canary_contaminated"`) pinned so the Phase 89 reporter and the Task 2 aggregator read the same key names (forward-compatibility).
- Pure leaf package (stdlib `strings` only); no bench dependency, so it is safely consumable at score time without dragging the loaders in.

### Task 2 — additive `CanaryPassRate` column + real `fetch-datasets` (TDD RED→GREEN)
- `LeaderRow.CanaryPassRate ciValue` added additively in `report.go` (mirrors the `ciValue` column convention).
- `rowCanary(r)` in `aggregate.go` reads the open `completion` doc key and runs `canary.IsContaminated` — score-time derivation, mirroring `rowLanguage`/`rowModelID` exactly.
- `reduceCanaryRate(loaded, tasks, mode)` pools a clean-fraction over rows-carrying-a-completion; absent completion data ⇒ NULL `ciValue` (em-dash). It is wired into the per-mode loop AFTER the determinism-locked metric reductions and consumes no RNG.
- The existing `leaderboard.golden.md` / `cost_quality.golden.md` byte-stable goldens still pass unchanged (CanaryPassRate is populated on the struct but not rendered — Phase 89 owns rendering, exactly the ByLanguage discipline).
- `cmd/helix-bench/main.go`: the `fetch-datasets` `notYetImplemented` stub is replaced with a real `RunE` invoking `crosscodeeval.Fetch(PinnedRev, lang)` over `crosscodeeval.Languages` and `repobench.Fetch(PinnedRev(lang), lang)` over `repobench.Languages`, reporting cached bytes per (adapter, language), non-fatal on a partial mirror gap (errors iff every fetch fails).

## Deviations from Plan

None — plan executed exactly as written. (The "additive column is populated but not rendered" choice is the explicit ByLanguage precedent the plan instructed to mirror, not a deviation.)

## Authentication Gates

None.

## Verification

- `go build ./...` — pass (incl. `./cmd/helix`).
- `go vet ./bench/... ./cmd/helix-bench/...` — clean; `make vet` (all 7 vettools) — clean.
- `go test ./bench/canary/... ./bench/aggregator/... ./cmd/helix-bench/...` — green offline (canary teeth proven; CanaryPassRate=0.5 over 1-clean/1-contaminated; absent-completion ⇒ em-dash; existing aggregator goldens byte-for-byte; fetch-datasets registered + not the stub).
- `go test ./...` — green offline (exit 0); the live HF-fetch leg (`TestFetchDatasetsLive`) and the adapter live-fetch tests SKIP without `HELIX_BENCH_NETWORK`.
- `git diff bench/datasets/crosscodeeval/loader.go bench/datasets/repobench/loader.go` across this plan's commits — EMPTY (loaders untouched; Plans 03/04 own them).

## Known Stubs

None. The `fetch-datasets` live fetch is intentionally network-gated (not a stub): offline it returns a real fetch error, never the deferred-phase sentinel; the hermetic test asserts the stub is gone.

## Commits

- `15bfd250` test(86-05): RED — minimal contamination-canary probe
- `9412db95` feat(86-05): GREEN — minimal forward-compatible contamination-canary probe
- `7f996c5b` test(86-05): RED — additive CanaryPassRate column + implemented fetch-datasets
- `87e126a6` feat(86-05): GREEN — additive CanaryPassRate column + real fetch-datasets CLI

## TDD Gate Compliance

Both tasks followed RED→GREEN with separate `test(...)` then `feat(...)` commits, verified in git log. No REFACTOR commit was needed (GREEN was clean).

## Self-Check: PASSED

- FOUND: bench/canary/canary.go
- FOUND: bench/canary/canary_test.go
- FOUND: bench/aggregator/aggregate_canary_test.go
- FOUND: cmd/helix-bench/fetch_datasets_test.go
- FOUND commit: 15bfd250, 9412db95, 7f996c5b, 87e126a6
- Loaders untouched by this plan: confirmed (empty diff over plan commits)
