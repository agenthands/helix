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

## Column meanings

- **dataset** — adapter / dataset identifier (matches its dir under `bench/datasets/`).
- **source_url** — canonical upstream URL, or `(in-repo: <path>)` for internal datasets.
- **license** — the dataset's license (SPDX identifier where applicable).
- **redistribution_clause** — whether and how the dataset may be redistributed / vendored,
  and any attribution requirement.
- **sha256_or_pin** — the pinned content hash or upstream git revision the adapter targets,
  so the audited license maps to an exact dataset snapshot.
- **verified_on** — `YYYY-MM-DD` date the license terms were last confirmed.
