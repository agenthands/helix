package container

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"

	"github.com/google/go-containerregistry/pkg/crane"
)

// errFetchMetaNotWired / errPullNotWired are the fail-closed sentinels returned
// by the default (un-wired) fetch/pull collaborators. The live wiring lands
// with the Plan 04 GHCR mirror; until then a production VerifyThenPull call
// fails closed here rather than silently skipping verification or publishing
// unverified bytes.
var (
	errFetchMetaNotWired = errors.New("bench/container: manifest+bundle fetch not wired until Plan 04 mirror")
	errPullNotWired      = errors.New("bench/container: image pull not wired until Plan 04 mirror")
)

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
// when the mirror exists; until then production VerifyThenPull is exercised only
// via the gated live test (which injects a real fetchMeta) and the hermetic
// tests (which inject recorders). Returning a canonical-shaped failure here
// keeps an un-wired production call fail-closed rather than silently skipping
// verification.
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
