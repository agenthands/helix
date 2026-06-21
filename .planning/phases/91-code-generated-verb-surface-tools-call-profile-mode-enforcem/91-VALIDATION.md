---
phase: 91
slug: code-generated-verb-surface-tools-call-profile-mode-enforcement
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-21
---

# Phase 91 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`); generator `cmd/helix-cligen` with `--check` drift gate |
| **Config file** | none — repo `go.mod` |
| **Quick run command** | `go test ./internal/cli/... ./internal/mcp/...` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Drift gate** | `go run ./cmd/helix-cligen --check` (exit non-zero on drift) |
| **Profile goldens** | `go test -tags integration ./test/integration/ -run Profile` (CLI verb-surface oracle) |
| **HELIX_BIN E2E** | `go build -o /tmp/helix-91 ./cmd/helix && HELIX_BIN=/tmp/helix-91 go test ./internal/cli/ -run 'TestCLI_Sec' -count=1` |
| **Estimated runtime** | ~30–120 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test` on the touched package(s)
- **After every plan wave:** Run `go vet ./... && go test ./...` plus `go run ./cmd/helix-cligen --check`
- **Before `/gsd-verify-work`:** Full suite green + drift gate green + profile goldens green + the HELIX_BIN-gated SEC-01 E2E RAN (not skipped)
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Requirement | Test Type | Automated Command | Status |
|---------|------|-------------|-----------|-------------------|--------|
| 91-01-T1 | 91-01 | VERB-04 (AST scan recovers tool-name → *Args binding for every registered tool) | generator unit | `go test ./cmd/helix-cligen/ -run ScanAllTools -count=1` | ⬜ pending |
| 91-01-T2 | 91-01 | VERB-01/02/03/04 (render gofmt-stable verbs_gen.go + commit + `--check` drift gate + VerbToolNames() accessor) | generator unit + build | `go run ./cmd/helix-cligen --check && go build ./cmd/helix ./cmd/helix-cligen && go test ./cmd/helix-cligen/ -count=1` | ⬜ pending |
| 91-01-T3 | 91-01 | VERB-01/02/03 (parity by name + VerbToolNames coverage + flatten smoke + `make verify-cligen` + CI) | parity unit | `go test ./internal/cli/ -run 'Verbs\|Parity\|Flatten\|RequiredBeforeDial\|VerbToolNames' -count=1 && make verify-cligen` | ⬜ pending |
| 91-02-T1 | 91-02 | SEC-01 (ProfileEnforcementMiddleware refuse/allow/passthrough + typed PermissionDenied round-trip) | middleware unit | `go test ./internal/mcp/ -run 'ProfileEnforce' -count=1 && go vet ./internal/mcp/` | ⬜ pending |
| 91-02-T2 | 91-02 | SEC-01 (install AFTER Guardrail / BEFORE LazyInit; LazyInit-first LIFO preserved) | wiring unit | `go test ./internal/mcp/ -run 'ProfileEnforce\|LIFO\|Order' -count=1 && go build ./internal/daemon/ && go vet ./...` | ⬜ pending |
| 91-03-T1 | 91-03 | SEC-02 (CLI verb-surface helper via cli.VerbToolNames() ∩ profile allowed set) | integration build | `go build -tags integration ./test/integration/ && go vet ./test/integration/` | ⬜ pending |
| 91-03-T2 | 91-03 | SEC-02 (re-point Profile_Contract_Golden to CLI verb surface; hidden AND refused) | golden | `go test -tags integration ./test/integration/ -run 'Profile_Contract\|CLI_Surface' -count=1` | ⬜ pending |
| 91-04-T1 | 91-04 | VERB-03 (re-point Phase 90 E2E oracle from `helix call <verb>` to flat `helix <verb>`) | E2E (HELIX_BIN) | `go build -o /tmp/helix-91 ./cmd/helix && HELIX_BIN=/tmp/helix-91 go test ./internal/cli/ -run 'TestCLI_E2E_OneShot\|TestCLI_WarmReuseSLO' -count=1` | ⬜ pending |
| 91-04-T2 | 91-04 | SEC-01 (live: read-mode refuses a destructive verb non-zero+typed; edit-mode allows) | E2E (HELIX_BIN) | `go build -o /tmp/helix-91 ./cmd/helix && HELIX_BIN=/tmp/helix-91 go test ./internal/cli/ -run 'TestCLI_SecRefusal\|TestCLI_SecAllow' -count=1 -v` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*
*Task IDs finalized by the planner; this map is the requirement→test contract.*

### Requirement Coverage Cross-Check

| Req ID | Covered By |
|--------|-----------|
| VERB-01 | 91-01-T2, 91-01-T3 |
| VERB-02 | 91-01-T2, 91-01-T3 |
| VERB-03 | 91-01-T2, 91-01-T3, 91-04-T1 |
| VERB-04 | 91-01-T1, 91-01-T2 |
| SEC-01 | 91-02-T1, 91-02-T2, 91-04-T2 |
| SEC-02 | 91-03-T1, 91-03-T2 |

---

## Wave 0 Requirements

Every code-producing task has a co-located test file authored as part of the same plan (TDD plans 91-01 and 91-02 write the failing test first; the execute plans 91-03/91-04 are themselves test files). No task carries a `MISSING` `<automated>` gate.

- [x] `cmd/helix-cligen/scan_test.go` — AST scan recovers name↔args for all tools (authored in 91-01-T1, RED-first)
- [x] `internal/cli/verbs_gen_test.go` — VERB-01 parity (count by name), VerbToolNames coverage, VERB-04 (every verb has resolved flags) (91-01-T3)
- [x] `internal/mcp/profile_enforce_test.go` — SEC-01 refuse/allow + typed-error (`errors.Is(PermissionDenied)`) + LIFO order (91-02-T1/T2, mirrors `guardrail_middleware_test.go:221-258`)
- [x] `test/integration/cli_verb_surface.go` + re-pointed `test/integration/profile_golden_test.go` — SEC-02 CLI verb-surface oracle (91-03-T1/T2)
- [x] `internal/cli/cli_sec_e2e_test.go` + re-pointed `internal/cli/cli_e2e_test.go` — `HELIX_BIN`-gated `CLI_SecRefusal`/`CLI_SecAllow` (91-04-T2) and flat-verb oracle re-point (91-04-T1)
- [x] `make verify-cligen` target + CI wiring for the `--check` drift gate (91-01-T3)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (none) | — | — | The SEC-01 live refusal — previously listed as manual-only — is now an automated `HELIX_BIN`-gated E2E (91-04-T2: `TestCLI_SecRefusal_ReadMode` / `TestCLI_SecAllow_EditMode`), so no behavior is manual-only. The phase gate requires that test to RUN (not skip) before `/gsd-verify-work`. |

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-21
