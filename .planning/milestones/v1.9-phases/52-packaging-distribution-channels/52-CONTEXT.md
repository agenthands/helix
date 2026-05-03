# Phase 52: packaging-distribution-channels - Context

**Gathered:** 2026-04-29
**Status:** Ready for planning

> **Phase rescoped during discussion.** The original ROADMAP scope (PKG-02 Homebrew tap, PKG-03 Scoop bucket, PKG-04 native Linux package) is **dropped**. The user explicitly abandoned the package-manager distribution model in favor of self-contained binary archives (continued from Phase 51) plus an in-binary self-upgrade subcommand. PKG-02/03/04 are removed from v1.9 scope; if revisited later, they would be additive on the goreleaser pipeline already in place. ROADMAP.md and REQUIREMENTS.md must be updated by the planner / first plan as part of this phase.

<domain>
## Phase Boundary

This phase delivers two coupled changes:

1. **Product/binary rename:** `serena` → `helix`. Hard cut at v1.9. The CLI binary, archive names, env-var prefix, config-dir names, project-marker dirs, MCP server registration, all docs/branding flip to "Helix". The repo (`agenthands/helix`) and Go module path (`github.com/agenthands/helix`) already match — no module rename. No backwards-compat symlink. Existing user installs lose their config on upgrade and must re-onboard via `helix setup <client>`.

2. **In-binary self-upgrade subcommand:** `helix update` (read-only check) and `helix upgrade` (install) that downloads the latest GitHub Release archive for the running OS/arch, verifies its minisign signature against the embedded `minisign.pub`, atomically swaps the binary in place (inode-replace on Unix, rename-current-to-`.old` on Windows), and `os.Exec`s the new binary with the same args.

Plus the supporting work that makes both possible: an embed audit producing `EMBED-AUDIT.md` (the living manifest of what's in-binary vs. runtime-resolved) and embedding `minisign.pub` so the running binary can verify the next archive.

**In scope:**
- Binary + archive + product rename `serena` → `helix` (cmd dir, goreleaser config, archive name template, all docs).
- Env-var rename `SERENA_*` → `HELIX_*` (no fallback).
- Config + project-marker dir rename `~/.serena/` → `~/.helix/`, `.serena/project.yml` → `.helix/project.yml`, `.serena/memories/` → `.helix/memories/` (no fallback).
- MCP server registration name updated (the string registered with `claude mcp add-json` and equivalents in `internal/cli/setup_clients.go`).
- `helix update` subcommand: GitHub Releases API check; print `current: vX / latest: vY` and the release-notes summary from the GitHub Release body.
- `helix upgrade` subcommand: download → minisign-verify → atomic swap → `os.Exec` with same args. Daemon-aware ("restart daemon manually" hint when running in-daemon).
- Flags on `helix upgrade`: `--prerelease`, `--version vX.Y.Z`, `--check`, `--dry-run`. `--version` always wins over `--prerelease`.
- Hard-refuse downgrade: if remote latest ≤ current (or `--version` targets older), exit 0 with "already up to date" message. No `--force-downgrade`.
- Permission probe BEFORE download: if install path is not writable by the current user, print sudo hint with the exact re-invocation and exit cleanly. No internal sudo prompt.
- `//go:embed minisign.pub` so the binary self-verifies the next archive. Single source of truth — same file consumed by INSTALL.md curl-fetch.
- Embed audit producing `.planning/phases/52-packaging-distribution-channels/EMBED-AUDIT.md`: a written manifest classifying every runtime asset read as embedded / external-by-design / gap-to-close. Gaps closed in this phase.
- INSTALL.md updated to document the upgrade workflow and the new product/binary name.
- CHANGELOG.md v1.9 entry documenting the rename + self-upgrade.
- REQUIREMENTS.md edits: invert "Auto-update mechanism inside the binary | Out of scope" (its rationale was "package managers own updates" — that no longer holds); mark PKG-02/03/04 as deferred.
- ROADMAP.md edits: rewrite Phase 52 goal/success-criteria to match this rescoped definition.

**Out of scope (other phases or future milestones):**
- Homebrew tap (PKG-02 → deferred from v1.9 entirely).
- Scoop bucket (PKG-03 → deferred from v1.9 entirely).
- Native Linux packages — apt/deb, rpm, AUR (PKG-04 → deferred).
- Embedding LS binaries (gopls, jdtls, rust-analyzer, etc.) — they stay runtime-resolved by the three-tier installer. The "self-contained" property covers Helix itself, not the LSes it drives. Rationale: jdtls alone is ~200MB; embedding all 52 LSes would push archives to GB-class.
- Curl-pipe-sh installer script (`install.sh` / iwr-iex) — explicitly rejected; the Phase 51 manual copy-paste verification recipe remains the install path.
- A migration tool / fallback shim that reads old `~/.serena/` configs. Hard cut. Users re-onboard.
- Container images (PKG-DEFER-02, deferred project-wide).

</domain>

<decisions>
## Implementation Decisions

### Binary + product rename

- **D-01:** **Binary renamed `serena` → `helix`.** Hard cut at v1.9. Archive naming template in `.goreleaser.yaml` flips from `serena_v{{ .Version }}_*` to `helix_v{{ .Version }}_*`. Build dir `cmd/serena/main.go` moves to `cmd/helix/main.go`. **The Go module path renames `github.com/postfix/serena` → `github.com/agenthands/helix`** (resolved 2026-04-29 after research found the original CONTEXT.md "module path unchanged" assumption was wrong: actual `go.mod` is `github.com/postfix/serena` with 392 import-line references across the codebase). Rationale: user explicitly chose the clean-break principle (D-04); a residual `postfix/serena` import path would contradict it. The mechanical edit is a single sed + `go build` catches misses.
- **D-02:** **Full env-var + config-path rename, no transparent migration.** `SERENA_*` env vars become `HELIX_*` (every match in the codebase). User-config dir `~/.serena/` becomes `~/.helix/`; per-project marker `.serena/project.yml` becomes `.helix/project.yml`; memories `.serena/memories/` becomes `.helix/memories/`. No fallback — existing users lose their config on upgrade and must re-onboard via `helix setup <client>`. Rationale: a one-time clean break is preferable to a permanent fallback codebase. CHANGELOG.md must call this out as a breaking change.
- **D-03:** **Product is "Helix"** in PROJECT.md, README.md, USAGE.md, INSTALL.md, CHANGELOG.md, log lines, and every other branding surface. The `helix --version` output prints "Helix" as the product name. Original-Serena attribution can stay in PROJECT.md / CHANGELOG.md history but the active product is Helix.
- **D-04:** **No backwards-compat symlink or wrapper.** Archive contains only the `helix` binary. Users with scripts invoking `serena` get a clean "command not found" failure; CHANGELOG documents the break and recommends `command -v helix || echo 'rename helix from serena'`-style fixups. Rationale: a symlink that ages out in v1.10 is half the work for a fraction of the value; users prefer a clean break to an "is it still there?" ambiguity.
- **D-05:** **MCP server registration name flips.** Whatever string `internal/cli/setup_clients.go` currently registers with `claude mcp add-json` (and equivalents for VS Code, JetBrains, OpenCode, Gemini CLI) becomes `helix`. Existing users with `serena` registered must re-run `helix setup <client>` to drop the old registration and add the new one. The setup CLI should help: print "previous `serena` MCP registration detected — remove with `claude mcp remove serena`" if it can detect one.

### In-binary self-upgrade

- **D-06:** **Two distinct verbs:** `helix update` is read-only — it queries the GitHub Releases API, prints `current: vX / latest: vY` plus the release-notes body summary fetched live from the release, and exits. `helix upgrade` is the install verb. They are not aliases. Rationale: matches `apt update` / `apt upgrade` semantics; makes "just check" cheap and obvious.
- **D-07:** **Upgrade flow:** GitHub Releases API check → download archive for current OS/arch into a stage dir (planner picks; `os.TempDir()` or `~/.cache/helix/upgrade-stage/` are both acceptable) → verify minisign signature using the embedded public key → extract → atomic swap → `os.Exec` the new binary with the same args minus the `upgrade` verb. Inode-replace on Linux/macOS; on Windows, rename current to `.old`, write new to original path, then `os.Exec`. After successful swap, re-launch is mandatory (this is the "after swap, do what" decision below).
- **D-08:** **Restart via `os.Exec` same args.** After a successful swap, the upgrade subcommand re-launches the new binary in place of itself, passing the original arguments minus the `upgrade` verb. Daemon-aware: if the upgrade is invoked from inside the daemon process (detect by reading the binary's own pid/role), do NOT exec — print "restart the daemon manually" with a clean instruction and exit cleanly. The user sees the new version's startup banner.
- **D-09:** **Permission probe BEFORE download.** Before any network I/O, the upgrade subcommand checks write access to the install path (the path of the running binary). If unwritable, print the exact re-invocation: `helix lives at /usr/local/bin/helix — re-run with: sudo helix upgrade <flags>` and exit. No download attempt; no internal sudo prompt; no privilege-escalation surprises. Rationale: failing fast saves a multi-MB download and avoids a permission-error confusion mid-flow.
- **D-10:** **Hard-refuse downgrades, no override flag.** If `latest <= current` (semver comparison), or `--version vX.Y.Z` targets a version older than current, exit 0 with `helix is already up to date (or newer than upstream)`. No `--force-downgrade`, no `--allow-downgrade`. Users who want an older version go to GitHub Releases manually and run the Phase 51 verification recipe. Rationale: prevents rollback attacks and accidental downgrades; users wanting a downgrade have a perfectly good manual path already.
- **D-11:** **Default upgrade target = latest stable only.** Tags matching `v*-rc*` / `v*-beta*` / `v*-alpha*` are skipped unless `--prerelease` is set. Matches every package manager's user expectation.
- **D-12:** **Flags shipped in v1.9:** `--prerelease`, `--version vX.Y.Z`, `--check`, `--dry-run`. Composition rule: `--version` always wins over `--prerelease` (explicit pin > channel default). `--check` (on `upgrade`) is functionally identical to `helix update` and is provided for muscle-memory convenience. `--dry-run` runs check + download + signature verification but skips the swap and exit-replace.

### Embed audit + self-verify

- **D-13:** **Embed `minisign.pub` via `//go:embed` with a build-time-synced copy.** Repo-root `minisign.pub` remains the single source of truth (INSTALL.md curl-fetch keeps reading it). A Makefile pre-build step + `go generate` directive copies `minisign.pub` into `internal/upgrade/minisign.pub` (the package-local synced copy) before every build; the Go file in `internal/upgrade/` embeds the package-local copy via `//go:embed minisign.pub`. **Resolved 2026-04-29 after research found the originally-suggested `//go:embed ../../minisign.pub` is invalid Go — the embed directive forbids `..` traversal (golang/go#46056).** A CI gate verifies the two files are byte-identical (fails the build if they drift). Rationale: keeps single-source-of-truth at repo root, satisfies Go's embed constraints, automatic sync on every build, manual key-rotation still touches one file. Hardcoded `const` strings are explicitly NOT used.
- **D-14:** **Embed audit produces a written manifest.** The first plan in this phase runs the audit (grep for `os.ReadFile`, `os.Open`, `embed:` directives, fixture-loading code, `filepath.Join` reads relative to the binary location) and writes `.planning/phases/52-packaging-distribution-channels/EMBED-AUDIT.md`. The manifest classifies every finding as: **embedded** (already `//go:embed`'d), **external-by-design** (LS binaries, user config, project workspace files), or **gap** (should be embedded but isn't). Subsequent plans in this phase close the gaps. The manifest stays in the phase directory as a living artifact; future phases adding disk-reads check it.
- **D-15:** **LS binaries stay runtime-downloaded** by the three-tier installer (`internal/langregistry/`). The "self-contained binary" property covers Helix itself. Rationale: jdtls is ~200MB, rust-analyzer ~30MB, gopls bundled with the Go toolchain — embedding everything would push archives to GB-class which goreleaser-CI cannot realistically build, sign, and reproducibility-verify on tag in reasonable time. EMBED-AUDIT.md classifies every LS as **external-by-design**.

### Claude's Discretion

The planner has authority to decide the following without re-asking:
- Exact stage-dir path for the upgrade download (`os.TempDir()` vs. `os.UserCacheDir()/helix/upgrade-stage/` vs. next to the running binary). Either is fine; planner picks based on disk-pressure and cleanup ergonomics.
- Exact GitHub Releases API endpoint pattern (latest vs. tags vs. releases) — researcher picks based on what's needed for both `update` (release notes) and `upgrade` (asset URL).
- The exact daemon-detection heuristic for D-08's "do not exec, print manual hint" branch (e.g., env var set by daemon supervisor, parent-pid match, dedicated flag).
- Wording of error messages and the sudo hint string in D-09 (must be unambiguous; planner has freedom on tone).
- The exact location of the `//go:embed minisign.pub` file (`internal/upgrade/`, `internal/release/`, `internal/cli/upgrade/`, etc.).
- Exit codes for `--dry-run` (0 if everything except swap succeeded; non-zero if check or verify failed).
- Whether `helix update` and `helix upgrade --check` print the same output or slightly different formatting.
- Whether the binary rename is done in one atomic plan (move `cmd/serena/` → `cmd/helix/`, sed all references in one commit) or staged across multiple plans (cmd-dir + goreleaser first, then env vars, then config dirs, then docs). Planner picks based on git-log readability vs. CI-bisect-ability.
- Exact message text for the "previous `serena` MCP registration detected" nudge in D-05 (or whether to ship that nudge at all in v1.9 vs. defer to v1.10 since most users will simply re-run setup).
- Whether `cmd/helix/main.go` keeps the same exact source as `cmd/serena/main.go` minus path-string updates, or gets restructured during the move (preference: minimal change — just rename + path fixups).

### Folded Todos
None — `gsd-sdk query todo.match-phase 52` returned no matches.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project decisions and constraints
- `.planning/PROJECT.md` — current product identity (still "Serena" until Phase 52 ships); core value statement; tech-debt list.
- `.planning/REQUIREMENTS.md` — REQUIRES EDITS in this phase: invert the "Auto-update mechanism inside the binary | Out of scope" row (rationale invalid post-rescope); mark PKG-02/03/04 as deferred from v1.9 entirely.
- `.planning/ROADMAP.md` — Phase 52 entry REQUIRES REWRITE in this phase: original goal/success-criteria reference brew/scoop/native-Linux which are now out of scope. New goal/success-criteria align with the rescoped phase boundary above.
- `.planning/STATE.md` — current milestone position (Phase 51.1 complete; Phase 52 next).

### Prior phase context (carry-forward)
- `.planning/phases/51-packaging-goreleaser/51-CONTEXT.md` — establishes the goreleaser pipeline this phase extends. Critical decisions reused here: D-01 (minisign signing — embedded pubkey reads the same file), D-02 (`minisign.pub` at repo root — single source of truth), D-04 (INSTALL.md verification recipe — Phase 52 adds an "Upgrading" section without disturbing the install lead), D-06 (tag-triggered + RC pre-release marker — `helix upgrade --prerelease` consumes the same RC tags), D-07 (auto-generated release notes — `helix update` displays them).
- `.planning/phases/51.1-cgo-treesitter-gate-gate-internal-treesitter-behind-go-build/51.1-CONTEXT.md` — establishes the CGO=0 cross-compile path. Phase 52's binary rename and any new package paths must continue to compile under `CGO_ENABLED=0` to keep the goreleaser pipeline green.
- `.planning/phases/50-toolchain-go1.25-bench-local/50-CONTEXT.md` — Makefile self-doc style (`bench` / `bench-baseline` targets with `## help` annotations). Any Makefile additions in this phase (e.g. `release-snapshot` exists already; possibly an `upgrade-test` target for dev) follow the same self-doc style.

### CI / workflows (touched by this phase)
- `.github/workflows/release.yml` — KEEP. The renamed binary still uses the same release flow; only archive-name template strings inside `.goreleaser.yaml` change.
- `.github/workflows/go-test.yml` — KEEP. May need a new test target for the upgrade subcommand (httptest fixture for GitHub Releases API + minisign-verify with a test keypair).
- `.github/workflows/codeql.yml`, `codespell.yml`, `docker.yml`, `docs.yaml`, `junie.yml`, `pytest.yml` — KEEP, untouched.

### Code & files to create/modify

**Rename (D-01..D-05):**
- `cmd/serena/main.go` → `cmd/helix/main.go` — directory move + any path-string updates.
- `.goreleaser.yaml` — `project_name`, `builds[].id`, `builds[].binary`, `archives[].name_template` flip from `serena` to `helix`.
- `Makefile` — every `serena` reference becomes `helix`; the `release-snapshot` target stays but archive-name expectations update.
- `INSTALL.md` — every `serena` reference flips; D-04's verification recipe block uses `helix_v{{ .Version }}_*` archive names. NEW "Upgrading" subsection covering `helix update` + `helix upgrade` + `--prerelease` + `--version` + `--dry-run`.
- `README.md` — product name, install instructions, every code-block invocation. "Originally inspired by Python Serena" attribution stays for v1.9 history; can be revisited in v2.0.
- `USAGE.md`, `CONTRIBUTING.md`, `CHANGELOG.md` — every `serena` → `helix`. CHANGELOG v1.9 entry must call out the breaking renames.
- `internal/cli/*.go` — every `serena` string in user-facing output; cobra command name; setup-CLI client registration strings (`internal/cli/setup_clients.go`); status output (`internal/cli/status_output.go`).
- `internal/skill/`, `internal/profile/`, `internal/config/`, `internal/daemon/`, `internal/mcp/` — every `SERENA_*` env-var reference becomes `HELIX_*`; every `~/.serena/` / `.serena/` path becomes `~/.helix/` / `.helix/`; every log-line "serena" prefix becomes "helix"; the MCP server's registered name string updates.
- `CLAUDE.md` (root and any nested) — every `serena` reference; build command examples (`go build ./cmd/serena` → `go build ./cmd/helix`).
- `Makefile` test, build, vet targets — path updates from `./cmd/serena` to `./cmd/helix`.

**New (D-06..D-12):**
- `internal/upgrade/` (or `internal/cli/upgrade/`) — NEW package. Contains: GitHub Releases API client, archive download, minisign signature verification (consuming the embedded pubkey from D-13), atomic-swap implementation with platform branches (`upgrade_unix.go` / `upgrade_windows.go`), permission-probe helper, `os.Exec` re-launcher, daemon-detect heuristic, version comparison + downgrade refusal.
- `cmd/helix/` (post-rename) — register `update` and `upgrade` cobra subcommands wired to the new `internal/upgrade/` package.
- Test fixtures: a stub GitHub Releases API server (httptest) + a test minisign keypair so `helix upgrade --dry-run` and `helix update` are testable in CI without hitting the real GitHub API.

**Embed audit + minisign embedding (D-13..D-15):**
- `.planning/phases/52-packaging-distribution-channels/EMBED-AUDIT.md` — NEW. Living manifest of every runtime asset read, classified embedded / external-by-design / gap.
- `internal/upgrade/pubkey.go` (or wherever D-13 lands) — NEW. Single Go file with `//go:embed ../../minisign.pub` directive (or build-tag equivalent). Exports the public key bytes for the verifier.
- `minisign.pub` at repo root — NO content change required (still the placeholder until the maintainer rotates in the real key per Phase 51's DEF-51-03). Phase 52 just adds the embed reference; the placeholder still works for `helix upgrade` because `helix upgrade` itself is gated by minisign verification — if the placeholder is in the binary, no real signature can ever verify, so upgrades fail closed (which is the correct behavior until the maintainer rotates the key).

### External / upstream context (research-phase reading)
- Goreleaser docs §"Archives" — confirm the `name_template` syntax and naming-pattern variables for the rename.
- minisign upstream README — confirm the verification API for Go (whether to shell out to `minisign -V` or use a pure-Go implementation; goreleaser's signing config in Phase 51 shells to the binary).
- GitHub REST API §"Releases" — `GET /repos/{owner}/{repo}/releases/latest` for stable; `GET /repos/{owner}/{repo}/releases` for listing pre-releases; asset-download URL pattern.
- Go `os.Rename` semantics on Windows for in-place binary replacement — confirm the canonical "rename current to .old, write new" dance and whether a dedicated library handles this (e.g., `github.com/inconshreveable/go-update`).
- Go `//go:embed` constraints — confirm path-relative-to-the-go-file requirement; whether embedding from outside the module is permitted (it is not — `minisign.pub` may need to live in `internal/upgrade/` or be copied at build time).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`.goreleaser.yaml` (Phase 51)** — already produces 6 archives, signs with minisign, enforces reproducibility, runs CGO=0. The rename is mechanical: `project_name`, `builds[].binary`, and `archives[].name_template` flip. The signing config and reproducibility flags are unchanged.
- **`internal/langregistry/` three-tier LS installer** — proves the runtime-resolved-LS pattern works. The upgrade subcommand can crib its progress/error UX (the installer already has download-with-progress + verify + cache patterns; the upgrade flow is similar minus the cache).
- **`cmd/serena/main.go` cobra root** — single entrypoint with subcommands mounted from `internal/cli/`. Adding `update` and `upgrade` subcommands follows this exact pattern.
- **`internal/cli/setup*.go` family** — establishes the per-client subcommand pattern. `update` and `upgrade` are cleaner (no per-client branching) but reuse the same CLI conventions: `--json`, `--verbose`, structured exit codes.
- **`internal/skill/workflow/onboarding/`** — Phase 52 doesn't change onboarding text directly, but the rename touches every "serena" reference inside.
- **`Makefile` self-doc style (Phase 50 D-07)** — any new dev-only targets (e.g., `make upgrade-test` to exercise the upgrade flow against a local httptest server) follow this style.

### Established Patterns

- **`//go:embed` for everything not user-data** — the codebase already embeds tree-sitter grammars (23 languages), the language registry YAML, generated LSP types, and likely skill markdowns. The audit confirms the gap surface; the established pattern is: if it ships with the binary and isn't user-mutable, embed it.
- **Three-tier resolution (PATH > managed download > error)** — the LS installer's pattern. The upgrade subcommand mirrors it loosely: prefer existing-binary location, fall back to user-cache dir for stage, fail clean if neither writable.
- **Typed-error taxonomy (Phase 22-24)** — 7 error kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout). The upgrade subcommand returns typed errors at every boundary so the calling shell sees structured exit codes.
- **Tech-debt notes inline in PROJECT.md "Context" section** — the rename + self-upgrade are big enough that PROJECT.md needs a v1.9 status update; follow the Phase 50/51 inline-edit pattern.

### Integration Points

- **The MCP server registration name (in `internal/cli/setup_clients.go`)** is the most user-visible cascade point. Existing users who registered via `serena setup claude-code` will see "no MCP server" errors after upgrading until they re-run `helix setup claude-code`. The setup CLI should detect the old `serena` registration and either remove it automatically or print a clear instruction.
- **The forwarder + daemon** (`internal/forwarder/`, `internal/daemon/`) — log lines, stdio prefix, gRPC service names. Most are internal but any user-visible string needs the rename.
- **The minisign verification at `helix upgrade` time and the minisign signing at goreleaser time** consume the same key material. Phase 52 introduces no new key material — only embeds the existing `minisign.pub`.
- **Phase 51's reproducibility CI gate** — the rename causes archive-name template changes, which the gate must accommodate. Verify the gate compares archives by their post-rename names; otherwise the first post-rename release will fail the diff check.
- **The hooks installer (`internal/cli/setup_hooks.go`)** — installs `SessionStart`, `PreToolUse`, `Stop` hooks during `serena setup claude-code`. The hook commands inside `.claude/hooks/*` likely reference the binary by name — these need updating to call `helix` instead of `serena`. Since hooks are written from embedded source, this is a one-line update in the hook templates plus a re-install.

</code_context>

<specifics>
## Specific Ideas

- The user explicitly **abandoned the package-manager distribution model** (brew/scoop/native-Linux). The rationale matters for downstream agents and future planners: the user prefers a single self-contained-binary story over a fragmented multi-channel one. Do NOT propose re-introducing brew/scoop/apt in this phase or in v1.10 without explicit user approval. The dropped requirements are recorded in the deferred section below for traceability.
- The user explicitly chose `helix` as the binary name, full rename, no symlink. They preferred a clean break over a migration tool. Do NOT propose symlinks, wrapper scripts, env-var fallbacks, or `~/.serena/` migration tooling. The break IS the design.
- The user specified the upgrade implementation pattern in detail (paste from research): version-compare via embedded ldflags, atomic-swap with Unix inode trick + Windows rename-then-overwrite, `os.Exec` for restart, Ed25519 signature verification (which our minisign already uses), downgrade prevention. These are LOCKED — the planner should not propose alternative architectures (e.g., systemd-managed update, OCI-image pull, side-by-side install).
- The user picked the `apt update` / `apt upgrade` semantic split deliberately. `helix update` is read-only; `helix upgrade` installs. This is a clean affordance — keep it.
- The user picked **hard-refuse downgrade** explicitly. Do NOT propose `--force-downgrade`, `--allow-rollback`, or similar escape hatches. Users who want an older version go to GitHub Releases manually.

</specifics>

<deferred>
## Deferred Ideas

- **PKG-02 (Homebrew tap)** — explicitly dropped from v1.9 scope per user re-scope. If revisited, would consume the goreleaser archives produced by Phase 51 and could be added additively to `.goreleaser.yaml` (`brews:` section). Not a v1.9 deliverable.
- **PKG-03 (Scoop bucket)** — explicitly dropped from v1.9 scope per user re-scope. Same posture as PKG-02; goreleaser supports `scoops:` additively. Not a v1.9 deliverable.
- **PKG-04 (native Linux package: deb/rpm/AUR)** — explicitly dropped from v1.9 scope. goreleaser's `nfpms` could produce deb+rpm in-pipeline if revisited; AUR would need an external SSH-key push. Not a v1.9 deliverable.
- **PKG-DEFER-01 (second Linux package format)** — moot now that PKG-04 itself is deferred.
- **PKG-DEFER-02 (Docker / container images)** — already deferred project-wide. Reaffirmed.
- **Curl-pipe-sh installer script (`install.sh`, iwr-iex)** — considered during the re-scope and rejected. The Phase 51 manual copy-paste recipe stays as the install path; the in-binary `helix upgrade` handles upgrades.
- **`~/.serena/` migration tool / fallback shim** — considered as an alternative to the hard-cut env-var rename and rejected. Users re-onboard.
- **Embedding LS binaries** — considered during the embed-audit gray area and rejected for archive-size reasons. EMBED-AUDIT.md classifies LSes as **external-by-design**. If revisited, would be a per-LS opt-in (e.g., `helix-bundle-go` archive variant containing gopls).
- **Auto-rotation of `minisign.pub` via embedded fingerprint check at build time** — considered (defense-in-depth against accidental key swap) and deferred. Adds a build step for marginal value during a phase that's already large. Could be revisited in v1.10.
- **MCP-side automatic re-registration** (the setup CLI removing the old `serena` registration automatically) — considered and noted as Claude's discretion in D-05; planner can choose to ship it in v1.9 or defer to a v1.10 polish phase.
- **`serena` shell-completion fallback** — not raised by user but worth noting: bash/zsh/fish completions installed under the old name will need to be re-generated for `helix`. The planner can either ship a one-line CHANGELOG note or have `helix setup` re-install completions. Defer the decision to planner discretion.

### Reviewed Todos (not folded)
None — the cross_reference_todos step found no matching todos for Phase 52.

</deferred>

---

*Phase: 52-packaging-distribution-channels*
*Context gathered: 2026-04-29*
