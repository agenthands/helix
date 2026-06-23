---
phase: 100
slug: polyglot-edit-benchmark-committed-baseline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-23
---

# Phase 100 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + `HELIX_BIN`-gated live cell tests |
| **Config file** | none — Go toolchain; `HELIX_BIN=<built helix>` for live legs |
| **Quick run command** | `go test ./bench/runtime/... ./bench/datasets/aider-polyglot/...` |
| **Full suite command** | `go vet ./... && HELIX_BIN="$(pwd)/helix" go test ./bench/...` |
| **Estimated runtime** | ~60–180 seconds (live leg longer) |

---

## Sampling Rate

- **After every task commit:** `go test ./bench/runtime/... ./bench/datasets/aider-polyglot/...` (hermetic)
- **After every plan wave:** `go vet ./...` + `HELIX_BIN="$(pwd)/helix" go test ./bench/...` (live + hermetic)
- **Before `/gsd-verify-work`:** hermetic golden sibling green; HELIX_BIN live cell RAN (not skipped) and produced a schema-valid result.v2.json; baseline regenerates byte-identically
- **Max feedback latency:** ~180 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills) | — | — | EDITBENCH-01/02/03, BASELINE-01 | — | HELIX_BIN fail-not-skip; deterministic baseline | unit + live-gated | `go test ./bench/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] EDIT-verb `AgentFn` in `bench/runtime` (daemon-dialing via `forwarder.CallTool`) — plugged into the reused `RunExercise` seam, loader untouched, WR-01 restore preserved (EDITBENCH-01)
- [ ] `bench/runners/aider_edit/MODE.md` + `runAiderEditCell` (sibling of `runRAGCell`), zero mode-resolver Go change (EDITBENCH-02)
- [ ] `edit_format_applied *bool` additive open key on result.v2 (mirror `swebench_raw_resolved`); no schema v3 bump; a literal `false` is preserved (EDITBENCH-03)
- [ ] Deterministic scripted-agent baseline (apply exercism `.meta/example` via `replace_in_file` → guaranteed PASS, one go/python exercise × one mode); committed `bench/reports/<run>/BENCH-RESULTS.md` + `result.v2.json` (BASELINE-01)
- [ ] HELIX_BIN fail-not-skip: hermetic golden sibling (no binary) as sole proof + "did it RUN" sentinel when HELIX_BIN set + fail-closed on missing result.v2.json
- [ ] Byte-reproducible: report through zero-RNG `renderAll`, deterministic metrics only (no live latency/tokens)

*Existing infrastructure (`runRAGCell` template, result.v2 additive-open-key pattern, `renderAll`, mode_resolver filesystem-table, Phase 77 subprocess daemon lifecycle) covers most machinery — reuse, don't rebuild.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none expected) | — | The deterministic baseline + hermetic sibling are fully automatable | — |

*All phase behaviors should have automated verification (deterministic scripted agent + hermetic golden).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
