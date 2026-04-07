# Phase 2: Code Intelligence Kernel - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-07
**Phase:** 02-code-intelligence-kernel
**Areas discussed:** LS worker lifecycle, LSP type generation, Symbol editing approach, First language scope

---

## LS Worker Lifecycle

**User provided extensive architecture via freeform response:**

Key decisions:
- Standalone engine service model (not embedded evaluator)
- Three core objects: WorkspaceRuntime, SessionView, WorkerLease
- Share-until-dirty (gopls pattern)
- Adaptive TTL with decaying reuse score (not lifetime counter)
- Auto-tuned from observed reuse gaps, not hardcoded language lore
- Platform-aware pressure eviction: Linux PSI, macOS os_proc_available_memory
- Circuit breaking with exponential backoff (never fully breaks)
- TTL=0 means no idle timeout, NOT immortal (pressure eviction always active)

**Notes:** User explicitly pushed back on keying TTL primarily off eval time — should use reuse gap probability instead, with warm_penalty as secondary weight.

---

## LSP Type Generation

| Option | Description | Selected |
|--------|-------------|----------|
| Full metamodel codegen | Generate all LSP 3.17 types from metamodel JSON | ✓ |
| Subset first, expand | Generate only needed types | |

**User's choice:** Full metamodel codegen, implement subset of methods
**Notes:** "Generate everything, expose everything internally, implement selectively." Package layout: cmd/lspgen/, protocol/metaModel.json, protocol/gen/, protocol/patch/

| Option | Description | Selected |
|--------|-------------|----------|
| Custom Go generator | Real generator binary reading metamodel | ✓ |
| go generate + template | Lighter weight text/template | |

**User's choice:** Custom Go generator wired via //go:generate

| Option | Description | Selected |
|--------|-------------|----------|
| Tagged wrapper types | Named wrappers with decode logic (gopls hybrid) | ✓ |
| any + helpers | interface{} with marshal/unmarshal | |

**User's choice:** gopls-style pragmatic hybrid — named wrappers by default, promoted one-of structs for important unions

---

## Symbol Editing Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Pure LSP ranges | documentSymbol ranges only | |
| Tree-sitter parsing | Full AST parsing for boundaries | |
| Hybrid | LSP for discovery, tree-sitter for body surgery | ✓ |

**User's choice:** Hybrid — LSP knows WHAT symbol, tree-sitter knows WHERE to cut
**Notes:** "I would not copy Serena's current LSP-only symbolic editing literally. I would treat it as the reference to improve on." insert-before/after LSP-first, replace-body tree-sitter-first with fallback.

---

## First Language Scope

| Option | Description | Selected |
|--------|-------------|----------|
| gopls only | Single language | |
| gopls + pyright | Two languages | |
| 3-4 languages | gopls + pyright + typescript + rust-analyzer | ✓ |

**User's choice:** 3-4 languages to prove multi-LS worker pool

---

## Claude's Discretion

- File operations, diagnostics, multi-project routing — straightforward, no gray areas

## Deferred Ideas

None — discussion stayed within phase scope.
