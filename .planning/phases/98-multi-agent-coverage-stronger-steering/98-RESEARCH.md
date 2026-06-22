# Phase 98: Multi-Agent Coverage + Stronger Steering - Research

**Researched:** 2026-06-22
**Domain:** Coding-agent adoption layer — broaden the PreToolUse steering classifier (STEER-01/02/03) and add per-agent instruction-file + Codex-hook install (AGENT-01/02/03), on top of the mature Helix `internal/cli/` skill+nudge+setup surface
**Confidence:** HIGH — every code claim below was read from current in-tree source this session; agent-runtime conventions (Codex 32 KiB cap, Gemini no-hook) carried from the milestone STACK.md (verified against official docs there) and re-flagged where they need a planning-time confirmation.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

The discuss phase was skipped (`workflow.skip_discuss: true`); all implementation choices are at Claude's discretion. The following constraints are carried from research (SUMMARY.md / ARCHITECTURE.md / PITFALLS.md) and the Phase 97 deviation, and the planner MUST honor them:

- **DEFER-97-01 lands here:** Phase 97's nudge golden pinned Bash `sed -i`→`replace-in-file` and `cat`→`read-file` as *silent* sub-cases because the classifier's Bash arm only matched grep/find/rg/ag. STEER-01 broadens the Bash recognizer so those shapes steer; once they fire, flip the Phase 97 golden's `bash-sed-i-silent`/`bash-cat-silent` sub-cases to asserting the specific verb (keep the contract non-vacuous and still green).
- Steering stays **advisory exit-0 / fail-open** — a deny/block (exit 2) path is an explicit anti-feature. STEER-03 negative-control golden rows MUST prove the nudge does NOT fire on prose/log/config/build-output (e.g. `grep TODO README.md`).
- Codex's `PreToolUse` hook uses the SAME `additionalContext` envelope as Claude — reuse the existing `internal/cli/nudge.go` steering engine (one engine, two runtimes), do not build a second.
- Gemini CLI has **NO** PreToolUse-equivalent — instruction-file (`GEMINI.md`) steering only; do NOT fabricate a hook. IDE/generic also instruction-file only.
- AGENT-02: per-agent instruction files via **idempotent sentinel-delimited append** that never clobbers a user's existing `AGENTS.md`/`GEMINI.md`/generic file (Codex `AGENTS.md` ≤32 KiB cap). Re-run must not duplicate the Helix block. Multi-agent is a shared markdown reference + thin per-agent file — NOT a per-agent bespoke skill engine.
- SessionStart priming (STEER-02) presents the terse "use X not Y" matrix once per session, SKILL-04-style size-capped, fail-open.
- Reuse existing setup machinery in `internal/cli/setup_clients.go` / `setup*.go` (today the 4 non-Claude clients get MCP-teardown only — flip to also-install the reference + instruction file).

### Claude's Discretion

All implementation choices (exact recognizer broadening, where new code lands, helper signatures, sentinel format) at Claude's discretion within the locked constraints above.

### Deferred Ideas (OUT OF SCOPE)

None — discuss phase skipped. (Note: AGENT-04 "first-class skill/hook install for additional runtimes beyond Codex/Gemini/generic" is a v2 requirement, not this phase.)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STEER-01 | Broaden the PreToolUse nudge's code-target classifier to steer more standard-tool invocations (grep/sed/cat/find/Read-shaped Bash) toward the specific equivalent `helix` verb, preserving advisory exit-0 / fail-open. | The gate is `isGrepReadTool` (`nudge.go:429-441`), NOT `classifyBashTarget` or `bashSteerMessage`. Its Bash arm matches only grep/find/rg/ag; broaden it to sed/cat. `bashSteerMessage` already has correct sed/cat branches (`nudge.go:191-197`) — they are currently dead for Bash callers. sed→`replace-in-file`/`replace-symbol-body`, cat→`read-file`/`get-symbol-overview` (verbs verified in `bashSteerMessage`). |
| STEER-02 | SessionStart priming surface presents the terse "use X not Y" matrix once per session, size-capped (SKILL-04-style), fail-open. | The SessionStart hook today runs `helix activate` (`setup_hooks.go:30-43`) which prints one line to stdout (`activate.go:73`). Stdout from a SessionStart `command` hook is added to agent context. STEER-02 extends the activate output (or adds a sibling command) to emit the terse matrix, capped like SKILL-04. |
| STEER-03 | Negative-control coverage proves the nudge does NOT fire on legitimately-correct standard-tool use (prose/log/config/build-output), enforced by golden classifier rows. | `classifyBashTarget` already classifies non-code via `nonCodeExtensions`/`nonCodeBasenames` (`nudge.go:307-319`); `TestClassifyBashTarget_NonCodeTargets` (`nudge_test.go:191-207`) + the folded negative control in `TestNudgeShapeGolden` (`nudge_test.go:529-537`) exist. STEER-03 extends these to the broadened sed/cat shapes. |
| AGENT-01 | The generated verb reference is installable for non-Claude agents (shared markdown reference, not a per-agent bespoke skill engine). | `reference.md` (32 KB, generated by `cmd/helix-refgen`, embedded via `//go:embed skills/helix/*` in `skill.go:21`) already ships for Claude. AGENT-01 surfaces the SAME bytes to non-Claude agents (either copy the file, or fold it into the appended instruction block). `EmbeddedSkillBody()` returns SKILL.md only; a new accessor (e.g. `EmbeddedReference()`) reads `skills/helix/reference.md` from `embeddedSkillFS`. |
| AGENT-02 | `helix setup` writes/updates per-agent instruction files (Codex `AGENTS.md` ≤32 KiB, Gemini `GEMINI.md`, generic) via idempotent sentinel-delimited append that never clobbers a user's existing file. | NO instruction-file writer exists today; the 4 non-Claude clients are teardown-only (`teardownOnlyRegister`, `setup_clients.go:94-102`). New work: a sentinel-delimited append helper + per-client wiring. The hook-merge code (`mergeHooksIntoSettings` + `filterOutHelixEntries`/`isHelixManaged`, `setup_hooks.go`) is the in-repo idempotent-block precedent to mirror. |
| AGENT-03 | Codex's `PreToolUse` hook wired to `helix nudge` reusing the existing advisory envelope; agents with no PreToolUse-equivalent (Gemini, IDE, generic) get instruction-file steering only — no fabricated hook. | `nudge.go` emits the camelCase `hookSpecificOutput.additionalContext` envelope (`nudge.go:128-149`) Codex shares. New work: a `codex` client (NOT in `clientRegistry()` today — only 7 clients, none is codex) writing `AGENTS.md` + a Codex `hooks.json` pointing at `helix nudge`. Gemini/generic write the instruction file only; a test asserts no Gemini hook artifact. |
</phase_requirements>

## Summary

Phase 98 is **two small, well-scoped edits against existing seams plus one genuinely new client.** Thrust A (STEER-01/02/03) broadens an already-correct steering engine; Thrust B (AGENT-01/02/03) flips four teardown-only clients to also-write a per-agent instruction file and adds a new Codex client that reuses the nudge. There is **zero new Go dependency** and **no new architectural layer** — this matches the milestone-wide finding.

The single most important code discovery, contradicting the naïve reading of "broaden `classifyBashTarget`": **STEER-01's real edit is in `isGrepReadTool` (`nudge.go:429-441`), not `classifyBashTarget` or `bashSteerMessage`.** `runNudge` only reaches the steering path when `isGrepReadTool()` returns true (`nudge.go:105`); its Bash arm matches only grep/find/rg/ag, so a Bash `sed`/`cat` never enters steering — even though `bashSteerMessage` (`nudge.go:191-197`) already has correct sed→`replace-in-file` and cat→`read-file` branches. Those branches are **dead for Bash callers today**. STEER-01 = add `sed`/`cat` to `isGrepReadTool`'s Bash recognizer; `classifyBashTarget` and `bashSteerMessage` need little-to-no change (both already handle sed/cat operands and verbs). The Phase 97 golden's `bash-sed-i-silent`/`bash-cat-silent` sub-cases (`nudge_test.go:543-558`) then flip from "asserted silent" to "asserted firing the specific verb" — that flip is DEFER-97-01.

For Thrust B, there is **no instruction-file writer and no Codex client in the tree today**. The 7 clients in `clientRegistry()` (`setup_clients.go:43-53`) are claude-code, vscode, jetbrains, claude-desktop, gemini-cli, opencode, generic — codex is absent (it exists only as a *profile* name, `root.go:133`). The idempotent-append pattern to mirror already exists in two forms: `mergeHooksIntoSettings` (sentinel = `"helix_managed": true` on hook entries; `setup_hooks.go:77-123`) and `installSkill`'s two-pass atomic temp+rename (`skill.go:159-236`). The new sentinel-delimited markdown append (`<!-- helix:begin -->`…`<!-- helix:end -->`) is a fresh helper but follows the same idempotent-filter-then-rewrite shape.

**Primary recommendation:** STEER-01 — broaden `isGrepReadTool`'s Bash arm to recognize `sed`/`cat` (the minimal correct edit); keep `classifyBashTarget`/`bashSteerMessage`/exit-0 untouched. STEER-02 — extend the SessionStart `activate` stdout (or a sibling `nudge`-family command) to emit the terse matrix, capped à la SKILL-04. AGENT-01/02/03 — add a `setup_agents.go` with one sentinel-delimited idempotent append helper, a new `codex` client (AGENTS.md + Codex hooks.json → `helix nudge`), and flip gemini-cli/generic (and vscode/jetbrains/opencode if in scope) to also-write the instruction file; reuse `nudge` for Codex only; assert no Gemini hook.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Broadened Bash classifier (STEER-01) | CLI hook (`internal/cli/nudge.go`) | — | The nudge is a per-call hot path invoked by the agent runtime's PreToolUse; it parses Bash command strings as DATA, never executes. Pure CLI-layer concern. |
| SessionStart priming (STEER-02) | CLI hook (`activate`/sibling) + setup wiring | Agent runtime (consumes stdout) | The hook command runs in the helix CLI; the agent runtime surfaces its stdout into session context. Helix owns the emitted text + size cap; the runtime owns when it loads. |
| Negative-control goldens (STEER-03) | CLI test (`internal/cli/nudge_test.go`) | — | Pure unit-level classifier assertion; no runtime, no daemon. |
| Shared reference for non-Claude agents (AGENT-01) | CLI setup (`internal/cli/`) + embed.FS | — | Same generated bytes already embedded; only the install destination differs per agent. |
| Per-agent instruction-file append (AGENT-02) | CLI setup (`setup_agents.go` NEW) | Filesystem (user's AGENTS.md/GEMINI.md) | Idempotent file mutation owned by `helix setup`; the file is consumed by the external agent runtime. |
| Codex PreToolUse hook (AGENT-03) | CLI setup (writes hooks.json) + reused `nudge` | Codex runtime (invokes the hook) | Helix writes the hook config + reuses the one nudge engine; Codex invokes `helix nudge` and consumes the same envelope Claude does. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `encoding/json` | go 1.25.x | Marshal the PreToolUse envelope + Codex hooks.json + read/write JSON configs | Already the entire codebase's JSON path; `emitAdvisory` (`nudge.go:139-149`) and `mergeHooksIntoSettings` (`setup_hooks.go`) use it; NEVER string-concatenate JSON (T-34-01 spirit). |
| Go stdlib `embed` | go 1.25.x | `//go:embed skills/helix/*` already bundles SKILL.md + reference.md (`skill.go:21`) | AGENT-01 reuses the same embedded FS; a new `EmbeddedReference()` accessor reads `skills/helix/reference.md`. |
| Go stdlib `os`/`path/filepath`/`strings` | go 1.25.x | File writes, path containment, Bash tokenization | The whole setup + nudge surface is stdlib-only; the zero-dep invariant (`git diff go.mod` empty) MUST hold. |
| `github.com/spf13/cobra` | v1.9.1 (in tree) | `setup`/`nudge`/`activate` subcommands | Existing CLI framework; a new `codex` ValidArg + (optional) priming command plug into it. |
| `github.com/stretchr/testify` | (in tree) | `assert`/`require` in goldens | Used by every `internal/cli/*_test.go`. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/fatih/color` | (in tree) | `SetupPrinter` colored stderr output (`setup_output.go`) | New per-agent install paths emit Success/Info/DryRunAction via the same printer. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Sentinel-delimited markdown append (`<!-- helix:begin -->`) | JSON-keyed block like the hooks (`helix_managed:true`) | AGENTS.md/GEMINI.md are markdown prose files users hand-edit; an HTML-comment sentinel is the idiomatic, invisible-in-render delimiter. JSON keys don't apply to a `.md` file. Recommend the sentinel. |
| Extend `activate` stdout for STEER-02 | A new `helix prime`/`helix session-start` command | Extending `activate` keeps the SessionStart hook a single command (already installed at `setup_hooks.go:30-43`); a new command means a second hook entry + a setup change. Extending activate is lower-surface; either is acceptable (Claude's discretion). |
| New `codex` ClientRegistrar | Reuse the generic client with `--output` | Codex needs BOTH an AGENTS.md write AND a hooks.json write — distinct from generic's stdout config. A dedicated registrar is cleaner and lets `ValidArgs` advertise `codex`. |

**Installation:** No `go get`. Verify the tree builds + gates pass:
```bash
go build ./cmd/helix
go vet ./...
go test ./...
go run ./cmd/helix-refgen --check   # reference.md drift gate (must stay clean)
git diff go.mod                      # MUST be empty (zero-dep invariant)
```

**Version verification:** No new packages — nothing to verify against a registry. The only embedded asset touched is `internal/cli/skills/helix/reference.md` (32421 bytes on disk, generated by `cmd/helix-refgen`); its `--check` gate is the authority.

## Package Legitimacy Audit

> Not applicable — this phase installs **zero external packages**. All work uses Go stdlib + already-vendored modules (`cobra`, `testify`, `fatih/color`). `git diff go.mod` MUST remain empty (locked milestone invariant). No legitimacy check needed.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                         helix setup <client>            (internal/cli/setup.go runSetup)
                                  │
        ┌─────────────────────────┼──────────────────────────────────────┐
        │ claude-code/-desktop    │ codex (NEW)        │ gemini-cli/generic/(vscode/jb/oc)
        ▼                         ▼                    ▼
  installSkill(bundle)      writeAgentInstructions   writeAgentInstructions   ← AGENT-01/02
  (SKILL.md+reference.md)   (AGENTS.md, append)      (GEMINI.md / AGENTS.md, append)
  mergeHooksIntoSettings    writeCodexHooks(hooks.json→`helix nudge`)  [NO hook] ← AGENT-03
  (SessionStart/PreToolUse  │                         │
   /Stop)                   │                         │
        │                   └──────────┬──────────────┘
        ▼                              ▼
  .claude/skills/helix/    sentinel-delimited idempotent append
        │                  <!-- helix:begin --> … <!-- helix:end -->  (preserve user content)
        │
   ── runtime (per tool call) ──────────────────────────────────────────────
        ▼
  Claude PreToolUse hook ─► `helix nudge` ─┐        Codex PreToolUse hook ─► `helix nudge`
                                           ▼                                         │
                                   runNudge (nudge.go:64)                            │
                                           ▼  (ONE engine, two runtimes — AGENT-03)  │
                            isGrepReadTool? ◄── STEER-01 broadens Bash arm: + sed/cat │
                                   │ yes                                              │
                                   ▼                                                 │
                      steerMessage → classifyBashTarget (code vs prose/log/config)   │
                                   │ code? ─► bashSteerMessage → specific helix verb  │
                                   │ prose/log/config? ─► SILENT (STEER-03)           │
                                   ▼                                                 ▼
                      emitAdvisory → {hookSpecificOutput.additionalContext} exit 0 ◄─┘

  SessionStart hook ─► `helix activate` (+STEER-02 terse matrix, size-capped) ─► stdout→context
```

### Recommended Project Structure (delta only)
```
internal/cli/
├── nudge.go              ◆ STEER-01: broaden isGrepReadTool Bash arm (+ sed/cat)
│                            (classifyBashTarget/bashSteerMessage already handle them)
├── nudge_test.go         ◆ flip bash-sed-i/bash-cat from SILENT→FIRING (DEFER-97-01);
│                            add broadened negative-control rows (STEER-03)
├── activate.go           ◆ (option A) STEER-02: emit terse matrix on SessionStart, size-capped
│  (or) prime.go          ★ (option B) new sibling priming command
├── skill.go              ◆ AGENT-01: add EmbeddedReference() accessor (reads skills/helix/reference.md)
├── setup_clients.go      ◆ add &CodexRegistrar{}; flip gemini-cli/generic (≥) to also-write instruction file
├── setup_agents.go       ★ NEW: sentinel-delimited idempotent append + Codex hooks.json writer
├── setup.go              ◆ add "codex" to ValidArgs
├── setup_agents_test.go  ★ NEW: round-trip / idempotency / 32 KiB-cap / no-Gemini-hook goldens
└── nudge_test.go (STEER-02 cap test, or a new activate/prime test)
```

### Pattern 1: One steering engine, two runtimes (AGENT-03)
**What:** `runNudge` reads a generic JSON `hookInput` from stdin (`nudge.go:16-23`) and emits `preToolUseOutput` (`nudge.go:128-133`) with camelCase `hookSpecificOutput.additionalContext` at exit 0. Codex's PreToolUse hook uses the *same* envelope shape.
**When to use:** Codex steering — write a Codex `hooks.json` whose `type:"command"` handler runs `helix nudge`; do NOT branch the nudge code per runtime.
**Example:**
```go
// Codex hooks.json (type:"command" only handlers run — STACK.md):
// { "hooks": { "PreToolUse": [ { "command": ["<helixBinaryPath>", "nudge"] } ] } }
// runNudge already parses ToolName/ToolInput.command generically (nudge.go:64-121);
// no Codex-specific output struct is needed — the camelCase envelope is shared.
```
**Landmine:** The exact Codex `hooks.json` schema (key names, command array vs string, file location `~/.codex/hooks.json` vs `.codex/hooks.json` vs `config.toml [hooks]`) is from the milestone STACK.md (HIGH-confidence vs official Codex hooks docs) but was NOT re-verified against a live Codex install this session. **The plan should gate the exact hooks.json byte-shape behind a `checkpoint:human-verify` or a doc re-fetch** before pinning it in a golden. The envelope Helix *emits* is already correct (it's Claude's); only the *config that invokes* it is the uncertain part.

### Pattern 2: Idempotent sentinel-delimited append (AGENT-02)
**What:** Append a Helix block bracketed by `<!-- helix:begin -->` / `<!-- helix:end -->`; on re-run, replace only the bytes between the sentinels, preserving all surrounding user content. Mirrors the hook merge's filter-then-rewrite (`mergeHooksIntoSettings` removes prior `helix_managed:true` entries then re-adds — `setup_hooks.go:91-106`).
**When to use:** Writing `AGENTS.md` / `GEMINI.md` / generic instruction file.
**Example:**
```go
// Pseudocode for the new helper in setup_agents.go:
//  existing := readFileOrEmpty(path)
//  stripped := removeBetweenSentinels(existing, beginMark, endMark)  // idempotent
//  block := beginMark + "\n" + helixInstructionBody + "\n" + endMark
//  newContent := ensureTrailingNewline(stripped) + block
//  // 32 KiB cap (Codex AGENTS.md): if len(newContent) > 32*1024 → trim the
//  // appended block (e.g. link to reference.md instead of inlining it), never the user content.
//  atomicWrite(path, newContent)   // temp+rename, mirror saveSessionStats / installSkill
```
**Landmine:** The 32 KiB cap is on the WHOLE Codex `AGENTS.md` file (`project_doc_max_bytes`), not just the Helix block. If a user already has a near-32 KiB AGENTS.md, appending the full reference would blow the cap. **Recommendation:** for Codex, append a *terse* block (the decision matrix + a pointer to install reference.md or run `helix get-tool-help`), NOT the full 32 KB reference inline. The cap test asserts the appended block keeps the file ≤32 KiB given a representative user file.

### Pattern 3: Two-pass atomic file write (reused)
**What:** `installSkill` stages temp siblings then renames (`skill.go:179-235`); `saveSessionStats` does temp+rename (`nudge.go:243-251`); `mergeHooksIntoSettings` writes append+newline (`setup_hooks.go:119`). All avoid torn/partial writes from concurrent setup.
**When to use:** The new instruction-file writer should use temp+rename for the same crash-safety.

### Anti-Patterns to Avoid
- **Editing `classifyBashTarget` for STEER-01:** the classifier already handles sed/cat operands correctly (`TestClassifyBashTarget_CodeTargets` includes `cat …edit.go` and `sed -n …server.go`, `nudge_test.go:174-189`). The gate that blocks them is `isGrepReadTool`. Editing the classifier instead of the gate is the wrong fix and risks the conservative-non-code logic.
- **A deny/block (exit 2) hook or a Codex `permissionDecision:"deny"` default:** explicit anti-feature. Keep exit-0; `emitAdvisory` never returns an error and `runNudge` always returns nil (asserted by `TestNudgeAdvisory_AlwaysExitZero`, `nudge_test.go:389-406`).
- **Overwriting an existing AGENTS.md/GEMINI.md wholesale:** Pitfall 7. Always sentinel-append.
- **Fabricating a Gemini PreToolUse hook:** Gemini has no such surface; a test must assert no Gemini hook artifact is produced.
- **A second steering engine for Codex:** reuse `helix nudge`.
- **Inlining the full 32 KB reference into Codex AGENTS.md:** blows the 32 KiB cap when combined with user content; append a terse block + pointer.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Idempotent block in a managed file | A bespoke "is my block already there" scan | Mirror `mergeHooksIntoSettings`'s filter-then-rewrite (`setup_hooks.go:91-106`) — strip prior block by sentinel, re-add | Proven idempotent; avoids duplicate-block bug (Pitfall 7 warning sign). |
| Crash-safe file write | Naïve `os.WriteFile` over the live file | temp+rename (`saveSessionStats` nudge.go:243-251 / `installSkill` skill.go:179-235) | Concurrent setup never observes a partial file. |
| Emitting the PreToolUse envelope | Hand-built JSON string | `emitAdvisory` / `preToolUseOutput` struct (`nudge.go:128-149`) marshalled via `encoding/json` | T-34-01: never string-concat JSON; a marshal failure stays silent, never blocks. |
| Path containment for instruction files | Substring checks | The `withinSkillRoot`/`containedIn` filepath.Rel pattern (`skill.go:252-285`) adapted to the agent file's expected dir | Defeats `..` traversal (WR-93-02). |
| Surfacing the reference to non-Claude agents | Re-rendering verbs | `EmbeddedReference()` reading the SAME `embeddedSkillFS` bytes (`skill.go:21`) | One generated source; no drift from the `--check`-gated reference. |

**Key insight:** Every primitive this phase needs (idempotent managed-block write, atomic file write, JSON envelope, path containment, embedded asset accessor) already exists in `internal/cli/`. The new code is *wiring and one append helper*, not new infrastructure.

## Runtime State Inventory

> This is a feature-addition phase (broaden a classifier; add a setup path), NOT a rename/refactor/migration. The categories below are answered for completeness; none triggers a data migration.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `.helix/session-stats.json` (per-workspace, written by `saveSessionStats`, `nudge.go:91`) records GrepReadCount/HelixToolCount. STEER-01 makes more shapes count as grep/read, so the counter increments on more shapes — but it is **telemetry only**, does NOT gate emission (`nudge.go:25-38`), and the schema is unchanged. | None — no migration; counter semantics broaden naturally. |
| Live service config | The agent-runtime instruction files (`AGENTS.md`/`GEMINI.md`) and hook configs (`.claude/settings.json`, Codex `hooks.json`) live OUTSIDE git in the user's home/project. These are exactly what AGENT-02/03 writes. | Code edit (new writers) + idempotent re-run safety; no migration of existing files (we append, preserving them). |
| OS-registered state | None. No Task Scheduler / launchd / systemd registration touched. | None — verified: no such registration in `internal/cli/`. |
| Secrets/env vars | None. The nudge reads `$CLAUDE_PROJECT_DIR` (set by the runtime in the hook command, `setup_hooks.go:36`); no secret keys renamed. | None. |
| Build artifacts | The embedded `reference.md` is a build-time artifact (`go:embed`); AGENT-01 reuses it, does not regenerate it. Touching the bundle requires no reinstall — `installSkill` reads the live FS. | None — verified: AGENT-01 adds a read accessor, not a new generator. |

**Nothing found requiring data migration.** The only "live config" interaction (AGENT-02/03 writing user instruction files) is handled by the idempotent-append contract, which preserves existing user content by design.

## Common Pitfalls

### Pitfall 1: STEER-01 edits the wrong function
**What goes wrong:** The plan says "broaden `classifyBashTarget`," the author edits the classifier, and Bash sed/cat still don't steer — because the real gate is `isGrepReadTool` (`nudge.go:105`, `:429-441`).
**Why it happens:** `bashSteerMessage` *looks* like it handles sed/cat (it has those branches, `nudge.go:191-197`), so the author assumes the path is reachable. It is dead for Bash callers (Phase 97 DEFER-97-01 deviation documents this exactly).
**How to avoid:** Add `"sed"`, `"cat"` to the `isGrepReadTool` Bash recognizer (currently `strings.Contains(cmd, "grep"|"find"|"rg"|"ag")`, `nudge.go:435-438`). Then the existing `classifyBashTarget` + `bashSteerMessage` path fires unchanged. Verify by flipping the `bash-sed-i-silent`/`bash-cat-silent` golden sub-cases to firing.
**Warning signs:** A diff to `classifyBashTarget`'s switch but no diff to `isGrepReadTool`; the golden still asserts those shapes silent.

### Pitfall 2: Flipping the Phase 97 golden vacuously (DEFER-97-01)
**What goes wrong:** The `bash-sed-i-silent`/`bash-cat-silent` sub-cases (`nudge_test.go:543-558`) are flipped to "fires" but only assert `Contains(..., "helix")` — the exact vacuity Phase 97 killed.
**Why it happens:** Easiest flip is to assert non-emptiness.
**How to avoid:** Add the two shapes to `nudgeShapeGoldenCases()` (`nudge_test.go:475-490`) with the SPECIFIC verb: `bash-sed-i`→`helix replace-in-file` (matches `bashSteerMessage` `nudge.go:194-195`), `bash-cat`→`helix read-file` (matches `nudge.go:191-192`). Bump the empty-bucket floor from `>= 5` to `>= 7` (`nudge_test.go:506`). Keep `TestNudgeGoldenRevertFails` green. Remove the now-obsolete silent sub-case loop (`nudge_test.go:543-558`) — leaving it asserting silence would directly contradict the new firing rows and FAIL.
**Warning signs:** The flipped row asserts `Contains(..., "helix")` not `helix replace-in-file`; the floor is still `>= 5`; the silent loop still present.

### Pitfall 3: Steering over-reach on prose/log/config (STEER-03 / Pitfall 6)
**What goes wrong:** Broadening the recognizer makes `sed -i 's/x/y/' README.md` or `cat app.log` fire the nudge.
**Why it happens:** `isGrepReadTool` now passes sed/cat through, but the SILENCE for prose must still come from `classifyBashTarget` returning non-code.
**How to avoid:** `classifyBashTarget` already demotes `.md`/`.log`/`.yaml`/etc. to non-code (`nonCodeExtensions`, `nudge.go:307-313`) and `steerMessage` stays silent when `!isCode` (`nudge.go:168-171`). Add STEER-03 negative-control rows for the NEW shapes: `sed -i … README.md` SILENT, `cat config.yaml` SILENT, `cat app.log` SILENT. Mirror `TestClassifyBashTarget_NonCodeTargets` (`nudge_test.go:191-207`) and the folded negative control (`nudge_test.go:529-537`).
**Warning signs:** A nudge fires on a `.md`/`.log`/`.yaml` target; no negative-control row for the broadened sed/cat shapes.

### Pitfall 4: Multi-agent install clobbers user files (Pitfall 7)
**What goes wrong:** `helix setup codex` overwrites a user's existing AGENTS.md; or re-running duplicates the Helix block; or the Codex block exceeds 32 KiB.
**How to avoid:** Sentinel-delimited idempotent append (Pattern 2); round-trip goldens for (a) empty dir, (b) pre-existing user content survives, (c) re-run idempotent (no dup block), (d) 32 KiB cap with a representative user file. A test asserts NO Gemini hook artifact is produced.
**Warning signs:** Setup truncates an existing file; re-run duplicates the block; a Gemini `hooks.json` appears.

### Pitfall 5: SessionStart priming bloat (STEER-02)
**What goes wrong:** The SessionStart hook injects the full reference (32 KB) or the whole SKILL body every session, regressing the 599-byte idle-cost win.
**How to avoid:** Inject ONLY the terse matrix (the decision-table rows, not the per-verb reference), with a SKILL-04-style byte-cap assertion. Source the matrix text from a single constant (so it can't drift from SKILL.md's table). Fail-open: a priming failure must not break the session (the activate command already returns its error non-fatally up the hook chain; keep stdout best-effort).
**Warning signs:** SessionStart stdout > the terse matrix; the reference or SKILL body is inlined; no size-cap test.

## Code Examples

### STEER-01: the one-line-class edit (broaden the gate)
```go
// internal/cli/nudge.go:429-441 — CURRENT (Bash arm matches only grep/find/rg/ag):
func isGrepReadTool(name string, input map[string]any) bool {
	switch name {
	case "Grep", "Read":
		return true
	case "Bash":
		cmd, _ := input["command"].(string)
		return strings.Contains(cmd, "grep") ||
			strings.Contains(cmd, "find") ||
			strings.Contains(cmd, "rg") ||
			strings.Contains(cmd, "ag")
	}
	return false
}
// STEER-01: add `|| strings.Contains(cmd, "sed") || strings.Contains(cmd, "cat")`.
// NOTE: classifyBashTarget then decides code-vs-prose, so `cat README.md` still stays
// silent. bashSteerMessage already maps cat→read-file (nudge.go:191) and sed→replace-in-file
// (nudge.go:194). Consider a token-anchored check (fields[0]) over Contains to avoid a
// stray "cat" substring inside a path — see Open Question 1.
```

### STEER-01 verb mapping (already present in bashSteerMessage — verified, nudge.go:180-206)
```go
// case "cat":  → "helix read-file --path=<file>" + "helix get-symbol-overview"   (nudge.go:191-192)
// case "sed":  if -i → "helix replace-in-file" / "helix replace-symbol-body"     (nudge.go:194-195)
//              else  → "helix read-file --path=<file>" (sed -n range read)        (nudge.go:196-197)
// case "find": → "helix find-files --pattern='**/*.ext'"                          (nudge.go:188-189)
// default (grep/rg/ag/egrep/fgrep): -r/-R → find-references/get-call-hierarchy;   (nudge.go:198-204)
//              else → search-symbols / search-in-files
```

### AGENT-02: idempotent sentinel append (new helper, mirrors mergeHooksIntoSettings)
```go
// internal/cli/setup_agents.go (NEW). Sentinels chosen so they are invisible in
// rendered markdown and unique enough to strip on re-run.
const (
	helixBlockBegin = "<!-- helix:begin (managed by `helix setup`) -->"
	helixBlockEnd   = "<!-- helix:end -->"
)

// writeAgentInstructions appends/refreshes the Helix block in path idempotently,
// preserving all user content outside the sentinels, atomically (temp+rename),
// and capped at maxBytes (e.g. 32*1024 for Codex AGENTS.md). Returns nil on a
// no-op (block unchanged). Path containment + dry-run honored by the caller.
func writeAgentInstructions(path, body string, maxBytes int) error { /* strip-between-sentinels → append → cap → atomic write */ }
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Non-Claude clients teardown-only (MCP removal, no skill) | Also-write a per-agent instruction file + (Codex) a reused nudge hook | This phase (98) | The 4 non-Claude clients flip from `teardownOnlyRegister` to also-install; a new `codex` client is added. |
| Bash sed/cat silently NOT steered (DEFER-97-01) | Bash sed/cat steer to the specific verb | This phase (STEER-01) | The dead `bashSteerMessage` sed/cat branches become live; Phase 97 golden flips. |
| SessionStart hook = `activate` only | `activate` (or sibling) also primes the terse matrix once/session | This phase (STEER-02) | Idle-cost-bounded session priming, fail-open. |

**Deprecated/outdated:** none — this is additive. The exit-0 advisory contract, the embed.FS bundle, and the idempotent hook-merge are all carried forward unchanged.

## Project Constraints (from CLAUDE.md)

- **Go build/test gates:** `go build ./cmd/helix`, `go test ./...`, `go vet ./...`, `gofmt -w .` — "Always run go vet and go test before completing any Go task." All new/edited tests run in the default untagged suite.
- **CLI-first / no MCP surface:** the agent surface is `helix <verb>`; MCP SDK + gRPC are internal plumbing. STEER/AGENT work touches only the CLI setup + hook layer; do NOT reintroduce an MCP-facing surface.
- **Helix CLI verb routing:** the verbs referenced in `bashSteerMessage` (`read-file`, `replace-in-file`, `replace-symbol-body`, `find-files`, `search-symbols`, `search-in-files`, `find-references`, `get-call-hierarchy`, `get-symbol-overview`) MUST be real frozen verbs in `verbs_gen.go` — they are (the nudge golden + `VerbToolNames()` drift test already enforce this). Do NOT invent verbs.
- **GSD workflow:** file-changing work goes through a GSD command (this phase is `/gsd-execute-phase`).
- **Zero-dep invariant:** `git diff go.mod` MUST stay empty (milestone-wide rule).
- **Reference is generated, never hand-edited:** `reference.md` carries `Code generated by helix-refgen; DO NOT EDIT`; AGENT-01 must read it, not modify it; `helix-refgen --check` stays clean.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Codex `PreToolUse` hooks.json schema (key names, command array-vs-string, file location `~/.codex/hooks.json`/`.codex/hooks.json`/`config.toml [hooks]`, `type:"command"`-only) matches the milestone STACK.md findings. | Pattern 1, AGENT-03 | A wrong hooks.json shape means Codex silently never invokes `helix nudge` (no error, just no steering). **Mitigation: re-fetch the Codex hooks doc or gate the exact byte-shape behind `checkpoint:human-verify` before pinning a golden.** The envelope Helix *emits* is independently correct. |
| A2 | Codex `AGENTS.md` 32 KiB cap (`project_doc_max_bytes`) and discovery order (project root → cwd walk; `~/.codex/AGENTS.md`); Gemini `GEMINI.md` location (`~/.gemini/GEMINI.md` + workspace/parent; filename configurable via `context.fileName`). | AGENT-02, Pattern 2 | Writing to the wrong path → the instruction file is silently ignored by the agent. Carried HIGH-confidence from STACK.md (vs official docs) but not re-verified live this session. **Mitigation: the cap is a hard test; the path should be confirmed against current Codex/Gemini docs at plan time.** |
| A3 | Gemini CLI has NO PreToolUse-equivalent (instruction-file steering only). | AGENT-03 | If Gemini *did* gain a hook, we'd be under-delivering — but fabricating one when it doesn't exist is the worse failure (locked anti-feature). Safe default: no Gemini hook. |
| A4 | SessionStart `command`-hook stdout is surfaced into agent context (the basis for STEER-02 via `activate` stdout). | STEER-02 | If stdout isn't surfaced, the priming text never reaches the agent. The existing `activate` already writes a status line to stdout for exactly this hook (`activate.go:73`), so the mechanism is in-use today — LOW risk, but the *size-cap value* and whether to extend activate vs add a command is a planning choice. |
| A5 | The scope of "non-Claude clients to flip" — whether vscode/jetbrains/opencode also get instruction files, or only codex/gemini-cli/generic (the three AGENT-02 names). | AGENT-01/02 structure | REQUIREMENTS AGENT-02 names "Codex AGENTS.md, Gemini GEMINI.md, generic." vscode/jetbrains/opencode are not named. **Recommend: scope AGENT-02 to codex + gemini-cli + generic; leave vscode/jetbrains/opencode teardown-only unless the planner decides otherwise.** Risk if wrong: over- or under-scoping the flip. |

**If this table is empty:** it is not — A1/A2 (Codex/Gemini conventions) are the items the discuss/plan step should confirm before locking the hooks.json/AGENTS.md byte-shapes.

## Open Questions

1. **`isGrepReadTool` substring vs token-anchored recognition for sed/cat.**
   - What we know: the current arm uses `strings.Contains(cmd, "grep"|...)`. Adding `Contains(cmd, "cat")` would also match `cat` inside a path like `concatenate.go` or `application.go`.
   - What's unclear: whether to accept that (the downstream `classifyBashTarget` still gates on a real code operand, so a false `isGrepReadTool` true on `echo concat` simply yields no advisory via `classifyBashTarget` fail-open) or to anchor on `fields[0]` (the command token) for precision.
   - Recommendation: anchor on the first token (`strings.Fields(cmd)[0] == "sed"/"cat"`) to avoid spurious counter increments and keep parity with `classifyBashTarget`'s own `fields[0]` switch (`nudge.go:344-345`). Low cost, higher precision. (Note `classifyBashTarget` already only recognizes `fields[0]` in {grep,rg,ag,sed,cat,find,egrep,fgrep}, so sed/cat are already in its switch — only `isGrepReadTool` lags.)

2. **STEER-02: extend `activate` vs add a dedicated priming command.**
   - What we know: SessionStart currently runs one command (`activate`). Either extend its stdout or add a second SessionStart hook entry.
   - Recommendation: extend `activate` stdout (lower surface, no new hook entry, no setup change). Emit the terse matrix after the activation status line, behind a size cap. If a clean separation is preferred, a `helix prime`/`nudge --session-start` sibling is acceptable (Claude's discretion).

3. **Exact terse-matrix text + byte cap for STEER-02.**
   - What we know: SKILL-04 caps the idle description at 1,536 chars; the SessionStart matrix should be comparably bounded.
   - Recommendation: a single constant holding ~the decision-table rows (use X / not Y), asserted ≤ a fixed cap (e.g. ≤2 KiB) by a unit test, sourced so it cannot drift silently from SKILL.md's matrix.

## Environment Availability

> This phase is code/config-only (Go edits + tests + file-writer wiring). No external runtime/service/tool dependency is introduced. The only "environments" are the *target* agent runtimes (Codex/Gemini) whose file/hook conventions are consumed at install time, not at build/test time — those are covered by Assumptions A1–A3, not by a local availability probe.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test/vet | ✓ (repo builds today) | go 1.25.x | — |
| Codex CLI (target runtime) | AGENT-03 hooks.json validation | not required for build/test | — | hooks.json shape verified by doc + golden, not a live Codex; gate byte-shape behind human-verify (A1) |
| Gemini CLI (target runtime) | AGENT-02 GEMINI.md path | not required for build/test | — | path from docs; no hook fabricated |

**Missing dependencies with no fallback:** none — the phase builds and tests with only the Go toolchain.
**Missing dependencies with fallback:** Codex/Gemini live installs are NOT needed to implement or test this phase; their conventions are encoded as goldens + assumptions to confirm at plan time.

## Validation Architecture

> `workflow.nyquist_validation` is `true` in `.planning/config.json` — this section is REQUIRED.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify` (`assert`/`require`) |
| Config file | none — standard `go test`; tests live beside code (`internal/cli/*_test.go`) |
| Quick run command | `go test ./internal/cli/...` |
| Full suite command | `go test ./...` (untagged; this is the merge gate — ADOPT-01 goldens already run here) |
| Drift gate | `go run ./cmd/helix-refgen --check` (reference.md unchanged) + `git diff go.mod` empty |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STEER-01 | Bash `sed -i …code.go` steers to `helix replace-in-file`; Bash `cat …code.go` steers to `helix read-file` | unit golden | `go test ./internal/cli/ -run TestNudgeShapeGolden` | ◆ extend `nudge_test.go` (add bash-sed-i / bash-cat firing rows; floor `>=7`) |
| STEER-01 | `isGrepReadTool` now passes sed/cat Bash shapes into steering | unit | `go test ./internal/cli/ -run TestIsGrepReadTool` | ◆ extend (`nudge_test.go:106-124`) add sed/cat true-cases |
| STEER-01 (anti-vacuity) | a wrong-verb expectation for sed/cat goes RED | unit | `go test ./internal/cli/ -run TestNudgeGoldenRevertFails` | ◆ extend revert proof to a sed/cat shape, OR rely on per-shape specific-verb assert |
| STEER-02 | SessionStart priming emits the terse matrix, size-capped, fail-open | unit | `go test ./internal/cli/ -run TestSessionStartPriming` (new) | ❌ Wave 0 — new test for the activate/prime cap + fail-open |
| STEER-03 | nudge does NOT fire on `sed -i … README.md`, `cat app.log`, `cat config.yaml` | unit golden | `go test ./internal/cli/ -run TestClassifyBashTarget_NonCodeTargets` + the golden negative control | ◆ extend non-code cases with the broadened sed/cat shapes |
| STEER-03 (exit-0) | every broadened shape returns nil (exit 0) | unit | `go test ./internal/cli/ -run TestNudgeAdvisory_AlwaysExitZero` | ◆ add the new sed/cat shapes to the input list (`nudge_test.go:392-401`) |
| AGENT-01 | reference bytes are surfaceable to non-Claude agents | unit | `go test ./internal/cli/ -run TestEmbeddedReference` (new) | ❌ Wave 0 — new accessor test (reference non-empty, matches embedded FS) |
| AGENT-02 | append into empty dir; preserve pre-existing user content; idempotent re-run (no dup block); ≤32 KiB cap | unit golden round-trip | `go test ./internal/cli/ -run TestWriteAgentInstructions` (new) | ❌ Wave 0 — new `setup_agents_test.go` |
| AGENT-03 | Codex client writes AGENTS.md + hooks.json→`helix nudge`; Gemini/generic write instruction file ONLY (NO hook artifact) | unit golden | `go test ./internal/cli/ -run TestCodexRegistrar` / `TestGeminiNoHookArtifact` (new) | ❌ Wave 0 — new tests |
| AGENT-03 (reuse) | Codex nudge reuses the SAME envelope (no second engine) | unit | covered by existing `runNudge` envelope tests; assert Codex hooks.json command == `<bin> nudge` | ◆/❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/...` (the touched package — fast, < 5s) + `go vet ./internal/cli/...`
- **Per wave merge:** `go test ./...` (full untagged suite — the merge gate, includes ADOPT-01) + `go run ./cmd/helix-refgen --check` + `git diff go.mod` empty
- **Phase gate:** full suite green + `go vet ./...` clean + reference `--check` clean before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/cli/setup_agents.go` + `internal/cli/setup_agents_test.go` — covers AGENT-02 (round-trip / idempotency / 32 KiB cap) and AGENT-03 (Codex AGENTS.md + hooks.json; Gemini no-hook). NEW.
- [ ] `internal/cli/nudge_test.go` — STEER-01 firing rows (flip DEFER-97-01 silent sub-cases → specific-verb firing; floor `>=7`; remove the obsolete silent loop); STEER-03 broadened negative controls; add sed/cat to `TestIsGrepReadTool` + `TestNudgeAdvisory_AlwaysExitZero`. EXTEND.
- [ ] STEER-02 priming test (in `activate_test.go` or a new `prime_test.go`) — terse-matrix size cap + fail-open. NEW.
- [ ] `EmbeddedReference()` accessor + test (`skill.go` / `skill_test.go`) for AGENT-01. NEW.
- [ ] Framework install: none — `testing`+`testify` already in tree.

**Anti-vacuity reminder (milestone-wide):** STEER-01's golden flip and AGENT-02's idempotency MUST each ship a deliberate break-the-invariant assertion: a wrong-verb expectation for sed/cat goes RED (extend `TestNudgeGoldenRevertFails`); a re-run that duplicated the block would fail the idempotency assert; an over-firing sed/cat on prose would fail the negative control. A gate with only a green-path test is presumed broken (the v1.12 four-CRITICAL lesson).

## Security Domain

> `security_enforcement` is not disabled in config (`workflow` has no `security_enforcement: false`) — included. This is a CLI setup/hook phase with no auth/session/crypto surface; the relevant categories are input-validation (parsing Bash strings as DATA) and safe file writes.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface touched. |
| V3 Session Management | no | The "session" is agent-runtime session telemetry (`session-stats.json`), not a security session. |
| V4 Access Control | no | No new authz path; the nudge is advisory exit-0. |
| V5 Input Validation | **yes** | Bash command strings are tokenized as DATA via `strings.Fields` (no `os/exec`, no regex backtracking — `classifyBashTarget` `nudge.go:337-425`, T-93-05). The new sed/cat recognition MUST preserve this (no execution of the command string). Stdin hook JSON parsed via `json.Decoder` (`nudge.go:65-70`), fail-silent on malformed input. |
| V6 Cryptography | no | No crypto. |
| V12 File handling | **yes** | New instruction-file writes MUST be path-contained (adapt `withinSkillRoot`/`containedIn` filepath.Rel guard, `skill.go:252-285`) and atomic (temp+rename). User content preserved by sentinel-append (no data loss). |

### Known Threat Patterns for {Go CLI hook + file writer}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Command injection via the Bash string the nudge inspects | Tampering / EoP | NEVER execute the string; tokenize as DATA only (existing `classifyBashTarget` contract; `TestClassifyBashTarget_PureDataNoExec` `nudge_test.go:235-249` proves `grep x f.go && rm -rf /` is read as data, no side effects). The sed/cat broadening rides the same code path. |
| Path traversal in the instruction-file destination | Tampering | filepath.Rel containment guard (reuse the skill-install pattern); entry/dest names derived from a fixed convention, never raw user input. |
| Clobbering / data loss of user's AGENTS.md/GEMINI.md | Tampering (data loss) | Sentinel-delimited idempotent append; round-trip golden proves user content survives; temp+rename atomic write. |
| Codex hook deny-by-default | DoS of legit workflows / model-evasion | Advisory exit-0 only; assert the Codex hook command emits `additionalContext` and never `permissionDecision:"deny"` by default (Helix emits the shared envelope; no deny). |

## Sources

### Primary (HIGH confidence) — direct in-tree read this session
- `internal/cli/nudge.go` (runNudge, isGrepReadTool, classifyBashTarget, steerMessage, bashSteerMessage, emitAdvisory, preToolUseOutput, codeExtensions/nonCodeExtensions/nonCodeBasenames) — the STEER-01/03 surface
- `internal/cli/nudge_test.go` (TestNudgeShapeGolden, nudgeShapeGoldenCases, the DEFER-97-01 silent sed/cat sub-cases, TestNudgeGoldenRevertFails, TestClassifyBashTarget_*, TestNudgeAdvisory_AlwaysExitZero) — the golden to flip
- `internal/cli/setup_clients.go` (clientRegistry 7 clients, ClaudeCodeRegistrar.Register, teardownOnlyRegister, GeminiCLIRegistrar/Generic/VSCode/JetBrains/OpenCode) — confirmed NO codex client, NO instruction-file writer
- `internal/cli/setup_hooks.go` (helixHookConfig SessionStart/PreToolUse/Stop, mergeHooksIntoSettings, filterOutHelixEntries, isHelixManaged) — the idempotent-block precedent + the PreToolUse `Grep|Read|Bash` matcher
- `internal/cli/skill.go` (//go:embed skills/helix/*, embeddedSkillFS, EmbeddedSkillBody, installSkill two-pass atomic, withinSkillRoot/containedIn) — AGENT-01 reuse + path containment
- `internal/cli/setup.go` (ValidArgs 7 clients, runSetup), `internal/cli/setup_detect.go`, `internal/cli/setup_output.go`, `internal/cli/activate.go` (SessionStart stdout), `cmd/helix-refgen/main.go` (blank-import parity), `internal/cli/skills/helix/{SKILL.md,reference.md}`
- `.planning/phases/97-.../97-02-SUMMARY.md` (DEFER-97-01 deviation: the exact silent-sed/cat sub-cases + why; section-header completeness; revert-and-fail pattern)
- `.planning/config.json` (nyquist_validation=true, skip_discuss=true, zero-dep context)

### Secondary (MEDIUM confidence) — carried from milestone research (verified there vs official docs, re-flagged as assumptions)
- `.planning/research/STACK.md` §1B — Codex AGENTS.md 32 KiB cap + PreToolUse `additionalContext`/`type:"command"`-only; Gemini GEMINI.md no-hook (HIGH in STACK.md vs developers.openai.com/codex/hooks + geminicli.com; NOT re-fetched this session → Assumptions A1/A2/A3)
- `.planning/research/ARCHITECTURE.md` (Pattern 3 nudge envelope reuse; setup_agents.go new file), `.planning/research/PITFALLS.md` (Pitfall 6 steering over-reach, Pitfall 7 multi-agent clobber), `.planning/research/SUMMARY.md` (Phase A3 maps to this phase)

## Metadata

**Confidence breakdown:**
- STEER-01/02/03 (the steering thrust): **HIGH** — the exact functions, line anchors, the real gate (`isGrepReadTool`), the existing-but-dead sed/cat branches, and the Phase 97 golden to flip were all read directly this session.
- AGENT-01 (reference reuse): **HIGH** — the embed.FS bundle + accessor pattern is in-tree.
- AGENT-02 (idempotent append): **HIGH** on the in-repo precedent (hook merge); **MEDIUM** on the exact Codex/Gemini file paths + 32 KiB cap (carried from STACK.md, flagged A1/A2).
- AGENT-03 (Codex hook): **HIGH** on the reuse-the-nudge design + no-Gemini-hook rule; **MEDIUM** on the exact Codex `hooks.json` byte-schema (flagged A1 — gate behind human-verify before pinning a golden).

**Research date:** 2026-06-22
**Valid until:** 2026-07-22 for the in-tree code anchors (stable); ~2026-06-29 for the Codex/Gemini external conventions (fast-moving agent-runtime docs — re-confirm A1/A2 at plan time).
