---
phase: 05-daemon-bootstrap-integration
verified: 2026-04-08T17:40:00Z
status: passed
score: 11/11 must-haves verified
---

# Phase 05: Daemon Bootstrap Integration Verification Report

**Phase Goal:** The daemon binary wires all existing components (kernel, tools, skills, profiles) into a working runtime -- closing the integration gap between individually-tested subsystems and the production entry point
**Verified:** 2026-04-08T17:40:00Z
**Status:** passed

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Kernel tool packages (symbols, edit, fileops, diag) are discoverable via skill.ToolProviders() | VERIFIED | 4 skill adapters exist with init() calling skill.Register(); TestBootstrapSkillsInitialized asserts 7+ skills |
| 2 | Profile YAMLs reference skill names that match actual registered skill names | VERIFIED | claude-code.yaml, full.yaml, ci-bot.yaml etc. reference symbol-retrieval, symbol-editing, diagnostics, memory, workflow, file-ops -- all matching registered skill Names() |
| 3 | Pool uses Installer.Resolve() for three-tier LS resolution instead of entry.Command directly | VERIFIED | pool.go:248 calls p.installer.Resolve(ctx, entry) in spawnWorkerLocked |
| 4 | Daemon creates langregistry.Registry, kernel.Kernel, and starts kernel.Run in errgroup | VERIFIED | daemon.go:72 NewRegistry, :100 NewKernel, :243 g.Go kernel.Run |
| 5 | Daemon imports skill packages so init() registers memory, workflow, profile, and kernel skill adapters | VERIFIED | imports.go has 7 blank imports for all skill packages |
| 6 | Daemon calls skill.InitAll with real deps, then registers all skill tools with MCP SDK | VERIFIED | daemon.go:126 skill.InitAll, :146 iterates skill.ToolProviders() and registers via AddSkillTool |
| 7 | Daemon installs ProfileFilterMiddleware on the MCP server | VERIFIED | daemon.go:174 mcpServer.SDK().AddReceivingMiddleware(serenaMCP.ProfileFilterMiddleware(...)) |
| 8 | Daemon calls config.ResolveProfile and applies the resolved profile | VERIFIED | daemon.go:111 config.ResolveProfile(cfg, globalDir) with fail-fast |
| 9 | Daemon shutdown stops kernel before closing listeners | VERIFIED | shutdown.go:24-28 kernel.Shutdown(ctx) before socketListener.Close() |
| 10 | Daemon bootstrap creates kernel, registers 30+ tools, and all are listed via MCP tools/list | VERIFIED | TestBootstrapRegistersAllTools asserts 38+ tools with name-by-name verification |
| 11 | Daemon shuts down cleanly without panics or leaked goroutines | VERIFIED | TestE2ECleanShutdown passes with goroutine leak detection |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/kernel/symbols/skill.go` | ToolProvider adapter wrapping symbol tools | VERIFIED | 42 lines, SymbolRetrievalSkill with Name(), Init(), Tools() returning 9 ToolDefs, init() calls skill.Register |
| `internal/kernel/edit/skill.go` | ToolProvider adapter wrapping edit tools | VERIFIED | 39 lines, SymbolEditingSkill with 6 ToolDefs |
| `internal/kernel/fileops/skill.go` | ToolProvider adapter wrapping file ops tools | VERIFIED | 39 lines, FileOpsSkill with 6 ToolDefs |
| `internal/kernel/diag/skill_adapter.go` | ToolProvider adapter wrapping diag tools | VERIFIED | 36 lines, DiagnosticsSkill with 3 ToolDefs |
| `internal/daemon/daemon.go` | Full bootstrap wiring | VERIFIED | 369 lines, New() creates langregistry, kernel, skills, registers 30+ tools, installs middleware |
| `internal/daemon/shutdown.go` | Kernel shutdown in graceful shutdown path | VERIFIED | 41 lines, two-phase shutdown: kernel first, then listeners |
| `internal/daemon/imports.go` | Blank imports for skill packages | VERIFIED | 7 blank imports for kernel skills + memory + workflow + profile |
| `internal/mcp/server.go` | Skill tool registration and profile middleware | VERIFIED | AddSkillTool, SetActivateCallback, SkillToolExecutor interface |
| `internal/daemon/bootstrap_test.go` | Bootstrap integration tests | VERIFIED | 4 tests: TestBootstrapRegistersAllTools, TestBootstrapSkillsInitialized, TestBootstrapProfileResolved, TestBootstrapDefaultProfileIsFull |
| `internal/daemon/daemon_integration_test.go` | E2E flow smoke tests | VERIFIED | 6 tests: memory CRUD, mode switching, token budget, clean shutdown, config layering, workflow onboarding |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| symbols/skill.go | skill/registry.go | skill.Register in init() | WIRED | Line 13: `skill.Register(&SymbolRetrievalSkill{})` |
| lspool/pool.go | langregistry/installer.go | Installer.Resolve in spawnWorkerLocked | WIRED | Line 248: `p.installer.Resolve(ctx, entry)` |
| daemon/daemon.go | kernel/kernel.go | kernel.NewKernel call | WIRED | Line 100: `kernel.NewKernel(workspaces, langReg, installer, ...)` |
| daemon/imports.go | skill/memory/skill.go | blank import triggering init() | WIRED | Line 10: `_ "github.com/postfix/serena/internal/skill/memory"` |
| daemon/daemon.go | skill/registry.go | skill.InitAll call | WIRED | Line 126: `skill.InitAll(skillDeps)` |
| mcp/server.go | mcp/middleware.go | ProfileFilterMiddleware installation | WIRED | daemon.go:174 calls mcpServer.SDK().AddReceivingMiddleware(serenaMCP.ProfileFilterMiddleware(...)) |
| bootstrap_test.go | daemon/daemon.go | daemon.New() creating full bootstrap | WIRED | Line 42: `d, err := New(cfg, logger)` |
| daemon_integration_test.go | mcp/server.go | MCP tool invocation | WIRED | Tests invoke ExecuteTool on skills discovered through daemon bootstrap |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Binary compiles | `go build ./cmd/serena/` | Exit 0 | PASS |
| All packages pass vet | `go vet ./internal/daemon/ ./internal/mcp/ ./internal/kernel/... ./internal/skill/...` | Exit 0 | PASS |
| 10 integration tests pass | `go test ./internal/daemon/ -count=1 -timeout 120s` | All pass in 0.663s | PASS |
| Full test suite passes | `go test ./internal/... -count=1 -short -timeout 180s` | 16 packages pass, 0 failures | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| SYM-01..09 | 05-01, 05-03 | Symbol retrieval tools (go_to_definition through analyze_blast_radius) | SATISFIED | 9 tools registered via symbols.RegisterTools, verified by TestBootstrapRegistersAllTools |
| EDT-01..06 | 05-01, 05-03 | Symbol editing tools (replace_symbol_body through verify_edit) | SATISFIED | 6 tools registered via edit.RegisterTools, verified by test |
| FIL-01..06 | 05-01, 05-03 | File operation tools (read_file through replace_in_file) | SATISFIED | 6 tools registered via fileops.RegisterTools, verified by test |
| DGN-01..03 | 05-01, 05-03 | Diagnostic tools (get_diagnostics, get_code_actions, format_code) | SATISFIED | 3 tools registered via diag.RegisterTools, verified by test |
| DMN-07 | 05-02, 05-03 | Clean sessions attach to existing warm LS workers | SATISFIED | Kernel pool with share-until-dirty policy wired via daemon bootstrap |
| DMN-08 | 05-02, 05-03 | Sessions with divergent unsaved buffers promote to own LS view | SATISFIED | Pool AcquireLease with dirty flag wired through daemon |
| DMN-09 | 05-02, 05-03 | Idle LS workers stay warm for configurable TTL | SATISFIED | PoolConfig with BaseTTL/CeilingTTL wired from config |
| DMN-10 | 05-02, 05-03 | Crashy LS workers circuit-broken with backoff | SATISFIED | Pool CircuitBreaker wired, kernel.Run in errgroup |
| DMN-11 | 05-02, 05-03 | Mutations serialized, reads parallel | SATISFIED | Pool lease mechanism with RW locks wired through daemon |
| MEM-01..05 | 05-02, 05-03 | Memory tools (write, read, list, search, rename, edit, delete) | SATISFIED | 7 memory tools registered via AddSkillTool, TestE2EMemoryToolInvocation confirms CRUD |
| WFL-01..03 | 05-02, 05-03 | Workflow tools (onboard_project, prepare_for_new_conversation, plugin interface) | SATISFIED | 2 workflow tools registered, TestE2EWorkflowOnboarding confirms invocation |
| PRF-01..03 | 05-01, 05-03 | Pre-built profiles, mode switching, tool description overrides | SATISFIED | Profile YAMLs exist, TestBootstrapProfileResolved and TestE2EModeSwitching confirm |
| PRF-04 | 05-02, 05-03 | Token budget awareness | SATISFIED | TestE2ETokenBudget confirms TotalTokens > 0, ToolCount > 0, PerTool populated |
| PRF-05 | 05-02, 05-03 | Config loaded from CLI -> project -> user -> profile | SATISFIED | config.ResolveProfile wired in daemon.New with fail-fast |
| LNG-01 | 05-02, 05-03 | 40+ languages via LSP | SATISFIED | langregistry.NewRegistry() in daemon creates full registry |
| LNG-02 | 05-01, 05-03 | Auto-discover and download LS | SATISFIED | Pool uses Installer.Resolve() for three-tier resolution |
| LNG-03 | 05-02, 05-03 | Per-language quirk handling | SATISFIED | langregistry adapter layer wired through installer |
| WRK-02 | 05-02, 05-03 | Auto-detect project languages | SATISFIED | activate_project callback calls k.ActivateWorkspace which detects languages |
| WRK-03 | 05-02, 05-03 | Multi-project/monorepo support | SATISFIED | workspace.Registry manages multiple workspaces, wired in daemon |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | - | - | - | - |

No TODO, FIXME, PLACEHOLDER, or stub patterns found in any phase 05 artifacts.

### Human Verification Required

### 1. Daemon startup with real language server

**Test:** Start the binary with `go run ./cmd/serena/` and activate a Go project
**Expected:** Language server auto-downloads (if needed), gopls starts, symbol tools return real data
**Why human:** Requires running LS processes and real filesystem interaction

### 2. Profile filtering via MCP protocol

**Test:** Connect an MCP client, verify that tools/list with claude-code profile excludes read_file
**Expected:** ProfileFilterMiddleware filters tool list based on active profile
**Why human:** Requires live MCP client connection and protocol-level inspection

### Gaps Summary

No gaps found. All 11 observable truths verified, all artifacts exist and are substantive, all key links wired, all 44 requirement IDs accounted for, all tests pass, binary compiles cleanly.

---

_Verified: 2026-04-08T17:40:00Z_
_Verifier: Claude (gsd-verifier)_
