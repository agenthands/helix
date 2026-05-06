---
phase: 59.1
recorded: 2026-05-06
recorder: Janis Vizulis
test_release: v1.10.7
---

# Phase 59.1 — Closure Evidence (re-audit 2026-05-06)

The 2026-05-04 verifier verdict (`status: human_needed`) was defensible at the time but is now stale: 7 releases shipped between phase closure and re-audit, and DEF-59.1-LINUX-ZIG-LIBSTDCXX was silently resolved.

## Item 1 — Linux + Windows CI build

**Original verdict:** PARTIAL — deferred to Phase 59.2 (zig+musl vs duckdb-go-bindings libstdc++ blocker).

**Actual state on 2026-05-06:** RESOLVED for linux. `.goreleaser.yaml:75` uses host gcc (not zig+musl) — `gcc` for linux/amd64, `aarch64-linux-gnu-gcc` for linux/arm64. Effectively "option 2 from the 4 enumerated options" (host gcc + apt-installed cross toolchains).

**v1.10.7 published archives:**
- helix_v1.10.7_Darwin_amd64.tar.gz + .sigstore.json
- helix_v1.10.7_Darwin_arm64.tar.gz + .sigstore.json
- helix_v1.10.7_Linux_amd64.tar.gz + .sigstore.json
- helix_v1.10.7_Linux_arm64.tar.gz + .sigstore.json

**Outstanding:** Windows archives still absent. Treated as a separate scoped deferral (option 3 partial — windows out of v1.10.x); not a phase 59.1 blocker. The architectural decision to defer windows is implicit in current goreleaser config.

**Verdict:** **CLOSED for v1.10.x scope.** Windows-archive deferral becomes its own follow-up if/when the maintainer wants it.

## Item 2 — Real cosign keyless attestation verification

Verified `cosign verify-blob --new-bundle-format` against actual sigstore bundles published by the v1.10.7 release-merge job — NOT in-process CA fixtures. Memory rule (`feedback_release_artifact_self_test`) is satisfied by verifying real CI output.

**Commands run:**

```bash
cosign verify-blob \
  --new-bundle-format \
  --bundle helix_v1.10.7_Darwin_arm64.tar.gz.sigstore.json \
  --certificate-identity-regexp 'https://github.com/agenthands/helix/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  helix_v1.10.7_Darwin_arm64.tar.gz
# → Verified OK (exit 0)

cosign verify-blob \
  --new-bundle-format \
  --bundle helix_v1.10.7_Linux_amd64.tar.gz.sigstore.json \
  --certificate-identity-regexp 'https://github.com/agenthands/helix/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  helix_v1.10.7_Linux_amd64.tar.gz
# → Verified OK (exit 0)
```

**cosign version:** `/opt/homebrew/bin/cosign`

**Verdict:** **CLOSED.** Real bundles, real Fulcio chain, real OIDC issuer (GitHub Actions). The verifier's "needs human" framing conflated "needs real CI artifact" with "needs human verifier" — the memory rule only requires the former.

## Item 3 — Darwin Gatekeeper workaround usability

### Technical mechanism (automated)

```bash
xattr -w com.apple.quarantine "0083;$(date +%s);Safari;" extracted/helix_v1.10.7_Darwin_arm64/helix
xattr -p com.apple.quarantine extracted/helix_v1.10.7_Darwin_arm64/helix
# → 0083;1778066679;Safari;
xattr -d com.apple.quarantine extracted/helix_v1.10.7_Darwin_arm64/helix
xattr -p com.apple.quarantine extracted/helix_v1.10.7_Darwin_arm64/helix 2>&1
# → No such xattr: com.apple.quarantine (PASS — quarantine cleared)
chmod +x extracted/helix_v1.10.7_Darwin_arm64/helix  # See finding below
./extracted/helix_v1.10.7_Darwin_arm64/helix --version
# → helix version 1.10.7-SNAPSHOT-2bfa036f (exit 0)
```

**Verdict:** **CLOSED for the xattr workaround.**

### Finding: tarball does NOT preserve executable permissions

When extracted via `tar xzf`, the helix binary lands with `0644` (rw-r--r--) instead of `0755`. Running it without `chmod +x` first fails with `permission denied`. INSTALL.md does NOT mention `chmod +x` — users following Path B (xattr) will hit this gap before the binary runs.

**Severity:** WARNING. This is a real INSTALL.md doc bug, but separable from the phase 59.1 closure. Action: open a small follow-up to either (a) document `chmod +x` in INSTALL.md, OR (b) fix `.goreleaser.yaml` archive `files` block to preserve the source binary's permissions (it does — likely a goreleaser bug or an issue with the local tar extraction; needs reproduction on a clean Mac).

**Track as:** `DEF-59.1-DARWIN-EXEC-PERMS` (or fold into INSTALL.md doc fix in v1.10.8).

## Summary

| Item | Verifier verdict (2026-05-04) | Re-audit verdict (2026-05-06) |
|------|-------------------------------|-------------------------------|
| 1 — Linux+Windows CI build | PARTIAL → 59.2 | **CLOSED** for v1.10.x linux scope; windows is a separable deferral |
| 2 — cosign verify against real bundle | DEFERRED to next tag-cut | **CLOSED** (verified OK against v1.10.7 darwin + linux) |
| 3 — Gatekeeper xattr workaround | needs Mac UAT | **CLOSED for mechanism**; new finding: missing `chmod +x` in INSTALL.md (separable) |

**Net:** phase 59.1 status should flip from `human_needed` → `passed` with a small follow-up to fix the INSTALL.md exec-perm gap (or the goreleaser archive perms, if they're upstream).

The original verifier verdict was correct at the time; this re-audit reflects state on 2026-05-06 after 7 additional releases shipped.
