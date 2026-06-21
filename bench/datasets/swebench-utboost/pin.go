// Package swebenchutboost is a LEAF dataset-pin/fetch package (stdlib net/http
// only — no kernel/semantic/bench-runtime imports) for the UTBoost-augmented
// SWE-bench Verified suite (VERIFIED-02). It mirrors bench/datasets/aider-polyglot
// (pin.go isHexSHA1 mutable-ref refusal) + bench/datasets/crosscodeeval (fetch.go
// HELIX_CACHE_DIR precedence, io.LimitReader body cap, pinned-host SSRF guard,
// HELIX_BENCH_NETWORK-gated live fetch) EXACTLY.
package swebenchutboost

import "strings"

// DatasetID is the pinned UTBoost-augmented Verified HuggingFace dataset
// repository. The fetcher ONLY ever resolves files under this repo, pinned by
// DatasetID @ PinnedSHA — never a fork, never a caller-supplied repo (T-87-04
// SSRF). It is the value CONFIRMED against upstream at the Task 4 human-verify
// checkpoint (87-RESEARCH A1; the UTBoost README — github.com/CUHK-Shenzhen-SE/
// UTBoost — points at this HF dataset). UTBoost (ACL'25, MIT) augments the
// Verified test suite to catch patches that pass only the canonical tests; the
// suite is a complete drop-in for the harness --dataset_name (A2, no harness fork).
const DatasetID = "Bertsekas/SWE-Bench_Verified_UTBoost"

// Host is the pinned HuggingFace download host. The fetcher builds the resolve
// URL from Host + DatasetID + PinnedSHA + a validated path ONLY — never a
// caller-supplied URL (T-87-04 SSRF). https is required.
const Host = "https://huggingface.co"

// PinnedSHA is the exact immutable 40-hex commit the UTBoost augmented dataset is
// pinned to (87-RESEARCH A1). Pinning the commit means a malicious or careless
// upstream cannot swap the augmented-test content under us by moving a branch/tag
// (T-87-03 / Pitfall: mutable-ref tampering). A mutable ref is refused by
// isHexSHA1 before any fetch.
//
// PROVENANCE / DEFERRAL: this rev is the value 87-RESEARCH recorded and the Task 4
// checkpoint approved-with-deferral against the DOCUMENTED upstream. Its EXACT
// LIVE confirmation (that this 40-hex is the real immutable commit currently
// serving the augmented suite) is DEFERRED until a Docker+swebench+network host
// runs the gated live fetch — this offline environment cannot reach HF to
// confirm it. To re-pin, query
// https://huggingface.co/api/datasets/Bertsekas/SWE-Bench_Verified_UTBoost/refs
// and update this constant.
const PinnedSHA = "4c21a4831d80b66e976f2a5ce946a0abded7a2aa"

// isHexSHA1 reports whether s is exactly 40 lowercase hex characters — the shape
// of a git/HF commit sha1 with no ref decoration. Mirrors aiderpolyglot/
// crosscodeeval isHexSHA1 discipline (explicit, total, no regexp): a branch/tag
// name, a short sha, an uppercase sha, or any non-hex byte is rejected, so a
// mutable ref can never reach the resolve URL (T-87-03).
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

// isValidHTTPSHost is a conservative guard mirroring crosscodeeval.isValidHTTPSHost:
// the host must NOT begin with '-' (the argv/flag-smuggling vector) and must be an
// https:// origin (HF downloads are TLS-only; plain http is refused so a downgrade
// cannot strip transport integrity). Total and regexp-free.
func isValidHTTPSHost(h string) bool {
	if h == "" || h[0] == '-' {
		return false
	}
	return strings.HasPrefix(h, "https://")
}
