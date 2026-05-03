// Package upgrade implements the in-binary self-upgrade subcommand pair
// `helix update` (read-only API check) and `helix upgrade` (download → verify
// → atomic swap → relaunch). Phase 52 D-06..D-13; Phase 58 D-02 swaps
// minisign verification for sigstore cosign keyless via sigstore-go.
//
// The sigstore TUF trusted root consumed by `verify.go` is embedded at build
// time from `internal/upgrade/trusted_root.json` — a snapshot of the
// upstream public-good Sigstore TUF trust root, refreshed periodically
// (typically before each minor release) via `make update-trust-root` per
// CONTRIBUTING.md §"Trust root refresh".
package upgrade

import _ "embed"

// trustedRootJSON is the embedded sigstore TUF trusted root. The byte
// content is a snapshot of the public-good Sigstore trust root taken at
// build time; refresh per CONTRIBUTING.md §"Trust root refresh" before
// each minor release. Consumed by verify.go via root.NewTrustedRootFromJSON.
//
//go:embed trusted_root.json
var trustedRootJSON []byte
