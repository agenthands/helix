---
author: architect
responsible: architect
phase: 141
milestone: v2.14
phase_type: documentation
hard_bar: false
security_relevant: false
design_fork: false
status: complete
parent_artifacts:
  - .planning/milestones/v2.14-REQUIREMENTS.md
---

# Phase 141 CONTEXT — Structural + Stack skeleton (MAP-01, MAP-03)

## Contract {P} S {Q}
- **P (assume):** the current Go tree at HEAD; `go.mod`, `ls cmd/`, `ls internal/` are authoritative; the old STRUCTURE.md/STACK.md describe the removed Python `serena` tree.
- **S (do):** re-map `.planning/codebase/STRUCTURE.md` + `.planning/codebase/STACK.md` to the Go tree.
- **Q (establish):** both files describe ONLY the current Go tree — directory layout (16 `cmd/` binaries, 24 `internal/` packages with per-package purpose, `api/proto`, `protocol/{gen,patch}`, `bench/`, `tools/dspy-tune/`), 4-layer mapping, LOC/scale context, and the real dependency stack read verbatim from `go.mod`. `Analysis Date` = 2026-07-01. Staleness banner removed.

## Verifiable bar
- Every `cmd/` binary and `internal/` package named exists (ls/read-confirmed); no fabricated paths.
- All dep versions match `go.mod` exactly.
- Residue-grep: only labeled retained-lineage (`SerenaMCPServer`, `serena/v1`) remains.

## Deliverables (canonical home)
`.planning/codebase/STRUCTURE.md`, `.planning/codebase/STACK.md` (the map's canonical location — not duplicated under this phase dir).
