---
phase: 59
slug: tree-sitter-extraction-stable-symbol-ids
audited: 2026-05-04
asvs_level: L1
block_on: HIGH
verifier: gsd-security-auditor
threats_total: 18
threats_closed: 16
threats_transferred: 2
threats_open: 0
status: SECURED
---

# Phase 59 — Security Audit

Verification of every threat in the per-plan `<threat_model>` blocks against the implemented code. Implementation files were not modified; this report is the only artifact emitted.

Adversarial stance: each declared mitigation was treated as absent until a grep / file:line match in the right location proved it. `accept` dispositions were treated as closed only when the rationale is documented in the relevant SUMMARY (operator-facing telemetry, public symbol surface, etc.).

## Verdict: SECURED — 16 closed, 2 transferred to Phase 60

Initial audit returned `OPEN_THREATS` for two DoS gates declared in plan 59-04 (T-59-04-01 per-file parse timeout, T-59-04-03 file-size gate). Both gates were declared with handoffs ("P03 owns the gate" / "P03 owns the per-file timeout"), but plan 59-03 ships only the orchestration surface — `ScheduleInitialExtraction` admits a job and transitions state, while the actual file-walk loop that would invoke `Provider.Extract(ctx, source, file)` does not exist in the merged tree. `grep -rn "Provider.Extract\|\.Extract(ctx" internal/semantic/ internal/daemon/ internal/kernel/` returns no hits in non-`_test.go` code.

**Disposition (operator decision, 2026-05-04):** Both threats are transferred to Phase 60. The DoS surface is dormant in the current tree — there is no production call site to gate today. Phase 60 will land the file-walk worker that opens files and invokes `Provider.Extract`, which is the natural and only enforcement point for both `extraction_file_timeout` and `MaxFileSize`. Phase 60's PLAN.md MUST inherit T-59-04-01 and T-59-04-03 as acceptance criteria.

## Threat verification table

### CLOSED (16/18)

| Threat ID | Category | Disposition | Evidence |
|-----------|----------|-------------|----------|
| T-59-01-01 | Tampering (forward-incompat schema) | mitigate | `internal/semantic/store/migrations_registry_cgo.go:55-57` returns wrapped `ErrForwardIncompatible` when `currentVersion > CurrentSchemaVersion`; `internal/semantic/store/duckdb.go:126` propagates the sentinel via `errors.Is` rather than quarantining. `TestMigration_ForwardIncompatible` exercises the path. |
| T-59-01-02 | DoS (partially-applied migration) | mitigate | `internal/semantic/store/migrations_registry_cgo.go:73-80` re-reads `schema_version` after each `m.Apply` and returns an explicit error if the body forgot to stamp the new version. |
| T-59-01-03 | InfoDisc (error_message column) | accept | Operator-facing telemetry; rationale documented in 59-01-SUMMARY "Threat Model Coverage" table. No PII flow defined. Accepted risk recorded below. |
| T-59-02-01 | Tampering (NewExtractorRegistry nil/dup) | mitigate | `internal/semantic/extract/registry.go:28-29` panics on nil `*treesitter.GrammarRegistry`; lines 36-37 panic on duplicate `Provider.Language()`. `TestRegistry_NilGrammarPanics` and `TestRegistry_DuplicateLanguagePanics` exercise both paths. |
| T-59-02-02 | DoS (metric label cardinality) | mitigate | `internal/obs/metrics.go` `SemanticExtractionTotal` helper uses closed `allowedExtractionLanguages` / `allowedExtractionOutcomes` allowlists (4×4 = 16 combos); unknown language coerced to `"other"`, unknown outcome dropped. `TestSemanticExtractionTotal_BoundedLabels` exercises both paths. |
| T-59-02-03 | Tampering (StableSymbolKey field order) | mitigate | `TestCanonicalizeStableSymbolKey_FieldOrder` asserts NUL-byte count and field positioning; `TestCanonicalize_KnownVector` pins one full key to `0x032208087c467dcd`. Reordering fields drifts the frozen vector and fails CI. |
| T-59-02-04 | InfoDisc (confidence ladder) | accept | Confidence values are non-secret operator/agent-facing telemetry; rationale documented in 59-02-SUMMARY. Accepted risk recorded below. |
| T-59-03-01 | DoS (RequireReady wait loop) | mitigate | `internal/semantic/scheduler/ready.go:126-148` sets `time.NewTimer(policy.Timeout)` and a `select` with `<-ctx.Done()`, `<-timer.C`, and `<-sub` arms. `DefaultReadyPolicy` returns `Timeout=30 * time.Second` (line 53). On timer fire (line 133-135), returns `cur.toResult(cur.FilesDone > 0), nil` (best-partial). The `defer timer.Stop()` at line 127 prevents leak. Tests `TestRequireReady_TimeoutReturnsBestPartial` and `TestRequireReady_ContextCancel` exercise both escape paths. |
| T-59-03-02 | DoS (Subscribe channel) | mitigate | `internal/semantic/scheduler/scheduler.go:106` allocates `make(chan SemanticStatus, 8)` (cap 8). Lines 119-122 publish via `select { case sub <- st: default: }` (non-blocking; drop on full subscriber). |
| T-59-03-03 | Tampering (test-only state mutators) | mitigate | `TestSetStatus` and `HasInflightJob` live in `internal/semantic/scheduler/export_test.go` (the `_test.go` suffix gates them to test compilation). Production source files (`scheduler.go`, `ready.go`, `state.go`, `priority.go`, `doc.go`) contain no `TestSetStatus` reference; verified by grep. |
| T-59-04-02 | Tampering (queries.scm) | mitigate | All three providers bind queries via `//go:embed queries.scm`: `internal/semantic/extract/golang/provider.go:26-27`, `internal/semantic/extract/typescript/provider.go:29` (similar), `internal/semantic/extract/python/provider.go:21`. Compile-time bound; runtime cannot swap the string. |
| T-59-04-04 | InfoDisc (extracted symbol names) | accept | Symbol names + qualified names are public surface of the workspace's source; rationale documented in 59-04-SUMMARY. Accepted risk recorded below. |
| T-59-05-01 | Tampering (duplicate GrammarRegistry) | mitigate | Dual gate: (a) runtime `internal/daemon/daemon_grammar_test.go:TestBootstrap_GrammarRegistrySingleton` asserts pointer-equality across `extract.Registry`, `BodyExtractor`, `RepoMapSkill` and the daemon-owned singleton; (b) static `internal/semantic/extract/registry_grep_test.go:TestNoNewGrammarRegistry` walks `internal/semantic/extract/*` (excluding `_test.go` and `testutil/`) and fails on any `treesitter.NewGrammarRegistry` call site. `internal/daemon/imports.go:13-26` documents the non-blank-import policy with an explicit "do NOT add blank imports" warning. |
| T-59-05-02 | DoS (ActivateWorkspace blocked) | mitigate | `internal/daemon/daemon.go:481-489` invokes `semanticScheduler.ScheduleInitialExtraction(...)` inside the activate callback after `k.ActivateWorkspace(ctx, repoPath)` returns (line 463). `ScheduleInitialExtraction` is non-blocking — it locks the scheduler mutex briefly, records the in-flight job, transitions state, and returns the JobID without launching any walk goroutine. The activate callback returns immediately after the kickoff. |
| T-59-05-03 | DoS (infinite extraction job per workspace) | mitigate | `internal/semantic/scheduler/scheduler.go:67-79` `ScheduleInitialExtraction` checks `s.jobs[ws]` first and returns the existing JobID on duplicate admission. Tests `TestScheduler_Idempotent` and `TestScheduler_OneJobPerWorkspace` exercise the invariant. The daemon's repeated activations of the same workspace therefore cannot multiply jobs. |
| T-59-05-04 | InfoDisc (scheduler error surface) | accept | Operator-facing telemetry only; rationale documented in 59-05-SUMMARY. Accepted risk recorded below. |

### TRANSFERRED to Phase 60 (2/18)

These threats describe DoS gates whose only enforcement point is a file-walk worker that does not exist in the Phase 59 merged tree. Phase 60 will land the worker; both threats are inherited as Phase 60 acceptance criteria.

| Threat ID | Category | Mitigation Expected | Why deferred | Phase 60 acceptance |
|-----------|----------|---------------------|--------------|---------------------|
| T-59-04-01 | DoS (tree-sitter parser timeout) | `extraction_file_timeout` (3s default) injected as deadline-bounded `ctx` into `Provider.Extract` from a scheduler-owned worker; on `ctx.Err()` provider emits `PartialReasonTimeout`. | No production caller of `Provider.Extract` exists today (`grep -rn "\.Extract(ctx" internal/semantic/ internal/daemon/ internal/kernel/ \| grep -v _test.go` returns 0 matches). Provider-side `ctx.Err()` checks (e.g., `golang/provider.go:111-113`) are wired but cannot fire because no parent supplies a deadline. The post-merge wiring commit `9f1a623f` added only the `ScheduleInitialExtraction` kickoff, not the file-walk loop. | Phase 60 PLAN MUST wrap each `Provider.Extract` invocation in `context.WithTimeout(parent, cfg.SemanticIndex.Extraction.ExtractionFileTimeout)` and verify the timeout path fires `PartialReasonTimeout`. |
| T-59-04-03 | DoS (huge file size gate) | `cfg.SemanticIndex.Indexing.MaxFileSize` (2 MiB default) enforced before parser invocation; over-budget files emit `PartialReasonFileTooLarge` and skip parsing. | Same root cause as T-59-04-01 — no caller exists to perform the pre-flight size check. `PartialReasonFileTooLarge` is defined (`fact.go:49`) and mapped (`partial.go:28`) but no caller emits it. | Phase 60 PLAN MUST stat each file before extraction, emit `PartialReasonFileTooLarge` and skip parsing when `info.Size() > cfg.SemanticIndex.Indexing.MaxFileSize`. |

## Threat Flags from SUMMARY

Plan SUMMARYs do not declare a `## Threat Flags` section per the project template. No unregistered_flag entries.

## Accepted risks log

| Threat ID | Category | Component | Rationale |
|-----------|----------|-----------|-----------|
| T-59-01-03 | Information Disclosure | `semantic_files.error_message` column | Operator-facing telemetry. Extractor errors are file paths and parse-error positions. No PII flow defined; column is nullable, populated only on extraction failure. Documented in 59-01-SUMMARY. |
| T-59-02-04 | Information Disclosure | confidence ladder constants | Confidence values are non-secret operator/agent-facing telemetry. Documented in 59-02-SUMMARY. |
| T-59-04-04 | Information Disclosure | extracted symbol names | Symbol names and qualified names are the public API surface of the workspace's source code. No PII redaction needed. Documented in 59-04-SUMMARY. |
| T-59-05-04 | Information Disclosure | scheduler error surface (`scheduler.Status`) | Operator-facing telemetry. `IndexError.Reason` matches the closed `PartialReason` enum; `IndexError.Message` flows through the same operator-only surface as the `error_message` DB column. Documented in 59-05-SUMMARY. |

## Audit trail

### Security Audit 2026-05-04

| Metric | Count |
|--------|-------|
| Threats found | 18 |
| Closed | 16 |
| Transferred to Phase 60 | 2 |
| Open | 0 |

- Auditor (gsd-security-auditor) returned `OPEN_THREATS` with T-59-04-01 and T-59-04-03 as BLOCKER.
- Operator review confirmed dormant DoS surface: zero production callers of `Provider.Extract` in the merged tree.
- Disposition: transferred to Phase 60 pending file-walk worker implementation. Phase 60's PLAN.md must inherit both as acceptance criteria.
- Recommend updating 59-04-SUMMARY and 59-05-SUMMARY to retract "P03 owns the gate" / "deferred to merge" handoff claims and replace with explicit "deferred to Phase 60" pointers.

## Appendix: verification commands

```bash
# T-59-04-01 / T-59-04-03 negative grep (should return matches in production code; returns nothing today):
grep -rn "ExtractionFileTimeout\|MaxFileSize\|context.WithTimeout" \
  internal/semantic/scheduler/ internal/daemon/ \
  --include='*.go' \
  | grep -v _test.go

# Provider.Extract production caller check (expected: 0 matches until Phase 60 lands):
grep -rn "\.Extract(ctx" internal/semantic/ internal/daemon/ internal/kernel/ \
  --include='*.go' | grep -v _test.go

# T-59-05-02 callback non-blocking (matches expected):
grep -nE 'ScheduleInitialExtraction|SetActivateCallback' internal/daemon/daemon.go

# T-59-05-03 idempotency (matches expected):
sed -n '67,79p' internal/semantic/scheduler/scheduler.go

# T-59-05-01 dual gate (both tests expected to pass):
go test ./internal/daemon/ -run TestBootstrap_GrammarRegistrySingleton -count=1
go test ./internal/semantic/extract/ -run TestNoNewGrammarRegistry -count=1

# T-59-03-03 test-only mutator gating:
grep -n "TestSetStatus\|HasInflightJob" internal/semantic/scheduler/*.go \
  | grep -v _test.go    # expected: 0 matches
```
