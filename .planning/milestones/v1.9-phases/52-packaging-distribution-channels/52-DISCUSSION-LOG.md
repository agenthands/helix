# Phase 52: packaging-distribution-channels - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-29
**Phase:** 52-packaging-distribution-channels
**Areas discussed:** Phase rescope (preflight), Binary name, Upgrade mechanics, Update target & flags, Embed audit gap & minisign.pub

---

## Phase Rescope (preflight, before standard gray areas)

The original ROADMAP scope (PKG-02 Homebrew tap, PKG-03 Scoop bucket, PKG-04 native Linux package) was abandoned during the discussion. The conversation moved through three reactions:

1. Initial gray-area presentation framed packaging as separate brew-tap + scoop-bucket + native-Linux package channels (goreleaser's standard pattern).
2. User pushed back on separate-repo tap/bucket layout: "we provide archived release binaries in the same repo!" — locking single-repo packaging if the package-manager path were retained.
3. User pushed back further: "we whole abandoned packages idea" — abandoning the package-manager distribution model entirely.

The replacement scope was confirmed via a yes/no probe:

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, proceed | Embed-everything-embeddable + curl-to-sh installer + INSTALL.md refresh, replacing brew/scoop/native-Linux | |
| Yes, but LS binaries embedded too | Same as above PLUS bundle every supported LS in the archive (GB-class) | |
| Yes, but no installer script | Embed-everything-embeddable, skip the installer script, keep Phase 51 copy-paste recipe as install path | ✓ (with addendum: in-binary upgrade command) |
| Different shape | User describes scope | |

**User's choice:** "Yes, but upgrade also can be in binary helix update , helix upgrade" — selected the no-installer-script option AND added an in-binary self-upgrade subcommand.

**Notes:** The rescope inverts REQUIREMENTS.md's "Auto-update mechanism inside the binary | Out of scope" row because the original rationale ("package managers own updates") no longer applies — there are no package managers in v1.9. PKG-02/03/04 are recorded as deferred ideas with single-repo packaging noted as the path forward if any of them are ever revived.

---

## Binary name

### Q1: What should the installed CLI be called?

| Option | Description | Selected |
|--------|-------------|----------|
| serena (no change) | Keep `serena` everywhere; lowest blast radius | |
| helix (rename) | Rename to `helix`; cascades to docs, Makefile, INSTALL.md, goreleaser config, MCP registration, env vars | ✓ |
| Both (alias) | Ship `helix` as canonical, `serena` symlink for back-compat | |
| You decide | Claude picks | |

**User's choice:** helix (rename)

### Q2: Does the rename cascade to env vars, config dirs, and project marker dirs?

| Option | Description | Selected |
|--------|-------------|----------|
| Full rename everywhere | `SERENA_*` → `HELIX_*`; `~/.serena/` → `~/.helix/`; `.serena/project.yml` → `.helix/project.yml`; no migration | ✓ |
| Binary only, keep paths | Binary becomes `helix` but config + env vars stay under `serena` namespace | |
| Full rename + auto-migration | Full rename plus a transparent fallback that detects old paths for one release | |

**User's choice:** Full rename everywhere

### Q3: What is the product name now?

| Option | Description | Selected |
|--------|-------------|----------|
| Helix | Product is renamed end-to-end; PROJECT.md, README, all docs say "Helix" | ✓ |
| Serena (product) / helix (CLI) | Product stays Serena; the CLI is just the binary's typing convenience | |
| You decide | Claude picks | |

**User's choice:** Helix

### Q4: Old `serena` binary backwards-compat — ship a symlink for one release?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard cut, no symlink | Archive only contains `helix`; users with scripts get clean failure; documented in CHANGELOG | ✓ |
| Ship `serena` as symlink for v1.9 | Symlink in archive; deprecation nudge; remove in v1.10 | |
| Ship `serena` as wrapper that prints + exec's | Tiny wrapper that nudges then exec's `helix` | |

**User's choice:** Hard cut, no symlink

---

## Upgrade mechanics

### Q1: How does `helix upgrade` swap the running binary in place?

| Option | Description | Selected |
|--------|-------------|----------|
| Atomic rename (download + os.Rename) | Download + verify + extract + os.Rename; Unix inode-replace; Windows rename-current-to-.old dance | (effectively chosen) |
| Self-exec relauncher | Stage `helix.new` and let an embedded shim swap after parent exits | |
| Print instructions, don't auto-swap | Stage verified file and tell user to mv it themselves | |
| You decide | Claude picks | |

**User's choice:** Pasted research describing the standard self-upgrade workflow: version-compare via ldflags, download + Ed25519 signature verify, atomic swap (Unix inode-replace; Windows rename current to .old), restart via `os.Exec`, downgrade prevention, permission handling. This effectively maps to the "atomic rename" option with the Windows-specific subbranch.

**Notes:** The pasted research is locked verbatim as the implementation blueprint. Decisions D-07/D-08 in CONTEXT.md reflect each step.

### Q2: `update` vs `upgrade` — same command or different?

| Option | Description | Selected |
|--------|-------------|----------|
| Aliases (both work) | Single subcommand, two cobra aliases | |
| Only `upgrade` | One canonical verb, `update` not registered | |
| `update` checks, `upgrade` installs | apt-style split: `update` is read-only, `upgrade` installs | ✓ |

**User's choice:** `update` checks, `upgrade` installs

### Q3: After successful swap, what does the binary do?

| Option | Description | Selected |
|--------|-------------|----------|
| os.Exec same args | Replace the running process image with the new binary, same args minus `upgrade` | ✓ |
| Print success, exit | Swap completes, print success message, user re-invokes manually | |
| You decide | Claude picks | |

**User's choice:** os.Exec same args

### Q4: What happens if the install path needs root (e.g. /usr/local/bin/helix)?

| Option | Description | Selected |
|--------|-------------|----------|
| Detect early, print sudo hint | Check write access before download; print sudo hint and exit clean | ✓ |
| Download first, then prompt at swap | Download + verify before checking permissions; prompt with staged path on EACCES | |
| Stage to user-writable, instruct user | Always stage to `~/.cache/helix/upgrade-stage/`; never touch privileged paths internally | |

**User's choice:** Detect early, print sudo hint

### Q5: Downgrade behavior — refuse, warn, or require flag?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard refuse | If remote ≤ current, exit 0 with "already up to date" message; no override flag | ✓ |
| Require --force-downgrade | Default refuses; explicit flag allows pinning older version | |
| Warn + proceed | Most permissive; proceeds with a warning | |

**User's choice:** Hard refuse

---

## Update target & flags

### Q1: What does `helix upgrade` target by default (no flags)?

| Option | Description | Selected |
|--------|-------------|----------|
| Latest stable only | Skips `v*-rc*`/`-beta*`/`-alpha*`; pre-releases via opt-in flag | ✓ |
| Latest of any kind | Default takes literally the newest tag including pre-releases | |
| You decide | Claude picks | |

**User's choice:** Latest stable only

### Q2: Beyond default, which flags do we ship in v1.9?

| Option | Description | Selected |
|--------|-------------|----------|
| --prerelease | Opt into latest pre-release | ✓ |
| --version vX.Y.Z | Pin to specific tag; forward-only per hard-refuse-downgrade | ✓ |
| --check (or that's what `update` is for) | Read-only on `upgrade`; redundant with `update` but provided for muscle memory | ✓ |
| --dry-run | Run check + download + verify + skip swap | ✓ |

**User's choice:** All four selected — `--prerelease`, `--version vX.Y.Z`, `--check`, `--dry-run`.

### Q3: What does `helix update` (check-only) print?

| Option | Description | Selected |
|--------|-------------|----------|
| Versions only | Two lines: current + latest | |
| Versions + release notes summary | Versions plus the GitHub Release body (auto-generated by goreleaser per Phase 51 D-07) | ✓ |
| Versions + diff URL | Versions plus a clickable Releases-tag URL | |

**User's choice:** Versions + release notes summary

### Q4: If `--prerelease` and `--version` are both set, what wins?

| Option | Description | Selected |
|--------|-------------|----------|
| --version always wins | Explicit pin overrides channel; `--version v1.9.2-rc1` works regardless of `--prerelease` | ✓ |
| --version + --prerelease incompatible | CLI errors if both set | |
| You decide | Claude picks | |

**User's choice:** --version always wins

---

## Embed audit gap & minisign.pub

### Q1: Embedding `minisign.pub` for self-verify — yes, but how?

| Option | Description | Selected |
|--------|-------------|----------|
| //go:embed minisign.pub | Single source of truth — same file consumed by INSTALL.md curl-fetch and self-verify | ✓ |
| Hardcoded const string | Bake key into source as a const | |
| Both: const + embed verification | Embed plus build-time fingerprint check; defense in depth | |

**User's choice:** //go:embed minisign.pub

### Q2: Beyond minisign.pub, what's the embed audit scope for this phase?

| Option | Description | Selected |
|--------|-------------|----------|
| Audit + fix actual gaps | Discovery task drives plan content; produces a concrete list | ✓ |
| Targeted: skill markdowns, profile YAMLs, hook scripts | Pre-list suspects without an audit | |
| Just minisign.pub, defer the rest | Phase 52 only embeds the pubkey; defer further audit | |
| You decide | Claude picks | |

**User's choice:** Audit + fix actual gaps

### Q3: LS binaries (gopls, jdtls, rust-analyzer) — confirm: stay runtime-downloaded?

| Option | Description | Selected |
|--------|-------------|----------|
| Stay runtime-downloaded | Three-tier installer keeps resolving LSes at runtime; archive size stays bounded | ✓ |
| Embed a curated subset | Embed top 3-5 most common LSes; adds 100-300MB | |
| Embed everything | Bundle all 52 LSes; multi-GB archives; ruled out for practicality | |

**User's choice:** Stay runtime-downloaded

### Q4: Does the audit produce a written manifest, or is it a one-shot grep in-plan?

| Option | Description | Selected |
|--------|-------------|----------|
| Written manifest (EMBED-AUDIT.md) | Living document classifying every runtime asset read; checked by future PRs | ✓ |
| One-shot grep, fix in plan | Findings go into plan SUMMARY.md, no standalone artifact | |
| You decide | Claude picks | |

**User's choice:** Written manifest (EMBED-AUDIT.md)

---

## Claude's Discretion

The user explicitly deferred these to Claude/planner judgment (recorded inline in CONTEXT.md):

- Exact stage-dir path for upgrade downloads (`os.TempDir()` vs. `os.UserCacheDir()/helix/upgrade-stage/` vs. next-to-binary).
- Exact GitHub Releases API endpoint pattern (`/latest` vs. `/tags` vs. `/releases`).
- Daemon-detection heuristic for the "do not exec, print manual hint" branch.
- Wording of error messages and the sudo hint string.
- Location of the `//go:embed minisign.pub` Go file (`internal/upgrade/`, `internal/release/`, etc.).
- Exit codes for `--dry-run` success vs. failure.
- Whether `helix update` and `helix upgrade --check` produce identical or slightly different output.
- Whether the rename is one atomic plan or staged across multiple plans.
- Exact text for the "previous `serena` MCP registration detected" nudge in setup CLI (or whether to ship it at all in v1.9).
- Whether to ship shell-completion regeneration or just document it in CHANGELOG.

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section. Highlights:
- PKG-02 / PKG-03 / PKG-04 (brew, scoop, native Linux) — explicitly dropped from v1.9; reusable on top of goreleaser if ever revived.
- Curl-pipe-sh installer script — rejected.
- `~/.serena/` migration tool — rejected (hard cut chosen).
- Embedding LS binaries — rejected (archive size).
- Build-time minisign fingerprint check — deferred to v1.10 polish.
- Auto-removal of old `serena` MCP registration — Claude's discretion.
- Shell-completion regeneration — Claude's discretion.
