---
phase: 99
slug: vendored-aider-fixtures-mixed-license-gate
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-23
---

# Phase 99 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) + `make verify-licenses` (hard-fail gate) |
| **Config file** | none — Go toolchain |
| **Quick run command** | `go test ./cmd/helix-bench/... ./bench/...` |
| **Full suite command** | `go vet ./... && go test ./... && make verify-licenses` |
| **Estimated runtime** | ~60–120 seconds |

---

## Sampling Rate

- **After every task commit:** `go test ./cmd/helix-bench/... ./bench/...` + `make verify-licenses`
- **After every plan wave:** `go vet ./... && go test ./... && make verify-licenses`
- **Before `/gsd-verify-work`:** full suite + license gate green; manifest sha256 walk matches disk
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| (planner fills) | — | — | VENDOR-01/02/03 | — | pinned-SHA + path-safe vendoring; no byte mutation of executed fixtures | unit + gate | `make verify-licenses` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Vendored MIT polyglot subset under `bench/datasets/aider-polyglot/fixtures/<lang>/<exercise>/` (9 exercises × python/go/rust/java) — committed, byte-identical to upstream @ pinned SHA (VENDOR-01)
- [ ] Vendored Apache-2.0 aider edit-format fixtures subtree (small `tests/fixtures/languages/*` + trimmed SEARCH/REPLACE gold excerpt) with Apache-2.0 NOTICE/LICENSE (VENDOR-02)
- [ ] `VENDOR-MANIFEST.md` recording exact exercise selection + pinned SHAs + per-file sha256 + license disposition (MIT vs Apache-2.0)
- [ ] Extended `cmd/helix-bench/verify_licenses.go` — dual-disposition manifest-vs-disk sha256 walk; hard-fail on missing/incorrect disposition (VENDOR-03)
- [ ] Anti-vacuity tamper test: flip/remove a manifest entry or a sidecar SPDX/NOTICE → `make verify-licenses` goes RED (assert-RED)
- [ ] SPDX headers ONLY on non-executed sidecar/doc files (never inlined into fixtures the bench executes — would break byte-reproducibility / invalid JSON)

*Existing infrastructure (`verify_licenses.go` strict-decode gate, `LICENSE-AUDIT.md`, `pin.go` isHexSHA1/PinnedSHA, `bench/LICENSES.md`) covers most of the gate machinery — extend, don't rebuild.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none expected) | — | Vendoring + gate are fully automatable | — |

*All phase behaviors should have automated verification (the vendored bytes are committed; the gate is deterministic).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
