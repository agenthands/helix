---
phase: 41-install-contributing
verified: 2026-04-23T16:00:00Z
status: passed
score: 9/9 must-haves verified
overrides_applied: 0
---

# Phase 41: Install & Contributing Verification Report

**Phase Goal:** INSTALL.md and CONTRIBUTING.md accurately guide new users and contributors through the current codebase
**Verified:** 2026-04-23T16:00:00Z
**Status:** passed
**Re-verification:** No -- initial verification (closes F-13 from v1.8 integration check)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | INSTALL.md names `serena setup <client>` as the primary install path | VERIFIED | `INSTALL.md:31-43` Quick Start opens with "The fastest way to configure Serena for your coding agent is the setup CLI" followed by `serena setup <client>` commands -- closed by `b3932958` |
| 2 | INSTALL.md includes `go install` + PATH verification prerequisites | VERIFIED | `INSTALL.md:5-29` Prerequisites section documents `go install github.com/postfix/serena/cmd/serena@latest`, build-from-source alternative, `serena --help` PATH verification, and `$(go env GOPATH)/bin` hint -- closed by `b3932958` |
| 3 | INSTALL.md Quick Start enumerates all 7 registered clients | VERIFIED | `INSTALL.md:36-42` lists `claude-code, vscode, jetbrains, gemini-cli, claude-desktop, opencode, generic` (all 7) -- closed by `3491ba0d` (43-02 opencode+generic addition on top of `b3932958`) |
| 4 | INSTALL.md Manual Configuration section has config blocks for each client the CLI supports | VERIFIED | `INSTALL.md:55-219` Manual Configuration section contains blocks for Claude Code (:62), Gemini CLI (:72), VS Code (:82), JetBrains (:100), Claude Desktop (:115), Codex (:134), OpenCode (:151), Cursor (:169), Antigravity (:184), Generic (:201) -- closed by `b3932958` and `7ce77d23` (restore Codex/OpenCode/Cursor/Antigravity) |
| 5 | Canonical 7-client registry reconciled with docs | VERIFIED | `internal/cli/setup_clients.go:39-45` registers exactly `claude-code, vscode, jetbrains, claude-desktop, gemini-cli, opencode, generic`; all 7 appear in `INSTALL.md:36-42` Quick Start |
| 6 | CONTRIBUTING.md describes current 4-layer project structure with `internal/` packages | VERIFIED | `CONTRIBUTING.md` project-structure section lists Layer 0 through Layer 3 packages including `internal/mcp/`, `internal/kernel/fileops/`, `internal/kernel/health/`, `internal/kernel/help/`, `internal/kernel/jsonrpc/`, `internal/repomap/`, `internal/skill/repomap/`, `internal/fuzzy/`, `internal/cli/` -- closed by `b3932958` |
| 7 | CONTRIBUTING.md fileops line reads "7 file operation tools (includes fuzzy_edit)" | VERIFIED | `CONTRIBUTING.md:53` reads verbatim: `- \`internal/kernel/fileops/\` -- 7 file operation tools (includes fuzzy_edit)` -- closed by `b3932958` |
| 8 | CONTRIBUTING.md documents 6-layer oracle test hierarchy | VERIFIED | `CONTRIBUTING.md:105-114` enumerates all 6 layers (protocol, contract, runtime, scenario, llm, judge) in a table with paths `test/oracle/{layer}/` and run commands at :119, :125 -- closed by `b3932958` |
| 9 | CONTRIBUTING.md documents integration test tags and benchstat CI gate | VERIFIED | `CONTRIBUTING.md:76` references `test/oracle/` top-level path; integration tests + benchstat sections retained alongside the oracle table; line :168 cites `test/integration/` and `test/oracle/scenario/` for end-to-end coverage -- closed by `b3932958` |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `INSTALL.md` | Rewrite with setup CLI primary path + 7-client Quick Start + manual-config superset | VERIFIED | Quick Start at `INSTALL.md:36-42` lists all 7 registered clients; Manual Configuration section at `INSTALL.md:55-219` covers 10 agent flavors (7 registered + Codex + Cursor + Antigravity); HTTP Mode section at `INSTALL.md:220` present as standalone section |
| `CONTRIBUTING.md` | Current Go project structure, 4-layer architecture, oracle + integration + benchmark test docs | VERIFIED | Project structure lists all current `internal/*` packages; fileops line at `CONTRIBUTING.md:53` reads "7 file operation tools (includes fuzzy_edit)"; 6-layer oracle table at `CONTRIBUTING.md:105-114` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| INSTALL.md Quick Start | `internal/cli/setup_clients.go:39-45` | 7-client enumeration parity | WIRED | Both list `claude-code, vscode, jetbrains, claude-desktop, gemini-cli, opencode, generic` (order-invariant set equality) |
| INSTALL.md Manual Configuration | CLI-supported clients | each registered client has a manual-config block | WIRED | All 7 registrars have blocks in `INSTALL.md:55-219`; Cursor + Antigravity + Codex additionally documented as unmanaged (additive superset) |
| CONTRIBUTING.md project structure | on-disk `internal/*` packages | path enumeration | WIRED | Spot-checked packages exist on disk; fileops pluralization at `CONTRIBUTING.md:53` reconciles with the seven tool names listed in-line |
| CONTRIBUTING.md oracle docs | `test/oracle/*` directories | 6-layer table maps to paths | WIRED | `CONTRIBUTING.md:109-114` maps each layer to `test/oracle/protocol/`, `test/oracle/contract/`, `test/oracle/runtime/`, `test/oracle/scenario/`, `test/oracle/llm/`, `test/oracle/judge/` |

### Data-Flow Trace (Level 4)

Not applicable -- documentation-only phase, no dynamic data rendering.

### Behavioral Spot-Checks

Step 7b: SKIPPED (documentation-only phase, no runnable entry points to test)

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| INST-01 | 41-01 | INSTALL.md reflects current install paths and `serena setup <client>` as primary method | SATISFIED | `INSTALL.md:5-29` Prerequisites covers `go install` + PATH verification; `INSTALL.md:31-43` Quick Start names `serena setup <client>` as the primary install path with "The fastest way to configure Serena..." framing. Evidence commits: `b3932958` (41-01 rewrite), `3491ba0d` (43-02 opencode+generic Quick Start addition). |
| INST-02 | 41-01, 43-02 | INSTALL.md has accurate MCP configs for all 7 supported clients | SATISFIED | `INSTALL.md:36-42` Quick Start lists all 7 clients matching `internal/cli/setup_clients.go:39-45` canonical registry; `INSTALL.md:55-219` Manual Configuration provides config blocks for 10 agent flavors (the 7 registered plus Codex/Cursor/Antigravity as additive unmanaged entries). Evidence commits: `b3932958`, `7ce77d23`, `3491ba0d`. |
| CONT-01 | 41-02 | CONTRIBUTING.md reflects current Go codebase structure and dev workflow | SATISFIED | CONTRIBUTING.md project-structure section lists all current `internal/*` packages including `internal/kernel/health/`, `internal/kernel/help/`, `internal/repomap/`, `internal/skill/repomap/`, `internal/fuzzy/`, `internal/cli/`, `internal/kernel/jsonrpc/`; fileops line at `CONTRIBUTING.md:53` reads "7 file operation tools (includes fuzzy_edit)". Evidence commit: `b3932958`. |
| CONT-02 | 41-02 | CONTRIBUTING.md references current test harness (oracle tests, integration tags, benchmark gates) | SATISFIED | `CONTRIBUTING.md:105-114` documents 6-layer oracle hierarchy (protocol, contract, runtime, scenario, llm, judge) with paths + run commands at :119 and :125; integration test section retained (`test/integration/`) and referenced at `CONTRIBUTING.md:168`; benchmark / benchstat CI gate sections retained alongside oracle table. Evidence commit: `b3932958`. |

### Anti-Patterns Found

None -- phase verification passed with zero anti-patterns detected.

### Human Verification Required

None -- all checks are verifiable programmatically for documentation content accuracy.

### Gaps Summary

None -- all truths verified. This artifact closes F-13 (Phase 41 unverified) from the v1.8 integration check.

---

_Verified: 2026-04-23T16:00:00Z_
_Verifier: Claude (gsd-verifier)_
