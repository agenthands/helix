---
phase: 51-packaging-goreleaser
plan: 06
type: execute
wave: 3
depends_on: [51-01, 51-02]
files_modified:
  - .github/workflows/release.yml
  - CONTRIBUTING.md
  - minisign.pub
autonomous: true
user_setup:
  - service: minisign-keypair
    why: "release.yml's PLACEHOLDER pre-flight (Task 1) refuses to publish until minisign.pub contains a real key. The keypair must be generated on a trusted maintainer machine; the private half must be uploaded as a GitHub Actions secret; the public half must overwrite the placeholder file in this repo."
    env_vars:
      - name: MINISIGN_PRIVATE_KEY
        source: "Generated locally via `minisign -G -p minisign.pub -s minisign.key` on a trusted machine; upload the contents of the resulting `minisign.key` file via `gh secret set MINISIGN_PRIVATE_KEY < minisign.key` (or paste into GitHub repo Settings > Secrets and variables > Actions)"
      - name: MINISIGN_PASSWORD
        source: "Password chosen during `minisign -G` keypair generation (per D-02a + Open Question 2: password-protected key); upload via `gh secret set MINISIGN_PASSWORD` (interactive prompt) or the GitHub web UI"
    dashboard_config:
      - task: "Generate the minisign keypair on a trusted local machine"
        location: "Maintainer's local shell: `minisign -G -p minisign.pub -s minisign.key`"
      - task: "Upload MINISIGN_PRIVATE_KEY secret"
        location: "Run `gh secret set MINISIGN_PRIVATE_KEY < minisign.key`, OR GitHub repo Settings > Secrets and variables > Actions > New repository secret"
      - task: "Upload MINISIGN_PASSWORD secret"
        location: "Run `gh secret set MINISIGN_PASSWORD` (interactive prompt), OR GitHub repo Settings > Secrets and variables > Actions"
      - task: "Commit a real minisign.pub overwriting the placeholder"
        location: "Repo root: replace BOTH lines of `minisign.pub` with the real keypair output -- (a) the comment line MUST NOT contain the literal word `PLACEHOLDER` (release.yml Task 1 grep-fails on it), and (b) line 2 MUST be the real RWQ-prefixed Ed25519 public key from the local keypair generation, not the all-zeros placeholder"
gap_closure: true
requirements: [PKG-01]
requirements_addressed: [PKG-01]
tags: [packaging, ci, supply-chain, threat-model, gap-closure, cr-02, cr-03, cr-04, wr-01, wr-06, in-02, in-03, in-04]
must_haves:
  truths:
    - "CLOSES VERIFICATION concern B (CI pre-flight) -- release.yml fails the workflow before publishing if minisign.pub still contains the PLACEHOLDER marker (CR-02). Maintainer cannot accidentally ship a release that no user can verify."
    - "CLOSES VERIFICATION concern C -- the doc and the gate agree on what reproducibility is enforced. CONTRIBUTING.md:161 is softened to say 'two consecutive snapshot builds with identical inputs' rather than 'a non-deterministic build cannot reach users'. The gate is unchanged (snapshot-vs-snapshot is what is enforced); the doc no longer overstates it. Reference: CR-03, IN-04."
    - "CLOSES VERIFICATION concern E -- third-party actions are pinned by full commit SHA instead of mutable major-version tags (WR-01); minisign tarball is verified against a published SHA-256 before extraction (CR-04); /tmp/minisign.key is wiped in an if: always() post-step (WR-06); minisign URL is parameterized by uname -m so a future runner-arch flip fails loud instead of 404'ing cryptically (IN-02); dist-pass-1 is cleaned after the gate to keep the runner workspace tidy (IN-03)."
    - "An attacker who repoints actions/checkout@v4 to a malicious commit cannot exfiltrate MINISIGN_PRIVATE_KEY -- the workflow pins by commit SHA"
    - "If GitHub release infrastructure for jedisct1/minisign serves a compromised tarball, release.yml's sha256 verify step fails the workflow before the unsigned binary writes any signatures"
    - "If a future step (e.g. announce-on-Slack) tries to read /tmp/minisign.key after the signing step, the file is already shredded by the if: always() cleanup"
    - "minisign.pub still contains the PLACEHOLDER literal until the maintainer overwrites it offline; the workflow's pre-flight catches this on every release attempt and refuses to publish"
  artifacts:
    - path: ".github/workflows/release.yml"
      provides: "Hardened release workflow: SHA-pinned third-party actions, minisign tarball SHA-256 verification, runner-arch sanity check, /tmp/minisign.key shred-on-always cleanup, dist-pass-1 cleanup, PLACEHOLDER pre-flight check"
      contains: ["actions/checkout@", "actions/setup-go@", "goreleaser/goreleaser-action@", "MINISIGN_TARBALL_SHA256", "sha256sum -c", "if: always()", "shred -u /tmp/minisign.key", "PLACEHOLDER", "uname -m"]
    - path: "CONTRIBUTING.md"
      provides: "Releasing section's reproducibility paragraph softened to match what the gate actually proves (snapshot-pair determinism, not real-release determinism)"
      contains: ["two consecutive snapshot builds with identical inputs"]
    - path: "minisign.pub"
      provides: "Comment line tightened so the maintainer dropping in the real key does not have to remember to remove the word PLACEHOLDER -- the marker is the workflow gate, not aesthetic prose; key body unchanged from 51-01 (still placeholder until maintainer offline action)"
      contains: ["PLACEHOLDER", "RWQ"]
  key_links:
    - from: ".github/workflows/release.yml pre-flight step"
      to: "minisign.pub at repo root"
      via: "grep -q 'PLACEHOLDER' minisign.pub -> exit 1 if match"
      pattern: "grep -q.*PLACEHOLDER.*minisign\\.pub"
    - from: ".github/workflows/release.yml minisign install step"
      to: "jedisct1/minisign release tarball SHA-256"
      via: "sha256sum -c with a pinned MINISIGN_TARBALL_SHA256 constant"
      pattern: "MINISIGN_TARBALL_SHA256|sha256sum -c"
    - from: ".github/workflows/release.yml cleanup step"
      to: "/tmp/minisign.key file"
      via: "if: always() shred -u || rm -f"
      pattern: "shred -u /tmp/minisign\\.key"
    - from: "CONTRIBUTING.md Releasing -> reproducibility paragraph"
      to: ".github/workflows/release.yml diff gate"
      via: "doc accurately describes what the gate validates (snapshot-pair determinism, not real-release determinism)"
      pattern: "two consecutive snapshot builds with identical inputs"
---

<objective>
Close VERIFICATION Concerns B (CI pre-flight half), C (doc/gate truth alignment), and E (supply-chain hardening). All three live primarily in `.github/workflows/release.yml` plus a small CONTRIBUTING.md text edit and a minisign.pub comment update. Combining them in a single plan with single ownership of release.yml prevents merge conflicts with Plan 51-05 (which owns .goreleaser.yaml + INSTALL.md + Makefile, no release.yml overlap) and Plan 51-04 (README.md only).

Concrete fixes:
- **CR-02 / Concern B (CI pre-flight):** Add a workflow step that grep-fails on the PLACEHOLDER marker in `minisign.pub` BEFORE the reproducibility gate runs. If the maintainer pushes a tag with the placeholder still in main, the workflow fails closed before any artifact is built or published. Real keypair generation remains a `user_setup` item the maintainer performs offline -- NOT in scope for this plan, but the gate makes "shipped a placeholder release" structurally impossible.
- **CR-03 + IN-04 / Concern C (doc vs gate):** Soften CONTRIBUTING.md:161 to say "two consecutive snapshot builds with identical inputs" instead of "a non-deterministic build cannot reach users". The gate is unchanged (snapshot-vs-snapshot is what it actually proves); the doc now matches reality. Extending the gate to compare real-release artifacts (the alternative fix in CR-03) would require a Pass 4 build with `--skip=publish --skip=sign` after the real release; that doubles CI time on every tag push and is a larger change. Doc-softening is the cheaper of the two CR-03 closure paths and is what this plan picks.
- **CR-04 / Concern E (minisign tarball verification):** Pin the SHA-256 of the minisign tarball; verify before extracting. If GitHub or jedisct1's account is compromised, the verification fails and the workflow stops before the compromised binary handles the project's private key. The pinned SHA-256 is fetched from the upstream release's `signing.txt` or computed locally and recorded; the executor verifies the value against the upstream signed file before committing.
- **WR-01 / Concern E (action pinning):** Replace `actions/checkout@v4`, `actions/setup-go@v5`, `goreleaser/goreleaser-action@v7` with full commit SHA pins (with the version tag in a trailing comment for human readability). A repointed major tag cannot exfiltrate the signing key.
- **WR-06 / Concern E (key cleanup):** Add an `if: always()` post-step that shreds (or rm-fs) `/tmp/minisign.key`. Reduces in-job blast radius if a future step is added that pulls a third-party action.
- **IN-02 / Concern E (arch guard):** Replace the hardcoded `linux-x86_64` minisign URL with a uname-m sanity check that fails loud if ubuntu-latest ever flips to a non-x86_64 arch.
- **IN-03 / Concern E (workspace cleanup):** Remove `dist-pass-1` after the diff gate succeeds. Cosmetic on ephemeral runners but tidy.

The minisign.pub comment update is a tiny doc-only change so the marker the workflow grep-fails on is purely the literal string `PLACEHOLDER`. The current comment "PLACEHOLDER, replace before first release" already contains the marker; no change is strictly necessary, but this plan adds an explicit cross-reference line in the comment pointing to the workflow gate for future maintainers reading the file.

Output: A release.yml that fails closed on every supply-chain footgun the review identified, a CONTRIBUTING.md whose reproducibility paragraph accurately describes the gate, and a minisign.pub whose PLACEHOLDER marker is documented as load-bearing for the CI gate.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/51-packaging-goreleaser/51-CONTEXT.md
@.planning/phases/51-packaging-goreleaser/51-RESEARCH.md
@.planning/phases/51-packaging-goreleaser/51-VERIFICATION.md
@.planning/phases/51-packaging-goreleaser/51-REVIEW.md
@.github/workflows/release.yml
@CONTRIBUTING.md
@minisign.pub

<interfaces>
release.yml structural skeleton (committed in 51-01, lines 1-95):

Order of steps in jobs.release.steps (existing):
  1. Checkout (uses: actions/checkout@v4)
  2. Set up Go (uses: actions/setup-go@v5)
  3. Install minisign (pinned)            <-- needs SHA-256 verification (CR-04)
                                             needs uname -m sanity (IN-02)
  4. Write minisign secret key to disk (umask 077)
  5. Reproducibility gate -- snapshot pass 1 (uses: goreleaser/goreleaser-action@v7)
  6. Capture pass 1 sha256s (mv dist dist-pass-1)
  7. Reproducibility gate -- snapshot pass 2 (uses: goreleaser/goreleaser-action@v7)
  8. Diff sha256s -- fail if non-reproducible       <-- after this, dist-pass-1 should be cleaned (IN-03)
  9. Real release (sign + publish) (uses: goreleaser/goreleaser-action@v7)

This plan inserts:
  Between steps 1 and 2:  "Refuse PLACEHOLDER minisign.pub" (CR-02 pre-flight)
  After step 8:           "Clean dist-pass-1" (IN-03)
  After step 9 (last):    "Wipe minisign secret key" (if: always(), WR-06)

This plan modifies:
  Step 3:  Add MINISIGN_TARBALL_SHA256 + sha256sum -c (CR-04); add uname -m guard (IN-02)
  Steps 1, 2, 5, 7, 9: Replace @v4 / @v5 / @v7 tags with full commit SHAs (WR-01)

Action commit SHA references (verify before pinning -- do not blindly trust the values below):

  actions/checkout@v4 (current pin target):
    Latest as of 2026-04: v4.2.2 = b4ffde65f46336ab88eb53be808477a3936bae11
    The executor MUST run: gh api repos/actions/checkout/git/refs/tags/v4.2.2 to confirm
    the SHA before committing.

  actions/setup-go@v5:
    Latest as of 2026-04: v5.2.0 = 41dfa10bad2bb2ae585af6ee5bb4d7d973ad74ed
    Verify with: gh api repos/actions/setup-go/git/refs/tags/v5.2.0

  goreleaser/goreleaser-action@v7:
    Latest as of 2026-04: v7.0.0 = 9c156ee8a17a598857849441385a2041ef570552
    Verify with: gh api repos/goreleaser/goreleaser-action/git/refs/tags/v7.0.0

  Minisign tarball SHA-256 for v0.12 linux-x86_64:
    Upstream signing.txt at https://github.com/jedisct1/minisign/releases/download/0.12/signing.txt
    The executor downloads this file (or fetches the precomputed SHA-256 from
    the release notes) and pastes the value into release.yml's
    MINISIGN_TARBALL_SHA256 variable. Document the source URL in a comment.

The executor is responsible for re-confirming each SHA against the upstream
source at execution time -- the values above may have rotated by the time
this plan executes.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Add PLACEHOLDER pre-flight + dist-pass-1 cleanup + always() key cleanup steps to release.yml</name>
  <files>.github/workflows/release.yml</files>
  <read_first>
    - .github/workflows/release.yml full file (95 lines committed in Plan 51-01)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md CR-02 (the pre-flight step text), WR-06 (the cleanup step text), IN-03 (the dist-pass-1 cleanup mention)
    - minisign.pub (confirm the literal substring "PLACEHOLDER" is present on line 1; this is what the pre-flight greps for)
  </read_first>
  <action>
Insert THREE new steps in `.github/workflows/release.yml`:

1. **PLACEHOLDER pre-flight** -- inserted between the existing "Checkout" step (line ~17-20) and the "Set up Go" step (line ~22-26). Order matters: the workflow must fail BEFORE downloading Go or building anything if the public key is the placeholder, so wasted CI minutes are minimized.

2. **dist-pass-1 cleanup** -- inserted AFTER the "Diff sha256s -- fail if non-reproducible" step (current step 8, line ~75-84) and BEFORE the "Real release" step (current step 9, line ~86-94).

3. **Wipe minisign secret key** -- inserted AS THE LAST STEP of the job (after the existing "Real release" step). Uses `if: always()` so it runs whether the real release succeeds, fails, or is cancelled.

**Step A: Insert PLACEHOLDER pre-flight after the Checkout step.**

After the existing checkout block (which ends at line ~20 with `fetch-depth: 0`), add:

      - name: Refuse PLACEHOLDER minisign public key
        run: |
          set -euo pipefail
          # If minisign.pub still contains the literal "PLACEHOLDER" marker, the
          # maintainer has not yet generated and committed the real public key.
          # Publishing a release in that state would silently break verification
          # for every user (CR-02). Fail closed.
          if grep -q 'PLACEHOLDER' minisign.pub; then
            echo "::error file=minisign.pub::minisign.pub still contains the PLACEHOLDER marker; replace with the real public key before tagging a release. See CONTRIBUTING.md (Releasing > One-time keypair setup)."
            exit 1
          fi
          echo "minisign.pub pre-flight passed (no PLACEHOLDER marker found)."

Indentation: 6 spaces for `- name:` (matching the surrounding step blocks), 8 spaces for `run:`, and 10 spaces for the script body lines (matching the existing run-block bodies in the file -- consistent with how the existing "Install minisign (pinned)" step at lines 28-39 is indented).

**Step B: Insert dist-pass-1 cleanup between the diff step and the real release step.**

After the existing "Diff sha256s -- fail if non-reproducible" step (which ends with `echo "Reproducibility gate PASS -- proceeding to real release"` on line ~84), add:

      - name: Clean dist-pass-1 after diff gate
        run: |
          set -euo pipefail
          # The reproducibility gate kept dist-pass-1 around for the diff; remove
          # it before the real release pass so workspace caches stay tidy (IN-03).
          rm -rf dist-pass-1

**Step C: Insert minisign key cleanup as the last step of the job.**

After the existing "Real release (sign + publish)" step (which ends at line ~94), add:

      - name: Wipe minisign secret key
        if: always()
        run: |
          # Defense-in-depth: shred the in-job copy of the secret key as soon as
          # signing is complete (or aborted). The runner is ephemeral so this is
          # belt-and-suspenders, but it shrinks the in-job blast radius if a
          # future step pulls a third-party action that scans /tmp (WR-06).
          shred -u /tmp/minisign.key 2>/dev/null || rm -f /tmp/minisign.key

The `if: always()` MUST be on its own line at the step level (not nested inside `with:`). It runs the step regardless of whether prior steps succeeded, failed, were cancelled, or were skipped -- the only way the cleanup gets skipped is if the runner itself crashes before the step is reached, which is a runner-failure mode the cleanup cannot help with anyway.

Style non-negotiables:
1. Three new step blocks added; existing 9 steps preserved byte-identical (other than what Tasks 2 and 3 modify -- the action SHAs and the minisign install body).
2. Indentation MUST match the existing file's pattern: 4 spaces for the `-` bullet of each step, 6 spaces for the `name:` / `run:` / `if:` / `uses:` keys, 8 spaces for `with:` keys, 10 spaces for run-block body content.
3. The pre-flight step MUST run BEFORE the Set up Go step. Order is enforced by position in the steps list (no `needs:` -- it is a single job with sequential steps).
4. The cleanup step MUST be the absolute last step. Adding any step after it would cause that later step to run before the cleanup and possibly read `/tmp/minisign.key`.
5. Each new step has a meaningful `name:` for the GitHub Actions UI; do not use generic "Step 1 / Step 2".
6. After all three insertions, `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` MUST exit 0 (YAML still valid).
7. Do NOT touch the existing `secrets.MINISIGN_PRIVATE_KEY` reference, the `--skip=sign` arguments, the `printf '%s'` line, the `umask 077` line, the `diff -u /tmp/pass1.sha256 /tmp/pass2.sha256` line, or any of the goreleaser-action invocations' `args:` (those are owned by 51-01's structural skeleton; this task adds steps around them, not inside them).
  </action>
  <verify>
    <automated>grep -q "name: Refuse PLACEHOLDER minisign public key" .github/workflows/release.yml && grep -q "grep -q 'PLACEHOLDER' minisign.pub" .github/workflows/release.yml && grep -q "name: Clean dist-pass-1 after diff gate" .github/workflows/release.yml && grep -q "rm -rf dist-pass-1" .github/workflows/release.yml && grep -q "name: Wipe minisign secret key" .github/workflows/release.yml && grep -q "if: always()" .github/workflows/release.yml && grep -q "shred -u /tmp/minisign.key" .github/workflows/release.yml && grep -q "rm -f /tmp/minisign.key" .github/workflows/release.yml && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && awk '/name: Refuse PLACEHOLDER/{found_pre=NR} /name: Set up Go/{if (found_pre && NR>found_pre) {pre_before_go=1}} END {exit !pre_before_go}' .github/workflows/release.yml && awk '/name: Clean dist-pass-1/{cleanup=NR} /name: Real release/{if (cleanup && NR>cleanup) {cleanup_before_release=1}} END {exit !cleanup_before_release}' .github/workflows/release.yml && awk '/name: Wipe minisign secret key/{wipe=NR} /name: Real release/{real=NR} END {exit !(wipe>real)}' .github/workflows/release.yml</automated>
  </verify>
  <acceptance_criteria>
    - .github/workflows/release.yml contains a step `name: Refuse PLACEHOLDER minisign public key`
    - That step's body contains `grep -q 'PLACEHOLDER' minisign.pub` and `exit 1`
    - That step's body uses `::error file=minisign.pub::` annotation form
    - The pre-flight step appears BEFORE the `name: Set up Go` step (line-order check via awk)
    - .github/workflows/release.yml contains a step `name: Clean dist-pass-1 after diff gate` with body `rm -rf dist-pass-1`
    - The cleanup step appears AFTER the diff gate step (`name: Diff sha256s ...`) and BEFORE `name: Real release`
    - .github/workflows/release.yml contains a step `name: Wipe minisign secret key` with `if: always()` and a body containing `shred -u /tmp/minisign.key 2>/dev/null || rm -f /tmp/minisign.key`
    - The wipe step is the LAST step in the job (no subsequent steps after it)
    - YAML parses: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` exits 0
    - The existing 9 steps are byte-identical (other than what Tasks 2 and 3 modify -- this task adds 3 steps without altering the others)
  </acceptance_criteria>
  <done>The workflow now fails closed BEFORE building anything if minisign.pub is still the placeholder, cleans dist-pass-1 after the diff gate, and shreds the in-job secret key copy whether the release succeeds or fails. Concern B's CI pre-flight half (CR-02), WR-06, and IN-03 closed.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Pin third-party actions to commit SHAs and verify minisign tarball SHA-256</name>
  <files>.github/workflows/release.yml</files>
  <read_first>
    - .github/workflows/release.yml (re-read after Task 1 inserted the three new steps; the action `uses:` lines to be modified are at the existing step positions: Checkout, Set up Go, the three goreleaser-action invocations)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md WR-01 (action pinning text), CR-04 (minisign tarball verification text), IN-02 (arch guard text)
    - https://github.com/actions/checkout/releases (executor MUST verify the SHA for the chosen v4.x tag at execution time)
    - https://github.com/actions/setup-go/releases (same)
    - https://github.com/goreleaser/goreleaser-action/releases (same)
    - https://github.com/jedisct1/minisign/releases/tag/0.12 (executor MUST fetch the SHA-256 of the linux x86_64 tarball at execution time -- it lives in `signing.txt` published alongside the tarball)
  </read_first>
  <action>
This task makes two related sets of changes to release.yml: (a) replace mutable major-tag action pins with full commit SHAs, and (b) add SHA-256 verification + arch guard to the minisign install step.

**Step A: Pin actions to commit SHAs (WR-01)**

For each `uses:` line in the workflow, the executor:
1. Picks the latest stable patch release on the desired major version (e.g., v4.x latest, v5.x latest, v7.x latest).
2. Resolves the tag to its commit SHA via either the GitHub web UI ("Tags" page on the action's repo) or the gh CLI: `gh api repos/<owner>/<repo>/git/refs/tags/<tag>`.
3. Replaces the `@<tag>` form with `@<full-40-char-sha>  # <tag>` (the tag in a trailing inline comment, for human readability when reviewing diffs).

Example transformations (the executor MUST verify the SHAs at execution time -- the values below were current as of 2026-04 but may have rotated):

  uses: actions/checkout@v4
  -> uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11  # v4.2.2

  uses: actions/setup-go@v5
  -> uses: actions/setup-go@41dfa10bad2bb2ae585af6ee5bb4d7d973ad74ed  # v5.2.0

  uses: goreleaser/goreleaser-action@v7
  -> uses: goreleaser/goreleaser-action@9c156ee8a17a598857849441385a2041ef570552  # v7.0.0

The goreleaser-action appears THREE times (snapshot pass 1, snapshot pass 2, real release). All three MUST pin to the same SHA. Failing to update one of the three leaves a mutable-tag pin behind and the supply-chain hardening claim is invalidated.

**Step B: Verify minisign tarball SHA-256 + add arch guard (CR-04 + IN-02)**

The current "Install minisign (pinned)" step body is (verbatim from release.yml lines 28-39):

      - name: Install minisign (pinned)
        run: |
          set -euo pipefail
          # Pinned for reproducible CI behavior. Bump only after verifying the
          # tarball URL pattern + signature format are unchanged.
          # See .planning/phases/51-packaging-goreleaser/51-RESEARCH.md Pitfall 3.
          MINISIGN_VERSION="0.12"
          curl -fsSL -o /tmp/minisign.tar.gz \
            "https://github.com/jedisct1/minisign/releases/download/${MINISIGN_VERSION}/minisign-${MINISIGN_VERSION}-linux.tar.gz"
          tar -xzf /tmp/minisign.tar.gz -C /tmp
          sudo install -m 0755 /tmp/minisign-linux/x86_64/minisign /usr/local/bin/minisign
          minisign -v

Replace the body of this step (between `run: |` and the next step) with this exact content (preserving the `- name:` line and the `run: |` line above it):

          set -euo pipefail
          # Pinned for reproducible CI behavior. Bump only after verifying the
          # tarball URL pattern + signature format are unchanged.
          # See .planning/phases/51-packaging-goreleaser/51-RESEARCH.md Pitfall 3
          # and 51-REVIEW.md CR-04 (sha256 verification rationale).
          MINISIGN_VERSION="0.12"
          # SHA-256 of minisign-${MINISIGN_VERSION}-linux.tar.gz, sourced from
          # https://github.com/jedisct1/minisign/releases/download/${MINISIGN_VERSION}/signing.txt
          # Last verified by maintainer: <DATE>. Bump alongside MINISIGN_VERSION.
          MINISIGN_TARBALL_SHA256="<PASTE FROM signing.txt -- 64 hex chars>"

          # IN-02: fail loud if ubuntu-latest ever flips to a non-x86_64 arch.
          ARCH="$(uname -m)"
          if [ "$ARCH" != "x86_64" ]; then
            echo "::error::release.yml minisign install assumes x86_64 runner; got $ARCH. Update the URL + checksum and re-run."
            exit 1
          fi

          curl -fsSL -o /tmp/minisign.tar.gz \
            "https://github.com/jedisct1/minisign/releases/download/${MINISIGN_VERSION}/minisign-${MINISIGN_VERSION}-linux.tar.gz"

          # CR-04: verify SHA-256 BEFORE extracting. If the tarball hosting is
          # compromised, this fails the workflow before any attacker-controlled
          # binary handles MINISIGN_PRIVATE_KEY.
          echo "${MINISIGN_TARBALL_SHA256}  /tmp/minisign.tar.gz" | sha256sum -c -

          tar -xzf /tmp/minisign.tar.gz -C /tmp
          sudo install -m 0755 /tmp/minisign-linux/x86_64/minisign /usr/local/bin/minisign
          minisign -v

The executor:
1. Downloads `https://github.com/jedisct1/minisign/releases/download/0.12/signing.txt` (or the equivalent published checksums file for the chosen MINISIGN_VERSION).
2. Greps the SHA-256 line for `minisign-0.12-linux.tar.gz` from that file.
3. Pastes the 64-character hex digest into `MINISIGN_TARBALL_SHA256="..."`. Replace the literal `<PASTE FROM signing.txt -- 64 hex chars>` placeholder with the real value.
4. Replaces `<DATE>` with today's date (YYYY-MM-DD).

If `signing.txt` is not published for the chosen version (some upstream releases omit it), the executor:
1. Downloads the tarball locally,
2. Computes `sha256sum minisign-0.12-linux.tar.gz`,
3. Manually verifies the upstream minisign-signed checksum (jedisct1 self-signs releases with a documented public key from the README),
4. Pastes the verified SHA-256 into MINISIGN_TARBALL_SHA256 and records the verification method in a code comment.

If neither of those paths is feasible, document the failure in the SUMMARY -- DO NOT commit a placeholder SHA-256 (that would be worse than the current state because the workflow would silently pass anything that matches the placeholder).

Style non-negotiables:
1. The `MINISIGN_TARBALL_SHA256` variable name uses the full prefix to avoid shadowing other shell vars in the step.
2. The `sha256sum -c -` form (with the `-` for stdin) MUST be exact -- this is the canonical idiom for "verify a single SHA-256 line that you piped via echo".
3. The arch guard MUST come BEFORE the curl, not after. A non-x86_64 runner would fetch the wrong-arch tarball, fail the SHA-256 check anyway, but the arch error message is more actionable than a checksum mismatch.
4. Replace the literal `<PASTE FROM signing.txt -- 64 hex chars>` with the real value before committing. A grep for that placeholder string in the final file MUST return 0.
5. The action SHA pins MUST be 40-character full SHAs (NOT 7-char short SHAs). The trailing comment with the version tag is mandatory for review readability.
6. After the edits, `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` MUST exit 0.
  </action>
  <verify>
    <automated>! grep -q "uses: actions/checkout@v4$" .github/workflows/release.yml && ! grep -q "uses: actions/setup-go@v5$" .github/workflows/release.yml && ! grep -q "uses: goreleaser/goreleaser-action@v7$" .github/workflows/release.yml && grep -cE "uses: actions/checkout@[0-9a-f]{40}" .github/workflows/release.yml | grep -qE '^[[:space:]]*1$' && grep -cE "uses: actions/setup-go@[0-9a-f]{40}" .github/workflows/release.yml | grep -qE '^[[:space:]]*1$' && grep -cE "uses: goreleaser/goreleaser-action@[0-9a-f]{40}" .github/workflows/release.yml | grep -qE '^[[:space:]]*3$' && grep -q "MINISIGN_TARBALL_SHA256=" .github/workflows/release.yml && ! grep -q '<PASTE FROM signing.txt' .github/workflows/release.yml && grep -qE 'MINISIGN_TARBALL_SHA256="[0-9a-f]{64}"' .github/workflows/release.yml && grep -q 'sha256sum -c -' .github/workflows/release.yml && grep -q 'ARCH="$(uname -m)"' .github/workflows/release.yml && grep -q 'if \[ "$ARCH" != "x86_64" \]' .github/workflows/release.yml && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"</automated>
  </verify>
  <acceptance_criteria>
    - .github/workflows/release.yml does NOT contain `uses: actions/checkout@v4` (the mutable major-tag pin is gone)
    - .github/workflows/release.yml does NOT contain `uses: actions/setup-go@v5`
    - .github/workflows/release.yml does NOT contain `uses: goreleaser/goreleaser-action@v7`
    - .github/workflows/release.yml contains exactly ONE `uses: actions/checkout@<40-hex-sha>` line
    - .github/workflows/release.yml contains exactly ONE `uses: actions/setup-go@<40-hex-sha>` line
    - .github/workflows/release.yml contains exactly THREE `uses: goreleaser/goreleaser-action@<40-hex-sha>` lines (snapshot pass 1, snapshot pass 2, real release -- all three pinned to the SAME SHA)
    - Each pinned `uses:` line has a trailing comment with the human-readable version tag (e.g., `# v4.2.2`)
    - .github/workflows/release.yml contains a `MINISIGN_TARBALL_SHA256="<64-hex-chars>"` assignment with a real 64-character hex value (NOT the placeholder string `<PASTE FROM signing.txt ...>`)
    - .github/workflows/release.yml contains the line `echo "${MINISIGN_TARBALL_SHA256}  /tmp/minisign.tar.gz" | sha256sum -c -` (verification is BEFORE extraction)
    - .github/workflows/release.yml contains the arch guard `ARCH="$(uname -m)"` and `if [ "$ARCH" != "x86_64" ]` block with a `::error::` annotation and `exit 1`
    - The arch guard appears BEFORE the curl line in the install-minisign step
    - YAML still parses
    - The 3 new steps from Task 1 are still present
  </acceptance_criteria>
  <done>Third-party actions are SHA-pinned; the minisign tarball is SHA-256 verified before extraction; the workflow fails loud on a non-x86_64 runner. WR-01, CR-04, and IN-02 closed.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Soften CONTRIBUTING.md reproducibility paragraph (CR-03 / IN-04 doc-side fix)</name>
  <files>CONTRIBUTING.md</files>
  <read_first>
    - CONTRIBUTING.md lines 159-180 (the Releasing section's lead paragraph; line 161 is the overstatement)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md CR-03 + IN-04 (the doc/gate disagreement)
    - .planning/phases/51-packaging-goreleaser/51-VERIFICATION.md gap text on SC-4
  </read_first>
  <action>
Modify a single sentence in CONTRIBUTING.md (line 161 inside the "## Releasing" section) so it accurately describes what the reproducibility gate validates: snapshot-pair determinism, NOT real-release determinism. The CR-03 / IN-04 finding documents the doc/gate disagreement; this task picks the doc-softening closure path (rather than extending the gate to compare real-release artifacts -- that alternative would double CI time on every tag push).

Current CONTRIBUTING.md line 161 (verbatim, the second sentence of the Releasing intro paragraph):

  Releases ship as multi-arch signed binaries via a goreleaser pipeline (see `.goreleaser.yaml` and `.github/workflows/release.yml`). A `v*` git tag triggers the release workflow automatically -- there is no manual draft step. The CI-enforced reproducibility gate refuses to publish if two consecutive snapshot builds produce non-byte-identical archives, so a non-deterministic build cannot reach users.

The PROBLEM is the third sentence's claim "a non-deterministic build cannot reach users". The gate compares two snapshot builds (which use a synthesized version like `0.0.0-next-...` in -ldflags) and never compares the real-release build (which uses the actual tag). A non-determinism source that only manifests in real-release mode (e.g., changelog generation, tag-only build constants, a future `release.extra_files` that touches a non-deterministic input) would silently bypass the gate.

Replace that paragraph (the three sentences as a unit) with:

  Releases ship as multi-arch signed binaries via a goreleaser pipeline (see `.goreleaser.yaml` and `.github/workflows/release.yml`). A `v*` git tag triggers the release workflow automatically -- there is no manual draft step. The CI-enforced reproducibility gate runs two consecutive snapshot builds with identical inputs and refuses to publish if their archive sha256s differ, catching most build-environment non-determinism (toolchain drift, mod_timestamp, trimpath, GOFLAGS) before publication. The gate does not, however, compare against the real-release artifacts that ship to users -- a non-determinism source that lives only behind the real-release code path (e.g. tag-only build constants, changelog generation) would not be caught. If you suspect a real-release-only non-determinism, do a local build of the same tag with `goreleaser release --snapshot --clean --skip=sign` after your tag and diff against the published `dist/` from CI.

Style non-negotiables:
1. The replacement preserves the existing first two sentences ("Releases ship as ..." and "A `v*` git tag triggers ...") byte-identical. Only the third sentence (the overstatement) is rewritten.
2. The new prose is two sentences instead of one to capture both what the gate DOES validate (snapshot-pair determinism, build-env drift) AND what it does NOT (real-release-only non-determinism). Honesty over brevity.
3. The guidance "If you suspect a real-release-only non-determinism, do a local build ..." gives maintainers an actionable workaround for the residual gap.
4. The phrase "two consecutive snapshot builds with identical inputs" is from REVIEW IN-04's recommended fix verbatim.
5. The mention of `goreleaser release --snapshot --clean --skip=sign` references the same command the local Makefile target invokes (Plan 51-02's release-snapshot target). A maintainer who follows the suggestion uses the same code path the CI snapshot passes use.
6. Do NOT modify the rest of the Releasing section (the H3 subsections, the "Key details:" trailer, etc.). The change is limited to the lead paragraph.
7. After the edit, the `## Releasing` heading is still on its own line; the paragraph immediately below it is the rewritten version; the next H3 subsection (`### Repository secrets`) follows after a blank line, byte-identical.
8. `grep -c "non-deterministic build cannot reach users" CONTRIBUTING.md` MUST return 0 (the offending phrase is gone).
9. `grep -c "two consecutive snapshot builds with identical inputs" CONTRIBUTING.md` MUST return 1 (the new accurate phrasing is in place).
  </action>
  <verify>
    <automated>! grep -q "non-deterministic build cannot reach users" CONTRIBUTING.md && grep -q "two consecutive snapshot builds with identical inputs" CONTRIBUTING.md && grep -q "real-release artifacts that ship to users" CONTRIBUTING.md && grep -q "goreleaser release --snapshot --clean --skip=sign" CONTRIBUTING.md && grep -q "^## Releasing$" CONTRIBUTING.md && grep -q "^### Repository secrets$" CONTRIBUTING.md && grep -q "^### One-time keypair setup$" CONTRIBUTING.md && grep -q "^### Key rotation$" CONTRIBUTING.md</automated>
  </verify>
  <acceptance_criteria>
    - CONTRIBUTING.md does NOT contain `non-deterministic build cannot reach users` (the offending overstatement is gone)
    - CONTRIBUTING.md contains `two consecutive snapshot builds with identical inputs` (the accurate replacement)
    - CONTRIBUTING.md contains `real-release artifacts that ship to users` (acknowledging the residual gap)
    - CONTRIBUTING.md contains `goreleaser release --snapshot --clean --skip=sign` (the actionable maintainer workaround)
    - The `## Releasing` heading and all three H3 subsections (`### Repository secrets`, `### One-time keypair setup`, `### Key rotation`) are still present
    - The "Key details:" trailer at the end of the Releasing section is byte-identical
    - No incidental edits to other sections of CONTRIBUTING.md (`git diff CONTRIBUTING.md` shows changes only in the Releasing-section lead paragraph)
  </acceptance_criteria>
  <done>CONTRIBUTING.md's Releasing section's reproducibility claim accurately reflects what the CI gate enforces. CR-03 and IN-04 closed via the doc-softening path (the alternative gate-extension closure path is documented in the SUMMARY as an option for a future plan if the residual gap proves to matter).</done>
</task>

<task type="auto" tdd="false">
  <name>Task 4: Add cross-reference comment in minisign.pub pointing at the workflow gate</name>
  <files>minisign.pub</files>
  <read_first>
    - minisign.pub (full read; 2 lines committed in Plan 51-01)
    - .github/workflows/release.yml (just-edited; confirm the pre-flight step's grep target is the literal string PLACEHOLDER)
  </read_first>
  <action>
Update the `untrusted comment:` line of `minisign.pub` so a future maintainer dropping in the real key understands that the word `PLACEHOLDER` in the comment is load-bearing for the CI gate. The base64 key body (line 2) is NOT modified -- the maintainer overwrites that offline during the one-time keypair setup; this plan does not generate or commit a real key.

Current minisign.pub (verbatim):
  untrusted comment: serena minisign public key -- PLACEHOLDER, replace before first release
  RWQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

Replace line 1 with (preserving line 2 byte-identical):
  untrusted comment: serena minisign public key -- PLACEHOLDER (release.yml pre-flight greps this marker; remove "PLACEHOLDER" when overwriting with the real key)

The line 2 base64 placeholder body stays exactly as committed in Plan 51-01.

Style non-negotiables:
1. The `untrusted comment:` prefix is preserved literally -- minisign's pubkey-file parser requires it.
2. The literal substring `PLACEHOLDER` MUST appear in line 1. release.yml Task 1 grep-fails on this exact string. If a future maintainer naively rewords the comment to remove the word PLACEHOLDER while leaving the placeholder key body in place, the pre-flight check would silently pass and the broken key would publish -- the comment + the gate are coupled.
3. The new comment text MUST cross-reference release.yml so a future maintainer reading minisign.pub understands why the marker matters. The wording "release.yml pre-flight greps this marker" is the trip-wire prose.
4. The wording "remove 'PLACEHOLDER' when overwriting with the real key" tells the maintainer exactly what to do during keypair generation.
5. Line 2 (the base64 key body) MUST be byte-identical to before -- this plan does NOT generate or commit a real key. The keypair generation is a `user_setup` task that only the maintainer can do, on a trusted local machine, with key material that never lives in the planning agent's context.
6. The file MUST still be exactly 2 lines (`wc -l minisign.pub` returns 2).
7. After the edit, the workflow's pre-flight step's `grep -q 'PLACEHOLDER' minisign.pub` MUST still return 0 (i.e., still match) -- and that is the desired behavior: until the maintainer overwrites this file, every release attempt fails closed.
  </action>
  <verify>
    <automated>head -1 minisign.pub | grep -q "untrusted comment:" && head -1 minisign.pub | grep -q "PLACEHOLDER" && head -1 minisign.pub | grep -q "release.yml pre-flight greps this marker" && head -1 minisign.pub | grep -q 'remove "PLACEHOLDER" when overwriting with the real key' && sed -n '2p' minisign.pub | grep -qE "^RWQ[A-Za-z0-9+/=]+$" && test "$(wc -l < minisign.pub | tr -d ' ')" = "2"</automated>
  </verify>
  <acceptance_criteria>
    - minisign.pub line 1 starts with `untrusted comment:` (literal prefix preserved)
    - minisign.pub line 1 still contains the literal substring `PLACEHOLDER` (the workflow grep target)
    - minisign.pub line 1 contains `release.yml pre-flight greps this marker` (the cross-reference)
    - minisign.pub line 1 contains the substring `remove "PLACEHOLDER" when overwriting with the real key`
    - minisign.pub line 2 is byte-identical to the original placeholder body (`RWQA...AAAA`); NOT a real key
    - minisign.pub has exactly 2 lines (`wc -l` returns 2)
    - `grep -q 'PLACEHOLDER' minisign.pub` STILL returns 0 (matches) -- this is REQUIRED so the release.yml pre-flight from Task 1 still fails closed until the maintainer overwrites the file
    - The file is committed (no .gitignore changes; minisign.pub is tracked)
  </acceptance_criteria>
  <done>minisign.pub's comment line cross-references the CI gate so a future maintainer overwriting the file knows that the word PLACEHOLDER is load-bearing. The placeholder key body is unchanged -- the real keypair is still a maintainer offline action.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Maintainer's tag push -> CI publish event | Before this plan: pushing a `v*` tag with `minisign.pub` still the placeholder publishes a release whose .minisig sidecars cannot be verified by any user. After Task 1: the workflow refuses to publish in that state. |
| GitHub Actions runner -> third-party action repos | Before this plan: actions/checkout@v4, actions/setup-go@v5, goreleaser-action@v7 are mutable major-tag pins; an upstream maintainer (or attacker who compromised one) can repoint the tag to a malicious commit silently. After Task 2: full commit SHAs lock the dependency. Updates require a deliberate diff. |
| GitHub Actions runner -> jedisct1/minisign release infrastructure | Before this plan: minisign tarball downloaded over TLS with no checksum verification. A compromise of the jedisct1 account or GitHub release storage swaps the binary that signs every release. After Task 2: SHA-256 verification fails the workflow before the compromised binary handles MINISIGN_PRIVATE_KEY. |
| Job step -> /tmp/minisign.key on disk | Before this plan: the secret key file persists for the duration of the runner job; any subsequent step (e.g., a future "post-release announce" step that pulls a third-party action) can read it. After Task 1's wipe step: the file is shredded as the last step regardless of release outcome. |
| User reading CONTRIBUTING.md -> mental model of reproducibility | Before this plan: users believe "a non-deterministic build cannot reach users". After Task 3: users understand that snapshot-pair determinism is enforced; real-release-only non-determinism is not, and they have an actionable workaround. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-51-29 (T-placeholder-publish) | Spoofing / Tampering | minisign.pub all-zeros placeholder reaches a real release | mitigate | Task 1: release.yml pre-flight greps for the literal "PLACEHOLDER" marker in minisign.pub and exits 1 with a `::error file=minisign.pub::` annotation if found. Pre-flight runs BEFORE Set up Go so wasted CI minutes are minimized. Task 4: minisign.pub's comment line tells a future maintainer that the word PLACEHOLDER is load-bearing, so a naive comment rewrite cannot bypass the gate. Reference: REVIEW CR-02. ASVS V14.1. |
| T-51-30 (T-mutable-action-pin) | Tampering | Third-party actions pinned to mutable major tags | mitigate | Task 2 replaces @v4, @v5, @v7 pins with full 40-char commit SHAs. The trailing comment retains the human-readable version. A repointed major tag has no effect on this workflow because Go Actions resolves `@<sha>` literally. Reference: REVIEW WR-01. ASVS V14.1 (supply-chain). |
| T-51-31 (T-minisign-tarball-substitution) | Tampering | Upstream minisign binary download | mitigate | Task 2 adds SHA-256 verification of the tarball BEFORE extraction. The pinned SHA-256 is sourced from upstream's signing.txt (or computed locally and verified against jedisct1's signed releases). A compromised tarball fails the sha256sum step before the binary handles the project's private key. The original phase 51-01 plan documented this as ACCEPTED (T-51-07); this plan upgrades the disposition to MITIGATE. Reference: REVIEW CR-04. ASVS V14.4. |
| T-51-32 (T-secret-on-disk-residual) | Information Disclosure | /tmp/minisign.key persists for the runner job | mitigate | Task 1's `if: always()` post-step shreds (or rm -f) /tmp/minisign.key. Runs whether real release succeeds, fails, or is cancelled. Reduces in-job blast radius if a future step pulls a third-party action that scans /tmp. Reference: REVIEW WR-06. ASVS V2.10. |
| T-51-33 (T-runner-arch-flip) | Confusion / DoS | minisign URL hardcoded for x86_64 | mitigate | Task 2 adds a `uname -m` guard that fails loud with an actionable error message if the runner is not x86_64. ubuntu-latest is x86_64 today, but GitHub has signaled future arm64 transitions. The guard turns a cryptic curl-404-then-tar-error into an explicit "release.yml minisign install assumes x86_64 runner" message. Reference: REVIEW IN-02. |
| T-51-34 (T-doc-overstates-reproducibility) | Confusion / Misrepresentation | CONTRIBUTING.md:161 claims gate prevents all non-determinism | mitigate | Task 3 softens the prose to "two consecutive snapshot builds with identical inputs" and explicitly acknowledges the residual gap (real-release-only non-determinism). Provides an actionable maintainer workaround (local snapshot build of the tag + diff against published dist/). Reference: REVIEW CR-03 + IN-04. |
| T-51-35 (T-runner-workspace-bloat) | Operational | dist-pass-1 left around after diff gate | mitigate | Task 1 adds an explicit cleanup step after the diff gate succeeds. Cosmetic on ephemeral runners; meaningful if a future workspace-cache step is added. Reference: REVIEW IN-03. |

**Severity assessment:** No HIGH-severity unmitigated threats remain after this plan. T-51-29 was the most material (a placeholder release would silently break verification for every user); the pre-flight gate makes that structurally impossible. T-51-30 + T-51-31 close the supply-chain gaps that the original plan accepted as residual risk.
</threat_model>

<verification>
Plan-level verification (after all 4 tasks complete):

1. .github/workflows/release.yml has a pre-flight step that grep-fails on PLACEHOLDER in minisign.pub before Set up Go (Task 1).
2. .github/workflows/release.yml has a `Clean dist-pass-1` step after the diff gate (Task 1).
3. .github/workflows/release.yml has a `Wipe minisign secret key` step with `if: always()` as the last step (Task 1).
4. .github/workflows/release.yml has 0 mutable major-tag pins on third-party actions; all `uses:` lines have full 40-char SHAs (Task 2).
5. .github/workflows/release.yml has a `MINISIGN_TARBALL_SHA256` constant with a real 64-hex-char value and a `sha256sum -c -` verification step BEFORE extraction (Task 2).
6. .github/workflows/release.yml has a uname -m guard that fails on non-x86_64 runners (Task 2).
7. CONTRIBUTING.md no longer says "non-deterministic build cannot reach users"; the new prose accurately describes snapshot-pair determinism (Task 3).
8. minisign.pub line 1 cross-references release.yml's pre-flight (Task 4); the placeholder key body is unchanged (still placeholder until the maintainer overwrites offline).
9. YAML still parses: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` exits 0.
10. `git status` shows exactly 3 modified files: release.yml, CONTRIBUTING.md, minisign.pub. No incidental edits.
11. `go vet ./...` and `go test ./...` continue to pass (no Go source files modified).

End-to-end UAT (deferred to /gsd-verify-work, sampled at the phase gate after 51-03 + 51-05 + 51-04 + 51-06 + 51-07 land): the maintainer (a) resolves DEF-51-01 via 51-03, (b) generates a real minisign keypair locally, (c) overwrites minisign.pub with the real key (REMOVING the word "PLACEHOLDER" from the comment line per Task 4's hint), (d) uploads MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD secrets, (e) cuts a v0.0.0-rc-test tag. The release.yml then: passes the placeholder pre-flight (PLACEHOLDER no longer present); installs minisign with verified SHA-256; SHA-pinned actions resolve correctly; reproducibility gate runs against snapshot pairs; real release publishes 6 archives + 6 .minisig + checksums.txt + checksums.txt.minisig; the wipe step shreds /tmp/minisign.key. On a fresh shell, the new INSTALL.md verify block (from 51-05) chains successfully end-to-end.
</verification>

<success_criteria>
Plan 51-06 succeeds when ALL of the following are true:

1. release.yml has 3 new steps: PLACEHOLDER pre-flight (before Set up Go), Clean dist-pass-1 (after diff gate), Wipe minisign secret key (last step, if: always()).
2. release.yml has 0 mutable major-tag action pins; all 5 `uses:` lines (1 checkout + 1 setup-go + 3 goreleaser-action) have full 40-char commit SHAs with trailing version-tag comments.
3. release.yml has a real 64-hex-char MINISIGN_TARBALL_SHA256 (not the `<PASTE FROM signing.txt>` placeholder) and a sha256sum -c step BEFORE extraction.
4. release.yml has a uname -m guard that fails on non-x86_64 runners.
5. CONTRIBUTING.md's Releasing intro paragraph accurately describes snapshot-pair determinism and acknowledges the real-release-only residual gap.
6. minisign.pub line 1 cross-references the workflow gate; line 2 is byte-identical to Plan 51-01's placeholder.
7. YAML parses; `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` exits 0.
8. Every threat in the STRIDE register has a mitigation tied to a specific text change in this plan.
9. `git status` shows exactly 3 modified files; no incidental edits.
10. `go vet ./...` and `go test ./...` continue to pass.

VERIFICATION gaps closed: CR-02 (PLACEHOLDER pre-flight), CR-03/IN-04 (doc/gate truth alignment via doc softening), CR-04 (minisign tarball SHA verification), WR-01 (action SHA pinning), WR-06 (key cleanup), IN-02 (arch guard), IN-03 (dist-pass-1 cleanup). Concern B's keypair-generation half remains a maintainer offline `user_setup` action -- documented in CONTRIBUTING.md "Releasing > One-time keypair setup".
</success_criteria>

<output>
After completion, create `.planning/phases/51-packaging-goreleaser/51-06-SUMMARY.md` capturing:
- The 3 modified files with one-line description per Task.
- The full 40-char commit SHAs chosen for each pinned action (so future maintainers can audit the choice).
- The MINISIGN_TARBALL_SHA256 value committed and the source URL it was fetched from (signing.txt or local computation method).
- Date the SHAs and tarball checksum were verified ("verified <DATE>").
- Confirmation that minisign.pub line 2 (the placeholder key body) was NOT modified (still RWQ + 53 zero-bytes per Plan 51-01).
- The CR-03 closure choice: doc-softening path was taken (NOT the gate-extension path). If a future Phase deems the residual real-release-only gap worth closing, the alternative is a Pass 4 build with `--skip=publish --skip=sign` after the real release pass; record this in deferred-items.md as DEF-51-XX if needed.
- Cross-reference to VERIFICATION gaps closed: CR-02 (pre-flight), CR-03/IN-04 (doc), CR-04 (tarball SHA), WR-01 (action SHA pins), WR-06 (key cleanup), IN-02 (arch guard), IN-03 (dist-pass-1 cleanup).
- Outstanding maintainer prerequisites (carried from Plan 51-01): generate keypair locally, overwrite minisign.pub (removing the word PLACEHOLDER), upload MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD secrets. CONTRIBUTING.md "Releasing > One-time keypair setup" is the canonical reference.
</output>
