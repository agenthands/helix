# Phase 66: Agent Guardrails (G-001..G-005, warn-default) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-09
**Phase:** 66-agent-guardrails
**Areas discussed:** Receipt schema + storage, Tool argument surface, Per-rule detection, Override precedence + GUARDRAILS.md scope

---

## Receipt Schema + Storage

### Q1 — Which read-side tools issue safety receipts?

| Option | Description | Selected |
|--------|-------------|----------|
| Narrow: 2 tools | `find_references` + `analyze_blast_radius` only. Smallest surface, easiest to reason about, matches research.M4 minimal example. | |
| Medium: + get_context / get_repo_map | Adds context-gathered + structural-overview receipts so G-004 (large-fuzzy-edit-without-prior-context) can be satisfied without forcing a per-symbol find_references. | ✓ |
| Wide: every read tool | Including goto_definition/hover/find_implementations etc. Receipt-class explosion (10+ types) and weakened semantics. | |

**User's choice:** Medium (4 receipt classes).
**Notes:** User extended the answer with explicit G-rule → receipt-class mapping: G-001/G-002 = references_checked OR impact_checked; G-003 = impact_checked; G-004 = context_gathered OR structural_overview; G-005 = structural_overview + context_gathered. Reasoning: receipts must represent "safety claims," not just "a read happened" — wide set creates receipt inflation where any prior read appears to justify a destructive op.

### Q2 — Receipt record schema (fields + scope shape)

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal + class-specific scope | Closed enum class + per-class typed scope union; common fields id/class/workspace_key/graph_version/issued_at/expires_at/issuing_tool/issuing_call_id/scope. | |
| Minimal + opaque scope blob | `scope` is `map[string]any`; loses static typing in Go and makes destructive-tool checks stringly-typed. | |
| Maximal: include freshness snapshot + LSP enrichment level | Adds `freshness_mode`, `pending_lsp_files`, `enrichment_level`. Matches freshness-as-API doctrine. | |

**User's choice:** "Option 1 + the useful part of option 3" (custom answer).
**Notes:** Locked typed Go union (not map[string]any) AND mandated freshness snapshot fields (snapshot_id, graph_version, freshness, score_status, cluster_status, pending_lsp_files, lsp_coverage). User provided full Go struct definitions for `Receipt`, `ReceiptScope` interface, and the four per-class scope structs. Validation predicate `ValidateReceiptForMutation` rejects on workspace mismatch, graph_version mismatch, expiry, stale/failed freshness, or pending_lsp_files when full semantic validation required. Default expiry: 5 minutes from issued_at.

### Q3 — Where do receipts live?

| Option | Description | Selected |
|--------|-------------|----------|
| In-memory sync.Map + janitor | Per-workspace map, RWMutex, janitor every 30s, max 10k LRU. Lost on restart (acceptable: TTL=5m + graph_version invalidation). Zero overhead, no DuckDB coupling. | ✓ |
| DuckDB table in semantic.duckdb | Survives restart but adds I/O on every read tool, requires schema migration, breaks when `semantic_index.enabled=false`. | |
| In-memory + snapshot to disk on shutdown | Modest restart-survival; complexity in serializing typed unions; 5-min TTL means most restarts lose them anyway. | |

**User's choice:** In-memory sync.Map + janitor.
**Notes:** Decision rationale matches the GUARD-03 5-min TTL contract — disk-backed receipts buy nothing meaningful and create coupling problems. Also satisfies REQUIREMENTS.md line 167 (persistent receipts detached from graph_version explicitly forbidden).

### Q4 — Receipt ID shape

| Option | Description | Selected |
|--------|-------------|----------|
| Random UUIDv7 | Time-ordered, opaque, unguessable. Stdlib via `crypto/rand`. | ✓ |
| Hash-of-claim (rcpt_<sha256>) | Idempotent dedupe but creates probing/guessing risks; needs HMAC to defend → more complex than UUIDv7. | |
| Opaque integer counter | Enumerable; only safe with per-session scoping. | |

**User's choice:** Random UUIDv7 with `rcpt_` prefix.
**Notes:** Final shape `rcpt_<UUIDv7-base32-26char>`. User provided full `NewReceiptID` and `ParseReceiptID` Go signatures. Receipts are authorization-like references to server-side claims and must be opaque, not guessable, not derived from claim content, and safe to log.

---

## Tool Argument Surface

### Q1 — How does an agent forward receipts?

| Option | Description | Selected |
|--------|-------------|----------|
| Optional `receipts: []string` array on every destructive tool | Uniform field name; supports multi-receipt rules; middleware-friendly. | ✓ |
| Single optional `receipt_id: string` | Cleaner UX but G-005 (`structural_overview + context_gathered`) needs two distinct receipts. | |
| Opaque `evidence` object envelope | Future-proof for non-receipt evidence (CI green, code-owner approval) but tempts agents to pass evidence the server doesn't actually check. | |

**User's choice:** Uniform `receipts: []ReceiptID` array.
**Notes:** User locked ID-only forwarding (matches GUARD-03), uniform field name across all destructive tools, middleware-side `ExtractReceipts` and `GuardDestructiveCall` helpers, and structured `GuardrailViolation` error envelope with `rule`, `required_receipts`, `suggested_tools`. Explicitly rejected the `evidence` envelope as premature — start strict with receipt IDs only; if override workflows are needed later, add a separate top-level `override_policy` field (NOT call arbitrary agent text "evidence").

### Q2 — Scope match per class or class-only?

| Option | Description | Selected |
|--------|-------------|----------|
| Strict scope match per class | references/impact require symbol_id match; context_gathered requires file_set contains target; structural_overview requires path_within. | ✓ |
| Class-only — any receipt of right class | Simpler middleware, opens loophole "find_references on UnrelatedSymbol then rename AuthMiddleware". | |
| Strict for symbol-scoped, lenient for file/repo-scoped | Compromise that catches symbol-target loophole but leaves file/repo loophole open. | |

**User's choice:** Strict scope match per class.
**Notes:** User provided full `ValidateReceiptForOperation` Go signature plus per-class match functions (`MatchReferencesChecked`, `MatchImpactChecked`, `MatchContextGathered`, `MatchStructuralOverview`). Reasoning: class-only matching turns receipts into a checkbox, not a safety claim — same loophole at file/repo scope as at symbol scope ("get_context on internal/logging then fuzzy_edit in internal/auth").

### Q3 — Rollout / backward-compat for existing destructive tools

| Option | Description | Selected |
|--------|-------------|----------|
| Warn-default + telemetry, never break | GUARD-07 default; only enforce-mode profiles refuse. ci-bot/legacy callers keep working. | |
| Per-tool opt-in via config flag | Each tool reads `guardrails.tools.<name>.enforce_receipts`; surgical but config-heavy. | |
| Hard require receipts, gated by profile=review | review profile enforces hard; other profiles continue without enforcement. | |

**User's choice:** Custom — schema-day-one + warn dev/edit / enforce review/ci-bot/security.
**Notes:** User explicitly noted "this is not in production yet, do not optimize for legacy compatibility." Rolled out as: receipts field present in every destructive tool schema from day one; missing field normalized to `[]`; warn for `claude-code/codex/ide-assistant/edit` profiles; enforce for `review/ci-bot/security` profiles. Telemetry always emitted regardless of mode. **Delta from REQUIREMENTS.md GUARD-07:** GUARD-07 only specifies warn for read/edit + require_force for review. User's defaults extend this to ci-bot=enforce and add a new `security` profile=enforce. Captured as OI-01 for researcher resolution.

---

## Per-Rule Detection

### Q1 — G-001 (rename-by-grep) detection

| Option | Description | Selected |
|--------|-------------|----------|
| Identifier-shaped fuzzy_edit/replace_in_file | Trigger when `find` matches `^[A-Za-z_][A-Za-z0-9_]{2,}$` AND equals/substring of known symbol in file outline. rename_symbol exempt. | ✓ |
| Any text edit on file with ≥1 known symbol | Simple; high false-positive rate; trains agents to ignore warnings. | |
| Only when `find` matches workspace symbol index | Most precise but couples G-001 to semantic-index availability. | |

**User's choice:** Identifier-shaped fuzzy_edit + tree-sitter outline membership.
**Notes:** Tree-sitter outline is independent of semantic index, so G-001 still works in degraded mode. rename_symbol going through LSP rename is the safe path and explicitly exempted.

### Q2 — G-004 (large fuzzy edit) threshold

| Option | Description | Selected |
|--------|-------------|----------|
| LOC-based: changed-lines > 50 OR files > 1 | Reviewer-aligned, easy to compute pre-apply. Both knobs configurable. | ✓ |
| Byte-size: edit_size_bytes > 4096 | Conflates reformatting with risky changes; doesn't catch multi-file. | |
| Percentage: changed-lines / file-line-count > 30% | Catches rewrites; harder for multi-file; needs file line counts. | |

**User's choice:** LOC-based + optional ratio refinement.
**Notes:** User added a secondary trigger `changed_lines / file_line_count > 0.30` (configurable, default on). Final config: `guardrails.G-004.{max_changed_lines:50, max_files:1, max_file_change_ratio:0.30, enable_ratio_trigger:true}`.

### Q3 — G-005 (security-sensitive) detection

| Option | Description | Selected |
|--------|-------------|----------|
| Configurable path globs + identifier denylist | Path globs + symbol/identifier patterns. Configurable; no semantic-index dependency. | |
| Heuristic: path globs only | Just path-glob; misses generic-named files containing security logic. | |
| Tree-sitter import-graph: edits to files importing crypto/auth | Most precise; doesn't catch security work in non-imported helpers. | |

**User's choice:** Custom — 3-signal (path globs + identifier patterns + language-aware import patterns).
**Notes:** User chose all three signals from day one, not v2. Added language-aware import-pattern catalog covering Go (crypto/*, golang.org/x/crypto/*, golang-jwt/*, lestrrat-go/jwx/*, coreos/go-oidc/*, golang.org/x/oauth2), TypeScript/JavaScript (jsonwebtoken, jose, bcrypt/bcryptjs, argon2, passport*, @auth/*, next-auth, openid-client, oauth4webapi, express-session, cookie-session, csurf, helmet), Python (jwt, jose, authlib, oauthlib, requests_oauthlib, cryptography, Crypto, OpenSSL, passlib, bcrypt, argon2, itsdangerous, django.contrib.auth, flask_login, flask_jwt_extended, fastapi.security). Import-pattern matching: exact, wildcard suffix, package prefix, scoped wildcard, Python dotted prefix, Go module prefix. Added 5th receipt class `diagnostics_clean` issued by `get_diagnostics`/`verify_edit`/`run_diagnostics`. Timing: G-005 is a post-edit validation rule.

### Q4 — G-002 (delete) and G-003 (public-API) triggers

| Option | Description | Selected |
|--------|-------------|----------|
| Symbol-level + visibility-aware | G-002 on delete_file + symbol delete with refs; G-003 on public/exported/protected/entry-point-reachable. Degraded fallback to warn. | ✓ |
| Pure delete/edit type — always trigger | High false-positive rate; trains agents to ignore. | |
| Source-file-extension-only | Cheap; doesn't distinguish public vs private. | |

**User's choice:** Symbol-level + visibility-aware.
**Notes:** User provided full per-tool predicate matrix for both rules. G-002 covers delete_file (tracked source), safe_delete_symbol (always), replace_symbol_body (when refs/callers/public/entrypoint), and fuzzy_edit/replace_in_file when removing declarations/bodies/large blocks containing symbols. G-003 covers replace_symbol_body, rename_symbol, safe_delete_symbol, delete_file, and fuzzy_edit/replace_in_file touching public symbol declaration. Visibility enum (Private/Internal/Protected/Public/Exported/Unknown) is language-aware (Go uppercase, TS export, Java public, Rust pub/pub(crate), Python __all__/no-leading-underscore, C# public/protected). Critically: degraded mode when semantic index is unavailable triggers conservatively in warn — DO NOT silently no-op.

---

## Override Precedence + GUARDRAILS.md Scope

### Q1 — Enforcement-level override precedence

| Option | Description | Selected |
|--------|-------------|----------|
| 5-layer: CLI > per-tool > per-rule > profile > global | Standard koanf precedence + ad-hoc CLI override; 5 levels of granularity. | ✓ |
| Profile-default + global CLI only | Simpler, inflexible — can't say "warn G-001 but enforce G-003". | |
| Per-rule only (no per-tool) | Smaller surface; loses tool-specific risk distinction. | |

**User's choice:** 5-layer precedence.
**Notes:** Critical clarification from user: "highest wins" means highest *precedence layer* wins, NOT most restrictive value. Example: `guardrails.enforcement: enforce` + `guardrails.rules.G-001.enforcement: warn` resolves G-001 to `warn`. User provided full `ResolveGuardrailEnforcement` Go signature and the closed `EnforcementLevel` enum (off/warn/enforce/require_force). Note: `require_force` should not silently bypass receipt checks — it should require an explicit force override and create a separate audit event (override surface itself is OUT OF SCOPE this phase per Deferred).

### Q2 — GUARDRAILS.md and DoD.md scope

| Option | Description | Selected |
|--------|-------------|----------|
| Two docs, two audiences | GUARDRAILS.md = operator-facing (semantics, config, runbook, telemetry); DoD.md = agent-consumable (per-task-class checklists). | ✓ |
| Single GUARDRAILS.md with both sections | Simpler maintenance; harder to surface different content to different audiences. | |
| GUARDRAILS.md only — inline DoD per rule | Skips DoD.md; arguably misses GUARD-06 wording. | |

**User's choice:** Two docs, two audiences.
**Notes:** Per-task-class checklists in DoD.md cover rename, delete, public-API-change, large-edit, security-sensitive-edit. Designed so an agent loading DoD.md as system-context knows the canonical sequence (e.g., `find_references` → receipt → `rename_symbol{receipts:[...]}` → `verify_edit`).

### Q3 — Doc location + runtime discovery

| Option | Description | Selected |
|--------|-------------|----------|
| Repo root + get_tool_help integration | `./GUARDRAILS.md` + `./DoD.md`; new `get_tool_help` topics; `see_also` recovery hints in violations. | ✓ |
| docs/ subdirectory | Cleaner repo root; less casual-browser discoverable; same get_tool_help surface. | |
| Phase-internal only | Satisfies GUARD-06 technically but invisible after phase ships; agents can't load via get_tool_help. | |

**User's choice:** Repo root + get_tool_help integration.
**Notes:** New `get_tool_help` topics: `guardrails`, `dod`, `workflow:rename`, `workflow:delete`, `workflow:large-edit`, `workflow:security-sensitive-edit`. Every guardrail violation includes `see_also: [{tool: "get_tool_help", args: {topic: "workflow:<name>"}}]`. Guardrails and DoD are part of the product contract for agents and operators — they belong at repo root alongside README/USAGE/CONTRIBUTING/CHANGELOG.

---

## Claude's Discretion

None — every gray area was resolved by an explicit user decision. Researcher Open Items (OI-01..OI-05) in CONTEXT.md are tasks for the next phase, not Claude-discretion items.

## Deferred Ideas

- G-006..G-010 — out of scope per REQUIREMENTS.md line 143
- Force-override surface (`force: bool` + audit-event format) — `require_force` enum value lands but transport deferred until enforce-mode false-positive rate is measured
- Persistent / disk-backed receipts — explicitly forbidden by REQUIREMENTS.md line 167
- `semantic_guarded` eval-harness mode — Phase 67
- Java / Rust / C# G-005 import catalogs — researcher to decide whether to fold into Phase 66 or follow-up plan
- `get_tool_help` topic prefix-matching support — researcher picks; per-topic registration is fine for v1
