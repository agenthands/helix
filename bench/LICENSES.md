# Bench Dataset License Audit

This file is the **per-dataset license audit** for every dataset the bench stack consumes
(INFRA-02). Each external dataset adapter added in Phases 85–88 (Aider Polyglot, SWE-bench,
Multi-SWE-bench, Terminal-Bench, etc.) **MUST append one row** to the table below recording
the dataset's source, license, redistribution clause, the pinned content hash/revision, and
the date the license was verified. A dataset with no row here is not cleared for use.

The table is seeded with the internal ToolBench-Go suite (authored in-repo, covered by the
repository license — no external redistribution concern).

| dataset | source_url | license | redistribution_clause | sha256_or_pin | verified_on |
|---------|-----------|---------|-----------------------|---------------|-------------|
| internal-toolbench | (in-repo: `bench/datasets/internal-toolbench/`) | internal — repo license | covered by Helix repository license; no external redistribution | (in-repo; pinned by git) | 2026-06-14 |
| multi-swe-bench | huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench | CC0 | redistributable (CC0; users still respect each source-project's license) | 0000000000000000000000000000000000000000 | 2026-06-21 |
| terminal-bench | huggingface.co/datasets/harborframework/terminal-bench-2.0 | Apache-2.0 | redistributable (Apache-2.0; attribution required) | 0000000000000000000000000000000000000000 | 2026-06-21 |

## Full-set license status (INFRA-02)

Both Phase 88 datasets carry the rows above for the **shipped scope** — the
Multi-SWE-bench **Mini set** (`bench/datasets/multi-swe-bench-mini/`, ~400
instances, 8 langs) and Terminal-Bench 2.0. The Multi-SWE-bench **full
1,632-instance set** remains the documented reach goal (deferred to v1.13).
The dataset license itself is **CC0** and therefore clear for redistribution
(88-RESEARCH O-2); the v1.13 full-set deferral applies **only** if a specific
source-project license is found to restrict a particular repo's instances —
not because the umbrella dataset license is in doubt. Terminal-Bench 2.0 is
**Apache-2.0** (attribution); no full-vs-mini license split applies.

**Pin / digest provenance (deferred).** The `sha256_or_pin` cells above carry a
40-zero placeholder rev: 88-RESEARCH did not record an exact upstream commit sha
(the offline build environment cannot reach HF to query the refs API). The exact
Mini-set repo id (A4), pinned commit, and per-file content digests are confirmed
against the **documented** upstream but their **live** confirmation is DEFERRED
to a Docker + multi_swe_bench + network host (the A1–A7 `checkpoint:human-verify`,
Phase 87 precedent). On re-pin, update both the `sha256_or_pin` column here and
`bench/datasets/multi-swe-bench-mini/pin.go` `PinnedSHA` in lockstep.

## Column meanings

- **dataset** — adapter / dataset identifier (matches its dir under `bench/datasets/`).
- **source_url** — canonical upstream URL, or `(in-repo: <path>)` for internal datasets.
- **license** — the dataset's license (SPDX identifier where applicable).
- **redistribution_clause** — whether and how the dataset may be redistributed / vendored,
  and any attribution requirement.
- **sha256_or_pin** — the pinned content hash or upstream git revision the adapter targets,
  so the audited license maps to an exact dataset snapshot.
- **verified_on** — `YYYY-MM-DD` date the license terms were last confirmed.
