---
phase: 83
slug: cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-21
---

# Phase 83 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from the "Validation Architecture" section of 83-RESEARCH.md — the planner
> fills the Per-Task Verification Map from the four success criteria.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./... && go test ./cmd/helix-bench-rag/... ./bench/runners/... ./internal/lint/ablationleakage/...` |
| **Full suite command** | `go test ./...` (bench smoke needs `HELIX_BIN="$(pwd)/helix"`) |
| **Estimated runtime** | ~60–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run the quick run command for touched packages
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills from RESEARCH Validation Architecture) | | | ABLATE-04 | — | N/A | unit | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Import-boundary vet test asserting `cmd/helix-bench-rag` does not import `internal/kernel/` or `internal/semantic/` (SC #1)
- [ ] `cmd/helix-bench-rag` tool-list test asserting exactly 4 tools (SC #1)
- [ ] schema-valid `result.v2.json` row test with recorded `embedder_id` (SC #3)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live OpenAI `text-embedding-3-small` + Ollama `nomic-embed-text` reachability | ABLATE-04 | Requires network / local Ollama; CI uses deterministic stub embedder | Run a `baseline_rag` build with an OpenAI key set, then again with Ollama running |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
