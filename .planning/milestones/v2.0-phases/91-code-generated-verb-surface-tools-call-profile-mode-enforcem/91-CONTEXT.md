# Phase 91: Code-Generated Verb Surface + `tools/call` Profile/Mode Enforcement - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Every callable tool in the live registry gets a code-generated `helix <verb>`
subcommand (committed `*_gen.go` behind a `--check` drift gate), with grouped
`--help` and arg-struct-derived flags — and, in the *same* phase, profile/mode is
enforced at the `tools/call` boundary so the moment the destructive edit verbs
become always-visible, a read-mode or ci-bot agent cannot invoke them. This closes
the security regression that the loss of `tools/list` filtering would otherwise open.

**Requirements:** VERB-01, VERB-02, VERB-03, VERB-04, SEC-01, SEC-02

**Success Criteria (what must be TRUE):**
1. A parity test asserts the generated subcommand count equals the live registry
   tool count, enumerated by name — every registered tool has exactly one `helix`
   verb, with no manual per-tool edits.
2. Editing a tool's `*Args` struct without regenerating fails CI via the
   `helix-cligen --check` drift gate; a regenerate makes it green.
3. `helix --help` groups verbs by capability (navigation / edit / fileops /
   diagnostics / repomap / memory); a missing required flag errors before the
   daemon is dialed.
4. `helix replace-symbol-body` under read mode is refused with a typed error; the
   same verb succeeds under edit mode.
5. Per-profile goldens (re-pointed from the MCP `tools/list` goldens) verify the
   CLI verb surface for each profile — verbs outside the active profile are hidden
   and refused.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped
per user setting. Use ROADMAP phase goal, success criteria, and codebase
conventions to guide decisions.

### Carried-forward roadmap constraints (from STATE.md)
- **Security gate co-located with verbs:** `ProfileFilterMiddleware` only filters
  `tools/list` — there is no `tools/call` rejection path today. The moment the
  always-visible generated verbs land, profile/mode enforcement at `tools/call`
  MUST ship in the SAME phase, or a read-mode/ci-bot agent could invoke
  destructive edit verbs. Do NOT defer SEC-01/02.
- **Tool count = 53** (live registry, reconciled at v1.12 quick 260617-t7x).
  VERB-01 acceptance is generated-count == live-registry-count enumerated BY NAME,
  not a hardcoded number.
- Build on the Phase 90 one-shot dial spine: generated verbs dispatch through it.
- Keep docgen's blank imports == the daemon's (the v1.12 docgen-drift lesson) — the
  same discipline applies to the new `helix-cligen` generator's imports.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Likely touch points:
- `internal/cli/verb.go` (the Phase 90 verb spine + `verbSpecs` registry stub —
  Phase 91 populates the full catalog via codegen)
- `internal/mcp/` middleware stack (`ProfileFilterMiddleware`, where the
  `tools/call` enforcement path must be added)
- `internal/profile/` (5 profiles, 4 modes) and `internal/config/` (profile resolution)
- `cmd/docgen` (the blank-import discipline analog for the new generator)
- the tool registry (`ToolRegistry`) as the single source of truth for verb enumeration

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase
description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
