package upgrade

import (
	"errors"

	"github.com/jedisct1/go-minisign"
)

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

// testPubKeyOverride lets tests substitute a non-production minisign
// public key (typically the testdata test_keypair.pub) without touching
// the embedded pubKeyBytes. Tests set this in setup, defer-clear it,
// and call VerifyArchive normally. Production callers leave it nil so
// the embedded key is used.
//
// Package-private to prevent external callers from disabling
// verification at runtime.
var testPubKeyOverride []byte

// currentPubKey returns the active public key bytes — the test override
// when set, otherwise the embedded production key.
func currentPubKey() []byte {
	if testPubKeyOverride != nil {
		return testPubKeyOverride
	}
	return pubKeyBytes
}

// VerifyArchive verifies that sigPath is a valid minisign signature for
// archivePath under the embedded public key (or the test override during
// tests). Returns nil iff the signature parses, the key parses, and the
// archive's digest matches the signature's claim.
//
// On any failure — missing file, malformed signature, malformed key,
// digest mismatch, key mismatch — VerifyArchive returns the SAME
// canonical error message ("signature verification FAILED"). This is
// intentional per RESEARCH.md Pitfall 4: an attacker who can probe the
// failure mode (by sending tampered vs. wrong-key signatures) must not
// be able to distinguish them by error text or timing. The single-
// message rule means the upgrade subcommand always prints the same
// string; downstream logging never branches on the verify outcome.
//
// minisign verification quirk: `(*PublicKey).VerifyFromFile` returns
// `(bool, error)`. A malformed signature produces `(false, error)`. A
// valid-format-but-wrong-key signature produces `(false, nil)`. We
// reject the operation when EITHER condition fires (`err != nil || !ok`)
// so wrong-key signatures cannot bypass the gate.
func VerifyArchive(archivePath, sigPath string) error {
	pub, err := minisign.DecodePublicKey(string(currentPubKey()))
	if err != nil {
		// canonical: signature verification FAILED — see Pitfall 4.
		return errors.New("signature verification FAILED")
	}
	sig, err := minisign.NewSignatureFromFile(sigPath)
	if err != nil {
		// canonical: signature verification FAILED — see Pitfall 4.
		return errors.New("signature verification FAILED")
	}
	ok, err := pub.VerifyFromFile(archivePath, sig)
	if err != nil || !ok {
		// canonical: signature verification FAILED — see Pitfall 4.
		return errors.New("signature verification FAILED")
	}
	return nil
}
