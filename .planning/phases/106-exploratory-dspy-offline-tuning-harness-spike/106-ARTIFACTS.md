# Phase 106 — Artifacts & Multi-Source Coverage Audit

**Planned:** 2026-06-24
**Plans:** 2 (Wave 1: `106-01` Go-side gates · Wave 2: `106-02` Python tree, depends_on 106-01)
**Requirement:** TUNE-01 (covered by both plans)

---

## Artifacts this phase produces (every NEW file + every edit)

### NEW — Go side (Plan 106-01, Wave 1)
| Path | Role |
|------|------|
| `tools/dspy-tune/golden/parity_cases.json` | the SINGLE shared golden corpus (read by both the Go parity test and the Python pytest; never duplicated) |
| `test/oracle/adopt/parity_test.go` | Go side of the parity cross-check — pins the corpus to `ClassifyChoice` + `FirstCommand` |
| `internal/lint/toolsquarantine/analyzer.go` | go/analysis import-boundary analyzer (no package outside `tools/` imports `github.com/agenthands/helix/tools/...`) |
| `internal/lint/toolsquarantine/analyzer_test.go` | analysistest harness: RED (RejectsRuntimeImportingTools), GREEN (AllowsCleanRuntime, AllowsSlashBoundaryLookalike) |
| `internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/leakyruntime/imports.go` | RED fixture (planted runtime→tools/ import, `// want`) |
| `internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/goodruntime/imports.go` | GREEN fixture (clean runtime, no `// want`) |
| `internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/internal/toolsupport/imports.go` | GREEN slash-boundary lookalike (bare "tools" substring, no `// want`) |
| `internal/lint/toolsquarantine/testdata/src/github.com/agenthands/helix/tools/dspytune/dspytune.go` | stub package so the RED fixture's import resolves (under the self-import-exempt tools/ prefix) |
| `cmd/vet-tools-quarantine/main.go` | singlechecker wrapper (copy of `cmd/vet-ablation-leakage/main.go`, swapped analyzer) |

### NEW — Python tree (Plan 106-02, Wave 2) — dev-time only, no `.go`, no `go.mod`
| Path | Role |
|------|------|
| `tools/dspy-tune/scorer.py` | Python re-impl of the adopt classifier (exact mirror of `scorecard.go`) |
| `tools/dspy-tune/test_split.py` | overfit guard — held-out TEST disjoint from train∪val (the FIRST harness step) |
| `tools/dspy-tune/test_parity.py` | parity over the shared corpus + `test_broken_classifier_diverges` (planted-divergence anti-vacuity) |
| `tools/dspy-tune/test_degenerate.py` | metric-gaming guard — degenerate always-`helix` steering flagged, legitimate conditional not |
| `tools/dspy-tune/optimize.py` | DSPy GEPA harness over TRAIN/VAL (TEST excluded); output git-ignored; never auto-adopts |
| `tools/dspy-tune/data/train.jsonl` | TRAIN tasks (GEPA trainset/valset source) |
| `tools/dspy-tune/data/test.jsonl` | sequestered held-out TEST split (optimizer never sees) |
| `tools/dspy-tune/requirements.txt` | pinned dev deps (`dspy==3.1.3`, `pytest`) — never in go.mod or the binary |
| `tools/dspy-tune/README.md` | how to run; dev-env-only `OPENAI_API_KEY`; no-ship legitimate; human-review re-entry gate |
| `tools/dspy-tune/REPORT.md` | the documented spike outcome (no-ship/ship conclusion + MinTasks=5 finding) |

### MODIFIED
| Path | Edit | Plan |
|------|------|------|
| `Makefile` | add `VETTOOL_TOOLS_QUARANTINE` var + `vet:` prereq + `go vet -vettool` recipe line + install rule | 106-01 |
| `.gitignore` | add `/tools/dspy-tune/output/` (raw optimizer dump must never be committed) | 106-02 |

### UNCHANGED gates this phase must keep green (no edits)
- `test/oracle/adopt/scorecard.go` — the single source of truth (mirrored, never re-implemented in Go).
- `cmd/helix-refgen --check` (Makefile `verify-reference`) — byte-clean; this phase adds NO adopted text.
- `internal/cli/skills/helix/SKILL.md` + `reference.md` + the 1536 idle-cost cap — re-entry is a separate human step, out of this phase.
- `internal/langregistry/installer.go` — the legit pip/pipx LS installer the analyzer must NOT flag.

---

## Prohibitions (negative checks — enforced across both plans)
- No runtime package (`internal/...`, `cmd/...` except the analyzer's own cmd) imports `github.com/agenthands/helix/tools/...` — proven by `make vet` running the new analyzer clean.
- `go.mod` unchanged: no `dspy`/python module edge (Go cannot depend on PyPI; asserted == 0).
- No `.go` file and no `go.mod` under `tools/` — it stays invisible to `go test ./...` (only `.py`, `.jsonl`, `.json`, `.txt`, `.md`).
- The leakage analyzer is import-boundary-ONLY (no blanket pip/exec.Command ban) → `internal/langregistry/installer.go` never flagged.
- The golden corpus is a SINGLE committed file (not duplicated under `test/oracle/adopt/testdata`).
- `optimize.py` never writes to / auto-adopts `SKILL.md` or `reference.md`; `output/optimized.json` git-ignored, never committed raw; no `OPENAI_API_KEY` committed.

---

## Multi-Source Coverage Audit

### GOAL (ROADMAP Phase 106 — 4 success criteria)
| # | Success criterion | Covered by | Status |
|---|-------------------|-----------|--------|
| 1 | `tools/` dev-time tree, pinned requirements + git-ignored venv/output, excluded from `go test ./...`, no runtime Python in binary/`helix setup` | 106-02 (the tree, requirements.txt, .gitignore) + 106-01 (analyzer + no-.go invariant enforce it) | COVERED |
| 2 | Optimize skill/steering text against the Phase 101 adoption metric, Python re-impl with golden parity cross-check vs Go `adopt` classifier | 106-01 (corpus + Go parity_test.go) + 106-02 (scorer.py + test_parity.py + optimize.py) | COVERED |
| 3 | Overfit + metric-gaming guards (held-out TEST, degenerate-steering inspection); may conclude no-ship; Go fallback | 106-02 (test_split.py, test_degenerate.py, REPORT.md no-ship, README Go-fallback note) | COVERED |
| 4 | Adopted output re-enters only via human SKILL.md/refgen passing `helix-refgen --check`; `make vet`-style analyzer asserts no Python/optimizer coupling leaks into runtime/merge path | 106-01 (toolsquarantine analyzer + make vet wiring; refgen --check kept green) + 106-02 (README/REPORT document the re-entry gate; optimize.py never auto-adopts) | COVERED |

### REQ (REQUIREMENTS.md)
| ID | Covered by | Status |
|----|-----------|--------|
| TUNE-01 | 106-01 + 106-02 (both `requirements: [TUNE-01]`) | COVERED |

### RESEARCH (106-RESEARCH.md Wave 0 gaps, lines 492-500)
| Wave 0 gap | Covered by | Status |
|------------|-----------|--------|
| `test/oracle/adopt/parity_test.go` | 106-01 Task 1 | COVERED |
| `tools/dspy-tune/golden/parity_cases.json` (≥8 branch-covering incl. substring-trap + ls-lookalike) | 106-01 Task 1 | COVERED |
| `scorer.py` + `test_parity.py` (re-impl + parity + planted-divergence) | 106-02 Tasks 1-2 | COVERED |
| `test_split.py` + `test_degenerate.py` (overfit + metric-gaming) | 106-02 Tasks 1-2 | COVERED |
| `internal/lint/toolsquarantine/{analyzer.go,analyzer_test.go}` + RED/GREEN/lookalike fixtures | 106-01 Task 2 | COVERED |
| `cmd/vet-tools-quarantine/main.go` + Makefile `vet:` wiring + install rule | 106-01 Task 3 | COVERED |
| `requirements.txt` (pinned), `README.md` (no-ship OK), `.gitignore` entry for `output/` | 106-02 Task 3 | COVERED |
| Anti-vacuity break-the-invariant test per gate | 106-01 (RED `// want` fixture, corpus discriminators) + 106-02 (test_broken_classifier_diverges, split disjointness, degenerate flag/no-flag pair) | COVERED |

### CONTEXT (106-CONTEXT.md — D-XX decisions / deferred ideas)
| Item | Disposition |
|------|-------------|
| All implementation = Claude's discretion (discuss skipped) | Exercised per ROADMAP goal + codebase conventions; documented in task actions |
| No locked D-XX decisions | none to cover |
| Deferred: TUNE-FUT-01 (GEPA→MIPROv2/COPRO, val_size>50), TUNE-FUT-02 (quality-joined metric), runtime Python, hand-editing reference.md, new Go deps | EXCLUDED from plans (correctly absent) — REPORT.md notes TUNE-FUT-01 gates the real run on val_size>50 |

**Audit result:** All GOAL / REQ / RESEARCH / CONTEXT items COVERED. No unplanned items. No phase split required (the spike fits two coarse-granularity plans within budget). Deferred ideas correctly excluded.
