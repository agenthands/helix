---
phase: 51-packaging-goreleaser
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .goreleaser.yaml
  - .github/workflows/release.yml
  - .github/workflows/publish.yml
  - minisign.pub
autonomous: true
requirements: [PKG-01]
requirements_addressed: [PKG-01]
tags: [packaging, release-engineering, goreleaser, ci, signing]
user_setup:
  - service: github-actions-secrets
    why: "Release workflow signs archives with minisign. Without these secrets, release.yml fails on the real release pass after the reproducibility gate."
    env_vars:
      - name: MINISIGN_PRIVATE_KEY
        source: "Locally generated via `minisign -G -p minisign.pub -s minisign.key`; upload the file contents of `minisign.key` to GitHub repo Settings > Secrets and variables > Actions > New repository secret"
      - name: MINISIGN_PASSWORD
        source: "Password chosen during `minisign -G` keypair generation (per D-02a + Open Question 2: password-protected key)"
    dashboard_config:
      - task: "Generate keypair locally and upload secrets"
        location: "Maintainer's local shell, then GitHub repo Settings > Secrets and variables > Actions"
must_haves:
  truths:
    - "A `v*` git tag pushed to agenthands/helix triggers .github/workflows/release.yml"
    - "release.yml fails the job (no GitHub Release published) if two consecutive snapshot builds produce non-byte-identical .tar.gz archives"
    - ".goreleaser.yaml builds 6 archives: serena_{version}_{darwin,linux,windows}_{amd64,arm64}.tar.gz"
    - "Each archive in a published Release has a sibling .minisig file"
    - "The published Release contains exactly one checksums.txt file (NOT serena_v1.9.0_checksums.txt)"
    - "Pre-release tags (v*-rc*, v*-beta*, v*-alpha*) auto-mark the GitHub Release as Pre-release; production tags do not"
    - "go.mod module path is NOT silently renamed in this plan (per Open Question 1; staying github.com/postfix/serena until a separate decision)"
    - ".github/workflows/publish.yml (legacy Python uv->PyPI workflow) is deleted so it does not fire on `release: created` and confuse the new pipeline"
    - "minisign.pub exists at the repo root as a placeholder file the maintainer overwrites with the real public key during one-time keypair setup"
  artifacts:
    - path: ".goreleaser.yaml"
      provides: "Goreleaser v2 config -- 6-arch builds, tar.gz archives, checksums.txt override, minisign signing, conventional-commit changelog, agenthands/helix release block"
      contains: ["version: 2", "mod_timestamp: '{{ .CommitTimestamp }}'", "-trimpath", "CGO_ENABLED=0", "name_template: \"checksums.txt\"", "formats: [\"tar.gz\"]", "owner: agenthands", "name: helix", "prerelease: auto"]
    - path: ".github/workflows/release.yml"
      provides: "Tag-triggered (v*) release workflow with three goreleaser passes (snapshot, snapshot, real) and a sha256 diff gate between the snapshot passes; signs only on the real release pass"
      contains: ["push:", "tags:", "'v*'", "contents: write", "fetch-depth: 0", "go-version: '1.25.x'", "MINISIGN_PRIVATE_KEY", "MINISIGN_PASSWORD", "--snapshot", "--skip=sign", "diff -u"]
    - path: "minisign.pub"
      provides: "Repo-root placeholder for the project's minisign public key (per D-02). Maintainer overwrites during one-time keypair setup."
      min_lines: 1
    - path: ".github/workflows/publish.yml"
      provides: "DELETED -- legacy Python uv->PyPI publish workflow is removed so it does not fire on `release: created` (Pitfall 5)"
      contains: []
  key_links:
    - from: ".github/workflows/release.yml"
      to: ".goreleaser.yaml"
      via: "goreleaser/goreleaser-action@v7 reads ./.goreleaser.yaml from the repo root"
      pattern: "goreleaser/goreleaser-action@v7"
    - from: ".goreleaser.yaml signs.cmd"
      to: "minisign binary on $PATH (installed by release.yml step)"
      via: "subprocess invocation with -S -s /tmp/minisign.key -x ${signature} -m ${artifact}, password on stdin from $MINISIGN_PASSWORD"
      pattern: "cmd: minisign"
    - from: "release.yml secrets ref"
      to: "GitHub Actions secret store"
      via: "${{ secrets.MINISIGN_PRIVATE_KEY }} and ${{ secrets.MINISIGN_PASSWORD }} (NEVER hardcoded, NEVER printed)"
      pattern: "secrets\\.MINISIGN_PRIVATE_KEY"
    - from: ".goreleaser.yaml release.github"
      to: "agenthands/helix GitHub repo"
      via: "owner: agenthands / name: helix"
      pattern: "owner: agenthands"
---

<objective>
Build the goreleaser-driven release pipeline that turns a `v*` git tag into a published GitHub Release with 6 reproducible signed archives. This plan creates the two configuration files (`.goreleaser.yaml`, `.github/workflows/release.yml`), commits a placeholder public key file (`minisign.pub`), and deletes the legacy Python `publish.yml` so it cannot collide with the new release event.

Purpose: Closes ROADMAP success criteria 1, 2, and 4 for Phase 51 (tag triggers workflow producing 6 binaries; each binary has SHA-256 + minisign signature; reproducibility gate prevents publishing non-deterministic builds). Plan 02 closes criterion 3 (user-facing verification UX in INSTALL.md) and depends on the secret names introduced here.

Output: A repo state where pushing a `v*` tag (after the maintainer uploads MINISIGN_PRIVATE_KEY and MINISIGN_PASSWORD secrets) publishes a signed Release end-to-end, and where a non-reproducible build is fail-closed before publication.
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
@.planning/phases/51-packaging-goreleaser/51-PATTERNS.md
@.planning/phases/51-packaging-goreleaser/51-VALIDATION.md
@.github/workflows/go-test.yml
@.github/workflows/publish.yml
@.gitignore

<interfaces>
<!-- The release workflow consumes the goreleaser config; nothing else in the repo reads either file. -->
<!-- Goreleaser's "interface" is the v2 schema. The relevant top-level keys this plan uses: -->

`.goreleaser.yaml` v2 top-level keys this plan touches:
- `version: 2`                -- schema major
- `project_name: serena`      -- controls binary name + default templates
- `builds:`                   -- cross-compile matrix (RESEARCH Pattern 1)
- `archives:`                 -- packaging + naming (RESEARCH Pattern 2)
- `checksum:`                 -- sha256 file name override (RESEARCH Pattern 2)
- `signs:`                    -- minisign subprocess (RESEARCH Pattern 3)
- `changelog:`                -- conventional-commit grouping (RESEARCH Pattern 4)
- `release:`                  -- GitHub repo owner/name + prerelease auto-detect (RESEARCH Pattern 5)

`.github/workflows/release.yml` GitHub Actions interface this plan uses:
- trigger: `on: push: tags: ['v*']`         (D-06; NO workflow_dispatch, NO pull_request)
- permissions: `contents: write`            (need to create release + upload assets)
- action: `actions/checkout@v4` with `fetch-depth: 0`  (goreleaser changelog needs full history)
- action: `actions/setup-go@v5` with `go-version: '1.25.x'` and `cache: true`  (mirrors go-test.yml exactly)
- action: `goreleaser/goreleaser-action@v7` with `distribution: goreleaser`, `version: '~> v2'`
- secrets: `${{ secrets.MINISIGN_PRIVATE_KEY }}`, `${{ secrets.MINISIGN_PASSWORD }}`, `${{ secrets.GITHUB_TOKEN }}`
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Delete legacy publish.yml so it cannot fire on release: created</name>
  <files>.github/workflows/publish.yml</files>
  <read_first>
    - .github/workflows/publish.yml (full read -- confirm no maintained content; current trigger is `on: release: types: [created]` which would fire when goreleaser-action publishes the new release and run `uv build` against a Go-only working tree)
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pitfall 5: Conflict with stale Python publish.yml"
    - .planning/phases/51-packaging-goreleaser/51-PATTERNS.md section "`.github/workflows/publish.yml` (DELETE)" (preserved verbatim there for the executor's reference)
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "canonical_refs" (lists publish.yml as "LIKELY DELETE" planner-discretion)
  </read_first>
  <action>
Delete the file `.github/workflows/publish.yml` outright. No replacement, no archival rename.

Rationale (executor must not relitigate): the workflow's trigger is `on: release: types: [created]`. When goreleaser-action publishes the new GitHub Release in Task 3's workflow, GitHub fires `release: created`, which would re-trigger publish.yml and run `astral-sh/setup-uv@v6 + uv build` against a Go-only working tree -- confusing failure noise on every release. Zero cross-references exist (research confirmed: no mention in `.github/`, `docs/`, `CONTRIBUTING.md`, `README.md`, or `legacy/` invokes this workflow). The legacy/ Python tree is read-only reference per CLAUDE.md and does not depend on this CI workflow.

Use `git rm` (not plain `rm`) so the deletion is staged for commit:

```sh
git rm .github/workflows/publish.yml
```

Do NOT touch any of the other workflow files (`codeql.yml`, `codespell.yml`, `docker.yml`, `docs.yaml`, `go-test.yml`, `junie.yml`, `pytest.yml`) -- those stay untouched per CONTEXT canonical_refs.
  </action>
  <verify>
    <automated>test ! -f .github/workflows/publish.yml &amp;&amp; ls .github/workflows/go-test.yml .github/workflows/codeql.yml &gt;/dev/null 2&gt;&amp;1</automated>
  </verify>
  <acceptance_criteria>
    - `.github/workflows/publish.yml` does NOT exist on disk
    - `git status --short .github/workflows/publish.yml` shows the file as staged-deleted (`D` in column 1) or absent
    - All other files under `.github/workflows/` still exist (`go-test.yml`, `codeql.yml`, `codespell.yml`, `docker.yml`, `docs.yaml`, `junie.yml`, `pytest.yml`) -- verified by `ls .github/workflows/`
    - No new files were created in `.github/workflows/`
  </acceptance_criteria>
  <done>publish.yml is gone from the working tree and from the git index. Listing `.github/workflows/` shows the original 7 files minus publish.yml. The new release.yml will be added in Task 3.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Write .goreleaser.yaml -- 6-arch reproducible build with minisign signing and conventional-commit changelog</name>
  <files>.goreleaser.yaml</files>
  <behavior>
    - `goreleaser check .goreleaser.yaml` exits 0 (config lints clean)
    - `goreleaser release --snapshot --clean --skip=sign` produces exactly 6 `.tar.gz` archives in `dist/` named `serena_{version}_{os}_{arch}.tar.gz` for the cartesian product of {darwin, linux, windows} x {amd64, arm64}
    - Two consecutive `goreleaser release --snapshot --clean --skip=sign` invocations on the same checkout produce byte-identical archives (sha256 of each .tar.gz matches across runs)
    - `dist/checksums.txt` exists (literal name, NOT `serena_v1.9.0_checksums.txt`)
    - LICENSE, README.md, INSTALL.md, CHANGELOG.md are bundled INSIDE each archive (`files:` listing)
    - The release section addresses GitHub repo `agenthands/helix` (NOT `postfix/serena`)
  </behavior>
  <read_first>
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 1: Reproducible builds: block (D-03a)" -- full builds: stanza with mandatory ldflags
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 2: archives + checksum block (D-04 INSTALL match)" -- formats override and checksums.txt name override
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 3: signs block (minisign -- D-01)" -- exact args and stdin password wiring
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 4: changelog (D-07 -- conventional commits)"
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 5: release block + GitHub Action workflow (D-06)" -- release: subsection only (the workflow goes in Task 3)
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Anti-Patterns to Avoid" -- six pitfalls including "Don't omit ldflags", "Don't trust default checksum.name_template", "Don't trust default archive formats"
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "decisions" -- D-01..D-07 locked
    - cmd/serena/main.go (single-line confirmation that `./cmd/serena` is the binary entrypoint; do not modify)
    - .gitignore (confirm line 77 has `dist/` -- already verified, no edit needed)
  </read_first>
  <action>
Create `.goreleaser.yaml` at the repo root with the following exact content. Every block is from RESEARCH Patterns 1-5 (verified against goreleaser docs 2026-04-28). Do NOT improvise the schema; do NOT omit any reproducibility flag; do NOT change `checksums.txt` or `formats: ["tar.gz"]` to defaults.

```yaml
# Source of truth: goreleaser v2 schema. Generated for Phase 51 (PKG-01).
# Locked decisions: D-01 (minisign), D-03/D-03a (reproducibility), D-04 (INSTALL match),
# D-06 (tag-triggered + auto pre-release), D-07 (auto-generated notes from git log).
version: 2

project_name: serena

builds:
  - id: serena
    main: ./cmd/serena
    binary: serena
    env:
      - CGO_ENABLED=0
      - GOFLAGS=-mod=readonly
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.CommitDate}}
    mod_timestamp: '{{ .CommitTimestamp }}'

archives:
  - id: serena
    builds: [serena]
    formats: ["tar.gz"]
    name_template: "serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE*
      - README.md
      - INSTALL.md
      - CHANGELOG.md

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

signs:
  - id: minisign
    cmd: minisign
    artifacts: archive
    signature: "${artifact}.minisig"
    args:
      - "-S"
      - "-s"
      - "/tmp/minisign.key"
      - "-x"
      - "${signature}"
      - "-m"
      - "${artifact}"
    stdin: '{{ .Env.MINISIGN_PASSWORD }}'

changelog:
  use: git
  sort: asc
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Refactor
      regexp: '^.*?refactor(\([[:word:]]+\))??!?:.+$'
      order: 2
    - title: Documentation
      regexp: '^.*?docs(\([[:word:]]+\))??!?:.+$'
      order: 3
    - title: Others
      order: 999
  filters:
    exclude:
      - '^test:'
      - '^chore:'
      - 'Merge pull request'
      - 'Merge branch'

release:
  github:
    owner: agenthands
    name: helix
  prerelease: auto
  draft: false
  mode: replace
  name_template: "Serena {{ .Tag }}"
```

Key non-negotiables (executor must NOT change these):
1. `version: 2` literal at the top -- required for the v2 schema field names (`formats:` list, not `format:` string).
2. `mod_timestamp: '{{ .CommitTimestamp }}'` -- the `.CommitTimestamp` variant (Unix epoch int) NOT `.CommitDate` (RFC3339 string). Per RESEARCH Pitfall 6: the wrong template type produces "invalid mod_timestamp" errors.
3. `ldflags:` block is MANDATORY. Goreleaser's default injects `main.date={{.Date}}` (wall-clock) which destroys reproducibility. The override sets `main.date={{.CommitDate}}`.
4. `checksum.name_template: "checksums.txt"` -- MUST be the override (the default is `{{ .ProjectName }}_{{ .Version }}_checksums.txt` which breaks the D-04 INSTALL block that downloads `checksums.txt`).
5. `formats: ["tar.gz"]` -- uniform across all OSes (D-04 INSTALL UX is a single fenced block; modern Windows ships `tar` in System32 since 10 1803).
6. `signs.artifacts: archive` -- sign archives only, NOT checksums.txt (per RESEARCH Anti-Pattern: "Don't sign checksums.txt").
7. `signs.stdin: '{{ .Env.MINISIGN_PASSWORD }}'` -- uses the password-protected keypair per Open Question 2 / D-02a. Do NOT remove the stdin line, do NOT switch to a `-W`-unencrypted key.
8. `release.github.owner: agenthands` and `name: helix` -- repo identity per Pitfall 2 / Open Question 1. The `go.mod` module path stays `github.com/postfix/serena` for now (separate decision); goreleaser does not care about module path, only GitHub owner/name.
9. `prerelease: auto` -- auto-detects `v*-rc*`, `v*-beta*`, `v*-alpha*` per D-06.

After writing the file, run `goreleaser check` if available locally (developer environment may or may not have goreleaser installed). If goreleaser is available, the lint must pass. If it is not available locally, document that fact in the SUMMARY and rely on the CI workflow (Task 3) to surface any lint errors on the first tag push.
  </action>
  <verify>
    <automated>test -f .goreleaser.yaml &amp;&amp; grep -q "^version: 2$" .goreleaser.yaml &amp;&amp; grep -q "mod_timestamp: '{{ .CommitTimestamp }}'" .goreleaser.yaml &amp;&amp; grep -q 'name_template: "checksums.txt"' .goreleaser.yaml &amp;&amp; grep -q 'formats: \["tar.gz"\]' .goreleaser.yaml &amp;&amp; grep -q "owner: agenthands" .goreleaser.yaml &amp;&amp; grep -q "name: helix" .goreleaser.yaml &amp;&amp; grep -q "prerelease: auto" .goreleaser.yaml &amp;&amp; grep -q "cmd: minisign" .goreleaser.yaml &amp;&amp; grep -q "artifacts: archive" .goreleaser.yaml &amp;&amp; grep -q "MINISIGN_PASSWORD" .goreleaser.yaml &amp;&amp; grep -q -- "-trimpath" .goreleaser.yaml &amp;&amp; grep -q "CGO_ENABLED=0" .goreleaser.yaml &amp;&amp; grep -q "X main.date={{.CommitDate}}" .goreleaser.yaml</automated>
  </verify>
  <acceptance_criteria>
    - `.goreleaser.yaml` exists at the repo root (NOT under `.github/`, NOT `.goreleaser.yml`)
    - `grep -c '^  - id: serena$' .goreleaser.yaml` returns 2 (one in `builds:`, one in `archives:`)
    - `grep -E '^      - (darwin|linux|windows)$' .goreleaser.yaml | wc -l` returns 3 (all three OSes listed under `goos:`)
    - `grep -E '^      - (amd64|arm64)$' .goreleaser.yaml | wc -l` returns 2 (both architectures listed under `goarch:`)
    - File contains `mod_timestamp: '{{ .CommitTimestamp }}'` literally (string match, NOT `.Date` or `.CommitDate`)
    - File contains `name_template: "checksums.txt"` (literal -- NOT a templated default)
    - File contains `formats: ["tar.gz"]` (uniform tar.gz, no `format_overrides:` block)
    - File contains `cmd: minisign` and `artifacts: archive` and `stdin: '{{ .Env.MINISIGN_PASSWORD }}'`
    - File contains `owner: agenthands` and `name: helix` (NOT `postfix` or `serena`)
    - File contains `prerelease: auto` and `draft: false`
    - File does NOT contain the strings `cosign`, `sigstore`, `format_overrides:`, `workflow_dispatch:`
    - File does NOT contain the substring `postfix/serena`
    - If `goreleaser` is available on the executor's machine: `goreleaser check` exits 0 with no errors. If not available: SUMMARY notes "lint deferred to CI on first tag push".
  </acceptance_criteria>
  <done>`.goreleaser.yaml` exists, lints clean (or is queued for CI lint), and contains all 9 non-negotiable elements listed above.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Write .github/workflows/release.yml -- tag-triggered three-pass workflow with reproducibility diff gate</name>
  <files>.github/workflows/release.yml</files>
  <behavior>
    - On `push: tags: ['v*']`, the workflow runs on ubuntu-latest with Go 1.25.x (matching go-test.yml exactly)
    - The job installs minisign at a pinned version (NOT apt; pinned tarball from jedisct1/minisign GitHub Releases for reproducible CI behavior -- RESEARCH Pitfall 3)
    - Snapshot pass 1 runs `goreleaser release --snapshot --clean --skip=sign`, then `mv dist dist-pass-1`, then captures sha256s of all `*.tar.gz` to `/tmp/pass1.sha256`
    - Snapshot pass 2 runs `goreleaser release --snapshot --clean --skip=sign`, captures sha256s, normalizes paths, runs `diff -u /tmp/pass1.sha256 /tmp/pass2.sha256`
    - If the diff fails (non-zero exit), the job fails with a `::error::` annotation BEFORE the real release pass -- non-reproducible build cannot publish (D-03)
    - Only after the diff gate passes does goreleaser-action run with `args: release --clean` (real publish, signs every archive, uploads to GitHub Releases)
    - Real release pass receives `MINISIGN_PRIVATE_KEY` (written to /tmp/minisign.key earlier with umask 077), `MINISIGN_PASSWORD` (env var into goreleaser process), and `GITHUB_TOKEN` (auto-injected)
    - Workflow does NOT add `needs:` on go-test.yml or any cross-workflow gate (per Open Question 3 + D-06)
    - Workflow does NOT add `workflow_dispatch:` or pull_request triggers (per D-06)
  </behavior>
  <read_first>
    - .github/workflows/go-test.yml (full read -- analog for `runs-on`, `timeout-minutes`, `actions/checkout@v4`, `actions/setup-go@v5` blocks; release.yml mirrors these exactly)
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 5: release block + GitHub Action workflow (D-06)" -- the full ~80-line YAML quoted verbatim there is the source for this task
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pitfall 1: Snapshot version template breaks the diff" -- explains the `mv dist dist-pass-1` + path normalization trick
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pitfall 3: minisign install method on ubuntu-latest" -- pinned tarball (option c) over apt (option a) because apt drift is a reproducibility hazard
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pitfall 5: Conflict with stale Python publish.yml" -- Task 1 already deleted publish.yml; this task confirms the new workflow does not collide with anything
    - .planning/phases/51-packaging-goreleaser/51-PATTERNS.md section "`.github/workflows/release.yml` (NEW -- CI workflow)" -- pattern assignments for header / checkout / setup-go (copy verbatim from go-test.yml with documented changes)
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "decisions" -- D-03 (CI-enforced reproducibility), D-06 (tag-triggered, no draft/dispatch)
  </read_first>
  <action>
Create `.github/workflows/release.yml` with the following exact content. The body is taken verbatim from RESEARCH Pattern 5 (verified against goreleaser-action@v7 + actions/setup-go@v5 + actions/checkout@v4 docs 2026-04-28).

```yaml
name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # required: create release, upload assets

jobs:
  release:
    name: goreleaser
    runs-on: ubuntu-latest
    timeout-minutes: 30
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0   # required: goreleaser changelog needs full git history

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true

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

      - name: Write minisign secret key to disk (umask 077)
        env:
          MINISIGN_PRIVATE_KEY: ${{ secrets.MINISIGN_PRIVATE_KEY }}
        run: |
          set -euo pipefail
          umask 077
          # Write secret key contents to /tmp/minisign.key for goreleaser signs: subprocess.
          # umask 077 ensures only this runner user can read the file.
          # The variable contents NEVER appear in logs because we use printf '%s'
          # (no echo, no expansion in the command line).
          printf '%s' "$MINISIGN_PRIVATE_KEY" > /tmp/minisign.key

      - name: Reproducibility gate -- snapshot pass 1
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --snapshot --clean --skip=sign

      - name: Capture pass 1 sha256s
        run: |
          set -euo pipefail
          mv dist dist-pass-1
          (cd dist-pass-1 && find . -name '*.tar.gz' -print0 | sort -z | xargs -0 sha256sum) > /tmp/pass1.sha256
          echo "Pass 1 archives:"
          cat /tmp/pass1.sha256

      - name: Reproducibility gate -- snapshot pass 2
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --snapshot --clean --skip=sign

      - name: Diff sha256s -- fail if non-reproducible
        run: |
          set -euo pipefail
          (cd dist && find . -name '*.tar.gz' -print0 | sort -z | xargs -0 sha256sum) \
            | sed 's| dist/| dist-pass-1/|' > /tmp/pass2.sha256
          if ! diff -u /tmp/pass1.sha256 /tmp/pass2.sha256; then
            echo "::error::archives are not byte-identical between two snapshot runs -- refusing to publish"
            exit 1
          fi
          echo "Reproducibility gate PASS -- proceeding to real release"

      - name: Real release (sign + publish)
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          MINISIGN_PASSWORD: ${{ secrets.MINISIGN_PASSWORD }}
```

Key non-negotiables (executor must NOT change these):
1. `on: push: tags: ['v*']` and ONLY this trigger. Do NOT add `workflow_dispatch:`, do NOT add `pull_request:`, do NOT add `release:` (per D-06; tag-triggered only). Do NOT add `needs:` on go-test.yml (per Open Question 3).
2. `permissions: contents: write` -- bumped from go-test.yml's `read` because publishing a Release requires write. Do NOT add `id-token: write` (we are using minisign, NOT cosign keyless).
3. `fetch-depth: 0` on checkout -- REQUIRED for goreleaser's changelog generator to see git history.
4. `go-version: '1.25.x'` and `cache: true` -- must match go-test.yml exactly. Drift between these two workflows is a smell.
5. minisign install via PINNED tarball (option c per Pitfall 3), NOT `apt-get install minisign`. The tarball URL pattern and pinned version `0.12` are from RESEARCH section "Standard Stack" / Assumption A3 -- if `gh release list -R jedisct1/minisign` shows a newer version, the executor MAY bump to a newer pinned version after verifying the tarball URL and Linux x86_64 binary path are unchanged, but MUST NOT switch to apt.
6. `--skip=sign` on BOTH snapshot passes -- minisign signatures contain a per-call nonce; signing during reproducibility-check would trip the diff. The real release pass (step "Real release") signs.
7. `mv dist dist-pass-1` between passes -- `--clean` wipes `dist/` before each goreleaser run, so the first run's output must be saved aside. The `sed 's| dist/| dist-pass-1/|'` step normalizes paths so the diff compares like-for-like.
8. `MINISIGN_PRIVATE_KEY` is written to disk with `printf '%s'` (NOT `echo`) inside `umask 077`. The secret is referenced as `${{ secrets.MINISIGN_PRIVATE_KEY }}` only -- never hardcoded, never printed to logs (GitHub Actions automatically masks secrets in log output, but `printf '%s' "$VAR" > file` keeps the value off the command line entirely).
9. `MINISIGN_PASSWORD` is passed as a step-level `env:` ONLY to the real release step -- not to the snapshot passes (which use `--skip=sign`).
10. `GITHUB_TOKEN` is on the real release pass only -- snapshot passes do not upload anywhere.

Permissions / safety rationale (for the threat model):
- `contents: write` is the minimum needed. We do NOT request `packages: write`, `id-token: write`, or anything else.
- Trigger is `push: tags: ['v*']` ONLY. Pull-request-triggered workflows have NO access to repository secrets, so even a malicious PR cannot trigger a signed-release run.

After writing the file, the workflow itself cannot be sampled in this task without pushing a tag (and that requires the maintainer to have uploaded the secrets first). Verification at this task scope is YAML lint + grep matrix (see acceptance criteria below). The end-to-end CI run is sampled at the phase gate (per VALIDATION.md "Sampling Rate" section "Before /gsd-verify-work").
  </action>
  <verify>
    <automated>test -f .github/workflows/release.yml &amp;&amp; grep -q "^name: release$" .github/workflows/release.yml &amp;&amp; grep -q "tags:" .github/workflows/release.yml &amp;&amp; grep -q "      - 'v\*'" .github/workflows/release.yml &amp;&amp; grep -q "contents: write" .github/workflows/release.yml &amp;&amp; grep -q "fetch-depth: 0" .github/workflows/release.yml &amp;&amp; grep -q "go-version: '1.25.x'" .github/workflows/release.yml &amp;&amp; grep -q "goreleaser/goreleaser-action@v7" .github/workflows/release.yml &amp;&amp; grep -q -- "--skip=sign" .github/workflows/release.yml &amp;&amp; grep -q "secrets.MINISIGN_PRIVATE_KEY" .github/workflows/release.yml &amp;&amp; grep -q "secrets.MINISIGN_PASSWORD" .github/workflows/release.yml &amp;&amp; grep -q "diff -u /tmp/pass1.sha256 /tmp/pass2.sha256" .github/workflows/release.yml &amp;&amp; ! grep -q "workflow_dispatch:" .github/workflows/release.yml &amp;&amp; ! grep -q "pull_request:" .github/workflows/release.yml &amp;&amp; ! grep -q "^    needs:" .github/workflows/release.yml &amp;&amp; python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"</automated>
  </verify>
  <acceptance_criteria>
    - `.github/workflows/release.yml` exists
    - File contains `name: release` on its own line
    - Trigger block contains `on:` then `push:` then `tags:` then `'v*'` (case-sensitive)
    - File contains `permissions:` with `contents: write` (NOT `read`)
    - File does NOT contain `workflow_dispatch:`, `pull_request:`, `release:` (event), or `id-token:`
    - File does NOT contain a `needs:` clause anywhere
    - Checkout step contains `fetch-depth: 0`
    - setup-go step contains `go-version: '1.25.x'` and `cache: true` (matching go-test.yml exactly -- drift would be a smell)
    - File contains `goreleaser/goreleaser-action@v7` exactly 3 times (3 invocations: snapshot pass 1, snapshot pass 2, real release)
    - File contains `--skip=sign` exactly 2 times (both snapshot passes; the real release does NOT skip signing)
    - File contains `${{ secrets.MINISIGN_PRIVATE_KEY }}` (referenced from secret store, never inline)
    - File contains `${{ secrets.MINISIGN_PASSWORD }}` (referenced from secret store, never inline)
    - File contains `${{ secrets.GITHUB_TOKEN }}` (auto-injected by Actions runtime)
    - File contains `printf '%s' "$MINISIGN_PRIVATE_KEY"` and `umask 077` (secret-write hardening)
    - File contains `mv dist dist-pass-1` (so the first snapshot's archives survive `--clean` of the second)
    - File contains the literal string `diff -u /tmp/pass1.sha256 /tmp/pass2.sha256` (the reproducibility gate)
    - File contains `::error::archives are not byte-identical` (annotation on diff failure)
    - YAML syntax is valid: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` exits 0
  </acceptance_criteria>
  <done>`.github/workflows/release.yml` is a valid GitHub Actions workflow that on `v*` tag push runs the three-pass reproducibility gate and, only on success, signs and publishes 6 archives + 6 .minisig + checksums.txt to the agenthands/helix Releases.</done>
</task>

<task type="auto">
  <name>Task 4: Commit minisign.pub placeholder at the repo root</name>
  <files>minisign.pub</files>
  <read_first>
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "decisions" D-02 ("public key file is at the repo root: `minisign.pub`")
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Maintainer prerequisite" -- `minisign -G -p minisign.pub -s minisign.key` produces the real file; the placeholder gets overwritten at one-time keypair setup
    - .planning/phases/51-packaging-goreleaser/51-PATTERNS.md section "No Analog Found" -> `minisign.pub` row -- file format is two-line ASCII (untrusted comment + base64 key)
  </read_first>
  <action>
Create `minisign.pub` at the repo root with placeholder content. The maintainer will overwrite this file with the real public key during one-time keypair generation (documented in CONTRIBUTING.md "Releasing" subsection -- added in Plan 02).

The placeholder MUST be a syntactically-shaped minisign public key file (untrusted comment line + base64 key line) so the file is committable today and so anyone reading the repo sees the file exists. The base64 key value is intentionally a placeholder string that will not verify any real signature -- that is the point: until the maintainer overwrites it, `minisign -V` MUST fail loudly rather than silently accept a forgery.

Create `minisign.pub` at the repo root with this exact content (note the literal text -- no trailing whitespace beyond the final newline):

```
untrusted comment: serena minisign public key -- PLACEHOLDER, replace before first release
RWQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
```

Notes for the executor:
1. The `untrusted comment:` prefix MUST be present and literal -- minisign's pubkey-file parser requires it.
2. The second line is base64-shaped (starts with `RWQ` -- minisign Ed25519 algo identifier prefix per upstream README; padded with `A`s to a realistic length). It is NOT a real key. The intent is "this file slot is reserved; the maintainer fills it in".
3. Do NOT commit a real keypair generated for testing. The placeholder is intentional.
4. CONTRIBUTING.md "Releasing" subsection in Plan 02 documents the full one-time setup: `minisign -G -p minisign.pub -s minisign.key`, then upload the contents of `minisign.key` (and the chosen password) as GitHub Actions secrets.
5. The file lives at the repo root (NOT under `.github/` or `docs/`). Per D-02: "Visible at first directory listing -- signals 'this project signs releases' immediately."

Do NOT modify `.gitignore` -- `minisign.pub` is a tracked file. (The corresponding *secret* key, `minisign.key`, must NEVER be committed; the maintainer keeps that locally and uploads its contents as a GitHub secret.)
  </action>
  <verify>
    <automated>test -f minisign.pub &amp;&amp; head -1 minisign.pub | grep -q "^untrusted comment:" &amp;&amp; sed -n 2p minisign.pub | grep -qE "^RWQ[A-Za-z0-9+/=]+$" &amp;&amp; test "$(wc -l &lt; minisign.pub | tr -d ' ')" = "2"</automated>
  </verify>
  <acceptance_criteria>
    - `minisign.pub` exists at the repo root (NOT inside `.github/`, `docs/`, or any subdirectory)
    - File has exactly 2 lines (`wc -l` returns 2)
    - Line 1 starts with `untrusted comment:` (literal prefix; minisign parser requires it)
    - Line 1 contains the word `PLACEHOLDER` (so a maintainer or contributor reading the file knows it must be replaced before the first real release)
    - Line 2 starts with `RWQ` (Ed25519 algo identifier in minisign v2 pubkey format)
    - Line 2 matches the regex `^RWQ[A-Za-z0-9+/=]+$` (base64 alphabet only, no whitespace)
    - `git status --short minisign.pub` shows the file as `A ` (added, staged) or `??` (untracked, ready for staging)
    - `.gitignore` was NOT modified (the file should be tracked, not ignored)
  </acceptance_criteria>
  <done>`minisign.pub` exists at the repo root as a placeholder shaped like a real minisign public key file, ready for the maintainer to overwrite during one-time keypair generation. CONTRIBUTING.md (Plan 02) documents the overwrite procedure.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| GitHub Actions runner -> repo secret store | The release.yml job reads `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` from the per-repo secret store. These secrets must never appear in CI logs, must never be committed to git, and must only be readable on tag-triggered workflow runs (not pull_request runs). |
| GitHub Actions runner -> upstream binary servers | The release.yml job downloads the minisign binary from `github.com/jedisct1/minisign/releases`. A pinned version + URL pattern is the only mitigation against a tarball-substitution attack on that server. |
| Goreleaser subprocess -> minisign secret key file | Goreleaser invokes `minisign -S -s /tmp/minisign.key -m <archive>`. The key file is written with umask 077 so only the runner user can read it. The file lives only for the duration of the job (ephemeral runner disk). |
| Maintainer's local machine -> repo | The minisign keypair is generated locally and the public half is committed to the repo (`minisign.pub`). The private half NEVER touches the repo -- it is uploaded directly to GitHub Settings as a secret. |
| Published GitHub Release -> end user | Users download `.tar.gz` + `.minisig` + `checksums.txt` and verify locally with `minisign -V` and `sha256sum -c`. The trust anchor is `minisign.pub` fetched from the main branch. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-51-01 (T-secret-leak) | Information Disclosure | release.yml `MINISIGN_PRIVATE_KEY` handling | mitigate | Reference secret as `${{ secrets.MINISIGN_PRIVATE_KEY }}` only (never hardcoded). Write to disk with `printf '%s' "$VAR" > file` (NOT `echo`) inside `umask 077`. Step does NOT echo, cat, or print the variable. GitHub Actions automatically masks secret values in log output as a defense-in-depth layer. ASVS V2.10 / V14.1. |
| T-51-02 (T-supply-chain) | Tampering | Published Release archives | mitigate | Every archive has a sibling `.minisig` (goreleaser `signs.artifacts: archive`). `checksums.txt` lists every archive sha256. The verify recipe in INSTALL.md (Plan 02) chains: download checksums.txt -> sha256sum -c -> minisign -V against an archive whose sha256 is in checksums.txt. The workflow fail-closes if signing fails (goreleaser non-zero exit -> job failure -> no Release published). ASVS V14.4. |
| T-51-03 (T-non-repro-publish) | Tampering | Build pipeline reproducibility | mitigate | Two `--snapshot --clean --skip=sign` passes BEFORE the real release pass. `diff -u /tmp/pass1.sha256 /tmp/pass2.sha256` runs as a workflow step that exits non-zero (with `::error::` annotation) if any archive sha256 differs. Real release step is sequenced AFTER the diff step in the same job, so a non-reproducible build cannot publish. ASVS V14.4. |
| T-51-04 (T-pubkey-tamper) | Spoofing | `minisign.pub` distribution | mitigate | Public key file lives at repo root, version-controlled. Rotation = a normal commit visible in git log + diff. INSTALL.md (Plan 02) tells users to fetch the key from `raw.githubusercontent.com/agenthands/helix/main/minisign.pub` so they always get the current key from the main branch. Branch protection on main is the upstream gate against unauthorized key rotation -- not enforced by this plan but assumed by the design. ASVS V6.4. |
| T-51-05 (T-pull-request-secret-access) | Elevation of Privilege | Workflow trigger surface | mitigate | release.yml triggers ONLY on `push: tags: ['v*']`. Pull-request-triggered workflows have NO access to repository secrets per GitHub's security model, so even a malicious PR that modifies release.yml cannot exfiltrate the minisign key -- the new YAML would only execute on a tag push, which requires push access to the repo. The job does NOT use `pull_request_target` or other elevated triggers. ASVS V14.1. |
| T-51-06 (T-legacy-workflow-collision) | Tampering | Stale `publish.yml` triggering on Release events | mitigate | Task 1 deletes `.github/workflows/publish.yml`. Its trigger was `on: release: types: [created]`, which would fire when goreleaser publishes the new Release and run `uv build` against a Go-only working tree. Deletion is the mitigation; no other workflow in `.github/workflows/` listens for `release: created`. ASVS V14.1. |
| T-51-07 (T-minisign-tarball-substitution) | Tampering | Upstream minisign binary download | accept | release.yml downloads minisign 0.12 from `github.com/jedisct1/minisign/releases/download/0.12/minisign-0.12-linux.tar.gz`. The pinned URL + version is the mitigation; we accept the residual risk that GitHub itself or the jedisct1 account is compromised. The minisign binary is not used to verify the binary it produces -- it is the signing tool. A compromised minisign would produce signatures that verify against a compromised public key, but the public key in this repo is generated locally by the maintainer. Defense-in-depth: future phase could pin to a git submodule pointing at jedisct1/minisign at a known-good commit, but that is overkill for a single CI tool. ASVS V14.4. |
| T-51-08 (T-overprivileged-token) | Elevation of Privilege | Workflow `permissions:` | mitigate | release.yml requests ONLY `permissions: contents: write` (the minimum needed to create a GitHub Release and upload assets). Does NOT request `packages: write`, `id-token: write`, `actions: write`, or anything else. Least-privilege per ASVS V14.1. |

**Severity assessment:** No HIGH-severity unmitigated threats remain. T-51-07 is ACCEPTED with a documented residual risk (upstream supplier compromise) that is the same risk every Go project's release pipeline carries.
</threat_model>

<verification>
Plan-level verification (after all 4 tasks complete):

1. **File presence:** `test -f .goreleaser.yaml && test -f .github/workflows/release.yml && test -f minisign.pub && test ! -f .github/workflows/publish.yml` exits 0.
2. **Goreleaser config lint:** If `goreleaser` is available locally, `goreleaser check .goreleaser.yaml` exits 0. If not available, the lint is sampled by the first CI tag-push (Phase 02 will run the throwaway tag).
3. **Local snapshot smoke (Plan 02 will exercise this via `make release-snapshot` once the Make target exists):** `goreleaser release --snapshot --clean --skip=sign` produces 6 .tar.gz archives + checksums.txt under `dist/`.
4. **Workflow YAML lint:** `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` exits 0.
5. **Repo identity sanity:** `! grep -r "postfix/serena" .goreleaser.yaml .github/workflows/release.yml` (the new files MUST use agenthands/helix throughout).
6. **No secret leakage in committed files:** `! grep -E "(BEGIN.*PRIVATE|RWR[A-Za-z0-9+/=]{60,})" .goreleaser.yaml .github/workflows/release.yml minisign.pub` (no real secret-key material committed; the placeholder pattern starting with `RWQ...AAA...AAA` is shaped like a pubkey, which is OK).
7. **Threat model coverage:** Every threat in the STRIDE register has a mitigation tied to a specific file change in this plan (Tasks 1-4). No threat disposition is "accept" without rationale (T-51-07 is the only accept).

The end-to-end CI run (push a `v0.0.0-rc-test` tag, observe release.yml fires the three-pass gate and signs+publishes 6 archives + 6 .minisig + checksums.txt) is sampled at the phase gate in Plan 02's verification step (per VALIDATION.md). It cannot be sampled inside Plan 01 because the maintainer must first upload the minisign secrets, which is a one-time human action documented in CONTRIBUTING.md (Plan 02).
</verification>

<success_criteria>
Plan 01 succeeds when ALL of the following are true:

1. `.goreleaser.yaml` exists at repo root, contains all 9 non-negotiable elements from Task 2 acceptance criteria, and lints clean (locally or deferred to CI).
2. `.github/workflows/release.yml` exists and contains the full three-pass workflow body verbatim from Task 3 (tag-trigger, contents:write permission, fetch-depth:0 checkout, Go 1.25.x setup, pinned minisign install, umask 077 secret write, two `--skip=sign` snapshot passes, sha256 diff gate with `::error::` annotation, real release pass with secrets).
3. `minisign.pub` exists at repo root as a 2-line placeholder (untrusted-comment line + base64-shaped key line starting with `RWQ`), ready for one-time maintainer overwrite.
4. `.github/workflows/publish.yml` is DELETED (no longer in working tree, staged for commit removal).
5. Every threat in the STRIDE register has a mitigation tied to a specific file change in this plan, or is accepted with documented residual-risk rationale (T-51-07 only).
6. `git status` shows exactly the 4 file changes expected (3 new files + 1 deletion); no other files modified.
7. `go vet ./...` and `go test ./...` continue to pass (this plan touches NO Go source files; tests should remain green).

The release pipeline is now structurally complete. Plan 02 layers the user-facing documentation and developer ergonomics on top.
</success_criteria>

<output>
After completion, create `.planning/phases/51-packaging-goreleaser/51-01-SUMMARY.md` capturing:
- The four files created/deleted with one-line description each.
- Whether `goreleaser check` was run locally and its result (or "deferred to CI").
- Confirmation that no Go sources were modified (`go vet ./...` and `go test ./...` baseline status).
- Reminder for Plan 02: CONTRIBUTING.md "Releasing" subsection MUST reference the secret names `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` exactly as wired in release.yml; INSTALL.md verification block MUST use `agenthands/helix` URLs and the same `checksums.txt` filename as the goreleaser config.
- Maintainer prerequisite still pending: generate keypair locally, overwrite `minisign.pub`, upload secrets via `gh secret set MINISIGN_PRIVATE_KEY < minisign.key && gh secret set MINISIGN_PASSWORD` (or the GitHub web UI). This is a one-time human task tracked as `user_setup` in the frontmatter.
</output>
