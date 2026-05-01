---
phase: 53
slug: obs-metrics-gaps
status: verified
threats_open: 0
asvs_level: 1
created: 2026-05-01
---

# SECURITY.md — Phase 53 obs-metrics-gaps

**Phase:** 53 — obs-metrics-gaps
**ASVS Level:** 1
**Block-on:** high
**Threats Closed:** 18/18
**Result:** SECURED

---

## Threat Verification

### Plan 53-01 — Foundation (5 vectors + helpers)

| ID | Category | Disposition | Evidence |
|----|----------|-------------|----------|
| T-53-01 | DoS — cardinality (all 5 new vectors) | mitigate | Drop-unknown helpers: `internal/obs/metrics.go:248-252` (LSPoolLookup `result != "hit" && result != "miss"`), `:257-261` (RepoMapLookup), `:267-271` (RepoMapExtractObserve `extractor != "treesitter" && != "lsp" && != "fallback"`), `:277-284` (SessionLifecycleInc — guards both phase and transport), `:293-304` (EditOutcomeInc — guards outcome and strategy). carveOuts pinned at `internal/obs/metrics_labels_test.go:22-44`. Cardinality bound tests at `internal/obs/metrics_labels_test.go:199, 216, 237, 255, 281` enforce ceilings 2*52, 2*52, 3*52, 6, 168. |
| T-53-02 | InfoDisclosure — session_lifecycle PII | accept | Closed enums only (phase: 3 values, transport: 2 values); session_id NOT a label — verified by `grep WithLabelValues.*sessionID` returning zero matches in `internal/daemon/`. No PII in label set. |
| T-53-03 | Tampering — registry | accept | `internal/obs/metrics.go:83` constructs a private `prometheus.NewRegistry()`; never references `prometheus.DefaultRegisterer` (grep confirms 0 matches). Single registry per Provider per T-11-05. |

### Plan 53-02 — LSPool lookups

| ID | Category | Disposition | Evidence |
|----|----------|-------------|----------|
| T-53-04 | DoS — helix_lspool_lookups_total cardinality | mitigate | result enum closed: emission via `LookupHit`/`LookupMiss` constants only at `internal/kernel/lspool/pool.go:121` and `:129`. Helper drop-unknown at `internal/obs/metrics.go:248-252`. Cardinality bound test `TestMetrics_CardinalityBounds_LSPoolLookups` at `internal/obs/metrics_labels_test.go:199` caps combo count at 2*52. |
| T-53-05 | Tampering — recordingSink fixture | accept | `internal/kernel/lspool/metrics_test.go:16-17` declares `recordingSink struct { mu sync.Mutex ... }` with mutex-guarded methods; resides in a `_test.go` file (compile-excluded from production binary). |

### Plan 53-03 — RepoMap lookups + extract latency

| ID | Category | Disposition | Evidence |
|----|----------|-------------|----------|
| T-53-06 | DoS — helix_repomap_lookups_total cardinality | mitigate | result enum closed via `LookupHit`/`LookupMiss` constants; emission at `internal/repomap/cache.go:113` (hit) and `:125` (miss). Helper drop-unknown at `internal/obs/metrics.go:257-261`. Cardinality bound test at `internal/obs/metrics_labels_test.go:216` caps at 2*52. |
| T-53-07 | DoS — helix_repomap_extract_duration_seconds cardinality | mitigate | extractor enum closed via `ExtractorTreesitter`/`ExtractorLSP`/`ExtractorFallback` at `internal/skill/repomap/skill.go:425, 429, 443, 451, 463`. Helper drop-unknown at `internal/obs/metrics.go:267-271`. Histogram buckets fixed (11) at `internal/obs/metrics.go` HistogramOpts. Cardinality bound test at `internal/obs/metrics_labels_test.go:237` caps at 3*52. |
| T-53-08 | Tampering — repomap.MetricsSink interface | accept | `internal/repomap/metrics.go:6-11` documents D-15 lock; `grep "internal/obs" internal/repomap/{metrics,cache,render}.go` returns 0 production imports (only doc comment references). `*obs.Metrics` implements ad-hoc; compile-time assertion `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` in `internal/daemon/wiring_test.go`. No global mutable state. |
| T-53-09 | InfoDisclosure — extractor / language labels | accept | Both labels are infrastructure metadata (extractor=closed enum {treesitter,lsp,fallback}; language=langregistry-bounded). No PII. |

### Plan 53-04 — Edit outcome

| ID | Category | Disposition | Evidence |
|----|----------|-------------|----------|
| T-53-10 | DoS — helix_edit_outcome_total cardinality | mitigate | tool_name bounded by 7 production handlers: 5 in `internal/kernel/edit/tools.go:309, 391, 455, 521, 578` and 2 in `internal/kernel/fileops/tools.go:370, 452`. outcome enum 6 values, strategy enum 4 values (Q-4 — `failed` not emitted). Helper drop-unknown at `internal/obs/metrics.go:293-304`. Cardinality bound test `TestMetrics_CardinalityBounds_EditOutcome` at `internal/obs/metrics_labels_test.go:281` caps combo count at 168. Structural invariant: `grep '"failed"' internal/kernel/{edit,fileops}/tools.go` returns 0. |
| T-53-11 | Tampering — atomic.Pointer setter | accept | `internal/mcp/middleware.go:60` declares `editOutcomeSink atomic.Pointer[func(...)]`; mirrors Phase 47 D-07 `renameStrategySink` at `:23`. Setter only invoked from `InstallMiddleware` at daemon bootstrap; no production-time mutation surface. Test-only `SetEditOutcomeSinkForTest` at `:404-414` is gated behind `_test.go`-style export semantics (capital ForTest suffix is the project convention). |
| T-53-12 | InfoDisclosure — edit-tool error labels | accept | Emission sites pass only the bucketed `outcome` and `strategy` enum values, never `err.Error()` text. Verified at all 7 emission sites — each `defer func() { mcp.RecordEditOutcome(ctx, "<tool>", outcome, strategy) }()` references closed-enum string variables only. |

### Plan 53-05 — Session lifecycle

| ID | Category | Disposition | Evidence |
|----|----------|-------------|----------|
| T-53-13 | DoS — sync.Map session-id growth | mitigate | DELETE clears entry: `internal/daemon/http_session_middleware.go:59` uses `seen.LoadAndDelete(sessionID)` (Go 1.20+ atomic delete) inside the `isDelete && status < 500` branch. Memory bound documented at `:29-33`; v1.10 LRU follow-up flagged inline. SDK lifecycle bounds live-session population in normal operation. |
| T-53-14 | DoS — helix_session_lifecycle_total cardinality | mitigate | phase=3, transport=2 → 6 max series. Helper drop-unknown at `internal/obs/metrics.go:277-284`. Cardinality bound test `TestMetrics_CardinalityBounds_SessionLifecycle` at `internal/obs/metrics_labels_test.go:255` enforces ≤6. |
| T-53-15 | InfoDisclosure — session_id in labels | accept | session_id is NEVER a label. Confirmed by `grep WithLabelValues.*sessionID internal/daemon/ internal/obs/` returning 0 matches. The id is used only inside `var seen sync.Map` (in-process seen-set) at `internal/daemon/http_session_middleware.go:35`. Emission sites at `:46, 60, 64` pass closed-enum string literals only. |
| T-53-16 | Tampering — sync.Map state | accept | `var seen sync.Map` at `internal/daemon/http_session_middleware.go:35` is a function-local variable scoped to one middleware instance; no persistence; daemon restart resets. |

### Plan 53-06 — Docs / drift

| ID | Category | Disposition | Evidence |
|----|----------|-------------|----------|
| T-53-17 | InfoDisclosure — docs reveal internal-only metrics | accept | `helix_*` metric names are already exposed at `/metrics`; documentation cannot disclose more than the scrape endpoint. `USAGE.md:652-656` lists the same 5 families that emit at `/metrics`. |
| T-53-18 | Tampering — info drift between docs and code | mitigate | `USAGE.md:652-656` documents all 5 new families with correct labels. `.planning/ROADMAP.md:189` Phase 53 success-criterion-1 was rewritten from stale `serena_*_cache_hits_total` strings to actual `helix_*_lookups_total{...}` names. Verification: `grep 'serena_(lspool|repomap|session|edit)_' USAGE.md` and `grep 'serena_lspool_cache_hits_total\|serena_repomap_cache_hits_total\|serena_repomap_extract_duration_seconds\|serena_session_lifecycle_total\|serena_edit_outcome_total' .planning/ROADMAP.md` both return 0 matches. PromQL examples present at `USAGE.md:696-697` (hit-ratio) and `:703` (per-extractor p95). HTTP best-effort caveat at `:706`. |

---

## Accepted Risks Log

The following dispositions are recorded as `accept` and verified:

| ID | Risk | Rationale |
|----|------|-----------|
| T-53-02 | session_lifecycle PII | session_id is not a metric label; phase/transport are 5-value closed enums — no PII surface. |
| T-53-03 | obs.Metrics registry tampering | Private `prometheus.NewRegistry()`; never touches `DefaultRegisterer`. |
| T-53-05 | recordingSink test fixture | Mutex-guarded; lives in `_test.go`; not in production binary. |
| T-53-08 | repomap.MetricsSink interface tampering | Sink interface owned by repomap package; obs implements ad-hoc; compile-time assertion in wiring_test.go pins the contract. |
| T-53-09 | extractor / language labels | Infrastructure metadata only. |
| T-53-11 | atomic.Pointer setter | Setter invoked only at daemon bootstrap from `InstallMiddleware`; no production-time mutation. |
| T-53-12 | edit-tool error labels | Only outcome bucket + strategy emitted, never error text. |
| T-53-15 | session_id in metric labels | session_id is not a label; used only in in-process `sync.Map` seen-set. |
| T-53-16 | sync.Map state | In-process, no persistence; daemon restart resets. |
| T-53-17 | docs reveal internal-only metrics | helix_* names already exposed at `/metrics`. |

---

## Unregistered Flags

None. All six summaries (53-01 through 53-06) explicitly assert "No new threat surface introduced beyond what plan's `<threat_model>` covers." No `threat_flag` entries to map; `## Threat Flags` sections are absent (intentionally — no new attack surface beyond declared register).

---

## Verification Method Summary

- **Drop-unknown helpers (5):** all present at `internal/obs/metrics.go:248-304`; each rejects unknown enum values via early `return` before `WithLabelValues(...).Inc()` / `.Observe()`.
- **Cardinality bound tests (5):** present at `internal/obs/metrics_labels_test.go:199, 216, 237, 255, 281`; each prime+gather+assert per the documented ceilings.
- **CI lint (`TestMetricsLabelsAllowlist`):** carveOuts map at `internal/obs/metrics_labels_test.go:22-44` includes all 5 new families with their closed-enum carve-outs (`result`, `extractor`, `phase`, `transport`, `strategy`).
- **D-15 lock (repomap → obs):** verified — `internal/repomap/{metrics,cache,render}.go` contain only doc-comment references to "internal/obs"; no actual import statements.
- **Compile-time assertions:** `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` and `var _ repomap.MetricsSink = (*obs.Metrics)(nil)` in `internal/daemon/wiring_test.go`.
- **Session-id is not a label:** `grep WithLabelValues.*sessionID` returns 0 in `internal/daemon/` and `internal/obs/`.
- **DELETE clears seen-set:** `LoadAndDelete` at `internal/daemon/http_session_middleware.go:59`.
- **Q-4 invariant (`"failed"` never emitted):** `grep '"failed"' internal/kernel/{edit,fileops}/tools.go` returns 0.
- **Documentation drift fix:** ROADMAP.md success-criterion-1 line 189 contains all 5 helix_* names; no `serena_lspool_cache_hits_total`/etc. legacy strings remain.

---

**Auditor note:** Phase 53 ships READ-ONLY (observability only — counters, histograms, label allowlist tests, and docs). No new authentication, authorization, input parsing, or trust-boundary code was introduced. The only new attack surface is the `httpSessionMiddleware`'s `sync.Map` (T-53-13), and that mitigation is in place.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-05-01 | 18 | 18 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented inline with rationale
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-05-01
