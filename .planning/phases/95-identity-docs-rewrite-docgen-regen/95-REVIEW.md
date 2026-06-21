---
phase: 95-identity-docs-rewrite-docgen-regen
reviewed: 2026-06-22T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - CLAUDE.md
  - README.md
  - cmd/docgen/main.go
findings:
  blocker: 1
  warning: 5
  info: 2
  total: 8
status: issues_found
---

# Phase 95: Code Review Report

**Reviewed:** 2026-06-22
**Depth:** standard
**Files Reviewed:** 3 (PROJECT.md listed in scope does not exist in the tree — see IN-02)
**Status:** issues_found

## Summary

Phase 95 rewrote Helix's identity to CLI-first and re-keyed the docgen tool table to `helix <verb>` names. The substance that was *new* in this phase is largely sound:

- **docgen re-key (cmd/docgen/main.go) is correct.** The `_`→`-` mapping (line 129) is mechanically valid for all 50 frozen verbs; `go run ./cmd/docgen --check` passes (README in sync); the CI drift gate (DOCS-02) is wired at `.github/workflows/go-test.yml:108-112`. No tool dropped — all 50 distinct verb names in `internal/cli/verbs_gen.go` render, with `analyze-blast-radius` correctly appearing twice (dual-categorization → 51 rows).
- **CLAUDE.md Helix-CLI routing matrix (DOCS-03) cites only real verbs.** All 27 verbs in the new table at CLAUDE.md:198-223 exist in `verbs_gen.go`. The external `mcp__smtc__*` SMTC matrix is intact and explicitly fenced off from the Helix surface by the NOTE at CLAUDE.md:227 — the two are not conflated.
- **MCP framing is balanced.** No doc claims MCP as the primary *agent* interface, and no doc over-claims "MCP removed" — the MCP Go SDK + gRPC `StreamMCP` are correctly described as retained internal plumbing (CLAUDE.md:20,94,179; README:19,47,102), which matches `internal/forwarder/session.go:12-30`.

However, **the rewrite did not finish the staleness sweep it set out to do.** Several lines in the *unchanged* portions of CLAUDE.md still describe the agent-facing MCP transports Phase 94 DELETED and the MCP-registration setup behavior Phase 93 FLIPPED, as if current. One of these (CLAUDE.md:64) is a direct restatement of the OLD setup contract and will mislead an agent/operator about what `helix setup` does. There is also a stale tool count the rewrite introduced an inconsistency around.

## Blockers

### BL-01: CLAUDE.md still describes `helix setup` as "MCP registration" — Phase 93 flipped it to skill+hooks install + MCP teardown

**File:** `CLAUDE.md:64`
**Issue:** The Layer 3 description reads: *"`helix setup <client>` one-command **MCP registration** for 7 clients (...) with language detection, LS pre-installation, and Claude Code hook installer."* Phase 93 inverted this contract. The actual code (`internal/cli/setup.go:19`) describes the command as *"Install the Helix skill + hooks for a coding agent and **remove any prior MCP registration**"*, and `setup_clients.go:693-699` shows even the generic client is now MCP-teardown-only. This line states the deleted behavior as the current behavior — an agent or operator reading CLAUDE.md as ground truth will expect `helix setup` to register an MCP server (it tears one down). This is the GSD project-instruction file, so the blast radius is every future agent session in this repo.
**Fix:** Rewrite to match the Phase 93 contract, e.g.:
```
`internal/cli/setup.go`, ... -- `helix setup <client>` installs the Helix Agent
Skill + hooks (Claude-family clients) and tears down any prior Helix MCP-server
registration for 7 clients (Claude Code, VS Code, JetBrains, Claude Desktop,
Gemini CLI, OpenCode, generic), with language detection and LS pre-installation.
Non-skill clients (vscode, jetbrains, gemini-cli, opencode, generic) get
MCP-teardown only (no skill written).
```

## Warnings

### WR-01: CLAUDE.md:37 lists deleted agent-facing transports as current

**File:** `CLAUDE.md:37`
**Issue:** *"Transports: stdio (via forwarder), Streamable HTTP (direct)"* — both agent-facing transports were deleted in Phase 94. Evidence: commit `a1d4da7c` "delete the stdio MCP head (RunForwarder + stdio route + dead RunStdio)", commit `abd76b13` "delete the HTTP /mcp head and thread out --http-addr", and `internal/forwarder/session.go:18` ("the agent-facing stdio forwarder head (`helix --mode=stdio`) is deleted (Phase 94)"). `internal/cli/root.go:195` confirms `--mode http` no longer serves MCP. The only remaining transport is the internal gRPC unix socket the CLI dials. This line is factually false.
**Fix:** Replace with: `Transport: internal gRPC over a unix socket (CLI ↔ daemon); the agent-facing stdio forwarder and Streamable-HTTP /mcp heads were removed in Phase 94 — only the internal gRPC StreamMCP wire remains.`

### WR-02: CLAUDE.md:162 says setup ends by "registering the MCP server"

**File:** `CLAUDE.md:162`
**Issue:** Under "Setup & Hooks": *"Language detection (`internal/cli/setup_detect.go`) scans the project directory for known file extensions and pre-installs LSs via the three-tier installer **before registering the MCP server**."* Same Phase 93 staleness as BL-01 — there is no MCP-server registration step anymore; setup installs skill + hooks and tears down any prior MCP entry. The trailing clause is false.
**Fix:** Change "before registering the MCP server" to "before installing the Helix skill + hooks (and tearing down any prior MCP registration)".

### WR-03: CLAUDE.md:160 describes the setup mechanism via the deleted `claude mcp add-json` path

**File:** `CLAUDE.md:160`
**Issue:** *"`helix setup <client>` (...) invokes client CLIs as subprocess (e.g., `claude mcp add-json`) rather than writing config files directly."* Post-Phase-93 the primary action is skill + hooks installation; `claude mcp add-json` (MCP *registration*) is no longer what setup does — the inverse (`claude mcp remove` / teardown) plus skill install is. Citing `claude mcp add-json` as the representative example describes the pre-Phase-93 mechanism as current.
**Fix:** Re-describe the mechanism around skill-dir writes + hook installation + MCP teardown; if a subprocess example is kept, use the teardown call, not `add-json`.

### WR-04: CLAUDE.md:133 tool count "53 callable tools" is stale and now inconsistent with the rest of the corpus

**File:** `CLAUDE.md:133`
**Issue:** *"Full tool inventory (53 callable tools with profile/mode matrix) is auto-generated in `README.md`."* The frozen surface is **50 verbs** (`internal/cli/verbs_gen.go` has exactly 50 distinct `Name:` entries; the README table renders 51 rows only because `analyze_blast_radius` is dual-categorized). cmd/docgen/main.go:128 itself states "all 50 frozen verbs". So within Phase 95's own deliverables, docgen says 50 while CLAUDE.md says 53 — a self-inconsistency the rewrite introduced/left behind. The old "53 MCP tools" figure also predates the CLI-first framing the phase just established.
**Fix:** Change "53 callable tools" to "50 callable `helix` verbs (51 README rows — `analyze-blast-radius` is dual-categorized)", matching the docgen comment at cmd/docgen/main.go:128.

### WR-05: README "MCP registration" / "Generic MCP client" framing is stale post-Phase-93

**File:** `README.md:141`, `README.md:92`
**Issue:** Two README lines still frame setup as MCP registration:
- `README.md:141` (Key Features table): *"Setup CLI | One-command **client registration**: `helix setup claude-code` ..."* — setup now installs skill + hooks and tears down MCP, it does not "register" a client onto an MCP server.
- `README.md:92`: `helix setup generic        # Generic MCP client` — per `setup_clients.go:693-699` the generic path is MCP-teardown-only and writes no skill; calling it the "Generic MCP client" setup target implies it wires up MCP (it removes it).

Note the body text at README:102 was correctly updated ("installs the agent skill + hooks ... There is no MCP-server registration to hand-write"), so this is an incomplete sweep — the comment/table lines contradict the corrected prose in the same file.
**Fix:** README:141 → "One-command skill + hooks install (and prior-MCP teardown)". README:92 → drop "MCP" from the comment, e.g. `# Generic client (MCP-teardown only; no skill)`.

## Info

### IN-01: README:332 "any MCP tool" in get-tool-help description is a generated artifact, mildly off-message

**File:** `README.md:332`
**Issue:** The generated row reads "Get comprehensive documentation for any **MCP tool** ...". In the CLI-first framing the agent calls `helix get-tool-help`, not an MCP tool. This text comes from the tool's `Description` in source (it is auto-generated, so not hand-editable here), so it is not a Phase 95 doc-edit defect — but it leaves an "MCP tool" reference in the agent-facing verb table. Out of strict scope (source-level description, regenerates), flagged for awareness.
**Fix:** If desired, update the underlying tool Description in `internal/kernel/help/` to say "any `helix` verb", then re-run `go run ./cmd/docgen`. Do not hand-edit README.md (CI gate DOCS-02 would revert it).

### IN-02: PROJECT.md listed in review scope does not exist

**File:** `PROJECT.md` (scope), commit `617e07bc` touched it
**Issue:** The phase config lists `PROJECT.md` as a reviewed file and commit `617e07bc` ("rewrite README/CLAUDE.md/PROJECT.md to CLI-first identity") references it, but there is no `PROJECT.md` at repo root (nor found in the working tree). It could be located elsewhere (e.g., `.planning/`) or have been removed/renamed after the commit. The CLI-first accuracy of PROJECT.md could not be verified.
**Fix:** Confirm PROJECT.md's actual path; if it lives under `.planning/` it is out of review scope, but the scope list should be corrected. If it was intended at root and is missing, that is a delivery gap to investigate.

---

_Reviewed: 2026-06-22_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
