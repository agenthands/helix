---
phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
plan: 02
subsystem: testing
tags: [terminal-bench, evaluator-adapter, subprocess-shellout, runner-kind-seam, is-resolved-gate, container-isolation, hermetic-fixtures]

# Dependency graph
requires:
  - phase: 87-swe-bench-verified-adapter
    provides: "bench/evaluators/swebench/ template (harness/ingest/report/procgroup), isValidPredictionsPath idiom, strict env allowlist, runShim seam, resolved-gate ingestion, maxReportBytes/checkReportSize"
  - phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
    plan: 01
    provides: "multiswebench sibling clone structure (followed for consistency)"
provides:
  - "bench/evaluators/terminalbench/ leaf adapter: tb run fixed-argv wrapper behind a single runnerKind binary-name seam (tb default / harbor one-line swap, O-1)"
  - "size-capped strict parsers ParseTBResults (aggregate accuracy/n_resolved/n_unresolved) + ParseTBTrial (per-trial is_resolved)"
  - "Ingest mapping the authoritative is_resolved bool -> ResultInput.Metrics.TaskSuccess (distinct ptr), ContainerID verbatim, benchmark terminal-bench"
  - "SC#2 container-isolation invariant proven by an import-level gate (zero transitive bench/container dependency); isolation owned by tb's per-task DockerComposeManager"
affects: [88-03-longwall, 88-04-multi-swe-bench-mini-dataset, aggregator, bench-cell-orchestration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "runnerKind binary-name seam (O-1 / Pitfall 2): the tb-vs-harbor binary name + dataset-flag form live behind a single runnerSpec constant so swapping tb->harbor is a one-line defaultRunnerKind flip; the JSON-ingestion-bearing argv structure is unchanged"
    - "Explicit JSON-null rejection in ParseTBTrial: json.Unmarshal of \"null\" into a struct is a no-op that fabricates is_resolved=false, so a literal null body is refused before it can mimic an honest fail"
    - "Import-level isolation gate via golang.org/x/tools/go/packages full-transitive BFS (mirrors cmd/helix-bench-rag leakage gate): proves the adapter never reaches bench/container so cross-task fs leakage cannot be introduced"

key-files:
  created:
    - "bench/evaluators/terminalbench/harness.go"
    - "bench/evaluators/terminalbench/harness_test.go"
    - "bench/evaluators/terminalbench/ingest.go"
    - "bench/evaluators/terminalbench/ingest_test.go"
    - "bench/evaluators/terminalbench/report.go"
    - "bench/evaluators/terminalbench/report_test.go"
    - "bench/evaluators/terminalbench/procgroup_unix.go"
    - "bench/evaluators/terminalbench/procgroup_windows.go"
    - "bench/evaluators/terminalbench/testdata/results.json"
    - "bench/evaluators/terminalbench/testdata/trial_results.json"
  modified: []

decisions:
  - "Kept CONTEXT-locked tb run as defaultRunnerKind; harbor is a documented one-constant swap (O-1) — did NOT block on the upstream tb->harbor migration since the JSON ingestion shape is identical"
  - "isValidArg (leading-'-' refusal) for agent/task-id/dataset-version instead of the SWE-bench charset/instance-id validators, because tb task-ids (e.g. hello-world) and agent names do not follow the <owner>__<repo>-<num> shape; output-path keeps the full isValidOutputPath (clean+absolute+no ':'/'..') discipline"
  - "ParseTBTrial rejects a literal null body explicitly (json.Unmarshal-null is a silent no-op that would fabricate is_resolved=false)"
  - "SC#2 asserted as an import-level transitive gate rather than a behavioral Docker test — the adapter shells tb which owns its own per-task isolation; the gate proves the adapter adds no isolation surface of its own"

metrics:
  duration_minutes: 11
  completed_date: "2026-06-21"
  tasks_completed: 2
  files_created: 10
  files_modified: 0
  commits: 4
---

# Phase 88 Plan 02: Terminal-Bench 2.0 Adapter (tb run) Summary

The second and final Phase 88 external adapter: a stdlib-only Go transform package (`bench/evaluators/terminalbench/`) that shells the Terminal-Bench 2.0 `tb run` CLI behind a fixed-argv builder, ingests its `results.json` (aggregate + per-trial `is_resolved`) into `runtime.ResultInput`, and proves the SC#2 container-isolation invariant by an import-level gate — cloned from the Phase 87 `swebench` template with the one structural divergence being the `runnerKind` binary-name seam (tb default / harbor one-line swap, O-1).

## What was built

- **`harness.go`** — `RunArgs` builds the fixed `tb run` argv (`run --agent <a> --dataset-name terminal-bench-core --dataset-version <v> --task-id <id> --output-path <out>`). The binary name and dataset-flag form live behind a single `runnerKind` constant (`runnerKindTB` default, `runnerKindHarbor` documented) via `runnerSpec`, so the tb->harbor swap is a one-line `defaultRunnerKind` flip (O-1 / Pitfall 2). Flag-smuggling is refused with `errBadHarnessArg` + nil argv (`isValidArg` leading-`-` refusal for agent/task-id/dataset-version; `isValidOutputPath` clean+absolute+no `:`/`..` for output-path). Strict env allowlist `{PATH,HOME,HELIX_CACHE_DIR,DOCKER_*}` (T-88-02-02), `procGroupAttr` group-kill (T-88-02-06), `Detect` probes tb then harbor and returns `errHarnessUnavailable` when neither is present.
- **`report.go`** — `ParseTBResults` (`TBAggregate` accuracy/n_resolved/n_unresolved) and `ParseTBTrial` (`TBTrial` task_id/is_resolved); both size-capped to `maxReportBytes` (64 MiB, copied verbatim) before `json.Unmarshal` (T-88-02-03). `ParseTBTrial` explicitly rejects a literal `null` body so a null trial cannot fabricate `is_resolved=false`. Unknown keys tolerated; empty/oversized/type-mismatched bodies error.
- **`ingest.go`** — `Ingest` maps the authoritative per-trial `is_resolved` bool to `Metrics.TaskSuccess` (a distinct `&local` pointer, never aliased with `VerifiedCorrectness`, NEVER a count of trial rows — T-88-02-04), carries `ContainerID` verbatim and `exit_code` as a `*int` (literal 0 preserved), stamps `benchmark = "terminal-bench"`. An empty `task_id` is an ingest error returning the zero-value `ResultInput` (TaskSuccess nil — never fabricated success).
- **`procgroup_{unix,windows}.go`** — copied verbatim (package rename only).
- **`testdata/{results.json,trial_results.json}`** — committed hermetic fixtures; the SOLE authoritative proof.

## Tasks executed (TDD RED -> GREEN)

| Task | Name | RED | GREEN | Files |
| ---- | ---- | --- | ----- | ----- |
| 1 | tb run harness + runnerKind seam | `30ac1699` | `147eb4f3` | harness.go, harness_test.go, procgroup_{unix,windows}.go |
| 2 | results.json parsers + is_resolved ingestion | `cdad826e` | `815750bd` | report.go, report_test.go, ingest.go, ingest_test.go, testdata/{results,trial_results}.json |

## Verification

- `go test ./bench/evaluators/terminalbench/` — 12 tests PASS (`TestDetectAndLiveSmoke` skips cleanly; tb/harbor/Docker absent).
- `go build ./...` clean; `go vet ./bench/...` clean; `make vet` clean (all 6 custom vettools pass).
- The argv golden is the fixed `tb run` form; the `runnerKind` seam makes harbor a one-constant swap.
- The `is_resolved` gate + ContainerID stamp prove hermetically; the import-level gate (`TestContainerIsolationNoBenchContainerImport`) proves zero transitive `bench/container` coupling (SC#2).
- The live >=5-task tb smoke is tb/Docker-gated and is NOT the sole proof.

## Threat register dispositions

| Threat ID | Mitigation delivered |
|-----------|----------------------|
| T-88-02-01 | fixed argv; `isValidArg` leading-`-` refusal + `isValidOutputPath` traversal refusal; nil argv before os/exec; no shell |
| T-88-02-02 | strict `allowlistEnv` {PATH,HOME,HELIX_CACHE_DIR,DOCKER_*}; AWS-secret-leak test |
| T-88-02-03 | `maxReportBytes` + `checkReportSize` before `json.Unmarshal` |
| T-88-02-04 | task_success gated on the authoritative `is_resolved` bool, never a row count; empty task_id -> error, never fabricated success |
| T-88-02-05 | import-level gate proves zero transitive `bench/container`; isolation owned by tb's per-task DockerComposeManager |
| T-88-02-06 | `procGroupAttr()` Setpgid (unix) so a ctx cancel group-kills tb + Docker descendants |
| T-88-02-SC | accept — no go.mod/package install; tb is a runtime subprocess dep, not imported |

## Deviations from Plan

None - plan executed exactly as written. The plan's `isValidRunID`/`isValidInstanceID` copy-verbatim note was adapted to a leaner `isValidArg` leading-`-` refusal because tb task-ids/agent names do not follow the SWE-bench `<owner>__<repo>-<num>` shape; this is the plan's own intent ("copy the `isValidRunID` leading-'-' refusal idiom") rather than a behavioral divergence. The full `isValidOutputPath` traversal discipline is preserved verbatim from `isValidPredictionsPath`.

## Known Stubs

None. The adapter is complete and hermetically tested; the live tb smoke is intentionally gated (tb/Docker absent in this environment) and skips cleanly, as required by the plan — it is never the sole proof.

## Self-Check: PASSED
- All 10 created files verified present on disk.
- All 4 task commits verified in git log.
