package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testdataPubkey   = "testdata/test_keypair.pub"
	testdataArchive  = "testdata/sample-archive.tar.gz"
	testdataMinisig  = "testdata/sample-archive.tar.gz.minisig"
	canonicalErrText = "signature verification FAILED"
)

// withTestKey installs the testdata public key as the override and returns
// a cleanup func. Tests should `defer cleanup()`.
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

func TestVerifyArchiveHappyPath(t *testing.T) {
	withTestKey(t)
	if err := VerifyArchive(testdataArchive, testdataMinisig); err != nil {
		t.Fatalf("VerifyArchive(happy path) = %v, want nil", err)
	}
}

func TestVerifyArchiveTamperedSig(t *testing.T) {
	withTestKey(t)

	// Copy the real signature into a temp file and flip a single byte in the
	// base64-encoded signature payload. The signature format is:
	//   untrusted comment: ...
	//   <base64 signature>
	//   trusted comment: ...
	//   <base64 global signature>
	// We flip one byte on line 2 so the signature decodes as well-formed-but-invalid.
	sigBytes, err := os.ReadFile(testdataMinisig)
	if err != nil {
		t.Fatalf("reading testdata sig: %v", err)
	}
	tampered := append([]byte(nil), sigBytes...)
	// Find first newline (end of "untrusted comment:" line) and flip a byte
	// on the next line.
	nl := -1
	for i, b := range tampered {
		if b == '\n' {
			nl = i
			break
		}
	}
	if nl < 0 || nl+5 >= len(tampered) {
		t.Fatalf("malformed test fixture signature")
	}
	// Flip a single byte on the signature line (skip past newline).
	target := nl + 5
	tampered[target] ^= 0x01

	tmp := filepath.Join(t.TempDir(), "tampered.minisig")
	if err := os.WriteFile(tmp, tampered, 0o644); err != nil {
		t.Fatalf("writing tampered sig: %v", err)
	}

	err = VerifyArchive(testdataArchive, tmp)
	if err == nil {
		t.Fatalf("VerifyArchive(tampered) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(tampered) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveWrongKey(t *testing.T) {
	// Use the EMBEDDED production pubkey (which is the placeholder repo-root
	// minisign.pub Plan 01 copied in) against the testdata signature. The
	// embedded key is a different keypair than test_keypair.pub, so the
	// signature must fail to verify.
	if testPubKeyOverride != nil {
		t.Fatal("test override should not be set in this case")
	}
	err := VerifyArchive(testdataArchive, testdataMinisig)
	if err == nil {
		t.Fatalf("VerifyArchive(wrong-key) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(wrong-key) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveMissingArchive(t *testing.T) {
	withTestKey(t)
	err := VerifyArchive("testdata/nonexistent.tar.gz", testdataMinisig)
	if err == nil {
		t.Fatalf("VerifyArchive(missing-archive) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(missing-archive) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveMissingSig(t *testing.T) {
	withTestKey(t)
	err := VerifyArchive(testdataArchive, "testdata/nonexistent.minisig")
	if err == nil {
		t.Fatalf("VerifyArchive(missing-sig) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(missing-sig) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}

func TestVerifyArchiveMalformedKey(t *testing.T) {
	prev := testPubKeyOverride
	testPubKeyOverride = []byte("not-a-real-pubkey\n")
	t.Cleanup(func() { testPubKeyOverride = prev })

	err := VerifyArchive(testdataArchive, testdataMinisig)
	if err == nil {
		t.Fatalf("VerifyArchive(malformed-key) = nil, want error")
	}
	if !strings.Contains(err.Error(), canonicalErrText) {
		t.Fatalf("VerifyArchive(malformed-key) err = %q, want canonical %q", err.Error(), canonicalErrText)
	}
}
