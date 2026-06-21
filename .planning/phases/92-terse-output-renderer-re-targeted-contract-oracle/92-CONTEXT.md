# Phase 92: Terse Output Renderer + Re-Targeted Contract Oracle - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Default CLI output is terse `relpath:line:col<TAB>payload` — workspace-relative,
1-based coordinates converted from LSP 0-based, deterministically sorted and
deduped, no ANSI off-TTY, honoring `NO_COLOR` — with each verb's output
self-contained enough to act on in one round-trip and copy-paste-able into the
next verb. The v1.5 typed-error taxonomy survives as stable stderr prefixes plus
per-kind exit codes, and global `--json` / `--color` flags exist. This is the
load-bearing product work; the output shape is frozen here so SKILL.md (Phase 93)
can cite real verbs and real output. The contract oracle is re-targeted from MCP
JSON goldens to CLI stdout goldens in the same phase the shape stabilizes.

**Requirements:** OUT-01, OUT-02, OUT-03, OUT-04, OUT-05, OUT-06, OUT-07, TEST-02

**Success Criteria (what must be TRUE):**
1. Piped output contains zero ANSI bytes, coordinates are 1-based
   workspace-relative, and the same query yields byte-identical sorted+deduped
   output across N repeated runs (per-verb goldens).
2. `helix find-symbol` output feeds `helix replace-symbol-body` / `get-callers`
   verbatim, and nav verbs print locus + enclosing symbol + one snippet line so no
   follow-up `Read` is forced (behavioral-oracle confirmed).
3. Each documented error kind surfaces a stable stderr prefix plus a per-kind
   non-zero exit code the agent can branch on.
4. Global `--json` emits compact JSON lines while omitting it yields terse text;
   `--color=never` is byte-equivalent to piped behavior; `--abs` produces absolute
   paths.
5. The re-targeted contract oracle (CLI stdout goldens for ordering / `file:line` /
   error-kind prefix; "typed args → cobra flags" parity replacing MCP schema
   meta-validation) passes against CLI output.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped
per user setting. Use ROADMAP phase goal, success criteria, and codebase
conventions to guide decisions.

### Carried-forward roadmap constraints (from STATE.md)
- **Output shape freezes before SKILL.md (Phase 92 → 93):** the terse renderer is
  the load-bearing product work and precedes SKILL.md authoring (Phase 93's
  decision table cites real verb names + real output shape). Freeze the shape here.
- The v1.5 typed-error taxonomy (already in the codebase) survives as stable
  stderr prefixes + per-kind exit codes — reuse it, do not reinvent.
- Coordinates: LSP is 0-based; CLI output is 1-based workspace-relative. Convert
  at the render boundary.
- Builds on Phase 90/91: the one-shot dial spine + generated verbs. The renderer
  sits between the tool result and stdout for every verb.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Likely touch points:
- `internal/cli/verb.go` + `verbs_gen.go` (the generated verb surface; the renderer
  hooks into `runVerb`'s result handling — currently raw/JSON per Phase 90)
- `internal/errors/` (the v1.5 typed-error taxonomy / `serr` kinds → stderr prefix
  + exit code mapping)
- the per-tool result payloads from the kernel (symbol locations, references, etc.)
  that must be rendered to `relpath:line:col<TAB>payload`
- existing MCP JSON contract goldens (to re-target to CLI stdout goldens for TEST-02)

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
