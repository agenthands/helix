---
mode: baseline_rag
profile: baseline
---

# baseline_rag

A **registered fail-closed stub**. This mode is part of the ABLATE-01
six-mode convention so the resolver knows the name and the matrix can
enumerate it, but it produces **NO result row this phase**.

The `profile: baseline` frontmatter is a **placeholder** so the file is
well-formed and the resolver parses it; the profile is never actually used
because Plan 03's cell wiring fail-closes this mode **before** any daemon
spawn (the baseline_rag stub is detected by mode name in the cell wiring, not
by a frontmatter marker — that keeps the two-key resolver change-free).

## Deferred to Phase 83

The real RAG arm is **deferred** to **Phase 83** (ABLATE-04). That phase drags
in the standalone `cmd/helix-bench-rag` MCP server plus a chromem-go embedding
index builder — a substantial standalone subsystem that does not belong in the
five-of-six ablation matrix this phase delivers. Until Phase 83 lands that
infrastructure, this mode is a documented placeholder that emits no row.
