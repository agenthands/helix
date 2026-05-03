# Phase 58: v1.9 Carryover — Release & Distribution - Context

**Gathered:** 2026-05-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Cut the first real signed Helix release, close the v1.9 deployment-gated and architectural-debt items, **and explicitly de-scope distribution channels (Homebrew tap, Scoop bucket, native Linux package) that were rolled forward from v1.9 deferral without a real product decision.** Phase ships in parallel with the early semantic phases (57–60) so release risk does not concentrate near the v1.10 ship date.

**In scope:**
- REL-01 — Real signed v1.10.0 release **using sigstore cosign keyless** (replaces minisign approach inherited from v1.9)
- REL-05 — Reproducibility-gate Pass-3 limitation documented in `CONTRIBUTING.md`
- REL-06 — `forwarder.tools.call` span unified with the gRPC server span via `otelgrpc.NewClientHandler`
- Roadmap + REQUIREMENTS.md update marking REL-02/03/04 as won't-do with rationale

**Out of scope (de-scoped from inherited Phase 58):**
- ~~REL-02 — Homebrew tap~~ → won't-do
- ~~REL-03 — Scoop bucket~~ → won't-do
- ~~REL-04 — Native Linux package (.deb/.rpm)~~ → won't-do
- Anything in Phases 59–67 (semantic-index work — runs in parallel)

</domain>

<decisions>
## Implementation Decisions

### Distribution scope (meta-decision, drove the rest)

- **D-01:** **Drop REL-02/03/04 entirely.** Helix ships exclusively as a single self-contained signed binary. Package channels (Homebrew, Scoop, native Linux pkg) are not pursued in v1.10 or beyond unless real user demand emerges.
  - **Rationale:** PROJECT.md and CLAUDE.md both describe Helix as "single Go binary with no Python, Docker, or runtime dependencies." Audience is coding agents (Claude Code, Codex, Gemini CLI, etc.), not end-users browsing taps. PKG-DEFER-03/04/05 were tagged "deferred" at v1.9 close because the team had not yet decided whether they were worth doing — v1.10 inheriting them as REL-02/03/04 was calendar roll-forward, not a real commitment. Each channel adds maintenance burden (formula curation, manifest updates, distro signing nuances) without reaching the actual audience.
  - **Recording mechanism:** `.planning/milestones/v1.10-ROADMAP.md` Phase 58 entry shrinks to REL-01/05/06 only. `.planning/REQUIREMENTS.md` lines for REL-02/03/04 change from `- [ ]` to `- [~]` with the rationale string `won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience`. Phase 58 SC-2 and SC-3 in the roadmap are removed.

### REL-01 — Signing infrastructure

- **D-02:** **Replace minisign with sigstore cosign keyless signing.** No long-lived private key. CI uses GitHub Actions OIDC → Fulcio short-lived certificate → cosign sign-blob → Rekor transparency-log entry. `goreleaser.yaml` `signs:` block swaps from minisign to cosign.
  - **Rationale chosen by user:** modern keyless flow eliminates the "where do we store the private key" question entirely; matches sigstore's threat model with transparency-log inclusion.
  - **Larger blast radius than the original REL-01 framing:** also rewrites `internal/upgrade/` verification path, removes both `minisign.pub` files, makes `release.yml`'s PLACEHOLDER pre-flight grep obsolete. Planner must scope this as a verifier rewrite, not a key rotation.

- **D-03:** **Hard cut at v1.10.0 — cosign-only, no minisign coexistence.**
  - **Rationale:** v1.9.0 was never actually published with a real minisign signature (the keypair stayed PLACEHOLDER through v1.9 close), so there is no shipped v1.9.x binary in the wild that would lose its self-upgrade path. Co-signing or backporting cosign verification to v1.9.x is unnecessary maintenance toil.
  - **Implication:** anyone running a development v1.9.x binary needs to download v1.10.0 manually once. Acceptable — PROJECT.md tech-debt section already records that v1.9.0 was deployment-gated.

- **D-04:** **`helix upgrade` requires online Rekor for transparency-log inclusion-proof verification.** Default cosign-go client behavior. If Rekor is unreachable, return a structured error explaining the network requirement; do not fall back to weaker verification.
  - **Rationale:** self-upgrade is inherently online — if the user can fetch the binary they can fetch a Rekor entry. Bundling cached inclusion proofs adds release-time complexity without materially helping a flow that already needs network access.

### REL-05 — Reproducibility gate

- **D-05:** **Document the Pass-3 limitation in `CONTRIBUTING.md`. No real-artifact comparison job.**
  - **Rationale:** the v1.9 milestone audit explicitly accepted "snapshot-vs-snapshot, not against published release artifacts" as a documented limitation; this Phase 58 decision finalizes that position. Adding a real-artifact comparison job commits the project to ongoing toil (regenerating expected hashes per release) for verification value the audit already deemed acceptable.
  - **Form:** new short paragraph in `CONTRIBUTING.md` under whichever section currently covers reproducibility (or a new `## Reproducibility` heading if none exists), explicitly stating the gate compares Pass-1 vs Pass-2 vs Pass-3 hashes within the same source revision and does NOT diff against published release artifacts. Rationale must include the trade-off so a future maintainer can flip the decision.

### REL-06 — Forwarder span unification

- **D-06:** **Add `otelgrpc.NewClientHandler` to the forwarder's gRPC dial; emit a real `forwarder.tools.call` client span.** OpenTelemetry context propagates as gRPC metadata into the server-side `serena.v1.ForwarderService/StreamMCP` span automatically (server-side handler is already installed).
  - **Resulting span tree:** `stdio entrypoint → forwarder.tools.call (client) → serena.v1.ForwarderService/StreamMCP (server) → daemon work`
  - **Verification:** end-to-end trace assertion in an integration test — exporting via OTLP-stdout or in-memory exporter, asserting that a single trace ID covers stdio → forwarder → daemon → kernel.
  - **Closes the architectural caveat from PROJECT.md tech-debt section** ("Phase 55 forwarder.tools.call span emits via Noop tracer — root span is `serena.v1.ForwarderService/StreamMCP` gRPC server span"). The PROJECT.md entry must be moved out of tech-debt and into the v1.10 progress paragraph after the phase ships.

### Claude's Discretion

- Whether the cosign migration is one plan or two (split between `goreleaser.yaml` swap + verifier rewrite vs combined). Planner decides based on commit granularity and worktree-isolation safety.
- Test strategy for the Rekor-online requirement: real Rekor calls in integration tests (network-dependent) vs mocked Rekor responses. Default to mocked unless real-call is genuinely cheap.
- Whether the `won't-do` recording for REL-02/03/04 lands as its own plan or as a tail-end task in the cosign plan. Cosmetic; either is fine.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### v1.10 milestone scope and current state
- `.planning/milestones/v1.10-ROADMAP.md` §"Phase 58" — original phase scope (REL-01..06) before the won't-do decision in this CONTEXT.md
- `.planning/REQUIREMENTS.md` lines 121–126 — REL-01..REL-06 definitions and v1.9-origin tags (PKG-01 SC-3, PKG-DEFER-03/04/05, Phase 51 fix, Phase 55 follow-up)
- `.planning/PROJECT.md` "Tech debt accepted at v1.9 close" — current statements of the three architectural debts being closed (PKG-01 SC-3 deployment-gate, Phase 51 reproducibility-gate Pass-3, Phase 55 forwarder.tools.call Noop tracer)
- `.planning/milestones/v1.9-MILESTONE-AUDIT.md` — v1.9 audit that flagged each of these as deferred / deployment-gated; rationale that informs the won't-do decision

### Existing release infrastructure (REL-01 baseline)
- `.goreleaser.yaml` — current goreleaser config with minisign `signs:` block; cosign migration replaces this block
- `.github/workflows/release.yml` — current release workflow; PLACEHOLDER pre-flight grep becomes obsolete after cosign migration
- `internal/upgrade/` (whole directory) — current minisign verification path; rewrite to cosign-go verifier
- `minisign.pub` (root) — PLACEHOLDER pubkey; deleted after cosign migration
- `internal/upgrade/minisign.pub` — PLACEHOLDER pubkey embedded in binary; deleted after cosign migration

### Forwarder tracing (REL-06 baseline)
- `internal/forwarder/forwarder.go` — gRPC dial site that needs `otelgrpc.NewClientHandler`; current Noop tracer location
- `internal/forwarder/forwarder_test.go` — existing forwarder tests; integration trace assertion lands here or alongside
- `api/proto/serena/v1/` — gRPC service definition; server-side OTel handler already installed in daemon (verify before adding client-side)

### Reproducibility gate (REL-05 baseline)
- `CONTRIBUTING.md` — target file for the Pass-3 limitation paragraph
- v1.9 Phase 51 SUMMARY (if archived) — original snapshot-vs-snapshot rationale; cite if the new paragraph references prior context

### External docs (cosign keyless)
- https://docs.sigstore.dev/cosign/signing/signing_with_blobs/ — cosign sign-blob workflow
- https://docs.sigstore.dev/cosign/verifying/verify/ — verification API including Rekor inclusion-proof requirements
- https://docs.sigstore.dev/cosign/signing/overview/#keyless-signing — keyless OIDC flow with GitHub Actions
- https://github.com/sigstore/cosign-go (or `github.com/sigstore/cosign/v2/pkg/...`) — Go API surface for the verifier rewrite
- https://github.com/goreleaser/goreleaser/blob/main/www/docs/customization/sign.md — goreleaser cosign integration

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/upgrade/` — atomic-swap upgrade flow already exists from v1.9 Phase 52; only the verification step needs a tool swap (minisign → cosign), not the swap mechanics
- `internal/forwarder/` gRPC dial site — `otelgrpc.NewClientHandler` is a one-line additive change at the dial; OTel propagator config likely already present (check before assuming)
- `.goreleaser.yaml` — multi-arch archive matrix (6 archives) is intact; only the `signs:` block changes

### Established Patterns
- v1.9 closed with a clear "deployment-gated" pattern for items that need a maintainer side-step (real keypair, real release tag) — Phase 58 closes this gate by removing the side-step entirely (cosign keyless eliminates the keypair-management dependency)
- PROJECT.md tech-debt section is the canonical place where unresolved architectural caveats live; this phase removes 3 entries from it

### Integration Points
- `release.yml` workflow → `goreleaser` invocation: cosign signing happens here via OIDC. Workflow needs `id-token: write` permission grant and the cosign installer step
- `helix upgrade` runtime → `internal/upgrade/` verification: the rewritten verifier is the integration boundary; existing self-upgrade flow (download → verify → atomic swap) keeps the same shape with a different verify step
- daemon bootstrap (`internal/daemon/daemon.go`) → forwarder client: forwarder is a separate process so OTel context propagates over the wire via gRPC metadata, not in-process — server-side handler must already exist for the chain to unify

</code_context>

<specifics>
## Specific Ideas

- The REL-02/03/04 won't-do recording uses the literal status marker `- [~]` in REQUIREMENTS.md (an explicit "rejected" state distinct from `- [ ]` pending and `- [x]` complete). The rationale string is fixed: `won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience`.
- The `CONTRIBUTING.md` reproducibility paragraph must explicitly use the phrase "Pass-3 limitation" so future grep finds it; future maintainers reading the v1.9 audit will look for that exact term.
- Cosign hard-cut at v1.10.0 is acceptable specifically because v1.9.0 was never actually published with a real signature — v1.9 PROJECT.md tech-debt entry confirms this. Planner should not invent a backport step.
- Rekor-required `helix upgrade`: error message wording matters. Should explicitly state "Rekor transparency-log verification requires network access" so users understand it's not a generic network error.

</specifics>

<deferred>
## Deferred Ideas

- **Real-artifact reproducibility comparison job** — deferred to v1.11+. If post-v1.10.0 we find the snapshot-vs-snapshot gate fires often (false positives or false negatives), revisit. CONTRIBUTING.md paragraph should leave this door open: "if release-artifact divergence becomes a concern, a comparison job can be added."
- **Backporting cosign verification to v1.9.x** — explicitly rejected (D-03). If real users on v1.9.x emerge before v1.10.0 ships, revisit; until then, hard cut.
- **Distribution channels (Homebrew/Scoop/Linux pkg)** — explicitly rejected (D-01). Reopen if user-demand signal emerges (e.g., issues asking for `brew install`); rationale captured in REQUIREMENTS.md so future maintainers see the prior decision.
- **Cached Rekor inclusion proofs for offline upgrade** — rejected for v1.10 (D-04). Reopen if a concrete air-gapped use case emerges.
- **PROJECT.md "Resolved at v1.10" section** — once Phase 58 ships, the three closed tech-debt entries should move from the "Tech debt accepted at v1.9 close" paragraph to a new "Resolved at v1.10" entry parallel to the existing "Resolved at v1.9" line. Belongs in the post-phase `update_project_md` step, not the plan itself.

</deferred>

---

*Phase: 58-v1-9-carryover-release-distribution*
*Context gathered: 2026-05-03*
