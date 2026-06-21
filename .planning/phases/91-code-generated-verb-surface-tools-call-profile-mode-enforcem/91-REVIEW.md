---
phase: 91-code-generated-verb-surface-tools-call-profile-mode-enforcem
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - cmd/helix-cligen/main.go
  - cmd/helix-cligen/scan.go
  - cmd/helix-cligen/render.go
  - internal/cli/verb.go
  - internal/cli/root.go
  - internal/mcp/profile_enforce.go
  - internal/daemon/daemon.go
  - internal/eval/sandbox/sandbox.go
  - test/integration/cli_verb_surface.go
  - test/integration/helpers.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 91: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Reviewed the Phase 91 verb-surface generator (`cmd/helix-cligen`), the generated-verb
CLI spine (`internal/cli/verb.go` + `root.go`), and the SEC-01 `tools/call`
profile/mode enforcement middleware (`internal/mcp/profile_enforce.go`) plus its
daemon install site.

**Security verdict (SEC-01): the enforcement design is sound.** The membership
check is an exact-name match against a server-side, defensively-copied
`AllowedTools` snapshot resolved from the active profile/mode — it is not
client-supplied and not bypassable from the CLI. The `alwaysAllowedCoreTools`
exemption set (`ping`, `echo`, `activate_project`, `switch_mode`,
`get_token_budget`) does NOT enable privilege escalation: it is an exact-name map
lookup, `switch_mode` is independently gated by `validateModeTransition` (a
read-only profile still cannot escalate), and `get_token_budget` is read-only
introspection. The deny is returned as a typed `serr.PermissionDenied` error
(errors.Is round-trips), and the refusal message leaks only the tool name plus the
already-public profile/mode — no enumeration of other tools. Install order
(Guardrail @883 → ProfileEnforce @910 → LazyInit @955) yields the documented LIFO
execution with LazyInit-first preserved. `go build`, `go vet`, the unit tests, and
the `helix-cligen --check` drift gate all pass.

The findings below are robustness/maintainability issues, not exploitable holes.
The most consequential is WR-01: enforcement **fails open** (all tools allowed)
when the initial operational mode cannot be resolved to a registered mode.

## Warnings

### WR-01: Enforcement fails open when the initial mode does not resolve

**File:** `internal/daemon/daemon.go:700-715`, `internal/daemon/daemon.go:78-85`, `internal/mcp/profile_enforce.go:108-111`
**Issue:** `initialAllowedTools := resolveAllowedToolsForMode(...)` returns `nil`
whenever `store.Mode(modeName)` misses (daemon.go:82-85). The session is then
created with `AllowedTools: nil`, and `ProfileEnforcementMiddleware` treats a `nil`
whitelist as "all tools allowed" (profile_enforce.go:108-111). The initial mode is
`activeProfile.DefaultMode`, falling back to `"edit"`, overridable by `cfg.Mode`
(daemon.go:699-706). None of these is validated against the registered mode set
before resolution. A profile whose `DefaultMode` names an unregistered mode, or a
config file with a bad `mode:` value, therefore silently disables `tools/call`
enforcement entirely for the whole session — the opposite of the intended
read-only restriction. This is a fail-open authz default, which is the dangerous
direction.
**Fix:** Fail closed. Either (a) make `resolveAllowedToolsForMode` distinguish
"unknown mode" (error / empty non-nil slice) from "mode with no tools", and return
a non-nil empty slice so enforcement denies rather than allows; or (b) validate
`initialMode` against `store.Mode()` at daemon.go:700-706 and refuse to start (or
fall back to a known-restrictive mode) on a miss. Example for (a):
```go
mode, ok := store.Mode(modeName)
if !ok {
    return []string{} // non-nil empty == deny-all, NOT nil (== allow-all)
}
```
Note the `nil == all tools` convention is load-bearing across ProfileFilter and
ProfileEnforce; if you change it, change both. Preferred is (b) at the call site so
the operator gets a startup error instead of a silently-wide-open daemon.

### WR-02: Generator does not dedupe flag names within a verb (latent cobra panic)

**File:** `cmd/helix-cligen/render.go:95-119`, `cmd/helix-cligen/render.go:131-142`, `internal/cli/verb.go:122-135`
**Issue:** `flagNameAndKind` kebab-cases each field's `jsonKey` independently. Two
distinct fields in the same `*Args` struct can collapse to the same flag name —
e.g. a `flagJSON` field `foo` (→ `foo-json`) alongside a string field `foo_json`
(→ `foo-json`), or a scalar `repo-path` colliding with another field after the
`-arg`/`-json` suffixing. The renderer emits both `verbFlag` entries verbatim, and
`newVerbSubcommand` (verb.go:122-135) then registers two cobra flags with the same
name, which panics ("flag redefined") at CLI startup — a hard crash of every
`helix <verb>` invocation, not just the affected verb. I verified the **current**
50-verb registry has zero in-verb flag-name collisions, so this is latent, not
active; but nothing in the generator prevents the next tool from tripping it, and
the `--check` drift gate would happily commit the colliding output.
**Fix:** Detect duplicate flag names per verb in `renderVerbsGen` (after
`flagNameAndKind`) and either deterministically disambiguate (append `-2`, `-3`) or
hard-fail generation with a clear error so the collision is caught at
generate/CI time rather than at the user's CLI:
```go
seen := map[string]bool{}
for _, f := range info.fields {
    flagName, kind := flagNameAndKind(f)
    if seen[flagName] {
        return "", fmt.Errorf("verb %q: duplicate flag --%s", verb, flagName)
    }
    seen[flagName] = true
    // ...emit...
}
```

### WR-03: Generated verb names can collide with hand-written root subcommands

**File:** `internal/cli/verb.go:92-104`, `internal/cli/root.go:146-157`
**Issue:** `registerGeneratedVerbs` calls `rootCmd.AddCommand(sub)` for every entry
in `verbSpecs`. The root command also has hand-written subcommands `setup`,
`status`, `activate`, `deactivate`, `nudge`, `update`, `upgrade` (root.go:146-152),
added BEFORE the generated verbs. cobra's `AddCommand` panics if two children share
the same `Use` token. A future tool named, say, `status` or `activate` (kebab ==
an existing subcommand name) would crash the binary at init for ALL invocations.
There is a `reservedRootFlags` guard for flag shadowing (render.go:17-30) but no
analogous guard for the subcommand-name namespace. Currently no live tool name
collides, so this is latent.
**Fix:** Add a `reservedSubcommands` set to the generator mirroring the
hand-written root subcommands, and either rename or hard-fail generation on a
collision (symmetric with the `reservedRootFlags` treatment). At minimum, document
the invariant alongside `reservedRootFlags` so the two reserved namespaces are
maintained together.

## Info

### IN-01: Dead assignment `_ = rendered` in cligen main

**File:** `cmd/helix-cligen/main.go:67`
**Issue:** `rendered, err := renderVerbsGen(...)` is assigned at line 63 and then
discarded with `_ = rendered` at line 67, yet `rendered` is used at lines 74 and 82.
The `_ = rendered` line is leftover dead code from an earlier scaffold and is
misleading (it reads as if `rendered` were unused).
**Fix:** Delete line 67.

### IN-02: Blank-import set diverges from daemon's, relying on a non-obvious zero-tools invariant

**File:** `cmd/helix-cligen/main.go:26-38`, `internal/daemon/imports.go`
**Issue:** cligen blank-imports `internal/kernel/health` and `internal/kernel/help`
(which the daemon pulls in transitively via direct `health.RegisterTools` /
`help.RegisterTools` calls), and OMITS `internal/skill/guardrails` (which the daemon
blank-imports). The catalogs still match only because `GuardrailsSkill.Tools()`
returns `nil` (guardrails/skill.go:32-33), so guardrails contributes zero verbs.
This is correct today but fragile: if guardrails ever exposes a real tool, cligen's
catalog would silently drop it (init() never runs), and the parity is guarded only
by the SEC-02 integration test, not by the `--check` gate. The MEMORY note
"Keep docgen's imports == daemon's" is the relevant tripwire; cligen matches docgen
but neither matches the daemon exactly.
**Fix:** Either (a) make cligen/docgen blank-import the exact same set as
`internal/daemon/imports.go` (add guardrails) so the surfaces are provably
identical by construction, or (b) add a comment in main.go explaining that the
guardrails omission is safe precisely because `Tools()` returns nil, so the next
maintainer does not assume drift.

### IN-03: Session `Profile` field can mismatch the actually-enforced profile

**File:** `internal/daemon/daemon.go:711-715`, `internal/config/resolve.go:102-105`
**Issue:** The session is built with `Profile: cfg.Profile`, but `AllowedTools` and
`initialMode` are derived from `activeProfile`. `ResolveProfile` falls back to
`store.DefaultProfile()` when `cfg.Profile` does not resolve (resolve.go:102-105),
so a bad/empty `cfg.Profile` yields enforcement based on the default profile while
`snap.Profile` reports the unresolved name. The PermissionDenied refusal message
(profile_enforce.go:139) and the telemetry `profile` label would then name a
profile that does not match the basis of the denial, which can mislead an operator
debugging a refusal. Not a security bypass (AllowedTools is still from
`activeProfile`).
**Fix:** Set `Profile: activeProfile.Name` (or the resolved profile's canonical
name) instead of `cfg.Profile` so the session's reported profile matches the one
whose tools are actually enforced.

### IN-04: `--mode` flag name overloads two distinct concepts

**File:** `internal/cli/root.go:109`, `internal/config/config.go:21-23`
**Issue:** The root `--mode` flag is the *transport* mode (stdio/http/auto,
root.go:109), while `cfg.Mode` is the *operational* mode (read/edit/admin,
config.go:21-23) consumed by the Phase 91 enforcement resolution. `runDaemon`
correctly does NOT wire the transport flag into `cfg.Mode` (it is only settable via
config-file `mode:`), so there is no live bug — but the shared name is a trap: a
future change that binds `--mode` into the daemon overrides would silently feed
`stdio`/`http` into `resolveAllowedToolsForMode`, which would miss and (per WR-01)
fail open.
**Fix:** Rename the transport flag (e.g. `--transport`) or the operational-mode
config key, or add a comment at both sites documenting that the two `mode` concepts
must never be bridged. Fixing WR-01 (fail-closed) also neutralizes the latent risk.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
