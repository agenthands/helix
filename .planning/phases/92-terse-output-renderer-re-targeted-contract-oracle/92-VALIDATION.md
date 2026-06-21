---
phase: 92
slug: terse-output-renderer-re-targeted-contract-oracle
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-21
---

# Phase 92 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`); CLI stdout goldens |
| **Quick run command** | `go test ./internal/cli/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Goldens / oracle** | `go test ./internal/cli/... -run 'Render|Golden|Terse'` |
| **Behavioral oracle** | `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... -run 'E2E|Chain'` (real subprocess) |
| **Estimated runtime** | ~30–90 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched package(s)
- **After every plan wave:** Run `go vet ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite green + per-verb stdout goldens green + behavioral chaining oracle RAN (HELIX_BIN)
- **Max feedback latency:** ~90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Requirement | Test Type | Automated Command | Status |
|---------|------|-------------|-----------|-------------------|--------|
| 92-01 T1 | 92-01 | OUT-01 (render-class map covers all 50 verbs; unknown → classOpaque) | render unit | `go test ./internal/cli/ -run 'RenderClass|RenderPolicy' -count=1` | ⬜ pending |
| 92-01 T2 | 92-01 | OUT-01/OUT-02 (parseLocusLine both grammars, no coord re-convert; sortDedupLoci deterministic) | render unit | `go test ./internal/cli/ -run 'Locus|SortDedup' -count=5` | ⬜ pending |
| 92-01 T3 | 92-01 | OUT-05 (serr.Kind → per-kind exit code + stderr prefix; spoof guard) | render unit | `go test ./internal/cli/ -run 'ExitCode|ParseKind|Kind' -count=1` | ⬜ pending |
| 92-02 T1 | 92-02 | OUT-01/02/03/06/07 (terse renderer: class dispatch, color gate, sort/dedup, clamped snippet, --abs/--json) | render unit | `go test ./internal/cli/ -run 'Render' -count=5` | ⬜ pending |
| 92-02 T2 | 92-02 | OUT-04/05/06/07 (wire renderer + persistent --color/--abs/--json; per-kind exit codes; cligen denylist + regen) | unit + cligen | `go test ./internal/cli/... ./cmd/helix-cligen/... -count=1 && go run ./cmd/helix-cligen --check && go vet ./...` | ⬜ pending |
| 92-03 T1 | 92-03 | TEST-02 (contract oracle re-targeted to CLI stdout goldens; typed-args→cobra-flags parity) | golden + parity | `go test ./test/oracle/contract/... -run 'Parity' -count=1 && HELIX_BIN=<built> go test -tags integration -run TestGolden -count=1 -timeout 5m ./test/oracle/contract/...` | ⬜ pending |
| 92-03 T2 | 92-03 | OUT-04/OUT-03 (behavioral chain: nav locus feeds downstream verb verbatim; nav output self-contained snippet) | behavioral e2e | `HELIX_BIN=<built> go test -run 'Chain|NavSelfContained' -count=1 ./internal/cli/...` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Every task above has an `<automated>` verify in its plan; no task relies on manual-only verification. The single manual check below is supplementary (real-TTY color), not a substitute.*

---

## Wave 0 Requirements

- [x] per-verb CLI stdout golden fixtures re-targeted from the MCP JSON goldens — owned by 92-03 T1 (regenerate `test/oracle/contract/testdata/golden/*` with `GOLDEN_UPDATE=1` against the frozen renderer)
- [x] render-class map test scaffold (locus-list / tree / opaque per tool) — owned by 92-01 T1 (`internal/cli/render_policy_test.go`, coverage test iterates `verbSpecs`)
- [x] error-kind → stderr-prefix + exit-code table test scaffold — owned by 92-01 T3 (`internal/cli/exitcode_test.go`, table-driven over the 9 kinds)
- [x] locus parse + sort/dedup test scaffold (both daemon grammars; determinism) — owned by 92-01 T2 (`internal/cli/locus_test.go`, `-count=5`)
- [x] terse renderer test scaffold (class dispatch, color/abs/json, snippet + traversal clamp) — owned by 92-02 T1 (`internal/cli/render_test.go`)
- [x] typed-args→cobra-flags parity test scaffold — owned by 92-03 T1 (`test/oracle/contract/cli_parity_test.go`, plain untagged unit test)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real-TTY color emission | OUT-04 | Genuine TTY behavior is hard to assert in CI (tests assert the piped/no-color path); a quick manual check confirms color renders on a real terminal | Run a verb in an interactive terminal, confirm color; pipe it, confirm zero ANSI |

*All other phase behaviors have automated verification. The piped/no-color path (OUT-01/06) IS asserted automatically (zero-0x1b-byte assertion in 92-02 T1); only the real-TTY positive case is manual.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-21
