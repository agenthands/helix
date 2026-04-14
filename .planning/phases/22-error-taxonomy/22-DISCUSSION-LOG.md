# Phase 22: Error Taxonomy - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-14
**Phase:** 22-error-taxonomy
**Areas discussed:** Error granularity, Package placement, Detail richness, CircuitOpen migration

---

## Error Granularity

| Option | Description | Selected |
|--------|-------------|----------|
| Flat kinds only | 7 top-level Kind constants (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout). Agents match on Kind; Detail carries specifics. | ✓ |
| Hierarchical sub-kinds | Top-level kinds with sub-kinds (e.g., NotFound.Symbol, NotFound.File). More expressive but more complex API. | |
| Flat kinds + optional Code | 7 flat kinds plus an optional string Code field for sub-classification. | |

**User's choice:** Flat kinds only (Recommended)
**Notes:** None — straightforward selection.

---

## Package Placement

| Option | Description | Selected |
|--------|-------------|----------|
| internal/errors/ | New dedicated package. Clean import path, no coupling to MCP layer, usable by all layers. | ✓ |
| internal/mcp/errors.go expansion | Expand existing file. Ties taxonomy to MCP package — layer violation for kernel/skill imports. | |
| internal/serr/ | Short package name, avoids shadowing stdlib 'errors'. Less conventional naming. | |

**User's choice:** internal/errors/ (Recommended)
**Notes:** None — straightforward selection.

---

## Detail Richness

| Option | Description | Selected |
|--------|-------------|----------|
| String detail | Detail is a plain string. Agents parse Kind programmatically; Detail is for display/logging. | ✓ |
| Typed detail (map) | Detail is map[string]any with structured metadata (path, line, etc.). | |
| Typed detail (any) | Detail is 'any' — flexible but less predictable for consumers. | |

**User's choice:** String detail (Recommended)
**Notes:** None — straightforward selection.

---

## CircuitOpen Migration

| Option | Description | Selected |
|--------|-------------|----------|
| Flatten into Detail string | CircuitOpenError metadata formatted into Detail string. errors.Is backward compat via Kind-based Is(). | ✓ |
| Keep as specialized subtype | CircuitOpenError stays in lspool, also satisfies typed error interface. Two representations coexist. | |
| Embed in new Error | Wrap CircuitOpenError as cause. errors.As extracts rich metadata when needed. | |

**User's choice:** Flatten into Detail string (Recommended)
**Notes:** None — straightforward selection.

---

## Claude's Discretion

- Constructor ergonomics (method signatures, builder vs functional options)
- File organization within internal/errors/
- Convenience constructors per Kind
- JSON serialization approach

## Deferred Ideas

None — discussion stayed within phase scope.
