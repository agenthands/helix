package container

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"runtime"

	"github.com/google/go-containerregistry/pkg/crane"
)

// ErrNotConfigured is returned by VerifyThenPull when its fetch/pull
// collaborators are still the un-wired Plan 04 defaults. It is deliberately a
// SINGLE, clearly-labeled "not configured" error raised BEFORE any verify is
// attempted, so a production caller can never confuse an un-wired path with a
// verification result (WR-03). The per-collaborator errFetchMetaNotWired /
// errPullNotWired sentinels remain as the closure-level fail-closed backstop,
// but they are no longer the error a production VerifyThenPull surfaces: the
// top-of-function guard intercepts first, keeping the package's oracle-free
// discipline intact (no error text announces which collaborator is un-wired).
var ErrNotConfigured = errors.New("bench/container: VerifyThenPull is not configured (live fetch/pull wiring lands with the Plan 04 GHCR mirror)")

// errFetchMetaNotWired / errPullNotWired are the fail-closed sentinels returned
// by the default (un-wired) fetch/pull collaborators. The live wiring lands
// with the Plan 04 GHCR mirror. With the top-of-function ErrNotConfigured guard
// these are unreachable on a production call (the guard fires first), but they
// remain as a defense-in-depth backstop should the guard ever be bypassed: a
// closure that somehow runs with an un-wired collaborator still fails closed,
// publishing zero cache bytes.
var (
	errFetchMetaNotWired = errors.New("bench/container: manifest+bundle fetch not wired until Plan 04 mirror")
	errPullNotWired      = errors.New("bench/container: image pull not wired until Plan 04 mirror")
)

// isNotWired reports whether fn is one of the package's un-wired default
// collaborators (notWiredFetchMeta / notWiredPull). It compares function
// identity via reflect.Value.Pointer rather than error text, so the guard never
// depends on (and never leaks) the branch-distinguishing sentinel messages.
func isNotWired(fn any) bool {
	target := reflect.ValueOf(fn).Pointer()
	return target == reflect.ValueOf(notWiredFetchMeta).Pointer() ||
		target == reflect.ValueOf(notWiredPull).Pointer()
}

// VerifyThenPull resolves a digest-pinned image, inspects its manifest
// architecture daemon-free, runs the Plan 01 ArchGate, verifies the cosign
// keyless signature, and ONLY THEN fills the Plan 02 cache. The ordering is the
// CONTAINER-03 invariant (Pitfall 3 TOCTOU): a verify failure must publish ZERO
// cache bytes.
//
// Order of operations:
//
//  1. isHexSHA256(digest) fail-closed → errBadDigest (no crafted ref crosses
//     the crane boundary).
//  2. isValidRepo(repo) fail-closed → errBadRepo (argv/ref flag-smuggling).
//  3. Resolve the manifest arch daemon-free via crane.Config(repo@sha256:digest)
//     → unmarshal config JSON → read .architecture.
//  4. ArchGate(runtime.GOARCH, manifestArch) — REFUSE a cross-arch run unless
//     BENCH_ARCH_MISMATCH_OK=1, BEFORE any verify or pull (Pitfall 5: never
//     qemu-emulate a bench image).
//  5. cache.Ensure(digest, fetch): the fetch closure FIRST fetches the manifest
//     bytes + signature bundle and calls VerifyImage; on verify failure it
//     returns the canonical error so Ensure removes the staging dir and nothing
//     is published. Only on verify success does it pull the image bytes into
//     the staging dir. (verify-then-pull, Pitfall 3.)
//
// A cache hit (sentinel present) short-circuits BEFORE any arch inspect, verify,
// or pull — Ensure returns hit=true and the closure never runs.
//
// The verifyFn / archFn / fetchFn functional-option seams are UNEXPORTED test
// hooks: production callers get the real crane/VerifyImage defaults; hermetic
// tests inject recorders to assert ordering and the no-bytes-on-verify-failure
// invariant without a live registry.
//
// Until the Plan 04 GHCR mirror wires the live fetch/pull collaborators,
// VerifyThenPull refuses to run with the un-wired defaults: it returns the
// clearly-labeled ErrNotConfigured BEFORE any verify (after the digest/repo
// fail-closed checks), so a production caller cannot mistake an un-wired path
// for a verification outcome and no error text leaks which collaborator is
// missing (WR-03).
func VerifyThenPull(ctx context.Context, repo, digest string, opts ...Option) (hit bool, dir string, err error) {
	cfg := defaultPullConfig()
	for _, o := range opts {
		o(&cfg)
	}

	if !isHexSHA256(digest) {
		return false, "", errBadDigest
	}
	if !isValidRepo(repo) {
		return false, "", errBadRepo
	}

	// Refuse to run with the un-wired Plan 04 defaults BEFORE any arch inspect,
	// verify, or pull (WR-03). This surfaces a single, clearly-labeled
	// ErrNotConfigured so a production caller cannot mistake an un-wired path for
	// a verification outcome, and so no error text announces which collaborator
	// is missing. The guard runs AFTER the digest/repo fail-closed checks so a
	// crafted ref is still rejected with errBadDigest/errBadRepo first.
	if isNotWired(cfg.fetchMetaFn) || isNotWired(cfg.pullFn) {
		return false, "", ErrNotConfigured
	}

	ref := repo + "@sha256:" + digest

	// Arch inspect + gate BEFORE any verify or pull (daemon-free, Pitfall 5).
	manifestArch, archErr := cfg.archFn(ctx, ref)
	if archErr != nil {
		return false, "", archErr
	}
	if gateErr := ArchGate(cfg.hostArch, manifestArch); gateErr != nil {
		return false, "", gateErr
	}

	// Ensure gates the cache fill with verify-then-pull INSIDE the fetch closure.
	return cfg.ensureFn(digest, func(stageDir string) error {
		manifestBytes, bundleBytes, fetchMetaErr := cfg.fetchMetaFn(ctx, ref)
		if fetchMetaErr != nil {
			return fetchMetaErr
		}
		// VERIFY BEFORE PULL: any verify failure returns the canonical error so
		// Ensure removes stageDir and publishes nothing (Pitfall 3 TOCTOU).
		if verErr := cfg.verifyFn(manifestBytes, bundleBytes); verErr != nil {
			return verErr
		}
		// Only reached on a verified signature.
		return cfg.pullFn(ctx, ref, stageDir)
	})
}

// pullConfig holds the resolved (overridable) collaborators for VerifyThenPull.
// All fields default to the real crane/VerifyImage/cache.Ensure implementations;
// the unexported Option seams swap them in hermetic tests.
type pullConfig struct {
	hostArch    string
	archFn      func(ctx context.Context, ref string) (string, error)
	fetchMetaFn func(ctx context.Context, ref string) (manifestBytes, bundleBytes []byte, err error)
	verifyFn    func(manifestBytes, bundleBytes []byte) error
	pullFn      func(ctx context.Context, ref, stageDir string) error
	ensureFn    func(sha string, fetch func(stageDir string) error) (hit bool, dir string, err error)
}

// Option configures a VerifyThenPull call. The constructors below are
// UNEXPORTED on purpose — they are test seams, not a public surface — so callers
// outside the package cannot disable verification or arch-gating.
type Option func(*pullConfig)

func defaultPullConfig() pullConfig {
	return pullConfig{
		hostArch:    runtime.GOARCH,
		archFn:      craneArch,
		fetchMetaFn: notWiredFetchMeta,
		verifyFn:    VerifyImage,
		pullFn:      notWiredPull,
		ensureFn:    Ensure,
	}
}

// craneArch reads the image config JSON daemon-free and returns its
// .architecture field. crane.Config performs a registry round-trip (no docker
// daemon) for the pinned ref.
func craneArch(_ context.Context, ref string) (string, error) {
	cfgBytes, err := crane.Config(ref)
	if err != nil {
		return "", err
	}
	var c struct {
		Architecture string `json:"architecture"`
	}
	if err := json.Unmarshal(cfgBytes, &c); err != nil {
		return "", err
	}
	return c.Architecture, nil
}

// notWiredFetchMeta is the default manifest+bundle fetcher. The live wiring
// (crane.Manifest + GHCR cosign-bundle referrer lookup) is deferred to Plan 04
// when the mirror exists. The top-of-VerifyThenPull ErrNotConfigured guard
// intercepts before this stub ever runs on a production call (see isNotWired);
// it remains as a defense-in-depth backstop so any path that somehow reaches it
// with an un-wired fetcher still fails closed rather than skipping verification.
func notWiredFetchMeta(_ context.Context, _ string) (_, _ []byte, err error) {
	return nil, nil, errFetchMetaNotWired
}

// notWiredPull is the default image-bytes puller. Like notWiredFetchMeta, the
// live crane.Pull → layer-export wiring lands with the Plan 04 mirror; the
// gated live test injects the real puller.
func notWiredPull(_ context.Context, _, _ string) error {
	return errPullNotWired
}

// --- unexported Option seams (test-only; NOT a public surface) ---
//
// These swap individual collaborators so hermetic tests can drive ordering and
// the no-bytes-on-verify-failure invariant without a live registry, and so the
// gated live test can inject the real crane fetch+pull. They are unexported so
// external callers can never disable verification or arch-gating.

func withHostArch(arch string) Option {
	return func(c *pullConfig) { c.hostArch = arch }
}

func withArchFn(fn func(ctx context.Context, ref string) (string, error)) Option {
	return func(c *pullConfig) { c.archFn = fn }
}

func withFetchMetaFn(fn func(ctx context.Context, ref string) (manifestBytes, bundleBytes []byte, err error)) Option {
	return func(c *pullConfig) { c.fetchMetaFn = fn }
}

func withVerifyFn(fn func(manifestBytes, bundleBytes []byte) error) Option {
	return func(c *pullConfig) { c.verifyFn = fn }
}

func withPullFn(fn func(ctx context.Context, ref, stageDir string) error) Option {
	return func(c *pullConfig) { c.pullFn = fn }
}

func withEnsureFn(fn func(sha string, fetch func(stageDir string) error) (hit bool, dir string, err error)) Option {
	return func(c *pullConfig) { c.ensureFn = fn }
}
