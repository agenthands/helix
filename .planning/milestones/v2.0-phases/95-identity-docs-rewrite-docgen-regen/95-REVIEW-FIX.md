---
phase: 95-identity-docs-rewrite-docgen-regen
fixed_at: 2026-06-22T00:00:00Z
review_path: .planning/phases/95-identity-docs-rewrite-docgen-regen/95-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
deferred: 2
status: all_fixed
---

# Phase 95: Code Review Fix Report

**Fixed at:** 2026-06-22
**Source review:** .planning/phases/95-identity-docs-rewrite-docgen-regen/95-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (1 BLOCKER + 5 WARNING)
- Fixed: 6
- Skipped: 0
- Deferred (out of scope): 2 (IN-01, IN-02)

These were all incomplete-staleness-sweep doc-accuracy fixes: lines in the
unchanged portions of CLAUDE.md / README.md still described Phase-94-deleted
agent-facing transports and the Phase-93-flipped setup contract as current.
Each claim was verified against live source before editing.

## Fixed Issues

### BL-01: CLAUDE.md described `helix setup` as "one-command MCP registration"

**Files modified:** `CLAUDE.md` (Layer 3 setup line, ~64)
**Commit:** 46207709
**Applied fix:** Rewrote the line to the Phase 93 contract: `helix setup`
installs the Helix Agent Skill + hooks (Claude-family clients) and
idempotently tears down any prior Helix MCP-server registration across the 7
clients; non-skill clients (vscode, jetbrains, gemini-cli, opencode, generic)
get MCP-teardown only (no skill written); the internal MCP daemon head is left
intact. Verified against `internal/cli/setup.go:19-27` (`Short`/`Long`) and
`setup_clients.go:693-699` (generic = `teardownOnlyRegister`).

### WR-01: CLAUDE.md:37 listed deleted agent-facing transports as current

**Files modified:** `CLAUDE.md` (Layer 0 transports line, ~37)
**Commit:** 46207709
**Applied fix:** Replaced "Transports: stdio (via forwarder), Streamable HTTP
(direct)" with the accurate current story — the agent surface is the `helix`
CLI dialing the daemon over gRPC `StreamMCP` (unix socket / named-pipe
default; opt-in loopback-gated gRPC TCP); the stdio MCP forwarder head and
Streamable-HTTP `/mcp` head were removed in Phase 94; only the internal gRPC
`StreamMCP` wire remains. Did not over-claim "MCP gone" — the SDK + gRPC
plumbing is explicitly retained.

### WR-02: CLAUDE.md:162 said setup ends by "registering the MCP server"

**Files modified:** `CLAUDE.md` (Setup & Hooks line, ~162)
**Commit:** 46207709
**Applied fix:** Changed "before registering the MCP server" to "before
installing the Helix skill + hooks (and tearing down any prior MCP
registration)".

### WR-03: CLAUDE.md:160 described setup via the deleted `claude mcp add-json` path

**Files modified:** `CLAUDE.md` (Setup & Hooks line, ~160)
**Commit:** 46207709
**Applied fix:** Re-described the mechanism around skill + hooks installation
plus MCP teardown; replaced the `claude mcp add-json` example with the
teardown call (`claude mcp remove`) and stated setup no longer registers an
MCP server.

### WR-04: CLAUDE.md:133 "53 callable tools" stale and self-inconsistent with docgen

**Files modified:** `CLAUDE.md` (Tool Registration line, ~133)
**Commit:** 46207709
**Applied fix:** Changed "53 callable tools" to "50 frozen `helix` verbs ...
the README table renders 51 rows because `analyze-blast-radius` is
dual-categorized", matching `cmd/docgen/main.go:128`.

### WR-05: README MCP-registration / "Generic MCP client" framing stale post-Phase-93

**Files modified:** `README.md` (Key Features table ~141; Configure-Your-Client
codeblock ~92)
**Commit:** 35841b39
**Applied fix:** README:141 → "One-command skill + hooks install (and
prior-MCP teardown)". README:92 → `# Generic client (MCP-teardown only; no
skill)`. Now consistent with the already-corrected CLI-first prose at
README:102. Left the auto-generated TOOLS table untouched.

## Deferred Issues (Info — out of scope)

### IN-01: README:332 "any MCP tool" in get-tool-help description

**File:** `README.md:332`
**Reason:** Out of scope (Info). It is an auto-generated row sourced from the
tool's `Description` in `internal/kernel/help/`; hand-editing README would be
reverted by the docgen CI gate (DOCS-02), and this is a DOCS phase (no `.go`
edits). Fixing it requires a source-level Description change + docgen regen,
which belongs to a separate task.

### IN-02: PROJECT.md listed in review scope does not exist

**File:** `PROJECT.md` (scope)
**Reason:** Out of scope (Info) and not a doc-accuracy edit. No `PROJECT.md`
exists at repo root; the scope-list discrepancy is a review-config note, not a
source/doc correction.

## Verification

- `go run ./cmd/docgen --check` → exit 0 ("README.md is up to date"; tool
  table undisturbed)
- `make verify-docs` → exit 0
- `go build ./...` → exit 0
- `git diff go.mod` → empty
- `grep -c "mcp__smtc__" CLAUDE.md` → 31 (unchanged; SMTC routing matrix intact)
- grep for stale claims-as-current (`--mode=stdio` / `--mode=http` / `/mcp` /
  `mcp add-json` / "MCP registration" / "53 callable tools" / "53 MCP tools" /
  "Generic MCP client" / "client registration") in README.md + CLAUDE.md →
  none remaining as current behavior (only the WR-01 line describing `/mcp` as
  *removed* and legitimate `internal/mcp/` package paths persist)

---

_Fixed: 2026-06-22_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
