---
author: architect
responsible: architect
phase: 142
milestone: v2.14
phase_type: documentation
hard_bar: false
security_relevant: false
design_fork: false
status: complete
parent_artifacts:
  - .planning/milestones/v2.14-REQUIREMENTS.md
---

# Phase 142 CONTEXT — Architecture + Integrations + Conventions (MAP-02, MAP-04, MAP-05)

## Contract {P} S {Q}
- **P (assume):** the current Go tree; CLAUDE.md's Architecture section is broadly current but under-covers `internal/semantic/` and carries some stale counts; the old files describe SerenaAgent/Python-MCP/JetBrains.
- **S (do):** re-map `ARCHITECTURE.md` + `INTEGRATIONS.md` + `CONVENTIONS.md`.
- **Q (establish):** ARCHITECTURE documents the 4 Go layers + daemon bootstrap + the MCP middleware LIFO chain (confirmed from code, not copied) + `internal/semantic/` surveyed fresh as a first-class subsystem + key abstractions/entry-points/error-handling/cross-cutting; INTEGRATIONS documents LSP/tree-sitter/gRPC/sigstore/otel/container/LLM-SDK-dev-time; CONVENTIONS documents Go naming/gofmt+vet+`vet-*` gates/typed errors/skill `init()` registration/generated-code discipline. All grounded, `Analysis Date` = 2026-07-01.

## Verifiable bar
- Middleware chain + 4 layers confirmed against real `internal/mcp/` + `internal/daemon/daemon.go`, not parroted.
- `internal/semantic/` documented as first-class (not omitted like CLAUDE.md).
- Residue-grep clean (only retained-lineage); every convention has a real example path.

## Deliverables (canonical home)
`.planning/codebase/{ARCHITECTURE,INTEGRATIONS,CONVENTIONS}.md`.
