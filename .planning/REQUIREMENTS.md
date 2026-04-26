# Requirements: Serena v1.9 Polish & Infra

**Defined:** 2026-04-24
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## Milestone v1.9 Requirements

Requirements for v1.9 (Polish & Infra). Each maps to exactly one roadmap phase during `/gsd-plan-milestone-gaps` or `/gsd-plan-phase`.

### Bug Fixes & Polish

- [ ] **BUG-01**: `get_repo_map` returns Go sources (not the Lua fixture) when invoked on a Go workspace that contains polyglot testdata fixtures — closes backlog Phase 999.1
- [ ] **BUG-02**: `rename_symbol` succeeds against a Rust symbol in a temp workspace via rust-analyzer (or a documented, tool-level workaround is shipped if the upstream bug cannot be closed in-milestone)
- [ ] **BUG-03**: Java integration tests run without `-short=false` by reusing a warm jdtls workspace across test runs (target: the Java integration suite is green in default `go test ./...`)
- [ ] **BUG-04**: A single canonical `GrammarRegistry` instance is constructed at daemon bootstrap and shared across repomap, tagcache, and grammar consumers — the two redundant instances are removed without behavioral regression

### Infra — Toolchain

- [ ] **TOOL-01**: Serena builds, tests, and benches green on `ubuntu-latest` with Go 1.25 and a compatible gopls version — the root-cause of the gopls v0.17.1 linux/amd64 incompatibility is resolved (upgrade, patch, or replacement strategy)
- [x] ~~**TOOL-02**~~: ~~The CI benchmark gate runs on `ubuntu-latest` (not just darwin/arm64 locally) and enforces PR-tier thresholds on every PR — closes the benchmarks tech-debt note in PROJECT.md~~ — Cancelled in Phase 50 (CI-hosted bench gate proven unreliable on shared-CPU GitHub runners across two failed capture attempts; benchmarks now run locally pre-release. See `.planning/phases/50-toolchain-go1.25-gopls-ci/50-CONTEXT.md` "Pivot 2026-04-25").

### Packaging & Distribution

- [ ] **PKG-01**: GitHub Releases publish multi-arch binaries (darwin/linux/windows × amd64/arm64) with SHA-256 checksums and cryptographic signatures (cosign or minisign) via a reproducible goreleaser pipeline
- [~] **PKG-02**: ~~Homebrew tap install~~ — Removed 2026-04-26 (Phase 52 cancelled; binary releases on `postfix/serena` are sufficient)
- [~] **PKG-03**: ~~Scoop bucket install~~ — Removed 2026-04-26 (Phase 52 cancelled)
- [~] **PKG-04**: ~~Linux native package install~~ — Removed 2026-04-26 (Phase 52 cancelled)

### Observability

- [ ] **OBS-01**: Ship JSON Grafana dashboards in `deploy/grafana/` covering RED metrics, lspool worker health, and workspace activity — documented in USAGE.md with screenshots
- [ ] **OBS-02**: Ship written runbooks (in `docs/runbooks/`) for the four most common operational failure modes: `ErrCircuitOpen`, deadline timeouts, LS crash / restart, and memory-pressure eviction
- [ ] **OBS-03**: Close the v1.2 metrics gaps — add cache hit-rate (lspool + repomap), RepoMap extraction latency histogram, session lifecycle counters, and edit-tool outcome counters with bounded labels
- [ ] **OBS-04**: Audit and close trace coverage gaps — every MCP tool handler and every outbound LS call has a span, sampling configuration is documented, and trace attributes pass a hygiene review (no PII, no unbounded cardinality)

## Future Requirements

Deferred to a later milestone, tracked here to prevent loss.

### LSP quirk follow-ups

- **BUG-DEFER-01**: Upstream jdtls cold-start fix (tune jdtls itself, not just cache it) — only if the warm-cache approach proves insufficient in practice
- **BUG-DEFER-02**: Upstream rust-analyzer rename fix submitted + landed — currently mitigated at tool level in BUG-02

### Packaging coverage expansion

- **PKG-DEFER-01**: Second Linux package format (whichever of apt/rpm/AUR is not shipped in PKG-04)
- **PKG-DEFER-02**: Docker / container images published to GHCR

### Observability expansion

- **OBS-DEFER-01**: Prometheus Alertmanager rules shipped alongside dashboards
- **OBS-DEFER-02**: OpenTelemetry collector reference config

## Out of Scope

Explicitly excluded from v1.9 to prevent scope creep.

| Feature | Reason |
|---------|--------|
| New MCP tools | v1.9 is polish + infra; new capabilities belong to a future feature milestone |
| Breaking config changes | v1.9 is a minor release; migrations are deferred to v2.0 |
| Custom LS implementations | Out of scope project-wide — wrap existing LSP servers, don't reimplement |
| Auto-update mechanism inside the binary | Package managers (brew/scoop/apt) own updates; in-binary updater adds security surface |
| Docker / container images | Deferred to PKG-DEFER-02 — binary-first story is stronger for local dev agents |
| Alertmanager rules | Dashboards + runbooks first; alerting rules require production deployment context |

## Traceability

Maps each REQ-ID to exactly one phase. Filled by the roadmapper at v1.9 kickoff.

| Requirement | Phase | Status |
|-------------|-------|--------|
| BUG-01 | Phase 46 | Pending |
| BUG-02 | Phase 47 | Pending |
| BUG-03 | Phase 48 | Pending |
| BUG-04 | Phase 49 | Pending |
| TOOL-01 | Phase 50 | Pending |
| TOOL-02 | Phase 50 | Cancelled (Phase 50) |
| PKG-01 | Phase 51 | Pending |
| PKG-02 | -- | Removed (Phase 52 cancelled) |
| PKG-03 | -- | Removed (Phase 52 cancelled) |
| PKG-04 | -- | Removed (Phase 52 cancelled) |
| OBS-01 | Phase 54 | Pending |
| OBS-02 | Phase 54 | Pending |
| OBS-03 | Phase 53 | Pending |
| OBS-04 | Phase 55 | Pending |

**Coverage:**
- v1.9 requirements: 14 total
- Mapped to phases: 14
- Unmapped: 0

---
*Requirements defined: 2026-04-24*
*Last updated: 2026-04-24 — traceability filled at roadmap creation (10 phases, 46–55)*
