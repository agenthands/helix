---
phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcement
verified: 2026-06-21T18:10:00Z
status: passed
score: 11/11 must-haves verified
overrides_applied: 0
---

# Phase 91: Code-Generated Verb Surface + `tools/call` Profile/Mode Enforcement Verification Report

**Phase Goal:** Every callable tool in the live registry gets a code-generated `helix <verb>` subcommand (committed `*_gen.go` behind a `--check` drift gate, grouped `--help`, arg-struct-derived flags) AND, in the same phase, profile/mode is enforced at the `tools/call` boundary so a read-mode/ci-bot agent cannot invoke destructive edit verbs once all verbs are always-visible.
**Verified:** 2026-06-21T18:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + merged PLAN must-haves)

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Parity test asserts generated subcommand count == live registry tool count, by name; every tool has exactly one verb (VERB-01) | ✓ VERIFIED | `verbs_gen.go` has 50 `toolName:` entries; `verbs_gen_test.go:36` ranges `skill.ToolProviders()` and asserts `len(gen)==len(live)` by name (line 61), never hardcoded. `go test ./internal/cli/ -run Parity\|Verbs` → ok |
| 2 | Editing `*Args` without regen fails CI via `helix-cligen --check`; regen makes it green (VERB-02) | ✓ VERIFIED | `go run ./cmd/helix-cligen --check` → "verbs_gen.go is up to date" exit 0. `Makefile:80 verify-cligen` HARD-FAIL target; `.github/workflows/go-test.yml:102-106` drift-gate CI step |
| 3 | `helix --help` groups verbs by capability; missing required flag errors before daemon dial (VERB-03) | ✓ VERIFIED | `/tmp/helix-91 --help` shows Navigation/Edit/File Operations/Diagnostics/Memory & Workflow groups with flat verbs. `TestVerb_RequiredBeforeDial`-class tests pass (callToolFn fail-if-called seam) |
| 4 | Generator resolves the `*Args` struct for every tool with no manual per-tool table (VERB-04) | ✓ VERIFIED | `cmd/helix-cligen/scan.go` (331 LOC) uses `packages.Load` + AST walk of `AddTool` recovering name→`*Args`; `go test ./cmd/helix-cligen/...` → ok (11.9s); generated `replace_symbol_body` verb derives path/symbol-name/new-body (required) + search-body/receipts (optional) from json tags |
| 5 | `tools/call` for a tool outside resolved AllowedTools is refused with typed PermissionDenied; allowed tool passes through (SEC-01) | ✓ VERIFIED | `profile_enforce.go:138` returns `serr.New(serr.PermissionDenied,...).WithTool(name)`; `go test ./internal/mcp/ -run ProfileEnforce` → ok (9 behaviors incl. refuse/allow/passthrough/nil/typed round-trip) |
| 6 | Enforcement installed AFTER Guardrail / BEFORE LazyInit, LazyInit-first LIFO preserved (SEC-01) | ✓ VERIFIED | `daemon.go`: Guardrail @883 → ProfileEnforce @910 → LazyInit @955. LIFO-order regression test passes (`-run LIFO\|Order`) |
| 7 | `helix replace-symbol-body` under read mode refused with typed error; succeeds under edit mode (SEC-01 live) | ✓ VERIFIED | `HELIX_BIN=/tmp/helix-91 go test -run TestCLI_SecRefusal\|TestCLI_SecAllow` → RAN (0.28s / 5.50s, not SKIP) and PASS. Refusal: `permission_denied: tool "replace_symbol_body" is not available in profile=ci-bot mode=review`; full-profile edit succeeds |
| 8 | Per-profile goldens (re-pointed from tools/list to CLI verb surface) verify each profile; out-of-profile verbs hidden AND refused (SEC-02) | ✓ VERIFIED | `test/integration/cli_verb_surface.go` consumes `cli.VerbToolNames()`; `go test -tags integration -run Profile_Contract\|CLI_Surface` → ok (5 profiles + refusal sub-tests) |
| 9 | `internal/cli.VerbToolNames()` exported, read-only, sorted, no internal aliasing (seam) | ✓ VERIFIED | `verb.go:79 func VerbToolNames`; `TestVerbToolNames_SortedSetEqual` + `TestVerbToolNames_ReadOnly` pass |
| 10 | Core/control-plane tools exempt from enforcement (intentional, not a gap) | ✓ VERIFIED | `profile_enforce.go:57 alwaysAllowedCoreTools` = {ping, echo, activate_project, switch_mode, get_token_budget} — matches design; `switch_mode` independently validated by `validateModeTransition` |
| 11 | Full untagged gate green; in-scope TestProfile_* integration tests pass | ✓ VERIFIED | `go test ./...` → exit 0, no failures. `go test -tags integration -run TestProfile` → all PASS (ExcludedToolNotInvocable, Contract_Golden×5, CLI_Surface_Refusal×5, ModeAndBudget incl. switch_mode) |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `cmd/helix-cligen/scan.go` | go/packages+AST tool→*Args recovery | ✓ VERIFIED | 331 LOC, `packages.Load`, scan tests green |
| `cmd/helix-cligen/render.go` | gofmt-stable verbs_gen.go emit | ✓ VERIFIED | 154 LOC, `format.Source` stable; drift gate exits 0 |
| `cmd/helix-cligen/main.go` | blank-import discipline + --check gate | ✓ VERIFIED | 120 LOC, mirrors docgen |
| `internal/cli/verbs_gen.go` | committed catalog, 50 verbs | ✓ VERIFIED | header marker present, 50 toolName entries, gofmt-clean |
| `internal/cli/verb.go` | VerbToolNames + new flag kinds + wiring | ✓ VERIFIED | flagStringSlice/flagJSON, VerbToolNames(), registerGeneratedVerbs |
| `internal/mcp/profile_enforce.go` | ProfileEnforcementMiddleware | ✓ VERIFIED | 143 LOC, typed PermissionDenied, install-order doc |
| `internal/daemon/daemon.go` | install between Guardrail & LazyInit | ✓ VERIFIED | step 14b.6 @ line 910 |
| `test/integration/cli_verb_surface.go` | CLI verb-surface oracle helper | ✓ VERIFIED | 90 LOC, consumes VerbToolNames() |
| `internal/cli/cli_sec_e2e_test.go` | HELIX_BIN live SEC-01 E2E | ✓ VERIFIED | 223 LOC, tests RAN+PASS |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `cmd/helix-cligen/main.go` | `skill.ToolProviders()` | blank imports == docgen | ✓ WIRED | parity test green confirms no import drift |
| `internal/cli/root.go` | `verbs_gen.go` | registerGeneratedVerbs + 6 cobra groups | ✓ WIRED | root.go:96-102 AddGroup; --help renders groups |
| `test/integration` | `cli.VerbToolNames()` | exported accessor | ✓ WIRED | oracle tests green |
| `daemon.go` | `InstallProfileEnforcementMiddleware` | between Guardrail & LazyInit | ✓ WIRED | line 910 confirmed |
| `Makefile`/CI | `helix-cligen --check` | verify-cligen target + CI step | ✓ WIRED | both present |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Drift gate fresh | `go run ./cmd/helix-cligen --check` | exit 0, "up to date" | ✓ PASS |
| Binaries build | `go build ./cmd/helix ./cmd/helix-cligen` | clean | ✓ PASS |
| Grouped help | `helix --help` | 6 capability groups rendered | ✓ PASS |
| Live read-mode refusal | `HELIX_BIN=... -run TestCLI_SecRefusal` | PASS 0.28s (RAN) | ✓ PASS |
| Live edit-mode allow | `HELIX_BIN=... -run TestCLI_SecAllow` | PASS 5.50s (RAN) | ✓ PASS |
| Full untagged gate | `go test ./...` | exit 0 | ✓ PASS |
| In-scope integration | `go test -tags integration -run TestProfile` | all PASS | ✓ PASS |
| go vet | `go vet ./...` | clean | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| VERB-01 | 91-01 | Every tool has a code-generated verb; parity by name | ✓ SATISFIED | 50 verbs, by-name parity test |
| VERB-02 | 91-01 | --check drift gate + CI | ✓ SATISFIED | gate exits 0; Makefile + CI step |
| VERB-03 | 91-01, 91-04 | grouped --help + required-flag-before-dial + flat verbs | ✓ SATISFIED | --help groups; E2E flat verbs |
| VERB-04 | 91-01 | generator resolves *Args, no manual table | ✓ SATISFIED | AST scan; cligen tests green |
| SEC-01 | 91-02, 91-04 | tools/call enforcement, typed error; live read-refuse/edit-allow | ✓ SATISFIED | unit + live E2E PASS |
| SEC-02 | 91-03 | per-profile CLI verb-surface goldens, hidden AND refused | ✓ SATISFIED | re-pointed oracle PASS |

No orphaned requirements: REQUIREMENTS.md maps exactly VERB-01..04 + SEC-01/02 to Phase 91; all claimed by plans.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| (none) | — | No TBD/FIXME/XXX in any phase-modified file | ℹ️ Info | Completion is auditable |

**Known design exceptions (not stubs):**
- Generated verb `short` help strings are deterministic placeholders ("Run the `<tool>` tool against the warm daemon"). 91-01 SUMMARY documents richer help as Phase 92 scope. Every verb is callable with correct flags/routing today — not goal-blocking.
- 14 empty-flag verbs (memory/repomap/workflow/profile tools registered via dynamic `map[string]any` handler, no typed `*Args`) expose no CLI flags by documented design; args accepted at runtime via the daemon handler. Covered by a documented zero-arg allowlist in the no-empty-flags test.

### Stale Documentation Note (resolved within phase)

`deferred-items.md` lists two integration tests (`TestProfile_ExcludedToolNotInvocable`, `TestProfile_ModeAndBudget/switch_mode`) as failing/deferred. **These were subsequently FIXED inside the phase** by commit `71d02cad` ("fix(91-02): exempt switch_mode/get_token_budget control-plane tools; update enforcement integration tests to typed-refusal contract"), which landed after the 91-03 SUMMARY was written. Both tests now PASS (verified directly). The deferred-items.md is stale documentation, not an open gap — no action required.

### Human Verification Required

None. The previously manual-only SEC-01 live refusal is now an automated HELIX_BIN-gated E2E (`TestCLI_SecRefusal_ReadMode` / `TestCLI_SecAllow_EditMode`), confirmed by the verifier to have RAN (non-zero timings, not SKIP) and PASSED.

### Gaps Summary

No gaps. All 5 ROADMAP success criteria are observably true in the codebase, all 6 requirements satisfied, all key links wired, the drift gate and full untagged test gate are green, and the load-bearing SEC-01 live boundary is proven by an end-to-end subprocess test that the verifier ran independently of the SUMMARY claims.

---

_Verified: 2026-06-21T18:10:00Z_
_Verifier: Claude (gsd-verifier)_
