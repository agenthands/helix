// Package upgrade implements the in-binary self-upgrade subcommand pair
// `helix update` (read-only API check) and `helix upgrade` (download → verify
// → atomic swap → relaunch). Phase 52 D-06..D-13.
//
// The minisign public key consumed by `verify.go` is embedded at build time
// from `internal/upgrade/minisign.pub` — a sibling of this file that is
// kept byte-identical to the repo-root `minisign.pub` by the Makefile
// `embed-pubkey` target plus the `verify-embed-pubkey` CI gate (Plan 01).
//
// The build-time copy lives next to this Go file (NOT `//go:embed
// ../../minisign.pub`) because the Go embed directive forbids `..` traversal
// (golang/go#46056). See 52-CONTEXT.md D-13 for the resolution.
package upgrade

import (
	_ "embed"
	"bytes"
)

// pubKeyBytes is the embedded minisign public key. The byte content is the
// `minisign.pub` text emitted by `minisign -G` (a `untrusted comment:` line
// followed by the base64-encoded raw key on the next line). Consumed by
// `verify.go` via `minisign.DecodePublicKey(string(pubKeyBytes))`.
//
//go:embed minisign.pub
var pubKeyBytes []byte

// placeholderMarker is the literal that release.yml's pre-flight grep
// gates on (and that minisign.pub's `untrusted comment:` line carries
// until the maintainer rotates in the production keypair). A binary
// built before key rotation embeds the all-zeros placeholder; every
// signature verification under that key fails closed with the canonical
// "signature verification FAILED" message, which is indistinguishable
// from a tampered archive. See REVIEW.md WR-05.
const placeholderMarker = "PLACEHOLDER"

// IsPlaceholderPubKey reports whether the active minisign public key
// is the pre-rotation placeholder. Callers (the `helix upgrade`
// subcommand wiring) print a developer-experience-friendly error
// distinguishing "this build was made before the maintainer rotated in
// the production minisign key" from the generic signature-verification
// failure path that fires on tampered archives.
//
// The check inspects the active key bytes via currentPubKey() so tests
// that substitute a real test_keypair.pub via testPubKeyOverride are
// NOT flagged as placeholder builds — only an actual placeholder
// (whether embedded or test-override) reports true.
func IsPlaceholderPubKey() bool {
	return bytes.Contains(currentPubKey(), []byte(placeholderMarker))
}
