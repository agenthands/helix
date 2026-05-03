---
phase: 52-packaging-distribution-channels
plan: 01
subsystem: testing
tags: [packaging, testing, scaffolding, minisign, embed, makefile, ci]

# Dependency graph
requires:
  - phase: 51-packaging-goreleaser
    provides: minisign signing pipeline + repo-root minisign.pub single source of truth (D-02) + release.yml PLACEHOLDER pre-flight gate (51-06)
provides:
  - internal/upgrade/testdata/ populated with six test fixtures (test keypair, signed stub archive, captured release JSON, README)
  - Makefile embed-pubkey + verify-embed-pubkey targets (D-13 build-time copy mechanism)
  - internal/upgrade/minisign.pub checked-in baseline that mirrors repo-root minisign.pub byte-for-byte
  - .github/workflows/release.yml gated on `make verify-embed-pubkey` (T-52-01-01 mitigation)
affects:
  - 52-02 binary rename (will rename BINARY=serena → helix and ./cmd/serena → ./cmd/helix; embed-pubkey target name + path stay)
  - 52-03 internal/upgrade package (consumes the testdata fixtures and the embedded internal/upgrade/minisign.pub)
  - 52-04 cobra subcommands update/upgrade (use the testdata httptest fixture pattern)
  - 52-05/52-06 docs + EMBED-AUDIT.md (reference the embed-pubkey mechanism as the canonical sync recipe)

# Tech tracking
tech-stack:
  added:
    - minisign 0.12 test fixture pattern (passwordless secret key checked in under testdata/)
    - Makefile self-doc targets embed-pubkey + verify-embed-pubkey (Phase 50 D-07 style)
  patterns:
    - "Build-time embed copy: cp repo-root → internal/<pkg>/<file> via Makefile pre-build, gated on cmp -s in CI"
    - "Test-only minisign keypair pattern: untrusted-comment header marked test-only; passwordless secret key checked in under <pkg>/testdata/; trusted-comment fixed to a recognizable string for assertion"
    - "GitHub Releases /latest fixture using https://example.invalid/... per RFC 6761 to fail loudly on missed httptest stubs"

key-files:
  created:
    - internal/upgrade/testdata/test_keypair.pub
    - internal/upgrade/testdata/test_keypair.key
    - internal/upgrade/testdata/sample-archive.tar.gz
    - internal/upgrade/testdata/sample-archive.tar.gz.minisig
    - internal/upgrade/testdata/release_latest.json
    - internal/upgrade/testdata/README.md
    - internal/upgrade/minisign.pub
    - .planning/phases/52-packaging-distribution-channels/deferred-items.md
  modified:
    - Makefile (added embed-pubkey + verify-embed-pubkey targets; build: embed-pubkey)
    - .github/workflows/release.yml (added verify-embed-pubkey step in pre-flight)

key-decisions:
  - "Embedded copy lives at internal/upgrade/minisign.pub and is checked in (NOT gitignored) so a fresh checkout has a baseline for verify-embed-pubkey to diff against"
  - "Test fixtures use https://example.invalid/... per RFC 6761 to ensure tests that miss the httptest stub fail loudly with DNS errors instead of silently leaking the runner IP"
  - "The plan-text size hint of '28 bytes' for the stub binary content is treated as informational; the literal content 'helix-stub-binary-v0.0.0-test\\n' (30 bytes) takes precedence — the verify check only requires the archive contains exactly entry `helix`"

patterns-established:
  - "Build-time embed copy pattern (D-13): cp repo-root file → internal/<pkg>/<file> via Makefile target, plus a verify-* twin that runs cmp -s for CI. Future packages embedding repo-root assets follow this pattern instead of `//go:embed ../../<file>` (forbidden by golang/go#46056)"
  - "Test fixture provenance README: every <pkg>/testdata/ tree gets a README.md documenting (1) the test-only nature of the keys/secrets, (2) the exact regeneration recipe, (3) why example.invalid hosts are used"

requirements-completed: []

# Metrics
duration: 5min
completed: 2026-04-29
---

# Phase 52 Plan 01: Wave 0 test scaffolding Summary

**Test minisign keypair + signed stub archive + captured release JSON + Makefile embed-pubkey/verify-embed-pubkey targets + release.yml CI gate that blocks any drift between repo-root minisign.pub and internal/upgrade/minisign.pub.**

## Performance

- **Duration:** 5 min 9 s (309 s)
- **Started:** 2026-04-29T18:17:35Z
- **Completed:** 2026-04-29T18:22:44Z
- **Tasks:** 2 / 2
- **Files created:** 8 (6 fixtures + embed copy + deferred-items.md)
- **Files modified:** 2 (Makefile, release.yml)

## Accomplishments

- Wave 0 fixture set in place: every plan in 52-02..52-06 can now stub the GitHub Releases API with a captured `release_latest.json`, exercise minisign verification with a self-consistent test keypair, and assert against a known trusted-comment (`helix-test-fixture`) without ever shelling out to a real key or hitting the network.
- D-13 build-time-copy mechanism shipped end-to-end: `make build` re-runs `embed-pubkey` first, contributors cannot ship a stale embedded pub by accident, and CI fails the release pipeline on any drift before goreleaser is invoked.
- Negative-test confirmed manually during Task 2: appending whitespace to `internal/upgrade/minisign.pub` flips `make verify-embed-pubkey` to non-zero with the documented message; running `embed-pubkey` reverts cleanly.

## Task Commits

Each task was committed atomically:

1. **Task 1: Generate test minisign keypair + fixture archive + captured release JSON** — `9fe326ea` (test)
2. **Task 2: Add embed-pubkey + verify-embed-pubkey Makefile targets and gate CI on the verify target** — `a551b1e0` (build)

**Plan metadata:** added in the next commit alongside SUMMARY.md / STATE.md / ROADMAP.md / deferred-items.md updates.

## Files Created/Modified

### Created
- `internal/upgrade/testdata/test_keypair.pub` — test-only minisign public key (passwordless; `untrusted comment:` marked `test-only`)
- `internal/upgrade/testdata/test_keypair.key` — test-only minisign secret key (passwordless; `untrusted comment:` marked `test-only`)
- `internal/upgrade/testdata/sample-archive.tar.gz` — gzipped tar containing exactly one entry `helix` with placeholder content `helix-stub-binary-v0.0.0-test\n`
- `internal/upgrade/testdata/sample-archive.tar.gz.minisig` — minisign signature over the stub archive, trusted comment `helix-test-fixture`
- `internal/upgrade/testdata/release_latest.json` — captured GitHub Releases /latest shape with `tag_name: v1.9.0` and four `https://example.invalid/...` asset URLs
- `internal/upgrade/testdata/README.md` — provenance + 4-command regeneration recipe + hard warning that the test secret key cannot reach production
- `internal/upgrade/minisign.pub` — byte-for-byte copy of repo-root `minisign.pub` produced by `make embed-pubkey`; checked in so `verify-embed-pubkey` has a baseline on a fresh clone
- `.planning/phases/52-packaging-distribution-channels/deferred-items.md` — out-of-scope note documenting the gitignored `tmp/graphify/` cgo warning

### Modified
- `Makefile` — added `embed-pubkey` and `verify-embed-pubkey` self-doc targets after `release-snapshot`; added both to `.PHONY:`; made `build:` depend on `embed-pubkey`. The 3-line addition shape:

  ```makefile
  embed-pubkey: ## Sync repo-root minisign.pub into internal/upgrade/minisign.pub before build
  	@cp minisign.pub internal/upgrade/minisign.pub

  verify-embed-pubkey: ## CI gate: assert internal/upgrade/minisign.pub matches repo-root copy byte-for-byte
  	@cmp -s minisign.pub internal/upgrade/minisign.pub || { \
  	  echo "internal/upgrade/minisign.pub drift; run: make embed-pubkey"; exit 1; }
  ```

  And the build dependency:

  ```makefile
  build: embed-pubkey
  	$(GO) build -o $(BINARY) ./cmd/serena
  ```

  (Plan 52-02 owns the `cmd/serena` → `cmd/helix` flip; this plan deliberately leaves it.)

- `.github/workflows/release.yml` — new step `Verify embedded minisign.pub matches repo-root` inserted directly after the existing PLACEHOLDER pre-flight (line ~37) and before `Set up Go`. The step runs `make verify-embed-pubkey` with a comment block explaining the T-52-01-01 mitigation.

## Regeneration Recipe (the 4 commands referenced by the plan output spec)

If the test keypair must ever be rotated, reproduce the testdata fixtures from
the repo root with:

```sh
# 1. Generate a passwordless test keypair (test-only — never used in production).
minisign -G -p internal/upgrade/testdata/test_keypair.pub \
            -s internal/upgrade/testdata/test_keypair.key -W -f

# 2. Build the stub archive (single file `helix` with placeholder content).
mkdir -p /tmp/helix-stub-build
printf 'helix-stub-binary-v0.0.0-test\n' > /tmp/helix-stub-build/helix
( cd /tmp/helix-stub-build && \
  tar -czf "$OLDPWD/internal/upgrade/testdata/sample-archive.tar.gz" helix )

# 3. Sign the stub archive with the test secret key.
minisign -S -s internal/upgrade/testdata/test_keypair.key \
            -m internal/upgrade/testdata/sample-archive.tar.gz \
            -x internal/upgrade/testdata/sample-archive.tar.gz.minisig \
            -W -t "helix-test-fixture"

# 4. Re-edit the `untrusted comment:` headers of test_keypair.pub and
#    test_keypair.key to include the literal "test-only" string so a future
#    code reviewer cannot mistake them for the real key.
```

After regeneration, run:
- `minisign -V -p internal/upgrade/testdata/test_keypair.pub -m internal/upgrade/testdata/sample-archive.tar.gz -x internal/upgrade/testdata/sample-archive.tar.gz.minisig` (must print "Signature and comment signature verified" with trusted comment `helix-test-fixture`).
- `cmp -s minisign.pub internal/upgrade/minisign.pub` (must exit 0 — confirmed during this plan's verification).

## Decisions Made

- **Embedded pub is checked in, not gitignored.** A gitignored `internal/upgrade/minisign.pub` would mean fresh clones have nothing for `verify-embed-pubkey` to diff against, defeating the gate. Checking it in adds 218 bytes to the repo and makes the gate meaningful from the first PR.
- **Plan text said "28 bytes" for the stub-archive helix content** but the literal byte string `helix-stub-binary-v0.0.0-test\n` is 30 bytes. The literal content takes precedence because the verify check only requires the archive to contain exactly entry `helix` — the byte count was an informational aside, not an assertion.
- **Postponed `BINARY=serena` → `helix` and `./cmd/serena` → `./cmd/helix` flips.** Plan 52-02 owns those edits; this plan deliberately keeps the existing `build:` recipe pointing at `./cmd/serena` to avoid double-touching the same lines.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] `go vet ./...` and `go test ./...` fail to enumerate packages because of gitignored `tmp/graphify/`**
- **Found during:** Task 2 verification (the project rule "always run go vet and go test before completing any Go task")
- **Issue:** A pre-existing gitignored `tmp/graphify/tests/fixtures/` tree (local scratch from the user's `graphify` skill) contains C source without cgo, which makes `go list ./...` exit non-zero. This is environmental and unrelated to Plan 52-01's changes — it surfaces on a stash of HEAD~1 too.
- **Fix:** Ran `go vet` and `go test` against the real project package list `./internal/... ./cmd/... ./api/... ./protocol/...` via `go list ... | xargs`. Both pass cleanly (`go vet` exit 0; all 33 test packages pass).
- **Files modified:** none (workaround is invocation-only); the discovery is documented in `.planning/phases/52-packaging-distribution-channels/deferred-items.md` per scope-boundary rule.
- **Verification:** `go list ./internal/... ./cmd/... ./api/... ./protocol/... | xargs go vet` exits 0; same with `xargs go test -count=1` — every package shows `ok` or `[no test files]`.
- **Committed in:** the deferred-items.md note ships with this plan's final metadata commit.

---

**Total deviations:** 1 (Rule 3 blocking workaround for a pre-existing gitignored local tree)
**Impact on plan:** None on plan deliverables. Documented for visibility so future plans don't chase the same noise.

## Issues Encountered

- During Task 1 the minisign secret-key file's `untrusted comment:` header was rewritten by minisign back to its default after the `-S` (sign) operation. Resolved by re-editing the header after signing — confirmed via `Read` that the comment persists in the final committed `test_keypair.key`. Future regenerations should re-edit the comment as the last step (recipe step 4 in the README captures this).

## Self-Check: PASSED

Verified after writing SUMMARY.md:

```
internal/upgrade/testdata/test_keypair.pub          FOUND
internal/upgrade/testdata/test_keypair.key          FOUND
internal/upgrade/testdata/sample-archive.tar.gz     FOUND
internal/upgrade/testdata/sample-archive.tar.gz.minisig FOUND
internal/upgrade/testdata/release_latest.json       FOUND
internal/upgrade/testdata/README.md                 FOUND
internal/upgrade/minisign.pub                       FOUND (218 bytes, byte-identical to minisign.pub)
Makefile                                            FOUND (embed-pubkey, verify-embed-pubkey, build: embed-pubkey)
.github/workflows/release.yml                       FOUND (make verify-embed-pubkey step under non-comment lines)
.planning/phases/52-packaging-distribution-channels/deferred-items.md FOUND

git log:
  9fe326ea test(52-01): add upgrade-package test fixtures and minisign keypair  FOUND
  a551b1e0 build(52-01): sync embedded minisign.pub via Makefile + CI gate     FOUND
```

## Next Phase Readiness

- **Plan 52-02 (binary rename `serena` → `helix`)** can proceed; it edits `BINARY=serena` and `./cmd/serena` in the Makefile and moves `cmd/serena/main.go`. The `embed-pubkey` target is already in place and is path-independent on the source side.
- **Plan 52-03 (internal/upgrade package)** unblocked; the embedded `internal/upgrade/minisign.pub` file is in place for `//go:embed`, and the testdata directory has every fixture the upgrade tests need.
- **Plan 52-04..06** unblocked from a Wave 0 perspective — every per-task `<automated>` verify in subsequent plans now has fixtures to read.
- No new blockers. The `tmp/graphify/` cgo warning is documented as out-of-scope; future plans should run `go vet` / `go test` against the real package subtrees as shown.

---
*Phase: 52-packaging-distribution-channels*
*Completed: 2026-04-29*
