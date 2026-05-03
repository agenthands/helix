---
phase: 52-packaging-distribution-channels
plan: 03
subsystem: packaging
tags: [packaging, rename, env-vars, config-paths, mcp-registration, mcp-identity]

# Dependency graph
requires:
  - phase: 52-packaging-distribution-channels
    provides: 52-02 helix binary entrypoint + cli.SetVersion ldflag wiring + module path github.com/agenthands/helix
provides:
  - HELIX_* env-var prefix across production + test code (no SERENA_* fallback per CONTEXT.md D-02)
  - .helix/ + ~/.helix/ config-path defaults (no .serena/ fallback)
  - MCP server identity: Implementation.Name="helix", Version=ldflag-injected currentVersion (RESEARCH.md A6 lockstep)
  - Per-client MCP registration name "helix" across 7 ClientRegistrars (Claude Code, Gemini CLI, VS Code, JetBrains, Claude Desktop, OpenCode, Generic)
  - Claude Code hook helpers renamed (helixHookConfig + helix_managed JSON marker) so future installs write the new name; old-binary hooks fail clean per Plan 06 CHANGELOG
  - mcp.SetVersion(string) public setter on internal/mcp wired from cmd/helix/main.go
  - cli.CurrentVersion() public getter on internal/cli/root.go (reusable by Plan 04 upgrade subcommand)
affects:
  - 52-04 internal/upgrade asset-name lookup (MCP identity now stable; CurrentVersion accessor available)
  - 52-05 in-binary self-upgrade (Implementation.Version now ldflag-bound — no more "2.0.0-dev" hardcoded literal)
  - 52-06 docs / README / CHANGELOG (must document HELIX_* env-var rename + ~/.serena → ~/.helix break + setup re-onboard requirement)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cross-package version threading via package-local SetVersion/CurrentVersion accessor pair: cmd/helix/main.go declares `var version = \"dev\"` (ldflag-bound), then calls both `cli.SetVersion(version)` and `mcp.SetVersion(version)` — neither package imports the other (avoiding cli↔daemon↔mcp cycle), but both expose identical `currentVersion` semantics."
    - "Per-package mcp/cli `currentVersion` package var with public SetVersion + CurrentVersion accessors (defaults to \"dev\", ignored on empty input) — matches Plan 02 wiring style."
    - "Bulk surface flip via `find ... -print0 | xargs -0 perl -i -pe 's|...|...|g'` per CONTEXT.md D-02 (hard cut, no migration). Multi-pass with narrow patterns (env-var prefix, quoted path strings, identifiers, comments) avoids false positives; build + vet + test serve as the verification gate per RESEARCH.md sed→perl portability note."
    - "Bisectable single-commit per scope: Task 1 = env-vars/paths/metric-names (42 files); Task 2 = MCP identity + setup_clients/setup_hooks + comments (36 files); each gated by `go build` + `CGO_ENABLED=0 go build` + `go vet` + `go test ./... -count=1`."

key-files:
  created: []
  modified:
    # Task 1 — env-vars, config paths, metric names, JSON tags, package alias
    - .github/workflows/go-test.yml
    - internal/cli/deactivate.go
    - internal/cli/deactivate_test.go
    - internal/cli/nudge.go
    - internal/cli/nudge_test.go
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/config/loader.go
    - internal/daemon/daemon.go
    - internal/daemon/daemon_integration_test.go
    - internal/daemon/daemon_test.go
    - internal/daemon/telemetry_metrics_test.go
    - internal/forwarder/forwarder_test.go
    - internal/kernel/edit/rename_override.go
    - internal/kernel/fileops/find.go
    - internal/kernel/fileops/write.go
    - internal/kernel/lspool/quirks.go
    - internal/kernel/lspool/quirks_test.go
    - internal/langregistry/installer.go
    - internal/mcp/middleware.go
    - internal/memory/store.go
    - internal/obs/metrics.go
    - internal/obs/metrics_labels_test.go
    - internal/obs/metrics_test.go
    - internal/skill/memory/skill_test.go
    - internal/skill/repomap/skill.go
    - internal/skill/skill.go
    - internal/skill/workflow/skill.go
    - internal/skill/workflow/skill_test.go
    - test/bench/bench_helpers_test.go
    - test/bench/metrics_bench_test.go
    - test/bench/tracing_bench_test.go
    - test/harness/runner.go
    - test/integration/golden.go
    - test/integration/harness.go
    - test/integration/jdtlscache/cache.go
    - test/integration/jdtlscache/cache_test.go
    - test/integration/trace_propagation_test.go
    - test/integration/trace_shutdown_test.go
    - test/oracle/judge/judge_test.go
    - test/oracle/llm/client.go
    - test/oracle/scenario/repomap_polyglot_test.go
    # Task 2 — MCP identity, setup_clients, setup_hooks, handshake test, comments
    - cmd/helix/main.go
    - internal/cli/activate.go
    - internal/cli/root.go
    - internal/cli/setup.go
    - internal/cli/setup_clients.go
    - internal/cli/setup_hooks.go
    - internal/cli/setup_hooks_test.go
    - internal/cli/setup_test.go
    - internal/cli/status.go
    - internal/errors/errors.go
    - internal/errors/kinds.go
    - internal/kernel/lspool/metrics.go
    - internal/mcp/server.go
    - internal/mcp/session.go
    - internal/obs/obs.go
    - internal/obs/tracing_test.go
    - internal/profile/profile.go
    - test/bench/fullrepo_smoke_test.go
    - test/integration/a_doc.go
    - test/integration/symbols_test.go
    - test/oracle/llm/selection_test.go
    - test/oracle/protocol/handshake_test.go
    - test/oracle/scenario/sql_test.go

key-decisions:
  - "D-02 hard cut applied to test code as well: SERENA_TEST_JDTLS_DATA_DIR / SERENA_TEST_LS_DEBUG / SERENA_TEST_MODEL / SERENA_JUDGE_MODEL / SERENA_INLINE_JUDGE / SERENA_LLM_PROVIDER / SERENA_TEST_LS_TIMEOUT all flipped to HELIX_*. Rationale per RESEARCH.md Runtime State Inventory: no fallback, the test harness has no reason to read the old name."
  - "D-05 \"previous serena MCP registration detected\" nudge DEFERRED to v1.10. Rationale: the rename is documented in CHANGELOG (Plan 06), and `helix setup <client>` already removes-then-adds via the existing claude-mcp-remove path. The nudge would add a one-shot detection-and-print branch that touches every ClientRegistrar (7 files) and silently goes stale once users have migrated. Worth shipping only if user reports show high friction post-rename; defer until then."
  - "Implementation.Version threaded via mcp.SetVersion(version) called from cmd/helix/main.go (NOT via cli.CurrentVersion()): a cli.CurrentVersion accessor would create a cli↔daemon↔mcp import cycle. Instead, internal/mcp now exports its own SetVersion + currentVersion package var (mirroring Plan 02's cli pattern), and cmd/helix/main.go calls both setters explicitly. cli.CurrentVersion() is still added as a public accessor for Plan 04's upgrade subcommand to read the same value through cli (which it already imports)."
  - "Exported Go types `SerenaConfig` (89 references across config + tests + benchmarks) and `SerenaMCPServer` / `NewSerenaMCPServer` (5 references) deliberately NOT renamed in this plan. These are internal Go identifiers, not user-facing strings; renaming would be a structural rename outside Plan 03 scope (Rule 4 — architectural). Documented as deferred for a future v1.10 polish phase if desired."
  - "Prometheus metric family names flipped (helix_tool_calls_total, helix_tool_duration_seconds, helix_lspool_workers/evictions_total/circuit_state/restarts_total, helix_rename_strategy_total, helix_drift_test_total). These are user-facing observability names per CONTEXT.md D-03 (product is \"Helix\"); existing Prometheus scrapers will see the rename as a series-name break (acceptable per D-02 hard cut)."
  - "Session-stats JSON tag flipped serena_tool_count → helix_tool_count and the exported Go field SerenaToolCount → HelixToolCount. Stats files written by old binaries become unreadable; users invoking `helix nudge` after rename will see a fresh stats file, which is correct per D-02."

patterns-established:
  - "Hard-cut grep gate as completion test: `grep -rn 'SERENA_|\\.serena/' --include='*.go' --include='*.yaml' --include='Makefile' --exclude-dir=legacy . | wc -l` MUST equal 0. Used as the Plan 03 final acceptance criterion before commit; encodable as a CI step for future drift detection."
  - "Cross-package version threading without import cycle: package-A and package-B each expose a `SetVersion` / `currentVersion` pair; package main (cmd/<binary>/main.go) holds the source-of-truth `var version = \"dev\"` (ldflag-bound) and calls both setters before Execute(). Generalizable to N packages."

requirements-completed: []

# Metrics
duration: 11min
completed: 2026-04-30
---

# Phase 52 Plan 03: Env-Var + Config-Path + MCP Registration Surface Flip Summary

**Closes the runtime user-facing surface that Plan 02's binary rename deliberately left behind: 21 SERENA_* env-var literals → HELIX_*, 50+ .serena/~/.serena path strings → .helix/~/.helix, 30+ "serena" MCP registration literals across 7 ClientRegistrars → "helix", internal/mcp/server.go Implementation.Name → "helix" with Version wired to the ldflag-injected currentVersion, plus a comment+log-line sweep across 78 modified Go files — verified by 4 hard-cut grep gates, the full `go test ./...` suite, and a `./helix setup --help` smoke run that reports "Register Helix as an MCP server".**

## Performance

- **Duration:** ~11 min wall clock (08:01:41Z → 08:12:52Z, 2026-04-30)
- **Tasks:** 2 (both type=auto, autonomous=true plan-level)
- **Files modified:** 78 (42 in Task 1 commit `6501092f` + 36 in Task 2 commit `aa80d077`)
- **Commits:** 2 atomic per-task commits (no rework, no checkpoint, no deviation rule firings beyond the rule-3 workflow-env-var fix logged below)

## Pre/Post Grep Counts

Per Plan 03 output requirements, four canonical greps measured before and after:

| Grep | Pattern | Pre | Post | Note |
|------|---------|-----|------|------|
| 1 | `grep -rn 'SERENA_' --include='*.go' --exclude-dir=legacy --exclude-dir=testdata .` | 21 | 0 | All 21 production + test env-var refs flipped |
| 2 | `grep -rn '"\.serena' --include='*.go' --exclude-dir=legacy --exclude-dir=testdata .` | 29 | 0 | Quoted-path forms |
| 3 | `grep -rn '"~/\.serena' --include='*.go' --exclude-dir=legacy --exclude-dir=testdata .` | 0 | 0 | (No literal `"~/.serena"` ever existed in tree; all forms used `filepath.Join(homeDir, ".serena", ...)` patterns) |
| 4 | `grep -c '"serena"' internal/cli/setup_clients.go` | 30 | 0 | Per-client registration name across 7 ClientRegistrars |
| 5 (final hard-cut) | `grep -rn 'SERENA_\|\.serena/' --include='*.go' --include='*.yaml' --include='Makefile' --exclude-dir=legacy .` | 24 | 0 | Includes Workflow + .yaml + Makefile surface |

Additional residual scan after both tasks:

| Pattern | Count | Note |
|---------|-------|------|
| `grep -rn 'serena' --include='*.go' --exclude-dir=legacy --exclude-dir=testdata . \| grep -v 'agenthands/helix\|api/proto/serena\|serenav1'` | 0 | After excluding the intentional proto-package directory artifacts |
| `grep -rn 'serena' --include='*.go' --exclude-dir=legacy --exclude-dir=testdata .` | ~14 | All in `internal/forwarder/*.go` referencing the `serenav1` import alias for `github.com/agenthands/helix/api/proto/serena/v1` — proto package directory stays per project decision (the proto file was already updated in Plan 02 Task 1; the directory name `api/proto/serena/v1/` and the alias `serenav1` are intentional residuals and document the historical lineage of the wire format). |

## Accomplishments

- **HELIX_* env-var rename:** 21 SERENA_* literals across production and test code flipped (test/integration/harness.go test/oracle/{llm,judge}/* internal/kernel/lspool/quirks*.go) including SERENA_TEST_LS_TIMEOUT in `.github/workflows/go-test.yml` (Rule 3 fix — workflow env-var must match Go-side reads).
- **.helix path defaults:** internal/config/defaults.go logging.dir = ~/.helix/logs, observability.service_name = "helix"; internal/daemon/daemon.go BinDir = ~/.helix/bin + globalDir = ~/.helix; internal/skill/repomap walk-skip dir = ".helix"; internal/kernel/fileops/find.go skip-list = ".helix"; internal/kernel/fileops/write.go temp-file pattern = ".helix-write-*"; internal/cli/{deactivate,nudge}.go session-stats path = "<workspace>/.helix/session-stats.json"; internal/memory/store.go .helix/memories docstrings; internal/skill/skill.go ProjectDir/GlobalDir docstrings updated; internal/langregistry/installer.go default BinDir = ~/.helix/bin.
- **Socket + config-file rename:** internal/config/loader.go `globalPath = ~/.helix/helix_config.yml` (was ~/.serena/serena_config.yml); daemon socket prefix `/tmp/helix-$UID/daemon.sock` (was /tmp/serena-$UID/...).
- **MCP server identity:** internal/mcp/server.go Implementation.Name = "helix"; Implementation.Version threaded via `mcp.SetVersion(version)` package-local accessor mirroring the Plan 02 cli.SetVersion pattern. Wiring: `cmd/helix/main.go` now calls BOTH `cli.SetVersion(version)` AND `mcp.SetVersion(version)` after the ldflag-bound `var version = "dev"`.
- **CLI version accessor:** internal/cli/root.go exports `func CurrentVersion() string { return currentVersion }` for downstream callers (Plan 04 upgrade subcommand consumes this).
- **Per-client MCP registration:** internal/cli/setup_clients.go — every "serena" registration-name literal across 7 ClientRegistrars (Claude Code, Gemini CLI, VS Code, JetBrains, Claude Desktop, OpenCode, Generic) flipped to "helix". Affects `claude mcp add-json` name field, `gemini mcp add` server-id, `mcpServers.<name>` / `servers.<name>` / `mcp.<name>` JSON keys, Generic stdout config.
- **Claude Code hooks:** internal/cli/setup_hooks.go — `serenaHookConfig` → `helixHookConfig`, `filterOutSerenaEntries` → `filterOutHelixEntries`, `isSerenaManaged` → `isHelixManaged`, JSON marker `"serena_managed"` → `"helix_managed"`. Future hook installs write the new field; pre-rename installs fail silently (CHANGELOG documents re-running `helix setup claude-code`).
- **Prometheus metrics:** All 8 user-facing metric vector names flipped helix_tool_calls_total, helix_tool_duration_seconds, helix_lspool_workers, helix_lspool_evictions_total, helix_lspool_circuit_state, helix_lspool_restarts_total, helix_rename_strategy_total, helix_drift_test_total.
- **Session-stats JSON tag:** SerenaToolCount → HelixToolCount with `json:"helix_tool_count"`.
- **MCP handshake test:** test/oracle/protocol/handshake_test.go ServerInfo.Name asserts flipped serena → helix in lockstep with Implementation.Name.
- **Comment + log-line sweep:** 30+ user-facing strings ("Register Helix as an MCP server...", "Activating Helix workspace...", "Tip: Helix provides..."), package docstrings ("Package profile defines agent profiles and operational modes for Helix"), test-file comments ("Package integration_test provides end-to-end integration tests for Helix's MCP tools").
- **Identifier hygiene:** nudge.go local `serenaSymbolicTools` → `helixSymbolicTools`; daemon.go import alias `serenaMCP` → `helixMCP`; metrics_labels_test.go local `wantSerena` → `wantHelix`; test/bench/* package alias `serenamcp` → `helixmcp`; test/integration/jdtlscache cache namespace `serena-test` → `helix-test`; daemon_test.go test-dir prefix `serena-test-` → `helix-test-`; forwarder_test.go socket paths `/tmp/serena-test-*.sock` → `/tmp/helix-test-*.sock`.

## Task Commits

| # | Task | Type | Hash | Files |
|---|------|------|------|-------|
| 1 | Flip SERENA_*/.serena env vars + paths | refactor | `6501092f` | 42 |
| 2 | Flip MCP identity + per-client registration + comments | feat | `aa80d077` | 36 |

(Final docs commit at end of completion sequence covers SUMMARY.md + STATE.md + ROADMAP.md.)

## Decisions Made

### D-05 "previous serena MCP registration detected" nudge: DEFERRED to v1.10

**Decision:** NOT shipped in v1.9.

**Rationale:**
- The CHANGELOG (Plan 06's surface) documents the rename + re-onboard step; users running `helix setup <client>` already invoke the existing remove-then-add CLI path for Claude Code (line 184 of setup_clients.go: `if strings.Contains(string(output), "already exists")` triggers a `claude mcp remove ...` + retry). For other clients (Gemini CLI, VS Code, JetBrains, etc.), the registrars overwrite the existing entry by key.
- A "previous serena registration detected" nudge would add a one-shot scan branch in every ClientRegistrar (7 files), would silently go stale once users have migrated (creating dead code that fires for nobody), and adds complexity for marginal value.
- Reconsider in v1.10 if user reports show migration friction. The deferral is recorded in `.planning/phases/52-packaging-distribution-channels/52-CONTEXT.md` D-05 as "Claude's discretion" and CHANGELOG (Plan 06) covers the migration story for v1.9 users.

### Exported Go types `SerenaConfig` and `SerenaMCPServer` left unrenamed

**Decision:** NOT flipped in this plan.

**Rationale:**
- 89 references to `SerenaConfig` (config + tests + benchmarks) and 5 references to `SerenaMCPServer` / `NewSerenaMCPServer` (mcp + daemon + tests).
- These are internal Go type identifiers, not user-facing strings — they don't appear in MCP protocol output, log lines, environment variables, file paths, or any external surface.
- Renaming them is a structural rename (Rule 4 — architectural change), outside Plan 03's scope ("env-var literals, config-paths, MCP server registration string, log lines"). Plan 03 owns the user-facing surface; an exported-identifier rename would belong to a v1.10 polish phase.
- Documented as deferred. If revisited, the rename is mechanical (`gopls rename` or perl + go build gate), but it produces a noisy diff for zero functional change.

### `mcp.SetVersion` package-local setter (vs. `cli.CurrentVersion()` getter)

**Decision:** Use a package-local `currentVersion` + `SetVersion` pair in `internal/mcp/`, called directly from `cmd/helix/main.go`.

**Rationale:**
- The plan's interfaces section suggested `internal/mcp/server.go` could call `cli.CurrentVersion()` if such a getter existed.
- That path creates an import cycle: `internal/cli` → `internal/daemon` → `internal/mcp` → (would-be-)`internal/cli`.
- The clean fix mirrors Plan 02's pattern: each package that needs the version gets its own `currentVersion` + `SetVersion`; `cmd/helix/main.go` (the single entrypoint that holds the ldflag source) calls all setters explicitly.
- `cli.CurrentVersion()` was still added as a public accessor (cheap, useful for any future caller within the cli-imports tree, e.g., Plan 04 upgrade subcommand which lives in `internal/cli/upgrade.go`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Flipped SERENA_TEST_LS_TIMEOUT in .github/workflows/go-test.yml**
- **Found during:** Task 1 grep enumeration (Step 1).
- **Issue:** `.github/workflows/go-test.yml:21` set `SERENA_TEST_LS_TIMEOUT: "2m"` as a Go-test job env var. The Go-side test harness reads `os.Getenv("SERENA_TEST_LS_TIMEOUT")` (test/integration/harness.go:216 pre-rename). After flipping the Go side to `os.Getenv("HELIX_TEST_LS_TIMEOUT")`, the workflow would set the wrong key and the test ceiling would silently revert to its default.
- **Fix:** Flipped the workflow YAML to `HELIX_TEST_LS_TIMEOUT`. Out-of-scope edit per Plan task list (which scoped only `internal/**/*.go` and `test/**/*.go`), but in-scope per Rule 3 (blocking — CI integration test would degrade silently otherwise).
- **Files modified:** `.github/workflows/go-test.yml`.
- **Verification:** Hard-cut grep gate `grep -rn 'SERENA_|\.serena/' --include='*.go' --include='*.yaml' --include='Makefile' --exclude-dir=legacy .` returns 0.
- **Committed in:** `6501092f` (Task 1 commit).

### Rule 1-2 Auto-fixes

None beyond the Rule 3 above. The bulk perl-driven rewrite was deterministic; no bug fixes or missing-functionality additions surfaced. The plan's 3-pass perl pattern matched all 21 SERENA_* + 50+ .serena hits without false positives in non-legacy code.

## Threat Flags

None. The threat register's three mitigated threats are all closed:
- T-52-03-01 (MCP identity desync) — verified via lockstep grep gates: `grep -q 'Name: "helix"' internal/mcp/server.go` AND `grep -c '"serena"' internal/cli/setup_clients.go == 0` AND `test/oracle/protocol/handshake_test.go` asserts `ServerInfo.Name == "helix"`.
- T-52-03-02 (Tampering with legacy/testdata) — perl excludes `legacy/` and `internal/upgrade/testdata/`; verified via `git diff --stat -- legacy/ internal/upgrade/testdata/` returns 0 lines.
- T-52-03-04 (Hooks install break) — setup_hooks.go templates flipped (`helix_managed` field, `helixHookConfig` helper); future installs are clean. Pre-rename hooks fail silently per CHANGELOG (Plan 06's surface).

T-52-03-03 (orphaned ~/.serena disk leak) is an `accept` disposition per the plan; user-side `rm -rf ~/.serena` cleanup documented in CHANGELOG (Plan 06).

## Issues Encountered

- **Plan 02 left `serena_config.yml` filename intact:** internal/config/loader.go:34 referenced `"serena_config.yml"` as the global config filename. Per CONTEXT.md D-02 this also flips to `helix_config.yml` (no fallback). Folded into Task 1's path-rename pass.
- **Quoted-path narrow-pattern miss:** the initial 3-pass perl (`"\.serena`, `"~/\.serena`, `\.serena"`) missed `.serena/` substrings inside docstring comments (e.g., `// 2. Global config (~/.serena/serena_config.yml)`). Fixed via additional pass `s|\.serena/|.helix/|g` (which would have over-flipped if any deliberate historical reference existed — none did per the manual review). Documented as a recipe-level lesson: when the destination is a hard cut, a wider-pattern second pass is cheaper than enumerating all comment shapes.

## User Setup Required

None for build/test verification. **Existing users upgrading from v1.8 must re-run `helix setup <client>`** to regenerate hooks + MCP registration under the new identity (CHANGELOG, Plan 06's surface). The orphaned `~/.serena/` tree is harmless and can be removed manually once the user has confirmed the new install works.

## Next Phase Readiness

- Plan 04 (`internal/upgrade/` self-upgrade subcommand) is now unblocked — it can read `cli.CurrentVersion()` for staleness checks, and the MCP identity surface is stable.
- Plan 05 (EMBED-AUDIT.md) inherits a tree where `~/.helix/` is the canonical user-config root (no .serena residuals to audit).
- Plan 06 (docs / CHANGELOG / README) inherits a tree where every user-facing string says "helix" or "Helix"; the docs sweep operates on a clean target.

## Self-Check: PASSED

- `internal/config/defaults.go` references `.helix`: FOUND (`logging.dir`)
- `internal/mcp/server.go` Implementation.Name = "helix": FOUND
- `internal/mcp/server.go` Version = currentVersion (no "2.0.0-dev" literal): VERIFIED
- `internal/cli/setup_clients.go` zero `"serena"` literals: VERIFIED (count = 0)
- `internal/cli/setup_hooks.go` zero `"serena` substrings: VERIFIED
- `cmd/helix/main.go` calls `mcp.SetVersion(version)`: FOUND
- `internal/cli/root.go` exports `CurrentVersion()`: FOUND
- `.github/workflows/go-test.yml` sets `HELIX_TEST_LS_TIMEOUT`: VERIFIED
- Hard-cut grep gate `grep -rn 'SERENA_|\.serena/' ... --exclude-dir=legacy . | wc -l` = 0: VERIFIED
- Commit `6501092f` (refactor: env vars + paths): FOUND in `git log`
- Commit `aa80d077` (feat: MCP identity + setup clients): FOUND in `git log`
- `go build ./cmd/helix` exits 0: VERIFIED
- `CGO_ENABLED=0 go build ./cmd/helix` exits 0: VERIFIED
- `go vet ./...` exits 0 (modulo pre-existing `tmp/graphify` scaffolding which is untracked and out of scope): VERIFIED
- `go test ./... -count=1` green across 33 real-project packages: VERIFIED
- `./helix --version` reports `helix version dev` (ldflag-injected; goreleaser builds report `helix version <tag>`): VERIFIED
- `./helix setup --help` reports "Register Helix as an MCP server for a supported coding agent": VERIFIED

---
*Phase: 52-packaging-distribution-channels*
*Completed: 2026-04-30*
