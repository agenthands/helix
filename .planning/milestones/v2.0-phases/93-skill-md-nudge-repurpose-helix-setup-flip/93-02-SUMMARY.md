---
phase: 93-skill-md-nudge-repurpose-helix-setup-flip
plan: 02
subsystem: cli
tags: [nudge, pretooluse-hook, claude-code-hooks, classifier, advisory, helix-verbs]

# Dependency graph
requires:
  - phase: 92-terse-renderer-contract-oracle
    provides: the 50 frozen kebab helix verbs (search-symbols, search-in-files, find-references, get-call-hierarchy, find-files, read-file, get-symbol-overview, replace-in-file, replace-symbol-body) cited verbatim by the steer table
provides:
  - classifyBashTarget code-vs-noncode Bash-command classifier (fail-open, pure-data, no os/exec)
  - per-call PreToolUse advisory steer toward frozen helix verbs via hookSpecificOutput.additionalContext JSON (always exit 0)
  - preToolUseOutput envelope + emitAdvisory helper (json.Marshal, never string-concat)
affects: [93-03 (setup flip wires the PreToolUse hook), 94 (MCP head deletion), skill-trigger behavioral oracle]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Static code-extension allowlist in the hot hook path (no per-call Registry rebuild) to bound classify cost (T-93-05)"
    - "Advisory PreToolUse output as structured JSON envelope (hookSpecificOutput.additionalContext), exit 0 = non-blocking"
    - "Conservative fail-open classifier: any non-code operand demotes the whole command to non-code; no operand / unknown shape → ok=false"

key-files:
  created: []
  modified:
    - internal/cli/nudge.go
    - internal/cli/nudge_test.go

key-decisions:
  - "Used a static code-extension allowlist (codeExtensions) instead of constructing a langregistry.Registry per hook call — the plan explicitly permits this for the hot path and it stays allocation-free (T-93-05 ReDoS/cost mitigation)."
  - "Mixed code+non-code operands classify conservatively as non-code (false suggestion is the failure mode to avoid)."
  - "Kept the existing per-session GrepReadCount tracking but removed the 5-call threshold tip; the per-call advisory supersedes it. No existing test asserted the tip text, so TestNudgeThreshold_* remained valid unchanged."
  - "Bare directory/path operands with no extension (e.g. `grep x internal/cli`) produce no signal → fail open (no suggestion)."

patterns-established:
  - "Pattern: emitAdvisory marshals the PreToolUse envelope and prints to stdout; a marshal error stays silent rather than blocking."
  - "Pattern: steerMessage maps tool-shape → frozen helix verb; bashSteerMessage branches grep/-r/find/cat/sed-i to the closest verb."

requirements-completed: [SKILL-03]

# Metrics
duration: 4min
completed: 2026-06-21
status: complete
---

# Phase 93 Plan 02: PreToolUse Nudge Repurpose (advisory helix-verb steer) Summary

**Repurposed the PreToolUse nudge from a generic 5-call "use find_symbol" tip into a per-call advisory that steers grep/sed/cat/find over positively-identified CODE targets toward the frozen Phase 92 `helix` verbs via `hookSpecificOutput.additionalContext` JSON — fail-open and exit-0 on prose/log/config, unparseable, and no-operand commands.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-06-21T20:04:08Z
- **Completed:** 2026-06-21T20:07:42Z
- **Tasks:** 2 (both TDD: RED → GREEN)
- **Files modified:** 2

## Accomplishments
- `classifyBashTarget(cmd) (isCode, ok bool)` — tokenizes a Bash command string as DATA, finds file operands, classifies code-vs-noncode via a static allowlist; fail-open on no-operand/unparseable.
- Per-call advisory emission: Grep/Read tools and code-target Bash greps now yield a `helix <verb>` suggestion as a valid `hookSpecificOutput.{hookEventName,additionalContext}` JSON object on stdout, exit 0.
- README/log/config greps, no-operand commands, and unparseable Bash stay silent (fail-open) — the nudge never blocks a tool call (T-93-04).
- All citations use real frozen verb names verified against `verbs_gen.go` (search-symbols, search-in-files, find-references, get-call-hierarchy, find-files, read-file, get-symbol-overview, replace-in-file, replace-symbol-body).

## Task Commits

Each task was committed atomically (TDD RED → GREEN):

1. **Task 1 (RED): classifyBashTarget tests** - `3b081961` (test)
2. **Task 1 (GREEN): classifyBashTarget classifier** - `3e5bd15d` (feat)
3. **Task 2 (RED): advisory additionalContext + exit-0 tests** - `c57a21a9` (test)
4. **Task 2 (GREEN): advisory helix-verb steer via additionalContext JSON** - `99ef86e5` (feat)

**Plan metadata:** committed with this SUMMARY (docs)

## Files Created/Modified
- `internal/cli/nudge.go` - Added `classifyBashTarget`, `codeExtensions`/`nonCodeExtensions`/`nonCodeBasenames` sets, `preToolUseOutput` struct, `emitAdvisory`, `steerMessage`, `bashSteerMessage`; rewired `runNudge` to emit per-call advisories and dropped the 5-call threshold tip.
- `internal/cli/nudge_test.go` - Added table-driven `classifyBashTarget` tests (code/non-code/fail-open/mixed/pure-data) and advisory tests (Bash-code-suggest, Bash-noncode-silent, unparseable-fail-open, Grep/Read suggest, always-exit-0, exact-JSON-shape) with stdin/stdout-capture harness.

## Decisions Made
See `key-decisions` frontmatter. Headline: static allowlist over per-call Registry (hot path), conservative mixed-operand demotion, threshold tip removed in favor of per-call advisory.

## Deviations from Plan

None - plan executed exactly as written. The plan permitted either `langregistry.ByExtension` or a static allowlist; the allowlist was chosen per the documented hot-path rationale (not a deviation — an explicitly offered choice).

## Issues Encountered
None. The Go module cache caveat noted in the prompt did not materialize; `go build ./...` and `go vet ./...` were clean throughout.

## Threat Surface

All STRIDE register dispositions honored:
- **T-93-03 (Tampering/Elevation):** `classifyBashTarget` parses the command string purely as DATA via `strings.Fields`; no `os/exec` import in nudge.go (verified by grep) — a malicious command yields at worst a wrong-but-silent classification.
- **T-93-04 (DoS / blocking gate):** `runNudge` returns nil on every path; no exit-2, no `permissionDecision: deny` (verified by grep). Asserted by `TestNudgeAdvisory_AlwaysExitZero`.
- **T-93-05 (DoS / ReDoS):** bounded whitespace tokenization (no regex backtracking); static allowlist, no per-call Registry rebuild.
- **T-93-SC (deps):** `git diff go.mod` / `go.sum` empty (verified).

## Verification Results
- `go test ./internal/cli/ -count=1` — green (incl. classify + advisory table tests).
- `go vet ./internal/cli/...` — clean.
- `go build ./...` — succeeds.
- `git diff go.mod` empty; `git diff go.sum` empty; `git diff api/proto/` empty.
- No `os/exec` import, no `permissionDecision`/`os.Exit`/exit-2 in nudge.go (only in comments).

## Next Phase Readiness
- The PreToolUse hook handler now emits the correct advisory channel; Plan 93-03 (`helix setup` flip) can wire the `Grep|Read|Bash` PreToolUse matcher to `helix nudge` knowing the contract is exit-0 advisory.
- The behavioral oracle (TEST-03) can assert the nudge fires on code targets and stays silent on prose/log.
- No blockers.

## Self-Check: PASSED

---
*Phase: 93-skill-md-nudge-repurpose-helix-setup-flip*
*Completed: 2026-06-21*
