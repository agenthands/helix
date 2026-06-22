---
phase: 96-address-v2-0-tech-debt
reviewed: 2026-06-22T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - internal/daemon/telemetry.go
  - internal/daemon/telemetry_test.go
  - internal/cli/setup_clients.go
  - internal/cli/setup_test.go
  - internal/cli/nudge.go
  - internal/cli/nudge_test.go
  - internal/kernel/help/skill_adapter.go
  - internal/kernel/help/tools.go
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: issues_found
---

# Phase 96: Code Review Report

**Reviewed:** 2026-06-22
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found (info-only; no Blocker/Warning)

## Summary

Adversarial review of the four independent v2.0 tech-debt fixes (TD-01..TD-04).
All four changes are correct, well-scoped, and well-tested. `go build ./...`,
`go vet`, and `go test` on the three affected packages (`internal/daemon`,
`internal/cli`, `internal/kernel/help`) are all green.

Verification performed against the phase-context contracts:

- **TD-01 (`validateAdminAddr`)** — Confirmed parity with `validateGRPCAddr`
  (`internal/daemon/grpc_tcp.go:85`). The admin validator deliberately does NOT
  short-circuit on `addr == ""` (unlike `validateGRPCAddr`, which guards its own
  empty-addr no-op); the empty-addr no-op is correctly owned by `listenAdmin`
  (`telemetry.go:42-44`), and the `{empty → wantErr:true}` test row stays green.
  The new `""` host branch rejects `:9090`/`:0` wildcard binds; the
  `IsUnspecified()` belt-and-suspenders guard rejects `0.0.0.0`/`::`. Both error
  strings retain the `v1.3` auth-roadmap reference (`telemetry.go:122,129`), and
  the test asserts `errHas: "v1.3"` for the LAN/zero/DNS rows. Verified `:9090`,
  `:0`, `[::]:9090` all return errors via the test matrix.

- **TD-02 (dead `mergeJSONConfig` removal)** — `grep -rn mergeJSONConfig
  --include=*.go` returns zero hits across the tree: the helper and its four
  tests are fully removed with no orphans. The live sibling
  `removeFromJSONConfig` (~18 teardown callers) is intact, and
  `uninstallSkill`/`installSkill` (skill.go) still resolve. `go vet` exit 0
  proves no unused imports remain in `setup_clients.go` or `setup_test.go` after
  the deletions (`encoding/json`, `fmt`, `os`, `path/filepath` are all still
  consumed by `removeFromJSONConfig` and the remaining tests).

- **TD-03 (grep-family pattern skip)** — The leading-PATTERN skip is gated on a
  `grepFamily` set containing exactly `{grep, rg, ag, egrep, fgrep}`; cat/sed/find
  are excluded, so their first operand is still treated as a file (verified:
  `cat main.go`→code, `find . -name '*.go'`→code, `find src/main.go`→code, all
  unchanged). Tokenization remains a bounded `strings.Fields` whitespace split
  with no `os/exec` and no regex — DATA-only, fail-open (`$(whoami).go` and
  `grep x f.go && rm -rf /` probed: no side effects, deterministic return). The
  advisory path stays exit-0 (`TestNudgeAdvisory_AlwaysExitZero`).

- **TD-04 (string-literal rewording)** — The four "any MCP tool" → "any Helix
  tool" edits in `skill_adapter.go` (Description + BriefDescription) and
  `tools.go` (the `AddTool` Description and the `Registry().Register` Description)
  are pure string-literal changes with no behavior impact. The docgen-rendered
  field (`skill_adapter.go` Description) and the BriefDescription are reflected in
  `test/bench/testdata/tool_descriptions.golden:28` (`Get detailed usage
  documentation for a Helix tool`), which was regenerated consistently.

No correctness, security, or maintainability defects were found that rise to
Blocker or Warning. The two Info items below are non-blocking observations.

## Info

### IN-01: `grep -- foo.go` and `grep -e PAT file` rely on incidental tokenization

**File:** `internal/cli/nudge.go:366-413`
**Issue:** The flag handling treats any token with a leading `-` as a flag and
the first remaining non-flag token as the pattern. Two consequences, both benign:
(1) `grep -- foo.go` skips `--` as a "flag", then treats `foo.go` as the pattern,
yielding fail-open (no file operand found) — a false negative. (2) `grep -e foo
bar.go` happens to work only because the `-e` *value* `foo` is the first non-flag
token and is consumed as the pattern; `grep -e foo` (no file) correctly fails
open. Both outcomes are within the documented fail-open contract (a missed
advisory is acceptable; a false advisory is the failure mode being avoided), so
this is acceptable as-is.
**Fix:** No change required. If precision is ever desired, document the `--` and
`-e VALUE` corner cases in the function comment, or note them in the steer-table
research doc. Do not add flag-value consumption logic — that would increase
parsing surface for marginal benefit and risks regressing the DATA-only / no
backtracking property (T-93-05).

### IN-02: `bashSteerMessage` and `classifyBashTarget` independently re-tokenize the same command

**File:** `internal/cli/nudge.go:180-185` and `internal/cli/nudge.go:337-338`
**Issue:** `steerMessage` calls `classifyBashTarget(cmd)` (which runs
`strings.Fields(cmd)`), and on a positive result calls `bashSteerMessage(cmd)`
which runs `strings.Fields(cmd)` again to recover `fields[0]`. This is a tiny
duplicate tokenization on the advisory hot path. Not a correctness issue — both
agree on `fields[0]` — and out of v1 performance scope, so noting for
maintainability only.
**Fix:** Optional: have `classifyBashTarget` return the leading verb (or have
`steerMessage` pass `fields[0]` to `bashSteerMessage`) to tokenize once. Low
priority; the current form is readable and the cost is negligible.

---

_Reviewed: 2026-06-22_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
