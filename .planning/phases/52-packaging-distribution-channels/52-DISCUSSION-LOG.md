# Phase 52: packaging-distribution-channels - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-26
**Phase:** 52-packaging-distribution-channels
**Areas discussed:** Repo strategy, Linux package format, Auto-update credentials, Install verification

---

## Repo strategy

### Top-level layout

| Option | Description | Selected |
|--------|-------------|----------|
| Separate repos | `postfix/homebrew-serena` + `postfix/scoop-serena` (goreleaser default; idiomatic; brew/scoop autodiscover by name) | |
| Single monorepo | `postfix/serena-packages` with subdirs; one token, one place to look; slightly longer install command | ✓ |
| In-repo | Tap+bucket inside the main `serena` repo; no new repos but unconventional | |

**User's choice:** Single monorepo
**Follow-up — repo name:** `postfix/serena-packages` chosen over `homebrew-serena` and `serena-dist`. Rationale: the `homebrew-` prefix is misleading for the Scoop bucket and any future package metadata; `serena-packages` is generic and future-proof.

---

## Linux package format

### Format(s) to ship

| Option | Description | Selected |
|--------|-------------|----------|
| deb + rpm via goreleaser nfpms | Both formats free from the existing pipeline; covers Debian/Ubuntu and Fedora/RHEL | ✓ |
| deb only + apt repo on GH Pages | Better UX (apt update) but adds GH Pages + GPG signing infra; rpm users get nothing | |
| AUR via PKGBUILD publisher | Distinct ecosystem; smaller user base; needs AUR account + SSH key | |
| All three (deb + rpm + AUR) | Maximum coverage but exceeds PKG-04's "at least one" bar; AUR maintenance burden | |

**User's choice:** deb + rpm via goreleaser nfpms

### Distribution method

| Option | Description | Selected |
|--------|-------------|----------|
| Direct download from GH Releases | Simplest; manual updates; INSTALL.md documents the wget+install one-liner | ✓ |
| Direct download + cosign verify step | Same one-liner, plus the Phase 51 cosign verify command; slightly longer instructions | |
| Hosted apt + dnf repo | `apt update` UX; adds GPG/repo metadata infra; Cloudsmith free tier or self-hosted GH Pages | |

**User's choice:** Direct download from GH Releases (deferred apt/dnf repo to PKG-DEFER-01)

---

## Auto-update credentials

### Authentication mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Fine-grained PAT, repo-scoped | Scoped to only `postfix/serena-packages` with `contents: write`; one secret to rotate | ✓ |
| Classic PAT (`repo` scope) | Larger blast radius; discouraged by GitHub | |
| GitHub App with installation token | Short-lived tokens, audit trail, no owner binding; ~30 min one-time setup | |

**User's choice:** Fine-grained PAT — stored as `PACKAGES_PAT` GitHub Actions secret. App-based approach revisited if Serena ever accepts co-maintainers.

### Update delivery mode

| Option | Description | Selected |
|--------|-------------|----------|
| Direct push | Goreleaser commits straight to the default branch; users `brew upgrade` immediately after tag | ✓ |
| PR-based | Goreleaser opens an auto-mergeable PR; adds a manual step per release | |

**User's choice:** Direct push — bad releases mitigated by rolling forward with a patch tag.

---

## Install verification

### How to prove install actually works post-release

| Option | Description | Selected |
|--------|-------------|----------|
| Manual checklist in RELEASING.md | Maintainer runs install commands locally; zero CI cost; gates on discipline | |
| Dedicated CI matrix on tag | Full matrix (macOS+Windows+Linux containers); ~10 min CI per release; full automation | |
| Both (CI + manual fallback) | Belt-and-suspenders; modest duplication | |
| Lightweight smoke only | One job per channel: `install + serena --version`; ~2 min CI; catches 90% failure mode | ✓ |

**User's choice:** Smoke test only — CI matrix runs `install + serena --version` per channel and asserts the version string matches the tag. On failure: opens a GitHub issue.

---

## Claude's Discretion

- Exact goreleaser v2 block layout for `brews:` / `scoops:` / `nfpms:` — follow goreleaser docs and Phase 51's existing style.
- INSTALL.md section order — extend Phase 51's structure; same heading style.
- RELEASING.md additions — extend the existing maintainer ritual with `serena-packages` bootstrap + PAT rotation note.
- CI matrix runner versions, container images, timeout values — pick reasonable defaults.

## Deferred Ideas

- Hosted apt + dnf repos (Cloudsmith / self-hosted) — PKG-DEFER-01.
- AUR PKGBUILD publisher — PKG-DEFER-01.
- GitHub App for releases — revisit at multi-maintainer scale.
- Deeper functional CI verification (`serena setup …` post-install) — out of scope for PKG-04's bar.
- PR-based formula update flow — rejected in favor of direct-push speed.
- Docker / container images — already deferred project-wide to PKG-DEFER-02.
