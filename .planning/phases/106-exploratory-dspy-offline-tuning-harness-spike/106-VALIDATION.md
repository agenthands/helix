---
phase: 106
slug: exploratory-dspy-offline-tuning-harness-spike
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-24
---

# Phase 106 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. SPIKE — a documented no-ship is a legitimate, success-meeting outcome.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (Go)** | Go `testing` + `testify/require`; `golang.org/x/tools/go/analysis/analysistest` for the quarantine analyzer |
| **Framework (Python, dev-time only)** | `pytest` (pinned in `tools/dspy-tune/requirements.txt`); OUT of `go test ./...` by design |
| **Quick run command (Go)** | `go test ./test/oracle/adopt/... ./internal/lint/toolsquarantine/...` |
| **Full suite command** | `make vet && go test ./...` |
| **Python parity run (dev-time)** | `cd tools/dspy-tune && . .venv/bin/activate && pytest -x` |
| **Estimated runtime** | Go ~60–120s; Python parity ~seconds (LM-free) |

---

## Sampling Rate

- **Per task commit:** `go test ./test/oracle/adopt/... ./internal/lint/toolsquarantine/...` + (for Python-touching tasks) `pytest tools/dspy-tune/`
- **Per wave merge:** `make vet && go test ./...`
- **Phase gate (before verify-work):** Full suite green + `make vet` (incl. the new quarantine analyzer) + `go run ./cmd/helix-refgen --check` byte-clean

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 106-01-01 | 01 | 0 | TUNE-01 | Python scorer ≡ Go classifier on golden corpus | unit (Go+pytest) | `go test ./test/oracle/adopt/ -run Parity` / `pytest tools/dspy-tune/test_parity.py` | ❌ W0 | ⬜ pending |
| 106-01-02 | 01 | 0 | TUNE-01 | a deliberately-wrong Python classifier FAILS parity (anti-vacuity) | unit (pytest) | `pytest tools/dspy-tune/test_parity.py::test_broken_classifier_diverges` | ❌ W0 | ⬜ pending |
| 106-01-03 | 01 | 0 | TUNE-01 | runtime pkg importing `tools/` FLAGGED; clean tree + slash-lookalike NOT flagged | analyzer | `go test ./internal/lint/toolsquarantine/` | ❌ W0 | ⬜ pending |
| 106-01-04 | 01 | 0 | TUNE-01 | held-out TEST split excluded from optimizer; degenerate always-`helix` steering flagged | unit (pytest) | `pytest tools/dspy-tune/test_split.py test_degenerate.py` | ❌ W0 | ⬜ pending |
| 106-01-05 | 01 | 0 | TUNE-01 | `make vet` runs the quarantine analyzer on the real tree (no LS-installer false-positive); `go.mod` carries no Python edge | integration | `make vet` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky — refined by the planner.*

---

## Wave 0 Requirements

- [ ] `test/oracle/adopt/parity_test.go` — Go side of the parity cross-check against the shared golden corpus
- [ ] `tools/dspy-tune/golden/parity_cases.json` — shared golden corpus (≥8 branch-covering cases incl. substring-trap + `ls`-lookalike)
- [ ] `tools/dspy-tune/scorer.py` + `test_parity.py` — Python re-impl + parity + **planted-divergence anti-vacuity** (a broken classifier must FAIL)
- [ ] `tools/dspy-tune/test_split.py` + `test_degenerate.py` — overfit (held-out TEST) + metric-gaming (degenerate steering) guards
- [ ] `internal/lint/toolsquarantine/{analyzer.go,analyzer_test.go}` + `testdata/src/.../{leakyruntime,goodruntime,lookalike}` — leakage analyzer with **RED/GREEN/lookalike** fixtures
- [ ] `cmd/vet-tools-quarantine/main.go` + Makefile `vet:` wiring
- [ ] `tools/dspy-tune/requirements.txt` (pinned `dspy==3.1.3`, `pytest`), `README.md` (no-ship is OK), `.gitignore` for `output/` + `.venv/`

> **Anti-vacuity (mandatory for EACH gate):** planted parity-break → pytest RED; planted runtime→tools import → analyzer trips; degenerate always-`helix` steering → flagged. A gate with only a green-path test is presumed broken (CR-01 class).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| The DSPy GEPA optimization run itself (needs an LM backend + API key) | TUNE-01 | LLM optimization is non-deterministic / not bit-reproducible — gate the ARTIFACT, never the optimizer PROCESS | Dev runs `pytest`-validated harness with `OPENAI_API_KEY` set in the DEV env only; inspects the no-ship/ship report. Out of the automated `go test` path by design. |

*The optimizer's adopted output (if any) re-enters ONLY as a human-reviewed SKILL.md/refgen commit passing `helix-refgen --check` — never as a raw optimizer dump.*

---

## Validation Sign-Off

- [ ] All automated gates have `<automated>` verify or Wave 0 dependencies
- [ ] Each gate ships a deliberate break-the-invariant → assert-RED test
- [ ] Held-out TEST split is the FIRST harness task; optimizer never sees it
- [ ] No runtime Python: `go.mod` clean, `tools/` excluded from `go test ./...`, quarantine analyzer in `make vet`
- [ ] No watch-mode flags
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
