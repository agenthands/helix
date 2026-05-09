<!-- Synced from /DoD.md; do not edit directly — edit the root copy and re-run `make sync-docs` -->

# Definition of Done — Helix Agent Checklists

Canonical per-task-class checklists for agents using Helix MCP tools. Each section defines the exact tool sequence required to satisfy guardrail rules (G-001..G-005) for that task class. Load this document as system context so your agent knows exactly which tools to call, in which order, with which receipt IDs.

For operator configuration and rule semantics, see `GUARDRAILS.md`. For runtime access, call `get_tool_help` with `topic: "dod"` or the individual workflow topics.

---

## Definition of Done — Rename

**Satisfies:** G-001 (rename-by-grep), G-003 (public API edit if exported)

### Checklist

- [ ] **Step 1** — Call `find_references` on the symbol being renamed.
  - Confirm the response includes a `_receipt_id` field.
  - If the symbol is public/exported, call `analyze_blast_radius` instead to satisfy G-003.
- [ ] **Step 2** — Record the `_receipt_id` value from the response.
- [ ] **Step 3** — Call `rename_symbol` (preferred) OR `fuzzy_edit`/`replace_in_file` with `receipts: ["<receipt_id>"]`.
- [ ] **Step 4** — Call `verify_edit` to confirm the rename applied cleanly.

---

## Definition of Done — Delete

**Satisfies:** G-002 (delete-without-refs)

### Checklist

- [ ] **Step 1a** — For `safe_delete_symbol`: call `find_references` AND/OR `analyze_blast_radius`.
- [ ] **Step 1b** — For `delete_file` with exported symbols: call `analyze_blast_radius` for each exported symbol.
- [ ] **Step 2** — Confirm zero callers or all callers are safe to orphan.
- [ ] **Step 3** — Call delete with `receipts: ["<id>"]`.
- [ ] **Step 4** — Call `verify_edit`.

---

## Definition of Done — Public API Change

**Satisfies:** G-003 (public API edit without blast-radius check)

### Checklist

- [ ] **Step 1** — Call `analyze_blast_radius` (NOT just `find_references`).
- [ ] **Step 2** — Review `blast_nodes` to understand downstream impact.
- [ ] **Step 3** — Issue `replace_symbol_body` or `rename_symbol` with `receipts: ["<impact_checked_id>"]`.
- [ ] **Step 4** — Call `verify_edit`.

---

## Definition of Done — Large Edit

**Satisfies:** G-004 (large fuzzy edit without context)

### Checklist

- [ ] **Step 1** — For single-file: call `get_context` on the file. For multi-file: call `get_repo_map` on the directory.
- [ ] **Step 2** — Record the `_receipt_id`.
- [ ] **Step 3** — Issue the destructive call with `receipts: ["<context_or_overview_id>"]`.
- [ ] **Step 4** — Call `verify_edit`.

---

## Definition of Done — Security-Sensitive Edit

**Satisfies:** G-005 (security-sensitive edit)

### Checklist

- [ ] **Step 1** — Call `get_context` covering all files you intend to edit.
- [ ] **Step 2** — Record the `_receipt_id` from the response.
- [ ] **Step 3** — Issue the destructive call with `receipts: ["<context_gathered_id>"]`.
- [ ] **Step 4** — Call `verify_edit`. On success, response includes `diagnostics_clean` receipt with `error_count == 0`.
- [ ] **Step 5** — Confirm `error_count == 0`. Report this in session handoff.
