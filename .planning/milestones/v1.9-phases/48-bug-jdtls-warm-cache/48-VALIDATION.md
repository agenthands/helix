---
phase: 48
slug: bug-jdtls-warm-cache
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-24
---

# Phase 48 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — Go builtin |
| **Quick run command** | `go test ./test/integration/ -run 'TestSymbols_JavaFixture\|TestEdit_JavaFixture' -count=1` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~60–180 s cold, ~15–30 s warm |

---

## Sampling Rate

- **After every task commit:** `go vet ./... && go test ./<package-touched>/... -count=1`
- **After every plan wave:** `go test ./...`
- **Before `/gsd-verify-work`:** Full suite green; `make bench-jdtls-warm` output pasted into phase review
- **Max feedback latency:** 180 s

---

## Per-Task Verification Map

*Planner populates this table. Each task must verify through one of:*

- **unit:** `go test ./internal/<pkg> -run <TestName> -count=1`
- **integration:** `go test ./test/integration/ -run <TestName> -count=1`
- **build:** `go build ./cmd/serena`
- **static:** `go vet ./...`
- **manual:** recorded in Manual-Only Verifications table below

Required coverage:

| Requirement | Secure Behavior | Covering Test(s) |
|-------------|-----------------|------------------|
| BUG-03 (cache-key stability) | Same fixture + jdtls ⇒ same dir name | Unit test on key helper |
| BUG-03 (cold→warm speedup) | Second run measurably faster | `make bench-jdtls-warm` wall-clocks in review |
| BUG-03 (cache-miss fallback) | Missing/new cache ⇒ cold start, no crash | Integration test deleting cache dir between runs |
| BUG-03 (default `go test ./...`) | Java suite runs without `-tags integration` | `go test ./test/integration/ -run TestSymbols_JavaFixture` with no tags |
| BUG-03 (jdtls absent) | `requireLS` clean skip preserved | Integration test with PATH stripped of jdtls |
| BUG-03 (CI warm cache) | `actions/cache` hit on second workflow run | CI log inspection in phase review |

---

## Wave 0 Requirements

- [ ] Verify `go test ./test/integration/` compiles with the current tag set before any changes (baseline)
- [ ] Confirm `jdtls` on PATH in developer environment (or record `requireLS`-skip baseline)

*If jdtls not locally installed, the warm-cache logic still compiles; the speedup assertion moves to CI.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cold vs warm wall-clock delta | BUG-03 | Absolute timings are machine-dependent | Run `make bench-jdtls-warm`; paste both wall-clocks into phase review |
| CI cache hit/miss | BUG-03 | Requires two consecutive workflow runs | Trigger workflow twice; confirm `actions/cache` reports HIT on second run |
| `make clean-jdtls-cache` | BUG-03 (D-07) | Filesystem side effect under `$XDG_CACHE_HOME` | Run target; confirm `$XDG_CACHE_HOME/serena-test/jdtls/` is absent |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180 s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
