# Phase 24: Validation & Testing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 24-validation-testing
**Areas discussed:** Validation scope, Test upgrade strategy, Error golden files, Validation strictness

---

## Validation Scope

### Where should input validation live?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-tool inline | Each tool handler validates its own required args at the top of its exec function. Follows existing ValidatePath pattern. | ✓ |
| Shared validation helper | A small helper like serr.RequireString(args, "path") that each tool calls. Reduces boilerplate. | |
| MCP middleware layer | Validate required fields declaratively via tool schema before handler runs. | |

**User's choice:** Per-tool inline (Recommended)
**Notes:** None

### Coverage: all tools or only missing?

| Option | Description | Selected |
|--------|-------------|----------|
| Audit all, fix gaps | Audit every tool for missing required-field checks. Skip tools that already validate. | ✓ |
| Only missing tools | Trust that fileops/memory tools already validate, only add to known gaps. | |
| You decide | Claude audits and adds validation wherever missing. | |

**User's choice:** Audit all, fix gaps (Recommended)
**Notes:** None

---

## Test Upgrade Strategy

### How to extract error Kind from MCP responses?

| Option | Description | Selected |
|--------|-------------|----------|
| Parse structured content | Parse JSON error body to extract Kind field. Add expectedKind to errCase struct. | ✓ |
| errors.Is sentinel check | Use errors.Is/As on raw error. Requires harness access to underlying error. | |
| You decide | Claude picks based on existing harness infrastructure. | |

**User's choice:** Parse structured content (Recommended)
**Notes:** None

### Expand test coverage beyond current ~30 cases?

| Option | Description | Selected |
|--------|-------------|----------|
| Expand to cover all tools | Every tool gets at least one InvalidArgs test case. Fill known gaps. | ✓ |
| Upgrade existing cases only | Keep current 30 cases, just add Kind assertions. | |
| You decide | Claude determines which gaps are worth filling. | |

**User's choice:** Expand to cover all tools (Recommended)
**Notes:** None

---

## Error Golden Files

### What should error golden files capture?

| Option | Description | Selected |
|--------|-------------|----------|
| Per-kind template | One golden file per error Kind. All tools returning that Kind must match the template structure. | ✓ |
| Per-tool error snapshots | One golden file per tool per error scenario. More files, per-tool regression. | |
| You decide | Claude picks best approach for regression coverage. | |

**User's choice:** Per-kind template (Recommended)
**Notes:** None

### Where should error golden files live?

| Option | Description | Selected |
|--------|-------------|----------|
| Alongside success goldens | In test/oracle/contract/testdata/golden/errors/. Extends existing infrastructure. | ✓ |
| In integration test dir | In test/integration/testdata/golden/errors/. Closer to three-band tests. | |

**User's choice:** Alongside success goldens (Recommended)
**Notes:** None

---

## Validation Strictness

### Zero-value numeric fields (line=0, column=0)?

| Option | Description | Selected |
|--------|-------------|----------|
| Zero is valid | 0 is a legitimate position (LSP 0-based). Only reject missing keys or wrong types. | ✓ |
| Reject zero for position fields | Treat 0 as likely mistake (1-based indexing). Require explicit values. | |
| You decide | Claude determines based on LSP protocol expectations. | |

**User's choice:** Zero is valid (Recommended)
**Notes:** None

### Empty strings for required string fields?

| Option | Description | Selected |
|--------|-------------|----------|
| Reject empty strings | Empty string for required field = missing. Return InvalidArgs. | ✓ |
| Accept empty strings | Only reject absent keys, not empty values. | |
| You decide | Claude determines per field. | |

**User's choice:** Reject empty strings (Recommended)
**Notes:** None

### Optional fields: silently ignore or validate if present?

| Option | Description | Selected |
|--------|-------------|----------|
| Validate if present | Wrong type for optional field = InvalidArgs. Absent optional = silent default. | ✓ |
| Silently ignore invalid optionals | Wrong type = use default. Never error on optionals. | |

**User's choice:** Validate if present (Recommended)
**Notes:** None

---

## Claude's Discretion

- Exact validation code per tool
- Plan decomposition and wave ordering
- Golden file matching approach (exact vs structural wildcards)
- Which tools need expanded test cases beyond InvalidArgs

## Deferred Ideas

None -- discussion stayed within phase scope
