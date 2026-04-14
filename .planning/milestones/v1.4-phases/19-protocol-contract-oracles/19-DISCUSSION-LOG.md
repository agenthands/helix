# Phase 19: Protocol & Contract Oracles - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 19-protocol-contract-oracles
**Areas discussed:** Protocol test scope, Golden file strategy, Schema validation, Overlap with existing tests

---

## Protocol Test Scope

### Transport Parity

| Option | Description | Selected |
|--------|-------------|----------|
| Handshake + tool call | Test initialize/shutdown + one representative tool call on both transports | ✓ |
| Full mirror | Run every protocol test on both InMemory AND HTTP transports | |
| InMemory primary, HTTP smoke | All protocol tests on InMemory, single HTTP smoke test | |

**User's choice:** Handshake + tool call (Recommended)
**Notes:** Proves both paths work without duplicating the full contract suite on HTTP.

### Session Isolation

| Option | Description | Selected |
|--------|-------------|----------|
| Workspace + mode isolation | Two concurrent sessions with different workspaces and modes | ✓ |
| Full state isolation | Also verify memory stores, skill state, LS worker assignments | |
| You decide | Claude picks depth | |

**User's choice:** Workspace + mode isolation (Recommended)
**Notes:** Matches SC-3 directly.

### Reconnect Simulation

| Option | Description | Selected |
|--------|-------------|----------|
| Client-side disconnect/reconnect | Drop MCP client session, create new one against same daemon | ✓ |
| Transport-level disconnect | Close underlying transport mid-request | |
| You decide | Claude picks approach | |

**User's choice:** Client-side disconnect/reconnect (Recommended)
**Notes:** Tests the common real-world scenario.

---

## Golden File Strategy

### Golden Content

| Option | Description | Selected |
|--------|-------------|----------|
| Normalized response shape | Structure with placeholders for non-deterministic values | ✓ |
| Full response snapshots | Exact JSON output with snapshot testing | |
| Schema-only | No golden files, rely on JSON Schema validation | |

**User's choice:** Normalized response shape (Recommended)

### Golden Location

| Option | Description | Selected |
|--------|-------------|----------|
| test/oracle/contract/testdata/ | Co-located with contract oracle package | ✓ |
| testdata/golden/ at repo root | Central golden directory | |
| You decide | Claude picks layout | |

**User's choice:** test/oracle/contract/testdata/ (Recommended)

### Error Goldens

| Option | Description | Selected |
|--------|-------------|----------|
| Error category table | One golden per error category | ✓ |
| Per-tool error goldens | Each tool gets its own error golden | |
| You decide | Claude picks granularity | |

**User's choice:** Error category table (Recommended)

---

## Schema Validation

### Library Choice

| Option | Description | Selected |
|--------|-------------|----------|
| santhosh-tekuri/jsonschema/v6 | Supports Draft 2020-12, passes official test suite | ✓ |
| xeipuuv/gojsonschema | Stuck on Draft 07 | |
| You decide | Claude picks | |

**User's choice:** santhosh-tekuri/jsonschema/v6
**Notes:** User provided detailed rationale — v6 explicitly supports Draft 2020-12, passes JSON Schema Test Suite. Implementation plan: compile meta-schema once at startup, validate every tool inputSchema, fail fast, keep instance and schema validation separate.

### Validation Scope

| Option | Description | Selected |
|--------|-------------|----------|
| inputSchema only | Validate inputSchema against meta-schema, outputs via goldens | |
| inputSchema + outputSchema (declared only) | Also validate declared outputSchema and actual results against it | ✓ |
| You decide | Claude determines | |

**User's choice:** inputSchema + outputSchema (when declared)
**Notes:** User pointed out MCP spec and Go SDK both support outputSchema on Tool type. Policy: always validate declared inputSchema, validate declared outputSchema when present, validate actual results against outputSchema when present, golden files for tools without outputSchema.

---

## Overlap with Existing Tests

### Oracle vs Integration Relationship

| Option | Description | Selected |
|--------|-------------|----------|
| Oracle is authoritative layer | Oracle canonical, integration continues as safety net | ✓ |
| Oracle replaces coverage | Track and deprecate covered integration tests | |
| Oracle extends only | No overlap with integration tests | |

**User's choice:** Oracle is the authoritative layer (Recommended)

### Description Selectability (CONT-04)

| Option | Description | Selected |
|--------|-------------|----------|
| Deterministic heuristics | String analysis — action verbs, uniqueness, disambiguation | ✓ |
| Include LLM tests | Actual LLM-backed selectability in this phase | |
| You decide | Claude determines boundary | |

**User's choice:** Deterministic heuristics (Recommended)
**Notes:** LLM behavioral selectability tests come in Phase 20.

---

## Claude's Discretion

- Test helper structure within protocol/ and contract/ packages
- Whether to add schema validation helpers to test/harness/ or keep local
- Specific normalization strategy for golden file placeholders
- Transport parity test structure

## Deferred Ideas

None
