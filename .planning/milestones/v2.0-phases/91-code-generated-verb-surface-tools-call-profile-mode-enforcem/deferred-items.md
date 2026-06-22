# Phase 91 — Deferred Items

Out-of-scope discoveries logged during execution. Not fixed in the discovering plan
(scope-boundary rule: a plan only fixes issues in its own `files_modified`).

> **STATUS: ALL RESOLVED in-phase (commit `71d02cad`).** The two items below were
> discovered during 91-03 and resolved during the post-wave integration gate by the
> autonomous orchestrator: `switch_mode`/`get_token_budget` were added to the
> `ProfileEnforcementMiddleware` control-plane exemption set (safe — `switch_mode`
> transitions are independently validated by `validateModeTransition`, so a read-only
> profile still cannot escalate), and both tests were updated to the typed-refusal
> contract (`TestProfile_ExcludedToolNotInvocable` now asserts a protocol-level
> `permission_denied`; `TestProfile_ModeAndBudget/switch_mode` now passes via the
> exemption). Verified passing at HEAD. No open work remains.

## Discovered during 91-03 (CLI verb-surface oracle re-point) — RESOLVED

Two pre-existing integration tests fail under the integration build tag. The
failures are caused by the **91-02 `ProfileEnforcementMiddleware`** (which now
returns a typed `serr.PermissionDenied` Go error for an out-of-profile
`tools/call`), NOT by any 91-03 change. They were verified failing on the clean
post-91-02 / pre-91-03 tree (`git checkout`-ed `profile_golden_test.go` back to its
pre-Task-2 form — both still fail). They live in test files OUTSIDE 91-03's
`files_modified` (`test/integration/mode_golden_test.go`,
`test/integration/profile_test.go`), so fixing them here would violate 91-03's
file ownership.

| Test | File | Symptom | Root cause | Suggested owner |
|------|------|---------|------------|-----------------|
| `TestProfile_ExcludedToolNotInvocable` | `test/integration/mode_golden_test.go:95-115` | Uses `callToolExpectError` (expects a protocol-level success with `result.IsError==true`); 91-02 now returns a Go `error` from `CallTool`, so `require.NoError` inside the helper fails. | 91-02 enforcement returns the deny as an `error` (per SEC-01 decision: `errors.Is(err, serr.ErrPermissionDenied)` round-trips), not an `IsError` result. The test pre-dates that contract. | 91-02 follow-up (update the test to `require.Error` + `errors.Is`), or a dedicated 91-04 test-debt pass. |
| `TestProfile_ModeAndBudget/switch_mode` | `test/integration/profile_test.go:22-33` | `callTool(... "switch_mode" ...)` errors with `permission_denied: tool "switch_mode" is not available in profile=full mode=edit`. | `switch_mode` is absent from `full`'s default-mode (`edit`) allowed set, so the new enforcement refuses it. The test assumed `switch_mode` was always callable (old `tools/list`-only filter never gated calls). | 91-02 follow-up: either start the daemon in a mode whose golden includes `switch_mode`, or assert the refusal. |

**Note:** 91-03's own re-pointed oracle (`TestProfile_Contract_Golden`) and the new
`TestProfile_CLI_Surface_Refusal` both pass; the deferred items are unrelated
test-expectation debt left by the 91-02 enforcement landing.
