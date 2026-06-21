package repobench

import "strings"

// Host is the pinned HuggingFace download host. The fetcher builds the resolve
// URL from Host + the per-language pinned Repo + PinnedRev + a validated path
// ONLY — never a caller-supplied URL (T-86-04-02 SSRF). https is required.
const Host = "https://huggingface.co"

// repoForLanguage maps a RepoBench language to its pinned HuggingFace dataset
// repository. RepoBench ships Python and Java as SEPARATE per-language repos
// (tianyang/repobench_python_v1.1, tianyang/repobench_java_v1.1), so the pin is
// per-language rather than a single Repo constant. The adapter ONLY ever
// resolves files under one of these pinned repos, each at its pinned rev — never
// a fork, never a caller-supplied repo (T-86-04-02 SSRF).
var repoForLanguage = map[string]string{
	"python": "tianyang/repobench_python_v1.1",
	"java":   "tianyang/repobench_java_v1.1",
}

// revForLanguage maps a RepoBench language to the EXACT immutable revision its
// repo is pinned to. Each is a REAL 40-hex commit (the refs/heads/main
// targetCommit of the corresponding tianyang/repobench_<lang>_v1.1 repo resolved
// at plan time, 2026-06-21, via the HF refs API), NOT a placeholder and NOT a
// mutable branch/tag: pinning the commit means a malicious or careless upstream
// cannot swap dataset content under us by moving `main` (T-86-04-01). To re-pin,
// query https://huggingface.co/api/datasets/tianyang/repobench_<lang>_v1.1/refs
// and update the corresponding entry (and re-confirm fixture provenance).
var revForLanguage = map[string]string{
	"python": "8a7cf0c8942cc1aa066bf261839650ac55a2ff79",
	"java":   "0c2b0db49ea372525f5db37b6b6ef438aa20f35d",
}

// PinnedRev returns the immutable pinned rev for a supported language, or ""
// when the language is unknown. It is the lookup half of the pin; the caller
// still validates the returned rev with isHexSHA1 before any URL build.
func PinnedRev(language string) string {
	return revForLanguage[language]
}

// PinnedRepo returns the pinned HF repo for a supported language, or "" when the
// language is unknown.
func PinnedRepo(language string) string {
	return repoForLanguage[language]
}

// isHexSHA1 reports whether s is exactly 40 lowercase hex characters — the shape
// of a git/HF commit sha1 with no ref decoration. Mirrors the crosscodeeval /
// aiderpolyglot isHexSHA1 guard discipline (explicit, total, no regexp): a
// branch/tag name, a short sha, an uppercase sha, or any non-hex byte is
// rejected, so a mutable ref can never reach the resolve URL (T-86-04-01).
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
// the host must NOT begin with '-' (the argv/flag-smuggling vector) and must be
// an https:// origin (HF downloads are TLS-only; plain http is refused so a
// downgrade cannot strip transport integrity). It is total and regexp-free.
func isValidHTTPSHost(h string) bool {
	if h == "" || h[0] == '-' {
		return false
	}
	return strings.HasPrefix(h, "https://")
}
