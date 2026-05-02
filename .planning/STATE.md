---
gsd_state_version: 1.0
milestone: v1.9
milestone_name: Polish & Infra
status: completed
last_updated: "2026-05-02T09:53:16.983Z"
last_activity: 2026-05-02 -- Phase 55 marked complete
progress:
  total_phases: 12
  completed_phases: 12
  total_plans: 51
  completed_plans: 51
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-24)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** Phase 55 — obs-trace-coverage-audit

## Current Position

Phase: 55 — COMPLETE
Plan: 1 of 7
Status: Phase 55 complete
Last activity: 2026-05-02 -- Phase 55 marked complete

## Performance Metrics

**Velocity:**

- Total plans completed: 144
- Average duration: ~15 min
- Total execution time: ~18 hours

**By Milestone:**

| Milestone | Phases | Plans | Timeline |
|-----------|--------|-------|----------|
| v1.0 (1-5) | 5 | 20 | 2 days |
| v1.1 (6-8) | 3 | 11 | 2 days |
| v1.2 (9-15) | 7 | 25 | 2 days |
| v1.3 (16-17) | 2 | 4 | 1 day |
| v1.4 (18-21) | 4 | 11 | 4 days |
| v1.5 (22-24) | 3 | 12 | 1 day |
| v1.6 (25-33) | 7 | 22 | 4 days |
| v1.7 (34-38) | 5 | 11 | 2 days |
| v1.8 (39-45) | 7 | 19 | 1 day |
| v1.9 (46-55) | 10 | 0 (planning) | in flight |
| Phase 52 P01 | 5min | 2 tasks | 10 files |
| Phase 52 P02 | 3min | 2 tasks | 206 files |
| Phase 52 P03 | 11min | 2 tasks | 78 files |
| Phase 52 P04 | 15min | 4 tasks | 27 files |
| Phase 52 P05 | 6m | 1 tasks | 1 files |
| Phase 52 P06 | 18m | 2 tasks | 10 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [v1.9]: One phase per bug for BUG-01..BUG-04 (user-confirmed scoping)
- [v1.9]: TOOL-01 + TOOL-02 clustered into Phase 50 (Go 1.25/gopls fix is prerequisite for restoring the CI bench gate)
- [v1.9]: PKG-02 + PKG-03 + PKG-04 clustered into Phase 52 (all downstream of goreleaser pipeline in Phase 51; goreleaser `nfpms` handles deb/rpm natively so PKG-04 does not need its own phase)
- [v1.9]: Orphan backlog phase 999.1 promoted to Phase 46; phase directory to be renamed during `/gsd-plan-phase 46`
- [v1.9]: OBS-03 (metrics) before OBS-01/02 (dashboards/runbooks) so dashboards reference metrics that exist
- [v1.8]: Acknowledge Python legacy briefly ("Originally inspired by") -- no "port" or "rewrite" framing
- [v1.8]: Full doc overhaul scope -- README, USAGE, INSTALL, CONTRIBUTING, CHANGELOG, CLAUDE.md
- [v1.9 / Phase 52-01]: D-13 build-time embed-copy mechanism shipped (embed-pubkey + verify-embed-pubkey Makefile targets; CI gate in release.yml); embedded internal/upgrade/minisign.pub is checked in (NOT gitignored) so verify-embed-pubkey has a baseline on a fresh clone
- [v1.9 / Phase 52-01]: Test fixtures use https://example.invalid/... per RFC 6761 to ensure tests that miss the httptest stub fail loudly with DNS errors instead of silently leaking the runner IP
- [v1.9 / Phase 52-02]: Module path rename to github.com/agenthands/helix executed via mechanical perl rewrite + `go build` verification gate (Phase 52 D-01) — gopls rename does not operate on module paths; layered build/vet/test verification catches misses
- [v1.9 / Phase 52-02]: Protobuf rawDesc rule — any project-wide textual rewrite touching `.proto` files MUST be followed by `make proto` regeneration; rawDesc length-prefix bytes encode descriptor lengths and `perl` substitution invalidates the descriptor hash even when length is preserved
- [Phase ?]: Plan 52-03: D-05 'previous serena MCP registration' nudge deferred to v1.10
- [Phase ?]: Plan 52-03: SerenaConfig + SerenaMCPServer types not renamed (89 refs, structural change out of scope)
- [Phase ?]: Plan 52-03: mcp.SetVersion package-local setter avoids cli<->daemon<->mcp import cycle
- [Phase ?]: 52-04: Daemon env-var setter at Daemon.Run() entry via os.Setenv — descendants inherit through os.Environ() rather than threading through every spawn site
- [Phase ?]: 52-04: Asset-name template constant pinned to helix_v{version}_{os}_{arch}.tar.gz with parity test against .goreleaser.yaml — drift detected at PR-review time
- [Phase ?]: 52-04: Single canonical 'signature verification FAILED' error literal at three return sites in verify.go — keeps grep-based CI gate auditable as a guard against future refactors
- [Phase ?]: 52-05: EMBED-AUDIT.md ships with zero gaps + zero deferred — 24 //go:embed directives + 3 implicit/structural embeds (langregistry defaults, treesitter bindings, generated LSP types) + 14 external-by-design groups (52 LSes per D-15, ~/.helix/, project marker per D-02, memories, MCP client configs per D-05, hooks settings, /proc, os.Executable(), upgrade-time downloads, override YAMLs)
- [Phase ?]: 52-05: internal/kernel/edit/queries/*.scm (22 files) recorded as Out-of-Scope dead-reference (Phase 31 documentation; never read by any production Go file) — cleanup deferred to Phase 53+ per executor scope-boundary rule
- [Phase ?]: 52-05: cmd/docgen + cmd/lspgen excluded from audit surface — they are build-time dev tools, not part of the shipped helix binary
- [Phase ?]: Plan 52-06: PKG-02/03/04 deferred to PKG-DEFER-03/04/05; PKG-05/06/07 added in-scope; ROADMAP Phase 52 rewritten with 8 success criteria and Rescope rationale; CHANGELOG v1.9 ships Breaking Changes + self-upgrade + embed-audit reference + Known Issues
- [Phase ?]: Plan 52-06: README brand-asset image references DROPPED (broken since Go-native rewrite — SVGs only ship in legacy/); replaced with TODO for post-v1.9 brand-asset rename
- [Phase ?]: Plan 52-06: .goreleaser.yaml release.name_template 'Serena {{ .Tag }}' → 'Helix {{ .Tag }}' flipped (Plan 02 deferred this single-line goreleaser docs/marketing string to Plan 06)
- [Phase ?]: Plan 52-06: llms-install.md rewritten end-to-end (legacy Python-Serena uv-clone → Helix install + setup + upgrade) as Rule 3 blocking auto-fix; not in original files_modified list but discovered during smoke-check

### Pending Todos

None.

### Blockers/Concerns

None.

### Roadmap Evolution

- Phase 56 added: bug-ls-notification-dispatch-and-jdtls-readiness (LS notification dispatch unwired in production — `jsonrpc.Conn.OnNotification` never set, all `QuirkAdapter.NotificationHandlers()` silently dropped; jdtls has no `language/status: ServiceReady` listener, causing functional Java test failures surfaced after Phase 48)

## Deferred Items

Items acknowledged and deferred at v1.7 milestone close on 2026-04-22:

| Category | Item | Status | Notes |
|----------|------|--------|-------|
| uat_gaps | Phase 36: 36-HUMAN-UAT.md | partial | 4 scenarios require live Claude Code hooks pipeline |
| verification | Phase 34: 34-VERIFICATION.md | human_needed | Requires Claude CLI, Gemini CLI, VS Code, JetBrains |
| verification | Phase 35: 35-VERIFICATION.md | human_needed | CLI color output + exit codes |
| verification | Phase 36: 36-VERIFICATION.md | human_needed | Requires live Claude Code session |
| verification | Phase 37: 37-VERIFICATION.md | human_needed | Covered by integration test |
| verification | Phase 38: 38-VERIFICATION.md | human_needed | Covered by 3 integration tests |
