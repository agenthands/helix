---
phase: 83
slug: cmd-helix-bench-rag-baseline-rag-mode-embedding-index-builde
status: planned
nyquist_compliant: true
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
| **Quick run command** | `go vet ./... && go test ./cmd/helix-bench-rag/... ./bench/ragindex/... ./bench/runners/... ./internal/lint/benchragleakage/...` |
| **Full suite command** | `go test ./...` (bench cell smoke needs `HELIX_BIN="$(pwd)/helix"`) |
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
| T-83-01-01 | 83-01 | 1 | ABLATE-04 (#2 dep) | T-83-SC | package legitimacy gate | checkpoint | `go list -m github.com/philippgille/chromem-go \| grep -q v0.7.0` | ❌ W0 | ⬜ pending |
| T-83-01-02 | 83-01 | 1 | ABLATE-04 (#2 determinism) | T-83-01-03 | corpus_sha cannot escape cache root | unit | `go test ./bench/ragindex/ -run 'TestCorpusSHADeterministic\|TestCacheDirPrecedence\|TestIndexPathLayout\|TestChunkDeterministic'` | ❌ W0 | ⬜ pending |
| T-83-01-03 | 83-01 | 1 | ABLATE-04 (#2 cache) | T-83-01-01/02 | no API-key logging; pinned Ollama URL (no SSRF) | unit | `go test ./bench/ragindex/ -run 'TestEmbedderIDsDistinct\|TestStubEmbedderDeterministic\|TestCachePathAndReuse\|TestQueryReturnsRelevantChunk'` | ❌ W0 | ⬜ pending |
| T-83-01-04 | 83-01 | 1 | ABLATE-04 (#2 doc) | — | N/A | doc | `grep -q text-embedding-3-small bench/runners/baseline_rag_agent/EMBED-CHOICE.md` | ❌ W0 | ⬜ pending |
| T-83-02-01 | 83-02 | 2 | ABLATE-04 (#1a/#1b) | T-83-02-01/03 | path-traversal rejected; exactly 4 tools | smoke/unit | `go test ./cmd/helix-bench-rag/ -run 'TestHelp\|TestToolListIsExactlyFour\|TestPathTraversalRejected'` | ❌ W0 | ⬜ pending |
| T-83-02-02 | 83-02 | 2 | ABLATE-04 (#1c) | T-83-02-02 | no kernel/semantic import (transitive + static) | unit/vet | `go test ./cmd/helix-bench-rag/ -run TestNoKernelSemanticImport && go test ./internal/lint/benchragleakage/...` | ❌ W0 | ⬜ pending |
| T-83-03-01 | 83-03 | 3 | ABLATE-04 (#3 schema) | T-83-03-01 | embedder_id provenance recorded | unit | `go test ./bench/runtime/ -run TestEmbedderID` | ❌ W0 | ⬜ pending |
| T-83-03-02 | 83-03 | 3 | ABLATE-04 (#3/#4) | T-83-03-02/03 | same contract+budget; embedding cost excluded | integration (HELIX_BIN) | `HELIX_BIN="$(pwd)/helix" go test ./bench/runtime/ -run 'TestBaselineRag\|TestDeferred'` | ❌ W0 | ⬜ pending |
| T-83-03-03 | 83-03 | 3 | ABLATE-04 (#4 doc) | — | N/A | doc | `grep -qi 'out-of-band\|not charged\|excluded from' bench/BENCH.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/ragindex/{cache,chunk,embedder,index}_test.go` — corpus_sha determinism, cache reuse, stub-embedder query (#2, determinism)
- [ ] `cmd/helix-bench-rag/main_test.go` — `--help` works; tool-list == exactly 4; path-traversal rejected (#1a/#1b)
- [ ] `cmd/helix-bench-rag/leakage_test.go` — transitive import-set excludes kernel/semantic (#1c)
- [ ] `internal/lint/benchragleakage/` analyzer + testdata fixtures + `cmd/vet-bench-rag-leakage` wired into `make vet` (#1c static)
- [ ] `bench/runtime/result_test.go` — embedder_id additive key emitted/omitted + schema-valid (#3)
- [ ] `bench/runtime/baseline_rag_test.go` — schema-valid row with embedder_id + same-contract/budget (#3/#4), replaces `cell_test.go` TestDeferred*
- [ ] Framework install: chromem-go (`go get`, behind the Task 1 legitimacy checkpoint); Go testing + testify already present

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live OpenAI `text-embedding-3-small` + Ollama `nomic-embed-text` reachability | ABLATE-04 | Requires network / local Ollama; CI uses the deterministic `stub-deterministic` embedder | Run a `baseline_rag` build with an OPENAI_API_KEY set, then again with Ollama running; confirm distinct `embedder_id` recorded |

*All other phase behaviors have automated verification (stub embedder makes the index path unit-testable with zero network).*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** planned
