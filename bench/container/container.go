// Package container drives a local container engine (docker or podman) purely
// via os/exec with fixed argv and a strict env allowlist — it NEVER imports
// github.com/docker/docker (or any Docker SDK); the ban is enforced statically
// by the `verify-no-docker-sdk` Makefile gate wired into `make vet` (SC#1 /
// CONTAINER-01).
//
// It is a LEAF package: it imports only the Go standard library +
// go-containerregistry + sigstore-go + golang.org/x/sys, never internal/kernel
// or internal/semantic. This leaf invariant keeps the bench container stack
// independent of the kernel/semantic subsystems (mirrors bench/ragindex).
package container

import "errors"

// ImageRef identifies a digest-pinned image. Refs are ALWAYS pinned by digest,
// never by a mutable tag (CONTAINER-02 / Pitfall 2): the on-the-wire form is
// "<Repo>@sha256:<Digest>".
type ImageRef struct {
	// Repo is the registry repository, e.g. "ghcr.io/owner/name".
	Repo string
	// Digest is the lowercase hex sha256 of the image manifest WITHOUT the
	// "sha256:" prefix (exactly 64 hex chars).
	Digest string
}

// errBadDigest is returned by any path handed a digest that is not exactly 64
// lowercase hex characters. Callers fail closed before crossing the os/exec
// boundary (T-84-01-02).
var errBadDigest = errors.New("bench/container: digest must be 64-char hex sha256")

// isHexSHA256 reports whether s is exactly 64 lowercase hex characters — the
// shape of a bare sha256 digest with no "sha256:" prefix. Mirrors the
// guard-discipline of bench/ragindex's path-escape checks: explicit, total, no
// regexp.
func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// errBadRepo is returned when a repository string is not a well-formed lowercase
// OCI image reference. It fail-closes argv flag-smuggling: a repo beginning with
// '-' (which docker/podman would parse as a flag) or carrying shell/flag
// metacharacters can never reach the engine argv (T-84-01-03 / Pitfall 2).
var errBadRepo = errors.New("bench/container: repo must be a lowercase OCI image reference")

// isValidRepo reports whether repo is a conservative, lowercase OCI image
// reference: it MUST begin with [a-z0-9] (never '-' — that is the argv
// flag-smuggling vector) and contain only [a-z0-9], '.', '_', '-', '/' and ':'
// (registry port). Same explicit, total, no-regexp discipline as isHexSHA256.
// Combined with the "--" end-of-options separator in pullArgs this closes
// argv flag-smuggling even if docker/podman flag parsing changes.
func isValidRepo(repo string) bool {
	if repo == "" {
		return false
	}
	// A leading '-' is the flag-smuggling vector; a leading separator is malformed.
	if c0 := repo[0]; !((c0 >= 'a' && c0 <= 'z') || (c0 >= '0' && c0 <= '9')) {
		return false
	}
	for i := 0; i < len(repo); i++ {
		c := repo[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-' || c == '/' || c == ':':
		default:
			return false
		}
	}
	return true
}
