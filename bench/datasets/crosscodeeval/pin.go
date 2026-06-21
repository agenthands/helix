package crosscodeeval

import "strings"

// Repo is the pinned CrossCodeEval HuggingFace dataset repository. The adapter
// ONLY ever resolves files under this repo, pinned by Repo @ PinnedRev — never a
// fork, never a caller-supplied repo (T-86-03-02 SSRF). It is interpolated into
// the resolve URL together with PinnedRev and a validated per-language file path.
const Repo = "Vincentvmt/CrossCodeEval"

// Host is the pinned HuggingFace download host. The fetcher builds the resolve
// URL from Host + Repo + PinnedRev + a validated path ONLY — never a
// caller-supplied URL (T-86-03-02 SSRF). https is required.
const Host = "https://huggingface.co"

// PinnedRev is the exact immutable revision the dataset is pinned to. It is a
// REAL 40-hex commit (refs/heads/main targetCommit of Vincentvmt/CrossCodeEval
// resolved at plan time, 2026-06-21 via the HF refs API), NOT a placeholder and
// NOT a mutable branch/tag: pinning the commit means a malicious or careless
// upstream cannot swap dataset content under us by moving `main` (T-86-03-01 /
// RESEARCH Pitfall 3 — the Vincentvmt/CrossCodeEval HF viewer has a known cast
// error, so we never rely on the auto-loader/viewer; we resolve THIS commit
// explicitly). To re-pin, query
// https://huggingface.co/api/datasets/Vincentvmt/CrossCodeEval/refs and update
// this constant (and re-confirm fixture provenance at the human-verify
// checkpoint).
const PinnedRev = "41f916e35cc48bcca5dc369664f931afd9ffa22f"

// isHexSHA1 reports whether s is exactly 40 lowercase hex characters — the shape
// of a git/HF commit sha1 with no ref decoration. Mirrors aiderpolyglot's
// isHexSHA1 guard discipline (explicit, total, no regexp): a branch/tag name, a
// short sha, an uppercase sha, or any non-hex byte is rejected, so a mutable ref
// can never reach the resolve URL (T-86-03-01).
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

// isValidHTTPSHost is a conservative guard mirroring aiderpolyglot.isValidGitURL:
// the host must NOT begin with '-' (the argv/flag-smuggling vector) and must be
// an https:// origin (HF downloads are TLS-only; plain http is refused so a
// downgrade cannot strip transport integrity). It is total and regexp-free.
func isValidHTTPSHost(h string) bool {
	if h == "" || h[0] == '-' {
		return false
	}
	return strings.HasPrefix(h, "https://")
}
