---
phase: 79
slug: evaluators-result-schema-metrics-layer
status: verified
threats_open: 0
asvs_level: 1
created: 2026-06-18
---

# Phase 79 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Source: State B (built from the 4 PLAN.md `<threat_model>` blocks + SUMMARY threat flags).
> All `mitigate`-disposition controls were verified present in the shipped code (greps recorded in the audit trail). No `high`-severity threats. ASVS L1.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| repo working tree → `git`/`go` subprocess | The cell's repo dir (mutated by the agent under test) is the input to `git ls-files`/`git diff` and the test double-run. Repo content (filenames, diff hunks) is untrusted relative to the grader. | filenames, diff hunks, test output |
| agent CLI subprocess stream → token_meter | The CC stream-json (and its `usage` block) originates from the agent-under-test's subprocess. The provider `usage` is the trusted token source-of-truth (METRIC-03); daemon-side counters are NOT trusted as a token source. | provider `usage` block (token counts) |
| daemon OTel log → tool_trace_analyzer | The merged trace already passed the Phase 67 PID-gate (`TapDaemonLog`); the analyzer only reads the merged object, introducing no new tap. | merged span/event record |
| cell metadata (task/mode/run_index) → durable path | The `run_index` is now part of the durable filesystem path and must not enable path traversal. | path segment |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-79-01-01 | Tampering | `result.v2.schema.json` constraint change | accept | Additive-only edit; only `tokens_input/output` RELAXED, no existing field tightened; old fixtures proven valid by the additive-only regression test (`bench/schema/result.v2_test.go`). | closed |
| T-79-01-SC | Tampering | package-manager installs | n/a | Zero new external packages (stdlib + already-present `jsonschema/v6 v6.0.2`). | closed |
| T-79-02-01 | Tampering | `git` argv in patch_validator / regression_checker | mitigate | `exec.CommandContext` with FIXED argv; `cmd.Dir = repoDir`; no shell. **Verified:** no `sh -c`/`bash -c` in `bench/evaluators/`; `patch_validator.go:168` uses `exec.CommandContext(ctx, "git", args...)`. | closed |
| T-79-02-02 | Tampering | patch path escaping `repoDir` | mitigate | Modified paths resolved under `repoDir` before counting (path-prefix invariant). **Verified:** `underRepo(repoDir, p)` guard at `patch_validator.go:80,151`. | closed |
| T-79-02-03 | Denial of Service | runaway `go test` in regression double-run | accept | Go ToolBench fixtures are tiny single-module repos (D-05); double-run cost negligible. Caller ctx cancellation/timeout honored as infra error, not a hang. External-scale cost deferred to Phase 87 (D-05). | closed |
| T-79-02-SC | Tampering | package-manager installs | n/a | Zero new external packages. | closed |
| T-79-03-01 | Spoofing | MCP/daemon counter masquerading as provider usage | mitigate | `token_meter` reads ONLY `mt.Usage.{InputTokens,OutputTokens,CacheReadTokens,CacheCreationTokens}` — never a daemon/MCP counter. **Verified:** `token_meter.go:55-58`; no `ToolCallSummary`/byte-counter read for token values. | closed |
| T-79-03-02 | Tampering | re-merge divergence / cross-cell PID leakage | mitigate | `tool_trace_analyzer` consumes the already-merged `MergedTrace`; does NOT re-merge (preserves the proven `RejectedForeignPid==0` invariant). **Verified:** no `trace.Merge(` call in `tool_trace_analyzer.go`. | closed |
| T-79-03-SC | Tampering | package-manager installs | n/a | Zero new external packages; `internal/eval/trace` is in-repo. | closed |
| T-79-04-01 | Tampering | `run_index` path segment in `RunCell` durable path | mitigate | `run_index` formatted via `strconv.Itoa`, routed through `validateRunIndexSegment` → `validatePathSegment` (rejects `..`/separators/leading dots) BEFORE `filepath.Join`. **Verified:** `cell.go:161-164`, `validate.go:23`. | closed |
| T-79-04-02 | Tampering | `git`/`go` exec via the coordinator | mitigate | Inherited from T-79-02-01: fixed argv, `cmd.Dir`, no shell. Coordinator adds no new exec surface (delegates to already-mitigated graders). | closed |
| T-79-04-03 | Information Disclosure | `metric_errors` free-text reason leaking repo paths | accept | `MetricError.Reason` is a short grader-authored diagnostic string, not interpolated untrusted repo content; `result.v2.json` is a local bench artifact, no PII, low-value target. | closed |
| T-79-04-SC | Tampering | package-manager installs | n/a | Zero new external packages. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-79-01 | T-79-01-01 | Additive-only schema change relaxes (never tightens) existing fields; no committed result can become retroactively invalid; proven by additive-only regression test. | secure-phase (plan-time disposition) | 2026-06-18 |
| AR-79-02 | T-79-02-03 | Tiny single-module fixtures make the regression double-run cost negligible; external-scale strategy explicitly deferred to Phase 87 (D-05). | secure-phase (plan-time disposition) | 2026-06-18 |
| AR-79-03 | T-79-04-03 | `metric_errors.reason` is a bounded grader-authored diagnostic, not untrusted repo content; the artifact is local and PII-free. | secure-phase (plan-time disposition) | 2026-06-18 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-06-18 | 13 | 13 | 0 | secure-phase (orchestrator, plan-time register + source verification) |

Notes: register built from all 4 PLAN.md `<threat_model>` blocks (`register_authored_at_plan_time: true`); `threats_open: 0` with the register authored at plan time → short-circuit verification (no auditor scan). All 6 `mitigate` controls verified present in shipped source via grep (recorded inline in the Threat Register). No `high`-severity threats. Cross-references: `79-REVIEW.md` (independent adversarial security pass — same conclusion: no traversal/injection surface) and `79-VERIFICATION.md`.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-06-18
