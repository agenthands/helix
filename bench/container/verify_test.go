package container

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	tdManifest     = "testdata/manifest.json"
	tdTrustRoot    = "testdata/trusted_root.json"
	tdCanonical    = "testdata/manifest.canonical.sigstore.json"
	tdWrongOrg     = "testdata/manifest.wrong-org.sigstore.json"
	tdWrongIssuer  = "testdata/manifest.wrong-issuer.sigstore.json"
	canonicalError = "signature verification FAILED"
)

// withTestTrustRoot installs the testdata trust root as the override and
// arranges cleanup. The fixture trust root is a JSON serialization of an
// ephemeral VirtualSigstore CA produced by
// `go run -tags fixturegen ./bench/container/testdata/generate_fixtures.go`.
// It shares no trust material with any production root.
func withTestTrustRoot(t *testing.T) {
	t.Helper()
	tr, err := os.ReadFile(tdTrustRoot)
	if err != nil {
		t.Fatalf("reading testdata trust root: %v", err)
	}
	prev := testTrustedRootOverride
	testTrustedRootOverride = tr
	t.Cleanup(func() { testTrustedRootOverride = prev })
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	return b
}

// TestVerifyImageAcceptsCanonical: a bundle signed against the test CA with the
// canonical mirror SAN + canonical issuer over the manifest bytes verifies.
func TestVerifyImageAcceptsCanonical(t *testing.T) {
	withTestTrustRoot(t)
	manifest := readFixture(t, tdManifest)
	bundle := readFixture(t, tdCanonical)
	if err := VerifyImage(manifest, bundle); err != nil {
		t.Fatalf("VerifyImage(canonical) = %v, want nil", err)
	}
}

// TestVerifyImageRejectsTampered: the canonical bundle verified against
// DIFFERENT (tampered) manifest bytes returns the exact canonical literal.
func TestVerifyImageRejectsTampered(t *testing.T) {
	withTestTrustRoot(t)
	manifest := readFixture(t, tdManifest)
	bundle := readFixture(t, tdCanonical)

	tampered := append([]byte(nil), manifest...)
	tampered[len(tampered)-2] ^= 0x01 // flip a byte inside the JSON body

	err := VerifyImage(tampered, bundle)
	if err == nil {
		t.Fatalf("VerifyImage(tampered manifest) = nil, want canonical error")
	}
	if err.Error() != canonicalError {
		t.Fatalf("VerifyImage(tampered) = %q, want exactly %q", err.Error(), canonicalError)
	}
}

// TestVerifyImageRejectsWrongOrgSAN: a wrong-org SAN bundle is rejected on
// identity with the canonical literal.
func TestVerifyImageRejectsWrongOrgSAN(t *testing.T) {
	withTestTrustRoot(t)
	manifest := readFixture(t, tdManifest)
	bundle := readFixture(t, tdWrongOrg)

	err := VerifyImage(manifest, bundle)
	if err == nil {
		t.Fatalf("VerifyImage(wrong-org SAN) = nil, want canonical error")
	}
	if err.Error() != canonicalError {
		t.Fatalf("VerifyImage(wrong-org) = %q, want exactly %q", err.Error(), canonicalError)
	}
}

// TestVerifyImageRejectsWrongIssuer: a wrong-issuer bundle is rejected with the
// canonical literal.
func TestVerifyImageRejectsWrongIssuer(t *testing.T) {
	withTestTrustRoot(t)
	manifest := readFixture(t, tdManifest)
	bundle := readFixture(t, tdWrongIssuer)

	err := VerifyImage(manifest, bundle)
	if err == nil {
		t.Fatalf("VerifyImage(wrong-issuer) = nil, want canonical error")
	}
	if err.Error() != canonicalError {
		t.Fatalf("VerifyImage(wrong-issuer) = %q, want exactly %q", err.Error(), canonicalError)
	}
}

// TestVerifyImageRejectsUnsigned: garbage/missing bundle bytes return the
// canonical literal (no parse-stage oracle leakage).
func TestVerifyImageRejectsUnsigned(t *testing.T) {
	withTestTrustRoot(t)
	manifest := readFixture(t, tdManifest)

	for name, bundle := range map[string][]byte{
		"empty":   {},
		"garbage": []byte("not a sigstore bundle at all"),
		"nil":     nil,
	} {
		err := VerifyImage(manifest, bundle)
		if err == nil {
			t.Fatalf("VerifyImage(unsigned/%s) = nil, want canonical error", name)
		}
		if err.Error() != canonicalError {
			t.Fatalf("VerifyImage(unsigned/%s) = %q, want exactly %q", name, err.Error(), canonicalError)
		}
	}
}

// TestVerifyErrorTextIsIdenticalAcrossBranches asserts tampered, wrong-org,
// wrong-issuer, and unsigned ALL produce byte-identical error text (Pitfall 4 —
// no tampered-vs-wrong-identity oracle).
func TestVerifyErrorTextIsIdenticalAcrossBranches(t *testing.T) {
	withTestTrustRoot(t)
	manifest := readFixture(t, tdManifest)

	tampered := append([]byte(nil), manifest...)
	tampered[len(tampered)-2] ^= 0x01

	cases := []struct {
		name           string
		manifest       []byte
		bundle         []byte
	}{
		{"tampered", tampered, readFixture(t, tdCanonical)},
		{"wrong-org", manifest, readFixture(t, tdWrongOrg)},
		{"wrong-issuer", manifest, readFixture(t, tdWrongIssuer)},
		{"unsigned", manifest, []byte("garbage")},
		{"no-trust-root-parse", manifest, readFixture(t, tdCanonical)}, // overridden below
	}

	var texts []string
	for _, c := range cases {
		if c.name == "no-trust-root-parse" {
			// Force the trust-root-parse failure branch with garbage root.
			prev := testTrustedRootOverride
			testTrustedRootOverride = []byte("{not json")
			err := VerifyImage(c.manifest, c.bundle)
			testTrustedRootOverride = prev
			if err == nil {
				t.Fatalf("VerifyImage(%s) = nil, want canonical error", c.name)
			}
			texts = append(texts, err.Error())
			continue
		}
		err := VerifyImage(c.manifest, c.bundle)
		if err == nil {
			t.Fatalf("VerifyImage(%s) = nil, want canonical error", c.name)
		}
		texts = append(texts, err.Error())
	}

	for i := 1; i < len(texts); i++ {
		if texts[i] != texts[0] {
			t.Fatalf("error text differs across branches: %q (%s) != %q (%s) — oracle leak (Pitfall 4)",
				texts[i], cases[i].name, texts[0], cases[0].name)
		}
	}
	if texts[0] != canonicalError {
		t.Fatalf("canonical error text = %q, want %q", texts[0], canonicalError)
	}
}

// TestCanonicalLiteralGrepCoverage asserts every failure return site in
// verify.go carries the canonical literal: a comment-stripped count of the
// literal must be >= the number of failure branches (>=5). This is the
// in-test mirror of the Makefile-level grep gate (Pitfall 4 coverage).
func TestCanonicalLiteralGrepCoverage(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(".", "verify.go"))
	if err != nil {
		t.Fatalf("reading verify.go: %v", err)
	}
	count := 0
	for _, line := range splitLines(string(src)) {
		trimmed := leadingTrim(line)
		if hasPrefix(trimmed, "//") {
			continue // skip comment lines (incl. the // canonical: markers and doc block)
		}
		if containsStr(line, `errors.New("signature verification FAILED")`) {
			count++
		}
	}
	if count < 5 {
		t.Fatalf("found %d canonical-error return sites in verify.go (comment-stripped), want >= 5", count)
	}
}

// --- tiny string helpers (avoid pulling strings into the test for one-offs) ---

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func leadingTrim(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
