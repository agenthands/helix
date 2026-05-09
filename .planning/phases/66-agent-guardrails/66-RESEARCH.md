# Phase 66: Agent Guardrails (G-001..G-005, warn-default) - Research

**Researched:** 2026-05-09
**Domain:** MCP middleware design, server-side capability receipts, agent-context-survival policy enforcement, multi-language symbol classification
**Confidence:** HIGH (architecture, code anchors, locked decisions); MEDIUM (visibility/entry-point predicates, OI-02/OI-03); LOW (G-005 import-pattern catalogs for Java/Rust/C# — domain-knowledge based, not verified against ecosystem)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

D-01..D-24 from `66-CONTEXT.md` are LOCKED and MUST NOT be re-evaluated. Summary (verbatim semantics):

- **D-01** Receipt-issuing tools: `find_references`, `analyze_blast_radius`, `get_context` / `get_semantic_context`, `get_repo_map` (read-side); `get_diagnostics`, `verify_edit`, `run_diagnostics` (post-edit). Five receipt classes: `references_checked`, `impact_checked`, `context_gathered`, `structural_overview`, `diagnostics_clean`.
- **D-02** Receipt common fields: `id, class, schema_version, workspace_key, snapshot_id, graph_version, freshness, score_status, cluster_status, pending_lsp_files, lsp_coverage, issued_at, expires_at, issuing_tool, issuing_call_id, trace_id, scope`.
- **D-03** Scope shape: typed Go union (closed `ReceiptClass` enum + per-class scope structs). NOT `map[string]any`.
- **D-04** Default TTL: 5 minutes. Configurable via `semantic_index.guardrails.receipt_ttl`.
- **D-05** `ValidateReceiptForOperation` predicate with typed sentinel errors (`ErrWrongWorkspace`, `ErrGraphVersionMismatch`, `ErrReceiptExpired`, `ErrReceiptTooStale`, `ErrWrongReceiptClass`, `ErrReceiptScopeMismatch`).
- **D-06** Storage: in-memory `sync.Map` per workspace, RWMutex, 30s janitor + opportunistic-on-lookup expiry; max 10k receipts/workspace with LRU eviction; receipts lost on daemon restart (acceptable since TTL=5min and graph_version invalidates on commit).
- **D-07** Receipt ID: `rcpt_<UUIDv7-base32-26char>`. `crypto/rand` + UUIDv7 prefix; opaque to agent; time-ordered.
- **D-08** Uniform `receipts: []ReceiptID` field on every destructive tool: `rename_symbol`, `safe_delete_symbol`, `replace_symbol_body`, `fuzzy_edit`, `replace_in_file`, `delete_file`.
- **D-09** STRICT scope-match per class (no class-only loophole). Symbol-anchored receipts require `scope.SymbolID == target.SymbolID`; file-set receipts require `FileSetContainsAll`; structural receipts require `PathIsWithin`; diagnostics-clean requires `FileSetContainsAll(scope.FileSet, target.SecuritySensitiveTouchedFiles) AND scope.ErrorCount == 0`.
- **D-10** Schema-day-one rollout — no opt-in flag; missing/null `receipts` normalized to `[]`.
- **D-11** Profile-defaulted enforcement: `warn` for `claude-code`/`codex`/`ide-assistant`/`edit`; `enforce` for `review`/`ci-bot` (plus proposed new `security` profile = `enforce` — open per OI-01).
- **D-12** Response envelopes: warn = `ok:true` + `warnings[]`; enforce = `ok:false` + typed `error{kind:"guardrail_violation", rule, message, required_receipts, suggested_tools, see_also}`.
- **D-13** G-001 trigger: `fuzzy_edit`/`replace_in_file` when `find` matches identifier regex AND equals/substring of a known tree-sitter outline symbol. `rename_symbol` exempt.
- **D-14** G-002 triggers: `delete_file` (always), `safe_delete_symbol` (always), `replace_symbol_body` (when refcount/caller>0 OR public OR entry-point), `fuzzy_edit`/`replace_in_file` when removing decl/body or large block.
- **D-15** G-003 triggers: `IsPublicLike()` OR entry-point-reachable OR interface contract OR external-package refs OR signature change with refcount>0. Required receipt: `impact_checked`. Visibility enum is language-aware.
- **D-16** G-004 thresholds: `changed_lines > 50` OR `touched_files > 1`; secondary `changed_lines / file_line_count > 0.30` (default on, configurable).
- **D-17** G-005 detection: 3-signal (path glob, identifier pattern, security-sensitive import). Default catalogs ship for Go/TS/JS/Python.
- **D-18** G-005 timing: post-edit. Pre-edit classify+warn/enforce; apply edit; run diagnostics; if ErrorCount==0 issue `diagnostics_clean` receipt + auto-attach to outcome envelope.
- **D-19** Degraded mode (`semantic_index.enabled=false`): conservative warn for G-002/G-003; G-001/G-004/G-005 path+identifier signals still work; G-005 import detection degrades to grep-of-imports.
- **D-20** 5-layer enforcement precedence: `CLI > per-tool > per-rule > profile > global default`. **Highest-precedence layer that is set wins — NOT most-restrictive.**
- **D-21** `EnforcementLevel` closed enum: `off | warn | enforce | require_force`.
- **D-22** Config shape extends `internal/semantic/config.go` `GuardrailsConfig` with `rules`/`tools` maps + `G004`/`G005` config substructs.
- **D-23** Two docs: `./GUARDRAILS.md` (operator) + `./DoD.md` (agent-consumable per-task-class checklists).
- **D-24** `get_tool_help` adds 6 topics: `guardrails`, `dod`, `workflow:rename`, `workflow:delete`, `workflow:large-edit`, `workflow:security-sensitive-edit`.

### Claude's Discretion

Researcher resolves: OI-01..OI-05 (see Open Items section below), G-005 catalog scope (Java/Rust/C# parity vs defer), receipt-issuance hook strategy (post-handler callback vs wrapping middleware), `get_tool_help` topic dispatch (per-topic register vs prefix dispatch).

### Deferred Ideas (OUT OF SCOPE)

- G-006..G-010 (REQUIREMENTS.md line 143).
- Persistent / disk-backed receipts (REQUIREMENTS.md line 167 — explicitly forbidden).
- Force-override surface (`force: bool`, `override_reason: string`, audit-event format) — `require_force` enum value lands but the override transport ships in a follow-up phase.
- `semantic_guarded` eval-harness mode (Phase 67).
- New `semantic` profile catalog entry (D-22 mention is illustrative; `semantic` profile is NOT a v1.10 deliverable).

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| GUARD-01 | `GuardrailMiddleware` installed at daemon step 14b.5; LIFO order `LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler`; LazyInit-last invariant preserved. | "Architecture Patterns → Middleware install/execution LIFO"; daemon.go install sites verified at lines 698 / 706 / 729; `internal/mcp/middleware_test.go` is the canonical regression-assertion location. |
| GUARD-02 | Five rules enforced at warn-default with per-rule trigger logic. | "Per-Rule Detection" section maps each of G-001..G-005 to existing kernel signals (tree-sitter outline, refcount, blast nodes, line-count, path/identifier/import catalog). |
| GUARD-03 | Server-side receipts (5-min TTL keyed by `graph_version`); ID-only forwarding survives context compaction. | "Receipt Architecture" section + D-06 in-memory `sync.Map` storage; integration test pattern in "Validation Architecture → Integration: context-truncation E2E". |
| GUARD-04 | Receipt invalidation when `graph_version` advances past snapshot; clear "graph drifted" error. | `internal/semantic/integ.SemanticStatus.GraphVersion` is the live source; `ValidateReceiptForOperation` (D-05) returns `ErrGraphVersionMismatch`. |
| GUARD-05 | `TelemetryMiddleware` classifies `guardrail_blocked` / `guardrail_warned`; RED metrics surface them. | "Metrics" section — extend the closed `outcomeEnum` in `internal/mcp/middleware.go` lines 159-178; new `helix_receipt_*` counters mirror the `RecordEditOutcome` / `RecordRenameStrategy` bounded-label pattern. |
| GUARD-06 | `GUARDRAILS.md` + `DoD.md` committed; rule semantics + DoD for rename/delete/public-API-change. | "Documentation" section — repo-root files; `embed.FS` in daemon for runtime serving via `get_tool_help`. |
| GUARD-07 | Per-profile enforcement levels (`off | warn | require_force | enforce`); defaults `warn` read/edit, `require_force` review on high-risk pre-checks. | "Configuration" section — D-22 config shape; D-11 default table; OI-01 amendment proposal for `ci-bot=enforce` extension. |

</phase_requirements>

## Summary

Phase 66 lands a **server-side capability-receipt system** that gates six destructive MCP tools on prior evidence collected by seven read-side / diagnostics tools. The mechanism is intentionally narrow: an agent calling `find_references` or `analyze_blast_radius` receives a 26-char opaque receipt ID; the destructive tool's argument schema requires that ID; a new `GuardrailMiddleware` (installed at daemon step 14b.5) extracts and validates the receipt against an in-memory per-workspace `sync.Map` keyed by `graph_version`. Because the agent only carries the ID (not the receipt body), context compaction cannot fabricate a passing receipt, and `graph_version` advance auto-invalidates everything stale.

The design has three load-bearing invariants: **(1)** middleware install order is LIFO and `LazyInit` MUST remain installed last (executes first) so workspace activation precedes both telemetry deadlines and guardrail evaluation; **(2)** receipt scope-match is STRICT per class — class-only matching is rejected because it lets agents trade unrelated `find_references` calls for unrelated `rename_symbol` permissions; **(3)** receipts are server-only and graph-version-keyed — disk-backed receipts are explicitly forbidden by REQUIREMENTS.md line 167. Five guardrail rules (G-001 rename-by-grep, G-002 delete-without-refs, G-003 public-API-edit, G-004 large-fuzzy-edit, G-005 security-sensitive-edit) ship at warn-default, with `enforce` for `review`/`ci-bot` profiles.

The phase has **moderate research uncertainty** in three areas, all addressed below: (a) which Phase 65 `SemanticLookup` extensions are needed for visibility / entry-point / interface-contract predicates (OI-02, OI-03); (b) whether `get_tool_help`'s tool-name-keyed registry can carry non-tool topics (OI-04 — answer: it can't today, needs a topic-registry refactor); (c) the cleanest receipt-issuance hook for read tools (OI-05 — recommended: a thin post-handler closure invoked from each issuing tool's success path, NOT a global wrapping middleware, because read-tool result types are heterogeneous and middleware-side scope extraction would re-parse JSON).

**Primary recommendation:** Treat receipts as a typed-Go-union value object owned by `internal/guardrails/`, validated by a closed-enum switch with compile-time exhaustiveness. The middleware is a thin extractor + dispatcher; all rule logic lives behind a `RuleEvaluator` interface that takes `(toolName, args, sessionContext) → (Decision, []RequiredReceipt, error)`. This keeps middleware testable as a pure function and rule logic testable as pure business logic — direct alignment with TDD-mode RED/GREEN/REFACTOR task heuristics.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Middleware install + LIFO ordering | MCP runtime (`internal/mcp/`) | Daemon bootstrap (`internal/daemon/`) | Mirrors existing 4-middleware install pattern; daemon owns the call site at step 14b.5. |
| Receipt store + janitor | Guardrails subsystem (`internal/guardrails/`) | Workspace lifecycle (`internal/workspace/`) | Per-workspace `sync.Map` lifecycle binds to workspace activate/deactivate; storage is purely in-process. |
| Per-rule predicate evaluation | Guardrails subsystem (`internal/guardrails/`) | Semantic lookup seam (`internal/semantic/integ`) | Predicates need symbol metadata (refcount, visibility, entry-point) — semantic lookup is the read-only seam; degraded fallback when `semantic_index.enabled=false`. |
| Receipt issuance from read tools | Kernel tools (`internal/kernel/symbols`, `internal/kernel/diag`) + Skill tools (`internal/skill/repomap`) | Guardrails subsystem (closure injection) | Each issuing tool calls a daemon-injected closure on its success path — same pattern as `RecordEditOutcome` (Phase 53 D-16). |
| Receipt extraction from destructive tools | MCP middleware (`internal/mcp/guardrail_middleware.go`) | Tool argument schemas (per-tool struct) | Destructive tools declare `receipts []ReceiptID` in their argument struct; middleware extracts via reflection-free JSON path. |
| Telemetry outcome classification | Telemetry middleware (`internal/mcp/middleware.go`) | Guardrails (typed error sentinel) | Existing `classifyOutcome` switch reads `serr.Kind`; add `Kind == "guardrail_violation"` branch returning `outcomeGuardrailBlocked` or `outcomeGuardrailWarned`. |
| Guardrails / DoD documentation | Repo-root markdown + `internal/kernel/help/` | `embed.FS` in daemon | Same shipping pattern as `internal/profile/embed.go`; runtime serving via extended `get_tool_help` topic dispatch. |
| Configuration | `internal/semantic/config.go` `GuardrailsConfig` | `internal/config/defaults.go` | Existing struct extends with per-rule + per-tool maps + nested `G004` / `G005` configs (D-22). |

## Standard Stack

### Core (already in tree — verified by `ls`/`grep`)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `sync` | 1.25.1 (`go version` confirms) | `sync.Map` for receipt store; `sync.RWMutex` for janitor coordination | `[VERIFIED: go version on host]` `sync.Map` is the idiomatic in-process keyed cache for read-mostly workloads with per-key independence |
| Go stdlib `crypto/rand` | 1.25.1 | Receipt ID entropy | `[VERIFIED]` Standard for opaque-token generation |
| `github.com/google/uuid` (likely already pulled) | check `go.mod` | UUIDv7 timestamp-prefixed IDs (D-07) | `[ASSUMED]` Phase 66 may need to add; if not present, choose between `github.com/google/uuid` (most popular, supports v7 since 1.6) and `github.com/gofrs/uuid` |
| Go stdlib `encoding/base32` | 1.25.1 | Receipt ID rendering (no-padding alphabet per D-07) | `[VERIFIED]` |
| `github.com/modelcontextprotocol/go-sdk` | (already used) | `mcp.Middleware`, `mcp.MethodHandler`, `mcp.AddReceivingMiddleware` | `[VERIFIED: internal/mcp/middleware.go uses it]` |
| `github.com/knadh/koanf/v2` | (already pulled) | Config layering for D-22 shape | `[VERIFIED: CLAUDE.md "koanf v2"]` |
| `github.com/agenthands/helix/internal/errors` (`serr`) | in-repo | Typed-error taxonomy; new `Kind = "guardrail_violation"` constant | `[VERIFIED: internal/errors/kinds.go lines 13-22]` |

### Supporting (in-tree references)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/semantic/integ.SemanticLookup` | Phase 65 | Read-only seam for `GraphVersion`, `LocateSymbol`, `Status`; D-15 visibility extension | Every guardrail predicate that needs semantic-graph data; degraded-mode fallback when `Available()` is false |
| `internal/obs` Prometheus provider | Phase 47/53 | New `helix_receipt_issued_total`, `helix_receipt_expired_total`, `helix_receipt_lookup_total{outcome}` counters | Bounded-label pattern from `RecordEditOutcome` (Phase 53 D-16) and `RecordRenameStrategy` (Phase 47 D-07) |
| `embed.FS` (Go stdlib) | 1.25.1 | Bundle `GUARDRAILS.md` + `DoD.md` + workflow snippets into the binary | Same shipping pattern as `internal/profile/embed.go` |
| `internal/kernel/edit/treesitter.go` | in-tree | Symbol outline lookup for G-001 detection (D-13) | Already extracts symbols per file; G-001 reuses existing extractor |
| `internal/fuzzy/match.go` | Phase 53 | Reports strategy used; G-001 inspects `find` arg BEFORE matching | No change to fuzzy package |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In-memory `sync.Map` (D-06) | Disk-backed BoltDB / DuckDB receipt rows | **Rejected** — REQUIREMENTS.md line 167 explicitly forbids persistent receipts detached from `graph_version`; in-memory matches threat model |
| Typed Go union for scope (D-03) | `map[string]any` or `json.RawMessage` | **Rejected** — D-03 mandates typed; benefit: switch with compile-time exhaustiveness, no per-tool stringly-typed validation |
| Wrapping middleware for receipt issuance (OI-05) | Per-tool post-handler closure | **Recommend per-tool closure** — read-tool result types are heterogeneous; middleware-side scope extraction would re-marshal/unmarshal JSON; closure injection mirrors proven `RecordEditOutcome` pattern |
| Class-only receipt match | STRICT scope match (D-09) | **Locked: STRICT** — class-only is the loophole receipts exist to prevent |
| Single global `EnforcementLevel` | 5-layer precedence (D-20) | **Locked: 5-layer** — operators need per-rule and per-tool overrides for enforce-mode rollout |

**Installation (no new deps if `google/uuid` is already in go.mod):**
```bash
# Verify first:
grep "google/uuid" go.mod || go get github.com/google/uuid@latest
```

**Version verification:** `[ASSUMED]` `github.com/google/uuid` v1.6.0+ supports UUIDv7 (`uuid.NewV7()`). Researcher should confirm with `go list -m github.com/google/uuid` once the planner runs Wave 0; if absent, `go get github.com/google/uuid@v1.6.0` (or later — verify via `npm view` equivalent: `go list -m -versions github.com/google/uuid` at plan time).

## Architecture Patterns

### System Architecture Diagram

```
                        ┌──────────────────────────────────────────────┐
   tools/call request   │           MCP Server (mcpsdk)                │
   ───────────────────► │                                              │
                        │  LIFO middleware chain (install = step,      │
                        │  execution = reverse):                       │
                        │                                              │
                        │  ┌──────────────────────────────────┐        │
                        │  │ LazyInit (14c — installed last)  │        │
                        │  │  → activate workspace if needed  │        │
                        │  └──────────────┬───────────────────┘        │
                        │                 ▼                            │
                        │  ┌──────────────────────────────────┐        │
                        │  │ Guardrail (14b.5 — NEW)          │ ──► reads receipts
                        │  │  → extract receipts[] from args  │      from per-workspace
                        │  │  → evaluate per-rule predicates  │      sync.Map
                        │  │  → ValidateReceiptForOperation   │ ──► consults
                        │  │  → warn|block|enforce|require_f  │      SemanticLookup
                        │  │  → emit guardrail_warned/blocked │      (visibility, refcount,
                        │  │  → typed serr.GuardrailViolation │      entry-point, blast)
                        │  └──────────────┬───────────────────┘        │
                        │                 ▼                            │
                        │  ┌──────────────────────────────────┐        │
                        │  │ Suggestion (14b)                 │        │
                        │  │  → "did you mean" enrichment     │        │
                        │  └──────────────┬───────────────────┘        │
                        │                 ▼                            │
                        │  ┌──────────────────────────────────┐        │
                        │  │ ProfileFilter (14)               │        │
                        │  │  → filters tools/list (not call) │        │
                        │  └──────────────┬───────────────────┘        │
                        │                 ▼                            │
                        │  ┌──────────────────────────────────┐        │
                        │  │ Telemetry (14)                   │        │
                        │  │  → RED metrics, deadline inject  │        │
                        │  │  → outcome enum {success, ...,   │        │
                        │  │     guardrail_warned, blocked}   │        │
                        │  └──────────────┬───────────────────┘        │
                        │                 ▼                            │
                        │            tool handler                      │
                        │                 │                            │
                        │                 ▼ (read tools only)          │
                        │     ┌─────────────────────────────┐          │
                        │     │ post-handler closure        │ ───► IssueReceipt(...)
                        │     │  IssueReceiptOnSuccess()    │      writes to per-workspace
                        │     │  → scope from result        │      sync.Map; emits
                        │     │  → graph_version snapshot   │      helix_receipt_issued_total
                        │     │  → freshness from envelope  │
                        │     └─────────────────────────────┘
                        └──────────────────────────────────────────────┘

                        Per-workspace receipt store
                        ───────────────────────────
                        sync.Map[ReceiptID]*Receipt
                        ├─ janitor: 30s tick → sweep expired
                        ├─ on-lookup expiry check
                        ├─ LRU eviction at 10k cap
                        └─ invalidation: graph_version > receipt.graph_version

                        Read tools (issue):           Destructive tools (consume):
                        ──────────────────            ──────────────────────────
                        find_references               rename_symbol
                        analyze_blast_radius          safe_delete_symbol
                        get_context                   replace_symbol_body
                        get_semantic_context          fuzzy_edit
                        get_repo_map                  replace_in_file
                        get_diagnostics (post-edit)   delete_file
                        verify_edit (post-edit)
                        run_diagnostics (post-edit)
```

### Recommended Project Structure

```
internal/
├── guardrails/                    # NEW
│   ├── doc.go                     # package doc + middleware order invariant restatement
│   ├── receipt.go                 # Receipt struct + closed ReceiptClass enum + per-class scope structs
│   ├── receipt_id.go              # rcpt_<base32-26> generator + ParseReceiptID validator
│   ├── store.go                   # in-memory sync.Map per-workspace store + janitor + LRU
│   ├── store_test.go              # TTL, graph_version invalidation, LRU eviction, concurrency
│   ├── validate.go                # ValidateReceiptForOperation + typed sentinel errors
│   ├── validate_test.go
│   ├── enforcement.go             # EnforcementLevel enum + ResolveGuardrailEnforcement (5-layer)
│   ├── enforcement_test.go
│   ├── rules/                     # one file per rule for testability
│   │   ├── g001_rename_by_grep.go
│   │   ├── g002_delete_without_refs.go
│   │   ├── g003_public_api_edit.go
│   │   ├── g004_large_fuzzy_edit.go
│   │   ├── g005_security_sensitive.go
│   │   └── *_test.go              # one test file per rule, pure-business-logic tests
│   ├── catalogs/                  # G-005 import-pattern catalog YAMLs
│   │   ├── go.yaml
│   │   ├── typescript.yaml
│   │   ├── javascript.yaml
│   │   ├── python.yaml
│   │   └── embed.go               # embed.FS + load func
│   ├── visibility.go              # IsPublicLike + per-language Visibility enum (D-15)
│   ├── visibility_test.go
│   └── deps.go                    # MiddlewareDeps interface (skill-setter pattern)
│
├── mcp/
│   ├── guardrail_middleware.go    # NEW — InstallGuardrailMiddleware + middleware fn
│   └── guardrail_middleware_test.go
│
├── skill/
│   └── guardrails/                # NEW — blank-imported by daemon for init() registration
│       ├── skill.go
│       └── doc.go
│
├── daemon/
│   ├── daemon.go                  # ADD step 14b.5 between line 706 and 729
│   └── imports.go                 # add blank-import of internal/skill/guardrails
│
├── kernel/
│   ├── help/
│   │   ├── topics.go              # NEW — topic-registry refactor (OI-04)
│   │   └── topics_test.go
│   ├── symbols/
│   │   ├── retrieval.go           # ADD IssueReceipt closure call on find_references success
│   │   └── blast.go               # ADD IssueReceipt closure call on analyze_blast_radius success
│   ├── diag/
│   │   └── tools.go               # ADD IssueReceipt closure call on get_diagnostics success
│   └── edit/
│       └── verify.go              # ADD IssueReceipt closure call on verify_edit success (post-edit)
│
├── skill/repomap/
│   └── tools.go                   # ADD IssueReceipt for get_repo_map / get_context success
│
├── semantic/
│   ├── config.go                  # EXTEND GuardrailsConfig (D-22)
│   └── integ/
│       └── lookup.go              # EXTEND SemanticLookup with Visibility, IsEntrypointReachable (OI-02/OI-03)
│
└── config/
    └── defaults.go                # EXTEND semantic_index.guardrails.* defaults

# Repo root (NEW):
GUARDRAILS.md   # operator/user docs
DoD.md          # agent-consumable Definition-of-Done checklists
```

### Pattern 1: Middleware install order = LIFO execution order
**What:** `mcp-go-sdk.AddReceivingMiddleware` composes in LIFO order; the LAST installed middleware runs FIRST on incoming requests.
**When to use:** Every middleware install in `internal/daemon/daemon.go`. The `LazyInit-last-installed (executes-first)` invariant is **load-bearing**: workspace activation MUST precede `TelemetryMiddleware`'s deadline injection (per `internal/mcp/lazy_init.go:106-108`).
**Example:**
```go
// Source: internal/daemon/daemon.go (verified at lines 698, 706, 729)

// Step 14: install Telemetry + ProfileFilter
helixMCP.InstallMiddleware(mcpServer.SDK(), observability, profileStore, getSessionFn, budgetFn, mcpServer.Registry(), logger)

// Step 14b: install Suggestion
helixMCP.InstallSuggestionMiddleware(mcpServer.SDK(), suggestionSchemaMap, logger)

// Step 14b.5 (NEW — Phase 66): install Guardrail
helixMCP.InstallGuardrailMiddleware(mcpServer.SDK(), guardrailDeps, logger)

// Step 14c: install LazyInit LAST
helixMCP.InstallLazyInitMiddleware(mcpServer.SDK(), lazyActivateFn, isActiveFn, "", logger)

// Resulting LIFO execution order:
// LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler
```
The regression test extends `internal/mcp/middleware_test.go` (verified to exist) with a new 5-step order assertion. `[VERIFIED: internal/mcp/lazy_init.go:106-108, internal/daemon/daemon.go:698-729]`

### Pattern 2: Skill setter for cross-package wiring (Phase 65)
**What:** A skill or middleware exposes a setter (`Set*Lookup`, `Set*Deps`) that the daemon calls post-init to inject a concrete adapter without taking a build dependency on heavy subsystems.
**When to use:** Wiring `internal/guardrails` into the middleware without making `internal/mcp` import the guardrail rule logic.
**Example:**
```go
// In internal/guardrails/deps.go:
type MiddlewareDeps interface {
    WorkspaceState(ctx context.Context) WorkspaceKey
    IssueReceipt(ctx context.Context, class ReceiptClass, scope any, issuingTool string, callID string) (ReceiptID, error)
    LookupReceipt(ctx context.Context, ws WorkspaceKey, id ReceiptID) (*Receipt, bool)
    ValidateReceiptForOperation(ctx context.Context, ws WorkspaceKey, id ReceiptID, target Target) error
    Visibility(ctx context.Context, ws WorkspaceKey, sym SymbolID) (Visibility, error)
    IsEntrypointReachable(ctx context.Context, ws WorkspaceKey, sym SymbolID) (bool, error)
    ParticipatesInInterfaceContract(ctx context.Context, ws WorkspaceKey, sym SymbolID) (bool, error)
    ResolveEnforcement(profile, tool, rule string) EnforcementLevel
    Config() GuardrailsConfig
    GraphVersion(ctx context.Context, ws WorkspaceKey) uint64
}

// daemon.go post-init:
guardrailDeps := guardrails.NewProductionDeps(semanticLookup, profileStore, cfg.SemanticIndex.Guardrails, store, logger)
helixMCP.InstallGuardrailMiddleware(mcpServer.SDK(), guardrailDeps, logger)
```

### Pattern 3: Closed-enum bounded-label metrics (Phase 47/53)
**What:** Every Prometheus counter label is a closed-enum string with a small fixed cardinality; no source content, no error text, no agent-supplied data ever reaches a label.
**When to use:** All new `helix_receipt_*` counters and the extended `outcomeEnum`.
**Example:**
```go
// Extend internal/mcp/middleware.go outcomeEnum:
const (
    outcomeSuccess          = "success"
    outcomeInvalidArgs      = "invalid_args"
    outcomeNotFound         = "not_found"
    outcomeCircuitOpen      = "circuit_open"
    outcomeLSCrash          = "ls_crash"
    outcomeTimeout          = "timeout"
    outcomeInternal         = "internal"
    outcomeGuardrailWarned  = "guardrail_warned"   // NEW (Phase 66)
    outcomeGuardrailBlocked = "guardrail_blocked"  // NEW (Phase 66)
)

// Receipt-specific counters (mirror RecordEditOutcome pattern):
//   helix_receipt_issued_total{class}                — closed: 5 classes
//   helix_receipt_expired_total{reason}              — closed: {ttl, graph_drift, lru_evicted}
//   helix_receipt_lookup_total{outcome}              — closed: {hit, miss, expired, scope_mismatch,
//                                                              graph_drift, workspace_mismatch,
//                                                              freshness_rejected, wrong_class}
```
Cardinality: `lookup_outcome × 8 = 8` bounded-label values. Full taxonomy already drafted in CONTEXT.md "Established Patterns".

### Pattern 4: Per-tool closure injection for tool-side hooks (Phase 53 D-16)
**What:** Tool handler imports a thin function (e.g., `mcp.RecordEditOutcome`) that is set once at daemon bootstrap; if the sink is unset (test), the call is a no-op.
**When to use:** Receipt issuance from read tools (OI-05). Each read tool's handler calls `guardrails.IssueReceiptOnSuccess(ctx, class, scope)` on its success path.
**Example:**
```go
// In internal/guardrails/issue_sink.go (mirrors internal/mcp/middleware.go editOutcomeSink pattern):
var receiptIssueSink atomic.Pointer[func(ctx context.Context, class ReceiptClass, scope any, tool string) (ReceiptID, error)]

func SetReceiptIssueSink(fn func(...) (ReceiptID, error)) { receiptIssueSink.Store(&fn) }
func IssueReceiptOnSuccess(ctx context.Context, class ReceiptClass, scope any, tool string) (ReceiptID, error) {
    p := receiptIssueSink.Load()
    if p == nil || *p == nil { return "", nil } // no-op in tests
    return (*p)(ctx, class, scope, tool)
}

// In internal/kernel/symbols/retrieval.go FindReferences handler success path:
guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassReferencesChecked, guardrails.ReferencesCheckedScope{
    SymbolID: symID, RefCount: len(refs), FilePath: relPath, IncludeTests: args.IncludeTests,
}, "find_references")
// Receipt ID can be returned in the tool's envelope for debug, but per D-07 it is opaque to the agent.
```

### Pattern 5: Topic registry refactor for `get_tool_help` (OI-04 resolution)
**What:** Today `get_tool_help` looks up `tool_name` in the `ToolRegistry` (verified in `internal/kernel/help/tools.go` line 33). For D-24 it must serve 6 non-tool topics. Recommended: introduce a parallel `TopicRegistry` (file: `internal/kernel/help/topics.go`) keyed on `topic` arg with two dispatch shapes:
```go
type Topic struct {
    Name    string  // "guardrails", "dod", "workflow:rename", ...
    Content string  // markdown body, possibly loaded from embed.FS
}

type TopicRegistry struct { topics map[string]*Topic }
func (r *TopicRegistry) Get(name string) (*Topic, bool) { /* exact match */ }
```
**Decision:** **Exact-match-only registry** (defer prefix dispatch). Register all 6 topic names individually as `(guardrails, dod, workflow:rename, workflow:delete, workflow:large-edit, workflow:security-sensitive-edit)`. The `get_tool_help` tool's argument schema gains an optional `topic` field; if `topic` is set, dispatch to TopicRegistry; if `tool_name` is set, dispatch to ToolRegistry (existing path). This is forward-compatible with prefix dispatch later but ships less surface area now.

### Anti-Patterns to Avoid

- **Class-only receipt match:** lets agents trade `find_references(harmless_symbol)` for `rename_symbol(auth_middleware)`. STRICT scope match per D-09 prevents this.
- **Storing receipts client-side:** breaks the entire threat model — context compaction can fabricate receipts. Server-side only (D-06, REQUIREMENTS.md line 167).
- **Receipt store keyed only by ID (no workspace):** allows cross-workspace receipt forgery in multi-workspace daemons. Always validate `receipt.workspace_key == current_workspace`.
- **Most-restrictive-wins enforcement resolution:** D-20 explicitly mandates highest-precedence-layer-wins; the wrong reading hides operator overrides.
- **Issuing receipts BEFORE the read tool succeeds:** opens a race where a refused read still produces a passing receipt. Issue strictly on success path (D-18 mirrors this for diagnostics).
- **Reflection-based extraction of `receipts[]` field:** Each destructive tool's argument struct already serializes via the SDK's typed-args mechanism; declare `Receipts []ReceiptID `json:"receipts"`` in each struct and read it via `req.Params.Arguments` JSON path or via the typed args after `mcpsdk.ParseArguments`. NO reflection.
- **Mutating the request before `next(ctx, ...)`:** middlewares are read-side only; if a guardrail blocks, return a typed error or a `CallToolResult{IsError: true}` directly without calling `next`.
- **Silently dropping unknown receipt classes:** ParseReceiptID + class-from-store lookup MUST surface "wrong class" as a typed error so the agent gets a clear remediation message via D-12 envelopes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Time-ordered opaque IDs | Custom `time.Now().UnixNano()` + counter | `github.com/google/uuid` `NewV7()` + base32 encoding | UUIDv7 has a battle-tested timestamp-prefix scheme; rolling your own loses log-readability and risks clock-skew duplicates |
| In-process keyed cache with TTL | `map[string]*Receipt` + `sync.Mutex` | `sync.Map` + janitor goroutine (D-06) | `sync.Map` is the idiomatic Go pattern for read-mostly per-key independent workloads; rolling a wrapped mutex map invents a worse `sync.Map` |
| Closed-enum metric labels | Free-form string concatenation | Bounded-label discipline + `outcomeEnum`-style closed slice (Phase 47/53) | Cardinality-explosion is M6 in PITFALLS.md; CI has a PromQL validator that fails closed if a label leaks an unbounded value |
| Tree-sitter outline extraction for G-001 | New tree-sitter integration | `internal/kernel/edit/treesitter.go` (already extracts symbols per file) | The existing extractor is canonical; reusing it keeps the symbol-name source consistent with `replace_symbol_body` |
| LSP-backed reference count for G-002/G-003 | `grep -r` of identifier | `internal/kernel/symbols/retrieval.go` `FindReferences` (kernel tool) — invoked through the `SemanticLookup` seam or directly | Real LSP results catch dynamic dispatch and cross-file edges; SMTC routing rules in CLAUDE.md mandate this |
| Visibility classification | Hand-rolled regex on `name[0]` | `internal/semantic/extract/fact.go` `Visibility` field (`exported|private|package|...`) — already populated for indexed languages | `[VERIFIED: extract/fact.go line 98]` Already in the symbol-fact stream for languages that index; reuse via `SemanticLookup` extension (OI-02) |
| Typed error sentinel infrastructure | Custom `errors.As` chains | `internal/errors` (`serr`) — add `Kind = "guardrail_violation"` to `kinds.go` | `[VERIFIED: internal/errors/kinds.go]` Existing taxonomy already supports `errors.Is`/`errors.As`; one-line addition |
| Embedding markdown into the binary | Filesystem reads at runtime | `embed.FS` (Go stdlib) — same pattern as `internal/profile/embed.go` | Single-binary distribution invariant requires this; runtime FS reads break the no-runtime-deps charter |
| YAML config parsing | `gopkg.in/yaml.v3` direct calls | `koanf` v2 (already wired) — extend `GuardrailsConfig` struct, defaults flow through `internal/config/defaults.go` | 4-layer precedence (CLAUDE.md) is uniform across the project |

**Key insight:** Phase 66 should be ~80% wiring of existing primitives (`SemanticLookup`, fuzzy + tree-sitter symbol outline, koanf config, `embed.FS`, closed-enum metrics, typed `serr` errors) and ~20% new code (the receipt store, the rule-evaluator framework, and the middleware shell). If a wave looks like it's "building a custom X for guardrails," check whether the existing X (already used by Phase 60-65) can be extended.

## Runtime State Inventory

> Phase 66 is **greenfield + minor extension** of existing config/middleware. It does NOT rename, refactor, or migrate anything. The categories below are filled in for completeness; nothing requires a runtime-state migration.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — receipts are in-memory only (D-06); no DB tables, no SQLite rows, no DuckDB receipt tables (REQUIREMENTS.md line 167 forbids persistence) | None |
| Live service config | `helix daemon` running with prior version: receipts unknown to old daemon. Acceptable: receipt absence falls into warn/enforce path naturally. No live workflow definitions reference guardrails. | None |
| OS-registered state | None — no Task Scheduler / launchd / systemd entries reference guardrails. | None |
| Secrets / env vars | None — receipts are server-only opaque IDs; no secret material involved | None |
| Build artifacts / installed packages | If `github.com/google/uuid` newly added, `go.sum` updates; binary embeds new `embed.FS` for GUARDRAILS.md/DoD.md/catalogs. Old binaries continue to work without guardrails (config flag `enabled=true` default; absence-of-config = warn-default). | Standard `go build`/`make build` rebuild |

**Verified by:** grep across `.planning/PROJECT.md`, `internal/config/defaults.go`, `internal/semantic/store/` for receipt persistence — none found. CONTEXT.md D-06 explicitly chooses in-memory.

## Common Pitfalls

### Pitfall 1: Middleware install order silently wrong
**What goes wrong:** Guardrail middleware installed at step 14c (after LazyInit) — receipt validation runs BEFORE workspace activation, so workspace-scoped receipt lookup fails with "no workspace" instead of evaluating.
**Why it happens:** SDK `AddReceivingMiddleware` composes LIFO; the LazyInit-last invariant (`internal/mcp/lazy_init.go:106-108`) is easy to miss when adding step 14b.5.
**How to avoid:** Add a regression-asserted unit test in `internal/mcp/middleware_test.go` that invokes the actual `Install*Middleware` chain in production order and asserts the 5-step LIFO execution order via a sequence-recording test handler. **MANDATORY** per GUARD-01 success criterion ("LazyInit-last invariant preserved, regression-asserted").
**Warning signs:** `helix_receipt_lookup_total{outcome="workspace_mismatch"}` counter spike on first request after daemon restart; `lazy_init` middleware never called for guarded tools.

### Pitfall 2: Receipt scope-match validates class but not symbol identity
**What goes wrong:** Agent calls `find_references(HarmlessSymbol)` then forwards that receipt with `rename_symbol(AuthMiddleware)`; if validator only checks `class == "references_checked"` it admits the rename.
**Why it happens:** Cargo-culted from earlier draft designs that used class-only matching.
**How to avoid:** D-09 STRICT scope match. Validator MUST switch on class and call the per-class scope-equality check (`SymbolID == target.SymbolID`, or `FileSetContainsAll`, or `PathIsWithin`). Test fixture: receipt issued for symbol A, validation requested for symbol B, expect `ErrReceiptScopeMismatch`.
**Warning signs:** Increasing `guardrail_blocked` rate post-deploy with `lookup_outcome="scope_mismatch"` — that's the validator working as intended. The reverse (warn-mode false positives staying low while real prevention also stays low) suggests scope-match is too loose.

### Pitfall 3: `graph_version` not bumped when a destructive edit completes
**What goes wrong:** Agent runs `find_references`, gets receipt at `graph_version=42`, runs `rename_symbol`, `graph_version` stays at 42, agent re-uses the SAME receipt for a SECOND rename — but the codebase now contains stale references.
**Why it happens:** `BumpGraphVersion` is per-snapshot-commit, not per-tool-call. A rename's overlay update may not have committed before a second rename starts.
**How to avoid:** Receipts are SINGLE-USE in semantic effect — the `graph_version` advance after the first destructive edit invalidates them. The receipt store SHOULD also delete consumed receipts on lookup-and-validate-success (defensive, NOT a substitute for graph_version invalidation). Document explicitly in GUARDRAILS.md.
**Warning signs:** Same receipt ID appearing in two destructive tool calls within one minute; `helix_receipt_lookup_total{outcome="graph_drift"}` spiking.

### Pitfall 4: Receipt issuance from a tool that errored partway
**What goes wrong:** `find_references` returns 100 references for symbol A, then errors on the 101st file's LSP probe. Tool returns IsError=true. If the receipt is issued at the entry point or in defer, a passing receipt for symbol A is now in the store despite the partial result.
**Why it happens:** `defer` blocks run regardless of error; entry-point issuance can't see the success classification.
**How to avoid:** Issue receipts ONLY on the explicit success path of the handler — same discipline as `RecordEditOutcome` (Phase 53 D-16) which is gated on `outcome == "success"`. NEVER in `defer`. NEVER before the result is constructed.
**Warning signs:** Receipts issued for tool calls that returned `IsError: true`. Add a `helix_receipt_issued_total{from_error_path}` debug counter during early rollout.

### Pitfall 5: G-005 import-pattern catalog drift between languages
**What goes wrong:** Go catalog includes `crypto/*` but TS catalog forgets `bcrypt`/`argon2`; agents avoid guardrail by editing TS auth code. Or worse: catalog is over-broad and every edit to `fmt` triggers G-005.
**Why it happens:** Catalog is a domain-knowledge artifact; without explicit ownership it drifts toward whichever languages got attention first.
**How to avoid:** Ship default catalogs as embedded YAML with explicit user override (D-22 `guardrails.G-005.import_patterns.<lang>`); document the override path in GUARDRAILS.md; treat catalog updates as semver-minor (additions safe, removals breaking). Researcher recommends shipping **Go + TS + JS + Python in v1**, deferring **Java + Rust + C#** to a follow-up plan with eval-corpus signal — see "Open Questions" below.
**Warning signs:** `helix_receipt_lookup_total{outcome="hit", class="diagnostics_clean"}` going to zero for a language (no agent ever cleared the post-edit gate — likely false-positive trigger rate is too high).

### Pitfall 6: `require_force` enforcement path silently equivalent to `enforce` (this phase)
**What goes wrong:** Agent calling against a `require_force`-configured rule sees `enforce`-mode behavior and assumes the override surface is broken; or, the override surface ships in a follow-up phase but profiles already use `require_force` expecting the override to work.
**Why it happens:** D-21 enum lands `require_force` but the override transport is deferred; documentation must be explicit.
**How to avoid:** GUARDRAILS.md MUST contain a "Known limitations" section: "`require_force` currently behaves identically to `enforce`. Force-override transport (`force: bool`, `override_reason: string`) ships in v1.10.x or v1.11." Recommend `review` profile defaults stay `require_force` per GUARD-07 — the upgrade path will Just Work when override transport ships.
**Warning signs:** Operator confusion in support tickets; `review`-profile users disabling guardrails entirely instead of using force.

### Pitfall 7: Telemetry `outcome` enum drift
**What goes wrong:** Adding `guardrail_warned`/`guardrail_blocked` to `outcomeEnum` without updating the dashboard PromQL or the CI cardinality validator — ingestion silently drops the new label values.
**Why it happens:** Closed-enum discipline is enforced by a validator (CLAUDE.md "PromQL validator (registry-driven, fail-closed)"); forgetting to update the registry means the new values pass the local enum check but fail the validator.
**How to avoid:** Update `outcomeEnum` (slice in `internal/mcp/middleware.go` line 170-178) AND the validator's enum registry in the same wave. Add a unit test that marshals every enum value through the metric pipeline.
**Warning signs:** CI metric-cardinality test failure on first commit; missing dashboard panels.

### Pitfall 8: Context-truncation E2E test harness drift
**What goes wrong:** Integration test for GUARD-02 SC-2 runs in-process against the daemon — easy to write, but doesn't actually prove ID-only-forwarding survives compaction because in-process tests don't model the forwarder→gRPC→daemon hop where compaction-deletion would occur.
**Why it happens:** Convenience: in-process is faster and the assertion shape is identical.
**How to avoid:** Drive the test through the stdio forwarder + gRPC daemon (see CONTEXT.md "Specifics" — `test/harness/Runner` pattern). The test deletes the prior read-tool's response from the agent-side message log before invoking the destructive tool with a fabricated `receipts: []`. This satisfies both GUARD-02 SC-2 and the user-memory directive `feedback_uat_no_manual_mcp.md` (UAT must automate MCP/daemon checks).
**Warning signs:** Test passes locally but a real agent (Claude Code) bypasses guardrails after compaction — indicates the test is testing the wrong layer.

## Code Examples

### Receipt struct + closed-enum class

```go
// Source: pattern derived from internal/semantic/integ/source.go closed-enum + Phase 47 D-07 bounded-label
package guardrails

type ReceiptClass string

const (
    ClassReferencesChecked ReceiptClass = "references_checked"
    ClassImpactChecked     ReceiptClass = "impact_checked"
    ClassContextGathered   ReceiptClass = "context_gathered"
    ClassStructuralOverview ReceiptClass = "structural_overview"
    ClassDiagnosticsClean  ReceiptClass = "diagnostics_clean"
)

var receiptClassEnum = []ReceiptClass{
    ClassReferencesChecked, ClassImpactChecked, ClassContextGathered,
    ClassStructuralOverview, ClassDiagnosticsClean,
}

// Receipt is the in-memory record. Wire format on the agent's side is
// just the ID — never the body.
type Receipt struct {
    ID             ReceiptID
    Class          ReceiptClass
    SchemaVersion  uint32
    WorkspaceKey   workspace.WorkspaceKey
    SnapshotID     uint64
    GraphVersion   uint64
    Freshness      string  // closed-enum mirror of integ.Freshness
    ScoreStatus    string
    ClusterStatus  string
    PendingLSPFiles int
    LSPCoverage    float64
    IssuedAt       time.Time
    ExpiresAt      time.Time
    IssuingTool    string
    IssuingCallID  string
    TraceID        string
    Scope          ReceiptScope  // typed Go union (not interface{}) — see D-03
}

// ReceiptScope is the typed union. Per D-03, it is NOT map[string]any.
// Concrete realisation: an interface with a marker method, validated by
// a switch on Class.
type ReceiptScope interface{ isReceiptScope() }

type ReferencesCheckedScope struct {
    SymbolID     SymbolID
    RefCount     int
    FilePath     string
    IncludeTests bool
}
func (ReferencesCheckedScope) isReceiptScope() {}

type ImpactCheckedScope struct {
    SymbolID         SymbolID
    RefCount         int
    PublicAPI        bool
    BlastNodes       int
    MaxDepth         int
    IncludedCallers  bool
    IncludedTypes    bool
}
func (ImpactCheckedScope) isReceiptScope() {}

// ... ContextGatheredScope, StructuralOverviewScope, DiagnosticsCleanScope
```

### ValidateReceiptForOperation predicate

```go
// Source: D-05 (CONTEXT.md) + classifyOutcome pattern from internal/mcp/middleware.go:241-258
func ValidateReceiptForOperation(
    ctx context.Context,
    rcpt *Receipt,
    target Target,
    currentGraphVersion uint64,
    currentWorkspace workspace.WorkspaceKey,
) error {
    if rcpt.WorkspaceKey != currentWorkspace {
        return ErrWrongWorkspace
    }
    if rcpt.GraphVersion != currentGraphVersion {
        return ErrGraphVersionMismatch
    }
    if time.Now().After(rcpt.ExpiresAt) {
        return ErrReceiptExpired
    }
    if !freshnessAllowed(rcpt.Freshness, target.MinFreshness) {
        return ErrReceiptTooStale
    }
    if rcpt.Class != target.RequiredClass {
        return ErrWrongReceiptClass
    }
    return validateScope(rcpt.Class, rcpt.Scope, target)  // STRICT per D-09
}

// validateScope is a switch on class — compile-time exhaustiveness via
// closed enum + go vet / linter check.
func validateScope(class ReceiptClass, scope ReceiptScope, target Target) error {
    switch class {
    case ClassReferencesChecked, ClassImpactChecked:
        s := scope.(SymbolAnchored)  // typed assertion enforced by Go
        if s.SymbolIdentity() != target.SymbolID { return ErrReceiptScopeMismatch }
    case ClassContextGathered:
        s := scope.(ContextGatheredScope)
        if !FileSetContainsAll(s.FileSet, target.TouchedFiles) { return ErrReceiptScopeMismatch }
    case ClassStructuralOverview:
        s := scope.(StructuralOverviewScope)
        if !PathIsWithin(target.Path, s.RootPath) { return ErrReceiptScopeMismatch }
    case ClassDiagnosticsClean:
        s := scope.(DiagnosticsCleanScope)
        if !FileSetContainsAll(s.FileSet, target.SecuritySensitiveTouchedFiles) || s.ErrorCount != 0 {
            return ErrReceiptScopeMismatch
        }
    }
    return nil
}
```

### Receipt store with janitor (D-06)

```go
// Source: D-06 pattern, mirrors internal/kernel/lspool worker-cache discipline
type Store struct {
    mu        sync.RWMutex
    receipts  *sync.Map               // map[ReceiptID]*Receipt
    perWS     map[workspace.WorkspaceKey]*lru.Cache  // 10k cap per workspace
    janitorTk *time.Ticker            // 30s
    closeCh   chan struct{}
    metrics   MetricsSink
}

func (s *Store) Get(ws workspace.WorkspaceKey, id ReceiptID) (*Receipt, bool) {
    v, ok := s.receipts.Load(string(id))
    if !ok { s.metrics.IncLookupOutcome("miss"); return nil, false }
    rcpt := v.(*Receipt)
    if time.Now().After(rcpt.ExpiresAt) {
        s.receipts.Delete(string(id))
        s.metrics.IncLookupOutcome("expired")
        return nil, false
    }
    if rcpt.WorkspaceKey != ws {
        s.metrics.IncLookupOutcome("workspace_mismatch")
        return nil, false
    }
    return rcpt, true
}

func (s *Store) janitor() {
    for {
        select {
        case <-s.janitorTk.C:
            now := time.Now()
            s.receipts.Range(func(k, v any) bool {
                if now.After(v.(*Receipt).ExpiresAt) { s.receipts.Delete(k); s.metrics.IncExpired("ttl") }
                return true
            })
        case <-s.closeCh: return
        }
    }
}

func (s *Store) InvalidateOnGraphVersionAdvance(ws workspace.WorkspaceKey, newGV uint64) {
    s.receipts.Range(func(k, v any) bool {
        rcpt := v.(*Receipt)
        if rcpt.WorkspaceKey == ws && rcpt.GraphVersion < newGV {
            s.receipts.Delete(k); s.metrics.IncExpired("graph_drift")
        }
        return true
    })
}
```

### Middleware shell

```go
// Source: derived from internal/mcp/lazy_init.go:106-112 + suggest.go:253-256 install patterns

func InstallGuardrailMiddleware(server *mcpsdk.Server, deps guardrails.MiddlewareDeps, logger *slog.Logger) {
    server.AddReceivingMiddleware(GuardrailMiddleware(deps, logger))
}

func GuardrailMiddleware(deps guardrails.MiddlewareDeps, logger *slog.Logger) mcpsdk.Middleware {
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            if method != "tools/call" {
                return next(ctx, method, req)
            }
            ctr, ok := req.(*mcpsdk.CallToolRequest)
            if !ok || ctr == nil || ctr.Params == nil {
                return next(ctx, method, req)
            }
            toolName := ctr.Params.Name

            // Only intercept destructive tools (closed list per D-08).
            if !isDestructiveTool(toolName) {
                return next(ctx, method, req)
            }

            decision, err := deps.Evaluate(ctx, toolName, ctr.Params.Arguments)
            if err != nil {
                return nil, err  // typed serr; TelemetryMiddleware classifies via Kind
            }
            switch decision.Action {
            case ActionAllow:
                return next(ctx, method, req)
            case ActionWarn:
                result, callErr := next(ctx, method, req)
                return attachWarnings(result, decision.Warnings), callErr
            case ActionBlock:
                // Return typed serr.GuardrailViolation; TelemetryMiddleware
                // classifies as outcome=guardrail_blocked.
                return nil, serr.NewGuardrailViolation(decision.Rule, decision.Message,
                    decision.RequiredReceipts, decision.SuggestedTools, decision.SeeAlso)
            }
            return next(ctx, method, req)
        }
    }
}
```

## Per-Rule Detection (research-supplemental to D-13..D-18)

| Rule | Trigger source | Required receipt | Existing code reuse |
|------|---------------|------------------|---------------------|
| **G-001** rename-by-grep | `internal/kernel/edit/treesitter.go` symbol outline + identifier regex `^[A-Za-z_][A-Za-z0-9_]{2,}$` | `references_checked` OR `impact_checked` (strict symbol scope) | Tree-sitter outline already extracted for `replace_symbol_body`; reuse directly |
| **G-002** delete-without-refs | `SemanticLookup.ExpandFrom` for refcount; tree-sitter outline for "removing decl" detection | `references_checked` OR `impact_checked` | Phase 65 `analyze_blast_radius` produces the same data; degraded fallback per D-19 |
| **G-003** public-API edit | `SemanticLookup.Visibility` (NEW per OI-02) + `IsEntrypointReachable` (NEW per OI-03) + interface-contract predicate | `impact_checked` (NOT `references_checked`) | Visibility populated in `internal/semantic/extract/fact.go` line 98; promote via `SemanticLookup` extension |
| **G-004** large-fuzzy-edit | LOC counting in tool args (`changed_lines`, `touched_files`); `file_line_count` from filesystem | `context_gathered` or `structural_overview` | Pure-Go arithmetic; no semantic dependency; works with `semantic_index.enabled=false` |
| **G-005** security-sensitive | 3-signal: path glob (config), identifier pattern (config), import catalog (embed.FS YAMLs per language) | `diagnostics_clean` (post-edit, ErrorCount==0) | `verify_edit` already runs diagnostics; wire receipt issuance into its existing post-edit hook (D-18); G-005 import detection uses `internal/kernel/edit/treesitter.go` import-extraction (verify exists at plan time) or grep fallback in degraded mode |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Client-side conversation policies (Claude Code's "approval prompts", Cursor's "review before apply") | Server-side capability receipts surviving compaction | Phase 66 (this phase) | Differentiator vs Copilot/Devin/Claude/Codex per `.planning/research/FEATURES.md` lines 63, 84, 122 — no published prior art (`.planning/research/SUMMARY.md` line 149) |
| Class-only receipt match (early v1.10 draft) | STRICT scope match per class (D-09) | Phase 66 CONTEXT.md | Closes a forge-by-substitution loophole; small false-positive cost on warn mode |
| Persistent receipts in DuckDB | In-memory `sync.Map` keyed by `graph_version` (D-06) | Phase 66 CONTEXT.md (forbidden by REQUIREMENTS.md line 167) | Threat model: server-side only; restart loses receipts (acceptable since TTL=5min) |
| Per-rule scattered booleans (`RequireReferencesBeforeRename: true`) | Per-rule + per-tool maps with closed-enum `EnforcementLevel` (D-22) | Phase 66 CONTEXT.md | Replaces the existing `GuardrailsConfig` flat-bool shape; old keys can be subsumed (back-compat is a non-issue per pre-production stance) |

**Deprecated/outdated:**
- `RequireImpactForPublicAPIEdit`, `RequireReferencesBeforeRename`, `RequireReferencesBeforeDelete`, `RequireVerifyAfterEdit` flat booleans in `GuardrailsConfig`: **subsumed** by per-rule `Rules[]RuleConfig` map. D-22 explicitly notes they "can be subsumed by per-rule entries." Plan should remove (or alias-deprecate with a TODO if any external doc references them — verify at plan time).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `github.com/google/uuid` v1.6+ supports UUIDv7 (`uuid.NewV7()`) | Standard Stack | Low — easy to verify at plan time; if absent, switch to `gofrs/uuid` or generate UUIDv7 manually (~30 LOC) |
| A2 | `internal/kernel/edit/treesitter.go` already extracts symbols per file in a way reusable for G-001's "is `find` a known symbol?" check | Per-Rule Detection table | Medium — if the extractor is private to `replace_symbol_body`, may need a small refactor to expose; verify in Wave 0 |
| A3 | `internal/semantic/extract/fact.go` `Visibility` field is populated for ALL languages with semantic indexing (Go/TS/JS/Java/Rust/Python/C#), not just a subset | Don't Hand-Roll table; OI-02 | Medium — if some languages report `Visibility = ""` or `"unknown"`, D-19 conservative-warn fallback applies but degrades G-003 effectiveness; researcher recommends an explicit per-language coverage matrix in Wave 0 (audit `internal/semantic/extract/<lang>.go` files) |
| A4 | The CI cardinality validator's enum registry is updatable in the same wave that adds new outcome values | Pitfall 7 | Low — verify by grep for the validator's enum source-of-truth (`internal/obs/registry.go` or similar) at plan time |
| A5 | Java / Rust / C# G-005 import-pattern catalogs are not required for v1 (recommend deferring) | "Specifics" + Open Questions | Medium — eval-corpus signal in Phase 67 will tell us; if the eval shows agents bypass G-005 for Java/Rust auth code, this becomes a v1.10.x hot-fix |
| A6 | Receipt-issuance from each read tool can be implemented as a single line in the success path (closure injection) without restructuring the tool handler | Pattern 4 | Low — `RecordEditOutcome` (Phase 53) is a working precedent; same shape applies |
| A7 | `verify_edit` (`internal/kernel/edit/verify.go` line 28 confirmed) already runs diagnostics on a post-edit URI, so G-005 post-edit `diagnostics_clean` issuance is purely additive | D-18 row | Low — verified file exists; need to inspect handler shape in Wave 0 |
| A8 | `get_tool_help` topic-registry refactor (OI-04) is a self-contained change in `internal/kernel/help/` and does not require any kernel-tool API change | Pattern 5 | Low — verified the existing tool only takes `tool_name`; adding optional `topic` arg is additive |
| A9 | Default G-005 catalogs (Go/TS/JS/Python) are domain-knowledge based, not verified against the actual ecosystem of security-sensitive packages | "Specifics" | Medium — initial catalog will have gaps; mitigated by user-override config key (D-22) and post-launch tuning |

## Open Questions

### OI-01 — GUARD-07 default-profile delta resolution

**What we know:**
- `internal/profile/profiles/ci-bot.yaml` (verified) already has `default_mode: review` and excludes destructive tools via `exclude_tools`. It carries NO guardrail-specific keys today. There is no `guardrails:` section in any profile YAML.
- D-11 proposes `ci-bot=enforce` and a NEW `security` profile=`enforce`.
- GUARD-07 in REQUIREMENTS.md only specifies "warn for read/edit, require_force for review."
- No `security` profile exists in `internal/profile/profiles/` (only `ci-bot.yaml`, `claude-code.yaml`, `codex.yaml`, `full.yaml`, `ide-assistant.yaml`).

**What's unclear:** Is `security` profile a new YAML to ship, or per-tool/per-rule overrides on an existing profile?

**Recommendation:**
1. **Ship `ci-bot=enforce` in v1.10** by adding a `guardrails: { enforcement: enforce }` block to `internal/profile/profiles/ci-bot.yaml`. Justification: ci-bot already excludes all destructive tools, so `enforce` is effectively a no-op for excluded tools but provides defense-in-depth if a future plan adds destructive tools to ci-bot.
2. **Defer `security` profile to a follow-up plan.** Reason: a new profile YAML carries skill-allowlists and mode transitions that need their own design pass; expressing "security audit mode" as `claude-code` profile + `review` mode + per-tool `enforce` overrides covers 90% of the use case via D-22's existing `tools:` override map.
3. **REQUIREMENTS.md amendment:** propose adding to GUARD-07: "ci-bot defaults to `enforce`; the `security` profile (if added in v1.10.x) defaults to `enforce`." Mark as `(amended Phase 66)`.

### OI-02 — Visibility classifier source

**What we know:**
- `internal/semantic/extract/fact.go` line 98 confirms `Visibility string` field on the symbol-fact stream with values `"exported|private|package|..."` `[VERIFIED]`.
- `SemanticLookup` interface in `internal/semantic/integ/lookup.go` (verified) does NOT currently expose visibility. Closest method is `LocateSymbol(SymbolID) → (path, line, col)`.
- Per-language coverage of `Visibility` population is unknown without a per-extractor audit.

**Recommendation:**
1. **Extend `SemanticLookup`** with a single new method:
   ```go
   Visibility(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (Visibility, error)
   ```
2. **Define `guardrails.Visibility` closed enum:** `Public | Exported | Protected | Package | Private | Unknown`. The lookup-side string `"exported"` maps to `Exported`; missing data maps to `Unknown` (triggers D-19 conservative-warn for G-003).
3. **`IsPublicLike()`** is a method on the closed enum:
   ```go
   func (v Visibility) IsPublicLike() bool {
       return v == Public || v == Exported || v == Protected
   }
   ```
4. **Wave 0 audit task:** for each of Go/TS/JS/Java/Rust/Python/C# extractor files, confirm `Visibility` is populated. If a language is missing, document in GUARDRAILS.md "Known limitations" and rely on D-19 fallback.

### OI-03 — Entry-point-reachable + interface-contract predicates

**What we know:**
- D-14 / D-15 reference `IsEntrypointReachable` and `ParticipatesInInterfaceContract`.
- SMTC's `find_entry_points` is **NOT applicable to this codebase** — it's a Java-gated security tool (verified in CLAUDE.md "Activation" + SMTC-TODO.md). Helix is Go.
- Phase 62 graph engine produces a call-graph projection; entry-point reachability would be `reverse-BFS from public entry symbols`.
- Interface participation requires `RESOLVES_TO` edges from Phase 62 TYPES-01.

**Recommendation:**
1. **Ship `IsEntrypointReachable` as a thin wrapper** in `SemanticLookup`:
   ```go
   IsEntrypointReachable(ctx, ws, sym SymbolID) (bool, error)
   ```
   Implementation: traverse the Phase 62 call-graph from `sym` upward looking for any symbol marked as an entry point. Entry-point classification heuristics (per language): Go `func main`, `func TestX`, exported-from-`main`-package symbols; TS/JS exported-from-entry-files; Java `@RestController` / `main`; Python `if __name__ == "__main__"` blocks.
2. **For v1.10, ship a CONSERVATIVE `IsEntrypointReachable`:** "is this symbol exported AND does its enclosing package look like an entry-point package (heuristic: `cmd/`, `main.go`, `app/`, has a `main` func)?" Returns `true` on uncertainty (defensive — false positives are warn/enforce events that an agent can resolve by gathering blast-radius).
3. **`ParticipatesInInterfaceContract`** — DEFER to a follow-up plan and rely on **conservative visibility check alone** per OI fallback. Reason: clean computation requires Phase 62 `RESOLVES_TO` edges + interface-set discovery, which is non-trivial to wire correctly. G-003 trigger logic should be: `IsPublicLike() OR IsEntrypointReachable() OR signature_change_with_refcount_gt_0`. Document the gap in GUARDRAILS.md "Known limitations" with a TODO referencing the follow-up.

### OI-04 — `get_tool_help` topic surface extension

**What we know:**
- `internal/kernel/help/tools.go` (verified) currently dispatches on `args.ToolName` and looks up in `server.Registry()` (which holds tools, not arbitrary topics).
- The `ToolDef` struct has `HelpText string` for inline help — no separate topic content path.
- No prefix-match logic exists today.

**Recommendation:**
1. **Add a `TopicRegistry`** sibling to `ToolRegistry` in `internal/kernel/help/`:
   ```go
   type Topic struct { Name string; Content string }
   type TopicRegistry struct { topics map[string]*Topic }
   ```
2. **Extend the tool's argument schema** to accept either `tool_name` or `topic` (validation: exactly one set).
3. **Register all 6 topics individually** at daemon start (no prefix dispatch v1):
   ```go
   topics.Register("guardrails", embeddedGuardrailsMD)
   topics.Register("dod", embeddedDoDMD)
   topics.Register("workflow:rename", embeddedWorkflowRenameMD)
   // ... 3 more
   ```
4. **Source content via `embed.FS`** in the new `internal/kernel/help/` package or `internal/guardrails/docs/`. The 4 workflow snippets are sections of DoD.md extracted at embed time (or hand-split for clarity).
5. **Forward-compat:** prefix dispatch (`workflow:*`) can land in v1.10.x if topic count grows; v1 ships with the explicit list.

### OI-05 — Receipt-issuance hook on read tools

**What we know:**
- Tested candidate: TelemetryMiddleware's `outcome=success` path classifies the tool's success but doesn't have access to typed scope data (it sees `mcpsdk.Result`, not the typed result struct).
- Existing precedent: `RecordEditOutcome` (Phase 53 D-16) — invoked from each edit tool's handler success path via a sink closure.

**Recommendation:**
1. **Per-tool closure injection (NOT wrapping middleware).** Reasons: (a) read-tool result types are heterogeneous; middleware-side scope extraction would re-marshal/unmarshal JSON; (b) only 7 tools issue receipts — explicit calls are cleaner than reflection; (c) mirrors the proven `RecordEditOutcome` pattern; (d) keeps middleware testable as a pure function.
2. **API:**
   ```go
   // In internal/guardrails/issue_sink.go (mirrors internal/mcp/middleware.go editOutcomeSink):
   var receiptIssueSink atomic.Pointer[ReceiptIssueFn]
   type ReceiptIssueFn func(ctx context.Context, class ReceiptClass, scope ReceiptScope, tool string) (ReceiptID, error)

   func SetReceiptIssueSink(fn ReceiptIssueFn) { /* atomic.Store */ }
   func IssueReceiptOnSuccess(ctx context.Context, class ReceiptClass, scope ReceiptScope, tool string) ReceiptID {
       // returns "" if sink unset (test mode); never errors out (logged).
   }
   ```
3. **Wiring:** daemon post-init calls `guardrails.SetReceiptIssueSink(store.Issue)`; each read-tool handler calls `IssueReceiptOnSuccess` on its success path. Receipt ID is OPTIONALLY echoed in the tool's response envelope as `_receipt_id` for debug — the agent does NOT need to read it (server forwards the ID to the destructive tool side via... wait — agent IS the carrier). Correction per D-07/D-08: receipt ID IS surfaced to the agent in the read-tool response (agent then echoes it on the destructive tool call), but the receipt BODY stays server-side. Document this asymmetry clearly.
4. **The hook fires AFTER the tool result is constructed but BEFORE `next` returns** — same place as the result-marshaling code in each read-tool handler. NEVER in a `defer` block.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All compilation | ✓ | go1.25.1 (host: darwin/arm64) `[VERIFIED]` | — |
| `github.com/modelcontextprotocol/go-sdk` | Middleware | ✓ (already in go.mod) | per existing | — |
| `github.com/google/uuid` | UUIDv7 receipt IDs | ? (verify in go.mod at plan time) | v1.6+ for `NewV7()` | Generate UUIDv7 manually (~30 LOC) or use `github.com/gofrs/uuid` |
| `github.com/knadh/koanf/v2` | Config layering | ✓ (already in go.mod per CLAUDE.md) | per existing | — |
| `embed.FS` (stdlib) | Bundle GUARDRAILS.md / DoD.md / catalogs | ✓ (Go ≥1.16) | go1.25.1 | — |
| Phase 62 graph engine + `graph_version` | Receipt invalidation | Pending Phase 62 completion | — | If Phase 62 ships before Phase 66, fully available. If parallel, Phase 66 stubs `GraphVersion()` to return `1` and accepts that receipts never auto-invalidate (only TTL-expire). Document as a v1.10 limitation with a TODO. |
| Phase 65 `SemanticLookup` seam | Visibility / blast-radius / freshness | Pending Phase 65 completion | — | D-19 degraded-warn fallback covers this |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** Phase 62 / 65 — handled by D-19 degraded-mode design.

## Validation Architecture

> Phase 66 has nyquist_validation enabled (`.planning/config.json` workflow.nyquist_validation = true) and TDD mode enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `go test ./...` (project standard per CLAUDE.md) |
| Config file | `go.mod` / `Makefile` (`make test`) |
| Quick run command | `go test ./internal/guardrails/... ./internal/mcp/... -run TestGuard -count=1` |
| Full suite command | `go test ./... -race -count=1` (or `make test`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GUARD-01 | LIFO middleware execution order = LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler | unit | `go test ./internal/mcp/ -run TestMiddlewareLIFOOrder_Phase66 -count=1` | ❌ Wave 0 — extend existing `internal/mcp/middleware_test.go` |
| GUARD-02 (G-001) | rename-by-grep on identifier-shaped find triggers warn/block when no `references_checked` receipt | unit | `go test ./internal/guardrails/rules/ -run TestG001 -count=1` | ❌ Wave 0 |
| GUARD-02 (G-002) | delete_file / safe_delete_symbol triggers without receipt | unit | `go test ./internal/guardrails/rules/ -run TestG002 -count=1` | ❌ Wave 0 |
| GUARD-02 (G-003) | public-API edit (visibility=Exported) requires `impact_checked` not `references_checked` | unit | `go test ./internal/guardrails/rules/ -run TestG003 -count=1` | ❌ Wave 0 |
| GUARD-02 (G-004) | changed_lines>50 OR touched_files>1 triggers; ratio trigger configurable | unit | `go test ./internal/guardrails/rules/ -run TestG004 -count=1` | ❌ Wave 0 |
| GUARD-02 (G-005) | path/identifier/import 3-signal triggers; post-edit `diagnostics_clean` issuance on ErrorCount==0 | unit | `go test ./internal/guardrails/rules/ -run TestG005 -count=1` | ❌ Wave 0 |
| GUARD-02 SC-2 | context-truncation E2E: agent's read-tool result deleted, fabricated `receipts: []` refused | integration (forwarder→daemon) | `go test ./test/harness/ -run TestContextTruncationE2E -count=1` | ❌ Wave 0 — extends `test/harness/Runner` (verify path at plan time) |
| GUARD-03 | receipt store: TTL=5min, in-memory, ID-only forwarding | unit | `go test ./internal/guardrails/ -run TestStore -count=1 -race` | ❌ Wave 0 |
| GUARD-04 | `graph_version` advance invalidates receipts | unit | `go test ./internal/guardrails/ -run TestStore_InvalidateOnGraphVersionAdvance -count=1` | ❌ Wave 0 |
| GUARD-05 | Telemetry classifies guardrail_warned / guardrail_blocked; `helix_receipt_*` counters increment | unit | `go test ./internal/mcp/ -run TestTelemetry_GuardrailOutcome -count=1` | ❌ Wave 0 — extend `telemetry_middleware_test.go` |
| GUARD-06 | GUARDRAILS.md and DoD.md exist + are reachable via `get_tool_help` | smoke | `go test ./internal/kernel/help/ -run TestTopicRegistry -count=1` AND file existence check | ❌ Wave 0 |
| GUARD-07 | enforcement levels resolve via 5-layer precedence; default warn for edit, enforce for review/ci-bot | unit | `go test ./internal/guardrails/ -run TestResolveGuardrailEnforcement -count=1` | ❌ Wave 0 |
| Receipt ID parser | `rcpt_<base32-26>` validates length+alphabet; reject `rcpt_*` w/ wrong length / padding | unit (table-driven) | `go test ./internal/guardrails/ -run TestParseReceiptID -count=1` | ❌ Wave 0 |
| Scope-match STRICT | symbol A receipt rejected for symbol B operation | unit | `go test ./internal/guardrails/ -run TestValidateScope_StrictMatch -count=1` | ❌ Wave 0 |
| 5-class exhaustiveness | `validateScope` switch covers every `ReceiptClass` value | unit (compile-time + runtime) | `go test ./internal/guardrails/ -run TestReceiptClassExhaustive -count=1` | ❌ Wave 0 |
| Cardinality validator | new `outcome` and `receipt_*` enums pass PromQL validator | CI | existing CI metric-cardinality test (verify path at plan time) | ✓ existing |
| Degraded mode | `semantic_index.enabled=false` → G-002/G-003 conservative warn; G-001/G-004/G-005 path+identifier still work | unit | `go test ./internal/guardrails/ -run TestDegradedMode -count=1` | ❌ Wave 0 |
| LRU eviction | 10k cap per workspace; oldest evicted | unit | `go test ./internal/guardrails/ -run TestStore_LRU -count=1` | ❌ Wave 0 |
| Janitor | 30s tick removes expired entries | unit (with fake clock) | `go test ./internal/guardrails/ -run TestStore_Janitor -count=1` | ❌ Wave 0 |
| Concurrent issue+lookup | race-free under `-race` | unit | `go test ./internal/guardrails/ -run TestStore_Concurrent -race -count=1` | ❌ Wave 0 |

**Test-tier summary:**
- **Pure business logic (TDD-friendly RED/GREEN):** receipt ID parsing, store TTL/LRU/janitor/invalidation, scope-match STRICT, 5-layer enforcement resolution, every per-rule predicate (G-001..G-005), visibility classification, response-envelope shape (D-12), config defaults.
- **Integration only:** middleware install order (needs real `mcpsdk.Server`), context-truncation E2E (needs forwarder+daemon process pair), `verify_edit` post-edit receipt issuance (needs LSP fixture).
- **Smoke / file existence:** GUARDRAILS.md, DoD.md, `get_tool_help` topic dispatch.

### Sampling Rate
- **Per task commit:** `go test ./internal/guardrails/... ./internal/mcp/... -count=1` (target ~5s)
- **Per wave merge:** `go test ./... -race -count=1` (full suite; baseline ~60-90s per CLAUDE.md `make test`)
- **Phase gate:** Full suite green + context-truncation E2E + `go vet ./...` before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/guardrails/` package directory (entire subtree)
- [ ] `internal/guardrails/store_test.go` — TTL, LRU, graph_version invalidation, concurrent
- [ ] `internal/guardrails/validate_test.go` — STRICT scope match, all error sentinels
- [ ] `internal/guardrails/enforcement_test.go` — 5-layer precedence (D-20)
- [ ] `internal/guardrails/rules/g00*_test.go` — one file per rule, table-driven
- [ ] `internal/mcp/guardrail_middleware_test.go` — install order + warn/block dispatch
- [ ] `internal/mcp/middleware_test.go` extension — 5-step LIFO order assertion
- [ ] `test/harness/context_truncation_test.go` — forwarder+daemon E2E (verify harness exists at plan time; if not, this becomes a separate wave dependency)
- [ ] No new framework install required — Go stdlib `testing` already used.

## Security Domain

> Phase 66 IS the security feature. Below is ASVS mapping for the design itself.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No auth surface — receipts are capability tokens scoped to a single daemon-process workspace, not user/agent credentials |
| V3 Session Management | yes (lightly) | Receipts ARE session-scoped capability tokens (5-min TTL). Standard: opaque IDs (D-07 base32-26), server-side only (D-06), keyed by workspace + graph_version |
| V4 Access Control | yes | Receipts gate destructive operations on prior read evidence. STRICT scope-match per D-09. Closed-enum `EnforcementLevel` per D-21 |
| V5 Input Validation | yes | `ParseReceiptID` validates prefix + length + alphabet (D-07); receipt class is closed enum; tool argument struct enforces typed `Receipts []ReceiptID` field (D-08) |
| V6 Cryptography | yes (lightly) | Receipt ID entropy from `crypto/rand` via UUIDv7 — never `math/rand`. Receipts are NOT signed (server-only storage means signing adds no security; the bearer-token property is contained to a single daemon process) |
| V7 Error Handling | yes | Typed `serr.GuardrailViolation` (Phase 22 taxonomy) carries structured fields, never raw error text; D-12 envelope schema |
| V8 Data Protection | yes | Receipts contain symbol IDs and file paths (NOT source content); per CLAUDE.md "no source content in metric labels" — same discipline applies to receipts and to error envelopes |
| V11 Business Logic | yes | This IS business logic — see "Common Pitfalls" for forge-by-substitution / replay / scope-confusion threats |

### Known Threat Patterns for guardrail-receipt design

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Receipt forge by class-only match (find_references on harmless symbol → rename auth code) | Spoofing / Tampering | STRICT scope match per D-09 — `validateScope` switch with typed Go union |
| Receipt replay across graph versions (rename, then rename again with same receipt) | Tampering | `graph_version` advance invalidates receipts (D-04, GUARD-04) + opportunistic single-use deletion on consume |
| Cross-workspace receipt forgery (multi-workspace daemon) | Spoofing | `WorkspaceKey` field on receipt; validator checks `receipt.WorkspaceKey == current_workspace` (`ErrWrongWorkspace`) |
| Context-compaction-driven fabrication (agent invents a receipt ID) | Spoofing | ID-only forwarding (GUARD-03) — agent never sees the body, server only validates by lookup; opaque base32-26 IDs (D-07) make brute-force collision astronomically unlikely (130 bits of search space — see UUIDv7 spec) |
| Receipt enumeration via timing | Information Disclosure | Constant-time lookup via `sync.Map` (no early returns leaking class) |
| TTL bypass via clock skew | Tampering | `ExpiresAt` is server-side; agent has no clock authority |
| Resource exhaustion via receipt issuance flood | DoS | LRU cap at 10k per workspace (D-06); janitor sweeps 30s; cardinality bounded |
| `require_force` mode bypass | Elevation of Privilege | Override surface NOT shipped this phase; `require_force` behaves as `enforce` until override transport lands (Pitfall 6) — fail-closed |
| Source content leak in error / metric labels | Information Disclosure | Bounded-label discipline (Phase 47/53); error envelopes carry symbol IDs + file paths only, never source content (CLAUDE.md invariant) |

## Project Constraints (from CLAUDE.md)

- **Go single binary**, no Python/Docker/runtime deps. → Receipts in-memory; markdown via `embed.FS`.
- **`make test` and `go vet ./...` MUST pass before completing any Go task.**
- **MCP-primary interface.** → Receipt + middleware design surfaces only via MCP tool args + tool envelope; no separate API.
- **GSD workflow enforcement.** → Edits go through this RESEARCH → PLAN → EXECUTE flow; no direct edits.
- **SMTC-first tool routing for code-aware operations.** → Researcher used SMTC tools where applicable (verified read tools, file outlines); used `grep`/`Read` only for non-code (markdown, YAML, configs) and identifier-substring searches.
- **Middleware install LIFO order is a must-not-regress invariant** (`v1.10-ROADMAP.md` line 255). → GUARD-01 regression test mandatory.
- **Bounded-label metrics, no source content in labels.** → Closed enums for `outcome`, `class`, `lookup_outcome`, `reason`.
- **Single canonical `GrammarRegistry`** (BUG-04, Phase 49). → G-001 reuses the existing extractor — does NOT instantiate a second registry.
- **CGO_ENABLED=1 single-mode (Phase 59.1).** → No CGO branching needed for guardrails.

## Sources

### Primary (HIGH confidence)
- `[VERIFIED]` `internal/mcp/middleware.go` lines 99-258 — `InstallMiddleware`, `outcomeEnum` closed list, `classifyOutcome` switch
- `[VERIFIED]` `internal/mcp/lazy_init.go` lines 106-112 — LazyInit-last invariant
- `[VERIFIED]` `internal/mcp/suggest.go` lines 253-256 — `InstallSuggestionMiddleware` shape
- `[VERIFIED]` `internal/daemon/daemon.go` lines 698, 706, 729 — actual install sites for steps 14, 14b, 14c
- `[VERIFIED]` `internal/semantic/config.go` lines 269-279 — existing `GuardrailsConfig` struct (D-22 extends)
- `[VERIFIED]` `internal/semantic/integ/lookup.go` — `SemanticLookup` interface (OI-02 extension target)
- `[VERIFIED]` `internal/semantic/integ/source.go` — closed-enum + sentinel error pattern to mirror
- `[VERIFIED]` `internal/semantic/integ/status.go` — `SemanticStatus.GraphVersion` (GUARD-04 source)
- `[VERIFIED]` `internal/semantic/integ/envelope.go` — envelope shape with `freshness`, `graph_version`
- `[VERIFIED]` `internal/semantic/extract/fact.go` line 98 — `Visibility` field (OI-02 source)
- `[VERIFIED]` `internal/errors/kinds.go` — typed-error `Kind` taxonomy (extend with `guardrail_violation`)
- `[VERIFIED]` `internal/kernel/help/tools.go` lines 21-71, `skill_adapter.go` — tool dispatch shape (OI-04)
- `[VERIFIED]` `internal/profile/profiles/ci-bot.yaml` — current ci-bot profile (OI-01 source)
- `[VERIFIED]` `internal/profile/profile.go` — Profile struct shape
- `[VERIFIED]` `internal/mcp/registry.go` — `ToolDef` / `ToolRegistry` shape
- `[CITED]` `.planning/REQUIREMENTS.md` lines 98-106 — GUARD-01..GUARD-07 verbatim
- `[CITED]` `.planning/REQUIREMENTS.md` line 143 — G-006..G-010 deferral
- `[CITED]` `.planning/REQUIREMENTS.md` line 167 — persistent receipts forbidden
- `[CITED]` `.planning/milestones/v1.10-ROADMAP.md` lines 176-186 — Phase 66 success criteria
- `[CITED]` `.planning/milestones/v1.10-ROADMAP.md` line 255 — middleware install LIFO must-not-regress
- `[CITED]` `.planning/phases/66-agent-guardrails/66-CONTEXT.md` — D-01..D-24, OI-01..OI-05
- `[CITED]` `CLAUDE.md` — Helix architecture, middleware ordering, bounded-label discipline

### Secondary (MEDIUM confidence)
- `[ASSUMED]` Go stdlib `sync.Map` semantics (idiomatic for read-mostly per-key independent workloads) — well-established Go pattern
- `[ASSUMED]` UUIDv7 specification (RFC 9562, Time-ordered + monotonic + 130 bits entropy)
- `[ASSUMED]` `github.com/google/uuid` v1.6 supports `NewV7()` — verify at plan time

### Tertiary (LOW confidence — flagged for validation)
- `[ASSUMED]` G-005 default catalogs (Go/TS/JS/Python security-sensitive imports) — domain-knowledge based, will need eval-corpus tuning post-launch
- `[ASSUMED]` Java/Rust/C# G-005 catalogs not required for v1 (recommend deferring) — eval-corpus signal will validate
- `[ASSUMED]` `IsEntrypointReachable` heuristic (Go: `cmd/`, `main.go`, `func main`; TS/JS: entry-file exports; Java: `@RestController`/`main`; Python: `__main__`) — needs per-language Wave 0 validation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every library is in-tree and verified
- Architecture (middleware order, receipt store, validation predicate): HIGH — design follows 4 existing precedent patterns (LazyInit invariant, RecordEditOutcome closure, closed-enum metrics, koanf config)
- Per-rule detection (D-13..D-18): HIGH for G-001/G-004; MEDIUM for G-002/G-003 (depends on OI-02/OI-03 resolution); MEDIUM for G-005 (catalog accuracy)
- Open Items resolution (OI-01..OI-05): MEDIUM — recommendations are evidence-backed but the user may have alternative preferences; flagged for `/gsd-discuss-phase` confirmation
- G-005 catalog completeness: LOW — domain-knowledge based, will need eval-corpus signal to tune

**Research date:** 2026-05-09
**Valid until:** 2026-06-08 (30 days; design is stable, but Phase 62/65 completion may surface new seams worth using)

---

*Phase: 66-Agent-Guardrails*
*Researcher: gsd-researcher agent*
*Consumer: gsd-planner agent*
