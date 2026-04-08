# Phase 7: Symbol Editing + Multi-Language Fixtures - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-08
**Phase:** 07-symbol-editing-multi-language-fixtures
**Areas discussed:** Fixture porting strategy, LS availability per lang, Edit test isolation

---

## Fixture Porting Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Create minimal from scratch | 2-3 files per language with known symbols. Clean, no legacy baggage. Easy to maintain. | ✓ |
| Port legacy repos as-is | Copy from legacy/test/resources/repos/. Battle-tested symbols but may carry Maven/npm complexity. | |
| Port and trim | Start from legacy repos but strip to minimal files. Best of both: known symbols, no cruft. | |

**User's choice:** Create minimal from scratch
**Notes:** Clean fixtures with controlled symbols, no legacy baggage.

---

## LS Availability Per Language

| Option | Description | Selected |
|--------|-------------|----------|
| Standard set | pyright (Python), typescript-language-server (TS), jdtls (Java), rust-analyzer (Rust). Most common, matches Serena's defaults. | ✓ |
| Lightweight set | pylsp (Python), tsserver (TS), skip Java (heavy), rust-analyzer (Rust). Easier CI setup, faster. | |
| You decide | Claude picks based on what Serena's language registry already supports | |

**User's choice:** Standard set
**Notes:** Matches Serena's language registry defaults.

---

## Edit Test Isolation

| Option | Description | Selected |
|--------|-------------|----------|
| PrepareFixture per test | Existing pattern: copy entire fixture dir to t.TempDir() per test. Safe, ~1ms overhead. Already proven in Phase 6. | ✓ |
| Shared fixture + revert | One copy, revert files after each edit test. Faster but fragile if test fails mid-edit. | |
| You decide | Claude picks based on Phase 6 patterns | |

**User's choice:** PrepareFixture per test
**Notes:** Proven pattern from Phase 6, safe isolation.

---

## Claude's Discretion

- Exact fixture contents, generic requireLS helper design, edit operation coverage per language

## Deferred Ideas

None
