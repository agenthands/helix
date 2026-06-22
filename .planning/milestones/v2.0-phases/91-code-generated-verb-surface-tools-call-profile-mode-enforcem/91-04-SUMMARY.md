---
phase: 91
plan: 04
subsystem: cli-e2e
tags: [SEC-01, VERB-03, e2e, profile-enforcement, helix-bin-gated]
requires:
  - "91-01 flat verb surface (call parent removed; verbs root-attached)"
  - "91-02 ProfileEnforcementMiddleware (typed serr.PermissionDenied on tools/call)"
  - "v1.12 internal/eval/sandbox harness (StartDaemon with --profile)"
provides:
  - "internal/cli/cli_sec_e2e_test.go — live SEC-01 refusal/allow E2E proof (CLI->gRPC->daemon round trip)"
  - "re-pointed Phase 90 dial oracle (flat helix <verb>, no call prefix)"
affects:
  - "internal/cli/cli_e2e_test.go"
  - "internal/cli/cli_sec_e2e_test.go"
tech-stack:
  added: []
  patterns:
    - "HELIX_BIN-gated //go:build !windows real-subprocess E2E"
    - "newE2EFixtureWithProfile: profile passed as StartDaemon 4th arg (--profile)"
    - "robust typed-error assertion accepting either Kind prefix or human message"
key-files:
  created:
    - internal/cli/cli_sec_e2e_test.go
  modified:
    - internal/cli/cli_e2e_test.go
decisions:
  - "Flat verb name is the FULL generated name (search-in-files), not the short call-alias (search); flag is --pattern not --query"
  - "Refusal assertion accepts permission_denied OR 'is not available in profile' so it is robust to wire stringification"
  - "ci-bot (default mode review) is the read-restricted profile; its exclude_tools omits replace_symbol_body"
  - "Allow-path asserts ONLY the absence of the permission_denied signal (not exit 0), since full-profile call may still fail on its own tool semantics"
metrics:
  duration: 6min
  tasks: 2
  files: 2
  completed: 2026-06-21
---

# Phase 91 Plan 04: Live SEC-01 Refusal Oracle + Flat-Verb Re-point Summary

Stood up the live, real-subprocess proof that the SEC-01 tools/call profile/mode
boundary survives the full CLI→gRPC→daemon→back round trip, and re-pointed the
Phase 90 dial oracle from the removed `helix call <verb>` parent to the flat
`helix <verb>` surface that 91-01 shipped.

## What Was Built

### Task 1 — Re-point the Phase 90 E2E oracle to flat `helix <verb>` (VERB-03)

`internal/cli/cli_e2e_test.go`:
- `runCLIVerb` dropped the `call` prefix — it now execs `helix <verb> --flag=...`
  directly (root-attached verbs after 91-01).
- `TestCLI_ParallelColdSingleDaemon` inline argv dropped its `"call"` element.
- `representativeVerbName`: `search` → `search-in-files` (the full generated verb
  name; the short `search` alias only existed under the removed `call` parent).
- Introduced `representativeVerbFlag = "--pattern="` (the generated flag for
  search_in_files); the call-search alias accepted `--query`, the root verb does
  not.
- Stale `helix call` doc/message strings tidied.

The done criterion `grep -n '"call"' internal/cli/cli_e2e_test.go` returns no
literal `"call"` argv string.

### Task 2 — Live SEC-01 refusal + allow E2E (SEC-01)

New `internal/cli/cli_sec_e2e_test.go` (`//go:build !windows`, HELIX_BIN-gated):
- `newE2EFixtureWithProfile(t, runID, profile)` — profile-parameterized bringup
  that passes the profile as `StartDaemon`'s 4th arg (`--profile`), the only path
  by which profile/mode reaches the daemon (the CLI never passes mode per call).
- `TestCLI_SecRefusal_ReadMode` (T-91-13): a REAL `helix replace-symbol-body`
  subprocess under the read-restricted `ci-bot` profile EXITS NON-ZERO and its
  combined output carries the typed permission_denied refusal — the deny survived
  the full round trip rather than being swallowed CLI-side.
- `TestCLI_SecAllow_EditMode` (T-91-14): the SAME verb under the default `full`
  profile is NOT gate-refused (it actually performs the edit), proving the refusal
  is profile-conditioned, not unconditional.

## Verification Evidence (tests RAN, not skipped)

Built `helix`, then:

```
HELIX_BIN=$(pwd)/helix go test ./internal/cli/ \
  -run 'TestCLI_SecRefusal|TestCLI_SecAllow|TestCLI_E2E_OneShot$' -count=1 -v
=== RUN   TestCLI_E2E_OneShot
--- PASS: TestCLI_E2E_OneShot (0.28s)
=== RUN   TestCLI_SecRefusal_ReadMode
--- PASS: TestCLI_SecRefusal_ReadMode (0.28s)
=== RUN   TestCLI_SecAllow_EditMode
--- PASS: TestCLI_SecAllow_EditMode (5.53s)
PASS
ok  	github.com/agenthands/helix/internal/cli	6.101s
```

Non-zero timings (not `0.00s SKIP`) confirm the HELIX_BIN gate was satisfied and
the tests actually executed (T-91-15: a green-but-skipped test would mask an
unproven boundary).

**Refusal output (verbatim):**
```
calling replace_symbol_body: calling tool "replace_symbol_body": calling "tools/call": permission_denied: tool "replace_symbol_body" is not available in profile=ci-bot mode=review
```

**Allow output (verbatim):**
```
Replaced body of "Target" in /tmp/helix-eval-sec-allow-.../cli-sec-e2e/full/repo/main.go

Post-edit verification: OK (no errors)
```

Also green: `TestCLI_WarmReuseSLO` (observed_p50=13.2ms, recorded SLO 66.1ms),
`TestCLI_E2E_OneShotCleanShutdown`, `TestCLI_ParallelColdSingleDaemon`.

Project gates:
- `go vet ./...` — clean.
- `go build ./...` — clean.
- `git diff --exit-code api/proto/` — empty (zero-proto invariant held).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Flat verb name and flag differed from the plan's assumptions**
- **Found during:** Task 1
- **Issue:** The plan assumed re-pointing was a one-line drop of the `"call"`
  prefix with `representativeVerbName="search"` / `--query`. After the drop the
  oracle failed with `unknown command "search"` and (once renamed)
  `unknown flag --query`. 91-01 attached verbs to root under their FULL generated
  names (`search-in-files`) with generated flag names (`--pattern`); the short
  `search`/`--query` form only existed as a `call`-parent alias that 91-01 removed.
- **Fix:** Updated `representativeVerbName` to `search-in-files` and introduced
  `representativeVerbFlag = "--pattern="`, replacing all `--query=` usages.
- **Files modified:** internal/cli/cli_e2e_test.go
- **Commit:** 2179a752

## Threat Coverage

| Threat ID | Disposition | Proven by |
|-----------|-------------|-----------|
| T-91-13 (deny swallowed on round trip) | mitigated | TestCLI_SecRefusal_ReadMode — non-zero exit + typed permission_denied on stderr |
| T-91-14 (edit-mode false-positive refusal) | mitigated | TestCLI_SecAllow_EditMode — full profile NOT gate-refused (edit succeeds) |
| T-91-15 (green-but-skipped masks boundary) | mitigated | tests RAN with real timing under HELIX_BIN; evidence above |
| T-91-SC (npm/pip/cargo install) | mitigated | zero new external packages; reused existing sandbox harness |

## Commits

- 2179a752: test(91-04): re-point Phase 90 E2E oracle to flat `helix <verb>` (VERB-03)
- a69b90cb: test(91-04): live SEC-01 E2E — read-mode refuses, full-mode allows a destructive verb

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/cli/cli_sec_e2e_test.go
- FOUND: internal/cli/cli_e2e_test.go (modified)
- FOUND commit: 2179a752
- FOUND commit: a69b90cb
