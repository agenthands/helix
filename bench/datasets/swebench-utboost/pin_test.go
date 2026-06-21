package swebenchutboost

import "testing"

// TestPinnedSHAIsImmutable (Phase 87 Task 5, T-87-03): the pinned UTBoost rev is
// a 40-hex immutable commit (isHexSHA1 accepts it), and the host is a valid https
// origin — so a fetch built from the pinned constants can never resolve a mutable
// ref or a downgraded transport.
func TestPinnedSHAIsImmutable(t *testing.T) {
	if !isHexSHA1(PinnedSHA) {
		t.Errorf("PinnedSHA %q is not a 40-hex immutable commit", PinnedSHA)
	}
	if !isValidHTTPSHost(Host) {
		t.Errorf("Host %q is not a valid https origin", Host)
	}
	if DatasetID == "" {
		t.Error("DatasetID must be the pinned UTBoost HF dataset id")
	}
}

// TestIsHexSHA1RejectsMutableRefs (Phase 87 Task 5, T-87-03 / Pitfall 5): a
// branch/tag name, a short sha, an uppercase sha, a 39-hex (off by one), and a
// non-hex byte are all REJECTED — only an exact 40-lowercase-hex commit passes,
// so a mutable ref can never reach the resolve URL.
func TestIsHexSHA1RejectsMutableRefs(t *testing.T) {
	bad := []string{
		"",
		"main",
		"v1.0",
		"HEAD",
		"4c21a4831d80b66e976f2a5ce946a0abded7a2a",  // 39 hex (short)
		"4c21a4831d80b66e976f2a5ce946a0abded7a2aaa", // 41 hex (long)
		"4C21A4831D80B66E976F2A5CE946A0ABDED7A2AA",  // uppercase
		"4c21a4831d80b66e976f2a5ce946a0abded7a2ag",  // non-hex 'g'
	}
	for _, b := range bad {
		if isHexSHA1(b) {
			t.Errorf("isHexSHA1(%q) = true, want false (mutable/malformed ref must be refused)", b)
		}
	}
	if !isHexSHA1(PinnedSHA) {
		t.Errorf("isHexSHA1(PinnedSHA) = false, want true")
	}
}
