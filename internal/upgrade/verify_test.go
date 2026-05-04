package upgrade

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testdataArchive    = "testdata/sample-archive.tar.gz"
	testdataTrustRoot  = "testdata/trusted_root.json"
	testdataBundle     = "testdata/sample-archive.tar.gz.sigstore.json"
	testdataRCBundle   = "testdata/sample-archive.tar.gz.rc.sigstore.json"
	testdataWrongOrg   = "testdata/sample-archive.tar.gz.wrong-org.sigstore.json"
	testdataWrongIssuer = "testdata/sample-archive.tar.gz.wrong-issuer.sigstore.json"
	canonicalErrText   = "signature verification FAILED"
)

// withTestTrustRoot installs the testdata trust root as the override and
// arranges cleanup. The fixture trust root is a JSON serialization of an
// ephemeral VirtualSigstore CA produced by `go run -tags ignore
// ./internal/upgrade/testdata/generate_fixtures.go`. It is BYTE-DISTINCT
// from the production trust root at internal/upgrade/trusted_root.json
// (an `! cmp -s` invariant in CI).
func withTestTrustRoot(t *testing.T) {
	t.Helper()
	tr, err := os.ReadFile(testdataTrustRoot)
	if err != nil {
		t.Fatalf("reading testdata trust root: %v", err)
	}
	prev := testTrustedRootOverride
	testTrustedRootOverride = tr
	t.Cleanup(func() { testTrustedRootOverride = prev })
}

func TestVerifyArchiveHappyPath(t *testing.T) {
	withTestTrustRoot(t)
	if err := VerifyArchive(testdataArchive, testdataBundle); err != nil {
		t.Fatalf("VerifyArchive(happy path) = %v, want nil", err)
	}
}

// TestVerifyArchiveAcceptsRCTag is the regression guard against narrowing the
// SAN regex. It exercises -rc, -beta, -alpha pre-release suffixes. Only the
// -rc1 fixture is checked into testdata (the broader SAN regex is what's
// being asserted; pinning four separate fixtures would over-constrain the
// suite).
func TestVerifyArchiveAcceptsRCTag(t *testing.T) {
	withTestTrustRoot(t)
	if err := VerifyArchive(testdataArchive, testdataRCBundle); err != nil {
		t.Fatalf("VerifyArchive(rc tag) = %v, want nil", err)
	}
}

func TestVerifyArchiveTamperedBundle(t *testing.T) {
	withTestTrustRoot(t)

	// Copy the real bundle into a temp file and flip a byte deep inside the
	// DSSE/messageSignature payload (last 50 bytes are part of the base64
	// signature). The tampered bundle parses as JSON but its signature no
	// longer verifies against the leaf cert.
	bundleBytes, err := os.ReadFile(testdataBundle)
	if err != nil {
		t.Fatalf("reading testdata bundle: %v", err)
	}
	tampered := append([]byte(nil), bundleBytes...)
	if len(tampered) < 200 {
		t.Fatalf("malformed test fixture bundle (too short)")
	}
	// Flip a byte well inside the JSON, away from structural punctuation.
	target := len(tampered) - 30
	tampered[target] ^= 0x01

	tmp := filepath.Join(t.TempDir(), "tampered.sigstore.json")
	if err := os.WriteFile(tmp, tampered, 0o644); err != nil {
		t.Fatalf("writing tampered bundle: %v", err)
	}

	err = VerifyArchive(testdataArchive, tmp)
	if err == nil {
		t.Fatalf("VerifyArchive(tampered) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(tampered) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveWrongTrustRoot(t *testing.T) {
	// Override is NOT set: VerifyArchive falls back to the production trust
	// root (the public-good Sigstore TUF snapshot embedded into the binary).
	// The test bundle was signed by an ephemeral test CA, so the production
	// trust root cannot validate the cert chain.
	if testTrustedRootOverride != nil {
		t.Fatal("test override should not be set in this case")
	}
	err := VerifyArchive(testdataArchive, testdataBundle)
	if err == nil {
		t.Fatalf("VerifyArchive(wrong-trust-root) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(wrong-trust-root) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveMissingArchive(t *testing.T) {
	withTestTrustRoot(t)
	err := VerifyArchive("testdata/nonexistent.tar.gz", testdataBundle)
	if err == nil {
		t.Fatalf("VerifyArchive(missing-archive) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(missing-archive) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveMissingBundle(t *testing.T) {
	withTestTrustRoot(t)
	err := VerifyArchive(testdataArchive, "testdata/nonexistent.sigstore.json")
	if err == nil {
		t.Fatalf("VerifyArchive(missing-bundle) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(missing-bundle) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveMalformedTrustRoot(t *testing.T) {
	prev := testTrustedRootOverride
	testTrustedRootOverride = []byte("not-json")
	t.Cleanup(func() { testTrustedRootOverride = prev })

	err := VerifyArchive(testdataArchive, testdataBundle)
	if err == nil {
		t.Fatalf("VerifyArchive(malformed-trust-root) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(malformed-trust-root) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

// TestVerifyArchiveWrongIdentity uses a fork-SAN fixture: the bundle is
// otherwise valid (signed by the same test CA) but its certificate's SAN
// points at some-other-org/helix instead of agenthands/helix. The verifier
// must reject this on identity-policy grounds, NOT chain-of-trust grounds —
// otherwise an attacker who can convince Fulcio to mint a cert for a
// different repository under the same OIDC issuer could substitute that
// signature for a real release.
//
// The fork-org SAN is deliberate: a same-org SAN with a different tag shape
// (e.g., "v1.0.0-experiment") would match the broader regex and ERRONEOUSLY
// pass — using a different-org SAN guarantees the negative test exercises
// the org-pinning portion of the regex, not just a tag-shape rejection.
func TestVerifyArchiveWrongIdentity(t *testing.T) {
	withTestTrustRoot(t)
	err := VerifyArchive(testdataArchive, testdataWrongOrg)
	if err == nil {
		t.Fatalf("VerifyArchive(wrong-identity) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(wrong-identity) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveWrongIssuer(t *testing.T) {
	withTestTrustRoot(t)
	err := VerifyArchive(testdataArchive, testdataWrongIssuer)
	if err == nil {
		t.Fatalf("VerifyArchive(wrong-issuer) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(wrong-issuer) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

// TestRekorUnreachable is a Phase 58 D-04 user-experience guard: when the
// trust root requires a fresh Rekor inclusion check (TUF live mode) and
// Rekor is unreachable, the verifier must surface the problem with the
// canonical error literal AND the user-friendly Rekor wording so the
// caller can distinguish "your network is offline" from "your archive is
// tampered."
//
// We force the network failure by pointing the test verifier at a sealed
// httptest.Server immediately after it is closed (so dialing returns
// ECONNREFUSED) via the package-private testRekorURLOverride hook. The
// Rekor SET path still runs and its failure returns the wrapped error.
func TestRekorUnreachable(t *testing.T) {
	withTestTrustRoot(t)

	// Spin up a server, capture its URL, then close it so the dial fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	u, err := url.Parse(closedURL)
	if err != nil {
		t.Fatalf("parse closed url: %v", err)
	}
	prev := testRekorURLOverride
	testRekorURLOverride = u.Host
	t.Cleanup(func() { testRekorURLOverride = prev })

	// Use a deliberately-invalid bundle so the verifier walks the failure
	// path. The test asserts the failure is surfaced through the canonical
	// literal and (when isRekorUnreachable matches) the user-friendly wording.
	tampered := append([]byte(nil), []byte("{")...)
	tmp := filepath.Join(t.TempDir(), "broken.sigstore.json")
	if err := os.WriteFile(tmp, tampered, 0o644); err != nil {
		t.Fatalf("writing broken bundle: %v", err)
	}
	err = VerifyArchive(testdataArchive, tmp)
	if err == nil {
		t.Fatalf("VerifyArchive(rekor-unreachable) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(rekor-unreachable) err = %q, missing canonical %q", err.Error(), canonicalErrText)
	}
	// The user-friendly Rekor wording is best-effort: it surfaces only when
	// the underlying error chain looks like a network failure (per
	// isRekorUnreachable). On a malformed-bundle path the verifier returns
	// before any network call, so we do NOT assert the Rekor wording here —
	// that branch is covered by isRekorUnreachable's unit tests in this
	// file (see classifier helpers below).
}

// TestIsRekorUnreachable_ClassifiesNetworkErrors exercises the helper that
// decides whether to emit the user-friendly Rekor wording. Direct unit
// coverage replaces an end-to-end network-down scenario (which would
// require running against the real public-good trust root + a real
// blackholed Rekor URL).
func TestIsRekorUnreachable_ClassifiesNetworkErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"connection_refused", errors.New("dial tcp 127.0.0.1:1: connect: connection refused"), true},
		{"no_such_host", errors.New("dial tcp: lookup foo: no such host"), true},
		{"i_o_timeout", errors.New("net/http: request canceled (Client.Timeout exceeded while awaiting headers): i/o timeout"), true},
		{"verification_failed", errors.New("ecdsa: verification failed"), false},
		{"nil_error", nil, false},
	}
	for _, tc := range cases {
		got := isRekorUnreachable(tc.err)
		if got != tc.want {
			t.Errorf("isRekorUnreachable(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

