# Phase 66: Agent Guardrails (G-001..G-005, warn-default) - Context

**Gathered:** 2026-05-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 66 ships **server-side semantic safety guardrails** for destructive MCP
tools. Five rules (G-001..G-005) gate destructive operations on prior
evidence collected through read-side tools, surviving agent context
compaction via ID-only forwarding to a server-side receipt store.

**Phase 66 ships:**

1. **`internal/guardrails/`** — receipt store (in-memory sync.Map per
   workspace, janitor-evicted), receipt schema (typed-union scope per
   class), `EnforcementLevel` enum, 5-layer config precedence resolver,
   per-rule predicates, language-aware import-pattern catalog for G-005,
   visibility-aware symbol classifier for G-002/G-003.

2. **`internal/mcp/guardrail_middleware.go`** — `GuardrailMiddleware`
   installed at daemon step 14b.5 (between `SuggestionMiddleware` and
   `LazyInitMiddleware`). Final LIFO execution order:
   `LazyInit → Guardrail → Suggestion → ProfileFilter → Telemetry → handler`
   (LazyInit-last-installed/executes-first invariant preserved and
   regression-asserted).

3. **Receipt-issuing wiring on 4 read tools + 3 diagnostics tools** —
   `find_references`, `analyze_blast_radius`, `get_context` /
   `get_semantic_context`, `get_repo_map` issue receipts of classes
   `references_checked` / `impact_checked` / `context_gathered` /
   `structural_overview`. `get_diagnostics`, `verify_edit`,
   `run_diagnostics` issue `diagnostics_clean` post-edit when ErrorCount==0.

4. **Receipt-consuming wiring on every destructive tool** — uniform
   `receipts: []ReceiptID` field added to: `rename_symbol`,
   `safe_delete_symbol`, `replace_symbol_body`, `fuzzy_edit`,
   `replace_in_file`, `delete_file`. Schema-day-one (no opt-in flag);
   missing/null normalized to `[]`.

5. **Telemetry classes** `guardrail_warned` / `guardrail_blocked` added to
   `TelemetryMiddleware` outcome taxonomy alongside existing
   `success / timeout / circuit_open / internal`. New
   `helix_receipt_issued_total`, `helix_receipt_expired_total`,
   `helix_receipt_lookup_total{outcome}` bounded-label counters.

6. **`GUARDRAILS.md`** + **`DoD.md`** at repo root, surfaced at runtime
   via new `get_tool_help` topics (`guardrails`, `dod`,
   `workflow:rename`, `workflow:delete`, `workflow:large-edit`,
   `workflow:security-sensitive-edit`).

7. **Integration test for context-truncation scenario** — agent's prior
   read-tool results are deleted from context; the destructive tool
   refuses (in enforce mode) without a server-side receipt match,
   proving ID-only forwarding survives compaction. Mandated by GUARD-02
   SC-2.

**Out of scope (deferred to later phases):**

- **G-006..G-010** (prefer-symbol-edits, no-stale-impact,
  no-false-certainty, multi-file-verify, generated-files) — explicitly
  deferred per `REQUIREMENTS.md` line 143.
- **Persistent receipts across sessions / disk-backed receipt store** —
  forbidden per REQUIREMENTS.md line 167 ("Persistent receipts detached
  from `graph_version` are explicitly forbidden") and matches the
  in-memory storage decision below (D-09).
- **Force-override workflow + audit-event format** — `require_force`
  mode is in the enum but the override surface (`force: bool` /
  `override_reason: string`) is *not* shipped this phase. Deferred to a
  follow-up plan once enforce-mode false-positive rate is measured.
- **Eval-harness integration** (`semantic_guarded` mode that proves
  guardrails reduce agent footguns) — Phase 67.
- **`semantic` profile / `security` profile catalog entries** — see D-22
  delta below; researcher must resolve whether to add these to the
  profile catalog or absorb them into existing profiles.

</domain>

<decisions>
## Implementation Decisions

### Receipt Schema + Storage

- **D-01 — Receipt-issuing tools (medium issuer set):** Four read-side
  tools issue receipts: `find_references` →
  `references_checked{symbol_id, ref_count, file_path?, include_tests}`;
  `analyze_blast_radius` →
  `impact_checked{symbol_id, ref_count, public_api, blast_nodes, max_depth, included_callers, included_types}`;
  `get_context` / `get_semantic_context` →
  `context_gathered{file_set, target_symbols, task_hash, token_budget_used, max_tokens}`;
  `get_repo_map` →
  `structural_overview{root_path, depth, file_count, max_tokens}`. Plus
  three post-edit tools issue the 5th class (D-15):
  `get_diagnostics` / `verify_edit` / `run_diagnostics` →
  `diagnostics_clean{file_set, diagnostic_count, error_count, warning_count, tool}`.
  Reasoning: narrow set (just find_references + analyze_blast_radius)
  excludes file/context-centered destructive ops; wide set inflates the
  taxonomy with reads that don't actually justify any destructive op.

- **D-02 — Receipt common fields:** Every receipt carries
  `id, class, schema_version, workspace_key, snapshot_id, graph_version,
  freshness, score_status, cluster_status, pending_lsp_files,
  lsp_coverage, issued_at, expires_at, issuing_tool, issuing_call_id,
  trace_id, scope`. Freshness snapshot included so destructive tools can
  refuse stale-graph evidence (matches the freshness-as-API doctrine).

- **D-03 — Scope shape: typed Go union (not `map[string]any`).** Closed
  enum `ReceiptClass` + per-class scope struct
  (`ReferencesCheckedScope`, `ImpactCheckedScope`, `ContextGatheredScope`,
  `StructuralOverviewScope`, `DiagnosticsCleanScope`). Reason: opaque
  blob makes destructive-tool validation stringly-typed; explicit Go
  types let the validator be a switch with compile-time exhaustiveness.

- **D-04 — Default TTL: 5 minutes (`expires_at = issued_at + 5m`).**
  Matches GUARD-03 verbatim. Configurable via
  `semantic_index.guardrails.receipt_ttl` (already wired through koanf
  per existing `GuardrailsConfig`).

- **D-05 — Validation predicate `ValidateReceiptForOperation`:** matches
  on workspace + graph_version + not-expired + freshness-allowed +
  class match + STRICT scope match (D-07). Returns typed error sentinels
  `ErrWrongWorkspace`, `ErrGraphVersionMismatch`, `ErrReceiptExpired`,
  `ErrReceiptTooStale`, `ErrWrongReceiptClass`, `ErrReceiptScopeMismatch`.

- **D-06 — Storage: in-memory sync.Map per workspace.** RWMutex,
  background janitor goroutine sweeps expired entries every 30s, plus
  opportunistic-on-lookup expiry check, max 10k receipts per workspace
  with LRU eviction. Lost on daemon restart — acceptable since TTL=5min
  and graph_version invalidates on every committed change. Zero
  DuckDB coupling; works with `semantic_index.enabled=false`.

- **D-07 — Receipt ID shape: `rcpt_<UUIDv7-base32-26char>`.** Generated
  via `crypto/rand` + UUIDv7 timestamp prefix; opaque to agent (no
  semantic content to reconstruct claim client-side); time-ordered for
  janitor + log readability. `ParseReceiptID` validates prefix +
  length-26 + base32-no-padding alphabet.

### Tool Argument Surface

- **D-08 — Uniform `receipts: []ReceiptID` field on every destructive
  tool.** Same field name across `rename_symbol`, `safe_delete_symbol`,
  `replace_symbol_body`, `fuzzy_edit`, `replace_in_file`, `delete_file`.
  Middleware extracts uniformly without per-tool reflection. Array
  because some rules (G-005) require multiple receipt classes; empty
  array = no receipts presented. ID-only forwarding (GUARD-03 verbatim).

- **D-09 — STRICT scope-match per class (no class-only loophole).**
  References/impact receipts require `scope.SymbolID == target.SymbolID`;
  context-gathered receipts require
  `FileSetContainsAll(scope.FileSet, target.TouchedFiles)`;
  structural-overview receipts require
  `PathIsWithin(target.Path, scope.RootPath)`;
  diagnostics-clean requires
  `FileSetContainsAll(scope.FileSet, target.SecuritySensitiveTouchedFiles) AND scope.ErrorCount == 0`.
  Rejected class-only matching because it lets an agent run
  `find_references` on `HarmlessSymbol` then rename `AuthMiddleware` —
  exactly the loophole receipts are supposed to prevent.

- **D-10 — Schema-day-one rollout (no opt-in flag).** `receipts` field
  present on every destructive tool from the first commit; missing/null
  normalized to `[]`. Reason: pre-production codebase, no compat
  burden; teaches agents the contract immediately.

- **D-11 — Profile-defaulted enforcement (extends GUARD-07).** `warn`
  for `claude-code` / `codex` / `ide-assistant` / `edit` profiles;
  `enforce` for `review` / `ci-bot` profiles. **Delta from GUARD-07
  (REQUIREMENTS.md):** GUARD-07 only specifies "warn for read/edit,
  require_force for review". The user chose to extend defaults to
  `ci-bot=enforce` and proposed a new `security` profile=enforce. This
  is a delta the researcher MUST address (see "Open Items for Researcher"
  below).

- **D-12 — Response envelopes:** Warn-mode response is `ok:true` with
  `warnings: [{kind:"guardrail_warning", rule, message, suggested_tools, see_also}]`.
  Enforce-mode response is `ok:false` with
  `error: {kind:"guardrail_violation", rule, message, required_receipts, suggested_tools, see_also}`,
  using the existing typed-error taxonomy (Phase 22 `serr.Kind`).
  `see_also` always points to a `get_tool_help` invocation (D-23).

### Per-Rule Detection

- **D-13 — G-001 (rename-by-grep) trigger:** `fuzzy_edit` OR
  `replace_in_file` when the `find` argument matches the identifier
  regex `^[A-Za-z_][A-Za-z0-9_]{2,}$` AND the `find` string equals or
  is a substring of a known symbol name in the current file's
  tree-sitter outline. `rename_symbol` itself is exempt — LSP-native
  rename is the safe path. Required receipt: `references_checked` OR
  `impact_checked` for the matched symbol.

- **D-14 — G-002 (delete-without-refs) triggers:**
  `delete_file` (tracked source file always);
  `safe_delete_symbol` (always);
  `replace_symbol_body` (when `refcount>0` OR caller>0 OR public OR
  entry-point-reachable);
  `fuzzy_edit` / `replace_in_file` (when removing symbol declaration
  or body, or deleting a large block containing symbols).
  Required receipt: `references_checked` OR `impact_checked` (strict
  symbol scope); for `delete_file` with exported symbols also require
  `impact_checked` for those symbols (fallback `context_gathered` for
  fully-private files).

- **D-15 — G-003 (public-API edit) triggers:** target symbol is
  `IsPublicLike()` (Public OR Exported OR Protected per the closed
  `Visibility` enum) OR is entry-point-reachable OR participates in
  interface/type contract OR has external-package references OR the
  edit changes signature with `refcount>0`. Required receipt:
  `impact_checked` (NOT `references_checked` — public API edits need
  blast-radius). Visibility enum is language-aware: Go uppercase, TS
  export, Java public, Rust `pub`/`pub(crate)`, Python `__all__` /
  no-leading-underscore, C# public/protected.

- **D-16 — G-004 (large-fuzzy-edit) thresholds:** `changed_lines
  (inserted+deleted) > 50` OR `touched_files > 1`. Optional secondary
  trigger `changed_lines / file_line_count > 0.30` (configurable via
  `guardrails.G-004.enable_ratio_trigger`, default `true`). Config keys:
  `guardrails.G-004.{max_changed_lines:50, max_files:1, max_file_change_ratio:0.30, enable_ratio_trigger:true}`.
  Required receipt: `context_gathered` for single-file large;
  `structural_overview` for multi-file/module; both for very broad
  rewrites.

- **D-17 — G-005 (security-sensitive) detection: 3-signal.** ANY of:
  (1) edited file path matches security-sensitive path glob;
  (2) touched symbol/identifier matches security-sensitive pattern;
  (3) edited file imports security-sensitive package/module
  (language-aware import-pattern catalog) OR a changed import
  adds/removes such a package. Default catalogs ship for `go`,
  `typescript`, `javascript`, `python` (full lists in
  `66-DISCUSS-CHECKPOINT.json` `per_rule_detection.G-005_security_sensitive`).
  Import matching rules: exact, wildcard suffix (`crypto/*`), package
  prefix (`passport-*`), scoped wildcard (`@auth/*`), Python dotted
  prefix, Go module prefix.

- **D-18 — G-005 timing: post-edit validation.** Pre-edit: classify +
  warn/enforce per mode. Apply edit + update graph/overlay. Post-edit:
  run diagnostics → if ErrorCount==0, issue `diagnostics_clean` receipt
  AND auto-attach to the edit's outcome envelope; if not clean, return
  guardrail violation/warning. `verify_edit` already runs diagnostics —
  wire receipt issuance into its existing post-edit hook.

- **D-19 — Degraded mode when `semantic_index.enabled=false`:** G-002
  and G-003 fall back to **conservative warn** (NOT silent no-op) for
  source-file deletes, replace_symbol_body / rename_symbol, and
  exported-looking-name edits. G-001 still works (tree-sitter outline
  is independent of semantic index). G-004 still works (LOC counting
  needs no semantic data). G-005 path-glob + identifier-pattern signals
  still work; import-pattern signal degrades to grep-of-imports per
  language.

### Override Precedence + Documentation

- **D-20 — 5-layer enforcement-level precedence:**
  `CLI > per-tool > per-rule > profile > global default`.
  Resolution function `ResolveGuardrailEnforcement(cli, cfg, profile, tool, rule)`
  returns the **highest-precedence layer that is set** — NOT the most
  restrictive value. This means
  `guardrails.enforcement: enforce` + `guardrails.rules.G-001.enforcement: warn`
  resolves G-001 to `warn`. Documented explicitly to avoid the common
  "most-restrictive wins" misreading.

- **D-21 — `EnforcementLevel` closed enum:**
  `off | warn | enforce | require_force`. `off` skips evaluation;
  `warn` evaluates and emits warning but allows tool; `enforce` refuses
  unless required receipts are satisfied; `require_force` refuses
  unless receipts satisfied OR explicit force override is present (the
  override surface itself is OUT OF SCOPE this phase — see Open Items).

- **D-22 — Config shape:**
  ```yaml
  guardrails:
    enforcement: warn       # global default
    receipt_ttl: 5m
    rules:
      G-001: { enforcement: warn }
      G-002: { enforcement: enforce }
      G-003: { enforcement: enforce }
      G-004: { enforcement: warn }
      G-005: { enforcement: enforce }
    tools:
      rename_symbol: { enforcement: enforce }
      safe_delete_symbol: { enforcement: enforce }
      replace_symbol_body: { enforcement: enforce }
      fuzzy_edit: { enforcement: warn }
      replace_in_file: { enforcement: warn }
  profiles:
    edit:    { guardrails: { enforcement: warn } }
    review:  { guardrails: { enforcement: enforce } }
    ci-bot:  { guardrails: { enforcement: enforce } }
  ```
  This shape extends the existing `GuardrailsConfig` struct in
  `internal/semantic/config.go`. The current struct only has flat
  `enforcement` + per-flag booleans; per-rule + per-tool maps need to
  be added (and the `RequireImpactForPublicAPIEdit`-style booleans can
  be subsumed by per-rule entries).

- **D-23 — Two docs, two audiences.**
  - **`./GUARDRAILS.md`** (repo root): operator/user-facing — per-rule
    semantics (trigger, required receipt class, examples of valid &
    invalid call sequences), full config reference (precedence, enum,
    defaults, every key with example), runbook ("my agent is being
    blocked, what now?"), telemetry interpretation, profile defaults
    table.
  - **`./DoD.md`** (repo root): agent-consumable — per-task-class
    checklists for `rename`, `delete`, `public-API-change`,
    `large-edit`, `security-sensitive-edit`. Each lists the canonical
    tool sequence (e.g., `find_references` → receipt issued →
    `rename_symbol{receipts:[...]}` → `verify_edit`). Designed so an
    agent loading DoD.md as system-context knows exactly what sequence
    to run.

- **D-24 — Runtime doc discovery via `get_tool_help`.** New topics:
  `guardrails` → returns GUARDRAILS.md; `dod` → full DoD.md;
  `workflow:rename` / `workflow:delete` / `workflow:large-edit` /
  `workflow:security-sensitive-edit` → just the relevant DoD section.
  Every guardrail violation includes
  `see_also: [{tool: "get_tool_help", args: {topic: "workflow:<name>"}}]`
  for self-recovery.

### Open Items for Researcher

- **OI-01 — GUARD-07 default-profile delta.** D-11 extends GUARD-07's
  stated defaults (`warn` read/edit + `require_force` review) to also
  set `ci-bot=enforce` and proposes a new `security` profile=enforce.
  Researcher must: (a) verify whether `ci-bot` profile already has any
  guardrail-relevant policy in `internal/profile/`; (b) decide whether
  the proposed `security` profile is a new entry or can be expressed
  via per-tool/per-rule overrides on an existing profile; (c) propose
  a REQUIREMENTS.md amendment if the user's defaults ship.

- **OI-02 — Visibility classifier source.** D-15 needs a per-language
  `Visibility` lookup. Phase 62 type-resolver and Phase 59 tree-sitter
  extractors both produce identifier metadata; researcher must locate
  the cleanest existing seam (likely
  `internal/semantic/integ.SemanticLookup` extension) and confirm it
  exposes visibility for Go/TS/JS/Java/Rust/Python/C#. If a language
  is missing visibility data, document the degraded behavior (D-19
  fallback applies).

- **OI-03 — Entry-point-reachable + interface-contract predicates.**
  D-14 and D-15 reference `IsEntrypointReachable` and
  `ParticipatesInInterfaceContract`. Researcher must check which of
  these are already computable from the Phase 62 graph (entry points
  per `find_entry_points` are SMTC-side; interface participation is
  type-resolver-side) and propose the API on `SemanticLookup`. If
  not computable in this phase's scope, fall back to the conservative
  visibility check alone and document the gap.

- **OI-04 — `get_tool_help` topic surface extension.** D-24 adds 6
  topics. Researcher must verify the existing `internal/kernel/help/`
  topic registry shape and whether it supports prefix-matched topics
  (`workflow:*`) or needs a new dispatch shape.

- **OI-05 — Receipt-issuance threading on read tools.** Receipts must
  be issued AFTER the read succeeds and BEFORE the response is
  serialized. Researcher must locate the cleanest hook (likely a
  post-execution callback in the tool's handler, or a wrapping
  middleware that observes successful read-tool returns) so that
  every read-tool implementation doesn't have to know about receipts.
  Telemetry middleware's outcome classification (success path) is one
  candidate hook.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 66 Charter & Requirements
- `.planning/REQUIREMENTS.md` lines 98-106 — GUARD-01..GUARD-07 verbatim text.
- `.planning/REQUIREMENTS.md` line 143 — explicitly defers G-006..G-010 (out of scope).
- `.planning/REQUIREMENTS.md` line 167 — explicitly forbids persistent receipts detached from `graph_version`.
- `.planning/ROADMAP.md` Phase 66 entry (line ~find via grep) — "Agent Guardrails (G-001..G-005, warn-default)".
- `.planning/milestones/v1.10-ROADMAP.md` lines 176-186 — Phase 66 goal, depends-on (62, 65), 5 success criteria.
- `.planning/milestones/v1.10-ROADMAP.md` line 255 — middleware install LIFO order invariant ("LazyInit MUST remain installed last; Guardrail inserts at 14b.5 between Suggestion and LazyInit").

### Research Foundations
- `.planning/research/SUMMARY.md` lines 149-153 — P9 Guardrails section: scope, deliverables, "Avoids: M4", "no published prior art" research flag.
- `.planning/research/SUMMARY.md` line 201 — open research flag: "Receipt TTL × `graph_version` invalidation interplay" needs prototype.
- `.planning/research/PITFALLS.md` lines 238-250 — M4 "Receipts don't survive context compaction": exact prevention plan + integration test mandate.
- `.planning/research/FEATURES.md` lines 63, 84, 122 — guardrail+receipt as differentiator vs Copilot/Devin/Claude Code/Codex; persistent-receipt interplay.

### Architectural Anchors (Existing Code)
- `internal/mcp/middleware.go` lines 99-119 — `InstallMiddleware` signature; `TelemetryMiddleware`+`ProfileFilterMiddleware` install (step 14).
- `internal/mcp/suggest.go` line 253 — `InstallSuggestionMiddleware` install (step 14b).
- `internal/mcp/lazy_init.go` lines 106-109 — `InstallLazyInitMiddleware` MUST be installed LAST (step 14c) to run FIRST in LIFO order. **Hard invariant** for D-08 install at 14b.5.
- `internal/daemon/daemon.go` lines 698, 706, 729 — actual install sites for the three existing middleware steps; Phase 66 inserts new step 14b.5 between 706 and 729.
- `internal/semantic/config.go` `GuardrailsConfig` struct — existing config skeleton; D-22 extends with `rules` and `tools` maps.
- `internal/config/defaults.go` `semantic_index.guardrails.*` keys — existing defaults; per-rule + per-tool keys to be added.

### Cross-Phase Dependencies
- Phase 62 — graph engine + `graph_version` advance + `freshness` semantics (D-01 receipt fields, D-04 invalidation, D-05 validation predicate).
- Phase 65 — `internal/semantic/integ.SemanticLookup` seam (OI-02, OI-03 — extending this seam with visibility / entry-point / interface-contract APIs).
- Phase 64 — `get_semantic_context` tool (D-01 issuer); confidence/evidence envelope shape mirrored in receipt scope.
- Phase 22/47/53 — typed-error taxonomy (`internal/errors`) and existing bounded-label outcome counters (`helix_edit_outcome_total`, `helix_rename_strategy_total`); Phase 66 adds `guardrail_warned` / `guardrail_blocked` outcomes and `helix_receipt_*` counters using the same patterns.

### Helix Project Discipline
- `CLAUDE.md` — project conventions (Go-only, MCP-primary, layered config precedence, fuzzy-edit ambiguity-refusal pattern).
- `.planning/PROJECT.md` line 155 — "Agent guardrails (G-001..G-010) — policy engine, safety receipts, GUARDRAILS.md + DoD.md, profile/mode-gated enforcement" charter line.

### Memory Anchors (User Preferences)
- `~/.claude/projects/.../memory/feedback_no_ci_benchmarks.md` — no CI benchmarks (relevant if researcher proposes a CI receipt-throughput bench).
- `~/.claude/projects/.../memory/feedback_uat_no_manual_mcp.md` — UAT must automate MCP/daemon checks; the GUARD-02 SC-2 context-truncation integration test must drive forwarder + daemon end-to-end without manual MCP invocations.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`internal/mcp/middleware.go` install pattern** — three existing
  middleware install functions follow a uniform shape:
  `Install*(server *mcpsdk.Server, ...deps, logger *slog.Logger)`. New
  `InstallGuardrailMiddleware` follows the same shape; daemon.go
  inserts a new `helixMCP.InstallGuardrailMiddleware(...)` call at the
  unique step 14b.5 between line 706 and line 729.
- **`internal/mcp/middleware.go` outcome classification** —
  `TelemetryMiddleware` already classifies outcomes (`success / timeout /
  circuit_open / internal`). Adding `guardrail_warned` / `guardrail_blocked`
  as new outcome strings is a small, well-pattern-fit extension. Use the
  same closed-enum + bounded-label discipline (Phase 47 D-07).
- **`internal/semantic/integ.SemanticLookup`** — read-only seam already
  exposes `GraphVersion`, `ExpandFrom`, `LocateSymbol`, `RankFiles`. D-05
  (`ValidateReceiptForOperation`) and D-15 visibility predicates extend
  this seam; the lookup-by-callback inversion pattern from Phase 65
  applies here (`SetGuardrailLookup(...)` skill-side setter).
- **`internal/kernel/help/skill_adapter.go`** — `get_tool_help` is
  already a kernel-resident tool wrapped as a skill. New topics (D-24)
  plug into the existing topic registry; no new tool needed.
- **`internal/errors` typed-error taxonomy (Phase 22)** — `serr.Kind`
  already supports `InvalidArgs`, `Unsupported`, `NotFound`. Define
  guardrail violation as a new sentinel
  (`serr.NewGuardrailViolation(rule, message, suggested_tools, see_also)`)
  that satisfies `errors.As` to a typed `*GuardrailViolation` carrying
  the structured fields. Telemetry middleware reads the `Kind` for
  outcome classification.
- **`internal/fuzzy/`** — `fuzzy.Match` strategies already report which
  strategy succeeded. G-001 detection (D-13) inspects the `find` arg
  shape BEFORE matching; the existing fuzzy package doesn't need to
  change.
- **Existing `helix_*` Prometheus counters** in `internal/obs/` —
  bounded-label discipline (closed enums, no source content) already
  enforced; new `helix_receipt_*` counters slot in alongside.

### Established Patterns

- **Middleware install order = LIFO execution order.** Hard invariant
  documented at `internal/mcp/lazy_init.go:106-108` and re-stated in
  `CLAUDE.md`. Install at step 14b.5 means execution at position 4
  (between LazyInit and Suggestion). Regression test exists in
  `internal/mcp/middleware_test.go`; D-01 of Phase 66 must extend it
  with the new 5-step order assertion.
- **Skill setter for cross-package wiring.** Phase 65 used
  `RepoMapSkill.SetSemanticLookup(...)` to inject the semantic seam
  without taking a build dependency on the semantic store. Phase 66
  follows: a `guardrail.MiddlewareDeps` interface with
  `WorkspaceState(ctx)`, `IssueReceipt(...)`, `LookupReceipt(id)`,
  `Visibility(symbol_id)`, `IsEntrypointReachable(symbol_id)` etc., and
  daemon wires the production implementation post-init.
- **Closed-enum bounded-label metrics** — `RecordEditOutcome` (Phase 53
  D-16) and `RecordRenameStrategy` (Phase 47 D-07) define the pattern.
  Receipt-related counters use the same pattern with closed enums:
  `class ∈ {references_checked, impact_checked, context_gathered,
  structural_overview, diagnostics_clean}`,
  `lookup_outcome ∈ {hit, miss, expired, scope_mismatch, graph_drift,
  workspace_mismatch, freshness_rejected, wrong_class}`.
- **Per-language asset catalogs.** `internal/langregistry/` already
  ships an embedded YAML registry per language. G-005 import-pattern
  catalog (D-17) follows the same shape — embed default catalog as
  YAML, allow user override via `guardrails.G-005.import_patterns.<lang>`.
- **DegradedMode + structured warning vs hard error.** Phase 22 typed
  errors carry both kind and remediation hint; D-12 envelopes follow
  the same shape extended with `required_receipts` and
  `suggested_tools` lists.

### Integration Points

- **Daemon bootstrap step 14b.5** — new `InstallGuardrailMiddleware`
  call between line 706 (`InstallSuggestionMiddleware`) and line 729
  (`InstallLazyInitMiddleware`) in `internal/daemon/daemon.go`.
- **Skill init phase** — new `internal/skill/guardrails/` package with
  blank-import in `internal/daemon/imports.go`; `init()` registers the
  receipt-store dependency closure that read tools call to issue
  receipts, and the validator closure middleware uses on destructive
  tool calls.
- **Read tool wiring** — `find_references`, `analyze_blast_radius`,
  `get_context`, `get_semantic_context`, `get_repo_map` need a
  post-success `IssueReceipt(...)` call. Cleanest hook is a wrapping
  helper invoked in the tool handler's success path, mirroring the
  RecordEditOutcome pattern.
- **Destructive tool wiring** — every destructive tool's `mcp.ToolDef`
  schema gets the `receipts` field added to its argument struct. The
  middleware extracts receipts; tools themselves don't need to read the
  `receipts` field directly.
- **`get_tool_help` topic registry** — extend
  `internal/kernel/help/topics.go` (or equivalent) with 6 new topics
  per D-24. Topic content is loaded from the GUARDRAILS.md / DoD.md
  files at daemon start (or embedded via `embed.FS` for binary-only
  shipping).
- **`internal/semantic/config.go` `GuardrailsConfig`** — extend struct
  with `Rules map[string]RuleConfig`, `Tools map[string]ToolConfig`,
  `ReceiptTTL time.Duration`, `G005 G005Config{PathGlobs, IdentifierPatterns, ImportPatterns map[string][]string}`,
  `G004 G004Config{MaxChangedLines, MaxFiles, MaxFileChangeRatio, EnableRatioTrigger}`.
  Update `internal/config/defaults.go` accordingly.

</code_context>

<specifics>
## Specific Ideas

- **G-005 default catalogs ship with sensible-but-overridable Go / TS /
  JS / Python lists** — full catalog is in
  `66-DISCUSS-CHECKPOINT.json` `per_rule_detection.G-005_security_sensitive`.
  Researcher should review whether to add Java/Rust/C# catalogs in this
  phase or defer. Per CLAUDE.md, first-class language tier is
  Java/Go/Rust/TS/JS — Java + Rust catalogs are arguably also
  required-by-tier.
- **`require_force` enforcement level is in the enum but the override
  surface is NOT implemented.** Per OI explicitly out-of-scope. Tools
  in `require_force` mode behave identically to `enforce` until the
  override surface is added in a follow-up plan. Document this in
  GUARDRAILS.md.
- **Integration test for context-truncation must drive
  forwarder→daemon end-to-end**, not in-process. This satisfies both
  GUARD-02 SC-2 ("agent's context is artificially truncated") and the
  user's UAT-no-manual-MCP memory feedback. Likely pattern: `test/harness/Runner`
  with the forwarder transport; the test deletes the read-tool
  response from the agent-side message log before invoking the
  destructive tool with a fabricated `receipts: []`.

</specifics>

<deferred>
## Deferred Ideas

- **G-006..G-010** (prefer-symbol-edits, no-stale-impact,
  no-false-certainty, multi-file-verify, generated-files) — out of
  scope per REQUIREMENTS.md line 143. Phase 66 ships only G-001..G-005.
- **Force-override surface (`force: bool`, `override_reason: string`,
  audit-event format)** — `require_force` enum value lands but the
  override transport is deferred until enforce-mode false-positive rate
  is measured. Belongs in a v1.10.x patch or v1.11 phase.
- **Persistent / disk-backed receipts** — explicitly forbidden by
  REQUIREMENTS.md line 167. Capture so that future contributors
  understand it was considered and rejected on threat-model grounds.
- **`semantic_guarded` eval-harness mode** — proves guardrails reduce
  agent footguns by running the same eval corpus with and without
  guardrails. Phase 67.
- **Java / Rust / C# G-005 import catalogs** — first-class languages
  per CLAUDE.md but not in the user's specified default catalog. May
  be expanded in a follow-up plan; or, if researcher decides they're
  required for parity, fold into Phase 66 wave plan. Flagged for
  researcher resolution.
- **`get_tool_help` topic prefix-matching support** — if the existing
  topic registry is exact-match only, prefix-matching (`workflow:*`)
  can be implemented two ways: (a) register every concrete topic
  individually; (b) extend registry with prefix dispatch. Researcher
  picks; (a) is fine for v1.

</deferred>

---

*Phase: 66-Agent-Guardrails*
*Context gathered: 2026-05-09*
