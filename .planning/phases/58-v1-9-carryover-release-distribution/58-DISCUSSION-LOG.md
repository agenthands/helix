# Phase 58: v1.9 Carryover — Release & Distribution - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-03
**Phase:** 58-v1-9-carryover-release-distribution
**Areas discussed:** Distribution scope (meta), REL-01 signing infra, REL-05 repro gate, REL-06 forwarder span, won't-do recording, Cosign migration staging, Cosign offline-upgrade behavior

---

## Distribution scope (meta — premise question raised by user)

User pushed back on the inherited phase scope: *"WTF is going on?? We doesn't ship the packages for any OS, just self-contained binary exe."*

The push-back was accurate — REL-02/03/04 (Homebrew tap, Scoop bucket, native Linux pkg) were rolled forward from v1.9 PKG-DEFER-03/04/05 without a real product re-evaluation. PROJECT.md and CLAUDE.md both describe Helix as a single self-contained binary.

| Option | Description | Selected |
|--------|-------------|----------|
| Drop REL-02/03/04 entirely | Single self-contained binary stays the only distribution channel. Phase 58 shrinks to REL-01/05/06. Recommended given product positioning. | ✓ |
| Drop REL-02/03 (Brew/Scoop), keep REL-04 (Linux pkg) | Self-installing daemon on Linux servers/CI is a real use case. Phase 58 keeps REL-01/04/05/06. | |
| Keep all REL-02/03/04 but downscope | Cheapest form: auto-bumped Homebrew tap, auto-bumped Scoop manifest, .deb only. No daemon-registration plumbing. ~2 plans of work. | |
| Keep all six REL items as-is | Full inherited roadmap scope. | |

**User's choice:** Drop REL-02/03/04 entirely.
**Notes:** This is a structural scope reduction, not a delivery deferral — REL-02/03/04 are recorded as won't-do in REQUIREMENTS.md with rationale. Reopens only on real user-demand signal.

---

## REL-01 — Minisign keypair & CI signing access

| Option | Description | Selected |
|--------|-------------|----------|
| GitHub Actions secrets (MINISIGN_PRIVATE_KEY + passphrase) | Workflow writes key to temp file, signs, deletes. Pubkey lives in 2 places via single canonical source. Rotation: manual on compromise. Recommended. | |
| Sigstore cosign keyless signing | Replace minisign with cosign OIDC-keyless. Requires rewriting `internal/upgrade/` verifier. Larger change, modern threat model. | ✓ |
| External secret manager (1Password/Vault) | Pull key from external secret manager via service-account creds. Adds external dependency. | |

**User's choice:** Sigstore cosign keyless signing.
**Notes:** Significantly larger blast radius than the original "rotate the placeholder pubkey" framing — also rewrites `internal/upgrade/` verification path, removes both `minisign.pub` files, makes `release.yml` PLACEHOLDER pre-flight grep obsolete. Surfaced explicitly to user before locking; user confirmed via follow-up questions.

---

## REL-05 — Reproducibility gate

| Option | Description | Selected |
|--------|-------------|----------|
| Document the Pass-3 limitation in CONTRIBUTING.md | v1.9 audit already accepted this. Zero ongoing maintenance. Recommended. | ✓ |
| Add a real-artifact comparison job | Download released binary, re-run Pass-3, byte-compare. Real verification value, ongoing toil. | |
| Hybrid (doc now, comparison job after v1.10.0) | Defer comparison job to v1.11+ once snapshot-vs-snapshot fires often enough to need it. | |

**User's choice:** Document the Pass-3 limitation in CONTRIBUTING.md.
**Notes:** Matches v1.9 milestone audit decision. Comparison-job option preserved in CONTEXT.md deferred-ideas section for v1.11+ revisit if needed.

---

## REL-06 — Forwarder span unification

| Option | Description | Selected |
|--------|-------------|----------|
| Add `otelgrpc.NewClientHandler` to forwarder's gRPC dial | OTel context propagates as gRPC metadata into server span. Span tree: stdio → forwarder.tools.call (client) → ForwarderService/StreamMCP (server) → daemon. Recommended — actually unified. | ✓ |
| Document multi-process limitation, drop unification goal | Acknowledge two separate trace roots. Update PROJECT.md tech-debt to remove the carryover commitment. | |
| Restructure so daemon emits parent span, forwarder is child | Inverts current orientation. stdio MCP has no inbound trace context, so this only makes sense if forwarder synthesizes one — effectively the same as option 1. | |

**User's choice:** Add `otelgrpc.NewClientHandler`.
**Notes:** End-to-end trace assertion lives in an integration test (mocked OTel exporter or in-memory span recorder). Server-side handler is presumed installed already; planner verifies before adding client-side.

---

## Won't-do tracking (recording REL-02/03/04 rejection)

| Option | Description | Selected |
|--------|-------------|----------|
| Update v1.10-ROADMAP.md + REQUIREMENTS.md with `[~]` won't-do marker + rationale | Phase 58 entry shrinks. REQUIREMENTS.md preserves audit trail of "considered, rejected with reason." Recommended. | ✓ |
| Delete REL-02/03/04 lines entirely | Cleaner roadmap, loses audit trail. | |
| Move to a backlog file | Captures "may revisit if demand emerges." Adds a backlog file the project doesn't currently use. | |

**User's choice:** Update v1.10-ROADMAP.md + REQUIREMENTS.md with won't-do marker.
**Notes:** Preserves the explicit "we considered package channels and decided against them" record so future maintainers don't re-relitigate. Rationale string is fixed (see CONTEXT.md `<specifics>`).

---

## Cosign migration staging (follow-up to REL-01 cosign choice)

| Option | Description | Selected |
|--------|-------------|----------|
| Hard cut — cosign-only at v1.10.0 | Clean. Breaks self-upgrade across the v1.9.x → v1.10.0 boundary, but v1.9.0 was never actually published. Defensible. | ✓ |
| Sign both with minisign AND cosign during v1.10.x; drop minisign in v1.11.0 | Maintenance toil for one cycle without real benefit. | |
| Backport cosign verification to v1.9.x first, then cut v1.10.0 cosign-only | Most correct for project with real v1.9.x users; adds backport step that doesn't currently exist in workflow. | |

**User's choice:** Hard cut at v1.10.0.
**Notes:** Acceptable specifically because no real v1.9.x with a real signature was ever shipped — minisign.pub stayed PLACEHOLDER through v1.9 close. Anyone running development v1.9.x downloads v1.10.0 manually once.

---

## Cosign offline-upgrade behavior (follow-up to REL-01 cosign choice)

| Option | Description | Selected |
|--------|-------------|----------|
| Require online Rekor; error clearly if unreachable | Default cosign-go behavior. Self-upgrade is inherently online. Recommended. | ✓ |
| Use cached Rekor inclusion proofs (TUF-style metadata bundle) | Bundle recent inclusion-proof set in release artifact; verify offline. Adds release-time complexity. | |
| Skip Rekor verification — verify only against Fulcio CA | Drops transparency-log guarantee. Faster, weaker than typical cosign threat model. | |

**User's choice:** Require online Rekor; error clearly if unreachable.
**Notes:** Error message wording matters — must explicitly state "Rekor transparency-log verification requires network access" so it's distinguishable from generic network errors.

---

## Claude's Discretion

- Whether the cosign migration is one plan or two (split between `goreleaser.yaml` swap and verifier rewrite vs combined). Planner decides based on commit granularity and worktree-isolation safety.
- Test strategy for the Rekor-online requirement: real Rekor calls in integration tests (network-dependent) vs mocked Rekor responses. Default to mocked unless real-call is genuinely cheap.
- Whether the won't-do recording for REL-02/03/04 lands as its own plan or as a tail-end task in the cosign plan. Cosmetic; either is fine.

## Deferred Ideas

- Real-artifact reproducibility comparison job → v1.11+ revisit if snapshot-vs-snapshot gate proves insufficient.
- Backporting cosign verification to v1.9.x → revisit only if real users on v1.9.x emerge before v1.10.0 ships.
- Distribution channels (Homebrew, Scoop, Linux pkg) → reopen only on real user-demand signal.
- Cached Rekor inclusion proofs for offline upgrade → reopen only if concrete air-gapped use case emerges.
- PROJECT.md "Resolved at v1.10" entry → belongs in post-phase `update_project_md` step, not in any plan.
