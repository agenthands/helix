# Phase 58: v1.9 Carryover — Release & Distribution - Pattern Map

**Mapped:** 2026-05-03
**Files analyzed:** 22 (modified: 16, deleted: 4, created: 5; one file appears in both modified+renamed buckets)
**Analogs found:** 19 / 22 (3 have no analog: trust-root JSON fixture, fixture-generation script, end-to-end trace integration test — first integration test of its kind)

## File Classification

| Action | File | Role | Data Flow | Closest Analog | Match Quality |
|--------|------|------|-----------|----------------|---------------|
| MODIFY | `.goreleaser.yaml` | release config (YAML) | build-time | self (lines 50-63 minisign block) → cosign block | exact (in-place block swap) |
| MODIFY | `.github/workflows/release.yml` | CI workflow (YAML) | build-time | self (existing pinned-action pattern lines 21, 48, 92) | exact (pattern preserved) |
| MODIFY | `internal/upgrade/verify.go` | Go verifier (request-response in-process) | file-I/O + crypto verify | self (Pitfall-4 canonical-error pattern) | exact (pattern preserved, library swapped) |
| MODIFY | `internal/upgrade/verify_test.go` | Go test fixture (testify-style) | unit | self (6-case structure + `withTestKey` helper) | exact (cases map 1:1) |
| MODIFY | `internal/upgrade/upgrade.go` | Go orchestrator (request-response) | streaming download + verify | self (lines 188-262 — only string + asset-name updates) | exact (mechanical asset-name swap) |
| RENAME | `internal/upgrade/pubkey.go` → `trustroot.go` | Go embed module | build-time embed | self (`//go:embed` directive pattern, lines 16-26) | exact (re-purposing) |
| MODIFY | `internal/forwarder/forwarder.go` | Go gRPC client (event-driven stdio→gRPC) | request-response | `internal/obs/tracing.go:75-93` (`WithTracing` constructor) | role-match (provider construction) |
| MODIFY | `internal/forwarder/dial.go` | Go gRPC dial site | request-response | self (lines 70-72 already has handler; add propagator option) | exact (single option add) |
| MODIFY | `internal/daemon/daemon.go` | Go gRPC server bootstrap | request-response | self (lines 572-576 mirror of dial.go) | exact (single option add) |
| MODIFY | `internal/forwarder/forwarder_test.go` | Go test (testify + tracetest) | unit | self (`TestForwarderRootSpan` lines 111-141 — in-memory exporter) | exact (extends existing pattern) |
| MODIFY | `Makefile` | build script | build-time | self (target-deletion = remove from `.PHONY` + drop dep on line 6) | exact |
| MODIFY | `go.mod` / `go.sum` | dep manifest | build-time | self (existing `require` block) | exact (`go mod tidy` mechanical) |
| MODIFY | `CONTRIBUTING.md` | docs (Markdown) | docs | self (`## Releasing` section, line 159) | exact (in-place edit) |
| MODIFY | `.planning/REQUIREMENTS.md` | planning doc (Markdown) | docs | self (existing `- [ ]` / `- [x]` markers introduce new `- [~]`) | exact |
| MODIFY | `.planning/milestones/v1.10-ROADMAP.md` | planning doc (Markdown) | docs | self (Phase 57 section as shape template, lines 36-40) | exact |
| MODIFY | `.planning/PROJECT.md` | planning doc (Markdown) | docs | self (existing "Resolved at v1.9" prose pattern) | exact |
| DELETE | `minisign.pub` (root) | embed source | — | — | n/a (deletion) |
| DELETE | `internal/upgrade/minisign.pub` | embed source | — | — | n/a (deletion) |
| DELETE | `internal/upgrade/testdata/test_keypair.{pub,key}` | test fixture | — | — | n/a (deletion) |
| DELETE | `internal/upgrade/testdata/sample-archive.tar.gz.minisig` | test fixture | — | — | n/a (deletion) |
| CREATE | `internal/upgrade/testdata/trusted_root.json` | embedded JSON fixture | static asset | — | **no analog** (new asset class) |
| CREATE | `internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json` | sigstore bundle fixture | static asset | self (existing `.minisig` fixture replaced) | partial-match (replaces .minisig) |
| CREATE | `internal/upgrade/testdata/generate_fixtures.sh` (or `.go`) | fixture-regen script | build-time | `Makefile:53-54` (bench-baseline pattern: regenerate-into-testdata) | role-match |
| CREATE | `test/integration/trace_continuity_test.go` | Go integration test | end-to-end | `test/integration/harness.go:1-50` + `internal/forwarder/forwarder_test.go:111-141` | role-match (combine harness + tracetest pattern) |

---

## Pattern Assignments

### `.goreleaser.yaml` (release config, build-time)

**Analog:** `.goreleaser.yaml:50-63` (current minisign `signs:` block — replace in place)

**Existing minisign block to remove** (lines 50-63):
```yaml
signs:
  - id: minisign
    cmd: minisign
    artifacts: all
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
```

**Replace with** (per RESEARCH.md §"goreleaser cosign block"):
```yaml
signs:
  - id: cosign
    cmd: cosign
    artifacts: all
    signature: "${artifact}.sigstore.json"
    args:
      - "sign-blob"
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
```

**Project conventions visible in analog:**
- Top-of-file comment block records "Locked decisions" cross-references (lines 1-3) — new block should append a Phase 58 D-02/D-03 reference
- `artifacts: all` and `signature: "${artifact}.<ext>"` are the established pattern shape — new block keeps both, only ext changes from `.minisig` to `.sigstore.json`
- No `stdin:` / no env-var password (cosign keyless reads OIDC from the runner)

---

### `.github/workflows/release.yml` (CI workflow, build-time)

**Analog:** self — same workflow demonstrates the pinned-action SHA convention

**Permission-block pattern** (lines 8-9, current):
```yaml
permissions:
  contents: write   # required: create release, upload assets
```

**Pattern to copy for cosign migration:**
```yaml
permissions:
  contents: write   # required: create release, upload assets
  id-token: write   # required: cosign keyless OIDC token (Phase 58 D-02)
```

**Pinned-action pattern** (existing example, line 92):
```yaml
- name: Reproducibility gate -- snapshot pass 1
  uses: goreleaser/goreleaser-action@ec59f474b9834571250b370d4735c50f8e2d1e29  # v7.0.0
```

**New cosign installer step (must follow this pattern — pin SHA, comment with version):**
```yaml
- name: Install cosign
  uses: sigstore/cosign-installer@<PIN_SHA>  # v3.x.x
  with:
    cosign-release: 'v2.4.1'
```

**Steps to delete** (RESEARCH.md §"release.yml additions" table):
- Lines 25-36: "Refuse PLACEHOLDER minisign public key" entire step
- Lines 38-45: "Verify embedded minisign.pub matches repo-root" entire step
- Lines 53-89: "Install minisign (pinned)" entire 37-line block
- Lines 131-145: "Write minisign secret key to disk (umask 077)" entire step
- Line 155: `MINISIGN_PASSWORD: ${{ secrets.MINISIGN_PASSWORD }}` env line in "Real release" step
- Lines 157-164: "Wipe minisign secret key" entire step

**Project conventions visible in analog:**
- Comments tie steps back to `.planning/phases/...` research (e.g., "WR-04", "CR-04" — preserve this style for the cosign edits)
- Pinned commits with `#vX.Y.Z` trailing comment for human-readable version is mandatory across the file
- `set -euo pipefail` at the top of every shell `run:` block
- Errors use `::error::` / `::error file=...` GitHub Actions annotations (lines 33, 75, 119)

---

### `internal/upgrade/verify.go` (Go verifier, file-I/O + crypto verify)

**Analog:** self — preserve the Pitfall-4 canonical-error pattern; swap library

**Pattern to PRESERVE** (current verify.go:9-19, 60-75 — the invariant, NOT the implementation):
```go
// The error string "signature verification FAILED" is the SINGLE message
// returned by VerifyArchive at every failure site (decode-pubkey,
// parse-signature, verify, missing-file). The wording is deliberately
// identical at all branches so an attacker probing the failure modes
// cannot distinguish "tampered signature" from "wrong key" from
// "malformed signature" by inspecting the error text. See
// 52-RESEARCH.md Pitfall 4.
//
// The literal is repeated at each return site (rather than centralized
// in a sentinel) so a `grep -c 'signature verification FAILED'` gate can
// confirm coverage at every failure branch.
```

**Per-failure-site error pattern** (verify.go:62-63 — repeat literally at every return):
```go
if err != nil {
    // canonical: signature verification FAILED — see Pitfall 4.
    return errors.New("signature verification FAILED")
}
```

**Imports to delete:**
```go
"github.com/jedisct1/go-minisign"
```

**Imports to add** (RESEARCH.md §"Verifier API surface", verbatim from sigstore-go example):
```go
"github.com/sigstore/sigstore-go/pkg/bundle"
"github.com/sigstore/sigstore-go/pkg/root"
"github.com/sigstore/sigstore-go/pkg/verify"
```

**Test override pattern to PRESERVE** (current lines 21-38):
```go
// testPubKeyOverride lets tests substitute a non-production minisign
// public key … without touching the embedded pubKeyBytes …
// Package-private to prevent external callers from disabling
// verification at runtime.
var testPubKeyOverride []byte

func currentPubKey() []byte {
    if testPubKeyOverride != nil {
        return testPubKeyOverride
    }
    return pubKeyBytes
}
```

→ Re-cast as `testTrustedRootOverride []byte` + `currentTrustedRoot()` returning `trustedRootJSON` from the embed; preserve the package-private + nil-default pattern.

**Project conventions visible in analog:**
- Single-line `// canonical: signature verification FAILED — see Pitfall 4.` comment above every `errors.New("signature verification FAILED")` return
- Function godoc explicitly enumerates failure modes that collapse to the canonical string (verify.go:46-58)
- `errors.New(...)` (NOT `fmt.Errorf`) at every failure branch — no `%w` wrapping that would leak the underlying sigstore-go error message

---

### `internal/upgrade/verify_test.go` (Go test, unit)

**Analog:** self — 6-case structure + `withTestKey` helper

**Test-helper pattern to PRESERVE** (lines 19-28):
```go
func withTestKey(t *testing.T) {
    t.Helper()
    pub, err := os.ReadFile(testdataPubkey)
    if err != nil {
        t.Fatalf("reading testdata pubkey: %v", err)
    }
    prev := testPubKeyOverride
    testPubKeyOverride = pub
    t.Cleanup(func() { testPubKeyOverride = prev })
}
```

→ Re-cast as `withTestTrustRoot(t *testing.T)` reading `testdata/trusted_root.json` and setting `testTrustedRootOverride`; preserve `t.Helper()` + `t.Cleanup` posture.

**Canonical-error assertion pattern** (lines 77-79 — copy literally to every new test case):
```go
if !strings.Contains(err.Error(), canonicalErrText) {
    t.Fatalf("VerifyArchive(<case>) err = %q, want canonical %q", err.Error(), canonicalErrText)
}
```

**Test-case map (existing → cosign-equivalent)** — keep names where possible:

| Existing | Cosign-equivalent | Action |
|----------|-------------------|--------|
| `TestVerifyArchiveHappyPath` | same name | rewrite (use bundle fixture) |
| `TestVerifyArchiveTamperedSig` | rename → `TestVerifyArchiveTamperedBundle` | rewrite (flip byte in bundle) |
| `TestVerifyArchiveWrongKey` | rename → `TestVerifyArchiveWrongTrustRoot` | rewrite |
| `TestVerifyArchiveMissingArchive` | same name | rewrite (path swap only) |
| `TestVerifyArchiveMissingSig` | rename → `TestVerifyArchiveMissingBundle` | rewrite |
| `TestVerifyArchiveMalformedKey` | rename → `TestVerifyArchiveMalformedTrustRoot` | rewrite |
| (Wave 0 NEW) | `TestVerifyArchiveWrongIdentity` | new — pen-test SAN regex (R-2) |
| (Wave 0 NEW) | `TestVerifyArchiveWrongIssuer` | new — wrong OIDC issuer |

**Project conventions visible in analog:**
- Top-of-file `const` block consolidates testdata paths + canonical error text (lines 10-15)
- Tampered-fixture tests build the tampered file in `t.TempDir()` rather than checking it in (lines 68-71)
- No `testify` in this file — uses stdlib `t.Fatalf` + `strings.Contains` (NOT `assert.Contains`); preserve this style for parity with the Pitfall-4 grep gate

---

### `internal/upgrade/upgrade.go` (Go orchestrator, streaming download + verify)

**Analog:** self — only the string constants change

**Asset-name pattern** (lines 188-189):
```go
archiveName := archiveAssetName(rel.TagName, runtime.GOOS, runtime.GOARCH)
sigName := archiveName + ".minisig"
```

→ Change to:
```go
archiveName := archiveAssetName(rel.TagName, runtime.GOOS, runtime.GOARCH)
bundleName := archiveName + ".sigstore.json"
```

**Asymmetric-pair guard pattern** (lines 201-224 — preserve shape, swap strings):
```go
checksumsAsset := rel.FindAsset("checksums.txt")
checksumsSigAsset := rel.FindAsset("checksums.txt.minisig")
// …
if (checksumsAsset != nil) != (checksumsSigAsset != nil) {
    var present, missing string
    if checksumsAsset != nil {
        present, missing = "checksums.txt", "checksums.txt.minisig"
    } else {
        present, missing = "checksums.txt.minisig", "checksums.txt"
    }
    fmt.Fprintf(out, "warning: release asset %s present but %s missing — refusing to upgrade …", present, missing)
    return serr.New(serr.NotFound, "release artifacts incomplete: …")
}
```

→ Replace `"checksums.txt.minisig"` → `"checksums.txt.sigstore.json"` at both string literal sites; keep guard structure verbatim.

**Verify-and-keep-stage pattern** (lines 258-262 — preserve verbatim):
```go
// Step 6: minisign verify the archive itself.
if err := VerifyArchive(stageArchive, stageSig); err != nil {
    verifyKept = true
    return err
}
```

→ Update comment to "Step 6: cosign verify the archive bundle."; rename `stageSig` → `stageBundle` for clarity; signature-of-VerifyArchive unchanged (still `(archivePath, bundlePath string) error`).

**`IsPlaceholderPubKey()` call site to DELETE** (RESEARCH.md confirms `upgrade.go:122-133` — call site becomes dead code; remove the entire developer-experience-friendly placeholder branch).

**Project conventions visible in analog:**
- `serr.New(serr.NotFound, …).WithDetail(…)` for structured release-asset errors (lines 193, 198, 222)
- `verifyKept = true` flag pattern keeps stage dir on verification failure — preserved verbatim across the rewrite
- User-visible `fmt.Fprintf(os.Stderr, …)` warnings reference the literal stage-dir path so the user can find postmortem artifacts (lines 184-185, 221)

---

### `internal/upgrade/pubkey.go` → `internal/upgrade/trustroot.go` (Go embed module, build-time embed)

**Analog:** self — preserve the `//go:embed` pattern, swap the asset

**Existing pattern** (pubkey.go:13-26):
```go
package upgrade

import (
    _ "embed"
    "bytes"
)

// pubKeyBytes is the embedded minisign public key. …
//
//go:embed minisign.pub
var pubKeyBytes []byte
```

**Re-cast as** (RESEARCH.md §"Trust root sourcing" — embed option):
```go
package upgrade

import (
    _ "embed"
)

// trustedRootJSON is the embedded sigstore TUF trusted root. The byte
// content is a snapshot of the public-good Sigstore trust root taken at
// build time; refresh per CONTRIBUTING.md §Releasing before each minor
// release. Consumed by verify.go via root.NewTrustedRootFromJSON.
//
//go:embed trusted_root.json
var trustedRootJSON []byte
```

**Project conventions visible in analog:**
- Package-doc comment at top of file (`// Package upgrade implements …`) — keep on `verify.go` or `upgrade.go`, NOT on the embed file
- Multi-line godoc on the `var pubKeyBytes` (lines 20-23) explains where the byte content originates and who consumes it — preserve this shape for `trustedRootJSON`
- The `placeholderMarker` const + `IsPlaceholderPubKey()` function are DELETED (no placeholder concept in cosign keyless); also delete the `bytes` import they pulled in

**R-3 ordering hazard:** the `//go:embed minisign.pub` directive must be deleted in the SAME commit that deletes `minisign.pub` (RESEARCH.md §"Risks & Gotchas R-3"). The recommended commit order: rewrite `pubkey.go` → `trustroot.go` with the new `//go:embed trusted_root.json`, ship `trusted_root.json` in the same commit, THEN delete the old `minisign.pub` files in a follow-up commit. Otherwise `go build` fails with "pattern minisign.pub: no matching files found."

---

### `internal/forwarder/forwarder.go` (Go gRPC client, request-response)

**Analog (primary):** `internal/obs/tracing.go:75-93` — `WithTracing` constructor pattern

**`WithTracing` constructor pattern** (the analog the new code copies):
```go
// WithTracing constructs a Provider with a real SDK TracerProvider backed by
// an OTLP/gRPC exporter. If cfg.Endpoint is empty, the caller should use
// Noop() instead — this function is intended for the non-empty endpoint path.
func WithTracing(inner slog.Handler, cfg TracingConfig, logger *slog.Logger) *Provider {
    p := Noop(inner)
    if cfg.Endpoint == "" {
        return p
    }
    tp, err := newTracerProvider(context.Background(), cfg)
    if err != nil {
        logger.Warn("tracing exporter construction failed; falling back to noop", "error", err)
        return p
    }
    p.tracerProvider = tp
    logger.Info("tracing enabled", "endpoint", cfg.Endpoint, ...)
    return p
}
```

**Existing forwarder.go:25 (current Noop construction):**
```go
fwdProvider := obs.Noop(logger.Handler())
```

**New construction pattern** (RESEARCH.md §"TracerProvider sourcing" — env-var-driven, Option 1):
```go
endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
fwdProvider := obs.WithTracing(logger.Handler(), obs.TracingConfig{
    Endpoint:    endpoint,                  // empty → falls back to noop, see tracing.go:78
    ServiceName: "helix-forwarder",
    SampleRatio: 1.0,
}, logger)
```

**Project conventions visible in analog:**
- `degraded-optional` semantics (D-09 in `obs/tracing.go:1-13`): construction never panics; failure logs a warning and returns noop. Forwarder must adopt the same pattern — DON'T fail the forwarder if tracing setup errors.
- `*Provider` is always non-nil (`obs/obs.go:73-77` — `TracerProvider()` never returns nil); preserve this invariant for `fwdProvider`.
- `ShutdownTracing(ctx)` (`obs/obs.go:89-95`) — forwarder must call this before exit if it switches to a real SDK provider; existing forwarder.go has no shutdown call (because Noop has nothing to flush). Add a `defer fwdProvider.ShutdownTracing(...)` near the existing `defer conn.Close()` (forwarder.go:31).

**Global propagator install** (a NEW pattern — no existing analog in repo per RESEARCH.md §"Why traces don't unify today" point 2):
- The decision in RESEARCH.md is to use `otelgrpc.WithPropagators(propagation.TraceContext{})` per-handler in BOTH `dial.go` and `daemon.go` (NOT a global `otel.SetTextMapPropagator`). This avoids touching package-global state, which `obs/tracing.go:13` explicitly forbids ("No call to otel.SetTracerProvider anywhere in this package (D-01).").

---

### `internal/forwarder/dial.go` (Go gRPC dial site, request-response)

**Analog:** self — single-option add to existing handler installation

**Existing pattern** (lines 70-72):
```go
grpc.WithStatsHandler(otelgrpc.NewClientHandler(
    otelgrpc.WithTracerProvider(tp),
)),
```

**New pattern** (RESEARCH.md §"Code Examples", REL-06):
```go
grpc.WithStatsHandler(otelgrpc.NewClientHandler(
    otelgrpc.WithTracerProvider(tp),
    otelgrpc.WithPropagators(propagation.TraceContext{}),
)),
```

**Required new import:**
```go
"go.opentelemetry.io/otel/propagation"
```

**Project conventions visible in analog:**
- The existing handler-installation block carries an inline comment ("Phase 12: otelgrpc client handler … WithTracerProvider is MANDATORY") — extend the comment to note the propagator addition with a Phase 58 D-06 reference
- "Pitfall 5: only one WithStatsHandler call (gRPC silently overwrites duplicates)" warning is preserved — DO NOT add a second `WithStatsHandler` line

---

### `internal/daemon/daemon.go` (Go gRPC server bootstrap, request-response)

**Analog:** self — mirror of dial.go change

**Existing pattern** (lines 572-576):
```go
d.grpcServer = grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler(
        otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
    )),
)
```

**New pattern:**
```go
d.grpcServer = grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler(
        otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
        otelgrpc.WithPropagators(propagation.TraceContext{}),
    )),
)
```

**Project conventions visible in analog:**
- Inline comment at lines 569-571 references "D-12 … WithTracerProvider is MANDATORY — omitting it falls back to the OTel global which D-01 forbids." Same applies to the propagator addition: the per-handler `WithPropagators` keeps the no-global rule intact. Update the comment.

---

### `internal/forwarder/forwarder_test.go` (Go test, unit)

**Analog:** self — `TestForwarderRootSpan` lines 111-141 demonstrates the in-memory exporter pattern

**Pattern to copy** (lines 111-141 verbatim shape):
```go
exporter := tracetest.NewInMemoryExporter()
tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sdktrace.AlwaysSample()),
    sdktrace.WithSyncer(exporter),
)
defer func() { _ = tp.Shutdown(context.Background()) }()

tracer := tp.Tracer("test")
// … exercise the system under test …
_ = tp.ForceFlush(context.Background())
spans := exporter.GetSpans()
require.Len(t, spans, 1, "expected exactly one span")
assert.Equal(t, "forwarder.tools.call", spans[0].Name)
```

**Imports to copy verbatim** (lines 11-13 — already in this file):
```go
sdktrace "go.opentelemetry.io/otel/sdk/trace"
"go.opentelemetry.io/otel/sdk/trace/tracetest"
tracenoop "go.opentelemetry.io/otel/trace/noop"
```

**Project conventions visible in analog:**
- `defer func() { _ = tp.Shutdown(context.Background()) }()` — discard error; matches `obs.ShutdownTracing` interface
- `tracetest.NewInMemoryExporter` + `WithSyncer` (not `WithBatcher`) so spans are visible immediately without `ForceFlush` racing
- `tracenoop.NewTracerProvider()` is the explicit noop in tests (NOT `nil`) — pass it to `tryConnect` / `waitForDaemon` for unit cases that should not record spans (lines 19, 36, 46)

**For trace-continuity test**: this file's `TestForwarderRootSpan` only tests the forwarder side. The new end-to-end test (REL-06) belongs in `test/integration/trace_continuity_test.go` because it needs the daemon harness — see that file's pattern below.

---

### `Makefile` (build script, build-time)

**Analog:** self — target deletion is a mechanical removal from `.PHONY` + the dep edge

**Pattern to delete (lines 1, 6, 61-66):**
```makefile
.PHONY: build clean … embed-pubkey verify-embed-pubkey

build: embed-pubkey
    $(GO) build -o $(BINARY) ./cmd/helix

# … (lines 61-66) …
embed-pubkey: ## Sync repo-root minisign.pub into internal/upgrade/minisign.pub before build
    @cp minisign.pub internal/upgrade/minisign.pub

verify-embed-pubkey: ## CI gate: assert internal/upgrade/minisign.pub matches repo-root copy byte-for-byte
    @cmp -s minisign.pub internal/upgrade/minisign.pub || { \
      echo "internal/upgrade/minisign.pub drift; run: make embed-pubkey"; exit 1; }
```

**Becomes:**
```makefile
.PHONY: build clean … (remove embed-pubkey verify-embed-pubkey)

build:
    $(GO) build -o $(BINARY) ./cmd/helix

# (lines 61-66 deleted entirely)
```

**Project conventions visible in analog:**
- `## doc-string after target` (line 32, 35, 50, etc.) — used by some help targets; if any new target replaces these, preserve the convention
- `@cp ...` / `@cmp -s ... || { … exit 1; }` shell idioms — none of these need to be carried forward; nothing replaces the deleted gate (RESEARCH.md A7: drift detection moves to upstream sigstore TUF root sourcing)

---

### `go.mod` / `go.sum` (dep manifest, build-time)

**Analog:** self — `go mod tidy` mechanical

**Operation:**
```bash
# After verify.go rewrite:
go mod tidy
# Removes: github.com/jedisct1/go-minisign
# Adds:    github.com/sigstore/sigstore-go v1.1.4 (or latest v1.1.x)
#          + transitive deps (go-tuf/v2 if TUF runtime path; not for embed path)
```

**Pin recommendation:** `github.com/sigstore/sigstore-go v1.1.x` (per RESEARCH.md §"Risks R-8"). Pin to a specific minor version; bump deliberately.

---

### `CONTRIBUTING.md` (docs, Markdown)

**Analog:** self — line 161 contains the existing reproducibility paragraph

**Pattern to PRESERVE (line 161 first sentence):**
```
The CI-enforced reproducibility gate runs two consecutive snapshot builds with identical inputs and refuses to publish if their archive sha256s differ, catching most build-environment non-determinism (toolchain drift, mod_timestamp, trimpath, GOFLAGS) before publication.
```

**Pattern to EDIT** (insert "Pass-3 limitation" anchor — Option A from RESEARCH.md §REL-05):

Change `"The gate does not, however, compare against the real-release artifacts"` →
`"This is the documented "Pass-3 limitation": the gate does NOT compare Pass-1 / Pass-2 / Pass-3 hashes against the real-release artifacts that ship to users"`.

The CONTEXT §Specifics REQUIRES the literal phrase **"Pass-3 limitation"** (verifiable via `grep -q 'Pass-3 limitation' CONTRIBUTING.md`).

**Sections to delete** (RESEARCH.md §"internal/upgrade/ blast radius" CONTRIBUTING.md row, lines 159-214):
- Lines 180-187: "### Repository secrets" (`MINISIGN_PRIVATE_KEY` / `MINISIGN_PASSWORD`)
- Lines 189-203: "### One-time keypair setup"
- Lines 205-207: "### Key rotation" — replace with cosign-keyless equivalent (no keypair, no rotation)

**New cosign section to add (replacement for deleted ceremony):**
```
### Repository secrets

The release workflow signs archives with sigstore cosign keyless via the
GitHub Actions OIDC token. No long-lived secrets are required. The workflow
declares `id-token: write` permission to obtain the OIDC token, exchanges
it with Fulcio for a short-lived signing certificate, and submits the
signature to Rekor for transparency-log inclusion.

### Trust root refresh

The verifier embeds `internal/upgrade/trusted_root.json` (the public-good
Sigstore TUF trust root snapshot). Refresh before each minor release …
```

**Project conventions visible in analog:**
- `## Releasing` is the established H2 anchor; keep it as the parent heading
- H3 sub-sections (`### Repository secrets`, `### One-time keypair setup`, etc.) — preserve this structure for the new cosign sections
- Code fences use `sh` for shell snippets (line 165, 173, 193) — preserve

---

### `.planning/REQUIREMENTS.md` (planning doc, Markdown)

**Analog:** self — existing `- [ ]` markers are the shape; `- [~]` is new

**Existing pattern** (lines 121-126, REL-01..REL-06):
```markdown
- [ ] **REL-01** (was PKG-01 SC-3): The first signed Helix release …
- [ ] **REL-02** (was PKG-DEFER-03): A Homebrew tap publishes `helix` …
- [ ] **REL-03** (was PKG-DEFER-04): A Scoop bucket publishes `helix` …
- [ ] **REL-04** (was PKG-DEFER-05): A native Linux package …
- [ ] **REL-05** (Phase 51 architectural fix): The reproducibility gate …
- [ ] **REL-06** (Phase 55 follow-up): `forwarder.tools.call` span …
```

**Update REL-02/03/04** to (CONTEXT §Specifics — literal):
```markdown
- [~] **REL-02** (was PKG-DEFER-03): A Homebrew tap publishes `helix` … — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
- [~] **REL-03** (was PKG-DEFER-04): A Scoop bucket publishes `helix` … — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
- [~] **REL-04** (was PKG-DEFER-05): A native Linux package … — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
```

**Status legend (NEW — RESEARCH.md §"Marker convention"):** add at top of file (or just above REL-section):
```markdown
## Status legend

- `- [ ]` pending
- `- [x]` complete
- `- [~]` won't-do (rejected with rationale appended after em-dash)
```

**Project conventions visible in analog:**
- `**REQ-ID** (origin tag): description.` is the established line shape — preserve when appending the rationale (use ` — ` em-dash separator)
- Origin tag in parens (e.g., "was PKG-DEFER-03") records milestone-roll-forward provenance — keep verbatim
- Verifiable via `grep -E '^- \[~\] \*\*REL-(02|03|04)' .planning/REQUIREMENTS.md | wc -l` (expects 3) per VALIDATION.md

---

### `.planning/milestones/v1.10-ROADMAP.md` (planning doc, Markdown)

**Analog:** self — Phase 57 entry (lines 36-40) shows the desired post-shrink shape

**Phase 57 shape (analog for the post-shrink Phase 58):**
```markdown
**Plans**: 4 plans
- [ ] 57-01-PLAN.md — `internal/phasegraph/` library … [DAG-01..04]
- [ ] 57-02-PLAN.md — `internal/semantic/{config,store,types}` skeleton … [STORE-01,02,03,06]
```

**Phase 58 current state to EDIT** (lines 42-51):
- Drop `REL-02, REL-03, REL-04` from the `**Requirements**:` line
- Drop SC-2 (line 48: brew/scoop) and SC-3 (line 49: native Linux) from `**Success Criteria**`
- Renumber surviving SC items (current SC-1 stays, current SC-4 becomes SC-2)

**Project conventions visible in analog:**
- `**Goal**:` / `**Depends on**:` / `**Requirements**:` / `**Success Criteria**` / `**Plans**:` — bold-prefixed list pattern preserved across all phases
- Numbered SC (`1.`, `2.`, …) — renumber sequentially after deletion
- Current Phase 58 `**Plans**: TBD` becomes `**Plans**: 4 plans` populated with the planner's plan list

---

### `.planning/PROJECT.md` (planning doc, Markdown)

**Analog:** self — "Resolved at v1.9 close" prose (CONTEXT.md mentions lines 166-169)

**Pattern (post-phase update — see CONTEXT §Deferred):**
- Three tech-debt entries (PKG-01 SC-3, Phase 51 Pass-3, Phase 55 Noop tracer) move from "Tech debt accepted at v1.9 close" → new "Resolved at v1.10" line parallel to the existing "Resolved at v1.9" line.
- This is **NOT a plan task**; it lands in the post-phase `update_project_md` step (CONTEXT §Deferred final bullet). Pattern flagged here so the planner does NOT include it in any plan.

---

### `internal/upgrade/testdata/sample-archive.tar.gz.sigstore.json` (CREATE — sigstore bundle fixture)

**Analog:** the existing `internal/upgrade/testdata/sample-archive.tar.gz.minisig` it replaces (location convention preserved)

**Generation approach (RESEARCH.md §"internal/upgrade/ blast radius" Option (b)):**
- Use sigstore-go's signing API in test setup to produce a bundle against a test trust root + ephemeral key. Fixture is regenerable via the new script (`generate_fixtures.sh` / `.go`).

**No analog for the bundle file format itself.** Test fixture is opaque JSON; do not hand-edit.

---

### `internal/upgrade/testdata/trusted_root.json` (CREATE — embedded JSON fixture, no analog)

**Source:** [Sigstore public-good TUF trusted_root.json snapshot](https://github.com/sigstore/sigstore-go/blob/main/examples/trusted-root-public-good.json) (per RESEARCH.md §"Trust root sourcing").

**Generation:** vendor a snapshot at fixture-prep time; refresh per CONTRIBUTING.md release ceremony.

**No analog file in the repo.** This is a new asset class for v1.10.

---

### `internal/upgrade/testdata/generate_fixtures.sh` (CREATE — fixture-regen script)

**Analog (closest):** `Makefile:53-54` (`bench-baseline` target — regenerate-into-testdata pattern):
```makefile
bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
    $(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt
```

**Pattern to follow:**
- Single command that overwrites the testdata file
- Document in CONTRIBUTING.md as a maintainer step (matches `bench-baseline`'s "local-only" posture)
- Shell or Go program — RESEARCH.md is agnostic; recommend `.go` for portability + dependency reuse with verify.go (sigstore-go signing API is available in the same module)

**Project conventions visible in analog:**
- Maintainer-side regeneration scripts live near the data they generate (`test/bench/baselines/` for benches, `internal/upgrade/testdata/` for upgrade fixtures); preserve this co-location

---

### `test/integration/trace_continuity_test.go` (CREATE — Go integration test, end-to-end)

**Analog (combine):** `test/integration/harness.go` (daemon spin-up) + `internal/forwarder/forwarder_test.go:111-141` (in-memory exporter pattern)

**Test-package pattern** (`harness.go:1`):
```go
package integration_test

import (
    "context"
    // … standard test imports …
    "github.com/agenthands/helix/internal/daemon"
    // Blank imports trigger skill registration via init() (Caddy-style).
    _ "github.com/agenthands/helix/internal/kernel/diag"
    _ "github.com/agenthands/helix/internal/kernel/edit"
    _ "github.com/agenthands/helix/internal/kernel/fileops"
    _ "github.com/agenthands/helix/internal/kernel/symbols"
    _ "github.com/agenthands/helix/internal/profile"
    _ "github.com/agenthands/helix/internal/skill/memory"
    _ "github.com/agenthands/helix/internal/skill/workflow"
)
```

**In-memory exporter pattern** (forwarder_test.go:114-119):
```go
exporter := tracetest.NewInMemoryExporter()
tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sdktrace.AlwaysSample()),
    sdktrace.WithSyncer(exporter),
)
defer func() { _ = tp.Shutdown(context.Background()) }()
```

**Provider injection** (`obs/obs.go:58-62`):
```go
// NewForTest constructs a Provider with the given TracerProvider. Intended for
// test code that needs to inject a tracetest-backed provider.
func NewForTest(tp trace.TracerProvider) *Provider { … }
```

**Assertion shape** (RESEARCH.md §"Single-trace assertion test pattern"):
```go
spans := exporter.GetSpans()
// Find both spans by name
var fwdSpan, srvSpan tracetest.SpanStub
for _, s := range spans {
    switch s.Name {
    case "forwarder.tools.call":
        fwdSpan = s
    case "serena.v1.ForwarderService/StreamMCP":
        srvSpan = s
    }
}
require.NotZero(t, fwdSpan.SpanContext.TraceID(), "forwarder span missing")
require.NotZero(t, srvSpan.SpanContext.TraceID(), "server span missing")
require.Equal(t, fwdSpan.SpanContext.TraceID(), srvSpan.SpanContext.TraceID(), "trace IDs differ — propagator not wired")
```

**Project conventions visible in analog:**
- `package integration_test` — external test package convention for `test/integration/`
- Daemon spin-up via `Options{}` from `harness.go:34-50` — reuse this struct rather than re-implementing
- `serena.v1.ForwarderService/StreamMCP` is the literal otelgrpc-generated server-span name (note: gRPC service identifier is `serena.v1` even though the user-facing product is `helix` — see PROJECT.md / Phase 52-03 SUMMARY for the wire-format-lineage rationale)

---

## Shared Patterns

### Pitfall-4 canonical-error-string invariant
**Source:** `internal/upgrade/verify.go:9-19, 60-75`
**Apply to:** All new failure branches in the rewritten `verify.go`; ALL test cases in `verify_test.go`
**Excerpt** (the comment block — copy verbatim, swap "minisign" → "sigstore"):
```go
// The error string "signature verification FAILED" is the SINGLE message
// returned by VerifyArchive at every failure site. … The literal is
// repeated at each return site (rather than centralized in a sentinel)
// so a `grep -c 'signature verification FAILED'` gate can confirm
// coverage at every failure branch.
```
**Convention rule:** every failure-site `errors.New(…)` carries the inline comment `// canonical: signature verification FAILED — see Pitfall 4.` Verifiable via `grep -c 'canonical: signature verification FAILED'` in CI.

---

### Per-handler propagator install (no global state)
**Source:** `internal/obs/tracing.go:1-13` design rules + `internal/forwarder/dial.go:67-72` Phase-12 comment
**Apply to:** `internal/forwarder/dial.go`, `internal/daemon/daemon.go`
**Excerpt** (the rule):
> "No call to `otel.SetTracerProvider` anywhere in this package (D-01)."

**Convention rule:** the propagator is added per-handler via `otelgrpc.WithPropagators(propagation.TraceContext{})`, NOT via global `otel.SetTextMapPropagator`. This matches the no-global TracerProvider rule already in force.

---

### Pinned action SHAs in CI workflows
**Source:** `.github/workflows/release.yml:21, 48, 92` (and others)
**Apply to:** New cosign-installer step
**Excerpt:**
```yaml
uses: <action>@<40-char-commit-sha>  # vX.Y.Z
```
**Convention rule:** every `uses:` line MUST pin a 40-char commit SHA with a trailing `# vX.Y.Z` comment for human readability. Tags alone are forbidden (supply-chain attack surface).

---

### `set -euo pipefail` shell preamble
**Source:** `.github/workflows/release.yml:27, 55, 100, 115, 126, 136, 159` (every `run:` block)
**Apply to:** Any new shell `run:` block in release.yml
**Excerpt:**
```yaml
run: |
  set -euo pipefail
  # …
```
**Convention rule:** all multi-line shell blocks start with `set -euo pipefail` (no exceptions in this workflow today).

---

### Phase-/decision-tagged inline comments
**Source:** scattered across `dial.go:66-69`, `daemon.go:569-571`, `verify.go:9-19`, `release.yml:14, 38-44`
**Apply to:** All Phase 58 edits — preserve the convention
**Excerpt examples:**
```go
// Phase 12: otelgrpc client handler for trace propagation (D-01, D-12).
// WithTracerProvider is MANDATORY — without it otelgrpc falls back to the
// OTel global, violating the no-global rule.
```
```yaml
# WR-04: pin to ubuntu-22.04 (LTS) so the x86_64 minisign install URL stays
# valid as ubuntu-latest rolls forward.
```
**Convention rule:** every architecturally-load-bearing edit carries an inline comment naming the phase and decision ID (e.g., `// Phase 58 D-02: cosign keyless via Fulcio OIDC.`). Preserve this when adding the propagator option, the trust-root embed, the env-var-driven forwarder TracerProvider, etc.

---

## No Analog Found

| File | Role | Reason |
|------|------|--------|
| `internal/upgrade/testdata/trusted_root.json` | embedded sigstore TUF root JSON | First time the repo embeds an upstream-vendored JSON cryptographic asset; sigstore source is the authority. |
| `internal/upgrade/testdata/generate_fixtures.sh` (or `.go`) | fixture-regen script for sigstore bundles | The closest in-repo analog is `Makefile:53-54` `bench-baseline` (regenerate-into-testdata pattern), but no Go fixture-generator using a real signing API exists today. Treat as new pattern; co-locate next to fixtures it generates. |
| `test/integration/trace_continuity_test.go` | end-to-end OTel trace continuity test | No existing integration test asserts on cross-process trace IDs; combine `test/integration/harness.go` (daemon spin-up) + `forwarder_test.go:111-141` (in-memory exporter) — no single existing analog. |

---

## Metadata

**Analog search scope:**
- `internal/upgrade/` (entire package — verify.go, pubkey.go, upgrade.go, verify_test.go)
- `internal/forwarder/` (entire package — forwarder.go, dial.go, forwarder_test.go)
- `internal/daemon/daemon.go` (lines 560-600 — gRPC server bootstrap)
- `internal/obs/` (obs.go, tracing.go — provider construction patterns)
- `test/integration/harness.go` (daemon test harness)
- `.goreleaser.yaml`, `.github/workflows/release.yml`, `Makefile`, `CONTRIBUTING.md` (release infra)
- `.planning/REQUIREMENTS.md`, `.planning/milestones/v1.10-ROADMAP.md` (planning doc shapes)

**Files scanned:** 12 source files + 4 docs + 2 planning docs = 18

**Pattern extraction date:** 2026-05-03

**SMTC vs grep posture for Phase 58 implementation:** the patterns above were extracted via direct file reads (focused targeted reads, no SMTC needed for this docs-heavy mapping pass). Implementation-time edits in `internal/upgrade/` and `internal/forwarder/` SHOULD use `mcp__smtc__find_references` and `mcp__smtc__goto_definition` to verify minisign call-site coverage and otelgrpc symbol re-validation before `Edit`-ing — this is the project CLAUDE.md SMTC-first guidance.
