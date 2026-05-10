<!-- Synced from /GUARDRAILS.md; do not edit directly — edit the root copy and re-run `make sync-docs` -->

# Helix Agent Guardrails (G-001..G-005)

Server-side capability receipts gate destructive MCP tools on prior evidence, surviving agent context compaction via ID-only forwarding to a server-side receipt store. Each destructive operation must present the opaque receipt ID returned by a prior read-side or diagnostics tool; the daemon validates the ID against an in-memory workspace-scoped store keyed by `graph_version`, so no agent-reconstructed claim can satisfy the check. See the v1.10 Phase 66 roadmap entry for design motivation and acceptance criteria.

---

## The Five Rules

### G-001 — Rename by Grep

**Trigger:** `fuzzy_edit` or `replace_in_file` when the `find` argument matches the identifier regex `^[A-Za-z_][A-Za-z0-9_]{2,}$` AND the `find` string equals or is a substring of a known symbol name in the current file's tree-sitter outline. `rename_symbol` itself is exempt — it uses the LSP-native rename path.

**Required Receipt:** `references_checked` OR `impact_checked` for the matched symbol (strict scope: `scope.SymbolID == target.SymbolID`).

**Default Enforcement:** `warn`

**Example Valid Sequence:**
1. Call `find_references` on `AuthMiddleware` → response includes `_receipt_id: "rcpt_AAAA..."`.
2. Call `fuzzy_edit` with `find: "AuthMiddleware"`, `receipts: ["rcpt_AAAA..."]`.
3. Call `verify_edit`.

**Example Rejected Sequence:**
1. Call `fuzzy_edit` with `find: "AuthMiddleware"`, `receipts: []`.
2. Daemon returns `guardrail_violation`, rule `G-001`: "No references_checked or impact_checked receipt for AuthMiddleware. Call find_references first."

---

### G-002 — Delete Without References

**Trigger:**
- `delete_file` — always, for any tracked source file.
- `safe_delete_symbol` — always.
- `replace_symbol_body` — when `refcount > 0`, or caller > 0, or the symbol is public, or entry-point-reachable.
- `fuzzy_edit` / `replace_in_file` — when removing a symbol declaration, body, or large block containing symbols.

**Required Receipt:** `references_checked` OR `impact_checked` (strict symbol scope). For `delete_file` with exported symbols, `impact_checked` is required for those symbols (fallback: `context_gathered` for fully-private files).

**Default Enforcement:** `enforce`

**Example Valid Sequence:**
1. Call `find_references` on `LegacyRouter` → receipt issued.
2. Confirm no other callers in the response.
3. Call `safe_delete_symbol` with `receipts: ["rcpt_BBBB..."]`.
4. Call `verify_edit`.

**Example Rejected Sequence:**
1. Call `safe_delete_symbol` with `receipts: []` → `guardrail_violation`, rule `G-002`.

---

### G-003 — Public API Edit Without Blast-Radius Check

**Trigger:** Target symbol `IsPublicLike()` (Go uppercase, TypeScript `export`, Java `public`, Rust `pub`/`pub(crate)`, Python `__all__`/no-leading-underscore) OR entry-point-reachable OR participates in interface/type contract OR has external-package references OR the edit changes the signature with `refcount > 0`.

**Required Receipt:** `impact_checked` — not merely `references_checked`, because public API edits need blast-radius assessment including type hierarchy and transitive callers.

**Default Enforcement:** `enforce`

**Example Valid Sequence:**
1. Call `analyze_blast_radius` on `UserService.GetUser` → receipt `rcpt_CCCC...` (class: `impact_checked`).
2. Call `replace_symbol_body` with `receipts: ["rcpt_CCCC..."]`.
3. Call `verify_edit`.

**Example Rejected Sequence:**
1. Call `find_references` (issues `references_checked`, not `impact_checked`) on `UserService.GetUser`.
2. Call `replace_symbol_body` with `receipts: ["rcpt_DDDD..."]` (wrong class) → `guardrail_violation`, rule `G-003`: "impact_checked receipt required for public API edit; references_checked is insufficient."

---

### G-004 — Large Fuzzy Edit Without Context

**Trigger:** `changed_lines (inserted + deleted) > 50` OR `touched_files > 1`. Secondary trigger (configurable): `changed_lines / file_line_count > 0.30`.

**Required Receipt:** `context_gathered` for single-file large edits; `structural_overview` for multi-file or module-wide edits; both for very broad rewrites.

**Default Enforcement:** `warn`

**Example Valid Sequence:**
1. Call `get_context` on the file being edited → receipt `rcpt_EEEE...` (class: `context_gathered`).
2. Call `fuzzy_edit` (which will change > 50 lines), `receipts: ["rcpt_EEEE..."]`.
3. Call `verify_edit`.

**Example Rejected Sequence:**
1. Directly call `replace_in_file` changing 200 lines with `receipts: []` → `guardrail_warning` (warn mode): "Large edit without context receipt. Call get_context first."

---

### G-005 — Security-Sensitive Edit Without Context

**Trigger:** ANY of the following for the edited file or symbols:
1. File path matches a security-sensitive path glob (e.g., `**/auth/**`, `**/crypto/**`, `**/middleware/**`).
2. Touched symbol or identifier matches a security-sensitive pattern (e.g., `*Password*`, `*Token*`, `*Secret*`, `*Auth*`).
3. Edited file imports a security-sensitive package/module (language-aware catalog: Go `crypto/*`, `net/http`, `golang.org/x/crypto/*`; TypeScript/JavaScript `jsonwebtoken`, `bcrypt`, `passport`; Python `cryptography`, `jwt`, `hashlib`).

**Required Receipt:** `context_gathered` covering all touched files AND a `diagnostics_clean` receipt post-edit (ErrorCount == 0).

**Default Enforcement:** `enforce`

**Example Valid Sequence:**
1. Call `get_context` on `auth/middleware.go` → receipt `rcpt_FFFF...` (class: `context_gathered`).
2. Call `fuzzy_edit`, `receipts: ["rcpt_FFFF..."]`.
3. Call `verify_edit` → on success, issues `diagnostics_clean` receipt auto-attached to the outcome envelope.

**Example Rejected Sequence:**
1. Call `replace_in_file` on `auth/middleware.go` with `receipts: []` → `guardrail_violation`, rule `G-005`.

---

## Receipts

### Lifecycle

Receipts are issued by 7 tools on their success path:

| Tool | Receipt Class | Scope Fields |
|------|--------------|--------------|
| `find_references` | `references_checked` | `symbol_id`, `ref_count`, `file_path`, `include_tests` |
| `analyze_blast_radius` | `impact_checked` | `symbol_id`, `ref_count`, `public_api`, `blast_nodes`, `max_depth`, `included_callers`, `included_types` |
| `get_context` / `get_semantic_context` | `context_gathered` | `file_set`, `target_symbols`, `task_hash`, `token_budget_used`, `max_tokens` |
| `get_repo_map` | `structural_overview` | `root_path`, `depth`, `file_count`, `max_tokens` |
| `get_diagnostics` / `verify_edit` / `run_diagnostics` | `diagnostics_clean` | `file_set`, `diagnostic_count`, `error_count`, `warning_count`, `tool` |

### ID-Only Forwarding

The agent receives only the receipt **ID** (`rcpt_<26-char-base32>`). The daemon stores the full receipt body server-side. This means:

- Context compaction cannot remove the evidence: the agent only needs to retain the opaque ID string.
- Agents cannot fabricate a passing receipt: the daemon validates `scope.SymbolID == target.SymbolID` (or the class-appropriate check). A forged ID will miss the lookup.
- Receipt bodies are never transmitted to the client — they are server-internal.

### TTL and Invalidation

- **Default TTL:** 5 minutes (`expires_at = issued_at + receipt_ttl`).
- **Graph-version invalidation:** Receipts are keyed by `graph_version`. When `graph_version` advances (any committed change), all receipts issued under the prior graph version are rejected with `ErrGraphVersionMismatch`. Re-gather is the correct recovery: call the read-side tool again.
- **LRU eviction:** Maximum 10,000 receipts per workspace. Oldest by `issued_at` are evicted under pressure.
- **Daemon restart:** Receipts are in-memory only. They are lost on daemon restart. This is intentional — 5-minute TTL makes re-gather cheap; disk-backed receipts are explicitly forbidden (see Known Limitations).

---

## Configuration Reference

```yaml
guardrails:
  enforcement: warn           # Global default for all rules. Options: off | warn | enforce | require_force
  receipt_ttl: 5m             # Receipt TTL. Default: 5 minutes.

  rules:
    G-001: { enforcement: warn }       # Rename by grep
    G-002: { enforcement: enforce }    # Delete without references
    G-003: { enforcement: enforce }    # Public API edit without blast-radius
    G-004: { enforcement: warn }       # Large fuzzy edit without context
    G-005: { enforcement: enforce }    # Security-sensitive edit

  tools:
    rename_symbol:        { enforcement: enforce }
    safe_delete_symbol:   { enforcement: enforce }
    replace_symbol_body:  { enforcement: enforce }
    fuzzy_edit:           { enforcement: warn }
    replace_in_file:      { enforcement: warn }

  G-004:
    max_changed_lines:       50    # Lines threshold triggering G-004
    max_files:               1     # Multi-file threshold
    max_file_change_ratio:   0.30  # Ratio of changed/total lines (secondary trigger)
    enable_ratio_trigger:    true  # Whether ratio secondary trigger is active

  G-005:
    path_globs:
      - "**/auth/**"
      - "**/crypto/**"
      - "**/security/**"
      - "**/middleware/**"
      - "**/*_secret*"
    identifier_patterns:
      - "*Password*"
      - "*Token*"
      - "*Secret*"
      - "*Auth*"
      - "*Credential*"
      - "*Encrypt*"
      - "*Decrypt*"
      - "*Hash*"
      - "*Sign*"
      - "*Verify*"
    import_patterns:
      go:
        - "crypto/*"
        - "net/http"
        - "golang.org/x/crypto/*"
        - "github.com/golang-jwt/jwt*"
      typescript:
        - "jsonwebtoken"
        - "bcrypt"
        - "bcryptjs"
        - "passport*"
        - "@auth/*"
        - "crypto"
      javascript:
        - "jsonwebtoken"
        - "bcrypt"
        - "bcryptjs"
        - "passport*"
        - "crypto"
      python:
        - "cryptography*"
        - "jwt"
        - "hashlib"
        - "hmac"
        - "secrets"
        - "passlib*"
        - "flask_login*"
        - "django.contrib.auth*"
```

### 4-Layer Config Precedence

```
CLI flags  >  project (.helix/project.yml)  >  user (~/.helix/helix_config.yml)  >  profile defaults
```

Each layer can override any key from the layer below. The guardrails config nests under `semantic_index.guardrails` in the full config tree.

---

## Enforcement Precedence

Guardrail enforcement levels are resolved using a **5-layer precedence** ordered from highest to lowest:

```
CLI  >  per-tool  >  per-rule  >  profile  >  global default
```

**The highest-precedence layer that is explicitly set wins** — this is NOT "most-restrictive wins."

### Example

Configuration:
```yaml
guardrails:
  enforcement: enforce           # global default: enforce
  rules:
    G-001: { enforcement: warn } # per-rule override for G-001: warn
```

Profile: `edit` (profile default: `warn`)

**Resolution for G-001:**
- CLI: not set
- Per-tool: not set
- Per-rule: `warn` ← **this is the highest-precedence layer that is set**
- Profile: `warn`
- Global: `enforce`

**Result: G-001 resolves to `warn`**

Even though `global = enforce` is a stricter setting, it is at a lower precedence layer than the per-rule `warn`. The per-rule value wins regardless of its relative strictness.

See D-20 in `66-CONTEXT.md` for the formal specification.

---

## Profile Defaults Table

| Profile | Default Enforcement |
|---------|---------------------|
| claude-code | warn |
| codex | warn |
| ide-assistant | warn |
| full | warn |
| ci-bot | enforce |

---

## Telemetry

### Outcome Classification

The `TelemetryMiddleware` outcome enum is extended with two guardrail-specific values:

| Outcome | When |
|---------|------|
| `guardrail_warned` | Enforce level is `warn`; guardrail fired but tool was allowed. The result includes a `warnings[]` field. |
| `guardrail_blocked` | Enforce level is `enforce` or `require_force`; tool call was rejected. The result is an error with `kind: "guardrail_violation"`. |

### New Counters

Three new bounded-label Prometheus counters (follow the same closed-enum discipline as existing `helix_edit_outcome_total`):

| Counter | Labels | Bound |
|---------|--------|-------|
| `helix_receipt_issued_total` | `class` | `{references_checked, impact_checked, context_gathered, structural_overview, diagnostics_clean}` |
| `helix_receipt_expired_total` | `reason` | `{ttl, graph_version_mismatch, workspace_evicted, daemon_restart_implied}` |
| `helix_receipt_lookup_total` | `outcome` | `{hit, miss, expired, scope_mismatch, graph_drift, workspace_mismatch, freshness_rejected, wrong_class}` |

---

## Runbook

### "My agent is being blocked — what now?"

**Scenario 1: `guardrail_violation`, rule G-001 (rename-by-grep)**

Your agent attempted a text-substitution rename without a references receipt.

Fix:
```
1. Call find_references on the symbol being renamed.
2. Copy the _receipt_id from the response.
3. Retry the fuzzy_edit or replace_in_file with receipts: ["<id>"].
```

Better fix: Use `rename_symbol` instead of `fuzzy_edit`. It is LSP-native, handles cross-file propagation, and is G-001 exempt.

---

**Scenario 2: `guardrail_violation`, rule G-002 (delete-without-refs)**

Fix:
```
1. Call find_references (or analyze_blast_radius for exported symbols) on the symbol/file.
2. Confirm zero callers or that all callers are safe to orphan.
3. Retry the delete with receipts: ["<id>"].
```

---

**Scenario 3: `guardrail_violation`, rule G-003 (public-API-edit)**

A `references_checked` receipt is not sufficient here — you need `impact_checked`.

Fix:
```
1. Call analyze_blast_radius (NOT find_references) on the public symbol.
2. Review the blast_nodes in the response.
3. Retry with receipts: ["<impact_checked_receipt_id>"].
```

---

**Scenario 4: `guardrail_violation`, rule G-005 (security-sensitive)**

Fix:
```
1. Call get_context on the file(s) you are editing to get a context_gathered receipt.
2. Edit with receipts: ["<context_gathered_id>"].
3. Call verify_edit after the edit — it issues a diagnostics_clean receipt automatically.
4. Confirm error_count == 0 in the verify_edit response.
```

---

**Scenario 5: Receipt expired (graph_version mismatch)**

The graph advanced (a prior commit changed the index) after your receipt was issued.

Fix:
```
1. Re-call the read-side tool (find_references, analyze_blast_radius, get_context, etc.).
2. Use the newly-issued receipt ID for the subsequent destructive call.
```

---

**Scenario 6: Lower enforcement temporarily**

Via project config (`.helix/project.yml`):
```yaml
semantic_index:
  guardrails:
    rules:
      G-001: { enforcement: warn }
      G-004: { enforcement: off }
```

Via CLI flag (all rules):
```bash
helix daemon --guardrails.enforcement=warn
```

---

## Known Limitations

- **`require_force` currently behaves identically to `enforce`.** The force-override surface (`force: bool`, `override_reason: string`) and its audit-event format are not yet implemented. When a rule is set to `require_force`, it refuses the operation unless a valid receipt is presented — exactly the same as `enforce`. The distinction will be meaningful once the override transport ships in a follow-up plan.

- **Java, Rust, and C# G-005 import catalogs are deferred.** Default catalogs ship for Go, TypeScript, JavaScript, and Python only. Java/Rust/C# security import detection falls back to path-glob and identifier-pattern signals only. Eval-corpus signal will determine v1.10.x inclusion.

- **Receipts are lost on daemon restart by design.** The in-memory receipt store is not persisted to disk (explicitly forbidden by REQUIREMENTS.md to prevent detached-from-`graph_version` stale receipts). After a daemon restart, re-issue receipts by re-calling the read-side tools.

---

## Forbidden Patterns

The following are explicitly disallowed by design:

- **Persistent receipts detached from `graph_version`** — forbidden by REQUIREMENTS.md line 167. Disk-backed or cross-session receipt stores would allow stale evidence to satisfy guardrails after graph changes.
- **Class-only receipt matching** — rejected (D-09). An agent cannot call `find_references` on `HarmlessSymbol` and use that receipt to rename `AuthMiddleware`. Scope match is strict: `scope.SymbolID == target.SymbolID`.
- **Client-stored receipt bodies** — agents only receive opaque IDs. The full receipt body never leaves the daemon. Agents must not attempt to reconstruct or forge receipts from observable fields.
