---
phase: 58-v1-9-carryover-release-distribution
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .goreleaser.yaml
  - .github/workflows/release.yml
autonomous: true
requirements: [REL-01]
must_haves:
  truths:
    - "goreleaser snapshot build produces a .sigstore.json bundle alongside every archive"
    - "release.yml declares id-token: write so cosign keyless OIDC can mint a Fulcio cert"
    - "release.yml has no remaining minisign references (PLACEHOLDER pre-flight grep, key-write step, password env line, key-wipe step all removed)"
    - "cosign-installer is pinned by 40-char commit SHA with a trailing version comment"
  artifacts:
    - path: ".goreleaser.yaml"
      provides: "cosign keyless signs: block in place of the prior minisign block"
      contains: "id: cosign"
    - path: ".github/workflows/release.yml"
      provides: "id-token: write permission + sigstore/cosign-installer step"
      contains: "id-token: write"
  key_links:
    - from: ".github/workflows/release.yml"
      to: ".goreleaser.yaml signs.cosign block"
      via: "goreleaser invocation in 'Real release' step (workflow already runs goreleaser; cosign installer must be on PATH before that step)"
      pattern: "sigstore/cosign-installer@"
---

<objective>
Swap the release-side signing infrastructure from minisign to sigstore cosign keyless. After this plan, `goreleaser release --snapshot --skip=publish` produces `.sigstore.json` bundles for every archive, and the live release workflow has the OIDC permission + cosign installer required for the real v1.10.0 tag push. This is Wave 1 because it does not touch any Go code; Plan 02 (verifier rewrite) consumes the new artifact format and therefore depends on this plan.

Purpose: REL-01 release side (D-02 cosign keyless, D-03 hard-cut). Closes "where do we store the private key" by eliminating the keypair entirely.
Output: Updated `.goreleaser.yaml` and `.github/workflows/release.yml` with cosign keyless signing wired and all minisign ceremony deleted.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md
@.goreleaser.yaml
@.github/workflows/release.yml
</context>

<tasks>

<task type="auto">
  <name>Task 1: Replace minisign signs: block with cosign keyless in .goreleaser.yaml</name>
  <files>.goreleaser.yaml</files>
  <read_first>
    - .goreleaser.yaml (full file — locked-decisions header at top, signs: block at lines 50-63)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §".goreleaser.yaml (release config, build-time)" — exact replacement block
    - .planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md D-02 (cosign keyless rationale) and D-03 (hard cut)
  </read_first>
  <action>
    Edit `.goreleaser.yaml`:

    1. Replace the existing `signs:` block (lines 50-63 in the current file — the minisign block per PATTERNS) with the cosign block from PATTERNS:
    ```yaml
    signs:
      - id: cosign
        cmd: cosign
        artifacts: all
        signature: "${artifact}.sigstore.json"
        args:
          - "sign-blob"
          - "--bundle=${signature}"
          - "${artifact}"
          - "--yes"
    ```
    Per D-02: keyless OIDC, no `stdin:`, no env-var password.

    2. Append a Phase 58 reference to the locked-decisions header at the top of `.goreleaser.yaml`. Follow the existing header convention by adding a single-line comment naming Phase 58 D-02 (cosign keyless) and D-03 (hard cut). Example shape (preserve existing wording style):
    ```yaml
    # Phase 58 D-02/D-03: cosign keyless replaces minisign at v1.10.0 (hard cut).
    ```

    Do NOT modify any other section of the file (build matrix, archives, checksum, release sections all unchanged). Do NOT add a `cosign verify` step here — verification is the consumer's responsibility (Plan 02).
  </action>
  <verify>
    <automated>grep -q 'id: cosign' .goreleaser.yaml &amp;&amp; grep -q 'sign-blob' .goreleaser.yaml &amp;&amp; grep -q '\.sigstore\.json' .goreleaser.yaml &amp;&amp; ! grep -q 'minisign' .goreleaser.yaml &amp;&amp; ! grep -q 'MINISIGN_PASSWORD' .goreleaser.yaml</automated>
  </verify>
  <acceptance_criteria>
    - `grep -c 'id: cosign' .goreleaser.yaml` returns 1
    - `grep -c 'minisign' .goreleaser.yaml` returns 0
    - `grep -c 'sigstore.json' .goreleaser.yaml` returns at least 1 (signature template)
    - `grep -c 'Phase 58' .goreleaser.yaml` returns at least 1 (decision-tag comment present)
    - File still parses as valid YAML (verified by goreleaser snapshot in Task 3)
  </acceptance_criteria>
  <done>
    Cosign keyless signs: block lives in `.goreleaser.yaml`. Zero minisign references remain. Phase 58 decision tag present in the file header.
  </done>
</task>

<task type="auto">
  <name>Task 2: Strip minisign ceremony from release.yml and add cosign-installer + id-token permission</name>
  <files>.github/workflows/release.yml</files>
  <read_first>
    - .github/workflows/release.yml (full file — permissions block lines 8-9; minisign steps to delete at lines 25-36, 38-45, 53-89, 131-145, 155, 157-164 per PATTERNS)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §".github/workflows/release.yml" — full enumeration of steps to delete + add
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"Pinned action SHAs in CI workflows"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`set -euo pipefail` shell preamble"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"Phase-/decision-tagged inline comments"
  </read_first>
  <action>
    Edit `.github/workflows/release.yml`:

    1. **Permissions block** (current lines 8-9 per PATTERNS): add `id-token: write` for cosign keyless OIDC. After edit:
    ```yaml
    permissions:
      contents: write   # required: create release, upload assets
      id-token: write   # Phase 58 D-02: cosign keyless OIDC token (Fulcio short-lived cert)
    ```

    2. **Delete all minisign ceremony steps** (per PATTERNS exact line ranges):
       - Lines 25-36: "Refuse PLACEHOLDER minisign public key" entire step
       - Lines 38-45: "Verify embedded minisign.pub matches repo-root" entire step
       - Lines 53-89: "Install minisign (pinned)" entire 37-line block
       - Lines 131-145: "Write minisign secret key to disk (umask 077)" entire step
       - Line 155: `MINISIGN_PASSWORD: ${{ secrets.MINISIGN_PASSWORD }}` env line in "Real release" step
       - Lines 157-164: "Wipe minisign secret key" entire step

       (Line numbers above are AS-OF the current file per PATTERNS; if numbers have drifted slightly, match by step `name:` rather than line number.)

    3. **Add a `Install cosign` step** before the existing "Real release" goreleaser invocation. Pattern from PATTERNS §".github/workflows/release.yml":
    ```yaml
    - name: Install cosign  # Phase 58 D-02: keyless signing via Fulcio + Rekor.
      uses: sigstore/cosign-installer@<PIN_SHA>  # v3.x.x
      with:
        cosign-release: 'v2.4.1'
    ```
    Resolve `<PIN_SHA>` to the 40-char commit SHA for `sigstore/cosign-installer` v3.7.0 (or the current latest v3.x as of plan execution). The pinning comment MUST follow the existing convention `# vX.Y.Z` (per PATTERNS "Pinned action SHAs"). Place this step AFTER `actions/checkout` and BEFORE the `goreleaser-action@...` snapshot/release invocation so cosign is on PATH when goreleaser shells out.

    4. **Inline-comment style:** preserve the existing `# WR-XX` / `# CR-XX` cross-reference style for any comment additions; tag new comments `# Phase 58 D-02` per PATTERNS §"Phase-/decision-tagged inline comments".

    Do NOT touch the reproducibility-gate steps (Pass-1, Pass-2, Pass-3) — REL-05 doc-only changes land in Plan 04. Do NOT add a `cosign verify` step here — verifier exercise is for Plan 02 / manual post-release.
  </action>
  <verify>
    <automated>grep -q 'id-token: write' .github/workflows/release.yml &amp;&amp; grep -q 'sigstore/cosign-installer@' .github/workflows/release.yml &amp;&amp; ! grep -q 'minisign' .github/workflows/release.yml &amp;&amp; ! grep -q 'MINISIGN_PASSWORD' .github/workflows/release.yml &amp;&amp; ! grep -q 'PLACEHOLDER' .github/workflows/release.yml</automated>
  </verify>
  <acceptance_criteria>
    - `grep -c 'id-token: write' .github/workflows/release.yml` returns 1
    - `grep -c 'sigstore/cosign-installer@' .github/workflows/release.yml` returns exactly 1
    - The cosign-installer line matches `uses: sigstore/cosign-installer@[a-f0-9]\{40\}  # v[0-9]` (40-char SHA + version comment)
    - `grep -c 'minisign' .github/workflows/release.yml` returns 0
    - `grep -c 'MINISIGN_PASSWORD' .github/workflows/release.yml` returns 0
    - `grep -c 'PLACEHOLDER' .github/workflows/release.yml` returns 0 (the pre-flight grep step is gone)
    - `actionlint .github/workflows/release.yml` (if installed) reports no errors; otherwise file is valid YAML and has no `${{ secrets.MINISIGN_* }}` interpolations.
  </acceptance_criteria>
  <done>
    `release.yml` declares `id-token: write`, installs cosign with a SHA-pinned action, and contains zero minisign references.
  </done>
</task>

<task type="auto">
  <name>Task 3: Verify snapshot build produces .sigstore.json bundles</name>
  <files></files>
  <read_first>
    - .goreleaser.yaml (post-Task-1 state)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md row 58-P1 (snapshot verification command)
  </read_first>
  <action>
    Run a local goreleaser snapshot build to confirm the cosign block invokes correctly. This requires `cosign` v2.4+ on PATH (install locally if missing — `brew install cosign` on macOS or download from sigstore releases pinned to v2.4.x).

    Local cosign keyless sign-blob WILL prompt for OIDC interactively unless `--yes` is passed (already in args) and `COSIGN_EXPERIMENTAL=1` set OR an OIDC token is available. For local snapshot validation, set `COSIGN_YES=true` and run with the local-keyless path. Acceptable shape:

    ```bash
    COSIGN_YES=true goreleaser release --snapshot --skip=publish --clean
    ```

    Confirm `dist/` contains one `.sigstore.json` bundle per archive (6 archives → 6 bundles per the multi-arch matrix). If local OIDC is unavailable (typical on a dev workstation without sigstore browser flow), the goreleaser snapshot is still considered passing if the `signs:` block is invoked and produces a `cosign sign-blob` command that fails ONLY on OIDC token acquisition (not on YAML parse, not on missing cosign binary). Document the local-vs-CI distinction in the plan SUMMARY.

    If the snapshot fails for any non-OIDC reason (YAML parse, cosign-not-on-PATH, wrong arg name), revise Task 1 and rerun.

    **No code edits in this task** — it is a verification gate. If verification fails, the plan re-enters Task 1 with the specific error.
  </action>
  <verify>
    <automated>command -v cosign &gt;/dev/null &amp;&amp; command -v goreleaser &gt;/dev/null &amp;&amp; set -o pipefail &amp;&amp; (COSIGN_YES=true goreleaser release --snapshot --skip=publish --skip=announce --clean 2&gt;&amp;1 | tee /tmp/58-01-snapshot.log) &amp;&amp; (ls dist/*.sigstore.json 2&gt;/dev/null | head -1 || grep -q 'cosign sign-blob' /tmp/58-01-snapshot.log) &amp;&amp; ! grep -E 'yaml: line|error parsing' /tmp/58-01-snapshot.log</automated>
  </verify>
  <acceptance_criteria>
    - `set -o pipefail` is honored: if goreleaser exits non-zero, the verify gate fails (no silent OR-fallback)
    - Either: `dist/` contains at least one `*.sigstore.json` file after the snapshot run (full local OIDC available), OR
    - `/tmp/58-01-snapshot.log` shows goreleaser invoked the `cosign sign-blob` command with the configured args (OIDC-acquisition failure is acceptable on dev workstation; CI will succeed)
    - **No YAML parse errors:** `! grep -E 'yaml: line|error parsing' /tmp/58-01-snapshot.log` (split assertion — the previous OR-fallback could mask YAML parse failures)
    - Cosign binary is on PATH at v2.4+ (`cosign version` reports v2.4 or later)
    - The local result is captured in the plan SUMMARY with a note distinguishing "local snapshot OIDC-gated" vs "CI snapshot will succeed end-to-end"
  </acceptance_criteria>
  <done>
    Snapshot build either produces `.sigstore.json` bundles locally OR demonstrably invokes the cosign sign-blob command (OIDC failure on a dev workstation is acceptable). Plan SUMMARY documents the local vs CI status.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| GitHub Actions runner → Fulcio | OIDC token exchange for short-lived signing certificate |
| GitHub Actions runner → Rekor | Signature submission for transparency-log inclusion |
| GitHub Releases → end-user | Released `.sigstore.json` bundle is the verification root for `helix upgrade` |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-58-01 | Tampering | release-pipeline integrity | mitigate | `id-token: write` permission scoped to release workflow only; cosign-installer pinned to 40-char commit SHA (rejects supply-chain tag rewrites per PATTERNS "Pinned action SHAs"); no long-lived signing key exists to compromise (D-02) |
| T-58-02 | Spoofing | release artifact origin | mitigate (server-side coverage) | Plan 01 ships the `.sigstore.json` bundle; Plan 02 enforces verifier-side identity pinning (deferred to Plan 02 acceptance criteria) |
| T-58-01-aux | Elevation of Privilege | overly-broad workflow token | mitigate | `id-token: write` is added at the workflow scope where `contents: write` already lives (no new attack surface beyond the pre-existing release-publish privilege) |
</threat_model>

<verification>
- `goreleaser release --snapshot --skip=publish` produces `.sigstore.json` bundles per archive (or, on a dev workstation without local OIDC, demonstrably invokes the cosign sign-blob command with correct args)
- `release.yml` syntax is valid; `actionlint` passes if installed
- No grep hits for `minisign`, `MINISIGN_PASSWORD`, or `PLACEHOLDER` in either file post-edit
- `id-token: write` permission is present in `release.yml`
- Cosign-installer step is SHA-pinned with a `# vX.Y.Z` trailing comment
</verification>

<success_criteria>
- `.goreleaser.yaml` `signs:` block is the cosign keyless block from PATTERNS, verbatim
- `.github/workflows/release.yml` has zero minisign ceremony, `id-token: write` permission, and a SHA-pinned `sigstore/cosign-installer` step before the goreleaser invocation
- Snapshot build succeeds (or fails only on local OIDC, which is acceptable)
- Plan 02 can land without further release-side blockers (its verifier consumes `.sigstore.json` artifacts that this plan now guarantees)
</success_criteria>

<output>
After completion, create `.planning/phases/58-v1-9-carryover-release-distribution/58-01-SUMMARY.md` documenting:
- Whether local snapshot produced bundles or only invoked the cosign command (OIDC-gated)
- The exact cosign-installer SHA chosen
- Any deviations from PATTERNS line numbers (if the file had drifted)
- Confirmation that Plan 02 may now proceed (no remaining release-side blockers)
</output>
