# `test/bench/baselines/` — local-first benchmark baselines

This directory holds reference benchmark output for Serena. After Phase 50
(milestone v1.9), baselines are captured **locally pre-release** on stable
maintainer hardware. The CI bench gate has been retired — see
`.planning/phases/50-toolchain-go1.25-gopls-ci/50-CONTEXT.md` "Pivot
2026-04-25" for the rationale.

## How to capture a new baseline

Pre-release, on stable maintainer hardware, run the same flags the retired
CI gate used (parity preserved for direct comparison against historical
files):

```sh
go test -short -bench=. -benchmem -count=10 -run=^$ \
  ./test/bench/... | tee test/bench/baselines/v<version>-local-<goos>-<goarch>.txt
```

Filename pattern: `v<version>-local-<goos>-<goarch>.txt` (e.g.
`v1.9-local-darwin-arm64.txt`, `v1.10-local-linux-amd64.txt`).

For a human-readable diff against an earlier baseline:

```sh
benchstat test/bench/baselines/<old>.txt test/bench/baselines/<new>.txt
```

For a regression decision against the v1.2 PR-tier thresholds (15% time /
25% allocs at p<0.05) using the in-tree gate tool:

```sh
go run ./test/bench/cmd/benchgate \
  --baseline test/bench/baselines/<old>.txt \
  --new      test/bench/baselines/<new>.txt
```

Exit code zero means no regression at the configured tier.

## Why CI is no longer enforcing this

GitHub-hosted `ubuntu-latest` runners are shared-CPU hosts whose noise floor
and cold-start variance proved incompatible with PR-blocking benchmark
enforcement (two CI baseline-capture runs failed on this exact symptom
in April 2026). See
`.planning/phases/50-toolchain-go1.25-gopls-ci/50-RESEARCH.md` Pitfall 5
and the "Pivot 2026-04-25" block in 50-CONTEXT.md. The strategy was the
problem, not the timeouts. The CONTRIBUTING.md "Benchmarks" section
documents the contributor-facing workflow.

## Historical baselines (captured under the retired CI gate)

The following files were captured on GitHub-hosted `ubuntu-latest` while
the CI bench gate was active. They are kept on disk as historical
reference and MUST NOT be used as the comparison baseline for new local
captures (different hardware class — see Pitfall 5).

| File | Captured under | Status |
|------|----------------|--------|
| `v1.1-github-hosted.txt` | retired CI gate (Phase 9–15) | historical reference |
| `v1.2-phase10-github-hosted.txt` | retired CI gate (Phase 10) | historical reference |
| `v1.2-phase11-github-hosted.txt` | retired CI gate (Phase 11) | historical reference |
| `v1.2-phase12-github-hosted.txt` | retired CI gate (Phase 12) | historical reference |

These files are intentionally not deleted — they document the v1.1/v1.2
performance plateau and remain useful as cross-architecture reference
points. They are NOT consumed by any active workflow.

## What ships in Phase 50 (v1.9)

Phase 50 documents the new local-first workflow but does NOT capture a
v1.9 baseline. The first local baseline lands when the next release-driven
capture happens on the maintainer's hardware. See CONTRIBUTING.md
"Benchmarks" for when and how to run.
