---
phase: 51-packaging-goreleaser
plan: 05
type: execute
wave: 3
depends_on: [51-01, 51-02]
files_modified:
  - .goreleaser.yaml
  - INSTALL.md
  - Makefile
autonomous: true
gap_closure: true
requirements: [PKG-01]
requirements_addressed: [PKG-01]
tags: [packaging, signing, checksums, gap-closure, wr-02, wr-05, in-01]
must_haves:
  truths:
    - "CLOSES VERIFICATION concern B (signs.txt strengthening) -- checksums.txt is also signed by minisign so an attacker cannot swap both archive AND checksums.txt to bypass verification (WR-02)"
    - "CLOSES VERIFICATION/REVIEW finding WR-05 -- the INSTALL.md sha256sum step fails LOUD on a typo'd archive name instead of silently exiting 0 via --ignore-missing"
    - "CLOSES VERIFICATION/REVIEW finding IN-01 -- make release-snapshot prints a clear error message when goreleaser is not installed locally, instead of the cryptic `make: goreleaser: command not found`"
    - "An attacker who swaps an archive must also produce a valid minisign signature on a substituted checksums.txt -- impossible without the project's private key"
    - "A user who fat-fingers OS or ARCH in the INSTALL.md verify block sees a checksum FAILED message (non-zero exit) instead of a misleading OK (--ignore-missing)"
    - "A contributor without goreleaser installed sees a one-line install hint instead of a confusing make error"
  artifacts:
    - path: ".goreleaser.yaml"
      provides: "signs.artifacts changed from `archive` to `all` so checksums.txt also gets a .minisig sidecar"
      contains: ["artifacts: all"]
    - path: "INSTALL.md"
      provides: "Verify ceremony tightened: --ignore-missing dropped, replaced by an explicit grep-for-the-archive-line that exits non-zero if the archive's sha256 was not present in checksums.txt; minisign step extended to verify checksums.txt itself before sha256sum runs"
      contains: ["minisign -V -p minisign.pub -m checksums.txt", "sha256sum -c checksums.txt", "checksum FAILED"]
    - path: "Makefile"
      provides: "release-snapshot target gains a command -v goreleaser guard with a one-line install hint"
      contains: ["command -v goreleaser", "brew install goreleaser"]
  key_links:
    - from: "INSTALL.md verify block"
      to: ".goreleaser.yaml signs.artifacts: all"
      via: "checksums.txt.minisig published as a sidecar; INSTALL.md tells users to verify it BEFORE running sha256sum -c"
      pattern: "checksums\\.txt\\.minisig|minisign -V -p minisign\\.pub -m checksums\\.txt"
    - from: "Makefile release-snapshot target"
      to: "goreleaser binary on $PATH"
      via: "command -v guard with a fail-fast message pointing at brew install / release-tarball install"
      pattern: "command -v goreleaser"
---

<objective>
Close the half of VERIFICATION Concern B that does not require maintainer-local action (the keypair generation + secret upload remains a `user_setup` item the maintainer performs offline). Specifically:

1. Make `checksums.txt` itself signed by minisign by changing `.goreleaser.yaml` `signs.artifacts: archive` to `all` (closes WR-02). Today, an attacker who swaps both an archive AND `checksums.txt` defeats the sha256sum step; only the per-archive minisign signature catches it. After this fix, `checksums.txt.minisig` is published, and INSTALL.md verifies that signature before running sha256sum -- so the chain is "verify checksums.txt is signed by us -> trust the sha256s in it -> verify the archive matches one of those sha256s". Independent integrity at every step.

2. Tighten the INSTALL.md verify block so a user who typos `OS` or `ARCH` cannot see a misleading "OK" (closes WR-05). The fix replaces `sha256sum -c --ignore-missing checksums.txt` with an explicit grep that asserts the user's archive name appears in checksums.txt with status OK; if it does not, the step exits non-zero with a "checksum FAILED" message.

3. Add a `command -v goreleaser` guard to the `make release-snapshot` recipe (closes IN-01). A contributor running the target without goreleaser installed currently sees `make: goreleaser: No such file or directory` and has to dig through CONTRIBUTING.md to learn the install hint. The guard prints the hint inline and exits 1 cleanly.

The release.yml CI pre-flight that grep-fails on the PLACEHOLDER minisign.pub (the OTHER half of Concern B) is owned by Plan 51-06 because release.yml has a single owner this wave (also touched by 51-07). 51-05 owns only `.goreleaser.yaml`, `INSTALL.md`, and `Makefile` -- no overlap with 51-06 or 51-07 in this wave.

Output: A repo state where checksums.txt has its own minisign signature, the INSTALL.md verify block fails LOUD on user typos, and contributors without goreleaser get a clear install hint instead of a cryptic make error.
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
@.planning/phases/51-packaging-goreleaser/51-VERIFICATION.md
@.planning/phases/51-packaging-goreleaser/51-REVIEW.md
@.goreleaser.yaml
@INSTALL.md
@Makefile

<interfaces>
.goreleaser.yaml signs block (committed in Plan 51-01, lines 46-59):
  signs:
    - id: minisign
      cmd: minisign
      artifacts: archive            <-- this line changes to: artifacts: all
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

Goreleaser docs (verify before changing): `signs.artifacts` enum values are
"none", "checksum", "archive", "all", "binary", "package", "manifest",
"sbom", "source". The value "all" signs every produced artifact except sboms
(which have their own block). Ref: https://goreleaser.com/customization/sign/

Makefile release-snapshot target (committed in Plan 51-02, lines 50-51):
  release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
  	goreleaser release --snapshot --clean --skip=sign

INSTALL.md verify block (committed in Plan 51-02, lines 14-31). The two lines
this plan changes:
  Line 23: sha256sum -c --ignore-missing checksums.txt
  Line 26: minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz

After this plan:
  Line 23-26 will be a strict checksum check (grep "OK" or fail) PLUS a
  minisign verification of checksums.txt itself BEFORE the sha256sum step.
  The per-archive minisign verify step is preserved.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Change .goreleaser.yaml signs.artifacts from archive to all</name>
  <files>.goreleaser.yaml</files>
  <read_first>
    - .goreleaser.yaml full file (94 lines; the signs block is lines 46-59; line 49 is the artifacts: archive line)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md WR-02 (the finding text and the recommended fix)
  </read_first>
  <action>
Change a single line in .goreleaser.yaml: `artifacts: archive` -> `artifacts: all`. This makes goreleaser produce a `.minisig` sidecar for checksums.txt in addition to the per-archive sidecars, closing WR-02.

Current line 49 of .goreleaser.yaml (verbatim):
  `    artifacts: archive`

Replace with:
  `    artifacts: all`

That is the only edit. Indentation is 4 spaces (not tab) -- matching the surrounding YAML structure.

Style non-negotiables:
1. The change is a single character-level swap on a single line. Do NOT touch the `cmd: minisign`, `signature: "${artifact}.minisig"`, `args:`, or `stdin:` lines.
2. After the edit, the resulting goreleaser config still has exactly one `signs:` block with one `id: minisign` entry. Do NOT add a second signs entry.
3. Goreleaser's `artifacts: all` value covers archives + checksums + (if configured) packages + sboms + manifests. For this project, "all" effectively means "archives + checksums.txt" because no other artifact types are configured.
4. The `.minisig` filename for checksums.txt will be `checksums.txt.minisig` -- automatic via the `signature: "${artifact}.minisig"` template that's already in place.
5. After the edit, if `goreleaser` is available locally, `goreleaser check` MUST exit 0 (the schema accepts `all` as a valid value).
6. Do NOT add a comment or rationale inline in the YAML -- the value `all` is self-explanatory and the project's other YAML configs (.github/workflows/) do not pepper inline rationale comments.

Why "all" over the more granular alternatives:
- `signs.artifacts: archive` (current): only archives signed; checksums.txt unsigned. Defeats integrity if both archive and checksums.txt are swapped.
- `signs.artifacts: checksum`: ONLY checksums.txt signed; archives unsigned. Worse than current (an attacker can swap a single archive without changing checksums.txt -- the per-archive minisig is the only catch).
- `signs.artifacts: all`: archives AND checksums.txt both signed. Strictly the safer choice. Reference: REVIEW WR-02 fix recommendation.

What this enables in INSTALL.md (Task 2):
- The verify block can chain "minisign -V on checksums.txt" -> "sha256sum -c on the archive" -> "minisign -V on the archive". Each step provides independent integrity.
- An attacker has to forge minisign signatures on BOTH the archive AND checksums.txt to defeat the new chain -- impossible without the project's private key.
  </action>
  <verify>
    <automated>grep -q "    artifacts: all$" .goreleaser.yaml && ! grep -q "    artifacts: archive$" .goreleaser.yaml && grep -c "^signs:$" .goreleaser.yaml | grep -qE '^[[:space:]]*1$' && grep -c "id: minisign" .goreleaser.yaml | grep -qE '^[[:space:]]*1$' && grep -q 'signature: "${artifact}.minisig"' .goreleaser.yaml</automated>
  </verify>
  <acceptance_criteria>
    - .goreleaser.yaml contains the literal line `    artifacts: all` (4 spaces of indentation, value "all")
    - .goreleaser.yaml does NOT contain the literal line `    artifacts: archive`
    - .goreleaser.yaml has exactly ONE `^signs:$` block (no duplicates introduced by the edit)
    - .goreleaser.yaml has exactly ONE `id: minisign` line (still a single signing config)
    - .goreleaser.yaml retains `signature: "${artifact}.minisig"` (the sidecar template is unchanged)
    - .goreleaser.yaml retains `cmd: minisign`, the `args:` list, and `stdin: '{{ .Env.MINISIGN_PASSWORD }}'` byte-identical
    - If goreleaser is locally installed: `goreleaser check .goreleaser.yaml` exits 0
    - YAML is still valid: `python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml'))"` exits 0
    - No incidental edits to other blocks (`git diff .goreleaser.yaml` shows ONE line changed)
  </acceptance_criteria>
  <done>.goreleaser.yaml signs.artifacts: all -- checksums.txt will be signed alongside the archives on the next release. WR-02 closed at the build-config level. Plan 51-06 enforces the publish-time check that the maintainer's pubkey is real, not the placeholder.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Strengthen INSTALL.md verify block (drop --ignore-missing, verify checksums.txt signature, fail loud on typos)</name>
  <files>INSTALL.md</files>
  <read_first>
    - INSTALL.md lines 1-55 (the "Install (pre-built binary)" lead section committed in 51-02; the verify block is at lines 12-31)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md WR-02 (recommends signing checksums.txt) and WR-05 (recommends dropping --ignore-missing)
    - .planning/phases/51-packaging-goreleaser/51-VERIFICATION.md gap text on SC-3 ("checksum step is documentation theater unless the user runs minisign too")
  </read_first>
  <action>
Replace the existing verify block in INSTALL.md (currently lines 14-31) with a stronger version that:
1. Downloads `checksums.txt.minisig` in addition to the existing four files.
2. Verifies the minisign signature on checksums.txt FIRST (so the sha256s in it are trusted).
3. Replaces `sha256sum -c --ignore-missing checksums.txt` with a strict grep that asserts the user's archive line is present and OK.
4. Preserves the per-archive minisign verification as an independent integrity check.

Current INSTALL.md lines 14-31 (verbatim, the bash fenced block):

  ```bash
  VERSION=v1.9.0
  OS=linux
  ARCH=amd64

  # Download archive, signature, checksums, and (one-time) the project public key
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz.minisig
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt
  curl -LO https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub  # one-time

  # Verify checksum
  sha256sum -c --ignore-missing checksums.txt

  # Verify signature
  minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz

  # Extract and run
  tar -xzf serena_${VERSION}_${OS}_${ARCH}.tar.gz
  ./serena --help
  ```

Replace with this exact content (the fenced block keeps the same surrounding markdown -- the heading, the lead paragraph, and the post-block prose are unchanged):

  ```bash
  VERSION=v1.9.0
  OS=linux
  ARCH=amd64

  # Download archive, signatures, checksums, and (one-time) the project public key
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz.minisig
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt
  curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt.minisig
  curl -LO https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub  # one-time

  # Verify the checksums file is signed by the project (catches a substituted checksums.txt)
  minisign -V -p minisign.pub -m checksums.txt

  # Verify the archive's sha256 is present in the (now-trusted) checksums.txt -- fails loud on typos
  sha256sum -c checksums.txt 2>&1 | grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz: OK" \
    || { echo "checksum FAILED"; exit 1; }

  # Verify the archive's minisign signature (independent of checksums.txt)
  minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz

  # Extract and run
  tar -xzf serena_${VERSION}_${OS}_${ARCH}.tar.gz
  ./serena --help
  ```

Style non-negotiables:
1. Single fenced block. Do NOT split into multiple ```bash blocks. The whole verification ceremony is one copy-paste flow per CONTEXT D-04.
2. Variable names (VERSION, OS, ARCH) are unchanged.
3. The five `curl -LO` lines (one more than before -- now five not four) preserve the existing URL pattern. The new fifth line downloads `checksums.txt.minisig` from the same Releases base URL.
4. The `minisign -V -p minisign.pub -m checksums.txt` step runs BEFORE sha256sum. This is the trust-anchor step: the user verifies the project signed checksums.txt before they trust any of the sha256s in it.
5. The strict sha256sum step uses the exact form from REVIEW WR-05: `sha256sum -c checksums.txt 2>&1 | grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz: OK" || { echo "checksum FAILED"; exit 1; }`. The line break with `\` continuation MUST be preserved -- removing it makes the line >130 chars and ugly.
6. The per-archive `minisign -V -p minisign.pub -m serena_..._.tar.gz` step is preserved verbatim. It is independent of the checksums.txt chain (a defense-in-depth: even if the user skips the checksums step, the minisign step catches a substituted archive).
7. The `tar -xzf` and `./serena --help` post-verify steps are preserved verbatim.
8. The macOS one-line note (just below the fenced block, currently line 33) needs ONE small update: after this task, the substitution is `sha256sum -c` -> `shasum -a 256 -c` AND `minisign -V` -> `minisign -V` (unchanged, since brew installs the same `minisign` binary). Update the note to read:
   `**macOS users:** replace `sha256sum -c` with `shasum -a 256 -c`, and install minisign with `brew install minisign`. The `grep "...: OK"` line works as-is on macOS. Everything else is identical.`
9. Do NOT modify the surrounding markdown structure (the H2 headings, the lead paragraph "Pre-built binaries for darwin/linux/windows...", the supported-OS-values paragraph, or the "Build from source" section that follows).

Why minisign-on-checksums.txt comes BEFORE sha256sum:
- The trust order is: pubkey (TOFU on first fetch) -> checksums.txt.minisig (verify that the project signed THIS checksums file) -> sha256s in checksums.txt (now trusted) -> archive's sha256 matches one of those (now-trusted) lines.
- If sha256sum runs first and the user skips minisign, they have ZERO integrity guarantee (a swapped archive + swapped checksums.txt passes).
- Putting minisign first makes the ceremony fail-closed if the user accidentally skips a step -- they cannot get past minisign without a real signature.

Why the strict sha256sum form (not just dropping --ignore-missing):
- `sha256sum -c checksums.txt` without `--ignore-missing` fails if ANY listed archive is not present locally. Since checksums.txt lists 6 archives but the user only downloaded 1, plain `-c` exits non-zero with "5 missing files" warnings on stderr -- loud and confusing.
- The grep form filters for "the user's specific archive: OK" and exits non-zero only if THAT line is missing. Clean signal: either the user's archive verifies, or they see "checksum FAILED" and exit.
- Reference: REVIEW WR-05 verbatim: `sha256sum -c checksums.txt 2>&1 | grep "serena_${VERSION}_${OS}_${ARCH}.tar.gz: OK" || { echo "checksum FAILED"; exit 1; }`.
  </action>
  <verify>
    <automated>grep -q 'minisign -V -p minisign.pub -m checksums.txt' INSTALL.md && grep -q 'curl -LO https://github.com/agenthands/helix/releases/download/$VERSION/checksums.txt.minisig' INSTALL.md && ! grep -q 'sha256sum -c --ignore-missing checksums.txt' INSTALL.md && grep -q 'sha256sum -c checksums.txt 2>&1 | grep' INSTALL.md && grep -q 'echo "checksum FAILED"; exit 1' INSTALL.md && grep -q 'minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz' INSTALL.md && grep -c 'minisign -V -p minisign.pub' INSTALL.md | grep -qE '^[[:space:]]*2$' && grep -q 'shasum -a 256 -c' INSTALL.md && grep -q '## Install (pre-built binary)' INSTALL.md && grep -q '## Build from source' INSTALL.md</automated>
  </verify>
  <acceptance_criteria>
    - INSTALL.md contains the literal string `minisign -V -p minisign.pub -m checksums.txt` (the new checksums-file signature verification step)
    - INSTALL.md contains a curl line for `checksums.txt.minisig` (the new download)
    - INSTALL.md does NOT contain `sha256sum -c --ignore-missing checksums.txt` (the old fail-open form is gone)
    - INSTALL.md contains the strict form `sha256sum -c checksums.txt 2>&1 | grep` (the new fail-loud form is in place)
    - INSTALL.md contains the failure exit handler `echo "checksum FAILED"; exit 1`
    - INSTALL.md contains the per-archive minisign step `minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz` (preserved from before)
    - INSTALL.md has exactly TWO occurrences of `minisign -V -p minisign.pub` (one for checksums.txt, one for the archive)
    - INSTALL.md still has the macOS note containing `shasum -a 256 -c` and `brew install minisign`
    - The H2 structure is preserved: `## Install (pre-built binary)` and `## Build from source` headings still exist
    - The lines from `## Quick Start` onwards are byte-identical to the original (this plan touches only the verify block)
    - No incidental edits to the README.md (Plan 51-04 owns README.md; this plan is INSTALL.md-only here)
  </acceptance_criteria>
  <done>INSTALL.md verify ceremony has independent integrity at every step: the checksums.txt is verified to be signed by the project, the archive's sha256 is asserted to match a now-trusted line, and the archive's own minisign signature is verified separately. WR-02 and WR-05 closed at the user-facing-doc level.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Add command -v goreleaser guard to Makefile release-snapshot recipe</name>
  <files>Makefile</files>
  <read_first>
    - Makefile (full read; the release-snapshot target is at lines 50-51)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md IN-01 (the finding text and recommended fix)
  </read_first>
  <action>
Modify the `release-snapshot` recipe in Makefile to add a `command -v goreleaser` guard with a one-line install hint. Today the recipe is one line; after this task it is two lines (a guard + the existing goreleaser invocation).

Current Makefile lines 50-51 (verbatim):
  release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
  	goreleaser release --snapshot --clean --skip=sign

Replace with (preserving the TAB indentation on recipe lines -- Make is whitespace-sensitive; SPACES will fail with `*** missing separator`):

  release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
  	@command -v goreleaser >/dev/null 2>&1 || { \
  	  echo "goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser"; exit 1; }
  	goreleaser release --snapshot --clean --skip=sign

Style non-negotiables:
1. Both recipe lines start with a literal TAB (NOT spaces). The continuation backslash on the first guard line connects it to the `echo` line; the `echo` line starts with `\t  ` (TAB + two spaces of secondary indent).
2. The `@` prefix on the guard suppresses Make's command echo for the guard itself (the guard is plumbing, not behavior the user wants to see). The `goreleaser release ...` line on line 4 of the new recipe does NOT have the `@` prefix -- that line is the actual work, and Make's default echo of it is informative.
3. The error message text MUST contain both "CONTRIBUTING.md (Releasing)" (so users know where to find the full setup) AND "brew install goreleaser" (so macOS users see a one-shot install hint inline). Linux users get a pointer to the broader doc.
4. The `## help` annotation on the target line is unchanged. The Makefile self-doc style established in Phase 50 D-07 is preserved.
5. The `goreleaser release --snapshot --clean --skip=sign` recipe line is unchanged byte-for-byte.
6. After the edit, `make -n release-snapshot` (dry-run) MUST exit 0 and print BOTH the guard line and the goreleaser line (because -n prints commands without executing).
7. After the edit, on a machine WITHOUT goreleaser: `make release-snapshot` MUST exit 1 with the install-hint message and MUST NOT print "make: goreleaser: command not found" (the guard catches it before make tries to exec the missing binary).
8. After the edit, on a machine WITH goreleaser: `make release-snapshot` runs goreleaser as before. The guard is silent (returns 0).
9. Do NOT modify any other Makefile target. The `bench`, `bench-baseline`, `build`, `clean`, `proto`, `test`, `vet`, `fmt`, `docs`, `clean-jdtls-cache`, `bench-jdtls-warm` targets stay byte-identical.
10. The `.PHONY` line 1 is unchanged.
  </action>
  <verify>
    <automated>grep -q "@command -v goreleaser >/dev/null 2>&1" Makefile && grep -q "goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser" Makefile && grep -q "goreleaser release --snapshot --clean --skip=sign" Makefile && grep -c "^release-snapshot:" Makefile | grep -qE '^[[:space:]]*1$' && awk '/^release-snapshot:/{found=1; next} found && /^[a-z]/{exit} found{print NR": "$0}' Makefile | grep -q "command -v goreleaser"</automated>
  </verify>
  <acceptance_criteria>
    - Makefile contains the literal string `@command -v goreleaser >/dev/null 2>&1` (the guard prefix)
    - Makefile contains the literal string `goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser` (the install hint)
    - Makefile contains the literal string `goreleaser release --snapshot --clean --skip=sign` (the unchanged main command)
    - Makefile has exactly ONE `^release-snapshot:` target line (no duplicates introduced)
    - Makefile line 1 (`.PHONY: ...`) is byte-identical to before (`release-snapshot` was already on .PHONY from Plan 51-02)
    - The guard line is between the target line and the goreleaser invocation (verified by line-order check via awk)
    - `make -n release-snapshot` exits 0 (dry-run print works)
    - On a machine without goreleaser: `command -v goreleaser >/dev/null 2>&1; echo "ec=$?"` returns `ec=1`, and `make release-snapshot` would exit 1 with the install hint (this acceptance criterion is environment-dependent; documented in the SUMMARY)
    - All other Makefile targets are byte-identical (`git diff Makefile` shows changes ONLY in the release-snapshot recipe block)
  </acceptance_criteria>
  <done>make release-snapshot prints a clear install hint when goreleaser is missing instead of the cryptic make-level error. IN-01 closed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Published GitHub Release -> end user | The release publishes archives + .minisig sidecars + checksums.txt. Before this plan, an attacker who compromises BOTH the archive and checksums.txt defeats the sha256sum step; only the per-archive .minisig catches it. After this plan, checksums.txt also has a .minisig sidecar -- both must be forged together for an attack to succeed. |
| INSTALL.md verify block -> shell exit codes | A user who fat-fingers OS or ARCH currently sees a misleading "OK" from sha256sum --ignore-missing. After this plan, the strict grep form exits non-zero with "checksum FAILED" -- the user cannot accidentally skip past a typo. |
| Makefile release-snapshot -> contributor environment | A contributor without goreleaser installed currently sees `make: goreleaser: command not found`. After this plan, they see "goreleaser not installed; see CONTRIBUTING.md (Releasing). brew install goreleaser" and exit cleanly. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-51-25 (T-checksums-substitution) | Tampering | checksums.txt unsigned | mitigate | Task 1: signs.artifacts: all -> goreleaser produces checksums.txt.minisig sidecar. Task 2: INSTALL.md verifies the sidecar BEFORE trusting any sha256 in checksums.txt. An attacker swapping checksums.txt now needs to forge a minisign signature on the new file -- impossible without the project's private key. Reference: REVIEW WR-02. ASVS V14.4. |
| T-51-26 (T-fat-finger-checksum-bypass) | Tampering / Confusion | INSTALL.md sha256sum --ignore-missing | mitigate | Task 2: --ignore-missing dropped, replaced by `sha256sum -c checksums.txt | grep "<archive>: OK" || exit 1`. A user who typos the archive name sees "checksum FAILED" instead of an empty success. Reference: REVIEW WR-05. ASVS V14.4. |
| T-51-27 (T-doc-theater-checksum) | Confusion | INSTALL.md checksum step "documentation theater" | mitigate | Task 2 sequences the verifications so each step has independent integrity: minisign-on-checksums.txt validates the trust anchor; sha256sum-grep validates the archive bytes via the now-trusted file; minisign-on-archive validates the archive bytes via the project's signature. A user who skips ANY step still has a real integrity claim from the others. Reference: VERIFICATION.md SC-3 gap text. |
| T-51-28 (T-cryptic-make-error) | Confusion | Makefile release-snapshot without goreleaser | mitigate | Task 3 adds `command -v goreleaser` guard with an actionable error message. Reduces support burden on contributors; not a security threat per se but a quality-of-life fix that prevents misdiagnosed setup issues. Reference: REVIEW IN-01. |

**Severity assessment:** No HIGH-severity threats. T-51-25 is the most material -- the checksum-bypass-via-double-swap was a real gap in the supply-chain story; closing it makes the integrity claim defensible.
</threat_model>

<verification>
Plan-level verification (after all 3 tasks complete):

1. .goreleaser.yaml has exactly `    artifacts: all` (4-space indent, value "all"); the previous `archive` value is gone (Task 1).
2. INSTALL.md verify block has TWO `minisign -V -p minisign.pub` invocations (one for checksums.txt, one for the archive); --ignore-missing is gone; the strict grep form is present (Task 2).
3. INSTALL.md downloads `checksums.txt.minisig` in addition to the existing four files (Task 2).
4. Makefile release-snapshot recipe contains the `command -v goreleaser` guard with an actionable error message (Task 3).
5. `make -n release-snapshot` exits 0 (Task 3 dry-run sanity).
6. If goreleaser is locally installed: `goreleaser check .goreleaser.yaml` exits 0 (Task 1 schema sanity).
7. YAML parser accepts the modified .goreleaser.yaml: `python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml'))"` exits 0.
8. `git status` shows exactly 3 modified files: .goreleaser.yaml, INSTALL.md, Makefile. No incidental edits.
9. `go vet ./...` and `go test ./...` continue to pass (no Go source files modified).

End-to-end UAT (deferred to /gsd-verify-work, sampled at the phase gate after 51-03 unblocks the build): cut a throwaway v0.0.0-rc-test tag; release.yml publishes 6 archives + 6 .minisig + checksums.txt + checksums.txt.minisig; on a fresh shell, copy-paste the new INSTALL.md verify block end-to-end; observe minisign verifying checksums.txt, the strict sha256sum exiting 0 on the matched archive, and minisign verifying the archive. The four-step chain is the goal-state for SC-3.
</verification>

<success_criteria>
Plan 51-05 succeeds when ALL of the following are true:

1. `.goreleaser.yaml` `signs.artifacts: all` (was `archive`); WR-02 closed at the build-config level.
2. INSTALL.md verify block: minisign-on-checksums.txt step added BEFORE sha256sum; sha256sum step uses the strict grep-OK-or-exit-1 form (WR-05 closed); per-archive minisign step preserved.
3. INSTALL.md downloads `checksums.txt.minisig` from the same Releases URL as the other artifacts (the new file the maintainer's first signed release will produce).
4. Makefile `release-snapshot` recipe has a `command -v goreleaser` guard with an actionable error message (IN-01 closed).
5. `make -n release-snapshot` exits 0 (dry-run sanity).
6. If goreleaser is locally installed: `goreleaser check .goreleaser.yaml` exits 0 (the new `artifacts: all` value is a valid v2 schema value).
7. YAML still parses: `python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml'))"` exits 0.
8. Every threat in the STRIDE register has a mitigation tied to a specific text change in this plan.
9. `git status` shows exactly 3 modified files; no incidental edits to other files.
10. `go vet ./...` and `go test ./...` continue to pass.

VERIFICATION gaps closed: WR-02 (checksums.txt unsigned), WR-05 (--ignore-missing fail-open), IN-01 (cryptic make error). Concern B's keypair-rotation half remains a `user_setup` action the maintainer performs offline (already documented in the original 51-01 user_setup); Concern B's CI pre-flight half is owned by Plan 51-06.
</success_criteria>

<output>
After completion, create `.planning/phases/51-packaging-goreleaser/51-05-SUMMARY.md` capturing:
- The 3 files modified with one-line description per Task.
- Result of `goreleaser check .goreleaser.yaml` (if locally installed) or "deferred to CI" otherwise.
- Result of `make -n release-snapshot` (dry-run command print).
- Confirmation that the strict sha256sum form was tested mentally against a typo'd archive name (e.g., $OS=darwn instead of darwin -> grep finds no matching OK line -> exit 1) -- this is a thought exercise, not a CI-verifiable assertion.
- Cross-reference to VERIFICATION gaps closed: WR-02 (checksums.txt unsigned -> signed), WR-05 (--ignore-missing -> strict grep), IN-01 (cryptic make error -> actionable hint).
- Note for Plan 51-06: the CI pre-flight grep-fail on PLACEHOLDER minisign.pub is a related Concern B fix that lives in release.yml; this plan did not touch release.yml because 51-06 owns it this wave. Together, 51-05 + 51-06 close all of Concern B except the maintainer's offline keypair generation.
</output>
