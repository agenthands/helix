# Architecture Patterns

**Domain:** Developer Experience & Auto-Setup for MCP Code Intelligence Platform
**Researched:** 2026-04-20

## Recommended Architecture

The v1.7 DX features map cleanly onto the existing 4-layer architecture. No new layers needed. Each feature is either a new component in an existing layer or a modification to an existing component.

### Integration Map

```
Layer 0 (MCP Runtime)
  internal/mcp/server.go       -- MODIFY: lazy init interceptor, smart error wrapper
  internal/mcp/middleware.go    -- MODIFY: add ErrorEnrichmentMiddleware
  internal/mcp/registry.go     -- MODIFY: progressive description support

Layer 0 (CLI)
  internal/cli/root.go         -- MODIFY: register setup subcommand
  internal/cli/setup.go        -- NEW: setup command orchestrator
  internal/cli/hooks.go        -- NEW: client hook generation/installation

Layer 1 (Kernel)
  internal/kernel/workspace.go -- MODIFY: expose health state (active LSes, indexing status)

Layer 2 (Skills)
  internal/skill/health/       -- NEW: health/status MCP tool skill

Layer 3 (Profiles)
  internal/profile/profiles/   -- MODIFY: hook templates per client, progressive descriptions
  internal/profile/hooks/      -- NEW: hook spec + generator per client type
```

### Component Boundaries

| Component | Responsibility | Communicates With | New/Modified |
|-----------|---------------|-------------------|--------------|
| `internal/cli/setup.go` | Orchestrates `serena setup <client>`: detect languages, install LSes, write MCP config, install hooks | langregistry, installer, profile, hooks | NEW |
| `internal/cli/hooks.go` | Generate and install client-specific hook files (Claude Code, VS Code, JetBrains) | profile/hooks, filesystem | NEW |
| `internal/profile/hooks/` | Hook templates and specs per client type (PreToolUse, SessionStart, Stop) | profile store | NEW |
| `internal/skill/health/` | `get_health` MCP tool: active LSes, indexing state, workspace capabilities | kernel (Pool, WorkspaceRuntime), langregistry | NEW |
| `internal/mcp/lazyinit.go` | Middleware that triggers workspace activation on first tool call if no workspace active | daemon's ActivateCallback, workspace.Registry | NEW |
| `internal/mcp/smarterror.go` | ErrorEnrichmentMiddleware: intercepts tool errors, adds suggestions | errors package (Kind-based matching), tool registry (for name similarity) | NEW |
| `internal/mcp/registry.go` | Extended ToolDef with progressive description tiers | existing registry | MODIFIED |

### Data Flow

**Setup CLI flow:**
```
serena setup claude-code
  |
  +--> langregistry.NewRegistry() -- detect languages in cwd
  +--> installer.Resolve() per detected language -- pre-install LSes
  +--> profile/hooks.Generate("claude-code") -- emit hook files
  +--> write MCP config JSON to client's config location
  +--> print summary: languages detected, LSes installed, hooks written
```

**Lazy init flow (first tool call without activate_project):**
```
Agent calls any kernel tool (e.g., get_symbols_overview)
  |
  +--> LazyInitMiddleware intercepts (receiving middleware on tools/call)
  |    Check: is workspace active? (activeWSKey.RepoRoot != "")
  |    NO  --> infer repo root from tool args or cwd
  |            call daemon's ActivateCallback(ctx, inferredRoot)
  |            proceed to actual tool handler
  |    YES --> pass through
```

**Health tool flow:**
```
Agent calls get_health
  |
  +--> health skill queries kernel.Pool().Stats()
  +--> health skill queries kernel active WorkspaceRuntime
  +--> health skill queries langregistry for capabilities
  +--> returns structured JSON: {active_ls: [...], indexing: bool, languages: [...], capabilities: {...}}
```

**Smart error flow:**
```
Tool handler returns *serr.Error
  |
  +--> ErrorEnrichmentMiddleware (receiving middleware, runs after tool handler)
  |    Match error Kind:
  |    - InvalidArgs --> suggest correct param names/types from tool schema
  |    - NotFound    --> suggest similar tool names or check workspace activation
  |    - NoWorkspace --> suggest "call activate_project first" or trigger lazy init
  |    - Unsupported --> explain which languages/capabilities support the operation
  |    Append suggestion to error text field in CallToolResult
```

**Progressive descriptions flow:**
```
Agent calls tools/list
  |
  +--> ProfileFilterMiddleware filters tools as today
  +--> DescriptionMiddleware (or registry enhancement):
  |    For each tool, select description tier based on session state:
  |    - Tier 0 (cold start): full description with usage examples
  |    - Tier 1 (after first successful call): compact description
  |    Tier selection from session call counter or explicit mode
```

## Detailed Design Per Feature

### 1. Setup CLI (`internal/cli/setup.go`)

**What:** New cobra subcommand `serena setup <client>` breaking the current "flat CLI with flags, no subcommands" pattern (D-02). This is intentional -- setup is a one-time user-facing operation distinct from the daemon runtime.

**Integration points:**
- Reuses `langregistry.NewRegistry()` for language detection (same as daemon.New step 1)
- Reuses `langregistry.NewInstaller()` for LS pre-installation (same as daemon.New step 2)
- Reads `profile.ProfileStore` to know which hooks a client needs
- Writes to client-specific config locations (e.g., `~/.claude/claude_desktop_config.json` for Claude Code)

**Decision: subcommand vs flag.** Setup is not a daemon mode -- it runs once and exits. A subcommand is the right pattern. Add `rootCmd.AddCommand(setupCmd)` in root.go.

**Supported clients (initial):**
- `claude-code` -- write to MCP settings, install hooks in `~/.claude/`
- `vscode` -- write to VS Code settings.json MCP section
- `cursor` -- same as vscode but different settings path

```go
// internal/cli/setup.go
type SetupConfig struct {
    Client      string   // "claude-code", "vscode", "cursor"
    ProjectDir  string   // defaults to cwd
    Languages   []string // auto-detected if empty
    SkipInstall bool     // skip LS pre-installation
    SkipHooks   bool     // skip hook installation
}
```

### 2. Lazy Workspace Init (`internal/mcp/lazyinit.go`)

**What:** Receiving middleware that auto-activates workspace on first kernel tool call.

**Why middleware, not per-tool logic:** Every kernel tool already checks `NoWorkspace` and returns an error. Intercepting at the middleware level avoids modifying 24+ tool handlers. The middleware runs before the tool handler, checks workspace state, and activates if needed.

**Integration points:**
- Needs access to `activeWSKey` (or a func that returns it) -- same pattern as `wsKeyFn` in daemon.go
- Needs access to `ActivateCallback` -- already exposed on SerenaMCPServer
- Needs to infer repo root: check tool args for `relative_path`, fall back to cwd detection

**Key design decision:** The middleware must be idempotent and fast. After first activation, it becomes a no-op check (single atomic load). Use `sync.Once` or atomic bool.

```go
// internal/mcp/lazyinit.go
func LazyInitMiddleware(
    isActive func() bool,
    activate func(ctx context.Context, root string) error,
    inferRoot func(args map[string]any) string,
) mcpsdk.ReceivingMiddleware
```

**Wiring in daemon.go:** Install after profile middleware, before telemetry.

### 3. Client Hooks (`internal/profile/hooks/`)

**What:** Hook templates that clients execute at lifecycle events.

**Claude Code hooks:**
- `PreToolUse` -- remind agent of workspace context, suggest activate_project if not active
- `SessionStart` -- activate workspace, run health check
- `Stop` -- cleanup (deactivate workspace, flush memory)

**Format:** Claude Code uses hook files. VS Code uses tasks.json. JetBrains uses run configurations.

**Integration points:**
- Hook templates live in `internal/profile/hooks/` as embedded Go templates
- `setup.go` calls hook generator to produce client-specific files
- Templates reference Serena tool names (e.g., `activate_project`, `get_health`)

```go
// internal/profile/hooks/hooks.go
type HookSpec struct {
    Event   string // "pre_tool_use", "session_start", "stop"
    Client  string // "claude-code", "vscode"
    Content string // rendered template
    Path    string // where to write
}

func GenerateHooks(client string, projectDir string) ([]HookSpec, error)
```

### 4. Health/Status MCP Tool (`internal/skill/health/`)

**What:** New `get_health` MCP tool as a skill (Caddy-style init registration).

**Why skill, not kernel tool:** Health is cross-cutting -- it reports on kernel state, LS pool, workspace, and language capabilities. It does not need direct LS communication. Skill is the right abstraction.

**Integration points:**
- Needs read access to `kernel.Pool().Stats()` -- Pool already has Stats() or similar
- Needs read access to workspace runtime languages
- Needs langregistry for capability reporting
- SkillDeps needs extension: add `KernelHealthProvider` interface to avoid importing kernel directly

**Design: dependency injection via interface.**

```go
// internal/skill/health/health.go
type KernelHealth interface {
    ActiveWorkers() []WorkerInfo
    IsIndexing() bool
    WorkspaceLanguages() []string
    WorkspaceRoot() string
}

// Skill implements skill.Skill + skill.ToolProvider
type HealthSkill struct {
    health KernelHealth
}
```

**Wiring:** Use a setter pattern like `repomap.SetWorkspaceRoot()`. After daemon creates kernel, call `health.SetKernelHealth(adapter)` where adapter wraps kernel.Pool and workspace state.

### 5. Smart Error Responses (`internal/mcp/smarterror.go`)

**What:** Post-execution error enrichment in middleware.

**Why middleware:** Errors already flow through TelemetryMiddleware which classifies by Kind. Adding suggestion text is a natural extension of the same pipeline.

**Recommendation: separate middleware.** Keeps concerns clean. TelemetryMiddleware records metrics; ErrorEnrichmentMiddleware adds user-facing suggestions.

**Integration points:**
- Reads `*serr.Error` Kind from tool results (already available -- v1.5 migrated all tools)
- Reads tool schema from registry for InvalidArgs suggestions
- Reads tool names from registry for "did you mean?" on NotFound

**Suggestion rules:**

| Error Kind | Suggestion |
|------------|-----------|
| `NoWorkspace` | "Call activate_project with your repo path first, or use `serena setup` for automatic configuration." |
| `InvalidArgs` | "Parameter '{param}' expects {type}. See tool schema." + list valid params |
| `NotFound` | "Symbol '{name}' not found. Check spelling, or use search_symbols for fuzzy matching." |
| `Unsupported` | "Operation not supported for {language}. Supported: {list}." |
| `CircuitOpen` | "Language server for {lang} is temporarily unavailable. It will retry automatically." |

### 6. Progressive Tool Descriptions (`internal/mcp/registry.go` modification)

**What:** Tool descriptions that adapt based on session state.

**Recommendation: Two tiers -- detailed and compact.** Keep it simple. Two description fields: `Description` (compact, always present) and `DetailedDescription` (verbose, shown to cold sessions). Session tracks tool call count; after N successful calls, tools/list returns compact descriptions.

**Integration points:**
- Modify `ToolDef` in `internal/mcp/registry.go` to add `DetailedDescription`
- Modify `ProfileFilterMiddleware` (it already touches tools/list) to swap descriptions based on session state
- Session state: add `ToolCallCount` to `SessionInfo` in `internal/mcp/session.go`

### 7. Error-Only Reporting

**What:** Suppress verbose success output, surface only actionable failures.

**This is not a new component.** It is a policy change in existing tool handlers. Each tool's success response should return structured data without verbose explanatory text. Error responses should include actionable guidance (handled by smart error middleware above).

**Implementation:** Audit existing tool response strings. Remove "Success: " prefixes and explanatory padding. Return clean structured data. This is a refactoring task across tool handlers, not an architecture change.

## Patterns to Follow

### Pattern 1: Middleware for Cross-Cutting Concerns
**What:** Use MCP SDK receiving middleware for lazy init, error enrichment, and description adaptation.
**When:** Feature needs to intercept all tool calls or tools/list without modifying individual tool handlers.
**Why:** Serena already uses this pattern for telemetry and profile filtering. Adding more middleware is low-risk and consistent.

### Pattern 2: Skill for New MCP Tools
**What:** New MCP tools (health) register as skills via Caddy-style init().
**When:** The tool does not need direct LS communication and can work through interfaces.
**Why:** Consistent with memory, workflow, repomap skills. Daemon registers centrally.

### Pattern 3: Interface-Based Dependency Injection for Skills
**What:** Skills depend on kernel state through narrow interfaces, not direct kernel imports.
**When:** Skill needs kernel data (pool stats, workspace state) but should not import kernel package.
**Why:** Avoids import cycles. RepoMap skill already uses setter pattern with FallbackDeps.

### Pattern 4: Cobra Subcommand for User-Facing CLI
**What:** `serena setup` as a subcommand, breaking D-02 flat CLI for good reason.
**When:** One-time user operations that are not daemon modes.
**Why:** Setup is fundamentally different from runtime -- it configures the environment and exits. Flags would be confusing.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Per-Tool Lazy Init Checks
**What:** Adding `if !workspaceActive { activate() }` to each of 24+ kernel tool handlers.
**Why bad:** Duplicated logic, easy to miss tools, inconsistent behavior.
**Instead:** Single middleware intercept point.

### Anti-Pattern 2: Health Tool in Kernel Package
**What:** Putting get_health as a kernel tool alongside symbol/edit/fileops tools.
**Why bad:** Health is cross-cutting, not an LSP operation. Kernel tools all go through LS workers.
**Instead:** Skill with interface-based access to kernel state.

### Anti-Pattern 3: Hardcoded Client Paths
**What:** Embedding client config paths (e.g., `~/.claude/`) directly in setup logic.
**Why bad:** Paths change between OS and client versions.
**Instead:** Client spec structs with configurable paths, OS-aware defaults.

### Anti-Pattern 4: Over-Engineering Progressive Descriptions
**What:** Complex ML-driven description adaptation, per-agent learning, or multi-tier cascades.
**Why bad:** Two tiers (verbose/compact) cover 95% of the value. More complexity means more bugs.
**Instead:** Two tiers, simple session call counter threshold.

## New vs Modified Components

### New Components (create from scratch)

| Component | Package | Layer | LOC Estimate | Dependencies |
|-----------|---------|-------|-------------|-------------|
| Setup CLI command | `internal/cli/setup.go` | 0 | 200-300 | langregistry, installer, profile/hooks |
| Hook generator | `internal/cli/hooks.go` | 0 | 150-200 | profile/hooks |
| Hook specs/templates | `internal/profile/hooks/` | 3 | 200-250 | embed, text/template |
| Health skill | `internal/skill/health/` | 2 | 150-200 | skill interface, KernelHealth interface |
| Lazy init middleware | `internal/mcp/lazyinit.go` | 0 | 80-120 | mcp sdk, workspace state |
| Error enrichment middleware | `internal/mcp/smarterror.go` | 0 | 150-200 | errors package, tool registry |

### Modified Components

| Component | Change | Scope |
|-----------|--------|-------|
| `internal/cli/root.go` | Add setup subcommand | Small (5-10 lines) |
| `internal/mcp/registry.go` | Add DetailedDescription to ToolDef | Small (10-20 lines) |
| `internal/mcp/session.go` | Add ToolCallCount to SessionInfo | Small (5-10 lines) |
| `internal/mcp/middleware.go` | Wire new middleware in InstallMiddleware | Small (10-15 lines) |
| `internal/daemon/daemon.go` | Wire health skill deps, lazy init, setup imports | Medium (30-50 lines) |
| `internal/daemon/imports.go` | Blank import for health skill | Trivial (1 line) |
| `internal/skill/skill.go` | Extend SkillDeps with KernelHealthProvider | Small (5-10 lines) |
| `internal/kernel/lspool/` | Expose pool stats if not already public | Small (20-30 lines) |
| Tool handlers (scattered) | Trim verbose success messages for error-only reporting | Medium (audit 41+ tools) |

## Suggested Build Order

Based on dependency analysis:

1. **Progressive descriptions + error-only reporting** -- Lowest risk. Modify existing ToolDef and tool handlers. No new packages. Tests: update golden files for profile contracts.

2. **Smart error middleware** -- Depends only on existing error kinds and tool registry. Self-contained new file. Tests: unit test middleware with mock tool results.

3. **Lazy init middleware** -- Depends on workspace activation callback (already exists). Self-contained. Tests: unit test with mock workspace state.

4. **Health skill** -- Needs KernelHealth interface definition and pool stats exposure. New skill package. Tests: unit test skill with mock health provider.

5. **Hook specs and templates** -- New package, no runtime dependencies. Pure template generation. Tests: render templates, verify output.

6. **Setup CLI** -- Depends on all above components being available. Orchestrates langregistry, installer, hooks. Tests: integration test with temp directories.

**Rationale:** Items 1-3 are modifications/middleware with minimal blast radius. Items 4-5 are new packages with clear interfaces. Item 6 ties everything together last, reducing integration risk.

## Scalability Considerations

| Concern | At 1 client | At 5 clients | At 20 clients |
|---------|-------------|--------------|---------------|
| Setup templates | Trivial | 5 client specs | Registry pattern, embed all |
| Hook generation | Single file write | Multiple format outputs | Template engine, no perf concern |
| Health queries | Single pool.Stats() call | Same (pool is shared) | Same |
| Progressive descriptions | 41 tools x 2 tiers | Same | Same (descriptions are static strings) |
| Error enrichment | Map lookup per error | Same | Same |

No scalability concerns for v1.7 -- all features are bounded by the fixed tool count and operate per-request.

## Sources

- Existing codebase: `internal/daemon/daemon.go` (bootstrap wiring, middleware installation)
- Existing codebase: `internal/mcp/middleware.go` (receiving middleware pattern)
- Existing codebase: `internal/mcp/registry.go` (ToolDef structure)
- Existing codebase: `internal/skill/skill.go` (Skill/ToolProvider interfaces)
- Existing codebase: `internal/cli/root.go` (cobra command structure)
- Existing codebase: `internal/errors/kinds.go` (7-kind error taxonomy)
- Existing codebase: `internal/kernel/workspace.go` (language detection, workspace runtime)
- Existing codebase: `internal/langregistry/installer.go` (three-tier LS resolution)
- Existing codebase: `internal/skill/repomap/` (setter pattern for skill wiring)
- Project: `.planning/PROJECT.md` (v1.7 requirements)
- Confidence: HIGH -- all integration points verified against existing source code
