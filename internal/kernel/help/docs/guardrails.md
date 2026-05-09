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

---

### G-003 — Public API Edit Without Blast-Radius Check

**Trigger:** Target symbol `IsPublicLike()` (Go uppercase, TypeScript `export`, Java `public`, Rust `pub`/`pub(crate)`, Python `__all__`/no-leading-underscore) OR entry-point-reachable OR participates in interface/type contract OR has external-package references OR the edit changes the signature with `refcount > 0`.

**Required Receipt:** `impact_checked` — not merely `references_checked`.

**Default Enforcement:** `enforce`

---

### G-004 — Large Fuzzy Edit Without Context

**Trigger:** `changed_lines (inserted + deleted) > 50` OR `touched_files > 1`. Secondary trigger: `changed_lines / file_line_count > 0.30`.

**Required Receipt:** `context_gathered` for single-file; `structural_overview` for multi-file.

**Default Enforcement:** `warn`

---

### G-005 — Security-Sensitive Edit

**Trigger:** File path matches security glob, touched symbol matches security pattern, or edited file imports a security-sensitive package.

**Required Receipt:** `context_gathered` + `diagnostics_clean` (ErrorCount == 0) post-edit.

**Default Enforcement:** `enforce`

---

## Receipts

Receipts are issued by `find_references`, `analyze_blast_radius`, `get_context`/`get_semantic_context`, `get_repo_map`, and `get_diagnostics`/`verify_edit`/`run_diagnostics`. Agents receive only the opaque ID (`rcpt_<26-char>`); the daemon validates the ID against a per-workspace in-memory store. Default TTL: 5 minutes. Receipts are invalidated on `graph_version` advance.

## Configuration Reference

```yaml
guardrails:
  enforcement: warn
  receipt_ttl: 5m
  rules:
    G-001: { enforcement: warn }
    G-002: { enforcement: enforce }
    G-003: { enforcement: enforce }
    G-004: { enforcement: warn }
    G-005: { enforcement: enforce }
  tools:
    rename_symbol:        { enforcement: enforce }
    safe_delete_symbol:   { enforcement: enforce }
    replace_symbol_body:  { enforcement: enforce }
    fuzzy_edit:           { enforcement: warn }
    replace_in_file:      { enforcement: warn }
```

## Enforcement Precedence

`CLI > per-tool > per-rule > profile > global default`. Highest-precedence layer that is set wins — NOT most-restrictive.

## Profile Defaults

| Profile | Default Enforcement |
|---------|---------------------|
| claude-code | warn |
| codex | warn |
| ide-assistant | warn |
| full | warn |
| ci-bot | enforce |

## Known Limitations

- `require_force` behaves identically to `enforce` (override transport deferred).
- Java/Rust/C# G-005 catalogs deferred.
- Receipts lost on daemon restart by design.

For the full operator reference including runbook, telemetry, and forbidden patterns, see `/GUARDRAILS.md` in the repository root.
