# Phase 66: Agent Guardrails - Pattern Map

**Mapped:** 2026-05-09
**Files analyzed:** 22 (new + modified)
**Analogs found:** 22 / 22

## File Classification

### New Files

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `internal/guardrails/doc.go` | package-doc | n/a | `internal/mcp/doc.go` (if exists) / package header in `internal/fuzzy/match.go` | role-match |
| `internal/guardrails/receipt.go` | model | data-record | `internal/semantic/integ/source.go` closed-enum + per-class structs | role+flow match |
| `internal/guardrails/receipt_id.go` | utility | transform | `crypto/rand` consumers in repo + `internal/errors/kinds.go` sentinel discipline | role-match |
| `internal/guardrails/store.go` | service | CRUD (in-mem) | `internal/kernel/lspool/` worker-cache discipline; `sync.Map` + janitor pattern | role+flow match |
| `internal/guardrails/validate.go` | service | request-response | `internal/mcp/middleware.go` `classifyOutcome` switch (lines 241-258) + `internal/errors/kinds.go` sentinel pattern | role+flow match |
| `internal/guardrails/enforcement.go` | service | transform | `internal/profile/profile.go` resolution functions; layered config in `internal/config/` | role-match |
| `internal/guardrails/rules/g00*.go` (5 files) | service (predicate) | request-response | `internal/mcp/middleware.go` `classifyOutcome`; `internal/kernel/edit/tools.go` `ClassifyEditError` (lines 52-82) | role+flow match |
| `internal/guardrails/catalogs/{go,ts,js,python}.yaml` | config | static-data | `internal/profile/profiles/*.yaml` + `internal/langregistry/` embedded YAML | role+flow match |
| `internal/guardrails/catalogs/embed.go` | utility | file-I/O | `internal/profile/embed.go` (`//go:embed profiles/*.yaml`) | exact |
| `internal/guardrails/visibility.go` | service | transform | `internal/semantic/extract/fact.go` `Visibility` field (line 98) — extends via SemanticLookup | role-match |
| `internal/guardrails/deps.go` | model (interface) | n/a | `internal/semantic/integ/lookup.go` `SemanticLookup` interface (lines 95-104) | exact |
| `internal/guardrails/issue_sink.go` | utility | event-driven | `internal/mcp/middleware.go` `editOutcomeSink` (lines 50-97) — atomic.Pointer[Fn] | exact |
| `internal/mcp/guardrail_middleware.go` | middleware | request-response | `internal/mcp/lazy_init.go` (full file — 113 lines) + `internal/mcp/suggest.go` (lines 253-256) | exact |
| `internal/skill/guardrails/skill.go` | skill-adapter | n/a | `internal/kernel/help/skill_adapter.go` (33 lines) | exact |
| `GUARDRAILS.md` | doc | n/a | repo-root markdown convention; embedded via embed.FS | role-match |
| `DoD.md` | doc | n/a | same | role-match |
| `test/harness/context_truncation_test.go` | test (integration) | event-driven | existing `test/harness/Runner` (verify path at plan-time) | role-match |

### Modified Files

| Modified File | Role | Data Flow | Closest Analog (in-file) | Notes |
|---------------|------|-----------|--------------------------|-------|
| `internal/daemon/daemon.go` | bootstrap | request-response | step 14b at line 706 (`InstallSuggestionMiddleware`) | insert step 14b.5 between line 706 and 729 |
| `internal/daemon/imports.go` | bootstrap | n/a | existing blank-imports of `internal/skill/{memory,repomap,semantic,workflow}` | add `_ "internal/skill/guardrails"` |
| `internal/mcp/middleware.go` | middleware | request-response | `outcomeEnum` (lines 159-178) + `editOutcomeSink` (lines 50-97) | extend outcomeEnum + add receipt sinks |
| `internal/semantic/config.go` | config | static-data | `GuardrailsConfig` struct (lines 269-279) | extend with Rules/Tools maps + G004/G005 substructs |
| `internal/config/defaults.go` | config | static-data | existing `semantic_index.guardrails.*` keys (lines 149-156) | add per-rule + per-tool defaults |
| `internal/profile/profiles/ci-bot.yaml` | config | static-data | existing structure (full file, 56 lines) | append `guardrails: { enforcement: enforce }` |
| `internal/semantic/integ/lookup.go` | model (interface) | n/a | `SemanticLookup` interface (lines 95-104) | add `Visibility`, `IsEntrypointReachable` methods |
| `internal/kernel/help/tools.go` | controller | request-response | `registerFindReferences` shape (`internal/kernel/symbols/tools.go` line 366) | extend args with `topic` field; add TopicRegistry dispatch |
| `internal/kernel/help/skill_adapter.go` | skill-adapter | n/a | own current file | no change needed (tool name unchanged) |
| `internal/kernel/symbols/tools.go` | controller | request-response | `registerFindReferences` (line 366-391); `registerReplaceBody` (`edit/tools.go` line 299-392) success-path closure | add `IssueReceiptOnSuccess` after `formatLocations` |
| `internal/kernel/symbols/blast.go` (or registration site) | controller | request-response | same | add `IssueReceiptOnSuccess` for `analyze_blast_radius` |
| `internal/kernel/diag/tools.go` | controller | request-response | same | add `IssueReceiptOnSuccess` for diagnostics tools |
| `internal/kernel/edit/verify.go` | controller | request-response | post-edit hook in `registerReplaceBody` `appendVerifyInfoWithStatus` (line 374) | issue `diagnostics_clean` when ErrorCount==0 |
| `internal/kernel/edit/tools.go` | controller | request-response | `registerReplaceBody` (line 299-392) | add `Receipts []ReceiptID` to ReplaceBodyArgs / RenameSymbolArgs / SafeDeleteArgs |
| `internal/kernel/fileops/fuzzy_edit.go` (+ `replace.go` + `write.go` for delete_file) | controller | request-response | same | add `Receipts` field to args |
| `internal/skill/repomap/skill.go` | controller | request-response | success path of `get_repo_map` / `get_context` handlers | add `IssueReceiptOnSuccess` |
| `internal/errors/kinds.go` | model | static-data | `Kind` constants (lines 13-22) + `ErrNotFound` sentinels (lines 28-35) | add `Kind = "guardrail_violation"` + sentinel |
| `internal/obs/metrics.go` | service | event-driven | `EditOutcomeInc` (lines 550-568) drop-unknown discipline | add `ReceiptIssuedInc`, `ReceiptExpiredInc`, `ReceiptLookupInc` |
| `internal/mcp/middleware_test.go` | test | n/a | existing LIFO order assertion | extend to 5-step order |

---

## Pattern Assignments

### `internal/mcp/guardrail_middleware.go` (middleware, request-response)

**Analog:** `internal/mcp/lazy_init.go` (entire file, especially lines 71-112) + `internal/mcp/suggest.go` lines 253-256.

**Imports pattern** (from `lazy_init.go:1-10`):
```go
package mcp

import (
    "context"
    "encoding/json"
    "log/slog"
    "sync"

    mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)
```

**Middleware shell pattern** (`lazy_init.go:72-104`):
```go
func (m *LazyInitMiddleware) Middleware() mcpsdk.Middleware {
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            if method != "tools/call" {
                return next(ctx, method, req)
            }
            // ... evaluate ...
            return next(ctx, method, req)
        }
    }
}
```

**Argument extraction pattern** (`lazy_init.go:51-69`) — mirror for receipt extraction:
```go
ctr, ok := req.(*mcpsdk.CallToolRequest)
if !ok || ctr == nil || ctr.Params == nil {
    return m.defaultRoot
}
if len(ctr.Params.Arguments) > 0 {
    var args map[string]any
    if err := json.Unmarshal(ctr.Params.Arguments, &args); err == nil {
        if rp, ok := args["repo_path"]; ok { ... }
    }
}
```

**Install function pattern** (`suggest.go:253-256`, `lazy_init.go:106-112`):
```go
// InstallLazyInitMiddleware wires the lazy init middleware onto the MCP SDK server.
// MUST be called LAST (after InstallSuggestionMiddleware) so it runs FIRST in the
// LIFO middleware chain -- before TelemetryMiddleware's deadline (Pitfall 3).
func InstallLazyInitMiddleware(server *mcpsdk.Server, activateFn func(...) error, ..., logger *slog.Logger) {
    m := NewLazyInitMiddleware(activateFn, isActiveFn, defaultRoot, logger)
    server.AddReceivingMiddleware(m.Middleware())
}
```

For Phase 66: `InstallGuardrailMiddleware` MUST be installed at step 14b.5 (after Suggestion, before LazyInit). Daemon.go inserts the call between current line 706 and line 729 (verified install sites).

**Block-path response shape** (mirror `lazy_init.go:91-98`):
```go
return &mcpsdk.CallToolResult{
    Content: []mcpsdk.Content{
        &mcpsdk.TextContent{Text: "Guardrail violation: " + msg + ". See get_tool_help{topic:\"workflow:rename\"}."},
    },
    IsError: true,
}, nil
```
Or, preferred per RESEARCH §"Middleware shell": return typed `serr.NewGuardrailViolation(...)` so `TelemetryMiddleware.classifyOutcome` reads `Kind` and maps to `outcome=guardrail_blocked`.

---

### `internal/guardrails/issue_sink.go` (utility, event-driven)

**Analog:** `internal/mcp/middleware.go` lines 50-97 (`editOutcomeSink` + `RecordEditOutcome`).

**Sink declaration + setter + caller pattern** (verbatim from `middleware.go:50-97`):
```go
// editOutcomeSink is the package-level recorder wired by InstallMiddleware.
var editOutcomeSink atomic.Pointer[func(ctx context.Context, toolName, outcome, strategy string)]

func setEditOutcomeSink(fn func(ctx context.Context, toolName, outcome, strategy string)) {
    editOutcomeSink.Store(&fn)
}

func RecordEditOutcome(ctx context.Context, toolName, outcome, strategy string) {
    p := editOutcomeSink.Load()
    if p == nil || *p == nil {
        return
    }
    (*p)(ctx, toolName, outcome, strategy)
}
```

**Phase 66 adaptation:** rename to `receiptIssueSink`, signature `func(ctx, class ReceiptClass, scope ReceiptScope, tool string) (ReceiptID, error)`. Sink is wired in `daemon.go` post-init alongside the existing `setEditOutcomeSink` block (currently at `middleware.go:138-143`):
```go
setEditOutcomeSink(func(_ context.Context, toolName, outcome, strategy string) {
    m.EditOutcomeInc(toolName, outcome, strategy)
})
// NEW: parallel block for receipt issuance
guardrails.SetReceiptIssueSink(store.Issue)
```

---

### `internal/guardrails/receipt.go` (model, data-record)

**Analog:** `internal/errors/kinds.go` lines 8-35 (closed enum + sentinel discipline) + `internal/semantic/integ/source.go` (closed-enum + per-class shape — mentioned in RESEARCH).

**Closed-enum pattern** (from `kinds.go:8-22`):
```go
type Kind string

const (
    NotFound         Kind = "not_found"
    InvalidArgs      Kind = "invalid_args"
    NoWorkspace      Kind = "no_workspace"
    Unsupported      Kind = "unsupported"
    Internal         Kind = "internal"
    CircuitOpen      Kind = "circuit_open"
    Timeout          Kind = "timeout"
    PermissionDenied Kind = "permission_denied"
)
```

**Sentinel error discipline** (`kinds.go:24-35`):
```go
var (
    ErrNotFound         = &Error{Kind: NotFound}
    ErrInvalidArgs      = &Error{Kind: InvalidArgs}
    // ...
)
```

For Phase 66: `ReceiptClass` mirrors `Kind` (closed string-typed enum); `ErrWrongWorkspace`, `ErrGraphVersionMismatch`, etc. (per D-05) mirror the sentinel block.

---

### `internal/guardrails/store.go` (service, CRUD in-mem)

**Analog:** Pattern derived from RESEARCH §"Receipt store with janitor" (D-06) + `lazy_init.go:38-47` (sync.Map + mutex pattern).

**sync map + mutex pattern** (`lazy_init.go:38-47`):
```go
func (m *LazyInitMiddleware) getOnce(path string) *sync.Once {
    m.mu.Lock()
    defer m.mu.Unlock()
    if once, ok := m.initOnce[path]; ok {
        return once
    }
    once := &sync.Once{}
    m.initOnce[path] = once
    return once
}
```

For Phase 66: `Store.Get/Put` follows the same `sync.Map` + RWMutex shape; janitor goroutine ticks every 30s (per D-06).

---

### `internal/guardrails/validate.go` (service, request-response)

**Analog:** `internal/mcp/middleware.go` lines 241-258 (`classifyOutcome`) + `internal/kernel/edit/tools.go` lines 52-82 (`ClassifyEditError`).

**Switch + closed-enum + sentinel pattern** (`middleware.go:241-258`):
```go
func classifyOutcome(result mcpsdk.Result, err error) string {
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return outcomeTimeout
        }
        if errors.Is(err, serr.ErrCircuitOpen) {
            return outcomeCircuitOpen
        }
        return outcomeInternal
    }
    if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil && ctr.IsError {
        return outcomeInternal
    }
    return outcomeSuccess
}
```

**ClassifyEditError ordered-sentinel pattern** (`edit/tools.go:52-82`):
```go
func ClassifyEditError(err error) string {
    if err == nil { return "success" }
    if errors.Is(err, fuzzy.ErrAmbiguous) { return "ambiguous_match" }
    if errors.Is(err, fuzzy.ErrNoMatch)   { return "no_match" }
    return "internal"
}
```

For Phase 66: `ValidateReceiptForOperation` returns typed sentinels in priority order (workspace > graph_version > expired > stale > class > scope) — same ordered-sentinel idiom.

---

### `internal/skill/guardrails/skill.go` (skill-adapter)

**Analog:** `internal/kernel/help/skill_adapter.go` (entire file, 33 lines).

**Verbatim shape to copy** (`help/skill_adapter.go:1-33`):
```go
package help

import (
    "github.com/agenthands/helix/internal/mcp"
    "github.com/agenthands/helix/internal/skill"
)

type HelpSkill struct{}

func init() {
    skill.Register(&HelpSkill{})
}

func (s *HelpSkill) Name() string { return "help" }
func (s *HelpSkill) Description() string { return "Tool documentation (get_tool_help)" }
func (s *HelpSkill) Init(deps skill.SkillDeps) error { return nil }

func (s *HelpSkill) Tools() []*mcp.ToolDef {
    return []*mcp.ToolDef{
        {Name: "get_tool_help", Description: "..."},
    }
}
```

For Phase 66: Guardrails skill primarily exists to register the package via blank-import; no direct MCP tool surface (the guardrail middleware is wired separately in daemon.go). The skill MAY register zero tools (`Tools() []*mcp.ToolDef { return nil }`).

---

### `internal/guardrails/deps.go` (model — interface)

**Analog:** `internal/semantic/integ/lookup.go` lines 95-104 (`SemanticLookup` interface).

**Verbatim interface shape** (lookup.go:95-104):
```go
type SemanticLookup interface {
    Available() bool
    SymbolID(ctx context.Context, ws workspace.WorkspaceKey, path string, line, col uint32) (SymbolID, error)
    RankFiles(ctx context.Context, ws workspace.WorkspaceKey) ([]RankedFile, error)
    RankFromSeeds(ctx context.Context, ws workspace.WorkspaceKey, seeds []string) ([]RankedFile, error)
    ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID, depth int) ([]Impact, error)
    ValidateCriticalEdges(ctx context.Context, ws workspace.WorkspaceKey, edges []Edge) ([]ValidatedEdge, error)
    LocateSymbol(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (path string, line, col uint32, ok bool, err error)
    Status(ctx context.Context, ws workspace.WorkspaceKey) (SemanticStatus, error)
}
```

For Phase 66: `MiddlewareDeps` (per RESEARCH Pattern 2) takes the same shape — small read-only interface with method signatures keyed on `(ctx, ws)`. Add `Visibility(ctx, ws, sym) (Visibility, error)` and `IsEntrypointReachable(ctx, ws, sym) (bool, error)` to `SemanticLookup` itself per OI-02/OI-03.

---

### `internal/kernel/symbols/tools.go` modifications (read-tool receipt issuance)

**Analog:** existing `registerFindReferences` (`tools.go:366-391`) — success-path closure pattern.

**Existing handler success path to extend** (lines 384-389):
```go
locs, err := FindReferences(ctx, lease, pathToURI(rt.Key().RepoRoot, args.Path), lspLine, lspCol, args.IncludeDecl)
if err != nil {
    return errorResult(err.Error()), nil, nil
}
return textResult(formatLocations(locs)), nil, nil
```

**Phase 66 modification (added BEFORE the final `return`):**
```go
locs, err := FindReferences(ctx, lease, ..., args.IncludeDecl)
if err != nil {
    return errorResult(err.Error()), nil, nil
}
// Phase 66 D-01: issue receipt on success path ONLY (Pitfall 4 — never in defer).
guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassReferencesChecked,
    guardrails.ReferencesCheckedScope{
        SymbolID:     symID,             // resolved from ctx/lease before the call
        RefCount:     len(locs),
        FilePath:     relPath,
        IncludeTests: args.IncludeDecl,
    }, "find_references")
return textResult(formatLocations(locs)), nil, nil
```

Apply identical shape to `registerAnalyzeBlastRadius`, `get_diagnostics`, `verify_edit`, `run_diagnostics`, `get_context`, `get_repo_map`. The closure in `internal/skill/repomap/skill.go` follows the same pattern at the success leaf of each handler.

---

### `internal/kernel/edit/tools.go` modifications (destructive-tool args extension)

**Analog:** existing `ReplaceBodyArgs` (`tools.go:86-92`) and `registerReplaceBody` (`tools.go:299-392`).

**Existing args struct pattern** (`tools.go:86-114`):
```go
type ReplaceBodyArgs struct {
    Path       string `json:"path" jsonschema:"File path"`
    SymbolName string `json:"symbol_name" jsonschema:"Name of the symbol whose body to replace"`
    NewBody    string `json:"new_body" jsonschema:"New body content to replace with"`
    SearchBody string `json:"search_body,omitempty" jsonschema:"Optional: fuzzy-match this text within the symbol body before replacing. When absent, replaces the entire body."`
}

type RenameSymbolArgs struct {
    Path    string `json:"path" jsonschema:"File path where symbol is defined"`
    Line    int    `json:"line" jsonschema:"Line number of symbol (1-indexed)"`
    Col     int    `json:"column" jsonschema:"Column number of symbol (1-indexed)"`
    NewName string `json:"new_name" jsonschema:"New name for the symbol"`
}
```

**Phase 66 modification — uniform `Receipts` field per D-08:**
```go
type ReplaceBodyArgs struct {
    Path       string                  `json:"path" ...`
    SymbolName string                  `json:"symbol_name" ...`
    NewBody    string                  `json:"new_body" ...`
    SearchBody string                  `json:"search_body,omitempty" ...`
    Receipts   []guardrails.ReceiptID  `json:"receipts,omitempty" jsonschema:"Receipt IDs from prior find_references / analyze_blast_radius calls (Phase 66 GUARD-03)"`
}
```

**Existing defer-emission pattern** (verbatim, `tools.go:303-309`) — already proven; Phase 66 leaves untouched:
```go
outcome, strategy := "success", "none"
defer func() { mcp.RecordEditOutcome(ctx, "replace_symbol_body", outcome, strategy) }()
```

The receipts themselves are extracted by the middleware (NOT by the tool handler) — the tool handler does not need to read `args.Receipts` directly; the middleware does the validation before the handler runs.

---

### `internal/semantic/config.go` `GuardrailsConfig` extension (config)

**Analog:** existing struct (lines 269-279).

**Existing struct to extend:**
```go
// GuardrailsConfig holds GuardrailMiddleware settings (P66).
type GuardrailsConfig struct {
    Enabled                       bool   `koanf:"enabled"`
    Enforcement                   string `koanf:"enforcement"`                        // "warn" | "block"
    RequireImpactForPublicAPIEdit bool   `koanf:"require_impact_for_public_api_edit"`
    RequireReferencesBeforeRename bool   `koanf:"require_references_before_rename"`
    RequireReferencesBeforeDelete bool   `koanf:"require_references_before_delete"`
    RequireVerifyAfterEdit        bool   `koanf:"require_verify_after_edit"`
    StaleGraphPolicy              string `koanf:"stale_graph_policy"`                 // "warn" | "block"
}
```

**Phase 66 extension (per D-22):** add `Rules map[string]RuleConfig`, `Tools map[string]ToolConfig`, `ReceiptTTL time.Duration`, `G004 G004Config`, `G005 G005Config`. Per RESEARCH "State of the Art": flat booleans (`RequireImpactForPublicAPIEdit` etc.) are **subsumed** by per-rule entries — pre-production, removable.

---

### `internal/profile/profiles/ci-bot.yaml` modification (per OI-01)

**Analog:** the file itself (full 56 lines shown above).

**Existing structure** — append-only insertion point:
```yaml
# Insert after line 27 (`tools: []`) and before `exclude_tools:`:
guardrails:
  enforcement: enforce
```

Other profile YAMLs (`claude-code.yaml`, `codex.yaml`, `ide-assistant.yaml`, `full.yaml`) gain `guardrails: { enforcement: warn }` per D-11. Profile config struct extension lives in `internal/profile/profile.go` (not yet read — verify at plan time).

---

### `internal/errors/kinds.go` extension

**Analog:** lines 8-35 of own file.

**Phase 66 addition (single-line):**
```go
const (
    NotFound         Kind = "not_found"
    // ... existing ...
    GuardrailViolation Kind = "guardrail_violation"  // NEW (Phase 66)
)

var (
    ErrNotFound = &Error{Kind: NotFound}
    // ... existing ...
    ErrGuardrailViolation = &Error{Kind: GuardrailViolation}  // NEW
)
```

---

### `internal/obs/metrics.go` extension (telemetry counters)

**Analog:** `EditOutcomeInc` (lines 550-568) — drop-unknown closed-enum discipline.

**Verbatim pattern to replicate** (`metrics.go:556-568`):
```go
func (m *Metrics) EditOutcomeInc(toolName, outcome, strategy string) {
    switch outcome {
    case "success", "no_match", "ambiguous_match", "validation_failed", "ls_error", "internal":
    default:
        return
    }
    switch strategy {
    case "exact", "whitespace_normalized", "indentation_flexible", "none":
    default:
        return
    }
    m.EditOutcome.WithLabelValues(toolName, outcome, strategy).Inc()
}
```

**Phase 66 new counters (mirror shape):**
```go
func (m *Metrics) ReceiptIssuedInc(class string) {
    switch class {
    case "references_checked", "impact_checked", "context_gathered", "structural_overview", "diagnostics_clean":
    default: return
    }
    m.ReceiptIssued.WithLabelValues(class).Inc()
}

func (m *Metrics) ReceiptExpiredInc(reason string) {
    switch reason {
    case "ttl", "graph_drift", "lru_evicted":
    default: return
    }
    m.ReceiptExpired.WithLabelValues(reason).Inc()
}

func (m *Metrics) ReceiptLookupInc(outcome string) {
    switch outcome {
    case "hit", "miss", "expired", "scope_mismatch", "graph_drift",
         "workspace_mismatch", "freshness_rejected", "wrong_class":
    default: return
    }
    m.ReceiptLookup.WithLabelValues(outcome).Inc()
}
```

---

### `internal/mcp/middleware.go` outcomeEnum extension

**Analog:** lines 159-178 of own file.

**Existing closed enum:**
```go
const (
    outcomeSuccess     = "success"
    outcomeInvalidArgs = "invalid_args"
    outcomeNotFound    = "not_found"
    outcomeCircuitOpen = "circuit_open"
    outcomeLSCrash     = "ls_crash"
    outcomeTimeout     = "timeout"
    outcomeInternal    = "internal"
)

var outcomeEnum = []string{
    outcomeSuccess, outcomeInvalidArgs, outcomeNotFound,
    outcomeCircuitOpen, outcomeLSCrash, outcomeTimeout, outcomeInternal,
}
```

**Phase 66 addition:**
```go
const (
    // ... existing ...
    outcomeGuardrailWarned  = "guardrail_warned"   // NEW
    outcomeGuardrailBlocked = "guardrail_blocked"  // NEW
)
var outcomeEnum = []string{ ..., outcomeGuardrailWarned, outcomeGuardrailBlocked }
```

**`classifyOutcome` extension** (lines 241-258): add a branch BEFORE the generic `outcomeInternal` fallback:
```go
if errors.Is(err, serr.ErrGuardrailViolation) {
    var ge *serr.Error
    if errors.As(err, &ge) && ge.Kind == serr.GuardrailViolation {
        // Distinguish warn vs block via a typed-error field, OR
        // by inspecting decision.Action recorded on the error.
        return outcomeGuardrailBlocked
    }
}
```

---

### `internal/daemon/daemon.go` step 14b.5 insertion

**Analog:** existing step 14b (line 706) and step 14c (line 729).

**Existing install ordering (verified):**
```go
// step 14: Telemetry + ProfileFilter
helixMCP.InstallMiddleware(mcpServer.SDK(), observability, profileStore, getSessionFn, budgetFn, mcpServer.Registry(), logger)

// 14b. Suggestion middleware
suggestionSchemaMap := helixMCP.BuildToolSchemaMap(mcpServer.CollectToolSchemas())
helixMCP.InstallSuggestionMiddleware(mcpServer.SDK(), suggestionSchemaMap, logger)

// 14c. lazy init MUST be installed LAST
helixMCP.InstallLazyInitMiddleware(mcpServer.SDK(), lazyActivateFn, isActiveFn, "", logger)
```

**Phase 66 insertion (between 14b and 14c):**
```go
// 14b.5 (Phase 66 GUARD-01): Guardrail middleware. Installed AFTER Suggestion
// and BEFORE LazyInit so LIFO execution = LazyInit -> Guardrail -> Suggestion ->
// ProfileFilter -> Telemetry -> handler. LazyInit-last invariant preserved.
guardrailDeps := guardrails.NewProductionDeps(sBndl.integLookupAccessor(),
    profileStore, cfg.SemanticIndex.Guardrails, store, logger)
helixMCP.InstallGuardrailMiddleware(mcpServer.SDK(), guardrailDeps, logger)
```

---

### `internal/daemon/imports.go` extension

**Analog:** lines 10-13 of own file.

**Existing blank-imports:**
```go
import (
    _ "github.com/agenthands/helix/internal/skill/memory"
    _ "github.com/agenthands/helix/internal/skill/repomap"
    _ "github.com/agenthands/helix/internal/skill/semantic"
    _ "github.com/agenthands/helix/internal/skill/workflow"
)
```

**Phase 66 addition:**
```go
_ "github.com/agenthands/helix/internal/skill/guardrails"
```

---

### `internal/guardrails/catalogs/embed.go` (utility, file-I/O)

**Analog:** `internal/profile/embed.go` (full file).

**Verbatim shape:**
```go
package profile

import "embed"

//go:embed profiles/*.yaml
var embeddedProfiles embed.FS

//go:embed modes/*.yaml
var embeddedModes embed.FS
```

**Phase 66 application:**
```go
package catalogs

import "embed"

//go:embed *.yaml
var EmbeddedCatalogs embed.FS
```

Same pattern for `GUARDRAILS.md`/`DoD.md` embedding into the help topic registry (per D-23/D-24).

---

## Shared Patterns

### Pattern A: Atomic-Pointer Sink Closure (cross-package wiring)
**Source:** `internal/mcp/middleware.go:50-97` (`editOutcomeSink` + `RecordEditOutcome`)
**Apply to:** `internal/guardrails/issue_sink.go` (receipt issuance from read tools).
**Why:** Avoids a build-time dependency from `internal/kernel/symbols` on `internal/guardrails`; sink wired once at daemon init via `setReceiptIssueSink(...)`; no-op until wired (test-friendly).

```go
var editOutcomeSink atomic.Pointer[func(ctx context.Context, toolName, outcome, strategy string)]

func setEditOutcomeSink(fn func(ctx context.Context, toolName, outcome, strategy string)) {
    editOutcomeSink.Store(&fn)
}

func RecordEditOutcome(ctx context.Context, toolName, outcome, strategy string) {
    p := editOutcomeSink.Load()
    if p == nil || *p == nil {
        return
    }
    (*p)(ctx, toolName, outcome, strategy)
}
```

### Pattern B: Drop-Unknown Closed-Enum Metric Helper
**Source:** `internal/obs/metrics.go:556-568` (`EditOutcomeInc`)
**Apply to:** All new `helix_receipt_*` counters and the extended `outcomeEnum` in TelemetryMiddleware.
**Why:** Bounded-label discipline is enforced at the metric layer; agent-supplied or unknown values are silently dropped, never leaking into Prometheus cardinality.

```go
func (m *Metrics) EditOutcomeInc(toolName, outcome, strategy string) {
    switch outcome {
    case "success", "no_match", "ambiguous_match", "validation_failed", "ls_error", "internal":
    default:
        return
    }
    // ... bounded-label increment ...
}
```

### Pattern C: Closed-Enum + Sentinel Error
**Source:** `internal/errors/kinds.go:8-35`
**Apply to:** `internal/guardrails/validate.go` (`ErrWrongWorkspace`, `ErrGraphVersionMismatch`, ...) and the new `Kind = "guardrail_violation"` sentinel.

### Pattern D: Skill Setter Post-Init Wiring
**Source:** `internal/skill/repomap/skill.go:131-182` (`SetEnrichFn` / `SetSemanticLookup` / `SetConfigGate`)
**Apply to:** `internal/guardrails` `MiddlewareDeps` setter — daemon post-init injects production adapter without making `internal/mcp` import the guardrails package.

```go
// Existing pattern in repomap:
func (s *RepoMapSkill) SetSemanticLookup(lookup integ.SemanticLookup) { ... }
// Daemon wires it post-init:
rs.SetSemanticLookup(sBndl.integLookupAccessor())
```

### Pattern E: Per-Tool Defer-Emission for Outcome Recording
**Source:** `internal/kernel/edit/tools.go:303-309` (`registerReplaceBody`)
**Apply to:** `internal/kernel/edit/verify.go` for `diagnostics_clean` receipt issuance (post-edit ErrorCount==0 path).

```go
outcome, strategy := "success", "none"
defer func() { mcp.RecordEditOutcome(ctx, "replace_symbol_body", outcome, strategy) }()
```

### Pattern F: Tool Handler Success-Path Closure
**Source:** `internal/kernel/symbols/tools.go:366-391` (`registerFindReferences`)
**Apply to:** All 7 read/diagnostics issuers — `IssueReceiptOnSuccess` is called ONLY on the explicit success path (per Pitfall 4 — never in `defer`).

### Pattern G: embed.FS for Single-Binary Resource Bundling
**Source:** `internal/profile/embed.go`
**Apply to:** G-005 catalog YAMLs in `internal/guardrails/catalogs/embed.go`; `GUARDRAILS.md`/`DoD.md` for `get_tool_help` topic content.

### Pattern H: Closed-Type-Union via Marker Interface
**Source:** `internal/semantic/integ/source.go` (closed-enum + per-class shape — referenced in RESEARCH §"Code Examples")
**Apply to:** `ReceiptScope` interface with `isReceiptScope()` marker method (per D-03 typed Go union):
```go
type ReceiptScope interface{ isReceiptScope() }
type ReferencesCheckedScope struct { /* fields */ }
func (ReferencesCheckedScope) isReceiptScope() {}
```

### Pattern I: Skill Adapter (kernel-resident tool exposed as skill)
**Source:** `internal/kernel/help/skill_adapter.go` (full file)
**Apply to:** `internal/skill/guardrails/skill.go` (no MCP tools registered — purely for blank-import side-effect).

---

## No Analog Found

| File | Role | Data Flow | Reason / Fallback |
|------|------|-----------|-------------------|
| `test/harness/context_truncation_test.go` | integration test | event-driven (forwarder→daemon) | `test/harness/Runner` exists per RESEARCH but its E2E shape for `tools/call` driven from forwarder needs verification at plan-time. Fall back to RESEARCH §"Pitfall 8" guidance: drive through stdio forwarder + gRPC daemon, NOT in-process. |
| `internal/guardrails/rules/g004_large_fuzzy_edit.go` | service (predicate) | request-response | LOC-counting predicate is pure-Go arithmetic; no semantic dependency. Closest analog is the line-count math in `internal/kernel/fileops/fuzzy_edit.go` — verify at plan time. |
| `GUARDRAILS.md` / `DoD.md` content shape | doc | n/a | No prior repo-root operator/agent-doc pair exists with the same dual-audience split. Use RESEARCH §"D-23" content outline as authoritative; mirror Markdown style of `CLAUDE.md`. |

---

## Metadata

**Analog search scope:** `internal/mcp/`, `internal/kernel/{symbols,edit,fileops,diag,help}/`, `internal/skill/{memory,repomap,workflow,semantic}/`, `internal/profile/`, `internal/semantic/{config,integ,extract}/`, `internal/errors/`, `internal/obs/`, `internal/daemon/`, `internal/config/`.
**Files scanned:** ~28 (plus directory listings of 12 packages).
**Pattern extraction date:** 2026-05-09
**Phase:** 66-Agent-Guardrails
