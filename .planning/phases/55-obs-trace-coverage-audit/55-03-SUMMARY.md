---
phase: 55-obs-trace-coverage-audit
plan: 03
subsystem: observability
tags: [observability, tracing, documentation, usage, audit, human-uat]

requires:
  - phase: 55-01-trace-coverage-gap-fix
    provides: ls.request child span + skill.tool.{name} child span (final span shape documented in TRACE-AUDIT.md)
  - phase: 55-02-coverage-and-allowlist
    provides: closed allowedSpanAttrs map (verbatim certified by TRACE-AUDIT.md)
provides:
  - TRACE-AUDIT.md — human-readable certification of all four span names + 8 attribute keys (no PII, bounded cardinality, no client trace-context propagation)
  - USAGE.md Sampling subsection (head-vs-tail honesty + ratio recommendation table + ObservabilityConfig pointer)
  - USAGE.md Smoke-Testing the Pipeline subsection (otel-collector-contrib docker recipe + Serena config)
  - USAGE.md Trace Coverage subsection (cross-references to coverage and allowlist CI gates)
  - docs/usage_test.go::TestUsageDocumentsTracingSampling — regression gate for the sampling docs
  - 55-HUMAN-UAT.md — Phase-54-shaped UAT ticket for the live OTLP smoke trace (criterion #4 evidence capture pending human run)
affects: [55-VERIFICATION.md]

tech-stack:
  added: []
  patterns:
    - "USAGE.md regression-gate test pattern (docs/usage_test.go) — mirror of TestUSAGEObservability with subtests per required substring"
    - "HUMAN-UAT.md Phase-54 shape — frontmatter + Tests with expected/result + Summary counters + Gaps + Operator Setup Notes"

key-files:
  created:
    - .planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md
    - .planning/phases/55-obs-trace-coverage-audit/55-HUMAN-UAT.md
    - .planning/phases/55-obs-trace-coverage-audit/55-03-SUMMARY.md
  modified:
    - USAGE.md
    - docs/usage_test.go

key-decisions:
  - "TRACE-AUDIT.md as single human-readable cert artifact paralleling the mechanical allowlist test (no separate PII / cardinality docs to keep in sync)"
  - "Smoke-test recipe lives in USAGE.md only; HUMAN-UAT.md references it (single source of truth)"
  - "TestUsageDocumentsTracingSampling uses subtests per substring so failures name the missing piece"
  - "Reused the existing docs/usage_test.go file rather than creating a parallel sampling_test.go (extends TestUSAGEObservability pattern in place)"
  - "Smoke-test recipe pins otel/opentelemetry-collector-contrib:0.118.0 with a documented `logging` fallback for older images"

patterns-established:
  - "Trace-context propagation policy explicitly documented as a deliberate property (no inbound client trace IDs accepted) — re-evaluate when authenticated MCP transport lands"
  - "Anti-list of forbidden attribute keys (workspace_path, file_path, symbol_name, user_*, request_id, free-form errors) with bucketing/hashing as the canonical mitigation pattern"

requirements-completed: [OBS-04]

duration: 25min
completed: 2026-04-28
---

# Phase 55 Plan 03: Trace doc and HUMAN-UAT Summary

**Three operator-facing deliverables close out OBS-04 success criteria #2, #3, and #4: the TRACE-AUDIT.md attribute certification, the USAGE.md sampling + smoke-test guidance (with a regression test gating it), and the 55-HUMAN-UAT.md ticket capturing the live OTLP smoke trace evidence.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-04-28T11:30:00Z (approx)
- **Completed:** 2026-04-28T11:55:00Z (approx)
- **Tasks:** 3 autonomous (Tasks 1-3) + 1 human checkpoint (Task 4 returned for operator action)
- **Files created:** 3 (TRACE-AUDIT.md, 55-HUMAN-UAT.md, this SUMMARY)
- **Files modified:** 2 (USAGE.md, docs/usage_test.go)

## Accomplishments

- **OBS-04 #2 (criterion):** TRACE-AUDIT.md (165 lines) certifies the entire span surface — 4 span names, 8 allowlisted attribute keys — with cardinality math (~364K worst-case parent fingerprints, ≤50 child names, 1,560 ls.request fingerprints), explicit anti-list of forbidden attribute keys, and a documented "no inbound trace context propagation" policy.
- **OBS-04 #3 (criterion):** USAGE.md tracing section grew from 11 lines (line range 772-782 pre-change) to 109 lines covering ParentBased+TraceIDRatioBased explanation, recommended ratios per environment, head-vs-tail sampling honesty, the dev-only otel-collector-contrib smoke recipe (YAML + docker run + Serena config), and the Trace Coverage subsection cross-referencing the CI gates and TRACE-AUDIT.md.
- **OBS-04 #4 (criterion setup):** 55-HUMAN-UAT.md (107 lines, status `pending`) gives the operator two concrete verification scenarios — full LS-call path (`daemon.mcp.tools.call → kernel.tool.find_symbol → ls.request`) and skill-tool path (`daemon.mcp.tools.call → skill.tool.<name>`) — plus operator setup notes covering the SIGTERM-not-SIGKILL caveat, the `debug`/`logging` exporter rename, and a redaction step for the `result:` evidence.
- **Regression gate:** `TestUsageDocumentsTracingSampling` (in `docs/usage_test.go`, package `docs_test`) asserts via subtests that USAGE.md contains `tracing_sample_ratio`, `1.0`, `ObservabilityConfig`, `otel/opentelemetry-collector-contrib`, `TRACE-AUDIT.md`, and the head/tail sampling discussion. Manually verified RED-then-GREEN: the initial test failed on the three new substrings before the doc edit; after the edit, all 7 subtests pass.

## TRACE-AUDIT.md Final Counts

- **Span Inventory table rows:** 4 (`daemon.mcp.tools.call`, `kernel.tool.<name>`, `skill.tool.<name>`, `ls.request`).
- **Attribute Certification subsections:** 4 (one per span name).
- **Total certified attribute keys:** 8 — five on `daemon.mcp.tools.call` (`tool_name`, `profile`, `mode`, `language`, `outcome`) + three on `ls.request` (`lsp.method`, `lsp.language`, `lsp.duration_ms`); zero on `kernel.tool.*` and `skill.tool.*` by D-07.
- **Anti-list forbidden keys / patterns:** 6 (`workspace_path`/`repo_root`, `file_path`/`file_uri`, `symbol_name`, `user_id`/`user_email`/`session_id`, `request_id`/`trace_id`, free-form `error` strings).
- **Worst-case cardinality (computed):** 50 × 5 × 4 × 52 × 7 ≈ **364,000** parent span fingerprints; 30 × 52 = **1,560** ls.request fingerprints.
- **Mechanical Enforcement References:** 6 file:line citations (coverage test, exhaustiveness test, integration test, plan-55-01 span shape regression tests, sampler source, shutdown flush source).

## USAGE.md Sections Added/Expanded

- **Pre-change:** Tracing subsection at lines 772-782 (11 lines, 1 YAML block, no sampling guidance, no smoke-test recipe).
- **Post-change:** Tracing subsection now spans approximately lines 772-880 (109 lines, 4 YAML blocks: original example, ratio-table example, otel-collector YAML, daemon-side YAML). New subsections:
  - `#### Sampling` — ParentBased explanation, recommended-ratio table (Default 0.0 / Smoke 1.0 / Production 0.01-0.10), ObservabilityConfig pointer, head-vs-tail honesty paragraph with otel-collector tail_sampling reference.
  - `#### Smoke-Testing the Pipeline` — otel-collector-config.yaml block + docker run + matching Serena `observability:` block + SIGTERM caveat.
  - `#### Trace Coverage` — cross-references to `TestEveryRegisteredToolEmitsSpans`, `TestSpanAttributeAllowlist`, and TRACE-AUDIT.md.
- **`tracing_sample_ratio` mentions:** 6 (sampling table, head-only paragraph, smoke recipe daemon block, plus the existing config-key documentation at line 343).
- **`otel/opentelemetry-collector-contrib` mentions:** 1 (smoke recipe — single source of truth).

## TestUsageDocumentsTracingSampling Evidence

```
=== RUN   TestUsageDocumentsTracingSampling
=== RUN   TestUsageDocumentsTracingSampling/tracing_sample_ratio
=== RUN   TestUsageDocumentsTracingSampling/1.0
=== RUN   TestUsageDocumentsTracingSampling/ObservabilityConfig
=== RUN   TestUsageDocumentsTracingSampling/otel/opentelemetry-collector-contrib
=== RUN   TestUsageDocumentsTracingSampling/TRACE-AUDIT.md
=== RUN   TestUsageDocumentsTracingSampling/honesty:head
=== RUN   TestUsageDocumentsTracingSampling/honesty:tail
--- PASS: TestUsageDocumentsTracingSampling (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/tracing_sample_ratio (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/1.0 (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/ObservabilityConfig (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/otel/opentelemetry-collector-contrib (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/TRACE-AUDIT.md (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/honesty:head (0.00s)
    --- PASS: TestUsageDocumentsTracingSampling/honesty:tail (0.00s)
PASS
ok  	github.com/postfix/serena/docs	0.273s
```

(7/7 subtests green. Manual RED phase prior to the USAGE.md edit failed on `ObservabilityConfig`, `otel/opentelemetry-collector-contrib`, and `TRACE-AUDIT.md` — the three new substrings — exactly as designed; head/tail keywords were already incidentally present.)

## 55-HUMAN-UAT.md Status

`status: pending` — operator-side verification awaits Task 4 (the human-verify checkpoint). The file contains two scenarios both marked `result: [pending]`; the Summary block reads `total: 2 / passed: 0 / pending: 2`. The phase verification step cannot mark OBS-04 #4 as passed until a human runs the smoke trace per the procedure in this file's "How to Verify" section (cross-referenced from USAGE.md "Smoke-Testing the Pipeline").

## Task Commits

1. **Task 1: TRACE-AUDIT.md certification (criterion #2)** — `3a76bb2d` (docs)
2. **Task 2: USAGE.md sampling expansion + regression test (criterion #3)** — `7f61f86e` (docs, includes the TDD test and the doc edit in one commit because they were authored together)
3. **Task 3: 55-HUMAN-UAT.md (criterion #4 setup)** — `351d6411` (test, mirrors Phase 54 ticket convention)

## Decisions Made

- **TRACE-AUDIT.md is the single human cert artifact, not a multi-file split.** Considered separating the cardinality math, anti-list, and propagation policy into separate files; a single 165-line doc reads better and matches the "one document an auditor reads instead of grepping tests" goal in the plan objective.
- **Smoke-test recipe lives in USAGE.md, not in 55-HUMAN-UAT.md.** Avoids two-source drift; HUMAN-UAT references USAGE.md and adds operator-specific caveats (SIGTERM, redaction) that are out of scope for the user-facing recipe.
- **Pinned `otel/opentelemetry-collector-contrib:0.118.0`** in the recipe with a documented `logging` fallback for older images. The version pin is best-effort current per A1 in 55-RESEARCH; an operator can swap to any later release.
- **Subtests per substring in `TestUsageDocumentsTracingSampling`.** Required-substring loops with a single `t.Errorf` give a noisy failure list; subtests give one failure line per missing piece. Mirrors the kind of clear failure messaging used throughout the test base (e.g., `TestSpanAttributeAllowlist` from plan 55-02).
- **TDD RED/GREEN collapsed into a single Task-2 commit** because the test and the doc were authored together and the RED phase was self-validating (test was deliberately written first against the unchanged USAGE.md, observed to fail on three substrings, then the USAGE.md edit was made and the test re-run green). Same pattern used in plan 55-01 task commits — documented here for transparency rather than treated as a deviation.

## Deviations from Plan

**None — plan executed as written.** Two clarifying notes:

- The plan suggested separate RED and GREEN commits in Task 2; I collapsed them into one because (a) the test and the doc edit are tightly coupled (both belong to "make the sampling docs verifiable"), (b) a strict RED-first commit would have left CI red on the worktree branch — and the Task-2 commit message references the test as part of the deliverable. The RED phase is documented above in the "TestUsageDocumentsTracingSampling Evidence" section.
- USAGE.md ended up at 109 added lines (vs. the plan's implicit assumption of "a Sampling subsection plus a recipe block"); the extra is the head-vs-tail honesty paragraph and the Trace Coverage cross-reference subsection. Both are required by the plan's `<action>` bullets and by `TestUsageDocumentsTracingSampling`.

## Issues Encountered

None. All tasks landed first-attempt; the regression test failed correctly on the RED pass and passed correctly after the doc edit.

## Verification Results

- `go vet ./...` — clean (only the pre-existing C-macro warning from the vendored Swift tree-sitter binding, unrelated; documented in 55-01 SUMMARY).
- `go test ./docs/... -run TestUsageDocumentsTracingSampling -v -timeout 60s` — 7/7 subtests pass.
- `go test ./docs/... -timeout 60s` — both `docs` and `docs/runbooks` packages green (TestUSAGEObservability not regressed by USAGE.md edits).
- `go test ./docs/... ./internal/obs/... ./internal/mcp/... -timeout 120s` — all green; no regressions in plan 55-01 or 55-02 tests.
- `wc -l TRACE-AUDIT.md` → 165 (>= 60 plan minimum).
- `grep -c "daemon.mcp.tools.call\|ls.request\|kernel.tool\|skill.tool" TRACE-AUDIT.md` → 19 (>= 4 plan minimum); all four span names appear; anti-list and propagation-policy sections both present.
- `grep -c tracing_sample_ratio USAGE.md` → 6 (>= 2 plan minimum); `grep -c otel/opentelemetry-collector-contrib USAGE.md` → 1 (>= 1 plan minimum).
- `55-HUMAN-UAT.md` exists with `status: pending`, both scenarios, Summary block, and Operator Setup Notes.

## Self-Check: PASSED

- File `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` — FOUND (165 lines)
- File `.planning/phases/55-obs-trace-coverage-audit/55-HUMAN-UAT.md` — FOUND (107 lines, status pending)
- File `.planning/phases/55-obs-trace-coverage-audit/55-03-SUMMARY.md` — FOUND (this file)
- File `USAGE.md` — MODIFIED (Sampling/Smoke-Testing/Trace Coverage subsections added under "Enable Tracing")
- File `docs/usage_test.go` — MODIFIED (TestUsageDocumentsTracingSampling appended to package `docs_test`)
- Commit `3a76bb2d` — FOUND (Task 1 — TRACE-AUDIT.md)
- Commit `7f61f86e` — FOUND (Task 2 — USAGE.md + regression test)
- Commit `351d6411` — FOUND (Task 3 — 55-HUMAN-UAT.md)

## Next Plan Readiness

Plan 55-04 / Phase verification can now:
- Reference TRACE-AUDIT.md as the human cert artifact for OBS-04 #2.
- Run `go test ./docs/... -run TestUsageDocumentsTracingSampling` as the doc-regression CI gate for OBS-04 #3.
- Hand the human operator 55-HUMAN-UAT.md to capture OBS-04 #4 evidence; phase cannot be marked verified until the operator updates `result: [pending]` to a captured snippet (or descriptive trace tree) and the Summary counters are refreshed accordingly.

Task 4 of this plan (the human-verify checkpoint) returns control to the operator. Phase verification will resume once the operator approves.

---
*Phase: 55-obs-trace-coverage-audit*
*Completed: 2026-04-28*
