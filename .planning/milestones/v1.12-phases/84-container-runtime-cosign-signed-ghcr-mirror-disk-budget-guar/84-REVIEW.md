---
phase: 84-container-runtime-cosign-signed-ghcr-mirror-disk-budget-guar
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - bench/container/container.go
  - bench/container/engine.go
  - bench/container/arch.go
  - bench/container/cache.go
  - bench/container/disk.go
  - bench/container/disk_unix.go
  - bench/container/disk_windows.go
  - bench/container/procgroup_unix.go
  - bench/container/procgroup_windows.go
  - bench/container/pull.go
  - bench/container/verify.go
  - bench/container/engine_test.go
  - bench/container/cache_test.go
  - bench/container/disk_test.go
  - bench/container/arch_test.go
  - bench/container/pull_test.go
  - bench/container/verify_test.go
  - bench/container/testdata/generate_fixtures.go
  - .github/workflows/bench-mirror.yml
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 84: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

This is a carefully engineered, security-conscious package and the core
security invariants hold up under adversarial inspection:

- **Verify-then-pull ordering is correct.** `VerifyThenPull` (pull.go:76-88)
  runs `verifyFn` strictly before `pullFn` inside the cache-fill closure;
  `Ensure` (cache.go:84-92) removes the staging dir on any closure error
  and only writes the sentinel + renames on success, so a verify failure
  publishes zero cache bytes. Proven hermetically (no skip-only criterion).
- **No verification oracle leak.** Every failure branch in `VerifyImage`
  returns the byte-identical canonical literal; `TestVerifyErrorTextIsIdentical
  AcrossBranches` and `TestCanonicalLiteralGrepCoverage` are real hermetic
  guards.
- **Identity pinning is sound.** Issuer is exact-matched, SAN is matched
  against a properly `^…$`-anchored, dot-escaped regex
  (verify.go:36/49). The SAN literal matches the cosign cert SAN minted by
  bench-mirror.yml (path + `@refs/heads/main`). Wrong-org and wrong-issuer
  fixtures exercise both pins.
- **argv hardening is consistent.** `isHexSHA256` + `isValidRepo` + the `--`
  end-of-options separator fail-close flag/shell smuggling before the
  os/exec and crane boundaries; env is a strict 3-key allowlist; no
  `github.com/docker/docker` in go.mod (only the explicitly-excluded
  `docker-credential-helpers`); `make vet` runs the static SDK-ban gate.
- **Builds and vets clean; all hermetic tests pass.**

The findings below are robustness/correctness defects (no BLOCKERs): a
cache-fill failure mode under concurrency/crash recovery, a fragile CI
digest-capture, an unverified-default error-shape leak in `pull.go`, and a
windows-cancellation gap. None opens a signature-bypass or injection path.

No structural findings block was provided.

## Warnings

### WR-01: `Ensure` permanently wedges the cache when `finalDir` already exists (concurrency / crash recovery)

**File:** `bench/container/cache.go:64-99`
**Issue:** `Ensure` treats any dir lacking the `.container-cache-ok`
sentinel as cold and proceeds to fetch into a fresh staging dir, then
`os.Rename(stageDir, finalDir)`. But `os.Rename` over an **existing**
directory fails with "file exists" on Linux/macOS — confirmed empirically,
it fails even when the target dir is empty, and certainly when non-empty.
Two realistic triggers:

1. **Concurrent double-fetch (the sentinel race called out in scope):**
   two `Ensure(sha, …)` calls for the same digest both miss the sentinel,
   both fetch into separate staging dirs. The first renames successfully and
   publishes `finalDir` *with* the sentinel. The second then does
   `os.Rename(stage2, finalDir)` → **fails with "file exists"** and returns a
   hard error to its caller, even though the cache is now valid. There is no
   `sync`/`flock`/`O_EXCL` coordination anywhere in the package.
2. **Crash/interrupt recovery:** if a `finalDir` ever exists without the
   sentinel (e.g. a previous interrupted/partial publish, or operator
   tampering), every subsequent run re-fetches, re-verifies, and then dies
   at the rename — the cache is permanently un-fillable with no self-heal.

This is the same TOCTOU surface the design otherwise guards well; it just
isn't closed on the publish side.

**Fix:** before renaming, handle an existing target: if `finalDir` already
carries the sentinel, drop the freshly-staged dir and treat it as a hit;
otherwise atomically replace it. For example:

```go
// Re-check the sentinel after fetch: a concurrent winner may have published.
if _, err := os.Stat(filepath.Join(finalDir, cacheOKMarker)); err == nil {
    os.RemoveAll(stageDir)
    return true, finalDir, nil
}
// Clear any stale/partial finalDir so the rename can land atomically.
if err := os.RemoveAll(finalDir); err != nil {
    os.RemoveAll(stageDir)
    return false, "", err
}
if renErr := os.Rename(stageDir, finalDir); renErr != nil {
    os.RemoveAll(stageDir)
    return false, "", renErr
}
```

(For true concurrency safety against a torn `RemoveAll`+`Rename` window,
gate the whole fill behind a per-sha `O_CREATE|O_EXCL` lock file under
`imagesRoot`.)

### WR-02: CI signs the wrong (possibly upstream) digest via `index .RepoDigests 0`

**File:** `.github/workflows/bench-mirror.yml:120-132`
**Issue:** After `docker pull UPSTREAM; docker tag UPSTREAM DEST; docker push
DEST`, the local image carries `RepoDigests` for **both** the upstream repo
(from the pull) and the dest repo (from the push). `docker inspect
--format='{{index .RepoDigests 0}}'` selects element 0 of an array whose
ordering is not contract-guaranteed, so `DIGEST` can be the **upstream**
repo's digest. The subsequent `cosign sign "${DEST_REPO}@${DIGEST}"` then
signs `ghcr.io/agenthands/helix-bench-<suite>@sha256:<upstream-digest>`. If
the upstream and dest manifest digests differ (different media-type
normalization, multi-arch index vs. manifest, registry re-compression), the
signed reference points at a manifest that may not exist in the dest repo —
publishing an image whose signature the runtime verifier cannot match
(fail-closed, but the mirror is broken). Even when digests coincide today,
this is silently order-dependent.

Additionally, `DIGEST="$(docker inspect … | awk …)"` under `set -o pipefail`
will abort the job if `docker inspect` errors, which is fine, but the
`-z "${DIGEST}"` guard is the only correctness check and it does not verify
the digest belongs to `DEST_REPO`.

**Fix:** select the digest for the dest repo explicitly rather than by
index, e.g.:

```bash
DIGEST="$(docker inspect --format='{{range .RepoDigests}}{{println .}}{{end}}' "${DEST}" \
  | grep "^${DEST_REPO}@" | head -n1 | awk -F'@' '{print $2}')"
[ -n "${DIGEST}" ] || { echo "::error::no ${DEST_REPO} digest on pushed image"; exit 1; }
```

Or capture the digest from `docker push`'s own output / `docker
buildx imagetools inspect "${DEST}"`, which reports the registry digest of
exactly what was pushed.

### WR-03: Un-wired production `VerifyThenPull` returns a non-canonical, branch-distinguishable error

**File:** `bench/container/pull.go:18-19, 143-145, 150-152`
**Issue:** The default (un-wired) `fetchMetaFn`/`pullFn` return the distinct
sentinels `errFetchMetaNotWired` ("…fetch not wired until Plan 04 mirror")
and `errPullNotWired`. These are inside the verify-then-pull closure but
*around* the `VerifyImage` call, so a production caller (no test seams)
hitting `VerifyThenPull` today gets a descriptive, branch-revealing error
rather than the deliberately oracle-free canonical literal that the rest of
the security story is built around. It is fail-closed (no bytes published),
so not a BLOCKER, but it is an inconsistency: the package took pains to make
*verification* failures indistinguishable, then ships a default path whose
error text announces exactly which collaborator is un-wired. A confused
production caller could also mistake "not wired" for "verification was
performed and passed/failed."

**Fix:** either (a) make the package's exported entry point refuse to run
with the not-wired defaults at all (return a clearly-labeled
`ErrNotConfigured` from a guard at the top of `VerifyThenPull` so it cannot
be confused with a verify result), or (b) document explicitly that
`VerifyThenPull` is not a public production API until Plan 04 and keep the
exported surface unexported until then. Given the Option seams are already
unexported test-only, tightening `VerifyThenPull`/`VerifyImage` visibility
(or adding a build-tag guard) would prevent an accidental
un-verified-by-omission call site.

### WR-04: Windows cancellation cannot group-kill engine descendants

**File:** `bench/container/procgroup_windows.go:12-14`, `engine.go:67-77`
**Issue:** On windows `procGroupAttr()` returns `nil`, so cancellation falls
back to `exec.CommandContext`'s default `os.Process.Kill`, which terminates
only the direct `docker`/`podman` process — not its child processes (the
real pull/transfer workers a CLI may spawn). On a context cancel or timeout
the engine's descendants can be orphaned and continue consuming disk/network
on the windows release archive. The unix side correctly uses `Setpgid` +
implies a group-kill. The comment asserts default kill "is sufficient" on
windows, but that is only true when the engine spawns no surviving children.

**Fix:** on windows, attach the child to a Win32 Job Object with
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` (via `golang.org/x/sys/windows`, which
is already the platform import in this build-tag split) so a cancel tears
down the whole tree. If that is out of scope for this phase, downgrade the
code comment from "is sufficient" to an explicit known-limitation note and
track it, rather than asserting parity with the unix path.

## Info

### IN-01: `cacheDir()` is read twice per `Ensure`/`ImagePath` call

**File:** `bench/container/cache.go:48-53, 64-77`
**Issue:** `Ensure` calls `ImagePath(sha)` (which calls `cacheDir()`) and
then independently calls `cacheDir()` again to build `imagesRoot`. Within a
single call there is no env mutation so the two reads agree, and the staging
dir is correctly a sibling of `finalDir` (same `imagesRoot`) keeping the
rename on one filesystem. It is nonetheless a latent footgun: if
`HELIX_CACHE_DIR` were ever changed concurrently the staging and final paths
could diverge across filesystems and break atomic rename.
**Fix:** resolve `root := cacheDir()` once at the top of `Ensure` and derive
both `finalDir` and `imagesRoot` from that single snapshot.

### IN-02: `disk_unix.go` free-space multiplication can overflow (benign)

**File:** `bench/container/disk_unix.go:17`
**Issue:** `st.Bavail * uint64(st.Bsize)` is an unchecked `uint64`
multiplication. Overflow only occurs at exabyte-scale free space, so it is
practically unreachable, but the result is silently wrong if it ever wraps,
and a wrapped-small value would *falsely* trip the 50 GiB refusal (safe
direction) — or a wrapped value could in principle pass. Worth a one-line
guard or comment.
**Fix:** note the assumption, or clamp: `if st.Bsize > 0 && st.Bavail >
math.MaxUint64/uint64(st.Bsize) { return math.MaxUint64, nil }` before the
multiply.

### IN-03: Live verify+pull test is the sole live coverage; hermetic siblings present (informational, not a defect)

**File:** `bench/container/pull_test.go:264-303`
**Issue:** `TestVerifyThenPullLive` SKIPs without an engine + `HELIX_BENCH_
MIRROR`. Per review guidance this is acceptable because the verify-before-
fetch decision branch, no-bytes-on-verify-failure, arch gating, cache hit,
and bad-digest/repo rejection all have hermetic siblings
(`TestVerifyThenPull*`). Flagged only to record that the *production*
`fetchMetaFn`/`pullFn` wiring itself (the real crane.Manifest + bundle
referrer lookup + crane.Pull→layer export) has **no** hermetic coverage —
it is `notWired*` stubs until Plan 04. That is by design here, but the
phase's "verify+pull works end to end" claim rests entirely on a skipped
test until Plan 04 lands; downstream should not read green CI as proof of
the live crane wiring.
**Fix:** none required this phase; ensure Plan 04 adds a hermetic test that
injects a real (registry-backed or recorded) `fetchMetaFn`/`pullFn` so the
production fetch/pull path is not first exercised only in the gated live
test.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
