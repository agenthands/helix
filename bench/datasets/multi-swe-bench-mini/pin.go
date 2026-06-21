// Package multiswebenchmini is a LEAF dataset-pin/fetch package (stdlib net/http
// only — no kernel/semantic/bench-runtime imports) for the Multi-SWE-bench Mini
// set (INFRA-02 / ADAPTER-MULTI-01). It is a verbatim clone of
// bench/datasets/swebench-utboost (pin.go isHexSHA1 mutable-ref refusal +
// fetch.go HELIX_CACHE_DIR precedence, io.LimitReader body cap, pinned-host SSRF
// guard, HELIX_BENCH_NETWORK-gated live fetch) with only the dataset constants
// swapped — keeping the same audited SSRF/DoS/tamper discipline.
package multiswebenchmini

import "strings"

// DatasetID is the pinned Multi-SWE-bench HuggingFace dataset repository. The
// fetcher ONLY ever resolves files under this repo, pinned by DatasetID @
// PinnedSHA — never a fork, never a caller-supplied repo (T-88-04-02 SSRF).
//
// PROVENANCE / DEFERRAL (A4 [ASSUMED]): the upstream Mini-set HF repo id is
// documented two ways — the umbrella dataset card `ByteDance-Seed/Multi-SWE-bench`
// (CC0, redistributable; 88-RESEARCH O-2) and the Mini-set split
// `bytedance-research/Multi-SWE-bench_mini` (~400 instances, 8 langs;
// 88-RESEARCH A4). The plan pins the umbrella `ByteDance-Seed/Multi-SWE-bench`
// id (the value whose CC0 license + redistributability are CONFIRMED in
// 88-RESEARCH O-2); whether the Mini split is served as a separate repo id
// vs a subdirectory/config of the umbrella repo is A4 [ASSUMED] and its LIVE
// confirmation is DEFERRED to a Docker+network host (Phase 87 precedent,
// recorded in 88-04-SUMMARY). To re-pin to the Mini split repo id if the
// checkpoint confirms it, query
// https://huggingface.co/api/datasets/<id>/refs and update this constant.
const DatasetID = "ByteDance-Seed/Multi-SWE-bench"

// Host is the pinned HuggingFace download host. The fetcher builds the resolve
// URL from Host + DatasetID + PinnedSHA + a validated path ONLY — never a
// caller-supplied URL (T-88-04-02 SSRF). https is required.
const Host = "https://huggingface.co"

// PinnedSHA is the exact immutable 40-hex commit the Multi-SWE-bench Mini set is
// pinned to. Pinning the commit means a malicious or careless upstream cannot
// swap the dataset content under us by moving a branch/tag (T-88-04-01 mutable-ref
// tampering). A mutable ref is refused by isHexSHA1 before any fetch.
//
// PROVENANCE / DEFERRAL (A4/A5 [ASSUMED]): 88-RESEARCH did NOT record an exact
// upstream commit sha (this offline environment cannot reach HF to query the
// refs API), so this rev is a documented 40-hex PLACEHOLDER chosen to satisfy
// the isHexSHA1 immutable-ref invariant. Its EXACT LIVE confirmation (that this
// 40-hex is the real immutable commit currently serving the Mini set, and that
// the CC0 license + no source-project restriction hold — A5) is DEFERRED until a
// Docker+multi_swe_bench+network host runs the gated live fetch and the A1-A7
// human-verify checkpoint is confirmed (Phase 87 precedent). To re-pin, query
// https://huggingface.co/api/datasets/ByteDance-Seed/Multi-SWE-bench/refs
// and update this constant.
const PinnedSHA = "0000000000000000000000000000000000000000"

// PinnedContentDigests is the per-file content-integrity layer that closes the
// gap PinnedSHA alone cannot. The rev-pin proves we asked HF for an immutable
// commit; it does NOT prove the bytes HF serves at that commit are the audited
// content (a wrong/moved 40-hex would silently cache whatever HF returns "forever
// as a hit"). When a file's lowercase sha256 hex is recorded here, Fetch asserts
// the downloaded payload's sha256 against it and FAILS CLOSED on mismatch — a
// moved/wrong commit, an MITM, or a poisoned mirror can never be cached or
// returned.
//
// PROVENANCE / DEFERRAL: the expected sha256s are EMPTY (rev-pin-only residual)
// until a Docker+multi_swe_bench+network host runs the gated live fetch and
// records the audited payload digests — the same offline-environment constraint
// that defers the PinnedSHA live confirmation. An UNLISTED file is currently
// allowed through on the rev-pin alone (the documented, reviewed residual until
// digests land); a LISTED file MUST match.
//
// To populate after a confirmed live fetch:
//
//	sha256sum <cacheDir>/multi-swe-bench-mini/<PinnedSHA>/<file>
//
// and add `"<file>": "<64-hex>"` here.
var PinnedContentDigests = map[string]string{}

// expectedDigest returns the pinned lowercase-hex sha256 for file and whether one
// is recorded. A caller that gets ok=false must NOT treat the absence as a
// failure (the residual rev-pin-only path), but a recorded digest MUST be
// asserted by the caller (Fetch).
func expectedDigest(file string) (string, bool) {
	d, ok := PinnedContentDigests[file]
	return d, ok && d != ""
}

// isHexSHA1 reports whether s is exactly 40 lowercase hex characters — the shape
// of a git/HF commit sha1 with no ref decoration. Mirrors swebench-utboost/
// aiderpolyglot isHexSHA1 discipline (explicit, total, no regexp): a branch/tag
// name, a short sha, an uppercase sha, or any non-hex byte is rejected, so a
// mutable ref can never reach the resolve URL (T-88-04-01).
func isHexSHA1(s string) bool {
	if len(s) != 40 {
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

// isValidHTTPSHost is a conservative guard mirroring swebench-utboost.isValidHTTPSHost:
// the host must NOT begin with '-' (the argv/flag-smuggling vector) and must be an
// https:// origin (HF downloads are TLS-only; plain http is refused so a downgrade
// cannot strip transport integrity). Total and regexp-free.
func isValidHTTPSHost(h string) bool {
	if h == "" || h[0] == '-' {
		return false
	}
	return strings.HasPrefix(h, "https://")
}
