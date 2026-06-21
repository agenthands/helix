---
phase: 84-container-runtime-cosign-signed-ghcr-mirror-disk-budget-guar
fixed_at: 2026-06-21T00:00:00Z
review_path: .planning/phases/84-container-runtime-cosign-signed-ghcr-mirror-disk-budget-guar/84-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 84: Code Review Fix Report

**Fixed at:** 2026-06-21
**Source review:** 84-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (Warnings; Info IN-01..03 out of scope)
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: `Ensure` permanently wedges the cache when `finalDir` already exists

**Files modified:** `bench/container/cache.go`, `bench/container/cache_test.go`
**Commit:** 1567a7b6
**Applied fix:** Made `Ensure` idempotent under the publish-side TOCTOU. After a
successful, fully-verified fetch+mark, it re-checks the `.container-cache-ok`
sentinel: if a concurrent winner already published a valid `finalDir`, it
discards the freshly-staged copy and returns a hit (the two copies are
byte-equivalent — both passed the same verify-then-pull). Otherwise it
`os.RemoveAll(finalDir)` (only ever a stale/partial dir lacking the sentinel, so
no valid cache is ever discarded) before the atomic rename, with a final
defensive sentinel re-check on rename failure. This closes the `os.Rename`
"file exists" wedge from the concurrent double-fetch race and from crash/partial-
publish recovery, while preserving the no-bytes-without-verify invariant. Added
two hermetic tests: a second-fill-over-pre-existing-finalDir case and a
16-goroutine concurrent-fill case (passes under `-race`).

### WR-02: CI signs the wrong (possibly upstream) digest via `index .RepoDigests 0`

**Files modified:** `.github/workflows/bench-mirror.yml`
**Commit:** ca08a68c
**Applied fix:** Replaced the order-dependent `index .RepoDigests 0` capture with
an explicit dest-repo selection: range over `.RepoDigests`, `grep "^${DEST_REPO}@"`,
take the first match, and extract the digest. The guard now fails loudly when no
dest-repo digest is present on the pushed image, so cosign can never sign an
upstream-repo digest against a manifest that may not exist in the dest repo.
Uses only shell env vars (`${DEST}`, `${DEST_REPO}`) sourced from the matrix
`env:` block — no untrusted `${{ ... }}` event input is interpolated into the
`run:` script. YAML validated with `yaml.safe_load`.

### WR-03: Un-wired production `VerifyThenPull` returns a non-canonical, branch-distinguishable error

**Files modified:** `bench/container/pull.go`, `bench/container/pull_test.go`
**Commit:** 8110c574
**Applied fix:** Added an exported `ErrNotConfigured` and a top-of-function guard
in `VerifyThenPull` (after the digest/repo fail-closed checks, before any arch
inspect/verify/pull) that refuses to run while the fetch/pull collaborators are
still the un-wired Plan 04 defaults. The un-wired state is detected by function
identity (`reflect.Value.Pointer` on `notWiredFetchMeta`/`notWiredPull`), not by
error text, so the guard neither depends on nor leaks the branch-distinguishing
sentinels. A production caller now gets a single, clearly-labeled "not configured"
error that cannot be confused with a verification outcome and announces no
collaborator name. The per-collaborator `errFetchMetaNotWired`/`errPullNotWired`
sentinels remain as a defense-in-depth backstop. The gated live test now SKIPs on
`ErrNotConfigured` (correct semantics until Plan 04). Added a hermetic guard test
asserting the un-wired defaults yield `ErrNotConfigured` and never leak the
per-collaborator sentinels. The bad-digest/bad-repo tests still pass because the
guard runs after those checks.

### WR-04: Windows cancellation cannot group-kill engine descendants

**Files modified:** `bench/container/procgroup_windows.go`
**Commit:** 6e70ebf7
**Applied fix:** Comment-only. Downgraded the overstated "default Process.Kill is
sufficient" claim to an explicit `KNOWN LIMITATION (WR-04)` note documenting that
the windows default kills only the direct docker/podman process (not its
descendants), unlike the unix `Setpgid` group-kill, and recording the Win32 Job
Object (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`) remedy as deferred/tracked. No Job
Object implementation was added (out of scope for this phase per the review's own
guidance). Cross-compiles clean for `GOOS=windows GOARCH=amd64`.

## Skipped Issues

None — all 4 in-scope Warning findings were fixed.

## Out-of-scope (Info, not addressed)

- **IN-01** (`cacheDir()` read twice in `Ensure`): latent footgun only under
  concurrent `HELIX_CACHE_DIR` mutation; not a defect in current usage.
- **IN-02** (`disk_unix.go` free-space multiply overflow): benign, exabyte-scale
  unreachable; wraps in the safe (refusal) direction.
- **IN-03** (live verify+pull is sole live coverage): by design until Plan 04;
  flagged for Plan 04 to add hermetic coverage of the real crane fetch/pull.

## Verification (all green)

- `go build ./...` — PASS
- `go vet ./bench/container/...` — PASS
- `go test -count=1 ./bench/container/...` — PASS
- `go test -count=1 -race ./bench/container/...` — PASS
- `make vet` (verify-no-docker-sdk + all vettools + `go vet ./...`) — PASS
- `GOOS=windows GOARCH=amd64 go build ./bench/container/...` — PASS
- `bench-mirror.yml` `yaml.safe_load` — PASS
- `github.com/docker/docker` absent from `go.mod` — confirmed

---

_Fixed: 2026-06-21_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
