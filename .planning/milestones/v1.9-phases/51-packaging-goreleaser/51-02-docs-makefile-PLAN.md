---
phase: 51-packaging-goreleaser
plan: 02
type: execute
wave: 2
depends_on: [51-01]
files_modified:
  - INSTALL.md
  - README.md
  - Makefile
  - CONTRIBUTING.md
autonomous: true
requirements: [PKG-01]
requirements_addressed: [PKG-01]
tags: [packaging, documentation, makefile, contributing, install-guide]
must_haves:
  truths:
    - "A user reading INSTALL.md sees a 'Install (pre-built binary)' H2 section as the first install path BEFORE 'Build from source'"
    - "INSTALL.md contains a single fenced bash block users can copy-paste end-to-end to download, verify checksum, verify minisign signature, and extract a release archive (D-04)"
    - "All `<owner>/<repo>` placeholders and literal `postfix/serena` references in INSTALL.md and README.md (lines 18, 65, 71) are replaced with `agenthands/helix` -- closes Pitfall 2 in the public-facing docs (Open Question 1: go.mod stays unchanged)"
    - "macOS users get a one-line note explaining `shasum -a 256 -c` substitution and `brew install minisign` -- D-04 single-block UX preserved"
    - "A contributor can run `make release-snapshot` to produce 6 archives in `dist/` locally without needing the project's minisign secret key (--skip=sign locally)"
    - "CONTRIBUTING.md contains a new H2 'Releasing' subsection placed AFTER 'Running Benchmarks' and BEFORE 'Adding a New MCP Tool', covering: cutting a release (push v* tag), the two GitHub Actions secret names from Plan 01 (MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD), local dry-run command, key rotation procedure, one-time keypair setup"
    - "CONTRIBUTING.md uses the same house style as 'Running Benchmarks': ```sh fences (NOT ```bash), one-paragraph intro, 'Key details:' trailing bullet list, references make-targets rather than open-coded shell"
  artifacts:
    - path: "INSTALL.md"
      provides: "Restructured install guide with pre-built binary path leading; D-04 verification ceremony as a single copy-paste block; agenthands/helix URLs throughout; legacy `go install postfix/serena` line either removed or flagged with TODO until go.mod rename decision"
      contains: ["## Install (pre-built binary)", "minisign -V", "sha256sum -c", "agenthands/helix/releases/download", "raw.githubusercontent.com/agenthands/helix/main/minisign.pub", "## Build from source", "git clone https://github.com/agenthands/helix.git"]
    - path: "README.md"
      provides: "Two surgical line edits at lines 65 and 71 -- postfix/serena -> agenthands/helix in the Quick Start install snippet"
      contains: ["go install github.com/agenthands/helix/cmd/serena@latest", "git clone https://github.com/agenthands/helix.git"]
    - path: "Makefile"
      provides: "New `release-snapshot` target following Phase 50 D-07 self-doc style (single ## help annotation naming the gitignored output path); added to .PHONY line 1"
      contains: ["release-snapshot:", "goreleaser release --snapshot --clean --skip=sign", "release-snapshot"]
    - path: "CONTRIBUTING.md"
      provides: "New ## Releasing H2 section placed between 'Running Benchmarks' and 'Adding a New MCP Tool', documenting tag-cut workflow, secret names, dry-run command, key-rotation procedure, one-time keypair setup; mirrors Running Benchmarks house style"
      contains: ["## Releasing", "make release-snapshot", "MINISIGN_PRIVATE_KEY", "MINISIGN_PASSWORD", "minisign -G", "Key details:"]
  key_links:
    - from: "INSTALL.md verify block"
      to: "agenthands/helix GitHub Releases"
      via: "https://github.com/agenthands/helix/releases/download/$VERSION/serena_${VERSION}_${OS}_${ARCH}.tar.gz"
      pattern: "agenthands/helix/releases/download"
    - from: "INSTALL.md verify block"
      to: "minisign.pub at repo root (committed in Plan 01)"
      via: "https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub"
      pattern: "raw.githubusercontent.com/agenthands/helix/main/minisign.pub"
    - from: "Makefile release-snapshot"
      to: ".goreleaser.yaml (committed in Plan 01)"
      via: "goreleaser binary subprocess reads .goreleaser.yaml from working directory"
      pattern: "goreleaser release --snapshot"
    - from: "CONTRIBUTING.md Releasing section"
      to: "release.yml secret references (committed in Plan 01)"
      via: "Documents MINISIGN_PRIVATE_KEY + MINISIGN_PASSWORD secret names that release.yml consumes"
      pattern: "MINISIGN_PRIVATE_KEY"
    - from: "CONTRIBUTING.md Releasing section"
      to: "minisign.pub at repo root (committed in Plan 01)"
      via: "Documents key rotation: edit minisign.pub in a normal commit"
      pattern: "minisign.pub"
---

<objective>
Land the user-facing and contributor-facing documentation that turns the bare release pipeline (Plan 01) into a usable product story. This plan restructures INSTALL.md so a user can download and verify a signed binary in one terminal session (ROADMAP success criterion 3), fixes the postfix/serena -> agenthands/helix repo identity drift in INSTALL.md and README.md, adds a `make release-snapshot` Makefile target so contributors can dry-run the build matrix locally, and documents the maintainer release ceremony (secret setup, tag-cut, key rotation) in CONTRIBUTING.md.

Purpose: Closes ROADMAP success criterion 3 ("A user following INSTALL.md can verify a downloaded binary's signature and checksum in one terminal session"). Closes the public-facing half of Pitfall 2 (repo identity drift). Provides the local dry-run path requested in CONTEXT D-07 / Claude's Discretion. Documents the maintainer-side ceremony Plan 01 cannot self-document (secret upload, key generation).

Output: A user reading INSTALL.md can install a verified binary; a contributor running `make release-snapshot` can produce 6 dry-run archives locally; a maintainer reading CONTRIBUTING.md "Releasing" knows exactly how to cut a release, set up secrets, and rotate the key.

Depends on Plan 01 because: (a) the secret names referenced in CONTRIBUTING.md must match exactly what release.yml consumes; (b) the verify-block URL pattern must match exactly the archive naming `serena_{version}_{os}_{arch}.tar.gz` that .goreleaser.yaml produces; (c) the `checksums.txt` filename must match the `name_template: "checksums.txt"` override in .goreleaser.yaml; (d) `make release-snapshot` invokes `goreleaser` which reads `.goreleaser.yaml`. None of those four files exist before Plan 01.
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
@.planning/phases/51-packaging-goreleaser/51-01-SUMMARY.md
@INSTALL.md
@README.md
@Makefile
@CONTRIBUTING.md

<interfaces>
<!-- This plan has no code interfaces -- all four files are documentation or build-config text. -->
<!-- The "interfaces" are the contracts established by Plan 01 that this plan must reference exactly: -->

From .goreleaser.yaml (Plan 01):
- Archive name template: `serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}` -> `serena_v1.9.0_linux_amd64.tar.gz`
- Checksum filename: `checksums.txt` (literal, NOT `serena_v1.9.0_checksums.txt`)
- Signature filename: `${artifact}.minisig` -> `serena_v1.9.0_linux_amd64.tar.gz.minisig`
- Goreleaser dry-run command: `goreleaser release --snapshot --clean --skip=sign`
- GitHub repo: agenthands/helix (release.github.owner/name)

From .github/workflows/release.yml (Plan 01):
- Secret names: `MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`
- Trigger: `push: tags: ['v*']`
- Pre-release detection: pattern `v*-rc*`, `v*-beta*`, `v*-alpha*` (via goreleaser `prerelease: auto`)

From minisign.pub (Plan 01):
- Lives at repo root
- Two-line ASCII (untrusted comment + base64 key)
- Maintainer overwrites the placeholder with a real key during one-time setup

From Makefile (existing analog -- bench / bench-baseline at lines 44-48):
- Self-doc style: `target: ## one-line help annotation naming output path + gitignored status`
- Single command per recipe, no @-prefix, no multi-step orchestration
- Target name added to `.PHONY` line 1

From CONTRIBUTING.md (existing analog -- "Running Benchmarks" at lines 130-157):
- House style: ```sh fences (NOT ```bash), one-paragraph intro, "Key details:" trailing bullet list, plain prose (no emoji, no admonition syntax)
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add release-snapshot target to Makefile + register on .PHONY (Phase 50 D-07 style)</name>
  <files>Makefile</files>
  <read_first>
    - Makefile (full read -- 49 lines; the analog `bench-baseline` is on line 47)
    - .planning/phases/51-packaging-goreleaser/51-PATTERNS.md section "`Makefile` (MODIFY -- add `release-snapshot` target)" -- house style rules extracted from `bench-baseline`
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 6: Makefile release-snapshot target"
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "decisions" -> Claude's Discretion bullet "The exact local dry-run command surface -- likely a Makefile target mirroring the self-doc style from Phase 50 D-07"
    - .gitignore (line 77 confirms `dist/` is already ignored -- no edit needed; mirrors how `test/bench/baselines/local.txt` is gitignored for `bench-baseline`)
  </read_first>
  <action>
Modify `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/Makefile` with two surgical edits.

**Edit 1: Append `release-snapshot` to the .PHONY list on line 1.**

Current line 1:
```
.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline
```

Change to (single space-separated append at the end -- alphabetical ordering is NOT enforced; matches how `bench` / `bench-baseline` were appended):
```
.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline release-snapshot
```

**Edit 2: Append the `release-snapshot` target as the last block in the file (after `bench-baseline` on line 47-48).**

Add a blank line after the existing `bench-baseline` recipe, then this exact block (note the single TAB indent on the recipe line -- Make is whitespace-sensitive; SPACES will fail with `*** missing separator`):

```makefile
release-snapshot: ## Run a local goreleaser dry-run; writes archives to dist/ (overwrites; gitignored)
	goreleaser release --snapshot --clean --skip=sign
```

Style mirrors `bench-baseline` exactly:
- Single-line `## help` annotation naming the **fixed output path** (`dist/`) and **gitignored** status (mirrors "test/bench/baselines/local.txt (gitignored, overwrites)")
- One short imperative-voice sentence
- No `NAME=` parameter -- single fixed-path output
- Single command line, no `@` prefix, no multi-step orchestration
- Recipe indented with one literal TAB (Makefile syntax requirement)

**Why `--skip=sign` locally:** Most contributors do NOT have the project's minisign secret key (it lives only as a GitHub Actions secret). Signing locally would fail with "no such file: /tmp/minisign.key". The dry-run verifies that the build matrix produces 6 archives + checksums.txt; signing is exercised in CI only via release.yml (Plan 01). Mirrors the Phase 50 pattern of "make this runnable locally without CI-only state".

**Prerequisite for the contributor:** `goreleaser` must be installed (`brew install goreleaser` on macOS, or download a release tarball from `github.com/goreleaser/goreleaser/releases` on Linux). This is documented in CONTRIBUTING.md "Releasing" subsection (Task 4).
  </action>
  <verify>
    <automated>grep -q "^\.PHONY:.*release-snapshot" Makefile &amp;&amp; grep -q "^release-snapshot:" Makefile &amp;&amp; grep -q "goreleaser release --snapshot --clean --skip=sign" Makefile &amp;&amp; awk '/^release-snapshot:/{getline; if (substr($0,1,1)=="\t") exit 0; else exit 1}' Makefile</automated>
  </verify>
  <acceptance_criteria>
    - `Makefile` line 1 contains `release-snapshot` as a member of the `.PHONY:` space-separated list
    - `Makefile` contains a target line starting with `release-snapshot:` (column 1) with `## ` help annotation containing the substrings `dist/` and `gitignored`
    - The recipe line directly under `release-snapshot:` is exactly `\tgoreleaser release --snapshot --clean --skip=sign` (TAB-prefixed; literal command)
    - The recipe is exactly ONE line (no multi-line orchestration -- matches `bench-baseline` shape)
    - `make -n release-snapshot` (dry-run) prints `goreleaser release --snapshot --clean --skip=sign` and exits 0 (works even if goreleaser is not installed because `-n` does not execute)
    - File ends with a newline (POSIX make convention)
    - No other Makefile targets were modified (`bench`, `bench-baseline`, `build`, etc. unchanged)
  </acceptance_criteria>
  <done>`make release-snapshot` is callable on any contributor machine that has `goreleaser` on PATH. The target follows the Phase 50 self-doc style verbatim. Output goes to `dist/` (already gitignored). No CI-only state is required for local execution.</done>
</task>

<task type="auto">
  <name>Task 2: Restructure INSTALL.md per D-04 + D-05 (pre-built lead, copy-paste verify block, agenthands/helix repo identity)</name>
  <files>INSTALL.md</files>
  <read_first>
    - INSTALL.md (full read -- 247 lines; lines 1-29 are what gets demoted to "Build from source"; lines 31-247 stay UNCHANGED)
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "decisions" -- D-04 (single fenced block), D-05 (pre-built leads, build-from-source demoted)
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Code Examples -- Verifying a release on Linux (D-04 verify block -- final form)" -- the exact final form with placeholders resolved
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Code Examples -- macOS verification one-line note (parallel to D-04)" -- the recommended one-line macOS note
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pitfall 2: Repo identity drift (postfix -> agenthands)" -- exact line numbers (10, 18) and what to change
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Open Questions" -> Q1 (do NOT silently rename go.mod; either delete the `go install postfix/serena` line or leave it flagged with a TODO)
    - .planning/phases/51-packaging-goreleaser/51-PATTERNS.md section "`INSTALL.md` (MODIFY -- restructure per D-05)" -- structural plan
  </read_first>
  <action>
Restructure `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/INSTALL.md` per D-04 and D-05. Surgical edit: lines 1-29 are reorganized; lines 31-247 stay UNCHANGED.

**Step A: Replace lines 1-29 with a new structure.**

Read the current top of the file (verbatim):

```markdown
# Install Guide

Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

## Prerequisites

**Install the binary:**

```bash
go install github.com/postfix/serena/cmd/serena@latest
```

Requires Go 1.25 or later.

Or build from source:

```bash
git clone https://github.com/postfix/serena.git
cd serena
go build ./cmd/serena
```

Verify the binary is in your PATH:

```bash
serena --help
```

If `serena` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.
```

Replace those 29 lines with the following exact content (then leave lines 31 onward -- starting with the `## Quick Start` heading -- UNCHANGED):

```markdown
# Install Guide

Serena is a single Go binary. Install it, point your coding agent at it, and you are ready to go.

## Install (pre-built binary)

Pre-built binaries for darwin/linux/windows on amd64/arm64 are published on the [Releases page](https://github.com/agenthands/helix/releases) for every tagged version. Each archive ships with a SHA-256 checksum and a minisign signature.

The verification recipe below downloads an archive, verifies the checksum, verifies the cryptographic signature, and extracts the binary -- a single block you can copy and paste end-to-end. Fill in `VERSION`, `OS`, and `ARCH` for your platform.

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

**macOS users:** replace `sha256sum -c` with `shasum -a 256 -c`, and install minisign with `brew install minisign`. Everything else is identical.

Supported `OS` values: `darwin`, `linux`, `windows`. Supported `ARCH` values: `amd64`, `arm64`. Modern Windows (10 1803+) ships `tar` in System32, so the same `.tar.gz` archive extracts on Windows without third-party tools.

## Build from source

If you prefer to build the binary yourself:

```bash
git clone https://github.com/agenthands/helix.git
cd helix
go build ./cmd/serena
```

Requires Go 1.25 or later. Verify the binary is in your PATH:

```bash
serena --help
```

If `serena` is not found, ensure `$(go env GOPATH)/bin` is in your PATH.

```

(Note: the closing ``` of the verify block above is part of INSTALL.md content -- the executor copies the markdown verbatim including the fence-close.)

**Step B: Do NOT touch lines 31 onwards.** The current `## Quick Start` H2 (line 31 in the original file) and everything after it (HTTP Mode, Manual Configuration, Verify Installation, Next Steps, Legacy Python sections, totaling ~217 lines through the end at line 247) stays exactly as-is. The restructure is surgical -- only the top ~29 lines move.

**Repo identity decisions (per RESEARCH Open Question 1):**
- `git clone https://github.com/postfix/serena.git` -> `git clone https://github.com/agenthands/helix.git` -- DONE in the new "Build from source" block above.
- `cd serena` -> `cd helix` -- DONE (the cloned directory is named after the repo).
- The original `go install github.com/postfix/serena/cmd/serena@latest` line is OMITTED entirely from the restructure. Rationale per RESEARCH Q1: until `go.mod` is renamed in a separate decision, that command would 404 against Go's proxy if anyone ran it post-publish. Removing it from INSTALL.md is the safe move; users who want a one-liner install have `go install` available against `github.com/postfix/serena` (still works because go.mod retains that path), but advertising it in the restructured doc would be misleading. The "Build from source" block above is the canonical source-install path now.

**Verify-block non-negotiables (every executor must preserve):**
1. The bash block is a SINGLE fenced block (one ```bash ... ``` pair). Do NOT split into multiple blocks.
2. The block uses `agenthands/helix` (NOT `<owner>/<repo>` placeholder, NOT `postfix/serena`).
3. The block uses `minisign.pub` (the literal filename committed at repo root in Plan 01) and fetches it from `raw.githubusercontent.com/agenthands/helix/main/minisign.pub`.
4. The block uses `checksums.txt` (the literal filename produced by `.goreleaser.yaml`'s `checksum.name_template: "checksums.txt"` override -- NOT `serena_v1.9.0_checksums.txt`).
5. The block uses `serena_${VERSION}_${OS}_${ARCH}.tar.gz` archive names (matching `.goreleaser.yaml`'s `name_template: "serena_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`).
6. The macOS note is a SINGLE inline paragraph (per CONTEXT D-04 Discretion -- the planner picked the one-line-note shape over a parallel fenced block).
7. The verify block uses `minisign -V -p minisign.pub -m <archive>` syntax (per RESEARCH Code Examples -- this is the canonical jedisct1/minisign verify form).
  </action>
  <verify>
    <automated>grep -q "^## Install (pre-built binary)$" INSTALL.md &amp;&amp; grep -q "^## Build from source$" INSTALL.md &amp;&amp; grep -q "agenthands/helix/releases/download" INSTALL.md &amp;&amp; grep -q "raw.githubusercontent.com/agenthands/helix/main/minisign.pub" INSTALL.md &amp;&amp; grep -q "minisign -V -p minisign.pub" INSTALL.md &amp;&amp; grep -q "sha256sum -c --ignore-missing checksums.txt" INSTALL.md &amp;&amp; grep -q "shasum -a 256 -c" INSTALL.md &amp;&amp; grep -q "git clone https://github.com/agenthands/helix.git" INSTALL.md &amp;&amp; ! grep -q "git clone https://github.com/postfix/serena.git" INSTALL.md &amp;&amp; ! grep -q "github.com/postfix/serena/cmd/serena@latest" INSTALL.md &amp;&amp; grep -q "^## Quick Start" INSTALL.md</automated>
  </verify>
  <acceptance_criteria>
    - INSTALL.md contains a `## Install (pre-built binary)` H2 section
    - INSTALL.md contains a `## Build from source` H2 section
    - The `## Install (pre-built binary)` heading appears BEFORE `## Build from source` (line number ordering verified by `grep -n`)
    - The `## Build from source` heading appears BEFORE the existing `## Quick Start` heading (which itself stays unchanged)
    - The verify block contains all 4 `curl -LO` invocations (archive, .minisig, checksums.txt, minisign.pub)
    - The verify block contains exactly ONE `sha256sum -c --ignore-missing checksums.txt` line and ONE `minisign -V -p minisign.pub -m serena_${VERSION}_${OS}_${ARCH}.tar.gz` line
    - The macOS note is present as inline prose (not a separate fenced block) and contains both `shasum -a 256 -c` and `brew install minisign`
    - Repo URLs use `agenthands/helix` exclusively in the new sections; the old `postfix/serena` URLs are GONE from INSTALL.md (`grep -c "postfix/serena" INSTALL.md` returns 0)
    - The legacy `go install github.com/postfix/serena/cmd/serena@latest` line is removed (NOT preserved with a TODO -- per RESEARCH Q1 recommendation: omit until go.mod rename decision)
    - Lines from `## Quick Start` (was line 31 in the original) through end of file are byte-identical to the original (no incidental edits to Quick Start, HTTP Mode, Manual Configuration, Verify Installation, Next Steps, Legacy Python sections)
    - File ends with a newline
  </acceptance_criteria>
  <done>A user reading INSTALL.md sees the pre-built binary install path FIRST, with a single copy-paste verification ceremony. The repo identity is consistently `agenthands/helix`. The "Build from source" path remains for power users but is demoted. ROADMAP success criterion 3 is achievable end-to-end against a real Release once Plan 01's pipeline ships its first artifact.</done>
</task>

<task type="auto">
  <name>Task 3: Fix README.md repo identity drift on lines 65 and 71 (postfix/serena -> agenthands/helix)</name>
  <files>README.md</files>
  <read_first>
    - README.md (read lines 60-80 -- the Quick Start install snippet block; do NOT read or modify the rest of the file)
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pitfall 2: Repo identity drift (postfix -> agenthands)" -- lists exact line numbers (65, 71) confirmed by grep
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Open Questions" -> Q1 (go.mod stays unchanged in this phase; only public-facing doc URLs change)
  </read_first>
  <action>
Make two surgical edits to `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/README.md`. NO other lines change.

**Edit 1: Line 65.**

Current line 65:
```
go install github.com/postfix/serena/cmd/serena@latest
```

Decision per RESEARCH Open Question 1: until `go.mod` is renamed (a separate decision outside this phase), the literal `go install github.com/postfix/serena/cmd/serena@latest` command STILL WORKS against Go's module proxy because `go.mod` still says `module github.com/postfix/serena`. Renaming it to `agenthands/helix` in README.md WITHOUT renaming go.mod would 404. Therefore: **leave line 65 unchanged for now**, but add a blank line immediately after line 65 (before the closing ` ``` ` of the fenced block) with a one-line note explaining the transitional state. The replacement keeps the working command and signposts the post-rename target:

Change the fenced bash block at lines 64-66 (`go install ...`) to:

```bash
go install github.com/postfix/serena/cmd/serena@latest
# Note: module path is github.com/postfix/serena pending a separate rename decision; the public repo lives at agenthands/helix.
```

Rationale: the line is correct as-is; the comment inside the fence keeps the copy-paste safe (shell ignores `#` comments) while making the postfix/agenthands relationship explicit to anyone reading the README.

**Edit 2: Line 71.**

Current line 71:
```
git clone https://github.com/postfix/serena.git
```

This URL is INCORRECT -- the repo physically lives at `github.com/agenthands/helix.git` (`git remote -v` confirms). `git clone https://github.com/postfix/serena.git` would 404 (or hit a stale fork at best). Replace verbatim:

```
git clone https://github.com/agenthands/helix.git
```

The line immediately after (`cd serena`) ALSO needs updating because the cloned directory inherits the repo name. Change `cd serena` to `cd helix` (one occurrence in the same fenced block, lines 71-73).

**Final state of lines 70-74 in the fenced block (the "Or build from source" snippet):**

```bash
git clone https://github.com/agenthands/helix.git
cd helix
go build ./cmd/serena
```

Do NOT modify any other line in README.md. The Quick Start `## Configure Your Client` section (line 76 onwards), Key Features, Architecture, Tools tables, etc. are all out of scope for this plan.
  </action>
  <verify>
    <automated>grep -q "git clone https://github.com/agenthands/helix.git" README.md &amp;&amp; ! grep -q "git clone https://github.com/postfix/serena.git" README.md &amp;&amp; grep -q "^cd helix$" README.md &amp;&amp; grep -q "module path is github.com/postfix/serena pending a separate rename decision" README.md &amp;&amp; grep -q "go install github.com/postfix/serena/cmd/serena@latest" README.md</automated>
  </verify>
  <acceptance_criteria>
    - README.md contains `git clone https://github.com/agenthands/helix.git` (replaces the postfix URL)
    - README.md does NOT contain `git clone https://github.com/postfix/serena.git` (the broken URL is gone)
    - README.md contains `cd helix` (replaces `cd serena`)
    - README.md does NOT contain a `cd serena` line in the Quick Start fenced block (verify by checking the surrounding context with `grep -B2 -A2 "cd helix" README.md`)
    - README.md still contains the literal `go install github.com/postfix/serena/cmd/serena@latest` line (left unchanged because go.mod still says postfix/serena -- per RESEARCH Q1)
    - README.md contains the new in-fence comment line `# Note: module path is github.com/postfix/serena pending a separate rename decision; the public repo lives at agenthands/helix.`
    - No other lines in README.md are modified (`git diff README.md` shows only the Quick Start block changes -- approximately 4 line changes total)
  </acceptance_criteria>
  <done>README.md Quick Start install snippet reflects reality: the `go install` line still works (because go.mod is unchanged) with a clarifying comment, and the `git clone` URL points at the actual repo. No silent go.mod rename. Pitfall 2 closed for the public-facing docs.</done>
</task>

<task type="auto">
  <name>Task 4: Add ## Releasing subsection to CONTRIBUTING.md (between Running Benchmarks and Adding a New MCP Tool)</name>
  <files>CONTRIBUTING.md</files>
  <read_first>
    - CONTRIBUTING.md (full read -- 192 lines; the analog "Running Benchmarks" is at lines 130-157; the next section "Adding a New MCP Tool" starts at line 159)
    - .planning/phases/51-packaging-goreleaser/51-PATTERNS.md section "`CONTRIBUTING.md` (MODIFY -- add 'Releasing' subsection)" -- house style rules verbatim from "Running Benchmarks" analog
    - .planning/phases/51-packaging-goreleaser/51-CONTEXT.md section "decisions" D-02, D-02a, D-04, D-06, D-07 -- material the Releasing section must cover
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Maintainer prerequisite" -- exact `minisign -G -p minisign.pub -s minisign.key` keypair generation command, `gh secret set` upload command, password-protected key rationale (Q2)
    - .planning/phases/51-packaging-goreleaser/51-RESEARCH.md section "Pattern 6: Makefile release-snapshot target" -- the `make release-snapshot` command + `--skip=sign` rationale for local dry-run
  </read_first>
  <action>
Insert a new `## Releasing` H2 subsection in `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/CONTRIBUTING.md` immediately AFTER the "Running Benchmarks" subsection (which currently ends at line 157, just before the blank line and the `## Adding a New MCP Tool` heading on line 159).

The new section MUST mirror the "Running Benchmarks" house style: ` ```sh ` fences (NOT ```bash), one-paragraph intro that names scope and constraints up front, code blocks with surrounding prose, "Key details:" trailing bullet list. No emoji, no admonition syntax (no "Note:", no "> warning"), plain prose only.

Insert exactly this block between the existing "Running Benchmarks" closing line and the existing "## Adding a New MCP Tool" heading. Preserve the blank line before and after the new section (a single blank line separates H2s in the existing file -- mirror that exactly):

```markdown
## Releasing

Releases ship as multi-arch signed binaries via a goreleaser pipeline (see `.goreleaser.yaml` and `.github/workflows/release.yml`). A `v*` git tag triggers the release workflow automatically -- there is no manual draft step. The CI-enforced reproducibility gate refuses to publish if two consecutive snapshot builds produce non-byte-identical archives, so a non-deterministic build cannot reach users.

To dry-run the build matrix locally (signs are skipped because the secret key lives only in CI):

```sh
make release-snapshot
```

Output goes to `dist/` (gitignored, overwrites). On a clean checkout you should see 6 archives (`serena_<version>_<os>_<arch>.tar.gz`) and a `checksums.txt` file. The local dry-run requires `goreleaser` on `$PATH`; install with `brew install goreleaser` on macOS, or download a release tarball from `github.com/goreleaser/goreleaser/releases` on Linux.

To cut a release, push a version tag from a green-CI commit on `main`:

```sh
git tag v1.9.0
git push origin v1.9.0
```

Pre-release tags (`v1.9.0-rc1`, `v1.9.0-beta1`, `v1.9.0-alpha1`) are auto-detected by goreleaser and marked as Pre-release on the GitHub Releases page. Production tags (`v1.9.0`) publish as a regular release.

### Repository secrets

The release workflow signs archives with the project's minisign keypair. Two GitHub Actions secrets must exist on the repo BEFORE the first `v*` tag is pushed:

- `MINISIGN_PRIVATE_KEY` -- contents of the locally generated `minisign.key` file (NOT the file path; the actual file contents pasted into the secret)
- `MINISIGN_PASSWORD` -- the password chosen during keypair generation

These are uploaded under repo Settings > Secrets and variables > Actions.

### One-time keypair setup

The maintainer generates the keypair once on a trusted local machine, commits the public half to the repo, and uploads the private half plus password as repo secrets:

```sh
minisign -G -p minisign.pub -s minisign.key
# minisign prompts for a password; choose a strong one and store it securely
gh secret set MINISIGN_PRIVATE_KEY < minisign.key
gh secret set MINISIGN_PASSWORD   # paste the password when prompted
git add minisign.pub
git commit -m "feat(release): commit minisign public key"
git push
```

After this is done, delete the local `minisign.key` (the secret is now in GitHub's secret store; the local copy is no longer needed and should not linger on disk). The password should be stored in a password manager.

### Key rotation

To rotate the minisign keypair, repeat the one-time setup with a new pair, commit the new `minisign.pub`, and update both repo secrets. Users who have already downloaded the old `minisign.pub` will need to re-fetch it from `https://raw.githubusercontent.com/agenthands/helix/main/minisign.pub` -- INSTALL.md tells them to re-fetch the key as part of the verification recipe, so users on the latest INSTALL.md instructions pick up the rotated key automatically.

Key details:

- The release workflow does NOT re-run `go test` or `go vet` -- tags are assumed to be cut from a commit that has already passed `go-test.yml` on `main`. If you tag a commit that has not been through CI, the release may publish a binary built from broken code (the reproducibility gate cannot catch logic bugs, only build determinism).
- Release notes are auto-generated from the git log between tags using conventional-commit prefix grouping (`feat:`, `fix:`, `docs:`, `refactor:`). `CHANGELOG.md` stays hand-curated separately for human-readable narrative.
- The local dry-run skips signing (`--skip=sign`) because contributors do not have access to the project's minisign secret key -- signing is exercised in CI only. The dry-run still validates the build matrix, archive packaging, and checksums.txt generation.
- A typo'd tag publishes a release immediately; there is no draft step. The reproducibility gate is the safety net against non-deterministic artifacts, not against typo'd tags. If a release is published in error, delete it via the GitHub Releases UI and re-tag with a corrected version.
```

**Placement verification:** The new `## Releasing` heading sits AFTER the last bullet of "Running Benchmarks" ("If your changes are likely to affect bench numbers...") and BEFORE the existing `## Adding a New MCP Tool` heading. The "Adding a New MCP Tool" heading and everything below it stays UNCHANGED.

**Style cross-checks (MUST mirror "Running Benchmarks" exactly):**
1. Use ` ```sh ` for shell fences -- NOT ` ```bash ` -- every code fence in the analog uses `sh`. Verify: `grep -c '^```sh' CONTRIBUTING.md` should INCREASE by the number of fences in the new section (4: dry-run, tag-push, keypair-setup, key-rotation).
2. Use H3 (`###`) for the inline subsections within Releasing (Repository secrets, One-time keypair setup, Key rotation) -- "Running Benchmarks" itself is flat but the Releasing section needs more structure; H3 nesting is the natural fit and matches how some other H2 sections in the file (e.g. the "Running Integration Tests" content) use prose paragraphs to introduce code blocks. Use H3 only for the three sub-procedures; the lead paragraph and "Key details:" trailer stay at the H2 level.
3. "Key details:" trailing bullet list -- exact phrasing matches the analog (`Key details:` with capitalized K and trailing colon; bullets start with `- `).
4. NO emoji, NO admonition syntax (no `> Note:`, no `:::warning`, no `[!IMPORTANT]`).
5. Reference make-targets (`make release-snapshot`) rather than open-coded shell where a target exists -- mirrors the analog's `make bench` reference.
6. Plain prose, imperative voice for procedural steps.
  </action>
  <verify>
    <automated>grep -q "^## Releasing$" CONTRIBUTING.md &amp;&amp; grep -q "make release-snapshot" CONTRIBUTING.md &amp;&amp; grep -q "MINISIGN_PRIVATE_KEY" CONTRIBUTING.md &amp;&amp; grep -q "MINISIGN_PASSWORD" CONTRIBUTING.md &amp;&amp; grep -q "minisign -G -p minisign.pub -s minisign.key" CONTRIBUTING.md &amp;&amp; grep -q "gh secret set MINISIGN_PRIVATE_KEY" CONTRIBUTING.md &amp;&amp; grep -q "raw.githubusercontent.com/agenthands/helix/main/minisign.pub" CONTRIBUTING.md &amp;&amp; grep -q "^Key details:$" CONTRIBUTING.md &amp;&amp; grep -q "^### Repository secrets$" CONTRIBUTING.md &amp;&amp; grep -q "^### One-time keypair setup$" CONTRIBUTING.md &amp;&amp; grep -q "^### Key rotation$" CONTRIBUTING.md &amp;&amp; ! grep -A1 "## Releasing" CONTRIBUTING.md | grep -q '```bash'</automated>
  </verify>
  <acceptance_criteria>
    - CONTRIBUTING.md contains a `## Releasing` H2 section
    - The `## Releasing` heading appears AFTER the "Running Benchmarks" subsection (which ends with the bullet about "the local before/after deltas")
    - The `## Releasing` heading appears BEFORE the existing `## Adding a New MCP Tool` heading
    - The new section contains H3 subsections: `### Repository secrets`, `### One-time keypair setup`, `### Key rotation`
    - The new section contains the exact secret names `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` (matching Plan 01's release.yml refs verbatim)
    - The new section contains the `make release-snapshot` reference (matching the Makefile target added in Task 1 of this plan)
    - The new section contains the `minisign -G -p minisign.pub -s minisign.key` keygen command (matches RESEARCH Maintainer prerequisite)
    - The new section contains the `gh secret set MINISIGN_PRIVATE_KEY < minisign.key` upload command
    - The new section contains the `raw.githubusercontent.com/agenthands/helix/main/minisign.pub` URL (the canonical pubkey distribution URL referenced in INSTALL.md verify block)
    - All shell fences in the new section use ` ```sh ` (NOT ` ```bash ` -- house style match)
    - The new section ends with a `Key details:` bulleted trailer (matching "Running Benchmarks" analog shape)
    - The new section contains NO emoji and NO admonition syntax
    - The "Adding a New MCP Tool" heading and everything after it (including "Adding Language Support", "gopls Compatibility", "Legacy Python") is byte-identical to the original
    - File ends with a newline
  </acceptance_criteria>
  <done>A maintainer reading CONTRIBUTING.md "Releasing" knows exactly how to: (1) cut a release (push a v* tag), (2) what the two repo secrets are called and how to upload them, (3) how to dry-run locally, (4) how to rotate the key, (5) what the safety nets and gotchas are. Style matches the existing "Running Benchmarks" subsection one-for-one.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| User's terminal -> github.com release CDN | INSTALL.md tells the user to `curl -LO https://github.com/agenthands/helix/releases/download/...`. The user is trusting GitHub's TLS + the project's release pipeline. The minisign verification step is the local trust anchor that catches a tampered archive even if the CDN is compromised. |
| User's terminal -> raw.githubusercontent.com (minisign.pub) | INSTALL.md tells the user to fetch the pubkey from `raw.githubusercontent.com/agenthands/helix/main/minisign.pub`. A user fetching the key for the first time has no prior trust anchor to verify it against -- this is the standard TOFU (trust-on-first-use) problem for any signature scheme. Mitigated only by GitHub's TLS + the user's choice to trust the project. |
| Maintainer's local machine -> GitHub repo | Maintainer commits `minisign.pub` and uploads `MINISIGN_PRIVATE_KEY` + `MINISIGN_PASSWORD` as secrets. CONTRIBUTING.md tells the maintainer to delete the local `minisign.key` after upload (no lingering file on disk). |
| Contributor's local machine -> goreleaser | `make release-snapshot` invokes a locally installed `goreleaser` binary. Contributor is trusting their own Homebrew tap or jedisct1/minisign release tarball. Local dry-run does NOT touch any secrets. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-51-09 (T-doc-instructs-skipping-verify) | Tampering | INSTALL.md verify block | mitigate | The verify block is a SINGLE copy-paste flow that includes BOTH `sha256sum -c` and `minisign -V` steps inline. Users cannot skip verification by accident -- they would have to actively delete lines from the block. The macOS one-line note preserves both verify steps (only the `sha256sum` -> `shasum` substitution differs). ASVS V14.4. |
| T-51-10 (T-stale-pubkey-doc) | Spoofing | INSTALL.md pubkey URL | mitigate | INSTALL.md tells users to fetch from `raw.githubusercontent.com/agenthands/helix/main/minisign.pub` -- ALWAYS the current key from the main branch. After a key rotation, users following the latest INSTALL.md pick up the new key automatically. CONTRIBUTING.md "Key rotation" subsection explicitly documents this re-fetch behavior. ASVS V6.4. |
| T-51-11 (T-makefile-secret-leak) | Information Disclosure | `make release-snapshot` | accept | The Makefile target uses `--skip=sign`, so it never reads or references any secret. There is no path by which `make release-snapshot` could leak a secret because no secret is in scope. (Documented as accept rather than mitigate because there is no threat to mitigate -- this is a "by construction" property.) |
| T-51-12 (T-contributing-instructs-secret-on-disk) | Information Disclosure | CONTRIBUTING.md keypair-setup procedure | mitigate | The procedure tells the maintainer to delete the local `minisign.key` after uploading it as a secret. The password is to be stored in a password manager (NOT in plain text on disk). The procedure does NOT instruct any `cat`, `echo`, or `printf` of the secret to a terminal where it might end up in shell history. ASVS V2.10. |
| T-51-13 (T-readme-go-install-stale) | Tampering / Confusion | README.md Quick Start `go install` line | accept | The README.md line `go install github.com/postfix/serena/cmd/serena@latest` is preserved as-is because go.mod still says `module github.com/postfix/serena`. The in-fence comment explains the postfix/agenthands relationship transparently. Risk accepted: a user might be confused, but the command actually works. The full rename is a separate decision per RESEARCH Q1. |
| T-51-14 (T-install-md-references-go-install-broken) | Confusion | INSTALL.md (legacy `go install` line removed) | mitigate | RESEARCH Q1 recommendation: omit the `go install` line from INSTALL.md to avoid pointing users at a path that will break post-go.mod-rename. The "Build from source" section uses `git clone agenthands/helix` exclusively. Users who want a one-liner can still find it in README.md (with the explanatory comment), but INSTALL.md as the authoritative install guide does not advertise it. |

**Severity assessment:** No HIGH-severity threats. T-51-11 and T-51-13 are accepted with documented rationale (T-51-11 is a property of the design; T-51-13 is a transitional state pending a separate go.mod rename decision).
</threat_model>

<verification>
Plan-level verification (after all 4 tasks complete):

1. **Makefile dry-run sanity:** `make -n release-snapshot` prints `goreleaser release --snapshot --clean --skip=sign` and exits 0.
2. **If goreleaser is installed locally:** `make release-snapshot` runs end-to-end; `ls dist/serena_*.tar.gz | wc -l` returns 6; `test -f dist/checksums.txt` exits 0. (If goreleaser is not installed, this verification is deferred to CONTRIBUTING.md "Releasing" walkthrough during /gsd-verify-work.)
3. **INSTALL.md repo identity:** `grep -c "postfix/serena" INSTALL.md` returns 0 (the postfix URLs are gone from INSTALL.md; the new structure uses agenthands/helix throughout).
4. **README.md repo identity:** `grep -c "git clone https://github.com/postfix/serena.git" README.md` returns 0 (the broken clone URL is fixed); `grep -c "go install github.com/postfix/serena/cmd/serena@latest" README.md` returns 1 (the working go install line is preserved with a clarifying comment per Q1).
5. **CONTRIBUTING.md cross-references match Plan 01:** the secret names `MINISIGN_PRIVATE_KEY` and `MINISIGN_PASSWORD` in CONTRIBUTING.md match exactly what release.yml consumes (string-for-string).
6. **House style preserved:** `grep -c '^```sh' CONTRIBUTING.md` increased by 4 (one fence per code block in the new section); `grep -c '^```bash' CONTRIBUTING.md` did NOT increase (no bash fences in the new section).
7. **No incidental edits:** `git diff README.md` shows only the Quick Start block changes (~4 lines); `git diff INSTALL.md` shows the top ~29 lines restructured + new Build from source content; `git diff CONTRIBUTING.md` shows ONLY the new ## Releasing section insertion.
8. **Code unaffected:** `go vet ./...` and `go test ./...` continue to pass (this plan touches NO Go source files).

End-to-end UAT (deferred to /gsd-verify-work, sampled at the phase gate per VALIDATION.md): cut a `v0.0.0-rc-test` tag on a test branch -> observe release.yml fires -> reproducibility gate passes -> 6 archives + 6 .minisig + checksums.txt published -> a fresh shell on darwin and linux can copy-paste the INSTALL.md verify block end-to-end and `serena --help` exits 0. This UAT requires the maintainer to have completed the one-time keypair setup documented in CONTRIBUTING.md.
</verification>

<success_criteria>
Plan 02 succeeds when ALL of the following are true:

1. `Makefile` has a `release-snapshot` target on `.PHONY` line 1 with the recipe `goreleaser release --snapshot --clean --skip=sign` and a `## help` annotation matching the Phase 50 D-07 self-doc style.
2. `INSTALL.md` leads with `## Install (pre-built binary)` containing the D-04 single-block verify ceremony with `agenthands/helix` URLs and `minisign.pub`/`checksums.txt` filenames matching Plan 01's pipeline output. `## Build from source` is demoted below it. The original Quick Start, HTTP Mode, Manual Configuration, etc. sections are byte-identical from the original.
3. `README.md` Quick Start install snippet uses `git clone https://github.com/agenthands/helix.git` + `cd helix` (replacing the broken postfix URL); the `go install github.com/postfix/serena/cmd/serena@latest` line is preserved with a clarifying in-fence comment about the pending go.mod rename.
4. `CONTRIBUTING.md` has a new `## Releasing` H2 section between "Running Benchmarks" and "Adding a New MCP Tool", with H3 subsections (Repository secrets, One-time keypair setup, Key rotation), house-style ` ```sh ` fences, and a "Key details:" trailing bullet list. The section references `MINISIGN_PRIVATE_KEY`, `MINISIGN_PASSWORD`, `make release-snapshot`, and the canonical `raw.githubusercontent.com/agenthands/helix/main/minisign.pub` URL.
5. Every threat in the STRIDE register has a mitigation tied to a specific text change in this plan, or is accepted with documented residual-risk rationale (T-51-11 and T-51-13).
6. `git status` shows exactly the 4 modified files (no incidental edits to other files); `git diff` for each file matches the surgical scope described in the corresponding task action.
7. `go vet ./...` and `go test ./...` continue to pass (no Go source files were modified).

ROADMAP success criterion 3 ("A user following INSTALL.md can verify a downloaded binary's signature and checksum in one terminal session") is now achievable end-to-end against any signed Release published by Plan 01's pipeline.
</success_criteria>

<output>
After completion, create `.planning/phases/51-packaging-goreleaser/51-02-SUMMARY.md` capturing:
- The four files modified with one-line description each.
- Whether `make release-snapshot` was exercised locally (and produced 6 archives) or deferred (goreleaser not installed locally).
- Confirmation that no Go sources were modified (`go vet ./...` and `go test ./...` baseline status preserved).
- Cross-reference check: secret names + filenames + URLs in this plan match Plan 01's pipeline output exactly (string-for-string).
- Outstanding maintainer prerequisite (carried from Plan 01): generate keypair locally, overwrite `minisign.pub`, upload `MINISIGN_PRIVATE_KEY` + `MINISIGN_PASSWORD` secrets via `gh secret set` or the GitHub web UI BEFORE pushing the first `v*` tag. CONTRIBUTING.md "Releasing > One-time keypair setup" is the canonical reference.
- Phase gate UAT (per VALIDATION.md): cutting a throwaway `v0.0.0-rc-test` tag on a test branch must fire release.yml, pass the reproducibility gate, publish 6 archives + 6 .minisig + checksums.txt, and a fresh shell must be able to copy-paste the INSTALL.md verify block end-to-end on darwin and linux. This UAT is deferred to `/gsd-verify-work` because it requires the maintainer to have completed the one-time keypair setup.
</output>
