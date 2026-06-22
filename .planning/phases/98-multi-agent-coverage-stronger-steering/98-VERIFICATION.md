---
phase: 98-multi-agent-coverage-stronger-steering
verified: 2026-06-22T00:00:00Z
status: human_needed
score: 6/6 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Codex/Gemini LIVE round-trip (A1/A2 end-to-end). Run `helix setup codex` into a real Codex config dir and `helix setup gemini-cli` into a real Gemini config dir, then exercise the live runtimes."
    expected: "(a) Codex loads AGENTS.md AND fires the .codex/hooks.json PreToolUse hook, invoking `helix nudge` and surfacing the `additionalContext` advisory; (b) Gemini loads GEMINI.md into context. If the live runtime rejects the pinned hooks.json byte-shape/path, adjust writeCodexHooks / the codex path helpers and re-pin the golden."
    why_human: "Requires a live external Codex/Gemini runtime to confirm the file-discovery path and hook execution. The byte-shape + paths are pinned from documented conventions (STACK.md A1/A2, HIGH confidence) and all Helix-controlled parts are asserted automated, but no live external install was exercised this session. This is a legitimate external-runtime UAT item — NOT a gap."
---

# Phase 98: Multi-Agent Coverage + Stronger Steering — Verification Report

**Phase Goal:** Non-Claude agents (Codex, Gemini, generic) receive the shared generated reference plus a per-agent instruction file installed without clobbering user content, Codex gets the reused advisory nudge hook, and the broadened steering classifier reaches more standard-tool shapes while provably never firing on legitimately-correct prose/log/config use.
**Verified:** 2026-06-22
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Bash `sed -i` over a code file steers to `helix replace-in-file` | ✓ VERIFIED | `isGrepReadTool` Bash arm broadened, token-anchored on `fields[0] ∈ {…,sed,cat}` (nudge.go:441-455). Behavioral: `echo '{...sed -i...pkg/s.go}' \| helix nudge` → `helix replace-in-file` advisory, exit 0. |
| 2 | Bash `cat` over a code file steers to `helix read-file` | ✓ VERIFIED | bashSteerMessage cat→read-file (nudge.go:191-192). Behavioral: `cat internal/edit.go \| helix nudge` → `helix read-file` advisory, exit 0. |
| 3 | Prose/log/config targets produce NO advisory and exit 0 (STEER-03) | ✓ VERIFIED | Golden negative controls (nudge_test.go:555-571) + behavioral: `sed -i …README.md`, `cat app.log` → no advisory, exit 0. Silence enforced by classifyBashTarget returning non-code (untouched). |
| 4 | Every broadened sed/cat shape returns nil (exit 0 / fail-open); NO exit-2/deny path | ✓ VERIFIED | grep for `permissionDecision`/`os.Exit(2)`/`deny` in nudge.go/prime.go/activate.go/setup_agents.go finds only explanatory comments. Behavioral: every fired advisory exits 0 with 0 deny tokens. |
| 5 | SessionStart priming emits the terse use-X-not-Y matrix, byte-capped, single-constant, fail-open (STEER-02) | ✓ VERIFIED | prime.go: `primingMatrix` compiled const, `sessionPrimingText()` pure, cap 2048, no refgen banner; wired at activate.go:77. Drift guard maps every cited verb to VerbToolNames(). (Runtime stdout emission gated on live daemon — see Behavioral note.) |
| 6 | Generated reference surfaceable to non-Claude agents via a shared accessor (AGENT-01) | ✓ VERIFIED | `EmbeddedReference()` (skill.go:60-66) returns byte-identical `--check`-gated reference.md; differs from SKILL.md; agent instruction body points at `helix get-tool-help` (real frozen verb). See WARNING below on accessor wiring. |
| 7 | `helix setup` appends sentinel-delimited block without clobbering user content, idempotent, Codex AGENTS.md ≤32 KiB (AGENT-02) | ✓ VERIFIED | writeAgentInstructions (setup_agents.go:198): strip-then-append (exactly one block), cap trims appended block not user content, errors rather than truncates, path-contained, atomic. Behavioral: real `helix setup codex` re-run keeps 1 block. |
| 8 | Codex writes AGENTS.md + hooks.json invoking `helix nudge`; Gemini/generic instruction-file only, no fabricated hook (AGENT-03) | ✓ VERIFIED | Real `helix setup codex` → `.codex/hooks.json` command `[<bin>,"nudge"]`, matcher Bash, no deny. Real `helix setup gemini-cli` → GEMINI.md, NO hook artifact. |
| 9 | codex flipped to also-install; vscode/jetbrains/opencode remain teardown-only | ✓ VERIFIED | codex in ValidArgs (setup.go:31) + clientRegistry. VSCode/JetBrains/OpenCode Register = `teardownOnlyRegister`, 0 writeAgentInstructions calls each. |

**Score:** 6/6 must-haves verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/cli/nudge.go` | isGrepReadTool Bash arm token-anchored on fields[0] incl sed/cat | ✓ VERIFIED | Broadened, classifyBashTarget/bashSteerMessage untouched as planned |
| `internal/cli/nudge_test.go` | firing golden rows + negative controls + flipped DEFER-97-01 (silent loop removed) | ✓ VERIFIED | 7 firing shapes, floor ≥7, silent loop deleted, revert-and-fail proven |
| `internal/cli/prime.go` | primingMatrix const + sessionPrimingText() capped builder | ✓ VERIFIED | Compiled const, pure, cap 2048 |
| `internal/cli/prime_test.go` | cap + fail-open + drift guard | ✓ VERIFIED | TestSessionStartPriming passes |
| `internal/cli/activate.go` | emits priming on stdout after activation line | ✓ VERIFIED | activate.go:77 sessionPrimingText() best-effort |
| `internal/cli/skill.go` | EmbeddedReference() accessor | ✓ VERIFIED (see WARNING) | Byte-identical reference.md; only referenced by tests, not by a setup path |
| `internal/cli/setup_agents.go` | idempotent writer + 32 KiB cap + path containment + Codex hooks writer | ✓ VERIFIED | All primitives real, atomic temp+rename, filepath.Rel containment |
| `internal/cli/setup_agents_test.go` | round-trip/idempotency/cap/codex-hook/no-gemini-hook goldens | ✓ VERIFIED | All anti-vacuity sub-cases pass |
| `internal/cli/setup_clients.go` | CodexRegistrar + gemini-cli/generic flips | ✓ VERIFIED | Confirmed end-to-end |
| `internal/cli/setup.go` | codex in ValidArgs | ✓ VERIFIED | Present |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| nudge.go isGrepReadTool | nudge.go bashSteerMessage | broadened gate reaches sed/cat branches | ✓ WIRED | Behavioral: advisory fires for sed/cat over code |
| activate.go | prime.go | runActivate calls sessionPrimingText() | ✓ WIRED | activate.go:77 |
| setup_clients.go CodexRegistrar | setup_agents.go | Register calls writeAgentInstructions + writeCodexHooks | ✓ WIRED | Real setup writes both files |
| setup_agents.go Codex hooks | nudge.go | hooks.json command `[<bin>,"nudge"]` reuses runNudge | ✓ WIRED | Real hooks.json confirmed |
| setup_agents.go agent block | skill.go EmbeddedReference | block points at reference / get-tool-help | ⚠️ PARTIAL | Block points at `helix get-tool-help` (live, real verb); EmbeddedReference() accessor exists but is not invoked by any setup path (test-only reference) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| sed -i over code steers | `echo '{…sed -i…pkg/s.go}' \| helix nudge` | `helix replace-in-file` advisory, exit 0 | ✓ PASS |
| cat over code steers | `echo '{…cat internal/edit.go}' \| helix nudge` | `helix read-file` advisory, exit 0 | ✓ PASS |
| log negative control silent | `echo '{…cat app.log}' \| helix nudge` | no advisory, exit 0 | ✓ PASS |
| prose negative control silent | `echo '{…sed -i…README.md}' \| helix nudge` | no advisory, exit 0 | ✓ PASS |
| no deny path | grep permissionDecision/deny on fired advisory | 0 matches | ✓ PASS |
| real codex setup | `helix setup codex` in temp dir | AGENTS.md + .codex/hooks.json `[<bin>,nudge]`, no deny | ✓ PASS |
| codex idempotency | re-run setup codex | exactly 1 helix block | ✓ PASS |
| gemini no hook | `helix setup gemini-cli` | GEMINI.md written, NO hook artifact | ✓ PASS |
| DEFER-97-01 non-vacuity | flip bash-cat golden to wrong verb | TestNudgeShapeGolden goes RED, restored green | ✓ PASS |
| helix activate (priming runtime) | `helix activate` (no daemon in sandbox) | exit 0 (fail-open); priming emission gated on live daemon | ? SKIP (needs daemon) |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| STEER-01 | 98-01 | Broaden classifier to steer standard-tool Bash to specific verb, exit-0/fail-open | ✓ SATISFIED | Truths 1,2,4 |
| STEER-02 | 98-01 | SessionStart priming matrix, size-capped, fail-open | ✓ SATISFIED | Truth 5 |
| STEER-03 | 98-01 | Negative controls: nudge does NOT fire on prose/log/config | ✓ SATISFIED | Truth 3 |
| AGENT-01 | 98-02 | Generated reference installable/surfaceable for non-Claude agents (shared markdown, no bespoke engine) | ✓ SATISFIED | Truth 6 (+ WARNING) |
| AGENT-02 | 98-02 | Idempotent sentinel-append, no-clobber, Codex AGENTS.md ≤32 KiB | ✓ SATISFIED | Truth 7 |
| AGENT-03 | 98-02 | Codex hook → helix nudge; Gemini/generic instruction-file only, no fabricated hook | ✓ SATISFIED | Truth 8 |

All 6 requirement IDs from PLAN frontmatter are mapped to Phase 98 in REQUIREMENTS.md (lines 112-117) and all 6 are claimed by the two plans. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER in any phase-modified file | — | Clean |

### Gates

| Gate | Result |
|------|--------|
| `go build ./cmd/helix` | RC=0 |
| `go vet ./internal/cli/...` | clean |
| `go test ./internal/cli/...` | green |
| `go test ./...` (full suite, run once) | RC=0, no failures |
| `go run ./cmd/helix-refgen --check` | "reference.md is up to date" |
| `git diff go.mod` | empty (zero-dep invariant) |
| Commits 2f90cfb6 / cf5bb109 / e87adcdc / 5ddd38be | all exist with matching messages |

### Human Verification Required

**1. Codex/Gemini LIVE round-trip (A1/A2 end-to-end)**

**Test:** Run `helix setup codex` into a real Codex config dir and `helix setup gemini-cli` into a real Gemini config dir, then exercise both live runtimes.
**Expected:** (a) Codex loads AGENTS.md AND fires the `.codex/hooks.json` PreToolUse hook, invoking `helix nudge` and surfacing the `additionalContext` advisory; (b) Gemini loads GEMINI.md into context. If the live runtime rejects the pinned hooks.json byte-shape/path, adjust `writeCodexHooks` / the codex path helpers and re-pin the golden.
**Why human:** Requires a live external Codex/Gemini runtime to confirm file-discovery path and hook execution. Byte-shapes/paths are pinned from documented conventions (STACK.md A1/A2, HIGH confidence) and every Helix-controlled part is asserted automated, but no live external install was exercised this session. This is a legitimate external-runtime UAT item, NOT a gap.

### Gaps Summary

No gaps. All automated checks pass, all 6 requirements satisfied, all build/vet/test/refgen gates green, zero-dep invariant held. The DEFER-97-01 flip was independently proven non-vacuous (flipping a golden mapping produced a real RED, then restored). The single outstanding item is the Codex/Gemini live external-runtime round-trip — correctly recorded as `human_needed` (NOT asserted automated-passed), which is the expected and correct outcome for an external-runtime confirmation. Per the status decision tree, the presence of one human-verification item makes the overall status `human_needed`, not `passed` and not `gaps_found`.

**WARNING (non-blocking):** `EmbeddedReference()` (AGENT-01's shared accessor) is exported and byte-correct but is only referenced by `TestEmbeddedReference` — no setup/registrar path invokes it to write reference.md to a non-Claude location. Non-Claude agents reach the shared reference content via the instruction-file pointer to `helix get-tool-help` (a real frozen verb backed by the same registry). This satisfies the plan's explicit truth ("surfaceable to non-Claude agents via a shared accessor") and AGENT-01's "shared markdown reference, not a per-agent bespoke skill engine," but if a future intent is to physically install reference.md into the Codex/Gemini config dir, that wiring (EmbeddedReference → a registrar write) does not yet exist. Flagged for awareness, not a blocker.

---

_Verified: 2026-06-22_
_Verifier: Claude (gsd-verifier)_
