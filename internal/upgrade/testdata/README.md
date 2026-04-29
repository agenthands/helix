# Test-only fixtures

These files are checked-in test fixtures consumed by `internal/upgrade/*_test.go`.
They exist solely to drive httptest stubs and exercise the minisign verification
path without touching the network or the production keypair.

> **Hard warning.** `test_keypair.key` is checked in *deliberately* because it
> cannot sign production releases — the repo-root `minisign.pub` and the
> production private key (held only as a GitHub Secret per Phase 51) never
> appear in this directory. The keys here exist for one job: making
> `internal/upgrade/...` unit tests deterministic.

## Provenance

Generated locally on 2026-04-29 per
`.planning/phases/52-packaging-distribution-channels/52-01-PLAN.md` Task 1.

| File | Purpose |
|------|---------|
| `test_keypair.pub` | Test-only minisign public key (passwordless) |
| `test_keypair.key` | Test-only minisign secret key (passwordless) |
| `sample-archive.tar.gz` | Stub release archive containing a single placeholder `helix` file |
| `sample-archive.tar.gz.minisig` | Valid signature over `sample-archive.tar.gz` produced with `test_keypair.key` (trusted comment: `helix-test-fixture`) |
| `release_latest.json` | Captured shape of `GET /repos/agenthands/helix/releases/latest` for httptest stubs (uses `https://example.invalid/...` per RFC 6761) |

## Regeneration recipe

If the keypair must be rotated (very rarely — only if it leaks or the format
changes upstream), reproduce these fixtures with the four commands below from
the repo root:

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

# 4. After regeneration, manually re-edit the `untrusted comment:` headers
#    of test_keypair.pub and test_keypair.key to include the literal string
#    "test-only" so a future code reviewer cannot mistake them for the real
#    key. The trusted comment in the .minisig is set via the -t flag above.
```

After regeneration:

- Run `minisign -V -p internal/upgrade/testdata/test_keypair.pub \
                -m internal/upgrade/testdata/sample-archive.tar.gz \
                -x internal/upgrade/testdata/sample-archive.tar.gz.minisig`
  to confirm the new keypair is internally consistent.
- Run `go test ./internal/upgrade/...` to confirm all tests still pass.

## Why is `test_keypair.key` checked in?

A passwordless test secret key is a deliberate choice: the upgrade-package
tests must produce signatures deterministically, and shelling out to a
key-generation step at test time would slow CI without adding any security.
The key cannot sign production releases because no infrastructure outside
`internal/upgrade/...` tests references it, and the public key it produces
(`test_keypair.pub`) is *not* embedded into the binary — only the repo-root
`minisign.pub` reaches `internal/upgrade/minisign.pub` via `make embed-pubkey`.

## Why `example.invalid`?

Per RFC 6761, `*.invalid` is reserved and guaranteed never to resolve. Using
it in `release_latest.json` ensures any test that *forgets* to override the
GitHub API base URL with an httptest server will fail loudly with a DNS error
instead of silently leaking the test runner's IP to a real host.
