---
phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcement
plan: 03
subsystem: test/integration
tags: [SEC-02, profile, verb-surface, golden-oracle, security-boundary]
requires:
  - "91-01: internal/cli.VerbToolNames() read-only catalog accessor"
  - "91-02: ProfileEnforcementMiddleware (server-side tools/call refusal)"
provides:
  - "Per-profile CLI verb-surface oracle (re-pointed from MCP tools/list)"
  - "Hidden-AND-refused contract assertion for out-of-profile destructive verbs"
affects:
  - "test/integration profile contract suite"
tech-stack:
  added: []
  patterns:
    - "golden file as oracle (reused unchanged); intersect generated catalog with profile-allowed set"
key-files:
  created:
    - test/integration/cli_verb_surface.go
  modified:
    - test/integration/profile_golden_test.go
decisions:
  - "CLI verb surface = intersection(cli.VerbToolNames(), listSessionTools(profile)); byte-identical to profile allowed set due to 91-01 verb parity, so it compares cleanly against the unchanged goldens"
  - "Hidden-AND-refused: refusal probe asserts a Go error from CallTool (91-02 returns serr.PermissionDenied as error, not IsError)"
  - "Refusal probe skips edit-mode profiles where replace_symbol_body is legitimately in-surface"
metrics:
  duration: 3min
  completed: 2026-06-21
  tasks: 2
  files: 2
---

# Phase 91 Plan 03: CLI Verb-Surface Profile Contract Oracle (SEC-02) Summary

Re-pointed the per-profile contract oracle from the MCP `tools/list` surface to the
**CLI verb surface** — the set of `helix <verb>`s visible+invokable under each
profile — reusing the existing `testdata/profiles/*.tools.golden` files unchanged as
the contract, and added a hidden-AND-refused assertion proving an out-of-profile
destructive verb is both absent from the CLI surface and refused server-side by the
91-02 enforcement middleware.

## What Was Built

### Task 1 — CLI verb-surface helper (`test/integration/cli_verb_surface.go`, 02c85801)
`cliVerbSurfaceTools(tb, td)` returns the sorted tool-name list the CLI verb surface
exposes for the daemon's active profile session. Computed as the INTERSECTION of:
- (a) the 91-01 generated verb catalog, read via the exported read-only accessor
  `internal/cli.VerbToolNames()` (no code added to `internal/cli` — the accessor is
  the 91-01 → 91-03 seam), and
- (b) the profile session's authoritative allowed set via `listSessionTools(td.Session)`
  (MCP `tools/list`, which reflects `AllowedTools` through `ProfileFilterMiddleware`).

Two invariants are asserted: every profile-allowed tool has a corresponding verb
(no profile tool unreachable from the CLI), and the surface contains no tool outside
the profile's allowed set. Core protocol tools (`activate_project`, `ping`, `echo`)
are not verbs and not in any golden; they are absent from `VerbToolNames()` and so
excluded by the intersection.

Because every profile-allowed tool has exactly one verb (91-01 parity) and the
profile's allowed set is a subset of the verb catalog (50 catalog tools ⊇ 40-tool
widest golden; the 10 extra are semantic-graph tools in no profile), the intersection
is byte-identical to the profile's allowed set — so it compares cleanly against the
unchanged golden.

### Task 2 — Re-pointed oracle + refusal contract (`test/integration/profile_golden_test.go`, 381f5e36)
- `TestProfile_Contract_Golden`: swapped `tools := listSessionTools(...)` for
  `tools := cliVerbSurfaceTools(t, td)`, keeping the same
  `assertGoldenTools(t, p+"."+defMode, tools)` against the unchanged golden files.
  Green for all 5 profiles.
- `TestProfile_CLI_Surface_Refusal` (new): for each profile at default mode, probes
  the destructive tool `replace_symbol_body`. When it is absent from the profile's
  default-mode golden (ci-bot/review, ide-assistant/read, etc.), asserts it is
  (i) HIDDEN — not in `cliVerbSurfaceTools` — AND (ii) REFUSED — a direct
  `td.Session.CallTool` returns a Go error (91-02's `serr.PermissionDenied`).
  Edit-mode profiles where the tool is legitimately in-surface are skipped for the
  probe.

## Deviations from Plan

None — plan executed as written. The plan named both files in `files_modified`; both
were created/modified exactly as specified, and `internal/cli`, `testdata/profiles/`,
and `api/proto/` are all untouched.

## Deferred Issues (out of scope — scope-boundary rule)

Two pre-existing integration tests fail under the integration build tag, caused by the
**91-02 enforcement middleware** (now returns a Go error for out-of-profile
`tools/call`), NOT by any 91-03 change. Verified failing on the clean post-91-02 /
pre-91-03 tree. They live in files OUTSIDE 91-03's `files_modified`, so fixing them
here would violate file ownership. Logged to
`.planning/phases/91-.../deferred-items.md`:

| Test | File | Root cause |
|------|------|-----------|
| `TestProfile_ExcludedToolNotInvocable` | `test/integration/mode_golden_test.go` | Expects `result.IsError` (protocol success); 91-02 returns a Go `error`. |
| `TestProfile_ModeAndBudget/switch_mode` | `test/integration/profile_test.go` | `switch_mode` absent from `full`'s default (`edit`) allowed set → enforcement refuses the call. |

Suggested owner: a 91-02 follow-up / 91-04 test-debt pass.

## Verification

- `go build -tags integration ./test/integration/` — OK
- `go vet ./test/integration/` and `go vet -tags integration ./test/integration/` — clean
- `go test -tags integration ./test/integration/ -run 'Profile_Contract|CLI_Surface' -count=1` — ok (all 5 profiles + refusal sub-tests pass)
- `git status --porcelain testdata/profiles/` — empty (no golden regenerated)
- `git status --porcelain internal/cli/` — empty (no code added to internal/cli)
- `git diff --exit-code api/proto/` — empty (zero-proto invariant held)

## Threat Model Outcomes

- **T-91-10 (EoP — out-of-profile verb surfaced/invokable):** mitigated — the
  re-pointed golden oracle asserts CLI surface == per-profile allowed set, and the
  hidden-AND-refused sub-assertion proves server-side denial.
- **T-91-11 (Tampering — golden regenerated to mask drift):** mitigated — no `-update`
  run; goldens unchanged (verified clean).
- **T-91-12 (Info disclosure — surface helper leaks tool names):** accept — the helper
  intersects with the profile's authoritative allowed set, so it cannot surface tools
  the profile filter hides.
- **T-91-SC (supply chain):** mitigated — zero new external packages.

## Self-Check: PASSED

- FOUND: test/integration/cli_verb_surface.go
- FOUND: test/integration/profile_golden_test.go
- FOUND commit 02c85801 (Task 1)
- FOUND commit 381f5e36 (Task 2)
